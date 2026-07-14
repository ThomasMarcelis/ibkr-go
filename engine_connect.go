package ibkr

import (
	"context"
	"errors"
	"net"
	"strconv"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/transport"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
)

type connectResult struct {
	attempt       uint64
	reconnect     bool
	conn          net.Conn
	serverVersion int
	op            string
	err           error
	unsupported   bool
}

func (e *engine) startConnect(ctx context.Context, reconnect bool) {
	if e.closed {
		return
	}
	if e.connectCancel != nil {
		e.connectCancel()
	}
	var timeoutCancel context.CancelFunc
	if reconnect {
		ctx, timeoutCancel = context.WithTimeout(ctx, 5*time.Second)
	}
	e.bootstrap = bootstrapState{}
	if reconnect {
		e.setState(StateReconnecting, 0, "reconnect attempt", nil)
	} else {
		e.setState(StateConnecting, 0, "", nil)
	}

	e.connectAttemptID++
	attempt := e.connectAttemptID
	lifetime := e.lifetimeCtx
	if lifetime == nil {
		lifetime = context.Background()
	}
	attemptCtx, cancel := context.WithCancelCause(lifetime)
	stopParent := context.AfterFunc(ctx, func() { cancel(context.Cause(ctx)) })
	e.connectCancel = func() {
		stopParent()
		cancel(context.Canceled)
	}
	cfg := e.cfg
	advertisedMax := advertisedServerVersionMax
	go func() {
		if timeoutCancel != nil {
			defer timeoutCancel()
		}
		result := dialConnection(attemptCtx, cfg, advertisedMax)
		result.attempt = attempt
		result.reconnect = reconnect
		stopParent()
		cancel(context.Canceled)
		select {
		case <-e.done:
			if result.conn != nil {
				_ = result.conn.Close()
			}
		case e.connectResults <- result:
		}
	}()
}

func dialConnection(ctx context.Context, cfg config, advertisedMax int) connectResult {
	conn, err := cfg.dialer.DialContext(ctx, "tcp", net.JoinHostPort(cfg.host, strconv.Itoa(cfg.port)))
	if err != nil {
		return connectResult{op: "dial", err: err}
	}
	fail := func(op string, err error) connectResult {
		_ = conn.Close()
		if cause := context.Cause(ctx); cause != nil {
			err = cause
		}
		return connectResult{op: op, err: err}
	}
	if err := configureTCPKeepAlive(conn, cfg.tcpKeepAlive); err != nil {
		return fail("keepalive", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return fail("handshake deadline", err)
	}
	stopContextClose := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopContextClose()

	if err := transport.WriteRaw(conn, codec.EncodeHandshakePrefix()); err != nil {
		return fail("handshake", err)
	}
	if err := wire.WriteFrame(conn, codec.EncodeVersionRange(minServerVersion, advertisedMax)); err != nil {
		return fail("handshake", err)
	}
	serverPayload, err := transport.ReadOneFrameWithLimit(conn, deadline, cfg.maxInboundFrameBytes)
	if err != nil {
		if publicErr, ok := inboundFrameError(err); ok {
			err = publicErr
		}
		return fail("handshake", err)
	}
	info, err := codec.DecodeServerInfo(serverPayload)
	if err != nil {
		return fail("handshake", err)
	}
	if info.ServerVersion < minServerVersion || info.ServerVersion > advertisedMax {
		_ = conn.Close()
		return connectResult{unsupported: true, err: ErrUnsupportedServerVersion}
	}
	startPayload, err := codec.Encode(info.ServerVersion, codec.StartAPI{ClientID: int(cfg.clientID)})
	if err != nil {
		return fail("handshake", err)
	}
	if err := wire.WriteFrame(conn, startPayload); err != nil {
		return fail("handshake", err)
	}
	if cause := context.Cause(ctx); cause != nil {
		return fail("handshake", cause)
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return fail("handshake deadline", err)
	}
	return connectResult{conn: conn, serverVersion: info.ServerVersion}
}

func (e *engine) handleConnectResult(result connectResult) {
	if e.closed || result.attempt != e.connectAttemptID {
		if result.conn != nil {
			_ = result.conn.Close()
		}
		return
	}
	e.connectCancel = nil
	if result.unsupported {
		e.reportReady(ErrUnsupportedServerVersion)
		e.closeEngine(ErrUnsupportedServerVersion, ErrUnsupportedServerVersion)
		return
	}
	if result.err != nil {
		e.connectFailed(result.op, result.err, result.reconnect)
		return
	}

	e.serverVersion = result.serverVersion
	e.transportGeneration++
	clear(e.dirtySingletons)
	e.updateSnapshot(func(s *Snapshot) {
		s.ServerVersion = result.serverVersion
	})
	if result.reconnect {
		e.requireOrderRecovery(e.connectionSeq() + 1)
	}
	e.bootstrap.serverInfo = true
	e.transport = transport.NewWithInboundFrameLimit(result.conn, e.cfg.logger, e.cfg.sendRate, e.cfg.maxInboundFrameBytes)
	e.attachTransport(e.transport)
	e.scheduleBootstrapTimeout(e.transport)
	e.setState(StateHandshaking, 0, "", nil)
}

func (e *engine) connectFailed(op string, err error, reconnect bool) {
	connectErr := &ConnectError{Op: op, Err: err}
	e.rememberConnectionError(connectErr)
	if !reconnect {
		e.reportReady(connectErr)
		e.closeEngine(connectErr, connectErr)
		return
	}
	e.setState(StateReconnecting, 0, "reconnect failed", connectErr)
	e.scheduleReconnect()
}

func configureTCPKeepAlive(conn net.Conn, period time.Duration) error {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return nil
	}
	if period <= 0 {
		return tcpConn.SetKeepAlive(false)
	}
	if err := tcpConn.SetKeepAlive(true); err != nil {
		return err
	}
	return tcpConn.SetKeepAlivePeriod(period)
}

func (e *engine) attachTransport(tr *transport.Conn) {
	// Capture the negotiated version by value: each reconnect re-attaches with
	// the freshly negotiated version, and the decode pump runs off the actor
	// goroutine, so it must not read e.serverVersion directly.
	sv := e.serverVersion
	decodeResult := make(chan error, 1)
	go func() {
		var result error
		defer func() { decodeResult <- result }()
		for payload := range tr.Incoming() {
			msgs, err := codec.DecodeBatch(sv, payload)
			if err != nil {
				_ = tr.Close()
				result = &ProtocolError{Direction: "inbound", Err: err}
				return
			}
			for _, msg := range msgs {
				select {
				case e.incoming <- msg:
				case <-e.done:
					return
				}
			}
		}
	}()

	writesDone := make(chan struct{})
	go func() {
		defer close(writesDone)
		discard := false
		for result := range tr.Completions() {
			if discard {
				continue
			}
			select {
			case e.incoming <- transportWrite{transport: tr, result: result}:
			case <-e.done:
				// The transport writer must remain able to publish every tracked
				// outcome before it can close Done. Drain the source after engine
				// termination instead of leaving the writer blocked behind a full
				// completion channel.
				discard = true
			}
		}
	}()

	go func() {
		<-tr.Done()
		decodeErr := <-decodeResult
		<-writesDone
		// The ordering guarantee (all of this connection's decoded messages
		// and tracked write outcomes reach e.incoming before transportErr) is
		// preserved by waiting for both pumps.
		lossErr := errors.Join(tr.Wait(), decodeErr)
		if publicErr, ok := inboundFrameError(lossErr); ok {
			lossErr = errors.Join(lossErr, publicErr)
		}
		select {
		case e.transportErr <- transportLoss{transport: tr, err: lossErr}:
		case <-e.done:
		}
	}()
}

func inboundFrameError(err error) (*InboundFrameTooLargeError, bool) {
	frameErr, ok := errors.AsType[*wire.FrameTooLargeError](err)
	if !ok {
		return nil, false
	}
	return &InboundFrameTooLargeError{Size: frameErr.Size, Limit: frameErr.Limit}, true
}

func (e *engine) scheduleBootstrapTimeout(tr *transport.Conn) {
	time.AfterFunc(bootstrapTimeout, func() {
		e.enqueue(func() {
			if e.closed || e.transport != tr {
				return
			}
			e.snapshotMu.RLock()
			state := e.snapshot.State
			e.snapshotMu.RUnlock()
			if state != StateHandshaking {
				return
			}
			_ = tr.Close()
		})
	})
}

func (e *engine) maybeReady() {
	if e.bootstrap.readyReported || !e.bootstrap.serverInfo || !e.bootstrap.managed || !e.bootstrap.nextValidID {
		return
	}
	e.updateSnapshot(func(s *Snapshot) {
		s.ConnectionSeq++
	})
	e.setState(StateReady, 0, "", nil)
	e.restoreExecutionEvents()
	e.reportReady(nil)
	e.resumeRoutes()
}

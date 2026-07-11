package ibkr

import (
	"context"
	"net"
	"strconv"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/internal/transport"
	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

func (e *engine) startConnect(ctx context.Context, reconnect bool) {
	if e.closed {
		return
	}
	e.bootstrap = bootstrapState{}
	if reconnect {
		e.setState(StateReconnecting, 0, "reconnect attempt", nil)
	} else {
		e.setState(StateConnecting, 0, "", nil)
	}

	conn, err := e.cfg.dialer.DialContext(ctx, "tcp", net.JoinHostPort(e.cfg.host, strconv.Itoa(e.cfg.port)))
	if err != nil {
		e.connectFailed("dial", err, reconnect)
		return
	}
	if err := configureTCPKeepAlive(conn, e.cfg.tcpKeepAlive); err != nil {
		conn.Close()
		e.connectFailed("keepalive", err, reconnect)
		return
	}

	deadline := time.Now().Add(10 * time.Second)
	contextDeadline := false
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
		contextDeadline = true
	}
	if err := conn.SetDeadline(deadline); err != nil {
		conn.Close()
		e.connectFailed("handshake deadline", err, reconnect)
		return
	}
	stopContextClose := context.AfterFunc(ctx, func() { _ = conn.Close() })
	handshakeFailed := func(err error) {
		stopContextClose()
		conn.Close()
		if ctx.Err() != nil {
			err = ctx.Err()
		} else if contextDeadline && !time.Now().Before(deadline) {
			err = context.DeadlineExceeded
		}
		e.connectFailed("handshake", err, reconnect)
	}

	if err := transport.WriteRaw(conn, codec.EncodeHandshakePrefix()); err != nil {
		handshakeFailed(err)
		return
	}

	if err := wire.WriteFrame(conn, codec.EncodeVersionRange(minServerVersion, advertisedServerVersionMax)); err != nil {
		handshakeFailed(err)
		return
	}

	serverPayload, err := transport.ReadOneFrame(conn, deadline)
	if err != nil {
		handshakeFailed(err)
		return
	}
	info, err := codec.DecodeServerInfo(serverPayload)
	if err != nil {
		handshakeFailed(err)
		return
	}

	if info.ServerVersion < minServerVersion || info.ServerVersion > advertisedServerVersionMax {
		// A server-version mismatch is a protocol capability failure, not a
		// transient reconnect failure. Terminate even during reconnect.
		stopContextClose()
		conn.Close()
		e.reportReady(ErrUnsupportedServerVersion)
		e.closeEngine(ErrUnsupportedServerVersion, ErrUnsupportedServerVersion)
		return
	}
	e.serverVersion = info.ServerVersion
	e.updateSnapshot(func(s *Snapshot) {
		s.ServerVersion = info.ServerVersion
	})
	e.bootstrap.serverInfo = true

	startPayload, err := codec.Encode(e.serverVersion, codec.StartAPI{ClientID: e.cfg.clientID})
	if err != nil {
		handshakeFailed(err)
		return
	}
	if err := wire.WriteFrame(conn, startPayload); err != nil {
		handshakeFailed(err)
		return
	}
	stopContextClose()
	if ctx.Err() != nil {
		conn.Close()
		e.connectFailed("handshake", ctx.Err(), reconnect)
		return
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		conn.Close()
		e.connectFailed("handshake deadline", err, reconnect)
		return
	}

	e.transport = transport.New(conn, e.cfg.logger, e.cfg.sendRate)
	e.attachTransport(e.transport)
	e.scheduleBootstrapTimeout(e.transport)
	e.setState(StateHandshaking, 0, "", nil)
}

func (e *engine) connectFailed(op string, err error, reconnect bool) {
	connectErr := &ConnectError{Op: op, Err: err}
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
	decodedDone := make(chan struct{})
	go func() {
		defer close(decodedDone)
		for payload := range tr.Incoming() {
			msgs, err := codec.DecodeBatch(sv, payload)
			if err != nil {
				_ = tr.Close()
				// Every send races engine shutdown: once run() has exited
				// (e.done closed on Close) nothing drains e.incoming or
				// e.transportErr, so an unguarded send on a hot feed wedges
				// this goroutine forever. Bail on e.done instead.
				select {
				case e.transportErr <- transportLoss{transport: tr, err: &ProtocolError{Direction: "inbound", Err: err}}:
				case <-e.done:
				}
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

	go func() {
		<-tr.Done()
		<-decodedDone
		// The ordering guarantee (all of this connection's decoded messages
		// reach e.incoming before its transportErr) is preserved: this send is
		// gated on decodedDone, and the decode goroutine only closes it after
		// its final incoming send or an e.done bail-out.
		select {
		case e.transportErr <- transportLoss{transport: tr, err: tr.Wait()}:
		case <-e.done:
		}
	}()
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
	// Completed bootstrap is the success boundary for reconnect backoff.
	e.reconnectAttempt = 0
	e.updateSnapshot(func(s *Snapshot) {
		s.ConnectionSeq++
	})
	e.setState(StateReady, 0, "", nil)
	e.reportReady(nil)
	e.resumeRoutes()
	e.flushReadySetups()
}

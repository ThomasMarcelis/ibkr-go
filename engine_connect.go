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
		_ = conn.Close()
		e.connectFailed("keepalive", err, reconnect)
		return
	}

	// Synchronous handshake before starting transport goroutines.
	deadline := time.Now().Add(10 * time.Second)

	// 1. Send API prefix (raw bytes, not framed)
	if err := transport.WriteRaw(conn, codec.EncodeHandshakePrefix()); err != nil {
		conn.Close()
		e.connectFailed("handshake", err, reconnect)
		return
	}

	// 2. Send version range (framed)
	if err := wire.WriteFrame(conn, codec.EncodeVersionRange(minServerVersion, maxServerVersion)); err != nil {
		conn.Close()
		e.connectFailed("handshake", err, reconnect)
		return
	}

	// 3. Read server info (framed, but no msg_id prefix)
	serverPayload, err := transport.ReadOneFrame(conn, deadline)
	if err != nil {
		conn.Close()
		e.connectFailed("handshake", err, reconnect)
		return
	}
	info, err := codec.DecodeServerInfo(serverPayload)
	if err != nil {
		conn.Close()
		e.connectFailed("handshake", err, reconnect)
		return
	}

	// 4. Version check
	if info.ServerVersion < minServerVersion {
		// A server-version mismatch is a protocol capability failure, not a
		// transient reconnect failure. Terminate even during reconnect.
		conn.Close()
		e.reportReady(ErrUnsupportedServerVersion)
		e.closeEngine(ErrUnsupportedServerVersion)
		return
	}
	e.updateSnapshot(func(s *Snapshot) {
		s.ServerVersion = info.ServerVersion
	})
	e.bootstrap.serverInfo = true

	// 5. Send START_API (framed normal message)
	startPayload, err := codec.Encode(codec.StartAPI{ClientID: e.cfg.clientID})
	if err != nil {
		conn.Close()
		e.connectFailed("handshake", err, reconnect)
		return
	}
	if err := wire.WriteFrame(conn, startPayload); err != nil {
		conn.Close()
		e.connectFailed("handshake", err, reconnect)
		return
	}

	// 6. Start async transport — ManagedAccounts + NextValidID arrive on incoming channel
	e.transport = transport.New(conn, e.cfg.logger, e.cfg.sendRate)
	e.attachTransport(e.transport)
	e.scheduleBootstrapTimeout(e.transport)
	e.setState(StateHandshaking, 0, "", nil)
}

func (e *engine) connectFailed(op string, err error, reconnect bool) {
	connectErr := &ConnectError{Op: op, Err: err}
	if !reconnect {
		e.reportReady(connectErr)
		e.closeEngine(connectErr)
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
	decodedDone := make(chan struct{})
	go func() {
		defer close(decodedDone)
		for payload := range tr.Incoming() {
			msgs, err := codec.DecodeBatch(payload)
			if err != nil {
				_ = tr.Close()
				e.transportErr <- &ProtocolError{Direction: "inbound", Err: err}
				return
			}
			for _, msg := range msgs {
				e.incoming <- msg
			}
		}
	}()

	go func() {
		<-tr.Done()
		<-decodedDone
		e.transportErr <- tr.Wait()
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
	if !e.bootstrap.serverInfo || !e.bootstrap.managed || !e.bootstrap.nextValidID {
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
}

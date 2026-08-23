package ibkr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/transport"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
)

type cancelBlockingDialer struct {
	started  chan struct{}
	canceled chan struct{}
	once     sync.Once
}

type bootstrapDropDialer struct {
	mu       sync.Mutex
	conn     net.Conn
	redialed chan struct{}
	once     sync.Once
}

type connectTestDialer func(context.Context, string, string) (net.Conn, error)

func (d connectTestDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return d(ctx, network, address)
}

func (d *bootstrapDropDialer) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	d.mu.Lock()
	conn := d.conn
	d.conn = nil
	d.mu.Unlock()
	if conn != nil {
		return conn, nil
	}
	d.once.Do(func() { close(d.redialed) })
	<-ctx.Done()
	return nil, context.Cause(ctx)
}

func (d *cancelBlockingDialer) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	d.once.Do(func() { close(d.started) })
	<-ctx.Done()
	close(d.canceled)
	return nil, context.Cause(ctx)
}

func TestMaybeReadyRunsCompletionOnce(t *testing.T) {
	t.Parallel()

	runs := 0
	e := &engine{
		bootstrap: bootstrapState{serverInfo: true, managed: true, nextValidID: true},
		ready:     make(chan error, 1),
		events:    newObserver[Event](2),
		snapshot:  Snapshot{State: StateHandshaking},
		readySetups: []*readySetup{{
			ctx:  context.Background(),
			fn:   func() { runs++ },
			stop: func() bool { return true },
		}},
	}

	e.maybeReady()
	e.maybeReady()

	if runs != 1 {
		t.Fatalf("ready setup runs = %d, want 1", runs)
	}
	if got := e.Session().ConnectionSeq; got != 1 {
		t.Fatalf("connection sequence = %d, want 1", got)
	}
}

func TestCloseCancelsDialWithoutWaitingForConnectionSetup(t *testing.T) {
	t.Parallel()

	dialer := &cancelBlockingDialer{
		started:  make(chan struct{}),
		canceled: make(chan struct{}),
	}
	cfg := defaultConfig()
	cfg.dialer = dialer
	e := &engine{
		cfg:                cfg,
		cmds:               make(chan func(), 1),
		incoming:           make(chan any, 1),
		transportErr:       make(chan transportLoss, 1),
		connectResults:     make(chan connectResult),
		ready:              make(chan error, 1),
		done:               make(chan struct{}),
		events:             newObserver[Event](1),
		keyed:              make(map[int]*route),
		singletons:         make(map[string]*route),
		orders:             make(map[int64]*orderRoute),
		previews:           make(map[int64]*previewRoute),
		execDeliveries:     make(map[string]*execDelivery),
		pendingOrderWrites: make(map[transportWriteKey]int64),
		snapshot:           Snapshot{State: StateDisconnected},
	}
	e.lifetimeCtx, e.cancelLifetime = context.WithCancel(context.Background())
	go e.run()
	e.enqueue(func() { e.startConnect(context.Background(), false) })
	<-dialer.started

	e.Close()

	select {
	case <-dialer.canceled:
	case <-time.After(time.Second):
		t.Fatal("dial context was not canceled by Close")
	}
	if err := e.Wait(); err != nil {
		t.Fatalf("Wait() error = %v, want nil", err)
	}
}

func TestStaleConnectResultClosesConnection(t *testing.T) {
	t.Parallel()

	client, server := net.Pipe()
	defer server.Close()
	if err := server.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("SetReadDeadline() error = %v", err)
	}
	e := &engine{connectAttemptID: 2}
	e.handleConnectResult(connectResult{attempt: 1, conn: client})

	if _, err := server.Read(make([]byte, 1)); err == nil {
		t.Fatal("stale connection remained open")
	} else if timeout, ok := errors.AsType[net.Error](err); ok && timeout.Timeout() {
		t.Fatal("stale connection was not closed")
	}
}

func TestDialConnectionRejectsOversizedHandshakeFrame(t *testing.T) {
	t.Parallel()

	server, client := net.Pipe()
	defer server.Close()
	cfg := defaultConfig()
	cfg.maxInboundFrameBytes = 8
	cfg.dialer = connectTestDialer(func(context.Context, string, string) (net.Conn, error) {
		return client, nil
	})

	serverErr := make(chan error, 1)
	go func() {
		prefix := make([]byte, len(codec.EncodeHandshakePrefix()))
		if _, err := io.ReadFull(server, prefix); err != nil {
			serverErr <- err
			return
		}
		if _, err := wire.ReadFrame(server); err != nil {
			serverErr <- err
			return
		}
		_, err := server.Write(bytes.Join([][]byte{{0, 0, 0, 9}, []byte("unreadbody")}, nil))
		serverErr <- err
	}()

	result := dialConnection(context.Background(), cfg, advertisedServerVersionMax)
	frameErr, ok := errors.AsType[*InboundFrameTooLargeError](result.err)
	if !ok || frameErr.Size != 9 || frameErr.Limit != 8 || result.op != "handshake" {
		t.Fatalf("dialConnection() = op %q err %v", result.op, result.err)
	}
	if IsRetryable(errors.Join(ErrInterrupted, result.err)) {
		t.Fatal("oversized handshake frame became retryable when joined with ErrInterrupted")
	}
	if err := <-serverErr; err != nil && !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
}

func TestInboundFrameErrorPreservesFullUint32Header(t *testing.T) {
	t.Parallel()

	_, err := wire.ReadFrameWithLimit(bytes.NewReader([]byte{0xff, 0xff, 0xff, 0xff}), 8)
	frameErr, ok := inboundFrameError(err)
	if !ok {
		t.Fatalf("inboundFrameError() = %v, %t, want InboundFrameTooLargeError", frameErr, ok)
	}
	const wantSize uint32 = math.MaxUint32
	if frameErr.Size != wantSize || frameErr.Limit != 8 {
		t.Fatalf("inboundFrameError() = %v, %t, want size %d limit 8", frameErr, ok, uint64(math.MaxUint32))
	}
}

func TestAttachTransportTranslatesFrameLimitWithoutLosingDecodeFailure(t *testing.T) {
	t.Parallel()

	server, client := net.Pipe()
	defer server.Close()

	const frameLimit = 8
	tr := transport.NewWithInboundFrameLimit(client, nil, 0, frameLimit)
	malformed, err := wire.EncodeFrame([]byte("bad\x00"))
	if err != nil {
		t.Fatal(err)
	}
	// The first frame has an invalid classic message ID. The following header
	// exceeds the configured limit and is rejected before its body is read.
	// Starting the decoder only after Stopping deterministically exercises both
	// independent failures without relying on goroutine scheduling.
	input := append(malformed, 0, 0, 0, frameLimit+1)
	writeErr := make(chan error, 1)
	go func() {
		_, err := server.Write(input)
		writeErr <- err
	}()
	select {
	case <-tr.Stopping():
	case <-time.After(time.Second):
		t.Fatal("transport did not reject oversized header")
	}

	e := &engine{
		serverVersion: 200,
		incoming:      make(chan any, 1),
		transportErr:  make(chan transportLoss, 1),
		done:          make(chan struct{}),
	}
	e.attachTransport(tr)

	var loss transportLoss
	select {
	case loss = <-e.transportErr:
	case <-time.After(time.Second):
		t.Fatal("attachTransport did not publish terminal causes")
	}
	frameErr, ok := errors.AsType[*InboundFrameTooLargeError](loss.err)
	if !ok || frameErr.Size != frameLimit+1 || frameErr.Limit != frameLimit {
		t.Fatalf("transport loss = %v, want public frame-limit error", loss.err)
	}
	if _, leaked := errors.AsType[*wire.FrameTooLargeError](loss.err); leaked {
		t.Fatalf("transport loss leaked internal wire error: %v", loss.err)
	}
	protocolErr, ok := errors.AsType[*ProtocolError](loss.err)
	if !ok || !errors.Is(protocolErr, wire.ErrMalformedFrame) {
		t.Fatalf("transport loss = %v, want independent decode ProtocolError", loss.err)
	}
	if err := <-writeErr; err != nil {
		t.Fatal(err)
	}
}

func TestReconnectBackoffResetsAfterStableReadySession(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		tr := &transport.Conn{}
		e := &engine{
			cmds:             make(chan func(), 1),
			done:             make(chan struct{}),
			transport:        tr,
			reconnectAttempt: 4,
			snapshot:         Snapshot{State: StateReady},
		}
		defer close(e.done)

		e.scheduleReconnectStability(tr)
		time.Sleep(reconnectBackoffMax - time.Nanosecond)
		synctest.Wait()
		select {
		case <-e.cmds:
			t.Fatal("stability completed before the full window")
		default:
		}

		time.Sleep(time.Nanosecond)
		synctest.Wait()
		(<-e.cmds)()
		if got := e.reconnectAttempt; got != 0 {
			t.Fatalf("reconnectAttempt = %d, want 0", got)
		}
	})
}

func TestReconnectBackoffDoesNotResetAfterAnotherGap(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		tr := &transport.Conn{}
		e := &engine{
			cmds:             make(chan func(), 1),
			done:             make(chan struct{}),
			transport:        tr,
			reconnectAttempt: 4,
			snapshot:         Snapshot{State: StateReady},
		}
		defer close(e.done)

		e.scheduleReconnectStability(tr)
		e.invalidateReconnectStability()
		time.Sleep(reconnectBackoffMax)
		synctest.Wait()
		(<-e.cmds)()
		if got := e.reconnectAttempt; got != 4 {
			t.Fatalf("reconnectAttempt = %d, want 4", got)
		}
	})
}

func TestDialContextReconnectTimeoutIncludesLastConnectionFailure(t *testing.T) {
	t.Parallel()

	server, client := net.Pipe()
	dialer := &bootstrapDropDialer{conn: client, redialed: make(chan struct{})}
	serverErr := make(chan error, 1)
	go func() {
		defer server.Close()
		prefix := make([]byte, len(codec.EncodeHandshakePrefix()))
		if _, err := io.ReadFull(server, prefix); err != nil {
			serverErr <- fmt.Errorf("read handshake prefix: %w", err)
			return
		}
		if _, err := wire.ReadFrame(server); err != nil {
			serverErr <- fmt.Errorf("read version range: %w", err)
			return
		}
		if err := wire.WriteFrame(server, wire.EncodeFields([]string{"200", "2026-07-11T12:00:00Z"})); err != nil {
			serverErr <- fmt.Errorf("write server info: %w", err)
			return
		}
		if _, err := wire.ReadFrame(server); err != nil {
			serverErr <- fmt.Errorf("read START_API: %w", err)
			return
		}
		// Let the client clear its handshake deadline and attach the transport,
		// then drop the connection before bootstrap can complete.
		time.Sleep(20 * time.Millisecond)
		serverErr <- nil
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := dialEngine(ctx, WithDialer(dialer), WithReconnectPolicy(ReconnectAuto))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("DialContext() error = %v, want context deadline", err)
	}
	connectErr, ok := errors.AsType[*ConnectError](err)
	if !ok {
		t.Fatalf("DialContext() error = %v, want joined *ConnectError", err)
	}
	if connectErr.Op != "bootstrap" {
		t.Fatalf("ConnectError.Op = %q, want bootstrap", connectErr.Op)
	}
	select {
	case <-dialer.redialed:
	default:
		t.Fatal("ReconnectAuto did not attempt a redial")
	}
	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
}

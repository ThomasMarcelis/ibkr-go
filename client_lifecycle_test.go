package ibkr_test

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
)

type recordingDialer struct {
	called bool
}

func (d *recordingDialer) DialContext(context.Context, string, string) (net.Conn, error) {
	d.called = true
	return nil, errors.New("unexpected dial")
}

type pipeDialer struct {
	conn net.Conn
}

func (d *pipeDialer) DialContext(context.Context, string, string) (net.Conn, error) {
	if d.conn == nil {
		return nil, errors.New("unexpected dial")
	}
	conn := d.conn
	d.conn = nil
	return conn, nil
}

type stalledGateway struct {
	dialer *pipeDialer
	stop   chan struct{}
	errCh  chan error
}

type protocolErrorGateway struct {
	dialer *pipeDialer
	errCh  chan error
}

type unsupportedVersionGateway struct {
	dialer *pipeDialer
	errCh  chan error
}

func newStalledGateway(t *testing.T) *stalledGateway {
	t.Helper()

	serverConn, clientConn := net.Pipe()
	gateway := &stalledGateway{
		dialer: &pipeDialer{conn: clientConn},
		stop:   make(chan struct{}),
		errCh:  make(chan error, 1),
	}

	go func() {
		gateway.errCh <- serveStalledGateway(serverConn, gateway.stop)
	}()

	return gateway
}

func newProtocolErrorGateway(t *testing.T) *protocolErrorGateway {
	t.Helper()

	serverConn, clientConn := net.Pipe()
	gateway := &protocolErrorGateway{
		dialer: &pipeDialer{conn: clientConn},
		errCh:  make(chan error, 1),
	}

	go func() {
		gateway.errCh <- serveProtocolErrorGateway(serverConn)
	}()

	return gateway
}

func newUnknownInboundGateway(t *testing.T) *protocolErrorGateway {
	t.Helper()

	serverConn, clientConn := net.Pipe()
	gateway := &protocolErrorGateway{
		dialer: &pipeDialer{conn: clientConn},
		errCh:  make(chan error, 1),
	}

	go func() {
		gateway.errCh <- serveUnknownInboundGateway(serverConn)
	}()

	return gateway
}

func newUnsupportedVersionGateway(t *testing.T, serverVersion string) *unsupportedVersionGateway {
	t.Helper()

	serverConn, clientConn := net.Pipe()
	gateway := &unsupportedVersionGateway{
		dialer: &pipeDialer{conn: clientConn},
		errCh:  make(chan error, 1),
	}

	go func() {
		gateway.errCh <- serveUnsupportedVersionGateway(serverConn, serverVersion)
	}()

	return gateway
}

func serveStalledGateway(conn net.Conn, stop <-chan struct{}) error {
	defer conn.Close()

	prefix := make([]byte, len(codec.EncodeHandshakePrefix()))
	if _, err := io.ReadFull(conn, prefix); err != nil {
		return fmt.Errorf("read handshake prefix: %w", err)
	}
	if string(prefix) != string(codec.EncodeHandshakePrefix()) {
		return fmt.Errorf("handshake prefix = %q, want %q", string(prefix), string(codec.EncodeHandshakePrefix()))
	}
	if _, err := wire.ReadFrame(conn); err != nil {
		return fmt.Errorf("read version range: %w", err)
	}
	if err := wire.WriteFrame(conn, wire.EncodeFields([]string{"225", "20260824 21:57:33 CET"})); err != nil {
		return fmt.Errorf("write server info: %w", err)
	}
	if _, err := wire.ReadFrame(conn); err != nil {
		return fmt.Errorf("read START_API: %w", err)
	}
	managed, err := base64.StdEncoding.DecodeString("AAAA1woJRFU5MDAwMDAx")
	if err != nil {
		return fmt.Errorf("decode managed accounts: %w", err)
	}
	if err := wire.WriteFrame(conn, managed); err != nil {
		return fmt.Errorf("write managed accounts: %w", err)
	}
	nextID, err := base64.StdEncoding.DecodeString("AAAA0QjFBA==")
	if err != nil {
		return fmt.Errorf("decode next valid id: %w", err)
	}
	if err := wire.WriteFrame(conn, nextID); err != nil {
		return fmt.Errorf("write next valid id: %w", err)
	}

	<-stop
	return nil
}

func serveProtocolErrorGateway(conn net.Conn) error {
	defer conn.Close()

	prefix := make([]byte, len(codec.EncodeHandshakePrefix()))
	if _, err := io.ReadFull(conn, prefix); err != nil {
		return fmt.Errorf("read handshake prefix: %w", err)
	}
	if string(prefix) != string(codec.EncodeHandshakePrefix()) {
		return fmt.Errorf("handshake prefix = %q, want %q", string(prefix), string(codec.EncodeHandshakePrefix()))
	}
	if _, err := wire.ReadFrame(conn); err != nil {
		return fmt.Errorf("read version range: %w", err)
	}
	if err := wire.WriteFrame(conn, wire.EncodeFields([]string{"225", "20260824 21:57:33 CET"})); err != nil {
		return fmt.Errorf("write server info: %w", err)
	}
	if _, err := wire.ReadFrame(conn); err != nil {
		return fmt.Errorf("read START_API: %w", err)
	}
	managed, err := base64.StdEncoding.DecodeString("AAAA1woJRFU5MDAwMDAx")
	if err != nil {
		return fmt.Errorf("decode captured managed accounts: %w", err)
	}
	if err := wire.WriteFrame(conn, managed); err != nil {
		return fmt.Errorf("write managed accounts: %w", err)
	}
	nextID, err := base64.StdEncoding.DecodeString("AAAA0QgB")
	if err != nil {
		return fmt.Errorf("decode captured next valid id: %w", err)
	}
	if err := wire.WriteFrame(conn, nextID); err != nil {
		return fmt.Errorf("write next valid id: %w", err)
	}

	// Exact sanitized position callback from
	// captures/20260824T195734Z-positions_snapshot, events.jsonl SHA-256
	// a1b1c8fdda7af5634f462b11c62f63200f7a1125a3c0c5419f0945dee99c3919.
	// Truncating its protobuf body is deterministic structural fault injection.
	position, err := base64.StdEncoding.DecodeString("AAABBQoJRFU5MDAwMDAxEjEI6anfFRIETUVMSRoDU1RLKQAAAAAAAAAAQgZOQVNEQVFSA1VTRFoETUVMSWIDTk1TGgExIY/C9ShctJhA")
	if err != nil {
		return fmt.Errorf("decode captured position: %w", err)
	}
	if err := wire.WriteFrame(conn, position[:len(position)-1]); err != nil {
		return fmt.Errorf("write malformed frame: %w", err)
	}

	buf := make([]byte, 1)
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		if errors.Is(err, io.ErrClosedPipe) || errors.Is(err, net.ErrClosed) {
			return nil
		}
		return fmt.Errorf("set read deadline: %w", err)
	}
	_, err = conn.Read(buf)
	if err == nil {
		return fmt.Errorf("server read succeeded after protocol error; want client-side close")
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return fmt.Errorf("client did not close transport after malformed-frame test")
	}
	return nil
}

func serveUnknownInboundGateway(conn net.Conn) error {
	defer conn.Close()

	prefix := make([]byte, len(codec.EncodeHandshakePrefix()))
	if _, err := io.ReadFull(conn, prefix); err != nil {
		return fmt.Errorf("read handshake prefix: %w", err)
	}
	if string(prefix) != string(codec.EncodeHandshakePrefix()) {
		return fmt.Errorf("handshake prefix = %q, want %q", string(prefix), string(codec.EncodeHandshakePrefix()))
	}
	if _, err := wire.ReadFrame(conn); err != nil {
		return fmt.Errorf("read version range: %w", err)
	}
	if err := wire.WriteFrame(conn, wire.EncodeFields([]string{"225", "20260824 21:57:33 CET"})); err != nil {
		return fmt.Errorf("write server info: %w", err)
	}
	if _, err := wire.ReadFrame(conn); err != nil {
		return fmt.Errorf("read START_API: %w", err)
	}
	managed, err := base64.StdEncoding.DecodeString("AAAA1woJRFU5MDAwMDAx")
	if err != nil {
		return fmt.Errorf("encode managed accounts: %w", err)
	}
	if err := wire.WriteFrame(conn, managed); err != nil {
		return fmt.Errorf("write managed accounts: %w", err)
	}
	nextID, err := base64.StdEncoding.DecodeString("AAAA0QgB")
	if err != nil {
		return fmt.Errorf("encode next valid id: %w", err)
	}
	if err := wire.WriteFrame(conn, nextID); err != nil {
		return fmt.Errorf("write next valid id: %w", err)
	}

	// Reassign the raw ID of the exact captured current-time response below to
	// an unregistered protobuf ID. This is protocol-drift fault injection, not a
	// claim that the Gateway emitted base msg_id 3895.
	unknown, err := base64.StdEncoding.DecodeString("AAAP/wjC0rLUBg==")
	if err != nil {
		return fmt.Errorf("decode unknown frame: %w", err)
	}
	if err := wire.WriteFrame(conn, unknown); err != nil {
		return fmt.Errorf("write unknown frame: %w", err)
	}

	request, err := wire.ReadFrame(conn)
	if err != nil {
		return fmt.Errorf("read current time request: %w", err)
	}
	want, err := base64.StdEncoding.DecodeString("AAAA+Q==")
	if err != nil {
		return fmt.Errorf("encode current time request: %w", err)
	}
	if string(request) != string(want) {
		return fmt.Errorf("current time request = %q, want %q", request, want)
	}
	// Exact request/response from capture 20260824T202747Z-current_time,
	// events SHA-256 a9029ff8e7cfed19cab1e3e2eccc4c36d7c91b95aa6aa03f75543bacac454a9e.
	response, err := base64.StdEncoding.DecodeString("AAAA+QjC0rLUBg==")
	if err != nil {
		return fmt.Errorf("encode current time response: %w", err)
	}
	if err := wire.WriteFrame(conn, response); err != nil {
		return fmt.Errorf("write current time response: %w", err)
	}

	buf := make([]byte, 1)
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		if errors.Is(err, io.ErrClosedPipe) || errors.Is(err, net.ErrClosed) {
			return nil
		}
		return fmt.Errorf("set read deadline: %w", err)
	}
	if _, err := conn.Read(buf); err == nil {
		return fmt.Errorf("server read succeeded after lifecycle probe; want client-side close")
	} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		return fmt.Errorf("client did not close transport after lifecycle probe")
	}
	return nil
}

func serveUnsupportedVersionGateway(conn net.Conn, serverVersion string) error {
	defer conn.Close()

	prefix := make([]byte, len(codec.EncodeHandshakePrefix()))
	if _, err := io.ReadFull(conn, prefix); err != nil {
		return fmt.Errorf("read handshake prefix: %w", err)
	}
	if string(prefix) != string(codec.EncodeHandshakePrefix()) {
		return fmt.Errorf("handshake prefix = %q, want %q", string(prefix), string(codec.EncodeHandshakePrefix()))
	}
	versionRange, err := wire.ReadFrame(conn)
	if err != nil {
		return fmt.Errorf("read version range: %w", err)
	}
	if string(versionRange) != "v208..225" {
		return fmt.Errorf("version range = %q, want v208..225", string(versionRange))
	}
	if err := wire.WriteFrame(conn, wire.EncodeFields([]string{serverVersion, "2026-04-14T12:00:00Z"})); err != nil {
		return fmt.Errorf("write server info: %w", err)
	}
	return nil
}

func (g *stalledGateway) Close(t *testing.T) {
	t.Helper()

	close(g.stop)
	if err := <-g.errCh; err != nil {
		t.Fatalf("stalled gateway error = %v", err)
	}
}

func (g *protocolErrorGateway) Wait(t *testing.T) {
	t.Helper()

	if err := <-g.errCh; err != nil {
		t.Fatalf("protocol error gateway error = %v", err)
	}
}

func (g *unsupportedVersionGateway) Wait(t *testing.T) {
	t.Helper()

	if err := <-g.errCh; err != nil {
		t.Fatalf("unsupported version gateway error = %v", err)
	}
}

// TestBootstrapNoManagedAccounts verifies that DialContext fails with a timeout
// when the server sends next_valid_id but never sends managed_accounts.
func TestBootstrapNoManagedAccounts(t *testing.T) {
	t.Parallel()

	host := newHost(t, "lifecycle_bootstrap_no_accounts.txt")

	addrHost, addrPort, err := net.SplitHostPort(host.Addr())
	if err != nil {
		t.Fatalf("SplitHostPort() error = %v", err)
	}
	port, err := net.LookupPort("tcp", addrPort)
	if err != nil {
		t.Fatalf("LookupPort() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = ibkr.DialContext(ctx,
		ibkr.WithHost(addrHost),
		ibkr.WithPort(port),
		ibkr.WithReconnectPolicy(ibkr.ReconnectOff),
	)
	if err == nil {
		t.Fatal("expected error from DialContext, got nil")
	}
	// Script is still sleeping; do not waitHost.
	_ = host.Close()
}

func TestDialContextRejectsInvalidEventBuffers(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		field  string
		option ibkr.Option
	}{
		{name: "zero session events", field: "EventBuffer", option: ibkr.WithEventBuffer(0)},
		{name: "negative session events", field: "EventBuffer", option: ibkr.WithEventBuffer(-1)},
		{name: "zero order events", field: "OrderEventBuffer", option: ibkr.WithOrderEventBuffer(0)},
		{name: "negative order events", field: "OrderEventBuffer", option: ibkr.WithOrderEventBuffer(-1)},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dialer := &recordingDialer{}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			_, err := ibkr.DialContext(ctx,
				ibkr.WithDialer(dialer),
				tc.option,
			)
			if err == nil {
				t.Fatal("DialContext() error = nil, want buffer validation error")
			}
			validationErr, ok := errors.AsType[*ibkr.ValidationError](err)
			if !ok || validationErr.Field != tc.field {
				t.Fatalf("DialContext() error = %v, want %s ValidationError", err, tc.field)
			}
			if dialer.called {
				t.Fatal("DialContext() called dialer before validating buffer")
			}
		})
	}
}

func TestDialContextRejectsUnsupportedServerVersion(t *testing.T) {
	t.Parallel()

	for _, serverVersion := range []string{"207", "226"} {
		t.Run(serverVersion, func(t *testing.T) {
			t.Parallel()
			gateway := newUnsupportedVersionGateway(t, serverVersion)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			_, err := ibkr.DialContext(ctx,
				ibkr.WithDialer(gateway.dialer),
				ibkr.WithReconnectPolicy(ibkr.ReconnectOff),
			)
			if !errors.Is(err, ibkr.ErrUnsupportedServerVersion) {
				t.Fatalf("DialContext() error = %v, want ErrUnsupportedServerVersion", err)
			}
			gateway.Wait(t)
		})
	}
}

func TestDialContextCancellationInterruptsHandshakeIO(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		serve func(net.Conn)
	}{
		{name: "write", serve: func(conn net.Conn) { time.Sleep(100 * time.Millisecond) }},
		{name: "read", serve: func(conn net.Conn) {
			prefix := make([]byte, len(codec.EncodeHandshakePrefix()))
			_, _ = io.ReadFull(conn, prefix)
			_, _ = wire.ReadFrame(conn)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			serverConn, clientConn := net.Pipe()
			defer serverConn.Close()
			go test.serve(serverConn)

			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
			defer cancel()
			_, err := ibkr.DialContext(ctx,
				ibkr.WithDialer(&pipeDialer{conn: clientConn}),
				ibkr.WithReconnectPolicy(ibkr.ReconnectOff),
			)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("DialContext() error = %v, want context deadline", err)
			}
		})
	}
}

// TestBootstrapNoNextValidID verifies that DialContext fails with a timeout
// when the server sends managed_accounts but never sends next_valid_id.
func TestBootstrapNoNextValidID(t *testing.T) {
	t.Parallel()

	host := newHost(t, "lifecycle_bootstrap_no_valid_id.txt")

	addrHost, addrPort, err := net.SplitHostPort(host.Addr())
	if err != nil {
		t.Fatalf("SplitHostPort() error = %v", err)
	}
	port, err := net.LookupPort("tcp", addrPort)
	if err != nil {
		t.Fatalf("LookupPort() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = ibkr.DialContext(ctx,
		ibkr.WithHost(addrHost),
		ibkr.WithPort(port),
		ibkr.WithReconnectPolicy(ibkr.ReconnectOff),
	)
	if err == nil {
		t.Fatal("expected error from DialContext, got nil")
	}
	_ = host.Close()
}

// TestSetMarketDataTypeAfterClose verifies that MarketData().SetType returns an
// error (not blocks forever) when called after the client has been closed.
func TestSetMarketDataTypeAfterClose(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "grounded_bootstrap.txt")
	client.Close()
	<-client.Done()
	_ = host.Close()

	done := make(chan error, 1)
	go func() {
		done <- client.MarketData().SetType(context.Background(), ibkr.MarketDataDelayed)
	}()
	select {
	case err := <-done:
		if !errors.Is(err, ibkr.ErrClosed) {
			t.Fatalf("SetType() after Close = %v, want ErrClosed", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("MarketData().SetType blocked after Close — deadlock")
	}
}

// TestContextCancelDuringOneShot verifies that a one-shot method returns a
// context error when the caller's context expires before the server responds.
func TestContextCancelDuringOneShot(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "lifecycle_context_cancel.txt")
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := client.Contracts().Details(ctx, ibkr.Contract{
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	})
	if err == nil {
		t.Fatal("expected error from ContractDetails, got nil")
	}
	// The host script is still sleeping; close the listener to unblock it.
	_ = host.Close()
}

func TestKnownMalformedInboundRetiresTransportGeneration(t *testing.T) {
	t.Parallel()

	gateway := newProtocolErrorGateway(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx,
		ibkr.WithDialer(gateway.dialer),
		ibkr.WithReconnectPolicy(ibkr.ReconnectOff),
	)
	if err != nil {
		t.Fatalf("DialContext() error = %v", err)
	}

	for {
		select {
		case event := <-client.SessionEvents():
			if protocolErr, ok := errors.AsType[*ibkr.ProtocolError](event.Err); ok {
				if protocolErr.Direction != "inbound" {
					t.Fatalf("ProtocolError.Direction = %q, want inbound", protocolErr.Direction)
				}
			}
		case <-client.Done():
			err := client.Wait()
			if !errors.Is(err, ibkr.ErrInterrupted) {
				t.Fatalf("client.Wait() = %v, want ErrInterrupted", err)
			}
			if _, ok := errors.AsType[*ibkr.ProtocolError](err); !ok {
				t.Fatalf("client.Wait() = %T %v, want ProtocolError", err, err)
			}
			if ibkr.IsRetryable(err) {
				t.Fatalf("IsRetryable(%v) = true, want false", err)
			}
			gateway.Wait(t)
			return
		case <-ctx.Done():
			t.Fatal("timeout waiting for malformed generation retirement")
		}
	}
}

func TestUnknownInboundDoesNotCloseTransport(t *testing.T) {
	t.Parallel()

	gateway := newUnknownInboundGateway(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx,
		ibkr.WithDialer(gateway.dialer),
		ibkr.WithReconnectPolicy(ibkr.ReconnectOff),
	)
	if err != nil {
		t.Fatalf("DialContext() error = %v", err)
	}

	for {
		select {
		case event := <-client.SessionEvents():
			if strings.Contains(event.Message, "unknown msg_id 3895") {
				if event.Err != nil {
					t.Fatalf("unknown-ID event error = %v, want nil", event.Err)
				}
				goto observed
			}
		case <-client.Done():
			t.Fatalf("client closed after unknown inbound: %v", client.Wait())
		case <-ctx.Done():
			t.Fatal("timeout waiting for unknown inbound session event")
		}
	}

observed:

	ts, err := client.CurrentTime(ctx)
	if err != nil {
		t.Fatalf("CurrentTime() after unknown malformed inbound = %v", err)
	}
	if want := time.Unix(1787603266, 0).UTC(); !ts.Equal(want) {
		t.Fatalf("CurrentTime() = %v, want %v", ts, want)
	}

	client.Close()
	if err := client.Wait(); err != nil {
		t.Fatalf("client.Wait() error = %v, want nil after Close", err)
	}
	gateway.Wait(t)
}

func TestTransportQueueBackpressureDoesNotCloseClient(t *testing.T) {
	t.Parallel()

	gateway := newStalledGateway(t)
	defer gateway.Close(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx,
		ibkr.WithDialer(gateway.dialer),
		ibkr.WithReconnectPolicy(ibkr.ReconnectOff),
		ibkr.WithSendRate(0),
	)
	if err != nil {
		t.Fatalf("DialContext() error = %v", err)
	}
	defer client.Close()

	backpressured := false
	for i := 0; i < 512; i++ {
		err := client.MarketData().SetType(ctx, ibkr.MarketDataLive)
		if err == nil {
			continue
		}
		if !errors.Is(err, ibkr.ErrInterrupted) {
			t.Fatalf("MarketData().SetType() error = %v, want ErrInterrupted from local backpressure", err)
		}
		backpressured = true
		break
	}
	if !backpressured {
		t.Fatal("MarketData().SetType() never hit transport backpressure")
	}

	// Local observation still works while the outbound queue is full. An
	// unresolved clock request would instead retire the session on cancellation.
	observer, err := client.Orders().SubscribeExecutionEvents(ctx)
	if err != nil {
		t.Fatalf("SubscribeExecutionEvents() after local backpressure = %v", err)
	}
	observer.Close()
	if err := observer.Wait(); err != nil {
		t.Fatalf("execution observer Wait() after local backpressure = %v", err)
	}

	select {
	case <-client.Done():
		t.Fatalf("client closed after local backpressure; Wait() = %v", client.Wait())
	default:
	}

	client.Close()
	select {
	case <-client.Done():
	case <-time.After(time.Second):
		t.Fatal("client did not close promptly after local backpressure")
	}
}

// TestSubscriptionCloseImmediatelyAfterCreate verifies that closing a
// subscription immediately after creation does not panic and Wait returns nil.
func TestSubscriptionCloseImmediatelyAfterCreate(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "lifecycle_subscription_close_immediate.txt")
	defer client.Close()
	defer func() {
		// Subscription creation owns transport admission, not a completed
		// socket write. Immediate pre-snapshot retirement may therefore close
		// before the test host observes the admitted request.
		if err := host.Wait(); err != nil && !isImmediateReplayPeerClose(err) {
			t.Fatalf("host.Wait() error = %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.Accounts().SubscribePositions(ctx)
	if err != nil {
		t.Fatalf("SubscribePositions() error = %v", err)
	}

	sub.Close()
	if err := sub.Wait(); err != nil {
		t.Fatalf("sub.Wait() error = %v", err)
	}
}

func isImmediateReplayPeerClose(err error) bool {
	return errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, net.ErrClosed) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ECONNRESET)
}

// TestSingletonSubscriptionRejectsSecond verifies that a second positions
// subscription is rejected while the first is still active.
func TestSingletonSubscriptionRejectsSecond(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "lifecycle_singleton_reject.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub1, err := client.Accounts().SubscribePositions(ctx)
	if err != nil {
		t.Fatalf("SubscribePositions() first error = %v", err)
	}

	// Wait for snapshot complete so we know the first subscription is established.
	waitForStateKind(t, sub1.Events(), ibkr.StreamSnapshotComplete)

	sub2, err := client.Accounts().SubscribePositions(ctx)
	if err == nil {
		if sub2 != nil {
			sub2.Close()
		}
		t.Fatal("SubscribePositions() second error = nil, want rejection")
	}
	if !errors.Is(err, ibkr.ErrOperationActive) {
		t.Fatalf("SubscribePositions() second error = %v, want ErrOperationActive", err)
	}

	sub1.Close()
}

// TestConcurrentAccountSummaryLimit verifies that account summary enforces
// a maximum of 2 concurrent subscriptions.
func TestConcurrentAccountSummaryLimit(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "lifecycle_account_summary_limit.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub1, err := client.Accounts().SubscribeSummary(ctx, ibkr.AccountSummaryRequest{
		Group: "All",
		Tags:  []string{"NetLiquidation"},
	})
	if err != nil {
		t.Fatalf("SubscribeAccountSummary() first error = %v", err)
	}

	sub2, err := client.Accounts().SubscribeSummary(ctx, ibkr.AccountSummaryRequest{
		Group: "All",
		Tags:  []string{"TotalCashValue"},
	})
	if err != nil {
		t.Fatalf("SubscribeAccountSummary() second error = %v", err)
	}

	// The live capture issued both requests before either snapshot completed.
	waitForStateKind(t, sub1.Events(), ibkr.StreamSnapshotComplete)
	waitForStateKind(t, sub2.Events(), ibkr.StreamSnapshotComplete)

	// Third subscription must be rejected.
	sub3, err := client.Accounts().SubscribeSummary(ctx, ibkr.AccountSummaryRequest{
		Group: "All",
		Tags:  []string{"ExcessLiquidity"},
	})
	if err == nil {
		if sub3 != nil {
			sub3.Close()
		}
		t.Fatal("SubscribeAccountSummary() third error = nil, want rejection")
	}
	if !errors.Is(err, ibkr.ErrOperationActive) {
		t.Fatalf("SubscribeAccountSummary() third error = %v, want ErrOperationActive", err)
	}

	sub1.Close()
	sub2.Close()
}

// TestMultipleOneShotsInFlight verifies that concurrent one-shot requests are
// correctly demultiplexed when the second request overlaps the first response.
// Both requests and every server frame retain their captured request IDs.
func TestMultipleOneShotsInFlight(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "lifecycle_concurrent_oneshots.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type result struct {
		details []ibkr.ContractDetails
		err     error
	}

	var wg sync.WaitGroup
	results := make([]result, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		d, err := client.Contracts().Details(ctx, ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		})
		results[0] = result{details: d, err: err}
	}()
	go func() {
		defer wg.Done()
		// Small delay so AAPL request is queued first, matching the replay.
		time.Sleep(50 * time.Millisecond)
		d, err := client.Contracts().Details(ctx, ibkr.Contract{
			Symbol:   "EUR",
			SecType:  ibkr.SecTypeForex,
			Exchange: "IDEALPRO",
			Currency: "USD",
		})
		results[1] = result{details: d, err: err}
	}()
	wg.Wait()

	if results[0].err != nil {
		t.Fatalf("AAPL ContractDetails() error = %v", results[0].err)
	}
	if len(results[0].details) != 1 {
		t.Fatalf("AAPL details len = %d, want 1", len(results[0].details))
	}
	if results[0].details[0].Symbol != "AAPL" {
		t.Fatalf("AAPL symbol = %q, want AAPL", results[0].details[0].Symbol)
	}

	if results[1].err != nil {
		t.Fatalf("EURUSD ContractDetails() error = %v", results[1].err)
	}
	if len(results[1].details) != 1 {
		t.Fatalf("EURUSD details len = %d, want 1", len(results[1].details))
	}
	if details := results[1].details[0]; details.Symbol != "EUR" || details.SecType != ibkr.SecTypeForex {
		t.Fatalf("EURUSD details = %+v, want EUR CASH", details)
	}
}

// TestSessionEventsDelivered verifies that the session events channel
// receives a Ready event after bootstrap completes.
func TestSessionEventsDelivered(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "grounded_bootstrap.txt")
	defer cleanupClientHost(t, client, host)

	events := client.SessionEvents()
	found := false
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				if !found {
					t.Fatal("session events channel closed without Ready event")
				}
				return
			}
			if ev.State == ibkr.StateReady {
				found = true
			}
		case <-time.After(5 * time.Second):
			if !found {
				t.Fatal("timed out waiting for Ready session event")
			}
			return
		}
		if found {
			return
		}
	}
}

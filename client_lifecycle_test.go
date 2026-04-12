package ibkr_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
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
	if err := wire.WriteFrame(conn, wire.EncodeFields([]string{"200", "2026-04-10T12:00:00Z"})); err != nil {
		return fmt.Errorf("write server info: %w", err)
	}
	if _, err := wire.ReadFrame(conn); err != nil {
		return fmt.Errorf("read START_API: %w", err)
	}
	if err := wire.WriteFrame(conn, wire.EncodeFields([]string{"15", "1", "DU12345"})); err != nil {
		return fmt.Errorf("write managed accounts: %w", err)
	}
	if err := wire.WriteFrame(conn, wire.EncodeFields([]string{"9", "1", "1001"})); err != nil {
		return fmt.Errorf("write next valid id: %w", err)
	}

	<-stop
	return nil
}

func (g *stalledGateway) Close(t *testing.T) {
	t.Helper()

	close(g.stop)
	if err := <-g.errCh; err != nil {
		t.Fatalf("stalled gateway error = %v", err)
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

func TestDialContextRejectsInvalidEventBuffer(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		size int
	}{
		{name: "zero", size: 0},
		{name: "negative", size: -1},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dialer := &recordingDialer{}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			_, err := ibkr.DialContext(ctx,
				ibkr.WithDialer(dialer),
				ibkr.WithEventBuffer(tc.size),
			)
			if err == nil {
				t.Fatal("DialContext() error = nil, want event buffer validation error")
			}
			if !strings.Contains(err.Error(), "event buffer must be >= 1") {
				t.Fatalf("DialContext() error = %v, want event buffer validation error", err)
			}
			if dialer.called {
				t.Fatal("DialContext() called dialer before validating event buffer")
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

// TestBootstrapOutOfOrder verifies that bootstrap completes even when the
// server sends next_valid_id before managed_accounts.
func TestBootstrapOutOfOrder(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "lifecycle_bootstrap_reordered.txt")
	defer client.Close()
	defer waitHost(t, host)

	snapshot := client.Session()
	if snapshot.State != ibkr.StateReady {
		t.Fatalf("state = %s, want %s", snapshot.State, ibkr.StateReady)
	}
	if len(snapshot.ManagedAccounts) != 1 || snapshot.ManagedAccounts[0] != "DU9000001" {
		t.Fatalf("managed accounts = %v, want [DU9000001]", snapshot.ManagedAccounts)
	}
	if snapshot.NextValidID != 1 {
		t.Fatalf("next valid id = %d, want 1", snapshot.NextValidID)
	}
}

// TestSetMarketDataTypeAfterClose verifies that MarketData().SetType returns an
// error (not blocks forever) when called after the client has been closed.
func TestSetMarketDataTypeAfterClose(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "lifecycle_set_mdt_after_close.txt")
	_ = client.Close()
	<-client.Done()
	_ = host.Close()

	done := make(chan error, 1)
	go func() {
		done <- client.MarketData().SetType(context.Background(), ibkr.MarketDataDelayed)
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error from SetMarketDataType after Close, got nil")
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

	reqCtx, cancelReq := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancelReq()

	reqErrCh := make(chan error, 1)
	go func() {
		_, err := client.Contracts().Details(reqCtx, ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		})
		reqErrCh <- err
	}()

	time.Sleep(50 * time.Millisecond)

	backpressured := false
	for i := 0; i < 512; i++ {
		sendCtx, cancelSend := context.WithTimeout(context.Background(), 20*time.Millisecond)
		err := client.MarketData().SetType(sendCtx, ibkr.MarketDataLive)
		cancelSend()
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

	select {
	case err := <-reqErrCh:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("ContractDetails() error = %v, want its own context deadline", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ContractDetails() did not return after its context deadline")
	}

	select {
	case <-client.Done():
		t.Fatalf("client closed after local backpressure; Wait() = %v", client.Wait())
	default:
	}

	if err := client.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
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
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.Accounts().SubscribePositions(ctx)
	if err != nil {
		t.Fatalf("SubscribePositions() error = %v", err)
	}

	if err := sub.Close(); err != nil {
		t.Fatalf("sub.Close() error = %v", err)
	}
	if err := sub.Wait(); err != nil {
		t.Fatalf("sub.Wait() error = %v", err)
	}
}

// TestSingletonSubscriptionRejectsSecond verifies that a second positions
// subscription is rejected while the first is still active.
func TestSingletonSubscriptionRejectsSecond(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "lifecycle_singleton_reject.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub1, err := client.Accounts().SubscribePositions(ctx)
	if err != nil {
		t.Fatalf("SubscribePositions() first error = %v", err)
	}

	// Wait for snapshot complete so we know the first subscription is established.
	waitForStateKind(t, sub1.Lifecycle(), ibkr.SubscriptionSnapshotComplete)

	sub2, err := client.Accounts().SubscribePositions(ctx)
	if err == nil {
		if sub2 != nil {
			_ = sub2.Close()
		}
		t.Fatal("SubscribePositions() second error = nil, want rejection")
	}

	if err := sub1.Close(); err != nil {
		t.Fatalf("sub1.Close() error = %v", err)
	}
}

// TestConcurrentAccountSummaryLimit verifies that account summary enforces
// a maximum of 2 concurrent subscriptions.
func TestConcurrentAccountSummaryLimit(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "lifecycle_account_summary_limit.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub1, err := client.Accounts().SubscribeSummary(ctx, ibkr.AccountSummaryRequest{
		Account: "All",
		Tags:    []string{"NetLiquidation"},
	})
	if err != nil {
		t.Fatalf("SubscribeAccountSummary() first error = %v", err)
	}

	// Wait for first subscription's snapshot to complete before creating second.
	waitForStateKind(t, sub1.Lifecycle(), ibkr.SubscriptionSnapshotComplete)

	sub2, err := client.Accounts().SubscribeSummary(ctx, ibkr.AccountSummaryRequest{
		Account: "All",
		Tags:    []string{"BuyingPower"},
	})
	if err != nil {
		t.Fatalf("SubscribeAccountSummary() second error = %v", err)
	}

	// Wait for second subscription's snapshot to complete.
	waitForStateKind(t, sub2.Lifecycle(), ibkr.SubscriptionSnapshotComplete)

	// Third subscription must be rejected.
	sub3, err := client.Accounts().SubscribeSummary(ctx, ibkr.AccountSummaryRequest{
		Account: "All",
		Tags:    []string{"ExcessLiquidity"},
	})
	if err == nil {
		if sub3 != nil {
			_ = sub3.Close()
		}
		t.Fatal("SubscribeAccountSummary() third error = nil, want rejection")
	}

	if err := sub1.Close(); err != nil {
		t.Fatalf("sub1.Close() error = %v", err)
	}
	if err := sub2.Close(); err != nil {
		t.Fatalf("sub2.Close() error = %v", err)
	}
}

// TestMultipleOneShotsInFlight verifies that concurrent one-shot requests
// are correctly demultiplexed even when the server responds out of order.
// Both requests are issued from goroutines. The server responds to MSFT
// ($req2) before AAPL ($req1), exercising req-id-based routing.
func TestMultipleOneShotsInFlight(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "lifecycle_concurrent_oneshots.txt")
	defer client.Close()
	defer waitHost(t, host)

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
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		})
		results[0] = result{details: d, err: err}
	}()
	go func() {
		defer wg.Done()
		// Small delay so AAPL request is queued first, matching transcript.
		time.Sleep(50 * time.Millisecond)
		d, err := client.Contracts().Details(ctx, ibkr.Contract{
			Symbol:   "MSFT",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
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
		t.Fatalf("MSFT ContractDetails() error = %v", results[1].err)
	}
	if len(results[1].details) != 1 {
		t.Fatalf("MSFT details len = %d, want 1", len(results[1].details))
	}
	if results[1].details[0].Symbol != "MSFT" {
		t.Fatalf("MSFT symbol = %q, want MSFT", results[1].details[0].Symbol)
	}
}

// TestSessionEventsDelivered verifies that the session events channel
// receives a Ready event after bootstrap completes.
func TestSessionEventsDelivered(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "lifecycle_bootstrap_reordered.txt")
	defer client.Close()
	defer waitHost(t, host)

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

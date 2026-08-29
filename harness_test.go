package ibkr_test

import (
	"context"
	"errors"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/testhost"
)

func newClient(t *testing.T, script string, opts ...ibkr.Option) (*ibkr.Client, *testhost.Host) {
	t.Helper()

	host := newHost(t, script)
	client := dialHostClient(t, host, opts...)
	return client, host
}

func newHost(t *testing.T, script string) *testhost.Host {
	t.Helper()

	path := filepath.Join("testdata", "transcripts", script)
	host, err := testhost.NewFromFile(path)
	if err != nil {
		t.Fatalf("NewFromFile(%q) error = %v", path, err)
	}
	return host
}

func dialHostClient(t *testing.T, host *testhost.Host, opts ...ibkr.Option) *ibkr.Client {
	t.Helper()

	addrHost, addrPort, err := net.SplitHostPort(host.Addr())
	if err != nil {
		t.Fatalf("SplitHostPort() error = %v", err)
	}
	port, err := net.LookupPort("tcp", addrPort)
	if err != nil {
		t.Fatalf("LookupPort() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dialOpts := []ibkr.Option{
		ibkr.WithHost(addrHost),
		ibkr.WithPort(port),
		ibkr.WithReconnectPolicy(ibkr.ReconnectOff),
	}
	dialOpts = append(dialOpts, opts...)

	client, err := ibkr.DialContext(ctx, dialOpts...)
	if err != nil {
		t.Fatalf("DialContext() error = %v", err)
	}
	return client
}

func waitHost(t *testing.T, host *testhost.Host) {
	t.Helper()
	if err := host.Wait(); err != nil {
		t.Fatalf("host.Wait() error = %v", err)
	}
}

func cleanupClientHost(t *testing.T, client *ibkr.Client, host *testhost.Host) {
	t.Helper()
	defer client.Close()
	if t.Failed() {
		// A failed assertion can leave the scripted host waiting for a request
		// the test will never send. Closing first unblocks that read.
		client.Close()
	}
	waitHost(t, host)
}

func waitForEvent[T any](t *testing.T, ch <-chan T) T {
	t.Helper()

	select {
	case value, ok := <-ch:
		if !ok {
			t.Fatal("event channel closed before value arrived")
		}
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for event")
		var zero T
		return zero
	}
}

func waitForStateKind[T any](t *testing.T, ch <-chan ibkr.StreamEvent[T], want ibkr.StreamEventKind) ibkr.StreamEvent[T] {
	t.Helper()

	for {
		state := waitForEvent(t, ch)
		if state.Kind == want {
			return state
		}
	}
}

func waitForStreamData[T any](t *testing.T, ch <-chan ibkr.StreamEvent[T]) T {
	t.Helper()

	for {
		event := waitForEvent(t, ch)
		if event.Kind == ibkr.StreamData {
			return event.Value
		}
	}
}

func waitForSessionReady(t *testing.T, ctx context.Context, events <-chan ibkr.Event, connectionSeq uint64) {
	t.Helper()
	for {
		select {
		case event, ok := <-events:
			if !ok {
				t.Fatalf("session events closed before Ready on connection %d", connectionSeq)
			}
			if event.State == ibkr.StateReady && event.ConnectionSeq >= connectionSeq {
				return
			}
		case <-ctx.Done():
			t.Fatalf("waiting for Ready on connection %d: %v", connectionSeq, context.Cause(ctx))
		}
	}
}

// waitForSessionEventCode drains the session events channel until an event
// with the wanted code arrives.
func waitForSessionEventCode(t *testing.T, ctx context.Context, events <-chan ibkr.Event, code int) ibkr.Event {
	t.Helper()

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				t.Fatalf("session events closed before code %d", code)
			}
			if evt.Code == code {
				return evt
			}
		case <-ctx.Done():
			t.Fatalf("timeout waiting for session event code %d", code)
		}
	}
}

// waitOrderStatusUpdate consumes handle events until the wanted status
// arrives and returns the full update for field-level assertions.
func waitOrderStatusUpdate(t *testing.T, ctx context.Context, handle *ibkr.OrderHandle, want ibkr.OrderStatus) ibkr.OrderStatusUpdate {
	t.Helper()

	for {
		select {
		case evt, ok := <-handle.Events():
			if !ok {
				t.Fatalf("order events closed before status %s", want)
			}
			if evt.Status != nil && evt.Status.Status == want {
				return *evt.Status
			}
		case <-ctx.Done():
			t.Fatalf("timeout waiting for order status %s", want)
		}
	}
}

func waitForOrderStatus(t *testing.T, ctx context.Context, handle *ibkr.OrderHandle, want ibkr.OrderStatus) {
	t.Helper()
	waitOrderStatusUpdate(t, ctx, handle, want)
	if ibkr.IsTerminalOrderStatus(want) && want != ibkr.OrderStatusInactive {
		handle.Close()
	}
}

func waitForOpenOrder(t *testing.T, ctx context.Context, handle *ibkr.OrderHandle) ibkr.OpenOrder {
	t.Helper()

	for {
		select {
		case evt, ok := <-handle.Events():
			if !ok {
				t.Fatal("order events closed before OpenOrder")
			}
			if evt.OpenOrder != nil {
				return *evt.OpenOrder
			}
		case <-handle.Done():
			for {
				select {
				case evt, ok := <-handle.Events():
					if !ok {
						t.Fatal("order events closed before OpenOrder")
					}
					if evt.OpenOrder != nil {
						return *evt.OpenOrder
					}
				default:
					t.Fatal("order done before OpenOrder")
				}
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for OpenOrder")
		}
	}
}

// waitForOrderWarning drains a handle's events until a non-terminal Warning
// arrives, failing if the handle closes first. It proves the warning is
// delivered without tearing the handle down.
func waitForOrderWarning(t *testing.T, ctx context.Context, handle *ibkr.OrderHandle) *ibkr.APIError {
	t.Helper()

	for {
		select {
		case evt, ok := <-handle.Events():
			if !ok {
				t.Fatal("order events closed before the warning arrived")
			}
			if evt.Warning != nil {
				return evt.Warning
			}
		case <-handle.Done():
			t.Fatal("handle closed before delivering the non-terminal warning")
		case <-ctx.Done():
			t.Fatal("timeout waiting for order warning")
		}
	}
}

func waitOrderStatuses(t *testing.T, ctx context.Context, handle *ibkr.OrderHandle) []ibkr.OrderStatus {
	t.Helper()

	var statuses []ibkr.OrderStatus
	for {
		select {
		case evt, ok := <-handle.Events():
			if !ok {
				return statuses
			}
			if evt.Status != nil {
				statuses = append(statuses, evt.Status.Status)
				if ibkr.IsTerminalOrderStatus(evt.Status.Status) {
					if evt.Status.Status != ibkr.OrderStatusInactive {
						handle.Close()
					}
					return statuses
				}
			}
		case <-handle.Done():
			for {
				select {
				case evt, ok := <-handle.Events():
					if !ok {
						return statuses
					}
					if evt.Status != nil {
						statuses = append(statuses, evt.Status.Status)
					}
				default:
					return statuses
				}
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for order terminal status")
		}
	}
}

func waitOrderFillAndExecution(t *testing.T, ctx context.Context, handle *ibkr.OrderHandle) (bool, bool) {
	t.Helper()

	var filled bool
	var execution bool
	for {
		select {
		case evt, ok := <-handle.Events():
			if !ok {
				return filled, execution
			}
			if evt.Execution != nil {
				execution = true
			}
			if evt.Status != nil && evt.Status.Status == ibkr.OrderStatusFilled {
				filled = true
			}
			if filled && execution {
				handle.Close()
				return filled, execution
			}
		case <-handle.Done():
			for {
				select {
				case evt, ok := <-handle.Events():
					if !ok {
						return filled, execution
					}
					if evt.Execution != nil {
						execution = true
					}
					if evt.Status != nil && evt.Status.Status == ibkr.OrderStatusFilled {
						filled = true
					}
				default:
					return filled, execution
				}
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for order fill")
		}
	}
}

func hasOrderStatus(statuses []ibkr.OrderStatus, want ibkr.OrderStatus) bool {
	for _, status := range statuses {
		if status == want {
			return true
		}
	}
	return false
}

func cancelAndAwaitZeroFill(t *testing.T, ctx context.Context, handle *ibkr.OrderHandle) {
	t.Helper()
	if err := handle.Cancel(ctx); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	for {
		select {
		case event, ok := <-handle.Events():
			if !ok {
				t.Fatalf("order events closed before cancellation: %v", handle.Wait())
			}
			if event.Status == nil || !ibkr.IsTerminalOrderStatus(event.Status.Status) {
				continue
			}
			if event.Status.Status != ibkr.OrderStatusCancelled || !event.Status.Filled.IsZero() {
				t.Fatalf("terminal status = %+v, want zero-fill Cancelled", event.Status)
			}
			return
		case <-ctx.Done():
			t.Fatalf("waiting for targeted cancellation: %v", ctx.Err())
		}
	}
}

func requireCloseOrCapturedDisconnect(t *testing.T, label string, err error) {
	t.Helper()
	if err == nil {
		return
	}
	if errors.Is(err, ibkr.ErrOrderRecoveryRequired) && errors.Is(err, io.EOF) {
		return
	}
	t.Fatalf("%s Close/Wait: %v", label, err)
}

func replayAAPLQuoteEntitlement(t *testing.T, ctx context.Context, client *ibkr.Client) {
	t.Helper()

	if err := client.MarketData().SetType(ctx, ibkr.MarketDataLive); err != nil {
		t.Fatalf("SetType(live): %v", err)
	}
	_, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: ibkr.Contract{
		ConID: 265598, Symbol: "AAPL", SecType: ibkr.SecTypeStock, Exchange: "SMART", Currency: "USD",
	}})
	apiErr, ok := errors.AsType[*ibkr.APIError](err)
	if !ok || apiErr.Code != ibkr.ErrCodeAdditionalSubscriptionRequired || apiErr.OpKind != ibkr.OpQuotes {
		t.Fatalf("live AAPL Quote error = %v, want typed quotes code %d", err, ibkr.ErrCodeAdditionalSubscriptionRequired)
	}
}

func replayDelayedAAPLQuoteAnchor(t *testing.T, ctx context.Context, client *ibkr.Client) ibkr.Quote {
	t.Helper()

	replayAAPLQuoteEntitlement(t, ctx, client)
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("SetType(delayed): %v", err)
	}
	quote, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: ibkr.Contract{
		ConID: 265598, Symbol: "AAPL", SecType: ibkr.SecTypeStock, Exchange: "SMART", Currency: "USD",
	}})
	if err != nil {
		t.Fatalf("delayed AAPL Quote: %v", err)
	}
	return quote
}

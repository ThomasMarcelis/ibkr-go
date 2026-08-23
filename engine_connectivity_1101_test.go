package ibkr

import (
	"bytes"
	"context"
	"errors"
	"net"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/transport"
	"github.com/shopspring/decimal"
)

func TestConnectivity1101BareGapsOrderBeforeRecovery(t *testing.T) {
	t.Parallel()

	e, _ := newEngineForDispatchTest()
	e.cmds = make(chan func(), 1)
	e.done = make(chan struct{})
	e.events = newObserver[Event](8)
	e.serverVersion = 200
	e.snapshot = Snapshot{State: StateReady, ConnectionSeq: 4}
	peer, client := net.Pipe()
	e.transport = transport.New(client, nil, 0)
	t.Cleanup(func() {
		_ = e.transport.Close()
		_ = peer.Close()
		_ = e.transport.Wait()
	})
	handle := e.bindOrderHandle(47, Stock("AAPL"))
	executions := newSubscription[ExecutionEvent](subscriptionConfig{buffer: 2}, nil)
	e.executionEvents = &executionEventRoute{sub: executions}

	e.handleAPIError(codec.APIError{Code: 1101, Message: "Connectivity restored - data lost."})

	gap := nextConnectivityOrderLifecycle(t, handle)
	if gap.Kind != OrderGap || gap.ConnectionSeq != 4 || !errors.Is(gap.Err, ErrInterrupted) {
		t.Fatalf("first order lifecycle = %+v, want interrupted Gap at sequence 4", gap)
	}
	recovery := nextConnectivityOrderLifecycle(t, handle)
	if recovery.Kind != OrderRecoveryRequired || recovery.ConnectionSeq != 4 ||
		!errors.Is(recovery.Err, ErrOrderRecoveryRequired) {
		t.Fatalf("second order lifecycle = %+v, want RecoveryRequired at sequence 4", recovery)
	}

	executionGap := <-executions.Events()
	if executionGap.Kind != StreamGap || !errors.Is(executionGap.Err, ErrInterrupted) {
		t.Fatalf("execution event = %+v, want interrupted Gap", executionGap)
	}
	if restored := <-executions.Events(); restored.Kind != StreamRestored {
		t.Fatalf("execution restoration = %+v, want Restored", restored)
	}
	select {
	case event := <-executions.Events():
		t.Fatalf("duplicate execution lifecycle event = %+v", event)
	default:
	}

	replaceResult := make(chan error, 1)
	go func() {
		replaceResult <- handle.Replace(context.Background(), Order{
			Action: ActionBuy, OrderType: OrderTypeMarket,
			Quantity: decimal.NewFromInt(1), TIF: TIFDay,
		})
	}()
	(<-e.cmds)()
	if err := <-replaceResult; !errors.Is(err, ErrOrderRecoveryRequired) {
		t.Fatalf("Replace after bare 1101 = %v, want ErrOrderRecoveryRequired", err)
	}
}

func TestDegradedWork1101ClearsQuoteBeforeResubscribe(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	e.serverVersion = 200
	e.nextReqID = 1001
	e.handleAPIError(codec.APIError{Code: 1100, Message: "Connectivity between IB and TWS has been lost."})

	req := QuoteRequest{Contract: Stock("AAPL")}
	auto := installObservedQuoteRoute(t, e, req, WithResumePolicy(ResumeAuto))
	initialRequest := readObservedFrame(t, peer)
	if event := <-auto.Events(); event.Kind != StreamStarted {
		t.Fatalf("automatic subscription first event = %+v, want Started", event)
	}
	never := installObservedQuoteRoute(t, e, req, WithResumePolicy(ResumeNever))
	_ = readObservedFrame(t, peer)
	if event := <-never.Events(); event.Kind != StreamStarted {
		t.Fatalf("non-resuming subscription first event = %+v, want Started", event)
	}

	// Capture 20260405T215752Z-quote_stream_genericticks, server_version 200,
	// raw.txt sha256 9c4fec0cd44041ccfec4fee372ed6cea437418183b42591936c64ee4fdf52bee.
	e.handleIncoming(decodeOne(t, []byte("1\x006\x001001\x0068\x00255.45\x00200\x000\x00")))
	before := <-auto.Events()
	if before.Kind != StreamData || before.Value.Snapshot.Available&QuoteFieldLast == 0 {
		t.Fatalf("pre-restoration quote = %+v, want captured last price", before)
	}

	e.handleAPIError(codec.APIError{Code: 1101, Message: "Connectivity restored - data lost."})

	if event := nextConnectivityStreamEvent(t, auto); event.Kind != StreamGap {
		t.Fatalf("first automatic event after 1101 = %+v, want Gap", event)
	}
	if event := nextConnectivityStreamEvent(t, auto); event.Kind != StreamResubscribed {
		t.Fatalf("second automatic event after 1101 = %+v, want Resubscribed", event)
	}
	if resumedRequest := readObservedFrame(t, peer); !bytes.Equal(resumedRequest, initialRequest) {
		t.Fatalf("resumed quote request = %x, want original %x", resumedRequest, initialRequest)
	}
	if err := never.Wait(); !errors.Is(err, ErrResumeRequired) || !IsRetryable(err) {
		t.Fatalf("ResumeNever Wait() = %v, want retryable ErrResumeRequired", err)
	}

	// The matching captured MarketDataType frame exposes whether the quote
	// accumulator retained the captured last price across the 1101 boundary.
	e.handleIncoming(decodeOne(t, []byte("58\x001\x001001\x003\x00")))
	after := <-auto.Events()
	if after.Kind != StreamData || after.Value.Snapshot.Available != QuoteFieldMarketDataType {
		t.Fatalf("first quote after resubscribe retained pre-loss fields: %+v", after)
	}

	auto.Close()
	(<-e.cmds)()
	_ = readObservedFrame(t, peer)
	if err := auto.Wait(); err != nil {
		t.Fatalf("automatic subscription cleanup: %v", err)
	}
}

func nextConnectivityOrderLifecycle(t *testing.T, handle *OrderHandle) OrderLifecycleEvent {
	t.Helper()
	select {
	case event := <-handle.Events():
		if event.Lifecycle == nil {
			t.Fatalf("order event = %+v, want lifecycle", event)
		}
		return *event.Lifecycle
	default:
		t.Fatal("order lifecycle event missing")
		return OrderLifecycleEvent{}
	}
}

func nextConnectivityStreamEvent[T any](t *testing.T, sub *Subscription[T]) StreamEvent[T] {
	t.Helper()
	select {
	case event := <-sub.Events():
		return event
	default:
		t.Fatal("stream lifecycle event missing")
		return StreamEvent[T]{}
	}
}

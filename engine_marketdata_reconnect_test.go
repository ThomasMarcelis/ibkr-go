package ibkr

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
)

// The delayed selection and request order are live-verified in
// delayed_quote_reconnect.txt (sv225). These tests inject queue saturation
// and transport generations without changing any Gateway callback.
func setObservedMarketDataType(e *engine, dataType MarketDataType) error {
	result := make(chan error, 1)
	go func() { result <- e.SetMarketDataType(context.Background(), dataType) }()
	(<-e.cmds)()
	return <-result
}

func TestMarketDataTypeRetainsOnlyAdmittedSelection(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	if err := setObservedMarketDataType(e, MarketDataDelayed); err != nil {
		t.Fatal(err)
	}
	_ = readObservedFrame(t, peer)
	fillTransportQueue(t, e.transport, peer)
	if err := setObservedMarketDataType(e, MarketDataFrozen); !errors.Is(err, ErrInterrupted) {
		t.Fatalf("SetType: %v", err)
	}
	if e.marketDataType != MarketDataDelayed {
		t.Fatalf("selection = %v", e.marketDataType)
	}
}

func TestMarketDataTypeRestoresWithoutSubscriptionsOncePerGeneration(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	if err := setObservedMarketDataType(e, MarketDataDelayed); err != nil {
		t.Fatal(err)
	}
	want := readObservedFrame(t, peer)
	clock, err := codec.Encode(225, codec.CurrentTimeRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for generation := uint64(2); generation <= 4; generation++ {
		// Inject the next physical generation, retaining the test pipe to inspect
		// admitted bytes. There are deliberately no surviving subscriptions.
		e.transportGeneration = generation
		if e.isReady() {
			t.Fatal("new work admitted before restoring selection")
		}
		e.resumeRoutes()
		if got := readObservedFrame(t, peer); !bytes.Equal(got, want) {
			t.Fatalf("restore = %x, want %x", got, want)
		}
		if !e.isReady() {
			t.Fatal("selection restoration did not release new work")
		}
		e.resumeRoutes() // Same-socket data-lost restoration must not resend SetType.
		if err := e.transport.Send(context.Background(), clock); err != nil {
			t.Fatal(err)
		}
		if got := readObservedFrame(t, peer); !bytes.Equal(got, clock) {
			t.Fatalf("duplicate selection: %x", got)
		}
	}
}

func TestMarketDataTypeRestorationWaitsForCapacityBeforeNewWork(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	if err := setObservedMarketDataType(e, MarketDataDelayed); err != nil {
		t.Fatal(err)
	}
	want := readObservedFrame(t, peer)
	fillTransportQueue(t, e.transport, peer)
	<-e.transport.Writable() // Discard the prime dequeue's edge.
	e.transportGeneration++
	admitted := false
	e.readySetups = []*readySetup{{ctx: context.Background(), stop: func() bool { return true }, fn: func() { admitted = true }}}
	e.resumeRoutes()
	if !e.resumeWaiting || admitted || e.isReady() {
		t.Fatal("restoration bypassed the full queue")
	}
	// Finish the prime frame, releasing exactly one queue slot.
	prime, err := codec.Encode(225, codec.CancelOrderRequest{OrderID: 477})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(peer, make([]byte, len(prime))); err != nil {
		t.Fatal(err)
	}
	select {
	case wake := <-e.cmds:
		wake()
	case <-time.After(time.Second):
		t.Fatal("missing capacity wake")
	}
	if !admitted || !e.isReady() {
		t.Fatal("restoration did not release waiting work")
	}
	for {
		got := readObservedFrame(t, peer)
		if bytes.Equal(got, want) {
			break
		}
		if !bytes.Equal(got, prime) {
			t.Fatalf("unexpected queue frame: %x", got)
		}
	}
}

func TestNeverSelectedMarketDataTypeLeavesGatewayDefault(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	e.transportGeneration++
	e.resumeRoutes()
	payload, err := codec.Encode(225, codec.CurrentTimeRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.transport.Send(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	if got := readObservedFrame(t, peer); !bytes.Equal(got, payload) {
		t.Fatalf("unexpected default selection: %x", got)
	}
}

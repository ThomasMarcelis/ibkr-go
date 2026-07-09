package ibkr

import (
	"context"
	"errors"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/transport"
	"github.com/shopspring/decimal"
)

// TestPlaceOrderOrphanBestEffortCancel freezes the ctx-cancellation
// orphan-recovery contract: when PlaceOrder's caller abandons the call after
// the order already reached the wire, the engine best-effort cancels the now
// ownerless order and detaches its handle with the caller's cancellation
// cause. The abandonment is driven directly through resolveOrphanedPlaceOrder
// (the cancel func PlaceOrder installs on awaitOneShotResponse's ctx.Done arm);
// the exact select-race window cannot be reproduced deterministically without a
// production seam, but the recovery path it triggers is exercised end to end.
// A net.Pipe observes only the two client frames; no Gateway reply is invented.
//
// Pre-fix, PlaceOrder passed a nil cancel func and a delivered handle was never
// drained: no cancel_order was ever emitted and the order stayed live at IB.
func TestPlaceOrderOrphanBestEffortCancel(t *testing.T) {
	t.Parallel()

	serverConn, clientConn := net.Pipe()
	tr := transport.New(clientConn, nil, 0)
	cfg := defaultConfig()
	e := &engine{
		cfg:            cfg,
		cmds:           make(chan func(), 8),
		incoming:       make(chan any, 8),
		transportErr:   make(chan transportLoss, 1),
		done:           make(chan struct{}),
		events:         newObserver[Event](8),
		transport:      tr,
		serverVersion:  200,
		keyed:          make(map[int]*route),
		singletons:     make(map[string]*route),
		orders:         make(map[int64]*orderRoute),
		executions:     newExecutionCorrelator(),
		execDeliveries: make(map[string]*execDelivery),
		snapshot:       Snapshot{State: StateReady, ConnectionSeq: 1, NextValidID: 500},
	}
	go e.run()
	t.Cleanup(func() {
		_ = e.Close()
		_ = e.Wait()
		_ = serverConn.Close()
	})

	handle, err := e.PlaceOrder(context.Background(), PlaceOrderRequest{
		Contract: Contract{Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD"},
		Order:    Order{Action: ActionBuy, OrderType: OrderTypeMarket, Quantity: decimal.NewFromInt(10)},
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if handle.orderID != 500 {
		t.Fatalf("order id = %d, want 500", handle.orderID)
	}
	placeFields := readWireFields(t, serverConn)
	wantPlacePrefix := []string{"3", "500", "0", "AAPL", "STK"}
	if len(placeFields) < len(wantPlacePrefix) || !slices.Equal(placeFields[:len(wantPlacePrefix)], wantPlacePrefix) {
		t.Fatalf("place_order prefix = %q, want %q", placeFields, wantPlacePrefix)
	}

	// Simulate the abandoned caller: hand the delivered result to the resolver
	// with the caller's cancellation cause.
	resp := make(chan placeOrderResult, 1)
	resp <- placeOrderResult{handle: handle}
	e.resolveOrphanedPlaceOrder(resp, context.Canceled)

	if got := readWireFields(t, serverConn); !slices.Equal(got, []string{"4", "500", "", "", ""}) {
		t.Fatalf("cancel_order fields = %q, want [4 500 <empty> <empty> <empty>]", got)
	}

	select {
	case <-handle.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("handle not detached after orphan resolution")
	}
	if err := handle.Wait(); !errors.Is(err, context.Canceled) {
		t.Fatalf("handle.Wait() = %v, want context.Canceled", err)
	}

	// The route is torn down: no orderRoute lingers for the cancelled order.
	gone := make(chan bool, 1)
	e.enqueue(func() { _, ok := e.orders[500]; gone <- !ok })
	select {
	case ok := <-gone:
		if !ok {
			t.Fatal("order route 500 still registered after orphan cancel")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout querying route teardown")
	}
}

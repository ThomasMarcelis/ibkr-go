package ibkr

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/testing/testhost"
	"github.com/shopspring/decimal"
)

// dialEngineToHost starts a testhost from an inline script and dials a real
// engine at it. The script's only server content is the standard sv200
// bootstrap; the remaining steps assert the client frames the engine emits.
func dialEngineToHost(t *testing.T, script string) (*engine, *testhost.Host) {
	t.Helper()

	host, err := testhost.New(script)
	if err != nil {
		t.Fatalf("testhost.New: %v", err)
	}
	addrHost, addrPort, err := net.SplitHostPort(host.Addr())
	if err != nil {
		t.Fatalf("SplitHostPort: %v", err)
	}
	port, err := net.LookupPort("tcp", addrPort)
	if err != nil {
		t.Fatalf("LookupPort: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	e, err := dialEngine(ctx, WithHost(addrHost), WithPort(port), WithReconnectPolicy(ReconnectOff))
	if err != nil {
		t.Fatalf("dialEngine: %v", err)
	}
	return e, host
}

// TestPlaceOrderOrphanBestEffortCancel freezes the ctx-cancellation
// orphan-recovery contract: when PlaceOrder's caller abandons the call after
// the order already reached the wire, the engine best-effort cancels the now
// ownerless order and detaches its handle with the caller's cancellation
// cause. The abandonment is driven directly through resolveOrphanedPlaceOrder
// (the cancel func PlaceOrder installs on awaitOneShotResponse's ctx.Done arm);
// the exact select-race window cannot be reproduced deterministically without a
// production seam, but the recovery path it triggers is exercised end to end:
// the host asserts a real cancel_order frame goes out for the allocated id, and
// the route is torn down.
//
// Pre-fix, PlaceOrder passed a nil cancel func and a delivered handle was never
// drained: no cancel_order was ever emitted and the order stayed live at IB.
func TestPlaceOrderOrphanBestEffortCancel(t *testing.T) {
	script := `handshake {"server_version":200,"connection_time":"20260705 00:00:00 Coordinated Universal Time"}
server managed_accounts {"accounts":["DU9000001"]}
server next_valid_id {"order_id":500}
client place_order {"order_id":"500","action":"BUY","order_type":"MKT","total_quantity":"10"}
client cancel_order {"field_count":"5","order_id":"500"}
sleep 300ms
disconnect
`
	e, host := dialEngineToHost(t, script)
	defer func() { _ = e.Close() }()

	// A real placement: the handle and its route exist and place_order reached
	// the wire (the host matches it). It is then treated as ownerless.
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

	// Simulate the abandoned caller: hand the delivered result to the resolver
	// with the caller's cancellation cause.
	resp := make(chan placeOrderResult, 1)
	resp <- placeOrderResult{handle: handle}
	e.resolveOrphanedPlaceOrder(resp, context.Canceled)

	// The handle is detached with the caller's cause once the best-effort
	// cancel has gone out.
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

	// The host proves both frames — place_order(500) then cancel_order(500) —
	// really went out on the wire.
	if err := host.Wait(); err != nil {
		t.Fatalf("host.Wait() = %v, want place_order then cancel_order for id 500", err)
	}
}

package ibkr_test

import (
	"context"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/shopspring/decimal"
)

func TestOrderLifecycleServer203Replay(t *testing.T) {
	restore := ibkr.SetAdvertisedServerVersionMaxForTest(203)
	defer restore()

	client, host := newClient(t, "order_lifecycle_sv203_live.txt")
	defer client.Close()
	defer waitHost(t, host)
	if got := client.Session().ServerVersion; got != 203 {
		t.Fatalf("Session().ServerVersion = %d, want 203", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplContract,
		Order: ibkr.Order{
			Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeLimit,
			Quantity: decimal.NewFromInt(1), LmtPrice: new(decimal.NewFromInt(50)), TIF: ibkr.TIFDay,
		},
	})
	if err != nil {
		t.Fatalf("Place() error = %v", err)
	}
	var sawOpen, sawStatus bool
	for !sawOpen || !sawStatus {
		select {
		case event := <-handle.Events():
			if event.OpenOrder != nil {
				sawOpen = true
				if event.OpenOrder.Contract.ConID != 265598 || event.OpenOrder.State.Status != ibkr.OrderStatusPreSubmitted {
					t.Fatalf("OpenOrder = %+v", event.OpenOrder)
				}
				if event.OpenOrder.Order.IncludeOvernight != nil {
					t.Fatalf("OpenOrder IncludeOvernight = %v, want omitted", event.OpenOrder.Order.IncludeOvernight)
				}
			}
			if event.Status != nil {
				sawStatus = true
				if event.Status.Status != ibkr.OrderStatusPreSubmitted || event.Status.PermID != 900000001 {
					t.Fatalf("OrderStatus = %+v", event.Status)
				}
			}
		case <-ctx.Done():
			t.Fatalf("waiting for live-derived sv203 callbacks: %v", ctx.Err())
		}
	}
	if err := handle.Cancel(ctx); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	for {
		select {
		case event := <-handle.Events():
			if event.Status != nil && ibkr.IsTerminalOrderStatus(event.Status.Status) {
				handle.Close()
				if err := handle.Wait(); err != nil {
					t.Fatalf("Close/Wait() error = %v", err)
				}
				goto canceled
			}
		case <-ctx.Done():
			t.Fatalf("waiting for cancellation: %v", ctx.Err())
		}
	}
canceled:
	if err := client.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("CancelAll() error = %v", err)
	}
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime() after global cancel error = %v", err)
	}
}

func TestOpenOrdersEndServer203Replay(t *testing.T) {
	restore := ibkr.SetAdvertisedServerVersionMaxForTest(203)
	defer restore()

	client, host := newClient(t, "open_orders_empty_sv203_live.txt")
	defer client.Close()
	defer waitHost(t, host)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	orders, err := client.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if len(orders) != 0 {
		t.Fatalf("Open() returned %d orders, want live empty snapshot", len(orders))
	}
}

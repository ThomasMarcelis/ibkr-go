package ibkr_test

import (
	"context"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/shopspring/decimal"
)

// TestAPIDelayedSuccessModifyReplay freezes the sv225 after-hours modify
// lifecycle: a far LMT BUY rests PreSubmitted, then an in-place replacement
// echoes as MKT while remaining held and unfilled. It replays capture
// 20260824T204727Z-api_delayed_success_modify_aapl, events SHA-256
// 4544f9f88050f696dd6098e8bf5c2d9db4a9aa8f8b29512b170bc7fa08b4c741.
func TestAPIDelayedSuccessModifyReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_delayed_success_modify_aapl.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	replayDelayedAAPLQuoteAnchor(t, ctx, client)

	order := ibkr.Order{
		Action:    ibkr.ActionBuy,
		OrderType: ibkr.OrderTypeLimit,
		Quantity:  decimal.NewFromInt(1),
		LmtPrice:  new(decimal.NewFromInt(10)),
		TIF:       ibkr.TIFDay,
		Account:   "DU9000001",
		OrderRef:  "sanitized-order-ref-0000000000000001",
	}
	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: orderReplayAAPL, Order: order})
	if err != nil {
		t.Fatalf("Place: %v", err)
	}
	if handle.OrderID() != 494 {
		t.Fatalf("OrderID() = %d, want 494", handle.OrderID())
	}
	open := waitForOpenOrder(t, ctx, handle)
	if open.Order.OrderType != ibkr.OrderTypeLimit || !open.Order.Prices.LmtPrice.Equal(decimal.NewFromInt(10)) {
		t.Fatalf("resting open order = %s @ %s, want LMT @ 10", open.Order.OrderType, open.Order.Prices.LmtPrice)
	}
	if *open.Order.PermID != 900000494 {
		t.Fatalf("PermID = %d, want 900000494", *open.Order.PermID)
	}
	waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusPreSubmitted)

	order.OrderType = ibkr.OrderTypeMarket
	order.LmtPrice = nil
	if err := handle.Replace(ctx, order); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	for open.Order.OrderType != ibkr.OrderTypeMarket {
		open = waitForOpenOrder(t, ctx, handle)
	}
	if open.Order.OrderType != ibkr.OrderTypeMarket || !open.Order.Prices.LmtPrice.IsZero() {
		t.Fatalf("modified open order = %s @ %s, want MKT @ 0", open.Order.OrderType, open.Order.Prices.LmtPrice)
	}
	status := waitOrderStatusUpdate(t, ctx, handle, ibkr.OrderStatusPreSubmitted)
	if !status.Filled.IsZero() || !status.Remaining.Equal(decimal.NewFromInt(1)) {
		t.Fatalf("modified status = %+v, want zero-fill PreSubmitted", status)
	}
	handle.Close()
	requireCloseOrCapturedDisconnect(t, "delayed modify", handle.Wait())
}

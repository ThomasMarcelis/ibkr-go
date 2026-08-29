package ibkr_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/shopspring/decimal"
)

// The five replays below freeze the resting-order cancel family (matrix row
// ORD-001 variants) recaptured against paper Gateway server_version 225 on
// 2026-08-24. Each test drives the public API exactly as its transcript's
// client frames show; every asserted value is taken from that capture.
//
// Shared behavior frozen across the family: a safety global cancel against an
// order that is already terminal draws a real code-161 "Cancel attempted when
// order is not in a cancellable state" from the Gateway. The engine routes a
// code-161 carrying a known order id as a session notice; terminal order status
// remains authoritative, so the handle closes cleanly after its drain window.

var orderReplayAAPL = ibkr.Contract{
	ConID:    265598,
	Symbol:   "AAPL",
	SecType:  ibkr.SecTypeStock,
	Exchange: "SMART",
	Currency: "USD",
}

// requireOrderAPIError asserts the handle terminated with an *ibkr.APIError
// of the wanted code on the place-order op whose message carries fragment.
func requireOrderAPIError(t *testing.T, name string, handle *ibkr.OrderHandle, code int, fragment string) {
	t.Helper()

	err := handle.Wait()
	apiErr, ok := errors.AsType[*ibkr.APIError](err)
	if !ok {
		t.Fatalf("%s Wait() = %v, want *ibkr.APIError", name, err)
	}
	if apiErr.Code != code {
		t.Fatalf("%s error code = %d, want %d", name, apiErr.Code, code)
	}
	if apiErr.OpKind != ibkr.OpPlaceOrder {
		t.Fatalf("%s error op kind = %s, want %s", name, apiErr.OpKind, ibkr.OpPlaceOrder)
	}
	if !strings.Contains(apiErr.Message, fragment) {
		t.Fatalf("%s error message = %q, want fragment %q", name, apiErr.Message, fragment)
	}
}

func requireOrderWaitNil(t *testing.T, name string, handle *ibkr.OrderHandle) {
	t.Helper()

	handle.Close()
	requireCloseOrCapturedDisconnect(t, name, handle.Wait())
}

// TestAPIOrderStopCancelReplay freezes the sv225 STP and STP LMT rest/cancel
// lifecycles. Both orders hold at PreSubmitted with why_held=trigger and
// targeted cancellation reaches Cancelled without a fill.
func TestAPIOrderStopCancelReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_order_stop_cancel_aapl.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	events := client.SessionEvents()
	replayDelayedAAPLQuoteAnchor(t, ctx, client)

	stop, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: orderReplayAAPL,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeStop,
			Quantity:  decimal.NewFromInt(1),
			AuxPrice:  new(decimal.NewFromInt(2000)),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "sanitized-order-ref-0000000000000001",
		},
	})
	if err != nil {
		t.Fatalf("stop Place: %v", err)
	}
	if got := stop.OrderID(); got != 515 {
		t.Fatalf("stop order id = %d, want 515", got)
	}

	open := waitForOpenOrder(t, ctx, stop)
	if open.Order.OrderType != ibkr.OrderTypeStop {
		t.Fatalf("stop open order type = %s, want STP", open.Order.OrderType)
	}
	if !open.Order.Prices.AuxPrice.Equal(decimal.NewFromInt(2000)) {
		t.Fatalf("stop aux price = %s, want 2000", open.Order.Prices.AuxPrice)
	}
	if !open.Order.Prices.LmtPrice.IsZero() {
		t.Fatalf("stop echoed lmt price = %s, want zero", open.Order.Prices.LmtPrice)
	}
	if (*open.Order.PermID) != 900000515 {
		t.Fatalf("stop perm id = %d, want 900000515", (*open.Order.PermID))
	}

	preSubmitted := waitOrderStatusUpdate(t, ctx, stop, ibkr.OrderStatusPreSubmitted)
	if preSubmitted.WhyHeld != "trigger" {
		t.Fatalf("stop why held = %q, want trigger", preSubmitted.WhyHeld)
	}

	if err := stop.Cancel(ctx); err != nil {
		t.Fatalf("stop Cancel: %v", err)
	}
	waitForOrderStatus(t, ctx, stop, ibkr.OrderStatusCancelled)
	waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderCanceled)

	stopLimit, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: orderReplayAAPL,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeStopLimit,
			Quantity:  decimal.NewFromInt(1),
			LmtPrice:  new(decimal.NewFromInt(2001)),
			AuxPrice:  new(decimal.NewFromInt(2000)),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "sanitized-order-ref-0000000000000006",
		},
	})
	if err != nil {
		t.Fatalf("stop-limit Place: %v", err)
	}
	if got := stopLimit.OrderID(); got != 516 {
		t.Fatalf("stop-limit order id = %d, want 516", got)
	}

	open = waitForOpenOrder(t, ctx, stopLimit)
	if open.Order.OrderType != ibkr.OrderTypeStopLimit {
		t.Fatalf("stop-limit open order type = %s, want STP LMT", open.Order.OrderType)
	}
	if !open.Order.Prices.LmtPrice.Equal(decimal.NewFromInt(2001)) {
		t.Fatalf("stop-limit lmt price = %s, want 2001", open.Order.Prices.LmtPrice)
	}
	if !open.Order.Prices.AuxPrice.Equal(decimal.NewFromInt(2000)) {
		t.Fatalf("stop-limit aux price = %s, want 2000", open.Order.Prices.AuxPrice)
	}
	if (*open.Order.PermID) != 900000516 {
		t.Fatalf("stop-limit perm id = %d, want 900000516", (*open.Order.PermID))
	}

	preSubmitted = waitOrderStatusUpdate(t, ctx, stopLimit, ibkr.OrderStatusPreSubmitted)
	if preSubmitted.WhyHeld != "trigger" {
		t.Fatalf("stop-limit why held = %q, want trigger", preSubmitted.WhyHeld)
	}

	if err := stopLimit.Cancel(ctx); err != nil {
		t.Fatalf("stop-limit Cancel: %v", err)
	}
	waitForOrderStatus(t, ctx, stopLimit, ibkr.OrderStatusCancelled)
	waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderCanceled)

	requireOrderWaitNil(t, "stop-limit", stopLimit)
	requireOrderWaitNil(t, "stop", stop)
}

// TestAPIOrderTrailingCancelReplay freezes the current sv225 TRAIL and TRAIL
// LIMIT rest/cancel lifecycles from capture
// 20260824T205533Z-api_order_trailing_cancel_aapl, events SHA-256
// 45fe17083f97d726fce85582024104173f40021a6976f759e7456197f13d8be0.
func TestAPIOrderTrailingCancelReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_order_trailing_cancel_aapl.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	replayDelayedAAPLQuoteAnchor(t, ctx, client)

	cases := []struct {
		name     string
		wantID   int64
		orderRef string
		order    ibkr.Order
	}{
		{"trail", 517, "sanitized-order-ref-0000000000000001", ibkr.Order{
			Action: ibkr.ActionSell, OrderType: ibkr.OrderTypeTrailingStop, Quantity: decimal.NewFromInt(1),
			AuxPrice: new(decimal.NewFromInt(1)), TrailStopPrice: new(decimal.NewFromInt(2000)),
			TIF: ibkr.TIFDay, Account: "DU9000001",
		}},
		{"trail-limit", 518, "sanitized-order-ref-0000000000000006", ibkr.Order{
			Action: ibkr.ActionSell, OrderType: ibkr.OrderTypeTrailingLimit, Quantity: decimal.NewFromInt(1),
			AuxPrice: new(decimal.NewFromInt(1)), TrailStopPrice: new(decimal.NewFromInt(2000)),
			LmtPriceOffset: new(decimal.RequireFromString("0.05")), TIF: ibkr.TIFDay, Account: "DU9000001",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.order.OrderRef = tc.orderRef
			handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: orderReplayAAPL, Order: tc.order})
			if err != nil {
				t.Fatalf("Place: %v", err)
			}
			if handle.OrderID() != tc.wantID {
				t.Fatalf("OrderID() = %d, want %d", handle.OrderID(), tc.wantID)
			}
			open := waitForOpenOrder(t, ctx, handle)
			if open.Order.OrderType != tc.order.OrderType || open.Order.Quantity.String() != "1" {
				t.Fatalf("open order = %s qty %s", open.Order.OrderType, open.Order.Quantity)
			}
			waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusPreSubmitted)
			if err := handle.Cancel(ctx); err != nil {
				t.Fatalf("Cancel: %v", err)
			}
			waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusCancelled)
			requireOrderWaitNil(t, tc.name, handle)
		})
	}
}

// TestAPIOrderRelativeCancelReplay freezes the sv225 REL rest/cancel
// lifecycle. The Gateway assigns offset 0.01 to an order placed with only a
// price cap, holds it PreSubmitted after hours, and cancels it without a fill.
func TestAPIOrderRelativeCancelReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_order_relative_cancel_aapl.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	events := client.SessionEvents()
	replayDelayedAAPLQuoteAnchor(t, ctx, client)

	rel, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: orderReplayAAPL,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeRelative,
			Quantity:  decimal.NewFromInt(1),
			LmtPrice:  new(decimal.NewFromInt(10)),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "sanitized-order-ref-0000000000000001",
		},
	})
	if err != nil {
		t.Fatalf("Place: %v", err)
	}
	if got := rel.OrderID(); got != 513 {
		t.Fatalf("order id = %d, want 513", got)
	}

	open := waitForOpenOrder(t, ctx, rel)
	if open.Order.OrderType != ibkr.OrderTypeRelative {
		t.Fatalf("open order type = %s, want REL", open.Order.OrderType)
	}
	if !open.Order.Prices.LmtPrice.Equal(decimal.NewFromInt(10)) {
		t.Fatalf("lmt price = %s, want 10", open.Order.Prices.LmtPrice)
	}
	// The client sent no offset; the live Gateway assigned 0.01.
	if !open.Order.Prices.AuxPrice.Equal(decimal.RequireFromString("0.01")) {
		t.Fatalf("gateway-assigned offset = %s, want 0.01", open.Order.Prices.AuxPrice)
	}
	if (*open.Order.PermID) != 900000513 {
		t.Fatalf("perm id = %d, want 900000513", (*open.Order.PermID))
	}

	waitForOrderStatus(t, ctx, rel, ibkr.OrderStatusPreSubmitted)
	if warning := waitForOrderWarning(t, ctx, rel); warning.Code != 399 {
		t.Fatalf("order warning = %v, want off-hours code 399", warning)
	}

	if err := rel.Cancel(ctx); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	waitForOrderStatus(t, ctx, rel, ibkr.OrderStatusCancelled)
	notice := waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderCanceled)
	if notice.Message != "Order Canceled - reason:" {
		t.Fatalf("202 message = %q", notice.Message)
	}

	requireOrderWaitNil(t, "relative", rel)
}

// TestAPIOrderRejectsReplay freezes the current sv225 order-reject family from
// capture 20260824T205352Z-api_order_rejects_aapl, events SHA-256
// d7b6fe707d3c7c297928e3df02b587b73ddd7c9ed82687be44ca08a293ab3be6: code 321 for a bogus order type, the
// Gateway-initiated price-band cancel with its code-202 reject text, code
// 10148 for cancelling the already-cancelled order, code 200 for an unknown
// symbol, code 10147 for cancelling an unknown order id, and the code-161
// safety-cancel tail.
func TestAPIOrderRejectsReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_order_rejects_aapl.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	replayDelayedAAPLQuoteAnchor(t, ctx, client)

	invalid, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: orderReplayAAPL, Order: ibkr.Order{
		Action: ibkr.ActionBuy, OrderType: ibkr.OrderType("FEELINGS"), Quantity: decimal.NewFromInt(1),
		LmtPrice: new(decimal.NewFromInt(10)), TIF: ibkr.TIFDay, Account: "DU9000001",
		OrderRef: "sanitized-order-ref-0000000000000001",
	}})
	if err != nil {
		t.Fatalf("invalid order Place: %v", err)
	}
	if invalid.OrderID() != 510 {
		t.Fatalf("invalid order ID = %d, want 510", invalid.OrderID())
	}
	requireOrderAPIError(t, "invalid order type", invalid, ibkr.ErrCodeServerErrorValidatingRequest, "Invalid order type")

	priceBand, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: orderReplayAAPL, Order: ibkr.Order{
		Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeLimit, Quantity: decimal.NewFromInt(1),
		LmtPrice: new(decimal.NewFromInt(2000)), TIF: ibkr.TIFDay, Account: "DU9000001",
		OrderRef: "sanitized-order-ref-0000000000000003",
	}})
	if err != nil {
		t.Fatalf("price-band Place: %v", err)
	}
	if priceBand.OrderID() != 511 {
		t.Fatalf("price-band order ID = %d, want 511", priceBand.OrderID())
	}
	open := waitForOpenOrder(t, ctx, priceBand)
	if !open.Order.Prices.LmtPrice.Equal(decimal.NewFromInt(2000)) {
		t.Fatalf("price-band echoed limit = %s, want 2000", open.Order.Prices.LmtPrice)
	}
	waitForOrderStatus(t, ctx, priceBand, ibkr.OrderStatusPreSubmitted)
	if err := priceBand.Cancel(ctx); err != nil {
		t.Fatalf("price-band Cancel: %v", err)
	}
	waitForOrderStatus(t, ctx, priceBand, ibkr.OrderStatusCancelled)
	requireOrderWaitNil(t, "price-band", priceBand)

	unknown, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{Symbol: "ZZZZNONE", SecType: ibkr.SecTypeStock, Exchange: "SMART", Currency: "USD"},
		Order: ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeMarket, Quantity: decimal.NewFromInt(1),
			TIF: ibkr.TIFDay, Account: "DU9000001", OrderRef: "sanitized-order-ref-0000000000000009"},
	})
	if err != nil {
		t.Fatalf("invalid-contract Place: %v", err)
	}
	if unknown.OrderID() != 512 {
		t.Fatalf("invalid-contract order ID = %d, want 512", unknown.OrderID())
	}
	requireOrderAPIError(t, "invalid contract", unknown, ibkr.ErrCodeNoSecurityDefinition, "No security definition")
}

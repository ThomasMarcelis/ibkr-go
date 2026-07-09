package ibkr_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
	"github.com/shopspring/decimal"
)

// The five replays below freeze the resting-order cancel family (matrix row
// ORD-001 variants) captured live on 2026-06-10 against paper Gateway
// server_version 200 with the 5-field cancel encoder. Each test drives the
// public API exactly as the capture's client frames show; every asserted
// value is taken from the capture.
//
// Shared behavior frozen across the family: a safety global cancel against an
// order that is already terminal draws a real code-161 "Cancel attempted when
// order is not in a cancellable state" from the Gateway. The engine routes a
// code-161 carrying a known order id to that order's route; in replay the 161
// arrives inside the post-terminal drain window, so the handle closes with
// the code-161 *ibkr.APIError instead of the nil terminal close.

var orderReplayAAPL = ibkr.Contract{
	ConID:    265598,
	Symbol:   "AAPL",
	SecType:  ibkr.SecTypeStock,
	Exchange: "SMART",
	Currency: "USD",
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

// TestAPIOrderRestCancelReplay freezes the resting-order cancel baseline
// re-captured live on 2026-06-10 (captures/20260610T195745Z-
// api_order_rest_cancel_aapl, events.jsonl sha256 prefix cab24496228ff1fb):
// a far LMT BUY rests at Submitted, the API cancel yields order_status
// Cancelled plus the code-202 session notice, and the safety global cancel
// draws a real code 161 that closes the handle as its terminal error.
func TestAPIOrderRestCancelReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_order_rest_cancel_161_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	events := client.SessionEvents()

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: orderReplayAAPL,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("100"),
			LmtPrice:  decimal.RequireFromString("14.61"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "ibkrgo-sanitized-20260610T195745Z-001",
		},
	})
	if err != nil {
		t.Fatalf("Place: %v", err)
	}
	if got := handle.OrderID(); got != 337 {
		t.Fatalf("order id = %d, want 337", got)
	}

	// Modify must refuse a what-if flag client-side, before any wire I/O —
	// the companion contract to Place's rejection (use Orders().Preview).
	var whatIfErr *ibkr.ValidationError
	if err := handle.Modify(ctx, ibkr.Order{
		Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeLimit,
		Quantity: decimal.RequireFromString("100"),
		LmtPrice: decimal.RequireFromString("14.61"),
		TIF:      ibkr.TIFDay, WhatIf: new(true),
	}); !errors.As(err, &whatIfErr) {
		t.Fatalf("Modify(WhatIf) error = %v, want *ValidationError", err)
	}

	open := waitForOpenOrder(t, ctx, handle)
	if open.OrderID != 337 || open.PermID != 900337 {
		t.Fatalf("open order id/perm = %d/%d, want 337/900337", open.OrderID, open.PermID)
	}
	if open.OrderType != ibkr.OrderTypeLimit {
		t.Fatalf("open order type = %s, want LMT", open.OrderType)
	}
	if !open.LmtPrice.Equal(decimal.RequireFromString("14.61")) {
		t.Fatalf("open lmt price = %s, want 14.61", open.LmtPrice)
	}
	if open.OrderRef != "ibkrgo-sanitized-20260610T195745Z-001" {
		t.Fatalf("open order ref = %q", open.OrderRef)
	}

	waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusSubmitted)

	if err := handle.Cancel(ctx); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusCancelled)

	// The code-202 cancellation notice is a session event, not a handle
	// error (the handle already holds the terminal Cancelled status).
	notice := waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderCanceled)
	if notice.Message != "Order Canceled - reason:" {
		t.Fatalf("202 message = %q", notice.Message)
	}

	// Safety re-cancel: the live Gateway answered the global cancel with
	// code 161 for the already-cancelled order; it lands on the open order
	// route and becomes the handle's terminal error.
	if err := client.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("CancelAll: %v", err)
	}
	requireOrderAPIError(t, "rest", handle, ibkr.ErrCodeCancelNotCancellableState,
		"Cancel attempted when order is not in a cancellable state.  Order permId =900337")
}

// TestAPIOrderStopCancelReplay freezes the STP and STP LMT rest/cancel
// lifecycles captured live on 2026-06-10 (captures/20260610T195758Z-
// api_order_stop_cancel_aapl, events.jsonl sha256 prefix fb50c6d6a49dc509).
// Both stops hold at PreSubmitted with why_held=trigger; the plain STP echo
// carries the Gateway-computed limit 2921.23 next to the 2921.2 stop. The
// final global cancel draws code 161 for both already-cancelled orders.
func TestAPIOrderStopCancelReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_order_stop_cancel_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	events := client.SessionEvents()

	stop, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: orderReplayAAPL,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeStop,
			Quantity:  decimal.RequireFromString("100"),
			AuxPrice:  decimal.RequireFromString("2921.2"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "ibkrgo-sanitized-20260610T195758Z-001",
		},
	})
	if err != nil {
		t.Fatalf("stop Place: %v", err)
	}
	if got := stop.OrderID(); got != 338 {
		t.Fatalf("stop order id = %d, want 338", got)
	}

	open := waitForOpenOrder(t, ctx, stop)
	if open.OrderType != ibkr.OrderTypeStop {
		t.Fatalf("stop open order type = %s, want STP", open.OrderType)
	}
	if !open.AuxPrice.Equal(decimal.RequireFromString("2921.2")) {
		t.Fatalf("stop aux price = %s, want 2921.2", open.AuxPrice)
	}
	// The live Gateway echoed a computed limit price next to the stop.
	if !open.LmtPrice.Equal(decimal.RequireFromString("2921.23")) {
		t.Fatalf("stop echoed lmt price = %s, want 2921.23", open.LmtPrice)
	}
	if open.PermID != 900338 {
		t.Fatalf("stop perm id = %d, want 900338", open.PermID)
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
			Quantity:  decimal.RequireFromString("100"),
			LmtPrice:  decimal.RequireFromString("2922.2"),
			AuxPrice:  decimal.RequireFromString("2921.2"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "ibkrgo-sanitized-20260610T195758Z-002",
		},
	})
	if err != nil {
		t.Fatalf("stop-limit Place: %v", err)
	}
	if got := stopLimit.OrderID(); got != 339 {
		t.Fatalf("stop-limit order id = %d, want 339", got)
	}

	open = waitForOpenOrder(t, ctx, stopLimit)
	if open.OrderType != ibkr.OrderTypeStopLimit {
		t.Fatalf("stop-limit open order type = %s, want STP LMT", open.OrderType)
	}
	if !open.LmtPrice.Equal(decimal.RequireFromString("2922.2")) {
		t.Fatalf("stop-limit lmt price = %s, want 2922.2", open.LmtPrice)
	}
	if !open.AuxPrice.Equal(decimal.RequireFromString("2921.2")) {
		t.Fatalf("stop-limit aux price = %s, want 2921.2", open.AuxPrice)
	}
	if open.PermID != 900339 {
		t.Fatalf("stop-limit perm id = %d, want 900339", open.PermID)
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

	// The live Gateway answered the final global cancel with code 161 for
	// both terminal orders, 339 first.
	if err := client.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("CancelAll: %v", err)
	}
	requireOrderAPIError(t, "stop-limit", stopLimit, ibkr.ErrCodeCancelNotCancellableState, "Order permId =900339")
	requireOrderAPIError(t, "stop", stop, ibkr.ErrCodeCancelNotCancellableState, "Order permId =900338")
}

// TestAPIOrderTrailingCancelReplay freezes the TRAIL and TRAIL LIMIT
// lifecycles captured live on 2026-06-10 (captures/20260610T195819Z-
// api_order_trailing_cancel_aapl, events.jsonl sha256 prefix
// 0d3098f03fd68839). The TRAIL SELL triggered off-hours and filled in two
// executions (80+20 @ 292.14); the cancel raced the fill and drew code 10148
// "cannot be cancelled, state: Filled.". The capture's execution_data frames
// carry the Gateway's UTC dash time form ("20260610-19:58:22"), which this
// client's execution time parser does not accept: the executions and their
// commission reports are dropped (live and in replay), so the fill is
// asserted through order_status only and the handle sees zero
// Execution/Commission events. The TRAIL LIMIT rests and cancels cleanly.
func TestAPIOrderTrailingCancelReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_order_trailing_cancel_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	events := client.SessionEvents()

	trail, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: orderReplayAAPL,
		Order: ibkr.Order{
			Action:         ibkr.ActionSell,
			OrderType:      ibkr.OrderTypeTrailingStop,
			Quantity:       decimal.RequireFromString("100"),
			AuxPrice:       decimal.RequireFromString("1"),
			TrailStopPrice: decimal.RequireFromString("2921"),
			TIF:            ibkr.TIFDay,
			Account:        "DU9000001",
			OrderRef:       "ibkrgo-sanitized-20260610T195819Z-001",
		},
	})
	if err != nil {
		t.Fatalf("trail Place: %v", err)
	}
	if got := trail.OrderID(); got != 340 {
		t.Fatalf("trail order id = %d, want 340", got)
	}

	open := waitForOpenOrder(t, ctx, trail)
	if open.OrderType != ibkr.OrderTypeTrailingStop {
		t.Fatalf("trail open order type = %s, want TRAIL", open.OrderType)
	}
	// First echo carries the Gateway-computed trigger limit 2921.03 next to
	// the trailing amount 1.
	if !open.LmtPrice.Equal(decimal.RequireFromString("2921.03")) {
		t.Fatalf("trail echoed lmt price = %s, want 2921.03", open.LmtPrice)
	}
	if !open.AuxPrice.Equal(decimal.RequireFromString("1")) {
		t.Fatalf("trail aux price = %s, want 1", open.AuxPrice)
	}
	if open.PermID != 900340 {
		t.Fatalf("trail perm id = %d, want 900340", open.PermID)
	}

	// Drain to Filled, tracking the partial fill (80/20 @ 292.14) and
	// counting Execution/Commission events, which must stay at zero because
	// the dash-UTC execution time is dropped at decode.
	var executions, commissions int
	var sawPartial, sawTrigger bool
	for {
		var filled *ibkr.OrderStatusUpdate
		select {
		case evt, ok := <-trail.Events():
			if !ok {
				t.Fatal("trail events closed before Filled status")
			}
			if evt.Execution != nil {
				executions++
			}
			if evt.CommissionAndFees != nil {
				commissions++
			}
			if evt.Status != nil {
				switch {
				case evt.Status.Status == ibkr.OrderStatusPreSubmitted && evt.Status.WhyHeld == "trigger":
					sawTrigger = true
					if evt.Status.Filled.Equal(decimal.RequireFromString("80")) &&
						evt.Status.Remaining.Equal(decimal.RequireFromString("20")) &&
						evt.Status.AvgFillPrice.Equal(decimal.RequireFromString("292.14")) {
						sawPartial = true
					}
				case evt.Status.Status == ibkr.OrderStatusFilled:
					filled = evt.Status
				}
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for trail Filled status")
		}
		if filled != nil {
			if !filled.Filled.Equal(decimal.RequireFromString("100")) ||
				!filled.Remaining.IsZero() ||
				!filled.AvgFillPrice.Equal(decimal.RequireFromString("292.14")) ||
				!filled.LastFillPrice.Equal(decimal.RequireFromString("292.14")) {
				t.Fatalf("trail filled status = %+v, want 100 @ 292.14", filled)
			}
			break
		}
	}
	if !sawTrigger {
		t.Fatal("trail never reported PreSubmitted with why_held=trigger")
	}
	if !sawPartial {
		t.Fatal("trail never reported the live partial fill 80/20 @ 292.14")
	}

	// The cancel raced the fill live; the Gateway rejects it with code
	// 10148, surfaced as a session event (the handle holds Filled).
	if err := trail.Cancel(ctx); err != nil {
		t.Fatalf("trail Cancel: %v", err)
	}
	cannotCancel := waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderCannotBeCancelled)
	if !strings.Contains(cannotCancel.Message, "cannot be cancelled, state: Filled.") {
		t.Fatalf("10148 message = %q", cannotCancel.Message)
	}

	trailLimit, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: orderReplayAAPL,
		Order: ibkr.Order{
			Action:         ibkr.ActionSell,
			OrderType:      ibkr.OrderTypeTrailingLimit,
			Quantity:       decimal.RequireFromString("100"),
			AuxPrice:       decimal.RequireFromString("1"),
			TrailStopPrice: decimal.RequireFromString("2921"),
			Adjustment:     ibkr.OrderAdjustment{LmtPriceOffset: decimal.RequireFromString("0.05")},
			TIF:            ibkr.TIFDay,
			Account:        "DU9000001",
			OrderRef:       "ibkrgo-sanitized-20260610T195819Z-002",
		},
	})
	if err != nil {
		t.Fatalf("trail-limit Place: %v", err)
	}
	if got := trailLimit.OrderID(); got != 341 {
		t.Fatalf("trail-limit order id = %d, want 341", got)
	}

	open = waitForOpenOrder(t, ctx, trailLimit)
	if open.OrderType != ibkr.OrderTypeTrailingLimit {
		t.Fatalf("trail-limit open order type = %s, want TRAIL LIMIT", open.OrderType)
	}
	// The Gateway computed limit 2920.95 from trail stop 2921 minus the
	// 0.05 offset.
	if !open.LmtPrice.Equal(decimal.RequireFromString("2920.95")) {
		t.Fatalf("trail-limit echoed lmt price = %s, want 2920.95", open.LmtPrice)
	}
	if open.PermID != 900341 {
		t.Fatalf("trail-limit perm id = %d, want 900341", open.PermID)
	}

	preSubmitted := waitOrderStatusUpdate(t, ctx, trailLimit, ibkr.OrderStatusPreSubmitted)
	if preSubmitted.WhyHeld != "trigger" {
		t.Fatalf("trail-limit why held = %q, want trigger", preSubmitted.WhyHeld)
	}
	waitForOrderStatus(t, ctx, trailLimit, ibkr.OrderStatusSubmitted)

	if err := trailLimit.Cancel(ctx); err != nil {
		t.Fatalf("trail-limit Cancel: %v", err)
	}
	waitForOrderStatus(t, ctx, trailLimit, ibkr.OrderStatusCancelled)
	waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderCanceled)

	// Final safety global cancel: code 161 for both terminal orders.
	if err := client.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("CancelAll: %v", err)
	}

	// Drain the trail handle to its close, counting execution and commission
	// events. The capture's execution times use the Gateway's UTC dash
	// notation; the parser accepts that form, so both fills (80 and 20 at
	// 292.14) and both commission reports must reach the handle.
	for evt := range trail.Events() {
		if evt.Execution != nil {
			executions++
			if got := evt.Execution.Shares.String(); got != "80" && got != "20" {
				t.Errorf("execution shares = %s, want 80 or 20", got)
			}
			if got := evt.Execution.Price.String(); got != "292.14" {
				t.Errorf("execution price = %s, want 292.14", got)
			}
		}
		if evt.CommissionAndFees != nil {
			commissions++
		}
	}
	if executions != 2 || commissions != 2 {
		t.Fatalf("trail surfaced %d executions and %d commissions, want 2/2", executions, commissions)
	}
	requireOrderAPIError(t, "trail", trail, ibkr.ErrCodeCancelNotCancellableState, "Order permId =900340")
	requireOrderAPIError(t, "trail-limit", trailLimit, ibkr.ErrCodeCancelNotCancellableState, "Order permId =900341")
}

// TestAPIOrderRelativeCancelReplay freezes the REL rest/cancel lifecycle
// captured live on 2026-06-10 (captures/20260610T195833Z-
// api_order_relative_cancel_aapl, events.jsonl sha256 prefix
// 65c28d7faea45243): the Gateway assigns offset 0.01 to a REL order placed
// with only a price cap, the order moves PreSubmitted to Submitted, cancels
// with Cancelled + 202, and the final global cancel draws code 161 (plus a
// second 161 for a previous scenario's order that this client drops).
func TestAPIOrderRelativeCancelReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_order_relative_cancel_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	events := client.SessionEvents()

	rel, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: orderReplayAAPL,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeRelative,
			Quantity:  decimal.RequireFromString("100"),
			LmtPrice:  decimal.RequireFromString("14.61"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "ibkrgo-sanitized-20260610T195833Z-001",
		},
	})
	if err != nil {
		t.Fatalf("Place: %v", err)
	}
	if got := rel.OrderID(); got != 342 {
		t.Fatalf("order id = %d, want 342", got)
	}

	open := waitForOpenOrder(t, ctx, rel)
	if open.OrderType != ibkr.OrderTypeRelative {
		t.Fatalf("open order type = %s, want REL", open.OrderType)
	}
	if !open.LmtPrice.Equal(decimal.RequireFromString("14.61")) {
		t.Fatalf("lmt price = %s, want 14.61", open.LmtPrice)
	}
	// The client sent no offset; the live Gateway assigned 0.01.
	if !open.AuxPrice.Equal(decimal.RequireFromString("0.01")) {
		t.Fatalf("gateway-assigned offset = %s, want 0.01", open.AuxPrice)
	}
	if open.PermID != 900342 {
		t.Fatalf("perm id = %d, want 900342", open.PermID)
	}

	waitForOrderStatus(t, ctx, rel, ibkr.OrderStatusPreSubmitted)
	waitForOrderStatus(t, ctx, rel, ibkr.OrderStatusSubmitted)

	if err := rel.Cancel(ctx); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	waitForOrderStatus(t, ctx, rel, ibkr.OrderStatusCancelled)
	notice := waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderCanceled)
	if notice.Message != "Order Canceled - reason:" {
		t.Fatalf("202 message = %q", notice.Message)
	}

	// The replayed global-cancel response also carries a 161 for the
	// previous scenario's order 340; this client has no route for it and
	// drops it, which the replay completing proves (waitHost).
	if err := client.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("CancelAll: %v", err)
	}
	requireOrderAPIError(t, "relative", rel, ibkr.ErrCodeCancelNotCancellableState, "Order permId =900342")
}

// TestAPIOrderRejectsReplay freezes the order-reject family captured live on
// 2026-06-10 (captures/20260610T195923Z-api_order_rejects_aapl, events.jsonl
// sha256 prefix 69d0e71dadb875c3): code 321 for a bogus order type, the
// Gateway-initiated price-band cancel with its code-202 reject text, code
// 10148 for cancelling the already-cancelled order, code 200 for an unknown
// symbol, code 10147 for cancelling an unknown order id, and the code-161
// safety-cancel tail.
func TestAPIOrderRejectsReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_order_rejects_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	events := client.SessionEvents()

	// Bogus order type: rejected outright with code 321, no order_status.
	bogus, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: orderReplayAAPL,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderType("FEELINGS"),
			Quantity:  decimal.RequireFromString("100"),
			LmtPrice:  decimal.RequireFromString("14.61"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "ibkrgo-sanitized-20260610T195923Z-001",
		},
	})
	if err != nil {
		t.Fatalf("bogus Place: %v", err)
	}
	if got := bogus.OrderID(); got != 347 {
		t.Fatalf("bogus order id = %d, want 347", got)
	}
	requireOrderAPIError(t, "bogus", bogus, ibkr.ErrCodeServerErrorValidatingRequest,
		"Invalid order type was entered")

	// Aggressive limit at 10x the market: accepted to PreSubmitted, then
	// price-band cancelled by the Gateway itself with the reject text on
	// the code-202 notice.
	aggressive, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: orderReplayAAPL,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("100"),
			LmtPrice:  decimal.RequireFromString("2922.3"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "ibkrgo-sanitized-20260610T195923Z-002",
		},
	})
	if err != nil {
		t.Fatalf("aggressive Place: %v", err)
	}
	if got := aggressive.OrderID(); got != 348 {
		t.Fatalf("aggressive order id = %d, want 348", got)
	}

	open := waitForOpenOrder(t, ctx, aggressive)
	if open.PermID != 900348 {
		t.Fatalf("aggressive perm id = %d, want 900348", open.PermID)
	}
	if !open.LmtPrice.Equal(decimal.RequireFromString("2922.3")) {
		t.Fatalf("aggressive lmt price = %s, want 2922.3", open.LmtPrice)
	}
	waitForOrderStatus(t, ctx, aggressive, ibkr.OrderStatusPreSubmitted)
	waitForOrderStatus(t, ctx, aggressive, ibkr.OrderStatusPendingCancel)
	waitForOrderStatus(t, ctx, aggressive, ibkr.OrderStatusCancelled)
	notice := waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderCanceled)
	if !strings.Contains(notice.Message, "We cannot accept an order at a limit price at or more aggressive than 300.934796") {
		t.Fatalf("price-band 202 message = %q", notice.Message)
	}

	// Cancelling the already-cancelled order draws code 10148 as a session
	// event.
	if err := aggressive.Cancel(ctx); err != nil {
		t.Fatalf("aggressive Cancel: %v", err)
	}
	cannotCancel := waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderCannotBeCancelled)
	if !strings.Contains(cannotCancel.Message, "OrderId 348 that needs to be cancelled cannot be cancelled, state: Cancelled.") {
		t.Fatalf("10148 message = %q", cannotCancel.Message)
	}

	// Unknown symbol: code 200, terminal handle error.
	unknown, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			Symbol:   "ZZZZNONE",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("100"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "ibkrgo-sanitized-20260610T195923Z-003",
		},
	})
	if err != nil {
		t.Fatalf("unknown Place: %v", err)
	}
	if got := unknown.OrderID(); got != 349 {
		t.Fatalf("unknown order id = %d, want 349", got)
	}
	requireOrderAPIError(t, "unknown", unknown, ibkr.ErrCodeNoSecurityDefinition,
		"No security definition has been found for the request")

	// Cancelling an unknown order id draws code 10147 as a session event.
	if err := client.Orders().Cancel(ctx, 999999999); err != nil {
		t.Fatalf("Cancel(999999999): %v", err)
	}
	notFound := waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderToCancelNotFound)
	if !strings.Contains(notFound.Message, "OrderId 999999999 that needs to be cancelled is not found.") {
		t.Fatalf("10147 message = %q", notFound.Message)
	}

	// Safety global cancel: code 161 for order 348 (the 161 for a previous
	// scenario's order 346 has no route here and is dropped).
	if err := client.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("CancelAll: %v", err)
	}
	requireOrderAPIError(t, "aggressive", aggressive, ibkr.ErrCodeCancelNotCancellableState, "Order permId =900348")
}

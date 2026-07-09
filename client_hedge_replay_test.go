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

// TestAPIHedgeOrderReplay freezes the hedge-order rule matrix (AORD-006)
// captured live on 2026-06-11 against paper Gateway server_version 200
// (captures/20260611T134021Z-api_hedge_order_aapl, events.jsonl sha256
// prefix 96145b1775c02629). Five live-attested rules, each a subtest:
//
//   - delta_hedge_compliant: a zero-size stock delta child (HedgeType D,
//     HedgeParam 0.5) of an OPTION parent still draws code 320 "Invalid
//     hedge ratio" and the placement dies outright;
//   - delta_hedge_stock_parent: the same delta child under a STOCK parent
//     draws the rule by name, code 320 "Invalid delta hedge order. The
//     parent order has to be option order ...";
//   - beta_hedge_zero_size: a zero-quantity beta child (B, 1.0) is accepted,
//     the Gateway assigns quantity 100 itself, binds the child to the
//     parent's perm id via oca_group, holds it PreSubmitted with
//     why_held=child, and cancels it cleanly (Cancelled + code 202);
//   - fx_hedge: code 10063 "Invalid FX hedge order..." rejects the placement
//     outright — the Gateway never sends anything else for the id, so the
//     attested placement-rejection set (isOrderPlacementRejection) closes
//     the child handle with the code-10063 APIError as its terminal error;
//   - pair_hedge_zero_size: a zero-quantity pair child (P, 0.8) is accepted
//     with Gateway-computed quantity 80 = 0.8 x the parent's 100.
//
// The sibling capture 20260611T133853Z (sha256 prefix 833e9477724d4c8f)
// attested the zero-size rule itself: sized beta/FX/pair children drew code
// 10032 "Specifying size for hedge order is not allowed, send zero." That
// session is not replayed here, so 10032 stays unregistered until a
// transcript attests it.
//
// HedgeType/HedgeParam ride the client place_order frames and are pinned by
// the transcript; zero quantity encodes as an empty total_quantity field.
func TestAPIHedgeOrderReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_hedge_order_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	events := client.SessionEvents()

	// Option parent: far LMT BUY 1 of the AAPL Jun-12 292.5 call. The live
	// Gateway sent no echo for it until its cancel was processed much later
	// in the session, so nothing is awaited here.
	optionParent, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: exerciseAAPLJun12Call2925,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.NewFromInt(1),
			LmtPrice:  decimal.RequireFromString("14.61"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "ibkrgo-sanitized-20260611T134021Z-001",
		},
	})
	if err != nil {
		t.Fatalf("option parent Place: %v", err)
	}
	if got := optionParent.OrderID(); got != 418 {
		t.Fatalf("option parent order id = %d, want 418", got)
	}

	hedgeChild := func(orderRef string, quantity decimal.Decimal, parentID int64, hedgeType, hedgeParam string) ibkr.PlaceOrderRequest {
		return ibkr.PlaceOrderRequest{
			Contract: orderReplayAAPL,
			Order: ibkr.Order{
				Action:    ibkr.ActionSell,
				OrderType: ibkr.OrderTypeLimit,
				Quantity:  quantity,
				LmtPrice:  decimal.RequireFromString("2922.4"),
				TIF:       ibkr.TIFDay,
				Account:   "DU9000001",
				OrderRef:  orderRef,
				ParentID:  parentID,
				Hedge:     ibkr.OrderHedge{Type: ibkr.HedgeType(hedgeType), Param: hedgeParam},
			},
		}
	}

	t.Run("delta_hedge_compliant", func(t *testing.T) {
		child, err := client.Orders().Place(ctx,
			hedgeChild("ibkrgo-sanitized-20260611T134021Z-002", decimal.Zero, optionParent.OrderID(), "D", "0.5"))
		if err != nil {
			t.Fatalf("Place: %v", err)
		}
		if got := child.OrderID(); got != 419 {
			t.Fatalf("order id = %d, want 419", got)
		}
		requireOrderAPIError(t, "delta compliant", child, ibkr.ErrCodeServerErrorReadingRequest,
			"Invalid hedge ratio")

		// Cancelling the rejected child draws code 10147 as a session event.
		if err := child.Cancel(ctx); err != nil {
			t.Fatalf("Cancel: %v", err)
		}
		notFound := waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderToCancelNotFound)
		if !strings.Contains(notFound.Message, "OrderId 419 that needs to be cancelled is not found.") {
			t.Fatalf("10147 message = %q", notFound.Message)
		}
	})

	// Cancel the option parent; the live Gateway acknowledged it only after
	// the fx hedge placement below.
	if err := optionParent.Cancel(ctx); err != nil {
		t.Fatalf("option parent Cancel: %v", err)
	}

	// Stock parent: far LMT BUY 100 AAPL. Its first echo arrives after the
	// beta child is placed.
	stockParent, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: orderReplayAAPL,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("100"),
			LmtPrice:  decimal.RequireFromString("14.61"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "ibkrgo-sanitized-20260611T134021Z-003",
		},
	})
	if err != nil {
		t.Fatalf("stock parent Place: %v", err)
	}
	if got := stockParent.OrderID(); got != 420 {
		t.Fatalf("stock parent order id = %d, want 420", got)
	}

	t.Run("delta_hedge_stock_parent", func(t *testing.T) {
		child, err := client.Orders().Place(ctx,
			hedgeChild("ibkrgo-sanitized-20260611T134021Z-004", decimal.RequireFromString("100"), stockParent.OrderID(), "D", "0.5"))
		if err != nil {
			t.Fatalf("Place: %v", err)
		}
		if got := child.OrderID(); got != 421 {
			t.Fatalf("order id = %d, want 421", got)
		}
		requireOrderAPIError(t, "delta stock parent", child, ibkr.ErrCodeServerErrorReadingRequest,
			"Invalid delta hedge order. The parent order has to be option order")

		if err := child.Cancel(ctx); err != nil {
			t.Fatalf("Cancel: %v", err)
		}
		notFound := waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderToCancelNotFound)
		if !strings.Contains(notFound.Message, "OrderId 421 that needs to be cancelled is not found.") {
			t.Fatalf("10147 message = %q", notFound.Message)
		}
	})

	var beta *ibkr.OrderHandle
	t.Run("beta_hedge_zero_size", func(t *testing.T) {
		var err error
		beta, err = client.Orders().Place(ctx,
			hedgeChild("ibkrgo-sanitized-20260611T134021Z-005", decimal.Zero, stockParent.OrderID(), "B", "1.0"))
		if err != nil {
			t.Fatalf("Place: %v", err)
		}
		if got := beta.OrderID(); got != 422 {
			t.Fatalf("order id = %d, want 422", got)
		}

		// The stock parent's first echo lands now.
		parentOpen := waitForOpenOrder(t, ctx, stockParent)
		if parentOpen.PermID != 900420 {
			t.Fatalf("stock parent perm id = %d, want 900420", parentOpen.PermID)
		}
		waitForOrderStatus(t, ctx, stockParent, ibkr.OrderStatusSubmitted)

		// The zero-size beta child is accepted: the Gateway assigned
		// quantity 100, floored the limit to 0.01, and bound the child to
		// the parent's perm id via oca_group.
		open := waitForOpenOrder(t, ctx, beta)
		if !open.Quantity.Equal(decimal.RequireFromString("100")) {
			t.Fatalf("gateway-assigned quantity = %s, want 100", open.Quantity)
		}
		if !open.LmtPrice.Equal(decimal.RequireFromString("0.01")) {
			t.Fatalf("gateway-floored lmt price = %s, want 0.01", open.LmtPrice)
		}
		if open.OcaGroup != "900420" {
			t.Fatalf("oca group = %q, want parent perm id 900420", open.OcaGroup)
		}
		if open.ParentID != 420 || open.PermID != 900422 {
			t.Fatalf("parent/perm = %d/%d, want 420/900422", open.ParentID, open.PermID)
		}
		held := waitOrderStatusUpdate(t, ctx, beta, ibkr.OrderStatusPreSubmitted)
		if held.WhyHeld != "child" {
			t.Fatalf("why held = %q, want child", held.WhyHeld)
		}
		if !held.Remaining.Equal(decimal.RequireFromString("100")) {
			t.Fatalf("remaining = %s, want 100", held.Remaining)
		}

		// Cancel now; the Gateway acknowledged it only after the fx hedge
		// placement, asserted in the fx subtest.
		if err := beta.Cancel(ctx); err != nil {
			t.Fatalf("Cancel: %v", err)
		}
	})
	if beta == nil {
		t.Fatal("beta subtest did not produce a handle")
	}

	var fx *ibkr.OrderHandle
	t.Run("fx_hedge", func(t *testing.T) {
		var err error
		fx, err = client.Orders().Place(ctx,
			hedgeChild("ibkrgo-sanitized-20260611T134021Z-006", decimal.Zero, stockParent.OrderID(), "F", ""))
		if err != nil {
			t.Fatalf("Place: %v", err)
		}
		if got := fx.OrderID(); got != 423 {
			t.Fatalf("order id = %d, want 423", got)
		}

		// Code 10063 is an attested outright placement rejection: no
		// order_status ever follows it, so it closes the child handle as
		// the terminal place error instead of riding the 10xxx
		// session-event fallback.
		requireOrderAPIError(t, "fx child", fx, ibkr.ErrCodeInvalidFXHedgeOrder,
			"Invalid FX hedge order. Hedging contract can only be a currency pair where one of the currencies is the same as in parent order.")

		// The beta child's cancel acknowledges now: Cancelled + code 202.
		waitForOrderStatus(t, ctx, beta, ibkr.OrderStatusCancelled)
		waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderCanceled)

		// The option parent's cancel from before the stock-parent stage
		// finally processes: PendingCancel -> Cancelled + code 202.
		parentOpen := waitForOpenOrder(t, ctx, optionParent)
		if parentOpen.PermID != 900418 || parentOpen.Contract.ConID != 886441502 {
			t.Fatalf("option parent echo = perm %d con %d, want 900418/886441502", parentOpen.PermID, parentOpen.Contract.ConID)
		}
		waitForOrderStatus(t, ctx, optionParent, ibkr.OrderStatusPendingCancel)
		waitForOrderStatus(t, ctx, optionParent, ibkr.OrderStatusCancelled)
		waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderCanceled)

		// Cancelling the rejected fx child draws code 10147 as a session
		// event, like the rejected delta children.
		if err := fx.Cancel(ctx); err != nil {
			t.Fatalf("Cancel: %v", err)
		}
		notFound := waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderToCancelNotFound)
		if !strings.Contains(notFound.Message, "OrderId 423 that needs to be cancelled is not found.") {
			t.Fatalf("10147 message = %q", notFound.Message)
		}
	})
	if fx == nil {
		t.Fatal("fx subtest did not produce a handle")
	}

	var pair *ibkr.OrderHandle
	t.Run("pair_hedge_zero_size", func(t *testing.T) {
		var err error
		pair, err = client.Orders().Place(ctx,
			hedgeChild("ibkrgo-sanitized-20260611T134021Z-007", decimal.Zero, stockParent.OrderID(), "P", "0.8"))
		if err != nil {
			t.Fatalf("Place: %v", err)
		}
		if got := pair.OrderID(); got != 424 {
			t.Fatalf("order id = %d, want 424", got)
		}

		// Accepted with the Gateway-computed quantity 80 (= 0.8 x the
		// parent's 100), same oca binding to the parent's perm id.
		open := waitForOpenOrder(t, ctx, pair)
		if !open.Quantity.Equal(decimal.RequireFromString("80")) {
			t.Fatalf("gateway-computed quantity = %s, want 80", open.Quantity)
		}
		if open.OcaGroup != "900420" || open.ParentID != 420 || open.PermID != 900424 {
			t.Fatalf("oca/parent/perm = %q/%d/%d, want 900420/420/900424", open.OcaGroup, open.ParentID, open.PermID)
		}
		held := waitOrderStatusUpdate(t, ctx, pair, ibkr.OrderStatusPreSubmitted)
		if held.WhyHeld != "child" {
			t.Fatalf("why held = %q, want child", held.WhyHeld)
		}
		if !held.Remaining.Equal(decimal.RequireFromString("80")) {
			t.Fatalf("remaining = %s, want 80", held.Remaining)
		}
	})
	if pair == nil {
		t.Fatal("pair subtest did not produce a handle")
	}

	// Teardown as captured: cancel the pair child and the stock parent, then
	// the safety global cancel. The code-161 reply for order 420 raced ahead
	// of its Cancelled status live, so the stock parent's handle terminates
	// with the 161 and the trailing 202 for 420 is dropped (its route closed
	// with the handle); the pair child cancels cleanly with Cancelled + 202.
	if err := pair.Cancel(ctx); err != nil {
		t.Fatalf("pair Cancel: %v", err)
	}
	if err := stockParent.Cancel(ctx); err != nil {
		t.Fatalf("stock parent Cancel: %v", err)
	}
	if err := client.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("CancelAll: %v", err)
	}
	requireOrderAPIError(t, "stock parent", stockParent, ibkr.ErrCodeCancelNotCancellableState,
		"Order permId =900420")
	waitForOrderStatus(t, ctx, pair, ibkr.OrderStatusCancelled)
	waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderCanceled)

	// Closure shapes on the transcript disconnect: handles that reached a
	// terminal status close clean; the fx child closed on its code-10063
	// rejection back in the fx subtest, having emitted no business events.
	if err := pair.Wait(); err != nil {
		t.Fatalf("pair Wait() = %v, want nil (terminal Cancelled)", err)
	}
	if err := optionParent.Wait(); err != nil {
		t.Fatalf("option parent Wait() = %v, want nil (terminal Cancelled)", err)
	}
	requireNoMoreOrderEvents(t, ctx, "fx child", fx)
	if _, ok := errors.AsType[*ibkr.APIError](fx.Wait()); !ok {
		t.Fatalf("fx Wait() = %v, want the terminal code-10063 *ibkr.APIError", fx.Wait())
	}
}

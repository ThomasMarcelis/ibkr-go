package ibkr_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/shopspring/decimal"
)

// TestAPIHedgeOrderReplay freezes the hedge-order rule matrix (AORD-006)
// captured live on 2026-08-24 against paper Gateway server_version 225
// (captures/20260824T210913Z-api_hedge_order_aapl, events.jsonl SHA-256
// 68514f4d1f92b17ca2141c2cf7834387881e889aabf6aaef2ca2adab6591f242).
// Five live-attested rules, each a subtest:
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
// HedgeType/HedgeParam ride the client place_order frames and are pinned by
// the transcript; zero quantity encodes as an empty total_quantity field.
func TestAPIHedgeOrderReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_hedge_order_aapl.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	replayDelayedAAPLQuoteAnchor(t, ctx, client)
	params, err := client.Contracts().SecDefOptParams(ctx, ibkr.SecDefOptParamsRequest{
		UnderlyingSymbol: "AAPL", UnderlyingSecType: ibkr.SecTypeStock, UnderlyingConID: 265598,
	})
	if err != nil || len(params) == 0 {
		t.Fatalf("SecDefOptParams = %d rows, %v", len(params), err)
	}
	details, err := client.Contracts().Details(ctx, ibkr.Contract{
		Symbol: "AAPL", SecType: ibkr.SecTypeOption, Expiry: "20260826", Strike: new(decimal.NewFromInt(310)),
		Right: ibkr.RightCall, Multiplier: "100", Exchange: "SMART", Currency: "USD", TradingClass: "AAPL",
	})
	if err != nil || len(details) != 1 {
		t.Fatalf("option Details = %d rows, %v", len(details), err)
	}

	// Option parent: far LMT BUY 1 of the AAPL Jun-12 292.5 call. The live
	// Gateway sent no echo for it until its cancel was processed much later
	// in the session, so nothing is awaited here.
	optionParent, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: details[0].Contract,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.NewFromInt(1),
			LmtPrice:  new(decimal.RequireFromString("15.53")),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "sanitized-order-ref-0000000000000001",
		},
	})
	if err != nil {
		t.Fatalf("option parent Place: %v", err)
	}
	if got := optionParent.OrderID(); got != 574 {
		t.Fatalf("option parent order id = %d, want 574", got)
	}

	hedgeChild := func(orderRef string, quantity decimal.Decimal, parentID int64, hedgeType, hedgeParam string) ibkr.PlaceOrderRequest {
		return ibkr.PlaceOrderRequest{
			Contract: orderReplayAAPL,
			Order: ibkr.Order{
				Action:    ibkr.ActionSell,
				OrderType: ibkr.OrderTypeLimit,
				Quantity:  quantity,
				LmtPrice:  new(decimal.RequireFromString("3105")),
				TIF:       ibkr.TIFDay,
				Account:   "DU9000001",
				OrderRef:  orderRef,
				ParentID:  parentID,
				Hedge:     ibkr.OrderHedge{Type: ibkr.HedgeType(hedgeType), Param: hedgeParam},
				Transmit:  new(true),
			},
		}
	}

	t.Run("delta_hedge_compliant", func(t *testing.T) {
		child, err := client.Orders().Place(ctx,
			hedgeChild("sanitized-order-ref-0000000000000003", decimal.Zero, optionParent.OrderID(), "D", "0.5"))
		if err != nil {
			t.Fatalf("Place: %v", err)
		}
		if got := child.OrderID(); got != 575 {
			t.Fatalf("order id = %d, want 575", got)
		}
		requireOrderAPIError(t, "delta compliant", child, ibkr.ErrCodeServerErrorReadingRequest,
			"Invalid hedge ratio")
	})

	// Stock parent: far LMT BUY 100 AAPL. Its first echo arrives after the
	// beta child is placed.
	stockParent, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: orderReplayAAPL,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  new(decimal.RequireFromString("15.53")),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "sanitized-order-ref-0000000000000005",
		},
	})
	if err != nil {
		t.Fatalf("stock parent Place: %v", err)
	}
	if got := stockParent.OrderID(); got != 576 {
		t.Fatalf("stock parent order id = %d, want 576", got)
	}

	t.Run("delta_hedge_stock_parent", func(t *testing.T) {
		child, err := client.Orders().Place(ctx,
			hedgeChild("sanitized-order-ref-0000000000000007", decimal.RequireFromString("1"), stockParent.OrderID(), "D", "0.5"))
		if err != nil {
			t.Fatalf("Place: %v", err)
		}
		if got := child.OrderID(); got != 577 {
			t.Fatalf("order id = %d, want 577", got)
		}
		requireOrderAPIError(t, "delta stock parent", child, ibkr.ErrCodeServerErrorReadingRequest,
			"Invalid delta hedge order. The parent order has to be option order")
	})

	var beta *ibkr.OrderHandle
	t.Run("beta_hedge_zero_size", func(t *testing.T) {
		var err error
		beta, err = client.Orders().Place(ctx,
			hedgeChild("sanitized-order-ref-0000000000000009", decimal.Zero, stockParent.OrderID(), "B", "1.0"))
		if err != nil {
			t.Fatalf("Place: %v", err)
		}
		if got := beta.OrderID(); got != 578 {
			t.Fatalf("order id = %d, want 578", got)
		}

		// The zero-size beta child is accepted: the Gateway assigns the
		// parent's one-share quantity and floors the limit to 0.01.
		open := waitForOpenOrder(t, ctx, beta)
		if !open.Order.Quantity.Equal(decimal.RequireFromString("1")) {
			t.Fatalf("gateway-assigned quantity = %s, want 1", open.Order.Quantity)
		}
		if !open.Order.Prices.LmtPrice.Equal(decimal.RequireFromString("0.01")) {
			t.Fatalf("gateway-floored lmt price = %s, want 0.01", open.Order.Prices.LmtPrice)
		}
		if open.Order.OCA.Group != "9000000576" {
			t.Fatalf("oca group = %q, want sanitized parent binding", open.Order.OCA.Group)
		}
		if (*open.Order.ParentID) != 576 {
			t.Fatalf("parent = %d, want 576", (*open.Order.ParentID))
		}
		held := waitOrderStatusUpdate(t, ctx, beta, ibkr.OrderStatusPreSubmitted)
		if held.WhyHeld != "child" {
			t.Fatalf("why held = %q, want child", held.WhyHeld)
		}
		if !held.Remaining.Equal(decimal.RequireFromString("1")) {
			t.Fatalf("remaining = %s, want 1", held.Remaining)
		}
	})
	if beta == nil {
		t.Fatal("beta subtest did not produce a handle")
	}

	var fx *ibkr.OrderHandle
	t.Run("fx_hedge", func(t *testing.T) {
		var err error
		fx, err = client.Orders().Place(ctx,
			hedgeChild("sanitized-order-ref-0000000000000015", decimal.Zero, stockParent.OrderID(), "F", ""))
		if err != nil {
			t.Fatalf("Place: %v", err)
		}
		if got := fx.OrderID(); got != 579 {
			t.Fatalf("order id = %d, want 579", got)
		}

		// Code 10063 is an attested outright placement rejection: no
		// order_status ever follows it, so it closes the child handle as
		// the terminal place error instead of riding the 10xxx
		// session-event fallback.
		requireOrderAPIError(t, "fx child", fx, ibkr.ErrCodeInvalidFXHedgeOrder,
			"Invalid FX hedge order. Hedging contract can only be a currency pair where one of the currencies is the same as in parent order.")
	})
	if fx == nil {
		t.Fatal("fx subtest did not produce a handle")
	}

	var pair *ibkr.OrderHandle
	t.Run("pair_hedge_zero_size", func(t *testing.T) {
		var err error
		pair, err = client.Orders().Place(ctx,
			hedgeChild("sanitized-order-ref-0000000000000017", decimal.Zero, stockParent.OrderID(), "P", "0.8"))
		if err != nil {
			t.Fatalf("Place: %v", err)
		}
		if got := pair.OrderID(); got != 580 {
			t.Fatalf("order id = %d, want 580", got)
		}

		// The current Gateway preserves the zero-sized pair child.
		open := waitForOpenOrder(t, ctx, pair)
		if !open.Order.Quantity.IsZero() {
			t.Fatalf("gateway-computed quantity = %s, want 0", open.Order.Quantity)
		}
		if open.Order.OCA.Group != "9000000576" || (*open.Order.ParentID) != 576 {
			t.Fatalf("oca/parent = %q/%d, want sanitized parent binding/576", open.Order.OCA.Group, (*open.Order.ParentID))
		}
		held := waitOrderStatusUpdate(t, ctx, pair, ibkr.OrderStatusPreSubmitted)
		if held.WhyHeld != "child" {
			t.Fatalf("why held = %q, want child", held.WhyHeld)
		}
		if !held.Remaining.IsZero() {
			t.Fatalf("remaining = %s, want 0", held.Remaining)
		}
	})
	if pair == nil {
		t.Fatal("pair subtest did not produce a handle")
	}

	// Teardown follows the recorder cleanup order exactly.
	if err := pair.Cancel(ctx); err != nil {
		t.Fatalf("pair Cancel: %v", err)
	}
	waitForOrderStatus(t, ctx, pair, ibkr.OrderStatusCancelled)
	if err := beta.Cancel(ctx); err != nil {
		t.Fatalf("beta Cancel: %v", err)
	}
	waitForOrderStatus(t, ctx, beta, ibkr.OrderStatusCancelled)
	if err := stockParent.Cancel(ctx); err != nil {
		t.Fatalf("stock parent Cancel: %v", err)
	}
	waitForOrderStatus(t, ctx, stockParent, ibkr.OrderStatusCancelled)
	if err := optionParent.Cancel(ctx); err != nil {
		t.Fatalf("option parent Cancel: %v", err)
	}
	waitForOrderStatus(t, ctx, optionParent, ibkr.OrderStatusCancelled)
	openOrders, err := client.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
	if err != nil {
		t.Fatalf("Open(all): %v", err)
	}
	if len(openOrders) != 0 {
		t.Fatalf("Open(all) returned %d orders after cleanup", len(openOrders))
	}
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime fence: %v", err)
	}

	// Closure shapes on the transcript disconnect: handles that reached a
	// terminal status close clean; the fx child closed on its code-10063
	// rejection back in the fx subtest, having emitted no business events.
	pair.Close()
	optionParent.Close()
	stockParent.Close()
	if err := pair.Wait(); err != nil {
		t.Fatalf("pair Wait() = %v, want nil (terminal Cancelled)", err)
	}
	if err := optionParent.Wait(); err != nil {
		t.Fatalf("option parent Wait() = %v, want nil (terminal Cancelled)", err)
	}
	requireOrderWaitNil(t, "stock parent", stockParent)
	requireOrderWaitNil(t, "beta child", beta)
	if _, ok := errors.AsType[*ibkr.APIError](fx.Wait()); !ok {
		t.Fatalf("fx Wait() = %v, want the terminal code-10063 *ibkr.APIError", fx.Wait())
	}
}

package ibkr_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/shopspring/decimal"
)

// TestAPIConditionsMatrixAAPLReplay freezes the order-conditions matrix from
// captures/20260824T204558Z-api_conditions_matrix_aapl, paper Gateway
// server_version 225, events.jsonl SHA-256
// 492109a4e1ecebd8ed6bdabfdc0e5f30c0ebdfaea08fc796b03ace771be52e5d.
//
// All six condition families were accepted by the live Gateway: each
// far-from-market LMT BUY reached PreSubmitted, then received the off-hours
// code-399 deferral warning. Under the current contract that warning is
// non-terminal: the order stays working at IB and the handle stays open,
// surfacing the 399 as an OrderEvent.Warning. The order's real lifecycle then
// continues to the Cancelled status the cancel produces, which closes the
// handle cleanly. The transcript's client place_order lines pin the exact
// condition wire shape (operator/value before conId/exchange for
// contract-bound types); a regression to the pre-fix field order fails the
// testhost frame match.
func TestAPIConditionsMatrixAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_conditions_matrix_aapl.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	replayDelayedAAPLQuoteAnchor(t, ctx, client)

	contract := ibkr.Contract{
		ConID:    265598,
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	}

	// Condition values are the exact live-captured inputs: the price value is
	// 10x the delayed AAPL quote anchor, the time value is the Gateway-accepted
	// UTC dash form, volume/percent are unreachable thresholds.
	cases := []struct {
		name      string
		condition ibkr.OrderCondition
		orderID   int64
	}{
		{
			name:      "price",
			condition: ibkr.OrderCondition{Type: ibkr.ConditionPrice, Conjunction: ibkr.ConditionAnd, Operator: ibkr.ConditionMore, ConID: 265598, Exchange: "SMART", Value: "2000", TriggerMethod: 4},
			orderID:   487,
		},
		{
			name:      "time",
			condition: ibkr.OrderCondition{Type: ibkr.ConditionTime, Conjunction: ibkr.ConditionAnd, Operator: ibkr.ConditionMore, Value: "20260824-20:48:09"},
			orderID:   488,
		},
		{
			name:      "margin",
			condition: ibkr.OrderCondition{Type: ibkr.ConditionMargin, Conjunction: ibkr.ConditionAnd, Operator: ibkr.ConditionMore, Value: "10"},
			orderID:   489,
		},
		{
			name:      "execution",
			condition: ibkr.OrderCondition{Type: ibkr.ConditionExecution, Conjunction: ibkr.ConditionAnd, SecType: ibkr.SecTypeStock, Exchange: "SMART", Symbol: "AAPL"},
			orderID:   490,
		},
		{
			name:      "volume",
			condition: ibkr.OrderCondition{Type: ibkr.ConditionVolume, Conjunction: ibkr.ConditionAnd, Operator: ibkr.ConditionMore, ConID: 265598, Exchange: "SMART", Value: "999999999"},
			orderID:   491,
		},
		{
			name:      "percent_change",
			condition: ibkr.OrderCondition{Type: ibkr.ConditionPercentChange, Conjunction: ibkr.ConditionAnd, Operator: ibkr.ConditionMore, ConID: 265598, Exchange: "SMART", Value: "50"},
			orderID:   492,
		},
	}

	handles := make([]*ibkr.OrderHandle, 0, len(cases))
	for _, tc := range cases {
		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: contract,
			Order: ibkr.Order{
				Action:    ibkr.ActionBuy,
				OrderType: ibkr.OrderTypeLimit,
				Quantity:  decimal.RequireFromString("1"),
				LmtPrice:  new(decimal.RequireFromString("10")),
				TIF:       ibkr.TIFDay,
				Account:   "DU9000001",
				OrderRef:  "sanitized-order-ref-0000000000000001",
				Conditions: ibkr.OrderConditions{
					Values:    []ibkr.OrderCondition{tc.condition},
					IgnoreRTH: true,
				},
			},
		})
		if err != nil {
			t.Fatalf("%s Place: %v", tc.name, err)
		}
		if got := handle.OrderID(); got != tc.orderID {
			t.Fatalf("%s order id = %d, want %d", tc.name, got, tc.orderID)
		}
		handles = append(handles, handle)

		// The exact live open_order frame fully decodes; these assertions pin
		// the base fields needed by every condition family.
		open := waitForOpenOrder(t, ctx, handle)
		if (*open.Order.OrderID) != tc.orderID {
			t.Fatalf("%s open order id = %d, want %d", tc.name, (*open.Order.OrderID), tc.orderID)
		}
		if open.Order.OrderRef != "sanitized-order-ref-0000000000000001" {
			t.Fatalf("%s order ref = %q", tc.name, open.Order.OrderRef)
		}
		if !open.Order.Prices.LmtPrice.Equal(decimal.RequireFromString("10")) {
			t.Fatalf("%s lmt price = %s, want 10", tc.name, open.Order.Prices.LmtPrice)
		}

		// Every condition family was accepted live and drew the code-399
		// off-hours deferral warning. It is delivered non-terminally as an
		// OrderEvent.Warning; the handle must stay open.
		warning := waitForOrderWarning(t, ctx, handle)
		if warning.Code != ibkr.ErrCodeOrderMessage {
			t.Fatalf("%s warning code = %d, want %d", tc.name, warning.Code, ibkr.ErrCodeOrderMessage)
		}
		if warning.OpKind != ibkr.OpPlaceOrder {
			t.Fatalf("%s warning op kind = %s, want %s", tc.name, warning.OpKind, ibkr.OpPlaceOrder)
		}
		if !strings.Contains(warning.Message, "will not be placed at the exchange until 2026-08-25 09:30:00 US/Eastern") {
			t.Fatalf("%s warning message = %q, want live deferral warning", tc.name, warning.Message)
		}
		select {
		case <-handle.Done():
			t.Fatalf("%s handle closed by the 399 warning; the order stays working at IB", tc.name)
		default:
		}

		// Cancel goes out on the wire; the transcript asserts the 5-field
		// cancel_order frame and replays the live order_status Cancelled +
		// code 202 acknowledgement.
		if err := handle.Cancel(ctx); err != nil {
			t.Fatalf("%s Cancel: %v", tc.name, err)
		}
	}

	// Each order's real lifecycle continues past the 399: the cancel produces
	// the live Cancelled status, which reaches the still-open handle and closes
	// it cleanly. Detachment is actor-serialized, so the transcript's immediate
	// EOF may win the race after the caller has already received Cancelled; that
	// exact captured-disconnect result is accepted below.
	for i, handle := range handles {
		name := cases[i].name
		statuses := waitOrderStatuses(t, ctx, handle)
		if !hasOrderStatus(statuses, ibkr.OrderStatusPreSubmitted) {
			t.Fatalf("%s statuses = %v, want PreSubmitted from live capture", name, statuses)
		}
		if !hasOrderStatus(statuses, ibkr.OrderStatusCancelled) {
			t.Fatalf("%s statuses = %v, want Cancelled to reach the still-open handle", name, statuses)
		}
		requireCloseOrCapturedDisconnect(t, name, handle.Wait())
	}
}

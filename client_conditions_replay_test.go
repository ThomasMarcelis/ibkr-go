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

// TestAPIConditionsMatrixAAPLReplay freezes the order-conditions matrix
// captured live on 2026-06-10 against paper Gateway server_version 200
// (captures/20260610T200935Z-api_conditions_matrix_aapl, events.jsonl sha256
// prefix 87059663ed139026) after the contract-bound condition field-order fix.
//
// All six condition families were accepted by the live Gateway: each
// far-from-market LMT BUY reached PreSubmitted, then received the off-hours
// code-399 deferral warning, which closes the handle before the cancel
// acknowledgement arrives. The transcript's client place_order lines pin the
// exact condition wire shape (operator/value before conId/exchange for
// contract-bound types); a regression to the pre-fix field order fails the
// testhost frame match.
func TestAPIConditionsMatrixAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_conditions_matrix_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

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
		permID    int64
	}{
		{
			name:      "price",
			condition: ibkr.OrderCondition{Type: 1, Conjunction: "a", Operator: 2, ConID: 265598, Exchange: "SMART", Value: "2918.1", TriggerMethod: 4},
			orderID:   356,
			permID:    900356,
		},
		{
			name:      "time",
			condition: ibkr.OrderCondition{Type: 3, Conjunction: "a", Operator: 2, Value: "20260610-20:11:37"},
			orderID:   357,
			permID:    900357,
		},
		{
			name:      "margin",
			condition: ibkr.OrderCondition{Type: 4, Conjunction: "a", Operator: 2, Value: "10"},
			orderID:   358,
			permID:    900358,
		},
		{
			name:      "execution",
			condition: ibkr.OrderCondition{Type: 5, Conjunction: "a", SecType: ibkr.SecTypeStock, Exchange: "SMART", Symbol: "AAPL"},
			orderID:   359,
			permID:    900359,
		},
		{
			name:      "volume",
			condition: ibkr.OrderCondition{Type: 6, Conjunction: "a", Operator: 2, ConID: 265598, Exchange: "SMART", Value: "999999999"},
			orderID:   360,
			permID:    900360,
		},
		{
			name:      "percent_change",
			condition: ibkr.OrderCondition{Type: 7, Conjunction: "a", Operator: 2, ConID: 265598, Exchange: "SMART", Value: "50"},
			orderID:   361,
			permID:    900361,
		},
	}

	for _, tc := range cases {
		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: contract,
			Order: ibkr.Order{
				Action:              ibkr.ActionBuy,
				OrderType:           ibkr.OrderTypeLimit,
				Quantity:            decimal.RequireFromString("100"),
				LmtPrice:            decimal.RequireFromString("14.59"),
				TIF:                 ibkr.TIFDay,
				Account:             "DU9000001",
				OrderRef:            "ibkrgo-sanitized-20260610T200935Z-001",
				Conditions:          []ibkr.OrderCondition{tc.condition},
				ConditionsIgnoreRTH: true,
			},
		})
		if err != nil {
			t.Fatalf("%s Place: %v", tc.name, err)
		}
		if got := handle.OrderID(); got != tc.orderID {
			t.Fatalf("%s order id = %d, want %d", tc.name, got, tc.orderID)
		}

		// Live open_order echoes for conditioned orders decode as the base
		// open-order shape (the conditioned frame takes the partial decode
		// path), so the assertions cover the base fields the live client
		// surfaced.
		open := waitForOpenOrder(t, ctx, handle)
		if open.OrderID != tc.orderID {
			t.Fatalf("%s open order id = %d, want %d", tc.name, open.OrderID, tc.orderID)
		}
		if open.PermID != tc.permID {
			t.Fatalf("%s perm id = %d, want %d", tc.name, open.PermID, tc.permID)
		}
		if open.OrderRef != "ibkrgo-sanitized-20260610T200935Z-001" {
			t.Fatalf("%s order ref = %q", tc.name, open.OrderRef)
		}
		if !open.LmtPrice.Equal(decimal.RequireFromString("14.59")) {
			t.Fatalf("%s lmt price = %s, want 14.59", tc.name, open.LmtPrice)
		}

		// Every condition family was accepted live: PreSubmitted, then the
		// code-399 deferral warning terminates the handle, so the later
		// Cancelled status never reaches it.
		statuses := waitOrderStatuses(t, ctx, handle)
		if !hasOrderStatus(statuses, ibkr.OrderStatusPreSubmitted) {
			t.Fatalf("%s statuses = %v, want PreSubmitted from live capture", tc.name, statuses)
		}
		if hasOrderStatus(statuses, ibkr.OrderStatusCancelled) {
			t.Fatalf("%s statuses = %v, want handle closed by code 399 before Cancelled", tc.name, statuses)
		}

		apiErr, ok := errors.AsType[*ibkr.APIError](handle.Wait())
		if !ok {
			t.Fatalf("%s Wait() = %v, want *ibkr.APIError", tc.name, handle.Wait())
		}
		if apiErr.Code != ibkr.ErrCodeOrderMessage {
			t.Fatalf("%s error code = %d, want %d", tc.name, apiErr.Code, ibkr.ErrCodeOrderMessage)
		}
		if apiErr.OpKind != ibkr.OpPlaceOrder {
			t.Fatalf("%s error op kind = %s, want %s", tc.name, apiErr.OpKind, ibkr.OpPlaceOrder)
		}
		if !strings.Contains(apiErr.Message, "will not be placed at the exchange until 2026-06-11 09:30:00 US/Eastern") {
			t.Fatalf("%s error message = %q, want live deferral warning", tc.name, apiErr.Message)
		}

		// Cancel still goes out on the wire after the 399 closed the handle;
		// the transcript asserts the 5-field cancel_order frame and replays
		// the live order_status Cancelled + code 202 acknowledgement.
		if err := handle.Cancel(ctx); err != nil {
			t.Fatalf("%s Cancel: %v", tc.name, err)
		}
	}
}

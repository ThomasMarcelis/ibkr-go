package ibkr_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
	"github.com/shopspring/decimal"
)

// waitForOrderWarning drains a handle's events until a non-terminal Warning
// arrives, failing if the handle closes first. It proves the warning is
// delivered without tearing the handle down.
func waitForOrderWarning(t *testing.T, ctx context.Context, handle *ibkr.OrderHandle) *ibkr.APIError {
	t.Helper()

	for {
		select {
		case evt, ok := <-handle.Events():
			if !ok {
				t.Fatal("order events closed before the warning arrived")
			}
			if evt.Warning != nil {
				return evt.Warning
			}
		case <-handle.Done():
			t.Fatal("handle closed before delivering the non-terminal warning")
		case <-ctx.Done():
			t.Fatal("timeout waiting for order warning")
		}
	}
}

// TestAPIConditionsMatrixAAPLReplay freezes the order-conditions matrix
// captured live on 2026-06-10 against paper Gateway server_version 200
// (captures/20260610T200935Z-api_conditions_matrix_aapl, events.jsonl sha256
// prefix 87059663ed139026) after the contract-bound condition field-order fix.
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
			condition: ibkr.OrderCondition{Type: ibkr.ConditionPrice, Conjunction: ibkr.ConditionAnd, Operator: ibkr.ConditionMore, ConID: 265598, Exchange: "SMART", Value: "2918.1", TriggerMethod: 4},
			orderID:   356,
			permID:    900356,
		},
		{
			name:      "time",
			condition: ibkr.OrderCondition{Type: ibkr.ConditionTime, Conjunction: ibkr.ConditionAnd, Operator: ibkr.ConditionMore, Value: "20260610-20:11:37"},
			orderID:   357,
			permID:    900357,
		},
		{
			name:      "margin",
			condition: ibkr.OrderCondition{Type: ibkr.ConditionMargin, Conjunction: ibkr.ConditionAnd, Operator: ibkr.ConditionMore, Value: "10"},
			orderID:   358,
			permID:    900358,
		},
		{
			name:      "execution",
			condition: ibkr.OrderCondition{Type: ibkr.ConditionExecution, Conjunction: ibkr.ConditionAnd, SecType: ibkr.SecTypeStock, Exchange: "SMART", Symbol: "AAPL"},
			orderID:   359,
			permID:    900359,
		},
		{
			name:      "volume",
			condition: ibkr.OrderCondition{Type: ibkr.ConditionVolume, Conjunction: ibkr.ConditionAnd, Operator: ibkr.ConditionMore, ConID: 265598, Exchange: "SMART", Value: "999999999"},
			orderID:   360,
			permID:    900360,
		},
		{
			name:      "percent_change",
			condition: ibkr.OrderCondition{Type: ibkr.ConditionPercentChange, Conjunction: ibkr.ConditionAnd, Operator: ibkr.ConditionMore, ConID: 265598, Exchange: "SMART", Value: "50"},
			orderID:   361,
			permID:    900361,
		},
	}

	handles := make([]*ibkr.OrderHandle, 0, len(cases))
	for _, tc := range cases {
		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: contract,
			Order: ibkr.Order{
				Action:    ibkr.ActionBuy,
				OrderType: ibkr.OrderTypeLimit,
				Quantity:  decimal.RequireFromString("100"),
				LmtPrice:  decimal.RequireFromString("14.59"),
				TIF:       ibkr.TIFDay,
				Account:   "DU9000001",
				OrderRef:  "ibkrgo-sanitized-20260610T200935Z-001",
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
		if !strings.Contains(warning.Message, "will not be placed at the exchange until 2026-06-11 09:30:00 US/Eastern") {
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
	// it cleanly (the terminal status is authoritative, so the transcript
	// disconnect does not bleed an error into per-order Wait).
	for i, handle := range handles {
		name := cases[i].name
		statuses := waitOrderStatuses(t, ctx, handle)
		if !hasOrderStatus(statuses, ibkr.OrderStatusPreSubmitted) {
			t.Fatalf("%s statuses = %v, want PreSubmitted from live capture", name, statuses)
		}
		if !hasOrderStatus(statuses, ibkr.OrderStatusCancelled) {
			t.Fatalf("%s statuses = %v, want Cancelled to reach the still-open handle", name, statuses)
		}
		if err := handle.Wait(); err != nil {
			t.Fatalf("%s Wait() = %v, want nil (terminal Cancelled is authoritative)", name, err)
		}
	}
}

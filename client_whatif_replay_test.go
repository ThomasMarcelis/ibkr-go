package ibkr_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
	"github.com/shopspring/decimal"
)

// TestAPIWhatIfMarginPreviewReplay freezes the WhatIf margin preview
// (AORD-010) captured live on 2026-06-10 against paper Gateway
// server_version 200 (captures/20260610T200009Z-api_whatif_margin_aapl,
// events.jsonl sha256 prefix e8ee70b24de3fe2f): a WhatIf MKT BUY 100 AAPL
// draws exactly one open_order reply whose order-state block carries the
// complete margin and commission preview, and no order_status lifecycle
// ever follows.
//
// The request goes through Orders().Preview, which forces the what-if flag.
// The place_order frame it emits is byte-identical to the captured what-if
// order (same order id, contract, and attributes plus what_if=true), so the
// transcript is untouched; Preview returns the order-state block as an
// [ibkr.OrderState] instead of surfacing a handle.
func TestAPIWhatIfMarginPreviewReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_whatif_margin_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	state, err := client.Orders().Preview(ctx, ibkr.PlaceOrderRequest{
		Contract: orderReplayAAPL,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("100"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "ibkrgo-sanitized-20260610T200009Z-001",
		},
	})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	for _, field := range []struct {
		name string
		got  decimal.Decimal
		want string
	}{
		{"InitMarginBefore", state.InitMarginBefore, "156037.86"},
		{"MaintMarginBefore", state.MaintMarginBefore, "141852.6"},
		{"EquityWithLoanBefore", state.EquityWithLoanBefore, "1127799.82"},
		{"InitMarginChange", state.InitMarginChange, "0"},
		{"MaintMarginChange", state.MaintMarginChange, "0"},
		{"EquityWithLoanChange", state.EquityWithLoanChange, "-0.8600000001024455"},
		{"InitMarginAfter", state.InitMarginAfter, "156037.86"},
		{"MaintMarginAfter", state.MaintMarginAfter, "141852.6"},
		{"EquityWithLoanAfter", state.EquityWithLoanAfter, "1127798.96"},
		{"Commission", state.Commission, "1.0003"},
	} {
		if !field.got.Equal(decimal.RequireFromString(field.want)) {
			t.Errorf("%s = %s, want %s", field.name, field.got, field.want)
		}
	}
	if state.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", state.Currency)
	}
	if !state.CommissionMin.IsZero() || !state.CommissionMax.IsZero() {
		t.Errorf("CommissionMin/Max = %s/%s, want zero (unset in capture)",
			state.CommissionMin, state.CommissionMax)
	}
}

// TestPreviewRejectedByGatewayReplay freezes the Preview rejection path: a
// code-201 api_error targeting the what-if order ID must resolve the blocked
// Preview caller with an *ibkr.APIError instead of dereferencing the nil
// handle of the preview route (actor crash before 2026-07-05). Frame shapes
// are live-attested; see the fixture header.
func TestPreviewRejectedByGatewayReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "whatif_rejected.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := client.Orders().Preview(ctx, ibkr.PlaceOrderRequest{
		Contract: orderReplayAAPL,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("100"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	var apiErr *ibkr.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("Preview error = %v, want *ibkr.APIError", err)
	}
	if apiErr.Code != 201 {
		t.Fatalf("Preview rejection code = %d, want 201", apiErr.Code)
	}
}

// TestPreviewInterruptedByDisconnectReplay freezes the Preview transport-loss
// path: a disconnect before the open_order echo must resolve the blocked
// caller with ErrInterrupted instead of crashing on the preview route's nil
// handle (see the fixture header).
func TestPreviewInterruptedByDisconnectReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "whatif_disconnect.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := client.Orders().Preview(ctx, ibkr.PlaceOrderRequest{
		Contract: orderReplayAAPL,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("100"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if !errors.Is(err, ibkr.ErrInterrupted) {
		t.Fatalf("Preview error after disconnect = %v, want ErrInterrupted", err)
	}
}

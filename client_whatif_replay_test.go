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
			OrderRef:  "ibkrgo-redacted-20260610T200009Z-001",
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

// TestPreviewRejectionsReplay freezes both preview error-routing branches from
// one exact live campaign: an ordinary request error and a 10xxx placement
// rejection. Neither request gets an open-order echo, so each targeted error
// must resolve the blocked Preview caller without an OrderHandle.
func TestPreviewRejectionsReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "whatif_rejections.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := client.Orders().Preview(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			Symbol:   "ZZZZNONE",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("1"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "ibkrgo-redacted-20260711T041916Z-001",
		},
	})
	var apiErr *ibkr.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("invalid-contract Preview error = %v, want *ibkr.APIError", err)
	}
	if apiErr.Code != ibkr.ErrCodeNoSecurityDefinition {
		t.Fatalf("invalid-contract Preview code = %d, want %d", apiErr.Code, ibkr.ErrCodeNoSecurityDefinition)
	}

	_, err = client.Orders().Preview(ctx, ibkr.PlaceOrderRequest{
		Contract: orderReplayAAPL,
		Order: ibkr.Order{
			Action:      ibkr.ActionBuy,
			OrderType:   ibkr.OrderTypeLimit,
			Quantity:    decimal.RequireFromString("1"),
			LmtPrice:    decimal.RequireFromString("150"),
			TIF:         ibkr.TIFDay,
			Account:     "DU9000001",
			OrderRef:    "ibkrgo-redacted-20260711T041916Z-002",
			DisplaySize: 1,
			Algorithm: ibkr.OrderAlgorithm{
				Strategy: "DarkIce",
				Params: []ibkr.TagValue{
					{Tag: "displaySize", Value: "1"},
					{Tag: "startTime", Value: "20260711 04:22:17 UTC"},
					{Tag: "endTime", Value: "20260711 04:39:17 UTC"},
					{Tag: "allowPastEndTime", Value: "1"},
				},
			},
		},
	})
	apiErr = nil
	if !errors.As(err, &apiErr) {
		t.Fatalf("DarkIce Preview error = %v, want *ibkr.APIError", err)
	}
	if apiErr.Code != ibkr.ErrCodeDisplaySizeNotAllowed {
		t.Fatalf("DarkIce Preview code = %d, want %d", apiErr.Code, ibkr.ErrCodeDisplaySizeNotAllowed)
	}
	if apiErr.Message != "The 'Display Size' order attribute may not be specified for this order." {
		t.Fatalf("DarkIce Preview message = %q", apiErr.Message)
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
			OrderRef:  "ibkrgo-redacted-20260711T040155Z-001",
		},
	})
	if !errors.Is(err, ibkr.ErrInterrupted) {
		t.Fatalf("Preview error after disconnect = %v, want ErrInterrupted", err)
	}
}

// TestPreviewInterruptedBy1101Replay freezes the Preview data-lost path: a
// 1100/1101 connectivity cycle between the what-if place_order and its
// open_order echo means the echo is never coming, so the blocked caller
// must get ErrInterrupted on the 1101 frame — mirroring the transport-loss
// resolution — rather than hang until its context deadline. The trailing
// CurrentTime call proves the engine resolved the preview while the
// connection was still up (see the fixture header).
func TestPreviewInterruptedBy1101Replay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "whatif_gap_1101.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
		t.Fatalf("Preview error after 1101 = %v, want ErrInterrupted", err)
	}

	if got := client.Session().State; got != ibkr.StateReady {
		t.Fatalf("session state after 1101 = %s, want %s", got, ibkr.StateReady)
	}
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime after 1101 = %v, want success on the live connection", err)
	}
}

package ibkr_test

import (
	"context"
	"errors"
	"io"
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
// ever follows. The handle never reaches a terminal order state, so it only
// closes when the session ends; on the transcript disconnect (ReconnectOff)
// it surfaces the transport EOF as its terminal error.
func TestAPIWhatIfMarginPreviewReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_whatif_margin_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: orderReplayAAPL,
		Order: ibkr.Order{
			Action:    ibkr.Buy,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("100"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "ibkrgo-sanitized-20260610T200009Z-001",
			WhatIf:    new(true),
		},
	})
	if err != nil {
		t.Fatalf("WhatIf Place: %v", err)
	}

	preview := waitForOpenOrder(t, ctx, handle)
	if preview.Status != ibkr.OrderStatusPreSubmitted {
		t.Errorf("Status = %q, want %q", preview.Status, ibkr.OrderStatusPreSubmitted)
	}
	if preview.Account != "DU9000001" {
		t.Errorf("Account = %q, want DU9000001", preview.Account)
	}
	if preview.PermID != 900355 {
		t.Errorf("PermID = %d, want 900355", preview.PermID)
	}
	for _, field := range []struct {
		name string
		got  decimal.Decimal
		want string
	}{
		{"InitMarginBefore", preview.InitMarginBefore, "156037.86"},
		{"MaintMarginBefore", preview.MaintMarginBefore, "141852.6"},
		{"EquityWithLoanBefore", preview.EquityWithLoanBefore, "1127799.82"},
		{"InitMarginChange", preview.InitMarginChange, "0"},
		{"MaintMarginChange", preview.MaintMarginChange, "0"},
		{"EquityWithLoanChange", preview.EquityWithLoanChange, "-0.8600000001024455"},
		{"InitMarginAfter", preview.InitMarginAfter, "156037.86"},
		{"MaintMarginAfter", preview.MaintMarginAfter, "141852.6"},
		{"EquityWithLoanAfter", preview.EquityWithLoanAfter, "1127798.96"},
		{"Commission", preview.Commission, "1.0003"},
	} {
		if !field.got.Equal(decimal.RequireFromString(field.want)) {
			t.Errorf("%s = %s, want %s", field.name, field.got, field.want)
		}
	}
	if preview.CommissionCurrency != "USD" {
		t.Errorf("CommissionCurrency = %q, want USD", preview.CommissionCurrency)
	}
	if !preview.MinCommission.IsZero() || !preview.MaxCommission.IsZero() {
		t.Errorf("Min/MaxCommission = %s/%s, want zero (unset in capture)",
			preview.MinCommission, preview.MaxCommission)
	}

	// No order_status lifecycle follows a what-if. The next observation on
	// the events channel is its closure when the transcript disconnect tears
	// down the session; any business event in between is a regression.
	select {
	case evt, ok := <-handle.Events():
		if ok {
			t.Fatalf("unexpected order event after preview: %+v", evt)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for handle close after disconnect")
	}

	if err := handle.Wait(); !errors.Is(err, io.EOF) {
		t.Fatalf("handle.Wait() = %v, want io.EOF from session teardown", err)
	}
}

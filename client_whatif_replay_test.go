package ibkr_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/shopspring/decimal"
)

// TestAPIWhatIfMarginPreviewReplay freezes the current sv225 WhatIf sequence:
// exact typed rejections for an unknown contract and an invalid DarkIce
// display-size combination, followed by a successful AAPL margin preview.
func TestAPIWhatIfMarginPreviewReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_whatif_margin_aapl.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	replayWhatIfRejections(t, ctx, client)

	state, err := client.Orders().Preview(ctx, ibkr.PlaceOrderRequest{
		Contract: orderReplayAAPL,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("100"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "sanitized-order-ref-0000000000000003",
		},
	})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	for _, field := range []struct {
		name string
		got  *decimal.Decimal
		want string
	}{
		{"InitMarginBefore", state.InitMarginBefore, "148839.9"},
		{"MaintMarginBefore", state.MaintMarginBefore, "132591.07"},
		{"EquityWithLoanBefore", state.EquityWithLoanBefore, "1170559.72"},
		{"InitMarginChange", state.InitMarginChange, "4398.700000000012"},
		{"MaintMarginChange", state.MaintMarginChange, "3998.820000000007"},
		{"EquityWithLoanChange", state.EquityWithLoanChange, "-0.8499999998603016"},
		{"InitMarginAfter", state.InitMarginAfter, "153238.6"},
		{"MaintMarginAfter", state.MaintMarginAfter, "136589.89"},
		{"EquityWithLoanAfter", state.EquityWithLoanAfter, "1170558.87"},
		{"CommissionAndFees", state.CommissionAndFees, "1.0003"},
	} {
		if field.got == nil || !field.got.Equal(decimal.RequireFromString(field.want)) {
			t.Errorf("%s = %s, want %s", field.name, field.got, field.want)
		}
	}
	if state.CommissionAndFeesCurrency != "USD" {
		t.Errorf("CommissionAndFeesCurrency = %q, want USD", state.CommissionAndFeesCurrency)
	}
	if state.MinCommissionAndFees != nil || state.MaxCommissionAndFees != nil {
		t.Errorf("CommissionMin/Max = %v/%v, want nil (unset in capture)",
			state.MinCommissionAndFees, state.MaxCommissionAndFees)
	}
	if state.Status != ibkr.OrderStatusPreSubmitted || state.MarginCurrency != "EUR" {
		t.Errorf("status/margin currency = %s/%q, want PreSubmitted/EUR", state.Status, state.MarginCurrency)
	}
	for _, field := range []struct {
		name string
		got  *decimal.Decimal
	}{
		{"InitMarginBeforeOutsideRTH", state.InitMarginBeforeOutsideRTH},
		{"MaintMarginBeforeOutsideRTH", state.MaintMarginBeforeOutsideRTH},
		{"EquityWithLoanBeforeOutsideRTH", state.EquityWithLoanBeforeOutsideRTH},
		{"InitMarginChangeOutsideRTH", state.InitMarginChangeOutsideRTH},
		{"MaintMarginChangeOutsideRTH", state.MaintMarginChangeOutsideRTH},
		{"EquityWithLoanChangeOutsideRTH", state.EquityWithLoanChangeOutsideRTH},
		{"InitMarginAfterOutsideRTH", state.InitMarginAfterOutsideRTH},
		{"MaintMarginAfterOutsideRTH", state.MaintMarginAfterOutsideRTH},
		{"EquityWithLoanAfterOutsideRTH", state.EquityWithLoanAfterOutsideRTH},
		{"SuggestedSize", state.SuggestedSize},
	} {
		if field.got != nil {
			t.Errorf("%s = %s, want nil (unset in capture)", field.name, field.got)
		}
	}
	if state.RejectReason != "" || state.Allocations != nil {
		t.Errorf("reject reason/allocations = %q/%v, want empty/nil", state.RejectReason, state.Allocations)
	}
}

func replayWhatIfRejections(t *testing.T, ctx context.Context, client *ibkr.Client) {
	t.Helper()

	_, err := client.Orders().Preview(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{Symbol: "ZZZZNONE", SecType: ibkr.SecTypeStock, Exchange: "SMART", Currency: "USD"},
		Order: ibkr.Order{
			Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeMarket, Quantity: decimal.NewFromInt(1),
			TIF: ibkr.TIFDay, Account: "DU9000001", OrderRef: "sanitized-order-ref-0000000000000001",
		},
	})
	apiErr, ok := errors.AsType[*ibkr.APIError](err)
	if !ok || apiErr.Code != ibkr.ErrCodeNoSecurityDefinition || apiErr.OpKind != ibkr.OpPlaceOrder {
		t.Fatalf("invalid-contract Preview error = %v, want typed code %d", err, ibkr.ErrCodeNoSecurityDefinition)
	}

	_, err = client.Orders().Preview(ctx, ibkr.PlaceOrderRequest{
		Contract: orderReplayAAPL,
		Order: ibkr.Order{
			Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeLimit, Quantity: decimal.NewFromInt(1),
			LmtPrice: new(decimal.NewFromInt(150)), TIF: ibkr.TIFDay, Account: "DU9000001",
			OrderRef: "sanitized-order-ref-0000000000000002", DisplaySize: 1,
			Algorithm: ibkr.OrderAlgorithm{
				Strategy: "DarkIce",
				Params: []ibkr.TagValue{
					{Tag: "displaySize", Value: "1"},
					{Tag: "startTime", Value: "20260824 21:07:26 UTC"},
					{Tag: "endTime", Value: "20260824 21:24:26 UTC"},
					{Tag: "allowPastEndTime", Value: "1"},
				},
			},
		},
	})
	apiErr, ok = errors.AsType[*ibkr.APIError](err)
	if !ok || apiErr.Code != ibkr.ErrCodeDisplaySizeNotAllowed || apiErr.OpKind != ibkr.OpPlaceOrder {
		t.Fatalf("DarkIce Preview error = %v, want typed code %d", err, ibkr.ErrCodeDisplaySizeNotAllowed)
	}

}

// TestPreviewInterruptedByDisconnectReplay freezes the Preview transport-loss
// path: a disconnect before the open_order echo must resolve the blocked
// caller with ErrInterrupted instead of crashing on the preview route's nil
// handle (see the fixture header).
func TestPreviewInterruptedByDisconnectReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "whatif_disconnect.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	replayWhatIfRejections(t, ctx, client)

	_, err := client.Orders().Preview(ctx, ibkr.PlaceOrderRequest{
		Contract: orderReplayAAPL,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("100"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "sanitized-order-ref-0000000000000003",
		},
	})
	if !errors.Is(err, ibkr.ErrInterrupted) {
		t.Fatalf("Preview error after disconnect = %v, want ErrInterrupted", err)
	}
}

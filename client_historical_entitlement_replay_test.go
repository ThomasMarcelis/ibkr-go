package ibkr_test

import (
	"context"
	"errors"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
)

func TestHistoricalBarsSubscriptionRequiredReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "historical_bars_subscription_required.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.History().Bars(ctx, ibkr.HistoricalBarsRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Duration:   ibkr.Days(1),
		BarSize:    ibkr.Bar1Hour,
		WhatToShow: ibkr.ShowTrades,
		UseRTH:     true,
	})
	apiErr, ok := errors.AsType[*ibkr.APIError](err)
	if !ok {
		t.Fatalf("History().Bars() error = %v, want *APIError", err)
	}
	if apiErr.OpKind != ibkr.OpHistoricalBars || apiErr.Code != ibkr.ErrCodeHistoricalDataSubscriptionRequired {
		t.Fatalf("History().Bars() error = %+v", apiErr)
	}
	if !apiErr.IsEntitlement() {
		t.Fatal("historical code 2188 is not classified as an entitlement error")
	}
	if ibkr.IsRetryable(apiErr) {
		t.Fatal("historical code 2188 is classified as retryable")
	}
}

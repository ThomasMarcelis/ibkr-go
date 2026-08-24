package ibkr_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
)

func TestRealTimeBarsAPIRejectionIsNonRetryable(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "realtime_bars_api_error.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("SetType() error = %v", err)
	}

	sub, err := client.MarketData().SubscribeRealTimeBars(ctx, ibkr.RealTimeBarsRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		WhatToShow: ibkr.ShowTrades,
		UseRTH:     true,
	})
	if err != nil {
		t.Fatalf("SubscribeRealTimeBars() error = %v", err)
	}

	started := waitForStateKind(t, sub.Events(), ibkr.StreamStarted)
	if started.Err != nil {
		t.Fatalf("started.Err = %v, want nil", started.Err)
	}

	waitErr := sub.Wait()
	apiErr, ok := errors.AsType[*ibkr.APIError](waitErr)
	if !ok {
		t.Fatalf("sub.Wait() error type = %T, want *ibkr.APIError", waitErr)
	}
	if apiErr.Code != 420 {
		t.Fatalf("APIError.Code = %d, want 420", apiErr.Code)
	}

	select {
	case _, ok := <-sub.Events():
		if ok {
			t.Fatal("Events() produced a bar after API rejection")
		}
	default:
		t.Fatal("Events() still open after sub.Wait()")
	}
	if ibkr.IsRetryable(waitErr) {
		t.Fatal("IsRetryable(sub.Wait()) = true, want false for API rejection")
	}
	waitAPIErr, ok := errors.AsType[*ibkr.APIError](waitErr)
	if !ok {
		t.Fatalf("sub.Wait() error type = %T, want *ibkr.APIError", waitErr)
	}
	if waitAPIErr.Code != 420 {
		t.Fatalf("sub.Wait() APIError.Code = %d, want 420", waitAPIErr.Code)
	}
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime() fence error = %v", err)
	}
}

func TestDisconnectDuringSnapshotPhase(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "error_disconnect_during_snapshot.txt",
		ibkr.WithReconnectPolicy(ibkr.ReconnectOff))
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Group: "All",
		Tags:  []string{"NetLiquidation", "TotalCashValue", "BuyingPower", "ExcessLiquidity"},
	})
	if err == nil {
		t.Fatal("AccountSummary() error = nil, want error on disconnect before end marker")
	}
}

func TestWSHMetaDataError10xxx(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "wsh_meta_data_error.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.WSH().MetaData(ctx)
	if err == nil {
		t.Fatal("WSHMetaData() error = nil, want API error 10276")
	}

	apiErr, ok := errors.AsType[*ibkr.APIError](err)
	if !ok {
		t.Fatalf("error type = %T, want *ibkr.APIError", err)
	}
	if apiErr.Code != 10276 {
		t.Fatalf("APIError.Code = %d, want 10276", apiErr.Code)
	}
}

func TestAPIWSHVariantsAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_wsh_variants_aapl.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	assertWSHEntitlementError := func(label string, err error, wantOp ibkr.OpKind) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s error = nil, want API error 10276", label)
		}
		apiErr, ok := errors.AsType[*ibkr.APIError](err)
		if !ok {
			t.Fatalf("%s error type = %T, want *ibkr.APIError", label, err)
		}
		if apiErr.Code != 10276 {
			t.Fatalf("%s APIError.Code = %d, want 10276", label, apiErr.Code)
		}
		if apiErr.OpKind != wantOp {
			t.Fatalf("%s APIError.OpKind = %s, want %s", label, apiErr.OpKind, wantOp)
		}
	}

	_, err := client.WSH().MetaData(ctx)
	assertWSHEntitlementError("MetaData", err, ibkr.OpWSHMetaData)

	start := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	eventCases := []struct {
		label string
		req   ibkr.WSHEventDataRequest
	}{
		{label: "by_conid", req: ibkr.WSHEventDataRequest{ConID: 265598, StartDate: start, EndDate: end, TotalLimit: 10}},
		{label: "portfolio", req: ibkr.WSHEventDataRequest{FillPortfolio: true, StartDate: start, EndDate: end, TotalLimit: 10}},
		{label: "watchlist_competitors", req: ibkr.WSHEventDataRequest{ConID: 265598, FillWatchlist: true, FillCompetitors: true, TotalLimit: 10}},
	}
	for _, tc := range eventCases {
		_, err := client.WSH().EventData(ctx, tc.req)
		assertWSHEntitlementError(tc.label, err, ibkr.OpWSHEventData)
	}
}

func TestMarketDepthError10xxx(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "market_depth_error.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.MarketData().SubscribeDepth(ctx, ibkr.MarketDepthRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		NumRows: 5,
	}, ibkr.WithResumePolicy(ibkr.ResumeNever))
	if err != nil {
		t.Fatalf("SubscribeMarketDepth() error = %v", err)
	}

	waitErr := sub.Wait()
	apiErr, ok := errors.AsType[*ibkr.APIError](waitErr)
	if !ok {
		t.Fatalf("sub.Wait() error type = %T, want *ibkr.APIError", waitErr)
	}
	if apiErr.Code != 10092 {
		t.Fatalf("APIError.Code = %d, want 10092", apiErr.Code)
	}
	if ibkr.IsRetryable(waitErr) {
		t.Fatal("IsRetryable(sub.Wait()) = true, want false for API error")
	}
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime() fence error = %v", err)
	}
}

package ibkr_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
)

func TestAPIErrorOnOneShot(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "error_api_error_oneshot.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Contracts().Details(ctx, ibkr.Contract{
		Symbol:   "ZZZZNONE",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	})
	if err == nil {
		t.Fatal("ContractDetails() error = nil, want API error")
	}

	apiErr, ok := errors.AsType[*ibkr.APIError](err)
	if !ok {
		t.Fatalf("error type = %T, want *ibkr.APIError", err)
	}
	if apiErr.Code != 200 {
		t.Fatalf("APIError.Code = %d, want 200", apiErr.Code)
	}
}

func TestAPIErrorOnSubscription(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "error_api_error_subscription.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
	})
	if err != nil {
		t.Fatalf("SubscribeQuotes() error = %v", err)
	}

	closed := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionClosed)

	apiErr, ok := errors.AsType[*ibkr.APIError](closed.Err)
	if !ok {
		t.Fatalf("closed.Err type = %T, want *ibkr.APIError", closed.Err)
	}
	if apiErr.Code != 354 {
		t.Fatalf("APIError.Code = %d, want 354", apiErr.Code)
	}
	if closed.Retryable {
		t.Fatal("closed.Retryable = true, want false for API error")
	}

	waitErr := sub.Wait()
	if waitErr == nil {
		t.Fatal("sub.Wait() error = nil, want API error")
	}
	waitAPIErr, ok := errors.AsType[*ibkr.APIError](waitErr)
	if !ok {
		t.Fatalf("sub.Wait() error type = %T, want *ibkr.APIError", waitErr)
	}
	if waitAPIErr.Code != 354 {
		t.Fatalf("sub.Wait() APIError.Code = %d, want 354", waitAPIErr.Code)
	}
	if ibkr.IsRetryable(waitErr) {
		t.Fatal("IsRetryable(sub.Wait()) = true, want false for API error")
	}
}

func TestRealTimeBarsAPIRejectionIsNonRetryable(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "realtime_bars_api_error.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.MarketData().SubscribeRealTimeBars(ctx, ibkr.RealTimeBarsRequest{
		Contract: ibkr.Contract{
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

	started := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionStarted)
	if started.Retryable {
		t.Fatal("started.Retryable = true, want false")
	}

	closed := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionClosed)
	if closed.Retryable {
		t.Fatal("closed.Retryable = true, want false for API rejection")
	}
	apiErr, ok := errors.AsType[*ibkr.APIError](closed.Err)
	if !ok {
		t.Fatalf("closed.Err type = %T, want *ibkr.APIError", closed.Err)
	}
	if apiErr.Code != 10089 {
		t.Fatalf("APIError.Code = %d, want 10089", apiErr.Code)
	}

	waitErr := sub.Wait()
	if waitErr == nil {
		t.Fatal("sub.Wait() error = nil, want API error")
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
	if waitAPIErr.Code != 10089 {
		t.Fatalf("sub.Wait() APIError.Code = %d, want 10089", waitAPIErr.Code)
	}
}

func TestDisconnectDuringOneShot(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "error_disconnect_during_oneshot.txt",
		ibkr.WithReconnectPolicy(ibkr.ReconnectOff))
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.History().Bars(ctx, ibkr.HistoricalBarsRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		EndTime:    time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
		Duration:   ibkr.Days(1),
		BarSize:    ibkr.Bar1Hour,
		WhatToShow: ibkr.ShowTrades,
		UseRTH:     true,
	})
	if err == nil {
		t.Fatal("HistoricalBars() error = nil, want error on disconnect")
	}
}

func TestDisconnectDuringSnapshotPhase(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "error_disconnect_during_snapshot.txt",
		ibkr.WithReconnectPolicy(ibkr.ReconnectOff))
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Account: "All",
		Tags:    []string{"NetLiquidation"},
	})
	if err == nil {
		t.Fatal("AccountSummary() error = nil, want error on disconnect before end marker")
	}
}

func TestMarketDataWarningDoesNotCloseSubscription(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "error_market_data_warning.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
	})
	if err != nil {
		t.Fatalf("SubscribeQuotes() error = %v", err)
	}

	waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionStarted)

	// Read quote updates to confirm data flows after the warning.
	// Each tick_price arrives as a separate update; accumulate until both
	// bid and ask are populated.
	var lastUpdate ibkr.QuoteUpdate
	for i := 0; i < 5; i++ {
		lastUpdate = waitForEvent(t, sub.Events())
		if !lastUpdate.Snapshot.Bid.IsZero() && !lastUpdate.Snapshot.Ask.IsZero() {
			break
		}
	}
	if lastUpdate.Snapshot.Bid.IsZero() {
		t.Fatal("expected non-zero bid after market data warning")
	}
	if lastUpdate.Snapshot.Ask.IsZero() {
		t.Fatal("expected non-zero ask after market data warning")
	}

	if err := sub.Close(); err != nil {
		t.Fatalf("sub.Close() error = %v", err)
	}
}

func TestFarmStatusCodesAreInformational(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "error_farm_status_codes.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	details, err := client.Contracts().Details(ctx, ibkr.Contract{
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	})
	if err != nil {
		t.Fatalf("ContractDetails() error = %v", err)
	}
	if len(details) != 1 {
		t.Fatalf("details len = %d, want 1", len(details))
	}
	if details[0].LongName != "APPLE INC" {
		t.Fatalf("long name = %q, want APPLE INC", details[0].LongName)
	}
}

func TestEmptyResultSets(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "error_empty_results.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	positions, err := client.Accounts().Positions(ctx)
	if err != nil {
		t.Fatalf("PositionsSnapshot() error = %v", err)
	}
	if positions == nil {
		t.Fatal("PositionsSnapshot() = nil, want non-nil empty slice")
	}
	if len(positions) != 0 {
		t.Fatalf("PositionsSnapshot() len = %d, want 0", len(positions))
	}

	completed, err := client.Orders().Completed(ctx, true)
	if err != nil {
		t.Fatalf("CompletedOrders() error = %v", err)
	}
	if len(completed) != 0 {
		t.Fatalf("CompletedOrders() len = %d, want 0", len(completed))
	}
}

func TestWSHMetaDataError10xxx(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "wsh_meta_data_error.txt")
	defer client.Close()
	defer waitHost(t, host)

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

func TestMarketDepthError10xxx(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "market_depth_error.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.MarketData().SubscribeDepth(ctx, ibkr.MarketDepthRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		NumRows:      5,
		IsSmartDepth: true,
	}, ibkr.WithResumePolicy(ibkr.ResumeNever))
	if err != nil {
		t.Fatalf("SubscribeMarketDepth() error = %v", err)
	}

	closed := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionClosed)

	apiErr, ok := errors.AsType[*ibkr.APIError](closed.Err)
	if !ok {
		t.Fatalf("closed.Err type = %T, want *ibkr.APIError", closed.Err)
	}
	if apiErr.Code != 10092 {
		t.Fatalf("APIError.Code = %d, want 10092", apiErr.Code)
	}
	if closed.Retryable {
		t.Fatal("closed.Retryable = true, want false for API error")
	}

	waitErr := sub.Wait()
	if waitErr == nil {
		t.Fatal("sub.Wait() error = nil, want API error")
	}
	if ibkr.IsRetryable(waitErr) {
		t.Fatal("IsRetryable(sub.Wait()) = true, want false for API error")
	}
}

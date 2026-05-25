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

func TestAPIFundamentalReportErrorsAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_fundamental_report_errors_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const wantMessage = "The fundamentals data for the security specified is not available.Missing reportType"
	cases := []struct {
		label      string
		reportType ibkr.FundamentalReportType
	}{
		{label: "ReportRatios", reportType: ibkr.FundamentalReportRatios},
		{label: "ReportsFinStatements", reportType: ibkr.FundamentalReportsFinStatements},
	}
	for _, tc := range cases {
		_, err := client.Contracts().FundamentalData(ctx, ibkr.FundamentalDataRequest{
			Contract: ibkr.Contract{
				ConID:    265598,
				Symbol:   "AAPL",
				SecType:  ibkr.SecTypeStock,
				Exchange: "SMART",
				Currency: "USD",
			},
			ReportType: tc.reportType,
		})
		if err == nil {
			t.Fatalf("%s FundamentalData() error = nil, want API error 430", tc.label)
		}
		apiErr, ok := errors.AsType[*ibkr.APIError](err)
		if !ok {
			t.Fatalf("%s error type = %T, want *ibkr.APIError", tc.label, err)
		}
		if apiErr.Code != 430 {
			t.Fatalf("%s APIError.Code = %d, want 430", tc.label, apiErr.Code)
		}
		if apiErr.OpKind != ibkr.OpFundamentalData {
			t.Fatalf("%s APIError.OpKind = %s, want %s", tc.label, apiErr.OpKind, ibkr.OpFundamentalData)
		}
		if apiErr.Message != wantMessage {
			t.Fatalf("%s APIError.Message = %q, want %q", tc.label, apiErr.Message, wantMessage)
		}
	}
}

func TestAPISecurityTypeProbeErrorsReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_security_type_probe_errors.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cases := []struct {
		label    string
		contract ibkr.Contract
	}{
		{label: "BOND", contract: ibkr.Contract{Symbol: "912797", SecType: ibkr.SecTypeBond, Exchange: "SMART", Currency: "USD"}},
		{label: "BILL", contract: ibkr.Contract{Symbol: "912797", SecType: ibkr.SecTypeBill, Exchange: "SMART", Currency: "USD"}},
	}
	for _, tc := range cases {
		_, err := client.Contracts().Details(ctx, tc.contract)
		if err == nil {
			t.Fatalf("%s ContractDetails() error = nil, want API error 200", tc.label)
		}
		apiErr, ok := errors.AsType[*ibkr.APIError](err)
		if !ok {
			t.Fatalf("%s error type = %T, want *ibkr.APIError", tc.label, err)
		}
		if apiErr.Code != 200 {
			t.Fatalf("%s APIError.Code = %d, want 200", tc.label, apiErr.Code)
		}
		if apiErr.OpKind != ibkr.OpContractDetails {
			t.Fatalf("%s APIError.OpKind = %s, want %s", tc.label, apiErr.OpKind, ibkr.OpContractDetails)
		}
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

func TestAPITickByTickEntitlementErrorsAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_tick_by_tick_entitlement_errors_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cases := []struct {
		label      string
		tickType   ibkr.TickByTickType
		ignoreSize bool
	}{
		{label: "Last", tickType: ibkr.TickByTickLast},
		{label: "AllLast", tickType: ibkr.TickByTickAllLast},
		{label: "AllLastIgnoreSize", tickType: ibkr.TickByTickAllLast, ignoreSize: true},
		{label: "BidAsk", tickType: ibkr.TickByTickBidAsk},
		{label: "MidPoint", tickType: ibkr.TickByTickMidPoint},
	}
	for _, tc := range cases {
		sub, err := client.MarketData().SubscribeTickByTick(ctx, ibkr.TickByTickRequest{
			Contract: ibkr.Contract{
				ConID:    265598,
				Symbol:   "AAPL",
				SecType:  ibkr.SecTypeStock,
				Exchange: "SMART",
				Currency: "USD",
			},
			TickType:   tc.tickType,
			IgnoreSize: tc.ignoreSize,
		})
		if err != nil {
			t.Fatalf("%s SubscribeTickByTick() error = %v", tc.label, err)
		}

		closed := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionClosed)
		apiErr, ok := errors.AsType[*ibkr.APIError](closed.Err)
		if !ok {
			t.Fatalf("%s closed.Err type = %T, want *ibkr.APIError", tc.label, closed.Err)
		}
		if apiErr.Code != 10089 {
			t.Fatalf("%s APIError.Code = %d, want 10089", tc.label, apiErr.Code)
		}
		if apiErr.OpKind != ibkr.OpTickByTick {
			t.Fatalf("%s APIError.OpKind = %s, want %s", tc.label, apiErr.OpKind, ibkr.OpTickByTick)
		}
		if closed.Retryable {
			t.Fatalf("%s closed.Retryable = true, want false", tc.label)
		}

		waitErr := sub.Wait()
		if waitErr == nil {
			t.Fatalf("%s sub.Wait() error = nil, want API error", tc.label)
		}
		waitAPIErr, ok := errors.AsType[*ibkr.APIError](waitErr)
		if !ok {
			t.Fatalf("%s sub.Wait() error type = %T, want *ibkr.APIError", tc.label, waitErr)
		}
		if waitAPIErr.Code != 10089 {
			t.Fatalf("%s sub.Wait() APIError.Code = %d, want 10089", tc.label, waitAPIErr.Code)
		}
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

func TestAPIWSHVariantsAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_wsh_variants_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

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

	start := time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
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

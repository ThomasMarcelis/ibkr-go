package ibkr_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
)

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

func TestRealTimeBarsAPIRejectionIsNonRetryable(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "realtime_bars_api_error.txt")
	defer client.Close()
	defer waitHost(t, host)

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
	if apiErr.Code != 420 {
		t.Fatalf("APIError.Code = %d, want 420", apiErr.Code)
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
	if waitAPIErr.Code != 420 {
		t.Fatalf("sub.Wait() APIError.Code = %d, want 420", waitAPIErr.Code)
	}
}

func TestAPIRealTimeBarsRequestErrorsAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_realtime_bars_request_errors_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cases := []struct {
		label      string
		whatToShow ibkr.WhatToShow
		wantCode   int
	}{
		{label: "TRADES", whatToShow: ibkr.ShowTrades, wantCode: 420},
		{label: "BID_ASK", whatToShow: ibkr.ShowBidAsk, wantCode: 321},
		{label: "MIDPOINT", whatToShow: ibkr.ShowMidpoint, wantCode: 10089},
	}
	for _, tc := range cases {
		sub, err := client.MarketData().SubscribeRealTimeBars(ctx, ibkr.RealTimeBarsRequest{
			Contract: ibkr.Contract{
				ConID:    265598,
				Symbol:   "AAPL",
				SecType:  ibkr.SecTypeStock,
				Exchange: "SMART",
				Currency: "USD",
			},
			WhatToShow: tc.whatToShow,
		})
		if err != nil {
			t.Fatalf("%s SubscribeRealTimeBars() error = %v", tc.label, err)
		}

		closed := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionClosed)
		apiErr, ok := errors.AsType[*ibkr.APIError](closed.Err)
		if !ok {
			t.Fatalf("%s closed.Err type = %T, want *ibkr.APIError", tc.label, closed.Err)
		}
		if apiErr.Code != tc.wantCode {
			t.Fatalf("%s APIError.Code = %d, want %d", tc.label, apiErr.Code, tc.wantCode)
		}
		if apiErr.OpKind != ibkr.OpRealTimeBars {
			t.Fatalf("%s APIError.OpKind = %s, want %s", tc.label, apiErr.OpKind, ibkr.OpRealTimeBars)
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
		if waitAPIErr.Code != tc.wantCode {
			t.Fatalf("%s sub.Wait() APIError.Code = %d, want %d", tc.label, waitAPIErr.Code, tc.wantCode)
		}
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
		Group: "All",
		Tags:  []string{"NetLiquidation"},
	})
	if err == nil {
		t.Fatal("AccountSummary() error = nil, want error on disconnect before end marker")
	}
}

func TestCompletedOrdersReturnsEmptyResult(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "completed_orders_empty.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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

func TestMarketDepthRejectsLossySlowConsumerPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		clientOpts []ibkr.Option
		subOpts    []ibkr.SubscriptionOption
	}{
		{
			name:       "inherited client default",
			clientOpts: []ibkr.Option{ibkr.WithDefaultSlowConsumerPolicy(ibkr.SlowConsumerDropOldest)},
		},
		{
			name:    "subscription override",
			subOpts: []ibkr.SubscriptionOption{ibkr.WithSlowConsumerPolicy(ibkr.SlowConsumerDropOldest)},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			client, host := newClient(t, "grounded_bootstrap.txt", test.clientOpts...)
			defer client.Close()
			defer waitHost(t, host)

			sub, err := client.MarketData().SubscribeDepth(context.Background(), ibkr.MarketDepthRequest{}, test.subOpts...)
			if sub != nil {
				t.Fatalf("SubscribeDepth() subscription = %v, want nil", sub)
			}
			validationErr, ok := errors.AsType[*ibkr.ValidationError](err)
			if !ok || validationErr.Field != "SlowConsumerPolicy" || validationErr.Value != string(ibkr.SlowConsumerDropOldest) {
				t.Fatalf("SubscribeDepth() error = %v, want SlowConsumerPolicy ValidationError", err)
			}
		})
	}
}

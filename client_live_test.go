package ibkr_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/ibkrlive"
	"github.com/shopspring/decimal"
)

var aaplContract = ibkr.Contract{
	Symbol:   "AAPL",
	SecType:  ibkr.SecTypeStock,
	Exchange: "SMART",
	Currency: "USD",
}

func TestLiveDialContext(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 10*time.Second)
	defer cancel()
	defer client.Close()

	if got := client.Session().State; got != ibkr.StateReady {
		t.Fatalf("client.Session().State = %s, want %s", got, ibkr.StateReady)
	}
}

func TestLiveContractDetailsAAPL(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 10*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelReq()

	details, err := client.Contracts().Details(ctx, aaplContract)
	if err != nil {
		t.Fatalf("ContractDetails() error = %v", err)
	}
	if len(details) == 0 {
		t.Fatal("details len = 0, want at least one contract")
	}
}

func TestLiveCFDQuoteReroute(t *testing.T) {
	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelReq()
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("MarketData().SetType(delayed) error = %v", err)
	}
	sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{Contract: ibkr.Contract{
		Symbol: "IBM", SecType: ibkr.SecTypeCFD, Exchange: "SMART", Currency: "USD",
	}})
	if err != nil {
		t.Fatalf("SubscribeQuotes(IBM CFD): %v", err)
	}
	defer sub.Close()

	for {
		select {
		case event, ok := <-sub.Events():
			if !ok {
				err := sub.Err()
				if isExactLiveCFDQuoteBlocker(err) {
					t.Skipf("IBM CFD reroute reached exact live entitlement blocker: %v", err)
				}
				t.Fatalf("IBM CFD quote closed before rerouted data: %v", err)
			}
			if event.Kind == ibkr.StreamData &&
				event.Value.Changed&^ibkr.QuoteFieldMarketDataType != 0 {
				return
			}
		case <-ctx.Done():
			t.Fatalf("IBM CFD quote produced no rerouted data: %v", context.Cause(ctx))
		}
	}
}

func TestLiveAccountSummary(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()

	values, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Group: "All",
		Tags:  []string{"NetLiquidation", "BuyingPower"},
	})
	if err != nil {
		t.Fatalf("AccountSummary() error = %v", err)
	}
	if len(values) == 0 {
		t.Fatal("AccountSummary() returned 0 values, want >= 1")
	}
	for _, v := range values {
		if v.Value == "" {
			t.Errorf("AccountValue{Tag:%s, Account:%s} has empty Value", v.Tag, v.Account)
		}
		if v.Currency == "" {
			t.Errorf("AccountValue{Tag:%s, Account:%s} has empty Currency", v.Tag, v.Account)
		}
	}
}

func TestLivePositions(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()

	positions, err := client.Accounts().Positions(ctx)
	if err != nil {
		t.Fatalf("PositionsSnapshot() error = %v", err)
	}
	if len(positions) == 0 {
		t.Log("PositionsSnapshot() returned 0 positions")
		return
	}
	if positions[0].Contract.Symbol == "" {
		t.Error("first position has empty Symbol")
	}
}

func TestLiveHistoricalBars(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()

	bars, err := client.History().Bars(ctx, ibkr.HistoricalBarsRequest{
		Contract:   aaplContract,
		EndTime:    time.Now(),
		Duration:   ibkr.Days(1),
		BarSize:    ibkr.Bar1Hour,
		WhatToShow: ibkr.ShowTrades,
		UseRTH:     true,
	})
	if err != nil {
		if isLiveHistoricalDataUnavailable(err, ibkr.OpHistoricalBars) {
			t.Skipf("HistoricalBars() reached exact live historical-data blocker: %v", err)
		}
		t.Fatalf("HistoricalBars() error = %v", err)
	}
	if len(bars) == 0 {
		t.Fatal("HistoricalBars() returned 0 bars, want >= 1")
	}
	for i, b := range bars {
		if b.Time.IsZero() {
			t.Errorf("bar[%d] has zero Time", i)
		}
	}
}

func TestLivePersistentClientSequentialRequests(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 45*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancelReq()

	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("MarketData().SetType() error = %v", err)
	}

	first, err := client.History().Bars(ctx, ibkr.HistoricalBarsRequest{
		Contract:   aaplContract,
		EndTime:    time.Now(),
		Duration:   ibkr.Days(5),
		BarSize:    ibkr.Bar1Day,
		WhatToShow: ibkr.ShowTrades,
		UseRTH:     true,
	})
	if err != nil {
		if isLiveHistoricalDataUnavailable(err, ibkr.OpHistoricalBars) {
			t.Skipf("HistoricalBars() returned: %v (current Gateway historical data session constraint)", err)
		}
		t.Fatalf("first HistoricalBars() error = %v", err)
	}
	if len(first) == 0 {
		t.Fatal("first HistoricalBars() returned 0 bars")
	}

	second, err := client.History().Bars(ctx, ibkr.HistoricalBarsRequest{
		Contract:   aaplContract,
		EndTime:    time.Now(),
		Duration:   ibkr.Days(1),
		BarSize:    ibkr.Bar1Hour,
		WhatToShow: ibkr.ShowTrades,
		UseRTH:     true,
	})
	if err != nil {
		t.Fatalf("second HistoricalBars() error = %v", err)
	}
	if len(second) == 0 {
		t.Fatal("second HistoricalBars() returned 0 bars")
	}

	if _, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: aaplContract}); err != nil {
		t.Fatalf("Quote() after historical bars error = %v", err)
	}
	if _, err := client.Contracts().Details(ctx, aaplContract); err != nil {
		t.Fatalf("ContractDetails() after historical bars error = %v", err)
	}
	if got := client.Session().State; got != ibkr.StateReady && got != ibkr.StateDegraded {
		t.Fatalf("session state after sequential requests = %s, want usable session", got)
	}
}

func TestLiveCurrentTime(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelReq()

	ts, err := client.CurrentTime(ctx)
	if err != nil {
		t.Fatalf("CurrentTime() error = %v", err)
	}
	if ts.IsZero() {
		t.Fatal("CurrentTime() returned zero time")
	}
	// Sanity check: server time should be within 5 minutes of local now.
	if delta := time.Since(ts); delta > 5*time.Minute || delta < -5*time.Minute {
		t.Errorf("CurrentTime() drift = %v, want < 5 minutes", delta)
	}
}

func TestLiveManagedAccountsRefresh(t *testing.T) {
	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelReq()
	accounts, err := client.ManagedAccounts(ctx)
	if err != nil {
		t.Fatalf("ManagedAccounts() error = %v", err)
	}
	if len(accounts) == 0 {
		t.Fatal("ManagedAccounts() returned no accounts")
	}
	if !slices.Equal(accounts, client.Session().ManagedAccounts) {
		t.Fatalf("ManagedAccounts() = %v, Session().ManagedAccounts = %v", accounts, client.Session().ManagedAccounts)
	}
}

func TestLiveHistoricalSchedule(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()

	schedule, err := client.History().Schedule(ctx, ibkr.HistoricalScheduleRequest{
		Contract: aaplContract,
		Duration: ibkr.Months(1),
		BarSize:  ibkr.Bar1Day,
		UseRTH:   true,
	})
	if err != nil {
		if isLiveHistoricalDataUnavailable(err, ibkr.OpHistoricalSchedule) {
			t.Skipf("HistoricalSchedule() reached exact live historical-data blocker: %v", err)
		}
		t.Fatalf("HistoricalSchedule() error = %v", err)
	}
	if schedule.TimeZone == "" {
		t.Error("HistoricalSchedule returned empty TimeZone")
	}
	if len(schedule.Sessions) == 0 {
		t.Fatal("HistoricalSchedule() returned 0 sessions, want >= 1")
	}
	for i, s := range schedule.Sessions {
		if s.StartDateTime == "" || s.EndDateTime == "" || s.RefDate == "" {
			t.Errorf("session[%d] has empty field(s): %+v", i, s)
		}
	}
}

func TestLiveQuoteSnapshot(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()

	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("MarketData().SetType(delayed) error = %v", err)
	}
	quote, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{
		Contract: aaplContract,
	})
	if err != nil {
		t.Fatalf("QuoteSnapshot() error = %v", err)
	}
	if quote.Available == 0 {
		t.Fatal("QuoteSnapshot() returned no available quote fields")
	}
	t.Logf("QuoteSnapshot: Available=%d, Bid=%s, Ask=%s, Last=%s",
		quote.Available, quote.Bid, quote.Ask, quote.Last)
}

func TestLiveRegulatorySnapshot(t *testing.T) {
	t.Skip("the sole authorized v2.0.1 attempt was consumed by capture 20260824T195855Z-regulatory_snapshot_aapl_v201_authorized_once; never retry it")
}

func TestLiveOpenOrders(t *testing.T) {
	t.Parallel()

	// "all" scope requires clientID=0.
	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second, ibkr.WithClientID(0))
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()

	orders, err := client.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
	if err != nil {
		t.Fatalf("OpenOrdersSnapshot() error = %v", err)
	}
	// May be empty if no open orders exist; that is fine.
	t.Logf("OpenOrdersSnapshot: %d orders", len(orders))
}

func TestLiveCompletedOrders(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()

	orders, err := client.Orders().Completed(ctx, true)
	if err != nil {
		if apiErr, ok := errors.AsType[*ibkr.APIError](err); ok &&
			apiErr.OpKind == ibkr.OpCompletedOrders &&
			apiErr.Code == ibkr.ErrCodeServerErrorValidatingRequest &&
			strings.Contains(apiErr.Message, "Error validating request.-'S'") &&
			strings.Contains(apiErr.Message, "API interface is currently in Read-Only mode") {
			t.Skipf("CompletedOrders() reached exact live read-only blocker: %v", err)
		}
		t.Fatalf("CompletedOrders() error = %v", err)
	}
	t.Logf("CompletedOrders: %d orders", len(orders))
}

func TestLiveExecutions(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()

	executions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{})
	if err != nil {
		t.Fatalf("Executions() error = %v", err)
	}
	t.Logf("Executions: %d rows", len(executions.Executions))
}

func TestLiveSubscribeAccountSummary(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 20*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelReq()

	sub, err := client.Accounts().SubscribeSummary(ctx, ibkr.AccountSummaryRequest{
		Group: "All",
		Tags:  []string{"NetLiquidation", "BuyingPower"},
	}, ibkr.WithResumePolicy(ibkr.ResumeNever))
	if err != nil {
		t.Fatalf("SubscribeAccountSummary() error = %v", err)
	}
	defer sub.Close()

	var events int
	deadline := time.After(15 * time.Second)
	for {
		select {
		case evt, ok := <-sub.Events():
			if !ok {
				t.Fatal("Events channel closed before SnapshotComplete")
			}
			if evt.Kind == ibkr.StreamData {
				events++
			}
			if evt.Kind == ibkr.StreamSnapshotComplete {
				if events == 0 {
					t.Fatal("SnapshotComplete with 0 events")
				}
				t.Logf("SubscribeAccountSummary: %d events before SnapshotComplete", events)
				return
			}
			if evt.Err != nil {
				t.Fatalf("subscription state error: %v", evt.Err)
			}
		case <-deadline:
			t.Fatal("timed out waiting for SnapshotComplete")
		}
	}
}

func TestLiveSoftDollarTiers(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 10*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelReq()

	tiers, err := client.Advisors().SoftDollarTiers(ctx)
	if err != nil {
		t.Fatalf("SoftDollarTiers() error = %v", err)
	}
	t.Logf("SoftDollarTiers: %d tiers", len(tiers))
	for _, tier := range tiers {
		if tier.Name == "" {
			t.Error("tier has empty Name")
		}
	}
}

func TestLiveQueryDisplayGroups(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 10*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelReq()

	groups, err := client.TWS().DisplayGroups(ctx)
	if err != nil {
		t.Fatalf("QueryDisplayGroups() error = %v", err)
	}
	if len(groups) == 0 {
		t.Fatal("QueryDisplayGroups() returned no groups")
	}
	t.Logf("QueryDisplayGroups: %v", groups)
}

func TestLiveQualifyContractAAPL(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()

	details, err := client.Contracts().Qualify(ctx, aaplContract)
	if err != nil {
		t.Fatalf("QualifyContract() error = %v", err)
	}
	if details.MinTick.IsZero() {
		t.Error("QualifyContract() returned zero MinTick")
	}
	t.Logf("QualifyContract: conID=%d, minTick=%s",
		details.ConID,
		details.MinTick.String())
}

func TestLiveWSHMetaData(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()

	data, err := client.WSH().MetaData(ctx)
	if err != nil {
		if apiErr, ok := errors.AsType[*ibkr.APIError](err); ok &&
			apiErr.OpKind == ibkr.OpWSHMetaData && apiErr.Code == 10276 &&
			strings.Contains(apiErr.Message, "News feed is not allowed") {
			t.Skipf("WSHMetaData() reached exact live news-feed blocker: %v", err)
		}
		t.Fatalf("WSHMetaData() error = %v", err)
	}
	if len(data) == 0 {
		t.Fatal("WSHMetaData() returned empty document")
	}
	t.Logf("WSHMetaData: %d bytes", len(data))
}

func TestLiveRequestFA(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()

	data, err := client.Advisors().Config(ctx, 1)
	if err != nil {
		if apiErr, ok := errors.AsType[*ibkr.APIError](err); ok &&
			apiErr.OpKind == ibkr.OpFAConfig &&
			apiErr.Code == ibkr.ErrCodeServerErrorValidatingRequest &&
			strings.Contains(apiErr.Message, "FA data operations ignored for non FA customers") {
			t.Skipf("RequestFA() reached exact live non-FA blocker: %v", err)
		}
		t.Fatalf("RequestFA() error = %v", err)
	}
	if len(data) == 0 {
		t.Fatal("RequestFA() returned empty document")
	}
	t.Logf("RequestFA: %d bytes", len(data))
}

func TestLiveSubscribeMarketDepth(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()

	sub, err := client.MarketData().SubscribeDepth(ctx, ibkr.MarketDepthRequest{
		Contract:     aaplContract,
		NumRows:      5,
		IsSmartDepth: false,
	}, ibkr.WithResumePolicy(ibkr.ResumeNever))
	if err != nil {
		if isExactLiveMarketDepthBlocker(err) {
			t.Skipf("SubscribeMarketDepth() reached exact live depth blocker: %v", err)
		}
		t.Fatalf("SubscribeMarketDepth() error = %v", err)
	}
	defer sub.Close()

	var events int
	deadline := time.After(10 * time.Second)
	sessionEvents := client.SessionEvents()
	streamEvents := sub.Events()
	for {
		select {
		case event, ok := <-streamEvents:
			if !ok {
				err := sub.Err()
				if isExactLiveMarketDepthBlocker(err) {
					t.Skipf("SubscribeMarketDepth() reached exact live depth blocker: %v", err)
				}
				t.Fatalf("SubscribeMarketDepth() closed after %d rows: %v", events, err)
			}
			if event.Err != nil {
				if isExactLiveMarketDepthBlocker(event.Err) {
					t.Skipf("SubscribeMarketDepth() reached exact live depth blocker: %v", event.Err)
				}
				t.Fatalf("SubscribeMarketDepth() stream error = %v", event.Err)
			}
			if event.Kind == ibkr.StreamData {
				events++
				if events >= 5 {
					t.Logf("SubscribeMarketDepth: received %d depth rows", events)
					return
				}
			}
			if event.Kind == ibkr.StreamNotice && event.Notice != nil && event.Notice.Code == ibkr.ErrCodeSmartDepthExchanges {
				t.Logf("SubscribeMarketDepth availability: %s", event.Notice.Message)
			}
		case evt, ok := <-sessionEvents:
			if !ok {
				sessionEvents = nil
				continue
			}
			if evt.Code == ibkr.ErrCodeSmartDepthExchanges {
				if isExactLiveNoMarketDepthNotice(evt.Message) {
					t.Skipf("SubscribeMarketDepth() reached exact live availability blocker: %s", evt.Message)
				}
				t.Logf("SubscribeMarketDepth availability: %s", evt.Message)
				continue
			}
			if evt.Err != nil {
				t.Fatalf("session error while awaiting market depth = %v", evt.Err)
			}
		case <-deadline:
			if events == 0 {
				t.Fatal("SubscribeMarketDepth() produced no row or exact typed blocker before timeout")
			}
			if events < 5 {
				t.Logf("SubscribeMarketDepth: received %d depth rows", events)
				return
			}
		}
	}
}

func TestLiveSubscribePositions(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 20*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelReq()

	sub, err := client.Accounts().SubscribePositions(ctx, ibkr.WithResumePolicy(ibkr.ResumeNever))
	if err != nil {
		t.Fatalf("SubscribePositions() error = %v", err)
	}
	defer sub.Close()

	var events int
	deadline := time.After(15 * time.Second)
	for {
		select {
		case event, ok := <-sub.Events():
			if !ok {
				t.Fatal("Events channel closed before SnapshotComplete")
			}
			if event.Kind == ibkr.StreamSnapshotComplete {
				t.Logf("SubscribePositions: %d events before SnapshotComplete", events)
				return
			}
			if event.Err != nil {
				t.Fatalf("subscription state error: %v", event.Err)
			}
			events++
		case <-deadline:
			t.Fatal("timed out waiting for SnapshotComplete")
		}
	}
}

func TestLiveWSHEventData(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()

	data, err := client.WSH().EventData(ctx, ibkr.WSHEventDataRequest{ConID: 265598})
	if err != nil {
		if apiErr, ok := errors.AsType[*ibkr.APIError](err); ok &&
			apiErr.OpKind == ibkr.OpWSHEventData && apiErr.Code == 10276 &&
			strings.Contains(apiErr.Message, "News feed is not allowed") {
			t.Skipf("WSHEventData() reached exact live news-feed blocker: %v", err)
		}
		t.Fatalf("WSHEventData() error = %v", err)
	}
	if len(data) == 0 {
		t.Fatal("WSHEventData() returned empty document")
	}
	t.Logf("WSHEventData: %d bytes", len(data))
}

func TestLiveHistoricalNews(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 20*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelReq()
	lowerBound := time.Now().UTC().AddDate(0, -6, 0).Truncate(time.Second)

	items, err := client.News().Historical(ctx, ibkr.HistoricalNewsRequest{
		ConID:         265598,
		ProviderCodes: []ibkr.NewsProviderCode{"BRFG", "BRFUPDN", "DJNL"},
		EndTime:       lowerBound,
		TotalResults:  20,
	})
	if err != nil {
		t.Fatalf("HistoricalNews() error = %v", err)
	}
	if len(items.Items) == 0 {
		t.Fatal("HistoricalNews() returned 0 items")
	}
	for _, item := range items.Items {
		if item.Time.Before(lowerBound) {
			t.Fatalf("HistoricalNews() item %s is before lower bound %s", item.Time, lowerBound)
		}
	}
	t.Logf("HistoricalNews: %d items (has_more=%t)", len(items.Items), items.HasMore)
}

func TestLiveMatchingSymbols(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 10*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelReq()

	matches, err := client.Contracts().Search(ctx, "AAPL")
	if err != nil {
		t.Fatalf("MatchingSymbols() error = %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("MatchingSymbols() returned 0 matches")
	}
	t.Logf("MatchingSymbols: %d matches", len(matches))
}

func TestLiveHeadTimestamp(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 10*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelReq()

	ts, err := client.History().HeadTimestamp(ctx, ibkr.HeadTimestampRequest{
		Contract:   aaplContract,
		WhatToShow: ibkr.ShowTrades,
	})
	if err != nil {
		t.Fatalf("HeadTimestamp() error = %v", err)
	}
	if ts.IsZero() {
		t.Fatal("HeadTimestamp() returned zero time")
	}
	t.Logf("HeadTimestamp: %s", ts)
}

func TestLiveHistoricalTicks(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()
	end := time.Now().UTC()

	result, err := client.History().Ticks(ctx, ibkr.HistoricalTicksRequest{
		Contract:      aaplContract,
		EndTime:       end,
		NumberOfTicks: 10,
		WhatToShow:    ibkr.ShowMidpoint,
	})
	if err != nil {
		if isLiveHistoricalDataUnavailable(err, ibkr.OpHistoricalTicks) {
			t.Skipf("HistoricalTicks() reached exact live historical-data blocker: %v", err)
		}
		t.Fatalf("HistoricalTicks() error = %v", err)
	}
	if len(result.Ticks) == 0 {
		t.Fatal("HistoricalTicks() returned 0 ticks")
	}
	t.Logf("HistoricalTicks: %d ticks", len(result.Ticks))
}

func TestLiveOptionCalculations(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 2*time.Minute)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelReq()
	runLiveOptionCalculations(t, ctx, client)
}

func runLiveOptionCalculations(t *testing.T, ctx context.Context, client *ibkr.Client) {
	t.Helper()

	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("SetType(Delayed) error = %v", err)
	}
	quote, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: aaplContract})
	if err != nil {
		t.Fatalf("delayed Quote(AAPL) error = %v", err)
	}
	underPrice := firstPositiveDecimal(quote.Last, quote.Ask, quote.Bid, quote.Close)
	if !underPrice.IsPositive() {
		t.Fatalf("delayed Quote(AAPL) has no positive anchor: %+v", quote)
	}
	contract := liveQualifiedAAPLCallForCalculation(t, ctx, client, underPrice)

	for _, test := range []struct {
		name string
		call func() (ibkr.OptionComputation, error)
	}{
		{
			name: "price",
			call: func() (ibkr.OptionComputation, error) {
				return client.Options().Price(ctx, ibkr.CalcOptionPriceRequest{
					Contract: contract, Volatility: decimal.RequireFromString("0.3"), UnderPrice: underPrice,
				})
			},
		},
		{
			name: "implied volatility",
			call: func() (ibkr.OptionComputation, error) {
				return client.Options().ImpliedVolatility(ctx, ibkr.CalcImpliedVolatilityRequest{
					Contract: contract, OptionPrice: decimal.RequireFromString("5"), UnderPrice: underPrice,
				})
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := test.call()
			if err != nil {
				t.Fatalf("option calculation error = %v", err)
			}
			if result.Available == 0 {
				t.Fatal("option calculation returned no available fields")
			}
			t.Logf("option calculation: %+v", result)
		})
	}
}

func firstPositiveDecimal(values ...decimal.Decimal) decimal.Decimal {
	for _, value := range values {
		if value.IsPositive() {
			return value
		}
	}
	return decimal.Zero
}

func liveChooseOptionParams(params []ibkr.SecDefOptParams) (ibkr.SecDefOptParams, bool) {
	for _, param := range params {
		if param.Exchange == "SMART" && param.Multiplier != "" && len(param.Expirations) > 0 && len(param.Strikes) > 0 {
			return param, true
		}
	}
	for _, param := range params {
		if param.Multiplier != "" && len(param.Expirations) > 0 && len(param.Strikes) > 0 {
			return param, true
		}
	}
	return ibkr.SecDefOptParams{}, false
}

func liveChooseFutureExpiry(expirations []string) (string, bool) {
	sorted := slices.Clone(expirations)
	slices.Sort(sorted)
	now := time.Now().Format("20060102")
	for _, expiry := range sorted {
		if expiry >= now {
			return expiry, true
		}
	}
	return "", false
}

func liveChooseNearestStrike(strikes []decimal.Decimal, anchor decimal.Decimal) (decimal.Decimal, bool) {
	if len(strikes) == 0 {
		return decimal.Zero, false
	}
	best := strikes[0]
	bestDistance := best.Sub(anchor).Abs()
	for _, strike := range strikes[1:] {
		distance := strike.Sub(anchor).Abs()
		if distance.LessThan(bestDistance) {
			best = strike
			bestDistance = distance
		}
	}
	return best, true
}

func liveQualifiedAAPLCallForCalculation(t *testing.T, ctx context.Context, client *ibkr.Client, underPrice decimal.Decimal) ibkr.Contract {
	t.Helper()

	params, err := client.Contracts().SecDefOptParams(ctx, ibkr.SecDefOptParamsRequest{
		UnderlyingSymbol: "AAPL", UnderlyingSecType: ibkr.SecTypeStock, UnderlyingConID: 265598,
	})
	if err != nil {
		t.Fatalf("SecDefOptParams(AAPL) error = %v", err)
	}
	param, ok := liveChooseOptionParams(params)
	if !ok {
		t.Fatal("SecDefOptParams(AAPL) returned no SMART option parameters")
	}
	expiry, ok := liveChooseFutureExpiry(param.Expirations)
	if !ok {
		t.Fatal("SecDefOptParams(AAPL) returned no future expiration")
	}
	strike, ok := liveChooseNearestStrike(param.Strikes, underPrice)
	if !ok {
		t.Fatal("SecDefOptParams(AAPL) returned no strikes")
	}
	details, err := client.Contracts().Details(ctx, ibkr.Contract{
		Symbol: "AAPL", SecType: ibkr.SecTypeOption, Expiry: expiry, Strike: new(strike),
		Right: ibkr.RightCall, Multiplier: param.Multiplier, Exchange: "SMART", Currency: "USD",
		TradingClass: param.TradingClass,
	})
	if err != nil {
		t.Fatalf("ContractDetails(AAPL call) error = %v", err)
	}
	if len(details) == 0 {
		t.Fatal("ContractDetails(AAPL call) returned no contract")
	}
	return details[0].Contract
}

func TestLiveSubscribeDisplayGroup(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()

	handle, err := client.TWS().SubscribeDisplayGroup(ctx, 1)
	if err != nil {
		t.Fatalf("SubscribeDisplayGroup() error = %v", err)
	}
	defer handle.Close()

	waitForStateKind(t, handle.Events(), ibkr.StreamStarted)

	// Update the display group to AAPL's conID.
	if err := handle.Update(ctx, "265598"); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	for {
		select {
		case event, ok := <-handle.Events():
			if !ok {
				t.Fatalf("Events channel closed before display-group data: %v", handle.Err())
			}
			if event.Err != nil {
				t.Fatalf("SubscribeDisplayGroup() stream error = %v", event.Err)
			}
			if event.Kind == ibkr.StreamData {
				t.Logf("DisplayGroupUpdate: %s", event.Value.ContractInfo)
				return
			}
		case <-ctx.Done():
			t.Fatalf("SubscribeDisplayGroup() produced no data: %v", context.Cause(ctx))
		}
	}
}

func TestLiveSubscribeOpenOrders(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second,
		ibkr.WithClientID(0))
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()

	sub, err := client.Orders().SubscribeOpen(ctx, ibkr.OpenOrdersScopeAll,
		ibkr.WithResumePolicy(ibkr.ResumeNever))
	if err != nil {
		t.Fatalf("SubscribeOpenOrders() error = %v", err)
	}
	defer sub.Close()

	deadline := time.After(10 * time.Second)
	var events int
	for {
		select {
		case evt, ok := <-sub.Events():
			if !ok {
				t.Fatalf("Events closed after %d events", events)
			}
			if evt.Kind == ibkr.StreamData {
				events++
			}
			if evt.Kind == ibkr.StreamSnapshotComplete {
				t.Logf("SubscribeOpenOrders: %d open orders", events)
				return
			}
			if evt.Err != nil {
				t.Fatalf("subscription error: %v", evt.Err)
			}
		case <-deadline:
			t.Fatalf("timeout waiting for SnapshotComplete (got %d events)", events)
		}
	}
}

func isLiveHistoricalDataUnavailable(err error, opKind ibkr.OpKind) bool {
	apiErr, ok := errors.AsType[*ibkr.APIError](err)
	if !ok || apiErr.OpKind != opKind {
		return false
	}
	switch apiErr.Code {
	case ibkr.ErrCodeHistoricalDataSubscriptionRequired:
		return apiErr.Message == "Up-to-the-second historical data requires additional subscription for the API."
	case 162:
		return apiErr.Message == "Historical Market Data Service error message:No market data permissions for ISLAND STK. Requested market data requires additional subscription for API. See link in 'Market Data Connections' dialog for more details."
	case 10187:
		return apiErr.Message == "Failed to request historical ticks:No market data permissions for ISLAND STK"
	}
	return false
}

func TestLiveHistoricalBlockerClassifier(t *testing.T) {
	t.Parallel()

	const subscriptionMessage = "Up-to-the-second historical data requires additional subscription for the API."
	for _, test := range []struct {
		name string
		err  error
		op   ibkr.OpKind
		want bool
	}{
		{
			name: "exact bars blocker",
			err:  &ibkr.APIError{OpKind: ibkr.OpHistoricalBars, Code: ibkr.ErrCodeHistoricalDataSubscriptionRequired, Message: subscriptionMessage},
			op:   ibkr.OpHistoricalBars,
			want: true,
		},
		{
			name: "exact ticks blocker",
			err:  &ibkr.APIError{OpKind: ibkr.OpHistoricalTicks, Code: ibkr.ErrCodeHistoricalDataSubscriptionRequired, Message: subscriptionMessage},
			op:   ibkr.OpHistoricalTicks,
			want: true,
		},
		{
			name: "wrong operation",
			err:  &ibkr.APIError{OpKind: ibkr.OpHistoricalTicks, Code: ibkr.ErrCodeHistoricalDataSubscriptionRequired, Message: subscriptionMessage},
			op:   ibkr.OpHistoricalBars,
		},
		{
			name: "unattested message",
			err:  &ibkr.APIError{OpKind: ibkr.OpHistoricalBars, Code: ibkr.ErrCodeHistoricalDataSubscriptionRequired, Message: "different historical failure"},
			op:   ibkr.OpHistoricalBars,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := isLiveHistoricalDataUnavailable(test.err, test.op); got != test.want {
				t.Fatalf("isLiveHistoricalDataUnavailable() = %t, want %t", got, test.want)
			}
		})
	}
}

func isExactLiveMarketDepthBlocker(err error) bool {
	apiErr, ok := errors.AsType[*ibkr.APIError](err)
	if !ok || apiErr.OpKind != ibkr.OpMarketDepth {
		return false
	}
	return apiErr.Code == ibkr.ErrCodeDeepMarketDataNotSupported &&
		apiErr.Message == "Deep market data is not supported for this combination of security type/exchange"
}

func isExactLiveNoMarketDepthNotice(message string) bool {
	return strings.HasPrefix(message, "Exchanges - Top: ") &&
		strings.Contains(message, "; Need additional market data permissions - Depth: ") &&
		strings.HasSuffix(message, "; ") &&
		!strings.Contains(message, "Exchanges - Depth:")
}

func isExactLiveCFDQuoteBlocker(err error) bool {
	apiErr, ok := errors.AsType[*ibkr.APIError](err)
	return ok &&
		apiErr.OpKind == ibkr.OpQuotes &&
		apiErr.Code == ibkr.ErrCodeAdditionalSubscriptionRequired &&
		apiErr.Message == "Requested market data requires additional subscription for API. See link in 'Market Data Connections' dialog for more details.Delayed market data is available.IBM NYSE/TOP/ALL"
}

func TestLiveMarketDataBlockerClassifiers(t *testing.T) {
	t.Parallel()

	const cfdMessage = "Requested market data requires additional subscription for API. See link in 'Market Data Connections' dialog for more details.Delayed market data is available.IBM NYSE/TOP/ALL"
	for _, test := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "exact CFD", err: &ibkr.APIError{OpKind: ibkr.OpQuotes, Code: ibkr.ErrCodeAdditionalSubscriptionRequired, Message: cfdMessage}, want: true},
		{name: "CFD wrong operation", err: &ibkr.APIError{OpKind: ibkr.OpMarketDepth, Code: ibkr.ErrCodeAdditionalSubscriptionRequired, Message: cfdMessage}},
		{name: "CFD wrong instrument", err: &ibkr.APIError{OpKind: ibkr.OpQuotes, Code: ibkr.ErrCodeAdditionalSubscriptionRequired, Message: strings.Replace(cfdMessage, "IBM", "AAPL", 1)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := isExactLiveCFDQuoteBlocker(test.err); got != test.want {
				t.Fatalf("isExactLiveCFDQuoteBlocker() = %t, want %t", got, test.want)
			}
		})
	}

	const depthMessage = "Deep market data is not supported for this combination of security type/exchange"
	if !isExactLiveMarketDepthBlocker(&ibkr.APIError{OpKind: ibkr.OpMarketDepth, Code: ibkr.ErrCodeDeepMarketDataNotSupported, Message: depthMessage}) {
		t.Fatal("isExactLiveMarketDepthBlocker() rejected the exact captured blocker")
	}
	if isExactLiveMarketDepthBlocker(&ibkr.APIError{OpKind: ibkr.OpMarketDepth, Code: ibkr.ErrCodeDeepMarketDataNotSupported, Message: depthMessage + "."}) {
		t.Fatal("isExactLiveMarketDepthBlocker() accepted an unattested message")
	}

	const notice = "Exchanges - Top: IBEOS; OVERNIGHT; Need additional market data permissions - Depth: NASDAQ; BATS; ARCA; "
	if !isExactLiveNoMarketDepthNotice(notice) {
		t.Fatal("isExactLiveNoMarketDepthNotice() rejected the captured notice shape")
	}
	if isExactLiveNoMarketDepthNotice("Need additional market data permissions - Depth: NASDAQ") {
		t.Fatal("isExactLiveNoMarketDepthNotice() accepted an unstructured substring")
	}
}

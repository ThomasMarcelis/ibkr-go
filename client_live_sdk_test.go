//go:build ibkr_sdk && cgo && linux

package ibkr_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
	"github.com/ThomasMarcelis/ibkr-go/testing/ibkrlive"
	"github.com/shopspring/decimal"
)

func TestLiveOfficialSDKSmoke(t *testing.T) {
	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	snapshot := client.Session()
	t.Logf("official SDK session ready: serverVersion=%d managedAccounts=%d nextValidID=%d",
		snapshot.ServerVersion,
		len(snapshot.ManagedAccounts),
		snapshot.NextValidID,
	)
	if snapshot.State != ibkr.StateReady {
		t.Fatalf("state = %s, want %s", snapshot.State, ibkr.StateReady)
	}
	if snapshot.ServerVersion == 0 {
		t.Fatal("server version is zero")
	}

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	currentTime, err := client.CurrentTime(ctx)
	if err != nil {
		t.Fatalf("CurrentTime() error = %v", err)
	}
	if currentTime.IsZero() {
		t.Fatal("CurrentTime() returned zero time")
	}
	t.Logf("official SDK currentTime received: %s", currentTime.Format(time.RFC3339))

	// Gateway serverVersion 203 answers only the first current-time-style
	// request on a session. CurrentTimeMillis has its own fresh-session live
	// smoke below.

	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("MarketData().SetType() error = %v", err)
	}
	t.Log("official SDK marketDataType set to delayed")

	values, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Account: "All",
		Tags:    []string{"NetLiquidation", "BuyingPower"},
	})
	if err != nil {
		t.Fatalf("AccountSummary() error = %v", err)
	}
	if len(values) == 0 {
		t.Fatal("AccountSummary() returned no rows")
	}
	t.Logf("official SDK accountSummary rows=%d", len(values))

	details, err := client.Contracts().Details(ctx, ibkr.Contract{
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	})
	if err != nil {
		t.Fatalf("ContractDetails() error = %v", err)
	}
	if len(details) == 0 {
		t.Fatal("ContractDetails() returned no rows")
	}
	t.Logf("official SDK contractDetails rows=%d", len(details))

	qualified, err := client.Contracts().Qualify(ctx, ibkr.Contract{
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	})
	if err != nil {
		t.Fatalf("Contracts().Qualify() error = %v", err)
	}
	if qualified.Contract.ConID != details[0].Contract.ConID {
		t.Fatalf("Contracts().Qualify() conID = %d, want %d", qualified.Contract.ConID, details[0].Contract.ConID)
	}
	t.Logf("official SDK qualify conID=%d", qualified.Contract.ConID)

	headTimestamp, err := client.History().HeadTimestamp(ctx, ibkr.HeadTimestampRequest{
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
		t.Fatalf("HeadTimestamp() error = %v", err)
	}
	if headTimestamp.IsZero() {
		t.Fatal("HeadTimestamp() returned zero time")
	}
	t.Logf("official SDK headTimestamp received: %s", headTimestamp.Format(time.RFC3339))

	histogramEntries, err := client.History().Histogram(ctx, ibkr.HistogramDataRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		UseRTH: true,
		Period: "1 week",
	})
	if err != nil {
		if liveCompetingSessionError(err) {
			t.Logf("official SDK histogram blocked by current TWS/Gateway session state: %v", err)
		} else {
			t.Fatalf("Histogram() error = %v", err)
		}
	} else if len(histogramEntries) == 0 {
		t.Fatal("Histogram() returned no rows")
	} else {
		t.Logf("official SDK histogram rows=%d", len(histogramEntries))
	}

	secDefOptParams, err := client.Contracts().SecDefOptParams(ctx, ibkr.SecDefOptParamsRequest{
		UnderlyingSymbol:  "AAPL",
		FutFopExchange:    "",
		UnderlyingSecType: ibkr.SecTypeStock,
		UnderlyingConID:   details[0].Contract.ConID,
	})
	if err != nil {
		t.Fatalf("SecDefOptParams() error = %v", err)
	}
	if len(secDefOptParams) == 0 {
		t.Fatal("SecDefOptParams() returned no rows")
	}
	t.Logf("official SDK secDefOptParams rows=%d", len(secDefOptParams))

	symbols, err := client.Contracts().Search(ctx, "AAPL")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(symbols) == 0 {
		t.Fatal("Search() returned no rows")
	}
	t.Logf("official SDK matchingSymbols rows=%d", len(symbols))

	marketRule, err := client.Contracts().MarketRule(ctx, 26)
	if err != nil {
		t.Fatalf("MarketRule() error = %v", err)
	}
	if len(marketRule.Increments) == 0 {
		t.Fatal("MarketRule() returned no increments")
	}
	t.Logf("official SDK marketRule increments=%d", len(marketRule.Increments))

	quote, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
	})
	if err != nil {
		if liveCompetingSessionError(err) {
			t.Logf("official SDK quote/smart-components path blocked by current competing live session: %v", err)
		} else {
			t.Fatalf("Quote() for smart components BBO exchange error = %v", err)
		}
	} else if quote.BBOExchange == "" {
		t.Fatal("Quote() returned empty BBO exchange")
	} else {
		t.Logf("official SDK quote BBO exchange=%q", quote.BBOExchange)

		smartComponents, err := client.Contracts().SmartComponents(ctx, quote.BBOExchange)
		if err != nil {
			t.Fatalf("SmartComponents() error = %v", err)
		}
		t.Logf("official SDK smartComponents rows=%d", len(smartComponents))
	}

	positions, err := client.Accounts().Positions(ctx)
	if err != nil {
		t.Fatalf("Positions() error = %v", err)
	}
	t.Logf("official SDK positions rows=%d", len(positions))

	familyCodes, err := client.Accounts().FamilyCodes(ctx)
	if err != nil {
		t.Fatalf("FamilyCodes() error = %v", err)
	}
	t.Logf("official SDK familyCodes rows=%d", len(familyCodes))

	depthExchanges, err := client.Contracts().DepthExchanges(ctx)
	if err != nil {
		t.Fatalf("DepthExchanges() error = %v", err)
	}
	if len(depthExchanges) == 0 {
		t.Fatal("DepthExchanges() returned no rows")
	}
	t.Logf("official SDK depthExchanges rows=%d", len(depthExchanges))

	newsProviders, err := client.News().Providers(ctx)
	if err != nil {
		t.Fatalf("NewsProviders() error = %v", err)
	}
	if len(newsProviders) == 0 {
		t.Fatal("NewsProviders() returned no rows")
	}
	t.Logf("official SDK newsProviders rows=%d", len(newsProviders))

	scannerParameters, err := client.Scanner().Parameters(ctx)
	if err != nil {
		t.Fatalf("ScannerParameters() error = %v", err)
	}
	if len(scannerParameters) == 0 {
		t.Fatal("ScannerParameters() returned empty XML")
	}
	t.Logf("official SDK scannerParameters bytes=%d", len(scannerParameters))

	softDollarTiers, err := client.Advisors().SoftDollarTiers(ctx)
	if err != nil {
		t.Fatalf("SoftDollarTiers() error = %v", err)
	}
	t.Logf("official SDK softDollarTiers rows=%d", len(softDollarTiers))

	whiteBrandingID, err := client.TWS().UserInfo(ctx)
	if err != nil {
		t.Fatalf("UserInfo() error = %v", err)
	}
	t.Logf("official SDK userInfo whiteBrandingID=%q", whiteBrandingID)

	displayGroups, err := client.TWS().DisplayGroups(ctx)
	if err != nil {
		t.Fatalf("DisplayGroups() error = %v", err)
	}
	t.Logf("official SDK displayGroups rows=%d", len(displayGroups))

	completedOrders, err := client.Orders().Completed(ctx, true)
	if err != nil {
		t.Fatalf("CompletedOrders() error = %v", err)
	}
	t.Logf("official SDK completedOrders rows=%d", len(completedOrders))

	executions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{})
	if err != nil {
		t.Fatalf("Executions() error = %v", err)
	}
	t.Logf("official SDK executions rows=%d", len(executions))

	wshMetaData, err := client.WSH().MetaData(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "does not support") {
			t.Fatalf("WSHMetaData() unsupported by SDK runtime: %v", err)
		}
		t.Logf("official SDK WSHMetaData returned expected entitlement/account error: %v", err)
	} else if len(wshMetaData) == 0 {
		t.Fatal("WSHMetaData() returned empty document")
	} else {
		t.Logf("official SDK WSHMetaData bytes=%d", len(wshMetaData))
	}

	wshEventData, err := client.WSH().EventData(ctx, ibkr.WSHEventDataRequest{ConID: 265598})
	if err != nil {
		if strings.Contains(err.Error(), "does not support") {
			t.Fatalf("WSHEventData() unsupported by SDK runtime: %v", err)
		}
		t.Logf("official SDK WSHEventData returned expected entitlement/account error: %v", err)
	} else if len(wshEventData) == 0 {
		t.Fatal("WSHEventData() returned empty document")
	} else {
		t.Logf("official SDK WSHEventData bytes=%d", len(wshEventData))
	}
}

func TestLiveOfficialSDKCurrentTimeMillis(t *testing.T) {
	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	currentTimeMillis, err := client.CurrentTimeMillis(ctx)
	if err != nil {
		t.Fatalf("CurrentTimeMillis() error = %v", err)
	}
	if currentTimeMillis.IsZero() {
		t.Fatal("CurrentTimeMillis() returned zero time")
	}
	t.Logf("official SDK currentTimeMillis received: %s", currentTimeMillis.Format(time.RFC3339Nano))
}

func TestLiveOfficialSDKHistoryAndFundamental(t *testing.T) {
	client, _, cancel := ibkrlive.DialContext(t, 20*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancelReq()

	aaplContract := ibkr.Contract{
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	}
	competingSessionSeen := false
	bars, err := client.History().Bars(ctx, ibkr.HistoricalBarsRequest{
		Contract:   aaplContract,
		Duration:   ibkr.Days(1),
		BarSize:    ibkr.Bar1Hour,
		WhatToShow: ibkr.ShowTrades,
		UseRTH:     true,
	})
	if err != nil {
		if liveCompetingSessionError(err) {
			competingSessionSeen = true
			t.Logf("official SDK historical bars blocked by current competing live session: %v", err)
		} else {
			t.Fatalf("History().Bars() error = %v", err)
		}
	} else if len(bars) == 0 {
		t.Fatal("History().Bars() returned no rows")
	} else {
		t.Logf("official SDK historical bars rows=%d", len(bars))
	}

	schedule, err := client.History().Schedule(ctx, ibkr.HistoricalScheduleRequest{
		Contract: aaplContract,
		Duration: ibkr.Months(1),
		BarSize:  ibkr.Bar1Day,
		UseRTH:   true,
	})
	if err != nil {
		if liveCompetingSessionError(err) {
			competingSessionSeen = true
			t.Logf("official SDK historical schedule blocked by current competing live session: %v", err)
		} else {
			t.Fatalf("History().Schedule() error = %v", err)
		}
	} else if len(schedule.Sessions) == 0 {
		t.Fatal("History().Schedule() returned no sessions")
	} else {
		t.Logf("official SDK historical schedule sessions=%d timezone=%q", len(schedule.Sessions), schedule.TimeZone)
	}

	eastern, err := time.LoadLocation("US/Eastern")
	if err != nil {
		t.Fatalf("LoadLocation(US/Eastern) error = %v", err)
	}
	ticks, err := client.History().Ticks(ctx, ibkr.HistoricalTicksRequest{
		Contract:      aaplContract,
		EndTime:       time.Date(2026, 5, 1, 16, 0, 0, 0, eastern),
		NumberOfTicks: 10,
		WhatToShow:    ibkr.ShowMidpoint,
		UseRTH:        true,
	})
	if err != nil {
		if liveCompetingSessionError(err) {
			competingSessionSeen = true
			t.Logf("official SDK historical midpoint ticks blocked by current competing live session: %v", err)
		} else {
			t.Fatalf("History().Ticks() error = %v", err)
		}
	} else if len(ticks.Ticks) == 0 {
		t.Fatal("History().Ticks() returned no midpoint ticks")
	} else {
		t.Logf("official SDK historical midpoint ticks rows=%d", len(ticks.Ticks))
	}

	fundamentalCtx, cancelFundamental := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelFundamental()
	fundamentalData, err := client.Contracts().FundamentalData(fundamentalCtx, ibkr.FundamentalDataRequest{
		Contract:   aaplContract,
		ReportType: ibkr.FundamentalReportSnapshot,
	})
	if err != nil {
		if competingSessionSeen && errors.Is(err, context.DeadlineExceeded) {
			t.Logf("official SDK fundamentalData timed out after competing-session historical data blockers: %v", err)
		} else {
			t.Fatalf("FundamentalData() error = %v", err)
		}
	} else if len(fundamentalData) == 0 {
		t.Fatal("FundamentalData() returned empty XML")
	} else {
		t.Logf("official SDK fundamentalData bytes=%d", len(fundamentalData))
	}
}

func TestLiveOfficialSDKReadOnlySubscriptions(t *testing.T) {
	client, _, cancel := ibkrlive.DialContext(t, 20*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelReq()

	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("MarketData().SetType() error = %v", err)
	}

	summarySub, err := client.Accounts().SubscribeSummary(ctx, ibkr.AccountSummaryRequest{
		Account: "All",
		Tags:    []string{"NetLiquidation", "BuyingPower"},
	})
	if err != nil {
		t.Fatalf("Accounts().SubscribeSummary() error = %v", err)
	}
	if err := summarySub.AwaitSnapshot(ctx); err != nil {
		t.Fatalf("Accounts().SubscribeSummary().AwaitSnapshot() error = %v", err)
	}
	summaryRows := drainLiveEvents(summarySub.Events())
	if summaryRows == 0 {
		t.Fatal("Accounts().SubscribeSummary() snapshot returned no rows")
	}
	t.Logf("official SDK account summary subscription rows=%d", summaryRows)
	closeLiveSubscription(t, ctx, summarySub)

	positionsSub, err := client.Accounts().SubscribePositions(ctx)
	if err != nil {
		t.Fatalf("Accounts().SubscribePositions() error = %v", err)
	}
	if err := positionsSub.AwaitSnapshot(ctx); err != nil {
		t.Fatalf("Accounts().SubscribePositions().AwaitSnapshot() error = %v", err)
	}
	positionRows := drainLiveEvents(positionsSub.Events())
	t.Logf("official SDK positions subscription rows=%d", positionRows)
	closeLiveSubscription(t, ctx, positionsSub)

	aaplContract := ibkr.Contract{
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	}
	quoteSub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{Contract: aaplContract})
	if err != nil {
		t.Fatalf("MarketData().SubscribeQuotes() error = %v", err)
	}
	quoteStreamActive := true
	if quoteUpdate, ok := waitLiveQuoteUpdate(t, ctx, quoteSub); ok {
		t.Logf("official SDK quote stream update changed=%v available=%v", quoteUpdate.Changed, quoteUpdate.Snapshot.Available)
	} else {
		quoteStreamActive = false
	}
	if quoteStreamActive {
		closeLiveSubscription(t, ctx, quoteSub)
	} else {
		_ = quoteSub.Close()
	}

	barsSub, err := client.History().SubscribeBars(ctx, ibkr.HistoricalBarsRequest{
		Contract:   aaplContract,
		Duration:   ibkr.Days(1),
		BarSize:    ibkr.Bar1Hour,
		WhatToShow: ibkr.ShowTrades,
		UseRTH:     true,
	})
	if err != nil {
		t.Fatalf("History().SubscribeBars() error = %v", err)
	}
	if err := barsSub.AwaitSnapshot(ctx); err != nil {
		if liveCompetingSessionError(err) {
			t.Logf("official SDK historical bars subscription blocked by current competing live session: %v", err)
			_ = barsSub.Close()
		} else {
			t.Fatalf("History().SubscribeBars().AwaitSnapshot() error = %v", err)
		}
	} else {
		barRows := drainLiveEvents(barsSub.Events())
		if barRows == 0 {
			t.Fatal("History().SubscribeBars() snapshot returned no rows")
		}
		t.Logf("official SDK historical bars subscription snapshot rows=%d", barRows)
		closeLiveSubscription(t, ctx, barsSub)
	}

	scannerSub, err := client.Scanner().SubscribeResults(ctx, ibkr.ScannerSubscriptionRequest{
		NumberOfRows: 5,
		Instrument:   "STK",
		LocationCode: "STK.US.MAJOR",
		ScanCode:     "TOP_PERC_GAIN",
	})
	if err != nil {
		t.Fatalf("Scanner().SubscribeResults() error = %v", err)
	}
	scannerActive := true
	if scannerRows, ok := waitLiveSubscriptionValue(t, ctx, scannerSub, "scanner subscription rows"); ok {
		if len(scannerRows) == 0 {
			t.Log("official SDK scanner subscription emitted an empty row set")
		} else {
			t.Logf("official SDK scanner subscription rows=%d", len(scannerRows))
		}
	} else {
		scannerActive = false
	}
	if scannerActive {
		closeLiveSubscription(t, ctx, scannerSub)
	} else {
		_ = scannerSub.Close()
	}
}

func TestLiveOfficialSDKAccountStreams(t *testing.T) {
	client, _, cancel := ibkrlive.DialContext(t, 20*time.Second)
	defer cancel()
	defer client.Close()

	account := liveAccount(t, client)
	ctx, cancelReq := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancelReq()

	updatesSub, err := client.Accounts().SubscribeUpdates(ctx, account)
	if err != nil {
		t.Fatalf("Accounts().SubscribeUpdates() error = %v", err)
	}
	if err := updatesSub.AwaitSnapshot(ctx); err != nil {
		t.Fatalf("Accounts().SubscribeUpdates().AwaitSnapshot() error = %v", err)
	}
	updateRows := drainLiveEvents(updatesSub.Events())
	if updateRows == 0 {
		t.Fatal("Accounts().SubscribeUpdates() snapshot returned no rows")
	}
	t.Logf("official SDK account updates subscription rows=%d", updateRows)
	closeLiveSubscription(t, ctx, updatesSub)

	updatesMultiSub, err := client.Accounts().SubscribeUpdatesMulti(ctx, ibkr.AccountUpdatesMultiRequest{Account: account})
	if err != nil {
		t.Fatalf("Accounts().SubscribeUpdatesMulti() error = %v", err)
	}
	if err := updatesMultiSub.AwaitSnapshot(ctx); err != nil {
		t.Fatalf("Accounts().SubscribeUpdatesMulti().AwaitSnapshot() error = %v", err)
	}
	updateMultiRows := drainLiveEvents(updatesMultiSub.Events())
	if updateMultiRows == 0 {
		t.Fatal("Accounts().SubscribeUpdatesMulti() snapshot returned no rows")
	}
	t.Logf("official SDK account updates multi subscription rows=%d", updateMultiRows)
	closeLiveSubscription(t, ctx, updatesMultiSub)

	positionsMultiSub, err := client.Accounts().SubscribePositionsMulti(ctx, ibkr.PositionsMultiRequest{Account: account})
	if err != nil {
		t.Fatalf("Accounts().SubscribePositionsMulti() error = %v", err)
	}
	if err := positionsMultiSub.AwaitSnapshot(ctx); err != nil {
		t.Fatalf("Accounts().SubscribePositionsMulti().AwaitSnapshot() error = %v", err)
	}
	positionMultiRows := drainLiveEvents(positionsMultiSub.Events())
	t.Logf("official SDK positions multi subscription rows=%d", positionMultiRows)
	closeLiveSubscription(t, ctx, positionsMultiSub)

	pnlSub, err := client.Accounts().SubscribePnL(ctx, ibkr.PnLRequest{Account: account})
	if err != nil {
		t.Fatalf("Accounts().SubscribePnL() error = %v", err)
	}
	_ = waitLiveEvent(t, ctx, pnlSub.Events(), "PnL stream update")
	t.Log("official SDK PnL subscription emitted an update")
	closeLiveSubscription(t, ctx, pnlSub)

	positions, err := client.Accounts().Positions(ctx)
	if err != nil {
		t.Fatalf("Accounts().Positions() for PnLSingle conID error = %v", err)
	}
	conID := firstLivePositionConID(positions)
	if conID == 0 {
		t.Skip("PnLSingle live probe needs at least one live position with a conID")
	}
	pnlSingleSub, err := client.Accounts().SubscribePnLSingle(ctx, ibkr.PnLSingleRequest{
		Account: account,
		ConID:   conID,
	})
	if err != nil {
		t.Fatalf("Accounts().SubscribePnLSingle() error = %v", err)
	}
	_ = waitLiveEvent(t, ctx, pnlSingleSub.Events(), "PnLSingle stream update")
	t.Log("official SDK PnLSingle subscription emitted an update")
	closeLiveSubscription(t, ctx, pnlSingleSub)
}

func TestLiveOfficialSDKPaperOrderPlaceCancel(t *testing.T) {
	ibkrlive.RequireTrading(t)

	client, _, cancel := ibkrlive.DialContext(t, 20*time.Second)
	defer cancel()
	defer client.Close()

	account := requireLivePaperAccount(t, client)
	ctx, cancelReq := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancelReq()

	contract := ibkr.Contract{
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	}
	limitPrice := conservativeLiveBuyLimit(t, ctx, client, contract)
	transmit := true
	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: contract,
		Order: ibkr.Order{
			Action:    ibkr.Buy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.NewFromInt(1),
			LmtPrice:  limitPrice,
			TIF:       ibkr.TIFDay,
			Account:   account,
			Transmit:  &transmit,
			OrderRef:  "ibkr-go-sdk-live-" + time.Now().UTC().Format("20060102T150405"),
		},
	})
	if err != nil {
		t.Fatalf("Orders().Place() error = %v", err)
	}
	cancelSent := false
	defer func() {
		if !cancelSent {
			cancelCtx, cancelCleanup := context.WithTimeout(context.Background(), 10*time.Second)
			_ = handle.Cancel(cancelCtx)
			cancelCleanup()
		}
		_ = handle.Close()
	}()
	t.Logf("official SDK paper order sent: orderID=%d action=BUY quantity=1 orderType=LMT", handle.OrderID())

	waitForLiveOrderAccepted(t, ctx, handle)
	assertLiveOpenOrdersContains(t, ctx, client, handle.OrderID())
	assertLiveSubscribeOpenContains(t, ctx, client, handle.OrderID())
	if err := handle.Cancel(ctx); err != nil {
		t.Fatalf("OrderHandle.Cancel() error = %v", err)
	}
	cancelSent = true
	t.Logf("official SDK paper order cancel sent: orderID=%d", handle.OrderID())
	waitForLiveOrderCancelled(t, ctx, handle)
}

func TestLiveOfficialSDKPaperOrderModifyCancel(t *testing.T) {
	ibkrlive.RequireTrading(t)

	client, _, cancel := ibkrlive.DialContext(t, 20*time.Second)
	defer cancel()
	defer client.Close()

	account := requireLivePaperAccount(t, client)
	ctx, cancelReq := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancelReq()

	contract := ibkr.Contract{
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	}
	limit := conservativeLiveBuyLimit(t, ctx, client, contract)
	transmit := true
	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: contract,
		Order: ibkr.Order{
			Action:    ibkr.Buy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.NewFromInt(1),
			LmtPrice:  limit,
			TIF:       ibkr.TIFDay,
			Account:   account,
			Transmit:  &transmit,
			OrderRef:  "ibkr-go-sdk-live-mod-" + time.Now().UTC().Format("20060102T150405"),
		},
	})
	if err != nil {
		t.Fatalf("Orders().Place() error = %v", err)
	}
	cancelSent := false
	defer func() {
		if !cancelSent {
			cancelCtx, cancelCleanup := context.WithTimeout(context.Background(), 10*time.Second)
			_ = handle.Cancel(cancelCtx)
			cancelCleanup()
		}
		_ = handle.Close()
	}()
	t.Logf("official SDK paper order sent for modify: orderID=%d action=BUY quantity=1 orderType=LMT", handle.OrderID())

	waitForLiveOrderAccepted(t, ctx, handle)
	if err := handle.Modify(ctx, ibkr.Order{
		Action:    ibkr.Buy,
		OrderType: ibkr.OrderTypeLimit,
		Quantity:  decimal.NewFromInt(2),
		LmtPrice:  limit,
		TIF:       ibkr.TIFDay,
		Account:   account,
		Transmit:  &transmit,
		OrderRef:  "ibkr-go-sdk-live-mod-" + time.Now().UTC().Format("20060102T150405"),
	}); err != nil {
		t.Fatalf("OrderHandle.Modify() error = %v", err)
	}
	t.Logf("official SDK paper order modify sent: orderID=%d", handle.OrderID())
	waitForLiveOrderQuantity(t, ctx, handle, decimal.NewFromInt(2))

	if err := handle.Cancel(ctx); err != nil {
		t.Fatalf("OrderHandle.Cancel() error = %v", err)
	}
	cancelSent = true
	t.Logf("official SDK paper modified order cancel sent: orderID=%d", handle.OrderID())
	waitForLiveOrderCancelled(t, ctx, handle)
}

func TestLiveOfficialSDKPaperGlobalCancel(t *testing.T) {
	ibkrlive.RequireTrading(t)

	client, _, cancel := ibkrlive.DialContext(t, 20*time.Second)
	defer cancel()
	defer client.Close()

	account := requireLivePaperAccount(t, client)
	ctx, cancelReq := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancelReq()

	existing, err := client.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
	if err != nil {
		t.Fatalf("preflight Orders().Open(all) error = %v", err)
	}
	if len(existing) != 0 {
		if !cancelLiveTestOrders(t, ctx, client, existing) {
			t.Skipf("refusing global cancel test because %d non-test open orders already exist", len(existing))
		}
		existing, err = client.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
		if err != nil {
			t.Fatalf("post-cleanup Orders().Open(all) error = %v", err)
		}
		if len(existing) != 0 {
			t.Skipf("refusing global cancel test because %d open orders remain after test-order cleanup", len(existing))
		}
	}

	contract := ibkr.Contract{
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	}
	limit := conservativeLiveBuyLimit(t, ctx, client, contract)
	transmit := true
	handles := make([]*ibkr.OrderHandle, 0, 2)
	cancelSent := false
	defer func() {
		if !cancelSent {
			cancelCtx, cancelCleanup := context.WithTimeout(context.Background(), 15*time.Second)
			for _, handle := range handles {
				_ = handle.Cancel(cancelCtx)
			}
			cancelCleanup()
		}
		for _, handle := range handles {
			_ = handle.Close()
		}
	}()

	for i := 0; i < 2; i++ {
		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: contract,
			Order: ibkr.Order{
				Action:    ibkr.Buy,
				OrderType: ibkr.OrderTypeLimit,
				Quantity:  decimal.NewFromInt(1),
				LmtPrice:  limit,
				TIF:       ibkr.TIFDay,
				Account:   account,
				Transmit:  &transmit,
				OrderRef:  "ibkr-go-sdk-live-global-" + time.Now().UTC().Format("20060102T150405"),
			},
		})
		if err != nil {
			t.Fatalf("Orders().Place(%d) error = %v", i, err)
		}
		handles = append(handles, handle)
		t.Logf("official SDK paper global-cancel order sent: orderID=%d", handle.OrderID())
		waitForLiveOrderAccepted(t, ctx, handle)
	}

	if err := client.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("Orders().CancelAll() error = %v", err)
	}
	cancelSent = true
	t.Log("official SDK paper global cancel sent")
	for _, handle := range handles {
		waitForLiveOrderCancelled(t, ctx, handle)
	}
}

func cancelLiveTestOrders(t *testing.T, ctx context.Context, client *ibkr.Client, orders []ibkr.OpenOrder) bool {
	t.Helper()
	for _, order := range orders {
		if !strings.HasPrefix(order.OrderRef, "ibkr-go-sdk-") {
			return false
		}
	}
	for _, order := range orders {
		if err := client.Orders().Cancel(ctx, order.OrderID); err != nil {
			t.Fatalf("cleanup cancel orderID=%d error = %v", order.OrderID, err)
		}
		t.Logf("official SDK paper cleanup cancel sent for stale test orderID=%d", order.OrderID)
	}
	waitLiveOpenOrdersCleared(t, ctx, client)
	return true
}

func waitLiveOpenOrdersCleared(t *testing.T, ctx context.Context, client *ibkr.Client) {
	t.Helper()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		orders, err := client.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
		if err != nil {
			t.Fatalf("cleanup Orders().Open(all) error = %v", err)
		}
		if len(orders) == 0 {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("timeout waiting for stale test open orders to clear: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}

func requireLivePaperAccount(t *testing.T, client *ibkr.Client) string {
	t.Helper()
	accounts := client.Session().ManagedAccounts
	if len(accounts) == 0 {
		t.Fatal("live trading test refused to place order: session has no managed accounts")
	}
	for _, account := range accounts {
		if !strings.HasPrefix(account, "DU") {
			t.Fatalf("live trading test refused to place order: managed account does not look like an IBKR paper account")
		}
	}
	return accounts[0]
}

func conservativeLiveBuyLimit(t *testing.T, ctx context.Context, client *ibkr.Client, contract ibkr.Contract) decimal.Decimal {
	t.Helper()
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("MarketData().SetType(delayed) error = %v", err)
	}
	quote, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: contract})
	if err != nil {
		t.Fatalf("Quote() for paper order reference price error = %v", err)
	}

	reference := firstPositiveDecimal(quote.Bid, quote.Last, quote.Close, quote.Ask)
	if reference.IsZero() {
		t.Fatalf("Quote() returned no positive bid/last/close/ask for paper order reference")
	}
	limit := reference.Mul(decimal.NewFromInt(9)).Div(decimal.NewFromInt(10)).Round(2)
	minimum := decimal.NewFromInt(1).Div(decimal.NewFromInt(100))
	if limit.LessThan(minimum) {
		return minimum
	}
	return limit
}

func firstPositiveDecimal(values ...decimal.Decimal) decimal.Decimal {
	for _, value := range values {
		if value.GreaterThan(decimal.Zero) {
			return value
		}
	}
	return decimal.Zero
}

func waitLiveEvent[T any](t *testing.T, ctx context.Context, ch <-chan T, label string) T {
	t.Helper()
	select {
	case value, ok := <-ch:
		if !ok {
			t.Fatalf("%s channel closed before event", label)
		}
		return value
	case <-ctx.Done():
		t.Fatalf("timeout waiting for %s: %v", label, ctx.Err())
	}
	var zero T
	return zero
}

func waitLiveQuoteUpdate(t *testing.T, ctx context.Context, sub *ibkr.Subscription[ibkr.QuoteUpdate]) (ibkr.QuoteUpdate, bool) {
	t.Helper()
	for {
		select {
		case update := <-sub.Events():
			if update.Changed == 0 {
				continue
			}
			return update, true
		case <-sub.Done():
			if err := sub.Wait(); err != nil {
				if liveCompetingSessionError(err) {
					t.Logf("official SDK quote stream blocked by current competing live session: %v", err)
					return ibkr.QuoteUpdate{}, false
				}
				t.Fatalf("quote stream subscription closed with error: %v", err)
			}
			t.Fatal("quote stream subscription closed before a non-empty update")
		case <-ctx.Done():
			if err := sub.Err(); err != nil && liveCompetingSessionError(err) {
				t.Logf("official SDK quote stream blocked by current competing live session: %v", err)
				return ibkr.QuoteUpdate{}, false
			}
			t.Fatalf("timeout waiting for quote stream update: %v", ctx.Err())
		}
	}
}

func waitLiveSubscriptionValue[T any](t *testing.T, ctx context.Context, sub *ibkr.Subscription[T], label string) (T, bool) {
	t.Helper()
	select {
	case value := <-sub.Events():
		return value, true
	case <-sub.Done():
		if err := sub.Wait(); err != nil {
			t.Fatalf("%s subscription closed with error: %v", label, err)
		}
		t.Fatalf("%s subscription closed before event", label)
	case <-ctx.Done():
		t.Fatalf("timeout waiting for %s: %v", label, ctx.Err())
	}
	var zero T
	return zero, false
}

func drainLiveEvents[T any](ch <-chan T) int {
	count := 0
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return count
			}
			count++
		default:
			return count
		}
	}
}

func firstLivePositionConID(positions []ibkr.Position) int {
	for _, position := range positions {
		if position.Contract.ConID != 0 && !position.Position.IsZero() {
			return position.Contract.ConID
		}
	}
	for _, position := range positions {
		if position.Contract.ConID != 0 {
			return position.Contract.ConID
		}
	}
	return 0
}

func liveCompetingSessionError(err error) bool {
	apiErr, ok := errors.AsType[*ibkr.APIError](err)
	if !ok {
		return false
	}
	switch apiErr.Code {
	case 10188, 10197:
		return true
	case 10187:
		return strings.Contains(apiErr.Message, "Trading TWS session is connected from a different IP address")
	case 162, 165:
		return strings.Contains(apiErr.Message, "Trading TWS session is connected from a different IP address")
	default:
		return false
	}
}

func closeLiveSubscription[T any](t *testing.T, ctx context.Context, sub *ibkr.Subscription[T]) {
	t.Helper()
	if err := sub.Close(); err != nil {
		t.Fatalf("subscription Close() error = %v", err)
	}
	select {
	case <-sub.Done():
		if err := sub.Wait(); err != nil {
			if liveCompetingSessionError(err) {
				t.Logf("subscription closed under current competing live session: %v", err)
				return
			}
			t.Fatalf("subscription Wait() error = %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("timeout waiting for subscription close: %v", ctx.Err())
	}
}

func waitForLiveOrderAccepted(t *testing.T, ctx context.Context, handle *ibkr.OrderHandle) {
	t.Helper()
	for {
		select {
		case event, ok := <-handle.Events():
			if !ok {
				if err := handle.Wait(); err != nil {
					t.Fatalf("order handle closed before live acceptance: %v", err)
				}
				t.Fatal("order handle closed before live acceptance")
			}
			if event.OpenOrder != nil {
				t.Logf("official SDK paper order openOrder observed: orderID=%d status=%s", handle.OrderID(), event.OpenOrder.Status)
				return
			}
			if event.Status == nil {
				continue
			}
			t.Logf("official SDK paper order status observed: orderID=%d status=%s", handle.OrderID(), event.Status.Status)
			switch event.Status.Status {
			case ibkr.OrderStatusPendingSubmit, ibkr.OrderStatusPreSubmitted, ibkr.OrderStatusSubmitted:
				return
			case ibkr.OrderStatusFilled:
				t.Fatalf("paper safety order filled before cancellation")
			case ibkr.OrderStatusCancelled, ibkr.OrderStatusApiCancelled, ibkr.OrderStatusInactive:
				t.Fatalf("paper order reached terminal status before cancellation: %s", event.Status.Status)
			}
		case <-ctx.Done():
			t.Fatalf("timeout waiting for live order acceptance: %v", ctx.Err())
		}
	}
}

func assertLiveOpenOrdersContains(t *testing.T, ctx context.Context, client *ibkr.Client, orderID int64) {
	t.Helper()
	orders, err := client.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
	if err != nil {
		t.Fatalf("Orders().Open(all) error = %v", err)
	}
	for _, order := range orders {
		if order.OrderID == orderID {
			t.Logf("official SDK paper open-orders snapshot observed orderID=%d status=%s", orderID, order.Status)
			return
		}
	}
	t.Fatalf("Orders().Open(all) returned %d rows without orderID=%d", len(orders), orderID)
}

func assertLiveSubscribeOpenContains(t *testing.T, ctx context.Context, client *ibkr.Client, orderID int64) {
	t.Helper()
	sub, err := client.Orders().SubscribeOpen(ctx, ibkr.OpenOrdersScopeAll)
	if err != nil {
		t.Fatalf("Orders().SubscribeOpen(all) error = %v", err)
	}
	defer func() {
		_ = sub.Close()
	}()

	sawOrder := false
	for {
		select {
		case update, ok := <-sub.Events():
			if !ok {
				if err := sub.Wait(); err != nil {
					t.Fatalf("SubscribeOpen(all) closed before snapshot with error: %v", err)
				}
				t.Fatal("SubscribeOpen(all) closed before snapshot")
			}
			if update.Order.OrderID == orderID {
				sawOrder = true
				t.Logf("official SDK paper subscribe-open observed orderID=%d status=%s", orderID, update.Order.Status)
			}
		case state, ok := <-sub.Lifecycle():
			if !ok {
				if !sawOrder {
					t.Fatal("SubscribeOpen(all) lifecycle closed before observing active order")
				}
				return
			}
			if state.Kind == ibkr.SubscriptionSnapshotComplete {
				if !sawOrder {
					sawOrder = drainLiveSubscribeOpenEvents(t, sub.Events(), orderID)
				}
				if !sawOrder {
					t.Fatal("SubscribeOpen(all) reached snapshot complete without active order")
				}
				return
			}
		case <-ctx.Done():
			t.Fatalf("timeout waiting for subscribe-open snapshot: %v", ctx.Err())
		}
	}
}

func drainLiveSubscribeOpenEvents(t *testing.T, events <-chan ibkr.OpenOrderUpdate, orderID int64) bool {
	t.Helper()
	for {
		select {
		case update, ok := <-events:
			if !ok {
				return false
			}
			if update.Order.OrderID == orderID {
				t.Logf("official SDK paper subscribe-open observed orderID=%d status=%s", orderID, update.Order.Status)
				return true
			}
		default:
			return false
		}
	}
}

func waitForLiveOrderCancelled(t *testing.T, ctx context.Context, handle *ibkr.OrderHandle) {
	t.Helper()
	sawCancel := false
	for {
		select {
		case event, ok := <-handle.Events():
			if !ok {
				if err := handle.Wait(); err != nil {
					t.Fatalf("order handle closed after cancel with error: %v", err)
				}
				if !sawCancel {
					t.Fatal("order handle closed before cancelled status")
				}
				return
			}
			if event.Status == nil {
				continue
			}
			t.Logf("official SDK paper order status observed after cancel: orderID=%d status=%s", handle.OrderID(), event.Status.Status)
			switch event.Status.Status {
			case ibkr.OrderStatusCancelled, ibkr.OrderStatusApiCancelled:
				sawCancel = true
			case ibkr.OrderStatusFilled:
				t.Fatalf("paper safety order filled before cancellation completed")
			case ibkr.OrderStatusInactive:
				t.Fatalf("paper order became inactive after cancel instead of cancelled")
			}
		case <-handle.Done():
			if err := handle.Wait(); err != nil {
				t.Fatalf("order handle completed after cancel with error: %v", err)
			}
			if !sawCancel {
				t.Fatal("order handle completed before cancelled status")
			}
			return
		case <-ctx.Done():
			t.Fatalf("timeout waiting for live order cancellation: %v", ctx.Err())
		}
	}
}

func waitForLiveOrderQuantity(t *testing.T, ctx context.Context, handle *ibkr.OrderHandle, want decimal.Decimal) {
	t.Helper()
	for {
		select {
		case event, ok := <-handle.Events():
			if !ok {
				if err := handle.Wait(); err != nil {
					t.Fatalf("order handle closed before modified openOrder echo: %v", err)
				}
				t.Fatal("order handle closed before modified openOrder echo")
			}
			if event.OpenOrder != nil {
				t.Logf("official SDK paper order openOrder observed after modify: orderID=%d status=%s quantity=%s", handle.OrderID(), event.OpenOrder.Status, event.OpenOrder.Quantity)
				if event.OpenOrder.Quantity.Equal(want) {
					return
				}
			}
			if event.Status == nil {
				continue
			}
			t.Logf("official SDK paper order status observed after modify: orderID=%d status=%s", handle.OrderID(), event.Status.Status)
			switch event.Status.Status {
			case ibkr.OrderStatusFilled:
				t.Fatalf("paper safety order filled before modified openOrder echo")
			case ibkr.OrderStatusCancelled, ibkr.OrderStatusApiCancelled, ibkr.OrderStatusInactive:
				t.Fatalf("paper order reached terminal status before modified openOrder echo: %s", event.Status.Status)
			}
		case <-ctx.Done():
			t.Fatalf("timeout waiting for modified openOrder echo: %v", ctx.Err())
		}
	}
}

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

func TestLiveAccountSummary(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()

	values, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Account: "All",
		Tags:    []string{"NetLiquidation", "BuyingPower"},
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
		if isLiveHistoricalSessionError(err) {
			t.Logf("HistoricalBars() returned: %v (current Gateway historical data session constraint)", err)
			return
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
		if isLiveHistoricalSessionError(err) {
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
		if isLiveHistoricalSessionError(err) {
			t.Logf("HistoricalSchedule() returned: %v (current Gateway historical data session constraint)", err)
			return
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

	// Short timeout: accounts without real-time market data subscriptions
	// may need OutReqMarketDataType(3) sent first to receive delayed data.
	// Without that, the snapshot may time out.
	ctx, cancelReq := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelReq()

	quote, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{
		Contract: aaplContract,
	})
	if err != nil {
		t.Logf("QuoteSnapshot() returned: %v (may need delayed data mode setup)", err)
		return
	}
	t.Logf("QuoteSnapshot: Available=%d, Bid=%s, Ask=%s, Last=%s",
		quote.Available, quote.Bid, quote.Ask, quote.Last)
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

func TestLiveExecutions(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	// Short timeout: the IB Gateway may not send ExecutionsEnd for
	// read-only accounts with no recent executions.
	ctx, cancelReq := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelReq()

	updates, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{})
	if err != nil {
		// Timeout is acceptable when the account has no executions.
		t.Logf("Executions() returned: %v (acceptable for read-only/empty accounts)", err)
		return
	}
	t.Logf("Executions: %d updates", len(updates))
}

func TestLiveSubscribeAccountSummary(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 20*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelReq()

	sub, err := client.Accounts().SubscribeSummary(ctx, ibkr.AccountSummaryRequest{
		Account: "All",
		Tags:    []string{"NetLiquidation", "BuyingPower"},
	}, ibkr.WithResumePolicy(ibkr.ResumeNever))
	if err != nil {
		t.Fatalf("SubscribeAccountSummary() error = %v", err)
	}
	defer sub.Close()

	var events int
	deadline := time.After(15 * time.Second)
	for {
		select {
		case _, ok := <-sub.Events():
			if !ok {
				t.Fatal("Events channel closed before SnapshotComplete")
			}
			events++
		case evt, ok := <-sub.Lifecycle():
			if !ok {
				t.Fatal("State channel closed unexpectedly")
			}
			if evt.Kind == ibkr.SubscriptionSnapshotComplete {
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

func TestLiveFundamentalData(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()

	data, err := client.Contracts().FundamentalData(ctx, ibkr.FundamentalDataRequest{
		Contract:   aaplContract,
		ReportType: "ReportSnapshot",
	})
	if err != nil {
		// Fundamental data may require a subscription not available on paper accounts.
		t.Logf("FundamentalData() returned: %v (may need subscription)", err)
		return
	}
	if len(data) == 0 {
		t.Fatal("FundamentalData() returned empty document")
	}
	t.Logf("FundamentalData: %d bytes", len(data))
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
		// Paper accounts without news subscriptions return error 10276.
		t.Logf("WSHMetaData() returned: %v (expected on paper accounts without news feed)", err)
		return
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
		// Non-FA accounts return an error; this is expected on paper accounts.
		t.Logf("RequestFA() returned: %v (expected on non-FA accounts)", err)
		return
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
		IsSmartDepth: true,
	}, ibkr.WithResumePolicy(ibkr.ResumeNever))
	if err != nil {
		// Deep market data may not be available on all paper accounts.
		t.Logf("SubscribeMarketDepth() returned: %v (may need market data subscription)", err)
		return
	}
	defer sub.Close()

	// Read a few depth events or accept timeout.
	var events int
	deadline := time.After(10 * time.Second)
	for {
		select {
		case _, ok := <-sub.Events():
			if !ok {
				t.Logf("SubscribeMarketDepth: events channel closed after %d events", events)
				return
			}
			events++
			if events >= 5 {
				t.Logf("SubscribeMarketDepth: received %d depth rows", events)
				return
			}
		case evt := <-sub.Lifecycle():
			if evt.Err != nil {
				// Market depth errors (e.g. 10092, 2152) arrive as subscription errors.
				t.Logf("SubscribeMarketDepth state error: %v (expected without deep data subscription)", evt.Err)
				return
			}
		case <-deadline:
			t.Logf("SubscribeMarketDepth: timed out after %d events (may need market data subscription)", events)
			return
		}
	}
}

func TestLivePlaceOrderLimitAndCancel(t *testing.T) {
	ibkrlive.RequireTrading(t)

	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	// Safety: cancel all orders on cleanup.
	defer func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanCancel()
		_ = client.Orders().CancelAll(cleanCtx)
	}()

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplContract,
		Order: ibkr.Order{
			Action:    ibkr.Buy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  decimal.RequireFromString("50"),
			TIF:       ibkr.TIFDay,
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	t.Logf("PlaceOrder: orderID=%d", handle.OrderID())

	// Wait for any status before cancelling.
	var sawStatus bool
	for !sawStatus {
		select {
		case evt := <-handle.Events():
			if evt.Status != nil {
				t.Logf("order status: %s", evt.Status.Status)
				sawStatus = true
			}
		case <-ctx.Done():
			t.Log("timeout waiting for initial status; cancelling live paper order anyway")
			cancelCtx, cancelOrder := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancelOrder()
			if err := handle.Cancel(cancelCtx); err != nil {
				t.Logf("Cancel after status timeout: %v", err)
			}
			return
		}
	}

	if err := handle.Cancel(ctx); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	// Wait for terminal status.
	for {
		select {
		case evt := <-handle.Events():
			if evt.Status != nil {
				t.Logf("order status after cancel: %s", evt.Status.Status)
				if evt.Status.Status == "Cancelled" || evt.Status.Status == "Inactive" {
					return
				}
			}
		case <-handle.Done():
			return
		case <-ctx.Done():
			t.Fatal("timeout waiting for cancelled status")
		}
	}
}

func TestLiveGlobalCancel(t *testing.T) {
	ibkrlive.RequireTrading(t)

	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	// Place two far-from-market limit orders.
	var handles []*ibkr.OrderHandle
	for i := 0; i < 2; i++ {
		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: aaplContract,
			Order: ibkr.Order{
				Action:    ibkr.Buy,
				OrderType: ibkr.OrderTypeLimit,
				Quantity:  decimal.RequireFromString("1"),
				LmtPrice:  decimal.RequireFromString("50"),
				TIF:       ibkr.TIFDay,
			},
		})
		if err != nil {
			t.Fatalf("PlaceOrder[%d]: %v", i, err)
		}
		handles = append(handles, handle)
		t.Logf("PlaceOrder[%d]: orderID=%d", i, handle.OrderID())
	}

	// Wait for at least one status event on each handle.
	for i, h := range handles {
		select {
		case evt := <-h.Events():
			if evt.Status != nil {
				t.Logf("handle[%d] initial status: %s", i, evt.Status.Status)
			}
		case <-ctx.Done():
			t.Fatalf("timeout waiting for handle[%d] initial status", i)
		}
	}

	if err := client.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("GlobalCancel: %v", err)
	}

	// Wait for all handles to reach terminal state.
	for i, h := range handles {
		select {
		case <-h.Done():
			t.Logf("handle[%d] done", i)
		case <-ctx.Done():
			t.Fatalf("timeout waiting for handle[%d] to finish after GlobalCancel", i)
		}
	}
}

func TestLiveTradingSplitBuySellExecutionRoundTrip(t *testing.T) {
	ibkrlive.RequireTrading(t)

	client, _, cancel := ibkrlive.DialContext(t, 90*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancelReq()

	defer func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		_ = client.Orders().CancelAll(cleanCtx)
	}()

	snapshot := client.Session()
	if len(snapshot.ManagedAccounts) == 0 {
		t.Fatal("no managed accounts in live session")
	}
	account := snapshot.ManagedAccounts[0]

	beforeSummary, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Account: account,
		Tags:    []string{"NetLiquidation", "TotalCashValue", "BuyingPower"},
	})
	if err != nil {
		t.Fatalf("pre-trade account summary: %v", err)
	}
	t.Logf("pre-trade account summary values: %d", len(beforeSummary))

	var filledBuys int
	for i := 0; i < 2; i++ {
		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: aaplContract,
			Order: ibkr.Order{
				Action:    ibkr.Buy,
				OrderType: ibkr.OrderTypeMarket,
				Quantity:  decimal.RequireFromString("1"),
				TIF:       ibkr.TIFDay,
				Account:   account,
			},
		})
		if err != nil {
			t.Fatalf("split buy[%d]: %v", i, err)
		}
		filled, sawExecution := waitLiveOrderFill(t, ctx, handle, "split buy")
		t.Logf("split buy[%d]: filled=%v saw_execution=%v", i, filled, sawExecution)
		if filled {
			filledBuys++
		}
	}

	if filledBuys == 0 {
		t.Log("no market fills observed; market may be closed or the account may be holding orders")
		return
	}

	executions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{
		Account: account,
		Symbol:  "AAPL",
	})
	if err != nil {
		t.Fatalf("executions after split buys: %v", err)
	}
	if len(executions) == 0 {
		t.Fatal("executions after split buys = 0, want at least one execution/commission update")
	}

	for i := 0; i < filledBuys; i++ {
		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: aaplContract,
			Order: ibkr.Order{
				Action:    ibkr.Sell,
				OrderType: ibkr.OrderTypeMarket,
				Quantity:  decimal.RequireFromString("1"),
				TIF:       ibkr.TIFDay,
				Account:   account,
			},
		})
		if err != nil {
			t.Fatalf("split sell[%d]: %v", i, err)
		}
		filled, sawExecution := waitLiveOrderFill(t, ctx, handle, "split sell")
		if !filled {
			t.Fatalf("split sell[%d] did not fill after a buy fill", i)
		}
		t.Logf("split sell[%d]: saw_execution=%v", i, sawExecution)
	}

	positions, err := client.Accounts().Positions(ctx)
	if err != nil {
		t.Fatalf("positions after round trip: %v", err)
	}
	t.Logf("positions after round trip: %d", len(positions))

	completed, err := client.Orders().Completed(ctx, true)
	if err != nil {
		t.Fatalf("completed orders after round trip: %v", err)
	}
	t.Logf("completed orders after round trip: %d", len(completed))
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
		case _, ok := <-sub.Events():
			if !ok {
				t.Fatal("Events channel closed before SnapshotComplete")
			}
			events++
		case evt, ok := <-sub.Lifecycle():
			if !ok {
				t.Fatal("State channel closed unexpectedly")
			}
			if evt.Kind == ibkr.SubscriptionSnapshotComplete {
				t.Logf("SubscribePositions: %d events before SnapshotComplete", events)
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

func waitLiveOrderFill(t *testing.T, ctx context.Context, handle *ibkr.OrderHandle, label string) (bool, bool) {
	t.Helper()

	deadline := time.After(30 * time.Second)
	sawExecution := false
	for {
		select {
		case evt, ok := <-handle.Events():
			if !ok {
				return false, sawExecution
			}
			if evt.OpenOrder != nil {
				t.Logf("%s open order: orderID=%d status=%s", label, evt.OpenOrder.OrderID, evt.OpenOrder.Status)
			}
			if evt.Execution != nil {
				sawExecution = true
				t.Logf("%s execution: execID=%s side=%s shares=%s price=%s", label, evt.Execution.ExecID, evt.Execution.Side, evt.Execution.Shares, evt.Execution.Price)
			}
			if evt.Commission != nil {
				t.Logf("%s commission: execID=%s commission=%s currency=%s pnl=%s", label, evt.Commission.ExecID, evt.Commission.Commission, evt.Commission.Currency, evt.Commission.RealizedPNL)
			}
			if evt.Status != nil {
				t.Logf("%s status: %s filled=%s remaining=%s", label, evt.Status.Status, evt.Status.Filled, evt.Status.Remaining)
				if evt.Status.Status == ibkr.OrderStatusFilled {
					return true, sawExecution
				}
				if ibkr.IsTerminalOrderStatus(evt.Status.Status) {
					return false, sawExecution
				}
			}
		case <-handle.Done():
			return false, sawExecution
		case <-deadline:
			_ = handle.Cancel(ctx)
			return false, sawExecution
		case <-ctx.Done():
			t.Fatalf("%s: context done waiting for order fill: %v", label, ctx.Err())
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
		// Paper accounts without news subscriptions return error 10276.
		t.Logf("WSHEventData() returned: %v (expected on paper accounts without news feed)", err)
		return
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
	end := time.Now().UTC()
	start := end.AddDate(0, 0, -7)

	items, err := client.News().Historical(ctx, ibkr.HistoricalNewsRequest{
		ConID:         265598,
		ProviderCodes: []ibkr.NewsProviderCode{"BRFG", "BRFUPDN", "DJNL"},
		StartTime:     start,
		EndTime:       end,
		TotalResults:  20,
	})
	if err != nil {
		t.Fatalf("HistoricalNews() error = %v", err)
	}
	if len(items) == 0 {
		t.Fatal("HistoricalNews() returned 0 items")
	}
	t.Logf("HistoricalNews: %d items", len(items))
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
		if isLiveHistoricalSessionError(err) {
			t.Logf("HistoricalTicks() returned: %v (current Gateway historical data session constraint)", err)
			return
		}
		t.Fatalf("HistoricalTicks() error = %v", err)
	}
	if len(result.Ticks) == 0 {
		t.Fatal("HistoricalTicks() returned 0 ticks")
	}
	t.Logf("HistoricalTicks: %d ticks", len(result.Ticks))
}

func TestLiveCalcImpliedVolatility(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()

	result, err := client.Options().ImpliedVolatility(ctx, ibkr.CalcImpliedVolatilityRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeOption,
			Exchange: "SMART",
			Currency: "USD",
			Expiry:   time.Now().AddDate(0, 1, 0).Format("20060102"),
			Strike:   "200",
			Right:    ibkr.RightCall,
		},
		OptionPrice: decimal.RequireFromString("10"),
		UnderPrice:  decimal.RequireFromString("200"),
	})
	if err != nil {
		// Option calc may fail if contract is not found or data unavailable.
		t.Logf("CalcImpliedVolatility() returned: %v (may be expected)", err)
		return
	}
	t.Logf("CalcImpliedVolatility: %+v", result)
}

func TestLiveCalcOptionPrice(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()

	result, err := client.Options().Price(ctx, ibkr.CalcOptionPriceRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeOption,
			Exchange: "SMART",
			Currency: "USD",
			Expiry:   time.Now().AddDate(0, 1, 0).Format("20060102"),
			Strike:   "200",
			Right:    ibkr.RightCall,
		},
		Volatility: decimal.RequireFromString("0.3"),
		UnderPrice: decimal.RequireFromString("200"),
	})
	if err != nil {
		t.Logf("CalcOptionPrice() returned: %v (may be expected)", err)
		return
	}
	t.Logf("CalcOptionPrice: %+v", result)
}

func TestLivePlaceOrderModify(t *testing.T) {
	ibkrlive.RequireTrading(t)

	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	defer func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanCancel()
		_ = client.Orders().CancelAll(cleanCtx)
	}()

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplContract,
		Order: ibkr.Order{
			Action:    ibkr.Buy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  decimal.RequireFromString("50"),
			TIF:       ibkr.TIFDay,
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	t.Logf("PlaceOrder: orderID=%d", handle.OrderID())

	// Wait for initial status.
	select {
	case evt := <-handle.Events():
		if evt.Status != nil {
			t.Logf("initial status: %s", evt.Status.Status)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for initial status")
	}

	// Modify to $51.
	if err := handle.Modify(ctx, ibkr.Order{
		Action:    ibkr.Buy,
		OrderType: ibkr.OrderTypeLimit,
		Quantity:  decimal.RequireFromString("1"),
		LmtPrice:  decimal.RequireFromString("51"),
		TIF:       ibkr.TIFDay,
	}); err != nil {
		t.Fatalf("Modify: %v", err)
	}

	// Wait for an OpenOrder event reflecting the new price.
	var sawModified bool
	deadline := time.After(10 * time.Second)
	for !sawModified {
		select {
		case evt := <-handle.Events():
			if evt.OpenOrder != nil {
				t.Logf("OpenOrder after modify: lmt_price=%s", evt.OpenOrder.LmtPrice)
				if evt.OpenOrder.LmtPrice.String() == "51" {
					sawModified = true
				}
			}
			if evt.Status != nil {
				t.Logf("status after modify: %s", evt.Status.Status)
			}
		case <-handle.Done():
			// Order went terminal (e.g. Inactive after hours).
			t.Log("order reached terminal state before seeing modified OpenOrder")
			return
		case <-deadline:
			t.Fatal("timeout waiting for modified OpenOrder")
		}
	}

	if err := handle.Cancel(ctx); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	select {
	case <-handle.Done():
	case <-ctx.Done():
		t.Log("timeout waiting for cancelled state after live modify; global cleanup will cancel remaining paper orders")
	}
}

func TestLivePlaceOrderBracket(t *testing.T) {
	ibkrlive.RequireTrading(t)

	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	defer func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanCancel()
		_ = client.Orders().CancelAll(cleanCtx)
	}()

	// Parent: MKT BUY 1 AAPL (don't transmit yet).
	parent, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplContract,
		Order: ibkr.Order{
			Action:    ibkr.Buy,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("1"),
			TIF:       ibkr.TIFDay,
			Transmit:  new(false),
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder (parent): %v", err)
	}
	t.Logf("parent orderID=%d", parent.OrderID())

	// Take-profit: LMT SELL 1 @ $500 (far above market).
	tp, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplContract,
		Order: ibkr.Order{
			Action:    ibkr.Sell,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  decimal.RequireFromString("500"),
			TIF:       ibkr.TIFGTC,
			ParentID:  parent.OrderID(),
			Transmit:  new(false),
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder (take-profit): %v", err)
	}
	t.Logf("take-profit orderID=%d", tp.OrderID())

	// Stop-loss: STP SELL 1 @ $50 (far below market). Transmit=true triggers the bracket.
	sl, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplContract,
		Order: ibkr.Order{
			Action:    ibkr.Sell,
			OrderType: ibkr.OrderTypeStop,
			Quantity:  decimal.RequireFromString("1"),
			AuxPrice:  decimal.RequireFromString("50"),
			TIF:       ibkr.TIFGTC,
			ParentID:  parent.OrderID(),
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder (stop-loss): %v", err)
	}
	t.Logf("stop-loss orderID=%d", sl.OrderID())

	// Wait for all three handles to receive at least one event.
	handles := []*ibkr.OrderHandle{parent, tp, sl}
	names := []string{"parent", "take-profit", "stop-loss"}
	for i, h := range handles {
		select {
		case evt := <-h.Events():
			if evt.Status != nil {
				t.Logf("%s status: %s", names[i], evt.Status.Status)
			}
			if evt.OpenOrder != nil {
				t.Logf("%s open_order received", names[i])
			}
		case <-h.Done():
			t.Logf("%s reached terminal state immediately", names[i])
		case <-ctx.Done():
			t.Fatalf("timeout waiting for %s initial event", names[i])
		}
	}
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

	waitForStateKind(t, handle.Lifecycle(), ibkr.SubscriptionStarted)

	// Update the display group to AAPL's conID.
	if err := handle.Update(ctx, "265598"); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Wait for an update event or timeout.
	select {
	case evt, ok := <-handle.Events():
		if !ok {
			t.Fatal("Events channel closed")
		}
		t.Logf("DisplayGroupUpdate: %s", evt.ContractInfo)
	case <-time.After(5 * time.Second):
		t.Log("no display group update event received (may be expected)")
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
		if _, ok := errors.AsType[*ibkr.APIError](err); ok {
			t.Logf("SubscribeOpenOrders() API error: %v (may require clientID=0)", err)
			return
		}
		t.Fatalf("SubscribeOpenOrders() error = %v", err)
	}
	defer sub.Close()

	deadline := time.After(10 * time.Second)
	var events int
	for {
		select {
		case _, ok := <-sub.Events():
			if !ok {
				t.Fatalf("Events closed after %d events", events)
			}
			events++
		case evt := <-sub.Lifecycle():
			if evt.Kind == ibkr.SubscriptionSnapshotComplete {
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

func isLiveHistoricalSessionError(err error) bool {
	apiErr, ok := errors.AsType[*ibkr.APIError](err)
	return ok &&
		(apiErr.Code == 162 || apiErr.Code == 10187) &&
		strings.Contains(apiErr.Message, "Trading TWS session is connected from a different IP address")
}

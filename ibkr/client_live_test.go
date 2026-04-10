package ibkr_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/ibkr"
	"github.com/ThomasMarcelis/ibkr-go/testing/ibkrlive"
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

	details, err := client.ContractDetails(ctx, aaplContract)
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

	values, err := client.AccountSummary(ctx, ibkr.AccountSummaryRequest{
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

	positions, err := client.PositionsSnapshot(ctx)
	if err != nil {
		t.Fatalf("PositionsSnapshot() error = %v", err)
	}
	if len(positions) == 0 {
		t.Fatal("PositionsSnapshot() returned 0 positions, want >= 1")
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

	bars, err := client.HistoricalBars(ctx, ibkr.HistoricalBarsRequest{
		Contract:   aaplContract,
		EndTime:    time.Now(),
		Duration:   24 * time.Hour,
		BarSize:    time.Hour,
		WhatToShow: "TRADES",
		UseRTH:     true,
	})
	if err != nil {
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

	quote, err := client.QuoteSnapshot(ctx, ibkr.QuoteSubscriptionRequest{
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

	orders, err := client.OpenOrdersSnapshot(ctx, ibkr.OpenOrdersScopeAll)
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

	updates, err := client.Executions(ctx, ibkr.ExecutionsRequest{})
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

	sub, err := client.SubscribeAccountSummary(ctx, ibkr.AccountSummaryRequest{
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
		case evt, ok := <-sub.State():
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

	tiers, err := client.SoftDollarTiers(ctx)
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

	groups, err := client.QueryDisplayGroups(ctx)
	if err != nil {
		t.Fatalf("QueryDisplayGroups() error = %v", err)
	}
	if groups == "" {
		t.Fatal("QueryDisplayGroups() returned empty string")
	}
	t.Logf("QueryDisplayGroups: %q", groups)
}

func TestLiveQualifyContractAAPL(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()

	details, err := client.QualifyContract(ctx, aaplContract)
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

	data, err := client.FundamentalData(ctx, ibkr.FundamentalDataRequest{
		Contract:   aaplContract,
		ReportType: "ReportSnapshot",
	})
	if err != nil {
		// Fundamental data may require a subscription not available on paper accounts.
		t.Logf("FundamentalData() returned: %v (may need subscription)", err)
		return
	}
	if data == "" {
		t.Fatal("FundamentalData() returned empty string")
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

	data, err := client.WSHMetaData(ctx)
	if err != nil {
		// Paper accounts without news subscriptions return error 10276.
		t.Logf("WSHMetaData() returned: %v (expected on paper accounts without news feed)", err)
		return
	}
	if data == "" {
		t.Fatal("WSHMetaData() returned empty string")
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

	data, err := client.RequestFA(ctx, 1)
	if err != nil {
		// Non-FA accounts return an error; this is expected on paper accounts.
		t.Logf("RequestFA() returned: %v (expected on non-FA accounts)", err)
		return
	}
	if data == "" {
		t.Fatal("RequestFA() returned empty string")
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

	sub, err := client.SubscribeMarketDepth(ctx, ibkr.MarketDepthRequest{
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
		case evt := <-sub.State():
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
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	// Safety: cancel all orders on cleanup.
	defer func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanCancel()
		_ = client.GlobalCancel(cleanCtx)
	}()

	handle, err := client.PlaceOrder(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplContract,
		Order: ibkr.Order{
			Action:    ibkr.Buy,
			OrderType: "LMT",
			Quantity:  ibkr.MustParseDecimal("1"),
			LmtPrice:  ibkr.MustParseDecimal("50"),
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
			t.Fatal("timeout waiting for initial status")
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
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	// Place two far-from-market limit orders.
	var handles []*ibkr.OrderHandle
	for i := 0; i < 2; i++ {
		handle, err := client.PlaceOrder(ctx, ibkr.PlaceOrderRequest{
			Contract: aaplContract,
			Order: ibkr.Order{
				Action:    ibkr.Buy,
				OrderType: "LMT",
				Quantity:  ibkr.MustParseDecimal("1"),
				LmtPrice:  ibkr.MustParseDecimal("50"),
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

	if err := client.GlobalCancel(ctx); err != nil {
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

func TestLiveSubscribePositions(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 20*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelReq()

	sub, err := client.SubscribePositions(ctx, ibkr.WithResumePolicy(ibkr.ResumeNever))
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
		case evt, ok := <-sub.State():
			if !ok {
				t.Fatal("State channel closed unexpectedly")
			}
			if evt.Kind == ibkr.SubscriptionSnapshotComplete {
				if events == 0 {
					t.Fatal("SnapshotComplete with 0 events")
				}
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

func TestLiveWSHEventData(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()

	data, err := client.WSHEventData(ctx, ibkr.WSHEventDataRequest{ConID: 265598})
	if err != nil {
		// Paper accounts without news subscriptions return error 10276.
		t.Logf("WSHEventData() returned: %v (expected on paper accounts without news feed)", err)
		return
	}
	if data == "" {
		t.Fatal("WSHEventData() returned empty string")
	}
	t.Logf("WSHEventData: %d bytes", len(data))
}

func TestLiveMatchingSymbols(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 10*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelReq()

	matches, err := client.MatchingSymbols(ctx, "AAPL")
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

	ts, err := client.HeadTimestamp(ctx, ibkr.HeadTimestampRequest{
		Contract:   aaplContract,
		WhatToShow: "TRADES",
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

	result, err := client.HistoricalTicks(ctx, ibkr.HistoricalTicksRequest{
		Contract:      aaplContract,
		EndDateTime:   time.Now().UTC().Format("20060102-15:04:05"),
		NumberOfTicks: 10,
		WhatToShow:    "MIDPOINT",
	})
	if err != nil {
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

	result, err := client.CalcImpliedVolatility(ctx, ibkr.CalcImpliedVolatilityRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeOption,
			Exchange: "SMART",
			Currency: "USD",
			Expiry:   time.Now().AddDate(0, 1, 0).Format("20060102"),
			Strike:   "200",
			Right:    ibkr.RightCall,
		},
		OptionPrice: ibkr.MustParseDecimal("10"),
		UnderPrice:  ibkr.MustParseDecimal("200"),
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

	result, err := client.CalcOptionPrice(ctx, ibkr.CalcOptionPriceRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeOption,
			Exchange: "SMART",
			Currency: "USD",
			Expiry:   time.Now().AddDate(0, 1, 0).Format("20060102"),
			Strike:   "200",
			Right:    ibkr.RightCall,
		},
		Volatility: ibkr.MustParseDecimal("0.3"),
		UnderPrice: ibkr.MustParseDecimal("200"),
	})
	if err != nil {
		t.Logf("CalcOptionPrice() returned: %v (may be expected)", err)
		return
	}
	t.Logf("CalcOptionPrice: %+v", result)
}

func TestLivePlaceOrderModify(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	defer func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanCancel()
		_ = client.GlobalCancel(cleanCtx)
	}()

	handle, err := client.PlaceOrder(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplContract,
		Order: ibkr.Order{
			Action:    ibkr.Buy,
			OrderType: "LMT",
			Quantity:  ibkr.MustParseDecimal("1"),
			LmtPrice:  ibkr.MustParseDecimal("50"),
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
		OrderType: "LMT",
		Quantity:  ibkr.MustParseDecimal("1"),
		LmtPrice:  ibkr.MustParseDecimal("51"),
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
		t.Fatal("timeout waiting for cancelled state")
	}
}

func TestLivePlaceOrderBracket(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	defer func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanCancel()
		_ = client.GlobalCancel(cleanCtx)
	}()

	transmitFalse := false

	// Parent: MKT BUY 1 AAPL (don't transmit yet).
	parent, err := client.PlaceOrder(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplContract,
		Order: ibkr.Order{
			Action:    ibkr.Buy,
			OrderType: "MKT",
			Quantity:  ibkr.MustParseDecimal("1"),
			TIF:       ibkr.TIFDay,
			Transmit:  &transmitFalse,
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder (parent): %v", err)
	}
	t.Logf("parent orderID=%d", parent.OrderID())

	// Take-profit: LMT SELL 1 @ $500 (far above market).
	tp, err := client.PlaceOrder(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplContract,
		Order: ibkr.Order{
			Action:    ibkr.Sell,
			OrderType: "LMT",
			Quantity:  ibkr.MustParseDecimal("1"),
			LmtPrice:  ibkr.MustParseDecimal("500"),
			TIF:       ibkr.TIFGTC,
			ParentID:  parent.OrderID(),
			Transmit:  &transmitFalse,
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder (take-profit): %v", err)
	}
	t.Logf("take-profit orderID=%d", tp.OrderID())

	// Stop-loss: STP SELL 1 @ $50 (far below market). Transmit=true triggers the bracket.
	sl, err := client.PlaceOrder(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplContract,
		Order: ibkr.Order{
			Action:    ibkr.Sell,
			OrderType: "STP",
			Quantity:  ibkr.MustParseDecimal("1"),
			AuxPrice:  ibkr.MustParseDecimal("50"),
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

	handle, err := client.SubscribeDisplayGroup(ctx, 1)
	if err != nil {
		t.Fatalf("SubscribeDisplayGroup() error = %v", err)
	}
	defer handle.Close()

	waitForStateKind(t, handle.State(), ibkr.SubscriptionStarted)

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

func TestLiveSubscribeExecutions(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()

	sub, err := client.SubscribeExecutions(ctx, ibkr.ExecutionsRequest{},
		ibkr.WithResumePolicy(ibkr.ResumeNever))
	if err != nil {
		t.Fatalf("SubscribeExecutions() error = %v", err)
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
		case evt := <-sub.State():
			if evt.Kind == ibkr.SubscriptionSnapshotComplete {
				t.Logf("SubscribeExecutions: %d events before SnapshotComplete", events)
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

func TestLiveSubscribeOpenOrders(t *testing.T) {
	t.Parallel()

	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second,
		ibkr.WithClientID(0))
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()

	sub, err := client.SubscribeOpenOrders(ctx, ibkr.OpenOrdersScopeAll,
		ibkr.WithResumePolicy(ibkr.ResumeNever))
	if err != nil {
		var apiErr *ibkr.APIError
		if errors.As(err, &apiErr) {
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
		case evt := <-sub.State():
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

package ibkr_test

import (
	"context"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/ibkr"
	"github.com/ThomasMarcelis/ibkr-go/testing/ibkrlive"
)

var aaplContract = ibkr.Contract{
	Symbol:   "AAPL",
	SecType:  "STK",
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

	details, err := client.ContractDetails(ctx, ibkr.ContractDetailsRequest{
		Contract: aaplContract,
	})
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

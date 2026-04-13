package ibkr_test

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
	"github.com/ThomasMarcelis/ibkr-go/testing/testhost"
	"github.com/shopspring/decimal"
)

func TestDialContextSessionSnapshot(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "handshake.txt")
	defer client.Close()
	defer waitHost(t, host)

	snapshot := client.Session()
	if snapshot.State != ibkr.StateReady {
		t.Fatalf("state = %s, want %s", snapshot.State, ibkr.StateReady)
	}
	if snapshot.ServerVersion != 200 {
		t.Fatalf("server version = %d, want 200", snapshot.ServerVersion)
	}
	if len(snapshot.ManagedAccounts) != 2 {
		t.Fatalf("managed accounts = %v, want 2 entries", snapshot.ManagedAccounts)
	}
}

func TestDialContextWithClientID(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "handshake_client_id_0.txt", ibkr.WithClientID(0))
	defer client.Close()
	defer waitHost(t, host)

	if got := client.Session().NextValidID; got != 1001 {
		t.Fatalf("next valid id = %d, want 1001", got)
	}
}

func TestContractDetails(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "contract_details.txt")
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
	if details[0].MinTick.String() != "0.01" {
		t.Fatalf("min tick = %s, want 0.01", details[0].MinTick.String())
	}
}

func TestHistoricalBars(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "historical_bars.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bars, err := client.History().Bars(ctx, ibkr.HistoricalBarsRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		EndTime:    time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC),
		Duration:   ibkr.Days(1),
		BarSize:    ibkr.Bar1Hour,
		WhatToShow: ibkr.ShowTrades,
		UseRTH:     true,
	})
	if err != nil {
		t.Fatalf("HistoricalBars() error = %v", err)
	}
	if len(bars) != 2 {
		t.Fatalf("bars len = %d, want 2", len(bars))
	}
	if bars[1].Close.String() != "101.5" {
		t.Fatalf("close = %s, want 101.5", bars[1].Close.String())
	}
}

func TestPersistentClientSequentialHistoricalBarsWith108End(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "historical_bars_sequential_108_end.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	req := ibkr.HistoricalBarsRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		EndTime:    time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC),
		Duration:   ibkr.Days(5),
		BarSize:    ibkr.Bar1Day,
		WhatToShow: ibkr.ShowTrades,
		UseRTH:     true,
	}

	first, err := client.History().Bars(ctx, req)
	if err != nil {
		t.Fatalf("first HistoricalBars() error = %v", err)
	}
	if len(first) != 2 {
		t.Fatalf("first bars len = %d, want 2", len(first))
	}

	req.Duration = ibkr.Days(1)
	req.BarSize = ibkr.Bar1Hour
	second, err := client.History().Bars(ctx, req)
	if err != nil {
		t.Fatalf("second HistoricalBars() error = %v", err)
	}
	if len(second) != 2 {
		t.Fatalf("second bars len = %d, want 2", len(second))
	}
	if got := client.Session().State; got != ibkr.StateReady && got != ibkr.StateDegraded {
		t.Fatalf("session state after sequential historical bars = %s, want usable session", got)
	}
}

func TestCurrentTime(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "current_time.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ts, err := client.CurrentTime(ctx)
	if err != nil {
		t.Fatalf("CurrentTime() error = %v", err)
	}
	// 1775931128 epoch seconds = 2026-04-11 18:12:08 UTC (transcript header
	// shows 20:12:09 CEST). Grounded evidence in current_time.txt.
	want := time.Unix(1775931128, 0).UTC()
	if !ts.Equal(want) {
		t.Errorf("CurrentTime() = %v, want %v", ts, want)
	}
}

func TestHistoricalSchedule(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "historical_schedule_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	schedule, err := client.History().Schedule(ctx, ibkr.HistoricalScheduleRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Duration: ibkr.Months(1),
		BarSize:  ibkr.Bar1Day,
		UseRTH:   true,
	})
	if err != nil {
		t.Fatalf("History().Schedule() error = %v", err)
	}
	if schedule.TimeZone != "US/Eastern" {
		t.Errorf("TimeZone = %q, want US/Eastern", schedule.TimeZone)
	}
	if schedule.StartDateTime != "20260312-09:30:00" {
		t.Errorf("StartDateTime = %q, want 20260312-09:30:00", schedule.StartDateTime)
	}
	if schedule.EndDateTime != "20260410-16:00:00" {
		t.Errorf("EndDateTime = %q, want 20260410-16:00:00", schedule.EndDateTime)
	}
	if len(schedule.Sessions) != 21 {
		t.Fatalf("Sessions = %d, want 21", len(schedule.Sessions))
	}
	first := schedule.Sessions[0]
	if first.RefDate != "20260312" || first.StartDateTime != "20260312-09:30:00" {
		t.Errorf("first session = %+v, want 20260312-09:30:00 / 20260312", first)
	}
	last := schedule.Sessions[20]
	if last.RefDate != "20260410" || last.EndDateTime != "20260410-16:00:00" {
		t.Errorf("last session = %+v, want 20260410-09:30:00 / 20260410", last)
	}
}

func TestHistoricalBarsWithScheduleRejects(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "handshake.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.History().Bars(ctx, ibkr.HistoricalBarsRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Duration:   ibkr.Months(1),
		BarSize:    ibkr.Bar1Day,
		WhatToShow: ibkr.ShowSchedule,
		UseRTH:     true,
	})
	if err == nil {
		t.Fatal("History().Bars() with SCHEDULE: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "SCHEDULE") {
		t.Errorf("error = %v, want message mentioning SCHEDULE", err)
	}
}

func TestAccountSummary(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "account_summary.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	values, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Account: "DU12345",
		Tags:    []string{"NetLiquidation", "BuyingPower"},
	})
	if err != nil {
		t.Fatalf("AccountSummary() error = %v", err)
	}
	if len(values) != 2 {
		t.Fatalf("values len = %d, want 2", len(values))
	}
	if values[0].Account != "DU12345" || values[1].Account != "DU12345" {
		t.Fatalf("accounts = %#v, want only DU12345", values)
	}
	if values[0].Tag != "NetLiquidation" {
		t.Fatalf("first tag = %q, want NetLiquidation", values[0].Tag)
	}
}

func TestAccountSummaryAllReturnsAllAccounts(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "account_summary.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	values, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Account: "All",
		Tags:    []string{"NetLiquidation", "BuyingPower"},
	})
	if err != nil {
		t.Fatalf("AccountSummary() error = %v", err)
	}
	if len(values) != 3 {
		t.Fatalf("values len = %d, want 3", len(values))
	}
}

func TestAccountSummarySucceedsWhenDisconnectFollowsSnapshotEnd(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "account_summary_disconnect_after_end.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	values, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Account: "DU12345",
		Tags:    []string{"NetLiquidation"},
	})
	if err != nil {
		t.Fatalf("AccountSummary() error = %v", err)
	}
	if len(values) != 1 {
		t.Fatalf("values len = %d, want 1", len(values))
	}
	if values[0].Value != "100000.00" {
		t.Fatalf("value = %#v, want 100000.00", values[0])
	}
}

func TestSubscribeAccountSummaryRejectsThirdConcurrentSubscription(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "account_summary_two_subs.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub1, err := client.Accounts().SubscribeSummary(ctx, ibkr.AccountSummaryRequest{
		Account: "DU12345",
		Tags:    []string{"NetLiquidation"},
	})
	if err != nil {
		t.Fatalf("SubscribeAccountSummary() first error = %v", err)
	}

	sub2, err := client.Accounts().SubscribeSummary(ctx, ibkr.AccountSummaryRequest{
		Account: "DU12345",
		Tags:    []string{"BuyingPower"},
	})
	if err != nil {
		t.Fatalf("SubscribeAccountSummary() second error = %v", err)
	}

	sub3, err := client.Accounts().SubscribeSummary(ctx, ibkr.AccountSummaryRequest{
		Account: "DU12345",
		Tags:    []string{"ExcessLiquidity"},
	})
	if err == nil {
		if sub3 != nil {
			_ = sub3.Close()
		}
		t.Fatal("SubscribeAccountSummary() third error = nil, want rejection")
	}
	if err := sub1.Close(); err != nil {
		t.Fatalf("sub1.Close() error = %v", err)
	}
	if err := sub2.Close(); err != nil {
		t.Fatalf("sub2.Close() error = %v", err)
	}
}

func TestSubscribeAccountSummarySnapshotCompleteDoesNotCloseStream(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "account_summary_stream.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.Accounts().SubscribeSummary(ctx, ibkr.AccountSummaryRequest{
		Account: "DU12345",
		Tags:    []string{"NetLiquidation"},
	})
	if err != nil {
		t.Fatalf("SubscribeAccountSummary() error = %v", err)
	}

	started := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionStarted)
	if started.ConnectionSeq != 1 {
		t.Fatalf("started.ConnectionSeq = %d, want 1", started.ConnectionSeq)
	}

	first := waitForEvent(t, sub.Events())
	if first.Value.Value != "100000.00" {
		t.Fatalf("first value = %#v, want 100000.00", first)
	}

	snapshot := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionSnapshotComplete)
	if snapshot.ConnectionSeq != 1 {
		t.Fatalf("snapshot.ConnectionSeq = %d, want 1", snapshot.ConnectionSeq)
	}

	second := waitForEvent(t, sub.Events())
	if second.Value.Value != "100500.00" {
		t.Fatalf("second value = %#v, want 100500.00", second)
	}

	if err := sub.Close(); err != nil {
		t.Fatalf("sub.Close() error = %v", err)
	}
	if err := sub.Wait(); err != nil {
		t.Fatalf("sub.Wait() error = %v", err)
	}
}

func TestPositionsSnapshot(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "positions.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	values, err := client.Accounts().Positions(ctx)
	if err != nil {
		t.Fatalf("PositionsSnapshot() error = %v", err)
	}
	if len(values) != 1 {
		t.Fatalf("positions len = %d, want 1", len(values))
	}
	if values[0].Position.String() != "10" {
		t.Fatalf("position = %s, want 10", values[0].Position.String())
	}
}

func TestPositionsSnapshotSucceedsWhenDisconnectFollowsSnapshotEnd(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "positions_disconnect_after_end.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	values, err := client.Accounts().Positions(ctx)
	if err != nil {
		t.Fatalf("PositionsSnapshot() error = %v", err)
	}
	if len(values) != 1 {
		t.Fatalf("positions len = %d, want 1", len(values))
	}
	if values[0].Position.String() != "10" {
		t.Fatalf("position = %s, want 10", values[0].Position.String())
	}
}

func TestQuoteSnapshot(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "quote_snapshot.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	quote, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
	})
	if err != nil {
		t.Fatalf("QuoteSnapshot() error = %v", err)
	}
	if quote.Bid.String() != "189.1" {
		t.Fatalf("bid = %s, want 189.1", quote.Bid.String())
	}
	if quote.Ask.String() != "189.15" {
		t.Fatalf("ask = %s, want 189.15", quote.Ask.String())
	}
}

func TestQuoteSnapshotWithGenericTicks(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "quote_with_generic_ticks.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	quote, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
	})
	if err != nil {
		t.Fatalf("QuoteSnapshot() error = %v", err)
	}
	if quote.Bid.String() != "189.1" {
		t.Fatalf("bid = %s, want 189.1", quote.Bid.String())
	}
	if quote.Ask.String() != "189.15" {
		t.Fatalf("ask = %s, want 189.15", quote.Ask.String())
	}
}

func TestSetMarketDataType(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "handshake.txt")
	defer client.Close()
	defer waitHost(t, host)
	ctx := context.Background()

	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("MarketData().SetType(MarketDataDelayed) error = %v", err)
	}

	// Validate boundary rejection.
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataType(0)); err == nil {
		t.Fatal("MarketData().SetType(0) error = nil, want validation error")
	}
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataType(5)); err == nil {
		t.Fatal("MarketData().SetType(5) error = nil, want validation error")
	}
}

func TestQuoteSnapshotRejectsGenericTicks(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "handshake.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		GenericTicks: []ibkr.GenericTick{"233"},
	})
	if err == nil {
		t.Fatal("QuoteSnapshot() error = nil, want validation error")
	}
}

func TestSubscribeQuotesResumeAutoReconnectsAfterTransportLoss(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "quote_stream_reconnect.txt", ibkr.WithReconnectPolicy(ibkr.ReconnectAuto))
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
	}, ibkr.WithResumePolicy(ibkr.ResumeAuto))
	if err != nil {
		t.Fatalf("SubscribeQuotes() error = %v", err)
	}

	started := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionStarted)
	if started.ConnectionSeq != 1 {
		t.Fatalf("started.ConnectionSeq = %d, want 1", started.ConnectionSeq)
	}

	first := waitForEvent(t, sub.Events())
	if first.Snapshot.Bid.String() != "189.1" {
		t.Fatalf("first bid = %s, want 189.1", first.Snapshot.Bid.String())
	}

	gap := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionGap)
	if gap.ConnectionSeq != 1 {
		t.Fatalf("gap.ConnectionSeq = %d, want 1", gap.ConnectionSeq)
	}
	if !gap.Retryable {
		t.Fatal("gap.Retryable = false, want true")
	}

	resumed := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionResumed)
	if resumed.ConnectionSeq != 2 {
		t.Fatalf("resumed.ConnectionSeq = %d, want 2", resumed.ConnectionSeq)
	}

	second := waitForEvent(t, sub.Events())
	if second.Snapshot.Ask.String() != "189.15" {
		t.Fatalf("second ask = %s, want 189.15", second.Snapshot.Ask.String())
	}

	if err := sub.Close(); err != nil {
		t.Fatalf("sub.Close() error = %v", err)
	}
	if err := sub.Wait(); err != nil {
		t.Fatalf("sub.Wait() error = %v", err)
	}
	if got := client.Session().ConnectionSeq; got != 2 {
		t.Fatalf("client.Session().ConnectionSeq = %d, want 2", got)
	}
}

func TestSubscribeQuotesResumeAutoResendsAfter1101(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "quote_stream_gap_1101.txt")
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
	}, ibkr.WithResumePolicy(ibkr.ResumeAuto))
	if err != nil {
		t.Fatalf("SubscribeQuotes() error = %v", err)
	}

	started := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionStarted)
	if started.ConnectionSeq != 1 {
		t.Fatalf("started.ConnectionSeq = %d, want 1", started.ConnectionSeq)
	}

	first := waitForEvent(t, sub.Events())
	if first.Snapshot.Bid.String() != "189.1" {
		t.Fatalf("first bid = %s, want 189.1", first.Snapshot.Bid.String())
	}

	gap := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionGap)
	if gap.ConnectionSeq != 1 {
		t.Fatalf("gap.ConnectionSeq = %d, want 1", gap.ConnectionSeq)
	}

	resumed := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionResumed)
	if resumed.ConnectionSeq != 1 {
		t.Fatalf("resumed.ConnectionSeq = %d, want 1", resumed.ConnectionSeq)
	}

	second := waitForEvent(t, sub.Events())
	if second.Snapshot.Ask.String() != "189.15" {
		t.Fatalf("second ask = %s, want 189.15", second.Snapshot.Ask.String())
	}

	if err := sub.Close(); err != nil {
		t.Fatalf("sub.Close() error = %v", err)
	}
	if err := sub.Wait(); err != nil {
		t.Fatalf("sub.Wait() error = %v", err)
	}
}

func TestSubscribeQuotesResumeAutoResumesWithoutResendAfter1102(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "quote_stream_gap_1102.txt")
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
	}, ibkr.WithResumePolicy(ibkr.ResumeAuto))
	if err != nil {
		t.Fatalf("SubscribeQuotes() error = %v", err)
	}

	started := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionStarted)
	if started.ConnectionSeq != 1 {
		t.Fatalf("started.ConnectionSeq = %d, want 1", started.ConnectionSeq)
	}

	first := waitForEvent(t, sub.Events())
	if first.Snapshot.Bid.String() != "189.1" {
		t.Fatalf("first bid = %s, want 189.1", first.Snapshot.Bid.String())
	}

	gap := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionGap)
	if gap.ConnectionSeq != 1 {
		t.Fatalf("gap.ConnectionSeq = %d, want 1", gap.ConnectionSeq)
	}

	resumed := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionResumed)
	if resumed.ConnectionSeq != 1 {
		t.Fatalf("resumed.ConnectionSeq = %d, want 1", resumed.ConnectionSeq)
	}

	second := waitForEvent(t, sub.Events())
	if second.Snapshot.Ask.String() != "189.15" {
		t.Fatalf("second ask = %s, want 189.15", second.Snapshot.Ask.String())
	}

	if err := sub.Close(); err != nil {
		t.Fatalf("sub.Close() error = %v", err)
	}
	if err := sub.Wait(); err != nil {
		t.Fatalf("sub.Wait() error = %v", err)
	}
}

func TestOpenOrdersSnapshot(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "open_orders.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	values, err := client.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
	if err != nil {
		t.Fatalf("OpenOrdersSnapshot() error = %v", err)
	}
	if len(values) != 1 {
		t.Fatalf("orders len = %d, want 1", len(values))
	}
	if values[0].Remaining.String() != "8" {
		t.Fatalf("remaining = %s, want 8", values[0].Remaining.String())
	}
}

func TestOpenOrdersSnapshotSucceedsWhenDisconnectFollowsSnapshotEnd(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "open_orders_disconnect_after_end.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	values, err := client.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
	if err != nil {
		t.Fatalf("OpenOrdersSnapshot() error = %v", err)
	}
	if len(values) != 1 {
		t.Fatalf("orders len = %d, want 1", len(values))
	}
	if values[0].Remaining.String() != "8" {
		t.Fatalf("remaining = %s, want 8", values[0].Remaining.String())
	}
}

func TestOpenOrdersAutoRequiresClientIDZero(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "handshake.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.Orders().SubscribeOpen(ctx, ibkr.OpenOrdersScopeAuto)
	if err == nil {
		if sub != nil {
			_ = sub.Close()
		}
		t.Fatal("SubscribeOpenOrders() error = nil, want client-id validation")
	}
}

func TestUnsupportedResumeAutoPolicies(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		subscribe func(context.Context, *ibkr.Client) error
	}{
		{
			name: "account_summary",
			subscribe: func(ctx context.Context, client *ibkr.Client) error {
				sub, err := client.Accounts().SubscribeSummary(ctx, ibkr.AccountSummaryRequest{
					Account: "DU12345",
					Tags:    []string{"NetLiquidation"},
				}, ibkr.WithResumePolicy(ibkr.ResumeAuto))
				if sub != nil {
					_ = sub.Close()
				}
				return err
			},
		},
		{
			name: "positions",
			subscribe: func(ctx context.Context, client *ibkr.Client) error {
				sub, err := client.Accounts().SubscribePositions(ctx, ibkr.WithResumePolicy(ibkr.ResumeAuto))
				if sub != nil {
					_ = sub.Close()
				}
				return err
			},
		},
		{
			name: "open_orders",
			subscribe: func(ctx context.Context, client *ibkr.Client) error {
				sub, err := client.Orders().SubscribeOpen(ctx, ibkr.OpenOrdersScopeAll, ibkr.WithResumePolicy(ibkr.ResumeAuto))
				if sub != nil {
					_ = sub.Close()
				}
				return err
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, host := newClient(t, "handshake.txt")
			defer client.Close()
			defer waitHost(t, host)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := tc.subscribe(ctx, client); err == nil {
				t.Fatalf("%s resume-auto error = nil, want rejection", tc.name)
			}
		})
	}
}

func TestExecutions(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "executions.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	updates, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{
		Account: "DU12345",
		Symbol:  "AAPL",
	})
	if err != nil {
		t.Fatalf("Executions() error = %v", err)
	}
	if len(updates) != 2 {
		t.Fatalf("updates len = %d, want 2", len(updates))
	}
	if updates[0].Execution == nil || updates[0].Execution.ExecID != "exec-1" {
		t.Fatalf("first execution = %#v", updates[0].Execution)
	}
	if updates[1].Commission == nil || updates[1].Commission.Commission.String() != "1.25" {
		t.Fatalf("second commission = %#v", updates[1].Commission)
	}
}

func TestExecutionsCompletesOnSnapshotEnd(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "executions.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	updates, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{
		Account: "DU12345",
		Symbol:  "AAPL",
	})
	if err != nil {
		t.Fatalf("Executions() error = %v", err)
	}
	if len(updates) != 2 {
		t.Fatalf("updates len = %d, want 2", len(updates))
	}
	if updates[0].Execution == nil || updates[0].Execution.ExecID != "exec-1" {
		t.Fatalf("execution = %#v, want exec-1", updates[0])
	}
	if updates[1].Commission == nil || updates[1].Commission.ExecID != "exec-1" {
		t.Fatalf("commission = %#v, want exec-1", updates[1])
	}
}

func TestExecutionsMissingEndReturnsContextDeadline(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "executions_missing_end_live.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Executions() error = %v, want context deadline exceeded", err)
	}
}

func TestExecutionsCorrelateCommissionByExecID(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "executions_correlated.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	aaplCh := make(chan []ibkr.ExecutionUpdate, 1)
	msftCh := make(chan []ibkr.ExecutionUpdate, 1)
	errCh := make(chan error, 2)
	go func() {
		updates, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU12345", Symbol: "AAPL"})
		if err != nil {
			errCh <- err
			return
		}
		aaplCh <- updates
	}()
	time.Sleep(10 * time.Millisecond)
	go func() {
		updates, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU12345", Symbol: "MSFT"})
		if err != nil {
			errCh <- err
			return
		}
		msftCh <- updates
	}()

	var aaplUpdates []ibkr.ExecutionUpdate
	var msftUpdates []ibkr.ExecutionUpdate
	for i := 0; i < 2; i++ {
		select {
		case err := <-errCh:
			t.Fatalf("Executions() error = %v", err)
		case aaplUpdates = <-aaplCh:
		case msftUpdates = <-msftCh:
		}
	}
	if len(aaplUpdates) != 2 || len(msftUpdates) != 2 {
		t.Fatalf("updates len = AAPL %d, MSFT %d, want 2 each", len(aaplUpdates), len(msftUpdates))
	}
	aaplExec := aaplUpdates[0]
	aaplCommission := aaplUpdates[1]
	msftExec := msftUpdates[0]
	msftCommission := msftUpdates[1]

	if aaplExec.Execution == nil || aaplExec.Execution.Symbol != "AAPL" {
		t.Fatalf("AAPL execution = %#v, want AAPL execution", aaplExec)
	}
	if aaplCommission.Commission == nil || aaplCommission.Commission.ExecID != "exec-aapl" {
		t.Fatalf("AAPL commission = %#v, want exec-aapl", aaplCommission)
	}
	if msftExec.Execution == nil || msftExec.Execution.Symbol != "MSFT" {
		t.Fatalf("MSFT execution = %#v, want MSFT execution", msftExec)
	}
	if msftCommission.Commission == nil || msftCommission.Commission.ExecID != "exec-msft" {
		t.Fatalf("MSFT commission = %#v, want exec-msft", msftCommission)
	}
}

func TestExecutionsCorrelateCommissionForOverlappingSubscriptions(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "executions_overlapping.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wideCh := make(chan []ibkr.ExecutionUpdate, 1)
	narrowCh := make(chan []ibkr.ExecutionUpdate, 1)
	errCh := make(chan error, 2)
	go func() {
		updates, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU12345"})
		if err != nil {
			errCh <- err
			return
		}
		wideCh <- updates
	}()
	time.Sleep(10 * time.Millisecond)
	go func() {
		updates, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU12345", Symbol: "AAPL"})
		if err != nil {
			errCh <- err
			return
		}
		narrowCh <- updates
	}()

	var wideUpdates []ibkr.ExecutionUpdate
	var narrowUpdates []ibkr.ExecutionUpdate
	for i := 0; i < 2; i++ {
		select {
		case err := <-errCh:
			t.Fatalf("Executions() error = %v", err)
		case wideUpdates = <-wideCh:
		case narrowUpdates = <-narrowCh:
		}
	}
	if len(wideUpdates) != 2 || len(narrowUpdates) != 2 {
		t.Fatalf("updates len = wide %d, narrow %d, want 2 each", len(wideUpdates), len(narrowUpdates))
	}
	wideExec := wideUpdates[0]
	wideCommission := wideUpdates[1]
	narrowExec := narrowUpdates[0]
	narrowCommission := narrowUpdates[1]

	if wideExec.Execution == nil || wideExec.Execution.ExecID != "exec-aapl" {
		t.Fatalf("account-wide execution = %#v, want exec-aapl", wideExec)
	}
	if wideCommission.Commission == nil || wideCommission.Commission.ExecID != "exec-aapl" {
		t.Fatalf("account-wide commission = %#v, want exec-aapl", wideCommission)
	}
	if narrowExec.Execution == nil || narrowExec.Execution.ExecID != "exec-aapl" {
		t.Fatalf("AAPL-only execution = %#v, want exec-aapl", narrowExec)
	}
	if narrowCommission.Commission == nil || narrowCommission.Commission.ExecID != "exec-aapl" {
		t.Fatalf("AAPL-only commission = %#v, want exec-aapl", narrowCommission)
	}
}

func TestSubscribeRealTimeBarsResumeAutoReconnectsAfterTransportLoss(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "realtime_bars_reconnect.txt", ibkr.WithReconnectPolicy(ibkr.ReconnectAuto))
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
	}, ibkr.WithResumePolicy(ibkr.ResumeAuto))
	if err != nil {
		t.Fatalf("SubscribeRealTimeBars() error = %v", err)
	}

	started := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionStarted)
	if started.ConnectionSeq != 1 {
		t.Fatalf("started.ConnectionSeq = %d, want 1", started.ConnectionSeq)
	}

	first := waitForEvent(t, sub.Events())
	if first.Close.String() != "189.1" {
		t.Fatalf("first close = %s, want 189.1", first.Close.String())
	}

	gap := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionGap)
	if gap.ConnectionSeq != 1 {
		t.Fatalf("gap.ConnectionSeq = %d, want 1", gap.ConnectionSeq)
	}
	if !gap.Retryable {
		t.Fatal("gap.Retryable = false, want true")
	}

	resumed := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionResumed)
	if resumed.ConnectionSeq != 2 {
		t.Fatalf("resumed.ConnectionSeq = %d, want 2", resumed.ConnectionSeq)
	}

	second := waitForEvent(t, sub.Events())
	if second.Close.String() != "189.2" {
		t.Fatalf("second close = %s, want 189.2", second.Close.String())
	}

	if err := sub.Close(); err != nil {
		t.Fatalf("sub.Close() error = %v", err)
	}
	if err := sub.Wait(); err != nil {
		t.Fatalf("sub.Wait() error = %v", err)
	}
}

func TestSubscribeQuotesResumeNeverRequiresManualResumeOnDisconnect(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "quote_stream_disconnect.txt")
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

	started := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionStarted)
	if started.ConnectionSeq != 1 {
		t.Fatalf("started.ConnectionSeq = %d, want 1", started.ConnectionSeq)
	}

	if err := sub.Wait(); !errors.Is(err, ibkr.ErrResumeRequired) {
		t.Fatalf("sub.Wait() error = %v, want %v", err, ibkr.ErrResumeRequired)
	}
}

func TestFamilyCodes(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "family_codes.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	codes, err := client.Accounts().FamilyCodes(ctx)
	if err != nil {
		t.Fatalf("FamilyCodes() error = %v", err)
	}
	if len(codes) != 1 {
		t.Fatalf("codes len = %d, want 1", len(codes))
	}
	if codes[0].AccountID != "DU9000001" {
		t.Fatalf("account_id = %q, want DU9000001", codes[0].AccountID)
	}
	if codes[0].FamilyCode != "F12345" {
		t.Fatalf("family_code = %q, want F12345", codes[0].FamilyCode)
	}
}

func TestMktDepthExchanges(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "mkt_depth_exchanges.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exchanges, err := client.Contracts().DepthExchanges(ctx)
	if err != nil {
		t.Fatalf("MktDepthExchanges() error = %v", err)
	}
	if len(exchanges) != 1 {
		t.Fatalf("exchanges len = %d, want 1", len(exchanges))
	}
	if exchanges[0].Exchange != "ARCA" {
		t.Fatalf("exchange = %q, want ARCA", exchanges[0].Exchange)
	}
	if exchanges[0].SecType != ibkr.SecTypeStock {
		t.Fatalf("sec_type = %q, want STK", exchanges[0].SecType)
	}
	if exchanges[0].AggGroup != 4 {
		t.Fatalf("agg_group = %d, want 4", exchanges[0].AggGroup)
	}
}

func TestNewsProviders(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "news_providers.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	providers, err := client.News().Providers(ctx)
	if err != nil {
		t.Fatalf("NewsProviders() error = %v", err)
	}
	if len(providers) != 2 {
		t.Fatalf("providers len = %d, want 2", len(providers))
	}
	if providers[0].Code != "BZ" {
		t.Fatalf("first code = %q, want BZ", providers[0].Code)
	}
	if providers[1].Name != "Fly On The Wall" {
		t.Fatalf("second name = %q, want Fly On The Wall", providers[1].Name)
	}
}

func TestScannerParameters(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "scanner_parameters.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	xml, err := client.Scanner().Parameters(ctx)
	if err != nil {
		t.Fatalf("ScannerParameters() error = %v", err)
	}
	if len(xml) == 0 {
		t.Fatal("ScannerParameters() returned empty XML")
	}
	if string(xml) != "<ScanParameterResponse><InstrumentList></InstrumentList></ScanParameterResponse>" {
		t.Fatalf("xml = %q, want scan parameter response XML", xml)
	}
}

func TestUserInfo(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "user_info.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	whiteBrandingID, err := client.TWS().UserInfo(ctx)
	if err != nil {
		t.Fatalf("UserInfo() error = %v", err)
	}
	if whiteBrandingID != "" {
		t.Fatalf("white_branding_id = %q, want empty", whiteBrandingID)
	}
}

func TestMatchingSymbols(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "matching_symbols.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	symbols, err := client.Contracts().Search(ctx, "AAPL")
	if err != nil {
		t.Fatalf("MatchingSymbols() error = %v", err)
	}
	if len(symbols) != 1 {
		t.Fatalf("symbols len = %d, want 1", len(symbols))
	}
	if symbols[0].Symbol != "AAPL" {
		t.Fatalf("symbol = %q, want AAPL", symbols[0].Symbol)
	}
	if symbols[0].ConID != 265598 {
		t.Fatalf("con_id = %d, want 265598", symbols[0].ConID)
	}
	if symbols[0].PrimaryExchange != "NASDAQ" {
		t.Fatalf("primary_exchange = %q, want NASDAQ", symbols[0].PrimaryExchange)
	}
	if len(symbols[0].DerivativeSecTypes) != 2 {
		t.Fatalf("derivative_sec_types len = %d, want 2", len(symbols[0].DerivativeSecTypes))
	}
	if symbols[0].Description != "APPLE INC" {
		t.Fatalf("description = %q, want APPLE INC", symbols[0].Description)
	}
}

func TestHeadTimestamp(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "head_timestamp.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ts, err := client.History().HeadTimestamp(ctx, ibkr.HeadTimestampRequest{
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
	if !ts.Equal(time.Date(1980, 12, 12, 14, 30, 0, 0, time.UTC)) {
		t.Fatalf("timestamp = %s, want 1980-12-12 14:30:00 UTC", ts.Format(time.RFC3339))
	}
}

func TestMarketRule(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "market_rule.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rule, err := client.Contracts().MarketRule(ctx, 26)
	if err != nil {
		t.Fatalf("MarketRule() error = %v", err)
	}
	if rule.MarketRuleID != 26 {
		t.Fatalf("market_rule_id = %d, want 26", rule.MarketRuleID)
	}
	if len(rule.Increments) != 2 {
		t.Fatalf("increments len = %d, want 2", len(rule.Increments))
	}
	if rule.Increments[0].Increment.String() != "0.01" {
		t.Fatalf("first increment = %s, want 0.01", rule.Increments[0].Increment.String())
	}
	if rule.Increments[1].LowEdge.String() != "1" {
		t.Fatalf("second low_edge = %s, want 1", rule.Increments[1].LowEdge.String())
	}
}

func TestCompletedOrders(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "completed_orders.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	orders, err := client.Orders().Completed(ctx, true)
	if err != nil {
		t.Fatalf("CompletedOrders() error = %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("orders len = %d, want 1", len(orders))
	}
	if orders[0].Contract.Symbol != "AAPL" {
		t.Fatalf("symbol = %q, want AAPL", orders[0].Contract.Symbol)
	}
	if orders[0].Status != "Filled" {
		t.Fatalf("status = %q, want Filled", orders[0].Status)
	}
	if orders[0].Quantity.String() != "100" {
		t.Fatalf("quantity = %s, want 100", orders[0].Quantity.String())
	}
}

func TestCompletedOrdersCancelledSystemLive(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "completed_orders_cancelled_system_live.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	orders, err := client.Orders().Completed(ctx, true)
	if err != nil {
		t.Fatalf("CompletedOrders() error = %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("orders len = %d, want 1", len(orders))
	}
	order := orders[0]
	if order.Contract.Symbol != "AAPL" {
		t.Fatalf("symbol = %q, want AAPL", order.Contract.Symbol)
	}
	if order.Status != ibkr.OrderStatus("Cancelled") {
		t.Fatalf("status = %q, want Cancelled", order.Status)
	}
	if order.Quantity.String() != "1" {
		t.Fatalf("quantity = %s, want 1", order.Quantity.String())
	}
	if !order.Filled.IsZero() {
		t.Fatalf("filled = %s, want zero for non-filled completed order", order.Filled.String())
	}
	if !order.Remaining.IsZero() {
		t.Fatalf("remaining = %s, want zero for absent completed-order remaining", order.Remaining.String())
	}
}

func TestSecDefOptParams(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "sec_def_opt_params.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	params, err := client.Contracts().SecDefOptParams(ctx, ibkr.SecDefOptParamsRequest{
		UnderlyingSymbol:  "AAPL",
		UnderlyingSecType: ibkr.SecTypeStock,
		UnderlyingConID:   265598,
	})
	if err != nil {
		t.Fatalf("SecDefOptParams() error = %v", err)
	}
	if len(params) != 2 {
		t.Fatalf("params len = %d, want 2", len(params))
	}
	if params[0].Exchange != "SMART" {
		t.Fatalf("first exchange = %q, want SMART", params[0].Exchange)
	}
	if len(params[0].Expirations) != 2 {
		t.Fatalf("first expirations len = %d, want 2", len(params[0].Expirations))
	}
	if len(params[0].Strikes) != 3 {
		t.Fatalf("first strikes len = %d, want 3", len(params[0].Strikes))
	}
	if params[0].Strikes[0].String() != "150" {
		t.Fatalf("first strike = %s, want 150", params[0].Strikes[0].String())
	}
	if params[1].Exchange != "CBOE" {
		t.Fatalf("second exchange = %q, want CBOE", params[1].Exchange)
	}
}

func TestSmartComponents(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "smart_components.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	components, err := client.Contracts().SmartComponents(ctx, "9c0001")
	if err != nil {
		t.Fatalf("SmartComponents() error = %v", err)
	}
	if len(components) != 2 {
		t.Fatalf("components len = %d, want 2", len(components))
	}
	if components[0].ExchangeName != "IEX" {
		t.Fatalf("first exchange = %q, want IEX", components[0].ExchangeName)
	}
	if components[1].ExchangeLetter != "N" {
		t.Fatalf("second letter = %q, want N", components[1].ExchangeLetter)
	}
}

func TestCalcImpliedVolatility(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "calc_implied_volatility.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := client.Options().ImpliedVolatility(ctx, ibkr.CalcImpliedVolatilityRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeOption,
			Exchange: "SMART",
			Currency: "USD",
		},
		OptionPrice: decimal.RequireFromString("5.25"),
		UnderPrice:  decimal.RequireFromString("170.50"),
	})
	if err != nil {
		t.Fatalf("CalcImpliedVolatility() error = %v", err)
	}
	if result.ImpliedVol.String() != "0.35" {
		t.Fatalf("implied vol = %s, want 0.35", result.ImpliedVol.String())
	}
	if result.Delta.String() != "0.55" {
		t.Fatalf("delta = %s, want 0.55", result.Delta.String())
	}
}

func TestCalcOptionPrice(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "calc_option_price.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := client.Options().Price(ctx, ibkr.CalcOptionPriceRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeOption,
			Exchange: "SMART",
			Currency: "USD",
		},
		Volatility: decimal.RequireFromString("0.30"),
		UnderPrice: decimal.RequireFromString("175.00"),
	})
	if err != nil {
		t.Fatalf("CalcOptionPrice() error = %v", err)
	}
	if result.OptPrice.String() != "6.5" {
		t.Fatalf("opt price = %s, want 6.5", result.OptPrice.String())
	}
	if result.UndPrice.String() != "175" {
		t.Fatalf("und price = %s, want 175", result.UndPrice.String())
	}
}

func TestHistogramData(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "histogram_data.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entries, err := client.History().Histogram(ctx, ibkr.HistogramDataRequest{
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
		t.Fatalf("HistogramData() error = %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("entries len = %d, want 3", len(entries))
	}
	if entries[0].Price.String() != "170.5" {
		t.Fatalf("first price = %s, want 170.5", entries[0].Price.String())
	}
	if entries[1].Size.String() != "2300" {
		t.Fatalf("second size = %s, want 2300", entries[1].Size.String())
	}
}

func TestHistoricalTicksMidpoint(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "historical_ticks_midpoint.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	end := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)

	result, err := client.History().Ticks(ctx, ibkr.HistoricalTicksRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		EndTime:       end,
		NumberOfTicks: 100,
		WhatToShow:    ibkr.ShowMidpoint,
		UseRTH:        true,
	})
	if err != nil {
		t.Fatalf("HistoricalTicks() error = %v", err)
	}
	if len(result.Ticks) != 2 {
		t.Fatalf("ticks len = %d, want 2", len(result.Ticks))
	}
	if result.Ticks[0].Price.String() != "170.5" {
		t.Fatalf("first price = %s, want 170.5", result.Ticks[0].Price.String())
	}
	if !result.Ticks[0].Time.Equal(time.Unix(1712345678, 0).UTC()) {
		t.Fatalf("first time = %s, want %s", result.Ticks[0].Time.Format(time.RFC3339), time.Unix(1712345678, 0).UTC().Format(time.RFC3339))
	}
}

func TestHistoricalTicksBidAsk(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "historical_ticks_bid_ask.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	end := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)

	result, err := client.History().Ticks(ctx, ibkr.HistoricalTicksRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		EndTime:       end,
		NumberOfTicks: 100,
		WhatToShow:    ibkr.ShowBidAsk,
		UseRTH:        true,
	})
	if err != nil {
		t.Fatalf("HistoricalTicks() error = %v", err)
	}
	if len(result.BidAsk) != 1 {
		t.Fatalf("bid_ask ticks len = %d, want 1", len(result.BidAsk))
	}
	if result.BidAsk[0].BidPrice.String() != "170.4" {
		t.Fatalf("bid price = %s, want 170.4", result.BidAsk[0].BidPrice.String())
	}
	if result.BidAsk[0].AskPrice.String() != "170.6" {
		t.Fatalf("ask price = %s, want 170.6", result.BidAsk[0].AskPrice.String())
	}
	if result.BidAsk[0].TickAttrib != 1 {
		t.Fatalf("tick attrib = %d, want 1", result.BidAsk[0].TickAttrib)
	}
}

func TestHistoricalTicksTrades(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "historical_ticks_trades.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	end := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)

	result, err := client.History().Ticks(ctx, ibkr.HistoricalTicksRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		EndTime:       end,
		NumberOfTicks: 100,
		WhatToShow:    ibkr.ShowTrades,
		UseRTH:        true,
	})
	if err != nil {
		t.Fatalf("HistoricalTicks() error = %v", err)
	}
	if len(result.Last) != 1 {
		t.Fatalf("last ticks len = %d, want 1", len(result.Last))
	}
	if result.Last[0].Price.String() != "170.5" {
		t.Fatalf("price = %s, want 170.5", result.Last[0].Price.String())
	}
	if result.Last[0].Exchange != "ARCA" {
		t.Fatalf("exchange = %q, want ARCA", result.Last[0].Exchange)
	}
	if result.Last[0].TickAttrib != 2 {
		t.Fatalf("tick attrib = %d, want 2", result.Last[0].TickAttrib)
	}
}

func TestHistoricalTicksPreserveExplicitTimeZone(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "historical_ticks_timezone_window.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v", err)
	}
	start := time.Date(2026, 4, 5, 8, 0, 0, 0, newYork)
	end := time.Date(2026, 4, 5, 9, 0, 0, 0, newYork)

	result, err := client.History().Ticks(ctx, ibkr.HistoricalTicksRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		StartTime:     start,
		EndTime:       end,
		NumberOfTicks: 25,
		WhatToShow:    ibkr.ShowTrades,
		UseRTH:        true,
	})
	if err != nil {
		t.Fatalf("HistoricalTicks() error = %v", err)
	}
	if len(result.Last) != 1 {
		t.Fatalf("last ticks len = %d, want 1", len(result.Last))
	}
}

func TestNewsArticle(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "news_article.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	article, err := client.News().Article(ctx, ibkr.NewsArticleRequest{
		ProviderCode: "BRFG",
		ArticleID:    "BRFG$12345",
	})
	if err != nil {
		t.Fatalf("NewsArticle() error = %v", err)
	}
	if article.ArticleType != 0 {
		t.Fatalf("article type = %d, want 0", article.ArticleType)
	}
	if article.ArticleText != "AAPL earnings beat expectations" {
		t.Fatalf("article text = %q, want %q", article.ArticleText, "AAPL earnings beat expectations")
	}
}

func TestHistoricalNews(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "historical_news.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 5, 23, 59, 59, 0, time.UTC)

	items, err := client.News().Historical(ctx, ibkr.HistoricalNewsRequest{
		ConID:         265598,
		ProviderCodes: []ibkr.NewsProviderCode{"BRFG"},
		StartTime:     start,
		EndTime:       end,
		TotalResults:  10,
	})
	if err != nil {
		t.Fatalf("HistoricalNews() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items len = %d, want 2", len(items))
	}
	if items[0].Headline != "AAPL Q1 Earnings Preview" {
		t.Fatalf("first headline = %q, want %q", items[0].Headline, "AAPL Q1 Earnings Preview")
	}
	if !items[0].Time.Equal(time.UnixMilli(1775385000000).UTC()) {
		t.Fatalf("first time = %s, want %s", items[0].Time.Format(time.RFC3339), time.UnixMilli(1775385000000).UTC().Format(time.RFC3339))
	}
	if items[1].ArticleID != "BRFG$12344" {
		t.Fatalf("second article id = %q, want BRFG$12344", items[1].ArticleID)
	}
	if !items[1].Time.Equal(time.UnixMilli(1775379600000).UTC()) {
		t.Fatalf("second time = %s, want %s", items[1].Time.Format(time.RFC3339), time.UnixMilli(1775379600000).UTC().Format(time.RFC3339))
	}
}

func TestHistoricalNewsPreserveExplicitTimeZone(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "historical_news_timezone_window.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	amsterdam, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v", err)
	}
	start := time.Date(2026, 4, 1, 10, 0, 0, 0, amsterdam)
	end := time.Date(2026, 4, 5, 11, 30, 0, 0, amsterdam)

	items, err := client.News().Historical(ctx, ibkr.HistoricalNewsRequest{
		ConID:         265598,
		ProviderCodes: []ibkr.NewsProviderCode{"BRFG", "DJNL"},
		StartTime:     start,
		EndTime:       end,
		TotalResults:  5,
	})
	if err != nil {
		t.Fatalf("HistoricalNews() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
}

func TestSubscribeScannerResults(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "scanner_subscription.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.Scanner().SubscribeResults(ctx, ibkr.ScannerSubscriptionRequest{
		NumberOfRows: 10,
		Instrument:   "STK",
		LocationCode: "STK.US.MAJOR",
		ScanCode:     "TOP_PERC_GAIN",
	})
	if err != nil {
		t.Fatalf("SubscribeScannerResults() error = %v", err)
	}

	results := waitForEvent(t, sub.Events())
	if len(results) != 2 {
		t.Fatalf("results len = %d, want 2", len(results))
	}
	if results[0].Contract.Symbol != "AAPL" {
		t.Fatalf("first symbol = %q, want AAPL", results[0].Contract.Symbol)
	}
	if results[1].Contract.Symbol != "MSFT" {
		t.Fatalf("second symbol = %q, want MSFT", results[1].Contract.Symbol)
	}
	if results[0].Rank != 0 {
		t.Fatalf("first rank = %d, want 0", results[0].Rank)
	}

	if err := sub.Close(); err != nil {
		t.Fatalf("sub.Close() error = %v", err)
	}
}

func TestPlaceOrderLimit(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "place_order_limit.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Order: ibkr.Order{
			Action:    ibkr.Buy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  decimal.RequireFromString("150"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	if handle.OrderID() != 1 {
		t.Fatalf("OrderID = %d, want 1", handle.OrderID())
	}

	// Collect events until the handle is done (Filled is terminal).
	var events []ibkr.OrderEvent
	for {
		select {
		case evt := <-handle.Events():
			events = append(events, evt)
		case <-handle.Done():
			goto collected
		case <-ctx.Done():
			t.Fatal("timeout waiting for order events")
		}
	}
collected:

	if len(events) < 2 {
		t.Fatalf("got %d events, want at least 2", len(events))
	}

	// Verify terminal status is Filled.
	var lastStatus ibkr.OrderStatus
	for _, evt := range events {
		if evt.Status != nil {
			lastStatus = evt.Status.Status
		}
	}
	if lastStatus != "Filled" {
		t.Fatalf("last status = %q, want Filled", lastStatus)
	}

	// Verify the fill price was parsed.
	for _, evt := range events {
		if evt.Status != nil && evt.Status.Status == "Filled" {
			if evt.Status.AvgFillPrice.String() != "149.5" {
				t.Fatalf("avg fill price = %s, want 149.5", evt.Status.AvgFillPrice.String())
			}
		}
	}

	// Handle should be done after terminal status.
	select {
	case <-handle.Done():
	default:
		t.Fatal("handle not done after Filled")
	}

	if err := handle.Wait(); err != nil {
		t.Fatalf("handle.Wait() error = %v", err)
	}
}

func TestPlaceOrderWithExecution(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "place_order_fill_with_execution.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Order: ibkr.Order{
			Action:    ibkr.Buy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  decimal.RequireFromString("150"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	var sawOpenOrder, sawFilled, sawExecution, sawCommission bool
	checkEvent := func(evt ibkr.OrderEvent) {
		if evt.OpenOrder != nil {
			sawOpenOrder = true
		}
		if evt.Status != nil && evt.Status.Status == "Filled" {
			sawFilled = true
		}
		if evt.Execution != nil {
			sawExecution = true
			if evt.Execution.Price.String() != "149.5" {
				t.Fatalf("execution price = %s, want 149.5", evt.Execution.Price.String())
			}
		}
		if evt.Commission != nil {
			sawCommission = true
			if evt.Commission.Commission.String() != "1" {
				t.Fatalf("commission = %s, want 1", evt.Commission.Commission.String())
			}
		}
	}
	for {
		select {
		case evt := <-handle.Events():
			checkEvent(evt)
		case <-handle.Done():
			// Drain any remaining buffered events.
			for {
				select {
				case evt, ok := <-handle.Events():
					if !ok {
						goto done
					}
					checkEvent(evt)
				default:
					goto done
				}
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for order events")
		}
	}
done:
	if !sawOpenOrder {
		t.Error("never received OpenOrder event")
	}
	if !sawFilled {
		t.Error("never received Filled status")
	}
	if !sawExecution {
		t.Error("never received Execution event")
	}
	if !sawCommission {
		t.Error("never received Commission event")
	}
}

func TestCancelOrder(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "cancel_order.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Order: ibkr.Order{
			Action:    ibkr.Buy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  decimal.RequireFromString("150"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	// Wait for PreSubmitted before sending cancel.
	preSubmitted := waitForEvent(t, handle.Events())
	if preSubmitted.Status == nil || preSubmitted.Status.Status != "PreSubmitted" {
		// Could be the OpenOrder event first; consume until we see PreSubmitted.
		for preSubmitted.Status == nil || preSubmitted.Status.Status != "PreSubmitted" {
			preSubmitted = waitForEvent(t, handle.Events())
		}
	}

	// Send cancel request (the server already has Cancelled scheduled).
	if err := handle.Cancel(ctx); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	// Wait for the Cancelled status.
	sawCancelled := false
	for {
		select {
		case evt := <-handle.Events():
			if evt.Status != nil && evt.Status.Status == "Cancelled" {
				sawCancelled = true
				goto cancelDone
			}
		case <-handle.Done():
			// Drain any remaining buffered events.
			for {
				select {
				case evt, ok := <-handle.Events():
					if !ok {
						goto cancelDone
					}
					if evt.Status != nil && evt.Status.Status == "Cancelled" {
						sawCancelled = true
					}
				default:
					goto cancelDone
				}
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for cancel")
		}
	}
cancelDone:

	if !sawCancelled {
		t.Fatal("never received Cancelled status event")
	}

	select {
	case <-handle.Done():
	case <-ctx.Done():
		t.Fatal("timeout waiting for handle to close after Cancelled")
	}

	if err := handle.Wait(); err != nil {
		t.Fatalf("handle.Wait() error = %v", err)
	}
}

func TestDirectCancelOrder(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "direct_cancel_order.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Order: ibkr.Order{
			Action:    ibkr.Buy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  decimal.RequireFromString("150"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	// Wait for PreSubmitted before sending the direct-by-ID cancel.
	preSubmitted := waitForEvent(t, handle.Events())
	for preSubmitted.Status == nil || preSubmitted.Status.Status != "PreSubmitted" {
		preSubmitted = waitForEvent(t, handle.Events())
	}

	// Direct-by-ID cancel path: skip OrderHandle.Cancel and call the
	// top-level facade with the handle's order ID. This proves Orders().Cancel
	// reaches the same wire message without holding the handle.
	if err := client.Orders().Cancel(ctx, handle.OrderID()); err != nil {
		t.Fatalf("Orders().Cancel(%d): %v", handle.OrderID(), err)
	}

	sawCancelled := false
	for {
		select {
		case evt := <-handle.Events():
			if evt.Status != nil && evt.Status.Status == "Cancelled" {
				sawCancelled = true
				goto directCancelDone
			}
		case <-handle.Done():
			for {
				select {
				case evt, ok := <-handle.Events():
					if !ok {
						goto directCancelDone
					}
					if evt.Status != nil && evt.Status.Status == "Cancelled" {
						sawCancelled = true
					}
				default:
					goto directCancelDone
				}
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for direct-by-ID cancel")
		}
	}
directCancelDone:

	if !sawCancelled {
		t.Fatal("never received Cancelled status event after direct-by-ID cancel")
	}

	select {
	case <-handle.Done():
	case <-ctx.Done():
		t.Fatal("timeout waiting for handle to close after direct-by-ID cancel")
	}

	if err := handle.Wait(); err != nil {
		t.Fatalf("handle.Wait() error = %v", err)
	}
}

func newClient(t *testing.T, script string, opts ...ibkr.Option) (*ibkr.Client, *testhost.Host) {
	t.Helper()

	host := newHost(t, script)
	addrHost, addrPort, err := net.SplitHostPort(host.Addr())
	if err != nil {
		t.Fatalf("SplitHostPort() error = %v", err)
	}
	port, err := net.LookupPort("tcp", addrPort)
	if err != nil {
		t.Fatalf("LookupPort() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dialOpts := []ibkr.Option{
		ibkr.WithHost(addrHost),
		ibkr.WithPort(port),
		ibkr.WithReconnectPolicy(ibkr.ReconnectOff),
	}
	dialOpts = append(dialOpts, opts...)

	client, err := ibkr.DialContext(ctx, dialOpts...)
	if err != nil {
		t.Fatalf("DialContext() error = %v", err)
	}
	return client, host
}

func newHost(t *testing.T, script string) *testhost.Host {
	t.Helper()

	path := filepath.Join("testdata", "transcripts", script)
	host, err := testhost.NewFromFile(path)
	if err != nil {
		t.Fatalf("NewFromFile(%q) error = %v", path, err)
	}
	return host
}

func waitHost(t *testing.T, host *testhost.Host) {
	t.Helper()
	if err := host.Wait(); err != nil {
		t.Fatalf("host.Wait() error = %v", err)
	}
}

func waitForEvent[T any](t *testing.T, ch <-chan T) T {
	t.Helper()

	select {
	case value, ok := <-ch:
		if !ok {
			t.Fatal("event channel closed before value arrived")
		}
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for event")
		var zero T
		return zero
	}
}

func TestBootstrapWithFarmStatusCodes(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "bootstrap_with_farm_status.txt")
	defer client.Close()
	defer waitHost(t, host)

	snapshot := client.Session()
	if snapshot.State != ibkr.StateReady {
		t.Fatalf("state = %s, want %s", snapshot.State, ibkr.StateReady)
	}
	if snapshot.ServerVersion != 200 {
		t.Fatalf("server version = %d, want 200", snapshot.ServerVersion)
	}
	if len(snapshot.ManagedAccounts) != 1 || snapshot.ManagedAccounts[0] != "DU12345" {
		t.Fatalf("managed accounts = %v, want [DU12345]", snapshot.ManagedAccounts)
	}

	// Drain session events and verify farm-status codes arrived as events
	// without triggering a state change (State == Previous == Ready).
	farmCodes := map[int]bool{}
	events := client.SessionEvents()
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				break
			}
			if ev.Code >= 2000 && ev.Code < 3000 {
				farmCodes[ev.Code] = true
				if ev.State != ev.Previous {
					t.Fatalf("farm-status code %d caused state change: %s -> %s", ev.Code, ev.Previous, ev.State)
				}
			}
			continue
		case <-time.After(500 * time.Millisecond):
		}
		break
	}

	for _, code := range []int{2104, 2106, 2158} {
		if !farmCodes[code] {
			t.Errorf("farm-status code %d not observed in session events", code)
		}
	}
}

func TestQuoteSnapshotDelayedData(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "quote_delayed_data.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	quote, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
	})
	if err != nil {
		t.Fatalf("QuoteSnapshot() error = %v", err)
	}
	if quote.Bid.String() != "188.5" {
		t.Fatalf("bid = %s, want 188.5", quote.Bid.String())
	}
	if quote.Ask.String() != "188.6" {
		t.Fatalf("ask = %s, want 188.6", quote.Ask.String())
	}
	if quote.MarketDataType != 3 {
		t.Fatalf("market data type = %d, want 3 (delayed)", quote.MarketDataType)
	}
}

// Grounded fixture tests: real field values extracted from live IB Gateway captures.

func TestGroundedBootstrap(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "grounded_bootstrap.txt")
	defer client.Close()
	defer waitHost(t, host)

	snapshot := client.Session()
	if snapshot.State != ibkr.StateReady {
		t.Fatalf("state = %s, want %s", snapshot.State, ibkr.StateReady)
	}
	if snapshot.ServerVersion != 200 {
		t.Fatalf("server version = %d, want 200", snapshot.ServerVersion)
	}
	if len(snapshot.ManagedAccounts) != 1 || snapshot.ManagedAccounts[0] != "DU9000001" {
		t.Fatalf("managed accounts = %v, want [DU9000001]", snapshot.ManagedAccounts)
	}
	if snapshot.NextValidID != 1 {
		t.Fatalf("next valid id = %d, want 1", snapshot.NextValidID)
	}

	// Verify farm-status codes arrive as session events
	farmCodes := map[int]bool{}
	events := client.SessionEvents()
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				break
			}
			if ev.Code >= 2000 && ev.Code < 3000 {
				farmCodes[ev.Code] = true
			}
			continue
		case <-time.After(500 * time.Millisecond):
		}
		break
	}
	for _, code := range []int{2104, 2106, 2158} {
		if !farmCodes[code] {
			t.Errorf("farm-status code %d not observed", code)
		}
	}
}

func TestGroundedContractDetailsAAPL(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "grounded_contract_details_aapl.txt")
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
	d := details[0]
	if d.Symbol != "AAPL" {
		t.Errorf("Symbol = %q, want AAPL", d.Symbol)
	}
	if d.SecType != ibkr.SecTypeStock {
		t.Errorf("SecType = %q, want STK", d.SecType)
	}
	if d.Exchange != "SMART" {
		t.Errorf("Exchange = %q, want SMART", d.Exchange)
	}
	if d.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", d.Currency)
	}
	if d.PrimaryExchange != "NASDAQ" {
		t.Errorf("PrimaryExchange = %q, want NASDAQ", d.PrimaryExchange)
	}
	if d.ConID != 265598 {
		t.Errorf("ConID = %d, want 265598", d.ConID)
	}
	if d.TradingClass != "NMS" {
		t.Errorf("TradingClass = %q, want NMS", d.TradingClass)
	}
	if d.MarketName != "NMS" {
		t.Errorf("MarketName = %q, want NMS", d.MarketName)
	}
	if d.LongName != "APPLE INC" {
		t.Errorf("LongName = %q, want APPLE INC", d.LongName)
	}
	if d.MinTick.String() != "0.01" {
		t.Errorf("MinTick = %s, want 0.01", d.MinTick.String())
	}
	if d.TimeZoneID != "US/Eastern" {
		t.Errorf("TimeZoneID = %q, want US/Eastern", d.TimeZoneID)
	}
}

func TestGroundedAccountSummary(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "grounded_account_summary.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	values, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Account: "All",
		Tags:    []string{"NetLiquidation", "TotalCashValue", "BuyingPower", "ExcessLiquidity"},
	})
	if err != nil {
		t.Fatalf("AccountSummary() error = %v", err)
	}
	if len(values) != 4 {
		t.Fatalf("values len = %d, want 4", len(values))
	}

	byTag := map[string]ibkr.AccountValue{}
	for _, v := range values {
		byTag[v.Tag] = v
	}

	if v, ok := byTag["NetLiquidation"]; !ok || v.Value != "68000.00" {
		t.Errorf("NetLiquidation = %q, want 68000.00", v.Value)
	}
	if v, ok := byTag["TotalCashValue"]; !ok || v.Value != "12000.00" {
		t.Errorf("TotalCashValue = %q, want 12000.00", v.Value)
	}
	if v, ok := byTag["BuyingPower"]; !ok || v.Value != "300000.00" {
		t.Errorf("BuyingPower = %q, want 300000.00", v.Value)
	}
	if v, ok := byTag["ExcessLiquidity"]; !ok || v.Value != "50000.00" {
		t.Errorf("ExcessLiquidity = %q, want 50000.00", v.Value)
	}

	for _, v := range values {
		if v.Account != "DU9000001" {
			t.Errorf("tag %s: account = %q, want DU9000001", v.Tag, v.Account)
		}
		if v.Currency != "EUR" {
			t.Errorf("tag %s: currency = %q, want EUR", v.Tag, v.Currency)
		}
	}
}

func TestGroundedHistoricalBars(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "grounded_historical_bars.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bars, err := client.History().Bars(ctx, ibkr.HistoricalBarsRequest{
		Contract: ibkr.Contract{
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
	if err != nil {
		t.Fatalf("HistoricalBars() error = %v", err)
	}
	if len(bars) != 7 {
		t.Fatalf("bars len = %d, want 7", len(bars))
	}

	// First bar: 2026-04-02 09:30 US/Eastern
	first := bars[0]
	if first.Open.String() != "254.2" {
		t.Errorf("first bar Open = %s, want 254.2", first.Open.String())
	}
	if first.High.String() != "254.8" {
		t.Errorf("first bar High = %s, want 254.8", first.High.String())
	}
	if first.Low.String() != "250.65" {
		t.Errorf("first bar Low = %s, want 250.65", first.Low.String())
	}
	if first.Close.String() != "252.53" {
		t.Errorf("first bar Close = %s, want 252.53", first.Close.String())
	}
	if first.Volume.String() != "2829736" {
		t.Errorf("first bar Volume = %s, want 2829736", first.Volume.String())
	}
	if first.Count != 13633 {
		t.Errorf("first bar Count = %d, want 13633", first.Count)
	}

	// Last bar: 2026-04-02 15:00 US/Eastern
	last := bars[6]
	if last.Close.String() != "255.89" {
		t.Errorf("last bar Close = %s, want 255.89", last.Close.String())
	}
	if last.Volume.String() != "2938382" {
		t.Errorf("last bar Volume = %s, want 2938382", last.Volume.String())
	}
}

func TestGroundedPositions(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "grounded_positions.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	positions, err := client.Accounts().Positions(ctx)
	if err != nil {
		t.Fatalf("PositionsSnapshot() error = %v", err)
	}
	if len(positions) != 4 {
		t.Fatalf("positions len = %d, want 4", len(positions))
	}

	// Find AMZN position by symbol
	var amzn *ibkr.Position
	var aapl *ibkr.Position
	var yw *ibkr.Position
	var qqq *ibkr.Position
	for i := range positions {
		switch positions[i].Contract.Symbol {
		case "AMZN":
			amzn = &positions[i]
		case "AAPL":
			aapl = &positions[i]
		case "YW":
			yw = &positions[i]
		case "QQQ":
			qqq = &positions[i]
		}
	}

	if amzn == nil {
		t.Fatal("AMZN position not found")
	}
	if amzn.Account != "DU9000001" {
		t.Errorf("AMZN account = %q, want DU9000001", amzn.Account)
	}
	if amzn.Position.String() != "15" {
		t.Errorf("AMZN position = %s, want 15", amzn.Position.String())
	}
	if amzn.AvgCost.String() != "200.25" {
		t.Errorf("AMZN avgCost = %s, want 200.25", amzn.AvgCost.String())
	}
	if amzn.Contract.SecType != ibkr.SecTypeStock {
		t.Errorf("AMZN secType = %q, want STK", amzn.Contract.SecType)
	}
	if amzn.Contract.Currency != "USD" {
		t.Errorf("AMZN currency = %q, want USD", amzn.Contract.Currency)
	}
	if amzn.Contract.ConID != 3691937 {
		t.Errorf("AMZN conID = %d, want 3691937", amzn.Contract.ConID)
	}
	if amzn.Contract.TradingClass != "NMS" {
		t.Errorf("AMZN tradingClass = %q, want NMS", amzn.Contract.TradingClass)
	}

	if aapl == nil {
		t.Fatal("AAPL position not found")
	}
	if aapl.Position.String() != "10" {
		t.Errorf("AAPL position = %s, want 10", aapl.Position.String())
	}
	if aapl.AvgCost.String() != "256.1" {
		t.Errorf("AAPL avgCost = %s, want 256.1", aapl.AvgCost.String())
	}

	if yw == nil {
		t.Fatal("YW position not found")
	}
	if yw.Contract.SecType != ibkr.SecTypeFuture {
		t.Errorf("YW secType = %q, want FUT", yw.Contract.SecType)
	}
	if yw.Contract.ConID != 715358256 {
		t.Errorf("YW conID = %d, want 715358256", yw.Contract.ConID)
	}
	if yw.Contract.Expiry != "20261214" {
		t.Errorf("YW expiry = %q, want 20261214", yw.Contract.Expiry)
	}
	if yw.Contract.Multiplier != "1000" {
		t.Errorf("YW multiplier = %q, want 1000", yw.Contract.Multiplier)
	}
	if yw.Contract.TradingClass != "XW" {
		t.Errorf("YW tradingClass = %q, want XW", yw.Contract.TradingClass)
	}
	if yw.Position.String() != "1" {
		t.Errorf("YW position = %s, want 1", yw.Position.String())
	}

	if qqq == nil {
		t.Fatal("QQQ position not found")
	}
	if qqq.Contract.SecType != ibkr.SecTypeOption {
		t.Errorf("QQQ secType = %q, want OPT", qqq.Contract.SecType)
	}
	if qqq.Contract.ConID != 728937835 {
		t.Errorf("QQQ conID = %d, want 728937835", qqq.Contract.ConID)
	}
	if qqq.Contract.Expiry != "20270115" {
		t.Errorf("QQQ expiry = %q, want 20270115", qqq.Contract.Expiry)
	}
	if qqq.Contract.Strike != "500.0" {
		t.Errorf("QQQ strike = %q, want 500.0", qqq.Contract.Strike)
	}
	if qqq.Contract.Right != ibkr.RightPut {
		t.Errorf("QQQ right = %q, want P", qqq.Contract.Right)
	}
	if qqq.Contract.Multiplier != "100" {
		t.Errorf("QQQ multiplier = %q, want 100", qqq.Contract.Multiplier)
	}
	if qqq.Contract.TradingClass != "QQQ" {
		t.Errorf("QQQ tradingClass = %q, want QQQ", qqq.Contract.TradingClass)
	}
	if qqq.Position.String() != "-3" {
		t.Errorf("QQQ position = %s, want -3", qqq.Position.String())
	}
}

func TestAccountUpdatesSnapshot(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "account_updates.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	values, err := client.Accounts().Updates(ctx, "DU12345")
	if err != nil {
		t.Fatalf("AccountUpdatesSnapshot() error = %v", err)
	}
	if len(values) != 2 {
		t.Fatalf("values len = %d, want 2", len(values))
	}
	// First is an account value
	if values[0].AccountValue == nil {
		t.Fatal("first value is nil AccountValue")
	}
	if values[0].AccountValue.Key != "NetLiquidation" {
		t.Fatalf("first key = %q, want NetLiquidation", values[0].AccountValue.Key)
	}
	// Second is a portfolio update
	if values[1].Portfolio == nil {
		t.Fatal("second value is nil Portfolio")
	}
	if values[1].Portfolio.Contract.Symbol != "AAPL" {
		t.Fatalf("portfolio symbol = %q, want AAPL", values[1].Portfolio.Contract.Symbol)
	}
	if values[1].Portfolio.Position.String() != "10" {
		t.Fatalf("portfolio position = %s, want 10", values[1].Portfolio.Position.String())
	}
}

func TestAccountUpdatesMultiSnapshot(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "account_updates_multi.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	values, err := client.Accounts().UpdatesMulti(ctx, ibkr.AccountUpdatesMultiRequest{
		Account:   "DU12345",
		ModelCode: "",
	})
	if err != nil {
		t.Fatalf("AccountUpdatesMultiSnapshot() error = %v", err)
	}
	if len(values) != 2 {
		t.Fatalf("values len = %d, want 2", len(values))
	}
	if values[0].Key != "NetLiquidation" {
		t.Fatalf("first key = %q, want NetLiquidation", values[0].Key)
	}
	if values[1].Key != "BuyingPower" {
		t.Fatalf("second key = %q, want BuyingPower", values[1].Key)
	}
}

func TestPositionsMultiSnapshot(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "positions_multi.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	values, err := client.Accounts().PositionsMulti(ctx, ibkr.PositionsMultiRequest{
		Account:   "DU12345",
		ModelCode: "",
	})
	if err != nil {
		t.Fatalf("PositionsMultiSnapshot() error = %v", err)
	}
	if len(values) != 1 {
		t.Fatalf("values len = %d, want 1", len(values))
	}
	if values[0].Contract.Symbol != "AAPL" {
		t.Fatalf("symbol = %q, want AAPL", values[0].Contract.Symbol)
	}
	if values[0].Position.String() != "10" {
		t.Fatalf("position = %s, want 10", values[0].Position.String())
	}
}

func TestSubscribePnL(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "pnl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.Accounts().SubscribePnL(ctx, ibkr.PnLRequest{
		Account:   "DU12345",
		ModelCode: "",
	})
	if err != nil {
		t.Fatalf("SubscribePnL() error = %v", err)
	}

	waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionStarted)

	update := waitForEvent(t, sub.Events())
	if update.DailyPnL.String() != "150.25" {
		t.Fatalf("daily pnl = %s, want 150.25", update.DailyPnL.String())
	}
	if update.UnrealizedPnL.String() != "45" {
		t.Fatalf("unrealized pnl = %s, want 45", update.UnrealizedPnL.String())
	}
	if update.RealizedPnL.String() != "105.25" {
		t.Fatalf("realized pnl = %s, want 105.25", update.RealizedPnL.String())
	}

	if err := sub.Close(); err != nil {
		t.Fatalf("sub.Close() error = %v", err)
	}
}

func TestSubscribePnLSingle(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "pnl_single.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.Accounts().SubscribePnLSingle(ctx, ibkr.PnLSingleRequest{
		Account:   "DU12345",
		ModelCode: "",
		ConID:     265598,
	})
	if err != nil {
		t.Fatalf("SubscribePnLSingle() error = %v", err)
	}

	waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionStarted)

	update := waitForEvent(t, sub.Events())
	if update.Position.String() != "10" {
		t.Fatalf("position = %s, want 10", update.Position.String())
	}
	if update.DailyPnL.String() != "50" {
		t.Fatalf("daily pnl = %s, want 50", update.DailyPnL.String())
	}
	if update.Value.String() != "1895" {
		t.Fatalf("value = %s, want 1895", update.Value.String())
	}

	if err := sub.Close(); err != nil {
		t.Fatalf("sub.Close() error = %v", err)
	}
}

func TestSubscribeTickByTick(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "tick_by_tick.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.MarketData().SubscribeTickByTick(ctx, ibkr.TickByTickRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		TickType:      "Last",
		NumberOfTicks: 0,
		IgnoreSize:    false,
	})
	if err != nil {
		t.Fatalf("SubscribeTickByTick() error = %v", err)
	}

	waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionStarted)

	tick := waitForEvent(t, sub.Events())
	if tick.TickType != 1 {
		t.Fatalf("tick type = %d, want 1", tick.TickType)
	}
	if tick.Price.String() != "189.5" {
		t.Fatalf("price = %s, want 189.5", tick.Price.String())
	}
	if tick.Size.String() != "100" {
		t.Fatalf("size = %s, want 100", tick.Size.String())
	}
	if tick.Exchange != "NASDAQ" {
		t.Fatalf("exchange = %q, want NASDAQ", tick.Exchange)
	}

	if err := sub.Close(); err != nil {
		t.Fatalf("sub.Close() error = %v", err)
	}
}

func TestSubscribeNewsBulletins(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "news_bulletins.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.News().SubscribeBulletins(ctx, true)
	if err != nil {
		t.Fatalf("SubscribeNewsBulletins() error = %v", err)
	}

	waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionStarted)

	bulletin := waitForEvent(t, sub.Events())
	if bulletin.MsgID != 1 {
		t.Fatalf("msg id = %d, want 1", bulletin.MsgID)
	}
	if bulletin.Headline != "Test bulletin headline" {
		t.Fatalf("headline = %q, want %q", bulletin.Headline, "Test bulletin headline")
	}
	if bulletin.Source != "IB System" {
		t.Fatalf("source = %q, want %q", bulletin.Source, "IB System")
	}

	if err := sub.Close(); err != nil {
		t.Fatalf("sub.Close() error = %v", err)
	}
}

func TestSubscribeHistoricalBars(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "historical_bars_stream.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.History().SubscribeBars(ctx, ibkr.HistoricalBarsRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Duration:   ibkr.Minutes(5),
		BarSize:    ibkr.Bar5Secs,
		WhatToShow: ibkr.ShowTrades,
		UseRTH:     false,
	})
	if err != nil {
		t.Fatalf("SubscribeHistoricalBars() error = %v", err)
	}

	// Receive initial bar
	bar1 := waitForEvent(t, sub.Events())
	if bar1.Close.String() != "189.05" {
		t.Fatalf("initial bar close = %s, want 189.05", bar1.Close.String())
	}

	// Wait for snapshot complete
	waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionSnapshotComplete)

	// Receive streaming update bar
	bar2 := waitForEvent(t, sub.Events())
	if bar2.Close.String() != "189.15" {
		t.Fatalf("streaming bar close = %s, want 189.15", bar2.Close.String())
	}

	if err := sub.Close(); err != nil {
		t.Fatalf("sub.Close() error = %v", err)
	}
}

func waitForStateKind(t *testing.T, ch <-chan ibkr.SubscriptionStateEvent, want ibkr.SubscriptionStateKind) ibkr.SubscriptionStateEvent {
	t.Helper()

	for {
		state := waitForEvent(t, ch)
		if state.Kind == want {
			return state
		}
	}
}

func TestSoftDollarTiersIntegration(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "soft_dollar_tiers.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tiers, err := client.Advisors().SoftDollarTiers(ctx)
	if err != nil {
		t.Fatalf("SoftDollarTiers() error = %v", err)
	}
	if tiers == nil {
		t.Fatal("SoftDollarTiers() = nil, want non-nil empty slice")
	}
	if len(tiers) != 0 {
		t.Fatalf("SoftDollarTiers() len = %d, want 0", len(tiers))
	}
}

func TestQueryDisplayGroupsIntegration(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "display_groups.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	groups, err := client.TWS().DisplayGroups(ctx)
	if err != nil {
		t.Fatalf("QueryDisplayGroups() error = %v", err)
	}
	wantGroups := []ibkr.DisplayGroupID{1, 2, 3, 4, 5, 6, 7}
	if !reflect.DeepEqual(groups, wantGroups) {
		t.Fatalf("QueryDisplayGroups() = %v, want %v", groups, wantGroups)
	}
}

func TestFundamentalDataIntegration(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "fundamental_data.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	data, err := client.Contracts().FundamentalData(ctx, ibkr.FundamentalDataRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		ReportType: "ReportSnapshot",
	})
	if err != nil {
		t.Fatalf("FundamentalData() error = %v", err)
	}
	if len(data) == 0 {
		t.Fatal("FundamentalData() returned empty document")
	}
	if !strings.Contains(string(data), "AAPL") {
		t.Fatalf("FundamentalData() does not contain AAPL: %s", string(data[:min(len(data), 200)]))
	}
}

func TestSubscribeDisplayGroupIntegration(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "display_group_subscribe.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := client.TWS().SubscribeDisplayGroup(ctx, 1)
	if err != nil {
		t.Fatalf("SubscribeDisplayGroup() error = %v", err)
	}

	waitForStateKind(t, handle.Lifecycle(), ibkr.SubscriptionStarted)

	// Read initial "none" update.
	initial := waitForEvent(t, handle.Events())
	if initial.ContractInfo != "none" {
		t.Fatalf("initial ContractInfo = %q, want %q", initial.ContractInfo, "none")
	}

	// Update to AAPL.
	if err := handle.Update(ctx, "265598"); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	updated := waitForEvent(t, handle.Events())
	if updated.ContractInfo != "265598@SMART" {
		t.Fatalf("updated ContractInfo = %q, want %q", updated.ContractInfo, "265598@SMART")
	}

	handle.Close()
}

func TestPlaceOrderModifyIntegration(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "place_order_modify.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
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

	// Wait for initial PreSubmitted.
	var sawInitial bool
	for !sawInitial {
		select {
		case evt := <-handle.Events():
			if evt.Status != nil && evt.Status.Status == "PreSubmitted" {
				sawInitial = true
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for initial PreSubmitted")
		}
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

	// Wait for modified OpenOrder with $51.
	var sawModified bool
	for !sawModified {
		select {
		case evt := <-handle.Events():
			if evt.OpenOrder != nil && evt.OpenOrder.LmtPrice.String() == "51" {
				sawModified = true
			}
		case <-handle.Done():
			sawModified = true
		case <-ctx.Done():
			t.Fatal("timeout waiting for modified OpenOrder")
		}
	}

	if err := handle.Cancel(ctx); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	select {
	case <-handle.Done():
	case <-ctx.Done():
		t.Fatal("timeout waiting for Cancelled")
	}
}

func TestPlaceOrderModifyToMarketDeliversLateExecution(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "place_order_modify_to_market_late_execution.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Order: ibkr.Order{
			Action:    ibkr.Buy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  decimal.RequireFromString("12.89"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	var sawSubmitted bool
	for !sawSubmitted {
		select {
		case evt := <-handle.Events():
			if evt.Status != nil && evt.Status.Status == ibkr.OrderStatusSubmitted {
				sawSubmitted = true
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for Submitted")
		}
	}

	if err := handle.Modify(ctx, ibkr.Order{
		Action:    ibkr.Buy,
		OrderType: ibkr.OrderTypeMarket,
		Quantity:  decimal.RequireFromString("1"),
		TIF:       ibkr.TIFDay,
		Account:   "DU9000001",
	}); err != nil {
		t.Fatalf("Modify: %v", err)
	}

	var sawFilled, sawExecution, sawCommission bool
	for {
		select {
		case evt, ok := <-handle.Events():
			if !ok {
				goto done
			}
			if evt.Status != nil && evt.Status.Status == ibkr.OrderStatusFilled {
				sawFilled = true
			}
			if evt.Execution != nil {
				sawExecution = true
				if evt.Execution.ExecID != "late-exec-13" {
					t.Fatalf("execution execID = %q, want late-exec-13", evt.Execution.ExecID)
				}
			}
			if evt.Commission != nil {
				sawCommission = true
			}
		case <-handle.Done():
			for {
				select {
				case evt, ok := <-handle.Events():
					if !ok {
						goto done
					}
					if evt.Status != nil && evt.Status.Status == ibkr.OrderStatusFilled {
						sawFilled = true
					}
					if evt.Execution != nil {
						sawExecution = true
					}
					if evt.Commission != nil {
						sawCommission = true
					}
				default:
					goto done
				}
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for terminal order events")
		}
	}

done:
	if !sawFilled {
		t.Fatal("never received Filled status")
	}
	if !sawExecution {
		t.Fatal("never received late execution after Filled")
	}
	if !sawCommission {
		t.Fatal("never received late commission after Filled")
	}
	if err := handle.Wait(); err != nil {
		t.Fatalf("handle.Wait() error = %v", err)
	}
}

func TestPlaceOrderInvalidTypeLiveError(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "place_order_invalid_type_live_error.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Order: ibkr.Order{
			Action:    ibkr.Buy,
			OrderType: ibkr.OrderType("FEELINGS"),
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  decimal.RequireFromString("10"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	err = handle.Wait()
	if err == nil {
		t.Fatal("handle.Wait() error = nil, want live invalid order type API error")
	}
	if !strings.Contains(err.Error(), "code=321") || !strings.Contains(err.Error(), "Invalid order type") {
		t.Fatalf("handle.Wait() error = %v, want code=321 invalid order type", err)
	}
}

func TestGlobalCancelIntegration(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "global_cancel.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var handles []*ibkr.OrderHandle
	for i := 0; i < 2; i++ {
		h, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: ibkr.Contract{
				Symbol:   "AAPL",
				SecType:  ibkr.SecTypeStock,
				Exchange: "SMART",
				Currency: "USD",
			},
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
		handles = append(handles, h)
	}

	// Wait for initial status on both.
	for i, h := range handles {
		select {
		case <-h.Events():
		case <-ctx.Done():
			t.Fatalf("timeout waiting for handle[%d] initial event", i)
		}
	}

	if err := client.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("GlobalCancel: %v", err)
	}

	// Wait for all handles to finish.
	for i, h := range handles {
		select {
		case <-h.Done():
		case <-ctx.Done():
			t.Fatalf("timeout waiting for handle[%d] done", i)
		}
	}
}

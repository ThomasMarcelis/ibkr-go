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

func TestDialContextWithClientID(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "handshake_client_id_0.txt", ibkr.WithClientID(0))
	defer client.Close()
	defer waitHost(t, host)

	if got := client.Session().NextValidID; got != 1001 {
		t.Fatalf("next valid id = %d, want 1001", got)
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

func TestAPIDuplicateQuoteSubscriptionsAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_duplicate_quote_subscriptions_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	contract := ibkr.Contract{
		ConID:    265598,
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	}

	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("MarketData().SetType(MarketDataDelayed) error = %v", err)
	}
	first, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{Contract: contract})
	if err != nil {
		t.Fatalf("first SubscribeQuotes() error = %v", err)
	}
	second, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{Contract: contract})
	if err != nil {
		_ = first.Close()
		t.Fatalf("second SubscribeQuotes() error = %v", err)
	}

	waitDelayedBidAsk := func(label string, events <-chan ibkr.QuoteUpdate) ibkr.Quote {
		t.Helper()

		wantFields := ibkr.QuoteFieldBid | ibkr.QuoteFieldAsk | ibkr.QuoteFieldMarketDataType
		for {
			select {
			case update, ok := <-events:
				if !ok {
					t.Fatalf("%s quote events closed before delayed bid/ask", label)
				}
				if update.Snapshot.Available&wantFields == wantFields {
					return update.Snapshot
				}
			case <-ctx.Done():
				t.Fatalf("timeout waiting for %s delayed bid/ask", label)
			}
		}
	}

	firstQuote := waitDelayedBidAsk("first duplicate subscription", first.Events())
	secondQuote := waitDelayedBidAsk("second duplicate subscription", second.Events())
	for label, quote := range map[string]ibkr.Quote{
		"first":  firstQuote,
		"second": secondQuote,
	} {
		if quote.MarketDataType != ibkr.MarketDataDelayed {
			t.Fatalf("%s market data type = %s, want Delayed", label, quote.MarketDataType)
		}
		if got := quote.Bid.String(); got != "263.45" {
			t.Fatalf("%s bid = %s, want 263.45", label, got)
		}
		if got := quote.Ask.String(); got != "263.48" {
			t.Fatalf("%s ask = %s, want 263.48", label, got)
		}
	}

	if err := first.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if err := first.Wait(); err != nil {
		t.Fatalf("first Wait() error = %v", err)
	}
	if err := second.Wait(); err != nil {
		t.Fatalf("second Wait() error = %v", err)
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

func TestAPIMarketDataTypeCycleReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_market_data_type_cycle.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, dataType := range []ibkr.MarketDataType{
		ibkr.MarketDataLive,
		ibkr.MarketDataFrozen,
		ibkr.MarketDataDelayed,
		ibkr.MarketDataDelayedFrozen,
	} {
		if err := client.MarketData().SetType(ctx, dataType); err != nil {
			t.Fatalf("MarketData().SetType(%s) error = %v", dataType, err)
		}
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

// TestSubscribeQuotesResumeNeverClosesAfter1101 freezes the data-lost
// restoration contract for non-resumable subscriptions: code 1101 means the
// Gateway dropped every data subscription, so a ResumeNever quote stream
// must close with ErrResumeRequired — mirroring its transport-loss close —
// instead of staying open on a stream that will never tick again. The
// trailing CurrentTime call proves the close happened on the 1101 frame
// while the connection was still up (see the fixture header).
func TestSubscribeQuotesResumeNeverClosesAfter1101(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "quote_stream_gap_1101_resume_never.txt")
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

	first := waitForEvent(t, sub.Events())
	if first.Snapshot.Bid.String() != "189.1" {
		t.Fatalf("first bid = %s, want 189.1", first.Snapshot.Bid.String())
	}

	closed := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionClosed)
	if !errors.Is(closed.Err, ibkr.ErrResumeRequired) {
		t.Fatalf("closed.Err = %v, want ErrResumeRequired", closed.Err)
	}
	if !closed.Retryable {
		t.Fatal("closed.Retryable = false, want true")
	}
	if err := sub.Wait(); !errors.Is(err, ibkr.ErrResumeRequired) {
		t.Fatalf("sub.Wait() = %v, want ErrResumeRequired", err)
	}

	if got := client.Session().State; got != ibkr.StateReady {
		t.Fatalf("session state after 1101 = %s, want %s", got, ibkr.StateReady)
	}
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime after 1101 = %v, want success on the live connection", err)
	}
}

// TestOneShotInterruptedBy1101 freezes the data-lost restoration contract
// for in-flight one-shots: the Gateway lost the request with the data
// connection and will never answer it, so the blocked caller must get
// ErrInterrupted on the 1101 frame — mirroring the transport-loss
// interruption — rather than hang until its context deadline. The trailing
// CurrentTime call proves the interruption happened while the connection
// was still up (see the fixture header).
func TestOneShotInterruptedBy1101(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "contract_details_gap_1101.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Contracts().Details(ctx, ibkr.Contract{
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	})
	if !errors.Is(err, ibkr.ErrInterrupted) {
		t.Fatalf("ContractDetails error after 1101 = %v, want ErrInterrupted", err)
	}

	if got := client.Session().State; got != ibkr.StateReady {
		t.Fatalf("session state after 1101 = %s, want %s", got, ibkr.StateReady)
	}
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime after 1101 = %v, want success on the live connection", err)
	}
}

// TestOpenOrdersSnapshotBurstExceedsSubscriptionBuffer uses the first
// open-order snapshot from the live capture frozen in
// open_orders_snapshot_burst_live.txt. The Gateway sent an open-order echo and
// its paired status before open_order_end; a one-shot must retain the complete
// snapshot even when the configured live-stream buffer cannot hold that burst.
func TestOpenOrdersSnapshotBurstExceedsSubscriptionBuffer(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "open_orders_snapshot_burst_live.txt", ibkr.WithClientID(0), ibkr.WithSubscriptionBuffer(1))
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
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  decimal.RequireFromString("50"),
			TIF:       ibkr.TIFGTC,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("Place: %v", err)
	}
	waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusPreSubmitted)

	orders, err := client.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(orders) != 1 || orders[0].OrderID != handle.OrderID() {
		t.Fatalf("orders = %+v, want order %d", orders, handle.OrderID())
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
	if values[0].OrderID != 2001 {
		t.Fatalf("order id = %d, want 2001", values[0].OrderID)
	}
	if values[0].Status != ibkr.OrderStatusSubmitted {
		t.Fatalf("status = %q, want Submitted", values[0].Status)
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

// TestExecutionsBurstExceedsSubscriptionBuffer freezes a finite live execution
// query whose 29 response rows exceed the configured consumer buffer. The
// one-shot must retain every row and return when execution-data-end arrives;
// the capture's fifteenth commission arrived only after that boundary.
func TestExecutionsBurstExceedsSubscriptionBuffer(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "executions.txt", ibkr.WithSubscriptionBuffer(1))
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	updates, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU9000001", Symbol: "AAPL"})
	if err != nil {
		t.Fatalf("Executions: %v", err)
	}
	if len(updates) != 29 {
		t.Fatalf("updates len = %d, want 29 before execution-data-end", len(updates))
	}

	var executionIDs, commissionIDs []string
	for i, update := range updates {
		switch {
		case update.Execution != nil:
			executionIDs = append(executionIDs, update.Execution.ExecID)
		case update.CommissionAndFees != nil:
			commissionIDs = append(commissionIDs, update.CommissionAndFees.ExecID)
		default:
			t.Fatalf("updates[%d] = %#v, want execution or commission", i, update)
		}
	}
	wantExecutionIDs := []string{
		"sanitized-fill-014", "sanitized-fill-015",
		"sanitized-fill-001", "sanitized-fill-002", "sanitized-fill-003",
		"sanitized-fill-004", "sanitized-fill-005", "sanitized-fill-006",
		"sanitized-fill-007", "sanitized-fill-008", "sanitized-fill-009",
		"sanitized-fill-010", "sanitized-fill-011", "sanitized-fill-012",
		"sanitized-fill-013",
	}
	wantCommissionIDs := []string{
		"sanitized-fill-014", "sanitized-fill-015",
		"sanitized-fill-001", "sanitized-fill-002", "sanitized-fill-003",
		"sanitized-fill-004", "sanitized-fill-005", "sanitized-fill-006",
		"sanitized-fill-007", "sanitized-fill-008", "sanitized-fill-009",
		"sanitized-fill-010", "sanitized-fill-011", "sanitized-fill-012",
	}
	if !reflect.DeepEqual(executionIDs, wantExecutionIDs) {
		t.Fatalf("execution IDs = %v, want %v", executionIDs, wantExecutionIDs)
	}
	if !reflect.DeepEqual(commissionIDs, wantCommissionIDs) {
		t.Fatalf("commission IDs = %v, want %v", commissionIDs, wantCommissionIDs)
	}

	first := updates[0].Execution
	if first.Price.String() != "292.76" || first.Shares.String() != "1" {
		t.Fatalf("first execution price/shares = %s/%s, want 292.76/1", first.Price, first.Shares)
	}
	wantTime := time.Date(2026, 6, 11, 13, 30, 10, 0, time.UTC)
	if !first.Time.Equal(wantTime) {
		t.Fatalf("first execution time = %s, want %s", first.Time, wantTime)
	}
	if got := updates[15].CommissionAndFees.Amount.String(); got != "1.000003" {
		t.Fatalf("first commission = %s, want 1.000003", got)
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

// TestExecutionsCorrelateCommissionByExecID freezes partitioning across two
// simultaneous, disjoint execution queries: a commission must reach only the
// route that observed its ExecID.
func TestExecutionsCorrelateCommissionByExecID(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "executions_correlated.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	buyCh := make(chan []ibkr.ExecutionUpdate, 1)
	sellCh := make(chan []ibkr.ExecutionUpdate, 1)
	errCh := make(chan error, 2)
	go func() {
		updates, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{
			Account: "DU9000001",
			Symbol:  "AAPL",
			Side:    ibkr.ExecutionFilterBuy,
		})
		if err != nil {
			errCh <- err
			return
		}
		buyCh <- updates
	}()
	time.Sleep(10 * time.Millisecond)
	go func() {
		updates, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{
			Account: "DU9000001",
			Symbol:  "AAPL",
			Side:    ibkr.ExecutionFilterSell,
		})
		if err != nil {
			errCh <- err
			return
		}
		sellCh <- updates
	}()

	var buyUpdates []ibkr.ExecutionUpdate
	var sellUpdates []ibkr.ExecutionUpdate
	for i := 0; i < 2; i++ {
		select {
		case err := <-errCh:
			t.Fatalf("Executions() error = %v", err)
		case buyUpdates = <-buyCh:
		case sellUpdates = <-sellCh:
		}
	}

	assertRoute := func(name string, updates []ibkr.ExecutionUpdate, execID string, side ibkr.ExecutionSide, price, commission string) {
		t.Helper()
		if len(updates) != 2 {
			t.Fatalf("%s updates len = %d, want execution and commission only", name, len(updates))
		}
		execution := updates[0].Execution
		if execution == nil || execution.ExecID != execID || execution.Side != side ||
			!execution.Shares.Equal(decimal.RequireFromString("1")) ||
			!execution.Price.Equal(decimal.RequireFromString(price)) {
			t.Fatalf("%s execution = %#v, want %s %s 1 @ %s", name, execution, execID, side, price)
		}
		fees := updates[1].CommissionAndFees
		if fees == nil || fees.ExecID != execID || fees.Amount == nil ||
			!fees.Amount.Equal(decimal.RequireFromString(commission)) {
			t.Fatalf("%s commission = %#v, want %s amount %s", name, fees, execID, commission)
		}
	}
	assertRoute("BUY", buyUpdates, "sanitized-fill-014", ibkr.ExecutionSideBought, "292.76", "1.000003")
	assertRoute("SELL", sellUpdates, "sanitized-fill-015", ibkr.ExecutionSideSold, "292.70", "1.006228")
}

// TestExecutionsCorrelateCommissionForOverlappingSubscriptions freezes the
// opposite invariant: a commission that races ahead of its execution must be
// retained and delivered once to every overlapping route that observes it.
func TestExecutionsCorrelateCommissionForOverlappingSubscriptions(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "executions_overlapping.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	allCh := make(chan []ibkr.ExecutionUpdate, 1)
	sellCh := make(chan []ibkr.ExecutionUpdate, 1)
	errCh := make(chan error, 2)
	go func() {
		updates, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU9000001", Symbol: "AAPL"})
		if err != nil {
			errCh <- err
			return
		}
		allCh <- updates
	}()
	time.Sleep(10 * time.Millisecond)
	go func() {
		updates, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{
			Account: "DU9000001",
			Symbol:  "AAPL",
			Side:    ibkr.ExecutionFilterSell,
		})
		if err != nil {
			errCh <- err
			return
		}
		sellCh <- updates
	}()

	var allUpdates []ibkr.ExecutionUpdate
	var sellUpdates []ibkr.ExecutionUpdate
	for i := 0; i < 2; i++ {
		select {
		case err := <-errCh:
			t.Fatalf("Executions() error = %v", err)
		case allUpdates = <-allCh:
		case sellUpdates = <-sellCh:
		}
	}

	assertRoute := func(name string, updates []ibkr.ExecutionUpdate) {
		t.Helper()
		if len(updates) != 2 {
			t.Fatalf("%s updates len = %d, want one execution and one commission", name, len(updates))
		}
		execution := updates[0].Execution
		if execution == nil || execution.ExecID != "sanitized-fill-012" || execution.Side != ibkr.ExecutionSideSold ||
			!execution.Shares.Equal(decimal.RequireFromString("40")) ||
			!execution.Price.Equal(decimal.RequireFromString("292.20")) {
			t.Fatalf("%s execution = %#v, want sanitized-fill-012 SLD 40 @ 292.20", name, execution)
		}
		fees := updates[1].CommissionAndFees
		if fees == nil || fees.ExecID != "sanitized-fill-012" || fees.Amount == nil ||
			!fees.Amount.Equal(decimal.RequireFromString("0.248693")) {
			t.Fatalf("%s commission = %#v, want sanitized-fill-012 amount 0.248693", name, fees)
		}
	}
	assertRoute("all-side", allUpdates)
	assertRoute("SELL", sellUpdates)
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
	if codes[0].AccountID != "*" {
		t.Fatalf("account_id = %q, want *", codes[0].AccountID)
	}
	if codes[0].FamilyCode != "" {
		t.Fatalf("family_code = %q, want empty", codes[0].FamilyCode)
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
	if len(providers) != 3 {
		t.Fatalf("providers len = %d, want 3", len(providers))
	}
	wantProviders := []ibkr.NewsProvider{
		{Code: "BRFG", Name: "Briefing.com General Market Columns"},
		{Code: "BRFUPDN", Name: "Briefing.com Analyst Actions"},
		{Code: "DJNL", Name: "Dow Jones Newsletters"},
	}
	for i, want := range wantProviders {
		if providers[i] != want {
			t.Errorf("providers[%d] = %+v, want %+v", i, providers[i], want)
		}
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

func TestScannerSubscriptionReturnsLiveRankedResults(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "scanner_subscription_live.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.Scanner().SubscribeResults(ctx, ibkr.ScannerSubscriptionRequest{
		NumberOfRows: 10,
		Instrument:   "STK",
		LocationCode: "STK.US.MAJOR",
		ScanCode:     "HOT_BY_VOLUME",
	})
	if err != nil {
		t.Fatalf("SubscribeResults() error = %v", err)
	}

	var results []ibkr.ScannerResult
	select {
	case results = <-sub.Events():
	case <-ctx.Done():
		t.Fatalf("scanner results: %v", ctx.Err())
	}
	if len(results) != 10 {
		t.Fatalf("results len = %d, want 10", len(results))
	}
	if got := results[0]; got.Rank != 0 || got.Contract.ConID != 888872117 || got.Contract.Symbol != "BGIA" {
		t.Fatalf("first result = %+v, want live rank 0 BGIA contract 888872117", got)
	}
	if got := results[9]; got.Rank != 9 || got.Contract.Symbol != "FAB" {
		t.Fatalf("last result = %+v, want live rank 9 FAB", got)
	}

	if err := sub.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := sub.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
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
	if len(rule.Increments) != 1 {
		t.Fatalf("increments len = %d, want 1", len(rule.Increments))
	}
	if rule.Increments[0].LowEdge.String() != "0" || rule.Increments[0].Increment.String() != "0.01" {
		t.Fatalf("increment = %+v, want low edge 0 increment 0.01", rule.Increments[0])
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
	if order.Contract.ConID != 265598 || order.Contract.Right != "" {
		t.Fatalf("contract = %+v, want AAPL contract with unset right", order.Contract)
	}
	if order.Order.Action != ibkr.ActionSell || order.Order.OrderType != ibkr.OrderTypeLimit ||
		order.Order.Quantity.String() != "1" || order.Order.TIF != ibkr.TIFGTC {
		t.Fatalf("order = %+v, want SELL 1 LMT GTC", order.Order)
	}
	if order.Order.Prices.LmtPrice == nil || order.Order.Prices.LmtPrice.String() != "500" {
		t.Fatalf("limit price = %v, want explicit 500", order.Order.Prices.LmtPrice)
	}
	if order.Order.Prices.AuxPrice == nil || !order.Order.Prices.AuxPrice.IsZero() {
		t.Fatalf("aux price = %v, want explicit zero", order.Order.Prices.AuxPrice)
	}
	if order.Order.OCA.Group != "1001" || order.Order.OCA.Type != ibkr.OCAReduceWithoutBlock {
		t.Fatalf("OCA = %+v, want group 1001 type 3", order.Order.OCA)
	}
	if order.Order.PermID == nil || *order.Order.PermID != 1002 || order.Order.Account != "DU12345" {
		t.Fatalf("identity = account %q perm %v, want DU12345/1002", order.Order.Account, order.Order.PermID)
	}
	if order.Order.Execution.DisplaySize != nil || order.Order.Scale.InitialLevelSize != nil ||
		order.Order.Scale.SubsequentLevelSize != nil || order.Order.Execution.RefFuturesConID != nil {
		t.Fatalf("integer sentinels were exposed as values: execution=%+v scale=%+v", order.Order.Execution, order.Order.Scale)
	}
	if order.Order.Prices.StopPrice != nil || order.Order.Prices.LmtPriceOffset != nil ||
		order.Order.Routing.ExemptCode != nil {
		t.Fatalf("numeric sentinels were exposed as values: prices=%+v routing=%+v", order.Order.Prices, order.Order.Routing)
	}
	if order.Order.Volatility.DeltaNeutral != nil || order.Contract.DeltaNeutral != nil {
		t.Fatalf("inactive delta-neutral blocks = %+v/%+v, want nil", order.Order.Volatility.DeltaNeutral, order.Contract.DeltaNeutral)
	}
	if order.Order.Hedge.DisableAutomaticPrice == nil || !*order.Order.Hedge.DisableAutomaticPrice {
		t.Fatalf("disable automatic hedge price = %v, want explicit true", order.Order.Hedge.DisableAutomaticPrice)
	}
	if order.Completion.Status != ibkr.OrderStatusCancelled || !order.Completion.Filled.IsZero() {
		t.Fatalf("completion = %+v, want Cancelled with zero filled", order.Completion)
	}
	if order.Completion.ParentPermID == nil || *order.Completion.ParentPermID != 1001 {
		t.Fatalf("parent perm id = %v, want 1001", order.Completion.ParentPermID)
	}
	if order.Completion.AutoCancelDate != "20260930 22:00:00 Central European Standard Time" ||
		order.Completion.Time != "20260411 22:18:53 US/Eastern" ||
		order.Completion.StatusText != "Cancelled by System:\n" {
		t.Fatalf("completion metadata = %+v", order.Completion)
	}
	if order.Order.Compliance.Shareholder != "Not an insider or substantial shareholder" {
		t.Fatalf("shareholder = %q", order.Order.Compliance.Shareholder)
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

	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("SetType() error = %v", err)
	}
	sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{Contract: ibkr.Contract{
		ConID: 265598, Symbol: "AAPL", SecType: ibkr.SecTypeStock, Exchange: "SMART", Currency: "USD",
	}}, ibkr.WithResumePolicy(ibkr.ResumeNever))
	if err != nil {
		t.Fatalf("SubscribeQuotes() error = %v", err)
	}

	bboExchange := ""
	for bboExchange == "" {
		select {
		case update, ok := <-sub.Events():
			if !ok {
				t.Fatalf("quote closed before parameters: %v", sub.Err())
			}
			if update.Kind == ibkr.QuoteUpdateParameters {
				bboExchange = update.Parameters.BBOExchange
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	components, err := client.Contracts().SmartComponents(ctx, bboExchange)
	if err != nil {
		t.Fatalf("SmartComponents() error = %v", err)
	}
	if len(components) != 20 {
		t.Fatalf("components len = %d, want 20", len(components))
	}
	if components[0].ExchangeName != "AMEX" || components[0].ExchangeLetter != "A" {
		t.Fatalf("first component = %+v, want AMEX/A", components[0])
	}
	if components[10].ExchangeName != "IEX" || components[10].ExchangeLetter != "V" {
		t.Fatalf("IEX component = %+v, want IEX/V", components[10])
	}
	if components[19].ExchangeName != "TXSE" || components[19].ExchangeLetter != "F" {
		t.Fatalf("last component = %+v, want TXSE/F", components[19])
	}
	if err := sub.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := sub.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
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

	client, host := newClient(t, "historical_news_end_bound_sv206_live.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	lowerBound := time.Date(2026, 1, 10, 23, 11, 4, 0, time.UTC)

	items, err := client.News().Historical(ctx, ibkr.HistoricalNewsRequest{
		ConID:         265598,
		ProviderCodes: []ibkr.NewsProviderCode{"BRFG", "BRFUPDN", "DJNL"},
		EndTime:       lowerBound,
		TotalResults:  20,
	})
	if err != nil {
		t.Fatalf("HistoricalNews() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items len = %d, want 2", len(items))
	}
	if !items[0].Time.Equal(time.Date(2026, 5, 1, 14, 27, 6, 0, time.UTC)) || items[0].ProviderCode != "BRFG" {
		t.Fatalf("first item = %+v", items[0])
	}
	for _, item := range items {
		if item.Time.Before(lowerBound) {
			t.Fatalf("item %s is before lower bound %s", item.Time, lowerBound)
		}
	}
}

func TestOrderEventBufferOverflowPreservesOrderCoordinate(t *testing.T) {
	t.Parallel()

	// captures/20260413T192703Z-place_order_mkt_buy_aapl, server_version 200,
	// events.jsonl sha256 prefix 301b075b217cbd99. The grounded replay's exact
	// order is OpenOrder(PreSubmitted), PreSubmitted status, Execution,
	// OpenOrder(Filled), Filled status, then CommissionAndFees. A four-event
	// queue must close when the fifth event arrives instead of silently dropping
	// it and continuing.
	client, host := newClient(t, "place_order_fill_native_execution_time.txt", ibkr.WithOrderEventBuffer(4))
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
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("1"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	select {
	case <-handle.Done():
	case <-ctx.Done():
		t.Fatal("timeout waiting for order event overflow")
	}
	if err := handle.Wait(); !errors.Is(err, ibkr.ErrSlowConsumer) {
		t.Fatalf("Wait() error = %v, want ErrSlowConsumer", err)
	}

	var events []ibkr.OrderEvent
	for event := range handle.Events() {
		events = append(events, event)
	}
	if len(events) != 4 {
		t.Fatalf("buffered event count = %d, want configured capacity 4", len(events))
	}
	if events[0].OpenOrder == nil || events[0].OpenOrder.Status != ibkr.OrderStatusPreSubmitted ||
		events[1].Status == nil || events[1].Status.Status != ibkr.OrderStatusPreSubmitted ||
		events[2].Execution == nil || events[2].Execution.ExecID != "sanitized-native-exec-001" ||
		events[3].OpenOrder == nil || events[3].OpenOrder.Status != ibkr.OrderStatusFilled {
		t.Fatalf("buffered live-derived prefix = %+v, want OpenOrder(PreSubmitted), PreSubmitted, Execution, OpenOrder(Filled)", events)
	}

	// Observation has ended, but this stable server coordinate remains the
	// authority for open-order reconciliation or direct Orders().Cancel.
	if orderID := handle.OrderID(); orderID != 1 {
		t.Fatalf("OrderID after observation overflow = %d, want reconciliation coordinate 1", orderID)
	}
}

func TestPlaceOrderWithNativeExecutionTime(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "place_order_fill_native_execution_time.txt")
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
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("1"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	var sawFilled bool
	var execution *ibkr.Execution
	var commission *ibkr.CommissionAndFeesReport
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
				execution = evt.Execution
			}
			if evt.CommissionAndFees != nil {
				commission = evt.CommissionAndFees
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for native execution-time order events")
		}
	}

done:
	if !sawFilled {
		t.Fatal("never received Filled status")
	}
	if execution == nil {
		t.Fatal("never received Execution event")
		return
	}
	if execution.ExecID != "sanitized-native-exec-001" {
		t.Fatalf("Execution.ExecID = %q", execution.ExecID)
	}
	wantTime := time.Date(2026, 4, 13, 19, 27, 4, 0, time.UTC)
	if !execution.Time.Equal(wantTime) {
		t.Fatalf("Execution.Time = %s, want %s", execution.Time.Format(time.RFC3339), wantTime.Format(time.RFC3339))
	}
	if execution.Price.String() != "257.95" {
		t.Fatalf("Execution.Price = %s, want 257.95", execution.Price.String())
	}
	if commission == nil {
		t.Fatal("never received Commission event")
		return
	}
	if commission.ExecID != execution.ExecID {
		t.Fatalf("Commission.ExecID = %q, want %q", commission.ExecID, execution.ExecID)
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
	events := client.SessionEvents()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  decimal.RequireFromString("50"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if handle.OrderID() != 350 {
		t.Fatalf("OrderID = %d, want 350", handle.OrderID())
	}

	// Preserve the captured rest chronology before sending the direct cancel.
	sawPreSubmitted := false
statuses:
	for {
		evt := waitForEvent(t, handle.Events())
		if evt.Status == nil {
			continue
		}
		switch evt.Status.Status {
		case ibkr.OrderStatusPreSubmitted:
			sawPreSubmitted = true
		case ibkr.OrderStatusSubmitted:
			break statuses
		}
	}
	if !sawPreSubmitted {
		t.Fatal("Submitted arrived without the captured PreSubmitted transition")
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
	notice := waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderCanceled)
	if notice.Message != "Order Canceled - reason:" {
		t.Fatalf("code-202 message = %q, want %q", notice.Message, "Order Canceled - reason:")
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

// Regression: cancel_order at server_version >= 192 requires extOperator and
// manualOrderIndicator fields (CME_TAGGING_FIELDS). Missing fields caused the
// gateway to silently drop the cancel. This test uses the full
// PreSubmitted → Submitted → PendingCancel → Cancelled lifecycle grounded from
// live paper Gateway sv=200 on 2026-04-14.
func TestAPIOrderRestCancelAAPL(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_order_rest_cancel_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			ConID: 265598, Symbol: "AAPL", SecType: ibkr.SecTypeStock,
			Exchange: "SMART", Currency: "USD",
		},
		Order: ibkr.Order{
			Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeLimit,
			Quantity: decimal.RequireFromString("1"),
			LmtPrice: decimal.RequireFromString("10"),
			TIF:      ibkr.TIFDay, Account: "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	// Consume events until Submitted.
	for {
		evt := waitForEvent(t, handle.Events())
		if evt.Status != nil && evt.Status.Status == ibkr.OrderStatusSubmitted {
			break
		}
	}

	if err := handle.Cancel(ctx); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	// Drain until Cancelled; a following code 202 cancellation notice must not
	// convert a successful terminal status into handle error.
	var sawPendingCancel, sawCancelled bool
	for evt := range handle.Events() {
		if evt.Status != nil {
			switch evt.Status.Status {
			case ibkr.OrderStatusPendingCancel:
				sawPendingCancel = true
			case ibkr.OrderStatusCancelled:
				sawCancelled = true
			}
		}
	}
	if !sawPendingCancel {
		t.Error("expected PendingCancel status before Cancelled")
	}
	if !sawCancelled {
		t.Fatal("never received Cancelled status")
	}
	if err := handle.Wait(); err != nil {
		t.Fatalf("handle.Wait() error = %v, want nil after cancellation notice", err)
	}
}

func newClient(t *testing.T, script string, opts ...ibkr.Option) (*ibkr.Client, *testhost.Host) {
	t.Helper()

	host := newHost(t, script)
	client := dialHostClient(t, host, opts...)
	return client, host
}

func dialHostClient(t *testing.T, host *testhost.Host, opts ...ibkr.Option) *ibkr.Client {
	t.Helper()

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
	return client
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

func TestGroundedAccountSummaryBurstExceedsSubscriptionBuffer(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "grounded_account_summary.txt", ibkr.WithSubscriptionBuffer(1))
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
		return
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
		return
	}
	if aapl.Position.String() != "10" {
		t.Errorf("AAPL position = %s, want 10", aapl.Position.String())
	}
	if aapl.AvgCost.String() != "256.1" {
		t.Errorf("AAPL avgCost = %s, want 256.1", aapl.AvgCost.String())
	}

	if yw == nil {
		t.Fatal("YW position not found")
		return
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
		return
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
	// The wire strike "500.0" normalizes to "500" through decimal; compare by
	// value, never by string.
	if qqq.Contract.Strike == nil || !qqq.Contract.Strike.Equal(decimal.RequireFromString("500.0")) {
		t.Errorf("QQQ strike = %s, want 500.0", qqq.Contract.Strike)
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
		Account:   "DU9000001",
		ModelCode: "",
	})
	if err != nil {
		t.Fatalf("PositionsMultiSnapshot() error = %v", err)
	}
	if len(values) != 0 {
		t.Fatalf("values = %+v, want empty live snapshot", values)
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
		Account:   "DU9000001",
		ModelCode: "",
	})
	if err != nil {
		t.Fatalf("SubscribePnL() error = %v", err)
	}

	waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionStarted)

	update := waitForEvent(t, sub.Events())
	if update.DailyPnL.String() != "11340.427636781911" {
		t.Fatalf("daily pnl = %s, want 11340.427636781911", update.DailyPnL.String())
	}
	if update.UnrealizedPnL.String() != "54385.58271885987" {
		t.Fatalf("unrealized pnl = %s, want 54385.58271885987", update.UnrealizedPnL.String())
	}
	if update.RealizedPnL.String() != "-103.92738339177643" {
		t.Fatalf("realized pnl = %s, want -103.92738339177643", update.RealizedPnL.String())
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

func TestDisplayGroupLifecycleIntegration(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "display_group_subscribe.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	groups, err := client.TWS().DisplayGroups(ctx)
	if err != nil {
		t.Fatalf("DisplayGroups() error = %v", err)
	}
	wantGroups := []ibkr.DisplayGroupID{1, 2, 3, 4, 5, 6, 7}
	if !reflect.DeepEqual(groups, wantGroups) {
		t.Fatalf("DisplayGroups() = %v, want %v", groups, wantGroups)
	}

	handle, err := client.TWS().SubscribeDisplayGroup(ctx, groups[0])
	if err != nil {
		t.Fatalf("SubscribeDisplayGroup() error = %v", err)
	}

	waitForStateKind(t, handle.Lifecycle(), ibkr.SubscriptionStarted)

	// Read initial "none" update.
	initial := waitForEvent(t, handle.Events())
	if initial.ContractInfo != "none" {
		t.Fatalf("initial ContractInfo = %q, want %q", initial.ContractInfo, "none")
	}

	if err := handle.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := handle.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime() fence error = %v", err)
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
			Action:    ibkr.ActionBuy,
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
		Action:    ibkr.ActionBuy,
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
			if evt.CommissionAndFees != nil {
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
					if evt.CommissionAndFees != nil {
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
			Action:    ibkr.ActionBuy,
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

func TestAPIIOCFOKAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_ioc_fok_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ioc, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  decimal.RequireFromString("309.6"),
			TIF:       ibkr.TIFIOC,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("IOC PlaceOrder: %v", err)
	}
	iocStatuses := waitOrderStatuses(t, ctx, ioc)
	if !hasOrderStatus(iocStatuses, ibkr.OrderStatusPendingCancel) {
		t.Fatalf("IOC statuses = %v, want PendingCancel from live capture", iocStatuses)
	}
	if !hasOrderStatus(iocStatuses, ibkr.OrderStatusCancelled) {
		t.Fatalf("IOC statuses = %v, want Cancelled from live capture", iocStatuses)
	}

	fokMarketable, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  decimal.RequireFromString("309.6"),
			TIF:       ibkr.TIFFOK,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("FOK marketable PlaceOrder: %v", err)
	}
	fokMarketableStatuses := waitOrderStatuses(t, ctx, fokMarketable)
	if !hasOrderStatus(fokMarketableStatuses, ibkr.OrderStatusInactive) {
		t.Fatalf("FOK marketable statuses = %v, want Inactive from live capture", fokMarketableStatuses)
	}

	fokFar, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  decimal.RequireFromString("12.9"),
			TIF:       ibkr.TIFFOK,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("FOK far PlaceOrder: %v", err)
	}
	fokFarStatuses := waitOrderStatuses(t, ctx, fokFar)
	if !hasOrderStatus(fokFarStatuses, ibkr.OrderStatusInactive) {
		t.Fatalf("FOK far statuses = %v, want Inactive from live capture", fokFarStatuses)
	}
}

func TestAPITIFAttributeMatrixAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_tif_attribute_matrix_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gtc, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  decimal.RequireFromString("10"),
			TIF:       ibkr.TIFGTC,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("GTC PlaceOrder: %v", err)
	}
	for {
		evt := waitForEvent(t, gtc.Events())
		if evt.Status != nil && evt.Status.Status == ibkr.OrderStatusSubmitted {
			break
		}
	}
	if err := gtc.Cancel(ctx); err != nil {
		t.Fatalf("GTC Cancel: %v", err)
	}
	gtcStatuses := waitOrderStatuses(t, ctx, gtc)
	if !hasOrderStatus(gtcStatuses, ibkr.OrderStatusCancelled) {
		t.Fatalf("GTC statuses = %v, want Cancelled from live capture", gtcStatuses)
	}

	trailing, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Order: ibkr.Order{
			Action:          ibkr.ActionSell,
			OrderType:       ibkr.OrderTypeTrailingStop,
			Quantity:        decimal.RequireFromString("1"),
			TIF:             ibkr.TIFDay,
			Account:         "DU9000001",
			TrailStopPrice:  decimal.RequireFromString("2000"),
			TrailingPercent: decimal.RequireFromString("1.5"),
		},
	})
	if err != nil {
		t.Fatalf("TrailingPercent PlaceOrder: %v", err)
	}
	var sawExecution, sawFilled bool
	for {
		select {
		case evt, ok := <-trailing.Events():
			if !ok {
				goto trailingDone
			}
			if evt.Execution != nil {
				sawExecution = true
			}
			if evt.Status != nil && evt.Status.Status == ibkr.OrderStatusFilled {
				sawFilled = true
			}
		case <-trailing.Done():
			for {
				select {
				case evt, ok := <-trailing.Events():
					if !ok {
						goto trailingDone
					}
					if evt.Execution != nil {
						sawExecution = true
					}
					if evt.Status != nil && evt.Status.Status == ibkr.OrderStatusFilled {
						sawFilled = true
					}
				default:
					goto trailingDone
				}
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for trailing-percent live replay")
		}
	}

trailingDone:
	if !sawExecution {
		t.Fatal("never received trailing-percent execution from live capture")
	}
	if !sawFilled {
		t.Fatal("never received trailing-percent Filled status from live capture")
	}
	if err := trailing.Wait(); err != nil {
		t.Fatalf("TrailingPercent Wait: %v", err)
	}
}

func TestAPIStopLossManagementAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_stop_loss_management_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	contract := ibkr.Contract{
		ConID:    265598,
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	}

	buy, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: contract,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("1"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("entry PlaceOrder: %v", err)
	}
	buyFilled, buyExecution := waitOrderFillAndExecution(t, ctx, buy)
	if !buyFilled || !buyExecution {
		t.Fatalf("entry filled=%v execution=%v, want both true", buyFilled, buyExecution)
	}

	stopOrder := ibkr.Order{
		Action:    ibkr.ActionSell,
		OrderType: ibkr.OrderTypeStop,
		Quantity:  decimal.RequireFromString("1"),
		AuxPrice:  decimal.RequireFromString("13.13"),
		TIF:       ibkr.TIFDay,
		Account:   "DU9000001",
	}
	stop, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: contract, Order: stopOrder})
	if err != nil {
		t.Fatalf("stop PlaceOrder: %v", err)
	}
	waitForOrderStatus(t, ctx, stop, ibkr.OrderStatusPreSubmitted)

	stopOrder.AuxPrice = decimal.RequireFromString("14.13")
	if err := stop.Modify(ctx, stopOrder); err != nil {
		t.Fatalf("stop Modify: %v", err)
	}
	waitForOrderStatus(t, ctx, stop, ibkr.OrderStatusPreSubmitted)

	if err := stop.Cancel(ctx); err != nil {
		t.Fatalf("stop Cancel: %v", err)
	}
	stopStatuses := waitOrderStatuses(t, ctx, stop)
	if !hasOrderStatus(stopStatuses, ibkr.OrderStatusCancelled) {
		t.Fatalf("stop statuses = %v, want Cancelled from live capture", stopStatuses)
	}

	flatten, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: contract,
		Order: ibkr.Order{
			Action:    ibkr.ActionSell,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("1"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("flatten PlaceOrder: %v", err)
	}
	flatFilled, flatExecution := waitOrderFillAndExecution(t, ctx, flatten)
	if !flatFilled || !flatExecution {
		t.Fatalf("flatten filled=%v execution=%v, want both true", flatFilled, flatExecution)
	}
}

func TestAPIDollarCostAveragingAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_dollar_cost_averaging_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	contract := ibkr.Contract{
		ConID:    265598,
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	}

	for i := 0; i < 3; i++ {
		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: contract,
			Order: ibkr.Order{
				Action:    ibkr.ActionBuy,
				OrderType: ibkr.OrderTypeMarket,
				Quantity:  decimal.RequireFromString("5"),
				TIF:       ibkr.TIFDay,
				Account:   "DU9000001",
			},
		})
		if err != nil {
			t.Fatalf("DCA buy[%d] PlaceOrder: %v", i, err)
		}
		filled, execution := waitOrderFillAndExecution(t, ctx, handle)
		if !filled || !execution {
			t.Fatalf("DCA buy[%d] filled=%v execution=%v, want both true", i, filled, execution)
		}
	}

	flatten, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: contract,
		Order: ibkr.Order{
			Action:    ibkr.ActionSell,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("15"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("DCA flatten PlaceOrder: %v", err)
	}
	filled, execution := waitOrderFillAndExecution(t, ctx, flatten)
	if !filled || !execution {
		t.Fatalf("DCA flatten filled=%v execution=%v, want both true", filled, execution)
	}
}

func TestAPIScaleInCampaignAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_scale_in_campaign_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	contract := ibkr.Contract{
		ConID:    265598,
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	}

	for i := 0; i < 2; i++ {
		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: contract,
			Order: ibkr.Order{
				Action:    ibkr.ActionBuy,
				OrderType: ibkr.OrderTypeMarket,
				Quantity:  decimal.RequireFromString("1"),
				TIF:       ibkr.TIFDay,
				Account:   "DU9000001",
			},
		})
		if err != nil {
			t.Fatalf("scale buy[%d] PlaceOrder: %v", i, err)
		}
		filled, execution := waitOrderFillAndExecution(t, ctx, handle)
		if !filled || !execution {
			t.Fatalf("scale buy[%d] filled=%v execution=%v, want both true", i, filled, execution)
		}
	}

	stop, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: contract,
		Order: ibkr.Order{
			Action:    ibkr.ActionSell,
			OrderType: ibkr.OrderTypeStop,
			Quantity:  decimal.RequireFromString("2"),
			AuxPrice:  decimal.RequireFromString("12.98"),
			TIF:       ibkr.TIFGTC,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("scale stop-loss PlaceOrder: %v", err)
	}
	stopOpen := waitForOpenOrder(t, ctx, stop)
	if stopOpen.OrderType != ibkr.OrderTypeStop || stopOpen.Action != ibkr.ActionSell {
		t.Fatalf("scale stop OpenOrder type/action = %s/%s, want STP/SELL", stopOpen.OrderType, stopOpen.Action)
	}
	if got := stopOpen.Quantity.String(); got != "2" {
		t.Fatalf("scale stop quantity = %s, want 2", got)
	}
	if got := stopOpen.AuxPrice.String(); got != "12.98" {
		t.Fatalf("scale stop aux price = %s, want 12.98", got)
	}

	var stopStatus ibkr.OrderStatusUpdate
	for sawStopStatus := false; !sawStopStatus; {
		select {
		case evt, ok := <-stop.Events():
			if !ok {
				t.Fatal("scale stop events closed before PreSubmitted status")
			}
			if evt.Status != nil && evt.Status.Status == ibkr.OrderStatusPreSubmitted {
				stopStatus = *evt.Status
				sawStopStatus = true
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for scale stop PreSubmitted status")
		}
	}
	if stopStatus.WhyHeld != "trigger" {
		t.Fatalf("scale stop whyHeld = %q, want trigger", stopStatus.WhyHeld)
	}
}

func TestAPIStressRapidFireAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_stress_rapid_fire_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	contract := ibkr.Contract{
		ConID:    265598,
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	}
	prices := []string{"12.98", "13.98", "14.98", "15.98", "16.98", "17.98", "18.98", "19.98", "20.98", "21.98"}
	handles := make([]*ibkr.OrderHandle, 0, len(prices))
	orderIDs := map[int64]bool{}
	for i, price := range prices {
		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: contract,
			Order: ibkr.Order{
				Action:    ibkr.ActionBuy,
				OrderType: ibkr.OrderTypeLimit,
				Quantity:  decimal.RequireFromString("1"),
				LmtPrice:  decimal.RequireFromString(price),
				TIF:       ibkr.TIFDay,
				Account:   "DU9000001",
			},
		})
		if err != nil {
			t.Fatalf("stress order[%d] PlaceOrder: %v", i, err)
		}
		if orderIDs[handle.OrderID()] {
			t.Fatalf("stress order[%d] reused order ID %d", i, handle.OrderID())
		}
		orderIDs[handle.OrderID()] = true
		handles = append(handles, handle)
	}
	if len(orderIDs) != len(prices) {
		t.Fatalf("distinct order IDs = %d, want %d", len(orderIDs), len(prices))
	}

	for i, handle := range handles {
		waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusSubmitted)
		if got := handle.OrderID(); got == 0 {
			t.Fatalf("stress order[%d] order ID = 0", i)
		}
	}

	if err := client.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("CancelAll: %v", err)
	}
	for i, handle := range handles {
		statuses := waitOrderStatuses(t, ctx, handle)
		if !hasOrderStatus(statuses, ibkr.OrderStatusCancelled) {
			t.Fatalf("stress order[%d] statuses = %v, want Cancelled from live capture", i, statuses)
		}
		if err := handle.Wait(); err != nil {
			t.Fatalf("stress order[%d] Wait: %v", i, err)
		}
	}
}

func TestAPIForexLifecycleEURUSDReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_forex_lifecycle_eurusd.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			Symbol:   "EUR",
			SecType:  ibkr.SecTypeForex,
			Exchange: "IDEALPRO",
			Currency: "USD",
		},
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("20000"),
			LmtPrice:  decimal.RequireFromString("0.99"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("forex PlaceOrder: %v", err)
	}

	statuses := waitOrderStatuses(t, ctx, handle)
	if !hasOrderStatus(statuses, ibkr.OrderStatusInactive) {
		t.Fatalf("forex statuses = %v, want Inactive from live capture", statuses)
	}
	err = handle.Wait()
	if err == nil {
		t.Fatal("forex Wait error = nil, want live leverage rejection")
	}
	apiErr, ok := errors.AsType[*ibkr.APIError](err)
	if !ok {
		t.Fatalf("forex Wait error type = %T, want *ibkr.APIError", err)
	}
	if apiErr.Code != 201 || !strings.Contains(apiErr.Message, "currency leverage") {
		t.Fatalf("forex Wait error = %v, want code=201 currency leverage rejection", err)
	}
}

func TestAPIBracketTriggerAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_bracket_trigger_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	contract := ibkr.Contract{
		ConID:    265598,
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	}
	bracket, err := client.Orders().PlaceBracket(ctx, ibkr.PlaceBracketRequest{
		Contract: contract,
		Parent: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("1"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
		TakeProfit: ibkr.Order{
			Action:    ibkr.ActionSell,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  decimal.RequireFromString("2578.5"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
		StopLoss: ibkr.Order{
			Action:    ibkr.ActionSell,
			OrderType: ibkr.OrderTypeStop,
			Quantity:  decimal.RequireFromString("1"),
			AuxPrice:  decimal.RequireFromString("12.89"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("PlaceBracket: %v", err)
	}
	parent := bracket.Parent
	takeProfit := bracket.TakeProfit
	stopLoss := bracket.StopLoss

	parentFilled, parentExecution := waitOrderFillAndExecution(t, ctx, parent)
	if !parentFilled || !parentExecution {
		t.Fatalf("bracket parent filled=%v execution=%v, want both true", parentFilled, parentExecution)
	}
	takeProfitOpen := waitForOpenOrder(t, ctx, takeProfit)
	stopLossOpen := waitForOpenOrder(t, ctx, stopLoss)
	if takeProfitOpen.ParentID != parent.OrderID() {
		t.Fatalf("take-profit parent = %d, want %d", takeProfitOpen.ParentID, parent.OrderID())
	}
	if stopLossOpen.ParentID != parent.OrderID() {
		t.Fatalf("stop-loss parent = %d, want %d", stopLossOpen.ParentID, parent.OrderID())
	}
	if takeProfitOpen.OcaGroup == "" || stopLossOpen.OcaGroup != takeProfitOpen.OcaGroup {
		t.Fatalf("child OCA groups = %q and %q, want same non-empty group", takeProfitOpen.OcaGroup, stopLossOpen.OcaGroup)
	}

	err = takeProfit.Modify(ctx, ibkr.Order{
		Action:    ibkr.ActionSell,
		OrderType: ibkr.OrderTypeLimit,
		Quantity:  decimal.RequireFromString("1"),
		LmtPrice:  decimal.RequireFromString("206.28"),
		TIF:       ibkr.TIFDay,
		Account:   "DU9000001",
		ParentID:  parent.OrderID(),
	})
	if err != nil {
		t.Fatalf("bracket take-profit Modify: %v", err)
	}
	statuses := waitOrderStatuses(t, ctx, takeProfit)
	if !hasOrderStatus(statuses, ibkr.OrderStatusPendingCancel) {
		t.Fatalf("bracket take-profit statuses = %v, want PendingCancel from live capture", statuses)
	}
	if !hasOrderStatus(statuses, ibkr.OrderStatusCancelled) {
		t.Fatalf("bracket take-profit statuses = %v, want Cancelled from live capture", statuses)
	}
	if err := client.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("bracket cleanup CancelAll: %v", err)
	}
}

func TestAPIBracketTrailingStopAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_bracket_trailing_stop_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	contract := ibkr.Contract{
		ConID:    265598,
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	}
	parent, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: contract,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("10"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			Transmit:  new(false),
		},
	})
	if err != nil {
		t.Fatalf("trailing bracket parent PlaceOrder: %v", err)
	}
	takeProfit, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: contract,
		Order: ibkr.Order{
			Action:    ibkr.ActionSell,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("10"),
			LmtPrice:  decimal.RequireFromString("2649.6"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			ParentID:  parent.OrderID(),
			Transmit:  new(false),
		},
	})
	if err != nil {
		t.Fatalf("trailing bracket take-profit PlaceOrder: %v", err)
	}
	if takeProfit.OrderID() != parent.OrderID()+1 {
		t.Fatalf("take-profit order ID = %d, want parent+1 from live capture", takeProfit.OrderID())
	}
	trailingStop, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: contract,
		Order: ibkr.Order{
			Action:         ibkr.ActionSell,
			OrderType:      ibkr.OrderTypeTrailingStop,
			Quantity:       decimal.RequireFromString("10"),
			AuxPrice:       decimal.RequireFromString("1"),
			TIF:            ibkr.TIFDay,
			Account:        "DU9000001",
			ParentID:       parent.OrderID(),
			TrailStopPrice: decimal.RequireFromString("13.25"),
		},
	})
	if err != nil {
		t.Fatalf("trailing bracket stop PlaceOrder: %v", err)
	}
	if trailingStop.OrderID() != parent.OrderID()+2 {
		t.Fatalf("trailing stop order ID = %d, want parent+2 from live capture", trailingStop.OrderID())
	}
	err = trailingStop.Wait()
	if err == nil {
		t.Fatal("trailing stop Wait error = nil, want live code 328 rejection")
	}
	apiErr, ok := errors.AsType[*ibkr.APIError](err)
	if !ok {
		t.Fatalf("trailing stop Wait error type = %T, want *ibkr.APIError", err)
	}
	if apiErr.Code != 328 || !strings.Contains(apiErr.Message, "Trailing stop orders can be attached") {
		t.Fatalf("trailing stop Wait error = %v, want code=328 attachment rejection", err)
	}
}

func TestAPIOCATriggerAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_oca_trigger_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	contract := ibkr.Contract{
		ConID:    265598,
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	}
	group := "ibkr-go-api-oca-1776102346"
	resting, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: contract,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  decimal.RequireFromString("12.9"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OCA:       ibkr.OrderOCA{Group: group, Type: ibkr.OCACancelWithBlock},
		},
	})
	if err != nil {
		t.Fatalf("OCA resting PlaceOrder: %v", err)
	}
	restingOpen := waitForOpenOrder(t, ctx, resting)
	if restingOpen.OcaGroup != group {
		t.Fatalf("resting OCA group = %q, want %q", restingOpen.OcaGroup, group)
	}
	waitForOrderStatus(t, ctx, resting, ibkr.OrderStatusPreSubmitted)

	marketable, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: contract,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  decimal.RequireFromString("309.48"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OCA:       ibkr.OrderOCA{Group: group, Type: ibkr.OCACancelWithBlock},
		},
	})
	if err != nil {
		t.Fatalf("OCA marketable PlaceOrder: %v", err)
	}
	marketableOpen := waitForOpenOrder(t, ctx, marketable)
	if marketableOpen.OcaGroup != group {
		t.Fatalf("marketable OCA group = %q, want %q", marketableOpen.OcaGroup, group)
	}
	statuses := waitOrderStatuses(t, ctx, marketable)
	if !hasOrderStatus(statuses, ibkr.OrderStatusPendingCancel) {
		t.Fatalf("OCA marketable statuses = %v, want PendingCancel from live capture", statuses)
	}
	if !hasOrderStatus(statuses, ibkr.OrderStatusCancelled) {
		t.Fatalf("OCA marketable statuses = %v, want Cancelled from live capture", statuses)
	}
	if err := marketable.Wait(); err != nil {
		t.Fatalf("OCA marketable Wait: %v", err)
	}
	if err := client.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("OCA CancelAll: %v", err)
	}
}

func TestAPIPairsTradingAAPLMSFTReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_pairs_trading_aapl_msft.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	aapl := ibkr.Contract{
		ConID:    265598,
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	}
	msft := ibkr.Contract{
		ConID:    272093,
		Symbol:   "MSFT",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	}

	aaplBuy, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: aapl,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("5"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("AAPL pair entry PlaceOrder: %v", err)
	}
	msftSell, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: msft,
		Order: ibkr.Order{
			Action:    ibkr.ActionSell,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("5"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("MSFT pair entry PlaceOrder: %v", err)
	}
	for _, entry := range []struct {
		label  string
		handle *ibkr.OrderHandle
	}{
		{label: "AAPL entry", handle: aaplBuy},
		{label: "MSFT entry", handle: msftSell},
	} {
		filled, execution := waitOrderFillAndExecution(t, ctx, entry.handle)
		if !filled || !execution {
			t.Fatalf("%s filled=%v execution=%v, want both true", entry.label, filled, execution)
		}
	}

	aaplSell, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: aapl,
		Order: ibkr.Order{
			Action:    ibkr.ActionSell,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("5"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("AAPL pair flatten PlaceOrder: %v", err)
	}
	filled, execution := waitOrderFillAndExecution(t, ctx, aaplSell)
	if !filled || !execution {
		t.Fatalf("AAPL flatten filled=%v execution=%v, want both true", filled, execution)
	}

	msftBuy, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: msft,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("5"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("MSFT pair flatten PlaceOrder: %v", err)
	}
	filled, execution = waitOrderFillAndExecution(t, ctx, msftBuy)
	if !filled || !execution {
		t.Fatalf("MSFT flatten filled=%v execution=%v, want both true", filled, execution)
	}
}

func TestAPICompletedOrdersVariantsAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_completed_orders_variants_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	contract := ibkr.Contract{
		ConID:    265598,
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	}

	buy, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: contract,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("5"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("entry PlaceOrder: %v", err)
	}
	buyFilled, buyExecution := waitOrderFillAndExecution(t, ctx, buy)
	if !buyFilled || !buyExecution {
		t.Fatalf("entry filled=%v execution=%v, want both true", buyFilled, buyExecution)
	}

	sell, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: contract,
		Order: ibkr.Order{
			Action:    ibkr.ActionSell,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("5"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("flatten PlaceOrder: %v", err)
	}
	sellFilled, sellExecution := waitOrderFillAndExecution(t, ctx, sell)
	if !sellFilled || !sellExecution {
		t.Fatalf("flatten filled=%v execution=%v, want both true", sellFilled, sellExecution)
	}

	allCompleted, err := client.Orders().Completed(ctx, false)
	if err != nil {
		t.Fatalf("Completed(false): %v", err)
	}
	if len(allCompleted) != 1 {
		t.Fatalf("Completed(false) len = %d, want 1", len(allCompleted))
	}
	if allCompleted[0].Contract.Symbol != "AAPL" || allCompleted[0].Completion.Status != ibkr.OrderStatusFilled {
		t.Fatalf("Completed(false)[0] = %+v, want filled AAPL order", allCompleted[0])
	}

	apiCompleted, err := client.Orders().Completed(ctx, true)
	if err != nil {
		t.Fatalf("Completed(true): %v", err)
	}
	if len(apiCompleted) != 1 {
		t.Fatalf("Completed(true) len = %d, want 1", len(apiCompleted))
	}
	if apiCompleted[0].Contract.Symbol != "AAPL" || apiCompleted[0].Completion.Status != ibkr.OrderStatusFilled {
		t.Fatalf("Completed(true)[0] = %+v, want filled AAPL order", apiCompleted[0])
	}
}

func TestAPIFutureCampaignMESReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_future_campaign_mes.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	contract := ibkr.Contract{
		ConID:    770561194,
		Symbol:   "MES",
		SecType:  ibkr.SecTypeFuture,
		Exchange: "CME",
		Currency: "USD",
	}

	buy, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: contract,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("1"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("future buy PlaceOrder: %v", err)
	}
	buyFilled, buyExecution := waitOrderFillAndExecution(t, ctx, buy)
	if !buyFilled || !buyExecution {
		t.Fatalf("future buy filled=%v execution=%v, want both true", buyFilled, buyExecution)
	}

	sell, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: contract,
		Order: ibkr.Order{
			Action:    ibkr.ActionSell,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("1"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("future flatten PlaceOrder: %v", err)
	}
	sellFilled, sellExecution := waitOrderFillAndExecution(t, ctx, sell)
	if !sellFilled || !sellExecution {
		t.Fatalf("future flatten filled=%v execution=%v, want both true", sellFilled, sellExecution)
	}
}

func TestAPIReconnectActiveOrderAAPLReplay(t *testing.T) {
	t.Parallel()

	host := newHost(t, "api_reconnect_active_order_aapl.txt")
	defer waitHost(t, host)

	first := dialHostClient(t, host, ibkr.WithClientID(1))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	handle, err := first.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("10"),
			LmtPrice:  decimal.RequireFromString("13.27"),
			TIF:       ibkr.TIFGTC,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("first Place: %v", err)
	}
	waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusSubmitted)
	if err := first.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	second := dialHostClient(t, host, ibkr.WithClientID(1))
	defer second.Close()

	orders, err := second.Orders().Open(ctx, ibkr.OpenOrdersScopeClient)
	if err != nil {
		t.Fatalf("second Open(client): %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("open orders len = %d, want 1", len(orders))
	}
	if orders[0].OrderID != handle.OrderID() || orders[0].TIF != ibkr.TIFGTC || orders[0].Quantity.String() != "10" {
		t.Fatalf("open order = %+v, want reconnected GTC order %d qty 10", orders[0], handle.OrderID())
	}
	if err := second.Orders().Cancel(ctx, handle.OrderID()); err != nil {
		t.Fatalf("second Cancel: %v", err)
	}
	waitHost(t, host)
}

func TestAPIOrderHandleReconnectCancelAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_order_handle_reconnect_cancel_aapl.txt",
		ibkr.WithReconnectPolicy(ibkr.ReconnectAuto))
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
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("10"),
			LmtPrice:  decimal.RequireFromString("13.27"),
			TIF:       ibkr.TIFGTC,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusSubmitted)

	gap := waitForStateKind(t, handle.Lifecycle(), ibkr.SubscriptionGap)
	if gap.ConnectionSeq != 1 {
		t.Fatalf("gap.ConnectionSeq = %d, want 1", gap.ConnectionSeq)
	}
	resumed := waitForStateKind(t, handle.Lifecycle(), ibkr.SubscriptionResumed)
	if resumed.ConnectionSeq != 2 {
		t.Fatalf("resumed.ConnectionSeq = %d, want 2", resumed.ConnectionSeq)
	}

	if err := handle.Cancel(ctx); err != nil {
		t.Fatalf("Cancel after reconnect: %v", err)
	}
	statuses := waitOrderStatuses(t, ctx, handle)
	if !hasOrderStatus(statuses, ibkr.OrderStatusCancelled) {
		t.Fatalf("statuses = %v, want Cancelled after reconnect", statuses)
	}
	if err := handle.Wait(); err != nil {
		t.Fatalf("handle.Wait() error = %v", err)
	}
}

func TestAPIClientID0OrderObservationAAPLReplay(t *testing.T) {
	t.Parallel()

	host := newHost(t, "api_client_id0_order_observation_aapl.txt")
	defer waitHost(t, host)

	placer := dialHostClient(t, host, ibkr.WithClientID(1))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	handle, err := placer.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("10"),
			LmtPrice:  decimal.RequireFromString("13.27"),
			TIF:       ibkr.TIFGTC,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("placer Place: %v", err)
	}
	waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusSubmitted)
	if err := placer.Close(); err != nil {
		t.Fatalf("placer Close: %v", err)
	}

	observer := dialHostClient(t, host, ibkr.WithClientID(0))
	defer observer.Close()

	orders, err := observer.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
	if err != nil {
		t.Fatalf("client0 Open(all): %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("client0 open orders len = %d, want 1", len(orders))
	}
	if orders[0].OrderID != handle.OrderID() || orders[0].ClientID != 1 {
		t.Fatalf("client0 open order = %+v, want original client order %d", orders[0], handle.OrderID())
	}
	if err := observer.Orders().Cancel(ctx, handle.OrderID()); err != nil {
		t.Fatalf("client0 Cancel: %v", err)
	}
	waitHost(t, host)
}

func TestAPICrossClientCancelAAPLReplay(t *testing.T) {
	t.Parallel()

	host := newHost(t, "api_cross_client_cancel_aapl.txt")
	defer waitHost(t, host)

	placer := dialHostClient(t, host, ibkr.WithClientID(1))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	handle, err := placer.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("10"),
			LmtPrice:  decimal.RequireFromString("13.27"),
			TIF:       ibkr.TIFGTC,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("placer Place: %v", err)
	}
	waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusSubmitted)
	if err := placer.Close(); err != nil {
		t.Fatalf("placer Close: %v", err)
	}

	canceller := dialHostClient(t, host, ibkr.WithClientID(2))
	defer canceller.Close()

	orders, err := canceller.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
	if err != nil {
		t.Fatalf("client2 Open(all): %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("client2 open orders len = %d, want 1", len(orders))
	}
	if orders[0].OrderID != handle.OrderID() || orders[0].ClientID != 1 {
		t.Fatalf("client2 open order = %+v, want original client order %d", orders[0], handle.OrderID())
	}
	if err := canceller.Orders().Cancel(ctx, handle.OrderID()); err != nil {
		t.Fatalf("client2 Cancel: %v", err)
	}
	waitHost(t, host)
}

// TestSubscribeOpenDeliversCancelStatusForRecoveredOrder freezes the fix for
// https://github.com/ThomasMarcelis/ibkr-go/issues/20: an order recovered via
// SubscribeOpen has no OrderHandle in this process, so its status transitions
// (the paired snapshot status and the cancel confirmation) must be routed to
// the open-orders subscription instead of being dropped. Grounded by live
// capture captures/20260704T174748Z-api_reconnect_active_order_aapl,
// server_version 200, events.jsonl sha256 prefix 57943d05bd0242ca.
func TestSubscribeOpenDeliversCancelStatusForRecoveredOrder(t *testing.T) {
	t.Parallel()

	host := newHost(t, "api_reconnect_recovered_cancel_status_aapl.txt")
	defer waitHost(t, host)

	placer := dialHostClient(t, host, ibkr.WithClientID(1))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	handle, err := placer.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("100"),
			LmtPrice:  decimal.RequireFromString("15.42"),
			TIF:       ibkr.TIFGTC,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("placer Place: %v", err)
	}
	waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusPreSubmitted)
	if err := placer.Close(); err != nil {
		t.Fatalf("placer Close: %v", err)
	}

	observer := dialHostClient(t, host, ibkr.WithClientID(1))
	defer observer.Close()

	sub, err := observer.Orders().SubscribeOpen(ctx, ibkr.OpenOrdersScopeClient)
	if err != nil {
		t.Fatalf("SubscribeOpen(client): %v", err)
	}
	defer sub.Close()

	// Recovery snapshot: the Gateway pairs each open_order with an
	// order_status.
	var recovered ibkr.OpenOrderUpdate
	select {
	case recovered = <-sub.Events():
	case <-ctx.Done():
		t.Fatal("timed out waiting for recovered open-order event")
	}
	if recovered.Order == nil {
		t.Fatalf("recovery event = %+v, want Order payload", recovered)
	}
	if recovered.Order.OrderID != handle.OrderID() {
		t.Fatalf("recovered order id = %d, want %d", recovered.Order.OrderID, handle.OrderID())
	}
	select {
	case evt := <-sub.Events():
		if evt.Status == nil || evt.Status.Status != ibkr.OrderStatusPreSubmitted {
			t.Fatalf("paired snapshot event = %+v, want PreSubmitted Status payload", evt)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for paired snapshot status event")
	}
	waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionSnapshotComplete)

	if err := observer.Orders().Cancel(ctx, handle.OrderID()); err != nil {
		t.Fatalf("observer Cancel: %v", err)
	}

	select {
	case evt, ok := <-sub.Events():
		if !ok {
			t.Fatal("subscription closed without delivering the cancel status")
		}
		if evt.Status == nil {
			t.Fatalf("post-cancel event = %+v, want Status payload", evt)
		}
		if evt.Status.OrderID != handle.OrderID() {
			t.Fatalf("status order id = %d, want %d", evt.Status.OrderID, handle.OrderID())
		}
		if evt.Status.Status != ibkr.OrderStatusCancelled {
			t.Fatalf("status = %q, want %q", evt.Status.Status, ibkr.OrderStatusCancelled)
		}
	case <-ctx.Done():
		t.Fatal("cancel confirmed on the wire but no status event delivered to SubscribeOpen")
	}
}

// TestSubscribeOpenRefreshDeliversFreshSnapshot freezes the fix for
// https://github.com/ThomasMarcelis/ibkr-go/issues/21: RefreshOpen re-sends
// the open-orders request on the active subscription and the Gateway answers
// with a fresh burst terminated by another SnapshotComplete, so a consumer
// can resync without tearing the subscription down. Grounded by live capture
// captures/20260704T181808Z-api_open_orders_refresh_aapl, server_version 200,
// events.jsonl sha256 prefix 9d5a9af337237a67.
func TestSubscribeOpenRefreshDeliversFreshSnapshot(t *testing.T) {
	t.Parallel()

	host := newHost(t, "api_open_orders_refresh_aapl.txt")
	defer waitHost(t, host)

	client := dialHostClient(t, host, ibkr.WithClientID(0))
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Orders().RefreshOpen(ctx); !errors.Is(err, ibkr.ErrNoSubscription) {
		t.Fatalf("RefreshOpen without subscription error = %v, want ErrNoSubscription", err)
	}

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  decimal.RequireFromString("50"),
			TIF:       ibkr.TIFGTC,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("Place: %v", err)
	}
	waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusPreSubmitted)

	sub, err := client.Orders().SubscribeOpen(ctx, ibkr.OpenOrdersScopeAll)
	if err != nil {
		t.Fatalf("SubscribeOpen(all): %v", err)
	}
	defer sub.Close()

	// Consume until the initial SnapshotComplete, then refresh and consume
	// until the second one. The transcript carries four open_order +
	// order_status pairs in total: the initial snapshot, two unsolicited
	// echo pairs between the snapshots, and the refresh burst.
	orders, statuses, snapshots := 0, 0, 0
	for snapshots < 2 {
		select {
		case evt, ok := <-sub.Events():
			if !ok {
				t.Fatal("subscription closed while consuming snapshots")
			}
			if evt.Order != nil {
				orders++
			}
			if evt.Status != nil {
				statuses++
			}
		case state, ok := <-sub.Lifecycle():
			if !ok {
				t.Fatal("lifecycle closed while consuming snapshots")
			}
			if state.Kind == ibkr.SubscriptionSnapshotComplete {
				snapshots++
				if snapshots == 1 {
					if err := client.Orders().RefreshOpen(ctx); err != nil {
						t.Fatalf("RefreshOpen: %v", err)
					}
				}
			}
		case <-ctx.Done():
			t.Fatalf("timed out: snapshots=%d orders=%d statuses=%d", snapshots, orders, statuses)
		}
	}
	// Drain events emitted before the second SnapshotComplete that may still
	// be buffered behind the lifecycle read.
	for orders < 4 || statuses < 4 {
		select {
		case evt := <-sub.Events():
			if evt.Order != nil {
				orders++
			}
			if evt.Status != nil {
				statuses++
			}
		case <-ctx.Done():
			t.Fatalf("timed out draining: orders=%d statuses=%d, want 4/4", orders, statuses)
		}
	}

	if err := handle.Cancel(ctx); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusCancelled)
	waitHost(t, host)
}

// TestOpenOrdersAutoScopeHasNoSnapshot freezes the live-probed contract that
// req_auto_open_orders has no snapshot boundary (the Gateway sends no
// open_order_end for the bind, probed 2026-07-04, server_version 200), so
// one-shot and snapshot-wait APIs reject it while the persistent subscription
// remains available. The transcript expects only SubscribeOpen's request,
// proving Open rejects the unsupported scope before writing to the wire.
func TestOpenOrdersAutoScopeHasNoSnapshot(t *testing.T) {
	t.Parallel()

	host := newHost(t, "open_orders_auto_refresh.txt")
	defer waitHost(t, host)

	client := dialHostClient(t, host, ibkr.WithClientID(0))
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	orders, err := client.Orders().Open(ctx, ibkr.OpenOrdersScopeAuto)
	if !errors.Is(err, ibkr.ErrNoSnapshot) {
		t.Fatalf("Open(auto) error = %v, want ErrNoSnapshot", err)
	}
	if orders != nil {
		t.Fatalf("Open(auto) orders = %#v, want nil", orders)
	}

	sub, err := client.Orders().SubscribeOpen(ctx, ibkr.OpenOrdersScopeAuto)
	if err != nil {
		t.Fatalf("SubscribeOpen(auto): %v", err)
	}
	defer sub.Close()

	if err := sub.AwaitSnapshot(ctx); !errors.Is(err, ibkr.ErrNoSnapshot) {
		t.Fatalf("AwaitSnapshot(auto) error = %v, want ErrNoSnapshot", err)
	}

	if err := client.Orders().RefreshOpen(ctx); !errors.Is(err, ibkr.ErrNoSnapshot) {
		t.Fatalf("RefreshOpen(auto) error = %v, want ErrNoSnapshot", err)
	}
	waitHost(t, host)
}

// TestOpenOrdersSnapshotSkipsPairedStatuses freezes that the one-shot
// open-orders snapshot returns only the orders and filters the paired
// order_status frames the Gateway interleaves into the recovery snapshot.
// Same live grounding as TestSubscribeOpenDeliversCancelStatusForRecoveredOrder.
func TestOpenOrdersSnapshotSkipsPairedStatuses(t *testing.T) {
	t.Parallel()

	host := newHost(t, "api_reconnect_recovered_cancel_status_aapl.txt")
	defer waitHost(t, host)

	placer := dialHostClient(t, host, ibkr.WithClientID(1))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	handle, err := placer.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("100"),
			LmtPrice:  decimal.RequireFromString("15.42"),
			TIF:       ibkr.TIFGTC,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("placer Place: %v", err)
	}
	waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusPreSubmitted)
	if err := placer.Close(); err != nil {
		t.Fatalf("placer Close: %v", err)
	}

	observer := dialHostClient(t, host, ibkr.WithClientID(1))
	defer observer.Close()

	orders, err := observer.Orders().Open(ctx, ibkr.OpenOrdersScopeClient)
	if err != nil {
		t.Fatalf("Open(client): %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("open orders len = %d, want 1 (paired statuses must be filtered)", len(orders))
	}
	if orders[0].OrderID != handle.OrderID() {
		t.Fatalf("open order id = %d, want %d", orders[0].OrderID, handle.OrderID())
	}
	if err := observer.Orders().Cancel(ctx, handle.OrderID()); err != nil {
		t.Fatalf("observer Cancel: %v", err)
	}
	waitHost(t, host)
}

func TestAPITransmitFalseThenTransmitAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_transmit_false_then_transmit_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	order := ibkr.Order{
		Action:    ibkr.ActionBuy,
		OrderType: ibkr.OrderTypeLimit,
		Quantity:  decimal.RequireFromString("10"),
		LmtPrice:  decimal.RequireFromString("13.26"),
		TIF:       ibkr.TIFDay,
		Account:   "DU9000001",
		Transmit:  new(false),
	}
	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Order: order,
	})
	if err != nil {
		t.Fatalf("Transmit=false Place: %v", err)
	}

	order.Transmit = new(true)
	if err := handle.Modify(ctx, order); err != nil {
		t.Fatalf("Transmit=true Modify: %v", err)
	}
	waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusSubmitted)

	if err := handle.Cancel(ctx); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	statuses := waitOrderStatuses(t, ctx, handle)
	if !hasOrderStatus(statuses, ibkr.OrderStatusCancelled) {
		t.Fatalf("statuses = %v, want Cancelled from live capture", statuses)
	}
}

func waitForOrderStatus(t *testing.T, ctx context.Context, handle *ibkr.OrderHandle, want ibkr.OrderStatus) {
	t.Helper()
	waitOrderStatusUpdate(t, ctx, handle, want)
}

func waitForOpenOrder(t *testing.T, ctx context.Context, handle *ibkr.OrderHandle) ibkr.OpenOrder {
	t.Helper()

	for {
		select {
		case evt, ok := <-handle.Events():
			if !ok {
				t.Fatal("order events closed before OpenOrder")
			}
			if evt.OpenOrder != nil {
				return *evt.OpenOrder
			}
		case <-handle.Done():
			for {
				select {
				case evt, ok := <-handle.Events():
					if !ok {
						t.Fatal("order events closed before OpenOrder")
					}
					if evt.OpenOrder != nil {
						return *evt.OpenOrder
					}
				default:
					t.Fatal("order done before OpenOrder")
				}
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for OpenOrder")
		}
	}
}

func waitOrderFillAndExecution(t *testing.T, ctx context.Context, handle *ibkr.OrderHandle) (bool, bool) {
	t.Helper()

	var filled bool
	var execution bool
	for {
		select {
		case evt, ok := <-handle.Events():
			if !ok {
				return filled, execution
			}
			if evt.Execution != nil {
				execution = true
			}
			if evt.Status != nil && evt.Status.Status == ibkr.OrderStatusFilled {
				filled = true
			}
		case <-handle.Done():
			for {
				select {
				case evt, ok := <-handle.Events():
					if !ok {
						return filled, execution
					}
					if evt.Execution != nil {
						execution = true
					}
					if evt.Status != nil && evt.Status.Status == ibkr.OrderStatusFilled {
						filled = true
					}
				default:
					return filled, execution
				}
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for order fill")
		}
	}
}

func waitOrderStatuses(t *testing.T, ctx context.Context, handle *ibkr.OrderHandle) []ibkr.OrderStatus {
	t.Helper()

	var statuses []ibkr.OrderStatus
	for {
		select {
		case evt, ok := <-handle.Events():
			if !ok {
				return statuses
			}
			if evt.Status != nil {
				statuses = append(statuses, evt.Status.Status)
			}
		case <-handle.Done():
			for {
				select {
				case evt, ok := <-handle.Events():
					if !ok {
						return statuses
					}
					if evt.Status != nil {
						statuses = append(statuses, evt.Status.Status)
					}
				default:
					return statuses
				}
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for order terminal status")
		}
	}
}

func hasOrderStatus(statuses []ibkr.OrderStatus, want ibkr.OrderStatus) bool {
	for _, status := range statuses {
		if status == want {
			return true
		}
	}
	return false
}

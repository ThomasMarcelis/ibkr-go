package ibkr_test

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/ibkr"
	"github.com/ThomasMarcelis/ibkr-go/testing/testhost"
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

	details, err := client.ContractDetails(ctx, ibkr.ContractDetailsRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  "STK",
			Exchange: "SMART",
			Currency: "USD",
		},
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

	bars, err := client.HistoricalBars(ctx, ibkr.HistoricalBarsRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  "STK",
			Exchange: "SMART",
			Currency: "USD",
		},
		EndTime:    time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC),
		Duration:   24 * time.Hour,
		BarSize:    time.Hour,
		WhatToShow: "TRADES",
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

func TestAccountSummary(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "account_summary.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	values, err := client.AccountSummary(ctx, ibkr.AccountSummaryRequest{
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

	values, err := client.AccountSummary(ctx, ibkr.AccountSummaryRequest{
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

	values, err := client.AccountSummary(ctx, ibkr.AccountSummaryRequest{
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

	sub1, err := client.SubscribeAccountSummary(ctx, ibkr.AccountSummaryRequest{
		Account: "DU12345",
		Tags:    []string{"NetLiquidation"},
	})
	if err != nil {
		t.Fatalf("SubscribeAccountSummary() first error = %v", err)
	}

	sub2, err := client.SubscribeAccountSummary(ctx, ibkr.AccountSummaryRequest{
		Account: "DU12345",
		Tags:    []string{"BuyingPower"},
	})
	if err != nil {
		t.Fatalf("SubscribeAccountSummary() second error = %v", err)
	}

	sub3, err := client.SubscribeAccountSummary(ctx, ibkr.AccountSummaryRequest{
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

	sub, err := client.SubscribeAccountSummary(ctx, ibkr.AccountSummaryRequest{
		Account: "DU12345",
		Tags:    []string{"NetLiquidation"},
	})
	if err != nil {
		t.Fatalf("SubscribeAccountSummary() error = %v", err)
	}

	started := waitForStateKind(t, sub.State(), ibkr.SubscriptionStarted)
	if started.ConnectionSeq != 1 {
		t.Fatalf("started.ConnectionSeq = %d, want 1", started.ConnectionSeq)
	}

	first := waitForEvent(t, sub.Events())
	if first.Value.Value != "100000.00" {
		t.Fatalf("first value = %#v, want 100000.00", first)
	}

	snapshot := waitForStateKind(t, sub.State(), ibkr.SubscriptionSnapshotComplete)
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

	values, err := client.PositionsSnapshot(ctx)
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

	values, err := client.PositionsSnapshot(ctx)
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

	quote, err := client.QuoteSnapshot(ctx, ibkr.QuoteSubscriptionRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  "STK",
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

func TestQuoteSnapshotRejectsGenericTicks(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "handshake.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.QuoteSnapshot(ctx, ibkr.QuoteSubscriptionRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  "STK",
			Exchange: "SMART",
			Currency: "USD",
		},
		GenericTicks: []string{"233"},
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

	sub, err := client.SubscribeQuotes(ctx, ibkr.QuoteSubscriptionRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  "STK",
			Exchange: "SMART",
			Currency: "USD",
		},
	}, ibkr.WithResumePolicy(ibkr.ResumeAuto))
	if err != nil {
		t.Fatalf("SubscribeQuotes() error = %v", err)
	}

	started := waitForStateKind(t, sub.State(), ibkr.SubscriptionStarted)
	if started.ConnectionSeq != 1 {
		t.Fatalf("started.ConnectionSeq = %d, want 1", started.ConnectionSeq)
	}

	first := waitForEvent(t, sub.Events())
	if first.Snapshot.Bid.String() != "189.1" {
		t.Fatalf("first bid = %s, want 189.1", first.Snapshot.Bid.String())
	}

	gap := waitForStateKind(t, sub.State(), ibkr.SubscriptionGap)
	if gap.ConnectionSeq != 1 {
		t.Fatalf("gap.ConnectionSeq = %d, want 1", gap.ConnectionSeq)
	}

	resumed := waitForStateKind(t, sub.State(), ibkr.SubscriptionResumed)
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

	sub, err := client.SubscribeQuotes(ctx, ibkr.QuoteSubscriptionRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  "STK",
			Exchange: "SMART",
			Currency: "USD",
		},
	}, ibkr.WithResumePolicy(ibkr.ResumeAuto))
	if err != nil {
		t.Fatalf("SubscribeQuotes() error = %v", err)
	}

	started := waitForStateKind(t, sub.State(), ibkr.SubscriptionStarted)
	if started.ConnectionSeq != 1 {
		t.Fatalf("started.ConnectionSeq = %d, want 1", started.ConnectionSeq)
	}

	first := waitForEvent(t, sub.Events())
	if first.Snapshot.Bid.String() != "189.1" {
		t.Fatalf("first bid = %s, want 189.1", first.Snapshot.Bid.String())
	}

	gap := waitForStateKind(t, sub.State(), ibkr.SubscriptionGap)
	if gap.ConnectionSeq != 1 {
		t.Fatalf("gap.ConnectionSeq = %d, want 1", gap.ConnectionSeq)
	}

	resumed := waitForStateKind(t, sub.State(), ibkr.SubscriptionResumed)
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

	sub, err := client.SubscribeQuotes(ctx, ibkr.QuoteSubscriptionRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  "STK",
			Exchange: "SMART",
			Currency: "USD",
		},
	}, ibkr.WithResumePolicy(ibkr.ResumeAuto))
	if err != nil {
		t.Fatalf("SubscribeQuotes() error = %v", err)
	}

	started := waitForStateKind(t, sub.State(), ibkr.SubscriptionStarted)
	if started.ConnectionSeq != 1 {
		t.Fatalf("started.ConnectionSeq = %d, want 1", started.ConnectionSeq)
	}

	first := waitForEvent(t, sub.Events())
	if first.Snapshot.Bid.String() != "189.1" {
		t.Fatalf("first bid = %s, want 189.1", first.Snapshot.Bid.String())
	}

	gap := waitForStateKind(t, sub.State(), ibkr.SubscriptionGap)
	if gap.ConnectionSeq != 1 {
		t.Fatalf("gap.ConnectionSeq = %d, want 1", gap.ConnectionSeq)
	}

	resumed := waitForStateKind(t, sub.State(), ibkr.SubscriptionResumed)
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

	values, err := client.OpenOrdersSnapshot(ctx, ibkr.OpenOrdersScopeAll)
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

	values, err := client.OpenOrdersSnapshot(ctx, ibkr.OpenOrdersScopeAll)
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

	sub, err := client.SubscribeOpenOrders(ctx, ibkr.OpenOrdersScopeAuto)
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
				sub, err := client.SubscribeAccountSummary(ctx, ibkr.AccountSummaryRequest{
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
				sub, err := client.SubscribePositions(ctx, ibkr.WithResumePolicy(ibkr.ResumeAuto))
				if sub != nil {
					_ = sub.Close()
				}
				return err
			},
		},
		{
			name: "open_orders",
			subscribe: func(ctx context.Context, client *ibkr.Client) error {
				sub, err := client.SubscribeOpenOrders(ctx, ibkr.OpenOrdersScopeAll, ibkr.WithResumePolicy(ibkr.ResumeAuto))
				if sub != nil {
					_ = sub.Close()
				}
				return err
			},
		},
		{
			name: "executions",
			subscribe: func(ctx context.Context, client *ibkr.Client) error {
				sub, err := client.SubscribeExecutions(ctx, ibkr.ExecutionsRequest{
					Account: "DU12345",
					Symbol:  "AAPL",
				}, ibkr.WithResumePolicy(ibkr.ResumeAuto))
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

	updates, err := client.Executions(ctx, ibkr.ExecutionsRequest{
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

func TestSubscribeExecutionsClosesAfterSnapshotEnd(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "executions.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.SubscribeExecutions(ctx, ibkr.ExecutionsRequest{
		Account: "DU12345",
		Symbol:  "AAPL",
	})
	if err != nil {
		t.Fatalf("SubscribeExecutions() error = %v", err)
	}

	started := waitForStateKind(t, sub.State(), ibkr.SubscriptionStarted)
	if started.ConnectionSeq != 1 {
		t.Fatalf("started.ConnectionSeq = %d, want 1", started.ConnectionSeq)
	}

	exec := waitForEvent(t, sub.Events())
	if exec.Execution == nil || exec.Execution.ExecID != "exec-1" {
		t.Fatalf("execution = %#v, want exec-1", exec)
	}

	commission := waitForEvent(t, sub.Events())
	if commission.Commission == nil || commission.Commission.ExecID != "exec-1" {
		t.Fatalf("commission = %#v, want exec-1", commission)
	}

	snapshot := waitForStateKind(t, sub.State(), ibkr.SubscriptionSnapshotComplete)
	if snapshot.ConnectionSeq != 1 {
		t.Fatalf("snapshot.ConnectionSeq = %d, want 1", snapshot.ConnectionSeq)
	}

	closed := waitForStateKind(t, sub.State(), ibkr.SubscriptionClosed)
	if closed.Err != nil {
		t.Fatalf("closed.Err = %v, want nil", closed.Err)
	}

	if err := sub.Wait(); err != nil {
		t.Fatalf("sub.Wait() error = %v", err)
	}
}

func TestExecutionsCorrelateCommissionByExecID(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "executions_correlated.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	aapl, err := client.SubscribeExecutions(ctx, ibkr.ExecutionsRequest{
		Account: "DU12345",
		Symbol:  "AAPL",
	})
	if err != nil {
		t.Fatalf("SubscribeExecutions() AAPL error = %v", err)
	}
	defer aapl.Close()

	msft, err := client.SubscribeExecutions(ctx, ibkr.ExecutionsRequest{
		Account: "DU12345",
		Symbol:  "MSFT",
	})
	if err != nil {
		t.Fatalf("SubscribeExecutions() MSFT error = %v", err)
	}
	defer msft.Close()

	aaplExec := waitForEvent(t, aapl.Events())
	aaplCommission := waitForEvent(t, aapl.Events())
	msftExec := waitForEvent(t, msft.Events())
	msftCommission := waitForEvent(t, msft.Events())

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

	accountWide, err := client.SubscribeExecutions(ctx, ibkr.ExecutionsRequest{
		Account: "DU12345",
	})
	if err != nil {
		t.Fatalf("SubscribeExecutions() account-wide error = %v", err)
	}
	defer accountWide.Close()

	aaplOnly, err := client.SubscribeExecutions(ctx, ibkr.ExecutionsRequest{
		Account: "DU12345",
		Symbol:  "AAPL",
	})
	if err != nil {
		t.Fatalf("SubscribeExecutions() AAPL-only error = %v", err)
	}
	defer aaplOnly.Close()

	wideExec := waitForEvent(t, accountWide.Events())
	wideCommission := waitForEvent(t, accountWide.Events())
	narrowExec := waitForEvent(t, aaplOnly.Events())
	narrowCommission := waitForEvent(t, aaplOnly.Events())

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

	sub, err := client.SubscribeRealTimeBars(ctx, ibkr.RealTimeBarsRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  "STK",
			Exchange: "SMART",
			Currency: "USD",
		},
		WhatToShow: "TRADES",
		UseRTH:     true,
	}, ibkr.WithResumePolicy(ibkr.ResumeAuto))
	if err != nil {
		t.Fatalf("SubscribeRealTimeBars() error = %v", err)
	}

	started := waitForStateKind(t, sub.State(), ibkr.SubscriptionStarted)
	if started.ConnectionSeq != 1 {
		t.Fatalf("started.ConnectionSeq = %d, want 1", started.ConnectionSeq)
	}

	first := waitForEvent(t, sub.Events())
	if first.Close.String() != "189.1" {
		t.Fatalf("first close = %s, want 189.1", first.Close.String())
	}

	gap := waitForStateKind(t, sub.State(), ibkr.SubscriptionGap)
	if gap.ConnectionSeq != 1 {
		t.Fatalf("gap.ConnectionSeq = %d, want 1", gap.ConnectionSeq)
	}

	resumed := waitForStateKind(t, sub.State(), ibkr.SubscriptionResumed)
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

	sub, err := client.SubscribeQuotes(ctx, ibkr.QuoteSubscriptionRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  "STK",
			Exchange: "SMART",
			Currency: "USD",
		},
	})
	if err != nil {
		t.Fatalf("SubscribeQuotes() error = %v", err)
	}

	started := waitForStateKind(t, sub.State(), ibkr.SubscriptionStarted)
	if started.ConnectionSeq != 1 {
		t.Fatalf("started.ConnectionSeq = %d, want 1", started.ConnectionSeq)
	}

	if err := sub.Wait(); !errors.Is(err, ibkr.ErrResumeRequired) {
		t.Fatalf("sub.Wait() error = %v, want %v", err, ibkr.ErrResumeRequired)
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

	path := filepath.Join("..", "testdata", "transcripts", script)
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

	quote, err := client.QuoteSnapshot(ctx, ibkr.QuoteSubscriptionRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  "STK",
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
	if len(snapshot.ManagedAccounts) != 1 || snapshot.ManagedAccounts[0] != "U10597365" {
		t.Fatalf("managed accounts = %v, want [U10597365]", snapshot.ManagedAccounts)
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

	details, err := client.ContractDetails(ctx, ibkr.ContractDetailsRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  "STK",
			Exchange: "SMART",
			Currency: "USD",
		},
	})
	if err != nil {
		t.Fatalf("ContractDetails() error = %v", err)
	}
	if len(details) != 1 {
		t.Fatalf("details len = %d, want 1", len(details))
	}
	d := details[0]
	if d.Contract.Symbol != "AAPL" {
		t.Errorf("Symbol = %q, want AAPL", d.Contract.Symbol)
	}
	if d.Contract.SecType != "STK" {
		t.Errorf("SecType = %q, want STK", d.Contract.SecType)
	}
	if d.Contract.Exchange != "SMART" {
		t.Errorf("Exchange = %q, want SMART", d.Contract.Exchange)
	}
	if d.Contract.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", d.Contract.Currency)
	}
	if d.Contract.PrimaryExchange != "NASDAQ" {
		t.Errorf("PrimaryExchange = %q, want NASDAQ", d.Contract.PrimaryExchange)
	}
	if d.MarketName != "NMS" {
		t.Errorf("MarketName = %q, want NMS", d.MarketName)
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

	values, err := client.AccountSummary(ctx, ibkr.AccountSummaryRequest{
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

	if v, ok := byTag["NetLiquidation"]; !ok || v.Value != "58846.35" {
		t.Errorf("NetLiquidation = %q, want 58846.35", v.Value)
	}
	if v, ok := byTag["TotalCashValue"]; !ok || v.Value != "10103.19" {
		t.Errorf("TotalCashValue = %q, want 10103.19", v.Value)
	}
	if v, ok := byTag["BuyingPower"]; !ok || v.Value != "266701.40" {
		t.Errorf("BuyingPower = %q, want 266701.40", v.Value)
	}
	if v, ok := byTag["ExcessLiquidity"]; !ok || v.Value != "42071.58" {
		t.Errorf("ExcessLiquidity = %q, want 42071.58", v.Value)
	}

	for _, v := range values {
		if v.Account != "U10597365" {
			t.Errorf("tag %s: account = %q, want U10597365", v.Tag, v.Account)
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

	bars, err := client.HistoricalBars(ctx, ibkr.HistoricalBarsRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  "STK",
			Exchange: "SMART",
			Currency: "USD",
		},
		Duration:   24 * time.Hour,
		BarSize:    time.Hour,
		WhatToShow: "TRADES",
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

	positions, err := client.PositionsSnapshot(ctx)
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
	if amzn.Account != "U10597365" {
		t.Errorf("AMZN account = %q, want U10597365", amzn.Account)
	}
	if amzn.Position.String() != "15" {
		t.Errorf("AMZN position = %s, want 15", amzn.Position.String())
	}
	if amzn.AvgCost.String() != "199.66666665" {
		t.Errorf("AMZN avgCost = %s, want 199.66666665", amzn.AvgCost.String())
	}
	if amzn.Contract.SecType != "STK" {
		t.Errorf("AMZN secType = %q, want STK", amzn.Contract.SecType)
	}
	if amzn.Contract.Currency != "USD" {
		t.Errorf("AMZN currency = %q, want USD", amzn.Contract.Currency)
	}

	if aapl == nil {
		t.Fatal("AAPL position not found")
	}
	if aapl.Position.String() != "10" {
		t.Errorf("AAPL position = %s, want 10", aapl.Position.String())
	}
	if aapl.AvgCost.String() != "255.244" {
		t.Errorf("AAPL avgCost = %s, want 255.244", aapl.AvgCost.String())
	}

	if yw == nil {
		t.Fatal("YW position not found")
	}
	if yw.Contract.SecType != "FUT" {
		t.Errorf("YW secType = %q, want FUT", yw.Contract.SecType)
	}
	if yw.Position.String() != "1" {
		t.Errorf("YW position = %s, want 1", yw.Position.String())
	}

	if qqq == nil {
		t.Fatal("QQQ position not found")
	}
	if qqq.Contract.SecType != "OPT" {
		t.Errorf("QQQ secType = %q, want OPT", qqq.Contract.SecType)
	}
	if qqq.Position.String() != "-3" {
		t.Errorf("QQQ position = %s, want -3", qqq.Position.String())
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

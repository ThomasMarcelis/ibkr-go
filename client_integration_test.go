package ibkr_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/testhost"
	"github.com/shopspring/decimal"
)

func TestDialContextWithClientID(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "handshake_client_id_0.txt", ibkr.WithClientID(0))
	defer cleanupClientHost(t, client, host)

	if got := client.Session().NextValidID; got != 1 {
		t.Fatalf("next valid id = %d, want 1", got)
	}
}

func TestHistoricalSchedule(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "historical_schedule_aapl.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	schedule, err := client.History().Schedule(ctx, ibkr.HistoricalScheduleRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
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
	if schedule.StartDateTime != "20260727-09:30:00" {
		t.Errorf("StartDateTime = %q, want 20260727-09:30:00", schedule.StartDateTime)
	}
	if schedule.EndDateTime != "20260824-16:00:00" {
		t.Errorf("EndDateTime = %q, want 20260824-16:00:00", schedule.EndDateTime)
	}
	if len(schedule.Sessions) != 21 {
		t.Fatalf("Sessions = %d, want 21", len(schedule.Sessions))
	}
	first := schedule.Sessions[0]
	if first.RefDate != "20260727" || first.StartDateTime != "20260727-09:30:00" {
		t.Errorf("first session = %+v, want 20260727-09:30:00 / 20260727", first)
	}
	last := schedule.Sessions[20]
	if last.RefDate != "20260824" || last.EndDateTime != "20260824-16:00:00" {
		t.Errorf("last session = %+v, want 20260824-16:00:00 / 20260824", last)
	}
}

func TestHistoricalBarsWithScheduleRejects(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "grounded_bootstrap.txt")
	defer cleanupClientHost(t, client, host)

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
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	values, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		AccountFilter: "DU9000001",
		Tags:          []string{"NetLiquidation", "TotalCashValue", "BuyingPower", "ExcessLiquidity"},
	})
	if err != nil {
		t.Fatalf("AccountSummary() error = %v", err)
	}
	if len(values) != 4 {
		t.Fatalf("values len = %d, want 4", len(values))
	}
	for _, value := range values {
		if value.Account != "DU9000001" {
			t.Fatalf("account = %q, want DU9000001", value.Account)
		}
	}
	if values[0].Tag != "BuyingPower" || values[0].Value != "183875.22" {
		t.Fatalf("first value = %+v, want BuyingPower 183875.22", values[0])
	}
}

func TestAccountSummaryAllReturnsAllAccounts(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "account_summary.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	values, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Group: "All",
		Tags:  []string{"NetLiquidation", "TotalCashValue", "BuyingPower", "ExcessLiquidity"},
	})
	if err != nil {
		t.Fatalf("AccountSummary() error = %v", err)
	}
	if len(values) != 4 {
		t.Fatalf("values len = %d, want 4", len(values))
	}
}

func TestAccountSummarySucceedsWhenDisconnectFollowsSnapshotEnd(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "account_summary_disconnect_after_end.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	values, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		AccountFilter: "DU9000001",
		Tags:          []string{"NetLiquidation", "TotalCashValue", "BuyingPower", "ExcessLiquidity"},
	})
	if err != nil {
		t.Fatalf("AccountSummary() error = %v", err)
	}
	if len(values) != 4 {
		t.Fatalf("values len = %d, want 4", len(values))
	}
	if values[2].Tag != "NetLiquidation" || values[2].Value != "36992.49" {
		t.Fatalf("net liquidation = %+v, want 36992.49", values[2])
	}
}

func TestSubscribeAccountSummarySnapshotCompleteDoesNotCloseStream(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "account_summary_stream.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.Accounts().SubscribeSummary(ctx, ibkr.AccountSummaryRequest{
		AccountFilter: "DU9000001",
		Tags:          []string{"NetLiquidation", "TotalCashValue", "BuyingPower"},
	})
	if err != nil {
		t.Fatalf("SubscribeAccountSummary() error = %v", err)
	}

	started := waitForStateKind(t, sub.Events(), ibkr.StreamStarted)
	if started.ConnectionSeq != 1 {
		t.Fatalf("started.ConnectionSeq = %d, want 1", started.ConnectionSeq)
	}

	want := []struct {
		tag   string
		value string
	}{
		{tag: "BuyingPower", value: "183875.22"},
		{tag: "NetLiquidation", value: "36992.49"},
		{tag: "TotalCashValue", value: "-1441.67"},
	}
	for _, expected := range want {
		got := waitForStreamData(t, sub.Events())
		if got.Tag != expected.tag || got.Value != expected.value {
			t.Fatalf("value = %+v, want %s %s", got, expected.tag, expected.value)
		}
	}

	snapshot := waitForStateKind(t, sub.Events(), ibkr.StreamSnapshotComplete)
	if snapshot.ConnectionSeq != 1 {
		t.Fatalf("snapshot.ConnectionSeq = %d, want 1", snapshot.ConnectionSeq)
	}

	select {
	case <-sub.Done():
		t.Fatal("snapshot end closed streaming account summary")
	default:
	}

	sub.Close()
	if err := sub.Wait(); err != nil {
		t.Fatalf("sub.Wait() error = %v", err)
	}
}

func TestPositionsSnapshotSucceedsWhenDisconnectFollowsSnapshotEnd(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "positions_disconnect_after_end.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	values, err := client.Accounts().Positions(ctx)
	if err != nil {
		t.Fatalf("PositionsSnapshot() error = %v", err)
	}
	if len(values) != 8 {
		t.Fatalf("positions len = %d, want 8", len(values))
	}
	foundZeroMES := false
	for _, value := range values {
		foundZeroMES = foundZeroMES || value.Contract.LocalSymbol == "MESU6" && value.Position.IsZero()
	}
	if !foundZeroMES {
		t.Fatal("positions snapshot lacks the live-attested zero-quantity MESU6 row")
	}
}

func TestQuoteSnapshot(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "quote_snapshot.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("SetType() error = %v", err)
	}

	quote, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
	})
	if err != nil {
		t.Fatalf("QuoteSnapshot() error = %v", err)
	}
	if quote.Bid.String() != "310.45" {
		t.Fatalf("bid = %s, want 310.45", quote.Bid.String())
	}
	if quote.Ask.String() != "310.47" {
		t.Fatalf("ask = %s, want 310.47", quote.Ask.String())
	}
	if quote.MarketDataType != ibkr.MarketDataDelayed {
		t.Fatalf("market data type = %s, want delayed", quote.MarketDataType)
	}
}

func TestAPIDuplicateQuoteSubscriptionsAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_duplicate_quote_subscriptions_aapl.txt")
	defer cleanupClientHost(t, client, host)

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
		first.Close()
		t.Fatalf("second SubscribeQuotes() error = %v", err)
	}

	waitDelayedBidAsk := func(label string, events <-chan ibkr.StreamEvent[ibkr.QuoteUpdate]) ibkr.Quote {
		t.Helper()

		wantFields := ibkr.QuoteFieldBid | ibkr.QuoteFieldAsk | ibkr.QuoteFieldMarketDataType
		for {
			select {
			case event, ok := <-events:
				if !ok {
					t.Fatalf("%s quote events closed before delayed bid/ask", label)
				}
				if event.Kind != ibkr.StreamData {
					continue
				}
				update := event.Value
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
		if got := quote.Bid.String(); got != "310.4" {
			t.Fatalf("%s bid = %s, want 310.4", label, got)
		}
		if got := quote.Ask.String(); got != "310.55" {
			t.Fatalf("%s ask = %s, want 310.55", label, got)
		}
	}

	first.Close()
	second.Close()
	if err := first.Wait(); err != nil {
		t.Fatalf("first Wait() error = %v", err)
	}
	if err := second.Wait(); err != nil {
		t.Fatalf("second Wait() error = %v", err)
	}
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime() fence error = %v", err)
	}
}

func TestSetMarketDataType(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "grounded_bootstrap.txt")
	defer cleanupClientHost(t, client, host)
	ctx := context.Background()

	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("MarketData().SetType(MarketDataDelayed) error = %v", err)
	}

	for _, dataType := range []ibkr.MarketDataType{0, 5} {
		err := client.MarketData().SetType(ctx, dataType)
		validationErr, ok := errors.AsType[*ibkr.ValidationError](err)
		if !ok || validationErr.Field != "MarketDataType" {
			t.Fatalf("MarketData().SetType(%d) error = %v, want MarketDataType validation", dataType, err)
		}
	}
}

func TestQuoteSnapshotRejectsGenericTicks(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "grounded_bootstrap.txt")
	defer cleanupClientHost(t, client, host)

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
	validationErr, ok := errors.AsType[*ibkr.ValidationError](err)
	if !ok || validationErr.Field != "QuoteRequest.GenericTicks" {
		t.Fatalf("QuoteSnapshot() error = %v, want QuoteRequest.GenericTicks validation", err)
	}
}

func TestSubscribeQuotesResumeAutoReconnectsAfterTransportLoss(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "quote_stream_reconnect.txt", ibkr.WithReconnectPolicy(ibkr.ReconnectAuto))
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("MarketData().SetType(delayed) error = %v", err)
	}

	sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
	}, ibkr.WithResumePolicy(ibkr.ResumeAuto))
	if err != nil {
		t.Fatalf("SubscribeQuotes() error = %v", err)
	}

	started := waitForStateKind(t, sub.Events(), ibkr.StreamStarted)
	if started.ConnectionSeq != 1 {
		t.Fatalf("started.ConnectionSeq = %d, want 1", started.ConnectionSeq)
	}

	first := waitForStreamData(t, sub.Events())
	for first.Changed&ibkr.QuoteFieldHigh == 0 {
		first = waitForStreamData(t, sub.Events())
	}
	if first.Snapshot.High.String() != "313.36" {
		t.Fatalf("first high = %s, want 313.36", first.Snapshot.High.String())
	}

	gap := waitForStateKind(t, sub.Events(), ibkr.StreamGap)
	if gap.ConnectionSeq != 1 {
		t.Fatalf("gap.ConnectionSeq = %d, want 1", gap.ConnectionSeq)
	}
	if gap.Err == nil {
		t.Fatal("gap.Err = nil, want transport cause")
	}

	resumed := waitForStateKind(t, sub.Events(), ibkr.StreamResubscribed)
	if resumed.ConnectionSeq != 2 {
		t.Fatalf("resumed.ConnectionSeq = %d, want 2", resumed.ConnectionSeq)
	}

	second := waitForStreamData(t, sub.Events())
	for second.Changed&ibkr.QuoteFieldLow == 0 {
		second = waitForStreamData(t, sub.Events())
	}
	if second.Snapshot.Low.String() != "309.97" {
		t.Fatalf("second low = %s, want 309.97", second.Snapshot.Low.String())
	}
	if second.Snapshot.Available&ibkr.QuoteFieldHigh != 0 {
		t.Fatalf("second quote retained pre-gap high: %+v", second.Snapshot)
	}

	sub.Close()
	if err := sub.Wait(); err != nil {
		t.Fatalf("sub.Wait() error = %v", err)
	}
	if got := client.Session().ConnectionSeq; got != 2 {
		t.Fatalf("client.Session().ConnectionSeq = %d, want 2", got)
	}
	if got := client.Session().ManagedAccounts; len(got) != 1 || got[0] != "DU9000001" {
		t.Fatalf("client.Session().ManagedAccounts = %v, want [DU9000001]", got)
	}
}

func TestOpenOrdersAutoRequiresClientIDZero(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "grounded_bootstrap.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.Orders().SubscribeOpen(ctx, ibkr.OpenOrdersScopeAuto)
	if err == nil {
		if sub != nil {
			sub.Close()
		}
		t.Fatal("SubscribeOpenOrders() error = nil, want client-id validation")
	}
	validationErr, ok := errors.AsType[*ibkr.ValidationError](err)
	if !ok || validationErr.Field != "OpenOrdersScope" {
		t.Fatalf("SubscribeOpenOrders() error = %v, want OpenOrdersScope validation", err)
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
					AccountFilter: "DU12345",
					Tags:          []string{"NetLiquidation"},
				}, ibkr.WithResumePolicy(ibkr.ResumeAuto))
				if sub != nil {
					sub.Close()
				}
				return err
			},
		},
		{
			name: "positions",
			subscribe: func(ctx context.Context, client *ibkr.Client) error {
				sub, err := client.Accounts().SubscribePositions(ctx, ibkr.WithResumePolicy(ibkr.ResumeAuto))
				if sub != nil {
					sub.Close()
				}
				return err
			},
		},
		{
			name: "open_orders",
			subscribe: func(ctx context.Context, client *ibkr.Client) error {
				sub, err := client.Orders().SubscribeOpen(ctx, ibkr.OpenOrdersScopeAll, ibkr.WithResumePolicy(ibkr.ResumeAuto))
				if sub != nil {
					sub.Close()
				}
				return err
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, host := newClient(t, "grounded_bootstrap.txt")
			defer cleanupClientHost(t, client, host)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := tc.subscribe(ctx, client)
			validationErr, ok := errors.AsType[*ibkr.ValidationError](err)
			if !ok || validationErr.Field != "ResumePolicy" {
				t.Fatalf("%s resume-auto error = %v, want ResumePolicy validation", tc.name, err)
			}
		})
	}
}

// TestExecutionsBurstExceedsSubscriptionBuffer freezes a finite live execution
// query whose response rows exceed the configured consumer buffer.
func TestExecutionsBurstExceedsSubscriptionBuffer(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "executions.txt", ibkr.WithSubscriptionBuffer(1))
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	executions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{})
	if err != nil {
		t.Fatalf("Executions: %v", err)
	}
	if len(executions.Executions) != 2 {
		t.Fatalf("executions len = %d, want 2", len(executions.Executions))
	}
	if len(executions.CommissionAndFees) != 2 {
		t.Fatalf("commission-and-fees len = %d, want 2", len(executions.CommissionAndFees))
	}

	executionIDs := make([]string, len(executions.Executions))
	for i, execution := range executions.Executions {
		executionIDs[i] = execution.ExecID
	}
	wantExecutionIDs := []string{"sanitized-exec-00000001", "sanitized-exec-00000002"}
	if !reflect.DeepEqual(executionIDs, wantExecutionIDs) {
		t.Fatalf("execution IDs = %v, want %v", executionIDs, wantExecutionIDs)
	}
	first := executions.Executions[0]
	if first.Price.String() != "7670.75" || first.Shares.String() != "1" {
		t.Fatalf("first execution price/shares = %s/%s, want captured MES fill", first.Price, first.Shares)
	}
	wantTime := time.Date(2026, 8, 24, 20, 50, 7, 0, time.UTC)
	if !first.Time.Equal(wantTime) {
		t.Fatalf("first execution time = %s, want %s", first.Time, wantTime)
	}
}

func TestExecutionsMissingEndReturnsContextDeadline(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "executions_missing_end_live.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Executions() error = %v, want context deadline exceeded", err)
	}
}

// TestExecutionsConcurrentQueries freezes request-ID routing across three
// live-concurrent queries: the BUY and SELL partitions must remain disjoint,
// while the overlapping all-side result contains both partitions.
func TestExecutionsConcurrentQueries(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "executions_concurrent_aapl.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type result struct {
		label      string
		executions []ibkr.Execution
		err        error
	}
	results := make(chan result, 3)
	start := func(label string, req ibkr.ExecutionsRequest) {
		sub, err := client.Orders().SubscribeExecutions(ctx, req)
		if err != nil {
			t.Fatalf("%s SubscribeExecutions() error = %v", label, err)
		}
		go func() {
			var executions []ibkr.Execution
			for {
				select {
				case event, ok := <-sub.Events():
					if !ok {
						results <- result{label: label, err: sub.Wait()}
						return
					}
					if event.Kind == ibkr.StreamData && event.Value.Execution != nil {
						executions = append(executions, *event.Value.Execution)
					}
					if event.Kind == ibkr.StreamSnapshotComplete {
						sub.Close()
						results <- result{label: label, executions: executions, err: sub.Wait()}
						return
					}
				case <-ctx.Done():
					sub.Close()
					results <- result{label: label, err: context.Cause(ctx)}
					return
				}
			}
		}()
	}
	start("all", ibkr.ExecutionsRequest{
		Account: "DU9000001",
		Symbol:  "AAPL",
	})
	start("buy", ibkr.ExecutionsRequest{
		Account: "DU9000001",
		Symbol:  "AAPL",
		Side:    ibkr.ExecutionFilterBuy,
	})
	start("sell", ibkr.ExecutionsRequest{
		Account: "DU9000001",
		Symbol:  "AAPL",
		Side:    ibkr.ExecutionFilterSell,
	})

	byLabel := make(map[string][]ibkr.Execution, 3)
	for range 3 {
		select {
		case result := <-results:
			if result.err != nil {
				t.Fatalf("%s Executions() error = %v", result.label, result.err)
			}
			byLabel[result.label] = result.executions
		case <-ctx.Done():
			t.Fatal(context.Cause(ctx))
		}
	}

	allIDs := make(map[string]ibkr.ExecutionSide, len(byLabel["all"]))
	for _, execution := range byLabel["all"] {
		allIDs[execution.ExecID] = execution.Side
	}
	for _, filtered := range []struct {
		label string
		side  ibkr.ExecutionSide
	}{{label: "buy", side: ibkr.ExecutionSideBought}, {label: "sell", side: ibkr.ExecutionSideSold}} {
		executions := byLabel[filtered.label]
		if len(executions) == 0 {
			t.Fatalf("%s executions are empty", filtered.label)
		}
		for _, execution := range executions {
			if execution.Side != filtered.side {
				t.Fatalf("%s execution %q side = %s, want %s", filtered.label, execution.ExecID, execution.Side, filtered.side)
			}
			if allIDs[execution.ExecID] != execution.Side {
				t.Fatalf("%s execution %q is absent from the overlapping all-side result", filtered.label, execution.ExecID)
			}
		}
	}

	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime cleanup fence: %v", err)
	}
}

func TestSubscribeQuotesResumeNeverRequiresManualResumeOnDisconnect(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "quote_stream_disconnect.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("SetType() error = %v", err)
	}

	sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
	})
	if err != nil {
		t.Fatalf("SubscribeQuotes() error = %v", err)
	}

	started := waitForStateKind(t, sub.Events(), ibkr.StreamStarted)
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
	defer cleanupClientHost(t, client, host)

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
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exchanges, err := client.Contracts().DepthExchanges(ctx)
	if err != nil {
		t.Fatalf("MktDepthExchanges() error = %v", err)
	}
	if len(exchanges) != 306 {
		t.Fatalf("exchanges len = %d, want 306", len(exchanges))
	}
	wantExchanges := []ibkr.DepthExchange{
		{Exchange: "IDEALPRO", SecType: ibkr.SecTypeForex, ServiceDataType: "Deep", AggGroup: 4},
		{Exchange: "SMART", SecType: ibkr.SecTypeStock, ListingExch: "PINK", ServiceDataType: "AggDeep", AggGroup: 1},
		{Exchange: "SMART", SecType: ibkr.SecTypeBond, ServiceDataType: "Deep", AggGroup: 7},
	}
	for _, want := range wantExchanges {
		found := false
		for _, got := range exchanges {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("exchanges missing %+v", want)
		}
	}
}

func TestNewsProviders(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "news_providers.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	providers, err := client.News().Providers(ctx)
	if err != nil {
		t.Fatalf("NewsProviders() error = %v", err)
	}
	if len(providers) != 8 {
		t.Fatalf("providers len = %d, want 8", len(providers))
	}
	wantProviders := []ibkr.NewsProvider{
		{Code: "BRFG", Name: "Briefing.com General Market Columns"},
		{Code: "BRFUPDN", Name: "Briefing.com Analyst Actions"},
		{Code: "DJ-N", Name: "Dow Jones Global Equity Trader"},
		{Code: "DJ-RT", Name: "Dow Jones Trader News"},
		{Code: "DJ-RTA", Name: "Dow Jones Top Stories Asia Pacific"},
		{Code: "DJ-RTE", Name: "Dow Jones Top Stories Europe"},
		{Code: "DJ-RTG", Name: "Dow Jones Top Stories Global"},
		{Code: "DJNL", Name: "Dow Jones Newsletters"},
	}
	for i, want := range wantProviders {
		if providers[i] != want {
			t.Errorf("providers[%d] = %+v, want %+v", i, providers[i], want)
		}
	}
}

func TestUserInfo(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "user_info.txt")
	defer cleanupClientHost(t, client, host)

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
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	symbols, err := client.Contracts().Search(ctx, "AAPL")
	if err != nil {
		t.Fatalf("MatchingSymbols() error = %v", err)
	}
	if len(symbols) != 20 {
		t.Fatalf("symbols len = %d, want 20", len(symbols))
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
	if want := []string{"CFD", "OPT", "IOPT", "WAR", "BAG"}; !reflect.DeepEqual(symbols[0].DerivativeSecTypes, want) {
		t.Fatalf("derivative_sec_types = %v, want %v", symbols[0].DerivativeSecTypes, want)
	}
	if symbols[0].Description != "APPLE INC" {
		t.Fatalf("description = %q, want APPLE INC", symbols[0].Description)
	}
}

func TestHeadTimestamp(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "head_timestamp.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ts, err := client.History().HeadTimestamp(ctx, ibkr.HeadTimestampRequest{
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
		t.Fatalf("HeadTimestamp() error = %v", err)
	}
	if !ts.Equal(time.Date(1980, 12, 12, 14, 30, 0, 0, time.UTC)) {
		t.Fatalf("timestamp = %s, want 1980-12-12 14:30:00 UTC", ts.Format(time.RFC3339))
	}
}

func TestMarketRule(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "market_rule.txt")
	defer cleanupClientHost(t, client, host)

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

func TestSecDefOptParams(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "sec_def_opt_params.txt")
	defer cleanupClientHost(t, client, host)

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
	if len(params) != 39 {
		t.Fatalf("params len = %d, want 39", len(params))
	}
	var smart, cboe bool
	for _, param := range params {
		if param.TradingClass == "" || param.Multiplier != "100" || len(param.Expirations) == 0 || len(param.Strikes) == 0 {
			t.Fatalf("incomplete option parameters = %+v", param)
		}
		switch param.Exchange {
		case "SMART":
			smart = true
		case "CBOE":
			cboe = true
		}
	}
	if !smart || !cboe {
		t.Fatalf("option exchanges include SMART=%t CBOE=%t, want both", smart, cboe)
	}
}

func TestSmartComponents(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "smart_components.txt")
	defer cleanupClientHost(t, client, host)

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
		case event, ok := <-sub.Events():
			if !ok {
				t.Fatalf("quote closed before parameters: %v", sub.Err())
			}
			if event.Kind != ibkr.StreamData {
				continue
			}
			update := event.Value
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
	sub.Close()
	if err := sub.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
}

func TestNewsArticle(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "news_article.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	items, err := client.News().Historical(ctx, ibkr.HistoricalNewsRequest{
		ConID:         265598,
		ProviderCodes: []ibkr.NewsProviderCode{"BRFG", "BRFUPDN", "DJNL"},
		TotalResults:  5,
	})
	if err != nil {
		t.Fatalf("Historical() error = %v", err)
	}
	if len(items.Items) != 5 {
		t.Fatalf("historical items = %d, want 5", len(items.Items))
	}
	if items.Items[0].ProviderCode != "BRFG" || items.Items[0].ArticleID != "BRFG$1f064106" {
		t.Fatalf("first historical item = %+v", items.Items[0])
	}

	article, err := client.News().Article(ctx, ibkr.NewsArticleRequest{
		ProviderCode: items.Items[0].ProviderCode,
		ArticleID:    items.Items[0].ArticleID,
	})
	if err != nil {
		t.Fatalf("Article() error = %v", err)
	}
	if article.ArticleType != 0 {
		t.Fatalf("article type = %d, want 0", article.ArticleType)
	}
	if !strings.Contains(article.ArticleText, "Apple (AAPL -8%)") ||
		!strings.Contains(article.ArticleText, "supply constraints") {
		t.Fatalf("article text = %q, want captured Apple earnings article", article.ArticleText)
	}
}

func TestDirectCancelOrder(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "direct_cancel_order.txt")
	defer cleanupClientHost(t, client, host)
	events := client.SessionEvents()

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
			LmtPrice:  new(decimal.RequireFromString("10")),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "sanitized-order-ref-0000000000000001",
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if handle.OrderID() != 506 {
		t.Fatalf("OrderID = %d, want 506", handle.OrderID())
	}

	// The after-hours order remains PreSubmitted until the direct cancel.
	waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusPreSubmitted)

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
	handle.Close()
	if err := handle.Wait(); err != nil {
		t.Fatalf("handle.Wait() error = %v", err)
	}
}

// Regression: missing extOperator and manualOrderIndicator fields caused the
// Gateway to silently drop cancel_order. This replay is the current sv225
// after-hours lifecycle: the order remains PreSubmitted and cancels cleanly.
func TestAPIOrderRestCancelAAPL(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_order_rest_cancel_aapl.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	replayDelayedAAPLQuoteAnchor(t, ctx, client)

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			ConID: 265598, Symbol: "AAPL", SecType: ibkr.SecTypeStock,
			Exchange: "SMART", Currency: "USD",
		},
		Order: ibkr.Order{
			Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeLimit,
			Quantity: decimal.RequireFromString("1"),
			LmtPrice: new(decimal.RequireFromString("10")),
			TIF:      ibkr.TIFDay, Account: "DU9000001",
			OrderRef: "sanitized-order-ref-0000000000000001",
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	if handle.OrderID() != 514 {
		t.Fatalf("OrderID() = %d, want 514", handle.OrderID())
	}
	waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusPreSubmitted)
	if warning := waitForOrderWarning(t, ctx, handle); warning.Code != 399 {
		t.Fatalf("order warning = %v, want off-hours code 399", warning)
	}

	if err := handle.Cancel(ctx); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	statuses := waitOrderStatuses(t, ctx, handle)
	if !hasOrderStatus(statuses, ibkr.OrderStatusCancelled) {
		t.Fatalf("statuses = %v, want Cancelled", statuses)
	}
	// waitOrderStatuses detaches the handle after the terminal status. Observe
	// that local teardown before the transcript's final CurrentTime exchange:
	// the live capture ends immediately after the fence, so its EOF may
	// otherwise race the queued detach and correctly mark the still-observed
	// order as requiring recovery.
	if err := handle.Wait(); err != nil {
		t.Fatalf("handle.Wait() error = %v, want nil after cancellation notice", err)
	}
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime() post-cancel fence error = %v", err)
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

func cleanupClientHost(t *testing.T, client *ibkr.Client, host *testhost.Host) {
	t.Helper()
	defer client.Close()
	if t.Failed() {
		// A failed assertion can leave the scripted host waiting for a request
		// the test will never send. Closing first unblocks that read.
		client.Close()
	}
	waitHost(t, host)
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

// Grounded fixture tests: real field values extracted from live IB Gateway captures.

func TestGroundedBootstrap(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "grounded_bootstrap.txt")
	defer cleanupClientHost(t, client, host)

	snapshot := client.Session()
	if snapshot.State != ibkr.StateReady {
		t.Fatalf("state = %s, want %s", snapshot.State, ibkr.StateReady)
	}
	if snapshot.ServerVersion != 225 {
		t.Fatalf("server version = %d, want 225", snapshot.ServerVersion)
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
			t.Errorf("farm-status code %d not observed", code)
		}
	}
}

func TestGroundedContractDetailsAAPL(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "grounded_contract_details_aapl.txt")
	defer cleanupClientHost(t, client, host)

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
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	values, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Group: "All",
		Tags:  []string{"NetLiquidation", "TotalCashValue", "BuyingPower", "ExcessLiquidity"},
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

	if v, ok := byTag["NetLiquidation"]; !ok || v.Value != "36992.49" {
		t.Errorf("NetLiquidation = %q, want 36992.49", v.Value)
	}
	if v, ok := byTag["TotalCashValue"]; !ok || v.Value != "-1441.67" {
		t.Errorf("TotalCashValue = %q, want -1441.67", v.Value)
	}
	if v, ok := byTag["BuyingPower"]; !ok || v.Value != "183875.22" {
		t.Errorf("BuyingPower = %q, want 183875.22", v.Value)
	}
	if v, ok := byTag["ExcessLiquidity"]; !ok || v.Value != "28591.69" {
		t.Errorf("ExcessLiquidity = %q, want 28591.69", v.Value)
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

func TestGroundedPositions(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "grounded_positions.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	positions, err := client.Accounts().Positions(ctx)
	if err != nil {
		t.Fatalf("PositionsSnapshot() error = %v", err)
	}
	if len(positions) != 8 {
		t.Fatalf("positions len = %d, want 8", len(positions))
	}
	foundZeroMES := false
	for _, position := range positions {
		if position.Account != "DU9000001" {
			t.Fatalf("position account = %q, want DU9000001", position.Account)
		}
		foundZeroMES = foundZeroMES || position.Contract.LocalSymbol == "MESU6" && position.Position.IsZero()
	}
	if !foundZeroMES {
		t.Fatal("positions snapshot lacks the live-attested zero-quantity MESU6 row")
	}
}

func TestAccountUpdatesSnapshot(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "account_updates.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	values, err := client.Accounts().Updates(ctx, "DU9000001")
	if err != nil {
		t.Fatalf("AccountUpdatesSnapshot() error = %v", err)
	}
	if len(values) != 4 {
		t.Fatalf("values len = %d, want 4", len(values))
	}
	if values[0].AccountValue == nil {
		t.Fatal("first value is nil AccountValue")
	}
	if values[0].AccountValue.Key != "AccountCode" || values[0].AccountValue.Value != "DU9000001" {
		t.Fatalf("first value = %+v, want sanitized AccountCode", values[0].AccountValue)
	}
	if values[1].AccountValue == nil {
		t.Fatal("second value is nil AccountValue")
	}
	if got := values[1].AccountValue; got.Key != "Billable" || got.Value != "0.00" || got.Currency != "EUR" {
		t.Fatalf("second value = %+v, want Billable=0.00 EUR", got)
	}
	if values[2].Portfolio == nil {
		t.Fatal("third value is nil Portfolio")
	}
	if values[2].Portfolio.Contract.Symbol != "VEEV" || values[2].Portfolio.Position.String() != "10" {
		t.Fatalf("portfolio = %+v, want VEEV position 10", values[2].Portfolio)
	}
	if values[3].UpdateTime == nil || *values[3].UpdateTime != "22:21" {
		t.Fatalf("update time = %v, want 22:21", values[3].UpdateTime)
	}
}

func TestAccountUpdatesMultiSnapshot(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "account_updates_multi.txt", ibkr.WithSubscriptionBuffer(256))
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	allValues, err := client.Accounts().UpdatesMulti(ctx, ibkr.AccountUpdatesMultiRequest{
		Account:   "DU9000001",
		ModelCode: "",
	})
	if err != nil {
		t.Fatalf("AccountUpdatesMultiSnapshot(ledger=false) error = %v", err)
	}
	if len(allValues) != 201 {
		t.Fatalf("ledger=false values len = %d, want 201", len(allValues))
	}
	if allValues[0].Account != "DU9000001" {
		t.Fatalf("ledger=false first account = %q, want DU9000001", allValues[0].Account)
	}

	sub, err := client.Accounts().SubscribeUpdatesMulti(ctx, ibkr.AccountUpdatesMultiRequest{
		Account:      "DU9000001",
		ModelCode:    "",
		LedgerAndNLV: true,
	})
	if err != nil {
		t.Fatalf("SubscribeUpdatesMulti() error = %v", err)
	}
	var values []ibkr.AccountUpdateMultiValue
	for {
		event := waitForEvent(t, sub.Events())
		if event.Err != nil {
			t.Fatalf("account updates multi event error = %v", event.Err)
		}
		if event.Kind == ibkr.StreamData {
			values = append(values, event.Value)
		}
		if event.Kind == ibkr.StreamSnapshotComplete {
			break
		}
	}
	sub.Close()
	if err := sub.Wait(); err != nil {
		t.Fatalf("SubscribeUpdatesMulti().Wait() error = %v", err)
	}
	if len(values) != 125 {
		t.Fatalf("values len = %d, want 125", len(values))
	}
	if values[0].Account != "DU9000001" || values[0].Key != "Currency" || values[0].Value != "HKD" || values[0].Currency != "HKD" {
		t.Fatalf("first value = %+v, want HKD currency row", values[0])
	}
	last := values[124]
	if last.Account != "DU9000001" || last.Key != "Cryptocurrency" || last.Value != "0.00" || last.Currency != "BASE" {
		t.Fatalf("last value = %+v, want base cryptocurrency row", last)
	}
}

func TestPositionsMultiSnapshot(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "positions_multi.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.Accounts().SubscribePositionsMulti(ctx, ibkr.PositionsMultiRequest{
		Account:   "DU9000001",
		ModelCode: "",
	})
	if err != nil {
		t.Fatalf("SubscribePositionsMulti() error = %v", err)
	}
	var values []ibkr.PositionMulti
	for {
		event := waitForEvent(t, sub.Events())
		if event.Err != nil {
			t.Fatalf("positions multi event error = %v", event.Err)
		}
		if event.Kind == ibkr.StreamData {
			values = append(values, event.Value)
		}
		if event.Kind == ibkr.StreamSnapshotComplete {
			break
		}
	}
	sub.Close()
	if err := sub.Wait(); err != nil {
		t.Fatalf("SubscribePositionsMulti().Wait() error = %v", err)
	}
	if len(values) != 19 {
		t.Fatalf("values len = %d, want 19", len(values))
	}
	if first := values[0]; first.Account != "DU9000001" || first.Contract.Symbol != "MELI" || first.Position.String() != "1" || first.AvgCost.String() != "1581.09" {
		t.Fatalf("first position = %+v, want MELI 1 at 1581.09", first)
	}
	if last := values[len(values)-1]; last.Contract.Symbol != "UBER" || last.Position.String() != "35" {
		t.Fatalf("last position = %+v, want UBER 35", last)
	}
}

func TestSubscribePnL(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "pnl.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.Accounts().SubscribePnL(ctx, ibkr.PnLRequest{
		Account:   "DU9000001",
		ModelCode: "",
	})
	if err != nil {
		t.Fatalf("SubscribePnL() error = %v", err)
	}

	waitForStateKind(t, sub.Events(), ibkr.StreamStarted)

	update := waitForStreamData(t, sub.Events())
	if update.DailyPnL.String() != "-25.20770993999969" {
		t.Fatalf("daily pnl = %s, want -25.20770993999969", update.DailyPnL.String())
	}
	if update.UnrealizedPnL.String() != "817.2099484832976" {
		t.Fatalf("unrealized pnl = %s, want 817.2099484832976", update.UnrealizedPnL.String())
	}
	if update.RealizedPnL.String() != "0" {
		t.Fatalf("realized pnl = %s, want 0", update.RealizedPnL.String())
	}

	sub.Close()
}

func waitForStateKind[T any](t *testing.T, ch <-chan ibkr.StreamEvent[T], want ibkr.StreamEventKind) ibkr.StreamEvent[T] {
	t.Helper()

	for {
		state := waitForEvent(t, ch)
		if state.Kind == want {
			return state
		}
	}
}

func waitForStreamData[T any](t *testing.T, ch <-chan ibkr.StreamEvent[T]) T {
	t.Helper()

	for {
		event := waitForEvent(t, ch)
		if event.Kind == ibkr.StreamData {
			return event.Value
		}
	}
}

func TestSoftDollarTiersIntegration(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "soft_dollar_tiers.txt")
	defer cleanupClientHost(t, client, host)

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
	defer cleanupClientHost(t, client, host)

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

	waitForStateKind(t, handle.Events(), ibkr.StreamStarted)

	// Read initial "none" update.
	initial := waitForStreamData(t, handle.Events())
	if initial.ContractInfo != "none" {
		t.Fatalf("initial ContractInfo = %q, want %q", initial.ContractInfo, "none")
	}

	handle.Close()
	if err := handle.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime() fence error = %v", err)
	}
}

func TestAPIIOCFOKAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_ioc_fok_aapl.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	replayDelayedAAPLQuoteAnchor(t, ctx, client)

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
			LmtPrice:  new(decimal.NewFromInt(240)),
			TIF:       ibkr.TIFIOC,
			Account:   "DU9000001",
			OrderRef:  "sanitized-order-ref-0000000000000001",
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
			LmtPrice:  new(decimal.NewFromInt(240)),
			TIF:       ibkr.TIFFOK,
			Account:   "DU9000001",
			OrderRef:  "sanitized-order-ref-0000000000000006",
		},
	})
	if err != nil {
		t.Fatalf("FOK marketable PlaceOrder: %v", err)
	}
	fokMarketableStatuses := waitOrderStatuses(t, ctx, fokMarketable)
	if !hasOrderStatus(fokMarketableStatuses, ibkr.OrderStatusInactive) {
		t.Fatalf("FOK marketable statuses = %v, want Inactive from live capture", fokMarketableStatuses)
	}
	fokMarketable.Close()

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
			LmtPrice:  new(decimal.NewFromInt(10)),
			TIF:       ibkr.TIFFOK,
			Account:   "DU9000001",
			OrderRef:  "sanitized-order-ref-0000000000000009",
		},
	})
	if err != nil {
		t.Fatalf("FOK far PlaceOrder: %v", err)
	}
	fokFarStatuses := waitOrderStatuses(t, ctx, fokFar)
	if !hasOrderStatus(fokFarStatuses, ibkr.OrderStatusInactive) {
		t.Fatalf("FOK far statuses = %v, want Inactive from live capture", fokFarStatuses)
	}
	fokFar.Close()

	executions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU9000001", Symbol: "AAPL"})
	if err != nil {
		t.Fatalf("Executions(AAPL): %v", err)
	}
	if len(executions.Executions) != 0 || len(executions.CommissionAndFees) != 0 {
		t.Fatalf("IOC/FOK executions/fees = %d/%d, want 0/0", len(executions.Executions), len(executions.CommissionAndFees))
	}
}

func TestAPITIFAttributeMatrixAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_tif_attribute_matrix_aapl.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	replayDelayedAAPLQuoteAnchor(t, ctx, client)

	contract := ibkr.Contract{
		ConID: 265598, Symbol: "AAPL", SecType: ibkr.SecTypeStock, Exchange: "SMART", Currency: "USD",
	}
	farBuy := decimal.RequireFromString("15.55")
	farSell := decimal.RequireFromString("3109.2")
	refs := []string{
		"sanitized-order-ref-0000000000000001",
		"sanitized-order-ref-0000000000000006",
		"sanitized-order-ref-0000000000000011",
		"sanitized-order-ref-0000000000000016",
		"sanitized-order-ref-0000000000000022",
		"sanitized-order-ref-0000000000000024",
		"sanitized-order-ref-0000000000000031",
		"sanitized-order-ref-0000000000000036",
		"sanitized-order-ref-00000040",
		"sanitized-order-ref-0000000000000046",
		"sanitized-order-ref-0000000000000052",
		"sanitized-order-ref-0000000000000054",
		"sanitized-order-ref-0000000000000059",
		"sanitized-order-ref-0000000000000064",
		"sanitized-order-ref-0000000000000070",
	}
	base := func(index int, action ibkr.OrderAction, orderType ibkr.OrderType, quantity decimal.Decimal) ibkr.Order {
		return ibkr.Order{
			Action: action, OrderType: orderType, Quantity: quantity, TIF: ibkr.TIFDay,
			Account: "DU9000001", OrderRef: refs[index],
		}
	}
	limit := func(order ibkr.Order) ibkr.Order {
		order.LmtPrice = new(farBuy)
		return order
	}

	cases := []struct {
		name  string
		order ibkr.Order
	}{
		{name: "GTC", order: func() ibkr.Order {
			order := limit(base(0, ibkr.ActionBuy, ibkr.OrderTypeLimit, decimal.NewFromInt(1)))
			order.TIF = ibkr.TIFGTC
			return order
		}()},
		{name: "GTD", order: func() ibkr.Order {
			order := limit(base(1, ibkr.ActionBuy, ibkr.OrderTypeLimit, decimal.NewFromInt(1)))
			order.TIF = ibkr.TIFGTD
			order.GoodTillDate = "20260825 00:44:45 UTC"
			return order
		}()},
		{name: "good-after", order: func() ibkr.Order {
			order := limit(base(2, ibkr.ActionBuy, ibkr.OrderTypeLimit, decimal.NewFromInt(1)))
			order.GoodAfterTime = "20260824 22:46:45 UTC"
			return order
		}()},
		{name: "all-or-none", order: func() ibkr.Order {
			order := limit(base(3, ibkr.ActionBuy, ibkr.OrderTypeLimit, decimal.NewFromInt(1)))
			order.AllOrNone = new(true)
			return order
		}()},
		{name: "minimum-quantity", order: func() ibkr.Order {
			order := limit(base(4, ibkr.ActionBuy, ibkr.OrderTypeLimit, decimal.NewFromInt(3)))
			order.MinQty = new(2)
			return order
		}()},
		{name: "relative-percent-offset", order: func() ibkr.Order {
			order := limit(base(5, ibkr.ActionBuy, ibkr.OrderTypeRelative, decimal.NewFromInt(1)))
			order.PercentOffset = new(decimal.RequireFromString("0.03"))
			return order
		}()},
		{name: "trailing-percent", order: func() ibkr.Order {
			order := base(6, ibkr.ActionSell, ibkr.OrderTypeTrailingStop, decimal.NewFromInt(1))
			order.TrailStopPrice = new(farSell)
			order.TrailingPercent = new(decimal.RequireFromString("1.5"))
			return order
		}()},
		{name: "trigger-method", order: func() ibkr.Order {
			order := base(7, ibkr.ActionBuy, ibkr.OrderTypeStop, decimal.NewFromInt(1))
			order.AuxPrice = new(farSell)
			order.TriggerMethod = 4
			return order
		}()},
		{name: "explicit-ref", order: limit(base(8, ibkr.ActionBuy, ibkr.OrderTypeLimit, decimal.NewFromInt(1)))},
		{name: "scale", order: func() ibkr.Order {
			order := limit(base(9, ibkr.ActionBuy, ibkr.OrderTypeLimit, decimal.NewFromInt(1)))
			order.Scale.InitialLevelSize = 1
			order.Scale.SubsequentLevelSize = 1
			order.Scale.PriceIncrement = decimal.RequireFromString("0.05")
			return order
		}()},
		{name: "active-window", order: func() ibkr.Order {
			order := limit(base(10, ibkr.ActionBuy, ibkr.OrderTypeLimit, decimal.NewFromInt(1)))
			order.Scale.ActiveStartTime = "20260824 22:46:45 UTC"
			order.Scale.ActiveStopTime = "20260824 22:48:45 UTC"
			return order
		}()},
		{name: "price-management", order: func() ibkr.Order {
			order := limit(base(11, ibkr.ActionBuy, ibkr.OrderTypeLimit, decimal.NewFromInt(1)))
			order.UsePriceMgmtAlgo = new(true)
			return order
		}()},
		{name: "adjusted-stop", order: func() ibkr.Order {
			order := base(12, ibkr.ActionSell, ibkr.OrderTypeStop, decimal.NewFromInt(1))
			order.AuxPrice = new(farBuy)
			order.Adjustment = ibkr.OrderAdjustment{
				OrderType: ibkr.OrderTypeStopLimit, TriggerPrice: farBuy,
				StopPrice: decimal.RequireFromString("14.55"), StopLimitPrice: decimal.RequireFromString("15.05"),
				TrailingAmount: decimal.NewFromInt(1),
			}
			return order
		}()},
		{name: "manual-time", order: func() ibkr.Order {
			order := limit(base(13, ibkr.ActionBuy, ibkr.OrderTypeLimit, decimal.NewFromInt(1)))
			order.ManualOrderTime = "20260824 22:44:45 UTC"
			return order
		}()},
		{name: "advanced-error-override", order: func() ibkr.Order {
			order := limit(base(14, ibkr.ActionBuy, ibkr.OrderTypeLimit, decimal.NewFromInt(1)))
			order.AdvancedErrorOverride = "IBDBUYTX"
			return order
		}()},
	}

	handles := make([]*ibkr.OrderHandle, 0, len(cases))
	for i, tc := range cases {
		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: contract, Order: tc.order})
		if err != nil {
			t.Fatalf("%s Place: %v", tc.name, err)
		}
		if handle.OrderID() != 581+int64(i) {
			t.Fatalf("%s OrderID() = %d, want %d", tc.name, handle.OrderID(), 581+i)
		}
		handles = append(handles, handle)
		if err := handle.Cancel(ctx); err != nil {
			t.Fatalf("%s Cancel: %v", tc.name, err)
		}
	}

	executions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU9000001", Symbol: "AAPL"})
	if err != nil {
		t.Fatalf("Executions(AAPL): %v", err)
	}
	if len(executions.Executions) != 0 || len(executions.CommissionAndFees) != 0 {
		t.Fatalf("TIF matrix executions/fees = %d/%d, want 0/0", len(executions.Executions), len(executions.CommissionAndFees))
	}
	for i, handle := range handles {
		handle.Close()
		requireCloseOrCapturedDisconnect(t, cases[i].name, handle.Wait())
	}
}

func TestAPIDollarCostAveragingAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_dollar_cost_averaging_aapl.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	contract := ibkr.Contract{
		ConID:    265598,
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	}

	refs := []string{
		"sanitized-order-ref-0000000000000001",
		"sanitized-order-ref-0000000000000006",
		"sanitized-order-ref-0000000000000011",
	}
	for i := 0; i < 3; i++ {
		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: contract,
			Order: ibkr.Order{
				Action:    ibkr.ActionBuy,
				OrderType: ibkr.OrderTypeMarket,
				Quantity:  decimal.NewFromInt(1),
				TIF:       ibkr.TIFDay,
				Account:   "DU9000001",
				OrderRef:  refs[i],
			},
		})
		if err != nil {
			t.Fatalf("DCA buy[%d] PlaceOrder: %v", i, err)
		}
		if handle.OrderID() != int64(495+i) {
			t.Fatalf("DCA buy[%d] OrderID() = %d, want %d", i, handle.OrderID(), 495+i)
		}
		waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusPreSubmitted)
		handle.Close()
		requireCloseOrCapturedDisconnect(t, fmt.Sprintf("DCA buy[%d]", i), handle.Wait())
	}

	executions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU9000001", Symbol: "AAPL"})
	if err != nil {
		t.Fatalf("DCA Executions: %v", err)
	}
	if len(executions.Executions) != 0 || len(executions.CommissionAndFees) != 0 {
		t.Fatalf("DCA executions/fees = %d/%d, want 0/0 after-hours", len(executions.Executions), len(executions.CommissionAndFees))
	}
}

func TestAPIStressRapidFireAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_stress_rapid_fire_aapl.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	replayDelayedAAPLQuoteAnchor(t, ctx, client)

	contract := ibkr.Contract{
		ConID:    265598,
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	}
	prices := []string{"15.53", "16.53", "17.53", "18.53", "19.53", "20.53", "21.53", "22.53", "23.53", "24.53"}
	handles := make([]*ibkr.OrderHandle, 0, len(prices))
	orderIDs := map[int64]bool{}
	for i, price := range prices {
		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: contract,
			Order: ibkr.Order{
				Action:    ibkr.ActionBuy,
				OrderType: ibkr.OrderTypeLimit,
				Quantity:  decimal.RequireFromString("1"),
				LmtPrice:  new(decimal.RequireFromString(price)),
				TIF:       ibkr.TIFDay,
				Account:   "DU9000001",
				OrderRef:  fmt.Sprintf("sanitized-order-ref-%016d", 2*i+1),
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
		waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusPreSubmitted)
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
		requireCloseOrCapturedDisconnect(t, fmt.Sprintf("stress order[%d]", i), handle.Wait())
	}
}

func TestAPIForexLifecycleEURUSDReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_forex_lifecycle_eurusd.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	contract := ibkr.Contract{
		Symbol: "EUR", SecType: ibkr.SecTypeForex, Exchange: "IDEALPRO", Currency: "USD",
	}
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataLive); err != nil {
		t.Fatalf("SetType(live): %v", err)
	}
	quote, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: contract})
	if err != nil {
		t.Fatalf("EUR.USD Quote: %v", err)
	}
	if !quote.Ask.Equal(decimal.RequireFromString("1.16644")) {
		t.Fatalf("EUR.USD ask = %s, want captured 1.16644", quote.Ask)
	}

	order := ibkr.Order{
		Action:    ibkr.ActionBuy,
		OrderType: ibkr.OrderTypeLimit,
		Quantity:  decimal.NewFromInt(100000),
		LmtPrice:  new(decimal.RequireFromString("1.0498")),
		TIF:       ibkr.TIFDay,
		Account:   "DU9000001",
		OrderRef:  "sanitized-order-ref-0000000000000001",
	}
	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: contract,
		Order:    order,
	})
	if err != nil {
		t.Fatalf("forex PlaceOrder: %v", err)
	}

	statuses := waitOrderStatuses(t, ctx, handle)
	if !hasOrderStatus(statuses, ibkr.OrderStatusInactive) {
		t.Fatalf("forex statuses = %v, want Inactive from live capture", statuses)
	}
	if handle.OrderID() != 498 {
		t.Fatalf("forex OrderID() = %d, want 498", handle.OrderID())
	}
	order.LmtPrice = new(decimal.RequireFromString("1.07312"))
	if err := handle.Replace(ctx, order); err != nil {
		t.Fatalf("forex Replace: %v", err)
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
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	replayDelayedAAPLQuoteAnchor(t, ctx, client)

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
			OrderRef:  "sanitized-order-ref-0000000000000001",
		},
		TakeProfit: ibkr.Order{
			Action:    ibkr.ActionSell,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  new(decimal.RequireFromString("3106")),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "sanitized-order-ref-0000000000000003",
		},
		StopLoss: ibkr.Order{
			Action:    ibkr.ActionSell,
			OrderType: ibkr.OrderTypeStop,
			Quantity:  decimal.RequireFromString("1"),
			AuxPrice:  new(decimal.RequireFromString("15.53")),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "sanitized-order-ref-0000000000000005",
		},
	})
	if err != nil {
		t.Fatalf("PlaceBracket: %v", err)
	}
	parent := bracket.Parent
	takeProfit := bracket.TakeProfit
	stopLoss := bracket.StopLoss

	if parent.OrderID() != 477 || takeProfit.OrderID() != 478 || stopLoss.OrderID() != 479 {
		t.Fatalf("bracket order IDs = %d/%d/%d, want 477/478/479", parent.OrderID(), takeProfit.OrderID(), stopLoss.OrderID())
	}
	parentOpen := waitForOpenOrder(t, ctx, parent)
	if parentOpen.State.Status != ibkr.OrderStatusPreSubmitted {
		t.Fatalf("bracket parent status = %s, want PreSubmitted", parentOpen.State.Status)
	}
	waitForOrderStatus(t, ctx, parent, ibkr.OrderStatusPreSubmitted)
	takeProfitOpen := waitForOpenOrder(t, ctx, takeProfit)
	waitForOrderStatus(t, ctx, takeProfit, ibkr.OrderStatusPreSubmitted)
	stopLossOpen := waitForOpenOrder(t, ctx, stopLoss)
	waitForOrderStatus(t, ctx, stopLoss, ibkr.OrderStatusPreSubmitted)
	if (*takeProfitOpen.Order.ParentID) != parent.OrderID() {
		t.Fatalf("take-profit parent = %d, want %d", (*takeProfitOpen.Order.ParentID), parent.OrderID())
	}
	if (*stopLossOpen.Order.ParentID) != parent.OrderID() {
		t.Fatalf("stop-loss parent = %d, want %d", (*stopLossOpen.Order.ParentID), parent.OrderID())
	}
	if takeProfitOpen.Order.OCA.Group == "" || stopLossOpen.Order.OCA.Group != takeProfitOpen.Order.OCA.Group {
		t.Fatalf("child OCA groups = %q and %q, want same non-empty group", takeProfitOpen.Order.OCA.Group, stopLossOpen.Order.OCA.Group)
	}

	for label, handle := range map[string]*ibkr.OrderHandle{
		"parent": parent, "take-profit": takeProfit, "stop-loss": stopLoss,
	} {
		handle.Close()
		requireCloseOrCapturedDisconnect(t, "bracket "+label, handle.Wait())
	}
}

func TestAPIBracketTrailingStopAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_bracket_trailing_stop_aapl.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	replayDelayedAAPLQuoteAnchor(t, ctx, client)

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
			Quantity:  decimal.RequireFromString("1"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			Transmit:  new(false),
			OrderRef:  "sanitized-order-ref-0000000000000001",
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
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  new(decimal.RequireFromString("3105.2")),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			ParentID:  parent.OrderID(),
			Transmit:  new(false),
			OrderRef:  "sanitized-order-ref-0000000000000003",
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
			Quantity:       decimal.RequireFromString("1"),
			AuxPrice:       new(decimal.RequireFromString("1")),
			TIF:            ibkr.TIFDay,
			Account:        "DU9000001",
			ParentID:       parent.OrderID(),
			TrailStopPrice: new(decimal.RequireFromString("15.53")),
			OrderRef:       "sanitized-order-ref-0000000000000005",
		},
	})
	if err != nil {
		t.Fatalf("trailing bracket stop PlaceOrder: %v", err)
	}
	if trailingStop.OrderID() != parent.OrderID()+2 {
		t.Fatalf("trailing stop order ID = %d, want parent+2 from live capture", trailingStop.OrderID())
	}
	if parent.OrderID() != 474 || takeProfit.OrderID() != 475 || trailingStop.OrderID() != 476 {
		t.Fatalf("trailing bracket order IDs = %d/%d/%d, want 474/475/476", parent.OrderID(), takeProfit.OrderID(), trailingStop.OrderID())
	}
	err = trailingStop.Wait()
	apiErr, ok := errors.AsType[*ibkr.APIError](err)
	if !ok || apiErr.Code != 328 || !strings.Contains(apiErr.Message, "Trailing stop orders can be attached") {
		t.Fatalf("trailing stop Wait = %v, want code 328 attachment rejection", err)
	}
	if err := client.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("trailing bracket CancelAll: %v", err)
	}
	executions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU9000001", Symbol: "AAPL"})
	if err != nil {
		t.Fatalf("trailing bracket Executions: %v", err)
	}
	if len(executions.Executions) != 0 || len(executions.CommissionAndFees) != 0 {
		t.Fatalf("trailing bracket executions/fees = %d/%d, want 0/0", len(executions.Executions), len(executions.CommissionAndFees))
	}
	for label, handle := range map[string]*ibkr.OrderHandle{
		"parent": parent, "take-profit": takeProfit,
	} {
		handle.Close()
		requireCloseOrCapturedDisconnect(t, "trailing bracket "+label, handle.Wait())
	}
}

func TestAPIOCATriggerAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_oca_trigger_aapl.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	replayDelayedAAPLQuoteAnchor(t, ctx, client)

	contract := ibkr.Contract{
		ConID:    265598,
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	}
	group := "oca-0000000000000000000001"
	resting, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: contract,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  new(decimal.RequireFromString("10")),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "sanitized-order-ref-0000000000000001",
			OCA:       ibkr.OrderOCA{Group: group, Type: ibkr.OCACancelWithBlock},
		},
	})
	if err != nil {
		t.Fatalf("OCA resting PlaceOrder: %v", err)
	}
	if resting.OrderID() != 504 {
		t.Fatalf("OCA resting OrderID() = %d, want 504", resting.OrderID())
	}

	marketable, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: contract,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  new(decimal.RequireFromString("240")),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "sanitized-order-ref-0000000000000003",
			OCA:       ibkr.OrderOCA{Group: group, Type: ibkr.OCACancelWithBlock},
		},
	})
	if err != nil {
		t.Fatalf("OCA marketable PlaceOrder: %v", err)
	}
	if marketable.OrderID() != 505 {
		t.Fatalf("OCA marketable OrderID() = %d, want 505", marketable.OrderID())
	}

	restingOpen := waitForOpenOrder(t, ctx, resting)
	if restingOpen.Order.OCA.Group != group {
		t.Fatalf("resting OCA group = %q, want %q", restingOpen.Order.OCA.Group, group)
	}
	waitForOrderStatus(t, ctx, resting, ibkr.OrderStatusPreSubmitted)

	marketableOpen := waitForOpenOrder(t, ctx, marketable)
	if marketableOpen.Order.OCA.Group != group {
		t.Fatalf("marketable OCA group = %q, want %q", marketableOpen.Order.OCA.Group, group)
	}
	waitForOrderStatus(t, ctx, marketable, ibkr.OrderStatusPreSubmitted)

	resting.Close()
	marketable.Close()
	requireCloseOrCapturedDisconnect(t, "OCA resting", resting.Wait())
	requireCloseOrCapturedDisconnect(t, "OCA marketable", marketable.Wait())
}

func TestAPIPairsTradingAAPLMSFTReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_pairs_trading_aapl_msft.txt")
	defer cleanupClientHost(t, client, host)

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
	for _, contract := range []ibkr.Contract{aapl, msft} {
		_, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: contract})
		apiErr, ok := errors.AsType[*ibkr.APIError](err)
		if !ok || apiErr.Code != ibkr.ErrCodeAdditionalSubscriptionRequired || apiErr.OpKind != ibkr.OpQuotes {
			t.Fatalf("%s Quote error = %v, want typed quotes code %d", contract.Symbol, err, ibkr.ErrCodeAdditionalSubscriptionRequired)
		}
	}

	aaplBuy, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: aapl,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("1"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "sanitized-order-ref-0000000000000001",
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
			Quantity:  decimal.RequireFromString("1"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "sanitized-order-ref-0000000000000003",
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
		waitForOrderStatus(t, ctx, entry.handle, ibkr.OrderStatusPreSubmitted)
	}
	if aaplBuy.OrderID() != 540 || msftSell.OrderID() != 541 {
		t.Fatalf("pairs order IDs = %d/%d, want 540/541", aaplBuy.OrderID(), msftSell.OrderID())
	}
	for _, symbol := range []string{"AAPL", "MSFT"} {
		executions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU9000001", Symbol: symbol})
		if err != nil {
			t.Fatalf("%s Executions: %v", symbol, err)
		}
		if len(executions.Executions) != 0 || len(executions.CommissionAndFees) != 0 {
			t.Fatalf("%s executions/fees = %d/%d, want 0/0", symbol, len(executions.Executions), len(executions.CommissionAndFees))
		}
	}
	for label, handle := range map[string]*ibkr.OrderHandle{"AAPL": aaplBuy, "MSFT": msftSell} {
		handle.Close()
		requireCloseOrCapturedDisconnect(t, label+" pair", handle.Wait())
	}
}

func TestAPICompletedOrdersVariantsAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_completed_orders_variants_aapl.txt")
	defer cleanupClientHost(t, client, host)

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
			Quantity:  decimal.NewFromInt(1),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "sanitized-order-ref-0000000000000001",
		},
	})
	if err != nil {
		t.Fatalf("entry PlaceOrder: %v", err)
	}
	if buy.OrderID() != 486 {
		t.Fatalf("entry OrderID() = %d, want 486", buy.OrderID())
	}
	waitForOrderStatus(t, ctx, buy, ibkr.OrderStatusPreSubmitted)

	allCompleted, err := client.Orders().Completed(ctx, false)
	if err != nil {
		t.Fatalf("Completed(false): %v", err)
	}
	if len(allCompleted) != 24 {
		t.Fatalf("Completed(false) len = %d, want 24", len(allCompleted))
	}

	completedSub, err := client.Orders().StreamCompleted(ctx, true)
	if err != nil {
		t.Fatalf("StreamCompleted(true): %v", err)
	}
	var apiCompleted []ibkr.CompletedOrderResult
	var completedSnapshot bool
	for event := range completedSub.Events() {
		if event.Err != nil {
			t.Fatalf("completed-order event error = %v", event.Err)
		}
		switch event.Kind {
		case ibkr.StreamData:
			apiCompleted = append(apiCompleted, event.Value)
		case ibkr.StreamSnapshotComplete:
			completedSnapshot = true
		}
	}
	if err := completedSub.Wait(); err != nil {
		t.Fatalf("StreamCompleted(true).Wait(): %v", err)
	}
	if !completedSnapshot {
		t.Fatal("StreamCompleted(true) closed without SnapshotComplete")
	}
	if len(apiCompleted) != 24 {
		t.Fatalf("StreamCompleted(true) len = %d, want 24", len(apiCompleted))
	}
	for i, completed := range apiCompleted {
		if completed.Contract.Symbol != "AAPL" || completed.Completion.Status == "" {
			t.Fatalf("Completed(true)[%d] = %+v, want terminal AAPL order", i, completed)
		}
	}

	executions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU9000001", Symbol: "AAPL"})
	if err != nil {
		t.Fatalf("Executions(AAPL): %v", err)
	}
	if len(executions.Executions) != 0 || len(executions.CommissionAndFees) != 0 {
		t.Fatalf("Executions(AAPL) = %+v, want empty current-session snapshot", executions)
	}
}

func TestAPIFutureCampaignMESReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_future_campaign_mes.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	details, err := client.Contracts().Details(ctx, ibkr.Contract{
		Symbol: "MES", SecType: ibkr.SecTypeFuture, Exchange: "CME", Currency: "USD",
	})
	if err != nil {
		t.Fatalf("MES ContractDetails: %v", err)
	}
	var contract ibkr.Contract
	for _, detail := range details {
		if detail.ConID == 793356217 {
			contract = detail.Contract
			break
		}
	}
	if contract.ConID == 0 {
		t.Fatal("MES details lack captured front future 793356217")
	}
	if _, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: contract}); err != nil {
		apiErr, ok := errors.AsType[*ibkr.APIError](err)
		if !ok || apiErr.Code != ibkr.ErrCodeMarketDataNotSubscribed || apiErr.OpKind != ibkr.OpQuotes {
			t.Fatalf("MES Quote error = %v, want typed code %d", err, ibkr.ErrCodeMarketDataNotSubscribed)
		}
	}

	buy, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: contract,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("1"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "sanitized-order-ref-0000000000000001",
		},
	})
	if err != nil {
		t.Fatalf("future buy PlaceOrder: %v", err)
	}
	buyFilled, buyExecution := waitOrderFillAndExecution(t, ctx, buy)
	if !buyFilled || !buyExecution {
		t.Fatalf("future buy filled=%v execution=%v, want both true", buyFilled, buyExecution)
	}
	if buy.OrderID() != 499 {
		t.Fatalf("future buy OrderID() = %d, want 499", buy.OrderID())
	}

	sell, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: contract,
		Order: ibkr.Order{
			Action:    ibkr.ActionSell,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("1"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "sanitized-order-ref-0000000000000006",
		},
	})
	if err != nil {
		t.Fatalf("future flatten PlaceOrder: %v", err)
	}
	sellFilled, sellExecution := waitOrderFillAndExecution(t, ctx, sell)
	if !sellFilled || !sellExecution {
		t.Fatalf("future flatten filled=%v execution=%v, want both true", sellFilled, sellExecution)
	}
	if sell.OrderID() != 500 {
		t.Fatalf("future flatten OrderID() = %d, want 500", sell.OrderID())
	}

	positions, err := client.Accounts().Positions(ctx)
	if err != nil {
		t.Fatalf("Positions: %v", err)
	}
	if len(positions) != 8 {
		t.Fatalf("Positions len = %d, want captured reconciled 8 rows", len(positions))
	}
	executions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU9000001", Symbol: "MES"})
	if err != nil {
		t.Fatalf("MES Executions: %v", err)
	}
	if len(executions.Executions) != 2 || len(executions.CommissionAndFees) != 2 {
		t.Fatalf("MES executions/fees = %d/%d, want 2/2", len(executions.Executions), len(executions.CommissionAndFees))
	}
}

func TestAPIReconnectActiveOrderAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_reconnect_active_order_aapl.txt",
		ibkr.WithReconnectPolicy(ibkr.ReconnectAuto))
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("baseline CancelAll: %v", err)
	}
	replayDelayedAAPLQuoteAnchor(t, ctx, client)

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
			LmtPrice:  new(decimal.RequireFromString("15.53")),
			TIF:       ibkr.TIFGTC,
			Account:   "DU9000001",
			OrderRef:  "sanitized-order-ref-0000000000000001",
		},
	})
	if err != nil {
		t.Fatalf("first Place: %v", err)
	}
	waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusPreSubmitted)
	if handle.OrderID() != 546 {
		t.Fatalf("reconnect OrderID() = %d, want 546", handle.OrderID())
	}

	gap := waitForOrderLifecycle(t, ctx, handle.Events(), ibkr.OrderGap)
	if gap.ConnectionSeq != 1 || gap.Err == nil {
		t.Fatalf("order gap = %+v, want connection 1 with transport cause", gap)
	}
	recovery := waitForOrderLifecycle(t, ctx, handle.Events(), ibkr.OrderRecoveryRequired)
	if recovery.ConnectionSeq != 2 || !errors.Is(recovery.Err, ibkr.ErrOrderRecoveryRequired) {
		t.Fatalf("order recovery = %+v, want ErrOrderRecoveryRequired on connection 2", recovery)
	}
	if errors.Is(recovery.Err, ibkr.ErrResumeRequired) || ibkr.IsRetryable(recovery.Err) {
		t.Fatalf("order recovery error = %v, want non-retryable order recovery", recovery.Err)
	}
	if err := handle.Replace(ctx, ibkr.Order{
		Action:    ibkr.ActionBuy,
		OrderType: ibkr.OrderTypeLimit,
		Quantity:  decimal.RequireFromString("1"),
		LmtPrice:  new(decimal.RequireFromString("15.53")),
		TIF:       ibkr.TIFGTC,
		Account:   "DU9000001",
		OrderRef:  "sanitized-order-ref-0000000000000001",
	}); !errors.Is(err, ibkr.ErrOrderRecoveryRequired) || ibkr.IsRetryable(err) {
		t.Fatalf("Replace after reconnect = %v, want non-retryable ErrOrderRecoveryRequired", err)
	}
	waitForSessionReady(t, ctx, client.SessionEvents(), 2)

	orders, err := client.Orders().Open(ctx, ibkr.OpenOrdersScopeClient)
	if err != nil {
		t.Fatalf("Open(client) after reconnect: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("open orders len = %d, want 1", len(orders))
	}
	if *orders[0].Order.OrderID != handle.OrderID() || orders[0].Order.TIF != ibkr.TIFGTC || orders[0].Order.Quantity.String() != "1" {
		t.Fatalf("open order = %+v, want reconnected GTC order %d qty 1", orders[0], handle.OrderID())
	}
	if err := handle.Cancel(ctx); err != nil {
		t.Fatalf("OrderHandle.Cancel after reconnect: %v", err)
	}
	statuses := waitOrderStatuses(t, ctx, handle)
	if !hasOrderStatus(statuses, ibkr.OrderStatusCancelled) {
		t.Fatalf("statuses after reconnect cancel = %v, want Cancelled", statuses)
	}
	if err := handle.Wait(); err != nil {
		t.Fatalf("OrderHandle.Wait after reconnect cancel = %v", err)
	}
}

func TestAPIClientID0OrderObservationAAPLReplay(t *testing.T) {
	t.Parallel()

	host := newHost(t, "api_client_id0_order_observation_aapl.txt")
	defer waitHost(t, host)

	placer := dialHostClient(t, host, ibkr.WithClientID(1))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := placer.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("baseline CancelAll: %v", err)
	}
	replayDelayedAAPLQuoteAnchor(t, ctx, placer)

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
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  new(decimal.RequireFromString("15.53")),
			TIF:       ibkr.TIFGTC,
			Account:   "DU9000001",
			OrderRef:  "sanitized-order-ref-0000000000000001",
		},
	})
	if err != nil {
		t.Fatalf("placer Place: %v", err)
	}
	waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusPreSubmitted)
	if handle.OrderID() != 481 {
		t.Fatalf("client0 observed OrderID() = %d, want 481", handle.OrderID())
	}
	placer.Close()

	observer := dialHostClient(t, host, ibkr.WithClientID(0))
	defer observer.Close()

	orders, err := observer.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
	if err != nil {
		t.Fatalf("client0 Open(all): %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("client0 open orders len = %d, want 1", len(orders))
	}
	if *orders[0].Order.OrderID != handle.OrderID() || *orders[0].Order.ClientID != 1 {
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
	if err := placer.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("baseline CancelAll: %v", err)
	}
	replayDelayedAAPLQuoteAnchor(t, ctx, placer)

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
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  new(decimal.RequireFromString("10")),
			TIF:       ibkr.TIFGTC,
			Account:   "DU9000001",
			OrderRef:  "sanitized-order-ref-0000000000000001",
		},
	})
	if err != nil {
		t.Fatalf("placer Place: %v", err)
	}
	waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusPreSubmitted)
	if handle.OrderID() != 493 {
		t.Fatalf("cross-client OrderID() = %d, want 493", handle.OrderID())
	}
	placer.Close()

	canceller := dialHostClient(t, host, ibkr.WithClientID(2))
	defer canceller.Close()

	orders, err := canceller.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
	if err != nil {
		t.Fatalf("client2 Open(all): %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("client2 open orders len = %d, want 1", len(orders))
	}
	if *orders[0].Order.OrderID != handle.OrderID() || *orders[0].Order.ClientID != 1 {
		t.Fatalf("client2 open order = %+v, want original client order %d", orders[0], handle.OrderID())
	}
	if err := canceller.Orders().Cancel(ctx, handle.OrderID()); err != nil {
		t.Fatalf("client2 Cancel: %v", err)
	}
	waitHost(t, host)
}

func TestAPITransmitFalseThenTransmitAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_transmit_false_then_transmit_aapl.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	contract := ibkr.Contract{
		ConID:    265598,
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	}

	if err := client.MarketData().SetType(ctx, ibkr.MarketDataLive); err != nil {
		t.Fatalf("SetType(live): %v", err)
	}
	_, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: contract})
	apiErr, ok := errors.AsType[*ibkr.APIError](err)
	if !ok || apiErr.Code != ibkr.ErrCodeAdditionalSubscriptionRequired || apiErr.OpKind != ibkr.OpQuotes {
		t.Fatalf("live Quote error = %v, want typed quotes code %d", err, ibkr.ErrCodeAdditionalSubscriptionRequired)
	}
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("SetType(delayed): %v", err)
	}
	quote, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: contract})
	if err != nil {
		t.Fatalf("delayed Quote: %v", err)
	}
	if !quote.Bid.Equal(decimal.RequireFromString("310.55")) || !quote.Ask.Equal(decimal.RequireFromString("310.6")) {
		t.Fatalf("delayed quote bid/ask = %s/%s, want 310.55/310.6", quote.Bid, quote.Ask)
	}

	order := ibkr.Order{
		Action:    ibkr.ActionBuy,
		OrderType: ibkr.OrderTypeLimit,
		Quantity:  decimal.NewFromInt(1),
		LmtPrice:  new(decimal.RequireFromString("15.53")),
		TIF:       ibkr.TIFDay,
		Account:   "DU9000001",
		OrderRef:  "sanitized-order-ref-0000000000000001",
		Transmit:  new(false),
	}
	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: contract, Order: order})
	if err != nil {
		t.Fatalf("Transmit=false Place: %v", err)
	}
	if handle.OrderID() != 557 {
		t.Fatalf("OrderID() = %d, want 557", handle.OrderID())
	}

	order.Transmit = new(true)
	if err := handle.Replace(ctx, order); err != nil {
		t.Fatalf("Transmit=true Replace: %v", err)
	}
	waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusPreSubmitted)
	warning := waitForOrderWarning(t, ctx, handle)
	if warning.Code != 399 || warning.OpKind != ibkr.OpPlaceOrder || !strings.Contains(warning.Message, "will not be placed at the exchange until") {
		t.Fatalf("replacement warning = %+v, want typed off-hours code 399", warning)
	}
	if ibkr.IsRetryable(warning) {
		t.Fatalf("replacement warning = %v, want non-retryable", warning)
	}

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
	if ibkr.IsTerminalOrderStatus(want) && want != ibkr.OrderStatusInactive {
		handle.Close()
	}
}

func waitForOrderLifecycle(t *testing.T, ctx context.Context, events <-chan ibkr.OrderEvent, want ibkr.OrderLifecycleKind) ibkr.OrderLifecycleEvent {
	t.Helper()
	for {
		select {
		case event, ok := <-events:
			if !ok {
				t.Fatalf("order events closed before lifecycle %s", want)
			}
			if event.Lifecycle != nil && event.Lifecycle.Kind == want {
				return *event.Lifecycle
			}
		case <-ctx.Done():
			t.Fatalf("waiting for order lifecycle %s: %v", want, context.Cause(ctx))
		}
	}
}

func waitForSessionReady(t *testing.T, ctx context.Context, events <-chan ibkr.Event, connectionSeq uint64) {
	t.Helper()
	for {
		select {
		case event, ok := <-events:
			if !ok {
				t.Fatalf("session events closed before Ready on connection %d", connectionSeq)
			}
			if event.State == ibkr.StateReady && event.ConnectionSeq >= connectionSeq {
				return
			}
		case <-ctx.Done():
			t.Fatalf("waiting for Ready on connection %d: %v", connectionSeq, context.Cause(ctx))
		}
	}
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
			if filled && execution {
				handle.Close()
				return filled, execution
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
				if ibkr.IsTerminalOrderStatus(evt.Status.Status) {
					if evt.Status.Status != ibkr.OrderStatusInactive {
						handle.Close()
					}
					return statuses
				}
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

func requireCloseOrCapturedDisconnect(t *testing.T, label string, err error) {
	t.Helper()
	if err == nil {
		return
	}
	if errors.Is(err, ibkr.ErrOrderRecoveryRequired) && errors.Is(err, io.EOF) {
		return
	}
	t.Fatalf("%s Close/Wait: %v", label, err)
}

func replayAAPLQuoteEntitlement(t *testing.T, ctx context.Context, client *ibkr.Client) {
	t.Helper()

	if err := client.MarketData().SetType(ctx, ibkr.MarketDataLive); err != nil {
		t.Fatalf("SetType(live): %v", err)
	}
	_, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: ibkr.Contract{
		ConID: 265598, Symbol: "AAPL", SecType: ibkr.SecTypeStock, Exchange: "SMART", Currency: "USD",
	}})
	apiErr, ok := errors.AsType[*ibkr.APIError](err)
	if !ok || apiErr.Code != ibkr.ErrCodeAdditionalSubscriptionRequired || apiErr.OpKind != ibkr.OpQuotes {
		t.Fatalf("live AAPL Quote error = %v, want typed quotes code %d", err, ibkr.ErrCodeAdditionalSubscriptionRequired)
	}
}

func replayDelayedAAPLQuoteAnchor(t *testing.T, ctx context.Context, client *ibkr.Client) ibkr.Quote {
	t.Helper()

	replayAAPLQuoteEntitlement(t, ctx, client)
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("SetType(delayed): %v", err)
	}
	quote, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: ibkr.Contract{
		ConID: 265598, Symbol: "AAPL", SecType: ibkr.SecTypeStock, Exchange: "SMART", Currency: "USD",
	}})
	if err != nil {
		t.Fatalf("delayed AAPL Quote: %v", err)
	}
	return quote
}

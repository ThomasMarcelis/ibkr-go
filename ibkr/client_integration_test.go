package ibkr_test

import (
	"context"
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
	if snapshot.ServerVersion != 1 {
		t.Fatalf("server version = %d, want 1", snapshot.ServerVersion)
	}
	if len(snapshot.ManagedAccounts) != 2 {
		t.Fatalf("managed accounts = %v, want 2 entries", snapshot.ManagedAccounts)
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
	if values[0].Tag != "NetLiquidation" {
		t.Fatalf("first tag = %q, want NetLiquidation", values[0].Tag)
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

func newClient(t *testing.T, script string) (*ibkr.Client, *testhost.Host) {
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

	client, err := ibkr.DialContext(ctx,
		ibkr.WithHost(addrHost),
		ibkr.WithPort(port),
		ibkr.WithReconnectPolicy(ibkr.ReconnectOff),
	)
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

//go:build legacy_native_socket

package ibkr_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
)

func TestReconnectLastPriceSurvivesTransportLoss(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "reconnect_multi_cycle.txt",
		ibkr.WithReconnectPolicy(ibkr.ReconnectAuto))
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	// Session 1: started + last-price tick
	started := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionStarted)
	if started.ConnectionSeq != 1 {
		t.Fatalf("started.ConnectionSeq = %d, want 1", started.ConnectionSeq)
	}

	first := waitForEvent(t, sub.Events())
	if first.Snapshot.Last.String() != "250" {
		t.Fatalf("first last = %s, want 250", first.Snapshot.Last.String())
	}

	// Reconnect: gap + resumed + updated last-price tick
	gap := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionGap)
	if gap.ConnectionSeq != 1 {
		t.Fatalf("gap.ConnectionSeq = %d, want 1", gap.ConnectionSeq)
	}

	resumed := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionResumed)
	if resumed.ConnectionSeq != 2 {
		t.Fatalf("resumed.ConnectionSeq = %d, want 2", resumed.ConnectionSeq)
	}

	second := waitForEvent(t, sub.Events())
	if second.Snapshot.Last.String() != "251" {
		t.Fatalf("second last = %s, want 251", second.Snapshot.Last.String())
	}

	// Verify ConnectionSeq advanced
	if got := client.Session().ConnectionSeq; got != 2 {
		t.Fatalf("client.Session().ConnectionSeq = %d, want 2", got)
	}

	// Verify managed accounts from reconnected session
	accts := client.Session().ManagedAccounts
	if len(accts) != 1 || accts[0] != "DU9000001" {
		t.Fatalf("managed accounts = %v, want [DU9000001]", accts)
	}

	if err := sub.Close(); err != nil {
		t.Fatalf("sub.Close() error = %v", err)
	}
	if err := sub.Wait(); err != nil {
		t.Fatalf("sub.Wait() error = %v", err)
	}
}

func TestReconnectOneShotInterrupted(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "reconnect_oneshot_interrupted.txt",
		ibkr.WithReconnectPolicy(ibkr.ReconnectOff))
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := client.History().Bars(ctx, ibkr.HistoricalBarsRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		EndTime:    time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
		Duration:   ibkr.Days(1),
		BarSize:    ibkr.Bar1Hour,
		WhatToShow: ibkr.ShowTrades,
		UseRTH:     true,
	})
	if !errors.Is(err, ibkr.ErrInterrupted) {
		t.Fatalf("HistoricalBars() error = %v, want %v", err, ibkr.ErrInterrupted)
	}
}

func TestReconnectPolicyOff(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "reconnect_policy_off.txt",
		ibkr.WithReconnectPolicy(ibkr.ReconnectOff))
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

	first := waitForEvent(t, sub.Events())
	if first.Snapshot.Last.String() != "250" {
		t.Fatalf("first last = %s, want 250", first.Snapshot.Last.String())
	}

	// Transport drops. With ReconnectOff, the subscription and client close.
	if err := sub.Wait(); err == nil {
		t.Fatal("sub.Wait() error = nil, want error on disconnect")
	}

	select {
	case <-client.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("client.Done() did not complete after disconnect")
	}

	if err := client.Wait(); err == nil {
		t.Fatal("client.Wait() error = nil, want error on disconnect")
	}
}

// TestSingleGapOn1100ThenTransportLoss verifies that when IBKR sends code 1100
// (connectivity lost) and then the TCP connection drops, the subscription
// receives exactly one Gap event, not two.
func TestSingleGapOn1100ThenTransportLoss(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "reconnect_1100_then_transport_loss.txt",
		ibkr.WithReconnectPolicy(ibkr.ReconnectAuto))
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	// Wait for started + initial tick
	waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionStarted)
	first := waitForEvent(t, sub.Events())
	if first.Snapshot.Last.String() != "250" {
		t.Fatalf("first last = %s, want 250", first.Snapshot.Last.String())
	}

	// Code 1100 arrives, then transport drops. Should see exactly one Gap.
	gap := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionGap)
	if gap.ConnectionSeq != 1 {
		t.Fatalf("gap.ConnectionSeq = %d, want 1", gap.ConnectionSeq)
	}

	// After reconnect, should get Resumed (not another Gap first).
	resumed := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionResumed)
	if resumed.ConnectionSeq != 2 {
		t.Fatalf("resumed.ConnectionSeq = %d, want 2", resumed.ConnectionSeq)
	}

	// Verify data flows again
	second := waitForEvent(t, sub.Events())
	if second.Snapshot.Last.String() != "251" {
		t.Fatalf("second last = %s, want 251", second.Snapshot.Last.String())
	}

	_ = sub.Close()
}

func TestGapEventWithout1101(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "reconnect_gap_no_resume.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
	if first.Snapshot.Last.String() != "250" {
		t.Fatalf("first last = %s, want 250", first.Snapshot.Last.String())
	}

	// Code 1100 arrives: subscription emits Gap
	gap := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionGap)
	if gap.ConnectionSeq != 1 {
		t.Fatalf("gap.ConnectionSeq = %d, want 1", gap.ConnectionSeq)
	}

	// Session state should be Degraded
	if got := client.Session().State; got != ibkr.StateDegraded {
		t.Fatalf("session state = %s, want %s", got, ibkr.StateDegraded)
	}

	// The subscription is still alive: Done channel not closed
	select {
	case <-sub.Done():
		t.Fatal("sub.Done() closed after 1100, but subscription should still be alive")
	default:
	}

	// Close subscription manually
	if err := sub.Close(); err != nil {
		t.Fatalf("sub.Close() error = %v", err)
	}
	if err := sub.Wait(); err != nil {
		t.Fatalf("sub.Wait() error = %v", err)
	}
}

func TestDegradedToReadyVia1102(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "reconnect_1102_resume.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	// First tick
	first := waitForEvent(t, sub.Events())
	if first.Snapshot.Last.String() != "250" {
		t.Fatalf("first last = %s, want 250", first.Snapshot.Last.String())
	}

	// Code 1100 -> Gap
	gap := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionGap)
	if gap.ConnectionSeq != 1 {
		t.Fatalf("gap.ConnectionSeq = %d, want 1", gap.ConnectionSeq)
	}

	// Code 1102 -> Resumed (no re-subscribe, data maintained)
	resumed := waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionResumed)
	if resumed.ConnectionSeq != 1 {
		t.Fatalf("resumed.ConnectionSeq = %d, want 1 (same session, no transport loss)", resumed.ConnectionSeq)
	}

	// More ticks arrive after resume
	second := waitForEvent(t, sub.Events())
	if second.Snapshot.Last.String() != "251" {
		t.Fatalf("second last = %s, want 251", second.Snapshot.Last.String())
	}

	// Session state restored to Ready
	if got := client.Session().State; got != ibkr.StateReady {
		t.Fatalf("session state = %s, want %s", got, ibkr.StateReady)
	}

	if err := sub.Close(); err != nil {
		t.Fatalf("sub.Close() error = %v", err)
	}
	if err := sub.Wait(); err != nil {
		t.Fatalf("sub.Wait() error = %v", err)
	}
}

package ibkr_test

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/ibkr"
)

// TestBootstrapNoManagedAccounts verifies that DialContext fails with a timeout
// when the server sends next_valid_id but never sends managed_accounts.
func TestBootstrapNoManagedAccounts(t *testing.T) {
	t.Parallel()

	host := newHost(t, "lifecycle_bootstrap_no_accounts.txt")

	addrHost, addrPort, err := net.SplitHostPort(host.Addr())
	if err != nil {
		t.Fatalf("SplitHostPort() error = %v", err)
	}
	port, err := net.LookupPort("tcp", addrPort)
	if err != nil {
		t.Fatalf("LookupPort() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = ibkr.DialContext(ctx,
		ibkr.WithHost(addrHost),
		ibkr.WithPort(port),
		ibkr.WithReconnectPolicy(ibkr.ReconnectOff),
	)
	if err == nil {
		t.Fatal("expected error from DialContext, got nil")
	}
	// Script is still sleeping; do not waitHost.
	_ = host.Close()
}

// TestBootstrapNoNextValidID verifies that DialContext fails with a timeout
// when the server sends managed_accounts but never sends next_valid_id.
func TestBootstrapNoNextValidID(t *testing.T) {
	t.Parallel()

	host := newHost(t, "lifecycle_bootstrap_no_valid_id.txt")

	addrHost, addrPort, err := net.SplitHostPort(host.Addr())
	if err != nil {
		t.Fatalf("SplitHostPort() error = %v", err)
	}
	port, err := net.LookupPort("tcp", addrPort)
	if err != nil {
		t.Fatalf("LookupPort() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = ibkr.DialContext(ctx,
		ibkr.WithHost(addrHost),
		ibkr.WithPort(port),
		ibkr.WithReconnectPolicy(ibkr.ReconnectOff),
	)
	if err == nil {
		t.Fatal("expected error from DialContext, got nil")
	}
	_ = host.Close()
}

// TestBootstrapOutOfOrder verifies that bootstrap completes even when the
// server sends next_valid_id before managed_accounts.
func TestBootstrapOutOfOrder(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "lifecycle_bootstrap_reordered.txt")
	defer client.Close()
	defer waitHost(t, host)

	snapshot := client.Session()
	if snapshot.State != ibkr.StateReady {
		t.Fatalf("state = %s, want %s", snapshot.State, ibkr.StateReady)
	}
	if len(snapshot.ManagedAccounts) != 1 || snapshot.ManagedAccounts[0] != "DU9000001" {
		t.Fatalf("managed accounts = %v, want [DU9000001]", snapshot.ManagedAccounts)
	}
	if snapshot.NextValidID != 1 {
		t.Fatalf("next valid id = %d, want 1", snapshot.NextValidID)
	}
}

// TestContextCancelDuringOneShot verifies that a one-shot method returns a
// context error when the caller's context expires before the server responds.
func TestContextCancelDuringOneShot(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "lifecycle_context_cancel.txt")
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := client.ContractDetails(ctx, ibkr.ContractDetailsRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  "STK",
			Exchange: "SMART",
			Currency: "USD",
		},
	})
	if err == nil {
		t.Fatal("expected error from ContractDetails, got nil")
	}
	// The host script is still sleeping; close the listener to unblock it.
	_ = host.Close()
}

// TestSubscriptionCloseImmediatelyAfterCreate verifies that closing a
// subscription immediately after creation does not panic and Wait returns nil.
func TestSubscriptionCloseImmediatelyAfterCreate(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "lifecycle_subscription_close_immediate.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.SubscribePositions(ctx)
	if err != nil {
		t.Fatalf("SubscribePositions() error = %v", err)
	}

	if err := sub.Close(); err != nil {
		t.Fatalf("sub.Close() error = %v", err)
	}
	if err := sub.Wait(); err != nil {
		t.Fatalf("sub.Wait() error = %v", err)
	}
}

// TestSingletonSubscriptionRejectsSecond verifies that a second positions
// subscription is rejected while the first is still active.
func TestSingletonSubscriptionRejectsSecond(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "lifecycle_singleton_reject.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub1, err := client.SubscribePositions(ctx)
	if err != nil {
		t.Fatalf("SubscribePositions() first error = %v", err)
	}

	// Wait for snapshot complete so we know the first subscription is established.
	waitForStateKind(t, sub1.State(), ibkr.SubscriptionSnapshotComplete)

	sub2, err := client.SubscribePositions(ctx)
	if err == nil {
		if sub2 != nil {
			_ = sub2.Close()
		}
		t.Fatal("SubscribePositions() second error = nil, want rejection")
	}

	if err := sub1.Close(); err != nil {
		t.Fatalf("sub1.Close() error = %v", err)
	}
}

// TestConcurrentAccountSummaryLimit verifies that account summary enforces
// a maximum of 2 concurrent subscriptions.
func TestConcurrentAccountSummaryLimit(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "lifecycle_account_summary_limit.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub1, err := client.SubscribeAccountSummary(ctx, ibkr.AccountSummaryRequest{
		Account: "All",
		Tags:    []string{"NetLiquidation"},
	})
	if err != nil {
		t.Fatalf("SubscribeAccountSummary() first error = %v", err)
	}

	// Wait for first subscription's snapshot to complete before creating second.
	waitForStateKind(t, sub1.State(), ibkr.SubscriptionSnapshotComplete)

	sub2, err := client.SubscribeAccountSummary(ctx, ibkr.AccountSummaryRequest{
		Account: "All",
		Tags:    []string{"BuyingPower"},
	})
	if err != nil {
		t.Fatalf("SubscribeAccountSummary() second error = %v", err)
	}

	// Wait for second subscription's snapshot to complete.
	waitForStateKind(t, sub2.State(), ibkr.SubscriptionSnapshotComplete)

	// Third subscription must be rejected.
	sub3, err := client.SubscribeAccountSummary(ctx, ibkr.AccountSummaryRequest{
		Account: "All",
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

// TestMultipleOneShotsInFlight verifies that concurrent one-shot requests
// are correctly demultiplexed even when the server responds out of order.
// Both requests are issued from goroutines. The server responds to MSFT
// ($req2) before AAPL ($req1), exercising req-id-based routing.
func TestMultipleOneShotsInFlight(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "lifecycle_concurrent_oneshots.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type result struct {
		details []ibkr.ContractDetails
		err     error
	}

	var wg sync.WaitGroup
	results := make([]result, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		d, err := client.ContractDetails(ctx, ibkr.ContractDetailsRequest{
			Contract: ibkr.Contract{
				Symbol:   "AAPL",
				SecType:  "STK",
				Exchange: "SMART",
				Currency: "USD",
			},
		})
		results[0] = result{details: d, err: err}
	}()
	go func() {
		defer wg.Done()
		// Small delay so AAPL request is queued first, matching transcript.
		time.Sleep(50 * time.Millisecond)
		d, err := client.ContractDetails(ctx, ibkr.ContractDetailsRequest{
			Contract: ibkr.Contract{
				Symbol:   "MSFT",
				SecType:  "STK",
				Exchange: "SMART",
				Currency: "USD",
			},
		})
		results[1] = result{details: d, err: err}
	}()
	wg.Wait()

	if results[0].err != nil {
		t.Fatalf("AAPL ContractDetails() error = %v", results[0].err)
	}
	if len(results[0].details) != 1 {
		t.Fatalf("AAPL details len = %d, want 1", len(results[0].details))
	}
	if results[0].details[0].Contract.Symbol != "AAPL" {
		t.Fatalf("AAPL symbol = %q, want AAPL", results[0].details[0].Contract.Symbol)
	}

	if results[1].err != nil {
		t.Fatalf("MSFT ContractDetails() error = %v", results[1].err)
	}
	if len(results[1].details) != 1 {
		t.Fatalf("MSFT details len = %d, want 1", len(results[1].details))
	}
	if results[1].details[0].Contract.Symbol != "MSFT" {
		t.Fatalf("MSFT symbol = %q, want MSFT", results[1].details[0].Contract.Symbol)
	}
}

// TestSessionEventsDelivered verifies that the session events channel
// receives a Ready event after bootstrap completes.
func TestSessionEventsDelivered(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "lifecycle_bootstrap_reordered.txt")
	defer client.Close()
	defer waitHost(t, host)

	events := client.SessionEvents()
	found := false
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				if !found {
					t.Fatal("session events channel closed without Ready event")
				}
				return
			}
			if ev.State == ibkr.StateReady {
				found = true
			}
		case <-time.After(5 * time.Second):
			if !found {
				t.Fatal("timed out waiting for Ready session event")
			}
			return
		}
		if found {
			return
		}
	}
}

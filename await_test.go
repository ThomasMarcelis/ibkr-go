package ibkr

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
)

func TestEnqueueOneShotSetupSkipsCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	e := &engine{
		cmds: make(chan func(), 1),
		done: make(chan struct{}),
	}

	called := false
	enqueueOneShotSetup(ctx, e, func() {
		called = true
	})

	fn := <-e.cmds
	fn()

	if called {
		t.Fatal("enqueueOneShotSetup executed canceled work")
	}
}

func TestEnqueueSubscriptionSetupSkipsCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	e := &engine{
		cmds: make(chan func(), 1),
		done: make(chan struct{}),
	}
	resp := make(chan int, 1)

	called := false
	enqueueSubscriptionSetup(ctx, e, resp, func() {
		called = true
	})

	fn := <-e.cmds
	fn()

	if called {
		t.Fatal("enqueueSubscriptionSetup executed canceled work")
	}

	select {
	case got := <-resp:
		if got != 0 {
			t.Fatalf("enqueueSubscriptionSetup zero result = %d, want 0", got)
		}
	default:
		t.Fatal("enqueueSubscriptionSetup did not publish zero result")
	}
}

func TestAwaitOneShotResponseCancelsImmediately(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		e := &engine{done: make(chan struct{})}
		resp := make(chan int)
		canceled := make(chan struct{}, 1)

		_, err := awaitOneShotResponse(ctx, e, resp, func() {
			canceled <- struct{}{}
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("awaitOneShotResponse() error = %v, want context.Canceled", err)
		}

		select {
		case <-canceled:
		default:
			t.Fatal("cancel callback did not run")
		}
	})
}

func TestAwaitSubscriptionResponseRollsBackLateResult(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		e := &engine{done: make(chan struct{})}
		resp := make(chan int, 1)
		rolledBack := make(chan int, 1)

		_, err := awaitSubscriptionResponse(ctx, e, resp, func(v int) {
			rolledBack <- v
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("awaitSubscriptionResponse() error = %v, want context.Canceled", err)
		}

		resp <- 42

		synctest.Wait()
		select {
		case got := <-rolledBack:
			if got != 42 {
				t.Fatalf("rollback value = %d, want 42", got)
			}
		default:
			t.Fatal("rollback did not run for late subscription result")
		}
	})
}

func TestAwaitSubscriptionResponseReturnsContextErrorWhenAlreadyCanceled(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		e := &engine{done: make(chan struct{})}
		resp := make(chan int, 1)
		resp <- 42
		rolledBack := make(chan int, 1)

		_, err := awaitSubscriptionResponse(ctx, e, resp, func(v int) {
			rolledBack <- v
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("awaitSubscriptionResponse() error = %v, want context.Canceled", err)
		}

		synctest.Wait()
		select {
		case got := <-rolledBack:
			if got != 42 {
				t.Fatalf("rollback value = %d, want 42", got)
			}
		default:
			t.Fatal("rollback did not run for already-canceled response")
		}
	})
}

func TestEnqueueOneShotSetupRunsActiveContext(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	e := &engine{
		cmds:    make(chan func(), 1),
		done:    make(chan struct{}),
		adapter: sdkadapter.NewReplayAdapter(nil),
		snapshot: Snapshot{
			State: StateReady,
		},
	}

	called := false
	enqueueOneShotSetup(ctx, e, func() {
		called = true
	})

	fn := <-e.cmds
	fn()

	if !called {
		t.Fatal("enqueueOneShotSetup did not execute active work")
	}
}

func TestEnqueueOneShotSetupWaitsForReady(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		e := &engine{
			cmds: make(chan func(), 2),
			done: make(chan struct{}),
		}

		called := false
		enqueueOneShotSetup(ctx, e, func() {
			called = true
		})

		(<-e.cmds)()
		if called {
			t.Fatal("enqueueOneShotSetup ran before the session was ready")
		}

		time.Sleep(reconnectBackoff)
		synctest.Wait()
		e.adapter = sdkadapter.NewReplayAdapter(nil)
		e.snapshot = Snapshot{State: StateReady}
		(<-e.cmds)()

		if !called {
			t.Fatal("enqueueOneShotSetup did not run after readiness returned")
		}
	})
}

func TestEnqueueHistoricalSetupForwardsCancelDuringPacing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		e := &engine{
			cmds:                  make(chan func(), 2),
			done:                  make(chan struct{}),
			adapter:               sdkadapter.NewReplayAdapter(nil),
			nextHistoricalRequest: time.Now().Add(time.Second),
			snapshot:              Snapshot{State: StateReady},
		}

		resp := make(chan int, 1)
		called := false
		enqueueHistoricalSetup(ctx, e, "key", func() {
			resp <- 0
		}, func() {
			called = true
		})

		(<-e.cmds)()
		if called {
			t.Fatal("enqueueHistoricalSetup ran while request was paced")
		}

		cancel()
		time.Sleep(time.Second)
		synctest.Wait()
		(<-e.cmds)()

		select {
		case got := <-resp:
			if got != 0 {
				t.Fatalf("canceled setup result = %d, want zero", got)
			}
		default:
			t.Fatal("enqueueHistoricalSetup did not publish canceled setup result")
		}
		if called {
			t.Fatal("enqueueHistoricalSetup ran work after cancellation")
		}
	})
}

func TestEnqueueHistoricalSetupPrunesStalePacingKeys(t *testing.T) {
	t.Parallel()

	now := time.Now()
	ctx := context.Background()
	e := &engine{
		cmds:    make(chan func(), 1),
		done:    make(chan struct{}),
		adapter: sdkadapter.NewReplayAdapter(nil),
		recentHistoricalRequests: map[string]time.Time{
			"old":    now.Add(-historicalIdenticalSpacing - time.Second),
			"recent": now.Add(-historicalIdenticalSpacing / 2),
		},
		snapshot: Snapshot{State: StateReady},
	}

	called := false
	enqueueHistoricalSetup(ctx, e, "new", nil, func() {
		called = true
	})
	(<-e.cmds)()

	if !called {
		t.Fatal("enqueueHistoricalSetup did not run active work")
	}
	if _, ok := e.recentHistoricalRequests["old"]; ok {
		t.Fatal("stale historical pacing key was not pruned")
	}
	if _, ok := e.recentHistoricalRequests["recent"]; !ok {
		t.Fatal("recent historical pacing key was pruned")
	}
	if _, ok := e.recentHistoricalRequests["new"]; !ok {
		t.Fatal("new historical pacing key was not recorded")
	}
}

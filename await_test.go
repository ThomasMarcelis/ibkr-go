package ibkr

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/transport"
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

func TestAwaitSubscriptionResponseAdmissionWinsCallerBoundaries(t *testing.T) {
	t.Run("caller cancellation waits for admission result", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			resp := make(chan int, 1)
			result := make(chan struct {
				value int
				err   error
			}, 1)
			go func() {
				value, err := awaitSubscriptionResponse(canceledContext(), placementWaitEngine(false), resp, func(v int) bool { return v != 0 })
				result <- struct {
					value int
					err   error
				}{value: value, err: err}
			}()

			synctest.Wait()
			select {
			case out := <-result:
				t.Fatalf("awaitSubscriptionResponse() returned before admission result: %+v", out)
			default:
			}

			resp <- 42
			synctest.Wait()
			out := <-result
			if out.err != nil || out.value != 42 {
				t.Fatalf("awaitSubscriptionResponse() = (%d, %v), want (42, nil)", out.value, out.err)
			}
		})
	})

	t.Run("engine shutdown preserves buffered actor result", func(t *testing.T) {
		resp := make(chan int, 1)
		resp <- 42
		got, err := awaitSubscriptionResponse(context.Background(), placementWaitEngine(true), resp, func(v int) bool { return v != 0 })
		if err != nil || got != 42 {
			t.Fatalf("awaitSubscriptionResponse() = (%d, %v), want (42, nil)", got, err)
		}
	})

	t.Run("pre-admission cancellation returns context error", func(t *testing.T) {
		resp := make(chan int, 1)
		resp <- 0 // enqueueSubscriptionSetup's pre-admission cancellation result.
		got, err := awaitSubscriptionResponse(canceledContext(), placementWaitEngine(false), resp, func(v int) bool { return v != 0 })
		if got != 0 {
			t.Fatalf("pre-admission result = %d, want zero", got)
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("pre-admission error = %v, want context.Canceled", err)
		}
	})
}

func TestFireAndForgetAdmissionResultWinsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	e := &engine{
		cmds:      make(chan func(), 1),
		done:      make(chan struct{}),
		transport: new(transport.Conn),
		snapshot:  Snapshot{State: StateReady},
	}
	result := make(chan error, 1)
	go func() {
		result <- awaitFireAndForget(ctx, e, func(context.Context) error {
			cancel()
			return nil
		})
	}()
	(<-e.cmds)()
	if err := <-result; err != nil {
		t.Fatalf("awaitFireAndForget() = %v, want admitted nil result", err)
	}
}

func TestEnqueueOneShotSetupRunsActiveContext(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	e := &engine{
		cmds:      make(chan func(), 1),
		done:      make(chan struct{}),
		transport: &transport.Conn{},
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
	t.Parallel()

	ctx := context.Background()
	e := &engine{
		cmds: make(chan func(), 1),
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

	e.transport = &transport.Conn{}
	e.snapshot = Snapshot{State: StateReady}
	e.flushReadySetups()
	if !called {
		t.Fatal("enqueueOneShotSetup did not run when readiness returned")
	}
}

func TestEnqueueSubscriptionSetupRemovesCanceledWaiter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		e := &engine{
			cmds: make(chan func(), 1),
			done: make(chan struct{}),
		}
		resp := make(chan int, 1)

		enqueueSubscriptionSetup(ctx, e, resp, func() {
			t.Fatal("canceled setup executed")
		})
		(<-e.cmds)()
		if len(e.readySetups) != 1 {
			t.Fatalf("ready setups = %d, want 1", len(e.readySetups))
		}

		cancel()
		synctest.Wait()
		(<-e.cmds)()
		if len(e.readySetups) != 0 {
			t.Fatalf("ready setups after cancel = %d, want 0", len(e.readySetups))
		}
		select {
		case got := <-resp:
			if got != 0 {
				t.Fatalf("canceled setup result = %d, want zero", got)
			}
		default:
			t.Fatal("canceled setup did not publish a result")
		}
	})
}

func TestEnqueueHistoricalSetupForwardsCancelDuringPacing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		e := &engine{
			cmds:                  make(chan func(), 2),
			done:                  make(chan struct{}),
			transport:             &transport.Conn{},
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
		cmds:      make(chan func(), 1),
		done:      make(chan struct{}),
		transport: &transport.Conn{},
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

func TestEnqueueClockSetupCancelsBeforeWrite(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		e := &engine{
			cmds:             make(chan func(), 4),
			done:             make(chan struct{}),
			transport:        &transport.Conn{},
			nextClockRequest: time.Now().Add(clockRequestSpacing),
			singletons:       make(map[string]*route),
			snapshot:         Snapshot{State: StateReady},
		}
		go e.run()
		defer close(e.done)

		canceled := make(chan struct{}, 1)
		called := false
		enqueueClockSetup(ctx, e, singletonCurrentTime, func() {
			canceled <- struct{}{}
		}, func() {
			t.Fatal("inactive clock route reported active")
		}, func() {
			called = true
		})
		synctest.Wait()
		if called {
			t.Fatal("clock setup ran before the pacing boundary")
		}

		cancel()
		<-canceled
		if called {
			t.Fatal("clock setup ran after cancellation")
		}
	})
}

func TestEnqueueClockSetupUsesSharedGate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := &engine{
			cmds:       make(chan func(), 2),
			done:       make(chan struct{}),
			transport:  &transport.Conn{},
			singletons: make(map[string]*route),
			snapshot:   Snapshot{State: StateReady},
		}
		go e.run()
		defer close(e.done)
		firstCalled := false
		secondCalled := false
		enqueueClockSetup(context.Background(), e, singletonCurrentTime, nil, func() {}, func() {
			firstCalled = true
		})
		synctest.Wait()
		ctx, cancel := context.WithCancel(context.Background())
		enqueueClockSetup(ctx, e, singletonCurrentTimeMillis, nil, func() {}, func() {
			secondCalled = true
		})
		synctest.Wait()

		if !firstCalled {
			t.Fatal("first clock opcode did not run")
		}
		if secondCalled {
			t.Fatal("millisecond clock opcode bypassed the shared pacing gate")
		}
		cancel()
		synctest.Wait()
		if secondCalled {
			t.Fatal("canceled millisecond clock opcode ran after pacing")
		}
	})
}

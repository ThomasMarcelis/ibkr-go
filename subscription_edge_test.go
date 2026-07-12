package ibkr

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func TestSubscriptionEmitAfterClose(t *testing.T) {
	t.Parallel()

	sub := newSubscription[int](subscriptionConfig{buffer: 1}, func() {})
	sub.closeWithErr(nil)

	if sub.emit(42) {
		t.Error("emit after close returned true, want false")
	}
}

func TestSubscriptionDoubleClose(t *testing.T) {
	t.Parallel()

	t.Run("Close twice", func(t *testing.T) {
		t.Parallel()
		sub := newSubscription[int](subscriptionConfig{buffer: 1}, func() {
			// no-op cancel
		})
		sub.closeWithErr(nil) // ensure done channel is closed so Close doesn't block

		sub.Close()
		sub.Close()
	})

	t.Run("closeWithErr twice", func(t *testing.T) {
		t.Parallel()
		sub := newSubscription[int](subscriptionConfig{buffer: 1}, func() {})
		sub.closeWithErr(nil)
		sub.closeWithErr(errors.New("second")) // must not panic
	})
}

func TestSubscriptionSlowConsumerClose(t *testing.T) {
	t.Parallel()

	sub := newSubscription[int](subscriptionConfig{buffer: 1}, nil)

	if !sub.emit(1) {
		t.Fatal("first emit returned false, want true")
	}
	if sub.emit(2) {
		t.Error("second emit (buffer full) returned true, want false")
	}

	if err := sub.Wait(); err != ErrSlowConsumer {
		t.Errorf("Wait() = %v, want exact ErrSlowConsumer", err)
	}
	if err := sub.Err(); err != ErrSlowConsumer {
		t.Errorf("Err() = %v, want exact ErrSlowConsumer", err)
	}
	if _, ok := <-sub.Events(); !ok {
		t.Fatal("buffered data event was not drainable after close")
	}
	if _, ok := <-sub.Events(); ok {
		t.Fatal("Events remained open after slow-consumer close")
	}
}

func TestSubscriptionSlowConsumerWinsCompetingTeardown(t *testing.T) {
	t.Parallel()

	for _, teardownErr := range []error{nil, ErrResumeRequired, ErrInterrupted} {
		name := "clean cancellation"
		if teardownErr != nil {
			name = teardownErr.Error()
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			sub := newSubscription[int](subscriptionConfig{buffer: 1}, func() {})
			if !sub.emit(1) {
				t.Fatal("first emit returned false, want true")
			}
			sub.Close()
			if sub.emit(2) {
				t.Fatal("in-flight emit with full queue returned true")
			}
			if err := sub.Err(); err != nil {
				t.Fatalf("Err() before actor terminal close = %v, want nil", err)
			}
			// A competing teardown may terminally close before the actor reaches
			// the queued cancel, but it must not replace the already recorded
			// local data-loss cause.
			sub.closeWithErr(teardownErr)

			if err := sub.Wait(); err != ErrSlowConsumer {
				t.Fatalf("Wait() = %v, want exact ErrSlowConsumer", err)
			}
			if err := sub.Err(); err != ErrSlowConsumer {
				t.Fatalf("Err() = %v, want exact ErrSlowConsumer", err)
			}
		})
	}
}

func TestEmitKeyedSubscriptionDeletesRouteOnSlowConsumer(t *testing.T) {
	t.Parallel()

	e := &engine{keyed: map[int]*route{7: {}}}
	var sub *Subscription[int]
	sub = newSubscription[int](subscriptionConfig{buffer: 1}, func() {
		e.deleteKeyedRoute(7)
		sub.closeWithErr(nil)
	})
	if !sub.emit(1) {
		t.Fatal("first emit returned false, want true")
	}
	if sub.emit(2) {
		t.Fatal("second emit returned true, want slow-consumer close")
	}
	if _, ok := e.keyed[7]; ok {
		t.Fatal("keyed route retained after slow-consumer close")
	}
	if err := sub.Wait(); err != ErrSlowConsumer {
		t.Fatalf("Wait() = %v, want exact ErrSlowConsumer", err)
	}
}

func TestEmitSingletonSubscriptionDeletesRouteOnSlowConsumer(t *testing.T) {
	t.Parallel()

	e := &engine{singletons: map[string]*route{singletonOpenOrders: {}}}
	var sub *Subscription[int]
	sub = newSubscription[int](subscriptionConfig{buffer: 1}, func() {
		delete(e.singletons, singletonOpenOrders)
		sub.closeWithErr(nil)
	})
	if !sub.emit(1) {
		t.Fatal("first emit returned false, want true")
	}
	if sub.emit(2) {
		t.Fatal("second emit returned true, want slow-consumer close")
	}
	if _, ok := e.singletons[singletonOpenOrders]; ok {
		t.Fatal("singleton route retained after slow-consumer close")
	}
	if err := sub.Wait(); err != ErrSlowConsumer {
		t.Fatalf("Wait() = %v, want exact ErrSlowConsumer", err)
	}
}

func TestSubscriptionWaitBlocksUntilClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sub := newSubscription[int](subscriptionConfig{buffer: 1}, func() {})

		done := make(chan error, 1)
		go func() {
			done <- sub.Wait()
		}()

		synctest.Wait()
		select {
		case <-done:
			t.Fatal("Wait() returned before close")
		default:
		}

		sub.closeWithErr(nil)

		synctest.Wait()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Wait() = %v, want nil", err)
			}
		default:
			t.Fatal("Wait() did not return after close")
		}
	})
}

func TestSubscriptionWaitReturnsError(t *testing.T) {
	t.Parallel()

	sub := newSubscription[int](subscriptionConfig{buffer: 1}, func() {})
	want := errors.New("test error")
	sub.closeWithErr(want)

	if got := sub.Wait(); !errors.Is(got, want) {
		t.Errorf("Wait() = %v, want %v", got, want)
	}
}

func TestSubscriptionErrReturnsCloseErrorWithoutBlocking(t *testing.T) {
	t.Parallel()

	sub := newSubscription[int](subscriptionConfig{buffer: 1}, func() {})
	if err := sub.Err(); err != nil {
		t.Fatalf("Err() before close = %v, want nil", err)
	}

	want := errors.New("test error")
	sub.closeWithErr(want)

	if got := sub.Err(); !errors.Is(got, want) {
		t.Fatalf("Err() after close = %v, want %v", got, want)
	}
}

func TestSubscriptionEventsPreserveLifecycleAndDataOrder(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sub := newSubscription[int](subscriptionConfig{buffer: 4}, func() {})

		sub.emitState(StreamStarted, 3, nil)
		sub.emit(42)
		sub.emitState(StreamSnapshotComplete, 3, nil)

		started := <-sub.Events()
		data := <-sub.Events()
		complete := <-sub.Events()
		if started.Kind != StreamStarted || started.ConnectionSeq != 3 {
			t.Fatalf("first event = %+v, want Started on connection 3", started)
		}
		if data.Kind != StreamData || data.Value != 42 || data.ConnectionSeq != 3 {
			t.Fatalf("second event = %+v, want data 42 on connection 3", data)
		}
		if complete.Kind != StreamSnapshotComplete || complete.ConnectionSeq != 3 {
			t.Fatalf("third event = %+v, want SnapshotComplete on connection 3", complete)
		}
		for i, event := range []StreamEvent[int]{started, data, complete} {
			if event.At.IsZero() || event.At.Location() != time.UTC {
				t.Fatalf("event %d observation time = %v, want nonzero UTC", i, event.At)
			}
		}
		if data.At.Before(started.At) || complete.At.Before(data.At) {
			t.Fatalf("event observation times are out of queue order: %v, %v, %v", started.At, data.At, complete.At)
		}
	})
}

func TestSubscriptionSnapshotCompleteFlag(t *testing.T) {
	t.Parallel()

	sub := newSubscription[int](subscriptionConfig{buffer: 1}, func() {})

	if sub.snapshotComplete() {
		t.Error("snapshotComplete() = true before any state event, want false")
	}

	sub.emitState(StreamSnapshotComplete, 1, nil)
	if !sub.snapshotComplete() {
		t.Error("snapshotComplete() = false after SnapshotComplete, want true")
	}

	// Latched: additional events do not clear it.
	sub.emitState(StreamStarted, 1, nil)
	if !sub.snapshotComplete() {
		t.Error("snapshotComplete() = false after subsequent event, want true (latched)")
	}
}

func TestSubscriptionDoneClosedOnClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sub := newSubscription[int](subscriptionConfig{buffer: 1}, func() {})

		select {
		case <-sub.Done():
			t.Fatal("Done() closed before subscription closed")
		default:
		}

		sub.closeWithErr(nil)

		select {
		case <-sub.Done():
		default:
			t.Fatal("Done() not closed after closeWithErr")
		}
	})
}

func TestSubscriptionEventsClosedOnClose(t *testing.T) {
	t.Parallel()

	sub := newSubscription[int](subscriptionConfig{buffer: 1}, func() {})
	sub.closeWithErr(nil)

	v, ok := <-sub.Events()
	if ok {
		t.Errorf("Events() receive ok = true, want false (closed channel)")
	}
	if v != (StreamEvent[int]{}) {
		t.Errorf("Events() zero-value = %+v, want zero event", v)
	}
}

func TestSubscriptionEventsDrainAfterClose(t *testing.T) {
	t.Parallel()

	sub := newSubscription[int](subscriptionConfig{buffer: 2}, func() {})
	if !sub.emit(1) {
		t.Fatal("first emit returned false, want true")
	}
	if !sub.emit(2) {
		t.Fatal("second emit returned false, want true")
	}

	sub.closeWithErr(nil)

	var got []int
	for event := range sub.Events() {
		if event.Kind == StreamData {
			got = append(got, event.Value)
		}
	}
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("drained events = %v, want [1 2]", got)
	}
	if err := sub.Wait(); err != nil {
		t.Fatalf("Wait() = %v, want nil", err)
	}
}

func TestSubscriptionCancelFnCalledOnce(t *testing.T) {
	t.Parallel()

	var count atomic.Int32
	sub := newSubscription[int](subscriptionConfig{buffer: 1}, func() {
		count.Add(1)
	})

	// Close the subscription so Wait doesn't block, then call Close multiple times.
	sub.closeWithErr(nil)

	for i := 0; i < 3; i++ {
		sub.Close()
	}

	if got := count.Load(); got != 1 {
		t.Errorf("cancelFn called %d times, want 1", got)
	}
}

// TestAwaitSnapshotReturnsNilWhenSnapshotComplete freezes the happy path:
// after SnapshotComplete is emitted, AwaitSnapshot returns nil immediately
// regardless of whether the subscription has since closed.
func TestAwaitSnapshotReturnsNilWhenSnapshotComplete(t *testing.T) {
	t.Parallel()

	sub := newSubscription[int](subscriptionConfig{buffer: 1}, func() {})
	sub.expectSnapshot()
	sub.emitState(StreamSnapshotComplete, 0, nil)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := sub.AwaitSnapshot(ctx); err != nil {
		t.Errorf("AwaitSnapshot() = %v, want nil", err)
	}

	// Same contract after a subsequent clean close.
	sub.closeWithErr(nil)
	if err := sub.AwaitSnapshot(ctx); err != nil {
		t.Errorf("AwaitSnapshot() post-close = %v, want nil", err)
	}
}

// TestAwaitSnapshotReturnsNilWhenCompleteThenClose freezes the Executions-style
// flow where SnapshotComplete and closeWithErr(nil) race; the select may wake
// on either done channel, both must yield nil.
func TestAwaitSnapshotReturnsNilWhenCompleteThenClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		for i := 0; i < 50; i++ {
			sub := newSubscription[int](subscriptionConfig{buffer: 1}, func() {})
			sub.expectSnapshot()

			done := make(chan error, 1)
			go func() { done <- sub.AwaitSnapshot(context.Background()) }()

			sub.emitState(StreamSnapshotComplete, 0, nil)
			sub.closeWithErr(nil)

			synctest.Wait()
			select {
			case err := <-done:
				if err != nil {
					t.Errorf("iter %d: AwaitSnapshot() = %v, want nil", i, err)
				}
			default:
				t.Fatalf("iter %d: AwaitSnapshot() did not return", i)
			}
		}
	})
}

// TestAwaitSnapshotReturnsErrInterruptedOnCleanCancel verifies the W2 fix:
// when a subscription is closed cleanly (err=nil) without ever emitting
// SnapshotComplete — the cancel-path scenario for every expectSnapshot flow —
// AwaitSnapshot must surface ErrInterrupted, not silently report success.
func TestAwaitSnapshotReturnsErrInterruptedOnCleanCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sub := newSubscription[int](subscriptionConfig{buffer: 1}, func() {})
		sub.expectSnapshot()

		done := make(chan error, 1)
		go func() { done <- sub.AwaitSnapshot(context.Background()) }()

		// Wait for the goroutine to enter the select in AwaitSnapshot.
		synctest.Wait()
		sub.closeWithErr(nil)

		synctest.Wait()
		select {
		case err := <-done:
			if !errors.Is(err, ErrInterrupted) {
				t.Errorf("AwaitSnapshot() = %v, want ErrInterrupted", err)
			}
		default:
			t.Fatal("AwaitSnapshot() did not return after closeWithErr")
		}
	})
}

// TestAwaitSnapshotReturnsCloseErrorOnErrorClose freezes the error path:
// when the subscription closes with a non-nil error before the snapshot
// reaches, AwaitSnapshot surfaces the underlying error rather than
// ErrInterrupted.
func TestAwaitSnapshotReturnsCloseErrorOnErrorClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sub := newSubscription[int](subscriptionConfig{buffer: 1}, func() {})
		sub.expectSnapshot()

		want := errors.New("api error 162")
		done := make(chan error, 1)
		go func() { done <- sub.AwaitSnapshot(context.Background()) }()

		synctest.Wait()
		sub.closeWithErr(want)

		synctest.Wait()
		select {
		case err := <-done:
			if !errors.Is(err, want) {
				t.Errorf("AwaitSnapshot() = %v, want %v", err, want)
			}
		default:
			t.Fatal("AwaitSnapshot() did not return after closeWithErr")
		}
	})
}

// TestAwaitSnapshotReturnsErrNoSnapshotWithoutExpectation freezes the contract
// that AwaitSnapshot on a subscription that never called expectSnapshot is an
// error: the caller asked for something the flow does not promise.
func TestAwaitSnapshotReturnsErrNoSnapshotWithoutExpectation(t *testing.T) {
	t.Parallel()

	sub := newSubscription[int](subscriptionConfig{buffer: 1}, func() {})
	// Do not call expectSnapshot.

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := sub.AwaitSnapshot(ctx); !errors.Is(err, ErrNoSnapshot) {
		t.Errorf("AwaitSnapshot() = %v, want ErrNoSnapshot", err)
	}
}

// TestAwaitSnapshotReturnsContextError freezes ctx cancellation propagation.
func TestAwaitSnapshotReturnsContextError(t *testing.T) {
	t.Parallel()

	sub := newSubscription[int](subscriptionConfig{buffer: 1}, func() {})
	sub.expectSnapshot()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sub.AwaitSnapshot(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("AwaitSnapshot() = %v, want context.Canceled", err)
	}
}

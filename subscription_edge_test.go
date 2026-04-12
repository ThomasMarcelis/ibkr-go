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

	sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerClose}, func() {})
	sub.closeWithErr(nil)

	if sub.emit(42) {
		t.Error("emit after close returned true, want false")
	}
}

func TestSubscriptionDoubleClose(t *testing.T) {
	t.Parallel()

	t.Run("Close twice", func(t *testing.T) {
		t.Parallel()
		sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerClose}, func() {
			// no-op cancel
		})
		sub.closeWithErr(nil) // ensure done channel is closed so Close doesn't block

		if err := sub.Close(); err != nil {
			t.Errorf("first Close() = %v, want nil", err)
		}
		if err := sub.Close(); err != nil {
			t.Errorf("second Close() = %v, want nil", err)
		}
	})

	t.Run("closeWithErr twice", func(t *testing.T) {
		t.Parallel()
		sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerClose}, func() {})
		sub.closeWithErr(nil)
		sub.closeWithErr(errors.New("second")) // must not panic
	})
}

func TestSubscriptionSlowConsumerClose(t *testing.T) {
	t.Parallel()

	sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerClose}, func() {})

	if !sub.emit(1) {
		t.Fatal("first emit returned false, want true")
	}
	if sub.emit(2) {
		t.Error("second emit (buffer full) returned true, want false")
	}

	err := sub.Wait()
	if !errors.Is(err, ErrSlowConsumer) {
		t.Errorf("Wait() = %v, want ErrSlowConsumer", err)
	}
}

func TestSubscriptionSlowConsumerDropOldest(t *testing.T) {
	t.Parallel()

	sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerDropOldest}, func() {})

	if !sub.emit(1) {
		t.Fatal("first emit returned false, want true")
	}
	if !sub.emit(2) {
		t.Fatal("second emit (drop oldest) returned false, want true")
	}

	select {
	case v := <-sub.Events():
		if v != 2 {
			t.Errorf("Events() received %d, want 2 (oldest should have been dropped)", v)
		}
	default:
		t.Error("Events() channel empty, expected value 2")
	}
}

func TestSubscriptionWaitBlocksUntilClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerClose}, func() {})

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

	sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerClose}, func() {})
	want := errors.New("test error")
	sub.closeWithErr(want)

	if got := sub.Wait(); !errors.Is(got, want) {
		t.Errorf("Wait() = %v, want %v", got, want)
	}
}

func TestSubscriptionStateEventDelivery(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerClose}, func() {})

		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionSnapshotComplete})

		evt := <-sub.Lifecycle()
		if evt.Kind != SubscriptionSnapshotComplete {
			t.Errorf("Lifecycle() event Kind = %q, want %q", evt.Kind, SubscriptionSnapshotComplete)
		}
		if evt.At.IsZero() {
			t.Error("Lifecycle() event At is zero, expected auto-filled timestamp")
		}
	})
}

func TestSubscriptionStateRetryableClassification(t *testing.T) {
	t.Parallel()

	apiErr := &APIError{OpKind: OpRealTimeBars, Code: 420, Message: "Invalid Real-time Query"}
	tests := []struct {
		name      string
		evt       SubscriptionStateEvent
		retryable bool
	}{
		{
			name:      "gap",
			evt:       SubscriptionStateEvent{Kind: SubscriptionGap},
			retryable: true,
		},
		{
			name:      "closed interrupted",
			evt:       SubscriptionStateEvent{Kind: SubscriptionClosed, Err: ErrInterrupted},
			retryable: true,
		},
		{
			name:      "closed resume required",
			evt:       SubscriptionStateEvent{Kind: SubscriptionClosed, Err: ErrResumeRequired},
			retryable: true,
		},
		{
			name:      "closed api error",
			evt:       SubscriptionStateEvent{Kind: SubscriptionClosed, Err: apiErr},
			retryable: false,
		},
		{
			name:      "closed slow consumer",
			evt:       SubscriptionStateEvent{Kind: SubscriptionClosed, Err: ErrSlowConsumer},
			retryable: false,
		},
		{
			name:      "closed clean",
			evt:       SubscriptionStateEvent{Kind: SubscriptionClosed},
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerClose}, func() {})
			sub.emitState(tt.evt)
			got := <-sub.Lifecycle()
			if got.Retryable != tt.retryable {
				t.Fatalf("Retryable = %v, want %v", got.Retryable, tt.retryable)
			}
			if retryable := IsRetryable(tt.evt.Err); retryable != tt.retryable && tt.evt.Kind == SubscriptionClosed {
				t.Fatalf("IsRetryable(%v) = %v, want %v", tt.evt.Err, retryable, tt.retryable)
			}
		})
	}
}

func TestSubscriptionStateChannelFull(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerClose}, func() {})

		for i := 0; i < 12; i++ {
			sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: uint64(i + 1)})
		}
		sub.closeWithErr(nil)

		var seqs []uint64
		for evt := range sub.Lifecycle() {
			if evt.Kind == SubscriptionStarted {
				seqs = append(seqs, evt.ConnectionSeq)
			}
		}
		if len(seqs) != 7 {
			t.Fatalf("read %d state events, want 7 (one buffered state replaced by Closed)", len(seqs))
		}
		for i, seq := range seqs {
			want := uint64(i + 6)
			if seq != want {
				t.Fatalf("seqs[%d] = %d, want %d (keep latest 7 before Closed)", i, seq, want)
			}
		}
	})
}

func TestSubscriptionClosedEventSurvivesFullStateBuffer(t *testing.T) {
	t.Parallel()

	sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerClose}, func() {})

	for i := 0; i < 8; i++ {
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted})
	}
	sub.closeWithErr(ErrSlowConsumer)

	seenClosed := false
	for evt := range sub.Lifecycle() {
		if evt.Kind == SubscriptionClosed {
			seenClosed = true
			if !errors.Is(evt.Err, ErrSlowConsumer) {
				t.Fatalf("closed event err = %v, want ErrSlowConsumer", evt.Err)
			}
		}
	}
	if !seenClosed {
		t.Fatal("SubscriptionClosed event was dropped when state buffer was full")
	}
}

func TestSubscriptionSnapshotCompleteFlag(t *testing.T) {
	t.Parallel()

	sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerClose}, func() {})

	if sub.snapshotComplete() {
		t.Error("snapshotComplete() = true before any state event, want false")
	}

	sub.emitState(SubscriptionStateEvent{Kind: SubscriptionSnapshotComplete})
	if !sub.snapshotComplete() {
		t.Error("snapshotComplete() = false after SnapshotComplete, want true")
	}

	// Latched: additional events do not clear it.
	sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted})
	if !sub.snapshotComplete() {
		t.Error("snapshotComplete() = false after subsequent event, want true (latched)")
	}
}

func TestSubscriptionDoneClosedOnClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerClose}, func() {})

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

	sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerClose}, func() {})
	sub.closeWithErr(nil)

	v, ok := <-sub.Events()
	if ok {
		t.Errorf("Events() receive ok = true, want false (closed channel)")
	}
	if v != 0 {
		t.Errorf("Events() zero-value = %d, want 0", v)
	}
}

func TestSubscriptionCancelFnCalledOnce(t *testing.T) {
	t.Parallel()

	var count atomic.Int32
	sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerClose}, func() {
		count.Add(1)
	})

	// Close the subscription so Wait doesn't block, then call Close multiple times.
	sub.closeWithErr(nil)

	for i := 0; i < 3; i++ {
		_ = sub.Close()
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

	sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerClose}, func() {})
	sub.expectSnapshot()
	sub.emitState(SubscriptionStateEvent{Kind: SubscriptionSnapshotComplete})

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
			sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerClose}, func() {})
			sub.expectSnapshot()

			done := make(chan error, 1)
			go func() { done <- sub.AwaitSnapshot(context.Background()) }()

			sub.emitState(SubscriptionStateEvent{Kind: SubscriptionSnapshotComplete})
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
		sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerClose}, func() {})
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
		sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerClose}, func() {})
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

	sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerClose}, func() {})
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

	sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerClose}, func() {})
	sub.expectSnapshot()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sub.AwaitSnapshot(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("AwaitSnapshot() = %v, want context.Canceled", err)
	}
}

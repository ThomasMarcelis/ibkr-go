package session

import (
	"errors"
	"sync/atomic"
	"testing"
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
	t.Parallel()

	sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerClose}, func() {})

	done := make(chan error, 1)
	go func() {
		done <- sub.Wait()
	}()

	select {
	case <-done:
		t.Fatal("Wait() returned before close")
	case <-time.After(100 * time.Millisecond):
		// expected: still blocking
	}

	sub.closeWithErr(nil)

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Wait() = %v, want nil", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Wait() did not return after close")
	}
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
	t.Parallel()

	sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerClose}, func() {})

	sub.emitState(SubscriptionStateEvent{Kind: SubscriptionSnapshotComplete})

	select {
	case evt := <-sub.State():
		if evt.Kind != SubscriptionSnapshotComplete {
			t.Errorf("State() event Kind = %q, want %q", evt.Kind, SubscriptionSnapshotComplete)
		}
		if evt.At.IsZero() {
			t.Error("State() event At is zero, expected auto-filled timestamp")
		}
	case <-time.After(time.Second):
		t.Fatal("State() channel timed out")
	}
}

func TestSubscriptionStateChannelFull(t *testing.T) {
	t.Parallel()

	sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerClose}, func() {})

	// State channel has buffer 8. Emit 12 events.
	for i := 0; i < 12; i++ {
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted})
	}

	// Read up to 8 buffered events.
	count := 0
	for {
		select {
		case <-sub.State():
			count++
		default:
			goto done
		}
	}
done:
	if count != 8 {
		t.Errorf("read %d state events, want 8 (buffer capacity)", count)
	}
}

func TestSubscriptionClosedEventSurvivesFullStateBuffer(t *testing.T) {
	t.Parallel()

	sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerClose}, func() {})

	for i := 0; i < 8; i++ {
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted})
	}
	sub.closeWithErr(ErrSlowConsumer)

	seenClosed := false
	for evt := range sub.State() {
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
	t.Parallel()

	sub := newSubscription[int](subscriptionConfig{buffer: 1, slowConsumer: SlowConsumerClose}, func() {})

	select {
	case <-sub.Done():
		t.Fatal("Done() closed before subscription closed")
	default:
		// expected: not yet closed
	}

	sub.closeWithErr(nil)

	select {
	case <-sub.Done():
		// expected: closed
	case <-time.After(time.Second):
		t.Fatal("Done() not closed after closeWithErr")
	}
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

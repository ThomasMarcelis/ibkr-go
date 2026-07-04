package ibkr

import (
	"context"
	"testing"
)

func TestSubscriptionCloseWaitsForCloseWithErr(t *testing.T) {
	t.Parallel()

	var sub *Subscription[int]
	sub = newSubscription[int](defaultSubscriptionConfig(defaultConfig()), func() {
		sub.closeWithErr(nil)
	})

	if err := sub.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := sub.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
}

func TestAllYieldsEventsUntilClose(t *testing.T) {
	t.Parallel()

	sub := newSubscription[int](defaultSubscriptionConfig(defaultConfig()), nil)
	for i := 1; i <= 3; i++ {
		if !sub.emit(i) {
			t.Fatalf("emit(%d) = false", i)
		}
	}
	sub.closeWithErr(nil)

	var got []int
	for v := range sub.All(context.Background()) {
		got = append(got, v)
	}
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("All() yielded %v, want [1 2 3]", got)
	}
	if err := sub.Err(); err != nil {
		t.Fatalf("Err() after exhaustion = %v, want nil", err)
	}
}

func TestAllDrainsBufferedEventsThenReportsTerminalError(t *testing.T) {
	t.Parallel()

	sub := newSubscription[int](defaultSubscriptionConfig(defaultConfig()), nil)
	if !sub.emit(7) {
		t.Fatal("emit(7) = false")
	}
	sub.closeWithErr(ErrSlowConsumer)

	var got []int
	for v := range sub.All(context.Background()) {
		got = append(got, v)
	}
	if len(got) != 1 || got[0] != 7 {
		t.Fatalf("All() yielded %v, want [7]", got)
	}
	if err := sub.Err(); err != ErrSlowConsumer {
		t.Fatalf("Err() = %v, want ErrSlowConsumer", err)
	}
}

func TestAllStopsOnContextCancel(t *testing.T) {
	t.Parallel()

	sub := newSubscription[int](defaultSubscriptionConfig(defaultConfig()), nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	for range sub.All(ctx) {
		t.Fatal("All() yielded after ctx cancel with empty buffer")
	}
}

func TestAllEarlyBreakLeavesSubscriptionUsable(t *testing.T) {
	t.Parallel()

	cfg := defaultSubscriptionConfig(defaultConfig())
	cfg.buffer = 4
	sub := newSubscription[int](cfg, nil)
	for i := 1; i <= 3; i++ {
		if !sub.emit(i) {
			t.Fatalf("emit(%d) = false", i)
		}
	}

	for v := range sub.All(context.Background()) {
		if v == 1 {
			break
		}
	}
	// The remaining buffered events stay consumable via Events().
	if v := <-sub.Events(); v != 2 {
		t.Fatalf("Events() after break = %d, want 2", v)
	}
}

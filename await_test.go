package ibkr

import (
	"context"
	"errors"
	"testing"
	"time"
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
	t.Parallel()

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
	case <-time.After(time.Second):
		t.Fatal("cancel callback did not run")
	}
}

func TestAwaitSubscriptionResponseRollsBackLateResult(t *testing.T) {
	t.Parallel()

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

	select {
	case got := <-rolledBack:
		if got != 42 {
			t.Fatalf("rollback value = %d, want 42", got)
		}
	case <-time.After(time.Second):
		t.Fatal("rollback did not run for late subscription result")
	}
}

func TestAwaitSubscriptionResponseReturnsContextErrorWhenAlreadyCanceled(t *testing.T) {
	t.Parallel()

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

	select {
	case got := <-rolledBack:
		if got != 42 {
			t.Fatalf("rollback value = %d, want 42", got)
		}
	case <-time.After(time.Second):
		t.Fatal("rollback did not run for already-canceled response")
	}
}

func TestEnqueueOneShotSetupRunsActiveContext(t *testing.T) {
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

	fn := <-e.cmds
	fn()

	if !called {
		t.Fatal("enqueueOneShotSetup did not execute active work")
	}
}

package ibkr

import (
	"context"
	"time"
)

func enqueueContextSetup(ctx context.Context, e *engine, onCanceled func(), fn func()) {
	e.enqueue(func() {
		if ctx.Err() != nil {
			if onCanceled != nil {
				onCanceled()
			}
			return
		}
		fn()
	})
}

func enqueueReadySetup(ctx context.Context, e *engine, onCanceled func(), fn func()) {
	enqueueContextSetup(ctx, e, onCanceled, func() {
		if e.isReady() {
			fn()
			return
		}
		time.AfterFunc(reconnectBackoff, func() {
			enqueueReadySetup(ctx, e, onCanceled, fn)
		})
	})
}

func enqueueHistoricalSetup(ctx context.Context, e *engine, key string, onCanceled func(), fn func()) {
	enqueueReadySetup(ctx, e, onCanceled, func() {
		now := time.Now()
		if e.recentHistoricalRequests == nil {
			e.recentHistoricalRequests = make(map[string]time.Time)
		}
		e.pruneHistoricalRequests(now)
		readyAt := e.nextHistoricalRequest
		if last, ok := e.recentHistoricalRequests[key]; ok {
			if identicalReadyAt := last.Add(historicalIdenticalSpacing); identicalReadyAt.After(readyAt) {
				readyAt = identicalReadyAt
			}
		}
		if wait := readyAt.Sub(now); wait > 0 {
			time.AfterFunc(wait, func() {
				enqueueHistoricalSetup(ctx, e, key, onCanceled, fn)
			})
			return
		}
		e.nextHistoricalRequest = now.Add(historicalRequestSpacing)
		e.recentHistoricalRequests[key] = now
		fn()
	})
}

func (e *engine) pruneHistoricalRequests(now time.Time) {
	cutoff := now.Add(-historicalIdenticalSpacing)
	for key, at := range e.recentHistoricalRequests {
		if at.Before(cutoff) {
			delete(e.recentHistoricalRequests, key)
		}
	}
}

// enqueueOneShotSetup drops one-shot setup work when the caller context has
// already been canceled before the actor gets to it.
func enqueueOneShotSetup(ctx context.Context, e *engine, fn func()) {
	enqueueReadySetup(ctx, e, nil, fn)
}

func enqueueSubscriptionSetup[T any](ctx context.Context, e *engine, resp chan<- T, fn func()) {
	enqueueReadySetup(ctx, e, func() {
		var zero T
		resp <- zero
	}, fn)
}

func awaitOneShotResponse[T any](ctx context.Context, e *engine, resp <-chan T, cancel func()) (T, error) {
	var zero T

	select {
	case out := <-resp:
		return out, nil
	case <-ctx.Done():
		if cancel != nil {
			cancel()
		}
		return zero, ctx.Err()
	case <-e.done:
		return zero, e.Wait()
	}
}

func awaitSubscriptionResponse[T any](ctx context.Context, e *engine, resp <-chan T, rollback func(T)) (T, error) {
	var zero T

	if err := ctx.Err(); err != nil {
		rollbackSubscriptionResponse(e, resp, rollback)
		return zero, err
	}

	select {
	case out := <-resp:
		if err := ctx.Err(); err != nil {
			if rollback != nil {
				rollback(out)
			}
			return zero, err
		}
		return out, nil
	case <-ctx.Done():
		rollbackSubscriptionResponse(e, resp, rollback)
		return zero, ctx.Err()
	case <-e.done:
		return zero, e.Wait()
	}
}

// awaitFireAndForget sends a fire-and-forget command through the actor and
// waits for its result, respecting context cancellation and engine shutdown.
func awaitFireAndForget(ctx context.Context, e *engine, fn func(context.Context) error) error {
	resp := make(chan error, 1)
	enqueueReadySetup(ctx, e, func() {
		resp <- ctx.Err()
	}, func() {
		resp <- fn(ctx)
	})
	select {
	case err := <-resp:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-e.done:
		return ErrClosed
	}
}

func rollbackSubscriptionResponse[T any](e *engine, resp <-chan T, rollback func(T)) {
	if rollback == nil {
		return
	}
	go func() {
		select {
		case out := <-resp:
			rollback(out)
		case <-e.done:
		}
	}()
}

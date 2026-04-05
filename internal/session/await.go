package session

import "context"

func enqueueContextSetup(ctx context.Context, e *Engine, onCanceled func(), fn func()) {
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

// enqueueOneShotSetup drops one-shot setup work when the caller context has
// already been canceled before the actor gets to it.
func enqueueOneShotSetup(ctx context.Context, e *Engine, fn func()) {
	enqueueContextSetup(ctx, e, nil, fn)
}

func enqueueSubscriptionSetup[T any](ctx context.Context, e *Engine, resp chan<- T, fn func()) {
	enqueueContextSetup(ctx, e, func() {
		var zero T
		resp <- zero
	}, fn)
}

func awaitOneShotResponse[T any](ctx context.Context, e *Engine, resp <-chan T, cancel func()) (T, error) {
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

func awaitSubscriptionResponse[T any](ctx context.Context, e *Engine, resp <-chan T, rollback func(T)) (T, error) {
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

func rollbackSubscriptionResponse[T any](e *Engine, resp <-chan T, rollback func(T)) {
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

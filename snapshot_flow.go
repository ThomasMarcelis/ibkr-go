package ibkr

import (
	"context"
	"errors"
)

// A one-shot already promises to return every row, so it retains rows inside
// the subscription until the server's snapshot boundary instead of applying a
// live stream's bounded slow-consumer policy before the caller can start.
func withSnapshotCollector() SubscriptionOption {
	return func(cfg *subscriptionConfig) {
		cfg.collectSnapshot = true
	}
}

func bindContext[T any](ctx context.Context, sub *Subscription[T]) {
	go func() {
		select {
		case <-ctx.Done():
			sub.cancel(context.Cause(ctx), sub.cancelFn)
		case <-sub.Done():
		}
	}()
}

func collectSnapshot[T any, U any](ctx context.Context, sub *Subscription[T], mapFn func(T) (U, bool)) ([]U, error) {
	select {
	case <-sub.snapshotDone:
		return mapSnapshotEvents(sub.takeSnapshotEvents(), mapFn), nil
	case <-sub.done:
		values := mapSnapshotEvents(sub.takeSnapshotEvents(), mapFn)
		if sub.snapshotComplete() {
			return values, nil
		}
		return values, sub.Wait()
	case <-ctx.Done():
		return nil, context.Cause(ctx)
	}
}

// collectSnapshotAndClose keeps one-shot snapshot cleanup inside the result
// boundary. Rows already delivered by the Gateway remain available even when
// cancellation cannot enter the active transport queue.
func collectSnapshotAndClose[T any, U any](ctx context.Context, sub *Subscription[T], mapFn func(T) (U, bool)) ([]U, error) {
	values, collectErr := collectSnapshot(ctx, sub, mapFn)
	sub.Close()
	closeErr := sub.Wait()
	_, cancellationUncertain := errors.AsType[*SubscriptionCancelError](closeErr)
	if collectErr == nil {
		// The snapshot boundary is authoritative for the query result. A
		// transport/session close racing later cleanup destroys the remote
		// stream and does not invalidate collected rows; only cancellation
		// uncertainty on the still-active connection crosses this boundary.
		if cancellationUncertain {
			return values, closeErr
		}
		return values, nil
	}
	if !cancellationUncertain || errors.Is(collectErr, closeErr) {
		return values, collectErr
	}
	return values, errors.Join(collectErr, closeErr)
}

func mapSnapshotEvents[T any, U any](events []T, mapFn func(T) (U, bool)) []U {
	values := make([]U, 0, len(events))
	for _, event := range events {
		if value, ok := mapFn(event); ok {
			values = append(values, value)
		}
	}
	return values
}

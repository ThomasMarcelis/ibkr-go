package session

import "context"

func bindContext[T any](ctx context.Context, sub *Subscription[T]) {
	go func() {
		select {
		case <-ctx.Done():
			_ = sub.Close()
		case <-sub.Done():
		}
	}()
}

func collectSnapshot[T any, U any](ctx context.Context, sub *Subscription[T], mapFn func(T) U) ([]U, error) {
	values := make([]U, 0, 8)
	for {
		select {
		case item, ok := <-sub.Events():
			if !ok {
				if sub.snapshotComplete() {
					return values, nil
				}
				return values, sub.Wait()
			}
			values = append(values, mapFn(item))
		case state, ok := <-sub.State():
			if !ok {
				if sub.snapshotComplete() {
					return drainSnapshotEvents(values, sub, mapFn), nil
				}
				return values, sub.Wait()
			}
			switch state.Kind {
			case SubscriptionSnapshotComplete:
				return drainSnapshotEvents(values, sub, mapFn), nil
			case SubscriptionClosed:
				if sub.snapshotComplete() {
					return drainSnapshotEvents(values, sub, mapFn), nil
				}
				return values, state.Err
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func drainSnapshotEvents[T any, U any](values []U, sub *Subscription[T], mapFn func(T) U) []U {
	for {
		select {
		case item, ok := <-sub.Events():
			if !ok {
				return values
			}
			values = append(values, mapFn(item))
		default:
			return values
		}
	}
}

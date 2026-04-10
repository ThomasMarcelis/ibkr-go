package session

import "sync"

// observer is a bounded observational channel that favors the latest event by
// evicting the oldest queued item when full.
type observer[T any] struct {
	ch chan T

	mu     sync.Mutex
	closed bool
}

func newObserver[T any](buffer int) *observer[T] {
	if buffer < 1 {
		buffer = 1
	}
	return &observer[T]{
		ch: make(chan T, buffer),
	}
}

func (o *observer[T]) Chan() <-chan T {
	return o.ch
}

func (o *observer[T]) EmitLatest(value T) bool {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.closed {
		return false
	}
	select {
	case o.ch <- value:
		return true
	default:
	}

	select {
	case <-o.ch:
	default:
	}
	select {
	case o.ch <- value:
		return true
	default:
		return false
	}
}

func (o *observer[T]) Close() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.closed {
		return
	}
	o.closed = true
	close(o.ch)
}

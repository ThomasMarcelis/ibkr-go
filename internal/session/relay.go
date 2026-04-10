package session

import "sync"

// relay serializes delivery onto a public channel without forcing producers to
// coordinate on the consumer's read pace. It preserves event order and closes
// the public channel only after queued values have been drained.
type relay[T any] struct {
	out chan T

	mu     sync.Mutex
	cond   *sync.Cond
	queue  []T
	closed bool
}

func newRelay[T any](buffer int) *relay[T] {
	if buffer < 0 {
		buffer = 0
	}
	r := &relay[T]{
		out: make(chan T, buffer),
	}
	r.cond = sync.NewCond(&r.mu)
	go r.run()
	return r
}

func (r *relay[T]) Chan() <-chan T {
	return r.out
}

func (r *relay[T]) Emit(value T) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return false
	}
	r.queue = append(r.queue, value)
	r.cond.Signal()
	return true
}

func (r *relay[T]) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.closed = true
	r.cond.Broadcast()
}

func (r *relay[T]) run() {
	for {
		r.mu.Lock()
		for len(r.queue) == 0 && !r.closed {
			r.cond.Wait()
		}
		if len(r.queue) == 0 && r.closed {
			r.mu.Unlock()
			close(r.out)
			return
		}
		value := r.queue[0]
		var zero T
		r.queue[0] = zero
		r.queue = r.queue[1:]
		r.mu.Unlock()

		r.out <- value
	}
}

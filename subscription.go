package ibkr

import (
	"context"
	"iter"
	"sync"
	"time"
)

// Subscription is a live stream of typed events from the Gateway. Business
// events arrive on [Subscription.Events]; lifecycle state changes (gaps,
// resumes, close) arrive on [Subscription.Lifecycle]. Call [Subscription.Close]
// to cancel the server-side subscription and drain channels.
// [Subscription.AwaitSnapshot] blocks until the initial snapshot boundary,
// [Subscription.Wait] blocks until the subscription closes, and
// [Subscription.Err] returns the terminal error without blocking.
type Subscription[T any] struct {
	events         chan T
	state          *observer[SubscriptionStateEvent]
	done           chan struct{}
	cancelFn       func()
	cancelOnce     sync.Once
	closeOnce      sync.Once
	errMu          sync.Mutex
	err            error
	snapshotMu     sync.Mutex
	snapshotClosed bool
	snapshotWant   bool
	snapshotDone   chan struct{}
	snapshotOnce   sync.Once
	cfg            subscriptionConfig
}

func newSubscription[T any](cfg subscriptionConfig, cancelFn func()) *Subscription[T] {
	if cfg.buffer <= 0 {
		cfg.buffer = 1
	}
	return &Subscription[T]{
		events:       make(chan T, cfg.buffer),
		state:        newObserver[SubscriptionStateEvent](8),
		done:         make(chan struct{}),
		snapshotDone: make(chan struct{}),
		cancelFn:     cancelFn,
		cfg:          cfg,
	}
}

// Events returns the channel of business events. It closes when the
// subscription closes; after ranging it to exhaustion, call [Subscription.Err]
// for the terminal error. See also [Subscription.All] for a range-friendly
// iterator.
func (s *Subscription[T]) Events() <-chan T { return s.events }

// All returns an iterator over the subscription's events for use with a
// range statement. It yields until the subscription closes or ctx is
// canceled. Iterating to exhaustion drains every buffered event, so after
// the loop [Subscription.Err] reports the terminal error: nil for a clean
// close, or e.g. [ErrSlowConsumer] / [ErrInterrupted] otherwise. Callers
// that break early or rely on ctx cancellation should also check ctx.Err.
//
//	for q := range sub.All(ctx) {
//		fmt.Println(q.Last)
//	}
//	if err := sub.Err(); err != nil {
//		log.Fatal(err)
//	}
//
// Lifecycle transitions (gap, resume, snapshot boundaries) are not part of
// the iteration; consumers that need them use [Subscription.Lifecycle].
func (s *Subscription[T]) All(ctx context.Context) iter.Seq[T] {
	return func(yield func(T) bool) {
		for {
			select {
			case ev, ok := <-s.events:
				if !ok {
					return
				}
				if !yield(ev) {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}
}

// Lifecycle returns the channel of lifecycle transitions (started, snapshot
// complete, gap, resume, close), distinct from the business events on Events.
func (s *Subscription[T]) Lifecycle() <-chan SubscriptionStateEvent { return s.state.Chan() }

// Done returns a channel closed when the subscription has terminated. After it
// is closed, [Subscription.Wait] and [Subscription.Err] report the terminal error.
func (s *Subscription[T]) Done() <-chan struct{} { return s.done }

// AwaitSnapshot blocks until the subscription's initial snapshot boundary is
// reached, then returns nil. It returns [ErrNoSnapshot] for streams that have
// no snapshot phase, the terminal error (or [ErrInterrupted]) if the
// subscription closes first, or ctx.Err if ctx is canceled.
func (s *Subscription[T]) AwaitSnapshot(ctx context.Context) error {
	s.snapshotMu.Lock()
	if s.snapshotClosed {
		s.snapshotMu.Unlock()
		return nil
	}
	if !s.snapshotWant {
		s.snapshotMu.Unlock()
		return ErrNoSnapshot
	}
	done := s.snapshotDone
	s.snapshotMu.Unlock()

	select {
	case <-done:
		return nil
	case <-s.done:
		// The subscription closed. If SnapshotComplete was emitted just
		// before close (the normal success path), snapshotClosed is true and
		// we return nil. Otherwise the cancel/error path tore the
		// subscription down without reaching a snapshot boundary, so report
		// the underlying close error — or ErrInterrupted when the close was
		// clean (e.g. the caller invoked Close before snapshot complete).
		s.snapshotMu.Lock()
		closed := s.snapshotClosed
		s.snapshotMu.Unlock()
		if closed {
			return nil
		}
		if err := s.Wait(); err != nil {
			return err
		}
		return ErrInterrupted
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Wait blocks until the subscription terminates and returns its terminal
// error, or nil on a clean close.
func (s *Subscription[T]) Wait() error {
	<-s.done
	s.errMu.Lock()
	defer s.errMu.Unlock()
	return s.err
}

// Err returns the currently recorded terminal error without waiting for Done.
// It returns nil until the subscription has closed with an error.
func (s *Subscription[T]) Err() error {
	s.errMu.Lock()
	defer s.errMu.Unlock()
	return s.err
}

// Close cancels the server-side subscription and drains its channels. It is
// idempotent and safe to call concurrently.
func (s *Subscription[T]) Close() error {
	s.cancelOnce.Do(func() {
		if s.cancelFn != nil {
			s.cancelFn()
		}
	})
	return nil
}

func (s *Subscription[T]) emit(value T) bool {
	select {
	case <-s.done:
		return false
	default:
	}

	select {
	case s.events <- value:
		return true
	default:
	}

	switch s.cfg.slowConsumer {
	case SlowConsumerDropOldest:
		select {
		case <-s.events:
		default:
		}
		select {
		case s.events <- value:
			return true
		default:
			s.fail(ErrSlowConsumer)
			return false
		}
	default:
		s.fail(ErrSlowConsumer)
		return false
	}
}

func (s *Subscription[T]) emitState(evt SubscriptionStateEvent) {
	select {
	case <-s.done:
		return
	default:
	}
	evt.Retryable = retryableSubscriptionState(evt)
	if evt.At.IsZero() {
		evt.At = time.Now().UTC()
	}
	if evt.Kind == SubscriptionSnapshotComplete {
		s.snapshotMu.Lock()
		s.snapshotClosed = true
		s.snapshotMu.Unlock()
		s.snapshotOnce.Do(func() { close(s.snapshotDone) })
	}
	s.state.EmitLatest(evt)
}

func (s *Subscription[T]) fail(err error) {
	s.closeWithErr(err)
}

func (s *Subscription[T]) snapshotComplete() bool {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	return s.snapshotClosed
}

func (s *Subscription[T]) expectSnapshot() {
	s.snapshotMu.Lock()
	s.snapshotWant = true
	s.snapshotMu.Unlock()
}

func (s *Subscription[T]) closeWithErr(err error) {
	s.closeOnce.Do(func() {
		s.errMu.Lock()
		s.err = err
		s.errMu.Unlock()
		s.emitState(SubscriptionStateEvent{Kind: SubscriptionClosed, Err: err})
		// Close events before done so Done reports completion only after the
		// engine has stopped publishing business events. Consumers that need
		// every buffered event should range Events(), then call Wait().
		close(s.events)
		s.state.Close()
		close(s.done)
	})
}

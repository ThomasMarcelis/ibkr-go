package ibkr

import (
	"context"
	"errors"
	"iter"
	"sync"
)

// Subscription is a live stream of ordered data and lifecycle events from the
// Gateway. Call [Subscription.Close] to initiate server-side cancellation,
// then [Subscription.Wait] when shutdown completion matters.
// [Subscription.AwaitSnapshot] blocks until the initial snapshot boundary,
// [Subscription.Wait] blocks until the subscription closes, and
// [Subscription.Err] returns the terminal error without blocking.
type Subscription[T any] struct {
	events         chan StreamEvent[T]
	done           chan struct{}
	cancelFn       func()
	actorCancelFn  func()
	cancelOnce     sync.Once
	closeOnce      sync.Once
	errMu          sync.Mutex
	err            error
	cancelCause    error
	snapshotMu     sync.Mutex
	snapshotClosed bool
	snapshotWant   bool
	snapshotDone   chan struct{}
	snapshotOnce   sync.Once
	snapshotEvents []T
	cfg            subscriptionConfig
	connectionSeq  uint64
}

func newSubscription[T any](cfg subscriptionConfig, cancelFn func()) *Subscription[T] {
	if cfg.buffer <= 0 {
		cfg.buffer = 1
	}
	return &Subscription[T]{
		events:       make(chan StreamEvent[T], cfg.buffer),
		done:         make(chan struct{}),
		snapshotDone: make(chan struct{}),
		cancelFn:     cancelFn,
		cfg:          cfg,
	}
}

// Events returns the single ordered stream of data and lifecycle events. It
// closes when the subscription terminates; after ranging it to exhaustion,
// call [Subscription.Err] for the terminal error.
func (s *Subscription[T]) Events() <-chan StreamEvent[T] { return s.events }

// All returns an iterator over the subscription's data values for use with a
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
// Lifecycle transitions are filtered out. Events and All consume the same
// queue, so use one or the other rather than reading them concurrently.
func (s *Subscription[T]) All(ctx context.Context) iter.Seq[T] {
	return func(yield func(T) bool) {
		for {
			select {
			case event, ok := <-s.events:
				if !ok {
					return
				}
				if event.Kind != StreamData {
					continue
				}
				if !yield(event.Value) {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}
}

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
		return context.Cause(ctx)
	}
}

// Wait blocks until the subscription terminates and returns its terminal
// error, or nil on a clean close. If slow-consumer shutdown cannot admit its
// cancellation request, the error matches [ErrSlowConsumer] and contains a
// [*SubscriptionCancelError].
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

// Close initiates cancellation of the server-side subscription. It is
// idempotent and safe to call concurrently. Events closes asynchronously; use
// [Subscription.Done] or [Subscription.Wait] to
// observe completion. If cancellation cannot enter the active transport queue,
// Wait returns a non-retryable [*SubscriptionCancelError]. When cancellation
// follows [ErrSlowConsumer], Wait preserves both causes in the terminal error.
func (s *Subscription[T]) Close() {
	s.cancel(nil, s.cancelFn)
}

// cancelOnce serializes public cancellation initiation. The shutdown cause is
// independent so an in-flight actor emit can still record local data loss after
// public Close has started but before the actor terminally closes the route.
func (s *Subscription[T]) cancel(cause error, cancelFn func()) {
	s.recordCancelCause(cause)
	s.cancelOnce.Do(func() {
		if cancelFn != nil {
			cancelFn()
			return
		}
		s.closeWithErr(nil)
	})
}

// cancelFromActor runs the engine-owned cancellation directly. Enqueuing from
// the actor can deadlock when the command queue is full because only that same
// actor can drain it. It deliberately does not wait on cancelOnce: a concurrent
// public Close may hold that once while blocked in enqueue. Both paths invoke
// the same route-owned callback, whose ownership check makes the later path a
// no-op. Non-engine subscriptions fall back to their ordinary cancellation
// callback.
func (s *Subscription[T]) cancelFromActor(cause error) {
	s.recordCancelCause(cause)

	cancelFn := s.actorCancelFn
	if cancelFn != nil {
		cancelFn()
		return
	}
	s.cancel(cause, s.cancelFn)
}

func (s *Subscription[T]) recordCancelCause(cause error) {
	if cause == nil {
		return
	}
	s.errMu.Lock()
	if s.cancelCause == nil {
		s.cancelCause = cause
	}
	s.errMu.Unlock()
}

func (s *Subscription[T]) emit(value T) bool {
	select {
	case <-s.done:
		return false
	default:
	}
	if s.cfg.collectSnapshot {
		s.snapshotMu.Lock()
		if !s.snapshotClosed {
			s.snapshotEvents = append(s.snapshotEvents, value)
		}
		s.snapshotMu.Unlock()
		return true
	}

	select {
	case s.events <- StreamEvent[T]{Kind: StreamData, Value: value, ConnectionSeq: s.connectionSeq}:
		return true
	default:
		s.cancelFromActor(ErrSlowConsumer)
		return false
	}
}

func (s *Subscription[T]) emitState(kind StreamEventKind, connectionSeq uint64, err error) {
	select {
	case <-s.done:
		return
	default:
	}
	if connectionSeq != 0 {
		s.connectionSeq = connectionSeq
	}
	if kind == StreamSnapshotComplete {
		s.snapshotMu.Lock()
		s.snapshotClosed = true
		s.snapshotMu.Unlock()
		s.snapshotOnce.Do(func() { close(s.snapshotDone) })
	}
	if s.cfg.collectSnapshot {
		return
	}
	select {
	case s.events <- StreamEvent[T]{Kind: kind, ConnectionSeq: connectionSeq, Err: err}:
	default:
		s.cancelFromActor(ErrSlowConsumer)
	}
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

func (s *Subscription[T]) takeSnapshotEvents() []T {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	events := s.snapshotEvents
	s.snapshotEvents = nil
	return events
}

func (s *Subscription[T]) closeWithErr(err error) {
	s.closeOnce.Do(func() {
		s.errMu.Lock()
		if s.cancelCause != nil {
			if _, ok := errors.AsType[*SubscriptionCancelError](err); ok {
				err = errors.Join(s.cancelCause, err)
			} else {
				err = s.cancelCause
			}
		}
		s.err = err
		s.errMu.Unlock()
		// Close events before done so Done reports completion only after the
		// engine has stopped publishing events. Consumers that need
		// every buffered event should range Events(), then call Wait().
		close(s.events)
		close(s.done)
	})
}

package ibkr

import (
	"context"
	"sync"
	"time"
)

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

func (s *Subscription[T]) Events() <-chan T { return s.events }

func (s *Subscription[T]) Lifecycle() <-chan SubscriptionStateEvent { return s.state.Chan() }

func (s *Subscription[T]) Done() <-chan struct{} { return s.done }

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

func (s *Subscription[T]) Wait() error {
	<-s.done
	s.errMu.Lock()
	defer s.errMu.Unlock()
	return s.err
}

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
		close(s.done)
		close(s.events)
		s.state.Close()
	})
}

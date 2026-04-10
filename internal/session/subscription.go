package session

import (
	"sync"
	"time"
)

type Subscription[T any] struct {
	events         chan T
	done           chan struct{}
	cancelFn       func()
	cancelOnce     sync.Once
	closeOnce      sync.Once
	errMu          sync.Mutex
	err            error
	stateMu        sync.Mutex
	snapshotClosed bool
	cfg            subscriptionConfig
	stateRelay     *relay[SubscriptionStateEvent]
}

func newSubscription[T any](cfg subscriptionConfig, cancelFn func()) *Subscription[T] {
	if cfg.buffer <= 0 {
		cfg.buffer = 1
	}
	return &Subscription[T]{
		events:     make(chan T, cfg.buffer),
		done:       make(chan struct{}),
		cancelFn:   cancelFn,
		cfg:        cfg,
		stateRelay: newRelay[SubscriptionStateEvent](8),
	}
}

func (s *Subscription[T]) Events() <-chan T { return s.events }

func (s *Subscription[T]) State() <-chan SubscriptionStateEvent { return s.stateRelay.Chan() }

func (s *Subscription[T]) Done() <-chan struct{} { return s.done }

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
		s.stateMu.Lock()
		s.snapshotClosed = true
		s.stateMu.Unlock()
	}
	if evt.Kind == SubscriptionClosed {
		s.stateRelay.Emit(evt)
		return
	}
	s.stateRelay.Emit(evt)
}

func (s *Subscription[T]) fail(err error) {
	s.closeWithErr(err)
}

func (s *Subscription[T]) snapshotComplete() bool {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return s.snapshotClosed
}

func (s *Subscription[T]) closeWithErr(err error) {
	s.closeOnce.Do(func() {
		s.errMu.Lock()
		s.err = err
		s.errMu.Unlock()
		s.emitState(SubscriptionStateEvent{Kind: SubscriptionClosed, Err: err})
		s.stateRelay.Close()
		close(s.done)
		close(s.events)
	})
}

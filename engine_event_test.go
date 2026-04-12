package ibkr

import (
	"testing"
	"testing/synctest"
)

func TestSessionEventsCloseEvenWhenUnread(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := &engine{
			events:      newObserver[Event](1),
			ready:       make(chan error, 1),
			done:        make(chan struct{}),
			keyed:       make(map[int]*route),
			singletons:  make(map[string]*route),
			orders:      make(map[int64]*orderRoute),
			execToOrder: make(map[string]int64),
			snapshot: Snapshot{
				State: StateConnecting,
			},
		}

		for i := 0; i < 12; i++ {
			e.setState(StateReady, i+1, "", nil)
		}

		done := make(chan struct{})
		go func() {
			e.closeEngine(nil)
			close(done)
		}()

		synctest.Wait()
		select {
		case <-done:
		default:
			t.Fatal("closeEngine() blocked with unread session events")
		}

		for range e.SessionEvents() {
		}
	})
}

func TestSessionEventsDropOldestKeepsLatest(t *testing.T) {
	t.Parallel()

	e := &engine{
		events: newObserver[Event](2),
		snapshot: Snapshot{
			State: StateReady,
		},
	}

	e.emitEvent(2104, "one")
	e.emitEvent(2106, "two")
	e.emitEvent(2158, "three")
	e.events.Close()

	var codes []int
	for evt := range e.SessionEvents() {
		codes = append(codes, evt.Code)
	}
	if len(codes) != 2 {
		t.Fatalf("codes len = %d, want 2", len(codes))
	}
	if codes[0] != 2106 || codes[1] != 2158 {
		t.Fatalf("codes = %v, want [2106 2158]", codes)
	}
}

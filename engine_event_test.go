package ibkr

import (
	"testing"
	"testing/synctest"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
)

func TestSessionEventsCloseEvenWhenUnread(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := &engine{
			events:         newObserver[Event](1),
			ready:          make(chan error, 1),
			done:           make(chan struct{}),
			keyed:          make(map[int]*route),
			singletons:     make(map[string]*route),
			orders:         make(map[int64]*orderRoute),
			execDeliveries: make(map[string]*execDelivery),
			snapshot: Snapshot{
				State: StateConnecting,
			},
		}

		for i := 0; i < 12; i++ {
			e.setState(StateReady, i+1, "", nil)
		}

		done := make(chan struct{})
		go func() {
			e.closeEngine(nil, nil, nil)
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

func TestConnectivityEventPreservesFullAPIError(t *testing.T) {
	t.Parallel()

	e := &engine{
		events: newObserver[Event](1),
		snapshot: Snapshot{
			State:         StateReady,
			ConnectionSeq: 3,
		},
	}
	e.handleAPIError(codec.APIError{
		ReqID:   0,
		Code:    1100,
		Message: "Connectivity between IB and TWS has been lost.",
	})

	event := <-e.SessionEvents()
	if event.APIError == nil {
		t.Fatal("Event.APIError = nil")
	}
	if event.APIError.RequestID != 0 || event.APIError.Code != 1100 || event.APIError.ConnectionSeq != 3 {
		t.Fatalf("Event.APIError = %+v", event.APIError)
	}
	if !event.APIError.ServerTime.IsZero() {
		t.Fatalf("ServerTime = %s, want zero when the classic capture omitted errorTime", event.APIError.ServerTime)
	}
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

func TestSessionEventCarriesAtomicStateRevision(t *testing.T) {
	t.Parallel()

	e := &engine{
		events: newObserver[Event](1),
		snapshot: Snapshot{
			State:           StateReady,
			ConnectionSeq:   4,
			ManagedAccounts: []string{"DU123"},
		},
	}

	e.setState(StateDegraded, 1100, "lost", nil)
	e.setState(StateReady, 1102, "restored", nil)
	e.emitEvent(2104, "farm ready")

	event := <-e.SessionEvents()
	if event.Code != 2104 {
		t.Fatalf("Event.Code = %d, want 2104", event.Code)
	}
	if event.TransitionSeq != 2 || event.Snapshot.TransitionSeq != 2 {
		t.Fatalf("transition sequence = %d/%d, want 2", event.TransitionSeq, event.Snapshot.TransitionSeq)
	}
	if event.State != StateReady || event.State != event.Snapshot.State || event.ConnectionSeq != event.Snapshot.ConnectionSeq {
		t.Fatalf("Event and Snapshot disagree: %+v", event)
	}
	event.Snapshot.ManagedAccounts[0] = "mutated"
	if got := e.Session().ManagedAccounts[0]; got != "DU123" {
		t.Fatalf("Session.ManagedAccounts[0] = %q after event mutation", got)
	}
}

func TestSessionTransitionSequenceIgnoresSameStateAndNotices(t *testing.T) {
	t.Parallel()

	e := &engine{events: newObserver[Event](4), snapshot: Snapshot{State: StateReady}}
	e.setState(StateReady, 0, "same", nil)
	e.emitEvent(2104, "notice")
	e.setState(StateDegraded, 1100, "lost", nil)

	for i := range 3 {
		event := <-e.SessionEvents()
		if i < 2 && event.TransitionSeq != 0 {
			t.Fatalf("event %d TransitionSeq = %d, want 0", i, event.TransitionSeq)
		}
		if i == 2 && event.TransitionSeq != 1 {
			t.Fatalf("transition event TransitionSeq = %d, want 1", event.TransitionSeq)
		}
	}
}

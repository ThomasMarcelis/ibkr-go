package session

import (
	"strconv"
	"testing"
)

func TestSessionEventRelayDoesNotDropQueuedEvents(t *testing.T) {
	t.Parallel()

	e := &Engine{
		eventRelay: newRelay[Event](1),
		snapshot: Snapshot{
			State: StateConnecting,
		},
	}

	for i := 0; i < 12; i++ {
		e.setState(StateReady, i+1, strconv.Itoa(i+1), nil)
	}
	e.eventRelay.Close()

	count := 0
	for range e.SessionEvents() {
		count++
	}
	if count != 12 {
		t.Fatalf("session events count = %d, want 12", count)
	}
}

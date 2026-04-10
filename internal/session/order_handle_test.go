package session

import "testing"

func TestOrderHandleStateRelayDoesNotDropQueuedEvents(t *testing.T) {
	t.Parallel()

	handle := newOrderHandle(7)
	for i := 0; i < 12; i++ {
		handle.emitState(SubscriptionStateEvent{Kind: SubscriptionGap})
	}
	if err := handle.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	count := 0
	for evt := range handle.State() {
		if evt.Kind == SubscriptionGap {
			count++
		}
	}
	if count != 12 {
		t.Fatalf("gap event count = %d, want 12", count)
	}
}

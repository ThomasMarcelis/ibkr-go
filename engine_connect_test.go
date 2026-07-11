package ibkr

import (
	"context"
	"testing"
)

func TestMaybeReadyRunsCompletionOnce(t *testing.T) {
	t.Parallel()

	runs := 0
	e := &engine{
		bootstrap: bootstrapState{serverInfo: true, managed: true, nextValidID: true},
		ready:     make(chan error, 1),
		events:    newObserver[Event](2),
		snapshot:  Snapshot{State: StateHandshaking},
		readySetups: []*readySetup{{
			ctx:  context.Background(),
			fn:   func() { runs++ },
			stop: func() bool { return true },
		}},
	}

	e.maybeReady()
	e.maybeReady()

	if runs != 1 {
		t.Fatalf("ready setup runs = %d, want 1", runs)
	}
	if got := e.Session().ConnectionSeq; got != 1 {
		t.Fatalf("connection sequence = %d, want 1", got)
	}
}

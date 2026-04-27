package ibkr

import (
	"errors"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
)

func TestHandleTransportLossPreservesReconnectAttempt(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	e := &engine{
		cfg:              config{reconnect: ReconnectAuto},
		cmds:             make(chan func(), 1),
		done:             done,
		events:           newObserver[Event](1),
		adapter:          sdkadapter.NewReplayAdapter(nil),
		keyed:            make(map[int]*route),
		singletons:       make(map[string]*route),
		orders:           make(map[int64]*orderRoute),
		executions:       newExecutionCorrelator(),
		execToOrder:      make(map[string]int64),
		reconnectAttempt: 2,
		snapshot: Snapshot{
			State:         StateHandshaking,
			ConnectionSeq: 1,
		},
	}
	defer close(done)

	e.handleTransportLoss(errors.New("bootstrap timeout"))

	if got := e.reconnectAttempt; got != 3 {
		t.Fatalf("reconnectAttempt = %d, want 3", got)
	}
}

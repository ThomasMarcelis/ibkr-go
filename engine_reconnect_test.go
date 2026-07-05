package ibkr

import (
	"errors"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/transport"
)

func TestHandleTransportLossPreservesReconnectAttempt(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	e := &engine{
		cfg:              config{reconnect: ReconnectAuto},
		cmds:             make(chan func(), 1),
		done:             done,
		events:           newObserver[Event](1),
		transport:        &transport.Conn{},
		keyed:            make(map[int]*route),
		singletons:       make(map[string]*route),
		orders:           make(map[int64]*orderRoute),
		executions:       newExecutionCorrelator(),
		execDeliveries:   make(map[string]*execDelivery),
		reconnectAttempt: 2,
		snapshot: Snapshot{
			State:         StateHandshaking,
			ConnectionSeq: 1,
		},
	}
	defer close(done)

	e.handleTransportLoss(transportLoss{transport: e.transport, err: errors.New("bootstrap timeout")})

	if got := e.reconnectAttempt; got != 3 {
		t.Fatalf("reconnectAttempt = %d, want 3", got)
	}
}

func TestHandleTransportLossIgnoresStaleTransport(t *testing.T) {
	t.Parallel()

	oldTransport := &transport.Conn{}
	currentTransport := &transport.Conn{}
	e := &engine{
		cfg:              config{reconnect: ReconnectAuto},
		cmds:             make(chan func(), 1),
		done:             make(chan struct{}),
		events:           newObserver[Event](1),
		transport:        currentTransport,
		keyed:            make(map[int]*route),
		singletons:       make(map[string]*route),
		orders:           make(map[int64]*orderRoute),
		executions:       newExecutionCorrelator(),
		execDeliveries:   make(map[string]*execDelivery),
		reconnectAttempt: 2,
		snapshot: Snapshot{
			State:         StateReady,
			ConnectionSeq: 1,
		},
	}
	defer close(e.done)

	e.handleTransportLoss(transportLoss{transport: oldTransport, err: errors.New("old transport closed")})

	if e.transport != currentTransport {
		t.Fatal("stale transport loss cleared the current transport")
	}
	if got := e.reconnectAttempt; got != 2 {
		t.Fatalf("reconnectAttempt = %d, want unchanged 2", got)
	}
}

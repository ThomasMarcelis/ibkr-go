package ibkr

import (
	"log/slog"
	"net"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/internal/transport"
	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

// TestAttachTransportPumpUnblocksOnEngineShutdown freezes the decode-pump
// shutdown fix. The pump sends decoded messages to e.incoming and terminal
// errors to e.transportErr. If the run loop has already exited (Close) while a
// hot feed keeps decoding, an unguarded send wedges the pump forever, leaking
// the goroutine and the connection. With the fix every send races e.done, so
// engine shutdown drains the pump. synctest fails the bubble if a goroutine is
// still alive when the test body returns, which is exactly the pre-fix leak.
func TestAttachTransportPumpUnblocksOnEngineShutdown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		serverConn, clientConn := net.Pipe()
		tr := transport.New(clientConn, nil, 0)

		e := &engine{
			incoming:      make(chan any),   // unbuffered: no run loop, so one decode wedges the pump
			transportErr:  make(chan error), // unbuffered: same for the error forwarder
			done:          make(chan struct{}),
			serverVersion: 200,
		}
		e.attachTransport(tr)

		// Deliver one decodable frame. The pump decodes it and blocks on the
		// unconsumed e.incoming, standing in for a hot feed at shutdown.
		payload, err := codec.Encode(200, codec.CurrentTime{Time: "1700000000"})
		if err != nil {
			t.Fatalf("encode CurrentTime: %v", err)
		}
		go func() { _ = wire.WriteFrame(serverConn, payload) }()

		synctest.Wait() // pump is now durably blocked on e.incoming <- msg

		// Engine shutdown: run loop gone (done closed) and transport torn down,
		// exactly as closeEngine does.
		close(e.done)
		_ = tr.Close()
		_ = serverConn.Close()

		synctest.Wait() // pump and forwarder must observe e.done and exit
	})
}

// TestRunServicesCommandsUnderIncomingFlood freezes the actor-fairness fix.
// The old run loop drained e.incoming to empty at the top of every iteration,
// so a sustained inbound feed starved control commands (subscribe/cancel/place)
// on e.cmds. A single fair select now interleaves them.
//
// The feed is made deterministic rather than timing-dependent: a keyed route
// whose handler re-injects one message for every message it handles. Because
// that runs on the actor goroutine itself, e.incoming is never observed empty,
// so pre-fix drainIncoming loops forever on the actor and the enqueued command
// is never reached. Post-fix the fair select interleaves the command in.
func TestRunServicesCommandsUnderIncomingFlood(t *testing.T) {
	t.Parallel()

	e := &engine{
		cfg:          config{logger: slog.Default()},
		cmds:         make(chan func(), 1),
		incoming:     make(chan any, 256),
		transportErr: make(chan error, 1),
		done:         make(chan struct{}),
		keyed:        make(map[int]*route),
		snapshot:     Snapshot{State: StateReady},
	}

	const feedReqID = 7
	feedStop := make(chan struct{})
	reinject := codec.TickPrice{ReqID: feedReqID}
	e.keyed[feedReqID] = &route{handle: func(_ any, e *engine) {
		// Refill on the actor goroutine so e.incoming stays non-empty across
		// drainIncoming iterations. The non-blocking send is a safety valve;
		// the just-drained slot always has room in steady state.
		select {
		case <-feedStop:
		case e.incoming <- reinject:
		default:
		}
	}}
	// Seed the self-sustaining feed before the loop starts.
	e.incoming <- reinject

	go e.run()
	defer close(e.done)
	defer close(feedStop)

	ran := make(chan struct{})
	e.enqueue(func() { close(ran) })

	select {
	case <-ran:
	case <-time.After(3 * time.Second):
		t.Fatal("enqueued command starved: run() never serviced e.cmds under a sustained inbound feed")
	}
}

// TestTerminalOrderCloseForgetsRouteAndExecMappings freezes the state-retention
// fix. After a terminal order's drain window elapses the engine closes the
// handle and must also drop e.orders[id] and every execToOrder /
// execCommissionDelivered entry the order owned; unrelated orders' mappings
// stay. Pre-fix those entries lived until the connection ended.
func TestTerminalOrderCloseForgetsRouteAndExecMappings(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := &engine{
			cfg:                     config{logger: slog.Default()},
			cmds:                    make(chan func(), 8),
			incoming:                make(chan any, 8),
			transportErr:            make(chan error, 1),
			done:                    make(chan struct{}),
			events:                  newObserver[Event](8),
			keyed:                   make(map[int]*route),
			singletons:              make(map[string]*route),
			orders:                  make(map[int64]*orderRoute),
			executions:              newExecutionCorrelator(),
			execToOrder:             make(map[string]int64),
			execCommissionDelivered: make(map[string]struct{}),
			snapshot:                Snapshot{State: StateReady},
		}
		go e.run()
		defer close(e.done)

		route := &orderRoute{orderID: 50, handle: newOrderHandle(50)}
		e.enqueue(func() {
			e.orders[50] = route
			e.execToOrder["exec-a"] = 50
			e.execToOrder["exec-b"] = 50
			e.execCommissionDelivered["exec-a"] = struct{}{}
			e.execToOrder["exec-other"] = 99
			e.scheduleTerminalOrderClose(50, route)
		})
		synctest.Wait()

		type mapState struct{ order50, execA, execB, execOther, commA bool }
		read := func() mapState {
			out := make(chan mapState, 1)
			e.enqueue(func() {
				_, o50 := e.orders[50]
				_, ea := e.execToOrder["exec-a"]
				_, eb := e.execToOrder["exec-b"]
				_, eo := e.execToOrder["exec-other"]
				_, ca := e.execCommissionDelivered["exec-a"]
				out <- mapState{o50, ea, eb, eo, ca}
			})
			return <-out
		}

		// Before the drain window elapses everything is still retained.
		if got := read(); !got.order50 || !got.execA || !got.execB {
			t.Fatalf("route/exec mappings dropped before drain window: %+v", got)
		}

		time.Sleep(orderTerminalDrainWindow + time.Millisecond)
		synctest.Wait()

		got := read()
		if got.order50 {
			t.Error("e.orders[50] retained after terminal drain window")
		}
		if got.execA || got.execB || got.commA {
			t.Errorf("order 50 exec mappings retained after terminal close: %+v", got)
		}
		if !got.execOther {
			t.Error("unrelated order's execToOrder entry was dropped")
		}
	})
}

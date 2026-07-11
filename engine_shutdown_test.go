package ibkr

import (
	"errors"
	"log/slog"
	"net"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/internal/transport"
	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

func TestClientCloseWaitsCleanlyAndInterruptsActiveWork(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cfg := defaultConfig()
		callErr := make(chan error, 1)
		sub := newSubscription[int](defaultSubscriptionConfig(cfg), func() {})
		e := &engine{
			cfg:          cfg,
			cmds:         make(chan func(), 1),
			incoming:     make(chan any),
			transportErr: make(chan transportLoss),
			ready:        make(chan error, 1),
			done:         make(chan struct{}),
			events:       newObserver[Event](cfg.eventBuffer),
			keyed: map[int]*route{
				1: {close: func(err error) { callErr <- err }},
			},
			singletons: map[string]*route{
				"active-stream": {subscription: true, close: sub.closeWithErr},
			},
			orders:         make(map[int64]*orderRoute),
			execDeliveries: make(map[string]*execDelivery),
			snapshot:       Snapshot{State: StateReady},
		}
		go e.run()

		client := &Client{engine: e}
		client.Close()
		synctest.Wait()

		if err := client.Wait(); err != nil {
			t.Fatalf("Wait() after intentional Close = %v, want nil", err)
		}
		if err := <-callErr; !errors.Is(err, ErrClosed) {
			t.Fatalf("active call close error = %v, want ErrClosed", err)
		}
		if err := sub.Wait(); !errors.Is(err, ErrClosed) {
			t.Fatalf("active subscription Wait() = %v, want ErrClosed", err)
		}
	})
}

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
			incoming:      make(chan any),           // unbuffered: no run loop, so one decode wedges the pump
			transportErr:  make(chan transportLoss), // unbuffered: same for the error forwarder
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
		transportErr: make(chan transportLoss, 1),
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

// TestTerminalOrderRemainsObservedUntilClose freezes the lossless lifecycle:
// a terminal status is a business event, not an observation boundary. The
// route and execution correlations remain until the caller explicitly closes
// the handle, allowing arbitrarily late fee reports to arrive.
func TestTerminalOrderRemainsObservedUntilClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := &engine{
			cfg:            config{logger: slog.Default()},
			cmds:           make(chan func(), 8),
			incoming:       make(chan any, 8),
			transportErr:   make(chan transportLoss, 1),
			done:           make(chan struct{}),
			events:         newObserver[Event](8),
			keyed:          make(map[int]*route),
			singletons:     make(map[string]*route),
			orders:         make(map[int64]*orderRoute),
			execDeliveries: make(map[string]*execDelivery),
			snapshot:       Snapshot{State: StateReady},
		}
		go e.run()
		defer close(e.done)

		route := &orderRoute{orderID: 50, handle: newOrderHandle(50, 64)}
		route.handle.detachFn = func() {
			e.enqueue(func() { e.closeOrderRoute(50, route, nil) })
		}
		e.enqueue(func() {
			e.orders[50] = route
			e.execDeliveries["exec-a"] = &execDelivery{orderID: 50, delivered: &codec.CommissionReport{ExecID: "exec-a"}}
			e.execDeliveries["exec-b"] = &execDelivery{orderID: 50}
			e.execDeliveries["exec-other"] = &execDelivery{orderID: 99}
			e.dispatchObservedOrderStatus(codec.OrderStatus{OrderID: 50, Status: string(OrderStatusFilled)})
		})
		synctest.Wait()

		type mapState struct{ order50, execA, execB, execOther bool }
		read := func() mapState {
			out := make(chan mapState, 1)
			e.enqueue(func() {
				_, o50 := e.orders[50]
				_, ea := e.execDeliveries["exec-a"]
				_, eb := e.execDeliveries["exec-b"]
				_, eo := e.execDeliveries["exec-other"]
				out <- mapState{o50, ea, eb, eo}
			})
			return <-out
		}

		if got := read(); !got.order50 || !got.execA || !got.execB {
			t.Fatalf("route/exec mappings dropped at terminal status: %+v", got)
		}
		select {
		case <-route.handle.Done():
			t.Fatal("terminal status closed order handle")
		default:
		}

		route.handle.Close()
		synctest.Wait()

		got := read()
		if got.order50 {
			t.Error("e.orders[50] retained after explicit close")
		}
		if got.execA || got.execB {
			t.Errorf("order 50 exec mappings retained after explicit close: %+v", got)
		}
		if !got.execOther {
			t.Error("unrelated order's delivery record was dropped")
		}
	})
}

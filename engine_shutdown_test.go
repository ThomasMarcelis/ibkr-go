package ibkr

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"net"
	"runtime"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/transport"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
)

func TestClientCloseWaitsCleanlyAndInterruptsActiveWork(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cfg := defaultConfig()
		callErr := make(chan error, 1)
		sub := newSubscription[int](defaultSubscriptionConfig(cfg), func() {})
		e := &engine{
			cfg:          cfg,
			cmds:         make(chan func(), 1),
			incoming:     make(chan actorInput),
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
			incoming:      make(chan actorInput),    // unbuffered: no run loop, so one decode wedges the pump
			transportErr:  make(chan transportLoss), // unbuffered: same for the error forwarder
			done:          make(chan struct{}),
			serverVersion: 225,
		}
		e.attachTransport(tr)

		// Deliver one decodable frame. The pump decodes it and blocks on the
		// unconsumed e.incoming, standing in for a hot feed at shutdown. This
		// exact CurrentTime payload is from capture
		// 20260824T202747Z-current_time at server_version 225, events
		// SHA-256 a9029ff8e7cfed19cab1e3e2eccc4c36d7c91b95aa6aa03f75543bacac454a9e.
		payload := liveCapturedFrame(t, "AAAACgAAAPkIwtKy1AY=")
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

func TestCloseDrainsTrackedCompletionsWithoutActorDeadlock(t *testing.T) {
	t.Parallel()

	peer, client := net.Pipe()
	go func() { _, _ = io.Copy(io.Discard, peer) }()
	tr := transport.New(client, nil, 0)
	cfg := defaultConfig()
	e := &engine{
		cfg:            cfg,
		incoming:       make(chan actorInput),
		transportErr:   make(chan transportLoss, 1),
		ready:          make(chan error, 1),
		done:           make(chan struct{}),
		events:         newObserver[Event](cfg.eventBuffer),
		transport:      tr,
		serverVersion:  225,
		keyed:          make(map[int]*route),
		singletons:     make(map[string]*route),
		orders:         make(map[int64]*orderRoute),
		execDeliveries: make(map[string]*execDelivery),
		snapshot:       Snapshot{State: StateReady},
	}
	e.attachTransport(tr)

	target := cap(tr.Completions()) + 2
	for admitted := 0; admitted < target; {
		if _, err := tr.SendTracked(context.Background(), []byte("tracked")); err != nil {
			if errors.Is(err, transport.ErrSendQueueFull) {
				<-tr.Writable()
				continue
			}
			t.Fatalf("SendTracked: %v", err)
		}
		admitted++
	}
	deadline := time.Now().Add(3 * time.Second)
	for len(tr.Completions()) != cap(tr.Completions()) {
		if time.Now().After(deadline) {
			t.Fatalf("completion queue len = %d, want saturated %d", len(tr.Completions()), cap(tr.Completions()))
		}
		runtime.Gosched()
	}

	closed := make(chan struct{})
	go func() {
		e.closeEngine(ErrClosed, ErrClosed, nil)
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("closeEngine blocked behind tracked completion backpressure")
	}
	select {
	case <-tr.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("transport writer did not finish after engine shutdown")
	}
	_ = peer.Close()
}

func TestClosedStateIsAbsorbing(t *testing.T) {
	t.Parallel()

	e := &engine{
		incoming:       make(chan actorInput, 1),
		ready:          make(chan error, 1),
		done:           make(chan struct{}),
		events:         newObserver[Event](1),
		keyed:          make(map[int]*route),
		singletons:     make(map[string]*route),
		orders:         make(map[int64]*orderRoute),
		execDeliveries: make(map[string]*execDelivery),
		snapshot:       Snapshot{State: StateReady},
	}
	e.closeEngine(ErrClosed, ErrClosed, nil)
	e.incoming <- actorInput{kind: actorInputDecoded, message: codec.APIError{Code: 1102, Message: "Connectivity restored"}}
	runReturned := make(chan struct{})
	go func() {
		e.run()
		close(runReturned)
	}()
	select {
	case <-runReturned:
	case <-time.After(time.Second):
		t.Fatal("run loop did not stop after close")
	}
	if got := len(e.incoming); got != 1 {
		t.Fatalf("queued post-close inputs consumed = %d, want 0", 1-got)
	}

	// The state guard remains a second boundary for any direct/internal caller.
	e.handleIncoming(codec.APIError{Code: 1102, Message: "Connectivity restored"})
	if got := e.Session().State; got != StateClosed {
		t.Fatalf("state after queued post-close input = %s, want Closed", got)
	}
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
		incoming:     make(chan actorInput, 256),
		transportErr: make(chan transportLoss, 1),
		done:         make(chan struct{}),
		keyed:        make(map[int]*route),
		snapshot:     Snapshot{State: StateReady},
	}

	const feedReqID = 7
	feedStop := make(chan struct{})
	reinject := codec.TickPrice{ReqID: feedReqID}
	e.keyed[feedReqID] = &route{opKind: OpQuotes, handle: func(_ any, e *engine) {
		// Refill on the actor goroutine so e.incoming stays non-empty across
		// drainIncoming iterations. The non-blocking send is a safety valve;
		// the just-drained slot always has room in steady state.
		select {
		case <-feedStop:
		case e.incoming <- actorInput{kind: actorInputDecoded, message: reinject}:
		default:
		}
	}}
	// Seed the self-sustaining feed before the loop starts.
	e.incoming <- actorInput{kind: actorInputDecoded, message: reinject}

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

func TestPhysicalReconnectMarksOrderRecoveryBeforeBusinessEvents(t *testing.T) {
	t.Parallel()

	peer, client := net.Pipe()
	cfg := defaultConfig()
	handle := newOrderHandle(546, 8)
	e := &engine{
		cfg:              cfg,
		incoming:         make(chan actorInput, 8),
		transportErr:     make(chan transportLoss, 1),
		done:             make(chan struct{}),
		events:           newObserver[Event](cfg.eventBuffer),
		keyed:            make(map[int]*route),
		singletons:       make(map[string]*route),
		orders:           map[int64]*orderRoute{546: {orderID: 546, handle: handle, gapped: true}},
		execDeliveries:   make(map[string]*execDelivery),
		connectAttemptID: 4,
		snapshot:         Snapshot{State: StateReconnecting, ConnectionSeq: 9},
	}

	e.handleConnectResult(connectResult{
		attempt: 4, reconnect: true, conn: client, serverVersion: 225,
	})
	// Capture 20260824T210312Z-api_reconnect_active_order_aapl, server_version
	// 225, events.jsonl sha256
	// bb1d3c6803a7140fd4636570f43e91f047f57177e280b10b602f4786ace980a9.
	// On reconnect the Gateway sent OpenOrder and this exact OrderStatus before
	// ManagedAccounts/NextValidID, so recovery must be published at connection
	// attachment rather than delayed until bootstrap readiness.
	framed, err := base64.StdEncoding.DecodeString("AAAAQAAAAMsIogQSDFByZVN1Ym1pdHRlZBoBMCIBMSkAAAAAAAAAADCiuMTDITgAQQAAAAAAAAAASAFZAAAAAAAAAAA=")
	if err != nil {
		t.Fatalf("decode captured reconnect order status: %v", err)
	}
	message, err := codec.Decode(225, framed[4:])
	if err != nil {
		t.Fatalf("decode captured reconnect order status: %v", err)
	}
	e.handleIncoming(message)

	first := <-handle.Events()
	if first.Lifecycle == nil || first.Lifecycle.Kind != OrderRecoveryRequired {
		t.Fatalf("first reconnect order event = %+v, want RecoveryRequired", first)
	}
	if first.Lifecycle.ConnectionSeq != 10 {
		t.Fatalf("RecoveryRequired connection sequence = %d, want 10", first.Lifecycle.ConnectionSeq)
	}
	second := <-handle.Events()
	if second.Status == nil || second.Status.OrderID != 546 || second.Status.Status != OrderStatusPreSubmitted {
		t.Fatalf("second reconnect order event = %+v, want captured PreSubmitted status", second)
	}

	_ = e.transport.Close()
	_ = peer.Close()
	_ = e.transport.Wait()
}

func TestRepeatedPhysicalReconnectEmitsRecoveryAfterEveryGap(t *testing.T) {
	t.Parallel()

	handle := newOrderHandle(77, 4)
	e := &engine{
		orders: map[int64]*orderRoute{77: {
			orderID: 77, handle: handle, gapped: true, recoveryRequired: true,
		}},
	}
	e.requireOrderRecovery(11)

	event := <-handle.Events()
	if event.Lifecycle == nil || event.Lifecycle.Kind != OrderRecoveryRequired {
		t.Fatalf("repeated reconnect event = %+v, want RecoveryRequired", event)
	}
	if event.Lifecycle.ConnectionSeq != 11 {
		t.Fatalf("repeated RecoveryRequired sequence = %d, want 11", event.Lifecycle.ConnectionSeq)
	}
	if e.orders[77].gapped {
		t.Fatal("repeated recovery left the order route gapped")
	}
}

func TestOrderLifecycleOverflowDeletesOnlyItsLocalRoute(t *testing.T) {
	t.Parallel()

	first := newOrderHandle(41, 1)
	if !first.emitLifecycle(OrderStarted, 1, nil) {
		t.Fatal("seed lifecycle event was not admitted")
	}
	sibling := newOrderHandle(42, 1)
	e := &engine{
		keyed:      make(map[int]*route),
		singletons: make(map[string]*route),
		orders: map[int64]*orderRoute{
			41: {orderID: 41, handle: first},
			42: {orderID: 42, handle: sibling},
		},
		pendingOrderWrites: make(map[transportWriteKey]int64),
		execDeliveries: map[string]*execDelivery{
			"exec-41": {orderID: 41},
			"exec-42": {orderID: 42},
		},
		snapshot: Snapshot{State: StateReady, ConnectionSeq: 1},
	}

	e.disconnectRoutes(ErrInterrupted, true)
	if err := first.Wait(); !errors.Is(err, ErrSlowConsumer) {
		t.Fatalf("overflowed handle Wait() = %v, want ErrSlowConsumer", err)
	}
	if _, ok := e.orders[41]; ok {
		t.Fatal("overflowed order retained its route")
	}
	if _, ok := e.execDeliveries["exec-41"]; ok {
		t.Fatal("overflowed order retained its execution correlation")
	}
	if e.orders[42] == nil || e.execDeliveries["exec-42"] == nil {
		t.Fatal("overflow cleanup removed sibling ownership")
	}
}

func TestOrderBusinessEventOverflowReportsSlowConsumer(t *testing.T) {
	t.Parallel()

	handle := newOrderHandle(51, 1)
	if !handle.emitLifecycle(OrderStarted, 1, nil) {
		t.Fatal("seed lifecycle event was not admitted")
	}
	e := &engine{
		orders:         map[int64]*orderRoute{51: {orderID: 51, handle: handle}},
		execDeliveries: make(map[string]*execDelivery),
	}
	e.dispatchObservedOrderStatus(codec.OrderStatus{
		OrderID: 51, Status: string(OrderStatusSubmitted), Filled: "0", Remaining: "1",
		AvgFillPrice: "0", LastFillPrice: "0",
	})

	if err := handle.Wait(); !errors.Is(err, ErrSlowConsumer) {
		t.Fatalf("overflowed status handle Wait() = %v, want ErrSlowConsumer", err)
	}
	if _, ok := e.orders[51]; ok {
		t.Fatal("overflowed status retained its order route")
	}
}

func TestExecutionProjectionFailureClosesLocalOrderHandle(t *testing.T) {
	t.Parallel()

	handle := newOrderHandle(91, 4)
	e := &engine{
		cfg:            config{logger: slog.Default(), orderExecutionCorrelationLimit: defaultExecutionCorrelationLimit},
		orders:         map[int64]*orderRoute{91: {orderID: 91, handle: handle}},
		execDeliveries: make(map[string]*execDelivery),
	}
	e.dispatchExecutionToOrder(codec.ExecutionDetail{
		OrderID: 91,
		ExecID:  "bad-exec",
		Shares:  "not-a-decimal",
		Price:   "1",
		Time:    "20260713-16:00:00",
	})

	if _, ok := e.orders[91]; ok {
		t.Fatal("execution projection failure retained the order route")
	}
	err := handle.Wait()
	protocolErr, ok := errors.AsType[*ProtocolError](err)
	if !ok || protocolErr.Direction != "inbound" {
		t.Fatalf("handle Wait() = %#v, want inbound ProtocolError", err)
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
			incoming:       make(chan actorInput, 8),
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
			e.orders[99] = &orderRoute{orderID: 99, handle: newOrderHandle(99, 1)}
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

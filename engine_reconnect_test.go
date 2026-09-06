package ibkr

import (
	"bytes"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/transport"
)

func TestConnectivity1101DropsLostWorkAndResubscribes(t *testing.T) {
	t.Parallel()

	peer, client := net.Pipe()
	tr := transport.New(client, nil, 0)
	t.Cleanup(func() {
		_ = tr.Close()
		_ = peer.Close()
		_ = tr.Wait()
	})

	resubscribed := 0
	resumeErr := make(chan error, 1)
	oneShotErr := make(chan error, 1)
	singletonErr := make(chan error, 1)
	preview := &previewRoute{result: make(chan previewResult, 1)}
	auto := &route{
		opKind:       OpQuotes,
		subscription: true,
		resume:       ResumeAuto,
		request: codec.QuoteRequest{
			ReqID: 1,
			Contract: codec.Contract{
				Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD",
			},
		},
		gapped:           true,
		emitResubscribed: func(*engine) { resubscribed++ },
		close:            func(error) {},
	}
	e := &engine{
		cfg:            config{reconnect: ReconnectAuto},
		cmds:           make(chan func(), 1),
		done:           make(chan struct{}),
		events:         newObserver[Event](8),
		transport:      tr,
		serverVersion:  225,
		keyed:          map[int]*route{1: auto},
		singletons:     make(map[string]*route),
		orders:         make(map[int64]*orderRoute),
		previews:       map[int64]*previewRoute{4: preview},
		execDeliveries: make(map[string]*execDelivery),
		snapshot:       Snapshot{State: StateDegraded, ConnectionSeq: 1},
	}
	defer close(e.done)
	e.keyed[2] = &route{
		subscription: true,
		resume:       ResumeNever,
		onDisconnect: func(_ *engine, err error) bool {
			resumeErr <- resumeRequired(err)
			return false
		},
		close: func(error) {},
	}
	e.keyed[3] = &route{close: func(err error) { oneShotErr <- err }}
	e.singletons[singletonMarketRule] = &route{close: func(err error) { singletonErr <- err }}

	e.handleAPIError(codec.APIError{Code: 1101, Message: "Connectivity restored - data lost."})

	wantRequest, err := codec.Encode(225, auto.request)
	if err != nil {
		t.Fatalf("encode resumed request: %v", err)
	}
	gotRequest, err := transport.ReadOneFrame(peer, time.Now().Add(time.Second))
	if err != nil {
		t.Fatalf("read resumed request: %v", err)
	}
	if !bytes.Equal(gotRequest, wantRequest) {
		t.Fatalf("resumed request = %x, want %x", gotRequest, wantRequest)
	}
	if e.keyed[1] != auto || auto.gapped || resubscribed != 1 {
		t.Fatalf("auto route retained=%t gapped=%t resubscribed=%d", e.keyed[1] == auto, auto.gapped, resubscribed)
	}
	if _, ok := e.keyed[2]; ok {
		t.Fatal("ResumeNever route survived data-lost restoration")
	}
	if err := <-resumeErr; !errors.Is(err, ErrResumeRequired) || !IsRetryable(err) {
		t.Fatalf("ResumeNever error = %v, want retryable ErrResumeRequired", err)
	}
	if _, ok := e.keyed[3]; ok {
		t.Fatal("one-shot route survived data-lost restoration")
	}
	if err := <-oneShotErr; !errors.Is(err, ErrInterrupted) {
		t.Fatalf("one-shot error = %v, want ErrInterrupted", err)
	}
	if _, ok := e.previews[4]; ok {
		t.Fatal("what-if preview survived data-lost restoration")
	}
	if result := <-preview.result; !errors.Is(result.err, ErrInterrupted) {
		t.Fatalf("preview error = %v, want ErrInterrupted", result.err)
	}
	if err := <-singletonErr; !errors.Is(err, ErrInterrupted) || len(e.singletons) != 0 {
		t.Fatalf("singleton survived data loss: %v", err)
	}
	if got := e.Session().State; got != StateReady {
		t.Fatalf("session state = %s, want %s", got, StateReady)
	}
}

func TestConnectivity1100And1102GapAndRestoreOnce(t *testing.T) {
	t.Parallel()

	gaps := 0
	restored := 0
	recoveryRoute := &route{
		subscription: true,
		resume:       ResumeAuto,
		emitGap:      func(*engine) { gaps++ },
		emitRestored: func(*engine) { restored++ },
	}
	e := &engine{
		events:         newObserver[Event](8),
		keyed:          map[int]*route{1: recoveryRoute},
		singletons:     make(map[string]*route),
		orders:         make(map[int64]*orderRoute),
		execDeliveries: make(map[string]*execDelivery),
		snapshot:       Snapshot{State: StateReady, ConnectionSeq: 1},
	}

	h := newOrderHandle(47, 4)
	e.orders[47] = &orderRoute{orderID: 47, handle: h, working: true}
	e.handleAPIError(codec.APIError{Code: 1100, Message: "Connectivity between IB and TWS has been lost."})
	e.handleAPIError(codec.APIError{Code: 1100, Message: "Connectivity between IB and TWS has been lost."})
	if got := e.Session().State; got != StateDegraded || gaps != 1 || !recoveryRoute.gapped {
		t.Fatalf("after 1100 state=%s gaps=%d gapped=%t", got, gaps, recoveryRoute.gapped)
	}

	e.handleAPIError(codec.APIError{Code: 1102, Message: "Connectivity restored - data maintained."})
	if got := e.Session().State; got != StateReady || restored != 1 || recoveryRoute.gapped {
		t.Fatalf("after 1102 state=%s restored=%d gapped=%t", got, restored, recoveryRoute.gapped)
	}
	for _, want := range []OrderLifecycleKind{OrderGap, OrderRestored} {
		event := <-h.Events()
		if event.Lifecycle == nil || event.Lifecycle.Kind != want {
			t.Fatalf("order lifecycle = %+v, want %v", event, want)
		}
	}
	if e.orders[47].gapped || e.orders[47].recoveryRequired {
		t.Fatal("data-maintained restoration disabled replacement")
	}
	select {
	case extra := <-h.Events():
		t.Fatalf("duplicate lifecycle: %+v", extra)
	default:
	}
}

func TestConnectivity1100ThenTransportLossDoesNotDuplicateGap(t *testing.T) {
	t.Parallel()

	gaps := 0
	lost := &transport.Conn{}
	e := &engine{
		cfg:              config{reconnect: ReconnectAuto},
		cmds:             make(chan func(), 1),
		done:             make(chan struct{}),
		events:           newObserver[Event](8),
		transport:        lost,
		keyed:            map[int]*route{1: {subscription: true, resume: ResumeAuto, emitGap: func(*engine) { gaps++ }}},
		singletons:       make(map[string]*route),
		orders:           make(map[int64]*orderRoute),
		execDeliveries:   make(map[string]*execDelivery),
		snapshot:         Snapshot{State: StateReady, ConnectionSeq: 1},
		reconnectAttempt: 0,
	}
	defer close(e.done)

	e.handleAPIError(codec.APIError{Code: 1100, Message: "Connectivity between IB and TWS has been lost."})
	e.handleTransportLoss(transportLoss{transport: lost, err: errors.New("transport closed")})

	if gaps != 1 {
		t.Fatalf("gap callbacks = %d, want 1", gaps)
	}
	if got := e.Session().State; got != StateReconnecting {
		t.Fatalf("session state = %s, want %s", got, StateReconnecting)
	}
}

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

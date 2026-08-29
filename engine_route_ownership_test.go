package ibkr

import (
	"errors"
	"math"
	"net"
	"sync"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/shopspring/decimal"
)

func TestObservedOpenOrderOwnsEachConsumerPayload(t *testing.T) {
	t.Parallel()

	// The ownership-only composite starts from the current sv225 paper BAG
	// order echo (order 485, legs 909446204/909903509, and NonGuaranteed
	// routing), capture 20260824T204518Z-api_combo_option_vertical_aapl,
	// events SHA-256
	// 2bc51d3660286ba2b739fdf44acab818a3468efbceb172712d278eb4bb59278b.
	// The 0.05 per-leg decimal is the official ComboLeg.proto tag-9 source-law
	// vector frozen in codec_orders_proto203_test.go; IncludeOvernight=true is
	// likewise an official Order.proto source-law value, not a positive live
	// attestation. Combining them here exercises ownership, not Gateway meaning.
	openOrders := make(chan OpenOrder, 1)
	handle := newOrderHandle(485, 64)
	e := &engine{
		orders: map[int64]*orderRoute{485: {orderID: 485, handle: handle}},
		singletons: map[string]*route{singletonOpenOrders: {
			handle: func(msg any, _ *engine) {
				openOrders <- msg.(OpenOrder)
			},
		}},
	}
	e.dispatchObservedOpenOrder(codec.OpenOrder{
		OrderID: 485,
		OrderDetails: codec.OrderDetails{Contract: codec.Contract{
			Symbol: "AAPL", SecType: "BAG", Exchange: "SMART", Currency: "USD",
			ComboLegs: []codec.ComboLeg{
				{ConID: 909446204, Ratio: 1, Action: "SELL", Exchange: "SMART", OpenClose: "0", ShortSaleSlot: "0", ExemptCode: "-1"},
				{ConID: 909903509, Ratio: 1, Action: "BUY", Exchange: "SMART", OpenClose: "0", ShortSaleSlot: "0", ExemptCode: "-1"},
			},
		},
			Action: "BUY", OrderType: "LMT", Quantity: "5", LmtPrice: "0.05", IncludeOvernight: "1", OrderComboLegPrices: []string{"0.05", ""},
			SmartComboRouting: []codec.TagValue{{Tag: "NonGuaranteed", Value: "1"}},
		},
	})

	handleOrder := (<-handle.Events()).OpenOrder
	openOrdersOrder := <-openOrders
	if handleOrder == nil {
		t.Fatal("order handle did not receive open order")
	}
	if handleOrder.Order.IncludeOvernight == nil || openOrdersOrder.Order.IncludeOvernight == nil {
		t.Fatalf("include overnight pointers = %v/%v, want explicit true", handleOrder.Order.IncludeOvernight, openOrdersOrder.Order.IncludeOvernight)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		handleOrder.Contract.ComboLegs[0].ConID = 887307502
		*handleOrder.Order.Combo.LegPrices[0] = decimal.RequireFromString("0.05")
		handleOrder.Order.Combo.SmartRouting[0].Value = "0"
		*handleOrder.Order.IncludeOvernight = false
	}()
	go func() {
		defer wg.Done()
		openOrdersOrder.Contract.ComboLegs[0].ConID = 887307536
		*openOrdersOrder.Order.Combo.LegPrices[0] = decimal.RequireFromString("291.09")
		openOrdersOrder.Order.Combo.SmartRouting[0].Value = "1"
		*openOrdersOrder.Order.IncludeOvernight = true
	}()
	wg.Wait()

	if handleOrder.Contract.ComboLegs[0].ConID != 887307502 || openOrdersOrder.Contract.ComboLegs[0].ConID != 887307536 ||
		!handleOrder.Order.Combo.LegPrices[0].Equal(decimal.RequireFromString("0.05")) || !openOrdersOrder.Order.Combo.LegPrices[0].Equal(decimal.RequireFromString("291.09")) ||
		handleOrder.Order.Combo.SmartRouting[0].Value != "0" || openOrdersOrder.Order.Combo.SmartRouting[0].Value != "1" ||
		*handleOrder.Order.IncludeOvernight || !*openOrdersOrder.Order.IncludeOvernight {
		t.Fatalf("dual dispatch shares mutable storage: handle=%+v subscription=%+v", *handleOrder, openOrdersOrder)
	}
}

func TestRequestIDSkipsPendingPreview(t *testing.T) {
	t.Parallel()

	e := &engine{
		nextReqID: 41,
		orders:    make(map[int64]*orderRoute),
		previews: map[int64]*previewRoute{
			41: {result: make(chan previewResult, 1)},
		},
	}

	if got, err := e.allocReqID(); err != nil || got != 42 {
		t.Fatalf("allocReqID() = %d, %v; want 42 after pending preview 41", got, err)
	}
}

func TestRequestIDWrapsWithinProtocolRangeWithoutColliding(t *testing.T) {
	t.Parallel()

	e := &engine{
		nextReqID: math.MaxInt32,
		keyed:     map[int]*route{math.MaxInt32: {}},
		orders:    map[int64]*orderRoute{1: {}},
		previews:  map[int64]*previewRoute{2: {}},
	}

	if got, err := e.allocReqID(); err != nil || got != 3 {
		t.Fatalf("allocReqID() = %d, %v; want 3 after wrapping past live IDs", got, err)
	}
}

func TestRequestIDAllocationReportsExhaustedWireRange(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name            string
		snapshot        Snapshot
		orderIDLowWater int64
		nextReqID       int
		keyed           map[int]*route
	}{
		{
			name:            "only candidate is active",
			snapshot:        Snapshot{NextValidID: maxWireOrderID},
			orderIDLowWater: 1,
			nextReqID:       math.MaxInt32,
			keyed:           map[int]*route{math.MaxInt32: {}},
		},
		{
			name:            "orders cover the wire range",
			snapshot:        Snapshot{NextValidID: maxWireOrderID + 1},
			orderIDLowWater: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := &engine{
				snapshot: tc.snapshot, orderIDLowWater: tc.orderIDLowWater,
				nextReqID: tc.nextReqID, keyed: tc.keyed,
			}
			got, err := e.allocReqID()
			if got != 0 || !errors.Is(err, errRequestIDExhausted) || err == errRequestIDExhausted {
				t.Fatalf("allocReqID() = %d, %#v; want zero and wrapped exhaustion error", got, err)
			}
		})
	}
}

func TestOrderIDRemainsMonotonicAfterBackwardReseed(t *testing.T) {
	t.Parallel()

	e := &engine{
		keyed:    make(map[int]*route),
		orders:   make(map[int64]*orderRoute),
		previews: make(map[int64]*previewRoute),
		snapshot: Snapshot{NextValidID: 47},
	}
	if got, err := e.allocOrderID(); err != nil || got != 47 {
		t.Fatalf("first allocOrderID() = %d, want 47", got)
	}

	e.handleIncoming(codec.NextValidID{OrderID: 47})
	if got := e.snapshot.NextValidID; got != 48 {
		t.Fatalf("NextValidID after backward reseed = %d, want 48", got)
	}
	if got, err := e.allocOrderID(); err != nil || got != 48 {
		t.Fatalf("second allocOrderID() = %d, want 48", got)
	}
}

func TestOrderIDSkipsEveryLiveRouteNamespace(t *testing.T) {
	t.Parallel()

	e := &engine{
		keyed: map[int]*route{
			47: {},
		},
		orders: map[int64]*orderRoute{
			48: {},
		},
		previews: map[int64]*previewRoute{
			49: {},
		},
		snapshot: Snapshot{NextValidID: 47},
	}
	if got, err := e.allocOrderID(); err != nil || got != 50 {
		t.Fatalf("allocOrderID() = %d, want 50 after keyed/order/preview conflicts", got)
	}
}

func TestOrderIDAllocationStopsAtSignedWireBoundary(t *testing.T) {
	t.Parallel()

	e := &engine{
		keyed:    make(map[int]*route),
		orders:   make(map[int64]*orderRoute),
		previews: make(map[int64]*previewRoute),
		snapshot: Snapshot{NextValidID: maxWireOrderID},
	}
	if got, err := e.allocOrderID(); err != nil || got != maxWireOrderID {
		t.Fatalf("boundary allocOrderID() = %d, %v", got, err)
	}
	if got, err := e.allocOrderID(); got != 0 {
		t.Fatalf("overflow allocOrderID() ID = %d, want 0", got)
	} else if _, ok := errors.AsType[*ValidationError](err); !ok {
		t.Fatalf("overflow allocOrderID() error = %#v, want ValidationError", err)
	}
}

// The values below are the second-client leg of capture
// 20260824T204700Z-api_cross_client_cancel_aapl at server_version 225,
// events SHA-256
// 9d7fafb105cfe1cd1b12b46b6dcf00d983a20b4e3c71a14239dd441795b1f3c6:
// NextValidID regressed to 299 before the client received OpenOrder for another
// client's order 493. The independent low request sequence must remain intact
// while a bounded conservative interval prevents it from ever reaching order
// 493.
func TestObservedCrossClientOrderBoundsIdentifierInterval(t *testing.T) {
	t.Parallel()

	e := &engine{
		nextReqID: 299,
		keyed:     make(map[int]*route),
		orders:    make(map[int64]*orderRoute),
		previews:  make(map[int64]*previewRoute),
		snapshot:  Snapshot{NextValidID: 299},
	}
	// Only the identifier-bearing fields are needed here; both values are the
	// exact projection of the second-leg OpenOrder callback.
	e.dispatchObservedOpenOrder(codec.OpenOrder{
		OrderID: 493,
		OrderDetails: codec.OrderDetails{
			ClientID: "1",
		},
	})
	if got := e.snapshot.NextValidID; got != 494 {
		t.Fatalf("NextValidID after observed order 493 = %d, want 494", got)
	}
	if e.orderIDLowWater != 493 {
		t.Fatalf("order ID low-water = %d, want 493", e.orderIDLowWater)
	}
	if got, err := e.allocReqID(); err != nil || got != 299 {
		t.Fatalf("independent request ID after observed order 493 = %d, %v; want 299", got, err)
	}
	if got, err := e.allocOrderID(); err != nil || got != 494 {
		t.Fatalf("order ID after request 299 = %d, %v; want 494", got, err)
	}
	e.nextReqID = 492
	if got, err := e.allocReqID(); err != nil || got != 492 {
		t.Fatalf("request ID below historical order interval = %d, %v; want 492", got, err)
	}
	if got, err := e.allocReqID(); err != nil || got != 495 {
		t.Fatalf("request ID entering historical order interval = %d, %v; want 495", got, err)
	}
}

func TestIdentifierFloorsSurviveReconnect(t *testing.T) {
	t.Parallel()

	e := newEngineForErrorTest()
	e.nextReqID = 71
	e.requestIDHighWater = 70
	e.orderIDLowWater = 69
	e.previews = make(map[int64]*previewRoute)
	e.snapshot = Snapshot{NextValidID: 70}
	e.cmds = make(chan func(), 1)
	e.incoming = make(chan actorInput, 1)
	e.transportErr = make(chan transportLoss, 1)
	e.done = make(chan struct{})
	e.dirtySingletons = make(map[string]uint64)
	server, client := net.Pipe()
	e.handleConnectResult(connectResult{conn: client, serverVersion: 225, reconnect: true})
	t.Cleanup(func() {
		_ = e.transport.Close()
		_ = server.Close()
		_ = e.transport.Wait()
		close(e.done)
	})
	if e.orderIDLowWater != 69 || e.snapshot.NextValidID != 70 || e.requestIDHighWater != 70 {
		t.Fatalf("reconnect reset identifier bounds: low=%d order=%d request=%d", e.orderIDLowWater, e.snapshot.NextValidID, e.requestIDHighWater)
	}
	e.observeNextValidID(1)
	if got, err := e.allocOrderID(); err != nil || got != 71 {
		t.Fatalf("post-reconnect order ID = %d, %v; want 71", got, err)
	}
	e.nextReqID = 69
	if got, err := e.allocReqID(); err != nil || got != 72 {
		t.Fatalf("post-reconnect request ID = %d, %v; want 72 above shared floor", got, err)
	}
}

func TestExerciseIDAdvancesBothIdentifierFloors(t *testing.T) {
	t.Parallel()

	e := newEngineForErrorTest()
	e.nextReqID = 77
	e.installExerciseRoute(77)
	e.closeOrderRoute(77, e.orders[77], nil)

	if e.snapshot.NextValidID != 78 || e.requestIDHighWater != 77 {
		t.Fatalf("exercise ID floors = order %d request %d, want 78/77", e.snapshot.NextValidID, e.requestIDHighWater)
	}
	e.nextReqID = 77
	if got, err := e.allocReqID(); err != nil || got != 78 {
		t.Fatalf("request ID after exercise 77 = %d, %v; want 78", got, err)
	}
	if got, err := e.allocOrderID(); err != nil || got != 79 {
		t.Fatalf("order ID after exercise/request = %d, %v; want 79", got, err)
	}
}

func TestLateRequestErrorCannotReachLaterOrder(t *testing.T) {
	t.Parallel()

	e := newEngineForErrorTest()
	e.nextReqID = 47
	e.previews = make(map[int64]*previewRoute)
	reqID, err := e.allocReqID()
	if err != nil || reqID != 47 {
		t.Fatalf("request ID = %d, %v; want 47", reqID, err)
	}
	orderID, err := e.allocOrderID()
	if err != nil || orderID != 48 {
		t.Fatalf("order ID after request 47 = %d, %v; want 48", orderID, err)
	}
	handle := newOrderHandle(orderID, 4)
	e.orders[orderID] = &orderRoute{orderID: orderID, handle: handle}

	e.handleAPIError(codec.APIError{ReqID: reqID, Code: ErrCodeNoSecurityDefinition, Message: "late abandoned request error"})
	select {
	case event := <-handle.Events():
		t.Fatalf("late request error reached later order: %+v", event)
	case <-handle.Done():
		t.Fatal("late request error closed later order")
	default:
	}
}

func TestInvalidNextValidOrderIDIsInboundProtocolError(t *testing.T) {
	t.Parallel()

	var got error
	e := &engine{
		singletons: map[string]*route{
			singletonOrderID: {close: func(err error) { got = err }},
		},
	}
	e.handleIncoming(codec.NextValidID{OrderID: maxWireOrderID + 1})

	protocolErr, ok := errors.AsType[*ProtocolError](got)
	if !ok || protocolErr.Direction != "inbound" {
		t.Fatalf("refresh error = %#v, want inbound ProtocolError", got)
	}
	if e.bootstrap.nextValidID {
		t.Fatal("invalid next-valid ID satisfied bootstrap")
	}
	if _, ok := e.singletons[singletonOrderID]; ok {
		t.Fatal("invalid next-valid ID retained refresh route")
	}
}

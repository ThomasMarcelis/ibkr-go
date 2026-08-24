package ibkr

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
)

const (
	attributionOrderID = int64(296)
	attributionPermID  = int64(900296)
)

// These identity values and transitions are retained exactly in
// api_cross_client_cancel_aapl.txt (paper Gateway sv225, capture
// 20260824T204700Z-api_cross_client_cancel_aapl, events SHA-256
// 9d7fafb105cfe1cd1b12b46b6dcf00d983a20b4e3c71a14239dd441795b1f3c6).
// Typed callbacks below use compact local identities while varying only those
// slots needed to specify route ownership; they do not claim new wire frames.
func TestOrderAttributionBindsPermanentIdentityAcrossCallbacks(t *testing.T) {
	for _, family := range []string{"open_order", "order_status", "execution", "order_bound"} {
		t.Run(family, func(t *testing.T) {
			e, handle := newAttributionEngine()
			steps := []struct {
				name      string
				clientID  int
				permID    int64
				wantLocal bool
			}{
				{"foreign before binding", 2, attributionPermID, false},
				{"configured client binds", 1, attributionPermID, true},
				{"matching PermID changes client", 2, attributionPermID, true},
				{"conflicting PermID", 1, attributionPermID + 1, false},
				{"missing PermID configured client", 1, 0, true},
				{"missing PermID foreign client", 2, 0, false},
			}
			for i, step := range steps {
				e.handleIncoming(attributionCallback(family, step.clientID, step.permID, i))
				assertOrderAttributionEvent(t, handle, step.name, step.wantLocal)
			}
		})
	}
}

func TestOrderAttributionKeepsForeignCallbacksOnPassiveObservers(t *testing.T) {
	e, handle := newAttributionEngine()
	// Bind the local route before attaching passive observers.
	e.handleIncoming(attributionCallback("open_order", 1, attributionPermID, 0))
	assertOrderAttributionEvent(t, handle, "binding open order", true)
	openOrders := make(chan OpenOrder, 1)
	e.singletons[singletonOpenOrders] = &route{request: codec.OpenOrdersRequest{Scope: string(OpenOrdersScopeAll)}, handle: func(msg any, _ *engine) {
		openOrders <- msg.(OpenOrder)
	}}
	executions := newSubscription[ExecutionEvent](subscriptionConfig{buffer: 1}, nil)
	e.executionEvents = &executionEventRoute{sub: executions}

	// Both callbacks carry a foreign PermID.
	e.handleIncoming(attributionCallback("open_order", 1, attributionPermID+1, 1))
	assertOrderAttributionEvent(t, handle, "foreign open order", false)
	select {
	case <-openOrders:
	default:
		t.Fatal("all-open-orders observer did not receive foreign open order")
	}

	e.handleIncoming(attributionCallback("execution", 1, attributionPermID+1, 2))
	assertOrderAttributionEvent(t, handle, "foreign execution", false)
	select {
	case event := <-executions.Events():
		if event.Value.Execution == nil {
			t.Fatalf("passive execution event = %#v, want execution", event)
		}
	default:
		t.Fatal("passive observer did not receive foreign execution")
	}
}

func TestOrderAttributionCommissionFollowsClaimedExecution(t *testing.T) {
	e, handle := newAttributionEngine()
	e.handleIncoming(attributionCallback("execution", 1, attributionPermID, 0))
	assertOrderAttributionEvent(t, handle, "local execution", true)
	e.routeCommissionReport(codec.CommissionReport{ExecID: "attribution-0", Commission: "1.25", Currency: "USD"})
	assertOrderAttributionEvent(t, handle, "claimed commission", true)

	e.handleIncoming(attributionCallback("execution", 1, attributionPermID+1, 1))
	assertOrderAttributionEvent(t, handle, "foreign execution", false)
	e.routeCommissionReport(codec.CommissionReport{ExecID: "attribution-1", Commission: "1.25", Currency: "USD"})
	assertOrderAttributionEvent(t, handle, "foreign commission", false)
}

func newAttributionEngine() (*engine, *OrderHandle) {
	e, _ := newEngineForDispatchTest()
	e.cfg.clientID = 1
	e.previews = make(map[int64]*previewRoute)
	handle := newOrderHandle(attributionOrderID, 16)
	e.orders[attributionOrderID] = &orderRoute{orderID: attributionOrderID, handle: handle}
	return e, handle
}

func attributionCallback(family string, clientID int, permID int64, sequence int) any {
	client, perm := strconv.Itoa(clientID), strconv.FormatInt(permID, 10)
	switch family {
	case "open_order":
		return codec.OpenOrder{OrderID: attributionOrderID, OrderDetails: codec.OrderDetails{
			Contract: codec.Contract{ConID: 265598, Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"},
			Action:   "BUY", Quantity: "10", OrderType: "LMT", LmtPrice: "13.27", AuxPrice: "0", TIF: "GTC",
			Account: "DU9000001", Origin: "0", ClientID: client, PermID: perm,
		}, Status: "Submitted"}
	case "order_status":
		return codec.OrderStatus{OrderID: attributionOrderID, Status: "Submitted", Filled: "0", Remaining: "10",
			AvgFillPrice: "0", PermID: perm, ParentID: "0", LastFillPrice: "0", ClientID: client, MktCapPrice: "0"}
	case "execution":
		return codec.ExecutionDetail{ReqID: -1, OrderID: attributionOrderID, ExecID: fmt.Sprintf("attribution-%d", sequence),
			Time: "20260415-16:28:58", Shares: "1", Price: "13.27", PermID: perm, ClientID: client}
	case "order_bound":
		return codec.OrderBound{OrderID: attributionOrderID, PermID: permID, ClientID: clientID}
	default:
		panic("unknown callback family")
	}
}

func assertOrderAttributionEvent(t *testing.T, handle *OrderHandle, step string, want bool) {
	t.Helper()
	select {
	case event := <-handle.Events():
		if !want {
			t.Errorf("%s reached local handle: %+v", step, event)
		}
	default:
		if want {
			t.Errorf("%s did not reach local handle", step)
		}
	}
}

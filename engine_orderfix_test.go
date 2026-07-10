package ibkr

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
)

// newEngineForErrorTest extends the dispatch-test engine with a session-event
// observer so handleAPIError's emitEvent path is exercisable without the actor
// loop or a transport.
func newEngineForErrorTest() *engine {
	e, _ := newEngineForDispatchTest()
	e.events = newObserver[Event](16)
	return e
}

// TestDispatchExecutionToOrderDedupesReplayedFill freezes the snapshot-replay
// dedupe fix: a fill delivered live to an OrderHandle must not be re-emitted
// when a later Executions() snapshot query replays the same ExecID. The
// claimed delivery record marks exactly the fills the handle has already seen, so a
// second dispatch of the same ExecID drops.
func TestDispatchExecutionToOrderDedupesReplayedFill(t *testing.T) {
	t.Parallel()

	e, _ := newEngineForDispatchTest()
	handle := newOrderHandle(90, 64)
	e.orders[90] = &orderRoute{orderID: 90, handle: handle}

	detail := codec.ExecutionDetail{
		ReqID:   -1,
		OrderID: 90,
		ExecID:  "exec-live-1",
		Shares:  "10",
		Price:   "150",
		Time:    "20260610-19:58:22",
	}
	// The live fill reaches the handle.
	e.dispatchExecutionToOrder(detail)
	// A later executions snapshot replays the identical fill.
	e.dispatchExecutionToOrder(detail)

	count := 0
	for {
		select {
		case evt := <-handle.Events():
			if evt.Execution != nil {
				count++
			}
			continue
		default:
		}
		break
	}
	if count != 1 {
		t.Fatalf("handle saw %d execution events for one ExecID, want exactly 1 (deduped)", count)
	}
}

// TestRouteCommissionReportDedupesReplayedCommission freezes the commission
// half of the snapshot-replay dedupe: a fill and its commission delivered live
// to an OrderHandle must each land exactly once, even when a later Executions()
// snapshot query replays the same ExecID for both. Pre-fix the execution was
// deduped but the commission was not, so the handle double-counted it.
func TestRouteCommissionReportDedupesReplayedCommission(t *testing.T) {
	t.Parallel()

	e, _ := newEngineForDispatchTest()
	handle := newOrderHandle(90, 64)
	e.orders[90] = &orderRoute{orderID: 90, handle: handle}

	exec := codec.ExecutionDetail{
		ReqID:   -1,
		OrderID: 90,
		ExecID:  "exec-live-1",
		Shares:  "10",
		Price:   "150",
		Time:    "20260610-19:58:22",
	}
	comm := codec.CommissionReport{ExecID: "exec-live-1", Commission: "1.25", Currency: "USD", RealizedPNL: "0"}

	// Live delivery: execution, then its commission.
	e.dispatchExecutionToOrder(exec)
	e.routeCommissionReport(comm)
	// A later executions snapshot replays the identical fill and commission.
	e.dispatchExecutionToOrder(exec)
	e.routeCommissionReport(comm)

	execs, comms := 0, 0
	for {
		select {
		case evt := <-handle.Events():
			if evt.Execution != nil {
				execs++
			}
			if evt.CommissionAndFees != nil {
				comms++
			}
			continue
		default:
		}
		break
	}
	if execs != 1 || comms != 1 {
		t.Fatalf("handle saw %d executions / %d commissions for one ExecID, want 1/1 (both deduped)", execs, comms)
	}
}

// TestRouteCommissionBeforeExecutionReachesHandle freezes the racing-commission
// fix: the Gateway can deliver a commission_report before its execution detail
// (the keyed leg buffers the same race in the correlator). Pre-fix the
// order-handle leg looked up the ExecID's owner, found none, and dropped the
// commission permanently; nothing re-triggered it when the execution arrived.
func TestRouteCommissionBeforeExecutionReachesHandle(t *testing.T) {
	t.Parallel()

	e, _ := newEngineForDispatchTest()
	handle := newOrderHandle(90, 64)
	e.orders[90] = &orderRoute{orderID: 90, handle: handle}

	comm := codec.CommissionReport{ExecID: "exec-race-1", Commission: "1.25", Currency: "USD", RealizedPNL: "0"}
	exec := codec.ExecutionDetail{
		ReqID:   -1,
		OrderID: 90,
		ExecID:  "exec-race-1",
		Shares:  "10",
		Price:   "150",
		Time:    "20260610-19:58:22",
	}

	// Commission first, then the execution it belongs to.
	e.routeCommissionReport(comm)
	e.dispatchExecutionToOrder(exec)

	var events []OrderEvent
	for {
		select {
		case evt := <-handle.Events():
			events = append(events, evt)
			continue
		default:
		}
		break
	}
	if len(events) != 2 || events[0].Execution == nil || events[1].CommissionAndFees == nil {
		t.Fatalf("handle events = %+v, want execution then flushed commission", events)
	}
	if events[1].CommissionAndFees.ExecID != "exec-race-1" {
		t.Fatalf("flushed commission ExecID = %q, want exec-race-1", events[1].CommissionAndFees.ExecID)
	}
}

// TestRouteCommissionResendWithChangedContentReachesHandle freezes the
// content-aware half of the commission dedupe: an identical re-send (a
// snapshot replay) is dropped, but a re-send whose content changed — e.g. the
// Gateway filling in realizedPNL after the fact — must reach the handle.
// Pre-fix the dedupe keyed on ExecID presence alone and dropped the update.
func TestRouteCommissionResendWithChangedContentReachesHandle(t *testing.T) {
	t.Parallel()

	e, _ := newEngineForDispatchTest()
	handle := newOrderHandle(90, 64)
	e.orders[90] = &orderRoute{orderID: 90, handle: handle}

	exec := codec.ExecutionDetail{
		ReqID:   -1,
		OrderID: 90,
		ExecID:  "exec-upd-1",
		Shares:  "10",
		Price:   "150",
		Time:    "20260610-19:58:22",
	}
	initial := codec.CommissionReport{ExecID: "exec-upd-1", Commission: "1.25", Currency: "USD"}
	updated := codec.CommissionReport{ExecID: "exec-upd-1", Commission: "1.25", Currency: "USD", RealizedPNL: "42.10"}

	e.dispatchExecutionToOrder(exec)
	e.routeCommissionReport(initial)
	e.routeCommissionReport(initial) // identical replay: deduped
	e.routeCommissionReport(updated) // changed content: delivered
	e.routeCommissionReport(updated) // identical replay of the update: deduped

	comms := 0
	var last CommissionAndFeesReport
	for {
		select {
		case evt := <-handle.Events():
			if evt.CommissionAndFees != nil {
				comms++
				last = *evt.CommissionAndFees
			}
			continue
		default:
		}
		break
	}
	if comms != 2 {
		t.Fatalf("handle saw %d commissions, want 2 (initial + changed re-send)", comms)
	}
	if !last.RealizedPNL.Equal(decimal.RequireFromString("42.10")) {
		t.Fatalf("last commission realizedPNL = %s, want 42.10", last.RealizedPNL)
	}
}

// TestHandleAPIErrorEmitsUnroutedRequestError freezes Bug 2: a request-range
// (<10000) error carrying a reqID that matches no keyed route and no order
// route must surface as a session event, not vanish. This is the path that
// carries live option-exercise refusals (code 322) once the exercise route is
// gone or was never registered.
func TestHandleAPIErrorEmitsUnroutedRequestError(t *testing.T) {
	t.Parallel()

	e := newEngineForErrorTest()
	e.handleAPIError(codec.APIError{
		ReqID:   777,
		Code:    ErrCodeServerErrorProcessingRequest,
		Message: "Error processing request.Exercise ignored because option is not in-the-money.",
	})

	select {
	case evt := <-e.events.Chan():
		if evt.Code != ErrCodeServerErrorProcessingRequest {
			t.Fatalf("session event code = %d, want %d", evt.Code, ErrCodeServerErrorProcessingRequest)
		}
	default:
		t.Fatal("unrouted request-range error was dropped; want a session event")
	}
}

func TestHandleAPIErrorRoutesUnkeyedSingletonByLiveMarker(t *testing.T) {
	t.Parallel()

	// Both code-321 messages are exact live Gateway text. reqIds is from
	// 20260611T074047Z (events SHA-256 prefix 00b11cbce4cefc31);
	// open orders is from 20260710T225552Z (events SHA-256
	// 0e838de9d463070ac711be4950948c682c01e8ad02546d8be32f47f35ce68d25).
	const cause = " : cause - The API interface is currently in Read-Only mode."
	for _, tc := range []struct {
		name    string
		marker  string
		wantHit string
	}{
		{name: "order ID", marker: "b7", wantHit: singletonOrderID},
		{name: "open orders", marker: "as", wantHit: singletonOpenOrders},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hit := ""
			e := newEngineForErrorTest()
			e.singletons[singletonOrderID] = &route{handleAPIErr: func(codec.APIError, *engine) { hit = singletonOrderID }}
			e.singletons[singletonOpenOrders] = &route{
				request:      codec.OpenOrdersRequest{Scope: string(OpenOrdersScopeClient)},
				handleAPIErr: func(codec.APIError, *engine) { hit = singletonOpenOrders },
			}
			e.handleAPIError(codec.APIError{ReqID: -1, Code: 321, Message: "Error validating request.-'" + tc.marker + "'" + cause})
			if hit != tc.wantHit {
				t.Fatalf("routed singleton = %q, want %q", hit, tc.wantHit)
			}
		})
	}

	e := newEngineForErrorTest()
	hit := false
	e.singletons[singletonOpenOrders] = &route{
		request:      codec.OpenOrdersRequest{Scope: string(OpenOrdersScopeClient)},
		handleAPIErr: func(codec.APIError, *engine) { hit = true },
	}
	e.handleAPIError(codec.APIError{ReqID: -1, Code: 321, Message: "Error validating request.-'b7'" + cause})
	if hit {
		t.Fatal("late reqIds rejection was assigned to open orders")
	}
	select {
	case event := <-e.events.Chan():
		if event.Code != 321 {
			t.Fatalf("session event code = %d, want 321", event.Code)
		}
	default:
		t.Fatal("unowned reqIds rejection was not surfaced as a session event")
	}

	e = newEngineForErrorTest()
	hit = false
	e.singletons[singletonOpenOrders] = &route{
		request:      codec.OpenOrdersRequest{Scope: string(OpenOrdersScopeAll)},
		handleAPIErr: func(codec.APIError, *engine) { hit = true },
	}
	e.handleAPIError(codec.APIError{ReqID: -1, Code: 321, Message: "Error validating request.-'as'" + cause})
	if hit {
		t.Fatal("late client-scope refusal was assigned to all-scope open orders")
	}
	select {
	case event := <-e.events.Chan():
		if event.Code != 321 {
			t.Fatalf("session event code = %d, want 321", event.Code)
		}
	default:
		t.Fatal("unowned open-orders rejection was not surfaced as a session event")
	}
}

// TestHandleAPIErrorExerciseRouteShieldsCollidingOrder freezes the Bug 3
// id-collision invariant: when an exercise request id numerically collides
// with a live order id, the exercise's keyed route (checked before the order
// fallback) absorbs the refusal as a session event, and the unrelated order
// handle is left untouched.
func TestHandleAPIErrorExerciseRouteShieldsCollidingOrder(t *testing.T) {
	t.Parallel()

	e := newEngineForErrorTest()
	handle := newOrderHandle(5, 64)
	e.orders[5] = &orderRoute{orderID: 5, handle: handle}
	e.installExerciseRoute(5)

	e.handleAPIError(codec.APIError{
		ReqID:   5,
		Code:    ErrCodeServerErrorProcessingRequest,
		Message: "Error processing request.Exercise ignored because option is not in-the-money.",
	})

	if handle.isDone() {
		t.Fatal("exercise refusal closed the colliding order handle; keyed route must shield it")
	}
	select {
	case evt := <-e.events.Chan():
		if evt.Code != ErrCodeServerErrorProcessingRequest {
			t.Fatalf("session event code = %d, want %d", evt.Code, ErrCodeServerErrorProcessingRequest)
		}
		apiErr, ok := errors.AsType[*APIError](evt.Err)
		if !ok || apiErr.OpKind != OpExerciseOptions {
			t.Fatalf("session event err = %v, want exercise *APIError", evt.Err)
		}
	default:
		t.Fatal("exercise refusal did not surface as a session event")
	}
}

func TestExerciseRouteTerminalErrorDeletesKeyedRoute(t *testing.T) {
	t.Parallel()

	e := newEngineForErrorTest()
	e.installExerciseRoute(77)

	e.handleAPIError(codec.APIError{
		ReqID:   77,
		Code:    ErrCodeServerErrorProcessingRequest,
		Message: "Error processing request.Exercise ignored because option is not in-the-money.",
	})

	if _, ok := e.keyed[77]; ok {
		t.Fatal("exercise route retained after terminal exercise refusal")
	}
}

func TestExerciseRoutePresetNoticeStaysActive(t *testing.T) {
	t.Parallel()

	e := newEngineForErrorTest()
	e.installExerciseRoute(77)

	e.handleAPIError(codec.APIError{
		ReqID:   77,
		Code:    ErrCodeOrderTIFSetFromPreset,
		Message: "Order TIF was set to DAY based on order preset.",
	})

	if _, ok := e.keyed[77]; !ok {
		t.Fatal("exercise route deleted after non-terminal preset notice")
	}
}

// TestHandleAPIErrorRejectionDropsOrderRoute freezes the rejection-path half
// of the retention fix: a terminal placement rejection (code 201) must close
// the handle AND drop the route and its execution correlations. Pre-fix only
// status-terminal closes (via the drain window) deleted the route, so every
// rejected order leaked a route until reconnect.
func TestHandleAPIErrorRejectionDropsOrderRoute(t *testing.T) {
	t.Parallel()

	e := newEngineForErrorTest()
	handle := newOrderHandle(42, 64)
	e.orders[42] = &orderRoute{orderID: 42, handle: handle}
	e.execDeliveries["exec-42"] = &execDelivery{orderID: 42}

	e.handleAPIError(codec.APIError{
		ReqID:   42,
		Code:    ErrCodeOrderRejected,
		Message: "Order rejected - reason:",
	})

	apiErr, ok := errors.AsType[*APIError](handle.Wait())
	if !ok || apiErr.Code != ErrCodeOrderRejected {
		t.Fatalf("handle.Wait() = %v, want *APIError code %d", handle.Wait(), ErrCodeOrderRejected)
	}
	if _, retained := e.orders[42]; retained {
		t.Fatal("rejected order's route retained; want deleted with the rejection")
	}
	if _, retained := e.execDeliveries["exec-42"]; retained {
		t.Fatal("rejected order's execution correlations retained; want forgotten")
	}
}

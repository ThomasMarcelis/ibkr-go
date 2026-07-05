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
	handle := newOrderHandle(90)
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
	handle := newOrderHandle(90)
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
			if evt.Commission != nil {
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
	handle := newOrderHandle(90)
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
	if len(events) != 2 || events[0].Execution == nil || events[1].Commission == nil {
		t.Fatalf("handle events = %+v, want execution then flushed commission", events)
	}
	if events[1].Commission.ExecID != "exec-race-1" {
		t.Fatalf("flushed commission ExecID = %q, want exec-race-1", events[1].Commission.ExecID)
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
	handle := newOrderHandle(90)
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
	var last CommissionReport
	for {
		select {
		case evt := <-handle.Events():
			if evt.Commission != nil {
				comms++
				last = *evt.Commission
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

// TestHandleAPIErrorExerciseRouteShieldsCollidingOrder freezes the Bug 3
// id-collision invariant: when an exercise request id numerically collides
// with a live order id, the exercise's keyed route (checked before the order
// fallback) absorbs the refusal as a session event, and the unrelated order
// handle is left untouched.
func TestHandleAPIErrorExerciseRouteShieldsCollidingOrder(t *testing.T) {
	t.Parallel()

	e := newEngineForErrorTest()
	handle := newOrderHandle(5)
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
	handle := newOrderHandle(42)
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

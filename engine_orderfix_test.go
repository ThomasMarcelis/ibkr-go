package ibkr

import (
	"testing"

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
// execToOrder map records exactly the fills the handle has already seen, so a
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
	e.keyed[5] = &route{
		opKind:       OpExerciseOptions,
		handleAPIErr: func(m codec.APIError, e *engine) { e.emitEvent(m.Code, m.Message) },
	}

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
	default:
		t.Fatal("exercise refusal did not surface as a session event")
	}
}

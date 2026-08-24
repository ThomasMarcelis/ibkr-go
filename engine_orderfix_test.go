package ibkr

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
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
		ReqID:    -1,
		OrderID:  90,
		ExecID:   "exec-live-1",
		Shares:   "10",
		Price:    "150",
		Time:     "20260610-19:58:22",
		ClientID: "1",
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
		ReqID:    -1,
		OrderID:  90,
		ExecID:   "exec-live-1",
		Shares:   "10",
		Price:    "150",
		Time:     "20260610-19:58:22",
		ClientID: "1",
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
		ReqID:    -1,
		OrderID:  90,
		ExecID:   "exec-race-1",
		Shares:   "10",
		Price:    "150",
		Time:     "20260610-19:58:22",
		ClientID: "1",
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
		ReqID:    -1,
		OrderID:  90,
		ExecID:   "exec-upd-1",
		Shares:   "10",
		Price:    "150",
		Time:     "20260610-19:58:22",
		ClientID: "1",
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
	if !last.RealizedPnL.Equal(decimal.RequireFromString("42.10")) {
		t.Fatalf("last commission realizedPnL = %s, want 42.10", last.RealizedPnL)
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

	// All code-321 messages are exact current sv225 Gateway text from captures
	// 20260824T202844Z-req_ids, 20260824T202816Z-open_orders_empty, and
	// 20260824T202745Z-completed_orders.
	const cause = " : cause - The API interface is currently in Read-Only mode."
	for _, tc := range []struct {
		name    string
		marker  string
		wantHit string
	}{
		{name: "order ID", marker: "aa", wantHit: singletonOrderID},
		{name: "open orders", marker: "as", wantHit: singletonOpenOrders},
		{name: "completed orders", marker: "S", wantHit: singletonCompletedOrders},
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
			e.singletons[singletonCompletedOrders] = &route{handleAPIErr: func(codec.APIError, *engine) { hit = singletonCompletedOrders }}
			e.handleAPIError(codec.APIError{ReqID: -1, Code: 321, Message: "Error validating request.-'" + tc.marker + "'" + cause})
			if hit != tc.wantHit {
				t.Fatalf("routed singleton = %q, want %q", hit, tc.wantHit)
			}
		})
	}

	// Request IDs moves from classic marker b7 to protobuf marker aa at
	// server_version 213. This exact refusal is from the official SDK capture
	// with events SHA-256
	// 6e793d3f48bd609810aede9a4f483f44ab70ebf09cd4da5c9fa0e5ba8a79c9d3.
	e := newEngineForErrorTest()
	hit := false
	e.singletons[singletonOrderID] = &route{handleAPIErr: func(codec.APIError, *engine) { hit = true }}
	e.handleAPIError(codec.APIError{ReqID: -1, Code: 321, Message: "Error validating request.-'aa'" + cause})
	if !hit {
		t.Fatal("server_version 213 request-ID refusal was not routed to the active request")
	}

	e = newEngineForErrorTest()
	hit = false
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

// TestHandleAPIErrorRoutesFARefusalByCause freezes the exact Gateway
// server_version 211 refusal captured through the SDK on 2026-07-13
// (events.jsonl SHA-256
// 562c394c0570e39ebf34776710f9d7c146005144b96b990a00eb9a93e3b601ae).
// RequestFA has no request ID, and its protobuf operation marker differs from
// the classic marker, so the operation-specific cause is the routing identity.
func TestHandleAPIErrorRoutesFARefusalByCause(t *testing.T) {
	t.Parallel()

	e := newEngineForErrorTest()
	hit := false
	e.singletons[singletonFA] = &route{handleAPIErr: func(msg codec.APIError, _ *engine) {
		hit = msg.Code == 321
	}}
	e.handleAPIError(codec.APIError{
		ReqID:   -1,
		Code:    321,
		Message: "Error validating request.-'X' : cause - FA data operations ignored for non FA customers",
	})
	if !hit {
		t.Fatal("server_version 211 FA refusal was not routed to the active FA request")
	}
}

func TestCompletedOrdersReturnsExactLiveReadOnlyRefusal(t *testing.T) {
	// Capture 20260824T202745Z-completed_orders, server_version 225,
	// events.jsonl SHA-256
	// 359a65545562c495774d6782f64c9b4716296219917c1826533835690c7fa7df.
	e, peer := newObservedMarketDataEngine(t)
	e.serverVersion = 225
	result := make(chan error, 1)
	go func() {
		_, err := e.CompletedOrders(context.Background(), true)
		result <- err
	}()
	(<-e.cmds)()
	_ = readObservedFrame(t, peer)

	message, err := codec.Decode(225, liveCapturedFrame(t, "AAAAdAAAAMwI////////////ARDEx9SrgzQYwQIiWUVycm9yIHZhbGlkYXRpbmcgcmVxdWVzdC4tJ1MnIDogY2F1c2UgLSBUaGUgQVBJIGludGVyZmFjZSBpcyBjdXJyZW50bHkgaW4gUmVhZC1Pbmx5IG1vZGUu"))
	if err != nil {
		t.Fatalf("decode exact live completed-orders refusal: %v", err)
	}
	e.handleIncoming(message)

	apiErr, ok := errors.AsType[*APIError](<-result)
	if !ok || apiErr.OpKind != OpCompletedOrders || apiErr.Code != ErrCodeServerErrorValidatingRequest {
		t.Fatalf("CompletedOrders error = %#v, want typed code-321 completed-orders refusal", apiErr)
	}
	if _, ok := e.singletons[singletonCompletedOrders]; ok {
		t.Fatal("completed-orders refusal left the singleton route active")
	}
}

func TestFAConfigReturnsExactLiveNonFARefusal(t *testing.T) {
	// Capture 20260824T202844Z-request_fa, server_version 225,
	// events.jsonl SHA-256
	// 4406b8b95e03199f93d3a52d4022f2b79b72c06e7cb6447ff7ddfc1c207eb07b.
	e, peer := newObservedMarketDataEngine(t)
	e.serverVersion = 225
	result := make(chan error, 1)
	go func() {
		_, err := e.RequestFA(context.Background(), FADataGroups)
		result <- err
	}()
	(<-e.cmds)()
	_ = readObservedFrame(t, peer)

	message, err := codec.Decode(225, liveCapturedFrame(t, "AAAAcwAAAMwI////////////ARDDm9irgzQYwQIiWEVycm9yIHZhbGlkYXRpbmcgcmVxdWVzdC4tJ1gnIDogY2F1c2UgLSBGQSBkYXRhIG9wZXJhdGlvbnMgaWdub3JlZCBmb3Igbm9uIEZBIGN1c3RvbWVycy4="))
	if err != nil {
		t.Fatalf("decode exact live non-FA refusal: %v", err)
	}
	e.handleIncoming(message)

	apiErr, ok := errors.AsType[*APIError](<-result)
	if !ok || apiErr.OpKind != OpFAConfig || apiErr.Code != ErrCodeServerErrorValidatingRequest {
		t.Fatalf("Config error = %#v, want typed code-321 FA refusal", apiErr)
	}
	if _, ok := e.singletons[singletonFA]; ok {
		t.Fatal("FA refusal left the singleton route active")
	}
}

// TestExerciseRouteCloseOwnsOnlyItsPairedOrderRoute injects an impossible
// allocator collision after registration to freeze the ownership invariant:
// exercise teardown closes its exact paired handle without deleting whichever
// unrelated order route later occupies the same numeric ID.
func TestExerciseRouteCloseOwnsOnlyItsPairedOrderRoute(t *testing.T) {
	t.Parallel()

	e := newEngineForErrorTest()
	exerciseHandle := e.installExerciseRoute(5)
	unrelatedHandle := newOrderHandle(5, 64)
	unrelatedRoute := &orderRoute{orderID: 5, handle: unrelatedHandle}
	e.orders[5] = unrelatedRoute

	e.handleAPIError(codec.APIError{
		ReqID:   5,
		Code:    ErrCodeServerErrorProcessingRequest,
		Message: "Error processing request.Exercise ignored because option is not in-the-money.",
	})

	if e.orders[5] != unrelatedRoute || unrelatedHandle.isDone() {
		t.Fatal("exercise refusal changed the unrelated colliding order route")
	}
	apiErr, ok := errors.AsType[*APIError](exerciseHandle.Wait())
	if !ok || apiErr.Code != ErrCodeServerErrorProcessingRequest || apiErr.OpKind != OpExerciseOptions {
		t.Fatalf("exercise Wait() = %#v, want exercise code-322 APIError", apiErr)
	}
}

func TestExerciseRouteTerminalErrorDeletesKeyedRoute(t *testing.T) {
	t.Parallel()

	e := newEngineForErrorTest()
	handle := e.installExerciseRoute(77)

	e.handleAPIError(codec.APIError{
		ReqID:   77,
		Code:    ErrCodeServerErrorProcessingRequest,
		Message: "Error processing request.Exercise ignored because option is not in-the-money.",
	})

	if _, ok := e.keyed[77]; ok {
		t.Fatal("exercise route retained after terminal exercise refusal")
	}
	if _, ok := e.orders[77]; ok {
		t.Fatal("exercise order route retained after terminal exercise refusal")
	}
	waitErr := handle.Wait()
	if _, uncertain := errors.AsType[*ExerciseUncertainError](waitErr); uncertain {
		t.Fatalf("exercise Wait() = %v, definitive API rejection must not be uncertain", waitErr)
	}
	apiErr, ok := errors.AsType[*APIError](waitErr)
	if !ok || apiErr.Code != ErrCodeServerErrorProcessingRequest {
		t.Fatalf("exercise Wait() = %#v, want code-322 APIError", apiErr)
	}
}

func TestExerciseHandleExplicitCloseIsCleanDetach(t *testing.T) {
	t.Parallel()

	e := newEngineForErrorTest()
	e.cmds = make(chan func(), 1)
	handle := e.installExerciseRoute(77)
	handle.Close()
	select {
	case closeOnActor := <-e.cmds:
		closeOnActor()
	default:
		t.Fatal("ExerciseHandle.Close did not enqueue its detach")
	}

	if err := handle.Wait(); err != nil {
		t.Fatalf("exercise Wait() after Close = %v, want nil", err)
	}
	if _, ok := e.keyed[77]; ok {
		t.Fatal("closed exercise left the keyed route active")
	}
	if _, ok := e.orders[77]; ok {
		t.Fatal("closed exercise left the paired order route active")
	}
}

func TestExerciseRoutePresetNoticeStaysActive(t *testing.T) {
	t.Parallel()

	e := newEngineForErrorTest()
	handle := e.installExerciseRoute(77)

	e.handleAPIError(codec.APIError{
		ReqID:   77,
		Code:    ErrCodeOrderTIFSetFromPreset,
		Message: "Order TIF was set to DAY based on order preset.",
	})

	if _, ok := e.keyed[77]; !ok {
		t.Fatal("exercise route deleted after non-terminal preset notice")
	}
	if _, ok := e.orders[77]; !ok {
		t.Fatal("exercise order route deleted after non-terminal preset notice")
	}
	select {
	case event := <-handle.Events():
		if event.Warning == nil || event.Warning.Code != ErrCodeOrderTIFSetFromPreset {
			t.Fatalf("exercise event = %#v, want code-10349 warning", event)
		}
	default:
		t.Fatal("exercise preset notice was not delivered to handle")
	}
}

func TestExerciseRouteInterruptionIsUncertainAndNotRetryable(t *testing.T) {
	t.Parallel()

	e := newEngineForErrorTest()
	handle := e.installExerciseRoute(77)
	route := e.keyed[77]
	route.onDisconnect(e, io.EOF)

	waitErr := handle.Wait()
	uncertain, ok := errors.AsType[*ExerciseUncertainError](waitErr)
	if !ok || uncertain.RequestID != 77 {
		t.Fatalf("exercise Wait() = %T %v, want request 77 *ExerciseUncertainError", waitErr, waitErr)
	}
	if !errors.Is(waitErr, ErrInterrupted) || !errors.Is(waitErr, io.EOF) {
		t.Fatalf("exercise Wait() = %v, want ErrInterrupted and io.EOF", waitErr)
	}
	if _, nested := errors.AsType[*ExerciseUncertainError](uncertain.Err); nested {
		t.Fatalf("exercise uncertainty nested another uncertainty: %#v", uncertain.Err)
	}
	if IsRetryable(waitErr) {
		t.Fatal("uncertain admitted exercise is retryable")
	}
	if _, ok := e.keyed[77]; ok {
		t.Fatal("interrupted exercise route retained")
	}
	if _, ok := e.orders[77]; ok {
		t.Fatal("interrupted exercise order route retained")
	}
}

func TestReconnectOffClassifiesExerciseBeforeOrders(t *testing.T) {
	t.Parallel()

	e, peer := newObservedMarketDataEngine(t)
	e.cfg.reconnect = ReconnectOff
	handle := e.installExerciseRoute(77)

	e.handleTransportLoss(transportLoss{transport: e.transport, err: io.EOF})

	err := handle.Wait()
	if _, ok := errors.AsType[*ExerciseUncertainError](err); !ok {
		t.Fatalf("ExerciseHandle.Wait() error = %T %v, want *ExerciseUncertainError", err, err)
	}
	if errors.Is(err, ErrOrderRecoveryRequired) {
		t.Fatalf("exercise interruption was overwritten by order recovery: %v", err)
	}
	_ = peer.Close()
}

func TestExerciseRouteClientShutdownIsUncertain(t *testing.T) {
	t.Parallel()

	e := newEngineForErrorTest()
	handle := e.installExerciseRoute(77)
	e.keyed[77].close(ErrClosed)

	waitErr := handle.Wait()
	uncertain, ok := errors.AsType[*ExerciseUncertainError](waitErr)
	if !ok || uncertain.RequestID != 77 {
		t.Fatalf("exercise Wait() = %T %v, want request 77 *ExerciseUncertainError", waitErr, waitErr)
	}
	if !errors.Is(waitErr, ErrInterrupted) || !errors.Is(waitErr, ErrClosed) {
		t.Fatalf("exercise Wait() = %v, want ErrInterrupted and ErrClosed", waitErr)
	}
	if _, nested := errors.AsType[*ExerciseUncertainError](uncertain.Err); nested {
		t.Fatalf("exercise uncertainty nested another uncertainty: %#v", uncertain.Err)
	}
	if _, ok := e.keyed[77]; ok {
		t.Fatal("client shutdown left the exercise keyed route active")
	}
	if _, ok := e.orders[77]; ok {
		t.Fatal("client shutdown left the exercise order route active")
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

func TestHandleAPIErrorKeepsUnattestedOrderNotice(t *testing.T) {
	t.Parallel()

	e := newEngineForErrorTest()
	handle := newOrderHandle(42, 64)
	e.orders[42] = &orderRoute{orderID: 42, handle: handle}

	e.handleAPIError(codec.APIError{
		ReqID:   42,
		Code:    404,
		Message: "Order held while shares are located",
	})

	if _, retained := e.orders[42]; !retained {
		t.Fatal("unattested order notice detached the live route")
	}
	event := <-handle.Events()
	if event.Warning == nil || event.Warning.Code != 404 {
		t.Fatalf("order event = %#v, want code-404 warning", event)
	}
	handle.Close()
}

// TestLateOrderCancellationNoticeDoesNotHitKeyedRoute freezes capture
// 20260824T204700Z-api_cross_client_cancel_aapl at server_version 225,
// events.jsonl SHA-256
// 9d7fafb105cfe1cd1b12b46b6dcf00d983a20b4e3c71a14239dd441795b1f3c6.
// A code-202 notice for order ID 486 is replayed while a quote owns request ID
// 486. Cancellation-only codes belong to the
// order namespace and must not terminate the unrelated keyed request.
func TestLateOrderCancellationNoticeDoesNotHitKeyedRoute(t *testing.T) {
	t.Parallel()

	e := newEngineForErrorTest()
	keyedCalled := false
	e.keyed[486] = &route{
		handleAPIErr: func(codec.APIError, *engine) { keyedCalled = true },
	}

	message, err := codec.Decode(225, liveCapturedFrame(t, "AAAAKwAAAMwI5gMQiJObrIM0GMoBIhhPcmRlciBDYW5jZWxlZCAtIHJlYXNvbjo="))
	if err != nil {
		t.Fatalf("decode exact live code-202 frame: %v", err)
	}
	e.handleIncoming(message)

	if keyedCalled {
		t.Fatal("late order cancellation notice was routed to keyed request 486")
	}
	event := <-e.SessionEvents()
	if event.APIError == nil || event.APIError.Code != ErrCodeOrderCanceled || event.APIError.RequestID != 486 {
		t.Fatalf("session event = %+v, want order cancellation notice for id 486", event)
	}
}

func TestHandleAPIErrorKeepsRejectionAfterWorkingEvidence(t *testing.T) {
	t.Parallel()

	e := newEngineForErrorTest()
	handle := newOrderHandle(42, 64)
	e.orders[42] = &orderRoute{orderID: 42, handle: handle, working: true}

	e.handleAPIError(codec.APIError{
		ReqID:   42,
		Code:    ErrCodeOrderRejected,
		Message: "late order validation notice",
	})

	if _, retained := e.orders[42]; !retained {
		t.Fatal("late rejection detached an order with working evidence")
	}
	event := <-handle.Events()
	if event.Warning == nil || event.Warning.Code != ErrCodeOrderRejected {
		t.Fatalf("order event = %#v, want code-201 warning", event)
	}
	handle.Close()
}

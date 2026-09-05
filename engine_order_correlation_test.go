package ibkr

import (
	"errors"
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
)

// Exact MES callbacks at sv225, events.jsonl SHA-256:
// dd5eeefb0d5bb095dc3da778767570b5286f733491489cde66abcf70488bd005.
// Only local timing, callback ordering, and resource limits are injected.
func installCapturedOrder(t *testing.T, e *engine, execution codec.ExecutionDetail) *OrderHandle {
	t.Helper()
	clientID, err := strconv.ParseInt(execution.ClientID, 10, 32)
	if err != nil {
		t.Fatal(err)
	}
	e.cfg.clientID = ClientID(clientID)
	h := newOrderHandle(execution.OrderID, 16)
	e.orders[execution.OrderID] = &orderRoute{orderID: execution.OrderID, handle: h}
	return h
}

func TestOrderFeeSurvivesDelayedExecution(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sequence := capturedQueryReplaySequence(t)
		e, _ := newEngineForDispatchTest()
		e.cmds = make(chan func(), 8)
		e.done = make(chan struct{})
		execution := sequence[1].(codec.ExecutionDetail)
		h := installCapturedOrder(t, e, execution)
		e.routeCommissionReport(sequence[0].(codec.CommissionReport))
		// Inject an execution delayed beyond the old 750 ms expiry. synctest
		// advances virtual time and runs any pending actor callbacks first.
		<-time.After(time.Second)
		synctest.Wait()
		for len(e.cmds) > 0 {
			(<-e.cmds)()
		}
		e.dispatchExecutionToOrder(execution)
		h.closeWithErr(nil)
		events := []OrderEvent{}
		for event := range h.Events() {
			events = append(events, event)
		}
		if len(events) != 2 || events[0].Execution == nil || events[1].CommissionAndFees == nil {
			t.Fatalf("events = %+v, want execution then retained fee", events)
		}
	})
}

func TestOrderFeeReplayUsesOneSlotAndSurvivesReconnect(t *testing.T) {
	sequence := capturedQueryReplaySequence(t)
	execution := sequence[1].(codec.ExecutionDetail)
	fee := sequence[0].(codec.CommissionReport)
	e, _ := newEngineForDispatchTest()
	e.cfg.orderExecutionCorrelationLimit = 1
	h := installCapturedOrder(t, e, execution)
	for range 10000 {
		e.routeCommissionReport(fee)
	}
	if len(e.execDeliveries) != 1 || e.pendingOrderFees != 1 {
		t.Fatalf("retained IDs/fees = %d/%d", len(e.execDeliveries), e.pendingOrderFees)
	}
	e.disconnectRoutes(ErrInterrupted, true)
	e.requireOrderRecovery(2)
	e.dispatchExecutionToOrder(execution)
	e.routeCommissionReport(fee)
	if e.pendingOrderFees != 0 {
		t.Fatalf("pending fees after claim = %d", e.pendingOrderFees)
	}
	e.closeOrderRoute(execution.OrderID, e.orders[execution.OrderID], nil)
	executions, fees := 0, 0
	for event := range h.Events() {
		if event.Execution != nil {
			executions++
		}
		if event.CommissionAndFees != nil {
			fees++
		}
	}
	if executions != 1 || fees != 1 || h.Wait() != nil {
		t.Fatalf("executions/fees/Wait = %d/%d/%v", executions, fees, h.Wait())
	}
	if len(e.execDeliveries) != 0 || e.pendingOrderFees != 0 {
		t.Fatal("last handle retained correlations")
	}
	e.routeCommissionReport(fee)
	if len(e.execDeliveries) != 0 {
		t.Fatal("unobserved fee retained without any orders")
	}
}

func TestOrderCorrelationKnownOwnerOverflowIsLocal(t *testing.T) {
	executions := capturedExecutions(t)
	e, _ := newEngineForDispatchTest()
	e.cfg.orderExecutionCorrelationLimit = 1
	first := executions[0]
	var second codec.ExecutionDetail
	for _, execution := range executions {
		if execution.OrderID != first.OrderID {
			second = execution
			break
		}
	}
	if second.OrderID == 0 {
		t.Fatal("capture lacks second order")
	}
	h1 := installCapturedOrder(t, e, first)
	h2 := installCapturedOrder(t, e, second)
	e.dispatchExecutionToOrder(first)
	e.dispatchExecutionToOrder(second)
	if err := h2.Wait(); !errors.Is(err, ErrExecutionCorrelationOverflow) || !errors.Is(err, ErrOrderRecoveryRequired) || IsRetryable(err) {
		t.Fatalf("overflow: %v", err)
	}
	select {
	case <-h1.Done():
		t.Fatal("unrelated order closed")
	default:
	}
	if len(e.execDeliveries) != 1 || e.orders[first.OrderID] == nil {
		t.Fatal("surviving order lost correlation")
	}
	e.closeOrderRoute(first.OrderID, e.orders[first.OrderID], nil)
	if len(e.execDeliveries) != 0 {
		t.Fatal("claimed ID leaked")
	}
}

func TestOrderCorrelationUnknownFeeOverflowPreservesQueriesAndObserver(t *testing.T) {
	messages := capturedServerMessages(t, executionsCapturePath)
	reports := capturedCommissionsFrom(messages)
	executions := executionByID(capturedExecutionsFrom(messages))
	if len(reports) < 2 || reports[0].ExecID == reports[1].ExecID {
		t.Fatal("capture lacks distinct fees")
	}
	e, peer := newObservedExecutionEngine(t)
	e.cfg.orderExecutionCorrelationLimit = 1
	reqID, query := installObservedExecutionRoute(t, e, WithQueueSize(16))
	_ = readObservedFrame(t, peer)
	h1 := installCapturedOrder(t, e, executions[reports[0].ExecID])
	h2 := installCapturedOrder(t, e, executions[reports[1].ExecID])
	exercise := e.installExerciseRoute(10001) // Local route identity, no exercise instruction.
	observer := newSubscription[ExecutionEvent](subscriptionConfig{buffer: 4}, nil)
	e.executionEvents = &executionEventRoute{sub: observer}
	e.handleIncoming(reports[0])
	e.handleIncoming(reports[1])
	for _, h := range []*OrderHandle{h1, h2} {
		if err := h.Wait(); !errors.Is(err, ErrExecutionCorrelationOverflow) || !errors.Is(err, ErrOrderRecoveryRequired) {
			t.Fatalf("Wait: %v", err)
		}
	}
	err := exercise.Wait()
	if _, ok := errors.AsType[*ExerciseUncertainError](err); !ok || !errors.Is(err, ErrExecutionCorrelationOverflow) {
		t.Fatalf("exercise Wait: %v", err)
	}
	if len(e.orders) != 0 || len(e.execDeliveries) != 0 || e.pendingOrderFees != 0 || e.keyed[10001] != nil {
		t.Fatal("overflow leaked order/exercise state")
	}
	select {
	case <-e.transport.Stopping():
		t.Fatal("order overflow retired transport")
	default:
	}
	if e.keyed[reqID] == nil {
		t.Fatal("order overflow closed query")
	}
	e.keyed[reqID].handle(executions[reports[0].ExecID], e)
	updates := availableExecutionUpdates(query)
	if len(updates) != 2 || updates[0].Execution == nil || updates[1].CommissionAndFees == nil {
		t.Fatalf("query updates: %+v", updates)
	}
	for range 2 {
		if event := <-observer.Events(); event.Value.CommissionAndFees == nil {
			t.Fatal("passive observer lost fee")
		}
	}
	query.Close()
	(<-e.cmds)()
}

func TestOrderCorrelationPendingVersionLimitAndConsecutiveDedupe(t *testing.T) {
	reports := capturedCommissionsFrom(capturedServerMessages(t, executionsCapturePath))
	if len(reports) < 2 || reports[0] == reports[1] {
		t.Fatal("capture lacks distinct reports")
	}
	e, _ := newEngineForDispatchTest()
	execution := capturedExecutions(t)[0]
	h := installCapturedOrder(t, e, execution)
	e.cfg.orderExecutionCorrelationLimit = 3
	// Structural fault injection aliases two real captured reports to the same
	// delivery record, exercising A -> B -> A retention without inventing a
	// Gateway fee revision. No live same-ID revision has been captured yet.
	st := &execDelivery{}
	e.execDeliveries[reports[0].ExecID] = st
	e.execDeliveries[reports[1].ExecID] = st
	e.routeCommissionReport(reports[0])
	e.routeCommissionReport(reports[0])
	e.routeCommissionReport(reports[1])
	e.routeCommissionReport(reports[0])
	if len(st.pending) != 3 || e.pendingOrderFees != 3 || st.pending[2] != reports[0] {
		t.Fatal("lost pending version or retained duplicate")
	}
	e.routeCommissionReport(reports[0]) // Identical at capacity is still accepted.
	select {
	case <-h.Done():
		t.Fatal("duplicate consumed capacity")
	default:
	}
	e.routeCommissionReport(reports[1])
	if err := h.Wait(); !errors.Is(err, ErrExecutionCorrelationOverflow) || !errors.Is(err, ErrOrderRecoveryRequired) {
		t.Fatalf("pending overflow: %v", err)
	}
	if e.pendingOrderFees != 0 || len(e.execDeliveries) != 0 {
		t.Fatal("overflow retained pending versions")
	}
}

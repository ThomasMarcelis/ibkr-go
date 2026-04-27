package ibkr

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/synctest"
)

func TestOrderHandleStateChannelClosesWhenFull(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		handle := newOrderHandle(7)
		for i := 0; i < 12; i++ {
			handle.emitState(SubscriptionStateEvent{Kind: SubscriptionGap, ConnectionSeq: uint64(i + 1)})
		}
		if err := handle.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}

		var seqs []uint64
		for evt := range handle.Lifecycle() {
			if evt.Kind == SubscriptionGap {
				seqs = append(seqs, evt.ConnectionSeq)
			}
		}
		if len(seqs) != 8 {
			t.Fatalf("gap event count = %d, want 8", len(seqs))
		}
		for i, seq := range seqs {
			want := uint64(i + 5)
			if seq != want {
				t.Fatalf("seqs[%d] = %d, want %d (keep latest 8)", i, seq, want)
			}
		}
	})
}

func TestOrderHandleCloseSerializedWithEngineEmits(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newRunningEngineForOrderHandleTest(t)
		handle := newOrderHandle(101)
		bindOrderHandleForEngineTest(t, e, handle)

		for i := 0; i < 8; i++ {
			enqueueOrderHandleEmit(e, handle)
		}
		if err := handle.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		for i := 0; i < 8; i++ {
			enqueueOrderHandleEmit(e, handle)
		}

		synctest.Wait()
		select {
		case <-handle.Done():
		default:
			t.Fatal("OrderHandle.Done() did not close")
		}
		if err := handle.Wait(); err != nil {
			t.Fatalf("Wait() error = %v, want nil", err)
		}
	})
}

func TestOrderHandleCloseWhenEventsBufferFull(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newRunningEngineForOrderHandleTest(t)
		handle := newOrderHandle(102)
		bindOrderHandleForEngineTest(t, e, handle)

		for i := 0; i < cap(handle.events)+1; i++ {
			enqueueOrderHandleEmit(e, handle)
		}
		if err := handle.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}

		synctest.Wait()
		select {
		case <-handle.Done():
		default:
			t.Fatal("OrderHandle.Done() did not close after events buffer filled")
		}
		if err := handle.Wait(); !errors.Is(err, ErrSlowConsumer) {
			t.Fatalf("Wait() error = %v, want ErrSlowConsumer", err)
		}
	})
}

func TestOrderHandleEventsDrainAfterClose(t *testing.T) {
	t.Parallel()

	handle := newOrderHandle(103)
	if !handle.emitStatus(OrderStatusUpdate{OrderID: 103, Status: OrderStatusSubmitted}) {
		t.Fatal("emitStatus returned false, want true")
	}
	if !handle.emitExecution(Execution{OrderID: 103, ExecID: "exec-103"}) {
		t.Fatal("emitExecution returned false, want true")
	}

	handle.closeWithErr(nil)

	var events []OrderEvent
	for evt := range handle.Events() {
		events = append(events, evt)
	}
	if len(events) != 2 {
		t.Fatalf("drained %d events after close, want 2", len(events))
	}
	if events[0].Status == nil || events[0].Status.Status != OrderStatusSubmitted {
		t.Fatalf("first event = %#v, want Submitted status", events[0])
	}
	if events[1].Execution == nil || events[1].Execution.ExecID != "exec-103" {
		t.Fatalf("second event = %#v, want execution exec-103", events[1])
	}
	if err := handle.Wait(); err != nil {
		t.Fatalf("Wait() = %v, want nil", err)
	}
}

func TestIsTerminalOrderStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status OrderStatus
		want   bool
	}{
		{OrderStatusFilled, true},
		{OrderStatusCancelled, true},
		{OrderStatusApiCancelled, true},
		{OrderStatusInactive, true},
		{OrderStatusPendingCancel, false},
		{OrderStatusSubmitted, false},
	}
	for _, tt := range tests {
		if got := IsTerminalOrderStatus(tt.status); got != tt.want {
			t.Fatalf("IsTerminalOrderStatus(%s) = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func newRunningEngineForOrderHandleTest(t *testing.T) *engine {
	t.Helper()

	cfg := defaultConfig()
	e := &engine{
		cfg:         cfg,
		cmds:        make(chan func(), 256),
		incoming:    make(chan any, 256),
		adapterErr:  make(chan error, 8),
		ready:       make(chan error, 1),
		done:        make(chan struct{}),
		events:      newObserver[Event](cfg.eventBuffer),
		keyed:       make(map[int]*route),
		singletons:  make(map[string]*route),
		orders:      make(map[int64]*orderRoute),
		executions:  newExecutionCorrelator(),
		execToOrder: make(map[string]int64),
		nextReqID:   1,
		snapshot: Snapshot{
			State: StateReady,
		},
	}
	go e.run()

	t.Cleanup(func() {
		_ = e.Close()
		synctest.Wait()
		select {
		case <-e.Done():
		default:
			t.Fatal("engine did not close")
		}
	})

	return e
}

func bindOrderHandleForEngineTest(t *testing.T, e *engine, handle *OrderHandle) {
	t.Helper()

	orderID := handle.OrderID()
	done := make(chan struct{})
	e.enqueue(func() {
		handle.detachFn = func() {
			e.enqueue(func() {
				if or, ok := e.orders[orderID]; ok && !or.closed {
					or.closed = true
				}
				handle.closeWithErr(nil)
			})
		}
		e.orders[orderID] = &orderRoute{orderID: orderID, handle: handle}
		close(done)
	})

	synctest.Wait()
	select {
	case <-done:
	default:
		t.Fatal("order handle was not registered")
	}
}

func enqueueOrderHandleEmit(e *engine, handle *OrderHandle) {
	e.enqueue(func() {
		if or, ok := e.orders[handle.OrderID()]; ok && !or.closed {
			if !or.handle.emitOrder(OpenOrder{}) {
				or.closed = true
			}
		}
	})
}

// TestOrderHandleModifyRejectsMismatchedOrderID freezes the S3 fix: calling
// Modify with a non-zero Order.OrderID that does not match the handle's bound
// order ID is a misuse and must produce an explicit error rather than a
// silent override of the wire-level ID.
func TestOrderHandleModifyRejectsMismatchedOrderID(t *testing.T) {
	t.Parallel()

	handle := newOrderHandle(100)
	var modifyCalled bool
	handle.modifyFn = func(ctx context.Context, order Order) error {
		modifyCalled = true
		return nil
	}

	err := handle.Modify(context.Background(), Order{OrderID: 999, Action: Buy, OrderType: OrderTypeLimit})
	if err == nil {
		t.Fatal("Modify() with mismatched OrderID returned nil, want error")
	}
	if !strings.Contains(err.Error(), "999") || !strings.Contains(err.Error(), "100") {
		t.Errorf("Modify() error = %v, want message containing both 999 and 100", err)
	}
	if modifyCalled {
		t.Error("Modify() invoked modifyFn despite mismatched OrderID")
	}
}

// TestOrderHandleModifyAllowsMatchingOrderID verifies the guard does not
// block the legitimate case where the caller explicitly sets the handle's
// order ID.
func TestOrderHandleModifyAllowsMatchingOrderID(t *testing.T) {
	t.Parallel()

	handle := newOrderHandle(100)
	var gotOrder Order
	handle.modifyFn = func(ctx context.Context, order Order) error {
		gotOrder = order
		return nil
	}

	err := handle.Modify(context.Background(), Order{OrderID: 100, Action: Buy, OrderType: OrderTypeLimit})
	if err != nil {
		t.Fatalf("Modify() error = %v, want nil", err)
	}
	if gotOrder.OrderID != 100 {
		t.Errorf("modifyFn received OrderID = %d, want 100", gotOrder.OrderID)
	}
}

// TestOrderHandleModifyAllowsZeroOrderID preserves the ergonomic convention
// that callers can reuse a freshly constructed Order without threading the
// handle's ID through.
func TestOrderHandleModifyAllowsZeroOrderID(t *testing.T) {
	t.Parallel()

	handle := newOrderHandle(100)
	var gotOrder Order
	handle.modifyFn = func(ctx context.Context, order Order) error {
		gotOrder = order
		return nil
	}

	err := handle.Modify(context.Background(), Order{Action: Buy, OrderType: OrderTypeLimit})
	if err != nil {
		t.Fatalf("Modify() error = %v, want nil", err)
	}
	if gotOrder.OrderID != 0 {
		t.Errorf("modifyFn received OrderID = %d, want 0 (handler will inject real ID downstream)", gotOrder.OrderID)
	}
}

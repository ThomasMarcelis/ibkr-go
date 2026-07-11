package ibkr

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
)

func TestOrderHandleStateChannelClosesWhenFull(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		handle := newOrderHandle(7, 64)
		for i := 0; i < 12; i++ {
			handle.emitState(SubscriptionStateEvent{Kind: SubscriptionGap, ConnectionSeq: uint64(i + 1)})
		}
		handle.Close()

		var seqs []uint64
		for evt := range handle.Lifecycle() {
			if evt.Kind == SubscriptionGap {
				seqs = append(seqs, evt.ConnectionSeq)
			}
		}
		if len(seqs) != 7 {
			t.Fatalf("gap event count = %d, want 7 plus the terminal Closed event", len(seqs))
		}
		for i, seq := range seqs {
			want := uint64(i + 6)
			if seq != want {
				t.Fatalf("seqs[%d] = %d, want %d (keep latest 8)", i, seq, want)
			}
		}
	})
}

func TestOrderHandleCloseSerializedWithEngineEmits(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newRunningEngineForOrderHandleTest(t)
		handle := newOrderHandle(101, 64)
		bindOrderHandleForEngineTest(t, e, handle)

		for i := 0; i < 8; i++ {
			enqueueOrderHandleEmit(e, handle)
		}
		handle.Close()
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
		handle := newOrderHandle(102, 64)
		bindOrderHandleForEngineTest(t, e, handle)

		for i := 0; i < cap(handle.events)+1; i++ {
			enqueueOrderHandleEmit(e, handle)
		}
		handle.Close()

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

	handle := newOrderHandle(103, 64)
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

func TestOrderHandleCancelAfterSlowConsumerUsesStableOrderID(t *testing.T) {
	t.Parallel()

	handle := newOrderHandle(1, 1)
	var canceledOrderID int64
	handle.cancelFn = func(context.Context, cancelConfig) error {
		canceledOrderID = handle.OrderID()
		return nil
	}
	handle.closeWithErr(ErrSlowConsumer)

	if err := handle.Wait(); !errors.Is(err, ErrSlowConsumer) {
		t.Fatalf("Wait() error = %v, want ErrSlowConsumer", err)
	}
	if err := handle.Cancel(context.Background()); err != nil {
		t.Fatalf("Cancel() after observation overflow = %v", err)
	}
	if canceledOrderID != 1 {
		t.Fatalf("cancel coordinate = %d, want stable OrderID 1", canceledOrderID)
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
		cfg:            cfg,
		cmds:           make(chan func(), 256),
		incoming:       make(chan any, 256),
		transportErr:   make(chan transportLoss, 8),
		ready:          make(chan error, 1),
		done:           make(chan struct{}),
		events:         newObserver[Event](cfg.eventBuffer),
		keyed:          make(map[int]*route),
		singletons:     make(map[string]*route),
		orders:         make(map[int64]*orderRoute),
		execDeliveries: make(map[string]*execDelivery),
		nextReqID:      1,
		snapshot: Snapshot{
			State: StateReady,
		},
	}
	go e.run()

	t.Cleanup(func() {
		e.Close()
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
				if or, ok := e.orders[orderID]; ok {
					e.closeOrderRoute(orderID, or, nil)
					return
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

func TestOrderHandleReplaceAfterCloseReturnsErrClosed(t *testing.T) {
	t.Parallel()

	handle := newOrderHandle(100, 64)
	handle.replaceFn = func(ctx context.Context, order Order) error {
		t.Fatal("Replace invoked replaceFn after handle close")
		return nil
	}
	handle.closeWithErr(nil)

	if err := handle.Replace(context.Background(), Order{Action: ActionBuy, OrderType: OrderTypeLimit}); !errors.Is(err, ErrClosed) {
		t.Fatalf("Replace() after close = %v, want ErrClosed", err)
	}
}

func TestOrderHandleCloseDropsRouteAndExecMappings(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newRunningEngineForOrderHandleTest(t)
		handle := newOrderHandle(100, 64)
		bindOrderHandleForEngineTest(t, e, handle)

		closed := make(chan struct{})
		e.enqueue(func() {
			e.execDeliveries["exec-100"] = &execDelivery{orderID: 100}
			close(closed)
		})
		synctest.Wait()
		<-closed

		handle.Close()
		synctest.Wait()

		got := make(chan struct {
			route bool
			exec  bool
		}, 1)
		e.enqueue(func() {
			_, route := e.orders[100]
			_, exec := e.execDeliveries["exec-100"]
			got <- struct {
				route bool
				exec  bool
			}{route: route, exec: exec}
		})
		synctest.Wait()

		state := <-got
		if state.route {
			t.Fatal("OrderHandle.Close retained e.orders route")
		}
		if state.exec {
			t.Fatal("OrderHandle.Close retained execution mapping")
		}
		if err := handle.Wait(); err != nil {
			t.Fatalf("handle.Wait() = %v, want nil", err)
		}
	})
}

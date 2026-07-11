package ibkr

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// OrderHandle tracks a placed order's lifecycle. Events arrive via Events();
// lifecycle state changes (Gap, Resumed) arrive via Lifecycle(). Close() detaches
// the handle without cancelling the order. Cancel() sends a cancel request.
// If the lossless event queue fills, Wait returns [ErrSlowConsumer]. That ends
// only local observation: the live order may keep executing, and OrderID
// remains available for cancellation and reconciliation.
//
// A terminal order status does not close the handle: executions and fee reports
// can arrive later. Call Close when observation is no longer needed.
type OrderHandle struct {
	orderID int64
	events  chan OrderEvent
	state   *observer[SubscriptionStateEvent]
	done    chan struct{}

	closeOnce sync.Once
	err       error
	errMu     sync.Mutex

	cancelFn  func(context.Context, cancelConfig) error // set by engine, sends CancelOrder
	replaceFn func(context.Context, Order) error        // set by engine, sends PlaceOrder with same ID
	detachFn  func()                                    // set by engine, routes Close through the actor loop
}

func newOrderHandle(orderID int64, eventCapacity int) *OrderHandle {
	return &OrderHandle{
		orderID: orderID,
		events:  make(chan OrderEvent, eventCapacity),
		state:   newObserver[SubscriptionStateEvent](8),
		done:    make(chan struct{}),
	}
}

// OrderID returns the order ID bound to this handle.
func (h *OrderHandle) OrderID() int64 { return h.orderID }

// Events returns the channel of order events (open-order echoes, status
// updates, executions, commissions). It closes when the handle closes. Events
// are never silently dropped; queue overflow closes the handle with
// [ErrSlowConsumer] without changing the live order.
func (h *OrderHandle) Events() <-chan OrderEvent { return h.events }

// Lifecycle returns the channel of lifecycle transitions (gap, resume, close)
// for this order handle, distinct from the business events on Events.
func (h *OrderHandle) Lifecycle() <-chan SubscriptionStateEvent {
	return h.state.Chan()
}

// Done returns a channel closed when the handle has terminated. After it is
// closed, Wait reports the terminal error.
func (h *OrderHandle) Done() <-chan struct{} { return h.done }

// Wait blocks until the handle is explicitly closed or observation terminates
// with an error. A terminal order status alone does not complete Wait.
func (h *OrderHandle) Wait() error {
	<-h.done
	h.errMu.Lock()
	defer h.errMu.Unlock()
	return h.err
}

// Close initiates detachment of the handle. The order continues executing on
// the server. Events() and Lifecycle() channels close asynchronously on the
// engine goroutine, serialized with in-flight emits; use Done or Wait to
// observe completion.
func (h *OrderHandle) Close() {
	if h.detachFn != nil {
		h.detachFn()
		return
	}
	// Unbound handles only appear in tests; without an engine route there is no
	// concurrent protocol emitter, so direct teardown is safe.
	h.closeWithErr(nil)
}

// Cancel sends a cancel request for this order to the server. Options are only
// needed for operator-entered compliance metadata.
func (h *OrderHandle) Cancel(ctx context.Context, opts ...CancelOption) error {
	if h.cancelFn == nil {
		return fmt.Errorf("ibkr: order handle not connected")
	}
	cfg, err := applyCancelOptions(opts)
	if err != nil {
		return err
	}
	return h.cancelFn(ctx, cfg)
}

// Replace re-sends the complete order with the handle's bound order ID. The
// contract is fixed at placement time; omitted order fields reset to defaults.
func (h *OrderHandle) Replace(ctx context.Context, order Order) error {
	if h.replaceFn == nil {
		return fmt.Errorf("ibkr: order handle not connected")
	}
	if h.isDone() {
		return ErrClosed
	}
	return h.replaceFn(ctx, order)
}

func (h *OrderHandle) isDone() bool {
	select {
	case <-h.done:
		return true
	default:
		return false
	}
}

func (h *OrderHandle) emitEvent(evt OrderEvent) bool {
	select {
	case <-h.done:
		return false
	default:
	}

	select {
	case h.events <- evt:
		return true
	case <-h.done:
		return false
	default:
		h.closeWithErr(ErrSlowConsumer)
		return false
	}
}

func (h *OrderHandle) emitOrder(o OpenOrder) bool {
	return h.emitEvent(OrderEvent{OpenOrder: &o})
}

// IsTerminalOrderStatus reports whether a status represents a final order
// state. Live Gateway can still deliver execution or commission callbacks just
// after a terminal status, so the engine owns the final handle close.
func IsTerminalOrderStatus(status OrderStatus) bool {
	return status == OrderStatusFilled || status == OrderStatusCancelled || status == OrderStatusApiCancelled || status == OrderStatusInactive
}

func (h *OrderHandle) emitStatus(s OrderStatusUpdate) bool {
	return h.emitEvent(OrderEvent{Status: &s})
}

func (h *OrderHandle) emitExecution(exec Execution) bool {
	return h.emitEvent(OrderEvent{Execution: &exec})
}

func (h *OrderHandle) emitCommissionAndFees(report CommissionAndFeesReport) bool {
	return h.emitEvent(OrderEvent{CommissionAndFees: &report})
}

// emitWarning delivers a non-terminal, order-targeted notice without closing
// the handle. The order stays working at IB; the caller keeps consuming events.
func (h *OrderHandle) emitWarning(w *APIError) bool {
	return h.emitEvent(OrderEvent{Warning: w})
}

func (h *OrderHandle) emitState(evt SubscriptionStateEvent) {
	if h.isDone() {
		return
	}
	evt.Retryable = retryableSubscriptionState(evt)
	if evt.At.IsZero() {
		evt.At = time.Now().UTC()
	}
	h.state.EmitLatest(evt)
}

func (h *OrderHandle) closeWithErr(err error) {
	h.closeOnce.Do(func() {
		h.errMu.Lock()
		h.err = err
		h.errMu.Unlock()
		h.emitState(SubscriptionStateEvent{Kind: SubscriptionClosed, Err: err})
		// Close events before done so Done reports completion only after the
		// engine has stopped publishing business events. Consumers that need
		// every buffered event should range Events(), then call Wait().
		close(h.events)
		h.state.Close()
		close(h.done)
	})
}

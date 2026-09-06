package ibkr

import (
	"context"
	"fmt"
	"sync"
)

// OrderHandle tracks a placed order's business and lifecycle events through a
// single ordered Events stream. Close detaches the handle without cancelling
// the order. Cancel sends a cancel request.
// If the lossless event queue fills, Wait returns [ErrSlowConsumer]. That ends
// only local observation: the live order may keep executing, and OrderID
// remains available for cancellation and reconciliation.
//
// A terminal order status does not close the handle: executions and fee reports
// can arrive later. Call Close when observation is no longer needed.
// Execution and fee correlation is bounded separately by
// [WithOrderExecutionCorrelationLimit]; overflow ends affected observation
// with both [ErrExecutionCorrelationOverflow] and [ErrOrderRecoveryRequired].
type OrderHandle struct {
	orderID      int64
	events       chan OrderEvent
	done         chan struct{}
	acknowledged chan struct{}

	closeOnce sync.Once
	err       error
	errMu     sync.Mutex

	cancelFn  func(context.Context, cancelConfig) error // set by engine, sends CancelOrder
	replaceFn func(context.Context, Order) error        // set by engine, sends PlaceOrder with same ID
	detachFn  func()                                    // set by engine, routes Close through the actor loop
}

func newOrderHandle(orderID int64, eventCapacity int) *OrderHandle {
	return &OrderHandle{
		orderID:      orderID,
		events:       make(chan OrderEvent, eventCapacity),
		done:         make(chan struct{}),
		acknowledged: make(chan struct{}),
	}
}

// OrderID returns the order ID bound to this handle.
func (h *OrderHandle) OrderID() int64 { return h.orderID }

// Acknowledged closes once the Gateway supplies a successfully decoded and
// attributed open-order echo, status, or execution for this order. It records
// broker observation, not acceptance or current working state. Local writes
// and API warnings do not acknowledge an order. The signal survives reconnect
// but does not restore replacement after lost continuity.
//
// If observation ends first, this channel stays open: wait alongside Done and
// an observation deadline. Waiting here does not consume Events, which still
// needs a single reader to prevent queue overflow.
func (h *OrderHandle) Acknowledged() <-chan struct{} { return h.acknowledged }

// Only the actor records broker evidence, independently of queue delivery.
func (h *OrderHandle) acknowledge() {
	select {
	case <-h.acknowledged:
	default:
		close(h.acknowledged)
	}
}

// Events returns the channel of order events: open-order echoes, status
// updates, executions, commission-and-fees reports, warnings, bindings, and
// lifecycle changes. It closes when the handle closes. Events are never
// silently dropped; queue overflow closes the handle with [ErrSlowConsumer]
// without changing the live order.
func (h *OrderHandle) Events() <-chan OrderEvent { return h.events }

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
// the server. Events closes asynchronously on the engine goroutine, serialized
// with in-flight emits; use Done or Wait to
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
// needed for operator-entered compliance metadata. The context bounds admission;
// nil reports local queue admission, not broker cancellation.
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
// contract and parent are fixed at placement time; a conflicting nonzero
// ParentID is rejected, while omission preserves the placement-time parent.
// Other omitted order fields reset to defaults. The context bounds admission;
// nil reports local queue admission, not broker acceptance of the replacement.
// After a physical reconnect or data-lost restoration (code 1101), it
// permanently returns [ErrOrderRecoveryRequired] because account reconciliation
// cannot restore the handle's lost event history. A data-maintained 1100-to-1102
// gap preserves replacement. Cancellation remains available by stable OrderID.
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
// after a terminal status, so callers close the handle after collecting the
// trailing evidence they need.
func IsTerminalOrderStatus(status OrderStatus) bool {
	return status == OrderStatusFilled || status == OrderStatusCancelled || status == OrderStatusAPICancelled || status == OrderStatusInactive
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

func (h *OrderHandle) emitBinding(binding OrderBinding) bool {
	return h.emitEvent(OrderEvent{Binding: &binding})
}

// emitWarning delivers a non-terminal, order-targeted notice without closing
// the handle. The order stays working at IB; the caller keeps consuming events.
func (h *OrderHandle) emitWarning(w *APIError) bool {
	return h.emitEvent(OrderEvent{Warning: w})
}

func (h *OrderHandle) emitLifecycle(kind OrderLifecycleKind, connectionSeq uint64, err error) bool {
	return h.emitEvent(OrderEvent{Lifecycle: &OrderLifecycleEvent{
		Kind:          kind,
		ConnectionSeq: connectionSeq,
		Err:           err,
	}})
}

func (h *OrderHandle) closeWithErr(err error) {
	h.closeOnce.Do(func() {
		h.errMu.Lock()
		h.err = err
		h.errMu.Unlock()
		// Close events before done so Done reports completion only after the
		// engine has stopped publishing business events. Consumers that need
		// every buffered event should range Events(), then call Wait().
		close(h.events)
		close(h.done)
	})
}

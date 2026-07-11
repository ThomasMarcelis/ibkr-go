package ibkr

// ExerciseHandle observes one option exercise or lapse instruction. It has no
// cancel or replace methods because the live socket protocol does not provide
// an attested request-scoped control operation for an admitted instruction.
type ExerciseHandle struct {
	requestID int
	order     *OrderHandle
}

// RequestID returns the request ID used to correlate Gateway replies.
func (h *ExerciseHandle) RequestID() int { return h.requestID }

// Events returns warnings and pseudo-order lifecycle events for the instruction.
func (h *ExerciseHandle) Events() <-chan OrderEvent { return h.order.Events() }

// Lifecycle returns local observation state changes.
func (h *ExerciseHandle) Lifecycle() <-chan SubscriptionStateEvent { return h.order.Lifecycle() }

// Done closes when observation ends.
func (h *ExerciseHandle) Done() <-chan struct{} { return h.order.Done() }

// Wait blocks until observation ends and returns any request-scoped error.
func (h *ExerciseHandle) Wait() error { return h.order.Wait() }

// Close detaches local observation without changing the instruction at IBKR.
func (h *ExerciseHandle) Close() { h.order.Close() }

package ibkr

// ExerciseHandle observes one option exercise or lapse instruction after its
// request entered the client transport queue. Admission does not prove that
// IBKR accepted or settled the instruction. It has no cancel or replace methods
// because the live socket protocol does not provide an attested request-scoped
// control operation for an admitted instruction.
type ExerciseHandle struct {
	requestID int
	order     *OrderHandle
}

// RequestID returns the request ID used to correlate Gateway replies.
func (h *ExerciseHandle) RequestID() int { return h.requestID }

// Events returns warnings and pseudo-order lifecycle events for the
// instruction. Working-order evidence is observable progress, not proof of
// eventual exercise or lapse settlement.
func (h *ExerciseHandle) Events() <-chan OrderEvent { return h.order.Events() }

// Done closes when observation ends.
func (h *ExerciseHandle) Done() <-chan struct{} { return h.order.Done() }

// Wait blocks until observation ends and returns any request-scoped error. A
// nil result after Close means only that local observation detached cleanly.
// If the connection is lost while the instruction is unresolved, Wait returns
// a non-retryable [*ExerciseUncertainError]; reconcile the account or position
// independently instead of blindly resubmitting.
func (h *ExerciseHandle) Wait() error { return h.order.Wait() }

// Close detaches local observation without changing or settling the instruction
// at IBKR.
func (h *ExerciseHandle) Close() { h.order.Close() }

package ibkr

import "errors"

// ExerciseHandle observes one option exercise or lapse instruction after its
// request entered the client transport queue. Admission does not prove that
// IBKR accepted or settled the instruction. It has no cancel or replace methods
// because the live socket protocol does not provide an attested request-scoped
// control operation for an admitted instruction.
type ExerciseHandle struct {
	requestID RequestID
	order     *OrderHandle
}

// RequestID returns the request ID used to correlate Gateway replies.
func (h *ExerciseHandle) RequestID() RequestID { return h.requestID }

// Events returns warnings and pseudo-order lifecycle events for the
// instruction. Working-order evidence is observable progress, not proof of
// eventual exercise or lapse settlement.
func (h *ExerciseHandle) Events() <-chan OrderEvent { return h.order.Events() }

// Done closes when observation ends.
func (h *ExerciseHandle) Done() <-chan struct{} { return h.order.Done() }

// Wait blocks until observation ends and returns any request-scoped error. A
// nil result after Close means only that local observation detached cleanly.
// Any involuntary non-API end of observation after admission returns a
// non-retryable [*ExerciseUncertainError]; reconcile the account or position
// independently instead of blindly resubmitting. A definitive request-scoped
// [*APIError] remains unchanged.
func (h *ExerciseHandle) Wait() error {
	err := h.order.Wait()
	if err == nil {
		return nil
	}
	if _, ok := errors.AsType[*ExerciseUncertainError](err); ok {
		return err
	}
	if _, ok := errors.AsType[*APIError](err); ok {
		return err
	}
	return &ExerciseUncertainError{RequestID: h.requestID, Err: err}
}

// Close detaches local observation without changing or settling the instruction
// at IBKR.
func (h *ExerciseHandle) Close() { h.order.Close() }

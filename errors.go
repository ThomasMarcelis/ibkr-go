package ibkr

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Sentinel errors returned across the package. Match them with [errors.Is].
var (
	ErrNotReady                     = errors.New("ibkr: session not ready")                      // request issued before the session reached Ready
	ErrInterrupted                  = errors.New("ibkr: request interrupted")                    // in-flight request cut short by a connection loss; retryable
	ErrResumeRequired               = errors.New("ibkr: subscription resume required")           // subscription needs re-establishment after a gap; retryable
	ErrOrderRecoveryRequired        = errors.New("ibkr: order recovery required")                // live order state is uncertain; reconcile before a new retry, but the affected handle cannot replace again
	ErrRegulatorySnapshotUncertain  = errors.New("ibkr: regulatory snapshot outcome uncertain")  // admitted fee-bearing request lost its completion evidence; do not retry blindly
	ErrNoSnapshot                   = errors.New("ibkr: subscription has no snapshot boundary")  // AwaitSnapshot on a stream with no snapshot phase
	ErrSlowConsumer                 = errors.New("ibkr: slow consumer")                          // consumer fell behind and a bounded event queue overflowed
	ErrUnsupportedServerVersion     = errors.New("ibkr: unsupported server version")             // request requires a newer server_version than negotiated
	ErrClosed                       = errors.New("ibkr: closed")                                 // operation on a closed client
	ErrNoMatch                      = errors.New("ibkr: no contract match")                      // Qualify found no matching contract
	ErrAmbiguousContract            = errors.New("ibkr: ambiguous contract")                     // Qualify matched more than one contract
	ErrOperationActive              = errors.New("ibkr: operation already active")               // singleton operation already owns its response route
	ErrExecutionCorrelationOverflow = errors.New("ibkr: execution correlation limit exceeded")   // execution stream correlation state reached its configured bound
	ErrInboundFrameTooLarge         = errors.New("ibkr: inbound frame exceeds configured limit") // raw frame rejected before body allocation
)

// InboundFrameTooLargeError reports a raw inbound frame rejected from its
// four-byte header, before its body was allocated or read.
type InboundFrameTooLargeError struct {
	Size  uint32
	Limit int
}

func (e *InboundFrameTooLargeError) Error() string {
	return fmt.Sprintf("%v: size %d exceeds limit %d", ErrInboundFrameTooLarge, e.Size, e.Limit)
}

func (e *InboundFrameTooLargeError) Unwrap() error { return ErrInboundFrameTooLarge }

// ConnectError wraps a failure during the connection phase (dial, TLS
// negotiation, or protocol handshake).
type ConnectError struct {
	Op  string
	Err error
}

func (e *ConnectError) Error() string {
	return fmt.Sprintf("ibkr: connect %s: %v", e.Op, e.Err)
}

func (e *ConnectError) Unwrap() error {
	return e.Err
}

// ProtocolError wraps a framing or encoding error encountered while reading
// or writing TWS API messages on the wire.
type ProtocolError struct {
	Direction string
	Message   string
	Err       error
}

func (e *ProtocolError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("ibkr: protocol %s: %v", e.Direction, e.Err)
	}
	return fmt.Sprintf("ibkr: protocol %s %s: %v", e.Direction, e.Message, e.Err)
}

func (e *ProtocolError) Unwrap() error {
	return e.Err
}

func inboundProtocolError(message string, err error) error {
	return &ProtocolError{Direction: "inbound", Message: message, Err: err}
}

// APIError is an error or notification returned by TWS or IB Gateway. RequestID
// identifies the request when the callback is scoped; session notifications
// commonly use zero or -1. Code and Message preserve the Gateway payload.
type APIError struct {
	RequestID               RequestID
	Code                    int
	Message                 string
	AdvancedOrderRejectJSON string
	ServerTime              time.Time
	OpKind                  OpKind
	ConnectionSeq           uint64
}

func (e *APIError) Error() string {
	return fmt.Sprintf("ibkr: api %s code=%d conn=%d: %s", e.OpKind, e.Code, e.ConnectionSeq, e.Message)
}

// OrderRecoveryError reports a bracket placement failure after at least one
// place request entered the transport queue. OrderIDs contains every admitted
// order ID; their live state remains uncertain until reconciled with the
// Gateway. A nil CancelErr means every compensating cancellation also entered
// the queue, not that the Gateway acknowledged any cancellation. It matches
// [ErrOrderRecoveryRequired]; unwrap it to inspect the independent placement
// and cancellation-admission causes.
type OrderRecoveryError struct {
	OrderIDs     []int64
	PlacementErr error
	CancelErr    error
}

// ExerciseUncertainError reports that observation of an admitted option
// exercise or lapse ended involuntarily before a definitive request-scoped API
// outcome arrived. The instruction may already have reached IBKR and must not
// be retried blindly. RequestID is the identifier exposed by ExerciseHandle.
type ExerciseUncertainError struct {
	RequestID RequestID
	Err       error
}

func (e *ExerciseUncertainError) Error() string {
	return fmt.Sprintf("ibkr: exercise/lapse request %d outcome is uncertain: %v", e.RequestID, e.Err)
}

func (e *ExerciseUncertainError) Unwrap() error {
	return e.Err
}

// RegulatorySnapshotUncertainError reports that an admitted fee-bearing
// snapshot lost its definitive completion evidence. RequestID and ConnectionSeq
// identify the exact request and physical session that may have incurred the
// charge. The request must be reconciled before it is retried.
type RegulatorySnapshotUncertainError struct {
	RequestID     RequestID
	ConnectionSeq uint64
	Err           error
}

func (e *RegulatorySnapshotUncertainError) Error() string {
	return fmt.Sprintf(
		"ibkr: regulatory snapshot request %d on connection %d outcome is uncertain: %v",
		e.RequestID,
		e.ConnectionSeq,
		e.Err,
	)
}

func (e *RegulatorySnapshotUncertainError) Unwrap() error { return e.Err }

func (e *RegulatorySnapshotUncertainError) Is(target error) bool {
	return target == ErrRegulatorySnapshotUncertain
}

// SubscriptionCancelError reports that a subscription cancellation could not
// enter the active transport queue. The engine retires the owning connection
// generation so the remote subscription cannot survive into a replacement;
// wait for the client to become ready again before subscribing anew.
type SubscriptionCancelError struct {
	OpKind OpKind
	Err    error
}

func (e *SubscriptionCancelError) Error() string {
	return fmt.Sprintf(
		"ibkr: cancel %s subscription: %v; owning connection generation retired, wait for a ready replacement before subscribing again",
		e.OpKind,
		e.Err,
	)
}

func (e *SubscriptionCancelError) Unwrap() error {
	return e.Err
}

func newOrderRecoveryError(orderIDs []int64, placementErr, cancelErr error) *OrderRecoveryError {
	return &OrderRecoveryError{
		OrderIDs:     append([]int64(nil), orderIDs...),
		PlacementErr: placementErr,
		CancelErr:    cancelErr,
	}
}

func (e *OrderRecoveryError) Error() string {
	if e.CancelErr == nil {
		return fmt.Sprintf(
			"ibkr: order recovery required for IDs %v: placement failed: %v; cancellation requests admitted but not acknowledged",
			e.OrderIDs,
			e.PlacementErr,
		)
	}
	return fmt.Sprintf(
		"ibkr: order recovery required for IDs %v: placement failed: %v; cancellation admission failed: %v",
		e.OrderIDs,
		e.PlacementErr,
		e.CancelErr,
	)
}

func (e *OrderRecoveryError) Unwrap() []error {
	errs := make([]error, 0, 2)
	if e.PlacementErr != nil {
		errs = append(errs, e.PlacementErr)
	}
	if e.CancelErr != nil {
		errs = append(errs, e.CancelErr)
	}
	return errs
}

// Is reports whether target is [ErrOrderRecoveryRequired].
func (e *OrderRecoveryError) Is(target error) bool {
	return target == ErrOrderRecoveryRequired
}

// ValidationError is a client-side input validation failure caught before
// the request is sent to the Gateway.
type ValidationError struct {
	Field   string
	Value   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Value == "" {
		return fmt.Sprintf("ibkr: invalid %s: %s", e.Field, e.Message)
	}
	return fmt.Sprintf("ibkr: invalid %s %q: %s", e.Field, e.Value, e.Message)
}

func operationActive(operation string) error {
	return fmt.Errorf("%w: %s", ErrOperationActive, operation)
}

func interrupted(cause error) error {
	if cause == nil {
		return ErrInterrupted
	}
	return fmt.Errorf("%w: %w", ErrInterrupted, cause)
}

func resumeRequired(cause error) error {
	if cause == nil {
		return ErrResumeRequired
	}
	return fmt.Errorf("%w: %w", ErrResumeRequired, cause)
}

// IsRetryable reports whether retrying with backoff is safe and useful.
// Recovery and uncertainty errors, caller cancellation, local data loss, and
// non-pacing API errors are terminal. Otherwise [ErrNotReady], [ErrInterrupted],
// [ErrResumeRequired], pacing violations, and [ConnectError] values not caused
// by [ErrUnsupportedServerVersion] are retryable. Terminal conditions take
// precedence over retryable causes when errors are joined. All other errors
// return false.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	// An admitted exercise or lapse may already be working at IBKR. A transport
	// interruption cannot prove otherwise, so retry requires reconciliation.
	if _, ok := errors.AsType[*ExerciseUncertainError](err); ok {
		return false
	}
	// A failed cancellation retires its owning transport, but retrying the
	// cancellation itself is neither necessary nor useful.
	if _, ok := errors.AsType[*SubscriptionCancelError](err); ok {
		return false
	}
	// Correlation overflow is local data loss. Even if another joined cause is
	// transient, retrying the same unbounded result without raising the explicit
	// limit will fail again and can conceal the missing execution evidence.
	if errors.Is(err, ErrExecutionCorrelationOverflow) {
		return false
	}
	// Uncertain order state must be reconciled. Joining a transient connection
	// cause must not turn recovery into a blind retry.
	if errors.Is(err, ErrOrderRecoveryRequired) {
		return false
	}
	if errors.Is(err, ErrRegulatorySnapshotUncertain) {
		return false
	}
	if errors.Is(err, ErrInboundFrameTooLarge) {
		return false
	}
	// These errors describe a permanent session state, an invalid operation, or
	// a result that cannot become valid by repeating the same request.
	if errors.Is(err, ErrNoSnapshot) ||
		errors.Is(err, ErrSlowConsumer) ||
		errors.Is(err, ErrUnsupportedServerVersion) ||
		errors.Is(err, ErrClosed) ||
		errors.Is(err, ErrNoMatch) ||
		errors.Is(err, ErrAmbiguousContract) ||
		errors.Is(err, ErrOperationActive) {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if _, ok := errors.AsType[*ValidationError](err); ok {
		return false
	}
	if _, ok := errors.AsType[*ProtocolError](err); ok {
		return false
	}
	// Every API failure in a joined tree must be a pacing violation. Otherwise a
	// pacing sibling could incorrectly make a request rejection retryable.
	if containsNonPacingAPIError(err) {
		return false
	}
	if _, ok := errors.AsType[*ConnectError](err); ok {
		return true
	}
	if _, ok := errors.AsType[*APIError](err); ok {
		return true
	}
	if errors.Is(err, ErrNotReady) || errors.Is(err, ErrInterrupted) || errors.Is(err, ErrResumeRequired) {
		return true
	}
	return false
}

func containsNonPacingAPIError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr == nil || !apiErr.IsPacingViolation()
	}
	switch err := err.(type) {
	case interface{ Unwrap() []error }:
		for _, cause := range err.Unwrap() {
			if containsNonPacingAPIError(cause) {
				return true
			}
		}
	case interface{ Unwrap() error }:
		return containsNonPacingAPIError(err.Unwrap())
	}
	return false
}

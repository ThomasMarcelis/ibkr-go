package ibkr

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Sentinel errors returned across the package. Match them with [errors.Is].
var (
	ErrNotReady                 = errors.New("ibkr: session not ready")                     // request issued before the session reached Ready
	ErrInterrupted              = errors.New("ibkr: request interrupted")                   // in-flight request cut short by a connection loss; retryable
	ErrResumeRequired           = errors.New("ibkr: subscription resume required")          // subscription needs re-establishment after a gap; retryable
	ErrNoSnapshot               = errors.New("ibkr: subscription has no snapshot boundary") // AwaitSnapshot on a stream with no snapshot phase
	ErrSlowConsumer             = errors.New("ibkr: slow consumer")                         // consumer fell behind and a bounded event queue overflowed
	ErrUnsupportedServerVersion = errors.New("ibkr: unsupported server version")            // request requires a newer server_version than negotiated
	ErrClosed                   = errors.New("ibkr: closed")                                // operation on a closed client
	ErrNoMatch                  = errors.New("ibkr: no contract match")                     // Qualify found no matching contract
	ErrAmbiguousContract        = errors.New("ibkr: ambiguous contract")                    // Qualify matched more than one contract
	ErrNoSubscription           = errors.New("ibkr: no active subscription")                // RefreshOpen with no active open-orders subscription
	ErrOperationActive          = errors.New("ibkr: operation already active")              // singleton operation already owns its response route
)

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

// APIError is an error or notification returned by TWS or IB Gateway. RequestID
// identifies the request when the callback is scoped; session notifications
// commonly use zero or -1. Code and Message preserve the Gateway payload.
type APIError struct {
	RequestID               int
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
// the queue, not that the Gateway acknowledged any cancellation.
type OrderRecoveryError struct {
	OrderIDs     []int64
	PlacementErr error
	CancelErr    error
}

// SubscriptionCancelError reports that a subscription cancellation could not
// enter the active transport queue. The remote subscription may still be live
// on the current connection; recycle the client connection before creating a
// replacement subscription.
type SubscriptionCancelError struct {
	OpKind OpKind
	Err    error
}

func (e *SubscriptionCancelError) Error() string {
	return fmt.Sprintf(
		"ibkr: cancel %s subscription: %v; remote state is uncertain, recycle the client connection before subscribing again",
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

// IsRetryable reports whether retrying with backoff is safe and useful.
// It returns true for [ErrNotReady], [ErrInterrupted], [ErrResumeRequired],
// transient [ConnectError] values, and [APIError.IsPacingViolation]. It returns
// false for caller context cancellation, protocol and validation failures,
// ordinary API rejections, [ErrSlowConsumer], [ErrClosed],
// [OrderRecoveryError], and [SubscriptionCancelError]. The two recovery error
// types remain non-retryable even when they wrap a transient error because a
// blind retry could duplicate a live order or subscription.
func IsRetryable(err error) bool {
	return isRetryableError(err)
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	// Retrying an uncertain bracket can duplicate orders that are still live
	// at the Gateway, even when the underlying transport error is retryable.
	if _, ok := errors.AsType[*OrderRecoveryError](err); ok {
		return false
	}
	// A failed cancellation can leave the old remote stream live. Retrying by
	// subscribing again could duplicate it or consume another subscription
	// slot; callers must recycle the connection first.
	if _, ok := errors.AsType[*SubscriptionCancelError](err); ok {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if apiErr, ok := errors.AsType[*APIError](err); ok {
		return apiErr.IsPacingViolation()
	}
	if errors.Is(err, ErrNotReady) || errors.Is(err, ErrInterrupted) || errors.Is(err, ErrResumeRequired) {
		return true
	}
	if _, ok := errors.AsType[*ValidationError](err); ok {
		return false
	}
	if _, ok := errors.AsType[*ProtocolError](err); ok {
		return false
	}
	if _, ok := errors.AsType[*ConnectError](err); ok {
		return !errors.Is(err, ErrUnsupportedServerVersion)
	}
	return false
}

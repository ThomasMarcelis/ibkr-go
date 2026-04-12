package ibkr

import (
	"errors"
	"fmt"
)

var (
	ErrNotReady                 = errors.New("ibkr: session not ready")
	ErrInterrupted              = errors.New("ibkr: request interrupted")
	ErrResumeRequired           = errors.New("ibkr: subscription resume required")
	ErrNoSnapshot               = errors.New("ibkr: subscription has no snapshot boundary")
	ErrSlowConsumer             = errors.New("ibkr: slow consumer")
	ErrUnsupportedServerVersion = errors.New("ibkr: unsupported server version")
	ErrClosed                   = errors.New("ibkr: closed")
	ErrNoMatch                  = errors.New("ibkr: no contract match")
	ErrAmbiguousContract        = errors.New("ibkr: ambiguous contract")
)

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

type APIError struct {
	Code          int
	Message       string
	OpKind        OpKind
	ConnectionSeq uint64
}

func (e *APIError) Error() string {
	return fmt.Sprintf("ibkr: api %s code=%d conn=%d: %s", e.OpKind, e.Code, e.ConnectionSeq, e.Message)
}

// IsRetryable reports whether err represents a transient client/session
// condition that a caller may retry. IBKR API errors are server-side request
// rejections and are not retryable by default.
func IsRetryable(err error) bool {
	return isRetryableError(err)
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := errors.AsType[*APIError](err); ok {
		return false
	}
	return errors.Is(err, ErrInterrupted) || errors.Is(err, ErrResumeRequired)
}

func retryableSubscriptionState(evt SubscriptionStateEvent) bool {
	if evt.Kind == SubscriptionGap {
		return true
	}
	return evt.Kind == SubscriptionClosed && isRetryableError(evt.Err)
}

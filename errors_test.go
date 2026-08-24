package ibkr

import (
	"context"
	"errors"
	"io"
	"testing"
)

func TestIsRetryable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil"},
		{name: "not ready", err: ErrNotReady, want: true},
		{name: "interrupted wrapped", err: errors.Join(errors.New("send"), ErrInterrupted), want: true},
		{name: "resume required", err: ErrResumeRequired, want: true},
		{name: "order recovery required", err: ErrOrderRecoveryRequired},
		{name: "regulatory snapshot uncertain", err: ErrRegulatorySnapshotUncertain},
		{name: "regulatory uncertainty overrides interrupted", err: errors.Join(ErrInterrupted, ErrRegulatorySnapshotUncertain)},
		{name: "order recovery overrides interrupted", err: errors.Join(ErrInterrupted, ErrOrderRecoveryRequired)},
		{name: "transient connect", err: &ConnectError{Op: "bootstrap", Err: io.EOF}, want: true},
		{name: "unsupported connect", err: &ConnectError{Op: "handshake", Err: ErrUnsupportedServerVersion}},
		{name: "caller deadline wins over joined connect", err: errors.Join(context.DeadlineExceeded, &ConnectError{Op: "bootstrap", Err: io.EOF})},
		{name: "pacing api", err: &APIError{Code: ErrCodeMaxMessageRate}, want: true},
		{name: "historical pacing api", err: &APIError{Code: ErrCodeHistoricalDataService, Message: "Historical data request pacing violation"}, want: true},
		{name: "unsupported connect overrides pacing api", err: errors.Join(&APIError{Code: ErrCodeMaxMessageRate}, &ConnectError{Op: "handshake", Err: ErrUnsupportedServerVersion})},
		{name: "generic historical api", err: &APIError{Code: ErrCodeHistoricalDataService, Message: "Historical data service unavailable"}},
		{name: "order rejected api", err: &APIError{Code: ErrCodeOrderRejected}},
		{name: "non-pacing api overrides pacing api", err: errors.Join(&APIError{Code: ErrCodeMaxMessageRate}, &APIError{Code: ErrCodeOrderRejected})},
		{name: "non-pacing api overrides transient connect", err: errors.Join(&ConnectError{Op: "bootstrap", Err: io.EOF}, &APIError{Code: ErrCodeOrderRejected})},
		{name: "protocol", err: &ProtocolError{Direction: "decode", Err: io.ErrUnexpectedEOF}},
		{name: "protocol overrides interrupted", err: interrupted(&ProtocolError{Direction: "decode", Err: io.ErrUnexpectedEOF})},
		{name: "validation", err: &ValidationError{Field: "Contract", Message: "required"}},
		{name: "validation overrides resume required", err: errors.Join(ErrResumeRequired, &ValidationError{Field: "Contract", Message: "required"})},
		{name: "unsupported connect overrides interrupted", err: errors.Join(ErrInterrupted, &ConnectError{Op: "handshake", Err: ErrUnsupportedServerVersion})},
		{name: "slow consumer", err: ErrSlowConsumer},
		{name: "slow consumer overrides interrupted", err: errors.Join(ErrInterrupted, ErrSlowConsumer)},
		{name: "no snapshot overrides resume required", err: errors.Join(ErrResumeRequired, ErrNoSnapshot)},
		{name: "execution correlation overflow overrides interrupted", err: errors.Join(ErrInterrupted, ErrExecutionCorrelationOverflow)},
		{name: "inbound frame limit overrides interrupted", err: errors.Join(ErrInterrupted, &InboundFrameTooLargeError{Size: 9, Limit: 8})},
		{name: "closed", err: ErrClosed},
		{name: "closed overrides interrupted", err: errors.Join(ErrInterrupted, ErrClosed)},
		{name: "no match overrides interrupted", err: errors.Join(ErrInterrupted, ErrNoMatch)},
		{name: "ambiguous contract overrides interrupted", err: errors.Join(ErrInterrupted, ErrAmbiguousContract)},
		{name: "active operation overrides interrupted", err: errors.Join(ErrInterrupted, ErrOperationActive)},
		{name: "order recovery overrides interrupted", err: newOrderRecoveryError([]int64{1}, ErrInterrupted, nil)},
		{name: "exercise uncertainty overrides interrupted", err: &ExerciseUncertainError{RequestID: 7, Err: ErrInterrupted}},
		{name: "subscription recovery overrides interrupted", err: &SubscriptionCancelError{OpKind: OpQuotes, Err: ErrInterrupted}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsRetryable(tt.err); got != tt.want {
				t.Fatalf("IsRetryable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestOrderRecoveryErrorMatchesSentinelAndPreservesCauses(t *testing.T) {
	t.Parallel()

	placementErr := errors.New("placement admission failed")
	cancelErr := errors.New("cancellation admission failed")
	err := newOrderRecoveryError([]int64{41, 42}, placementErr, cancelErr)

	for _, target := range []error{ErrOrderRecoveryRequired, placementErr, cancelErr} {
		if !errors.Is(err, target) {
			t.Errorf("errors.Is(OrderRecoveryError, %v) = false", target)
		}
	}
	if unwrapped := err.Unwrap(); len(unwrapped) != 2 || unwrapped[0] != placementErr || unwrapped[1] != cancelErr {
		t.Fatalf("OrderRecoveryError.Unwrap() = %v, want placement and cancellation causes", unwrapped)
	}
}

func TestPrimarySentinelErrorsRenderOnOneLine(t *testing.T) {
	t.Parallel()

	cause := errors.New("connection reset")
	for _, tc := range []struct {
		name     string
		wrap     func(error) error
		sentinel error
		want     string
	}{
		{name: "interrupted", wrap: interrupted, sentinel: ErrInterrupted, want: "ibkr: request interrupted: connection reset"},
		{name: "resume required", wrap: resumeRequired, sentinel: ErrResumeRequired, want: "ibkr: subscription resume required: connection reset"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.wrap(cause)
			if got := err.Error(); got != tc.want {
				t.Fatalf("error text = %q, want %q", got, tc.want)
			}
			if !errors.Is(err, tc.sentinel) || !errors.Is(err, cause) {
				t.Fatalf("error %v does not preserve sentinel and cause", err)
			}
			if got := tc.wrap(nil); got != tc.sentinel {
				t.Fatalf("nil cause = %v, want exact sentinel %v", got, tc.sentinel)
			}
		})
	}
}

func TestInterruptedDoesNotWrapItsSentinelTwice(t *testing.T) {
	t.Parallel()

	protocolErr := &ProtocolError{Direction: "inbound", Err: io.ErrUnexpectedEOF}
	once := interrupted(protocolErr)
	if got := interrupted(once); got != once {
		t.Fatalf("interrupted(interrupted(cause)) = %v, want original error", got)
	}
}

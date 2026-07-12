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
		{name: "transient connect", err: &ConnectError{Op: "bootstrap", Err: io.EOF}, want: true},
		{name: "unsupported connect", err: &ConnectError{Op: "handshake", Err: ErrUnsupportedServerVersion}},
		{name: "caller deadline wins over joined connect", err: errors.Join(context.DeadlineExceeded, &ConnectError{Op: "bootstrap", Err: io.EOF})},
		{name: "pacing api", err: &APIError{Code: ErrCodeMaxMessageRate}, want: true},
		{name: "historical pacing api", err: &APIError{Code: ErrCodeHistoricalDataService, Message: "Historical data request pacing violation"}, want: true},
		{name: "generic historical api", err: &APIError{Code: ErrCodeHistoricalDataService, Message: "Historical data service unavailable"}},
		{name: "order rejected api", err: &APIError{Code: ErrCodeOrderRejected}},
		{name: "protocol", err: &ProtocolError{Direction: "decode", Err: io.ErrUnexpectedEOF}},
		{name: "validation", err: &ValidationError{Field: "Contract", Message: "required"}},
		{name: "slow consumer", err: ErrSlowConsumer},
		{name: "execution correlation overflow overrides interrupted", err: errors.Join(ErrInterrupted, ErrExecutionCorrelationOverflow)},
		{name: "closed", err: ErrClosed},
		{name: "order recovery overrides interrupted", err: newOrderRecoveryError([]int64{1}, ErrInterrupted, nil)},
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

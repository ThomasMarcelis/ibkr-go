package ibkr_test

import (
	"context"
	"errors"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
)

func TestRegulatorySnapshotServerFailureUncertainReplay(t *testing.T) {
	t.Parallel()

	// This is the sole authorized fee-bearing regulatory request. Replay its
	// captured result; never repeat the live request.
	client, host := newClient(t, "regulatory_snapshot_aapl_error.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	details, err := client.Contracts().Qualify(ctx, ibkr.Contract{
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	})
	if err != nil {
		t.Fatalf("Qualify() error = %v", err)
	}
	_, err = client.MarketData().RegulatorySnapshot(ctx, details.Contract)
	apiErr, ok := errors.AsType[*ibkr.APIError](err)
	if !ok {
		t.Fatalf("RegulatorySnapshot() error = %T %v, want *APIError", err, err)
	}
	if apiErr.RequestID != 2 || apiErr.Code != 0 || apiErr.Message != "Internal server error" || apiErr.OpKind != ibkr.OpQuotes {
		t.Fatalf("RegulatorySnapshot() error = %+v, want request 2 code 0 Internal server error", apiErr)
	}
	if !errors.Is(err, ibkr.ErrRegulatorySnapshotUncertain) || ibkr.IsRetryable(err) {
		t.Fatalf("RegulatorySnapshot() error = %v, want non-retryable uncertainty with the API cause", err)
	}
}

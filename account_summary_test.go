package ibkr

import (
	"context"
	"testing"
)

func TestAccountSummaryRejectsMissingTagsBeforeEnqueue(t *testing.T) {
	t.Parallel()

	e := &engine{}
	for _, tags := range [][]string{nil, {"NetLiquidation", " "}} {
		if _, err := e.SubscribeAccountSummary(context.Background(), AccountSummaryRequest{Tags: tags}); err == nil {
			t.Fatalf("SubscribeAccountSummary(Tags=%q) error = nil", tags)
		}
	}
}

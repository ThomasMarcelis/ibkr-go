package ibkr_test

import (
	"context"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
)

func TestPositionsSubscriptionSnapshotCompleteReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "positions_subscription.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.Accounts().SubscribePositions(ctx, ibkr.WithResumePolicy(ibkr.ResumeNever))
	if err != nil {
		t.Fatalf("SubscribePositions() error = %v", err)
	}
	var positions []ibkr.Position
	for {
		event := waitForEvent(t, sub.Events())
		if event.Err != nil {
			t.Fatalf("positions event error = %v", event.Err)
		}
		switch event.Kind {
		case ibkr.StreamData:
			positions = append(positions, event.Value)
		case ibkr.StreamSnapshotComplete:
			goto snapshotComplete
		}
	}

snapshotComplete:
	if len(positions) != 8 {
		t.Fatalf("positions before SnapshotComplete = %d, want 8", len(positions))
	}
	foundZeroMES := false
	for _, position := range positions {
		if position.Account != "DU9000001" {
			t.Fatalf("position account = %q, want sanitized account", position.Account)
		}
		foundZeroMES = foundZeroMES || position.Contract.LocalSymbol == "MESU6" && position.Position.IsZero()
	}
	if !foundZeroMES {
		t.Fatal("positions snapshot lacks the live-attested zero-quantity MESU6 row")
	}

	sub.Close()
	if err := sub.Wait(); err != nil {
		t.Fatalf("positions subscription Wait() error = %v", err)
	}
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime() fence error = %v", err)
	}
}

package ibkr_test

import (
	"context"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
)

func TestExecutionsEmptyServer201Replay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "executions_empty_sv201_live.txt")
	defer client.Close()
	defer waitHost(t, host)

	if got := client.Session().ServerVersion; got != 201 {
		t.Fatalf("Session().ServerVersion = %d, want 201", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	updates, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{})
	if err != nil {
		t.Fatalf("Executions() error = %v", err)
	}
	if len(updates.Executions) != 0 {
		t.Fatalf("Executions() returned %d updates, want live empty snapshot", len(updates.Executions))
	}
}

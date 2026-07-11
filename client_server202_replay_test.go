package ibkr_test

import (
	"context"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
)

func TestExecutionsZeroStrikeServer202Replay(t *testing.T) {
	restore := ibkr.SetAdvertisedServerVersionMaxForTest(202)
	defer restore()

	client, host := newClient(t, "executions_zero_strike_sv202_live.txt")
	defer client.Close()
	defer waitHost(t, host)

	if got := client.Session().ServerVersion; got != 202 {
		t.Fatalf("Session().ServerVersion = %d, want 202", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	executions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{})
	if err != nil {
		t.Fatalf("Executions() error = %v", err)
	}
	if len(executions.Executions) != 1 {
		t.Fatalf("Executions() = %+v, want one live execution", executions)
	}
	execution := executions.Executions[0]
	if execution.Contract.ConID != 265598 || execution.Contract.Strike == nil || !execution.Contract.Strike.IsZero() {
		t.Fatalf("Contract = %+v, want conId 265598 with zero strike", execution.Contract)
	}
	if execution.ExecID != "sanitized-sv202-buy-001" {
		t.Fatalf("ExecID = %q, want sanitized-sv202-buy-001", execution.ExecID)
	}
}

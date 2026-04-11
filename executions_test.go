package ibkr

import (
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
)

func TestExecutionCorrelatorDeliversBacklogToOverlappingRoutes(t *testing.T) {
	t.Parallel()

	c := newExecutionCorrelator()
	c.registerRoute(1, ExecutionsRequest{Account: "DU12345"})
	c.registerRoute(2, ExecutionsRequest{Account: "DU12345", Symbol: "AAPL"})

	ready := c.recordCommission(codec.CommissionReport{ExecID: "exec-aapl", Commission: "1.25", Currency: "USD", RealizedPNL: "0"})
	if len(ready) != 0 {
		t.Fatalf("recordCommission() ready len = %d, want 0", len(ready))
	}

	c.observeExecution(1, codec.ExecutionDetail{ReqID: 1, ExecID: "exec-aapl", Account: "DU12345", Symbol: "AAPL"})
	if got := c.undeliveredCommissions(1, "exec-aapl"); len(got) != 1 {
		t.Fatalf("undeliveredCommissions(route1) len = %d, want 1", len(got))
	}

	if got := c.undeliveredCommissions(2, "exec-aapl"); len(got) != 0 {
		t.Fatalf("undeliveredCommissions(route2 before execution) len = %d, want 0", len(got))
	}

	c.observeExecution(2, codec.ExecutionDetail{ReqID: 2, ExecID: "exec-aapl", Account: "DU12345", Symbol: "AAPL"})
	if got := c.undeliveredCommissions(2, "exec-aapl"); len(got) != 1 {
		t.Fatalf("undeliveredCommissions(route2 after execution) len = %d, want 1", len(got))
	}
}

func TestExecutionCorrelatorClearsDeliveredHistoryButPreservesRoutes(t *testing.T) {
	t.Parallel()

	c := newExecutionCorrelator()
	c.registerRoute(1, ExecutionsRequest{Account: "DU12345", Symbol: "AAPL"})

	c.observeExecution(1, codec.ExecutionDetail{ReqID: 1, ExecID: "exec-aapl", Account: "DU12345", Symbol: "AAPL"})
	c.recordCommission(codec.CommissionReport{ExecID: "exec-aapl", Commission: "1.25", Currency: "USD", RealizedPNL: "0"})
	if got := c.undeliveredCommissions(1, "exec-aapl"); len(got) != 1 {
		t.Fatalf("first undeliveredCommissions() len = %d, want 1", len(got))
	}

	state := c.execs["exec-aapl"]
	if state == nil {
		t.Fatal("exec state missing after delivery")
	}
	if len(state.commissions) != 0 {
		t.Fatalf("state.commissions len = %d, want 0 after clear", len(state.commissions))
	}
	if state.routes[1] == nil || !state.routes[1].seenExecution {
		t.Fatal("route state missing after history clear")
	}

	ready := c.recordCommission(codec.CommissionReport{ExecID: "exec-aapl", Commission: "0.75", Currency: "USD", RealizedPNL: "0"})
	if len(ready) != 1 || ready[0] != 1 {
		t.Fatalf("recordCommission() ready = %#v, want route 1", ready)
	}
	if got := c.undeliveredCommissions(1, "exec-aapl"); len(got) != 1 {
		t.Fatalf("second undeliveredCommissions() len = %d, want 1", len(got))
	}
}

func TestExecutionCorrelatorDropsClosedRoutesFromPendingBacklog(t *testing.T) {
	t.Parallel()

	c := newExecutionCorrelator()
	c.registerRoute(1, ExecutionsRequest{Account: "DU12345"})
	c.registerRoute(2, ExecutionsRequest{Account: "DU12345", Symbol: "AAPL"})

	c.recordCommission(codec.CommissionReport{ExecID: "exec-aapl", Commission: "1.25", Currency: "USD", RealizedPNL: "0"})
	c.observeExecution(1, codec.ExecutionDetail{ReqID: 1, ExecID: "exec-aapl", Account: "DU12345", Symbol: "AAPL"})
	if got := c.undeliveredCommissions(1, "exec-aapl"); len(got) != 1 {
		t.Fatalf("route1 undeliveredCommissions() len = %d, want 1", len(got))
	}

	c.unregisterRoute(2)
	state := c.execs["exec-aapl"]
	if state == nil {
		t.Fatal("exec state missing after route close")
	}
	if len(state.commissions) != 0 {
		t.Fatalf("state.commissions len = %d, want 0 after dropping closed route", len(state.commissions))
	}
}

func TestExecutionCorrelatorKeepsPreDetailBacklogWhenRouteCloses(t *testing.T) {
	t.Parallel()

	c := newExecutionCorrelator()
	c.registerRoute(1, ExecutionsRequest{Account: "DU12345"})
	c.registerRoute(2, ExecutionsRequest{Account: "DU12345", Symbol: "MSFT"})

	c.recordCommission(codec.CommissionReport{ExecID: "exec-aapl", Commission: "1.25", Currency: "USD", RealizedPNL: "0"})
	c.unregisterRoute(2)

	c.observeExecution(1, codec.ExecutionDetail{ReqID: 1, ExecID: "exec-aapl", Account: "DU12345", Symbol: "AAPL"})
	if got := c.undeliveredCommissions(1, "exec-aapl"); len(got) != 1 {
		t.Fatalf("undeliveredCommissions(route1 after close) len = %d, want 1", len(got))
	}
}

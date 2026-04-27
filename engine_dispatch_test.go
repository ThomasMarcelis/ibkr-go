package ibkr

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
)

// newEngineForDispatchTest builds a minimal engine suitable for exercising
// the per-order dispatch paths (routeCommissionReport, dispatchExecutionToOrder)
// without starting the actor loop or a transport. The returned buffer captures
// any slog output emitted during the test.
func newEngineForDispatchTest() (*engine, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	cfg := defaultConfig()
	cfg.logger = slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	e := &engine{
		cfg:         cfg,
		keyed:       make(map[int]*route),
		singletons:  make(map[string]*route),
		orders:      make(map[int64]*orderRoute),
		executions:  newExecutionCorrelator(),
		execToOrder: make(map[string]int64),
	}
	return e, buf
}

// TestRouteCommissionReportLogsAndDropsOnDecodeError freezes the W3 fix:
// a malformed CommissionReport routed to a live OrderHandle must not
// terminate the handle, and the decode failure must be surfaced via the
// configured slog logger so it is observable in production.
func TestRouteCommissionReportLogsAndDropsOnDecodeError(t *testing.T) {
	t.Parallel()

	e, logs := newEngineForDispatchTest()
	handle := newOrderHandle(42)
	e.orders[42] = &orderRoute{orderID: 42, handle: handle}
	e.execToOrder["exec-bad"] = 42

	// A malformed decimal (not the sentinel, not empty) is the one case
	// that can still reach the engine after W1: the codec accepted it but
	// fromCodecCommission rejects it.
	e.routeCommissionReport(sdkadapter.CommissionReport{
		ExecID:     "exec-bad",
		Commission: "not-a-number",
		Currency:   "USD",
	})

	if handle.isDone() {
		t.Fatal("order handle was closed on decode error; must remain live")
	}

	select {
	case evt, ok := <-handle.Events():
		t.Fatalf("handle emitted event (%+v, ok=%v) after decode failure; must drop", evt, ok)
	default:
		// expected: no event emitted.
	}

	got := logs.String()
	if !strings.Contains(got, "drop commission report on decode error") {
		t.Errorf("logger missing commission drop diagnostic: %s", got)
	}
	if !strings.Contains(got, "exec-bad") {
		t.Errorf("logger missing exec_id attribute: %s", got)
	}
	if !strings.Contains(got, "order_id=42") {
		t.Errorf("logger missing order_id attribute: %s", got)
	}
}

// TestRouteCommissionReportDeliversValidReport confirms the happy path still
// emits the commission to the order handle.
func TestRouteCommissionReportDeliversValidReport(t *testing.T) {
	t.Parallel()

	e, logs := newEngineForDispatchTest()
	handle := newOrderHandle(42)
	e.orders[42] = &orderRoute{orderID: 42, handle: handle}
	e.execToOrder["exec-ok"] = 42

	e.routeCommissionReport(sdkadapter.CommissionReport{
		ExecID:      "exec-ok",
		Commission:  "1.25",
		Currency:    "USD",
		RealizedPNL: "0",
	})

	select {
	case evt, ok := <-handle.Events():
		if !ok {
			t.Fatal("Events() closed unexpectedly")
		}
		if evt.Commission == nil {
			t.Fatal("expected Commission event, got nil")
		}
		if evt.Commission.ExecID != "exec-ok" {
			t.Errorf("Commission.ExecID = %q, want %q", evt.Commission.ExecID, "exec-ok")
		}
	default:
		t.Fatal("handle received no commission event for a valid report")
	}

	if got := logs.String(); strings.Contains(got, "drop commission report") {
		t.Errorf("logger emitted drop warning for a valid report: %s", got)
	}
}

// TestDispatchExecutionToOrderLogsAndDropsOnDecodeError freezes the symmetric
// W3 fix for execution dispatch: a malformed ExecutionDetail routed to a live
// OrderHandle must be logged and dropped without tearing the handle down.
func TestDispatchExecutionToOrderLogsAndDropsOnDecodeError(t *testing.T) {
	t.Parallel()

	e, logs := newEngineForDispatchTest()
	handle := newOrderHandle(77)
	e.orders[77] = &orderRoute{orderID: 77, handle: handle}

	// Malformed Time field makes fromCodecExecution fail deterministically.
	e.dispatchExecutionToOrder(sdkadapter.ExecutionDetail{
		ReqID:   1,
		OrderID: 77,
		ExecID:  "exec-bad-time",
		Shares:  "1",
		Price:   "150",
		Time:    "not-a-timestamp",
	})

	if handle.isDone() {
		t.Fatal("order handle was closed on decode error; must remain live")
	}

	select {
	case evt, ok := <-handle.Events():
		t.Fatalf("handle emitted event (%+v, ok=%v) after decode failure; must drop", evt, ok)
	default:
		// expected: no event emitted.
	}

	if _, ok := e.execToOrder["exec-bad-time"]; ok {
		t.Error("execToOrder mapping was populated despite decode failure")
	}

	got := logs.String()
	if !strings.Contains(got, "drop execution detail on decode error") {
		t.Errorf("logger missing execution drop diagnostic: %s", got)
	}
	if !strings.Contains(got, "exec-bad-time") {
		t.Errorf("logger missing exec_id attribute: %s", got)
	}
	if !strings.Contains(got, "order_id=77") {
		t.Errorf("logger missing order_id attribute: %s", got)
	}
}

package ibkr

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
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
		cfg:            cfg,
		keyed:          make(map[int]*route),
		singletons:     make(map[string]*route),
		orders:         make(map[int64]*orderRoute),
		execDeliveries: make(map[string]*execDelivery),
	}
	return e, buf
}

// TestRouteCommissionReportClosesObservationOnDecodeError freezes the
// lossless local stream contract: a malformed report terminates observation
// without cancelling the live order.
func TestRouteCommissionReportClosesObservationOnDecodeError(t *testing.T) {
	t.Parallel()

	e, logs := newEngineForDispatchTest()
	handle := newOrderHandle(42, 64)
	e.orders[42] = &orderRoute{orderID: 42, handle: handle}
	e.execDeliveries["exec-bad"] = &execDelivery{orderID: 42}

	// A malformed decimal (not the sentinel, not empty) is the one case
	// that can still reach the engine after W1: the codec accepted it but
	// fromCodecCommission rejects it.
	e.routeCommissionReport(codec.CommissionReport{
		ExecID:     "exec-bad",
		Commission: "not-a-number",
		Currency:   "USD",
	})

	err := handle.Wait()
	protocolErr, ok := errors.AsType[*ProtocolError](err)
	if !ok || protocolErr.Direction != "inbound" {
		t.Fatalf("Wait() = %#v, want inbound ProtocolError", err)
	}
	if _, ok := e.orders[42]; ok {
		t.Fatal("commission projection failure retained the local order route")
	}
	if got := logs.String(); strings.Contains(got, "cancel") {
		t.Errorf("local projection failure logged a broker cancellation: %s", got)
	}
}

// TestRouteCommissionReportDeliversValidReport confirms the happy path still
// emits the commission to the order handle.
func TestRouteCommissionReportDeliversValidReport(t *testing.T) {
	t.Parallel()

	e, logs := newEngineForDispatchTest()
	handle := newOrderHandle(42, 64)
	e.orders[42] = &orderRoute{orderID: 42, handle: handle}
	e.execDeliveries["exec-ok"] = &execDelivery{orderID: 42}

	e.routeCommissionReport(codec.CommissionReport{
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
		if evt.CommissionAndFees == nil {
			t.Fatal("expected Commission event, got nil")
		}
		if evt.CommissionAndFees.ExecID != "exec-ok" {
			t.Errorf("Commission.ExecID = %q, want %q", evt.CommissionAndFees.ExecID, "exec-ok")
		}
	default:
		t.Fatal("handle received no commission event for a valid report")
	}

	if got := logs.String(); strings.Contains(got, "drop commission report") {
		t.Errorf("logger emitted drop warning for a valid report: %s", got)
	}
}

// TestDispatchExecutionToOrderClosesObservationOnDecodeError freezes the
// symmetric lossless-stream behavior for execution projection failures.
func TestDispatchExecutionToOrderClosesObservationOnDecodeError(t *testing.T) {
	t.Parallel()

	e, logs := newEngineForDispatchTest()
	handle := newOrderHandle(77, 64)
	e.orders[77] = &orderRoute{orderID: 77, handle: handle}

	// Malformed Time field makes fromCodecExecution fail deterministically.
	e.dispatchExecutionToOrder(codec.ExecutionDetail{
		ReqID:   1,
		OrderID: 77,
		ExecID:  "exec-bad-time",
		Shares:  "1",
		Price:   "150",
		Time:    "not-a-timestamp",
	})

	err := handle.Wait()
	protocolErr, ok := errors.AsType[*ProtocolError](err)
	if !ok || protocolErr.Direction != "inbound" {
		t.Fatalf("Wait() = %#v, want inbound ProtocolError", err)
	}
	if _, ok := e.orders[77]; ok {
		t.Fatal("execution projection failure retained the local order route")
	}
	if got := logs.String(); strings.Contains(got, "cancel") {
		t.Errorf("local projection failure logged a broker cancellation: %s", got)
	}
}

package ibkr

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
)

// Exact sv225 callbacks from api_reconnect_active_order_aapl.txt, SHA-256
// bb1d3c6803a7140fd4636570f43e91f047f57177e280b10b602f4786ace980a9.
// Fault injection delays local write completion until a captured callback
// arrives with the handle's queue already full. The sibling stays healthy.
func TestOrderStartOverflowPreservesOpenOrdersObserver(t *testing.T) {
	messages := capturedServerMessages(t, "testdata/transcripts/api_reconnect_active_order_aapl.txt")
	seen := make(map[string]bool)
	for _, message := range messages {
		var name, client string
		var id int64
		switch m := message.(type) {
		case codec.OpenOrder:
			name, client, id = "open", m.ClientID, m.OrderID
		case codec.OrderStatus:
			name, client, id = "status", m.ClientID, m.OrderID
		default:
			continue
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		t.Run(name, func(t *testing.T) {
			e, _ := newEngineForDispatchTest()
			clientID, err := strconv.ParseInt(client, 10, 32)
			if err != nil {
				t.Fatal(err)
			}
			e.cfg.clientID = ClientID(clientID)
			h := newOrderHandle(id, 1)
			h.emitLifecycle(OrderGap, 1, ErrInterrupted)
			e.orders[id] = &orderRoute{orderID: id, handle: h, pendingWrite: transportWriteKey{id: 1}}
			delivered := 0
			e.singletons[singletonOpenOrders] = &route{handle: func(any, *engine) { delivered++ }}
			e.handleIncoming(message)
			select {
			case <-h.Acknowledged():
			default:
				t.Fatal("broker evidence was lost with queue delivery")
			}
			if !errors.Is(h.Wait(), ErrSlowConsumer) || delivered != 1 {
				t.Fatalf("handle = %v; sibling deliveries = %d, want slow consumer and one delivery", h.Wait(), delivered)
			}
			if e.orders[id] != nil || e.singletons[singletonOpenOrders] == nil {
				t.Fatal("overflow damaged independent route ownership")
			}
		})
	}
	if len(seen) != 2 {
		t.Fatal("capture lacks both callback families")
	}
}

func TestOrderAcknowledgementPreservesEvidenceAcrossReconnect(t *testing.T) {
	messages := capturedServerMessages(t, "testdata/transcripts/api_reconnect_active_order_aapl.txt")
	messages = append(messages, capturedExecutions(t)[0])
	seen := make(map[string]bool)
	for _, message := range messages {
		var kind, client string
		var id int64
		switch m := message.(type) {
		case codec.OpenOrder:
			kind, client, id = "open", m.ClientID, m.OrderID
		case codec.OrderStatus:
			kind, client, id = "status", m.ClientID, m.OrderID
		case codec.ExecutionDetail:
			kind, client, id = "execution", m.ClientID, m.OrderID
		default:
			continue
		}
		if seen[kind] {
			continue
		}
		seen[kind] = true
		t.Run(kind, func(t *testing.T) {
			e, _ := newEngineForDispatchTest()
			clientID, err := strconv.ParseInt(client, 10, 32)
			if err != nil {
				t.Fatal(err)
			}
			e.cfg.clientID = ClientID(clientID)
			h := newOrderHandle(id, 16)
			e.orders[id] = &orderRoute{orderID: id, handle: h}
			h.emitLifecycle(OrderStarted, 1, nil)
			select {
			case <-h.Acknowledged():
				t.Fatal("local write counted as acknowledgement")
			default:
			}
			// Foreign attribution must not acknowledge even with the same ID.
			e.cfg.clientID++
			e.handleIncoming(message)
			select {
			case <-h.Acknowledged():
				t.Fatal("foreign callback acknowledged local order")
			default:
			}
			e.cfg.clientID--
			e.handleIncoming(message)
			e.handleIncoming(message)
			select {
			case <-h.Acknowledged():
			default:
				t.Fatal("attributed broker evidence did not acknowledge")
			}
			e.disconnectRoutes(ErrInterrupted, true)
			e.requireOrderRecovery(2)
			if !e.orders[id].recoveryRequired {
				t.Fatal("acknowledgement restored replacement safety")
			}
			e.closeOrderRoute(id, e.orders[id], nil)
			select {
			case <-h.Acknowledged():
			default:
				t.Fatal("closure discarded past acknowledgement")
			}
		})
	}
	if len(seen) != 3 {
		t.Fatal("captures lack acknowledgement families")
	}
}

func TestCapturedPreEchoWarningDoesNotAcknowledgeOrReject(t *testing.T) {
	// Exact sv225 stop-order sequence: code 399 precedes the first echo.
	messages := capturedServerMessages(t, "testdata/transcripts/api_order_stop_cancel_aapl.txt")
	observed := make(map[int64]bool)
	for _, message := range messages {
		switch m := message.(type) {
		case codec.OpenOrder:
			observed[m.OrderID] = true
		case codec.OrderStatus:
			observed[m.OrderID] = true
		case codec.APIError:
			if m.Code != ErrCodeOrderMessage || m.ReqID <= 0 || observed[int64(m.ReqID)] {
				continue
			}
			e, _ := newEngineForDispatchTest()
			id := int64(m.ReqID)
			h := newOrderHandle(id, 8)
			e.orders[id] = &orderRoute{orderID: id, handle: h}
			e.handleIncoming(m)
			select {
			case <-h.Done():
				t.Fatalf("warning rejected order: %v", h.Wait())
			case <-h.Acknowledged():
				t.Fatal("warning counted as acknowledgement")
			default:
			}
			event := <-h.Events()
			if event.Warning == nil || event.Warning.Code != m.Code {
				t.Fatalf("warning = %+v", event)
			}
			e.closeOrderRoute(id, e.orders[id], nil)
			select {
			case <-h.Acknowledged():
				t.Fatal("closing an unacknowledged handle fabricated acknowledgement")
			default:
			}
			return
		}
	}
	t.Fatal("capture lacks a warning before its first order echo")
}

func TestDirectCancelAcceptsRecoveredSameClientTarget(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	e.bootstrap.readyReported = true
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	// Order 493 and its exact cancel frame are retained by the cross-client
	// capture. This injects only local ownership after a process restart.
	go func() {
		result <- (OrdersClient{engine: e}).Cancel(ctx, OrderTarget{ClientID: e.cfg.clientID, OrderID: 493})
	}()
	(<-e.cmds)()
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if len(e.orders) != 0 {
		t.Fatal("direct cancellation manufactured a handle")
	}
	frame := readObservedFrame(t, peer)
	// The peer helper returns the body, without its four-byte length prefix.
	want, err := base64.StdEncoding.DecodeString("AAAACQAAAMwI7QMSAA==")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(frame, want[4:]) {
		t.Fatalf("recovered cancel wire body = %x", frame)
	}
}

// MES capture provenance is shared with engine_order_correlation_test.go.
// Only local handle ownership, the resource limit, and callback ordering vary.
func TestForeignExecutionFeesDoNotExhaustOrderCorrelation(t *testing.T) {
	messages := capturedServerMessages(t, executionsCapturePath)
	executions := executionByID(capturedExecutionsFrom(messages))
	reports := capturedCommissionsFrom(messages)
	owned := executions[reports[0].ExecID]
	for _, feeFirst := range []bool{false, true} {
		t.Run(strconv.FormatBool(feeFirst), func(t *testing.T) {
			e, _ := newEngineForDispatchTest()
			e.cfg.orderExecutionCorrelationLimit = 1
			h := newOrderHandle(maxWireOrderID, 16)
			e.orders[maxWireOrderID] = &orderRoute{orderID: maxWireOrderID, handle: h}
			foreign := make(map[string]bool)
			for _, report := range reports {
				execution := executions[report.ExecID]
				if foreign[report.ExecID] {
					continue
				}
				foreign[report.ExecID] = true
				if feeFirst {
					e.routeCommissionReport(report)
				}
				e.dispatchExecutionToOrder(execution)
				e.routeCommissionReport(report)
				select {
				case <-h.Done():
					t.Fatalf("foreign traffic closed owned handle: %v", h.Wait())
				default:
				}
				if len(e.execDeliveries) != 0 || e.pendingOrderFees != 0 {
					t.Fatal("proven foreign fee retained")
				}
			}
			if len(foreign) <= e.cfg.orderExecutionCorrelationLimit {
				t.Fatal("capture lacks enough distinct foreign executions")
			}
			h = installCapturedOrder(t, e, owned)
			e.dispatchExecutionToOrder(owned)
			e.routeCommissionReport(reports[0])
			first, second := <-h.Events(), <-h.Events()
			if first.Execution == nil || second.CommissionAndFees == nil {
				t.Fatalf("owned evidence = %+v, %+v", first, second)
			}
		})
	}
}

func TestCapturedActiveStartTimeRejectionTerminatesOrder(t *testing.T) {
	// sv225 api_tif_attribute_matrix_aapl.txt records outright refusal 10315,
	// followed by 10147 on cancellation of the discarded order 591.
	for _, message := range capturedServerMessages(t, "testdata/transcripts/api_tif_attribute_matrix_aapl.txt") {
		m, ok := message.(codec.APIError)
		if !ok || m.Code != 10315 {
			continue
		}
		e, _ := newEngineForDispatchTest()
		h := newOrderHandle(int64(m.ReqID), 4)
		e.orders[int64(m.ReqID)] = &orderRoute{orderID: int64(m.ReqID), handle: h}
		e.handleIncoming(m)
		select {
		case <-h.Done():
			apiErr, ok := errors.AsType[*APIError](h.Wait())
			if !ok || apiErr.Code != m.Code || !apiErr.IsOrderRejection() {
				t.Fatalf("Wait = %v", h.Wait())
			}
		default:
			t.Fatal("captured outright rejection left observation open")
		}
		return
	}
	t.Fatal("capture lacks 10315")
}

func TestExcludedExecutionCacheOwnershipAndEviction(t *testing.T) {
	messages := capturedServerMessages(t, executionsCapturePath)
	reports := capturedCommissionsFrom(messages)
	executions := executionByID(capturedExecutionsFrom(messages))
	e, _ := newEngineForDispatchTest()
	e.cfg.orderExecutionCorrelationLimit = 1
	survivor := newOrderHandle(maxWireOrderID, 4)
	e.orders[maxWireOrderID] = &orderRoute{orderID: maxWireOrderID, handle: survivor}
	first, second := reports[0], reports[1]
	if first.ExecID == second.ExecID {
		t.Fatal("capture lacks distinct execution IDs")
	}
	e.dispatchExecutionToOrder(executions[first.ExecID])
	e.dispatchExecutionToOrder(executions[second.ExecID])
	if len(e.excludedOrderExecutions) != 1 || len(e.excludedOrderExecFIFO) != 1 {
		t.Fatal("exclusion cache exceeded limit")
	}
	if _, stale := e.excludedOrderExecutions[first.ExecID]; stale {
		t.Fatal("oldest exclusion not evicted")
	}
	e.routeCommissionReport(first) // Evicted evidence is unknown again; retain conservatively.
	if e.pendingOrderFees != 1 {
		t.Fatal("evicted fee was silently lost")
	}
	owned := executions[first.ExecID]
	h := installCapturedOrder(t, e, owned)
	e.dispatchExecutionToOrder(owned)
	e.excludeOrderExecution(first.ExecID) // Contradictory negative evidence cannot erase a claim.
	if e.execDeliveries[first.ExecID] == nil || e.pendingOrderFees != 0 {
		t.Fatal("positive claim was discarded")
	}
	e.closeOrderRoute(owned.OrderID, e.orders[owned.OrderID], nil)
	if h.Wait() != nil {
		t.Fatal(h.Wait())
	}
	e.routeCommissionReport(first)
	if len(e.execDeliveries) != 0 || e.pendingOrderFees != 0 {
		t.Fatal("closed-handle late fee retained")
	}
	e.closeOrderRoute(maxWireOrderID, e.orders[maxWireOrderID], nil)
	if len(e.excludedOrderExecutions) != 0 || len(e.excludedOrderExecFIFO) != 0 {
		t.Fatal("last handle leaked exclusion cache")
	}
}

func TestMissingExecutionIdentityDoesNotExcludePendingFee(t *testing.T) {
	sequence := capturedQueryReplaySequence(t)
	execution, fee := sequence[1].(codec.ExecutionDetail), sequence[0].(codec.CommissionReport)
	for _, identity := range []string{"", "0"} {
		t.Run(identity, func(t *testing.T) {
			e, _ := newEngineForDispatchTest()
			h := installCapturedOrder(t, e, execution)
			e.orders[execution.OrderID].permID, _ = strconv.ParseInt(execution.PermID, 10, 64)
			e.routeCommissionReport(fee)
			// Explicit omission fault injection on a captured execution. Client zero
			// is a real identity, so use the empty/sentinel client values here.
			incomplete := execution
			incomplete.ClientID = "2147483647"
			incomplete.PermID = identity
			e.dispatchExecutionToOrder(incomplete)
			if len(e.excludedOrderExecutions) != 0 || e.pendingOrderFees != 1 {
				t.Fatal("missing identity discarded possible owned fee")
			}
			e.dispatchExecutionToOrder(execution)
			if (<-h.Events()).Execution == nil || (<-h.Events()).CommissionAndFees == nil {
				t.Fatal("resolved ownership lost evidence")
			}
		})
	}
}

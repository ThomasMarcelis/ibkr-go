package ibkr

import (
	"errors"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
)

// The callback vectors come from executions.txt (paper Gateway,
// server_version 200, SHA-256
// 91b7b157002bef352b1bae8eb79d3aca1057a43cf3ffcf9497c8f1369b816ddc).
func TestExecutionEventObserverSeesEveryCapturedCallback(t *testing.T) {
	t.Parallel()

	messages := capturedServerMessages(t, executionsCapturePath)
	wantExecutions, wantFees := 0, 0
	for _, message := range messages {
		switch message.(type) {
		case codec.ExecutionDetail:
			wantExecutions++
		case codec.CommissionReport:
			wantFees++
		}
	}
	if wantExecutions == 0 || wantFees == 0 {
		t.Fatalf("capture callbacks = %d executions, %d fees; want both families", wantExecutions, wantFees)
	}

	e, _ := newEngineForDispatchTest()
	sub := newSubscription[ExecutionEvent](subscriptionConfig{buffer: wantExecutions + wantFees}, nil)
	e.executionEvents = &executionEventRoute{sub: sub}
	// Prevent the order-handle correlator's unrelated unmatched-report timer
	// from entering this observer-only test.
	for _, report := range capturedCommissionsFrom(messages) {
		e.execDeliveries[report.ExecID] = &execDelivery{orderID: -1}
	}
	for _, message := range messages {
		switch message.(type) {
		case codec.ExecutionDetail, codec.CommissionReport:
			e.handleIncoming(message)
		}
	}

	gotExecutions, gotFees := 0, 0
	for i := 0; i < wantExecutions+wantFees; i++ {
		event := <-sub.Events()
		if event.Kind != StreamData {
			t.Fatalf("event %d kind = %s, want Data", i, event.Kind)
		}
		switch {
		case event.Value.Execution != nil && event.Value.CommissionAndFees == nil:
			gotExecutions++
			if event.Value.RequestID == nil {
				t.Fatalf("execution event %d omitted its wire request ID", i)
			}
		case event.Value.Execution == nil && event.Value.CommissionAndFees != nil:
			gotFees++
			if event.Value.RequestID != nil {
				t.Fatalf("fee event %d invented request ID %d", i, *event.Value.RequestID)
			}
		default:
			t.Fatalf("event %d has impossible payload union: %#v", i, event.Value)
		}
	}
	if gotExecutions != wantExecutions || gotFees != wantFees {
		t.Fatalf("observer callbacks = %d executions, %d fees; want %d and %d", gotExecutions, gotFees, wantExecutions, wantFees)
	}
}

func TestExecutionEventObserverRunsBeforeQueryCorrelation(t *testing.T) {
	t.Parallel()

	messages := capturedServerMessages(t, executionsCapturePath)
	executions := capturedExecutionsFrom(messages)
	reports := capturedCommissionsFrom(messages)
	if len(executions) == 0 || len(reports) == 0 {
		t.Fatal("live capture omitted execution callback families")
	}

	e, _ := newEngineForDispatchTest()
	sub := newSubscription[ExecutionEvent](subscriptionConfig{buffer: 2}, nil)
	e.executionEvents = &executionEventRoute{sub: sub}
	e.execDeliveries[reports[0].ExecID] = &execDelivery{orderID: -1}
	e.keyed[executions[0].ReqID] = &route{
		opKind: OpExecutions,
		handle: func(any, *engine) {
			event := <-sub.Events()
			if event.Value.Execution == nil {
				t.Fatal("query route ran before passive execution observer")
			}
		},
		handleCommission: func(codec.CommissionReport, *engine) {
			event := <-sub.Events()
			if event.Value.CommissionAndFees == nil {
				t.Fatal("query correlation ran before passive fee observer")
			}
		},
	}

	e.handleIncoming(executions[0])
	e.handleIncoming(reports[0])
}

func TestExecutionEventObserverMarksDataLostGap(t *testing.T) {
	t.Parallel()

	e, _ := newEngineForDispatchTest()
	sub := newSubscription[ExecutionEvent](subscriptionConfig{buffer: 2}, nil)
	e.executionEvents = &executionEventRoute{sub: sub}

	e.gapExecutionEvents(ErrInterrupted)
	e.gapExecutionEvents(ErrInterrupted)
	e.restoreExecutionEvents()

	gap := <-sub.Events()
	if gap.Kind != StreamGap || !errors.Is(gap.Err, ErrInterrupted) {
		t.Fatalf("gap = %#v, want one interrupted Gap", gap)
	}
	restored := <-sub.Events()
	if restored.Kind != StreamRestored {
		t.Fatalf("restored = %#v, want Restored", restored)
	}
	select {
	case event := <-sub.Events():
		t.Fatalf("duplicate lifecycle event = %#v", event)
	default:
	}
}

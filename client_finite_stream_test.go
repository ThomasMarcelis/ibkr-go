package ibkr_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
)

func TestContractDetailsFiniteStreamReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "grounded_contract_details_aapl.txt")
	defer cleanupClientHost(t, client, host)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.Contracts().StreamDetails(ctx, ibkr.Contract{
		Symbol: "AAPL", SecType: ibkr.SecTypeStock, Exchange: "SMART", Currency: "USD",
	})
	if err != nil {
		t.Fatalf("StreamDetails() error = %v", err)
	}
	data, complete := 0, 0
	for event := range sub.Events() {
		switch event.Kind {
		case ibkr.StreamData:
			data++
			if event.Value.Symbol != "AAPL" {
				t.Fatalf("detail symbol = %q, want AAPL", event.Value.Symbol)
			}
		case ibkr.StreamSnapshotComplete:
			complete++
		}
	}
	if err := sub.Wait(); err != nil {
		t.Fatalf("StreamDetails Wait() = %v", err)
	}
	if data != 1 || complete != 1 {
		t.Fatalf("StreamDetails events = %d data, %d complete; want 1, 1", data, complete)
	}
}

func TestSecDefOptParamsFiniteStreamReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "sec_def_opt_params.txt")
	defer cleanupClientHost(t, client, host)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.Contracts().StreamSecDefOptParams(ctx, ibkr.SecDefOptParamsRequest{
		UnderlyingSymbol: "AAPL", UnderlyingSecType: ibkr.SecTypeStock, UnderlyingConID: 265598,
	})
	if err != nil {
		t.Fatalf("StreamSecDefOptParams() error = %v", err)
	}
	data, complete := 0, 0
	for event := range sub.Events() {
		switch event.Kind {
		case ibkr.StreamData:
			data++
		case ibkr.StreamSnapshotComplete:
			complete++
		}
	}
	if err := sub.Wait(); err != nil {
		t.Fatalf("StreamSecDefOptParams Wait() = %v", err)
	}
	if data != 39 || complete != 1 {
		t.Fatalf("StreamSecDefOptParams events = %d data, %d complete; want 39, 1", data, complete)
	}
}

func TestPassiveExecutionObserverReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "executions.txt")
	defer cleanupClientHost(t, client, host)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := client.Orders().SubscribeExecutionEvents(ctx, ibkr.WithResumePolicy(ibkr.ResumeAuto)); err == nil {
		t.Fatal("SubscribeExecutionEvents(ResumeAuto) error = nil")
	} else if validation, ok := errors.AsType[*ibkr.ValidationError](err); !ok || validation.Field != "ResumePolicy" {
		t.Fatalf("SubscribeExecutionEvents(ResumeAuto) error = %#v, want ResumePolicy ValidationError", err)
	}
	if _, err := client.Orders().SubscribeExecutionEvents(ctx, ibkr.WithExecutionCorrelationLimit(8)); err == nil {
		t.Fatal("SubscribeExecutionEvents(correlation limit) error = nil")
	} else if validation, ok := errors.AsType[*ibkr.ValidationError](err); !ok || validation.Field != "ExecutionCorrelationLimit" {
		t.Fatalf("SubscribeExecutionEvents(correlation limit) error = %#v, want ExecutionCorrelationLimit ValidationError", err)
	}

	observer, err := client.Orders().SubscribeExecutionEvents(ctx, ibkr.WithQueueSize(64))
	if err != nil {
		t.Fatalf("SubscribeExecutionEvents() error = %v", err)
	}
	if _, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{}); err != nil {
		t.Fatalf("Executions() error = %v", err)
	}
	observer.Close()

	executions, fees := 0, 0
	for event := range observer.Events() {
		if event.Kind != ibkr.StreamData {
			continue
		}
		if event.Value.Execution != nil {
			executions++
			if event.Value.RequestID == nil || *event.Value.RequestID != 1 {
				t.Fatalf("execution request ID = %v, want 1", event.Value.RequestID)
			}
		}
		if event.Value.CommissionAndFees != nil {
			fees++
			if event.Value.RequestID != nil {
				t.Fatalf("fee request ID = %d, want absent", *event.Value.RequestID)
			}
		}
	}
	if err := observer.Wait(); err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("execution observer Wait() = %v, want clean close or captured disconnect EOF", err)
	}
	if executions != 2 || fees != 2 {
		t.Fatalf("execution observer = %d executions, %d fees; want 2, 2", executions, fees)
	}
}

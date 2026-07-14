package ibkr

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCancelOrderRequestAppliesComplianceOptions(t *testing.T) {
	t.Parallel()

	manualTime := time.Date(2022, 3, 14, 19, 0, 0, 0, time.UTC)
	cfg, err := applyCancelOptions([]CancelOption{
		WithManualCancelTime(manualTime),
		WithCancelExternalOperator("IB"),
		WithCancelManualOrderIndicator(1),
	})
	if err != nil {
		t.Fatalf("applyCancelOptions() error = %v", err)
	}
	req := cancelOrderRequest(295, cfg)
	if req.OrderID != 295 || req.ManualOrderCancelTime != "20220314-19:00:00" ||
		req.ExtOperator != "IB" || req.ManualOrderIndicator != "1" {
		t.Fatalf("cancelOrderRequest() = %+v", req)
	}
}

func TestCancelOrderRejectsIDOutsideWireRangeBeforeEnqueue(t *testing.T) {
	t.Parallel()

	err := new(engine).CancelOrder(context.Background(), maxWireOrderID+1, cancelConfig{})
	validation, ok := errors.AsType[*ValidationError](err)
	if !ok || validation.Field != "OrderID" {
		t.Fatalf("CancelOrder() error = %#v, want OrderID ValidationError", err)
	}
}

func TestCancelOptionsRejectInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts []CancelOption
	}{
		{name: "nil option", opts: []CancelOption{nil}},
		{name: "zero manual time", opts: []CancelOption{WithManualCancelTime(time.Time{})}},
		{name: "empty external operator", opts: []CancelOption{WithCancelExternalOperator(" ")}},
		{name: "NUL external operator", opts: []CancelOption{WithCancelExternalOperator("IB\x00X")}},
		{name: "negative manual indicator", opts: []CancelOption{WithCancelManualOrderIndicator(-1)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := applyCancelOptions(test.opts)
			if _, ok := errors.AsType[*ValidationError](err); !ok {
				t.Fatalf("applyCancelOptions() error = %v, want *ValidationError", err)
			}
		})
	}
}

func TestGlobalCancelRejectsManualCancelTime(t *testing.T) {
	t.Parallel()

	cfg, err := applyCancelOptions([]CancelOption{
		WithManualCancelTime(time.Date(2022, 3, 14, 19, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("applyCancelOptions() error = %v", err)
	}
	_, err = globalCancelRequest(cfg)
	if _, ok := errors.AsType[*ValidationError](err); !ok {
		t.Fatalf("globalCancelRequest() error = %v, want *ValidationError", err)
	}
}

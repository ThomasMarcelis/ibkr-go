package ibkr

import (
	"errors"
	"slices"
	"testing"
	"time"
)

func TestExecutionsRequestProjectsCompleteFilter(t *testing.T) {
	t.Parallel()

	location := time.FixedZone("caller-zone", 2*60*60)
	req := ExecutionsRequest{
		ClientID: 42,
		Account:  "DU9000001",
		Since:    time.Date(2026, 7, 9, 14, 30, 0, 0, location),
		Symbol:   "AAPL",
		SecType:  SecTypeStock,
		Exchange: "IEX",
		Side:     ExecutionFilterBuy,
		LastDays: 5,
		SpecificDates: []time.Time{
			time.Date(2026, 7, 8, 23, 30, 0, 0, location),
			time.Date(2026, 7, 9, 0, 30, 0, 0, location),
		},
	}

	got, err := executionsRequest(req, 200)
	if err != nil {
		t.Fatalf("executionsRequest() error = %v", err)
	}
	if got.ClientID != 42 || got.Account != "DU9000001" ||
		got.Time != "20260709-12:30:00" || got.Symbol != "AAPL" ||
		got.SecType != "STK" || got.Exchange != "IEX" || got.Side != "BUY" {
		t.Fatalf("wire request = %+v", got)
	}
	if got.LastNDays == nil || *got.LastNDays != 5 {
		t.Fatalf("LastNDays = %v, want 5", got.LastNDays)
	}
	if !slices.Equal(got.SpecificDates, []int{20260708, 20260709}) {
		t.Fatalf("SpecificDates = %v", got.SpecificDates)
	}
}

func TestExecutionsRequestValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		req   ExecutionsRequest
		field string
	}{
		{req: ExecutionsRequest{Side: "BOT"}, field: "ExecutionsRequest.Side"},
		{req: ExecutionsRequest{LastDays: -1}, field: "ExecutionsRequest.LastDays"},
		{req: ExecutionsRequest{LastDays: 8}, field: "ExecutionsRequest.LastDays"},
		{req: ExecutionsRequest{SpecificDates: []time.Time{{}}}, field: "ExecutionsRequest.SpecificDates[0]"},
	}
	for _, tt := range tests {
		_, err := executionsRequest(tt.req, 200)
		validationErr, ok := errors.AsType[*ValidationError](err)
		if !ok || validationErr.Field != tt.field {
			t.Errorf("executionsRequest(%+v) error = %v, want %s validation", tt.req, err, tt.field)
		}
	}
}

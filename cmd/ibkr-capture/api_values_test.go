package main

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestOptionalDecimalString(t *testing.T) {
	t.Parallel()

	if got := optionalDecimalString(nil); got != "" {
		t.Fatalf("optionalDecimalString(nil) = %q, want empty", got)
	}
	if got := optionalDecimalString(new(decimal.RequireFromString("1.25"))); got != "1.25" {
		t.Fatalf("optionalDecimalString(1.25) = %q, want 1.25", got)
	}
}

func TestSetRecordedOrderPricesPreservesOmissions(t *testing.T) {
	t.Parallel()

	event := &apiDriverEvent{}
	setRecordedOrderPrices(event, nil, nil)
	if event.LmtPrice != "" || event.AuxPrice != "" {
		t.Fatalf("recorded omitted prices = %q/%q, want empty", event.LmtPrice, event.AuxPrice)
	}

	setRecordedOrderPrices(event,
		new(decimal.RequireFromString("1.25")),
		new(decimal.RequireFromString("0.50")),
	)
	if event.LmtPrice != "1.25" || event.AuxPrice != "0.5" {
		t.Fatalf("recorded prices = %q/%q, want 1.25/0.5", event.LmtPrice, event.AuxPrice)
	}
}

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

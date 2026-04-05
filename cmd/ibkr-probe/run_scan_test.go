package main

import (
	"errors"
	"testing"
)

func TestScanVariantsContinuesAfterFailure(t *testing.T) {
	t.Parallel()

	var calls []string
	err := scanVariants([]probeVariant{
		{Name: "first"},
		{Name: "second"},
	}, func(variant probeVariant) error {
		calls = append(calls, variant.Name)
		if variant.Name == "first" {
			return errors.New("boom")
		}
		return nil
	})

	if err == nil {
		t.Fatal("scanVariants() error = nil, want aggregated failure")
	}
	if len(calls) != 2 {
		t.Fatalf("scanVariants() calls len = %d, want 2", len(calls))
	}
	if calls[0] != "first" || calls[1] != "second" {
		t.Fatalf("scanVariants() calls = %#v, want both variants in order", calls)
	}
}

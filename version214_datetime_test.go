package ibkr

import (
	"testing"
	"time"
)

// The official 10.48 protocol names server_version 214 as a UTC date-time
// boundary, but the outbound suffix behavior remains unresolved. Exact SDK
// differential
// captures 6ee54aa08c7dadc0168ae97c67a7e01ef418e8b434f916489f39edab9fef1cee
// (213) and 7c3dcccd016e3ebe2a74b7318f8ae49a56808158f10e574925fb3b7bf92b2245
// (214) prove only that non-UTC US/Eastern execution values remain unchanged.
// This test freezes inbound parsing tolerance; it does not attest outbound Z.
func TestInboundUTCDateTimeSuffixForms(t *testing.T) {
	t.Parallel()

	want := time.Date(2026, 7, 9, 22, 55, 5, 0, time.UTC)
	tests := []struct {
		name  string
		parse func(string) (time.Time, error)
		value string
	}{
		{"execution", parseExecutionTime, "20260709-22:55:05Z"},
		{"historical bar", parseBarTime, "20260709 22:55:05Z"},
		{"head timestamp", parseHeadTimestamp, "20260709-22:55:05Z"},
		{"historical news", parseHistoricalNewsTime, "2026-07-09 22:55:05.0Z"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.parse(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Equal(want) {
				t.Fatalf("parse(%q) = %s, want %s", tc.value, got, want)
			}
		})
	}
}

package codec

import (
	"math"
	"testing"
)

func TestFieldWriterMaxFloatSentinel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input float64
		want  string
	}{
		{"sentinel", math.MaxFloat64, ""},
		{"zero", 0, "0"},
		{"fractional", 1.5, "1.5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w fieldWriter
			w.WriteMaxFloat(tt.input)
			if got := w.fields[0]; got != tt.want {
				t.Errorf("WriteMaxFloat(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFieldWriterMaxIntSentinel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input int
		want  string
	}{
		{"sentinel", math.MaxInt32, ""},
		{"zero", 0, "0"},
		{"positive", 42, "42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w fieldWriter
			w.WriteMaxInt(tt.input)
			if got := w.fields[0]; got != tt.want {
				t.Errorf("WriteMaxInt(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFieldWriterBoolFormat(t *testing.T) {
	t.Parallel()

	var w fieldWriter
	w.WriteBool(true)
	w.WriteBool(false)

	fields := w.Fields()
	if len(fields) != 2 {
		t.Fatalf("Fields() len = %d, want 2", len(fields))
	}
	if fields[0] != "1" {
		t.Errorf("WriteBool(true) = %q, want %q", fields[0], "1")
	}
	if fields[1] != "0" {
		t.Errorf("WriteBool(false) = %q, want %q", fields[1], "0")
	}
}

func TestFieldWriterDecimalPassthrough(t *testing.T) {
	t.Parallel()
	cases := []string{"", "1.23", "abc"}
	for _, input := range cases {
		var w fieldWriter
		w.WriteDecimal(input)
		if got := w.fields[0]; got != input {
			t.Errorf("WriteDecimal(%q) = %q, want %q", input, got, input)
		}
	}
}

func TestFieldWriterIntNegative(t *testing.T) {
	t.Parallel()

	var w fieldWriter
	w.WriteInt(-1)
	w.WriteInt(0)

	fields := w.Fields()
	if len(fields) != 2 {
		t.Fatalf("Fields() len = %d, want 2", len(fields))
	}
	if fields[0] != "-1" {
		t.Errorf("WriteInt(-1) = %q, want %q", fields[0], "-1")
	}
	if fields[1] != "0" {
		t.Errorf("WriteInt(0) = %q, want %q", fields[1], "0")
	}
}

func TestFieldWriterEmptyFields(t *testing.T) {
	t.Parallel()
	var w fieldWriter
	got := w.Fields()
	// Zero-value fieldWriter has nil fields slice.
	if got != nil {
		t.Errorf("Fields() on empty writer = %v, want nil", got)
	}
}

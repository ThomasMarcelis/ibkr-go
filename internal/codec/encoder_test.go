package codec

import (
	"math"
	"testing"
)

func TestFieldWriterWriteInt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input int
		want  string
	}{
		{"zero", 0, "0"},
		{"negative", -1, "-1"},
		{"positive", 42, "42"},
		{"max_int32", math.MaxInt32, "2147483647"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w fieldWriter
			w.WriteInt(tt.input)
			if got := w.fields[0]; got != tt.want {
				t.Errorf("WriteInt(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFieldWriterWriteInt64(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input int64
		want  string
	}{
		{"zero", 0, "0"},
		{"negative", -1, "-1"},
		{"large", 123456789012, "123456789012"},
		{"max_int64", math.MaxInt64, "9223372036854775807"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w fieldWriter
			w.WriteInt64(tt.input)
			if got := w.fields[0]; got != tt.want {
				t.Errorf("WriteInt64(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFieldWriterWriteFloat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input float64
		want  string
	}{
		{"zero", 0.0, "0"},
		{"pi", 3.14, "3.14"},
		{"negative", -1.5, "-1.5"},
		{"integer_value", 100.0, "100"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w fieldWriter
			w.WriteFloat(tt.input)
			if got := w.fields[0]; got != tt.want {
				t.Errorf("WriteFloat(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFieldWriterWriteMaxFloat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input float64
		want  string
	}{
		{"sentinel", math.MaxFloat64, ""},
		{"pi", 3.14, "3.14"},
		{"zero", 0.0, "0"},
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

func TestFieldWriterWriteMaxInt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input int
		want  string
	}{
		{"sentinel", math.MaxInt32, ""},
		{"positive", 42, "42"},
		{"zero", 0, "0"},
		{"negative", -1, "-1"},
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

func TestFieldWriterWriteString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"hello", "hello", "hello"},
		{"with_null", "a\x00b", "a\x00b"},
		{"unicode", "AAPL\u00AE", "AAPL\u00AE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w fieldWriter
			w.WriteString(tt.input)
			if got := w.fields[0]; got != tt.want {
				t.Errorf("WriteString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFieldWriterWriteBool(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input bool
		want  string
	}{
		{"true", true, "1"},
		{"false", false, "0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w fieldWriter
			w.WriteBool(tt.input)
			if got := w.fields[0]; got != tt.want {
				t.Errorf("WriteBool(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFieldWriterWriteDecimal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"integer", "42", "42"},
		{"decimal", "3.14159265358979323846", "3.14159265358979323846"},
		{"empty", "", ""},
		{"negative", "-100.50", "-100.50"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w fieldWriter
			w.WriteDecimal(tt.input)
			if got := w.fields[0]; got != tt.want {
				t.Errorf("WriteDecimal(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFieldWriterFieldSequence(t *testing.T) {
	t.Parallel()
	var w fieldWriter
	w.WriteInt(1)
	w.WriteString("AAPL")
	w.WriteBool(true)
	w.WriteFloat(123.45)
	w.WriteMaxInt(math.MaxInt32)

	got := w.Fields()
	want := []string{"1", "AAPL", "1", "123.45", ""}
	if len(got) != len(want) {
		t.Fatalf("Fields() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Fields()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFieldWriterFieldsEmpty(t *testing.T) {
	t.Parallel()
	var w fieldWriter
	got := w.Fields()
	if got != nil {
		t.Errorf("Fields() on empty writer = %v, want nil", got)
	}
}

func TestEncodeExecutionsRequestServer200Layout(t *testing.T) {
	t.Parallel()

	fields, err := encodeFields(ExecutionsRequest{
		ReqID:   3,
		Account: "DUP770846",
		Symbol:  "AAPL",
	})
	if err != nil {
		t.Fatalf("encodeFields(ExecutionsRequest) error = %v", err)
	}

	want := []string{"7", "3", "3", "0", "DUP770846", "", "AAPL", "", "", "", "2147483647", "0"}
	if len(fields) != len(want) {
		t.Fatalf("fields len = %d, want %d: %v", len(fields), len(want), fields)
	}
	for i := range want {
		if fields[i] != want[i] {
			t.Fatalf("fields[%d] = %q, want %q; fields=%v", i, fields[i], want[i], fields)
		}
	}
}

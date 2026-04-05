package codec

import (
	"math"
	"testing"
)

func TestFieldReaderReadInt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{"positive", "42", 42, false},
		{"zero", "0", 0, false},
		{"negative", "-1", -1, false},
		{"empty", "", 0, false},
		{"invalid", "abc", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newFieldReader([]string{tt.input})
			got, err := r.ReadInt()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadInt() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ReadInt() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFieldReaderReadInt64(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{"large", "123456789012", 123456789012, false},
		{"zero", "0", 0, false},
		{"negative", "-1", -1, false},
		{"empty", "", 0, false},
		{"invalid", "xyz", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newFieldReader([]string{tt.input})
			got, err := r.ReadInt64()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadInt64() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ReadInt64() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFieldReaderReadFloat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{"pi", "3.14", 3.14, false},
		{"zero", "0", 0.0, false},
		{"negative", "-1.5", -1.5, false},
		{"empty", "", 0.0, false},
		{"invalid", "not_a_float", 0, true},
		{"max_float_string", "1.7976931348623157E308", math.MaxFloat64, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newFieldReader([]string{tt.input})
			got, err := r.ReadFloat()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadFloat() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ReadFloat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFieldReaderReadMaxFloat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{"empty_sentinel", "", math.MaxFloat64, false},
		{"pi", "3.14", 3.14, false},
		{"zero", "0", 0.0, false},
		{"invalid", "bad", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newFieldReader([]string{tt.input})
			got, err := r.ReadMaxFloat()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadMaxFloat() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ReadMaxFloat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFieldReaderReadMaxInt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{"empty_sentinel", "", math.MaxInt32, false},
		{"positive", "42", 42, false},
		{"zero", "0", 0, false},
		{"invalid", "bad", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newFieldReader([]string{tt.input})
			got, err := r.ReadMaxInt()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadMaxInt() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ReadMaxInt() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFieldReaderReadString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input []string
		want  string
	}{
		{"hello", []string{"hello"}, "hello"},
		{"empty_field", []string{""}, ""},
		{"past_end", []string{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newFieldReader(tt.input)
			if got := r.ReadString(); got != tt.want {
				t.Errorf("ReadString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFieldReaderReadBool(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    bool
		wantErr bool
	}{
		{"one", "1", true, false},
		{"zero", "0", false, false},
		{"true_word", "true", true, false},
		{"false_word", "false", false, false},
		{"empty", "", false, false},
		{"invalid", "yes", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newFieldReader([]string{tt.input})
			got, err := r.ReadBool()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadBool() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ReadBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFieldReaderReadDecimal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"integer", "42", "42"},
		{"high_precision", "3.14159265358979323846", "3.14159265358979323846"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newFieldReader([]string{tt.input})
			if got := r.ReadDecimal(); got != tt.want {
				t.Errorf("ReadDecimal() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFieldReaderSkip(t *testing.T) {
	t.Parallel()
	r := newFieldReader([]string{"a", "b", "c", "d"})
	r.Skip(2)
	if got := r.ReadString(); got != "c" {
		t.Errorf("after Skip(2), ReadString() = %q, want %q", got, "c")
	}
}

func TestFieldReaderRemaining(t *testing.T) {
	t.Parallel()
	r := newFieldReader([]string{"a", "b", "c"})
	if got := r.Remaining(); got != 3 {
		t.Errorf("Remaining() = %d, want 3", got)
	}
	r.ReadString()
	if got := r.Remaining(); got != 2 {
		t.Errorf("after 1 read, Remaining() = %d, want 2", got)
	}
	r.Skip(2)
	if got := r.Remaining(); got != 0 {
		t.Errorf("after skip past end, Remaining() = %d, want 0", got)
	}
}

func TestFieldReaderRemainingPastEnd(t *testing.T) {
	t.Parallel()
	r := newFieldReader([]string{"a"})
	r.Skip(5)
	if got := r.Remaining(); got != 0 {
		t.Errorf("Remaining() past end = %d, want 0", got)
	}
}

func TestFieldReaderPos(t *testing.T) {
	t.Parallel()
	r := newFieldReader([]string{"a", "b", "c"})
	if got := r.Pos(); got != 0 {
		t.Errorf("initial Pos() = %d, want 0", got)
	}
	r.ReadString()
	if got := r.Pos(); got != 1 {
		t.Errorf("after 1 read, Pos() = %d, want 1", got)
	}
	r.Skip(1)
	if got := r.Pos(); got != 2 {
		t.Errorf("after Skip(1), Pos() = %d, want 2", got)
	}
}

func TestFieldReaderReadPastEnd(t *testing.T) {
	t.Parallel()
	r := newFieldReader([]string{})

	if got := r.ReadString(); got != "" {
		t.Errorf("ReadString() past end = %q, want empty", got)
	}
	if got, err := r.ReadInt(); err != nil || got != 0 {
		t.Errorf("ReadInt() past end = (%d, %v), want (0, nil)", got, err)
	}
	if got, err := r.ReadFloat(); err != nil || got != 0 {
		t.Errorf("ReadFloat() past end = (%v, %v), want (0, nil)", got, err)
	}
	if got, err := r.ReadBool(); err != nil || got != false {
		t.Errorf("ReadBool() past end = (%v, %v), want (false, nil)", got, err)
	}
}

func TestFieldReaderMultiFieldSequence(t *testing.T) {
	t.Parallel()
	r := newFieldReader([]string{"1", "AAPL", "1", "123.45", ""})

	v1, err := r.ReadInt()
	if err != nil || v1 != 1 {
		t.Fatalf("ReadInt() = (%d, %v), want (1, nil)", v1, err)
	}
	v2 := r.ReadString()
	if v2 != "AAPL" {
		t.Fatalf("ReadString() = %q, want %q", v2, "AAPL")
	}
	v3, err := r.ReadBool()
	if err != nil || v3 != true {
		t.Fatalf("ReadBool() = (%v, %v), want (true, nil)", v3, err)
	}
	v4, err := r.ReadFloat()
	if err != nil || v4 != 123.45 {
		t.Fatalf("ReadFloat() = (%v, %v), want (123.45, nil)", v4, err)
	}
	v5, err := r.ReadMaxInt()
	if err != nil || v5 != math.MaxInt32 {
		t.Fatalf("ReadMaxInt() = (%d, %v), want (%d, nil)", v5, err, math.MaxInt32)
	}
	if rem := r.Remaining(); rem != 0 {
		t.Errorf("Remaining() = %d, want 0", rem)
	}
}

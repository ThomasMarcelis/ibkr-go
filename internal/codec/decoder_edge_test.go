//go:build legacy_native_socket

package codec

import (
	"math"
	"testing"
)

func TestFieldReaderPastEndRepeated(t *testing.T) {
	t.Parallel()
	r := newFieldReader([]string{"alpha", "beta"})

	// First two reads return actual values.
	if got := r.ReadString(); got != "alpha" {
		t.Fatalf("read 1: got %q, want %q", got, "alpha")
	}
	if got := r.ReadString(); got != "beta" {
		t.Fatalf("read 2: got %q, want %q", got, "beta")
	}

	// Reads 3-5 are past end: string returns "", int returns (0, nil), bool returns (false, nil).
	if got := r.ReadString(); got != "" {
		t.Errorf("read 3 (string past end): got %q, want %q", got, "")
	}
	if got, err := r.ReadInt(); err != nil || got != 0 {
		t.Errorf("read 4 (int past end): got (%d, %v), want (0, nil)", got, err)
	}
	if got, err := r.ReadBool(); err != nil || got != false {
		t.Errorf("read 5 (bool past end): got (%v, %v), want (false, nil)", got, err)
	}
}

func TestFieldReaderInvalidInt(t *testing.T) {
	t.Parallel()
	cases := []string{"abc", "1.5", "9999999999999999999", "-", "1 2"}
	for _, input := range cases {
		r := newFieldReader([]string{input})
		got, err := r.ReadInt()
		if err == nil {
			t.Errorf("ReadInt(%q) = (%d, nil), want error", input, got)
		}
		if got != 0 {
			t.Errorf("ReadInt(%q) value = %d, want 0", input, got)
		}
	}
}

func TestFieldReaderInvalidInt64(t *testing.T) {
	t.Parallel()
	cases := []string{"abc", "99999999999999999999"}
	for _, input := range cases {
		r := newFieldReader([]string{input})
		got, err := r.ReadInt64()
		if err == nil {
			t.Errorf("ReadInt64(%q) = (%d, nil), want error", input, got)
		}
		if got != 0 {
			t.Errorf("ReadInt64(%q) value = %d, want 0", input, got)
		}
	}
}

func TestFieldReaderInvalidFloat(t *testing.T) {
	t.Parallel()
	cases := []string{"abc", "not-a-float"}
	for _, input := range cases {
		r := newFieldReader([]string{input})
		got, err := r.ReadFloat()
		if err == nil {
			t.Errorf("ReadFloat(%q) = (%v, nil), want error", input, got)
		}
		if got != 0 {
			t.Errorf("ReadFloat(%q) value = %v, want 0", input, got)
		}
	}
}

func TestFieldReaderInvalidBool(t *testing.T) {
	t.Parallel()
	cases := []string{"2", "yes", "TRUE", "nope", "-1"}
	for _, input := range cases {
		r := newFieldReader([]string{input})
		got, err := r.ReadBool()
		if err == nil {
			t.Errorf("ReadBool(%q) = (%v, nil), want error", input, got)
		}
		if got != false {
			t.Errorf("ReadBool(%q) value = %v, want false", input, got)
		}
	}
}

func TestFieldReaderEmptyStringHandling(t *testing.T) {
	t.Parallel()

	t.Run("ReadInt", func(t *testing.T) {
		r := newFieldReader([]string{""})
		got, err := r.ReadInt()
		if err != nil || got != 0 {
			t.Errorf("ReadInt(\"\") = (%d, %v), want (0, nil)", got, err)
		}
	})

	t.Run("ReadFloat", func(t *testing.T) {
		r := newFieldReader([]string{""})
		got, err := r.ReadFloat()
		if err != nil || got != 0 {
			t.Errorf("ReadFloat(\"\") = (%v, %v), want (0, nil)", got, err)
		}
	})

	t.Run("ReadBool", func(t *testing.T) {
		r := newFieldReader([]string{""})
		got, err := r.ReadBool()
		if err != nil || got != false {
			t.Errorf("ReadBool(\"\") = (%v, %v), want (false, nil)", got, err)
		}
	})

	t.Run("ReadMaxFloat", func(t *testing.T) {
		r := newFieldReader([]string{""})
		got, err := r.ReadMaxFloat()
		if err != nil || got != math.MaxFloat64 {
			t.Errorf("ReadMaxFloat(\"\") = (%v, %v), want (%v, nil)", got, err, math.MaxFloat64)
		}
	})

	t.Run("ReadMaxInt", func(t *testing.T) {
		r := newFieldReader([]string{""})
		got, err := r.ReadMaxInt()
		if err != nil || got != math.MaxInt32 {
			t.Errorf("ReadMaxInt(\"\") = (%d, %v), want (%d, nil)", got, err, math.MaxInt32)
		}
	})
}

func TestFieldReaderSkipBeyondEnd(t *testing.T) {
	t.Parallel()
	r := newFieldReader([]string{"a", "b", "c"})
	r.Skip(100)

	if got := r.Remaining(); got != 0 {
		t.Errorf("Remaining() after Skip(100) = %d, want 0", got)
	}
	if got := r.Pos(); got != 100 {
		t.Errorf("Pos() after Skip(100) on 3 fields = %d, want 100", got)
	}
	if got := r.ReadString(); got != "" {
		t.Errorf("ReadString() after skip past end = %q, want %q", got, "")
	}
}

func TestFieldReaderDecimalPassthrough(t *testing.T) {
	t.Parallel()
	cases := []string{"abc", "", "3.14", "9999999999999999999"}
	for _, input := range cases {
		r := newFieldReader([]string{input})
		got := r.ReadDecimal()
		if got != input {
			t.Errorf("ReadDecimal(%q) = %q, want %q", input, got, input)
		}
	}
}

package ibkr

import (
	"encoding/json"
	"testing"
)

func TestParseDecimal_ValidInputs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input    string
		expected string
	}{
		{"0", "0"},
		{"1", "1"},
		{"-1", "-1"},
		{"0.00", "0"},
		{"-0", "0"},
		{"-0.00", "0"},
		{"+1.5", "1.5"},
		{"001.100", "1.1"},
		{"99999999999.99999999", "99999999999.99999999"},
		{".5", "0.5"},
		{"-.5", "-0.5"},
		{" 1.23 ", "1.23"},
		{"100", "100"},
		{"0.0001", "0.0001"},
		{"+0", "0"},
		{"0000", "0"},
		{"10.0", "10"},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			d, err := ParseDecimal(tc.input)
			if err != nil {
				t.Fatalf("ParseDecimal(%q) unexpected error: %v", tc.input, err)
			}
			if got := d.String(); got != tc.expected {
				t.Errorf("ParseDecimal(%q).String() = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestParseDecimal_InvalidInputs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		desc  string
	}{
		{"", "empty"},
		{" ", "whitespace only"},
		{"abc", "non-numeric"},
		{"1.2.3", "multiple dots"},
		{"1e10", "scientific notation"},
		{"1E10", "scientific notation uppercase"},
		{"1,000", "comma separator"},
		{"NaN", "not a number"},
		{"Infinity", "infinity"},
		{"inf", "inf"},
		{"+", "sign only"},
		{"-", "sign only"},
		{"--1", "double sign"},
		{"1..2", "consecutive dots"},
		{"12abc", "trailing garbage"},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			_, err := ParseDecimal(tc.input)
			if err == nil {
				t.Errorf("ParseDecimal(%q) expected error for %s, got nil", tc.input, tc.desc)
			}
		})
	}
}

func TestDecimalZeroValue(t *testing.T) {
	t.Parallel()

	var d Decimal
	if got := d.String(); got != "0" {
		t.Errorf("Decimal{}.String() = %q, want %q", got, "0")
	}
	if !d.IsZero() {
		t.Error("Decimal{}.IsZero() = false, want true")
	}
}

func TestDecimalIsZero(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  bool
	}{
		{"0", true},
		{"0.00", true},
		{"1", false},
		{"-1", false},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			d, err := ParseDecimal(tc.input)
			if err != nil {
				t.Fatalf("ParseDecimal(%q) error: %v", tc.input, err)
			}
			if got := d.IsZero(); got != tc.want {
				t.Errorf("ParseDecimal(%q).IsZero() = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestDecimalEqual(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		a, b string
		want bool
	}{
		{"normalized trailing zero", "1.0", "1", true},
		{"normalized fractional trailing zero", "0.10", "0.1", true},
		{"different values", "1", "2", false},
		{"negative zero", "-0", "0", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := MustParseDecimal(tc.a)
			b := MustParseDecimal(tc.b)
			if got := a.Equal(b); got != tc.want {
				t.Errorf("ParseDecimal(%q).Equal(ParseDecimal(%q)) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}

	t.Run("zero value vs parsed zero", func(t *testing.T) {
		t.Parallel()
		var zv Decimal
		parsed := MustParseDecimal("0")
		if !zv.Equal(parsed) {
			t.Error("Decimal{}.Equal(ParseDecimal(\"0\")) = false, want true")
		}
	})
}

func TestDecimalJSON_RoundTrip(t *testing.T) {
	t.Parallel()

	inputs := []string{"0", "1.5", "-999.001", "0.0001"}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			d := MustParseDecimal(input)
			data, err := json.Marshal(d)
			if err != nil {
				t.Fatalf("MarshalJSON error: %v", err)
			}

			var d2 Decimal
			if err := json.Unmarshal(data, &d2); err != nil {
				t.Fatalf("UnmarshalJSON error: %v", err)
			}
			if !d.Equal(d2) {
				t.Errorf("JSON round-trip: %q -> %s -> %q, not equal", d.String(), string(data), d2.String())
			}
		})
	}

	t.Run("json format is quoted string", func(t *testing.T) {
		t.Parallel()
		d := MustParseDecimal("1.5")
		data, err := json.Marshal(d)
		if err != nil {
			t.Fatalf("MarshalJSON error: %v", err)
		}
		if got := string(data); got != `"1.5"` {
			t.Errorf("MarshalJSON(1.5) = %s, want %q", got, `"1.5"`)
		}
	})
}

func TestDecimalText_RoundTrip(t *testing.T) {
	t.Parallel()

	inputs := []string{"0", "1.5", "-999.001", "0.0001"}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			d := MustParseDecimal(input)
			data, err := d.MarshalText()
			if err != nil {
				t.Fatalf("MarshalText error: %v", err)
			}

			var d2 Decimal
			if err := d2.UnmarshalText(data); err != nil {
				t.Fatalf("UnmarshalText error: %v", err)
			}
			if !d.Equal(d2) {
				t.Errorf("Text round-trip: %q -> %s -> %q, not equal", d.String(), string(data), d2.String())
			}
		})
	}
}

func TestMustParseDecimal_Panics(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Error("MustParseDecimal(\"abc\") did not panic")
		}
	}()
	MustParseDecimal("abc")
}

func TestMustParseDecimal_Valid(t *testing.T) {
	t.Parallel()

	d := MustParseDecimal("1.5")
	if got := d.String(); got != "1.5" {
		t.Errorf("MustParseDecimal(\"1.5\").String() = %q, want %q", got, "1.5")
	}
}

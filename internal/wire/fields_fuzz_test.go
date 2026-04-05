package wire

import (
	"strings"
	"testing"
)

func FuzzParseFields(f *testing.F) {
	f.Add([]byte("hello\x00"))
	f.Add([]byte("a\x00b\x00"))
	f.Add([]byte{0x00})
	f.Add([]byte("no terminator"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		ParseFields(data) // must not panic
	})
}

func FuzzEncodeParseFieldsRoundTrip(f *testing.F) {
	f.Add("hello", "world")
	f.Add("", "test")
	f.Add("one", "")
	f.Add("single", "")
	f.Add("123", "456")

	f.Fuzz(func(t *testing.T, a, b string) {
		// Null bytes within fields would split incorrectly on parse.
		if strings.ContainsRune(a, 0) || strings.ContainsRune(b, 0) {
			return
		}
		// ParseFields rejects payloads where the first field is empty.
		if a == "" {
			return
		}

		fields := []string{a, b}
		encoded := EncodeFields(fields)
		if encoded == nil {
			return
		}

		parsed, err := ParseFields(encoded)
		if err != nil {
			t.Fatalf("ParseFields failed: %v", err)
		}
		if len(parsed) != len(fields) {
			t.Fatalf("field count: got %d, want %d", len(parsed), len(fields))
		}
		for i := range fields {
			if parsed[i] != fields[i] {
				t.Errorf("field[%d]: got %q, want %q", i, parsed[i], fields[i])
			}
		}
	})
}

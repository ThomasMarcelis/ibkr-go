package wire

import (
	"bytes"
	"testing"
)

func FuzzReadFrame(f *testing.F) {
	// Valid frame: 4-byte big-endian length header + payload.
	var valid bytes.Buffer
	WriteFrame(&valid, []byte("hello\x00"))
	f.Add(valid.Bytes())

	// Zero-length header (triggers ErrEmptyMessage).
	f.Add([]byte{0, 0, 0, 0})

	// Truncated: header claims 100 bytes, only 5 available.
	f.Add([]byte{0, 0, 0, 100, 'h', 'e', 'l', 'l', 'o'})

	// Empty input.
	f.Add([]byte{})

	// Header only, no payload bytes.
	f.Add([]byte{0, 0, 0, 5})

	// Very large size in header with truncated payload.
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		ReadFrame(bytes.NewReader(data)) // must not panic
	})
}

func FuzzWriteFrameRoundTrip(f *testing.F) {
	f.Add([]byte("test\x00"))
	f.Add([]byte{0x00})
	f.Add([]byte("hello world with spaces\x00"))

	f.Fuzz(func(t *testing.T, payload []byte) {
		if len(payload) == 0 {
			return // WriteFrame rejects empty payloads
		}
		var buf bytes.Buffer
		if err := WriteFrame(&buf, payload); err != nil {
			return // payload rejected (too large or empty)
		}
		got, err := ReadFrame(bytes.NewReader(buf.Bytes()))
		if err != nil {
			t.Fatalf("ReadFrame failed on round-trip: %v", err)
		}
		if !bytes.Equal(got, payload) {
			t.Fatalf("round-trip mismatch: got %q, want %q", got, payload)
		}
	})
}

package wire

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	t.Parallel()

	fields := []string{"hello", "1", "7"}
	payload := EncodeFields(fields)

	var buf bytes.Buffer
	if err := WriteFrame(&buf, payload); err != nil {
		t.Fatalf("WriteFrame() error = %v", err)
	}

	gotPayload, err := ReadFrame(&buf)
	if err != nil {
		t.Fatalf("ReadFrame() error = %v", err)
	}

	gotFields, err := ParseFields(gotPayload)
	if err != nil {
		t.Fatalf("ParseFields() error = %v", err)
	}

	if len(gotFields) != len(fields) {
		t.Fatalf("field count = %d, want %d", len(gotFields), len(fields))
	}
	for i := range fields {
		if gotFields[i] != fields[i] {
			t.Fatalf("field[%d] = %q, want %q", i, gotFields[i], fields[i])
		}
	}
}

func TestParseFieldsRejectsMissingTerminator(t *testing.T) {
	t.Parallel()

	if _, err := ParseFields([]byte("hello\x001")); !errors.Is(err, ErrMalformedFrame) {
		t.Fatalf("ParseFields() error = %v, want ErrMalformedFrame", err)
	}
}

func TestReadFrameRejectsTruncatedPayload(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	buf.Write([]byte{0, 0, 0, 5})
	buf.Write([]byte("abc"))

	_, err := ReadFrame(&buf)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("ReadFrame() error = %v, want io.ErrUnexpectedEOF", err)
	}
}

func TestFrameMaxSizeBoundary(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte{1}, MaxFrameSize)

	var buf bytes.Buffer
	if err := WriteFrame(&buf, payload); err != nil {
		t.Fatalf("WriteFrame() error = %v", err)
	}

	gotPayload, err := ReadFrame(&buf)
	if err != nil {
		t.Fatalf("ReadFrame() error = %v", err)
	}
	if len(gotPayload) != len(payload) {
		t.Fatalf("payload len = %d, want %d", len(gotPayload), len(payload))
	}
}

func TestWriteFrameRejectsOversizePayload(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte{1}, MaxFrameSize+1)

	var buf bytes.Buffer
	if err := WriteFrame(&buf, payload); !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("WriteFrame() error = %v, want ErrFrameTooLarge", err)
	}
}

func TestReadFrameRejectsOversizeHeader(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	buf.Write([]byte{4, 0, 0, 1})

	_, err := ReadFrame(&buf)
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("ReadFrame() error = %v, want ErrFrameTooLarge", err)
	}
}

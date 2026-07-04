package codec

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"unsafe"
)

// fieldReader reads NUL-delimited fields directly out of the frame byte slice.
// It never copies the payload: numeric and boolean fields are parsed straight
// from the backing bytes, and only fields the decoder retains (via ReadString
// or ReadDecimal) allocate a string. buf holds the reader's field region, each
// field terminated by a NUL; off is the byte offset of the next field, pos its
// field index.
type fieldReader struct {
	buf   []byte
	off   int
	pos   int
	total int // total field count in buf; cached so Remaining/Len stay O(1)
	err   error
}

// newFieldReaderBytes wraps a frame's NUL-terminated field region. total is
// scanned once here so per-field Remaining/Len calls in the decoders — some
// invoked inside per-entry loops — cost O(1) rather than re-scanning the tail.
func newFieldReaderBytes(buf []byte) *fieldReader {
	return &fieldReader{buf: buf, total: bytes.Count(buf, nulSep)}
}

var nulSep = []byte{0}

// newFieldReader builds a reader over the NUL-joined encoding of fields. It is
// the convenience constructor used by unit tests; production decode wraps the
// live frame bytes directly (see DecodeBatch).
func newFieldReader(fields []string) *fieldReader {
	if len(fields) == 0 {
		return &fieldReader{}
	}
	n := 0
	for _, f := range fields {
		n += len(f) + 1
	}
	buf := make([]byte, 0, n)
	for _, f := range fields {
		buf = append(buf, f...)
		buf = append(buf, 0)
	}
	return &fieldReader{buf: buf, total: len(fields)}
}

// asString aliases b as a string without copying. The result is a transient
// argument for strconv over the immutable frame buffer; it must never be
// retained, since it shares b's backing array.
func asString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// field returns the bytes of the current field (excluding its NUL) and
// advances past it. ok is false once the reader is past the end; a genuinely
// empty field returns (empty, true). The frame is NUL-terminated per field, so
// a NUL always exists while off is in range.
func (r *fieldReader) field() ([]byte, bool) {
	if r.off >= len(r.buf) {
		return nil, false
	}
	rel := bytes.IndexByte(r.buf[r.off:], 0)
	f := r.buf[r.off : r.off+rel]
	r.off += rel + 1
	r.pos++
	return f, true
}

// peek returns the current field's bytes without advancing. It returns nil
// once past the end.
func (r *fieldReader) peek() []byte {
	if r.off >= len(r.buf) {
		return nil
	}
	rel := bytes.IndexByte(r.buf[r.off:], 0)
	return r.buf[r.off : r.off+rel]
}

func (r *fieldReader) setErr(err error) {
	if err != nil && r.err == nil {
		r.err = err
	}
}

func (r *fieldReader) Err() error {
	return r.err
}

func (r *fieldReader) ReadInt() (int, error) {
	f, ok := r.field()
	if !ok || len(f) == 0 {
		return 0, nil
	}
	v, err := strconv.Atoi(asString(f))
	if err != nil {
		parseErr := fmt.Errorf("codec: field %d: parse int %q: %w", r.pos-1, f, err)
		r.setErr(parseErr)
		return 0, parseErr
	}
	return v, nil
}

func (r *fieldReader) ReadInt64() (int64, error) {
	f, ok := r.field()
	if !ok || len(f) == 0 {
		return 0, nil
	}
	v, err := strconv.ParseInt(asString(f), 10, 64)
	if err != nil {
		parseErr := fmt.Errorf("codec: field %d: parse int64 %q: %w", r.pos-1, f, err)
		r.setErr(parseErr)
		return 0, parseErr
	}
	return v, nil
}

func (r *fieldReader) ReadFloat() (float64, error) {
	f, ok := r.field()
	if !ok || len(f) == 0 {
		return 0, nil
	}
	v, err := strconv.ParseFloat(asString(f), 64)
	if err != nil {
		parseErr := fmt.Errorf("codec: field %d: parse float %q: %w", r.pos-1, f, err)
		r.setErr(parseErr)
		return 0, parseErr
	}
	return v, nil
}

// ReadMaxFloat reads a float, returning math.MaxFloat64 for empty string (TWS sentinel).
func (r *fieldReader) ReadMaxFloat() (float64, error) {
	f, ok := r.field()
	if !ok || len(f) == 0 {
		return math.MaxFloat64, nil
	}
	v, err := strconv.ParseFloat(asString(f), 64)
	if err != nil {
		parseErr := fmt.Errorf("codec: field %d: parse float %q: %w", r.pos-1, f, err)
		r.setErr(parseErr)
		return 0, parseErr
	}
	return v, nil
}

// ReadMaxInt reads an int, returning math.MaxInt32 for empty string (TWS sentinel).
func (r *fieldReader) ReadMaxInt() (int, error) {
	f, ok := r.field()
	if !ok || len(f) == 0 {
		return math.MaxInt32, nil
	}
	v, err := strconv.Atoi(asString(f))
	if err != nil {
		parseErr := fmt.Errorf("codec: field %d: parse int %q: %w", r.pos-1, f, err)
		r.setErr(parseErr)
		return 0, parseErr
	}
	return v, nil
}

// ReadString returns the next field as a string. Returns "" if past end.
func (r *fieldReader) ReadString() string {
	f, ok := r.field()
	if !ok {
		return ""
	}
	return string(f)
}

func (r *fieldReader) ReadBool() (bool, error) {
	f, _ := r.field()
	// switch string(f) is compiled without allocating; nil past-end bytes
	// compare equal to "" and read as false, matching ReadString semantics.
	switch string(f) {
	case "1", "true":
		return true, nil
	case "0", "false", "":
		return false, nil
	default:
		parseErr := fmt.Errorf("codec: field %d: parse bool %q", r.pos-1, f)
		r.setErr(parseErr)
		return false, parseErr
	}
}

// ReadDecimal reads a raw decimal string without conversion (preserves precision).
func (r *fieldReader) ReadDecimal() string {
	return r.ReadString()
}

// Skip advances past n fields. Advancing past the end still increments the
// field index, so Pos reflects the total number of skipped fields.
func (r *fieldReader) Skip(n int) {
	for range n {
		if r.off < len(r.buf) {
			rel := bytes.IndexByte(r.buf[r.off:], 0)
			r.off += rel + 1
		}
		r.pos++
	}
}

// Len returns the total number of fields.
func (r *fieldReader) Len() int {
	return r.total
}

// Remaining returns how many unread fields remain.
func (r *fieldReader) Remaining() int {
	if r.pos >= r.total {
		return 0
	}
	return r.total - r.pos
}

// Pos returns the current read position.
func (r *fieldReader) Pos() int {
	return r.pos
}

func (r *fieldReader) ReadCount(label string) (int, error) {
	if r.off >= len(r.buf) {
		parseErr := fmt.Errorf("codec: field %d: missing %s", r.pos, label)
		r.setErr(parseErr)
		return 0, parseErr
	}
	f, _ := r.field()
	if len(f) == 0 {
		parseErr := fmt.Errorf("codec: field %d: empty %s", r.pos-1, label)
		r.setErr(parseErr)
		return 0, parseErr
	}
	count, err := strconv.Atoi(asString(f))
	if err != nil {
		parseErr := fmt.Errorf("codec: field %d: parse %s %q: %w", r.pos-1, label, f, err)
		r.setErr(parseErr)
		return 0, parseErr
	}
	if count < 0 {
		parseErr := fmt.Errorf("codec: field %d: negative %s %d", r.pos-1, label, count)
		r.setErr(parseErr)
		return 0, parseErr
	}
	return count, nil
}

func (r *fieldReader) ReadOptionalCount(label string) (int, error) {
	if r.off >= len(r.buf) {
		return 0, nil
	}
	f, _ := r.field()
	if len(f) == 0 {
		return 0, nil
	}
	count, err := strconv.Atoi(asString(f))
	if err != nil {
		parseErr := fmt.Errorf("codec: field %d: parse %s %q: %w", r.pos-1, label, f, err)
		r.setErr(parseErr)
		return 0, parseErr
	}
	if count < 0 {
		parseErr := fmt.Errorf("codec: field %d: negative %s %d", r.pos-1, label, count)
		r.setErr(parseErr)
		return 0, parseErr
	}
	return count, nil
}

func (r *fieldReader) RequireFixedEntryFields(label string, count, fieldsPerEntry, trailerFields int) error {
	if fieldsPerEntry <= 0 {
		return fmt.Errorf("codec: %s: invalid entry width %d", label, fieldsPerEntry)
	}
	remaining := r.Remaining()
	if remaining < trailerFields {
		return fmt.Errorf("codec: %s: want at least %d trailing fields, got %d", label, trailerFields, remaining)
	}
	if count > (remaining-trailerFields)/fieldsPerEntry {
		return fmt.Errorf(
			"codec: %s: count %d exceeds available fields (%d remaining, %d per entry, %d trailer)",
			label,
			count,
			remaining,
			fieldsPerEntry,
			trailerFields,
		)
	}
	return nil
}

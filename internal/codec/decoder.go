package codec

import (
	"fmt"
	"math"
	"strconv"
)

type fieldReader struct {
	fields []string
	pos    int
	err    error
}

func newFieldReader(fields []string) *fieldReader {
	return &fieldReader{fields: fields}
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
	s := r.ReadString()
	if s == "" {
		return 0, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		parseErr := fmt.Errorf("codec: field %d: parse int %q: %w", r.pos-1, s, err)
		r.setErr(parseErr)
		return 0, parseErr
	}
	return v, nil
}

func (r *fieldReader) ReadInt64() (int64, error) {
	s := r.ReadString()
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		parseErr := fmt.Errorf("codec: field %d: parse int64 %q: %w", r.pos-1, s, err)
		r.setErr(parseErr)
		return 0, parseErr
	}
	return v, nil
}

func (r *fieldReader) ReadFloat() (float64, error) {
	s := r.ReadString()
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		parseErr := fmt.Errorf("codec: field %d: parse float %q: %w", r.pos-1, s, err)
		r.setErr(parseErr)
		return 0, parseErr
	}
	return v, nil
}

// ReadMaxFloat reads a float, returning math.MaxFloat64 for empty string (TWS sentinel).
func (r *fieldReader) ReadMaxFloat() (float64, error) {
	s := r.ReadString()
	if s == "" {
		return math.MaxFloat64, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		parseErr := fmt.Errorf("codec: field %d: parse float %q: %w", r.pos-1, s, err)
		r.setErr(parseErr)
		return 0, parseErr
	}
	return v, nil
}

// ReadMaxInt reads an int, returning math.MaxInt32 for empty string (TWS sentinel).
func (r *fieldReader) ReadMaxInt() (int, error) {
	s := r.ReadString()
	if s == "" {
		return math.MaxInt32, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		parseErr := fmt.Errorf("codec: field %d: parse int %q: %w", r.pos-1, s, err)
		r.setErr(parseErr)
		return 0, parseErr
	}
	return v, nil
}

// ReadString returns the next field as a string. Returns "" if past end.
func (r *fieldReader) ReadString() string {
	if r.pos >= len(r.fields) {
		return ""
	}
	s := r.fields[r.pos]
	r.pos++
	return s
}

func (r *fieldReader) ReadBool() (bool, error) {
	s := r.ReadString()
	switch s {
	case "1", "true":
		return true, nil
	case "0", "false", "":
		return false, nil
	default:
		parseErr := fmt.Errorf("codec: field %d: parse bool %q", r.pos-1, s)
		r.setErr(parseErr)
		return false, parseErr
	}
}

// ReadDecimal reads a raw decimal string without conversion (preserves precision).
func (r *fieldReader) ReadDecimal() string {
	return r.ReadString()
}

// Skip advances past n fields.
func (r *fieldReader) Skip(n int) {
	r.pos += n
}

// Len returns the total number of fields.
func (r *fieldReader) Len() int {
	return len(r.fields)
}

// Remaining returns how many unread fields remain.
func (r *fieldReader) Remaining() int {
	rem := len(r.fields) - r.pos
	if rem < 0 {
		return 0
	}
	return rem
}

// Pos returns the current read position.
func (r *fieldReader) Pos() int {
	return r.pos
}

func (r *fieldReader) ReadCount(label string) (int, error) {
	if r.pos >= len(r.fields) {
		parseErr := fmt.Errorf("codec: field %d: missing %s", r.pos, label)
		r.setErr(parseErr)
		return 0, parseErr
	}
	s := r.ReadString()
	if s == "" {
		parseErr := fmt.Errorf("codec: field %d: empty %s", r.pos-1, label)
		r.setErr(parseErr)
		return 0, parseErr
	}
	count, err := strconv.Atoi(s)
	if err != nil {
		parseErr := fmt.Errorf("codec: field %d: parse %s %q: %w", r.pos-1, label, s, err)
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
	if r.pos >= len(r.fields) {
		return 0, nil
	}
	s := r.ReadString()
	if s == "" {
		return 0, nil
	}
	count, err := strconv.Atoi(s)
	if err != nil {
		parseErr := fmt.Errorf("codec: field %d: parse %s %q: %w", r.pos-1, label, s, err)
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

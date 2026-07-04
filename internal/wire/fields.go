package wire

import "strings"

// EncodeFields encodes null-delimited string fields with a trailing terminator.
// The exact frame length is known up front, so it fills a single right-sized
// buffer in one pass — no builder growth, no string-to-[]byte round trip.
func EncodeFields(fields []string) []byte {
	if len(fields) == 0 {
		return nil
	}

	size := 0
	for _, field := range fields {
		size += len(field) + 1
	}
	buf := make([]byte, 0, size)
	for _, field := range fields {
		buf = append(buf, field...)
		buf = append(buf, 0)
	}
	return buf
}

// ParseFields parses a null-delimited payload.
func ParseFields(payload []byte) ([]string, error) {
	if len(payload) == 0 {
		return nil, ErrEmptyMessage
	}
	if payload[len(payload)-1] != 0 {
		return nil, ErrMalformedFrame
	}

	parts := strings.Split(string(payload[:len(payload)-1]), "\x00")
	if len(parts) == 0 || parts[0] == "" {
		return nil, ErrMalformedFrame
	}
	return parts, nil
}

package wire

import "strings"

// EncodeFields encodes null-delimited string fields with a trailing terminator.
func EncodeFields(fields []string) []byte {
	if len(fields) == 0 {
		return nil
	}

	var b strings.Builder
	for _, field := range fields {
		b.WriteString(field)
		b.WriteByte(0)
	}
	return []byte(b.String())
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

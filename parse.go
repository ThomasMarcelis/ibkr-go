package ibkr

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
)

const (
	unsetDecimalSentinel    = "-9223372036854775808"
	minMaxFloat64TextLength = len("0x1fffffffffffffp971")
)

func parseRequiredDecimal(raw string, field string) (decimal.Decimal, error) {
	value, err := decimal.NewFromString(raw)
	if err != nil {
		return decimal.Decimal{}, inboundProtocolError(field, err)
	}
	return value, nil
}

func parseOptionalDecimal(raw string, field string) (decimal.Decimal, error) {
	trimmed := strings.TrimSpace(raw)
	if optionalDecimalUnset(trimmed) {
		return decimal.Decimal{}, nil
	}
	value, err := decimal.NewFromString(trimmed)
	if err != nil {
		return decimal.Decimal{}, inboundProtocolError(field, err)
	}
	return value, nil
}

func parseOptionalDecimalPointer(raw string, field string) (*decimal.Decimal, error) {
	trimmed := strings.TrimSpace(raw)
	if optionalDecimalUnset(trimmed) {
		return nil, nil
	}
	value, err := decimal.NewFromString(trimmed)
	if err != nil {
		return nil, inboundProtocolError(field, err)
	}
	return new(value), nil
}

// optionalDecimalUnset recognizes the numeric Double.MAX_VALUE sentinel, not
// one spelling of it. Protobuf formatting uses an explicit '+' in the
// exponent while classic fields do not. NaN, infinities, negative MAX_VALUE,
// and adjacent finite decimals remain values and fail or parse normally.
func optionalDecimalUnset(trimmed string) bool {
	if trimmed == "" || trimmed == unsetDecimalSentinel {
		return true
	}
	// The shortest ParseFloat spelling of MaxFloat64 is hexadecimal; ordinary
	// shorter decimals cannot be the sentinel.
	if len(trimmed) < minMaxFloat64TextLength {
		return false
	}
	value, err := strconv.ParseFloat(trimmed, 64)
	return err == nil && value == math.MaxFloat64
}

func parseOptionalInt(raw string, field string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, inboundProtocolError(field, fmt.Errorf("parse int %q: %w", raw, err))
	}
	return value, nil
}

func parseOptionalInt32(raw string, field string) (int32, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, inboundProtocolError(field, fmt.Errorf("parse int32 %q: %w", raw, err))
	}
	return int32(value), nil // #nosec G115 -- ParseInt's 32-bit bound proves the conversion
}

func parseOptionalInt64(raw string, field string) (int64, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, inboundProtocolError(field, fmt.Errorf("parse int64 %q: %w", raw, err))
	}
	return value, nil
}

func parseOptionalMaxIntPointer(raw string, field string) (*int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == strconv.FormatInt(math.MaxInt32, 10) {
		return nil, nil
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return nil, inboundProtocolError(field, fmt.Errorf("parse int %q: %w", raw, err))
	}
	return new(value), nil
}

func parseOptionalMaxInt64Pointer(raw string, field string) (*int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == strconv.FormatInt(math.MaxInt64, 10) {
		return nil, nil
	}
	value, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return nil, inboundProtocolError(field, fmt.Errorf("parse int64 %q: %w", raw, err))
	}
	return new(value), nil
}

func parseOptionalBoolString(raw string, field string) (bool, error) {
	switch strings.TrimSpace(raw) {
	case "", "0", "false":
		return false, nil
	case "1", "true":
		return true, nil
	default:
		return false, inboundProtocolError(field, fmt.Errorf("parse bool %q", raw))
	}
}

func parseOptionalBoolPointer(raw string, field string) (*bool, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	value, err := parseOptionalBoolString(raw, field)
	if err != nil {
		return nil, err
	}
	return new(value), nil
}

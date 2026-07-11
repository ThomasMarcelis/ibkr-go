package ibkr

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
)

// maxDoubleSentinel is the literal string the TWS/IB Gateway reference Java
// client uses to encode an unset optional double (Double.MAX_VALUE). It arrives
// verbatim on live open-order margin and commission fields, the PnL stream,
// option-computation Greeks, and tick-by-tick fields when the value is unknown.
// The Go client itself emits the empty-string form via WriteMaxFloat, so the
// receive path must accept both.
const (
	maxDoubleSentinel    = "1.7976931348623157E308"
	unsetDecimalSentinel = "-9223372036854775808"
)

func parseRequiredDecimal(raw string, field string) (decimal.Decimal, error) {
	value, err := decimal.NewFromString(raw)
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("ibkr: %s: %w", field, err)
	}
	return value, nil
}

func parseOptionalDecimal(raw string, field string) (decimal.Decimal, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.EqualFold(trimmed, maxDoubleSentinel) || trimmed == unsetDecimalSentinel {
		return decimal.Decimal{}, nil
	}
	value, err := decimal.NewFromString(trimmed)
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("ibkr: %s: %w", field, err)
	}
	return value, nil
}

func parseOptionalDecimalPointer(raw string, field string) (*decimal.Decimal, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.EqualFold(trimmed, maxDoubleSentinel) || trimmed == unsetDecimalSentinel {
		return nil, nil
	}
	value, err := decimal.NewFromString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("ibkr: %s: %w", field, err)
	}
	return new(value), nil
}

func parseOptionalInt(raw string, field string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("ibkr: %s: parse int %q: %w", field, raw, err)
	}
	return value, nil
}

func parseOptionalInt64(raw string, field string) (int64, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("ibkr: %s: parse int64 %q: %w", field, raw, err)
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
		return nil, fmt.Errorf("ibkr: %s: parse int %q: %w", field, raw, err)
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
		return nil, fmt.Errorf("ibkr: %s: parse int64 %q: %w", field, raw, err)
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
		return false, fmt.Errorf("ibkr: %s: parse bool %q", field, raw)
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

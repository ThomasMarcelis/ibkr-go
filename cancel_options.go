package ibkr

import (
	"strconv"
	"strings"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
)

// CancelOption adds operator-entered compliance metadata to an order
// cancellation. Most API-originated cancellations need no options.
type CancelOption func(*cancelConfig)

type cancelConfig struct {
	manualTime      *time.Time
	extOperator     *string
	manualIndicator *int
}

// WithManualCancelTime marks a single-order cancellation as manually entered
// at t. The timestamp is sent in IBKR's UTC yyyyMMdd-HH:mm:ss form. This option
// does not apply to [OrdersClient.CancelAll].
func WithManualCancelTime(t time.Time) CancelOption {
	return func(cfg *cancelConfig) {
		cfg.manualTime = new(t)
	}
}

// WithCancelExternalOperator sets the external operator identifier attached
// to an order cancellation.
func WithCancelExternalOperator(operator string) CancelOption {
	return func(cfg *cancelConfig) {
		cfg.extOperator = new(operator)
	}
}

// WithCancelManualOrderIndicator sets IBKR's CME manual-order indicator on an
// order cancellation. IBKR defines the value for the caller's compliance
// workflow; negative values are rejected locally.
func WithCancelManualOrderIndicator(indicator int) CancelOption {
	return func(cfg *cancelConfig) {
		cfg.manualIndicator = new(indicator)
	}
}

func applyCancelOptions(opts []CancelOption) (cancelConfig, error) {
	var cfg cancelConfig
	for i, opt := range opts {
		if opt == nil {
			return cancelConfig{}, &ValidationError{
				Field: "CancelOption", Value: strconv.Itoa(i), Message: "must not be nil",
			}
		}
		opt(&cfg)
	}
	if cfg.manualTime != nil && cfg.manualTime.IsZero() {
		return cancelConfig{}, &ValidationError{Field: "ManualCancelTime", Message: "must not be zero"}
	}
	if cfg.extOperator != nil {
		if strings.TrimSpace(*cfg.extOperator) == "" {
			return cancelConfig{}, &ValidationError{Field: "CancelExternalOperator", Message: "must not be empty"}
		}
		if strings.ContainsRune(*cfg.extOperator, '\x00') {
			return cancelConfig{}, &ValidationError{Field: "CancelExternalOperator", Message: "must not contain NUL"}
		}
	}
	if cfg.manualIndicator != nil && *cfg.manualIndicator < 0 {
		return cancelConfig{}, &ValidationError{
			Field: "CancelManualOrderIndicator", Value: strconv.Itoa(*cfg.manualIndicator), Message: "must be >= 0",
		}
	}
	return cfg, nil
}

func cancelOrderRequest(orderID int64, cfg cancelConfig) codec.CancelOrderRequest {
	req := codec.CancelOrderRequest{OrderID: orderID}
	if cfg.manualTime != nil {
		req.ManualOrderCancelTime = cfg.manualTime.UTC().Format("20060102-15:04:05")
	}
	if cfg.extOperator != nil {
		req.ExtOperator = *cfg.extOperator
	}
	if cfg.manualIndicator != nil {
		req.ManualOrderIndicator = strconv.Itoa(*cfg.manualIndicator)
	}
	return req
}

func globalCancelRequest(cfg cancelConfig) (codec.GlobalCancelRequest, error) {
	if cfg.manualTime != nil {
		return codec.GlobalCancelRequest{}, &ValidationError{
			Field: "ManualCancelTime", Message: "is only valid for a single-order cancellation",
		}
	}
	var req codec.GlobalCancelRequest
	if cfg.extOperator != nil {
		req.ExtOperator = *cfg.extOperator
	}
	if cfg.manualIndicator != nil {
		req.ManualOrderIndicator = strconv.Itoa(*cfg.manualIndicator)
	}
	return req, nil
}

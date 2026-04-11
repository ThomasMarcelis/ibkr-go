package ibkr

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
)

var validHistoricalBarSizes = map[BarSize]struct{}{
	Bar1Sec:   {},
	Bar5Secs:  {},
	Bar10Secs: {},
	Bar15Secs: {},
	Bar30Secs: {},
	Bar1Min:   {},
	Bar2Mins:  {},
	Bar3Mins:  {},
	Bar5Mins:  {},
	Bar10Mins: {},
	Bar15Mins: {},
	Bar20Mins: {},
	Bar30Mins: {},
	Bar1Hour:  {},
	Bar2Hours: {},
	Bar3Hours: {},
	Bar4Hours: {},
	Bar8Hours: {},
	Bar1Day:   {},
	Bar1Week:  {},
	Bar1Month: {},
}

func buildHistoricalBarsRequest(reqID int, req HistoricalBarsRequest) (codec.HistoricalBarsRequest, error) {
	duration, err := formatHistoricalDuration(req.Duration)
	if err != nil {
		return codec.HistoricalBarsRequest{}, err
	}
	barSize, err := formatHistoricalBarSize(req.BarSize)
	if err != nil {
		return codec.HistoricalBarsRequest{}, err
	}
	return codec.HistoricalBarsRequest{
		ReqID:       reqID,
		Contract:    toCodecContract(req.Contract),
		EndDateTime: formatHistoricalEndTime(req.EndTime),
		Duration:    duration,
		BarSize:     barSize,
		WhatToShow:  string(req.WhatToShow),
		UseRTH:      req.UseRTH,
	}, nil
}

// IBKR documents UTC historical data times as "YYYYMMDD-hh:mm:ss";
// existing replay fixtures freeze this compact UTC encoding.
func formatHistoricalEndTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("20060102-15:04:05")
}

func formatHistoricalDuration(duration HistoricalDuration) (string, error) {
	if strings.TrimSpace(string(duration)) == "" {
		return "", errors.New("ibkr: historical duration must be positive")
	}
	return string(duration), nil
}

func formatHistoricalBarSize(barSize BarSize) (string, error) {
	if strings.TrimSpace(string(barSize)) == "" {
		return "", errors.New("ibkr: historical bar size is required")
	}
	if _, ok := validHistoricalBarSizes[barSize]; !ok {
		return "", fmt.Errorf("ibkr: unsupported historical bar size %s", barSize)
	}
	return string(barSize), nil
}

func validateQuoteRequest(req QuoteRequest, snapshot bool, resume ResumePolicy) error {
	if snapshot && len(req.GenericTicks) > 0 {
		return errors.New("ibkr: quote snapshots do not support generic ticks")
	}
	if snapshot && resume == ResumeAuto {
		return errors.New("ibkr: quote snapshots do not support automatic resume")
	}
	return nil
}

func formatGenericTicks(values []GenericTick) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = string(value)
	}
	return out
}

func formatProviderCodes(values []NewsProviderCode) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = string(value)
	}
	return strings.Join(parts, "+")
}

// IBKR historical tick examples use "yyyyMMdd HH:mm:ss" with an optional
// timezone; without one TWS uses the login timezone. Always send an explicit
// zone so time.Time values keep their absolute instant across login zones.
func formatHistoricalTickTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return formatTimeWithZone(t, "20060102 15:04:05")
}

// IBKR historical news documents "yyyy-MM-dd HH:mm:ss". Use the same explicit
// zone policy as historical ticks so a non-UTC login zone cannot shift windows.
func formatHistoricalNewsTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return formatTimeWithZone(t, "2006-01-02 15:04:05")
}

func formatTimeWithZone(t time.Time, layout string) string {
	zone := t.Location().String()
	if zone == "" || zone == "Local" {
		t = t.UTC()
		zone = "UTC"
	}
	return t.Format(layout) + " " + zone
}

func formatWSHDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("20060102")
}

func validateResumePolicy(opKind OpKind, resume ResumePolicy) error {
	if resume != ResumeAuto {
		return nil
	}
	switch opKind {
	case OpQuotes, OpRealTimeBars:
		return nil
	default:
		return fmt.Errorf("ibkr: %s subscriptions do not support automatic resume", opKind)
	}
}

func validateOpenOrdersScope(scope OpenOrdersScope, clientID int) error {
	switch scope {
	case OpenOrdersScopeAll, OpenOrdersScopeClient:
		return nil
	case OpenOrdersScopeAuto:
		if clientID != 0 {
			return errors.New("ibkr: open orders auto scope requires client ID 0")
		}
		return nil
	default:
		return fmt.Errorf("ibkr: unsupported open orders scope %q", scope)
	}
}

package session

import (
	"errors"
	"fmt"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
)

var historicalBarSizes = map[time.Duration]string{
	1 * time.Second:    "1 sec",
	5 * time.Second:    "5 secs",
	10 * time.Second:   "10 secs",
	15 * time.Second:   "15 secs",
	30 * time.Second:   "30 secs",
	1 * time.Minute:    "1 min",
	2 * time.Minute:    "2 mins",
	3 * time.Minute:    "3 mins",
	5 * time.Minute:    "5 mins",
	10 * time.Minute:   "10 mins",
	15 * time.Minute:   "15 mins",
	20 * time.Minute:   "20 mins",
	30 * time.Minute:   "30 mins",
	1 * time.Hour:      "1 hour",
	2 * time.Hour:      "2 hours",
	3 * time.Hour:      "3 hours",
	4 * time.Hour:      "4 hours",
	8 * time.Hour:      "8 hours",
	24 * time.Hour:     "1 day",
	7 * 24 * time.Hour: "1 week",
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
		WhatToShow:  req.WhatToShow,
		UseRTH:      req.UseRTH,
	}, nil
}

// formatHistoricalEndTime returns the IBKR-format end time string.
// IBKR expects "yyyyMMdd-HH:mm:ss" in UTC (or empty for most recent).
func formatHistoricalEndTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("20060102-15:04:05")
}

func formatHistoricalDuration(duration time.Duration) (string, error) {
	if duration <= 0 {
		return "", errors.New("ibkr: historical duration must be positive")
	}
	if duration%time.Second != 0 {
		return "", fmt.Errorf("ibkr: historical duration %s must be a whole number of seconds", duration)
	}
	if duration%(24*time.Hour) == 0 {
		return fmt.Sprintf("%d D", int64(duration/(24*time.Hour))), nil
	}
	return fmt.Sprintf("%d S", int64(duration/time.Second)), nil
}

func formatHistoricalBarSize(barSize time.Duration) (string, error) {
	if value, ok := historicalBarSizes[barSize]; ok {
		return value, nil
	}
	return "", fmt.Errorf("ibkr: unsupported historical bar size %s", barSize)
}

func validateQuoteRequest(req QuoteSubscriptionRequest, resume ResumePolicy) error {
	if req.Snapshot && len(req.GenericTicks) > 0 {
		return errors.New("ibkr: quote snapshots do not support generic ticks")
	}
	if req.Snapshot && resume == ResumeAuto {
		return errors.New("ibkr: quote snapshots do not support automatic resume")
	}
	return nil
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

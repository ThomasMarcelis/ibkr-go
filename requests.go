package ibkr

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
)

func buildHistoricalBarsRequest(reqID int, req HistoricalBarsRequest) (sdkadapter.HistoricalBarsRequest, error) {
	duration, err := formatHistoricalDuration(req.Duration)
	if err != nil {
		return sdkadapter.HistoricalBarsRequest{}, err
	}
	barSize, err := formatHistoricalBarSize(req.BarSize)
	if err != nil {
		return sdkadapter.HistoricalBarsRequest{}, err
	}
	return sdkadapter.HistoricalBarsRequest{
		ReqID:       reqID,
		Contract:    toCodecContract(req.Contract),
		EndDateTime: formatHistoricalEndTime(req.EndTime),
		Duration:    duration,
		BarSize:     barSize,
		WhatToShow:  string(req.WhatToShow),
		UseRTH:      req.UseRTH,
	}, nil
}

func validateHistoricalBarsRequest(req HistoricalBarsRequest) error {
	if !req.WhatToShow.Valid() {
		return &ValidationError{
			Field:   "WhatToShow",
			Value:   string(req.WhatToShow),
			Message: "unsupported historical data type",
		}
	}
	if req.WhatToShow == ShowSchedule {
		// whatToShow=SCHEDULE has its own return shape (historicalSchedule,
		// msg_id 106) and does not produce Bar values. Callers should use
		// History().Schedule instead of History().Bars so they get the
		// correct typed response.
		return &ValidationError{
			Field:   "WhatToShow",
			Value:   string(req.WhatToShow),
			Message: "History().Bars does not support schedule data; use History().Schedule instead",
		}
	}
	if _, err := formatHistoricalDuration(req.Duration); err != nil {
		return err
	}
	if _, err := formatHistoricalBarSize(req.BarSize); err != nil {
		return err
	}
	return nil
}

func validateHistoricalBarsStreamRequest(req HistoricalBarsRequest) error {
	if err := validateHistoricalBarsRequest(req); err != nil {
		return err
	}
	if !req.EndTime.IsZero() {
		return &ValidationError{
			Field:   "EndTime",
			Value:   formatHistoricalEndTime(req.EndTime),
			Message: "must be empty for streaming historical bars",
		}
	}
	switch req.WhatToShow {
	case ShowTrades, ShowMidpoint, ShowBid, ShowAsk:
		return nil
	default:
		return &ValidationError{
			Field:   "WhatToShow",
			Value:   string(req.WhatToShow),
			Message: "streaming historical bars support TRADES, MIDPOINT, BID, or ASK",
		}
	}
}

// buildHistoricalScheduleRequest constructs the outbound wire request for a
// HistoricalSchedule one-shot. It reuses sdkadapter.HistoricalBarsRequest under the
// hood because REQ_HISTORICAL_DATA (msg_id 20) is the same outbound message;
// the inbound path is a separate InHistoricalSchedule decoder.
func buildHistoricalScheduleRequest(reqID int, req HistoricalScheduleRequest) (sdkadapter.HistoricalBarsRequest, error) {
	duration, err := formatHistoricalDuration(req.Duration)
	if err != nil {
		return sdkadapter.HistoricalBarsRequest{}, err
	}
	barSize, err := formatHistoricalBarSize(req.BarSize)
	if err != nil {
		return sdkadapter.HistoricalBarsRequest{}, err
	}
	return sdkadapter.HistoricalBarsRequest{
		ReqID:       reqID,
		Contract:    toCodecContract(req.Contract),
		EndDateTime: formatHistoricalEndTime(req.EndTime),
		Duration:    duration,
		BarSize:     barSize,
		WhatToShow:  string(ShowSchedule),
		UseRTH:      req.UseRTH,
	}, nil
}

func validateHistoricalScheduleRequest(req HistoricalScheduleRequest) error {
	if _, err := formatHistoricalDuration(req.Duration); err != nil {
		return err
	}
	if _, err := formatHistoricalBarSize(req.BarSize); err != nil {
		return err
	}
	if req.BarSize != Bar1Day {
		return &ValidationError{
			Field:   "BarSize",
			Value:   string(req.BarSize),
			Message: "schedule data requires 1 day bars",
		}
	}
	return nil
}

func historicalBarsPacingKey(req HistoricalBarsRequest) string {
	return strings.Join([]string{
		historicalContractPacingKey(req.Contract),
		formatHistoricalEndTime(req.EndTime),
		string(req.Duration),
		string(req.BarSize),
		string(req.WhatToShow),
		fmt.Sprintf("%t", req.UseRTH),
	}, "\x00")
}

func historicalSchedulePacingKey(req HistoricalScheduleRequest) string {
	return strings.Join([]string{
		historicalContractPacingKey(req.Contract),
		formatHistoricalEndTime(req.EndTime),
		string(req.Duration),
		string(req.BarSize),
		string(ShowSchedule),
		fmt.Sprintf("%t", req.UseRTH),
	}, "\x00")
}

func historicalContractPacingKey(contract Contract) string {
	return strings.Join([]string{
		contract.Symbol,
		string(contract.SecType),
		contract.Expiry,
		contract.Strike,
		string(contract.Right),
		contract.Multiplier,
		contract.Exchange,
		contract.PrimaryExchange,
		contract.Currency,
		contract.LocalSymbol,
		contract.TradingClass,
		fmt.Sprintf("%d", contract.ConID),
	}, "\x00")
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
		return "", &ValidationError{
			Field:   "Duration",
			Value:   string(duration),
			Message: "must be positive",
		}
	}
	if !duration.Valid() {
		return "", &ValidationError{
			Field:   "Duration",
			Value:   string(duration),
			Message: "must match N S|D|W|M|Y",
		}
	}
	return string(duration), nil
}

func formatHistoricalBarSize(barSize BarSize) (string, error) {
	if strings.TrimSpace(string(barSize)) == "" {
		return "", &ValidationError{
			Field:   "BarSize",
			Value:   string(barSize),
			Message: "is required",
		}
	}
	if !barSize.Valid() {
		return "", &ValidationError{
			Field:   "BarSize",
			Value:   string(barSize),
			Message: "unsupported historical bar size",
		}
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

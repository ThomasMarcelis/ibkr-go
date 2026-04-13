package ibkr

import (
	"errors"
	"testing"
	"time"
)

func TestHistoricalVocabularyValidMethods(t *testing.T) {
	t.Parallel()

	barSizes := []struct {
		value BarSize
		valid bool
	}{
		{value: Bar1Sec, valid: true},
		{value: Bar1Month, valid: true},
		{value: BarSize("90 mins"), valid: false},
		{value: BarSize(""), valid: false},
	}
	for _, tt := range barSizes {
		if got := tt.value.Valid(); got != tt.valid {
			t.Fatalf("BarSize(%q).Valid() = %v, want %v", tt.value, got, tt.valid)
		}
	}

	durations := []struct {
		value HistoricalDuration
		valid bool
	}{
		{value: Days(1), valid: true},
		{value: Minutes(90), valid: true},
		{value: HistoricalDuration("0 D"), valid: false},
		{value: HistoricalDuration("1 fortnight"), valid: false},
		{value: HistoricalDuration(""), valid: false},
	}
	for _, tt := range durations {
		if got := tt.value.Valid(); got != tt.valid {
			t.Fatalf("HistoricalDuration(%q).Valid() = %v, want %v", tt.value, got, tt.valid)
		}
	}

	whatToShow := []struct {
		value WhatToShow
		valid bool
	}{
		{value: ShowTrades, valid: true},
		{value: ShowMidpoint, valid: true},
		{value: ShowBid, valid: true},
		{value: ShowAsk, valid: true},
		{value: ShowBidAsk, valid: true},
		{value: ShowHistoricalVolatility, valid: true},
		{value: ShowOptionImpliedVolatility, valid: true},
		{value: ShowAdjustedLast, valid: true},
		{value: ShowFeeRate, valid: true},
		{value: ShowYieldBid, valid: true},
		{value: ShowYieldAsk, valid: true},
		{value: ShowYieldBidAsk, valid: true},
		{value: ShowYieldLast, valid: true},
		{value: ShowSchedule, valid: true},
		{value: ShowAggTrades, valid: true},
		{value: WhatToShow("FEELINGS"), valid: false},
		{value: WhatToShow(""), valid: false},
	}
	for _, tt := range whatToShow {
		if got := tt.value.Valid(); got != tt.valid {
			t.Fatalf("WhatToShow(%q).Valid() = %v, want %v", tt.value, got, tt.valid)
		}
	}
}

func TestFormatHistoricalDuration(t *testing.T) {
	t.Parallel()

	got, err := formatHistoricalDuration(Days(1))
	if err != nil {
		t.Fatalf("formatHistoricalDuration() error = %v", err)
	}
	if got != "1 D" {
		t.Fatalf("formatHistoricalDuration() = %q, want %q", got, "1 D")
	}

	got, err = formatHistoricalDuration(Minutes(90))
	if err != nil {
		t.Fatalf("formatHistoricalDuration() error = %v", err)
	}
	if got != "5400 S" {
		t.Fatalf("formatHistoricalDuration() = %q, want %q", got, "5400 S")
	}

	if _, err := formatHistoricalDuration(HistoricalDuration("1 fortnight")); !isValidationField(err, "Duration") {
		t.Fatalf("formatHistoricalDuration() error = %v, want Duration validation error", err)
	}
}

func TestFormatHistoricalBarSize(t *testing.T) {
	t.Parallel()

	got, err := formatHistoricalBarSize(Bar1Hour)
	if err != nil {
		t.Fatalf("formatHistoricalBarSize() error = %v", err)
	}
	if got != "1 hour" {
		t.Fatalf("formatHistoricalBarSize() = %q, want %q", got, "1 hour")
	}

	if _, err := formatHistoricalBarSize(BarSize("90 mins")); !isValidationField(err, "BarSize") {
		t.Fatalf("formatHistoricalBarSize() error = %v, want BarSize validation error", err)
	}
}

func TestValidateHistoricalBarsRequestErrorsAreTyped(t *testing.T) {
	t.Parallel()

	base := HistoricalBarsRequest{
		Contract: Contract{
			Symbol:   "AAPL",
			SecType:  SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Duration:   Days(1),
		BarSize:    Bar1Hour,
		WhatToShow: ShowTrades,
		UseRTH:     true,
	}

	testCases := []struct {
		name  string
		req   HistoricalBarsRequest
		field string
	}{
		{name: "schedule", req: withWhatToShow(base, ShowSchedule), field: "WhatToShow"},
		{name: "unsupported what to show", req: withWhatToShow(base, WhatToShow("FEELINGS")), field: "WhatToShow"},
		{name: "invalid duration", req: withDuration(base, HistoricalDuration("1 fortnight")), field: "Duration"},
		{name: "invalid bar size", req: withBarSize(base, BarSize("90 mins")), field: "BarSize"},
	}

	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := validateHistoricalBarsRequest(tt.req); !isValidationField(err, tt.field) {
				t.Fatalf("validateHistoricalBarsRequest() error = %v, want %s validation error", err, tt.field)
			}
		})
	}
}

func TestValidateHistoricalBarsRequestAcceptsFullVocabulary(t *testing.T) {
	t.Parallel()

	base := HistoricalBarsRequest{
		Contract: Contract{
			Symbol:   "AAPL",
			SecType:  SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Duration: Days(1),
		BarSize:  Bar1Hour,
		UseRTH:   true,
	}
	for _, value := range []WhatToShow{
		ShowTrades,
		ShowMidpoint,
		ShowBid,
		ShowAsk,
		ShowBidAsk,
		ShowHistoricalVolatility,
		ShowOptionImpliedVolatility,
		ShowAdjustedLast,
		ShowFeeRate,
		ShowYieldBid,
		ShowYieldAsk,
		ShowYieldBidAsk,
		ShowYieldLast,
		ShowAggTrades,
	} {
		req := withWhatToShow(base, value)
		if err := validateHistoricalBarsRequest(req); err != nil {
			t.Fatalf("validateHistoricalBarsRequest(%q) error = %v", value, err)
		}
	}
}

func TestValidateHistoricalBarsStreamRequest(t *testing.T) {
	t.Parallel()

	base := HistoricalBarsRequest{
		Contract: Contract{
			Symbol:   "AAPL",
			SecType:  SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Duration:   Days(1),
		BarSize:    Bar1Hour,
		WhatToShow: ShowTrades,
		UseRTH:     true,
	}
	for _, value := range []WhatToShow{ShowTrades, ShowMidpoint, ShowBid, ShowAsk} {
		req := withWhatToShow(base, value)
		if err := validateHistoricalBarsStreamRequest(req); err != nil {
			t.Fatalf("validateHistoricalBarsStreamRequest(%q) error = %v", value, err)
		}
	}

	nonZeroEnd := withEndTime(base, time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC))
	if err := validateHistoricalBarsStreamRequest(nonZeroEnd); !isValidationField(err, "EndTime") {
		t.Fatalf("validateHistoricalBarsStreamRequest() error = %v, want EndTime validation error", err)
	}

	for _, value := range []WhatToShow{ShowBidAsk, ShowAdjustedLast, ShowSchedule, ShowAggTrades} {
		req := withWhatToShow(base, value)
		if err := validateHistoricalBarsStreamRequest(req); !isValidationField(err, "WhatToShow") {
			t.Fatalf("validateHistoricalBarsStreamRequest(%q) error = %v, want WhatToShow validation error", value, err)
		}
	}
}

func TestValidateHistoricalScheduleRequestRequiresDailyBars(t *testing.T) {
	t.Parallel()

	req := HistoricalScheduleRequest{
		Contract: Contract{
			Symbol:   "AAPL",
			SecType:  SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Duration: Days(1),
		BarSize:  Bar1Day,
		UseRTH:   true,
	}
	if err := validateHistoricalScheduleRequest(req); err != nil {
		t.Fatalf("validateHistoricalScheduleRequest() error = %v", err)
	}

	req.BarSize = Bar1Hour
	if err := validateHistoricalScheduleRequest(req); !isValidationField(err, "BarSize") {
		t.Fatalf("validateHistoricalScheduleRequest() error = %v, want BarSize validation error", err)
	}
}

func withWhatToShow(req HistoricalBarsRequest, value WhatToShow) HistoricalBarsRequest {
	req.WhatToShow = value
	return req
}

func withDuration(req HistoricalBarsRequest, value HistoricalDuration) HistoricalBarsRequest {
	req.Duration = value
	return req
}

func withBarSize(req HistoricalBarsRequest, value BarSize) HistoricalBarsRequest {
	req.BarSize = value
	return req
}

func withEndTime(req HistoricalBarsRequest, value time.Time) HistoricalBarsRequest {
	req.EndTime = value
	return req
}

func isValidationField(err error, field string) bool {
	validationErr, ok := errors.AsType[*ValidationError](err)
	return ok && validationErr.Field == field
}

func TestFormatProviderCodes(t *testing.T) {
	t.Parallel()

	if got := formatProviderCodes(nil); got != "" {
		t.Fatalf("formatProviderCodes(nil) = %q, want empty", got)
	}

	got := formatProviderCodes([]NewsProviderCode{"BZ", "FLY"})
	if got != "BZ+FLY" {
		t.Fatalf("formatProviderCodes() = %q, want %q", got, "BZ+FLY")
	}
}

func TestFormatHistoricalTickTime(t *testing.T) {
	t.Parallel()

	if got := formatHistoricalTickTime(time.Time{}); got != "" {
		t.Fatalf("formatHistoricalTickTime(zero) = %q, want empty", got)
	}

	timestamp := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	if got := formatHistoricalTickTime(timestamp); got != "20260405 12:00:00 UTC" {
		t.Fatalf("formatHistoricalTickTime() = %q, want %q", got, "20260405 12:00:00 UTC")
	}

	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v", err)
	}
	timestamp = time.Date(2026, 4, 5, 8, 0, 0, 0, newYork)
	if got := formatHistoricalTickTime(timestamp); got != "20260405 08:00:00 America/New_York" {
		t.Fatalf("formatHistoricalTickTime() = %q, want %q", got, "20260405 08:00:00 America/New_York")
	}
}

func TestFormatHistoricalNewsTime(t *testing.T) {
	t.Parallel()

	if got := formatHistoricalNewsTime(time.Time{}); got != "" {
		t.Fatalf("formatHistoricalNewsTime(zero) = %q, want empty", got)
	}

	timestamp := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	if got := formatHistoricalNewsTime(timestamp); got != "2026-04-05 12:00:00 UTC" {
		t.Fatalf("formatHistoricalNewsTime() = %q, want %q", got, "2026-04-05 12:00:00 UTC")
	}

	amsterdam, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v", err)
	}
	timestamp = time.Date(2026, 4, 5, 14, 0, 0, 0, amsterdam)
	if got := formatHistoricalNewsTime(timestamp); got != "2026-04-05 14:00:00 Europe/Amsterdam" {
		t.Fatalf("formatHistoricalNewsTime() = %q, want %q", got, "2026-04-05 14:00:00 Europe/Amsterdam")
	}
}

func TestNewAccountSummaryPlanUsesAllGroup(t *testing.T) {
	t.Parallel()

	plan := newAccountSummaryPlan(7, AccountSummaryRequest{
		Account: "DU12345",
		Tags:    []string{"NetLiquidation"},
	})
	if plan.request.Account != "All" {
		t.Fatalf("newAccountSummaryPlan().request.Account = %q, want %q", plan.request.Account, "All")
	}
}

func TestAccountSummaryPlanMatchesWildcardAndConcreteAccounts(t *testing.T) {
	t.Parallel()

	allPlan := newAccountSummaryPlan(7, AccountSummaryRequest{Account: "All"})
	if !allPlan.matches("DU12345") || !allPlan.matches("DU99999") {
		t.Fatal("newAccountSummaryPlan(Account=All) did not match all accounts")
	}

	emptyPlan := newAccountSummaryPlan(7, AccountSummaryRequest{})
	if !emptyPlan.matches("DU12345") || !emptyPlan.matches("DU99999") {
		t.Fatal("newAccountSummaryPlan(Account=\"\") did not match all accounts")
	}

	concretePlan := newAccountSummaryPlan(7, AccountSummaryRequest{Account: "DU12345"})
	if !concretePlan.matches("DU12345") {
		t.Fatal("newAccountSummaryPlan(concrete).matches() = false, want true")
	}
	if concretePlan.matches("DU99999") {
		t.Fatal("newAccountSummaryPlan(concrete).matches() = true, want false")
	}
}

func TestValidateQuoteRequest(t *testing.T) {
	t.Parallel()

	err := validateQuoteRequest(QuoteRequest{
		GenericTicks: []GenericTick{"233"},
	}, true, ResumeNever)
	if err == nil {
		t.Fatal("validateQuoteRequest() error = nil, want rejection")
	}

	err = validateQuoteRequest(QuoteRequest{}, true, ResumeAuto)
	if err == nil {
		t.Fatal("validateQuoteRequest() error = nil, want auto-resume rejection")
	}
}

func TestValidateOpenOrdersScope(t *testing.T) {
	t.Parallel()

	if err := validateOpenOrdersScope(OpenOrdersScopeAuto, 1); err == nil {
		t.Fatal("validateOpenOrdersScope() error = nil, want client-id rejection")
	}
	if err := validateOpenOrdersScope(OpenOrdersScopeAuto, 0); err != nil {
		t.Fatalf("validateOpenOrdersScope() error = %v", err)
	}
}

func TestValidateResumePolicy(t *testing.T) {
	t.Parallel()

	if err := validateResumePolicy(OpQuotes, ResumeAuto); err != nil {
		t.Fatalf("validateResumePolicy() error = %v", err)
	}
	if err := validateResumePolicy(OpRealTimeBars, ResumeAuto); err != nil {
		t.Fatalf("validateResumePolicy() error = %v", err)
	}
	if err := validateResumePolicy(OpExecutions, ResumeAuto); err == nil {
		t.Fatal("validateResumePolicy() error = nil, want rejection")
	}
}

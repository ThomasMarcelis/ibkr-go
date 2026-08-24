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

func TestValidateHistoricalDuration(t *testing.T) {
	t.Parallel()

	if err := validateHistoricalDuration(Days(1)); err != nil {
		t.Fatalf("validateHistoricalDuration() error = %v", err)
	}

	if err := validateHistoricalDuration(Minutes(90)); err != nil {
		t.Fatalf("validateHistoricalDuration() error = %v", err)
	}

	if err := validateHistoricalDuration(HistoricalDuration("1 fortnight")); !isValidationField(err, "Duration") {
		t.Fatalf("validateHistoricalDuration() error = %v, want Duration validation error", err)
	}
}

func TestValidateHistoricalBarSize(t *testing.T) {
	t.Parallel()

	if err := validateHistoricalBarSize(Bar1Hour); err != nil {
		t.Fatalf("validateHistoricalBarSize() error = %v", err)
	}

	if err := validateHistoricalBarSize(BarSize("90 mins")); !isValidationField(err, "BarSize") {
		t.Fatalf("validateHistoricalBarSize() error = %v, want BarSize validation error", err)
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

func TestFormatProviderCodesOwnsCapturedRequest(t *testing.T) {
	t.Parallel()

	if got := formatProviderCodes(nil); got != "" {
		t.Fatalf("formatProviderCodes(nil) = %q, want empty", got)
	}

	// captures/20260824T202418Z-api_news_article_aapl, server_version 225,
	// events.jsonl SHA-256 b7c40100f09e2b865268d16af2e458360cb610a5028a4153745c0c6d0e525215.
	providers := []NewsProviderCode{"BRFG", "BRFUPDN", "DJNL"}
	got := formatProviderCodes(providers)
	providers[0] = "mutated"
	if got != "BRFG+BRFUPDN+DJNL" {
		t.Fatalf("formatProviderCodes() = %q, want %q", got, "BRFG+BRFUPDN+DJNL")
	}
}

func TestFormatGenericTicksOwnsCapturedRequest(t *testing.T) {
	t.Parallel()

	// captures/20260824T202842Z-quote_stream_genericticks, server_version 225,
	// events.jsonl SHA-256 87ff4c1b76c6e94c4cbec0cc20e230750d29aabf1137e5919bb3113dbd8a556f.
	ticks := []GenericTick{"233", "236"}
	formatted := formatGenericTicks(ticks)
	ticks[0] = "mutated"
	if len(formatted) != 2 || formatted[0] != "233" || formatted[1] != "236" {
		t.Fatalf("formatGenericTicks() = %v, want [233 236]", formatted)
	}
}

func TestCloneAccountSummaryRequestOwnsCapturedTags(t *testing.T) {
	t.Parallel()

	// captures/20260824T202344Z-account_summary_snapshot, server_version 225,
	// events.jsonl SHA-256 6f8ede19db82acc23de6bd988381d40b339af0270133fdcabceba33082f6c181.
	req := AccountSummaryRequest{Tags: []string{
		"NetLiquidation", "TotalCashValue", "BuyingPower", "ExcessLiquidity",
	}}
	cloned := cloneAccountSummaryRequest(req)
	req.Tags[0] = "mutated"
	if got := cloned.Tags[0]; got != "NetLiquidation" {
		t.Fatalf("cloned Tags[0] = %q, want NetLiquidation", got)
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

func TestValidateHistoricalTicksRequest(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	base := HistoricalTicksRequest{
		Contract: Contract{Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD"},
		EndTime:  now, NumberOfTicks: 100, WhatToShow: ShowTrades, UseRTH: true,
	}
	if err := validateHistoricalTicksRequest(base); err != nil {
		t.Fatalf("validateHistoricalTicksRequest(end bound) error = %v", err)
	}
	startBound := base
	startBound.StartTime, startBound.EndTime = now.Add(-time.Hour), time.Time{}
	if err := validateHistoricalTicksRequest(startBound); err != nil {
		t.Fatalf("validateHistoricalTicksRequest(start bound) error = %v", err)
	}

	testCases := []struct {
		name   string
		mutate func(*HistoricalTicksRequest)
		field  string
	}{
		{name: "neither bound", mutate: func(req *HistoricalTicksRequest) { req.EndTime = time.Time{} }, field: "StartTime/EndTime"},
		{name: "both bounds", mutate: func(req *HistoricalTicksRequest) { req.StartTime = now.Add(-time.Hour) }, field: "StartTime/EndTime"},
		{name: "zero count", mutate: func(req *HistoricalTicksRequest) { req.NumberOfTicks = 0 }, field: "NumberOfTicks"},
		{name: "count above limit", mutate: func(req *HistoricalTicksRequest) { req.NumberOfTicks = 1001 }, field: "NumberOfTicks"},
		{name: "unsupported kind", mutate: func(req *HistoricalTicksRequest) { req.WhatToShow = ShowBid }, field: "WhatToShow"},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			req := base
			tt.mutate(&req)
			if err := validateHistoricalTicksRequest(req); !isValidationField(err, tt.field) {
				t.Fatalf("validateHistoricalTicksRequest() error = %v, want %s validation error", err, tt.field)
			}
		})
	}
}

func TestFormatHistoricalNewsTime(t *testing.T) {
	t.Parallel()
	// The sv225 api_news_article_aapl capture returned historical-news rows
	// with the documented .0 bounds (events SHA-256
	// b7c40100f09e2b865268d16af2e458360cb610a5028a4153745c0c6d0e525215).

	if got := formatHistoricalNewsTime(time.Time{}); got != "" {
		t.Fatalf("formatHistoricalNewsTime(zero) = %q, want empty", got)
	}

	timestamp := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	if got := formatHistoricalNewsTime(timestamp); got != "2026-04-05 12:00:00.0 UTC" {
		t.Fatalf("formatHistoricalNewsTime() = %q, want %q", got, "2026-04-05 12:00:00.0 UTC")
	}

	amsterdam, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v", err)
	}
	timestamp = time.Date(2026, 4, 5, 14, 0, 0, 0, amsterdam)
	if got := formatHistoricalNewsTime(timestamp); got != "2026-04-05 14:00:00.0 Europe/Amsterdam" {
		t.Fatalf("formatHistoricalNewsTime() = %q, want %q", got, "2026-04-05 14:00:00.0 Europe/Amsterdam")
	}
}

func TestValidateHistoricalNewsRequest(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	for _, req := range []HistoricalNewsRequest{
		{TotalResults: 1},
		{StartTime: now, TotalResults: 300},
		{EndTime: now, TotalResults: 10},
	} {
		if err := validateHistoricalNewsRequest(req); err != nil {
			t.Fatalf("validateHistoricalNewsRequest() error = %v", err)
		}
	}

	testCases := []struct {
		name  string
		req   HistoricalNewsRequest
		field string
	}{
		{name: "both bounds", req: HistoricalNewsRequest{StartTime: now.Add(-time.Hour), EndTime: now, TotalResults: 10}, field: "StartTime/EndTime"},
		{name: "zero results", req: HistoricalNewsRequest{}, field: "TotalResults"},
		{name: "too many results", req: HistoricalNewsRequest{TotalResults: 301}, field: "TotalResults"},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := validateHistoricalNewsRequest(tt.req); !isValidationField(err, tt.field) {
				t.Fatalf("validateHistoricalNewsRequest() error = %v, want %s validation error", err, tt.field)
			}
		})
	}
}

func TestNewAccountSummaryPlanSelectsGroup(t *testing.T) {
	t.Parallel()

	defaultPlan := newAccountSummaryPlan(7, AccountSummaryRequest{})
	if defaultPlan.request.Account != "All" {
		t.Fatalf("newAccountSummaryPlan(default).request.Account = %q, want %q", defaultPlan.request.Account, "All")
	}

	groupPlan := newAccountSummaryPlan(7, AccountSummaryRequest{
		Group: "AdvisorGroup",
		Tags:  []string{"NetLiquidation"},
	})
	if groupPlan.request.Account != "AdvisorGroup" {
		t.Fatalf("newAccountSummaryPlan(group).request.Account = %q, want %q", groupPlan.request.Account, "AdvisorGroup")
	}
}

func TestAccountSummaryPlanMatchesWildcardAndConcreteAccounts(t *testing.T) {
	t.Parallel()

	allPlan := newAccountSummaryPlan(7, AccountSummaryRequest{Group: "All"})
	if !allPlan.matches("DU12345") || !allPlan.matches("DU99999") {
		t.Fatal("newAccountSummaryPlan(Group=All) did not match all accounts")
	}

	emptyPlan := newAccountSummaryPlan(7, AccountSummaryRequest{})
	if !emptyPlan.matches("DU12345") || !emptyPlan.matches("DU99999") {
		t.Fatal("newAccountSummaryPlan(Account=\"\") did not match all accounts")
	}

	concretePlan := newAccountSummaryPlan(7, AccountSummaryRequest{AccountFilter: "DU12345"})
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
	if !isValidationField(err, "QuoteRequest.GenericTicks") {
		t.Fatalf("validateQuoteRequest() error = %v, want QuoteRequest.GenericTicks validation", err)
	}

	err = validateQuoteRequest(QuoteRequest{}, true, ResumeAuto)
	if !isValidationField(err, "ResumePolicy") {
		t.Fatalf("validateQuoteRequest() error = %v, want ResumePolicy validation", err)
	}
}

func TestValidateOpenOrdersScope(t *testing.T) {
	t.Parallel()

	if err := validateOpenOrdersScope(OpenOrdersScopeAuto, 1); !isValidationField(err, "OpenOrdersScope") {
		t.Fatalf("validateOpenOrdersScope() error = %v, want OpenOrdersScope validation", err)
	}
	if err := validateOpenOrdersScope(OpenOrdersScopeAuto, 0); err != nil {
		t.Fatalf("validateOpenOrdersScope() error = %v", err)
	}
	if err := validateOpenOrdersScope(OpenOrdersScope("other"), 0); !isValidationField(err, "OpenOrdersScope") {
		t.Fatalf("validateOpenOrdersScope() invalid scope error = %v, want OpenOrdersScope validation", err)
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
	if err := validateResumePolicy(OpExecutions, ResumeAuto); !isValidationField(err, "ResumePolicy") {
		t.Fatalf("validateResumePolicy() error = %v, want ResumePolicy validation", err)
	}
}

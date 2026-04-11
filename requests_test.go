package ibkr

import (
	"testing"
	"time"
)

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

	if _, err := formatHistoricalBarSize(BarSize("90 mins")); err == nil {
		t.Fatal("formatHistoricalBarSize() error = nil, want rejection")
	}
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

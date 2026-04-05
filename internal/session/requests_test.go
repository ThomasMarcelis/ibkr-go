package session

import (
	"testing"
	"time"
)

func TestFormatHistoricalDuration(t *testing.T) {
	t.Parallel()

	got, err := formatHistoricalDuration(24 * time.Hour)
	if err != nil {
		t.Fatalf("formatHistoricalDuration() error = %v", err)
	}
	if got != "1 D" {
		t.Fatalf("formatHistoricalDuration() = %q, want %q", got, "1 D")
	}

	got, err = formatHistoricalDuration(90 * time.Minute)
	if err != nil {
		t.Fatalf("formatHistoricalDuration() error = %v", err)
	}
	if got != "5400 S" {
		t.Fatalf("formatHistoricalDuration() = %q, want %q", got, "5400 S")
	}
}

func TestFormatHistoricalBarSize(t *testing.T) {
	t.Parallel()

	got, err := formatHistoricalBarSize(time.Hour)
	if err != nil {
		t.Fatalf("formatHistoricalBarSize() error = %v", err)
	}
	if got != "1 hour" {
		t.Fatalf("formatHistoricalBarSize() = %q, want %q", got, "1 hour")
	}

	if _, err := formatHistoricalBarSize(90 * time.Minute); err == nil {
		t.Fatal("formatHistoricalBarSize() error = nil, want rejection")
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

	err := validateQuoteRequest(QuoteSubscriptionRequest{
		Snapshot:     true,
		GenericTicks: []string{"233"},
	}, ResumeNever)
	if err == nil {
		t.Fatal("validateQuoteRequest() error = nil, want rejection")
	}

	err = validateQuoteRequest(QuoteSubscriptionRequest{
		Snapshot: true,
	}, ResumeAuto)
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

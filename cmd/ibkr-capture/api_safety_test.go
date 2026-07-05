package main

import (
	"context"
	"strings"
	"testing"
)

// TestCancelsAllowedForRiskClass freezes the single source of truth for which
// risk classes may mutate order state. Only the four paper-trading classes run
// the pre/post global cancel; every other class (which may capture against the
// real-money readonly-live role) must not.
func TestCancelsAllowedForRiskClass(t *testing.T) {
	t.Parallel()

	allowed := map[string]bool{
		"paper_order":            true,
		"paper_marketable_order": true,
		"paper_trigger":          true,
		"paper_destructive":      true,
	}
	refused := []string{"read_only", "entitlement_probe", "", "unknown_future_class"}

	for class, want := range allowed {
		if got := cancelsAllowedForRiskClass(class); got != want {
			t.Errorf("cancelsAllowedForRiskClass(%q) = %t, want %t", class, got, want)
		}
	}
	for _, class := range refused {
		if cancelsAllowedForRiskClass(class) {
			t.Errorf("cancelsAllowedForRiskClass(%q) = true, want false", class)
		}
	}
}

// TestRequirePaperAccount proves the belt-and-braces guard admits IBKR paper
// accounts and refuses anything else with an error naming both the account and
// the attempted operation.
func TestRequirePaperAccount(t *testing.T) {
	t.Parallel()

	// Only the DU9000001 redaction token may carry a full account-id shape in
	// tracked files (see sanitization_test.go); the other samples stay short.
	for _, account := range []string{"DU9000001", "DU12345", "DUP12345"} {
		if err := requirePaperAccount(account, "global cancel"); err != nil {
			t.Errorf("requirePaperAccount(%q) = %v, want nil", account, err)
		}
	}

	for _, account := range []string{"U123456", "DF123456", "", "du-lowercase", "XU123"} {
		err := requirePaperAccount(account, "pre-scenario global cancel")
		if err == nil {
			t.Errorf("requirePaperAccount(%q) = nil, want refusal", account)
			continue
		}
		if account != "" && !strings.Contains(err.Error(), account) {
			t.Errorf("requirePaperAccount(%q) error %q does not name the account", account, err)
		}
		if !strings.Contains(err.Error(), "pre-scenario global cancel") {
			t.Errorf("requirePaperAccount(%q) error %q does not name the operation", account, err)
		}
	}
}

// TestGuardedCancelAllRefusesNonPaperAccountBeforeMutating passes a nil client:
// if the guard did not short-circuit on a non-paper account, calling CancelAll
// on the nil client would panic. Returning the refusal error proves no order
// mutation is attempted on a live account.
func TestGuardedCancelAllRefusesNonPaperAccountBeforeMutating(t *testing.T) {
	t.Parallel()

	err := guardedCancelAll(context.Background(), nil, "U123456", "cleanup global cancel")
	if err == nil {
		t.Fatal("guardedCancelAll on a non-paper account returned nil, want refusal")
	}
	if !strings.Contains(err.Error(), "U123456") {
		t.Errorf("guardedCancelAll error %q does not name the live account", err)
	}
}

// TestVerifyWrapperForScenario cross-checks, for every catalogued scenario, that
// verifyWrapperForScenario accepts the wrapper its RiskClass dictates and
// rejects the other, and that an unknown scenario is refused.
func TestVerifyWrapperForScenario(t *testing.T) {
	t.Parallel()

	for name, md := range scenarioMetadataByName {
		want := wrapperReadOnly
		other := wrapperTrading
		if cancelsAllowedForRiskClass(md.RiskClass) {
			want, other = wrapperTrading, wrapperReadOnly
		}
		if err := verifyWrapperForScenario(name, want); err != nil {
			t.Errorf("verifyWrapperForScenario(%q, correct) = %v, want nil", name, err)
		}
		if err := verifyWrapperForScenario(name, other); err == nil {
			t.Errorf("verifyWrapperForScenario(%q, wrong) = nil, want mismatch error", name)
		}
	}

	if err := verifyWrapperForScenario("scenario_that_does_not_exist", wrapperTrading); err == nil {
		t.Fatal("verifyWrapperForScenario(unknown) = nil, want error")
	}
}

// TestAPIRunFunctionsUseWrapperMatchingRiskClass invokes every api_* run
// function with an already-cancelled context so it cannot reach the network.
// The wrappers verify their kind against the catalog RiskClass before dialing;
// a run function wired to the wrong wrapper returns the mismatch error, while a
// correctly wired one fails fast at dial instead. This is the drift guard tying
// each function's actual wrapper choice to its catalog RiskClass.
func TestAPIRunFunctionsUseWrapperMatchingRiskClass(t *testing.T) {
	// Not parallel: mutates the apiDriver global that carries scenario identity.
	prev := apiDriver
	t.Cleanup(func() { apiDriver = prev })

	for name, sc := range scenarios {
		if sc.runAPI == nil {
			continue
		}
		apiDriver = &apiDriverRecorder{scenario: name}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := sc.runAPI(ctx, "127.0.0.1:0", 1)
		if err != nil && strings.Contains(err.Error(), "trading-wrapper") {
			t.Errorf("scenario %q is wired to the wrong capture wrapper: %v", name, err)
		}
	}
}

// TestScenarioCaptureRoleMatchesCancelPolicy freezes the invariant that the
// capture role and the cancel policy are driven by the same classification: a
// scenario routes to the paper-dev role if and only if it is allowed to cancel.
func TestScenarioCaptureRoleMatchesCancelPolicy(t *testing.T) {
	t.Parallel()

	for name, md := range scenarioMetadataByName {
		wantPaper := cancelsAllowedForRiskClass(md.RiskClass)
		gotPaper := scenarioCaptureRole(md) == captureRolePaperDev
		if wantPaper != gotPaper {
			t.Errorf("scenario %q: cancels=%t but capture role paper-dev=%t", name, wantPaper, gotPaper)
		}
	}
}

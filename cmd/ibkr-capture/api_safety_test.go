package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
)

func TestAPIDriverRecorderRetainsEncodeFailure(t *testing.T) {
	recorder, err := newAPIDriverRecorder(filepath.Join(t.TempDir(), "driver.jsonl"), "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := recorder.file.Close(); err != nil {
		t.Fatal(err)
	}
	recorder.file = nil
	recorder.record("scenario_start", "", nil)
	if err := recorder.Close(); err == nil || !strings.Contains(err.Error(), "encode scenario_start driver event") {
		t.Fatalf("Close() error = %v, want retained encode failure", err)
	}
}

func TestHistoricalDataUnavailableRequiresExactTypedError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		err  error
		op   ibkr.OpKind
		want bool
	}{
		{name: "permissions", err: &ibkr.APIError{Code: 10187, OpKind: ibkr.OpHistoricalTicks, Message: "No market data permissions for NASDAQ STK"}, op: ibkr.OpHistoricalTicks, want: true},
		{name: "subscription required", err: &ibkr.APIError{Code: ibkr.ErrCodeHistoricalDataSubscriptionRequired, OpKind: ibkr.OpHistoricalBars, Message: "Up-to-the-second historical data requires additional subscription for the API."}, op: ibkr.OpHistoricalBars, want: true},
		{name: "stream permissions", err: &ibkr.APIError{Code: 162, OpKind: ibkr.OpHistoricalBarsStream, Message: "Historical Market Data Service error message:No market data permissions for ISLAND STK."}, op: ibkr.OpHistoricalBarsStream, want: true},
		{name: "different IP", err: &ibkr.APIError{Code: 162, OpKind: ibkr.OpHistoricalBars, Message: "Trading TWS session is connected from a different IP address"}, op: ibkr.OpHistoricalBars, want: true},
		{name: "wrong operation", err: &ibkr.APIError{Code: 10187, OpKind: ibkr.OpHistoricalBars, Message: "No market data permissions"}, op: ibkr.OpHistoricalTicks},
		{name: "wrong code", err: &ibkr.APIError{Code: 200, OpKind: ibkr.OpHistoricalTicks, Message: "No market data permissions"}, op: ibkr.OpHistoricalTicks},
		{name: "wrong message", err: &ibkr.APIError{Code: 10187, OpKind: ibkr.OpHistoricalTicks, Message: "unrelated"}, op: ibkr.OpHistoricalTicks},
		{name: "untyped", err: errors.New("No market data permissions"), op: ibkr.OpHistoricalTicks},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if got := isHistoricalDataUnavailable(tt.err, tt.op); got != tt.want {
				t.Fatalf("isHistoricalDataUnavailable() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestOddLotEntitlementRefusalRequiresExactTypedError(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		err  *ibkr.APIError
		want bool
	}{
		{name: "exact subscription required", err: &ibkr.APIError{Code: ibkr.ErrCodeAdditionalSubscriptionRequired, OpKind: ibkr.OpQuotes, Message: "Requested market data requires additional subscription for API. See link"}, want: true},
		{name: "exact delayed fallback", err: &ibkr.APIError{Code: 2186, OpKind: ibkr.OpQuotes, Message: "Warning: Requested real-time market data requires additional subscription for API. You elected to receive delayed market data instead. To subscribe, see link"}, want: true},
		{name: "wrong operation", err: &ibkr.APIError{Code: ibkr.ErrCodeAdditionalSubscriptionRequired, OpKind: ibkr.OpHistoricalBars, Message: "Requested market data requires additional subscription for API."}},
		{name: "wrong code", err: &ibkr.APIError{Code: ibkr.ErrCodeDelayedMarketDataDisplayed, OpKind: ibkr.OpQuotes, Message: "Warning: Requested real-time market data requires additional subscription for API. You elected to receive delayed market data instead."}},
		{name: "wrong message", err: &ibkr.APIError{Code: ibkr.ErrCodeAdditionalSubscriptionRequired, OpKind: ibkr.OpQuotes, Message: "unrelated"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := isExactOddLotEntitlementRefusal(tt.err); got != tt.want {
				t.Fatalf("isExactOddLotEntitlementRefusal() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestCaptureWSHResultRequiresValidDataOrExactBlocker(t *testing.T) {
	t.Parallel()

	if err := captureWSHResult("metadata", ibkr.OpWSHMetaData, ibkr.JSONDocument(`{"sources":[]}`), nil); err != nil {
		t.Fatalf("captureWSHResult(valid JSON) = %v, want nil", err)
	}
	for _, data := range []ibkr.JSONDocument{nil, ibkr.JSONDocument(`not-json`)} {
		if err := captureWSHResult("metadata", ibkr.OpWSHMetaData, data, nil); err == nil {
			t.Fatalf("captureWSHResult(%q) = nil, want invalid-JSON error", data)
		}
	}

	exact := &ibkr.APIError{Code: 10276, OpKind: ibkr.OpWSHMetaData, Message: "News feed is not allowed."}
	if err := captureWSHResult("metadata", ibkr.OpWSHMetaData, nil, exact); err != nil {
		t.Fatalf("captureWSHResult(exact blocker) = %v, want nil", err)
	}
	for _, err := range []error{
		&ibkr.APIError{Code: 10276, OpKind: ibkr.OpWSHEventData, Message: "News feed is not allowed."},
		&ibkr.APIError{Code: 10089, OpKind: ibkr.OpWSHMetaData, Message: "News feed is not allowed."},
		&ibkr.APIError{Code: 10276, OpKind: ibkr.OpWSHMetaData, Message: "unrelated"},
		errors.New("News feed is not allowed"),
	} {
		if got := captureWSHResult("metadata", ibkr.OpWSHMetaData, nil, err); !errors.Is(got, err) {
			t.Fatalf("captureWSHResult(%v) = %v, want original unexpected error", err, got)
		}
	}
}

// TestCancelsAllowedForRiskClass freezes the single source of truth for which
// risk classes may mutate order state. Only the four paper-trading classes may
// use the globally gated cancel fallback; every other class (which may capture
// against the real-money readonly-live role) must not.
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
	t.Setenv("IBKR_PAPER_ACCOUNT", "DU9000001")
	if err := requirePaperAccount("DU9000001", "place order"); err != nil {
		t.Fatalf("requirePaperAccount(allowlisted) = %v, want nil", err)
	}
	for _, account := range []string{"DU12345", "DUP12345"} {
		if err := requirePaperAccount(account, "place order"); err == nil {
			t.Errorf("requirePaperAccount(%q) = nil, want exact-account refusal", account)
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

func TestRequirePaperAccountsRefusesMixedManagedSession(t *testing.T) {
	t.Setenv("IBKR_PAPER_ACCOUNT", "DU9000001")

	err := requirePaperAccounts([]string{"DU9000001", "U123456"}, "global cancel")
	if err == nil {
		t.Fatal("requirePaperAccounts(mixed) = nil, want refusal")
	}
	if !strings.Contains(err.Error(), "U123456") {
		t.Fatalf("mixed-account refusal %q does not name the live account", err)
	}
}

// TestGuardedCancelAllRefusesNonPaperAccountBeforeMutating passes a nil client:
// if the guard did not short-circuit on a non-paper account, calling CancelAll
// on the nil client would panic. Returning the refusal error proves no order
// mutation is attempted on a live account.
func TestGuardedCancelAllRefusesNonPaperAccountBeforeMutating(t *testing.T) {
	t.Setenv("IBKR_PAPER_ACCOUNT", "DU9000001")

	err := guardedCancelAll(context.Background(), nil, "U123456", "cleanup global cancel")
	if err == nil {
		t.Fatal("guardedCancelAll on a non-paper account returned nil, want refusal")
	}
	if !strings.Contains(err.Error(), "U123456") {
		t.Errorf("guardedCancelAll error %q does not name the live account", err)
	}
}

// TestCancelOrderRefusesNonPaperAccountBeforeMutating passes a nil handle: if
// the session guard did not short-circuit, reading or cancelling it would
// panic.
func TestCancelOrderRefusesNonPaperAccountBeforeMutating(t *testing.T) {
	t.Setenv("IBKR_PAPER_ACCOUNT", "DU9000001")

	cancelOrder(context.Background(), nil, "U123456", nil, "targeted cleanup")
}

func TestGuardedCancelAllRequiresPurposeSpecificGate(t *testing.T) {
	t.Setenv("IBKR_PAPER_ACCOUNT", "DU9000001")
	t.Setenv("IBKR_CAPTURE_GLOBAL_CANCEL", "0")

	err := guardedCancelAll(context.Background(), nil, "DU9000001", "global cancel proof")
	if err == nil || !strings.Contains(err.Error(), "IBKR_CAPTURE_GLOBAL_CANCEL") {
		t.Fatalf("guardedCancelAll() error = %v, want purpose-specific gate refusal", err)
	}
}

func TestRequireGlobalCancelGateFailsClosed(t *testing.T) {
	for _, value := range []string{"", "0", "false", "yes", "tru"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("IBKR_CAPTURE_GLOBAL_CANCEL", value)
			if err := requireGlobalCancelGate("paper campaign admission"); err == nil {
				t.Fatal("requireGlobalCancelGate() = nil, want refusal")
			}
		})
	}
	t.Run("enabled", func(t *testing.T) {
		t.Setenv("IBKR_CAPTURE_GLOBAL_CANCEL", "1")
		if err := requireGlobalCancelGate("paper campaign admission"); err != nil {
			t.Fatalf("requireGlobalCancelGate() = %v, want nil", err)
		}
	})
}

func TestEnvFlagFailsClosed(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: "1", want: true},
		{value: "true", want: true},
		{value: "TRUE", want: true},
		{value: "0", want: false},
		{value: "false", want: false},
		{value: "", want: false},
		{value: "tru", want: false},
		{value: "yes", want: false},
	} {
		t.Run(test.value, func(t *testing.T) {
			t.Setenv("IBKR_CAPTURE_TEST_FLAG", test.value)
			if got := envFlag("IBKR_CAPTURE_TEST_FLAG"); got != test.want {
				t.Fatalf("envFlag(%q) = %t, want %t", test.value, got, test.want)
			}
		})
	}
}

func TestVerifyNewExecutionFees(t *testing.T) {
	t.Parallel()

	baseline := ibkr.ExecutionSnapshot{Executions: []ibkr.Execution{{ExecID: "old"}}}
	complete := ibkr.ExecutionSnapshot{
		Executions: []ibkr.Execution{{ExecID: "old"}, {ExecID: "new"}},
		CommissionAndFees: []ibkr.CommissionAndFeesReport{
			{ExecID: "new"},
		},
	}
	if err := verifyNewExecutionFees(baseline, complete); err != nil {
		t.Fatalf("verifyNewExecutionFees(complete) = %v", err)
	}
	complete.CommissionAndFees = nil
	if err := verifyNewExecutionFees(baseline, complete); err == nil {
		t.Fatal("verifyNewExecutionFees(missing fee) = nil, want error")
	}
}

func TestGuardedCancelOrderRefusesNonPaperAccountBeforeMutating(t *testing.T) {
	t.Parallel()

	err := guardedCancelOrder(context.Background(), nil, "U123456", 1, 99, "direct cancel")
	if err == nil {
		t.Fatal("guardedCancelOrder on a non-paper account returned nil, want refusal")
	}
	if !strings.Contains(err.Error(), "direct cancel") || !strings.Contains(err.Error(), "U123456") {
		t.Fatalf("guardedCancelOrder error %q does not name operation and account", err)
	}
}

func TestOrderMutationHelpersRefuseNonPaperAccountBeforeMutating(t *testing.T) {
	t.Parallel()

	order := ibkr.Order{Account: "U123456"}
	if _, err := placeAPIOrder(context.Background(), nil, "unsafe place", ibkr.Contract{}, order); err == nil {
		t.Fatal("placeAPIOrder on a non-paper account returned nil, want refusal")
	}
	if err := modifyAPIOrder(context.Background(), nil, nil, "unsafe replace", order); err == nil {
		t.Fatal("modifyAPIOrder on a non-paper account returned nil, want refusal")
	}
}

// TestVerifyWrapperForScenario cross-checks, for every catalogued scenario, that
// verifyWrapperForScenario accepts the wrapper its RiskClass dictates and
// rejects the other, and that an unknown scenario is refused.
func TestVerifyWrapperForScenario(t *testing.T) {
	t.Parallel()

	for name, scenario := range scenarios {
		md := scenario.metadata
		want := wrapperReadOnly
		other := wrapperTrading
		if cancelsAllowedForRiskClass(md.RiskClass) {
			want, other = wrapperTrading, wrapperReadOnly
		}
		if err := verifyWrapperForScenario(name, scenario, want); err != nil {
			t.Errorf("verifyWrapperForScenario(%q, correct) = %v, want nil", name, err)
		}
		if err := verifyWrapperForScenario(name, scenario, other); err == nil {
			t.Errorf("verifyWrapperForScenario(%q, wrong) = nil, want mismatch error", name)
		}
	}

	if err := verifyWrapperForScenario("scenario_that_does_not_exist", nil, wrapperTrading); err == nil {
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
		if sc.run == nil {
			continue
		}
		apiDriver = &apiDriverRecorder{scenario: name, definition: sc}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := sc.run(ctx, "127.0.0.1:0", 1)
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

	for name, scenario := range scenarios {
		md := scenario.metadata
		wantPaper := cancelsAllowedForRiskClass(md.RiskClass)
		role, err := scenarioCaptureRole(md)
		if err != nil {
			t.Fatalf("scenarioCaptureRole(%q) error = %v", md.RiskClass, err)
		}
		gotPaper := role == captureRolePaperDev
		if wantPaper != gotPaper {
			t.Errorf("scenario %q: cancels=%t but capture role paper-dev=%t", name, wantPaper, gotPaper)
		}
	}
}

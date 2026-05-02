package ibkr

import (
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
)

func TestFromSDKOpenOrderRejectsMalformedNonEmptyNumericField(t *testing.T) {
	t.Parallel()

	_, err := fromSDKOpenOrder(sdkadapter.OpenOrder{
		OrderID:   1,
		Account:   "DU12345",
		Contract:  sdkadapter.Contract{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"},
		Action:    "BUY",
		OrderType: "LMT",
		Quantity:  "1",
		ClientID:  "not-an-int",
	})
	if err == nil {
		t.Fatal("fromSDKOpenOrder() error = nil, want malformed client id rejection")
	}
}

func TestFromSDKOrderStatusRejectsMalformedNonEmptyDecimalField(t *testing.T) {
	t.Parallel()

	_, err := fromSDKOrderStatus(sdkadapter.OrderStatus{
		OrderID:      1,
		Status:       "Submitted",
		Filled:       "abc",
		Remaining:    "1",
		AvgFillPrice: "0",
	})
	if err == nil {
		t.Fatal("fromSDKOrderStatus() error = nil, want malformed filled rejection")
	}
}

// TestFromSDKExecutionAcceptsNativeGatewayTime freezes the live Gateway
// execution timestamp shape observed in ExecutionDetail msg_id=11:
// "YYYYMMDD HH:MM:SS US/Eastern", not RFC3339.
func TestFromSDKExecutionAcceptsNativeGatewayTime(t *testing.T) {
	t.Parallel()

	update, err := fromSDKExecution(sdkadapter.ExecutionDetail{
		OrderID: 42,
		ExecID:  "0000e0d5.69dd4c37.01.01",
		Account: "DU12345",
		Symbol:  "AAPL",
		Side:    "BOT",
		Shares:  "1",
		Price:   "257.69",
		Time:    "20260413 13:35:50 US/Eastern",
	})
	if err != nil {
		t.Fatalf("fromSDKExecution() error = %v, want nil", err)
	}
	if update.Execution == nil {
		t.Fatal("Execution = nil, want decoded execution")
	}
	want := time.Date(2026, 4, 13, 17, 35, 50, 0, time.UTC)
	if !update.Execution.Time.Equal(want) {
		t.Fatalf("Execution.Time = %s, want %s", update.Execution.Time.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func TestFromSDKExecutionKeepsRFC3339TranscriptCompatibility(t *testing.T) {
	t.Parallel()

	update, err := fromSDKExecution(sdkadapter.ExecutionDetail{
		OrderID: 42,
		ExecID:  "exec-1",
		Account: "DU12345",
		Symbol:  "AAPL",
		Side:    "BOT",
		Shares:  "10",
		Price:   "189.11",
		Time:    "2026-04-05T12:01:00Z",
	})
	if err != nil {
		t.Fatalf("fromSDKExecution() error = %v, want nil", err)
	}
	want := time.Date(2026, 4, 5, 12, 1, 0, 0, time.UTC)
	if !update.Execution.Time.Equal(want) {
		t.Fatalf("Execution.Time = %s, want %s", update.Execution.Time.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func TestFromSDKExecutionRejectsMalformedTime(t *testing.T) {
	t.Parallel()

	_, err := fromSDKExecution(sdkadapter.ExecutionDetail{
		OrderID: 42,
		ExecID:  "exec-bad-time",
		Account: "DU12345",
		Symbol:  "AAPL",
		Side:    "BOT",
		Shares:  "1",
		Price:   "150",
		Time:    "not-a-timestamp",
	})
	if err == nil {
		t.Fatal("fromSDKExecution() error = nil, want malformed execution time rejection")
	}
}

func TestParseOptionalDecimalAllowsBlank(t *testing.T) {
	t.Parallel()

	value, err := parseOptionalDecimal("", "test field")
	if err != nil {
		t.Fatalf("parseOptionalDecimal() error = %v", err)
	}
	if !value.IsZero() {
		t.Fatalf("parseOptionalDecimal() = %s, want zero", value.String())
	}
}

// TestParseOptionalDecimalTreatsMaxDoubleSentinelAsAbsent freezes the rule
// that the literal Double.MAX_VALUE string TWS emits for unset optional
// doubles is treated the same as an empty string. The canonical form is
// preserved by a legacy live open-order capture.
func TestParseOptionalDecimalTreatsMaxDoubleSentinelAsAbsent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{"canonical_uppercase", "1.7976931348623157E308"},
		{"lowercase_exponent", "1.7976931348623157e308"},
		{"surrounding_whitespace", "  1.7976931348623157E308\t"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			value, err := parseOptionalDecimal(tt.raw, "test field")
			if err != nil {
				t.Fatalf("parseOptionalDecimal(%q) error = %v, want nil", tt.raw, err)
			}
			if !value.IsZero() {
				t.Fatalf("parseOptionalDecimal(%q) = %s, want zero (sentinel should decode as absent)", tt.raw, value.String())
			}
		})
	}
}

// TestFromSDKOpenOrderAcceptsSentinelCommissionFields is the end-to-end
// regression freeze for the reported P1: live TWS open-order traffic encodes
// unset commission/min/max commission as the MAX_DOUBLE sentinel, and that
// must not tear down the open-order decode path.
func TestFromSDKOpenOrderAcceptsSentinelCommissionFields(t *testing.T) {
	t.Parallel()

	const sentinel = "1.7976931348623157E308"

	order, err := fromSDKOpenOrder(sdkadapter.OpenOrder{
		OrderID:       1,
		Account:       "DU12345",
		Contract:      sdkadapter.Contract{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"},
		Action:        "BUY",
		OrderType:     "LMT",
		Quantity:      "1",
		Filled:        "0",
		Remaining:     "1",
		LmtPrice:      "150.00",
		AuxPrice:      "0",
		Commission:    sentinel,
		MinCommission: sentinel,
		MaxCommission: sentinel,
	})
	if err != nil {
		t.Fatalf("fromSDKOpenOrder() error = %v, want nil", err)
	}
	if !order.Commission.IsZero() {
		t.Errorf("Commission = %s, want zero (sentinel should decode as absent)", order.Commission.String())
	}
	if !order.MinCommission.IsZero() {
		t.Errorf("MinCommission = %s, want zero", order.MinCommission.String())
	}
	if !order.MaxCommission.IsZero() {
		t.Errorf("MaxCommission = %s, want zero", order.MaxCommission.String())
	}
}

// TestFromSDKCommissionAcceptsSentinelFields freezes the receive-path
// contract for CommissionReport: live TWS emits the MAX_DOUBLE sentinel for
// Commission and RealizedPNL when the server has not yet computed those
// values, and the Go client must decode both forms to a zero decimal instead
// of tearing down the executions subscription or silently dropping the report.
func TestFromSDKCommissionAcceptsSentinelFields(t *testing.T) {
	t.Parallel()

	const sentinel = "1.7976931348623157E308"

	tests := []struct {
		name       string
		commission string
		realized   string
	}{
		{"sentinel_commission", sentinel, "0"},
		{"sentinel_realized", "1.25", sentinel},
		{"both_sentinel", sentinel, sentinel},
		{"both_empty", "", ""},
		{"mixed_empty_sentinel", "", sentinel},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			report, err := fromSDKCommission(sdkadapter.CommissionReport{
				ExecID:      "exec-1",
				Commission:  tt.commission,
				Currency:    "USD",
				RealizedPNL: tt.realized,
			})
			if err != nil {
				t.Fatalf("fromSDKCommission() error = %v, want nil", err)
			}
			if report.ExecID != "exec-1" {
				t.Errorf("ExecID = %q, want %q", report.ExecID, "exec-1")
			}
			if report.Currency != "USD" {
				t.Errorf("Currency = %q, want %q", report.Currency, "USD")
			}
		})
	}
}

// TestFromSDKCommissionPreservesRealValues confirms that the sentinel fix
// did not alter decoding of real commission values.
func TestFromSDKCommissionPreservesRealValues(t *testing.T) {
	t.Parallel()

	report, err := fromSDKCommission(sdkadapter.CommissionReport{
		ExecID:      "exec-2",
		Commission:  "1.25",
		Currency:    "USD",
		RealizedPNL: "-50.00",
	})
	if err != nil {
		t.Fatalf("fromSDKCommission() error = %v, want nil", err)
	}
	if got := report.Commission.String(); got != "1.25" {
		t.Errorf("Commission = %s, want 1.25", got)
	}
	if got := report.RealizedPNL.String(); got != "-50" {
		t.Errorf("RealizedPNL = %s, want -50", got)
	}
}

// TestFromSDKCommissionRejectsMalformedField freezes the rule that a
// genuinely malformed decimal (not a sentinel, not empty) still produces an
// error so the engine's log-and-drop path has something to report.
func TestFromSDKCommissionRejectsMalformedField(t *testing.T) {
	t.Parallel()

	_, err := fromSDKCommission(sdkadapter.CommissionReport{
		ExecID:      "exec-3",
		Commission:  "not-a-decimal",
		Currency:    "USD",
		RealizedPNL: "0",
	})
	if err == nil {
		t.Fatal("fromSDKCommission() error = nil, want malformed commission rejection")
	}
}

package ibkr

import (
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/shopspring/decimal"
)

func TestFromCodecOpenOrderRejectsMalformedNonEmptyNumericField(t *testing.T) {
	t.Parallel()

	_, err := fromCodecOpenOrder(codec.OpenOrder{
		OrderID:   1,
		Account:   "DU12345",
		Contract:  codec.Contract{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"},
		Action:    "BUY",
		OrderType: "LMT",
		Quantity:  "1",
		ClientID:  "not-an-int",
	})
	if err == nil {
		t.Fatal("fromCodecOpenOrder() error = nil, want malformed client id rejection")
	}
}

func TestContractConversionPreservesCanonicalPresence(t *testing.T) {
	t.Parallel()

	// This conversion-law composite keeps provenance explicit rather than
	// claiming one accepted Gateway request: BAG conID 28812380 came from the
	// June paper order, the two legs from the exact-200 BAG quote, ISIN and
	// IncludeExpired from the read-only request matrix, and the delta-neutral
	// values from the exact-200 OPT request rejected with code 320. Explicit
	// zero presence for strike and exempt code is the official schema law under
	// test, not positive delta-neutral or exempt-code live evidence.
	explicitExempt := 0
	contract := Contract{
		ConID:          28812380,
		SecType:        SecTypeCombo,
		Strike:         new(decimal.Zero),
		IncludeExpired: true,
		SecurityID:     SecurityID{Type: SecurityIDISIN, Value: "US0378331005"},
		ComboLegs: []ComboLeg{
			{ConID: 887307502, Ratio: 1, Action: ActionBuy, Exchange: "SMART"},
			{ConID: 887307536, Ratio: 1, Action: ActionSell, Exchange: "SMART", ExemptCode: &explicitExempt},
		},
		DeltaNeutral: &DeltaNeutralContract{
			ConID: 265598,
			Delta: decimal.RequireFromString("0.5"),
			Price: decimal.RequireFromString("314.5"),
		},
	}

	wire := toCodecContract(contract)
	if wire.Strike != "0" || wire.SecurityIDType != "ISIN" || wire.SecurityID != "US0378331005" ||
		wire.ComboLegs[0].ExemptCode != "-1" || wire.ComboLegs[1].ExemptCode != "0" {
		t.Fatalf("wire contract presence = %+v", wire)
	}
	got, err := fromCodecContract(wire)
	if err != nil {
		t.Fatalf("fromCodecContract() error = %v", err)
	}
	if got.Strike == nil || !got.Strike.IsZero() || got.DeltaNeutral == nil ||
		got.DeltaNeutral.ConID != 265598 || !got.DeltaNeutral.Delta.Equal(decimal.RequireFromString("0.5")) ||
		got.ComboLegs[0].ExemptCode != nil || got.ComboLegs[1].ExemptCode == nil || *got.ComboLegs[1].ExemptCode != 0 {
		t.Fatalf("round-tripped contract presence = %+v", got)
	}
	if empty := toCodecContract(Contract{}); empty.Strike != "" {
		t.Fatalf("nil strike encoded as %q, want absent", empty.Strike)
	}
}

func TestFromCodecContractRejectsMalformedCanonicalNumerics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		contract codec.Contract
	}{
		{name: "strike", contract: codec.Contract{Strike: "not-a-decimal"}},
		{name: "combo open close", contract: codec.Contract{ComboLegs: []codec.ComboLeg{{OpenClose: "many"}}}},
		{name: "combo open close range", contract: codec.Contract{ComboLegs: []codec.ComboLeg{{OpenClose: "4"}}}},
		{name: "combo short-sale slot", contract: codec.Contract{ComboLegs: []codec.ComboLeg{{ShortSaleSlot: "broker"}}}},
		{name: "combo exempt code", contract: codec.Contract{ComboLegs: []codec.ComboLeg{{ExemptCode: "exempt"}}}},
		{name: "combo negative exempt code", contract: codec.Contract{ComboLegs: []codec.ComboLeg{{ExemptCode: "-2"}}}},
		{name: "delta-neutral delta", contract: codec.Contract{DeltaNeutral: &codec.DeltaNeutralContract{Delta: "half", Price: "1"}}},
		{name: "delta-neutral price", contract: codec.Contract{DeltaNeutral: &codec.DeltaNeutralContract{Delta: "0.5", Price: "market"}}},
		{name: "missing delta-neutral delta", contract: codec.Contract{DeltaNeutral: &codec.DeltaNeutralContract{Price: "1"}}},
		{name: "missing delta-neutral price", contract: codec.Contract{DeltaNeutral: &codec.DeltaNeutralContract{Delta: "0.5"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := fromCodecContract(tc.contract); err == nil {
				t.Fatalf("fromCodecContract(%+v) error = nil", tc.contract)
			}
		})
	}
}

func TestContractConversionAcceptsProtobufDeltaNeutralDefaults(t *testing.T) {
	t.Parallel()

	// The protobuf decoder materializes omitted optional doubles as canonical
	// zero strings before the strict public conversion boundary.
	got, err := fromCodecContract(codec.Contract{DeltaNeutral: &codec.DeltaNeutralContract{
		ConID: 265598,
		Delta: "0",
		Price: "0",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got.DeltaNeutral == nil || got.DeltaNeutral.ConID != 265598 ||
		!got.DeltaNeutral.Delta.IsZero() || !got.DeltaNeutral.Price.IsZero() {
		t.Fatalf("converted delta-neutral defaults = %+v", got.DeltaNeutral)
	}
}

func TestFromCodecOpenOrderRejectsMalformedComboLegPrice(t *testing.T) {
	t.Parallel()

	_, err := fromCodecOpenOrder(codec.OpenOrder{OrderID: 1, Quantity: "1", OrderComboLegPrices: []string{"market"}})
	if err == nil {
		t.Fatal("fromCodecOpenOrder() accepted malformed combo leg price")
	}
}

func TestFromCodecContractDetailsProjectsLiveFundMetadata(t *testing.T) {
	t.Parallel()

	// VTSAX from captures/20260415T150322Z-api_security_type_probe_matrix,
	// server_version=200, events SHA-256 prefix 9be83e57ed176a17.
	detail, err := fromCodecContractDetails(codec.ContractDetails{
		Contract: codec.Contract{
			ConID: 48013650, Symbol: "VTSAX", SecType: "FUND", Exchange: "FUNDSERV",
			Currency: "USD", LocalSymbol: "922908728", TradingClass: "922908728",
		},
		MarketName:             "VTSAX",
		MinTick:                "0.01",
		PriceMagnifier:         1,
		OrderTypes:             "AD,ALERT,ALLOC,BASKET,DAY,DEACT,DEACTDIS,FUNDSWAP,MKT,NONALGO,WHATIF",
		ValidExchanges:         "FUNDSERV",
		LongName:               "Vanguard Total Stock Market Index Fund A (Vanguard)",
		TimeZoneID:             "US/Eastern",
		SecurityIDs:            []codec.TagValue{{Tag: "ISIN", Value: "US9229087286"}},
		AggGroup:               2147483647,
		MarketRuleIDs:          "2963",
		MinSize:                "0.001",
		SizeIncrement:          "0.001",
		SuggestedSizeIncrement: "1",
		Fund: &codec.FundDetails{
			Name: "Vanguard Total Stock Market Index Fund A", Family: "Vanguard",
			ManagementFee: "0.04", MinimumInitialPurchase: "3000",
			MinimumSubsequentPurchase: "1", BlueSkyStates: "All",
		},
	})
	if err != nil {
		t.Fatalf("fromCodecContractDetails() error = %v", err)
	}
	if detail.AggGroup != nil {
		t.Errorf("AggGroup = %v, want nil for live max-int sentinel", detail.AggGroup)
	}
	if len(detail.ValidExchanges) != 1 || detail.ValidExchanges[0] != (ContractExchange{Exchange: "FUNDSERV", MarketRuleID: 2963}) {
		t.Errorf("ValidExchanges = %#v", detail.ValidExchanges)
	}
	if len(detail.OrderTypes) != 11 || detail.OrderTypes[7] != "FUNDSWAP" {
		t.Errorf("OrderTypes = %#v", detail.OrderTypes)
	}
	if detail.MinSize == nil || detail.MinSize.String() != "0.001" || detail.SizeIncrement == nil || detail.SuggestedSizeIncrement == nil {
		t.Errorf("size rules = %v/%v/%v", detail.MinSize, detail.SizeIncrement, detail.SuggestedSizeIncrement)
	}
	if len(detail.SecurityIDs) != 1 || detail.SecurityIDs[0] != (TagValue{Tag: "ISIN", Value: "US9229087286"}) {
		t.Errorf("SecurityIDs = %#v", detail.SecurityIDs)
	}
	if detail.Fund == nil || detail.Fund.Family != "Vanguard" || detail.Fund.ManagementFee != "0.04" || detail.Fund.MinimumInitialPurchase != "3000" {
		t.Errorf("Fund = %#v", detail.Fund)
	}
}

func TestFromCodecOrderStatusRejectsMalformedNonEmptyDecimalField(t *testing.T) {
	t.Parallel()

	_, err := fromCodecOrderStatus(codec.OrderStatus{
		OrderID:      1,
		Status:       "Submitted",
		Filled:       "abc",
		Remaining:    "1",
		AvgFillPrice: "0",
	})
	if err == nil {
		t.Fatal("fromCodecOrderStatus() error = nil, want malformed filled rejection")
	}
}

func TestFromCodecCompletedOrderProjectsLiveTrailLimitFields(t *testing.T) {
	t.Parallel()

	// Live-derived values from
	// captures/20260415T162637Z-api_completed_orders_variants_aapl,
	// server_version=200, events SHA-256 prefix 6415ad97b4c9f33e.
	order, err := fromCodecCompletedOrder(codec.CompletedOrder{
		Contract:        codec.Contract{ConID: 265598, Symbol: "AAPL", SecType: "STK", Strike: "0", Right: "?", Exchange: "SMART", Currency: "USD", LocalSymbol: "AAPL", TradingClass: "NMS"},
		Action:          "BUY",
		Quantity:        "1",
		OrderType:       "TRAIL LIMIT",
		LmtPrice:        "2000.05",
		AuxPrice:        "1.0",
		TIF:             "DAY",
		PermID:          "1426085924",
		TrailStopPrice:  "2000.0",
		Status:          "Cancelled",
		StopPrice:       "2000.0",
		LmtPriceOffset:  "0.05",
		Filled:          "0",
		ParentPermID:    "9223372036854775807",
		CompletedTime:   "20260415 11:00:11 US/Eastern",
		CompletedStatus: "Cancelled by Trader",
		Shareholder:     "Not an insider or substantial shareholder",
		Submitter:       "paper-user",
	})
	if err != nil {
		t.Fatalf("fromCodecCompletedOrder() error = %v", err)
	}
	if order.Contract.Right != "" || order.Order.OrderType != OrderTypeTrailingLimit {
		t.Fatalf("contract/order = %+v/%+v", order.Contract, order.Order)
	}
	if order.Order.PermID == nil || *order.Order.PermID != 1426085924 {
		t.Fatalf("permanent id = %v, want 1426085924", order.Order.PermID)
	}
	if order.Order.OrderID != nil || order.Order.ClientID != nil || order.Order.ParentID != nil {
		t.Fatalf("classic completed-order identities = %v/%v/%v, want absent", order.Order.OrderID, order.Order.ClientID, order.Order.ParentID)
	}
	prices := order.Order.Prices
	if prices.LmtPrice == nil || prices.LmtPrice.String() != "2000.05" ||
		prices.AuxPrice == nil || prices.AuxPrice.String() != "1" ||
		prices.TrailStopPrice == nil || prices.TrailStopPrice.String() != "2000" ||
		prices.StopPrice == nil || prices.StopPrice.String() != "2000" ||
		prices.LmtPriceOffset == nil || prices.LmtPriceOffset.String() != "0.05" {
		t.Fatalf("prices = %+v", prices)
	}
	if order.Completion.ParentPermID != nil {
		t.Fatalf("parent permanent id = %v, want unset sentinel normalized to nil", order.Completion.ParentPermID)
	}
	if order.Completion.Time != "20260415 11:00:11 US/Eastern" ||
		order.Completion.StatusText != "Cancelled by Trader" ||
		order.Order.Compliance.Submitter != "paper-user" {
		t.Fatalf("completion/compliance = %+v/%+v", order.Completion, order.Order.Compliance)
	}
}

func TestFromCodecCompletedOrderRejectsMalformedTypedField(t *testing.T) {
	t.Parallel()

	_, err := fromCodecCompletedOrder(codec.CompletedOrder{
		Quantity:    "1",
		Filled:      "0",
		DisplaySize: "not-an-int",
	})
	if err == nil {
		t.Fatal("fromCodecCompletedOrder() error = nil, want malformed display size rejection")
	}
}

// TestFromCodecExecutionAcceptsNativeGatewayTime freezes the live Gateway
// execution timestamp shape observed in ExecutionDetail msg_id=11:
// "YYYYMMDD HH:MM:SS US/Eastern", not RFC3339.
func TestFromCodecExecutionAcceptsNativeGatewayTime(t *testing.T) {
	t.Parallel()

	execution, err := fromCodecExecution(codec.ExecutionDetail{
		OrderID:  42,
		Contract: codec.Contract{Symbol: "AAPL"},
		ExecID:   "0000e0d5.69dd4c37.01.01",
		Account:  "DU12345",
		Side:     "BOT",
		Shares:   "1",
		Price:    "257.69",
		Time:     "20260413 13:35:50 US/Eastern",
	})
	if err != nil {
		t.Fatalf("fromCodecExecution() error = %v, want nil", err)
	}
	want := time.Date(2026, 4, 13, 17, 35, 50, 0, time.UTC)
	if !execution.Time.Equal(want) {
		t.Fatalf("Execution.Time = %s, want %s", execution.Time.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func TestFromCodecExecutionProjectsCompleteClassicResult(t *testing.T) {
	t.Parallel()

	exec, err := fromCodecExecution(codec.ExecutionDetail{
		OrderID: 1,
		Contract: codec.Contract{
			ConID: 265598, Symbol: "AAPL", SecType: "STK", Exchange: "IEX",
			Currency: "USD", LocalSymbol: "AAPL", TradingClass: "NMS",
		},
		ExecID: "sanitized-native-exec-001", Time: "20260413 15:27:04 US/Eastern",
		Account: "DU9000001", Exchange: "IEX", Side: "BOT", Shares: "1", Price: "257.95",
		PermID: "900001", ClientID: "94", Liquidation: "0", CumulativeQuantity: "1",
		AveragePrice: "257.95", OrderRef: "capture-ref", EconomicValueRule: "",
		EconomicValueMultiplier: "", ModelCode: "", LastLiquidity: "2",
		PendingPriceRevision: "0", Submitter: "",
	})
	if err != nil {
		t.Fatalf("fromCodecExecution() error = %v", err)
	}
	if exec.Contract.ConID != 265598 || exec.Contract.Symbol != "AAPL" ||
		exec.Contract.SecType != SecTypeStock || exec.Exchange != "IEX" {
		t.Errorf("contract/exchange = %+v/%q", exec.Contract, exec.Exchange)
	}
	if exec.Side != ExecutionSideBought || exec.PermID != 900001 || exec.ClientID != 94 || exec.Liquidation != 0 {
		t.Errorf("side/identity/liquidation = %q/%d/%d/%d", exec.Side, exec.PermID, exec.ClientID, exec.Liquidation)
	}
	if !exec.Shares.Equal(decimal.RequireFromString("1")) ||
		!exec.Price.Equal(decimal.RequireFromString("257.95")) ||
		!exec.CumulativeQuantity.Equal(decimal.RequireFromString("1")) ||
		!exec.AveragePrice.Equal(decimal.RequireFromString("257.95")) {
		t.Errorf("execution quantities/prices = %+v", exec)
	}
	if exec.OrderRef != "capture-ref" || exec.EconomicValueMultiplier != nil ||
		exec.Liquidity != ExecutionLiquidityRemoved || exec.PriceRevisionPending || exec.Submitter != "" {
		t.Errorf("execution tail = %+v", exec)
	}
}

func TestFromCodecExecutionKeepsRFC3339TranscriptCompatibility(t *testing.T) {
	t.Parallel()

	execution, err := fromCodecExecution(codec.ExecutionDetail{
		OrderID:  42,
		Contract: codec.Contract{Symbol: "AAPL"},
		ExecID:   "exec-1",
		Account:  "DU12345",
		Side:     "BOT",
		Shares:   "10",
		Price:    "189.11",
		Time:     "2026-04-05T12:01:00Z",
	})
	if err != nil {
		t.Fatalf("fromCodecExecution() error = %v, want nil", err)
	}
	want := time.Date(2026, 4, 5, 12, 1, 0, 0, time.UTC)
	if !execution.Time.Equal(want) {
		t.Fatalf("Execution.Time = %s, want %s", execution.Time.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func TestFromCodecExecutionOptionExerciseType(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		raw  string
		want OptionExerciseType
	}{
		{"gateway none sentinel", "-1", OptionExerciseTypeNone},
		{"future value is preserved", "444", OptionExerciseType(444)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			execution, err := fromCodecExecution(codec.ExecutionDetail{
				Shares: "1", Price: "315.48", Time: "20260709 18:55:05 US/Eastern",
				OptExerciseOrLapseType: tc.raw,
			})
			if err != nil {
				t.Fatalf("fromCodecExecution() error = %v", err)
			}
			if got := execution.OptionExerciseType; got != tc.want {
				t.Fatalf("OptionExerciseType = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestFromCodecExecutionRejectsMalformedTime(t *testing.T) {
	t.Parallel()

	_, err := fromCodecExecution(codec.ExecutionDetail{
		OrderID:  42,
		Contract: codec.Contract{Symbol: "AAPL"},
		ExecID:   "exec-bad-time",
		Account:  "DU12345",
		Side:     "BOT",
		Shares:   "1",
		Price:    "150",
		Time:     "not-a-timestamp",
	})
	if err == nil {
		t.Fatal("fromCodecExecution() error = nil, want malformed execution time rejection")
	}
	_, err = parseExecutionTime("20260413 13:35:50 Not/AZone")
	if err == nil {
		t.Fatal("parseExecutionTime() accepted an unknown zone")
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
// captured in internal/codec/codec_test.go alongside a live open-order
// payload.
func TestParseOptionalDecimalTreatsMaxDoubleSentinelAsAbsent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{"canonical_uppercase", "1.7976931348623157E308"},
		{"lowercase_exponent", "1.7976931348623157e308"},
		{"protobuf_plus_exponent", "1.7976931348623157e+308"},
		{"surrounding_whitespace", "  1.7976931348623157E308\t"},
		{"official_unset_decimal", "-9223372036854775808"},
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

func TestParseOptionalDecimalPreservesValuesAroundMaxDouble(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		"-1.7976931348623157e+308",
		"1.7976931348623156e+308",
	} {
		value, err := parseOptionalDecimalPointer(raw, "test field")
		if err != nil {
			t.Fatalf("parseOptionalDecimalPointer(%q) error = %v", raw, err)
		}
		if value == nil || value.String() == "0" {
			t.Fatalf("parseOptionalDecimalPointer(%q) = %v, want preserved value", raw, value)
		}
	}

	for _, raw := range []string{"NaN", "+Inf", "-Inf"} {
		if _, err := parseOptionalDecimalPointer(raw, "test field"); err == nil {
			t.Fatalf("parseOptionalDecimalPointer(%q) error = nil, want malformed value", raw)
		}
	}
}

// TestFromCodecOpenOrderAcceptsSentinelCommissionFields is the end-to-end
// regression freeze for the reported P1: live TWS open-order traffic encodes
// unset commission/min/max commission — and, on non-WhatIf orders, the
// order-state margin fields — as the MAX_DOUBLE sentinel, and that must not
// tear down the open-order decode path.
func TestFromCodecOpenOrderAcceptsSentinelCommissionFields(t *testing.T) {
	t.Parallel()

	const sentinel = "1.7976931348623157E308"

	_, state, err := decodeCodecOpenOrder(codec.OpenOrder{
		OrderID:              1,
		Account:              "DU12345",
		Contract:             codec.Contract{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"},
		Action:               "BUY",
		OrderType:            "LMT",
		Quantity:             "1",
		LmtPrice:             "150.00",
		AuxPrice:             "0",
		InitMarginBefore:     sentinel,
		MaintMarginBefore:    sentinel,
		EquityWithLoanBefore: sentinel,
		InitMarginChange:     sentinel,
		MaintMarginChange:    sentinel,
		EquityWithLoanChange: sentinel,
		InitMarginAfter:      sentinel,
		MaintMarginAfter:     sentinel,
		EquityWithLoanAfter:  sentinel,
		Commission:           sentinel,
		MinCommission:        sentinel,
		MaxCommission:        sentinel,
	})
	if err != nil {
		t.Fatalf("decodeCodecOpenOrder() error = %v, want nil", err)
	}
	for name, got := range map[string]*decimal.Decimal{
		"InitMarginBefore":     state.InitMarginBefore,
		"MaintMarginBefore":    state.MaintMarginBefore,
		"EquityWithLoanBefore": state.EquityWithLoanBefore,
		"InitMarginChange":     state.InitMarginChange,
		"MaintMarginChange":    state.MaintMarginChange,
		"EquityWithLoanChange": state.EquityWithLoanChange,
		"InitMarginAfter":      state.InitMarginAfter,
		"MaintMarginAfter":     state.MaintMarginAfter,
		"EquityWithLoanAfter":  state.EquityWithLoanAfter,
		"Commission":           state.CommissionAndFees,
		"CommissionMin":        state.MinCommissionAndFees,
		"CommissionMax":        state.MaxCommissionAndFees,
	} {
		if got != nil {
			t.Errorf("%s = %s, want nil (sentinel should decode as absent)", name, got)
		}
	}
}

// TestFromCodecCommissionAcceptsSentinelFields freezes the receive-path
// contract for commission-and-fees reports: unset sentinels remain nil while
// literal zero remains a non-nil decimal.
func TestFromCodecCommissionAcceptsSentinelFields(t *testing.T) {
	t.Parallel()

	const sentinel = "1.7976931348623157E308"

	tests := []struct {
		name               string
		commission         string
		realized           string
		wantCommissionNil  bool
		wantRealizedPNLNil bool
	}{
		{"sentinel_commission", sentinel, "0", true, false},
		{"sentinel_realized", "1.25", sentinel, false, true},
		{"both_sentinel", sentinel, sentinel, true, true},
		{"both_empty", "", "", true, true},
		{"mixed_empty_sentinel", "", sentinel, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			report, err := fromCodecCommission(codec.CommissionReport{
				ExecID:      "exec-1",
				Commission:  tt.commission,
				Currency:    "USD",
				RealizedPNL: tt.realized,
			})
			if err != nil {
				t.Fatalf("fromCodecCommission() error = %v, want nil", err)
			}
			if report.ExecID != "exec-1" {
				t.Errorf("ExecID = %q, want %q", report.ExecID, "exec-1")
			}
			if report.Currency != "USD" {
				t.Errorf("Currency = %q, want %q", report.Currency, "USD")
			}
			if (report.Amount == nil) != tt.wantCommissionNil {
				t.Errorf("Amount = %v, want nil=%v", report.Amount, tt.wantCommissionNil)
			}
			if (report.RealizedPnL == nil) != tt.wantRealizedPNLNil {
				t.Errorf("RealizedPnL = %v, want nil=%v", report.RealizedPnL, tt.wantRealizedPNLNil)
			}
			if tt.realized == "0" && (report.RealizedPnL == nil || !report.RealizedPnL.IsZero()) {
				t.Errorf("literal zero RealizedPnL = %v, want non-nil zero", report.RealizedPnL)
			}
		})
	}
}

// TestFromCodecCommissionPreservesRealValues confirms that the sentinel fix
// did not alter decoding of real commission values.
func TestFromCodecCommissionPreservesRealValues(t *testing.T) {
	t.Parallel()

	report, err := fromCodecCommission(codec.CommissionReport{
		ExecID: "exec-2", Commission: "1.25", Currency: "USD", RealizedPNL: "-50.00",
		Yield: "2.75", YieldRedemptionDate: "20301231",
	})
	if err != nil {
		t.Fatalf("fromCodecCommission() error = %v, want nil", err)
	}
	if got := report.Amount.String(); got != "1.25" {
		t.Errorf("Commission = %s, want 1.25", got)
	}
	if got := report.RealizedPnL.String(); got != "-50" {
		t.Errorf("RealizedPnL = %s, want -50", got)
	}
	if report.BondYield == nil || report.BondYield.String() != "2.75" || report.YieldRedemptionDate != "20301231" {
		t.Errorf("yield/date = %v/%q", report.BondYield, report.YieldRedemptionDate)
	}
}

// TestFromCodecCommissionRejectsMalformedField freezes the rule that a
// genuinely malformed decimal (not a sentinel, not empty) still produces an
// error so the engine's log-and-drop path has something to report.
func TestFromCodecCommissionRejectsMalformedField(t *testing.T) {
	t.Parallel()

	_, err := fromCodecCommission(codec.CommissionReport{
		ExecID:      "exec-3",
		Commission:  "not-a-decimal",
		Currency:    "USD",
		RealizedPNL: "0",
	})
	if err == nil {
		t.Fatal("fromCodecCommission() error = nil, want malformed commission rejection")
	}
	_, err = fromCodecCommission(codec.CommissionReport{
		ExecID: "exec-4", Commission: "1", Currency: "USD", RealizedPNL: "0",
		YieldRedemptionDate: "20260230",
	})
	if err == nil {
		t.Fatal("fromCodecCommission() accepted an invalid redemption date")
	}
}

func TestParseExecutionTimeForms(t *testing.T) {
	t.Parallel()

	// The dash form is the Gateway's UTC notation, observed live on
	// 2026-06-10 execution_data frames (capture
	// 20260610T195819Z-api_order_trailing_cancel_aapl, events.jsonl sha256
	// 0d3098f03fd68839); the space-and-zone form and RFC3339 were already
	// accepted.
	cases := []struct {
		raw  string
		want time.Time
	}{
		{"20260610-19:58:22", time.Date(2026, 6, 10, 19, 58, 22, 0, time.UTC)},
		{"20260413 13:35:50 US/Eastern", time.Date(2026, 4, 13, 17, 35, 50, 0, time.UTC)},
		{"2026-06-10T19:58:22Z", time.Date(2026, 6, 10, 19, 58, 22, 0, time.UTC)},
	}
	for _, tc := range cases {
		got, err := parseExecutionTime(tc.raw)
		if err != nil {
			t.Errorf("parseExecutionTime(%q) error = %v", tc.raw, err)
			continue
		}
		if !got.Equal(tc.want) {
			t.Errorf("parseExecutionTime(%q) = %v, want %v", tc.raw, got.UTC(), tc.want)
		}
	}
}

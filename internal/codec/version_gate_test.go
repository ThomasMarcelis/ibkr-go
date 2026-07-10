package codec

import (
	"reflect"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

// encFieldsAt encodes a message at the given server version and returns the
// wire field slice, failing the test on error.
func encFieldsAt(t *testing.T, m Message, sv int) []string {
	t.Helper()
	f, err := m.encodeWire(sv)
	if err != nil {
		t.Fatalf("encodeWire(sv=%d) error: %v", sv, err)
	}
	return f
}

func fieldsContain(fields []string, v string) bool {
	for _, f := range fields {
		if f == v {
			return true
		}
	}
	return false
}

// TestPlaceOrderVersionGates freezes the outbound field-count deltas at every
// PlaceOrder version boundary. Field order and gate placement are taken from
// the official client (client.py:2463-2464, 2746, 2749, 2752-2754, 2756, 2759,
// 2762). Each boundary is isolated so only one gate toggles across the pair.
func TestPlaceOrderVersionGates(t *testing.T) {
	t.Parallel()

	order := PlaceOrderRequest{
		OrderID:              1,
		Contract:             Contract{ConID: 265598, Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"},
		Action:               "BUY",
		TotalQuantity:        "100",
		OrderType:            "LMT",
		LmtPrice:             "150.00",
		ExemptCode:           "-1",
		CustomerAccount:      "DU9999CUST",
		ProfessionalCustomer: "1",
		IncludeOvernight:     "1",
		ManualOrderIndicator: "5",
		ImbalanceOnly:        "1",
	}

	cases := []struct {
		name        string
		lowSV       int
		highSV      int
		wantDelta   int    // len(high) - len(low)
		valueOnHigh string // present only at highSV (or "" to skip)
		client      string
	}{
		{"faProfile removed at 177", 176, 177, -1, "", "client.py:2463-2464"},
		{"customerAccount added at 183", 182, 183, +1, "DU9999CUST", "client.py:2746"},
		{"professionalCustomer added at 184", 183, 184, +1, "", "client.py:2749"},
		{"RFQ window opens at 187", 186, 187, +2, "", "client.py:2752-2754"},
		{"includeOvernight added at 189", 188, 189, +1, "", "client.py:2756"},
		{"RFQ window closes at 190", 189, 190, -2, "", "client.py:2752-2754"},
		{"manualOrderIndicator added at 192", 191, 192, +1, "5", "client.py:2759"},
		{"imbalanceOnly added at 199", 198, 199, +1, "", "client.py:2762"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			low := encFieldsAt(t, order, tc.lowSV)
			high := encFieldsAt(t, order, tc.highSV)
			if got := len(high) - len(low); got != tc.wantDelta {
				t.Fatalf("%s: field-count delta = %d, want %d (%s)", tc.name, got, tc.wantDelta, tc.client)
			}
			if tc.valueOnHigh != "" {
				if !fieldsContain(high, tc.valueOnHigh) {
					t.Fatalf("%s: %q absent at sv=%d, want present", tc.name, tc.valueOnHigh, tc.highSV)
				}
				if fieldsContain(low, tc.valueOnHigh) {
					t.Fatalf("%s: %q present at sv=%d, want absent", tc.name, tc.valueOnHigh, tc.lowSV)
				}
			}
		})
	}
}

// TestExerciseOptionsVersionGates freezes the exercise-options tail gates
// (client.py:1775, 1779, 1783).
func TestExerciseOptionsVersionGates(t *testing.T) {
	t.Parallel()

	req := ExerciseOptionsRequest{
		ReqID:            9,
		Contract:         Contract{ConID: 1, Symbol: "AAPL", SecType: "OPT", Exchange: "SMART", Currency: "USD"},
		ExerciseAction:   1,
		ExerciseQuantity: 1,
		Account:          "DU123",
	}
	cases := []struct {
		name           string
		lowSV, highSV  int
		wantDelta      int
		clientCitation string
	}{
		{"manualOrderTime added at 180", 179, 180, +1, "client.py:1775"},
		{"customerAccount added at 183", 182, 183, +1, "client.py:1779"},
		{"professionalCustomer added at 184", 183, 184, +1, "client.py:1783"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := len(encFieldsAt(t, req, tc.highSV)) - len(encFieldsAt(t, req, tc.lowSV))
			if got != tc.wantDelta {
				t.Fatalf("%s: delta = %d, want %d (%s)", tc.name, got, tc.wantDelta, tc.clientCitation)
			}
		})
	}
}

// TestCancelOrderVersionGates freezes the cancel-order gates: the legacy
// version field drops at 192 while extOperator+manualOrderIndicator appear, and
// the RFQ placeholders live only in [187,190) (client.py:2899-2912).
func TestCancelOrderVersionGates(t *testing.T) {
	t.Parallel()

	req := CancelOrderRequest{OrderID: 42, ExtOperator: "op", ManualOrderIndicator: "3"}

	// [OutCancelOrder, "1", orderId, manualCancelTime] at 176: 4 fields.
	if got := encFieldsAt(t, req, 176); len(got) != 4 || got[1] != "1" {
		t.Fatalf("sv=176 cancel = %v, want 4 fields with legacy version \"1\"", got)
	}
	// RFQ window opens at 187: +3 placeholders vs 186.
	if d := len(encFieldsAt(t, req, 187)) - len(encFieldsAt(t, req, 186)); d != 3 {
		t.Fatalf("RFQ open delta 186->187 = %d, want +3 (client.py:2905-2908)", d)
	}
	// At 192 the version field drops (-1) and extOperator+manualOrderIndicator
	// appear (+2) while the RFQ window has already closed (191 has none).
	at191 := encFieldsAt(t, req, 191)
	at192 := encFieldsAt(t, req, 192)
	if d := len(at192) - len(at191); d != 1 {
		t.Fatalf("191->192 delta = %d, want +1 (drop version, add ext+manual)", d)
	}
	if fieldsContain(at192, "1") && at192[1] == "1" {
		t.Fatalf("sv=192 cancel still carries leading legacy version field: %v", at192)
	}
	if !fieldsContain(at192, "op") || !fieldsContain(at192, "3") {
		t.Fatalf("sv=192 cancel missing extOperator/manualOrderIndicator: %v", at192)
	}
}

// TestGlobalCancelVersionGates freezes the global-cancel version boundary at
// CME_TAGGING_FIELDS (client.py:3131-3136).
func TestGlobalCancelVersionGates(t *testing.T) {
	t.Parallel()

	req := GlobalCancelRequest{ExtOperator: "op", ManualOrderIndicator: "3"}
	low := encFieldsAt(t, req, 191)  // [58, "1"]
	high := encFieldsAt(t, req, 192) // [58, "op", "3"]
	if len(low) != 2 || low[1] != "1" {
		t.Fatalf("sv=191 global cancel = %v, want [58 1]", low)
	}
	if len(high) != 3 || high[1] != "op" || high[2] != "3" {
		t.Fatalf("sv=192 global cancel = %v, want [58 op 3]", high)
	}
}

// TestExecutionsRequestVersionGate freezes the parametrized-days tail at 200
// (client.py:4085-4100).
func TestExecutionsRequestVersionGate(t *testing.T) {
	t.Parallel()

	req := ExecutionsRequest{ReqID: 1, Account: "DU123", Symbol: "AAPL"}
	if d := len(encFieldsAt(t, req, 200)) - len(encFieldsAt(t, req, 199)); d != 2 {
		t.Fatalf("199->200 delta = %d, want +2 (lastNDays + specificDates count)", d)
	}
	if got := encFieldsAt(t, req, 200); got[len(got)-2] != "2147483647" || got[len(got)-1] != "0" {
		t.Fatalf("sv=200 executions tail = %v, want lastNDays=2147483647 count=0", got[len(got)-2:])
	}
}

func TestExecutionDetailVersionGates(t *testing.T) {
	t.Parallel()

	msg := ExecutionDetail{
		ReqID: 1, OrderID: 2, Contract: Contract{Symbol: "AAPL"},
		ExecID: "exec", Time: "20260709-12:00:00", Account: "DU1",
		Side: "BOT", Shares: "1", Price: "200", CumulativeQuantity: "1",
		AveragePrice: "200", LastLiquidity: "2", PendingPriceRevision: "1",
		Submitter: "operator",
	}
	at177 := encFieldsAt(t, msg, 177)
	at178 := encFieldsAt(t, msg, 178)
	at197 := encFieldsAt(t, msg, 197)
	at198 := encFieldsAt(t, msg, 198)
	if len(at178)-len(at177) != 1 || at178[len(at178)-1] != "1" {
		t.Fatalf("177->178 pending-revision gate: sv177=%v sv178=%v", at177, at178)
	}
	if len(at198)-len(at197) != 1 || at198[len(at198)-1] != "operator" {
		t.Fatalf("197->198 submitter gate: sv197=%v sv198=%v", at197, at198)
	}
	if at177[len(at177)-1] != "2" {
		t.Fatalf("sv177 last liquidity = %q, want 2", at177[len(at177)-1])
	}
}

func TestExecutionAndCommissionRejectTrailingFields(t *testing.T) {
	t.Parallel()

	tests := []Message{
		ExecutionDetail{
			Contract: Contract{Symbol: "AAPL"}, ExecID: "exec", Time: "20260709-12:00:00",
			Shares: "1", Price: "200", CumulativeQuantity: "1", AveragePrice: "200",
		},
		CommissionReport{ExecID: "exec", Commission: "1", Currency: "USD", RealizedPNL: "0"},
	}
	for _, msg := range tests {
		payload, err := Encode(200, msg)
		if err != nil {
			t.Fatalf("Encode(%T): %v", msg, err)
		}
		payload = append(payload, []byte("unexpected\x00")...)
		if _, err := DecodeBatch(200, payload); err == nil {
			t.Errorf("DecodeBatch(%T with trailing field) error = nil", msg)
		}
	}
}

// TestErrMsgVersionGate freezes the error-message layout across ERROR_TIME
// (194): below it a leading version int precedes reqId and no errorTime
// trails; at/above it the version int is gone and errorTime trails
// (decoder.py:2368-2382). Overlapping fields decode identically.
func TestErrMsgVersionGate(t *testing.T) {
	t.Parallel()

	// Old layout (sv 193): [4, version, reqId, code, msg, advJson].
	oldFields := []string{itoa(InErrMsg), "1", "-1", "2104", "Market data farm OK", ""}
	// New layout (sv 194): [4, reqId, code, msg, advJson, errorTime].
	newFields := []string{itoa(InErrMsg), "-1", "2104", "Market data farm OK", "", "1712345678000"}

	oldMsg := decodeSingle[APIError](t, 193, oldFields)
	newMsg := decodeSingle[APIError](t, 194, newFields)

	if oldMsg.ReqID != -1 || oldMsg.Code != 2104 || oldMsg.Message != "Market data farm OK" {
		t.Fatalf("sv193 errMsg overlap wrong: %+v", oldMsg)
	}
	if oldMsg.ReqID != newMsg.ReqID || oldMsg.Code != newMsg.Code || oldMsg.Message != newMsg.Message {
		t.Fatalf("errMsg overlap mismatch: old=%+v new=%+v", oldMsg, newMsg)
	}
	if oldMsg.ErrorTimeMs != "" {
		t.Fatalf("sv193 errMsg should carry no errorTime, got %q", oldMsg.ErrorTimeMs)
	}
	if newMsg.ErrorTimeMs != "1712345678000" {
		t.Fatalf("sv194 errMsg errorTime = %q, want 1712345678000", newMsg.ErrorTimeMs)
	}
}

// TestContractDetailsVersionGate freezes the explicit-lastTradeDate gate at
// LAST_TRADE_DATE (182): below it the field is absent, at/above it a field sits
// between lastTradeDateOrContractMonth and strike (decoder.py:509-510). The
// public contract fields decode identically across the boundary.
func TestContractDetailsVersionGate(t *testing.T) {
	t.Parallel()

	// Shared prefix and suffix around the gated slot. Fields mirror
	// decodeContractData's read order.
	base := func(withLastTradeDate bool) []string {
		f := []string{
			itoa(InContractData), "7", // msg_id, reqId
			"AAPL", "STK", // symbol, secType
			"20260101", // lastTradeDateOrContractMonth
		}
		if withLastTradeDate {
			f = append(f, "20260101") // explicit lastTradeDate (sv>=182)
		}
		f = append(f,
			"150.0", "C", // strike, right
			"SMART", "USD", // exchange, currency
			"AAPL", "AAPL_MKT", "AAPL", // localSymbol, marketName, tradingClass
			"265598", "0.01", "100", // conId, minTick, multiplier
			"", "", "", "", // orderTypes, validExchanges, priceMagnifier, underConId
			"APPLE INC", "NASDAQ", // longName, primaryExchange
			"", "", "", "", // contractMonth, industry, category, subcategory
			"America/New_York", // timeZoneId
			"", "",             // tradingHours, liquidHours
			"", "", // economic value rule and multiplier
			"0",                      // security id count
			"2147483647", "", "", "", // aggGroup, underSymbol, underSecType, marketRuleIds
			"", "", // realExpirationDate, stockType
			"0.0001", "0.0001", "1", // size rules
		)
		return f
	}

	oldMsg := decodeSingle[ContractDetails](t, 181, base(false))
	newMsg := decodeSingle[ContractDetails](t, 182, base(true))

	if oldMsg.Contract.Symbol != "AAPL" || oldMsg.Contract.Strike != "150.0" ||
		oldMsg.Contract.ConID != 265598 || oldMsg.TimeZoneID != "America/New_York" {
		t.Fatalf("sv181 contract details desync: %+v", oldMsg)
	}
	if !reflect.DeepEqual(oldMsg.Contract, newMsg.Contract) || oldMsg.MinTick != newMsg.MinTick ||
		oldMsg.LongName != newMsg.LongName || oldMsg.TimeZoneID != newMsg.TimeZoneID {
		t.Fatalf("contract details overlap mismatch: old=%+v new=%+v", oldMsg, newMsg)
	}
	if oldMsg.LastTradeDate != "" || newMsg.LastTradeDate != "20260101" {
		t.Fatalf("explicit last trade date gate: old=%q new=%q", oldMsg.LastTradeDate, newMsg.LastTradeDate)
	}
}

func TestBondContractDetailsTradingHoursVersionGate(t *testing.T) {
	t.Parallel()

	msg := BondContractDetails{
		ContractDetails: ContractDetails{
			ReqID: 7, Contract: Contract{SecType: "BOND"},
			TimeZoneID: "US/Eastern", TradingHours: "trading", LiquidHours: "liquid",
			LastTradeTime: "17:00:00",
			AggGroup:      7, MarketRuleIDs: "1386", MinSize: "2", SizeIncrement: "1", SuggestedSizeIncrement: "1",
		},
		Maturity: "20430504",
	}

	at187 := encFieldsAt(t, msg, 187)
	at188 := encFieldsAt(t, msg, 188)
	if got := len(at188) - len(at187); got != 3 {
		t.Fatalf("field delta at bond trading-hours gate = %d, want 3", got)
	}
	decoded := decodeSingle[BondContractDetails](t, 187, at187)
	if decoded.Maturity != "20430504" || decoded.LastTradeTime != "17:00:00" || decoded.TimeZoneID != "US/Eastern" {
		t.Errorf("pre-gate maturity metadata = %q %q %q", decoded.Maturity, decoded.LastTradeTime, decoded.TimeZoneID)
	}
}

func TestContractDetailsFundAndIneligibilityVersionGates(t *testing.T) {
	t.Parallel()

	// VTSAX values are from captures/20260415T150322Z-api_security_type_probe_matrix,
	// server_version=200, events SHA-256 prefix 9be83e57ed176a17.
	msg := ContractDetails{
		Contract: Contract{Symbol: "VTSAX", SecType: "FUND", Exchange: "FUNDSERV", Currency: "USD"},
		Fund: &FundDetails{
			Name: "Vanguard Total Stock Market Index Fund A", Family: "Vanguard",
			ManagementFee: "0.04", MinimumInitialPurchase: "3000",
			MinimumSubsequentPurchase: "1", BlueSkyStates: "All",
		},
	}
	at178 := encFieldsAt(t, msg, 178)
	at179 := encFieldsAt(t, msg, 179)
	if len(at179)-len(at178) != 17 {
		t.Fatalf("178->179 fund tail delta = %d, want 17", len(at179)-len(at178))
	}
	if got := at179[len(at179)-17]; got != "Vanguard Total Stock Market Index Fund A" {
		t.Fatalf("first fund field = %q", got)
	}
	at185 := encFieldsAt(t, msg, 185)
	at186 := encFieldsAt(t, msg, 186)
	if len(at186)-len(at185) != 1 || at186[len(at186)-1] != "0" {
		t.Fatalf("185->186 ineligibility count gate: sv185=%v sv186=%v", at185, at186)
	}
}

func TestContractDetailsRejectsTrailingFields(t *testing.T) {
	t.Parallel()

	payload, err := Encode(200, ContractDetails{Contract: Contract{Symbol: "AAPL", SecType: "STK"}})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	payload = append(payload, []byte("unexpected\x00")...)
	if _, err := DecodeBatch(200, payload); err == nil {
		t.Fatal("DecodeBatch() error = nil, want trailing-field rejection")
	}
}

// TestHistoricalDataVersionGate freezes the inline start/end dates gate at
// HISTORICAL_DATA_END (196): below it the terminal frame inlines start/end
// dates before the bar count and synthesizes an end marker; at/above it the
// packed IN 17 frame carries bars only and standalone IN 108 owns completion.
func TestHistoricalDataVersionGate(t *testing.T) {
	t.Parallel()

	// Old layout (sv 195): [17, reqId, startDate, endDate, barCount, bars...].
	oldFields := []string{
		itoa(InHistoricalData), "3", "20260101", "20260102", "1",
		"20260101", "1.0", "2.0", "0.5", "1.5", "100", "1.2", "7",
	}
	// New layout (sv 196): [17, reqId, barCount, bars...].
	newFields := []string{
		itoa(InHistoricalData), "3", "1",
		"20260101", "1.0", "2.0", "0.5", "1.5", "100", "1.2", "7",
	}

	oldMsgs := decodeBatch(t, 195, oldFields)
	newMsgs := decodeBatch(t, 196, newFields)

	if len(oldMsgs) != 2 || len(newMsgs) != 1 {
		t.Fatalf("want sv195 [bar, end] and sv196 [bar], got old=%d new=%d", len(oldMsgs), len(newMsgs))
	}
	oldBar, ok := oldMsgs[0].(HistoricalBar)
	if !ok {
		t.Fatalf("sv195 msgs[0] = %T, want HistoricalBar", oldMsgs[0])
	}
	newBar := newMsgs[0].(HistoricalBar)
	if oldBar != newBar {
		t.Fatalf("bar mismatch across gate: old=%+v new=%+v", oldBar, newBar)
	}
	oldEnd := oldMsgs[1].(HistoricalBarsEnd)
	if oldEnd.StartDate != "20260101" || oldEnd.EndDate != "20260102" {
		t.Fatalf("sv195 end marker start/end = %q/%q, want 20260101/20260102", oldEnd.StartDate, oldEnd.EndDate)
	}
}

// openOrderTailWidth mirrors the decode-side tail formula so the builder emits
// exactly the fields the decoder expects at a given version
// (orderdecoder.py:372-391).
func openOrderTailWidth(sv int) int {
	n := 24
	for _, gate := range []int{
		MinServerVersionCustomerAccount,
		MinServerVersionProfessionalCustomer,
		MinServerVersionBondAccruedInterest,
		MinServerVersionIncludeOvernight,
		MinServerVersionSubmitter,
		MinServerVersionImbalanceOnly,
	} {
		if sv >= gate {
			n++
		}
	}
	if sv >= MinServerVersionCMETaggingFieldsInOpenOrder {
		n += 2
	}
	return n
}

// buildOpenOrderFields assembles a full server->client open_order frame for a
// simple LMT order (no combo legs, no algo, no conditions, deltaNeutral
// sentinel "None") at the given server version. The field order and version
// gates mirror decodeOpenOrder / orderdecoder.py (faProfile 136-137,
// FULL_ORDER_PREVIEW 369-395, tail 372-391). Distinctive status/margin values
// let callers assert the overlapping semantics decode identically.
func buildOpenOrderFields(sv int) []string {
	f := []string{itoa(InOpenOrder), "42"}
	// contract (11)
	f = append(f, "265598", "AAPL", "STK", "", "0.0", "", "", "SMART", "USD", "AAPL", "NMS")
	// core order (8)
	f = append(f, "BUY", "100", "LMT", "150.00", "", "DAY", "", "DU123")
	// detail (9)
	f = append(f, "O", "0", "ref1", "7", "123456", "0", "0", "0", "")
	// sharesAllocation, FAGroup, FAMethod, FAPercentage
	f = append(f, "", "", "", "")
	if sv < MinServerVersionFAProfileDesupport {
		f = append(f, "") // deprecated faProfile (orderdecoder.py:136-137)
	}
	// modelCode..NBBOPriceCap (23)
	for range 23 {
		f = append(f, "")
	}
	// preStatusParentID, triggerMethod
	f = append(f, "0", "")
	// volatility, volatilityType, deltaNeutralOrderType
	f = append(f, "", "", "None")
	// deltaNeutralAuxPrice
	f = append(f, "")
	// delta-neutral block (8, skipped because sentinel is "None")
	for range 8 {
		f = append(f, "0")
	}
	// continuousUpdate..comboLegsDescrip (7)
	for range 7 {
		f = append(f, "")
	}
	// combo legs count, order combo leg prices count, smart combo routing count
	f = append(f, "0", "0", "0")
	// scaleInit, scaleSubs, scalePriceIncrement (empty => no scale block)
	f = append(f, "2147483647", "2147483647", "")
	// hedgeType (empty => no hedgeParam)
	f = append(f, "")
	// optOutSmartRouting, clearingAccount, clearingIntent, notHeld
	f = append(f, "", "", "", "")
	// deltaNeutralContractPresent ("0")
	f = append(f, "0")
	// algoStrategy (empty => no params)
	f = append(f, "")
	// solicited, whatIf
	f = append(f, "", "")
	// status
	f = append(f, "Submitted")
	// margin/commission section (13)
	f = append(f,
		"im_b", "mm_b", "ewl_b",
		"im_c", "mm_c", "ewl_c",
		"im_a", "mm_a", "ewl_a",
		"comm", "mincomm", "maxcomm", "USD",
	)
	if sv >= MinServerVersionFullOrderPreviewFields {
		// FULL_ORDER_PREVIEW: marginCurrency + 9 outsideRTH + suggestedSize +
		// rejectReason + allocationsCount(0) = 13 (orderdecoder.py:369-395).
		for range 12 {
			f = append(f, "")
		}
		f = append(f, "0") // order allocations count
	}
	// warningText (unconditional)
	f = append(f, "")
	// randomizeSize, randomizePrice
	f = append(f, "", "")
	// conditions count (0)
	f = append(f, "0")
	// version-gated tail
	for range openOrderTailWidth(sv) {
		f = append(f, "")
	}
	return f
}

// TestOpenOrderVersionGates freezes the inbound open_order gates. A frame built
// at each version decodes fully (not partial) with the overlapping order-state
// fields intact, and the faProfile and FULL_ORDER_PREVIEW blocks change the
// frame width by exactly their gated field counts.
func TestOpenOrderVersionGates(t *testing.T) {
	t.Parallel()

	for _, sv := range []int{176, 177, 182, 187, 193, 194, 195, 198, 199, 200} {
		t.Run("full-decode-sv"+itoa(sv), func(t *testing.T) {
			msg := decodeSingle[OpenOrder](t, sv, buildOpenOrderFields(sv))
			if msg.Partial {
				t.Fatalf("sv=%d: frame decoded partial, want full", sv)
			}
			if msg.OrderID != 42 || msg.Action != "BUY" || msg.OrderType != "LMT" ||
				msg.Status != "Submitted" || msg.CommissionCurrency != "USD" ||
				msg.InitMarginBefore != "im_b" || msg.MaxCommission != "maxcomm" {
				t.Fatalf("sv=%d: overlapping fields wrong: %+v", sv, msg)
			}
		})
	}

	// faProfile gate: the 176 frame carries exactly one extra field vs 177.
	if d := len(buildOpenOrderFields(176)) - len(buildOpenOrderFields(177)); d != 1 {
		t.Fatalf("faProfile width delta 177->176 = %d, want +1 (orderdecoder.py:136-137)", d)
	}
	// FULL_ORDER_PREVIEW gate: the 195 frame carries 13 more fields than 194.
	if d := len(buildOpenOrderFields(195)) - len(buildOpenOrderFields(194)); d != 13 {
		t.Fatalf("full-preview width delta 194->195 = %d, want +13 (orderdecoder.py:369-395)", d)
	}

	// Tail-width sensitivity: a genuine sv200 frame has an extra tail field
	// (imbalanceOnly) vs sv198, so decoding sv200 bytes at sv198 overshoots the
	// expected tail and falls back to the partial decode.
	partial := decodeSingle[OpenOrder](t, 198, buildOpenOrderFields(200))
	if !partial.Partial {
		t.Fatalf("sv200 frame decoded at sv198 should be partial (tail too long)")
	}
	if partial.Status != "" {
		t.Fatalf("partial open order should have empty Status, got %q", partial.Status)
	}
}

// decodeSingle decodes fields (with msg_id prefix) at sv and asserts a single
// message of type T.
func decodeSingle[T Message](t *testing.T, sv int, fields []string) T {
	t.Helper()
	msgs := decodeBatch(t, sv, fields)
	if len(msgs) != 1 {
		t.Fatalf("decode sv=%d: got %d messages, want 1", sv, len(msgs))
	}
	out, ok := msgs[0].(T)
	if !ok {
		var zero T
		t.Fatalf("decode sv=%d: got %T, want %T", sv, msgs[0], zero)
	}
	return out
}

func decodeBatch(t *testing.T, sv int, fields []string) []Message {
	t.Helper()
	msgs, err := DecodeBatch(sv, wire.EncodeFields(fields))
	if err != nil {
		t.Fatalf("DecodeBatch(sv=%d) error: %v", sv, err)
	}
	return msgs
}

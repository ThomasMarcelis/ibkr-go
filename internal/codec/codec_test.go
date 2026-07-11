package codec

import (
	"fmt"
	"slices"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	t.Parallel()

	// Test messages that have consistent encode/decode via integer msg_id wire format.
	// Only includes types where Encode → DecodeBatch produces the same message.
	tests := []struct {
		msg  Message
		name string
	}{
		{ManagedAccounts{Accounts: []string{"DU12345", "DU67890"}}, "codec.ManagedAccounts"},
		{NextValidID{OrderID: 1001}, "codec.NextValidID"},
		{CurrentTime{Time: "1712345678"}, "codec.CurrentTime"},
		{CurrentTimeMillis{TimeMs: "1781169286652"}, "codec.CurrentTimeMillis"},
		{APIError{ReqID: -1, Code: 2104, Message: "Market data farm OK", AdvancedOrderRejectJSON: "", ErrorTimeMs: "1712345678000"}, "codec.APIError"},
		{ContractDetailsEnd{ReqID: 42}, "codec.ContractDetailsEnd"},
		{AccountSummaryValue{ReqID: 1, Account: "DU12345", Tag: "NetLiquidation", Value: "100000.00", Currency: "USD"}, "codec.AccountSummaryValue"},
		{AccountSummaryEnd{ReqID: 1}, "codec.AccountSummaryEnd"},
		{TickPrice{ReqID: 1, TickType: 1, Price: "189.10", Size: "400", AttrMask: 0}, "codec.TickPrice"},
		{TickSize{ReqID: 1, TickType: 0, Size: "400"}, "codec.TickSize"},
		{MarketDataType{ReqID: 1, DataType: 3}, "codec.MarketDataType"},
		{TickSnapshotEnd{ReqID: 1}, "codec.TickSnapshotEnd"},
		{RealTimeBar{ReqID: 1, Time: "1712345678", Open: "100.0", High: "101.0", Low: "99.5", Close: "100.5", Volume: "1000", WAP: "100.5", Count: "50"}, "codec.RealTimeBar"},
		{CommissionReport{ExecID: "exec-1", Commission: "1.00", Currency: "USD", RealizedPNL: "50.00"}, "codec.CommissionReport"},
		{TickGeneric{ReqID: 1, TickType: 49, Value: "0"}, "codec.TickGeneric"},
		{TickString{ReqID: 1, TickType: 45, Value: "1712300400"}, "codec.TickString"},
		{TickNews{ReqID: 1, Time: "1758294759000", ProviderCode: "BRFG", ArticleID: "BRFG$1c2d5728", Headline: "Headline", ExtraData: "L:en"}, "codec.TickNews"},
		{TickReqParams{ReqID: 1, MinTick: "0.01", BBOExchange: "SMART", SnapshotPermissions: new(3)}, "codec.TickReqParams"},
		{ExecutionDetail{ReqID: 1, OrderID: 42, Contract: Contract{Symbol: "AAPL"}, ExecID: "0001", Account: "DU12345", Side: "BOT", Shares: "100", Price: "150.50", Time: "20260407 10:30:00"}, "codec.ExecutionDetail"},
		{ExecutionsEnd{ReqID: 1}, "codec.ExecutionsEnd"},
		{OpenOrder{
			OrderID: 42, Account: "DU12345",
			Contract: Contract{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"},
			Action:   "BUY", Quantity: "10", OrderType: "LMT",
			LmtPrice: "150.00", AuxPrice: "0.0", TIF: "DAY",
			OpenClose: "", Origin: "0", OrderRef: "test-ref",
			ClientID: "99", PermID: "123456", OutsideRTH: "0",
			Hidden: "0", DiscretionAmt: "0", GoodAfterTime: "",
			Status:           "Submitted",
			InitMarginBefore: "1.7976931348623157E308", MaintMarginBefore: "1.7976931348623157E308",
			ParentID: "99",
		}, "codec.OpenOrder"},
		{OpenOrderEnd{}, "codec.OpenOrderEnd"},
		{PositionEnd{}, "codec.PositionEnd"},
		{OrderStatus{
			OrderID: 42, Status: "Filled", Filled: "100", Remaining: "0",
			AvgFillPrice: "150.50", PermID: "123456", ParentID: "0",
			LastFillPrice: "150.50", ClientID: "99", WhyHeld: "", MktCapPrice: "0",
		}, "codec.OrderStatus"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload, err := Encode(200, tt.msg)
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}
			msgs, err := DecodeBatch(200, payload)
			if err != nil {
				t.Fatalf("DecodeBatch() error = %v", err)
			}
			if len(msgs) == 0 {
				t.Fatal("DecodeBatch() returned 0 messages")
			}
			if got := fmt.Sprintf("%T", msgs[0]); got != tt.name {
				t.Fatalf("message type = %q, want %q", got, tt.name)
			}
		})
	}
}

func TestDecodeOrderBoundClassicAndProtobuf(t *testing.T) {
	t.Parallel()

	want := OrderBound{PermID: 123456789, ClientID: 0, OrderID: 42}
	classic, err := Encode(200, want)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(200, classic)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != want {
		t.Fatalf("classic OrderBound = %#v, want %#v", decoded, want)
	}

	body := appendProtoVarint(nil, 1, 123456789)
	body = appendProtoVarint(body, 2, 0)
	body = appendProtoVarint(body, 3, 42)
	protobuf, err := protocol.EncodeProtobufEnvelope(206, protocol.InOrderBound, body)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err = Decode(206, protobuf)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != want {
		t.Fatalf("protobuf OrderBound = %#v, want %#v", decoded, want)
	}
}

func TestManagedAccountsRequestExactServer206(t *testing.T) {
	t.Parallel()

	// Exact request from official API 10.48.01 against the readonly live
	// Gateway capped at server_version 206. Capture
	// 20260711T011054Z-sdk_exact206_managed_accounts_refresh,
	// events.jsonl SHA-256 433a03994d7fb1ec2af34870fdf156f8406fafabc8cb0ccbd1c5dedc70942e58.
	payload, err := Encode(206, ManagedAccountsRequest{})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	want := []byte("\x00\x00\x00\x111\x00")
	if !slices.Equal(payload, want) {
		t.Fatalf("Encode() = %x, want %x", payload, want)
	}
}

func TestDecodeLiveSymbolSamplesFrameShape(t *testing.T) {
	t.Parallel()

	payload := wire.EncodeFields([]string{
		"79", "1001", "2",
		"265598", "AAPL", "STK", "NASDAQ", "USD", "5", "CFD", "OPT", "IOPT", "WAR", "BAG", "APPLE INC", "",
		"38708077", "AAPL", "STK", "MEXI", "MXN", "0", "APPLE INC", "",
	})

	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch() error = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("messages len = %d, want 1", len(msgs))
	}
	m, ok := msgs[0].(MatchingSymbols)
	if !ok {
		t.Fatalf("message type = %T, want MatchingSymbols", msgs[0])
	}
	if len(m.Symbols) != 2 {
		t.Fatalf("symbols len = %d, want 2", len(m.Symbols))
	}
	if m.Symbols[0].Description != "APPLE INC" || m.Symbols[0].IssuerID != "" {
		t.Fatalf("first symbol metadata = %#v", m.Symbols[0])
	}
	if !slices.Equal(m.Symbols[0].DerivativeSecTypes, []string{"CFD", "OPT", "IOPT", "WAR", "BAG"}) {
		t.Fatalf("derivative types = %#v", m.Symbols[0].DerivativeSecTypes)
	}
}

func TestDecodeByMsgID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		fields []string
		want   string
	}{
		{"managed_accounts", []string{"15", "1", "DU12345,DU67890"}, "codec.ManagedAccounts"},
		{"next_valid_id", []string{"9", "1", "1001"}, "codec.NextValidID"},
		{"current_time", []string{"49", "1", "1712345678"}, "codec.CurrentTime"},
		{"api_error", []string{"4", "-1", "2104", "Market data farm connected", "", "1712345678000"}, "codec.APIError"},
		{"tick_generic", []string{"45", "6", "1", "49", "0"}, "codec.TickGeneric"},
		{"tick_string", []string{"46", "6", "1", "45", "1712300400"}, "codec.TickString"},
		{"tick_news", []string{"84", "1", "1758294759000", "BRFG", "BRFG$1c2d5728", "Headline", "L:en"}, "codec.TickNews"},
		{"tick_req_params", []string{"81", "1", "0.01", "SMART", "3"}, "codec.TickReqParams"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload := wire.EncodeFields(tt.fields)
			msgs, err := DecodeBatch(200, payload)
			if err != nil {
				t.Fatalf("DecodeBatch() error = %v", err)
			}
			if len(msgs) != 1 {
				t.Fatalf("DecodeBatch() len = %d, want 1", len(msgs))
			}
			if got := fmt.Sprintf("%T", msgs[0]); got != tt.want {
				t.Fatalf("message type = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEncodeHeadTimestampRequestFieldOrder(t *testing.T) {
	t.Parallel()

	payload, err := Encode(200, HeadTimestampRequest{
		ReqID: 42,
		Contract: Contract{
			Symbol:   "AAPL",
			SecType:  "STK",
			Exchange: "SMART",
			Currency: "USD",
		},
		WhatToShow: "TRADES",
		UseRTH:     true,
	})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	fields, err := wire.ParseFields(payload)
	if err != nil {
		t.Fatalf("ParseFields() error = %v", err)
	}
	if len(fields) < 18 {
		t.Fatalf("fields len = %d, want at least 18", len(fields))
	}
	if fields[14] != "0" {
		t.Fatalf("includeExpired field = %q, want 0", fields[14])
	}
	if fields[15] != "1" {
		t.Fatalf("useRTH field = %q, want 1", fields[15])
	}
	if fields[16] != "TRADES" {
		t.Fatalf("whatToShow field = %q, want TRADES", fields[16])
	}
	if fields[17] != "1" {
		t.Fatalf("formatDate field = %q, want 1", fields[17])
	}
}

func TestDecodeRejectsMissingOrEmptyCounts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		fields []string
	}{
		{"HistoricalData/bad_reqID", []string{"17", "bad", "0"}},
		{"HistoricalData/missing_count", []string{"17", "1"}},
		{"HistoricalData/empty_count", []string{"17", "1", ""}},
		{"HistoricalDataUpdate/missing_count", []string{"90", "1"}},
		{"HistoricalDataUpdate/empty_count", []string{"90", "1", ""}},
		{"HistoricalDataUpdate/bad_count", []string{"90", "1", "bad", "t", "o", "c", "h", "l", "w", "v"}},
		{"ScannerData/missing_count", []string{"20", "3", "1"}},
		{"ScannerData/empty_count", []string{"20", "3", "1", ""}},
		{"FamilyCodes/missing_count", []string{"78"}},
		{"FamilyCodes/empty_count", []string{"78", ""}},
		{"HistoricalTicks/missing_count", []string{"96", "1"}},
		{"HistoricalTicks/empty_count", []string{"96", "1", ""}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := DecodeBatch(200, wire.EncodeFields(tt.fields))
			if err == nil {
				t.Fatal("DecodeBatch() error = nil, want malformed count error")
			}
		})
	}
}

func TestDecodeHistoricalDataEnd(t *testing.T) {
	t.Parallel()

	// Live shape is [108, reqID, startDateTime, endDateTime]; some captures also
	// show a variant without the end timestamp.
	tests := [][]string{
		{"108", "1", "20260407 08:52:59 US/Eastern"},
		{"108", "1", "20260407 10:23:05 US/Eastern", "20260412 10:23:05 US/Eastern"},
	}
	for _, fields := range tests {
		t.Run(fields[len(fields)-1], func(t *testing.T) {
			t.Parallel()

			msgs, err := DecodeBatch(200, wire.EncodeFields(fields))
			if err != nil {
				t.Fatalf("DecodeBatch() error = %v", err)
			}
			if len(msgs) != 1 {
				t.Fatalf("DecodeBatch() len = %d, want 1", len(msgs))
			}
			end, ok := msgs[0].(HistoricalBarsEnd)
			if !ok {
				t.Fatalf("message = %T, want HistoricalBarsEnd", msgs[0])
			}
			if end.ReqID != 1 {
				t.Fatalf("HistoricalBarsEnd.ReqID = %d, want request id from terminal frame", end.ReqID)
			}
			if end.StartDate != fields[2] {
				t.Fatalf("HistoricalBarsEnd.StartDate = %q, want %q", end.StartDate, fields[2])
			}
		})
	}
}

func TestDecodeHistoricalDataUpdateBar(t *testing.T) {
	t.Parallel()

	// HISTORICAL_DATA_UPDATE (IN 90) streaming bar, official layout:
	// [90, reqID, barCount, time, open, close, high, low, WAP, volume].
	// Source-referenced; live attestation pending (see WIRE_TRUTH.md).
	msgs, err := DecodeBatch(200, wire.EncodeFields([]string{
		"90", "42", "1",
		"20260412 10:30:00 US/Eastern", "100.00", "100.50", "101.00", "99.50", "100.25", "1500",
	}))
	if err != nil {
		t.Fatalf("DecodeBatch() error = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("DecodeBatch() len = %d, want 1", len(msgs))
	}
	update, ok := msgs[0].(HistoricalDataUpdate)
	if !ok {
		t.Fatalf("message = %T, want HistoricalDataUpdate", msgs[0])
	}
	if update.ReqID != 42 || update.BarCount != 1 || update.Close != "100.50" ||
		update.High != "101.00" || update.Low != "99.50" || update.WAP != "100.25" || update.Volume != "1500" {
		t.Fatalf("HistoricalDataUpdate = %#v", update)
	}
}

// TestDecodeOpenOrderNonSimple verifies that an OpenOrder payload whose
// variable sections do not follow the live sv200 layout (here: an empty
// DeltaNeutralOrderType instead of the live "None" sentinel) produces a
// partial parse with only the reliably-positioned pre-variable-section fields.
func TestDecodeOpenOrderNonSimple(t *testing.T) {
	t.Parallel()

	// Build a synthetic OpenOrder payload with extra fields (simulating a
	// combo order with variable-length sections that expand the message).
	fields := make([]string, 0, 180)
	fields = append(fields, itoa(protocol.InOpenOrder))                                         // msg_id
	fields = append(fields, "42")                                                               // r[0] orderID
	fields = append(fields, "265598", "AAPL", "STK", "", "", "", "", "SMART", "USD", "", "NMS") // r[1..11] contract
	fields = append(fields, "BUY", "10", "LMT", "150.00", "0", "DAY", "", "DU9000001")          // r[12..19]
	fields = append(fields, "", "0", "myref", "1", "99999", "0", "0", "0", "")                  // r[20..28]
	// Pad with empty fields: the empty DeltaNeutralOrderType diverges from
	// the live "None"-sentinel walk regardless of total length.
	for len(fields) < 181 {
		fields = append(fields, "")
	}
	payload := wire.EncodeFields(fields)

	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch() error = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("len = %d, want 1", len(msgs))
	}
	oo, ok := msgs[0].(OpenOrder)
	if !ok {
		t.Fatalf("type = %T, want OpenOrder", msgs[0])
	}

	// Pre-variable fields should be correctly parsed.
	if oo.OrderID != 42 {
		t.Errorf("OrderID = %d, want 42", oo.OrderID)
	}
	if oo.Contract.Symbol != "AAPL" {
		t.Errorf("Symbol = %q, want AAPL", oo.Contract.Symbol)
	}
	if oo.Action != "BUY" {
		t.Errorf("Action = %q, want BUY", oo.Action)
	}
	if oo.Quantity != "10" {
		t.Errorf("Quantity = %q, want 10", oo.Quantity)
	}
	if oo.OrderType != "LMT" {
		t.Errorf("OrderType = %q, want LMT", oo.OrderType)
	}
	if oo.LmtPrice != "150.00" {
		t.Errorf("LmtPrice = %q, want 150.00", oo.LmtPrice)
	}
	if oo.Account != "DU9000001" {
		t.Errorf("Account = %q, want DU9000001", oo.Account)
	}
	if oo.OrderRef != "myref" {
		t.Errorf("OrderRef = %q, want myref", oo.OrderRef)
	}

	// Post-variable fields should be zero-valued (partial parse).
	if oo.Status != "" {
		t.Errorf("Status = %q, want empty (partial parse)", oo.Status)
	}
	if oo.ParentID != "" {
		t.Errorf("ParentID = %q, want empty (partial parse)", oo.ParentID)
	}
}

func TestEncodeDecodeOpenOrderAdvancedSections(t *testing.T) {
	t.Parallel()

	payload, err := Encode(200, OpenOrder{
		OrderID: 42,
		Contract: Contract{
			ConID: 265598, Symbol: "AAPL", SecType: "STK",
			Strike: "0", Exchange: "SMART", Currency: "USD",
			LocalSymbol: "AAPL", TradingClass: "NMS",
			ComboLegs: []ComboLeg{{ConID: 1, Ratio: 1, Action: "BUY", Exchange: "SMART", OpenClose: "0", ShortSaleSlot: "0", ExemptCode: "-1"}, {ConID: 2, Ratio: 1, Action: "SELL", Exchange: "SMART", OpenClose: "0", ShortSaleSlot: "0", ExemptCode: "-1"}},
		},
		Account:             "DU9000001",
		Action:              "BUY",
		Quantity:            "1",
		OrderType:           "LMT",
		LmtPrice:            "150.00",
		AuxPrice:            "0",
		TIF:                 "DAY",
		Origin:              "0",
		ClientID:            "1",
		PermID:              "12345",
		OutsideRTH:          "0",
		Hidden:              "0",
		DiscretionAmt:       "0",
		Status:              "PreSubmitted",
		OrderComboLegPrices: []string{"1.25", "2.50"},
		SmartComboRouting:   []TagValue{{Tag: "NonGuaranteed", Value: "1"}},
		AlgoStrategy:        "Adaptive",
		AlgoParams:          []TagValue{{Tag: "adaptivePriority", Value: "Normal"}},
		Conditions:          []OrderCondition{{Type: 1, Conjunction: "a", Operator: 2, ConID: 265598, Exchange: "SMART", Value: "175.0", TriggerMethod: 4}},
		ConditionsIgnoreRTH: "1",
		ParentID:            "0",
	})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch() error = %v", err)
	}
	oo, ok := msgs[0].(OpenOrder)
	if !ok {
		t.Fatalf("type = %T, want OpenOrder", msgs[0])
	}
	if got := len(oo.Contract.ComboLegs); got != 2 {
		t.Fatalf("ComboLegs len = %d, want 2", got)
	}
	if oo.Contract.ComboLegs[0].ConID != 1 || oo.Contract.ComboLegs[1].Action != "SELL" {
		t.Fatalf("decoded combo legs = %#v", oo.Contract.ComboLegs)
	}
	if !slices.Equal(oo.OrderComboLegPrices, []string{"1.25", "2.50"}) {
		t.Fatalf("OrderComboLegPrices = %#v", oo.OrderComboLegPrices)
	}
	if len(oo.SmartComboRouting) != 1 || oo.SmartComboRouting[0].Tag != "NonGuaranteed" {
		t.Fatalf("SmartComboRouting = %#v", oo.SmartComboRouting)
	}
	if oo.AlgoStrategy != "Adaptive" || len(oo.AlgoParams) != 1 || oo.AlgoParams[0].Value != "Normal" {
		t.Fatalf("algo decode = strategy %q params %#v", oo.AlgoStrategy, oo.AlgoParams)
	}
	if len(oo.Conditions) != 1 {
		t.Fatalf("Conditions len = %d, want 1", len(oo.Conditions))
	}
	if cond := oo.Conditions[0]; cond.Type != 1 || cond.Operator != 2 || cond.ConID != 265598 || cond.TriggerMethod != 4 {
		t.Fatalf("Condition = %#v", cond)
	}
	if oo.ConditionsIgnoreRTH != "1" {
		t.Fatalf("ConditionsIgnoreRTH = %q, want 1", oo.ConditionsIgnoreRTH)
	}
	if oo.Status != "PreSubmitted" {
		t.Fatalf("Status = %q, want PreSubmitted", oo.Status)
	}
}

func TestEncodePlaceOrderAdvancedSections(t *testing.T) {
	t.Parallel()

	payload, err := Encode(200, PlaceOrderRequest{
		OrderID: 77,
		Contract: Contract{
			ConID: 9001, Symbol: "BAG-TEST", SecType: "BAG", Exchange: "SMART", Currency: "USD",
			ComboLegs: []ComboLeg{{ConID: 101, Ratio: 1, Action: "BUY", Exchange: "SMART", OpenClose: "0", ShortSaleSlot: "0", DesignatedLocation: "", ExemptCode: "-1"}, {ConID: 102, Ratio: 1, Action: "SELL", Exchange: "SMART", OpenClose: "0", ShortSaleSlot: "0", DesignatedLocation: "", ExemptCode: "-1"}},
		},
		Action:                  "BUY",
		TotalQuantity:           "1",
		OrderType:               "LMT",
		LmtPrice:                "3.50",
		TIF:                     "DAY",
		Account:                 "DU9000001",
		Origin:                  "0",
		Transmit:                "1",
		ParentID:                "0",
		OutsideRTH:              "0",
		OrderComboLegPrices:     []string{"1.10", "2.40"},
		SmartComboRoutingParams: []TagValue{{Tag: "NonGuaranteed", Value: "1"}},
		AlgoStrategy:            "Adaptive",
		AlgoParams:              []TagValue{{Tag: "adaptivePriority", Value: "Patient"}},
		Solicited:               "0",
		RandomizeSize:           "0",
		RandomizePrice:          "0",
		Conditions:              []OrderCondition{{Type: 3, Conjunction: "a", Operator: 2, Value: "20260409 10:00:00 US/Eastern"}},
		ConditionsIgnoreRTH:     "1",
		ConditionsCancelOrder:   "0",
	})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	fields, err := wire.ParseFields(payload)
	if err != nil {
		t.Fatalf("ParseFields() error = %v", err)
	}
	assertSubsequence(t, fields, []string{"2", "101", "1", "BUY", "SMART", "0", "0", "", "-1", "102", "1", "SELL", "SMART", "0", "0", "", "-1", "2", "1.10", "2.40", "1", "NonGuaranteed", "1"})
	assertSubsequence(t, fields, []string{"Adaptive", "1", "adaptivePriority", "Patient"})
	assertSubsequence(t, fields, []string{"0", "0", "1", "3", "a", "1", "20260409 10:00:00 US/Eastern", "1", "0"})
}

func TestEncodeContractConditionValueBeforeContract(t *testing.T) {
	t.Parallel()

	// Live paper Gateway (server_version 200, 2026-06-10) rejected the prior
	// encoding with code 320 "Unable to parse field: 'Con Id' for input
	// string: 'SMART'" for volume and percent-change conditions, and
	// "Unable to parse field: 'Ignore Rth' for input string: '2923.1'" for a
	// price condition (captures/20260610T195846Z-api_conditions_matrix_aapl,
	// events.jsonl sha256 9602919b5a9c8c95; also
	// 20260610T195953Z-place_order_price_condition_aapl, 2c8cb7d7cb7515a2).
	// Contract-bound conditions serialize the OperatorCondition value BEFORE
	// the ContractCondition conId/exchange pair, matching the official
	// client's writeExternal hierarchy.
	cases := []struct {
		name string
		cond OrderCondition
		want []string
	}{
		{
			name: "price",
			cond: OrderCondition{Type: 1, Conjunction: "a", Operator: 2, ConID: 265598, Exchange: "SMART", Value: "9999.00", TriggerMethod: 4},
			want: []string{"1", "1", "a", "1", "9999.00", "265598", "SMART", "4", "1", "0"},
		},
		{
			name: "volume",
			cond: OrderCondition{Type: 6, Conjunction: "a", Operator: 2, ConID: 265598, Exchange: "SMART", Value: "999999999"},
			want: []string{"1", "6", "a", "1", "999999999", "265598", "SMART", "1", "0"},
		},
		{
			name: "percent_change",
			cond: OrderCondition{Type: 7, Conjunction: "a", Operator: 2, ConID: 265598, Exchange: "SMART", Value: "50"},
			want: []string{"1", "7", "a", "1", "50", "265598", "SMART", "1", "0"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			payload, err := Encode(200, PlaceOrderRequest{
				OrderID:               401,
				Contract:              Contract{ConID: 265598, Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"},
				Action:                "BUY",
				TotalQuantity:         "1",
				OrderType:             "LMT",
				LmtPrice:              "50.00",
				TIF:                   "DAY",
				Account:               "DU9000001",
				Origin:                "0",
				Transmit:              "1",
				Conditions:            []OrderCondition{tc.cond},
				ConditionsIgnoreRTH:   "1",
				ConditionsCancelOrder: "0",
			})
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}
			fields, err := wire.ParseFields(payload)
			if err != nil {
				t.Fatalf("ParseFields() error = %v", err)
			}
			assertSubsequence(t, fields, tc.want)
		})
	}
}

func TestEncodeCalcRequestsCarryNoIncludeExpired(t *testing.T) {
	t.Parallel()

	// Live paper Gateway (server_version 200, 2026-06-11) rejected the prior
	// encoding with code 320 "Please use 'Key=Value' format for Misc
	// Options" (captures/20260611T074859Z-api_option_campaign_aapl,
	// events.jsonl sha256 241a49023701e9ec): a phantom includeExpired bool
	// after tradingClass shifted optionPrice/underPrice one slot right, so
	// the Gateway read the numeric underPrice as miscOptions. The official
	// calc requests run [.., tradingClass, price, underPrice, miscOptions].
	contract := Contract{ConID: 886441502, Symbol: "AAPL", SecType: "OPT", Expiry: "20260612", Strike: "292.5", Right: "C", Multiplier: "100", Exchange: "SMART", Currency: "USD", TradingClass: "AAPL"}

	payload, err := Encode(200, CalcImpliedVolatilityRequest{ReqID: 6, Contract: contract, OptionPrice: "5.25", UnderPrice: "292.0"})
	if err != nil {
		t.Fatalf("Encode(200, CalcImpliedVolatilityRequest) error = %v", err)
	}
	fields, err := wire.ParseFields(payload)
	if err != nil {
		t.Fatalf("ParseFields() error = %v", err)
	}
	assertSubsequence(t, fields, []string{"AAPL", "5.25", "292.0", ""})

	payload, err = Encode(200, CalcOptionPriceRequest{ReqID: 7, Contract: contract, Volatility: "0.30", UnderPrice: "292.0"})
	if err != nil {
		t.Fatalf("Encode(200, CalcOptionPriceRequest) error = %v", err)
	}
	fields, err = wire.ParseFields(payload)
	if err != nil {
		t.Fatalf("ParseFields() error = %v", err)
	}
	assertSubsequence(t, fields, []string{"AAPL", "0.30", "292.0", ""})
}

func TestHistoricalClassicContractFieldsStayRequestSpecific(t *testing.T) {
	t.Parallel()

	// API 10.48.01 writes includeExpired immediately after the shared classic
	// contract block for these four families. The MES contract is the same
	// live-derived 202606 expiry used by the exact selector vectors.
	expired := Contract{
		ConID: 770561194, Symbol: "MES", SecType: "FUT", Expiry: "202606",
		Exchange: "CME", Currency: "USD", IncludeExpired: true,
	}
	requests := []struct {
		name string
		msg  Message
	}{
		{"bars", HistoricalBarsRequest{ReqID: 1, Contract: expired, Duration: "1 D", BarSize: "1 day", WhatToShow: "TRADES"}},
		{"head timestamp", HeadTimestampRequest{ReqID: 1, Contract: expired, WhatToShow: "TRADES"}},
		{"histogram", HistogramDataRequest{ReqID: 1, Contract: expired, Period: "1 day"}},
		{"historical ticks", HistoricalTicksRequest{ReqID: 1, Contract: expired, NumberOfTicks: 1, WhatToShow: "TRADES"}},
	}
	for _, tc := range requests {
		t.Run(tc.name, func(t *testing.T) {
			fields, err := tc.msg.encodeWire(200)
			if err != nil {
				t.Fatal(err)
			}
			if len(fields) <= 14 || fields[14] != "1" {
				t.Fatalf("includeExpired position = %#v, want field 14", fields)
			}
		})
	}

	// Historical bars alone append the BAG leg count and four-field legs,
	// after formatDate. These leg IDs come from the live AAPL vertical capture.
	combo := Contract{
		Symbol: "AAPL", SecType: "BAG", Exchange: "SMART", Currency: "USD",
		ComboLegs: []ComboLeg{
			{ConID: 887307502, Ratio: 1, Action: "BUY", Exchange: "SMART"},
			{ConID: 887307536, Ratio: 1, Action: "SELL", Exchange: "SMART"},
		},
	}
	fields, err := (HistoricalBarsRequest{
		ReqID: 2, Contract: combo, Duration: "1 D", BarSize: "1 day", WhatToShow: "TRADES",
	}).encodeWire(200)
	if err != nil {
		t.Fatal(err)
	}
	assertSubsequence(t, fields, []string{
		"TRADES", "1", "2",
		"887307502", "1", "BUY", "SMART",
		"887307536", "1", "SELL", "SMART",
	})
}

func TestEncodeExerciseOptionsTailFields(t *testing.T) {
	t.Parallel()

	// Live paper Gateway (server_version 200, 2026-06-11) rejected the prior
	// encoding with code 10300 "Manual Order Time ... invalid"
	// (captures/20260611T074859Z-api_option_campaign_aapl, events.jsonl
	// sha256 241a49023701e9ec): server_version 200 expects the
	// manualOrderTime, customerAccount, and professionalCustomer tail after
	// override, and the frame ended early.
	payload, err := Encode(200, ExerciseOptionsRequest{
		ReqID:            7,
		Contract:         Contract{ConID: 886441502, Symbol: "AAPL", SecType: "OPT", Expiry: "20260612", Strike: "292.5", Right: "C", Multiplier: "100", Exchange: "SMART", Currency: "USD"},
		ExerciseAction:   2,
		ExerciseQuantity: 1,
		Account:          "DU9000001",
		Override:         0,
	})
	if err != nil {
		t.Fatalf("Encode(200, ExerciseOptionsRequest) error = %v", err)
	}
	fields, err := wire.ParseFields(payload)
	if err != nil {
		t.Fatalf("ParseFields() error = %v", err)
	}
	assertSubsequence(t, fields, []string{"2", "1", "DU9000001", "0", "", "", "0"})
}

func assertSubsequence(t *testing.T, fields, want []string) {
	t.Helper()
	for i := 0; i+len(want) <= len(fields); i++ {
		if slices.Equal(fields[i:i+len(want)], want) {
			return
		}
	}
	t.Fatalf("wire fields do not contain subsequence %#v", want)
}

func TestDecodeServerInfo(t *testing.T) {
	t.Parallel()

	payload := wire.EncodeFields([]string{"200", "20260405 23:49:26 CET"})
	info, err := DecodeServerInfo(payload)
	if err != nil {
		t.Fatalf("DecodeServerInfo() error = %v", err)
	}
	if info.ServerVersion != 200 {
		t.Fatalf("ServerVersion = %d, want 200", info.ServerVersion)
	}
	if info.ConnectionTime != "20260405 23:49:26 CET" {
		t.Fatalf("ConnectionTime = %q, want %q", info.ConnectionTime, "20260405 23:49:26 CET")
	}
}

func TestEncodeStartAPI(t *testing.T) {
	t.Parallel()

	payload, err := Encode(200, StartAPI{ClientID: 1})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	fields, err := wire.ParseFields(payload)
	if err != nil {
		t.Fatalf("ParseFields() error = %v", err)
	}
	if fields[0] != "71" {
		t.Fatalf("msg_id = %q, want 71", fields[0])
	}
	if fields[1] != "2" {
		t.Fatalf("version = %q, want 2", fields[1])
	}
	if fields[2] != "1" {
		t.Fatalf("clientID = %q, want 1", fields[2])
	}
}

func TestEncodeReqMarketDataType(t *testing.T) {
	t.Parallel()

	payload, err := Encode(200, ReqMarketDataType{DataType: 3})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	fields, err := wire.ParseFields(payload)
	if err != nil {
		t.Fatalf("ParseFields() error = %v", err)
	}
	if fields[0] != "59" {
		t.Fatalf("msg_id = %q, want 59", fields[0])
	}
	if fields[1] != "1" {
		t.Fatalf("version = %q, want 1", fields[1])
	}
	if fields[2] != "3" {
		t.Fatalf("dataType = %q, want 3", fields[2])
	}
}

func TestEncodeCancelHistoricalData(t *testing.T) {
	t.Parallel()

	payload, err := Encode(200, CancelHistoricalData{ReqID: 42})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	fields, err := wire.ParseFields(payload)
	if err != nil {
		t.Fatalf("ParseFields() error = %v", err)
	}
	if fields[0] != "25" {
		t.Fatalf("msg_id = %q, want 25", fields[0])
	}
	if fields[1] != "1" {
		t.Fatalf("version = %q, want 1", fields[1])
	}
	if fields[2] != "42" {
		t.Fatalf("reqID = %q, want 42", fields[2])
	}
}

// Regression: missing extOperator and manualOrderIndicator fields caused the
// Gateway to silently drop cancel_order at live server_version 200.
func TestEncodeCancelOrderRequest(t *testing.T) {
	t.Parallel()

	payload, err := Encode(200, CancelOrderRequest{OrderID: 42})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	fields, err := wire.ParseFields(payload)
	if err != nil {
		t.Fatalf("ParseFields() error = %v", err)
	}

	// Wire format: [4, orderID, manualOrderCancelTime, extOperator, manualOrderIndicator]
	if len(fields) != 5 {
		t.Fatalf("field count = %d, want 5; fields = %v", len(fields), fields)
	}
	if fields[0] != "4" {
		t.Fatalf("msg_id = %q, want 4", fields[0])
	}
	if fields[1] != "42" {
		t.Fatalf("orderID = %q, want 42", fields[1])
	}
	if fields[2] != "" {
		t.Fatalf("manualOrderCancelTime = %q, want empty", fields[2])
	}
	if fields[3] != "" {
		t.Fatalf("extOperator = %q, want empty", fields[3])
	}
	if fields[4] != "" {
		t.Fatalf("manualOrderIndicator = %q, want empty", fields[4])
	}
}

func TestEncodeCancelOrderRequestRegulatoryFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		request             CancelOrderRequest
		wantExtOperator     string
		wantManualIndicator string
	}{
		{
			name: "ext operator",
			request: CancelOrderRequest{
				OrderID:     99,
				ExtOperator: "IB",
			},
			wantExtOperator: "IB",
		},
		{
			name: "automated manual order indicator",
			request: CancelOrderRequest{
				OrderID:              100,
				ManualOrderIndicator: "0",
			},
			wantManualIndicator: "0",
		},
		{
			name: "manual order indicator",
			request: CancelOrderRequest{
				OrderID:              101,
				ManualOrderIndicator: "1",
			},
			wantManualIndicator: "1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := Encode(200, tt.request)
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}
			fields, err := wire.ParseFields(payload)
			if err != nil {
				t.Fatalf("ParseFields() error = %v", err)
			}
			if len(fields) != 5 {
				t.Fatalf("field count = %d, want 5", len(fields))
			}
			if fields[3] != tt.wantExtOperator {
				t.Fatalf("extOperator = %q, want %q", fields[3], tt.wantExtOperator)
			}
			if fields[4] != tt.wantManualIndicator {
				t.Fatalf("manualOrderIndicator = %q, want %q", fields[4], tt.wantManualIndicator)
			}
		})
	}
}

func TestEncodeGlobalCancelRequest(t *testing.T) {
	t.Parallel()

	payload, err := Encode(200, GlobalCancelRequest{})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	fields, err := wire.ParseFields(payload)
	if err != nil {
		t.Fatalf("ParseFields() error = %v", err)
	}
	if len(fields) != 3 {
		t.Fatalf("field count = %d, want 3; fields = %v", len(fields), fields)
	}
	if fields[0] != "58" {
		t.Fatalf("msg_id = %q, want 58", fields[0])
	}
	if fields[1] != "" {
		t.Fatalf("extOperator = %q, want empty", fields[1])
	}
	if fields[2] != "" {
		t.Fatalf("manualOrderIndicator = %q, want empty", fields[2])
	}
}

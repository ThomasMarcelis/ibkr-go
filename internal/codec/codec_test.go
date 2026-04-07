package codec

import (
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	t.Parallel()

	// Test messages that have consistent encode/decode via integer msg_id wire format.
	// Only includes types where Encode → DecodeBatch produces the same message.
	tests := []struct {
		msg  Message
		name string
	}{
		{ManagedAccounts{Accounts: []string{"DU12345", "DU67890"}}, "managed_accounts"},
		{NextValidID{OrderID: 1001}, "next_valid_id"},
		{CurrentTime{Time: "1712345678"}, "current_time"},
		{APIError{ReqID: -1, Code: 2104, Message: "Market data farm OK", AdvancedOrderRejectJSON: "", ErrorTimeMs: "1712345678000"}, "api_error"},
		{ContractDetailsEnd{ReqID: 42}, "contract_details_end"},
		{AccountSummaryValue{ReqID: 1, Account: "DU12345", Tag: "NetLiquidation", Value: "100000.00", Currency: "USD"}, "account_summary"},
		{AccountSummaryEnd{ReqID: 1}, "account_summary_end"},
		{TickPrice{ReqID: 1, TickType: 1, Price: "189.10", Size: "400", AttrMask: 0}, "tick_price"},
		{TickSize{ReqID: 1, TickType: 0, Size: "400"}, "tick_size"},
		{MarketDataType{ReqID: 1, DataType: 3}, "market_data_type"},
		{TickSnapshotEnd{ReqID: 1}, "tick_snapshot_end"},
		{RealTimeBar{ReqID: 1, Time: "1712345678", Open: "100.0", High: "101.0", Low: "99.5", Close: "100.5", Volume: "1000", WAP: "100.5", Count: "50"}, "realtime_bar"},
		{CommissionReport{ExecID: "exec-1", Commission: "1.00", Currency: "USD", RealizedPNL: "50.00"}, "commission_report"},
		{TickGeneric{ReqID: 1, TickType: 49, Value: "0"}, "tick_generic"},
		{TickString{ReqID: 1, TickType: 45, Value: "1712300400"}, "tick_string"},
		{TickReqParams{ReqID: 1, MinTick: "0.01", BBOExchange: "SMART", SnapshotPermissions: 3}, "tick_req_params"},
		{ExecutionDetail{ReqID: 1, OrderID: 42, ExecID: "0001", Account: "DU12345", Symbol: "AAPL", Side: "BOT", Shares: "100", Price: "150.50", Time: "20260407 10:30:00"}, "execution_detail"},
		{ExecutionsEnd{ReqID: 1}, "executions_end"},
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
			Filled: "2", Remaining: "8", ParentID: "99",
		}, "open_order"},
		{OpenOrderEnd{}, "open_order_end"},
		{PositionEnd{}, "position_end"},
		{OrderStatus{
			OrderID: 42, Status: "Filled", Filled: "100", Remaining: "0",
			AvgFillPrice: "150.50", PermID: "123456", ParentID: "0",
			LastFillPrice: "150.50", ClientID: "99", WhyHeld: "", MktCapPrice: "0",
		}, "order_status"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload, err := Encode(tt.msg)
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}
			msgs, err := DecodeBatch(payload)
			if err != nil {
				t.Fatalf("DecodeBatch() error = %v", err)
			}
			if len(msgs) == 0 {
				t.Fatal("DecodeBatch() returned 0 messages")
			}
			if msgs[0].messageName() != tt.name {
				t.Fatalf("messageName() = %q, want %q", msgs[0].messageName(), tt.name)
			}
		})
	}
}

func TestDecodeByMsgID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		fields []string
		want   string
	}{
		{"managed_accounts", []string{"15", "1", "DU12345,DU67890"}, "managed_accounts"},
		{"next_valid_id", []string{"9", "1", "1001"}, "next_valid_id"},
		{"current_time", []string{"49", "1", "1712345678"}, "current_time"},
		{"api_error", []string{"4", "-1", "2104", "Market data farm connected", "", "1712345678000"}, "api_error"},
		{"tick_generic", []string{"45", "6", "1", "49", "0"}, "tick_generic"},
		{"tick_string", []string{"46", "6", "1", "45", "1712300400"}, "tick_string"},
		{"tick_req_params", []string{"81", "1", "0.01", "SMART", "3"}, "tick_req_params"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload := wire.EncodeFields(tt.fields)
			msgs, err := DecodeBatch(payload)
			if err != nil {
				t.Fatalf("DecodeBatch() error = %v", err)
			}
			if len(msgs) != 1 {
				t.Fatalf("DecodeBatch() len = %d, want 1", len(msgs))
			}
			if msgs[0].messageName() != tt.want {
				t.Fatalf("messageName() = %q, want %q", msgs[0].messageName(), tt.want)
			}
		})
	}
}

func TestEncodeHeadTimestampRequestFieldOrder(t *testing.T) {
	t.Parallel()

	payload, err := Encode(HeadTimestampRequest{
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
		{"HistoricalData/missing_count", []string{"17", "1"}},
		{"HistoricalData/empty_count", []string{"17", "1", ""}},
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

			_, err := DecodeBatch(wire.EncodeFields(tt.fields))
			if err == nil {
				t.Fatal("DecodeBatch() error = nil, want malformed count error")
			}
		})
	}
}

// TestDecodeOpenOrderCapture decodes the exact bytes from a live v200 IB Gateway
// capture (captures/20260405T215248Z-open_orders_all) and asserts the fields that
// the read-only decoder extracts.
func TestDecodeOpenOrderCapture(t *testing.T) {
	t.Parallel()

	// Payload from the 991-byte frame in the capture, after stripping the 4-byte
	// length prefix. This is the raw null-delimited OpenOrder message (msg_id 5)
	// from server_version 200.
	payload := []byte(
		"5\x000\x00853200900\x00OBDC\x00OPT\x0020261120\x0010\x00P\x00100\x00" +
			"SMART\x00USD\x00OBDC  261120P00010000\x00OBDC\x00" +
			"SELL\x001\x00LMT\x001.2\x000.0\x00GTC\x00\x00DU9000001\x00" +
			"\x000\x00\x000\x001518189976\x000\x000\x000\x00" +
			"\x001518189976.1/DU9000001/100\x00\x00\x00\x00\x00" +
			"\x000\x00\x000\x00\x00-1\x000\x00\x00\x00\x00\x00" +
			"\x002147483647\x000\x000\x000\x00\x003\x000\x000\x00" +
			"\x000\x000\x00\x000\x00None\x00\x000\x00\x00\x00\x00" +
			"?\x000\x000\x00\x000\x000\x00\x00\x00\x00\x00\x000\x000\x000\x00" +
			"2147483647\x002147483647\x00\x00\x000\x00\x00IB\x000\x000\x00" +
			"\x000\x000\x00" +
			"PreSubmitted\x00" +
			"1.7976931348623157E308\x001.7976931348623157E308\x00" +
			"1.7976931348623157E308\x001.7976931348623157E308\x00" +
			"1.7976931348623157E308\x001.7976931348623157E308\x00" +
			"1.7976931348623157E308\x001.7976931348623157E308\x00" +
			"1.7976931348623157E308\x00\x00\x00\x00\x00" +
			"\x001.7976931348623157E308\x001.7976931348623157E308\x00" +
			"1.7976931348623157E308\x001.7976931348623157E308\x00" +
			"1.7976931348623157E308\x001.7976931348623157E308\x00" +
			"1.7976931348623157E308\x001.7976931348623157E308\x00" +
			"1.7976931348623157E308\x00" +
			"-9223372036854775808\x00\x000\x00\x000\x000\x000\x00None\x00" +
			"1.7976931348623157E308\x001.7976931348623157E308\x00" +
			"1.7976931348623157E308\x001.7976931348623157E308\x00" +
			"1.7976931348623157E308\x001.7976931348623157E308\x00" +
			"0\x00\x00\x00\x000\x001\x000\x000\x000\x00\x00\x000\x00\x00\x00\x00\x00\x00" +
			"\x000\x00\x000\x00\x002147483647\x00\x000\x00\x00\x00\x00" +
			"+3\x000\x00PreSubmitted\x000\x001\x000\x001518189976\x000\x000\x000\x00\x000\x00")

	msgs, err := DecodeBatch(payload)
	if err != nil {
		t.Fatalf("DecodeBatch() error = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("DecodeBatch() len = %d, want 1", len(msgs))
	}
	oo, ok := msgs[0].(OpenOrder)
	if !ok {
		t.Fatalf("msg type = %T, want OpenOrder", msgs[0])
	}

	if oo.OrderID != 0 {
		t.Errorf("OrderID = %d, want 0", oo.OrderID)
	}
	if oo.Contract.ConID != 853200900 {
		t.Errorf("ConID = %d, want 853200900", oo.Contract.ConID)
	}
	if oo.Contract.Symbol != "OBDC" {
		t.Errorf("Symbol = %q, want OBDC", oo.Contract.Symbol)
	}
	if oo.Contract.SecType != "OPT" {
		t.Errorf("SecType = %q, want OPT", oo.Contract.SecType)
	}
	if oo.Contract.Expiry != "20261120" {
		t.Errorf("Expiry = %q, want 20261120", oo.Contract.Expiry)
	}
	if oo.Contract.Strike != "10" {
		t.Errorf("Strike = %q, want 10", oo.Contract.Strike)
	}
	if oo.Contract.Right != "P" {
		t.Errorf("Right = %q, want P", oo.Contract.Right)
	}
	if oo.Contract.Multiplier != "100" {
		t.Errorf("Multiplier = %q, want 100", oo.Contract.Multiplier)
	}
	if oo.Contract.Exchange != "SMART" {
		t.Errorf("Exchange = %q, want SMART", oo.Contract.Exchange)
	}
	if oo.Contract.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", oo.Contract.Currency)
	}
	if oo.Contract.LocalSymbol != "OBDC  261120P00010000" {
		t.Errorf("LocalSymbol = %q, want %q", oo.Contract.LocalSymbol, "OBDC  261120P00010000")
	}
	if oo.Contract.TradingClass != "OBDC" {
		t.Errorf("TradingClass = %q, want OBDC", oo.Contract.TradingClass)
	}
	if oo.Action != "SELL" {
		t.Errorf("Action = %q, want SELL", oo.Action)
	}
	if oo.Quantity != "1" {
		t.Errorf("Quantity = %q, want 1", oo.Quantity)
	}
	if oo.OrderType != "LMT" {
		t.Errorf("OrderType = %q, want LMT", oo.OrderType)
	}
	if oo.Account != "DU9000001" {
		t.Errorf("Account = %q, want DU9000001", oo.Account)
	}
	// Newly expanded fields at verified wire positions.
	if oo.LmtPrice != "1.2" {
		t.Errorf("LmtPrice = %q, want 1.2", oo.LmtPrice)
	}
	if oo.AuxPrice != "0.0" {
		t.Errorf("AuxPrice = %q, want 0.0", oo.AuxPrice)
	}
	if oo.TIF != "GTC" {
		t.Errorf("TIF = %q, want GTC", oo.TIF)
	}
	if oo.Origin != "0" {
		t.Errorf("Origin = %q, want 0", oo.Origin)
	}
	if oo.PermID != "1518189976" {
		t.Errorf("PermID = %q, want 1518189976", oo.PermID)
	}
	if oo.Status != "PreSubmitted" {
		t.Errorf("Status = %q, want PreSubmitted", oo.Status)
	}
	// OrderState margin fields (all UNSET in this capture).
	if oo.InitMarginBefore != "1.7976931348623157E308" {
		t.Errorf("InitMarginBefore = %q, want UNSET double", oo.InitMarginBefore)
	}
	if oo.Filled != "0" {
		t.Errorf("Filled = %q, want 0", oo.Filled)
	}
	if oo.Remaining != "1" {
		t.Errorf("Remaining = %q, want 1", oo.Remaining)
	}
}

// TestDecodeOpenOrderNonSimple verifies that an OpenOrder message with a
// field count different from the expected simple-order count (169) produces
// a partial parse with only the reliably-positioned pre-variable-section fields.
func TestDecodeOpenOrderNonSimple(t *testing.T) {
	t.Parallel()

	// Build a synthetic OpenOrder payload with extra fields (simulating a
	// combo order with variable-length sections that expand the message).
	fields := make([]string, 0, 180)
	fields = append(fields, itoa(InOpenOrder))                                                  // msg_id
	fields = append(fields, "42")                                                               // r[0] orderID
	fields = append(fields, "265598", "AAPL", "STK", "", "", "", "", "SMART", "USD", "", "NMS") // r[1..11] contract
	fields = append(fields, "BUY", "10", "LMT", "150.00", "0", "DAY", "", "DU9000001")         // r[12..19]
	fields = append(fields, "", "0", "myref", "1", "99999", "0", "0", "0", "")                  // r[20..28]
	// Pad to 180 fields (more than 169 — simulates variable-length sections).
	for len(fields) < 181 {
		fields = append(fields, "")
	}
	payload := wire.EncodeFields(fields)

	msgs, err := DecodeBatch(payload)
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
	if oo.Filled != "" {
		t.Errorf("Filled = %q, want empty (partial parse)", oo.Filled)
	}
	if oo.Remaining != "" {
		t.Errorf("Remaining = %q, want empty (partial parse)", oo.Remaining)
	}
	if oo.ParentID != "" {
		t.Errorf("ParentID = %q, want empty (partial parse)", oo.ParentID)
	}
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

	payload, err := Encode(StartAPI{ClientID: 1})
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

	payload, err := Encode(ReqMarketDataType{DataType: 3})
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

	payload, err := Encode(CancelHistoricalData{ReqID: 42})
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

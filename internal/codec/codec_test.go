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
		{ExecutionsEnd{ReqID: 1}, "executions_end"},
		{OpenOrder{OrderID: 42, Account: "DU12345", Contract: Contract{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, Action: "BUY", OrderType: "LMT", Status: "Submitted", Quantity: "10", Filled: "2", Remaining: "8"}, "open_order"},
		{OpenOrderEnd{}, "open_order_end"},
		{PositionEnd{}, "position_end"},
		{OrderStatus{OrderID: 42, Status: "Filled", Filled: "100", Remaining: "0"}, "order_status"},
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

// TestDecodeOpenOrderCapture decodes the exact bytes from a live v200 IB Gateway
// capture (captures/20260405T215248Z-open_orders_all) and asserts the fields that
// the v1 read-only decoder extracts.
func TestDecodeOpenOrderCapture(t *testing.T) {
	t.Parallel()

	// Payload from the 991-byte frame in the capture, after stripping the 4-byte
	// length prefix. This is the raw null-delimited OpenOrder message (msg_id 5)
	// from server_version 200.
	payload := []byte(
		"5\x000\x00853200900\x00OBDC\x00OPT\x0020261120\x0010\x00P\x00100\x00" +
			"SMART\x00USD\x00OBDC  261120P00010000\x00OBDC\x00" +
			"SELL\x001\x00LMT\x001.2\x000.0\x00GTC\x00\x00U10597365\x00" +
			"\x000\x00\x000\x001518189976\x000\x000\x000\x00" +
			"\x001518189976.1/U10597365/100\x00\x00\x00\x00\x00" +
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
	if oo.Account != "U10597365" {
		t.Errorf("Account = %q, want U10597365", oo.Account)
	}
	if oo.Status != "PreSubmitted" {
		t.Errorf("Status = %q, want PreSubmitted", oo.Status)
	}
	if oo.Filled != "0" {
		t.Errorf("Filled = %q, want 0", oo.Filled)
	}
	if oo.Remaining != "1" {
		t.Errorf("Remaining = %q, want 1", oo.Remaining)
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

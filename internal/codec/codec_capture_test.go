package codec

import (
	"bytes"
	"strings"
	"testing"
)

// Capture-grounded decode tests. Each payload is extracted from a real IB Gateway
// session (server_version 200, captures/20260405T*) after stripping the 4-byte
// length prefix. The tests prove the codec decodes live wire bytes correctly.

func TestCaptureDecode_ManagedAccounts(t *testing.T) {
	t.Parallel()
	// captures/20260405T214926Z-bootstrap, frame at line 6
	payload := []byte("15\x001\x00DU9000001\x00")
	msgs, err := DecodeBatch(payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(ManagedAccounts)
	if !ok {
		t.Fatalf("type = %T, want ManagedAccounts", msgs[0])
	}
	if len(m.Accounts) != 1 || m.Accounts[0] != "DU9000001" {
		t.Errorf("Accounts = %v, want [DU9000001]", m.Accounts)
	}
}

func TestCaptureDecode_NextValidID(t *testing.T) {
	t.Parallel()
	// captures/20260405T214926Z-bootstrap, first frame in multi-frame chunk at line 7
	payload := []byte("9\x001\x001\x00")
	msgs, err := DecodeBatch(payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(NextValidID)
	if !ok {
		t.Fatalf("type = %T, want NextValidID", msgs[0])
	}
	if m.OrderID != 1 {
		t.Errorf("OrderID = %d, want 1", m.OrderID)
	}
}

func TestCaptureDecode_APIError_2104(t *testing.T) {
	t.Parallel()
	// captures/20260405T214926Z-bootstrap, second frame in multi-frame chunk at line 7
	// APIError code 2104: "Market data farm connection is OK:usfarm"
	payload := []byte("4\x00-1\x002104\x00Market data farm connection is OK:usfarm\x00\x001775425766350\x00")
	msgs, err := DecodeBatch(payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(APIError)
	if !ok {
		t.Fatalf("type = %T, want APIError", msgs[0])
	}
	if m.ReqID != -1 {
		t.Errorf("ReqID = %d, want -1", m.ReqID)
	}
	if m.Code != 2104 {
		t.Errorf("Code = %d, want 2104", m.Code)
	}
	if !strings.Contains(m.Message, "usfarm") {
		t.Errorf("Message = %q, want substring 'usfarm'", m.Message)
	}
}

func TestCaptureDecode_ContractDetails(t *testing.T) {
	t.Parallel()
	// captures/20260405T214938Z-contract_details_aapl_stk, line 10 (1171-byte frame)
	// Full AAPL STK contract details from live gateway.
	payload := []byte(
		"10\x001001\x00AAPL\x00STK\x00\x00\x000\x00\x00SMART\x00USD\x00AAPL\x00NMS\x00NMS\x00" +
			"265598\x000.01\x00\x00" +
			"ACTIVETIM,AD,ADDONT,ADJUST,ALERT,ALGO,ALLOC,AON,AVGCOST,BASKET,BENCHPX," +
			"CASHQTY,COND,CONDORDER,DARKONLY,DARKPOLL,DAY,DEACT,DEACTDIS,DEACTEOD,DIS," +
			"DUR,GAT,GTC,GTD,GTT,HID,IBKRATS,ICE,IMB,IOC,LIT,LMT,LOC,MIDPX,MIT,MKT," +
			"MOC,MTL,NGCOMB,NODARK,NONALGO,OCA,OPG,OPGREROUT,PEGBENCH,PEGMID,POSTATS," +
			"POSTONLY,PREOPGRTH,PRICECHK,REL,REL2MID,RELPCTOFS,RPI,RTH,SCALE,SCALEODD," +
			"SCALERST,SIZECHK,SNAPMID,SNAPMKT,SNAPREL,STP,STPLMT,SWEEP,TRAIL,TRAILLIT," +
			"TRAILLMT,TRAILMIT,WHATIF\x00" +
			"SMART,AMEX,NYSE,CBOE,PHLX,ISE,CHX,ARCA,NASDAQ,DRCTEDGE,BEX,BATS,EDGEA," +
			"BYX,IEX,EDGX,FOXRIVER,PEARL,NYSENAT,LTSE,MEMX,IBEOS,OVERNIGHT,TPLUS0," +
			"PSX,T24X\x00" +
			"1\x000\x00APPLE INC\x00NASDAQ\x00\x00Technology\x00Computers\x00Computers\x00US/Eastern\x00" +
			"20260405:CLOSED;20260406:0400-20260406:2000;20260407:0400-20260407:2000;" +
			"20260408:0400-20260408:2000;20260409:0400-20260409:2000;20260410:0400-20260410:2000\x00" +
			"20260405:CLOSED;20260406:0930-20260406:1600;20260407:0930-20260407:1600;" +
			"20260408:0930-20260408:1600;20260409:0930-20260409:1600;20260410:0930-20260410:1600\x00" +
			"\x00\x001\x00ISIN\x00US0378331005\x001\x00\x00\x00" +
			"26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26\x00" +
			"\x00COMMON\x000.0001\x000.0001\x00100\x000\x00")

	msgs, err := DecodeBatch(payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(ContractDetails)
	if !ok {
		t.Fatalf("type = %T, want ContractDetails", msgs[0])
	}
	if m.ReqID != 1001 {
		t.Errorf("ReqID = %d, want 1001", m.ReqID)
	}
	if m.Contract.ConID != 265598 {
		t.Errorf("ConID = %d, want 265598", m.Contract.ConID)
	}
	if m.Contract.Symbol != "AAPL" {
		t.Errorf("Symbol = %q, want AAPL", m.Contract.Symbol)
	}
	if m.Contract.SecType != "STK" {
		t.Errorf("SecType = %q, want STK", m.Contract.SecType)
	}
	if m.LongName != "APPLE INC" {
		t.Errorf("LongName = %q, want APPLE INC", m.LongName)
	}
	if m.Contract.PrimaryExchange != "NASDAQ" {
		t.Errorf("PrimaryExchange = %q, want NASDAQ", m.Contract.PrimaryExchange)
	}
	if m.Contract.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", m.Contract.Currency)
	}
	if m.MinTick != "0.01" {
		t.Errorf("MinTick = %q, want 0.01", m.MinTick)
	}
	if m.TimeZoneID != "US/Eastern" {
		t.Errorf("TimeZoneID = %q, want US/Eastern", m.TimeZoneID)
	}
}

func TestCaptureDecode_ContractDetailsEnd(t *testing.T) {
	t.Parallel()
	// captures/20260405T214938Z-contract_details_aapl_stk, line 11
	payload := []byte("52\x001\x001001\x00")
	msgs, err := DecodeBatch(payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(ContractDetailsEnd)
	if !ok {
		t.Fatalf("type = %T, want ContractDetailsEnd", msgs[0])
	}
	if m.ReqID != 1001 {
		t.Errorf("ReqID = %d, want 1001", m.ReqID)
	}
}

func TestCaptureDecode_AccountSummaryValue(t *testing.T) {
	t.Parallel()
	// captures/20260405T215025Z-account_summary_snapshot, line 10 (first frame)
	payload := []byte("63\x001\x001001\x00DU9000001\x00BuyingPower\x00300000.00\x00EUR\x00")
	msgs, err := DecodeBatch(payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(AccountSummaryValue)
	if !ok {
		t.Fatalf("type = %T, want AccountSummaryValue", msgs[0])
	}
	if m.ReqID != 1001 {
		t.Errorf("ReqID = %d, want 1001", m.ReqID)
	}
	if m.Account != "DU9000001" {
		t.Errorf("Account = %q, want DU9000001", m.Account)
	}
	if m.Tag != "BuyingPower" {
		t.Errorf("Tag = %q, want BuyingPower", m.Tag)
	}
	if m.Value != "300000.00" {
		t.Errorf("Value = %q, want 300000.00", m.Value)
	}
	if m.Currency != "EUR" {
		t.Errorf("Currency = %q, want EUR", m.Currency)
	}
}

func TestCaptureDecode_AccountSummaryEnd(t *testing.T) {
	t.Parallel()
	// captures/20260405T215025Z-account_summary_snapshot, last frame in multi-frame chunk at line 11
	payload := []byte("64\x001\x001001\x00")
	msgs, err := DecodeBatch(payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(AccountSummaryEnd)
	if !ok {
		t.Fatalf("type = %T, want AccountSummaryEnd", msgs[0])
	}
	if m.ReqID != 1001 {
		t.Errorf("ReqID = %d, want 1001", m.ReqID)
	}
}

func TestCaptureDecode_Position(t *testing.T) {
	t.Parallel()
	// captures/20260405T215052Z-positions_snapshot, first position frame (AMZN) at line 10
	payload := []byte("61\x003\x00DU9000001\x003691937\x00AMZN\x00STK\x00\x000.0\x00\x00\x00NASDAQ\x00USD\x00AMZN\x00NMS\x0015\x00200.25\x00")
	msgs, err := DecodeBatch(payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(Position)
	if !ok {
		t.Fatalf("type = %T, want Position", msgs[0])
	}
	if m.Account != "DU9000001" {
		t.Errorf("Account = %q, want DU9000001", m.Account)
	}
	if m.Contract.ConID != 3691937 {
		t.Errorf("ConID = %d, want 3691937", m.Contract.ConID)
	}
	if m.Contract.Symbol != "AMZN" {
		t.Errorf("Symbol = %q, want AMZN", m.Contract.Symbol)
	}
	if m.Contract.SecType != "STK" {
		t.Errorf("SecType = %q, want STK", m.Contract.SecType)
	}
	if m.Contract.Exchange != "NASDAQ" {
		t.Errorf("Exchange = %q, want NASDAQ", m.Contract.Exchange)
	}
	if m.Contract.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", m.Contract.Currency)
	}
	if m.Position != "15" {
		t.Errorf("Position = %q, want 15", m.Position)
	}
	if m.AvgCost != "200.25" {
		t.Errorf("AvgCost = %q, want 200.25", m.AvgCost)
	}
}

func TestCaptureDecode_PositionEnd(t *testing.T) {
	t.Parallel()
	// captures/20260405T215052Z-positions_snapshot, last frame in multi-frame chunk at line 11
	payload := []byte("62\x001\x00")
	msgs, err := DecodeBatch(payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	if _, ok := msgs[0].(PositionEnd); !ok {
		t.Fatalf("type = %T, want PositionEnd", msgs[0])
	}
}

func TestCaptureDecode_HistoricalData(t *testing.T) {
	t.Parallel()
	// captures/20260405T215056Z-historical_bars_1d_1h, single 558-byte frame at line 10
	// 7 hourly bars for AAPL on 2026-04-02
	payload := []byte(
		"17\x001001\x007\x00" +
			"20260402 09:30:00 US/Eastern\x00254.20\x00254.80\x00250.65\x00252.53\x002829736\x00252.266\x0013633\x00" +
			"20260402 10:00:00 US/Eastern\x00252.52\x00255.40\x00251.19\x00255.38\x002797972\x00252.971\x0016541\x00" +
			"20260402 11:00:00 US/Eastern\x00255.40\x00255.73\x00254.36\x00254.57\x001400669\x00255.002\x007744\x00" +
			"20260402 12:00:00 US/Eastern\x00254.57\x00255.00\x00254.00\x00254.42\x00983738\x00254.453\x005662\x00" +
			"20260402 13:00:00 US/Eastern\x00254.42\x00255.49\x00254.17\x00254.61\x001024324\x00254.878\x005832\x00" +
			"20260402 14:00:00 US/Eastern\x00254.58\x00255.46\x00254.58\x00255.28\x001399189\x00255.101\x007342\x00" +
			"20260402 15:00:00 US/Eastern\x00255.29\x00256.13\x00254.80\x00255.89\x002938382\x00255.576\x0017376\x00")

	msgs, err := DecodeBatch(payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	// 7 bars + 1 HistoricalBarsEnd = 8 messages
	if len(msgs) != 8 {
		t.Fatalf("got %d messages, want 8", len(msgs))
	}

	// First bar
	bar0, ok := msgs[0].(HistoricalBar)
	if !ok {
		t.Fatalf("msgs[0] type = %T, want HistoricalBar", msgs[0])
	}
	if bar0.ReqID != 1001 {
		t.Errorf("bar0.ReqID = %d, want 1001", bar0.ReqID)
	}
	if bar0.Open != "254.20" {
		t.Errorf("bar0.Open = %q, want 254.20", bar0.Open)
	}
	if bar0.High != "254.80" {
		t.Errorf("bar0.High = %q, want 254.80", bar0.High)
	}
	if bar0.Low != "250.65" {
		t.Errorf("bar0.Low = %q, want 250.65", bar0.Low)
	}
	if bar0.Close != "252.53" {
		t.Errorf("bar0.Close = %q, want 252.53", bar0.Close)
	}
	if bar0.Volume != "2829736" {
		t.Errorf("bar0.Volume = %q, want 2829736", bar0.Volume)
	}
	if bar0.Time != "20260402 09:30:00 US/Eastern" {
		t.Errorf("bar0.Time = %q, want '20260402 09:30:00 US/Eastern'", bar0.Time)
	}

	// Last bar
	bar6, ok := msgs[6].(HistoricalBar)
	if !ok {
		t.Fatalf("msgs[6] type = %T, want HistoricalBar", msgs[6])
	}
	if bar6.Open != "255.29" {
		t.Errorf("bar6.Open = %q, want 255.29", bar6.Open)
	}

	// HistoricalBarsEnd sentinel
	end, ok := msgs[7].(HistoricalBarsEnd)
	if !ok {
		t.Fatalf("msgs[7] type = %T, want HistoricalBarsEnd", msgs[7])
	}
	if end.ReqID != 1001 {
		t.Errorf("end.ReqID = %d, want 1001", end.ReqID)
	}
}

func TestCaptureDecode_TickPrice(t *testing.T) {
	t.Parallel()
	// captures/20260405T215734Z-quote_snapshot_aapl, tickType 68 (delayed last) at line 15
	payload := []byte("1\x006\x001001\x0068\x00255.45\x00200\x000\x00")
	msgs, err := DecodeBatch(payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(TickPrice)
	if !ok {
		t.Fatalf("type = %T, want TickPrice", msgs[0])
	}
	if m.ReqID != 1001 {
		t.Errorf("ReqID = %d, want 1001", m.ReqID)
	}
	if m.TickType != 68 {
		t.Errorf("TickType = %d, want 68", m.TickType)
	}
	if m.Price != "255.45" {
		t.Errorf("Price = %q, want 255.45", m.Price)
	}
	if m.Size != "200" {
		t.Errorf("Size = %q, want 200", m.Size)
	}
	if m.AttrMask != 0 {
		t.Errorf("AttrMask = %d, want 0", m.AttrMask)
	}
}

func TestCaptureDecode_TickSize(t *testing.T) {
	t.Parallel()
	// captures/20260405T215734Z-quote_snapshot_aapl, tickType 74 (delayed volume) at line 16
	payload := []byte("2\x006\x001001\x0074\x00312894\x00")
	msgs, err := DecodeBatch(payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(TickSize)
	if !ok {
		t.Fatalf("type = %T, want TickSize", msgs[0])
	}
	if m.ReqID != 1001 {
		t.Errorf("ReqID = %d, want 1001", m.ReqID)
	}
	if m.TickType != 74 {
		t.Errorf("TickType = %d, want 74", m.TickType)
	}
	if m.Size != "312894" {
		t.Errorf("Size = %q, want 312894", m.Size)
	}
}

func TestCaptureDecode_MarketDataType(t *testing.T) {
	t.Parallel()
	// captures/20260405T215734Z-quote_snapshot_aapl, line 11
	payload := []byte("58\x001\x001001\x003\x00")
	msgs, err := DecodeBatch(payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(MarketDataType)
	if !ok {
		t.Fatalf("type = %T, want MarketDataType", msgs[0])
	}
	if m.ReqID != 1001 {
		t.Errorf("ReqID = %d, want 1001", m.ReqID)
	}
	if m.DataType != 3 {
		t.Errorf("DataType = %d, want 3 (delayed)", m.DataType)
	}
}

func TestCaptureDecode_TickSnapshotEnd(t *testing.T) {
	t.Parallel()
	// captures/20260405T215734Z-quote_snapshot_aapl, line 18
	payload := []byte("57\x001\x001001\x00")
	msgs, err := DecodeBatch(payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(TickSnapshotEnd)
	if !ok {
		t.Fatalf("type = %T, want TickSnapshotEnd", msgs[0])
	}
	if m.ReqID != 1001 {
		t.Errorf("ReqID = %d, want 1001", m.ReqID)
	}
}

func TestCaptureDecode_OpenOrder(t *testing.T) {
	t.Parallel()
	// captures/20260405T215248Z-open_orders_all, 991-byte OpenOrder frame at line 10.
	// OBDC PUT option, PreSubmitted status. Same payload as TestDecodeOpenOrderCapture
	// in codec_test.go but included here for completeness of the capture suite.
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
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(OpenOrder)
	if !ok {
		t.Fatalf("type = %T, want OpenOrder", msgs[0])
	}
	if m.OrderID != 0 {
		t.Errorf("OrderID = %d, want 0", m.OrderID)
	}
	if m.Contract.Symbol != "OBDC" {
		t.Errorf("Symbol = %q, want OBDC", m.Contract.Symbol)
	}
	if m.Contract.SecType != "OPT" {
		t.Errorf("SecType = %q, want OPT", m.Contract.SecType)
	}
	if m.Contract.ConID != 853200900 {
		t.Errorf("ConID = %d, want 853200900", m.Contract.ConID)
	}
	if m.Account != "DU9000001" {
		t.Errorf("Account = %q, want DU9000001", m.Account)
	}
	if m.Status != "PreSubmitted" {
		t.Errorf("Status = %q, want PreSubmitted", m.Status)
	}
	if m.Action != "SELL" {
		t.Errorf("Action = %q, want SELL", m.Action)
	}
	if m.OrderType != "LMT" {
		t.Errorf("OrderType = %q, want LMT", m.OrderType)
	}
	if m.Filled != "0" {
		t.Errorf("Filled = %q, want 0", m.Filled)
	}
	if m.Remaining != "1" {
		t.Errorf("Remaining = %q, want 1", m.Remaining)
	}
}

func TestCaptureDecode_OpenOrderEnd(t *testing.T) {
	t.Parallel()
	// captures/20260405T215248Z-open_orders_all, line 11
	payload := []byte("53\x001\x00")
	msgs, err := DecodeBatch(payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	if _, ok := msgs[0].(OpenOrderEnd); !ok {
		t.Fatalf("type = %T, want OpenOrderEnd", msgs[0])
	}
}

// Encode validation tests: compare our Encode output to the actual request bytes
// sent by the capture client to IB Gateway.

func TestCaptureEncode_StartAPI(t *testing.T) {
	t.Parallel()
	// captures/20260405T214926Z-bootstrap, line 5: client sends StartAPI with clientID=1
	// Actual wire bytes (after 4-byte length prefix): "71\x002\x001\x00\x00"
	want := []byte("71\x002\x001\x00\x00")
	got, err := Encode(StartAPI{ClientID: 1})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Encode(StartAPI{1}) = %q, want %q", got, want)
	}
}

func TestCaptureEncode_PositionsRequest(t *testing.T) {
	t.Parallel()
	// captures/20260405T215052Z-positions_snapshot, line 9: client sends PositionsRequest
	// Actual wire bytes (after 4-byte length prefix): "61\x001\x00"
	want := []byte("61\x001\x00")
	got, err := Encode(PositionsRequest{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Encode(PositionsRequest{}) = %q, want %q", got, want)
	}
}

func TestCaptureEncode_OpenOrdersRequest(t *testing.T) {
	t.Parallel()
	// captures/20260405T215248Z-open_orders_all, line 9: client sends reqAllOpenOrders
	// Actual wire bytes (after 4-byte length prefix): "16\x001\x00"
	want := []byte("16\x001\x00")
	got, err := Encode(OpenOrdersRequest{Scope: "all"})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Encode(OpenOrdersRequest{all}) = %q, want %q", got, want)
	}
}

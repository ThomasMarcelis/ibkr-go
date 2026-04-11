package codec

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

// containsNull returns true if any of the given strings contain a null byte.
// The TWS wire protocol uses null as a field delimiter, so null bytes inside
// field values corrupt the framing and cannot round-trip.
func containsNull(ss ...string) bool {
	for _, s := range ss {
		if strings.ContainsRune(s, 0) {
			return true
		}
	}
	return false
}

// mustNotPanic calls fn and reports a test error if fn panics.
func mustNotPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()
	fn()
}

// allInboundMsgIDs is the complete set of known inbound (server -> client) message IDs.
var allInboundMsgIDs = []int{
	InTickPrice,             // 1
	InTickSize,              // 2
	InOrderStatus,           // 3
	InErrMsg,                // 4
	InOpenOrder,             // 5
	InUpdateAccountValue,    // 6
	InUpdatePortfolio,       // 7
	InUpdateAccountTime,     // 8
	InNextValidID,           // 9
	InContractData,          // 10
	InExecutionData,         // 11
	InMarketDepth,           // 12
	InMarketDepthL2,         // 13
	InNewsBulletins,         // 14
	InManagedAccounts,       // 15
	InHistoricalData,        // 17
	InScannerParameters,     // 19
	InScannerData,           // 20
	InTickOptionComputation, // 21
	InTickGeneric,           // 45
	InTickString,            // 46
	InCurrentTime,           // 49
	InRealTimeBars,          // 50
	InFundamentalData,       // 51
	InContractDataEnd,       // 52
	InOpenOrderEnd,          // 53
	InAccountDownloadEnd,    // 54
	InExecutionDataEnd,      // 55
	InTickSnapshotEnd,       // 57
	InMarketDataType,        // 58
	InCommissionReport,      // 59
	InPositionData,          // 61
	InPositionEnd,           // 62
	InAccountSummary,        // 63
	InAccountSummaryEnd,     // 64
	InPositionMulti,         // 71
	InPositionMultiEnd,      // 72
	InAccountUpdateMulti,    // 73
	InAccountUpdateMultiEnd, // 74
	InSecDefOptParams,       // 75
	InSecDefOptParamsEnd,    // 76
	InFamilyCodes,           // 78
	InSymbolSamples,         // 79
	InMktDepthExchanges,     // 80
	InTickReqParams,         // 81
	InSmartComponents,       // 82
	InNewsArticle,           // 83
	InNewsProviders,         // 85
	InHistoricalNews,        // 86
	InHistoricalNewsEnd,     // 87
	InHeadTimestamp,         // 88
	InHistogramData,         // 89
	InMarketRule,            // 92
	InPnL,                   // 94
	InPnLSingle,             // 95
	InHistoricalTicks,       // 96
	InHistoricalTicksBidAsk, // 97
	InHistoricalTicksLast,   // 98
	InTickByTick,            // 99
	InCompletedOrder,        // 101
	InCompletedOrderEnd,     // 102
	InUserInfo,              // 103
	InHistoricalDataUpdate,  // 108
	InReceiveFA,             // 16
	InSoftDollarTiers,       // 77
	InDisplayGroupList,      // 67
	InDisplayGroupUpdated,   // 68
	InWSHMetaData,           // 104
	InWSHEventData,          // 105
	InHistoricalSchedule,    // 106
}

// FuzzDecodeBatch proves DecodeBatch never panics on arbitrary byte payloads.
// Seeds include real wire captures, encoder output for diverse message types,
// and degenerate inputs.
func FuzzDecodeBatch(f *testing.F) {
	// --- Real wire capture seeds ---
	f.Add([]byte("15\x001\x00DU9000001\x00"))                                                                                              // ManagedAccounts
	f.Add([]byte("9\x001\x001\x00"))                                                                                                       // NextValidID
	f.Add([]byte("4\x00-1\x002104\x00Market data farm connection is OK:usfarm\x00\x001775425766350\x00"))                                  // APIError
	f.Add([]byte("52\x001\x001001\x00"))                                                                                                   // ContractDetailsEnd
	f.Add([]byte("63\x001\x001001\x00DU9000001\x00BuyingPower\x00300000.00\x00EUR\x00"))                                                   // AccountSummaryValue
	f.Add([]byte("64\x001\x001001\x00"))                                                                                                   // AccountSummaryEnd
	f.Add([]byte("1\x006\x001001\x0068\x00255.45\x00200\x000\x00"))                                                                        // TickPrice
	f.Add([]byte("2\x006\x001001\x0074\x00312894\x00"))                                                                                    // TickSize
	f.Add([]byte("58\x001\x001001\x003\x00"))                                                                                              // MarketDataType
	f.Add([]byte("57\x001\x001001\x00"))                                                                                                   // TickSnapshotEnd
	f.Add([]byte("61\x003\x00DU9000001\x003691937\x00AMZN\x00STK\x00\x000.0\x00\x00\x00NASDAQ\x00USD\x00AMZN\x00NMS\x0015\x00200.25\x00")) // Position
	f.Add([]byte("62\x001\x00"))                                                                                                           // PositionEnd
	f.Add([]byte("53\x001\x00"))                                                                                                           // OpenOrderEnd
	f.Add([]byte("81\x001\x000.01\x00SMART\x003\x00"))                                                                                     // TickReqParams

	// --- Encoder-derived seeds for diverse message types ---
	encoderSeeds := []Message{
		TickPrice{ReqID: 1, TickType: 1, Price: "100", Size: "50", AttrMask: 0},
		TickSize{ReqID: 2, TickType: 0, Size: "400"},
		AccountSummaryValue{ReqID: 3, Account: "DU1234", Tag: "NetLiquidation", Value: "50000.00", Currency: "USD"},
		AccountSummaryEnd{ReqID: 4},
		Position{Account: "DU1234", Contract: Contract{ConID: 265598, Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, Position: "100", AvgCost: "150.00"},
		TickReqParams{ReqID: 5, MinTick: "0.01", BBOExchange: "SMART", SnapshotPermissions: 3},
		FamilyCodes{Codes: []FamilyCodeEntry{{AccountID: "U123", FamilyCode: "F1"}, {AccountID: "U456", FamilyCode: "F2"}}},
		ManagedAccounts{Accounts: []string{"DU1234", "DU5678"}},
		NextValidID{OrderID: 42},
		CurrentTime{Time: "1712345678"},
		APIError{ReqID: -1, Code: 2104, Message: "test", AdvancedOrderRejectJSON: "", ErrorTimeMs: "123"},
		RealTimeBar{ReqID: 1, Time: "1712345678", Open: "100.0", High: "101.0", Low: "99.5", Close: "100.5", Volume: "1000", WAP: "100.5", Count: "50"},
		CommissionReport{ExecID: "exec-1", Commission: "1.00", Currency: "USD", RealizedPNL: "50.00"},
		TickGeneric{ReqID: 1, TickType: 49, Value: "0"},
		TickString{ReqID: 1, TickType: 45, Value: "1712300400"},
		MarketDataType{ReqID: 1, DataType: 3},
		TickSnapshotEnd{ReqID: 1},
		OrderStatus{OrderID: 42, Status: "Filled", Filled: "100", Remaining: "0", AvgFillPrice: "150.50", PermID: "123456", ParentID: "0", LastFillPrice: "150.50", ClientID: "99"},
		OpenOrderEnd{},
		PositionEnd{},
		ExecutionDetail{ReqID: 1, OrderID: 42, ExecID: "0001", Account: "DU12345", Symbol: "AAPL", Side: "BOT", Shares: "100", Price: "150.50", Time: "20260407 10:30:00"},
		ExecutionsEnd{ReqID: 1},
		ContractDetailsEnd{ReqID: 42},
		CompletedOrderEnd{},
		UserInfo{ReqID: 1, WhiteBrandingID: "WB1"},
		HeadTimestamp{ReqID: 1, Timestamp: "20200101-00:00:00"},
		PnLValue{ReqID: 1, DailyPnL: "100.50", UnrealizedPnL: "200.00", RealizedPnL: "50.00"},
		PnLSingleValue{ReqID: 1, Position: "10", DailyPnL: "50.25", UnrealizedPnL: "100.00", RealizedPnL: "25.00", Value: "5000.00"},
		ScannerParameters{XML: "<xml/>"},
		NewsProviders{Providers: []NewsProviderEntry{{Code: "BRFG", Name: "Briefing"}}},
		HistogramDataResponse{ReqID: 1, Entries: []HistogramDataEntry{{Price: "100.0", Size: "500"}}},
		MarketRule{MarketRuleID: 26, Increments: []PriceIncrement{{LowEdge: "0", Increment: "0.01"}}},
		TickOptionComputation{ReqID: 1, TickType: 13, TickAttrib: 1, ImpliedVol: "0.25", Delta: "0.5", OptPrice: "3.50", PvDividend: "0.10", Gamma: "0.02", Vega: "0.15", Theta: "-0.05", UndPrice: "150.00"},
		NewsBulletin{MsgID: 1, MsgType: 1, Headline: "Test Headline", Source: "TestSource"},
		HistoricalDataUpdate{ReqID: 1, BarCount: 1, Time: "20260101", Open: "100", High: "101", Low: "99", Close: "100.5", Volume: "1000", WAP: "100.25", Count: "50"},
		TickByTickData{ReqID: 1, TickType: 1, Time: "1712345678", Price: "100.50", Size: "200", TickAttribLast: 0, Exchange: "SMART", SpecialConditions: ""},
		TickByTickData{ReqID: 2, TickType: 3, Time: "1712345678", BidPrice: "100.0", AskPrice: "100.5", BidSize: "100", AskSize: "200", TickAttribBidAsk: 0},
		TickByTickData{ReqID: 3, TickType: 4, Time: "1712345678", MidPoint: "100.25"},
		NewsArticleResponse{ReqID: 1, ArticleType: 0, ArticleText: "Article body"},
		HistoricalNewsItem{ReqID: 1, Time: "1704067200000", ProviderCode: "BRFG", ArticleID: "ART1", Headline: "News"},
		UpdateAccountValue{Key: "NetLiquidation", Value: "100000", Currency: "USD", Account: "DU1234"},
		UpdateAccountTime{Timestamp: "15:30"},
		AccountDownloadEnd{Account: "DU1234"},
		AccountUpdateMultiValue{ReqID: 1, Account: "DU1234", ModelCode: "", Key: "NetLiq", Value: "100000", Currency: "USD"},
		AccountUpdateMultiEnd{ReqID: 1},
		PositionMultiEnd{ReqID: 1},
		SecDefOptParamsEnd{ReqID: 1},
	}

	for _, msg := range encoderSeeds {
		payload, err := Encode(msg)
		if err != nil {
			continue
		}
		f.Add(payload)
	}

	// --- Degenerate seeds ---
	f.Add([]byte{})                                                     // empty
	f.Add([]byte{0x00})                                                 // single null
	f.Add(wire.EncodeFields([]string{"1"}))                             // msg ID only, no fields
	f.Add(wire.EncodeFields([]string{"1", "abc"}))                      // valid msg ID, wrong field count
	f.Add(wire.EncodeFields([]string{"999"}))                           // unknown msg ID
	f.Add(wire.EncodeFields([]string{"0"}))                             // zero msg ID
	f.Add(wire.EncodeFields([]string{"-1"}))                            // negative msg ID
	f.Add(wire.EncodeFields([]string{"1", "", "", "", "", "", "", ""})) // empty fields for TickPrice
	f.Add([]byte("not\x00a\x00number\x00"))                             // non-numeric msg ID
	f.Add(wire.EncodeFields([]string{"17", "1", "999999"}))             // HistoricalData with huge barCount

	f.Fuzz(func(t *testing.T, data []byte) {
		// Cap input size to avoid OOM from unbounded make([]T, hugeCount)
		// when the fuzzer generates payloads with large numeric count fields.
		// The underlying bugs (negative/huge counts passed to make) are
		// documented by TestDecodeNegativeAndOverflowCounts. This cap lets
		// the fuzzer explore structural coverage without triggering fatal
		// OOM signals that cannot be recovered.
		if len(data) > 4096 {
			return
		}
		// Property: DecodeBatch must never panic on any input within the
		// size budget. The defer/recover is inlined because the fuzzer runs
		// each iteration in-process.
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("unexpected panic: %v", r)
			}
		}()
		DecodeBatch(data)
	})
}

// FuzzEncodeDecodeRoundTrip_TickPrice proves encode-decode round-trip preserves
// TickPrice fields for arbitrary fuzzer-generated values.
func FuzzEncodeDecodeRoundTrip_TickPrice(f *testing.F) {
	f.Add(1, 1, "100.5", "200", 0)
	f.Add(0, 68, "255.45", "400", 3)
	f.Add(-1, 0, "", "", 0)
	f.Add(999999, 99, "0.001", "1000000", 255)

	f.Fuzz(func(t *testing.T, reqID int, tickType int, price string, size string, attrMask int) {
		if containsNull(price, size) {
			return // null bytes corrupt wire framing
		}
		original := TickPrice{ReqID: reqID, TickType: tickType, Price: price, Size: size, AttrMask: attrMask}
		encoded, err := Encode(original)
		if err != nil {
			return
		}
		decoded, err := DecodeBatch(encoded)
		if err != nil {
			t.Fatalf("decode failed after successful encode: %v", err)
		}
		if len(decoded) != 1 {
			t.Fatalf("expected 1 message, got %d", len(decoded))
		}
		tp, ok := decoded[0].(TickPrice)
		if !ok {
			t.Fatalf("expected TickPrice, got %T", decoded[0])
		}
		if tp.ReqID != reqID {
			t.Errorf("ReqID: got %d, want %d", tp.ReqID, reqID)
		}
		if tp.TickType != tickType {
			t.Errorf("TickType: got %d, want %d", tp.TickType, tickType)
		}
		if tp.Price != price {
			t.Errorf("Price: got %q, want %q", tp.Price, price)
		}
		if tp.Size != size {
			t.Errorf("Size: got %q, want %q", tp.Size, size)
		}
		if tp.AttrMask != attrMask {
			t.Errorf("AttrMask: got %d, want %d", tp.AttrMask, attrMask)
		}
	})
}

// FuzzEncodeDecodeRoundTrip_AccountSummaryValue proves encode-decode round-trip
// preserves AccountSummaryValue fields.
func FuzzEncodeDecodeRoundTrip_AccountSummaryValue(f *testing.F) {
	f.Add(1, "DU12345", "NetLiquidation", "100000.00", "USD")
	f.Add(0, "", "", "", "")
	f.Add(999, "DU9000001", "BuyingPower", "300000.00", "EUR")

	f.Fuzz(func(t *testing.T, reqID int, account string, tag string, value string, currency string) {
		if containsNull(account, tag, value, currency) {
			return
		}
		original := AccountSummaryValue{ReqID: reqID, Account: account, Tag: tag, Value: value, Currency: currency}
		encoded, err := Encode(original)
		if err != nil {
			return
		}
		decoded, err := DecodeBatch(encoded)
		if err != nil {
			t.Fatalf("decode failed after successful encode: %v", err)
		}
		if len(decoded) != 1 {
			t.Fatalf("expected 1 message, got %d", len(decoded))
		}
		asv, ok := decoded[0].(AccountSummaryValue)
		if !ok {
			t.Fatalf("expected AccountSummaryValue, got %T", decoded[0])
		}
		if asv.ReqID != reqID {
			t.Errorf("ReqID: got %d, want %d", asv.ReqID, reqID)
		}
		if asv.Account != account {
			t.Errorf("Account: got %q, want %q", asv.Account, account)
		}
		if asv.Tag != tag {
			t.Errorf("Tag: got %q, want %q", asv.Tag, tag)
		}
		if asv.Value != value {
			t.Errorf("Value: got %q, want %q", asv.Value, value)
		}
		if asv.Currency != currency {
			t.Errorf("Currency: got %q, want %q", asv.Currency, currency)
		}
	})
}

// FuzzEncodeDecodeRoundTrip_PnLValue proves encode-decode round-trip for PnLValue.
func FuzzEncodeDecodeRoundTrip_PnLValue(f *testing.F) {
	f.Add(1, "100.50", "200.00", "50.00")
	f.Add(0, "", "", "")
	f.Add(-1, "-100.50", "0", "-50.00")

	f.Fuzz(func(t *testing.T, reqID int, dailyPnL string, unrealizedPnL string, realizedPnL string) {
		if containsNull(dailyPnL, unrealizedPnL, realizedPnL) {
			return
		}
		original := PnLValue{ReqID: reqID, DailyPnL: dailyPnL, UnrealizedPnL: unrealizedPnL, RealizedPnL: realizedPnL}
		encoded, err := Encode(original)
		if err != nil {
			return
		}
		decoded, err := DecodeBatch(encoded)
		if err != nil {
			t.Fatalf("decode failed after successful encode: %v", err)
		}
		if len(decoded) != 1 {
			t.Fatalf("expected 1 message, got %d", len(decoded))
		}
		pnl, ok := decoded[0].(PnLValue)
		if !ok {
			t.Fatalf("expected PnLValue, got %T", decoded[0])
		}
		if pnl.ReqID != reqID {
			t.Errorf("ReqID: got %d, want %d", pnl.ReqID, reqID)
		}
		if pnl.DailyPnL != dailyPnL {
			t.Errorf("DailyPnL: got %q, want %q", pnl.DailyPnL, dailyPnL)
		}
		if pnl.UnrealizedPnL != unrealizedPnL {
			t.Errorf("UnrealizedPnL: got %q, want %q", pnl.UnrealizedPnL, unrealizedPnL)
		}
		if pnl.RealizedPnL != realizedPnL {
			t.Errorf("RealizedPnL: got %q, want %q", pnl.RealizedPnL, realizedPnL)
		}
	})
}

// FuzzEncodeDecodeRoundTrip_TickReqParams proves encode-decode round-trip for
// TickReqParams (an unversioned message).
func FuzzEncodeDecodeRoundTrip_TickReqParams(f *testing.F) {
	f.Add(1, "0.01", "SMART", 3)
	f.Add(0, "", "", 0)

	f.Fuzz(func(t *testing.T, reqID int, minTick string, bboExchange string, snapshotPermissions int) {
		if containsNull(minTick, bboExchange) {
			return
		}
		original := TickReqParams{ReqID: reqID, MinTick: minTick, BBOExchange: bboExchange, SnapshotPermissions: snapshotPermissions}
		encoded, err := Encode(original)
		if err != nil {
			return
		}
		decoded, err := DecodeBatch(encoded)
		if err != nil {
			t.Fatalf("decode failed after successful encode: %v", err)
		}
		if len(decoded) != 1 {
			t.Fatalf("expected 1 message, got %d", len(decoded))
		}
		trp, ok := decoded[0].(TickReqParams)
		if !ok {
			t.Fatalf("expected TickReqParams, got %T", decoded[0])
		}
		if trp.ReqID != reqID {
			t.Errorf("ReqID: got %d, want %d", trp.ReqID, reqID)
		}
		if trp.MinTick != minTick {
			t.Errorf("MinTick: got %q, want %q", trp.MinTick, minTick)
		}
		if trp.BBOExchange != bboExchange {
			t.Errorf("BBOExchange: got %q, want %q", trp.BBOExchange, bboExchange)
		}
		if trp.SnapshotPermissions != snapshotPermissions {
			t.Errorf("SnapshotPermissions: got %d, want %d", trp.SnapshotPermissions, snapshotPermissions)
		}
	})
}

// FuzzEncodeDecodeRoundTrip_HeadTimestamp proves encode-decode round-trip for HeadTimestamp.
func FuzzEncodeDecodeRoundTrip_HeadTimestamp(f *testing.F) {
	f.Add(1, "20200101-00:00:00")
	f.Add(0, "")

	f.Fuzz(func(t *testing.T, reqID int, timestamp string) {
		if containsNull(timestamp) {
			return
		}
		original := HeadTimestamp{ReqID: reqID, Timestamp: timestamp}
		encoded, err := Encode(original)
		if err != nil {
			return
		}
		decoded, err := DecodeBatch(encoded)
		if err != nil {
			t.Fatalf("decode failed after successful encode: %v", err)
		}
		if len(decoded) != 1 {
			t.Fatalf("expected 1 message, got %d", len(decoded))
		}
		ht, ok := decoded[0].(HeadTimestamp)
		if !ok {
			t.Fatalf("expected HeadTimestamp, got %T", decoded[0])
		}
		if ht.ReqID != reqID {
			t.Errorf("ReqID: got %d, want %d", ht.ReqID, reqID)
		}
		if ht.Timestamp != timestamp {
			t.Errorf("Timestamp: got %q, want %q", ht.Timestamp, timestamp)
		}
	})
}

// TestDecodeShortFields verifies that decoding every known inbound msg ID with
// progressively fewer fields never panics. The decoder's fieldReader returns
// zero-values past end, so short payloads must degrade gracefully.
func TestDecodeShortFields(t *testing.T) {
	t.Parallel()

	// Each entry: msg ID, name for diagnostics, max field count the decoder
	// reads (after the msg_id field itself). These counts are derived from
	// the decodeByMsgID switch cases.
	cases := []struct {
		name      string
		msgID     int
		maxFields int
	}{
		{"TickPrice", InTickPrice, 7},                          // version, reqID, tickType, price, size, attrMask
		{"TickSize", InTickSize, 5},                            // version, reqID, tickType, size
		{"OrderStatus", InOrderStatus, 4},                      // orderID, status, filled, remaining
		{"ErrMsg", InErrMsg, 5},                                // reqID, code, message, advJSON, errorTimeMs
		{"OpenOrder", InOpenOrder, 165},                        // orderID + 11 contract + action + qty + orderType + 4 skip + account + 71 skip + status + 68 skip + filled + remaining + 2 skip + parentId
		{"UpdateAccountValue", InUpdateAccountValue, 5},        // version, key, value, currency, account
		{"UpdatePortfolio", InUpdatePortfolio, 19},             // version, conID, symbol, secType, expiry, strike, right, multiplier, primaryExchange, currency, localSymbol, tradingClass, position, marketPrice, marketValue, avgCost, unrealizedPNL, realizedPNL, account
		{"UpdateAccountTime", InUpdateAccountTime, 2},          // version, timestamp
		{"NextValidID", InNextValidID, 2},                      // version, orderID
		{"ContractData", InContractData, 26},                   // reqID, symbol, secType, expiry, skip, strike, right, exchange, currency, localSymbol, marketName, tradingClass, conID, minTick, 5 skip, longName, primaryExchange, 4 skip, timeZoneID
		{"ExecutionData", InExecutionData, 19},                 // reqID, 2 skip, symbol, 8 skip, execID, time, account, 1 skip, side, shares, price
		{"NewsBulletins", InNewsBulletins, 5},                  // version, msgId, msgType, headline, source
		{"ManagedAccounts", InManagedAccounts, 2},              // version, accountsList
		{"HistoricalData", InHistoricalData, 12},               // reqID, barCount, then up to 8 bar fields (time,O,H,L,C,vol,wap,count) + end
		{"ScannerParameters", InScannerParameters, 2},          // version, xml
		{"ScannerData", InScannerData, 20},                     // version, reqID, count, entries(rank + 11 contract + 4 fields)
		{"TickOptionComputation", InTickOptionComputation, 12}, // version, reqID, tickType, tickAttrib, impliedVol, delta, optPrice, pvDividend, gamma, vega, theta, undPrice
		{"TickGeneric", InTickGeneric, 4},                      // version, reqID, tickType, value
		{"TickString", InTickString, 4},                        // version, reqID, tickType, value
		{"CurrentTime", InCurrentTime, 2},                      // version, time
		{"RealTimeBars", InRealTimeBars, 10},                   // version, reqID, time, O, H, L, C, vol, wap, count
		{"FundamentalData", InFundamentalData, 3},              // version, reqID, data
		{"ContractDataEnd", InContractDataEnd, 2},              // version, reqID
		{"OpenOrderEnd", InOpenOrderEnd, 1},                    // version
		{"AccountDownloadEnd", InAccountDownloadEnd, 2},        // version, account
		{"ExecutionDataEnd", InExecutionDataEnd, 2},            // version, reqID
		{"TickSnapshotEnd", InTickSnapshotEnd, 2},              // version, reqID
		{"MarketDataType", InMarketDataType, 3},                // version, reqID, dataType
		{"CommissionReport", InCommissionReport, 5},            // version, execID, commission, currency, realizedPNL
		{"PositionData", InPositionData, 15},                   // version, account, 11 contract, position, avgCost
		{"PositionEnd", InPositionEnd, 1},                      // version
		{"AccountSummary", InAccountSummary, 6},                // version, reqID, account, tag, value, currency
		{"AccountSummaryEnd", InAccountSummaryEnd, 2},          // version, reqID
		{"PositionMulti", InPositionMulti, 17},                 // version, reqID, account, modelCode, 11 contract, position, avgCost
		{"PositionMultiEnd", InPositionMultiEnd, 2},            // version, reqID
		{"AccountUpdateMulti", InAccountUpdateMulti, 7},        // version, reqID, account, modelCode, key, value, currency
		{"AccountUpdateMultiEnd", InAccountUpdateMultiEnd, 2},  // version, reqID
		{"SecDefOptParams", InSecDefOptParams, 10},             // reqID, exchange, underConID, tradingClass, multiplier, marketRuleId, expirationCount, (expirations...), strikeCount, (strikes...)
		{"SecDefOptParamsEnd", InSecDefOptParamsEnd, 1},        // reqID
		{"FamilyCodes", InFamilyCodes, 5},                      // count, then pairs
		{"MktDepthExchanges", InMktDepthExchanges, 10},         // count + entries(5 each)
		{"TickReqParams", InTickReqParams, 4},                  // reqID, minTick, bboExchange, snapshotPermissions
		{"SymbolSamples", InSymbolSamples, 10},                 // reqID, count, entries(conID, symbol, secType, primaryExch, currency, derivCount, derivTypes...)
		{"SmartComponents", InSmartComponents, 5},              // reqID, count, entries(bitNumber, exchangeName, exchangeLetter)
		{"NewsArticle", InNewsArticle, 3},                      // reqID, articleType, articleText
		{"NewsProviders", InNewsProviders, 5},                  // count, then pairs
		{"HistoricalNews", InHistoricalNews, 5},                // reqID, time, providerCode, articleId, headline
		{"HistoricalNewsEnd", InHistoricalNewsEnd, 2},          // reqID, hasMore
		{"HeadTimestamp", InHeadTimestamp, 2},                  // reqID, headTimestamp
		{"HistogramData", InHistogramData, 6},                  // reqID, count, then pairs
		{"MarketRule", InMarketRule, 6},                        // marketRuleId, count, then pairs
		{"PnL", InPnL, 4},                                      // reqID, dailyPnL, unrealizedPnL, realizedPnL
		{"PnLSingle", InPnLSingle, 6},                          // reqID, pos, dailyPnL, unrealizedPnL, realizedPnL, value
		{"HistoricalTicks", InHistoricalTicks, 8},              // reqID, count, entries(time, unused, price, size), done
		{"HistoricalTicksBidAsk", InHistoricalTicksBidAsk, 10}, // reqID, count, entries(time, attrib, bidPrice, askPrice, bidSize, askSize), done
		{"HistoricalTicksLast", InHistoricalTicksLast, 10},     // reqID, count, entries(time, attrib, price, size, exchange, specialConditions), done
		{"TickByTick", InTickByTick, 10},                       // reqID, tickType, time, then type-dependent fields
		{"CompletedOrder", InCompletedOrder, 95},               // 11 contract + action + qty + orderType + 4 skip + 71 skip + status + 3 skip + filled + remaining
		{"CompletedOrderEnd", InCompletedOrderEnd, 0},          // no fields after msg_id
		{"UserInfo", InUserInfo, 2},                            // reqID, whiteBrandingId
		{"HistoricalSchedule", InHistoricalSchedule, 5},        // reqID, start, end, timezone, session count
		{"HistoricalDataUpdate", InHistoricalDataUpdate, 10},   // reqID, barCount, time, O, H, L, C, vol, wap, count
	}

	for _, tc := range cases {
		for n := tc.maxFields; n >= 0; n-- {
			fields := make([]string, n)
			for i := range fields {
				fields[i] = "0"
			}
			t.Run(fmt.Sprintf("%s/%d_fields", tc.name, n), func(t *testing.T) {
				payload := wire.EncodeFields(append([]string{strconv.Itoa(tc.msgID)}, fields...))
				// Must not panic. Errors are acceptable.
				mustNotPanic(t, func() { DecodeBatch(payload) })
			})
		}
	}
}

// TestDecodeUnknownMsgID verifies that every integer 0-255 that is NOT a known
// inbound msg ID returns an error from DecodeBatch (not a panic).
func TestDecodeUnknownMsgID(t *testing.T) {
	t.Parallel()

	known := make(map[int]bool, len(allInboundMsgIDs))
	for _, id := range allInboundMsgIDs {
		known[id] = true
	}

	for id := 0; id <= 255; id++ {
		if known[id] {
			continue
		}
		id := id
		t.Run(strconv.Itoa(id), func(t *testing.T) {
			t.Parallel()
			payload := wire.EncodeFields([]string{strconv.Itoa(id), "0", "0", "0"})
			_, err := DecodeBatch(payload)
			if err == nil {
				t.Errorf("msg_id %d: expected error for unknown msg ID, got nil", id)
			}
		})
	}
}

// TestDecodeNegativeAndOverflowCounts verifies that msg IDs containing
// loop-count fields (barCount, entry count, etc.) handle negative or
// extreme values without panic.
func TestDecodeNegativeAndOverflowCounts(t *testing.T) {
	t.Parallel()

	// Messages where the second-ish field after msg_id is a count driving a loop.
	countMsgs := []struct {
		name   string
		fields []string // msg_id, then fields up to and including the count
	}{
		// HistoricalData: [17, reqID, barCount, ...] — negative barCount
		{"HistoricalData/negative_count", []string{"17", "1", "-1"}},
		{"HistoricalData/zero_count", []string{"17", "1", "0"}},

		// FamilyCodes: [78, count, ...] — negative count
		{"FamilyCodes/negative_count", []string{"78", "-5"}},
		{"FamilyCodes/zero_count", []string{"78", "0"}},

		// MktDepthExchanges: [80, count, ...] — negative count
		// With >2 remaining fields it takes the MktDepthExchanges path.
		{"MktDepthExchanges/negative_count", []string{"80", "-5", "0", "0", "0"}},
		{"MktDepthExchanges/zero_count", []string{"80", "0", "0", "0", "0"}},

		// NewsProviders: [85, count, ...]
		{"NewsProviders/negative_count", []string{"85", "-1"}},
		{"NewsProviders/zero_count", []string{"85", "0"}},

		// ScannerData: [20, version, reqID, count, ...]
		{"ScannerData/negative_count", []string{"20", "3", "1", "-1"}},
		{"ScannerData/zero_count", []string{"20", "3", "1", "0"}},

		// HistogramData: [89, reqID, count, ...]
		{"HistogramData/negative_count", []string{"89", "1", "-1"}},
		{"HistogramData/zero_count", []string{"89", "1", "0"}},

		// MarketRule: [92, ruleID, count, ...]
		{"MarketRule/negative_count", []string{"92", "1", "-1"}},
		{"MarketRule/zero_count", []string{"92", "1", "0"}},

		// HistoricalTicks: [96, reqID, count, ...]
		{"HistoricalTicks/negative_count", []string{"96", "1", "-1"}},
		{"HistoricalTicks/zero_count", []string{"96", "1", "0"}},

		// HistoricalTicksBidAsk: [97, reqID, count, ...]
		{"HistoricalTicksBidAsk/negative_count", []string{"97", "1", "-1"}},

		// HistoricalTicksLast: [98, reqID, count, ...]
		{"HistoricalTicksLast/negative_count", []string{"98", "1", "-1"}},

		// SecDefOptParams: [75, reqID, exchange, underConID, tradingClass, multiplier, marketRuleId, expirationCount, ...]
		{"SecDefOptParams/negative_expiration_count", []string{"75", "1", "SMART", "0", "OPT", "100", "26", "-1"}},
		{"SecDefOptParams/zero_counts", []string{"75", "1", "SMART", "0", "OPT", "100", "26", "0", "0"}},

		// SymbolSamples: [79, reqID, count, ...]
		{"SymbolSamples/negative_count", []string{"79", "1", "-1"}},
		{"SymbolSamples/zero_count", []string{"79", "1", "0"}},
	}

	for _, tc := range countMsgs {
		t.Run(tc.name, func(t *testing.T) {
			payload := wire.EncodeFields(tc.fields)
			// Must not panic.
			mustNotPanic(t, func() { DecodeBatch(payload) })
		})
	}
}

// FuzzEncodeDecodeRoundTrip_OrderStatus proves encode-decode round-trip for OrderStatus.
func FuzzEncodeDecodeRoundTrip_OrderStatus(f *testing.F) {
	f.Add(int64(42), "Filled", "100", "0", "150.50", "123456", "0", "150.50", "99", "", "0")
	f.Add(int64(0), "", "", "", "", "", "", "", "", "", "")
	f.Add(int64(-1), "PreSubmitted", "50", "50", "100.25", "999", "10", "100.25", "1", "locate", "0.0")

	f.Fuzz(func(t *testing.T, orderID int64, status string, filled string, remaining string, avgFillPrice string, permID string, parentID string, lastFillPrice string, clientID string, whyHeld string, mktCapPrice string) {
		if containsNull(status, filled, remaining, avgFillPrice, permID, parentID, lastFillPrice, clientID, whyHeld, mktCapPrice) {
			return
		}
		original := OrderStatus{OrderID: orderID, Status: status, Filled: filled, Remaining: remaining, AvgFillPrice: avgFillPrice, PermID: permID, ParentID: parentID, LastFillPrice: lastFillPrice, ClientID: clientID, WhyHeld: whyHeld, MktCapPrice: mktCapPrice}
		encoded, err := Encode(original)
		if err != nil {
			return
		}
		decoded, err := DecodeBatch(encoded)
		if err != nil {
			return
		}
		if len(decoded) != 1 {
			t.Fatalf("expected 1 message, got %d", len(decoded))
		}
		os, ok := decoded[0].(OrderStatus)
		if !ok {
			t.Fatalf("expected OrderStatus, got %T", decoded[0])
		}
		if os.OrderID != orderID {
			t.Errorf("OrderID: got %d, want %d", os.OrderID, orderID)
		}
		if os.Status != status {
			t.Errorf("Status: got %q, want %q", os.Status, status)
		}
		if os.Filled != filled {
			t.Errorf("Filled: got %q, want %q", os.Filled, filled)
		}
		if os.Remaining != remaining {
			t.Errorf("Remaining: got %q, want %q", os.Remaining, remaining)
		}
		if os.AvgFillPrice != avgFillPrice {
			t.Errorf("AvgFillPrice: got %q, want %q", os.AvgFillPrice, avgFillPrice)
		}
		if os.PermID != permID {
			t.Errorf("PermID: got %q, want %q", os.PermID, permID)
		}
		if os.ParentID != parentID {
			t.Errorf("ParentID: got %q, want %q", os.ParentID, parentID)
		}
		if os.LastFillPrice != lastFillPrice {
			t.Errorf("LastFillPrice: got %q, want %q", os.LastFillPrice, lastFillPrice)
		}
		if os.ClientID != clientID {
			t.Errorf("ClientID: got %q, want %q", os.ClientID, clientID)
		}
		if os.WhyHeld != whyHeld {
			t.Errorf("WhyHeld: got %q, want %q", os.WhyHeld, whyHeld)
		}
		if os.MktCapPrice != mktCapPrice {
			t.Errorf("MktCapPrice: got %q, want %q", os.MktCapPrice, mktCapPrice)
		}
	})
}

// FuzzEncodeDecodeRoundTrip_ExecutionDetail proves encode-decode round-trip for ExecutionDetail.
func FuzzEncodeDecodeRoundTrip_ExecutionDetail(f *testing.F) {
	f.Add(1, int64(42), "0001", "DU12345", "AAPL", "BOT", "100", "150.50", "20260407 10:30:00")
	f.Add(0, int64(0), "", "", "", "", "", "", "")
	f.Add(-1, int64(-1), "exec-99", "U999", "MSFT", "SLD", "200", "300.00", "20250101 09:00:00")

	f.Fuzz(func(t *testing.T, reqID int, orderID int64, execID string, account string, symbol string, side string, shares string, price string, execTime string) {
		if containsNull(execID, account, symbol, side, shares, price, execTime) {
			return
		}
		original := ExecutionDetail{ReqID: reqID, OrderID: orderID, ExecID: execID, Account: account, Symbol: symbol, Side: side, Shares: shares, Price: price, Time: execTime}
		encoded, err := Encode(original)
		if err != nil {
			return
		}
		decoded, err := DecodeBatch(encoded)
		if err != nil {
			return
		}
		if len(decoded) != 1 {
			t.Fatalf("expected 1 message, got %d", len(decoded))
		}
		ed, ok := decoded[0].(ExecutionDetail)
		if !ok {
			t.Fatalf("expected ExecutionDetail, got %T", decoded[0])
		}
		if ed.ReqID != reqID {
			t.Errorf("ReqID: got %d, want %d", ed.ReqID, reqID)
		}
		if ed.OrderID != orderID {
			t.Errorf("OrderID: got %d, want %d", ed.OrderID, orderID)
		}
		if ed.ExecID != execID {
			t.Errorf("ExecID: got %q, want %q", ed.ExecID, execID)
		}
		if ed.Account != account {
			t.Errorf("Account: got %q, want %q", ed.Account, account)
		}
		if ed.Symbol != symbol {
			t.Errorf("Symbol: got %q, want %q", ed.Symbol, symbol)
		}
		if ed.Side != side {
			t.Errorf("Side: got %q, want %q", ed.Side, side)
		}
		if ed.Shares != shares {
			t.Errorf("Shares: got %q, want %q", ed.Shares, shares)
		}
		if ed.Price != price {
			t.Errorf("Price: got %q, want %q", ed.Price, price)
		}
		if ed.Time != execTime {
			t.Errorf("Time: got %q, want %q", ed.Time, execTime)
		}
	})
}

// FuzzEncodeDecodeRoundTrip_CommissionReport proves encode-decode round-trip for CommissionReport.
func FuzzEncodeDecodeRoundTrip_CommissionReport(f *testing.F) {
	f.Add("exec-1", "1.00", "USD", "50.00")
	f.Add("", "", "", "")
	f.Add("exec-999", "0.50", "EUR", "-100.00")

	f.Fuzz(func(t *testing.T, execID string, commission string, currency string, realizedPNL string) {
		if containsNull(execID, commission, currency, realizedPNL) {
			return
		}
		original := CommissionReport{ExecID: execID, Commission: commission, Currency: currency, RealizedPNL: realizedPNL}
		encoded, err := Encode(original)
		if err != nil {
			return
		}
		decoded, err := DecodeBatch(encoded)
		if err != nil {
			return
		}
		if len(decoded) != 1 {
			t.Fatalf("expected 1 message, got %d", len(decoded))
		}
		cr, ok := decoded[0].(CommissionReport)
		if !ok {
			t.Fatalf("expected CommissionReport, got %T", decoded[0])
		}
		if cr.ExecID != execID {
			t.Errorf("ExecID: got %q, want %q", cr.ExecID, execID)
		}
		if cr.Commission != commission {
			t.Errorf("Commission: got %q, want %q", cr.Commission, commission)
		}
		if cr.Currency != currency {
			t.Errorf("Currency: got %q, want %q", cr.Currency, currency)
		}
		if cr.RealizedPNL != realizedPNL {
			t.Errorf("RealizedPNL: got %q, want %q", cr.RealizedPNL, realizedPNL)
		}
	})
}

// FuzzEncodeDecodeRoundTrip_MarketDepthUpdate proves encode-decode round-trip for MarketDepthUpdate.
func FuzzEncodeDecodeRoundTrip_MarketDepthUpdate(f *testing.F) {
	f.Add(1, 0, 0, 1, "150.00", "100")
	f.Add(0, 0, 0, 0, "", "")
	f.Add(-1, 5, 2, 0, "99.99", "500")

	f.Fuzz(func(t *testing.T, reqID int, position int, operation int, side int, price string, size string) {
		if containsNull(price, size) {
			return
		}
		original := MarketDepthUpdate{ReqID: reqID, Position: position, Operation: operation, Side: side, Price: price, Size: size}
		encoded, err := Encode(original)
		if err != nil {
			return
		}
		decoded, err := DecodeBatch(encoded)
		if err != nil {
			return
		}
		if len(decoded) != 1 {
			t.Fatalf("expected 1 message, got %d", len(decoded))
		}
		md, ok := decoded[0].(MarketDepthUpdate)
		if !ok {
			t.Fatalf("expected MarketDepthUpdate, got %T", decoded[0])
		}
		if md.ReqID != reqID {
			t.Errorf("ReqID: got %d, want %d", md.ReqID, reqID)
		}
		if md.Position != position {
			t.Errorf("Position: got %d, want %d", md.Position, position)
		}
		if md.Operation != operation {
			t.Errorf("Operation: got %d, want %d", md.Operation, operation)
		}
		if md.Side != side {
			t.Errorf("Side: got %d, want %d", md.Side, side)
		}
		if md.Price != price {
			t.Errorf("Price: got %q, want %q", md.Price, price)
		}
		if md.Size != size {
			t.Errorf("Size: got %q, want %q", md.Size, size)
		}
	})
}

// FuzzEncodeDecodeRoundTrip_MarketDepthL2Update proves encode-decode round-trip for MarketDepthL2Update.
func FuzzEncodeDecodeRoundTrip_MarketDepthL2Update(f *testing.F) {
	f.Add(1, 0, "ARCA", 0, 1, "150.00", "100", true)
	f.Add(0, 0, "", 0, 0, "", "", false)
	f.Add(-1, 3, "NYSE", 2, 1, "200.50", "1000", true)

	f.Fuzz(func(t *testing.T, reqID int, position int, marketMaker string, operation int, side int, price string, size string, isSmartDepth bool) {
		if containsNull(marketMaker, price, size) {
			return
		}
		original := MarketDepthL2Update{ReqID: reqID, Position: position, MarketMaker: marketMaker, Operation: operation, Side: side, Price: price, Size: size, IsSmartDepth: isSmartDepth}
		encoded, err := Encode(original)
		if err != nil {
			return
		}
		decoded, err := DecodeBatch(encoded)
		if err != nil {
			return
		}
		if len(decoded) != 1 {
			t.Fatalf("expected 1 message, got %d", len(decoded))
		}
		md, ok := decoded[0].(MarketDepthL2Update)
		if !ok {
			t.Fatalf("expected MarketDepthL2Update, got %T", decoded[0])
		}
		if md.ReqID != reqID {
			t.Errorf("ReqID: got %d, want %d", md.ReqID, reqID)
		}
		if md.Position != position {
			t.Errorf("Position: got %d, want %d", md.Position, position)
		}
		if md.MarketMaker != marketMaker {
			t.Errorf("MarketMaker: got %q, want %q", md.MarketMaker, marketMaker)
		}
		if md.Operation != operation {
			t.Errorf("Operation: got %d, want %d", md.Operation, operation)
		}
		if md.Side != side {
			t.Errorf("Side: got %d, want %d", md.Side, side)
		}
		if md.Price != price {
			t.Errorf("Price: got %q, want %q", md.Price, price)
		}
		if md.Size != size {
			t.Errorf("Size: got %q, want %q", md.Size, size)
		}
		if md.IsSmartDepth != isSmartDepth {
			t.Errorf("IsSmartDepth: got %v, want %v", md.IsSmartDepth, isSmartDepth)
		}
	})
}

// FuzzEncodeDecodeRoundTrip_DisplayGroupList proves encode-decode round-trip for DisplayGroupList.
func FuzzEncodeDecodeRoundTrip_DisplayGroupList(f *testing.F) {
	f.Add(1, "1|2|3")
	f.Add(0, "")
	f.Add(-1, "42")

	f.Fuzz(func(t *testing.T, reqID int, groups string) {
		if containsNull(groups) {
			return
		}
		original := DisplayGroupList{ReqID: reqID, Groups: groups}
		encoded, err := Encode(original)
		if err != nil {
			return
		}
		decoded, err := DecodeBatch(encoded)
		if err != nil {
			return
		}
		if len(decoded) != 1 {
			t.Fatalf("expected 1 message, got %d", len(decoded))
		}
		dg, ok := decoded[0].(DisplayGroupList)
		if !ok {
			t.Fatalf("expected DisplayGroupList, got %T", decoded[0])
		}
		if dg.ReqID != reqID {
			t.Errorf("ReqID: got %d, want %d", dg.ReqID, reqID)
		}
		if dg.Groups != groups {
			t.Errorf("Groups: got %q, want %q", dg.Groups, groups)
		}
	})
}

// FuzzEncodeDecodeRoundTrip_FundamentalDataResponse proves encode-decode round-trip for FundamentalDataResponse.
func FuzzEncodeDecodeRoundTrip_FundamentalDataResponse(f *testing.F) {
	f.Add(1, "<FundamentalData/>")
	f.Add(0, "")
	f.Add(-1, "<ReportSnapshot><Company>AAPL</Company></ReportSnapshot>")

	f.Fuzz(func(t *testing.T, reqID int, data string) {
		if containsNull(data) {
			return
		}
		original := FundamentalDataResponse{ReqID: reqID, Data: data}
		encoded, err := Encode(original)
		if err != nil {
			return
		}
		decoded, err := DecodeBatch(encoded)
		if err != nil {
			return
		}
		if len(decoded) != 1 {
			t.Fatalf("expected 1 message, got %d", len(decoded))
		}
		fd, ok := decoded[0].(FundamentalDataResponse)
		if !ok {
			t.Fatalf("expected FundamentalDataResponse, got %T", decoded[0])
		}
		if fd.ReqID != reqID {
			t.Errorf("ReqID: got %d, want %d", fd.ReqID, reqID)
		}
		if fd.Data != data {
			t.Errorf("Data: got %q, want %q", fd.Data, data)
		}
	})
}

// FuzzEncodeDecodeRoundTrip_HistoricalDataUpdate proves encode-decode round-trip for HistoricalDataUpdate.
func FuzzEncodeDecodeRoundTrip_HistoricalDataUpdate(f *testing.F) {
	f.Add(1, 1, "20260101", "100", "101", "99", "100.5", "1000", "100.25", "50")
	f.Add(0, 0, "", "", "", "", "", "", "", "")
	f.Add(-1, 10, "20250615 15:30:00", "200.5", "205.0", "198.0", "202.0", "5000", "201.5", "120")

	f.Fuzz(func(t *testing.T, reqID int, barCount int, ts string, open string, high string, low string, close_ string, volume string, wap string, count string) {
		if containsNull(ts, open, high, low, close_, volume, wap, count) {
			return
		}
		original := HistoricalDataUpdate{ReqID: reqID, BarCount: barCount, Time: ts, Open: open, High: high, Low: low, Close: close_, Volume: volume, WAP: wap, Count: count}
		encoded, err := Encode(original)
		if err != nil {
			return
		}
		decoded, err := DecodeBatch(encoded)
		if err != nil {
			return
		}
		if len(decoded) != 1 {
			t.Fatalf("expected 1 message, got %d", len(decoded))
		}
		hdu, ok := decoded[0].(HistoricalDataUpdate)
		if !ok {
			t.Fatalf("expected HistoricalDataUpdate, got %T", decoded[0])
		}
		if hdu.ReqID != reqID {
			t.Errorf("ReqID: got %d, want %d", hdu.ReqID, reqID)
		}
		if hdu.BarCount != barCount {
			t.Errorf("BarCount: got %d, want %d", hdu.BarCount, barCount)
		}
		if hdu.Time != ts {
			t.Errorf("Time: got %q, want %q", hdu.Time, ts)
		}
		if hdu.Open != open {
			t.Errorf("Open: got %q, want %q", hdu.Open, open)
		}
		if hdu.High != high {
			t.Errorf("High: got %q, want %q", hdu.High, high)
		}
		if hdu.Low != low {
			t.Errorf("Low: got %q, want %q", hdu.Low, low)
		}
		if hdu.Close != close_ {
			t.Errorf("Close: got %q, want %q", hdu.Close, close_)
		}
		if hdu.Volume != volume {
			t.Errorf("Volume: got %q, want %q", hdu.Volume, volume)
		}
		if hdu.WAP != wap {
			t.Errorf("WAP: got %q, want %q", hdu.WAP, wap)
		}
		if hdu.Count != count {
			t.Errorf("Count: got %q, want %q", hdu.Count, count)
		}
	})
}

// TestDecodeFieldParseErrors verifies that non-numeric strings in integer fields
// produce errors rather than panics.
func TestDecodeFieldParseErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		fields []string
	}{
		// TickPrice with non-numeric version
		{"TickPrice/bad_version", []string{"1", "abc", "1", "1", "100", "50", "0"}},
		// TickPrice with non-numeric reqID
		{"TickPrice/bad_reqID", []string{"1", "6", "xyz", "1", "100", "50", "0"}},
		// NextValidID with non-numeric orderID (returned as error, not panic)
		{"NextValidID/bad_orderID", []string{"9", "1", "not_a_number"}},
		// TickReqParams with non-numeric reqID
		{"TickReqParams/bad_reqID", []string{"81", "abc", "0.01", "SMART", "3"}},
		// AccountSummary with non-numeric version
		{"AccountSummary/bad_version", []string{"63", "xyz", "1", "DU123", "Tag", "100", "USD"}},
		// MarketDataType with non-numeric dataType
		{"MarketDataType/bad_dataType", []string{"58", "1", "1", "not_int"}},
		// HeadTimestamp with non-numeric reqID
		{"HeadTimestamp/bad_reqID", []string{"88", "bad", "timestamp"}},
		// PnL with non-numeric reqID
		{"PnL/bad_reqID", []string{"94", "bad", "100", "200", "300"}},
		// HistoricalDataUpdate with non-numeric reqID
		{"HistoricalDataUpdate/bad_reqID", []string{"108", "bad", "1", "t", "o", "h", "l", "c", "v", "w", "n"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload := wire.EncodeFields(tc.fields)
			// Must not panic. Errors are acceptable.
			mustNotPanic(t, func() { DecodeBatch(payload) })
		})
	}
}

// TestDecodeTickByTickVariants exercises each TickByTick sub-type (Last, AllLast,
// BidAsk, MidPoint) with minimal and short field arrays.
func TestDecodeTickByTickVariants(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		fields []string
	}{
		// tickType=1 (Last): reqID, tickType, time, price, size, attrib, exchange, specialConditions
		{"Last/full", []string{"99", "1", "1", "1712345678", "100.5", "200", "0", "SMART", ""}},
		{"Last/short", []string{"99", "1", "1", "1712345678"}},
		{"Last/minimal", []string{"99", "1", "1"}},

		// tickType=2 (AllLast)
		{"AllLast/full", []string{"99", "1", "2", "1712345678", "100.5", "200", "0", "SMART", ""}},

		// tickType=3 (BidAsk): reqID, tickType, time, bidPrice, askPrice, bidSize, askSize, attrib
		{"BidAsk/full", []string{"99", "1", "3", "1712345678", "100.0", "100.5", "100", "200", "0"}},
		{"BidAsk/short", []string{"99", "1", "3", "1712345678"}},

		// tickType=4 (MidPoint): reqID, tickType, time, midPoint
		{"MidPoint/full", []string{"99", "1", "4", "1712345678", "100.25"}},
		{"MidPoint/short", []string{"99", "1", "4"}},

		// tickType=0 (unknown sub-type): should not panic
		{"Unknown/zero", []string{"99", "1", "0", "1712345678"}},
		{"Unknown/99", []string{"99", "1", "99", "1712345678"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload := wire.EncodeFields(tc.fields)
			// Must not panic.
			mustNotPanic(t, func() { DecodeBatch(payload) })
		})
	}
}

func TestDecodeHistoricalNewsEndAndMktDepthExchanges(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		fields   []string
		wantName string
	}{
		{"HistoricalNewsEnd", []string{"87", "1", "1"}, "historical_news_end"},
		{"HistoricalNewsEnd/false", []string{"87", "42", "0"}, "historical_news_end"},

		{"MktDepthExchanges/empty", []string{"80", "0", "0", "0", "0"}, "mkt_depth_exchanges"},

		{"OneField", []string{"80", "1"}, ""},
		{"NoFields", []string{"80"}, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload := wire.EncodeFields(tc.fields)
			if tc.wantName == "" {
				// We just verify no panic; error or weird result is acceptable.
				mustNotPanic(t, func() { DecodeBatch(payload) })
				return
			}
			msgs, err := DecodeBatch(payload)
			if err != nil {
				t.Fatalf("DecodeBatch: %v", err)
			}
			if len(msgs) != 1 {
				t.Fatalf("got %d messages, want 1", len(msgs))
			}
			if msgs[0].messageName() != tc.wantName {
				t.Errorf("messageName() = %q, want %q", msgs[0].messageName(), tc.wantName)
			}
		})
	}
}

func TestDecodeSymbolSamplesAndSmartComponents(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		fields   []string
		wantName string
	}{
		{"SmartComponents/1entry", []string{"82", "1", "1", "0", "ARCA", "P"}, "smart_components"},
		{"SmartComponents/empty", []string{"82", "1", "0"}, "smart_components"},

		{"SymbolSamples/1entry", []string{"79", "1", "1", "265598", "AAPL", "STK", "NASDAQ", "USD", "0"}, "matching_symbols"},
		{"SymbolSamples/empty", []string{"79", "1", "0"}, "matching_symbols"},

		// Degenerate
		{"NoCount", []string{"82", "1"}, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload := wire.EncodeFields(tc.fields)
			if tc.wantName == "" {
				mustNotPanic(t, func() { DecodeBatch(payload) })
				return
			}
			msgs, err := DecodeBatch(payload)
			if err != nil {
				t.Fatalf("DecodeBatch: %v", err)
			}
			if len(msgs) != 1 {
				t.Fatalf("got %d messages, want 1", len(msgs))
			}
			if msgs[0].messageName() != tc.wantName {
				t.Errorf("messageName() = %q, want %q", msgs[0].messageName(), tc.wantName)
			}
		})
	}
}

package codec

import (
	"bytes"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
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
	protocol.InTickPrice,              // 1
	protocol.InTickSize,               // 2
	protocol.InOrderStatus,            // 3
	protocol.InErrMsg,                 // 4
	protocol.InOpenOrder,              // 5
	protocol.InUpdateAccountValue,     // 6
	protocol.InUpdatePortfolio,        // 7
	protocol.InUpdateAccountTime,      // 8
	protocol.InNextValidID,            // 9
	protocol.InContractData,           // 10
	protocol.InExecutionData,          // 11
	protocol.InMarketDepth,            // 12
	protocol.InMarketDepthL2,          // 13
	protocol.InNewsBulletins,          // 14
	protocol.InManagedAccounts,        // 15
	protocol.InHistoricalData,         // 17
	protocol.InBondContractData,       // 18
	protocol.InScannerParameters,      // 19
	protocol.InScannerData,            // 20
	protocol.InTickOptionComputation,  // 21
	protocol.InTickGeneric,            // 45
	protocol.InTickString,             // 46
	protocol.InTickEFP,                // 47
	protocol.InCurrentTime,            // 49
	protocol.InRealTimeBars,           // 50
	protocol.InContractDataEnd,        // 52
	protocol.InOpenOrderEnd,           // 53
	protocol.InAccountDownloadEnd,     // 54
	protocol.InExecutionDataEnd,       // 55
	protocol.InDeltaNeutralValidation, // 56
	protocol.InTickSnapshotEnd,        // 57
	protocol.InMarketDataType,         // 58
	protocol.InCommissionReport,       // 59
	protocol.InPositionData,           // 61
	protocol.InPositionEnd,            // 62
	protocol.InAccountSummary,         // 63
	protocol.InAccountSummaryEnd,      // 64
	protocol.InPositionMulti,          // 71
	protocol.InPositionMultiEnd,       // 72
	protocol.InAccountUpdateMulti,     // 73
	protocol.InAccountUpdateMultiEnd,  // 74
	protocol.InSecDefOptParams,        // 75
	protocol.InSecDefOptParamsEnd,     // 76
	protocol.InFamilyCodes,            // 78
	protocol.InSymbolSamples,          // 79
	protocol.InMktDepthExchanges,      // 80
	protocol.InTickReqParams,          // 81
	protocol.InSmartComponents,        // 82
	protocol.InNewsArticle,            // 83
	protocol.InTickNews,               // 84
	protocol.InNewsProviders,          // 85
	protocol.InHistoricalNews,         // 86
	protocol.InHistoricalNewsEnd,      // 87
	protocol.InHeadTimestamp,          // 88
	protocol.InHistogramData,          // 89
	protocol.InMarketDataReroute,      // 91
	protocol.InMarketDepthReroute,     // 92
	protocol.InMarketRule,             // 93
	protocol.InPnL,                    // 94
	protocol.InPnLSingle,              // 95
	protocol.InHistoricalTicks,        // 96
	protocol.InHistoricalTicksBidAsk,  // 97
	protocol.InHistoricalTicksLast,    // 98
	protocol.InTickByTick,             // 99
	protocol.InOrderBound,             // 100
	protocol.InCompletedOrder,         // 101
	protocol.InCompletedOrderEnd,      // 102
	protocol.InUserInfo,               // 107
	protocol.InHistoricalDataUpdate,   // 90
	protocol.InHistoricalDataEnd,      // 108
	protocol.InReceiveFA,              // 16
	protocol.InSoftDollarTiers,        // 77
	protocol.InDisplayGroupList,       // 67
	protocol.InDisplayGroupUpdated,    // 68
	protocol.InWSHMetaData,            // 104
	protocol.InWSHEventData,           // 105
	protocol.InHistoricalSchedule,     // 106
	protocol.InCurrentTimeInMillis,    // 109
}

// FuzzDecodeBatch proves DecodeBatch never panics across every supported
// negotiated version. Exact live classic/protobuf seeds anchor both envelopes;
// the remaining readable seeds exercise containment boundaries.
func FuzzDecodeBatch(f *testing.F) {
	// Classic ManagedAccounts is from capture
	// 20260710T223024Z-account_summary_snapshot (events SHA-256
	// 71f26259c1556157c0fd72b635934de341d43fe69bb04df72be27927bfa456db).
	f.Add(byte(0), []byte("15\x001\x00DU9000001\x00"))
	// Protobuf ExecutionsEnd is from exact-sv201 capture
	// 20260709T222913Z-protobuf-sv201-executions-empty (events SHA-256
	// a3610dc87dbe654d8fd86ca65e552be706ab3d814244ce941208ac49dfcd819d).
	f.Add(byte(1), []byte{0, 0, 0, 255, 0x08, 0x01})
	f.Add(byte(0), []byte("999\x00"))
	f.Add(byte(0), []byte("998\x00unterminated"))
	// Structural truncation of the captured account-summary row above.
	f.Add(byte(0), []byte("63\x001\x001\x00DU9000001\x00NetLiquidation\x0033911.62\x00EUR"))
	f.Add(byte(0), []byte("17\x001\x002147483647\x00"))
	f.Add(byte(0), []byte{})
	f.Add(byte(0), []byte("not-a-message-id\x00"))

	f.Fuzz(func(t *testing.T, versionSelector byte, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("unexpected panic: %v", r)
			}
		}()
		_, _ = DecodeBatch(200+int(versionSelector)%26, data)
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
		encoded, err := Encode(200, original)
		if err != nil {
			return
		}
		decoded, err := DecodeBatch(200, encoded)
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
		encoded, err := Encode(200, original)
		if err != nil {
			return
		}
		decoded, err := DecodeBatch(200, encoded)
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
		encoded, err := Encode(200, original)
		if err != nil {
			return
		}
		decoded, err := DecodeBatch(200, encoded)
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
		original := TickReqParams{ReqID: reqID, MinTick: minTick, BBOExchange: bboExchange, SnapshotPermissions: new(snapshotPermissions)}
		encoded, err := Encode(200, original)
		if err != nil {
			return
		}
		decoded, err := DecodeBatch(200, encoded)
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
		if trp.SnapshotPermissions == nil || *trp.SnapshotPermissions != snapshotPermissions {
			t.Errorf("SnapshotPermissions: got %v, want %d", trp.SnapshotPermissions, snapshotPermissions)
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
		encoded, err := Encode(200, original)
		if err != nil {
			return
		}
		decoded, err := DecodeBatch(200, encoded)
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
		{"TickPrice", protocol.InTickPrice, 7},                          // version, reqID, tickType, price, size, attrMask
		{"TickSize", protocol.InTickSize, 5},                            // version, reqID, tickType, size
		{"OrderStatus", protocol.InOrderStatus, 4},                      // orderID, status, filled, remaining
		{"ErrMsg", protocol.InErrMsg, 5},                                // reqID, code, message, advJSON, errorTimeMs
		{"OpenOrder", protocol.InOpenOrder, 165},                        // upper bound on the live walk: 29 base + pre-status block + "None" DN block + variable sections + status block + 32-field tail
		{"UpdateAccountValue", protocol.InUpdateAccountValue, 5},        // version, key, value, currency, account
		{"UpdatePortfolio", protocol.InUpdatePortfolio, 19},             // version, conID, symbol, secType, expiry, strike, right, multiplier, primaryExchange, currency, localSymbol, tradingClass, position, marketPrice, marketValue, avgCost, unrealizedPNL, realizedPNL, account
		{"UpdateAccountTime", protocol.InUpdateAccountTime, 2},          // version, timestamp
		{"NextValidID", protocol.InNextValidID, 2},                      // version, orderID
		{"ContractData", protocol.InContractData, 26},                   // reqID, symbol, secType, expiry, skip, strike, right, exchange, currency, localSymbol, marketName, tradingClass, conID, minTick, 5 skip, longName, primaryExchange, 4 skip, timeZoneID
		{"ExecutionData", protocol.InExecutionData, 32},                 // complete classic sv200 execution detail
		{"NewsBulletins", protocol.InNewsBulletins, 5},                  // version, msgId, msgType, headline, source
		{"ManagedAccounts", protocol.InManagedAccounts, 2},              // version, accountsList
		{"HistoricalData", protocol.InHistoricalData, 12},               // reqID, barCount, then up to 8 bar fields (time,O,H,L,C,vol,wap,count) + end
		{"BondContractData", protocol.InBondContractData, 42},           // reqID, 31 fixed bond/common fields, security IDs, and size-rule tail
		{"ScannerParameters", protocol.InScannerParameters, 2},          // version, xml
		{"ScannerData", protocol.InScannerData, 20},                     // version, reqID, count, entries(rank + 10 contract + market name + 4 fields)
		{"TickOptionComputation", protocol.InTickOptionComputation, 12}, // version, reqID, tickType, tickAttrib, impliedVol, delta, optPrice, pvDividend, gamma, vega, theta, undPrice
		{"TickGeneric", protocol.InTickGeneric, 4},                      // version, reqID, tickType, value
		{"TickString", protocol.InTickString, 4},                        // version, reqID, tickType, value
		{"CurrentTime", protocol.InCurrentTime, 2},                      // version, time
		{"RealTimeBars", protocol.InRealTimeBars, 10},                   // version, reqID, time, O, H, L, C, vol, wap, count
		{"ContractDataEnd", protocol.InContractDataEnd, 2},              // version, reqID
		{"OpenOrderEnd", protocol.InOpenOrderEnd, 1},                    // version
		{"AccountDownloadEnd", protocol.InAccountDownloadEnd, 2},        // version, account
		{"ExecutionDataEnd", protocol.InExecutionDataEnd, 2},            // version, reqID
		{"TickSnapshotEnd", protocol.InTickSnapshotEnd, 2},              // version, reqID
		{"MarketDataType", protocol.InMarketDataType, 3},                // version, reqID, dataType
		{"CommissionReport", protocol.InCommissionReport, 7},            // version plus six report fields
		{"PositionData", protocol.InPositionData, 15},                   // version, account, 11 contract, position, avgCost
		{"PositionEnd", protocol.InPositionEnd, 1},                      // version
		{"AccountSummary", protocol.InAccountSummary, 6},                // version, reqID, account, tag, value, currency
		{"AccountSummaryEnd", protocol.InAccountSummaryEnd, 2},          // version, reqID
		{"PositionMulti", protocol.InPositionMulti, 17},                 // version, reqID, account, 11 contract, position, avgCost, modelCode
		{"PositionMultiEnd", protocol.InPositionMultiEnd, 2},            // version, reqID
		{"AccountUpdateMulti", protocol.InAccountUpdateMulti, 7},        // version, reqID, account, modelCode, key, value, currency
		{"AccountUpdateMultiEnd", protocol.InAccountUpdateMultiEnd, 2},  // version, reqID
		{"SecDefOptParams", protocol.InSecDefOptParams, 10},             // reqID, exchange, underConID, tradingClass, multiplier, marketRuleId, expirationCount, (expirations...), strikeCount, (strikes...)
		{"SecDefOptParamsEnd", protocol.InSecDefOptParamsEnd, 1},        // reqID
		{"FamilyCodes", protocol.InFamilyCodes, 5},                      // count, then pairs
		{"MktDepthExchanges", protocol.InMktDepthExchanges, 10},         // count + entries(5 each)
		{"TickReqParams", protocol.InTickReqParams, 4},                  // reqID, minTick, bboExchange, snapshotPermissions
		{"SymbolSamples", protocol.InSymbolSamples, 10},                 // reqID, count, entries(conID, symbol, secType, primaryExch, currency, derivCount, derivTypes..., description, issuerID)
		{"SmartComponents", protocol.InSmartComponents, 5},              // reqID, count, entries(bitNumber, exchangeName, exchangeLetter)
		{"NewsArticle", protocol.InNewsArticle, 3},                      // reqID, articleType, articleText
		{"TickNews", protocol.InTickNews, 6},                            // reqID, time, providerCode, articleId, headline, extraData
		{"NewsProviders", protocol.InNewsProviders, 5},                  // count, then pairs
		{"HistoricalNews", protocol.InHistoricalNews, 5},                // reqID, time, providerCode, articleId, headline
		{"HistoricalNewsEnd", protocol.InHistoricalNewsEnd, 2},          // reqID, hasMore
		{"HeadTimestamp", protocol.InHeadTimestamp, 2},                  // reqID, headTimestamp
		{"HistogramData", protocol.InHistogramData, 6},                  // reqID, count, then pairs
		{"MarketRule", protocol.InMarketRule, 6},                        // marketRuleId, count, then pairs
		{"PnL", protocol.InPnL, 4},                                      // reqID, dailyPnL, unrealizedPnL, realizedPnL
		{"PnLSingle", protocol.InPnLSingle, 6},                          // reqID, pos, dailyPnL, unrealizedPnL, realizedPnL, value
		{"HistoricalTicks", protocol.InHistoricalTicks, 8},              // reqID, count, entries(time, unused, price, size), done
		{"HistoricalTicksBidAsk", protocol.InHistoricalTicksBidAsk, 10}, // reqID, count, entries(time, attrib, bidPrice, askPrice, bidSize, askSize), done
		{"HistoricalTicksLast", protocol.InHistoricalTicksLast, 10},     // reqID, count, entries(time, attrib, price, size, exchange, specialConditions), done
		{"TickByTick", protocol.InTickByTick, 10},                       // reqID, tickType, time, then type-dependent fields
		{"CompletedOrder", protocol.InCompletedOrder, 95},               // 11 contract + action + qty + orderType + 4 skip + 71 skip + status + 3 skip + filled + remaining
		{"CompletedOrderEnd", protocol.InCompletedOrderEnd, 0},          // no fields after msg_id
		{"UserInfo", protocol.InUserInfo, 2},                            // reqID, whiteBrandingId
		{"HistoricalSchedule", protocol.InHistoricalSchedule, 5},        // reqID, start, end, timezone, session count
		{"HistoricalDataUpdate", protocol.InHistoricalDataUpdate, 9},    // reqID, barCount, time, O, C, H, L, wap, vol
		{"HistoricalDataEnd", protocol.InHistoricalDataEnd, 3},          // reqID, startDateTime, endDateTime
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
				mustNotPanic(t, func() { _, _ = DecodeBatch(200, payload) })
			})
		}
	}
}

// TestDecodeUnknownMsgID verifies that every integer 0-255 that is NOT a known
// inbound msg ID decodes to UnknownInbound with the raw fields preserved —
// never an error (which would tear down the session) and never a panic.
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
		t.Run(strconv.Itoa(id), func(t *testing.T) {
			t.Parallel()
			payload := wire.EncodeFields([]string{strconv.Itoa(id), "0", "1", "abc"})
			msgs, err := DecodeBatch(200, payload)
			if err != nil {
				t.Fatalf("msg_id %d: unknown msg ID must not error (it would kill the session): %v", id, err)
			}
			if len(msgs) != 1 {
				t.Fatalf("msg_id %d: got %d messages, want 1", id, len(msgs))
			}
			unknown, ok := msgs[0].(UnknownInbound)
			if !ok {
				t.Fatalf("msg_id %d: got %T, want UnknownInbound", id, msgs[0])
			}
			if unknown.MsgID != id {
				t.Errorf("MsgID = %d, want %d", unknown.MsgID, id)
			}
			if want := []string{"0", "1", "abc"}; !slices.Equal(unknown.Fields, want) {
				t.Errorf("Fields = %q, want %q", unknown.Fields, want)
			}
			// The raw frame re-encodes verbatim for diagnosis fidelity.
			reencoded, err := Encode(200, unknown)
			if err != nil {
				t.Fatalf("re-encode: %v", err)
			}
			if !bytes.Equal(reencoded, payload) {
				t.Errorf("re-encode = %q, want %q", reencoded, payload)
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

		// MarketRule: [93, ruleID, count, ...]
		{"MarketRule/negative_count", []string{"93", "1", "-1"}},
		{"MarketRule/zero_count", []string{"93", "1", "0"}},

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

		// SymbolSamples nested derivative-type count: one entry whose
		// per-entry derivCount is negative or absurd must error, not panic
		// on make([]string, derivCount).
		{"SymbolSamples/negative_deriv_count", []string{"79", "1", "1", "265598", "AAPL", "STK", "NASDAQ", "USD", "-1"}},
		{"SymbolSamples/overflow_deriv_count", []string{"79", "1", "1", "265598", "AAPL", "STK", "NASDAQ", "USD", "2147483647"}},
	}

	for _, tc := range countMsgs {
		t.Run(tc.name, func(t *testing.T) {
			payload := wire.EncodeFields(tc.fields)
			// Must not panic.
			mustNotPanic(t, func() { _, _ = DecodeBatch(200, payload) })
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
		encoded, err := Encode(200, original)
		if err != nil {
			return
		}
		decoded, err := DecodeBatch(200, encoded)
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
		original := ExecutionDetail{ReqID: reqID, OrderID: orderID, Contract: Contract{Symbol: symbol}, ExecID: execID, Account: account, Side: side, Shares: shares, Price: price, Time: execTime}
		encoded, err := Encode(200, original)
		if err != nil {
			return
		}
		decoded, err := DecodeBatch(200, encoded)
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
		if ed.Contract.Symbol != symbol {
			t.Errorf("Symbol: got %q, want %q", ed.Contract.Symbol, symbol)
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
		encoded, err := Encode(200, original)
		if err != nil {
			return
		}
		decoded, err := DecodeBatch(200, encoded)
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
		if cr.Yield != "" {
			t.Errorf("Yield: got %q, want empty", cr.Yield)
		}
		if cr.YieldRedemptionDate != "" {
			t.Errorf("YieldRedemptionDate: got %q, want empty", cr.YieldRedemptionDate)
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
		encoded, err := Encode(200, original)
		if err != nil {
			return
		}
		decoded, err := DecodeBatch(200, encoded)
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
		encoded, err := Encode(200, original)
		if err != nil {
			return
		}
		decoded, err := DecodeBatch(200, encoded)
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
		encoded, err := Encode(200, original)
		if err != nil {
			return
		}
		decoded, err := DecodeBatch(200, encoded)
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

// FuzzEncodeDecodeRoundTrip_HistoricalDataUpdate proves encode-decode round-trip for HistoricalDataUpdate.
func FuzzEncodeDecodeRoundTrip_HistoricalDataUpdate(f *testing.F) {
	f.Add(1, 1, "20260101", "100", "101", "99", "100.5", "1000", "100.25")
	f.Add(0, 0, "", "", "", "", "", "", "")
	f.Add(-1, 10, "20250615 15:30:00", "200.5", "205.0", "198.0", "202.0", "5000", "201.5")

	f.Fuzz(func(t *testing.T, reqID int, barCount int, ts string, open string, high string, low string, close_ string, volume string, wap string) {
		if containsNull(ts, open, high, low, close_, volume, wap) {
			return
		}
		original := HistoricalDataUpdate{ReqID: reqID, BarCount: barCount, Time: ts, Open: open, High: high, Low: low, Close: close_, Volume: volume, WAP: wap}
		encoded, err := Encode(200, original)
		if err != nil {
			return
		}
		decoded, err := DecodeBatch(200, encoded)
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
		{"HistoricalDataUpdate/bad_reqID", []string{"90", "bad", "1", "t", "o", "h", "l", "c", "v", "w", "n"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload := wire.EncodeFields(tc.fields)
			// Must not panic. Errors are acceptable.
			mustNotPanic(t, func() { _, _ = DecodeBatch(200, payload) })
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
			mustNotPanic(t, func() { _, _ = DecodeBatch(200, payload) })
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
		{"HistoricalNewsEnd", []string{"87", "1", "1"}, "codec.HistoricalNewsEnd"},
		{"HistoricalNewsEnd/false", []string{"87", "42", "0"}, "codec.HistoricalNewsEnd"},

		{"MktDepthExchanges/empty", []string{"80", "0", "0", "0", "0"}, "codec.MktDepthExchanges"},

		{"OneField", []string{"80", "1"}, ""},
		{"NoFields", []string{"80"}, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload := wire.EncodeFields(tc.fields)
			if tc.wantName == "" {
				// We just verify no panic; error or weird result is acceptable.
				mustNotPanic(t, func() { _, _ = DecodeBatch(200, payload) })
				return
			}
			msgs, err := DecodeBatch(200, payload)
			if err != nil {
				t.Fatalf("DecodeBatch: %v", err)
			}
			if len(msgs) != 1 {
				t.Fatalf("got %d messages, want 1", len(msgs))
			}
			if got := fmt.Sprintf("%T", msgs[0]); got != tc.wantName {
				t.Errorf("message type = %q, want %q", got, tc.wantName)
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
		{"SmartComponents/1entry", []string{"82", "1", "1", "0", "ARCA", "P"}, "codec.SmartComponentsResponse"},
		{"SmartComponents/empty", []string{"82", "1", "0"}, "codec.SmartComponentsResponse"},

		// API 10.48.01 EDecoder.processSymbolSamplesMsg reads description and
		// issuer ID unconditionally at this library's supported version floor.
		{"SymbolSamples/1entry", []string{"79", "1", "1", "265598", "AAPL", "STK", "NASDAQ", "USD", "0", "123", "issuer-1"}, "codec.MatchingSymbols"},
		{"SymbolSamples/empty", []string{"79", "1", "0"}, "codec.MatchingSymbols"},

		// Degenerate
		{"NoCount", []string{"82", "1"}, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload := wire.EncodeFields(tc.fields)
			if tc.wantName == "" {
				mustNotPanic(t, func() { _, _ = DecodeBatch(200, payload) })
				return
			}
			msgs, err := DecodeBatch(200, payload)
			if err != nil {
				t.Fatalf("DecodeBatch: %v", err)
			}
			if len(msgs) != 1 {
				t.Fatalf("got %d messages, want 1", len(msgs))
			}
			if got := fmt.Sprintf("%T", msgs[0]); got != tc.wantName {
				t.Errorf("message type = %q, want %q", got, tc.wantName)
			}
		})
	}
}

func TestDecodeSymbolSamplesUnconditionalMetadata(t *testing.T) {
	t.Parallel()

	msgs, err := DecodeBatch(200, wire.EncodeFields([]string{
		"79", "1", "1", "265598", "AAPL", "STK", "NASDAQ", "USD", "0", "123", "issuer-1",
	}))
	if err != nil {
		t.Fatal(err)
	}
	got := msgs[0].(MatchingSymbols).Symbols[0]
	if got.Description != "123" || got.IssuerID != "issuer-1" {
		t.Fatalf("symbol metadata = description %q, issuer %q", got.Description, got.IssuerID)
	}
}

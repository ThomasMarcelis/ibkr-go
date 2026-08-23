package codec

import (
	"fmt"
	"slices"
	"strconv"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
)

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

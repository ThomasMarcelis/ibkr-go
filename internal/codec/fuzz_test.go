package codec

import (
	"fmt"
	"slices"
	"strconv"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
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

// FuzzDecodeBatch proves DecodeBatch never panics across every supported
// negotiated version. Exact live classic/protobuf seeds anchor both envelopes;
// the remaining readable seeds exercise containment boundaries.
func FuzzDecodeBatch(f *testing.F) {
	// Classic NextValidID is from the exact sv208 leg of capture
	// 20260824T213929Z-supported_version_matrix_paper (events SHA-256
	// 64ee4350f0bde347a9da914a82865e88e0a68d06924cb13335fd2084595a7727).
	f.Add(byte(0), []byte{0, 0, 0, 9, '1', 0, '5', '8', '1', 0})
	// Protobuf ExecutionsEnd is from sv225 capture
	// 20260824T210943Z-executions_snapshot (events SHA-256
	// 2afd72c3c685c29c00e0f9541eda8e56fd9f372369bbd11a45a30396be423eff).
	f.Add(byte(1), []byte{0, 0, 0, 255, 0x08, 0x01})
	f.Add(byte(0), []byte{0, 0, 3, 231})
	f.Add(byte(0), []byte{0, 0, 3, 230, 'u', 'n', 't', 'e', 'r', 'm', 'i', 'n', 'a', 't', 'e', 'd'})
	f.Add(byte(0), []byte{})
	f.Add(byte(0), []byte{0, 0, 0})

	f.Fuzz(func(t *testing.T, versionSelector byte, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("unexpected panic: %v", r)
			}
		}()
		_, _ = DecodeBatch(208+int(versionSelector)%18, data)
	})
}

func TestDecodeShortFields(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		msgID     int
		maxFields int
	}{
		{"CurrentTimeInMillis", protocol.InCurrentTimeInMillis, 1},
		{"NextValidID", protocol.InNextValidID, 2},
		{"ScannerParameters", protocol.InScannerParameters, 2},
		{"ScannerData", protocol.InScannerData, 20},
		{"TickEFP", protocol.InTickEFP, 10},
		{"CurrentTime", protocol.InCurrentTime, 2},
		{"DeltaNeutralValidation", protocol.InDeltaNeutralValidation, 13},
		{"SecDefOptParams", protocol.InSecDefOptParams, 10},
		{"SecDefOptParamsEnd", protocol.InSecDefOptParamsEnd, 1},
		{"FamilyCodes", protocol.InFamilyCodes, 5},
		{"MktDepthExchanges", protocol.InMktDepthExchanges, 10},
		{"NewsArticle", protocol.InNewsArticle, 3},
		{"TickNews", protocol.InTickNews, 6},
		{"NewsProviders", protocol.InNewsProviders, 5},
		{"SymbolSamples", protocol.InSymbolSamples, 10},
		{"SmartComponents", protocol.InSmartComponents, 5},
		{"HistoricalNews", protocol.InHistoricalNews, 5},
		{"HistoricalNewsEnd", protocol.InHistoricalNewsEnd, 2},
		{"MarketRule", protocol.InMarketRule, 6},
		{"UserInfo", protocol.InUserInfo, 2},
		{"NewsBulletins", protocol.InNewsBulletins, 5},
		{"PnL", protocol.InPnL, 4},
		{"PnLSingle", protocol.InPnLSingle, 6},
		{"ReceiveFA", protocol.InReceiveFA, 3},
		{"SoftDollarTiers", protocol.InSoftDollarTiers, 6},
		{"WSHMetaData", protocol.InWSHMetaData, 2},
		{"WSHEventData", protocol.InWSHEventData, 2},
		{"DisplayGroupList", protocol.InDisplayGroupList, 3},
		{"DisplayGroupUpdated", protocol.InDisplayGroupUpdated, 3},
	}

	covered := make(map[int]bool, len(cases))
	for _, tc := range cases {
		if covered[tc.msgID] {
			t.Fatalf("duplicate classic short-field case for msg_id %d", tc.msgID)
		}
		covered[tc.msgID] = true
		if _, ok := inboundDecoders[tc.msgID]; !ok {
			t.Fatalf("classic short-field case %s names inactive msg_id %d", tc.name, tc.msgID)
		}
	}
	for msgID := range inboundDecoders {
		if !covered[msgID] {
			t.Fatalf("active classic msg_id %d has no short-field case", msgID)
		}
	}

	for _, tc := range cases {
		for n := tc.maxFields; n >= 0; n-- {
			fields := make([]string, n)
			for i := range fields {
				fields[i] = "0"
			}
			t.Run(fmt.Sprintf("%s/%d_fields", tc.name, n), func(t *testing.T) {
				payload := mustEncodeClassicEnvelope(t, tc.msgID, fields)
				mustNotPanic(t, func() { _, _ = DecodeBatch(208, payload) })
			})
		}
	}
}

// TestDecodeUnknownMsgID verifies that every integer 0-255 that is NOT a known
// inbound msg ID decodes to UnknownInbound with the raw fields preserved —
// never an error (which would tear down the session) and never a panic.
func TestDecodeUnknownMsgID(t *testing.T) {
	t.Parallel()

	known := make(map[int]bool)
	for _, message := range protocol.Messages() {
		if message.Direction == protocol.ServerToClient {
			known[message.ID] = true
		}
	}

	for id := 1; id <= protocol.ProtobufMessageID; id++ {
		if known[id] {
			continue
		}
		t.Run(strconv.Itoa(id), func(t *testing.T) {
			t.Parallel()
			payload := mustEncodeClassicEnvelope(t, id, []string{"0", "1", "abc"})
			msgs, err := DecodeBatch(208, payload)
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

	cases := []struct {
		name   string
		fields []string
	}{
		{"FamilyCodes/negative_count", []string{"78", "-5"}},
		{"FamilyCodes/zero_count", []string{"78", "0"}},
		{"MktDepthExchanges/negative_count", []string{"80", "-5", "0", "0", "0"}},
		{"MktDepthExchanges/zero_count", []string{"80", "0", "0", "0", "0"}},
		{"NewsProviders/negative_count", []string{"85", "-1"}},
		{"NewsProviders/zero_count", []string{"85", "0"}},
		{"ScannerData/negative_count", []string{"20", "3", "1", "-1"}},
		{"ScannerData/zero_count", []string{"20", "3", "1", "0"}},
		{"MarketRule/negative_count", []string{"93", "1", "-1"}},
		{"MarketRule/zero_count", []string{"93", "1", "0"}},
		{"SecDefOptParams/negative_expiration_count", []string{"75", "1", "SMART", "0", "OPT", "100", "26", "-1"}},
		{"SecDefOptParams/zero_counts", []string{"75", "1", "SMART", "0", "OPT", "100", "26", "0", "0"}},
		{"SymbolSamples/negative_count", []string{"79", "1", "-1"}},
		{"SymbolSamples/zero_count", []string{"79", "1", "0"}},
		{"SymbolSamples/negative_deriv_count", []string{"79", "1", "1", "265598", "AAPL", "STK", "NASDAQ", "USD", "-1"}},
		{"SymbolSamples/overflow_deriv_count", []string{"79", "1", "1", "265598", "AAPL", "STK", "NASDAQ", "USD", "2147483647"}},
		{"SmartComponents/negative_count", []string{"82", "1", "-1"}},
		{"SoftDollarTiers/negative_count", []string{"77", "1", "-1"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msgID, _ := strconv.Atoi(tc.fields[0])
			if _, ok := inboundDecoders[msgID]; !ok {
				t.Fatalf("count case names inactive classic msg_id %d", msgID)
			}
			payload := mustEncodeClassicFields(t, tc.fields)
			mustNotPanic(t, func() { _, _ = DecodeBatch(208, payload) })
		})
	}
}

func TestDecodeFieldParseErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		fields []string
	}{
		{"NextValidID/bad_orderID", []string{"9", "1", "not_a_number"}},
		{"CurrentTime/bad_time", []string{"49", "1", "not_a_number"}},
		{"CurrentTimeInMillis/bad_time", []string{"109", "not_a_number"}},
		{"TickEFP/bad_reqID", []string{"47", "1", "bad"}},
		{"DeltaNeutralValidation/bad_reqID", []string{"56", "1", "bad"}},
		{"PnL/bad_reqID", []string{"94", "bad", "100", "200", "300"}},
		{"PnLSingle/bad_reqID", []string{"95", "bad", "1", "2", "3", "4", "5"}},
		{"ReceiveFA/bad_data_type", []string{"16", "1", "bad", "<xml/>"}},
		{"DisplayGroupList/bad_reqID", []string{"67", "1", "bad", "1|2"}},
		{"WSHMetaData/bad_reqID", []string{"104", "bad", "{}"}},
		{"UserInfo/bad_reqID", []string{"107", "bad", ""}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			msgID, _ := strconv.Atoi(tc.fields[0])
			if _, ok := inboundDecoders[msgID]; !ok {
				t.Fatalf("parse-error case names inactive classic msg_id %d", msgID)
			}
			payload := mustEncodeClassicFields(t, tc.fields)
			mustNotPanic(t, func() { _, _ = DecodeBatch(208, payload) })
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
			payload := mustEncodeClassicFields(t, tc.fields)
			if tc.wantName == "" {
				// We just verify no panic; error or weird result is acceptable.
				mustNotPanic(t, func() { _, _ = DecodeBatch(208, payload) })
				return
			}
			msgs, err := DecodeBatch(208, payload)
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
			payload := mustEncodeClassicFields(t, tc.fields)
			if tc.wantName == "" {
				mustNotPanic(t, func() { _, _ = DecodeBatch(208, payload) })
				return
			}
			msgs, err := DecodeBatch(208, payload)
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

	msgs, err := DecodeBatch(208, mustEncodeClassicFields(t, []string{
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

func mustEncodeClassicFields(t *testing.T, fields []string) []byte {
	t.Helper()
	msgID, err := strconv.Atoi(fields[0])
	if err != nil {
		t.Fatal(err)
	}
	return mustEncodeClassicEnvelope(t, msgID, fields[1:])
}

package codec

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"
)

// Retained client frames, including refused requests, prove encoder bytes only.
// Account identity is the sole sanitization: its NUL-delimited token becomes
// DU9000001. Hashes identify the original private events.jsonl, not an SDK model.
func TestEncodeRetainedClassicRequests(t *testing.T) {
	option := Contract{ConID: 909906426, Symbol: "AAPL", SecType: "OPT", Expiry: "20260826", Strike: "310", Right: "C", Multiplier: "100", Exchange: "SMART", Currency: "USD", LocalSymbol: "AAPL  260826C00310000", TradingClass: "AAPL"}
	tests := []struct {
		sv                int
		request           OutboundMessage
		hex, source, hash string
	}{
		{208, ScannerSubscriptionRequest{ReqID: 3, NumberOfRows: 10, Instrument: "STK", LocationCode: "STK.US.MAJOR", ScanCode: "HOT_BY_VOLUME"}, "00000016330031300053544b0053544b2e55532e4d414a4f5200484f545f42595f564f4c554d450000000000000000000000000000000000000000", "captures/20260825T195326Z-sv208_classic_boundary_families/events.jsonl", "25aa15fdaeff68a48689bc70e68ddcc519783427a0fd835172c9aaad55246b08"},
		{208, CancelScannerSubscription{ReqID: 3}, "0000001731003300", "captures/20260825T195326Z-sv208_classic_boundary_families/events.jsonl", "25aa15fdaeff68a48689bc70e68ddcc519783427a0fd835172c9aaad55246b08"},
		{208, ScannerParametersRequest{}, "000000183100", "captures/20260825T195326Z-sv208_classic_boundary_families/events.jsonl", "25aa15fdaeff68a48689bc70e68ddcc519783427a0fd835172c9aaad55246b08"},
		{210, CalcImpliedVolatilityRequest{ReqID: 5, Contract: option, OptionPrice: "5", UnderPrice: "309.89"}, "0000003633003500393039393036343236004141504c004f50540032303236303832360033313000430031303000534d4152540000555344004141504c2020323630383236433030333130303030004141504c0035003330392e38390000", "captures/20260825T203959Z-sv210_classic_option_calculations/events.jsonl", "510dedb3be94ed96c3201807cc7d91e0fcd9756e9f98444efa0dbb66faea2289"},
		{210, CalcOptionPriceRequest{ReqID: 4, Contract: option, Volatility: "0.3", UnderPrice: "309.89"}, "0000003732003400393039393036343236004141504c004f50540032303236303832360033313000430031303000534d4152540000555344004141504c2020323630383236433030333130303030004141504c00302e33003330392e38390000", "captures/20260825T203959Z-sv210_classic_option_calculations/events.jsonl", "510dedb3be94ed96c3201807cc7d91e0fcd9756e9f98444efa0dbb66faea2289"},
		{208, QueryDisplayGroupsRequest{ReqID: 8}, "0000004331003800", "captures/20260825T195326Z-sv208_classic_boundary_families/events.jsonl", "25aa15fdaeff68a48689bc70e68ddcc519783427a0fd835172c9aaad55246b08"},
		{208, SubscribeToGroupEventsRequest{ReqID: 1, GroupID: 1}, "00000044310031003100", "captures/20260825T200425Z-sv208_display_group_updated/events.jsonl", "8f56c47cc04a67aead4e491d5549d14db323a00fa8e6ad26d0bb1b71e01cd78e"},
		{211, SecDefOptParamsRequest{ReqID: 3, UnderlyingSymbol: "AAPL", UnderlyingSecType: "STK", UnderlyingConID: 265598}, "0000004e33004141504c000053544b0032363535393800", "captures/20260713T161732Z-api_option_calculations_aapl/events.jsonl", "59056822b51af4a00caa28afb922b4f79ee7014668591392e8f4fae229ea7222"},
		{208, SoftDollarTiersRequest{ReqID: 6}, "0000004f3600", "captures/20260825T195326Z-sv208_classic_boundary_families/events.jsonl", "25aa15fdaeff68a48689bc70e68ddcc519783427a0fd835172c9aaad55246b08"},
		{208, FamilyCodesRequest{}, "00000050", "captures/20260825T195326Z-sv208_classic_boundary_families/events.jsonl", "25aa15fdaeff68a48689bc70e68ddcc519783427a0fd835172c9aaad55246b08"},
		{208, MatchingSymbolsRequest{ReqID: 5, Pattern: "AAPL"}, "0000005135004141504c00", "captures/20260825T195326Z-sv208_classic_boundary_families/events.jsonl", "25aa15fdaeff68a48689bc70e68ddcc519783427a0fd835172c9aaad55246b08"},
		{208, MktDepthExchangesRequest{}, "00000052", "captures/20260825T195326Z-sv208_classic_boundary_families/events.jsonl", "25aa15fdaeff68a48689bc70e68ddcc519783427a0fd835172c9aaad55246b08"},
		{208, SmartComponentsRequest{ReqID: 12, BBOExchange: "9c0001"}, "0000005331320039633030303100", "captures/20260825T195326Z-sv208_classic_boundary_families/events.jsonl", "25aa15fdaeff68a48689bc70e68ddcc519783427a0fd835172c9aaad55246b08"},
		{208, NewsArticleRequest{ReqID: 2, ProviderCode: "BRFG", ArticleID: "BRFG$1f064106"}, "0000005432004252464700425246472431663036343130360000", "captures/20260825T195326Z-sv208_classic_boundary_families/events.jsonl", "25aa15fdaeff68a48689bc70e68ddcc519783427a0fd835172c9aaad55246b08"},
		{208, NewsProvidersRequest{}, "00000055", "captures/20260825T195326Z-sv208_classic_boundary_families/events.jsonl", "25aa15fdaeff68a48689bc70e68ddcc519783427a0fd835172c9aaad55246b08"},
		{208, HistoricalNewsRequest{ReqID: 1, ConID: 265598, ProviderCodes: "BRFG+BRFUPDN+DJNL", TotalResults: 5}, "00000056310032363535393800425246472b4252465550444e2b444a4e4c000000350000", "captures/20260825T195326Z-sv208_classic_boundary_families/events.jsonl", "25aa15fdaeff68a48689bc70e68ddcc519783427a0fd835172c9aaad55246b08"},
		{208, MarketRuleRequest{MarketRuleID: 26}, "0000005b323600", "captures/20260825T195326Z-sv208_classic_boundary_families/events.jsonl", "25aa15fdaeff68a48689bc70e68ddcc519783427a0fd835172c9aaad55246b08"},
		{208, PnLRequest{ReqID: 1, Account: "DU9000001"}, "0000005c31004455393030303030310000", "captures/20260825T195152Z-sv208_classic_pnl/events.jsonl", "dccee2ab425fca9c707dd682c7d9ffc0dafc1dfd1ce093050e425036a8fcb7d2"},
		{208, CancelPnL{ReqID: 1}, "0000005d3100", "captures/20260825T195152Z-sv208_classic_pnl/events.jsonl", "dccee2ab425fca9c707dd682c7d9ffc0dafc1dfd1ce093050e425036a8fcb7d2"},
		{208, PnLSingleRequest{ReqID: 2, Account: "DU9000001", ConID: 117589399}, "0000005e3200445539303030303031000031313735383933393900", "captures/20260825T195152Z-sv208_classic_pnl/events.jsonl", "dccee2ab425fca9c707dd682c7d9ffc0dafc1dfd1ce093050e425036a8fcb7d2"},
		{208, CancelPnLSingle{ReqID: 2}, "0000005f3200", "captures/20260825T195152Z-sv208_classic_pnl/events.jsonl", "dccee2ab425fca9c707dd682c7d9ffc0dafc1dfd1ce093050e425036a8fcb7d2"},
		{208, UserInfoRequest{ReqID: 1}, "000000683100", "captures/20260825T194619Z-sv208_user_info/events.jsonl", "672370162ad17e46cf045647775d1d6bc4480353b2f044392c43431a88717bd5"},
		{208, CurrentTimeMillisRequest{}, "00000069", "captures/20260825T195326Z-sv208_classic_boundary_families/events.jsonl", "25aa15fdaeff68a48689bc70e68ddcc519783427a0fd835172c9aaad55246b08"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("sv%d/%T", tt.sv, tt.request), func(t *testing.T) {
			want, err := hex.DecodeString(tt.hex)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Encode(tt.sv, tt.request)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("Encode = %x, %v; want %x; source %s SHA-256 %s", got, err, want, tt.source, tt.hash)
			}
		})
	}
}

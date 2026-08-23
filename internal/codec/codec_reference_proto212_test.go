package codec

import (
	"bytes"
	"encoding/base64"
	"reflect"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
)

const referenceSV212CaptureHash = "61a20128d4048ee3af2fefc5e16b73b784a227c647156127aa2e5ecb5471373f"

func TestEncodeReferenceProto212LiveVectors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  OutboundMessage
		hex  string
	}{
		{"security-definition options", SecDefOptParamsRequest{ReqID: 7301, UnderlyingSymbol: "AAPL", UnderlyingSecType: "STK", UnderlyingConID: 265598}, "0000011608853912044141504c220353544b28fe9a10"},
		{"soft-dollar tiers", SoftDollarTiersRequest{ReqID: 7302}, "00000117088639"},
		{"family codes", FamilyCodesRequest{}, "00000118"},
		{"matching symbols", MatchingSymbolsRequest{ReqID: 7303, Pattern: "AAPL"}, "0000011908873912044141504c"},
		{"smart components", SmartComponentsRequest{ReqID: 7304, BBOExchange: "a6"}, "0000011b08883912026136"},
		{"market rule", MarketRuleRequest{MarketRuleID: 26}, "00000123081a"},
		{"user info", UserInfoRequest{ReqID: 7305}, "00000130088939"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Encode(212, tc.msg)
			if err != nil {
				t.Fatal(err)
			}
			if want := decodeHex(t, tc.hex); !bytes.Equal(got, want) {
				t.Fatalf("Encode() = %x\nwant     = %x\ncapture events sha256 %s", got, want, referenceSV212CaptureHash)
			}
		})
	}
}

func TestDecodeReferenceProto212LiveVectors(t *testing.T) {
	t.Parallel()

	// This vector retains exact fields from the live CBOE row while selecting
	// its first two repeated expirations and strikes so packed-double handling
	// remains readable in the regression.
	secDef, err := Decode(212, decodeHex(t, "00000113088539120443424f4518fe9a1022044141504c2a0331303032083230323630373133320832303236303731353a1000000000000014400000000000002440"))
	if err != nil {
		t.Fatal(err)
	}
	wantSecDef := SecDefOptParamsResponse{
		ReqID: 7301, Exchange: "CBOE", UnderlyingConID: 265598,
		TradingClass: "AAPL", Multiplier: "100",
		Expirations: []string{"20260713", "20260715"}, Strikes: []string{"5", "10"},
	}
	if !reflect.DeepEqual(secDef, wantSecDef) {
		t.Fatalf("Decode(secdef) = %#v, want %#v; capture events sha256 %s", secDef, wantSecDef, referenceSV212CaptureHash)
	}

	end, err := Decode(212, decodeHex(t, "00000114088539"))
	if err != nil {
		t.Fatal(err)
	}
	if want := (SecDefOptParamsEnd{ReqID: 7301}); !reflect.DeepEqual(end, want) {
		t.Fatalf("Decode(secdef end) = %#v, want %#v", end, want)
	}

	softDollar, err := Decode(212, decodeHex(t, "00000115088639"))
	if err != nil {
		t.Fatal(err)
	}
	if want := (SoftDollarTiersResponse{ReqID: 7302}); !reflect.DeepEqual(softDollar, want) {
		t.Fatalf("Decode(soft dollar) = %#v, want %#v", softDollar, want)
	}

	family, err := Decode(212, decodeHex(t, "000001160a030a012a"))
	if err != nil {
		t.Fatal(err)
	}
	if want := (FamilyCodes{Codes: []FamilyCodeEntry{{AccountID: "*"}}}); !reflect.DeepEqual(family, want) {
		t.Fatalf("Decode(family codes) = %#v, want %#v", family, want)
	}

	// Exact first ContractDescription from the live symbol-samples frame,
	// re-enveloped independently to isolate its schema.
	symbols, err := Decode(212, decodeHex(t, "0000011708873912430a2708fe9a1012044141504c1a0353544b4a064e415344415152035553447a094150504c4520494e43120343464412034f50541204494f505412035741521203424147"))
	if err != nil {
		t.Fatal(err)
	}
	wantSymbols := MatchingSymbols{ReqID: 7303, Symbols: []SymbolSample{{
		ConID: 265598, Symbol: "AAPL", SecType: "STK", PrimaryExchange: "NASDAQ", Currency: "USD",
		DerivativeSecTypes: []string{"CFD", "OPT", "IOPT", "WAR", "BAG"}, Description: "APPLE INC",
	}}}
	if !reflect.DeepEqual(symbols, wantSymbols) {
		t.Fatalf("Decode(symbol samples) = %#v, want %#v", symbols, wantSymbols)
	}

	smartFrame, err := base64.StdEncoding.DecodeString("AAABGgiIORIJEgRBTUVYGgFBEgoIARIDQkVYGgFCEg4IAhIHTllTRU5BVBoBQxILCAMSBE5ZU0UaAU4SCggEEgNJU0UaAUkSDAgFEgVFREdFQRoBShIPCAYSCERSQ1RFREdFGgFLEgsIBxIETFRTRRoBTBIKCAgSA0NIWBoBTRILCAkSBEFSQ0EaAVASDQgKEgZOQVNEQVEaAVQSCggLEgNJRVgaAVYSCwgMEgRUMjRYGgFHEgoIDRIDUFNYGgFYEgoIDhIDQllYGgFZEgsIDxIEQkFUUxoBWhIMCBESBUZJTlJBGgFEEgsIEhIETUVNWBoBVRIMCBMSBVBFQVJMGgFIEgsIFBIEVFhTRRoBRg==")
	if err != nil {
		t.Fatal(err)
	}
	smart, err := Decode(212, smartFrame)
	if err != nil {
		t.Fatal(err)
	}
	smartResponse, ok := smart.(SmartComponentsResponse)
	if !ok || smartResponse.ReqID != 7304 || len(smartResponse.Components) != 20 || smartResponse.Components[0] != (SmartComponentEntry{ExchangeName: "AMEX", ExchangeLetter: "A"}) || smartResponse.Components[19] != (SmartComponentEntry{BitNumber: 20, ExchangeName: "TXSE", ExchangeLetter: "F"}) {
		t.Fatalf("Decode(smart components) = %#v; capture events sha256 %s", smart, referenceSV212CaptureHash)
	}

	marketRule, err := Decode(212, decodeHex(t, "00000125081a1212090000000000000000117b14ae47e17a843f"))
	if err != nil {
		t.Fatal(err)
	}
	wantRule := MarketRule{MarketRuleID: 26, Increments: []PriceIncrement{{LowEdge: "0", Increment: "0.01"}}}
	if !reflect.DeepEqual(marketRule, wantRule) {
		t.Fatalf("Decode(market rule) = %#v, want %#v", marketRule, wantRule)
	}

	userInfo, err := Decode(212, decodeHex(t, "00000133088939"))
	if err != nil {
		t.Fatal(err)
	}
	if want := (UserInfo{ReqID: 7305}); !reflect.DeepEqual(userInfo, want) {
		t.Fatalf("Decode(user info) = %#v, want %#v", userInfo, want)
	}
}

func TestReferenceEncodingBoundary212(t *testing.T) {
	t.Parallel()

	msg := MatchingSymbolsRequest{ReqID: 1, Pattern: "AAPL"}
	classic, err := Encode(211, msg)
	if err != nil {
		t.Fatal(err)
	}
	protobuf, err := Encode(212, msg)
	if err != nil {
		t.Fatal(err)
	}
	classicEnvelope, err := protocol.DecodeEnvelope(211, classic)
	if err != nil {
		t.Fatal(err)
	}
	protobufEnvelope, err := protocol.DecodeEnvelope(212, protobuf)
	if err != nil {
		t.Fatal(err)
	}
	if classicEnvelope.Encoding != protocol.ClassicBody || protobufEnvelope.Encoding != protocol.ProtobufBody {
		t.Fatalf("boundary encodings = %v, %v", classicEnvelope.Encoding, protobufEnvelope.Encoding)
	}
}

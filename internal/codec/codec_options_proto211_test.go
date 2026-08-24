package codec

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
)

const faSV211CaptureHash = "562c394c0570e39ebf34776710f9d7c146005144b96b990a00eb9a93e3b601ae"

const optionCalculationsSV210CaptureHash = "510dedb3be94ed96c3201807cc7d91e0fcd9756e9f98444efa0dbb66faea2289"

func TestEncodeOptionsClassicSV210LiveVectors(t *testing.T) {
	t.Parallel()

	contract := Contract{
		ConID: 909906426, Symbol: "AAPL", SecType: "OPT", Expiry: "20260826",
		Strike: "310", Right: "C", Multiplier: "100", Exchange: "SMART",
		Currency: "USD", LocalSymbol: "AAPL  260826C00310000", TradingClass: "AAPL",
	}
	tests := []struct {
		name string
		msg  OutboundMessage
		hex  string
	}{
		{"option price", CalcOptionPriceRequest{ReqID: 4, Contract: contract, Volatility: "0.3", UnderPrice: "309.89"}, "0000003732003400393039393036343236004141504c004f50540032303236303832360033313000430031303000534d4152540000555344004141504c2020323630383236433030333130303030004141504c00302e33003330392e38390000"},
		{"implied volatility", CalcImpliedVolatilityRequest{ReqID: 5, Contract: contract, OptionPrice: "5", UnderPrice: "309.89"}, "0000003633003500393039393036343236004141504c004f50540032303236303832360033313000430031303000534d4152540000555344004141504c2020323630383236433030333130303030004141504c0035003330392e38390000"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Encode(210, tc.msg)
			if err != nil {
				t.Fatal(err)
			}
			if want := decodeHex(t, tc.hex); !bytes.Equal(got, want) {
				t.Fatalf("Encode() = %x\nwant live sv210 vector %x; capture events sha256 %s", got, want, optionCalculationsSV210CaptureHash)
			}
		})
	}
}

func TestEncodeFAProto211LiveVector(t *testing.T) {
	t.Parallel()

	got, err := Encode(211, RequestFA{FADataType: 1})
	if err != nil {
		t.Fatal(err)
	}
	want := decodeHex(t, "000000da0801")
	if !bytes.Equal(got, want) {
		t.Fatalf("Encode() = %x, want %x; capture events sha256 %s", got, want, faSV211CaptureHash)
	}
}

func TestEncodeOptionsProto211OfficialVectors(t *testing.T) {
	t.Parallel()

	contract := Contract{
		ConID: 887307502, Symbol: "AAPL", SecType: "OPT", Expiry: "20260717",
		Strike: "320", Right: "C", Multiplier: "100", Exchange: "SMART",
		Currency: "USD", TradingClass: "AAPL",
	}
	tests := []struct {
		name string
		msg  OutboundMessage
		hex  string
	}{
		{"exercise", ExerciseOptionsRequest{ReqID: 7301, Contract: contract, ExerciseAction: 1, ExerciseQuantity: 1, Account: "DU9000001"}, "000000dd088539124208eef98ca70312044141504c1a034f5054220832303236303731372900000000000074403201433900000000000059404205534d415254520355534462044141504c180120012a09445539303030303031"},
		{"implied volatility", CalcImpliedVolatilityRequest{ReqID: 7302, Contract: contract, OptionPrice: "5.25", UnderPrice: "318.5"}, "000000fe088639124208eef98ca70312044141504c1a034f5054220832303236303731372900000000000074403201433900000000000059404205534d415254520355534462044141504c190000000000001540210000000000e87340"},
		{"cancel implied volatility", CancelCalcImpliedVolatility{ReqID: 7302}, "00000100088639"},
		{"option price", CalcOptionPriceRequest{ReqID: 7303, Contract: contract, Volatility: "0.25", UnderPrice: "318.5"}, "000000ff088739124208eef98ca70312044141504c1a034f5054220832303236303731372900000000000074403201433900000000000059404205534d415254520355534462044141504c19000000000000d03f210000000000e87340"},
		{"cancel option price", CancelCalcOptionPrice{ReqID: 7303}, "00000101088739"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Encode(211, tc.msg)
			if err != nil {
				t.Fatal(err)
			}
			if want := decodeHex(t, tc.hex); !bytes.Equal(got, want) {
				t.Fatalf("Encode() = %x\nwant official API 10.48 vector %x", got, want)
			}
		})
	}
}

func TestDecodeFAProto211(t *testing.T) {
	t.Parallel()

	body := appendProtoString(appendProtoVarint(nil, 1, 1), 2, "<ListOfGroups/>")
	payload, err := protocol.EncodeProtobufEnvelope(211, protocol.InReceiveFA, body)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(211, payload)
	if err != nil {
		t.Fatal(err)
	}
	want := ReceiveFA{FADataType: 1, XML: "<ListOfGroups/>"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Decode() = %#v, want %#v", got, want)
	}
}

func TestOptionsEncodingBoundary211(t *testing.T) {
	t.Parallel()

	msg := CalcOptionPriceRequest{ReqID: 1, Contract: Contract{ConID: 265598}, Volatility: "0.25", UnderPrice: "318.5"}
	classic, err := Encode(210, msg)
	if err != nil {
		t.Fatal(err)
	}
	protobuf, err := Encode(211, msg)
	if err != nil {
		t.Fatal(err)
	}
	classicEnvelope, err := protocol.DecodeEnvelope(210, classic)
	if err != nil {
		t.Fatal(err)
	}
	protobufEnvelope, err := protocol.DecodeEnvelope(211, protobuf)
	if err != nil {
		t.Fatal(err)
	}
	if classicEnvelope.Encoding != protocol.ClassicBody || protobufEnvelope.Encoding != protocol.ProtobufBody {
		t.Fatalf("boundary encodings = %v, %v", classicEnvelope.Encoding, protobufEnvelope.Encoding)
	}
}

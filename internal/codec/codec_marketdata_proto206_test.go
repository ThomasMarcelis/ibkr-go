package codec

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/protocol"
)

const marketDataSV206CaptureHashes = "eea31798e7e59830f5cda9daadcd94223045c8e9ee0e5aa10f48428447505822, 989563f9c4cad108e34058beac205c576a9ebdc0fffe03e421e829bca851e7de"

func TestContractCompositionLiveQuoteVectorsExact200And206(t *testing.T) {
	t.Parallel()

	combo := Contract{
		Symbol: "AAPL", SecType: "BAG", Exchange: "SMART", Currency: "USD",
		ComboLegs: []ComboLeg{
			{ConID: 887307502, Ratio: 1, Action: "BUY", Exchange: "SMART", OpenClose: "0", ShortSaleSlot: "0", ExemptCode: "-1"},
			{ConID: 887307536, Ratio: 1, Action: "SELL", Exchange: "SMART", OpenClose: "0", ShortSaleSlot: "0", ExemptCode: "-1"},
		},
	}
	tests := []struct {
		name string
		sv   int
		msg  QuoteRequest
		want string
		hash string
	}{
		{
			name: "exact200 combo",
			sv:   200,
			msg:  QuoteRequest{ReqID: 2002201, Contract: combo, Snapshot: true},
			want: "3100313100323030323230310030004141504c004241470000000000534d4152540000555344000000320038383733303735303200310042555900534d4152540038383733303735333600310053454c4c00534d415254003000003100300000",
			hash: "1f8354ee5d9ea0570472caa35d905127f5a8c5bab694ba1f9a74532178842c69",
		},
		{
			name: "exact206 combo",
			sv:   206,
			msg:  QuoteRequest{ReqID: 2062301, Contract: combo, Snapshot: true},
			want: "000000c908ddef7d1266080012044141504c1a034241474205534d4152545203555344a2012308eef98ca70310011a034255592205534d4152542800300040ffffffffffffffffff01a201240890fa8ca70310011a0453454c4c2205534d4152542800300040ffffffffffffffffff012001",
			hash: "d31c21c79b110fdba66e0db556ac12a48b0aa5c089ee47a19054c7a901823f3d",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Encode(tc.sv, tc.msg)
			if err != nil {
				t.Fatal(err)
			}
			if want := decodeHex(t, tc.want); !bytes.Equal(got, want) {
				t.Fatalf("Encode() = %x\nwant     = %x\ncapture events sha256 %s", got, want, tc.hash)
			}
		})
	}
}

func TestRejectedNonBAGDeltaNeutralLiveRequestVectorExact200(t *testing.T) {
	t.Parallel()

	msg := QuoteRequest{ReqID: 2002401, Snapshot: true, Contract: Contract{
		ConID: 887307502, Symbol: "AAPL", SecType: "OPT", Expiry: "20260710", Strike: "315", Right: "C",
		Multiplier: "100", Exchange: "SMART", Currency: "USD", TradingClass: "AAPL",
		DeltaNeutral: &DeltaNeutralContract{ConID: 265598, Delta: "0.5", Price: "314.5"},
	}}
	got, err := Encode(200, msg)
	if err != nil {
		t.Fatal(err)
	}
	want := decodeHex(t, "31003131003230303234303100383837333037353032004141504c004f50540032303236303731300033313500430031303000534d415254000055534400004141504c00310032363535393800302e35003331342e3500003100300000")
	if !bytes.Equal(got, want) {
		t.Fatalf("Encode() = %x\nwant     = %x", got, want)
	}
	// The live exact-200 Gateway rejected this OPT+delta-neutral request with
	// code 320; capture events sha256
	// 6180897b133f7b39fc99377d08a5ebf99a543e097a5a9821573df446e9fb3bad.
	// It freezes the historical wire evidence only. Public validation now
	// requires DeltaNeutral on a BAG contract and does not advertise this as a
	// supported option request.
}

func TestEncodeMarketDataProto206LiveVectors(t *testing.T) {
	t.Parallel()

	stock := Contract{
		ConID: 265598, Symbol: "AAPL", SecType: "STK", Exchange: "SMART",
		PrimaryExchange: "NASDAQ", Currency: "USD",
	}
	tests := []struct {
		name string
		msg  Message
		hex  string
	}{
		{
			"request market data",
			QuoteRequest{
				ReqID: 20611, Contract: stock,
				GenericTicks: strings.Split("100,101,105,106,165,221,225,233,236,293,294,295,318,375,411,456", ","),
			},
			"000000c90883a101122308fe9a1012044141504c1a0353544b4205534d4152544a064e415344415152035553441a3f3130302c3130312c3130352c3130362c3136352c3232312c3232352c3233332c3233362c3239332c3239342c3239352c3331382c3337352c3431312c343536",
		},
		{"cancel market data", CancelQuote{ReqID: 20611}, "000000ca0883a101"},
		{
			"request market depth",
			MarketDepthRequest{
				ReqID: 20613,
				Contract: Contract{
					ConID: 265598, Symbol: "AAPL", SecType: "STK", Exchange: "ISLAND",
					PrimaryExchange: "NASDAQ", Currency: "USD",
				},
				NumRows: 5,
			},
			"000000d20885a101122408fe9a1012044141504c1a0353544b420649534c414e444a064e415344415152035553441805",
		},
		{"cancel market depth", CancelMarketDepth{ReqID: 20613}, "000000d30885a101"},
		{"cancel smart market depth", CancelMarketDepth{ReqID: 20605, IsSmartDepth: true}, "000000d308fda0011001"},
		{"market data type", ReqMarketDataType{DataType: 3}, "000001030803"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Encode(206, tc.msg)
			if err != nil {
				t.Fatal(err)
			}
			if want := decodeHex(t, tc.hex); !bytes.Equal(got, want) {
				t.Fatalf("Encode() = %x\nwant     = %x\ncapture events sha256 %s", got, want, marketDataSV206CaptureHashes)
			}
		})
	}
}

func TestMarketDataEncodingBoundary206(t *testing.T) {
	t.Parallel()

	msg := CancelMarketDepth{ReqID: 17, IsSmartDepth: true}
	classic, err := Encode(205, msg)
	if err != nil {
		t.Fatal(err)
	}
	classicEnvelope, err := protocol.DecodeEnvelope(205, classic)
	if err != nil {
		t.Fatal(err)
	}
	if classicEnvelope.WireID != OutCancelMktDepth || classicEnvelope.Encoding != protocol.ClassicBody {
		t.Fatalf("server_version 205 envelope = %+v, want classic base ID", classicEnvelope)
	}
	protobuf, err := Encode(206, msg)
	if err != nil {
		t.Fatal(err)
	}
	protobufEnvelope, err := protocol.DecodeEnvelope(206, protobuf)
	if err != nil {
		t.Fatal(err)
	}
	if protobufEnvelope.WireID != OutCancelMktDepth+protocol.ProtobufMessageID || protobufEnvelope.Encoding != protocol.ProtobufBody {
		t.Fatalf("server_version 206 envelope = %+v, want protobuf raw ID", protobufEnvelope)
	}
}

func TestMarketDepthProto206PreservesExplicitZeroRows(t *testing.T) {
	t.Parallel()

	encoded, err := Encode(206, MarketDepthRequest{})
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := protocol.DecodeEnvelope(206, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(envelope.Body, []byte{0x18, 0x00}) {
		t.Fatalf("market depth body = %x, want explicit zero numRows", envelope.Body)
	}
}

func TestDecodeMarketDataProto206LiveVectors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		hex  string
		want Message
	}{
		{"tick price", "000000c90883a10110251935baff9f45a673402201302800", TickPrice{ReqID: 20611, TickType: 37, Price: "314.39199829", Size: "0"}},
		{"tick size", "000000ca0883a10110591a083837383731313632", TickSize{ReqID: 20611, TickType: 89, Size: "87871162"}},
		{
			"tick option computation",
			"000000dd08fba001100a18002176b042921244d83f294885d565e9cfaebf31000000a0703d16403944c078ac17aef93f415d13f5a767fc433f49707e746066f5e43f51e8f52f87ea1eadbf59000000000000f0bf",
			TickOptionComputation{
				ReqID: 20603, TickType: 10, ImpliedVol: "0.379154818375134", Delta: "-0.06017999046413053",
				OptPrice: "5.559999942779541", PvDividend: "1.6050030457663675", Gamma: "0.0006099229939669871",
				Vega: "0.6549560436141757", Theta: "-0.05687649631725461", UndPrice: "-1",
			},
		},
		{"tick generic", "000000f50883a101101819d5e9981b57a5d13f", TickGeneric{ReqID: 20611, TickType: 24, Value: "0.2757165688996371"}},
		{"tick string", "000000f60883a101103b1a17312e30352c312e30392c32303236303831302c302e3237", TickString{ReqID: 20611, TickType: 59, Value: "1.05,1.09,20260810,0.27"}},
		{"snapshot end", "0000010108f9a001", TickSnapshotEnd{ReqID: 20601}},
		{"market data type", "000001020883a1011003", MarketDataType{ReqID: 20611, DataType: 3}},
		{
			"request parameters",
			"000001190883a1011204302e30311a0639633030303120042a08302e3030303030313208302e303030303031",
			TickReqParams{
				ReqID: 20611, MinTick: "0.01", BBOExchange: "9c0001", SnapshotPermissions: new(4),
				LastPricePrecision: "0.000001", LastSizePrecision: "0.000001",
			},
		},
		{
			"market depth",
			"000000d40886a1011218080010001801216458c51b9947f23f2a0737353030303030",
			MarketDepthUpdate{ReqID: 20614, Position: 0, Operation: 0, Side: 1, Price: "1.14248", Size: "7500000"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Decode(206, decodeHex(t, tc.hex))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Decode() = %#v\nwant     = %#v\ncapture events sha256 %s", got, tc.want, marketDataSV206CaptureHashes)
			}
		})
	}
}

func TestMarketDataProto206PresenceAndSentinelSemantics(t *testing.T) {
	t.Parallel()

	reqID := appendProtoVarint(nil, 1, 7)
	params, err := decodeTickReqParamsProto(reqID, 206)
	if err != nil {
		t.Fatal(err)
	}
	absent := params[0].(TickReqParams)
	if absent.SnapshotPermissions != nil || absent.LastPricePrecision != "" || absent.LastSizePrecision != "" {
		t.Fatalf("omitted request parameters = %+v, want absent values", absent)
	}
	explicitZero, err := decodeTickReqParamsProto(appendProtoVarint(reqID, 4, 0), 206)
	if err != nil {
		t.Fatal(err)
	}
	present := explicitZero[0].(TickReqParams)
	if present.SnapshotPermissions == nil || *present.SnapshotPermissions != 0 {
		t.Fatalf("explicit zero permissions = %v, want pointer to zero", present.SnapshotPermissions)
	}

	genericBody := appendProtoVarint(nil, 1, 7)
	genericBody = appendProtoVarint(genericBody, 2, 24)
	generic, err := decodeTickGenericProto(appendProtoDouble(genericBody, 3, 0), 206)
	if err != nil || generic[0].(TickGeneric).Value != "0" {
		t.Fatalf("explicit generic zero = %#v, %v", generic, err)
	}

	optionBody := appendProtoVarint(nil, 1, 7)
	optionBody = appendProtoVarint(optionBody, 2, 10)
	optionBody = appendProtoDouble(optionBody, 4, -1)
	optionBody = appendProtoDouble(optionBody, 5, -2)
	option, err := decodeTickOptionComputationProto(optionBody, 206)
	if err != nil {
		t.Fatal(err)
	}
	computation := option[0].(TickOptionComputation)
	if computation.ImpliedVol != "-1" || computation.Delta != "-2" || computation.OptPrice != "" {
		t.Fatalf("option sentinels/absence = %+v", computation)
	}
}

func TestMarketDataProto206OfficialOmissionDefaults(t *testing.T) {
	t.Parallel()

	// API 10.48.01 EDecoder.cpp applies these defaults before invoking the
	// classic callbacks. Omitted optional scalars are valid protobuf, not a
	// malformed frame.
	tests := []struct {
		name   string
		decode protobufDecodeFunc
		body   []byte
		want   []Message
	}{
		{"tick price", decodeTickPriceProto, nil, []Message{TickPrice{ReqID: -1, Price: "0"}}},
		{"tick size", decodeTickSizeProto, nil, []Message{TickSize{ReqID: -1}}},
		{"tick option computation", decodeTickOptionComputationProto, nil, []Message{TickOptionComputation{ReqID: -1}}},
		{"tick generic", decodeTickGenericProto, nil, []Message{TickGeneric{ReqID: -1, Value: "0"}}},
		{"tick string", decodeTickStringProto, nil, []Message{TickString{ReqID: -1}}},
		{"snapshot end", decodeTickSnapshotEndProto, nil, []Message{TickSnapshotEnd{ReqID: -1}}},
		{"market data type", decodeMarketDataTypeProto, nil, []Message{MarketDataType{ReqID: -1}}},
		{
			"market depth data", decodeMarketDepthProto, appendProtoMessage(nil, 2, nil),
			[]Message{MarketDepthUpdate{ReqID: -1, Price: "0"}},
		},
		{
			"market depth L2 data", decodeMarketDepthL2Proto, appendProtoMessage(nil, 2, nil),
			[]Message{MarketDepthL2Update{ReqID: -1, Price: "0"}},
		},
		{"tick request parameters", decodeTickReqParamsProto, nil, []Message{TickReqParams{ReqID: -1}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.decode(tc.body, 206)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("decode omitted fields = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestMarketDepthProto206NestedMessageSemantics(t *testing.T) {
	t.Parallel()

	withoutDepth := appendProtoVarint(nil, 1, 20614)
	msgs, err := decodeMarketDepthProto(withoutDepth, 206)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Fatalf("missing nested depth produced %#v, want no callback", msgs)
	}
}

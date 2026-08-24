package codec

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
)

func TestEncodeRegulatorySnapshotProtobufFlag(t *testing.T) {
	t.Parallel()

	payload, err := Encode(208, QuoteRequest{
		ReqID: 42,
		Contract: Contract{
			ConID: 265598, Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD",
		},
		RegulatorySnapshot: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := protocol.DecodeEnvelope(208, payload)
	if err != nil {
		t.Fatal(err)
	}
	if envelope.MsgID != protocol.OutReqMktData || envelope.Encoding != protocol.ProtobufBody {
		t.Fatalf("envelope = %+v, want protobuf market-data request", envelope)
	}
	if !bytes.HasSuffix(envelope.Body, []byte{0x28, 0x01}) {
		t.Fatalf("protobuf body = %x, want field 5 regulatorySnapshot=true", envelope.Body)
	}
}

func TestMarketDepthProtobufPreservesExplicitZeroRows(t *testing.T) {
	t.Parallel()

	encoded, err := Encode(208, MarketDepthRequest{})
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := protocol.DecodeEnvelope(208, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(envelope.Body, []byte{0x18, 0x00}) {
		t.Fatalf("market depth body = %x, want explicit zero numRows", envelope.Body)
	}
}

func TestMarketDataProtobufPresenceAndSentinelSemantics(t *testing.T) {
	t.Parallel()

	reqID := appendProtoVarint(nil, 1, 7)
	params, err := decodeTickReqParamsProto(reqID, 208)
	if err != nil {
		t.Fatal(err)
	}
	absent := params[0].(TickReqParams)
	if absent.SnapshotPermissions != nil || absent.LastPricePrecision != "" || absent.LastSizePrecision != "" {
		t.Fatalf("omitted request parameters = %+v, want absent values", absent)
	}
	explicitZero, err := decodeTickReqParamsProto(appendProtoVarint(reqID, 4, 0), 208)
	if err != nil {
		t.Fatal(err)
	}
	present := explicitZero[0].(TickReqParams)
	if present.SnapshotPermissions == nil || *present.SnapshotPermissions != 0 {
		t.Fatalf("explicit zero permissions = %v, want pointer to zero", present.SnapshotPermissions)
	}

	genericBody := appendProtoVarint(nil, 1, 7)
	genericBody = appendProtoVarint(genericBody, 2, 24)
	generic, err := decodeTickGenericProto(appendProtoDouble(genericBody, 3, 0), 208)
	if err != nil || generic[0].(TickGeneric).Value != "0" {
		t.Fatalf("explicit generic zero = %#v, %v", generic, err)
	}

	optionBody := appendProtoVarint(nil, 1, 7)
	optionBody = appendProtoVarint(optionBody, 2, 10)
	optionBody = appendProtoDouble(optionBody, 4, -1)
	optionBody = appendProtoDouble(optionBody, 5, -2)
	option, err := decodeTickOptionComputationProto(optionBody, 208)
	if err != nil {
		t.Fatal(err)
	}
	computation := option[0].(TickOptionComputation)
	if computation.ImpliedVol != "-1" || computation.Delta != "-2" || computation.OptPrice != "" {
		t.Fatalf("option sentinels/absence = %+v", computation)
	}
}

func TestMarketDataProtobufOfficialOmissionDefaults(t *testing.T) {
	t.Parallel()

	// API 10.48.01 EDecoder.cpp applies these defaults before invoking the
	// callbacks. Omitted optional scalars are valid protobuf, not malformed.
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
		{"tick request parameters", decodeTickReqParamsProto, nil, []Message{TickReqParams{ReqID: -1}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.decode(tc.body, 208)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("decode omitted fields = %#v, want %#v", got, tc.want)
			}
		})
	}
}

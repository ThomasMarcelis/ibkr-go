package codec

import (
	"bytes"
	"encoding/hex"
	"math"
	"reflect"
	"strconv"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
	"google.golang.org/protobuf/encoding/protowire"
)

func TestEncodeExecutionsRequestFilterSchema(t *testing.T) {
	t.Parallel()

	got, err := Encode(208, ExecutionsRequest{
		ReqID:         41,
		ClientID:      7,
		Account:       "DU9000001",
		Time:          "20260709-00:00:00",
		Symbol:        "AAPL",
		SecType:       "STK",
		Exchange:      "SMART",
		Side:          "BOT",
		LastNDays:     new(2),
		SpecificDates: []int{20260709, 20260710},
	})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	envelope, err := protocol.DecodeEnvelope(208, got)
	if err != nil {
		t.Fatalf("DecodeEnvelope() error = %v", err)
	}
	if envelope.MsgID != protocol.OutReqExecutions || envelope.Encoding != protocol.ProtobufBody {
		t.Fatalf("envelope = %+v", envelope)
	}
	// Official API 10.48.01 ExecutionRequest.proto / ExecutionFilter.proto
	// field vector. This is a schema invariant, not a simulated Gateway trace.
	wantBody := make([]byte, 0, 96)
	wantBody = protowire.AppendTag(wantBody, 1, protowire.VarintType)
	wantBody = protowire.AppendVarint(wantBody, 41)
	filter := make([]byte, 0, 80)
	filter = appendProtoVarint(filter, 1, 7)
	filter = appendProtoString(filter, 2, "DU9000001")
	filter = appendProtoString(filter, 3, "20260709-00:00:00")
	filter = appendProtoString(filter, 4, "AAPL")
	filter = appendProtoString(filter, 5, "STK")
	filter = appendProtoString(filter, 6, "SMART")
	filter = appendProtoString(filter, 7, "BOT")
	filter = appendProtoVarint(filter, 8, 2)
	packedDates := protowire.AppendVarint(nil, 20260709)
	packedDates = protowire.AppendVarint(packedDates, 20260710)
	filter = appendProtoMessage(filter, 9, packedDates)
	wantBody = appendProtoMessage(wantBody, 2, filter)
	if !bytes.Equal(envelope.Body, wantBody) {
		t.Fatalf("protobuf body = %x, want %x", envelope.Body, wantBody)
	}
}

func TestDecodeProtobufCommissionAndErrorSchemas(t *testing.T) {
	t.Parallel()

	commissionBody := appendProtoString(nil, 1, "execution-1")
	commissionBody = appendProtoTestDouble(commissionBody, 2, 1.006695)
	commissionBody = appendProtoString(commissionBody, 3, "USD")
	commissionBody = appendProtoTestDouble(commissionBody, 4, -2.116698)
	commissionPayload, err := protocol.EncodeProtobufEnvelope(208, protocol.InCommissionReport, commissionBody)
	if err != nil {
		t.Fatal(err)
	}
	commission, err := Decode(208, commissionPayload)
	if err != nil {
		t.Fatalf("Decode(commission) error = %v", err)
	}
	wantCommission := CommissionReport{
		ExecID: "execution-1", Commission: "1.006695", Currency: "USD", RealizedPNL: "-2.116698",
	}
	if !reflect.DeepEqual(commission, wantCommission) {
		t.Fatalf("commission = %#v, want %#v", commission, wantCommission)
	}

	errorBody := appendProtoVarint(nil, 1, ^uint64(0))
	errorBody = appendProtoVarint(errorBody, 2, 1783637704284)
	errorBody = appendProtoVarint(errorBody, 3, 2104)
	errorBody = appendProtoString(errorBody, 4, "Market data farm connection is OK:usfarm")
	errorPayload, err := protocol.EncodeProtobufEnvelope(208, protocol.InErrMsg, errorBody)
	if err != nil {
		t.Fatal(err)
	}
	apiError, err := Decode(208, errorPayload)
	if err != nil {
		t.Fatalf("Decode(error) error = %v", err)
	}
	wantError := APIError{ReqID: -1, ErrorTimeMs: "1783637704284", Code: 2104, Message: "Market data farm connection is OK:usfarm"}
	if !reflect.DeepEqual(apiError, wantError) {
		t.Fatalf("error = %#v, want %#v", apiError, wantError)
	}
}

func TestDecodeUnknownProtobufPreservesBinaryBody(t *testing.T) {
	t.Parallel()

	payload, err := protocol.EncodeProtobufEnvelope(208, 123, []byte{0, 1, 0xff, 0})
	if err != nil {
		t.Fatal(err)
	}
	msg, err := Decode(208, payload)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	want := UnknownInbound{MsgID: 123, Encoding: protocol.ProtobufBody, Payload: []byte{0, 1, 0xff, 0}}
	if !reflect.DeepEqual(msg, want) {
		t.Fatalf("Decode() = %#v, want %#v", msg, want)
	}
}

func TestDecodeMalformedProtobuf(t *testing.T) {
	t.Parallel()

	payload, err := protocol.EncodeProtobufEnvelope(208, protocol.InExecutionDataEnd, []byte{0x08, 0x80})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(208, payload); err == nil {
		t.Fatal("Decode() accepted a truncated varint")
	}
	msgs, err := DecodeBatch(208, payload)
	malformed := requireMalformedInbound(t, msgs, err)
	if malformed.MsgID != protocol.InExecutionDataEnd || malformed.Encoding != protocol.ProtobufBody {
		t.Fatalf("MalformedInbound = %+v, want protobuf execution-data-end", malformed)
	}

	payload, err = protocol.EncodeProtobufEnvelope(208, protocol.InExecutionDataEnd, []byte{0x0a, 0})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(208, payload); err == nil {
		t.Fatal("Decode() accepted the wrong wire type for req_id")
	}
}

func TestEncodeExecutionsRequestRejectsInt32Overflow(t *testing.T) {
	t.Parallel()

	if strconv.IntSize == 32 {
		t.Skip("int cannot represent a protobuf int32 overflow on this platform")
	}
	_, err := Encode(208, ExecutionsRequest{ReqID: int64ToInt(math.MaxInt32 + 1)})
	if err == nil {
		t.Fatal("Encode() accepted an out-of-range protobuf int32")
	}
}

func decodeHex(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func int64ToInt(value int64) int {
	return int(value) // #nosec G115 -- test is skipped when int is 32 bits
}

func appendProtoTestDouble(dst []byte, number protowire.Number, value float64) []byte {
	dst = protowire.AppendTag(dst, number, protowire.Fixed64Type)
	return protowire.AppendFixed64(dst, math.Float64bits(value))
}

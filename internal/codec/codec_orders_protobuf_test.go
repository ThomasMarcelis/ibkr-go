package codec

import (
	"bytes"
	"encoding/hex"
	"math"
	"reflect"
	"strconv"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/protocol"
	"google.golang.org/protobuf/encoding/protowire"
)

func TestEncodeExecutionsRequestServer201LiveVector(t *testing.T) {
	t.Parallel()

	got, err := Encode(201, ExecutionsRequest{ReqID: 1001})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	// Local paper Gateway exact-server-version-201 capture, 2026-07-10:
	// request 1001 with a present empty ExecutionFilter.
	want := decodeHex(t, "000000cf08e9071200")
	if !bytes.Equal(got, want) {
		t.Fatalf("Encode() = %x, want live vector %x", got, want)
	}
}

func TestEncodeFailsClosedAtUnimplementedProtobufGate(t *testing.T) {
	t.Parallel()

	payload, err := Encode(212, StartAPI{ClientID: 7})
	if err != nil {
		t.Fatalf("Encode(212) error = %v", err)
	}
	envelope, err := protocol.DecodeEnvelope(212, payload)
	if err != nil {
		t.Fatal(err)
	}
	if envelope.Encoding != protocol.ClassicBody {
		t.Fatalf("Encode(212) encoding = %d, want classic", envelope.Encoding)
	}

	if _, err := Encode(213, StartAPI{ClientID: 7}); err == nil {
		t.Fatal("Encode(213) fell back to classic after the protobuf migration gate")
	}
}

func TestEncodeExecutionsRequestServer201Filter(t *testing.T) {
	t.Parallel()

	lastDays := 2
	got, err := Encode(201, ExecutionsRequest{
		ReqID:         41,
		ClientID:      7,
		Account:       "DU9000001",
		Time:          "20260709-00:00:00",
		Symbol:        "AAPL",
		SecType:       "STK",
		Exchange:      "SMART",
		Side:          "BOT",
		LastNDays:     &lastDays,
		SpecificDates: []int{20260709, 20260710},
	})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	envelope, err := protocol.DecodeEnvelope(201, got)
	if err != nil {
		t.Fatalf("DecodeEnvelope() error = %v", err)
	}
	if envelope.MsgID != OutReqExecutions || envelope.Encoding != protocol.ProtobufBody {
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

func TestDecodeServer201ExecutionEndLiveVector(t *testing.T) {
	t.Parallel()

	msg, err := Decode(201, decodeHex(t, "000000ff08e907"))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if got, want := msg, (ExecutionsEnd{ReqID: 1001}); !reflect.DeepEqual(got, want) {
		t.Fatalf("Decode() = %#v, want %#v", got, want)
	}
}

func TestDecodeServer201ExecutionDetailLiveVector(t *testing.T) {
	t.Parallel()

	// Exact-sv201 paper round-trip capture, 2026-07-10. Account, execution,
	// and permanent IDs are length-preserving substitutions. Private source
	// replay/frames.jsonl sha256:
	// abfaf7eb285b601e6a82f4db780985c815ac6a8e922f25319706b7ecf42fa119
	payload := decodeHex(t, "000000d30802123008fe9a1012044141504c1a0353544b29000000000000000042064e415344415152035553445a044141504c62034e4d531a81010801121773616e6974697a65642d73763230312d6275792d3030311a1c32303236303730392031383a35353a30352055532f4561737465726e22094455393030303030312a064e41534441513203424f543a01314148e17a14aeb773404881d293ad0350c3016201316948e17a14aeb77340900102a801ffffffffffffffffff01")
	msg, err := Decode(201, payload)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	want := ExecutionDetail{
		ReqID:   2,
		OrderID: 1,
		Contract: Contract{
			ConID: 265598, Symbol: "AAPL", SecType: "STK", Strike: "0",
			Exchange: "NASDAQ", Currency: "USD", LocalSymbol: "AAPL", TradingClass: "NMS",
		},
		ExecID: "sanitized-sv201-buy-001", Time: "20260709 18:55:05 US/Eastern",
		Account: "DU9000001", Exchange: "NASDAQ", Side: "BOT", Shares: "1", Price: "315.48",
		PermID: "900000001", ClientID: "195", CumulativeQuantity: "1", AveragePrice: "315.48",
		LastLiquidity: "2", OptExerciseOrLapseType: "-1",
	}
	if !reflect.DeepEqual(msg, want) {
		t.Fatalf("Decode() = %#v, want %#v", msg, want)
	}
}

func TestDecodeServer202ExecutionDetailPreservesPresentZeroStrike(t *testing.T) {
	t.Parallel()

	// Exact-sv202 paper Gateway capture, 2026-07-10. The Contract protobuf
	// carries both conId=265598 and an explicitly present fixed64 strike=0.
	// Request, account, execution, and permanent IDs are deterministic,
	// length-preserving substitutions where possible. Private source
	// replay/frames.jsonl sha256:
	// 3cddf301f80c16dd019979cf3617d1c2b17215c58d89fd35d10fb832714a37e4
	payload := decodeHex(t, "000000d30801123008fe9a1012044141504c1a0353544b29000000000000000042064e415344415152035553445a044141504c62034e4d531a81010801121773616e6974697a65642d73763230322d6275792d3030311a1c32303236303730392031383a35353a30352055532f4561737465726e22094455393030303030312a064e41534441513203424f543a01314148e17a14aeb773404881d293ad0350c3016201316948e17a14aeb77340900102a801ffffffffffffffffff01")
	msg, err := Decode(202, payload)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	detail, ok := msg.(ExecutionDetail)
	if !ok {
		t.Fatalf("Decode() = %T, want ExecutionDetail", msg)
	}
	if detail.ReqID != 1 {
		t.Fatalf("ReqID = %d, want 1", detail.ReqID)
	}
	if detail.Contract.ConID != 265598 || detail.Contract.Strike != "0" {
		t.Fatalf("Contract = %+v, want conId 265598 with explicit zero strike", detail.Contract)
	}
	if detail.ExecID != "sanitized-sv202-buy-001" || detail.Account != "DU9000001" {
		t.Fatalf("execution identity = %q/%q, want sanitized live identity", detail.ExecID, detail.Account)
	}
}

func TestDecodeProtobufCommissionAndErrorSchemas(t *testing.T) {
	t.Parallel()

	commissionBody := appendProtoString(nil, 1, "sanitized-sv201-sell-01")
	commissionBody = appendProtoTestDouble(commissionBody, 2, 1.006695)
	commissionBody = appendProtoString(commissionBody, 3, "USD")
	commissionBody = appendProtoTestDouble(commissionBody, 4, -2.116698)
	commissionPayload, err := protocol.EncodeProtobufEnvelope(201, InCommissionReport, commissionBody)
	if err != nil {
		t.Fatal(err)
	}
	commission, err := Decode(201, commissionPayload)
	if err != nil {
		t.Fatalf("Decode(commission) error = %v", err)
	}
	wantCommission := CommissionReport{
		ExecID: "sanitized-sv201-sell-01", Commission: "1.006695", Currency: "USD", RealizedPNL: "-2.116698",
	}
	if !reflect.DeepEqual(commission, wantCommission) {
		t.Fatalf("commission = %#v, want %#v", commission, wantCommission)
	}

	errorBody := appendProtoVarint(nil, 1, ^uint64(0))
	errorBody = appendProtoVarint(errorBody, 2, 1783637704284)
	errorBody = appendProtoVarint(errorBody, 3, 2104)
	errorBody = appendProtoString(errorBody, 4, "Market data farm connection is OK:usfarm")
	errorPayload, err := protocol.EncodeProtobufEnvelope(201, InErrMsg, errorBody)
	if err != nil {
		t.Fatal(err)
	}
	apiError, err := Decode(201, errorPayload)
	if err != nil {
		t.Fatalf("Decode(error) error = %v", err)
	}
	wantError := APIError{ReqID: -1, ErrorTimeMs: "1783637704284", Code: 2104, Message: "Market data farm connection is OK:usfarm"}
	if !reflect.DeepEqual(apiError, wantError) {
		t.Fatalf("error = %#v, want %#v", apiError, wantError)
	}
}

func TestDecodeServer201RawClassicBootstrap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		hex  string
		want Message
	}{
		{
			name: "managed accounts",
			hex:  "0000000f310044553930303030303100",
			want: ManagedAccounts{Accounts: []string{"DU9000001"}},
		},
		{
			name: "next valid id",
			hex:  "0000000931003200",
			want: NextValidID{OrderID: 2},
		},
		{
			name: "connection status error",
			hex:  "000000042d310032313034004d61726b65742064617461206661726d20636f6e6e656374696f6e206973204f4b3a75736661726d00003137383336333631363136383200",
			want: APIError{ReqID: -1, Code: 2104, Message: "Market data farm connection is OK:usfarm", ErrorTimeMs: "1783636161682"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			msg, err := Decode(201, decodeHex(t, tc.hex))
			if err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if !reflect.DeepEqual(msg, tc.want) {
				t.Fatalf("Decode() = %#v, want %#v", msg, tc.want)
			}
		})
	}
}

func TestDecodeUnknownProtobufPreservesBinaryBody(t *testing.T) {
	t.Parallel()

	payload, err := protocol.EncodeProtobufEnvelope(201, 123, []byte{0, 1, 0xff, 0})
	if err != nil {
		t.Fatal(err)
	}
	msg, err := Decode(201, payload)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	want := UnknownInbound{MsgID: 123, Encoding: protocol.ProtobufBody, Payload: []byte{0, 1, 0xff, 0}}
	if !reflect.DeepEqual(msg, want) {
		t.Fatalf("Decode() = %#v, want %#v", msg, want)
	}
	reencoded, err := Encode(201, msg)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if !bytes.Equal(reencoded, payload) {
		t.Fatalf("Encode() = %x, want %x", reencoded, payload)
	}
}

func TestDecodeMalformedProtobuf(t *testing.T) {
	t.Parallel()

	payload, err := protocol.EncodeProtobufEnvelope(201, InExecutionDataEnd, []byte{0x08, 0x80})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(201, payload); err == nil {
		t.Fatal("Decode() accepted a truncated varint")
	}

	payload, err = protocol.EncodeProtobufEnvelope(201, InExecutionDataEnd, []byte{0x0a, 0})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(201, payload); err == nil {
		t.Fatal("Decode() accepted the wrong wire type for req_id")
	}
}

func TestEncodeExecutionsRequestRejectsInt32Overflow(t *testing.T) {
	t.Parallel()

	if strconv.IntSize == 32 {
		t.Skip("int cannot represent a protobuf int32 overflow on this platform")
	}
	_, err := Encode(201, ExecutionsRequest{ReqID: int64ToInt(math.MaxInt32 + 1)})
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

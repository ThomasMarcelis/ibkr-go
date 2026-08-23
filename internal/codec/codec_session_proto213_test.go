package codec

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
)

const bootstrapSV213CaptureHash = "6e793d3f48bd609810aede9a4f483f44ab70ebf09cd4da5c9fa0e5ba8a79c9d3"

func TestEncodeSessionProto213LiveVectors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  OutboundMessage
		hex  string
	}{
		{"start API", StartAPI{ClientID: 9701}, "0000010f08e54b"},
		{"request IDs", ReqIDsRequest{NumIDs: 1}, "000000d00801"},
		{"current time", CurrentTimeRequest{}, "000000f9"},
		{"current time milliseconds", CurrentTimeMillisRequest{}, "00000131"},
		{"market-depth exchanges", MktDepthExchangesRequest{}, "0000011a"},
		{"query display groups", QueryDisplayGroupsRequest{ReqID: 7401}, "0000010b08e939"},
		{"subscribe display group", SubscribeToGroupEventsRequest{ReqID: 7402, GroupID: 1}, "0000010c08ea391001"},
		{"update display group", UpdateDisplayGroupRequest{ReqID: 7402, ContractInfo: "none"}, "0000010d08ea3912046e6f6e65"},
		{"unsubscribe display group", UnsubscribeFromGroupEventsRequest{ReqID: 7402}, "0000010e08ea39"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Encode(213, tc.msg)
			if err != nil {
				t.Fatal(err)
			}
			if want := decodeHex(t, tc.hex); !bytes.Equal(got, want) {
				t.Fatalf("Encode() = %x\nwant     = %x\ncapture events sha256 %s", got, want, bootstrapSV213CaptureHash)
			}
		})
	}
}

func TestDecodeSessionProto213LiveVectors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		hex  string
		want Message
	}{
		{"next valid ID", "000000d10801", NextValidID{OrderID: 1}},
		{"current time", "000000f90885a5d4d206", CurrentTime{Time: "1783960197"}},
		{"current time milliseconds", "0000013508ccb2c1e2f533", CurrentTimeMillis{TimeMs: "1783960197452"}},
		{"display group list", "0000010b08e939120d317c327c337c347c357c367c37", DisplayGroupList{ReqID: 7401, Groups: "1|2|3|4|5|6|7"}},
		{"display group update", "0000010c08ea3912046e6f6e65", DisplayGroupUpdated{ReqID: 7402, ContractInfo: "none"}},
		// Exact first repeated entry selected from the live depth-exchange frame.
		{"market-depth exchanges", "000001180a100a0344544212034f5054220444656570", MktDepthExchanges{Exchanges: []DepthExchangeEntry{{Exchange: "DTB", SecType: "OPT", ServiceDataType: "Deep"}}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Decode(213, decodeHex(t, tc.hex))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Decode() = %#v, want %#v; capture events sha256 %s", got, tc.want, bootstrapSV213CaptureHash)
			}
		})
	}
}

func TestSessionEncodingBoundary213(t *testing.T) {
	t.Parallel()

	classic, err := Encode(212, StartAPI{ClientID: 7})
	if err != nil {
		t.Fatal(err)
	}
	protobuf, err := Encode(213, StartAPI{ClientID: 7})
	if err != nil {
		t.Fatal(err)
	}
	classicEnvelope, err := protocol.DecodeEnvelope(212, classic)
	if err != nil {
		t.Fatal(err)
	}
	protobufEnvelope, err := protocol.DecodeEnvelope(213, protobuf)
	if err != nil {
		t.Fatal(err)
	}
	if classicEnvelope.Encoding != protocol.ClassicBody || protobufEnvelope.Encoding != protocol.ProtobufBody {
		t.Fatalf("boundary encodings = %v, %v", classicEnvelope.Encoding, protobufEnvelope.Encoding)
	}
}

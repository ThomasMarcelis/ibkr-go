package codec

import (
	"strings"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
)

func TestCaptureDecodeMarketDataRerouteProto225Live(t *testing.T) {
	t.Parallel()

	// Capture 20260825T201807Z-live_cfd_quote_reroute_v201_positive,
	// events.jsonl sha256
	// ca8fbdf11d260066fb7cd1c3d60e6e44808a54bf6a8fc678f3597bd71a666f1c.
	// The Gateway returned raw ID 291 with protobuf fields reqID=1, conID=8314,
	// exchange=SMART after a CFD quote request.
	payload := decodeHex(t, "00000123080110fa401a05534d415254")
	got, err := Decode(225, payload)
	if err != nil {
		t.Fatal(err)
	}
	want := MarketDataReroute{ReqID: 1, ConID: 8314, Exchange: "SMART"}
	if got != want {
		t.Fatalf("Decode() = %#v, want %#v", got, want)
	}
}

func TestCapturedMarketDataRerouteWithoutRequestIDIsMalformed(t *testing.T) {
	t.Parallel()

	// Remove protobuf field 1 from the exact live frame above. A registered
	// callback without its route identity cannot be confined to one request.
	payload := decodeHex(t, "00000123080110fa401a05534d415254")
	payload = append(payload[:4:4], payload[6:]...)
	messages, err := DecodeBatch(225, payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("DecodeBatch() messages = %d, want 1", len(messages))
	}
	malformed, ok := messages[0].(MalformedInbound)
	if !ok || malformed.MsgID != protocol.InMarketDataReroute || !strings.Contains(malformed.Err.Error(), "missing required request id") {
		t.Fatalf("DecodeBatch() = %#v, want malformed market-data reroute", messages[0])
	}
}

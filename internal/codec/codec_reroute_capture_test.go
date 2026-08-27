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

func TestCapturedMarketDataRerouteWithoutUsableRequestIDIsMalformed(t *testing.T) {
	t.Parallel()

	// Mutate only request-id field 1 in the exact live frame above. A registered
	// callback without a positive route identity cannot be confined to one
	// request and must poison its transport generation.
	for _, test := range []struct {
		name string
		hex  string
		want string
	}{
		{name: "missing", hex: "0000012310fa401a05534d415254", want: "missing required request id"},
		{name: "zero", hex: "00000123080010fa401a05534d415254", want: "invalid request id 0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			messages, err := DecodeBatch(225, decodeHex(t, test.hex))
			if err != nil {
				t.Fatal(err)
			}
			if len(messages) != 1 {
				t.Fatalf("DecodeBatch() messages = %d, want 1", len(messages))
			}
			malformed, ok := messages[0].(MalformedInbound)
			if !ok || malformed.MsgID != protocol.InMarketDataReroute || !strings.Contains(malformed.Err.Error(), test.want) {
				t.Fatalf("DecodeBatch() = %#v, want malformed market-data reroute containing %q", messages[0], test.want)
			}
		})
	}
}

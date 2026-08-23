package codec

import (
	"bytes"
	"strings"
	"testing"
)

const cancelSV215CaptureHash = "b3515b46284970f338db6ede7b2864f4d63449027f9f48ff203a67f4fd34d019"

func TestEncodeCancelProto215LiveVectors(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		msg  OutboundMessage
		hex  string
	}{
		{"contract details", CancelContractData{ReqID: 7601}, "0000013208b13b"},
		{"historical ticks", CancelHistoricalTicks{ReqID: 7602}, "0000013308b23b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Encode(215, tc.msg)
			if err != nil {
				t.Fatal(err)
			}
			if want := decodeHex(t, tc.hex); !bytes.Equal(got, want) {
				t.Fatalf("Encode() = %x\nwant     = %x\ncapture events sha256 %s", got, want, cancelSV215CaptureHash)
			}
		})
	}
}

func TestEncodeCancelProto215RejectsEarlierVersions(t *testing.T) {
	t.Parallel()

	for _, msg := range []OutboundMessage{
		CancelContractData{ReqID: 7601},
		CancelHistoricalTicks{ReqID: 7602},
	} {
		if _, err := Encode(214, msg); err == nil || !strings.Contains(err.Error(), "requires server_version 215") {
			t.Fatalf("Encode(214, %T) error = %v, want version rejection", msg, err)
		}
	}
}

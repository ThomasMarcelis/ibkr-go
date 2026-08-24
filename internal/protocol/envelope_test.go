package protocol

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
)

func TestEnvelopeNegotiatedEncoding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		version    int
		hexPayload string
		msgID      int
		wireID     int
		encoding   BodyEncoding
		hexBody    string
	}{
		{
			name:       "raw classic start api at supported floor",
			version:    208,
			hexPayload: "00000047320039320000",
			msgID:      71,
			wireID:     71,
			encoding:   ClassicBody,
			hexBody:    "320039320000",
		},
		{
			name:       "protobuf executions at supported floor",
			version:    208,
			hexPayload: "000000cf08e9071200",
			msgID:      7,
			wireID:     207,
			encoding:   ProtobufBody,
			hexBody:    "08e9071200",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload, err := hex.DecodeString(tc.hexPayload)
			if err != nil {
				t.Fatal(err)
			}
			envelope, err := DecodeEnvelope(tc.version, payload)
			if err != nil {
				t.Fatalf("DecodeEnvelope() error = %v", err)
			}
			if envelope.MsgID != tc.msgID || envelope.WireID != tc.wireID || envelope.Encoding != tc.encoding {
				t.Fatalf("DecodeEnvelope() = %+v, want msg_id=%d wire_id=%d encoding=%d", envelope, tc.msgID, tc.wireID, tc.encoding)
			}
			if got := hex.EncodeToString(envelope.Body); got != tc.hexBody {
				t.Fatalf("body = %s, want %s", got, tc.hexBody)
			}
		})
	}
}

func TestEncodeEnvelopeSupportedFloorVectors(t *testing.T) {
	t.Parallel()

	startAPI, err := EncodeClassicEnvelope(208, OutStartAPI, []string{"2", "92", ""})
	if err != nil {
		t.Fatalf("EncodeClassicEnvelope() error = %v", err)
	}
	if want := mustDecodeHex(t, "00000047320039320000"); !bytes.Equal(startAPI, want) {
		t.Fatalf("start API = %x, want %x", startAPI, want)
	}

	executions, err := EncodeProtobufEnvelope(208, OutReqExecutions, mustDecodeHex(t, "08e9071200"))
	if err != nil {
		t.Fatalf("EncodeProtobufEnvelope() error = %v", err)
	}
	if want := mustDecodeHex(t, "000000cf08e9071200"); !bytes.Equal(executions, want) {
		t.Fatalf("executions = %x, want %x", executions, want)
	}
}

func TestEncodeClassicEnvelopeRejectsEmbeddedNUL(t *testing.T) {
	t.Parallel()

	_, err := EncodeClassicEnvelope(208, OutStartAPI, []string{"2", "bad\x00field"})
	if err == nil {
		t.Fatal("EncodeClassicEnvelope() error = nil")
	}
}

func TestDecodeEnvelopeRejectsMalformedRawID(t *testing.T) {
	t.Parallel()

	for _, payload := range [][]byte{{0, 0, 1}, {0, 0, 0, 0}} {
		if _, err := DecodeEnvelope(208, payload); !errors.Is(err, wire.ErrMalformedFrame) {
			t.Fatalf("DecodeEnvelope(208, %x) error = %v, want ErrMalformedFrame", payload, err)
		}
	}
}

func TestDecodeEnvelopeAllocations(t *testing.T) {
	payload := append([]byte{0, 0, 0, 1}, []byte("6\x001001\x0068\x00255.45\x00200\x000\x00")...)
	if allocations := testing.AllocsPerRun(1000, func() {
		if _, err := DecodeEnvelope(208, payload); err != nil {
			t.Fatal(err)
		}
	}); allocations != 0 {
		t.Fatalf("DecodeEnvelope() allocations = %v, want 0", allocations)
	}
}

func mustDecodeHex(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

package main

import (
	"encoding/hex"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/capturelog"
	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

func TestTranscriptFrameStateServer201(t *testing.T) {
	t.Parallel()

	state := transcriptFrameState{serverVersions: make(map[int]int)}
	tests := []struct {
		direction string
		payload   []byte
		msgID     string
		encoding  string
	}{
		{"client", []byte("v100..201"), "version_range", "pre_session"},
		{"server", wire.EncodeFields([]string{"201", "20260710 00:29:21 CET"}), "server_info", "pre_session"},
		{"client", decodeHex(t, "00000047320039320000"), "71", "classic"},
		{"client", decodeHex(t, "000000cf08e9071200"), "7", "protobuf"},
		{"server", decodeHex(t, "000000ff08e907"), "55", "protobuf"},
	}
	for _, tc := range tests {
		msgID, encoding, err := state.describe(capturelog.ReplayEvent{Leg: 1, Direction: tc.direction}, tc.payload)
		if err != nil {
			t.Fatalf("describe(%s, %x) error = %v", tc.direction, tc.payload, err)
		}
		if msgID != tc.msgID || encoding != tc.encoding {
			t.Fatalf("describe(%s, %x) = (%q, %q), want (%q, %q)", tc.direction, tc.payload, msgID, encoding, tc.msgID, tc.encoding)
		}
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

package ibkr

import (
	"encoding/base64"
	"testing"
)

func liveCapturedFrame(t *testing.T, value string) []byte {
	t.Helper()
	frame, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatalf("decode exact live scanner frame: %v", err)
	}
	if len(frame) < 4 {
		t.Fatalf("exact live scanner frame has %d bytes", len(frame))
	}
	return frame[4:]
}

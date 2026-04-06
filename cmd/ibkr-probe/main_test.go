package main

import (
	"encoding/hex"
	"testing"
)

func TestBuildBootstrapPayload(t *testing.T) {
	t.Parallel()

	payload, err := buildBootstrapPayload(100, 200, []byte(" extra"), "null-newline", true)
	if err != nil {
		t.Fatalf("buildBootstrapPayload() error = %v", err)
	}

	got := hex.EncodeToString(payload)
	const want = "0000001541504900763130302e2e323030206578747261000a"
	if got != want {
		t.Fatalf("buildBootstrapPayload() = %s, want %s", got, want)
	}
}

func TestParseEscaped(t *testing.T) {
	t.Parallel()

	got, err := parseEscaped("API\\0v100..200\\n\\x01")
	if err != nil {
		t.Fatalf("parseEscaped() error = %v", err)
	}

	const want = "41504900763130302e2e3230300a01"
	if gotHex := hex.EncodeToString(got); gotHex != want {
		t.Fatalf("parseEscaped() = %s, want %s", gotHex, want)
	}
}

func TestStripWhitespace(t *testing.T) {
	t.Parallel()

	got := stripWhitespace("41 50\n49\t00\r76")
	if got != "4150490076" {
		t.Fatalf("stripWhitespace() = %q, want %q", got, "4150490076")
	}
}

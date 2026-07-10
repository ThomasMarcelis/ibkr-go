package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/capturelog"
	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

func TestCaptureFrameStateServer201(t *testing.T) {
	t.Parallel()

	state := newCaptureFrameState()
	if err := state.connect(1); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		direction string
		payload   []byte
		want      frameDescription
	}{
		{"client", []byte("v100..201"), frameDescription{label: "version_range", encoding: "pre_session"}},
		{"server", wire.EncodeFields([]string{"201", "20260710 00:29:21 CET"}), frameDescription{label: "server_info", encoding: "pre_session", serverVersion: 201}},
		{"client", decodeHex(t, "00000047320039320000"), frameDescription{msgID: 71, encoding: "classic", serverVersion: 201, session: true}},
		{"client", decodeHex(t, "000000cf08e9071200"), frameDescription{msgID: 7, encoding: "protobuf", serverVersion: 201, session: true}},
		{"server", decodeHex(t, "000000ff08e907"), frameDescription{msgID: 55, encoding: "protobuf", serverVersion: 201, session: true}},
	}
	for _, tc := range tests {
		got, err := state.describe(capturelog.ReplayEvent{Leg: 1, Direction: tc.direction}, tc.payload)
		if err != nil {
			t.Fatalf("describe(%s, %x) error = %v", tc.direction, tc.payload, err)
		}
		if got != tc.want {
			t.Fatalf("describe(%s, %x) = %#v, want %#v", tc.direction, tc.payload, got, tc.want)
		}
	}
}

func TestCaptureFrameStateRejectsMalformedAndOutOfOrderHandshake(t *testing.T) {
	t.Parallel()

	serverInfo := wire.EncodeFields([]string{"201", "20260710 00:29:21 Central European Standard Time"})
	tests := []struct {
		name      string
		direction string
		payload   []byte
	}{
		{"malformed version range", "client", []byte("X100..201")},
		{"server info before version range", "server", serverInfo},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			state := newCaptureFrameState()
			if err := state.connect(1); err != nil {
				t.Fatal(err)
			}
			if _, err := state.describe(capturelog.ReplayEvent{Leg: 1, Direction: tc.direction}, tc.payload); err == nil {
				t.Fatal("describe() error = nil, want handshake rejection")
			}
		})
	}
}

func TestCaptureFrameStateResetsReusedLeg(t *testing.T) {
	t.Parallel()

	state := newCaptureFrameState()
	for cycle := range 2 {
		if err := state.connect(1); err != nil {
			t.Fatalf("cycle %d connect: %v", cycle, err)
		}
		version, err := state.describe(capturelog.ReplayEvent{Leg: 1, Direction: "client"}, []byte("v100..201"))
		if err != nil {
			t.Fatalf("cycle %d version range: %v", cycle, err)
		}
		if version.label != "version_range" {
			t.Fatalf("cycle %d version label = %q", cycle, version.label)
		}
		if _, err := state.describe(
			capturelog.ReplayEvent{Leg: 1, Direction: "server"},
			wire.EncodeFields([]string{"201", "20260710 00:29:21 Central European Standard Time"}),
		); err != nil {
			t.Fatalf("cycle %d server info: %v", cycle, err)
		}
		if err := state.disconnect(1); err != nil {
			t.Fatalf("cycle %d disconnect: %v", cycle, err)
		}
	}
}

func TestWriteVerificationLiveClassicCapture(t *testing.T) {
	t.Parallel()

	// Exact fields retained in current_time_live.txt from readonly-live capture
	// 20260611T074046Z-current_time at server_version 200.
	events := liveCaptureEvents(t, 100, 200, "20260611 09:40:46 Central European Standard Time",
		[]captureFrame{{"client", wire.EncodeFields([]string{"71", "2", "1", ""})}},
		[]captureFrame{
			{"client", wire.EncodeFields([]string{"49", "1"})},
			{"server", wire.EncodeFields([]string{"49", "1", "1781163646"})},
		},
	)
	output := verifyLiveEvents(t, events)
	if !strings.Contains(output, "server_version=200") ||
		!strings.Contains(output, "client_msg_ids: 49:1,71:1,v100..200:1") ||
		!strings.Contains(output, "server_msg_ids: 49:1,200:1") {
		t.Fatalf("verification output missing classic evidence:\n%s", output)
	}
}

func TestWriteVerificationLiveProtobufEndAndAPIError(t *testing.T) {
	t.Parallel()

	t.Run("execution end", func(t *testing.T) {
		// Exact sanitized frames retained in executions_empty_sv201_live.txt;
		// source capture events sha256 a3610dc87dbe654d8fd86ca65e552be706ab3d814244ce941208ac49dfcd819d.
		events := liveCaptureEvents(t, 100, 201, "20260710 00:29:21 Central European Standard Time",
			[]captureFrame{{"client", decodeHex(t, "00000047320039320000")}},
			[]captureFrame{
				{"client", decodeHex(t, "000000cf08e9071200")},
				{"server", decodeHex(t, "000000ff08e907")},
			},
		)
		output := verifyLiveEvents(t, events)
		if !strings.Contains(output, "server_version=201") ||
			!strings.Contains(output, "end_markers: InExecutionDataEnd:1") {
			t.Fatalf("verification output missing protobuf completion:\n%s", output)
		}
	})

	t.Run("inline historical end", func(t *testing.T) {
		// Exact frames retained from readonly-live max-version-195 capture
		// 20260710T203504Z-historical_bars_sv195_inline_end. The source
		// events sha256 is 32ea0cde9c9cdac41aa93ed1b2a6345bfc182b97a8fc8bb90b47e2f57294ce97.
		events := liveCaptureEvents(t, 176, 195, "20260710 22:35:03 Central European Standard Time",
			[]captureFrame{{"client", decodeBase64Payload(t, "NzEAMgAxOTUAAA==")}},
			[]captureFrame{
				{"client", decodeBase64Payload(t, "MjAAMTAwMQAwAEFBUEwAU1RLAAAwAAAAU01BUlQAAFVTRAAAADAAADEgaG91cgAxIEQAMQBUUkFERVMAMQAwAAA=")},
				{"server", decodeBase64Payload(t, "MTcAMTAwMQAyMDI2MDcwOSAxNjozNTowMyBVUy9FYXN0ZXJuADIwMjYwNzEwIDE2OjM1OjAzIFVTL0Vhc3Rlcm4ANwAyMDI2MDcxMCAwOTozMDowMCBVUy9FYXN0ZXJuADMxNC42NgAzMTYuNDAAMzEzLjIxADMxNC4zMAAzMjkzNTUyADMxNC43NzEAMzE3MDQAMjAyNjA3MTAgMTA6MDA6MDAgVVMvRWFzdGVybgAzMTQuMzAAMzE0Ljc2ADMxMi4xNwAzMTMuODAAMzc5MTg2OAAzMTMuMjU2ADM4ODAwADIwMjYwNzEwIDExOjAwOjAwIFVTL0Vhc3Rlcm4AMzEzLjc4ADMxNC4xMQAzMTIuMzIAMzEzLjIzADI0MDMwMzMAMzEzLjQ0MwAyNTQ3MAAyMDI2MDcxMCAxMjowMDowMCBVUy9FYXN0ZXJuADMxMy4yNQAzMTQuNzgAMzEzLjA2ADMxNC43MAAxNTg4NjQ3ADMxNC4xNQAxNTg0NgAyMDI2MDcxMCAxMzowMDowMCBVUy9FYXN0ZXJuADMxNC43MQAzMTQuNzQAMzEzLjg5ADMxNC4xMwAxMTgwMTY0ADMxNC4zMzUAMTMzMDQAMjAyNjA3MTAgMTQ6MDA6MDAgVVMvRWFzdGVybgAzMTQuMTUAMzE1LjQwADMxNC4wMQAzMTUuMjcAMTUxNDU4NwAzMTQuNzcyADE2OTMyADIwMjYwNzEwIDE1OjAwOjAwIFVTL0Vhc3Rlcm4AMzE1LjI3ADMxNi45MQAzMTQuNzgAMzE1LjMzADQ4ODQ1NjYAMzE1Ljk4NgA0ODYyMQA=")},
			},
		)
		output := verifyLiveEvents(t, events)
		if !strings.Contains(output, "server_version=195") ||
			!strings.Contains(output, "server_msg_ids: 17:1,195:1") ||
			!strings.Contains(output, "end_markers: InHistoricalDataEnd:1") {
			t.Fatalf("verification output missing inline historical completion:\n%s", output)
		}
	})

	t.Run("api error", func(t *testing.T) {
		// Exact global-cancel and API-error frames retained in
		// order_lifecycle_sv203_live.txt from capture sha256
		// 8efd714c3885da232215b0f4f4bb661ac7f4364126d4c97f4200dfa71320c55d.
		events := liveCaptureEvents(t, 176, 203, "20260710 18:09:12 Central European Standard Time",
			[]captureFrame{{"client", decodeHex(t, "00000047320039320000")}},
			[]captureFrame{
				{"client", decodeOuterFrame(t, "AAAABgAAAQIKAA==")},
				{"server", decodeOuterFrame(t, "AAAAZgAAAMwIwAMQrZep5vQzGKEBIlNDYW5jZWwgYXR0ZW1wdGVkIHdoZW4gb3JkZXIgaXMgbm90IGluIGEgY2FuY2VsbGFibGUgc3RhdGUuICBPcmRlciBwZXJtSWQgPTkwMDAwMDAwMQ==")},
			},
		)
		output := verifyLiveEvents(t, events)
		if !strings.Contains(output, "server_version=203") ||
			!strings.Contains(output, "code=161") ||
			!strings.Contains(output, "client_msg_ids: 58:1,71:1,v176..203:1") ||
			!strings.Contains(output, "server_msg_ids: 4:1,203:1") {
			t.Fatalf("verification output missing protobuf API error:\n%s", output)
		}
	})
}

type captureFrame struct {
	direction string
	payload   []byte
}

func liveCaptureEvents(
	t *testing.T,
	minVersion int,
	serverVersion int,
	connectionTime string,
	bootstrap []captureFrame,
	session []captureFrame,
) []capturelog.Event {
	t.Helper()
	versionFrame, err := frameBytes([]byte(fmt.Sprintf("v%d..%d", minVersion, serverVersion)))
	if err != nil {
		t.Fatal(err)
	}
	serverInfo, err := frameBytes(wire.EncodeFields([]string{
		fmt.Sprint(serverVersion),
		connectionTime,
	}))
	if err != nil {
		t.Fatal(err)
	}
	events := []capturelog.Event{
		{Kind: capturelog.EventConnect, Leg: 1},
		captureChunk("client", append([]byte("API\x00"), versionFrame...)),
		captureChunk("server", serverInfo),
	}
	for _, frame := range append(bootstrap, session...) {
		outer, err := frameBytes(frame.payload)
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, captureChunk(frame.direction, outer))
	}
	return append(events, capturelog.Event{Kind: capturelog.EventDisconnect, Leg: 1})
}

func captureChunk(direction string, data []byte) capturelog.Event {
	return capturelog.Event{
		Kind:      capturelog.EventChunk,
		Leg:       1,
		Direction: direction,
		Length:    len(data),
		Data:      base64.StdEncoding.EncodeToString(data),
	}
}

func verifyLiveEvents(t *testing.T, events []capturelog.Event) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	// #nosec G304 -- the path is beneath this test's temporary directory.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(file)
	for _, event := range events {
		if err := encoder.Encode(event); err != nil {
			if closeErr := file.Close(); closeErr != nil {
				t.Errorf("close events after encode failure: %v", closeErr)
			}
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	replayEvents, err := capturelog.NormalizeEvents(events)
	if err != nil {
		t.Fatal(err)
	}
	var first, second bytes.Buffer
	if err := writeVerification(&first, dir, events, replayEvents); err != nil {
		t.Fatal(err)
	}
	if err := writeVerification(&second, dir, events, replayEvents); err != nil {
		t.Fatal(err)
	}
	if first.String() != second.String() {
		t.Fatalf("verification output is unstable:\nfirst:\n%s\nsecond:\n%s", first.String(), second.String())
	}
	contents, err := os.ReadFile(path) // #nosec G304 -- path is beneath the test's temporary directory.
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(contents)
	if want := fmt.Sprintf("sha256=%x", hash[:8]); !strings.Contains(first.String(), want) {
		t.Fatalf("verification output missing %q:\n%s", want, first.String())
	}
	return first.String()
}

func decodeOuterFrame(t *testing.T, value string) []byte {
	t.Helper()
	frame, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := wire.ReadFrame(bytes.NewReader(frame))
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func decodeBase64Payload(t *testing.T, value string) []byte {
	t.Helper()
	payload, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func decodeHex(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

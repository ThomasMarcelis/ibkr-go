package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/capturelog"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
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

func TestRunNormalizeVerifyDoesNotWriteArtifacts(t *testing.T) {
	t.Parallel()

	events := liveCaptureEvents(t, 100, 200, "20260611 09:40:46 Central European Standard Time",
		[]captureFrame{{"client", wire.EncodeFields([]string{"71", "2", "1", ""})}},
		[]captureFrame{
			{"client", wire.EncodeFields([]string{"49", "1"})},
			{"server", wire.EncodeFields([]string{"49", "1", "1781163646"})},
		},
	)
	dir := t.TempDir()
	writeCaptureFiles(t, dir, events)

	before, err := snapshotFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runNormalize(&output, dir, "", "", "", true); err != nil {
		t.Fatalf("runNormalize(-verify) error = %v", err)
	}
	after, err := snapshotFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("capture directory changed during verification\nbefore:\n%s\nafter:\n%s", before, after)
	}
	for _, path := range []string{"raw.txt", "replay"} {
		if _, err := os.Stat(filepath.Join(dir, path)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("verification created %s: %v", path, err)
		}
	}
}

func TestVerifyDriverEventsBindsScenarioRunAndSuccessfulLifecycle(t *testing.T) {
	t.Parallel()

	meta := capturelog.Meta{Scenario: "quote", ListenAddr: "127.0.0.1:4101", ClientID: 7}
	start := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
	valid := []driverEvidence{
		{At: start, Scenario: "quote", RunID: "run-1", Kind: "scenario_start", Server: meta.ListenAddr, ClientID: meta.ClientID},
		{At: start.Add(time.Second), Scenario: "quote", RunID: "run-1", Kind: "session_ready"},
		{At: start.Add(2 * time.Second), Scenario: "quote", RunID: "run-1", Kind: "quote_snapshot"},
		{At: start.Add(3 * time.Second), Scenario: "quote", RunID: "run-1", Kind: "scenario_end"},
	}
	path := filepath.Join(t.TempDir(), "driver_events.jsonl")
	writeDriverEvidence(t, path, valid)
	stats, err := verifyDriverEvents(path, meta)
	if err != nil {
		t.Fatal(err)
	}
	if stats.count != 4 || stats.runID != "run-1" || stats.outcomes != 1 {
		t.Fatalf("driver stats = %+v", stats)
	}

	for _, test := range []struct {
		name   string
		mutate func([]driverEvidence)
	}{
		{"scenario mismatch", func(events []driverEvidence) { events[2].Scenario = "other" }},
		{"run mismatch", func(events []driverEvidence) { events[2].RunID = "run-2" }},
		{"failed end", func(events []driverEvidence) { events[3].Error = "timeout" }},
		{"missing end", func(events []driverEvidence) { events[3].Kind = "quote_update" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			events := append([]driverEvidence(nil), valid...)
			test.mutate(events)
			path := filepath.Join(t.TempDir(), "driver_events.jsonl")
			writeDriverEvidence(t, path, events)
			if _, err := verifyDriverEvents(path, meta); err == nil {
				t.Fatal("verifyDriverEvents() error = nil")
			}
		})
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
	writeCaptureFiles(t, dir, events)
	path := filepath.Join(dir, "events.jsonl")
	replayEvents, err := capturelog.NormalizeEvents(events)
	if err != nil {
		t.Fatal(err)
	}
	var first, second bytes.Buffer
	meta := capturelog.Meta{Scenario: "test", StartedAt: time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)}
	if err := writeVerification(&first, dir, meta, events, replayEvents); err != nil {
		t.Fatal(err)
	}
	if err := writeVerification(&second, dir, meta, events, replayEvents); err != nil {
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
	if want := fmt.Sprintf("sha256=%x", hash[:]); !strings.Contains(first.String(), want) {
		t.Fatalf("verification output missing %q:\n%s", want, first.String())
	}
	return first.String()
}

func writeCaptureFiles(t *testing.T, dir string, events []capturelog.Event) {
	t.Helper()
	meta := capturelog.Meta{Scenario: "test", StartedAt: time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)}
	metaData, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), append(metaData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
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
}

func writeDriverEvidence(t *testing.T, path string, events []driverEvidence) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) // #nosec G304 -- test-owned temporary path.
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(file)
	for _, event := range events {
		if err := encoder.Encode(event); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func snapshotFiles(root string) ([]byte, error) {
	rootDir, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer rootDir.Close()

	var out strings.Builder
	err = fs.WalkDir(rootDir.FS(), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := rootDir.ReadFile(path)
		if err != nil {
			return err
		}
		fmt.Fprintf(&out, "%s %x\n", path, sha256.Sum256(data))
		return nil
	})
	return []byte(out.String()), err
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

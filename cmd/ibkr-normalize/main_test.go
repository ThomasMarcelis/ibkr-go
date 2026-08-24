package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/capturelog"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
)

func TestCaptureFrameStateServer225(t *testing.T) {
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
		{"client", []byte("v208..225"), frameDescription{label: "version_range", encoding: "pre_session"}},
		{"server", wire.EncodeFields([]string{"225", "20260825 00:44:27 CET"}), frameDescription{label: "server_info", encoding: "pre_session", serverVersion: 225, connectionTime: "20260825 00:44:27 CET"}},
		{"client", decodeOuterFrame(t, "AAAABgAAAQ8IAQ=="), frameDescription{msgID: 71, encoding: "protobuf", serverVersion: 225, session: true}},
		{"client", decodeOuterFrame(t, "AAAACAAAAM8IARIA"), frameDescription{msgID: 7, encoding: "protobuf", serverVersion: 225, session: true}},
		{"server", decodeOuterFrame(t, "AAAABgAAAP8IAQ=="), frameDescription{msgID: 55, encoding: "protobuf", serverVersion: 225, session: true}},
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

func TestTranscriptSkeletonEmitsExecutableSanitizedReplay(t *testing.T) {
	t.Parallel()

	events := liveCaptureEvents(t, 208, 225, "20260824 22:27:46 CET",
		[]captureFrame{
			{"client", decodeOuterFrame(t, "AAAABgAAAQ8IAQ==")},
			{"server", decodeOuterFrame(t, "AAAADwAAANcKCVpaMTIzNDU2Nw==")},
			{"server", decodeOuterFrame(t, "AAAABgAAANEIAQ==")},
		},
		[]captureFrame{
			{"client", decodeOuterFrame(t, "AAAABAAAAPk=")},
			{"server", decodeOuterFrame(t, "AAAACgAAAPkIwtKy1AY=")},
		},
	)
	dir := t.TempDir()
	writeCaptureFiles(t, dir, events)
	path := filepath.Join(dir, "transcript.txt")
	var output bytes.Buffer
	if err := runNormalize(&output, dir, filepath.Join(dir, "raw.txt"), filepath.Join(dir, "replay"), path, false); err != nil {
		t.Fatal(err)
	}
	// #nosec G304 -- path is constructed under this test's t.TempDir.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"events.jsonl sha256:",
		"handshake {\"server_version\":225,\"connection_time\":\"20260824 22:27:46 CET\"}",
		"raw client AAAABAAAAPk=",
		"disconnect",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("transcript missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "msg_id=version_range") || strings.Contains(text, "msg_id=71") {
		t.Fatalf("transcript retained pre-session or START_API frame:\n%s", text)
	}
	var foundCanonicalAccount bool
	for line := range strings.Lines(text) {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "raw ") {
			continue
		}
		frame, err := base64.StdEncoding.DecodeString(strings.Fields(line)[2])
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(frame, []byte("ZZ1234567")) {
			t.Fatalf("raw frame retained live account: %x", frame)
		}
		foundCanonicalAccount = foundCanonicalAccount || bytes.Contains(frame, []byte("DU9000001"))
	}
	if !foundCanonicalAccount {
		t.Fatal("raw frames lack canonical account DU9000001")
	}
}

func TestTrackedTranscriptFramesDeriveFromDeclaredCaptures(t *testing.T) {
	if _, err := os.Stat("../../captures"); err != nil {
		if os.IsNotExist(err) {
			t.Skip("local raw capture corpus is not present")
		}
		t.Fatal(err)
	}
	files, err := filepath.Glob("../../testdata/transcripts/*.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("tracked transcript corpus is empty")
	}
	capturePattern := regexp.MustCompile(`\b[0-9]{8}T[0-9]{6}Z-[A-Za-z0-9][A-Za-z0-9_-]*\b`)
	for _, file := range files {
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(file) // #nosec G304 -- fixed transcript glob.
			if err != nil {
				t.Fatal(err)
			}
			header := data
			if index := bytes.IndexByte(header, '\n'); index >= 0 {
				for offset := index + 1; offset < len(header); {
					next := bytes.IndexByte(header[offset:], '\n')
					end := len(header)
					if next >= 0 {
						end = offset + next
					}
					if !bytes.HasPrefix(header[offset:end], []byte("#")) {
						header = header[:offset]
						break
					}
					if next < 0 {
						break
					}
					offset = end + 1
				}
			}
			captureIDs := capturePattern.FindAllString(string(header), -1)
			if len(captureIDs) == 0 {
				t.Fatal("transcript header has no capture ID")
			}
			if declaration := lineageTransformationDeclaration(name); declaration != "" && !bytes.Contains(header, []byte(declaration)) {
				t.Fatalf("fault-injection fixture lacks bounded transformation declaration %q", declaration)
			}
			sourceFrames := make(map[string]struct{})
			var sourceSequence []string
			seenCapture := make(map[string]struct{})
			for _, captureID := range captureIDs {
				if _, ok := seenCapture[captureID]; ok {
					continue
				}
				seenCapture[captureID] = struct{}{}
				addSanitizedCaptureFrames(t, sourceFrames, &sourceSequence, filepath.Join("../../captures", captureID), name)
			}

			var transcriptSequence []string
			transcriptLegs := [][]string{{}}
			for line := range strings.Lines(string(data)) {
				line = strings.TrimSpace(line)
				if line == "disconnect" {
					if len(transcriptLegs[len(transcriptLegs)-1]) > 0 {
						transcriptLegs = append(transcriptLegs, nil)
					}
					continue
				}
				if !strings.HasPrefix(line, "raw ") {
					continue
				}
				fields := strings.Fields(line)
				if len(fields) != 3 || fields[1] != "client" && fields[1] != "server" {
					t.Fatalf("invalid raw transcript line %q", line)
				}
				key := fields[1] + "\x00" + fields[2]
				if _, ok := sourceFrames[key]; !ok {
					t.Fatalf("%s frame is not a field-aware sanitized frame from its declared capture", fields[1])
				}
				transcriptSequence = append(transcriptSequence, key)
				transcriptLegs[len(transcriptLegs)-1] = append(transcriptLegs[len(transcriptLegs)-1], key)
			}
			if len(transcriptLegs[len(transcriptLegs)-1]) == 0 {
				transcriptLegs = transcriptLegs[:len(transcriptLegs)-1]
			}
			if name == "quote_stream_reconnect.txt" {
				if len(transcriptLegs) != 2 {
					t.Fatalf("projected reconnect fixture has %d legs, want 2", len(transcriptLegs))
				}
				for i, leg := range transcriptLegs {
					if !isOrderedFrameSubsequence(sourceSequence, leg) {
						t.Fatalf("projected leg %d is not an ordered subsequence of the field-aware sanitized capture", i+1)
					}
				}
				// The declared fault injection reuses exactly the captured bootstrap
				// and quote-request scaffolding on the replacement connection. Its
				// distinct data tails must still consume the source capture in order;
				// otherwise two independently valid legs could reverse live events.
				if got := sharedFrameCount(transcriptLegs[0], transcriptLegs[1]); got != 4 {
					t.Fatalf("projected reconnect fixture reuses %d frames across legs, want 4 setup frames", got)
				}
				if !isOrderedFrameSubsequence(sourceSequence, distinctFrames(transcriptLegs[0], transcriptLegs[1])) {
					t.Fatal("projected reconnect data tails are not ordered across connection legs")
				}
			} else if !isOrderedFrameSubsequence(sourceSequence, transcriptSequence) {
				t.Fatal("transcript frames are not an ordered subsequence of the field-aware sanitized capture")
			}
		})
	}
}

func addSanitizedCaptureFrames(t *testing.T, frames map[string]struct{}, sequence *[]string, captureDir, transcriptName string) {
	t.Helper()
	events, err := capturelog.LoadEvents(filepath.Join(captureDir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	replayEvents, err := capturelog.NormalizeEvents(events)
	if err != nil {
		t.Fatal(err)
	}
	identities, err := loadTranscriptRedactionIdentities(
		filepath.Join(captureDir, "driver_events.jsonl"),
		filepath.Join(captureDir, "transcript_redactions.jsonl"),
	)
	if err != nil {
		t.Fatal(err)
	}
	serverIdentities, err := transcriptServerIdentities(replayEvents)
	if err != nil {
		t.Fatal(err)
	}
	redactions, err := transcriptRedactionsForIdentities(append(identities, serverIdentities...))
	if err != nil {
		t.Fatal(err)
	}
	state := newCaptureFrameState()
	for _, event := range replayEvents {
		switch event.Kind {
		case capturelog.EventConnect:
			if err := state.connect(event.Leg); err != nil {
				t.Fatal(err)
			}
		case capturelog.EventDisconnect:
			if err := state.disconnect(event.Leg); err != nil {
				t.Fatal(err)
			}
		case capturelog.ReplayEventFrame:
			payload, err := base64.StdEncoding.DecodeString(event.Data)
			if err != nil {
				t.Fatal(err)
			}
			description, err := state.describe(event, payload)
			if err != nil {
				t.Fatal(err)
			}
			if !description.session {
				continue
			}
			payload, err = redactions.applyFrame(event.Direction, description, payload)
			if err != nil {
				t.Fatalf("sanitize %s leg %d msg_id %s: %v", captureDir, event.Leg, description.messageID(), err)
			}
			framed, err := frameBytes(payload)
			if err != nil {
				t.Fatal(err)
			}
			encoded := base64.StdEncoding.EncodeToString(framed)
			key := event.Direction + "\x00" + encoded
			frames[key] = struct{}{}
			*sequence = append(*sequence, key)
		}
	}
}

func isOrderedFrameSubsequence(source, transcript []string) bool {
	index := 0
	for _, frame := range source {
		if index < len(transcript) && frame == transcript[index] {
			index++
		}
	}
	return index == len(transcript)
}

func sharedFrameCount(first, second []string) int {
	firstFrames := make(map[string]struct{}, len(first))
	for _, frame := range first {
		firstFrames[frame] = struct{}{}
	}
	var count int
	for _, frame := range second {
		if _, ok := firstFrames[frame]; ok {
			count++
		}
	}
	return count
}

func distinctFrames(first, second []string) []string {
	counts := make(map[string]int, len(first)+len(second))
	for _, frame := range append(append([]string(nil), first...), second...) {
		counts[frame]++
	}
	result := make([]string, 0, len(first)+len(second))
	for _, frame := range append(append([]string(nil), first...), second...) {
		if counts[frame] == 1 {
			result = append(result, frame)
		}
	}
	return result
}

func lineageTransformationDeclaration(name string) string {
	switch name {
	case "quote_stream_reconnect.txt":
		return "projected onto two connection legs"
	default:
		return ""
	}
}

func TestFormatTranscriptServerVersions(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		versions []int
		want     string
		wantErr  bool
	}{
		{name: "single", versions: []int{225}, want: "server_version 225"},
		{name: "consecutive matrix", versions: []int{208, 209, 210, 211}, want: "server_versions 208-211"},
		{name: "empty", wantErr: true},
		{name: "gap", versions: []int{208, 210}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := formatTranscriptServerVersions(tc.versions)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("formatTranscriptServerVersions(%v) error = nil", tc.versions)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("formatTranscriptServerVersions(%v) = %q, want %q", tc.versions, got, tc.want)
			}
		})
	}
}

func TestTranscriptRedactionsPreserveRecordedWidths(t *testing.T) {
	t.Parallel()

	const driver = `{"account":"ZZ7654321","order_id":557,"order_ref":"ibkrgo-c31d8cfc-20260824T210402Z-001","perm_id":1841516325,"oca_group":"campaign-private-000000001","submitter":"papermarcelis"}` + "\n"
	path := filepath.Join(t.TempDir(), "driver_events.jsonl")
	if err := os.WriteFile(path, []byte(driver), 0o600); err != nil {
		t.Fatal(err)
	}
	redactions, err := loadTranscriptRedactions(path)
	if err != nil {
		t.Fatal(err)
	}
	openOrder := decodeOuterFrame(t, "AAABMAAAAM0IrQQSLwj+mhASBEFBUEwaA1NUSykAAAAAAAAAAEIFU01BUlRSA1VTRFoEQUFQTGIDTk1TGuUBCAEQrQQYrbjEwyEgACoDQlVZMgExQgNMTVRJj8L1KFwPL0BRAAAAAAAAAABaA0RBWWIJRFU5MDAwMDAxegJJQuIBJHNhbml0aXplZC1vcmRlci1yZWYtMDAwMDAwMDAwMDAwMDAwMfABA/gBALACAMACAMoCBE5vbmXwAgCoBACwBADABP///////////wHqBQROb25lkAYA+AYBmgcBMLIHKU5vdCBhbiBpbnNpZGVyIG9yIHN1YnN0YW50aWFsIHNoYXJlaG9sZGVywAcA0AcAyggNcGFwZXItdXNlci0wMfAIACIOCgxQcmVTdWJtaXR0ZWQ=")
	openOrder = replaceCapturedValue(t, openOrder, []byte("DU9000001"), []byte("ZZ7654321"))
	openOrder = replaceCapturedValue(t, openOrder, []byte("sanitized-order-ref-0000000000000001"), []byte("ibkrgo-c31d8cfc-20260824T210402Z-001"))
	openOrder = replaceCapturedValue(t, openOrder, []byte("paper-user-01"), []byte("papermarcelis"))
	openOrder = replaceCapturedValue(t, openOrder, decodeHex(t, "adb8c4c321"), decodeHex(t, "a59e8dee06"))
	gotOpenOrder, err := redactions.applyFrame("server", frameDescription{
		msgID: protocol.InOpenOrder, encoding: "protobuf", serverVersion: 225, session: true,
	}, openOrder)
	if err != nil {
		t.Fatal(err)
	}

	placeOrder := decodeOuterFrame(t, "AAAAtQAAAMsI+AMSGwj+mhASBEFBUEwaA1NUS0IFU01BUlRSA1VTRBqMASAAKgNCVVkyATE4AEIDTE1USQAAAAAAACRAWgNEQVliCURVOTAwMDAwMdoBGm9jYS0wMDAwMDAwMDAwMDAwMDAwMDAwMDAx4gEkc2FuaXRpemVkLW9yZGVyLXJlZi0wMDAwMDAwMDAwMDAwMDAx8AEB+AEAkAQBqAQAwAT///////////8BkAYAygYAIgA=")
	placeOrder = replaceCapturedValue(t, placeOrder, []byte("oca-0000000000000000000001"), []byte("campaign-private-000000001"))
	gotPlaceOrder, err := redactions.applyFrame("client", frameDescription{
		msgID: protocol.OutPlaceOrder, encoding: "protobuf", serverVersion: 225, session: true,
	}, placeOrder)
	if err != nil {
		t.Fatal(err)
	}

	got := bytes.Join([][]byte{gotOpenOrder, gotPlaceOrder}, nil)
	for _, secret := range [][]byte{
		[]byte("ZZ7654321"),
		[]byte("ibkrgo-c31d8cfc-20260824T210402Z-001"),
		[]byte("1841516325"),
		[]byte("campaign-private-000000001"),
		[]byte("papermarcelis"),
		decodeHex(t, "a59e8dee06"),
	} {
		if bytes.Contains(got, secret) {
			t.Fatalf("redacted payload retained %q: %x", secret, got)
		}
	}
	if len(gotOpenOrder) != len(openOrder) || len(gotPlaceOrder) != len(placeOrder) {
		t.Fatalf("redacted payload lengths = (%d, %d), want (%d, %d)", len(gotOpenOrder), len(gotPlaceOrder), len(openOrder), len(placeOrder))
	}
	if !bytes.Contains(got, []byte("DU9000001")) ||
		!bytes.Contains(got, []byte("paper-user-01")) ||
		!bytes.Contains(got, []byte("oca-0000000000000000000001")) {
		t.Fatalf("redacted payload lacks canonical identities: %q", got)
	}
	if !bytes.Contains(got, decodeHex(t, "adb8c4c321")) {
		t.Fatalf("redacted payload lacks width-preserving permanent ID: %x", got)
	}
}

func TestTranscriptRedactionsPreserveNineDigitPermID(t *testing.T) {
	t.Parallel()

	const driver = `{"order_id":456,"perm_id":312255385}` + "\n"
	path := filepath.Join(t.TempDir(), "driver_events.jsonl")
	if err := os.WriteFile(path, []byte(driver), 0o600); err != nil {
		t.Fatal(err)
	}
	redactions, err := loadTranscriptRedactions(path)
	if err != nil {
		t.Fatal(err)
	}
	payload := decodeOuterFrame(t, "AAAAQAAAAMsIyAMSDFByZVN1Ym1pdHRlZBoBMCIBMSkAAAAAAAAAADDI1ZOtAzgAQQAAAAAAAAAASAFZAAAAAAAAAAA=")
	payload = replaceCapturedValue(t, payload, decodeHex(t, "c8d593ad03"), decodeHex(t, "99c7f29401"))
	got, err := redactions.applyFrame("server", frameDescription{
		msgID: protocol.InOrderStatus, encoding: "protobuf", serverVersion: 225, session: true,
	}, payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(payload) {
		t.Fatalf("redacted payload length = %d, want %d", len(got), len(payload))
	}
	if bytes.Contains(got, []byte("312255385")) || bytes.Contains(got, decodeHex(t, "99c7f29401")) {
		t.Fatalf("redacted payload retained permanent id: %x", got)
	}
	if !bytes.Contains(got, decodeHex(t, "c8d593ad03")) {
		t.Fatalf("redacted payload lacks width-preserving permanent id: %x", got)
	}
}

func TestTranscriptRedactionsCorrelateOCAGroupBeforeParentIdentity(t *testing.T) {
	t.Parallel()

	redactions, err := transcriptRedactionsForIdentities([]transcriptDriverIdentity{
		{OCAGroup: "312255385"},
		{OrderID: 456, PermID: 312255385},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, replacement := range redactions.replacements {
		if replacement.kind == transcriptOCAGroup {
			t.Fatalf("correlated OCA group was independently tokenized: %#v", replacement)
		}
	}
}

func TestTranscriptRedactionsRejectIdentityCollisionsOutsideApprovedFields(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "driver_events.jsonl")
	if err := os.WriteFile(path, []byte(`{"account":"ZZ7654321"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	redactions, err := loadTranscriptRedactions(path)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("protobuf", func(t *testing.T) {
		payload := decodeOuterFrame(t, "AAAALgAAAQcIARIJRFU5MDAwMDAxGgtCdXlpbmdQb3dlciIJMTgzODc1LjIyKgNFVVI=")
		payload = replaceCapturedValue(t, payload, []byte("183875.22"), []byte("ZZ7654321"))
		original := append([]byte(nil), payload...)
		if _, err := redactions.applyFrame("server", frameDescription{
			msgID: protocol.InAccountSummary, encoding: "protobuf", serverVersion: 225, session: true,
		}, payload); err == nil {
			t.Fatal("applyFrame() error = nil, want unapproved-path rejection")
		}
		if !bytes.Equal(payload, original) {
			t.Fatal("applyFrame mutated rejected protobuf input")
		}
	})

	t.Run("classic", func(t *testing.T) {
		payload := decodeOuterFrame(t, "AAAAEQAAADExADE3ODc2MDc1NjkA")
		payload = replaceCapturedValue(t, payload, []byte("1787607569"), []byte("XZZ7654321"))
		original := append([]byte(nil), payload...)
		if _, err := redactions.applyFrame("server", frameDescription{
			msgID: protocol.InCurrentTime, encoding: "classic", serverVersion: 208, session: true,
		}, payload); err == nil {
			t.Fatal("applyFrame() error = nil, want unapproved-field rejection")
		}
		if !bytes.Equal(payload, original) {
			t.Fatal("applyFrame mutated rejected classic input")
		}
	})
}

func replaceCapturedValue(t *testing.T, payload, from, to []byte) []byte {
	t.Helper()
	if len(from) != len(to) {
		t.Fatalf("captured replacement width = %d -> %d", len(from), len(to))
	}
	if count := bytes.Count(payload, from); count != 1 {
		t.Fatalf("captured replacement occurs %d times, want exactly once", count)
	}
	return bytes.Replace(payload, from, to, 1)
}

func TestCaptureFrameStateRejectsMalformedAndOutOfOrderHandshake(t *testing.T) {
	t.Parallel()

	serverInfo := wire.EncodeFields([]string{"225", "20260825 00:44:27 CET"})
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
		version, err := state.describe(capturelog.ReplayEvent{Leg: 1, Direction: "client"}, []byte("v208..225"))
		if err != nil {
			t.Fatalf("cycle %d version range: %v", cycle, err)
		}
		if version.label != "version_range" {
			t.Fatalf("cycle %d version label = %q", cycle, version.label)
		}
		if _, err := state.describe(
			capturelog.ReplayEvent{Leg: 1, Direction: "server"},
			wire.EncodeFields([]string{"225", "20260825 00:44:27 CET"}),
		); err != nil {
			t.Fatalf("cycle %d server info: %v", cycle, err)
		}
		if err := state.disconnect(1); err != nil {
			t.Fatalf("cycle %d disconnect: %v", cycle, err)
		}
	}
}

func TestWriteVerificationCurrentTimeSV225Capture(t *testing.T) {
	t.Parallel()

	// Exact frames retained in current_time_live.txt from readonly-live capture
	// 20260824T202747Z-current_time at server_version 225, events.jsonl SHA-256
	// a9029ff8e7cfed19cab1e3e2eccc4c36d7c91b95aa6aa03f75543bacac454a9e.
	events := liveCaptureEvents(t, 208, 225, "20260824 22:27:46 CET",
		[]captureFrame{{"client", decodeOuterFrame(t, "AAAABgAAAQ8IAQ==")}},
		[]captureFrame{
			{"client", decodeOuterFrame(t, "AAAABAAAAPk=")},
			{"server", decodeOuterFrame(t, "AAAACgAAAPkIwtKy1AY=")},
		},
	)
	output := verifyLiveEvents(t, events)
	if !strings.Contains(output, "server_version=225") ||
		!strings.Contains(output, "client_msg_ids: 49:1,71:1,v208..225:1") ||
		!strings.Contains(output, "server_msg_ids: 49:1,225:1") {
		t.Fatalf("verification output missing current-time evidence:\n%s", output)
	}
}

func TestRunNormalizeVerifyDoesNotWriteArtifacts(t *testing.T) {
	t.Parallel()

	events := liveCaptureEvents(t, 208, 225, "20260824 22:27:46 CET",
		[]captureFrame{{"client", decodeOuterFrame(t, "AAAABgAAAQ8IAQ==")}},
		[]captureFrame{
			{"client", decodeOuterFrame(t, "AAAABAAAAPk=")},
			{"server", decodeOuterFrame(t, "AAAACgAAAPkIwtKy1AY=")},
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
	if err := os.WriteFile(filepath.Join(dir, "driver.log"), []byte("--- FAIL: TestLiveScenario\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runNormalize(io.Discard, dir, "", "", "", true); err == nil || !strings.Contains(err.Error(), "incomplete driver evidence") {
		t.Fatalf("verification with driver.log but no driver_events.jsonl error = %v", err)
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
	stats, err := verifyDriverEvents(path, meta, nil)
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
		{"paper baseline without reconciliation", func(events []driverEvidence) { events[2].Kind = "paper_baseline" }},
		{"failed reconciliation", func(events []driverEvidence) {
			events[2].Kind = "paper_reconciliation_failed"
			events[2].Error = "positions unknown"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			events := append([]driverEvidence(nil), valid...)
			test.mutate(events)
			path := filepath.Join(t.TempDir(), "driver_events.jsonl")
			writeDriverEvidence(t, path, events)
			if _, err := verifyDriverEvents(path, meta, nil); err == nil {
				t.Fatal("verifyDriverEvents() error = nil")
			}
		})
	}

	lifecycleOnly := append([]driverEvidence(nil), valid[:2]...)
	lifecycleOnly = append(lifecycleOnly, valid[3])
	path = filepath.Join(t.TempDir(), "driver_events.jsonl")
	writeDriverEvidence(t, path, lifecycleOnly)
	if _, err := verifyDriverEvents(path, meta, nil); err == nil {
		t.Fatal("verifyDriverEvents() accepted a non-bootstrap scenario with no outcome")
	}
	meta.Scenario = "bootstrap"
	for i := range lifecycleOnly {
		lifecycleOnly[i].Scenario = "bootstrap"
	}
	path = filepath.Join(t.TempDir(), "driver_events.jsonl")
	writeDriverEvidence(t, path, lifecycleOnly)
	if _, err := verifyDriverEvents(path, meta, nil); err != nil {
		t.Fatalf("verifyDriverEvents() rejected bootstrap lifecycle: %v", err)
	}
}

func TestValidateDriverScenarioEvidenceIncludeOvernight(t *testing.T) {
	t.Parallel()

	for _, echo := range []string{"absent", "false"} {
		t.Run(echo, func(t *testing.T) {
			t.Parallel()
			stats := driverEvidenceStats{
				kinds: map[string]int{"paper_baseline": 1, "paper_reconciled": 1},
				events: []driverEvidence{
					{Kind: "include_overnight_echo", Label: "placement", Values: map[string]string{"include_overnight": "true"}},
					{Kind: "include_overnight_blocked", Label: "replacement", Values: map[string]string{"code": "462"}},
					{Kind: "include_overnight_echo", Label: "fresh placement", Values: map[string]string{
						"requested": "false", "include_overnight": echo, "tif": "DAY",
					}},
				},
			}
			if err := validateDriverScenarioEvidence("api_include_overnight_lifecycle_aapl", stats); err != nil {
				t.Fatal(err)
			}
			stats.events[2].Values["tif"] = "OVERNIGHT + DAY"
			if err := validateDriverScenarioEvidence("api_include_overnight_lifecycle_aapl", stats); err == nil {
				t.Fatal("validator accepted a fresh false placement whose broker echo retained overnight TIF")
			}
		})
	}
}

func TestValidateDriverScenarioEvidenceOptionExercise(t *testing.T) {
	t.Parallel()

	stats := driverEvidenceStats{
		kinds: map[string]int{"paper_baseline": 1, "paper_reconciled": 1},
		events: []driverEvidence{{
			Kind: "order_warning", Label: "option exercise seed",
			Values: map[string]string{
				"code":    "399",
				"message": "Warning: Your order will not be placed at the exchange until 2026-08-26 09:30:00 US/Eastern.",
			},
		}},
	}
	if err := validateDriverScenarioEvidence("api_option_exercise_aapl", stats); err != nil {
		t.Fatal(err)
	}
	stats.events[0].Values["code"] = "0"
	if err := validateDriverScenarioEvidence("api_option_exercise_aapl", stats); err == nil {
		t.Fatal("validator accepted a non-attested market-hours blocker")
	}
}

func TestAttestedScenarioBlockersRequireExactRawAPIEvidence(t *testing.T) {
	t.Parallel()

	// Exact code and message from
	// captures/20260824T202748Z-histogram_data_aapl, server_version 225,
	// events.jsonl SHA-256
	// 696c022d5a3355ae82b3dc994086ec557156bb4da8e691b1ec514db327af5081.
	apiErr := codec.APIError{
		ReqID:   1,
		Code:    2188,
		Message: "Up-to-the-second historical data requires additional subscription for the API.",
	}
	scenarioErr := "ibkr: api histogram_data code=2188 conn=1: " + apiErr.Message
	if !isAttestedScenarioBlocker("histogram_data_aapl", scenarioErr, []codec.APIError{apiErr}) {
		t.Fatal("exact captured blocker was rejected")
	}
	if isAttestedScenarioBlocker("histogram_data_aapl", scenarioErr, []codec.APIError{{ReqID: 1, Code: 2188, Message: "different"}}) {
		t.Fatal("mismatched raw blocker was accepted")
	}
	if isAttestedScenarioBlocker("other", scenarioErr, []codec.APIError{apiErr}) {
		t.Fatal("blocker was accepted for an unrelated scenario")
	}

	// Exact code and message shape from
	// captures/20260825T210100Z-api_option_exercise_aapl, server_version 225,
	// events.jsonl SHA-256
	// a10ff5818916cad50192579a39ce046143a1123a5a26f51bf359f161a0b5ad2c.
	optionErr := codec.APIError{
		ReqID: 7,
		Code:  399,
		Message: "Order Message:\nBUY 1 AAPL AUG 26 '26 302.5 Call (AAPL  260826C00302500) \n" +
			"Warning: Your order will not be placed at the exchange until 2026-08-26 09:30:00 US/Eastern.",
	}
	optionScenarioErr := "option exercise seed status=Cancelled filled=0 execution=false, want one-contract terminal fill"
	if !isAttestedScenarioBlocker("api_option_exercise_aapl", optionScenarioErr, []codec.APIError{optionErr}) {
		t.Fatal("exact option market-hours blocker was rejected")
	}
	optionErr.Code = 201
	if isAttestedScenarioBlocker("api_option_exercise_aapl", optionScenarioErr, []codec.APIError{optionErr}) {
		t.Fatal("mismatched option blocker was accepted")
	}
}

func TestWriteVerificationLiveProtobufEndAndAPIError(t *testing.T) {
	t.Parallel()

	t.Run("execution end", func(t *testing.T) {
		// Exact sanitized frames retained in executions_empty.txt from
		// captures/20260824T224428Z-executions_snapshot, server_version 225,
		// events.jsonl SHA-256
		// 22ce8b4111cb2a700216eb6c3a8c1f0ab56233e7b382ab8b3eb80b67a23def0e.
		events := liveCaptureEvents(t, 208, 225, "20260825 00:44:27 CET",
			[]captureFrame{{"client", decodeOuterFrame(t, "AAAABgAAAQ8IAQ==")}},
			[]captureFrame{
				{"client", decodeOuterFrame(t, "AAAACAAAAM8IARIA")},
				{"server", decodeOuterFrame(t, "AAAABgAAAP8IAQ==")},
			},
		)
		output := verifyLiveEvents(t, events)
		if !strings.Contains(output, "server_version=225") ||
			!strings.Contains(output, "end_markers: InExecutionDataEnd:1") {
			t.Fatalf("verification output missing protobuf completion:\n%s", output)
		}
	})

	t.Run("api error", func(t *testing.T) {
		// Exact global-cancel and code-161 frames retained in
		// api_cross_client_cancel_aapl.txt from
		// captures/20260824T204700Z-api_cross_client_cancel_aapl,
		// server_version 225, events.jsonl SHA-256
		// 9d7fafb105cfe1cd1b12b46b6dcf00d983a20b4e3c71a14239dd441795b1f3c6.
		events := liveCaptureEvents(t, 208, 225, "20260824 22:47:00 CET",
			[]captureFrame{{"client", decodeOuterFrame(t, "AAAABgAAAQ8IAQ==")}},
			[]captureFrame{
				{"client", decodeOuterFrame(t, "AAAABgAAAQIKAA==")},
				{"server", decodeOuterFrame(t, "AAAAZgAAAMwI2gMQlpKbrIM0GKEBIlNDYW5jZWwgYXR0ZW1wdGVkIHdoZW4gb3JkZXIgaXMgbm90IGluIGEgY2FuY2VsbGFibGUgc3RhdGUuICBPcmRlciBwZXJtSWQgPTMxMjI1NTQwOQ==")},
			},
		)
		output := verifyLiveEvents(t, events)
		if !strings.Contains(output, "server_version=225") ||
			!strings.Contains(output, "code=161") ||
			!strings.Contains(output, "client_msg_ids: 58:1,71:1,v208..225:1") ||
			!strings.Contains(output, "server_msg_ids: 4:1,225:1") {
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

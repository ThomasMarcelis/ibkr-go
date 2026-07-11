package capturelog

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
)

func TestCreateAndLoadEvents(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	session, err := Create(root, Meta{Scenario: "bootstrap"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := session.Record("client", []byte("hello")); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if err := session.Record("server", []byte{0x00, 0x01, 0x02}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	events, err := LoadEvents(filepath.Join(session.Dir(), "events.jsonl"))
	if err != nil {
		t.Fatalf("LoadEvents() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events len = %d, want 2", len(events))
	}
	if events[0].Kind != EventChunk || events[0].Leg != 1 {
		t.Fatalf("events[0] = %#v, want chunk leg 1", events[0])
	}

	got, err := DecodeData(events[0])
	if err != nil {
		t.Fatalf("DecodeData() error = %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("DecodeData() = %q, want %q", string(got), "hello")
	}
}

func TestCreateDoesNotReuseSameSecondScenarioDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	meta := Meta{
		Scenario:  "repeated",
		StartedAt: time.Date(2026, time.July, 10, 20, 0, 0, 0, time.UTC),
	}
	first, err := Create(root, meta)
	if err != nil {
		t.Fatalf("Create(first) error = %v", err)
	}
	if err := first.Record("client", []byte("first")); err != nil {
		t.Fatalf("first.Record() error = %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("first.Close() error = %v", err)
	}

	second, err := Create(root, meta)
	if err != nil {
		t.Fatalf("Create(second) error = %v", err)
	}
	if first.Dir() == second.Dir() {
		t.Fatalf("capture directories both = %q, want distinct runs", first.Dir())
	}
	if err := second.Close(); err != nil {
		t.Fatalf("second.Close() error = %v", err)
	}
	events, err := LoadEvents(filepath.Join(first.Dir(), "events.jsonl"))
	if err != nil {
		t.Fatalf("LoadEvents(first) error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("first events len = %d, want 1", len(events))
	}
	data, err := DecodeData(events[0])
	if err != nil {
		t.Fatalf("DecodeData(first) error = %v", err)
	}
	if string(data) != "first" {
		t.Fatalf("first capture data = %q, want first", data)
	}
}

func TestRedactionSpanningChunksPreservesLength(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	session, err := Create(root, Meta{Scenario: "redact"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	session.Redact("supersecret", "mask")

	if err := session.RecordChunk(1, "server", []byte("prefix-super")); err != nil {
		t.Fatalf("RecordChunk(first) error = %v", err)
	}
	if err := session.RecordChunk(1, "server", []byte("secret-suffix")); err != nil {
		t.Fatalf("RecordChunk(second) error = %v", err)
	}
	if err := session.RecordDisconnect(1); err != nil {
		t.Fatalf("RecordDisconnect() error = %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	events, err := LoadEvents(filepath.Join(session.Dir(), "events.jsonl"))
	if err != nil {
		t.Fatalf("LoadEvents() error = %v", err)
	}
	var got []byte
	for _, event := range events {
		if event.Kind != EventChunk {
			continue
		}
		chunk, err := DecodeData(event)
		if err != nil {
			t.Fatalf("DecodeData() error = %v", err)
		}
		if event.Length != len(chunk) {
			t.Fatalf("event length = %d, decoded len = %d", event.Length, len(chunk))
		}
		got = append(got, chunk...)
	}
	if bytes.Contains(got, []byte("supersecret")) {
		t.Fatalf("redacted stream still contains secret: %q", got)
	}
	if len(got) != len("prefix-supersecret-suffix") {
		t.Fatalf("redacted stream len = %d, want original len", len(got))
	}
}

func TestRedactionPreservesCrossDirectionOrder(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	session, err := Create(root, Meta{Scenario: "redact-order"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	session.Redact("supersecret", "mask")

	if err := session.RecordChunk(1, "client", []byte("c")); err != nil {
		t.Fatalf("RecordChunk(client) error = %v", err)
	}
	if err := session.RecordChunk(1, "server", []byte("server-payload")); err != nil {
		t.Fatalf("RecordChunk(server) error = %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	events, err := LoadEvents(filepath.Join(session.Dir(), "events.jsonl"))
	if err != nil {
		t.Fatalf("LoadEvents() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events len = %d, want 2", len(events))
	}
	if events[0].Direction != "client" || events[1].Direction != "server" {
		t.Fatalf("directions = %q, %q; want client, server", events[0].Direction, events[1].Direction)
	}
}

func TestLoadMetaAndWriteReplay(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	session, err := Create(root, Meta{
		Scenario:   "bootstrap",
		ListenAddr: "127.0.0.1:4101",
		Upstream:   "127.0.0.1:4001",
		ClientID:   7,
		Notes:      "live bootstrap",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	frame := mustFrame(t, wire.EncodeFields([]string{"hello", "1"}))
	if err := session.RecordConnect(1); err != nil {
		t.Fatalf("RecordConnect() error = %v", err)
	}
	if err := session.RecordChunk(1, "client", frame[:3]); err != nil {
		t.Fatalf("RecordChunk() error = %v", err)
	}
	if err := session.RecordChunk(1, "client", frame[3:]); err != nil {
		t.Fatalf("RecordChunk() error = %v", err)
	}
	if err := session.RecordDisconnect(1); err != nil {
		t.Fatalf("RecordDisconnect() error = %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	meta, err := LoadMeta(filepath.Join(session.Dir(), "meta.json"))
	if err != nil {
		t.Fatalf("LoadMeta() error = %v", err)
	}
	if meta.ClientID != 7 {
		t.Fatalf("LoadMeta().ClientID = %d, want 7", meta.ClientID)
	}

	events, err := LoadEvents(filepath.Join(session.Dir(), "events.jsonl"))
	if err != nil {
		t.Fatalf("LoadEvents() error = %v", err)
	}
	replayEvents, err := NormalizeEvents(events)
	if err != nil {
		t.Fatalf("NormalizeEvents() error = %v", err)
	}
	replayDir := filepath.Join(session.Dir(), "replay")
	if err := WriteReplay(replayDir, session.Dir(), meta, replayEvents); err != nil {
		t.Fatalf("WriteReplay() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(replayDir, "meta.json")); err != nil {
		t.Fatalf("replay meta stat error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(replayDir, "frames.jsonl")); err != nil {
		t.Fatalf("replay frames stat error = %v", err)
	}
}

func TestNormalizeEventsReassemblesSplitFramesAndReconnectLegs(t *testing.T) {
	t.Parallel()

	frame1 := mustFrame(t, wire.EncodeFields([]string{"hello", "1"}))
	frame2 := mustFrame(t, wire.EncodeFields([]string{"managed_accounts", "DU12345"}))

	events := []Event{
		{Kind: EventConnect, Leg: 1},
		{Kind: EventChunk, Leg: 1, Direction: "client", Length: 3, Data: encodeBase64(frame1[:3])},
		{Kind: EventChunk, Leg: 1, Direction: "client", Length: len(frame1[3:]) + len(frame2), Data: encodeBase64(append(append([]byte(nil), frame1[3:]...), frame2...))},
		{Kind: EventDisconnect, Leg: 1},
		{Kind: EventConnect, Leg: 2},
		{Kind: EventChunk, Leg: 2, Direction: "server", Length: len(frame2), Data: encodeBase64(frame2)},
		{Kind: EventDisconnect, Leg: 2},
	}

	replayEvents, err := NormalizeEvents(events)
	if err != nil {
		t.Fatalf("NormalizeEvents() error = %v", err)
	}
	if len(replayEvents) != 7 {
		t.Fatalf("replayEvents len = %d, want 7", len(replayEvents))
	}
	if replayEvents[0].Kind != EventConnect || replayEvents[0].Leg != 1 {
		t.Fatalf("replayEvents[0] = %#v, want connect leg 1", replayEvents[0])
	}
	if replayEvents[1].Kind != ReplayEventFrame || replayEvents[1].Direction != "client" {
		t.Fatalf("replayEvents[1] = %#v, want client frame", replayEvents[1])
	}
	if replayEvents[2].Kind != ReplayEventFrame || replayEvents[2].Direction != "client" {
		t.Fatalf("replayEvents[2] = %#v, want second client frame", replayEvents[2])
	}
	if replayEvents[3].Kind != EventDisconnect || replayEvents[3].Leg != 1 {
		t.Fatalf("replayEvents[3] = %#v, want disconnect leg 1", replayEvents[3])
	}
	if replayEvents[4].Kind != EventConnect || replayEvents[4].Leg != 2 {
		t.Fatalf("replayEvents[4] = %#v, want connect leg 2", replayEvents[4])
	}
	if replayEvents[5].Kind != ReplayEventFrame || replayEvents[5].Direction != "server" {
		t.Fatalf("replayEvents[5] = %#v, want server frame", replayEvents[5])
	}
	if replayEvents[6].Kind != EventDisconnect || replayEvents[6].Leg != 2 {
		t.Fatalf("replayEvents[6] = %#v, want disconnect leg 2", replayEvents[6])
	}

	got1, err := base64Decoded(replayEvents[1].Data)
	if err != nil {
		t.Fatalf("base64Decoded(frame1) error = %v", err)
	}
	if !bytes.Equal(got1, wire.EncodeFields([]string{"hello", "1"})) {
		t.Fatalf("frame1 payload = %x, want %x", got1, wire.EncodeFields([]string{"hello", "1"}))
	}

	got2, err := base64Decoded(replayEvents[2].Data)
	if err != nil {
		t.Fatalf("base64Decoded(frame2) error = %v", err)
	}
	if !bytes.Equal(got2, wire.EncodeFields([]string{"managed_accounts", "DU12345"})) {
		t.Fatalf("frame2 payload = %x, want %x", got2, wire.EncodeFields([]string{"managed_accounts", "DU12345"}))
	}
}

func TestNormalizeEventsRejectsTruncatedFrameOnDisconnect(t *testing.T) {
	t.Parallel()

	frame := mustFrame(t, wire.EncodeFields([]string{"hello", "1"}))
	events := []Event{
		{Kind: EventConnect, Leg: 1},
		{Kind: EventChunk, Leg: 1, Direction: "client", Length: 3, Data: encodeBase64(frame[:3])},
		{Kind: EventDisconnect, Leg: 1},
	}

	if _, err := NormalizeEvents(events); err == nil {
		t.Fatal("NormalizeEvents() error = nil, want truncated frame rejection")
	}
}

func TestNormalizeEventsRejectsDeclaredChunkLengthMismatch(t *testing.T) {
	t.Parallel()

	// First client chunk from live sv203 capture
	// 20260710T160907Z-protobuf_sv203_required_conid_order_cancel_aapl,
	// events sha256 8efd714c3885da232215b0f4f4bb661ac7f4364126d4c97f4200dfa71320c55d.
	events := []Event{
		{Kind: EventConnect, Leg: 1},
		{
			Kind:      EventChunk,
			Leg:       1,
			Direction: "client",
			Length:    999,
			Data:      "QVBJAAAAAAl2MTc2Li4yMDM=",
		},
	}

	_, err := NormalizeEvents(events)
	if err == nil || !strings.Contains(err.Error(), "chunk length = 999, decoded length = 17") {
		t.Fatalf("NormalizeEvents() error = %v, want declared length mismatch", err)
	}
}

func TestNormalizeEventsSkipsClientHandshakePrefix(t *testing.T) {
	t.Parallel()

	versionFrame := mustFrame(t, []byte("v100..200"))
	startFrame := mustFrame(t, wire.EncodeFields([]string{"71", "2", "1", ""}))
	serverFrame := mustFrame(t, wire.EncodeFields([]string{"200", "20260405 23:49:26 Central European Standard Time"}))

	events := []Event{
		{Kind: EventConnect, Leg: 1},
		{Kind: EventChunk, Leg: 1, Direction: "client", Length: 4 + len(versionFrame), Data: encodeBase64(append([]byte("API\x00"), versionFrame...))},
		{Kind: EventChunk, Leg: 1, Direction: "server", Length: len(serverFrame), Data: encodeBase64(serverFrame)},
		{Kind: EventChunk, Leg: 1, Direction: "client", Length: len(startFrame), Data: encodeBase64(startFrame)},
		{Kind: EventDisconnect, Leg: 1},
	}

	replayEvents, err := NormalizeEvents(events)
	if err != nil {
		t.Fatalf("NormalizeEvents() error = %v", err)
	}
	if len(replayEvents) != 5 {
		t.Fatalf("replayEvents len = %d, want 5", len(replayEvents))
	}
	gotVersion, err := base64Decoded(replayEvents[1].Data)
	if err != nil {
		t.Fatalf("base64Decoded(version) error = %v", err)
	}
	if string(gotVersion) != "v100..200" {
		t.Fatalf("version payload = %q, want v100..200", string(gotVersion))
	}
	gotStart, err := base64Decoded(replayEvents[3].Data)
	if err != nil {
		t.Fatalf("base64Decoded(start) error = %v", err)
	}
	if !bytes.Equal(gotStart, wire.EncodeFields([]string{"71", "2", "1", ""})) {
		t.Fatalf("start payload = %x, want START_API frame", gotStart)
	}
}

// TestCreatePermissions asserts capture sessions are owner-only: captures carry
// live account ids and login tokens, so the directory and its files must not be
// world-readable. Modes are meaningful on this repo's Linux CI; the assertions
// keep to the simple 0700/0600 contract Create promises.
func TestCreatePermissions(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows ignores Unix creation modes; capture access inherits the caller-selected parent ACL")
	}

	root := t.TempDir()
	session, err := Create(root, Meta{Scenario: "perms"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer session.Close()

	dirInfo, err := os.Stat(session.Dir())
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("dir mode = %o, want 700", got)
	}

	for _, name := range []string{"meta.json", "events.jsonl"} {
		info, err := os.Stat(filepath.Join(session.Dir(), name))
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("%s mode = %o, want 600", name, got)
		}
	}
}

func TestCreateRejectsScenarioPathTraversal(t *testing.T) {
	t.Parallel()

	if _, err := Create(t.TempDir(), Meta{Scenario: "../../outside"}); err == nil {
		t.Fatal("Create() error = nil, want invalid-scenario error")
	}
}

// TestRedactChunk proves a registered login literal is replaced before the
// chunk is base64-encoded to disk, so the raw secret never reaches the file.
func TestRedactChunk(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	session, err := Create(root, Meta{Scenario: "redact"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	const login = "secretlogin"
	session.Redact(login, "papertrader")

	frame := []byte("OpenOrder\x00tail\x00" + login + "\x000\x00")
	if err := session.Record("server", frame); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	eventsPath := filepath.Join(session.Dir(), "events.jsonl")
	events, err := LoadEvents(eventsPath)
	if err != nil {
		t.Fatalf("LoadEvents() error = %v", err)
	}
	var got []byte
	for _, event := range events {
		if event.Kind != EventChunk {
			continue
		}
		chunk, err := DecodeData(event)
		if err != nil {
			t.Fatalf("DecodeData() error = %v", err)
		}
		got = append(got, chunk...)
	}
	if bytes.Contains(got, []byte(login)) {
		t.Fatalf("decoded chunk still contains login token: %q", got)
	}
	if !bytes.Contains(got, []byte("papertrader")) {
		t.Fatalf("decoded chunk missing placeholder: %q", got)
	}
}

func mustFrame(t *testing.T, payload []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	if err := wire.WriteFrame(&buf, payload); err != nil {
		t.Fatalf("wire.WriteFrame() error = %v", err)
	}
	return buf.Bytes()
}

func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func base64Decoded(data string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(data)
}

package capturelog

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
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
	replayDir := filepath.Join(session.Dir(), "replay")
	if err := WriteReplay(replayDir, session.Dir(), meta, events); err != nil {
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

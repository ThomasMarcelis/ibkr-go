package capturelog

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

type ReplayMeta struct {
	Scenario   string            `json:"scenario"`
	ListenAddr string            `json:"listen_addr,omitempty"`
	Upstream   string            `json:"upstream,omitempty"`
	ClientID   int               `json:"client_id,omitempty"`
	Notes      string            `json:"notes,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	StartedAt  string            `json:"started_at,omitempty"`
	SourceDir  string            `json:"source_dir,omitempty"`
}

const ReplayEventFrame = "frame"

type ReplayEvent struct {
	At        string `json:"at,omitempty"`
	Kind      string `json:"kind"`
	Leg       int    `json:"leg,omitempty"`
	Direction string `json:"direction,omitempty"`
	Length    int    `json:"length,omitempty"`
	Data      string `json:"data,omitempty"`
}

type streamKey struct {
	leg       int
	direction string
}

func LoadMeta(path string) (Meta, error) {
	file, err := os.Open(path)
	if err != nil {
		return Meta{}, fmt.Errorf("capturelog: open meta: %w", err)
	}
	defer file.Close()

	var meta Meta
	if err := json.NewDecoder(file).Decode(&meta); err != nil {
		return Meta{}, fmt.Errorf("capturelog: decode meta: %w", err)
	}
	return meta, nil
}

func WriteReplay(dir string, sourceDir string, meta Meta, events []Event) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("capturelog: create replay dir: %w", err)
	}

	metaFile, err := os.Create(filepath.Join(dir, "meta.json"))
	if err != nil {
		return fmt.Errorf("capturelog: create replay meta: %w", err)
	}
	defer metaFile.Close()

	replayMeta := ReplayMeta{
		Scenario:   meta.Scenario,
		ListenAddr: meta.ListenAddr,
		Upstream:   meta.Upstream,
		ClientID:   meta.ClientID,
		Notes:      meta.Notes,
		Labels:     meta.Labels,
		SourceDir:  sourceDir,
	}
	if !meta.StartedAt.IsZero() {
		replayMeta.StartedAt = meta.StartedAt.UTC().Format("2006-01-02T15:04:05.000000000Z07:00")
	}
	if err := json.NewEncoder(metaFile).Encode(replayMeta); err != nil {
		return fmt.Errorf("capturelog: write replay meta: %w", err)
	}

	replayFile, err := os.Create(filepath.Join(dir, "frames.jsonl"))
	if err != nil {
		return fmt.Errorf("capturelog: create replay frames: %w", err)
	}
	defer replayFile.Close()

	replayEvents, err := NormalizeEvents(events)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(replayFile)
	for _, event := range replayEvents {
		if err := enc.Encode(event); err != nil {
			return fmt.Errorf("capturelog: write replay frame: %w", err)
		}
	}
	return nil
}

func NormalizeEvents(events []Event) ([]ReplayEvent, error) {
	replayEvents := make([]ReplayEvent, 0, len(events))
	activeLegs := make(map[int]bool)
	buffers := make(map[streamKey][]byte)

	for _, event := range events {
		kind := event.Kind
		if kind == "" {
			kind = EventChunk
		}
		leg := event.Leg
		if leg == 0 {
			leg = 1
		}

		switch kind {
		case EventConnect:
			if activeLegs[leg] {
				return nil, fmt.Errorf("capturelog: leg %d connected twice", leg)
			}
			activeLegs[leg] = true
			replayEvents = append(replayEvents, newReplayEvent(event.At, EventConnect, leg, "", nil))
		case EventDisconnect:
			if !activeLegs[leg] {
				return nil, fmt.Errorf("capturelog: leg %d disconnected before connect", leg)
			}
			if err := ensureLegDrained(leg, buffers); err != nil {
				return nil, err
			}
			delete(activeLegs, leg)
			replayEvents = append(replayEvents, newReplayEvent(event.At, EventDisconnect, leg, "", nil))
		case EventChunk:
			if event.Direction == "" {
				return nil, fmt.Errorf("capturelog: chunk event missing direction")
			}
			data, err := DecodeData(event)
			if err != nil {
				return nil, err
			}
			if !activeLegs[leg] {
				activeLegs[leg] = true
				replayEvents = append(replayEvents, newReplayEvent(event.At, EventConnect, leg, "", nil))
			}

			key := streamKey{leg: leg, direction: event.Direction}
			buffers[key] = append(buffers[key], data...)
			if key.direction == "client" && bytes.HasPrefix(buffers[key], []byte("API\x00")) {
				buffers[key] = buffers[key][4:]
			}
			for {
				payload, consumed, err := extractFrame(buffers[key])
				if err != nil {
					return nil, fmt.Errorf("capturelog: normalize leg %d %s: %w", leg, event.Direction, err)
				}
				if consumed == 0 {
					break
				}
				replayEvents = append(replayEvents, newReplayEvent(event.At, ReplayEventFrame, leg, event.Direction, payload))
				buffers[key] = buffers[key][consumed:]
			}
		default:
			return nil, fmt.Errorf("capturelog: unsupported event kind %q", kind)
		}
	}

	for leg := range activeLegs {
		if err := ensureLegDrained(leg, buffers); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("capturelog: leg %d missing disconnect event", leg)
	}

	return replayEvents, nil
}

func newReplayEvent(at time.Time, kind string, leg int, direction string, payload []byte) ReplayEvent {
	event := ReplayEvent{
		Kind: kind,
		Leg:  leg,
	}
	if !at.IsZero() {
		event.At = at.UTC().Format("2006-01-02T15:04:05.000000000Z07:00")
	}
	if direction != "" {
		event.Direction = direction
	}
	if payload != nil {
		event.Length = len(payload)
		event.Data = base64.StdEncoding.EncodeToString(payload)
	}
	return event
}

func ensureLegDrained(leg int, buffers map[streamKey][]byte) error {
	for key, pending := range buffers {
		if key.leg != leg {
			continue
		}
		if len(pending) > 0 {
			return fmt.Errorf("capturelog: leg %d %s ended with truncated frame (%d pending bytes)", leg, key.direction, len(pending))
		}
		delete(buffers, key)
	}
	return nil
}

func extractFrame(data []byte) ([]byte, int, error) {
	if len(data) < 4 {
		return nil, 0, nil
	}
	size := int(binary.BigEndian.Uint32(data[:4]))
	if size == 0 {
		return nil, 0, wire.ErrEmptyMessage
	}
	if len(data) < 4+size {
		return nil, 0, nil
	}

	payload, err := wire.ReadFrame(bytes.NewReader(data[:4+size]))
	if err != nil {
		return nil, 0, err
	}
	return payload, 4 + size, nil
}

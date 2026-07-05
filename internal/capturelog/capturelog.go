package capturelog

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Meta struct {
	Scenario   string            `json:"scenario"`
	ListenAddr string            `json:"listen_addr,omitempty"`
	Upstream   string            `json:"upstream,omitempty"`
	ClientID   int               `json:"client_id,omitempty"`
	Notes      string            `json:"notes,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	StartedAt  time.Time         `json:"started_at"`
}

const (
	EventConnect    = "connect"
	EventDisconnect = "disconnect"
	EventChunk      = "chunk"
)

type Event struct {
	At        time.Time `json:"at"`
	Kind      string    `json:"kind"`
	Leg       int       `json:"leg,omitempty"`
	Direction string    `json:"direction,omitempty"`
	Length    int       `json:"length,omitempty"`
	Data      string    `json:"data,omitempty"`
}

type Session struct {
	dir        string
	events     *os.File
	meta       *os.File
	enc        *json.Encoder
	closeOnce  sync.Once
	mu         sync.Mutex
	redactions []redaction
	maxSecret  int
	pending    map[streamKey][]byte
}

type redaction struct {
	secret      []byte
	placeholder []byte
}

// Redact registers an exact-literal replacement applied to every recorded chunk
// before it is base64-encoded to disk, so a known secret never reaches the
// capture files. The capture tool never learns the Gateway login from the wire
// (bootstrap observes only ManagedAccounts), so the operator seeds this with the
// login string it authenticated with, e.g. Redact(gatewayLogin, "papertrader").
//
// This is deliberately literal, not pattern-based: a generic username regex
// would false-positive on ordinary tokens, and field-position parsing of
// OpenOrder does not exist at this layer. It therefore redacts only secrets the
// caller names, and cannot discover an unknown login on its own. Call it before
// recording begins; it is not safe to call concurrently with Record.
func (s *Session) Redact(secret, placeholder string) {
	if secret == "" {
		return
	}
	secretBytes := []byte(secret)
	s.redactions = append(s.redactions, redaction{
		secret:      secretBytes,
		placeholder: lengthPreservingPlaceholder(len(secretBytes), placeholder),
	})
	if len(secretBytes) > s.maxSecret {
		s.maxSecret = len(secretBytes)
	}
}

func (s *Session) applyRedactions(data []byte) []byte {
	for _, r := range s.redactions {
		if bytes.Contains(data, r.secret) {
			data = bytes.ReplaceAll(data, r.secret, r.placeholder)
		}
	}
	return data
}

func lengthPreservingPlaceholder(n int, placeholder string) []byte {
	if n <= 0 {
		return nil
	}
	src := []byte(placeholder)
	if len(src) == 0 {
		src = []byte("x")
	}
	out := make([]byte, n)
	for i := range out {
		out[i] = src[i%len(src)]
	}
	return out
}

func Create(root string, meta Meta) (*Session, error) {
	if meta.StartedAt.IsZero() {
		meta.StartedAt = time.Now().UTC()
	}
	if meta.Scenario == "" {
		meta.Scenario = "capture"
	}

	// Captures carry live account ids, order refs, and login tokens, so the
	// session directory and its files are owner-only (0700/0600) rather than
	// the world-readable default of MkdirAll/Create.
	dir := filepath.Join(root, meta.StartedAt.Format("20060102T150405Z")+"-"+meta.Scenario)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("capturelog: create dir: %w", err)
	}

	metaFile, err := os.OpenFile(filepath.Join(dir, "meta.json"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("capturelog: create meta file: %w", err)
	}

	eventsFile, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		_ = metaFile.Close()
		return nil, fmt.Errorf("capturelog: create events file: %w", err)
	}

	if err := json.NewEncoder(metaFile).Encode(meta); err != nil {
		_ = eventsFile.Close()
		_ = metaFile.Close()
		return nil, fmt.Errorf("capturelog: write meta: %w", err)
	}

	return &Session{
		dir:    dir,
		events: eventsFile,
		meta:   metaFile,
		enc:    json.NewEncoder(eventsFile),
	}, nil
}

func (s *Session) Dir() string {
	return s.dir
}

func (s *Session) Record(direction string, data []byte) error {
	return s.RecordChunk(1, direction, data)
}

func (s *Session) RecordConnect(leg int) error {
	return s.recordEvent(Event{
		At:   time.Now().UTC(),
		Kind: EventConnect,
		Leg:  leg,
	})
}

func (s *Session) RecordDisconnect(leg int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.flushPendingLocked(func(key streamKey) bool { return key.leg == leg }); err != nil {
		return err
	}
	return s.recordEventLocked(Event{
		At:   time.Now().UTC(),
		Kind: EventDisconnect,
		Leg:  leg,
	})
}

func (s *Session) RecordChunk(leg int, direction string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	chunks := s.redactedChunksLocked(leg, direction, data, false)
	for _, chunk := range chunks {
		if err := s.recordEventLocked(Event{
			At:        time.Now().UTC(),
			Kind:      EventChunk,
			Leg:       leg,
			Direction: direction,
			Length:    len(chunk),
			Data:      base64.StdEncoding.EncodeToString(chunk),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Session) recordEvent(event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.recordEventLocked(event)
}

func (s *Session) recordEventLocked(event Event) error {
	return s.enc.Encode(event)
}

func (s *Session) redactedChunksLocked(leg int, direction string, data []byte, flush bool) [][]byte {
	if len(s.redactions) == 0 {
		if len(data) == 0 {
			return nil
		}
		return [][]byte{append([]byte(nil), data...)}
	}
	if s.pending == nil {
		s.pending = make(map[streamKey][]byte)
	}
	key := streamKey{leg: leg, direction: direction}
	buf := append(append([]byte(nil), s.pending[key]...), data...)
	buf = s.applyRedactions(buf)
	keep := 0
	if !flush && s.maxSecret > 1 && len(buf) > 0 {
		keep = s.maxSecret - 1
		if keep > len(buf) {
			keep = len(buf)
		}
	}
	ready := buf[:len(buf)-keep]
	if keep == 0 {
		delete(s.pending, key)
	} else {
		s.pending[key] = append(s.pending[key][:0], buf[len(buf)-keep:]...)
	}
	if len(ready) == 0 {
		return nil
	}
	return [][]byte{append([]byte(nil), ready...)}
}

func (s *Session) flushPendingLocked(match func(streamKey) bool) error {
	for key, pending := range s.pending {
		if !match(key) || len(pending) == 0 {
			continue
		}
		delete(s.pending, key)
		chunk := s.applyRedactions(pending)
		if err := s.recordEventLocked(Event{
			At:        time.Now().UTC(),
			Kind:      EventChunk,
			Leg:       key.leg,
			Direction: key.direction,
			Length:    len(chunk),
			Data:      base64.StdEncoding.EncodeToString(chunk),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Session) Close() error {
	var err error
	s.closeOnce.Do(func() {
		s.mu.Lock()
		if flushErr := s.flushPendingLocked(func(streamKey) bool { return true }); flushErr != nil && err == nil {
			err = flushErr
		}
		s.mu.Unlock()
		if syncErr := s.events.Sync(); syncErr != nil && err == nil {
			err = syncErr
		}
		if closeErr := s.events.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		if syncErr := s.meta.Sync(); syncErr != nil && err == nil {
			err = syncErr
		}
		if closeErr := s.meta.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	})
	return err
}

func LoadEvents(path string) ([]Event, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("capturelog: open events: %w", err)
	}
	defer file.Close()

	dec := json.NewDecoder(file)
	var events []Event
	for dec.More() {
		var event Event
		if err := dec.Decode(&event); err != nil {
			return nil, fmt.Errorf("capturelog: decode event: %w", err)
		}
		events = append(events, event)
	}
	return events, nil
}

func DecodeData(event Event) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(event.Data)
	if err != nil {
		return nil, fmt.Errorf("capturelog: decode event data: %w", err)
	}
	return data, nil
}

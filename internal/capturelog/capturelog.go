package capturelog

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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
	// queue keeps cross-direction chronology while each stream retains the
	// small suffix needed to find a secret split across socket reads.
	queue   []*queuedEvent
	streams map[streamKey][]*queuedEvent
}

type redaction struct {
	secret      []byte
	placeholder []byte
}

type queuedEvent struct {
	event Event
	data  []byte
	safe  int
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
	if root == "" {
		return nil, fmt.Errorf("capturelog: root is required")
	}
	if meta.StartedAt.IsZero() {
		meta.StartedAt = time.Now().UTC()
	}
	if meta.Scenario == "" {
		meta.Scenario = "capture"
	}
	if err := validateScenario(meta.Scenario); err != nil {
		return nil, err
	}

	// Captures carry live account ids, order refs, and login tokens, so the
	// session directory and its files are owner-only (0700/0600) rather than
	// the world-readable default of MkdirAll/Create.
	dir, err := createSessionDir(root, meta)
	if err != nil {
		return nil, fmt.Errorf("capturelog: create dir: %w", err)
	}

	// #nosec G304 -- dir is rooted under the caller-selected capture root and
	// the scenario suffix is restricted by validateScenario.
	metaFile, err := os.OpenFile(filepath.Join(dir, "meta.json"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("capturelog: create meta file: %w", err)
	}

	// #nosec G304 -- same validated capture directory as metaFile.
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

func createSessionDir(root string, meta Meta) (string, error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	timestamp := meta.StartedAt.Format("20060102T150405Z")
	for attempt := 1; ; attempt++ {
		name := timestamp + "-" + meta.Scenario
		if attempt > 1 {
			name = fmt.Sprintf("%s-%d-%s", timestamp, attempt, meta.Scenario)
		}
		dir := filepath.Join(root, name)
		if err := os.Mkdir(dir, 0o700); err == nil {
			return dir, nil
		} else if !errors.Is(err, fs.ErrExist) {
			return "", err
		}
	}
}

func validateScenario(scenario string) error {
	if scenario == "." || scenario == ".." || strings.ContainsAny(scenario, `/\\`) {
		return fmt.Errorf("capturelog: invalid scenario %q", scenario)
	}
	for _, r := range scenario {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.' {
			continue
		}
		return fmt.Errorf("capturelog: invalid scenario %q", scenario)
	}
	return nil
}

func (s *Session) Dir() string {
	return s.dir
}

func (s *Session) Record(direction string, data []byte) error {
	return s.RecordChunk(1, direction, data)
}

func (s *Session) RecordConnect(leg int) error {
	return s.enqueueEvent(Event{
		At:   time.Now().UTC(),
		Kind: EventConnect,
		Leg:  leg,
	})
}

func (s *Session) RecordDisconnect(leg int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.finishStreamsLocked(func(key streamKey) bool { return key.leg == leg }); err != nil {
		return err
	}
	s.queue = append(s.queue, &queuedEvent{event: Event{
		At:   time.Now().UTC(),
		Kind: EventDisconnect,
		Leg:  leg,
	}})
	return s.flushQueueLocked()
}

func (s *Session) RecordChunk(leg int, direction string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(data) == 0 {
		return nil
	}
	if len(s.redactions) == 0 {
		return s.recordChunkLocked(time.Now().UTC(), leg, direction, data)
	}

	if s.streams == nil {
		s.streams = make(map[streamKey][]*queuedEvent)
	}
	key := streamKey{leg: leg, direction: direction}
	record := &queuedEvent{
		event: Event{
			At:        time.Now().UTC(),
			Kind:      EventChunk,
			Leg:       leg,
			Direction: direction,
		},
		data: append([]byte(nil), data...),
	}
	s.queue = append(s.queue, record)
	s.streams[key] = append(s.streams[key], record)
	s.redactStreamLocked(key)
	return s.flushQueueLocked()
}

func (s *Session) enqueueEvent(event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.redactions) == 0 {
		return s.recordEventLocked(event)
	}
	s.queue = append(s.queue, &queuedEvent{event: event})
	return s.flushQueueLocked()
}

func (s *Session) recordEventLocked(event Event) error {
	return s.enc.Encode(event)
}

func (s *Session) recordChunkLocked(at time.Time, leg int, direction string, data []byte) error {
	chunk := append([]byte(nil), data...)
	return s.recordEventLocked(Event{
		At:        at,
		Kind:      EventChunk,
		Leg:       leg,
		Direction: direction,
		Length:    len(chunk),
		Data:      base64.StdEncoding.EncodeToString(chunk),
	})
}

func (s *Session) redactStreamLocked(key streamKey) {
	records := s.streams[key]
	unsafeLen := 0
	for _, record := range records {
		unsafeLen += len(record.data) - record.safe
	}
	unsafe := make([]byte, 0, unsafeLen)
	for _, record := range records {
		unsafe = append(unsafe, record.data[record.safe:]...)
	}
	unsafe = s.applyRedactions(unsafe)
	offset := 0
	for _, record := range records {
		n := len(record.data) - record.safe
		copy(record.data[record.safe:], unsafe[offset:offset+n])
		offset += n
	}

	keep := min(s.maxSecret-1, len(unsafe))
	ready := len(unsafe) - keep
	for _, record := range records {
		if ready == 0 {
			break
		}
		n := min(ready, len(record.data)-record.safe)
		record.safe += n
		ready -= n
	}
}

func (s *Session) finishStreamsLocked(match func(streamKey) bool) error {
	for key, records := range s.streams {
		if !match(key) {
			continue
		}
		for _, record := range records {
			record.safe = len(record.data)
		}
	}
	return s.flushQueueLocked()
}

func (s *Session) flushQueueLocked() error {
	for len(s.queue) > 0 {
		record := s.queue[0]
		if record.event.Kind != EventChunk {
			if err := s.recordEventLocked(record.event); err != nil {
				return err
			}
			s.queue = s.queue[1:]
			continue
		}
		if record.safe == 0 {
			return nil
		}

		if err := s.recordChunkLocked(record.event.At, record.event.Leg, record.event.Direction, record.data[:record.safe]); err != nil {
			return err
		}
		key := streamKey{leg: record.event.Leg, direction: record.event.Direction}
		if record.safe < len(record.data) {
			record.data = append([]byte(nil), record.data[record.safe:]...)
			record.safe = 0
			return nil
		}

		s.queue = s.queue[1:]
		records := s.streams[key]
		records = records[1:]
		if len(records) == 0 {
			delete(s.streams, key)
		} else {
			s.streams[key] = records
		}
	}
	return nil
}

func (s *Session) Close() error {
	var err error
	s.closeOnce.Do(func() {
		s.mu.Lock()
		if flushErr := s.finishStreamsLocked(func(streamKey) bool { return true }); flushErr != nil && err == nil {
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
	// #nosec G304 -- LoadEvents is a file-reading API; its caller explicitly
	// selects the private capture path to normalize.
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

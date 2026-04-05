package capturelog

import (
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
	dir       string
	events    *os.File
	meta      *os.File
	enc       *json.Encoder
	closeOnce sync.Once
	mu        sync.Mutex
}

func Create(root string, meta Meta) (*Session, error) {
	if meta.StartedAt.IsZero() {
		meta.StartedAt = time.Now().UTC()
	}
	if meta.Scenario == "" {
		meta.Scenario = "capture"
	}

	dir := filepath.Join(root, meta.StartedAt.Format("20060102T150405Z")+"-"+meta.Scenario)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("capturelog: create dir: %w", err)
	}

	metaFile, err := os.Create(filepath.Join(dir, "meta.json"))
	if err != nil {
		return nil, fmt.Errorf("capturelog: create meta file: %w", err)
	}

	eventsFile, err := os.Create(filepath.Join(dir, "events.jsonl"))
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
	return s.recordEvent(Event{
		At:   time.Now().UTC(),
		Kind: EventDisconnect,
		Leg:  leg,
	})
}

func (s *Session) RecordChunk(leg int, direction string, data []byte) error {
	return s.recordEvent(Event{
		At:        time.Now().UTC(),
		Kind:      EventChunk,
		Leg:       leg,
		Direction: direction,
		Length:    len(data),
		Data:      base64.StdEncoding.EncodeToString(data),
	})
}

func (s *Session) recordEvent(event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enc.Encode(event)
}

func (s *Session) Close() error {
	var err error
	s.closeOnce.Do(func() {
		if closeErr := s.events.Close(); closeErr != nil && err == nil {
			err = closeErr
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

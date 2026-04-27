package sdkadapter

import (
	"encoding/json"
	"fmt"
	"io"
)

type Fixture struct {
	Metadata FixtureMetadata `json:"metadata"`
	Events   []Event         `json:"events"`
}

type FixtureMetadata struct {
	SDKVersion     string `json:"sdk_version"`
	ServerVersion  int    `json:"server_version"`
	CapturedAt     string `json:"captured_at"`
	Scenario       string `json:"scenario"`
	RedactionNotes string `json:"redaction_notes"`
	SourceSHA256   string `json:"source_sha256"`
}

func DecodeFixture(r io.Reader) (Fixture, error) {
	var fixture Fixture
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&fixture); err != nil {
		return Fixture{}, err
	}
	if err := fixture.Validate(); err != nil {
		return Fixture{}, err
	}
	return fixture, nil
}

func (f Fixture) Validate() error {
	if f.Metadata.SDKVersion == "" {
		return fmt.Errorf("sdkadapter: fixture metadata sdk_version is required")
	}
	if f.Metadata.ServerVersion <= 0 {
		return fmt.Errorf("sdkadapter: fixture metadata server_version is required")
	}
	if f.Metadata.CapturedAt == "" {
		return fmt.Errorf("sdkadapter: fixture metadata captured_at is required")
	}
	if f.Metadata.Scenario == "" {
		return fmt.Errorf("sdkadapter: fixture metadata scenario is required")
	}
	if f.Metadata.RedactionNotes == "" {
		return fmt.Errorf("sdkadapter: fixture metadata redaction_notes is required")
	}
	if f.Metadata.SourceSHA256 == "" {
		return fmt.Errorf("sdkadapter: fixture metadata source_sha256 is required")
	}
	for i, event := range f.Events {
		if event.Kind == "" {
			return fmt.Errorf("sdkadapter: fixture event %d kind is required", i)
		}
	}
	return nil
}

func NewReplayAdapterFromFixture(f Fixture) (*ReplayAdapter, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	adapter := NewReplayAdapter(f.Events)
	adapter.serverVersion = f.Metadata.ServerVersion
	adapter.connectionTime = f.Metadata.CapturedAt
	return adapter, nil
}

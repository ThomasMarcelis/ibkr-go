package sdkadapter

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
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

func EncodeFixture(w io.Writer, fixture Fixture) error {
	if err := fixture.Validate(); err != nil {
		return err
	}
	events := make([]any, len(fixture.Events))
	for i, event := range fixture.Events {
		events[i] = compactJSONValue(reflect.ValueOf(event))
	}
	out := struct {
		Metadata FixtureMetadata `json:"metadata"`
		Events   []any           `json:"events"`
	}{
		Metadata: fixture.Metadata,
		Events:   events,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func compactJSONValue(v reflect.Value) any {
	if !v.IsValid() {
		return nil
	}
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Struct:
		out := make(map[string]any)
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			if field.PkgPath != "" {
				continue
			}
			value := compactJSONValue(v.Field(i))
			if value != nil {
				out[field.Name] = value
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case reflect.Slice, reflect.Array:
		if v.Len() == 0 {
			return nil
		}
		out := make([]any, 0, v.Len())
		for i := 0; i < v.Len(); i++ {
			value := compactJSONValue(v.Index(i))
			if value != nil {
				out = append(out, value)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case reflect.String:
		if v.String() == "" {
			return nil
		}
		return v.String()
	case reflect.Bool:
		if !v.Bool() {
			return nil
		}
		return v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.Int() == 0 {
			return nil
		}
		return v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if v.Uint() == 0 {
			return nil
		}
		return v.Uint()
	case reflect.Float32, reflect.Float64:
		if v.Float() == 0 {
			return nil
		}
		return v.Float()
	default:
		if v.IsZero() {
			return nil
		}
		return v.Interface()
	}
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

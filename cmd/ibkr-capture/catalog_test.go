package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestScenarioCatalogCoversEveryScenario(t *testing.T) {
	t.Parallel()

	entries, err := catalogEntries()
	if err != nil {
		t.Fatalf("catalogEntries() error = %v", err)
	}
	if len(entries) != len(scenarios) {
		t.Fatalf("catalog entries = %d, scenarios = %d", len(entries), len(scenarios))
	}
	for _, entry := range entries {
		if entry.Domain == "" {
			t.Errorf("%s missing domain", entry.Name)
		}
		if len(entry.PublicAPI) == 0 {
			t.Errorf("%s missing public API", entry.Name)
		}
		if len(entry.MessageIDs) == 0 {
			t.Errorf("%s missing message IDs", entry.Name)
		}
		if entry.RiskClass == "" {
			t.Errorf("%s missing risk class", entry.Name)
		}
		if len(entry.ExpectedOutcomes) == 0 {
			t.Errorf("%s missing expected outcomes", entry.Name)
		}
		if len(entry.Batches) == 0 {
			t.Errorf("%s missing batches", entry.Name)
		}
		if entry.DefaultClientID < 0 {
			t.Errorf("%s default client ID = %d, want >= 0", entry.Name, entry.DefaultClientID)
		}
		if entry.PromotionStatus == "" {
			t.Errorf("%s missing promotion status", entry.Name)
		}
	}
}

func TestWriteCatalogJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := writeCatalogJSON(&buf); err != nil {
		t.Fatalf("writeCatalogJSON() error = %v", err)
	}
	var entries []scenarioCatalogEntry
	if err := json.Unmarshal(buf.Bytes(), &entries); err != nil {
		t.Fatalf("catalog JSON did not decode: %v", err)
	}
	if len(entries) != len(scenarios) {
		t.Fatalf("JSON entries = %d, scenarios = %d", len(entries), len(scenarios))
	}
}

func TestWriteBatchList(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := writeBatchList(&buf, batchNewV2); err != nil {
		t.Fatalf("writeBatchList() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) == 0 {
		t.Fatal("new-v2 batch is empty")
	}
	for _, line := range lines {
		parts := strings.Split(line, "|")
		if len(parts) != 2 {
			t.Fatalf("batch line %q should be name|client_id", line)
		}
		if _, ok := scenarios[parts[0]]; !ok {
			t.Fatalf("batch line references unknown scenario %q", parts[0])
		}
	}
}

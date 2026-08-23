package ibkr_test

import (
	"encoding/json"
	"os"
	"testing"
)

type stableReleaseManifest struct {
	Schema            int                 `json:"schema"`
	Release           string              `json:"release"`
	Status            string              `json:"status"`
	Policy            string              `json:"policy"`
	KnownEvidenceGaps []stableEvidenceGap `json:"known_evidence_gaps"`
}

type stableEvidenceGap struct {
	ID       string `json:"id"`
	Gap      string `json:"gap"`
	Impact   string `json:"impact"`
	FollowUp string `json:"follow_up"`
}

func TestV2StableReleaseManifest(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/release/v2.0.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest stableReleaseManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode stable release manifest: %v", err)
	}
	if manifest.Schema != 2 {
		t.Fatalf("release schema = %d, want 2", manifest.Schema)
	}
	if manifest.Release != "v2.0.0" {
		t.Fatalf("release = %q, want v2.0.0", manifest.Release)
	}
	if manifest.Status != "ready-with-known-evidence-gaps" {
		t.Fatalf("release status = %q, want ready-with-known-evidence-gaps", manifest.Status)
	}
	if manifest.Policy == "" {
		t.Fatal("release policy is empty")
	}

	want := map[string]struct{}{
		"historical-update-message-90":    {},
		"malformed-generation-retirement": {},
		"include-overnight-live":          {},
		"regulatory-snapshot":             {},
		"order-bound":                     {},
		"seven-day-soak":                  {},
	}
	if len(manifest.KnownEvidenceGaps) != len(want) {
		t.Fatalf("known evidence gaps = %d, want %d", len(manifest.KnownEvidenceGaps), len(want))
	}
	for _, gap := range manifest.KnownEvidenceGaps {
		if _, ok := want[gap.ID]; !ok {
			t.Fatalf("unexpected or duplicate known evidence gap %q", gap.ID)
		}
		if gap.Gap == "" || gap.Impact == "" || gap.FollowUp == "" {
			t.Fatalf("incomplete known evidence gap: %+v", gap)
		}
		delete(want, gap.ID)
	}

	if os.Getenv("IBKR_STABLE_RELEASE") == "1" {
		if tag := os.Getenv("IBKR_RELEASE_TAG"); tag != manifest.Release {
			t.Fatalf("IBKR_RELEASE_TAG = %q, want manifest release %q", tag, manifest.Release)
		}
	}
}

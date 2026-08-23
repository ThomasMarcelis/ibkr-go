package ibkr_test

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type stableReleaseReadiness struct {
	Schema   int                    `json:"schema"`
	Release  string                 `json:"release"`
	Status   string                 `json:"status"`
	Evidence stableReleaseEvidence  `json:"evidence"`
	Blockers []stableReleaseBlocker `json:"blockers"`
}

type stableReleaseEvidence struct {
	RegulatorySnapshot *stableLiveProof `json:"regulatory_snapshot"`
	OrderBound         *stableLiveProof `json:"order_bound"`
	SoakDays           []stableSoakDay  `json:"soak_days"`
}

type stableLiveProof struct {
	Role          string `json:"role"`
	ServerVersion int    `json:"server_version"`
	CaptureID     string `json:"capture_id"`
	EventsSHA256  string `json:"events_sha256"`
	Transcript    string `json:"transcript"`
}

type stableSoakDay struct {
	Date       string `json:"date"`
	Candidate  string `json:"candidate"`
	GateRecord string `json:"gate_record"`
}

type stableReleaseBlocker struct {
	ID          string `json:"id"`
	Requirement string `json:"requirement"`
}

func TestV2StableReleaseReadiness(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/release/v2.0.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var readiness stableReleaseReadiness
	if err := json.Unmarshal(data, &readiness); err != nil {
		t.Fatalf("decode stable release readiness: %v", err)
	}
	if readiness.Schema != 1 {
		t.Fatalf("readiness schema = %d, want 1", readiness.Schema)
	}
	if readiness.Release != "v2.0.0" {
		t.Fatalf("readiness release = %q, want v2.0.0", readiness.Release)
	}
	if readiness.Status != "blocked" && readiness.Status != "ready" {
		t.Fatalf("readiness status = %q, want blocked or ready", readiness.Status)
	}
	seen := make(map[string]struct{}, len(readiness.Blockers))
	for _, blocker := range readiness.Blockers {
		if blocker.ID == "" || blocker.Requirement == "" {
			t.Fatalf("incomplete stable release blocker: %+v", blocker)
		}
		if _, ok := seen[blocker.ID]; ok {
			t.Fatalf("duplicate stable release blocker %q", blocker.ID)
		}
		seen[blocker.ID] = struct{}{}
	}
	validateStablePartialEvidence(t, readiness.Evidence)
	if readiness.Status == "ready" {
		if len(readiness.Blockers) != 0 {
			t.Fatalf("ready release still has %d blockers", len(readiness.Blockers))
		}
		validateStableReleaseEvidence(t, readiness.Evidence)
	}
	if os.Getenv("IBKR_STABLE_RELEASE") == "1" {
		if readiness.Status != "ready" {
			t.Fatalf("stable release blocked by %d declared readiness requirements", len(readiness.Blockers))
		}
		if tag := os.Getenv("IBKR_RELEASE_TAG"); tag != readiness.Release {
			t.Fatalf("IBKR_RELEASE_TAG = %q, want manifest release %q", tag, readiness.Release)
		}
	}
}

func validateStableReleaseEvidence(t *testing.T, evidence stableReleaseEvidence) {
	t.Helper()

	if evidence.RegulatorySnapshot == nil {
		t.Fatal("regulatory_snapshot evidence is absent")
	}
	if evidence.OrderBound == nil {
		t.Fatal("order_bound evidence is absent")
	}
	if len(evidence.SoakDays) != 7 {
		t.Fatalf("soak_days = %d, want 7", len(evidence.SoakDays))
	}
	validateStableSoakDays(t, evidence.SoakDays)
}

func validateStablePartialEvidence(t *testing.T, evidence stableReleaseEvidence) {
	t.Helper()

	if evidence.RegulatorySnapshot != nil {
		validateStableLiveProof(t, "regulatory_snapshot", evidence.RegulatorySnapshot)
	}
	if evidence.OrderBound != nil {
		validateStableLiveProof(t, "order_bound", evidence.OrderBound)
		if evidence.OrderBound.Role != "paper-dev" {
			t.Fatalf("order_bound role = %q, want paper-dev", evidence.OrderBound.Role)
		}
	}
	if len(evidence.SoakDays) > 7 {
		t.Fatalf("soak_days = %d, want at most 7", len(evidence.SoakDays))
	}
	if len(evidence.SoakDays) != 0 {
		validateStableSoakDays(t, evidence.SoakDays)
	}
}

func validateStableSoakDays(t *testing.T, days []stableSoakDay) {
	t.Helper()

	var previous time.Time
	candidate := days[0].Candidate
	if !validHex(candidate, 40) {
		t.Fatalf("soak candidate = %q, want 40 hexadecimal characters", candidate)
	}
	for i, day := range days {
		date, err := time.Parse(time.DateOnly, day.Date)
		if err != nil {
			t.Fatalf("soak_days[%d].date = %q: %v", i, day.Date, err)
		}
		if i > 0 && !date.Equal(previous.AddDate(0, 0, 1)) {
			t.Fatalf("soak_days[%d].date = %s, want day after %s", i, day.Date, previous.Format(time.DateOnly))
		}
		if day.Candidate != candidate {
			t.Fatalf("soak_days[%d].candidate = %q, want %q", i, day.Candidate, candidate)
		}
		if day.GateRecord == "" {
			t.Fatalf("soak_days[%d].gate_record is empty", i)
		}
		previous = date
	}
}

func validateStableLiveProof(t *testing.T, name string, proof *stableLiveProof) {
	t.Helper()

	if proof.Role != "readonly-live" && proof.Role != "paper-dev" {
		t.Fatalf("%s role = %q", name, proof.Role)
	}
	if proof.ServerVersion < 200 || proof.ServerVersion > 225 {
		t.Fatalf("%s server_version = %d, want 200..225", name, proof.ServerVersion)
	}
	if proof.CaptureID == "" || proof.Transcript == "" {
		t.Fatalf("%s capture_id and transcript are required", name)
	}
	if !validHex(proof.EventsSHA256, 64) {
		t.Fatalf("%s events_sha256 = %q, want 64 hexadecimal characters", name, proof.EventsSHA256)
	}
	if err := validateStableLiveProofTranscript(proof); err != nil {
		t.Fatalf("%s transcript evidence: %v", name, err)
	}
}

func validateStableLiveProofTranscript(proof *stableLiveProof) error {
	if filepath.Base(proof.Transcript) != proof.Transcript {
		return fmt.Errorf("transcript %q must be a basename directly under testdata/transcripts", proof.Transcript)
	}
	path := filepath.Join("testdata/transcripts", proof.Transcript)
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}
	data, err := os.ReadFile(path) // #nosec G304 -- path is constrained to the transcript directory.
	if err != nil {
		return err
	}
	provenance, err := parseTranscriptProvenance(data)
	if err != nil {
		return err
	}
	if provenance.LegacyEventsHashPrefix != "" {
		return fmt.Errorf("legacy-prefix evidence cannot satisfy a stable proof")
	}
	if len(provenance.CaptureIDs) != 1 || len(provenance.EventsSHA256) != 1 {
		return fmt.Errorf("stable proof transcript must declare exactly one capture ID and one full events hash")
	}
	if provenance.CaptureIDs[0] != proof.CaptureID {
		return fmt.Errorf("capture_id %q does not match %q", proof.CaptureID, provenance.CaptureIDs[0])
	}
	if provenance.ServerVersion != proof.ServerVersion {
		return fmt.Errorf("server_version %d does not match %d", proof.ServerVersion, provenance.ServerVersion)
	}
	if provenance.EventsSHA256[0] != proof.EventsSHA256 {
		return fmt.Errorf("events_sha256 %q does not match %q", proof.EventsSHA256, provenance.EventsSHA256[0])
	}
	return nil
}

func TestStableLiveProofTranscriptMatchesProvenance(t *testing.T) {
	t.Parallel()

	proof := stableLiveProof{
		Role:          "paper-dev",
		ServerVersion: 200,
		CaptureID:     "20260414T183626Z-api_order_rest_cancel_aapl",
		EventsSHA256:  "1c0849882d07e2cc08bdf17d5107d113263848ea60f6df9593cd13f50bef1448",
		Transcript:    "api_order_rest_cancel_aapl.txt",
	}
	if err := validateStableLiveProofTranscript(&proof); err != nil {
		t.Fatal(err)
	}

	for name, tc := range map[string]struct {
		mutate  func(*stableLiveProof)
		wantErr string
	}{
		"nested path":     {func(p *stableLiveProof) { p.Transcript = "nested/" + p.Transcript }, "must be a basename"},
		"nonregular path": {func(p *stableLiveProof) { p.Transcript = "." }, "is not a regular file"},
		"ambiguous transcript": {
			func(p *stableLiveProof) { p.Transcript = "lifecycle_concurrent_oneshots.txt" },
			"exactly one capture ID and one full events hash",
		},
		"capture mismatch": {func(p *stableLiveProof) { p.CaptureID += "-other" }, "capture_id"},
		"version mismatch": {func(p *stableLiveProof) { p.ServerVersion++ }, "server_version"},
		"hash mismatch":    {func(p *stableLiveProof) { p.EventsSHA256 = "0" + p.EventsSHA256[1:] }, "events_sha256"},
		"legacy prefix": {
			func(p *stableLiveProof) { p.Transcript = "completed_orders_cancelled_system_live.txt" },
			"legacy-prefix evidence",
		},
	} {
		t.Run(name, func(t *testing.T) {
			invalid := proof
			tc.mutate(&invalid)
			if err := validateStableLiveProofTranscript(&invalid); err == nil {
				t.Fatal("validation succeeded")
			} else if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("validation error = %q, want %q", err, tc.wantErr)
			}
		})
	}
}

func validHex(value string, size int) bool {
	if len(value) != size {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

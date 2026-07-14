package ibkr_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type transcriptProvenance struct {
	Schema              int                              `json:"schema"`
	UnverifiedLegacy    []string                         `json:"unverified_legacy"`
	DependsOnUnverified []transcriptProvenanceDependency `json:"depends_on_unverified"`
}

type transcriptProvenanceDependency struct {
	File         string   `json:"file"`
	Dependencies []string `json:"dependencies"`
}

func TestTranscriptProvenanceInventory(t *testing.T) {
	t.Parallel()

	dir := filepath.Join("testdata", "transcripts")
	data, err := os.ReadFile("testdata/transcripts/provenance.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest transcriptProvenance
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode provenance manifest: %v", err)
	}
	if manifest.Schema != 1 {
		t.Fatalf("provenance schema = %d, want 1", manifest.Schema)
	}

	headerless := make([]string, 0, len(manifest.UnverifiedLegacy))
	files, err := filepath.Glob(filepath.Join(dir, "*.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if !transcriptHasHeader(t, file) {
			headerless = append(headerless, filepath.Base(file))
		}
	}
	slices.Sort(headerless)
	slices.Sort(manifest.UnverifiedLegacy)
	if !slices.Equal(headerless, manifest.UnverifiedLegacy) {
		t.Fatalf("headerless transcripts = %v, manifest unverified_legacy = %v", headerless, manifest.UnverifiedLegacy)
	}

	unverified := make(map[string]struct{}, len(manifest.UnverifiedLegacy))
	for _, file := range manifest.UnverifiedLegacy {
		unverified[file] = struct{}{}
	}
	for _, dependent := range manifest.DependsOnUnverified {
		if dependent.File != filepath.Base(dependent.File) {
			t.Errorf("dependent transcript %q is not a base name", dependent.File)
			continue
		}
		header, err := os.ReadFile(filepath.Join(dir, dependent.File)) // #nosec G304 -- the manifest is repository-owned and the base name is checked above
		if err != nil {
			t.Errorf("read dependent transcript %q: %v", dependent.File, err)
			continue
		}
		for _, dependency := range dependent.Dependencies {
			if _, ok := unverified[dependency]; !ok {
				t.Errorf("%s dependency %s is not in unverified_legacy", dependent.File, dependency)
			}
			if !strings.Contains(string(header), dependency) {
				t.Errorf("%s header does not name dependency %s", dependent.File, dependency)
			}
		}
	}

	if os.Getenv("IBKR_STABLE_RELEASE") == "1" && (len(manifest.UnverifiedLegacy) != 0 || len(manifest.DependsOnUnverified) != 0) {
		t.Fatalf("stable release blocked by %d unverified legacy transcripts and %d dependent fixtures", len(manifest.UnverifiedLegacy), len(manifest.DependsOnUnverified))
	}
}

func transcriptHasHeader(t *testing.T, path string) bool {
	t.Helper()

	file, err := os.Open(path) // #nosec G304 -- paths come only from filepath.Glob over the fixed transcript directory
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			return strings.HasPrefix(line, "#")
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return false
}

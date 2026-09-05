package ibkr_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
)

var (
	captureIDPattern      = regexp.MustCompile(`\b[0-9]{8}T[0-9]{6}Z-[A-Za-z0-9][A-Za-z0-9_-]*\b`)
	serverVersionPattern  = regexp.MustCompile(`\bserver_version\s*(?:=|:)?\s*([0-9]+)\b`)
	serverVersionsPattern = regexp.MustCompile(`\bserver_versions\s*(?:=|:)?\s*([0-9]+)\s*-\s*([0-9]+)\b`)
	eventsHashPattern     = regexp.MustCompile(`(?i)\bevents\.jsonl sha256\s*:?\s*([0-9a-f]{64})\b`)
)

type transcriptProvenance struct {
	CaptureIDs     []string
	ServerVersions []int
	EventsSHA256   []string
}

func TestTranscriptProvenanceInventory(t *testing.T) {
	t.Parallel()
	const wantTranscripts = 115

	files, err := filepath.Glob("testdata/transcripts/*.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != wantTranscripts {
		t.Fatalf("transcript corpus contains %d files, want migration inventory count %d", len(files), wantTranscripts)
	}
	_, captureCorpusErr := os.Stat("captures")
	verifyCaptureSources := captureCorpusErr == nil
	if captureCorpusErr != nil && !os.IsNotExist(captureCorpusErr) {
		t.Fatal(captureCorpusErr)
	}
	for _, file := range files {
		data, err := os.ReadFile(file) // #nosec G304 -- files come from the fixed transcript-directory glob.
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "raw server ") && !strings.Contains(string(data), "raw client ") {
			t.Errorf("%s: transcript has no captured raw frame", file)
		}
		provenance, err := parseTranscriptProvenance(data)
		if err != nil {
			t.Errorf("%s: %v", file, err)
			continue
		}
		for _, version := range provenance.ServerVersions {
			if version < protocol.SupportedMinServerVersion || version > protocol.SupportedMaxServerVersion {
				t.Errorf("%s: server_version %d is outside supported range %d-%d", file, version, protocol.SupportedMinServerVersion, protocol.SupportedMaxServerVersion)
			}
		}
		if verifyCaptureSources {
			verifyTranscriptCaptureSources(t, file, provenance)
		}
	}
}

func verifyTranscriptCaptureSources(t *testing.T, transcript string, provenance transcriptProvenance) {
	t.Helper()

	for i, captureID := range provenance.CaptureIDs {
		path := filepath.Join("captures", captureID, "events.jsonl")
		data, err := os.ReadFile(path) // #nosec G304 -- captureID is constrained by captureIDPattern.
		if err != nil {
			t.Errorf("%s: read declared source %s: %v", transcript, path, err)
			continue
		}
		got := fmt.Sprintf("%x", sha256.Sum256(data))
		if want := provenance.EventsSHA256[i]; got != want {
			t.Errorf("%s: %s sha256 = %s, want %s", transcript, path, got, want)
		}
	}
}

func TestTranscriptProvenanceParserIgnoresLaterComments(t *testing.T) {
	t.Parallel()

	data := []byte("# capture 20260710T223024Z-account_summary_snapshot, server_version 225\n" +
		"# events.jsonl sha256: 71f26259c1556157c0fd72b635934de341d43fe69bb04df72be27927bfa456db\n" +
		"handshake {\"server_version\":225}\n" +
		"# capture 19990101T000000Z-not-header, server_version 999\n")
	provenance, err := parseTranscriptProvenance(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(provenance.CaptureIDs) != 1 || provenance.CaptureIDs[0] != "20260710T223024Z-account_summary_snapshot" {
		t.Fatalf("capture IDs = %v, want only the initial header capture", provenance.CaptureIDs)
	}
}

func TestTranscriptProvenanceParserAcceptsExactVersionRange(t *testing.T) {
	t.Parallel()

	data := "# capture 20260824T213929Z-supported_version_matrix_paper, server_versions 208-210\n" +
		"# events.jsonl sha256: 64ee4350f0bde347a9da914a82865e88e0a68d06924cb13335fd2084595a7727\n" +
		"handshake {\"server_version\":208}\n" +
		"disconnect\n" +
		"handshake {\"server_version\":209}\n" +
		"disconnect\n" +
		"handshake {\"server_version\":210}\n"
	provenance, err := parseTranscriptProvenance([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(provenance.ServerVersions, []int{208, 209, 210}) {
		t.Fatalf("server versions = %v, want 208-210", provenance.ServerVersions)
	}
}

func TestTranscriptProvenanceParserRejectsIncompleteEvidence(t *testing.T) {
	t.Parallel()

	const valid = "# capture 20260710T223024Z-account_summary_snapshot, server_version 225\n" +
		"# events.jsonl sha256: 71f26259c1556157c0fd72b635934de341d43fe69bb04df72be27927bfa456db\n" +
		"handshake {\"server_version\":225}\n"
	for name, data := range map[string]string{
		"missing capture":           strings.Replace(valid, "20260710T223024Z-account_summary_snapshot", "account_summary_snapshot", 1),
		"multiple versions":         strings.Replace(valid, "\n# events", ", server_version 224\n# events", 1),
		"unlabelled prefix":         strings.Replace(valid, "events.jsonl sha256: 71f26259c1556157c0fd72b635934de341d43fe69bb04df72be27927bfa456db", "events.jsonl sha256 prefix: 71f26259c1556157", 1),
		"handshake mismatch":        strings.Replace(valid, `"server_version":225`, `"server_version":224`, 1),
		"handshake missing version": strings.Replace(valid, `{"server_version":225}`, `{}`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseTranscriptProvenance([]byte(data)); err == nil {
				t.Fatal("parseTranscriptProvenance succeeded, want error")
			}
		})
	}
}

func parseTranscriptProvenance(data []byte) (transcriptProvenance, error) {
	text := string(data)
	var headerLines []string
	for line := range strings.Lines(text) {
		line = strings.TrimSuffix(line, "\n")
		if !strings.HasPrefix(line, "#") {
			break
		}
		headerLines = append(headerLines, strings.TrimSpace(strings.TrimPrefix(line, "#")))
	}
	header := strings.Join(headerLines, " ")
	provenance := transcriptProvenance{CaptureIDs: uniqueStrings(captureIDPattern.FindAllString(header, -1))}
	if len(provenance.CaptureIDs) == 0 {
		return provenance, fmt.Errorf("initial comment block has no capture ID")
	}
	versions := serverVersionPattern.FindAllStringSubmatch(header, -1)
	versionRanges := serverVersionsPattern.FindAllStringSubmatch(header, -1)
	if len(versions)+len(versionRanges) != 1 {
		return provenance, fmt.Errorf("initial comment block declares %d server-version specifications, want exactly one", len(versions)+len(versionRanges))
	}
	if len(versions) == 1 {
		version, _ := strconv.Atoi(versions[0][1])
		provenance.ServerVersions = []int{version}
	} else {
		first, _ := strconv.Atoi(versionRanges[0][1])
		last, _ := strconv.Atoi(versionRanges[0][2])
		if first > last {
			return provenance, fmt.Errorf("initial comment block has descending server-version range %d-%d", first, last)
		}
		provenance.ServerVersions = make([]int, last-first+1)
		for i := range provenance.ServerVersions {
			provenance.ServerVersions[i] = first + i
		}
	}
	provenance.EventsSHA256 = uniqueMatches(eventsHashPattern, header)
	if len(provenance.EventsSHA256) != len(provenance.CaptureIDs) {
		return provenance, fmt.Errorf("initial comment block declares %d capture IDs and %d full events hashes", len(provenance.CaptureIDs), len(provenance.EventsSHA256))
	}
	var handshakeVersions []int
	for line := range strings.Lines(text) {
		line = strings.TrimSuffix(line, "\n")
		if !strings.HasPrefix(line, "handshake ") {
			continue
		}
		var handshake struct {
			ServerVersion *int `json:"server_version"`
		}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "handshake ")), &handshake); err != nil {
			return provenance, fmt.Errorf("decode handshake: %w", err)
		}
		if handshake.ServerVersion == nil {
			return provenance, fmt.Errorf("handshake has no server_version")
		}
		if len(handshakeVersions) == 0 || handshakeVersions[len(handshakeVersions)-1] != *handshake.ServerVersion {
			handshakeVersions = append(handshakeVersions, *handshake.ServerVersion)
		}
	}
	if !slices.Equal(handshakeVersions, provenance.ServerVersions) {
		return provenance, fmt.Errorf("handshake server versions %v disagree with declared %v", handshakeVersions, provenance.ServerVersions)
	}
	return provenance, nil
}

func uniqueMatches(pattern *regexp.Regexp, text string) []string {
	matches := pattern.FindAllStringSubmatch(text, -1)
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		values = append(values, match[1])
	}
	return uniqueStrings(values)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}

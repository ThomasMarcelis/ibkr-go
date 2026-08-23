package ibkr_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var (
	captureIDPattern     = regexp.MustCompile(`\b[0-9]{8}T[0-9]{6}Z-[A-Za-z0-9][A-Za-z0-9_-]*\b`)
	serverVersionPattern = regexp.MustCompile(`\bserver_version\s*(?:=|:)?\s*([0-9]+)\b`)
	eventsHashPattern    = regexp.MustCompile(`(?i)\bevents\.jsonl sha256\s*:?\s*([0-9a-f]{64})\b`)
	legacyHashPattern    = regexp.MustCompile(`(?i)\bevents\.jsonl sha256\s+legacy prefix\s*:?\s*([0-9a-f]{16})\b`)
)

type transcriptProvenance struct {
	CaptureIDs             []string
	ServerVersion          int
	EventsSHA256           []string
	LegacyEventsHashPrefix string
}

func TestTranscriptProvenanceInventory(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob("testdata/transcripts/*.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 134 {
		t.Fatalf("transcript count = %d, want 134", len(files))
	}
	raw := 0
	legacyFiles := make(map[string]string)
	for _, file := range files {
		data, err := os.ReadFile(file) // #nosec G304 -- files come from the fixed transcript-directory glob.
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "raw server ") || strings.Contains(string(data), "raw client ") {
			raw++
		}
		provenance, err := parseTranscriptProvenance(data)
		if err != nil {
			t.Errorf("%s: %v", file, err)
			continue
		}
		if provenance.LegacyEventsHashPrefix != "" {
			legacyFiles[filepath.Base(file)] = provenance.LegacyEventsHashPrefix
		}
	}
	if raw != 101 {
		t.Fatalf("raw transcript count = %d, want 101", raw)
	}
	if len(legacyFiles) != 1 || legacyFiles["completed_orders_cancelled_system_live.txt"] != "889d6f7f0ea2308d" {
		t.Fatalf("legacy transcript evidence = %v, want only completed_orders_cancelled_system_live.txt at 889d6f7f0ea2308d", legacyFiles)
	}
}

func TestTranscriptProvenanceParserIgnoresLaterComments(t *testing.T) {
	t.Parallel()

	data := []byte("# capture 20260710T223024Z-account_summary_snapshot, server_version 206\n" +
		"# events.jsonl sha256: 71f26259c1556157c0fd72b635934de341d43fe69bb04df72be27927bfa456db\n" +
		"handshake {\"server_version\":206}\n" +
		"# capture 19990101T000000Z-not-header, server_version 999\n")
	provenance, err := parseTranscriptProvenance(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(provenance.CaptureIDs) != 1 || provenance.CaptureIDs[0] != "20260710T223024Z-account_summary_snapshot" {
		t.Fatalf("capture IDs = %v, want only the initial header capture", provenance.CaptureIDs)
	}
}

func TestTranscriptProvenanceParserRejectsIncompleteEvidence(t *testing.T) {
	t.Parallel()

	const valid = "# capture 20260710T223024Z-account_summary_snapshot, server_version 206\n" +
		"# events.jsonl sha256: 71f26259c1556157c0fd72b635934de341d43fe69bb04df72be27927bfa456db\n" +
		"handshake {\"server_version\":206}\n"
	for name, data := range map[string]string{
		"missing capture":           strings.Replace(valid, "20260710T223024Z-account_summary_snapshot", "account_summary_snapshot", 1),
		"multiple versions":         strings.Replace(valid, "\n# events", ", server_version 207\n# events", 1),
		"unlabelled prefix":         strings.Replace(valid, "events.jsonl sha256: 71f26259c1556157c0fd72b635934de341d43fe69bb04df72be27927bfa456db", "events.jsonl sha256 prefix: 71f26259c1556157", 1),
		"handshake mismatch":        strings.Replace(valid, `"server_version":206`, `"server_version":207`, 1),
		"handshake missing version": strings.Replace(valid, `{"server_version":206}`, `{}`, 1),
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
	if len(versions) != 1 {
		return provenance, fmt.Errorf("initial comment block declares %d server versions, want exactly one", len(versions))
	}
	provenance.ServerVersion, _ = strconv.Atoi(versions[0][1])
	provenance.EventsSHA256 = uniqueMatches(eventsHashPattern, header)
	legacy := legacyHashPattern.FindAllStringSubmatch(header, -1)
	if len(provenance.EventsSHA256) == 0 && len(legacy) != 1 {
		return provenance, fmt.Errorf("initial comment block must declare a full events hash or one explicitly labelled legacy prefix")
	}
	if len(legacy) > 1 || len(provenance.EventsSHA256) != 0 && len(legacy) != 0 {
		return provenance, fmt.Errorf("initial comment block has ambiguous events hashes")
	}
	if len(legacy) == 1 {
		provenance.LegacyEventsHashPrefix = legacy[0][1]
	}
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
		if *handshake.ServerVersion != provenance.ServerVersion {
			return provenance, fmt.Errorf("handshake server_version %d disagrees with declared %d", *handshake.ServerVersion, provenance.ServerVersion)
		}
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

package ibkr

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Account-identifier shapes that must never appear in committed files.
// Live-derived fixtures redact accounts to DU9000001 (docs/transcripts.md);
// raw captures stay local under captures/. The patterns match IBKR live
// (U + 7-8 digits) and paper (DU/DUP + 6+ digits) account-id shapes.
var (
	accountIDPattern   = regexp.MustCompile(`\b(?:U\d{7,8}|DUP?\d{6,})\b`)
	redactionAllowlist = regexp.MustCompile(`\bDU9000001\b`)
)

// TestNoAccountIdentifiersInTrackedFiles walks the repository's committed
// text surfaces and fails on any live or paper account-id shape that is not
// the DU9000001 redaction token. This is the structural guard behind the
// sanitization contract: a fixture, test, or doc naming a real account
// cannot land, even inside a comment that documents the redaction itself.
func TestNoAccountIdentifiersInTrackedFiles(t *testing.T) {
	t.Parallel()

	roots := []string{"testdata", "docs", "internal", "cmd", "testing", "examples", "scripts"}
	var files []string
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return filepath.SkipDir
				}
				return err
			}
			if d.IsDir() {
				if d.Name() == "private" || d.Name() == "references" {
					return filepath.SkipDir
				}
				return nil
			}
			switch filepath.Ext(path) {
			case ".go", ".md", ".txt", ".sh", ".yml", ".yaml", ".json":
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("WalkDir(%s) error = %v", root, err)
		}
	}
	rootEntries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir(.) error = %v", err)
	}
	for _, e := range rootEntries {
		if e.IsDir() {
			continue
		}
		switch filepath.Ext(e.Name()) {
		case ".go", ".md", ".yml", ".yaml":
			files = append(files, e.Name())
		}
	}

	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", path, err)
		}
		lineNo := 0
		for line := range strings.Lines(string(data)) {
			lineNo++
			for _, match := range accountIDPattern.FindAllString(line, -1) {
				if redactionAllowlist.MatchString(match) {
					continue
				}
				t.Errorf("%s:%d: account identifier %q must be sanitized (use the DU9000001 redaction token)", path, lineNo, match)
			}
		}
	}
}

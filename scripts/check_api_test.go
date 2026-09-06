//go:build unix

package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise the shell controller with real isolated Git history. The Go tool
// boundary uses small textual surfaces so these tests need no network or
// tool download. The repository API gate separately runs the pinned apidiff.
const apiGoStub = `#!/usr/bin/env bash
set -euo pipefail
if [[ "$1" == list ]]; then echo example.com/library/v2; exit; fi
[[ "$1" == run && "$2" == golang.org/x/exp/cmd/apidiff@* ]]
shift 2
if [[ "$1" == -w ]]; then cp surface.api "$2"; exit; fi
if [[ "$1" == -incompatible ]]; then
 shift
 awk 'NR==FNR { present[$0]=1; next } !($0 in present) { print "removed: " $0 }' "$2" "$1"
else
 diff -u "$1" "$2" || true
fi
`

type apiRepo struct {
	t        *testing.T
	dir, bin string
}

func newAPIRepo(t *testing.T) *apiRepo {
	t.Helper()
	for _, tool := range []string{"bash", "git"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is required", tool)
		}
	}
	r := &apiRepo{t: t, dir: t.TempDir(), bin: t.TempDir()}
	script, err := os.ReadFile("check-api.sh")
	if err != nil {
		t.Fatal(err)
	}
	r.write("scripts/check-api.sh", string(script))
	writeExecutable(t, filepath.Join(r.bin, "go"), apiGoStub)
	r.git("init", "-q", "-b", "main")
	r.git("config", "user.email", "api-test@example.invalid")
	r.git("config", "user.name", "API check test")
	r.freeze("v2.0.0", "Old\n")
	r.commit()
	r.git("tag", "v2.0.0")
	return r
}

func (r *apiRepo) write(path, data string) {
	r.t.Helper()
	path = filepath.Join(r.dir, path)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		r.t.Fatal(err)
	}
	// #nosec G703 -- all paths are fixed test literals under this test-owned temporary repository.
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		r.t.Fatal(err)
	}
}
func (r *apiRepo) git(args ...string) {
	r.t.Helper()
	// #nosec G204 -- fixed test commands mutate only the isolated temporary repository.
	command := exec.Command("git", args...)
	command.Dir = r.dir
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	if out, err := command.CombinedOutput(); err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
func (r *apiRepo) commit() {
	r.t.Helper()
	r.git("add", ".")
	r.git("commit", "-q", "-m", "freeze test surface")
}
func (r *apiRepo) freeze(version, surface string) {
	r.t.Helper()
	files, err := filepath.Glob(filepath.Join(r.dir, "testdata/api/*.api"))
	if err != nil {
		r.t.Fatal(err)
	}
	for _, path := range files {
		if err := os.Remove(path); err != nil {
			r.t.Fatal(err)
		}
	}
	r.write("testdata/api/"+version+".api", surface)
	r.write("surface.api", surface)
}
func (r *apiRepo) check(want string, args ...string) {
	r.t.Helper()
	// #nosec G204 -- fixed script and test-controlled arguments in the temporary repository.
	command := exec.Command("bash", append([]string{"scripts/check-api.sh"}, args...)...)
	command.Dir = r.dir
	command.Env = append(os.Environ(), "PATH="+r.bin+string(os.PathListSeparator)+os.Getenv("PATH"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	out, err := command.CombinedOutput()
	if want == "" {
		if err != nil {
			r.t.Fatalf("check %v: %v\n%s", args, err, out)
		}
	} else if err == nil || !strings.Contains(string(out), want) {
		r.t.Fatalf("check %v: %v\n%s\nwant failure containing %q", args, err, out, want)
	}
}

func TestAPICandidateCannotRewriteReleaseCompatibility(t *testing.T) {
	r := newAPIRepo(t)
	r.freeze("v2.1.0", "New\nOld\n")
	r.check("")
	r.check("", "--exact")
	r.check("", "--release", "v2.1.0")
	r.freeze("v2.1.0", "New\n") // Regeneration hides the deletion only from exact mode.
	r.check("", "--exact")
	r.check("incompatible public API changes since v2.0.0")
	r.check("incompatible public API changes since v2.0.0", "--release", "v2.1.0")
	r.commit()
	r.git("tag", "v2.1.0")
	r.check("incompatible public API changes since v2.0.0", "--release", "v2.1.0")
}

func TestAPISelectsHighestReachableStableSameMajor(t *testing.T) {
	r := newAPIRepo(t)
	for _, version := range []string{"v2.9.0", "v2.10.0", "v2.99.0-rc.1", "v3.0.0"} {
		r.freeze(version, version+"\n")
		r.commit()
		r.git("tag", version)
	}
	r.git("checkout", "-q", "--orphan", "unreachable")
	r.freeze("v2.100.0", "unreachable\n")
	r.commit()
	r.git("tag", "v2.100.0")
	r.git("checkout", "-q", "main")
	r.freeze("v2.11.0", "v2.10.0\n")
	r.check("")
	r.check("", "--release", "v2.11.0")
	r.check("requires testdata/api/v2.10.0.api", "--release", "v2.10.0")
	// Equal-core stable v2.10.0 must be excluded for a v2.10.0 prerelease.
	r.freeze("v2.10.0-rc.1", "v2.9.0\n")
	r.check("", "--release", "v2.10.0-rc.1")
}

func TestAPIExactManifestAndArgumentErrors(t *testing.T) {
	r := newAPIRepo(t)
	r.write("surface.api", "New\nOld\n")
	r.check("")
	r.check("public API differs", "--exact")
	r.check("requires testdata/api/v2.1.0.api", "--release", "v2.1.0")
	r.write("testdata/api/v2.1.0.api", "New\nOld\n")
	r.check("exactly one candidate", "--exact")
	r.check("usage:", "--exact", "extra")
	r.check("invalid release version", "--release", "latest")
	r.check("does not match module", "--release", "v3.0.0")
}

func TestAPIMissingReleaseEvidenceFailsClosed(t *testing.T) {
	t.Run("tags", func(t *testing.T) {
		r := newAPIRepo(t)
		r.git("tag", "-d", "v2.0.0")
		r.check("no reachable stable v2 baseline")
		r.check("", "--exact")
	})
	t.Run("manifest", func(t *testing.T) {
		r := newAPIRepo(t)
		r.git("tag", "v2.0.1") // Tag does not contain a manifest for its own version.
		r.check("tag v2.0.1 lacks testdata/api/v2.0.1.api")
	})
	t.Run("shallow", func(t *testing.T) {
		r := newAPIRepo(t)
		clone := filepath.Join(t.TempDir(), "shallow")
		r.git("clone", "-q", "--depth=1", "file://"+filepath.ToSlash(r.dir), clone)
		r.dir = clone
		r.check("release history is shallow")
	})
}

func TestAPIDocumentedBreakRequiresExactPairAndEvidence(t *testing.T) {
	for _, mode := range [][]string{nil, {"--release", "v2.1.0"}} {
		t.Run(strings.Join(mode, " "), func(t *testing.T) {
			r := newAPIRepo(t)
			r.freeze("v2.1.0", "New\n")
			record := "testdata/api/v2.0.0-v2.1.0.breaks"
			r.check("missing "+record, mode...)
			r.write(record, "docs/migration-v2.1.md\nremoved: Old\n")
			r.check("lacks migration evidence", mode...)
			r.write("docs/migration-v2.1.md", "Replace Old with New.\n")
			r.check("", mode...)
			r.write(record, "docs/migration-v2.1.md\nremoved: Different\n")
			r.check("does not exactly match", mode...)
			r.write(record, "docs/migration-v2.1.md\nremoved: Old\nremoved: Extra\n")
			r.check("does not exactly match", mode...)
			r.write(record, "docs/migration-v2.1.md\nremoved: Old\n")
			r.freeze("v2.1.0", "Old\nNew\n")
			r.check("does not exactly match", mode...) // Stale allowance fails, too.
			r.freeze("v2.1.0", "New\n")
			r.commit()
			r.git("tag", "v2.1.0")
			r.check("")                        // Historical allowance no longer applies to a v2.1.0 baseline.
			r.check("", "--release", "v2.1.0") // Own tag remains excluded here.
			r.freeze("v2.2.0", "Third\n")
			r.check("missing testdata/api/v2.1.0-v2.2.0.breaks")
		})
	}
}

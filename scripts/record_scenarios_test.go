//go:build unix

package scripts_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRecordScenariosWaitsForRecorderAndCopiesDriverEvidence(t *testing.T) {
	env := newScriptEnvironment(t, "success|1")
	oldDir := filepath.Join(env.captures, "19990101T000000Z-success")
	if err := os.MkdirAll(oldDir, 0o700); err != nil {
		t.Fatalf("create old capture: %v", err)
	}
	oldLog := filepath.Join(oldDir, "driver.log")
	if err := os.WriteFile(oldLog, []byte("old evidence\n"), 0o600); err != nil {
		t.Fatalf("seed old driver log: %v", err)
	}
	result := env.run()
	if result.err != nil {
		t.Fatalf("record-scenarios error = %v\n%s", result.err, result.output)
	}
	if got := env.read("recorder.term"); got != "" {
		t.Fatalf("recorder termination log = %q, want natural exit", got)
	}
	captureDir := filepath.Join(env.captures, "20000101T000000Z-1-success")
	if got := env.readPath(filepath.Join(captureDir, "driver.log")); !strings.Contains(got, "scenario success complete") {
		t.Fatalf("driver.log = %q, want completion evidence", got)
	}
	if got := env.readPath(filepath.Join(captureDir, "driver_events.jsonl")); got != "driver-event\n" {
		t.Fatalf("driver_events.jsonl = %q, want copied driver event", got)
	}
	if got := env.readPath(filepath.Join(captureDir, "recorder.log")); !strings.Contains(got, "recorder success ready") {
		t.Fatalf("recorder.log = %q, want recorder diagnostics", got)
	}
	if got := env.readPath(oldLog); got != "old evidence\n" {
		t.Fatalf("old driver.log = %q, want untouched evidence", got)
	}
	if got := env.read("normalize.started"); got != "success\n" {
		t.Fatalf("capture verification log = %q, want success", got)
	}
	if strings.Contains(result.output, oldDir) {
		t.Fatalf("output lists historical capture directory:\n%s", result.output)
	}
}

func TestRecordScenariosRejectsUnverifiedCapture(t *testing.T) {
	env := newScriptEnvironment(t, "unverified|1")
	env.vars["NORMALIZE_FAIL"] = "unverified"
	result := env.run()
	if result.err == nil {
		t.Fatalf("record-scenarios error = nil, want verification failure\n%s", result.output)
	}
	if !strings.Contains(result.output, "verify_rc=12") {
		t.Fatalf("output = %q, want verifier exit status", result.output)
	}
}

func TestRecordScenariosRejectsMissingDriverEvidence(t *testing.T) {
	env := newScriptEnvironment(t, "missing_events|1")
	env.vars["CAPTURE_NO_EVENTS"] = "missing_events"
	result := env.run()
	if result.err == nil {
		t.Fatalf("record-scenarios error = nil, want missing-evidence failure\n%s", result.output)
	}
	if !strings.Contains(result.output, "evidence_rc=1") {
		t.Fatalf("output = %q, want evidence failure", result.output)
	}
	if got := env.read("normalize.started"); got != "" {
		t.Fatalf("capture verification started without driver evidence: %q", got)
	}
}

func TestRecordScenariosTerminatesRecorderAfterDriverFailure(t *testing.T) {
	env := newScriptEnvironment(t, "failed|1")
	env.vars["CAPTURE_FAIL"] = "failed"
	result := env.run()
	if result.err == nil {
		t.Fatalf("record-scenarios error = nil, want failure\n%s", result.output)
	}
	if got := env.read("recorder.term"); got != "failed\n" {
		t.Fatalf("recorder termination log = %q, want failed scenario", got)
	}
	if got := env.read("recorder.flushed"); got != "failed\n" {
		t.Fatalf("recorder flush log = %q, want failed scenario", got)
	}
}

func TestRecordScenariosDoesNotStartDriverBeforeRecorderReady(t *testing.T) {
	env := newScriptEnvironment(t, "bind_failure|1")
	env.vars["RECORDER_FAIL"] = "bind_failure"
	env.vars["IBKR_RECORDER_START_TIMEOUT"] = "1"
	result := env.run()
	if result.err == nil {
		t.Fatalf("record-scenarios error = nil, want startup failure\n%s", result.output)
	}
	if got := env.read("capture.started"); got != "" {
		t.Fatalf("capture start log = %q, want no driver start", got)
	}
	if !strings.Contains(result.output, "bind failed") {
		t.Fatalf("output = %q, want recorder startup diagnostic", result.output)
	}
}

func TestRecordScenariosRejectsUnsafeRoleBeforeRecorderStart(t *testing.T) {
	env := newScriptEnvironment(t, "paper_order|1")
	env.vars["PAPER_SCENARIO"] = "paper_order"
	env.vars["IBKR_CAPTURE_ROLE"] = "readonly-live"
	result := env.run()
	if result.err == nil {
		t.Fatalf("record-scenarios error = nil, want role failure\n%s", result.output)
	}
	if got := env.read("recorder.started"); got != "" {
		t.Fatalf("recorder start log = %q, want no recorder start", got)
	}
}

func TestRecordScenariosPreservesBatchLookupFailure(t *testing.T) {
	env := newScriptEnvironment(t, "unused|1")
	env.vars["LIST_FAIL"] = "1"
	result := env.run()
	if result.err == nil {
		t.Fatalf("record-scenarios error = nil, want catalog failure\n%s", result.output)
	}
	if got := env.read("recorder.started"); got != "" {
		t.Fatalf("recorder start log = %q, want no recorder start", got)
	}
}

func TestRecordScenariosContinuesOrFailsFastAsConfigured(t *testing.T) {
	for _, test := range []struct {
		name     string
		failFast string
		want     string
	}{
		{name: "aggregate", failFast: "0", want: "first\nsecond\n"},
		{name: "fail fast", failFast: "1", want: "first\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			env := newScriptEnvironment(t, "first|1\nsecond|2")
			env.vars["CAPTURE_FAIL"] = "first"
			env.vars["IBKR_CAPTURE_FAIL_FAST"] = test.failFast
			result := env.run()
			if result.err == nil {
				t.Fatalf("record-scenarios error = nil, want batch failure\n%s", result.output)
			}
			if got := env.read("capture.started"); got != test.want {
				t.Fatalf("capture start log = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRecordScenariosRepeatingNameWaitsForEachRecorder(t *testing.T) {
	env := newScriptEnvironment(t, "repeat|1\nrepeat|1")
	env.vars["DELAY_SECOND_READY"] = "repeat"
	result := env.run()
	if result.err != nil {
		t.Fatalf("record-scenarios error = %v\n%s", result.err, result.output)
	}
	want := "ready repeat 1\ncapture repeat\nready repeat 2\ncapture repeat\n"
	if got := env.read("timeline"); got != want {
		t.Fatalf("workflow timeline = %q, want %q", got, want)
	}
}

func TestRecordScenariosInterruptionReapsBothChildren(t *testing.T) {
	env := newScriptEnvironment(t, "blocked|1")
	env.vars["CAPTURE_BLOCK"] = "blocked"
	cmd := env.command()
	if err := cmd.Start(); err != nil {
		t.Fatalf("start record-scenarios: %v", err)
	}
	env.waitFor(t, "capture.started")
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("signal record-scenarios: %v", err)
	}
	err := cmd.Wait()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 143 {
		t.Fatalf("record-scenarios error = %v, want exit 143", err)
	}
	if got := env.read("capture.term"); got != "blocked\n" {
		t.Fatalf("capture termination log = %q, want blocked scenario", got)
	}
	if got := env.read("recorder.term"); got != "blocked\n" {
		t.Fatalf("recorder termination log = %q, want blocked scenario", got)
	}
	if got := env.read("capture.closed"); got != "blocked\n" {
		t.Fatalf("capture close log = %q, want recorder-driven close", got)
	}
	if got := env.read("capture.timeout"); got != "" {
		t.Fatalf("capture timeout log = %q, want prompt recorder signal", got)
	}
}

type scriptEnvironment struct {
	t        *testing.T
	dir      string
	state    string
	captures string
	vars     map[string]string
}

type scriptResult struct {
	output string
	err    error
}

func newScriptEnvironment(t *testing.T, scenarios string) *scriptEnvironment {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is required for the shell workflow test")
	}
	dir := t.TempDir()
	state := filepath.Join(dir, "state")
	captures := filepath.Join(dir, "captures")
	if err := os.MkdirAll(state, 0o700); err != nil {
		t.Fatalf("create state directory: %v", err)
	}
	recorder := filepath.Join(dir, "recorder")
	capture := filepath.Join(dir, "capture")
	normalize := filepath.Join(dir, "normalize")
	writeExecutable(t, recorder, recorderStub)
	writeExecutable(t, capture, captureStub)
	writeExecutable(t, normalize, normalizeStub)
	return &scriptEnvironment{
		t:        t,
		dir:      dir,
		state:    state,
		captures: captures,
		vars: map[string]string{
			"STATE":                       state,
			"SCENARIOS":                   scenarios,
			"IBKR_RECORDER":               recorder,
			"IBKR_CAPTURE":                capture,
			"IBKR_NORMALIZE":              normalize,
			"IBKR_CAPTURES":               captures,
			"IBKR_RECORDER_START_TIMEOUT": "2",
		},
	}
}

func (e *scriptEnvironment) command() *exec.Cmd {
	e.t.Helper()
	cmd := exec.Command("bash", "record-scenarios.sh")
	cmd.Dir = "."
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "LC_ALL=C"}
	for key, value := range e.vars {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	return cmd
}

func (e *scriptEnvironment) run() scriptResult {
	e.t.Helper()
	cmd := e.command()
	output, err := cmd.CombinedOutput()
	return scriptResult{output: string(output), err: err}
}

func (e *scriptEnvironment) read(name string) string {
	e.t.Helper()
	return e.readPath(filepath.Join(e.state, name))
}

func (e *scriptEnvironment) readPath(path string) string {
	e.t.Helper()
	// #nosec G304 -- callers pass paths rooted in this test's temporary directory.
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	if err != nil {
		e.t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func (e *scriptEnvironment) waitFor(t *testing.T, name string) {
	t.Helper()
	path := filepath.Join(e.state, name)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		// #nosec G304 -- path is rooted in this test's temporary state directory.
		if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", name)
}

func writeExecutable(t *testing.T, path, contents string) {
	t.Helper()
	// #nosec G306 -- the temporary stub must be executable by this test process.
	if err := os.WriteFile(path, []byte(contents), 0o700); err != nil {
		t.Fatalf("write stub %s: %v", path, err)
	}
}

const recorderStub = `#!/usr/bin/env bash
set -u
scenario=""
ready_file=""
out=""
while [ $# -gt 0 ]; do
    case "$1" in
        -scenario) scenario="$2"; shift 2 ;;
        -ready-file) ready_file="$2"; shift 2 ;;
        -out) out="$2"; shift 2 ;;
        *) shift 2 ;;
    esac
done
echo "$scenario" >> "$STATE/recorder.started"
if [ "${RECORDER_FAIL:-}" = "$scenario" ]; then
    echo "bind failed" >&2
    exit 7
fi
counter_file="$STATE/recorder-count-$scenario"
count=0
if [ -f "$counter_file" ]; then count=$(<"$counter_file"); fi
count=$((count + 1))
echo "$count" > "$counter_file"
capture_dir="$out/20000101T000000Z-$count-$scenario"
mkdir -p "$capture_dir"
trap 'echo "$scenario" >> "$STATE/recorder.term"; echo "$scenario" >> "$STATE/recorder.flushed"; exit 0' TERM INT HUP
if [ "${DELAY_SECOND_READY:-}" = "$scenario" ] && [ "$count" -eq 2 ]; then sleep 0.2; fi
printf '%s\n' "$capture_dir" > "$ready_file"
echo "recorder $scenario ready"
echo "ready $scenario $count" >> "$STATE/timeline"
while [ ! -e "$STATE/release-$scenario" ]; do
    sleep 0.01
done
echo "$scenario" >> "$STATE/recorder.natural"
echo "$scenario" >> "$STATE/recorder.flushed"
`

const captureStub = `#!/usr/bin/env bash
set -u
case "${1:-}" in
    -list-batch)
        if [ "${LIST_FAIL:-0}" = 1 ]; then echo "catalog failed" >&2; exit 11; fi
        printf '%s\n' "$SCENARIOS"
        exit 0
        ;;
    -role-for)
        if [ "${PAPER_SCENARIO:-}" = "${2:-}" ]; then echo paper-dev; else echo readonly-live; fi
        exit 0
        ;;
esac
scenario=""
events_file=""
while [ $# -gt 0 ]; do
    case "$1" in
        -scenario) scenario="$2"; shift 2 ;;
        -driver-events) events_file="$2"; shift 2 ;;
        *) shift 2 ;;
    esac
done
echo "$scenario" >> "$STATE/capture.started"
echo "capture $scenario" >> "$STATE/timeline"
if [ "${CAPTURE_BLOCK:-}" = "$scenario" ]; then
    trap 'echo "$scenario" >> "$STATE/capture.term"' TERM INT HUP
    deadline=$((SECONDS + 2))
    while [ ! -e "$STATE/recorder.term" ]; do
        if [ "$SECONDS" -ge "$deadline" ]; then
            echo "$scenario" >> "$STATE/capture.timeout"
            exit 0
        fi
        sleep 0.01
    done
    echo "$scenario" >> "$STATE/capture.closed"
    exit 0
fi
if [ "${CAPTURE_FAIL:-}" = "$scenario" ]; then
    echo "scenario $scenario failed"
    exit 9
fi
if [ "${CAPTURE_NO_EVENTS:-}" != "$scenario" ]; then
    echo driver-event > "$events_file"
fi
touch "$STATE/release-$scenario"
echo "scenario $scenario complete"
`

const normalizeStub = `#!/usr/bin/env bash
set -u
capture_dir=""
while [ $# -gt 0 ]; do
    case "$1" in
        -dir) capture_dir="$2"; shift 2 ;;
        *) shift ;;
    esac
done
scenario="${capture_dir##*-}"
echo "$scenario" >> "$STATE/normalize.started"
if [ "${NORMALIZE_FAIL:-}" = "$scenario" ]; then
    echo "verification failed" >&2
    exit 12
fi
echo "verified $scenario"
`

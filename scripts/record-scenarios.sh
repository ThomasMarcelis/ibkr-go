#!/usr/bin/env bash
# Record live scenarios through ibkr-recorder. With no arguments, the catalog
# batch named by IBKR_CAPTURE_BATCH is recorded (default: exhaustive-read-only). Explicit
# entries may be passed as "name" or "name|client_id".

set -uo pipefail
umask 077

LISTEN="${IBKR_LISTEN:-127.0.0.1:4101}"
OUTDIR="${IBKR_CAPTURES:-captures}"
RECORDER="${IBKR_RECORDER:-/tmp/ibkr-recorder}"
CAPTURE="${IBKR_CAPTURE:-/tmp/ibkr-capture}"
NORMALIZE="${IBKR_NORMALIZE:-/tmp/ibkr-normalize}"
BATCH="${IBKR_CAPTURE_BATCH:-exhaustive-read-only}"
RECORDER_MAX_LEGS="${IBKR_RECORDER_MAX_LEGS:-8}"
RECORDER_IDLE_TIMEOUT="${IBKR_RECORDER_IDLE_TIMEOUT:-30s}"
ROLE="${IBKR_CAPTURE_ROLE:-}"
FAIL_FAST="${IBKR_CAPTURE_FAIL_FAST:-0}"
START_TIMEOUT="${IBKR_RECORDER_START_TIMEOUT:-5}"

case "$FAIL_FAST" in
    0|1) ;;
    *) echo "unsupported IBKR_CAPTURE_FAIL_FAST=$FAIL_FAST (want 0 or 1)" >&2; exit 1 ;;
esac
case "$START_TIMEOUT" in
    ''|*[!0-9]*) echo "unsupported IBKR_RECORDER_START_TIMEOUT=$START_TIMEOUT (want whole seconds)" >&2; exit 1 ;;
esac
case "$RECORDER_MAX_LEGS" in
    ''|*[!0-9]*|0) echo "unsupported IBKR_RECORDER_MAX_LEGS=$RECORDER_MAX_LEGS (want a positive whole number)" >&2; exit 1 ;;
esac

WORKDIR=$(mktemp -d)
recorder_pid=""
capture_pid=""

signal_child() {
    local pid="$1"
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
        kill -TERM "$pid" 2>/dev/null || true
    fi
}

wait_child() {
    local pid="$1"
    if [ -n "$pid" ]; then
        wait "$pid" 2>/dev/null || true
    fi
}

terminate_child() {
    signal_child "$1"
    wait_child "$1"
}

cleanup() {
    local status=$?
    trap - EXIT INT TERM HUP
    # The driver performs paper reconciliation while unwinding. Keep its
    # recorder proxy alive until that cleanup has completed.
    signal_child "$capture_pid"
    wait_child "$capture_pid"
    signal_child "$recorder_pid"
    wait_child "$recorder_pid"
    rm -rf "$WORKDIR"
    exit "$status"
}

trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
trap 'exit 129' HUP

scenario_entry() {
    local scenario="$1"
    if [[ "$scenario" == *"|"* ]]; then
        echo "$scenario"
        return
    fi
    "$CAPTURE" -list-batch all | awk -F'|' -v name="$scenario" '$1 == name { print; found = 1 } END { if (!found) exit 1 }'
}

if [ $# -gt 0 ]; then
    SCENARIOS=()
    for scenario in "$@"; do
        if ! entry=$(scenario_entry "$scenario"); then
            echo "unknown scenario $scenario"
            exit 1
        fi
        SCENARIOS+=("$entry")
    done
else
    if ! batch_entries=$("$CAPTURE" -list-batch "$BATCH"); then
        echo "failed to list capture batch $BATCH"
        exit 1
    fi
    SCENARIOS=()
    while IFS= read -r entry; do
        if [ -n "$entry" ]; then
            SCENARIOS+=("$entry")
        fi
    done <<< "$batch_entries"
fi

if [ ${#SCENARIOS[@]} -eq 0 ]; then
    echo "no scenarios found for batch $BATCH"
    exit 1
fi

deduplicated=()
for entry in "${SCENARIOS[@]}"; do
    scenario_name="${entry%|*}"
    previous_entry=""
    if [ ${#deduplicated[@]} -gt 0 ]; then
        for candidate in "${deduplicated[@]}"; do
            if [ "${candidate%|*}" = "$scenario_name" ]; then
                previous_entry="$candidate"
                break
            fi
        done
    fi
    if [ -n "$previous_entry" ]; then
        if [ "$previous_entry" != "$entry" ]; then
            echo "conflicting entries for scenario $scenario_name: $previous_entry and $entry" >&2
            exit 1
        fi
        continue
    fi
    deduplicated+=("$entry")
done
SCENARIOS=("${deduplicated[@]}")

role_for_scenario() {
    local scenario="$1"
    local catalog_role
    catalog_role=$("$CAPTURE" -role-for "$scenario") || return 1
    if [ -z "$ROLE" ]; then
        echo "$catalog_role"
        return
    fi
    if [ "$ROLE" != "readonly-live" ] && [ "$ROLE" != "paper-dev" ]; then
        echo "unsupported IBKR_CAPTURE_ROLE=$ROLE (want readonly-live or paper-dev)" >&2
        return 1
    fi
    if [ "$catalog_role" = "paper-dev" ] && [ "$ROLE" != "paper-dev" ]; then
        echo "refusing to route paper scenario $scenario through $ROLE" >&2
        return 1
    fi
    echo "$ROLE"
}

upstream_for_role() {
    local role="$1"
    if [ "$role" = "paper-dev" ]; then
        echo "${IBKR_PAPER_UPSTREAM:-${IBKR_LIVE_PAPER_ADDR:-127.0.0.1:4002}}"
    elif [ "$role" = "readonly-live" ]; then
        echo "${IBKR_READONLY_UPSTREAM:-${IBKR_LIVE_READONLY_ADDR:-${IBKR_UPSTREAM:-127.0.0.1:4001}}}"
    else
        echo "unsupported capture role=$role (want readonly-live or paper-dev)" >&2
        return 1
    fi
}

wait_for_ready() {
    local ready_file="$1"
    local elapsed=0
    while [ ! -s "$ready_file" ]; do
        if ! kill -0 "$recorder_pid" 2>/dev/null; then
            wait "$recorder_pid" 2>/dev/null
            return $?
        fi
        if [ "$elapsed" -ge $((START_TIMEOUT * 20)) ]; then
            return 124
        fi
        sleep 0.05
        elapsed=$((elapsed + 1))
    done
}

stop_recorder() {
    local status=0
    if kill -0 "$recorder_pid" 2>/dev/null; then
        kill -TERM "$recorder_pid" 2>/dev/null || true
    fi
    wait "$recorder_pid" 2>/dev/null || status=$?
    recorder_pid=""
    return "$status"
}

mkdir -p "$OUTDIR"
failures=0
capture_dirs=()
run_number=0

for entry in "${SCENARIOS[@]}"; do
    unset capture_rc recorder_rc
    run_number=$((run_number + 1))
    scenario="${entry%|*}"
    client_id="${entry#*|}"
    if ! scenario_role=$(role_for_scenario "$scenario"); then
        echo "failed to resolve capture role for $scenario"
        exit 1
    fi
    if ! upstream=$(upstream_for_role "$scenario_role"); then
        exit 1
    fi

    run_prefix="$WORKDIR/$run_number"
    log_file="$run_prefix.log"
    events_file="$run_prefix.events.jsonl"
    recorder_log="$run_prefix.recorder.log"
    ready_file="$run_prefix.ready"
    : > "$log_file"
    : > "$events_file"
    printf "recording %-40s role=%-13s client_id=%-3s " "$scenario" "$scenario_role" "$client_id"

    "$RECORDER" \
        -upstream "$upstream" \
        -listen "$LISTEN" \
        -out "$OUTDIR" \
        -scenario "$scenario" \
        -client-id "$client_id" \
        -max-legs "$RECORDER_MAX_LEGS" \
		-idle-timeout "$RECORDER_IDLE_TIMEOUT" \
        -ready-file "$ready_file" \
        -notes "batch=$BATCH role=$scenario_role client_id=$client_id" \
        >"$recorder_log" 2>&1 &
    recorder_pid=$!

    if wait_for_ready "$ready_file"; then
        capture_dir=$(<"$ready_file")
        capture_dirs+=("$capture_dir")
    else
        recorder_rc=$?
        terminate_child "$recorder_pid"
        recorder_pid=""
        recorder_last=$(tail -1 "$recorder_log")
        echo "FAILED (recorder startup rc=$recorder_rc, last: $recorder_last)"
        failures=$((failures + 1))
        if [ "$FAIL_FAST" -eq 1 ]; then
            break
        fi
        continue
    fi

    "$CAPTURE" \
        -addr "$LISTEN" \
        -scenario "$scenario" \
        -client-id "$client_id" \
        -driver-events "$events_file" \
        >"$log_file" 2>&1 &
    capture_pid=$!
    wait "$capture_pid" || capture_rc=$?
    capture_rc=${capture_rc:-0}
    capture_pid=""

	# The driver owns scenario lifetime, including deferred paper cleanup. Once
	# it exits, stop the recorder explicitly so reconnect gaps never determine
	# whether a capture is complete.
	stop_recorder || recorder_rc=$?
	recorder_rc=${recorder_rc:-0}

    evidence_rc=0
    if [ -d "$capture_dir" ]; then
        cp "$log_file" "$capture_dir/driver.log" || evidence_rc=1
        cp "$recorder_log" "$capture_dir/recorder.log" || evidence_rc=1
        if [ -s "$events_file" ]; then
            cp "$events_file" "$capture_dir/driver_events.jsonl" || evidence_rc=1
        else
            evidence_rc=1
        fi
    else
        evidence_rc=1
    fi

    verify_rc=1
    if [ "$capture_rc" -eq 0 ] && [ "$recorder_rc" -eq 0 ] && [ "$evidence_rc" -eq 0 ]; then
        if "$NORMALIZE" -dir "$capture_dir" -verify >"$run_prefix.verify.log" 2>&1; then
            verify_rc=0
        else
            verify_rc=$?
        fi
    fi

    last=$(tail -1 "$log_file")
    if [ "$capture_rc" -eq 0 ] && [ "$recorder_rc" -eq 0 ] && [ "$evidence_rc" -eq 0 ] && [ "$verify_rc" -eq 0 ]; then
        echo "ok"
    else
        echo "FAILED (rc=$capture_rc, recorder_rc=$recorder_rc, evidence_rc=$evidence_rc, verify_rc=$verify_rc, last: $last)"
        failures=$((failures + 1))
        if [ "$FAIL_FAST" -eq 1 ]; then
            break
        fi
    fi
done

echo
echo "done. new captures:"
if [ ${#capture_dirs[@]} -gt 0 ]; then
    printf '%s\n' "${capture_dirs[@]}"
fi

if [ "$failures" -gt 0 ]; then
    echo "$failures scenario(s) failed"
    exit 1
fi

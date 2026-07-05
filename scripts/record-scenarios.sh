#!/bin/bash
# Record capture scenarios through the ibkr-recorder proxy.
# Usage: ./scripts/record-scenarios.sh [scenario...]
# If no scenarios are given, records the catalog batch named by
# IBKR_CAPTURE_BATCH, defaulting to new-v2. Explicit scenarios may be passed as
# "name" or "name|client_id".
# Each scenario's Gateway role is read from the cmd/ibkr-capture catalog.
# IBKR_CAPTURE_ROLE may route read-only scenarios through paper-dev, but
# paper-order scenarios always stay on paper-dev.

LISTEN="${IBKR_LISTEN:-127.0.0.1:4101}"
OUTDIR="${IBKR_CAPTURES:-captures}"
RECORDER="${IBKR_RECORDER:-/tmp/ibkr-recorder}"
CAPTURE="${IBKR_CAPTURE:-/tmp/ibkr-capture}"
BATCH="${IBKR_CAPTURE_BATCH:-new-v2}"
RECORDER_MAX_LEGS="${IBKR_RECORDER_MAX_LEGS:-1}"
ROLE="${IBKR_CAPTURE_ROLE:-}"
TMPLOG=$(mktemp)
TMPEVENTS=$(mktemp)
trap "rm -f $TMPLOG $TMPEVENTS" EXIT

scenario_entry() {
    local scenario="$1"
    if [[ "$scenario" == *"|"* ]]; then
        echo "$scenario"
        return
    fi
    "$CAPTURE" -list-batch all | awk -F'|' -v name="$scenario" '$1 == name { print; found = 1; exit } END { if (!found) exit 1 }'
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
    mapfile -t SCENARIOS < <("$CAPTURE" -list-batch "$BATCH")
fi

if [ ${#SCENARIOS[@]} -eq 0 ]; then
    echo "no scenarios found for batch $BATCH"
    exit 1
fi

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

mkdir -p "$OUTDIR"
failures=0

for entry in "${SCENARIOS[@]}"; do
    scenario="${entry%|*}"
    client_id="${entry#*|}"
    if ! scenario_role=$(role_for_scenario "$scenario"); then
        echo "failed to resolve capture role for $scenario"
        exit 1
    fi
    if ! upstream=$(upstream_for_role "$scenario_role"); then
        exit 1
    fi
    printf "recording %-40s role=%-13s client_id=%-3s " "$scenario" "$scenario_role" "$client_id"

    # Start recorder in background, suppress all output
    "$RECORDER" \
        -upstream "$upstream" \
        -listen "$LISTEN" \
        -out "$OUTDIR" \
        -scenario "$scenario" \
        -client-id "$client_id" \
        -max-legs "$RECORDER_MAX_LEGS" \
        -notes "batch=$BATCH role=$scenario_role client_id=$client_id" \
        >/dev/null 2>&1 &
    rpid=$!

    # Give recorder a moment to bind. Do not probe the TCP port here: the
    # recorder is intentionally one-leg-per-scenario, so a readiness probe would
    # consume the capture connection.
    sleep 0.5

    # Run capture, write output to temp file
    : > "$TMPEVENTS"
    "$CAPTURE" \
        -addr "$LISTEN" \
        -scenario "$scenario" \
        -client-id "$client_id" \
        -driver-events "$TMPEVENTS" \
        >"$TMPLOG" 2>&1
    rc=$?

    # Wait for recorder to finish
    wait "$rpid" 2>/dev/null
    recorder_rc=$?

    latest_dir=$(ls -dt "$OUTDIR"/20*-"$scenario" 2>/dev/null | head -1)
    if [ -n "$latest_dir" ]; then
        cp "$TMPLOG" "$latest_dir/driver.log"
        if [ -s "$TMPEVENTS" ]; then
            cp "$TMPEVENTS" "$latest_dir/driver_events.jsonl"
        fi
    fi

    last=$(tail -1 "$TMPLOG")
    if [ $rc -eq 0 ] && [ $recorder_rc -eq 0 ] && echo "$last" | grep -q "complete"; then
        echo "ok"
    else
        echo "FAILED (rc=$rc, recorder_rc=$recorder_rc, last: $last)"
        failures=$((failures + 1))
    fi

    sleep 0.5
done

echo ""
echo "done. new captures:"
ls -dt "$OUTDIR"/20* 2>/dev/null | head -20

if [ "$failures" -gt 0 ]; then
    echo "$failures scenario(s) failed"
    exit 1
fi

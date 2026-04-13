#!/bin/bash
# Record capture scenarios through the ibkr-recorder proxy.
# Usage: ./scripts/record-scenarios.sh [scenario...]
# If no scenarios are given, records the catalog batch named by
# IBKR_CAPTURE_BATCH, defaulting to new-v2. Explicit scenarios may be passed as
# "name" or "name|client_id".

UPSTREAM="${IBKR_UPSTREAM:-127.0.0.1:4002}"
LISTEN="${IBKR_LISTEN:-127.0.0.1:4101}"
OUTDIR="${IBKR_CAPTURES:-captures}"
RECORDER="${IBKR_RECORDER:-/tmp/ibkr-recorder}"
CAPTURE="${IBKR_CAPTURE:-/tmp/ibkr-capture}"
BATCH="${IBKR_CAPTURE_BATCH:-new-v2}"
TMPLOG=$(mktemp)
trap "rm -f $TMPLOG" EXIT

if [ $# -gt 0 ]; then
    SCENARIOS=()
    for scenario in "$@"; do
        if [[ "$scenario" == *"|"* ]]; then
            SCENARIOS+=("$scenario")
        else
            SCENARIOS+=("$scenario|1")
        fi
    done
else
    mapfile -t SCENARIOS < <("$CAPTURE" -list-batch "$BATCH")
fi

if [ ${#SCENARIOS[@]} -eq 0 ]; then
    echo "no scenarios found for batch $BATCH"
    exit 1
fi

mkdir -p "$OUTDIR"

for entry in "${SCENARIOS[@]}"; do
    scenario="${entry%|*}"
    client_id="${entry#*|}"
    printf "recording %-40s client_id=%-3s " "$scenario" "$client_id"

    # Start recorder in background, suppress all output
    "$RECORDER" \
        -upstream "$UPSTREAM" \
        -listen "$LISTEN" \
        -out "$OUTDIR" \
        -scenario "$scenario" \
        -client-id "$client_id" \
        -notes "batch=$BATCH client_id=$client_id" \
        >/dev/null 2>&1 &
    rpid=$!

    # Give recorder a moment to bind. Do not probe the TCP port here: the
    # recorder is intentionally one-leg-per-scenario, so a readiness probe would
    # consume the capture connection.
    sleep 0.5

    # Run capture, write output to temp file
    "$CAPTURE" \
        -addr "$LISTEN" \
        -scenario "$scenario" \
        -client-id "$client_id" \
        >"$TMPLOG" 2>&1
    rc=$?

    # Wait for recorder to finish
    wait "$rpid" 2>/dev/null

    latest_dir=$(ls -dt "$OUTDIR"/20*-"$scenario" 2>/dev/null | head -1)
    if [ -n "$latest_dir" ]; then
        cp "$TMPLOG" "$latest_dir/driver.log"
    fi

    last=$(tail -1 "$TMPLOG")
    if [ $rc -eq 0 ] && echo "$last" | grep -q "complete"; then
        echo "ok"
    else
        echo "FAILED (rc=$rc, last: $last)"
    fi

    sleep 0.5
done

echo ""
echo "done. new captures:"
ls -dt "$OUTDIR"/20* 2>/dev/null | head -20

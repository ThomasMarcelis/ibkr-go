#!/bin/bash
# Record capture scenarios through the ibkr-recorder proxy.
# Usage: ./scripts/record-scenarios.sh [scenario...]
# If no scenarios given, records all new v2 scenarios.

UPSTREAM="${IBKR_UPSTREAM:-127.0.0.1:4002}"
LISTEN="${IBKR_LISTEN:-127.0.0.1:4101}"
LISTEN_HOST="${LISTEN%%:*}"
LISTEN_PORT="${LISTEN##*:}"
OUTDIR="${IBKR_CAPTURES:-captures}"
RECORDER="${IBKR_RECORDER:-/tmp/ibkr-recorder}"
CAPTURE="${IBKR_CAPTURE:-/tmp/ibkr-capture}"
TMPLOG=$(mktemp)
trap "rm -f $TMPLOG" EXIT

ALL_SCENARIOS=(
    soft_dollar_tiers
    display_groups
    display_group_subscribe
    wsh_meta_data
    wsh_event_data_aapl
    request_fa
    fundamental_data_aapl
    qualify_contract_aapl_exact
    qualify_contract_ambiguous
    place_order_lmt_buy_aapl
    place_order_cancel
    place_order_modify
    place_order_mkt_buy_aapl
    place_order_mkt_sell_aapl
    place_order_bracket_aapl
    global_cancel
    market_depth_aapl
    market_depth_aapl_smart
    place_order_option_buy
)

if [ $# -gt 0 ]; then
    SCENARIOS=("$@")
else
    SCENARIOS=("${ALL_SCENARIOS[@]}")
fi

mkdir -p "$OUTDIR"

for scenario in "${SCENARIOS[@]}"; do
    printf "recording %-40s " "$scenario"

    # Start recorder in background, suppress all output
    "$RECORDER" \
        -upstream "$UPSTREAM" \
        -listen "$LISTEN" \
        -out "$OUTDIR" \
        -scenario "$scenario" \
        -client-id 1 \
        >/dev/null 2>&1 &
    rpid=$!

    # Wait for recorder to be listening (up to 3s)
    for _i in $(seq 1 30); do
        (echo >/dev/tcp/"${LISTEN_HOST}"/"${LISTEN_PORT}") 2>/dev/null && break
        sleep 0.1
    done

    # Run capture, write output to temp file
    "$CAPTURE" \
        -addr "$LISTEN" \
        -scenario "$scenario" \
        -client-id 1 \
        >"$TMPLOG" 2>&1
    rc=$?

    # Wait for recorder to finish
    wait "$rpid" 2>/dev/null

    last=$(tail -1 "$TMPLOG")
    if [ $rc -eq 0 ] && echo "$last" | grep -q "complete"; then
        echo "ok"
    else
        echo "FAILED (rc=$rc, last: $last)"
    fi

    # Wait for listen port to be released (up to 2s)
    for _i in $(seq 1 20); do
        (echo >/dev/tcp/"${LISTEN_HOST}"/"${LISTEN_PORT}") 2>/dev/null || break
        sleep 0.1
    done
done

echo ""
echo "done. new captures:"
ls -dt "$OUTDIR"/20* 2>/dev/null | head -20

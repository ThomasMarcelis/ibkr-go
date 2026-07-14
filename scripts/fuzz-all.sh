#!/usr/bin/env bash

set -euo pipefail

duration="${1:-30s}"
targets=(
    './internal/wire:FuzzParseFields'
    './internal/wire:FuzzEncodeParseFieldsRoundTrip'
    './internal/wire:FuzzReadFrame'
    './internal/wire:FuzzWriteFrameRoundTrip'
    './internal/codec:FuzzDecodeBatch'
    './internal/codec:FuzzEncodeDecodeRoundTrip_TickPrice'
    './internal/codec:FuzzEncodeDecodeRoundTrip_AccountSummaryValue'
    './internal/codec:FuzzEncodeDecodeRoundTrip_PnLValue'
    './internal/codec:FuzzEncodeDecodeRoundTrip_TickReqParams'
    './internal/codec:FuzzEncodeDecodeRoundTrip_HeadTimestamp'
    './internal/codec:FuzzEncodeDecodeRoundTrip_OrderStatus'
    './internal/codec:FuzzEncodeDecodeRoundTrip_ExecutionDetail'
    './internal/codec:FuzzEncodeDecodeRoundTrip_CommissionReport'
    './internal/codec:FuzzEncodeDecodeRoundTrip_MarketDepthUpdate'
    './internal/codec:FuzzEncodeDecodeRoundTrip_MarketDepthL2Update'
    './internal/codec:FuzzEncodeDecodeRoundTrip_DisplayGroupList'
    './internal/codec:FuzzEncodeDecodeRoundTrip_HistoricalDataUpdate'
)

actual=()
for pkg in ./internal/wire ./internal/codec; do
    while IFS= read -r target; do
        [[ "$target" == Fuzz* ]] || continue
        actual+=("${pkg}:${target}")
    done < <(go test -list '^Fuzz' "$pkg")
done

if ! diff -u <(printf '%s\n' "${targets[@]}") <(printf '%s\n' "${actual[@]}"); then
    echo 'fuzz target inventory changed; update scripts/fuzz-all.sh intentionally' >&2
    exit 1
fi

if [[ "$duration" == '--check' ]]; then
    exit 0
fi

for entry in "${targets[@]}"; do
    pkg="${entry%%:*}"
    target="${entry#*:}"
    go test "$pkg" -run='^$' -fuzz="^${target}$" -fuzztime="$duration"
done

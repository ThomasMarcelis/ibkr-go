#!/usr/bin/env bash

set -euo pipefail

duration="${1:-30s}"
targets=(
    './internal/wire:FuzzParseFields'
    './internal/wire:FuzzEncodeParseFieldsRoundTrip'
    './internal/wire:FuzzReadFrame'
    './internal/wire:FuzzWriteFrameRoundTrip'
    './internal/codec:FuzzDecodeBatch'
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

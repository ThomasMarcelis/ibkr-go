#!/usr/bin/env bash
# Normalize captures and print a protocol-aware integrity summary.
# Usage: ./scripts/verify-captures.sh [capture_dir]

set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_DIR"
WORKDIR=""
cleanup() {
    if [[ -n "$WORKDIR" ]]; then
        rm -rf "$WORKDIR"
    fi
}
trap cleanup EXIT

if [[ -n "${IBKR_NORMALIZE:-}" ]]; then
    NORMALIZE="$IBKR_NORMALIZE"
else
    WORKDIR=$(mktemp -d)
    NORMALIZE="$WORKDIR/ibkr-normalize"
    go build -o "$NORMALIZE" ./cmd/ibkr-normalize
fi

verify() {
    local dir="$1"
    if [[ ! -f "$dir/events.jsonl" ]]; then
        echo "!! ${dir##*/}: missing events.jsonl"
        return 1
    fi
    "$NORMALIZE" -dir "$dir" -verify
}

if [[ $# -gt 0 ]]; then
    verify "$1"
else
    echo "=== capture verification ==="
    for dir in captures/20*; do
        [[ -d "$dir" ]] || continue
        verify "$dir"
    done
fi

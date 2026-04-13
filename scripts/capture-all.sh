#!/usr/bin/env bash
# Drive every ibkr-capture scenario through ibkr-recorder in sequence.
# Each scenario produces its own capture directory under captures/.
#
# Usage: ./scripts/capture-all.sh
#
# Requirements:
#  - IB Gateway / TWS running on 127.0.0.1:4002
#  - ./ibkr-capture and ./ibkr-recorder built at repo root (or IBKR_CAPTURE /
#    IBKR_RECORDER pointing at binaries)
#
# Notes:
#  - The recorder listens on 127.0.0.1:4101 and proxies to :4002.
#  - Between scenarios we sleep 3s and wait for the accept queue to drain so
#    we don't overrun the gateway (which happened once during early probing).
#  - The open_orders_all scenario uses client-id 0 (required for the scope).
#  - The script aborts on first scenario failure so the problem is visible.

set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_DIR"

UPSTREAM="${IBKR_UPSTREAM:-127.0.0.1:4002}"
LISTEN="${IBKR_LISTEN:-127.0.0.1:4101}"
BATCH="${IBKR_CAPTURE_BATCH:-all}"
CAPTURE_BIN="${IBKR_CAPTURE:-./ibkr-capture}"
RECORDER_BIN="${IBKR_RECORDER:-./ibkr-recorder}"
RECORDER_MAX_LEGS="${IBKR_RECORDER_MAX_LEGS:-1}"

mapfile -t SCENARIOS < <("$CAPTURE_BIN" -list-batch "$BATCH")
if [[ ${#SCENARIOS[@]} -eq 0 ]]; then
  echo "no scenarios found for batch $BATCH"
  exit 1
fi

run_scenario() {
  local scenario="$1"
  local client_id="$2"

  echo "=== [$(date +%H:%M:%S)] scenario: $scenario (client_id=$client_id) ==="

  "$RECORDER_BIN" \
    -listen "$LISTEN" \
    -upstream "$UPSTREAM" \
    -scenario "$scenario" \
    -out captures \
    -client-id "$client_id" \
    -max-legs "$RECORDER_MAX_LEGS" \
    -notes "automated capture run, batch=$BATCH, client_id=$client_id" \
    > "/tmp/ibkr-recorder-${scenario}.log" 2>&1 &
  local recorder_pid=$!

  # Give recorder 500ms to bind.
  sleep 0.5

  if ! "$CAPTURE_BIN" \
        -addr "$LISTEN" \
        -client-id "$client_id" \
        -scenario "$scenario" \
        -driver-events "/tmp/ibkr-driver-events-${scenario}.jsonl" \
        > "/tmp/ibkr-capture-${scenario}.log" 2>&1; then
    tail -40 "/tmp/ibkr-capture-${scenario}.log" || true
    echo "!!! scenario $scenario FAILED; killing recorder"
    kill "$recorder_pid" 2>/dev/null || true
    wait "$recorder_pid" 2>/dev/null || true
    return 1
  fi

  cat "/tmp/ibkr-capture-${scenario}.log"

  # Give recorder a moment to flush final chunks before killing it.
  sleep 0.5
  kill "$recorder_pid" 2>/dev/null || true
  wait "$recorder_pid" 2>/dev/null || true

  local latest_dir
  latest_dir=$(ls -dt captures/20*-"$scenario" 2>/dev/null | head -1)
  if [[ -n "$latest_dir" ]]; then
    cp "/tmp/ibkr-capture-${scenario}.log" "$latest_dir/driver.log"
    if [[ -s "/tmp/ibkr-driver-events-${scenario}.jsonl" ]]; then
      cp "/tmp/ibkr-driver-events-${scenario}.jsonl" "$latest_dir/driver_events.jsonl"
    fi
  fi

  # Let gateway accept queue drain between scenarios.
  sleep 2
}

echo "capture run started at $(date)"
for entry in "${SCENARIOS[@]}"; do
  scenario="${entry%|*}"
  client_id="${entry#*|}"
  run_scenario "$scenario" "$client_id"
done
echo "capture run complete at $(date)"
echo
echo "captured directories:"
ls -1 captures/

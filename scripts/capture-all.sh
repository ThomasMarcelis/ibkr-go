#!/usr/bin/env bash
# Drive every ibkr-capture scenario through ibkr-recorder in sequence.
# Each scenario produces its own capture directory under captures/.
#
# Usage: ./scripts/capture-all.sh
#
# Requirements:
#  - IB Gateway / TWS running on 127.0.0.1:4002
#  - ./ibkr-capture and ./ibkr-recorder built at repo root (go build ./cmd/...)
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

SCENARIOS=(
  "bootstrap|1"
  "bootstrap_client_id_0|0"
  "contract_details_aapl_stk|1"
  "contract_details_aapl_opt|1"
  "contract_details_eurusd_cash|1"
  "contract_details_es_fut|1"
  "contract_details_not_found|1"
  "account_summary_snapshot|1"
  "account_summary_stream|1"
  "account_summary_two_subs|1"
  "positions_snapshot|1"
  "historical_bars_1d_1h|1"
  "historical_bars_30d_1day|1"
  "historical_bars_bidask|1"
  "historical_bars_error|1"
  "quote_snapshot_aapl|1"
  "quote_stream_aapl|1"
  "quote_stream_genericticks|1"
  "quote_stream_multi_asset|1"
  "realtime_bars_aapl|1"
  "open_orders_empty|1"
  "open_orders_all|0"
  "executions_snapshot|1"
  "historical_ticks_aapl_timezone_window|1"
  "historical_news_aapl_timezone_window|1"
  "place_order_oca_pair_aapl|1"
  "trading_split_round_trip_aapl|1"
)

run_scenario() {
  local scenario="$1"
  local client_id="$2"

  echo "=== [$(date +%H:%M:%S)] scenario: $scenario (client_id=$client_id) ==="

  ./ibkr-recorder \
    -listen "$LISTEN" \
    -upstream "$UPSTREAM" \
    -scenario "$scenario" \
    -out captures \
    -client-id "$client_id" \
    -notes "automated capture run, client_id=$client_id" \
    > "/tmp/ibkr-recorder-${scenario}.log" 2>&1 &
  local recorder_pid=$!

  # Give recorder 500ms to bind.
  sleep 0.5

  if ! ./ibkr-capture \
        -addr "$LISTEN" \
        -client-id "$client_id" \
        -scenario "$scenario"; then
    echo "!!! scenario $scenario FAILED; killing recorder"
    kill "$recorder_pid" 2>/dev/null || true
    wait "$recorder_pid" 2>/dev/null || true
    return 1
  fi

  # Give recorder a moment to flush final chunks before killing it.
  sleep 0.5
  kill "$recorder_pid" 2>/dev/null || true
  wait "$recorder_pid" 2>/dev/null || true

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

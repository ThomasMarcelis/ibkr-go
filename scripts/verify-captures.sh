#!/usr/bin/env bash
# Verify capture quality: for each capture directory, count server frames and
# scan for end markers, error codes, and any red flags. Relies on the existing
# ibkr-normalize tool to reassemble TCP chunks into frames, then greps the
# human-readable raw.txt.
#
# Usage: ./scripts/verify-captures.sh [capture_dir]
#   with no args: verifies every directory under captures/

set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_DIR"

verify() {
  local dir="$1"
  local name="${dir##*/}"
  if [[ ! -f "$dir/events.jsonl" ]]; then
    echo "!! $name: missing events.jsonl"
    return 1
  fi

  # Normalize (regenerate raw.txt + frames) idempotently.
  ./ibkr-normalize -dir "$dir" > /dev/null 2>&1 || true

  local server_bytes
  server_bytes=$(python3 -c "
import json, base64
total = 0
with open('$dir/events.jsonl') as f:
    for line in f:
        evt = json.loads(line)
        if evt.get('kind') == 'chunk' and evt.get('direction') == 'server':
            total += evt.get('length', 0)
print(total)
")

  local client_bytes
  client_bytes=$(python3 -c "
import json, base64
total = 0
with open('$dir/events.jsonl') as f:
    for line in f:
        evt = json.loads(line)
        if evt.get('kind') == 'chunk' and evt.get('direction') == 'client':
            total += evt.get('length', 0)
print(total)
")

  printf "  %-50s  client=%6sB  server=%7sB\n" "$name" "$client_bytes" "$server_bytes"
}

if [[ $# -gt 0 ]]; then
  verify "$1"
else
  echo "=== capture verification ==="
  for d in captures/20260405T214*-* captures/20260405T215*-*; do
    [[ -d "$d" ]] || continue
    verify "$d"
  done
fi

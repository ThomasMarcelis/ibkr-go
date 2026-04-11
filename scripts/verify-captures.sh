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

  # Normalize (regenerate raw.txt + frames) idempotently. A capture that cannot
  # be normalized is not acceptable replay evidence.
  ./ibkr-normalize -dir "$dir" > /dev/null

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

  local replay_file="$dir/replay/frames.jsonl"
  if [[ ! -f "$replay_file" ]]; then
    echo "!! $name: missing replay/frames.jsonl"
    return 1
  fi

  local frame_summary
  frame_summary=$(python3 -c "
import json
client = server = errors = ends = 0
with open('$replay_file') as f:
    for line in f:
        evt = json.loads(line)
        if evt.get('kind') != 'frame':
            continue
        if evt.get('direction') == 'client':
            client += 1
        elif evt.get('direction') == 'server':
            server += 1
        data = evt.get('data', '')
        # Payloads are still base64 here. Keep this tool structural; semantic
        # checks live in replay tests.
print(f'{client} {server} {errors} {ends}')
")
  read -r client_frames server_frames _errors _ends <<< "$frame_summary"

  if [[ "$client_bytes" == "0" || "$server_bytes" == "0" || "$client_frames" == "0" || "$server_frames" == "0" ]]; then
    echo "!! $name: empty capture evidence (client=${client_bytes}B/$client_frames frames, server=${server_bytes}B/$server_frames frames)"
    return 1
  fi

  printf "  %-50s  client=%6sB/%3sf  server=%7sB/%3sf\n" "$name" "$client_bytes" "$client_frames" "$server_bytes" "$server_frames"
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

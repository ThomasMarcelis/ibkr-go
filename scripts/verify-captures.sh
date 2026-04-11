#!/usr/bin/env bash
# Verify capture quality: for each capture directory, count server frames and
# scan for message IDs, end markers, API errors, server version, and evidence
# hashes. Relies on ibkr-normalize to reassemble TCP chunks into frames.
#
# Usage: ./scripts/verify-captures.sh [capture_dir]
#   with no args: verifies every directory under captures/

set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_DIR"
NORMALIZE="${IBKR_NORMALIZE:-./ibkr-normalize}"

verify() {
  local dir="$1"
  local name="${dir##*/}"
  if [[ ! -f "$dir/events.jsonl" ]]; then
    echo "!! $name: missing events.jsonl"
    return 1
  fi

  # Normalize (regenerate raw.txt + frames) idempotently. A capture that cannot
  # be normalized is not acceptable replay evidence.
  "$NORMALIZE" -dir "$dir" > /dev/null

  local replay_file="$dir/replay/frames.jsonl"
  if [[ ! -f "$replay_file" ]]; then
    echo "!! $name: missing replay/frames.jsonl"
    return 1
  fi

  python3 - "$dir" <<'PY'
import base64
import collections
import hashlib
import json
import os
import sys

capture_dir = sys.argv[1]
name = os.path.basename(capture_dir)
events_path = os.path.join(capture_dir, "events.jsonl")
replay_path = os.path.join(capture_dir, "replay", "frames.jsonl")

with open(events_path, "rb") as f:
    events_bytes = f.read()
events_hash = hashlib.sha256(events_bytes).hexdigest()[:16]

client_bytes = 0
server_bytes = 0
connects = 0
disconnects = 0
for line in events_bytes.splitlines():
    evt = json.loads(line)
    kind = evt.get("kind") or "chunk"
    if kind == "connect":
        connects += 1
    elif kind == "disconnect":
        disconnects += 1
    elif kind == "chunk":
        if evt.get("direction") == "client":
            client_bytes += int(evt.get("length") or 0)
        elif evt.get("direction") == "server":
            server_bytes += int(evt.get("length") or 0)

hist = {
    "client": collections.Counter(),
    "server": collections.Counter(),
}
frame_counts = collections.Counter()
errors = []
end_ids = {
    "52": "contract_details_end",
    "53": "open_order_end",
    "55": "executions_end",
    "57": "tick_snapshot_end",
    "62": "position_end",
    "64": "account_summary_end",
    "72": "position_multi_end",
    "74": "account_update_multi_end",
    "76": "sec_def_opt_params_end",
    "87": "historical_news_end",
    "102": "completed_orders_end",
}
ends = collections.Counter()
server_version = ""

with open(replay_path, "r", encoding="utf-8") as f:
    for line in f:
        evt = json.loads(line)
        if evt.get("kind") != "frame":
            continue
        direction = evt.get("direction") or "unknown"
        frame_counts[direction] += 1
        payload = base64.b64decode(evt.get("data") or "")
        fields = payload.decode("utf-8", "replace").split("\x00")
        if fields and fields[-1] == "":
            fields.pop()
        if not fields:
            hist[direction]["empty"] += 1
            continue
        msg_id = fields[0]
        hist[direction][msg_id] += 1
        if direction == "server" and not server_version and len(fields) == 2 and msg_id.isdigit():
            server_version = msg_id
        if direction == "server" and msg_id == "4" and len(fields) >= 4:
            errors.append((fields[1], fields[2], fields[3]))
        if direction == "server" and msg_id in end_ids:
            ends[end_ids[msg_id]] += 1

client_frames = frame_counts["client"]
server_frames = frame_counts["server"]
if client_bytes == 0 or server_bytes == 0 or client_frames == 0 or server_frames == 0:
    print(
        f"!! {name}: empty capture evidence "
        f"(client={client_bytes}B/{client_frames}f, server={server_bytes}B/{server_frames}f)"
    )
    sys.exit(1)
if connects != disconnects:
    print(f"!! {name}: connect/disconnect mismatch connect={connects} disconnect={disconnects}")
    sys.exit(1)

def format_hist(counter):
    parts = [f"{key}:{counter[key]}" for key in sorted(counter, key=lambda x: (not x.isdigit(), int(x) if x.isdigit() else x))]
    return ",".join(parts) if parts else "-"

print(
    f"  {name:<50} client={client_bytes:6d}B/{client_frames:3d}f "
    f"server={server_bytes:7d}B/{server_frames:3d}f "
    f"server_version={server_version or '?'} sha256={events_hash}"
)
print(f"    client_msg_ids: {format_hist(hist['client'])}")
print(f"    server_msg_ids: {format_hist(hist['server'])}")
if ends:
    print("    end_markers: " + ", ".join(f"{name}:{count}" for name, count in sorted(ends.items())))
if errors:
    shown = "; ".join(f"req={req} code={code} msg={msg[:80]}" for req, code, msg in errors[:8])
    suffix = "" if len(errors) <= 8 else f"; ... +{len(errors) - 8} more"
    print(f"    api_errors: {shown}{suffix}")
PY
}

if [[ $# -gt 0 ]]; then
  verify "$1"
else
  echo "=== capture verification ==="
  for d in captures/20*; do
    [[ -d "$d" ]] || continue
    verify "$d"
  done
fi

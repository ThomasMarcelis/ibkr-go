#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
usage: scripts/scan-ibkr-sdk-drift.sh [SDK_DIR]

Checks that the native adapter's current EClientSocket requests and
DefaultEWrapper callbacks still exist in an installed official IBKR C++ SDK.
SDK_DIR defaults to IBKR_TWS_API_DIR.
USAGE
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
sdk_dir="${1:-${IBKR_TWS_API_DIR:-}}"
if [[ -z "$sdk_dir" ]]; then
  echo "IBKR_TWS_API_DIR is not set and no SDK_DIR argument was provided." >&2
  exit 2
fi
if [[ ! -d "$sdk_dir" ]]; then
  echo "IBKR SDK directory does not exist: $sdk_dir" >&2
  exit 2
fi
sdk_dir="$(cd "$sdk_dir" && pwd -P)"

tmp_check="$(mktemp)"
trap 'rm -f "$tmp_check"' EXIT
if ! "$repo_root/scripts/check-ibkr-sdk-env.sh" "$sdk_dir" >"$tmp_check" 2>&1; then
  cat "$tmp_check" >&2
  exit 2
fi

required_headers=(
  EClient.h
  EClientSocket.h
  EWrapper.h
  DefaultEWrapper.h
)

has_required_headers() {
  local dir="$1"
  local header
  for header in "${required_headers[@]}"; do
    if [[ ! -f "$dir/$header" ]]; then
      return 1
    fi
  done
}

client_dir=""
for candidate in \
  "$sdk_dir/source/cppclient/client" \
  "$sdk_dir/source/CppClient/client" \
  "$sdk_dir/Source/cppclient/client" \
  "$sdk_dir/Source/CppClient/client" \
  "$sdk_dir/IBJts/source/cppclient/client" \
  "$sdk_dir/IBJts/source/CppClient/client" \
  "$sdk_dir/IBJts/Source/cppclient/client" \
  "$sdk_dir/IBJts/Source/CppClient/client"
do
  if has_required_headers "$candidate"; then
    client_dir="$candidate"
    break
  fi
done

if [[ -z "$client_dir" ]]; then
  while IFS= read -r candidate; do
    candidate_dir="$(dirname "$candidate")"
    if has_required_headers "$candidate_dir"; then
      client_dir="$candidate_dir"
      break
    fi
  done < <(find "$sdk_dir" -maxdepth 6 -type f -name EClientSocket.h 2>/dev/null | sort)
fi

if [[ -z "$client_dir" ]]; then
  echo "could not find complete C++ client headers under $sdk_dir" >&2
  exit 2
fi

adapter_cpp="$repo_root/internal/sdkadapter/native/adapter.cpp"
if [[ ! -f "$adapter_cpp" ]]; then
  echo "missing adapter source: $adapter_cpp" >&2
  exit 2
fi

mapfile -t request_methods < <(
  sed -n 's/.*client_->\([A-Za-z_][A-Za-z0-9_]*\)(.*/\1/p' "$adapter_cpp" |
    sort -u |
    sed '/^eConnect$/d;/^eDisconnect$/d;/^isConnected$/d'
)

mapfile -t callback_methods < <(
  sed -n 's/^[[:space:]]*void \([A-Za-z_][A-Za-z0-9_]*\)(.* override.*/\1/p' "$adapter_cpp" |
    sort -u
)

missing=0
request_headers=("$client_dir/EClientSocket.h" "$client_dir/EClient.h")
callback_headers=("$client_dir/EWrapper.h" "$client_dir/DefaultEWrapper.h")
if [[ -f "$client_dir/EWrapper_prototypes.h" ]]; then
  callback_headers+=("$client_dir/EWrapper_prototypes.h")
fi
if [[ -f "$client_dir/DefaultEWrapper.cpp" ]]; then
  callback_headers+=("$client_dir/DefaultEWrapper.cpp")
fi

for method in "${request_methods[@]}"; do
  if ! grep -Eq "[[:space:]*&]${method}[[:space:]]*\\(" "${request_headers[@]}"; then
    echo "missing EClient request signature: $method" >&2
    missing=1
  fi
done

for method in "${callback_methods[@]}"; do
  if ! grep -Eq "[[:space:]*&]${method}[[:space:]]*\\(" "${callback_headers[@]}"; then
    echo "missing EWrapper callback signature: $method" >&2
    missing=1
  fi
done

if [[ "$missing" -ne 0 ]]; then
  exit 1
fi

echo "IBKR SDK drift scan OK"
echo "  SDK dir:          $sdk_dir"
echo "  C++ headers:      $client_dir"
echo "  adapter requests: ${#request_methods[@]}"
echo "  callbacks:        ${#callback_methods[@]}"

#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
usage: scripts/check-ibkr-swig-env.sh [--print-env] [SDK_DIR]

Checks the local toolchain and an externally installed official IBKR TWS API
SDK. The SDK is never vendored into this repository.

SDK_DIR defaults to IBKR_TWS_API_DIR.

Examples:
  export IBKR_TWS_API_DIR="$HOME/IBJts"
  scripts/check-ibkr-swig-env.sh

  eval "$(scripts/check-ibkr-swig-env.sh --print-env)"
  go test -tags=ibkr_swig ./internal/ibkrsdkprobe
USAGE
}

print_env=0
if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi
if [[ "${1:-}" == "--print-env" ]]; then
  print_env=1
  shift
fi

sdk_dir="${1:-${IBKR_TWS_API_DIR:-}}"
if [[ -z "$sdk_dir" ]]; then
  echo "IBKR_TWS_API_DIR is not set and no SDK_DIR argument was provided." >&2
  echo "Download the official IBKR TWS API Mac/Unix package, accept the IBKR license, unzip it, then set IBKR_TWS_API_DIR to the SDK root." >&2
  exit 2
fi

if [[ ! -d "$sdk_dir" ]]; then
  echo "IBKR SDK directory does not exist: $sdk_dir" >&2
  exit 2
fi

need_cmd() {
  local name="$1"
  if ! command -v "$name" >/dev/null 2>&1; then
    echo "missing required command: $name" >&2
    exit 2
  fi
}

need_cmd go
need_cmd swig
need_cmd g++
need_cmd make
need_cmd protoc
need_cmd pkg-config

if ! pkg-config --exists protobuf; then
  echo "pkg-config cannot find protobuf. Install protobuf-devel or set PKG_CONFIG_PATH." >&2
  exit 2
fi

version_file=""
while IFS= read -r candidate; do
  version_file="$candidate"
  break
done < <(find "$sdk_dir" -maxdepth 4 -type f -name API_VersionNum.txt 2>/dev/null | sort)

if [[ -z "$version_file" ]]; then
  echo "could not find API_VersionNum.txt under $sdk_dir" >&2
  exit 2
fi

client_dir=""
for candidate in \
  "$sdk_dir/source/cppclient/client" \
  "$sdk_dir/source/CppClient/client" \
  "$sdk_dir/IBJts/source/cppclient/client" \
  "$sdk_dir/IBJts/source/CppClient/client"
do
  if [[ -f "$candidate/EClient.h" && -f "$candidate/EWrapper.h" ]]; then
    client_dir="$candidate"
    break
  fi
done

if [[ -z "$client_dir" ]]; then
  while IFS= read -r candidate; do
    candidate_dir="$(dirname "$candidate")"
    if [[ -f "$candidate_dir/EWrapper.h" ]]; then
      client_dir="$candidate_dir"
      break
    fi
  done < <(find "$sdk_dir" -maxdepth 6 -type f -name EClient.h 2>/dev/null | sort)
fi

if [[ -z "$client_dir" ]]; then
  echo "could not find C++ client headers EClient.h and EWrapper.h under $sdk_dir" >&2
  exit 2
fi

lib_path=""
while IFS= read -r candidate; do
  lib_path="$candidate"
  break
done < <(find "$sdk_dir" -maxdepth 8 -type f -name libTwsSocketClient.a 2>/dev/null | sort)

if [[ -z "$lib_path" ]]; then
  echo "could not find libTwsSocketClient.a under $sdk_dir" >&2
  echo "If the official package only contains source, build the POSIX C++ sample/library first and rerun this check." >&2
  exit 2
fi

include_dirs=("$client_dir")
for candidate in \
  "$client_dir/protobuf" \
  "$client_dir/protobufUnix" \
  "$client_dir/../protobuf" \
  "$client_dir/../protobufUnix"
do
  if [[ -d "$candidate" ]]; then
    include_dirs+=("$candidate")
  fi
done

lib_dir="$(dirname "$lib_path")"

if [[ "$print_env" -eq 1 ]]; then
  printf 'export IBKR_TWS_API_DIR=%q\n' "$sdk_dir"
  printf 'export CGO_CXXFLAGS='
  printf '%q ' "-std=c++14"
  for dir in "${include_dirs[@]}"; do
    printf '%q ' "-I$dir"
  done
  printf '\n'
  printf 'export CGO_LDFLAGS='
  printf '%q ' "-L$lib_dir" "-lTwsSocketClient"
  # shellcheck disable=SC2046
  printf '%q ' $(pkg-config --libs protobuf)
  printf '%q\n' "-lpthread"
  exit 0
fi

echo "IBKR SWIG environment OK"
echo "  SDK dir:       $sdk_dir"
echo "  API version:   $(tr -d '\r\n' < "$version_file")"
echo "  C++ headers:   $client_dir"
echo "  static lib:    $lib_path"
echo "  Go:            $(go version)"
echo "  SWIG:          $(swig -version | sed -n '1p')"
echo "  g++:           $(g++ -dumpfullversion -dumpversion)"
echo "  protoc:        $(protoc --version)"
echo "  protobuf pc:   $(pkg-config --modversion protobuf)"


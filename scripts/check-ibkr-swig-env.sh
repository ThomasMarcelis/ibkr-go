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
sdk_dir="$(cd "$sdk_dir" && pwd -P)"

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

required_headers=(
  EClientSocket.h
  EWrapper.h
  DefaultEWrapper.h
  EReader.h
  EReaderOSSignal.h
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
  echo "could not find required C++ client headers under $sdk_dir" >&2
  echo "required headers: ${required_headers[*]}" >&2
  exit 2
fi

lib_path=""
for lib_name in libTwsSocketClient.a libTwsSocketClient.so libtwsapi.so; do
  while IFS= read -r candidate; do
    lib_path="$candidate"
    break
  done < <(find "$sdk_dir" -maxdepth 8 -type f -name "$lib_name" 2>/dev/null | sort)
  if [[ -n "$lib_path" ]]; then
    break
  fi
done

if [[ -z "$lib_path" ]]; then
  echo "could not find a built C++ SDK library under $sdk_dir" >&2
  echo "looked for: libTwsSocketClient.a, libTwsSocketClient.so, libtwsapi.so" >&2
  echo "The 10.46 Mac/Unix SDK ships source only on Linux. Regenerate protobufUnix if your system protobuf differs, build Intel libbid, then build the POSIX C++ library and rerun this check." >&2
  exit 2
fi

include_dirs=()
add_include_dir() {
  local dir="$1"
  local existing
  [[ -d "$dir" ]] || return 0
  for existing in "${include_dirs[@]}"; do
    if [[ "$existing" == "$dir" ]]; then
      return 0
    fi
  done
  include_dirs+=("$dir")
}

add_include_dir "$client_dir"
for candidate in \
  "$client_dir/protobuf" \
  "$client_dir/protobufUnix" \
  "$client_dir/../protobuf" \
  "$client_dir/../protobufUnix" \
  "$client_dir/../../protobuf" \
  "$client_dir/../../protobufUnix"
do
  add_include_dir "$candidate"
done
while IFS= read -r candidate; do
  add_include_dir "$candidate"
done < <(find "$(dirname "$client_dir")" -maxdepth 3 -type d \( -name 'protobuf' -o -name 'protobufUnix' -o -name 'protobuf*' \) 2>/dev/null | sort)

lib_dir="$(dirname "$lib_path")"
lib_file="$(basename "$lib_path")"

if [[ "$print_env" -eq 1 ]]; then
  cxxflags=("-std=c++14")
  for dir in "${include_dirs[@]}"; do
    cxxflags+=("-I$dir")
  done
  # shellcheck disable=SC2046
  cxxflags+=($(pkg-config --cflags protobuf))

  ldflags=("-L$lib_dir" "-l:$lib_file")
  if [[ "$lib_file" == *.so ]]; then
    ldflags+=("-Wl,-rpath,$lib_dir")
  fi
  # shellcheck disable=SC2046
  ldflags+=($(pkg-config --libs protobuf))
  ldflags+=("-lpthread")

  printf 'export IBKR_TWS_API_DIR=%q\n' "$sdk_dir"
  printf 'export CGO_CXXFLAGS=%q\n' "${cxxflags[*]}"
  printf 'export CGO_LDFLAGS=%q\n' "${ldflags[*]}"
  exit 0
fi

echo "IBKR SWIG environment OK"
echo "  SDK dir:       $sdk_dir"
echo "  API version:   $(tr -d '\r\n' < "$version_file")"
echo "  C++ headers:   $client_dir"
echo "  include dirs:  ${include_dirs[*]}"
echo "  SDK library:   $lib_path"
echo "  Go:            $(go version)"
echo "  SWIG:          $(swig -version | sed -n '/^SWIG Version/{p;q;}')"
echo "  g++:           $(g++ -dumpfullversion -dumpversion)"
echo "  protoc:        $(protoc --version)"
echo "  protobuf pc:   $(pkg-config --modversion protobuf)"

#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
usage: scripts/check-ibkr-sdk-env.sh [--print-env] [SDK_DIR]

Checks the local toolchain and an externally installed official IBKR TWS API
SDK for the manual cgo/C++ adapter. The SDK is never vendored into this
repository.

SDK_DIR defaults to IBKR_TWS_API_DIR.

Examples:
  export IBKR_TWS_API_DIR="$HOME/IBJts"
  scripts/check-ibkr-sdk-env.sh

  eval "$(scripts/check-ibkr-sdk-env.sh --print-env)"
  go test -tags=ibkr_sdk ./...
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
  Decimal.h
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
sdk_api_version="$(tr -d '\r\n' < "$version_file")"
sdk_api_version="${sdk_api_version#API_Version=}"

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

proto_dir=""
for candidate in \
  "$client_dir/protobufUnix" \
  "$client_dir/protobuf" \
  "$client_dir/../protobufUnix" \
  "$client_dir/../protobuf" \
  "$client_dir/../../protobufUnix" \
  "$client_dir/../../protobuf"
do
  if [[ -d "$candidate" ]]; then
    proto_header=""
    while IFS= read -r found_proto_header; do
      proto_header="$found_proto_header"
      break
    done < <(find "$candidate" -maxdepth 1 -type f -name '*.pb.h' 2>/dev/null)
    if [[ -n "$proto_header" ]]; then
      proto_dir="$candidate"
      break
    fi
  fi
done

if [[ -z "$proto_dir" ]]; then
  echo "could not find generated protobuf C++ headers under the SDK client tree." >&2
  echo "For Linux SDKs, regenerate from the SDK source directory with:" >&2
  echo "  protoc --proto_path=./proto --experimental_allow_proto3_optional --cpp_out=./cppclient/client/protobufUnix proto/*.proto" >&2
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
  echo "The 10.46 Mac/Unix SDK ships source only on Linux. Regenerate protobufUnix if your system protobuf differs, build Intel libbid with PIC, then build the POSIX C++ library and rerun this check." >&2
  exit 2
fi

libbid_path=""
while IFS= read -r candidate; do
  libbid_path="$candidate"
  break
done < <(find "$sdk_dir" "$(dirname "$sdk_dir")" -maxdepth 8 -type f \( -name 'libbid.a' -o -name 'libbid.so' \) 2>/dev/null)

if [[ -z "$libbid_path" ]]; then
  echo "could not find Intel decimal libbid near the SDK tree." >&2
  echo "Build the Intel Decimal Floating-Point Math Library locally with PIC and keep it outside git tracking." >&2
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
add_include_dir "$proto_dir"

lib_dir="$(dirname "$lib_path")"
lib_file="$(basename "$lib_path")"
libbid_dir="$(dirname "$libbid_path")"
libbid_file="$(basename "$libbid_path")"

cxxflags=("-std=c++14")
for dir in "${include_dirs[@]}"; do
  cxxflags+=("-I$dir")
done
cxxflags+=("-DIBKR_SDK_API_VERSION=$sdk_api_version")
# shellcheck disable=SC2046
cxxflags+=($(pkg-config --cflags protobuf))

ldflags=("-L$lib_dir" "-l:$lib_file" "-L$libbid_dir" "-l:$libbid_file")
runtime_path=""
if [[ "$lib_file" == *.so ]]; then
  ldflags+=("-Wl,-rpath,$lib_dir")
  runtime_path="$lib_dir"
fi
if [[ "$libbid_file" == *.so ]]; then
  ldflags+=("-Wl,-rpath,$libbid_dir")
  if [[ -n "$runtime_path" && "$libbid_dir" != "$lib_dir" ]]; then
    runtime_path="$runtime_path:$libbid_dir"
  elif [[ -z "$runtime_path" ]]; then
    runtime_path="$libbid_dir"
  fi
fi
# shellcheck disable=SC2046
ldflags+=($(pkg-config --libs protobuf))
ldflags+=("-lpthread" "-lstdc++")

if ! printf 'int main() { return 0; }\n' | g++ -std=c++14 -x c++ - -o /tmp/ibkr-sdk-cxx14-check >/dev/null 2>&1; then
  echo "g++ cannot compile a minimal C++14 program." >&2
  exit 2
fi
rm -f /tmp/ibkr-sdk-cxx14-check

probe_src="/tmp/ibkr-sdk-link-check-$$.cpp"
probe_bin="/tmp/ibkr-sdk-link-check-$$"
probe_log="/tmp/ibkr-sdk-link-check-$$.log"
cat > "$probe_src" <<'CPP'
#include "DefaultEWrapper.h"
#include "EClientSocket.h"
#include "EReaderOSSignal.h"

int main() {
  EReaderOSSignal signal(1);
  DefaultEWrapper wrapper;
  EClientSocket client(&wrapper, &signal);
  return client.isConnected() ? 1 : 0;
}
CPP
if ! g++ "${cxxflags[@]}" "$probe_src" "${ldflags[@]}" -o "$probe_bin" >"$probe_log" 2>&1; then
  echo "g++ cannot link a minimal program against the IBKR SDK library." >&2
  cat "$probe_log" >&2
  rm -f "$probe_src" "$probe_bin" "$probe_log"
  exit 2
fi
if ! "$probe_bin" >"$probe_log" 2>&1; then
  echo "linked IBKR SDK probe could not run. Check runtime library paths." >&2
  cat "$probe_log" >&2
  rm -f "$probe_src" "$probe_bin" "$probe_log"
  exit 2
fi
rm -f "$probe_src" "$probe_bin" "$probe_log"

if [[ "$print_env" -eq 1 ]]; then
  printf 'export IBKR_TWS_API_DIR=%q\n' "$sdk_dir"
  printf 'export CGO_CXXFLAGS=%q\n' "${cxxflags[*]}"
  printf 'export CGO_LDFLAGS=%q\n' "${ldflags[*]}"
  if [[ -n "$runtime_path" ]]; then
    if [[ -n "${LD_LIBRARY_PATH:-}" ]]; then
      printf 'export LD_LIBRARY_PATH=%q\n' "$runtime_path:$LD_LIBRARY_PATH"
    else
      printf 'export LD_LIBRARY_PATH=%q\n' "$runtime_path"
    fi
  fi
  exit 0
fi

echo "IBKR SDK cgo environment OK"
echo "  SDK dir:       $sdk_dir"
echo "  API version:   $sdk_api_version"
echo "  C++ headers:   $client_dir"
echo "  protobuf:      $proto_dir"
echo "  SDK library:   $lib_path"
echo "  Intel libbid:  $libbid_path"
if [[ -n "$runtime_path" ]]; then
  echo "  runtime path:  $runtime_path"
else
  echo "  runtime path:  (static SDK/libbid linkage)"
fi
echo "  Go:            $(go version)"
echo "  g++:           $(g++ -dumpfullversion -dumpversion)"
echo "  protoc:        $(protoc --version)"
echo "  protobuf pc:   $(pkg-config --modversion protobuf)"
echo "  link probe:    OK"

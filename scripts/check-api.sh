#!/usr/bin/env bash
set -euo pipefail

readonly module=github.com/ThomasMarcelis/ibkr-go/v2
readonly apidiff=golang.org/x/exp/cmd/apidiff@v0.0.0-20260709172345-9ea1abe57597
readonly root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly baseline="$root/testdata/api/v2.0.0-rc.3.api"
readonly approved_breaks="$root/testdata/api/approved-breaks-after-v2.0.0-rc.3.txt"
current="$(mktemp)"
trap 'rm -f "$current"' EXIT

cd "$root"
go run "$apidiff" -w "$current" "$module"
changes="$(go run "$apidiff" -incompatible "$baseline" "$current" | grep -Fvx -f "$approved_breaks" || true)"
if [[ -n "$changes" ]]; then
	printf 'incompatible public API changes since v2.0.0-rc.3:\n%s\n' "$changes" >&2
	exit 1
fi

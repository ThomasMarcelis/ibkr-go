#!/usr/bin/env bash
set -euo pipefail

readonly module=github.com/ThomasMarcelis/ibkr-go/v2
readonly apidiff=golang.org/x/exp/cmd/apidiff@v0.0.0-20260709172345-9ea1abe57597
readonly root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly compatibility_baseline="$root/testdata/api/v2.0.2.api"
readonly exact_baseline="$root/testdata/api/v2.0.2.api"
current="$(mktemp)"
trap 'rm -f "$current"' EXIT

case "${1:-}" in
"") exact=false ;;
--exact) exact=true ;;
*)
	printf 'usage: %s [--exact]\n' "${0##*/}" >&2
	exit 2
	;;
esac
if (( $# > 1 )); then
	printf 'usage: %s [--exact]\n' "${0##*/}" >&2
	exit 2
fi

cd "$root"
go run "$apidiff" -w "$current" "$module"
if "$exact"; then
	changes="$(go run "$apidiff" "$exact_baseline" "$current")"
	message='public API differs from the v2.0.2 release manifest'
else
	changes="$(go run "$apidiff" -incompatible "$compatibility_baseline" "$current")"
	message='incompatible public API changes since v2.0.2'
fi
if [[ -n "$changes" ]]; then
	printf '%s:\n%s\n' "$message" "$changes" >&2
	exit 1
fi

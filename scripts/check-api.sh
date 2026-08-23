#!/usr/bin/env bash
set -euo pipefail

readonly module=github.com/ThomasMarcelis/ibkr-go/v2
readonly apidiff=golang.org/x/exp/cmd/apidiff@v0.0.0-20260709172345-9ea1abe57597
readonly root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly baseline="$root/testdata/api/v2.0.0.api"
readonly approved_breaks="$root/testdata/api/approved-breaks-after-v2.0.0.txt"
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
	changes="$(go run "$apidiff" "$baseline" "$current")"
	message='public API differs from the v2.0.0 release manifest'
else
	incompatible="$(go run "$apidiff" -incompatible "$baseline" "$current")"
	set +e
	changes="$(printf '%s\n' "$incompatible" | grep -Fvx -f "$approved_breaks")"
	grep_status=$?
	set -e
	if (( grep_status > 1 )); then
		exit "$grep_status"
	fi
	message='incompatible public API changes since v2.0.0'
fi
if [[ -n "$changes" ]]; then
	printf '%s:\n%s\n' "$message" "$changes" >&2
	exit 1
fi

#!/usr/bin/env bash
set -euo pipefail

readonly apidiff=golang.org/x/exp/cmd/apidiff@v0.0.0-20260709172345-9ea1abe57597
readonly root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly stable='^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'
readonly version='^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$'

fail() { printf 'API check: %s\n' "$*" >&2; exit 1; }
usage() { printf 'usage: %s [--exact | --release VERSION]\n' "${0##*/}" >&2; exit 2; }
mode=compatibility
release=
case "${1:-}" in
    '') (( $# == 0 )) || usage ;;
    --exact) (( $# == 1 )) || usage; mode=exact ;;
    --release)
        (( $# == 2 )) || usage
        mode=release
        release=$2
        [[ "$release" =~ $version ]] || fail "invalid release version: $release"
        ;;
    *) usage ;;
esac

cd "$root"
module="$(go list -m)"
major=1
if [[ "$module" =~ /v([0-9]+)$ ]]; then major=${BASH_REMATCH[1]}; fi
if [[ -n "$release" && "$release" != "v$major."* ]]; then
    fail "release $release does not match module $module"
fi
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# Prereleases compare below the stable version with the same numeric core.
# Only stable tags are candidates, so equal cores must always be excluded.
older_than_release() {
    local tag=${1#v} target=${release#v} i
    local -a left right
    IFS=. read -r -a left <<< "$tag"
    IFS=. read -r -a right <<< "${target%%-*}"
    for i in 0 1 2; do
        # Compare decimal components without integer overflow or octal parsing.
        if (( ${#left[i]} != ${#right[i]} )); then
            (( ${#left[i]} < ${#right[i]} )); return
        fi
        if [[ "${left[i]}" != "${right[i]}" ]]; then
            [[ "${left[i]}" < "${right[i]}" ]]; return
        fi
    done
    return 1
}

if [[ "$mode" != exact ]]; then
    if [[ "$(git rev-parse --is-shallow-repository)" == true ]]; then
        fail 'release history is shallow; fetch full history and tags (git fetch --unshallow --tags)'
    fi
    # The baseline is immutable tag content, never a working-tree manifest.
    git tag --merged HEAD --list "v$major.*" --sort=-version:refname > "$work/tags"
    baseline=
    while IFS= read -r tag; do
        [[ "$tag" =~ $stable ]] || continue
        if [[ "$mode" == release ]] && ! older_than_release "$tag"; then continue; fi
        baseline=$tag
        break
    done < "$work/tags"
    [[ -n "$baseline" ]] || fail "no reachable stable v$major baseline; fetch full history and tags, then check the release ancestry"
    git show "$baseline:testdata/api/$baseline.api" > "$work/baseline.api" 2>/dev/null ||
        fail "tag $baseline lacks testdata/api/$baseline.api; restore the release evidence before checking compatibility"
fi

shopt -s nullglob
manifests=(testdata/api/*.api)
(( ${#manifests[@]} == 1 )) || fail 'keep exactly one candidate manifest in testdata/api'
candidate=${manifests[0]}
if [[ "$mode" == release && "$candidate" != "testdata/api/$release.api" ]]; then
    fail "release $release requires testdata/api/$release.api; found $candidate"
fi
candidate_version=${candidate##*/}
candidate_version=${candidate_version%.api}
[[ "$candidate_version" =~ $version && "$candidate_version" == "v$major."* ]] || fail "invalid candidate version: $candidate_version"

go run "$apidiff" -w "$work/current.api" "$module"
if [[ "$mode" != compatibility ]]; then
    changes="$(go run "$apidiff" "$candidate" "$work/current.api")"
    [[ -z "$changes" ]] || fail "public API differs from $candidate:
$changes"
fi
if [[ "$mode" != exact ]]; then
    changes="$(go run "$apidiff" -incompatible "$work/baseline.api" "$work/current.api")"
    # A minor release may document a deliberate clean break. The complete
    # incompatible diff is frozen for this exact immutable baseline/candidate
    # pair; additions still flow freely. Historical records are inapplicable
    # once a newer tag becomes the baseline.
    record="testdata/api/$baseline-$candidate_version.breaks"
    if [[ -f "$record" ]]; then
        IFS= read -r migration < "$record" || fail "empty break record: $record"
        [[ "$migration" == "docs/migration-v$major."*.md && -s "$migration" ]] ||
            fail "break record $record lacks migration evidence: $migration"
        expected="$(tail -n +2 "$record")"
        [[ -n "$expected" && "$changes" == "$expected" ]] ||
            fail "incompatible diff does not exactly match $record:
$changes"
        printf 'API check: documented breaks since %s match %s; see %s\n' "$baseline" "$record" "$migration"
    else
        [[ -z "$changes" ]] || fail "incompatible public API changes since $baseline; missing $record:
$changes"
    fi
fi

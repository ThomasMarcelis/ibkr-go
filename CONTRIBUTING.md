# Contributing to ibkr-go

Contributions are welcome. Read this document first.

## Scope and direction

`ibkr-go` is an idiomatic Go client for Interactive Brokers TWS and IB Gateway.
The production runtime is the official IBKR C++ SDK behind the repo-owned C ABI
in `internal/sdkadapter/native`; public callers see Go facades, request/result
types, subscriptions, and order handles. Legacy socket/codec tooling remains
behind the non-default `legacy_native_socket` build tag as historical replay and
capture material, not as a production fallback. See [`docs/roadmap.md`](docs/roadmap.md)
for the full charter.

## Development loop

Prerequisites: Go 1.26+, `golangci-lint` (latest), and optionally `gofmt` and
`goimports` integrated into your editor. SDK-tagged checks additionally need an
official local IBKR SDK tree such as `.external/IBJts`; do not commit SDK source
or binaries.

```
go build ./...
go vet ./...
gofmt -l .           # must produce no output
golangci-lint run
go test ./...
bash -n scripts/check-ibkr-sdk-env.sh
bash -n scripts/scan-ibkr-sdk-drift.sh
scripts/check-ibkr-sdk-env.sh /path/to/IBJts
scripts/scan-ibkr-sdk-drift.sh /path/to/IBJts
eval "$(scripts/check-ibkr-sdk-env.sh --print-env /path/to/IBJts)"
go test -tags=ibkr_sdk ./...
```

The deterministic checks must pass locally before opening a pull request. CI
runs the default deterministic subset on every push and pull request against
`main`. SDK-tagged checks are local/pre-release gates because the official SDK
tree is not vendored into this repository.

## Testing discipline

The test suite is this library's primary asset. It is grown deliberately. Every test must improve confidence or diagnosis quality — tests that only prove the code was written are rejected. Every bug fix lands with the transcript or test that would have caught the bug; that test becomes a permanent regression freeze.

Tests are organised in layers: invariants, state transitions, behavioral
scenarios, stress and edge cases. Routine CI stays deterministic, but
protocol-adjacent development is grounded in the local live TWS or IB Gateway
when available. SDK behavior should be frozen as copied `internal/sdkadapter`
command/event fixtures derived from official SDK or live behavior.

The `testing/testhost`, `internal/codec`, `internal/wire`, and capture tools are
legacy replay/capture tooling behind `legacy_native_socket`. They are useful
source evidence for migration work, not a place to define new production
protocol semantics.

## Reference policy

Protocol-adjacent work should be grounded in the local live TWS or IB Gateway
when available, plus official IBKR docs, official IBKR client-library source,
captured traffic, and other IBKR library implementations where useful. The
merged implementation must still follow `ibkr-go`'s typed public API and package
philosophy rather than mirroring the official public surface mechanically.

Live smoke tests use the paper Gateway default at `127.0.0.1:4002`:

```
eval "$(scripts/check-ibkr-sdk-env.sh --print-env /path/to/IBJts)"
IBKR_LIVE=1 IBKR_LIVE_ADDR=127.0.0.1:4002 go test -tags=ibkr_sdk ./... -run '^TestLive' -count=1
```

Trading live tests are paper-only and must refuse real-account writes. They
require the explicit opt-in flag:

```
IBKR_LIVE=1 IBKR_LIVE_TRADING=1 IBKR_LIVE_ADDR=127.0.0.1:4002 go test -tags=ibkr_sdk ./... -run '^TestLive(PlaceOrder|GlobalCancel|Trading)' -count=1
```

Do not run order-placement, option exercise, FA write, or other account-mutating
checks against a real account.

## Commit convention

- Subject line: ≤72 characters, imperative mood, concrete. Lowercase after any prefix. No trailing period.
- Body (optional, only when the change earns one): focuses on **why**. What constraint, incident, or design pressure forced the change. The diff shows *what*; the body carries the context the diff cannot.
- One logical change per commit.
- Protocol-adjacent commits should mention the live environment, captures, or source/docs references that justified the change when that context matters.
- No WIP commits and no "fix typo" follow-ups in main history; squash before landing.
- No emoji, no marketing voice.

## Pull requests

Keep pull requests tight. One logical change. Tests land alongside the code. The description follows the pull request template — what, why, tests, and protocol references/verification where relevant.

## Reporting bugs and requesting features

Use the issue templates at `.github/ISSUE_TEMPLATE/`. For security-sensitive reports, follow [`SECURITY.md`](SECURITY.md) — do not open public issues for vulnerabilities.

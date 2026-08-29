# Contributing to ibkr-go

Contributions are welcome. Read this document first.

## Scope and direction

`ibkr-go` is a Go client for the Interactive Brokers TWS and IB Gateway socket protocol. The project targets the full free read-only surface plus order management, market depth (Level 2), and option exercise. The current official source baseline is API 10.50.01. The client negotiates exactly `server_version` 208..225; API 10.50.01's `Order.conditionsIncludeOvernight` requires version 226 and is intentionally outside that range. Versions 200..207 are unsupported on the v2 line; use a current Gateway or remain on v2.0.0. Implemented and externally blocked areas are distinguished in [`docs/roadmap.md`](docs/roadmap.md) and the coverage matrix.

## Development loop

Prerequisites: Go 1.26+, `golangci-lint` (latest), and optionally `gofmt` and `goimports` integrated into your editor.

```
go build ./...
go vet ./...
gofmt -l .           # must produce no output
./scripts/check-api.sh
golangci-lint run
go test ./...
```

All six must pass locally before opening a pull request. The default API check
rejects incompatible changes from the frozen v2.0.2 release baseline while
allowing additive APIs for the next release. `./scripts/check-api.sh --exact`
instead requires the complete public surface to equal v2.0.2 exactly.
Historical manifests remain unchanged.
CI also checks module tidiness and verification, fuzz-target inventory, a
pure-Go 386 build, vulnerabilities, shuffled tests across supported host
platforms, and the race detector. See `.github/workflows/ci.yml` for the exact
commands.

## Testing discipline

The test suite is this library's primary asset. It is grown deliberately. Every test must improve confidence or diagnosis quality — tests that only prove the code was written are rejected. Every bug fix lands with the transcript or test that would have caught the bug; that test becomes a permanent regression freeze.

Tests are organised in layers: invariants, state transitions, behavioral scenarios, stress and edge cases. Routine CI stays deterministic, but protocol-adjacent development is grounded in the local live TWS or IB Gateway when available. The `internal/testhost` package and checked-in fixtures are replay tools for live-derived behavior, not a source of truth for invented protocol semantics.

Wire framing and codec round-trips are also fuzzed. The intent is not just to
have broad coverage, but to keep the protocol surface diagnosable and safe to
extend without a live broker in CI.

## Live verification

Live tests are opt-in and never run in CI:

```bash
IBKR_LIVE=1 IBKR_LIVE_READONLY_ADDR=127.0.0.1:4001 go test ./... -run '^TestLive' -count=1
IBKR_LIVE=1 IBKR_LIVE_TRADING=1 IBKR_LIVE_PAPER_ADDR=127.0.0.1:4002 go test ./cmd/ibkr-capture -run '^TestLiveCapture' -count=1
```

The maintainer lab uses two Gateway roles:

- `IBKR_LIVE_READONLY_ADDR` points at the real-money, read-only Gateway with
  live market data. Read-only tests and capture campaigns use this role.
- `IBKR_LIVE_PAPER_ADDR` points at the throwaway paper Gateway. Tests that
  place, modify, cancel, or flatten orders require both `IBKR_LIVE_TRADING=1`
  and this paper role.

Run the setup diagnostic before a live campaign:

```bash
go run ./cmd/ibkr-doctor -role readonly-live
go run ./cmd/ibkr-doctor -role paper-dev
```

## Live safety rules

- Only `paper-dev` may place, replace, exercise, or cancel orders.
  `readonly-live` is real money and API read-only.
- Capture campaigns (`scripts/record-scenarios.sh`, see
  [`docs/transcripts.md`](docs/transcripts.md)) need
  `IBKR_PAPER_ACCOUNT=<the exact DU account>` and
  `IBKR_CAPTURE_GLOBAL_CANCEL=1`. The driver refuses to trade otherwise, and
  reconciles open orders, positions, executions, and account values to its
  baseline afterwards.
- The one authorized regulatory snapshot (fee-bearing) has been consumed.
  Never repeat it from a batch, test, or manual command.
- Do not issue another live option-exercise instruction; the retained replay
  stops at accepted-but-unsettled admission on purpose.
- Manual `orderBound` evidence needs paper TWS and a user-entered order. Do
  not automate TWS or Gateway through GUI tooling, and do not change their
  configuration.

## Release checklist

Run on the exact tree to be tagged:

```text
go mod tidy -diff
go mod verify
gofmt -l .
go build ./...
go vet ./...
CGO_ENABLED=0 GOOS=linux GOARCH=386 go build ./...
golangci-lint run
go test -shuffle=on -count=1 ./...
go test -race -shuffle=on -count=1 ./...
./scripts/check-api.sh
./scripts/fuzz-all.sh --check
go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
```

Then freeze the public surface: write the new `testdata/api/<version>.api`
manifest, point `scripts/check-api.sh` at it, and confirm
`./scripts/check-api.sh --exact` passes. Old manifests are never edited. Run
the examples against the paper Gateway, write the CHANGELOG entry (link
release-time documents at the tag, not at `main`), and tag. Pushing and
publishing stay manual.

## Reference policy

Protocol-adjacent work should be grounded in the local live TWS or IB Gateway when available, plus official IBKR docs, official IBKR client-library source, captured traffic, and other IBKR library implementations where useful. The official IBKR C++ SDK may be used as an opt-in conformance oracle or capture tool, but production code remains pure Go on the default import path. The merged implementation must still follow `ibkr-go`'s typed public API and package philosophy rather than mirroring the official public surface mechanically.

## Commit convention

- Subject line: ≤72 characters, imperative mood, concrete. Lowercase after any prefix. No trailing period.
- Body (optional, only when the change earns one): focuses on **why**. What constraint, incident, or design pressure forced the change. The diff shows *what*; the body carries the context the diff cannot.
- One logical change per commit.
- Protocol-adjacent commits should mention the live environment, captures, or source/docs references that justified the change when that context matters.
- No WIP commits and no "fix typo" follow-ups in main history; squash before landing.
- No emoji, no marketing voice.

## Version control

Jujutsu (`jj`) is the default workflow for this repository; Git remains the
GitHub/interoperability layer and `origin` is canonical. Prefer `jj st`,
`jj diff --git`, `jj log`, `jj commit`, `jj split`, `jj squash`,
`jj git fetch`, `jj rebase`, and `jj git push --dry-run` / `jj git push`.
Raw Git is appropriate for read-only checks or explicit interoperability and
repair. Jujutsu does not run Git hooks automatically, so run the normal checks
before publishing.

## Pull requests

Keep pull requests tight. One logical change. Tests land alongside the code. The description follows the pull request template — what, why, tests, and protocol references/verification where relevant.

## Reporting bugs and requesting features

Use the issue templates at `.github/ISSUE_TEMPLATE/`. For security-sensitive reports, follow [`SECURITY.md`](SECURITY.md) — do not open public issues for vulnerabilities.

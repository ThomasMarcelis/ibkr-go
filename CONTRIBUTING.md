# Contributing to ibkr-go

Contributions are welcome. Read this document first.

## Scope and direction

`ibkr-go` is a Go client for the Interactive Brokers TWS and IB Gateway
socket protocol. It targets the full free read-only surface plus order
management, market depth (Level 2), and option exercise. The official source
baseline is API 10.50.01. The client negotiates exactly `server_version`
208..225; API 10.50.01's `Order.conditionsIncludeOvernight` needs version 226
and is intentionally outside that range. Versions 200..207 are unsupported on
the v2 line; use a current Gateway or stay on v2.0.0. What is proven and what
is blocked is in [`docs/live-coverage-matrix.md`](docs/live-coverage-matrix.md);
what is next is in [`docs/roadmap.md`](docs/roadmap.md).

## Development loop

Prerequisites: Go 1.26+ and `golangci-lint` (latest). `gofmt` and `goimports`
in your editor help.

```
go build ./...
go vet ./...
gofmt -l .           # must produce no output
./scripts/check-api.sh
golangci-lint run
go test ./...
```

All six must pass locally before opening a pull request. The default API
check compares against the highest stable same-major tag reachable from HEAD,
reading its manifest directly from that tag. It allows additions but rejects
incompatible changes even if the working-tree manifest was regenerated.
`./scripts/check-api.sh --exact` compares against the single candidate manifest
in `testdata/api`. Fetch full history and tags before compatibility checks;
missing release evidence is an error.

CI also checks module tidiness and verification, the fuzz-target inventory, a
pure-Go 386 build, vulnerabilities, shuffled tests on Linux, macOS, and
Windows, and the race detector. See `.github/workflows/ci.yml` for the exact
commands.

## Testing discipline

The test suite is this library's primary asset and is grown deliberately.
Every test must improve confidence or diagnosis quality; tests that only prove
the code was written are rejected. Every bug fix lands with the transcript or
test that would have caught the bug, and that test becomes a permanent
regression freeze.

Tests are organised in layers: invariants, state transitions, behavioral
scenarios, and stress or edge cases. Routine CI stays deterministic, but
protocol-adjacent development is grounded in the local live TWS or IB Gateway
when one is available. `internal/testhost` and the checked-in fixtures replay
live-derived behavior; they are not a place to invent protocol semantics.
Wire framing and codec round-trips are also fuzzed.

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

Then freeze the public surface with the pinned `apidiff` version in
`scripts/check-api.sh`: `go run <apidiff-version> -w testdata/api/<version>.api
$(go list -m)`. Replace the previous manifest; older ones live at their tags.
Run `./scripts/check-api.sh --release <version>` on the candidate tree. It
requires exact agreement with that candidate manifest and compatibility with
the highest reachable stable same-major tag strictly older than the candidate.
The candidate's own tag and prerelease tags cannot become its baseline.
Incompatible changes need a new major version; never rewrite release history.

Run the examples against the paper Gateway, write the CHANGELOG entry (link
release-time documents at the tag, not at `main`), and tag. Pushing and
publishing stay manual.

## Reference policy

Ground protocol-adjacent work in the local live TWS or IB Gateway when
available, plus official IBKR docs, official IBKR client-library source,
captured traffic, and other IBKR library implementations where useful. The
official IBKR C++ SDK may be used as an opt-in conformance oracle or capture
tool, but production code stays pure Go on the default import path. The
merged implementation must follow `ibkr-go`'s typed public API and package
philosophy rather than mirroring the official surface mechanically.

## Commit convention

- Subject line: at most 72 characters, imperative mood, concrete. Lowercase
  after any prefix. No trailing period.
- Body (optional, only when the change earns one): focuses on **why**. What
  constraint, incident, or design pressure forced the change. The diff shows
  *what*; the body carries the context the diff cannot.
- One logical change per commit.
- Protocol-adjacent commits should mention the live environment, captures, or
  source and docs references that justified the change when that context
  matters.
- No WIP commits and no "fix typo" follow-ups in main history; squash before
  landing.
- No emoji, no marketing voice.

## Version control

Jujutsu (`jj`) is the default workflow for this repository; Git remains the
GitHub and interoperability layer and `origin` is canonical. Prefer `jj st`,
`jj diff --git`, `jj log`, `jj commit`, `jj split`, `jj squash`,
`jj git fetch`, `jj rebase`, and `jj git push --dry-run` / `jj git push`.
Raw Git is appropriate for read-only checks or explicit interoperability and
repair. Jujutsu does not run Git hooks automatically, so run the normal checks
before publishing.

## Pull requests

Keep pull requests tight. One logical change. Tests land alongside the code.
The description follows the pull request template: what, why, tests, and
protocol references or verification where relevant.

## Reporting bugs and requesting features

Use the issue templates at `.github/ISSUE_TEMPLATE/`. For security-sensitive
reports, follow [`SECURITY.md`](SECURITY.md) and do not open public issues for
vulnerabilities.

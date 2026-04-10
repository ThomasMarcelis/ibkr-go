# Contributing to ibkr-go

Contributions are welcome. Read this document first.

## Scope and direction

`ibkr-go` is a Go client for the Interactive Brokers TWS and IB Gateway socket protocol. The library covers the full read-only surface plus order management, market depth (Level 2), fundamental data, and option exercise. See [`docs/roadmap.md`](docs/roadmap.md) for the full charter.

## Development loop

Prerequisites: Go 1.26+, `golangci-lint` (latest), and optionally `gofmt` and `goimports` integrated into your editor.

```
go build ./...
go vet ./...
gofmt -l .           # must produce no output
golangci-lint run
go test ./...
```

All five must pass locally before opening a pull request. CI runs the same five steps on every push and pull request against `main`.

## Testing discipline

The test suite is this library's primary asset. It is grown deliberately. Every test must improve confidence or diagnosis quality — tests that only prove the code was written are rejected. Every bug fix lands with the transcript or test that would have caught the bug; that test becomes a permanent regression freeze.

Tests are organised in layers: invariants, state transitions, behavioral scenarios, stress and edge cases. Routine CI stays deterministic, but protocol-adjacent development is grounded in the local live TWS or IB Gateway when available. The `testing/testhost` package and checked-in fixtures are replay tools for live-derived behavior, not a source of truth for invented protocol semantics.

Wire framing and codec round-trips are also fuzzed. The intent is not just to
have broad coverage, but to keep the protocol surface diagnosable and safe to
extend without a live broker in CI.

## Reference policy

Protocol-adjacent work should be grounded in the local live TWS or IB Gateway when available, plus official IBKR docs, official IBKR client-library source, captured traffic, and other IBKR library implementations where useful. The merged implementation must still follow `ibkr-go`'s typed public API and package philosophy rather than mirroring the official public surface mechanically.

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

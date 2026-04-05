# Contributing to ibkr-go

Contributions are welcome. Read this document first.

## Scope and direction

`ibkr-go` is a clean-room Go client for the Interactive Brokers TWS and IB Gateway socket protocol. The v1 target is a **read-only production core**. Order writes, scanners, news, Flex, and the Client Portal Web API are explicitly out of v1 scope. See [`docs/roadmap.md`](docs/roadmap.md) for the full charter and [`docs/provenance.md`](docs/provenance.md) for the clean-room boundary.

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

Tests are organised in layers: invariants, state transitions, behavioral scenarios, stress and edge cases. All routine tests run against the in-process `testing/testhost` package with checked-in deterministic transcripts. Live Gateway runs are reserved for transcript refresh and are never part of CI.

The full testing philosophy lives in [`AGENTS.md`](AGENTS.md).

## Clean-room policy

Every pull request that touches protocol, wire, codec, session, or transport code must include the clean-room attestation checkboxes from the pull request template. Read [`docs/provenance.md`](docs/provenance.md) before contributing to protocol-adjacent code. In short: official Interactive Brokers client source is a forbidden reference, and other OSS IB libraries may be read for anti-pattern study only.

## Commit convention

- Subject line: ≤72 characters, imperative mood, concrete. Lowercase after any prefix. No trailing period.
- Body (optional, only when the change earns one): focuses on **why**. What constraint, incident, or design pressure forced the change. The diff shows *what*; the body carries the context the diff cannot.
- One logical change per commit.
- Protocol-adjacent commits include a clean-room attestation line in the body: `clean-room: no official TWS API source was referenced.`
- No WIP commits and no "fix typo" follow-ups in main history; squash before landing.
- No emoji, no marketing voice, no co-author trailers implying non-human authorship.

## Pull requests

Keep pull requests tight. One logical change. Tests land alongside the code. The description follows the pull request template — what, why, tests, clean-room attestation.

## Reporting bugs and requesting features

Use the issue templates at `.github/ISSUE_TEMPLATE/`. For security-sensitive reports, follow [`SECURITY.md`](SECURITY.md) — do not open public issues for vulnerabilities.

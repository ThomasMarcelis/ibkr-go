# Slice 11: `ibkr-go` Core

## Objective

Implement the read-only production core of the `ibkr-go` library.

## Why This Slice Exists

This slice creates the actual reusable library core that external consumers will dogfood. It must be correct and testable on its own, not merely "good enough for one app."

## Repo

- `ibkr-go`

## Current Repo Truth

- Slice `10` is complete at `502be28`. The repo layout, CI pipeline, clean-room policy, testing philosophy, and commit convention are already locked and in place.
- `AGENTS.md`, `docs/provenance.md`, `docs/anti-patterns.md`, `docs/feature-matrix.md`, and `docs/roadmap.md` are normative and must not be contradicted.
- The package skeleton at `ibkr/`, `internal/{wire,codec,transport,session}/`, `testing/testhost/`, and `experimental/raw/` consists of `doc.go` files with package declarations only. Slice 11 fills these packages with real content.

## Decisions Already Made

- Root public package `ibkr`.
- Internal `wire` / `codec` / `transport` / `session`.
- `testing/testhost` as the default test integration path.
- Explicit session and reconnect semantics as first-class contract behavior.
- No primary `EWrapper` / `EClient` surface in v1.
- Go 1.26, zero non-stdlib deps where possible.

## Deliverable

- Handshake / session core.
- Request correlation.
- Typed one-shot request methods.
- Typed subscriptions with explicit lifecycle (`Events`, `Done`, `Wait`, `Close`).
- Fake host / testhost implementation that speaks the protocol from checked-in transcripts.
- Transcript / golden test harness.
- Read-only coverage of: managed accounts, account values / summary, positions / portfolio, contract details / qualification, market data (quote snapshots and streaming), historical bars, real-time bars, observation-only order/execution state.

## Public API / Schema Changes

- Public root package `ibkr`.
- Typed one-shot methods (e.g., `DialContext`, `ManagedAccounts`, `AccountSummary`, `PositionsSnapshot`, `HistoricalBars`).
- Typed subscription lifecycle (e.g., `SubscribePositions`, `SubscribeAccount`, `SubscribeQuotes`).
- No primary `EWrapper` / `EClient` public surface.

## Non-Goals

- Order writes.
- Client Portal Web API.
- Flex.
- First-class `compat/eclient` official bridge package.
- Scanners and news breadth.
- Near-full parity with the entire TWS API surface.

## Dependencies

- `10` (complete)

## Write Scope

- entire `ibkr-go` implementation repo

## Allowed Parallelism

- None within ibkr-go (only active slice).

## Required Tests

Landing alongside the protocol code per the five-layer pyramid in `AGENTS.md` §Testing Philosophy:

- **Invariants**: protocol framing, codec round-trip, session state machine transitions. Small, exhaustive.
- **State transitions**: table-driven single-message encode / decode cases.
- **Behavioral scenarios**: end-to-end typed one-shot flows against `testing/testhost/` with deterministic checked-in transcripts. Include `DialContext` → `ManagedAccounts` → `AccountSummary`, subscription lifecycle, historical bars.
- **Stress / edge**: malformed frames, partial reads, reconnect mid-subscription, negotiated-version edge cases.
- **Deterministic first**: all tests use `testhost` + transcripts. No live Gateway in CI.

Required behavior coverage:

- handshake readiness
- negotiated server version
- completion / snapshot boundary semantics
- reconnect / reset / lost-subscription semantics
- pacing and qualification-sensitive request handling where implemented
- deterministic testhost transcript format and replay

## Definition Of Done

- Read-only core is stable enough for external dogfooding.
- CI (`gofmt -l .`, `go vet`, `go build`, `golangci-lint run`, `go test ./...`) is green with real tests now populating `go test`.
- Tracker updated in this repo's `IMPLEMENTATION_STATUS.md` with slice 11's commit hash and `complete` status.
- Commit follows the convention in `AGENTS.md` §Commit Convention, including the clean-room attestation line in the body.

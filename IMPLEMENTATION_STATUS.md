# Implementation Status

## Invariants

- This file is the single source of truth for implementation progress in this
  repository.
- Clean-room boundary is load-bearing. Every protocol-adjacent commit carries a
  clean-room attestation line in its body.
- The repo remains independent. No status is synced from or to any other repo.
- The active implementation truth is now contract-first and phase-based.

## Current State

- Baseline charter work is complete at `502be28`.
- The public source of truth lives in:
  - `docs/architecture.md`
  - `docs/session-contract.md`
  - `docs/message-coverage.md`
  - `docs/transcripts.md`
- The repo has a real contract/harness implementation, but it is not yet a
  live IBKR-compatible protocol implementation.

## Milestone Table

| Milestone | Status | Notes |
|---|---|---|
| contract freeze | landed | live docs and public contracts are in repo |
| wire | landed | generic frame and field invariants are implemented |
| contract harness | landed | typed public API, session engine, fake host, and deterministic scenarios are in repo |
| real IBKR wire mapping | not started | replace symbolic codec/messages with real protocol compatibility |
| live transcript capture | not started | capture and normalize contributor-owned Gateway / TWS traces |
| reconnect and pacing hardening | not started | grow system-code, reconnect, and rate-limit coverage against the real protocol |
| compatibility floor | not started | establish minimum supported server version and tested host range |
| recorder + hardening | not started | capture, replay, race, fuzz, benchmark |

## Conditional Follow-Up

- The old `11b` reconciliation concept still applies as a downstream-gap
  follow-up, but any such work should be tracked as a concrete issue or
  milestone against the frozen contracts rather than as an architectural reset.

## Next Step

- land the real IBKR handshake and message mapping behind the already-frozen
  public contracts

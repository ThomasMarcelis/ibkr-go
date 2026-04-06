# Implementation Status

## Invariants

- This file is the single source of truth for implementation progress in this
  repository.
- Live protocol truth and deterministic replay are load-bearing for protocol
  work in this repository.
- The repo remains independent. No status is synced from or to any other repo.
- The active implementation truth is now contract-first and phase-based.

## Current State

The implementation is a live IBKR-compatible protocol client, validated
against server_version 200 captures and a live IB Gateway. The codec uses
real IBKR integer message IDs and field layouts. Replay fixtures are derived
from live captures rather than invented protocol semantics.

Public source of truth:
  - `docs/architecture.md`
  - `docs/session-contract.md`
  - `docs/message-coverage.md`
  - `docs/transcripts.md`

## Milestone Table

| Milestone | Status | Notes |
|---|---|---|
| contract freeze | landed | live docs and public contracts are in repo |
| wire | landed | generic frame and field invariants are implemented |
| contract harness | landed | typed public API, session engine, and deterministic replay tooling are in repo |
| real IBKR wire mapping | landed | v200 wire format validated against live captures |
| live transcript capture tooling | landed | 25 capture scenarios checked in, grounded fixtures derived |
| reconnect and pacing hardening | landed | system codes routed, per-subscription resume policies tested |
| compatibility floor | landed | server_version 200 tested; v100..200 negotiation range |
| replay, race, fuzz, benchmark hardening | in progress | race testing complete; fuzz and benchmark are post-v1 |

## Next Step

Post-v1 work: order writes, deeper contract detail fields, fuzz testing,
broader server version testing.

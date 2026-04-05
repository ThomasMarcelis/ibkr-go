# Implementation Status

## Invariants

- This is the `ibkr-go` project's own tracker. It is the single source of truth for slice progress in this repository. It does not depend on, reference, or sync with any other project's tracker.
- Clean-room boundary is load-bearing. Every protocol-adjacent commit carries a clean-room attestation line in its body.
- License: MIT. Visibility: currently private; will flip to public manually once v1 is in place.
- Testing philosophy: high signal, low maintenance, grown deliberately. See `AGENTS.md`.
- Commit convention: see `AGENTS.md` and `CONTRIBUTING.md`.

## Ready / Blocked Summary

- Completed:
  - `10` repo and charter — initial commit `502be28`
- Ready now:
  - `11` `ibkr-go` core
- Conditional (not yet ready):
  - `11b` reconciliation — only created if dogfooding from an external consumer (the personalapps stonks slice `12`) reveals a missing primitive or contract gap. Not triggered as part of normal forward progress.

## Current Next-Ready Slice Set

- `11` `ibkr-go` core

## Allowed Parallel Pairings Right Now

- none (`11` is the only active slice; no parallelism within ibkr-go)

## Slice Table

| Slice | Depends On | Status | Write Scope | Last Commit | Notes |
|---|---|---|---|---|---|
| `10` | none | complete | entire `ibkr-go` repo (initial population) | `502be28` | initial charter, clean-room policy, module skeleton, CI — see `docs/slices/10-repo-and-charter.md` |
| `11` | `10` | ready | entire `ibkr-go` implementation tree (`ibkr/`, `internal/{wire,codec,transport,session}/`, `testing/testhost/`, `testdata/transcripts/`) | - | read-only production core — see `docs/slices/11-core.md` |
| `11b` | `11` + external dogfood feedback | blocked (conditional) | `ibkr-go` repo only | - | only if dogfooding exposes a missing primitive — see `docs/slices/11b-reconciliation.md` |

## Prompt Mapping

| Slice | Prompt File |
|---|---|
| `10` | `docs/slices/10-repo-and-charter.md` |
| `11` | `docs/slices/11-core.md` |
| `11b` | `docs/slices/11b-reconciliation.md` |

## Current Blockers

- none

## Notes

- This project is developed independently. The `personalapps` platform consumes `ibkr-go` as an external published Go library via `go get github.com/ThomasMarcelis/ibkr-go@<version>`. If a downstream consumer discovers a missing primitive, they file it against this project's tracker and slice `11b` is created.
- Every slice must update this file before completion.
- Do not mark a slice complete without tests, review, docs, and a commit.
- Every protocol-adjacent commit carries the clean-room attestation line in its body: `clean-room: no official TWS API source was referenced.`

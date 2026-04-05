# Slice Prompts

This directory contains the implementation prompts for `ibkr-go`'s slice-based delivery. Each prompt is intended to be decision-complete for one slice, and each slice lands in a single commit (with the narrow exception of the initial commit cluster for slice 10).

## Ordered Slices

| Slice | Name | Status | Prompt |
|---|---|---|---|
| `10` | Repo and Charter | complete | [`10-repo-and-charter.md`](10-repo-and-charter.md) |
| `11` | Core (read-only production) | ready | [`11-core.md`](11-core.md) |
| `11b` | Reconciliation (conditional) | blocked | [`11b-reconciliation.md`](11b-reconciliation.md) |

## Current Start State

Ready now:

- `11` `ibkr-go` core

Slice `11b` is conditional — it is only created if external dogfooding (the `personalapps` `stonks` module) reveals a missing library primitive during its integration work. Until such a gap is filed, it remains blocked and the project is considered complete after slice `11`.

## Rule

If the orchestrator finds a prompt incomplete or stale, it must fix the prompt before executing the slice. All changes to slice scope are recorded in this directory; no silent scope creep.

## Live tracker

Slice status, commit hashes, and blockers live in [`../../IMPLEMENTATION_STATUS.md`](../../IMPLEMENTATION_STATUS.md). That file is the single source of truth; this index is a convenience.

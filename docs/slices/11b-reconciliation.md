# Slice 11b: `ibkr-go` Reconciliation

## Objective

Reconcile `ibkr-go` after real dogfooding from an external consumer exposes a missing primitive or contract gap.

## Why This Slice Exists

Dogfooding almost always finds something the initial library slice missed. This slice exists so those findings become explicit tracked work instead of silent scope creep inside slice `11`.

## Repo

- `ibkr-go`

## Current Repo Truth

- Slice `11` delivered the initial read-only core.
- An external consumer (e.g., the `personalapps` `stonks` broker adapter) has reported a specific missing library primitive, contract gap, or testhost deficiency.
- This slice is **conditional**: it is only created when such a gap is filed. It does not run speculatively.

## Decisions Already Made

- Do not silently mutate slice `11` in place to fix the gap — always create a separate slice so the change is explicit and reviewable.
- Keep the public API radically new and read-only-first unless an explicit higher-level decision changes that.

## Deliverable

- The smallest coherent library change needed to unblock the reported dogfood gap.
- Updated charter and docs if the reconciliation changes contract expectations (e.g., a new subscription event shape, a new request type).
- Updated tests and goldens.

## Public API / Schema Changes

- Only if required by the discovered dogfood gap.
- Any public API change must be documented in the relevant slice prompt and in `README.md`.

## Non-Goals

- Broad scope expansion beyond what the specific gap demands.
- Order writes.
- Client Portal Web API support.
- First-class `compat/eclient` bridge.

## Dependencies

- `11` (complete)
- External dogfood gap report from a downstream consumer. Typically filed as an issue or via direct communication from the consumer project; no cross-repo orchestration dependency.

## Write Scope

- `ibkr-go` repo only

## Allowed Parallelism

- none while active

## Required Tests

- Reproduce the dogfood gap in tests (new invariant, state transition, scenario, or stress case as appropriate to the gap).
- Prove the reconciliation fixes it.
- Keep existing testhost and transcript coverage green.

## Definition Of Done

- The dogfood-discovered gap is closed and demonstrably covered by a new regression test.
- Tracker updated in this repo's `IMPLEMENTATION_STATUS.md` with slice 11b's commit hash and `complete` status.
- Commit follows the convention in `AGENTS.md` §Commit Convention, including the clean-room attestation line in the body.

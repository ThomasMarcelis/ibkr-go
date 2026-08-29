# Live Test Execution Tracker

Companion to [`live-coverage-matrix.md`](live-coverage-matrix.md). New campaign
work follows the [`v2.0.2 coverage plan`](v2.0.2-coverage-plan.md). The v2.0.1
section below is the immutable starting baseline; older server-version runs
remain in repository history, not in the active support ledger.

Last updated: 2026-08-28.

## v2.0.2 current disposition

- The executable catalog has 124 scenarios: 103 promoted, no candidates, and
  21 explicitly blocked by entitlements, account type, market state, or
  TWS-only interaction.
- The tracked corpus has 113 live-derived sv208+ transcripts. Every transcript
  passes full-hash provenance validation.
- The decoder ledger partitions all 106 registered layout pairs: 86 have
  positive raw-frame attestation and 20 retain explicit external blockers.
- The exact sv208–225 negotiation matrix and native/SDK boundary vectors cover
  the supported protocol train. API 10.50.01's only supported-range addition,
  contract settlement method field 65, is live-replayed at sv225.
- Never repeat the consumed regulatory snapshot, and never automate manual TWS
  evidence through GUI/CUA tooling.

## v2.0.1 release baseline

- Production advertises and accepts exactly `server_version` 208–225.
- The checked-in corpus has 99 raw-frame transcripts: 95 at sv225, exact
  historical and user-info fixtures at sv208, one positive option-calculation
  fixture at sv211, and one exact sv208–225 handshake/`CurrentTime` matrix.
  Every transcript passes full-hash provenance validation.
- The retained raw corpus has 328 verifier-clean capture directories, all at
  sv208 or newer. No pre208 raw or tracked replay corpus remains.
- The executable catalog has 125 scenarios: 91 promoted, 26 candidates, and
  8 explicitly blocked.
- The exact-version matrix proves negotiation and `CurrentTime` at every
  version. It does not yet prove every request-family transition: 20 supported
  decoder/layout attestations remain pending in
  `internal/codec/codec_capture_coverage_test.go`.
- Positive market depth and a terminal option-exercise lifecycle remain
  blocked by current entitlement and broker-settlement constraints. Manual
  `orderBound` remains open because no manual TWS order was submitted while the
  API harness was armed.
- The complete deterministic, shuffled, race, lint, exact-API, timed-fuzz,
  vulnerability, raw-capture, and transcript-provenance gate passes on the
  release tree. The remaining gaps below are accepted known limitations and
  v2.0.2 coverage targets; they are not presented as positive proof.

## Current live roles

| Role | Address | Authority | Current negotiation |
|------|---------|-----------|---------------------|
| `readonly-live` | `127.0.0.1:4001` | real-money, API read-only | sv225 |
| `paper-dev` Gateway | `127.0.0.1:4002` | dedicated guarded trading | sv225 |

Run both role-aware doctors before a live sweep:

```bash
go run ./cmd/ibkr-doctor -role readonly-live
go run ./cmd/ibkr-doctor -role paper-dev
```

On 2026-08-28 both Gateway doctors passed at sv225. `readonly-live` returned
the expected code-10089 market-data entitlement warning; `paper-dev` returned
a delayed AAPL quote. The previously enumerated safe live suite also passed
against `readonly-live` and paper TWS. The regulatory,
manual-order, restart/outage, capture-campaign, and order-mutation tests were
excluded. `SubscribePositions` reached `SnapshotComplete` on both roles, and
the exact sv208–225 handshake matrix passed on both. Skips were limited to
exact typed entitlement, account, market-state, or recorder-only blockers.

The catalog selects the role from each scenario's risk class. Every paper-risk
scenario enters the centralized campaign wrapper, which requires the exact
`DU` account allowlist and emergency global-cancel gate before admission,
captures open-order, position, execution, and account-value baselines, and
reconciles working orders, position deltas, executions, and fees before the
client closes.

## Current positive and blocker evidence

| Behavior | Disposition | Evidence |
|----------|-------------|----------|
| Positions subscription | promoted | sv225 subscription reaches `SnapshotComplete`; public replay consumes each `StreamEvent` once |
| Historical bars | promoted | positive exact-sv208 capture and public replay; current sv225 entitlement failures are exact typed blockers |
| Option calculations | promoted | positive exact-sv210 classic and sv211 protobuf price and implied-volatility requests/results plus current partial sv225 evidence |
| Scanner subscription | promoted | current sv225 HOT_BY_VOLUME rows and clean cancellation |
| TickNews | promoted | current sv225 public quote-stream replay plus exact classic sv208 callback evidence |
| CFD reroute | promoted | initial CFD request, protobuf reroute, conID request, delayed-data notice, and positive high/low/volume/close callbacks (`ca8fbdf11d260066fb7cd1c3d60e6e44808a54bf6a8fc678f3597bd71a666f1c`) |
| PnL single | promoted | held-position selection, nonzero typed update, cancellation, and fence from `5c2be5fc5c73842b430c5644e69d5020ac45c41acd14d2a4349183b8355b0ab4` |
| Multi-asset and generic quotes | promoted | concurrent AAPL/EUR.USD streams plus 233/236 generic ticks, typed parameters, cancellation, and fences |
| Order campaigns | promoted | bracket, scale-in, per-leg-priced BAG, algorithmic, and protective-stop captures replay broker echoes, fills or zero-fill cancellation, fees, fences, and baseline reconciliation |
| Odd-lot fields | blocked positive proof | request accepted; current role returns code 10089 and no positive odd-lot values |
| Market depth | blocked positive proof | current account lacks an entitled depth venue |
| Option exercise admission | promoted | sv225 capture `37bfe1e3c3494f54e2f953936996086ecd31f9d7f2f0d6cb8ef2dd2a2323d4e2` freezes the option seed fill, exact warning 10349, `PreSubmitted`, and captured EOF as `ExerciseUncertainError`. It proves accepted-but-unsettled admission only; no terminal exercise, lapse, or settlement is claimed. |
| Manual `orderBound` | blocked | positive evidence requires paper TWS and a user-created manual order while the harness is armed; the available endpoints are Gateways |

The coverage matrix contains the exhaustive 124-scenario catalog mapping and
the protocol audit contains exact boundary hashes.

## IncludeOvernight disposition

Capture `20260825T181306Z-api_include_overnight_lifecycle_aapl`, events
SHA-256
`f3585ff96e9a0a936a559e52e9d69e860e6182e769418d987314ce9419c90e9c`,
proves the behavior supported by the current Gateway:

1. A nonmarketable one-share SMART AAPL DAY order with explicit true was
   accepted and echoed true.
2. Replacing that order with explicit false returned code 462 and retained
   true. Official SDK 10.48.01 reproduced the same broker rejection.
3. A fresh explicit-false order was accepted and broker-canonicalized to an
   absent field with `TIF=DAY`.
4. Both orders reached terminal cancellation, followed by a protocol fence and
   paper-state reconciliation.

The public `*bool` remains necessary because explicit true and explicit false
encode differently. The promoted replay records the broker's rejected
replacement and canonical fresh-order behavior; no further retry or
speculative codec change is warranted.

## Regulatory snapshot no-retry record

The sole newly authorized v2.0.1 regulatory request was capture
`20260824T195855Z-regulatory_snapshot_aapl_v201_authorized_once`, events
SHA-256
`bca23abbf9e562746b79b758378fcd6752130c0b489a53fd97db3fac3ba3a2e2`.
It returned code 0 (`Internal server error`). The attempt is permanently
consumed and must never be repeated by a batch, test, or manual command.

Post-attempt account-update capture `20260824T202345Z-account_updates`, events
SHA-256
`d7063b2455654c8aed9ecd6c9f395addf9f95a2bf70a08eff7af99bc28707f6c`,
reports `Billable=0.00 EUR` and is replay-promoted. This is the recorded fee
reconciliation for the attempt.

## Remaining external evidence

- The 20 pending decoder/layout rows require callbacks unavailable from the
  current entitlements, account types, market state, or Gateway-only setup.
  Exact blocker replies stay separate from positive decoder attestation.
- Preserve the verified paper baseline: no campaign-owned working order or
  position delta remains, and campaign executions have correlated fee reports.
- Keep the regulatory-snapshot path permanently disabled; the sole authorized
  attempt is consumed and must never be repeated.
- Do not issue another live option-exercise instruction. The retained replay
  deliberately stops at accepted-but-unsettled admission.

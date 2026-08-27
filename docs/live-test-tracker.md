# Live Test Execution Tracker

Companion to [`live-coverage-matrix.md`](live-coverage-matrix.md). New campaign
work follows the [`v2.0.2 coverage plan`](v2.0.2-coverage-plan.md). The v2.0.1
section below is the immutable starting baseline; older server-version runs
remain in repository history, not in the active support ledger.

Last updated: 2026-08-27.

## v2.0.2 planning baseline

- Start from 125 catalog scenarios: 91 promoted, 26 candidates, and 8 blocked.
- Preserve 99 tracked sv208+ transcripts and 328 verifier-clean sv208+ raw
  captures until a vertical promotion deliberately replaces an artifact.
- Close or explicitly disposition all 20 pending decoder/layout attestations.
- Expand the exact sv208–225 matrix from handshake/`CurrentTime` reachability
  to the request family or field introduced at each boundary.
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
| `paper-dev` TWS | `127.0.0.1:7497` | dedicated guarded trading and TWS-only evidence | sv225 |

Run both role-aware doctors before a live sweep:

```bash
go run ./cmd/ibkr-doctor -role readonly-live
go run ./cmd/ibkr-doctor -role paper-dev -addr 127.0.0.1:7497
```

On 2026-08-27 both doctors passed at sv225. The explicitly enumerated safe
live suite also passed against `readonly-live` and paper TWS. The regulatory,
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
| Odd-lot fields | blocked positive proof | request accepted; current role returns code 10089 and no positive odd-lot values |
| Market depth | blocked positive proof | current account lacks an entitled depth venue |
| Option exercise | partial positive proof | sv225 capture `37bfe1e3c3494f54e2f953936996086ecd31f9d7f2f0d6cb8ef2dd2a2323d4e2` bought one ITM AAPL call, received exact warning 10349 and `PreSubmitted`, then timed out without terminal evidence. Targeted/global cleanup was fenced; final direct cancel capture `f5ad48b54b8fc0867aeaa10931107b9850fb750ef1488fff245de11f43dd077c` returned exact code 10147 because order 8 was not found. Fresh open-order, position, execution, and fee snapshots match the reconciled baseline, but no terminal exercise callback was observed. |
| Manual `orderBound` | blocked | paper TWS is online at `7497`, but positive evidence requires the user to create and cancel a manual TWS order while the API harness is armed; no such order was submitted |

The coverage matrix contains the exhaustive 125-scenario catalog mapping and
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
encode differently. The broker's fresh-false canonical absence is useful
blocker evidence, but it does not prove that false survives a replacement and
echoes distinctly. The scenario remains a v2.0.2 evidence target; the SDK
result gives no basis for a speculative codec change in v2.0.1.

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

## v2.0.2 follow-up queue

- Disposition all 20 pending supported decoder/layout attestations with live
  positive evidence or exact typed account/entitlement blockers.
- Preserve the verified paper baseline: no campaign-owned working order or
  position delta remains, and both new executions have correlated fee reports.
- Keep the regulatory-snapshot path permanently disabled; the sole authorized
  attempt is consumed and must never be repeated.
- Add positive public replays only from verified current captures; exact
  account, entitlement, market-state, and TWS-only blockers remain distinct
  from successful callback evidence.

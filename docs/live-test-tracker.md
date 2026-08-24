# Live Test Execution Tracker

Companion to [`live-coverage-matrix.md`](live-coverage-matrix.md). This file
tracks only the current v2.0.1 campaign. Older server-version runs remain in
repository history, not in the active support ledger.

Last updated: 2026-08-25.

## Candidate status

- Production advertises and accepts exactly `server_version` 208–225.
- The checked-in corpus has 99 raw-frame transcripts: 95 at sv225, exact
  historical and user-info fixtures at sv208, one positive option-calculation
  fixture at sv211, and one exact sv208–225 handshake/`CurrentTime` matrix.
  Every transcript passes full-hash provenance validation.
- The retained raw corpus has 286 verifier-clean capture directories, all at
  sv208 or newer. No pre208 raw or tracked replay corpus remains.
- The executable catalog has 125 scenarios: 92 promoted, 24 candidates, and
  9 explicitly blocked.
- The exact-version matrix proves negotiation and `CurrentTime` at every
  version. It does not yet prove every request-family transition: 20 supported
  decoder/layout attestations remain pending in
  `internal/codec/codec_capture_coverage_test.go`.
- Positive market depth, option exercise, and manual paper-TWS `orderBound`
  remain blocked by the current entitlement, account, and Gateway-only
  application constraints.
- The exact candidate command gate and seven-day unchanged soak have not yet
  completed. v2.0.1 is not release-ready until those gates close.

## Current live roles

| Role | Address | Authority | Current negotiation |
|------|---------|-----------|---------------------|
| `readonly-live` | `127.0.0.1:4001` | real-money, API read-only | sv225 |
| `paper-dev` | `127.0.0.1:4002` | dedicated guarded trading | sv225 |

Run both role-aware doctors before a live sweep:

```bash
go run ./cmd/ibkr-doctor -role readonly-live
go run ./cmd/ibkr-doctor -role paper-dev
```

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
| Option exercise | blocked positive proof | sv225 capture `a10ff5818916cad50192579a39ce046143a1123a5a26f51bf359f161a0b5ad2c` qualified an ITM AAPL call, then exact warning 399 deferred the zero-fill seed order to the next options session; fresh-generation reconciliation proved no account mutation |
| Manual `orderBound` | blocked | requires paper TWS; both available applications are Gateways |

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
encode differently. The broker's fresh-false canonical absence is the
authoritative false disposition; the rejected replacement is not presented as
a successful false echo.

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

## Remaining acceptance gates

- Disposition all 20 pending supported decoder/layout attestations with live
  positive evidence or exact typed account/entitlement blockers.
- Run the two doctors and the safe role-specific live suites on the frozen
  candidate. Never enable or rerun the regulatory-snapshot path.
- Confirm the paper account has no campaign-owned working order or position
  delta and that every new execution has a correlated fee report.
- Pass the complete deterministic, race, lint, fuzz, vulnerability, API,
  capture, and transcript-provenance command gate on one unchanged tree.
- Commit that exact candidate, record its commit/tree hash in ignored local
  soak evidence, and complete 168 consecutive unchanged hours.

No tag, push, release publication, or remote mutation is authorized by this
tracker.

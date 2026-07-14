# Roadmap

## Direction

`ibkr-go` is a pure-Go, idiomatic client for the Interactive Brokers
TWS/Gateway socket protocol. The runtime stays pure Go: no cgo, no C++
toolchain, no SDK dependency on the production import path. See
[`AGENTS.md`](../AGENTS.md) for the full policy and
[`docs/architecture.md`](architecture.md) for the runtime model.
The RC.3-to-stable evidence boundary is tracked in
[`v2-release-readiness.md`](v2-release-readiness.md).

The goal is end-to-end coverage of the IBKR TWS/Gateway socket surface that
the official C++ SDK exposes for free entitlements — every request, every
callback, every version-gated edge — implemented as a typed Go facade with
deterministic replay coverage and live verification against the local
Gateway.

To get there, the official IBKR C++ SDK is used as a conformance oracle and
capture tool, never as a runtime engine:

- when adding or hardening a protocol area, run the SDK against the local
  Gateway to capture the reference request/callback shape and edge-case
  behavior;
- cross-check `ibkr-go`'s decode and request encoding against the SDK's
  interpretation of the same scenario when a discrepancy is suspected;
- promote sanitized, live-derived captures into the deterministic replay
  fixtures.

Live Gateway behavior remains the source of truth. When SDK behavior and live
behavior disagree, live wins.

## Near-Term Verification Plan

The maintainer lab uses two local Gateway roles:

- `readonly-live` is the real-money Gateway with live market data and read-only
  API permissions. It is used for market data, account, historical, scanner,
  news, WSH, and entitlement evidence.
- `paper-dev` is a throwaway paper Gateway. It is used for all order-placement,
  modification, cancellation, flattening, reconnect-with-active-order, and
  campaign evidence.

Before the next release-quality sweep, run `cmd/ibkr-doctor` against both roles,
refresh the executable capture catalog with `cmd/ibkr-capture -list-json`, and
record market-open captures through `scripts/record-scenarios.sh`. The script
derives each scenario's role from the catalog risk class, keeping the capture
target in one place. Every promoted behavior must still follow the
live-evidence path: live run, capture verification, sanitized transcript,
public replay test, and updates to the coverage matrix and tracker.

Current IBKR baseline and drift to check first:

- As of 2026-07-09, the public
  [IBKR API Software](https://interactivebrokers.github.io/) page lists API
  Latest 10.48, released 2026-07-07, and Stable 10.45, released 2026-03-30,
  and recommends TWS or IB Gateway 1045+ for comprehensive feature support.
  `server_version 200` remains the classic-wire evidence baseline. Exact
  version negotiation, protobuf migrations, and gated semantics are now
  implemented through 225; see
  [`protocol-audit-sv208-225.md`](protocol-audit-sv208-225.md).
- API 10.47 removed `reqFundamentalData`, its cancellation and callback, and
  fundamental-ratios tick type 47. The corresponding ibkr-go surface and
  classic message IDs have been retired. A final 2026-07-09 probe sent all
  seven legacy report requests through both local roles; every request
  returned code 10358. The readonly-live capture has event hash prefix
  `89db59e9e5abf7b7`, and paper-dev has `c326f314cbc4f1de`. Those captures and
  earlier sv200 captures remain historical evidence only. WSH is a separate
  API, not a replacement.
- API 10.48 changes `reqOpenOrders` results to include de-activated orders.
  The result-set behavior still needs a live deactivated-order capture; the
  wire family itself is covered.
- `$LEDGER-` account-value prefixes from the new account-value setting.
- Positive odd-lot ticks 105-110 behind generic tick 787. The request and
  ordinary delayed stream are live-proven at exact sv225, but the current
  account returned no odd-lot values.
- New or shifted order/account tail fields that affect OpenOrder,
  CompletedOrder, Execution, or Commission decoding.

## Maintainer Execution Roadmap

The north star is complete, idiomatic, pure-Go coverage of the IBKR
TWS/Gateway socket API. The work gets there through small reviewable slices,
not broad rewrites. A valuable slice usually moves at least one row in
[`live-coverage-matrix.md`](live-coverage-matrix.md) from target/candidate to
promoted, or records a real entitlement/account blocker that prevents it.

Maintainers should choose work in this order:

1. **Preserve safety and truth.** Read `AGENTS.md`, check `jj st`, and verify
   whether the task is read-only or paper-trading. Never place orders outside
   `paper-dev`.
2. **Prefer replay promotion over new surface area.** If a verified live
   capture already exists, promote it into a sanitized transcript plus public
   API test before inventing another scenario.
3. **Use the matrix to pick the next slice.** `live-coverage-matrix.md` owns
   capability status; `ibkr-api-inventory.md` owns official surface inventory;
   `message-coverage.md` owns codec message status; `live-test-tracker.md`
   owns run evidence.
4. **Keep slices vertical.** A complete slice includes the typed public API
   behavior, codec/wire shape if touched, deterministic replay or focused unit
   coverage, documentation status updates, and verification commands.
5. **Treat errors as evidence.** Entitlement, account-type, permission,
   market-state, pacing, and unsupported-feature responses are useful live
   facts. Freeze the real error when it improves diagnosis.
6. **Use the SDK only as an oracle.** SDK output can help compare request and
   callback shape for the same live scenario. It never becomes production code,
   generated repo artifacts, or a substitute for live Gateway behavior.

The next high-value workstreams are:

| Priority | Workstream | Why it matters | Next slices |
|----------|------------|----------------|-------------|
| 1 | Promote captured order campaigns | Raises deterministic CI confidence without more market dependency | what-if margin preview now has a usable API (`Orders().Preview`/`OrderState`); scale-in campaign promoted from `63db2db7cba21b68`; forex lifecycle promoted from `641eab5c0e6909f7`, OCA replay promoted from `2dc16869778bc497`, bracket replay promoted from `682a1390b2acf04c` |
| 2 | Market-open trading-basic captures | Grounds fill, modify-to-fill, order-type matrix, and rejection paths under regular-session behavior | `trading-basic` batch through `paper-dev`, then verify and triage |
| 3 | Entitlement and account blocker ledger | Keeps missing subscriptions from looking like library regressions | 10089 market data, 10187 historical ticks, 10276 WSH, option-data permissions |
| 4 | Protocol drift and version edges | Keeps the pure-Go codec current with IBKR releases | current-time millis, `$LEDGER-` account values, fractional tick sizes, order/completed-order tail fields |
| 5 | Multi-asset expansion | Proves the facade across real product classes | OPT/BAG/FOP with permissions, bond order/data permissions, CFD/CRYPTO/FUND probes |
| 6 | Public ergonomics and examples | Helps users trust and adopt the library | examples for live roles, error handling, replay-backed behavior notes, pkg.go.dev polish |
| 7 | Advanced-order live attestation | Placement and broker-echo shapes are implemented; nondefault live callbacks must now prove the rarer semantics without invented fixtures | scale extensions, PEG BENCH, auction, short-sale locate, adjusted families |

Each workstream should stay scoped to one logical change. If the work needs a
large plan, put the plan in a repo doc and keep the next handoff request
focused on the next slice and its completion criteria.

## Current state

ibkr-go covers the current public facade across the major Interactive Brokers
TWS/Gateway socket protocol domains. The core protocol areas below are
implemented across negotiated versions 200..225, but rare official callbacks,
entitlement-dependent products, and some advanced order branches still need
the live-evidence path described above.

### Bootstrap and session

Handshake, managed accounts, next valid ID, start API, current time, API
error and system status code routing, session state machine. reqMarketDataType
encoder (msg 59), reqUserInfo (msg 104).

### Account and portfolio

Account summary snapshot and streaming, positions snapshot and streaming.
reqAccountUpdates (msg 6), reqAccountUpdatesMulti (msg 76/77),
reqPositionsMulti (msg 74/75), reqPnL (msg 92/93), reqPnLSingle
(msg 94/95), reqFamilyCodes (msg 80), reqCompletedOrders (msg 99).

### Contract and reference data

Contract details, contract qualification. reqMatchingSymbols (msg 81),
reqMarketRule (msg 91), reqSecDefOptParams (msg 78), reqSmartComponents
(msg 83). `Contract` is the canonical owner of include-expired and external-ID
selectors plus BAG legs and delta-neutral composition. Nil versus explicit
zero is preserved for strike and combo-leg exempt code.

### Market data

Quote snapshot and streaming, real-time bars, historical bars, tick price,
tick size, and market data type. TickGeneric (inbound 45), TickString
(inbound 46), TickReqParams (inbound 81), reqMarketDataType encoder
(outbound 59), cancelHistoricalData encoder (outbound 25),
reqTickByTickData (msg 97/98), and historical bars keepUpToDate flag. Quote
subscriptions deliver generic, string, request-parameter, and option-
computation, EFP, delta-neutral validation, and contract-specific news
headlines as discriminated
`QuoteUpdate` values while keeping the normalized `Quote` snapshot small.
Every classic price and size callback is also preserved with its numeric tick
type, exact price-attribute mask, and optional companion size even when it has
no normalized field. Live/delayed volume and matching companion sizes are
normalized. The public path is frozen
against exact server-version-200 frames from the April 5 generic-tick, June 11
option campaign, July 9 generic-tick-matrix, and exact server-version-201 BRFG
TickNews captures. TickEFP and delta-neutral validation are typed and source-
law frozen; positive live callbacks remain entitlement/product-state gaps.

### Historical data extensions

reqHeadTimestamp (msg 87/90), reqHistogramData (msg 88/89),
reqHistoricalTicks (msg 96), and historicalSchedule delivery (msg 106) through
`History().Schedule`.

### Option calculations

reqCalcImpliedVolatility (msg 54/56), reqCalcOptionPrice (msg 55/57).

### News

reqNewsProviders (msg 85), reqNewsBulletins (msg 12/13), reqNewsArticle
(msg 84), reqHistoricalNews (msg 86). Three providers are free by default
(BRFG, BRFUPDN, DJNL).

### Scanner

reqScannerParameters (msg 24), reqScannerSubscription (msg 22/23). The public
subscription request covers the complete classic field set and both generic
option lists. A public-API server-version-200 capture freezes its exact request,
ten ranked scanner-data rows, and clean cancellation. An older capture retains
the real code 490 permission response and code 365 rejected-cancel path.

### Order and execution observation

Open orders snapshot and streaming (all three scopes), execution snapshots and
late-fee subscriptions, and commission-and-fees reports. The classic sv200
execution and fee payloads are fully projected; nondefault filters and
meaningful bond yield/redemption remain live-attestation targets.

### Order management

PlaceOrder (msg 3), CancelOrder (msg 4), reqGlobalCancel (msg 58). OrderHandle
tracks business and lifecycle evidence through one ordered Events() stream,
plus Done(), Wait(), Close(), Cancel(), and Replace(). Order IDs are
auto-allocated from NextValidID. OpenOrder and OrderStatus messages are
dual-dispatched to both per-order handles and the singleton open-orders
observer. Execution and fee callbacks stay on order handles and execution
observers. OrderHandle survives reconnectable disconnects, emits
RecoveryRequired after an observation gap, and permanently blocks replacement
on that handle while retaining stable-ID cancellation.
Terminal statuses do not end observation; the caller closes the handle after
collecting any late execution and fee callbacks it requires.

### Market depth (Level 2)

reqMktDepth (msg 10), cancelMktDepth (msg 11), inbound MarketDepth (12) /
MarketDepthL2 (13) — full order book depth as a keyed subscription. Requires a
paid L2 market data subscription.

### Retired fundamental data (historical)

IBKR API 10.47 removed the Reuters fundamental-data request, cancellation,
callback, and fundamental-ratios tick type. ibkr-go no longer exposes that
surface; classic outbound message IDs 52 and 53 and inbound message ID 51
remain unused gaps. Earlier `server_version 200` captures document behavior
before removal but are not current coverage targets. Final 2026-07-09 captures
against both local roles sent all seven legacy report requests and received
code 10358 for every request (`89db59e9e5abf7b7` readonly-live,
`c326f314cbc4f1de` paper-dev). WSH is a separate API, not a replacement.

### Exercise options

ExerciseOptions (msg 21) — option exercise request tracked by `ExerciseHandle`.

### FA configuration

RequestFA (msg 18), inbound ReceiveFA (16), reqSoftDollarTiers (msg 79), and
inbound SoftDollarTiers (77). Mutating FA configuration is outside the library
charter.

### WSH calendar

reqWSHMetaData (msg 100), cancelWSHMetaData (msg 101), reqWSHEventData (msg
102), cancelWSHEventData (msg 103), inbound WSHMetaData (104) / WSHEventData
(105). Keyed one-shots returning JSON. Requires WSH subscription.

### Display groups

queryDisplayGroups (msg 67, inbound 67), subscribeToGroupEvents (msg 68,
inbound 68), updateDisplayGroup (msg 69), unsubscribeFromGroupEvents (msg 70).
TWS window integration.

### Cross-cutting

reqMktDepthExchanges (msg 82) — exchange metadata for Level 2 availability.

## Toward full SDK-equivalent coverage

Each item below is a known gap between the current public surface and the
free-entitlement SDK surface. Closing it means: typed Go facade, deterministic
replay fixture seeded from a sanitized SDK/live capture, and live verification
against the local paper Gateway when applicable.

- Scale order extensions.
- Remaining ungrounded OpenOrder branches.
- Order-side coverage promised by the public API but not yet exhaustively
  grounded: OCA group semantics, bracket parent/child sequencing, condition
  families (price, time, margin, execution, volume, percent-change), IB algo
  parameter passthrough, hedging, and short-sale fields. Sequenced after the
  protobuf decision (see below), since these are classic-branch fields.
- ~~Server-version coverage through exactly `server_version 225`~~ **Done.**
  The client negotiates `server_version` 200..225. Exact 201..207 retain their
  individual protocol audits below. Exact 208..213 complete the protobuf
  migration table, and 214..225 implement every version gate in API 10.48,
  including one-shot cancels and order fields, read-only TWS configuration,
  share/fractional size semantics, precision metadata, and
  odd-lot quote projection. Byte-exact SDK/live vectors and the remaining
  positive-live entitlement boundaries are recorded in
  [`protocol-audit-sv208-225.md`](protocol-audit-sv208-225.md).
  Attached-preset wire support remains internal pending a native paper-order
  lifecycle.

[`docs/live-coverage-matrix.md`](live-coverage-matrix.md) and
[`docs/message-coverage.md`](message-coverage.md) are the authoritative gap
trackers; new gaps land there first, then graduate here.

## Protobuf era (sv 201+)

Exact `server_version 201` is implemented and live-attested. At this boundary
every normal message uses a raw four-byte big-endian ID; IDs above 200 select a
protobuf body and map back to base ID `wireID-200`. `reqExecutions` is the
first migrated outbound family. Its request, `execDetails`,
`execDetailsEnd`, commission-and-fees, and error protobuf schemas are owned by
the Go codec through `protowire`, with no generated SDK artifact or runtime
dependency. A sanitized public replay freezes the exact-201 empty query; a
one-share paper round trip live-attests non-empty execution details and classic
commission reports at the same negotiated version. The classic bootstrap,
request, detail, and end bytes are exact-vector tested.

Exact `server_version 202` is also implemented and live-attested. It introduces
the official `ZERO_STRIKE` capability but no new protobuf message migration.
The exact-202 public probe resolved an AAPL conId-only classic contract request
and decoded a real protobuf execution whose Contract carried conId 265598 and
an explicitly present fixed64 strike of zero. A sanitized non-empty replay
freezes that same field-presence vector. This proves the negotiated semantic
and codec boundary, not trading support for a zero-strike derivative; no such
live product was available in the local paper session.

Exact `server_version 203` is implemented and live-attested. Place order,
targeted cancel, and global cancel move to protobuf, as do the observed
open-order and order-status replies. Order errors use the protobuf family
already introduced at 201. The guarded paper scenario
rested a one-share AAPL order far below market, cancelled it, issued global
cancel, and completed an ordered round trip before disconnect. See
[`protocol-audit-sv203.md`](protocol-audit-sv203.md).

Exact `server_version 204` is implemented and live-attested. Client/all/auto
open-order requests and the completed-order request/results migrate to
protobuf. Completed results preserve optional order/client/parent identities
and observed commission-and-fees amount/currency. The capture campaign left no
working orders and a flat AAPL position. See
[`protocol-audit-sv204.md`](protocol-audit-sv204.md).

Exact `server_version 205` is implemented and live-attested. Contract-details
request, regular response, bond response, and end marker migrate to protobuf.
The shared Contract schema has one internal codec and the public `Contract`
owns its selector and composition fields across request families and returned
orders. Description and per-leg execution prices remain response/order
metadata rather than contract identity. See
[`protocol-audit-sv205.md`](protocol-audit-sv205.md).

Exact `server_version 206` is implemented and live-attested. Quote, market-
depth, their cancellations, and market-data-type requests move to protobuf,
along with ten L1/depth callbacks. At exactly sv206, CFD reroute callbacks 91
and 92 retain classic bodies. The current sv225 Gateway sends market-data
reroute as protobuf raw ID 291; both encodings transparently replace the active
request. See
[`protocol-audit-sv206.md`](protocol-audit-sv206.md).

Exact `server_version 207` is implemented and live-attested. Managed accounts,
account updates, account summary, positions, and their multi-account variants
move to protobuf together. Public routing and lifecycle semantics remain
unchanged. See [`protocol-audit-sv207.md`](protocol-audit-sv207.md).

Exact `server_version 208` through 225 are implemented and audited together in
[`protocol-audit-sv208-225.md`](protocol-audit-sv208-225.md). The production
ceiling is 225. Each later migration gate must add its encoder/decoder, live or
official-SDK evidence, and deterministic coverage before the advertised
maximum moves again. The migration table fails closed rather than sending a
classic body after IBKR has retired it.

## Ongoing

- SDK conformance oracle workflow: capture reference traces from the official
  SDK against the local Gateway when adding or hardening a protocol area, and
  fold sanitized live-derived captures into deterministic replay fixtures.
- Protobuf migrations for `server_version` 208+; see above.
- Expanded test coverage and replay scenarios.
- API ergonomics and documentation improvements.

## Not planned

- Client Portal Web API.
- Flex.
- `EWrapper` / `EClient` official-style bridge.
- The official IBKR C++ SDK as a runtime engine. The SDK is permitted only as
  a conformance oracle and capture tool; production code stays pure Go on the
  default import path.

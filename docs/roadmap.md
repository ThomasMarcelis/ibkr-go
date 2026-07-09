# Roadmap

## Direction

`ibkr-go` is a pure-Go, idiomatic client for the Interactive Brokers
TWS/Gateway socket protocol. The runtime stays pure Go: no cgo, no C++
toolchain, no SDK dependency on the production import path. See
[`AGENTS.md`](../AGENTS.md) for the full policy and
[`docs/architecture.md`](architecture.md) for the runtime model.

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
record market-open captures through `scripts/record-scenarios.sh`. The scripts
derive each scenario's role from the catalog risk class, keeping the capture
target in one place. Every promoted behavior must still follow the
live-evidence path: live run, capture verification, sanitized transcript,
public replay test, and updates to the coverage matrix and tracker.

Current IBKR drift to check first:

- The public [IBKR API Software](https://interactivebrokers.github.io/) page
  listed API Latest 10.47, released 2026-05-20, and recommended TWS or IB
  Gateway 1045+ when checked on 2026-05-25; the download page was unchanged
  on a 2026-06-10 re-check and both local Gateway roles still negotiated
  `server_version 200`, so the pinned codec baseline holds. The official
  [production release notes](https://www.ibkrguides.com/releasenotes/prod-2026.htm)
  already document a 10.48 change — `reqOpenOrders` will include
  de-activated orders — to re-verify against `Orders().Open` expectations
  once the local Gateways update.
- Fundamental data de-support or replacement behavior.
- `$LEDGER-` account-value prefixes from the new account-value setting.
- Fractional `tickSize` values and newer generic tick families. Concrete
  probe target: odd-lot ticks 105-110 behind generic tick 787 (official docs
  gate them on TWS/API 10.46+; observe what a `server_version 200` session
  returns).
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
| 3 | Entitlement and account blocker ledger | Keeps missing subscriptions from looking like library regressions | 10089 market data, 10187 historical ticks, 10358 fundamentals, 10276 WSH |
| 4 | Protocol drift and version edges | Keeps the pure-Go codec current with IBKR releases | current-time millis, `$LEDGER-` account values, fractional tick sizes, order/completed-order tail fields |
| 5 | Multi-asset expansion | Proves the facade across real product classes | OPT/BAG/FOP with permissions, BOND identifier, CFD/CRYPTO/FUND probes |
| 6 | Public ergonomics and examples | Helps users trust and adopt the library | examples for live roles, error handling, replay-backed behavior notes, pkg.go.dev polish |
| 7 | Advanced order semantics (after the protobuf decision) | Closes gaps between "order placement works" and "order model is trustworthy", but these are classic-branch fields whose shape a future protobuf migration could touch — see [Protobuf era](#protobuf-era-sv-201) | brackets, OCA, conditions, scale, hedge, pegged/adjusted families |

Each workstream should stay scoped to one logical change. If the work needs a
large plan, put the plan in a repo doc and keep the next handoff request
focused on the next slice and its completion criteria.

## Current state

ibkr-go covers the current public facade across the major Interactive Brokers
TWS/Gateway socket protocol domains. The core protocol areas below are
implemented and validated against live IB Gateway `server_version 200`, but
rare official callbacks, new SDK additions, entitlement-dependent products, and
some advanced order branches still need the live-evidence path described
above.

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
(msg 83).

### Market data

Quote snapshot and streaming, real-time bars, historical bars, tick price,
tick size, and market data type. TickGeneric (inbound 45), TickString
(inbound 46), TickReqParams (inbound 81), reqMarketDataType encoder
(outbound 59), cancelHistoricalData encoder (outbound 25),
reqTickByTickData (msg 97/98), and historical bars keepUpToDate flag.

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

reqScannerParameters (msg 24), reqScannerSubscription (msg 22/23).

### Order and execution observation

Open orders snapshot and streaming (all three scopes), executions finite query,
and commission-and-fees reports. The classic sv200 execution and fee payloads
are fully projected; nondefault filters and meaningful bond yield/redemption
remain live-attestation targets.

### Order management

PlaceOrder (msg 3), CancelOrder (msg 4), reqGlobalCancel (msg 58). OrderHandle
tracks lifecycle with Events(), Lifecycle(), Done(), Wait(), Close(), Cancel(),
and Modify(). Order IDs are auto-allocated from NextValidID. OpenOrder
messages are dual-dispatched to both per-order handles and the singleton
open-orders observer; OrderStatus remains part of the per-order handle
contract. OrderHandle survives disconnects (Gap/Resumed) and auto-closes on
terminal status (Filled, Cancelled, ApiCancelled, Inactive).

### Market depth (Level 2)

reqMktDepth (msg 10), cancelMktDepth (msg 11), inbound MarketDepth (12) /
MarketDepthL2 (13) — full order book depth as a keyed subscription. Requires a
paid L2 market data subscription.

### Fundamental data

FundamentalData (msg 52/53, inbound 51) — Reuters fundamental data as a keyed
one-shot returning the XML payload. Requires a subscription.

### Exercise options

ExerciseOptions (msg 21) — fire-and-forget option exercise request.

### FA configuration

RequestFA (msg 18), ReplaceFA (msg 19), inbound ReceiveFA (16),
reqSoftDollarTiers (msg 79), inbound SoftDollarTiers (77). FA-only account
configuration.

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

- Delta-neutral order extensions.
- Scale order extensions.
- Remaining ungrounded OpenOrder branches.
- CompletedOrder full-detail public projection (the exact classic wire parser is complete).
- Millisecond-precision current-time pair (`reqCurrentTimeInMillis` /
  `currentTimeInMillis`) added in newer SDK releases.
- Order-side coverage promised by the public API but not yet exhaustively
  grounded: OCA group semantics, bracket parent/child sequencing, condition
  families (price, time, margin, execution, volume, percent-change), IB algo
  parameter passthrough, hedging, and short-sale fields. Sequenced after the
  protobuf decision (see below), since these are classic-branch fields.
- ~~Server-version coverage beyond exactly `server_version 200`~~ **Done.**
  The client negotiates `server_version` 176..200 and gates every post-176
  wire field on the negotiated value; live-validated 2026-07-04/05 by
  down-negotiating the paper Gateway to 176/184/193/195/199/200 across
  contract details, historical bars, API-error frames, and the
  `CurrentTimeMillis` feature gate. The remaining server-version gap is
  crossing into `server_version` 201+ (protobuf), tracked separately below.

[`docs/live-coverage-matrix.md`](live-coverage-matrix.md) and
[`docs/message-coverage.md`](message-coverage.md) are the authoritative gap
trackers; new gaps land there first, then graduate here.

## Protobuf era (sv 201+)

IBKR's later Gateway/TWS releases move part of the wire protocol to
protobuf-framed messages at `server_version` 201 and above. `ibkr-go`
advertises a maximum of 200 (`maxServerVersion` in `engine.go`) and does not
negotiate higher — this ceiling is a deliberate wall, not an oversight.
`codec.DecodeBatch` already peeks the msg-id from raw frame bytes before
parsing any fields (see [`docs/architecture.md`](architecture.md#codec-dispatch)),
which is exactly the dispatch seam a mixed classic/protobuf wire needs: it
can choose NUL-delimited versus protobuf decoding per frame before the field
data, which may contain embedded NULs, is touched. Crossing the wall means
new protobuf message definitions, decode/encode paths for whichever messages
IBKR has migrated, and a fresh live-capture and replay campaign — scoped as
one coordinated future project, not a series of piecemeal codec changes
layered onto the classic path.

## Ongoing

- SDK conformance oracle workflow: capture reference traces from the official
  SDK against the local Gateway when adding or hardening a protocol area, and
  fold sanitized live-derived captures into deterministic replay fixtures.
- Protobuf era (`server_version` 201+); see above.
- Expanded test coverage and replay scenarios.
- API ergonomics and documentation improvements.

## Not planned

- Client Portal Web API.
- Flex.
- `EWrapper` / `EClient` official-style bridge.
- The official IBKR C++ SDK as a runtime engine. The SDK is permitted only as
  a conformance oracle and capture tool; production code stays pure Go on the
  default import path.

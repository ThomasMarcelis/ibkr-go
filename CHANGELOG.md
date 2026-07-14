# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

## v2.0.0-rc.3 — 2026-07-16

RC.3 continues the RC.2 hardening work. It extends the supported Gateway range
from `server_version` 200–207 to 200–225, completes the API 10.48 protobuf and
wire-layout migrations, and freezes the advanced order and broker-echo model.

The candidate also tightens ownership and terminal-error behavior across
disconnects, reconnects, cancellation, order placement, option exercise, and
regulatory snapshots. Finite streams, passive execution events, historical-news
iteration, TWS configuration, and odd-lot quote data round out the public API.

All legacy transcript exceptions have been replaced or retired, and the full
build, test, race, lint, vulnerability, capture, and 17-target fuzz gates pass.
The remaining regulatory-snapshot, manual paper-TWS `orderBound`, and seven-day
soak requirements apply to the stable v2.0.0 release, not this candidate.

## v2.0.0-rc.2 — 2026-07-11

v2 is a clean-break release on the `github.com/ThomasMarcelis/ibkr-go/v2` module path. Existing v1 users remain on v1 until they explicitly change imports.

### Highlights

**Gateway coverage.** The supported range is exactly `server_version` 200–207. Protobuf support follows IBKR's migration boundaries: executions at 201, zero-strike contract presence at 202, orders at 203, open/completed orders at 204, contracts at 205, market data/depth at 206, and accounts/positions at 207.

**Orders and executions.** Order handles preserve late executions and revised fee reports instead of closing at the first terminal-looking status. Execution snapshots include commission-and-fees reports, and `SubscribeExecutions` remains open for late corrections. What-if previews, completed orders, contract details, scanners, quotes, depth, and account callbacks retain substantially more of the Gateway payload.

**New operations.** The release adds regulatory snapshots and client-0 `orderBound` routing. WSH requests validate JSON and cancel cleanly. Historical-news pagination is preserved.

**Correctness.** Typed Gateway errors retain request IDs and advanced rejection details. Reconnects, slow consumers, cancellation uncertainty, bracket admission, warning events, malformed frames, unset decimals, and classic/protobuf version boundaries now fail or recover explicitly instead of silently losing state.

**RC.2 failure-path hardening.** Admitted order frames are tracked until the
socket write completes, singleton one-shots cannot wedge after context
cancellation, resume transport loss retains resumable subscriptions, order
errors use an attested terminal set, request/order IDs stay monotonic across
Gateway reseeds, and reconnect dial/bootstrap work no longer stalls the actor.
Subscription data and lifecycle boundaries now share one ordered event stream.

### Breaking changes

| Area | Migration |
|---|---|
| Module | Change imports to `github.com/ThomasMarcelis/ibkr-go/v2` |
| Lifecycle | `Close()` returns no value; read `Wait()` or `Err()` for terminal outcomes |
| Streams | `Subscription.Events()` now yields ordered `StreamEvent[T]`; use `All(ctx)` for data only; remove `Lifecycle()` handling |
| Orders | `Modify` is now `Replace`; close handles explicitly after collecting late events |
| Results | `Executions` returns `ExecutionSnapshot`; `HistoricalNews` returns `HistoricalNewsResult`; `Exercise` returns `ExerciseHandle` |
| Presence | Contract strike, open-order prices, quote/depth sizes, snapshot permissions, and fee values use pointers where absence differs from zero |
| Ownership | Order identity and preview mode moved out of `Order`; contract selection and composition moved onto `Contract` |
| Names | `Buy`/`Sell` became `ActionBuy`/`ActionSell`; commission types use commission-and-fees terminology; P&L fields use `PnL`; `OrderStatusAPICancelled` follows Go initialism casing |
| Accounts | Summary and position subscriptions emit direct values; account group and exact-account filtering are separate |
| Resume | Remove `WithDefaultResumePolicy`; configure `ResumeAuto` per supported subscription |
| Removed | Reuters fundamental data, FA mutation, `ibkr-probe`, and pre-200 compatibility |
| Test infrastructure | Repository replay/live helpers moved under `internal/`; external tests use the public `Dialer` seam or their own fixtures |

See [Migrating from v1 to v2](docs/migration-v2.md) for before-and-after examples and the complete source migration checklist.

### Release-candidate validation

The rc.1 baseline passed deterministic tests, race tests, lint, vulnerability
scan, 386, and the fuzz battery. RC.2 reruns those gates after the changes
above. With both Gateways restored on 2026-07-11, the isolated sv200/205/206
matrix passed on its second complete run (one earlier sv205 bootstrap ended in
EOF), keep-up-to-date bars, tick-by-tick midpoint, historical bid/ask and
midpoint ticks, and PnLSingle completed. Real-time bars returned the expected
10089 entitlement error; historical TRADES ticks received no Gateway reply in
two isolated 20-second captures. On 2026-07-15, exactly one explicitly
authorized fee-bearing regulatory request reached readonly-live at sv225 and
received definitive restriction code 10213; it was not retried. A successful
regulatory snapshot and a raw paper-TWS `orderBound` capture therefore remain
validation targets before v2.0.0 stable.

## v1.5.1 — 2026-07-04

### Added

- **`Orders().RefreshOpen` resyncs an active open-orders subscription
  ([#21](https://github.com/ThomasMarcelis/ibkr-go/issues/21)).** The
  open-orders reply carries no request ID on the wire, so a one-shot
  `Orders().Open` cannot coexist with `SubscribeOpen`; refresh re-sends the
  subscription's request and the Gateway answers with a fresh snapshot burst
  followed by another `SnapshotComplete` lifecycle event. Returns
  `ErrNoSubscription` with no active subscription and `ErrNoSnapshot` for
  auto-scope subscriptions (the live Gateway sends no `open_order_end` for
  `reqAutoOpenOrders`). Grounded by a 2026-07-04 `server_version 200`
  capture frozen as `api_open_orders_refresh_aapl.txt`.

## v1.5.0 — 2026-07-04

### Changed (breaking)

- **`OpenOrderUpdate` is a union of `Order` and `Status` pointers.** The
  open-orders subscription now carries order-status transitions alongside
  open-order echoes; exactly one field is non-nil per event, mirroring
  `OrderEvent` and `ExecutionUpdate`. Code that read the former value-typed
  `Order` field checks `Order != nil` and dereferences.
- **`OpenOrder` no longer carries `Filled`/`Remaining`.** Live
  `server_version 200` open-order frames never carry a fill echo; the
  fields existed because an early capture test misparsed a two-frame chunk
  (an open_order plus an embedded order_status) as one frame, and the
  replay harness fossilized that shape. Fill state is `OrderStatus` truth:
  read it from `OrderStatusUpdate` events or `OrderHandle` status. The
  replay encoder now emits the live wire layout exactly, and the decoder
  has a single live-grounded walk in place of the synthetic branch and the
  misparse-derived 169-field shortcut. Replay transcripts written against
  `testing/testhost` drop `filled`/`remaining` keys on `open_order` lines
  for the same reason.

### Added

- **`cmd/ibkr-doctor` verifies both Gateway roles before a capture
  campaign.** Order-capable live tests and capture scenarios route through
  the disposable `paper-dev` Gateway role; read-only evidence comes from
  `readonly-live`.
- **Verified `server_version 200` captures are frozen as deterministic
  replay tests.** Order campaigns (rapid-fire cancel, forex rejection,
  bracket, OCA, scale-in, trailing-bracket rejection), market-data replays
  (duplicate quote subscriptions, market-data type cycle, tick-by-tick and
  real-time bars request/entitlement errors), reference-data replays
  (contract-details asset matrix: option chain, forex, futures ladder,
  not-found, ambiguous qualify; security-type probes, fundamental reports
  later retired after IBKR API 10.47 de-support,
  WSH entitlement), and the order-conditions matrix (all six condition
  families accepted live) are public replay tests, each traceable to a
  capture hash.
- **Named IBKR error-code constants and `APIError` classification
  helpers.** Every code attested in the replay fixtures has an `ErrCode`
  constant; `IsEntitlement`, `IsConnectivityTransition`, `IsFarmStatus`,
  and `IsWarning` classify them, and an evidence-walk test keeps the
  registry aligned with the fixtures in both directions.

### Fixed

- **Status transitions for orders without a live `OrderHandle` reach
  `SubscribeOpen` ([#20](https://github.com/ThomasMarcelis/ibkr-go/issues/20)).**
  Orders recovered through the open-orders subscription after a restart had
  their `order_status` frames silently dropped, so a cancel confirmed by the
  Gateway was never observable. Status frames now dual-route to the
  subscription exactly like `open_order` frames, and the one-shot
  `Orders().Open` snapshot filters the paired statuses the live Gateway
  interleaves into the recovery snapshot. Grounded by a 2026-07-04
  `server_version 200` capture frozen as
  `api_reconnect_recovered_cancel_status_aapl.txt`.
- **The option pipeline works against the live Gateway.** Four phantom or
  missing wire fields, each mirrored between the codec and the replay
  harness so the suite stayed green, broke every option flow live:
  `secDefOptParams` decode skipped a nonexistent marketRuleId and killed
  the session on the first row of every option qualify; the option calc
  requests carried a phantom includeExpired that drew code 320;
  `exerciseOptions` ended before the `server_version 200` tail and drew
  code 10300; and `tickOptionComputation` kept a legacy version skip that
  aborted the session on the first real greeks reply. One live run now
  walks qualify, real greeks, an option order, and a lapse answered by the
  real code 322.
- **`ContractDetails` carries the contract multiplier and open-order
  echoes decode fully.** The v200 contractData multiplier slot was
  skipped, and `DeltaNeutralOrderType "None"` frames (every plain live
  order) fell to a partial decode that dropped condition echoes, the
  order-state tail, and the wire parentId; bracket children now surface
  their real `ParentID`.
- **Execution times in the Gateway's UTC dash form are parsed.** Live
  executions arrived as `yyyymmdd-hh:mm:ss` and were dropped with their
  commission reports; both now reach the order handle.
- **Contract-bound order conditions encode in the official field order.**
  Price, volume, and percent-change conditions placed through v1.4.5/v1.4.6
  were rejected live with code 320 field-parse errors because the condition
  value was serialized after the conId/exchange pair. Encoder, decoder, and
  replay harness now follow the official client hierarchy; all six
  condition families were re-verified accepted against a live
  `server_version 200` Gateway.
- **Live-derived fixtures no longer embed the paper account identifier.**
  Transcripts and codec tests use sanitized placeholders;
  `docs/transcripts.md` documents the scrubbing contract, and a
  sanitization test now fails the suite on any unredacted account-id shape
  in tracked files.

### Changed

- **Replay promotion status was realigned across the capture catalog,
  coverage matrix, and tracker** so the three ledgers agree; remaining gaps
  (what-if margin preview, IOC/FOK recapture) are recorded as explicit
  ledger entries.

## v1.4.6 — 2026-04-25

### Fixed

- **Completed-order tail decoding is resilient to server_version 200 tail drift.**
  Completed-order payloads include tail fields that must be validated against
  the structural suffix before decoding; quantities now flatten to observed
  filled values consistently.
- **`ReqExecutions` matches the server_version 200 wire layout.** The v200
  shape requires `lastNDays=2147483647` and `specificDatesCount=0`; codec,
  testhost, and a live one-share round-trip freeze that contract.
- **PlaceOrder default-int fields always serialize as decimal digits.**
  `OcaType`, `TriggerMethod`, `DisplaySize`, and `AdjustableTrailingUnit`
  follow IBKR reference clients' `send(int)` semantics: zero is a semantic
  value, not unset. Scale-size fields keep the `sendMax(int)` empty-sentinel
  encoding.

## v1.4.5 — 2026-04-14

### Changed (breaking)

- **`minServerVersion` raised to require CME tagging on cancel requests.**
  Gateway server_version 200 rejects individual and global cancel messages
  without CME tagging fields. Older servers are refused at connect time.

### Added

- **Public `Order` exposes the full advanced-order field surface.** Live
  capture scenarios now drive brackets, OCA, conditions, combos, and pegged
  orders through the typed client API rather than raw protocol shims.
- **Capture tooling records structured driver events and scenario
  checkpoints.** `driver_events.jsonl`, multi-leg recorder support, transcript
  skeleton generation in `ibkr-normalize`, and IOC/FOK plus pegged coverage
  in the capture matrix.

### Fixed

- **Subscription and OrderHandle drain `Events()` before closing `Done()`.**
  Consumers that drain events during teardown see every buffered execution
  and commission before `Done()` fires; the select-on-Done race is closed.
- **`OrderHandle` drains terminal routes briefly before closing.** Live
  delayed-modify-to-market captures showed `Filled` can arrive before
  execution and commission callbacks; the drain window preserves the full
  terminal trail.
- **Execution-time parsing handles Gateway's native format.**
  `parseExecutionTime` accepts `"YYYYMMDD HH:MM:SS TZ_NAME"` alongside
  RFC3339, fixing `ExecutionDetail` time extraction.
- **`ExecutionDetail` decoder skips the currency field.** The contract block
  includes a currency field between `exchange` and `localSymbol` that had
  been misaligning `execID`, time, and side extraction.
- **Code 202 is routed as a cancellation notice, not a request failure.**
  Terminal order status remains authoritative; cancellation acknowledgements
  no longer surface as order errors.

## v1.4.4

### Added

- **Subscriptions expose close errors without waiting on termination.**
  `Subscription.Err()` returns the currently recorded terminal close reason, so
  adapters can inspect retryability after `Done()` fires without calling `Wait()`.
- **Historical request vocabularies are publicly validatable.** `BarSize`,
  `HistoricalDuration`, and `WhatToShow` now expose `Valid()` methods, and
  caller-side historical request failures return `*ValidationError`.
- **TCP keepalive is explicitly configured for Gateway/TWS sockets.**
  Connections default to 30-second TCP keepalive; non-positive
  `WithTCPKeepAlive` values opt out.

### Fixed

- **Automatic reconnect now keeps retrying after failed redials.** Transient
  Gateway downtime keeps the session in `Reconnecting` and retries with capped
  exponential backoff instead of closing the client after one failed attempt.

## v1.4.3

### Added

- **Subscription lifecycle events now classify retryability.** `SubscriptionGap`
  and `SubscriptionClosed` include a `Retryable` flag, and `IsRetryable(err)`
  lets consumers classify the final `sub.Wait()` error after an `Events()`
  channel closes.

### Fixed

- **Real-time bars API rejections are now distinguishable from reconnect gaps.**
  Live Gateway can accept `reqRealTimeBars` and then asynchronously reject it
  with a request-scoped API error. The subscription still closes with the
  `*APIError`, but lifecycle state and `IsRetryable` now mark that terminal
  rejection as non-retryable so consumers do not start reconnect storms.

## v1.4.2

### Fixed

- **Completed-order decoding now follows the live Gateway field layout.** Live
  IB Gateway can send `OrderState.CompletedStatus` values such as
  `Cancelled by System:\n` in completed-order message `101`; the decoder no
  longer exposes that status text as `Filled` or attempts to parse it as a
  decimal. `Orders().Completed` now treats absent completed-order filled and
  remaining quantities as zero.
- **Execution snapshots without `ExecutionDataEnd` are frozen as
  context-driven.** Live Gateway can omit message `55` for empty execution
  snapshots. `Orders().Executions` continues to respect caller context
  cancellation/deadlines, and the live-derived regression now covers that
  behavior.

## v1.4.1

### Fixed

- **Historical bars no longer poison persistent client sessions.** Live IB
  Gateway can send `msg_id 108` as a historical-data range terminator after a
  packed one-shot historical bars response. The codec now decodes both observed
  terminal shapes (`reqID,start` and `reqID,start,end`) as
  `HistoricalBarsEnd` while preserving numeric `HistoricalDataUpdate` bars for
  keep-up-to-date streams.
- **Fatal inbound decoder errors now close the transport socket.** Protocol
  errors still terminate the session, but the underlying connection is closed
  immediately so IB Gateway releases the client ID before reconnect attempts.

## v1.4.0

### Changed (breaking)

- **Replaced custom `Decimal` with `shopspring/decimal`.** All prices,
  quantities, and money fields now use `decimal.Decimal` from
  `github.com/shopspring/decimal`. Consumers get exact arithmetic (Add, Sub,
  Mul, Div, Cmp) without float64 conversion. `ParseDecimal`,
  `MustParseDecimal`, and `ErrInvalidDecimal` are removed; use
  `decimal.NewFromString`, `decimal.RequireFromString`, or `decimal.NewFromInt`
  instead. This is the library's first external dependency.
- **Moved the public package to the module root.** Callers now import
  `github.com/ThomasMarcelis/ibkr-go`; the old `/ibkr` package is removed.
- **Reshaped `Client` into lifecycle plus domain facades.** Root `Client`
  keeps session lifecycle methods and exposes concrete accessors such as
  `Accounts()`, `Contracts()`, `MarketData()`, `History()`, `Orders()`,
  `Options()`, `News()`, `Scanner()`, `Advisors()`, `WSH()`, and `TWS()`.
  The old flat operation methods are removed.
- **Flattened contract metadata surface.** `ContractDetails` now embeds
  `Contract`, so callers read `details.Symbol`, `details.ConID`,
  `details.LongName` at one level. `ContractDetailsRequest`,
  `QualifiedContract`, and `MatchingSymbolsRequest` single-field wrappers are
  deleted. `Contracts().Details(ctx, Contract)`,
  `Contracts().Qualify(ctx, Contract) (ContractDetails, error)`, and
  `Contracts().Search(ctx, string)` all take bare values instead of
  wrappers.
- **Strengthened public vocabulary types.** Stable protocol vocabularies now
  use named types/constants, including market data type, what-to-show, bar size,
  tick-by-tick type, order type/status, FA data type, exercise action, market
  depth operation/side, fundamental report type (since removed after official
  de-support), news provider code, and display group IDs.
- **Reworked historical and raw payload boundaries.** Historical bars use
  `HistoricalDuration` and `BarSize`; historical tick/news windows use
  `time.Time` (zero time means unset). Scanner, FA, and the since-removed
  fundamental-data surface return `XMLDocument`;
  WSH payloads return `JSONDocument`; display groups return `[]DisplayGroupID`.
- **Renamed lifecycle APIs.** `Subscription.State()` and `OrderHandle.State()`
  are now `Lifecycle()`. `Subscription.AwaitSnapshot(ctx)` provides a durable
  snapshot-completion wait and returns `ErrInterrupted` when the subscription
  is cancelled or closed before a snapshot boundary is reached, rather than
  silently returning `nil`.
- **Executions are no longer public subscriptions.** `Orders().Executions` is
  the public finite execution query.
- **Removed redundant `SessionState`, `SessionSnapshot`, and `SessionEvent`
  type aliases.** Callers use `State`, `Snapshot`, and `Event` directly.

### Added

- **Order management**: `Orders().Place`, `Orders().Cancel`, and
  `Orders().CancelAll` with `OrderHandle` lifecycle tracking.
  Auto-closes on terminal status (Filled, Cancelled, Inactive).
- **Market depth (Level 2)**: `MarketData().SubscribeDepth` for full order book depth.
- **Fundamental data**: `Contracts().FundamentalData` for Reuters XML reports
  (removed in Unreleased after IBKR API 10.47 de-support).
- **Exercise options**: `Options().Exercise` fire-and-forget request.
- **FA configuration**: `Advisors().Config`, `Advisors().ReplaceConfig`,
  `Advisors().SoftDollarTiers`.
- **WSH calendar**: `WSH().MetaData`, `WSH().EventData`.
- **Display groups**: `TWS().DisplayGroups`, `TWS().SubscribeDisplayGroup`.
- **ParentID support** in OpenOrder for bracket and attached order tracking.
- Comprehensive GoDoc comments on all public types, methods, constants, and
  variables.
- **Runnable examples** in `examples/` for connect, quotes, historical bars,
  portfolio, and order placement.
- Additional Example functions for pkg.go.dev.
- CHANGELOG.md.

### Fixed

- **Persistent client sessions no longer degrade after the first one-shot
  request.** The transport layer previously called `finish()` (tearing down
  the connection) when the send queue was full. Now it returns
  `ErrSendQueueFull` without side effects, so a transient backpressure spike
  no longer permanently kills the session. Combined with context-aware sends,
  bootstrap timeouts, ready-retry logic, and prioritized message draining,
  long-lived clients can serve sequential and concurrent requests indefinitely.
  (Fixes #5)
- **Historical data request pacing prevents IBKR rate-limit disconnects.**
  The engine now enforces a 2 s minimum spacing between any historical
  requests and 15 s between identical requests, matching IBKR's documented
  pacing rules. Requests that arrive too early are transparently deferred
  rather than rejected.
- **`CommissionReport` decoding no longer fails on unset fields.** Live TWS
  emits the Java `Double.MAX_VALUE` sentinel for `Commission` and `RealizedPNL`
  when the server has not yet computed those values. The receive path now
  decodes both the sentinel and the empty-string form to a zero decimal,
  matching the existing open-order commission handling. Previously, a
  sentinel-valued commission either silently vanished on the order-handle
  dispatch path or tore down the executions subscription.
- **Per-order dispatch decode failures are now observable.** A malformed
  `CommissionReport` or `ExecutionDetail` routed to a live `OrderHandle`
  emits a `Warn`-level record via the configured logger (opt-in via
  `WithLogger`) while still dropping the event to keep the handle alive — the
  order remains valid on the server, so the handle must not terminate.
- **Historical tick/news windows now include explicit time zones.** The
  `time.Time` request APIs no longer emit UTC wall-clock strings without a
  zone suffix, which TWS can reinterpret in the login timezone.
- **Matching-symbol responses now decode the live `SymbolSamples` frame.** Live
  Gateway sends symbol samples as inbound message `79` and includes
  description/issuer fields after derivative security types; the codec now
  consumes those fields and exposes them on `MatchingSymbol`.
- **Historical-news live frame IDs are corrected.** Live Gateway sends
  historical news items as inbound message `86` and the end marker as `87`.
- **Historical-news timestamps now parse live date strings.** Gateway responses
  may use `yyyy-MM-dd HH:mm:ss.s` instead of epoch milliseconds.

### Changed

- README rewritten: punchier opening, bullet-based Why section, consolidated
  per-library comparison table, removed inline API overview.
- Package overview (doc.go) expanded to cover all major patterns: connecting,
  one-shots, subscriptions, orders, session lifecycle, errors, financial types.
- Roadmap updated to reflect full API coverage.
- Default logger uses `io.Discard` directly instead of a local replacement.
- Internal `engine.SubscribeExecutions` renamed to lowercase
  `engine.subscribeExecutions` to match its effective visibility.
- Live verification defaults now target the paper Gateway port `4002`, with
  `IBKR_LIVE_TRADING=1` required for order-placing live tests.

## v1.0.0

Initial release covering the full read-only TWS API surface.

### Added

- **Session**: DialContext, Close, Done, Wait, Session, SessionEvents.
  Observable state machine (Disconnected, Connecting, Handshaking, Ready,
  Degraded, Reconnecting, Closed). Automatic reconnect with configurable policy.
- **Account and portfolio**: AccountSummary, SubscribeAccountSummary,
  PositionsSnapshot, SubscribePositions, AccountUpdatesSnapshot,
  SubscribeAccountUpdates, AccountUpdatesMultiSnapshot,
  SubscribeAccountUpdatesMulti, PositionsMultiSnapshot,
  SubscribePositionsMulti, SubscribePnL, SubscribePnLSingle, FamilyCodes,
  CompletedOrders.
- **Market data**: QuoteSnapshot, SubscribeQuotes, SubscribeRealTimeBars,
  SubscribeTickByTick, SubscribeHistoricalBars, SetMarketDataType.
- **Contract and reference**: ContractDetails, QualifyContract, MatchingSymbols,
  MarketRule, SecDefOptParams, SmartComponents, MktDepthExchanges.
- **Historical data**: HistoricalBars, HeadTimestamp, HistogramData,
  HistoricalTicks.
- **Options**: CalcImpliedVolatility, CalcOptionPrice.
- **News**: NewsProviders, NewsArticle, HistoricalNews, SubscribeNewsBulletins.
- **Scanner**: ScannerParameters, SubscribeScannerResults.
- **Order and execution observation**: OpenOrdersSnapshot, SubscribeOpenOrders,
  Executions, SubscribeExecutions.
- **Typed subscriptions**: Generic Subscription[T] with Events/State/Done
  lifecycle separation.
- **Exact decimal type** for all prices and money.
- **Zero external dependencies**.
- **Replay transcripts** from live IB Gateway captures for deterministic CI.
- **Fuzz testing** on wire protocol (frame parsing, field encoding, codec
  round-trips).

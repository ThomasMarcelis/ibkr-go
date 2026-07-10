# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Added

- Quote subscriptions now deliver every implemented classic L1 callback.
  `QuoteUpdate.Kind` preserves every classic price and size callback, including
  its numeric tick type, price attributes, and optional price-frame companion
  size, even when it has no normalized `Quote` field. Numeric generic ticks,
  string ticks, request parameters, option computations, and contract-specific
  news headlines remain distinct payloads; ancillary events retain the
  cumulative `Quote` without mutating it. News ticks expose the provider time,
  code, article ID, headline, and provider metadata. Option computations expose
  an availability bitmask so IBKR's
  field-specific `-1`/`-2` sentinels no longer masquerade as real values.
  `Quote.Volume` normalizes live and delayed volume, and omitted request
  minimum ticks are represented by `nil` rather than terminating a stream.

- Scanner subscriptions now expose the complete classic request: every legacy
  numeric, rating, maturity, and stock filter plus generic filter and
  subscription options. Optional scalar filters distinguish unset from an
  explicit zero, and scanner tag values use IBKR's `tag=value;` wire format.
  The previous one-field-short encoder and untraceable success replay are
  removed. A fresh public-API capture now freezes the exact server-version-200
  request, ten ranked `scannerData` results, and clean cancellation.

- Order cancellation accepts optional, locally validated compliance metadata:
  `WithManualCancelTime`, `WithCancelExternalOperator`, and
  `WithCancelManualOrderIndicator`. The options work through both direct and
  handle cancellation; the two CME tagging fields also apply to global
  cancellation. Unsupported negotiated versions fail before any frame is
  sent.

- Exact `server_version 201` support adds the raw-ID envelope and the first
  protobuf family, `Orders().Executions`, implemented directly with Go's
  protobuf wire primitives. Live empty and non-empty paper queries freeze the
  request, execution detail, end marker, and mixed classic bootstrap behavior;
  unknown protobuf frames retain their binary payload for protocol-drift
  diagnosis.

- Exact `server_version 202` support freezes the official zero-strike-only
  boundary. API 10.48.01 migrates no message at this version; a live public
  conId-only contract lookup and a sanitized non-empty execution replay prove
  that a protobuf Contract preserves both its conId and an explicitly present
  strike of zero.

- Exact `server_version 203` support migrates place order, targeted cancel,
  global cancel, and their open-order/status callbacks to protobuf using
  pure-Go wire codecs. The lifecycle also freezes the protobuf error family
  already introduced at 201. A guarded far-limit paper order was observed,
  cancelled, globally cleaned up, and promoted to a sanitized replay.

- Exact `server_version 204` support migrates client/all/auto open-order
  requests and completed-order request/results to protobuf. Completed orders
  preserve presence-aware order, client, and parent identities plus observed
  commission-and-fees amount/currency. Exact official-source vectors, a
  sanitized cancelled/filled paper replay, and a read-only guarded live check
  freeze the boundary without placing another order.

- Exact `server_version 205` support migrates contract-details requests and
  regular, bond, and end responses to protobuf. One shared pure-Go Contract
  codec now owns the schema used across executions, order flows, and contract
  data. Live stock, bond, fund, option, issuer, and ineligibility captures
  freeze the boundary; the public result adds optional algorithmic-minimum and
  price/size-precision decimals plus the source-defined event-contract fields.

- Exact `server_version 206` support migrates quote, market-depth, their
  cancellations, and market-data-type requests plus ten L1/depth callbacks to
  protobuf. Quote request parameters preserve omitted snapshot permissions and
  last-price/last-size precision. Classic CFD reroute callbacks transparently
  replace active quote/depth requests while preserving subscription settings;
  repeated reroutes fail explicitly instead of looping. Official-source
  schemas, exact live vectors, and a curated public replay freeze the boundary.

### Changed (breaking)

- **`QuoteParameters.SnapshotPermissions` is now `*int`.** Nil means IBKR
  omitted the mask; a pointer to zero is an explicitly present zero. Quote
  parameters also expose optional last-price and last-size precision decimals.

- **`QuoteSizeTick.Size` and `DepthRow.Size` are now `*decimal.Decimal`.** Nil
  preserves the official exact-protobuf `UNSET_DECIMAL` result when IBKR omits
  a standalone tick or depth size; an explicit zero remains a non-nil decimal.

- **Legacy Reuters fundamental data has been removed.** IBKR API 10.47
  de-supported the request, cancellation, callback, and fundamental-ratios
  tick type. `Contracts().FundamentalData`, `FundamentalReportType`, and the
  associated XML result surface are removed; classic outbound message IDs 52
  and 53 and inbound message ID 51 remain unused gaps. WSH is a separate API,
  not a replacement. Earlier changelog and capture records remain historical
  evidence only.

- **Contract details now preserve the complete classic response.** The public
  result adds order capabilities, valid exchanges paired with market-rule IDs,
  trading and liquid hours, security IDs, underlier/classification metadata,
  size rules, the mutual-fund facet, and ineligibility reasons. Futures no
  longer put `YYYYMMDD HH:MM:SS Zone` into `Contract.Expiry`: expiry remains
  date-only, while `LastTradeDate`, `LastTradeTime`, and `TimeZoneID` expose the
  distinct IBKR fields.

- **Executions now expose the complete classic server-version-200 result and
  all nine official filters.** `Execution.Symbol` moves to
  `Execution.Contract.Symbol`; the result adds the full contract, execution
  exchange, permanent/client IDs, cumulative quantity, average price, order
  reference, economic-value fields, model, liquidity, price-revision state,
  and submitter. `ExecutionsRequest` adds client, time, security type,
  exchange, side, last-days, and specific-date filters. The date filters fail
  locally below server version 200 instead of disappearing from the wire.

- **Execution costs use current commission-and-fees terminology and preserve
  absence.** `CommissionReport` becomes `CommissionAndFeesReport`, and the
  `Commission` union fields on `ExecutionUpdate` and `OrderEvent` become
  `CommissionAndFees`. `Amount`, `RealizedPNL`, and `BondYield` are pointers:
  nil means IBKR sent an unset sentinel, while a pointer to zero means a real
  computed zero. Yield redemption dates are retained as validated `YYYYMMDD`
  strings. The decoder no longer discards the classic yield/date tail.

- **Completed orders now expose the full classic order echo as three explicit
  facets:** `Contract`, `Order`, and `Completion`. The old six-field projection
  discarded prices, timing, routing, advanced-order, completion, and compliance
  metadata. `Remaining` is removed because IBKR message 101 does not carry it;
  callers that need working quantity must use order-status or execution data.
  Optional numeric fields use pointers so an explicit zero remains distinct
  from IBKR's unset sentinels.

  ```go
  // Before
  status, qty := completed.Status, completed.Quantity

  // After
  status, qty := completed.Completion.Status, completed.Order.Quantity
  reason := completed.Completion.StatusText
  ```

- **Order warnings no longer close the handle.** An order-targeted `api_error`
  whose `(&APIError{Code: …}).IsWarning()` is true — notably code 399, the
  off-hours "will not be placed at the exchange until …" deferral — is now
  delivered non-terminally as a new `OrderEvent.Warning` field instead of
  closing the `OrderHandle` with that code as its terminal error. The order
  stays working at IB (live replays show it still cancellable), and the
  handle's real lifecycle continues to its actual terminal status. Code paths
  that read the 399 off `OrderHandle.Wait()` must move to consuming the
  `Warning` event from `Events()`.

  ```go
  // Before: the handle terminated on the 399 warning
  err := handle.Wait() // *APIError{Code: 399}, order still working at IB

  // After: the warning is an event; the handle stays open
  for evt := range handle.Events() {
      if evt.Warning != nil { /* code 399, non-terminal */ }
  }
  err := handle.Wait() // nil (or the real terminal status/error)
  ```

- **`Buy`/`Sell` renamed to `ActionBuy`/`ActionSell`.** The old names remain
  as deprecated aliases (`Buy = ActionBuy`, `Sell = ActionSell`) for one
  release and are removed in the next.

  ```go
  // Before
  Order{Action: ibkr.Buy}
  // After
  Order{Action: ibkr.ActionBuy}
  ```

- **`Orders().Place` rejects what-if orders.** Placing an `Order` with
  `WhatIf` set now returns a `*ValidationError` instead of creating a
  live `OrderHandle` for what is really a margin preview. Use
  `Orders().Preview`, which forces the what-if flag and returns the
  Gateway's margin-and-commission block as an `OrderState`; the
  `place_order` frame it sends is byte-identical to the old what-if
  `Place` call.

  ```go
  // Before
  order.WhatIf = ptr(true)
  handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: c, Order: order})
  // handle never terminates normally; margin/commission read off OpenOrder echoes

  // After
  state, err := client.Orders().Preview(ctx, ibkr.PlaceOrderRequest{Contract: c, Order: order})
  // state.InitMarginChange, state.Commission, ... — one-shot, no OrderHandle
  ```

- **`Contract.Strike` is `decimal.Decimal`, not `string`.** Closes the last
  stringly money field in the public contract surface. The zero value
  encodes to the same wire bytes the empty string produced.

  ```go
  // Before
  Contract{Strike: "150"}
  // After
  Contract{Strike: decimal.NewFromInt(150)}
  ```

- **`WithDialer` takes the new public `Dialer` interface, not
  `internal/transport.Dialer`.** `Dialer` has the same single method
  (`DialContext(ctx, network, address) (net.Conn, error)`), so
  `*net.Dialer` and any existing custom dialer keep compiling unchanged;
  only code that named the internal type in its own signature needs to
  switch to `ibkr.Dialer`.
- **The client negotiates IB Gateway `server_version` 176..206, with exact 201
  adding raw message IDs and protobuf executions, exact 202 adding the
  zero-strike contract boundary, exact 203 migrating the order lifecycle,
  exact 204 migrating open/completed-order queries and completed results,
  exact 205 migrating contract data, and exact 206 migrating market data and
  depth.** Fields are gated
  on the version the Gateway actually returns instead of assuming the latest
  layout. The classic sv200, mixed-envelope sv201, zero-strike sv202,
  order-protobuf sv203, completed-order-protobuf sv204, contract-data sv205,
  and market-data sv206
  boundaries are live-attested; versions 176..199 remain compatibility paths
  until independently evidenced, and 207+ is not advertised. The official API
  10.48.01 migration table moves no message at 202; a sanitized live replay
  freezes a protobuf execution Contract with both conId and explicit strike=0.
  `CurrentTimeMillis` returns
  `ErrUnsupportedServerVersion` below 197, the version that introduced
  `reqCurrentTimeInMillis` on the wire.

### Added

- Doc comments across the public API: field-level zero-value semantics
  on `Contract` and `Order` (which price fields each order type reads,
  `*bool` tri-state meaning), and slow-consumer guidance on the
  depth/tick subscription options.
- **`Subscription.All(ctx)` returns an `iter.Seq[T]`** over a
  subscription's events, draining to exhaustion as the canonical
  consumption loop; this replaces the documented two-channel
  `Events()`/`Lifecycle()` select with nil-channel toggling for callers
  that only need business events.
- **Contract and order constructors**: `Stock`, `Forex`, `OptionContract`,
  and `Future` build the common contract shapes; `MarketOrder`,
  `LimitOrder`, `StopOrder`, and `StopLimitOrder` fill the order-type,
  action, quantity, and price fields and leave the rest at server
  defaults.
- **`Orders().Preview`** and the new **`OrderState`** type (see Changed
  above).
- **`OpenOrder.Partial`** flags a degraded parse of an unattested advanced-
  order or version-gated layout, so a partial decode is observable
  instead of silently dropping fields.
- **Public `Dialer` interface** (see Changed above).
- **Widened classic-version compatibility: 176..200**, exercised by
  down-negotiating the paper Gateway (`server_version 200` capped to
  176/184/193/195/199 via the v100+ handshake) across contract details,
  historical bars, API-error frames, and the `CurrentTimeMillis` feature
  gate.
- **`BenchmarkE2EQuoteStreamTCP`** and actor-stage benchmarks: a real
  TCP handshake driving `DialContext` through `SubscribeQuotes` with
  sanitized live `sv200` tick frames, plus the syscall-visible transport
  benches used to measure the read-loop buffering below.
- **CI**: Codecov coverage upload (informational, no gate), tag-driven
  GitHub Releases that pull the matching `CHANGELOG.md` section, and
  `gosec` added to the curated lint set scoped to the production import
  path.
- **239 fuzz corpus entries** from 30s soaks across all 18 wire and codec
  fuzz targets, checked in and replayed on every run.

### Fixed

- **Code 161 no longer turns a successful terminal order into a handle
  error.** "Cancel attempted when order is not in a cancellable state" is a
  cancellation reply, like code 202, rather than a placement rejection. It
  now surfaces on `SessionEvents()` while the real `OrderStatus` remains
  authoritative. This also preserves late executions and commissions inside
  the terminal drain window.
- **`Orders().Place` no longer orphans a live order when its context is
  cancelled.** If the context fires in the window after the `place_order`
  frame reached the wire but before the caller received the handle,
  `Place` now best-effort cancels the now-ownerless order (bounded background
  context) and detaches the handle with the caller's cancellation cause,
  instead of returning the error and leaving a live order resting at IB with
  no handle to reach it.
- **Request-targeted `api_error` codes below 10000 with no matching route now
  surface as session events instead of vanishing.** Previously a `req_id`
  that matched no keyed route and no order route fell off the end of the
  error handler and was dropped; this is the path that carries option-exercise
  refusals (code 322) and stale request replies. Deliberate consequence:
  late replies for already-torn-down requests (a code 161 answering a cancel
  of an already-terminal order, duplicate code 300 responses) are now visible
  on `SessionEvents()` rather than silently discarded — session events are
  informational, and the code is preserved for filtering.
- **A commission report that raced ahead of its execution detail now reaches
  the order handle.** The Gateway can deliver `commission_report` before the
  `execution_detail` it belongs to; previously the handle leg dropped it with
  no owner and nothing re-triggered delivery. It is now buffered and flushed
  when the execution claims the ExecID (unclaimed buffers evict after the
  drain window).
- **A re-sent commission report with changed content reaches the order
  handle.** The snapshot-replay dedupe now compares content instead of ExecID
  presence, so an identical replay is still dropped but a report the Gateway
  re-sends with updated fields (e.g. realized PnL filled in later) is
  delivered.
- **Rejected orders no longer retain their routes until reconnect.** A
  terminal placement rejection, a decode failure, or a slow-consumer close
  now tears down the order route and its execution correlations the same way
  the post-fill drain window does.
- **`Options().Exercise` now registers a keyed route for its request id.**
  Exercise is fire-and-forget on the wire, but its request-id-targeted replies
  (refusals like 322, the 10349 TIF-preset acknowledgement, the 202 that
  cancels a working instruction) were dropped, and the bare request id could
  be mistaken for a live order id — misdelivering an exercise error to an
  unrelated order handle. The route surfaces every notice as a session event
  and keeps the request id out of the order-id space until disconnect.
- **A one-shot `Executions()` snapshot no longer double-delivers fills to a
  live `OrderHandle`.** Executions already seen live are deduped by `ExecID`
  on the order-handle leg, so a snapshot query that replays historical fills
  emits each fill to the handle exactly once. The query's own snapshot result
  still carries every row.
- **Order-targeted `api_error` codes in the 10xxx band now reach the order
  routes instead of unconditionally becoming session events.** A what-if
  rejected with such a code (live-reproduced 2026-07-05: code 10255 on a
  what-if DarkIce placement, paper Gateway `server_version 200`) left
  `Orders().Preview` blocked until its context deadline with the rejection
  reason lost; it now resolves with the `*APIError`. Live `OrderHandle`s
  close with the `*APIError` for the codes live-attested as outright
  placement rejections (10063 invalid FX hedge, 10255 display size not
  allowed) — the Gateway never sends an `order_status` after them, so the
  error is the handle's only completion signal. Order-targeted cancel
  notices (10147/10148) keep surfacing as session events, and the previous
  session-event fallback still covers unattested codes.
- **A data-lost connectivity restoration (code 1101) now interrupts every
  route the Gateway cannot answer instead of only re-sending auto-resumed
  subscriptions.** Non-resumable subscriptions closed with
  `ErrResumeRequired`, in-flight one-shots and pending what-if previews
  resolve with `ErrInterrupted` — mirroring the transport-loss teardown.
  Previously a `ResumeNever` stream stayed open on a dead server-side
  subscription indefinitely and an in-flight one-shot hung until its
  context deadline. Code 1102 (data maintained) still interrupts nothing,
  and live order handles still ride out the gap with Gap/Resumed lifecycle
  events.
- **`reqUserInfo`'s inbound msg-id corrected from 103 to the live-attested
  107.** 103 is `REPLACE_FA_END`; every live `UserInfo` call died with
  `ErrInterrupted` on the unrecognized id because the DSL-form testhost
  encoded server frames with the same (wrong) constant it decoded with,
  which let the bug replay green. Frozen with a capture-decode test on
  the raw live frame (`server_version 200`).
- **`replaceFA` now appends the trailing `reqId` the Gateway has required
  since `REPLACE_FA_END` (157), and its acknowledgement decodes as
  `ReplaceFAEnd` on msg-id 103.** The encoder never sent the id, and the
  ack frame it invites was previously misrouted to `UserInfo` because of
  the 103/107 mix-up above.
- **The testhost replay harness re-encodes server frames at the
  transcript-declared `server_version`** instead of a hardcoded 200. A
  transcript declaring an older version now actually exercises that
  version's layout instead of silently replaying `sv200` bytes, closing
  the class of version-gated bug that replays green and only fails live.
  Sub-200 transcripts must now carry raw captured bytes for server
  frames rather than DSL-form ones, for the same reason.

### Performance

- **Buffered transport read loop.** `wire.ReadFrame` issued two reads
  (length prefix, then payload) per frame; a 64 KiB `bufio.Reader`
  collapses that to roughly one syscall per buffer fill. End-to-end
  throughput on the new benchmark improved ~76% (~995k to ~1.75M
  msgs/sec on the measuring machine); live-verified against the paper
  Gateway.
- **Byte-level codec parsing.** The field reader is now a lazy cursor
  over the frame bytes: numeric fields parse in place via transient
  `strconv` views, and only retained strings are copied, eliminating the
  per-frame `[]string` split. `DecodeBatch` peeks the msg-id from raw
  bytes before any field parsing. Alloc bytes per decode drop 42-83%
  (geomean -55%); the hot tick path is flat in time at -42% bytes; batch
  backfills such as historical bars trade fewer bytes for more small
  string allocations on that infrequent one-shot path.

### Migration notes

Every breaking change in this release, before → after:

- **Actions.** `Buy`/`Sell` → `ActionBuy`/`ActionSell` (old names are
  deprecated aliases, removed next release).

  ```go
  Order{Action: ibkr.Buy}    // before
  Order{Action: ibkr.ActionBuy} // after
  ```

- **What-if orders.** `Orders().Place` on a `WhatIf` order → rejected
  with `*ValidationError`; use `Orders().Preview`.

  ```go
  handle, err := client.Orders().Place(ctx, req) // before: req.Order.WhatIf = ptr(true)
  state, err := client.Orders().Preview(ctx, req) // after: returns OrderState
  ```

- **Contract.Strike.** `string` → `decimal.Decimal`.

  ```go
  Contract{Strike: "150"}                 // before
  Contract{Strike: decimal.NewFromInt(150)} // after
  ```

- **WithDialer.** `internal/transport.Dialer` → public `ibkr.Dialer`.
  Source-compatible for `*net.Dialer` and any dialer that already
  implemented `DialContext(ctx, network, address) (net.Conn, error)`;
  only signatures that spelled out the internal type need to change.

  ```go
  func WithDialer(dialer transport.Dialer) Option // before (internal type leaked)
  func WithDialer(dialer ibkr.Dialer) Option       // after
  ```

- **Minimum server version.** Exactly `200` → range `176..200`, negotiated
  down as needed. `CurrentTimeMillis` now returns
  `ErrUnsupportedServerVersion` on Gateways below 197 instead of the
  wider handshake rejecting them outright.

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
- **`OrderHandle.Modify` rejects mismatched order IDs.** Setting
  `order.OrderID` to a value other than the handle's bound ID returns an
  explicit error rather than silently ignoring the caller-supplied ID. Zero
  remains accepted for the ergonomic "construct a fresh `Order` without
  threading the ID" case.
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

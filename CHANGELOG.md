# Changelog

Notable changes per release, following [Keep a Changelog](https://keepachangelog.com/).
The evidence behind each release (captures, gate runs, coverage) lives at its
tag.

## v2.1.0 - 2026-09-06

### Added

- `OrderHandle.Acknowledged` reports the first attributed broker callback;
  `Subscription.HasSnapshot` reports whether an initial boundary exists.
- `WithOrderExecutionCorrelationLimit` bounds retained order execution IDs
  and pending fee versions (default 4096 each).
- Named error codes for invalid active start time (10315), partial market-data
  subscription requirements (10091), and failed news articles (10172).

### Changed

- Direct cancellation requires `OrderTarget{ClientID, OrderID}` and rejects
  foreign client IDs locally; recovered same-client orders remain cancellable.
- Remove unsupported auction strategy fields, MOO/LOO/PEG PRI order constants,
  and `FADataProfiles`. See the [v2.1 migration guide](https://github.com/ThomasMarcelis/ibkr-go/blob/v2.1.0/docs/migration-v2.1.md).
- Historical tick/news bounds serialize in UTC; option calculations require
  Exchange, and market-rule lookups require a positive ID.

### Fixed

- Preserve healthy open-order observers when one handle overflows.
- Release fees for executions proven unrelated to local handles; retain
  possible owned fees until attribution or observation ends.
- Terminate order observation on captured rejection 10315; preserve uncertainty
  for code-0 regulatory snapshot failures and incomplete bootstrap errors.
- Wake canceled historical admission waits promptly and close SessionEvents
  before Client.Done.
- Restore selected market-data type before reconnect work and preserve a
  hedge's bound parent when Replace omits ParentID.
- Report a complete local order write before reporting connection loss.
- Close replay listeners deterministically; clarify per-method lifecycle,
  cancellation, snapshot, and financial-data interpretation documentation.
- Keep examples copyable and exit successfully on Ctrl-C during connection setup.

## v2.0.3 - 2026-08-29

### Added

- `BenchmarkPublicQuoteStream` measures the capture-backed public quote path.
  The published reference run sustains 1.26 million updates/s with 5.34 µs
  p50 and 11.8 µs p99 delivery latency, 240 B and 12 allocations per update,
  and 20.3 ms from cold dial to first quote.

### Changed

- Performance: concrete actor inputs avoid generic dispatch, ordered protobuf
  fields bypass sorting and copying, and ordinary decimals bypass sentinel
  float parsing.
- Orders with an empty `TIF` are sent as `DAY`. Live sv225 rejects an omitted
  time in force with code 10052, so constructor-built orders and every
  `Orders().Preview` were refused unless the caller set `TIF` by hand.
- Code 10052 (`ErrCodeInvalidTimeInForce`) is an order rejection: the handle
  closes with an error for which `IsOrderRejection` is true instead of staying
  open with a warning.
- Examples: new `bracket`; `quotes` waits for bid, ask, and last; `historical`
  explains an entitlement refusal; all default to the paper port.
- Docs: `roadmap.md` lists next steps only; release records, test trackers,
  the exhaustive test plan, and the sv203–207 audits are gone. Live safety
  rules and the release checklist live in `CONTRIBUTING.md`. `SECURITY.md`
  names the supported line, the issue templates no longer point at internal
  packages, and the API inventory drops the message-ID tables that
  duplicated `internal/protocol`.

### Fixed

- `cmd/ibkr-normalize` keeps protobuf varint width when sanitizing perm IDs.

## v2.0.2 - 2026-08-29

### Added

- `ContractDetails.SettlementMethod` (API 10.50.01 field 65).

### Fixed

- Per-leg-priced BAG limit orders no longer require a combo-level limit price.
- The capture batch script runs under macOS Bash 3.2.

### Known limitations

- Positive bulletin, FA, WSH, market-depth, real-time-bar, historical,
  tick-by-tick, EFP, delta-neutral, and manual `orderBound` callbacks are not
  live-attested; the option-exercise replay proves admission only.
- `Order.conditionsIncludeOvernight` (API 10.50.01) needs server version 226,
  outside the supported range.

## v2.0.1 - 2026-08-27

### Changed

- The supported Gateway range is exactly `server_version` 208–225. Gateways
  on 200–207 no longer connect; upgrade, or stay on v2.0.0.
- `Order.IncludeOvernight` is `*bool`, distinguishing omitted from explicit
  false. Source-compatibility break from v2.0.0.

### Fixed

- A malformed registered callback retires its whole transport generation;
  incomplete requests fail with an error matching both `ErrInterrupted` and
  `*ProtocolError`. Unknown message IDs stay nonfatal.
- Code 2188 (`ErrCodeHistoricalDataSubscriptionRequired`) is classified as an
  entitlement failure.

### Known limitations

- The broker rejects replacing `IncludeOvernight=true` with explicit false
  (code 462, reproduced with SDK 10.48.01).

## v2.0.0 - 2026-08-23

Breaking release on the `github.com/ThomasMarcelis/ibkr-go/v2` module path;
requires Go 1.26. v1 is deprecated.

### Changed

- Typed operations and results replace the `EWrapper`-style callback model.
- Subscriptions deliver data and lifecycle (loss, recovery, snapshot
  completion, errors) on one ordered `StreamEvent` stream.
- `Modify` is `Replace`; order handles stay open for late executions and fees
  until closed.
- Optional broker values use pointers where omitted differs from zero.
- Supported Gateway range `server_version` 200–225.

### Added

- Partial bracket placement returns `*OrderRecoveryError` with the IDs that
  reached IBKR.
- `Order.IncludeOvernight`.

### Migration

Imports move to `/v2`; read data and lifecycle from `Events()`; `Close()` is
a command and `Wait()` or `Err()` carries the outcome; `News().HistoricalAll`
is gone. Before-and-after examples:
[Migrating from v1 to v2](https://github.com/ThomasMarcelis/ibkr-go/blob/v2.0.0/docs/migration-v2.md).

### Known issues

Fixed in v2.0.1: a `-1` bar count could drop a historical update, and a
malformed broker message could let a partial snapshot look complete. Release
candidates rc.2 and rc.3 preceded this release; their notes are at those tags.

## v1 (deprecated)

v1 lives on the `github.com/ThomasMarcelis/ibkr-go` import path against
`server_version` 200. Full notes are at each tag.

- **v1.5.2** (2026-08-23): marks v1 deprecated in Go module metadata and directs users to v2.
- **v1.5.1** (2026-07-04): `Orders().RefreshOpen` resyncs an open-orders
  subscription ([#21](https://github.com/ThomasMarcelis/ibkr-go/issues/21)).
- **v1.5.0** (2026-07-04): `OpenOrderUpdate` is a union of `Order` and
  `Status`; `OpenOrder` lost `Filled` and `Remaining` (fill state comes from
  `OrderStatus`); status transitions for orders without a handle reach
  `SubscribeOpen` ([#20](https://github.com/ThomasMarcelis/ibkr-go/issues/20));
  the option pipeline, contract multiplier, execution times, and
  contract-bound order conditions were fixed against live captures; named
  `ErrCode` constants and `APIError` classifiers; `cmd/ibkr-doctor`.
- **v1.4.6** (2026-04-25): completed-order tail, `ReqExecutions` layout, and
  default-int order fields match the live wire.
- **v1.4.5** (2026-04-14): cancel requests carry CME tagging (minimum server
  version raised); the full advanced-order `Order` surface; `Events()` drains
  before `Done()`; code 202 is a cancellation notice, not an error.
- **v1.4.4**: `Subscription.Err()`; `Valid()` on historical vocabularies; TCP
  keepalive; reconnect keeps retrying with backoff.
- **v1.4.3**: `Retryable` on lifecycle events and `IsRetryable(err)`;
  asynchronous real-time-bar rejections are non-retryable.
- **v1.4.2**: completed-order and empty execution-snapshot decoding follow
  the live Gateway.
- **v1.4.1**: the historical range terminator (message 108) is decoded;
  fatal decoder errors close the socket.
- **v1.4.0**: `shopspring/decimal` replaces the custom `Decimal`; the package
  moved to the module root; `Client` split into domain facades; order
  management, market depth, option exercise, FA, WSH, and display groups
  added; persistent sessions no longer degrade under send-queue pressure
  ([#5](https://github.com/ThomasMarcelis/ibkr-go/issues/5)); historical
  request pacing.
- **v1.0.0**: initial release covering the read-only TWS API surface, with
  replay transcripts and fuzzing.

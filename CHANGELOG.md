# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

## Unreleased

### Changed (breaking)

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
  depth operation/side, fundamental report type, news provider code, and display
  group IDs.
- **Reworked historical and raw payload boundaries.** Historical bars use
  `HistoricalDuration` and `BarSize`; historical tick/news windows use
  `time.Time` (zero time means unset). Scanner, FA, and fundamental XML return `XMLDocument`;
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
- **Fundamental data**: `Contracts().FundamentalData` for Reuters XML reports.
- **Exercise options**: `Options().Exercise` fire-and-forget request.
- **FA configuration**: `Advisors().Config`, `Advisors().ReplaceConfig`,
  `Advisors().SoftDollarTiers`.
- **WSH calendar**: `WSH().MetaData`, `WSH().EventData`.
- **Display groups**: `TWS().DisplayGroups`, `TWS().SubscribeDisplayGroup`.
- **ParentID support** in OpenOrder for bracket and attached order tracking.
- Comprehensive GoDoc comments on all public types, methods, constants, and
  variables.
- Additional Example functions for pkg.go.dev.
- CHANGELOG.md.

### Fixed

- **`CommissionReport` decoding no longer fails on unset fields.** Live TWS
  emits the Java `Double.MAX_VALUE` sentinel for `Commission` and `RealizedPNL`
  when the server has not yet computed those values. The receive path now
  decodes both the sentinel and the empty-string form to a zero `Decimal`,
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
- **Exact Decimal type** for all prices and money.
- **Zero external dependencies**.
- **82 replay transcripts** from live IB Gateway captures for deterministic CI.
- **Fuzz testing** on wire protocol (frame parsing, field encoding, codec
  round-trips).

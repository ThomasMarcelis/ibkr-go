# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

## Unreleased

### Added

- **Order management**: PlaceOrder, CancelOrder, GlobalCancel with OrderHandle
  lifecycle tracking (Events, State, Done, Wait, Close, Cancel, Modify).
  Auto-closes on terminal status (Filled, Cancelled, Inactive).
- **Market depth (Level 2)**: SubscribeMarketDepth for full order book depth.
- **Fundamental data**: FundamentalData one-shot for Reuters XML reports.
- **Exercise options**: ExerciseOptions fire-and-forget request.
- **FA configuration**: RequestFA, ReplaceFA, SoftDollarTiers.
- **WSH calendar**: WSHMetaData, WSHEventData for Wall Street Horizon events.
- **Display groups**: QueryDisplayGroups, SubscribeDisplayGroup, UpdateDisplayGroup.
- **ParentID support** in OpenOrder for bracket and attached order tracking.
- Comprehensive GoDoc comments on all public types, methods, constants, and
  variables.
- 7 additional Example functions for pkg.go.dev (HistoricalBars, AccountSummary,
  PlaceOrder, PositionsSnapshot, QualifyContract, SubscribeRealTimeBars,
  SubscribeOpenOrders).
- CHANGELOG.md.

### Changed

- README rewritten: punchier opening, bullet-based Why section, consolidated
  per-library comparison table, removed inline API overview.
- Package overview (doc.go) expanded to cover all major patterns: connecting,
  one-shots, subscriptions, orders, session lifecycle, errors, financial types.
- Roadmap updated to reflect full API coverage.

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

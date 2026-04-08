# API Overview

Complete reference of the public `ibkr` package surface. Every method
listed here is verified against the current implementation.

For the public API contract — session state machine, subscription
lifecycle semantics, error taxonomy, and type guarantees — see
[`session-contract.md`](session-contract.md).

## Session

| Method | Returns | Description |
|--------|---------|-------------|
| `DialContext` | `*Client` | Connect and bootstrap a ready session |
| `Close` | `error` | Shut down the session |
| `Done` | `<-chan struct{}` | Closed when the session terminates |
| `Wait` | `error` | Block until session terminates, return final error |
| `Session` | `SessionSnapshot` | Current session state, managed accounts, server version |
| `SessionEvents` | `<-chan SessionEvent` | Observable session state changes |

## Account and Portfolio

| Method | Returns |
|--------|---------|
| `AccountSummary` / `SubscribeAccountSummary` | `[]AccountValue` / `*Subscription[AccountSummaryUpdate]` |
| `PositionsSnapshot` / `SubscribePositions` | `[]Position` / `*Subscription[PositionUpdate]` |
| `AccountUpdatesSnapshot` / `SubscribeAccountUpdates` | `[]AccountUpdate` / `*Subscription[AccountUpdate]` |
| `AccountUpdatesMultiSnapshot` / `SubscribeAccountUpdatesMulti` | `[]AccountUpdateMultiValue` / `*Subscription[AccountUpdateMultiValue]` |
| `PositionsMultiSnapshot` / `SubscribePositionsMulti` | `[]PositionMulti` / `*Subscription[PositionMulti]` |
| `SubscribePnL` / `SubscribePnLSingle` | `*Subscription[PnLUpdate]` / `*Subscription[PnLSingleUpdate]` |
| `FamilyCodes` | `[]FamilyCode` |
| `CompletedOrders` | `[]CompletedOrderResult` |

## Market Data

| Method | Returns |
|--------|---------|
| `QuoteSnapshot` / `SubscribeQuotes` | `Quote` / `*Subscription[QuoteUpdate]` |
| `SubscribeRealTimeBars` | `*Subscription[Bar]` |
| `SubscribeTickByTick` | `*Subscription[TickByTickData]` |
| `SubscribeHistoricalBars` | `*Subscription[Bar]` |
| `SubscribeMarketDepth` | `*Subscription[DepthRow]` |
| `SetMarketDataType` | `error` |

## Contract and Reference

| Method | Returns |
|--------|---------|
| `ContractDetails` | `[]ContractDetails` |
| `QualifyContract` | `QualifiedContract` |
| `MatchingSymbols` | `[]MatchingSymbol` |
| `MarketRule` | `MarketRuleResult` |
| `SecDefOptParams` | `[]SecDefOptParams` |
| `SmartComponents` | `[]SmartComponent` |
| `MktDepthExchanges` | `[]DepthExchange` |

## Historical Data

| Method | Returns |
|--------|---------|
| `HistoricalBars` | `[]Bar` |
| `HeadTimestamp` | `time.Time` |
| `HistogramData` | `[]HistogramEntry` |
| `HistoricalTicks` | `HistoricalTicksResult` |

## Options

| Method | Returns |
|--------|---------|
| `CalcImpliedVolatility` | `OptionComputation` |
| `CalcOptionPrice` | `OptionComputation` |

## News

| Method | Returns |
|--------|---------|
| `NewsProviders` | `[]NewsProvider` |
| `NewsArticle` | `NewsArticleResult` |
| `HistoricalNews` | `[]HistoricalNewsItemResult` |
| `SubscribeNewsBulletins` | `*Subscription[NewsBulletin]` |

## Scanner

| Method | Returns |
|--------|---------|
| `ScannerParameters` | `string` (XML) |
| `SubscribeScannerResults` | `*Subscription[[]ScannerResult]` |

## Order Management

| Method | Returns |
|--------|---------|
| `PlaceOrder` | `*OrderHandle` |
| `CancelOrder` | `error` |
| `GlobalCancel` | `error` |

## Orders and Executions (observation)

| Method | Returns |
|--------|---------|
| `OpenOrdersSnapshot` / `SubscribeOpenOrders` | `[]OpenOrder` / `*Subscription[OpenOrderUpdate]` |
| `Executions` / `SubscribeExecutions` | `[]ExecutionUpdate` / `*Subscription[ExecutionUpdate]` |

## Fundamental Data

| Method | Returns |
|--------|---------|
| `FundamentalData` | `string` (XML) |

## Exercise Options

| Method | Returns |
|--------|---------|
| `ExerciseOptions` | `error` |

## FA Configuration

| Method | Returns |
|--------|---------|
| `RequestFA` | `string` (XML) |
| `ReplaceFA` | `error` |
| `SoftDollarTiers` | `[]SoftDollarTier` |

## WSH Calendar

| Method | Returns |
|--------|---------|
| `WSHMetaData` | `string` (JSON) |
| `WSHEventData` | `string` (JSON) |

## Display Groups

| Method | Returns |
|--------|---------|
| `QueryDisplayGroups` | `string` |
| `SubscribeDisplayGroup` | `*DisplayGroupHandle` |

## Other

| Method | Returns |
|--------|---------|
| `UserInfo` | `string` |

# API Overview

Complete reference of the public root `ibkr` package surface. Every method
listed here is verified against the current implementation.

For lifecycle semantics, error taxonomy, and type guarantees, see
[`session-contract.md`](session-contract.md).

## Session

| Method | Returns | Description |
|--------|---------|-------------|
| `DialContext` | `*Client` | Connect and bootstrap a ready session |
| `Close` | `error` | Shut down the session |
| `Done` | `<-chan struct{}` | Closed when the session terminates |
| `Wait` | `error` | Block until session terminates, return final error |
| `Session` | `Snapshot` | Current session state, managed accounts, server version |
| `SessionEvents` | `<-chan Event` | Observable session state changes |

## Domain Accessors

| Accessor | Domain |
|----------|--------|
| `Accounts()` | account, portfolio, positions, PnL, family codes |
| `Contracts()` | contract details, qualification, reference data, fundamental XML |
| `MarketData()` | quotes, real-time bars, tick-by-tick, market depth, market data type |
| `History()` | historical bars, historical ticks, head timestamp, histogram |
| `Orders()` | order placement, cancellation, open/completed orders, executions query |
| `Options()` | option calculations and exercise |
| `News()` | providers, articles, historical headlines, bulletins |
| `Scanner()` | scanner parameters XML and scanner subscriptions |
| `Advisors()` | FA configuration XML and soft-dollar tiers |
| `WSH()` | Wall Street Horizon JSON metadata and event data |
| `TWS()` | user info and display groups |

## Account and Portfolio

| Method | Returns |
|--------|---------|
| `Accounts().Summary` / `SubscribeSummary` | `[]AccountValue` / `*Subscription[AccountSummaryUpdate]` |
| `Accounts().Positions` / `SubscribePositions` | `[]Position` / `*Subscription[PositionUpdate]` |
| `Accounts().Updates` / `SubscribeUpdates` | `[]AccountUpdate` / `*Subscription[AccountUpdate]` |
| `Accounts().UpdatesMulti` / `SubscribeUpdatesMulti` | `[]AccountUpdateMultiValue` / `*Subscription[AccountUpdateMultiValue]` |
| `Accounts().PositionsMulti` / `SubscribePositionsMulti` | `[]PositionMulti` / `*Subscription[PositionMulti]` |
| `Accounts().SubscribePnL` / `SubscribePnLSingle` | `*Subscription[PnLUpdate]` / `*Subscription[PnLSingleUpdate]` |
| `Accounts().FamilyCodes` | `[]FamilyCode` |

## Contracts and Reference

| Method | Returns |
|--------|---------|
| `Contracts().Details` | `[]ContractDetails` |
| `Contracts().Qualify` | `ContractDetails` |
| `Contracts().Search` | `[]MatchingSymbol` |
| `Contracts().MarketRule` | `MarketRuleResult` |
| `Contracts().SecDefOptParams` | `[]SecDefOptParams` |
| `Contracts().SmartComponents` | `[]SmartComponent` |
| `Contracts().DepthExchanges` | `[]DepthExchange` |
| `Contracts().FundamentalData` | `XMLDocument` |

## Market Data

| Method | Returns |
|--------|---------|
| `MarketData().Quote` / `SubscribeQuotes` | `Quote` / `*Subscription[QuoteUpdate]` |
| `MarketData().SubscribeRealTimeBars` | `*Subscription[Bar]` |
| `MarketData().SubscribeTickByTick` | `*Subscription[TickByTickData]` |
| `MarketData().SubscribeDepth` | `*Subscription[DepthRow]` |
| `MarketData().SetType` | `error` |

## Historical Data

| Method | Returns |
|--------|---------|
| `History().Bars` / `SubscribeBars` | `[]Bar` / `*Subscription[Bar]` |
| `History().HeadTimestamp` | `time.Time` |
| `History().Histogram` | `[]HistogramEntry` |
| `History().Ticks` | `HistoricalTicksResult` |

## Orders and Options

| Method | Returns |
|--------|---------|
| `Orders().Place` | `*OrderHandle` |
| `Orders().Cancel` / `CancelAll` | `error` |
| `Orders().Open` / `SubscribeOpen` | `[]OpenOrder` / `*Subscription[OpenOrderUpdate]` |
| `Orders().Completed` | `[]CompletedOrderResult` |
| `Orders().Executions` | `[]ExecutionUpdate` |
| `Options().ImpliedVolatility` | `OptionComputation` |
| `Options().Price` | `OptionComputation` |
| `Options().Exercise` | `error` |

## News, Scanner, Advisors, WSH, TWS

| Method | Returns |
|--------|---------|
| `News().Providers` | `[]NewsProvider` |
| `News().Article` | `NewsArticle` |
| `News().Historical` | `[]HistoricalNewsItem` |
| `News().SubscribeBulletins` | `*Subscription[NewsBulletin]` |
| `Scanner().Parameters` | `XMLDocument` |
| `Scanner().SubscribeResults` | `*Subscription[[]ScannerResult]` |
| `Advisors().Config` / `ReplaceConfig` | `XMLDocument` / `error` |
| `Advisors().SoftDollarTiers` | `[]SoftDollarTier` |
| `WSH().MetaData` / `EventData` | `JSONDocument` |
| `TWS().UserInfo` | `string` |
| `TWS().DisplayGroups` | `[]DisplayGroupID` |
| `TWS().SubscribeDisplayGroup` | `*DisplayGroupHandle` |

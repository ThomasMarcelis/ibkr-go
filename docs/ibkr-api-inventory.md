# IBKR API Inventory

This inventory is the source list used to keep
[`live-coverage-matrix.md`](live-coverage-matrix.md) MECE. It separates facts
from decisions:

- Official TWS API surface: what IBKR exposes through EClient/EWrapper.
- ibkr-go surface: what this repo currently implements.
- Verification surface: executable capture scenarios and replay transcripts.

The matrix may decide a capability is implemented, deferred, blocked, or out of
scope, but every current repo behavior must appear here and in the matrix.

## Official Sources

| Source | What It Contributes |
|--------|---------------------|
| [IBKR API Software](https://interactivebrokers.github.io/) | Latest downloadable API package, current release, license, and recommended TWS/IB Gateway version. As of 2026-04-11 it lists API 10.45, released 2026-03-30, and recommends TWS/IB Gateway 1037 or higher for comprehensive feature support. |
| [IBKR Campus TWS API docs](https://www.interactivebrokers.com/campus/ibkr-api-page/twsapi-doc/) | Current documentation hub and warning that the official API source is distributed through IBKR's MSI/ZIP package, not package registries. |
| [EClientSocket reference](https://interactivebrokers.github.io/tws-api/classIBApi_1_1EClientSocket.html) | Official client request/control method inventory. The page states this client class contains methods used to communicate with TWS/Gateway. |
| [EWrapper reference](https://interactivebrokers.github.io/tws-api/interfaceIBApi_1_1EWrapper.html) | Official callback/event inventory. The page states almost every EClientSocket call results in at least one EWrapper event. |
| [Class function index](https://interactivebrokers.github.io/tws-api/functions_func.html) | Cross-check for methods and callbacks that are easy to miss in topic pages. |
| [Historical bars](https://interactivebrokers.github.io/tws-api/historical_bars.html) | Historical bars, keep-up-to-date updates, schedule behavior, time-zone behavior, pacing and bust-event notes. |
| [Market data receiving](https://interactivebrokers.github.io/tws-api/md_receive.html) | Market data request, delayed/frozen modes, tick callbacks, and snapshot behavior. |
| [Tick-by-tick data](https://interactivebrokers.github.io/tws-api/tick_data.html) | Last, AllLast, BidAsk, and MidPoint tick-by-tick stream families and request limits. |
| [Market depth](https://interactivebrokers.github.io/tws-api/market_depth.html) | L2 depth, smart depth, depth-exchange metadata, and subscription limit behavior. |
| [Orders overview](https://interactivebrokers.github.io/tws-api/orders.html) | Broad order capability claim and date/time field format changes. |
| [Basic orders](https://interactivebrokers.github.io/tws-api/basic_orders.html) | Order-type families, required fields, and product applicability. |
| [Advanced orders and algos](https://interactivebrokers.github.io/tws-api/advanced_orders.html) | IB algo and advanced order patterns. |
| [Bracket orders](https://interactivebrokers.github.io/tws-api/bracket_order.html) | Parent/child/transmit sequencing. |
| [OCA](https://interactivebrokers.github.io/tws-api/oca.html) | One-cancels-all group behavior. |
| [Order conditions](https://interactivebrokers.github.io/tws-api/order_conditions.html) | Price, time, margin, execution, volume, and percent-change condition families. |
| [Hedging](https://interactivebrokers.github.io/tws-api/hedging.html) | Attached hedge order families. |
| [Scanner](https://interactivebrokers.github.io/tws-api/market_scanners.html) | Scanner parameters, scanner subscription fields, and scanner filter options. |
| [Account updates](https://interactivebrokers.github.io/tws-api/account_updates.html) | Account update timing, one-account subscription behavior, and account value vocabulary. |

The downloadable official API package is the implementation source of truth
after accepting IBKR's license. Do not vendor or redistribute it. For this repo,
only derived method/callback names and public behavioral notes should be
committed.

## Official EClient Request And Control Methods

| Group | Official Methods | ibkr-go Status |
|-------|------------------|----------------|
| Connection/session | `eConnect`, `eDisconnect`, `startApi`, `Close`, `IsConnected`, `SetConnectOptions`, `redirect`, `DisableUseV100Plus`, `reqCurrentTime`, `reqIds`, `reqManagedAccts`, `setServerLogLevel` | Connect/start/close implemented through `DialContext` and lifecycle APIs. `reqCurrentTime` is implemented as `Client.CurrentTime`; explicit `reqIds` is captured but has no public facade yet. `reqManagedAccts`, server log level, redirect, and old connection toggles are matrix targets or explicit non-goals. |
| Verification/internal auth | `verifyRequest`, `verifyMessage`, `verifyAndAuthRequest`, `verifyAndAuthMessage` | Officially internal-purpose. Matrix as out of public scope unless live Gateway emits callbacks. |
| Market data L1 | `reqMktData`, `cancelMktData`, `reqMarketDataType` | Implemented. Needs MECE coverage by live, frozen, delayed, delayed-frozen, generic tick families, snapshot, stream, and cancel. |
| Tick-by-tick | `reqTickByTickData`, `cancelTickByTickData` | Implemented. Needs distinct Last, AllLast, BidAsk, MidPoint rows. |
| Real-time and historical bars | `reqRealTimeBars`, `cancelRealTimeBars`, `reqHistoricalData`, `cancelHistoricalData`, `reqHeadTimestamp`, `cancelHeadTimestamp`, `reqHistogramData`, `cancelHistogramData`, `reqHistoricalTicks` | Implemented, including historical schedule support through `History().Schedule`. Needs separate rows for keep-up updates, schedule, time zones, and pacing/errors. |
| Market depth | `reqMarketDepth`, `cancelMktDepth`, `reqMktDepthExchanges` | Implemented. Needs regular depth, L2, smart depth, entitlement error, cancel, and depth metadata rows. |
| Contracts/reference | `reqContractDetails`, `reqMatchingSymbols`, `reqSecDefOptParams`, `reqSmartComponents`, `reqMarketRule` | Implemented. Needs asset-class and ambiguity/error rows. |
| Accounts/portfolio | `reqAccountSummary`, `cancelAccountSummary`, `reqAccountUpdates`, `reqPositions`, `cancelPositions`, `reqPositionsMulti`, `cancelPositionsMulti`, `reqAccountUpdatesMulti`, `cancelAccountUpdatesMulti`, `reqFamilyCodes`, `reqPnL`, `cancelPnL`, `reqPnLSingle`, `cancelPnLSingle` | Implemented. Needs account/model/concurrent/streaming/trade-interaction rows. |
| Orders/executions | `placeOrder`, `cancelOrder`, `reqGlobalCancel`, `reqOpenOrders`, `reqAllOpenOrders`, `reqAutoOpenOrders`, `reqCompletedOrders`, `reqExecutions` | Implemented with deferred rare order sections. Needs explicit direct cancel, open-order scopes, completed-order details, execution filters, commission ordering, and advanced order branches. |
| Options | `calculateImpliedVolatility`, `cancelCalculateImpliedVolatility`, `calculateOptionPrice`, `cancelCalculateOptionPrice`, `exerciseOptions` | Calc implemented. Exercise implemented fire-and-forget but needs live target rows. |
| News | `reqNewsProviders`, `reqNewsBulletins`, `cancelNewsBulletins`, `reqNewsArticle`, `reqHistoricalNews` | Implemented. `News().Article` lacks executable capture scenario. |
| Scanner | `reqScannerParameters`, `reqScannerSubscription`, `cancelScannerSubscription` | Implemented. Needs scanner filter-options rows beyond legacy core fields. |
| FA/advisor | `requestFA`, `replaceFA`, `reqSoftDollarTiers` | Implemented. `replaceFA` lacks executable capture scenario and should usually freeze non-FA error or read-back/restore behavior. |
| WSH | `reqWshMetaData`, `cancelWshMetaData`, `reqWshEventData`, `cancelWshEventData` | Implemented. Needs metadata, event, cancel, filter/date/portfolio/watchlist variants. |
| Display groups/TWS | `queryDisplayGroups`, `subscribeToGroupEvents`, `updateDisplayGroup`, `unsubscribeFromGroupEvents`, `reqUserInfo` | Implemented. Needs invalid group/update cases and TWS vs Gateway differences. |
| Fundamental data | `reqFundamentalData`, `cancelFundamentalData` | Implemented though official docs mark it legacy/deprecated. Needs every report type plus entitlement/error rows. |

## Official EWrapper Callback Families

| Group | Official Callbacks | ibkr-go Status |
|-------|--------------------|----------------|
| Errors/session | `error`, `connectionClosed`, `currentTime`, `nextValidId`, `managedAccounts` | Error/managed/next valid/current time implemented. `connectionClosed` still needs an explicit matrix row. |
| Market data L1 | `tickPrice`, `tickSize`, `tickString`, `tickGeneric`, `tickEFP`, `tickOptionComputation`, `tickSnapshotEnd`, `marketDataType`, `tickReqParams`, `tickNews` | Most implemented. `tickEFP` and `tickNews` are official callbacks not currently represented as implemented message IDs. |
| Tick-by-tick | `tickByTickAllLast`, `tickByTickBidAsk`, `tickByTickMidPoint` | Implemented through unified tick-by-tick decode. Needs separate verification rows. |
| Contracts/reference | `contractDetails`, `bondContractDetails`, `contractDetailsEnd`, `symbolSamples`, `securityDefinitionOptionParameter`, `securityDefinitionOptionParameterEnd`, `smartComponents`, `marketRule`, `mktDepthExchanges` | Implemented except bond-specific callback is represented through generic contract details only if live wire confirms same path. Needs explicit bond row. |
| Historical | `historicalData`, `historicalDataEnd`, `historicalDataUpdate`, `historicalSchedule`, `headTimestamp`, `histogramData`, `historicalTicks`, `historicalTicksBidAsk`, `historicalTicksLast`, `historicalNews`, `historicalNewsEnd` | Implemented. |
| Accounts/portfolio | `accountSummary`, `accountSummaryEnd`, `updateAccountValue`, `updatePortfolio`, `updateAccountTime`, `accountDownloadEnd`, `position`, `positionEnd`, `positionMulti`, `positionMultiEnd`, `accountUpdateMulti`, `accountUpdateMultiEnd`, `pnl`, `pnlSingle`, `familyCodes` | Implemented. Needs richer live scenarios. |
| Orders/executions | `openOrder`, `openOrderEnd`, `orderStatus`, `execDetails`, `execDetailsEnd`, `commissionReport`, `completedOrder`, `completedOrdersEnd`, `orderBound` | Implemented except `orderBound` not represented as a message/callback. Completed order details are simplified. |
| Market depth | `updateMktDepth`, `updateMktDepthL2` | Implemented. Needs success plus entitlement captures. |
| News/scanner | `newsProviders`, `newsArticle`, `updateNewsBulletin`, `scannerParameters`, `scannerData`, `scannerDataEnd` | Implemented. News article lacks executable capture scenario. |
| FA/WSH/display | `receiveFA`, `replaceFAEnd`, `softDollarTiers`, `wshMetaData`, `wshEventData`, `displayGroupList`, `displayGroupUpdated` | Implemented except `replaceFAEnd` not decoded/exposed. |
| Verification/reroute | `verifyMessageAPI`, `verifyCompleted`, `verifyAndAuthMessageAPI`, `verifyAndAuthCompleted`, `connectAck`, `rerouteMktDataReq`, `rerouteMktDepthReq`, `deltaNeutralValidation` | Official callbacks not in current implemented message inventory; matrix as target/deferred/out-of-public-scope based on live behavior and project scope. |

## Current ibkr-go Public Facade Methods

| Facade | Public Methods |
|--------|----------------|
| `Client` | `Close`, `Done`, `Wait`, `Session`, `SessionEvents`, `CurrentTime`, `Accounts`, `Contracts`, `MarketData`, `History`, `Orders`, `Options`, `News`, `Scanner`, `Advisors`, `WSH`, `TWS` |
| `Accounts()` | `Summary`, `SubscribeSummary`, `Positions`, `SubscribePositions`, `Updates`, `SubscribeUpdates`, `UpdatesMulti`, `SubscribeUpdatesMulti`, `PositionsMulti`, `SubscribePositionsMulti`, `SubscribePnL`, `SubscribePnLSingle`, `FamilyCodes` |
| `Contracts()` | `Details`, `Qualify`, `Search`, `MarketRule`, `SecDefOptParams`, `SmartComponents`, `DepthExchanges`, `FundamentalData` |
| `MarketData()` | `SetType`, `Quote`, `SubscribeQuotes`, `SubscribeRealTimeBars`, `SubscribeTickByTick`, `SubscribeDepth` |
| `History()` | `Bars`, `SubscribeBars`, `HeadTimestamp`, `Histogram`, `Ticks`, `Schedule` |
| `Orders()` | `Place`, `Cancel`, `CancelAll`, `Open`, `SubscribeOpen`, `Completed`, `Executions` |
| `Options()` | `ImpliedVolatility`, `Price`, `Exercise` |
| `News()` | `Providers`, `Article`, `Historical`, `SubscribeBulletins` |
| `Scanner()` | `Parameters`, `SubscribeResults` |
| `Advisors()` | `Config`, `ReplaceConfig`, `SoftDollarTiers` |
| `WSH()` | `MetaData`, `EventData` |
| `TWS()` | `UserInfo`, `DisplayGroups`, `SubscribeDisplayGroup` |

## Current Codec Message Inventory

Outbound message IDs:

| Constant | ID | Matrix Capability |
|----------|----|-------------------|
| `OutReqMktData` | 1 | Market data L1 |
| `OutCancelMktData` | 2 | Market data L1 cancel |
| `OutPlaceOrder` | 3 | Orders |
| `OutCancelOrder` | 4 | Orders cancel |
| `OutReqOpenOrders` | 5 | Open orders |
| `OutReqAccountUpdates` | 6 | Account updates |
| `OutReqExecutions` | 7 | Executions |
| `OutReqContractData` | 9 | Contract details |
| `OutReqMktDepth` | 10 | Market depth |
| `OutCancelMktDepth` | 11 | Market depth cancel |
| `OutReqNewsBulletins` | 12 | News bulletins |
| `OutCancelNewsBulletins` | 13 | News bulletins cancel |
| `OutReqAutoOpenOrders` | 15 | Open orders auto-bind |
| `OutReqAllOpenOrders` | 16 | Open orders all |
| `OutRequestFA` | 18 | FA config |
| `OutReplaceFA` | 19 | FA replace config |
| `OutReqHistoricalData` | 20 | Historical bars/schedule |
| `OutExerciseOptions` | 21 | Option exercise |
| `OutReqScannerSubscription` | 22 | Scanner subscription |
| `OutCancelScannerSubscription` | 23 | Scanner cancel |
| `OutReqScannerParameters` | 24 | Scanner parameters |
| `OutCancelHistoricalData` | 25 | Historical bars cancel |
| `OutReqRealTimeBars` | 50 | Real-time bars |
| `OutCancelRealTimeBars` | 51 | Real-time bars cancel |
| `OutReqFundamentalData` | 52 | Fundamental data |
| `OutCancelFundamentalData` | 53 | Fundamental data cancel |
| `OutReqCalcImpliedVolatility` | 54 | Option calculation |
| `OutReqCalcOptionPrice` | 55 | Option calculation |
| `OutCancelCalcImpliedVolatility` | 56 | Option calculation cancel |
| `OutCancelCalcOptionPrice` | 57 | Option calculation cancel |
| `OutReqGlobalCancel` | 58 | Global cancel |
| `OutReqMarketDataType` | 59 | Market data type |
| `OutReqPositions` | 61 | Positions |
| `OutReqAccountSummary` | 62 | Account summary |
| `OutCancelAccountSummary` | 63 | Account summary cancel |
| `OutCancelPositions` | 64 | Positions cancel |
| `OutQueryDisplayGroups` | 67 | Display groups |
| `OutSubscribeToGroupEvents` | 68 | Display group subscription |
| `OutUpdateDisplayGroup` | 69 | Display group update |
| `OutUnsubscribeFromGroupEvents` | 70 | Display group unsubscribe |
| `OutStartAPI` | 71 | Session bootstrap |
| `OutReqPositionsMulti` | 74 | Positions multi |
| `OutCancelPositionsMulti` | 75 | Positions multi cancel |
| `OutReqAccountUpdatesMulti` | 76 | Account updates multi |
| `OutCancelAccountUpdatesMulti` | 77 | Account updates multi cancel |
| `OutReqSecDefOptParams` | 78 | Sec-def option params |
| `OutReqSoftDollarTiers` | 79 | Soft-dollar tiers |
| `OutReqFamilyCodes` | 80 | Family codes |
| `OutReqMatchingSymbols` | 81 | Matching symbols |
| `OutReqMktDepthExchanges` | 82 | Depth exchanges |
| `OutReqSmartComponents` | 83 | Smart components |
| `OutReqNewsArticle` | 84 | News article |
| `OutReqNewsProviders` | 85 | News providers |
| `OutReqHistoricalNews` | 86 | Historical news |
| `OutReqHeadTimestamp` | 87 | Head timestamp |
| `OutReqHistogramData` | 88 | Histogram data |
| `OutCancelHistogramData` | 89 | Histogram cancel |
| `OutCancelHeadTimestamp` | 90 | Head timestamp cancel |
| `OutReqMarketRule` | 91 | Market rule |
| `OutReqPnL` | 92 | Account PnL |
| `OutCancelPnL` | 93 | Account PnL cancel |
| `OutReqPnLSingle` | 94 | Single-position PnL |
| `OutCancelPnLSingle` | 95 | Single-position PnL cancel |
| `OutReqHistoricalTicks` | 96 | Historical ticks |
| `OutReqTickByTickData` | 97 | Tick-by-tick |
| `OutCancelTickByTickData` | 98 | Tick-by-tick cancel |
| `OutReqCompletedOrders` | 99 | Completed orders |
| `OutReqWSHMetaData` | 100 | WSH metadata |
| `OutCancelWSHMetaData` | 101 | WSH metadata cancel |
| `OutReqWSHEventData` | 102 | WSH event data |
| `OutCancelWSHEventData` | 103 | WSH event data cancel |
| `OutReqUserInfo` | 104 | User info |
| `OutReqIds` | 8 | Explicit next-valid-ID refresh |
| `OutReqCurrentTime` | 49 | Server wall-clock time request |

Inbound message IDs:

| Constant | ID | Matrix Capability |
|----------|----|-------------------|
| `InTickPrice` | 1 | Market data tick price |
| `InTickSize` | 2 | Market data tick size |
| `InOrderStatus` | 3 | Order status |
| `InErrMsg` | 4 | API errors/status codes |
| `InOpenOrder` | 5 | Open order |
| `InUpdateAccountValue` | 6 | Account updates |
| `InUpdatePortfolio` | 7 | Portfolio updates |
| `InUpdateAccountTime` | 8 | Account update time |
| `InNextValidID` | 9 | Next valid ID |
| `InContractData` | 10 | Contract details |
| `InExecutionData` | 11 | Executions |
| `InMarketDepth` | 12 | Market depth |
| `InMarketDepthL2` | 13 | Market depth L2 |
| `InNewsBulletins` | 14 | News bulletins |
| `InManagedAccounts` | 15 | Managed accounts |
| `InReceiveFA` | 16 | FA config |
| `InHistoricalData` | 17 | Historical bars |
| `InScannerParameters` | 19 | Scanner parameters |
| `InScannerData` | 20 | Scanner data |
| `InTickOptionComputation` | 21 | Option computation |
| `InTickGeneric` | 45 | Market data generic tick |
| `InTickString` | 46 | Market data string tick |
| `InCurrentTime` | 49 | Current time |
| `InRealTimeBars` | 50 | Real-time bars |
| `InFundamentalData` | 51 | Fundamental data |
| `InContractDataEnd` | 52 | Contract details end |
| `InOpenOrderEnd` | 53 | Open order end |
| `InAccountDownloadEnd` | 54 | Account download end |
| `InExecutionDataEnd` | 55 | Executions end |
| `InTickSnapshotEnd` | 57 | Market data snapshot end |
| `InMarketDataType` | 58 | Market data type |
| `InCommissionReport` | 59 | Commission report |
| `InPositionData` | 61 | Positions |
| `InPositionEnd` | 62 | Positions end |
| `InAccountSummary` | 63 | Account summary |
| `InAccountSummaryEnd` | 64 | Account summary end |
| `InDisplayGroupList` | 67 | Display groups |
| `InDisplayGroupUpdated` | 68 | Display group updates |
| `InPositionMulti` | 71 | Positions multi |
| `InPositionMultiEnd` | 72 | Positions multi end |
| `InAccountUpdateMulti` | 73 | Account updates multi |
| `InAccountUpdateMultiEnd` | 74 | Account updates multi end |
| `InSecDefOptParams` | 75 | Sec-def option params |
| `InSecDefOptParamsEnd` | 76 | Sec-def option params end |
| `InSoftDollarTiers` | 77 | Soft-dollar tiers |
| `InFamilyCodes` | 78 | Family codes |
| `InSymbolSamples` | 79 | Matching symbols |
| `InMktDepthExchanges` | 80 | Depth exchanges |
| `InTickReqParams` | 81 | Tick request params |
| `InSmartComponents` | 82 | Smart components |
| `InNewsArticle` | 83 | News article |
| `InNewsProviders` | 85 | News providers |
| `InHistoricalNews` | 86 | Historical news |
| `InHistoricalNewsEnd` | 87 | Historical news end |
| `InHeadTimestamp` | 88 | Head timestamp |
| `InHistogramData` | 89 | Histogram data |
| `InMarketRule` | 92 | Market rule |
| `InPnL` | 94 | Account PnL |
| `InPnLSingle` | 95 | Single-position PnL |
| `InHistoricalTicks` | 96 | Historical midpoint ticks |
| `InHistoricalTicksBidAsk` | 97 | Historical bid/ask ticks |
| `InHistoricalTicksLast` | 98 | Historical last ticks |
| `InTickByTick` | 99 | Tick-by-tick |
| `InCompletedOrder` | 101 | Completed order |
| `InCompletedOrderEnd` | 102 | Completed orders end |
| `InUserInfo` | 103 | User info |
| `InWSHMetaData` | 104 | WSH metadata |
| `InWSHEventData` | 105 | WSH event data |
| `InHistoricalSchedule` | 106 | Historical schedule (whatToShow=SCHEDULE) |
| `InHistoricalDataUpdate` | 108 | Historical bar updates |

## Known Official Gaps Or Deferred Branches

These are not all defects. They are explicit matrix targets until live evidence
and project scope decide whether to implement, defer, or mark out of scope.

- `reqIds` public facade, `reqManagedAccts`, `setServerLogLevel`.
- Verification/auth callbacks and redirect/reroute callbacks.
- `tickEFP`, `tickNews`, and `deltaNeutralValidation`.
- `bondContractDetails` as a distinct callback shape.
- `orderBound`, completed-order full detail extraction, and rare OpenOrder
  branches.
- `replaceFAEnd`.
- Hedge, scale, delta-neutral, pegged, adjusted, FA allocation, MiFID/manual
  order, soft-dollar-on-order, and advanced-reject override order branches.

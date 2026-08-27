# IBKR API Inventory

This inventory is the source list used to keep
[`live-coverage-matrix.md`](live-coverage-matrix.md) MECE. It separates facts
from decisions:

- Official TWS API surface: what IBKR exposes through EClient/EWrapper.
- ibkr-go surface: what this repo currently implements.
- Verification surface: executable capture scenarios and replay transcripts.

The matrix may decide a capability is implemented, deferred, blocked, or out of
scope, but every current repo behavior must appear here and in the matrix.
Numeric message identities and negotiated-version gates are owned by
`internal/protocol`; the tables below are descriptive rather than an
independent protocol registry.

## Official Sources

| Source | What It Contributes |
|--------|---------------------|
| [IBKR API Software](https://interactivebrokers.github.io/) | Latest downloadable API package, current release, license, and recommended TWS/IB Gateway version. As of 2026-07-09 it lists Latest API 10.48, released 2026-07-07, Stable API 10.45, released 2026-03-30, and recommends TWS or IB Gateway 1045 or higher for comprehensive feature support. |
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
| Connection/session | `eConnect`, `eDisconnect`, `startApi`, `Close`, `IsConnected`, `SetConnectOptions`, `redirect`, `DisableUseV100Plus`, `reqCurrentTime`, `reqIds`, `reqManagedAccts`, `setServerLogLevel` | Connect/start/close are implemented through `DialContext` and lifecycle APIs. `reqCurrentTime` is `Client.CurrentTime`; `reqIds` is `Orders().RefreshOrderID`; `reqManagedAccts` is `Client.ManagedAccounts`. Server log level, redirect, and old connection toggles are explicit non-goals. |
| Verification/internal auth | `verifyRequest`, `verifyMessage`, `verifyAndAuthRequest`, `verifyAndAuthMessage` | Officially internal-purpose. Matrix as out of public scope unless live Gateway emits callbacks. |
| Market data L1 | `reqMktData`, `cancelMktData`, `reqMarketDataType` | Implemented. Quote streams preserve every price/size callback, including unmapped numeric tick types, price attributes, and optional companion size, while also delivering normalized fields plus generic, string, request-parameter, option-computation, EFP, delta-neutral validation, and contract-specific news callbacks. Generic tick 787 projects odd-lot tick IDs 105-110; positive entitled values remain a live-evidence target. |
| Tick-by-tick | `reqTickByTickData`, `cancelTickByTickData` | Implemented. Needs distinct Last, AllLast, BidAsk, MidPoint rows. |
| Real-time and historical bars | `reqRealTimeBars`, `cancelRealTimeBars`, `reqHistoricalData`, `cancelHistoricalData`, `reqHeadTimestamp`, `cancelHeadTimestamp`, `reqHistogramData`, `cancelHistogramData`, `reqHistoricalTicks` | Implemented, including historical schedule support through `History().Schedule`. Needs separate rows for keep-up updates, schedule, time zones, and pacing/errors. |
| Market depth | `reqMarketDepth`, `cancelMktDepth`, `reqMktDepthExchanges` | Implemented. Needs regular depth, L2, smart depth, entitlement error, cancel, and depth metadata rows. |
| Contracts/reference | `reqContractDetails`, `reqMatchingSymbols`, `reqSecDefOptParams`, `reqSmartComponents`, `reqMarketRule` | Implemented. `Contract` owns include-expired/external-ID selectors and BAG/delta-neutral composition; nil versus explicit zero is preserved for strike and leg exempt code. Current sv225 protobuf captures freeze stock, bond, fund, option, issuer, BAG, and selector shapes without adding unsupported fields. Bond issuer lookup and the distinct message-18 bond details shape are live-attested; empty coupon/maturity/rating fields remain unset rather than inferred. Positive BAG delta-neutral validation remains open. |
| Accounts/portfolio | `reqAccountSummary`, `cancelAccountSummary`, `reqAccountUpdates`, `reqPositions`, `cancelPositions`, `reqPositionsMulti`, `cancelPositionsMulti`, `reqAccountUpdatesMulti`, `cancelAccountUpdatesMulti`, `reqFamilyCodes`, `reqPnL`, `cancelPnL`, `reqPnLSingle`, `cancelPnLSingle` | Implemented, including distinct account-summary group selection and local result filtering. Needs FA named-group, account/model, concurrent, streaming, and trade-interaction rows. |
| Orders/executions | `placeOrder`, `cancelOrder`, `reqGlobalCancel`, `reqOpenOrders`, `reqAllOpenOrders`, `reqAutoOpenOrders`, `reqCompletedOrders`, `reqExecutions` | Implemented. Open and completed orders return BAG legs and delta-neutral composition on their canonical `Contract`; the shared order-level `OrderCombo` contains only prices and routing. Current sv225 completed-order, execution, and commission-and-fees results are projected. `SubscribeExecutionEvents` passively exposes every execution and fee callback without issuing a query or discarding unmatched fees. Current campaigns cover common and advanced order fields; attached preset-order metadata remains internal pending a native paper-order lifecycle. Nondefault execution filters, rare advanced-order branches, and meaningful bond yield/redemption still need live attestation. |
| Options | `calculateImpliedVolatility`, `cancelCalculateImpliedVolatility`, `calculateOptionPrice`, `cancelCalculateOptionPrice`, `exerciseOptions` | Implemented, but not fully promoted. Exact sv210 classic and sv211 protobuf calculation captures and public replay cover successful price and implied-volatility results; cancellation-before-first-result remains open. The guarded sv225 exercise campaign bought one qualified ITM AAPL call; exact warning 10349 preceded a `PreSubmitted` exercise instruction, but no terminal status arrived. Cleanup sold only the campaign delta. Later fenced cleanup removed the 2133 exclusion and a direct cancel returned code 10147 because the order was no longer found; fresh position, execution/fee, and ordinary-open-order snapshots reconcile to baseline. Terminal exercise, lapse, override, and clearing settlement remain matrix work. |
| News | `reqNewsProviders`, `reqNewsBulletins`, `cancelNewsBulletins`, `reqNewsArticle`, `reqHistoricalNews` | Implemented. `api_news_article_aapl` captures the article follow-up path from a historical-news result; invalid article/provider variants remain matrix work. |
| Scanner | `reqScannerParameters`, `reqScannerSubscription`, `cancelScannerSubscription` | Implemented across the sv210 protobuf migration, including every supported field and both generic option lists. A current sv225 public request, ten-row `scannerData` response, and clean cancel are replay-promoted. Additional rejection variants remain matrix work. |
| FA/advisor | `requestFA`, `reqSoftDollarTiers` | Implemented. Mutating FA configuration is outside the library charter. |
| WSH | `reqWshMetaData`, `cancelWshMetaData`, `reqWshEventData`, `cancelWshEventData` | Implemented. Needs metadata, event, cancel, filter/date/portfolio/watchlist variants. |
| Display groups/TWS | `queryDisplayGroups`, `subscribeToGroupEvents`, `updateDisplayGroup`, `unsubscribeFromGroupEvents`, `reqUserInfo`, `reqConfig` | Implemented, including presence-aware read-only `TWS().Config` at sv219. Configuration mutation is intentionally not exposed. Needs invalid group/update cases and TWS vs Gateway differences. |

Historical note: IBKR API 10.47 removed `reqFundamentalData`,
`cancelFundamentalData`, the matching callback, and fundamental-ratios tick
type 47. They are intentionally absent from the current official and ibkr-go
surface inventories. Earlier live captures are retained only as historical
evidence; WSH is a separate API, not a replacement.

## Official EWrapper Callback Families

| Group | Official Callbacks | ibkr-go Status |
|-------|--------------------|----------------|
| Errors/session | `error`, `connectionClosed`, `currentTime`, `nextValidId`, `managedAccounts` | Error/managed/next valid/current time implemented. `connectionClosed` still needs an explicit matrix row. |
| Market data L1 | `tickPrice`, `tickSize`, `tickString`, `tickGeneric`, `tickEFP`, `tickOptionComputation`, `tickSnapshotEnd`, `marketDataType`, `tickReqParams`, `tickNews` | All are implemented as typed quote updates. `tickEFP` is official-layout frozen but awaits a positive entitled callback. `tickNews` is inbound message 84 and is delivered as `QuoteUpdateNewsTick`. |
| Tick-by-tick | `tickByTickAllLast`, `tickByTickBidAsk`, `tickByTickMidPoint` | Implemented through unified tick-by-tick decode. Needs separate verification rows. |
| Contracts/reference | `contractDetails`, `bondContractDetails`, `contractDetailsEnd`, `symbolSamples`, `securityDefinitionOptionParameter`, `securityDefinitionOptionParameterEnd`, `smartComponents`, `marketRule`, `mktDepthExchanges` | Current protobuf stock/bond/fund/option shapes are implemented. `bondContractDetails` is projected through `ContractDetails.Bond`; tagged CUSIP/ISIN values, size rules, algorithmic minimum, precision fields, and real ineligibility reasons are preserved without inference. |
| Historical | `historicalData`, `historicalDataEnd`, `historicalDataUpdate`, `historicalSchedule`, `headTimestamp`, `histogramData`, `historicalTicks`, `historicalTicksBidAsk`, `historicalTicksLast`, `historicalNews`, `historicalNewsEnd` | Implemented. |
| Accounts/portfolio | `accountSummary`, `accountSummaryEnd`, `updateAccountValue`, `updatePortfolio`, `updateAccountTime`, `accountDownloadEnd`, `position`, `positionEnd`, `positionMulti`, `positionMultiEnd`, `accountUpdateMulti`, `accountUpdateMultiEnd`, `pnl`, `pnlSingle`, `familyCodes` | Implemented. Needs richer live scenarios. |
| Orders/executions | `openOrder`, `openOrderEnd`, `orderStatus`, `execDetails`, `execDetailsEnd`, `commissionAndFeesReport`, `completedOrder`, `completedOrdersEnd`, `orderBound` | Implemented with protobuf throughout the supported range. `orderBound` is delivered as `OpenOrderUpdate.Binding` only on the client-0 auto-open-orders scope; a raw paper-TWS capture is still required before calling it live-attested. Current sv225 open-order, status, completion, execution, and fee callbacks preserve their public identities and observed values. Advanced branches and meaningful bond yield/redemption remain explicitly unattested. |
| Market depth | `updateMktDepth`, `updateMktDepthL2` | Implemented. Needs success plus entitlement captures. |
| News/scanner | `newsProviders`, `newsArticle`, `updateNewsBulletin`, `scannerParameters`, `scannerData`, `scannerDataEnd` | Implemented. News article has live capture coverage through `api_news_article_aapl`; invalid article/provider variants remain matrix work. |
| FA/WSH/display | `receiveFA`, `softDollarTiers`, `wshMetaData`, `wshEventData`, `displayGroupList`, `displayGroupUpdated` | Implemented. |
| Verification/reroute | `verifyMessageAPI`, `verifyCompleted`, `verifyAndAuthMessageAPI`, `verifyAndAuthCompleted`, `connectAck`, `rerouteMktDataReq`, `rerouteMktDepthReq`, `deltaNeutralValidation` | Both current protobuf reroute callbacks are implemented and transparently replace active quote/depth requests. The current sv225 IBM CFD sequence proves the initial request, market-data reroute, conID request, delayed-data notice, and positive quote callbacks. Delta-neutral validation is decoded and delivered through the requesting quote subscription; positive BAG and market-depth reroute evidence remain pending. Verification/auth and connectAck remain outside the implemented inventory. |

## Current ibkr-go Public Surface

The generated [package reference](https://pkg.go.dev/github.com/ThomasMarcelis/ibkr-go/v2)
is authoritative for Go methods and types. The facade overview in the project
README is the shorter usage guide; this inventory tracks the official IBKR
request and callback surface rather than duplicating generated API listings.

## Current Negotiated Message-ID Inventory

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
| `OutReqIds` | 8 | Explicit next-valid-ID refresh |
| `OutReqContractData` | 9 | Contract details |
| `OutReqMktDepth` | 10 | Market depth |
| `OutCancelMktDepth` | 11 | Market depth cancel |
| `OutReqNewsBulletins` | 12 | News bulletins |
| `OutCancelNewsBulletins` | 13 | News bulletins cancel |
| `OutReqAutoOpenOrders` | 15 | Open orders auto-bind |
| `OutReqAllOpenOrders` | 16 | Open orders all |
| `OutReqManagedAccounts` | 17 | Managed accounts refresh |
| `OutRequestFA` | 18 | FA config |
| `OutReqHistoricalData` | 20 | Historical bars/schedule |
| `OutExerciseOptions` | 21 | Option exercise |
| `OutReqScannerSubscription` | 22 | Scanner subscription |
| `OutCancelScannerSubscription` | 23 | Scanner cancel |
| `OutReqScannerParameters` | 24 | Scanner parameters |
| `OutCancelHistoricalData` | 25 | Historical bars cancel |
| `OutReqCurrentTime` | 49 | Server wall-clock time request |
| `OutReqRealTimeBars` | 50 | Real-time bars |
| `OutCancelRealTimeBars` | 51 | Real-time bars cancel |
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
| `OutReqCurrentTimeInMillis` | 105 | Server wall-clock time request at millisecond precision |
| `OutCancelContractData` | 106 | Broker-side contract-details cancellation at sv215+ |
| `OutCancelHistoricalTicks` | 107 | Broker-side historical-ticks cancellation at sv215+ |
| `OutReqConfig` | 108 | Read-only TWS/Gateway configuration at sv219+ |

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
| `InBondContractData` | 18 | Bond contract details |
| `InScannerParameters` | 19 | Scanner parameters |
| `InScannerData` | 20 | Scanner data |
| `InTickOptionComputation` | 21 | Option computation |
| `InTickGeneric` | 45 | Market data generic tick |
| `InTickString` | 46 | Market data string tick |
| `InTickEFP` | 47 | Market data exchange-for-physical tick |
| `InCurrentTime` | 49 | Current time |
| `InCurrentTimeInMillis` | 109 | Current time in milliseconds |
| `InRealTimeBars` | 50 | Real-time bars |
| `InContractDataEnd` | 52 | Contract details end |
| `InOpenOrderEnd` | 53 | Open order end |
| `InAccountDownloadEnd` | 54 | Account download end |
| `InExecutionDataEnd` | 55 | Executions end |
| `InDeltaNeutralValidation` | 56 | Delta-neutral contract validation |
| `InTickSnapshotEnd` | 57 | Market data snapshot end |
| `InMarketDataType` | 58 | Market data type |
| `InCommissionReport` | 59 | Commission-and-fees report (legacy wire name) |
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
| `InTickNews` | 84 | Contract-specific news tick |
| `InNewsProviders` | 85 | News providers |
| `InHistoricalNews` | 86 | Historical news |
| `InHistoricalNewsEnd` | 87 | Historical news end |
| `InHeadTimestamp` | 88 | Head timestamp |
| `InHistogramData` | 89 | Histogram data |
| `InMarketDataReroute` | 91 | Transparent quote request reroute |
| `InMarketDepthReroute` | 92 | Transparent market-depth request reroute |
| `InMarketRule` | 93 | Market rule |
| `InPnL` | 94 | Account PnL |
| `InPnLSingle` | 95 | Single-position PnL |
| `InHistoricalTicks` | 96 | Historical midpoint ticks |
| `InHistoricalTicksBidAsk` | 97 | Historical bid/ask ticks |
| `InHistoricalTicksLast` | 98 | Historical last ticks |
| `InTickByTick` | 99 | Tick-by-tick |
| `InOrderBound` | 100 | Client-0 auto-open-order binding |
| `InCompletedOrder` | 101 | Completed order |
| `InCompletedOrderEnd` | 102 | Completed orders end |
| `InUserInfo` | 107 | User info |
| `InWSHMetaData` | 104 | WSH metadata |
| `InWSHEventData` | 105 | WSH event data |
| `InHistoricalSchedule` | 106 | Historical schedule (whatToShow=SCHEDULE) |
| `InHistoricalDataUpdate` | 90 | Historical bar updates (keepUpToDate) |
| `InHistoricalDataEnd` | 108 | Historical batch end marker |
| `InConfig` | 110 | Read-only TWS/Gateway configuration |

## Known Official Gaps Or Deferred Branches

These are not all defects. They are explicit matrix targets until live evidence
and project scope decide whether to implement, defer, or mark out of scope.

- `setServerLogLevel`.
- Verification/auth callbacks and redirect callbacks.
- Positive entitled `tickEFP`, `deltaNeutralValidation`, and odd-lot 105-110
  callbacks; all three public/codec paths are implemented.
- Raw paper-TWS evidence for `orderBound`, plus rare non-empty OpenOrder and
  what-if allocation/margin branches.
- Scale, nondefault delta-neutral, pegged, adjusted, FA allocation, MiFID/manual
  order, soft-dollar-on-order, and advanced-reject override order branches.

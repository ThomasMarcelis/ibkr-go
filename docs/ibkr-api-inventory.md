# IBKR API Inventory

The official TWS API surface (EClient requests, EWrapper callbacks) mapped to
what `ibkr-go` implements. It keeps
[`live-coverage-matrix.md`](live-coverage-matrix.md) complete: every official
family appears here, and every implemented behavior appears in the matrix.
Numeric message IDs and version gates are owned by `internal/protocol`; this
document is descriptive.

## Official Sources

| Source | What It Contributes |
|--------|---------------------|
| [IBKR API Software](https://interactivebrokers.github.io/) | Downloadable API package, license, and recommended TWS/IB Gateway version. The current source audit uses API 10.50.01. |
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

The `interactivebrokers.github.io/tws-api` pages above are IBKR's deprecated
legacy reference. Use Campus for current guidance and the downloaded API
source for newer requests and fields.

The downloadable official API package is the implementation source of truth
after accepting IBKR's license. Do not vendor or redistribute it. For this repo,
only derived method/callback names and public behavioral notes should be
committed.

API 10.50.01 was compared with the prior 10.48.01 source baseline. It adds
`ContractDetails.settlementMethod` at protobuf field 65, now exposed from
live sv225 option and future callbacks, and
`Order.conditionsIncludeOvernight` at field 145. The order field requires
`server_version` 226 and is intentionally absent while support remains
exactly 208–225.

## Official EClient Request And Control Methods

| Group | Official Methods | ibkr-go Status |
|-------|------------------|----------------|
| Connection/session | `eConnect`, `eDisconnect`, `startApi`, `Close`, `IsConnected`, `SetConnectOptions`, `redirect`, `DisableUseV100Plus`, `reqCurrentTime`, `reqCurrentTimeInMillis`, `reqIds`, `reqManagedAccts`, `setServerLogLevel` | `DialContext` and the client lifecycle cover connect, start, and close. `reqCurrentTime` / `reqCurrentTimeInMillis` are `Client.CurrentTime` / `Client.CurrentTimeMillis`, `reqIds` is `Orders().RefreshOrderID`, `reqManagedAccts` is `Client.ManagedAccounts`. Server log level, redirect, and the pre-V100 toggle are non-goals. |
| Verification/internal auth | `verifyRequest`, `verifyMessage`, `verifyAndAuthRequest`, `verifyAndAuthMessage` | Official-internal. Out of public scope. |
| Market data L1 | `reqMktData`, `cancelMktData`, `reqMarketDataType` | Implemented. Quote streams keep every price and size callback, including unmapped tick types and attributes, plus generic, string, request-parameter, option-computation, EFP, delta-neutral, and contract-news callbacks. Odd-lot ticks 105-110 (generic tick 787) decode; positive values await an entitled capture (MD1-004). |
| Tick-by-tick | `reqTickByTickData`, `cancelTickByTickData` | Implemented; positive callbacks await an entitled capture (MD2-002). |
| Real-time and historical bars | `reqRealTimeBars`, `cancelRealTimeBars`, `reqHistoricalData`, `cancelHistoricalData`, `reqHeadTimestamp`, `cancelHeadTimestamp`, `reqHistogramData`, `cancelHistogramData`, `reqHistoricalTicks` | Implemented, including `History().Schedule`. Real-time bars and keep-up-to-date updates await a market-hours capture (MD1-005, HIST-002). |
| Market depth | `reqMarketDepth`, `cancelMktDepth`, `reqMktDepthExchanges` | Implemented; positive depth rows await an entitled capture (MD2-001). |
| Contracts/reference | `reqContractDetails`, `reqMatchingSymbols`, `reqSecDefOptParams`, `reqSmartComponents`, `reqMarketRule` | Implemented. `Contract` owns include-expired and external-ID selectors plus BAG and delta-neutral composition; nil and explicit zero are distinct for strike and leg exempt code. sv225 captures freeze stock, bond, fund, option, issuer, BAG, and selector shapes, and `ContractDetails.SettlementMethod` (API 10.50.01 field 65). Positive BAG delta-neutral validation is blocked (AORD-007). |
| Accounts/portfolio | `reqAccountSummary`, `cancelAccountSummary`, `reqAccountUpdates`, `reqPositions`, `cancelPositions`, `reqPositionsMulti`, `cancelPositionsMulti`, `reqAccountUpdatesMulti`, `cancelAccountUpdatesMulti`, `reqFamilyCodes`, `reqPnL`, `cancelPnL`, `reqPnLSingle`, `cancelPnLSingle` | Implemented, including account-summary group selection and a returned-account filter. FA named groups need an FA account (ACCT-001). |
| Orders/executions | `placeOrder`, `cancelOrder`, `reqGlobalCancel`, `reqOpenOrders`, `reqAllOpenOrders`, `reqAutoOpenOrders`, `reqCompletedOrders`, `reqExecutions` | Implemented. Open and completed orders carry BAG legs and delta-neutral composition on their `Contract`; `Order.Combo` holds only prices and routing. sv225 campaigns replay common and advanced fields, bracket, OCA, scale, algorithmic, protective-stop, and per-leg-priced BAG lifecycles. `SubscribeExecutionEvents` passively observes every execution and fee callback. Attached preset-order metadata (sv218) stays internal until a native paper-order lifecycle is captured. |
| Options | `calculateImpliedVolatility`, `cancelCalculateImpliedVolatility`, `calculateOptionPrice`, `cancelCalculateOptionPrice`, `exerciseOptions` | Implemented. sv210 and sv211 captures cover price and implied-volatility results. The exercise replay proves accepted-but-unsettled admission only (OPT-002) and must not be repeated live. |
| News | `reqNewsProviders`, `reqNewsBulletins`, `cancelNewsBulletins`, `reqNewsArticle`, `reqHistoricalNews` | Implemented. Bulletins await a live bulletin event (NEWS-001). |
| Scanner | `reqScannerParameters`, `reqScannerSubscription`, `cancelScannerSubscription` | Implemented, including every subscription field and both option lists. |
| FA/advisor | `requestFA`, `replaceFA`, `reqSoftDollarTiers` | Read-only. `replaceFA` mutates configuration and is out of scope; a positive document needs an FA account (FA-001). |
| WSH | `reqWshMetaData`, `cancelWshMetaData`, `reqWshEventData`, `cancelWshEventData` | Implemented; positive callbacks need a WSH entitlement (WSH-001, WSH-002). |
| Display groups/TWS | `queryDisplayGroups`, `subscribeToGroupEvents`, `updateDisplayGroup`, `unsubscribeFromGroupEvents`, `reqUserInfo`, `reqConfig`, `updateConfig` | Implemented except `updateConfig`, which mutates operator configuration and is out of scope. Read-only `TWS().Config` is available at sv219. |

IBKR API 10.47 removed `reqFundamentalData`, `cancelFundamentalData`, their
callback, and fundamental-ratios tick type 47. They are absent from both
inventories. WSH is a separate API, not a replacement.

## Official EWrapper Callback Families

| Group | Official Callbacks | ibkr-go Status |
|-------|--------------------|----------------|
| Errors/session | `error`, `connectionClosed`, `currentTime`, `currentTimeInMillis`, `nextValidId`, `managedAccounts` | Implemented. Transport closure surfaces as session events and request lifecycle errors, not as a callback. |
| Market data L1 | `tickPrice`, `tickSize`, `tickString`, `tickGeneric`, `tickEFP`, `tickOptionComputation`, `tickSnapshotEnd`, `marketDataType`, `tickReqParams`, `tickNews` | All are implemented as typed quote updates. `tickEFP` is official-layout frozen but awaits a positive entitled callback. `tickNews` is inbound message 84 and is delivered as `QuoteUpdateNewsTick`. |
| Tick-by-tick | `tickByTickAllLast`, `tickByTickBidAsk`, `tickByTickMidPoint` | Implemented through one tick-by-tick decoder. |
| Contracts/reference | `contractDetails`, `bondContractDetails`, `contractDetailsEnd`, `symbolSamples`, `securityDefinitionOptionParameter`, `securityDefinitionOptionParameterEnd`, `smartComponents`, `marketRule`, `mktDepthExchanges` | Implemented. `bondContractDetails` is projected through `ContractDetails.Bond`; tagged CUSIP/ISIN values, size rules, precision fields, and ineligibility reasons are preserved without inference. |
| Historical | `historicalData`, `historicalDataEnd`, `historicalDataUpdate`, `historicalSchedule`, `headTimestamp`, `histogramData`, `historicalTicks`, `historicalTicksBidAsk`, `historicalTicksLast`, `historicalNews`, `historicalNewsEnd` | Implemented. |
| Accounts/portfolio | `accountSummary`, `accountSummaryEnd`, `updateAccountValue`, `updatePortfolio`, `updateAccountTime`, `accountDownloadEnd`, `position`, `positionEnd`, `positionMulti`, `positionMultiEnd`, `accountUpdateMulti`, `accountUpdateMultiEnd`, `pnl`, `pnlSingle`, `familyCodes` | Implemented. |
| Orders/executions | `openOrder`, `openOrderEnd`, `orderStatus`, `execDetails`, `execDetailsEnd`, `commissionAndFeesReport`, `completedOrder`, `completedOrdersEnd`, `orderBound` | Implemented. `orderBound` is delivered as `OpenOrderUpdate.Binding` on the client-0 auto-open-orders scope; it awaits a paper-TWS capture (ORD-005). Rare bond yield and account-specific branches are unattested. |
| Market depth | `updateMktDepth`, `updateMktDepthL2` | Implemented; positive rows await an entitled capture (MD2-001). |
| News/scanner | `newsProviders`, `newsArticle`, `updateNewsBulletin`, `scannerParameters`, `scannerData`, `scannerDataEnd` | Implemented. |
| FA/WSH/display | `receiveFA`, `softDollarTiers`, `wshMetaData`, `wshEventData`, `displayGroupList`, `displayGroupUpdated`, `userInfo`, `config` | Implemented. |
| Verification/reroute | `verifyMessageAPI`, `verifyCompleted`, `verifyAndAuthMessageAPI`, `verifyAndAuthCompleted`, `connectAck`, `rerouteMktDataReq`, `rerouteMktDepthReq`, `deltaNeutralValidation` | Both reroute callbacks are implemented and transparently replace the active quote or depth request; the sv225 IBM CFD capture proves the quote path. Delta-neutral validation is delivered through the requesting quote subscription. Verification/auth and `connectAck` are out of scope. |

## ibkr-go Public Surface

The [package reference](https://pkg.go.dev/github.com/ThomasMarcelis/ibkr-go/v2)
is authoritative for Go methods and types; the README carries the facade
overview.

## Message Identities

`internal/protocol/registry.go` is the only registry of outbound and inbound
message IDs; its tests reject duplicate names or IDs within a direction.
`cmd/ibkr-capture -list-json` names the message IDs each scenario exercises,
and the repository audit requires every one of them to exist in that
registry.

## Known Official Gaps Or Deferred Branches

Official surface without a positive live capture. Each is either blocked on
an entitlement, account type, or event, or deliberately out of scope.

- `setServerLogLevel`.
- Verification/auth callbacks and redirect callbacks.
- Positive entitled `tickEFP`, `deltaNeutralValidation`, and odd-lot 105-110
  callbacks; all three public/codec paths are implemented.
- Raw paper-TWS evidence for `orderBound`, plus rare account-specific OpenOrder
  and allocation/margin branches.
- Nondefault delta-neutral, FA allocation, MiFID/manual-order, and
  soft-dollar-on-order echoes that require unavailable products or accounts.

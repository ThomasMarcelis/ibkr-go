# Message Coverage

This matrix tracks the implemented message surface. The codec uses real IBKR
integer message IDs and field layouts validated against server_version 200
captures.

## Bootstrap

| Direction | Msg ID | Name | Status |
|-----------|--------|------|--------|
| out | 71 | StartAPI | landed |
| in | — | server hello ack | landed |
| in | 15 | ManagedAccounts | landed |
| in | 9 | NextValidID | landed |
| in | 49 | CurrentTime | landed |
| in | 4 | APIError / status codes | landed |
| out | 59 | reqMarketDataType | landed |
| out | 104 | reqUserInfo | landed |
| in | 103 | UserInfo | landed |

Bootstrap is load-bearing. `DialContext` is not ready until the negotiated
server version and managed-account bootstrap fields are known.

## Contract and Reference Data

| Direction | Msg ID | Name | Status |
|-----------|--------|------|--------|
| out | 9 | ContractDetailsRequest | landed |
| in | 10 | ContractDetails | landed |
| in | 52 | ContractDetailsEnd | landed |
| out | 81 | reqMatchingSymbols | landed |
| in | 79 | SymbolSamples | landed |
| out | 91 | reqMarketRule | landed |
| in | 92 | MarketRule | landed |
| out | 78 | reqSecDefOptParams | landed |
| in | 75 | SecurityDefinitionOptionParameter | landed |
| in | 76 | SecurityDefinitionOptionParameterEnd | landed |
| out | 83 | reqSmartComponents | landed |
| in | 82 | SmartComponents | landed |

## Accounts and Positions

| Direction | Msg ID | Name | Status |
|-----------|--------|------|--------|
| out | 62 | AccountSummaryRequest | landed |
| out | 63 | CancelAccountSummary | landed |
| in | 63 | AccountSummaryValue | landed |
| in | 64 | AccountSummaryEnd | landed |
| out | 61 | PositionsRequest | landed |
| out | 64 | CancelPositions | landed |
| in | 61 | Position | landed |
| in | 62 | PositionEnd | landed |
| out | 6 | reqAccountUpdates | landed |
| in | 6 | UpdateAccountValue | landed |
| in | 7 | UpdatePortfolio | landed |
| in | 8 | UpdateAccountTime | landed |
| in | 54 | AccountDownloadEnd | landed |
| out | 76 | reqAccountUpdatesMulti | landed |
| out | 77 | cancelAccountUpdatesMulti | landed |
| in | 73 | AccountUpdateMulti | landed |
| in | 74 | AccountUpdateMultiEnd | landed |
| out | 74 | reqPositionsMulti | landed |
| out | 75 | cancelPositionsMulti | landed |
| in | 71 | PositionMulti | landed |
| in | 72 | PositionMultiEnd | landed |
| out | 80 | reqFamilyCodes | landed |
| in | 78 | FamilyCodes | landed |

## Account PnL

| Direction | Msg ID | Name | Status |
|-----------|--------|------|--------|
| out | 92 | reqPnL | landed |
| out | 93 | cancelPnL | landed |
| in | 94 | PnL | landed |
| out | 94 | reqPnLSingle | landed |
| out | 95 | cancelPnLSingle | landed |
| in | 95 | PnLSingle | landed |

## Market Data

| Direction | Msg ID | Name | Status |
|-----------|--------|------|--------|
| out | 1 | QuoteRequest (reqMktData) | landed |
| out | 2 | CancelQuote (cancelMktData) | landed |
| in | 1 | TickPrice | landed |
| in | 2 | TickSize | landed |
| in | 45 | TickGeneric | landed |
| in | 46 | TickString | landed |
| in | 81 | TickReqParams | landed |
| in | 58 | MarketDataType | landed |
| in | 57 | TickSnapshotEnd | landed |
| out | 59 | reqMarketDataType | landed |
| out | 97 | reqTickByTickData | landed |
| out | 98 | cancelTickByTickData | landed |
| in | 99 | TickByTick | landed |

## Real-Time and Historical Bars

| Direction | Msg ID | Name | Status |
|-----------|--------|------|--------|
| out | 50 | RealTimeBarsRequest | landed |
| out | 51 | CancelRealTimeBars | landed |
| in | 50 | RealTimeBar | landed |
| out | 20 | HistoricalBarsRequest | landed |
| out | 25 | cancelHistoricalData | landed |
| in | 17 | HistoricalBar / HistoricalBarsEnd | landed |
| — | 20 | keepUpToDate flag | landed |
| in | 108 | HistoricalDataUpdate | landed |

## Historical Data Extensions

| Direction | Msg ID | Name | Status | Notes |
|-----------|--------|------|--------|-------|
| out | 87 | reqHeadTimestamp | landed | |
| out | 90 | cancelHeadTimestamp | landed | |
| in | 88 | HeadTimestamp | landed | |
| out | 88 | reqHistogramData | landed | |
| out | 89 | cancelHistogramData | landed | |
| in | 89 | HistogramData | landed | |
| out | 96 | reqHistoricalTicks | landed | |
| in | 96 | HistoricalTicks | landed | |
| in | 97 | HistoricalTicksBidAsk | landed | tickAttribBidAsk decoded and exposed |
| in | 98 | HistoricalTicksLast | landed | tickAttribLast decoded and exposed |
| in | 106 | HistoricalSchedule | landed | `whatToShow=SCHEDULE` response |

Historical tick and historical news request windows are formatted with explicit
time zone suffixes when callers provide non-zero `time.Time` values, so TWS does
not reinterpret UTC instants in the login time zone.

## Option Calculations

| Direction | Msg ID | Name | Status |
|-----------|--------|------|--------|
| out | 54 | reqCalcImpliedVolatility | landed |
| out | 56 | cancelCalcImpliedVolatility | landed |
| out | 55 | reqCalcOptionPrice | landed |
| out | 57 | cancelCalcOptionPrice | landed |
| in | 21 | TickOptionComputation | landed |

## Order Management

| Direction | Msg ID | Name | Status | Notes |
|-----------|--------|------|--------|-------|
| out | 3 | PlaceOrder | landed | BAG combo legs, algo params, and grounded order conditions encoded; delta-neutral/scale extensions remain deferred |
| out | 4 | CancelOrder | landed | |
| out | 58 | reqGlobalCancel | landed | |
| in | 5 | OpenOrder | landed | Simple 169-field orders fully parse; grounded non-simple combo/algo/conditional sections are decoded |
| in | 3 | OrderStatus | landed | Full parse, authoritative fill data for all order types |

OpenOrder is dual-dispatched to per-order handles and the singleton open-orders
observer. OrderStatus is routed to per-order handles; open-orders observers
read status from the OpenOrder payload.

OpenOrder uses a dual path:
- 169-field simple orders stay on the capture-grounded fixed-layout decoder.
- Expanded non-simple orders use a sequential decoder for grounded combo,
  algo, and condition sections. Rare delta-neutral, scale, and other
  ungrounded branches still fall back to the safe partial parse.

## Order and Execution Observation

| Direction | Msg ID | Name | Status | Notes |
|-----------|--------|------|--------|-------|
| out | 5 | ReqOpenOrders | landed | |
| out | 15 | ReqAutoOpenOrders | landed | |
| out | 16 | ReqAllOpenOrders | landed | |
| in | 5 | OpenOrder | landed | See Order Management notes |
| in | 53 | OpenOrderEnd | landed | |
| in | 3 | OrderStatus | landed | |
| out | 7 | ExecutionsRequest | landed | |
| in | 11 | ExecutionDetail | landed | |
| in | 55 | ExecutionsEnd | landed | |
| in | 59 | CommissionReport | landed | |
| out | 99 | reqCompletedOrders | landed | |
| in | 101 | CompletedOrder | landed | Simplified: advanced order detail sections still skipped |
| in | 102 | CompletedOrdersEnd | landed | |

## News

| Direction | Msg ID | Name | Status |
|-----------|--------|------|--------|
| out | 85 | reqNewsProviders | landed |
| in | 85 | NewsProviders | landed |
| out | 12 | reqNewsBulletins | landed |
| out | 13 | cancelNewsBulletins | landed |
| in | 14 | NewsBulletins | landed |
| out | 84 | reqNewsArticle | landed |
| in | 83 | NewsArticle | landed |
| out | 86 | reqHistoricalNews | landed |
| in | 86 | HistoricalNews | landed |
| in | 87 | HistoricalNewsEnd | landed |

## Scanner

| Direction | Msg ID | Name | Status |
|-----------|--------|------|--------|
| out | 24 | reqScannerParameters | landed |
| in | 19 | ScannerParameters | landed |
| out | 22 | reqScannerSubscription | landed |
| out | 23 | cancelScannerSubscription | landed |
| in | 20 | ScannerData | landed |

## Market Depth (Level 2)

| Direction | Msg ID | Name | Status |
|-----------|--------|------|--------|
| out | 10 | reqMktDepth | landed |
| out | 11 | cancelMktDepth | landed |
| in | 12 | MarketDepth | landed |
| in | 13 | MarketDepthL2 | landed |

## Fundamental Data

| Direction | Msg ID | Name | Status |
|-----------|--------|------|--------|
| out | 52 | reqFundamentalData | landed |
| out | 53 | cancelFundamentalData | landed |
| in | 51 | FundamentalData | landed |

## Exercise Options

| Direction | Msg ID | Name | Status |
|-----------|--------|------|--------|
| out | 21 | ExerciseOptions | landed |

## FA Configuration

| Direction | Msg ID | Name | Status |
|-----------|--------|------|--------|
| out | 18 | RequestFA | landed |
| out | 19 | ReplaceFA | landed |
| in | 16 | ReceiveFA | landed |
| out | 79 | reqSoftDollarTiers | landed |
| in | 77 | SoftDollarTiers | landed |

## WSH Calendar

| Direction | Msg ID | Name | Status |
|-----------|--------|------|--------|
| out | 100 | reqWSHMetaData | landed |
| out | 101 | cancelWSHMetaData | landed |
| in | 104 | WSHMetaData | landed |
| out | 102 | reqWSHEventData | landed |
| out | 103 | cancelWSHEventData | landed |
| in | 105 | WSHEventData | landed |

## Display Groups

| Direction | Msg ID | Name | Status |
|-----------|--------|------|--------|
| out | 67 | queryDisplayGroups | landed |
| out | 68 | subscribeToGroupEvents | landed |
| out | 69 | updateDisplayGroup | landed |
| out | 70 | unsubscribeFromGroupEvents | landed |
| in | 67 | DisplayGroupList | landed |
| in | 68 | DisplayGroupUpdated | landed |

## Other

| Direction | Msg ID | Name | Status |
|-----------|--------|------|--------|
| out | 82 | reqMktDepthExchanges | landed |
| in | 80 | MktDepthExchanges | landed |
| out | 104 | reqUserInfo | landed |
| in | 103 | UserInfo | landed |

## Session-Level Status

API/system codes that drive `Ready`, `Degraded`, `Reconnecting`, and
`Gap`/`Resumed` semantics.

## Completion Markers

Snapshot and one-shot flows rely on explicit end markers:

| Msg ID | Name | Status |
|--------|------|--------|
| 52 | ContractDetailsEnd | landed |
| 64 | AccountSummaryEnd | landed |
| 62 | PositionEnd | landed |
| 57 | TickSnapshotEnd | landed |
| 17 | HistoricalBarsEnd | landed |
| 53 | OpenOrderEnd | landed |
| 55 | ExecutionsEnd | landed |
| 54 | AccountDownloadEnd | landed |
| 74 | AccountUpdateMultiEnd | landed |
| 72 | PositionMultiEnd | landed |
| 76 | SecurityDefinitionOptionParameterEnd | landed |
| 80 | HistoricalNewsEnd | landed |
| 102 | CompletedOrdersEnd | landed |

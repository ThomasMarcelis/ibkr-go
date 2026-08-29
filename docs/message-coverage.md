# Message Coverage

This matrix tracks the implemented message surface. The canonical
`internal/protocol` registry owns numeric identities and version gates; the
codec consumes aliases from it. Runtime coverage begins at server_version 208;
exact migrations and semantic gates are covered through server_version 225.
The executable decoder ledger partitions all 106 registered layouts: 86 have
positive raw-frame attestation and 20 are explicitly pending callbacks that
require unavailable entitlements, account types, or paper-TWS interaction.

## Bootstrap

| Direction | Msg ID | Name | Status |
|-----------|--------|------|--------|
| out | 71 | StartAPI | landed |
| out | 17 | ManagedAccountsRequest | landed; protobuf throughout the supported range |
| out | 8 | ReqIds | landed |
| out | 49 | ReqCurrentTime | landed |
| in | — | server hello ack | landed |
| in | 15 | ManagedAccounts | landed; protobuf bootstrap/refresh throughout the supported range |
| in | 9 | NextValidID | landed |
| in | 49 | CurrentTime | landed |
| out | 105 | ReqCurrentTimeInMillis | landed |
| in | 109 | CurrentTimeInMillis | landed |
| in | 4 | APIError / status codes | landed |
| out | 59 | reqMarketDataType | landed |
| out | 104 | reqUserInfo | landed |
| in | 107 | UserInfo | landed |

Bootstrap is load-bearing. `DialContext` is not ready until the negotiated
server version and managed-account bootstrap fields are known.

## Contract and Reference Data

| Direction | Msg ID | Name | Status |
|-----------|--------|------|--------|
| out | 9 | ContractDetailsRequest | landed; protobuf selectors throughout the supported range |
| in | 10 | ContractDetails | landed; current protobuf common/FUND shapes, including API 10.50.01 settlement method field 65 |
| in | 18 | BondContractDetails | landed; current protobuf bond shape |
| in | 52 | ContractDetailsEnd | landed; protobuf terminator |
| out | 81 | reqMatchingSymbols | landed |
| in | 79 | SymbolSamples | landed |
| out | 91 | reqMarketRule | landed |
| in | 93 | MarketRule | landed |
| out | 78 | reqSecDefOptParams | landed |
| in | 75 | SecurityDefinitionOptionParameter | landed |
| in | 76 | SecurityDefinitionOptionParameterEnd | landed |
| out | 83 | reqSmartComponents | landed |
| in | 82 | SmartComponents | landed |
| out | 106 | cancelContractDetails | landed at sv215+ |

Canonical Contract fields fail closed against each supported request layout.
Quote, depth, contract-detail, order, historical, real-time-bar, and
tick-by-tick requests are protobuf at the sv208 floor and use their current
schemas. Calculation and exercise requests retain reachable classic layouts
through sv210 before migrating at sv211, so their narrower field validation is
still version-aware. Structural and field-presence tests cover every reachable
layout without relying on pre208 frames. API 10.50.01 also defines
`Order.conditionsIncludeOvernight` at field 145 and minimum server version
226. It is outside the supported 208–225 range and is not encoded or exposed.

## Accounts and Positions

| Direction | Msg ID | Name | Status |
|-----------|--------|------|--------|
| out | 62 | AccountSummaryRequest | landed; protobuf throughout the supported range |
| out | 63 | CancelAccountSummary | landed; protobuf throughout the supported range |
| in | 63 | AccountSummaryValue | landed; current protobuf callback |
| in | 64 | AccountSummaryEnd | landed; current protobuf terminator |
| out | 61 | PositionsRequest | landed; protobuf throughout the supported range |
| out | 64 | CancelPositions | landed; protobuf throughout the supported range |
| in | 61 | Position | landed; current protobuf callback |
| in | 62 | PositionEnd | landed; current protobuf terminator |
| out | 6 | reqAccountUpdates | landed; protobuf throughout the supported range |
| in | 6 | UpdateAccountValue | landed; current protobuf callback |
| in | 7 | UpdatePortfolio | landed; current protobuf callback |
| in | 8 | UpdateAccountTime | landed; current protobuf callback |
| in | 54 | AccountDownloadEnd | landed; current protobuf terminator |
| out | 76 | reqAccountUpdatesMulti | landed; protobuf throughout the supported range |
| out | 77 | cancelAccountUpdatesMulti | landed; protobuf throughout the supported range |
| in | 73 | AccountUpdateMulti | landed; current protobuf callback |
| in | 74 | AccountUpdateMultiEnd | landed; current protobuf terminator |
| out | 74 | reqPositionsMulti | landed; protobuf throughout the supported range |
| out | 75 | cancelPositionsMulti | landed; protobuf throughout the supported range |
| in | 71 | PositionMulti | landed; current protobuf callback |
| in | 72 | PositionMultiEnd | landed; current protobuf terminator |
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
| in | 21 | TickOptionComputation | landed |
| in | 45 | TickGeneric | landed |
| in | 46 | TickString | landed |
| in | 47 | TickEFP | landed; official-layout frozen, positive live callback pending |
| in | 56 | DeltaNeutralValidation | landed; official-layout frozen, positive live callback pending |
| in | 81 | TickReqParams | landed |
| in | 58 | MarketDataType | landed |
| in | 57 | TickSnapshotEnd | landed |
| in | 84 | TickNews | landed |
| in | 91 | MarketDataReroute | landed |
| in | 92 | MarketDepthReroute | landed |
| out | 59 | reqMarketDataType | landed |
| out | 97 | reqTickByTickData | landed |
| out | 98 | cancelTickByTickData | landed |
| in | 99 | TickByTick | landed |

`QuoteUpdate.Kind` publicly preserves every TickPrice and TickSize callback,
including unmapped numeric tick types, the exact price-attribute mask, and an
optional price-frame companion size, without expanding the normalized `Quote`
snapshot. TickGeneric, TickString, TickReqParams, TickOptionComputation,
TickNews, TickEFP, and delta-neutral validation are distinct public payloads.
Current supported-range captures attest ordinary delayed prices/sizes, generic
and string ticks, request parameters, option computations, TickNews, and the
complete IBM CFD reroute through positive delayed quote data. Positive TickEFP,
delta-neutral, odd-lot 105-110, the depth-reroute callback, and L2 rows remain
explicit evidence gaps; code 10089/10092 blockers do not count as positive
data callbacks.

## Real-Time and Historical Bars

| Direction | Msg ID | Name | Status |
|-----------|--------|------|--------|
| out | 50 | RealTimeBarsRequest | landed |
| out | 51 | CancelRealTimeBars | landed |
| in | 50 | RealTimeBar | landed |
| out | 20 | HistoricalBarsRequest | landed |
| out | 25 | cancelHistoricalData | landed |
| in | 17 | HistoricalBar | landed |
| — | 20 | keepUpToDate flag | landed |
| in | 90 | HistoricalDataUpdate | landed |
| in | 108 | HistoricalDataEnd | landed |

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
| out | 107 | cancelHistoricalTicks | landed at sv215+ | sent only while the caller still owns the route |
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

Exact sv211 public capture
`59056822b51af4a00caa28afb922b4f79ee7014668591392e8f4fae229ea7222`
is replay-promoted: option-price availability is `247` and implied-volatility
availability is `133`. A later sv225 partial result remains account/session
evidence rather than overriding the positive boundary proof.

## Order Management

| Direction | Msg ID | Name | Status | Notes |
|-----------|--------|------|--------|-------|
| out | 3 | PlaceOrder | landed | Protobuf throughout the supported range. Current sv225 campaigns freeze common and advanced fields. `IncludeOvernight=true` is field 135 and has a positive placement/echo; replacing it with false returned code 462 through both ibkr-go and SDK 10.48.01 and retained true. A fresh explicit-false placement was accepted and broker-canonicalized to absence with `TIF=DAY`. |
| out | 4 | CancelOrder | landed | Protobuf throughout the supported range, including compliance metadata. |
| out | 58 | reqGlobalCancel | landed | Protobuf throughout the supported range and live-flushed before teardown. |
| in | 5 | OpenOrder | landed | Current protobuf callback projects complete `OrderDetails`, including presence-aware `IncludeOvernight`, and carries no fill echo. |
| in | 3 | OrderStatus | landed | Current protobuf callback; authoritative fill data for all order types. |

OpenOrder and OrderStatus are dual-dispatched to per-order handles and the
singleton open-orders observer. Each OpenOrder consumer receives a deep-owned
Contract/OrderCombo payload, including decimal pointers. Strict canonical
numeric conversion errors close those affected routes without closing the
session.

OpenOrder uses one strict protobuf walk: the delta-neutral block, advanced
scale extensions, grounded combo, algo and condition
sections, PEG BENCH reference fields, and the official 32-field
adjustedOrderType..imbalanceOnly tail. It never returns a partial result.
Malformed canonical Contract/combo numerics or trailing layout drift close the
affected route. The testhost encoder emits the same layout, so replay fixtures
exercise the production decode path. Public open and completed orders share
`OrderDetails`, including `OrderCombo`, advanced scale, short-sale, auction,
and pegged-benchmark echoes. `IncludeOvernight` preserves response presence:
nil means the broker omitted it, while explicit true and false remain distinct
in decoded protobuf values. `ComboDescription` remains response-only.
Per-leg-priced BAG limit orders use `OrderCombo.LegPrices` without a
conflicting combo-level limit; the accepted sv225 AAPL vertical is replayed
through zero-fill cancellation.

## Order and Execution Observation

| Direction | Msg ID | Name | Status | Notes |
|-----------|--------|------|--------|-------|
| out | 5 | ReqOpenOrders | landed | protobuf throughout the supported range |
| out | 15 | ReqAutoOpenOrders | landed | protobuf optional-true bind and absent-false unbind requests |
| out | 16 | ReqAllOpenOrders | landed | protobuf throughout the supported range |
| in | 5 | OpenOrder | landed | See Order Management notes |
| in | 53 | OpenOrderEnd | landed | current protobuf terminator; live empty snapshot replay prevents request timeouts |
| in | 3 | OrderStatus | landed | |
| in | 100 | OrderBound | landed | Protobuf binding callback for client-0 auto-open orders; positive raw paper-TWS capture remains pending. |
| out | 7 | ExecutionsRequest | landed | protobuf throughout the supported range; current empty and filtered requests are replayed; nondefault finite day/date filters await live attestation |
| in | 11 | ExecutionDetail | landed | current protobuf result with presence-aware contract fields and complete public projection |
| in | 55 | ExecutionsEnd | landed | current protobuf terminator with empty and nonempty replay coverage |
| in | 59 | CommissionAndFeesReport | landed | current protobuf decoder and live fee reports; meaningful bond yield/redemption remains unattested |
| out | 99 | reqCompletedOrders | landed | protobuf throughout the supported range; absent false and present true are frozen |
| in | 101 | CompletedOrder | landed | current protobuf Contract/Order/OrderState decode with presence-aware `IncludeOvernight`, identities, commission, and currency |
| in | 102 | CompletedOrdersEnd | landed | current protobuf terminator |

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

The complete outbound subscription shape, a ten-row `ScannerData` response,
and clean cancellation are live-attested through the public API. The curated
replay retains the exact captured result frame; an older run freezes the real
permission-denied codes 490 and 365.

## Market Depth (Level 2)

| Direction | Msg ID | Name | Status |
|-----------|--------|------|--------|
| out | 10 | reqMktDepth | landed |
| out | 11 | cancelMktDepth | landed |
| in | 12 | MarketDepth | landed |
| in | 13 | MarketDepthL2 | landed |

## Retired Message IDs

Official API 10.47 removed the fundamental-data request, cancellation, and
callback. Classic outbound IDs 52 and 53 and inbound ID 51 therefore remain
unused gaps rather than current coverage rows. Historical capture evidence is
recorded in [`live-test-tracker.md`](live-test-tracker.md).

## Exercise Options

| Direction | Msg ID | Name | Status |
|-----------|--------|------|--------|
| out | 21 | ExerciseOptions | landed |

The sv225 public replay freezes exact warning 10349, a `PreSubmitted`
pseudo-order, and captured disconnect as an uncertain accepted instruction.
It does not claim terminal exercise, lapse, or settlement.

## FA Configuration

| Direction | Msg ID | Name | Status |
|-----------|--------|------|--------|
| out | 18 | RequestFA | landed |
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
| in | 107 | UserInfo | landed |
| out | 108 | reqConfig | landed at sv219+ |
| in | 110 | Config | landed at sv219+ |

`TWS().Config` exposes the full current configuration response with pointer
presence preserved. `updateConfig` is intentionally not registered or exposed:
operator-owned TWS/Gateway configuration mutation is outside the library API.

## Session-Level Status

API/system codes that drive `Ready`, `Degraded`, `Reconnecting`, and
ordered `Gap`/`Restored`/`Resubscribed` semantics.

## Completion Markers

Snapshot and one-shot flows rely on explicit end markers:

| Msg ID | Name | Status |
|--------|------|--------|
| 52 | ContractDetailsEnd | landed |
| 64 | AccountSummaryEnd | landed |
| 62 | PositionEnd | landed |
| 57 | TickSnapshotEnd | landed |
| 108 | HistoricalDataEnd | landed |
| 53 | OpenOrderEnd | landed |
| 55 | ExecutionsEnd | landed |
| 54 | AccountDownloadEnd | landed |
| 74 | AccountUpdateMultiEnd | landed |
| 72 | PositionMultiEnd | landed |
| 76 | SecurityDefinitionOptionParameterEnd | landed |
| 87 | HistoricalNewsEnd | landed |
| 102 | CompletedOrdersEnd | landed |

# Message Coverage

This matrix tracks the implemented message surface. The canonical
`internal/protocol` registry owns numeric identities and version gates; the
codec consumes aliases from it. Classic layouts are validated against
server_version 200 captures; exact migrations and semantic gates are covered
through server_version 225.

## Bootstrap

| Direction | Msg ID | Name | Status |
|-----------|--------|------|--------|
| out | 71 | StartAPI | landed |
| out | 17 | ManagedAccountsRequest | landed; classic through sv206 plus exact-sv207 protobuf |
| in | — | server hello ack | landed |
| in | 15 | ManagedAccounts | landed; classic through sv206 plus exact-sv207 protobuf bootstrap/refresh |
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
| out | 9 | ContractDetailsRequest | landed; classic through sv204 plus exact-sv205 protobuf selectors |
| in | 10 | ContractDetails | landed; complete classic v200 common/FUND shapes plus exact-sv205 protobuf |
| in | 18 | BondContractDetails | landed; live-attested classic v200 and exact-sv205 protobuf bond shapes |
| in | 52 | ContractDetailsEnd | landed; classic plus exact-sv205 protobuf terminator |
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

Canonical Contract fields fail closed against each request layout. Classic
contract details carry IncludeExpired/SecurityID/IssuerID; classic quotes carry
BAG legs/delta-neutral; classic depth carries none of those extended fields;
classic place order carries SecurityID/BAG legs/delta-neutral. Historical bars
carry IncludeExpired/BAG legs, while head/histogram/historical-tick requests
carry only IncludeExpired. Real-time bars, tick-by-tick, calculations, and
exercise carry none of these extended fields. Common classic layouts carry
PrimaryExchange, but exercise does not. Classic quote/historical BAG legs carry
only ConID/Ratio/Action/Exchange, so nondefault leg position/short-sale fields
fail before send. Place-order, contract-data, and market-data protobuf
migrations use the complete shared Contract schema at 203, 205, and 206
respectively.

## Accounts and Positions

| Direction | Msg ID | Name | Status |
|-----------|--------|------|--------|
| out | 62 | AccountSummaryRequest | landed; classic through sv206 plus exact-sv207 protobuf |
| out | 63 | CancelAccountSummary | landed; classic through sv206 plus exact-sv207 protobuf |
| in | 63 | AccountSummaryValue | landed; classic plus exact-sv207 protobuf |
| in | 64 | AccountSummaryEnd | landed; classic plus exact-sv207 protobuf |
| out | 61 | PositionsRequest | landed; classic through sv206 plus exact-sv207 protobuf |
| out | 64 | CancelPositions | landed; classic through sv206 plus exact-sv207 protobuf |
| in | 61 | Position | landed; classic plus exact-sv207 protobuf |
| in | 62 | PositionEnd | landed; classic plus exact-sv207 protobuf |
| out | 6 | reqAccountUpdates | landed; classic through sv206 plus exact-sv207 protobuf |
| in | 6 | UpdateAccountValue | landed; classic plus exact-sv207 protobuf |
| in | 7 | UpdatePortfolio | landed; classic plus exact-sv207 protobuf |
| in | 8 | UpdateAccountTime | landed; classic plus exact-sv207 protobuf |
| in | 54 | AccountDownloadEnd | landed; classic plus exact-sv207 protobuf |
| out | 76 | reqAccountUpdatesMulti | landed; classic through sv206 plus exact-sv207 protobuf |
| out | 77 | cancelAccountUpdatesMulti | landed; classic through sv206 plus exact-sv207 protobuf |
| in | 73 | AccountUpdateMulti | landed; classic plus exact-sv207 protobuf |
| in | 74 | AccountUpdateMultiEnd | landed; classic plus exact-sv207 protobuf |
| out | 74 | reqPositionsMulti | landed; classic through sv206 plus exact-sv207 protobuf |
| out | 75 | cancelPositionsMulti | landed; classic through sv206 plus exact-sv207 protobuf |
| in | 71 | PositionMulti | landed; classic plus exact-sv207 protobuf |
| in | 72 | PositionMultiEnd | landed; classic plus exact-sv207 protobuf |
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
snapshot. TickGeneric, TickString, TickReqParams, and TickOptionComputation are
also distinct public payloads. The first generic/string/parameter frames and a
companion size are attested by
`captures/20260405T215752Z-quote_stream_genericticks`; mark-price tick 37,
shortable ticks 46/89, volume-rate tick 56, delayed-timestamp string tick 88,
and an omitted minimum tick are attested by
`captures/20260709T223341Z-api_generic_tick_matrix_aapl`; a quote-subscription
option computation is attested by
`captures/20260611T080111Z-api_option_campaign_aapl`. Contract-specific BRFG
TickNews is attested by the exact server-version-201 frames in
`captures/20260709T230825Z-api_tick_news_aapl_probe` and frozen at the public
API boundary by `tick_news_aapl_live.txt`.
TickEFP and delta-neutral validation are distinct typed updates. Generic tick
787 maps odd-lot tick IDs 105-110 into typed quote prices, sizes, and exchanges;
the exact-sv225 public and SDK probes received ordinary delayed values but no
odd-lot values under the current entitlement, so positive live evidence stays
pending. Exact server-version-206 request/callback protobuf vectors, parameter
precision, and transparent CFD reroutes at exact-sv206 classic and sv225
protobuf boundaries are frozen by codec/engine
tests backed by `protocol-audit-sv206.md`; the quote-focused
`market_data_sv206_live.txt` replay covers the public quote path. Positive
raw-213 L2 evidence remains pending because the capture account lacked
entitlement.

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

Both calculation requests and their successful result shapes are frozen
byte-for-byte from a live-qualified AAPL option at exact server version 204.
The public replay also checks which decimal fields were computed versus sent
as IBKR's unavailable sentinels.

## Order Management

| Direction | Msg ID | Name | Status | Notes |
|-----------|--------|------|--------|-------|
| out | 3 | PlaceOrder | landed | Complete public classic surface plus exact-sv203 `PlaceOrderRequest` protobuf. `Contract.conId` is proto3 optional; official EClientUtils nevertheless emits zero because `Utils::isValidValue(0)` is true, and a fresh guarded paper lifecycle capture freezes that request. Combo-leg price tag 9 is source-law coverage, not claimed live priced-combo evidence. |
| out | 4 | CancelOrder | landed | Classic plus exact-sv203 `CancelOrderRequest` protobuf, including compliance metadata. |
| out | 58 | reqGlobalCancel | landed | Classic plus exact-sv203 `GlobalCancelRequest` protobuf, live-flushed before teardown. |
| in | 5 | OpenOrder | landed | Live-grounded sv200 classic walk plus exact-sv203 protobuf `OpenOrder`; no fill echo (fills are order_status truth). |
| in | 3 | OrderStatus | landed | Classic and exact-sv203 protobuf parse; authoritative fill data for all order types. |

OpenOrder and OrderStatus are dual-dispatched to per-order handles and the
singleton open-orders observer. Each OpenOrder consumer receives a deep-owned
Contract/OrderCombo payload, including decimal pointers. Strict canonical
numeric conversion errors close those affected routes without closing the
session.

OpenOrder uses one live-grounded sequential walk: the "None"-sentinel
delta-neutral block, the no-scale section, grounded combo, algo, and
condition sections, and the official 32-field adjustedOrderType..imbalanceOnly
tail. The classic codec marks an order Partial only at its explicit
unattested order-level delta-neutral/advanced-layout boundaries; it does not
best-effort canonical Contract or combo numerics. Those conversions return an
error and close the affected route. The codec's OpenOrder encoder emits the
same live layout, so replay fixtures exercise the production decode path.
Public open and completed orders use the same `OrderCombo` shape as placement
for per-leg prices and smart-routing tags; `ComboDescription` is response-only.

## Order and Execution Observation

| Direction | Msg ID | Name | Status | Notes |
|-----------|--------|------|--------|-------|
| out | 5 | ReqOpenOrders | landed | Classic plus exact-sv204 empty protobuf request. |
| out | 15 | ReqAutoOpenOrders | landed | Classic plus exact-sv204 optional-true bind and absent-false unbind requests. |
| out | 16 | ReqAllOpenOrders | landed | Classic plus exact-sv204 empty protobuf request. |
| in | 5 | OpenOrder | landed | See Order Management notes |
| in | 53 | OpenOrderEnd | landed | Classic plus exact-sv203 empty protobuf terminator after a still-classic `ReqAllOpenOrders`; live empty snapshot replay prevents request timeouts. |
| in | 3 | OrderStatus | landed | |
| out | 7 | ExecutionsRequest | landed | Complete classic filter plus sv200 day/date tail; exact-sv201 protobuf empty-filter request is live-frozen and unchanged at the zero-strike-only sv202 boundary. Nondefault day filters await live attestation. |
| in | 11 | ExecutionDetail | landed | Complete version-gated classic result plus sv201 protobuf decoder; a sanitized exact vector and public one-share paper round trip attest nonempty sv201 results. Exact sv202 adds a live vector with both Contract.conId and explicitly present strike=0. |
| in | 55 | ExecutionsEnd | landed | Raw sv200 freeze, exact-sv201 protobuf live vector/public empty-query replay, and exact-sv202 nonempty replay. |
| in | 59 | CommissionAndFeesReport | landed | Complete classic and sv201 protobuf decoders; the live sv201 round trip sent classic fee reports. Meaningful bond yield/redemption and a protobuf-encoded fee report remain unattested. |
| out | 99 | reqCompletedOrders | landed | Classic plus exact-sv204 protobuf request; absent false and present true are exact-vector frozen. |
| in | 101 | CompletedOrder | landed | Exact sequential classic decoder plus exact-sv204 protobuf Contract/Order/OrderState decode. The public projection preserves presence-aware order/client/parent identities and observed completion commission/currency. Advanced branches without a nondefault live frame remain unattested. |
| in | 102 | CompletedOrdersEnd | landed | Classic plus exact-sv204 empty protobuf terminator. |

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
| 17 | HistoricalBarsEnd | landed |
| 53 | OpenOrderEnd | landed |
| 55 | ExecutionsEnd | landed |
| 54 | AccountDownloadEnd | landed |
| 74 | AccountUpdateMultiEnd | landed |
| 72 | PositionMultiEnd | landed |
| 76 | SecurityDefinitionOptionParameterEnd | landed |
| 87 | HistoricalNewsEnd | landed |
| 102 | CompletedOrdersEnd | landed |

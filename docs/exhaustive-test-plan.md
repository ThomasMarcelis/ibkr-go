# Exhaustive Test Plan

Master plan for reaching complete live evidence and CI replay coverage of the
entire IBKR Gateway socket protocol. Every protocol message, every feature
combination, every edge case.

Companion to:
- [`live-coverage-matrix.md`](live-coverage-matrix.md) — capability-level status tracking
- [`ibkr-api-inventory.md`](ibkr-api-inventory.md) — official API surface inventory
- [`live-test-tracker.md`](live-test-tracker.md) — execution-level test results

## Coverage standard

A capability is **complete** when all three layers exist:

1. **Live test** — runs against a real IB Gateway and asserts behavior
2. **Capture** — raw wire recording from the live test or capture tool
3. **Promoted transcript** — curated replay in `testdata/transcripts/` with CI integration test

The goal is to reach this standard for every row below.

Executable batches now map this plan to `cmd/ibkr-capture` runs:
`exhaustive-read-only`, `exhaustive-trading`,
`exhaustive-market-hours`, `exhaustive-premarket`, and
`exhaustive-permission-probes`. These batches are generated from the scenario
catalog so new live probes remain discoverable and replay promotion can follow
the same capture pipeline.

Capture runs use the scenario catalog's risk class to choose the Gateway role:
`paper_*` scenarios run through the paper-dev Gateway, while read-only and
entitlement probes run through the readonly-live Gateway unless the maintainer
sets an explicit override.

## 1. Protocol Messages

Every outbound and inbound message ID must have at least one live-grounded
scenario.

### 1.1 Outbound (client → server): 74 message IDs

| ID | Name | Live | Capture | Transcript | Gap |
|----|------|------|---------|------------|-----|
| 1 | reqMktData | yes | yes | yes | snapshot vs stream vs generic ticks |
| 2 | cancelMktData | yes | yes | yes | |
| 3 | placeOrder | yes | yes | yes | nondefault scale/hedge, live per-leg prices, successful BAG delta-neutral |
| 4 | cancelOrder | yes | yes | yes | non-empty CME tagging metadata |
| 5 | reqOpenOrders | yes | yes | yes | |
| 6 | reqAccountUpdates | yes | yes | yes | |
| 7 | reqExecutions | yes | yes | yes | filter variants |
| 8 | reqIds | yes | yes | yes | |
| 9 | reqContractDetails | yes | yes | yes | Classic plus exact-sv205 protobuf stock/bond/fund/option/issuer vectors and public stock+bond replay; BAG and delta-neutral contract-data responses remain unattested |
| 10 | reqMktDepth | yes | yes | yes | smart depth, entitlement error |
| 11 | cancelMktDepth | yes | yes | yes | |
| 12 | reqNewsBulletins | yes | yes | yes | |
| 13 | cancelNewsBulletins | yes | yes | yes | |
| 15 | reqAutoOpenOrders | yes | yes | yes | |
| 16 | reqAllOpenOrders | yes | yes | yes | |
| 17 | reqManagedAccounts | yes | yes | yes | bootstrap managed-account discovery |
| 18 | requestFA | yes | yes | partial | non-FA error frozen; FA-account path missing |
| 20 | reqHistoricalData | yes | yes | yes | schedule variant, more bar sizes |
| 21 | exerciseOptions | yes | yes | yes | lapse, override, and successful clearing settlement |
| 22 | reqScannerSubscription | yes | yes | yes | complete 25-field public request and ten-row result are live-captured |
| 23 | cancelScannerSubscription | yes | yes | yes | clean cancel after live results plus rejected-request code 365 path |
| 24 | reqScannerParameters | yes | yes | yes | |
| 25 | cancelHistoricalData | yes | yes | yes | |
| 49 | reqCurrentTime | yes | yes | yes | |
| 50 | reqRealTimeBars | yes | yes | yes | BID_ASK, MIDPOINT variants |
| 51 | cancelRealTimeBars | yes | yes | yes | |
| 54 | reqCalcImpliedVolatility | yes | yes | yes | |
| 55 | reqCalcOptionPrice | yes | yes | yes | |
| 56 | cancelCalcImpliedVolatility | yes | yes | yes | |
| 57 | cancelCalcOptionPrice | yes | yes | yes | |
| 58 | reqGlobalCancel | yes | yes | yes | with mixed bracket/OCA/conditional |
| 59 | reqMarketDataType | yes | yes | partial | per-type live evidence |
| 61 | reqPositions | yes | yes | yes | |
| 62 | reqAccountSummary | yes | yes | yes | |
| 63 | cancelAccountSummary | yes | yes | yes | |
| 64 | cancelPositions | yes | yes | yes | |
| 67 | queryDisplayGroups | yes | yes | yes | |
| 68 | subscribeToGroupEvents | yes | yes | yes | |
| 69 | updateDisplayGroup | yes | yes | yes | |
| 70 | unsubscribeFromGroupEvents | yes | yes | yes | |
| 71 | startApi | yes | yes | yes | |
| 74 | reqPositionsMulti | yes | yes | yes | |
| 75 | cancelPositionsMulti | yes | yes | yes | |
| 76 | reqAccountUpdatesMulti | yes | yes | yes | |
| 77 | cancelAccountUpdatesMulti | yes | yes | yes | |
| 78 | reqSecDefOptParams | yes | yes | yes | blocked on paper (OPT permissions) |
| 79 | reqSoftDollarTiers | yes | yes | yes | |
| 80 | reqFamilyCodes | yes | yes | yes | |
| 81 | reqMatchingSymbols | yes | yes | yes | |
| 82 | reqMktDepthExchanges | yes | yes | yes | |
| 83 | reqSmartComponents | yes | yes | yes | |
| 84 | reqNewsArticle | yes | yes | yes | captured through `api_news_article_aapl`; add invalid article/provider variants |
| 85 | reqNewsProviders | yes | yes | yes | |
| 86 | reqHistoricalNews | yes | yes | yes | |
| 87 | reqHeadTimestamp | yes | yes | yes | |
| 88 | reqHistogramData | yes | yes | yes | |
| 89 | cancelHistogramData | yes | yes | yes | |
| 90 | cancelHeadTimestamp | yes | yes | yes | |
| 91 | reqMarketRule | yes | yes | yes | |
| 92 | reqPnL | yes | yes | yes | |
| 93 | cancelPnL | yes | yes | yes | |
| 94 | reqPnLSingle | yes | yes | yes | |
| 95 | cancelPnLSingle | yes | yes | yes | |
| 96 | reqHistoricalTicks | yes | yes | yes | |
| 97 | reqTickByTickData | yes | yes | partial | AllLast, ignoreSize variants |
| 98 | cancelTickByTickData | yes | yes | partial | |
| 99 | reqCompletedOrders | yes | yes | partial | classic and exact-sv204 absent-false/present-true requests are live-frozen; nondefault advanced completed-order branches remain |
| 100 | reqWSHMetaData | yes | yes | partial | entitlement error |
| 101 | cancelWSHMetaData | partial | partial | yes | |
| 102 | reqWSHEventData | yes | yes | partial | filter/date/portfolio variants |
| 103 | cancelWSHEventData | partial | partial | no | |
| 104 | reqUserInfo | yes | yes | yes | |
| 105 | reqCurrentTimeInMillis | yes | yes | yes | exact supported request/response |
| 106 | cancelContractDetails | yes | yes | yes | exact-sv215 broker-side cancellation |
| 107 | cancelHistoricalTicks | yes | yes | yes | exact-sv215 broker-side cancellation |
| 108 | reqConfig | yes | yes | yes | exact-sv219 response and public scenario |

### 1.2 Inbound (server → client): 79 message IDs

All are exercised through the outbound scenarios above. Individual gaps:

| ID | Name | Gap |
|----|------|-----|
| 47 | tickEFP | typed official layout; positive entitled callback not observed |
| 56 | deltaNeutralValidation | typed official layout; positive BAG callback not observed |
| 110 | config | exact-sv219 SDK/live and public evidence |
| 14 | newsBulletins | live capture exists; allMessages variant untested |
| 21 | tickOptionComputation | live calc scenarios exist; streaming OPT tick untested |
| 83 | newsArticle | captured through `api_news_article_aapl`; invalid article/provider variants remain |
| 101 | completedOrder | classic full-field and exact-sv204 protobuf projection landed; nondefault combo, scale, hedge, active delta-neutral, PEG BENCH, condition, and FA frames remain |
| 90 | historicalDataUpdate | keep-up-to-date exists; edge cases untested |
| 108 | historicalDataEnd | standalone end marker; edge cases untested |

### 1.3 Contract field-layout law

Extended canonical fields are rejected before route installation unless the
request's negotiated layout represents them:

| Request family | Classic fields | Protobuf gate and fields |
|---|---|---|
| Quote | ComboLegs, DeltaNeutral | sv206: all |
| Market depth | none | sv206: all |
| Contract details | IncludeExpired, SecurityID, IssuerID | sv205: all |
| Place/preview/modify/bracket | SecurityID, ComboLegs, DeltaNeutral | sv203: all |
| Historical bars/schedule/stream | IncludeExpired, ComboLegs | no migration through sv206 |
| Head/histogram/historical ticks | IncludeExpired | no migration through sv206 |
| Real-time bars/tick-by-tick/calculations/exercise | none | no migration through sv206 |

All classic rows above use the common identity block including
PrimaryExchange except exercise, whose custom layout omits it. Classic quote
and historical combo layouts carry only leg ConID, Ratio, Action, and Exchange;
nondefault OpenClose, ShortSaleSlot, DesignatedLocation, or ExemptCode is
rejected unless the request uses a full order/protobuf leg.

Structural invariants are separate: negative conIDs, one-leg BAGs, non-BAG
delta-neutral blocks, open/close values outside 0..3, negative explicit exempt
codes, and surrounding SecurityID whitespace fail before encoding. Empty BAG
legs remain valid for contract lookup.

### 1.4 Unimplemented official callbacks

These are known official EWrapper callbacks with no ibkr-go message ID:

- `tickEFP` — EFP tick pricing (no live data observed)
- `orderBound` — order-bound notification
- `connectAck` — TWS-specific connection ack
- `deltaNeutralValidation` — delta-neutral validation callback
- `verifyMessageAPI` / `verifyCompleted` / `verifyAndAuthMessageAPI` / `verifyAndAuthCompleted` — internal auth

**Action:** Probe the live gateway for each. If the gateway never sends them,
mark out_of_scope. If it does, implement and freeze.

## 2. Order Type Matrix

Every order type × applicable TIF × applicable action × applicable security.

### 2.1 Order types (19 types)

| Order Type | STK | FUT | CASH | OPT | BAG | Live Test | Capture | Transcript |
|-----------|-----|-----|------|-----|-----|-----------|---------|------------|
| MKT | fill | fill | — | blocked | — | yes | yes | yes |
| LMT | rest+fill | rest | rest | blocked | blocked | yes | yes | yes |
| STP | rest | — | — | — | — | yes | yes | partial |
| STP LMT | rest | — | — | — | — | yes | yes | partial |
| TRAIL | rest | — | — | — | — | yes | yes | no |
| TRAIL LIMIT | rest | — | — | — | — | yes | yes | no |
| MIT | rest | — | — | — | — | yes | yes | no |
| LIT | rest | — | — | — | — | yes | yes | no |
| MTL | fill | — | — | — | — | yes | yes | no |
| REL | rest | — | — | — | — | yes | yes | no |
| MOC | rest | — | — | — | — | yes | no | no |
| LOC | timeout | — | — | — | — | partial | no | no |
| MOO | timeout | — | — | — | — | no | no | no |
| LOO | timeout | — | — | — | — | no | no | no |
| PEG MKT | rest | — | — | — | — | yes | no | no |
| PEG PRI | rest | — | — | — | — | yes | no | no |
| PEG MID | rest | — | — | — | — | yes | no | no |
| PEG BEST | rest | — | — | — | — | yes | no | no |
| PEG BENCH | rest | — | — | — | — | yes | no | no |

**Gap:** 11 order types have no transcript. All FUT/CASH/OPT columns except
basics are untested. MOO/LOO need pre-market timing.

### 2.2 Time-in-force (7 TIF values)

| TIF | Tested | Gap |
|-----|--------|-----|
| DAY | yes (default) | |
| GTC | yes | live capture + replay promoted; reconnect persistence still needed |
| IOC | yes | |
| FOK | yes | |
| GTD | partial | live capture exists; expiry behavior replay still needed |
| OPG | no | need pre-market run (MOO/LOO use this) |
| DTC | no | niche; probe whether gateway accepts it |

### 2.3 Order actions

| Action | Tested | Gap |
|--------|--------|-----|
| BUY | yes | |
| SELL | yes | |
| SSHORT | no | needs short-sale entitlement or specific account type |
| SLONG | no | needs an institutional long/short account segment |

## 3. Order Attributes

Every `Order` struct field must be exercised in at least one scenario.

### 3.1 Core fields (tested)

Action, OrderType, Quantity, LmtPrice, AuxPrice, TIF, Account, Transmit,
ParentID, OCA, DisplaySize, OutsideRTH, Algorithm, and Conditions. Order IDs
and the what-if flag are operation-owned and therefore are not `Order` fields.

### 3.2 Untested fields

| Field | Type | Scenario Needed |
|-------|------|-----------------|
| GoodAfterTime | string | place GAT order, verify no fill before time |
| GoodTillDate | string | place GTD order, verify persists then expires |
| AllOrNone | *bool | AON limit buy, verify fill-or-nothing |
| MinQty | decimal | minimum fill quantity constraint |
| PercentOffset | decimal | REL order with percent offset |
| TrailingPercent | decimal | TRAIL with percent instead of dollar |
| TriggerMethod | int | explicit trigger method override |
| OrderRef | string | custom ref string echo in OpenOrder |
| IncludeOvernight | bool | eligible SMART-routed DAY placement/open echoes, plus protobuf completed-order presence |
| Scale.InitialLevelSize | int | scale order: initial level |
| Scale.SubsequentLevelSize | int | scale order: subsequent levels |
| Scale.PriceIncrement | decimal | scale order: price steps |
| Scale.Table | string | predefined scale table |
| Scale.ActiveStartTime | string | time-activated order |
| Scale.ActiveStopTime | string | time-deactivated order |
| Hedge.Type | HedgeType | delta/beta/FX/pair hedge |
| Hedge.Param | string | hedge parameter value |
| CashQty | decimal | forex cash quantity mode |
| Hedge.DisableAutomaticPrice | *bool | hedge pricing override |
| UsePriceMgmtAlgo | *bool | IB price management |
| ManualOrderTime | string | regulatory compliance |
| AdvancedErrorOverride | string | override advanced order validation |
| Adjustment.OrderType | OrderType | volatility order adjustment |
| Adjustment.TriggerPrice | decimal | adjusted order trigger |
| LmtPriceOffset | *decimal | order-level limit-price offset |
| Adjustment.StopPrice | decimal | adjusted stop price |
| Adjustment.StopLimitPrice | decimal | adjusted stop-limit price |
| Adjustment.TrailingAmount | decimal | adjusted trailing amount |
| Adjustment.TrailingUnit | int | dollar vs percent unit |

### 3.3 Combo/multi-leg fields

| Field | Scenario Needed |
|-------|-----------------|
| Contract.ComboLegs | option vertical is live accepted/cancelled; add STK combo, ratios, calendar, iron condor |
| Order.Combo.LegPrices | typed decimal pointer/source-law encoding landed; live nondefault per-leg pricing remains |
| Order.Combo.SmartRouting | NonGuaranteed BAG acceptance/cancel is live-attested; completed-order echo and additional nondefault routing variants remain |

## 4. Order Conditions

| Condition Type | ID | Tested | Scenario Needed |
|---------------|-----|--------|-----------------|
| Price | 1 | yes | already done (AAPL price <= $1) |
| Time | 3 | yes | OR conjunction and cancel-order behavior remain |
| Margin | 4 | yes | threshold variants remain |
| Execution | 5 | yes | alternate sec type/exchange variants remain |
| Volume | 6 | yes | threshold variants remain |
| Percent-change | 7 | yes | threshold variants remain |

**Cross-cutting:** AND/OR conjunction, multiple conditions, conditionsIgnoreRTH,
conditionsCancelOrder.

## 5. Algo Strategies

| Strategy | Tested | Scenario Needed |
|----------|--------|-----------------|
| Adaptive (Normal) | yes | already done |
| Adaptive (Urgent) | no | priority=Urgent variant |
| Adaptive (Patient) | no | priority=Patient variant |
| TWAP | no | time-weighted average price |
| VWAP | no | volume-weighted average price |
| ArrivalPx | no | arrival price algo |
| DarkIce | no | dark pool seeking |
| AccumDist | no | accumulate/distribute |
| Inline | no | inline algo |
| Close | no | market-on-close algo |
| PctVol | no | percent of volume |
| BalanceImpactRisk | no | balance impact and risk |
| MinImpact | no | minimize impact |
| AD | no | Jefferies algo |

**Note:** Algo availability depends on account entitlements and order routing.
Probe each: place → check for rejection or acceptance.

## 6. Security Types

| SecType | Constant | Live Test | Capture | Transcript | Blocker |
|---------|----------|-----------|---------|------------|---------|
| STK | SecTypeStock | yes | yes | yes | — |
| OPT | SecTypeOption | partial | yes | no | 2026-04-15 broad chain probe timed out; use narrower qualified option probe or OPRA |
| FUT | SecTypeFuture | yes | yes | yes | MES futures buy/flatten replay promoted |
| FOP | SecTypeFutureOption | partial | yes | no | 2026-04-15 broad probe timed out; qualify concrete FOP |
| CASH | SecTypeForex | yes | yes | no | — |
| BAG | SecTypeCombo | blocked | blocked | no | depends on OPT |
| BOND | SecTypeBond | yes | yes | yes | Apple issuer lookup and message-18 replay promoted; order/data permissions remain |
| CFD | SecTypeCFD | yes | yes | no | order permissions still unknown |
| WAR | SecTypeWarrant | partial | yes | no | 2026-04-15 probe timed out; needs concrete warrant |
| IND | SecTypeIndex | yes | yes | partial | read-only (no orders) |
| CRYPTO | SecTypeCrypto | yes | yes | no | order permissions still unknown |
| FUND | SecTypeFund | yes | yes | no | mutual-fund order path still unknown |
| BILL | SecTypeBill | yes | yes | no | placeholder returned real code 200; replace with concrete bill |
| CMDTY | SecTypeCommodity | yes | yes | no | order/data permissions still unknown |
| CONTFUT | SecTypeContFuture | yes | yes | no | read-only continuous future details |

**Action per blocked type:** Attempt `Contracts().Details` to get the gateway's
rejection. Freeze the error as a `blocked` transcript.

## 7. Complex Combinations

These test multi-feature interactions that can't be covered by single-feature
tests.

### 7.1 Order lifecycle combinations

| Scenario | Status | What It Tests |
|----------|--------|---------------|
| Bracket: parent MKT + TP LMT + SL STP | live test | parent fill → children activate |
| Bracket: trigger TP → SL auto-cancels | live test | sibling OCA cancellation |
| OCA: one fills, peers cancel | live test | group semantics |
| Replace resting to fill | live test | price modification → fill |
| Cancel after fill (rejected) | live test | terminal state protection |
| WhatIf margin preview | live test | no execution, commission data |
| Reconnect with active orders | replay promoted | `api_reconnect_active_order_aapl.txt` freezes live GTC visibility/cancel after reconnect; `api_order_handle_reconnect_cancel_aapl.txt` freezes original handle Gap/RecoveryRequired + cancel |
| Reconnect with active subscriptions + orders | no | mixed lifecycle across reconnect |
| Multi-client same account | replay promoted | `api_cross_client_cancel_aapl.txt` freezes client ID 2 observing/cancelling client ID 1 order |
| Client ID 0 order observation | replay promoted | `api_client_id0_order_observation_aapl.txt` freezes client ID 0 all-open-orders observation/cancel |
| Place from client A, cancel from client B | replay promoted | `api_cross_client_cancel_aapl.txt` |
| Order + quote + PnL concurrent | live test | multiplexed subscriptions + orders |
| Rapid fire 10 orders + CancelAll | live test | throughput stress |
| Concurrent modify + cancel race | live test | no panic/deadlock |

### 7.2 Multi-leg strategies

| Strategy | Securities | Status |
|----------|-----------|--------|
| Vertical call spread | OPT+OPT | blocked (OPT permissions) |
| Vertical put spread | OPT+OPT | blocked |
| Iron condor | 4 OPT legs | blocked |
| Calendar spread | OPT+OPT (different expiry) | blocked |
| Butterfly | 3 OPT legs | blocked |
| Straddle | OPT+OPT (C+P same strike) | blocked |
| Strangle | OPT+OPT (C+P different strike) | blocked |
| Ratio spread | OPT legs with unequal ratios | blocked |
| Stock pairs | STK+STK (buy A, sell B) | no — needs 2 contracts |
| Futures spread | FUT+FUT | no — needs 2 contracts |
| Conversion/reversal | STK+OPT | blocked |

### 7.3 Campaign workflows

| Campaign | Status | Steps |
|----------|--------|-------|
| Scale-in + protective stop + flatten | partial replay promoted | `api_scale_in_campaign_aapl.txt` covers 2 buys + STP trigger; cancel/flatten tail timed out in source capture |
| Buy + immediate flatten | live test | BUY MKT → SELL MKT |
| Algorithmic campaign | live test | subs + buys + modify + flatten |
| Pairs trading | no | BUY AAPL + SELL MSFT simultaneously |
| Options wheel | blocked | sell put → assignment → sell call |
| Multi-timeframe | partial | real-time bars + order placement |
| Dollar-cost averaging | no | repeated buys at intervals |
| Stop-loss management | no | move stop as price advances |
| Bracket with trailing stop-loss | no | parent + TP + TRAIL SL |

## 8. Market Data Coverage

### 8.1 Quote data

| Data Type | Tested | Gap |
|-----------|--------|-----|
| Live (type 1) | entitlement error | paper account lacks live data |
| Frozen (type 2) | set_type capture | no stream observed |
| Delayed (type 3) | yes | |
| Delayed-Frozen (type 4) | set_type capture | no stream observed |
| Generic ticks (all families) | partial | Mark price, shortable, and volume rate live-attested; RTVolume, trade count/rate, news, dividend, and newer families remain |
| Option computation ticks | calc tests | streaming option ticks on OPT quotes |

### 8.2 Historical data

| Variant | Tested | Gap |
|---------|--------|-----|
| TRADES bar sizes (1min-1month) | partial | need all 12 bar sizes |
| BID/ASK/MIDPOINT/ADJUSTED_LAST | partial | need each whatToShow |
| Keep-up-to-date | yes | edge cases |
| Historical schedule | yes | non-US exchanges |
| Head timestamp | yes | |
| Histogram | yes | |
| Historical ticks (midpoint) | yes | |
| Historical ticks (bid/ask) | yes | |
| Historical ticks (last) | yes | |
| Timezone handling | yes | more zone combinations |

### 8.3 Real-time and tick-by-tick

| Variant | Tested | Gap |
|---------|--------|-----|
| TRADES 5-second bars | yes | |
| BID_ASK bars | no | |
| MIDPOINT bars | no | |
| Tick-by-tick Last | yes | |
| Tick-by-tick AllLast | no | |
| Tick-by-tick BidAsk | yes | |
| Tick-by-tick MidPoint | yes | |

### 8.4 Market depth

| Variant | Tested | Gap |
|---------|--------|-----|
| Regular depth (L1) | yes | |
| Smart depth (L2) | yes | |
| Depth exchanges | yes | |
| Entitlement error | transcript | live error capture |
| Insert/update/delete ops | yes | |
| Market maker names | no | specific exchanges only |

## 9. Account and Portfolio Coverage

| Capability | Tested | Gap |
|-----------|--------|-----|
| Summary: all tags | partial | full tag set |
| Summary: streaming | yes | |
| Summary: two concurrent | yes | |
| Positions: empty | yes | |
| Positions: multi-asset | partial | post-trade with OPT/FUT positions |
| Positions multi: model variants | partial | |
| Account updates: streaming | yes | during active trading |
| Account updates multi | yes | |
| PnL: account-level | yes | |
| PnL: single-position | yes | with open position |
| Family codes | yes | multi-family account |
| Completed orders: apiOnly filter | replay promoted | `api_completed_orders_variants_aapl` recaptured after the TRAIL LIMIT completed-order decode fix; apiOnly=false and apiOnly=true both reached `completedOrdersEnd` |
| Completed orders: full details | yes | exact source-aligned parser and public full-detail projection are frozen; rare nondefault branches remain live-attestation targets |

## 10. Error and Edge Case Coverage

### 10.1 Error codes to freeze

| Error | Status | Scenario |
|-------|--------|----------|
| 161 — cancel not in cancellable state | replay promoted | safety re-cancel stays a session notice; hedge replay freezes 161-before-Cancelled race |
| 162 — historical data pacing | partial | rapid historical requests |
| 200 — no security definition | yes | |
| 201 — order rejected | yes | |
| 202 — order cancelled | yes | |
| 320 — error reading request | observed | malformed request |
| 354 — no market data subscription | yes | |
| 399 — order message error | no | |
| 504 — not connected | library handles | |
| 10089 — live data not available | observed | paper account + live type |
| 10148 — cancel already cancelled | observed | |
| 10168 — market data not subscribed | observed | |
| 10187 — different IP session | observed | |
| 10197 — competing live session | observed | |
| 1100 — connectivity lost | yes (transcript) | |
| 1101 — connectivity restored (data lost) | yes (transcript) | |
| 1102 — connectivity restored (data maintained) | yes (transcript) | |
| 2104/2106/2108 — farm status OK | yes | |
| 2103/2105/2107 — farm status connecting | yes | |

### 10.2 Protocol edge cases

| Edge Case | Status | Scenario |
|-----------|--------|----------|
| Partial TCP frame read | yes (transcript) | split message |
| Message split across frames | yes (transcript) | |
| Disconnect mid-oneshot | yes (transcript) | |
| Disconnect mid-subscription | yes (transcript) | |
| Slow consumer backpressure | yes (transcript) | |
| Context cancel during request | yes (transcript) | |
| Singleton limit rejection | yes (transcript) | |
| Bootstrap with reordered messages | yes (transcript) | |
| Bootstrap missing next_valid_id | yes (transcript) | |
| Bootstrap missing managed_accounts | yes (transcript) | |
| Concurrent one-shots | yes (transcript) | |
| Reconnect multi-cycle | yes (transcript) | |
| Reconnect with active order handle | replay promoted | GTC order survives reconnect; original in-memory handle emits Gap/RecoveryRequired and cancels after reconnect |
| Reconnect with open-orders observer | partial | client/all open-orders after reconnect captured and replay promoted |
| Server version negotiation edge | no | near-boundary version |

### 10.3 Library API edge cases

| Edge Case | Status |
|-----------|--------|
| OrderHandle.Close (detach without cancel) | transcript |
| OrderHandle after terminal (no-op) | live test |
| Subscription close before first event | transcript |
| Cancel after Done | live test |
| Replace after Done | live test |
| Place with Transmit=false, then cancel | live test |
| Place with Transmit=false, then transmit | replay promoted |
| Two subscriptions same contract | replay promoted |
| Subscribe, disconnect, resume | transcript (quotes, bars) |

## 11. Execution Phases

### Phase 1: Fix and freeze current state

- [x] Fix cancel_order (CME_TAGGING_FIELDS)
- [x] Add cancel regression tests
- [x] Tighten live test cancel assertions
- [x] Promote what-if, scale-in, forex lifecycle, OCA, and bracket transcripts
  from live captures. The public `Orders().Preview` replay replaced the failed
  2026-04-14 what-if attempts and was live-revalidated at server version 206
  on 2026-07-10.
- [x] Record focused public direct-cancel evidence at server version 206.
- [x] Freeze `PendingCancel` before terminal `Cancelled` in
  `api_order_direct_cancel_aapl`; retain only the grounded
  `direct_cancel_order.txt` focused replay.

### Phase 2: TIF and order attribute expansion

- [ ] GTC rest/cancel (verify persists across reconnect)
- [ ] GTD rest/cancel with GoodTillDate
- [ ] TrailingPercent variant
- [ ] AllOrNone, MinQty
- [ ] GoodAfterTime
- [ ] OrderRef echo verification
- [ ] Adaptive Urgent and Patient variants

Progress: `api_tif_attribute_matrix_aapl` was added and live-captured on
2026-04-15 (`e6601dcc2abfd001`). `api_tif_attribute_matrix_aapl.txt` promotes
the GTC rest/cancel and trailing-percent fill slices. Remaining Phase 2 work is
focused replay promotion for the other accepted/rejected attributes and
reconnect/expiry assertions.

### Phase 3: Condition type expansion

- [ ] Time condition (fire after specific time)
- [ ] Volume condition
- [ ] Percent-change condition
- [ ] Margin condition (may be hard to trigger on paper)
- [ ] Execution condition
- [ ] Multiple conditions with AND/OR

### Phase 4: Security type expansion

- [ ] Subscribe to OPRA data on paper account
- [ ] OPT: qualify, rest/cancel, buy/sell round-trip
- [ ] BAG: vertical spread, iron condor
- [ ] FOP: option on future
- [ ] Probe BOND, CFD, WAR, CRYPTO with ContractDetails
- [ ] Freeze rejection errors for blocked types
- [ ] CONTFUT: continuous future data

### Phase 5: Reconnect with active state

- [x] Reconnect with resting order → verify handle resumes
- [ ] Reconnect with filled order → verify execution delivered
- [ ] Reconnect with active quote subscription → verify ordered Gap followed by Restored or Resubscribed
- [ ] Reconnect with active PnL subscription
- [ ] Reconnect during order placement

Progress: `api_reconnect_active_order_aapl` was live-captured on 2026-04-15
(`9d72a4711c25c788`) and promoted as `api_reconnect_active_order_aapl.txt`.
It freezes a real GTC order visible after reconnect and direct cancellation
from the reconnected client. `api_order_handle_reconnect_cancel_aapl.txt`
adds deterministic replay coverage for the original in-memory `OrderHandle`
emitting Gap/RecoveryRequired and cancelling after reconnect, grounded in the same live
capture.

### Phase 6: Advanced order features

- [ ] Scale orders (init/subs/increment)
- [ ] Hedge orders (delta, beta, FX, pair)
- [ ] Delta-neutral extensions
- [ ] Adjusted orders (stop/trailing adjustments)
- [ ] Volatility orders
- [ ] TWAP/VWAP/ArrivalPx algos
- [ ] FA allocation fields (needs FA account)

Progress: `api_algo_variants_aapl` is now an executable live campaign for
Adaptive Urgent/Patient, TWAP, VWAP, ArrivalPx, DarkIce, AccumDist, Inline,
Close, PctVol, BalanceImpactRisk, MinImpact, and Jefferies-style AD variants.
It was live-captured on 2026-04-15 (`1855e2554d7de3ae`): Adaptive Urgent,
Adaptive Patient, VWAP, ArrivalPx, AccumDist, Close, and PctVol were accepted
and cancelled; TWAP and several unavailable variants produced real Gateway
rejections or no-status cleanup evidence.

### Phase 7: Market data completeness

- [x] Duplicate same-contract quote subscriptions (`api_duplicate_quote_subscriptions_aapl` captured 2026-04-15, replay-promoted from `84f1e78a18616e0f`)
- [x] Public generic-tick matrix preserves observed mark-price, shortable, volume-rate, and delayed-timestamp callbacks (`api_generic_tick_matrix_aapl`, 2026-07-09 raw `5c40260d783971d2`)
- [x] Contract-specific BRFG `tickNews` callback through the public quote stream (`api_tick_news_aapl_probe`, exact server-version-201 decoder frame `e3e1901503f7d1dc` and exact server-version-207 public lifecycle `739f3c2caa3d5379`)
- [ ] All historical bar sizes (1sec through 1month)
- [ ] All whatToShow values
- [ ] Real-time bars BID_ASK and MIDPOINT
- [ ] Tick-by-tick AllLast
- [ ] All generic tick families
- [ ] Regulatory snapshot
- [ ] Positive tickEFP callback (typed implementation is source-law frozen; `tick_efp_probe` needs an entitled active single-stock future)
- [ ] Positive delta-neutral validation callback (typed implementation is source-law frozen; needs an accepted BAG request)
- [ ] Positive odd-lot ticks 105-110 (generic tick 787 is exact-sv225 negative-live proven under the current entitlement)

### Phase 8: Complete error catalog

- [ ] Freeze every observed error code as a transcript
- [ ] Probe for unobserved error codes
- [ ] Entitlement error for every feature family
- [ ] Pacing limits for historical data
- [ ] Order rejection for every validation rule

### Phase 9: Protocol edge cases

- [x] Transmit=false then transmit (modify to transmit)
- [x] Two subscriptions same contract (independent delayed streams)
- [x] Cross-client order observation (client_id=0)
- [x] Supported server boundaries (199 rejected, 200 accepted, 207 accepted, 208 rejected)
- [x] Bond contract details callback shape

Progress: `api_transmit_false_then_transmit_aapl` was live-captured on
2026-04-15 (`003abb59dfced542`) and replay-promoted. Duplicate quote
subscriptions were live-captured and replay-promoted the same day
(`84f1e78a18616e0f`). Client ID 0
and cross-client order observation/cancel were live-captured and replay-promoted
from `5ff9cdc0f6f9b500` and `fcb7e811624e4aa9`.

### Phase 10: Multi-feature campaigns

- [x] Pairs trading workflow (`api_pairs_trading_aapl_msft` captured 2026-04-15, replay-promoted from `0dc806f7bb0868e8`; aggressive 500-share live run/capture also completed with `86dc8f389a457efc`; source execution-query tail timed out)
- [ ] Bracket with trailing stop-loss (`api_bracket_trailing_stop_aapl` captured 2026-04-15, replay-promoted from `2c0453360020a3ad`; current replay freezes live code 328 for TRAIL child under a market parent, so a valid limit/stop-limit parent variant remains)
- [x] Dollar-cost averaging (`api_dollar_cost_averaging_aapl` captured 2026-04-15, replay-promoted from `296bdf662eb84e30`; source execution-query tail timed out)
- [x] Stop-loss management (move as price advances) (`api_stop_loss_management_aapl` captured 2026-04-15, replay-promoted from `a563cafd26e366be`)
- [x] Rapid-fire global cancel (`api_stress_rapid_fire_aapl` re-captured 2026-07-11 and exact-raw replay-promoted from `d66113abb8382887`)
- [x] Scale-in campaign (`api_scale_in_campaign_aapl` captured 2026-04-14, replay-promoted from `63db2db7cba21b68`; replay covers two fills plus protective STP trigger, while flatten/execution-query tail timed out)
- [ ] Full reconciliation (positions + executions + PnL match)
- [ ] Options wheel (when OPT available)

Progress: aggressive campaign defaults now use 500-share equity clips, 100-share
standard probes, 5-lot option/BAG orders, and 100,000 EUR.USD forex notional.
The 2026-04-15 aggressive pairs run exposed a live `reqExecutions` layout bug;
server_version 200 requires `lastNDays=2147483647` and `specificDatesCount=0`,
now frozen by codec/testhost regression coverage and a live one-share
round-trip that returned 76 execution updates.

## 12. Blockers and Prerequisites

| Blocker | Impact | Resolution |
|---------|--------|------------|
| OPRA data subscription | OPT, BAG, FOP tests | Subscribe on paper account |
| FA account | FA-001, FA-002, AORD-010 | Access to FA paper account |
| Bond data | BOND contract/order tests | Bond market data subscription |
| CFD permissions | CFD tests | Enable on paper account |
| Crypto permissions | CRYPTO tests | Enable on paper account |
| Pre-market access | MOO/LOO/OPG tests | Run during 4:00-9:30 ET |
| Market closed | Fill-dependent tests | Run during 9:30-16:00 ET |
| Short-sale permission | SSHORT action | Account configuration |
| Position for exercise | OPT-002 | Buy option → hold → exercise |

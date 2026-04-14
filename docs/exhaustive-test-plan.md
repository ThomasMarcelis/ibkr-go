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

## 1. Protocol Messages

Every outbound and inbound message ID must have at least one live-grounded
scenario.

### 1.1 Outbound (client → server): 56 message IDs

| ID | Name | Live | Capture | Transcript | Gap |
|----|------|------|---------|------------|-----|
| 1 | reqMktData | yes | yes | yes | snapshot vs stream vs generic ticks |
| 2 | cancelMktData | yes | yes | yes | |
| 3 | placeOrder | yes | yes | yes | advanced fields (hedge, scale, delta-neutral) |
| 4 | cancelOrder | yes | yes | yes | **just fixed** — CME_TAGGING_FIELDS regression |
| 5 | reqOpenOrders | yes | yes | yes | |
| 6 | reqAccountUpdates | yes | yes | yes | |
| 7 | reqExecutions | yes | yes | yes | filter variants |
| 8 | reqIds | yes | yes | yes | |
| 9 | reqContractDetails | yes | yes | yes | BOND, FOP, IND, CRYPTO |
| 10 | reqMktDepth | yes | yes | yes | smart depth, entitlement error |
| 11 | cancelMktDepth | yes | yes | yes | |
| 12 | reqNewsBulletins | yes | yes | yes | |
| 13 | cancelNewsBulletins | yes | yes | yes | |
| 15 | reqAutoOpenOrders | yes | yes | yes | |
| 16 | reqAllOpenOrders | yes | yes | yes | |
| 18 | requestFA | yes | yes | partial | non-FA error frozen; FA-account path missing |
| 19 | replaceFA | no | no | no | **target** — needs FA account |
| 20 | reqHistoricalData | yes | yes | yes | schedule variant, more bar sizes |
| 21 | exerciseOptions | no | no | no | **target** — needs option position + OPT permissions |
| 22 | reqScannerSubscription | yes | yes | yes | |
| 23 | cancelScannerSubscription | yes | yes | yes | |
| 24 | reqScannerParameters | yes | yes | yes | |
| 25 | cancelHistoricalData | yes | yes | yes | |
| 49 | reqCurrentTime | yes | yes | yes | |
| 50 | reqRealTimeBars | yes | yes | yes | BID_ASK, MIDPOINT variants |
| 51 | cancelRealTimeBars | yes | yes | yes | |
| 52 | reqFundamentalData | yes | yes | partial | all report types, entitlement error |
| 53 | cancelFundamentalData | yes | partial | no | cancel mid-request |
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
| 84 | reqNewsArticle | no | no | no | **target** — needs article ID from historical news |
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
| 99 | reqCompletedOrders | yes | yes | partial | apiOnly filter, full detail |
| 100 | reqWSHMetaData | yes | yes | partial | entitlement error |
| 101 | cancelWSHMetaData | partial | partial | yes | |
| 102 | reqWSHEventData | yes | yes | partial | filter/date/portfolio variants |
| 103 | cancelWSHEventData | partial | partial | no | |
| 104 | reqUserInfo | yes | yes | yes | |

### 1.2 Inbound (server → client): 52 message IDs

All are exercised through the outbound scenarios above. Individual gaps:

| ID | Name | Gap |
|----|------|-----|
| 1 | tickPrice | EFP tick type never observed |
| 14 | newsBulletins | live capture exists; allMessages variant untested |
| 21 | tickOptionComputation | live calc scenarios exist; streaming OPT tick untested |
| 83 | newsArticle | no scenario (depends on outbound 84) |
| 101 | completedOrder | full field extraction deferred |
| 108 | historicalDataUpdate | keep-up-to-date exists; edge cases untested |

### 1.3 Unimplemented official callbacks

These are known official EWrapper callbacks with no ibkr-go message ID:

- `tickEFP` — EFP tick pricing (no live data observed)
- `tickNews` — news-aware tick (no live data observed)
- `orderBound` — order-bound notification
- `bondContractDetails` — bond-specific contract details
- `replaceFAEnd` — FA replace completion
- `connectAck` — TWS-specific connection ack
- `rerouteMktDataReq` / `rerouteMktDepthReq` — reroute suggestions
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
| GTC | no | need rest + cancel + verify persists after session |
| IOC | yes | |
| FOK | yes | |
| GTD | no | need GoodTillDate field + verify expiry |
| OPG | no | need pre-market run (MOO/LOO use this) |
| DTC | no | niche; probe whether gateway accepts it |

### 2.3 Order actions

| Action | Tested | Gap |
|--------|--------|-----|
| BUY | yes | |
| SELL | yes | |
| SSHORT | no | needs short-sale entitlement or specific account type |

## 3. Order Attributes

Every `Order` struct field must be exercised in at least one scenario.

### 3.1 Core fields (tested)

OrderID, Action, OrderType, Quantity, LmtPrice, AuxPrice, TIF, Account,
Transmit, ParentID, OcaGroup, OcaType, DisplaySize, OutsideRTH, WhatIf,
AlgoStrategy, AlgoParams, Conditions.

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
| ScaleInitLevelSize | int | scale order: initial level |
| ScaleSubsLevelSize | int | scale order: subsequent levels |
| ScalePriceIncrement | decimal | scale order: price steps |
| ScaleTable | string | predefined scale table |
| ActiveStartTime | string | time-activated order |
| ActiveStopTime | string | time-deactivated order |
| HedgeType | string | delta/beta/FX/pair hedge |
| HedgeParam | string | hedge parameter value |
| CashQty | decimal | forex cash quantity mode |
| DontUseAutoPriceForHedge | *bool | hedge pricing override |
| UsePriceMgmtAlgo | *bool | IB price management |
| ManualOrderTime | string | regulatory compliance |
| AdvancedErrorOverride | string | override advanced order validation |
| AdjustedOrderType | OrderType | volatility order adjustment |
| TriggerPrice | decimal | adjusted order trigger |
| LmtPriceOffset | decimal | adjusted limit offset |
| AdjustedStopPrice | decimal | adjusted stop price |
| AdjustedStopLimitPrice | decimal | adjusted stop-limit price |
| AdjustedTrailingAmount | decimal | adjusted trailing amount |
| AdjustableTrailingUnit | int | dollar vs percent unit |

### 3.3 Combo/multi-leg fields

| Field | Scenario Needed |
|-------|-----------------|
| ComboLegs | vertical spread, iron condor, calendar |
| OrderComboLegPrices | per-leg pricing |
| SmartComboRoutingParams | NonGuaranteed execution |

## 4. Order Conditions

| Condition Type | ID | Tested | Scenario Needed |
|---------------|-----|--------|-----------------|
| Price | 1 | yes | already done (AAPL price <= $1) |
| Time | 3 | no | order activates after specific time |
| Margin | 4 | no | order fires when margin cushion drops |
| Execution | 5 | no | order fires when another symbol trades |
| Volume | 6 | no | order fires when volume exceeds threshold |
| Percent-change | 7 | no | order fires on % price change |

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
| OPT | SecTypeOption | blocked | blocked | no | OPRA data subscription |
| FUT | SecTypeFuture | yes | yes | no | — |
| FOP | SecTypeFutureOption | no | no | no | OPT permissions |
| CASH | SecTypeForex | yes | yes | no | — |
| BAG | SecTypeCombo | blocked | blocked | no | depends on OPT |
| BOND | SecTypeBond | no | no | no | bond data subscription |
| CFD | SecTypeCFD | no | no | no | CFD permissions |
| WAR | SecTypeWarrant | no | no | no | non-US exchange access |
| IND | SecTypeIndex | partial (quotes) | partial | partial | read-only (no orders) |
| CRYPTO | SecTypeCrypto | no | no | no | crypto trading permissions |
| FUND | SecTypeFund | no | no | no | fund trading permissions |
| BILL | SecTypeBill | no | no | no | treasury access |
| CMDTY | SecTypeCommodity | no | no | no | commodity permissions |
| CONTFUT | SecTypeContFuture | no | no | no | continuous future data |

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
| Modify resting to fill | live test | price modification → fill |
| Cancel after fill (rejected) | live test | terminal state protection |
| WhatIf margin preview | live test | no execution, commission data |
| Reconnect with active orders | no | **critical gap** — handle survives disconnect |
| Reconnect with active subscriptions + orders | no | mixed lifecycle across reconnect |
| Multi-client same account | no | client_id isolation |
| Client ID 0 order observation | no | observes all clients' orders |
| Place from client A, cancel from client B | no | cross-client cancel |
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
| Scale-in + protective stop + flatten | live test | 2 buys, STP, cancel, flatten |
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
| Generic ticks (all families) | partial | RTVolume, shortable, news, dividend, fundamental ratio |
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
| Completed orders: apiOnly filter | no | |
| Completed orders: full details | no | deferred |

## 10. Error and Edge Case Coverage

### 10.1 Error codes to freeze

| Error | Status | Scenario |
|-------|--------|----------|
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
| Reconnect with active order handle | no | **critical gap** |
| Reconnect with open-orders observer | no | **critical gap** |
| Server version negotiation edge | no | near-boundary version |

### 10.3 Library API edge cases

| Edge Case | Status |
|-----------|--------|
| OrderHandle.Close (detach without cancel) | transcript |
| OrderHandle after terminal (no-op) | live test |
| Subscription close before first event | transcript |
| Cancel after Done | live test |
| Modify after Done | live test |
| Place with Transmit=false, then cancel | live test |
| Place with Transmit=false, then transmit | no |
| Two subscriptions same contract | no |
| Subscribe, disconnect, resume | transcript (quotes, bars) |

## 11. Execution Phases

### Phase 1: Fix and freeze current state

- [x] Fix cancel_order (CME_TAGGING_FIELDS)
- [x] Add cancel regression tests
- [x] Tighten live test cancel assertions
- [ ] Promote pending transcripts (bracket, OCA, rest_cancel)
- [ ] Record fresh captures for all scenarios with fixed cancel
- [ ] Update `cancel_order.txt` and `direct_cancel_order.txt` to include PendingCancel

### Phase 2: TIF and order attribute expansion

- [ ] GTC rest/cancel (verify persists across reconnect)
- [ ] GTD rest/cancel with GoodTillDate
- [ ] TrailingPercent variant
- [ ] AllOrNone, MinQty
- [ ] GoodAfterTime
- [ ] OrderRef echo verification
- [ ] Adaptive Urgent and Patient variants

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

- [ ] Reconnect with resting order → verify handle resumes
- [ ] Reconnect with filled order → verify execution delivered
- [ ] Reconnect with active quote subscription → verify Gap/Resumed
- [ ] Reconnect with active PnL subscription
- [ ] Reconnect during order placement

### Phase 6: Advanced order features

- [ ] Scale orders (init/subs/increment)
- [ ] Hedge orders (delta, beta, FX, pair)
- [ ] Delta-neutral extensions
- [ ] Adjusted orders (stop/trailing adjustments)
- [ ] Volatility orders
- [ ] TWAP/VWAP/ArrivalPx algos
- [ ] FA allocation fields (needs FA account)

### Phase 7: Market data completeness

- [ ] All historical bar sizes (1sec through 1month)
- [ ] All whatToShow values
- [ ] Real-time bars BID_ASK and MIDPOINT
- [ ] Tick-by-tick AllLast
- [ ] All generic tick families
- [ ] Regulatory snapshot
- [ ] tickEFP probe

### Phase 8: Complete error catalog

- [ ] Freeze every observed error code as a transcript
- [ ] Probe for unobserved error codes
- [ ] Entitlement error for every feature family
- [ ] Pacing limits for historical data
- [ ] Order rejection for every validation rule

### Phase 9: Protocol edge cases

- [ ] Transmit=false then transmit (modify to transmit)
- [ ] Two subscriptions same contract (singleton behavior)
- [ ] Cross-client order observation (client_id=0)
- [ ] Server version boundary testing (sv=191 vs sv=192)
- [ ] Bond contract details callback shape

### Phase 10: Multi-feature campaigns

- [ ] Pairs trading workflow
- [ ] Bracket with trailing stop-loss
- [ ] Dollar-cost averaging
- [ ] Stop-loss management (move as price advances)
- [ ] Full reconciliation (positions + executions + PnL match)
- [ ] Options wheel (when OPT available)

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

# Live Test Execution Tracker

Companion to [`live-coverage-matrix.md`](live-coverage-matrix.md). Tracks every
live test run against IB Gateway paper account, what passed, what failed, what
was fixed, and what remains untested.

Last updated: 2026-06-10 (readonly-live gateway 127.0.0.1:4001 and paper-dev
gateway 127.0.0.1:4002, both server_version 200).

## Current Campaign Contract

Future live sweeps use two Gateway roles. `readonly-live` points at the
real-money, read-only Gateway for market data, account, historical, scanner,
news, WSH, and entitlement evidence. `paper-dev` points at the disposable paper
Gateway for every scenario that places, modifies, cancels, reconnects around,
or flattens orders.

Before a sweep, run:

```bash
go run ./cmd/ibkr-doctor -role readonly-live
go run ./cmd/ibkr-doctor -role paper-dev
```

Capture scripts choose the role from `cmd/ibkr-capture` scenario metadata, so
mixed batches cannot accidentally send paper-order scenarios to the read-only
Gateway. Every new row below should record the role, server version, account
class, and promoted transcript or remaining blocker.

## Gateway Bring-Up Runs

### 2026-06-10

Campaign preflight drift gate. Both local Gateway roles connected with
`server_version=200`, so the pinned v200 codec baseline still holds.

- `go run ./cmd/ibkr-doctor -role readonly-live` connected to
  `127.0.0.1:4001`: `state=Ready`, `server_version=200`, `next_valid_id=1`,
  one managed account. `CurrentTime` returned `2026-06-10T19:32:07Z`. AAPL
  quote probe returned real IBKR code 10089 (live API market-data
  subscription unavailable; delayed offered), matching the standing
  entitlement ledger. Note the Gateway alternates between 10089 and 10168
  across runs depending on whether it offers delayed data: the 2026-05-25
  15:58 continuation check recorded 10168 for the same probe.
- `go run ./cmd/ibkr-doctor -role paper-dev` connected to `127.0.0.1:4002`:
  `state=Ready`, `server_version=200`, `next_valid_id=2`, one managed
  account. Delayed market data type requested; AAPL delayed quote received
  (bid/ask/last populated).
- [IBKR API Software](https://interactivebrokers.github.io/) re-checked:
  the download page still lists API Latest 10.47 (2026-05-20) and Stable
  10.45 (2026-03-30), recommended Gateway 1045+. The official
  [production release notes](https://www.ibkrguides.com/releasenotes/prod-2026.htm)
  already carry a 10.48 section: `reqOpenOrders` will include de-activated
  orders. That is a pending behavioral change to `Orders().Open` result
  sets, not a wire-shape change, and both local Gateways still negotiate
  `server_version 200`, so capture work proceeds on the pinned baseline
  with 10.48 recorded as a drift watch item.

### 2026-05-25

Both local Gateway roles connected with `server_version=200`.

- `readonly-live` diagnostic connected and `CurrentTime` passed. AAPL quote
  returned real IBKR code 10089: live API market-data subscription unavailable;
  delayed data was offered by Gateway.
- `readonly-live` broad `IBKR_LIVE=1 ... -run '^TestLive'` passed. The run
  logged current account/session blockers instead of treating them as library
  regressions: completed-orders silence until context deadline, historical
  ticks code 10187 for missing ISLAND market-data permission, fundamentals
  code 10358, WSH code 10276, and market-depth timeout with no rows.
- `paper-dev` diagnostic connected, requested delayed data, and received an
  AAPL delayed quote.
- `paper-dev` order smoke passed for limit rest/cancel, invalid order type,
  WhatIf, public limit cancel, and global cancel. `TestLiveOrderLimitRestCancel`
  observed code 399 because the order would not reach the exchange until
  `2026-05-26 09:30:00 US/Eastern`; the test now records that off-hours
  Gateway warning and relies on cleanup `CancelAll`.

Pre-window continuation check at 2026-05-25 15:58 Europe/Amsterdam, before the
planned Tuesday 2026-05-26 09:30 US/Eastern market-open campaign:

- `go run ./cmd/ibkr-doctor -role readonly-live -timeout 30s` connected to
  `127.0.0.1:4001`; `server_version=200`, `next_valid_id=1`, and one managed
  account were visible. `CurrentTime` returned `2026-05-25T13:58:39Z`. AAPL
  quote probing returned real IBKR code 10168: requested market data is not
  subscribed and delayed market data is not enabled.
- `go run ./cmd/ibkr-doctor -role paper-dev -timeout 30s` connected to
  `127.0.0.1:4002`; `server_version=200`, `next_valid_id=2`, and one managed
  account were visible. Delayed AAPL data was available (`type=Delayed`).
- `/tmp/ibkr-recorder`, `/tmp/ibkr-capture`, and `/tmp/ibkr-normalize` built
  successfully from the current worktree.
- `IBKR_CAPTURE_BATCH=trading-basic` resolves to
  `api_delayed_success_modify_aapl`, `api_ioc_fok_aapl`,
  `api_order_fill_aapl`, `api_order_rejects_aapl`,
  `api_order_rest_cancel_aapl`, and `api_order_type_matrix_aapl`; the
  executable catalog reports `paper-dev` for the whole set.
- Paper cleanup was checked without placing new orders:
  `IBKR_LIVE=1 IBKR_LIVE_ADDR=127.0.0.1:4002 go test . -run '^TestLiveOpenOrders$' -count=1 -timeout 1m -v`
  passed and logged
  `OpenOrdersSnapshot: 0 orders`.
- The MO-1 order smoke and MO-2 `trading-basic` capture batch were not started
  because the requested market-open window had not arrived. No new capture
  directories or capture hashes were produced in this continuation check.

## Live Test Results (client_live_order_test.go)

45 tests across 10 tiers. The full tier table below reflects the 2026-04-14
paper-order campaign; the 2026-05-25 bring-up reran the targeted paper smoke
against the role-aware `paper-dev` Gateway.

### Tier 1: Order Type Rest/Cancel

| Test | Order Type | Asset | Status | Notes |
|------|-----------|-------|--------|-------|
| TestLiveOrderLimitRestCancel | LMT | STK | pass | |
| TestLiveOrderStopRestCancel | STP | STK | pass | |
| TestLiveOrderStopLimitRestCancel | STP LMT | STK | pass | |
| TestLiveOrderTrailingStopRestCancel | TRAIL | STK | pass | fixed: BUY direction to prevent trigger |
| TestLiveOrderTrailingLimitRestCancel | TRAIL LIMIT | STK | pass | fixed: BUY direction |
| TestLiveOrderMITRestCancel | MIT | STK | pass | |
| TestLiveOrderLITRestCancel | LIT | STK | pass | |
| TestLiveOrderRelativeRestCancel | REL | STK | pass | |

### Tier 2: Immediate Fill

| Test | Order Type | TIF | Asset | Status | Notes |
|------|-----------|-----|-------|--------|-------|
| TestLiveOrderMarketBuyFill | MKT | DAY | STK | pass | |
| TestLiveOrderMarketableLimitFill | LMT | DAY | STK | pass (graceful) | exchange price protection cancels above-NBBO limit; test handles both fill and cancel |
| TestLiveOrderMarketToLimitFill | MTL | DAY | STK | pass | |
| TestLiveOrderIOCFill | LMT | IOC | STK | pass | IOC cancelled as expected |
| TestLiveOrderFOKFillOrReject | LMT | FOK | STK | pass | fillable FOK goes Inactive, unfillable also Inactive |

### Tier 3: Rejections

| Test | Scenario | Status | Notes |
|------|----------|--------|-------|
| TestLiveOrderRejectInvalidContract | bogus symbol | pass | |
| TestLiveOrderRejectInvalidType | type "FEELINGS" | pass | |
| TestLiveOrderCancelUnknownID | cancel ID=999999999 | pass | |
| TestLiveOrderDoubleCancelOrder | cancel after cancel | pass | fixed: drain Events() instead of waiting on Done() |
| TestLiveOrderOpenCloseTypesAcceptOrReject | MOC/LOC/MOO/LOO | pass | MOC accepted, LOC timeout, MOO/LOO context expired |
| TestLiveOrderPegFamiliesAcceptOrReject | PEG MKT/PRI/MID/BEST/BENCH | pass | 121s runtime, all 5 peg types tested |

### Tier 4: Modifications

| Test | Scenario | Status | Notes |
|------|----------|--------|-------|
| TestLiveOrderModifyFilledOrder | modify after fill | pass | fill via MKT, Modify returns nil (gateway ignores) |
| TestLiveOrderModifyLimitToFill | far LMT -> MKT | pass | |
| TestLiveOrderModifyQuantity | qty 5 -> 3 | pass | confirmed via OpenOrder echo |
| TestLiveOrderRapidModifications | 5 rapid price changes | pass | final price confirmed |

### Tier 5: Bracket Orders

| Test | Scenario | Status | Notes |
|------|----------|--------|-------|
| TestLiveBracketFillChildrenActivate | MKT parent + TP/SL | pass | children activate after parent fill |
| TestLiveBracketTriggerTakeProfit | TP modified to marketable | pass | SL auto-cancels via OCA |
| TestLiveBracketCancelBeforeTransmit | Transmit=false, cancel | pass | |

### Tier 6: OCA Groups

| Test | Scenario | Status | Notes |
|------|----------|--------|-------|
| TestLiveOCAFillCancelsOthers | marketable fills, peers cancel | pass (graceful) | marketable limit cancelled by exchange; test handles gracefully |
| TestLiveOCACancelAll | 2 resting, CancelAll | pass | drains all handles concurrently after global cancel |

### Tier 7: Multi-Asset

| Test | Asset | Status | Notes |
|------|-------|--------|-------|
| TestLiveOptionLimitRestCancel | OPT | skip | SecDefOptParams: request interrupted (missing option data permissions) |
| TestLiveOptionBuySellRoundTrip | OPT | skip | same permissions issue |
| TestLiveFutureLimitRestCancel | FUT (MES) | pass | conID=770561194, Jun 2026 |
| TestLiveFutureBuySellRoundTrip | FUT (MES) | pass | BUY 6994.25 / SELL 6994.00 |
| TestLiveForexLimitRestCancel | CASH (EUR.USD) | pass | goes Inactive (far-from-market forex not held) |
| TestLiveComboVerticalRestCancel | BAG | skip | depends on SecDefOptParams |

### Tier 8: Advanced Features

| Test | Feature | Status | Notes |
|------|---------|--------|-------|
| TestLiveOrderWhatIf | WhatIf=true | pass | no commission preview returned (account-dependent) |
| TestLiveOrderAdaptiveAlgo | Adaptive algo | pass | algo echo not observed in OpenOrder (paper gateway) |
| TestLiveOrderIceberg | DisplaySize=3 | pass | goes Inactive |
| TestLiveOrderOutsideRTH | OutsideRTH=true | pass | |
| TestLiveOrderConditionPrice | price condition <=1 | pass | |

### Tier 9: Stress

| Test | Scenario | Status | Notes |
|------|----------|--------|-------|
| TestLiveStressRapidFireTenOrders | 10 orders + CancelAll | pass | all 10 distinct IDs placed and terminal after global cancel |
| TestLiveStressConcurrentModifyCancel | modify+cancel race | pass | terminal or API race error, no panic or deadlock |
| TestLiveOrdersWithSubscriptions | orders + quote/PnL subs | pass | coexistence confirmed |

### Tier 10: Campaigns

| Test | Scenario | Status | Notes |
|------|----------|--------|-------|
| TestLiveAlgoScaleInWithStopLoss | 2 buys + stop + flatten | pass | position=3 after scale-in (includes residual) |
| TestLiveFillAndImmediateFlatten | MKT buy + sell | pass | 2026-04-15 rerun after reqExecutions layout fix returned 76 execution updates |
| TestLiveAlgorithmicCampaign | full lifecycle | pass | 3 buys + modify-to-fill + flatten, 212s |
| TestLiveCaptureHighSignalTradingScenarios/api_pairs_trading_aapl_msft | 500 AAPL buy + 500 MSFT short, then flatten both | pass | added in `cmd/ibkr-capture`; observed partial fills, commissions, and cleanup against server_version=200 |

## Bugs Found and Fixed

| Bug | Root Cause | Fix |
|-----|-----------|-----|
| Individual/global cancel dropped | sv>=192 cancel messages require CME tagging fields; old global cancel version field is ignored | Encode cancel_order and global_cancel with extOperator/manualOrderIndicator |
| Cancellation notice closed handles as errors | API code 202 is an order-cancel notice, not a placement failure | Route code 202 as a session notice and keep terminal status authoritative |
| DoubleCancelOrder timeout | Waited on Done() without draining Events(); cancel confirmations buffered | Replace Done() select with liveObserveOrder drain (8 locations) |
| MarketableLimitFill rejected | 1.20x anchor exceeds exchange price reasonability | Reduced to 1.03x; made non-fill graceful |
| Trailing stop tests trigger | SELL trail with liveFarSell triggers on rise | Changed to BUY direction with far trail price |
| Stop/stop-limit trigger | liveMarketableBuy too close to market for trigger price | Use liveFarSell (10x anchor) |
| AlgorithmicCampaign timeout | 120s context insufficient for 7+ operations | Increased to 180s, fresh context for flatten step |
| FillAndImmediateFlatten fatal | Executions query fatals when context expired | Changed to non-fatal log |
| `reqExecutions` account/symbol filters rejected by Gateway | Server-version 200 still requires version=3 and additionally requires `lastNDays` plus `specificDates` count. Omitting those fields produced real code 320 errors (`Trading Days` / `Server Id`) during aggressive pairs captures. | Encode `lastNDays=2147483647` and `specificDatesCount=0`; update testhost parsing and freeze the layout in `TestEncodeExecutionsRequestServer200Layout`. Live rerun of `TestLiveFillAndImmediateFlatten` returned execution updates. |

## Capture Scenarios Recorded

| Scenario | Date | Status |
|----------|------|--------|
| api_whatif_margin_aapl | 2026-04-14 | recorded, not promoted; `20260414T164207Z` produced no usable preview callback and cleanup timed out (`ac70de98ef2c239a`); `20260414T182608Z` returned live code 320 after the WhatIf place request (`e431bf7f0b84abd1`) |
| api_forex_lifecycle_eurusd | 2026-04-14 | recorded, verified; server_version=200, events sha256 prefix `641eab5c0e6909f7`; real paper-account code 201 leverage rejection |
| api_stress_rapid_fire_aapl | 2026-04-14 | recorded, verified; server_version=200, events sha256 prefix `69ee6be4cdf7d577` |
| api_scale_in_campaign_aapl | 2026-04-14 | recorded, verified; server_version=200, events sha256 prefix `63db2db7cba21b68`; two AAPL market fills plus protective stop-loss, with later flatten/executions/cleanup tail timing out |
| api_bracket_trigger_aapl | 2026-04-13 | recorded, verified; server_version=200, events sha256 prefix `682a1390b2acf04c`; bracket parent fill, child OCA echo, and real price-band cancel/reject on take-profit modify |
| api_oca_trigger_aapl | 2026-04-13 | recorded, verified; server_version=200, events sha256 prefix `2dc16869778bc497`; OCA group echo plus real price-band cancellation |
| api_ioc_fok_aapl | 2026-04-14 | recorded, not promoted; existing replay remains `20260413T184916Z`; recapture repeated IOC/FOK statuses but included quote code 320 and execution-query timeout (events sha256 prefix `cfeffdcaeee3bcd2`) |
| api_security_type_probe_matrix | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `9be83e57ed176a17`; BOND/BILL code 200 errors replay-promoted |
| api_tif_attribute_matrix_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `e6601dcc2abfd001` |
| api_algo_variants_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `1855e2554d7de3ae` |
| api_pairs_trading_aapl_msft | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `0dc806f7bb0868e8` |
| api_dollar_cost_averaging_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `296bdf662eb84e30` |
| api_stop_loss_management_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `a563cafd26e366be` |
| api_bracket_trailing_stop_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `2c0453360020a3ad`; live code 328 rejected TRAIL child under market bracket parent |
| api_future_campaign_mes | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `77d0b1b6a8c2d760` |
| api_option_campaign_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `5731e087403fe0f3`; OPRA/option path limited by real entitlement response |
| api_combo_option_vertical_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `495479ef4c345d96`; combo path blocked by option entitlement |
| api_market_data_completeness_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `f692fc168a53da9d`; bare SetType cycle, real-time-bars errors, and tick-by-tick code 10089 variants replay-promoted |
| api_historical_matrix_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `366075c3b171c44d` |
| api_news_article_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `3c6ef62da8d60e95`; fetched article from historical-news ID |
| api_fundamental_reports_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `02649216ff69f306`; mixed XML success and real 430 errors, with the `ReportRatios`/`ReportsFinStatements` errors replay-promoted |
| api_wsh_variants_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `65aeb0a3b716e4b6`; real 10276 entitlement errors |
| api_completed_orders_variants_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `6415ad97b4c9f33e`; exposed completed-order TRAIL LIMIT decode interruption |
| api_completed_orders_variants_aapl | 2026-04-15 | recorded, verified after fix; server_version=200, events sha256 prefix `6ac84daaf4084436`; apiOnly=false and apiOnly=true returned completed orders |
| api_transmit_false_then_transmit_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `003abb59dfced542` |
| api_duplicate_quote_subscriptions_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `84f1e78a18616e0f` |
| api_reconnect_active_order_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `9d72a4711c25c788` |
| api_client_id0_order_observation_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `5ff9cdc0f6f9b500` |
| api_cross_client_cancel_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `fcb7e811624e4aa9` |
| api_pairs_trading_aapl_msft | 2026-04-15 | aggressive 500-share AAPL/MSFT capture recorded after size increase; server_version=200, events sha256 prefix `86dc8f389a457efc`; order lifecycle/cleanup succeeded, execution-query layout bug fixed afterward |

## Transcript Promotions

| Transcript | Source Capture | Status |
|-----------|---------------|--------|
| api_ioc_fok_aapl.txt | 20260413T184916Z | promoted; covers IOC cancel plus FOK inactive/reject paths |
| api_tif_attribute_matrix_aapl.txt | 20260415T150535Z | promoted; covers GTC rest/cancel and trailing-percent fill replay |
| api_stop_loss_management_aapl.txt | 20260415T153735Z | promoted; covers market entry, protective stop modify/cancel, and flatten replay |
| api_transmit_false_then_transmit_aapl.txt | 20260415T162717Z | promoted; covers staged Transmit=false then transmit/cancel replay |
| api_reconnect_active_order_aapl.txt | 20260415T162822Z | promoted; covers GTC active order visible after reconnect |
| api_order_handle_reconnect_cancel_aapl.txt | 20260415T162822Z | promoted; covers original OrderHandle Gap/Resumed lifecycle and cancel after reconnect |
| api_client_id0_order_observation_aapl.txt | 20260415T162840Z | promoted; covers client ID 0 observing/cancelling another client's GTC order |
| api_cross_client_cancel_aapl.txt | 20260415T162857Z | promoted; covers client ID 2 observing/cancelling client ID 1 order |
| api_completed_orders_variants_aapl.txt | 20260415T170243Z | promoted; covers completed-orders apiOnly=false and apiOnly=true after live completed-order decode fix |
| api_future_campaign_mes.txt | 20260415T162047Z | promoted; covers MES futures buy/flatten round trip with executions and commissions |
| api_pairs_trading_aapl_msft.txt | 20260415T161858Z | promoted; covers paired AAPL long/MSFT short entries and per-symbol flatten replay; source execution-query tail timed out |
| api_dollar_cost_averaging_aapl.txt | 20260415T161924Z | promoted; covers three staged AAPL entries and aggregate flatten replay; source execution-query tail timed out |
| api_bracket_trailing_stop_aapl.txt | 20260415T161944Z | promoted; covers live code 328 rejection for TRAIL child attached under a market bracket parent; source tail entered execution-query timeout and cleanup |
| api_stress_rapid_fire_aapl.txt | 20260414T171824Z | promoted; covers ten far AAPL LMT orders, distinct order IDs, and global-cancel terminal replay |
| api_forex_lifecycle_eurusd.txt | 20260414T182627Z | promoted; covers EUR.USD far LMT OpenOrder, Inactive status, and real code 201 leverage rejection replay |
| api_bracket_trigger_aapl.txt | 20260413T174517Z | promoted; covers bracket parent fill, child OCA group echo, and real price-band cancel/reject on forced take-profit modify |
| api_oca_trigger_aapl.txt | 20260413T174546Z | promoted; covers OCA group echo on both AAPL peers and real PendingCancel/Cancelled price-band rejection for the aggressive peer |
| api_whatif_margin_aapl.txt | 20260414T164207Z | pending; no checked-in transcript because the live captures did not produce a usable WhatIf preview callback; `20260414T182608Z` returned live code 320 parser error after the WhatIf place request |
| api_scale_in_campaign_aapl.txt | 20260414T172617Z | promoted; covers two AAPL market fills and protective STP PreSubmitted trigger replay; source tail timed out before flatten/executions/cleanup callbacks |
| api_duplicate_quote_subscriptions_aapl.txt | 20260415T162742Z | promoted; covers SetType(Delayed) followed by two independent same-contract AAPL quote subscriptions receiving delayed market-data type plus bid/ask ticks |
| api_fundamental_report_errors_aapl.txt | 20260415T162248Z | promoted; covers live code 430 `ReportRatios` and `ReportsFinStatements` errors without checking in large XML report payloads |
| api_security_type_probe_errors.txt | 20260415T150322Z | promoted; covers live code 200 BOND and BILL contract-detail errors without checking in large successful contract-detail payloads |
| api_market_data_type_cycle.txt | 20260415T162200Z | promoted; covers bare SetType(1/2/3/4) requests accepted silently before later market-data entitlement probes |
| api_realtime_bars_request_errors_aapl.txt | 20260415T162200Z | promoted; covers TRADES code 420, BID_ASK code 321, and MIDPOINT code 10089 request-scoped errors |
| api_tick_by_tick_entitlement_errors_aapl.txt | 20260415T162200Z | promoted; covers Last, AllLast, AllLast ignore-size, BidAsk, and MidPoint live code 10089 subscription errors |
| api_wsh_variants_aapl.txt | 20260415T162255Z | promoted; covers WSH metadata plus conid, portfolio, watchlist, and competitor event-data variants returning real code 10276 entitlement errors |
| contract_details_aapl_opt.txt | 20260405T214941Z | promoted; covers OPT chain subset (strike ladder, call/put legs) with v200 option identifiers; source capture truncated before contractDataEnd, end frame taken verbatim from same-run sibling captures |
| contract_details_eurusd_cash.txt | 20260405T215014Z | promoted; covers single-match EUR.USD CASH details on IDEALPRO |
| contract_details_es_fut.txt | 20260405T215018Z | promoted; covers 21-expiry ES futures ladder including a full-session lastTradeDate timestamp (`20261218 08:30:00 US/Central`) |
| contract_details_not_found.txt | 20260405T215022Z | promoted; covers real code 200 not-found error surfaced as `APIError` with `OpContractDetails` |
| qualify_contract_ambiguous.txt | 20260407T190656Z | promoted; covers 26-row ambiguous MSFT qualify resolving to `ErrAmbiguousContract` |
| api_conditions_matrix_aapl.txt | 20260610T200935Z | promoted; covers all six condition families accepted to PreSubmitted after the field-order fix, off-hours code-399 handle closure, and 5-field cancel acknowledgements; Gateway condition echoes decode fully since the None-sentinel fix (frozen by capture-decode tests) |
| api_order_rest_cancel_161_aapl.txt | 20260610T195745Z | promoted; covers LMT rest/cancel plus the code-161 safety re-cancel closing the handle |
| api_order_stop_cancel_aapl.txt | 20260610T195758Z | promoted; covers STP and STP LMT rest/cancel with Gateway-computed echo limits and why_held=trigger |
| api_order_trailing_cancel_aapl.txt | 20260610T195819Z | promoted; covers TRAIL off-hours partial-then-full fill with cancel-after-fill 10148, TRAIL LIMIT rest/cancel, and the UTC-dash execution times the parser now accepts (both fills and commissions reach the handle in replay) |
| api_order_relative_cancel_aapl.txt | 20260610T195833Z | promoted; covers REL with Gateway-assigned 0.01 offset echo and full cancel lifecycle |
| api_order_rejects_aapl.txt | 20260610T195923Z | promoted; covers 321 invalid order type, price-band 202 cancel text, 10147 unknown-order cancel, 10148 re-cancel, and 161 safety re-cancel |
| current_time_live.txt | 20260611T074046Z | promoted; covers explicit reqCurrentTime with the live epoch reply and the Session snapshot |
| req_ids_read_only.txt | 20260611T074047Z | promoted; freezes the read-only Gateway's code-321 rejection of explicit reqIds as an unsolicited push (no public API sends reqIds; the client frame is omitted and disclosed in the header) and the drop-without-perturbation session surface |
| matching_symbols_partial.txt | 20260611T074053Z | promoted; covers the 97-row partial-pattern symbolSamples reply including BOND issuer ids, round-tripped field-for-field against the capture |
| set_type_switch_while_streaming.txt | 20260611T074112Z | promoted; covers the stream-tied marketDataType(3) push, delayed ticks, mid-stream SetType(Live) acceptance, and 10167 as a session event; the four bare set_type captures add nothing beyond api_market_data_type_cycle.txt and set_type_invalid is inconclusive (driver disconnected before any reply; public API validates 1..4 client-side) |

## Coverage Gaps: What We Need To Hit

### Security Types Not Yet Live-Tested

| SecType | Blocker | Path Forward |
|---------|---------|-------------|
| OPT | 2026-04-15 probe timed out while streaming chain details | Rerun narrower qualified option probe or subscribe to OPRA data |
| FOP | 2026-04-15 probe timed out | Rerun with a concrete future option contract after qualifying future expiry |
| BAG (combo) | Depends on OPT qualification | Same |
| BOND | 2026-04-15 placeholder probe returned real code 200; error replay is promoted | Replace placeholder with real live-derived bond identifier |
| CFD | ContractDetails succeeded in 2026-04-15 probe | Add order/market-data entitlement probe |
| WAR | 2026-04-15 probe timed out | Rerun with concrete warrant from exchange search |
| CRYPTO | ContractDetails succeeded for BTC/PAXOS in 2026-04-15 probe | Add trading-permission order probe |
| FUND | ContractDetails succeeded for VTSAX/FUNDSERV in 2026-04-15 probe | Add mutual-fund-specific order probe |

### Order Types Not Yet Live-Tested

| Order Type | Blocker | Priority |
|-----------|---------|----------|
| MOO | Context expired in batch; needs solo run | high |
| LOO | Same | high |
| SSHORT | Needs short-sale permission | medium |

### TIF Values Not Yet Live-Tested

| TIF | Blocker | Priority |
|-----|---------|----------|
| GTC | Live capture + replay promoted | covered by `api_tif_attribute_matrix_aapl.txt`; persistence across reconnect still target |
| GTD | Live capture exists | promote GTD-specific replay and expiry behavior |
| OPG | Only works at open | medium — run during pre-market |
| DTC | Niche TIF | low |

### Order Attributes Not Yet Live-Tested

| Attribute | Current Coverage | Missing |
|-----------|-----------------|---------|
| GoodAfterTime | live probe in `api_tif_attribute_matrix_aapl` | promote focused replay |
| GoodTillDate | live probe in `api_tif_attribute_matrix_aapl` | promote focused replay |
| AllOrNone | live probe in `api_tif_attribute_matrix_aapl` | promote focused replay |
| MinQty | live probe attempted in `api_tif_attribute_matrix_aapl` | inspect driver events and promote real accept/reject |
| PercentOffset | live probe in `api_tif_attribute_matrix_aapl` | promote focused replay |
| TrailingPercent | live capture + replay promoted | covered by `api_tif_attribute_matrix_aapl.txt` |
| HedgeType | none | Delta/beta/FX/pair hedge |
| ScaleInitLevelSize | live probe attempted in `api_tif_attribute_matrix_aapl` | inspect driver events and promote real accept/reject |
| DontUseAutoPriceForHedge | none | Hedge pricing override |
| UsePriceMgmtAlgo | live probe in `api_tif_attribute_matrix_aapl` | promote focused replay |
| CashQty | none | Forex cash quantity |
| ManualOrderTime | live probe in `api_tif_attribute_matrix_aapl` | promote focused replay |

### Algo Strategies Not Yet Live-Tested

| Strategy | Status |
|----------|--------|
| Adaptive (Normal) | tested |
| Adaptive (Urgent) | captured: accepted, cancelled |
| Adaptive (Patient) | captured: accepted, cancelled |
| TWAP | captured: real Gateway rejection (`Unknown algo attribute:strategyType`) |
| VWAP | captured: accepted, cancelled |
| ArrivalPx | captured: accepted, cancelled |
| DarkIce | captured: no status before cleanup; inspect raw events before promotion |
| AccumDist | captured: accepted, cancelled |
| Inline | captured: real Gateway rejection; inspect raw events before promotion |

### Condition Types Not Yet Live-Tested

| Condition | Status |
|-----------|--------|
| Price condition (type 1) | tested (conditionPrice test) |
| Time condition (type 3) | not tested |
| Margin condition (type 4) | not tested |
| Execution condition (type 5) | not tested |
| Volume condition (type 6) | not tested |
| Percent-change condition (type 7) | not tested |

### Multi-Leg / Complex Strategies Not Yet Live-Tested

| Strategy | Status |
|----------|--------|
| Vertical spread (call) | blocked (option permissions) |
| Vertical spread (put) | blocked |
| Iron condor | not written |
| Calendar spread | not written |
| Butterfly | not written |
| Straddle/strangle | not written |
| Ratio spread | not written |
| STK pairs (buy A + sell B) | not written |

### Campaign / Workflow Scenarios Not Yet Live-Tested

| Scenario | Status |
|----------|--------|
| Pairs trading (buy AAPL + sell MSFT) | not written |
| Options wheel (sell put -> assigned -> sell call) | not written (and blocked by OPT permissions) |
| Multi-timeframe (real-time bars + order placement) | partially covered (OrdersWithSubscriptions) |
| Reconnect with active orders | live-captured and replay-promoted via `api_reconnect_active_order_aapl.txt` |
| Multi-client same account | live-captured and replay-promoted via `api_cross_client_cancel_aapl.txt` |
| Client ID 0 order observation | live-captured and replay-promoted via `api_client_id0_order_observation_aapl.txt` |
| Order handle across reconnect | active order visibility/cancel after reconnect promoted; original in-memory handle Gap/Resumed and post-reconnect cancel replay-promoted |

### Capture Scenarios Not Yet Recorded

| Scenario | Priority | Notes |
|----------|----------|-------|
| api_gtic_gtd_aapl | high | GTC and GTD TIF with specific dates |
| api_trailing_percent_aapl | high | TRAIL with TrailingPercent instead of dollar |
| api_all_or_none_aapl | medium | AON order semantics |
| api_moo_loo_aapl | medium | Pre-market open orders |
| api_time_condition_aapl | medium | Time condition that fires at specific time |
| api_margin_condition_aapl | medium | Margin cushion condition |
| api_volume_condition_aapl | medium | Volume-triggered order |
| api_percent_condition_aapl | medium | Percent-change triggered |
| api_hedge_delta_aapl | low | Delta hedge order |
| api_scale_order_aapl | low | Scale order with levels |
| api_twap_aapl | medium | TWAP algo strategy |
| api_vwap_aapl | medium | VWAP algo strategy |
| api_crypto_btc | low | Crypto order if permissions available |
| api_bond_lookup | low | Bond contract details + order |
| api_reconnect_with_orders | high | Reconnect while orders are active |

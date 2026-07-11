# Live Test Execution Tracker

Companion to [`live-coverage-matrix.md`](live-coverage-matrix.md). Tracks every
live test run against IB Gateway paper account, what passed, what failed, what
was fixed, and what remains untested.

Last updated: 2026-07-11. Both local Gateway roles are available. The current
production ceiling and latest read-only account campaign were live-checked at
exact `server_version 207`; earlier campaign rows retain their negotiated
version as recorded.

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

### 2026-07-11

The complete accounts/positions protobuf boundary was implemented and checked
at exact `server_version=207`. Official Python SDK 10.48.01 capture
`20260711T011842Z-sdk_sv207_accounts_positions_boundary` covers all ten
request types and thirteen callback types (events SHA-256
`936f9f4ea1633770071d9bd07a5ec721b7ddd481fcae6ad1aac95a9c1287a153`).
The native client then completed managed-account refresh, account summary,
positions, account updates, account updates multi, and positions multi. The
five-leg native capture is
`20260711T012845Z-public_sv207_accounts_positions_boundary` (events SHA-256
`951ea55770f30d410cf971a5bf7bbf94da0183017cac0984b6b5d0370ab7ad86`).
`managed_accounts_sv207_live.txt` promotes the load-bearing protobuf bootstrap
and explicit refresh exchange.

Managed-account refresh was added as `Client.ManagedAccounts` and verified
against `readonly-live` at exact `server_version=206`. Official API 10.48.01
first established the `17,1` request and second managed-accounts callback; the
public scenario then reproduced the exchange and updated the session snapshot.
The public capture is `20260711T011427Z-managed_accounts_refresh` (events
SHA-256 `dc7a53d00e621d95e1f55ce518f225e0c1fa26e8806d5b7acf9bcbbaa252a095`),
and `managed_accounts_refresh.txt` retains the length-preserving sanitized
frames.

### 2026-07-10

Order preview ownership was revalidated after removing caller-settable
`Order.OrderID` and `Order.WhatIf`. The role-aware `paper-dev` doctor reached
`server_version=206`, and `api_whatif_margin_aapl` then called
`Orders().Preview` directly. Capture
`captures/20260710T210047Z-api_whatif_margin_aapl` (events SHA-256
`6f61cc2b44b11d339e5d5cda30a53fb0f1fdd6bd78f2e83aada4b996cd97afda`)
returned the complete margin and commission state with no order-status
lifecycle or resting order. The capture verifier passed.

Exact-206 market-data boundary verification. The official API 10.48.01 SDK
oracle produced exact quote/depth request and callback captures plus classic
CFD reroute callbacks 91/92. Both role-aware doctor runs then negotiated
`server_version=206` and completed read-only probes. Readonly-live returned the
expected code 10168 entitlement warning; paper-dev returned a delayed AAPL
quote with every normalized field available.

`TestLiveServer206MarketDataBoundary` passed independently against
127.0.0.1:4001 and 127.0.0.1:4002. Through the public API it selected delayed
data, opened an exact-206 protobuf quote subscription, and preserved request-
parameter permission presence plus both precision values. No order was sent.
The promoted `market_data_sv206_live.txt` transcript hashes to
`495b20087d9c548a814d4bedadf52dcfd80b489b4723197f818558dd366f1075`.
Positive raw-213 L2 remains blocked by account entitlement and is not
fabricated.

Exact-205 contract-data boundary verification. Both role-aware doctor runs
connected at `server_version=205` and completed the read-only time probe. The
readonly role returned its expected code 10089 AAPL market-data entitlement
warning; paper-dev returned a delayed AAPL quote.

`TestLiveServer205ContractDataBoundary` then passed independently against
127.0.0.1:4001 and 127.0.0.1:4002. Through the public API it qualified exact
stock, bond, fund, option, and ineligible-bond conIDs, preserving explicit zero
algorithmic minimum, stock price/size precision, fund metadata, both option
dates, and all three real ineligibility reasons. The test is read-only.

### 2026-07-09

Official SDK/source drift audit and live fundamental-data retirement check.
Both Gateway roles connected at `server_version 200`. The public
[IBKR API Software](https://interactivebrokers.github.io/) page now lists
Latest API 10.48 (released 2026-07-07), Stable API 10.45 (released
2026-03-30), and recommends TWS or IB Gateway 1045+.

- API 10.47 removed `reqFundamentalData`, its cancellation and callback, and
  fundamental-ratios tick type 47. A final live probe sent all seven legacy
  report requests through both roles; every request returned code 10358. The
  readonly-live capture is
  `captures/20260709T214833Z-api_fundamental_reports_aapl` (events sha256
  prefix `89db59e9e5abf7b7`); paper-dev is
  `captures/20260709T214857Z-api_fundamental_reports_aapl` (prefix
  `c326f314cbc4f1de`). The ibkr-go surface, active capture scenarios, replay
  transcripts, and classic message IDs are retired. These ignored capture
  directories and earlier sv200 records remain historical evidence only; WSH
  is a separate API, not a replacement.
- API 10.48 changes `reqOpenOrders` results to include de-activated orders.
  No wire-shape change is announced. Re-run open-order snapshot and
  subscription captures when a local Gateway is on the updated release before
  changing result-set expectations.
- The new public `api_generic_tick_matrix_aapl` scenario ran safely through
  `readonly-live` for 15 seconds and cancelled cleanly. The successful capture
  is `captures/20260709T223341Z-api_generic_tick_matrix_aapl` (raw sha256
  `5c40260d783971d22e6de209c90a61fd489479e0e7fc2ebf20be4e76d677a45e`,
  events sha256
  `d04dd1439dfec841dafac9be56e199f453693d063c2dfd9f5a912389fbba7676`,
  normalized frames sha256
  `97b3ac64149500f42e533eafa4ba890bd7f88698be11463643eeb2ea24dea9f0`).
  It observed delayed normalized prices/sizes plus unmapped mark-price tick 37,
  shortable ticks 46/89, volume-rate tick 56, and delayed-timestamp string tick
  88. Real-time volume, trade-count, and trade-rate values were requested but
  did not arrive in this window, so they remain open. The preceding capture
  `20260709T223247Z-api_generic_tick_matrix_aapl` exposed a real omitted
  TickReqParams minimum tick; its raw hash
  `bd284e22771394b3baf7b827d62ed22d45f15e401dd478f430e18f4e715b0377`
  now grounds the nil-minimum regression test.
- A delayed AAPL quote request for generic ticks `mdoff,292:BRFG` produced five
  exact contract-specific TickNews frames at server version 201 in
  `captures/20260709T230825Z-api_tick_news_aapl_probe` (events sha256
  `a0784d2eddda74681cc301befb98440a96bb76242efd43aec88a9f177a5411df`,
  normalized frames sha256
  `e3e1901503f7d1dc52489bccb2bce64467e35bad48bc025e438b916a5c639e60`).
  The first frame is frozen byte-for-byte in the codec and public transcript;
  its timestamp, provider code, article ID, headline, and extra-data string are
  projected without mutating the accumulated quote. Repeat probes may return
  no headlines when the Gateway has no new provider item for that session.

### 2026-07-04

Issue #20 fix session (US market holiday, orders rest PreSubmitted).
`paper-dev` on 4002 healthy. Live probes with scratch drivers plus a fresh
`api_reconnect_active_order_aapl` capture (20260704T174748Z) established:

- Same-clientID recover-and-cancel works end to end: after reconnect,
  `SubscribeOpen` recovers the resting order and `Orders().Cancel(orderID)`
  yields `order_status Cancelled` on the wire; the engine now routes it to
  the open-orders subscription (the issue #20 fix).
- Cross-client cancel by order ID was rejected with real code 10147
  ("OrderId N that needs to be cancelled is not found") for both client 0
  and a plain second client, with a single unambiguous resting order. This
  conflicts with the 2026-04-15 captures behind
  `api_client_id0_order_observation_aapl.txt` and
  `api_cross_client_cancel_aapl.txt`, which recorded the cancel succeeding
  during regular trading hours. Whether the delta is a Gateway build change
  or an outside-RTH/PreSubmitted condition needs a market-hours re-probe
  before those fixtures are touched.
- New Gateway reconnect behavior versus April: reconnecting with the same
  clientID now receives an unsolicited open-orders snapshot (open_order +
  order_status + open_order_end) immediately after START_API, before
  managed_accounts/next_valid_id, and the req_open_orders recovery snapshot
  pairs each open_order with an order_status. Frozen in
  `api_reconnect_recovered_cancel_status_aapl.txt`.
- Paper residue: none; all probe orders cancelled and the final snapshot
  reported zero open orders.

Issue #21 follow-up in the same session, capture 20260704T181808Z
(`api_open_orders_refresh_aapl`): re-sending `req_open_orders` on an active
open-orders subscription yields a fresh snapshot burst terminated by another
`open_order_end`, with unsolicited open_order/order_status echo pairs arriving
between the snapshots — this grounds `Orders().RefreshOpen`. A direct probe of
`req_auto_open_orders` showed the Gateway sends no `open_order_end` for the
auto bind (no snapshot boundary), so auto-scope refresh is rejected
client-side with `ErrNoSnapshot`. Probe orders cancelled; zero residue.

### 2026-06-11

Campaign day two. Market-window paper captures landed regular-session
fills (MKT buy/sell, the fill campaign, delayed modify, the order-type
matrix), the option exercise evidence pair (barely-ITM refusal 322;
deep-ITM accepted with the 10349 TIF preset notice then rejected by the
paper clearing server with 322 and cancelled 202), the hedge rule map
(beta and pair hedges accepted at zero size; delta requires an option
parent and a valid ratio; FX hedging requires a matching currency-pair
child; sizing draws 10032), and the pairs/DCA/algorithmic reconciliation
tails that timed out in April. Exercise-scenario residue: two long AAPL
Jun-12 calls (282.5, 292.5) remain in the paper account and expire the
next session. `paper-dev` healthy throughout. Operational incident on
`readonly-live`: two historical raw `ibkr-probe` attempts at the
`reqCurrentTimeInMillis` drift check (one pre-handshake payload that the
Gateway answered as a legacy version greeting, one framed message injected
before `startApi`) left the 4001 API listener refusing handshakes with
i/o timeouts; `ibkr-doctor` confirmed the wedge and recovery is pending a
listener drain or Gateway restart. Lesson recorded: protocol probes go
through the capture tooling and the library, never hand-rolled frames —
the `current_time_millis` grounding capture was rerouted through the
paper role (`IBKR_CAPTURE_ROLE=paper-dev`) and succeeded.

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
  with 10.48 recorded as a drift watch item. Superseded on 2026-07-09 when
  API 10.48 became the published Latest release; the live observations in
  this dated entry remain valid for that campaign.

### 2026-05-25

Both local Gateway roles connected with `server_version=200`.

- `readonly-live` diagnostic connected and `CurrentTime` passed. AAPL quote
  returned real IBKR code 10089: live API market-data subscription unavailable;
  delayed data was offered by Gateway.
- `readonly-live` broad `IBKR_LIVE=1 ... -run '^TestLive'` passed. The run
  logged current account/session blockers instead of treating them as library
  regressions: completed-orders silence until context deadline, historical
  ticks code 10187 for missing ISLAND market-data permission, fundamentals
  code 10358 (historical; the API was removed in official API 10.47), WSH code
  10276, and market-depth timeout with no rows.
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
| TestLiveOrderWhatIf | `Orders().Preview` | pass | margin state plus 1.0003 USD commission revalidated at server version 206 |
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
| Cancellation notice closed handles as errors | API codes 161 and 202 are cancel replies, not placement failures; 161 can precede the eventual Cancelled status | Route both as session notices and keep terminal status authoritative |
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
| api_scanner_subscription | 2026-07-10 | recorded and verified through the strict public runner; server_version=206 returned ten ranked HOT_BY_VOLUME rows followed by public cancel and a current-time fence, events sha256 `4d32e5b9b88ae43887d7391c50b3cbea2978fd85b72933363b62cd80d26700dd`; server_version=200 success `c84c81b3ee772bcc` remains replay-promoted in `scanner_subscription_live.txt` |
| api_fundamental_reports_aapl | 2026-07-09 | final retirement evidence; all seven legacy requests returned code 10358 on both server_version=200 roles; readonly-live events sha256 prefix `89db59e9e5abf7b7`, paper-dev prefix `c326f314cbc4f1de`; ignored capture directories retained as historical evidence only |
| scanner_subscription | 2026-04-07 | historical raw evidence, catalog duplicate retired 2026-07-10; server_version=200 events sha256 prefix `740b5dfb138df2a4`; the Gateway returned exact code 490 for missing scanner permissions and code 365 from the now-removed redundant cancel |
| scanner_subscription_no_items | 2026-07-10 | private recorder evidence from paper server_version=200; exact request-scoped code 165 `Historical Market Data Service query message:no items retrieved` is followed by a valid zero-row ScannerData response and grounds the engine's nonterminal scanner route; events sha256 `14cdc2913735bb3c3839beff106ab0524f5b8d509340a49d8d07470e39728e7e` |
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
| api_fundamental_reports_aapl | 2026-04-15 | historical, retired 2026-07-09 after official API 10.47 removed the feature; the original server_version=200 capture remains traceable by events sha256 prefix `02649216ff69f306` and contained mixed XML success and real 430 errors |
| api_wsh_variants_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `65aeb0a3b716e4b6`; real 10276 entitlement errors |
| api_completed_orders_variants_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `6415ad97b4c9f33e`; exposed completed-order TRAIL LIMIT decode interruption |
| api_completed_orders_variants_aapl | 2026-04-15 | recorded, verified after fix; server_version=200, events sha256 prefix `6ac84daaf4084436`; apiOnly=false and apiOnly=true returned completed orders |
| sdk_sv204_completed_order_boundary | 2026-07-09 | official SDK oracle recorded and replay-promoted; exact server_version=204, events sha256 `9be0a4fd1b27056803baf4dbfb16a0ec2daa02d7f43bdec06b5ceee4b6f817a5`; client/all/auto open-order queries, completed apiOnly=false, cancelled/filled results, and clean account state |
| sdk_sv204_completed_orders_api_only | 2026-07-09 | official SDK oracle recorded; exact server_version=204, events sha256 `36acaedf6b155ce6f1f0d2433d99864e1248cd1b16bc56701f4300c46def8a90`; completed apiOnly=true presence vector |
| sdk_sv205_contract_data_boundary | 2026-07-10 | official SDK oracle recorded; exact server_version=205, normalized frames sha256 `f9dc02ad6fd6b480c26be3e4628efbfa26821602f16e97f974a8d9ce61cd6d8c`; 58-row Apple issuer lookup plus a real three-reason ineligibility response |
| sdk_sv205_contract_data_compact | 2026-07-10 | official SDK oracle recorded and replay-promoted; exact server_version=205, events sha256 `3d1e3c303295a88e1923fa8127a991d83ce9dc86e742b8423eff047ce8ce6a49`; exact AAPL stock and single Apple bond round trips |
| sdk_sv205_contract_data_type_matrix | 2026-07-10 | official SDK oracle recorded; exact server_version=205, normalized frames sha256 `a55614fb9b32d35b8d0e3ba6479d5a111fe7db8bed83429527a20217a44c8ef5`; exact stock, bond, fund, and option responses |
| sdk_sv206_market_data_boundary | 2026-07-10 | official SDK oracle recorded; exact server_version=206, events sha256 `eea31798e7e59830f5cda9daadcd94223045c8e9ee0e5aa10f48428447505822`; quote snapshots, option computation, request parameters, request/cancel boundary |
| sdk_sv206_market_data_readonly_retry | 2026-07-10 | official SDK oracle recorded and replay-promoted; exact server_version=206, events sha256 `989563f9c4cad108e34058beac205c576a9ebdc0fffe03e421e829bca851e7de`; dense AAPL quote and raw-212 depth rows |
| sdk_sv206_cfd_reroute_readonly | 2026-07-10 | official SDK oracle recorded; exact server_version=206, events sha256 `7475841869bc53ceaf779b2672bb1606453e84b1dc0ff66a361510f45470d279`; IBM/BMW/EURUSD classic callbacks 91/92 |
| open_orders_empty | 2026-07-10 | public API recorded on read-only Gateway; server_version=206, events sha256 `0e838de9d463070ac711be4950948c682c01e8ad02546d8be32f47f35ce68d25`; req_id=-1/code 321 now terminates `Orders().Open` as a typed error |
| historical_news_aapl_timezone_window | 2026-07-10 | public API recorded on read-only Gateway; server_version=206, events sha256 `72f0bc03fb2e796685be47580d79b5e9363a9de9590702ee78da1e15c95f0629`; `.0 UTC` lower end bound returned 17 in-bound items and a clean end marker |
| sdk_exact206_historical_news_end_bound | 2026-07-10 | official SDK 10.48.01 oracle; exact server_version=206, events sha256 `77f0031eef632b2779333eefc051f6731bb0d77daa8b35d3585771e7eccb854c`; `.0 UTC` lower end bound returned 17 items |
| completed_orders | 2026-07-11 | public API recorded on read-only Gateway; server_version=206, events sha256 `ff9808ba8f3f33f2eb14e6da059e1e4437ab7c19553f4ebb4d0b9a7bfc97c289`; exact unkeyed `-'S'` code-321 refusal now terminates `Orders().Completed` as a typed error and the scenario completes with a current-time fence |
| completed_orders | 2026-07-11 | public API recorded on paper Gateway; server_version=206, events sha256 `9f7bdc11f8012a5ebf1c33f5ed979bfc0ac192476a8b5f9033a2ab9459ff352f`; empty apiOnly snapshot ended with `completedOrdersEnd` and a current-time fence |
| quote_stream_multi_asset | 2026-07-11 | public API recorded on read-only Gateway; server_version=206, events sha256 `aeb6123029b488bc34be31cde499af3665a4cc804efc9eecc5fe0f1d32288f2b`; concurrent AAPL and EUR.USD subscriptions each produced typed price/size evidence before both cancels and a current-time fence. A preceding raw sv200 diagnostic (`0ebd2c27df484799`) proved the old scenario falsely passed after tick 258's code-10358 refusal terminated only its AAPL leg; tick 258 remains covered by the dedicated generic-tick matrix |
| set_type_switch_while_streaming | 2026-07-11 | public API recorded on read-only Gateway; server_version=206, events sha256 `6040ff478db4d277a7a4c68572f4c269c47d0c5cc3883164e886ca29a98e5ba2`; typed delayed market-data type and price/size evidence preceded the silent live switch, cancel, and current-time fence |
| news_bulletins | 2026-07-11 | public API recorded on read-only Gateway; server_version=206, events sha256 `bcff4c85f318eb48adaa2e03aae3a970f015d3257204ca66da278c905799c2b5`; a valid quiet five-second observation window ended with typed subscription cancel/wait and a current-time fence; the existing live replay retains the two known bulletin payloads |
| pnl | 2026-07-11 | public API recorded on read-only Gateway; server_version=206, events sha256 `eeba9a0ab7c6e1e210a5dc83aad7a2af3521795d2fba9f6571be996bc2b5c189`; one typed nonzero account PnL update was followed by cancel, wait, and a current-time fence |
| account_updates | 2026-07-11 | public API recorded on read-only Gateway; server_version=206, events sha256 `67610b05ba57b09557059632f2039d47af5e9b3a93b0f09b1e3816a33b8f779f`; finite snapshot returned 194 typed account values and 18 typed portfolio rows before the end marker, unsubscribe, and current-time fence |
| historical_bars_keepup | 2026-07-11 | public API recorded on read-only Gateway; server_version=206, events sha256 `00690a72a718c0d50cb3d823bc0d7b2cc44e0610637ba938fcb27e0f14218379`; 1,950 typed initial one-minute bars ended cleanly before cancel, wait, and current-time fence. No `historicalDataUpdate` arrived, so live streaming-update coverage remains candidate rather than promoted |
| pnl_single | 2026-07-11 | public API recorded on read-only Gateway; server_version=206, events sha256 `46bd87b72db2c79fd603d5f56fda2ac94b11f25cc74896086d6fe7b3ee921c2f`; the scenario derived a real held stock contract from the typed account snapshot, received its typed position/PnL/value update, then cancelled, waited, and fenced. This replaces the ungrounded hardcoded-AAPL raw probe |
| request_fa | 2026-07-11 | public API recorded on read-only Gateway; server_version=206. The first capture (`137e972fe8fa61a3`) exposed a hang after the exact req_id=0/code-321 `-'b4'` non-FA refusal; after operation-specific singleton routing, capture `5ea5af05534a3e3bfaac6acdaa6a8d8843674f58068caa3213747718bf79b99d` returned a typed `OpFAConfig` error and completed with a current-time fence |
| display_group_subscribe | 2026-07-11 | public API recorded on read-only Gateway; server_version=206, events sha256 `bb3a476faf05a1d39ab39d318c472c759df22b66eda08c77c17b8ec8d58680a4`; queried real IDs, subscribed to group 1, received the typed initial `none` value, unsubscribed, waited, and fenced. No update was sent because re-selecting `none` is not a valid state-preserving update; nonempty-group update evidence remains candidate |
| api_order_direct_cancel_aapl | 2026-07-11 | public API recorded on paper Gateway; server_version=206, events sha256 `defd09098222e9f5936efb85994f63ef63bf31361b396ffd94b5ba2499c1df14`; placed allocated order 450 through `Orders().Place`, required the typed `PreSubmitted` open-order echo, called `Orders().Cancel(450)`, and observed `PendingCancel` then terminal `Cancelled` before fencing |
| api_bracket_place_aapl | 2026-07-11 | public API recorded on paper Gateway; server_version=206, events sha256 `137a29c661efb65b427297be6e146dc417280fde07a0dc51c09d7e8924b7837b`; direct `Orders().PlaceBracket` allocated 451/452/453, sent three place frames with controlled false/false/true transmit sequencing, received parent IDs 0/451/451 and typed `PreSubmitted` echoes, cancelled all three legs, verified none survived the public open-order snapshot, and fenced |
| raw order capture retirement | 2026-07-11 | removed thirteen duplicate hand-written wire scenarios and the capture tool's second place/cancel/global-cancel codec after public campaigns superseded every behavior. Five ungrounded symbolic order fixtures and tests were deleted; grounded live-derived focused replays remain |
| sdk_exact206_historical_news_end_bound_missing_fraction | 2026-07-10 | official SDK 10.48.01 negative oracle; exact server_version=206, events sha256 `63ca8e1ae5e99e8544491a051dd8f771d228b5d2b33b9f0baec8311b3b6e2b58`; identical bound without `.0` returned no items |
| sdk_exact206_historical_news_both_bounds | 2026-07-10 | official SDK 10.48.01 negative oracle; exact server_version=206, events sha256 `b099280f7ddbd26041358218f9b2c9fa3667e0e9be0af804180559b051465023`; Gateway ignored the lower `EndTime` and returned 127 rows through 2023, grounding the public both-bound rejection |
| quote_snapshot_aapl | 2026-07-10 | public API recorded and verified on readonly server_version=206; all twelve typed quote fields and the snapshot-end boundary were observed; events sha256 `a854ac41c4e0073f7578f2d2ac4a2c4b1f0954bd243c49167beda62f5f159e3e` |
| quote_stream_aapl | 2026-07-10 | public API recorded and verified on readonly server_version=206; typed delayed quote evidence followed by clean cancel, wait, and current-time fence; events sha256 `6c51009bdfe158b8ea07caf8b44896181b0e827b89e674186c1667d39791a491` |
| quote_stream_genericticks | 2026-07-10 | public API recorded and verified on readonly server_version=206; typed request parameters plus an exact 233/236 callback followed by clean cancel, wait, and current-time fence; events sha256 `327d7fbdeeb9979d0b2aa19585625dce19bf9ceb557116c0d0da4b0c89f5a34d` |
| realtime_bars_aapl | 2026-07-10 | public API recorded and verified on readonly server_version=206; exact typed `OpRealTimeBars` code 420 permission refusal followed by a current-time fence and no redundant cancel; events sha256 `ec740c40846dc4ad8183d694bd8882b0d7cd5185dc760e3e6cd779243cdf1fa0` |
| tick_by_tick_last | 2026-07-10 | public API recorded and verified on readonly server_version=206; exact typed `OpTickByTick` code 10089 entitlement refusal followed by a current-time fence and no redundant cancel; events sha256 `f9b2d1a88116f12b8b80285bce8176f55ff1b985cc78824f96aa6da4e6a279b8` |
| tick_by_tick_bidask | 2026-07-10 | public API recorded and verified on readonly server_version=206; exact typed `OpTickByTick` code 10089 entitlement refusal followed by a current-time fence and no redundant cancel; events sha256 `d055fa53d960c765dbfc2781823749b575302537fce607a1b27900e644115e1a` |
| tick_by_tick_midpoint | 2026-07-10 | public API recorded and verified on readonly server_version=206; exact typed `OpTickByTick` code 10089 entitlement refusal followed by a current-time fence and no redundant cancel; events sha256 `8cc2ca68e6eb2902765f0f6f0b11e780919ee543a06ed53067d0e4a58f50e6dd` |
| tick_by_tick_last_permission_refusal | 2026-07-10 | private recorder evidence from paper server_version=200; exact request-scoped code 10189 `Failed to request tick-by-tick data.No market data permissions for ISLAND STK.` grounds `ErrCodeTickByTickDataNotAllowed`; events sha256 `d29fed1cb48c2be9b25511a64c4b276dfe56ccd9ca7653e7601bef40b3768ec9` |
| market_depth_aapl | 2026-07-11 | public API recorded and verified on readonly server_version=206; exact typed `OpMarketDepth` code 10092 venue-combination refusal followed by a current-time fence and no redundant cancel; events sha256 `c45b90e97854b1cba584fa5d00b5882f1e16600328efe3bc18732741e6c72e54` |
| market_depth_aapl_smart | 2026-07-11 | public API recorded and verified on readonly server_version=206; exact request-scoped code 2152 listed no available depth venues, remained a nonterminal session availability notice, then the public runner explicitly cancelled the still-live SMART subscription and fenced it with current time; events sha256 `ea56919e479838305c23843b5daae4bdea17ad8d786824e6c1c274ef97d7821a` |
| api_transmit_false_then_transmit_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `003abb59dfced542` |
| api_duplicate_quote_subscriptions_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `84f1e78a18616e0f` |
| api_reconnect_active_order_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `9d72a4711c25c788` |
| api_client_id0_order_observation_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `5ff9cdc0f6f9b500` |
| api_cross_client_cancel_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `fcb7e811624e4aa9` |
| api_pairs_trading_aapl_msft | 2026-04-15 | aggressive 500-share AAPL/MSFT capture recorded after size increase; server_version=200, events sha256 prefix `86dc8f389a457efc`; order lifecycle/cleanup succeeded, execution-query layout bug fixed afterward |

## Transcript Promotions

| Transcript | Source Capture | Status |
|-----------|---------------|--------|
| market_data_sv206_live.txt | 20260710T133515Z + 20260710T134721Z | promoted; exact-sv206 raw protobuf quote frames, five request-family vectors in codec tests, parameter presence/precision, and clean cancellation |
| open_orders_readonly_refusal_sv206_live.txt | 20260710T225552Z | promoted; exact-sv206 client-scope open-orders request and req_id=-1/code 321 read-only refusal |
| historical_news_end_bound_sv206_live.txt | 20260710T231104Z | promoted; exact-sv206 `.0 UTC` lower end-bound request, two exact live rows, and end marker |
| api_ioc_fok_aapl.txt | 20260413T184916Z | promoted; covers IOC cancel plus FOK inactive/reject paths |
| api_tif_attribute_matrix_aapl.txt | 20260415T150535Z | promoted; covers GTC rest/cancel and trailing-percent fill replay |
| api_stop_loss_management_aapl.txt | 20260415T153735Z | promoted; covers market entry, protective stop modify/cancel, and flatten replay |
| api_transmit_false_then_transmit_aapl.txt | 20260415T162717Z | promoted; covers staged Transmit=false then transmit/cancel replay |
| api_reconnect_active_order_aapl.txt | 20260415T162822Z | promoted; covers GTC active order visible after reconnect |
| api_reconnect_recovered_cancel_status_aapl.txt | 20260704T174748Z | promoted; covers unsolicited reconnect snapshot push, paired order_status in the recovery snapshot, and cancel-status delivery to SubscribeOpen for a recovered order (issue #20) |
| api_open_orders_refresh_aapl.txt | 20260704T181808Z | promoted; covers RefreshOpen fresh snapshot burst on an active subscription with inter-snapshot echo pairs (issue #21) |
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
| api_whatif_margin_aapl.txt | 20260610T200009Z | promoted; covers the complete WhatIf margin preview (nine margin decimals plus commission 1.0003 USD) returned by the public one-shot with no OrderHandle or order lifecycle; the 2026-04-14 no-preview/code-320 attempts were the pre-v1.4.6 default-int placement bug; direct `Orders().Preview` was live-revalidated at server version 206 on 2026-07-10 (`6f61cc2b44b11d33`) |
| api_scale_in_campaign_aapl.txt | 20260414T172617Z | promoted; covers two AAPL market fills and protective STP PreSubmitted trigger replay; source tail timed out before flatten/executions/cleanup callbacks |
| api_duplicate_quote_subscriptions_aapl.txt | 20260415T162742Z | promoted; covers SetType(Delayed) followed by two independent same-contract AAPL quote subscriptions receiving delayed market-data type plus bid/ask ticks |
| api_fundamental_report_errors_aapl.txt | 20260415T162248Z | retired and removed from the active replay suite on 2026-07-09 after official API 10.47 removed the feature; formerly covered live code 430 `ReportRatios` and `ReportsFinStatements` errors without checking in large XML report payloads |
| api_security_type_probe_errors.txt | 20260415T150322Z | promoted; covers live code 200 BOND and BILL contract-detail errors without checking in large successful contract-detail payloads |
| api_market_data_type_cycle.txt | 20260415T162200Z | promoted; covers bare SetType(1/2/3/4) requests accepted silently before later market-data entitlement probes |
| api_realtime_bars_request_errors_aapl.txt | 20260415T162200Z | promoted; covers TRADES code 420, BID_ASK code 321, and MIDPOINT code 10089 request-scoped errors |
| api_tick_by_tick_entitlement_errors_aapl.txt | 20260415T162200Z | promoted; covers Last, AllLast, AllLast ignore-size, BidAsk, and MidPoint live code 10089 subscription errors |
| api_wsh_variants_aapl.txt | 20260415T162255Z | promoted; covers WSH metadata plus conid, portfolio, watchlist, and competitor event-data variants returning real code 10276 entitlement errors |
| contract_details_aapl_opt.txt | 20260710T221210Z | promoted; same-request sv206 replay retains three calls, one put, and ContractDetailsEnd from a complete 106-row nearest-expiry response selected through SecDefOptParams (`cff9f2c77e522104`) |
| smart_components.txt | 20260710T221437Z | promoted; same-session sv206 replay derives `9c0001` from AAPL TickReqParams while the quote is active, then returns the complete current 20-exchange smart-component mapping (`c32ec635ec50ff69`) |
| contract_details_apple_bonds.txt | 20260710T220545Z | promoted; exact sv206 request, all 58 message-18 Apple bond results by issuer ID, and end (`a433e9b0d181201c`) |
| contract_details_eurusd_cash.txt | 20260405T215014Z | promoted; covers single-match EUR.USD CASH details on IDEALPRO |
| contract_details_es_fut.txt | 20260710T220551Z | promoted; exact sv206 request, complete 21-expiry ES futures result set, and end (`8dd8b4b2b1aa6469`) |
| contract_details_not_found.txt | 20260405T215022Z | promoted; covers real code 200 not-found error surfaced as `APIError` with `OpContractDetails` |
| qualify_contract_ambiguous.txt | 20260710T220554Z | promoted; exact sv206 request, all 26 exchange-specific MSFT results, and end resolve to `ErrAmbiguousContract` (`4ee1d7204e71e3f1`) |
| api_conditions_matrix_aapl.txt | 20260610T200935Z | promoted; covers all six condition families accepted to PreSubmitted after the field-order fix, the non-terminal off-hours code-399 warning (handle stays open, then closes cleanly on the cancel's Cancelled status), and 5-field cancel acknowledgements; Gateway condition echoes decode fully since the None-sentinel fix (frozen by capture-decode tests) |
| api_order_rest_cancel_161_aapl.txt | 20260610T195745Z | promoted; covers LMT rest/cancel plus the code-161 safety re-cancel remaining a session notice while the handle closes cleanly |
| api_order_stop_cancel_aapl.txt | 20260610T195758Z | promoted; covers STP and STP LMT rest/cancel with Gateway-computed echo limits and why_held=trigger |
| api_order_trailing_cancel_aapl.txt | 20260610T195819Z | promoted; covers TRAIL off-hours partial-then-full fill with cancel-after-fill 10148, TRAIL LIMIT rest/cancel, and the UTC-dash execution times the parser now accepts (both fills and commissions reach the handle in replay) |
| api_order_relative_cancel_aapl.txt | 20260610T195833Z | promoted; covers REL with Gateway-assigned 0.01 offset echo and full cancel lifecycle |
| api_order_rejects_aapl.txt | 20260610T195923Z | promoted; covers 321 invalid order type, price-band 202 cancel text, 10147 unknown-order cancel, 10148 re-cancel, and 161 safety re-cancel |
| current_time_live.txt | 20260611T074046Z | promoted; covers explicit reqCurrentTime with the live epoch reply and the Session snapshot |
| req_ids_read_only.txt | 20260611T074047Z | promoted; freezes the read-only Gateway's code-321 rejection of explicit reqIds as an unsolicited push (no public API sends reqIds; the client frame is omitted and disclosed in the header) and the drop-without-perturbation session surface |
| matching_symbols_partial.txt | 20260611T074053Z | promoted; covers the 97-row partial-pattern symbolSamples reply including BOND issuer ids, round-tripped field-for-field against the capture |
| set_type_switch_while_streaming.txt | 20260611T074112Z | promoted; covers the stream-tied marketDataType(3) push, delayed ticks, mid-stream SetType(Live) acceptance, and 10167 as a session event; the four bare set_type captures add nothing beyond api_market_data_type_cycle.txt and set_type_invalid is inconclusive (driver disconnected before any reply; public API validates 1..4 client-side) |
| current_time_millis.txt | 20260611T091447Z | promoted; covers explicit reqCurrentTimeInMillis (OUT 105, bare id) answered by the live epoch milliseconds (IN 109), both versionless |
| api_whatif_margin_aapl.txt (preview) | 20260610T200009Z | see the AORD-010 row above; nine margin decimals plus commission on the public OpenOrder |
| place_order_mkt_buy_aapl.txt | 20260611T133005Z | promoted; MKT BUY single fill with execution, UTC-dash time, and commission carrying a real negative realized PnL |
| place_order_mkt_sell_aapl.txt | 20260611T133011Z | promoted; MKT SELL fill with the unset-PnL sentinel commission |
| place_order_lmt_buy_aapl.txt | 20260611T133017Z | promoted; far LMT rest and cancel on the current 5-field cancel encoder, closing the April recapture debt |
| api_order_fill_aapl.txt | 20260611T133024Z | promoted; six fill lifecycles with running-average partials, the 41-update executions query with dual dispatch, and the six-161 global-cancel tail |
| api_delayed_success_modify_aapl.txt | 20260611T133046Z | promoted; rest, modify to market, fill, flatten, 161 tail |
| api_order_type_matrix_aapl.txt | 20260611T133103Z | promoted; 22-case order-type matrix incl. silent MOC/LOC/PEG MID/PEG BEST acceptances, PEG BENCH accepted-then-321-on-cancel, and terminal 321/387 rejections |
| api_option_exercise_not_itm_aapl.txt | 20260611T133444Z | promoted; option fill then the silently-dropped 322 not-in-the-money refusal |
| api_option_exercise_server_reject_aapl.txt | 20260611T133636Z | promoted; deep-ITM fill, exercise accepted with the 10349 preset session event and a routeless pseudo-order, then the paper clearing 322 rejection |
| api_fa_replace_non_fa.txt | 20260611T143728Z | promoted; freezes the non-FA blocker: fire-and-forget ReplaceConfig, then the routeless code-321 "FA data operations ignored for non FA customers" the engine drops |
| api_hedge_order_aapl.txt | 20260611T134021Z | promoted; five hedge rules: zero-size beta/pair acceptances with Gateway-computed quantities, two delta 320 rejections, the FX 10063 currency-pair rule |
| api_algo_variants_aapl.txt | 20260415T153524Z | promoted; thirteen-variant algo matrix: seven accepted with Gateway-normalized param echoes, six rejected with real codes 439/443/10255; the two echoes that rode as sanitized raw live-layout frames were converted to typed open_order lines after the codec encoder converged on the live layout (verified value-equal at the public OpenOrder layer) |
| whatif_rejected_10255.txt | 20260705T011725Z | promoted; what-if DarkIce + display size draws the order-targeted 10255 rejection with no echo and must resolve the blocked Preview caller (raw-frame probe capture, frames.log sha256 e0eb615458f396a8) |

## Coverage Gaps: What We Need To Hit

### Security Types Not Yet Live-Tested

| SecType | Blocker | Path Forward |
|---------|---------|-------------|
| OPT | 2026-04-15 probe timed out while streaming chain details | Rerun narrower qualified option probe or subscribe to OPRA data |
| FOP | 2026-04-15 probe timed out | Rerun with a concrete future option contract after qualifying future expiry |
| BAG (combo) | Depends on OPT qualification | Same |
| BOND | Apple issuer ID `e1432232` returned 58 message-18 rows on 2026-07-09 | Contract details and replay are promoted; order and market-data permissions remain separate probes |
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
| SLONG | Needs institutional long/short account | low |

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
| Hedge.Type | live capture + replay promoted | delta/beta/FX/pair outcomes covered by `api_hedge_matrix_aapl.txt` |
| Scale.InitialLevelSize | live probe attempted in `api_tif_attribute_matrix_aapl` | inspect driver events and promote real accept/reject |
| Hedge.DisableAutomaticPrice | none | hedge pricing override |
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

# Live Coverage Matrix

This is the MECE target matrix for live IB Gateway/TWS evidence. It is broader
than the current replay suite and intentionally includes implemented,
partially implemented, deferred, blocked, and official-but-not-yet-implemented
capabilities.

Supporting inventory:

- [`ibkr-api-inventory.md`](ibkr-api-inventory.md) lists official sources,
  official EClient/EWrapper families, current public facade methods, current
  codec message IDs, and known official gaps.
- `cmd/ibkr-capture -list-json` is the executable scenario catalog.
- `testdata/transcripts` is the deterministic replay catalog.

As of 2026-08-25 the tracked replay catalog contains only sv208+ evidence: 95
sv225 fixtures, exact sv208 historical and user-info fixtures, an exact positive
sv211 option-calculation fixture, and one exact sv208–225 version matrix. The executable catalog has
125 scenarios: 92 promoted, 24 candidates, and 9 blocked. Older version numbers in
historical evidence notes explain prior observations; they are not active
support claims or checked-in replay dependencies. Rows that lost a pre208
positive fixture remain candidates or blocked until current positive evidence
exists.

## Status Vocabulary

| Status | Meaning |
|--------|---------|
| `promoted` | A replay transcript or codec capture test currently freezes this behavior. |
| `candidate` | Executable or captured, but still needs review, stronger assertions, or replay promotion. |
| `target` | Required for exhaustive coverage, but no executable capture exists yet. |
| `blocked` | Requires entitlement, market state, account type, or official behavior unavailable in the current paper account. Freeze the real IBKR error when observed. |
| `deferred` | In scope eventually, but deliberately not implemented yet. |
| `out_of_scope` | Official API surface that this project does not plan to expose. |

## Coverage Dimensions

Every matrix row has one primary capability owner. Cross-cutting behavior is
handled through dimensions rather than duplicate rows:

- `source`: public_api, codec, official_eclient, official_ewrapper,
  live_capture, replay
- `risk`: read_only, entitlement_probe, paper_order, marketable_paper_order,
  account_config
- `asset`: STK, OPT, FUT, FOP, CASH, CFD, BAG, BOND, FUND, IND, NEWS
- `lifecycle`: one_shot, stream, cancel, reconnect, multi_client, client_id_0,
  order_handle
- `verification`: live_success, live_error, replay_success, replay_error,
  codec_capture, missing

## Session And Protocol Control

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| SESS-001 | Client lifecycle and readiness | `DialContext`, `Client.Close`, `Client.Done`, `Client.Wait`, `Client.Session`, `Client.SessionEvents`, official `eConnect`, `eDisconnect`, `startApi` | `bootstrap`, `bootstrap_client_id_0`, `handshake_client_id_0.txt`, `grounded_bootstrap.txt` | promoted | Current sv225 raw replays freeze nonzero and client-ID-zero negotiation, managed accounts, next valid ID, and farm-status interleaving. Late open-order/status bootstrap traffic remains covered separately. |
| SESS-002 | Session observation and current time | `Client.CurrentTime`, `Client.CurrentTimeMillis`, official `reqCurrentTime`, `reqCurrentTimeInMillis`, `currentTime`, `currentTimeInMillis`, session events | `current_time`, `current_time_millis`, `grounded_bootstrap.txt`, `current_time_live.txt`, `current_time_millis.txt` | promoted | Current sv225 raw replays freeze seconds (`a9029ff8e7cfed19cab1e3e2eccc4c36d7c91b95aa6aa03f75543bacac454a9e`) and millisecond request/reply paths. The exact bootstrap replay proves farm-status callbacks leave the session snapshot unchanged. Unavailable-current-time behavior remains a watch item. |
| SESS-003 | ID allocation and managed-account refresh | `Orders().RefreshOrderID`, `Client.ManagedAccounts`, official `reqIds`, `reqManagedAccts`, `nextValidId`, `managedAccounts` | `req_ids`, `managed_accounts_refresh`, `req_ids.txt`, `req_ids_read_only.txt`, `managed_accounts_refresh.txt`, bootstrap fixtures | promoted | Current sv225 replays ground both order-ID outcomes: paper returns `NextValidID`, while read-only returns exact req_id=-1/code 321. Managed-account bootstrap and explicit refresh are likewise replayed at sv225. Repeated allocation after orders remains a target variant. |
| SESS-004 | Server control and old auth/redirect hooks | official `setServerLogLevel`, redirect, verify/auth, connectAck | none | out_of_scope | Classified 2026-06-11: no verify/auth, redirect, or connectAck callback was observed across the campaign's capture corpus, and the official client documents the verify/auth family as internal. `setServerLogLevel` controls server-side TWS log verbosity, an operator concern owned by the Gateway UI rather than a client library; callers control their own logging through the configured logger. Market-data reroutes are tracked under MD1/MD2 and are no longer part of this row. |
| SESS-005 | Reconnect and interruption | reconnect policy, transport loss, API 1100/1101/1102 | `api_reconnect_active_order_aapl`, `api_reconnect_active_order_aapl.txt`, `reconnect_oneshot_interrupted.txt`, `quote_stream_reconnect.txt`, direct engine fault tests | candidate | Active GTC order reconnect, order-handle recovery, one-shot interruption, and quote transport replacement use current sv225 frames. Data-lost/data-maintained connectivity behavior is frozen by direct engine fault injection around captured message shapes. Active account streams and broader real-time-bar coverage remain open. |
| SESS-006 | Lifecycle edge contracts | subscription close, context cancel, singleton limits, slow consumer, concurrent one-shots | lifecycle fixtures, including `lifecycle_concurrent_oneshots.txt` | promoted | Current sv225 AAPL and EURUSD frames are rebound and deliberately reordered to freeze concurrent demultiplexing. Malformed registered callbacks retire their entire generation; buffered later rows are dropped and incomplete requests match both `ErrInterrupted` and `*ProtocolError`. Unknown IDs remain nonfatal. |
| SESS-007 | Negotiated protocol train | version gates 208..225, official API 10.48 migration table | `supported_version_matrix.txt`, codec exact vectors, public routing tests, `protocol-audit-sv208-225.md` | promoted | The exact matrix negotiates all 18 supported versions and rejects 207/226. Every protobuf migration through 213 and implemented semantic gate through 225 fails closed at its lower boundary. Request-family boundary exercises remain incomplete; the sv214 outbound UTC suffix is unresolved and sv221 mutation is intentionally outside the API. |

## Accounts, Positions, Portfolio, And PnL

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| ACCT-001 | Account summary | `Accounts().Summary`, `Accounts().SubscribeSummary`, official `reqAccountSummary`, `cancelAccountSummary`, `accountSummary`, `accountSummaryEnd` | `account_summary_snapshot`, `account_summary_stream`, `account_summary.txt`, `account_summary_stream.txt`, `account_summary_disconnect_after_end.txt`, `grounded_account_summary.txt` | promoted | Current sv225 raw replays freeze a four-tag snapshot, a three-tag stream through snapshot-end and cancel, and transport loss immediately after a completed snapshot. The public request exposes the IBKR group and an optional exact returned-account filter. A real FA named-group capture, full tag set, two concurrent live subscriptions, and cancel-before-end remain open. |
| ACCT-002 | Account updates and portfolio | `Accounts().Updates`, `Accounts().SubscribeUpdates`, official `reqAccountUpdates`, `updateAccountValue`, `updatePortfolio`, `updateAccountTime`, `accountDownloadEnd` | `account_updates`, `account_updates.txt` | promoted | The post-regulatory sv225 finite snapshot returned account values, portfolio rows, timestamps, and the end marker. Its replay freezes every callback family, `Billable=0.00 EUR`, and unsubscribe. During-market-order deltas and the one-account timing limitation remain broader streaming targets. |
| ACCT-003 | Account updates multi | `Accounts().UpdatesMulti`, `Accounts().SubscribeUpdatesMulti`, official `reqAccountUpdatesMulti`, `cancelAccountUpdatesMulti`, `accountUpdateMulti`, `accountUpdateMultiEnd` | `account_updates_multi`, `account_updates_multi.txt` | promoted | The current sv225 raw replay freezes the complete ledger-and-NLV snapshot, end, and cancellation. Ledger-and-NLV is caller-configurable. Account/model combinations, cancel mid-stream, and post-trade deltas remain open. |
| ACCT-004 | Positions | `Accounts().Positions`, `Accounts().SubscribePositions`, official `reqPositions`, `cancelPositions`, `position`, `positionEnd` | `positions_snapshot`, `positions_subscription`, `positions_subscription.txt`, `positions_disconnect_after_end.txt`, `grounded_positions.txt` | promoted | Current sv225 replays freeze a sanitized nonempty snapshot, stream rows through `SnapshotComplete`, explicit cancellation and a `CurrentTime` fence. A transport-loss variant closes immediately after the captured end marker. CASH positions and streaming during trades remain open. |
| ACCT-005 | Positions multi | `Accounts().PositionsMulti`, `Accounts().SubscribePositionsMulti`, official `reqPositionsMulti`, `cancelPositionsMulti`, `positionMulti`, `positionMultiEnd` | `positions_multi`, `positions_multi.txt` | promoted | Current sv225 request/position/end/cancel protobuf frames and the native nonempty snapshot are replayed. Model variants and streaming during trades remain open. |
| ACCT-006 | PnL account and single-position streams | `Accounts().SubscribePnL`, `Accounts().SubscribePnLSingle`, official `reqPnL`, `cancelPnL`, `reqPnLSingle`, `cancelPnLSingle`, `pnl`, `pnlSingle` | `pnl`, `pnl_single`, `pnl.txt` | candidate | Both catalog scenarios are public-API-only. The current sv225 account replay freezes a typed nonzero update plus cancellation. PnLSingle replay promotion, model-code variants, invalid conID, and before/during/after-trade transitions remain targets. |
| ACCT-007 | Family codes | `Accounts().FamilyCodes`, official `reqFamilyCodes`, `familyCodes` | `family_codes`, `family_codes.txt` | promoted | The current sv225 raw replay freezes the non-family response as `AccountID="*"` with an empty family code. Named single-family and multi-family accounts remain open. |

## Contracts And Reference Data

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| REF-001 | Contract details and qualification | `Contracts().Details`, `Contracts().Qualify`, official `reqContractDetails`, `contractDetails`, `bondContractDetails`, `contractDetailsEnd` | current stock, option, bond, forex, future, not-found, and ambiguous captures; matching `.txt` fixtures | promoted | Current sv225 raw replays cover stock, cash, option, not-found, ES futures, Apple issuer bonds, and ambiguous MSFT results, including regular/bond response families and no-end error routing. IncludeExpired, external selectors, BAG composition, description, and delta-neutral remain broader variants. |
| REF-002 | Matching symbols | `Contracts().Search`, official `reqMatchingSymbols`, `symbolSamples` | `matching_symbols_aapl`, `matching_symbols_partial`, `matching_symbols.txt`, `matching_symbols_partial.txt` | promoted | Current sv225 raw replays freeze complete AAPL and partial `AA` match sets, including derivative types and issuer-ID bond rows. Broader international patterns remain open. |
| REF-003 | Option chain metadata | `Contracts().SecDefOptParams`, official `reqSecDefOptParams`, `securityDefinitionOptionParameter`, `securityDefinitionOptionParameterEnd` | `sec_def_opt_params_aapl`, `sec_def_opt_params.txt` | promoted | The current sv225 raw replay freezes the complete AAPL response and end marker, including SMART and CBOE. FUT/FOP underlyings, empty exchange, and invalid underlying remain open. |
| REF-004 | Market rules and smart components | `Contracts().MarketRule`, `Contracts().SmartComponents`, official `reqMarketRule`, `reqSmartComponents`, `marketRule`, `smartComponents` | `market_rule`, `smart_components`, `market_rule.txt`, `smart_components.txt` | candidate | Current sv225 raw replays freeze market rule 26 and AAPL's complete smart-component mapping. Additional equity, option, future, invalid-rule, and invalid-BBO variants remain open. |
| REF-005 | Market-depth exchanges | `Contracts().DepthExchanges`, official `reqMktDepthExchanges`, `mktDepthExchanges` | `mkt_depth_exchanges`, `mkt_depth_exchanges.txt` | promoted | The current sv225 raw replay freezes the complete response, including direct depth and SMART aggregate groups across stock, cash, bond, and other security types. Invalid routing implications remain open. |

## Market Data L1

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| MD1-001 | Market data type control | `MarketData().SetType`, official `reqMarketDataType`, `marketDataType` | current set-type scenarios, `quote_snapshot.txt`, `set_type_switch_while_streaming.txt` | promoted | Current sv225 evidence freezes delayed selection, type-3 response, typed delayed prices/sizes, request-scoped 10167, a fenced switch attempt, and cancellation. Public validation rejects values outside 1..4. Entitled type-1 pushes remain open. |
| MD1-002 | Quote snapshots | `MarketData().Quote`, `MarketData().RegulatorySnapshot`, official `reqMktData`, `cancelMktData`, `tickSnapshotEnd` | `quote_snapshot_aapl`, `quote_snapshot.txt`; one-shot regulatory capture | candidate | Current sv225 replay freezes a successful delayed AAPL snapshot and end boundary. The sole newly authorized regulatory attempt is `20260824T195855Z-regulatory_snapshot_aapl_v201_authorized_once` (`bca23abbf9e562746b79b758378fcd6752130c0b489a53fd97db3fac3ba3a2e2`); it returned code 0 and must never be retried. Post-attempt account updates report `Billable=0.00 EUR`. OPT/FUT/CASH snapshots remain broader targets. |
| MD1-003 | Quote streams and generic ticks | `MarketData().SubscribeQuotes`, official tick callbacks and `rerouteMktDataReq` | current quote/generic/duplicate/TickNews scenarios, `api_duplicate_quote_subscriptions_aapl.txt`, `tick_news_aapl.txt`, and `cfd_quote_reroute.txt` | candidate | Current sv225 evidence freezes ordinary and 233/236 generic delayed streams, a public TickNews callback, presence-aware price/size callbacks, request parameters, cancellation, and reconnect fault injection. The current IBM CFD sequence proves the initial request, protobuf reroute, conID request, delayed-data notice, and positive high/low/volume/close callbacks (`ca8fbdf11d260066fb7cd1c3d60e6e44808a54bf6a8fc678f3597bd71a666f1c`). Positive EFP, delta-neutral, odd-lot, and entitled live-price streams remain gaps. |
| MD1-004 | Tick callback edge shapes | official `tickPrice`, `tickSize`, `tickString`, `tickGeneric`, `tickEFP`, `tickOptionComputation`, `tickNews`, `tickReqParams`, `deltaNeutralValidation` | current generic-tick and TickNews captures, option-calculation replay, and source-law vectors | candidate | Current supported-range captures attest price/size, mapped/unmapped generic and string fields, request parameters, option computations, and TickNews. TickEFP, delta-neutral validation, and positive odd-lot callbacks still lack positive replay evidence. |
| MD1-005 | Real-time bars | `MarketData().SubscribeRealTimeBars`, official `reqRealTimeBars`, `cancelRealTimeBars`, `realtimeBar` | `realtime_bars_aapl`, `realtime_bars_api_error.txt` | candidate | The current sv225 replay freezes the exact typed entitlement/request error and terminal routing. Successful bars, RTH variants, cancellation after data, and reconnect remain open. |

## Market Data L2 And Tick-By-Tick

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| MD2-001 | Market depth regular and smart | `MarketData().SubscribeDepth`, official `reqMktDepth`, `cancelMktDepth`, `updateMktDepth`, `updateMktDepthL2`, `rerouteMktDepthReq` | `market_depth_aapl`, `market_depth_aapl_smart`, `market_depth_error.txt` | candidate | Current sv225 evidence ends at exact code 10092. Request/cancel and decoder schemas are tested, but no positive current depth row or protobuf reroute callback exists. Entitlement is a blocker, not positive proof. |
| MD2-002 | Tick-by-tick streams | `MarketData().SubscribeTickByTick`, official request/cancel and tick callbacks | current public scenarios; no retained positive transcript | candidate | Current accounts return typed entitlement failures. Successful Last/AllLast/BidAsk/MidPoint callbacks, request limits, pacing, and cancellation after data remain open and prevent a positive-proof claim. |

## Historical Data

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| HIST-001 | Historical bars | `History().Bars`, official `reqHistoricalData`, `cancelHistoricalData`, `historicalData`, `historicalDataEnd` | current historical scenarios, `historical_bars_sv208.txt`, `historical_bars_subscription_required.txt` | promoted | Exact sv208 capture `fe2fa8c99197756bb7f3753a3c49ab51191efff2230c386c5ca9b8a9ca838849` freezes three positive bars and the end boundary through the public API. Current sv225 requests separately return exact typed code 2188, frozen as an entitlement blocker and classified by `APIError.IsEntitlement`. |
| HIST-002 | Historical keep-up bars | `History().SubscribeBars`, official keepUpToDate, `historicalDataUpdate` | `historical_bars_keepup` | blocked | The latest sv225 market-hours request returned exact typed code 162 before a positive initial batch or `historicalDataUpdate`. A successful update and reconnect behavior remain explicit targets. |
| HIST-003 | Historical schedule | `History().Schedule`, official `whatToShow=SCHEDULE`, `historicalSchedule` | `historical_schedule_aapl`, `historical_schedule_aapl.txt`, `TestHistoricalSchedule`, `TestCaptureDecode_HistoricalSchedule` | promoted | The current sv225 raw replay freezes the AAPL request and complete schedule. Non-US exchanges remain open. |
| HIST-004 | Head timestamp and histogram | `History().HeadTimestamp`, `History().Histogram`, official head/histogram calls | `head_timestamp_aapl`, `histogram_data_aapl`, `head_timestamp.txt` | candidate | Current sv225 evidence promotes a positive head timestamp. Histogram currently ends at code 2188 and has no retained positive transcript. |
| HIST-005 | Historical ticks | `History().Ticks`, official historical midpoint/bidask/last callbacks | current historical-tick scenarios; no retained positive transcript | candidate | Current sv225 requests end at the exact historical entitlement blocker. Successful TRADES, BID_ASK, and MIDPOINT callbacks remain open. |
| HIST-006 | Historical news | `News().Historical`, official historical news callbacks | `api_news_article_aapl`, `news_article.txt` | promoted | Current sv225 capture freezes historical rows, end marker, `.0` time bounds, and article follow-up. Provider combinations and invalid-provider behavior remain open. |

## Orders And Executions

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| ORD-001 | Basic order placement | `Orders().Place`, official `placeOrder`, `openOrder`, `orderStatus` | current order campaigns and sv225 replay fixtures | promoted | Current guarded paper campaigns freeze placement, broker echoes, rejection, fills, replacement, targeted/global cancellation, and cleanup fences across common order types. `IncludeOvernight=true` placed and echoed at sv225; replacing it with false returned exact code 462 even through SDK 10.48.01 and retained true. A fresh explicit-false placement was accepted and broker-canonicalized to absence with `TIF=DAY`, so no codec change is justified. |
| ORD-002 | Direct and handle cancel | `Orders().Cancel`, `OrderHandle.Cancel`, `WithManualCancelTime`, `WithCancelExternalOperator`, `WithCancelManualOrderIndicator`, official `cancelOrder` | `api_order_direct_cancel_aapl`, `api_order_rest_cancel_aapl`, `direct_cancel_order.txt` | promoted | Current sv225 paper replays place nonmarketable orders, require typed open-order echoes, exercise top-level and handle cancellation, and observe terminal `Cancelled`. Compliance metadata is source-grounded and locally validated; non-empty live attestation remains pending. |
| ORD-003 | Replace | `OrderHandle.Replace`, official modify by re-sending `placeOrder` | `api_delayed_success_modify_aapl`, `api_delayed_success_modify_aapl.txt`, `api_order_fill_aapl.txt` | promoted | Current sv225 campaigns freeze accepted price/type modification, late execution delivery through the original handle, and cleanup. Quantity, TIF, forbidden side/contract changes, and mismatched order-ID rejection remain targeted variants. |
| ORD-004 | Global cancel | `Orders().CancelAll`, `WithCancelExternalOperator`, `WithCancelManualOrderIndicator`, official `reqGlobalCancel` | `api_stress_rapid_fire_aapl`, `api_stress_rapid_fire_aapl.txt` | promoted | Exact public campaign coverage places ten distinct resting orders, globally cancels them, and observes every terminal handle. Mixed bracket/OCA/conditional cleanup and non-empty compliance metadata remain live variants. |
| ORD-005 | Open orders | `Orders().Open`, `Orders().SubscribeOpen`, `OpenOrdersSubscription.Refresh`, official open/all/auto-open-order scopes and `orderBound` | existing open/reconnect/cross-client captures and replays; `TestLiveManualTWSOrderBound` capture harness | candidate | Own/all/client-0/cross-client/reconnect/refresh paths are promoted with current protobuf OpenOrder projection and deep-owned mutable payloads. `orderBound` decoding and client-0 routing are official-schema implemented, but the two available applications are Gateways; positive raw evidence still requires paper TWS. |
| ORD-006 | Completed orders | `Orders().Completed`, `Orders().StreamCompleted`, official request/result/end | `api_completed_orders_variants_aapl`, `api_completed_orders_variants_aapl.txt` | promoted | Current sv225 capture freezes cancelled and filled protobuf results, end markers, presence-aware identities, and observed completion fees. Nondefault combo, scale, hedge, delta-neutral, PEG BENCH, condition, and FA branches remain gaps. |
| ORD-007 | Executions and commissions/fees | `Orders().Executions`, execution subscriptions and passive observer | current execution fixtures and `api_order_fill_aapl.txt` | promoted | Current sv225 captures freeze complete execution/fee projection, end-marker correlation, empty and overlapping queries, late reports, and passive observation. Account/symbol filters and required unset day/date fields are attested; finite days/specific dates and rare liquidation/bond values remain targets. |
| ORD-008 | Order handle lifecycle | `OrderHandle.Events`, `OrderHandle.Done`, `OrderHandle.Wait`, `OrderHandle.Close` | `api_reconnect_active_order_aapl.txt`, `api_transmit_false_then_transmit_aapl.txt`, order replay and live tests | promoted | Current replays freeze caller-owned close after terminal status, late execution/fee delivery, detach without cancel, active-order reconnect/open-order recovery, and slow-consumer failure. |
| ORD-009 | End-to-end trading campaign | account, orders, executions, completed orders, PnL, positions | current stress, pairs, DCA, fill, bracket, hedge, and TIF campaign fixtures | candidate | Current guarded sv225 campaigns cover fills, concurrent observers, terminal cleanup, fences, and baseline reconciliation for their owned orders. Scale-in/protective-stop and broader account/PnL transition proof remain targets. |

## Advanced Orders

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| AORD-001 | Brackets and attached orders | `Orders().PlaceBracket`, `Orders().Place`, official bracket/attached order behavior | `api_bracket_place_aapl`, `api_bracket_trigger_aapl`, `api_bracket_trigger_aapl.txt`, `api_bracket_trailing_stop_aapl`, `api_bracket_trailing_stop_aapl.txt` | candidate | Current sv225 paper replays freeze child parent-ID echoes, staged transmit sequencing, terminal cleanup, parent fill, child OCA behavior, and the observed trailing-child rejection. Remaining variants are valid trailing children and natural child-trigger sibling cancellation. |
| AORD-002 | OCA | official OCA group/order type | `api_oca_trigger_aapl`, `api_oca_trigger_aapl.txt` | candidate | `api_oca_trigger_aapl.txt` freezes OCA group echo plus real aggressive-peer PendingCancel/Cancelled price-band rejection; remaining target variants: one fills and cancels peer, far-from-market cleanup, mixed buy/sell |
| AORD-003 | IB algos | official IB algos and TagValue params | `api_algorithmic_campaign_aapl`, `api_algo_variants_aapl`, `api_algo_variants_aapl.txt` | promoted | Current sv225 capture `20260824T203158Z-api_algo_variants_aapl` (`bf33cd7eea5fe75a06666f1af121980c1b5e863e5b5605291162a0b4e1cff291`) freezes the guarded variant matrix, Gateway-normalized echoes, exact rejections, cancellation, and cleanup. |
| AORD-004 | Order conditions | official price/time/margin/execution/volume/percent-change conditions | `api_conditions_matrix_aapl`, `api_conditions_matrix_aapl.txt` | promoted | All six condition families replay-promoted from live capture `87059663ed139026`: each accepted to PreSubmitted with the off-hours code-399 deferral, then cancelled. The run exposed and fixed the contract-bound field-order bug; conditioned open_order echoes now decode fully. Remaining variants: or-conjunction, conditionsCancelOrder=true, and market-hours non-deferred acceptance. |
| AORD-005 | Combo/BAG | official combo legs, combo prices, smart combo routing params | `api_combo_option_vertical_aapl`; source-schema vectors | candidate | Current sv225 capture `20260824T204518Z-api_combo_option_vertical_aapl` freezes a two-leg BAG request and open-order echo with NonGuaranteed routing; the expired-contract rejection is blocker evidence, not an accepted lifecycle. `Contract.ComboLegs` and shared `OrderCombo` placement/open/completed projection are implemented. A current accepted order, nondefault per-leg prices, execution, and completed-order observation remain gaps. |
| AORD-006 | Hedge orders | official hedging | `api_hedge_order_aapl`, `api_hedge_order_aapl.txt` | promoted | Current sv225 capture `20260824T210913Z-api_hedge_order_aapl` (`68514f4d1f92b17ca2141c2cf7834387881e889aabf6aaef2ca2adab6591f242`) freezes accepted and rejected hedge rules, typed terminal errors, and cleanup. Positive delta-hedge behavior remains coupled to the option-account blocker. |
| AORD-007 | Delta-neutral extensions | official delta-neutral order/contract fields and `deltaNeutralValidation` | `tick_efp_probe`; source-schema tests | candidate | Public DeltaNeutral requires BAG. Schema and conversion laws are frozen, but no current accepted BAG delta-neutral quote or order exists. Positive quote, order, open/completed, and validation callbacks remain live gaps. |
| AORD-008 | Scale orders | official scale fields | `api_tif_attribute_matrix_aapl`; official Testbed source-law vectors | candidate | Placement and strict OpenOrder decoding preserve initial/subsequent size, increment, adjust value/interval, profit offset, auto-reset, initial position/fill, random percent, table, and active times. Nondefault live placement/open/completed echoes remain required for promotion. |
| AORD-009 | Pegged and adjusted order families | official PEG BENCH, PEG BEST, PEG MID, adjusted stop/trailing fields | `api_order_type_matrix_aapl`, `api_tif_attribute_matrix_aapl`; official source-law vectors | candidate | Current sv225 protobuf captures freeze the supported request and OpenOrder fields plus exact broker acceptance/rejection outcomes. Correctly framed accepted lifecycles remain required for every specialized pegged and adjusted branch. |
| AORD-010 | Regulatory/allocation order fields | `Orders().Preview`, official FA allocation, MiFID, manual order time, soft-dollar, advancedErrorOverride, IBKRATS | `api_whatif_margin_aapl`, `api_whatif_margin_aapl.txt`, `api_tif_attribute_matrix_aapl` | candidate | Current sv225 capture `20260824T210426Z-api_whatif_margin_aapl` (`686932b92e69fcf4030a9637d82117682be8c76d1039819d048ee58272ae81ee`) freezes `Orders().Preview` outcomes without creating an order lifecycle. MiFID, manual-order-time acceptance, FA allocation, and IBKRATS variants remain open. |

## Options

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| OPT-001 | Option calculations | `Options().ImpliedVolatility`, `Options().Price`, official calculate/cancel calls | `option_calculations_aapl.txt`, exact sv211 schema vectors | promoted | Exact sv211 public capture `59056822b51af4a00caa28afb922b4f79ee7014668591392e8f4fae229ea7222` returns option-price availability `247` and implied-volatility availability `133`. Invalid-contract and cancel-before-result variants remain open. |
| OPT-002 | Option exercise/lapse | `Options().Exercise`, official `exerciseOptions` | current schema/validation tests; exact sv225 market-state blocker, no positive transcript | blocked | Capture `a10ff5818916cad50192579a39ce046143a1123a5a26f51bf359f161a0b5ad2c` qualified a live ITM AAPL call, then warning 399 deferred its zero-fill seed order to the next options session. Fresh-generation reconciliation proved no mutation; this is not proof of settlement or the pseudo-order lifecycle. |
| OPT-003 | Option order and data integration | `Orders().Place`, market data/history for OPT | current option details/calculation and combo captures | candidate | Current sv225 captures qualify real AAPL option contracts, return partial calculation data, and freeze an expired-contract BAG rejection. They do not prove an accepted option order, fill, completed/execution observation, or historical option ticks. |

## News, Scanner, FA, WSH, Display, And TWS

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| NEWS-001 | News providers and bulletins | `News().Providers`, `News().SubscribeBulletins`, official provider/bulletin calls | `news_providers`, `news_providers.txt`; bulletin live scenario | candidate | Current sv225 replay freezes the ordered provider response. A current positive bulletin callback transcript is absent; quiet subscribe/cancel fencing and `allMessages=false` remain targets. |
| NEWS-002 | News article | `News().Article`, official `reqNewsArticle`, `newsArticle` | `api_news_article_aapl`, `news_article.txt` | promoted | Current sv225 capture freezes historical lookup and article follow-up using the captured provider/article ID. Invalid IDs and provider-specific errors remain open. |
| NEWS-003 | Historical news | `News().Historical`, official historical news callbacks | `historical_news_aapl`, `historical_news_aapl_timezone_window`, `api_news_article_aapl` | promoted | see HIST-006 |
| SCAN-001 | Scanner parameters | `Scanner().Parameters`, official `reqScannerParameters`, `scannerParameters` | `scanner_parameters` | candidate | The current sv225 public capture decoded the full 1.8 MB XML response (`02ca289379189356eacedc56576be6d863e4a2b46feb8c28386c36ac07948ba7`). Checking in the volatile schema would add megabytes without a durable public invariant. An unavailable-service response remains a useful replay target. |
| SCAN-002 | Scanner subscriptions | `Scanner().SubscribeResults`, official subscription/cancel callbacks | `api_scanner_subscription`, `scanner_subscription.txt` | promoted | Current sv225 capture `20260824T202618Z-api_scanner_subscription` returned ten HOT_BY_VOLUME rows followed by public cancel and a `CurrentTime` fence (`fcf02a594c900dd73089d4da12f7e8509b37ecd13bf92c652496515470d40775`). Non-default filters and empty-result variants remain gaps. |
| FA-001 | FA request config | `Advisors().Config`, official `requestFA`, `receiveFA` | `request_fa` | candidate | Current sv225 evidence freezes the non-FA account's exact code-321 refusal as a typed terminal `OpFAConfig` result followed by a current-time fence. Successful groups/profiles/aliases documents remain blocked on an FA account. |
| FA-003 | Soft-dollar tiers | `Advisors().SoftDollarTiers`, official `reqSoftDollarTiers`, `softDollarTiers` | `soft_dollar_tiers`, `soft_dollar_tiers.txt` | promoted | The current sv225 raw replay freezes the empty tier list. A real non-empty tier list remains open. |
| WSH-001 | WSH metadata | `WSH().MetaData`, official request/cancel/callback | `wsh_meta_data`, `api_wsh_variants_aapl`, `api_wsh_variants_aapl.txt`, `wsh_meta_data_error.txt` | candidate | Current sv225 replays freeze exact code-10276 entitlement refusal and request variants. Success and cancel-after-data remain open. |
| WSH-002 | WSH event data | `WSH().EventData`, official request/cancel/callback | `wsh_event_data_aapl`, `api_wsh_variants_aapl`, `api_wsh_variants_aapl.txt` | candidate | Current sv225 campaign freezes conID, portfolio, watchlist, competitor, and date-window variants returning exact code 10276. Success and cancel-after-data remain targets. |
| TWS-001 | User info | `TWS().UserInfo`, official `reqUserInfo`, `userInfo` | `user_info`, `user_info.txt` | promoted | Exact sv208 capture `672370162ad17e46cf045647775d1d6bc4480353b2f044392c43431a88717bd5` freezes the classic request shape, callback ID 107, and an empty white-branding ID. |
| TWS-002 | Display groups | `TWS().DisplayGroups`, `TWS().SubscribeDisplayGroup`, `DisplayGroupHandle.Update`, official display group calls | `display_groups`, `display_group_subscribe`, `display_group_subscribe.txt` | candidate | Current sv225 captures query the real group list, subscribe to returned group 1, receive typed initial value `none`, unsubscribe, wait, and fence. A clean live update of a nonempty group plus invalid group/update cases remain gaps. |
| TWS-003 | Configuration snapshot | `TWS().Config`, official `reqConfig`/`config` | `tws_config`; exact-sv219 codec and public routing tests | promoted | The exact official SDK response is frozen by hash `928ada9da43be6e7`; native public capture `3b5a3dee08cb7cae` returned the typed messages, API settings, order settings, and trusted IP list. Pointer-valued fields preserve omitted versus explicit zero/false. Configuration mutation remains out of scope. |

## Error, Entitlement, And Negative Behavior

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| ERR-001 | API errors and farm status | official `error` callback, system status codes | `contract_details_not_found.txt`, `realtime_bars_api_error.txt`, `set_type_switch_while_streaming.txt`, `grounded_bootstrap.txt` | promoted | Exact request-scoped one-shot code-200 and subscription-closing code-420 refusals, session-scoped statuses, warning-only continuation, and advanced order reject JSON. Duplicate symbolic error/warning/farm replays were removed. |
| ERR-002 | Disconnect during operations | transport and protocol disconnects | `error_disconnect_during_snapshot.txt`, `reconnect_oneshot_interrupted.txt`, `quote_stream_disconnect.txt` | promoted | every one-shot and every stream family has at least one disconnect behavior row |
| ERR-003 | Entitlement and account-type failures | market data, history, WSH, FA, scanner, orders | current blocker fixtures including `market_depth_error.txt`, `historical_bars_subscription_required.txt`, and WSH replays | candidate | Current sv225 evidence freezes exact depth code 10092, historical code 2188, market-data code 10089, WSH code 10276, and role/account errors. These are typed blockers, not positive service proof. |

## Non-Goals

| ID | Capability | Status | Reason |
|----|------------|--------|--------|
| NG-001 | Flex Web Service | out_of_scope | Explicit project non-goal. |
| NG-002 | Client Portal Web API | out_of_scope | Explicit project non-goal. |
| NG-003 | EWrapper/EClient compatibility bridge | out_of_scope | Explicit project non-goal; official names are inventory references only. |

## Executable Scenario Catalog Coverage

Each executable `cmd/ibkr-capture` scenario must appear in this section and in
one primary matrix row above.

| Scenario | Primary Row |
|----------|-------------|
| `account_summary_snapshot` | ACCT-001 |
| `account_summary_stream` | ACCT-001 |
| `account_summary_two_subs` | ACCT-001 |
| `account_updates` | ACCT-002 |
| `account_updates_multi` | ACCT-003 |
| `api_algo_variants_aapl` | AORD-003 |
| `api_algorithmic_campaign_aapl` | ORD-009 |
| `api_bracket_place_aapl` | AORD-001 |
| `api_bracket_trailing_stop_aapl` | AORD-001 |
| `api_bracket_trigger_aapl` | AORD-001 |
| `api_client_id0_order_observation_aapl` | ORD-005 |
| `api_combo_option_vertical_aapl` | AORD-005 |
| `api_completed_orders_variants_aapl` | ORD-006 |
| `api_conditions_matrix_aapl` | AORD-004 |
| `api_cross_client_cancel_aapl` | ORD-005 |
| `api_delayed_success_modify_aapl` | ORD-001 |
| `api_dollar_cost_averaging_aapl` | ORD-009 |
| `api_duplicate_quote_subscriptions_aapl` | MD1-003 |
| `api_generic_tick_matrix_aapl` | MD1-003, MD1-004 |
| `api_tick_news_aapl_probe` | MD1-003, MD1-004 |
| `api_forex_lifecycle_eurusd` | ORD-001 |
| `api_future_campaign_mes` | ORD-001 |
| `api_hedge_order_aapl` | AORD-006 |
| `api_historical_matrix_aapl` | HIST-001 |
| `api_include_overnight_lifecycle_aapl` | ORD-001 |
| `api_ioc_fok_aapl` | ORD-001 |
| `api_market_data_completeness_aapl` | MD1-003 |
| `api_news_article_aapl` | NEWS-002 |
| `api_option_calculations_aapl` | MD1-004, OPT-001 |
| `api_option_exercise_aapl` | OPT-002 |
| `api_oca_trigger_aapl` | AORD-002 |
| `api_order_fill_aapl` | ORD-001 |
| `api_order_direct_cancel_aapl` | ORD-002 |
| `api_order_rejects_aapl` | ORD-001 |
| `api_order_relative_cancel_aapl` | ORD-001 |
| `api_order_rest_cancel_aapl` | ORD-001 |
| `api_order_stop_cancel_aapl` | ORD-001 |
| `api_order_trailing_cancel_aapl` | ORD-001 |
| `api_option_campaign_aapl` | OPT-003 |
| `api_order_type_matrix_aapl` | ORD-001 |
| `api_pairs_trading_aapl_msft` | ORD-009 |
| `api_reconnect_active_order_aapl` | SESS-005 |
| `api_scale_in_campaign_aapl` | ORD-009 |
| `api_security_type_probe_matrix` | REF-001 |
| `api_stop_loss_management_aapl` | ORD-009 |
| `api_stress_rapid_fire_aapl` | ORD-009 |
| `api_tif_attribute_matrix_aapl` | ORD-001 |
| `api_transmit_false_then_transmit_aapl` | ORD-001 |
| `api_whatif_margin_aapl` | AORD-010 |
| `api_wsh_variants_aapl` | WSH-001, WSH-002 |
| `bootstrap` | SESS-001 |
| `bootstrap_client_id_0` | SESS-001 |
| `completed_orders` | ORD-006 |
| `contract_details_aapl_opt` | REF-001 |
| `contract_details_aapl_stk` | REF-001 |
| `contract_details_apple_bonds` | REF-001 |
| `contract_details_concurrent` | REF-001 |
| `contract_details_es_fut` | REF-001 |
| `contract_details_eurusd_cash` | REF-001 |
| `contract_details_not_found` | REF-001 |
| `current_time` | SESS-002 |
| `current_time_millis` | SESS-002 |
| `display_group_subscribe` | TWS-002 |
| `display_groups` | TWS-002 |
| `executions_snapshot` | ORD-007 |
| `executions_concurrent_aapl` | ORD-007 |
| `family_codes` | ACCT-007 |
| `head_timestamp_aapl` | HIST-004 |
| `histogram_data_aapl` | HIST-004 |
| `historical_bars_1d_1h` | HIST-001 |
| `historical_bars_30d_1day` | HIST-001 |
| `historical_bars_bidask` | HIST-001 |
| `historical_bars_error` | HIST-001 |
| `historical_bars_keepup` | HIST-002 |
| `historical_news_aapl` | HIST-006 |
| `historical_news_aapl_timezone_window` | HIST-006 |
| `historical_schedule_aapl` | HIST-003 |
| `historical_ticks_aapl_bidask` | HIST-005 |
| `historical_ticks_aapl_midpoint` | HIST-005 |
| `historical_ticks_aapl_timezone_start` | HIST-005 |
| `historical_ticks_aapl_trades` | HIST-005 |
| `market_depth_aapl` | MD2-001 |
| `market_depth_aapl_smart` | MD2-001 |
| `market_rule` | REF-004 |
| `managed_accounts_refresh` | SESS-001 |
| `matching_symbols_aapl` | REF-002 |
| `matching_symbols_partial` | REF-002 |
| `mkt_depth_exchanges` | REF-005 |
| `news_bulletins` | NEWS-001 |
| `news_providers` | NEWS-001 |
| `open_orders_all` | ORD-005 |
| `open_orders_empty` | ORD-005 |
| `pnl` | ACCT-006 |
| `pnl_single` | ACCT-006 |
| `positions_multi` | ACCT-005 |
| `positions_snapshot` | ACCT-004 |
| `positions_subscription` | ACCT-004 |
| `qualify_contract_aapl_exact` | REF-001 |
| `qualify_contract_ambiguous` | REF-001 |
| `quote_snapshot_aapl` | MD1-002 |
| `quote_odd_lot_aapl` | MD1-003 |
| `quote_stream_aapl` | MD1-003 |
| `quote_stream_genericticks` | MD1-003 |
| `quote_stream_multi_asset` | MD1-003 |
| `realtime_bars_aapl` | MD1-005 |
| `req_ids` | SESS-003 |
| `request_fa` | FA-001 |
| `scanner_parameters` | SCAN-001 |
| `api_scanner_subscription` | SCAN-002 |
| `sec_def_opt_params_aapl` | REF-003 |
| `set_type_delayed` | MD1-001 |
| `set_type_delayed_frozen` | MD1-001 |
| `set_type_frozen` | MD1-001 |
| `set_type_live` | MD1-001 |
| `set_type_switch_while_streaming` | MD1-001 |
| `smart_components` | REF-004 |
| `soft_dollar_tiers` | FA-003 |
| `tick_by_tick_bidask` | MD2-002 |
| `tick_by_tick_last` | MD2-002 |
| `tick_by_tick_midpoint` | MD2-002 |
| `tick_efp_probe` | MD1-004 |
| `tws_config` | TWS-003 |
| `user_info` | TWS-001 |
| `wsh_event_data_aapl` | WSH-002 |
| `wsh_meta_data` | WSH-001 |

### Retired Historical Evidence

The `fundamental_data_aapl` and `api_fundamental_reports_aapl` scenarios and
the `fundamental_data.txt` and `api_fundamental_report_errors_aapl.txt`
replays covered live `server_version 200` behavior before IBKR API 10.47
removed the feature. The 2026-04-15 report campaign remains traceable to
capture hash prefix `02649216ff69f306`, including mixed XML success and real
code 430 responses. Final 2026-07-09 captures sent all seven legacy reports
through both local roles and received code 10358 for every request:
readonly-live hash prefix `89db59e9e5abf7b7`, paper-dev hash prefix
`c326f314cbc4f1de`. These artifacts are historical evidence, not executable
scenarios, active replays, or current coverage targets.

## Immediate Target Scenario Gaps

These gaps block any claim that the matrix is fully executable:

- `MarketData().SetType`: bare data types 1, 2, 3, and 4 are replay-promoted;
  add stream callback replays, invalid type, and switch-while-streaming
  behavior.
- `Options().Exercise`: add a positive exercise/lapse completion scenario;
  current sv225 evidence has not produced an acceptable lifecycle replay.
- Official callback gaps from `ibkr-api-inventory.md`: `tickEFP`; raw
  paper-TWS evidence for implemented `orderBound`. Verification callbacks are
  official-internal and remain outside the public charter.

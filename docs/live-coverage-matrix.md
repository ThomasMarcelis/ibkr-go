# Live Coverage Matrix

This matrix tracks live IB Gateway/TWS evidence by capability. It is broader
than the current replay suite and intentionally includes implemented,
partially implemented, deferred, blocked, and official-but-not-yet-implemented
capabilities.

Supporting inventory:

- [`ibkr-api-inventory.md`](ibkr-api-inventory.md) maps the official
  EClient/EWrapper surface to what `ibkr-go` implements.
- `cmd/ibkr-capture -list-json` is the executable scenario catalog.
- `testdata/transcripts` is the deterministic replay catalog.

As of 2026-09-05 the tracked replay corpus contains 115 live-derived sv208+
transcripts. The executable catalog has 125 scenarios: 104 promoted, no
candidates, and 21 explicitly blocked by entitlements, account type, market
state, or TWS-only interaction. A blocked successful callback stays blocked
even when its exact broker refusal has a replay; blocker frames are not
positive decoder attestation. Historical version notes are not active support
claims.

## Status Vocabulary

| Status | Meaning |
|--------|---------|
| `promoted` | A replay transcript or codec capture test currently freezes this behavior. |
| `candidate` | Executable or captured, but still needs review, stronger assertions, or replay promotion. No current catalog scenario has this status. |
| `target` | Required for exhaustive coverage, but no executable capture exists yet. |
| `blocked` | Requires entitlement, market state, account type, or official behavior unavailable in the current paper account. Freeze the real IBKR error when observed. |
| `deferred` | In scope eventually, but deliberately not implemented yet. |
| `out_of_scope` | Official API surface that this project does not plan to expose. |

## Coverage Dimensions

Capability rows describe positive behavior; a promoted refusal scenario does
not promote the successful callback it blocks. Read a row's evidence and
remaining variants alongside its status.

Every matrix row has one primary capability owner. The following review
dimensions help assess coverage; they are not fields of the executable catalog:

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
| SESS-005 | Reconnect and interruption | reconnect policy, transport loss, API 1100/1101/1102 | `api_reconnect_active_order_aapl`, `api_reconnect_active_order_aapl.txt`, `reconnect_oneshot_interrupted.txt`, `quote_stream_reconnect.txt`, `delayed_quote_reconnect.txt`, direct engine fault tests | promoted | Active GTC order reconnect, order-handle recovery, one-shot interruption, and quote transport replacement use current sv225 frames. A September sv225 proxy-outage capture proves delayed quote data returns after restoring selection. Data-lost/data-maintained connectivity behavior is frozen by direct engine fault injection around captured message shapes. |
| SESS-006 | Lifecycle edge contracts | subscription close, context cancel, singleton limits, slow consumer, concurrent one-shots | lifecycle fixtures, including `lifecycle_concurrent_oneshots.txt` | promoted | Current sv225 AAPL and EURUSD frames are rebound and deliberately reordered to freeze concurrent demultiplexing. Malformed registered callbacks retire their entire generation; buffered later rows are dropped and incomplete requests match both `ErrInterrupted` and `*ProtocolError`. Unknown IDs remain nonfatal. |
| SESS-007 | Negotiated protocol train | version gates 208..225, official API 10.50.01 schemas | `supported_version_matrix.txt`, codec exact vectors, public routing tests, `protocol-audit-sv208-225.md` | promoted | The exact matrix negotiates all 18 supported versions and rejects 207/226. Handshake and current-time requests are frozen at every supported version. Native/SDK message vectors cover selected layouts at 208, 209, 210, 211, 212, 213, 215, 219, and 225; source-gate tests cover additional boundaries. This is not every operation at every version. API 10.50.01 settlement field 65 is live-replayed; its order field 145 requires sv226 and is outside support. |

## Accounts, Positions, Portfolio, And PnL

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| ACCT-001 | Account summary | `Accounts().Summary`, `Accounts().SubscribeSummary`, official `reqAccountSummary`, `cancelAccountSummary`, `accountSummary`, `accountSummaryEnd` | `account_summary_snapshot`, `account_summary_stream`, `account_summary.txt`, `account_summary_stream.txt`, `account_summary_disconnect_after_end.txt`, `grounded_account_summary.txt` | promoted | Current sv225 raw replays freeze a four-tag snapshot, a three-tag stream through snapshot-end and cancel, and transport loss immediately after a completed snapshot. The public request exposes the IBKR group and an optional exact returned-account filter. A real FA named-group capture, full tag set, two concurrent live subscriptions, and cancel-before-end remain open. |
| ACCT-002 | Account updates and portfolio | `Accounts().Updates`, `Accounts().SubscribeUpdates`, official `reqAccountUpdates`, `updateAccountValue`, `updatePortfolio`, `updateAccountTime`, `accountDownloadEnd` | `account_updates`, `account_updates.txt` | promoted | The post-regulatory sv225 finite snapshot returned account values, portfolio rows, timestamps, and the end marker. Its replay freezes every callback family, `Billable=0.00 EUR`, and unsubscribe. During-market-order deltas and the one-account timing limitation remain broader streaming targets. |
| ACCT-003 | Account updates multi | `Accounts().UpdatesMulti`, `Accounts().SubscribeUpdatesMulti`, official `reqAccountUpdatesMulti`, `cancelAccountUpdatesMulti`, `accountUpdateMulti`, `accountUpdateMultiEnd` | `account_updates_multi`, `account_updates_multi.txt` | promoted | The current sv225 raw replay freezes the complete ledger-and-NLV snapshot, end, and cancellation. Ledger-and-NLV is caller-configurable. Account/model combinations, cancel mid-stream, and post-trade deltas remain open. |
| ACCT-004 | Positions | `Accounts().Positions`, `Accounts().SubscribePositions`, official `reqPositions`, `cancelPositions`, `position`, `positionEnd` | `positions_snapshot`, `positions_subscription`, `positions_subscription.txt`, `positions_disconnect_after_end.txt`, `grounded_positions.txt` | promoted | Current sv225 replays freeze a sanitized nonempty snapshot, stream rows through `SnapshotComplete`, explicit cancellation and a `CurrentTime` fence. A transport-loss variant closes immediately after the captured end marker. CASH positions and streaming during trades remain open. |
| ACCT-005 | Positions multi | `Accounts().PositionsMulti`, `Accounts().SubscribePositionsMulti`, official `reqPositionsMulti`, `cancelPositionsMulti`, `positionMulti`, `positionMultiEnd` | `positions_multi`, `positions_multi.txt` | promoted | Current sv225 request/position/end/cancel protobuf frames and the native nonempty snapshot are replayed. Model variants and streaming during trades remain open. |
| ACCT-006 | PnL account and single-position streams | `Accounts().SubscribePnL`, `Accounts().SubscribePnLSingle`, official `reqPnL`, `cancelPnL`, `reqPnLSingle`, `cancelPnLSingle`, `pnl`, `pnlSingle` | `pnl`, `pnl_single`, `pnl.txt`, `pnl_single.txt` | promoted | Current sv225 replays freeze typed nonzero account and held-position updates, cancellation, and protocol fences. |
| ACCT-007 | Family codes | `Accounts().FamilyCodes`, official `reqFamilyCodes`, `familyCodes` | `family_codes`, `family_codes.txt` | promoted | The current sv225 raw replay freezes the non-family response as `AccountID="*"` with an empty family code. Named single-family and multi-family accounts remain open. |

## Contracts And Reference Data

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| REF-001 | Contract details and qualification | `Contracts().Details`, `Contracts().Qualify`, official `reqContractDetails`, `contractDetails`, `bondContractDetails`, `contractDetailsEnd` | current stock, option, bond, forex, future, not-found, and ambiguous captures; matching `.txt` fixtures | promoted | Current sv225 raw replays cover stock, cash, option, not-found, ES futures, Apple issuer bonds, and ambiguous MSFT results, including regular/bond response families and no-end error routing. IncludeExpired, external selectors, BAG composition, description, and delta-neutral remain broader variants. |
| REF-002 | Matching symbols | `Contracts().Search`, official `reqMatchingSymbols`, `symbolSamples` | `matching_symbols_aapl`, `matching_symbols_partial`, `matching_symbols.txt`, `matching_symbols_partial.txt` | promoted | Current sv225 raw replays freeze complete AAPL and partial `AA` match sets, including derivative types and issuer-ID bond rows. Broader international patterns remain open. |
| REF-003 | Option chain metadata | `Contracts().SecDefOptParams`, official `reqSecDefOptParams`, `securityDefinitionOptionParameter`, `securityDefinitionOptionParameterEnd` | `sec_def_opt_params_aapl`, `sec_def_opt_params.txt` | promoted | The current sv225 raw replay freezes the complete AAPL response and end marker, including SMART and CBOE. FUT/FOP underlyings, empty exchange, and invalid underlying remain open. |
| REF-004 | Market rules and smart components | `Contracts().MarketRule`, `Contracts().SmartComponents`, official `reqMarketRule`, `reqSmartComponents`, `marketRule`, `smartComponents` | `market_rule`, `smart_components`, `market_rule.txt`, `smart_components.txt` | promoted | Current sv225 raw replays freeze market rule 26 and AAPL's complete smart-component mapping. |
| REF-005 | Market-depth exchanges | `Contracts().DepthExchanges`, official `reqMktDepthExchanges`, `mktDepthExchanges` | `mkt_depth_exchanges`, `mkt_depth_exchanges.txt` | promoted | The current sv225 raw replay freezes the complete response, including direct depth and SMART aggregate groups across stock, cash, bond, and other security types. Invalid routing implications remain open. |

## Market Data L1

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| MD1-001 | Market data type control | `MarketData().SetType`, official `reqMarketDataType`, `marketDataType` | current set-type scenarios, `quote_snapshot.txt`, `set_type_switch_while_streaming.txt`, `delayed_quote_reconnect.txt` | promoted | Current sv225 evidence freezes delayed selection, type-3 response, typed delayed prices/sizes, request-scoped 10167, a fenced switch attempt, and cancellation. Public validation rejects values outside 1..4. Entitled type-1 pushes remain open. |
| MD1-002 | Quote snapshots | `MarketData().Quote`, `MarketData().RegulatorySnapshot`, official `reqMktData`, `cancelMktData`, `tickSnapshotEnd` | `quote_snapshot_aapl`, `quote_snapshot.txt`, `regulatory_snapshot_aapl_error.txt` | promoted | Current sv225 replay freezes a successful delayed AAPL snapshot and end boundary. The sole authorized regulatory attempt returned code 0 and is replayed from `bca23abbf9e562746b79b758378fcd6752130c0b489a53fd97db3fac3ba3a2e2`; it must never be retried. Post-attempt account updates report `Billable=0.00 EUR`. |
| MD1-003 | Quote streams and generic ticks | `MarketData().SubscribeQuotes`, official tick callbacks and `rerouteMktDataReq` | current quote/generic/duplicate/TickNews scenarios, `quote_stream_genericticks.txt`, `quote_stream_multi_asset.txt`, `api_duplicate_quote_subscriptions_aapl.txt`, `tick_news_aapl.txt`, `cfd_quote_reroute.txt` | promoted | Current sv225 replays freeze ordinary and 233/236 generic delayed streams, concurrent AAPL/EUR.USD data, TickNews, presence-aware price/size callbacks, request parameters, cancellation, reconnect fault injection, and the complete positive IBM CFD reroute sequence. |
| MD1-004 | Tick callback edge shapes | official `tickPrice`, `tickSize`, `tickString`, `tickGeneric`, `tickEFP`, `tickOptionComputation`, `tickNews`, `tickReqParams`, `deltaNeutralValidation` | current generic-tick and TickNews captures, option-calculation replay, and source-law vectors | blocked | Supported-range captures attest price/size, mapped/unmapped generic and string fields, request parameters, option computations, and TickNews. Positive TickEFP, delta-neutral validation, and odd-lot callbacks require unavailable products or entitlements. |
| MD1-005 | Real-time bars | `MarketData().SubscribeRealTimeBars`, official `reqRealTimeBars`, `cancelRealTimeBars`, `realtimeBar` | `realtime_bars_aapl`, `realtime_bars_api_error.txt` | blocked | The current sv225 replay freezes the exact typed entitlement/request error and terminal routing; no positive bar callback is available. |

## Market Data L2 And Tick-By-Tick

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| MD2-001 | Market depth regular and smart | `MarketData().SubscribeDepth`, official `reqMktDepth`, `cancelMktDepth`, `updateMktDepth`, `updateMktDepthL2`, `rerouteMktDepthReq` | `market_depth_aapl`, `market_depth_aapl_smart`, `market_depth_error.txt` | blocked | Current sv225 evidence ends at exact code 10092. Request/cancel and decoder schemas are tested, but no positive current depth row or protobuf reroute callback exists. |
| MD2-002 | Tick-by-tick streams | `MarketData().SubscribeTickByTick`, official request/cancel and tick callbacks | current public scenarios; no retained positive transcript | blocked | Current accounts return typed entitlement failures; no Last/AllLast/BidAsk/MidPoint callback is available. |

## Historical Data

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| HIST-001 | Historical bars | `History().Bars`, official `reqHistoricalData`, `cancelHistoricalData`, `historicalData`, `historicalDataEnd` | current historical scenarios, `historical_bars_sv208.txt`, `historical_bars_subscription_required.txt` | promoted | Exact sv208 capture `fe2fa8c99197756bb7f3753a3c49ab51191efff2230c386c5ca9b8a9ca838849` freezes three positive bars and the end boundary through the public API. Current sv225 requests separately return exact typed code 2188, frozen as an entitlement blocker and classified by `APIError.IsEntitlement`. |
| HIST-002 | Historical keep-up bars | `History().SubscribeBars`, official keepUpToDate, `historicalDataUpdate` | `historical_bars_keepup` | blocked | The latest sv225 market-hours request returned exact typed code 162 before a positive initial batch or `historicalDataUpdate`. A successful update and reconnect behavior remain explicit targets. |
| HIST-003 | Historical schedule | `History().Schedule`, official `whatToShow=SCHEDULE`, `historicalSchedule` | `historical_schedule_aapl`, `historical_schedule_aapl.txt`, `TestHistoricalSchedule` | promoted | The current sv225 raw replay freezes the AAPL request and complete schedule. Non-US exchanges remain open. |
| HIST-004 | Head timestamp and histogram | `History().HeadTimestamp`, `History().Histogram`, official head/histogram calls | `head_timestamp_aapl`, `histogram_data_aapl`, `head_timestamp.txt` | blocked | A positive head timestamp is promoted; histogram ends at exact code 2188 and has no positive callback. |
| HIST-005 | Historical ticks | `History().Ticks`, official historical midpoint/bidask/last callbacks | current historical-tick scenarios; no retained positive transcript | blocked | Current sv225 requests end at the exact historical entitlement blocker; no midpoint, bid/ask, or last batch is available. |
| HIST-006 | Historical news | `News().Historical`, official historical news callbacks | `api_news_article_aapl`, `news_article.txt` | promoted | Current sv225 capture freezes historical rows, end marker, `.0` time bounds, and article follow-up. Provider combinations and invalid-provider behavior remain open. |

## Orders And Executions

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| ORD-001 | Basic order placement | `Orders().Place`, official `placeOrder`, `openOrder`, `orderStatus` | current order campaigns, `api_empty_tif_default_aapl`, `empty_tif_default_aapl.txt`, and sv225 replay fixtures | promoted | An order with empty `TIF` is sent as `DAY`: the omitted-field request drew exact code 10052 at sv225 (capture `a6d1543d90a099e14011adc9a2e4fac984d7ac4f512a49ec391e86a0aab5859b`) and the DAY request, echo, and cancellation are replayed from `50351d930e5f74c0974f6bd2553caacc2c7ca2e160f217177a78553a6cd45101`. Current guarded paper campaigns freeze placement, broker echoes, rejection, fills, replacement, targeted/global cancellation, and cleanup fences across common order types. `IncludeOvernight=true` placed and echoed at sv225; replacing it with false returned exact code 462 even through SDK 10.48.01 and retained true. A fresh explicit-false placement was accepted and broker-canonicalized to absence with `TIF=DAY`, so no codec change is justified. |
| ORD-002 | Direct and handle cancel | `Orders().Cancel`, `OrderHandle.Cancel`, `WithManualCancelTime`, `WithCancelExternalOperator`, `WithCancelManualOrderIndicator`, official `cancelOrder` | `api_order_direct_cancel_aapl`, `api_order_rest_cancel_aapl`, `direct_cancel_order.txt` | promoted | Current sv225 paper replays place nonmarketable orders, require typed open-order echoes, exercise top-level and handle cancellation, and observe terminal `Cancelled`. Compliance metadata is source-grounded and locally validated; non-empty live attestation remains pending. |
| ORD-003 | Replace | `OrderHandle.Replace`, official modify by re-sending `placeOrder` | `api_delayed_success_modify_aapl`, `api_delayed_success_modify_aapl.txt`, `api_order_fill_aapl.txt` | promoted | Current sv225 campaigns freeze accepted price/type modification, late execution delivery through the original handle, and cleanup. Quantity, TIF, forbidden side/contract changes, and mismatched order-ID rejection remain targeted variants. |
| ORD-004 | Global cancel | `Orders().CancelAll`, `WithCancelExternalOperator`, `WithCancelManualOrderIndicator`, official `reqGlobalCancel` | `api_stress_rapid_fire_aapl`, `api_stress_rapid_fire_aapl.txt` | promoted | Exact public campaign coverage places ten distinct resting orders, globally cancels them, and observes every terminal handle. Mixed bracket/OCA/conditional cleanup and non-empty compliance metadata remain live variants. |
| ORD-005 | Open orders | `Orders().Open`, `Orders().SubscribeOpen`, `OpenOrdersSubscription.Refresh`, official open/all/auto-open-order scopes and `orderBound` | existing open/reconnect/cross-client captures and replays; `TestLiveManualTWSOrderBound` capture harness | blocked | Own/all/client-0/cross-client/reconnect/refresh paths are promoted with current protobuf OpenOrder projection and deep-owned mutable payloads. Positive `orderBound` evidence requires paper TWS and a user-created manual order; the available endpoints are Gateways. |
| ORD-006 | Completed orders | `Orders().Completed`, `Orders().StreamCompleted`, official request/result/end | `api_completed_orders_variants_aapl`, `api_completed_orders_variants_aapl.txt` | promoted | Current sv225 capture freezes cancelled and filled protobuf results, end markers, presence-aware identities, and observed completion fees. Nondefault combo, scale, hedge, delta-neutral, PEG BENCH, condition, and FA branches remain gaps. |
| ORD-007 | Executions and commissions/fees | `Orders().Executions`, execution subscriptions and passive observer | current execution fixtures and `api_order_fill_aapl.txt` | promoted | Current sv225 captures freeze complete execution/fee projection, end-marker correlation, empty and overlapping queries, late reports, and passive observation. Account/symbol filters and required unset day/date fields are attested; finite days/specific dates and rare liquidation/bond values remain targets. |
| ORD-008 | Order handle lifecycle | `OrderHandle.Events`, `OrderHandle.Done`, `OrderHandle.Wait`, `OrderHandle.Close` | `api_reconnect_active_order_aapl.txt`, `api_transmit_false_then_transmit_aapl.txt`, order replay and live tests | promoted | Current replays freeze caller-owned close after terminal status, late execution/fee delivery, detach without cancel, active-order reconnect/open-order recovery, and slow-consumer failure. |
| ORD-009 | End-to-end trading campaign | account, orders, executions, completed orders, PnL, positions | current stress, pairs, DCA, fill, bracket, hedge, TIF, scale-in, stop-loss, and algorithmic campaign fixtures | promoted | Guarded sv225 replays cover fills, concurrent observers, scale and protective-stop fields, replacement, cancellation, exact flattening, fees, fences, completed orders, and baseline reconciliation. The algorithmic campaign is grounded in `2e258d840f3e29dfd7341cebecfbf5815c02dc5147c53b237b9cb82a9977fa86`; stop-loss management in `147873593a5fd3e3975e6bf3e53aaac80985fd1839a4a5f50dafa4cd99c91993`. |

## Advanced Orders

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| AORD-001 | Brackets and attached orders | `Orders().PlaceBracket`, `Orders().Place`, official bracket/attached order behavior | `api_bracket_place_aapl`, `api_bracket_place_aapl.txt`, `api_bracket_trigger_aapl.txt`, `api_bracket_trailing_stop_aapl.txt` | promoted | Current sv225 paper replays freeze child parent-ID echoes, staged transmit sequencing, terminal cleanup, parent fill, child OCA behavior, and observed trailing-child rejection. |
| AORD-002 | OCA | official OCA group/order type | `api_oca_trigger_aapl`, `api_oca_trigger_aapl.txt` | promoted | The replay freezes OCA group echo and the observed aggressive-peer cancellation/rejection lifecycle. |
| AORD-003 | IB algos | official IB algos and TagValue params | `api_algorithmic_campaign_aapl`, `api_algorithmic_campaign_aapl.txt`, `api_algo_variants_aapl.txt` | promoted | Current sv225 replays freeze the guarded algo variant matrix and the full multi-observer algorithmic campaign, including Gateway-normalized echoes, split fills, replacement, cancellation, fees, cleanup, and reconciliation. |
| AORD-004 | Order conditions | official price/time/margin/execution/volume/percent-change conditions | `api_conditions_matrix_aapl`, `api_conditions_matrix_aapl.txt` | promoted | All six condition families replay-promoted from live capture `87059663ed139026`: each accepted to PreSubmitted with the off-hours code-399 deferral, then cancelled. The run exposed and fixed the contract-bound field-order bug; conditioned open_order echoes now decode fully. Remaining variants: or-conjunction, conditionsCancelOrder=true, and market-hours non-deferred acceptance. |
| AORD-005 | Combo/BAG | official combo legs, combo prices, smart combo routing params | `api_combo_option_vertical_aapl`, `api_combo_option_vertical_aapl.txt`; source-schema vectors | promoted | Capture `c6f88ef6599771686097980c0c89f41734d0341ffd5468fa5aba4ff29502ce1f` freezes a live-qualified two-leg AAPL call vertical with per-leg prices, no combo-level limit, NonGuaranteed routing, accepted `PreSubmitted` echo, zero fill, and targeted cancellation. |
| AORD-006 | Hedge orders | official hedging | `api_hedge_order_aapl`, `api_hedge_order_aapl.txt` | promoted | Current sv225 capture `20260824T210913Z-api_hedge_order_aapl` (`68514f4d1f92b17ca2141c2cf7834387881e889aabf6aaef2ca2adab6591f242`) freezes accepted and rejected hedge rules, typed terminal errors, and cleanup. Positive delta-hedge behavior remains coupled to the option-account blocker. |
| AORD-007 | Delta-neutral extensions | official delta-neutral order/contract fields and `deltaNeutralValidation` | `tick_efp_probe`; source-schema tests | blocked | Public DeltaNeutral requires BAG. Schema and conversion laws are frozen, but a positive validation callback requires an unavailable entitled product. |
| AORD-008 | Scale orders | official scale fields | `api_scale_in_campaign_aapl`, `api_scale_in_campaign_aapl.txt`; official source-law vectors | promoted | Capture `4b71d719a68b4e8ec91d3bae803dec2144ab2c2f3c64961fc880deac089efe52` freezes nondefault scale-field echo, two fills, protective-stop cancellation, exact flatten, and execution/fee reconciliation. |
| AORD-009 | Pegged and adjusted order families | official PEG BENCH, PEG BEST, PEG MID, adjusted stop/trailing fields | `api_order_type_matrix_aapl`, `api_tif_attribute_matrix_aapl`; official source-law vectors | promoted | Current sv225 protobuf captures freeze supported request and OpenOrder fields plus exact broker acceptance or rejection outcomes. |
| AORD-010 | Regulatory/allocation order fields | `Orders().Preview`, official FA allocation, MiFID, manual order time, soft-dollar, advancedErrorOverride, IBKRATS | `api_whatif_margin_aapl`, `api_whatif_margin_aapl.txt`, `api_tif_attribute_matrix_aapl` | promoted | Capture `686932b92e69fcf4030a9637d82117682be8c76d1039819d048ee58272ae81ee` freezes public what-if outcomes without creating an order lifecycle. Account-specific FA/MiFID echoes remain externally unavailable. |

## Options

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| OPT-001 | Option calculations | `Options().ImpliedVolatility`, `Options().Price`, official calculate/cancel calls | `option_calculations_aapl.txt`, exact sv211 schema vectors | promoted | Exact sv211 public capture `59056822b51af4a00caa28afb922b4f79ee7014668591392e8f4fae229ea7222` returns option-price availability `247` and implied-volatility availability `133`. Invalid-contract and cancel-before-result variants remain open. |
| OPT-002 | Option exercise/lapse | `Options().Exercise`, official `exerciseOptions` | `api_option_exercise_aapl`, `api_option_exercise_aapl.txt` | promoted | Capture `37bfe1e3c3494f54e2f953936996086ecd31f9d7f2f0d6cb8ef2dd2a2323d4e2` freezes the option seed fill, exact warning 10349, `PreSubmitted` admission, and captured EOF as `ExerciseUncertainError`. It proves accepted-but-unsettled admission only and makes no terminal exercise, lapse, or settlement claim. Do not repeat the live instruction. |
| OPT-003 | Option order and data integration | `Orders().Place`, market data/history for OPT | option details/calculation, accepted option seed fill, and accepted combo captures | promoted | Current sv225 replays qualify AAPL options, expose settlement method, fill the bounded option seed, and accept a live-qualified vertical BAG. Historical option ticks remain entitlement-blocked. |

## News, Scanner, FA, WSH, Display, And TWS

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| NEWS-001 | News providers and bulletins | `News().Providers`, `News().SubscribeBulletins`, official provider/bulletin calls | `news_providers`, `news_providers.txt`; bulletin live scenario | blocked | The ordered provider response is replayed. No bulletin event occurred during the bounded live subscription, so the callback remains unattested. |
| NEWS-002 | News article | `News().Article`, official `reqNewsArticle`, `newsArticle` | `api_news_article_aapl`, `news_article.txt` | promoted | Current sv225 capture freezes historical lookup and article follow-up using the captured provider/article ID. Invalid IDs and provider-specific errors remain open. |
| NEWS-003 | Historical news | `News().Historical`, official historical news callbacks | `historical_news_aapl`, `historical_news_aapl_timezone_window`, `api_news_article_aapl` | promoted | see HIST-006 |
| SCAN-001 | Scanner parameters | `Scanner().Parameters`, official `reqScannerParameters`, `scannerParameters` | `scanner_parameters` | promoted | The current sv225 public capture decoded the full 1.8 MB XML response (`02ca289379189356eacedc56576be6d863e4a2b46feb8c28386c36ac07948ba7`). The volatile body is intentionally not checked in because it adds no stable public invariant. |
| SCAN-002 | Scanner subscriptions | `Scanner().SubscribeResults`, official subscription/cancel callbacks | `api_scanner_subscription`, `scanner_subscription.txt` | promoted | Current sv225 capture `20260824T202618Z-api_scanner_subscription` returned ten HOT_BY_VOLUME rows followed by public cancel and a `CurrentTime` fence (`fcf02a594c900dd73089d4da12f7e8509b37ecd13bf92c652496515470d40775`). Non-default filters and empty-result variants remain gaps. |
| FA-001 | FA request config | `Advisors().Config`, official `requestFA`, `receiveFA` | `request_fa` | blocked | Current sv225 evidence freezes the non-FA account's exact code-321 refusal and fence. A positive document requires an FA account. |
| FA-003 | Soft-dollar tiers | `Advisors().SoftDollarTiers`, official `reqSoftDollarTiers`, `softDollarTiers` | `soft_dollar_tiers`, `soft_dollar_tiers.txt` | promoted | The current sv225 raw replay freezes the empty tier list. A real non-empty tier list remains open. |
| WSH-001 | WSH metadata | `WSH().MetaData`, official request/cancel/callback | `wsh_meta_data`, `api_wsh_variants_aapl`, `api_wsh_variants_aapl.txt`, `wsh_meta_data_error.txt` | blocked | Current sv225 replays freeze exact code-10276 entitlement refusal and request variants; no positive metadata callback is available. |
| WSH-002 | WSH event data | `WSH().EventData`, official request/cancel/callback | `wsh_event_data_aapl`, `api_wsh_variants_aapl`, `api_wsh_variants_aapl.txt` | blocked | Current sv225 replay freezes conID, portfolio, watchlist, competitor, and date-window variants returning exact code 10276; no positive event callback is available. |
| TWS-001 | User info | `TWS().UserInfo`, official `reqUserInfo`, `userInfo` | `user_info`, `user_info.txt` | promoted | Exact sv208 capture `672370162ad17e46cf045647775d1d6bc4480353b2f044392c43431a88717bd5` freezes the classic request shape, callback ID 107, and an empty white-branding ID. |
| TWS-002 | Display groups | `TWS().DisplayGroups`, `TWS().SubscribeDisplayGroup`, `DisplayGroupHandle.Update`, official display group calls | `display_groups`, `display_group_subscribe`, `display_group_subscribe.txt` | promoted | Current sv225 captures query the real group list, subscribe to group 1, receive typed initial value `none`, unsubscribe, and fence. |
| TWS-003 | Configuration snapshot | `TWS().Config`, official `reqConfig`/`config` | `tws_config`; exact-sv219 codec and public routing tests | promoted | The exact official SDK response is frozen by hash `928ada9da43be6e7`; native public capture `3b5a3dee08cb7cae` returned the typed messages, API settings, order settings, and trusted IP list. Pointer-valued fields preserve omitted versus explicit zero/false. Configuration mutation remains out of scope. |

## Error, Entitlement, And Negative Behavior

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| ERR-001 | API errors and farm status | official `error` callback, system status codes | `contract_details_not_found.txt`, `realtime_bars_api_error.txt`, `set_type_switch_while_streaming.txt`, `grounded_bootstrap.txt` | promoted | Exact request-scoped one-shot code-200 and subscription-closing code-420 refusals, session-scoped statuses, warning-only continuation, and advanced order reject JSON. Duplicate symbolic error/warning/farm replays were removed. |
| ERR-002 | Disconnect during operations | transport and protocol disconnects | `error_disconnect_during_snapshot.txt`, `reconnect_oneshot_interrupted.txt`, `quote_stream_disconnect.txt` | promoted | every one-shot and every stream family has at least one disconnect behavior row |
| ERR-003 | Entitlement and account-type failures | market data, history, WSH, FA, scanner, orders | current blocker fixtures including `market_depth_error.txt`, `historical_bars_subscription_required.txt`, and WSH replays | promoted | Current sv225 evidence freezes exact depth code 10092, historical code 2188, market-data code 10089, WSH code 10276, and role/account errors. These promote typed error behavior, not the unavailable success callbacks. |

## Non-Goals

| ID | Capability | Status | Reason |
|----|------------|--------|--------|
| NG-001 | Flex Web Service | out_of_scope | Explicit project non-goal. |
| NG-002 | Client Portal Web API | out_of_scope | Explicit project non-goal. |
| NG-003 | EWrapper/EClient compatibility bridge | out_of_scope | Explicit project non-goal; official names are inventory references only. |

## Executable Scenario Catalog Coverage

Each executable `cmd/ibkr-capture` scenario appears here with its owning matrix
row or rows above.

| Scenario | Matrix Row(s) |
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
| `api_empty_tif_default_aapl` | ORD-001 |
| `api_generic_tick_matrix_aapl` | MD1-003, MD1-004 |
| `api_tick_news_aapl_probe` | MD1-003, MD1-004 |
| `api_forex_lifecycle_eurusd` | ORD-001 |
| `api_future_campaign_mes` | ORD-001 |
| `api_hedge_order_aapl` | AORD-006 |
| `api_historical_matrix_aapl` | HIST-001 |
| `api_include_overnight_lifecycle_aapl` | ORD-001 |
| `api_ioc_fok_aapl` | ORD-001 |
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
| `managed_accounts_refresh` | SESS-003 |
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

IBKR API 10.47 removed fundamental data. The scenarios and replays that
covered it were retired with the v2.0.1 corpus migration and remain at
that tag; they are not executable evidence for the supported range.

## External Evidence Boundaries

Every executable scenario is promoted or explicitly blocked. Remaining
positive callbacks require entitlements, account types, market state, or
paper-TWS interaction unavailable in the current Gateway setup. The retained
option-exercise replay proves accepted-but-unsettled admission and must not be
repeated merely to seek settlement. Verification callbacks are
official-internal and outside the public charter.

## September consumer observations and evidence limits

The ibkr-mobile measurements (2026-08-29 through 2026-09-05, sv225) use one
paper account with delayed data. They supplement the retained captures; they
are not promoted replay fixtures or universal entitlement rules.

| Operation | Consumer observation | Interpretation |
|---|---|---|
| Keep-up bars | 2188 refusal | This login could not prove positive live bar updates. |
| Tick-by-tick | 10089 refusal | Other retained sessions returned 10189; both are entitlement observations. |
| SMART depth | Notice 2152 followed by no rows within 15 seconds | The notice alone does not prove permanent unavailability. |
| Option quote | Intermittent 10091 despite earlier data for the same tier/contracts | Preserve the code and caller policy; do not infer a permanent entitlement state or universal retry permission. |
| News article | 10172, no data available | That request failed; future availability is not proven impossible. |
| Option computations | 10–13 seen; 80–83 not seen; model tick 13 carried greeks while other ticks carried price/dividend fields | Field presence and negotiated data type matter more than assuming a callback family. Type 4 did not work on this login. |
| Option calculation | ConID-only got 321 “Please enter exchange”; qualified requests worked | Price-from-volatility included greeks; volatility-from-price returned IV. Both requests now validate Exchange. |
| Generic ticks | RTVolume absent with 233 requested; delayed volume differed markedly from consolidated-volume observations | No scaling or invented RTVolume result. Volume feeds and measurement windows are not interchangeable. |
| P&L | Account-updates and dedicated reqPnL differed in the same second | Sources have different update/reset schedules and scope; this is not sufficient evidence of a codec bug. |
| All-scope open orders | Another client's newly placed order appeared on Refresh, not as an unsolicited push | All is a snapshot request. Gateway master-client configuration governs broader push delivery; client-0 auto-binding is a separate feature. |

The retained scanner XML (1,801,494 bytes) and the consumer's later XML
(1,801,525 bytes) are different captures. `TestScannerParametersPublicCaptured`
proves the retained document's public collector; it does not freeze the changing
catalog's counts. `TestPositionsMultiOneShotCaptured` proves the multi-position
collector. `TestHistoricalLastTicksPublicProjectionCaptured` proves positive
TRADES projection at sv215; MIDPOINT/BID_ASK success remains unproven here.

`TestEncodeRetainedClassicRequests` adds 23 outbound request types using retained
sv208/210/211 client bytes. Request encoding, handshake negotiation, positive
decoder layout, and a complete public operation are separate evidence levels.
The decoder ledger still records 86 attested layouts and 20 pending positives.

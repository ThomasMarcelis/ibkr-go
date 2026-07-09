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
| SESS-001 | Client lifecycle and readiness | `DialContext`, `Client.Close`, `Client.Done`, `Client.Wait`, `Client.Session`, `Client.SessionEvents`, official `eConnect`, `eDisconnect`, `startApi` | `bootstrap`, `bootstrap_client_id_0`, `handshake.txt`, `handshake_client_id_0.txt`, `grounded_bootstrap.txt` | promoted | client ID 0/nonzero, server version negotiation, managed accounts, next valid ID, farm-status interleaving, late open-order/status bootstrap traffic |
| SESS-002 | Session observation and current time | `Client.CurrentTime`, `Client.CurrentTimeMillis`, official `reqCurrentTime`, `reqCurrentTimeInMillis`, `currentTime`, `currentTimeInMillis`, session events | `current_time`, `current_time_millis`, `bootstrap_with_farm_status.txt`, `error_farm_status_codes.txt`, `current_time.txt`, `current_time_live.txt` | promoted | Explicit `reqCurrentTime` with the live epoch reply and the `Session().CurrentTime` snapshot frozen from the 2026-06-11 readonly-live capture (`efe321755946a395`); farm-status no-state-change covered by the read-only rejection replay. `reqCurrentTimeInMillis` (OUT 105 / IN 109, server_version 197+) implemented per the official client and pending its first live capture. Unavailable current-time behavior has no live evidence and stays a watch item. |
| SESS-003 | ID allocation and managed-account refresh | `Orders().RefreshOrderID`, official `reqIds`, `reqManagedAccts`, `nextValidId`, `managedAccounts` | `req_ids`, `req_ids.txt`, `req_ids_read_only.txt`, bootstrap fixtures | promoted | `Orders().RefreshOrderID` is grounded both ways: the paper Gateway answers NEXT_VALID_ID (`req_ids.txt`), while the read-only live Gateway rejects with req_id=-1/code 321 (`req_ids_read_only.txt`, capture hash `00b11cbce4cefc31`). The successful ID updates the actor-owned allocation seed; the returned value remains engine-owned. Managed-account refresh and repeated allocation after orders remain target variants. |
| SESS-004 | Server control and old auth/redirect hooks | official `setServerLogLevel`, redirect, verify/auth, connectAck, reroute callbacks | none | out_of_scope | Classified 2026-06-11: no verify/auth, redirect, connectAck, or reroute callback was ever observed across the campaign's capture corpus (both Gateway roles, server_version 200), and the official client documents the verify/auth family as internal. `setServerLogLevel` controls server-side TWS log verbosity, an operator concern owned by the Gateway UI rather than a client library; callers control their own logging through the configured logger. Reopen if a live Gateway ever emits one of these callbacks. |
| SESS-005 | Reconnect and interruption | reconnect policy, transport loss, API 1100/1101/1102 | `api_reconnect_active_order_aapl`, `api_reconnect_active_order_aapl.txt`, `api_order_handle_reconnect_cancel_aapl.txt`, `reconnect_policy_off.txt`, `reconnect_oneshot_interrupted.txt`, `reconnect_1100_then_transport_loss.txt`, `reconnect_1102_resume.txt`, `reconnect_gap_no_resume.txt`, `reconnect_multi_cycle.txt`, `quote_stream_gap_1101.txt`, `quote_stream_gap_1102.txt`, `quote_stream_reconnect.txt`, `quote_stream_disconnect.txt`, `realtime_bars_reconnect.txt` | promoted | active GTC order reconnect and original order-handle Gap/Resumed/cancel are replay-promoted; remaining variants: active account streams, historical keep-up bars |
| SESS-006 | Lifecycle edge contracts | subscription close, context cancel, singleton limits, slow consumer, concurrent one-shots | `lifecycle_subscription_close_immediate.txt`, `lifecycle_singleton_reject.txt`, `lifecycle_context_cancel.txt`, `lifecycle_concurrent_oneshots.txt`, `lifecycle_bootstrap_reordered.txt`, `lifecycle_bootstrap_no_valid_id.txt`, `lifecycle_bootstrap_no_accounts.txt`, `lifecycle_account_summary_limit.txt`, `lifecycle_set_mdt_after_close.txt` | promoted | add live-derived versions for singleton/account limits where possible |

## Accounts, Positions, Portfolio, And PnL

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| ACCT-001 | Account summary | `Accounts().Summary`, `Accounts().SubscribeSummary`, official `reqAccountSummary`, `cancelAccountSummary`, `accountSummary`, `accountSummaryEnd` | `account_summary_snapshot`, `account_summary_stream`, `account_summary_two_subs`, `account_summary.txt`, `account_summary_stream.txt`, `account_summary_two_subs.txt`, `account_summary_disconnect_after_end.txt`, `grounded_account_summary.txt` | promoted | All vs concrete account, full tag set, two concurrent subs, cancel before and after end |
| ACCT-002 | Account updates and portfolio | `Accounts().Updates`, `Accounts().SubscribeUpdates`, official `reqAccountUpdates`, `updateAccountValue`, `updatePortfolio`, `updateAccountTime`, `accountDownloadEnd` | `account_updates`, `account_updates.txt` | promoted | baseline, during marketable trades, multiple asset positions, unsubscribe, one-account official timing limitation |
| ACCT-003 | Account updates multi | `Accounts().UpdatesMulti`, `Accounts().SubscribeUpdatesMulti`, official `reqAccountUpdatesMulti`, `cancelAccountUpdatesMulti`, `accountUpdateMulti`, `accountUpdateMultiEnd` | `account_updates_multi`, `account_updates_multi.txt` | promoted | account/model combinations, empty model, cancel mid-stream, post-trade deltas |
| ACCT-004 | Positions | `Accounts().Positions`, `Accounts().SubscribePositions`, official `reqPositions`, `cancelPositions`, `position`, `positionEnd` | `positions_snapshot`, `positions.txt`, `positions_disconnect_after_end.txt`, `grounded_positions.txt` | promoted | empty and non-empty accounts, STK/OPT/FUT/CASH positions, streaming during trades |
| ACCT-005 | Positions multi | `Accounts().PositionsMulti`, `Accounts().SubscribePositionsMulti`, official `reqPositionsMulti`, `cancelPositionsMulti`, `positionMulti`, `positionMultiEnd` | `positions_multi`, `positions_multi.txt` | promoted | account/model variants, empty result, streaming during trades |
| ACCT-006 | PnL account and single-position streams | `Accounts().SubscribePnL`, `Accounts().SubscribePnLSingle`, official `reqPnL`, `cancelPnL`, `reqPnLSingle`, `cancelPnLSingle`, `pnl`, `pnlSingle` | `pnl`, `pnl_single`, `pnl.txt`, `pnl_single.txt` | promoted | before/during/after trades, invalid conID, model code, open option/future positions |
| ACCT-007 | Family codes | `Accounts().FamilyCodes`, official `reqFamilyCodes`, `familyCodes` | `family_codes`, `family_codes.txt` | promoted | single-account and multi-family accounts; non-family account response |

## Contracts And Reference Data

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| REF-001 | Contract details and qualification | `Contracts().Details`, `Contracts().Qualify`, official `reqContractDetails`, `contractDetails`, `bondContractDetails`, `contractDetailsEnd` | `contract_details_aapl_stk`, `contract_details_aapl_opt`, `contract_details_eurusd_cash`, `contract_details_es_fut`, `contract_details_not_found`, `qualify_contract_aapl_exact`, `qualify_contract_ambiguous`, `api_security_type_probe_matrix`, `api_security_type_probe_errors.txt`, `contract_details.txt`, `grounded_contract_details_aapl.txt`, `contract_details_aapl_opt.txt`, `contract_details_eurusd_cash.txt`, `contract_details_es_fut.txt`, `contract_details_not_found.txt`, `qualify_contract_ambiguous.txt` | promoted | Asset-type matrix replay-promoted 2026-06-10: OPT chain subset (strike ladder, call/put legs), CASH single match, FUT expiry ladder freezing a full-session lastTradeDate timestamp (`20261218 08:30:00 US/Central`), not-found code 200, ambiguous qualify `ErrAmbiguousContract`. BOND/BILL code 200 errors replay-promoted from capture `9be83e57ed176a17`; BOND success details stay blocked (no bond entitlement evidence). Remaining variants: FOP success rows, BAG, expired/includeExpired, FUND/IND success details. The multiplier decode gap found here is fixed: the slot between minTick and orderTypes decodes into `ContractDetails`, frozen by capture-decode tests on the OPT (100) and FUT (50) frames. |
| REF-002 | Matching symbols | `Contracts().Search`, official `reqMatchingSymbols`, `symbolSamples` | `matching_symbols_aapl`, `matching_symbols_partial`, `matching_symbols.txt`, `matching_symbols_partial.txt` | promoted | broad pattern, exact-ish pattern, derivative sec types, description/issuer fields; the 97-row partial-pattern reply with BOND issuer ids is frozen from the 2026-06-11 readonly-live capture (`00fdf812de6ae616`) |
| REF-003 | Option chain metadata | `Contracts().SecDefOptParams`, official `reqSecDefOptParams`, `securityDefinitionOptionParameter`, `securityDefinitionOptionParameterEnd` | `sec_def_opt_params_aapl`, `sec_def_opt_params.txt` | promoted | STK underlyings, FUT/FOP underlyings, empty exchange, invalid underlying |
| REF-004 | Market rules and smart components | `Contracts().MarketRule`, `Contracts().SmartComponents`, official `reqMarketRule`, `reqSmartComponents`, `marketRule`, `smartComponents` | `market_rule`, `smart_components`, `market_rule.txt`, `smart_components.txt` | promoted | US equity, option, future, invalid market rule, invalid BBO exchange |
| REF-005 | Market-depth exchanges | `Contracts().DepthExchanges`, official `reqMktDepthExchanges`, `mktDepthExchanges` | `mkt_depth_exchanges`, `mkt_depth_exchanges.txt` | promoted | all returned service data types, SMART support, invalid routing implication |
| REF-006 | Fundamental data | `Contracts().FundamentalData`, official `reqFundamentalData`, `cancelFundamentalData`, `fundamentalData` | `fundamental_data_aapl`, `api_fundamental_reports_aapl`, `fundamental_data.txt`, `api_fundamental_report_errors_aapl.txt` | candidate | all `FundamentalReportType` values, entitlement error, invalid report type, cancel path. `ReportRatios` and `ReportsFinStatements` code 430 responses are replay-promoted from 2026-04-15 capture `02649216ff69f306`; large successful XML report payloads remain capture evidence only. |

## Market Data L1

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| MD1-001 | Market data type control | `MarketData().SetType`, official `reqMarketDataType`, `marketDataType` | `set_type_live`, `set_type_frozen`, `set_type_delayed`, `set_type_delayed_frozen`, `set_type_invalid`, `set_type_switch_while_streaming`, `api_market_data_completeness_aapl`, `api_market_data_type_cycle.txt`, `quote_delayed_data.txt`, `lifecycle_set_mdt_after_close.txt`, `set_type_switch_while_streaming.txt` | promoted | Bare SetType(1/2/3/4) accepted silently (2026-04-15 capture `f692fc168a53da9d`); the 2026-06-11 readonly-live captures confirm the four bare variants stay ack-less and add the stream-tied push: marketDataType(3) arrives before the first tick, mid-stream SetType(Live) is accepted with the stream staying delayed in the captured window, and 10167 surfaces as a session event (`e244cae7f5eb7d57`). Invalid type 99 never reaches the wire from the public API (client-side validation, frozen by unit test); the 2026-06-11 invalid capture is inconclusive on Gateway behavior and is not promoted. Real-time (type 1) success pushes still need a market-hours entitled stream. |
| MD1-002 | Quote snapshots | `MarketData().Quote`, official `reqMktData`, `cancelMktData`, `tickSnapshotEnd` | `quote_snapshot_aapl`, `api_market_data_completeness_aapl`, `quote_snapshot.txt`, `quote_delayed_data.txt` | promoted | STK/OPT/FUT/CASH snapshots, entitlement error, no data, regulatory snapshot where applicable |
| MD1-003 | Quote streams and generic ticks | `MarketData().SubscribeQuotes`, official tick callbacks | `quote_stream_aapl`, `quote_stream_genericticks`, `quote_with_generic_ticks`, `quote_stream_multi_asset`, `api_market_data_completeness_aapl`, `api_duplicate_quote_subscriptions_aapl`, `api_duplicate_quote_subscriptions_aapl.txt`, `quote_with_generic_ticks.txt` | candidate | price/size/string/generic/option/EFP/news/dividend/shortable/RTVolume/fundamental-ratio generic tick families; duplicate same-contract subscriptions replay-promoted from 2026-04-15 capture `84f1e78a18616e0f` with SetType(Delayed) and independent delayed bid/ask streams |
| MD1-004 | Tick callback edge shapes | official `tickPrice`, `tickSize`, `tickString`, `tickGeneric`, `tickEFP`, `tickOptionComputation`, `tickNews`, `tickReqParams` | `calc_implied_volatility.txt`, `calc_option_price.txt`, quote fixtures | target | tickEFP, tickNews, tickReqParams, option computation live success/error |
| MD1-005 | Real-time bars | `MarketData().SubscribeRealTimeBars`, official `reqRealTimeBars`, `cancelRealTimeBars`, `realtimeBar` | `realtime_bars_aapl`, `api_market_data_completeness_aapl`, `api_realtime_bars_request_errors_aapl.txt`, `realtime_bars_reconnect.txt` | promoted | TRADES/MIDPOINT/BID_ASK, RTH true/false, cancel, reconnect. AAPL TRADES/BID_ASK/MIDPOINT request-scoped error variants are replay-promoted from 2026-04-15 capture `f692fc168a53da9d`; live success streams still need live-derived grounding. |

## Market Data L2 And Tick-By-Tick

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| MD2-001 | Market depth regular and smart | `MarketData().SubscribeDepth`, official `reqMktDepth`, `cancelMktDepth`, `updateMktDepth`, `updateMktDepthL2` | `market_depth_aapl`, `market_depth_aapl_smart`, `market_depth_error.txt` | candidate | L1 vs L2 rows, SMART depth, insert/update/delete, market maker names, entitlement errors, cancel |
| MD2-002 | Tick-by-tick streams | `MarketData().SubscribeTickByTick`, official `reqTickByTickData`, `cancelTickByTickData`, tick-by-tick callbacks | `tick_by_tick_last`, `tick_by_tick_bidask`, `tick_by_tick_midpoint`, `api_market_data_completeness_aapl`, `api_tick_by_tick_entitlement_errors_aapl.txt`, `tick_by_tick.txt` | candidate | Last, AllLast, BidAsk, MidPoint, ignoreSize true/false, numberOfTicks, pacing, unavailable data. AAPL Last/AllLast/AllLast ignore-size/BidAsk/MidPoint code 10089 entitlement errors are replay-promoted from 2026-04-15 capture `f692fc168a53da9d`; live success streams remain pending. |

## Historical Data

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| HIST-001 | Historical bars | `History().Bars`, official `reqHistoricalData`, `cancelHistoricalData`, `historicalData`, `historicalDataEnd` | `historical_bars_1d_1h`, `historical_bars_30d_1day`, `historical_bars_bidask`, `historical_bars_error`, `api_historical_matrix_aapl`, `historical_bars.txt`, `grounded_historical_bars.txt` | candidate | every supported bar-size family, durations, RTH true/false, TRADES/BID/ASK/BID_ASK/MIDPOINT/ADJUSTED_LAST, errors |
| HIST-002 | Historical keep-up bars | `History().SubscribeBars`, official keepUpToDate, `historicalDataUpdate` | `historical_bars_keepup`, `historical_bars_stream.txt` | promoted | live updates, cancel path, reconnect behavior |
| HIST-003 | Historical schedule | `History().Schedule`, official `whatToShow=SCHEDULE`, `historicalSchedule` | `historical_schedule_aapl`, `historical_schedule_aapl.txt`, `TestHistoricalSchedule`, `TestCaptureDecode_HistoricalSchedule` | candidate | grounded live decode from server_version 200, captures/20260411T175212Z events.jsonl sha256 1b207a57180e6197; extend to non-US exchanges |
| HIST-004 | Head timestamp and histogram | `History().HeadTimestamp`, `History().Histogram`, official head/histogram calls | `head_timestamp_aapl`, `histogram_data_aapl`, `head_timestamp.txt`, `histogram_data.txt` | promoted | RTH true/false, whatToShow variants, invalid contract, entitlement errors |
| HIST-005 | Historical ticks | `History().Ticks`, official historical midpoint/bidask/last callbacks | `historical_ticks_aapl_trades`, `historical_ticks_aapl_bidask`, `historical_ticks_aapl_midpoint`, `historical_ticks_aapl_timezone_window`, historical tick transcripts | promoted | start-only, end-only, explicit zone, UTC/local zone, no-data, tick attributes, ignoreSize |
| HIST-006 | Historical news | `News().Historical`, official `reqHistoricalNews`, historical news callbacks | `historical_news_aapl`, `historical_news_aapl_timezone_window`, `api_news_article_aapl`, `historical_news.txt`, `historical_news_timezone_window.txt` | promoted | provider combinations, timezone windows, no-result window, invalid provider, article follow-up |

## Orders And Executions

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| ORD-001 | Basic order placement | `Orders().Place`, official `placeOrder`, `openOrder`, `orderStatus` | `place_order_lmt_buy_aapl`, `place_order_mkt_buy_aapl`, `place_order_mkt_sell_aapl`, `api_order_fill_aapl`, `api_order_rest_cancel_aapl`, `api_order_relative_cancel_aapl`, `api_order_stop_cancel_aapl`, `api_order_trailing_cancel_aapl`, `api_order_rejects_aapl`, `api_order_type_matrix_aapl`, `api_tif_attribute_matrix_aapl`, `api_transmit_false_then_transmit_aapl`, `api_transmit_false_then_transmit_aapl.txt`, `api_delayed_success_modify_aapl`, `api_future_campaign_mes`, `api_future_campaign_mes.txt`, `api_forex_lifecycle_eurusd`, `api_forex_lifecycle_eurusd.txt`, `api_ioc_fok_aapl`, `api_ioc_fok_aapl.txt`, `api_tif_attribute_matrix_aapl.txt`, `place_order_limit.txt`, `place_order_fill_with_execution.txt`, `place_order_modify_to_market_late_execution.txt`, `place_order_invalid_type_live_error.txt`, `api_order_rest_cancel_161_aapl.txt`, `api_order_stop_cancel_aapl.txt`, `api_order_trailing_cancel_aapl.txt`, `api_order_relative_cancel_aapl.txt`, `api_order_rejects_aapl.txt` | promoted | The 2026-06-11 market window froze the family: MKT buy/sell fills with executions, commissions, and UTC-dash times; far-LMT rest/cancel on the current encoder; the fill campaign with six lifecycles and the 41-update executions query; delayed modify-to-market fills; and the 22-case order-type matrix (fills for MKT/TRAIL/MIT/LIT/MTL/modify, rests for STP/STP LMT/TRAIL LIMIT/REL, price-band cancel for marketable LMT, silent acceptance for MOC/LOC/PEG MID/PEG BEST, terminal 321/387 rejections for FEELINGS/MOO/LOO/PEG PRI/PEG MKT, and PEG BENCH accepted then 321 on cancel). New transcripts: `place_order_mkt_buy_aapl.txt`, `place_order_mkt_sell_aapl.txt`, `place_order_lmt_buy_aapl.txt`, `api_order_fill_aapl.txt`, `api_delayed_success_modify_aapl.txt`, `api_order_type_matrix_aapl.txt`. |
| ORD-002 | Direct and handle cancel | `Orders().Cancel`, `OrderHandle.Cancel`, `WithManualCancelTime`, `WithCancelExternalOperator`, `WithCancelManualOrderIndicator`, official `cancelOrder` | `place_order_cancel`, `place_order_direct_cancel`, `cancel_order.txt`, `direct_cancel_order.txt` | promoted | direct cancel by ID and handle cancel both frozen; compliance metadata is source-grounded and exposed with local validation, while non-empty live attestation remains pending; cancel unknown order and terminal status remain target variants |
| ORD-003 | Modify | `OrderHandle.Modify`, official modify by re-sending `placeOrder` | `place_order_modify`, `place_order_modify.txt` | promoted | price, quantity, TIF, forbidden side/contract changes, mismatched order ID rejection |
| ORD-004 | Global cancel | `Orders().CancelAll`, `WithCancelExternalOperator`, `WithCancelManualOrderIndicator`, official `reqGlobalCancel` | `global_cancel`, `global_cancel.txt` | promoted | no orders, many orders, mixed basic/bracket/OCA/conditional, post-cancel open-orders check; non-empty compliance metadata is source-grounded but still needs live attestation |
| ORD-005 | Open orders | `Orders().Open`, `Orders().SubscribeOpen`, `Orders().RefreshOpen`, official open/all/auto-open-order scopes | `api_client_id0_order_observation_aapl`, `api_client_id0_order_observation_aapl.txt`, `api_cross_client_cancel_aapl`, `api_cross_client_cancel_aapl.txt`, `api_reconnect_active_order_aapl`, `api_reconnect_active_order_aapl.txt`, `api_reconnect_recovered_cancel_status_aapl.txt`, `api_open_orders_refresh_aapl.txt`, `open_orders_empty`, `open_orders_all`, `open_orders.txt`, `open_orders_disconnect_after_end.txt`, `open_orders_auto_refresh.txt` | promoted | own, all, client_id_0, cross-client, active-order reconnect, recovered-order status delivery (issue #20), and subscription refresh (issue #21) are live-captured/replay-promoted; the 2026-07-04 probes showed cross-client cancel-by-ID drawing 10147 outside RTH (market-hours re-probe pending) and no open_order_end for the auto bind; remaining target: auto-bind/orderBound |
| ORD-006 | Completed orders | `Orders().Completed`, official `reqCompletedOrders`, `completedOrder`, `completedOrdersEnd` | `api_completed_orders_variants_aapl`, `api_completed_orders_variants_aapl.txt`, `completed_orders`, `completed_orders.txt`, `completed_orders_cancelled_system_live.txt` | promoted | `api_completed_orders_variants_aapl` was recaptured on 2026-04-15 (`6ac84daaf4084436`) after fixing the live TRAIL LIMIT completed-order path; apiOnly=false and apiOnly=true both reached `completedOrdersEnd`. The codec parses every source-defined classic field sequentially, rejects unparsed drift, and projects the complete typed v200 order/completion result. The raw system-cancel replay freezes sentinel normalization and free-form completion metadata. Nondefault combo, scale-extension, hedge, active delta-neutral, PEG BENCH, condition, and FA completed-order branches remain live-attestation gaps. |
| ORD-007 | Executions and commissions/fees | `Orders().Executions`, official `reqExecutions`, `execDetails`, `commissionAndFeesReport` | `executions_snapshot`, `executions.txt`, `executions_correlated.txt`, `executions_overlapping.txt`, `api_order_fill_aapl`, `trading_split_round_trip_aapl` | promoted | Raw sv200 captures freeze all 33 execution fields, all 8 commission-and-fees wire fields, and the execution-end marker. Account/symbol filters and the required unset last-days/date tail are live-attested. Client, time, secType, exchange, side, finite last-days, and specific-date filters are source-grounded public API but remain live targets. Liquidation, EV/model, pending-revision, submitter, and meaningful bond yield/redemption values also remain unattested beyond defaults. |
| ORD-008 | Order handle lifecycle | `OrderHandle.Events`, `OrderHandle.Lifecycle`, `OrderHandle.Done`, `OrderHandle.Wait`, `OrderHandle.Close` | `api_reconnect_active_order_aapl`, `api_order_handle_reconnect_cancel_aapl.txt`, `api_transmit_false_then_transmit_aapl`, order replay and live tests | candidate | handle detach without cancel, terminal auto-close, active-order reconnect/open-order recovery, slow consumer |
| ORD-009 | End-to-end trading campaign | account, orders, executions, completed orders, PnL, positions | `trading_split_round_trip_aapl`, `api_algorithmic_campaign_aapl`, `api_scale_in_campaign_aapl`, `api_scale_in_campaign_aapl.txt`, `api_stress_rapid_fire_aapl`, `api_stress_rapid_fire_aapl.txt`, `api_pairs_trading_aapl_msft`, `api_pairs_trading_aapl_msft.txt`, `api_dollar_cost_averaging_aapl`, `api_dollar_cost_averaging_aapl.txt`, `api_stop_loss_management_aapl`, `api_stop_loss_management_aapl.txt` | candidate | split buys/sells, concurrent observers, cleanup, stop management (`a563cafd26e366be` live capture), rapid-fire ten-order global cancel (`69ee6be4cdf7d577` live capture), scale-in two-fill plus protective stop replay (`63db2db7cba21b68` live capture), pair-order replay (`0dc806f7bb0868e8` live capture), repeated-buy replay (`296bdf662eb84e30` live capture), final account/position/PnL/open-order reconciliation. Later scale-in and campaign execution-query reconciliation remain targets because those source tails timed out. Aggressive paper sizing now defaults to 500-share campaign clips |

## Advanced Orders

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| AORD-001 | Brackets and attached orders | `Orders().PlaceBracket`, `Orders().Place`, official bracket/attached order behavior | `place_order_bracket_aapl`, `api_bracket_trigger_aapl`, `api_bracket_trigger_aapl.txt`, `api_bracket_trailing_stop_aapl`, `api_bracket_trailing_stop_aapl.txt` | candidate | `Orders().PlaceBracket` is replay-frozen against `api_bracket_trigger_aapl.txt`: it allocates all three IDs together and sends parent/take-profit/stop-loss with false/false/true transmit sequencing. The fixture also freezes parent fill, child OCA group echo, and real price-band cancel/reject on forced take-profit modify; `api_bracket_trailing_stop_aapl.txt` freezes live code 328 for TRAIL child under a market parent. Remaining target variants: valid limit/stop-limit parent trailing child, take-profit fill cancels stop-loss sibling, stop-loss trigger, cleanup |
| AORD-002 | OCA | official OCA group/order type | `place_order_oca_pair_aapl`, `api_oca_trigger_aapl`, `api_oca_trigger_aapl.txt` | candidate | `api_oca_trigger_aapl.txt` freezes OCA group echo plus real aggressive-peer PendingCancel/Cancelled price-band rejection; remaining target variants: one fills and cancels peer, far-from-market cleanup, mixed buy/sell |
| AORD-003 | IB algos | official IB algos and TagValue params | `place_order_algo_adaptive_aapl`, `api_algorithmic_campaign_aapl`, `api_algo_variants_aapl`, `api_algo_variants_aapl.txt` | promoted | Thirteen-variant matrix replay-promoted from the 2026-04-15 capture (`1855e2554d7de3ae`): Adaptive urgent/patient, Vwap, ArrivalPx, AccumDist, ClosePx, PctVol accepted with Gateway-normalized param echoes; Twap rejected 443 unknown-attribute, DarkIce rejected 10255 display-size, Inline/BalanceImpactRisk/MinImpact/JefAD rejected 439 definition-not-found. The two echoes that rode as sanitized raw live-layout frames (a synthetic five-param algo frame used to collide with the codec's 169-field length gate) were converted to typed open_order lines once the encoder converged on the live layout and the length gate was removed. `place_order_algo_adaptive_aapl` off-hours 201 recapture (2026-06-10) remains unpromoted ancillary evidence. |
| AORD-004 | Order conditions | official price/time/margin/execution/volume/percent-change conditions | `place_order_price_condition_aapl`, `api_conditions_matrix_aapl`, `api_conditions_matrix_aapl.txt` | promoted | All six condition families replay-promoted 2026-06-10 from a post-fix live capture (`87059663ed139026`): each accepted to PreSubmitted with the off-hours code-399 deferral, then cancelled. The 2026-06-10 run also exposed and fixed the contract-bound condition field-order codec bug (code 320 evidence). Remaining variants: or-conjunction, conditionsCancelOrder=true, market-hours non-deferred acceptance. The `"None"`-sentinel partial-decode gap found here is fixed: conditioned open_order echoes decode fully, frozen by capture-decode tests on the live price and execution condition frames; `place_order_price_condition_aapl` was recaptured clean after the raw serializer fix (20260611T073844Z). |
| AORD-005 | Combo/BAG | official combo legs, combo prices, smart combo routing params | `api_combo_option_vertical_aapl` | candidate | STK combo, option vertical, ratio legs, per-leg price, execution/open/completed observation |
| AORD-006 | Hedge orders | official hedging | `api_hedge_order_aapl`, `api_hedge_order_aapl.txt` | promoted | Five live rules frozen 2026-06-11: beta and pair hedges accepted at zero size (Gateway computes the child quantity and floors the limit), delta hedges require an option parent (320) and a valid ratio (320), FX hedging requires a matching currency-pair child (10063, terminal handle error via the attested placement-rejection set), and sizing a hedge child draws 10032 (sibling-session evidence in the header). |
| AORD-007 | Delta-neutral extensions | official delta-neutral order/contract fields | none | deferred | live grounding before removing partial OpenOrder fallback |
| AORD-008 | Scale orders | official scale fields | `api_tif_attribute_matrix_aapl` | candidate | scale init/subs size, increment, table, active times, open-order decode |
| AORD-009 | Pegged and adjusted order families | official PEG BENCH, PEG BEST, PEG MID, adjusted stop/trailing fields | `api_order_type_matrix_aapl`, `api_tif_attribute_matrix_aapl` | candidate | live accepted/rejected captures for each specialized branch |
| AORD-010 | Regulatory/allocation order fields | `Orders().Preview`, official FA allocation, MiFID, manual order time, soft-dollar, advancedErrorOverride, IBKRATS | `api_whatif_margin_aapl`, `api_whatif_margin_aapl.txt`, `api_tif_attribute_matrix_aapl` | candidate | WhatIf margin preview promoted 2026-06-11 from the 2026-06-10 capture (`e8ee70b24de3fe2f`): `Orders().Preview` forces the what-if flag, sends the byte-identical place_order frame, and returns the order-state block as an OrderState (the nine margin decimals plus commission range and currency); no lifecycle follows and no handle is created (the earlier no-preview/320 attempts were the v1.4.6 default-int bug). MiFID, manual-order-time acceptance, FA allocation, and IBKRATS variants remain open. |

## Options

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| OPT-001 | Option calculations | `Options().ImpliedVolatility`, `Options().Price`, official calculate/cancel calls | `calc_implied_volatility.txt`, `calc_option_price.txt`, live tests | promoted | valid qualified option, unavailable data, invalid option, cancel after first computation |
| OPT-002 | Option exercise/lapse | `Options().Exercise`, official `exerciseOptions` | `api_option_exercise_aapl`, `api_option_exercise_not_itm_aapl.txt`, `api_option_exercise_server_reject_aapl.txt` | promoted | Both live outcomes frozen 2026-06-11: a barely-ITM call drew 322 "not in-the-money", and a deep-ITM call was accepted with the 10349 TIF-preset session event before paper clearing returned 322 and 202. Exercise replies are surfaced as session events while an exercise req-id route is active, and terminal 322/202 notices retire that route. Lapse and override variants plus a true clearing settlement remain open. |
| OPT-003 | Option order and data integration | `Orders().Place`, market data/history for OPT | `place_order_option_buy`, `api_option_campaign_aapl`, `api_combo_option_vertical_aapl` | candidate | option quote, historical ticks if available, order fill/reject, completed/execution observation |

## News, Scanner, FA, WSH, Display, And TWS

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| NEWS-001 | News providers and bulletins | `News().Providers`, `News().SubscribeBulletins`, official provider/bulletin calls | `news_providers`, `news_bulletins`, `news_providers.txt`, `news_bulletins.txt` | promoted | allMessages true/false, cancel, no entitlement |
| NEWS-002 | News article | `News().Article`, official `reqNewsArticle`, `newsArticle` | `api_news_article_aapl`, `news_article.txt` | candidate | article from captured historical-news ID, invalid article ID, provider-specific errors |
| NEWS-003 | Historical news | `News().Historical`, official historical news callbacks | `historical_news_aapl`, `historical_news_aapl_timezone_window`, `api_news_article_aapl` | promoted | see HIST-006 |
| SCAN-001 | Scanner parameters | `Scanner().Parameters`, official `reqScannerParameters`, `scannerParameters` | `scanner_parameters`, `scanner_parameters.txt` | promoted | XML decode/size, unavailable scanner service |
| SCAN-002 | Scanner subscriptions | `Scanner().SubscribeResults`, official subscription/cancel callbacks | `scanner_subscription`, `scanner_subscription.txt` | promoted | legacy fields, filter TagValues, no-result scan, cancel |
| FA-001 | FA request config | `Advisors().Config`, official `requestFA`, `receiveFA` | `request_fa` | candidate | groups, profiles, aliases, FA account, non-FA error |
| FA-002 | FA replace config | `Advisors().ReplaceConfig`, official `replaceFA`, `replaceFAEnd` | `api_fa_replace_non_fa`, `api_fa_replace_non_fa.txt` | promoted | The non-FA blocker is frozen 2026-06-11 (`81e43254856879c6`): ReplaceConfig is fire-and-forget and the Gateway answers code 321 "FA data operations ignored for non FA customers", which matches no route and is dropped, so the public surface is silence. FA-account read-back/restore and the replaceFAEnd callback stay blocked without an FA account. |
| FA-003 | Soft-dollar tiers | `Advisors().SoftDollarTiers`, official `reqSoftDollarTiers`, `softDollarTiers` | `soft_dollar_tiers`, `soft_dollar_tiers.txt` | promoted | empty and non-empty tier list |
| WSH-001 | WSH metadata | `WSH().MetaData`, official `reqWshMetaData`, `cancelWshMetaData`, `wshMetaData` | `wsh_meta_data`, `api_wsh_variants_aapl`, `api_wsh_variants_aapl.txt`, `wsh_meta_data_error.txt` | candidate | success and cancel path remain target variants; `api_wsh_variants_aapl.txt` freezes real code 10276 metadata entitlement error |
| WSH-002 | WSH event data | `WSH().EventData`, official `reqWshEventData`, `cancelWshEventData`, `wshEventData` | `wsh_event_data_aapl`, `api_wsh_variants_aapl`, `api_wsh_variants_aapl.txt` | candidate | success, cancel path, and filter JSON success remain target variants; `api_wsh_variants_aapl.txt` freezes conID, portfolio, watchlist, competitor, and date-window variants returning real code 10276 |
| TWS-001 | User info | `TWS().UserInfo`, official `reqUserInfo`, `userInfo` | `user_info`, `user_info.txt`, capture-decode 014d470efb662e72 | promoted | msg_id was mis-numbered 103 until 2026-07-04 (live sends 107); DSL replay masked it — raw-frame test now freezes the id |
| TWS-002 | Display groups | `TWS().DisplayGroups`, `TWS().SubscribeDisplayGroup`, `DisplayGroupHandle.Update`, official display group calls | `display_groups`, `display_group_subscribe`, `display_groups.txt`, `display_group_subscribe.txt` | promoted | query, subscribe, update valid/invalid contract info, unsubscribe, invalid group |

## Error, Entitlement, And Negative Behavior

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| ERR-001 | API errors and farm status | official `error` callback, system status codes | `error_api_error_oneshot.txt`, `error_api_error_subscription.txt`, `error_empty_results.txt`, `error_market_data_warning.txt`, `error_farm_status_codes.txt` | promoted | request-scoped, subscription-scoped, session-scoped, warning-only, advanced order reject JSON |
| ERR-002 | Disconnect during operations | transport and protocol disconnects | `error_disconnect_during_snapshot.txt`, `error_disconnect_during_oneshot.txt`, `quote_stream_disconnect.txt` | promoted | every one-shot and every stream family has at least one disconnect behavior row |
| ERR-003 | Entitlement and account-type failures | market data, fundamentals, WSH, FA, scanner, orders | `market_depth_error.txt`, `wsh_meta_data_error.txt`, `api_wsh_variants_aapl.txt`, entitlement candidate captures | candidate | freeze real paper-account blocked responses instead of inventing mocks; WSH metadata/event variants replay-promoted from 2026-04-15 capture `65aeb0a3b716e4b6` |

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
| `api_bracket_trailing_stop_aapl` | AORD-001 |
| `api_bracket_trigger_aapl` | AORD-001 |
| `api_bracket_trigger_aapl.txt` | AORD-001 |
| `api_client_id0_order_observation_aapl` | ORD-005 |
| `api_combo_option_vertical_aapl` | AORD-005 |
| `api_completed_orders_variants_aapl` | ORD-006 |
| `api_conditions_matrix_aapl` | AORD-004 |
| `api_cross_client_cancel_aapl` | ORD-005 |
| `api_delayed_success_modify_aapl` | ORD-001 |
| `api_dollar_cost_averaging_aapl` | ORD-009 |
| `api_duplicate_quote_subscriptions_aapl` | MD1-003 |
| `api_duplicate_quote_subscriptions_aapl.txt` | MD1-003 |
| `api_fundamental_report_errors_aapl.txt` | REF-006 |
| `api_fundamental_reports_aapl` | REF-006 |
| `api_forex_lifecycle_eurusd` | ORD-001 |
| `api_forex_lifecycle_eurusd.txt` | ORD-001 |
| `api_future_campaign_mes` | ORD-001 |
| `api_historical_matrix_aapl` | HIST-001 |
| `api_ioc_fok_aapl` | ORD-001 |
| `api_market_data_completeness_aapl` | MD1-003 |
| `api_market_data_type_cycle.txt` | MD1-001 |
| `api_news_article_aapl` | NEWS-002 |
| `api_oca_trigger_aapl` | AORD-002 |
| `api_oca_trigger_aapl.txt` | AORD-002 |
| `api_order_fill_aapl` | ORD-001 |
| `api_order_rejects_aapl` | ORD-001 |
| `api_order_relative_cancel_aapl` | ORD-001 |
| `api_order_rest_cancel_aapl` | ORD-001 |
| `api_order_stop_cancel_aapl` | ORD-001 |
| `api_order_trailing_cancel_aapl` | ORD-001 |
| `api_option_campaign_aapl` | OPT-003 |
| `api_order_type_matrix_aapl` | ORD-001 |
| `api_pairs_trading_aapl_msft` | ORD-009 |
| `api_reconnect_active_order_aapl` | SESS-005 |
| `api_realtime_bars_request_errors_aapl.txt` | MD1-005 |
| `api_scale_in_campaign_aapl` | ORD-009 |
| `api_scale_in_campaign_aapl.txt` | ORD-009 |
| `api_security_type_probe_errors.txt` | REF-001 |
| `api_security_type_probe_matrix` | REF-001 |
| `api_stop_loss_management_aapl` | ORD-009 |
| `api_stress_rapid_fire_aapl` | ORD-009 |
| `api_tif_attribute_matrix_aapl` | ORD-001 |
| `api_tick_by_tick_entitlement_errors_aapl.txt` | MD2-002 |
| `api_transmit_false_then_transmit_aapl` | ORD-001 |
| `api_whatif_margin_aapl` | AORD-010 |
| `api_wsh_variants_aapl` | WSH-001 |
| `api_wsh_variants_aapl` | WSH-002 |
| `api_wsh_variants_aapl.txt` | WSH-001 |
| `api_wsh_variants_aapl.txt` | WSH-002 |
| `bootstrap` | SESS-001 |
| `bootstrap_client_id_0` | SESS-001 |
| `completed_orders` | ORD-006 |
| `contract_details_aapl_opt` | REF-001 |
| `contract_details_aapl_stk` | REF-001 |
| `contract_details_es_fut` | REF-001 |
| `contract_details_eurusd_cash` | REF-001 |
| `contract_details_not_found` | REF-001 |
| `current_time` | SESS-002 |
| `display_group_subscribe` | TWS-002 |
| `display_groups` | TWS-002 |
| `executions_snapshot` | ORD-007 |
| `family_codes` | ACCT-007 |
| `fundamental_data_aapl` | REF-006 |
| `global_cancel` | ORD-004 |
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
| `historical_ticks_aapl_timezone_window` | HIST-005 |
| `historical_ticks_aapl_trades` | HIST-005 |
| `market_depth_aapl` | MD2-001 |
| `market_depth_aapl_smart` | MD2-001 |
| `market_rule` | REF-004 |
| `matching_symbols_aapl` | REF-002 |
| `matching_symbols_partial` | REF-002 |
| `mkt_depth_exchanges` | REF-005 |
| `news_bulletins` | NEWS-001 |
| `news_providers` | NEWS-001 |
| `open_orders_all` | ORD-005 |
| `open_orders_empty` | ORD-005 |
| `place_order_algo_adaptive_aapl` | AORD-003 |
| `place_order_bracket_aapl` | AORD-001 |
| `place_order_cancel` | ORD-002 |
| `place_order_direct_cancel` | ORD-002 |
| `place_order_lmt_buy_aapl` | ORD-001 |
| `place_order_mkt_buy_aapl` | ORD-001 |
| `place_order_mkt_sell_aapl` | ORD-001 |
| `place_order_modify` | ORD-003 |
| `place_order_oca_pair_aapl` | AORD-002 |
| `place_order_option_buy` | OPT-003 |
| `place_order_price_condition_aapl` | AORD-004 |
| `pnl` | ACCT-006 |
| `pnl_single` | ACCT-006 |
| `positions_multi` | ACCT-005 |
| `positions_snapshot` | ACCT-004 |
| `qualify_contract_aapl_exact` | REF-001 |
| `qualify_contract_ambiguous` | REF-001 |
| `quote_snapshot_aapl` | MD1-002 |
| `quote_stream_aapl` | MD1-003 |
| `quote_stream_genericticks` | MD1-003 |
| `quote_stream_multi_asset` | MD1-003 |
| `quote_with_generic_ticks` | MD1-003 |
| `realtime_bars_aapl` | MD1-005 |
| `req_ids` | SESS-003 |
| `request_fa` | FA-001 |
| `scanner_parameters` | SCAN-001 |
| `scanner_subscription` | SCAN-002 |
| `sec_def_opt_params_aapl` | REF-003 |
| `set_type_delayed` | MD1-001 |
| `set_type_delayed_frozen` | MD1-001 |
| `set_type_frozen` | MD1-001 |
| `set_type_invalid` | MD1-001 |
| `set_type_live` | MD1-001 |
| `set_type_switch_while_streaming` | MD1-001 |
| `smart_components` | REF-004 |
| `soft_dollar_tiers` | FA-003 |
| `tick_by_tick_bidask` | MD2-002 |
| `tick_by_tick_last` | MD2-002 |
| `tick_by_tick_midpoint` | MD2-002 |
| `trading_split_round_trip_aapl` | ORD-009 |
| `user_info` | TWS-001 |
| `wsh_event_data_aapl` | WSH-002 |
| `wsh_meta_data` | WSH-001 |

## Immediate Target Scenario Gaps

These gaps block any claim that the matrix is fully executable:

- `MarketData().SetType`: bare data types 1, 2, 3, and 4 are replay-promoted;
  add stream callback replays, invalid type, and switch-while-streaming
  behavior.
- `Orders().Cancel`: add direct cancel-by-ID live scenario distinct from
  `OrderHandle.Cancel`.
- `Options().Exercise`: add exercise and lapse paper scenarios plus invalid
  option/account response.
- `News().Article`: add article lookup using an article ID from a live
  historical-news capture.
- `Advisors().ReplaceConfig`: add non-FA error capture and FA read-back/restore
  scenario if an FA account is ever available.
- Official callback gaps from `ibkr-api-inventory.md`: `tickEFP`, `tickNews`,
  `orderBound`, `replaceFAEnd`, reroute/verify callbacks, and bond contract
  details.

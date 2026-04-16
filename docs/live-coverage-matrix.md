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
| SESS-002 | Session observation and current time | `Client.CurrentTime`, official `reqCurrentTime`, `currentTime`, session events | `current_time`, `bootstrap_with_farm_status.txt`, `error_farm_status_codes.txt`, `current_time.txt` | candidate | explicit `reqCurrentTime` request with parsed epoch time, farm-status no-state-change assertions, unavailable current-time behavior |
| SESS-003 | ID allocation and managed-account refresh | official `reqIds`, `reqManagedAccts`, `nextValidId`, `managedAccounts` | `req_ids`, `req_ids.txt`, bootstrap fixtures | candidate | explicit `reqIds` request grounded (numIds=1 → NEXT_VALID_ID msg 9); managed-account refresh request and repeated ID allocation after order placement remain target variants |
| SESS-004 | Server control and old auth/redirect hooks | official `setServerLogLevel`, redirect, verify/auth, connectAck, reroute callbacks | none | deferred | classify live behavior as out_of_scope or target; freeze any Gateway callback if observed |
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
| REF-001 | Contract details and qualification | `Contracts().Details`, `Contracts().Qualify`, official `reqContractDetails`, `contractDetails`, `bondContractDetails`, `contractDetailsEnd` | `contract_details_aapl_stk`, `contract_details_aapl_opt`, `contract_details_eurusd_cash`, `contract_details_es_fut`, `contract_details_not_found`, `qualify_contract_aapl_exact`, `qualify_contract_ambiguous`, `api_security_type_probe_matrix`, `contract_details.txt`, `grounded_contract_details_aapl.txt` | candidate | STK, OPT, FUT, FOP, CASH, BOND, FUND, IND, BAG; exact, ambiguous, invalid, expired/includeExpired. 2026-04-15 probe captured real details/errors for STK/OPT/FUT/FOP/CASH/BOND/CFD/WAR/IND/CRYPTO/FUND/BILL/CMDTY/CONTFUT |
| REF-002 | Matching symbols | `Contracts().Search`, official `reqMatchingSymbols`, `symbolSamples` | `matching_symbols_aapl`, `matching_symbols_partial`, `matching_symbols.txt` | promoted | broad pattern, exact-ish pattern, derivative sec types, description/issuer fields |
| REF-003 | Option chain metadata | `Contracts().SecDefOptParams`, official `reqSecDefOptParams`, `securityDefinitionOptionParameter`, `securityDefinitionOptionParameterEnd` | `sec_def_opt_params_aapl`, `sec_def_opt_params.txt` | promoted | STK underlyings, FUT/FOP underlyings, empty exchange, invalid underlying |
| REF-004 | Market rules and smart components | `Contracts().MarketRule`, `Contracts().SmartComponents`, official `reqMarketRule`, `reqSmartComponents`, `marketRule`, `smartComponents` | `market_rule`, `smart_components`, `market_rule.txt`, `smart_components.txt` | promoted | US equity, option, future, invalid market rule, invalid BBO exchange |
| REF-005 | Market-depth exchanges | `Contracts().DepthExchanges`, official `reqMktDepthExchanges`, `mktDepthExchanges` | `mkt_depth_exchanges`, `mkt_depth_exchanges.txt` | promoted | all returned service data types, SMART support, invalid routing implication |
| REF-006 | Fundamental data | `Contracts().FundamentalData`, official `reqFundamentalData`, `cancelFundamentalData`, `fundamentalData` | `fundamental_data_aapl`, `api_fundamental_reports_aapl`, `fundamental_data.txt` | candidate | all `FundamentalReportType` values, entitlement error, invalid report type, cancel path |

## Market Data L1

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| MD1-001 | Market data type control | `MarketData().SetType`, official `reqMarketDataType`, `marketDataType` | `set_type_live`, `set_type_frozen`, `set_type_delayed`, `set_type_delayed_frozen`, `set_type_invalid`, `set_type_switch_while_streaming`, `api_market_data_completeness_aapl`, `quote_delayed_data.txt`, `lifecycle_set_mdt_after_close.txt` | candidate | live evidence: bare SetType(1/2/3/4) is accepted silently — no marketDataType push arrives without an active quote stream. Invalid value 99 is also accepted silently. Switching mid-stream triggers a real entitlement error 10089 for live data on a paper account. Transcript promotion pending per-type replay tests tied to quote streams that exercise the push |
| MD1-002 | Quote snapshots | `MarketData().Quote`, official `reqMktData`, `cancelMktData`, `tickSnapshotEnd` | `quote_snapshot_aapl`, `api_market_data_completeness_aapl`, `quote_snapshot.txt`, `quote_delayed_data.txt` | promoted | STK/OPT/FUT/CASH snapshots, entitlement error, no data, regulatory snapshot where applicable |
| MD1-003 | Quote streams and generic ticks | `MarketData().SubscribeQuotes`, official tick callbacks | `quote_stream_aapl`, `quote_stream_genericticks`, `quote_with_generic_ticks`, `quote_stream_multi_asset`, `api_market_data_completeness_aapl`, `api_duplicate_quote_subscriptions_aapl`, `quote_with_generic_ticks.txt` | candidate | price/size/string/generic/option/EFP/news/dividend/shortable/RTVolume/fundamental-ratio generic tick families; duplicate same-contract subscriptions captured 2026-04-15 (`84f1e78a18616e0f`) |
| MD1-004 | Tick callback edge shapes | official `tickPrice`, `tickSize`, `tickString`, `tickGeneric`, `tickEFP`, `tickOptionComputation`, `tickNews`, `tickReqParams` | `calc_implied_volatility.txt`, `calc_option_price.txt`, quote fixtures | target | tickEFP, tickNews, tickReqParams, option computation live success/error |
| MD1-005 | Real-time bars | `MarketData().SubscribeRealTimeBars`, official `reqRealTimeBars`, `cancelRealTimeBars`, `realtimeBar` | `realtime_bars_aapl`, `api_market_data_completeness_aapl`, `realtime_bars_reconnect.txt` | promoted | TRADES/MIDPOINT/BID_ASK, RTH true/false, cancel, reconnect |

## Market Data L2 And Tick-By-Tick

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| MD2-001 | Market depth regular and smart | `MarketData().SubscribeDepth`, official `reqMktDepth`, `cancelMktDepth`, `updateMktDepth`, `updateMktDepthL2` | `market_depth_aapl`, `market_depth_aapl_smart`, `market_depth_error.txt` | candidate | L1 vs L2 rows, SMART depth, insert/update/delete, market maker names, entitlement errors, cancel |
| MD2-002 | Tick-by-tick streams | `MarketData().SubscribeTickByTick`, official `reqTickByTickData`, `cancelTickByTickData`, tick-by-tick callbacks | `tick_by_tick_last`, `tick_by_tick_bidask`, `tick_by_tick_midpoint`, `api_market_data_completeness_aapl`, `tick_by_tick.txt` | candidate | Last, AllLast, BidAsk, MidPoint, ignoreSize true/false, numberOfTicks, pacing, unavailable data |

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
| ORD-001 | Basic order placement | `Orders().Place`, official `placeOrder`, `openOrder`, `orderStatus` | `place_order_lmt_buy_aapl`, `place_order_mkt_buy_aapl`, `place_order_mkt_sell_aapl`, `api_order_fill_aapl`, `api_order_rest_cancel_aapl`, `api_order_relative_cancel_aapl`, `api_order_stop_cancel_aapl`, `api_order_trailing_cancel_aapl`, `api_order_rejects_aapl`, `api_order_type_matrix_aapl`, `api_tif_attribute_matrix_aapl`, `api_transmit_false_then_transmit_aapl`, `api_transmit_false_then_transmit_aapl.txt`, `api_delayed_success_modify_aapl`, `api_future_campaign_mes`, `api_future_campaign_mes.txt`, `api_forex_lifecycle_eurusd`, `api_ioc_fok_aapl`, `api_ioc_fok_aapl.txt`, `api_tif_attribute_matrix_aapl.txt`, `place_order_limit.txt`, `place_order_fill_with_execution.txt`, `place_order_modify_to_market_late_execution.txt`, `place_order_invalid_type_live_error.txt` | candidate | MKT, LMT, STP, STP LMT, TRAIL, TRAIL LIMIT, MIT, LIT, MTL, REL, MOC, LOC, MOO, LOO, PEG families, DAY/GTC/IOC/FOK/GTD, marketable/far-from-market, staged Transmit=false then transmit |
| ORD-002 | Direct and handle cancel | `Orders().Cancel`, `OrderHandle.Cancel`, official `cancelOrder` | `place_order_cancel`, `place_order_direct_cancel`, `cancel_order.txt`, `direct_cancel_order.txt` | promoted | direct cancel by ID and handle cancel both frozen; cancel unknown order, manual cancel time, terminal status remain target variants |
| ORD-003 | Modify | `OrderHandle.Modify`, official modify by re-sending `placeOrder` | `place_order_modify`, `place_order_modify.txt` | promoted | price, quantity, TIF, forbidden side/contract changes, mismatched order ID rejection |
| ORD-004 | Global cancel | `Orders().CancelAll`, official `reqGlobalCancel` | `global_cancel`, `global_cancel.txt` | promoted | no orders, many orders, mixed basic/bracket/OCA/conditional, post-cancel open-orders check |
| ORD-005 | Open orders | `Orders().Open`, `Orders().SubscribeOpen`, official open/all/auto-open-order scopes | `api_client_id0_order_observation_aapl`, `api_client_id0_order_observation_aapl.txt`, `api_cross_client_cancel_aapl`, `api_cross_client_cancel_aapl.txt`, `api_reconnect_active_order_aapl`, `api_reconnect_active_order_aapl.txt`, `open_orders_empty`, `open_orders_all`, `open_orders.txt`, `open_orders_disconnect_after_end.txt` | promoted | own, all, client_id_0, cross-client, and active-order reconnect are live-captured/replay-promoted; remaining target: auto-bind/orderBound |
| ORD-006 | Completed orders | `Orders().Completed`, official `reqCompletedOrders`, `completedOrder`, `completedOrdersEnd` | `api_completed_orders_variants_aapl`, `api_completed_orders_variants_aapl.txt`, `completed_orders`, `completed_orders.txt` | promoted | `api_completed_orders_variants_aapl` was recaptured on 2026-04-15 (`6ac84daaf4084436`) after fixing the live TRAIL LIMIT completed-order decode path; apiOnly=false and apiOnly=true both reached `completedOrdersEnd` |
| ORD-007 | Executions and commissions | `Orders().Executions`, official `reqExecutions`, `execDetails`, `commissionReport` | `executions_snapshot`, `executions.txt`, `executions_correlated.txt`, `executions_overlapping.txt`, `trading_split_round_trip_aapl` | promoted | filters by account/client/symbol/secType/exchange/side/time, commission before/after exec, sentinel values. 2026-04-15 aggressive pairs live run exposed and fixed the server_version=200 `lastNDays`/`specificDates` request fields |
| ORD-008 | Order handle lifecycle | `OrderHandle.Events`, `OrderHandle.Lifecycle`, `OrderHandle.Done`, `OrderHandle.Wait`, `OrderHandle.Close` | `api_reconnect_active_order_aapl`, `api_order_handle_reconnect_cancel_aapl.txt`, `api_transmit_false_then_transmit_aapl`, order replay and live tests | candidate | handle detach without cancel, terminal auto-close, active-order reconnect/open-order recovery, slow consumer |
| ORD-009 | End-to-end trading campaign | account, orders, executions, completed orders, PnL, positions | `trading_split_round_trip_aapl`, `api_algorithmic_campaign_aapl`, `api_scale_in_campaign_aapl`, `api_stress_rapid_fire_aapl`, `api_pairs_trading_aapl_msft`, `api_pairs_trading_aapl_msft.txt`, `api_dollar_cost_averaging_aapl`, `api_dollar_cost_averaging_aapl.txt`, `api_stop_loss_management_aapl`, `api_stop_loss_management_aapl.txt` | candidate | split buys/sells, concurrent observers, cleanup, stop management (`a563cafd26e366be` live capture), pair orders, repeated buys, final account/position/PnL/open-order reconciliation. Aggressive paper sizing now defaults to 500-share campaign clips |

## Advanced Orders

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| AORD-001 | Brackets and attached orders | `Orders().Place`, official bracket/attached order behavior | `place_order_bracket_aapl`, `api_bracket_trigger_aapl`, `api_bracket_trailing_stop_aapl` | candidate | parent transmit false, take-profit, stop-loss, trailing stop-loss, activation after parent, cleanup |
| AORD-002 | OCA | official OCA group/order type | `place_order_oca_pair_aapl`, `api_oca_trigger_aapl` | candidate | one fills and cancels peer, far-from-market cleanup, mixed buy/sell |
| AORD-003 | IB algos | official IB algos and TagValue params | `place_order_algo_adaptive_aapl`, `api_algorithmic_campaign_aapl`, `api_algo_variants_aapl` | candidate | Adaptive normal/urgent/patient, TWAP, VWAP, ArrivalPx, DarkIce, AccumDist, Inline, Close, PctVol, BalanceImpactRisk, MinImpact, invalid param, open-order round trip. `api_algo_variants_aapl` captured 2026-04-15 (`1855e2554d7de3ae`) |
| AORD-004 | Order conditions | official price/time/margin/execution/volume/percent-change conditions | `place_order_price_condition_aapl`, `api_conditions_matrix_aapl` | candidate | every condition family, and/or conjunction, ignoreRTH, cancelOrder |
| AORD-005 | Combo/BAG | official combo legs, combo prices, smart combo routing params | `api_combo_option_vertical_aapl` | candidate | STK combo, option vertical, ratio legs, per-leg price, execution/open/completed observation |
| AORD-006 | Hedge orders | official hedging | none | target | delta, beta, FX hedge, pair hedge where supported, invalid hedge |
| AORD-007 | Delta-neutral extensions | official delta-neutral order/contract fields | none | deferred | live grounding before removing partial OpenOrder fallback |
| AORD-008 | Scale orders | official scale fields | `api_tif_attribute_matrix_aapl` | candidate | scale init/subs size, increment, table, active times, open-order decode |
| AORD-009 | Pegged and adjusted order families | official PEG BENCH, PEG BEST, PEG MID, adjusted stop/trailing fields | `api_order_type_matrix_aapl`, `api_tif_attribute_matrix_aapl` | candidate | live accepted/rejected captures for each specialized branch |
| AORD-010 | Regulatory/allocation order fields | official FA allocation, MiFID, manual order time, soft-dollar, advancedErrorOverride, IBKRATS | `api_whatif_margin_aapl`, `api_tif_attribute_matrix_aapl` | candidate | accepted and rejected variants; completed/open order detail |

## Options

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| OPT-001 | Option calculations | `Options().ImpliedVolatility`, `Options().Price`, official calculate/cancel calls | `calc_implied_volatility.txt`, `calc_option_price.txt`, live tests | promoted | valid qualified option, unavailable data, invalid option, cancel after first computation |
| OPT-002 | Option exercise/lapse | `Options().Exercise`, official `exerciseOptions` | none | target | exercise, lapse, override true/false, invalid option/account, paper account behavior |
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
| FA-002 | FA replace config | `Advisors().ReplaceConfig`, official `replaceFA`, `replaceFAEnd` | none | target | non-FA error, read-back/restore if FA account exists, replaceFAEnd callback |
| FA-003 | Soft-dollar tiers | `Advisors().SoftDollarTiers`, official `reqSoftDollarTiers`, `softDollarTiers` | `soft_dollar_tiers`, `soft_dollar_tiers.txt` | promoted | empty and non-empty tier list |
| WSH-001 | WSH metadata | `WSH().MetaData`, official `reqWshMetaData`, `cancelWshMetaData`, `wshMetaData` | `wsh_meta_data`, `api_wsh_variants_aapl`, `wsh_meta_data_error.txt` | candidate | success, entitlement error, cancel path |
| WSH-002 | WSH event data | `WSH().EventData`, official `reqWshEventData`, `cancelWshEventData`, `wshEventData` | `wsh_event_data_aapl`, `api_wsh_variants_aapl` | candidate | conID, filter JSON, watchlist, portfolio, competitors, date windows, entitlement error |
| TWS-001 | User info | `TWS().UserInfo`, official `reqUserInfo`, `userInfo` | `user_info`, `user_info.txt` | promoted | TWS vs Gateway differences |
| TWS-002 | Display groups | `TWS().DisplayGroups`, `TWS().SubscribeDisplayGroup`, `DisplayGroupHandle.Update`, official display group calls | `display_groups`, `display_group_subscribe`, `display_groups.txt`, `display_group_subscribe.txt` | promoted | query, subscribe, update valid/invalid contract info, unsubscribe, invalid group |

## Error, Entitlement, And Negative Behavior

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| ERR-001 | API errors and farm status | official `error` callback, system status codes | `error_api_error_oneshot.txt`, `error_api_error_subscription.txt`, `error_empty_results.txt`, `error_market_data_warning.txt`, `error_farm_status_codes.txt` | promoted | request-scoped, subscription-scoped, session-scoped, warning-only, advanced order reject JSON |
| ERR-002 | Disconnect during operations | transport and protocol disconnects | `error_disconnect_during_snapshot.txt`, `error_disconnect_during_oneshot.txt`, `quote_stream_disconnect.txt` | promoted | every one-shot and every stream family has at least one disconnect behavior row |
| ERR-003 | Entitlement and account-type failures | market data, fundamentals, WSH, FA, scanner, orders | `market_depth_error.txt`, `wsh_meta_data_error.txt`, entitlement candidate captures | candidate | freeze real paper-account blocked responses instead of inventing mocks |

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
| `api_client_id0_order_observation_aapl` | ORD-005 |
| `api_combo_option_vertical_aapl` | AORD-005 |
| `api_completed_orders_variants_aapl` | ORD-006 |
| `api_conditions_matrix_aapl` | AORD-004 |
| `api_cross_client_cancel_aapl` | ORD-005 |
| `api_delayed_success_modify_aapl` | ORD-001 |
| `api_dollar_cost_averaging_aapl` | ORD-009 |
| `api_duplicate_quote_subscriptions_aapl` | MD1-003 |
| `api_fundamental_reports_aapl` | REF-006 |
| `api_forex_lifecycle_eurusd` | ORD-001 |
| `api_future_campaign_mes` | ORD-001 |
| `api_historical_matrix_aapl` | HIST-001 |
| `api_ioc_fok_aapl` | ORD-001 |
| `api_market_data_completeness_aapl` | MD1-003 |
| `api_news_article_aapl` | NEWS-002 |
| `api_oca_trigger_aapl` | AORD-002 |
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
| `api_scale_in_campaign_aapl` | ORD-009 |
| `api_security_type_probe_matrix` | REF-001 |
| `api_stop_loss_management_aapl` | ORD-009 |
| `api_stress_rapid_fire_aapl` | ORD-009 |
| `api_tif_attribute_matrix_aapl` | ORD-001 |
| `api_transmit_false_then_transmit_aapl` | ORD-001 |
| `api_whatif_margin_aapl` | AORD-010 |
| `api_wsh_variants_aapl` | WSH-002 |
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

- `MarketData().SetType`: add separate live scenarios for data types 1, 2, 3,
  4, invalid type, and switch-while-streaming behavior.
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

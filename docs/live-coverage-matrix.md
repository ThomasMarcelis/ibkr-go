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
| SESS-002 | Session observation and current time | `Client.CurrentTime`, `Client.CurrentTimeMillis`, official `reqCurrentTime`, `reqCurrentTimeInMillis`, `currentTime`, `currentTimeInMillis`, session events | `current_time`, `current_time_millis`, `bootstrap_with_farm_status.txt`, `error_farm_status_codes.txt`, `current_time_live.txt`, `current_time_millis.txt` | promoted | Explicit second and millisecond requests/replies are live-frozen from readonly Gateway captures (`efe321755946a395` and `b3ca14922df144e09de0bd58a81bee132c0759d0acf516481cf59b455c2fc54e`). Farm-status callbacks leave the session snapshot unchanged. Unavailable-current-time behavior has no live evidence and stays a watch item. |
| SESS-003 | ID allocation and managed-account refresh | `Orders().RefreshOrderID`, official `reqIds`, `reqManagedAccts`, `nextValidId`, `managedAccounts` | `req_ids`, `req_ids.txt`, `req_ids_read_only.txt`, bootstrap fixtures | promoted | `Orders().RefreshOrderID` is grounded both ways: the paper Gateway answers NEXT_VALID_ID (`req_ids.txt`), while the read-only live Gateway rejects with req_id=-1/code 321 (`req_ids_read_only.txt`, capture hash `00b11cbce4cefc31`). The successful ID updates the actor-owned allocation seed; the returned value remains engine-owned. Managed-account refresh and repeated allocation after orders remain target variants. |
| SESS-004 | Server control and old auth/redirect hooks | official `setServerLogLevel`, redirect, verify/auth, connectAck | none | out_of_scope | Classified 2026-06-11: no verify/auth, redirect, or connectAck callback was observed across the campaign's capture corpus, and the official client documents the verify/auth family as internal. `setServerLogLevel` controls server-side TWS log verbosity, an operator concern owned by the Gateway UI rather than a client library; callers control their own logging through the configured logger. Market-data reroutes are tracked under MD1/MD2 and are no longer part of this row. |
| SESS-005 | Reconnect and interruption | reconnect policy, transport loss, API 1100/1101/1102 | `api_reconnect_active_order_aapl`, `api_reconnect_active_order_aapl.txt`, `api_order_handle_reconnect_cancel_aapl.txt`, `reconnect_policy_off.txt`, `reconnect_oneshot_interrupted.txt`, `reconnect_1100_then_transport_loss.txt`, `reconnect_1102_resume.txt`, `reconnect_gap_no_resume.txt`, `reconnect_multi_cycle.txt`, `quote_stream_gap_1101.txt`, `quote_stream_gap_1102.txt`, `quote_stream_reconnect.txt`, `quote_stream_disconnect.txt` | promoted | Active GTC order reconnect and original order-handle Gap/Resumed/cancel are replay-promoted. Active account streams, real-time bars, and historical keep-up bars remain open. |
| SESS-006 | Lifecycle edge contracts | subscription close, context cancel, singleton limits, slow consumer, concurrent one-shots | `lifecycle_subscription_close_immediate.txt`, `lifecycle_singleton_reject.txt`, `lifecycle_context_cancel.txt`, `lifecycle_concurrent_oneshots.txt`, `lifecycle_bootstrap_reordered.txt`, `lifecycle_bootstrap_no_valid_id.txt`, `lifecycle_bootstrap_no_accounts.txt`, `lifecycle_account_summary_limit.txt`, `lifecycle_set_mdt_after_close.txt` | promoted | add live-derived versions for singleton/account limits where possible |

## Accounts, Positions, Portfolio, And PnL

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| ACCT-001 | Account summary | `Accounts().Summary`, `Accounts().SubscribeSummary`, official `reqAccountSummary`, `cancelAccountSummary`, `accountSummary`, `accountSummaryEnd` | `account_summary_snapshot`, `account_summary_stream`, `account_summary.txt`, `account_summary_stream.txt`, `account_summary_disconnect_after_end.txt`, `grounded_account_summary.txt` | promoted | `All` versus concrete account, full tag set, two concurrent live subscriptions, and cancel-before-end remain target variants. |
| ACCT-002 | Account updates and portfolio | `Accounts().Updates`, `Accounts().SubscribeUpdates`, official `reqAccountUpdates`, `updateAccountValue`, `updatePortfolio`, `updateAccountTime`, `accountDownloadEnd` | `account_updates`, `account_updates.txt` | promoted | baseline, during marketable trades, multiple asset positions, unsubscribe, one-account official timing limitation |
| ACCT-003 | Account updates multi | `Accounts().UpdatesMulti`, `Accounts().SubscribeUpdatesMulti`, official `reqAccountUpdatesMulti`, `cancelAccountUpdatesMulti`, `accountUpdateMulti`, `accountUpdateMultiEnd` | `account_updates_multi`, `account_updates_multi.txt` | promoted | account/model combinations, empty model, cancel mid-stream, post-trade deltas |
| ACCT-004 | Positions | `Accounts().Positions`, `Accounts().SubscribePositions`, official `reqPositions`, `cancelPositions`, `position`, `positionEnd` | `positions_snapshot`, `positions.txt`, `positions_disconnect_after_end.txt`, `grounded_positions.txt` | promoted | empty and non-empty accounts, STK/OPT/FUT/CASH positions, streaming during trades |
| ACCT-005 | Positions multi | `Accounts().PositionsMulti`, `Accounts().SubscribePositionsMulti`, official `reqPositionsMulti`, `cancelPositionsMulti`, `positionMulti`, `positionMultiEnd` | `positions_multi`, `positions_multi.txt` | promoted | The replay freezes the exact empty snapshot observed on paper Gateway server version 200; non-empty account/model variants and streaming during trades remain open. |
| ACCT-006 | PnL account and single-position streams | `Accounts().SubscribePnL`, `Accounts().SubscribePnLSingle`, official `reqPnL`, `cancelPnL`, `reqPnLSingle`, `cancelPnLSingle`, `pnl`, `pnlSingle` | `pnl`, `pnl.txt` | candidate | The account PnL catalog scenario is public-API-only and exact sv206 evidence freezes a typed nonzero update, cancel, wait, and current-time fence (`eeba9a0ab7c6e1e2`). The replay separately preserves a real nonzero campaign update. PnLSingle, model-code variants, invalid conID, and before/during/after-trade transitions need grounded public replays. |
| ACCT-007 | Family codes | `Accounts().FamilyCodes`, official `reqFamilyCodes`, `familyCodes` | `family_codes`, `family_codes.txt` | promoted | The exact non-family response is `AccountID="*"` with an empty family code. Named single-family and multi-family accounts remain open. |

## Contracts And Reference Data

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| REF-001 | Contract details and qualification | `Contracts().Details`, `Contracts().Qualify`, official `reqContractDetails`, `contractDetails`, `bondContractDetails`, `contractDetailsEnd` | `contract_details_aapl_stk`, `contract_details_aapl_opt`, `contract_details_apple_bonds`, `contract_details_eurusd_cash`, `contract_details_es_fut`, `contract_details_not_found`, `qualify_contract_aapl_exact`, `qualify_contract_ambiguous`, `api_security_type_probe_matrix`, `api_security_type_probe_errors.txt`, `contract_details.txt`, `grounded_contract_details_aapl.txt`, `contract_details_aapl_opt.txt`, `contract_details_apple_bonds.txt`, `contract_details_sv205_live.txt`, `contract_details_eurusd_cash.txt`, `contract_details_es_fut.txt`, `contract_details_not_found.txt`, `qualify_contract_ambiguous.txt` | promoted | Regular classic v200 details are completely decoded and projected. Exact sv205 protobuf stock, bond, fund, option, issuer, ineligibility, and end frames are official-SDK/live-attested; the sv206 option replay now freezes one complete bounded request with its same-request end marker. Exact-200/206 IncludeExpired and SecurityID byte evidence freezes requests only; the public readonly matrix separately resolved AAPL by ISIN and expired MES 202606 at 200/205/206. Classic requests support IncludeExpired/SecurityID/IssuerID and fail closed on unrepresented composition; sv205 uses the complete shared Contract. Remaining positive response variants are BAG composition, description, and delta-neutral. |
| REF-002 | Matching symbols | `Contracts().Search`, official `reqMatchingSymbols`, `symbolSamples` | `matching_symbols_aapl`, `matching_symbols_partial`, `matching_symbols.txt`, `matching_symbols_partial.txt` | promoted | broad pattern, exact-ish pattern, derivative sec types, description/issuer fields; the 97-row partial-pattern reply with BOND issuer ids is frozen from the 2026-06-11 readonly-live capture (`00fdf812de6ae616`) |
| REF-003 | Option chain metadata | `Contracts().SecDefOptParams`, official `reqSecDefOptParams`, `securityDefinitionOptionParameter`, `securityDefinitionOptionParameterEnd` | `sec_def_opt_params_aapl`, `sec_def_opt_params.txt` | promoted | STK underlyings, FUT/FOP underlyings, empty exchange, invalid underlying |
| REF-004 | Market rules and smart components | `Contracts().MarketRule`, `Contracts().SmartComponents`, official `reqMarketRule`, `reqSmartComponents`, `marketRule`, `smartComponents` | `market_rule`, `smart_components`, `market_rule.txt`, `smart_components.txt` | candidate | Market rule 26 is frozen exactly as one `{low edge 0, increment 0.01}` tier. The sv206 replay derives AAPL's `9c0001` mapping identifier from TickReqParams on the active quote connection and freezes all 20 returned components. Additional equity, option, future, invalid-rule, and invalid-BBO variants remain open. |
| REF-005 | Market-depth exchanges | `Contracts().DepthExchanges`, official `reqMktDepthExchanges`, `mktDepthExchanges` | `mkt_depth_exchanges`, `mkt_depth_exchanges.txt` | promoted | all returned service data types, SMART support, invalid routing implication |

## Market Data L1

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| MD1-001 | Market data type control | `MarketData().SetType`, official `reqMarketDataType`, `marketDataType` | `set_type_live`, `set_type_frozen`, `set_type_delayed`, `set_type_delayed_frozen`, `set_type_switch_while_streaming`, `api_market_data_completeness_aapl`, `api_market_data_type_cycle.txt`, `quote_delayed_data.txt`, `lifecycle_set_mdt_after_close.txt`, `set_type_switch_while_streaming.txt` | promoted | Bare SetType(1/2/3/4) accepted silently (2026-04-15 capture `f692fc168a53da9d`); the 2026-06-11 readonly-live capture proves the stream-tied behavior: marketDataType(3) arrives before delayed ticks, mid-stream SetType(Live) is accepted without a type-1 acknowledgement, and 10167 surfaces as a session event (`e244cae7f5eb7d57`). The catalog now exercises that invariant through the public API and requires typed delayed-type plus price/size evidence before a fenced switch and cancellation; exact sv206 revalidation is `6040ff478db4d277`. The inconclusive raw invalid-type scenario was deleted; public validation already rejects values outside 1..4 before the wire. Real-time (type 1) success pushes still need a market-hours entitled stream. |
| MD1-002 | Quote snapshots | `MarketData().Quote`, official `reqMktData`, `cancelMktData`, `tickSnapshotEnd` | `quote_snapshot_aapl`, `api_market_data_completeness_aapl`, `quote_snapshot.txt`, `quote_delayed_data.txt` | promoted | The catalog scenario now runs only through the public API and was live-verified at server_version 206 with all twelve typed quote fields plus the snapshot-end boundary (`a854ac41c4e0073f`). STK/OPT/FUT/CASH snapshots, entitlement error, no data, and regulatory snapshots remain broader matrix targets. |
| MD1-003 | Quote streams and generic ticks | `MarketData().SubscribeQuotes`, official tick callbacks and `rerouteMktDataReq` | `quote_stream_aapl`, `quote_stream_genericticks`, `quote_stream_multi_asset`, `api_market_data_completeness_aapl`, `api_generic_tick_matrix_aapl`, `api_tick_news_aapl_probe`, `tick_news_aapl_sv201_live.txt`, `market_data_sv206_live.txt`, `api_duplicate_quote_subscriptions_aapl`, `api_duplicate_quote_subscriptions_aapl.txt` | candidate | The public `QuoteUpdate` union preserves every price/size callback with its numeric tick type, price-attribute mask, and optional companion size. The ordinary and 233/236 generic catalog streams are public-API-only and live-verified at server_version 206 with typed evidence plus cancel/wait/current-time fences (`6c51009bdfe158b8`, `327d7fbdeeb9979d`). The multi-asset scenario now requires concurrent typed price/size evidence from both AAPL and EUR.USD before fenced cancellation (`aeb6123029b488bc`); its former raw implementation falsely passed when generic tick 258 terminated only the AAPL leg with code 10358. Broad generic-tick evidence, including 258, remains solely owned by `api_generic_tick_matrix_aapl`. Exact sv206 protobuf requests/callbacks and presence-aware precision are replay-promoted from captures `eea31798e7e59830f` and `989563f9c4cad108e3`; classic CFD reroutes are live-attested by `7475841869bc53ce`. Classic quote composition is limited to four-field BAG legs plus delta-neutral; nondefault leg position/short-sale fields fail closed, while sv206 accepts the complete shared Contract. TickEFP, additional generic families, and entitled live-price streams remain gaps; duplicate same-contract subscriptions are replay-promoted from capture `84f1e78a18616e0f`. |
| MD1-004 | Tick callback edge shapes | official `tickPrice`, `tickSize`, `tickString`, `tickGeneric`, `tickEFP`, `tickOptionComputation`, `tickNews`, `tickReqParams` | `quote_stream_genericticks`, `api_generic_tick_matrix_aapl`, `api_tick_news_aapl_probe`, `tick_news_aapl_sv201_live.txt`, `api_option_campaign_aapl`, `api_option_calculations_aapl`, `option_calculations_aapl_live.txt` | candidate | TickPrice attributes and companion size, mapped and unmapped TickPrice/TickSize IDs, an omitted TickReqParams minimum tick, TickGeneric, TickString, TickNews, and option-computation success/sentinel shapes are live-attested and publicly delivered. The calculation replay freezes exact computed and unavailable field-presence semantics. TickEFP remains unimplemented and has no live capture. |
| MD1-005 | Real-time bars | `MarketData().SubscribeRealTimeBars`, official `reqRealTimeBars`, `cancelRealTimeBars`, `realtimeBar` | `realtime_bars_aapl`, `api_market_data_completeness_aapl`, `api_realtime_bars_request_errors_aapl.txt` | candidate | The catalog scenario now runs only through the public API and strictly accepts a typed bar or the live-attested `OpRealTimeBars` code 420 permission refusal; server_version 206 reverified the refusal and a current-time fence without a spurious cancel (`ec740c40846dc4ad`). AAPL TRADES/BID_ASK/MIDPOINT request-scoped errors are replay-promoted from capture `f692fc168a53da9d`. Successful bars, RTH variants, cancellation after data, and reconnect remain open. |

## Market Data L2 And Tick-By-Tick

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| MD2-001 | Market depth regular and smart | `MarketData().SubscribeDepth`, official `reqMktDepth`, `cancelMktDepth`, `updateMktDepth`, `updateMktDepthL2`, `rerouteMktDepthReq` | `market_depth_aapl`, `market_depth_aapl_smart`, `market_depth_error.txt`; exact-206 codec and engine tests backed by capture hashes in `protocol-audit-sv206.md` | candidate | Both catalog scenarios are public-API-only. Readonly server_version 206 returned terminal regular-depth code 10092 (`c45b90e97854b1cb`) and the nonterminal SMART-depth exchange notice 2152 (`ea56919e47983830`). The regular route fenced without a redundant cancel; the SMART route preserved the notice, then cancelled and fenced explicitly when its exact payload listed no available depth venues. This freezes the official code-2152 semantics: it is an availability message, not a universal entitlement failure, and valid rows may follow when the payload lists available depth exchanges. Exact sv206 request/cancel and 2,550 raw-212 rows remain live-attested; classic CFD reroutes replace the active request while preserving rows/smart mode. Classic depth rejects extended Contract fields because its layout carries none; sv206 accepts the complete shared Contract. Positive raw-213 L2 remains blocked on entitlement; smart cancellation and L1 insert/update/delete are frozen. |
| MD2-002 | Tick-by-tick streams | `MarketData().SubscribeTickByTick`, official `reqTickByTickData`, `cancelTickByTickData`, tick-by-tick callbacks | `tick_by_tick_last`, `tick_by_tick_bidask`, `tick_by_tick_midpoint`, `api_market_data_completeness_aapl`, `api_tick_by_tick_entitlement_errors_aapl.txt` | candidate | All three catalog scenarios now run only through the public API and strictly accept their typed tick shape or the live-attested 10089/10189 entitlement refusals. Server_version 206 returned exact code 10089 for Last, BidAsk, and MidPoint and each public runner fenced the terminal route without the old raw code-300 cancel noise (`f9b2d1a88116f12b`, `d055fa53d960c765`, `8cc2ca68e6eb2902`); server_version 200 separately attested tick-by-tick-specific code 10189 (`d29fed1cb48c2be9b`). The broader Last/AllLast/AllLast-ignore-size/BidAsk/MidPoint error matrix is replay-promoted from capture `f692fc168a53da9d`. Successful families, request limits, pacing, and cancellation after data remain open. |

## Historical Data

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| HIST-001 | Historical bars | `History().Bars`, official `reqHistoricalData`, `cancelHistoricalData`, `historicalData`, `historicalDataEnd` | `historical_bars_1d_1h`, `historical_bars_30d_1day`, `historical_bars_bidask`, `historical_bars_error`, `api_historical_matrix_aapl`, `historical_bars.txt`, `grounded_historical_bars.txt` | candidate | every supported bar-size family, durations, RTH true/false, TRADES/BID/ASK/BID_ASK/MIDPOINT/ADJUSTED_LAST, errors |
| HIST-002 | Historical keep-up bars | `History().SubscribeBars`, official keepUpToDate, `historicalDataUpdate` | `historical_bars_keepup`, `historical_bars_stream.txt` | promoted | live updates, cancel path, reconnect behavior |
| HIST-003 | Historical schedule | `History().Schedule`, official `whatToShow=SCHEDULE`, `historicalSchedule` | `historical_schedule_aapl`, `historical_schedule_aapl.txt`, `TestHistoricalSchedule`, `TestCaptureDecode_HistoricalSchedule` | candidate | grounded live decode from server_version 200, captures/20260411T175212Z events.jsonl sha256 1b207a57180e6197; extend to non-US exchanges |
| HIST-004 | Head timestamp and histogram | `History().HeadTimestamp`, `History().Histogram`, official head/histogram calls | `head_timestamp_aapl`, `histogram_data_aapl`, `head_timestamp.txt`, `histogram_data.txt` | promoted | RTH true/false, whatToShow variants, invalid contract, entitlement errors |
| HIST-005 | Historical ticks | `History().Ticks`, official historical midpoint/bidask/last callbacks | `historical_ticks_aapl_trades`, `historical_ticks_aapl_bidask`, `historical_ticks_aapl_midpoint`, `historical_ticks_aapl_timezone_start`, historical tick transcripts | promoted | Start-only and end-only public requests, explicit-zone formatting, tick attributes, and exact live permission failures. Multi-callback `done=false` accumulation remains unattested. |
| HIST-006 | Historical news | `News().Historical`, official `reqHistoricalNews`, historical news callbacks | `historical_news_aapl`, `historical_news_aapl_timezone_window`, `api_news_article_aapl`, `historical_news_end_bound_sv206_live.txt` | promoted | provider combinations, one-sided `.0 UTC` lower end bound, invalid provider, article follow-up; both time bounds are rejected because IBKR ignores the lower end bound when both are set |

## Orders And Executions

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| ORD-001 | Basic order placement | `Orders().Place`, official `placeOrder`, `openOrder`, `orderStatus` | existing basic/order-type campaigns plus `order_lifecycle_sv203_live.txt` | promoted | Classic families remain frozen by the 2026-06-11 campaigns. Exact sv203 now mirrors the official encoder's explicit `Contract.conId=0` behavior; fresh guarded paper capture `8efd714c3885da23` proves place/open/status and targeted cancel. Its later global cancel was live-flushed and returned code 161 because the sole order was already terminal. Classic place carries SecurityID/full BAG legs/delta-neutral but rejects IncludeExpired and IssuerID; sv203 uses the complete shared Contract. |
| ORD-002 | Direct and handle cancel | `Orders().Cancel`, `OrderHandle.Cancel`, `WithManualCancelTime`, `WithCancelExternalOperator`, `WithCancelManualOrderIndicator`, official `cancelOrder` | `place_order_cancel`, `place_order_direct_cancel`, `cancel_order.txt`, `direct_cancel_order.txt` | promoted | direct cancel by ID and handle cancel both frozen; compliance metadata is source-grounded and exposed with local validation, while non-empty live attestation remains pending; cancel unknown order and terminal status remain target variants |
| ORD-003 | Modify | `OrderHandle.Modify`, official modify by re-sending `placeOrder` | `place_order_modify`, `place_order_modify.txt` | promoted | price, quantity, TIF, forbidden side/contract changes, mismatched order ID rejection |
| ORD-004 | Global cancel | `Orders().CancelAll`, `WithCancelExternalOperator`, `WithCancelManualOrderIndicator`, official `reqGlobalCancel` | `global_cancel`, `global_cancel.txt` | promoted | no orders, many orders, mixed basic/bracket/OCA/conditional, post-cancel open-orders check; non-empty compliance metadata is source-grounded but still needs live attestation |
| ORD-005 | Open orders | `Orders().Open`, `Orders().SubscribeOpen`, `Orders().RefreshOpen`, official open/all/auto-open-order scopes | existing open/reconnect/cross-client captures and replays | promoted | Own/all/client-0/cross-client/reconnect/refresh paths are promoted. Dual delivery to an `OrderHandle` and open-orders subscription deep-clones Contract, `OrderCombo` decimal pointers, routing/algo tags, and conditions. The ownership regression labels order/routing from live BAG order 443, legs from the exact-200 BAG quote, and its leg price as source-law-only. Remaining target: auto-bind/orderBound. |
| ORD-006 | Completed orders | `Orders().Completed`, official `reqCompletedOrders`, `completedOrder`, `completedOrdersEnd` | `api_completed_orders_variants_aapl`, `api_completed_orders_variants_aapl.txt`, `completed_orders`, `completed_orders.txt`, `completed_orders_cancelled_system_live.txt`, `completed_orders_sv204_live.txt` | promoted | The catalog scenario now uses `Orders().Completed` rather than a parallel wire request. Exact sv206 evidence freezes both outcomes: the paper snapshot ends normally (`9f7bdc11f8012a5e`), while the read-only role's unkeyed `-'S'` code-321 refusal is a typed terminal result followed by a current-time fence (`ff9808ba8f3f33f2`). The classic codec parses every source-defined field sequentially, rejects unparsed drift, and projects the complete typed v200 result. Exact sv204 is official-SDK/live-attested for absent-false and present-true requests plus cancelled and filled protobuf results; the filled frame preserves commission-and-fees amount/currency, and optional order/client/parent identities preserve explicit zero. Nondefault combo, scale-extension, hedge, active delta-neutral, PEG BENCH, condition, and FA completed-order branches remain live-attestation gaps. |
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
| AORD-005 | Combo/BAG | official combo legs, combo prices, smart combo routing params | `api_combo_option_vertical_aapl`; exact quote composition vectors | candidate | Live option vertical order 443 was accepted then cancelled with NonGuaranteed routing. A separate exact-200 BAG quote freezes legs 887307502/887307536 as composition evidence. `Contract.ComboLegs` and shared `OrderCombo` placement/open/completed projection are implemented. Live nondefault per-leg prices, execution, and completed-order observation remain gaps. |
| AORD-006 | Hedge orders | official hedging | `api_hedge_order_aapl`, `api_hedge_order_aapl.txt` | promoted | Five live rules frozen 2026-06-11: beta and pair hedges accepted at zero size (Gateway computes the child quantity and floors the limit), delta hedges require an option parent (320) and a valid ratio (320), FX hedging requires a matching currency-pair child (10063, terminal handle error via the attested placement-rejection set), and sizing a hedge child draws 10032 (sibling-session evidence in the header). |
| AORD-007 | Delta-neutral extensions | official delta-neutral order/contract fields | exact-200 rejected OPT request; source-schema tests | candidate | Public DeltaNeutral requires BAG. The captured OPT+delta-neutral quote was rejected with code 320 and is negative evidence only. Successful BAG delta-neutral quote/order/open/completed behavior and deltaNeutralValidation remain live gaps. |
| AORD-008 | Scale orders | official scale fields | `api_tif_attribute_matrix_aapl` | candidate | scale init/subs size, increment, table, active times, open-order decode |
| AORD-009 | Pegged and adjusted order families | official PEG BENCH, PEG BEST, PEG MID, adjusted stop/trailing fields | `api_order_type_matrix_aapl`, `api_tif_attribute_matrix_aapl` | candidate | live accepted/rejected captures for each specialized branch |
| AORD-010 | Regulatory/allocation order fields | `Orders().Preview`, official FA allocation, MiFID, manual order time, soft-dollar, advancedErrorOverride, IBKRATS | `api_whatif_margin_aapl`, `api_whatif_margin_aapl.txt`, `api_tif_attribute_matrix_aapl` | candidate | WhatIf margin preview promoted 2026-06-11 from the 2026-06-10 capture (`e8ee70b24de3fe2f`): `Orders().Preview` owns the what-if flag and returns the order-state block as an OrderState (the nine margin decimals plus commission range and currency); no lifecycle follows and no handle is created. Direct Preview was revalidated at server version 206 on 2026-07-10 (`6f61cc2b44b11d33`) after removing caller-settable order identity and preview mode. MiFID, manual-order-time acceptance, FA allocation, and IBKRATS variants remain open. |

## Options

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| OPT-001 | Option calculations | `Options().ImpliedVolatility`, `Options().Price`, official calculate/cancel calls | `api_option_calculations_aapl`, `option_calculations_aapl_live.txt`, live tests | promoted | A live-qualified AAPL call freezes both successful calculations byte-for-byte, including distinct computed-field masks and unavailable sentinels. Invalid-contract/error and cancellation-before-first-computation variants remain open. |
| OPT-002 | Option exercise/lapse | `Options().Exercise`, official `exerciseOptions` | `api_option_exercise_aapl`, `api_option_exercise_not_itm_aapl.txt`, `api_option_exercise_server_reject_aapl.txt` | promoted | Both live outcomes frozen 2026-06-11: a barely-ITM call drew 322 "not in-the-money", and a deep-ITM call was accepted with the 10349 TIF-preset session event before paper clearing returned 322 and 202. Exercise replies are surfaced as session events while an exercise req-id route is active, and terminal 322/202 notices retire that route. Lapse and override variants plus a true clearing settlement remain open. |
| OPT-003 | Option order and data integration | `Orders().Place`, market data/history for OPT | `place_order_option_buy`, `api_option_campaign_aapl`, `api_combo_option_vertical_aapl` | candidate | option quote, historical ticks if available, order fill/reject, completed/execution observation |

## News, Scanner, FA, WSH, Display, And TWS

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| NEWS-001 | News providers and bulletins | `News().Providers`, `News().SubscribeBulletins`, official provider/bulletin calls | `news_providers`, `news_bulletins`, `news_providers.txt`, `news_bulletins_live.txt` | promoted | The exact three-provider list and two live exchange notices are replayed. The catalog bulletin scenario now uses the public subscription, treats a quiet bounded window as valid, and live-verifies cancel/wait/current-time fencing at sv206 (`bcff4c85f318eb48`). `allMessages=false` and provider-entitlement variants remain open. |
| NEWS-002 | News article | `News().Article`, official `reqNewsArticle`, `newsArticle` | `api_news_article_aapl`, `news_article.txt` | candidate | article from captured historical-news ID, invalid article ID, provider-specific errors |
| NEWS-003 | Historical news | `News().Historical`, official historical news callbacks | `historical_news_aapl`, `historical_news_aapl_timezone_window`, `api_news_article_aapl` | promoted | see HIST-006 |
| SCAN-001 | Scanner parameters | `Scanner().Parameters`, official `reqScannerParameters`, `scannerParameters` | `scanner_parameters`, `scanner_parameters.txt` | promoted | XML decode/size, unavailable scanner service |
| SCAN-002 | Scanner subscriptions | `Scanner().SubscribeResults`, official subscription/cancel callbacks | `api_scanner_subscription`, `scanner_subscription_live.txt` | promoted | The single retained catalog scenario is public-API-only and requires a complete ranked or empty result, or exact terminal code 490. Server-version-206 capture `4d32e5b9b88ae438` returned ten ranked rows followed by public cancel and a current-time fence; the server-version-200 replay `c84c81b3ee772bcc` preserves the exact result frame. Live capture `14cdc2913735bb3c` proves exact code 165 `no items retrieved` is nonterminal and precedes a valid empty result; `740b5dfb138df2a4` grounds the permission-refusal branch. Generic filter values and `tag=value;` serialization are grounded in official source. Non-default filter captures remain an evidence gap. |
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
| ERR-003 | Entitlement and account-type failures | market data, WSH, FA, scanner, orders | `market_depth_error.txt`, `wsh_meta_data_error.txt`, `api_wsh_variants_aapl.txt`, entitlement candidate captures | candidate | freeze real paper-account blocked responses instead of inventing mocks; WSH metadata/event variants replay-promoted from 2026-04-15 capture `65aeb0a3b716e4b6` |

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
| `api_generic_tick_matrix_aapl` | MD1-003, MD1-004 |
| `api_tick_news_aapl_probe` | MD1-003, MD1-004 |
| `api_forex_lifecycle_eurusd` | ORD-001 |
| `api_forex_lifecycle_eurusd.txt` | ORD-001 |
| `api_future_campaign_mes` | ORD-001 |
| `api_historical_matrix_aapl` | HIST-001 |
| `api_ioc_fok_aapl` | ORD-001 |
| `api_market_data_completeness_aapl` | MD1-003 |
| `api_market_data_type_cycle.txt` | MD1-001 |
| `api_news_article_aapl` | NEWS-002 |
| `api_option_calculations_aapl` | MD1-004, OPT-001 |
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
| `historical_ticks_aapl_timezone_start` | HIST-005 |
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
| `positions_multi` | ACCT-005 |
| `positions_snapshot` | ACCT-004 |
| `qualify_contract_aapl_exact` | REF-001 |
| `qualify_contract_ambiguous` | REF-001 |
| `quote_snapshot_aapl` | MD1-002 |
| `quote_stream_aapl` | MD1-003 |
| `quote_stream_genericticks` | MD1-003 |
| `quote_stream_multi_asset` | MD1-003 |
| `option_calculations_aapl_live.txt` | MD1-004, OPT-001 |
| `tick_news_aapl_sv201_live.txt` | MD1-003, MD1-004 |
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
| `trading_split_round_trip_aapl` | ORD-009 |
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
- `Orders().Cancel`: add direct cancel-by-ID live scenario distinct from
  `OrderHandle.Cancel`.
- `Options().Exercise`: add exercise and lapse paper scenarios plus invalid
  option/account response.
- `News().Article`: add article lookup using an article ID from a live
  historical-news capture.
- `Advisors().ReplaceConfig`: add non-FA error capture and FA read-back/restore
  scenario if an FA account is ever available.
- Official callback gaps from `ibkr-api-inventory.md`: `tickEFP`,
  `orderBound`, `replaceFAEnd`, verify callbacks, and bond contract
  details.

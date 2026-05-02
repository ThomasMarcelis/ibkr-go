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

SDK-native migration note: on 2026-05-02, `TestLiveOfficialSDKSmoke` passed
outside the workspace sandbox against local Gateway/TWS on `127.0.0.1:4002`,
serverVersion 203. The same C++ SDK connect path still fails inside the
workspace sandbox with SDK error 520 (`Failed to create socket`), so live SDK
verification commands that open C++ sockets must run outside the sandbox. The
read-only smoke covered session bootstrap, current time, market-data type
control, account summary, contract details, head timestamp and histogram for
AAPL, sec-def option params for AAPL, matching symbols, market rule 26, an AAPL
quote snapshot, smart components using the quote-derived BBO exchange (observed
`9c0001`, returned 0 rows), positions, family codes, market-depth exchanges,
news providers, scanner parameters, soft-dollar tiers, user info, display
groups, and WSH metadata/event data entitlement errors. `CurrentTimeMillis`
passed in a separate fresh-session smoke; serverVersion 203 only answered the
first current-time-style request per session in diagnostics. Sanitized
live-derived SDK-event fixtures now live under
`internal/sdkadapter/testdata/fixtures/`: `official_sdk_read_only_20260502.json`
and `official_sdk_current_time_millis_20260502.json`, both with account
identifiers redacted to `DU_REDACTED`; dedicated account-summary,
account-stream, and family-code fixtures freeze redacted account callback
shapes, with account-summary financial values redacted to `REDACTED_VALUE` and
account-stream values, private position contracts, model codes, and PnL values
redacted. Public SDK live coverage also exercises
the account-summary and positions subscription snapshot/cancel paths, delayed
AAPL quote streams, AAPL historical bars, keep-up-to-date historical bars through the
initial snapshot and close path, historical schedule, historical midpoint
ticks, AAPL `ReportSnapshot` fundamental data, and short scanner result
subscriptions. Fundamental data has a dedicated SDK-event fixture for AAPL
`ReportSnapshot` success on serverVersion 203; expected-error and cancel
variants remain open. News article and historical-news one-shots now have SDK
command/event/native coverage but remain
out of the live smoke until provider/article-ID and entitlement expectations are
frozen. News bulletin subscriptions now have SDK command/event/native
coverage but remain out of the live smoke until bulletin/cancel expectations
are frozen. Account update streams, account-update multi, positions multi,
PnL, and PnL single now have SDK command/event/native coverage plus public SDK
live subscription evidence and a sanitized SDK-event fixture; account/model,
post-trade delta, and contract variants remain open. Scanner result
subscriptions have a dedicated short SDK-event fixture and public SDK live
coverage; no-result and error variants remain open. Display group subscriptions have a
dedicated subscribe/unsubscribe SDK-event fixture; an explicit
`updateDisplayGroup` fixture attempt for `265598@SMART` timed out without a
target callback and invalid group variants remain open. FA config reads
and writes now have SDK command/event/native coverage, but the current
non-FA paper account produced no `receiveFA` callback or API error before a
15s public probe context expired; success and replace/read-back evidence need
an FA-enabled account. Historical
one-shots now have SDK command/event/native, fixture, and public live evidence
for the core AAPL RTH paths; keep-up historical bars have an SDK-event initial
snapshot fixture, but still need a market-hours `historicalDataUpdate` fixture
and reconnect evidence.

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
| ACCT-001 | Account summary | `Accounts().Summary`, `Accounts().SubscribeSummary`, official `reqAccountSummary`, `cancelAccountSummary`, `accountSummary`, `accountSummaryEnd` | `account_summary_snapshot`, `account_summary_stream`, `account_summary_two_subs`, `account_summary.txt`, `account_summary_stream.txt`, `account_summary_two_subs.txt`, `account_summary_disconnect_after_end.txt`, `grounded_account_summary.txt`; SDK-event fixture `official_sdk_account_summary_snapshot_20260502.json` source sha256 `64adc4661bdaeba4b6c40b3808159194a32cd302b65978a17d0a622cef2b7364` freezes `NetLiquidation` and `BuyingPower` callback shape with values redacted; SDK-backed `TestLiveOfficialSDKReadOnlySubscriptions` passed on 2026-05-02 with account-summary subscription rows=4 and clean close | promoted | one-shot All summary and public subscription snapshot/cancel frozen; concrete account, full tag set, two concurrent subs, cancel before end remain open |
| ACCT-002 | Account updates and portfolio | `Accounts().Updates`, `Accounts().SubscribeUpdates`, official `reqAccountUpdates`, `updateAccountValue`, `updatePortfolio`, `updateAccountTime`, `accountDownloadEnd` | `account_updates`, `account_updates.txt`; SDK-backed `TestLiveOfficialSDKAccountStreams` passed on 2026-05-02 with account-updates subscription snapshot rows=152 and clean close after default event/subscription buffers were raised to 1024 for burst snapshots; SDK-event fixture `official_sdk_account_streams_snapshot_20260502.json` source sha256 `287bbbdbe96422f18b116f67bc1fcc09b34d017e1009784539f21aad420bd923` freezes redacted `updateAccountValue`, `updatePortfolio`, and `accountDownloadEnd` shapes; `TestSDKAccountUpdatesPublicRouteReplaysAccountValueFixture` replays the redacted account-value public route and unsubscribe command | promoted | portfolio replay through the public facade is limited by private numeric redaction; during marketable trades, multiple asset positions, and one-account official timing limitation remain open |
| ACCT-003 | Account updates multi | `Accounts().UpdatesMulti`, `Accounts().SubscribeUpdatesMulti`, official `reqAccountUpdatesMulti`, `cancelAccountUpdatesMulti`, `accountUpdateMulti`, `accountUpdateMultiEnd` | `account_updates_multi`, `account_updates_multi.txt`; SDK-backed `TestLiveOfficialSDKAccountStreams` passed on 2026-05-02 with account-updates-multi subscription snapshot rows=75 and clean close; SDK-event fixture `official_sdk_account_streams_snapshot_20260502.json` freezes redacted `accountUpdateMulti` rows and `accountUpdateMultiEnd`; `TestSDKAccountUpdatesMultiPublicRouteReplaysOfficialFixture` replays the public snapshot route and cancel command from that fixture | promoted | account/model combinations, empty result, cancel mid-stream, and post-trade deltas |
| ACCT-004 | Positions | `Accounts().Positions`, `Accounts().SubscribePositions`, official `reqPositions`, `cancelPositions`, `position`, `positionEnd` | `positions_snapshot`, `positions.txt`, `positions_disconnect_after_end.txt`, `grounded_positions.txt`; SDK-backed `TestLiveOfficialSDKSmoke` passed on 2026-05-02 with positions rows=6, and `TestLiveOfficialSDKReadOnlySubscriptions` passed on 2026-05-02 with positions subscription rows=6 and clean close; SDK-event fixture `official_sdk_account_streams_snapshot_20260502.json` freezes redacted `position` rows and `positionEnd` | promoted | one-shot, public subscription snapshot/cancel path, and redacted SDK-event fixture frozen; empty accounts, asset mix variants, and streaming during trades remain open |
| ACCT-005 | Positions multi | `Accounts().PositionsMulti`, `Accounts().SubscribePositionsMulti`, official `reqPositionsMulti`, `cancelPositionsMulti`, `positionMulti`, `positionMultiEnd` | `positions_multi`, `positions_multi.txt`; SDK-backed `TestLiveOfficialSDKAccountStreams` passed on 2026-05-02 with positions-multi subscription snapshot rows=6 and clean close; SDK-event fixture `official_sdk_account_streams_snapshot_20260502.json` freezes redacted `positionMulti` rows and `positionMultiEnd` | promoted | account/model variants, empty result, and streaming during trades |
| ACCT-006 | PnL account and single-position streams | `Accounts().SubscribePnL`, `Accounts().SubscribePnLSingle`, official `reqPnL`, `cancelPnL`, `reqPnLSingle`, `cancelPnLSingle`, `pnl`, `pnlSingle` | `pnl`, `pnl_single`, `pnl.txt`, `pnl_single.txt`; SDK-backed `TestLiveOfficialSDKAccountStreams` passed on 2026-05-02 with one live PnL update and one live PnLSingle update, logging no account IDs, conIDs, holdings, or values; SDK-event fixture `official_sdk_account_streams_snapshot_20260502.json` freezes redacted `pnl` and `pnlSingle` callback shapes without committing the selected live conID; deterministic public tests freeze subscribe/cancel command lifecycles without parsing redacted private PnL values | promoted | before/during/after trades, invalid conID, model code, and open option/future positions |
| ACCT-007 | Family codes | `Accounts().FamilyCodes`, official `reqFamilyCodes`, `familyCodes` | `family_codes`, `family_codes.txt`; SDK-event fixture `official_sdk_family_codes_snapshot_20260502.json` source sha256 `af54ff7bd7a7cf603b9ca8774b9f42f99ca242ce3f0fa34db66f5177df44e3d1` freezes the single-account callback with account ID redacted | promoted | single-account callback frozen; multi-family accounts and non-family account response remain open |

## Contracts And Reference Data

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| REF-001 | Contract details and qualification | `Contracts().Details`, `Contracts().Qualify`, official `reqContractDetails`, `contractDetails`, `bondContractDetails`, `contractDetailsEnd` | `contract_details_aapl_stk`, `contract_details_aapl_opt`, `contract_details_eurusd_cash`, `contract_details_es_fut`, `contract_details_not_found`, `qualify_contract_aapl_exact`, `qualify_contract_ambiguous`, `api_security_type_probe_matrix`, `contract_details.txt`, `grounded_contract_details_aapl.txt`; SDK-backed `TestLiveOfficialSDKSmoke` passed on 2026-05-02 with AAPL contractDetails rows=1 and public `Contracts().Qualify` conID=265598; `TestSDKContractDetailsPublicRouteReplaysReadOnlyFixture` and `TestSDKQualifyPublicRouteReplaysReadOnlyFixture` replay the public facades from `official_sdk_read_only_20260502.json`; `official_sdk_bond_contract_details_snapshot_20260502.json` source sha256 `4c01eb8b0be1532ae2b678f62ac731838f57c89c48b83a9c5cfbb0141932171d` freezes the distinct `bondContractDetails` callback from the official SDK sample CUSIP | promoted | STK, OPT, FUT, FOP, CASH, BOND, FUND, IND, BAG; exact, ambiguous, invalid, expired/includeExpired. 2026-04-15 probe captured real details/errors for STK/OPT/FUT/FOP/CASH/BOND/CFD/WAR/IND/CRYPTO/FUND/BILL/CMDTY/CONTFUT |
| REF-002 | Matching symbols | `Contracts().Search`, official `reqMatchingSymbols`, `symbolSamples` | `matching_symbols_aapl`, `matching_symbols_partial`, `matching_symbols.txt` | promoted | broad pattern, exact-ish pattern, derivative sec types, description/issuer fields |
| REF-003 | Option chain metadata | `Contracts().SecDefOptParams`, official `reqSecDefOptParams`, `securityDefinitionOptionParameter`, `securityDefinitionOptionParameterEnd` | `sec_def_opt_params_aapl`, `sec_def_opt_params.txt` | promoted | STK underlyings, FUT/FOP underlyings, empty exchange, invalid underlying |
| REF-004 | Market rules and smart components | `Contracts().MarketRule`, `Contracts().SmartComponents`, official `reqMarketRule`, `reqSmartComponents`, `marketRule`, `smartComponents` | `market_rule`, `smart_components`, `market_rule.txt`, `smart_components.txt`; SDK-backed smoke passed earlier on 2026-05-02 with marketRule increments=1 and quote-derived BBO exchange `9c0001` returning smartComponents rows=0; later competing-session smoke rerun logged code 10197 before the quote-derived smart-components request | promoted | US equity, option, future, invalid market rule, invalid BBO exchange |
| REF-005 | Market-depth exchanges | `Contracts().DepthExchanges`, official `reqMktDepthExchanges`, `mktDepthExchanges` | `mkt_depth_exchanges`, `mkt_depth_exchanges.txt` | promoted | all returned service data types, SMART support, invalid routing implication |
| REF-006 | Fundamental data | `Contracts().FundamentalData`, official `reqFundamentalData`, `cancelFundamentalData`, `fundamentalData` | `fundamental_data_aapl`, `api_fundamental_reports_aapl`, `fundamental_data.txt`; SDK-event fixture `official_sdk_fundamental_data_snapshot_20260502.json` source sha256 `5a011b88fe2fb979be15619edfe925d193a9a63f7a5ff3083d5ff15f8dd6a84e` freezes AAPL `ReportSnapshot` XML success; SDK-backed `TestLiveOfficialSDKHistoryAndFundamental` passed on 2026-05-02 with AAPL `ReportSnapshot` bytes=11534; later competing-session rerun bounded the request and logged a timeout only after historical data had already returned competing-session blockers | candidate | all `FundamentalReportType` values, entitlement error, invalid report type, cancel path |

## Market Data L1

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| MD1-001 | Market data type control | `MarketData().SetType`, official `reqMarketDataType`, `marketDataType` | `set_type_live`, `set_type_frozen`, `set_type_delayed`, `set_type_delayed_frozen`, `set_type_invalid`, `set_type_switch_while_streaming`, `api_market_data_completeness_aapl`, `quote_delayed_data.txt`, `lifecycle_set_mdt_after_close.txt`; SDK-native delayed SetType smoke passed on 2026-05-02 against serverVersion 203 | candidate | live evidence: bare SetType(1/2/3/4) is accepted silently — no marketDataType push arrives without an active quote stream. Invalid value 99 is also accepted silently. Switching mid-stream triggers a real entitlement error 10089 for live data on a paper account. Transcript promotion pending per-type replay tests tied to quote streams that exercise the push |
| MD1-002 | Quote snapshots | `MarketData().Quote`, official `reqMktData`, `cancelMktData`, `tickSnapshotEnd` | `quote_snapshot_aapl`, `api_market_data_completeness_aapl`, `quote_snapshot.txt`, `quote_delayed_data.txt`; SDK-native AAPL quote smoke passed on 2026-05-02 against serverVersion 203 and exposed tickReqParams-derived BBO exchange `9c0001`; later competing-session smoke rerun logged exact code 10197 (`No market data during competing live session`) | promoted | STK/OPT/FUT/CASH snapshots, entitlement error, no data, regulatory snapshot where applicable |
| MD1-003 | Quote streams and generic ticks | `MarketData().SubscribeQuotes`, official tick callbacks | `quote_stream_aapl`, `quote_stream_genericticks`, `quote_with_generic_ticks`, `quote_stream_multi_asset`, `api_market_data_completeness_aapl`, `api_duplicate_quote_subscriptions_aapl`, `quote_with_generic_ticks.txt`; SDK-native adapter route has deterministic coverage; SDK-event fixture `official_sdk_quote_stream_short_20260502.json` source sha256 `a514f2e78dc4c5fb66ca5b7e9b21a322ce2dcadf8d87ee7d5324d408d639e72e` freezes delayed AAPL stream callbacks; SDK-backed `TestLiveOfficialSDKReadOnlySubscriptions` passed on 2026-05-02 with a delayed AAPL stream update and clean close; later competing-session rerun logged quote-stream code 10197 after an empty pre-terminal update | candidate | generic/option/EFP/news/dividend/shortable/RTVolume/fundamental-ratio generic tick families; duplicate same-contract subscriptions captured 2026-04-15 (`84f1e78a18616e0f`) |
| MD1-004 | Tick callback edge shapes | official `tickPrice`, `tickSize`, `tickString`, `tickGeneric`, `tickEFP`, `tickOptionComputation`, `tickNews`, `tickReqParams` | `calc_implied_volatility.txt`, `calc_option_price.txt`, quote fixtures | target | tickEFP, tickNews, tickReqParams, option computation live success/error |
| MD1-005 | Real-time bars | `MarketData().SubscribeRealTimeBars`, official `reqRealTimeBars`, `cancelRealTimeBars`, `realtimeBar` | `realtime_bars_aapl`, `api_market_data_completeness_aapl`, `realtime_bars_reconnect.txt`; SDK-event fixture `official_sdk_real_time_bars_short_20260502.json` source sha256 `baaa5d01dd79577b4c16d11632138bb3e751e811e2576950712daba7d3378b98` freezes current paper-account code 420 real-time-bar permission error | promoted | TRADES/MIDPOINT/BID_ASK, RTH true/false, cancel, reconnect, success fixture with market-data permission |

## Market Data L2 And Tick-By-Tick

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| MD2-001 | Market depth regular and smart | `MarketData().SubscribeDepth`, official `reqMktDepth`, `cancelMktDepth`, `updateMktDepth`, `updateMktDepthL2` | `market_depth_aapl`, `market_depth_aapl_smart`, `market_depth_error.txt`; SDK-native adapter route has deterministic coverage; SDK-event fixture `official_sdk_market_depth_smart_short_20260502.json` source sha256 `6850cc11ff16f4bb417b85c015de64a94a54dd2fff88025aaa3d9be2284fef4c` freezes current paper-account code 2152 smart-depth permission error | candidate | L1 vs L2 rows, SMART depth, insert/update/delete, market maker names, entitlement errors, cancel; success fixture requires market-depth permission |
| MD2-002 | Tick-by-tick streams | `MarketData().SubscribeTickByTick`, official `reqTickByTickData`, `cancelTickByTickData`, tick-by-tick callbacks | `tick_by_tick_last`, `tick_by_tick_bidask`, `tick_by_tick_midpoint`, `api_market_data_completeness_aapl`, `tick_by_tick.txt`; SDK-native adapter route has deterministic coverage; SDK-event fixture `official_sdk_tick_by_tick_midpoint_short_20260502.json` source sha256 `d43018da4703e8bda7cdac8f3eab51600ed8ba89cc676e46cab6c9398627b459` freezes current paper-account code 10189 entitlement error for AAPL MidPoint | candidate | Last, AllLast, BidAsk, MidPoint, ignoreSize true/false, numberOfTicks, pacing, unavailable data; success fixture requires market-data permission |

## Historical Data

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| HIST-001 | Historical bars | `History().Bars`, official `reqHistoricalData`, `cancelHistoricalData`, `historicalData`, `historicalDataEnd` | `historical_bars_1d_1h`, `historical_bars_30d_1day`, `historical_bars_bidask`, `historical_bars_error`, `api_historical_matrix_aapl`, `historical_bars.txt`, `grounded_historical_bars.txt`; SDK-event fixture `official_sdk_historical_bars_short_20260502.json` source sha256 `496348c4f40dfa0a64cd607d1921167ff344b20547bd406f7a353eda4f4bcc64` freezes AAPL one-day one-hour RTH bars; SDK-backed `TestLiveOfficialSDKHistoryAndFundamental` passed on 2026-05-02 with public AAPL rows=7; later competing-session rerun logged code 162 with the different-IP message | candidate | every supported bar-size family, durations, RTH true/false, BID/ASK/BID_ASK/MIDPOINT/ADJUSTED_LAST, errors |
| HIST-002 | Historical keep-up bars | `History().SubscribeBars`, official keepUpToDate, `historicalDataUpdate` | `historical_bars_keepup`, `historical_bars_stream.txt`; SDK-backed `TestLiveOfficialSDKReadOnlySubscriptions` passed on 2026-05-02 by awaiting the public keep-up initial snapshot rows=7 and closing the subscription; SDK-event fixture `official_sdk_historical_bars_keepup_short_20260502.json` source sha256 `f6d87ab4c4a407bcd24e003647eebb87866a871411ad8bb06edc3769a15fbf32` freezes the seven-row AAPL RTH initial snapshot with no update on the Saturday capture; `TestSDKHistoricalBarsSubscriptionPublicRouteReplaysOfficialFixture` replays the public subscription snapshot and cancel command from that fixture; later competing-session rerun logged code 162 with the different-IP message | blocked | `historicalDataUpdate` during market hours and reconnect behavior |
| HIST-003 | Historical schedule | `History().Schedule`, official `whatToShow=SCHEDULE`, `historicalSchedule` | `historical_schedule_aapl`, `historical_schedule_aapl.txt`, `TestHistoricalSchedule`, `TestCaptureDecode_HistoricalSchedule`; SDK-event fixture `official_sdk_historical_schedule_short_20260502.json` source sha256 `58271ad7ab173e061158f8a2f4b2fc36fd6a88514de744b8b9c179a00b1aa5d4` freezes AAPL one-month RTH sessions; SDK-backed `TestLiveOfficialSDKHistoryAndFundamental` passed on 2026-05-02 with sessions=21 and timezone `US/Eastern` | candidate | grounded live decode from server_version 200, captures/20260411T175212Z events.jsonl sha256 1b207a57180e6197; extend to non-US exchanges |
| HIST-004 | Head timestamp and histogram | `History().HeadTimestamp`, `History().Histogram`, official head/histogram calls | `head_timestamp_aapl`, `histogram_data_aapl`, `head_timestamp.txt`, `histogram_data.txt`; SDK-native head timestamp and histogram smoke passed on 2026-05-02 against serverVersion 203; later competing-session smoke reruns logged exact histogram code 10188 (`Trading TWS session is connected from a different IP address`) | promoted | RTH true/false, whatToShow variants, invalid contract, entitlement errors |
| HIST-005 | Historical ticks | `History().Ticks`, official historical midpoint/bidask/last callbacks | `historical_ticks_aapl_trades`, `historical_ticks_aapl_bidask`, `historical_ticks_aapl_midpoint`, `historical_ticks_aapl_timezone_window`, historical tick transcripts; SDK-event fixture `official_sdk_historical_ticks_midpoint_short_20260502.json` source sha256 `6d047b0ba55fe9f6874a766a1b35bc4164a40fb1ef1138ebfb4a14470893b2ae` freezes AAPL historical midpoint ticks; SDK-event fixture `official_sdk_historical_ticks_bidask_short_20260502.json` source sha256 `7a344933e7d5dd06221df36f9f3282b63bb2377c63149fd54899a19faf8ec3ec` freezes AAPL historical bid/ask ticks; SDK-event fixture `official_sdk_historical_ticks_trades_short_20260502.json` source sha256 `f1792cfa680c5d33a0e9834f616236085bfa862af84900f8546d1a545f1f7aa0` freezes AAPL historical trade ticks; SDK-backed `TestLiveOfficialSDKHistoryAndFundamental` passed on 2026-05-02 with public midpoint rows=172, and public replay tests cover `BID_ASK` and `TRADES` branches | promoted | start-only, explicit zone, UTC/local zone, no-data, tick attributes, ignoreSize |
| HIST-006 | Historical news | `News().Historical`, official `reqHistoricalNews`, historical news callbacks | `historical_news_aapl`, `historical_news_aapl_timezone_window`, `api_news_article_aapl`, `historical_news.txt`, `historical_news_timezone_window.txt`; SDK-event fixture `official_sdk_news_invalid_requests_20260502.json` source sha256 `fcb13d330984c46374c9afdd0c458a1f8e9eb5e97d244ba04c84cc5cc2401b9d` freezes invalid-provider code 321; SDK-event fixture `official_sdk_news_article_snapshot_20260502.json` source sha256 `e25f9cf9301804a7c6614efc580e1825d7487ab5b501b45242a67edde03567f6` freezes five redacted live AAPL historical-news rows and `historicalNewsEnd(hasMore=true)` | promoted | provider combinations, timezone windows, no-result window, invalid provider, article follow-up |

## Orders And Executions

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| ORD-001 | Basic order placement | `Orders().Place`, official `placeOrder`, `openOrder`, `orderStatus` | `place_order_lmt_buy_aapl`, `place_order_mkt_buy_aapl`, `place_order_mkt_sell_aapl`, `api_order_fill_aapl`, `api_order_rest_cancel_aapl`, `api_order_relative_cancel_aapl`, `api_order_stop_cancel_aapl`, `api_order_trailing_cancel_aapl`, `api_order_rejects_aapl`, `api_order_type_matrix_aapl`, `api_tif_attribute_matrix_aapl`, `api_transmit_false_then_transmit_aapl`, `api_transmit_false_then_transmit_aapl.txt`, `api_delayed_success_modify_aapl`, `api_future_campaign_mes`, `api_future_campaign_mes.txt`, `api_forex_lifecycle_eurusd`, `api_ioc_fok_aapl`, `api_ioc_fok_aapl.txt`, `api_tif_attribute_matrix_aapl.txt`, `place_order_limit.txt`, `place_order_fill_with_execution.txt`, `place_order_modify_to_market_late_execution.txt`, `place_order_invalid_type_live_error.txt`; SDK-native adapter route has deterministic coverage; `TestSDKPlaceOrderPublicRouteReplaysOfficialFixture` replays public handle event delivery from the paper fixture; SDK-backed `TestLiveOfficialSDKPaperOrderPlaceCancel` passed on 2026-05-02 against paper Gateway serverVersion 203 for non-marketable 1-share AAPL LMT placement through `openOrder` `PreSubmitted`; committed SDK fixture `official_sdk_paper_order_place_cancel_20260502.json` source sha256 `44f4dd370b903e85e74cfc11c9f1c464ad4d396890213dcc964632d12ebcf3e5`; committed SDK fixture `official_sdk_paper_order_reject_invalid_type_20260502.json` source sha256 `4ae50ed4eedf69ec7f6cb9421ad8dea7c6353a2777697470f2c051cc076e2642` freezes code 321 invalid order type rejection | candidate | MKT, LMT, STP, STP LMT, TRAIL, TRAIL LIMIT, MIT, LIT, MTL, REL, MOC, LOC, MOO, LOO, PEG families, DAY/GTC/IOC/FOK/GTD, marketable/far-from-market, staged Transmit=false then transmit |
| ORD-002 | Direct and handle cancel | `Orders().Cancel`, `OrderHandle.Cancel`, official `cancelOrder`, `orderStatus` | `place_order_cancel`, `place_order_direct_cancel`, `cancel_order.txt`, `direct_cancel_order.txt`; SDK-native adapter route has deterministic coverage; `TestSDKOrdersCancelPublicRouteUsesSDKCommand` freezes direct public cancel-by-ID command emission; SDK-backed `TestLiveOfficialSDKPaperOrderPlaceCancel` passed on 2026-05-02 against paper Gateway serverVersion 203 for cancel to final `Cancelled`; committed SDK fixture `official_sdk_paper_order_place_cancel_20260502.json` freezes warning code 399 as non-terminal and final `Cancelled` evidence | promoted | cancel unknown order, manual cancel time, terminal status remain target variants |
| ORD-003 | Modify | `OrderHandle.Modify`, official modify by re-sending `placeOrder` | `place_order_modify`, `place_order_modify.txt`; SDK-native adapter route has deterministic coverage; `TestSDKOrderHandleModifyPublicRouteUsesSDKCommand` freezes the public handle modify path and replays the live-derived quantity-2 `openOrder` update; SDK-backed `TestLiveOfficialSDKPaperOrderModifyCancel` passed on 2026-05-02 against paper Gateway serverVersion 203 for quantity modify from 1 to 2 and final handle cancel; committed SDK fixture `official_sdk_paper_order_modify_cancel_20260502.json` source sha256 `860489438be38dc855edaafe2496c4071cea832857d44c4972bd540e4b127aec`. Live runs also froze warning/late-notice handling for code 399 and code 201 `too late to replace` | promoted | price, TIF, forbidden side/contract changes, mismatched order ID rejection |
| ORD-004 | Global cancel | `Orders().CancelAll`, official `reqGlobalCancel`, `orderStatus` | `global_cancel`, `global_cancel.txt`; SDK-native adapter route has deterministic coverage; guarded SDK-backed paper test skipped on 2026-05-02 because `Orders().Open(all)` found 1 non-test open order and the harness refused to call global cancel | promoted | no unrelated open orders, many orders, mixed basic/bracket/OCA/conditional, post-cancel open-orders check, SDK-event fixture |
| ORD-005 | Open orders | `Orders().Open`, `Orders().SubscribeOpen`, official open/all/auto-open-order scopes | `api_client_id0_order_observation_aapl`, `api_client_id0_order_observation_aapl.txt`, `api_cross_client_cancel_aapl`, `api_cross_client_cancel_aapl.txt`, `api_reconnect_active_order_aapl`, `api_reconnect_active_order_aapl.txt`, `open_orders_empty`, `open_orders_all`, `open_orders.txt`, `open_orders_disconnect_after_end.txt`; SDK-native adapter route has deterministic coverage; `TestSDKOpenOrdersPublicRouteReplaysOfficialFixture` replays the public client-scope snapshot path and `TestSDKSubscribeOpenOrdersPublicRouteReplaysOfficialFixture` replays the public subscription snapshot path; SDK-backed `TestLiveOfficialSDKPaperOrderPlaceCancel` passed on 2026-05-02 against paper Gateway serverVersion 203 with both `Orders().Open(all)` and `Orders().SubscribeOpen(all)` observing the active paper order before cancel; SDK-event fixture `official_sdk_paper_open_orders_place_cancel_20260502.json` source sha256 `d93f8f351aef13bc227ac0d33e6d51798bbbb728227e675b1c044e485ee73971` freezes client-scope open-order snapshot callbacks for a scenario paper order | promoted | own, all, client_id_0, cross-client, and active-order reconnect are live-captured/replay-promoted; remaining targets: all-scope SDK-event fixture if the paper account has no unrelated open orders, auto-bind/orderBound |
| ORD-006 | Completed orders | `Orders().Completed`, official `reqCompletedOrders`, `completedOrder`, `completedOrdersEnd` | `api_completed_orders_variants_aapl`, `api_completed_orders_variants_aapl.txt`, `completed_orders`, `completed_orders.txt`; SDK-native adapter route has deterministic coverage; SDK-backed `TestLiveOfficialSDKSmoke` passed on 2026-05-02 against serverVersion 203 with completedOrders rows=8; SDK-event fixture `official_sdk_completed_orders_snapshot_20260502.json` source sha256 `3e7d4b241f5b122c6802b13b788b367e4583eaa77b7bfd442fd462fc54d66696` freezes redacted `apiOnly=true` completed-order callbacks and `completedOrdersEnd` | promoted | `api_completed_orders_variants_aapl` was recaptured on 2026-04-15 (`6ac84daaf4084436`) after fixing the live TRAIL LIMIT completed-order decode path; remaining targets: paper-specific completed-order scenario variants and apiOnly=false SDK-event fixture |
| ORD-007 | Executions and commissions | `Orders().Executions`, official `reqExecutions`, `execDetails`, `execDetailsEnd`, `commissionAndFeesReport` | `executions_snapshot`, `executions.txt`, `executions_correlated.txt`, `executions_overlapping.txt`, `trading_split_round_trip_aapl`; SDK-native adapter route has deterministic coverage; `TestSDKExecutionsPublicRouteReplaysEmptyOfficialFixture` replays the public empty-filter path; SDK-backed `TestLiveOfficialSDKSmoke` passed on 2026-05-02 against serverVersion 203 with executions rows=0 and `execDetailsEnd` completion; SDK-event fixture `official_sdk_executions_empty_filter_20260502.json` source sha256 `6643d2468e2d9c3490272db3b7f7e98a89bb8bfea9042de004380d7904a344b0` freezes an impossible-symbol empty query completing with `execDetailsEnd` | blocked | filters by account/client/symbol/secType/exchange/side/time, commission before/after exec, sentinel values, SDK-event fixture with non-empty execution/commission evidence; blocked until a safe paper fill/commission-producing run is possible. 2026-04-15 aggressive pairs live run exposed and fixed the server_version=200 `lastNDays`/`specificDates` request fields |
| ORD-008 | Order handle lifecycle | `OrderHandle.Events`, `OrderHandle.Lifecycle`, `OrderHandle.Done`, `OrderHandle.Wait`, `OrderHandle.Close` | `api_reconnect_active_order_aapl`, `api_order_handle_reconnect_cancel_aapl.txt`, `api_transmit_false_then_transmit_aapl`, order replay and live tests; SDK-backed `TestLiveOfficialSDKPaperOrderPlaceCancel` passed on 2026-05-02 for handle event delivery and terminal close after `Cancelled` | candidate | handle detach without cancel, active-order reconnect/open-order recovery, slow consumer |
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
| OPT-001 | Option calculations | `Options().ImpliedVolatility`, `Options().Price`, official calculate/cancel calls | `calc_implied_volatility.txt`, `calc_option_price.txt`, live tests; SDK-event fixture `official_sdk_option_calculations_short_20260502.json` source sha256 `2a3994aae03394ee462285aba2a88c0d0547ae50813bde22c8a8d2ccc4245975` freezes invalid option security-definition errors for both calculation calls; SDK-event fixture `official_sdk_option_calculations_qualified_20260502.json` source sha256 `c51064689faf03226922a56aa00da6d464784b60408d7f688ab5e6b9778435e5` freezes a qualified AAPL 20260618 200C `contractDetails` callback plus successful `tickOptionComputation` callbacks for implied volatility and option price; public-route tests replay expected-error and success branches | promoted | unavailable data, cancel after first computation, additional underlyings/expiries |
| OPT-002 | Option exercise/lapse | `Options().Exercise`, official `exerciseOptions` | none; SDK-native adapter route and public command path have deterministic coverage, but paper-account live evidence is not attempted in the current SDK live smoke | blocked | exercise, lapse, override true/false, invalid option/account, paper account behavior; needs a paper option position or agreed invalid-position probe plus safe restore plan before sending `exerciseOptions` |
| OPT-003 | Option order and data integration | `Orders().Place`, market data/history for OPT | `place_order_option_buy`, `api_option_campaign_aapl`, `api_combo_option_vertical_aapl` | candidate | option quote, historical ticks if available, order fill/reject, completed/execution observation |

## News, Scanner, FA, WSH, Display, And TWS

| ID | Capability | Public API / Official Surface | Current Scenarios / Replay | Status | Required Matrix Variants |
|----|------------|-------------------------------|----------------------------|--------|--------------------------|
| NEWS-001 | News providers and bulletins | `News().Providers`, `News().SubscribeBulletins`, official provider/bulletin calls | `news_providers`, `news_bulletins`, `news_providers.txt`, `news_bulletins.txt`; SDK-native provider smoke passed on 2026-05-02 with newsProviders rows=8; deterministic public subscription coverage freezes news-bulletin request/cancel commands without inventing bulletin callback data; `news_bulletins_short` live probe timed out with no `updateNewsBulletin` callback | blocked | allMessages true/false, live bulletin callback if available, no entitlement |
| NEWS-002 | News article | `News().Article`, official `reqNewsArticle`, `newsArticle` | `api_news_article_aapl`, `news_article.txt`; SDK-event fixture `official_sdk_news_invalid_requests_20260502.json` source sha256 `fcb13d330984c46374c9afdd0c458a1f8e9eb5e97d244ba04c84cc5cc2401b9d` freezes invalid-provider code 321 without article text; SDK-event fixture `official_sdk_news_article_snapshot_20260502.json` source sha256 `e25f9cf9301804a7c6614efc580e1825d7487ab5b501b45242a67edde03567f6` freezes a live article callback with provider article text redacted | promoted | invalid article ID, provider-specific errors, provider variants |
| NEWS-003 | Historical news | `News().Historical`, official historical news callbacks | `historical_news_aapl`, `historical_news_aapl_timezone_window`, `api_news_article_aapl`; SDK-event fixtures `official_sdk_news_invalid_requests_20260502.json` and `official_sdk_news_article_snapshot_20260502.json` freeze invalid-provider and sanitized success paths | promoted | see HIST-006 |
| SCAN-001 | Scanner parameters | `Scanner().Parameters`, official `reqScannerParameters`, `scannerParameters` | `scanner_parameters`, `scanner_parameters.txt`; SDK-event fixture `official_sdk_scanner_parameters_snapshot_20260502.json` source sha256 `fe731ef34f63face09077861a31ba57709b2c14db9e587ac495f2ac9d5aa3974` freezes the full public scanner catalog XML | promoted | XML decode/size, unavailable scanner service |
| SCAN-002 | Scanner subscriptions | `Scanner().SubscribeResults`, official subscription/cancel callbacks | `scanner_subscription`, `scanner_subscription.txt`; SDK-event fixture `official_sdk_scanner_subscription_short_20260502.json` source sha256 `2b8a6aebfb035bf8ac911393d7dd91932c9c91e77843f61697ce8dfe1c41a1c8` freezes five `TOP_PERC_GAIN` `STK.US.MAJOR` rows; SDK-backed `TestLiveOfficialSDKReadOnlySubscriptions` passed on 2026-05-02 with public scanner rows=5 and clean close; later competing-session rerun emitted an empty row set and then code 165 with the different-IP message | promoted | legacy fields, filter TagValues, no-result scan, error variant |
| FA-001 | FA request config | `Advisors().Config`, official `requestFA`, `receiveFA` | `request_fa`; SDK-backed `TestLiveRequestFA` probe on 2026-05-02 timed out after 15s on the current non-FA paper account with no `receiveFA` callback or request-scoped API error | blocked | groups, profiles, aliases, FA account; non-FA error if Gateway emits one in another environment |
| FA-002 | FA replace config | `Advisors().ReplaceConfig`, official `replaceFA`, `replaceFAEnd` | SDK-native adapter route has deterministic coverage, but live FA read-back/restore is blocked without an FA-enabled account | blocked | non-FA error, read-back/restore if FA account exists, replaceFAEnd callback |
| FA-003 | Soft-dollar tiers | `Advisors().SoftDollarTiers`, official `reqSoftDollarTiers`, `softDollarTiers` | `soft_dollar_tiers`, `soft_dollar_tiers.txt` | promoted | empty and non-empty tier list |
| WSH-001 | WSH metadata | `WSH().MetaData`, official `reqWshMetaData`, `cancelWshMetaData`, `wshMetaData` | `wsh_meta_data`, `api_wsh_variants_aapl`, `wsh_meta_data_error.txt`; SDK-native smoke reached expected entitlement error code 10276 on 2026-05-02 against serverVersion 203 | candidate | success, entitlement error, cancel path |
| WSH-002 | WSH event data | `WSH().EventData`, official `reqWshEventData`, `cancelWshEventData`, `wshEventData` | `wsh_event_data_aapl`, `api_wsh_variants_aapl`; SDK-native smoke reached expected entitlement error code 10276 on 2026-05-02 against serverVersion 203 | candidate | conID, filter JSON, watchlist, portfolio, competitors, date windows, entitlement error |
| TWS-001 | User info | `TWS().UserInfo`, official `reqUserInfo`, `userInfo` | `user_info`, `user_info.txt` | promoted | TWS vs Gateway differences |
| TWS-002 | Display groups | `TWS().DisplayGroups`, `TWS().SubscribeDisplayGroup`, `DisplayGroupHandle.Update`, official display group calls | `display_groups`, `display_group_subscribe`, `display_groups.txt`, `display_group_subscribe.txt`; SDK-event fixture `official_sdk_display_group_subscription_short_20260502.json` source sha256 `26b548ce8c3bcb6e6d73c1677407b4f79a23b0a15ae442490da3d00c5b3b1a0e` freezes query + subscribe initial update; 2026-05-02 SDK fixture attempt for update to `265598@SMART` timed out waiting for a target callback and follow-up subscription still reported `none` | promoted | query, subscribe, unsubscribe frozen; update valid/invalid contract info and invalid group remain open |

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
- `Advisors().ReplaceConfig`: add non-FA error capture and FA read-back/restore
  scenario if an FA account is ever available.
- Official callback gaps from `ibkr-api-inventory.md`: `tickEFP`, `tickNews`,
  `orderBound`, and reroute/verify callbacks.

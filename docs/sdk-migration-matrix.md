# SDK Migration Matrix

This matrix tracks the migration from the old socket/codec runtime to the
official IBKR C++ SDK runtime. It is intentionally stricter than
`live-coverage-matrix.md`: old wire transcripts and codec tests do not count as
SDK-native command, event, native ABI, or SDK-event replay coverage.

Current audit date: 2026-05-02.

Baseline evidence from this audit:

- `git status --short --branch`: branch `spike/evaluate-ibkr-swig`, existing
  uncommitted SDK-migration work present.
- `GOCACHE=/tmp/go-cache-ibkr-go go test ./...`: pass.
- `go test ./internal/sdkadapter ./internal/sdkadapter/native ./... -run 'SDK|Official|Architecture|CurrentTime|AccountSummary|AccountUpdates|PnL|ContractDetails|Positions|MarketDataType|Option|SecDefOptParams|SmartComponents|FundamentalData|HeadTimestamp|Histogram|WSH|News|Scanner|FA|Order|Execution|Commission|Open|Completed|Cancel|Modify' -count=1`: pass; native package had no default-build tests.
- `go test -tags=ibkr_sdk ./... -count=1` with `.external/IBJts`: pass.
- `scripts/scan-ibkr-sdk-drift.sh .external/IBJts`: pass; adapter requests
  77, callbacks 72, command kinds 72.
- Sandboxed live SDK smoke against `127.0.0.1:4002` failed before server
  version with `SDK error 520: Failed to create socket`; `ss -ltnp` showed
  `java` listening on `*:4002`, so this is a workspace-sandbox socket blocker.
- The same read-only SDK smokes passed outside the workspace sandbox:
  `IBKR_LIVE=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9140 ... go test -tags=ibkr_sdk ./ -run "TestLiveOfficialSDK(Smoke|CurrentTimeMillis)$" -count=1 -v -timeout 180s`.
  Evidence: serverVersion 203, managedAccounts=1, nextValidID=1, account
  summary rows=4, contract details rows=1, head timestamp
  `1980-12-12T14:30:00Z`, histogram rows=1973, sec-def option params rows=39,
  matching symbols rows=17, market rule increments=1, quote-derived BBO
  exchange observed as `9c0001`, smart components rows=0, positions rows=6,
  family codes rows=1, depth exchanges rows=306, news providers rows=8,
  scanner parameters bytes=1719923, soft-dollar tiers rows=0, display groups
  rows=7, completed-order snapshot rows=11, execution snapshot rows=0, and WSH
  metadata/event data both returned expected entitlement error code 10276. The
  same combined run logged currentTime `2026-05-02T17:35:08Z` and
  fresh-session currentTimeMillis `2026-05-02T17:35:15.566Z`.
- `TestLiveOfficialSDKCurrentTimeMillis` also passed outside the workspace
  sandbox in a fresh session. Diagnostics against serverVersion 203 showed that
  the first current-time-style request on a session answers, while a second
  `CurrentTime` or `CurrentTimeMillis` request on the same session times out.
- Live-derived SDK-event fixtures are committed under
  `internal/sdkadapter/testdata/fixtures/`: `official_sdk_read_only_20260502.json`
  (source sha256 `8567dc6ede541fc441feafa4e072369a1ff1062281cd37d745ef9856f757673f`)
  `official_sdk_current_time_millis_20260502.json` (source sha256
  `c605a7d012b27733f22f7782d7b369a3dff98799a18f49080ee1805a25349b69`), and
  `official_sdk_account_summary_snapshot_20260502.json` (source sha256
  `64adc4661bdaeba4b6c40b3808159194a32cd302b65978a17d0a622cef2b7364`),
  `official_sdk_account_streams_snapshot_20260502.json` (source sha256
  `287bbbdbe96422f18b116f67bc1fcc09b34d017e1009784539f21aad420bd923`), and
  `official_sdk_family_codes_snapshot_20260502.json` (source sha256
  `af54ff7bd7a7cf603b9ca8774b9f42f99ca242ce3f0fa34db66f5177df44e3d1`).
  Account identifiers are redacted to `DU_REDACTED`; the account-summary
  fixture also replaces financial values with `REDACTED_VALUE`, and the
  account-stream fixture redacts account values, portfolio values, position
  quantities/costs, PnL values, model codes, and private position contracts.

## Status Vocabulary

| Status | Meaning |
|--------|---------|
| `done` | Public route, copied command/event schema, native C ABI, replay fixture, and live evidence are all present. |
| `partial` | Some public route or adapter shape exists, but at least one required SDK-native layer or evidence artifact is missing. |
| `blocked` | Implementation or verification requires unavailable external prerequisites. |
| `de-scoped` | Intentionally not part of the public SDK-backed surface. |

## Coverage Notes

- Command coverage means `internal/sdkadapter.CommandKind` plus
  `engine.sendSDKContext` conversion exist.
- Event coverage means `internal/sdkadapter.EventKind` plus native/event
  conversion exist. Legacy structs in `internal/sdkadapter/messages.go` are
  noted as `legacy record only` until they are part of the copied SDK event
  stream.
- Native C ABI coverage means `internal/sdkadapter/native/ibkr_adapter.h`,
  `native.go`, and `adapter.cpp` expose and convert the SDK request/callback.
- Replay fixture coverage means a committed SDK-event fixture using
  `internal/sdkadapter.Fixture`, not a legacy socket transcript.
- Live coverage means SDK-backed live verification. Existing socket-era live
  tracker entries remain useful history but do not close SDK migration rows.

## Session And Bootstrap

| Public facade/method | SDK EClient request/cancel method | SDK EWrapper callbacks/completion marker | Go public result/subscription/order-handle type | `sdkadapter.Command` coverage | `sdkadapter.Event` coverage | Native C ABI coverage | Replay fixture coverage | Read-only live coverage | Paper-trading live coverage | Status |
|---|---|---|---|---|---|---|---|---|---|---|
| `DialContext`, `Client.Close`, `Client.Done`, `Client.Wait`, `Client.Session`, `Client.SessionEvents` | `eConnect`, `startApi`, `eDisconnect`, bootstrap `reqManagedAccts`, `reqIds(1)` | `connectAck`, connection metadata, `managedAccounts`, `nextValidId`, `connectionClosed` | `*Client`, `Snapshot`, `Event` | connect is `Adapter.Connect`; bootstrap requests are native-side only | yes for metadata/accounts/next ID/closed | yes | `official_sdk_read_only_20260502.json`, `official_sdk_current_time_millis_20260502.json`; `TestSDKBootstrapFixtureUpdatesClientSession` replays SDK bootstrap callbacks through the public session snapshot/event path and `TestSDKClientCloseDoneWait` freezes public close/done/wait behavior | pass 2026-05-02 serverVersion 203: SDK smoke connected outside the workspace sandbox, reported managedAccounts=1 and nextValidID=1; sandboxed C++ socket connect fails with SDK error 520 | n/a | done |
| `Client.CurrentTime` | `reqCurrentTime` | `currentTime` singleton response | `time.Time` | yes | yes | yes | `official_sdk_read_only_20260502.json`; `TestSDKCurrentTimePublicRouteReplaysReadOnlyFixture` drives the public route from the captured `currentTime` callback and updates the session snapshot | pass 2026-05-02 serverVersion 203: SDK smoke logged `2026-05-02T17:35:08Z`; diagnostics show only the first current-time-style request per session answers | n/a | done |
| `Client.CurrentTimeMillis` | `reqCurrentTimeInMillis` | `currentTimeInMillis` singleton response | `time.Time` | yes | yes | yes | `official_sdk_current_time_millis_20260502.json`, source sha256 `c605a7d012b27733f22f7782d7b369a3dff98799a18f49080ee1805a25349b69`; fixture test asserts fresh-session bootstrap metadata and captured millis timestamp `2026-05-02T15:39:52.854Z`; `TestSDKCurrentTimeMillisPublicRouteReplaysOfficialFixture` drives the public route from that captured callback and updates the session snapshot | pass 2026-05-02 fresh-session serverVersion 203: logged `2026-05-02T17:35:15.566Z`; keep separate from `CurrentTime` on the same session | n/a | done |

## Accounts

| Public facade/method | SDK EClient request/cancel method | SDK EWrapper callbacks/completion marker | Go public result/subscription/order-handle type | `sdkadapter.Command` coverage | `sdkadapter.Event` coverage | Native C ABI coverage | Replay fixture coverage | Read-only live coverage | Paper-trading live coverage | Status |
|---|---|---|---|---|---|---|---|---|---|---|
| `Accounts().Summary` | `reqAccountSummary`, `cancelAccountSummary` | `accountSummary`, `accountSummaryEnd` | `[]AccountValue` | yes | yes | yes | `official_sdk_account_summary_snapshot_20260502.json`, source sha256 `64adc4661bdaeba4b6c40b3808159194a32cd302b65978a17d0a622cef2b7364`; fixture test asserts redacted `NetLiquidation` and `BuyingPower` callback values plus `accountSummaryEnd` | pass 2026-05-02 serverVersion 203: SDK smoke returned accountSummary rows=4; dedicated sanitized fixture capture passed with financial values replaced by `REDACTED_VALUE` | n/a | done |
| `Accounts().SubscribeSummary` | `reqAccountSummary`, `cancelAccountSummary` | `accountSummary`, `accountSummaryEnd` | `*Subscription[AccountSummaryUpdate]` | yes | yes | yes | redacted callback shape covered by `official_sdk_account_summary_snapshot_20260502.json` | pass 2026-05-02 serverVersion 203: `TestLiveOfficialSDKReadOnlySubscriptions` awaited the account-summary snapshot, observed rows=4, and closed the subscription through the public API | n/a | done |
| `Accounts().Positions` | `reqPositions`, `cancelPositions` | `position`, `positionEnd` | `[]Position` | yes | yes | yes | `official_sdk_account_streams_snapshot_20260502.json`, source sha256 `287bbbdbe96422f18b116f67bc1fcc09b34d017e1009784539f21aad420bd923`; fixture test asserts positions and `positionEnd` with account IDs, position quantities, average costs, and position contracts redacted | pass 2026-05-02 serverVersion 203: SDK smoke returned positions rows=6; deterministic public replay is intentionally not attempted from redacted private quantities/costs | n/a | done |
| `Accounts().SubscribePositions` | `reqPositions`, `cancelPositions` | `position`, `positionEnd` | `*Subscription[PositionUpdate]` | yes | yes | yes | redacted position callback shape covered by `official_sdk_account_streams_snapshot_20260502.json` | pass 2026-05-02 serverVersion 203: `TestLiveOfficialSDKReadOnlySubscriptions` awaited the positions snapshot, observed rows=6, and closed the subscription through the public API; streaming during trades remains a live-coverage variant | n/a | done |
| `Accounts().Updates`, `Accounts().SubscribeUpdates` | `reqAccountUpdates`, unsubscribe with same method | `updateAccountValue`, `updatePortfolio`, `updateAccountTime`, `accountDownloadEnd` | `[]AccountUpdate`, `*Subscription[AccountUpdate]` | yes, including unsubscribe | yes | yes | `official_sdk_account_streams_snapshot_20260502.json`, source sha256 `287bbbdbe96422f18b116f67bc1fcc09b34d017e1009784539f21aad420bd923`; fixture test asserts `updateAccountValue`, `updatePortfolio`, and `accountDownloadEnd` with account identifiers, values, portfolio quantities, PnL fields, and portfolio contracts redacted; `TestSDKAccountUpdatesPublicRouteReplaysAccountValueFixture` drives the public snapshot route from a redacted account-value callback and live-derived end marker, then freezes the public unsubscribe command | pass 2026-05-02 serverVersion 203: `TestLiveOfficialSDKAccountStreams` awaited the account-updates snapshot, observed rows=152, and closed the singleton subscription through the public API; the first attempt exposed default buffer overflow as `ErrResumeRequired`, fixed by raising default event/subscription buffers to 1024; deterministic portfolio replay is intentionally not attempted from redacted private values | n/a | done |
| `Accounts().UpdatesMulti`, `Accounts().SubscribeUpdatesMulti` | `reqAccountUpdatesMulti`, `cancelAccountUpdatesMulti` | `accountUpdateMulti`, `accountUpdateMultiEnd` | `[]AccountUpdateMultiValue`, `*Subscription[AccountUpdateMultiValue]` | yes, including cancel | yes | yes | `official_sdk_account_streams_snapshot_20260502.json`, source sha256 `287bbbdbe96422f18b116f67bc1fcc09b34d017e1009784539f21aad420bd923`; `TestSDKAccountUpdatesMultiPublicRouteReplaysOfficialFixture` drives the public snapshot route from a redacted live-derived `accountUpdateMulti` callback and end marker, then freezes the public cancel command | pass 2026-05-02 serverVersion 203: `TestLiveOfficialSDKAccountStreams` awaited account-updates-multi snapshot rows=75 for the live account and closed the subscription through the public API | n/a | done |
| `Accounts().PositionsMulti`, `Accounts().SubscribePositionsMulti` | `reqPositionsMulti`, `cancelPositionsMulti` | `positionMulti`, `positionMultiEnd` | `[]PositionMulti`, `*Subscription[PositionMulti]` | yes, including cancel | yes | yes | redacted `positionMulti` and `positionMultiEnd` callback shape covered by `official_sdk_account_streams_snapshot_20260502.json` | pass 2026-05-02 serverVersion 203: `TestLiveOfficialSDKAccountStreams` awaited positions-multi snapshot rows=6 for the live account and closed the subscription through the public API; deterministic public replay is intentionally not attempted from redacted private quantities/costs | n/a | done |
| `Accounts().SubscribePnL` | `reqPnL`, `cancelPnL` | `pnl` stream | `*Subscription[PnLUpdate]` | yes, including cancel | yes | yes | redacted `pnl` callback shape covered by `official_sdk_account_streams_snapshot_20260502.json`; `TestSDKPnLPublicSubscriptionsSendSDKCancel` freezes the public subscribe/cancel command lifecycle without replaying redacted private PnL values as decimals | pass 2026-05-02 serverVersion 203: `TestLiveOfficialSDKAccountStreams` observed a live account PnL update and closed the subscription through the public API | n/a | done |
| `Accounts().SubscribePnLSingle` | `reqPnLSingle`, `cancelPnLSingle` | `pnlSingle` stream | `*Subscription[PnLSingleUpdate]` | yes, including cancel | yes | yes | redacted `pnlSingle` callback shape covered by `official_sdk_account_streams_snapshot_20260502.json`; fixture recorder selects a conID from live positions without committing it; `TestSDKPnLSinglePublicSubscriptionSendsSDKCancel` freezes the public subscribe/cancel command lifecycle without replaying redacted private PnLSingle values as decimals | pass 2026-05-02 serverVersion 203: `TestLiveOfficialSDKAccountStreams` selected a conID from the live positions snapshot without logging it, observed a PnLSingle update, and closed the subscription through the public API | n/a | done |
| `Accounts().FamilyCodes` | `reqFamilyCodes` | `familyCodes` singleton response | `[]FamilyCode` | yes | yes | yes | `official_sdk_family_codes_snapshot_20260502.json`, source sha256 `af54ff7bd7a7cf603b9ca8774b9f42f99ca242ce3f0fa34db66f5177df44e3d1`; fixture test asserts the single-account callback with account ID redacted to `DU_REDACTED` | pass 2026-05-02 serverVersion 203: SDK smoke returned familyCodes rows=1; dedicated sanitized fixture capture passed | n/a | done |

## Contracts And Reference Data

| Public facade/method | SDK EClient request/cancel method | SDK EWrapper callbacks/completion marker | Go public result/subscription/order-handle type | `sdkadapter.Command` coverage | `sdkadapter.Event` coverage | Native C ABI coverage | Replay fixture coverage | Read-only live coverage | Paper-trading live coverage | Status |
|---|---|---|---|---|---|---|---|---|---|---|
| `Contracts().Details` | `reqContractDetails` | `contractDetails`, `bondContractDetails`, `contractDetailsEnd` | `[]ContractDetails` | yes | yes for `contractDetails` and `bondContractDetails` | yes | `official_sdk_read_only_20260502.json`; `official_sdk_bond_contract_details_snapshot_20260502.json`, source sha256 `4c01eb8b0be1532ae2b678f62ac731838f57c89c48b83a9c5cfbb0141932171d`; `TestSDKContractDetailsPublicRouteReplaysReadOnlyFixture` and `TestSDKBondContractDetailsPublicRouteReplaysOfficialFixture` drive public routes from live-derived callbacks and end markers | pass 2026-05-02 serverVersion 203: SDK smoke returned AAPL contractDetails rows=1; SDK fixture capture returned distinct IBM `bondContractDetails` callback for official sample CUSIP `449276AA2` | n/a | done |
| `Contracts().Qualify` | `reqContractDetails` | `contractDetails`, `contractDetailsEnd` | `ContractDetails` | yes | yes for `contractDetails` and `bondContractDetails` | yes | `official_sdk_read_only_20260502.json`; `TestSDKQualifyPublicRouteReplaysReadOnlyFixture` drives the public route from the live-derived AAPL callback and end marker | pass 2026-05-02 serverVersion 203: `TestLiveOfficialSDKSmoke` called `Contracts().Qualify` for AAPL, matched the contract-details conID, and logged conID=265598 | n/a | done |
| `Contracts().Search` | `reqMatchingSymbols` | `symbolSamples` | `[]MatchingSymbol` | yes | yes | yes | `official_sdk_read_only_20260502.json` | pass 2026-05-02 serverVersion 203: SDK smoke returned matchingSymbols rows=17 | n/a | done |
| `Contracts().MarketRule` | `reqMarketRule` | `marketRule` | `MarketRuleResult` | yes | yes | yes | `official_sdk_read_only_20260502.json` | pass 2026-05-02 serverVersion 203: SDK smoke returned marketRule increments=1 | n/a | done |
| `Contracts().SecDefOptParams` | `reqSecDefOptParams` | `securityDefinitionOptionParameter`, `securityDefinitionOptionParameterEnd` | `[]SecDefOptParams` | yes | yes | yes | `official_sdk_read_only_20260502.json` | pass 2026-05-02 serverVersion 203: SDK smoke returned secDefOptParams rows=39 | n/a | done |
| `Contracts().SmartComponents` | `reqSmartComponents` | `smartComponents` | `[]SmartComponent` | yes | yes | yes | `official_sdk_read_only_20260502.json` | pass 2026-05-02 serverVersion 203: SDK smoke used quote-derived BBO exchange `9c0001` and returned smartComponents rows=0 | n/a | done |
| `Contracts().DepthExchanges` | `reqMktDepthExchanges` | `mktDepthExchanges` | `[]DepthExchange` | yes | yes | yes | `official_sdk_read_only_20260502.json` | pass 2026-05-02 serverVersion 203: SDK smoke returned depthExchanges rows=306 | n/a | done |
| `Contracts().FundamentalData` | `reqFundamentalData`, `cancelFundamentalData` | `fundamentalData` | `XMLDocument` | yes, including cancel | yes | yes | `official_sdk_fundamental_data_snapshot_20260502.json`, source sha256 `5a011b88fe2fb979be15619edfe925d193a9a63f7a5ff3083d5ff15f8dd6a84e`; fixture test asserts AAPL `ReportSnapshot` XML including company, ticker, ratios, and forecast data; `TestSDKFundamentalDataPublicRouteReplaysOfficialFixture` drives the public route from the live-derived callback and `TestSDKFundamentalDataContextCancelSendsCancel` freezes context-cancel cleanup | pass 2026-05-02 serverVersion 203: `TestLiveOfficialSDKHistoryAndFundamental` returned AAPL `ReportSnapshot` bytes=11534; later competing-session rerun bounded the call to 30s and logged `context deadline exceeded` only after historical data had already returned competing-session blockers | n/a | done |

## Market Data

| Public facade/method | SDK EClient request/cancel method | SDK EWrapper callbacks/completion marker | Go public result/subscription/order-handle type | `sdkadapter.Command` coverage | `sdkadapter.Event` coverage | Native C ABI coverage | Replay fixture coverage | Read-only live coverage | Paper-trading live coverage | Status |
|---|---|---|---|---|---|---|---|---|---|---|
| `MarketData().SetType` | `reqMarketDataType` | `marketDataType` only after active market data | `error` | yes | yes for callback copying | yes | callback fixture in `official_sdk_read_only_20260502.json` | pass 2026-05-02 serverVersion 203: SDK smoke set delayed market-data type before quote request | n/a | done |
| `MarketData().Quote` | `reqMktData`, `cancelMktData` | tick callbacks, `tickSnapshotEnd` | `Quote` | yes, including cancel | yes for tick price/size/generic/string, tick req params, market data type, and snapshot end | yes | `official_sdk_read_only_20260502.json` | pass 2026-05-02 serverVersion 203: SDK smoke returned AAPL quote data including tickReqParams BBO exchange `9c0001`; later smoke rerun with a competing live TWS session logged exact environment blocker code 10197 (`No market data during competing live session`) and continued | n/a | done |
| `MarketData().SubscribeQuotes` | `reqMktData`, `cancelMktData` | tick callbacks, `marketDataType`, `tickReqParams` | `*Subscription[QuoteUpdate]` | yes, including cancel | yes for tick price/size/generic/string, tick req params, and market data type | yes | `official_sdk_quote_stream_short_20260502.json`, source sha256 `a514f2e78dc4c5fb66ca5b7e9b21a322ce2dcadf8d87ee7d5324d408d639e72e`; fixture test asserts delayed market-data type, delayed-data warning, tickReqParams, tick price/size/string, and no `tickSnapshotEnd` | pass 2026-05-02 serverVersion 203: SDK fixture capture and `TestLiveOfficialSDKReadOnlySubscriptions` both observed a delayed AAPL quote stream; public test logged `changed=2048` and `available=2048`, then closed the subscription. Later competing-session reruns logged exact quote-stream blocker code 10197 and ignored empty pre-terminal updates | n/a | done |
| `MarketData().SubscribeRealTimeBars` | `reqRealTimeBars`, `cancelRealTimeBars` | `realtimeBar` | `*Subscription[Bar]` | yes, including cancel | yes | yes | expected-error fixture in `official_sdk_real_time_bars_short_20260502.json`, source sha256 `baaa5d01dd79577b4c16d11632138bb3e751e811e2576950712daba7d3378b98`; fixture test asserts request-scoped code 420 with no bar callback | expected-error SDK fixture capture passed 2026-05-02 serverVersion 203: paper account lacks ISLAND real-time bar market-data permissions; success/cancel live evidence requires market-data entitlement | n/a | blocked |
| `MarketData().SubscribeTickByTick` | `reqTickByTickData`, `cancelTickByTickData` | `tickByTickAllLast`, `tickByTickBidAsk`, `tickByTickMidPoint` | `*Subscription[TickByTickData]` | yes, including cancel | yes | yes | expected-error fixture in `official_sdk_tick_by_tick_midpoint_short_20260502.json`, source sha256 `d43018da4703e8bda7cdac8f3eab51600ed8ba89cc676e46cab6c9398627b459`; fixture test asserts request-scoped code 10189 with no data callback | expected-error SDK fixture capture passed 2026-05-02 serverVersion 203: paper account lacks ISLAND tick-by-tick market-data permissions; success/cancel live evidence requires market-data entitlement | n/a | blocked |
| `MarketData().SubscribeDepth` | `reqMktDepth`, `cancelMktDepth` | `updateMktDepth`, `updateMktDepthL2` | `*Subscription[DepthRow]` | yes, including cancel | yes | yes | expected-error fixture in `official_sdk_market_depth_smart_short_20260502.json`, source sha256 `6850cc11ff16f4bb417b85c015de64a94a54dd2fff88025aaa3d9be2284fef4c`; fixture test asserts request-scoped code 2152 with no depth data callback | expected-error SDK fixture capture passed 2026-05-02 serverVersion 203: paper account lacks smart-depth market-data permissions; success/cancel live evidence requires market-depth entitlement | n/a | blocked |

## History

| Public facade/method | SDK EClient request/cancel method | SDK EWrapper callbacks/completion marker | Go public result/subscription/order-handle type | `sdkadapter.Command` coverage | `sdkadapter.Event` coverage | Native C ABI coverage | Replay fixture coverage | Read-only live coverage | Paper-trading live coverage | Status |
|---|---|---|---|---|---|---|---|---|---|---|
| `History().Bars` | `reqHistoricalData`, `cancelHistoricalData` | `historicalData`, `historicalDataEnd` | `[]Bar` | yes, including cancel | yes | yes | `official_sdk_historical_bars_short_20260502.json`, source sha256 `496348c4f40dfa0a64cd607d1921167ff344b20547bd406f7a353eda4f4bcc64`; fixture test asserts AAPL one-day one-hour RTH bars and `historicalDataEnd` | pass 2026-05-02 serverVersion 203: SDK fixture capture and `TestLiveOfficialSDKHistoryAndFundamental` both returned AAPL `1 D`/`1 hour`/`TRADES`/RTH bars; public test returned rows=7. Later competing-session reruns logged exact historical data blocker code 162 with the different-IP message | n/a | done |
| `History().SubscribeBars` | `reqHistoricalData` with `keepUpToDate`, `cancelHistoricalData` | `historicalData`, `historicalDataEnd`, `historicalDataUpdate` | `*Subscription[Bar]` | yes, including cancel | yes | yes | `official_sdk_historical_bars_keepup_short_20260502.json`, source sha256 `f6d87ab4c4a407bcd24e003647eebb87866a871411ad8bb06edc3769a15fbf32`; fixture test asserts the seven-row AAPL RTH initial snapshot and `historicalDataEnd` with no `historicalDataUpdate` on the Saturday capture; `TestSDKHistoricalBarsSubscriptionPublicRouteReplaysOfficialFixture` drives the public subscription snapshot route and cancel command from the fixture | pass 2026-05-02 serverVersion 203: `TestLiveOfficialSDKReadOnlySubscriptions` awaited the keep-up-to-date initial snapshot, observed rows=7, and closed the subscription; no `historicalDataUpdate` arrived because the run was on Saturday after market close. Later competing-session reruns logged exact stream blocker code 162 with the different-IP message. Remaining update/reconnect evidence is blocked until a market-hours session can produce `historicalDataUpdate` callbacks | n/a | blocked |
| `History().HeadTimestamp` | `reqHeadTimestamp`, `cancelHeadTimestamp` | `headTimestamp` | `time.Time` | yes, including cancel | yes | yes | `official_sdk_read_only_20260502.json` | pass 2026-05-02 serverVersion 203: SDK smoke returned AAPL headTimestamp `1980-12-12T14:30:00Z` | n/a | done |
| `History().Histogram` | `reqHistogramData`, `cancelHistogramData` | `histogramData` | `[]HistogramEntry` | yes, including cancel | yes | yes | `official_sdk_read_only_20260502.json` | pass 2026-05-02 serverVersion 203: SDK smoke returned histogram rows=1973; later smoke reruns with a competing live TWS session logged exact environment blocker code 10188 (`Trading TWS session is connected from a different IP address`) and continued | n/a | done |
| `History().Ticks` | `reqHistoricalTicks`, `cancelHistoricalTicks` | `historicalTicks`, `historicalTicksBidAsk`, `historicalTicksLast` | `HistoricalTicksResult` | yes, including cancel | yes | yes | `official_sdk_historical_ticks_midpoint_short_20260502.json`, source sha256 `6d047b0ba55fe9f6874a766a1b35bc4164a40fb1ef1138ebfb4a14470893b2ae`; `official_sdk_historical_ticks_bidask_short_20260502.json`, source sha256 `7a344933e7d5dd06221df36f9f3282b63bb2377c63149fd54899a19faf8ec3ec`; `official_sdk_historical_ticks_trades_short_20260502.json`, source sha256 `f1792cfa680c5d33a0e9834f616236085bfa862af84900f8546d1a545f1f7aa0`; fixture tests assert done midpoint, bid/ask, and trade ticks, and public-route tests replay the `BID_ASK` and `TRADES` branches from live-derived callbacks | pass 2026-05-02 serverVersion 203: SDK fixture capture and `TestLiveOfficialSDKHistoryAndFundamental` both returned AAPL midpoint ticks ending `2026-05-01 16:00:00 US/Eastern`; public test returned rows=172. Fresh fixture retries with client IDs 9172 and 9173 captured `TRADES` and `BID_ASK` callbacks after earlier attempts had timed out. | n/a | done |
| `History().Schedule` | `reqHistoricalData` with `whatToShow=SCHEDULE`, `cancelHistoricalData` | `historicalSchedule` | `HistoricalSchedule` | yes through historical-data command | yes | yes | `official_sdk_historical_schedule_short_20260502.json`, source sha256 `58271ad7ab173e061158f8a2f4b2fc36fd6a88514de744b8b9c179a00b1aa5d4`; fixture test asserts US/Eastern AAPL RTH schedule sessions | pass 2026-05-02 serverVersion 203: SDK fixture capture and `TestLiveOfficialSDKHistoryAndFundamental` both returned AAPL `1 M`/`1 day`/RTH schedule data; public test returned sessions=21 and timezone `US/Eastern` | n/a | done |

## Orders And Executions

| Public facade/method | SDK EClient request/cancel method | SDK EWrapper callbacks/completion marker | Go public result/subscription/order-handle type | `sdkadapter.Command` coverage | `sdkadapter.Event` coverage | Native C ABI coverage | Replay fixture coverage | Read-only live coverage | Paper-trading live coverage | Status |
|---|---|---|---|---|---|---|---|---|---|---|
| `Orders().Place` | `placeOrder` | `openOrder`, `orderStatus`, `execDetails`, `commissionAndFeesReport`, API errors | `*OrderHandle` | yes | yes | yes | `official_sdk_paper_order_place_cancel_20260502.json`, source sha256 `44f4dd370b903e85e74cfc11c9f1c464ad4d396890213dcc964632d12ebcf3e5`; fixture test asserts redacted BUY LMT `open_order` and warning code 399; `TestSDKPlaceOrderPublicRouteReplaysOfficialFixture` drives the public route and order-handle event delivery from the live-derived `openOrder` callback. `official_sdk_paper_order_reject_invalid_type_20260502.json`, source sha256 `4ae50ed4eedf69ec7f6cb9421ad8dea7c6353a2777697470f2c051cc076e2642`, freezes order-scoped code 321 invalid order type rejection | n/a | pass 2026-05-02 serverVersion 203: `TestLiveOfficialSDKPaperOrderPlaceCancel` placed a non-marketable 1-share AAPL LMT paper order and observed `openOrder` status `PreSubmitted`; invalid-order fixture captured the SDK rejection path without an `openOrder` callback; tests refuse non-`DU` accounts before order placement | done |
| `Orders().Cancel`, `OrderHandle.Cancel` | `cancelOrder` | `orderStatus`, API errors | `error`, handle lifecycle events | yes | yes for `orderStatus` | yes | `official_sdk_paper_order_place_cancel_20260502.json`, source sha256 `44f4dd370b903e85e74cfc11c9f1c464ad4d396890213dcc964632d12ebcf3e5`; fixture test asserts final `Cancelled` order status; `TestSDKOrdersCancelPublicRouteUsesSDKCommand` freezes the direct public cancel route and handle tests cover warning/late-error behavior | n/a | pass 2026-05-02 serverVersion 203: same SDK paper run sent SDK `cancelOrder` and observed final `Cancelled`; warning code 399 is frozen as non-terminal by `TestSDKOrderWarningAPIErrorsDoNotCloseHandle` | done |
| `Orders().CancelAll` | `reqGlobalCancel` | `orderStatus`, API errors | `error` | yes | yes for `orderStatus` | yes | no SDK-event fixture | n/a | blocked in current paper session: guarded `TestLiveOfficialSDKPaperGlobalCancel` skipped on 2026-05-02 because `Orders().Open(all)` found 1 non-test open order; test refuses to call global cancel unless the paper account has no unrelated open orders | blocked |
| `OrderHandle.Modify` | `placeOrder` with existing order ID | `openOrder`, `orderStatus`, API errors | `error`, handle lifecycle events | yes through `placeOrder` | yes | yes | `official_sdk_paper_order_modify_cancel_20260502.json`, source sha256 `860489438be38dc855edaafe2496c4071cea832857d44c4972bd540e4b127aec`; fixture test asserts quantity=1, quantity=2, and final `Cancelled`; `TestSDKOrderHandleModifyPublicRouteUsesSDKCommand` freezes the public handle modify command and replays the live-derived quantity-2 `openOrder` callback through the handle | n/a | pass 2026-05-02 serverVersion 203: `TestLiveOfficialSDKPaperOrderModifyCancel` observed paper order quantity modify from 1 to 2, then handle cancel to final `Cancelled` | done |
| `Orders().Open`, `Orders().SubscribeOpen` | `reqOpenOrders`, `reqAllOpenOrders`, `reqAutoOpenOrders` | `openOrder`, `openOrderEnd`, `orderStatus` for handle routes | `[]OpenOrder`, `*Subscription[OpenOrderUpdate]` | yes | yes | yes | `official_sdk_paper_open_orders_place_cancel_20260502.json`, source sha256 `d93f8f351aef13bc227ac0d33e6d51798bbbb728227e675b1c044e485ee73971`; fixture test asserts client-scope snapshot `openOrder`, `openOrderEnd`, and final `Cancelled` for the scenario paper order; `TestSDKOpenOrdersPublicRouteReplaysOfficialFixture` drives `Orders().Open(client)` from the live-derived snapshot and `TestSDKSubscribeOpenOrdersPublicRouteReplaysOfficialFixture` drives the public subscription snapshot path | not attempted in read-only SDK smoke | pass on 2026-05-02 serverVersion 203: `TestLiveOfficialSDKPaperOrderPlaceCancel` observed the active paper order via both `Orders().Open(all)` and `Orders().SubscribeOpen(all)` before cancel | done |
| `Orders().Completed` | `reqCompletedOrders` | `completedOrder`, `completedOrdersEnd` | `[]CompletedOrderResult` | yes | yes | yes | `official_sdk_completed_orders_snapshot_20260502.json`, source sha256 `3e7d4b241f5b122c6802b13b788b367e4583eaa77b7bfd442fd462fc54d66696`; fixture test asserts `apiOnly=true` completed-order callbacks and `completedOrdersEnd` with completed-order contracts and order fields redacted | pass 2026-05-02 serverVersion 203: `TestLiveOfficialSDKSmoke` returned completedOrders rows=11; dedicated SDK-event fixture capture passed with redacted callback fields; deterministic public replay is intentionally not attempted from redacted order action/type/status/quantity fields | paper-specific completed-order variants remain live-coverage targets | done |
| `Orders().Executions` | `reqExecutions` | `execDetails`, `execDetailsEnd`, `commissionAndFeesReport` | `[]ExecutionUpdate` | yes | yes | yes | empty-filter fixture in `official_sdk_executions_empty_filter_20260502.json`, source sha256 `6643d2468e2d9c3490272db3b7f7e98a89bb8bfea9042de004380d7904a344b0`; fixture test asserts `execDetailsEnd` with no execution details or commission reports; `TestSDKExecutionsPublicRouteReplaysEmptyOfficialFixture` drives the public route from the live-derived empty completion | pass 2026-05-02 serverVersion 203: `TestLiveOfficialSDKSmoke` completed with executions rows=0; dedicated empty-filter SDK fixture capture passed with reqID 1101 | blocked until a paper fill/commission-producing scenario can run safely during market hours or another instrument session that fills in paper; the current Saturday paper order smokes intentionally used non-marketable orders and produced no executions | blocked |

## Options

| Public facade/method | SDK EClient request/cancel method | SDK EWrapper callbacks/completion marker | Go public result/subscription/order-handle type | `sdkadapter.Command` coverage | `sdkadapter.Event` coverage | Native C ABI coverage | Replay fixture coverage | Read-only live coverage | Paper-trading live coverage | Status |
|---|---|---|---|---|---|---|---|---|---|---|
| `Options().ImpliedVolatility` | `calculateImpliedVolatility`, `cancelCalculateImpliedVolatility` | `tickOptionComputation` | `OptionComputation` | yes, including cancel | yes | yes | expected-error fixture in `official_sdk_option_calculations_short_20260502.json`, source sha256 `2a3994aae03394ee462285aba2a88c0d0547ae50813bde22c8a8d2ccc4245975`; success fixture in `official_sdk_option_calculations_qualified_20260502.json`, source sha256 `c51064689faf03226922a56aa00da6d464784b60408d7f688ab5e6b9778435e5`; fixture tests assert request-scoped code 200 for the invalid option, the qualified AAPL option `contractDetails` callback, and live-derived `tickOptionComputation`; public-route tests replay both the expected error and successful computation paths | pass 2026-05-02 serverVersion 203: qualified AAPL 20260618 200C capture returned `tickOptionComputation` with impliedVol `0.17100140275259834`, option price `5.25`, and underlying `200`; invalid option fixture still freezes the no-security-definition error path | n/a | done |
| `Options().Price` | `calculateOptionPrice`, `cancelCalculateOptionPrice` | `tickOptionComputation` | `OptionComputation` | yes, including cancel | yes | yes | expected-error fixture in `official_sdk_option_calculations_short_20260502.json`, source sha256 `2a3994aae03394ee462285aba2a88c0d0547ae50813bde22c8a8d2ccc4245975`; success fixture in `official_sdk_option_calculations_qualified_20260502.json`, source sha256 `c51064689faf03226922a56aa00da6d464784b60408d7f688ab5e6b9778435e5`; fixture tests assert request-scoped code 200 for the invalid option, the qualified AAPL option `contractDetails` callback, and live-derived `tickOptionComputation`; public-route tests replay both the expected error and successful computation paths | pass 2026-05-02 serverVersion 203: qualified AAPL 20260618 200C capture returned `tickOptionComputation` with impliedVol `0.29999999999999999`, option price `8.9158022449933707`, delta/gamma/vega/theta, and underlying `200`; invalid option fixture still freezes the no-security-definition error path | n/a | done |
| `Options().Exercise` | `exerciseOptions` | API errors only; no normal completion callback | `error` | yes | n/a beyond API errors | yes | no SDK-event fixture; `TestSDKExerciseOptionsPublicRouteUsesSDKCommand` freezes the public command path without contacting a paper account | n/a | blocked: exercise/lapse evidence requires an explicit paper option position or agreed invalid-position probe, plus a safe restore plan before sending the account-affecting `exerciseOptions` instruction | blocked |

## News, Scanner, Advisors, WSH, And TWS

| Public facade/method | SDK EClient request/cancel method | SDK EWrapper callbacks/completion marker | Go public result/subscription/order-handle type | `sdkadapter.Command` coverage | `sdkadapter.Event` coverage | Native C ABI coverage | Replay fixture coverage | Read-only live coverage | Paper-trading live coverage | Status |
|---|---|---|---|---|---|---|---|---|---|---|
| `News().Providers` | `reqNewsProviders` | `newsProviders` | `[]NewsProvider` | yes | yes | yes | `official_sdk_read_only_20260502.json` | pass 2026-05-02 serverVersion 203: SDK smoke returned newsProviders rows=8 | n/a | done |
| `News().Article` | `reqNewsArticle` | `newsArticle` | `NewsArticle` | yes | yes | yes | expected-error fixture in `official_sdk_news_invalid_requests_20260502.json`, source sha256 `fcb13d330984c46374c9afdd0c458a1f8e9eb5e97d244ba04c84cc5cc2401b9d`; success fixture in `official_sdk_news_article_snapshot_20260502.json`, source sha256 `e25f9cf9301804a7c6614efc580e1825d7487ab5b501b45242a67edde03567f6`; fixture tests assert request-scoped code 321 for an unsubscribed provider and a redacted live `newsArticle` callback | expected-error and sanitized success SDK fixture captures passed 2026-05-02 serverVersion 203; the success capture requested an article ID sourced from live AAPL historical news and redacted provider-owned text before commit | n/a | done |
| `News().Historical` | `reqHistoricalNews` | `historicalNews`, `historicalNewsEnd` | `[]HistoricalNewsItem` | yes | yes | yes | expected-error fixture in `official_sdk_news_invalid_requests_20260502.json`, source sha256 `fcb13d330984c46374c9afdd0c458a1f8e9eb5e97d244ba04c84cc5cc2401b9d`; success fixture in `official_sdk_news_article_snapshot_20260502.json`, source sha256 `e25f9cf9301804a7c6614efc580e1825d7487ab5b501b45242a67edde03567f6`; fixture tests assert five redacted `historicalNews` rows and `historicalNewsEnd(hasMore=true)` | expected-error and sanitized success SDK fixture captures passed 2026-05-02 serverVersion 203; `TestSDKHistoricalNewsPublicRouteReplaysOfficialFixture` replays the live-derived callback through the public route | n/a | done |
| `News().SubscribeBulletins` | `reqNewsBulletins`, `cancelNewsBulletins` | `updateNewsBulletin` | `*Subscription[NewsBulletin]` | yes, including cancel | yes | yes | no SDK-event fixture; `TestSDKNewsBulletinsPublicSubscriptionSendsSDKCancel` freezes the public subscribe/cancel command path without inventing bulletin callback data | blocked 2026-05-02 serverVersion 203: `news_bulletins_short` submitted `reqNewsBulletins(allMessages=true)` and timed out after 30s with `news bulletins: context deadline exceeded`; a callback fixture requires live/cached bulletin availability | n/a | blocked |
| `Scanner().Parameters` | `reqScannerParameters` | `scannerParameters` | `XMLDocument` | yes | yes | yes | `official_sdk_scanner_parameters_snapshot_20260502.json`, source sha256 `fe731ef34f63face09077861a31ba57709b2c14db9e587ac495f2ac9d5aa3974`; fixture test asserts full scanner catalog XML with US stock instrument, scan codes, and filter list | pass 2026-05-02 serverVersion 203: SDK smoke returned scannerParameters bytes=1719923; dedicated SDK fixture capture returned the same public scanner catalog family | n/a | done |
| `Scanner().SubscribeResults` | `reqScannerSubscription`, `cancelScannerSubscription` | `scannerData`, `scannerDataEnd` | `*Subscription[[]ScannerResult]` | yes, including cancel | yes; batches rows on `scannerDataEnd` | yes | `official_sdk_scanner_subscription_short_20260502.json`, source sha256 `2b8a6aebfb035bf8ac911393d7dd91932c9c91e77843f61697ce8dfe1c41a1c8`; fixture test asserts five `TOP_PERC_GAIN` `STK.US.MAJOR` SMART/USD stock rows | pass 2026-05-02 serverVersion 203: SDK fixture capture and `TestLiveOfficialSDKReadOnlySubscriptions` both observed five `TOP_PERC_GAIN` `STK.US.MAJOR` scanner rows; public test closed the subscription after the first batch. Later competing-session reruns observed an empty scanner batch followed by exact blocker code 165 with the different-IP message | n/a | done |
| `Advisors().Config` | `requestFA` | `receiveFA` | `XMLDocument` | yes | yes | yes | no SDK-event fixture | blocked in current paper session: `TestLiveRequestFA` on 2026-05-02 serverVersion 203 returned `context deadline exceeded` after 15s with no `receiveFA` callback or request-scoped API error; success evidence requires an FA-enabled account | n/a | blocked |
| `Advisors().ReplaceConfig` | `replaceFA` | `replaceFAEnd`, API errors | `error` | yes | yes | yes | no SDK-event fixture | n/a; write path intentionally not attempted without FA read-back/restore | blocked: replace/read-back evidence requires an FA-enabled account and an explicit safe restore plan | blocked |
| `Advisors().SoftDollarTiers` | `reqSoftDollarTiers` | `softDollarTiers` | `[]SoftDollarTier` | yes | yes | yes | `official_sdk_read_only_20260502.json` | pass 2026-05-02 serverVersion 203: SDK smoke returned softDollarTiers rows=0 | n/a | done |
| `WSH().MetaData` | `reqWshMetaData`, `cancelWshMetaData` | `wshMetaData` | `JSONDocument` | yes, including cancel | yes | yes | expected-error fixture in `official_sdk_read_only_20260502.json` | blocked expected-error path 2026-05-02 serverVersion 203: SDK smoke returned entitlement/account error code 10276 (`News feed is not allowed`) | n/a | blocked |
| `WSH().EventData` | `reqWshEventData`, `cancelWshEventData` | `wshEventData` | `JSONDocument` | yes, including cancel | yes | yes | expected-error fixture in `official_sdk_read_only_20260502.json` | blocked expected-error path 2026-05-02 serverVersion 203: SDK smoke returned entitlement/account error code 10276 (`News feed is not allowed`) | n/a | blocked |
| `TWS().UserInfo` | `reqUserInfo` | `userInfo` | `string` | yes | yes | yes | `official_sdk_read_only_20260502.json` | pass 2026-05-02 serverVersion 203: SDK smoke returned whiteBrandingID=`""` | n/a | done |
| `TWS().DisplayGroups` | `queryDisplayGroups` | `displayGroupList` | `[]DisplayGroupID` | yes | yes | yes | `official_sdk_read_only_20260502.json` | pass 2026-05-02 serverVersion 203: SDK smoke returned displayGroups rows=7 | n/a | done |
| `TWS().SubscribeDisplayGroup`, `DisplayGroupHandle.Update` | `subscribeToGroupEvents`, `updateDisplayGroup`, `unsubscribeFromGroupEvents` | `displayGroupUpdated`, API errors | `*DisplayGroupHandle` | yes, including update and unsubscribe | yes | yes | `official_sdk_display_group_subscription_short_20260502.json`, source sha256 `26b548ce8c3bcb6e6d73c1677407b4f79a23b0a15ae442490da3d00c5b3b1a0e`; fixture test asserts Gateway display group list `1|2|3|4|5|6|7` and initial `displayGroupUpdated` value `none` | SDK fixture capture passed 2026-05-02 serverVersion 203 for subscribe/unsubscribe to group 1; `updateDisplayGroup` fixture attempt with contract info `265598@SMART` timed out after 90s waiting for a target `displayGroupUpdated` callback and a follow-up subscription probe still reported `none`; update callback and invalid group variants remain blocked in the current Gateway state | n/a | blocked |

## Immediate Migration Gaps

1. All advertised facade rows now have SDK command/event/native coverage or a
   documented external blocker. Completed rows include the redacted
   account-summary, account-stream, family-codes, qualified-option,
   historical/news, scanner, order, and public metadata/reference fixtures plus
   the public route replay tests that drive those callbacks through the SDK
   runtime.
2. The native adapter currently covers bootstrap/session metadata, current
   time, current time millis, account summary, account updates, account updates
   multi, positions, positions multi, PnL, PnL single, family codes, contract
   details, contract search, market rules, sec-def option params, smart
   components, market-depth exchange metadata, historical bars, historical
   schedule, historical ticks, head timestamp, histogram data, fundamental data,
   market-data type control, option implied-volatility and price calculations,
   news providers, news bulletins, news articles, historical news, scanner
   parameters, scanner result subscriptions, FA config reads/writes,
   soft-dollar tiers, WSH metadata/event data, user info, display groups,
   display group subscriptions, execution snapshots, commission reports,
   order placement, order modification, open-order observation, completed-order
   snapshots, order cancellation, global cancellation, and
   cancellation for account summary, account updates, account updates multi,
   positions, positions multi, PnL, PnL single, news bulletins, scanner subscriptions,
   display group subscriptions, historical bars, historical ticks, option
   calculations, head timestamp, histogram data, WSH metadata/event data, and
   fundamental data.
3. Advertised facade command/event/native coverage is closed for the current
   public API. Remaining non-`done` rows are `blocked` by external state:
   market-hours update callbacks, market-data entitlements, FA account type,
   WSH/news entitlement, paper-account safety preconditions, or live bulletin
   availability.
4. `internal/codec`, `internal/wire`, `internal/transport`, `testing/testhost`,
   and transcript-driven tests are quarantined behind the non-default
   `legacy_native_socket` build tag and remain documented as legacy replay
   tooling. They may be useful source evidence, but they are not the production
   SDK runtime and do not close SDK-native fixture or live-evidence rows.
5. Committed live-derived SDK-event fixture coverage is intentionally narrow:
   `official_sdk_read_only_20260502.json` avoids account financial values and
   order writes, while deterministic fixture tests assert its public
   contract/search/quote/head-timestamp/histogram/market-rule/sec-def option
   parameter/smart-component/depth-exchange/news-provider/soft-dollar-tier,
   user-info, display-group, and WSH entitlement callback shapes; the contract
   route tests also replay the public `Contracts().Details` and
   `Contracts().Qualify` facades from this fixture.
   `official_sdk_current_time_millis_20260502.json` exists separately because
   serverVersion 203 only answered the first
   current-time-style request per session.
   `official_sdk_account_summary_snapshot_20260502.json` freezes real
   `NetLiquidation` and `BuyingPower` account-summary callback shape with
   source sha256
   `64adc4661bdaeba4b6c40b3808159194a32cd302b65978a17d0a622cef2b7364`, while
   replacing financial values with `REDACTED_VALUE`.
   `official_sdk_account_streams_snapshot_20260502.json` freezes account
   updates, account updates multi, positions, positions multi, PnL, and
   PnLSingle callback shapes with source sha256
   `287bbbdbe96422f18b116f67bc1fcc09b34d017e1009784539f21aad420bd923`, while
   redacting account values, portfolio values, position quantities/costs, PnL
   values, model codes, and private position contracts.
   `official_sdk_family_codes_snapshot_20260502.json` freezes the single-account
   `familyCodes` callback with source sha256
   `af54ff7bd7a7cf603b9ca8774b9f42f99ca242ce3f0fa34db66f5177df44e3d1`.
   `official_sdk_bond_contract_details_snapshot_20260502.json` freezes the
   distinct `bondContractDetails` callback for the official SDK sample CUSIP
   `449276AA2` with source sha256
   `4c01eb8b0be1532ae2b678f62ac731838f57c89c48b83a9c5cfbb0141932171d`; the
   official sample conID `456467716` timed out in the current Gateway session.
   Paper-order
   fixtures are scoped to conservative non-marketable paper orders and sanitize
   account identifiers and permIDs. `official_sdk_completed_orders_snapshot_20260502.json`
   freezes the `apiOnly=true` completed-order callback shape with source sha256
   `3e7d4b241f5b122c6802b13b788b367e4583eaa77b7bfd442fd462fc54d66696`, while
   redacting completed-order contracts and order fields.
   `official_sdk_quote_stream_short_20260502.json` freezes a short
   delayed AAPL quote stream with source sha256
   `a514f2e78dc4c5fb66ca5b7e9b21a322ce2dcadf8d87ee7d5324d408d639e72e`.
   `official_sdk_real_time_bars_short_20260502.json` freezes the current
   paper-account real-time bars entitlement error with source sha256
   `baaa5d01dd79577b4c16d11632138bb3e751e811e2576950712daba7d3378b98`.
   `official_sdk_tick_by_tick_midpoint_short_20260502.json` freezes the
   current paper-account tick-by-tick entitlement error with source sha256
   `d43018da4703e8bda7cdac8f3eab51600ed8ba89cc676e46cab6c9398627b459`.
   `official_sdk_market_depth_smart_short_20260502.json` freezes the current
   paper-account smart-depth entitlement error with source sha256
   `6850cc11ff16f4bb417b85c015de64a94a54dd2fff88025aaa3d9be2284fef4c`.
   `official_sdk_historical_bars_short_20260502.json` freezes a short AAPL
   historical-bars response with source sha256
   `496348c4f40dfa0a64cd607d1921167ff344b20547bd406f7a353eda4f4bcc64`.
   `official_sdk_historical_bars_keepup_short_20260502.json` freezes the
   keep-up-to-date historical-bars initial snapshot with source sha256
   `f6d87ab4c4a407bcd24e003647eebb87866a871411ad8bb06edc3769a15fbf32`; the
   Saturday capture has no `historicalDataUpdate` callback.
   `official_sdk_historical_schedule_short_20260502.json` freezes an AAPL
   historical schedule response with source sha256
   `58271ad7ab173e061158f8a2f4b2fc36fd6a88514de744b8b9c179a00b1aa5d4`.
   `official_sdk_historical_ticks_midpoint_short_20260502.json` freezes AAPL
   historical midpoint ticks with source sha256
   `6d047b0ba55fe9f6874a766a1b35bc4164a40fb1ef1138ebfb4a14470893b2ae`;
   `official_sdk_historical_ticks_bidask_short_20260502.json` freezes AAPL
   historical bid/ask ticks with source sha256
   `7a344933e7d5dd06221df36f9f3282b63bb2377c63149fd54899a19faf8ec3ec`;
   `official_sdk_historical_ticks_trades_short_20260502.json` freezes AAPL
   historical trade ticks with source sha256
   `f1792cfa680c5d33a0e9834f616236085bfa862af84900f8546d1a545f1f7aa0`.
   `official_sdk_fundamental_data_snapshot_20260502.json` freezes AAPL
   `ReportSnapshot` XML with source sha256
   `5a011b88fe2fb979be15619edfe925d193a9a63f7a5ff3083d5ff15f8dd6a84e`.
   `official_sdk_scanner_parameters_snapshot_20260502.json` freezes the public
   scanner catalog XML with source sha256
   `fe731ef34f63face09077861a31ba57709b2c14db9e587ac495f2ac9d5aa3974`.
   `official_sdk_scanner_subscription_short_20260502.json` freezes a short
   `TOP_PERC_GAIN` `STK.US.MAJOR` scanner subscription with source sha256
   `2b8a6aebfb035bf8ac911393d7dd91932c9c91e77843f61697ce8dfe1c41a1c8`.
   `official_sdk_display_group_subscription_short_20260502.json` freezes a
   Gateway display-group subscribe/unsubscribe callback with source sha256
   `26b548ce8c3bcb6e6d73c1677407b4f79a23b0a15ae442490da3d00c5b3b1a0e`.
   A display-group update fixture attempt against the same Gateway timed out
   waiting for a target `displayGroupUpdated` callback after submitting
   `updateDisplayGroup` for `265598@SMART`; a follow-up subscription probe still
   reported `none`, so no update fixture was promoted.
   `official_sdk_news_invalid_requests_20260502.json` freezes invalid news
   provider errors without committing provider article text, with source sha256
   `fcb13d330984c46374c9afdd0c458a1f8e9eb5e97d244ba04c84cc5cc2401b9d`;
   `official_sdk_news_article_snapshot_20260502.json` freezes sanitized
   historical-news and article success callbacks with source sha256
   `e25f9cf9301804a7c6614efc580e1825d7487ab5b501b45242a67edde03567f6`.
   `official_sdk_option_calculations_short_20260502.json` freezes invalid
   option-contract calculation errors with source sha256
   `2a3994aae03394ee462285aba2a88c0d0547ae50813bde22c8a8d2ccc4245975`;
   `official_sdk_option_calculations_qualified_20260502.json` freezes a
   qualified AAPL 20260618 200C contract-details callback and successful
   option-computation callbacks with source sha256
   `c51064689faf03226922a56aa00da6d464784b60408d7f688ab5e6b9778435e5`.
   `official_sdk_executions_empty_filter_20260502.json` freezes an impossible
   symbol executions query completing with only `execDetailsEnd`, source sha256
   `6643d2468e2d9c3490272db3b7f7e98a89bb8bfea9042de004380d7904a344b0`.
6. SDK-backed paper-order live and fixture evidence is now present for the
   basic place/handle-cancel path, quantity modify/cancel, and client-scope
   open-orders snapshot callbacks. `TestLiveOfficialSDKPaperOrderPlaceCancel`
   passed on 2026-05-02 against paper Gateway serverVersion 203, after an
   initial run exposed warning code 399 as non-terminal order information.
   `official_sdk_paper_order_place_cancel_20260502.json` freezes that callback
   sequence with source sha256
   `44f4dd370b903e85e74cfc11c9f1c464ad4d396890213dcc964632d12ebcf3e5`;
   `official_sdk_paper_open_orders_place_cancel_20260502.json` freezes the
   client-scope open-orders snapshot sequence with source sha256
   `d93f8f351aef13bc227ac0d33e6d51798bbbb728227e675b1c044e485ee73971`.
   `official_sdk_paper_order_reject_invalid_type_20260502.json` freezes the
   SDK invalid-order-type rejection path with source sha256
   `4ae50ed4eedf69ec7f6cb9421ad8dea7c6353a2777697470f2c051cc076e2642`.
   Global cancel is blocked in the current paper session by one unrelated
   open order; the guarded live test refuses to call `reqGlobalCancel` in that
   state. Paper-specific completed-order variants, non-empty
   executions/commissions, and broader SDK-event order variants remain blocked
   until a safe paper fill/commission-producing session is available.
   `Orders().Open(all)` and `Orders().SubscribeOpen(all)` have SDK-backed paper
   live evidence; the committed fixture currently covers client-scope SDK
   open-order callbacks.

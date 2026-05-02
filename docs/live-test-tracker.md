# Live Test Execution Tracker

Companion to [`live-coverage-matrix.md`](live-coverage-matrix.md). Tracks every
live test run against IB Gateway paper account, what passed, what failed, what
was fixed, and what remains untested.

Last updated: 2026-05-02.

## SDK-Native Migration Runs

| Date | Scope | Environment | Result | Evidence |
|------|-------|-------------|--------|----------|
| 2026-05-02 | Sandboxed `TestLiveOfficialSDKSmoke` connect check | `ss -ltnp` showed `java` listening on `*:4002`; command ran inside the workspace sandbox | blocked | `IBKR_LIVE=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9001 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./ -run TestLiveOfficialSDKSmoke -count=1 -v -timeout 60s` failed immediately with `DialContext() error = ibkr: connect sdk_connect: connect: official SDK eConnect returned false; SDK error 520: Failed to create socket` |
| 2026-05-02 | `TestLiveOfficialSDKSmoke` with session bootstrap, current time, market-data type control, account summary, contract details, head timestamp and histogram for AAPL, sec-def option params for AAPL, matching symbols, market rule 26, quote snapshot for AAPL, smart components using quote-derived BBO exchange, positions, family codes, market-depth exchanges, news providers, scanner parameters, soft-dollar tiers, user info, display groups, and WSH metadata/event data | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | pass | `IBKR_LIVE=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9009 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./ -run TestLiveOfficialSDKSmoke -count=1 -v -timeout 180s` passed in 6.17s. Evidence highlights: managedAccounts=1, currentTime received, delayed market-data type accepted, accountSummary rows=4, contractDetails rows=1, headTimestamp `1980-12-12T14:30:00Z`, histogram rows=1973, secDefOptParams rows=39, matchingSymbols rows=17, marketRule increments=1, quote-derived BBO exchange observed as `9c0001`, smartComponents rows=0, positions rows=6, familyCodes rows=1, depthExchanges rows=306, newsProviders rows=8, scannerParameters bytes=1719923, softDollarTiers rows=0, displayGroups rows=7, WSH metadata/event data both returned expected entitlement error code 10276 |
| 2026-05-02 | `TestLiveOfficialSDKCurrentTimeMillis` fresh-session smoke | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | pass | `IBKR_LIVE=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9002 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./ -run TestLiveOfficialSDKCurrentTimeMillis -count=1 -v -timeout 60s` passed in 0.06s and logged `official SDK currentTimeMillis received: 2026-05-02T15:09:24.236Z` |
| 2026-05-02 | Combined rerun of `TestLiveOfficialSDKSmoke` and `TestLiveOfficialSDKCurrentTimeMillis` | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | pass | `IBKR_LIVE=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9010 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./ -run "TestLiveOfficialSDK(Smoke\|CurrentTimeMillis)$" -count=1 -v -timeout 180s` passed in 6.86s. Smoke logged currentTime `2026-05-02T15:27:55Z`; fresh-session currentTimeMillis logged `2026-05-02T15:28:01.92Z` |
| 2026-05-02 | Combined rerun of `TestLiveOfficialSDKSmoke` and `TestLiveOfficialSDKCurrentTimeMillis` after fixture/privacy guard hardening | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | pass | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && IBKR_LIVE=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9140 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./ -run "TestLiveOfficialSDK(Smoke\|CurrentTimeMillis)$" -count=1 -v -timeout 180s'` passed in 6.812s. Smoke logged currentTime `2026-05-02T17:35:08Z`, completedOrders rows=11, executions rows=0, WSH metadata/event data expected code 10276; fresh-session currentTimeMillis logged `2026-05-02T17:35:15.566Z` |
| 2026-05-02 | Initial public SDK read-only expansion with slower history/fundamental calls inside `TestLiveOfficialSDKSmoke` | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | fail, fixed | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && IBKR_LIVE=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9143 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./... -run "TestLiveOfficialSDK(Smoke\|CurrentTimeMillis\|ReadOnlySubscriptions)$" -count=1 -v -timeout 240s'` failed in 49.176s because the expanded shared smoke context expired before `SecDefOptParams`. Before the timeout, it had logged historical bars rows=7, schedule sessions=21, historical midpoint ticks rows=172, and fundamentalData bytes=11534; `TestLiveOfficialSDKReadOnlySubscriptions` passed with account-summary rows=4, quote stream update, keep-up historical bars snapshot rows=7, and scanner rows=5. The slower one-shot history/fundamental checks were split into `TestLiveOfficialSDKHistoryAndFundamental`. |
| 2026-05-02 | Combined public SDK read-only rerun: smoke, current time millis, history/fundamental, and read-only subscriptions | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | pass | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && IBKR_LIVE=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9144 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./... -run "TestLiveOfficialSDK(Smoke\|CurrentTimeMillis\|HistoryAndFundamental\|ReadOnlySubscriptions)$" -count=1 -v -timeout 300s'` passed in 13.669s. Smoke logged currentTime `2026-05-02T17:59:55Z`, completedOrders rows=13, executions rows=0, and WSH expected code 10276; fresh-session currentTimeMillis logged `2026-05-02T18:00:01.495Z`. `HistoryAndFundamental` logged bars rows=7, schedule sessions=21 timezone `US/Eastern`, midpoint ticks rows=172, and fundamentalData bytes=11534. `ReadOnlySubscriptions` logged account-summary rows=4, quote stream `changed=2048 available=2048`, keep-up historical bars initial snapshot rows=7, and scanner rows=5. |
| 2026-05-02 | Focused public SDK read-only subscription rerun with positions subscription added | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | pass | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && IBKR_LIVE=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9146 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./... -run "TestLiveOfficialSDKReadOnlySubscriptions$" -count=1 -v -timeout 120s'` passed in 2.097s. The test logged account-summary subscription rows=4, positions subscription rows=6, delayed AAPL quote stream `changed=2048 available=2048`, keep-up historical bars initial snapshot rows=7, and scanner rows=5; all subscriptions closed through the public API. |
| 2026-05-02 | Initial public SDK account stream probe with default 64-event buffers | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | fail, fixed | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && IBKR_LIVE=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9147 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./... -run "TestLiveOfficialSDKAccountStreams$" -count=1 -v -timeout 150s'` failed in 0.318s with `Accounts().SubscribeUpdates().AwaitSnapshot() error = ibkr: subscription resume required`. The burst account-update snapshot overflowed the previous default native/subscription buffers before `AwaitSnapshot` could complete; defaults were raised from 64 to 1024. |
| 2026-05-02 | Public SDK account stream rerun after buffer increase | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | pass | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && IBKR_LIVE=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9148 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./... -run "TestLiveOfficialSDKAccountStreams$" -count=1 -v -timeout 150s'` passed in 1.577s. Evidence: account updates snapshot rows=152, account updates multi rows=75, positions multi rows=6, PnL emitted one update, and PnLSingle emitted one update. The test logs no account IDs, holdings, conIDs, or financial values. |
| 2026-05-02 | SDK-event account-stream fixture capture | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9166 -scenario account_streams_snapshot -out internal/sdkadapter/testdata/fixtures/official_sdk_account_streams_snapshot_20260502.json -timeout 120s'` recorded a sanitized fixture with source sha256 `287bbbdbe96422f18b116f67bc1fcc09b34d017e1009784539f21aad420bd923`; account identifiers are redacted to `DU_REDACTED`; account values, portfolio values, position quantities/costs, PnL values, model codes, and private position contracts are replaced with redaction placeholders. |
| 2026-05-02 | SDK-event fixture attempt for historical `BID_ASK` ticks | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | blocked | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9149 -scenario historical_ticks_bidask_short -out internal/sdkadapter/testdata/fixtures/official_sdk_historical_ticks_bidask_short_20260502.json -timeout 90s'` failed with `historical bid/ask ticks: context deadline exceeded`; no fixture file was written. |
| 2026-05-02 | SDK-event fixture attempt for historical `TRADES` ticks | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | blocked | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9150 -scenario historical_ticks_trades_short -out internal/sdkadapter/testdata/fixtures/official_sdk_historical_ticks_trades_short_20260502.json -timeout 90s'` failed with `historical trade ticks: context deadline exceeded`; no fixture file was written. |
| 2026-05-02 | SDK-event fixture retry for historical `TRADES` ticks | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && GOCACHE=/tmp/go-cache-ibkr-go go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9172 -scenario historical_ticks_trades_short -out internal/sdkadapter/testdata/fixtures/official_sdk_historical_ticks_trades_short_20260502.json -timeout 90s'` recorded a public-market fixture with source sha256 `f1792cfa680c5d33a0e9834f616236085bfa862af84900f8546d1a545f1f7aa0`; deterministic tests assert a done `historicalTicksLast` callback and replay the public `History().Ticks(TRADES)` branch. |
| 2026-05-02 | SDK-event fixture retry for historical `BID_ASK` ticks | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && GOCACHE=/tmp/go-cache-ibkr-go go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9173 -scenario historical_ticks_bidask_short -out internal/sdkadapter/testdata/fixtures/official_sdk_historical_ticks_bidask_short_20260502.json -timeout 90s'` recorded a public-market fixture with source sha256 `7a344933e7d5dd06221df36f9f3282b63bb2377c63149fd54899a19faf8ec3ec`; deterministic tests assert a done `historicalTicksBidAsk` callback and replay the public `History().Ticks(BID_ASK)` branch. |
| 2026-05-02 | `TestLiveOfficialSDKSmoke` reruns after adding `Contracts().Qualify` evidence | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203, current session has a competing live TWS connection | fail, fixed | Client IDs 9151 and 9152 both ran `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && IBKR_LIVE=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=<id> GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./... -run "TestLiveOfficialSDKSmoke$" -count=1 -v -timeout 180s'` and failed at `History().Histogram` with API code 10188 (`Trading TWS session is connected from a different IP address`). Both runs had already logged `Contracts().Qualify` conID=265598. |
| 2026-05-02 | `TestLiveOfficialSDKSmoke` rerun after tolerating histogram competing-session code | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203, current session has a competing live TWS connection | fail, fixed | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && IBKR_LIVE=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9153 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./... -run "TestLiveOfficialSDKSmoke$" -count=1 -v -timeout 180s'` logged histogram code 10188 as an environment blocker, then failed at the AAPL quote request with API code 10197 (`No market data during competing live session`). |
| 2026-05-02 | `TestLiveOfficialSDKSmoke` rerun with exact competing-session tolerances | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203, current session has a competing live TWS connection | pass | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && IBKR_LIVE=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9154 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./... -run "TestLiveOfficialSDKSmoke$" -count=1 -v -timeout 180s'` passed in 6.777s. Evidence highlights: `Contracts().Qualify` conID=265598, histogram blocked by code 10188, quote/smart-components blocked by code 10197, positions rows=6, familyCodes rows=1, depthExchanges rows=306, newsProviders rows=8, scannerParameters bytes=1719923, displayGroups rows=7, completedOrders rows=13, executions rows=0, and WSH expected code 10276. |
| 2026-05-02 | `TestLiveOfficialSDKReadOnlySubscriptions` competing-session hardening | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203, current session has a competing live TWS connection | fail, fixed | Client IDs 9155 through 9160 exposed the current competing-session shapes: quote stream code 10197 after an empty update, historical keep-up bars code 162, scanner empty batch, and scanner close code 165. The test now treats only those exact competing-session codes/messages as environment blockers while preserving the earlier non-empty quote/history/scanner evidence. |
| 2026-05-02 | `TestLiveOfficialSDKReadOnlySubscriptions` rerun with exact competing-session handling | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203, current session has a competing live TWS connection | pass | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && IBKR_LIVE=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9161 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./... -run "TestLiveOfficialSDKReadOnlySubscriptions$" -count=1 -v -timeout 120s'` passed in 6.027s. Evidence: account-summary subscription rows=4, positions subscription rows=6, quote stream blocked by code 10197, historical keep-up bars blocked by code 162, scanner emitted an empty row set and then closed with code 165. |
| 2026-05-02 | `TestLiveOfficialSDKHistoryAndFundamental` competing-session hardening | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203, current session has a competing live TWS connection | fail, fixed | Client ID 9162 logged historical bars code 162 and schedule sessions=21, then failed on historical ticks code 10187. Client ID 9163 handled code 10187 but then timed out on fundamental data after 90s. The test now bounds the fundamental data request to 30s and accepts that timeout only after a competing-session historical data blocker has already been observed. |
| 2026-05-02 | `TestLiveOfficialSDKHistoryAndFundamental` rerun with exact competing-session handling | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203, current session has a competing live TWS connection | pass | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && IBKR_LIVE=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9164 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./... -run "TestLiveOfficialSDKHistoryAndFundamental$" -count=1 -v -timeout 120s'` passed in 32.377s. Evidence: historical bars blocked by code 162, historical schedule still returned sessions=21 timezone `US/Eastern`, historical midpoint ticks blocked by code 10187, and fundamental data timed out after the competing-session historical blockers. |
| 2026-05-02 | Combined read-only SDK live suite under competing-session state | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203, current session has a competing live TWS connection | pass | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && IBKR_LIVE=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9165 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./... -run "TestLiveOfficialSDK(Smoke\|CurrentTimeMillis\|HistoryAndFundamental\|ReadOnlySubscriptions\|AccountStreams)$" -count=1 -v -timeout 240s'` passed in 50.915s. Evidence: smoke logged `Contracts().Qualify` conID=265598, histogram code 10188, quote/smart-components code 10197, positions rows=6, familyCodes rows=1, depthExchanges rows=306, newsProviders rows=8, scannerParameters bytes=1719923, displayGroups rows=7, completedOrders rows=13, executions rows=0, and WSH expected code 10276; currentTimeMillis logged `2026-05-02T18:38:09.421Z`; history logged bars code 162, schedule sessions=21 timezone `US/Eastern`, ticks code 10187, and fundamental timeout after blockers; subscriptions logged account-summary rows=4, positions rows=6, quote code 10197, historical bars code 162, scanner empty then code 165; account streams logged updates rows=152, updates multi rows=75, positions multi rows=6, PnL update, and PnLSingle update. |
| 2026-05-02 | SDK-event fixture attempt for official sample bond conID contract details | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | blocked | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && GOCACHE=/tmp/go-cache-ibkr-go go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9169 -scenario bond_contract_details_snapshot -out internal/sdkadapter/testdata/fixtures/official_sdk_bond_contract_details_snapshot_20260502.json -timeout 90s'` used the official C++ sample bond conID `456467716` and failed with `bond contract details: context deadline exceeded`; no fixture file was written. |
| 2026-05-02 | SDK-event fixture capture for official sample bond CUSIP contract details | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && GOCACHE=/tmp/go-cache-ibkr-go go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9170 -scenario bond_contract_details_snapshot -out internal/sdkadapter/testdata/fixtures/official_sdk_bond_contract_details_snapshot_20260502.json -timeout 90s'` recorded a sanitized fixture with source sha256 `4c01eb8b0be1532ae2b678f62ac731838f57c89c48b83a9c5cfbb0141932171d`; deterministic tests assert the distinct `bond_contract_details` callback for official sample CUSIP `449276AA2`, returned conID `681308048`, secType `BOND`, tradingClass `IBM`, minTick `0.001`, `contract_details_end`, and the official bond-size warning code 2113. |
| 2026-05-02 | FA config read probe with `TestLiveRequestFA` | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203, current account is not an FA account | blocked | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && IBKR_LIVE=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9145 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./ -run "TestLiveRequestFA$" -count=1 -v -timeout 60s'` passed as an expected non-FA probe but `Advisors().Config(ctx, FADataGroups)` returned `context deadline exceeded` after 15s with no `receiveFA` callback or request-scoped API error. FA config success and replace/read-back evidence require an FA-enabled account. |
| 2026-05-02 | `TestLiveOfficialSDKSmoke` rerun with completed-order and execution snapshots | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | pass | `IBKR_LIVE=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9012 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./ -run TestLiveOfficialSDKSmoke -count=1 -v -timeout 180s` passed in 6.22s. Evidence highlights: completedOrders rows=8 and executions rows=0, alongside the existing read-only smoke coverage. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event completed-orders fixture capture | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9167 -scenario completed_orders_snapshot -out internal/sdkadapter/testdata/fixtures/official_sdk_completed_orders_snapshot_20260502.json -timeout 90s'` recorded a sanitized `apiOnly=true` fixture with source sha256 `3e7d4b241f5b122c6802b13b788b367e4583eaa77b7bfd442fd462fc54d66696`; account identifiers are redacted to `DU_REDACTED`, and completed-order contracts plus order fields are replaced with redaction placeholders. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event read-only fixture capture | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded | `go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9117 -scenario read_only_smoke -out internal/sdkadapter/testdata/fixtures/official_sdk_read_only_20260502.json -timeout 150s` recorded a sanitized fixture with source sha256 `8567dc6ede541fc441feafa4e072369a1ff1062281cd37d745ef9856f757673f`; account identifiers are redacted to `DU_REDACTED`; deterministic fixture tests assert only public metadata and WSH entitlement-error callback shapes from this broad fixture |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event current-time millis fixture capture | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded | `go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9115 -scenario current_time_millis -out internal/sdkadapter/testdata/fixtures/official_sdk_current_time_millis_20260502.json -timeout 60s` recorded a sanitized fixture with source sha256 `c605a7d012b27733f22f7782d7b369a3dff98799a18f49080ee1805a25349b69`; deterministic fixture tests assert fresh-session bootstrap metadata and captured millis timestamp `2026-05-02T15:39:52.854Z`; captured separately because serverVersion 203 only answered one current-time-style request per session |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event account-summary fixture capture | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9139 -scenario account_summary_snapshot -out internal/sdkadapter/testdata/fixtures/official_sdk_account_summary_snapshot_20260502.json -timeout 75s'` recorded a sanitized fixture with source sha256 `64adc4661bdaeba4b6c40b3808159194a32cd302b65978a17d0a622cef2b7364`; account identifiers are redacted to `DU_REDACTED`, account summary values are replaced with `REDACTED_VALUE`, and deterministic fixture tests assert `NetLiquidation`, `BuyingPower`, currency, and `accountSummaryEnd` callback shape without committing financial values. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event family-codes fixture capture | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9142 -scenario family_codes_snapshot -out internal/sdkadapter/testdata/fixtures/official_sdk_family_codes_snapshot_20260502.json -timeout 60s'` recorded a sanitized fixture with source sha256 `af54ff7bd7a7cf603b9ca8774b9f42f99ca242ce3f0fa34db66f5177df44e3d1`; account identifiers are redacted to `DU_REDACTED`, and deterministic fixture tests assert the single-account `familyCodes` callback shape. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event short quote-stream fixture capture | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded | `go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9123 -scenario quote_stream_short -out internal/sdkadapter/testdata/fixtures/official_sdk_quote_stream_short_20260502.json -timeout 60s` recorded a sanitized fixture with source sha256 `a514f2e78dc4c5fb66ca5b7e9b21a322ce2dcadf8d87ee7d5324d408d639e72e`; deterministic fixture tests assert delayed market-data type, delayed-data warning code 10167, tickReqParams BBO exchange `9c0001`, tick price/size/string callbacks, and no `tickSnapshotEnd` for the streaming request. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event real-time bars entitlement fixture capture | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded expected error | `go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9131 -scenario real_time_bars_short -out internal/sdkadapter/testdata/fixtures/official_sdk_real_time_bars_short_20260502.json -timeout 75s` recorded a sanitized fixture with source sha256 `baaa5d01dd79577b4c16d11632138bb3e751e811e2576950712daba7d3378b98`; deterministic fixture tests assert request-scoped code 420 (`No market data permissions`) and no real-time bar callback. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event tick-by-tick entitlement fixture capture | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded expected error | `go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9127 -scenario tick_by_tick_midpoint_short -out internal/sdkadapter/testdata/fixtures/official_sdk_tick_by_tick_midpoint_short_20260502.json -timeout 60s` recorded a sanitized fixture with source sha256 `d43018da4703e8bda7cdac8f3eab51600ed8ba89cc676e46cab6c9398627b459`; deterministic fixture tests assert request-scoped code 10189 (`No market data permissions`) and no tick-by-tick data callback. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event smart market-depth entitlement fixture capture | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded expected error | `go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9128 -scenario market_depth_smart_short -out internal/sdkadapter/testdata/fixtures/official_sdk_market_depth_smart_short_20260502.json -timeout 60s` recorded a sanitized fixture with source sha256 `6850cc11ff16f4bb417b85c015de64a94a54dd2fff88025aaa3d9be2284fef4c`; deterministic fixture tests assert request-scoped code 2152 (`Need additional market data permissions`) and no market-depth data callback. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event short historical-bars fixture capture | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded | `go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9124 -scenario historical_bars_short -out internal/sdkadapter/testdata/fixtures/official_sdk_historical_bars_short_20260502.json -timeout 90s` recorded a sanitized fixture with source sha256 `496348c4f40dfa0a64cd607d1921167ff344b20547bd406f7a353eda4f4bcc64`; deterministic fixture tests assert AAPL one-day one-hour RTH `historicalData` rows and `historicalDataEnd`. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event short keep-up historical-bars fixture capture | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9168 -scenario historical_bars_keepup_short -out internal/sdkadapter/testdata/fixtures/official_sdk_historical_bars_keepup_short_20260502.json -timeout 90s'` recorded a sanitized fixture with source sha256 `f6d87ab4c4a407bcd24e003647eebb87866a871411ad8bb06edc3769a15fbf32`; deterministic fixture tests assert the seven-row AAPL one-day one-hour RTH initial `historicalData` snapshot, `historicalDataEnd`, and no `historicalDataUpdate` on the Saturday capture. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event short historical-schedule fixture capture | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded | `go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9125 -scenario historical_schedule_short -out internal/sdkadapter/testdata/fixtures/official_sdk_historical_schedule_short_20260502.json -timeout 90s` recorded a sanitized fixture with source sha256 `58271ad7ab173e061158f8a2f4b2fc36fd6a88514de744b8b9c179a00b1aa5d4`; deterministic fixture tests assert US/Eastern AAPL one-month RTH schedule sessions. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event short historical-ticks fixture capture | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded | `go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9126 -scenario historical_ticks_midpoint_short -out internal/sdkadapter/testdata/fixtures/official_sdk_historical_ticks_midpoint_short_20260502.json -timeout 90s` recorded a sanitized fixture with source sha256 `6d047b0ba55fe9f6874a766a1b35bc4164a40fb1ef1138ebfb4a14470893b2ae`; deterministic fixture tests assert done AAPL midpoint historical ticks ending `20260501 16:00:00 US/Eastern`. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event fundamental-data fixture capture | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded | `go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9129 -scenario fundamental_data_snapshot -out internal/sdkadapter/testdata/fixtures/official_sdk_fundamental_data_snapshot_20260502.json -timeout 90s` recorded a sanitized public AAPL `ReportSnapshot` XML fixture with source sha256 `5a011b88fe2fb979be15619edfe925d193a9a63f7a5ff3083d5ff15f8dd6a84e`; deterministic fixture tests assert company/ticker/ratio/forecast XML and no request-scoped API error. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event scanner-parameters fixture capture | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded | `go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9133 -scenario scanner_parameters_snapshot -out internal/sdkadapter/testdata/fixtures/official_sdk_scanner_parameters_snapshot_20260502.json -timeout 90s` recorded a sanitized public scanner catalog XML fixture with source sha256 `fe731ef34f63face09077861a31ba57709b2c14db9e587ac495f2ac9d5aa3974`; deterministic fixture tests assert the full XML catalog shape, US stock instrument metadata, scan-code coverage, and filter list. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event scanner subscription fixture capture | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded | `go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9130 -scenario scanner_subscription_short -out internal/sdkadapter/testdata/fixtures/official_sdk_scanner_subscription_short_20260502.json -timeout 90s` recorded a sanitized public-market scanner fixture with source sha256 `2b8a6aebfb035bf8ac911393d7dd91932c9c91e77843f61697ce8dfe1c41a1c8`; deterministic fixture tests assert five `TOP_PERC_GAIN` `STK.US.MAJOR` SMART/USD stock rows and no request-scoped API error. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event display-group subscription fixture capture | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded | `go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9132 -scenario display_group_subscription_short -out internal/sdkadapter/testdata/fixtures/official_sdk_display_group_subscription_short_20260502.json -timeout 75s` recorded a sanitized Gateway display-group fixture with source sha256 `26b548ce8c3bcb6e6d73c1677407b4f79a23b0a15ae442490da3d00c5b3b1a0e`; deterministic fixture tests assert group list `1|2|3|4|5|6|7`, initial update `none`, and no request-scoped API error. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` display-group update callback attempt | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | blocked | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9137 -scenario display_group_update -out internal/sdkadapter/testdata/fixtures/official_sdk_display_group_update_20260502.json -timeout 90s'` timed out with `display group target update: context deadline exceeded`; no fixture was written. Follow-up read-only probe `go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9138 -scenario display_group_subscription_short -out /tmp/ibkr_display_group_probe.json -timeout 75s` recorded `DisplayGroupContractInfo: "none"`, so the timed-out update did not leave group 1 changed. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event invalid news request fixture capture | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded expected error | `go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9134 -scenario news_invalid_requests -out internal/sdkadapter/testdata/fixtures/official_sdk_news_invalid_requests_20260502.json -timeout 90s` recorded a sanitized fixture with source sha256 `fcb13d330984c46374c9afdd0c458a1f8e9eb5e97d244ba04c84cc5cc2401b9d`; deterministic fixture tests assert historical-news and article requests for `NO_SUCH_PROVIDER` both return request-scoped code 321 without committing provider article text. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event news article success fixture capture | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && GOCACHE=/tmp/go-cache-ibkr-go go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9171 -scenario news_article_snapshot -out internal/sdkadapter/testdata/fixtures/official_sdk_news_article_snapshot_20260502.json -timeout 120s'` recorded a sanitized fixture with source sha256 `e25f9cf9301804a7c6614efc580e1825d7487ab5b501b45242a67edde03567f6`; deterministic fixture tests assert five redacted AAPL historical-news rows, `historicalNewsEnd(hasMore=true)`, and a redacted `newsArticle` callback for an article ID sourced from that live capture. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event news-bulletin callback probe | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | blocked | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && GOCACHE=/tmp/go-cache-ibkr-go go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9175 -scenario news_bulletins_short -out /tmp/official_sdk_news_bulletins_short_20260502.json -timeout 60s'` exited with `news bulletins: context deadline exceeded`; no fixture was written because no live or cached `updateNewsBulletin` callback arrived during the 30s probe window. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event option calculation fixture capture | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded expected error | `go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9135 -scenario option_calculations_short -out internal/sdkadapter/testdata/fixtures/official_sdk_option_calculations_short_20260502.json -timeout 90s` recorded a sanitized fixture with source sha256 `2a3994aae03394ee462285aba2a88c0d0547ae50813bde22c8a8d2ccc4245975`; deterministic fixture tests assert both option calculation requests return request-scoped code 200 (`No security definition has been found`) and no `tickOptionComputation` callback. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event qualified option calculation fixture capture | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && GOCACHE=/tmp/go-cache-ibkr-go go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9174 -scenario option_calculations_qualified -out /tmp/official_sdk_option_calculations_qualified_20260502.json -timeout 120s'` recorded a sanitized fixture promoted to `internal/sdkadapter/testdata/fixtures/official_sdk_option_calculations_qualified_20260502.json` with source sha256 `c51064689faf03226922a56aa00da6d464784b60408d7f688ab5e6b9778435e5`; deterministic tests assert the qualified AAPL 20260618 200C `contractDetails` callback and successful implied-volatility and option-price `tickOptionComputation` callbacks. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event empty executions fixture capture | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | recorded | `go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9136 -scenario executions_empty_filter -out internal/sdkadapter/testdata/fixtures/official_sdk_executions_empty_filter_20260502.json -timeout 60s` recorded a sanitized fixture with source sha256 `6643d2468e2d9c3490272db3b7f7e98a89bb8bfea9042de004380d7904a344b0`; deterministic fixture tests assert `execDetailsEnd` for reqID 1101 with no execution details or commission reports. |
| 2026-05-02 | Initial `TestLiveOfficialSDKPaperOrderPlaceCancel` run | local paper Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203, test refused non-`DU` accounts before order placement | fail, fixed | `IBKR_LIVE=1 IBKR_LIVE_TRADING=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9021 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./ -run TestLiveOfficialSDKPaperOrderPlaceCancel -count=1 -v -timeout 180s` placed a non-marketable 1-share AAPL LMT paper order and sent handle cancel, but the engine closed the handle on warning code 399 (`Order Message: Warning: Your order will not be placed at the exchange until 2026-05-04 09:30:00 US/Eastern.`). Code 399 is now treated as an order informational notice rather than a terminal rejection. |
| 2026-05-02 | `TestLiveOfficialSDKPaperOrderPlaceCancel` rerun | local paper Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203, test refused non-`DU` accounts before order placement | pass | `IBKR_LIVE=1 IBKR_LIVE_TRADING=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9022 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./ -run TestLiveOfficialSDKPaperOrderPlaceCancel -count=1 -v -timeout 180s` passed in 4.20s. Evidence highlights: `openOrder` observed with status `PreSubmitted`, handle cancel sent, repeated `PreSubmitted` statuses observed, final `Cancelled` status received. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event paper order place/cancel fixture capture | local paper Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203, recorder refused non-`DU` accounts before order placement | recorded | `go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9119 -scenario paper_order_place_cancel -out internal/sdkadapter/testdata/fixtures/official_sdk_paper_order_place_cancel_20260502.json -timeout 90s` recorded a sanitized fixture with source sha256 `44f4dd370b903e85e74cfc11c9f1c464ad4d396890213dcc964632d12ebcf3e5`; account identifiers are redacted to `DU_REDACTED`, order permIDs are zeroed, and deterministic fixture tests assert `open_order`, warning code 399, and final `Cancelled` evidence. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event paper invalid-order fixture capture | local paper Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203, recorder refused non-`DU` accounts before order placement | recorded | `go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9122 -scenario paper_order_reject_invalid_type -out internal/sdkadapter/testdata/fixtures/official_sdk_paper_order_reject_invalid_type_20260502.json -timeout 90s` recorded a sanitized fixture with source sha256 `4ae50ed4eedf69ec7f6cb9421ad8dea7c6353a2777697470f2c051cc076e2642`; account identifiers are redacted to `DU_REDACTED`, no order permIDs were returned, and deterministic fixture tests assert order-scoped code 321 (`Invalid order type`) without an `openOrder` callback. |
| 2026-05-02 | `TestLiveOfficialSDKPaperOrderPlaceCancel` with open-orders snapshot | local paper Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203, test refused non-`DU` accounts before order placement | pass | `IBKR_LIVE=1 IBKR_LIVE_TRADING=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9027 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./ -run TestLiveOfficialSDKPaperOrderPlaceCancel -count=1 -v -timeout 180s` passed in 4.42s. Evidence highlights: `openOrder` observed with status `PreSubmitted`, `Orders().Open(all)` snapshot observed the same order ID with status `PreSubmitted`, handle cancel sent, final `Cancelled` status received. |
| 2026-05-02 | `TestLiveOfficialSDKPaperOrderPlaceCancel` with subscribe-open snapshot | local paper Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203, test refused non-`DU` accounts before order placement | pass | `IBKR_LIVE=1 IBKR_LIVE_TRADING=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9028 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./ -run TestLiveOfficialSDKPaperOrderPlaceCancel -count=1 -v -timeout 180s` passed in 4.61s. Evidence highlights: `openOrder` observed with status `PreSubmitted`, `Orders().Open(all)` observed the active order, `Orders().SubscribeOpen(all)` observed the active order, handle cancel sent, final `Cancelled` status received. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event paper open-orders fixture capture | local paper Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203, recorder refused non-`DU` accounts before order placement | recorded | `go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9121 -scenario paper_open_orders_place_cancel -out internal/sdkadapter/testdata/fixtures/official_sdk_paper_open_orders_place_cancel_20260502.json -timeout 90s` recorded a sanitized fixture with source sha256 `d93f8f351aef13bc227ac0d33e6d51798bbbb728227e675b1c044e485ee73971`; account identifiers are redacted to `DU_REDACTED`, order permIDs are zeroed, and deterministic fixture tests assert client-scope `openOrder`, `openOrderEnd`, and final `Cancelled` evidence. |
| 2026-05-02 | `TestLiveOfficialSDKPaperGlobalCancel` guarded preflight | local paper Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | skipped safely | `IBKR_LIVE=1 IBKR_LIVE_TRADING=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9030 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./ -run TestLiveOfficialSDKPaperGlobalCancel -count=1 -v -timeout 180s` skipped in 0.31s with `refusing global cancel test because 1 non-test open orders already exist`. The test only cleans up `ibkr-go-sdk-*` stale test orders and refuses to call global cancel when unrelated paper orders are open. |
| 2026-05-02 | Combined guarded paper-order rerun for place/cancel, modify/cancel, and global-cancel preflight | local paper Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203, tests refused non-`DU` accounts before order placement | pass with safe skip | `bash -lc 'eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)" && IBKR_LIVE=1 IBKR_LIVE_TRADING=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9141 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./ -run "TestLiveOfficialSDKPaper(OrderPlaceCancel\|OrderModifyCancel\|GlobalCancel)$" -count=1 -v -timeout 300s'` passed in 11.507s. Place/cancel observed `PreSubmitted`, open-orders snapshot, subscribe-open snapshot, and final `Cancelled`; modify/cancel observed quantity change from 1 to 2 and final `Cancelled`; global cancel skipped with `refusing global cancel test because 1 non-test open orders already exist`. |
| 2026-05-02 | Initial `TestLiveOfficialSDKPaperOrderModifyCancel` limit-price echo run | local paper Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203, test refused non-`DU` accounts before order placement | fail, revised | `IBKR_LIVE=1 IBKR_LIVE_TRADING=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9023 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./ -run TestLiveOfficialSDKPaperOrderModifyCancel -count=1 -v -timeout 180s` placed an AAPL LMT paper order and sent modify, but timed out waiting for a changed limit-price echo. The live assertion was revised to observe quantity modification instead of price normalization. |
| 2026-05-02 | `TestLiveOfficialSDKPaperOrderModifyCancel` quantity echo run | local paper Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203, test refused non-`DU` accounts before order placement | fail, fixed | `IBKR_LIVE=1 IBKR_LIVE_TRADING=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9024 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./ -run TestLiveOfficialSDKPaperOrderModifyCancel -count=1 -v -timeout 180s` observed modified `openOrder` quantity=2 and final `Cancelled`, but a live-derived code 201 (`Order has been cancelled already, too late to replace`) closed the handle as an error. That specific notice is now nonfatal; generic code 201 rejections remain fatal. |
| 2026-05-02 | `TestLiveOfficialSDKPaperOrderModifyCancel` rerun | local paper Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203, test refused non-`DU` accounts before order placement | pass | `IBKR_LIVE=1 IBKR_LIVE_TRADING=1 IBKR_LIVE_ADDR=127.0.0.1:4002 IBKR_LIVE_CLIENT_ID=9026 GOCACHE=/tmp/go-cache-ibkr-go go test -tags=ibkr_sdk ./ -run TestLiveOfficialSDKPaperOrderModifyCancel -count=1 -v -timeout 180s` passed in 4.74s. Evidence highlights: original `openOrder` quantity=1, modify sent, modified `openOrder` quantity=2 observed, handle cancel sent, final `Cancelled` status received. |
| 2026-05-02 | `cmd/ibkr-sdk-fixture` SDK-event paper order modify/cancel fixture capture | local paper Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203, recorder refused non-`DU` accounts before order placement | recorded | `go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture -host 127.0.0.1 -port 4002 -client-id 9120 -scenario paper_order_modify_cancel -out internal/sdkadapter/testdata/fixtures/official_sdk_paper_order_modify_cancel_20260502.json -timeout 90s` recorded a sanitized fixture with source sha256 `860489438be38dc855edaafe2496c4071cea832857d44c4972bd540e4b127aec`; account identifiers are redacted to `DU_REDACTED`, order permIDs are zeroed, and deterministic fixture tests assert initial quantity=1, modified quantity=2, and final `Cancelled` evidence. |
| 2026-05-02 | Diagnostic current-time sequencing: `CurrentTime` then `CurrentTimeMillis`, `CurrentTimeMillis` then `CurrentTime`, and `CurrentTime` twice | local Gateway/TWS on `127.0.0.1:4002`, outside the workspace sandbox, serverVersion 203 | observed limitation | In each diagnostic, the first current-time-style request completed and the second timed out. The long read-only smoke therefore exercises `CurrentTime`; `CurrentTimeMillis` is frozen as a separate fresh-session live smoke. The diagnostic tests were removed rather than kept as permanent failing tests |

Historical paper Gateway context before the SDK-native migration audit:
2026-04-15, `127.0.0.1:4002`, server_version 200.

## Live Test Results (client_live_order_test.go)

45 tests across 10 tiers. All pass or gracefully skip as of 2026-04-14.

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
| api_whatif_margin_aapl | 2026-04-14 | recorded |
| api_forex_lifecycle_eurusd | 2026-04-14 | recorded |
| api_stress_rapid_fire_aapl | 2026-04-14 | recorded |
| api_scale_in_campaign_aapl | 2026-04-14 | recorded |
| api_ioc_fok_aapl | 2026-04-14 | recorded (updated) |
| api_security_type_probe_matrix | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `9be83e57ed176a17` |
| api_tif_attribute_matrix_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `e6601dcc2abfd001` |
| api_algo_variants_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `1855e2554d7de3ae` |
| api_pairs_trading_aapl_msft | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `0dc806f7bb0868e8` |
| api_dollar_cost_averaging_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `296bdf662eb84e30` |
| api_stop_loss_management_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `a563cafd26e366be` |
| api_bracket_trailing_stop_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `2c0453360020a3ad` |
| api_future_campaign_mes | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `77d0b1b6a8c2d760` |
| api_option_campaign_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `5731e087403fe0f3`; OPRA/option path limited by real entitlement response |
| api_combo_option_vertical_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `495479ef4c345d96`; combo path blocked by option entitlement |
| api_market_data_completeness_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `f692fc168a53da9d`; mostly real entitlement errors |
| api_historical_matrix_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `366075c3b171c44d` |
| api_news_article_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `3c6ef62da8d60e95`; fetched article from historical-news ID |
| api_fundamental_reports_aapl | 2026-04-15 | recorded, verified; server_version=200, events sha256 prefix `02649216ff69f306`; mixed XML success and real 430 errors |
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
| api_ioc_fok_aapl.txt | 20260413T184916Z | promoted (exists) |
| api_tif_attribute_matrix_aapl.txt | 20260415T150535Z | promoted; covers GTC rest/cancel and trailing-percent fill replay |
| api_stop_loss_management_aapl.txt | 20260415T153735Z | promoted; covers market entry, protective stop modify/cancel, and flatten replay |
| api_transmit_false_then_transmit_aapl.txt | 20260415T162717Z | promoted; covers staged Transmit=false then transmit/cancel replay |
| api_reconnect_active_order_aapl.txt | 20260415T162822Z | promoted; covers GTC active order visible after reconnect |
| api_order_handle_reconnect_cancel_aapl.txt | 20260415T162822Z | promoted; covers original OrderHandle Gap/Resumed lifecycle and cancel after reconnect |
| api_client_id0_order_observation_aapl.txt | 20260415T162840Z | promoted; covers client ID 0 observing/cancelling another client's GTC order |
| api_cross_client_cancel_aapl.txt | 20260415T162857Z | promoted; covers client ID 2 observing/cancelling client ID 1 order |
| api_completed_orders_variants_aapl.txt | 20260415T170243Z | promoted; covers completed-orders apiOnly=false and apiOnly=true after live completed-order decode fix |
| api_future_campaign_mes.txt | 20260415T162047Z | promoted; covers MES futures buy/flatten round trip with executions and commissions |
| api_pairs_trading_aapl_msft.txt | 20260415T161858Z | promoted; covers paired AAPL long/MSFT short entries and per-symbol flatten replay |
| api_dollar_cost_averaging_aapl.txt | 20260415T161924Z | promoted; covers three staged AAPL entries and aggregate flatten replay |
| api_whatif_margin_aapl.txt | 20260414T164207Z | pending |
| api_forex_lifecycle_eurusd.txt | 20260414T164824Z | pending |
| api_bracket_trigger_aapl.txt | 20260413T174517Z | pending |
| api_oca_trigger_aapl.txt | 20260413T174546Z | pending |
| api_stress_rapid_fire_aapl.txt | 20260414T171824Z | pending |
| api_scale_in_campaign_aapl.txt | 20260414T172617Z | pending |

## Coverage Gaps: What We Need To Hit

### Security Types Not Yet Live-Tested

| SecType | Blocker | Path Forward |
|---------|---------|-------------|
| OPT | 2026-04-15 probe timed out while streaming chain details | Rerun narrower qualified option probe or subscribe to OPRA data |
| FOP | 2026-04-15 probe timed out | Rerun with a concrete future option contract after qualifying future expiry |
| BAG (combo) | Depends on OPT qualification | Same |
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

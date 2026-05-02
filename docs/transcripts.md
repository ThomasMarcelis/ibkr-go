# Transcripts

Behavioral scenarios from the legacy socket runtime use a canonical line-based
script format. This document describes historical replay and capture tooling
behind the non-default `legacy_native_socket` build tag. Current production
behavior is owned by the official IBKR C++ SDK bridge and should be frozen as
copied `internal/sdkadapter` command/event fixtures.

Current state:

- `testing/testhost` uses the legacy codec in both directions; transcript
  message names map to real IBKR integer message IDs
- checked-in transcripts cover both live-grounded scenarios and synthetic
  fault-injection cases for disconnects, partial frames, lifecycle edges, and
  other protocol failures
- live-grounded socket behavior was captured from IB Gateway `server_version
  200` and frozen into replay artifacts
- raw capture logs record per-leg connect/disconnect events plus TCP chunks;
  normalized replay artifacts reconstruct framed payloads from those chunks

## Goals

- human-diffable
- ordered by runtime sequence
- machine-validated by repo tooling
- expressive enough for delays, disconnects, partial frames, and bindings

## Script Shape

Each non-empty non-comment line is one step:

```text
client <message> <json-object>
server <message> <json-object>
sleep <duration>
disconnect
split <direction> <sizes> <message> <json-object>
raw <direction> <base64>
```

The JSON object is part of the line DSL. It provides typed values without
turning the scenario into a machine-first document format.

## Bindings

String values that start with `$` are symbolic bindings.

- In client expectation steps they bind on first match.
- In later client steps they match the previously bound value.
- In server steps they resolve to the bound value.

## Example

```text
client hello {"min_version":1,"max_version":1,"client_id":7}
server hello_ack {"server_version":1,"connection_time":"2026-04-05T12:00:00Z"}
server managed_accounts {"accounts":["DU12345"]}
server next_valid_id {"order_id":1001}

client req_contract_details {"req_id":"$req1","contract":{"symbol":"AAPL","sec_type":"STK","exchange":"SMART","currency":"USD"}}
server contract_details {"req_id":"$req1","contract":{"symbol":"AAPL","sec_type":"STK","exchange":"SMART","currency":"USD"},"market_name":"NMS","min_tick":"0.01","time_zone_id":"US/Eastern"}
server contract_details_end {"req_id":"$req1"}
```

## Testhost Contract

`testing/testhost` currently uses the legacy codec in both directions, but
it should be treated as replay tooling rather than as a place to define IBKR
protocol semantics.

- Client traffic is decoded and matched against the script.
- Server traffic is encoded from the script and written through the legacy wire
  framing code.
- Partial writes, malformed frames, delays, and disconnects are driven by the
  script rather than by ad hoc per-test logic.

## Capture Artifacts

The live capture tooling separates raw evidence from replay semantics:

- raw `events.jsonl` records connection lifecycle plus byte chunks as observed
  on the socket
- normalized `frames.jsonl` records connect/disconnect markers plus framed
  payloads reconstructed offline
- TCP chunk boundaries are not replay semantics and must never be treated as
  message boundaries

Official SDK adapter fixtures live under
`internal/sdkadapter/testdata/fixtures`. They are copied command/event fixtures
captured from the official C++ SDK adapter, not socket transcripts. Use
`cmd/ibkr-sdk-fixture` for SDK-owned fixture promotion:

```bash
eval "$(scripts/check-ibkr-sdk-env.sh --print-env .external/IBJts)"
go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture \
  -host 127.0.0.1 -port 4002 -client-id 9117 \
  -scenario read_only_smoke \
  -out internal/sdkadapter/testdata/fixtures/official_sdk_read_only_YYYYMMDD.json
go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture \
  -host 127.0.0.1 -port 4002 -client-id 9118 \
  -scenario current_time_millis \
  -out internal/sdkadapter/testdata/fixtures/official_sdk_current_time_millis_YYYYMMDD.json
go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture \
  -host 127.0.0.1 -port 4002 -client-id 9139 \
  -scenario account_summary_snapshot \
  -out internal/sdkadapter/testdata/fixtures/official_sdk_account_summary_snapshot_YYYYMMDD.json
go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture \
  -host 127.0.0.1 -port 4002 -client-id 9142 \
  -scenario family_codes_snapshot \
  -out internal/sdkadapter/testdata/fixtures/official_sdk_family_codes_snapshot_YYYYMMDD.json
go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture \
  -host 127.0.0.1 -port 4002 -client-id 9123 \
  -scenario quote_stream_short \
  -out internal/sdkadapter/testdata/fixtures/official_sdk_quote_stream_short_YYYYMMDD.json
go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture \
  -host 127.0.0.1 -port 4002 -client-id 9131 \
  -scenario real_time_bars_short \
  -out internal/sdkadapter/testdata/fixtures/official_sdk_real_time_bars_short_YYYYMMDD.json
go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture \
  -host 127.0.0.1 -port 4002 -client-id 9127 \
  -scenario tick_by_tick_midpoint_short \
  -out internal/sdkadapter/testdata/fixtures/official_sdk_tick_by_tick_midpoint_short_YYYYMMDD.json
go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture \
  -host 127.0.0.1 -port 4002 -client-id 9128 \
  -scenario market_depth_smart_short \
  -out internal/sdkadapter/testdata/fixtures/official_sdk_market_depth_smart_short_YYYYMMDD.json
go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture \
  -host 127.0.0.1 -port 4002 -client-id 9124 \
  -scenario historical_bars_short \
  -out internal/sdkadapter/testdata/fixtures/official_sdk_historical_bars_short_YYYYMMDD.json
go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture \
  -host 127.0.0.1 -port 4002 -client-id 9125 \
  -scenario historical_schedule_short \
  -out internal/sdkadapter/testdata/fixtures/official_sdk_historical_schedule_short_YYYYMMDD.json
go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture \
  -host 127.0.0.1 -port 4002 -client-id 9126 \
  -scenario historical_ticks_midpoint_short \
  -out internal/sdkadapter/testdata/fixtures/official_sdk_historical_ticks_midpoint_short_YYYYMMDD.json
go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture \
  -host 127.0.0.1 -port 4002 -client-id 9129 \
  -scenario fundamental_data_snapshot \
  -out internal/sdkadapter/testdata/fixtures/official_sdk_fundamental_data_snapshot_YYYYMMDD.json
go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture \
  -host 127.0.0.1 -port 4002 -client-id 9133 \
  -scenario scanner_parameters_snapshot \
  -out internal/sdkadapter/testdata/fixtures/official_sdk_scanner_parameters_snapshot_YYYYMMDD.json
go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture \
  -host 127.0.0.1 -port 4002 -client-id 9130 \
  -scenario scanner_subscription_short \
  -out internal/sdkadapter/testdata/fixtures/official_sdk_scanner_subscription_short_YYYYMMDD.json
go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture \
  -host 127.0.0.1 -port 4002 -client-id 9132 \
  -scenario display_group_subscription_short \
  -out internal/sdkadapter/testdata/fixtures/official_sdk_display_group_subscription_short_YYYYMMDD.json
go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture \
  -host 127.0.0.1 -port 4002 -client-id 9134 \
  -scenario news_invalid_requests \
  -out internal/sdkadapter/testdata/fixtures/official_sdk_news_invalid_requests_YYYYMMDD.json
go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture \
  -host 127.0.0.1 -port 4002 -client-id 9171 \
  -scenario news_article_snapshot \
  -out internal/sdkadapter/testdata/fixtures/official_sdk_news_article_snapshot_YYYYMMDD.json
go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture \
  -host 127.0.0.1 -port 4002 -client-id 9175 \
  -scenario news_bulletins_short \
  -out internal/sdkadapter/testdata/fixtures/official_sdk_news_bulletins_short_YYYYMMDD.json
go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture \
  -host 127.0.0.1 -port 4002 -client-id 9135 \
  -scenario option_calculations_short \
  -out internal/sdkadapter/testdata/fixtures/official_sdk_option_calculations_short_YYYYMMDD.json
go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture \
  -host 127.0.0.1 -port 4002 -client-id 9174 \
  -scenario option_calculations_qualified \
  -out internal/sdkadapter/testdata/fixtures/official_sdk_option_calculations_qualified_YYYYMMDD.json
go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture \
  -host 127.0.0.1 -port 4002 -client-id 9136 \
  -scenario executions_empty_filter \
  -out internal/sdkadapter/testdata/fixtures/official_sdk_executions_empty_filter_YYYYMMDD.json
go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture \
  -host 127.0.0.1 -port 4002 -client-id 9119 \
  -scenario paper_order_place_cancel \
  -out internal/sdkadapter/testdata/fixtures/official_sdk_paper_order_place_cancel_YYYYMMDD.json
go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture \
  -host 127.0.0.1 -port 4002 -client-id 9120 \
  -scenario paper_order_modify_cancel \
  -out internal/sdkadapter/testdata/fixtures/official_sdk_paper_order_modify_cancel_YYYYMMDD.json
go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture \
  -host 127.0.0.1 -port 4002 -client-id 9121 \
  -scenario paper_open_orders_place_cancel \
  -out internal/sdkadapter/testdata/fixtures/official_sdk_paper_open_orders_place_cancel_YYYYMMDD.json
go run -tags=ibkr_sdk ./cmd/ibkr-sdk-fixture \
  -host 127.0.0.1 -port 4002 -client-id 9122 \
  -scenario paper_order_reject_invalid_type \
  -out internal/sdkadapter/testdata/fixtures/official_sdk_paper_order_reject_invalid_type_YYYYMMDD.json
```

Run SDK fixture capture outside the sandbox when using a local Gateway or TWS;
the official SDK socket path may be blocked inside the sandbox. The recorder
hashes the raw unsanitized SDK event JSON and stores only that SHA-256 source
hash in the committed fixture. Account identifiers are redacted to
`DU_REDACTED`; the account-summary fixture also replaces financial values with
`REDACTED_VALUE`; paper-order permIDs are zeroed. The fixture tests reject
unredacted `DU`/`DUP`/`DA`/`U` account identifiers and nonzero committed
`PermID` values. The paper-order fixture scenarios refuse non-`DU` accounts and
capture sanitized non-marketable paper order placement/cancel, modify/cancel,
and client-scope open-orders snapshot callback sequences, plus a rejected
invalid-order-type scenario.
The read-only smoke fixture is a broad public-metadata regression artifact:
deterministic tests assert AAPL contract/search/quote/head-timestamp/histogram
callbacks, market-rule, sec-def option parameter, smart-component, depth
exchange, news-provider, soft-dollar-tier, user-info, display-group, and WSH
entitlement-error callback shapes. The dedicated account-summary fixture freezes
real `NetLiquidation`/`BuyingPower` callback shape with values redacted, and the
family-codes fixture freezes the redacted single-account callback shape; the
read-only smoke fixture deliberately does not freeze account financial values,
positions, FA config, executions, completed orders, or other account-private
callback payloads.
The empty executions fixture uses an impossible symbol filter and commits only
the public completion shape (`execDetailsEnd`) with no execution details or
commission reports; it does not replace non-empty paper execution evidence.
The fundamental-data fixture contains public AAPL issuer XML only; the scanner
fixtures contain public scanner catalog XML and scanner metadata for public US
stock symbols only; the display group fixture contains Gateway display-group IDs
and an empty initial group state. Do not commit account financial values,
broader order-writing evidence, provider news article bodies, or other private
live data unless it has been deliberately sanitized and is safe to publish as a
permanent regression artifact.

## Live Capture Runbook

The current paper Gateway target is `127.0.0.1:4002`. Capture through the
recorder proxy so raw evidence and normalized replay artifacts stay linked:

```bash
go build -o /tmp/ibkr-recorder ./cmd/ibkr-recorder
go build -o /tmp/ibkr-capture ./cmd/ibkr-capture
go build -o /tmp/ibkr-normalize ./cmd/ibkr-normalize
IBKR_UPSTREAM=127.0.0.1:4002 ./scripts/record-scenarios.sh quote_stream_multi_asset historical_ticks_aapl_timezone_window
./scripts/verify-captures.sh captures/<capture-dir>
```

Complex trading scenarios whose names start with `api_` are still recorded
through the same proxy, but the capture driver uses the public `ibkr.Client`
facade instead of hand-written wire calls. The raw `events.jsonl` remains the
protocol evidence; `driver.log` beside the capture records the human-readable
public-API order lifecycle, and `driver_events.jsonl` records structured
scenario/order/execution/commission checkpoints keyed by scenario run ID and
order ref.

Useful scenario batches:

```bash
IBKR_CAPTURE_BATCH=trading-basic ./scripts/record-scenarios.sh
IBKR_CAPTURE_BATCH=trading-advanced ./scripts/record-scenarios.sh
IBKR_CAPTURE_BATCH=trading-campaigns ./scripts/record-scenarios.sh
IBKR_CAPTURE_BATCH=trading-all ./scripts/record-scenarios.sh
```

For active-order reconnect captures, allow the recorder to accept multiple
connection legs:

```bash
IBKR_RECORDER_MAX_LEGS=3 IBKR_CAPTURE_BATCH=trading-campaigns ./scripts/record-scenarios.sh
```

`cmd/ibkr-normalize` can also emit a raw transcript skeleton for curation:

```bash
./ibkr-normalize -dir captures/<capture-dir> -transcript-out /tmp/<scenario>.txt
```

Raw capture directories remain local evidence because they may contain
account-specific details. When promoting behavior into CI, check in a curated
transcript under `testdata/transcripts` plus a public test that asserts the
behavior at the library API boundary. Record the raw capture directory name,
server version, scenario, and `events.jsonl` hash in the PR or accompanying
notes so the replay can be traced back to live evidence without committing raw
account data. Default replay tests should stay curated; exhaustive replay runs
use the `replay-all` catalog batch or an explicit test flag/env in the caller.

## Next Transcript Work

- use [`live-coverage-matrix.md`](live-coverage-matrix.md) as the target matrix
  for exhaustive live capture coverage and promotion status
- use [`ibkr-api-inventory.md`](ibkr-api-inventory.md) as the official/repo
  inventory that keeps the matrix from drifting away from IBKR's API surface
- grow scenario coverage for reconnect, pacing, and version-gated branches
- grow scenario coverage for order-management edge cases and more complex order
  shapes
- prefer complex live scenarios over one-request smoke captures when adding
  new coverage, especially for order, execution, account, PnL, historical
  window, and multi-subscription behavior
- broaden live capture coverage beyond `server_version 200`
- use the recorder and normalization tooling to derive new scenarios from
  contributor-owned Gateway or TWS sessions

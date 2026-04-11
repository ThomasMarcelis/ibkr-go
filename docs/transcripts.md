# Transcripts

Behavioral scenarios use a canonical line-based script format. Raw frame
goldens remain separate and are used only for `internal/wire` and malformed
frame cases.

Current state:

- `testing/testhost` uses the production codec in both directions; transcript
  message names map to real IBKR integer message IDs
- checked-in transcripts cover both live-grounded scenarios and synthetic
  fault-injection cases for disconnects, partial frames, lifecycle edges, and
  other protocol failures
- live-grounded behavior is captured from IB Gateway `server_version 200` and
  frozen into replay artifacts
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

`testing/testhost` currently uses the production codec in both directions, but
it should be treated as replay tooling rather than as a place to define IBKR
protocol semantics.

- Client traffic is decoded and matched against the script.
- Server traffic is encoded from the script and written through the same wire
  framing as production code.
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

Raw capture directories remain local evidence because they may contain
account-specific details. When promoting behavior into CI, check in a curated
transcript under `testdata/transcripts` plus a public test that asserts the
behavior at the library API boundary. Record the raw capture directory name,
server version, scenario, and `events.jsonl` hash in the PR or accompanying
notes so the replay can be traced back to live evidence without committing raw
account data.

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

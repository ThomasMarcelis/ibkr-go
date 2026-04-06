# Transcripts

Behavioral scenarios use a canonical line-based script format. Raw frame
goldens remain separate and are used only for `internal/wire` and malformed
frame cases.

Current repo truth:

- the checked-in scripts drive a legacy in-repo replay harness today
- the message names are currently symbolic logical names and are migration debt,
  not a source of truth for future protocol work
- capture and normalization tooling for real Gateway / TWS sessions is in repo,
  but the checked-in scenarios are not yet grounded in live-derived traces
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

## Next Transcript Work

- use the landed recorder and normalization tooling to derive checked-in
  scenarios from contributor-owned Gateway / TWS sessions
- replace logical scenario steps with real IBKR protocol messages or raw replay
  artifacts derived from live captures
- grow scenario coverage for reconnect, pacing, and version-gated branches

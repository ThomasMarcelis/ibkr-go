# Transcripts

Behavioral scenarios use a canonical line-based script format. Exact raw
frames are the executable protocol truth; symbolic steps remain only while
older `server_version 200` fixtures are migrated.

Current state:

- `internal/testhost` matches exact raw client frames and writes exact raw server
  frames for migrated fixtures; remaining symbolic steps still use the
  production codec in both directions
- checked-in transcripts cover live-grounded scenarios plus deliberate
  transport fault-injection around real message shapes for disconnects,
  partial frames, lifecycle edges, and other protocol failures
- live-grounded behavior includes the classic IB Gateway `server_version 200`
  baseline, the exact-201 protobuf executions migration, and the exact-202
  zero-strike execution-contract boundary, the exact-203 protobuf order
  lifecycle, the exact-204 completed-order boundary, and the exact-205
  contract-data boundary, frozen into replay artifacts
- raw capture logs record per-leg connect/disconnect events plus TCP chunks;
  normalized replay artifacts reconstruct framed payloads from those chunks

## Goals

- traceable to human-readable capture metadata and hashes
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

`internal/testhost` currently uses the production codec in both directions, but
it should be treated as replay tooling rather than as a place to define IBKR
protocol semantics.

- Raw client traffic is matched byte-for-byte. Remaining symbolic client
  traffic is decoded and matched against the script.
- Raw server traffic is written byte-for-byte. Remaining symbolic server
  traffic is encoded from the script through the production codec.
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

Maintainer live verification uses two Gateway roles:

- `readonly-live`: a real-money Gateway with live market data and read-only API
  permissions. It defaults to `127.0.0.1:4001` and can be set with
  `IBKR_LIVE_READONLY_ADDR`.
- `paper-dev`: a throwaway paper Gateway where order placement, cancellation,
  and flattening campaigns may run aggressively. It defaults to
  `127.0.0.1:4002` and can be set with `IBKR_LIVE_PAPER_ADDR`.

Run the diagnostic before recording:

```bash
go run ./cmd/ibkr-doctor -role readonly-live
go run ./cmd/ibkr-doctor -role paper-dev
```

Capture through the recorder proxy so raw evidence and normalized replay
artifacts stay linked:

```bash
go build -o /tmp/ibkr-recorder ./cmd/ibkr-recorder
go build -o /tmp/ibkr-capture ./cmd/ibkr-capture
go build -o /tmp/ibkr-normalize ./cmd/ibkr-normalize
IBKR_CAPTURE_ROLE=readonly-live ./scripts/record-scenarios.sh quote_stream_multi_asset historical_ticks_aapl_timezone_start
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
IBKR_CAPTURE_ROLE=readonly-live IBKR_CAPTURE_BATCH=exhaustive-read-only ./scripts/record-scenarios.sh
IBKR_CAPTURE_BATCH=trading-basic ./scripts/record-scenarios.sh
IBKR_CAPTURE_BATCH=trading-advanced ./scripts/record-scenarios.sh
IBKR_CAPTURE_BATCH=trading-campaigns ./scripts/record-scenarios.sh
IBKR_CAPTURE_BATCH=trading-all ./scripts/record-scenarios.sh
IBKR_CAPTURE_BATCH=all ./scripts/record-scenarios.sh
```

`record-scenarios.sh` is the single batch workflow, including the `all` batch.
It waits for an explicit recorder-ready handshake and gracefully reaps both
processes on failure or interruption so the recorder can flush capture files.
Set `IBKR_CAPTURE_FAIL_FAST=1` to stop after the first failed scenario; the
default records the rest of the batch and reports all failures at the end.

The script asks `cmd/ibkr-capture` for each scenario's role. Scenarios whose
catalog risk class is `paper_*` use `paper-dev`; all others use
`readonly-live`. `IBKR_CAPTURE_ROLE=paper-dev` may route read-only scenarios
through the paper Gateway during maintenance, but paper-order scenarios cannot
be downgraded to `readonly-live`. Use `IBKR_PAPER_UPSTREAM` or
`IBKR_LIVE_PAPER_ADDR` for paper overrides; legacy `IBKR_UPSTREAM` applies only
to read-only scenarios.

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

Curated live-derived fixtures use `DU9000001` as the canonical account
redaction token. Also sanitize execution IDs, order refs, perm IDs, and other
account-specific identifiers before checking in fixtures. If a transcript
header cites raw Gateway evidence and contains account-scoped fields, the
header should say that account-specific identifiers are sanitized.

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
- broaden live capture coverage beyond exact `server_version 225`, one
  migration gate at a time
- use the recorder and normalization tooling to derive new scenarios from
  contributor-owned Gateway or TWS sessions

## Raw frames are the canonical server-side representation

Server frames in transcripts at any version other than the default
`server_version 200` MUST use `raw server` steps carrying live-captured bytes;
the testhost rejects DSL-form server frames for such transcripts. DSL-form frames re-encode through the codec
under test, so a version-gated layout bug would replay green and only fail
live — the symmetric-codec-bug class. New captures land raw by default (the
capture pipeline already produces the raw bytes); the JSON message form is a
human-readable view, kept where a consistency check against the decoded raw
frame exists or where the flow is low-risk and sv200-only. Existing sv200
transcripts migrate opportunistically, orders and executions first.

Raw transcript files record the source capture hash and negotiated server
version. Use `splitraw <server|client> <sizes> <base64-frame>` when a transport
test must deliver or read an exact captured frame in partial chunks; chunking
changes transport behavior without reconstructing the protocol message through
the codec under test.

## Provenance gate

Every transcript starts with an initial contiguous comment block that names at
least one capture ID, exactly one negotiated server version, and either a full
64-hex `events.jsonl` SHA-256 or an explicitly labelled 16-hex legacy prefix.
Every recorded handshake must agree with the declared version. The structural
parser ignores comments after the first executable line, so later scenario
notes cannot satisfy provenance.

`TestTranscriptProvenanceInventory` freezes the measured corpus and its sole
legacy-prefix transcript. A legacy prefix remains honest historical evidence,
but it can never satisfy a stable-release proof. Stable proof records must name
a regular-file basename directly under `testdata/transcripts`; their capture
ID, server version, and full hash must agree exactly with the parsed header.
Do not add a guessed source or rewrite captured evidence to satisfy the gate.

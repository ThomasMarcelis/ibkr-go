# Transcripts

A transcript is a replayable record of one conversation with IB Gateway. The
test suite replays transcripts against `internal/testhost`, which plays the
Gateway's side byte for byte, so behavioral tests run in CI without a broker
login. Every server frame in the tracked corpus was captured from a live
Gateway at `server_version` 208 through 225; none is hand-written. Current
counts live in [`live-coverage-matrix.md`](live-coverage-matrix.md).

## Script shape

Transcripts are line-based scripts under `testdata/transcripts/`. Each
non-empty, non-comment line is one step:

```text
handshake {"server_version":225,"connection_time":"20260824 22:27:46 CET"}
raw <server|client> <base64-frame>
splitraw <server|client> <sizes> <base64-frame>
sleep <duration>
disconnect
```

- `handshake` supplies the negotiated `server_version` and the wire-format
  `connection_time`. An optional `"client_id":"$client"` binds the client ID
  the testhost decodes from `START_API`.
- `raw` carries one complete length-prefixed frame. Client frames are matched
  byte for byte; server frames are written byte for byte.
- `splitraw` delivers the same frame in the listed comma-separated chunk
  sizes, so transport tests can exercise partial reads without rebuilding the
  message through the codec under test.
- `sleep` and `disconnect` drive delays and transport loss from the script
  instead of from per-test logic.

There are no symbolic message steps. Re-encoding a server reply through the
codec under test would let a symmetric encode/decode bug replay green and
fail only live, so a new server-message shape always needs a captured raw
frame. Request-ID correlation is frozen in the captured bytes.

## Provenance header

Every transcript starts with a contiguous comment block that names the
capture ID, the negotiated server version (or an exact range), and the full
SHA-256 of the capture's `events.jsonl`. This is
[`current_time_live.txt`](../testdata/transcripts/current_time_live.txt) in
full:

```text
# Exact readonly-live capture 20260824T202747Z-current_time at
# server_version 225; events.jsonl sha256:
# a9029ff8e7cfed19cab1e3e2eccc4c36d7c91b95aa6aa03f75543bacac454a9e.
# The account identifier is a deterministic length-preserving substitution.
# Ambient farm-status frames are omitted. The retained request and
# seconds-resolution response are exact captured frames.
handshake {"server_version":225,"connection_time":"20260824 22:27:46 CET"}
raw server AAAADwAAANcKCURVOTAwMDAwMQ==
raw server AAAABgAAANEIAQ==
raw client AAAABAAAAPk=
raw server AAAACgAAAPkIwtKy1AY=
disconnect
```

`TestTranscriptProvenanceInventory` freezes the corpus size and rejects a
transcript whose header is incomplete or disagrees with its handshake. The
parser reads only the comments before the first executable line. Never
invent a source or edit captured bytes to satisfy the gate.

## Capture artifacts

The live tooling keeps raw evidence separate from replay semantics:

- `events.jsonl` (raw): connection lifecycle plus the byte chunks observed on
  the socket. TCP chunk boundaries are not message boundaries.
- `frames.jsonl` (normalized): connect and disconnect markers plus complete
  framed payloads reconstructed offline.
- `driver.log` and `driver_events.jsonl` (public-API scenarios only): the
  human-readable and structured record of what the scenario did through
  `ibkr.Client`. They hold normalized API output, never wire syntax, so never
  copy their timestamps into a transcript.

Raw capture directories stay local because they contain account data.

## Recording live captures

Maintainer live verification uses two Gateway roles:

- `readonly-live`: a real-money Gateway with live market data and read-only
  API permissions. Default `127.0.0.1:4001`; override with
  `IBKR_LIVE_READONLY_ADDR`.
- `paper-dev`: a throwaway paper Gateway where order placement, cancellation,
  and flattening may run aggressively. Default `127.0.0.1:4002`; override
  with `IBKR_LIVE_PAPER_ADDR`.

Run the diagnostic, build the three tools, and record through the recorder
proxy so raw evidence and normalized artifacts stay linked:

```bash
go run ./cmd/ibkr-doctor -role readonly-live
go run ./cmd/ibkr-doctor -role paper-dev

go build -o /tmp/ibkr-recorder ./cmd/ibkr-recorder
go build -o /tmp/ibkr-capture ./cmd/ibkr-capture
go build -o /tmp/ibkr-normalize ./cmd/ibkr-normalize
IBKR_CAPTURE_ROLE=readonly-live ./scripts/record-scenarios.sh quote_stream_multi_asset historical_ticks_aapl_timezone_start
./scripts/verify-captures.sh captures/<capture-dir>
```

`record-scenarios.sh` records the named scenarios, or the batch named by
`IBKR_CAPTURE_BATCH`: `exhaustive-read-only` (default), `trading-basic`,
`trading-advanced`, `trading-campaigns`, `trading-all`, `replay-all`, or
`all`. It waits for the recorder's ready handshake and reaps both processes
on failure so capture files are flushed.

- `IBKR_CAPTURE_FAIL_FAST=1` stops at the first failed scenario; the default
  records the rest and reports every failure at the end.
- `IBKR_RECORDER_MAX_LEGS` (default 8) bounds connection legs per capture so
  reconnect scenarios and the paper reconciliation pass fit in one capture.
- `IBKR_RECORDER`, `IBKR_CAPTURE`, and `IBKR_NORMALIZE` point at the built
  binaries when they are not under `/tmp`.

The script asks `ibkr-capture -role-for <scenario>` which role each scenario
needs. Scenarios with a `paper_*` risk class run against `paper-dev` and
cannot be downgraded; `IBKR_CAPTURE_ROLE=paper-dev` may route read-only
scenarios through the paper Gateway during maintenance. Paper-order scenarios
also need `IBKR_PAPER_ACCOUNT` and `IBKR_CAPTURE_GLOBAL_CANCEL=1`; the live
safety rules are in [`CONTRIBUTING.md`](../CONTRIBUTING.md).

Scenarios named `api_*` drive the public `ibkr.Client` rather than
hand-written wire calls. The raw `events.jsonl` is still the protocol
evidence.

`ibkr-normalize -dir captures/<capture-dir> -verify` checks framing,
handshake order, inbound decoding, and driver lifecycle, and prints the source
capture hash. It writes no output artifact and does not sanitize the capture
directory. Replay tests assert the scenario's behavior through the public API.

## Promoting a capture into CI

1. Emit a transcript skeleton:
   `/tmp/ibkr-normalize -dir captures/<capture-dir> -transcript-out /tmp/<scenario>.txt`
2. Review and finish sanitization. The skeleton redacts supported protocol
   fields; it still needs human review before sharing. `DU9000001` is the
   canonical account token. Also replace
   execution IDs, order refs, perm IDs, and other account-specific
   identifiers, and say so in the header. Preserve every non-sensitive wire
   value exactly, including timestamp syntax and timezone suffixes.
3. Curate the transcript under `testdata/transcripts/` with its provenance
   header, and add a public test that asserts the behavior at the library
   API boundary.
4. Record the capture directory name, server version, scenario, and
   `events.jsonl` hash in the PR so the replay stays traceable without
   committing account data.

Default replay tests stay curated; exhaustive runs use the `replay-all`
catalog batch.

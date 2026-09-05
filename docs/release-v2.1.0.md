# v2.1.0 candidate

Prepared on 2026-09-05. This is an untagged candidate; v2.0.3 remains the
published install target. The only public API addition is
`WithOrderExecutionCorrelationLimit`. No dependencies or supported Gateway
versions changed. User-visible changes are in the [changelog](../CHANGELOG.md).

## Evidence

- Reconnect: local Gateway sv225 returned code 10089 after a proxy outage before
  the fix. The same test receives quote data before and after recovery with
  selection restoration. `delayed_quote_reconnect.txt` retains the successful
  exchange. Four older replays insert that exact captured selection before
  replacement work; lineage checks still require the original callback order.
- Fees: exact MES callbacks reproduce loss when execution processing is delayed
  one second beyond the former expiry. Regressions cover retention, consecutive
  duplicate reports, bounded IDs and pending versions, cleanup, reconnect,
  overflow scope, exercise uncertainty, and independent queries and observation.
- Hedge replacement: the captured hedge validates with its parent, but failed
  when `Replace` omitted it. Omitted and matching parents now encode identically;
  conflicting parents and recovery-disabled replacement still fail.
- Socket writes: injected zero, partial, and full acceptance with an error use
  the captured clock request. Full acceptance emits `Started` before transport
  interruption, preserving the original error and uncertainty.
- Release compatibility: isolated Git histories cover ordering, prereleases,
  candidate exclusion, missing evidence, and exact mismatch. A separate run
  with the pinned real `apidiff` rejects a regenerated breaking candidate both
  before and after adding its tag.
- Documentation: examples use the public API while existing captures retain
  replay coverage. The reconnect example's Ctrl-C exit changed from 1 to 0
  during handshake and streaming against paper Gateway sv225; a subprocess
  regression covers interrupted setup. An invalid address still exits 1.

Private source `events.jsonl` hashes (raw captures are not committed):

| Capture | SHA-256 |
|---|---|
| `20260905T185228Z-delayed_quote_reconnect_before` | `a17818d88b7e05915c90d623bc254d2dfc6999215e707eb59d5ebab79ce391cb` |
| `20260905T185320Z-delayed_quote_reconnect` | `70f830c4e934dc9acc7be03a3e0ac23e00b1e46f84082c7e930a8f069cfdf7aa` |

MES, hedge, and clock provenance remains in their existing transcript headers
and regression comments. The corpus now contains 115 transcripts. The capture
catalog remains 125 scenarios: 104 promoted and 21 blocked.

## Verification

Passed with Go 1.26.7 on Linux/amd64:

- Module tidy/verify, formatting, build, vet, lint, pure-Go linux/386 build.
- All packages with `CGO_ENABLED=0`, shuffled tests, and race detection.
- API compatibility, exact candidate, and `--release v2.1.0` checks.
- All five fuzz targets for five seconds each; vulnerability scan found no issues.
- Capture normalization/verification and transcript provenance/lineage checks.
- All 17 documentation examples, nine README blocks, and four package-overview
  snippets compile in consumer modules against v2.0.3 and the candidate. Example
  attachment, rendered pkgsite links, and the 125-scenario inventory check out.
  All executable examples build.
- Focused live read-only checks, including exact handshakes 208–225 and quote
  recovery through a forced outage, passed. The changed order and bracket
  examples ran on paper sv225 and observed cancellation of all four placed legs.

An existing logger test intermittently assumed a fatal diagnostic would reach
its handler before shutdown discarded queued logs. It now uses the existing
nonfatal diagnostic and clock exchange to control shutdown; 50 race-enabled
repetitions and the final full suites pass. Production logging is unchanged.

The backpressure test also raced a clock timeout that correctly retires the
session. It now verifies local execution observation with the outbound queue
full; the separate singleton-cancellation regression retains retirement coverage.

## Performance check

Ten runs of `BenchmarkPublicQuoteStream`, `-benchtime=1x`, on Go 1.26.7,
Linux/amd64, Ryzen 7 9800X3D, `GOMAXPROCS=16` produced these medians:

| Workload | Result |
|---|---|
| 32-frame batches with consumer credits | 719,484 updates/s; 239.5 B and 12 allocations/update |
| One outstanding frame, unloaded delivery | p50 8.08 µs; p95 21.24 µs; p99 33.49 µs |
| Dial to first update, default outbound pacing | 20.64 ms |

These include loopback TCP, replay, and scheduling. They are separate
workloads, not simultaneous throughput and latency guarantees. Historical
README results retain their original Go 1.26.3 environment; this run does not
establish a regression against them. The three-second actor profile no longer
samples per-event validation or `testing.Helper` in its timed path. No
production optimization was added.

## Remaining work before publication

Follow the [release checklist](../CONTRIBUTING.md) on the exact release tree,
including platform CI and the release workflow's longer fuzz runs. Set the
changelog date and published install target when releasing. The candidate
remains untagged and unpublished.

Signed cancellation remains excluded. It needs client-0 binding of a
user-created, nonmarketable paper TWS order and capture of cancellation of that
observed order through its terminal status. The current live helper's
positive-ID assertion must be corrected with that work. New order allocation
must stay positive; cancellation should accept signed int32 IDs except zero
only after the required evidence is obtained.

No live same-ExecID changed-fee revision has been captured. The pending-version
regression aliases distinct captured reports to one local correlation record
to test A→B→A retention; it does not claim Gateway revision evidence. Native
macOS/Windows execution and a live hedge replacement were not repeated here.
Regulatory snapshots and option exercise were not repeated.

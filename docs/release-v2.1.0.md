# v2.1.0 release record

Prepared for release on 2026-09-06 after the maintainer review. It adds explicit
cancellation identity, order acknowledgement and snapshot-capability signals, and bounded
execution correlation. Seven deliberate source breaks and the behavior changes
are documented in [migration-v2.1](migration-v2.1.md). No dependencies or
supported Gateway versions changed. User-visible changes are in the
[changelog](../CHANGELOG.md); accepted, rejected and deferred findings are in
the [decision ledger](review-decisions-2026-09-06.md).

## Review evidence, 2026-09-06

Retained sv225 callbacks freeze independent handle/observer fanout,
acknowledgement, rejection 10315, and execution/fee attribution. Actor tests
cover canceled historical waits, bootstrap timeout under both reconnect
policies, partial writes, data-maintained restoration, and shutdown ordering.
Reordering, omitted callbacks, constrained queues and partial writes are local
fault injection, identified beside the tests. Regulatory uncertainty uses the
existing capture; no new fee-bearing request was made.

Public scanner XML, PositionsMulti and historical LAST collectors now have
retained-frame regressions. Classic request tests promote 23 request types from
sv208/210/211 captures. This does not complete classic encoder evidence or the
decoder ledger: 86/106 layouts have positive attestation, with 20 still pending.
The [coverage matrix](live-coverage-matrix.md) separates request bytes, positive
callbacks, public replay and per-login entitlement observations.

The API gate reads the immutable highest reachable stable same-major baseline.
`v2.0.3-v2.1.0.breaks` records exactly seven incompatible differences and names
the migration guide. Isolated-history regressions reject missing, stale, extra
and wrong-pair allowances. The candidate manifest and candidate-tag exclusion
checks remain mandatory; no general compatibility bypass was added.

## Earlier candidate evidence, 2026-09-05

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

## Verification, 2026-09-06

Passed with Go 1.26.7 on Linux/amd64:

- Module tidy/verify, formatting, build, vet, lint, pure-Go linux/386 build.
- All packages, shuffled tests, and shuffled race tests.
- API compatibility, exact candidate, and `--release v2.1.0` checks.
- Fuzz inventory and all five targets for 30 seconds each; govulncheck v1.6.0
  found no vulnerabilities.
- Capture normalization, transcript provenance/lineage, codec vectors and
  attestation-test citations through the deterministic suite.
- Successful and failed raw replays repeated 300 times under `ulimit -n 256`.
- Live read-only handshake/current-time checks for every supported version
  208–225 against `127.0.0.1:4002`, client ID 19207. These verify negotiation and
  clock behavior, not every operation at every version.
- Rendered pkgsite HTML verifies the release install commands, new API anchors
  and documentation links. The browser preview tool could not initialize in
  this environment, so this revision has no fresh screenshot inspection.

The final paper sv225 example run on September 6 exercised all ten programs.
Nine exited successfully, including Ctrl-C for resilient quotes. Historical
bars returned the expected typed code-2188 entitlement refusal. Order 702 and
bracket legs 703–705 all reached Cancelled. No regulatory snapshot or option
exercise was issued. The deliberately changed examples now require v2.1.

On September 5, before these review changes, the candidate also passed
pure-Go tests, documentation snippet compilation against v2.0.3 and that
candidate, rendered pkgsite checks, and live quote recovery. That forced-outage
run was not repeated for this revision.

An existing logger test intermittently assumed a fatal diagnostic would reach
its handler before shutdown discarded queued logs. It now uses the existing
nonfatal diagnostic and clock exchange to control shutdown; 50 race-enabled
repetitions and the final full suites pass. Production logging is unchanged.

The backpressure test also raced a clock timeout that correctly retires the
session. It now verifies local execution observation with the outbound queue
full; the separate singleton-cancellation regression retains retirement coverage.

## Consumer migration check, 2026-09-06

An isolated copy of the ibkr-mobile hub was migrated from its v2.0.2 pin using
a temporary module replacement. The real consumer checkout and dependency pin
were left unchanged. The edits propagate OrderTarget through the broker port,
live adapter, fake, simulator and recovered-order cancellation, and classify
opening orders using Market/Limit plus OPG. Targeted regressions cover that
classification and foreign-target refusal.

`go build -buildvcs=false ./...` and
`go test -race ./internal/orders ./internal/broker/... -count=1 -timeout=60s`
passed. VCS stamping was disabled only because this temporary copy has no
repository metadata. The full hub suite compiled but did not pass:

- `TestE2E_SimBarsCarryTheSessionsTheyFallIn/TIMEFRAME_1D` returned NO_DATA/404.
  The same failure reproduced in all three runs against the original v2.0.2
  dependency, establishing that it predates this migration.
- `TestSub_AnIdleScanIsGivenUpToMakeRoomForANewOne` observed four subscriptions
  while expecting five. Twenty isolated race runs passed on each dependency.
  Its immediate-close/asynchronous-open ordering remains an unresolved test
  limitation; this run does not establish the cause or a migration regression.

The reusable migration steps are in [migration-v2.1](migration-v2.1.md). The
temporary audit also produced an 11-file hub patch, excluding dependency pins
and local replacement paths. This checks compilation and affected behavior;
it does not claim a mobile rollout, GUI verification or live trading migration.

## Earlier performance check, 2026-09-05

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

## Publication checks and evidence limits

Publication requires the [release checklist](../CONTRIBUTING.md) on the exact
release tree, platform CI, and the annotated-tag workflow. GitHub's workflow
runs record the final remote checks and publication outcome.

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

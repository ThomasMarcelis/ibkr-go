# Roadmap

Next work in priority order. [Coverage](live-coverage-matrix.md) distinguishes
captured requests, positive callbacks, and public replay. The
[2026-09-06 decision ledger](review-decisions-2026-09-06.md) records every review
recommendation, including those deliberately rejected.

## Next

1. **Opening-auction evidence.** Validate MKT/LMT+OPG at an appropriate paper
   venue and auction window before claiming an opening fill. v2.1's source
   changes and consumer migration are recorded in the
   [migration guide](migration-v2.1.md) and [release record](release-v2.1.0.md).
2. **Complete classic request evidence.** Retained vectors now cover 23 request
   types at sv208/210/211. Audit remaining classic paths with coverage, promote
   any additional retained bytes, then capture missing read-only requests and
   cancellations with official-source invariants. Exercise is an exception: use
   existing source/capture evidence only; do not send another instruction.
   Remaining codec-package gaps include calculation cancels, FA requests, WSH
   requests/cancels, display-group update/unsubscribe, bulletin request/cancel,
   exercise and order-ID refresh. Classic current time already has root-package
   version replay; codec-only coverage does not measure that evidence.
3. **Positive market-data evidence.** Obtain the required API subscriptions for
   real-time bars, tick-by-tick, depth, odd-lot ticks, historical MIDPOINT/BID_ASK,
   histogram, and keep-up updates. Positive LAST at sv215 does not prove all tick
   families. `internal/codec/codec_capture_coverage_test.go` remains the checklist.
4. **Product/event and reconnect evidence.** FA, WSH, bulletins, EFP,
   delta-neutral callbacks, unknown conditions/incomplete OpenOrder, and code
   1300 need real callbacks. Capture executions/fees during replacement
   bootstrap with generation attribution, and bootstrap during nightly reset
   (including 1100 before next-valid-ID). Capture a same-ExecID changed fee
   revision. Do not infer success from an entitlement refusal.
5. **Manual binding and cross-client scope.** A synchronized owner/observer
   paper capture should correlate request time, client IDs, permanent ID,
   statuses, and cancellation outcome. Existing cross-client cancels drew 10147;
   the owner performed cleanup. All-scope snapshots do not grant cancellation
   rights. Signed cancellation needs client-0 binding of a user-entered,
   nonmarketable paper TWS order through terminal status. Do not automate the
   GUI/configuration, repeat regulatory snapshots, or repeat option exercise.
6. **Pure parsers requested by the consumer.** Add trading-hours parsing from
   dated/implicit-end, repeated-day, leading-empty, closed-session and DST/zone
   fixtures. Add a scanner catalog parser preserving duplicate list roles,
   varName and AbstractField semantics. Retain raw data and fail explicitly on
   unsupported input; use the consumer's captured corpus with provenance first.
7. **sv226 and official drift.** Extend the exact handshake matrix and add
   `conditionsIncludeOvernight` (field 145) when a Gateway negotiates 226.
   `Order.IncludeOvernight` (field 135) is already supported. Track official
   changes against API 10.50.01.

## Designs needing a specification before implementation

- **Unkeyed abandonment:** bounded tombstones, safe slot reuse, never-reply and
  shutdown policy. Keep connection retirement until those laws are proven.
- **Attach/recover orders:** explicit identity, reconstruction of required
  fields, missed-history and reconnect rules, independently verified broker
  state. Do not re-enable Replace on old handles using a snapshot alone.
- **Account retarget / open-order polling:** serialization, close/cancel
  acknowledgement limits, context lifetime and reconnect behavior. Existing
  account/position multi APIs and explicit Refresh remain the tools today.
- **Pacing observability/governor:** define the request families, small-bar
  scope, BID_ASK weighting, admission ownership, priorities and deadline costs;
  capture behavior with an entitled account. Avoid a blanket 60/10-minute rule.
- **Typed data helpers:** start with RTVolume and a few demanded generic ticks
  or numeric account tags; preserve raw value, units, currency, presence and
  errors. Any normalized no-quote view needs product-aware bid/ask/last-size
  evidence, including valid negative prices. AuxPrice remains order-specific.
- **Consumer testing support:** expose a narrow replay/fault boundary around
  the real engine with sanitized captures. Avoid a second facade implementation
  or public configuration structs solely to inspect options in fakes.
- **Cancel reason delivery:** prove operation/client attribution before also
  placing session-level cancellation notices on handles.
- **New outright rejection codes:** require captured outcome evidence. Unknown
  pre-echo errors remain warnings; acknowledgement timeouts and code 10147 do
  not establish rejection.

## Opportunistic hygiene

Consolidate singleton/subscription setup or codec loops only when a touched
behavior benefits and regressions remain stable. Move test helpers/projections,
remove unused internal parameters, split the capture driver or large test files,
and replace real-I/O ordering sleeps with observed-request barriers as those
areas change. A suite-wide factory/clock abstraction, pools, actor sharding,
custom decimal implementation, and mixed-stream coalescing are not scheduled.

## Not planned

Flex, Client Portal Web API, EWrapper/EClient compatibility, SDK runtime
integration, server-log/auth/redirect/configuration mutation, a generic
IsTransient or IsOrderTargeted safety classifier, and blanket price-sentinel
conversion remain outside the design.

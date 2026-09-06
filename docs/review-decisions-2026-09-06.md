# Maintainer decisions on the 2026-09-06 reviews

Scope: implementation above `1202c327`, reviewed for v2.1.0 before publication.
The three private reports remain unchanged. This public synthesis names their
findings without copying private account data or unsupported success claims.

Sources: **W** = whole-library review, numbered findings 1–7;
**R** = v2.1.0 deep review, original D/DOC/R/A/API/P/T/G identifiers;
**C** = ibkr-mobile consumer report, items 01–18. “Implemented” describes this
reviewed v2.1.0 implementation. Follow-ups have concrete destinations
in [the roadmap](roadmap.md); migration requirements are in [migration-v2.1](migration-v2.1.md).

## Correctness and API decisions

| Source | Verdict | Maintainer reasoning and implementation/evidence |
|---|---|---|
| W1 | Implemented, clean removal | `OrderAuction.Strategy` and inbound `OrderAuctionDetails.Strategy` have no supported protobuf representation. Remove input, conversion, clone inventory, and validation instead of retaining a permanently rejected field. Other auction fields survive. API 10.50.01 `Order.proto` is the source; no invented field or classic fallback. |
| W2 | Implemented | Independent handle/observer fanout in `engine_route.go`; captured sv225 OpenOrder and OrderStatus both reach the healthy observer after handle overflow. `TestOrderStartOverflowPreservesOpenOrdersObserver` proves both branches. |
| W3 | Implemented; positive auction outcome deferred | Remove MOO/LOO/PEG PRI constants. Keep negative capture strings; document MKT/LMT+OPG and REL. Do not silently translate PEG PRI to PEG PRIM or remove PEG MKT for an instrument-specific refusal. Official opening compositions need venue/auction-time evidence before claiming fills. |
| W4 | Implemented | Normalize news/tick absolute bounds to UTC in `requests.go`; `TestHistoricalBoundsPreserveRepeatedDSTHour` freezes repeated DST hours and equivalent instants. Positive news is observed; positive ticks on the current login remain entitlement-blocked. |
| W5; R R6; C14 cancellation | Implemented | Actor-owned historical waits react to context and timer exactly once, recheck readiness, stop at shutdown, and never enqueue recursively into a full actor queue. `await_test.go`, `engine_review_lifecycle_test.go` cover immediate cancellation, the 2/15-second boundaries, simultaneous wakes, bootstrap, and shutdown. |
| W6 | Implemented | `internal/testhost` closes listener and connection on every exit; constructor helpers register cleanup immediately; Close interrupts scripted sleep. Successful and failed raw replay repeat under `ulimit -n 256` without GC dependence. |
| R D1/R1/R2 | Implemented taxonomy and tests; retry redesign rejected | Pre-Ready transport failure wraps a cause in bootstrap `ConnectError` under both policies; the five-second timer is tested. The caller's earlier deadline need not contain a bootstrap failure that never occurred. Auto reconnect-until-ready remains useful and documented. Initial bootstrap during 1100/nightly reset is separately capture-gated. |
| R D2; C05/10052 | Implemented; 10052 already released | sv225 order 591 gets 10315 then not-found on cancel: add `ErrCodeActiveStartTimeInvalid` and terminate observation. Strengthen `TestAPITIFAttributeMatrixAAPLReplay`. DAY default and 10052 rejection already shipped in v2.0.3. |
| R P4/D2 adjacent | Implemented | Remove unproven 10342 from the cancel-reply classifier. A speculative classification must not hide what-if errors. Keep the unknown-code warning policy for live orders. |
| R D3 | Implemented | Release pending fees for executions with no local owner; retain a separately bounded FIFO exclusion set, whose eviction never closes handles. Positive ownership outranks exclusions; missing identity stays conservative. Tests cover both callback orders, eviction, late fees after Close, positive claims, unknown fees, cleanup, and independent query/observer paths. |
| R D4 | Implemented | Close SessionEvents before Done; buffered events remain readable. Shutdown and transition-sequence tests freeze observability. |
| R D5 | Implemented | Code-0 internal server failure proves neither completion nor absence of billing. Wrap regulatory uncertainty, retain the API cause, update `TestRegulatorySnapshotServerFailureUncertainReplay`. No new regulatory request. |
| C01 context lifetime | Retained and documented | Subscription contexts already own lifetime; separating default admission/lifetime or adding WithLifetime creates another ownership mode without solving singleton correlation. Per-method docs and a lifetime-context example address the actual consumer mistake. |
| C01 unkeyed abandonment | Deferred design, blanket abandonment rejected | A known reply shape does not identify which invocation it belongs to. Safe tombstones need a slot-reuse rule, bounded retention, shutdown behavior, and a never-reply policy. The actor continues retiring unresolved unkeyed generations. |
| C01/C09 market rule zero | Implemented validation; scalar retained | Reject <=0 before admission. A zero venue MarketRuleID already expresses no rule; document it instead of allocating a pointer for every entry. |
| C02 | Implemented HasSnapshot; nil-on-absence rejected | Capability is separate from historical completion and health. HasSnapshot remains stable through boundary/closure; AwaitSnapshot continues ErrNoSnapshot. Actual PositionsMulti and PnL routes now assert the distinction. |
| C03 | Deferred | Attaching/recovering a replacement handle needs identity, original fields, event-history, and reconnect invariants. An Open snapshot cannot restore missed history. Existing recovered same-client cancellation remains available. |
| C04 | Deferred | Retargeting a singleton is a broker lifecycle, not just a local swap. Keep explicit ownership and multi-account APIs; do not add implicit call queuing that hides competing owners. |
| C05 acknowledgement | Implemented | `OrderHandle.Acknowledged` records valid attributed OpenOrder, OrderStatus, or Execution independently of queue delivery. It survives reconnect as historical evidence, never permits Replace after recovery, and does not fire from warnings, writes, or closure. |
| C05 all pre-echo errors | Rejected | Corpus review found 26 code-399 warnings before first echo, plus code 10349. The consumer claim that these occur only afterward is false. `TestCapturedPreEchoWarningDoesNotAcknowledgeOrReject` freezes a real counterexample. Add rejection codes only with outcome evidence. |
| C06 allocation-only cancellation | Replaced by explicit identity | `OrderTarget` carries ClientID+OrderID; foreign targets fail before admission. Requiring current-process allocation would break restart recovery. Existing captured foreign-cancel requests are replayed through a test-only raw connection to preserve code-10147 evidence; public calls now assert local rejection. |
| C06; R D2/10147 | Rejected termination inference | 10147 also occurs for a real order owned by another client. It cannot safely terminate a local order route or prove an order was never held. The old roadmap success claim was wrong; owner cleanup completed cancellation in those captures. |
| C07 | Documentation implemented; automatic polling deferred | All-scope Open/Refresh asks for a snapshot. Cross-client pushes depend on Gateway master-client configuration, not merely requesting All. Auto/client-0 manual binding is a separate mechanism. Keep explicit Refresh and consumer reconciliation cadence. |
| C08 constants | Implemented, narrower semantics | Name 10091 and 10172. Intermittent 10091 on one delayed login is not a universal transient law; a news article unavailable now is not proven permanently impossible. Keep classifiers unchanged. |
| C08 IsTransient/IsOrderTargeted | Rejected as blanket classifiers | IDs share request/order space; targeting alone proves neither order rejection nor retry safety. Use operation context, existing typed classifications, and reconciliation policy. |
| C09 quote/AuxPrice | Blanket normalization rejected; narrow follow-up | -1 and zero can be legitimate prices; OHLC size zero is not bid/ask absence. AuxPrice meaning depends on order type. Preserve raw data and presence; document interpretation and require product-aware captures before a narrower normalized view. |
| C10 | Accepted follow-up | Pure trading-hours and scanner-catalog parsers are useful. Ground them in consumer captures, preserve raw input, handle dated/implicit-end grammar, DST/zone aliases and XML list varName/AbstractField distinctions. No silent skipping or guessed time zones. |
| C11 | Deferred | A narrow RTVolume parser and named generic-tick helpers can earn their keep. A large Extended Quote with every generic request adds presence/hot-path costs before positive evidence; requesting a tick does not guarantee its reply. |
| C12 numeric tags | Deferred | Numeric accessors must preserve raw strings, currency, nonnumeric tags, and absence. No universal AccountValue-to-decimal conversion. |
| C12 P&L | Documentation implemented | Account-window and dedicated P&L use different update/reset schedules and scopes. Neither is universally authoritative; choose the source matching the view. Do not scale or reconcile away observed differences as a codec error. |
| C13 | Implemented validation/docs | Calculation requests need Exchange; ConID alone still works for discovery. Record one login's 10–13 vs absent 80–83 observation and directional price/IV results without promising those callbacks on all accounts. |
| C14 governor/observability | Deferred | Local 2/15-second gates are documented, not a complete pacing model. A 60/10-minute rule needs its actual small-bar scope, BID_ASK weighting, scenario/queue semantics, and entitlement-capable evidence; a blanket global limiter would be wrong. |
| C15 | Rejected generic coalescing | A quote event includes a raw tick plus snapshot; account events are different keyed fields, and depth is a delta sequence. Dropping arbitrary mixed events loses business or lifecycle evidence. A future latest-state view must define its own keys and continuity law. |
| C16 | Deferred toolkit; config export rejected for now | Test against the real engine/replay boundary, not a second fake implementation of concrete facades. Exporting internal option/config machinery solely for a fake creates maintenance without enforcing protocol invariants. Consumer fakes retain their app-level role. |
| C18 | Corrected | `Order.IncludeOvernight` is already supported (field 135). Only `conditionsIncludeOvernight` (field 145) needs sv226. |
| R API5/API6 | Implemented | Omit empty APIError operation text and use inboundProtocolError at next-valid-ID, seconds/millis clock, and display-group parse failures. |
| R API9 | Implemented now | Remove always-rejected FADataProfiles within this documented v2 minor; waiting for v3 provides no value for an unusable constant. |

## Documentation and verification decisions

| Source | Verdict | Destination and limits |
|---|---|---|
| W7 PostOnly | Corrected | `types_order.go` follows official field 139: one-cent repricing around hidden same-price liquidity, and IOC at a better price when crossing. No promise of reject-only behavior or live venue proof. |
| W7 summary/context/admission; C17 | Corrected | Explicit summary tags; admission-only order/exercise control; per-method context, singleton, snapshot, continuity/resume and pacing comments in `client.go`; lifetime-context example in operation control. No generated runtime capability table solely for prose. |
| R DOC1 | Corrected | Snapshot.CurrentTime is zero until a completed clock request. No extra bootstrap clock request behind the 4.25-second gate. |
| R DOC2 | Corrected | ErrInterrupted includes local outbound backpressure. Do not treat every occurrence as a physical connection outage. |
| R DOC3; G1 | Claims narrowed; retained vectors promoted | `TestEncodeRetainedClassicRequests` freezes 23 request types from original sv208/210/211 captures, with sanitized account tokens and source hashes. This reduces, but does not complete, the review's classic encoder gap. Handshake/current-time coverage spans all 18 versions; positive decoder evidence stays 86/106. Missing classic requests and positive callbacks remain distinct roadmap work. |
| R DOC4 links/transcripts | Corrected | HistoryClient links and transcript grammar now match code; no client_id symbolic binding exists. |
| R DOC4 README/release | Corrected | Teaser exits after a usable quote and closes its stream; remove “each one file”; retain v2.0.3 pins until actual publication. Changelog describes user effects; migration carries detail. |
| R DOC4 architecture | Corrected | Document shared request/order IDs, minimum order floor and avoiding active request/order collisions. |
| C08/C11/C13 delayed observations | Documented, account-specific | Coverage matrix records 2188 keep-up bars, 10089 tick-by-tick, notice-only 2152 depth, intermittent 10091, unavailable article 10172, no observed RTVolume/delayed option IDs. Volume-ratio observations justify no scaling. |
| R R3/R4/R6 | Frozen | Data-maintained OrderRestored, incomplete tracked writes under both policies, and the fifteen-second historical gate now have direct regressions. |
| R R7 | Partly frozen; positive gaps retained | Public scanner XML and PositionsMulti collectors use retained frames. Historical LAST typed projection uses exact sv215 data. Histogram/keep-up bars/tick-by-tick/bulletins retain refusal or lifecycle evidence; positive data requires entitlement/event captures. |
| R R8 | Frozen | Both sides of order parameter gates at 216/217/218/223 run through Encode, not only the inner protobuf encoder. |
| R R9 singleton/sequence/truncation | Frozen | An unkeyed one-shot during 1101, detectable dropped TransitionSeq, and truncations of a real protobuf LAST frame. Malformed message-scoped decode is distinct from fatal framing failure. |
| R T13 | Implemented | `TestAttestationTestNamesExist` parses root and codec tests to detect stale ledger citations; existence complements the actual capture assertions. |
| R R9 remaining; W verification limits | Deferred | Capture 1300, execution/fee delivery during replacement bootstrap, and nightly reset; replace ordering sleeps only with request-observed barriers when touched. Existing Close/overflow/tracked-write tests remain, with no demonstrated production deadlock from a >513-write burst. |
| R R10 | Capture-gated | Unknown conditions/incomplete OpenOrder, OrderBound identity, signed manual IDs, fee revisions and sv226 need separate evidence. Do not add raw condition preservation or inferred binding on speculation. |
| R P6 | Deferred | Cancel notices remain on SessionEvents. Handle warnings need reliable operation/client attribution, particularly after ID reuse and cross-client observations. |
| R A1/A2/A3 | Opportunistic only | Singleton/subscription helper consolidation could remove duplication, but broad actor refactoring is not required for these fixes. Retain closure routes and black-box API tests. |
| R A4/A6/A7/T17 | Opportunistic only | Remove unused internal parameters/helpers, move projections, split test/capture files, or adopt a clock abstraction only when a concrete touched area benefits. No suite-wide timing redesign to save seconds. |
| R A5/API7/API10 | Rejected broad cleanup | Keep benign nil guards and WithDialer(nil)'s established default semantics. Correct test-engine bootstrap invariants locally; a new factory solely to remove guards is not earned. |
| W/R architecture/performance | Retained | One actor, generation-tagged drain-before-loss, pure-Go codecs, separate route ownership, decimal/presence semantics, no order replay. Benchmarks do not justify pooling, actor sharding, or decimal replacement. |

## Evidence and release policy

The official source baseline is API 10.50.01, archive SHA-256
`aa065722ca732a41aab202c7bb72932e179b86e7ec51cefa063eb1983fe9f597`.
Key exact public captures: reconnect-active-order
`bb1d3c6803a7140fd4636570f43e91f047f57177e280b10b602f4786ace980a9`,
order-type matrix `13462e4fb99d643c7a95b113636e9cd853ba9369c85817ffc4b44825e26cb015`.
Other provenance is recorded beside each fixture/vector. Callback reordering,
omission/corruption, local queue capacity, and partial writes are explicitly
fault injection; they are not claims that the complete scenario occurred live.

The compatibility gate still reads the highest reachable stable same-major
tag's immutable manifest. The exact baseline/candidate `.breaks` file must
match the full incompatible apidiff output and point to a migration document.
Extra, removed, stale, or wrong-pair differences fail. `--release` also requires
the exact candidate manifest and excludes its own tag. Historical allowances
are inapplicable once a newer release is the baseline. There is no fixed break
count or general bypass. Behavioral changes require manual migration review
because apidiff cannot detect them.

Final commands and consumer verification belong in the
[release record](release-v2.1.0.md). This ledger records the implementation
review; release publication is a separate step. Manual paper TWS binding,
another regulatory snapshot, and another exercise instruction remain excluded.

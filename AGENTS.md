# AGENTS.md

This document is the standing brief for anyone working on `ibkr-go`, human or agent. It is normative. Read it end to end before making changes.

## Core Mindset

- **Solo dev / YAGNI.** Do not overengineer. Do not overabstract. Build against the current codebase. Keep one implementation per behavior and move deliberately forward. No duplicate implementations, compatibility shims, or fallback code paths.
- **Implementation-first docs.** Docs are implementation-first: if a doc conflicts with code, the code wins — then update the doc. docs are aimed to be complementary to the code, with in-line code comments explaining why where it adds signal without noise and is worth the extra maintenance
- **Prefer clean breaks.** Breaking changes are acceptable when they simplify the system. Avoid compatibility baggage.

## Scope and Non-Goals

`ibkr-go` is an idiomatic Go client for the Interactive Brokers TWS and IB Gateway socket protocol designed from first principles.

**v1 target: read-only production core.** Handshake, negotiated version, managed accounts, account values, positions, contract details, quotes, real-time and historical bars, execution and open-order observation, explicit reconnect and resume behavior.

**Not in v1.** Order writes, scanners, news breadth, Flex, Client Portal Web API, near-full parity with the entire TWS API surface, an `EWrapper` / `EClient` compatibility bridge.

The full v1 charter lives in [`docs/roadmap.md`](docs/roadmap.md). The clean-room posture is in [`docs/provenance.md`](docs/provenance.md).

## Go Style

- Go 1.26.
- **No generic pointer helpers** like `ptr[T]()`. Prefer inline `new(expr)` for optional pointer values when it improves clarity.
- **Errors.** Prefer `errors.AsType` over `errors.As` for typed unwrapping.
- **Iterators.** Prefer `iter.Seq` / `iter.Seq2` for lazy iteration pipelines when it improves API clarity and avoids intermediate slice allocations. Do not force iterators into obviously simpler slice-based code.
- **Concurrency boundaries.** Slices and maps carry shared backing memory. Deep-copy them before transferring ownership across channels or to background goroutines when mutation or reuse is possible.
- **Interface discipline.** Accept interfaces, return concrete structs. Never use pointers to interfaces.
- **Configuration APIs.** Prefer functional options (`opts ...Option`) over builder structs for complex construction flows.
- **Concrete types by default.** Use interfaces only for real capability boundaries, not default `IFoo` / `Foo` ceremony.
- **Organize by domain, ownership, and runtime phase** — not by corporate controller / service / repository layers.
- **High signal, low noise.** Do not default to guard-heavy code; add guards when they meaningfully protect correctness, document an invariant, or make an invalid state explicit.
- **Earned and local abstractions.** YAGNI applies aggressively. If breaking one of these rules makes the code simpler, clearer, and more explicit, prefer the simpler design.

## Build by Specification

1. **Specify.** Name the truth first — the framing invariants, the codec round-trip laws, the session state transitions, the named scenarios that must hold over time.
2. **Test.** Write the tests in layers (see next section).
3. **Implement.** Build the feature to pass the tests.
4. **Verify and freeze.** Confirm behavior and freeze with regression tests.

**Rule: No new protocol area without invariants. No tuning without scenarios. No refactor without regression coverage.**

## Testing Philosophy

The test suite is this library's primary asset — the moat. It is grown deliberately and with discipline.

### Testing pyramid

Every subsystem gets its own layered tests. Each test proves one hypothesis.

1. **Invariants.** Protocol framing, codec round-trip, session state machine transitions. Must never regress. Small, exhaustive.
2. **State transitions.** Single-message encode and decode cases. Table-driven.
3. **Behavioral scenarios.** End-to-end typed one-shot flows (e.g., `DialContext` → `ManagedAccounts` → `AccountSummary`, subscription lifecycle) against the in-process `testing/testhost/` with deterministic checked-in transcripts.
4. **Stress and edge.** Malformed frames, partial reads, reconnect mid-subscription, negotiated-version edge cases.
5. **Deterministic first.** All tests use `testhost` plus checked-in transcripts. Live Gateway is only for transcript refresh, never routine CI.

### Quality bar

- Every test must improve confidence or diagnosis quality. If it does not, it does not get added.
- Tests target public API behavior, not implementation. Internal refactors must not force test rewrites.
- No test exists solely to prove the code was written.
- Every bug fix lands with the transcript or test that would have caught it. That test becomes a permanent regression freeze.

## Clean-Room Boundary

The library is a clean-room implementation. The full policy is in [`docs/provenance.md`](docs/provenance.md). Summary of the normative rules:

- **Forbidden sources.** Official Interactive Brokers client source in any language. Official package layout or file structure as a template. Official state machine structure or `EWrapper` / `EClient` callback shape as the primary public design. Code, types, or test fixtures copied from other OSS IB libraries.
- **Allowed references.** Official protocol documentation. Wire and protocol behavior observed by the contributor against a Gateway or TWS instance the contributor controls. Other OSS IB libraries read for anti-pattern study only.
- **Contributor gate.** Every protocol-adjacent pull request must include the clean-room attestation checkboxes from the pull request template. Every protocol-adjacent commit body includes the line `clean-room: no official TWS API source was referenced.` or equivalent.

## Commit Convention

Applies to every commit in this repository. See [`CONTRIBUTING.md`](CONTRIBUTING.md) for the developer-facing version.

- **Subject.** imperative mood, concrete. Lowercase after any prefix. No trailing period.
- **Body (optional,  when warranted).** Focuses on **why**. What constraint or design pressure forced the change. What alternatives were rejected. What invariant the change preserves.
- **One logical change per commit.**
- **Never** mention LLM, agent, Claude, Opus, Sonnet, Haiku, AI, "generated with", or any co-author trailer implying non-human authorship. The commit history reads as a senior engineer's work because that is the quality bar. Commit history needs to be meaningful without noise.
- No WIP commits, no "fix typo" follow-ups — squash before landing.
- No emoji, no marketing voice, no excitement.
- Protocol-adjacent commits carry a clean-room attestation line in the body.

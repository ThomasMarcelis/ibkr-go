# ibkr-go: Whole-Target Plan

## Context

`ibkr-go` is a clean-room Go client for the Interactive Brokers TWS and IB Gateway socket protocol. It is developed as an independent project with its own agent-orchestrated slice delivery. It is not managed from or coupled to any consumer project; consumers fetch it as an external Go library via `go get github.com/ThomasMarcelis/ibkr-go@<version>`.

The scope of this plan is the three slices that deliver v1:

- `10` — repo and charter (**complete** at `502be28`)
- `11` — read-only production core (**next**)
- `11b` — reconciliation (**conditional**, only if external dogfooding reports a gap)

After slice `11` lands and external consumers have validated the library against real workloads, the repository becomes eligible for public release and a v1.0 tag.

## End state

- A single Go module at `github.com/ThomasMarcelis/ibkr-go` with the layout locked in slice 10: `ibkr/` (public), `internal/{wire,codec,transport,session}/`, `testing/testhost/`, `experimental/raw/`, and `testdata/transcripts/`.
- Typed one-shot request methods and typed subscriptions as the primary public API. No `EWrapper` / `EClient` callback shape as the primary design.
- First-class observable session state and explicit reconnect behavior.
- A test suite organized in five layers (invariants, state transitions, behavioral scenarios, stress / edge, deterministic first) that is both high-signal and low-maintenance — see the Testing Philosophy section below.
- CI green on every push to `main` via GitHub Actions: `gofmt -l .`, `go vet`, `go build`, `golangci-lint run`, `go test ./...`.
- A public v1 release once the read-only core is stable and at least one external consumer has validated it against a live Gateway for the intended workload.

## 1. Orchestration framework

### 1.1 State source of truth

`IMPLEMENTATION_STATUS.md` at the repository root is the only persistent progress file. It encodes completed slices, ready slices, blocked slices, and the current next-ready set. The orchestrator reads it at cycle start; the active slice updates it before completion.

### 1.2 Orchestrator loop

Per cycle:

1. **Preflight.** Read: `AGENTS.md`, `IMPLEMENTATION_STATUS.md`, `docs/provenance.md`, `docs/anti-patterns.md`, `docs/roadmap.md`, `docs/slices/README.md`, and the specific slice prompt for the cycle. Verify tracker and slice prompts agree; fix any drift before feature work.
2. **Select.** Determine the next ready slice. Because ibkr-go has only a small linear chain (11 → maybe 11b), there is no parallel window; the selection is deterministic.
3. **Spawn or execute.** Execute the slice directly in the main session, or spawn a subagent with a brief rendered from §2. Since this repo has no sibling-repo work, worktree isolation can be used without the sandbox constraints that affected larger orchestrators.
4. **Wait for completion.** Receive the completion report.
5. **Post-completion review.** Run the verification commands, read the diff, check commit convention compliance, confirm the clean-room attestation is present for protocol-adjacent commits.
6. **Advance** or stop (if the roadmap is exhausted or the next slice is conditional and not yet triggered).

### 1.3 Slice gating

- Slice `11` is **not** plan-gated — its scope is already well-defined by `docs/slices/11-core.md` and the frozen decisions in `AGENTS.md` and `docs/roadmap.md`. Execute directly.
- Slice `11b` is **conditional** — it only runs when an external dogfood gap is filed against the project. When it runs, it is plan-gated because the scope depends on the specific gap reported.

## 2. Per-slice execution template

Each slice executes against a brief containing:

1. **Role / mission** — one sentence.
2. **Slice identity** — number, name, dependencies, write scope from the slice prompt.
3. **Authoritative references** — `AGENTS.md` (root), `docs/provenance.md`, `docs/slices/<slice>.md` (embedded verbatim), the relevant subsections of this file.
4. **Acceptance criteria** — union of the slice prompt's Definition Of Done, Required Tests, and the testing philosophy's pyramid coverage for the scope.
5. **Execution directives** — Go 1.26, zero non-stdlib deps unless explicitly approved, coding standards from `AGENTS.md` §Go Style, commit policy (one commit per slice), commit message per §3 with clean-room attestation.
6. **Report format** — summary, commit hash, tests run, self-review checklist, open notes.
7. **Failure protocol** — return BLOCKED with the exact blocker; never guess to unblock.

## 3. Commit message convention

Applies to every commit in this repository. Also in `AGENTS.md` and `CONTRIBUTING.md`.

- **Subject line.** ≤72 characters, imperative mood, concrete. For slice commits: `slice NN: <change>`. Lowercase after any prefix, no trailing period.
- **Body (optional, only when warranted).** Focuses on **why**. What constraint or design pressure forced the change. What alternatives were rejected. What invariant the change preserves. The diff shows *what*; the body carries context the diff cannot.
- **One logical change per commit.** One commit per completed slice.
- **Never** mention LLM, agent, Claude, Opus, Sonnet, Haiku, AI, "generated with", or any co-author trailer suggesting non-human authorship. The commit history reads as a senior engineer's work.
- No WIP commits and no "fix typo" follow-ups in main history; squash before landing.
- No emoji, no marketing voice, no excitement.
- **Protocol-adjacent commits carry a clean-room attestation line in the body**: `clean-room: no official TWS API source was referenced.`

## 4. Architecture bake (end state)

```
ibkr-go/
  LICENSE
  README.md CONTRIBUTING.md CODE_OF_CONDUCT.md SECURITY.md AGENTS.md
  .gitignore .golangci.yml go.mod go.sum
  .github/
    workflows/ci.yml
    ISSUE_TEMPLATE/{bug_report,feature_request}.md
    PULL_REQUEST_TEMPLATE.md
  docs/
    provenance.md anti-patterns.md feature-matrix.md roadmap.md
    slices/{README,10-repo-and-charter,11-core,11b-reconciliation}.md
    orchestration/{WHOLE_TARGET_PLAN,ORCHESTRATOR_PROMPT}.md
  IMPLEMENTATION_STATUS.md
  ibkr/
    client.go                 Client, Options, DialContext, typed one-shots, subscriptions (slice 11)
    types.go                  public types (slice 11)
    errors.go                 typed errors, errors.AsType-compatible (slice 11)
    doc.go                    package doc
  internal/
    wire/
      doc.go, frame.go, field.go, ...  (slice 11)
    codec/
      doc.go, encode.go, decode.go, message_*.go (slice 11)
    transport/
      doc.go, conn.go, dial.go, readloop.go, writeloop.go (slice 11)
    session/
      doc.go, handshake.go, state.go, reconnect.go, version.go (slice 11)
  testing/
    testhost/
      doc.go, server.go, transcript.go, dispatch.go (slice 11)
  testdata/
    transcripts/
      *.txt                   checked-in deterministic transcripts (slice 11 onward)
  experimental/
    raw/
      doc.go                  opt-in low-level access (v2+)
```

## 5. Testing philosophy (normative)

The test suite is this library's primary asset — the moat. It is grown deliberately.

### Testing pyramid

Each test proves one hypothesis.

1. **Invariants.** Protocol framing, codec round-trip, session state machine transitions. Must never regress. Small, exhaustive.
2. **State transitions.** Single-message encode / decode cases. Table-driven.
3. **Behavioral scenarios.** End-to-end typed one-shot flows (`DialContext` → `ManagedAccounts` → `AccountSummary`, subscription lifecycle) against `testing/testhost/` with deterministic checked-in transcripts.
4. **Stress / edge.** Malformed frames, partial reads, reconnect mid-subscription, negotiated-version edge cases.
5. **Deterministic first.** All tests use `testhost` + checked-in transcripts. Live Gateway is only for occasional transcript refresh, never routine CI.

### Quality bar

- Every test must improve confidence or diagnosis quality. If it does not, it does not get added.
- Tests target public API behavior, not implementation. Internal refactors must not force test rewrites.
- No test exists solely to prove the code was written.
- Every bug fix lands with the transcript or test that would have caught it. That test becomes a permanent regression freeze.
- **No new protocol area without invariants. No tuning without scenarios. No refactor without regression coverage.**

## 6. Engineering mindset (ported from MarketSimulator)

These conventions are load-bearing for the whole project. They also live in `AGENTS.md`.

1. **Solo dev / YAGNI.** No duplicate implementations, compatibility shims, or fallback code paths. One implementation per behavior. Move deliberately forward.
2. **Implementation-first docs.** If a doc conflicts with code, the code wins — then update the doc.
3. **Clean breaks.** Breaking changes are acceptable when they simplify the system. Avoid compatibility baggage.
4. **Build by specification.** Specify invariants → test in layers → implement → verify and freeze. No new mechanic without invariants. No tuning without scenarios. No refactor without regression coverage.
5. **Style defaults.** Small, explicit, systems-style. Concrete types by default. Interfaces only for real capability boundaries. Organize by domain, ownership, and runtime phase. High signal, low noise. Earned and local abstractions.

## 7. Verification strategy

### 7.1 Per-slice verification

After each slice completes, run from the repo root:

```
go build ./...
go vet ./...
gofmt -l .          # must produce no output
golangci-lint run
go test ./...
```

All five must be green. Then read the diff, verify commit message convention (subject ≤72 chars, imperative, no trailing period, clean-room attestation line for protocol-adjacent work, no LLM / agent / AI references), and verify the tracker row is updated.

### 7.2 Whole-system smoke

Before tagging any release:

- `go test ./...` against a refreshed transcript set runs fully green.
- At least one behavioral scenario per v1-scoped request type (managed accounts, account summary, positions, contract details, quotes, real-time bars, historical bars, order/execution observation) is covered by an end-to-end test against `testhost`.
- No live Gateway is required to run any CI test.
- `gh workflow view CI` on the latest push shows a green run.

## 8. Critical files the orchestrator reads each cycle

1. `IMPLEMENTATION_STATUS.md`
2. `AGENTS.md`
3. `docs/provenance.md`
4. `docs/roadmap.md`
5. `docs/slices/README.md`
6. The specific slice prompt for the cycle: `docs/slices/<slice>.md`
7. `docs/orchestration/ORCHESTRATOR_PROMPT.md`
8. This file (`docs/orchestration/WHOLE_TARGET_PLAN.md`)

## 9. Risks and mitigations

1. **Clean-room leakage.** Forbidden sources enter the repo via copy-paste or reference. Mitigation: `docs/provenance.md` is normative; every protocol-adjacent commit body contains the clean-room attestation; pull request template has the attestation checkboxes.
2. **Test suite becomes maintenance burden.** Too many tests, redundant coverage, tests that fail on irrelevant refactors. Mitigation: the quality bar in §5; every test earns its place; tests target public API behavior not implementation; regular review during slice reviews.
3. **Scope creep into v2 features inside slice 11.** Mitigation: `docs/slices/11-core.md` enumerates the read-only scope explicitly; anything else is out of scope and must land as a separate slice.
4. **Dogfood gap discovered too late.** An external consumer finds a missing primitive after slice 11 ships but before it can usefully build on the library. Mitigation: slice 11b exists as a named conditional slice specifically to handle this.
5. **Silent reconnect regression.** A change breaks the explicit reconnect contract without triggering a test. Mitigation: reconnect invariants live at layer 1 of the pyramid — small, exhaustive tests that fire on any state-machine change.

## 10. Locked decisions

- License: **MIT** (LICENSE file shipped in slice 10).
- CI: **GitHub Actions** (`.github/workflows/ci.yml`, green on initial push).
- Go version: **1.26**.
- Public root package: **`ibkr`**.
- Testhost approach: **in-process fake with checked-in deterministic transcripts**. Live Gateway only for transcript refresh.
- Commit convention: **§3 above**, applies to every commit.
- Repo visibility: **currently private, flips to public manually by owner once v1 is in place**.
- No `EWrapper` / `EClient` shape as the primary v1 public design. A `compat/eclient/` package is deferred until after explicit legal review.
- Dependencies: **zero non-stdlib deps at day 1**. Any addition requires an entry in `docs/dependencies.md` (create on first add) and rationale in the commit body.

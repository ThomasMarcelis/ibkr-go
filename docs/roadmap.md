# Roadmap

## Direction

`ibkr-go` is an idiomatic, pure-Go client for the Interactive Brokers
TWS/Gateway socket protocol. It covers the full free read-only surface plus
order management, market depth, and option exercise through a typed Go facade.
The production import path has no SDK, cgo, or platform toolchain dependency.

Live Gateway behavior is authoritative. Official client source and SDK runs
may supply conformance vectors for the same live scenario, but never become a
runtime engine or a substitute for live evidence.

## Current state

The public facade and production codec cover the in-scope socket API across
exactly `server_version` 208–225. The executable catalog has 124 scenarios:
103 promoted from live-derived evidence, no candidates, and 21 explicitly
blocked by the current entitlements, account types, market state, or
Gateway-only setup. The deterministic corpus contains 113 provenance-checked
transcripts.

The decoder ledger partitions all 106 registered layout pairs. Eighty-six
have positive raw-frame attestation; 20 implemented layouts await callbacks
that the current environment cannot produce. A typed blocker response does
not count as positive decoder evidence.

The major domains are implemented:

- session bootstrap, readiness, reconnect, interruption, and version gates;
- account summaries and updates, positions, PnL, family codes, executions,
  fees, and completed orders;
- contracts, qualification, security definitions, matching symbols, market
  rules, smart components, and depth exchanges;
- quote snapshots and streams, generic/string/option/news updates, real-time
  bars, historical bars/ticks/schedules, and reroutes;
- placement, replacement, cancellation, brackets, OCA, algos, conditions,
  scale, hedge, combo/BAG, what-if, and guarded campaign reconciliation;
- option calculations and accepted-but-unsettled exercise admission;
- news, scanner, FA reads, WSH, display groups, user info, and read-only
  configuration.

[`live-coverage-matrix.md`](live-coverage-matrix.md) owns capability status,
[`ibkr-api-inventory.md`](ibkr-api-inventory.md) owns the official surface,
and [`message-coverage.md`](message-coverage.md) owns message implementation.

## Supported protocol train

Production accepts exactly versions 208 through 225 and rejects 207 and 226.
Families already migrated at the floor use one protobuf implementation;
reachable classic variants remain only through their real transition. The
exact negotiation replay covers every supported version, and native or
official-SDK vectors freeze each implemented migration boundary.

The current official source audit is API 10.50.01. Relative to 10.48.01 it
adds:

- `ContractDetails.settlementMethod` at protobuf field 65, implemented and
  replayed from sv225 AAPL option and ES future callbacks;
- `Order.conditionsIncludeOvernight` at field 145, gated by server version
  226 and therefore intentionally outside the supported range.

Historical API 10.48.01 references remain where they identify the SDK version
that produced an existing boundary vector. See
[`protocol-audit-sv208-225.md`](protocol-audit-sv208-225.md).

## Evidence boundaries

Remaining work is environmental attestation, not a second implementation:

- positive depth, tick-by-tick, real-time-bar, EFP, delta-neutral, odd-lot,
  histogram, historical-update, and historical-tick callbacks require market
  data or historical entitlements unavailable on the current accounts;
- positive FA and WSH callbacks require those account products;
- bulletin attestation requires an event during a bounded subscription;
- `orderBound` requires paper TWS and a user-created manual order while the
  recorder is armed;
- the retained option-exercise replay proves exact warning 10349 and
  `PreSubmitted` admission followed by an uncertain disconnect. It makes no
  settlement or lapse claim, and the instruction must not be repeated merely
  to seek one.

The current catalog disposition and exact decoder rows are recorded in the
[`v2.0.2 coverage plan`](v2.0.2-coverage-plan.md).

## Maintainer workflow

1. Check the catalog and coverage row before creating another scenario.
2. Prefer promoting a verified capture over recording another live request.
3. Keep a change vertical: public behavior, protocol shape, live-derived
   regression, and evidence docs together.
4. Use `readonly-live` at `127.0.0.1:4001` only for read-only work and
   `paper-dev` at `127.0.0.1:4002` for every order mutation.
5. Preserve the paper baseline and never repeat the consumed regulatory
   snapshot or option-exercise instruction.
6. Run the deterministic candidate gate in the coverage plan before release.

## Ongoing

- Audit official SDK releases for schema drift within the supported version
  range.
- Promote an externally blocked callback only when real positive evidence
  becomes available.
- Improve API ergonomics and examples when they simplify real use.

## Not planned

- Client Portal Web API or Flex.
- An `EWrapper` / `EClient` compatibility bridge.
- Server-log, verification/auth, redirect, or operator-configuration mutation.
- The official SDK as a runtime dependency.

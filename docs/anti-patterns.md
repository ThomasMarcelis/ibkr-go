# Anti-Patterns

Patterns `ibkr-go` deliberately rejects, with the reasoning behind each rejection. This list is normative for maintainers reviewing pull requests.

## Callback inheritance as the primary public API

The official Interactive Brokers client exposes behavior through an `EWrapper` callback interface that users are expected to implement (or inherit from) and an `EClient` they drive. This shape forces application code to be structured around the library's event dispatching rather than around its own domain. It also makes typed request/response flows awkward: every call site has to correlate a later callback fire with the earlier request via request IDs.

`ibkr-go` rejects this as the primary public design. Typed one-shot request methods and typed subscriptions own the public surface. Callback registration, if it ever appears, is an opt-in escape hatch, not the default shape.

## Thin wrappers over the official shape

Several existing Go libraries wrap the official callback shape in a thinner ergonomic layer. The result inherits the correctness and state-machine concerns of the layer below without solving them, and the typed API on top cannot be stronger than the callback layer it depends on.

`ibkr-go` rejects wrapping. The library implements the protocol directly. Typed public API and internal state machine are both owned end-to-end.

## Implicit global state

Libraries that maintain a package-level singleton client, package-level logger, or package-level connection registry push ambiguity into every call site. Tests become brittle, parallelism becomes impossible without workarounds, and shutdown ordering becomes invisible.

`ibkr-go` rejects implicit global state. Every client is a concrete value constructed explicitly. Loggers, clocks, and network dialers are passed in through functional options. The library has no package-level mutable state.

## Silent reconnect

A client that silently reconnects without signaling the event to the caller hides a state change that may break the caller's invariants. Subscriptions may lose events, ordering may change, and the caller cannot decide how to recover.

`ibkr-go` rejects silent reconnect. Session state changes are observable through explicit contract events. Per-subscription resume policy is explicit. If the caller does not want automatic reconnect, it can opt out; if it does, it knows when reconnect happened and what was lost.

## Live-only CI

When the only way to exercise a protocol library's tests is against a live Interactive Brokers Gateway, CI becomes fragile, contributors are locked out unless they have credentials, and regressions go uncaught until a human runs the tests manually.

`ibkr-go` rejects live-only CI as the default path. Deterministic replay remains necessary for routine verification, but protocol-adjacent design and manual verification are expected to use the local live Gateway or TWS when available. The `testing/testhost` package is replay tooling for live-derived behavior, not a substitute protocol source.

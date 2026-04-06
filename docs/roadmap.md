# Roadmap

## v1 scope: read-only production core

- Connect, handshake, negotiated server version.
- Session state as a first-class, observable contract.
- Managed accounts.
- Account values and summary.
- Positions.
- Contract details and qualification.
- Quote snapshots and streaming.
- Real-time bars.
- Historical bars.
- Execution and open-order observation (read-only).
- Reconnect and resume behavior with explicit per-subscription policy.

## Current Status

The v1 read-only production core is complete. The implementation connects to
real IB Gateway, handles all v1 message types listed above, and has
comprehensive test coverage including grounded replay fixtures derived from
live server_version 200 captures.

## Explicitly not in v1

- Order writes (placement, modification, cancellation).
- Scanners and news breadth.
- Client Portal Web API.
- Flex.
- Near-full parity with the entire TWS API surface.
- `EWrapper` / `EClient` official-style bridge.

## Public API direction

Root package is `ibkr`. Primary shapes are typed one-shot request methods and
typed subscriptions, with explicit session info and explicit subscription
lifecycle. Subscriptions expose `Events() <-chan T`, `State() <-chan
SubscriptionStateEvent`, `Done() <-chan struct{}`, `Wait() error`, and
`Close() error`.

## v1 exit criteria

- Fake-host and transcript-based correctness coverage across invariants, state transitions, behavioral scenarios, and stress/edge cases.
- No live Gateway required for routine CI.
- Reconnect and session semantics are explicit and tested.
- Typed public API is stable enough for downstream consumption.
- A real read-only broker sync flow can be built on the library without custom protocol hacks.

## Post-v1 Direction

1. Order writes (placement, modification, cancellation).
2. Deeper contract detail fields and additional tick types.
3. Fuzz testing of the codec and frame parser.
4. Broader server version testing beyond v200.

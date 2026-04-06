# Architecture

`ibkr-go` is built as a session engine with a typed facade. The library does
not expose an `EWrapper` / `EClient` callback surface as its primary model.

Current repo truth:

- public/session/subscription contracts are frozen in code
- the current testhost and transcript harness are real and test-backed, but
  they are legacy replay tooling rather than a protocol design source
- the current codec is still a repo-local symbolic protocol model pending
  replacement by live-derived real IBKR message mapping
- live TWS / IB Gateway protocol compatibility is the next major milestone

## Layers

- `ibkr/`: public typed facade
- `internal/session/`: session engine, lifecycle, correlation, reconnect, and
  subscription management
- `internal/transport/`: socket dial, frame read/write loops, pacing
- `internal/codec/`: typed message encode/decode
- `internal/wire/`: frame and field framing
- `testing/testhost/`: deterministic replay and fault-injection harness for
  checked-in fixtures

The layer split is stable. The protocol details inside `codec`, `session`, and
`testhost` are expected to evolve as the legacy symbolic path is replaced with
real IBKR wire behavior derived from live traces and official docs.

## Runtime Model

- One session actor goroutine owns mutable state.
- One reader goroutine reads frames and forwards decoded messages to the actor.
- One writer goroutine serializes outbound frames and applies global pacing.
- Public methods talk to the actor through typed commands instead of sharing
  mutable maps or callback registries.

## Protocol Realities

- Request correlation is split between keyed flows and singleton flows. Not all
  protocol areas route cleanly through one `reqID -> channel` map.
- Snapshot completion is driven by explicit protocol end markers, never by
  silence or timeouts.
- Global pacing belongs in the write path. Endpoint-specific admission limits
  belong at the session layer.
- Managed accounts, negotiated server version, and next valid id are bootstrap
  state, not ordinary request/response calls.

## Public Direction

- `DialContext` returns a ready session, not a raw TCP socket.
- Managed accounts are bootstrap state on the session snapshot.
- One-shots and subscriptions are separate public contracts.
- Subscriptions expose business events through `Events()` and lifecycle through
  `State()`.

These public contracts are intended to survive the remaining protocol work.

## Reconnect

- Reconnect policy is a client policy.
- Resume policy is a per-subscription policy.
- One-shots are never replayed automatically.
- Session reconnect boundaries are surfaced via `ConnectionSeq`.

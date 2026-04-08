# Architecture

`ibkr-go` is built as a session engine with a typed facade. The library does
not expose an `EWrapper` / `EClient` callback surface as its primary model.

The library covers the full free read-only TWS API surface plus order
management, market depth, fundamental data, and option exercise. Public
contracts, codec, and replay fixtures are validated against live IB
Gateway server_version 200.

## Layers

- `ibkr/`: public typed facade
- `internal/session/`: session engine, lifecycle, correlation, reconnect, and
  subscription management
- `internal/transport/`: socket dial, frame read/write loops, pacing
- `internal/codec/`: typed message encode/decode
- `internal/wire/`: frame and field framing
- `testing/testhost/`: deterministic replay and fault-injection harness for
  checked-in fixtures

## Runtime Model

- One session actor goroutine owns mutable state.
- One reader goroutine reads frames and forwards decoded messages to the actor.
- One writer goroutine serializes outbound frames and applies global pacing.
- Public methods talk to the actor through typed commands instead of sharing
  mutable maps or callback registries.

## Routing Tables

The engine maintains three routing tables, each serving a different dispatch
pattern:

- **Keyed (`map[int]*route`)** — request-ID-correlated flows. One-shots and
  keyed subscriptions (account summary, quotes, historical bars, market depth,
  fundamental data, etc.) register a route keyed by `reqID`. Inbound messages
  carry the same `reqID` and dispatch directly to the registered handler.

- **Singleton (`map[string]*route`)** — flows that have at most one active
  instance and no request-ID correlation. Positions, open orders, family codes,
  news bulletins, and other singleton flows are keyed by a string constant.
  Inbound messages dispatch by message type to the matching singleton key.

- **Orders (`map[int64]*orderRoute`)** — per-order lifecycle tracking. Each
  placed order registers a route keyed by `orderID`. OpenOrder, OrderStatus,
  Execution, and CommissionReport messages dispatch to the matching order route.

### Dual dispatch for OpenOrder and OrderStatus

OpenOrder and OrderStatus messages are dispatched to both the per-order handle
(if one exists in the orders table) and the singleton open-orders observer (if
one is registered). This means a caller watching open orders via
`SubscribeOpenOrders` sees all order updates, while a caller holding a specific
`OrderHandle` sees only updates for that order. Neither blocks the other.

## Order ID Management

Order IDs are auto-allocated from `NextValidID`, which is received during
bootstrap and tracked on `SessionSnapshot`. Each `PlaceOrder` call increments
the counter atomically within the actor goroutine. Callers never need to manage
order IDs manually.

## OrderHandle Lifecycle

`PlaceOrder` returns an `OrderHandle` that tracks a single order's lifecycle:

- **Events()** delivers `OrderEvent` values (union of OpenOrder, OrderStatus,
  Execution, CommissionReport — exactly one field non-nil per event).
- **State()** delivers `SubscriptionStateEvent` values (Gap, Resumed).
- **Terminal states.** When an OrderStatus arrives with status Filled,
  Cancelled, or Inactive, the handle auto-closes with `nil` error.
- **Disconnect.** On session disconnect, active order handles receive a `Gap`
  event via State(). On reconnect, they receive `Resumed`. Handles are not
  closed on disconnect — orders continue executing on the server.
- **Close()** detaches the handle from the engine. The order continues
  executing on the server; the caller simply stops receiving events.
- **Cancel(ctx)** sends a CancelOrder request for this order.
- **Modify(order)** sends a modified PlaceOrder with the same OrderID.

## Protocol Realities

- Request correlation is split between keyed flows, singleton flows, and
  order flows. Not all protocol areas route cleanly through one
  `reqID -> channel` map.
- Snapshot completion is driven by explicit protocol end markers, never by
  silence or timeouts.
- Global pacing belongs in the write path. Endpoint-specific admission limits
  belong at the session layer.
- Managed accounts, negotiated server version, and next valid id are bootstrap
  state, not ordinary request/response calls.

## Public Direction

- `DialContext` returns a ready session, not a raw TCP socket.
- Managed accounts are bootstrap state on the session snapshot.
- One-shots, subscriptions, and order handles are separate public contracts.
- Subscriptions expose business events through `Events()` and lifecycle through
  `State()`. OrderHandle follows the same shape with order-specific extensions.

These public contracts are intended to survive the remaining protocol work.

## Reconnect

- Reconnect policy is a client policy.
- Resume policy is a per-subscription policy.
- One-shots are never replayed automatically.
- Order handles survive disconnects (Gap on disconnect, Resumed on reconnect).
- Session reconnect boundaries are surfaced via `ConnectionSeq`.

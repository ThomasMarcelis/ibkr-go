# Session Contract

This document freezes the public contract. Internal codec and transcript
plumbing may change as long as this public surface and its semantics do not.

## Session

`DialContext` returns only after transport connection, server-version
negotiation, bootstrap, managed-account loading, and transition to `Ready`.

```go
type Client struct{ /* opaque */ }

func DialContext(ctx context.Context, opts ...Option) (*Client, error)

func (c *Client) Close() error
func (c *Client) Done() <-chan struct{}
func (c *Client) Wait() error
func (c *Client) Session() Snapshot
func (c *Client) SessionEvents() <-chan Event
```

Session states are `Disconnected`, `Connecting`, `Handshaking`, `Ready`,
`Degraded`, `Reconnecting`, and `Closed`. `ConnectionSeq` increments each time a
fresh handshake reaches `Ready`. `SessionEvents()` is bounded and observational:
if unread, older queued events may be dropped in favor of the latest transition.

## Domain Facades

The root `Client` owns one shared session engine and exposes concrete domain
facades: `Accounts`, `Contracts`, `MarketData`, `History`, `Orders`, `Options`,
`News`, `Scanner`, `Advisors`, `WSH`, and `TWS`. These facades are namespaces
only; they do not create independent connections.

Managed accounts are bootstrap state on `Snapshot`, not a request-shaped
API.

## Subscriptions

```go
type Subscription[T any] struct {
    Events() <-chan T
    Lifecycle() <-chan SubscriptionStateEvent
    AwaitSnapshot(ctx context.Context) error
    Done() <-chan struct{}
    Wait() error
    Close() error
}
```

`Events()` carries business data only. `Lifecycle()` carries lifecycle only and
is bounded/observational: if unread, older queued lifecycle events may be
dropped in favor of the latest one. `SubscriptionClosed` is still guaranteed
before the lifecycle channel closes.

`AwaitSnapshot(ctx)` is durable for snapshot-style subscriptions. It returns
`nil` once `SnapshotComplete` has occurred, even if the lifecycle event was
dropped from the bounded channel. It returns `ErrNoSnapshot` for streams with no
snapshot boundary.

Lifecycle event kinds:

- `Started`
- `SnapshotComplete`
- `Gap`
- `Resumed`
- `Closed`

Default subscription behavior:

- bounded event queue
- close on slow consumer
- no implicit replay
- `ResumeAuto` is currently supported only for quote streams and real-time bars
- account summary, positions, open orders, account updates, multi-account
  snapshots, and live historical bars expose explicit snapshot boundaries

## OrderHandle

```go
type OrderHandle struct{ /* opaque */ }

func (h *OrderHandle) OrderID() int64
func (h *OrderHandle) Events() <-chan OrderEvent
func (h *OrderHandle) Lifecycle() <-chan SubscriptionStateEvent
func (h *OrderHandle) Done() <-chan struct{}
func (h *OrderHandle) Wait() error
func (h *OrderHandle) Close() error
func (h *OrderHandle) Cancel(ctx context.Context) error
func (h *OrderHandle) Modify(ctx context.Context, order Order) error
```

`Orders().Place` returns an `OrderHandle` that tracks a single order's
lifecycle. `Events()` delivers `OrderEvent` values. `OrderEvent` is a union:
exactly one of `OpenOrder`, `Status`, `Execution`, or `Commission` is non-nil
per event.

`Lifecycle()` delivers Gap and Resumed events across reconnect boundaries. It is
bounded and observational. `Close()` detaches the handle without cancelling the
server-side order. `Cancel(ctx)` sends a cancel request. `Modify(ctx, order)`
sends a modified order with the same OrderID.

Terminal states: when an OrderStatus arrives with status Filled, Cancelled, or
Inactive, the handle auto-closes with `nil` error.

## Completion and Reconnect

- One-shots complete only on explicit protocol completion markers.
- Snapshot-style subscriptions surface completion through `Lifecycle()` and
  `AwaitSnapshot`.
- Execution reports are modeled as `Orders().Executions(ctx, filter)`, a finite
  query, not a public subscription.
- Reconnect boundaries are explicit through `Event` and
  `SubscriptionStateEvent`, never mixed into business event streams.
- One-shots are interrupted by connection loss and are not replayed
  automatically.

## Errors and Types

Public error taxonomy:

- `*ConnectError`
- `*ProtocolError`
- `*APIError`
- `ErrNotReady`
- `ErrInterrupted`
- `ErrResumeRequired`
- `ErrNoSnapshot`
- `ErrSlowConsumer`
- `ErrUnsupportedServerVersion`
- `ErrClosed`

Numeric and payload types:

- Decimal-like values use exact `Decimal`.
- Instants use `time.Time`.
- Historical bar durations and bar sizes use `HistoricalDuration` and `BarSize`.
- Raw external XML/JSON boundaries use `XMLDocument` and `JSONDocument`.
- Stable protocol vocabularies use named types and constants instead of
  anonymous strings or ints where the vocabulary is stable.

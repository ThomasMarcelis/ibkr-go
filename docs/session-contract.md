# Session Contract

This document freezes the public contract. Internal codec and transcript
plumbing may change as long as this public surface and its semantics do not.

## Session

`DialContext` returns only after transport connection, server-version
negotiation, bootstrap, managed-account loading, and transition to `Ready`.
The client negotiates `server_version` 176..200; the Gateway's answer below
176 is rejected during handshake, and wire fields introduced above 176 are
gated on whatever version the Gateway actually negotiates. `sv200` remains
the primary live-validated layout.

```go
type Client struct{ /* opaque */ }

func DialContext(ctx context.Context, opts ...Option) (*Client, error)

func (c *Client) Close() error
func (c *Client) Done() <-chan struct{}
func (c *Client) Wait() error
func (c *Client) Session() Snapshot
func (c *Client) SessionEvents() <-chan Event
func (c *Client) CurrentTime(ctx context.Context) (time.Time, error)
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
    All(ctx context.Context) iter.Seq[T]
    AwaitSnapshot(ctx context.Context) error
    Done() <-chan struct{}
    Err() error
    Wait() error
    Close() error
}
```

`All(ctx)` is the canonical consumption loop: ranging over it drains `Events()`
to exhaustion (or until `ctx` ends), so `Err()`/`Wait()` are final by the time
the loop exits. It is equivalent to the drain-then-`Wait()` pattern described
below, without the caller managing channel state directly.

`Events()` carries business data only. `Lifecycle()` carries lifecycle only and
is bounded/observational: if unread, older queued lifecycle events may be
dropped in favor of the latest one. `SubscriptionClosed` is still guaranteed
before the lifecycle channel closes. `SubscriptionGap` and `SubscriptionClosed`
events include `Retryable`; callers that read only `Events()` should inspect
`sub.Err()` or `sub.Wait()` with `IsRetryable(err)` after the channel closes.
`Err()` does not wait for `Done()` and returns nil until a terminal close reason
is known.

`Events()` closes before `Done()`. Consumers that need every buffered business
event must drain `Events()` until it closes, then call `Wait()`. `Done()` is for
completion coordination and must not replace event draining.

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

Retryability:

- transport/session gaps are retryable
- `ErrInterrupted` and `ErrResumeRequired` closes are retryable
- `*APIError` closes are terminal request rejections and are not retryable

Default subscription behavior:

- bounded event queue
- close on slow consumer
- no implicit replay
- `ResumeAuto` is currently supported only for quote streams and real-time bars
- account summary, positions, open orders, account updates, multi-account
  snapshots, and live historical bars expose explicit snapshot boundaries

## Order Submission

`Orders().Place(ctx, req)` sends a live order and returns an `OrderHandle`.
It validates the contract, core order fields, and structural relationships in
advanced order settings before anything reaches the wire. A request whose
`Order.WhatIf` is set is rejected with a `*ValidationError` pointing at
`Orders().Preview` — a what-if request is a margin/commission query, not a
trade, and does not fit the `OrderHandle` lifecycle contract.

`Orders().PlaceBracket(ctx, req)` allocates the parent, take-profit, and
stop-loss IDs in one actor turn and owns their `ParentID` and `Transmit`
fields. It sends the orders in parent/child/child order with the live-attested
false/false/true transmit sequence. The result contains one `OrderHandle` per
leg. If setup is interrupted after any leg reaches the send queue, the engine
best-effort cancels every sent leg and closes all three routes.

`Orders().Preview(ctx, req)` is the one-shot counterpart: it forces the
what-if flag and returns an `OrderState` (the nine margin decimals plus the
commission range and currency) with no `OrderHandle` and nothing resting on
the server.

## OrderHandle

```go
type OrderHandle struct{ /* opaque */ }

func (h *OrderHandle) OrderID() int64
func (h *OrderHandle) Events() <-chan OrderEvent
func (h *OrderHandle) Lifecycle() <-chan SubscriptionStateEvent
func (h *OrderHandle) Done() <-chan struct{}
func (h *OrderHandle) Wait() error
func (h *OrderHandle) Close() error
func (h *OrderHandle) Cancel(ctx context.Context, opts ...CancelOption) error
func (h *OrderHandle) Modify(ctx context.Context, order Order) error
```

`Orders().Place` returns an `OrderHandle` that tracks a single order's
lifecycle. `Events()` delivers `OrderEvent` values. `OrderEvent` is a union:
exactly one of `OpenOrder`, `Status`, `Execution`, `CommissionAndFees`, or
`Warning` is non-nil per event.

`Lifecycle()` delivers Gap and Resumed events across reconnect boundaries. It is
bounded and observational. `Close()` detaches the handle without cancelling the
server-side order. `Cancel(ctx)` sends a cancel request; compliance workflows
can attach the manual cancel time, external operator, and manual-order
indicator through `CancelOption`. `Modify(ctx, order)` sends a modified order
with the same OrderID.

`Events()` closes before `Done()`. Consumers that need every order event,
including late `Execution` or `CommissionAndFees` callbacks after a terminal status,
must drain `Events()` until it closes, then call `Wait()`.

Terminal states: when an OrderStatus arrives with status Filled, Cancelled,
ApiCancelled, or Inactive, the handle auto-closes with `nil` error. An
order-targeted api_error that rejects the placement outright (no order_status
ever follows) closes the handle with the `*APIError` as its terminal error;
this covers both the sub-10000 rejection codes and the 10xxx codes
live-attested as outright placement rejections (10063, 10255). Order-targeted
10xxx notices for live orders — cancel replies such as 10147/10148 — stay
session events, because the handle already carries the order's real state.

An order-targeted api_error whose `(&APIError{Code: ...}).IsWarning()` is true —
notably code 399, the off-hours deferral — is delivered non-terminally as an
`OrderEvent.Warning`. The order stays working at IB and the handle stays open;
its real lifecycle (later status updates, the eventual terminal close)
continues.

## Completion and Reconnect

- One-shots complete only on explicit protocol completion markers.
- Snapshot-style subscriptions surface completion through `Lifecycle()` and
  `AwaitSnapshot`.
- Execution reports are modeled as `Orders().Executions(ctx, filter)`, a finite
  query, not a public subscription. It completes on IBKR's execution-details
  end marker; commission-and-fees reports are independent ExecID-correlated
  messages and may be unset, revised, or arrive after that marker.
- Reconnect boundaries are explicit through `Event` and
  `SubscriptionStateEvent`, never mixed into business event streams.
- Calls submitted while the session is reconnecting wait for the next `Ready`
  transition or for their context to end; callers do not need to add their own
  client-wide mutex for bursty request sequences.
- One-shots are interrupted by connection loss and are not replayed
  automatically.
- A data-lost restoration (code 1101) is a connection loss for everything the
  Gateway cannot replay: auto-resumed subscriptions are re-sent; every other
  route applies its transport-loss teardown — non-resumable subscriptions
  close with `ErrResumeRequired`, in-flight one-shots and pending what-if
  previews resolve with `ErrInterrupted` (both retryable). A data-maintained
  restoration (code 1102) interrupts nothing. Live order handles survive both:
  orders rest at IB, not on the Gateway's data connection.
- Historical bars and schedules use internal endpoint admission so rapid
  repeated requests respect Gateway pacing before they are written to the
  socket.

## Errors and Types

Public error taxonomy:

- `*ConnectError`
- `*ProtocolError`
- `*APIError`
- `IsRetryable(err)`
- `ErrNotReady`
- `ErrInterrupted`
- `ErrResumeRequired`
- `ErrNoSnapshot`
- `ErrSlowConsumer`
- `ErrUnsupportedServerVersion`
- `ErrClosed`

Numeric and payload types:

- Decimal-like values use `decimal.Decimal` from `github.com/shopspring/decimal`.
- Instants use `time.Time`.
- Historical bar durations and bar sizes use `HistoricalDuration` and `BarSize`.
- Raw external XML/JSON boundaries use `XMLDocument` and `JSONDocument`.
- Stable protocol vocabularies use named types and constants instead of
  anonymous strings or ints where the vocabulary is stable.

# Session Contract

This document freezes the public contract. Internal codec and transcript
plumbing may change as long as this public surface and its semantics do not.

## Session

`DialContext` returns only after transport connection, server-version
negotiation, bootstrap, managed-account loading, and transition to `Ready`.
The client negotiates `server_version` 200..207; answers outside that range
are rejected during handshake. `sv200` is the classic live-validated layout;
exact `sv201` adds the raw-ID envelope and
protobuf executions family, while exact `sv202` adds zero-strike contract
semantics without migrating another message family. Exact `sv203` migrates
place order, targeted cancel, global cancel, and the corresponding
open-order/status callbacks to protobuf. Order errors observed in that flow use
the protobuf error family introduced at 201. Exact `sv204` migrates the three
open-order requests, completed-order request, and completed-order result/end
pair. Exact `sv205` migrates contract-details requests and regular, bond, and
end responses to protobuf while retaining the same typed public operation.
Exact `sv206` migrates market-data, market-depth, and market-data-type requests
and their quote/depth callbacks. Exact `sv207` migrates managed accounts,
account updates and summaries, positions, and their multi-account variants to
protobuf without changing the public operations. Versions 208 and newer are
not advertised.

```go
type Client struct{ /* opaque */ }

func DialContext(ctx context.Context, opts ...Option) (*Client, error)

func (c *Client) Close()
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

`CurrentTime` and `CurrentTimeMillis` share a 4.25-second admission gate because
live Gateway suppresses closely spaced clock requests. The call context bounds
both the gate wait and the reply. A cancellation after a request-ID-less clock
request was admitted retires that exact connection generation; automatic
reconnect replaces it when enabled, while `ReconnectOff` closes the client.

## Domain Facades

The root `Client` owns one shared session engine and exposes concrete domain
facades: `Accounts`, `Contracts`, `MarketData`, `History`, `Orders`, `Options`,
`News`, `Scanner`, `Advisors`, `WSH`, and `TWS`. These facades are namespaces
only; they do not create independent connections.

Managed accounts are loaded into `Snapshot` during bootstrap and can be
refreshed explicitly with `Client.ManagedAccounts`.

Operation methods are the canonical validation boundary; v2 intentionally has
no separate public `Validate` facade. Static caller mistakes return
`*ValidationError` before a frame is sent, while negotiated-version, client-ID,
account, entitlement, and ownership checks remain session-dependent. A zero
`Client` is not a validation harness. Applications that need template
compatibility tests should exercise the real operation through a deterministic
`Dialer` or Gateway replay and assert `ValidationError.Field` where relevant.

## Subscriptions

```go
type Subscription[T any] struct {
    Events() <-chan StreamEvent[T]
    All(ctx context.Context) iter.Seq[T]
    AwaitSnapshot(ctx context.Context) error
    Done() <-chan struct{}
    Err() error
    Wait() error
    Close()
}
```

`Events()` is the single ordered queue for data and lifecycle boundaries. Its
event kinds are `Started`, `Data`, `SnapshotComplete`, `Gap`, `Restored`, and
`Resubscribed`. `Restored` means the Gateway retained the remote stream (1102);
`Resubscribed` means the client physically sent the request again. Channel
close is terminal; inspect `Err()` or `Wait()` for its cause. There is no
redundant `Closed` event. Every event's `At` is the UTC time the client enqueued
it; this is local observation time, not Gateway event time.

`All(ctx)` filters `Data` values from that same queue. Ranging it to exhaustion
drains every buffered event, so `Err()`/`Wait()` are final when the loop exits.
`Events()` and `All()` are alternative consumers and must not be read
concurrently. `Err()` does not wait for `Done()` and returns nil until a
terminal close reason is known.

`Events()` closes before `Done()`. Consumers that need every buffered business
event must drain `Events()` until it closes, then call `Wait()`. `Done()` is for
completion coordination and must not replace event draining.

`AwaitSnapshot(ctx)` is durable for snapshot-style subscriptions. It returns
`nil` once `SnapshotComplete` has occurred, even if the lifecycle event was
dropped from the bounded channel. It returns `ErrNoSnapshot` for streams with no
snapshot boundary.

Transport-queue admission is the ownership boundary for subscription setup.
Once the subscribe frame is admitted, its handle result wins caller-context and
client-shutdown races; before admission, setup returns an error and no handle.
The establishment context remains bound to the returned handle, so a context
that raced admission initiates cancellation immediately without hiding the
handle. `Wait()` preserves the context's exact cancellation cause.

`Close` initiates cancellation asynchronously and remains idempotent. `Wait`
and `Err` report `*SubscriptionCancelError` when
the cancel frame cannot enter the active transport queue. That error is
non-retryable. The client retires that connection generation automatically, so
with `ReconnectAuto` wait for the replacement session to become `Ready` before
creating a replacement subscription. Unrelated `ResumeAuto` streams survive
onto that replacement. With `ReconnectOff`, the client closes. A nil close
result means the cancel frame entered the queue, not that IBKR acknowledged it.
Closing a route retained after transport loss, or before it was resumed on a
replacement connection, is a clean local detach because no current connection
hosts that remote stream.

Slow-consumer shutdown uses the same cancellation path. If cancellation enters
the active transport queue, that transport is already dead, or another terminal
teardown wins the race, `Wait` and `Err` report
exactly `ErrSlowConsumer`. Only when the active transport refuses cancellation
admission do all three report the same joined error:
`errors.Is(err, ErrSlowConsumer)` remains true and
`errors.AsType[*SubscriptionCancelError](err)` exposes the uncertain remote
stream. The joined result is not retryable.

Ordered event kinds:

- `Started`
- `Data`
- `SnapshotComplete`
- `Gap`
- `Restored`
- `Resubscribed`

Retryability:

- `ErrNotReady`, `ErrInterrupted`, and `ErrResumeRequired` are retryable
- transient `*ConnectError` values are retryable
- API pacing violations are retryable with backoff; other `*APIError` values
  are terminal by default
- caller context cancellation, protocol/validation failures,
  `ErrSlowConsumer`, and `ErrClosed` are not retryable
- `*SubscriptionCancelError` is not retryable even when it wraps
  `ErrInterrupted`
- `*ExerciseUncertainError` is not retryable even when it wraps a transient
  connection cause
- `*OrderRecoveryError` is not retryable even when it wraps a transient cause

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
advanced order settings before anything reaches the wire. The operation owns
the wire-level what-if flag: `Place` and `OrderHandle.Replace` submit live
orders, while `Orders().Preview` performs the margin/commission query without
creating an `OrderHandle`.

Transport-queue admission is the placement ownership boundary. Once the place
frame is admitted, its buffered success result wins races with `ctx.Done()` and
engine shutdown: `Place` returns the handle and `nil`, even if concurrent
shutdown has already closed that handle. Before admission, it returns an error
and no handle. This keeps the result unambiguous; a live order is never hidden
behind a context error. The transport tracks each admitted order frame until
the socket write completes. If the connection dies while that frame is still
unwritten, the handle closes with `ErrInterrupted`: IBKR never received it.

`Orders().PlaceBracket(ctx, req)` allocates the parent, take-profit, and
stop-loss IDs in one actor turn and owns their `ParentID` and `Transmit`
fields. It sends the orders in parent/child/child order with the live-attested
false/false/true transmit sequence. The result contains one `OrderHandle` per
leg. If a later place frame is not admitted, the engine sends cancellation for
exactly the already-admitted IDs under a fresh bounded context and closes all
three routes. Every partial bracket returns a zero bracket and
`*OrderRecoveryError`; `OrderIDs` contains every admitted leg because admitting
a compensating cancellation to the local queue does not prove IBKR processed
it. `CancelErr == nil` means all compensating cancellations entered that queue;
a non-nil value identifies cancellation-admission failures. `PlacementErr` and
`CancelErr` preserve their causes through `errors.Is`. `IsRetryable` is false
because the caller must reconcile open orders before deciding whether to place
again. Only failure before the first place admission returns the original
placement error directly.

`Orders().Preview(ctx, req)` is the one-shot counterpart: it forces the
what-if flag and returns the complete `OrderState` preview block, including
status, margin currency, before/change/after margin both in and outside regular
trading hours, commission-and-fees range/currency, suggested size, reject
reason, and allocation results. It creates no `OrderHandle` and leaves nothing
resting on the server.

## OrderHandle

```go
type OrderHandle struct{ /* opaque */ }

func (h *OrderHandle) OrderID() int64
func (h *OrderHandle) Events() <-chan OrderEvent
func (h *OrderHandle) Done() <-chan struct{}
func (h *OrderHandle) Wait() error
func (h *OrderHandle) Close()
func (h *OrderHandle) Cancel(ctx context.Context, opts ...CancelOption) error
func (h *OrderHandle) Replace(ctx context.Context, order Order) error
```

`Orders().Place` returns an `OrderHandle` that tracks a single order's
lifecycle. `Events()` delivers `OrderEvent` values. `OrderEvent` is a union:
exactly one of `OpenOrder`, `Status`, `Execution`, `CommissionAndFees`, or
`Warning`, `Binding`, or `Lifecycle` is non-nil per event. `OpenOrder.State`
preserves the margin, commission, allocation, and rejection details from the
same open-order frame.

The event queue is bounded and lossless, with a client-wide default capacity of
64 configured by `WithOrderEventBuffer`. It never silently drops an event while
continuing to observe. If the queue fills, the handle closes with
`ErrSlowConsumer`; this ends only local observation and does not prove that
IBKR stopped or cancelled the live order. `OrderID()` remains available as the
stable coordinate for open-order reconciliation and direct cancellation.

`OrderEvent.Lifecycle` carries `Started`, `Gap`, `Restored`, and
`RecoveryRequired` in order with business events. `RecoveryRequired` means the
connection gap may have hidden order changes; reconcile open orders,
executions, and completed orders for business decisions. `Replace` remains
permanently disabled on that handle because reconciliation cannot restore its
lost event history. `Close()` detaches the handle without cancelling the
server-side order. `Cancel(ctx)` sends a cancel request; compliance workflows
can attach the manual cancel time, external operator, and manual-order
indicator through `CancelOption`. `Replace(ctx, order)` sends a modified order
with the same OrderID.

`Options().Exercise` returns an `ExerciseHandle` once the request enters the
client transport queue. The handle correlates request warnings, errors, and any
pseudo-order lifecycle emitted under that request ID. Neither a returned handle
nor working-order evidence proves final exercise or lapse settlement. `Close`
only detaches observation. Connection loss while the instruction is unresolved
closes it with non-retryable `*ExerciseUncertainError`; reconcile the account or
position independently instead of blindly resubmitting.

v2 deliberately exposes no adopt-or-replace-by-ID operation for orders found
after a process restart. A fresh process can reconcile open orders, executions,
and completed orders and can cancel by stable order ID through
`Orders().Cancel`, but it must not synthesize an `OrderHandle` to replace a
pre-existing order. Binding, ownership, `PermID`, partial projections, and the
need to resend the complete contract and order make that unsafe without a
separately live-attested adoption protocol.

`Events()` closes before `Done()`. A Filled, Cancelled, APICancelled, or
Inactive status is an order-state event, not the end of local observation.
Execution and commission-and-fees callbacks may follow it. The caller owns the
observation window and must call `Close()` after collecting the evidence it
needs; `Close()` then drains already-buffered events before `Done()` closes.
`Wait()` reports the explicit local close, a request error, slow-consumer
failure, or disconnect.

An order-targeted api_error closes the handle only when
`APIError.IsOrderRejection()` identifies a live-attested outright placement
failure and no working-order evidence has appeared. Unknown codes and every
error after working evidence are delivered as non-terminal warnings: retaining
an observable live order is safer than detaching it. Order-targeted 10xxx
notices for live orders — cancel replies such as 10147/10148 — stay session
events, because the handle already carries the order's real state.
Cancellation replies 161 (the order is not in a cancellable state) and 202
(the order was cancelled) follow the same rule even though they are below
10000: they are session notices, never placement failures. A 161 may race
ahead of the terminal status it describes, so it must not tear down the route;
the subsequent `OrderStatus` still owns the handle result.

Other order-targeted api_error values — notably code 399, the off-hours
deferral, and code 404, an order held while shares are located — are delivered
non-terminally as `OrderEvent.Warning`. The order stays observable and the
handle remains open; later status updates continue until the caller closes the
observation window or an error ends it.

## Completion and Reconnect

- One-shots complete only on explicit protocol completion markers.
- Snapshot-style subscriptions surface completion through the ordered event
  stream and `AwaitSnapshot`.
- `Orders().Executions(ctx, filter)` returns an `ExecutionSnapshot` containing
  executions and every commission-and-fees report observed through IBKR's
  execution-details end marker. Because fees are independent ExecID-correlated
  messages and may arrive or be revised after that boundary,
  `SubscribeExecutions` keeps the route open until the caller closes it and
  emits `ExecutionUpdate` values for executions and fee reports. Each execution
  route retains at most 4096 distinct ExecIDs and 4096 fee-report versions that
  arrived before their execution by default; `WithExecutionCorrelationLimit`
  changes both bounds. At the execution-details end marker, unmatched fee
  reports are discarded. Later fee reports are accepted only for execution IDs
  observed in the completed snapshot, so unrelated global commission
  broadcasts cannot consume a finished route's correlation budget. The first
  callback that would exceed a bound is not emitted: the exact route is removed
  and closes with the non-retryable `ErrExecutionCorrelationOverflow`. The
  finite `Executions` collector uses the same default and also bounds each
  collected event family.
- Reconnect boundaries are explicit through session `Event`, `StreamEvent`,
  and `OrderEvent` values.
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
  restoration (code 1102) interrupts nothing. Live order handles survive both,
  but a reconnect that cannot prove the missing order state emits
  `RecoveryRequired`; callers reconcile for business decisions, while Replace
  stays disabled on that handle. Orders rest at IB, not on the Gateway's data
  connection.
- Historical bars and schedules use internal endpoint admission so rapid
  repeated requests respect Gateway pacing before they are written to the
  socket.

## Errors and Types

Public error taxonomy:

- `*ConnectError`
- `*ProtocolError`
- `*APIError`
- `*ValidationError`
- `*SubscriptionCancelError`
- `*ExerciseUncertainError`
- `*OrderRecoveryError`
- `IsRetryable(err)`
- `ErrNotReady`
- `ErrInterrupted`
- `ErrResumeRequired`
- `ErrNoSnapshot`
- `ErrSlowConsumer`
- `ErrUnsupportedServerVersion`
- `ErrClosed`
- `ErrNoMatch`
- `ErrAmbiguousContract`
- `ErrNoSubscription`
- `ErrOperationActive`

`*APIError` exposes `IsEntitlement`, `IsPacingViolation`,
`IsOrderRejection`, `IsConnectivityTransition`, `IsFarmStatus`, and
`IsWarning` for control-flow classification without text parsing. Generic HMDS
code 162 and real-time-query code 420 count as pacing only when their Gateway
message explicitly says so.

Numeric and payload types:

- Decimal-like values use `decimal.Decimal` from `github.com/shopspring/decimal`.
- Instants use `time.Time`.
- Historical bar durations and bar sizes use `HistoricalDuration` and `BarSize`.
- Raw external XML/JSON boundaries use `XMLDocument` and `JSONDocument`.
- Stable protocol vocabularies use named types and constants instead of
  anonymous strings or ints where the vocabulary is stable.

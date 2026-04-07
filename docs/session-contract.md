# Session Contract

This document freezes the public contract. It is intentionally stronger than
the current protocol implementation details. Internal codec and transcript
plumbing may change as long as this public surface and its semantics do not.

## Session State

`ibkr-go` exposes a small explicit session state machine:

- `Disconnected`
- `Connecting`
- `Handshaking`
- `Ready`
- `Degraded`
- `Reconnecting`
- `Closed`

`ConnectionSeq` increments each time a fresh handshake reaches `Ready`.

## Client

```go
type Client struct{ /* opaque */ }

func DialContext(ctx context.Context, opts ...Option) (*Client, error)

func (c *Client) Close() error
func (c *Client) Done() <-chan struct{}
func (c *Client) Wait() error

func (c *Client) Session() SessionSnapshot
func (c *Client) SessionEvents() <-chan SessionEvent
```

`DialContext` returns only after:

1. transport connected
2. negotiated server version known
3. bootstrap exchange completed
4. managed accounts loaded
5. the session reached `Ready`

This contract is implemented in code and validated against live IB Gateway
server_version 200.

## Subscriptions

```go
type Subscription[T any] struct {
    Events() <-chan T
    State() <-chan SubscriptionStateEvent
    Done() <-chan struct{}
    Wait() error
    Close() error
}
```

`Events()` carries business data only. `State()` carries lifecycle only.

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
- account summary, positions, open orders, and executions require explicit re-subscribe after disconnect
- `SubscribeExecutions` is a finite snapshot flow: after `SnapshotComplete`, it closes with `nil`
- account summary, positions, and open orders remain open after their initial snapshot boundary until the caller closes them or the session drops

## OrderHandle

```go
type OrderHandle struct{ /* opaque */ }

func (h *OrderHandle) OrderID() int64
func (h *OrderHandle) Events() <-chan OrderEvent
func (h *OrderHandle) State() <-chan SubscriptionStateEvent
func (h *OrderHandle) Done() <-chan struct{}
func (h *OrderHandle) Wait() error
func (h *OrderHandle) Close() error
func (h *OrderHandle) Cancel(ctx context.Context) error
func (h *OrderHandle) Modify(ctx context.Context, order Order) error
```

`PlaceOrder` returns an `OrderHandle` that tracks a single order's lifecycle.

`Events()` delivers `OrderEvent` values. `OrderEvent` is a union: exactly one
of `OpenOrder`, `Status`, `Execution`, or `Commission` is non-nil per event.

`State()` delivers lifecycle events (Gap on disconnect, Resumed on reconnect).

`Close()` detaches the handle from the engine. The order continues executing
on the server; the caller simply stops receiving events. This is not a cancel.

`Cancel(ctx)` sends a CancelOrder request for this order.

`Modify(order)` sends a modified PlaceOrder with the same OrderID.

Terminal states: when an OrderStatus arrives with status Filled, Cancelled, or
Inactive, the handle auto-closes with `nil` error.

## Managed Accounts

Managed accounts are bootstrap state. They live on `SessionSnapshot` and are
not modeled as a request-shaped API.

## Errors

Public error taxonomy:

- `*ConnectError`
- `*ProtocolError`
- `*APIError`
- `ErrNotReady`
- `ErrInterrupted`
- `ErrResumeRequired`
- `ErrSlowConsumer`
- `ErrUnsupportedServerVersion`
- `ErrClosed`

## Numeric and Time Types

- Decimal-like values use a concrete exact `Decimal` type.
- Instants use `time.Time`.
- Durations use `time.Duration`.

## Completion Semantics

- One-shots complete only on explicit protocol completion markers.
- Snapshot-style subscriptions surface the initial completion boundary through
  `State()`.
- Order handles auto-close on terminal OrderStatus (Filled, Cancelled,
  Inactive).
- Reconnect boundaries are explicit through `SessionEvent` and
  `SubscriptionStateEvent`, not mixed into business event streams.

## Stability Goal

This contract is stable.

## Full API Surface

All methods below are implemented, tested against live IB Gateway
server_version 200 captures, and frozen into the contract.

### Order management

- `PlaceOrder` — submit a new order, returns an `OrderHandle` that tracks its
  full lifecycle. Order IDs are auto-allocated from NextValidID.
- `CancelOrder` — cancel an order by ID (fire-and-forget; result arrives via
  the OrderHandle's events as an OrderStatus with status Cancelled).
- `GlobalCancel` — cancel all open orders (fire-and-forget; individual results
  arrive via active OrderHandle events).

### Account and portfolio

- `SubscribeAccountUpdates` / `AccountUpdatesSnapshot` — streaming per-holding
  portfolio values (singleton subscription with cancel).
- `SubscribeAccountUpdatesMulti` / `AccountUpdatesMultiSnapshot` — concurrent
  multi-account variant (keyed subscription with cancel).
- `SubscribePositionsMulti` / `PositionsMultiSnapshot` — positions filtered to
  a specific account or model (keyed subscription with cancel).
- `SubscribePnL` — real-time daily, unrealized, and realized P&L per account
  (keyed subscription with cancel, no snapshot boundary).
- `SubscribePnLSingle` — real-time P&L for a single position by conId (keyed
  subscription with cancel, no snapshot boundary).
- `FamilyCodes` — account family membership (singleton one-shot).
- `CompletedOrders` — all filled or cancelled orders (singleton one-shot with
  end marker).

### Contract and reference data

- `MatchingSymbols` — symbol search by prefix, up to 16 results (keyed
  one-shot).
- `MarketRule` — min price increment table for a market rule ID (singleton
  one-shot).
- `SecDefOptParams` — option chain parameters: expirations, strikes, exchanges,
  multipliers (keyed one-shot with end marker).
- `SmartComponents` — decode exchange abbreviations from SMART routing (keyed
  one-shot).

### Market data

- `SetMarketDataType` — switch to delayed or frozen data (fire-and-forget).
- `SubscribeMarketDepth` — Level 2 order book depth (keyed subscription with
  cancel). Receives both L1 depth (IN 12) and L2/SmartDepth (IN 13) updates.
  Requires paid L2 market data subscription.
- `SubscribeTickByTick` — individual tick-level data: Last, AllLast, BidAsk,
  MidPoint (keyed subscription with cancel, no snapshot boundary).
- `SubscribeHistoricalBars` — live-updating historical bars via keepUpToDate
  flag (keyed subscription with snapshot boundary, streaming IN 108 updates).

### Historical data extensions

- `HeadTimestamp` — earliest available data point for a contract (keyed
  one-shot, returned as a UTC `time.Time` parsed from IBKR's
  `YYYYMMDD-hh:mm:ss` wire format when `formatDate=1`).
- `HistogramData` — price distribution histogram (keyed one-shot).
- `HistoricalTicks` — tick-level Time and Sales, max 1000 per request (keyed
  one-shot, with tick instants parsed from epoch-second wire values).

### Option calculations

- `CalcImpliedVolatility` — server-side IV from option and underlying prices
  (keyed one-shot).
- `CalcOptionPrice` — server-side theoretical price from vol and underlying
  (keyed one-shot).

### Fundamental data

- `FundamentalData` — Reuters fundamental data XML (keyed one-shot with
  cancel). Requires a subscription.

### Exercise options

- `ExerciseOptions` — option exercise request (fire-and-forget).

### News

- `NewsProviders` — list available sources (singleton one-shot).
- `SubscribeNewsBulletins` — IB system bulletins, exchange halts (singleton
  subscription with cancel, no snapshot boundary).
- `NewsArticle` — full article body (keyed one-shot).
- `HistoricalNews` — past headlines from cache (keyed one-shot with end
  marker, with headline instants parsed from epoch-millisecond wire values).

### Scanner

- `ScannerParameters` — XML of all scanner filters (singleton one-shot).
- `SubscribeScannerResults` — server-side screener results (keyed subscription
  with cancel).

### FA configuration

- `RequestFA` — FA account configuration XML (singleton one-shot).
- `ReplaceFA` — update FA configuration (fire-and-forget write).
- `SoftDollarTiers` — available soft dollar tiers (keyed one-shot).

### WSH calendar

- `WSHMetaData` — available WSH event types and filters (keyed one-shot,
  returns JSON).
- `WSHEventData` — calendar events matching filter criteria (keyed one-shot,
  returns JSON).

### Display groups

- `QueryDisplayGroups` — available TWS display groups (keyed one-shot).
- `SubscribeDisplayGroup` — display group contract changes (keyed subscription
  with cancel).
- `UpdateDisplayGroup` — push contract selection to a display group
  (fire-and-forget targeting an active subscription).

### Other

- `MktDepthExchanges` — exchange metadata for Level 2 availability (singleton
  one-shot).
- `UserInfo` — white branding ID (keyed one-shot).

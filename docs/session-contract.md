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
- Reconnect boundaries are explicit through `SessionEvent` and
  `SubscriptionStateEvent`, not mixed into business event streams.

## Stability Goal

This contract is stable.

## Full API Surface

All methods below are implemented, tested against live IB Gateway
server_version 200 captures, and frozen into the contract.

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

### Other

- `MktDepthExchanges` — exchange metadata for Level 2 availability (singleton
  one-shot).
- `UserInfo` — white branding ID (keyed one-shot).

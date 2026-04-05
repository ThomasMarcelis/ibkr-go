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

Current repo truth:

- this contract is implemented and exercised against the in-repo fake host
- it is not yet validated against live TWS / IB Gateway protocol traffic

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
- `ResumeAuto` only for transcript-proven safe flows

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

The intent is to keep this contract stable while the internals move from the
current symbolic protocol model to real IBKR wire compatibility.

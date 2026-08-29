# ibkr-go

[![CI](https://github.com/ThomasMarcelis/ibkr-go/actions/workflows/ci.yml/badge.svg)](https://github.com/ThomasMarcelis/ibkr-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ThomasMarcelis/ibkr-go/v2.svg)](https://pkg.go.dev/github.com/ThomasMarcelis/ibkr-go/v2)
[![Go Version](https://img.shields.io/badge/go-1.26-blue)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

An idiomatic, pure-Go client for the Interactive Brokers TWS and IB Gateway socket API. Typed methods for snapshots, typed subscriptions for streams, typed order lifecycle tracking. Exact decimal arithmetic for prices, quantities, and money.

```go
client, err := ibkr.DialContext(ctx, ibkr.WithHost("127.0.0.1"), ibkr.WithPort(4002))
if err != nil {
    return err
}
defer client.Close()

// One-shot lookup: a typed result, blocks until IBKR answers.
details, err := client.Contracts().Qualify(ctx, ibkr.Stock("AAPL"))
if err != nil {
    return err
}
fmt.Println(details.LongName, details.MinTick) // APPLE INC 0.01

// Stream quotes: a typed subscription you range over. Delayed data needs no
// market-data subscription.
if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
    return err
}
quotes, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
    Contract: ibkr.Stock("AAPL"),
})
if err != nil {
    return err
}
for q := range quotes.All(ctx) {
    fmt.Println(q.Snapshot.Bid, q.Snapshot.Ask)
}
quotes.Close()
return errors.Join(quotes.Wait(), context.Cause(ctx))
```

## Install

```bash
go get github.com/ThomasMarcelis/ibkr-go/v2@v2.0.2
```

Requires Go 1.26+. Two dependencies:
[shopspring/decimal](https://github.com/shopspring/decimal) for exact financial
arithmetic and the [protobuf runtime](https://pkg.go.dev/google.golang.org/protobuf).
Full API reference on [pkg.go.dev](https://pkg.go.dev/github.com/ThomasMarcelis/ibkr-go/v2).

## Why ibkr-go

- **Go-shaped API.** One-shots return typed results. Streams are
  `*Subscription[T]` with one ordered `Events()` channel and an `All(ctx)`
  iterator. No `EWrapper` / `EClient` callback surface.
- **Broad coverage.** Accounts, positions, quotes, historical data, orders,
  market depth, executions, options, scanners, news, FA, WSH, display groups.
  Works with current TWS and IB Gateway builds (`server_version` 208–225).
- **Reconnects are explicit.** Drops, gaps, and resumptions arrive as ordered
  events on the same stream as the data, so you always know what you missed.
- **Exact financial values.** Prices, quantities, and money are
  [`decimal.Decimal`](https://github.com/shopspring/decimal).
- **Backed by live evidence.** 114 replay transcripts captured from a live IB
  Gateway, fuzzed framing and codec, and a deterministic CI that needs no
  broker credentials.

## Performance

The capture-backed public quote path has the following client-side overhead:

| Metric | Result | Why it matters |
|--------|-------:|----------------|
| Sustained quote delivery | 1.26 million updates/s | Headroom before the client becomes the stream bottleneck |
| Delivery latency | p50 5.34 µs, p95 8.59 µs, p99 11.8 µs | Delay added before an application receives an update |
| Heap pressure | 240 B/update, 12 allocs/update | Allocation and GC cost for long-running feeds |
| Cold dial to first quote | 20.3 ms | Connection and subscription startup before usable data |

These are medians of ten runs on Linux/amd64 with Go 1.26.3,
`GOMAXPROCS=16`, and an AMD Ryzen 7 9800X3D. Exact `server_version` 225
frames travel through loopback TCP, framing, decoding, actor routing, quote
projection, and the public subscription channel. The results isolate
`ibkr-go`; they do not include Gateway or external network latency. Repeated
frames measure client capacity, not expected Gateway data rates. The full run
also covers a 4,096-update burst and TCP fragmentation:

```bash
go test -run '^$' -bench '^BenchmarkPublicQuoteStream$' -benchtime=1x -count=10 .
```

## What's covered

| Facade | One-shots and controls | Streams and handles |
|--------|------------------------|---------------------|
| `client` | `ManagedAccounts`, `CurrentTime`, `Session` | `SessionEvents`, `Done`, `Wait` |
| `client.Accounts()` | `Summary`, `Positions`, `Updates`, `UpdatesMulti`, `PositionsMulti`, `FamilyCodes` | `SubscribeSummary`, `SubscribePositions`, `SubscribeUpdates`, `SubscribeUpdatesMulti`, `SubscribePositionsMulti`, `SubscribePnL`, `SubscribePnLSingle` |
| `client.Contracts()` | `Qualify`, `Details`, `Search`, `MarketRule`, `SecDefOptParams`, `SmartComponents`, `DepthExchanges` | `StreamDetails`, `StreamSecDefOptParams` |
| `client.MarketData()` | `SetType`, `Quote`, `RegulatorySnapshot` | `SubscribeQuotes`, `SubscribeRealTimeBars`, `SubscribeTickByTick`, `SubscribeDepth` |
| `client.History()` | `Bars`, `HeadTimestamp`, `Histogram`, `Ticks`, `Schedule` | `SubscribeBars` |
| `client.Orders()` | `RefreshOrderID`, `Preview`, `Cancel`, `CancelAll`, `Open`, `Completed`, `Executions` | `Place` → `OrderHandle`, `PlaceBracket`, `StreamCompleted`, `SubscribeOpen`, `SubscribeExecutions`, `SubscribeExecutionEvents` |
| `client.Options()` | `ImpliedVolatility`, `Price` | `Exercise` → `ExerciseHandle` |
| `client.News()` | `Providers`, `Article`, `Historical` | `SubscribeBulletins` |
| `client.Scanner()` | `Parameters` | `SubscribeResults` |
| `client.Advisors()` | `Config`, `SoftDollarTiers` | |
| `client.WSH()` | `MetaData`, `EventData` | |
| `client.TWS()` | `Config`, `UserInfo`, `DisplayGroups` | `SubscribeDisplayGroup` |

One-shots return `(T, error)` or `([]T, error)`. Subscriptions return
`*Subscription[T]` with `Events()`, `All(ctx)`, `Done()`, and `Close()`. Order
placement returns `*OrderHandle` with one ordered event stream plus `Cancel()`
and `Replace()`.

## Quick start

`ibkr.Stock`, `ibkr.OptionContract`, `ibkr.Future`, and `ibkr.Forex` build the
common contract shapes (SMART routing, USD, the 100-share option multiplier).
Anything more exotic, such as a combo or a non-USD listing, is a `Contract{}`
literal.

### Account summary and positions

```go
values, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
    Group: "All",
    Tags:  []string{"NetLiquidation", "TotalCashValue"},
})
if err != nil {
    return err
}
for _, v := range values {
    fmt.Println(v.Tag, v.Value, v.Currency)
}

positions, err := client.Accounts().Positions(ctx)
if err != nil {
    return err
}
for _, p := range positions {
    fmt.Println(p.Contract.Symbol, p.Position, p.AvgCost)
}
```

Every account call has a `Subscribe*` twin that streams updates instead, and
`SubscribePnL` streams real-time P&L.

### Stream quotes with lifecycle events

```go
sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
    Contract: ibkr.Stock("AAPL"),
}, ibkr.WithResumePolicy(ibkr.ResumeAuto))
if err != nil {
    return err
}
defer sub.Close()

for event := range sub.Events() {
    switch event.Kind {
    case ibkr.StreamData:
        fmt.Println(event.Value.Snapshot.Bid, event.Value.Snapshot.Ask)
    case ibkr.StreamNotice:
        log.Printf("notice: %v", event.Notice)
    case ibkr.StreamGap:
        log.Printf("gap: %v", event.Err)
    case ibkr.StreamRestored, ibkr.StreamResubscribed:
        log.Println("stream recovered")
    }
}
return sub.Err()
```

`All(ctx)` yields only data. `Events()` also carries notices (such as the
delayed-data downgrade) and reconnect boundaries. Both drain the same queue, so
pick one and read it from one goroutine. After the channel closes, `Err()` says
why, nil for a clean close, and `ibkr.IsRetryable(err)` tells you whether to
back off and try again.

### Historical bars

```go
bars, err := client.History().Bars(ctx, ibkr.HistoricalBarsRequest{
    Contract:   ibkr.Stock("AAPL"),
    Duration:   ibkr.Days(1),
    BarSize:    ibkr.Bar1Hour,
    WhatToShow: ibkr.ShowTrades,
    UseRTH:     true,
})
if err != nil {
    return err
}
for _, bar := range bars {
    fmt.Println(bar.Time, bar.Open, bar.High, bar.Low, bar.Close, bar.Volume)
}
```

### Place an order

`Place` returns an `OrderHandle`. Its `Events()` channel carries statuses,
fills, commissions, and warnings in order. The handle stays open after a
terminal status because IBKR can still send fills and fees afterwards; call
`Close` when you have what you need. Closing stops watching, it does not cancel
the order.

```go
handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
    Contract: ibkr.Stock("AAPL"),
    Order:    ibkr.LimitOrder(ibkr.ActionBuy, decimal.NewFromInt(1), decimal.RequireFromString("150.00")),
})
if err != nil {
    return err
}
defer handle.Close()

for evt := range handle.Events() {
    switch {
    case evt.Status != nil:
        fmt.Println(evt.Status.Status, evt.Status.Filled, evt.Status.Remaining)
        if ibkr.IsTerminalOrderStatus(evt.Status.Status) {
            return nil
        }
    case evt.Execution != nil:
        fmt.Println("fill:", evt.Execution.Shares, "@", evt.Execution.Price)
    case evt.CommissionAndFees != nil:
        fmt.Println("fees:", evt.CommissionAndFees.Amount, evt.CommissionAndFees.Currency)
    }
}
return handle.Wait()
```

`Events()` stays open until you close the handle, so bound the loop with your
own context or timeout if the order may rest. Cancel or amend while the handle
is open:

```go
if err := handle.Cancel(ctx); err != nil {
    return err
}
if err := handle.Replace(ctx, revisedOrder); err != nil {
    return err
}
```

Both return once the request is queued for IBKR, so keep reading events for
the outcome. Order events are queued losslessly (64 by default, tune with
`ibkr.WithOrderEventBuffer`). If you stop reading and the queue fills, the
handle closes with `ErrSlowConsumer` while the order stays live at IBKR;
`handle.OrderID()` is what you reconcile with. `Orders().Open` lists working
orders after a restart, but only a handle from this process's `Place` can
`Replace`.

### Bracket orders

```go
quantity := decimal.NewFromInt(1)
bracket, err := client.Orders().PlaceBracket(ctx, ibkr.PlaceBracketRequest{
    Contract:   ibkr.Stock("AAPL"),
    Parent:     ibkr.LimitOrder(ibkr.ActionBuy, quantity, decimal.RequireFromString("150")),
    TakeProfit: ibkr.LimitOrder(ibkr.ActionSell, quantity, decimal.RequireFromString("165")),
    StopLoss:   ibkr.StopOrder(ibkr.ActionSell, quantity, decimal.RequireFromString("142")),
})
if err != nil {
    return err
}
fmt.Println(bracket.Parent.OrderID(), bracket.TakeProfit.OrderID(), bracket.StopLoss.OrderID())
```

`PlaceBracket` allocates the three IDs, links the children, and sets the
transmit flags for you. Each handle has the same event API as a single order.
If only some legs were accepted you get an `*OrderRecoveryError` listing them;
reconcile through `Orders().Open` instead of retrying. The full placement and
recovery rules are in the [session contract](docs/session-contract.md).

## Errors

Errors are typed so you can decide policy without parsing text:

- `*ConnectError`: dial or handshake failed.
- `*APIError`: IBKR rejected or warned about a request. `IsPacingViolation`,
  `IsEntitlement`, and `IsOrderRejection` classify it.
- `*ValidationError`: the request was rejected locally before it was sent.
- `*ProtocolError`: a malformed wire frame; the connection is retired.

`ibkr.IsRetryable(err)` is the one retry decision. It is true for session
interruptions, transient connection failures, and pacing violations. It is
false for ordinary rejections, validation and protocol failures, slow
consumers, and any error that leaves remote state uncertain
(`*OrderRecoveryError`, `*ExerciseUncertainError`,
`*SubscriptionCancelError`, `*RegulatorySnapshotUncertainError`), because a
blind retry could duplicate a live order.

```go
if apiErr, ok := errors.AsType[*ibkr.APIError](err); ok {
    switch {
    case apiErr.IsPacingViolation():
        // back off before retrying
    case apiErr.IsEntitlement():
        // request permissions, or select delayed market data where supported
    case apiErr.IsOrderRejection():
        // placement failed before working-order evidence appeared
    }
}
```

The [session contract](docs/session-contract.md) lists every error type and
its recovery rule.

## Reconnects

The client redials and handshakes after a dropped socket (`ReconnectAuto`, the
default). What happens to your streams is explicit:

- Quote and real-time-bar streams opened with `ResumeAuto` are re-requested
  automatically; `Gap`, `Restored`, and `Resubscribed` events mark the boundary
  in the stream itself.
- Every other stream ends with `ErrResumeRequired` (finite ones with
  `ErrInterrupted`). Open a new one; it waits for the next ready session.
- An order handle that crosses a full reconnect can no longer replace its
  order and reports `ErrOrderRecoveryRequired`. Reconcile with
  `Orders().Open` before acting on that order again.

Cancellation and connection-retirement details are in
[operation control](docs/operation-control.md).

## Examples

Ten runnable programs under [`examples/`](examples/), each one file against a
real Gateway. Start with `connect`, `quotes`, `historical`, `portfolio`, and
`order`, then `bracket`, `option-chain`, `scanner`, `resilient-quotes`, and
`margin-preview`. The order-shaped ones refuse to run outside a paper
account.

## Status

v2.0.2 covers the in-scope socket API against `server_version` 208–225.
The [coverage matrix](docs/live-coverage-matrix.md) says which paths have live proof and which are blocked by entitlements or market state. Release notes are in the [CHANGELOG](CHANGELOG.md).
Not planned: Flex, Client Portal Web API, or an `EWrapper` / `EClient` compatibility bridge. See [`docs/roadmap.md`](docs/roadmap.md).

## Development

```bash
go test ./...
```

The full pre-PR checklist and the live-verification setup are in
[`CONTRIBUTING.md`](CONTRIBUTING.md). Live tests are opt-in:

```bash
IBKR_LIVE=1 IBKR_LIVE_READONLY_ADDR=127.0.0.1:4001 go test ./... -run '^TestLive' -count=1
```

## Documentation

| Document | What it covers |
|----------|----------------|
| [`docs/session-contract.md`](docs/session-contract.md) | Public API contract: sessions, subscriptions, orders, errors |
| [`docs/operation-control.md`](docs/operation-control.md) | Cancellation and connection retirement |
| [`docs/migration-v2.md`](docs/migration-v2.md) | Upgrading from v1 |
| [`docs/anti-patterns.md`](docs/anti-patterns.md) | Design philosophy |
| [`docs/architecture.md`](docs/architecture.md) | Internal layer design |
| [`docs/transcripts.md`](docs/transcripts.md) | Replay transcript format |
| [`docs/live-coverage-matrix.md`](docs/live-coverage-matrix.md) | Capability coverage status |
| [`docs/protocol-audit-sv208-225.md`](docs/protocol-audit-sv208-225.md) | What each server version 208–225 changes, with capture evidence |
| [`docs/ibkr-api-inventory.md`](docs/ibkr-api-inventory.md) | Official TWS API surface mapped to ibkr-go |
| [`docs/roadmap.md`](docs/roadmap.md) | Next steps |

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md).

## License

MIT. See [`LICENSE`](LICENSE).

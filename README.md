# ibkr-go

> [!WARNING]
> **v1 is deprecated and no longer maintained.** Use
> [`github.com/ThomasMarcelis/ibkr-go/v2`](https://pkg.go.dev/github.com/ThomasMarcelis/ibkr-go/v2)
> and follow the [v2 migration guide](https://github.com/ThomasMarcelis/ibkr-go/blob/main/docs/migration-v2.md).

[![CI](https://github.com/ThomasMarcelis/ibkr-go/actions/workflows/ci.yml/badge.svg)](https://github.com/ThomasMarcelis/ibkr-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ThomasMarcelis/ibkr-go.svg)](https://pkg.go.dev/github.com/ThomasMarcelis/ibkr-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/ThomasMarcelis/ibkr-go)](https://goreportcard.com/report/github.com/ThomasMarcelis/ibkr-go)
[![Go Version](https://img.shields.io/badge/go-1.26-blue)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

An idiomatic Go client for the Interactive Brokers TWS and IB Gateway socket
protocol. Typed methods for snapshots. Typed subscriptions for streams. Full
order management with lifecycle tracking. Exact decimal arithmetic for all
financial values.

```go
client, _ := ibkr.DialContext(ctx, ibkr.WithHost("127.0.0.1"), ibkr.WithPort(4002))
defer client.Close()

// one-shot — typed result, blocks until done
positions, _ := client.Accounts().Positions(ctx)

// streaming — typed subscription with lifecycle events
sub, _ := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
    Contract: ibkr.Contract{Symbol: "AAPL", SecType: ibkr.SecTypeStock, Exchange: "SMART", Currency: "USD"},
})
defer sub.Close()
for update := range sub.Events() {
    fmt.Println(update.Snapshot.Bid, update.Snapshot.Ask)
}
```

## Install

New projects should use v2:

```bash
go get github.com/ThomasMarcelis/ibkr-go/v2@latest
```

Existing v1 users can pin this final release while migrating:

```bash
go get github.com/ThomasMarcelis/ibkr-go@v1.5.2
```

Requires Go 1.26+. One dependency: [shopspring/decimal](https://github.com/shopspring/decimal) for exact financial arithmetic.

Full API reference on [pkg.go.dev](https://pkg.go.dev/github.com/ThomasMarcelis/ibkr-go).

## Why ibkr-go

- **Go-shaped API.** One-shots return typed results. Streams return typed
  subscriptions with `Events()`, `Lifecycle()`, and `Done()`. No `EWrapper` /
  `EClient` callback surface.
- **Broad TWS/Gateway coverage.** Accounts, positions, quotes, historical data,
  order management, market depth, executions, options, scanners, news, FA
  configuration, WSH, display groups, and more, with remaining official gaps
  tracked in the roadmap and coverage matrix.
- **Reconnects are explicit.** Session transitions and subscription lifecycle
  events — `Gap`, `Resumed`, `SnapshotComplete`, `Closed` — are part of the
  contract, not hidden behind callbacks.
- **Exact financial values.** [`decimal.Decimal`](https://github.com/shopspring/decimal)
  for prices, quantities, and money throughout the API — no float64 rounding.
- **Protocol work backed by evidence.** Replay scenarios derived from live IB
  Gateway traffic, wire and codec fuzzing, and deterministic CI.

## Quick Start

The mental model: call a method for a snapshot, subscribe for a stream.

### Connect and qualify a contract

```go
client, err := ibkr.DialContext(ctx,
    ibkr.WithHost("127.0.0.1"),
    ibkr.WithPort(4002),
)
if err != nil {
    return err
}
defer client.Close()

details, err := client.Contracts().Qualify(ctx, ibkr.Contract{
    Symbol:   "AAPL",
    SecType:  ibkr.SecTypeStock,
    Exchange: "SMART",
    Currency: "USD",
})
if err != nil {
    return err
}
fmt.Println(details.LongName, details.MinTick) // APPLE INC 0.01
```

### Stream live quotes

```go
sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
    Contract: ibkr.Contract{
        Symbol:   "AAPL",
        SecType:  ibkr.SecTypeStock,
        Exchange: "SMART",
        Currency: "USD",
    },
})
if err != nil {
    return err
}
defer sub.Close()

events := sub.Events()
lifecycle := sub.Lifecycle()
for events != nil {
    select {
    case update, ok := <-events:
        if !ok {
            return sub.Wait()
        }
        fmt.Println(update.Snapshot.Bid, update.Snapshot.Ask)
    case state, ok := <-lifecycle:
        if !ok {
            lifecycle = nil
            continue
        }
        fmt.Println("lifecycle:", state.Kind, "retryable:", state.Retryable)
    }
}
```

`Events()` carries market data. `Lifecycle()` carries session boundaries —
`SnapshotComplete`, `Gap`, `Resumed`, `Closed`. They never mix. When a stream
ends, use `sub.Err()` or `Wait()` with `ibkr.IsRetryable(err)`, or inspect
`state.Retryable`, to distinguish reconnectable gaps from terminal IBKR API
rejections. Drain `Events()` until it closes when final buffered data matters;
use `Done()` for coordinating other goroutines, not as a replacement for event
draining.

### Fetch historical bars

```go
bars, err := client.History().Bars(ctx, ibkr.HistoricalBarsRequest{
    Contract: ibkr.Contract{
        Symbol:   "AAPL",
        SecType:  ibkr.SecTypeStock,
        Exchange: "SMART",
        Currency: "USD",
    },
    EndTime:    time.Now(),
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

### Place an order and track its lifecycle

`Place` returns an `OrderHandle` whose `Events()` channel carries a typed
union — exactly one of `Status`, `Execution`, `Commission`, or `OpenOrder` is
non-nil per event. The channel closes after the terminal status (`Filled`,
`Cancelled`, or `Inactive`).

```go
handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
    Contract: ibkr.Contract{
        Symbol:   "AAPL",
        SecType:  ibkr.SecTypeStock,
        Exchange: "SMART",
        Currency: "USD",
    },
    Order: ibkr.Order{
        Action:    ibkr.Buy,
        OrderType: ibkr.OrderTypeLimit,
        Quantity:  decimal.NewFromInt(1),
        LmtPrice:  decimal.RequireFromString("150.00"),
        TIF:       ibkr.TIFDay,
    },
})
if err != nil {
    return err
}

for evt := range handle.Events() {
    switch {
    case evt.Status != nil:
        fmt.Println(evt.Status.Status, evt.Status.Filled, evt.Status.Remaining)
    case evt.Execution != nil:
        fmt.Println("fill:", evt.Execution.Shares, "@", evt.Execution.Price)
    case evt.Commission != nil:
        fmt.Println("commission:", evt.Commission.Commission, evt.Commission.Currency)
    }
}
return handle.Wait() // nil when terminal status reached cleanly
```

Cancel or modify a working order at any time:

```go
handle.Cancel(ctx)               // request cancellation
handle.Modify(ctx, revisedOrder) // amend price, quantity, etc.
```

The events channel keeps delivering until the server confirms the terminal
state.

### Account data

```go
// snapshot
values, _ := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
    Account: "All",
    Tags:    []string{"NetLiquidation", "TotalCashValue"},
})

// streaming positions
sub, _ := client.Accounts().SubscribePositions(ctx)
defer sub.Close()
for pos := range sub.Events() {
    fmt.Println(pos.Position.Contract.Symbol, pos.Position.Position, pos.Position.AvgCost)
}

// real-time P&L
pnl, _ := client.Accounts().SubscribePnL(ctx, ibkr.PnLRequest{Account: "DU12345"})
defer pnl.Close()
```

## API Shape

Every domain is accessed through a facade on the client:

| Facade | Snapshots | Subscriptions |
|--------|-----------|---------------|
| `client.Accounts()` | `Summary`, `Positions`, `Updates`, `FamilyCodes` | `SubscribeSummary`, `SubscribePositions`, `SubscribePnL`, `SubscribePnLSingle` |
| `client.Contracts()` | `Qualify`, `Details`, `Search`, `MarketRule`, `SecDefOptParams`, `FundamentalData` | — |
| `client.MarketData()` | `Quote` | `SubscribeQuotes`, `SubscribeRealTimeBars`, `SubscribeTickByTick`, `SubscribeDepth` |
| `client.History()` | `Bars`, `HeadTimestamp`, `Histogram`, `Ticks`, `Schedule` | `SubscribeBars` |
| `client.Orders()` | `Open`, `Completed`, `Executions` | `Place` -> `OrderHandle`, `SubscribeOpen` |
| `client.Options()` | `ImpliedVolatility`, `Price`, `Exercise` | — |
| `client.News()` | `Providers`, `Article`, `Historical` | `SubscribeBulletins` |
| `client.Scanner()` | `Parameters` | `SubscribeResults` |
| `client.Advisors()` | `Config`, `ReplaceConfig`, `SoftDollarTiers` | — |
| `client.WSH()` | `MetaData`, `EventData` | — |
| `client.TWS()` | `UserInfo`, `DisplayGroups` | `SubscribeDisplayGroup` |

One-shots return `(T, error)` or `([]T, error)`. Subscriptions return
`*Subscription[T]` with `Events()`, `Lifecycle()`, `Done()`, and `Close()`.
Order placement returns `*OrderHandle` with the same channel pattern plus
`Cancel()` and `Modify()`.

## Examples

The [`examples/`](examples/) directory contains standalone programs you can run
against a local paper IB Gateway:

```bash
IBKR_ADDR=127.0.0.1:4002 go run ./examples/connect       # session info
IBKR_ADDR=127.0.0.1:4002 go run ./examples/quotes         # live quote stream
IBKR_ADDR=127.0.0.1:4002 go run ./examples/historical     # historical bars
IBKR_ADDR=127.0.0.1:4002 go run ./examples/portfolio      # account + positions + P&L stream
IBKR_ADDR=127.0.0.1:4002 IBKR_TRADING=1 go run ./examples/order  # place, observe, cancel
```

Each example demonstrates real error handling, context cancellation, and
graceful shutdown.

## Testing and Verification

Every protocol behavior this library claims has a test pinning it down.

- Checked-in replay transcripts under
  [`testdata/transcripts`](testdata/transcripts)
- Fuzz targets covering wire framing and codec round-trips
- Deterministic CI for routine verification, without broker credentials
- Separate live-gated tests for local verification against TWS or IB Gateway.
  Read-only live checks default to `127.0.0.1:4001`; paper-trading checks
  default to `127.0.0.1:4002`.

The goal is a library whose protocol behavior can be frozen, replayed,
stressed, and extended without guessing. For more on that approach, see
[`docs/transcripts.md`](docs/transcripts.md) and
[`docs/anti-patterns.md`](docs/anti-patterns.md).

## Status

ibkr-go covers the major Interactive Brokers TWS/Gateway socket protocol
domains through an idiomatic Go facade. Ongoing work focuses on closing the
remaining official edge branches, keeping pace with new Gateway versions,
expanding replay coverage, and tightening API ergonomics.

Not planned: Flex, Client Portal Web API, or an `EWrapper` / `EClient`
compatibility bridge. See [`docs/roadmap.md`](docs/roadmap.md) for the full
charter.

## Development

```bash
go build ./...
go vet ./...
gofmt -l .           # must produce no output
golangci-lint run
go test ./...
```

All five must pass before opening a pull request. CI runs the same checks on
every push.

Local live verification is opt-in:

```bash
IBKR_LIVE=1 IBKR_LIVE_READONLY_ADDR=127.0.0.1:4001 go test ./... -run '^TestLive' -count=1
IBKR_LIVE=1 IBKR_LIVE_TRADING=1 IBKR_LIVE_PAPER_ADDR=127.0.0.1:4002 go test ./... -run '^TestLive(PlaceOrder|GlobalCancel|Trading)' -count=1
```

The maintainer lab uses two Gateway roles:

- `IBKR_LIVE_READONLY_ADDR` points at the real-money, read-only Gateway with
  live market data. Read-only tests and capture campaigns use this role.
- `IBKR_LIVE_PAPER_ADDR` points at the throwaway paper Gateway. Tests that
  place, modify, cancel, or flatten orders require both `IBKR_LIVE_TRADING=1`
  and this paper role.

Run the setup diagnostic before a live campaign:

```bash
go run ./cmd/ibkr-doctor -role readonly-live
go run ./cmd/ibkr-doctor -role paper-dev
```

## Documentation

- [`docs/session-contract.md`](docs/session-contract.md) — public API contract
- [`docs/anti-patterns.md`](docs/anti-patterns.md) — design philosophy

For contributors and maintainers:

- [`docs/architecture.md`](docs/architecture.md) — internal layer design
- [`docs/transcripts.md`](docs/transcripts.md) — test transcript format
- [`docs/message-coverage.md`](docs/message-coverage.md) — protocol message matrix
- [`docs/ibkr-api-inventory.md`](docs/ibkr-api-inventory.md) — official/repo surface inventory
- [`docs/live-coverage-matrix.md`](docs/live-coverage-matrix.md) — capability coverage status
- [`docs/live-test-tracker.md`](docs/live-test-tracker.md) — live run and capture evidence
- [`docs/roadmap.md`](docs/roadmap.md) — project direction

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md).

## License

MIT. See [`LICENSE`](LICENSE).

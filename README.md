# ibkr-go

[![CI](https://github.com/ThomasMarcelis/ibkr-go/actions/workflows/ci.yml/badge.svg)](https://github.com/ThomasMarcelis/ibkr-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ThomasMarcelis/ibkr-go.svg)](https://pkg.go.dev/github.com/ThomasMarcelis/ibkr-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/ThomasMarcelis/ibkr-go)](https://goreportcard.com/report/github.com/ThomasMarcelis/ibkr-go)
[![Go Version](https://img.shields.io/badge/go-1.26-blue)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

An idiomatic Go client for the Interactive Brokers TWS and IB Gateway socket
protocol. Typed methods for snapshots. Typed subscriptions for streams. Typed
order lifecycle tracking across the currently implemented surface. Exact
decimal arithmetic for prices, quantities, and money.

```go
client, err := ibkr.DialContext(ctx, ibkr.WithHost("127.0.0.1"), ibkr.WithPort(4002))
if err != nil {
    return err
}
defer func() { _ = client.Close() }()

// one-shot — typed result, blocks until done
positions, err := client.Accounts().Positions(ctx)
if err != nil {
    return err
}
fmt.Println("positions:", len(positions))

// streaming — typed subscription with lifecycle events
sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
    Contract: ibkr.Contract{Symbol: "AAPL", SecType: ibkr.SecTypeStock, Exchange: "SMART", Currency: "USD"},
})
if err != nil {
    return err
}
defer func() { _ = sub.Close() }()
for update := range sub.All(ctx) {
    fmt.Println(update.Snapshot.Bid, update.Snapshot.Ask)
}
return sub.Err()
```

## Install

```bash
go get github.com/ThomasMarcelis/ibkr-go@latest
```

Requires Go 1.26+. One dependency: [shopspring/decimal](https://github.com/shopspring/decimal) for exact financial arithmetic.

Full API reference on [pkg.go.dev](https://pkg.go.dev/github.com/ThomasMarcelis/ibkr-go).

## Why ibkr-go

- **Go-shaped API.** One-shots return typed results. Streams return typed
  subscriptions with `Events()`, `Lifecycle()`, and `Done()`. No `EWrapper` /
  `EClient` callback surface.
- **Broad TWS/Gateway coverage.** Accounts, positions, quotes, historical data,
  order management, market depth, executions, options, scanners, news, FA
  configuration, WSH, display groups, and more. The supported baseline is
  `server_version 200`; remaining and partially decoded official branches are
  tracked explicitly in the roadmap and coverage matrix.
- **Reconnects are explicit.** Session transitions and subscription lifecycle
  events — `Gap`, `Resumed`, `SnapshotComplete`, `Closed` — are part of the
  contract, not hidden behind callbacks.
- **Exact financial values.** [`decimal.Decimal`](https://github.com/shopspring/decimal)
  for typed prices, quantities, and money — no float64 rounding. Heterogeneous
  IBKR values and extensible tag payloads remain strings where the protocol
  does not define a single numeric meaning.
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

details, err := client.Contracts().Qualify(ctx, ibkr.Stock("AAPL"))
if err != nil {
    return err
}
fmt.Println(details.LongName, details.MinTick) // APPLE INC 0.01
```

`ibkr.Stock`, `ibkr.OptionContract`, and `ibkr.Future` fill the common fields
(SMART routing, USD, and the 100-share option multiplier) for their standard
contract shapes. `ibkr.Forex` returns a contract and an error; it accepts
exactly six uppercase ASCII letters such as `EURUSD` and configures IDEALPRO
routing. Build a `Contract{}` literal directly for anything more exotic
(combos, non-USD listings, a specific primary exchange).

### Stream live quotes

```go
sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
    Contract: ibkr.Stock("AAPL"),
})
if err != nil {
    return err
}
defer sub.Close()

for update := range sub.All(ctx) {
    fmt.Println(update.Snapshot.Bid, update.Snapshot.Ask)
}
if err := sub.Err(); err != nil {
    return err
}
```

`sub.All(ctx)` ranges over business data until the subscription closes or ctx
is canceled, then `sub.Err()` reports why: nil for a clean close, or e.g.
`ibkr.ErrSlowConsumer` / `ibkr.ErrInterrupted` otherwise — check
`ibkr.IsRetryable(err)` to tell a reconnectable gap from a terminal IBKR API
rejection. Lifecycle transitions (`SnapshotComplete`, `Gap`, `Resumed`,
`Closed`) are a separate channel, `sub.Lifecycle()`, and are never mixed into
`All`/`Events`; a caller that needs both streams in one loop reads
`Events()`/`Lifecycle()` directly with a `select`, or reads `Lifecycle()` from
another goroutine.

### Fetch historical bars

```go
bars, err := client.History().Bars(ctx, ibkr.HistoricalBarsRequest{
    Contract:   ibkr.Stock("AAPL"),
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
union — exactly one of `Status`, `Execution`, `CommissionAndFees`, `OpenOrder`,
or `Warning` is non-nil per event. The channel closes after the terminal status
(`Filled`, `Cancelled`, or `Inactive`) or an earlier local observation error.

```go
handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
    Contract: ibkr.Stock("AAPL"),
    Order:    ibkr.LimitOrder(ibkr.ActionBuy, decimal.NewFromInt(1), decimal.RequireFromString("150.00")),
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
    case evt.CommissionAndFees != nil:
        fmt.Println("commission and fees:", evt.CommissionAndFees.Amount, evt.CommissionAndFees.Currency)
    }
}
return handle.Wait() // nil when terminal status reached cleanly
```

Order events use a bounded, lossless queue with a default capacity of 64;
configure it for the client with `ibkr.WithOrderEventBuffer`. Events are never
silently dropped while observation continues. If the queue fills, the handle
closes and `Wait` returns `ibkr.ErrSlowConsumer`. That ends only this process's
observation: the live order may keep executing at IBKR, and `handle.OrderID()`
remains the coordinate for open-order reconciliation or direct cancellation.

While the handle remains active, cancel or modify a working order:

```go
if err := handle.Cancel(ctx); err != nil { // request cancellation
    return err
}
if err := handle.Modify(ctx, revisedOrder); err != nil { // amend price, quantity, etc.
    return err
}
```

After `ErrSlowConsumer`, `Modify` returns `ErrClosed`; reconcile with the
stable `OrderID` and cancel through `handle.Cancel` or
`client.Orders().Cancel(ctx, handle.OrderID())` if the order is still working.

Absent an explicit local close or observation error, the events channel keeps
delivering until the server confirms the terminal state.

Place a bracket without managing IDs or transmit flags yourself:

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

`PlaceBracket` allocates all three IDs in one engine turn, links both children,
and sends the required `Transmit=false`, `false`, `true` sequence. Each returned
handle has the same lifecycle API as a regular order.

Transport-queue admission is the ownership boundary for both placement calls.
After admission, a concurrent context cancellation or client shutdown does not
replace a successful result: you receive the handle (or bracket) and own its
lifecycle. Before admission, placement returns an error and no handle. If a
bracket is only partially admitted, the zero bracket is returned with an
`*ibkr.OrderRecoveryError`; its `OrderIDs` contain every admitted leg to
reconcile through open orders. `CancelErr == nil` means every compensating
cancel entered the transport queue, not that IBKR acknowledged it. Recovery
errors are deliberately not retryable, because blind retry could duplicate a
live leg. Only failure before the parent enters the queue returns the original
placement error directly.

### Account data

```go
// snapshot
values, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
    Group: "All",
    Tags:  []string{"NetLiquidation", "TotalCashValue"},
})
if err != nil {
    return err
}
fmt.Println("account values:", len(values))

// streaming positions
sub, err := client.Accounts().SubscribePositions(ctx)
if err != nil {
    return err
}
defer func() { _ = sub.Close() }()
for pos := range sub.Events() {
    fmt.Println(pos.Position.Contract.Symbol, pos.Position.Position, pos.Position.AvgCost)
}
if err := sub.Wait(); err != nil {
    return err
}

// real-time P&L
pnl, err := client.Accounts().SubscribePnL(ctx, ibkr.PnLRequest{Account: "DU9000001"})
if err != nil {
    return err
}
defer func() { _ = pnl.Close() }()
```

## API Shape

Every domain is accessed through a facade on the client:

| Facade | Snapshots | Subscriptions |
|--------|-----------|---------------|
| `client.Accounts()` | `Summary`, `Positions`, `Updates`, `FamilyCodes` | `SubscribeSummary`, `SubscribePositions`, `SubscribePnL`, `SubscribePnLSingle` |
| `client.Contracts()` | `Qualify`, `Details`, `Search`, `MarketRule`, `SecDefOptParams`, `SmartComponents`, `DepthExchanges` | — |
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
IBKR_ADDR=127.0.0.1:4002 IBKR_TRADING=paper go run ./examples/order  # place, observe, cancel
```

Each example demonstrates real error handling, context cancellation, and
graceful shutdown.

## Testing and Verification

Supported behavior is frozen through deterministic tests; the coverage matrix
distinguishes implementation, replay, live attestation, partial branches, and
entitlement blockers rather than treating them as equivalent proof.

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
domains through an idiomatic Go facade. The supported and live-attested
classic baseline is `server_version 200`. The client does not yet implement
the raw-ID/protobuf protocol used above 200, and some advanced v200 response
layouts remain explicitly partial. See the coverage matrix for the evidence
behind each claim.

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

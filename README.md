# ibkr-go

[![CI](https://github.com/ThomasMarcelis/ibkr-go/actions/workflows/ci.yml/badge.svg)](https://github.com/ThomasMarcelis/ibkr-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ThomasMarcelis/ibkr-go/v2.svg)](https://pkg.go.dev/github.com/ThomasMarcelis/ibkr-go/v2)
[![Go Report Card](https://goreportcard.com/badge/github.com/ThomasMarcelis/ibkr-go/v2)](https://goreportcard.com/report/github.com/ThomasMarcelis/ibkr-go/v2)
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
defer client.Close()

// one-shot — typed result, blocks until done
positions, err := client.Accounts().Positions(ctx)
if err != nil {
    return err
}
fmt.Println("positions:", len(positions))

// streaming — one ordered stream, or data-only iteration with All
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
return sub.Err()
```

## Install

```bash
go get github.com/ThomasMarcelis/ibkr-go/v2@v2.0.0-rc.2
```

Requires Go 1.26+. One dependency: [shopspring/decimal](https://github.com/shopspring/decimal) for exact financial arithmetic.

Full API reference on [pkg.go.dev](https://pkg.go.dev/github.com/ThomasMarcelis/ibkr-go/v2).
v2 uses Go semantic import versioning, so existing v1 applications cannot upgrade accidentally. Adopting v2 requires changing imports to `github.com/ThomasMarcelis/ibkr-go/v2` and following the [v2 migration guide](docs/migration-v2.md).

## Why ibkr-go

- **Go-shaped API.** One-shots return typed results. Streams return typed
  subscriptions with one ordered `Events()` stream plus `Done()`. No `EWrapper` /
  `EClient` callback surface.
- **Broad TWS/Gateway coverage.** Accounts, positions, quotes, historical data,
  order management, market depth, executions, options, scanners, news, FA
  configuration, WSH, display groups, and more. The client negotiates
  `server_version` 200..207: 200 is the classic-wire live baseline, with exact
  post-200 protocol migrations through 207. Remaining official branches are
  tracked explicitly in the roadmap and coverage matrix.
- **Reconnects are explicit.** Subscription events preserve `Gap`, `Restored`,
  `Resubscribed`, and `SnapshotComplete` in order with data. Channel close plus
  `Err()` is the terminal signal.
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

### Stream quotes

```go
if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
    return err
}
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
`ibkr.IsRetryable(err)` to distinguish retry-with-backoff conditions from
terminal failures. `All` filters lifecycle transitions from the subscription's
single queue. To observe reconnect boundaries, consume `Events()` instead:

```go
for event := range sub.Events() {
    switch event.Kind {
    case ibkr.StreamData:
        fmt.Println(event.Value.Snapshot.Bid, event.Value.Snapshot.Ask)
    case ibkr.StreamNotice:
        log.Printf("quote notice: %v", event.Notice)
    case ibkr.StreamGap:
        log.Printf("quote gap on connection %d: %v", event.ConnectionSeq, event.Err)
    case ibkr.StreamRestored, ibkr.StreamResubscribed:
        log.Printf("quote stream recovered on connection %d", event.ConnectionSeq)
    }
}
return sub.Err()
```

`Events()` and `All(ctx)` consume the same ordered queue; choose one rather
than reading both concurrently. `All` filters lifecycle and notice events.
`AwaitSnapshot(ctx)` remains a durable initial snapshot boundary.

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
`Warning`, `Binding`, or `Lifecycle` is non-nil per event. A terminal order status does not close the
handle because IBKR can deliver executions and fees afterward. The caller ends
its observation window explicitly after collecting the evidence it needs.

```go
handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
    Contract: ibkr.Stock("AAPL"),
    Order:    ibkr.LimitOrder(ibkr.ActionBuy, decimal.NewFromInt(1), decimal.RequireFromString("150.00")),
})
if err != nil {
    return err
}

var terminal, execution bool
for !terminal || !execution {
    evt := <-handle.Events()
    switch {
    case evt.Status != nil:
        fmt.Println(evt.Status.Status, evt.Status.Filled, evt.Status.Remaining)
        terminal = ibkr.IsTerminalOrderStatus(evt.Status.Status)
    case evt.Execution != nil:
        fmt.Println("fill:", evt.Execution.Shares, "@", evt.Execution.Price)
        execution = true
    case evt.CommissionAndFees != nil:
        fmt.Println("commission and fees:", evt.CommissionAndFees.Amount, evt.CommissionAndFees.Currency)
    }
}
handle.Close()
return handle.Wait()
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
if err := handle.Replace(ctx, revisedOrder); err != nil { // amend price, quantity, etc.
    return err
}
```

After `ErrSlowConsumer`, `Replace` returns `ErrClosed`; reconcile with the
stable `OrderID` and cancel through `handle.Cancel` or
`client.Orders().Cancel(ctx, handle.OrderID())` if the order is still working.

The handle remains open after a terminal status so trailing executions and fees
can arrive. Call `Close` when the application's observation window is complete.

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
handle has the same ordered order-event API as a regular order.

Transport-queue admission is the ownership boundary for both placement calls.
After admission, a concurrent context cancellation does not replace a
successful result: you receive the handle (or bracket) and own its lifecycle.
If the admitted frame was still unwritten when the transport died, that handle
closes with `ErrInterrupted`; IBKR never received that frame. Before admission,
placement returns an error and no handle. If a
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
defer sub.Close()
for pos := range sub.All(ctx) {
    fmt.Println(pos.Contract.Symbol, pos.Position, pos.AvgCost)
}
if err := sub.Wait(); err != nil {
    return err
}

// real-time P&L
pnl, err := client.Accounts().SubscribePnL(ctx, ibkr.PnLRequest{Account: "DU9000001"})
if err != nil {
    return err
}
defer pnl.Close()
```

## API Shape

Every domain is accessed through a facade on the client:

| Facade | Snapshots | Subscriptions |
|--------|-----------|---------------|
| `client.Accounts()` | `Summary`, `Positions`, `Updates`, `FamilyCodes` | `SubscribeSummary`, `SubscribePositions`, `SubscribePnL`, `SubscribePnLSingle` |
| `client.Contracts()` | `Qualify`, `Details`, `Search`, `MarketRule`, `SecDefOptParams`, `SmartComponents`, `DepthExchanges` | — |
| `client.MarketData()` | `Quote`, `RegulatorySnapshot` | `SubscribeQuotes`, `SubscribeRealTimeBars`, `SubscribeTickByTick`, `SubscribeDepth` |
| `client.History()` | `Bars`, `HeadTimestamp`, `Histogram`, `Ticks`, `Schedule` | `SubscribeBars` |
| `client.Orders()` | `Open`, `Completed`, `Executions` | `Place` -> `OrderHandle`, `SubscribeOpen`, `SubscribeExecutions` |
| `client.Options()` | `ImpliedVolatility`, `Price` | `Exercise` -> `ExerciseHandle` |
| `client.News()` | `Providers`, `Article`, `Historical` | `SubscribeBulletins` |
| `client.Scanner()` | `Parameters` | `SubscribeResults` |
| `client.Advisors()` | `Config`, `SoftDollarTiers` | — |
| `client.WSH()` | `MetaData`, `EventData` | — |
| `client.TWS()` | `UserInfo`, `DisplayGroups` | `SubscribeDisplayGroup` |

One-shots return `(T, error)` or `([]T, error)`. Subscriptions return
`*Subscription[T]` with `Events()`, `All(ctx)`, `Done()`, and `Close()`.
Order placement returns `*OrderHandle` with one ordered event stream plus
`Cancel()` and `Replace()`.

## Errors

Errors are typed so callers can make a policy decision without parsing text:
`*ConnectError` covers dial/bootstrap failures, `*ProtocolError` wire failures,
`*ValidationError` rejected local input, and `*APIError` a Gateway response.
`*OrderRecoveryError`, `*ExerciseUncertainError`, and
`*SubscriptionCancelError` mean remote state is uncertain and deliberately
override any retryable wrapped cause. `ErrOrderRecoveryRequired` means an
order handle crossed an observation gap and can no longer be used for
replacement; it is distinct from retryable subscription resumption.

`ibkr.IsRetryable(err)` is the single retry-with-backoff decision. It returns
true for not-ready/session interruption, transient connection failures, and
actual pacing violations. Ordinary API rejections, context cancellation,
protocol/validation failures, slow consumers, execution-correlation overflow,
and uncertain recovery errors are false. `SubscribeExecutions` retains at most
4096 execution IDs and 4096 pre-execution fee-report versions by default; use
`WithExecutionCorrelationLimit` to tune both finite bounds. For finer handling:

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

`IsWarning`, `IsFarmStatus`, and `IsConnectivityTransition` classify
informational codes. Delayed-data notice 10167 is normally a session `Event`
while the quote stream continues, so consumers that care about the downgrade
must also observe `Client.SessionEvents()`. See the
[session contract](docs/session-contract.md) for the full terminal and recovery
semantics.

## Examples

See the [`examples/` guide](examples/) for a short learning path from
connection and snapshots through option discovery, scanners, reconnecting
streams, paper order lifecycle, and margin previews.

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
domains through an idiomatic Go facade. It negotiates `server_version`
200..207. Version 200 is the live-attested classic-wire baseline; exact
raw-ID/protobuf migrations from 201 through 207 are implemented and
live-attested. Protocol boundaries from 208 onward and remaining advanced
branches stay explicit in the coverage matrix rather than being claimed as
supported.

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

# ibkr-go

[![CI](https://github.com/ThomasMarcelis/ibkr-go/actions/workflows/ci.yml/badge.svg)](https://github.com/ThomasMarcelis/ibkr-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ThomasMarcelis/ibkr-go.svg)](https://pkg.go.dev/github.com/ThomasMarcelis/ibkr-go)
[![Go Version](https://img.shields.io/github/go-mod-go-version/ThomasMarcelis/ibkr-go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A Go client for Interactive Brokers that works the way Go works. Call a
method, get a typed result. Subscribe, range over events. No callback
interfaces to implement, no global state, no dependencies outside the
standard library.

## Why ibkr-go

The official Interactive Brokers client uses an `EWrapper`/`EClient`
callback pattern inherited from Java. Most Go libraries port that
pattern directly. That means you implement a large callback interface,
wire up a client object, fire off requests, and then correlate responses
yourself across scattered handler methods using request IDs.

ibkr-go takes a different approach. You call a method and get back a
typed result, like any other Go API. Streaming data comes through
subscriptions that give you separate channels for business events and
lifecycle state. The session tells you when it reconnects, what was
lost, and lets your code decide what to do about it.

The library has zero external dependencies. There is nothing to audit
and no transitive version conflicts. Financial values use an exact
`Decimal` type instead of `float64`, so you don't accumulate rounding
errors on money. And the built-in replay harness means your CI runs
deterministically without needing a live broker connection.

## Install

```bash
go get github.com/ThomasMarcelis/ibkr-go@latest
```

Requires Go 1.26+. No external dependencies.

## Quick Start

### Connect and query contract details

```go
client, err := ibkr.DialContext(ctx,
    ibkr.WithHost("127.0.0.1"),
    ibkr.WithPort(7497),
)
if err != nil {
    return err
}
defer client.Close()

details, err := client.ContractDetails(ctx, ibkr.ContractDetailsRequest{
    Contract: ibkr.Contract{
        Symbol:   "AAPL",
        SecType:  "STK",
        Exchange: "SMART",
        Currency: "USD",
    },
})
if err != nil {
    return err
}
fmt.Println(details[0].LongName, details[0].MinTick)
```

### Stream live quotes

```go
sub, err := client.SubscribeQuotes(ctx, ibkr.QuoteSubscriptionRequest{
    Contract: ibkr.Contract{
        Symbol:   "AAPL",
        SecType:  "STK",
        Exchange: "SMART",
        Currency: "USD",
    },
})
if err != nil {
    return err
}
defer sub.Close()

for {
    select {
    case update := <-sub.Events():
        fmt.Println(update.Snapshot.Bid, update.Snapshot.Ask)
    case state := <-sub.State():
        fmt.Println("lifecycle:", state.Kind)
    case <-sub.Done():
        return sub.Wait()
    }
}
```

`Events()` carries market data. `State()` carries lifecycle —
`SnapshotComplete`, `Gap`, `Resumed`, `Closed`. They never mix.

## How It Compares

Several Go libraries exist for Interactive Brokers. They each solve
real problems. Here is how ibkr-go differs and why those differences
matter in practice.

| | ibkr-go | Typical IBKR Go library |
|---|---------|-------------------------|
| **Using the API** | Call a method, get a typed result. Subscribe, range over a channel. | Implement a callback interface. Correlate request IDs across handler methods. |
| **Dependencies** | None — stdlib only. Nothing to audit, no version conflicts. | 2-4 external packages (logging, decimal, protobuf). |
| **When the connection drops** | Session state is observable. Your code decides what to do with explicit reconnect and resume policies. | Reconnect manually. Hope your callbacks still make sense. |
| **Subscription lifecycle** | Typed states: `SnapshotComplete`, `Gap`, `Resumed`. You always know where you are in the data stream. | Callbacks fire or stop. Figuring out "did the snapshot finish?" is on you. |
| **Money and prices** | Exact `Decimal` type. No `float64` rounding on financial values. | `float64` or third-party fixed-point library. |
| **Testing without a broker** | Built-in deterministic replay harness. CI runs without credentials, contributors can test immediately. | Requires a live TWS or IB Gateway for every test run. |
| **Order writes** | Not yet — v1 covers the full read-only surface. v2 targets full order management. | Available in some libraries. |

### Per-library breakdown

| | ibkr-go | [scmhub/ibapi](https://github.com/scmhub/ibapi) | [hadrianl/ibapi](https://github.com/hadrianl/ibapi) | [gofinance/ib](https://github.com/gofinance/ib) |
|---|---------|--------------|----------------|--------------|
| API | Typed methods + generic subscriptions | EWrapper/EClient | EWrapper/IbClient | Channel engine |
| Dependencies | 0 | zerolog, robaho/fixed, protobuf | zap | 0 |
| Protocol | Built from first principles | Port of official client | Port of API 9.80 | Custom (2014) |
| Orders | Read-only (v1) | Yes | Yes | Partial |
| Status | Active | Active (2025) | Dormant (2021) | Dormant (2021) |
| License | MIT | MIT | MIT | LGPL-3.0 |

## API Overview

### Session

| Method | Returns | Description |
|--------|---------|-------------|
| `DialContext` | `*Client` | Connect and bootstrap a ready session |
| `Close` | `error` | Shut down the session |
| `Session` | `SessionSnapshot` | Current session state, managed accounts, server version |
| `SessionEvents` | `<-chan SessionEvent` | Observable session state changes |

### Account and Portfolio

| Method | Returns |
|--------|---------|
| `AccountSummary` / `SubscribeAccountSummary` | `[]AccountValue` / `*Subscription[AccountSummaryUpdate]` |
| `PositionsSnapshot` / `SubscribePositions` | `[]Position` / `*Subscription[PositionUpdate]` |
| `AccountUpdatesSnapshot` / `SubscribeAccountUpdates` | `[]AccountUpdate` / `*Subscription[AccountUpdate]` |
| `SubscribeAccountUpdatesMulti` | `*Subscription[AccountUpdateMultiValue]` |
| `SubscribePositionsMulti` | `*Subscription[PositionMulti]` |
| `SubscribePnL` / `SubscribePnLSingle` | `*Subscription[PnLUpdate]` / `*Subscription[PnLSingleUpdate]` |
| `FamilyCodes` | `[]FamilyCode` |
| `CompletedOrders` | `[]CompletedOrderResult` |

### Market Data

| Method | Returns |
|--------|---------|
| `QuoteSnapshot` / `SubscribeQuotes` | `Quote` / `*Subscription[QuoteUpdate]` |
| `SubscribeRealTimeBars` | `*Subscription[Bar]` |
| `SubscribeTickByTick` | `*Subscription[TickByTickData]` |
| `SubscribeHistoricalBars` | `*Subscription[Bar]` |
| `SetMarketDataType` | `error` |

### Contract and Reference

| Method | Returns |
|--------|---------|
| `ContractDetails` | `[]ContractDetails` |
| `QualifyContract` | `QualifiedContract` |
| `MatchingSymbols` | `[]MatchingSymbol` |
| `MarketRule` | `MarketRuleResult` |
| `SecDefOptParams` | `[]SecDefOptParams` |
| `SmartComponents` | `[]SmartComponent` |
| `MktDepthExchanges` | `[]DepthExchange` |

### Historical Data

| Method | Returns |
|--------|---------|
| `HistoricalBars` | `[]Bar` |
| `HeadTimestamp` | `time.Time` |
| `HistogramData` | `[]HistogramEntry` |
| `HistoricalTicks` | `HistoricalTicksResult` |

### Options

| Method | Returns |
|--------|---------|
| `CalcImpliedVolatility` | `OptionComputation` |
| `CalcOptionPrice` | `OptionComputation` |

### News

| Method | Returns |
|--------|---------|
| `NewsProviders` | `[]NewsProvider` |
| `NewsArticle` | `NewsArticleResult` |
| `HistoricalNews` | `[]HistoricalNewsItemResult` |
| `SubscribeNewsBulletins` | `*Subscription[NewsBulletin]` |

### Scanner

| Method | Returns |
|--------|---------|
| `ScannerParameters` | `string` (XML) |
| `SubscribeScannerResults` | `*Subscription[[]ScannerResult]` |

### Orders and Executions (observation)

| Method | Returns |
|--------|---------|
| `OpenOrdersSnapshot` / `SubscribeOpenOrders` | `[]OpenOrder` / `*Subscription[OpenOrderUpdate]` |
| `Executions` / `SubscribeExecutions` | `[]ExecutionUpdate` / `*Subscription[ExecutionUpdate]` |

## v1 Coverage

| Area | Status |
|------|--------|
| Bootstrap and session | landed |
| Account and portfolio | landed |
| Contract and reference data | landed |
| Market data | landed |
| Historical data extensions | landed |
| Option calculations | landed |
| News | landed |
| Scanner | landed |
| Order and execution observation | landed |

See [`docs/message-coverage.md`](docs/message-coverage.md) for the full
message matrix.

## Roadmap

v2 targets the complete Interactive Brokers API, including full order
management — placement, modification, and cancellation. See
[`docs/roadmap.md`](docs/roadmap.md) for the full plan.

## Development

```bash
go build ./...
go vet ./...
gofmt -l .           # must produce no output
golangci-lint run
go test ./...
```

All five must pass before opening a pull request. CI runs the same
checks on every push.

## Documentation

- [`docs/architecture.md`](docs/architecture.md) — layer design and runtime model
- [`docs/session-contract.md`](docs/session-contract.md) — public API contract
- [`docs/message-coverage.md`](docs/message-coverage.md) — protocol message matrix
- [`docs/transcripts.md`](docs/transcripts.md) — test transcript format
- [`docs/roadmap.md`](docs/roadmap.md) — v1 scope and v2 direction
- [`docs/anti-patterns.md`](docs/anti-patterns.md) — design philosophy

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md).

## License

MIT. See [`LICENSE`](LICENSE).

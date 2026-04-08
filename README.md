# ibkr-go

[![CI](https://github.com/ThomasMarcelis/ibkr-go/actions/workflows/ci.yml/badge.svg)](https://github.com/ThomasMarcelis/ibkr-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ThomasMarcelis/ibkr-go.svg)](https://pkg.go.dev/github.com/ThomasMarcelis/ibkr-go)
[![Go Version](https://img.shields.io/github/go-mod-go-version/ThomasMarcelis/ibkr-go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Go client for the Interactive Brokers TWS/Gateway socket protocol.
Call a method, get a typed result. Subscribe, range over a channel.
Zero dependencies. MIT license.

## Why ibkr-go

- **Full TWS API surface** — accounts, positions, market data, historical data, order management, market depth, options, news, scanners, FA configuration, and more. Not a partial port.
- **Idiomatic Go** — typed methods and generic subscriptions. No callback interfaces, no request ID correlation, no global state.
- **Tested against real traffic** — 82 replay transcripts captured from a live IB Gateway, fuzz-tested wire protocol, deterministic CI without broker credentials.
- **Zero dependencies** — standard library only. Nothing to audit, no transitive version conflicts.
- **Exact financial types** — `Decimal` for all prices and money. No `float64` rounding errors.
- **Observable session lifecycle** — typed reconnect states (`Gap`, `Resumed`), explicit subscription lifecycle (`SnapshotComplete`, `Closed`), connection-loss detection you can act on.
- **Built from first principles** — codec designed from the IBKR wire protocol specification and live captures, not ported from the official Java/Python clients.

## Install

```bash
go get github.com/ThomasMarcelis/ibkr-go@latest
```

Requires Go 1.26+. No external dependencies.

Full API reference on [pkg.go.dev](https://pkg.go.dev/github.com/ThomasMarcelis/ibkr-go/ibkr).

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

Several Go libraries exist for Interactive Brokers. Here is how they differ.

| | ibkr-go | [scmhub/ibapi](https://github.com/scmhub/ibapi) | [hadrianl/ibapi](https://github.com/hadrianl/ibapi) | [gofinance/ib](https://github.com/gofinance/ib) |
|---|---------|--------------|----------------|--------------|
| **API pattern** | Typed methods + channel subscriptions | EWrapper/EClient callbacks | IbWrapper callbacks | Channel engine |
| **API coverage** | Full TWS surface | Full (ported from official Python) | TWS API 9.80 subset | Partial (2014-era) |
| **Order management** | OrderHandle with lifecycle tracking | Yes | Yes | Partial |
| **Reconnect handling** | Observable state machine (Gap/Resumed) | Manual | Manual | Manual |
| **Test strategy** | 82 live-capture replays, fuzz, deterministic CI | Live broker required | Live broker required | Live broker required |
| **Price type** | Exact Decimal | robaho/fixed | float64 | float64 |
| **Dependencies** | 0 | 3 (zerolog, fixed, protobuf) | 1 (zap) | 0 |
| **Go version** | 1.26 (generics, iterators) | 1.21+ | 1.16+ | 1.x |
| **License** | MIT | MIT | MIT | LGPL-3.0 |
| **Maintained** | Active (2026) | Active (2025) | Dormant (2021) | Dormant (2021) |

## Roadmap

ibkr-go covers the complete Interactive Brokers TWS/Gateway socket protocol.
Ongoing work targets continuous maintenance against new TWS/Gateway versions,
expanded test coverage, and API ergonomics. See
[`docs/roadmap.md`](docs/roadmap.md) for details.

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
- [`docs/api-overview.md`](docs/api-overview.md) — full method reference
- [`docs/message-coverage.md`](docs/message-coverage.md) — protocol message matrix
- [`docs/transcripts.md`](docs/transcripts.md) — test transcript format
- [`docs/roadmap.md`](docs/roadmap.md) — project direction
- [`docs/anti-patterns.md`](docs/anti-patterns.md) — design philosophy

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md).

## License

MIT. See [`LICENSE`](LICENSE).

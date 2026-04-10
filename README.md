# ibkr-go

[![CI](https://github.com/ThomasMarcelis/ibkr-go/actions/workflows/ci.yml/badge.svg)](https://github.com/ThomasMarcelis/ibkr-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ThomasMarcelis/ibkr-go.svg)](https://pkg.go.dev/github.com/ThomasMarcelis/ibkr-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/ThomasMarcelis/ibkr-go)](https://goreportcard.com/report/github.com/ThomasMarcelis/ibkr-go)
[![Go Version](https://img.shields.io/badge/go-1.26-blue)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

ibkr-go is an idiomatic Go client for the Interactive Brokers TWS and IB
Gateway socket protocol. It gives you typed request/response methods for
snapshots, typed subscriptions for streams, and explicit lifecycle events for
reconnects and shutdown.

The library targets the full TWS/Gateway socket API surface, keeps the public
API small and Go-shaped, and treats protocol verification as a first-class
feature rather than an afterthought.

## Why ibkr-go

- **A Go-shaped API**: one-shots return typed results. Streams are typed
  subscriptions with `Events()`, `State()`, `Done()`, and `Wait()`. No
  `EWrapper` / `EClient` callback surface as the primary interface.
- **Full TWS/Gateway coverage**: accounts, positions, quotes, historical
  data, executions, order management, market depth, scanners, news, FA
  configuration, WSH, display groups, and more.
- **Protocol work backed by evidence**: replay scenarios derived from live IB
  Gateway traffic, wire and codec fuzzing, stress tests, and deterministic CI.
- **Reconnects are explicit**: session transitions and subscription lifecycle
  events such as `Gap`, `Resumed`, `SnapshotComplete`, and `Closed` are part of
  the contract.
- **Exact financial values**: `Decimal` is used for prices, quantities, and
  money throughout the API.
- **Standard library only**: zero external dependencies.

## Install

```bash
go get github.com/ThomasMarcelis/ibkr-go@latest
```

Requires Go 1.26+. Standard library only.

Full API reference on [pkg.go.dev](https://pkg.go.dev/github.com/ThomasMarcelis/ibkr-go/ibkr).

## Quick Start

The mental model is simple: call a method for a snapshot, subscribe for a
stream.

### Connect and query contract details

```go
client, err := ibkr.DialContext(ctx,
    ibkr.WithHost("127.0.0.1"),
    ibkr.WithPort(4001),
)
if err != nil {
    return err
}
defer client.Close()

details, err := client.QualifyContract(ctx, ibkr.Contract{
    Symbol:   "AAPL",
    SecType:  ibkr.SecTypeStock,
    Exchange: "SMART",
    Currency: "USD",
})
if err != nil {
    return err
}
fmt.Println(details.LongName, details.MinTick)
```

### Stream live quotes

```go
sub, err := client.SubscribeQuotes(ctx, ibkr.QuoteSubscriptionRequest{
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

## Testing and Verification

ibkr-go is being built with a test suite that is an asset ensuring correctness

- `90` checked-in replay transcripts under
  [`testdata/transcripts`](testdata/transcripts)
- `18` fuzz targets covering wire framing and codec round-trips
- Deterministic CI for routine verification, without broker credentials
- Separate live-gated tests for local verification against TWS or IB Gateway

The goal is a library whose protocol behavior can be frozen, replayed, stressed, and extended without guessing. For more on that approach, see [`docs/transcripts.md`](docs/transcripts.md) and [`docs/anti-patterns.md`](docs/anti-patterns.md).

## Compared with the usual Go IBKR shape

Most Go IBKR libraries either mirror the official `EWrapper` / `EClient`
callback model or stay close to it. That is a reasonable fit if you want a
familiar port of the official clients.

ibkr-go takes a different route:

- The primary interface is typed request methods and typed subscriptions
- Reconnect boundaries are explicit instead of hidden behind callbacks
- Protocol work is designed to be tested in deterministic CI, not only against
  a live broker
- The implementation owns the protocol and state machine directly instead of
  wrapping the official clients

## Status

ibkr-go currently covers the Interactive Brokers TWS/Gateway socket protocol
end to end. Ongoing work focuses on keeping pace with new Gateway versions,
expanding replay coverage, and tightening API ergonomics.

This repository is not targeting Flex, the Client Portal Web API, or an
`EWrapper` / `EClient` compatibility bridge. See
[`docs/roadmap.md`](docs/roadmap.md) for the full charter.

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

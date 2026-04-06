# ibkr-go

`ibkr-go` is a Go client for the Interactive Brokers TWS and IB
Gateway socket API. The library is built around a typed session engine,
typed one-shot requests, and typed subscriptions with explicit lifecycle and
reconnect semantics. It does not expose `EWrapper` / `EClient` as its primary
public surface.

## Status

The active repo docs are contract-first and implementation-backed. Historical
planning briefs have been removed. The current repo contains a real typed
session engine plus legacy replay tooling, but it is not yet a real IBKR
protocol client. New protocol work is grounded in live TWS / IB Gateway
behavior and official IBKR docs rather than the legacy symbolic harness. The
repository remains private until the v1 read-only core is complete.

## Install

```bash
go get github.com/ThomasMarcelis/ibkr-go@latest
```

## Public Shape

```go
client, err := ibkr.DialContext(ctx, ibkr.WithHost("127.0.0.1"), ibkr.WithPort(7497))
if err != nil {
    return err
}
defer client.Close()

snapshot := client.Session()
fmt.Println(snapshot.ManagedAccounts)

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

quotes, err := client.SubscribeQuotes(ctx, ibkr.QuoteSubscriptionRequest{
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
defer quotes.Close()
```

## Contracts

- `DialContext` returns a ready session, not a bare TCP connection.
- Managed accounts are bootstrap state on `SessionSnapshot`.
- Subscriptions expose `Events()`, `State()`, `Done()`, `Wait()`, and `Close()`.
- `State()` carries lifecycle events such as `SnapshotComplete`, `Gap`, and
  `Resumed`.
- Snapshot completion is driven by explicit protocol end markers.
- Numeric fields use an exact `Decimal` type rather than `float64` in the
  public contract.
- `WithClientID` overrides the handshake client ID. The default is `1`.
- `OpenOrdersScopeAuto` requires `WithClientID(0)`.
- `ResumeAuto` is currently supported on quote streams and real-time bars only.
- `SubscribeExecutions` is a finite snapshot subscription and closes after
  `SnapshotComplete`.

## Current Implementation Boundary

- The public/session/subscription contracts are real.
- Capture and normalization tooling for live transcript work is landed.
- Raw capture logs store connect/disconnect markers plus socket chunks;
  normalized replay artifacts reconstruct framed payloads offline.
- The current codec/testhost path is a legacy symbolic replay harness and not
  the desired implementation target.
- Real TWS / IB Gateway message compatibility is the next major step.

## Documentation

- [`docs/architecture.md`](docs/architecture.md)
- [`docs/session-contract.md`](docs/session-contract.md)
- [`docs/message-coverage.md`](docs/message-coverage.md)
- [`docs/transcripts.md`](docs/transcripts.md)
- [`docs/roadmap.md`](docs/roadmap.md)
- [`docs/anti-patterns.md`](docs/anti-patterns.md)
- [`AGENTS.md`](AGENTS.md)

## License

MIT. See [`LICENSE`](LICENSE).

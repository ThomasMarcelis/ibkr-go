# ibkr-go

A Go library for the Interactive Brokers TWS and IB Gateway socket API.

`ibkr-go` is a Go library for the Interactive Brokers TWS and IB Gateway socket API. It is a clean-room implementation focused on correctness, durable session semantics, explicit version negotiation, and an idiomatic typed Go API. It is not a port of the official IB client libraries and does not expose an `EWrapper` or `EClient` surface. The v1 target is a read-only production core covering accounts, positions, contract details, quotes, real-time and historical bars, and execution observation.

## Status

Day 1: project charter, clean-room policy, module skeleton, CI pipeline, and testing philosophy are locked. The v1 read-only protocol core lands in the next release milestone. The repository will flip to public once v1 is in place.

## Install

```
go get github.com/ThomasMarcelis/ibkr-go@latest
```

## Example

The public API direction is typed one-shot request methods and typed subscriptions. The shape below reflects the v1 target. Working code lands in the next release milestone.

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/ThomasMarcelis/ibkr-go/ibkr"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    client, err := ibkr.DialContext(ctx, ibkr.WithHost("127.0.0.1"), ibkr.WithPort(7497))
    if err != nil {
        panic(err)
    }
    defer client.Close()

    accounts, err := client.ManagedAccounts(ctx)
    if err != nil {
        panic(err)
    }
    fmt.Println("accounts:", accounts)

    summary, err := client.AccountSummary(ctx, accounts[0])
    if err != nil {
        panic(err)
    }
    fmt.Println("summary:", summary)
}
```

## Documentation

- [`docs/roadmap.md`](docs/roadmap.md) — v1 charter and explicit non-v1 list
- [`docs/provenance.md`](docs/provenance.md) — clean-room policy
- [`docs/anti-patterns.md`](docs/anti-patterns.md) — patterns this library deliberately rejects
- [`docs/feature-matrix.md`](docs/feature-matrix.md) — comparison with existing Go IBKR libraries
- [`AGENTS.md`](AGENTS.md) — engineering mindset, Go style, testing philosophy
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — how to contribute

## License

MIT. See [`LICENSE`](LICENSE).

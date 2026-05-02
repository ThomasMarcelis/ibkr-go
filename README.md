# ibkr-go

[![CI](https://github.com/ThomasMarcelis/ibkr-go/actions/workflows/ci.yml/badge.svg)](https://github.com/ThomasMarcelis/ibkr-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ThomasMarcelis/ibkr-go.svg)](https://pkg.go.dev/github.com/ThomasMarcelis/ibkr-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/ThomasMarcelis/ibkr-go)](https://goreportcard.com/report/github.com/ThomasMarcelis/ibkr-go)
[![Go Version](https://img.shields.io/badge/go-1.26-blue)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

An idiomatic Go client for Interactive Brokers TWS and IB Gateway, backed by
the official IBKR C++ SDK through a narrow repo-owned C ABI. The public API is
Go-shaped: typed one-shots, typed subscriptions, order handles, explicit
lifecycle events, and exact decimal arithmetic for financial values.

Current SDK-native coverage is intentionally narrow while the migration is in
progress: session/bootstrap, current time, current time millis, account
summary, account updates, account updates multi, positions, positions multi,
PnL, PnL single, family codes, market-data type control, contract
details/qualification, quote snapshots and streams, real-time bars,
tick-by-tick data, market depth streams, market-depth exchange metadata,
contract search, market rules, sec-def option params, smart components, head
timestamp, histogram data, fundamental data, news providers, news bulletins,
news articles, historical news, historical bars, historical bar subscriptions,
historical schedule, historical ticks, scanner parameters, scanner result
subscriptions, option implied-volatility and price calculations, option
exercise, FA config reads and writes, soft-dollar tiers,
WSH metadata/event data, user info, display groups, display group
subscriptions, order placement and modification, open-order snapshots and
subscriptions, completed-order snapshots, execution snapshots, commission
reports, and cancellation for account summary, account updates, account updates
multi, positions, positions multi, PnL, PnL single, order cancellation, global
cancellation, news bulletins, scanner subscriptions, display group subscriptions, real-time bars,
tick-by-tick data, market depth, historical bars, historical ticks, option
calculations, head timestamp, histogram data, WSH metadata/event data, and
fundamental data flows.
Rows without current live SDK evidence are blocked with the exact missing
external prerequisite in the migration matrix.
See
[`docs/sdk-migration-matrix.md`](docs/sdk-migration-matrix.md).

```go
client, _ := ibkr.DialContext(ctx, ibkr.WithHost("127.0.0.1"), ibkr.WithPort(4002))
defer client.Close()

// one-shot: typed result, blocks until the SDK completion callback
positions, _ := client.Accounts().Positions(ctx)
```

## Install

```bash
go get github.com/ThomasMarcelis/ibkr-go@latest
```

Requires Go 1.26+, cgo on Linux for production use, and a locally installed
official IBKR TWS API SDK. One Go dependency:
[shopspring/decimal](https://github.com/shopspring/decimal) for exact financial
arithmetic.

Validate the SDK and export the required cgo flags before SDK-backed builds:

```bash
scripts/check-ibkr-sdk-env.sh /path/to/IBJts
eval "$(scripts/check-ibkr-sdk-env.sh --print-env /path/to/IBJts)"
go test -tags=ibkr_sdk ./...
```

Without `-tags=ibkr_sdk` on Linux with cgo, exported APIs still compile, but
`DialContext` fails closed with an SDK-runtime-unavailable error.

Full API reference on [pkg.go.dev](https://pkg.go.dev/github.com/ThomasMarcelis/ibkr-go).

## Why ibkr-go

- **Go-shaped API.** One-shots return typed results. Streams return typed
  subscriptions with `Events()`, `Lifecycle()`, and `Done()`. No `EWrapper` /
  `EClient` callback surface.
- **SDK-owned protocol I/O.** TWS/Gateway bytes are delegated to the official
  SDK. Go owns the public API, lifecycle, routing, validation, and tests.
- **Honest migration boundary.** The current SDK-native runtime supports
  session/bootstrap, current time, account summary, account updates, account
  updates multi, positions, positions multi, PnL, PnL single, family codes,
  contract details, market-data type control, quote snapshots and streams,
  contract search, market rules, sec-def option params, smart components,
  real-time bars, tick-by-tick data, market depth streams, market-depth exchange
  metadata, head timestamp, histogram data, fundamental data, news providers, news articles,
  historical news, news bulletins, scanner parameters,
  scanner result subscriptions, historical bars, historical bar subscriptions,
  historical schedule, historical ticks, option implied-volatility and price
  calculations, option exercise, soft-dollar tiers, FA config reads and writes,
  WSH metadata/event data, user info,
  display groups, display group subscriptions, order placement and modification,
  open-order snapshots and subscriptions, completed-order snapshots, execution
  snapshots, commission reports, order cancellation, and global cancellation.
  Unsupported command gaps in the advertised facade have been closed; remaining
  non-done rows are blocked by external prerequisites such as entitlements,
  market hours, account type, or paper-account safety state.
- **Reconnects are explicit.** Session transitions and subscription lifecycle
  events - `Gap`, `Resumed`, `SnapshotComplete`, `Closed` - are part of the
  contract, not hidden behind callbacks.
- **Exact financial values.** [`decimal.Decimal`](https://github.com/shopspring/decimal)
  for prices, quantities, and money throughout the API — no float64 rounding.

## Quick Start

The current SDK-backed quick path is session bootstrap plus read-only one-shots
that already have native adapter coverage.

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
```

Order placement and modification are SDK-native through official `placeOrder`,
but paper-account live verification is still required before treating them as
release-complete.

## API Shape

Every domain is accessed through a facade on the client:

| Facade | Public methods | SDK-native today |
|--------|----------------|------------------|
| `client.Accounts()` | `Summary`, `Positions`, `SubscribeSummary`, `SubscribePositions`, `Updates`, `FamilyCodes`, PnL and multi-account methods | `Summary`, `Positions`, `SubscribeSummary`, `SubscribePositions`, `Updates`, `SubscribeUpdates`, `UpdatesMulti`, `SubscribeUpdatesMulti`, `PositionsMulti`, `SubscribePositionsMulti`, `SubscribePnL`, `SubscribePnLSingle`, `FamilyCodes` |
| `client.Contracts()` | `Qualify`, `Details`, `Search`, `MarketRule`, `SecDefOptParams`, `SmartComponents`, `DepthExchanges`, `FundamentalData` | `Qualify`, `Details`, `Search`, `MarketRule`, `SecDefOptParams`, `SmartComponents`, `DepthExchanges`, `FundamentalData` |
| `client.MarketData()` | `SetType`, `Quote`, `SubscribeQuotes`, `SubscribeRealTimeBars`, `SubscribeTickByTick`, `SubscribeDepth` | `SetType`, `Quote`, `SubscribeQuotes`, `SubscribeRealTimeBars`, `SubscribeTickByTick`, `SubscribeDepth` |
| `client.History()` | `Bars`, `SubscribeBars`, `HeadTimestamp`, `Histogram`, `Ticks`, `Schedule` | `Bars`, `SubscribeBars`, `HeadTimestamp`, `Histogram`, `Ticks`, `Schedule` |
| `client.Orders()` | `Place`, `Cancel`, `CancelAll`, `Open`, `SubscribeOpen`, `Completed`, `Executions` | `Place`, `Cancel`, `CancelAll`, `Open`, `SubscribeOpen`, `Completed`, `Executions` |
| `client.Options()` | `ImpliedVolatility`, `Price`, `Exercise` | `ImpliedVolatility`, `Price`, `Exercise` |
| `client.News()` | `Providers`, `Article`, `Historical`, `SubscribeBulletins` | `Providers`, `Article`, `Historical`, `SubscribeBulletins` |
| `client.Scanner()` | `Parameters`, `SubscribeResults` | `Parameters`, `SubscribeResults` |
| `client.Advisors()` | `Config`, `ReplaceConfig`, `SoftDollarTiers` | `Config`, `ReplaceConfig`, `SoftDollarTiers` |
| `client.WSH()` | `MetaData`, `EventData` | `MetaData`, `EventData` |
| `client.TWS()` | `UserInfo`, `DisplayGroups`, `SubscribeDisplayGroup` | `UserInfo`, `DisplayGroups`, `SubscribeDisplayGroup` |

One-shots return `(T, error)` or `([]T, error)`. Subscriptions return
`*Subscription[T]` with `Events()`, `Lifecycle()`, `Done()`, and `Close()`.
Order placement returns `*OrderHandle` with the same channel pattern plus
`Cancel()` and `Modify()`.

## Examples

The [`examples/`](examples/) directory still mirrors the intended public API.
During the SDK migration, only examples that stay inside the current
SDK-native slice are expected to run against a local paper IB Gateway:

```bash
IBKR_ADDR=127.0.0.1:4002 go run ./examples/connect       # session info
```

The quote, historical, portfolio/PnL, and order examples are retained as API
shape examples and should be re-enabled as runnable SDK-backed examples when
their rows in the migration matrix are complete.

## Testing and Verification

Every SDK-backed behavior this library claims should have a test pinning it
down. Current coverage is strongest for the adapter foundation, SDK command
routing, native callback conversion, and deterministic facade route tests;
replay fixtures and live evidence are tracked in the migration matrix.

- Deterministic route and SDK-adapter fixture tests for routine verification
- Native ABI tests for allocation/free, copied values, and failed submission
  paths when `-tags=ibkr_sdk` is enabled
- Separate live-gated tests for local verification against TWS or IB Gateway.
  The paper Gateway default is `127.0.0.1:4002`; override with
  `IBKR_LIVE_ADDR` when needed.

The goal is a library whose SDK callback behavior can be frozen, replayed,
stressed, and extended without guessing. For local SDK setup, see
[`docs/official-sdk.md`](docs/official-sdk.md).

## Status

ibkr-go is being cut over to one production runtime: the official IBKR C++ SDK
behind the C ABI in `internal/sdkadapter/native`. Ongoing work expands
live-derived SDK fixtures and paper-account evidence while preserving the public
Go API. Unsupported command gaps in advertised facades are closed; the old
socket runtime is not a fallback.

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
bash -n scripts/check-ibkr-sdk-env.sh
bash -n scripts/scan-ibkr-sdk-drift.sh
scripts/check-ibkr-sdk-env.sh /path/to/IBJts
scripts/scan-ibkr-sdk-drift.sh /path/to/IBJts
eval "$(scripts/check-ibkr-sdk-env.sh --print-env /path/to/IBJts)"
go test -tags=ibkr_sdk ./...
```

These checks should pass before opening a pull request. CI runs the default
deterministic subset on every push.

Local live verification is opt-in:

```bash
IBKR_LIVE=1 IBKR_LIVE_ADDR=127.0.0.1:4002 go test -tags=ibkr_sdk ./... -run '^TestLive' -count=1
IBKR_LIVE=1 IBKR_LIVE_TRADING=1 IBKR_LIVE_ADDR=127.0.0.1:4002 go test -tags=ibkr_sdk ./... -run '^TestLive(OfficialSDKPaperOrder|PlaceOrder|GlobalCancel|Trading)' -count=1
```

`IBKR_LIVE_TRADING=1` permits paper-account order placement and marketable
test orders. SDK-backed paper-order tests refuse to place orders unless the
connected managed accounts look like paper accounts. Read-only live smoke tests
do not require it.

## Documentation

- [`docs/session-contract.md`](docs/session-contract.md) — public API contract
- [`docs/anti-patterns.md`](docs/anti-patterns.md) — design philosophy

For contributors and maintainers:

- [`docs/architecture.md`](docs/architecture.md) — internal layer design
- [`docs/official-sdk.md`](docs/official-sdk.md) — manual cgo SDK runtime setup
- [`docs/roadmap.md`](docs/roadmap.md) — project direction

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md).

## License

MIT. See [`LICENSE`](LICENSE).

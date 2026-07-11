# Migrating from v1 to v2

v2 is an intentional clean break. Go semantic import versioning keeps existing v1 applications on v1 until they explicitly adopt the new module path.

## Adopt the v2 module

Update the dependency and every ibkr-go import:

```bash
go get github.com/ThomasMarcelis/ibkr-go/v2@v2.0.0-rc.1
```

```go
// Before
import "github.com/ThomasMarcelis/ibkr-go"

// After
import "github.com/ThomasMarcelis/ibkr-go/v2"
```

## Close and order lifecycle

`Client.Close()`, `Subscription.Close()`, and `OrderHandle.Close()` are commands and return no value. Observe terminal results with `Wait()` or `Err()` when needed.

An `OrderHandle` no longer closes automatically on Filled, Cancelled, ApiCancelled, or Inactive because IBKR may deliver executions and commission-and-fees reports after those statuses. Keep consuming events until the application has the evidence it needs, then close the handle explicitly.

```go
for event := range handle.Events() {
	record(event)
	if observationComplete(event) {
		handle.Close()
	}
}
if err := handle.Wait(); err != nil {
	// Observation ended because of an API, transport, or consumer error.
}
```

`OrderHandle.Modify` is now `OrderHandle.Replace`.

## Executions and historical news

`Orders().Executions` returns `ExecutionSnapshot`, whose `Executions` and `CommissionAndFees` slices contain everything observed through IBKR's execution-details end marker.

```go
snapshot, err := client.Orders().Executions(ctx, request)
fills := snapshot.Executions
fees := snapshot.CommissionAndFees
```

Use `SubscribeExecutions` when late or revised fee reports matter. It remains open after the end marker and must be closed explicitly.

`HistoricalNews` returns `HistoricalNewsResult`. Read articles from `Items` and continue pagination when `HasMore` is true. `Options().Exercise` now returns an `ExerciseHandle`.

## Order and contract ownership

Order identity and preview mode no longer live on `Order`. `Place` allocates an ID, `Replace` uses its handle's ID, and `Preview` owns the wire-level what-if flag.

```go
// Before
order.OrderID = handle.OrderID()
order.WhatIf = new(true)

// After
err := handle.Replace(ctx, order)
state, err := client.Orders().Preview(ctx, request)
```

Contract selection and composition now live on `Contract`, including combo legs, delta-neutral data, security IDs, and `IncludeExpired`. `Contract.Strike` is a presence-aware decimal.

```go
// Before
Contract{Strike: "150"}

// After
Contract{Strike: new(decimal.NewFromInt(150))}
```

`OpenOrder.LmtPrice` and `OpenOrder.AuxPrice` are also `*decimal.Decimal`. Nil means omitted or unset; a pointer to zero means an explicit zero. `LmtPriceOffset` moved from `OrderAdjustment` to `Order`.

## Other source migrations

| Before | After |
|---|---|
| `Buy`, `Sell` | `ActionBuy`, `ActionSell` |
| `CommissionReport` | `CommissionAndFeesReport` |
| Flat completed-order projection | `Contract`, `Order`, and `Completion` facets |
| `Forex(code) Contract` | `Forex(code) (Contract, error)` |
| `AccountSummaryRequest.Account` | `Group` plus optional `AccountFilter` |
| Wrapped account-summary and position events | Direct `AccountValue` and `Position` values |
| `WithDefaultResumePolicy` | `WithResumePolicy(ResumeAuto)` on each supported subscription |
| `internal/transport.Dialer` in signatures | Public `ibkr.Dialer` |

FA configuration mutation, Reuters fundamental data, `ibkr-probe`, and pre-200 Gateway compatibility were removed. FA reads support Groups and Aliases only. WSH inputs and returned documents must contain valid non-empty JSON. Classic protocol fields cannot contain embedded NUL bytes.

## Gateway errors and added operations

Session events produced by Gateway errors now set `Event.APIError`, preserving the request ID, server time, and advanced-order-reject JSON. Existing event code and message fields remain available for simple notification handling.

`MarketData().RegulatorySnapshot` is distinct from an ordinary quote snapshot and may incur an IBKR fee. `OpenOrderUpdate.Binding` is populated by `orderBound` only for the client-0 auto-open-orders subscription that owns that callback's scope.

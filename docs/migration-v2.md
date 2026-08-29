# Migrating from v1 to v2

v2 is the supported release line and an intentional source-breaking release.
Existing v1 applications stay on v1 until they adopt the `/v2` module path.
v2 requires Go 1.26.

This guide covers the changes that normally require application edits. The
[v2.0.0 release notes](https://github.com/ThomasMarcelis/ibkr-go/releases/tag/v2.0.0)
contain a detailed change inventory and the disclosed evidence gaps.

## v2.0.1 and later

v2.0.1 intentionally raises the minimum negotiated Gateway version from
`server_version` 200 to 208. Gateways negotiating 200–207 no longer connect;
upgrade TWS/IB Gateway before adopting v2.0.1, or remain on v2.0.0.

`Order.IncludeOvernight` changes from `bool` to `*bool`. Use `nil` when the
field is omitted, `new(true)` to request overnight routing, and `new(false)`
for an explicit false request. Broker echoes remain presence-aware: the live
sv225 Gateway accepted a fresh explicit-false order but canonicalized the echo
to `nil` with `TIF=DAY`; it rejected replacing an existing true order with
false using code 462.

## 1. Update the module path

```bash
go get github.com/ThomasMarcelis/ibkr-go/v2@v2.0.3
```

```go
// Before
import "github.com/ThomasMarcelis/ibkr-go"

// After
import "github.com/ThomasMarcelis/ibkr-go/v2"
```

A local replacement still needs a real v2 requirement:

```go
require github.com/ThomasMarcelis/ibkr-go/v2 v2.0.3

replace github.com/ThomasMarcelis/ibkr-go/v2 => ../ibkr-go
```

## 2. Migrate subscriptions

`Subscription.Events()` now returns one ordered stream of data, notices, and
lifecycle boundaries. The separate `Lifecycle()` channel and redundant closed
event are gone.

```go
// Before
select {
case value := <-sub.Events():
    use(value)
case state := <-sub.Lifecycle():
    useState(state)
}

// After
for event := range sub.Events() {
    switch event.Kind {
    case ibkr.StreamData:
        use(event.Value)
    case ibkr.StreamNotice:
        log.Print(event.Notice)
    case ibkr.StreamGap, ibkr.StreamRestored, ibkr.StreamResubscribed:
        useState(event)
    }
}
return sub.Wait()
```

Use `sub.All(ctx)` when only data matters. It consumes and discards notices and
lifecycle events. `Events()` and `All(ctx)` read the same queue, so choose one
and have exactly one goroutine drain it.

`Client.Close()`, `Subscription.Close()`, and `OrderHandle.Close()` are now
commands with no return value. Use `Wait()` or `Err()` for the terminal result.

v2 never drops business data to keep a slow consumer alive. Subscriptions,
finite streams, and order handles end with `ErrSlowConsumer` when their queue
fills. Configure capacities with `WithSubscriptionBuffer`,
`WithOrderEventBuffer`, or per-subscription `WithQueueSize`.

Transport reconnect and request replay are separate:

- the client still defaults to `ReconnectAuto`;
- request-backed subscriptions default to `ResumeNever`;
- `ResumeAuto` is supported only by streaming quotes and real-time bars;
- configure it per subscription with `WithResumePolicy(ResumeAuto)`.

After a continuity loss, a non-resumed long-lived subscription ends with
`ErrResumeRequired`; a finite request ends with `ErrInterrupted`. Both are
retryable. A same-socket data-lost restoration (Gateway code 1101) follows the
same rule.

## 3. Migrate order handling

`OrderHandle.Modify` is now `OrderHandle.Replace`. Placement owns order-ID
allocation, replacement reuses the handle's ID, and preview owns the what-if
operation.

```go
// Before
order.OrderID = handle.OrderID()
order.WhatIf = new(true)

// After
if err := handle.Replace(ctx, order); err != nil {
    return err
}
state, err := client.Orders().Preview(ctx, request)
```

An order handle stays open after Filled, Cancelled, APICancelled, or Inactive
because executions and commission-and-fees reports may arrive later. Drain the
events your application needs, then close the handle explicitly:

```go
for event := range handle.Events() {
    record(event)
    if observationComplete(event) {
        handle.Close()
    }
}
return handle.Wait()
```

After a physical reconnect or data-lost restoration, the handle publishes
`RecoveryRequired` and permanently rejects `Replace` with
`ErrOrderRecoveryRequired`. Reconcile open orders, executions, and completed
orders before deciding what to do. Stable-ID cancellation remains available
when IBKR's client-ID ownership permits it.

If `PlaceBracket` admits only part of a bracket, it returns
`*OrderRecoveryError`. The error contains every admitted order ID, retains the
independent placement and cancellation-admission causes, and matches
`ErrOrderRecoveryRequired` through `errors.Is`.

v2 deliberately has no restart-time adopt-or-replace operation. Orders
rediscovered through `Open`, `Executions`, or `Completed` do not create an
`OrderHandle`.

Open-order refresh now belongs to the subscription that owns the response
stream:

```go
sub, err := client.Orders().SubscribeOpen(ctx, ibkr.OpenOrdersScopeAll)
if err != nil {
    return err
}
if err := sub.Refresh(ctx); err != nil {
    return err
}
```

Replace `Orders().RefreshOpen(ctx)` with `sub.Refresh(ctx)`. Overlapping
refreshes return `ErrOperationActive`; auto scope returns `ErrNoSnapshot`.

## 4. Update result shapes

v2 models broker callbacks by domain instead of flattening unrelated fields.

| v1 shape | v2 shape |
|---|---|
| Flat `OpenOrder` fields | `OpenOrder.Contract`, `OpenOrder.Order`, `OpenOrder.State` |
| Flat completed order | `CompletedOrderResult.Contract`, `.Order`, `.Completion` |
| `Orders().Executions` update slice | `ExecutionSnapshot` with `Executions` and `CommissionAndFees` |
| Historical-news slice | `HistoricalNewsResult.Items` plus broker `HasMore` metadata |
| Exercise result | `ExerciseHandle` with events and `Wait()` |
| Wrapped account summary/position events | Direct `AccountValue` and `Position` values |

Examples:

```go
fmt.Println(open.Order.Prices.LmtPrice)
fmt.Println(open.State.InitMarginBefore)

snapshot, err := client.Orders().Executions(ctx, request)
fills := snapshot.Executions
fees := snapshot.CommissionAndFees

fmt.Println(completed.Order.Action)
fmt.Println(completed.Completion.Status)
```

`SubscribeExecutions` remains open after its initial end marker so it can
deliver late or revised fees. `SubscribeExecutionEvents` is different: it is a
passive, unfiltered observer of execution and commission callbacks and sends no
Gateway query.

`News().Historical` returns one captured Gateway page. `HistoricalAll` was
removed because live evidence does not establish a safe pagination cursor;
do not derive one from `HasMore`.

## 5. Handle presence explicitly

Values the Gateway may omit now use pointers. Nil means omitted; a non-nil zero
means IBKR explicitly sent zero.

- `Contract.Strike` and optional order prices use `*decimal.Decimal`.
- `Order.MinQty` and combo-leg `ExemptCode` use `*int`.
- Optional portfolio, PnL, commission, depth, and permission values use
  pointers.
- Order echoes preserve presence for IDs, transmit flags, overnight routing,
  and version-gated attributes.

```go
contract := ibkr.Contract{
    Symbol: "AAPL",
    Strike: new(decimal.NewFromInt(150)),
}

if update.UnrealizedPnL != nil {
    fmt.Println(*update.UnrealizedPnL)
}
```

Audit direct dereferences and equality checks. `Contract` and several request
types now contain slices and are no longer comparable, so derive explicit map
keys from application-owned fields.

Protocol identifiers that are signed 32-bit values on the wire now use named
types such as `ContractID`, `ClientID`, `RequestID`, `MarketRuleID`,
`AggregateGroupID`, and `DisplayGroupID`. Convert application integers at the
boundary. Public order IDs remain `int64`, but valid wire values are
1..2147483647.

## 6. Update order configuration

Contract identity and composition belong to `Contract`; execution instructions
belong to `Order`. Advanced order settings are grouped by behavior:

| v1 `Order` fields | v2 field |
|---|---|
| `OcaGroup`, `OcaType` | `OCA.Group`, `OCA.Type` |
| Scale sizing, increments, table, and active window | `Scale` |
| `HedgeType`, `HedgeParam`, `DontUseAutoPriceForHedge` | `Hedge` |
| `AlgoStrategy`, `AlgoParams` | `Algorithm` |
| Conditions and their flags | `Conditions.Values`, `.IgnoreRTH`, `.CancelOrder` |
| Adjustable-order fields | `Adjustment` |
| Combo leg prices and smart-routing parameters | `Combo.LegPrices`, `.SmartRouting` |

```go
order.OCA = ibkr.OrderOCA{
    Group: "exit",
    Type:  ibkr.OCACancelWithBlock,
}
order.Algorithm = ibkr.OrderAlgorithm{
    Strategy: "Adaptive",
    Params: []ibkr.TagValue{
        {Tag: "adaptivePriority", Value: "Normal"},
    },
}
```

Combo definitions live in `Contract.ComboLegs`; per-leg prices and routing
instructions remain under `Order.Combo`. Conditions, actions, sides, and other
wire enums use named Go types instead of raw strings or integers.

## 7. Apply the remaining renames

| Before | After |
|---|---|
| `Buy`, `Sell` | `ActionBuy`, `ActionSell` |
| `CommissionReport` | `CommissionAndFeesReport` |
| `OrderStatusApiCancelled` | `OrderStatusAPICancelled` |
| `Execution.Symbol` | `Execution.Contract.Symbol` |
| raw execution side strings | `ExecutionSideBought`, `ExecutionSideSold` |
| `UnrealizedPNL`, `RealizedPNL` | `UnrealizedPnL`, `RealizedPnL` |
| `AccountSummaryRequest.Account` | `Group` and optional `AccountFilter` |
| `WithDefaultResumePolicy` | per-subscription `WithResumePolicy` |
| internal transport dialer in signatures | public `ibkr.Dialer` |

`NewsArticle.ArticleType` is now `NewsArticleType`; use
`NewsArticleTypeText` or `NewsArticleTypeBinary`. Tick attribute fields use
typed bitmasks with accessors while preserving unknown bits.

## 8. Account for removed scope

v2 removes unsupported or ungrounded surfaces rather than carrying
compatibility shims:

- Reuters fundamental data;
- FA configuration mutation;
- `ibkr-probe`;
- pre-`server_version 200` compatibility;
- multipage historical-news inference;
- public repository testhost/live-helper packages.

FA reads support Groups and Aliases. External tests should use the public
`Dialer` seam or application-owned captured fixtures.

## 9. Verify the migration

Before shipping:

1. Search for the old import path and renamed methods or constants.
2. Audit every pointer-valued field for omitted-versus-zero handling.
3. Ensure each stream has one consumer and every long-lived handle is closed.
4. Exercise reconnect, code 1101, slow-consumer, partial-bracket, and uncertain
   exercise or regulatory-snapshot paths relevant to the application.
5. Run `go test ./...` and the application's race tests.

For protocol coverage, added operations, internal hardening, removals, and
additional type changes, see the detailed inventory at the end of the
[v2.0.0 release notes](https://github.com/ThomasMarcelis/ibkr-go/releases/tag/v2.0.0).

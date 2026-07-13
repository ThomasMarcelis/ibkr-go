# Migrating from v1 to v2

v2 is an intentional clean break. Go semantic import versioning keeps existing v1 applications on v1 until they explicitly adopt the new module path.

## Adopt the v2 module

Update the dependency and every ibkr-go import:

```bash
go get github.com/ThomasMarcelis/ibkr-go/v2@v2.0.0-rc.2
```

```go
// Before
import "github.com/ThomasMarcelis/ibkr-go"

// After
import "github.com/ThomasMarcelis/ibkr-go/v2"
```

For a local checkout, keep a real v2 requirement; `replace` changes source
location, not the module's semantic major version:

```go
require github.com/ThomasMarcelis/ibkr-go/v2 v2.0.0-rc.2

replace github.com/ThomasMarcelis/ibkr-go/v2 => ../ibkr-go
```

## From rc.2 to the current release candidate

Open-order refresh now belongs to the exact subscription that owns the
request-ID-less response stream:

```go
sub, err := client.Orders().SubscribeOpen(ctx, ibkr.OpenOrdersScopeAll)
if err != nil {
    return err
}
if err := sub.Refresh(ctx); err != nil {
    return err
}
```

Replace `Orders().RefreshOpen(ctx)` with `sub.Refresh(ctx)`. The former
`ErrNoSubscription` sentinel is gone because there is no longer a global
refresh lookup; overlapping refreshes return `ErrOperationActive`, and auto
scope returns `ErrNoSnapshot`.

## Subscription events

Subscriptions now expose one ordered stream. Data and reconnect boundaries no
longer race across `Events()` and `Lifecycle()` channels, and terminal state is
channel close plus `Err()`/`Wait()` rather than a redundant Closed event.

```go
// Before
select {
case value := <-sub.Events():
    use(value)
case state := <-sub.Lifecycle():
    useState(state)
}

// After: consume data and lifecycle in order
for event := range sub.Events() {
    switch event.Kind {
    case ibkr.StreamData:
        use(event.Value)
    case ibkr.StreamGap, ibkr.StreamRestored, ibkr.StreamResubscribed:
        useState(event)
    }
}
return sub.Err()
```

Use `sub.All(ctx)` when only data values matter. `Events()` and `All(ctx)`
consume the same queue and must not be read concurrently. `Restored` means the
Gateway retained the stream; `Resubscribed` means the client sent the request
again.

## Close and order lifecycle

`Client.Close()`, `Subscription.Close()`, and `OrderHandle.Close()` are commands and return no value. Observe terminal results with `Wait()` or `Err()` when needed.

An `OrderHandle` no longer closes automatically on Filled, Cancelled,
APICancelled, or Inactive because IBKR may deliver executions and
commission-and-fees reports after those statuses. Keep consuming events until
the application has the evidence it needs, then close the handle explicitly.

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

Order lifecycle transitions are now `OrderEvent.Lifecycle` values in the same
ordered channel. After a physical connection gap, `RecoveryRequired` means
fills or status changes may have occurred. Reconcile open orders, executions,
and completed orders for business decisions; that handle remains permanently
blocked from `Replace` because reconciliation cannot restore its lost event
history. The lifecycle event and later replacement calls match non-retryable
`ErrOrderRecoveryRequired`; `ErrResumeRequired` remains subscription-only.
Cancellation remains safe by stable order ID.

There is no restart-time adopt or replace-by-ID API in v2. After a process
restart, reconcile through open orders, executions, and completed orders;
`Orders().Cancel` remains available by stable order ID. Do not synthesize an
`OrderHandle` for a pre-existing order.

Cancellation methods accept optional compliance metadata without changing old
call sites:

```go
err := handle.Cancel(ctx,
    ibkr.WithManualCancelTime(time.Now()),
    ibkr.WithCancelExternalOperator("operator"),
    ibkr.WithCancelManualOrderIndicator(1),
)
```

## Executions and historical news

`Orders().Executions` returns `ExecutionSnapshot`, whose `Executions` and `CommissionAndFees` slices contain everything observed through IBKR's execution-details end marker.

```go
// Before
updates, err := client.Orders().Executions(ctx, request)
for _, update := range updates {
    // update.Execution or update.Commission
}

// After
snapshot, err := client.Orders().Executions(ctx, request)
fills := snapshot.Executions
fees := snapshot.CommissionAndFees
```

Use `SubscribeExecutions` when late or revised fee reports matter. It remains open after the end marker and must be closed explicitly.

`HistoricalNews` returns `HistoricalNewsResult`. Read articles from `Items` and
continue pagination when `HasMore` is true. `Options().Exercise` now returns an
`ExerciseHandle`; the returned handle proves local transport admission, not
IBKR acceptance or settlement. If observation ends involuntarily without a
definitive request-scoped API rejection, `Wait` returns non-retryable
`*ExerciseUncertainError` while preserving the underlying cause. Callers must
reconcile the resulting account or position before deciding what to do next.

Completed orders are split into the wire's real facets instead of projecting
fields the callback does not carry:

```go
// Before
fmt.Println(result.Action, result.Quantity, result.Status)

// After
fmt.Println(result.Order.Action, result.Order.Quantity, result.Completion.Status)
```

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

`OpenOrder.LmtPrice`, `OpenOrder.AuxPrice`, and optional `Order` decimals such as
`LmtPriceOffset` are now `*decimal.Decimal`. Nil means omitted or unset; a
pointer to zero means an explicit zero. `LmtPriceOffset` remains on `Order`; only
its representation changed.

The margin/commission block formerly flattened onto `OpenOrder` is now the
`OpenOrder.State` facet, shared with preview semantics:

```go
// Before
fmt.Println(open.InitMarginBefore, open.Commission)

// After
fmt.Println(open.State.InitMarginBefore, open.State.CommissionAndFees)
```

## Advanced orders and combos

Advanced settings are grouped by behavior instead of adding every protocol
field to the top-level `Order`:

| v1 `Order` fields | v2 field |
|---|---|
| `OcaGroup`, `OcaType` | `OCA.Group`, `OCA.Type` |
| `ScaleInitLevelSize`, `ScaleSubsLevelSize`, `ScalePriceIncrement`, `ScaleTable`, `ActiveStartTime`, `ActiveStopTime` | `Scale` |
| `HedgeType`, `HedgeParam`, `DontUseAutoPriceForHedge` | `Hedge` |
| `AlgoStrategy`, `AlgoParams` | `Algorithm` |
| `Conditions`, `ConditionsIgnoreRTH`, `ConditionsCancelOrder` | `Conditions.Values`, `Conditions.IgnoreRTH`, `Conditions.CancelOrder` |
| `AdjustedOrderType`, `TriggerPrice`, `AdjustedStopPrice`, `AdjustedStopLimitPrice`, `AdjustedTrailingAmount`, `AdjustableTrailingUnit` | `Adjustment` |
| `OrderComboLegPrices`, `SmartComboRoutingParams` | `Combo.LegPrices`, `Combo.SmartRouting` |

```go
// Before
order.OcaGroup = "exit"
order.OcaType = 1
order.AlgoStrategy = "Adaptive"
order.AlgoParams = []ibkr.TagValue{{Tag: "adaptivePriority", Value: "Normal"}}

// After
order.OCA = ibkr.OrderOCA{Group: "exit", Type: ibkr.OCACancelWithBlock}
order.Algorithm = ibkr.OrderAlgorithm{
    Strategy: "Adaptive",
    Params: []ibkr.TagValue{{Tag: "adaptivePriority", Value: "Normal"}},
}
```

Contract leg definitions live in `Contract.ComboLegs`; per-leg prices and smart
routing remain order instructions under `Order.Combo`. `ComboLeg.Action` is an
`OrderAction`, `OpenClose` is `ComboLegOpenClose`, and `ExemptCode` is `*int`
so absence differs from explicit zero.

```go
leg := ibkr.ComboLeg{
    ConID: conID, Ratio: 1, Action: ibkr.ActionBuy,
    Exchange: "SMART", OpenClose: ibkr.ComboLegOpen,
    ExemptCode: new(-1),
}
```

`OrderCondition` similarly uses named `OrderConditionType`,
`ConditionConjunction`, and `ConditionOperator` values instead of raw ints and
strings.

## Execution and P&L values

`Execution.Symbol` was redundant with the complete contract and was removed;
read `Execution.Contract.Symbol`. `Execution.Side` is now `ExecutionSide`
(`ExecutionSideBought` or `ExecutionSideSold`).

```go
// Before
fmt.Println(execution.Symbol, execution.Side == "BOT")

// After
fmt.Println(execution.Contract.Symbol, execution.Side == ibkr.ExecutionSideBought)
```

Portfolio and P&L values that IBKR can omit are pointers. The three public P&L
spellings now consistently use Go's `PnL` casing.

```go
// Before
fmt.Println(update.UnrealizedPNL, update.RealizedPNL)

// After
if update.UnrealizedPnL != nil {
    fmt.Println(*update.UnrealizedPnL)
}
if update.RealizedPnL != nil {
    fmt.Println(*update.RealizedPnL)
}
```

This applies to `PortfolioUpdate` market/average-cost/P&L fields,
`PnLUpdate`, `PnLSingleUpdate`, and
`CommissionAndFeesReport.RealizedPnL`. Nil means omitted; non-nil zero means
IBKR explicitly reported zero.

Account summary and position subscriptions emit their domain values directly:

```go
// Before
fmt.Println(summary.Value.Value)
fmt.Println(position.Position.Position)

// After
fmt.Println(summary.Value)
fmt.Println(position.Position)
```

## Other source migrations

| Before | After |
|---|---|
| `Buy`, `Sell` | `ActionBuy`, `ActionSell` |
| `CommissionReport` | `CommissionAndFeesReport` |
| `OrderStatusApiCancelled` | `OrderStatusAPICancelled` |
| Flat completed-order projection | `Contract`, `Order`, and `Completion` facets |
| `AccountSummaryRequest.Account` | `Group` plus optional `AccountFilter` |
| Wrapped account-summary and position events | Direct `AccountValue` and `Position` values |
| `WithDefaultResumePolicy` | `WithResumePolicy(ResumeAuto)` on each supported subscription |
| `internal/transport.Dialer` in signatures | Public `ibkr.Dialer` |

`Forex(code) (Contract, error)` is a new v2 constructor, not a renamed v1
function. `HistoricalTickBidAsk.TickAttrib` and
`HistoricalTickLast.TickAttrib` now use typed bitmasks with discoverable accessors
while preserving unknown bits.

`Contract` now contains slices for combo legs, so it and structs embedding it
are no longer comparable. `ExecutionsRequest` and `ScannerSubscriptionRequest`
also contain slice filters. Do not use these request values as map keys; derive
an explicit stable key from the fields your application owns.

The repository replay helpers moved from public `testing/testhost` and
`testing/ibkrlive` packages to `internal/`. External consumers must test through
the public `Dialer` seam or their own captured fixtures; those repository
helpers were never intended as a supported compatibility surface.

Use `OrderStatusAPICancelled` in Go code; the wire value remains the IBKR token
`"ApiCancelled"`.

FA configuration mutation, Reuters fundamental data, `ibkr-probe`, and pre-200 Gateway compatibility were removed. FA reads support Groups and Aliases only. WSH inputs and returned documents must contain valid non-empty JSON. Classic protocol fields cannot contain embedded NUL bytes.

## Gateway errors and added operations

Session events produced by Gateway errors now set `Event.APIError`, preserving the request ID, server time, and advanced-order-reject JSON. Existing event code and message fields remain available for simple notification handling.

`MarketData().RegulatorySnapshot` is distinct from an ordinary quote snapshot and may incur an IBKR fee. After transport admission, loss of its completion evidence returns non-retryable `ErrRegulatorySnapshotUncertain`; reconcile billing before issuing another request. `OpenOrderUpdate.Binding` is populated by `orderBound` only for the client-0 auto-open-orders subscription that owns that callback's scope.

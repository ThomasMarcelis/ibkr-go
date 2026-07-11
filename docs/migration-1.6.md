# Migrating to v1.6

v1.6 is an intentional clean break on the existing module path. It removes
ambiguous ownership and presence semantics rather than carrying compatibility
aliases or fallback behavior.

## Lifecycle

`Client.Close()` is now a command and returns no value. Observe the terminal
session result with `Client.Wait()` when needed.

An `OrderHandle` no longer closes automatically when it receives Filled,
Cancelled, ApiCancelled, or Inactive. IBKR may deliver executions and
commission-and-fees reports after those statuses. Keep consuming events until
the application has the evidence it needs, then call `handle.Close()` and
`handle.Wait()`.

```go
for event := range handle.Events() {
	// Record statuses, executions, and fees.
	if observationComplete(event) {
		handle.Close()
	}
}
if err := handle.Wait(); err != nil {
	// Observation ended because of an API, transport, or consumer error.
}
```

## Executions and historical news

`Orders().Executions` now returns `ExecutionSnapshot`, whose `Executions` and
`CommissionAndFees` slices contain everything observed through IBKR's
execution-details end marker. Use `SubscribeExecutions` when late or revised
fee reports matter; close that subscription explicitly.

`HistoricalNews` now returns `HistoricalNewsResult`. Read articles from
`Items` and use `HasMore` to continue pagination.

## Presence and order ownership

`OpenOrder.LmtPrice` and `OpenOrder.AuxPrice` are `*decimal.Decimal`. Check for
nil before dereferencing; nil means IBKR omitted or unset the value, while a
pointer to zero is an explicit zero.

`LmtPriceOffset` moved from `OrderAdjustment` to `Order`, matching the field's
actual wire location. Move the value directly when constructing or replacing
an order.

What-if `OrderState` now exposes the full Gateway preview block. Commission
fields use the current `CommissionAndFees`, `MinCommissionAndFees`, and
`MaxCommissionAndFees` names.

## Gateway errors and added operations

Session events produced by Gateway errors now set `Event.APIError`. Prefer that
typed value when request ID, server time, or advanced-order-reject JSON matters;
the existing event code and message remain available for simple notification
handling.

`MarketData().RegulatorySnapshot` is distinct from an ordinary quote snapshot
and may incur an IBKR fee. `OpenOrderUpdate.Binding` is populated by
`orderBound` only for the client-0 auto-open-orders subscription that owns that
callback's scope.

## Removed unsupported inputs

FA configuration reads accept only Groups and Aliases. WSH filters and returned
documents must contain valid non-empty JSON. Classic protocol fields cannot
contain embedded NUL bytes.

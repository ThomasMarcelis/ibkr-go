# Upgrading to v2.1

```sh
go get github.com/ThomasMarcelis/ibkr-go/v2@v2.1.0
```

Imports stay the same. Apply the changes below if you use the affected APIs.

## Canceling orders

Replace `client.Orders().Cancel(ctx, observed.OrderID)` with both IDs from the
order you observed:

```go
// observed is an ibkr.OrderStatusUpdate.
err := client.Orders().Cancel(ctx, ibkr.OrderTarget{
    ClientID: observed.ClientID,
    OrderID:  observed.OrderID,
})
```

Use the order's actual client ID. Requests targeting another API client are
rejected locally; orders recovered after restarting with the same client ID
remain cancellable. If reading IDs from `OpenOrder`, check both pointers are
non-nil first. `OrderHandle.Cancel` is unchanged.

## Replacing removed constants and fields

| Removed API | What to use |
|---|---|
| `OrderTypeMarketOnOpen` | `OrderTypeMarket` with `TIF: TIFOPG`. |
| `OrderTypeLimitOnOpen` | `OrderTypeLimit` with `TIF: TIFOPG` and `LmtPrice`. |
| `OrderTypePeggedToPrimary` | `OrderTypeRelative` (`"REL"`); review its price, offset, and exchange requirements. |
| `OrderAuction.Strategy` | No equivalent. If your order depends on this strategy, choose a supported order configuration before submitting. |
| `OrderAuctionDetails.Strategy` | Remove code that reads this field; IBKR cannot supply it on the supported protocol. |
| `FADataProfiles` | No equivalent. Financial-adviser groups and account aliases remain supported. |

For example, a market-on-open order becomes:

```go
order := ibkr.MarketOrder(ibkr.ActionBuy, decimal.NewFromInt(1))
order.TIF = ibkr.TIFOPG
```

Also update code that recognizes returned opening orders: check both `OrderType`
and `TIF`. Sending the removed strings `"MOO"`, `"LOO"`, or `"PEG PRI"` directly
still fails. Other auction price and range fields remain available.

## Checking requests

- Option price and implied-volatility calculations require `Contract.Exchange`,
  even with a contract ID. Use the contract returned by qualification.
- Skip market-rule lookups when `MarketRuleID` is zero: no rule was supplied.
  Lookups now reject zero and negative IDs.
- Historical tick and news time bounds are sent in UTC. Update any tests that
  compare the exact time strings sent to IBKR.

## Handling errors

- An invalid order start time (10315) now closes the order's update stream with
  a rejection. Order previews can now return error 10342 instead of timing out.
- For fee-bearing quote requests, handle `RegulatorySnapshotUncertainError`
  without automatically retrying: IBKR may already have charged for the request.
- With `ReconnectOff`, startup failures now return `ConnectError` with the
  underlying cause. Use typed errors instead of matching error text; malformed
  replies consistently report `ProtocolError`.
- If order tracking ends with `ErrExecutionCorrelationOverflow`, reconcile the
  orders with IBKR; they have not been canceled. Adjust
  `WithOrderExecutionCorrelationLimit` if needed (default: 4,096 execution IDs
  and 4,096 pending fee reports).

Other fixes and optional new APIs are covered in the
[v2.1.0 release notes](https://github.com/ThomasMarcelis/ibkr-go/releases/tag/v2.1.0).

# Migrating to v2.1

v2.1.0 deliberately removes misleading or unsafe API shapes while
the consumer base is small. The module path remains `/v2`; there are no shims.
The [decision ledger](review-decisions-2026-09-06.md) explains every accepted,
deferred, and rejected review recommendation.

## Source changes

| Previous API | v2.1 replacement and consequence |
|---|---|
| `Orders().Cancel(ctx, orderID, opts...)` | Pass `OrderTarget{ClientID: observed.ClientID, OrderID: observed.OrderID}`. A foreign client ID fails locally. Keep `OrderHandle.Cancel(ctx, opts...)` for a bound handle. |
| `OrderTypeMarketOnOpen` (`"MOO"`) | `OrderTypeMarket` with `TIFOPG`. Opening-auction intent belongs in both fields. |
| `OrderTypeLimitOnOpen` (`"LOO"`) | `OrderTypeLimit` with `TIFOPG` and `LmtPrice`. |
| `OrderTypePeggedToPrimary` (`"PEG PRI"`) | The documented relative/primary operation uses `OrderTypeRelative` (`"REL"`). Review its price/offset and venue requirements; this is not an automatic translation to `"PEG PRIM"`. |
| `OrderAuction.Strategy` | Removed. No supported protobuf field represents this legacy instruction. Removing the assignment makes code compile but does **not** reproduce the intended strategy. Choose a supported order composition or do not submit it. Other auction price/range fields remain. |
| `OrderAuctionDetails.Strategy` | Removed from inbound details for the same reason. Remove consumer assumptions about an echo that the supported protocol cannot supply. |
| `FADataProfiles` | Removed: profiles were already rejected by validation. Groups and account aliases remain; there is no equivalent profile request. |

The typed string escape hatch still permits explicit order strings. It does
not make `MOO`, `LOO`, or `PEG PRI` valid instructions: sv225 captures reject
those strings with code 321. Negative replay tests retain the literal strings.
`PEG MKT` remains available; its captured refusal concerns the instrument/venue.

For an OpenOrder echo, check that `Order.ClientID` and `Order.OrderID` are
non-nil before constructing a target from their values.

Replace `client.Orders().Cancel(ctx, observed.OrderID)` with a target that
preserves the observed API client's namespace:

```go
// observed is an ibkr.OrderStatusUpdate.
err := client.Orders().Cancel(ctx, ibkr.OrderTarget{
    ClientID: observed.ClientID,
    OrderID:  observed.OrderID,
})
```

Do not substitute the current client's ID into a foreign order's target.
`OrderTarget` checks the declared namespace; it cannot authenticate a fabricated
identity. A same-client order recovered after restarting the process remains
cancellable without a locally allocated ID or handle. Client ID 0 must use an
actual bound manual-order identity. Signed manual IDs remain unsupported pending
binding/cancellation evidence. `CancelAll` is a separate explicit global action.
Nil from Cancel or Replace means queue admission; reconcile broker status before
reporting cancellation or modification as confirmed.

For opening orders, change outbound construction **and inbound classification**:

```go
order := ibkr.MarketOrder(ibkr.ActionBuy, decimal.NewFromInt(1))
order.TIF = ibkr.TIFOPG
// For a limit-on-open order, use LimitOrder(...) and the same TIF.

isMarketOnOpen := echo.Order.OrderType == ibkr.OrderTypeMarket && echo.Order.TIF == ibkr.TIFOPG
isLimitOnOpen := echo.Order.OrderType == ibkr.OrderTypeLimit && echo.Order.TIF == ibkr.TIFOPG
```

The compositions follow official API 10.50.01 `OrderSamples.cpp`. A `Submitted`
echo outside the opening auction does not prove an auction fill. Keep those
claims separate when adding venue-specific live verification.

## Behavior changes

- Historical tick bounds and historical news bounds serialize in UTC. Equivalent
  instants now have identical wire forms; repeated DST hours remain distinct.
  News retains its `.0` suffix. Tests comparing literal zone spellings must change.
- `Contracts().MarketRule` rejects IDs <= 0 before admission. Zero in
  `ContractExchange.MarketRuleID` means no supplied rule; skip that lookup.
- Option price and implied-volatility requests require `Contract.Exchange`, even
  with a ConID. Use the qualified contract. Contract discovery remains permissive.
- Code 10315 (invalid active start time) ends order observation as a rejection.
  Code 10342 is no longer treated as a cancellation reply without evidence;
  previews can now receive that error instead of waiting for a deadline.
- Code-0 regulatory server failures carry `RegulatorySnapshotUncertainError`
  while preserving the `APIError`. They do not prove the request was unbilled;
  never retry them automatically.
- An incomplete bootstrap with ReconnectOff returns a bootstrap `ConnectError`,
  preserving the cause, instead of bare EOF/ErrClosed. ReconnectAuto still retries
  until Ready or the caller's deadline. Caller cancellation remains non-retryable.
- Missing client identity is no longer interpreted as client ID 0 when routing
  order callbacks. A known matching permanent ID can still establish ownership.
- Canceling historical admission waits is prompt, including the fifteen-second
  identical-request gate. Context cancellation still cannot hide an admitted handle.
- `SessionEvents` closes before `Client.Done`; buffered events remain readable.
- Overflow on one order handle no longer suppresses a healthy open-order observer.
  Execution/fee pairs proven unrelated to local handles no longer consume their
  positive-correlation budget. Unknown fees remain bounded and conservative.
- Session `APIError.Error()` omits an empty operation token. Four inbound parsing
  failures now consistently unwrap to `ProtocolError`; use typed errors, not text.

The earlier candidate also restores the selected market-data type before
reconnect work, retains fees without a 750 ms expiry, preserves a hedge's parent
on replacement, and reports complete local order writes despite a simultaneous
socket error. `WithOrderExecutionCorrelationLimit` bounds retained IDs and pending
fee versions (default 4096 each); an eviction-only exclusion cache uses at most
that many additional IDs. These changes do not establish broker execution.

## New observation APIs

`Subscription.HasSnapshot()` describes capability, even after closure. Keep
`AwaitSnapshot`'s `ErrNoSnapshot` result: treating an absent boundary as completed
would hide a consumer mistake. Pass the stream's lifetime context to Subscribe;
use a separate deadline for waiting on its initial snapshot, while another
single reader drains Events. See [operation control](operation-control.md).

`OrderHandle.Acknowledged()` closes on the first successfully decoded,
attributed open-order, status, or execution callback. It does not consume Events,
does not prove a working or filled order, and does not restore Replace after
recovery is required. An unacknowledged handle's channel stays open even after
Done; wait with a deadline and Done, and continue draining Events. Warnings and
local socket writes do not acknowledge an order. Captured 399/10349 warnings
before an echo disprove the proposed blanket pre-acknowledgement rejection rule.

New constants name code 10091 (partial market-data subscription information) and
10172 (unavailable news article). They do not introduce a universal transient,
entitlement, or retry classification. Raw quote values, zero AuxPrice, account
currency/presence, and the existing context lifetime model remain meaningful.

## ibkr-mobile migration

The hub's v2.0.2 pin predates DAY defaulting and code-10052 rejection (v2.0.3).
Replace literals with `ErrCodeInvalidTimeInForce` where useful; retain the hub's
explicit-TIF policy. Durable `fees_pending` storage survives a hub restart and
is not made redundant by in-memory order correlation. The hub's Ready sequence
may still reapply SetType while rechecking accounts.

The consumer migration check covers these concrete edits in an isolated copy:

1. Change the broker port, live adapter, fake, and sim Cancel signatures to
   `OrderTarget`. Propagate both observed IDs from `orders/modify.go`, preserving
   the existing same-client guard. Make fake/sim record or validate the target.
2. Remove MOO/LOO constants from the type-only map and sim switch. Recognize
   Market/Limit **plus OPG** for observed echoes and journal reconstruction;
   retain opening-order placement semantics and update the stale spec comment.
3. Update direct-cancel tests with targets. Run the hub's build and test suite
   against a temporary local module replacement, and keep the real checkout's
   published dependency pin unchanged until the release is available.

The [release record](release-v2.1.0.md) records the actual consumer verification
and its limits. Upgrade the hub's dependency pin when applying this migration.

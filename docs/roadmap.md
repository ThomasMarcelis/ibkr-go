# Roadmap

## Current state

ibkr-go covers the complete Interactive Brokers TWS/Gateway socket protocol.
All protocol areas below are implemented, tested against live IB Gateway
server_version 200, and frozen into the public API contract.

### Bootstrap and session

Handshake, managed accounts, next valid ID, start API, current time, API
error and system status code routing, session state machine. reqMarketDataType
encoder (msg 59), reqUserInfo (msg 104).

### Account and portfolio

Account summary snapshot and streaming, positions snapshot and streaming.
reqAccountUpdates (msg 6), reqAccountUpdatesMulti (msg 76/77),
reqPositionsMulti (msg 74/75), reqPnL (msg 92/93), reqPnLSingle
(msg 94/95), reqFamilyCodes (msg 80), reqCompletedOrders (msg 99).

### Contract and reference data

Contract details, contract qualification. reqMatchingSymbols (msg 81),
reqMarketRule (msg 91), reqSecDefOptParams (msg 78), reqSmartComponents
(msg 83).

### Market data

Quote snapshot and streaming, real-time bars, historical bars, tick price,
tick size, and market data type. TickGeneric (inbound 45), TickString
(inbound 46), TickReqParams (inbound 81), reqMarketDataType encoder
(outbound 59), cancelHistoricalData encoder (outbound 25),
reqTickByTickData (msg 97/98), and historical bars keepUpToDate flag.

### Historical data extensions

reqHeadTimestamp (msg 87/90), reqHistogramData (msg 88/89),
reqHistoricalTicks (msg 96).

### Option calculations

reqCalcImpliedVolatility (msg 54/56), reqCalcOptionPrice (msg 55/57).

### News

reqNewsProviders (msg 85), reqNewsBulletins (msg 12/13), reqNewsArticle
(msg 84), reqHistoricalNews (msg 86). Three providers are free by default
(BRFG, BRFUPDN, DJNL).

### Scanner

reqScannerParameters (msg 24), reqScannerSubscription (msg 22/23).

### Order and execution observation

Open orders snapshot and streaming (all three scopes), executions snapshot
and streaming, commission reports.

### Order management

PlaceOrder (msg 3), CancelOrder (msg 4), GlobalCancel (msg 58). OrderHandle
tracks lifecycle with Events(), State(), Done(), Wait(), Close(), Cancel(),
and Modify(). Order IDs are auto-allocated from NextValidID. OpenOrder and
OrderStatus messages are dual-dispatched to both per-order handles and the
singleton open-orders observer. OrderHandle survives disconnects (Gap/Resumed)
and auto-closes on terminal status (Filled, Cancelled, Inactive).

### Market depth (Level 2)

SubscribeMarketDepth (msg 10/11, inbound 12/13) — full order book depth as a
keyed subscription. Requires a paid L2 market data subscription.

### Fundamental data

FundamentalData (msg 52/53, inbound 51) — Reuters fundamental data as a keyed
one-shot returning the XML payload. Requires a subscription.

### Exercise options

ExerciseOptions (msg 21) — fire-and-forget option exercise request.

### FA configuration

RequestFA, ReplaceFA (msg 18/19, inbound 16), SoftDollarTiers (msg 79,
inbound 77). FA-only account configuration.

### WSH calendar

WSHMetaData, WSHEventData (msg 100-103, inbound 105/106). Keyed one-shots
returning JSON. Requires WSH subscription.

### Display groups

QueryDisplayGroups (msg 67, inbound 67), SubscribeDisplayGroup (msg 68/70,
inbound 68), UpdateDisplayGroup (msg 69). TWS window integration.

### Cross-cutting

reqMktDepthExchanges (msg 82) — exchange metadata for Level 2 availability.

## Known limitations (v1.1)

The following protocol areas are decoded and routed but still keep a
deliberate grounded boundary instead of exposing the full rare-order wire
surface:

- **OpenOrder (inbound 5):** Simple 169-field orders parse on the fixed
  capture-grounded path. Expanded combo/algo/conditional orders parse on the
  grounded sequential path. Rare delta-neutral, scale, and other ungrounded
  branches still fall back to the safe partial parse.

- **CompletedOrder (inbound 101):** Extracts the public completed-order fields
  only. Advanced order detail sections remain simplified.

- **PlaceOrder (outbound 3):** BAG combo legs, algo parameters, and grounded
  order conditions are encoded. Delta-neutral and scale extensions remain
  deferred.

- **Historical tick attributes:** The tickAttribBidAsk and tickAttribLast
  bitmask fields in historical tick responses (inbound 97, 98) are decoded
  and exposed.

## v1.2 scope

Grounded advanced-order support is landed.
See [`docs/stories/v1.2-variable-length-orders.md`](stories/v1.2-variable-length-orders.md).

- OpenOrder sequential decode for grounded combo, algo, and condition sections.
- BAG (combo) order placement encoding.
- Algorithmic order parameter encoding.
- Grounded order condition encoding plus observation.
- Historical tick attribute decoding.

## v1.3 scope

Full deferred order wire surface.
See [`docs/stories/v1.3-full-order-wire-surface.md`](stories/v1.3-full-order-wire-surface.md).

- Delta-neutral order extensions.
- Scale order extensions.
- Remaining ungrounded OpenOrder branches.
- CompletedOrder full detail extraction.

## Ongoing

- Broader server version testing beyond v200.
- Expanded test coverage and replay scenarios.
- API ergonomics and documentation improvements.

## Public API direction

Root package is `ibkr`. Primary shapes are typed one-shot request methods and
typed subscriptions, with explicit session info and explicit subscription
lifecycle. Subscriptions expose `Events() <-chan T`, `State() <-chan
SubscriptionStateEvent`, `Done() <-chan struct{}`, `Wait() error`, and
`Close() error`. OrderHandle follows the same shape but adds `Cancel()`,
`Modify()`, and `OrderID()`, and its `Close()` detaches without cancelling.

## Not planned

- Client Portal Web API.
- Flex.
- `EWrapper` / `EClient` official-style bridge.

# Roadmap

## v1: full free read-only TWS API surface

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

### Cross-cutting

reqMktDepthExchanges (msg 82) — exchange metadata for Level 2 availability.

## v2: full IBKR API with order management

v2 expands the library to the complete Interactive Brokers API surface,
including all write operations.

### Order management

- Order placement, modification, and cancellation.
- Bracket orders, OCA groups, and conditional orders.
- The typed API and session engine were designed from the start to support
  writes — v1 focused on the read surface to prove the architecture.

### Market depth (Level 2)

- Full order book depth (requires paid L2 subscription).

### Fundamental data

- Reuters fundamental data (requires subscription).

### Additional surfaces

- WSH calendar events (requires WSH subscription).
- FA-only account configuration (reqFA, reqSoftDollarTiers).
- Display groups (TWS window integration).
- Broader server version testing beyond v200.

## Public API direction

Root package is `ibkr`. Primary shapes are typed one-shot request methods and
typed subscriptions, with explicit session info and explicit subscription
lifecycle. Subscriptions expose `Events() <-chan T`, `State() <-chan
SubscriptionStateEvent`, `Done() <-chan struct{}`, `Wait() error`, and
`Close() error`.

## Not planned

- Client Portal Web API.
- Flex.
- `EWrapper` / `EClient` official-style bridge.

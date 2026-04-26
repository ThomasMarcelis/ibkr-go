# SWIG Over the Official IBKR C++ SDK

This document evaluates whether `ibkr-go` should use the official Interactive
Brokers C++ TWS API SDK through SWIG, either as the production protocol backend
or as a conformance/reference path.

This is a research spike, not a production migration. The current branch must
not vendor or redistribute IBKR SDK source. All SDK use is through a local path
set with `IBKR_TWS_API_DIR` after the user has accepted IBKR's TWS API license.

## Current Facts

- Official TWS API Latest is API 10.46, released April 22, 2026, and Stable is
  API 10.37. IBKR recommends TWS or IB Gateway 1037 or higher for
  comprehensive feature support.
- The Mac/Unix package includes Java and POSIX C++ API source and sample code.
  The Latest Mac/Unix package also includes Python. C# and Excel APIs are
  Windows-only.
- IBKR documents C++14 as the minimum supported C++ standard.
- IBKR's TWS API remains documented as a TCP socket/message protocol exposed
  through `EClient`/`EClientSocket`, `EWrapper`, and `EReader`.
- IBKR added protobuf work beginning with 10.35.01. Release notes state:
  - 10.37: protobuf support for place/cancel/global cancel plus order/error
    callbacks.
  - 10.38: protobuf support for completed orders, contract data, market data,
    and market depth.
  - 10.39: protobuf support for historical data, account data, and positions.
  - 10.40: protobuf support in all requests/responses.
  - 10.42: protobuf library versions were updated for official SDKs.
- IBKR recommends static linking for C++ clients after the protobuf migration.
- IBKR's license prohibits redistributing the API code to third parties without
  permission. The repo must not commit SDK source, generated SDK source, or
  derived bulk copies of SDK headers.

## Evaluation Question

The useful question is not "can SWIG expose C++ to Go?" SWIG can do that. The
question is whether the resulting architecture is better than the current direct
Go protocol implementation once the full library scope is included:

- request correlation across keyed, singleton, and order flows,
- typed one-shot methods,
- typed subscriptions,
- order handle lifecycle,
- reconnect and resume semantics,
- deterministic CI without a live Gateway,
- live-derived protocol evidence,
- release drift as IBKR moves more payloads through protobuf.

## Candidate Designs

### Broad SWIG Wrapper

Wrap `EClientSocket`, `EWrapper`, and SDK structs directly.

Benefits:

- Tracks official SDK methods and protobuf-backed object layout closely.
- Less manual C++ code than a hand-written adapter.
- Easier to diff against new SDK releases.

Risks:

- Exposes callback inheritance as the implementation center.
- Requires SWIG directors or another callback bridge from C++ reader threads to
  Go.
- Creates lifetime and thread ownership hazards around Go values referenced by
  C++.
- The generated Go API will not match idiomatic `ibkr-go`; a thick adapter is
  still required.
- Harder to preserve deterministic replay because the official SDK owns
  decode/dispatch.

### Narrow C++ Adapter With SWIG Boundary

Write a small C++ adapter around the official SDK. SWIG wraps that adapter, not
the whole official SDK shape.

Benefits:

- Keeps the Go API idiomatic and owned by this repo.
- Lets C++ own `EWrapper`, `EReader`, protobuf objects, and SDK lifetime.
- Keeps the Go side closer to typed commands and typed event streams.
- Avoids direct Go subclassing of the official callback interface.

Risks:

- Every supported SDK feature needs adapter work.
- The adapter becomes a second protocol surface requiring its own tests.
- Cross-thread event queues, cancellation, reconnect, and shutdown must be
  designed carefully.

### Native Go Plus Generated Protobuf

Keep native socket/session ownership in Go, but consume official `.proto`
contracts directly when IBKR publishes or ships stable schemas.

Benefits:

- Preserves pure Go installation and test harnesses.
- Avoids cgo and C++ runtime requirements.
- Targets the likely long-term protocol contract if protobuf becomes the
  canonical message format.

Risks:

- Requires stable, accessible `.proto` files and clarity on wire framing.
- Does not automatically inherit official SDK behavior outside protobuf
  payloads.
- Still requires live Gateway verification of version-gated behavior.

### Official SDK As Conformance Oracle

Keep production native Go, but use the official C++ or Python SDK in tests and
capture tooling to compare behavior.

Benefits:

- Reduces migration risk.
- Avoids adding C++ to normal user builds.
- Gives a practical answer to release drift without replacing the engine.

Risks:

- Does not remove the need to update the native codec.
- Requires local SDK and Gateway for conformance runs.
- May not catch behavior that only appears in user-specific account or
  entitlement states.

## Build And Packaging Requirements

- Default module builds must remain pure Go and unaffected.
- All SWIG code must require `-tags=ibkr_swig` plus cgo plus Linux for this
  spike.
- Users must provide:
  - Go 1.26 or newer,
  - SWIG 4.3 or newer,
  - GCC/G++ with C++14 support,
  - make,
  - protobuf compiler and C++ protobuf development library,
  - locally downloaded official IBKR TWS API SDK,
  - `IBKR_TWS_API_DIR` pointing at the unpacked SDK root.
- The helper script must discover:
  - `API_VersionNum.txt`,
  - C++ client headers, usually under `source/cppclient/client` or
    `source/CppClient/client`,
  - protobuf include directories such as `protobuf` or `protobufUnix`,
  - `libTwsSocketClient.a`,
  - protobuf linker flags.
- The branch must not add SDK source, generated copies of SDK source, or large
  generated SWIG output.
- If the official SDK package does not ship a usable static library on Linux,
  the next step is documenting the SDK build command rather than committing a
  replacement build system.

## Runtime Architecture Requirements

The production-grade version, if pursued, needs a single ownership model:

- C++ owns official SDK objects: `EClientSocket`, `EReaderSignal`, `EReader`,
  `EWrapper` implementation, protobuf-backed SDK objects, and SDK threading.
- Go owns the public facade, contexts, typed result channels, subscriptions,
  order handles, and lifecycle events.
- The C++ to Go boundary should use copied value events, never borrowed SDK
  pointers whose lifetime continues in C++.
- All strings, vectors, maps, contracts, orders, and nested SDK objects must be
  deep-copied before crossing from C++ into Go.
- Callback delivery must not call arbitrary Go user code from a C++ SDK reader
  thread. C++ should enqueue events into a narrow bridge and Go should drain
  them.
- Shutdown must be explicit: stop reader loop, disconnect socket, drain or mark
  routes failed, release C++ objects, then close Go channels.

## Full Public Surface Mapping

This table maps current `ibkr-go` capability groups to official SDK
requirements. It is intentionally implementation-oriented: a production adapter
must prove each row before replacing the native engine.

| `ibkr-go` area | Official request side | Official callback side | SWIG/adapter requirement |
| --- | --- | --- | --- |
| `DialContext`, `Close`, `Wait`, session snapshot | `eConnect`, `startApi`, `eDisconnect`, `IsConnected`, optional connect options | `nextValidId`, `managedAccounts`, `currentTime`, `error`, SDK-side socket close | Preserve ready-session semantics; expose negotiated server version, managed accounts, next valid ID, connection sequence, and bootstrap errors. |
| Current time | `reqCurrentTime` | `currentTime` | One-shot route keyed by singleton request state, not silence/timeouts. |
| Account summary | `reqAccountSummary`, `cancelAccountSummary` | `accountSummary`, `accountSummaryEnd` | Keyed one-shot and stream forms; account/group/tag data copied into Go structs. |
| Account updates | `reqAccountUpdates` | `updateAccountValue`, `updatePortfolio`, `updateAccountTime`, `accountDownloadEnd` | Singleton subscription semantics; only one active account update stream; cancellation and replacement behavior preserved. |
| Account updates multi | `reqAccountUpdatesMulti`, `cancelAccountUpdatesMulti` | `accountUpdateMulti`, `accountUpdateMultiEnd` | Keyed subscription with model/account fields. |
| Positions | `reqPositions`, `cancelPositions` | `position`, `positionEnd` | Singleton snapshot/stream routing and reconnect behavior. |
| Positions multi | `reqPositionsMulti`, `cancelPositionsMulti` | `positionMulti`, `positionMultiEnd` | Keyed subscription; contract copy conversion. |
| PnL | `reqPnL`, `cancelPnL` | `pnl` | Keyed stream with account/model validation and no invented completion marker. |
| PnL single | `reqPnLSingle`, `cancelPnLSingle` | `pnlSingle` | Keyed stream with conID and unset value handling. |
| Family codes | `reqFamilyCodes` | `familyCodes` | Singleton one-shot; vector copy. |
| Contract details and qualify | `reqContractDetails` | `contractDetails`, `bondContractDetails`, `contractDetailsEnd` | Distinguish generic and bond callback shape if SDK exposes both; preserve ambiguity and no-definition errors. |
| Symbol search | `reqMatchingSymbols` | `symbolSamples` | Keyed one-shot with derivative security type arrays copied. |
| Sec def option params | `reqSecDefOptParams` | `securityDefinitionOptionParameter`, `securityDefinitionOptionParameterEnd` | Keyed one-shot with expirations/strikes/multiplier/exchange sets. |
| Smart components | `reqSmartComponents` | `smartComponents` | Keyed one-shot; map/list conversion. |
| Market rule | `reqMarketRule` | `marketRule` | Keyed one-shot; price increment array copy. |
| Depth exchanges | `reqMktDepthExchanges` | `mktDepthExchanges` | Singleton one-shot; vector copy. |
| Quote snapshot and stream | `reqMktData`, `cancelMktData` | `tickPrice`, `tickSize`, `tickString`, `tickGeneric`, `tickOptionComputation`, `tickSnapshotEnd`, `marketDataType`, `tickReqParams`, `tickEFP`, `tickNews` | Preserve generic tick selection, snapshot completion, tick attributes, option computations, EFP/news callback classification, and delayed/frozen type updates. |
| Market data type | `reqMarketDataType` | `marketDataType` on active streams | Global request with stream-observed effect; support invalid type errors. |
| Real-time bars | `reqRealTimeBars`, `cancelRealTimeBars` | `realtimeBar` | Keyed stream; support TRADES, BID_ASK, MIDPOINT, RTH variants. |
| Historical bars | `reqHistoricalData`, `cancelHistoricalData` | `historicalData`, `historicalDataEnd`, `historicalDataUpdate`, `historicalSchedule` | Keyed one-shot and keep-up stream; preserve schedule variant and timezone behavior. |
| Head timestamp | `reqHeadTimestamp`, `cancelHeadTimestamp` | `headTimestamp` | Keyed one-shot with cancel/error behavior. |
| Histogram | `reqHistogramData`, `cancelHistogramData` | `histogramData` | Keyed one-shot; vector copy. |
| Historical ticks | `reqHistoricalTicks` | `historicalTicks`, `historicalTicksBidAsk`, `historicalTicksLast` | Keyed one-shot; support midpoint, bid/ask, last, ignoreSize, start/end windows, tick attributes. |
| Orders place/modify | `placeOrder` | `openOrder`, embedded order state, `orderStatus`, `execDetails`, `commissionReport`, `error` | Preserve auto-allocated IDs, handle lifecycle, order state copies, combo legs, conditions, advanced attributes, zero/unset values, and protobuf-backed new fields. |
| Cancel order | `cancelOrder` | `orderStatus`, `error` | Preserve direct cancel and `OrderHandle.Cancel` behavior including terminal-state errors. |
| Global cancel | `reqGlobalCancel` | order callbacks and errors | Fire-and-observe semantics with existing order handles updated. |
| Open orders | `reqOpenOrders`, `reqAllOpenOrders`, `reqAutoOpenOrders` | `openOrder`, `openOrderEnd`, `orderStatus`, `orderBound` | Preserve three scopes, client ID 0 behavior, order-bound classification, and singleton observer routing. |
| Completed orders | `reqCompletedOrders` | `completedOrder`, `completedOrdersEnd` | Full detail extraction beyond current simplified fields if SDK exposes all fields. |
| Executions | `reqExecutions` | `execDetails`, `execDetailsEnd`, `commissionReport` | Preserve execution filter layout, commission correlation, quantity decimals, and replayable ordering. |
| Option calculations | `calculateImpliedVolatility`, `cancelCalculateImpliedVolatility`, `calculateOptionPrice`, `cancelCalculateOptionPrice` | `tickOptionComputation`, errors | Keyed one-shot/cancel flow with unset values and option computation fields. |
| Option exercise | `exerciseOptions` | errors and downstream account/order effects | Fire-and-forget semantics plus live-derived success/error evidence. |
| News providers | `reqNewsProviders` | `newsProviders` | Singleton one-shot. |
| News article | `reqNewsArticle` | `newsArticle` | Keyed one-shot; article type/body copied. |
| Historical news | `reqHistoricalNews` | `historicalNews`, `historicalNewsEnd` | Keyed one-shot with provider, timezone, and empty-window behavior. |
| News bulletins | `reqNewsBulletins`, `cancelNewsBulletins` | `updateNewsBulletin` | Singleton subscription and allMessages variant. |
| Scanner parameters | `reqScannerParameters` | `scannerParameters` | Singleton one-shot XML payload. |
| Scanner subscription | `reqScannerSubscription`, `cancelScannerSubscription` | `scannerData`, `scannerDataEnd` | Keyed subscription; scanner options and filter options copied. |
| FA config | `requestFA`, `replaceFA` | `receiveFA`, `replaceFAEnd`, errors | FA account and non-FA error paths; replace read-back/restore if available. |
| Soft dollar tiers | `reqSoftDollarTiers` | `softDollarTiers` | Keyed one-shot. |
| WSH metadata | `reqWshMetaData`, `cancelWshMetaData` | `wshMetaData` | Keyed one-shot JSON payload plus entitlement errors. |
| WSH event data | `reqWshEventData`, `cancelWshEventData` | `wshEventData` | Keyed one-shot JSON payload with filters/date windows/portfolio/watchlist. |
| Display groups | `queryDisplayGroups`, `subscribeToGroupEvents`, `updateDisplayGroup`, `unsubscribeFromGroupEvents` | `displayGroupList`, `displayGroupUpdated` | TWS-only behavior separated from Gateway; subscription handle lifecycle. |
| Errors | all requests | `error` overloads, advanced order reject JSON, SDK exceptions | Preserve `APIError`, `ProtocolError`, validation errors, request ID association, and advanced reject JSON. |
| Reconnect | disconnect/reconnect sequence | socket close, system status errors, bootstrap callbacks | Preserve explicit Gap/Resumed lifecycle, route invalidation, per-subscription resume policy, and order handle survival. |

## Type Conversion Requirements

- Decimal: IBKR SDK decimal quantities must map losslessly into current Go
  decimal handling. Do not silently convert order quantities, position sizes,
  tick sizes, or execution quantities through float64.
- Unset values: SDK unset double/int/string conventions must map to existing
  optional/zero semantics. Large sentinel values must never appear as real user
  data.
- Time: preserve exchange-local, UTC, and server-formatted strings where the
  current API intentionally returns strings; parse only where the current public
  type requires `time.Time`.
- Contracts and orders: conversion must be explicit, field-by-field, and
  version-aware. Do not pass Go-owned memory into C++ structs for async use.
- Vectors/lists: copy SDK vectors into Go slices before delivering events.
- Strings/XML/JSON: copy into Go strings at the boundary and treat payloads as
  opaque unless current public APIs already parse them.
- Errors: map C++ exceptions and SDK error callbacks into Go errors with
  request ID, code, text, and advanced order reject JSON.

## Testing Requirements

Default tests remain the same:

```sh
go test ./...
go build ./...
go vet ./...
```

Tagged compile/link probe:

```sh
export IBKR_TWS_API_DIR="$HOME/IBJts"
eval "$(scripts/check-ibkr-swig-env.sh --print-env)"
go test -tags=ibkr_swig ./internal/ibkrsdkprobe
```

Future live probe:

```sh
IBKR_LIVE=1 \
IBKR_HOST=127.0.0.1 \
IBKR_PORT=4002 \
IBKR_TWS_API_DIR="$HOME/IBJts" \
go test -tags=ibkr_swig -run TestSDKProbeLive ./internal/ibkrsdkprobe
```

Production migration, if ever pursued, needs parity tests for every row in the
mapping table:

- native `ibkr-go` result vs SDK-backed result for the same live Gateway,
- replay/conformance artifacts for deterministic CI,
- release-drift diff against every new official SDK package,
- stress tests around callback ordering, slow consumers, reconnect, and order
  lifecycle.

## Spike Recommendation

Start with a narrow C++ adapter compile/link probe. Do not start by exposing the
entire official SDK through SWIG directors. The broad wrapper is useful as a
source inventory, but it is unlikely to be the right production boundary for an
idiomatic Go library.

The first decision point is whether the official SDK can be integrated without
unacceptable installation, linking, and threading cost. If that fails, the
better direction is native Go plus release-drift tooling and generated protobuf
support where IBKR exposes stable schemas.

## Sources

- IBKR API Software: https://interactivebrokers.github.io/
- IBKR TWS API documentation:
  https://www.interactivebrokers.com/campus/ibkr-api-page/twsapi-doc/
- IBKR protobuf reference:
  https://www.interactivebrokers.com/campus/ibkr-api-page/protobuf-reference/
- IBKR 2025 API production release notes:
  https://www.ibkrguides.com/releasenotes/prod-2025.htm
- IBKR 2026 API production release notes:
  https://www.ibkrguides.com/releasenotes/prod-2026.htm
- SWIG Go documentation: https://www.swig.org/Doc4.3/Go.html


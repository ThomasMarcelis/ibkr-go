# Server version 206 market-data audit

This document freezes the complete `server_version 206` migration boundary.
The evidence is official API 10.48.01 source and protobuf schemas plus local
Gateway sessions forced to negotiate exactly 206. Version 207 is outside this
slice.

## Boundary

Five client requests move from classic fields to protobuf bodies:

| Base ID | Raw ID | Schema | Public operation |
|---:|---:|---|---|
| 1 | 201 | `MarketDataRequest` | quote/snapshot request |
| 2 | 202 | `CancelMarketData` | quote cancellation |
| 10 | 210 | `MarketDepthRequest` | depth subscription |
| 11 | 211 | `CancelMarketDepth` | depth cancellation |
| 59 | 259 | `MarketDataTypeRequest` | live/frozen/delayed selection |

Ten server callbacks use protobuf bodies:

| Base ID | Raw ID | Schema / typed result |
|---:|---:|---|
| 1 | 201 | `TickPrice` |
| 2 | 202 | `TickSize` |
| 12 | 212 | `MarketDepth` |
| 13 | 213 | `MarketDepthL2` |
| 21 | 221 | `TickOptionComputation` |
| 45 | 245 | `TickGeneric` |
| 46 | 246 | `TickString` |
| 57 | 257 | `TickSnapshotEnd` |
| 58 | 258 | `MarketDataType` |
| 81 | 281 | `TickReqParams` |

`TickEFP` base ID 47 remains classic. Tick-by-tick data migrates at 208, news
at 209, and the market-depth-exchanges inventory at 213; none belongs here.

CFD reroute callbacks are the important exception. Base/raw IDs 91
(`rerouteMktDataReq`) and 92 (`rerouteMktDepthReq`) retain classic bodies at
206: request ID, conID, exchange, with no version field. There are no raw
291/292 messages at this boundary.

## Codec and public semantics

- Quote and depth requests use the shared Contract protobuf encoder. Public
  `Contract` owns IncludeExpired, SecurityID, IssuerID, BAG ComboLegs, and
  DeltaNeutral; all are represented at 206. `Contract.conId` is proto3
  optional, but official EClientUtils emits it for zero because
  `Utils::isValidValue(0)` is true; ibkr-go mirrors that request behavior.
  `Contract.Strike` is `*decimal.Decimal`, so nil and an explicitly present
  zero remain distinct through the public conversion boundary.
- Request-family validation runs before route installation. Classic quotes
  carry ComboLegs and DeltaNeutral but not IncludeExpired, SecurityID, or
  IssuerID; classic depth carries none of those five. At 206 both migrated
  requests use the full shared schema, preventing a canonical field from
  disappearing due to a request-specific classic layout.
- Classic quote BAG legs carry only ConID, Ratio, Action, and Exchange.
  Nondefault OpenClose, ShortSaleSlot, DesignatedLocation, or ExemptCode is
  rejected before route installation; the 206 shared leg carries them all.
- DeltaNeutral is accepted only on BAG. The captured exact-200 OPT request was
  a negative code-320 result and is labeled as such; it is not positive
  delta-neutral support evidence. Exact-200 and exact-206 BAG quote request
  vectors are live-frozen. A successful nondefault delta-neutral BAG response
  remains a named live gap.
- False booleans and empty strings follow protobuf absence. Request IDs and
  market-data type retain explicit zero presence inside the codec.
- Tick price, generic, and option doubles use round-trip-safe Go formatting.
  Tick size remains the protocol decimal string, including a real `"0"`.
- Omitted callback scalars follow the API 10.48.01 `EDecoder` defaults rather
  than being treated as malformed. Request IDs default to -1, ordinary numeric
  values to zero, strings to empty, and omitted decimal sizes to unavailable.
  Public standalone size ticks and depth rows expose an unavailable size as
  nil without mutating the accumulated quote.
- Option fields preserve omission as unavailable and retain IBKR's field-
  specific `-1`/`-2` sentinels for the existing public availability mapping.
- `TickReqParams.snapshotPermissions` is presence-aware. Public
  `QuoteParameters.SnapshotPermissions` is nil when omitted and a pointer to
  zero when explicitly present as zero. Last-price and last-size precision are
  optional exact decimals.
- A missing nested `MarketDepthData` message emits no callback. Numeric zeros
  inside a present nested message remain valid position/operation/side/price
  values; omitted size remains unavailable.
- Quote and depth reroutes replace the active request Contract with the
  returned conID/exchange and reissue the same request ID. Quote generic ticks,
  snapshot mode, depth row count, smart-depth mode, and the stored resume
  request are preserved. A second reroute closes the route explicitly instead
  of creating a loop and best-effort cancels the reissued request first. Depth
  cancellation uses the active request's smart-depth mode.

## Official source evidence

The audited SDK was API 10.48.01, archive SHA-256
`0446c403cdfd3a059685c5e11814b32e0b811fdf5e1f68564f8e08b655e49547`.
The stable source/protobuf manifest used for the audit hashes to
`ad2a811e8ecaed96fd56851979c33cde68e569a779feb4be7302980488953927`.

- `EClient.h` lines 286..304 name the five request gates and raw IDs.
- `EClient.cpp` lines 284..644 and 3491..3544 select classic/protobuf request
  encoding and cancellation.
- `EClientUtils.cpp` lines 373..418 and 519..568 build the Contract and market-
  data protobuf messages.
- `EDecoder.h` lines 165 and 193..263 name the inbound migrations.
- `EDecoder.cpp` lines 107..388, 1231..1315, 1908..1952, 3101..3117, and the
  dispatch table at 3998..4620 define decoding, defaulting, nested depth, and
  the classic reroute callbacks.
- `MarketDataRequest.proto`, `CancelMarketData.proto`,
  `MarketDepthRequest.proto`, `CancelMarketDepth.proto`, and
  `MarketDataTypeRequest.proto` define the outbound bodies.
- `TickPrice.proto`, `TickSize.proto`, `MarketDepth.proto`,
  `MarketDepthL2.proto`, `TickOptionComputation.proto`, `TickGeneric.proto`,
  `TickString.proto`, `TickSnapshotEnd.proto`, `MarketDataType.proto`, and
  `TickReqParams.proto` define the inbound bodies.

Production uses `protowire` directly. No SDK source, generated protobuf code,
cgo dependency, or capture artifact is on the default import path.

## Live evidence

The official C++ SDK was used only as a capture oracle against local Gateway
sessions capped at 206. All scenarios were read-only.

`20260710T133515Z-sdk_sv206_market_data_boundary` captured quote snapshots,
option computation, request parameters, market-data type, both depth request
modes, and cancellations:

- `meta.json`: `f8b23384e260132b87a19c23ec1ebc764d0ee449220a4aa0c383db3d8b765957`
- `events.jsonl`: `eea31798e7e59830f5cda9daadcd94223045c8e9ee0e5aa10f48428447505822`

`20260710T134721Z-sdk_sv206_market_data_readonly_retry` captured a dense AAPL
quote and 2,550 L1 depth callbacks, including the exact vectors promoted into
tests and replay:

- `meta.json`: `30eb030686979e33c89e3447b2b92e3145a215d2a9d06ab73a33cd4e3024bfd7`
- `events.jsonl`: `989563f9c4cad108e34058beac205c576a9ebdc0fffe03e421e829bca851e7de`

`20260710T135301Z-sdk_sv206_cfd_reroute_readonly` captured IBM, BMW, and EURUSD
CFD quote/depth reroutes and proved that 91/92 remain classic:

- `meta.json`: `921b20b5f4629246bf0c6713f1524204bf109a4d7a9ad27ad4f8b1d4ad2a5b25`
- `events.jsonl`: `7475841869bc53ceaf779b2672bb1606453e84b1dc0ff66a361510f45470d279`

The local account lacked L2 entitlement, so no raw-213 positive fixture is
invented. `MarketDepthL2.proto` and the official decoder dispatch establish the
shared nested schema; a positive live raw-213 vector remains a named evidence
gap. Raw 212 is live-attested and freezes nested-message behavior.

The v2.0.0 `market_data_sv206_live.txt` replay retained raw protobuf quote
frames and exact quote/market-data-type request bodies with deterministic
request/account substitutions. That pre208 replay was removed from the active
v2.0.1 tree when the supported floor rose to 208; tagged v2.0.0 history retains
it. Current depth coverage is described in the sv208-225 audit.

## Regression gates

- Byte-exact outbound vectors cover all five migrated request families,
  including smart-depth cancellation and complete shared-contract BAG
  composition.
- Byte-exact inbound vectors cover every live-observed migrated callback.
- Boundary tests prove classic ID/body selection at 205 and protobuf selection
  at 206.
- Presence tests distinguish omitted request-parameter values from explicit
  zero; official callback defaults, option sentinels, and missing nested-depth
  behavior are frozen separately.
- Hardcoded live raw 91/92 frames cover the classic registry and engine
  dispatch path.
- Public route tests freeze transparent rerouting, stored resume replacement,
  loop prevention, and smart-depth cancellation.
- Field-support tests freeze the classic/protobuf request matrix so unsupported
  IncludeExpired, SecurityID, ComboLegs, DeltaNeutral, or IssuerID values fail
  before a route is installed.
- The public exact-206 replay proves negotiation, request encoding, typed quote
  updates, parameter presence, precision, and cancellation without a Gateway.
- `TestLiveServer206MarketDataBoundary` is the opt-in read-only public live
  probe.

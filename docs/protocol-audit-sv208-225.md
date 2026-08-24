# Protocol Audit: Server Versions 208-225

This audit records the API 10.48 protocol train beginning at the supported
floor. The production client negotiates exactly `server_version` 208..225. The runtime is
still pure Go; official SDK 10.48.01 was used only to produce exact live
request/callback evidence against the local Gateway.

## Migration boundaries

| Version | Boundary | Public behavior |
|---------|----------|-----------------|
| 208 | Historical-data protobuf family | Bars, real-time bars, head timestamp, histogram, historical ticks, tick-by-tick, and their cancellations keep the existing typed APIs. |
| 209 | News protobuf family | Bulletins, articles, providers, historical news, and WSH requests keep the existing typed APIs. |
| 210 | Scanner and PnL protobuf families | Scanner parameters/results and account/position PnL keep the existing typed APIs. |
| 211 | Remaining request family 1 | FA reads, option exercise, and option calculations migrate without changing the public operations. |
| 212 | Remaining request family 2 | Option-definition, soft-dollar, family-code, symbol, smart-component, market-rule, and user-info messages migrate. |
| 213 | Remaining request family 3 | Bootstrap/control, ID/time, display-group, and depth-exchange messages migrate. |
| 214 | UTC date-time format | Inbound `Z` timestamps are accepted. Outbound suffix behavior remains unresolved, so requests retain the existing format. |
| 215 | Broker-side one-shot cancellation | Context cancellation sends the new contract-details or historical-ticks cancellation request when the request still owns its route. |
| 216 | Additional order parameters 1 | `Deactivate`, `PostOnly`, `AllowPreOpen`, and `IgnoreOpenAuction`. |
| 217 | Additional order parameters 2 | Presence-aware `RouteMarketableToBBO`, `SeekPriceImprovement`, and `WhatIfType`. |
| 218 | Attached preset orders | The wire shape is implemented and regression-frozen internally. It is not public pending a native paper-order capture of the full lifecycle. |
| 219 | TWS configuration | `TWS().Config` returns the presence-aware configuration snapshot. Configuration mutation is intentionally not exposed. |
| 220 | Market-data volume units | Quote volume remains share-denominated; the exact boundary is frozen through a live protobuf callback. |
| 221 | Configuration updates | Operator configuration mutation remains outside the library API. |
| 222 | Fractional last size | Last-size values continue through the decimal pipeline without integer coercion. |
| 223 | Hedge maximum size | `Order.Hedge.MaxSize` is encoded as order field 144. |
| 224 | Security-definition precision | Contract details and quote parameters preserve optional last-price and last-size precision. |
| 225 | Odd-lot bid/ask | Generic tick 787 exposes typed odd-lot prices, sizes, and exchanges on `Quote` and `QuoteUpdate`. |

Source-defined outbound fields with an implemented minimum fail before send on
an older negotiated version. The sv214 outbound formatting boundary is the
explicit unresolved exception. The lifecycle test accepts exactly 208..225
and rejects both neighboring values.

## Evidence ledger

| Evidence | SHA-256 | What it proves |
|----------|---------|----------------|
| Native public sv208 historical bars | `fe2fa8c99197756bb7f3753a3c49ab51191efff2230c386c5ca9b8a9ca838849` | Exact request, three positive bars, end marker, and public result. |
| Native public sv208 classic boundary families | `25aa15fdaeff68a48689bc70e68ddcc519783427a0fd835172c9aaad55246b08` | Exact classic scanner, news, options/reference, time, display-group-list, and control callbacks. |
| Native public sv208 PnL | `dccee2ab425fca9c707dd682c7d9ffc0dafc1dfd1ce093050e425036a8fcb7d2` | Exact classic account and single-position PnL callbacks. |
| Native public sv208 TickNews | `55b765c665166a3c54f791f316211d51ae45fb876e877a028863ca66a5a24dcd` | Exact classic contract-specific news callback through delayed market data. |
| Native public sv208 UserInfo | `672370162ad17e46cf045647775d1d6bc4480353b2f044392c43431a88717bd5` | Exact classic request shape and callback at the supported floor. |
| Native public sv208 DisplayGroupUpdated | `8f56c47cc04a67aead4e491d5549d14db323a00fa8e6ad26d0bb1b71e01cd78e` | Exact classic display-group update callback. |
| Native public sv210 option calculations | `510dedb3be94ed96c3201807cc7d91e0fcd9756e9f98444efa0dbb66faea2289` | Exact last-classic option-price and implied-volatility requests plus positive results. |
| Native public sv211 option calculations | `59056822b51af4a00caa28afb922b4f79ee7014668591392e8f4fae229ea7222` | Public option-price availability `247` and implied-volatility availability `133`. |
| Native sv214 bootstrap | `75ecf971832226bf99ef30d0db75b8a9e3bb7831041e6b01adfede7ae6f79468` | The local Gateway negotiates the post-213 train. |
| Native sv214 historical request | `9b462902a853e689979ed89b77f8ac5927518d608c6da538d406ae877c2fcd70` | Negative boundary evidence only: this request omitted `endDateTime`, so it does not attest the UTC `Z` suffix. |
| SDK sv215 one-shot cancels | `b3515b46284970f338db6ede7b2864f4d63449027f9f48ff203a67f4fd34d019` | Exact contract-details and historical-ticks cancel protobuf bodies. |
| SDK sv216 order parameters | `dd8e34848c2de885947f2eec9c77b4901d428c5719dc85bcaea4e9417212b7cc` | Exact fields 138-141. |
| SDK sv217 order parameters | `302d403d32b43f1107b8e15fa62ac6fd318658d61bc10496d8331907f6e10dc2` | Exact fields 120, 142, and 143. |
| SDK sv218 attached orders | `34ebb09db5b427aed962859ba2f5c137fcd328987eb1c8aa72f18688960fcf62` | Exact nested stop-loss and take-profit preset metadata. |
| SDK sv219 configuration | `928ada9da43be6e71f18c31f9d3f69a07e6decadfbef33ef0e6cc7a8eb01253b` | Exact configuration response used by the decoder test. |
| Native public sv219 configuration | `3b5a3dee08cb7cae0be4da4eb409974a37dd71ef37ca2d2bc202e06902b74082` | `TWS().Config` returned typed messages, API settings, order settings, and trusted IPs. |
| SDK sv223 hedge maximum | `205f25d37f53daf6dcc0a7b2f93a58215dcf7e1091e5a3fbafb459f018764061` | Exact outbound field 144. The request then received the real missing-parent error, so this is wire proof rather than accepted-order proof. |
| Native public sv225 odd-lot probe | `70500b2228dc29e81e8823fa6a626bf51597317a83da257e2e3e1e520b7b52a3` | Generic tick 787 is accepted and the ordinary delayed stream remains healthy; no odd-lot callback was available with this entitlement. |
| SDK sv225 odd-lot probe | `85d0dba58ba9d80c029fac5b658d01ac48128d0513228c07f555dbac6fbff2b0` | The same entitlement boundary through the official client. |
| Native public sv225 CFD reroute | `ca8fbdf11d260066fb7cd1c3d60e6e44808a54bf6a8fc678f3597bd71a666f1c` | Initial CFD request, protobuf reroute, conID request, delayed-data notice, then positive high/low/volume/close callbacks. |
| Native public sv225 IncludeOvernight lifecycle | `f3585ff96e9a0a936a559e52e9d69e860e6182e769418d987314ce9419c90e9c` | True placement/echo, code-462 false replacement rejection with true retained, fresh false canonicalized to absence with `TIF=DAY`, and terminal cleanup. SDK 10.48.01 reproduced the rejection. |

The sv208-213 protobuf schemas have focused exact-vector and public routing
tests in `internal/codec`. The sv215-225 rows above additionally retain live
capture hashes in those tests where a byte-exact request or callback exists.
The exact-version matrix itself proves only handshake and `CurrentTime`;
20 supported decoder/layout pairs remain explicitly unattested.

## Explicit evidence boundaries

- The local market-data entitlement returned ordinary delayed AAPL ticks but
  no tick 105-110 odd-lot values. Odd-lot decoding and public projection are
  implemented and source-law tested, but positive live values remain pending.
- `tickEFP` (inbound 47) and `deltaNeutralValidation` (inbound 56) now have
  typed decoders and public quote-update variants. Their exact official layouts
  are frozen, while positive live callbacks still require an entitled active
  single-stock future or successful delta-neutral BAG request.
- Attached preset-order metadata has exact SDK wire evidence and remains
  internal. A public facade still requires its own native paper-order lifecycle
  capture.
- Inbound sv214 `Z` forms are parser-tested. The retained native request omitted
  `endDateTime`, so it does not establish outbound formatting; the client does
  not append `Z` until exact source/live evidence resolves that behavior.
- The sv225 CFD capture proves end-to-end protobuf market-data rerouting and
  positive delayed quote data after the conID request. The protobuf
  depth-reroute callback remains without positive live evidence.
- `updateConfig` is not exposed. Reading configuration helps diagnose session
  behavior; changing operator-owned TWS/Gateway configuration is outside the
  client library's responsibility.

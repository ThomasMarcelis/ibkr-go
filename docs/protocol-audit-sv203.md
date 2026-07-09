# Server version 203 order-protobuf audit

This document freezes the complete `server_version 203` migration boundary.
The source baseline is official TWS API 10.48.01 plus a guarded local paper
Gateway session negotiated at exactly 203. Version 204 is outside this slice.

## Boundary inventory

Official `EClient::GetServerVersionForMessage` and
`MIN_SERVER_VER_PROTOBUF_PLACE_ORDER` identify exactly three newly migrated
client messages. The Gateway capture establishes the newly migrated paired
open-order/status messages and observes the existing protobuf error family:

| Direction | Base ID | Raw ID at 203 | Schema / behavior |
| --- | ---: | ---: | --- |
| client | 3 | 203 | `PlaceOrderRequest` |
| client | 4 | 204 | `CancelOrderRequest` |
| client | 58 | 258 | `GlobalCancelRequest` |
| server | 3 | 203 | `OrderStatus` |
| server | 4 | 204 | `ErrorMessage` for order warnings/cancellation notices; protobuf since 201, observed here rather than newly migrated at 203 |
| server | 5 | 205 | `OpenOrder` |
| server | 53 | 253 | empty `OpenOrdersEnd`; observed after classic `reqAllOpenOrders` |

No other outbound message migrates at 203. Open-order snapshot requests remain
classic, but the Gateway already returns their empty protobuf terminator at
203. Completed-order protobuf is the next official boundary at 204 and is
deliberately not advertised here.

## Wire invariants

- The negotiated 201+ envelope starts with one four-byte big-endian raw ID.
  A protobuf raw ID is the base ID plus 200, exactly once.
- `PlaceOrderRequest` contains `orderId` (field 1), an emitted `Contract`
  message (2), an emitted `Order` message (3), and an emitted empty
  `AttachedOrders` message (4). The official client emits the nested message
  even when it has no fields.
- Contract combo legs and per-leg prices are merged into repeated
  `Contract.comboLegs`; an extra price with no matching leg fails before send.
- `Order.totalQuantity` remains a decimal string. Protobuf integer and double
  fields are parsed and range-checked; malformed, overflowing, NaN, and
  infinite values fail before send. Protobuf maps use deterministic key order
  and official duplicate-key last-wins behavior.
- `Order.softDollarTier` is emitted even when empty. Unsupported internal
  shapes such as an opaque classic order-options string or a delta-neutral
  contract with no typed public representation fail before send.
- `CancelOrderRequest` emits `orderId` plus a present `OrderCancel`, and
  `GlobalCancelRequest` emits a present `OrderCancel`, including when that
  nested message is empty.
- Inbound order-status decimal quantities stay strings until the public
  decimal conversion boundary. Open-order contract, order, condition, routing,
  and order-state messages are decoded directly with `protowire`; generated
  protobuf artifacts are not part of the repository or production import
  path.

## Evidence

Official source files used for the audit:

- `API_VersionNum.txt`: `fcc9663c0c0405e9a63c4b3177a47e8aa99d95f9123e98a2b6b5fea4ff4e95ac`
- `EClient.h`: `3e86ce64b6262b283a5d39d68ecc507732a5c014a0b9e1d8f08b5613a17ac42e`
- `EDecoder.h`: `75c3d2920ce47db156aed12a6c17e1de08c8f33755ef69052532169295b8df6a`
- `EClientUtils.cpp`: `f214567871040de6c5cb157de9f7f5842e515f85f8df22fde8c20925b556207e`
- `Contract.proto`: `96b445b6939459aecda044835099be045a8adfd69d76ca2147a943b9dbee82c9`
- `PlaceOrderRequest.proto`: `94dbdecbe9b6e6b3e95732d9ac1b1371a17a0e527e77cb28604efed6caa9099b`
- `CancelOrderRequest.proto`: `7176d0ab55e83ecb74e424cf4f6782e5cb59b9f09a52e36ea44c505232630f4d`
- `GlobalCancelRequest.proto`: `877a242144cdcd664ea475fe7a08958e75d091983850ccb9d65e93978b4c0060`
- `OpenOrder.proto`: `ad1838f27f271c0929a9e6b3c4889cf06bd094d7863c1fa757d71ee5525941e3`
- `OpenOrdersEnd.proto`: `dff8c69d2b1329941f7b6176f87589b7d34b79aa9d3184866bde33e790cd2e73`
- `OrderStatus.proto`: `0be18bcf2ad2321512383544b06a66786a0b1bcf4a5aac60cfacf45b6c6eeb85`
- `Order.proto`: `3a963d252987b4a1450d6ded1901f46fdbad16039b6f717dc8beb597e78695c8`
- `OrderState.proto`: `63b79d7d3de82591a61a1f0f9699edae93e8be4ef74a430fffff2ac2cf4d247e`
- `OrderCondition.proto`: `a0a1f6bba0620e59522d8e2d62a90bfd434ea8bbcdd511178025769723c4c76f`
- `OrderCancel.proto`: `a598b16f54dadbc6a490359df140ecb792169a058bab1cfa8d0b3a5d6f1c20c3`
- `AttachedOrders.proto`: `ce6730d9e6a10816f751ebd04a7226c4fc3f493a43140a8ef4ca103b3fccb382`
- `ComboLeg.proto`: `9d80a4a8ab9ab8b1b6e1eef8b79ef0cc0f8959f1f0a630d4ddfb7a82431549aa`
- `SoftDollarTier.proto`: `9d6ccdbb36074ad163ab546323881fbc9d48ef538b5d8c07a8f146f25f6927f0`
- `DeltaNeutralContract.proto`: `701fbd81041966073163de34878f7faa13dda5eca27553fb2343feb13f4219f0`
- `OrderAllocation.proto`: `571f8191500e64cdabd5166cd92c52f98cef4cc3164a4e76f74eec6a44bf7c60`
- `ErrorMessage.proto`: `6ae1e1d78b69b75603310b40194074a6706ebc4055aa0fbf904360ef2d316b4f`

The private capture
`20260709T233523Z-protobuf_sv203_order_cancel_aapl` placed one AAPL share at a
non-marketable $50 limit, observed the open/status callbacks, cancelled it,
issued global cancel, and completed a current-time round trip before teardown.
The account ended flat and no order remained working.

- `events.jsonl`: `26d77d633ee488dff8a6afd6c7d0ffd35c72b686e9378fed92e4019b02339a3a`
- `raw.txt`: `665cb95403e5bc18a59b82a4c6628f3b05da67447de2b87df5221e455b91484a`
- `replay/frames.jsonl`: `47aafceab7ad8afe33813cb96886a9cc7fce2af161097d9889bc9de8d8e6a295`

The committed transcript is sanitized and retains the exact protobuf field
presence. It replaces the account, client, permanent, and submitter identifiers
and omits farm-status noise and the time-dependent warning text. Unit tests
also compare the minimal place/cancel/global-cancel bytes with the official
10.48.01 schemas and freeze malformed-input failure behavior.

The separate read-only capture
`20260709T234704Z-protobuf_sv203_open_orders_empty` proves the mixed request/end
boundary: classic `reqAllOpenOrders` base ID 16 followed by empty protobuf raw
ID 253. Its `events.jsonl` hash is
`963a80c89cc19b5741e1a662e9437acad1d158bbfe4e2c5dbbccfd8ee75cb378`
and `replay/frames.jsonl` hash is
`d55c70849443ebcc6b17f6e14996ad5ed68ca11d6197d2fc3b455ba3317b4656`.

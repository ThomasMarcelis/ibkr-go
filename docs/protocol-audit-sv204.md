# Server version 204 completed-order audit

This document freezes the complete `server_version 204` migration boundary.
The evidence is API 10.48.01 source plus a local paper Gateway session forced
to negotiate exactly 204. Version 205 is outside this slice.

## Boundary

At 204, four order-query requests move from classic fields to a protobuf body,
and the completed-order response pair moves to protobuf. Raw IDs add the
protocol discriminator 200 to the base ID.

| Direction | Base ID | Raw ID at 204 | Schema / behavior |
|---|---:|---:|---|
| client | 5 | 205 | empty `OpenOrdersRequest` (client scope) |
| client | 15 | 215 | `AutoOpenOrdersRequest`; field 1 is present only when binding (`true`) |
| client | 16 | 216 | empty `AllOpenOrdersRequest` |
| client | 99 | 299 | `CompletedOrdersRequest`; field 1 is present only for `apiOnly=true` |
| server | 101 | 301 | `CompletedOrder` containing emitted Contract, Order, and OrderState messages |
| server | 102 | 302 | empty `CompletedOrdersEnd` |

`OpenOrder` raw ID 205 and `OpenOrdersEnd` raw ID 253 were already protobuf at
203. They are not newly migrated at 204. Order status, execution, order error,
place, cancel, and global-cancel behavior is likewise unchanged at this gate.

The official optional-scalar encoder omits `false`; it does not emit a tag with
a zero value. The exact request payloads are therefore:

| Operation | Payload hex | Length-framed SHA-256 |
|---|---|---|
| client open orders | `000000cd` | `9c28d72b295e6e3da3bed01a12fab93056830eb671a8d499bd49c2962f700c43` |
| all open orders | `000000d8` | `518b89b3b2cb7e3a65c5c5a388374777eea65c4e7a11fd4c98440215b69b6153` |
| bind auto open orders | `000000d70801` | `e2997ddea04caa27722225df930311b373511d56067fdc76580c53cc8aa898f5` |
| unbind auto open orders | `000000d7` | `df798bf433a454c4e5e210213ad52d9f1d85a11481704004f559c8b974d1e4ba` |
| all completed orders | `0000012b` | `d9358017f4903ec9c5d5dae93f7d4a00a140ddfcf77ec98cd40e54077b220c86` |
| API-only completed orders | `0000012b0801` | `823715256640eec0c5882f7346e862d7b6e76d7adfae4304ec0cfe2414113c8e` |
| completed orders end | `0000012e` | `f9810dc3afeed556ca081c3172293cfe996fc50624f58abb713d329c21a8ec75` |

The exact bytes are frozen in `codec_orders_proto204_test.go`.

## Official source evidence

The audited SDK was API 10.48.01, archive SHA-256
`fcc9663c0c0405e9a63c4b3177a47e8aa99d95f9123e98a2b6b5fea4ff4e95ac`.

- `EClient.h` maps the four outbound base IDs to the completed-order gate
  (`3e86ce64b6262b283a5d39d68ecc507732a5c014a0b9e1d8f08b5613a17ac42e`).
- `EClient.cpp` owns the four request encoders
  (`3c23aaca9afae0f6155ec8558b9e5430f6062e031577bd86d4de2e0925158f81`).
- `EClientUtils.cpp` establishes optional-bool presence
  (`f214567871040de6c5cb157de9f7f5842e515f85f8df22fde8c20925b556207e`).
- `EDecoder.h` names 204 as the completed-order protobuf gate
  (`75c3d2920ce47db156aed12a6c17e1de08c8f33755ef69052532169295b8df6a`).
- `EDecoder.cpp` dispatches and decodes completed order/end
  (`74aa6ea9790d30c9d1abacddba190871ac6b692141ae01a59323f6b4b54706e6`).
- `CompletedOrder.proto` requires the observed Contract, Order, and OrderState
  nesting (`2e23b6e9b91d94d2413f2e46f1c079116199dc932859843bc214c1b1dfa97581`).
- `Order.proto` defines presence-aware client, order, permanent, and parent
  identities (`3a963d252987b4a1450d6ded1901f46fdbad16039b6f717dc8beb597e78695c8`).
- `OrderState.proto` carries completed time/status and commission-and-fees
  amount/currency (`63b79d7d3de82591a61a1f0f9699edae93e8be4ef74a430fffff2ac2cf4d247e`).

Production uses only `protowire`. No generated SDK code, SDK artifact, cgo, or
schema-specific generated runtime is on the import path.

## Live evidence

The official C++ SDK was run only as a capture oracle against the local paper
Gateway at `127.0.0.1:4002`, with the handshake capped at 204.

`20260709T234535Z-sdk_sv204_completed_order_boundary` exercised client/all
open-order queries, bind/unbind auto-open-orders, and `apiOnly=false` completed
orders. Its hashes are:

- `events.jsonl`: `9be0a4fd1b27056803baf4dbfb16a0ec2daa02d7f43bdec06b5ceee4b6f817a5`
- raw capture: `b6d54d9c548e89a945ee1f82f66a7b785eb756d3c98645aff3b9b6ffd95a9759`
- replay frames: `d3c6b0758f3bb99bf5df422a8f01f3cde948746271125e1beada6bd735ed3fd3`

`20260709T234955Z-sdk_sv204_completed_orders_api_only` separately exercised
the present-true `apiOnly` request:

- `events.jsonl`: `36acaedf6b155ce6f1f0d2433d99864e1248cd1b16bc56701f4300c46def8a90`
- raw capture: `d31c9a72beb84fe81c17040408450d27584cdfe91d420455d73f296216c99455`
- replay frames: `532b84ff6cdc5f75e2f112a7975551b37258661a176b9b83d86e4ab6554950c9`

The promoted replay keeps two exact live frames: a cancelled AAPL order and a
filled AAPL order whose OrderState carries commission-and-fees `1.006695 USD`.
The original normalized payload hashes are
`f31d3e02418ee9ed73644a299ed77b81ec44f49e13e72c9ecad260c63026e35b`
and `6a825910d09262768923e87c864945ea42c7add93416adac7dd66196711379fe`.
Account, client, permanent, and submitter identities are sanitized in the
checked-in fixture; duplicate historical orders and farm notices are trimmed.

## Public semantics and invariants

- `CompletedOrderDetails.OrderID`, `ClientID`, and `ParentID` are pointers.
  Classic completed-order frames omit them and produce nil; protobuf explicit
  zero remains a non-nil pointer to zero.
- Permanent ID keeps the same presence-aware behavior.
- `CompletedOrderCompletion.CommissionAndFees` is nil when OrderState omits the
  amount and points to an exact decimal when present. Currency is preserved
  independently.
- The Contract, Order, and OrderState nested messages must all be present, as
  emitted by the live Gateway. Missing nested messages fail decoding.
- Unknown protobuf fields are skipped. Observed schema defaults with no useful
  completed-order meaning (`adjustedOrderType=None`, zero adjustable trailing
  unit, zero price-management mode, and zero seek-price-improvement mode) do
  not create noisy public fields.
- The guarded exact-204 live test is read-only: it checks completed orders,
  verifies there are no working orders, and verifies the AAPL position is flat.

Version 205 remains fail-closed until its contract-data migration receives the
same source audit, exact live capture, deterministic replay, and public test.

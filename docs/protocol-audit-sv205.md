# Server version 205 contract-data audit

This document freezes the complete `server_version 205` migration boundary.
The evidence is API 10.48.01 source plus local Gateway sessions forced to
negotiate exactly 205. Version 206 is outside this slice.

## Boundary

At 205, the contract-details request and its three response messages move from
classic fields to protobuf bodies. Raw IDs add the protocol discriminator 200
to the base ID.

| Direction | Base ID | Raw ID at 205 | Schema / behavior |
|---|---:|---:|---|
| client | 9 | 209 | `ContractDataRequest`: request ID plus emitted Contract |
| server | 10 | 210 | `ContractData`: request ID, Contract, and ContractDetails |
| server | 18 | 218 | the same `ContractData` schema projected as bond details |
| server | 52 | 252 | `ContractDataEnd`: request ID |

Market-data request/cancel migration starts at 206. Contract-data cancellation
is a separate operation introduced at 215. Neither belongs to this boundary.

The exact captured requests are:

| Selector | Payload hex |
|---|---|
| AAPL stock conID 265598 | `000000d10895a001120b08fe9a104205534d415254` |
| Apple bond conID 127128131 | `000000d10896a001120c08c3a4cf3c4205534d415254` |
| fund conID 57041934 | `000000d10897a001120f088ec8991b420846554e4453455256` |
| option conID 728937835 | `000000d10898a001120d08ebeacadb024205534d415254` |
| Apple issuer `e1432232` | `000000d10896a001120d08008201086531343332323332` |

The request vectors, four response types, ineligibility reasons, and end
markers are frozen in `codec_contracts_proto205_test.go`. The public replay in
`contract_details_sv205_live.txt` retains one stock and one bond round trip.

## Official source evidence

The audited SDK was API 10.48.01, archive SHA-256
`0446c403cdfd3a059685c5e11814b32e0b811fdf5e1f68564f8e08b655e49547`.

- `EClient.h` maps request base ID 9 to the 205 gate
  (`3e86ce64b6262b283a5d39d68ecc507732a5c014a0b9e1d8f08b5613a17ac42e`).
- `EClient.cpp` owns classic/protobuf request selection and transmission
  (`3c23aaca9afae0f6155ec8558b9e5430f6062e031577bd86d4de2e0925158f81`).
- `EClientUtils.cpp` maps request IDs and Contract fields into protobuf
  presence (`f214567871040de6c5cb157de9f7f5842e515f85f8df22fde8c20925b556207e`).
- `EDecoder.h` names 205 and the three inbound base IDs
  (`75c3d2920ce47db156aed12a6c17e1de08c8f33755ef69052532169295b8df6a`).
- `EDecoder.cpp` dispatches regular, bond, and end protobuf messages
  (`74aa6ea9790d30c9d1abacddba190871ac6b692141ae01a59323f6b4b54706e6`).
- `EDecoderUtils.cpp` maps the shared ContractDetails schema into regular and
  bond callbacks (`defc7b0b86da1a7fb2c3332bb13f530b0513cb90630c3b974890aaca105d360c`).
- `ContractDataRequest.proto` defines the request nesting
  (`e033e76d8f89b9d8efdb2be661c8c246a15d5e1bbca66f6312a5fe5fff92326c`).
- `ContractData.proto` defines the shared response envelope
  (`b50e8eda31841e187ec4d4500d24a071890682e56b17c51fd84bce6297babfa2`).
- `ContractDataEnd.proto` defines the request-correlated terminator
  (`c78298f3c4432ad2f85cd60ecaf0df8f951f2dcf74c468c0babf764c3f7d88b8`).
- `Contract.proto` defines the shared 21-field contract schema
  (`96b445b6939459aecda044835099be045a8adfd69d76ca2147a943b9dbee82c9`).
- `ContractDetails.proto` defines all 64 detail fields
  (`9bd4b6395ed75ebac05e676721d591a5671e597609744f7179aec5907b049151`).
- `IneligibilityReason.proto` defines the repeated reason pair
  (`2657adc2fde2ee2de13d8b50fffd45e98b023edc44e933a855cf4e60ffec4ffd`).

Production uses only `protowire`. No generated SDK code, SDK artifact, cgo, or
schema-specific generated runtime is on the import path.

The capture-oracle build patched only `EDecoder.h`'s advertised
`MAX_CLIENT_VER` to force exact negotiation at 205; that harness-local file
hashes to `1cb75811ea2b4dae4e091076cac8b883d5b902c95b0b7c0dcb14f6090d4a4f4c`.
It is not the official archive source hash.

## Live evidence

The official C++ SDK was used only as a capture oracle against local Gateway
sessions with the handshake capped at 205.

`20260710T000715Z-sdk_sv205_contract_data_compact` retained one exact AAPL
stock and one exact Apple bond lookup:

- `events.jsonl`: `3d1e3c303295a88e1923fa8127a991d83ce9dc86e742b8423eff047ce8ce6a49`
- raw capture: `000f45edc97ece34e0040fb56c83da43e086a41c3b0ea65971d951a1308c54fb`
- replay frames: `745fe188b46223efa532f31fca094c9b62e4f1576596172fc9401ccb356cd483`

`20260710T000921Z-sdk_sv205_contract_data_type_matrix` separately captured
stock, bond, fund, and option responses; its normalized replay hash is
`a55614fb9b32d35b8d0e3ba6479d5a111fe7db8bed83429527a20217a44c8ef5`.
It proves explicit `minAlgoSize=0` for all four types, stock price/size
precision `0.000001`, the fund block, and both option date fields.

`20260709T235933Z-sdk_sv205_contract_data_boundary` captured the 58-row Apple
issuer response; its normalized replay hash is
`f9dc02ad6fd6b480c26be3e4628efbfa26821602f16e97f974a8d9ce61cd6d8c`.
One real bond carried three ineligibility reasons; that response payload hashes
to `e33da4e82aeb2e4900d47bb4d43be7d9f11f41c036d902a05167e684ab0ef572`.

The checked-in replay trims farm notices and the post-bond advisory. Account
and request identifiers are deterministic substitutions inside otherwise
captured protobuf frames.

## Public semantics and invariants

- Public `Contract` is the single owner of selection and composition:
  include-expired, external security ID, issuer ID, BAG legs, and the
  delta-neutral underlier. Response-only combo descriptions remain outside
  `Contract`.
- One internal Contract codec is shared by executions, open orders, completed
  orders, order requests, and contract data.
- The sv205 contract-details request uses all public fields represented by the
  21-field shared schema. `Contract.conId` is proto3 optional, but the official
  EClientUtils request encoder sets it for zero because
  `Utils::isValidValue(0)` is true; ibkr-go mirrors that explicit tag. At
  classic sv204 and below, contract details accept only IncludeExpired,
  SecurityID, and IssuerID from the five extended canonical fields; request
  validation rejects BAG legs or DeltaNeutral before a route exists.
- Strike, delta-neutral decimals, combo-leg numeric fields, exempt codes, and
  order combo prices decode strictly. A malformed response closes only its
  request route rather than degrading a field or terminating the session.
- Regular and bond responses require the request ID, Contract, and
  ContractDetails messages. The end message requires its request ID.
- Size and precision strings become optional exact decimals. An explicitly
  present zero is a non-nil decimal pointer; an omitted field remains nil.
- Fund fields create the public fund facet only when a fund field is present.
  Missing aggregate group retains the established max-int sentinel internally
  and becomes nil publicly.
- Contract description, combo legs, and delta-neutral payloads are
  source-defined shared-schema branches. Combo legs and delta-neutral values
  project into public `Contract`; description is response-only. No nondefault
  205 contract-data response for those branches was captured, so that positive
  response behavior remains explicitly unattested.
- Event-contract fields 59..61 are source-defined and projected, but remained
  absent in the captured stock, bond, fund, and option matrix.

Version 206 market data is audited separately in
[`protocol-audit-sv206.md`](protocol-audit-sv206.md).

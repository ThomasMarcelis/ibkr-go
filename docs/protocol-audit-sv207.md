# Server version 207 accounts and positions audit

This document freezes the complete `server_version 207` migration boundary.
The evidence is official API 10.48.01 source and protobuf schemas, an official
SDK session capped at exactly 207, and native `ibkr-go` public operations
against the same local read-only Gateway at exact 207.

## Boundary

Ten requests move from classic fields to protobuf bodies:

| Base ID | Raw ID | Schema | Public operation |
|---:|---:|---|---|
| 6 | 206 | `AccountDataRequest` | account updates subscribe/unsubscribe |
| 17 | 217 | `ManagedAccountsRequest` | managed-account refresh |
| 61 | 261 | `PositionsRequest` | positions snapshot/stream |
| 64 | 264 | `CancelPositions` | positions cancellation |
| 62 | 262 | `AccountSummaryRequest` | account summary snapshot/stream |
| 63 | 263 | `CancelAccountSummary` | account summary cancellation |
| 74 | 274 | `PositionsMultiRequest` | model-aware positions |
| 75 | 275 | `CancelPositionsMulti` | model-aware positions cancellation |
| 76 | 276 | `AccountUpdatesMultiRequest` | model-aware account updates |
| 77 | 277 | `CancelAccountUpdatesMulti` | model-aware account cancellation |

Thirteen callbacks move with them:

| Base ID | Raw ID | Typed result |
|---:|---:|---|
| 6 | 206 | `UpdateAccountValue` |
| 7 | 207 | `UpdatePortfolio` |
| 8 | 208 | `UpdateAccountTime` |
| 54 | 254 | `AccountDownloadEnd` |
| 15 | 215 | `ManagedAccounts` |
| 61 | 261 | `Position` |
| 62 | 262 | `PositionEnd` |
| 63 | 263 | `AccountSummaryValue` |
| 64 | 264 | `AccountSummaryEnd` |
| 71 | 271 | `PositionMulti` |
| 72 | 272 | `PositionMultiEnd` |
| 73 | 273 | `AccountUpdateMultiValue` |
| 74 | 274 | `AccountUpdateMultiEnd` |

PnL and family codes do not migrate at 207. They retain their existing body
encoding until their later official migration groups.

## Codec semantics

- False booleans and empty strings use protobuf absence. In particular,
  account-update unsubscribe omits `subscribe`, while the account remains
  present. `AccountUpdatesMultiRequest.LedgerAndNLV` controls the corresponding
  classic field and is omitted from protobuf when false.
- Request IDs are range-checked protobuf `int32` values. Missing correlated
  callback IDs use the official `NO_VALID_ID` default of `-1`.
- Position quantities remain decimal strings. Optional portfolio and average-
  cost doubles default to zero and use the shared round-trip-safe formatter.
- Position, portfolio, and position-multi callbacks without their required
  nested Contract emit no typed callback, matching the official decoder.
- Unknown protobuf fields are skipped. Empty end messages remain callbacks.
- All nested Contracts use the same shared Contract decoder introduced for the
  earlier protobuf migrations.
- Bootstrap managed accounts is load-bearing at this boundary: `DialContext`
  must decode raw 215 before the session can become ready.
- Account-update time callbacks are exposed as `AccountUpdate.UpdateTime`
  instead of being silently discarded after decode.

Production remains pure Go and uses `protowire` directly. No SDK code,
generated protobuf type, cgo dependency, or private capture is on the default
import path.

## Live evidence

The official Python SDK 10.48.01 was used only as a capture oracle, capped to
exactly 207. The read-only scenario refreshed managed accounts; requested and
cancelled positions, account summary, positions multi, account updates multi,
and account updates; and used current time as a final write fence.

- capture: `20260711T011842Z-sdk_sv207_accounts_positions_boundary`
- `events.jsonl`: `936f9f4ea1633770071d9bd07a5ec721b7ddd481fcae6ad1aac95a9c1287a153`
- `meta.json`: `8de75d54d2f47723315192d6b0af2f3b90dbef26ec3f06e681c3558d1c409b2b`
- normalized frames: `408b8726ca3a7b713d6905b917cca7b2b9952081902aa384847da41d1580a846`

The native library then advertised 207 to the local Gateway and completed the
same public account families. A five-leg public capture retained managed-
account refresh, account summary, positions, account updates multi, and
positions multi; account updates was also run directly at exact 207.

- capture: `20260711T012845Z-public_sv207_accounts_positions_boundary`
- `events.jsonl`: `951ea55770f30d410cf971a5bf7bbf94da0183017cac0984b6b5d0370ab7ad86`
- `meta.json`: `335e205a424323310b4e410a28d4f6442e7fcb32d4b40f5f6bf4f022ddc432f4`

The checked-in `managed_accounts_sv207_live.txt` replay retains the exact
protobuf bootstrap and explicit refresh exchange with a deterministic,
length-preserving account substitution. Codec tests retain one exact vector
for every migrated request and callback family rather than copying the large
account snapshots into the repository.

## Regression gates

- Byte-exact captured vectors cover all ten request types and all thirteen
  callback types.
- A 206/207 boundary test proves classic versus protobuf selection.
- Missing-Contract behavior is frozen against the official decoder law.
- The public replay proves exact-207 negotiation, bootstrap, refresh routing,
  and session-snapshot update.
- `TestLiveManagedAccountsRefresh` requires exact 207 when live tests run.
- The full deterministic suite, race detector, vet, staticcheck, and repository
  lint remain release gates.

The next protocol migration boundary is `server_version 208`, not an extension
of this account-domain slice.

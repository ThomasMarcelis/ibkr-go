# Architecture

`ibkr-go` is built as a session engine with a typed facade. The library does
not expose an `EWrapper` / `EClient` callback surface as its primary model.

The library currently exposes a broad read-only surface plus order management,
market depth, and option exercise. Its classic baseline is live-attested at IB
Gateway `server_version 200`; exact `server_version 201` adds the negotiated
raw-ID envelope and protobuf executions flow. Exact `server_version 202` adds
zero-strike contract semantics but no message migration. Exact 203 migrates the
order placement/cancellation lifecycle to protobuf. Exact 204 migrates the
open/completed-order query family and completed-order replies. Exact 205
migrates contract-details requests and replies. Exact 206 migrates quote,
market-depth, and market-data-type requests plus their L1/depth callbacks.
Exact 207 migrates the accounts and positions request/callback family. The
session handshake accepts 200..225. Exact 208..213 complete the staged
protobuf migrations; inbound sv214 `Z` parsing is supported while outbound
formatting remains unresolved, and 215..225 add cancellation, order,
configuration, volume/precision, and odd-lot gates. See
[`protocol-audit-sv208-225.md`](protocol-audit-sv208-225.md).

## Layers

- module root: public typed facade plus the unexported session engine, split
  by concern and domain — lifecycle, run loop, connect/reconnect, routing,
  conversion, and one file per domain (`engine_account.go`,
  `engine_orders.go`, `engine_marketdata.go`, etc.) — plus correlation and
  subscription management
- `internal/transport/`: socket dial, buffered frame read loop, write loop,
  pacing
- `internal/protocol/`: dependency-free message identity, direction,
  negotiated classic/protobuf envelope, migration gates, and supported-version
  bounds
- `internal/codec/`: typed message encode/decode, split into per-domain files
  (`codec_orders.go`, `codec_marketdata.go`, etc.), the inbound decode
  registry, and protocol-owned version-gate aliases
- `internal/wire/`: frame and field framing
- `internal/testhost/`: deterministic replay and fault-injection harness for
  checked-in fixtures

## Runtime Model

- One session actor goroutine owns mutable state.
- One reader goroutine reads frames and forwards decoded messages to the actor.
  It reads through a 64 KiB `bufio.Reader` rather than issuing the raw
  length-prefix-then-payload read pair directly on the socket, collapsing two
  syscalls per frame into roughly one per buffer fill; the reader goroutine
  owns every post-handshake read, so this is safe without extra
  synchronization.
- The reader rejects a frame from its four-byte length header before allocating
  or reading the body when it exceeds `WithMaxInboundFrameBytes`. The default
  and hard ceiling are 64 MiB; the same limit covers the handshake and steady
  state.
- One writer goroutine serializes outbound frames and applies global pacing.
- Public methods talk to the actor through typed commands instead of sharing
  mutable maps or callback registries.
- Work submitted while an automatic reconnect is in progress is held in an
  actor-owned FIFO and released immediately after existing resumable routes are
  restored. Context cancellation removes pending work through
  `context.AfterFunc`; readiness uses no polling loop or retry timer.

## Codec Dispatch

- **Envelope.** Below server version 201, the message ID is the first classic
  NUL-delimited ASCII field. At 201, every normal frame starts with a raw
  four-byte big-endian wire ID. Wire IDs 1..200 have a classic field body;
  wire IDs above 200 select protobuf and map to base ID `wireID-200`. The
  pre-session server-info frame remains classic and has no message envelope.
- **Decode.** `codec.DecodeBatch` separates this negotiated envelope before
  inspecting the body. Classic bodies go through `inboundDecoders`, an
  explicit `map[int]decodeFunc`; protobuf bodies go through the equally
  explicit `inboundProtobufDecoders`. Unknown classic messages preserve their
  fields, while unknown protobuf messages preserve their binary body without
  interpreting embedded NUL bytes.
- **Encode.** Every message struct implements `encodeWire(sv int) ([]string,
  error)` directly; the `Message` interface is that encode capability, so a
  struct without `encodeWire` does not compile as a `Message` and encode
  coverage is checked at compile time. Migrated requests additionally
  implement the local protobuf encoder capability. The protocol migration
  table rejects a request once its protobuf gate is reached unless that
  encoder exists; it never falls back to an invalid classic body.
- **Field parsing.** `fieldReader` is a lazy cursor over the frame's backing
  byte slice, not a pre-split `[]string`. Numeric and boolean fields parse in
  place through transient `unsafe.String` views handed to `strconv`; only
  fields a decoder actually retains (`ReadString`, `ReadDecimal`) copy into a
  new Go string. Retained strings are always copied, never aliased into the
  transport buffer — aliasing would couple message lifetimes to buffer
  reuse, the symmetric silent-corruption failure mode this codebase
  deliberately avoids.
- **Request-ID routing.** Inbound messages that carry a request ID implement
  `codec.ReqIDer` (`RequestID() int`); the engine's keyed routing table type-
  asserts against this interface instead of maintaining a parallel switch.
  `OpenOrder`, `OrderStatus`, and `CompletedOrder` deliberately do not
  implement it. Open-order and order-status messages carry an order ID and
  dual-dispatch through order observation; completed orders belong to their
  request-ID-less singleton snapshot. `APIError` is routed ahead of keyed
  dispatch because its `ReqID` can be `-1` for unsolicited errors.

## Protocol Evidence

- `internal/protocol` is the numeric source of truth. Registry invariants reject
  duplicate names and duplicate IDs within a direction.
- `internal/codec/codec_capture_coverage_test.go` is the machine-checked inbound
  evidence ledger. Every registered decoder is either tied to a named test over
  a hardcoded live-derived frame or carries a concrete pending-capture reason.
- `cmd/ibkr-capture -list-json` is the executable scenario ledger. Repository
  audits require every scenario message ID to exist in the protocol registry.
- `docs/ibkr-api-inventory.md` records the official and public surfaces. Its
  Markdown is descriptive, never an independent source for numeric message
  identities.

## Routing Tables

The engine maintains three routing tables, each serving a different dispatch
pattern:

- **Keyed (`map[int]*route`)** — request-ID-correlated flows. One-shots and
  keyed subscriptions (account summary, quotes, historical bars, market depth,
  etc.) register a route keyed by `reqID`. Inbound messages
  carry the same `reqID` and dispatch directly to the registered handler.

- **Singleton (`map[string]*route`)** — flows that have at most one active
  instance and no request-ID correlation. Positions, open orders, family codes,
  news bulletins, and other singleton flows are keyed by a string constant.
  Inbound messages dispatch by message type to the matching singleton key.

- **Orders (`map[int64]*orderRoute`)** — per-order lifecycle tracking. Each
  placed order registers a route keyed by `orderID`. OpenOrder, OrderStatus,
  Execution, and commission-and-fees messages dispatch to the matching order route.

- **Passive execution observer** — at most one client-wide
  `SubscribeExecutionEvents` route owns no request ID and sends no wire
  request. Every execution-detail and commission callback reaches it before
  query correlation or order-handle deduplication.

### Open-order and order-handle routing

OpenOrder messages are dispatched to both the per-order handle (if one exists
in the orders table) and the singleton open-orders observer (if one is
registered). `Orders().SubscribeOpen` therefore observes open-order snapshots and
updates through `OpenOrder`. OrderStatus messages are likewise dispatched to
both owners, while execution and commission messages are routed through
`OrderHandle` and the execution observers, not the open-orders observer.

## Version Negotiation

- The negotiated `server_version` is threaded explicitly through the codec
  rather than read from shared state: every `encodeWire` method, every decode
  function, and the `codec.Encode`/`DecodeBatch` entry points take `sv int`.
  The engine records the negotiated version at handshake and the decode pump
  captures it by value when it attaches a transport, so each reconnect
  decodes with its own freshly negotiated version even if the Gateway answers
  differently on redial.
- The supported range is exactly 200..225. Version 200 owns one fixed classic
  layout; versions 201..213 progressively switch the message families named in
  `internal/protocol/version.go` to protobuf, and 215..225 gate later semantics.
  Handshake rejects versions outside that range instead of carrying dead
  classic-layout compatibility branches.
- Classic OpenOrder decoding is strict: every supported field is consumed in
  sequence and malformed or trailing layout drift fails the affected route.
  It never returns a partially decoded broker echo.
- The advertised handshake maximum (`maxServerVersion`, currently 225) is a
  package-level override point used only by the version-matrix live tests to
  force a lower supported layout for verification; production
  code always advertises the maximum.

## Order ID Management

Order IDs are auto-allocated from `NextValidID`, which is received during
bootstrap and tracked on `Snapshot`. Each `Orders().Place` call increments
the counter atomically within the actor goroutine. Callers never need to manage
order IDs manually. The engine rejects values outside the protocol's signed
32-bit range before encoding and does not accept an out-of-range
`nextValidId` as bootstrap evidence. `Orders().PlaceBracket` reserves three consecutive IDs in
one actor turn and sends all three frames without interleaving another request;
the final child is the only frame with `Transmit=true`.

Admission to the transport queue transfers placement ownership to the caller.
The buffered handle result therefore wins context and shutdown races after
admission. The transport retains a pending-write marker until each order frame
is written; transport loss closes any definitely-unwritten handle with
`ErrInterrupted`. A partially admitted bracket cancels only admitted IDs and always
returns an `OrderRecoveryError` containing every one of those IDs. Even a
queue-admitted cancellation is unacknowledged, so callers reconcile open orders
before retrying; `CancelErr` only records failures to admit those cancellations.

## OrderHandle Lifecycle

`Orders().Place` returns an `OrderHandle` that tracks a single order's lifecycle:

- **Events()** delivers one ordered stream of `OrderEvent` values (union of
  OpenOrder, OrderStatus, Execution, CommissionAndFees, Warning, Binding, and
  Lifecycle — exactly one field non-nil per event).
  A `Warning` is a non-terminal, order-targeted notice (e.g. code 399, the
  off-hours deferral): the order stays working at IB and the handle stays open.
- **Lifecycle events** are part of that same stream: Started, Gap, Restored,
  and RecoveryRequired cannot race past business events on another channel.
- **Event backpressure.** Each handle has a bounded, lossless event queue
  (default 64, configured by `WithOrderEventBuffer`). Overflow closes local
  observation with `ErrSlowConsumer` rather than dropping and continuing. It
  does not change the live order; its OrderID remains available for
  reconciliation or cancellation.
- **Terminal order states.** Filled, Cancelled, APICancelled, and Inactive are
  business events, not local observation boundaries. Execution and fee
  callbacks may follow them, so the caller closes the handle when its evidence
  requirements are satisfied. Cancellation replies 161 and 202 remain session
  notices and do not close the handle.
- **Disconnect.** On session disconnect, active order handles receive `Gap`.
  A data-maintained restoration yields `Restored`; a socket reconnect yields
  `RecoveryRequired` because fills or status changes may have occurred during
  the gap. Handles stay open, but replacement is permanently unavailable on
  that handle and reports non-retryable `ErrOrderRecoveryRequired`.
- **Close()** detaches the handle from the engine. The order continues
  executing on the server; the caller simply stops receiving events.
- **Cancel(ctx)** sends a CancelOrder request for this order.
- **Replace(order)** sends a modified PlaceOrder with the same OrderID.

## Protocol Realities

- Request correlation is split between keyed flows, singleton flows, and
  order flows. Not all protocol areas route cleanly through one
  `reqID -> channel` map.
- Snapshot completion is driven by explicit protocol end markers, never by
  silence or timeouts.
- Global pacing belongs in the write path. Endpoint-specific admission limits
  belong at the session layer.
- Public request setup waits for a usable session. Reconnect backoff and
  endpoint pacing are internal engine concerns, bounded by the caller context.
- Managed accounts, negotiated server version, and next valid id are bootstrap
  state, not ordinary request/response calls.

## Public Direction

- `DialContext` returns a ready session, not a raw TCP socket.
- Managed accounts are bootstrap state on the session snapshot.
- One-shots, subscriptions, and order handles are separate public contracts.
- Subscriptions expose data and lifecycle boundaries through one ordered
  `Events()` stream. `All(ctx)` is the data-only view over that same queue.
  It also consumes and discards request-scoped `StreamNotice` values, so
  consumers that need warnings use `Events()` directly.
  Channel close plus `Err()`/`Wait()` is terminal. OrderHandle uses the same
  single-stream principle with order-specific events.

These public contracts are intended to survive the remaining protocol work.

## Reconnect

- Reconnect policy is a client policy; the default remains `ReconnectAuto`.
- Resume policy is a per-subscription policy.
- One-shots are never replayed automatically.
- Order handles survive disconnects and require explicit reconciliation after
  an observation gap.
- Session reconnect boundaries are surfaced via `ConnectionSeq`.
- `SessionEvents` is bounded and drop-oldest, but each actual state transition
  increments `TransitionSeq`; gaps are therefore detectable, and every event
  carries its exact resulting `Snapshot`.

See [`operation-control.md`](operation-control.md) for the per-operation
request-ID, cancellation, detach, and connection-retirement matrix.

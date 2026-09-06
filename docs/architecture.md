# Architecture

`ibkr-go` is built as a session engine with a typed facade. The library does
not expose an `EWrapper` / `EClient` callback surface as its primary model.

This document explains internal ownership and data flow. Public lifecycle,
queue limits, and recovery behavior belong to the
[session contract](session-contract.md); protocol boundaries and their evidence
belong to [the version audit](protocol-audit-sv208-225.md).

## Layers

- module root: public typed facade plus the unexported session engine, split
  by concern and domain: lifecycle, run loop, connect/reconnect, routing,
  conversion, one file per domain (`engine_account.go`, `engine_orders.go`,
  `engine_marketdata.go`, and so on), plus correlation and subscription
  management; the engine owns dialing and the handshake
- `internal/transport/`: post-handshake buffered frame reads, writes, and pacing
- `internal/protocol/`: message identity, direction, negotiated
  classic/protobuf envelope, migration gates, and supported-version bounds; the
  envelope reuses `internal/wire` frame sentinels
- `internal/codec/`: typed message encode/decode, split into per-domain files
  (`codec_orders.go`, `codec_marketdata.go`, etc.), the inbound decode
  registry, and direct use of protocol-owned version gates
- `internal/wire/`: frame and field framing
- `internal/testhost/`: deterministic replay and fault-injection harness for
  checked-in fixtures

## Runtime Model

- One session actor goroutine owns mutable state.
- The transport reader owns socket reads and publishes complete frames. The
  engine's decode pump converts them and forwards typed messages to the actor.
  The transport reads through a 64 KiB `bufio.Reader` rather than issuing the raw
  length-prefix-then-payload read pair directly on the socket, collapsing two
  syscalls per frame into roughly one per buffer fill; the reader goroutine
  owns every post-handshake read, so this is safe without extra
  synchronization.
- The reader checks the frame length before allocating its body. Handshake
  and steady-state reads share the configured limit.
- One writer goroutine serializes outbound frames and applies global pacing.
- Public methods enqueue closures on the actor's `chan func()`; the actor owns
  mutable maps and callback registries.
- Work submitted while an automatic reconnect is in progress is held in an
  actor-owned FIFO. A generation-specific barrier admits the retained market
  data selection first, then resumable routes in request-ID order, then new
  work. A full transport queue uses its writable edge to continue the barrier.
  Context cancellation removes pending work through
  `context.AfterFunc`; readiness uses no polling loop or retry timer.

## Codec Dispatch

- **Envelope.** Every supported normal frame starts with a raw four-byte
  big-endian wire ID. Wire IDs 1..200 have a classic field body; wire IDs
  above 200 select protobuf and map to base ID `wireID-200`. The pre-session
  server-info frame remains a NUL-delimited field sequence with no message
  envelope.
- **Decode.** `codec.DecodeBatch` separates this negotiated envelope before
  inspecting the body. Classic bodies go through `inboundDecoders`, an
  explicit `map[int]decodeFunc`; protobuf bodies go through the equally
  explicit `inboundProtobufDecoders`. Unknown classic messages preserve their
  fields, while unknown protobuf messages preserve their binary body without
  interpreting embedded NUL bytes.
- **Encode.** `Message` is the decode-result type and has no encoding
  capability. Client-to-Gateway structs implement the sealed `OutboundMessage`
  capability accepted by production `Encode`. Post-handshake testhost replies
  are exact captured raw frames; there is no symbolic server encoder. `UnknownInbound`
  is decode-only. Migrated requests additionally implement the
  local protobuf encoder capability. The protocol migration table rejects a
  request once its protobuf gate is reached unless that encoder exists; it
  never falls back to an invalid classic body.
- **Field parsing.** `fieldReader` is a lazy cursor over the frame's backing
  byte slice, not a pre-split `[]string`. Numeric and boolean fields parse in
  place through transient `unsafe.String` views handed to `strconv`; only
  fields a decoder actually retains (`ReadString`, `ReadDecimal`) copy into a
  new Go string. Copying gives retained fields independent ownership without
  keeping the entire frame allocation alive.
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
- `docs/ibkr-api-inventory.md` maps the official EClient/EWrapper surface to
  what `ibkr-go` implements. It is descriptive, never an independent source
  for numeric message identities.

## Routing Tables

The engine maintains four route maps, each serving a different dispatch
pattern, plus one passive execution observer:

- **Keyed (`map[int]*route`).** Request-ID-correlated flows. One-shots and
  keyed subscriptions (account summary, quotes, historical bars, market depth,
  etc.) register a route keyed by `reqID`. Inbound messages
  carry the same `reqID` and dispatch directly to the registered handler.

- **Singleton (`map[string]*route`).** Flows that have at most one active
  instance and no request-ID correlation. Positions, open orders, family codes,
  news bulletins, and other singleton flows are keyed by a string constant.
  Inbound messages dispatch by message type to the matching singleton key.

- **Orders (`map[int64]*orderRoute`).** Per-order lifecycle tracking. Each
  placed order registers a route keyed by `orderID`. OpenOrder, OrderStatus,
  Execution, and commission-and-fees messages dispatch to the matching order route.

- **Previews (`map[int64]*previewRoute`).** Isolated what-if requests keyed by
  their engine-owned order ID. The matching OpenOrder echo resolves the preview
  without creating or dispatching to a live order handle.

- **Passive execution observer.** At most one client-wide
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
- The supported range is exactly 208..225. Families already migrated by 208
  use protobuf throughout the supported range; versions 209..213 switch the
  remaining families named in `internal/protocol/version.go`, and 214..225 gate
  later semantics. Handshake rejects versions outside that range.
- Protobuf OpenOrder decoding is strict: every supported canonical value must
  decode completely, and malformed numerics or layout drift fail the affected
  route. It never returns a partially decoded broker echo.
- The advertised handshake maximum (`advertisedServerVersionMax`, currently 225) is a
  package-level override point used only by the version-matrix live tests to
  force a lower supported layout for verification; production
  code always advertises the maximum.

## Order ownership

The actor allocates order IDs from the bootstrap `NextValidID` and reserves a
bracket's consecutive IDs in one turn. `pendingOrderWrites` maps each tracked
transport write to its order ID. The write-completion pump publishes all
results before reporting transport loss; the actor drains them before applying
the loss transition. This makes definitely-unwritten placement and complete
local acceptance visible without treating either as a broker acknowledgement.

Each order route owns one handle, its established permanent identity, pending
write, and recovery state. The replacement closure captures the immutable
contract and parent at placement. Exercise observation uses an order route plus
a keyed route whose shared cleanup removes both registrations. Preview routes
remain separate and resolve on their matching open-order echo.

`execDeliveries` is actor-owned correlation for order handles. Each entry
retains an execution's owner, last delivered fee, and any fees waiting for the
execution. The map size and pending-report count enforce the configured limits.
`closeOrderRoute` releases claimed entries; closing the final route clears the
map. Query correlation and the passive execution observer have independent
ownership. The [session contract](session-contract.md#orderhandle) defines
closure, overflow scope, late fees, and recovery consequences.

## Reconnect and teardown

A physical reconnect captures a new negotiated version and transport generation.
Decode and write-result pumps preserve their originating connection identity so
stale work cannot mutate a replacement connection. `marketDataTypeGeneration`
records where the client's retained selection was admitted; same-socket
restoration therefore does not resend it. The resume-capacity waiter belongs to
one transport and cannot release another transport's barrier.

`closeOrderRoute` and `deleteKeyedRoute` own route cleanup; `closeEngine` owns
client shutdown. Subscription cancellation may retire a connection when local
admission fails or a late request-ID-less response cannot safely be separated
from a replacement request. See [operation control](operation-control.md) for
the per-operation cancellation and retirement matrix.

## Request and order identity

Request IDs and order IDs share the Gateway error-correlation namespace. The
actor allocates positive request IDs while avoiding active orders, previews,
and routes. Order allocation also respects the Gateway's next-valid-ID floor
and IDs observed from other clients before avoiding active collisions. These
are different allocation rules within one namespace; an error's numeric ID
alone cannot identify a safe order action.

Direct cancellation therefore requires `OrderTarget{ClientID, OrderID}`. A
caller can use an observed same-client identity after a process restart;
process-local allocation history is not the source of broker ownership.

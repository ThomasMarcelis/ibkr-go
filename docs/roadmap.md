# Roadmap

## Current State

ibkr-go is being migrated to one production protocol engine: the official IBKR
C++ SDK through the repo-owned C ABI in `internal/sdkadapter/native`.

The public Go API still contains the broad facade shape built during the
socket-era implementation. The SDK-native runtime currently supports:

- session bootstrap and shutdown through the official SDK;
- server metadata, managed accounts, and next valid order ID;
- `Client.CurrentTime` and `Client.CurrentTimeMillis`;
- `Accounts().Summary` and `Accounts().SubscribeSummary`;
- `Accounts().Positions` and `Accounts().SubscribePositions`;
- `Accounts().Updates`, `Accounts().SubscribeUpdates`,
  `Accounts().UpdatesMulti`, `Accounts().SubscribeUpdatesMulti`,
  `Accounts().PositionsMulti`, `Accounts().SubscribePositionsMulti`,
  `Accounts().SubscribePnL`, and `Accounts().SubscribePnLSingle`;
- `Accounts().FamilyCodes`;
- `MarketData().SetType`, `MarketData().Quote`,
  `MarketData().SubscribeQuotes`, `MarketData().SubscribeRealTimeBars`,
  `MarketData().SubscribeTickByTick`, and `MarketData().SubscribeDepth`;
- `Contracts().Details` and `Contracts().Qualify`;
- `Contracts().Search` and `Contracts().MarketRule`;
- `Contracts().SecDefOptParams`, `Contracts().SmartComponents`,
  `Contracts().DepthExchanges`, and `Contracts().FundamentalData`;
- `History().Bars`, `History().SubscribeBars`, `History().HeadTimestamp`,
  `History().Histogram`, `History().Ticks`, and `History().Schedule`;
- `Options().ImpliedVolatility`, `Options().Price`, and `Options().Exercise`;
- `Orders().Place`, `Orders().Cancel`, `Orders().CancelAll`,
  `Orders().Open`, `Orders().SubscribeOpen`, `Orders().Completed`,
  `Orders().Executions`, and `OrderHandle.Modify`;
- `News().Providers`, `News().Article`, `News().Historical`, and
  `News().SubscribeBulletins`;
- `Scanner().Parameters` and `Scanner().SubscribeResults`;
- `Advisors().Config`, `Advisors().ReplaceConfig`, and `Advisors().SoftDollarTiers`;
- `WSH().MetaData` and `WSH().EventData`;
- `TWS().UserInfo`, `TWS().DisplayGroups`, and
  `TWS().SubscribeDisplayGroup`;
- cancellation for account summary, account updates, account updates multi,
  positions, positions multi, PnL, PnL single, orders, news bulletins, scanner
  subscriptions, display group subscriptions, real-time bars, historical bars,
  tick-by-tick data, market depth, quote streams, historical ticks, option
  calculations, head timestamp, histogram data, WSH metadata/event data, and
  fundamental data.

Rows without current SDK live evidence or replay fixtures are marked blocked
with the exact external prerequisite or safe-test condition that is missing. The
authoritative migration inventory is [`sdk-migration-matrix.md`](sdk-migration-matrix.md).

Legacy socket/codec transcripts and live coverage remain useful source
material, but they do not count as completed SDK-native production coverage
until promoted into copied SDK command/event records, native ABI coverage,
SDK-event replay fixtures, and SDK-backed live verification.

## Migration Order

### 1. SDK Adapter Foundation

- Keep `internal/sdkadapter` as the copied-value command/event schema and
  replay fixture package.
- Keep `internal/sdkadapter/native` as the only production SDK bridge.
- Make every slice, map, and string-owning value crossing goroutine or C ABI
  boundaries copy-owned.
- Extend architecture guards so public code cannot expose C++, cgo, SDK,
  SWIG, protobuf, or old wire-message concepts.
- Quarantine or remove old socket/codec/wire production paths from the default
  build.

### 2. Read-Only One-Shots

Port finite read-only calls before broad streaming or trading work. Remaining
finite targets now belong to their owning market-data, order, option-exercise,
or FA-write groups.

Each port needs command/event records, native C ABI coverage, deterministic
SDK-event replay, and read-only live smoke when a local Gateway/TWS is
available.

### 3. Streaming Subscriptions

Market-data, account, scanner, news, and display-group streams now have
SDK-native command/event coverage, but they still need live-derived SDK replay
fixtures before their matrix rows can move to done.

The public `Subscription[T]` contract remains the target: business events on
`Events()`, lifecycle on `Lifecycle()`, explicit snapshot boundaries where the
SDK provides them, and deterministic close/wait/error semantics.

### 4. Orders And Execution Lifecycle

Port trading after the read-only and streaming foundations are stable:

- next-valid-ID order allocation;
- broaden paper-account live evidence beyond the SDK-backed
  place/cancel/modify smokes and client-scope open-orders fixture into
  completed orders, executions, commissions, and cleanup behavior;
- order-handle lifecycle across reconnect;
- bracket, OCA, condition, algo, scale, hedge, combo, short-sale, FA, and
  advanced reject fields where the public `Order` surface claims support.

Live trading tests must run only against paper/sandbox Gateway/TWS with
harness-level checks that refuse real-account writes.

### 5. Documentation And Release Readiness

- Keep README, GoDoc, examples, and docs aligned with actual SDK-backed
  behavior.
- Keep the migration matrix current after every capability slice.
- Keep deterministic tests, `go vet ./...`, `gofmt -l .`, shell syntax checks,
  SDK-tagged tests, and live smoke results documented.

## Non-Goals

- Flex Web Service.
- Client Portal Web API.
- An `EWrapper` / `EClient` compatibility bridge.
- SWIG as the production binding layer.

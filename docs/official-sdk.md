# Official SDK Runtime

`ibkr-go` uses the official IBKR C++ SDK as its protocol engine through a
narrow manual cgo boundary. The production direction is a repo-owned C ABI and
C++ adapter, not SWIG.

Every production connection goes through this adapter. Capability groups are
tracked in the migration matrix until typed command/event records, native
adapter coverage, and live-derived adapter fixtures are all complete.

The current native slice is session/bootstrap, current time/current time
millis, account summary, account updates, account updates multi, positions,
positions multi, PnL, PnL single, family codes, contract details/qualification,
market-data type control, contract search, market rules, sec-def option params,
smart components, quote snapshots and streams, real-time bars, tick-by-tick
data, market depth streams, market-depth exchange metadata, head timestamp,
histogram data, fundamental data, news providers, news bulletins, news articles,
historical news, scanner parameters, scanner result subscriptions, option
implied-volatility and price calculations, historical bars, historical bar
subscriptions, historical schedule, historical ticks, option exercise,
soft-dollar tiers, WSH
metadata/event data, user info, display groups, display group subscriptions, FA
config reads and writes, order placement and modification, open-order snapshots
and subscriptions, completed-order snapshots, execution snapshots, commission
reports, and cancellation for account summary, account updates, account updates
multi, positions, positions multi, PnL, PnL single, order cancellation, global
cancellation, news bulletins, scanner subscriptions, display group
subscriptions, quote streams, real-time bars, tick-by-tick data, market depth,
historical bars, historical ticks, option calculations, head timestamp,
histogram data, WSH metadata/event data, and fundamental data.
See
[`sdk-migration-matrix.md`](sdk-migration-matrix.md) for the full current
inventory.

The SDK-owned foundation is split by responsibility:

- `internal/sdkadapter`: copied command and event values, fixture schema, and
  replay adapter.
- `internal/sdkadapter/native`: cgo wrapper and C++ adapter around the official SDK.

Native connect records server metadata, starts the SDK reader, and explicitly
requests managed accounts plus `reqIds(1)` so bootstrap is driven by official
SDK callbacks.

## Local Setup

Download and accept the official IBKR TWS API package locally, then keep the
SDK tree outside git tracking. A checked-in helper validates the expected Linux
toolchain and emits cgo flags:

```bash
scripts/check-ibkr-sdk-env.sh /path/to/IBJts
scripts/scan-ibkr-sdk-drift.sh /path/to/IBJts
eval "$(scripts/check-ibkr-sdk-env.sh --print-env /path/to/IBJts)"
go test -tags=ibkr_sdk ./...
```

The checker validates the API version file, C++ headers, generated protobuf
headers, `protoc`, C++14 compiler support, the SDK library, protobuf linker
flags, Intel decimal `libbid` linkage, and a minimal SDK link/runtime probe.
Its `--print-env` mode also injects the SDK API version into the native adapter
build info.
The drift scanner compares the native adapter's current request/callback use
against the installed SDK headers.

Never commit SDK source, generated SDK output, SDK binaries, Intel decimal
artifacts, account captures, private IBKR documentation, or unsanitized live
data.

## Live Smoke

With a local Gateway or TWS on `127.0.0.1:4002`, run the SDK-backed live smoke:

```bash
eval "$(scripts/check-ibkr-sdk-env.sh --print-env /path/to/IBJts)"
IBKR_LIVE=1 IBKR_LIVE_ADDR=127.0.0.1:4002 \
  go test -tags=ibkr_sdk ./ -run TestLiveOfficialSDKSmoke -count=1 -v
```

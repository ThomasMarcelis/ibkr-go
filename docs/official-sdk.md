# Official SDK Runtime

`ibkr-go` uses the official IBKR C++ SDK as its protocol engine through a
narrow manual cgo boundary. The production direction is a repo-owned C ABI and
C++ adapter, not SWIG.

Current SDK-backed coverage is the first typed ABI vertical slice: session
bootstrap, `CurrentTime`, `CurrentTimeMillis`, account summary, contract
details, and positions. Unsupported requests fail closed until each capability
group has typed command/event records, native adapter coverage, and
live-derived adapter fixtures.

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
The drift scanner compares the adapter's current request/callback use against
the installed SDK headers.

## Live Smoke

With a local Gateway or TWS on `127.0.0.1:4002`, run the SDK-backed live smoke:

```bash
eval "$(scripts/check-ibkr-sdk-env.sh --print-env /path/to/IBJts)"
IBKR_LIVE=1 IBKR_LIVE_ADDR=127.0.0.1:4002 \
  go test -tags=ibkr_sdk ./ -run TestLiveOfficialSDKSmoke -count=1 -v
```

Do not commit SDK source, generated SDK output, SDK binaries, Intel decimal
artifacts, or account captures.

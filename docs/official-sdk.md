# Official SDK Runtime

`ibkr-go` is migrating to the official IBKR C++ SDK as its protocol engine
through a narrow manual cgo boundary. The production direction is a repo-owned
C ABI and C++ adapter, not SWIG.

Current SDK-backed coverage is the first vertical slice: session bootstrap,
`CurrentTime`, and account summary. The rest of the public API still uses the
native Go engine until each capability group has SDK-backed parity and
live-derived adapter fixtures.

## Local Setup

Download and accept the official IBKR TWS API package locally, then keep the
SDK tree outside git tracking. A checked-in helper validates the expected Linux
toolchain and emits cgo flags:

```bash
scripts/check-ibkr-sdk-env.sh /path/to/IBJts
eval "$(scripts/check-ibkr-sdk-env.sh --print-env /path/to/IBJts)"
go test -tags=ibkr_sdk ./...
```

The checker validates the API version file, C++ headers, generated protobuf
headers, `protoc`, C++14 compiler support, the SDK library, protobuf linker
flags, and Intel decimal `libbid` linkage.

## Live Smoke

With a local Gateway or TWS on `127.0.0.1:4002`, run the SDK-backed vertical
slice explicitly:

```bash
eval "$(scripts/check-ibkr-sdk-env.sh --print-env /path/to/IBJts)"
IBKR_USE_OFFICIAL_SDK=1 IBKR_LIVE=1 IBKR_LIVE_ADDR=127.0.0.1:4002 \
  go test -tags=ibkr_sdk ./ -run TestLiveOfficialSDKVerticalSlice -count=1 -v
```

Do not commit SDK source, generated SDK output, SDK binaries, Intel decimal
artifacts, or account captures.

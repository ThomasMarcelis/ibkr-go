# v2.0.0 Release Evidence

v2.0.0 is the supported stable line; v1 is deprecated. The release is not
presented as having evidence that was never captured. The machine-readable
disclosure in `testdata/release/v2.0.0.json` must continue to name every known
evidence gap with its impact and follow-up.

## Known evidence gaps

- Exact server-version 200 MIDPOINT, BID, and ASK message-90 updates remain
  uncaptured. Official source defines a signed bar count, while the current
  classic decoder rejects a negative count and drops that update as malformed.
- Registered decoder failures do not yet retire the whole transport
  generation. A malformed row followed by a buffered end marker can therefore
  complete a partial snapshot.
- `IncludeOvernight` is source-grounded, but its focused sv200 true-to-false
  replacement and sv203 staged-to-transmitted live proofs remain missing.
- A successful fee-bearing regulatory snapshot remains unattested. The latest
  authorized attempt returned API code 10213.
- The client-0 manual-order `orderBound` path lacks a raw paper-TWS capture.
- The previously planned seven-day unchanged release-candidate soak was not
  completed. Post-publication defects must become v2.0.1; v2.0.0 is immutable.

These are evidence and hardening gaps, not completed proofs. The six-gap set
and completeness of its nonempty `Gap`, `Impact`, and `FollowUp` fields are
frozen by `TestV2StableReleaseManifest`; the exact prose is not.

## Follow-up capture pass

At market hours, capture the three classic message-90 variants before changing
signed-count decoding, then land registered-malformed generation retirement.
Run the two safely nonmarketable paper `IncludeOvernight` scenarios in the same
campaign. Preserve negotiated version, capture ID, full event hash,
cancellation, and a CurrentTime fence.

A regulatory snapshot remains fee-bearing and requires immediate explicit
authorization after the account restriction is resolved. Manual `orderBound`
evidence requires paper TWS—not Gateway—with client ID 0 and auto-open orders.
Every promoted proof must be a sanitized regular transcript under
`testdata/transcripts` whose initial comment block and handshake pass the
structural provenance parser.

## Stable release gate

Run on the exact tagged commit:

```bash
go mod tidy -diff
go mod verify
gofmt -l .
go build ./...
go vet ./...
CGO_ENABLED=0 GOOS=linux GOARCH=386 go build ./...
golangci-lint run
go test -shuffle=on -count=1 ./...
go test -race -shuffle=on -count=1 ./...
./scripts/check-api.sh --exact
./scripts/fuzz-all.sh --check
./scripts/fuzz-all.sh 30s
go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
IBKR_RELEASE_TAG=v2.0.0 \
  go test -shuffle=on -count=1 ./...
```

Maintainers also run `./scripts/verify-captures.sh` against the nonempty ignored
local capture corpus. Release CI verifies that `v2.0.0` is an annotated tag on
the `/v2` module path and publishes it as a stable GitHub release. Every test
run validates the frozen manifest; `IBKR_RELEASE_TAG` additionally binds it to
the initial `v2.0.0` tag and is not used for later compatible v2 releases.

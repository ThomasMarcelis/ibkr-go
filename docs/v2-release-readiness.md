# v2.0.1 release readiness

v2.0.1 is the current correction line. It deliberately supports exactly
`server_version` 208–225; Gateways negotiating 200–207 must upgrade or remain
on v2.0.0. The tagged v2.0.0 history, API manifest, and release evidence stay
immutable.

This document is a readiness checklist, not a release announcement. No tag,
push, or publication is authorized by completing it.

## Completed candidate work

- Production handshake validation accepts 208–225 and rejects 207 and 226.
- Protocol families already protobuf-only at the supported floor no longer
  retain their pre208 classic decoders.
- The tracked replay corpus contains 99 raw-frame transcripts: 95 at sv225,
  exact historical and user-info fixtures at sv208, one exact positive option-
  calculation fixture at sv211, and one exact sv208–225 handshake matrix.
  Every fixture has full capture-hash provenance and public replay or fault-
  injection coverage.
- Registered malformed callbacks poison and retire their complete transport
  generation. Incomplete routes match both `ErrInterrupted` and
  `*ProtocolError`; unknown IDs remain nonfatal.
- Historical subscription code 2188 is public and classified as an
  entitlement failure.
- A current positions subscription is frozen through `SnapshotComplete`.
- A current sv225 IBM CFD capture and public replay freeze the initial request,
  protobuf reroute, conID request, delayed-data notice, and positive quote
  callbacks.
- `IncludeOvernight=true` has a current placement and broker echo. The
  true-to-false replacement returned exact code 462 through both ibkr-go and
  SDK 10.48.01 and retained true. A fresh explicit-false placement was accepted
  and broker-canonicalized to an absent field with `TIF=DAY`.
- All 286 retained raw capture directories verify and negotiate sv208 or
  later. The exact enumerated pre208 corpus and quarantine were removed.
- The sole authorized regulatory attempt is permanently disabled in live
  tests. The post-attempt account-update capture reports `Billable=0.00 EUR`.
- The v2.0.0 API manifest remains unchanged. `v2.0.1.api` separately freezes
  the candidate surface, including the intentional `IncludeOvernight` pointer
  break and the code-2188 constant.

## Open evidence gates

- Positive current market-depth and option-exercise replays remain
  unavailable. The guarded sv225 option campaign qualified an ITM AAPL call,
  but exact warning 399 deferred its zero-fill seed order to the next options
  session; capture `a10ff5818916cad50192579a39ce046143a1123a5a26f51bf359f161a0b5ad2c`
  also proves fresh-generation reconciliation without mutation. Exact sv208
  historical bars and sv211 option calculations are replay-promoted; the
  current sv225 historical account remains blocked by code 2188.
- The complete request-family boundary campaign across sv208–225 is not done;
  the exact matrix currently proves handshake and `CurrentTime` reachability,
  while 20 decoder/layout pairs remain explicitly unattested. Exact classic
  sv208 boundary captures now cover the supported scanner, news, PnL,
  reference-data, time, display-group, and user-info callbacks that the
  available roles could produce.
- Manual `orderBound` requires a paper TWS rather than either available
  Gateway.
- The single newly authorized regulatory snapshot attempt returned code 0
  (`Internal server error`). Capture
  `20260824T195855Z-regulatory_snapshot_aapl_v201_authorized_once`, events
  SHA-256
  `bca23abbf9e562746b79b758378fcd6752130c0b489a53fd97db3fac3ba3a2e2`,
  must never be retried. Post-attempt capture
  `20260824T202345Z-account_updates` (events SHA-256
  `d7063b2455654c8aed9ecd6c9f395addf9f95a2bf70a08eff7af99bc28707f6c`)
  reports `Billable=0.00 EUR` and is replay-promoted.
- The exact candidate has not completed the seven-day unchanged soak.

## Candidate command gate

Run on the exact committed candidate tree:

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
```

Also verify every retained raw capture, transcript provenance, both role
doctors, the safe live suites, and paper baseline reconciliation. Do not repeat
the regulatory snapshot.

## Unchanged soak

After every evidence and command gate passes, commit one candidate tree and
record its commit/tree hash only in ignored local evidence. Keep that tree
unchanged for 168 consecutive hours, run the complete gate and safe live checks
at the start and end, and run the ordinary deterministic suite plus
non-mutating live smoke checks daily. Any tracked correctness change restarts
the soak. A transient Gateway outage does not reset the unchanged-code clock,
but its missed live check must be rerun.

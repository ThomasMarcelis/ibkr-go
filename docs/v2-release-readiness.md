# v2.0.1 release record

v2.0.1 is the released correction line. It deliberately supports exactly
`server_version` 208–225; Gateways negotiating 200–207 must upgrade or remain
on v2.0.0. The tagged v2.0.0 history, API manifest, and release evidence stay
immutable.

This document records the evidence accepted for the release and distinguishes
known limitations from positive proof.

## Completed release work

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
- All 328 retained raw capture directories verify and negotiate sv208 or
  later. The exact enumerated pre208 corpus and quarantine were removed.
- The sole authorized regulatory attempt is permanently disabled in live
  tests. The post-attempt account-update capture reports `Billable=0.00 EUR`.
- The v2.0.0 API manifest remains unchanged. `v2.0.1.api` separately freezes
  the release surface, including the intentional `IncludeOvernight` pointer
  break and the code-2188 constant.
- The complete build, vet, lint, shuffled, race, exact-API, timed-fuzz,
  vulnerability, capture, and transcript-provenance command gate passes. Both
  sv225 role doctors and the explicitly non-mutating safe live suites pass.
  The regulatory request was not repeated.

## Known limitations and v2.0.2 evidence backlog

- `IncludeOvernight=true` has a current placement and broker echo. The required
  true-to-false replacement instead returned exact code 462 through both
  ibkr-go and SDK 10.48.01 and retained true. A fresh explicit-false placement
  was accepted and broker-canonicalized to an absent field with `TIF=DAY`.
  This proves the wire distinction and broker blocker, but not a distinct false
  replacement echo. v2.0.1 exposes the evidenced pointer semantics without
  claiming broker acceptance of that replacement.
- Positive current market-depth evidence and a terminal option-exercise replay
  remain unavailable. The guarded sv225 option campaign bought one qualified
  ITM AAPL call and the exercise instruction reached `PreSubmitted` after exact
  warning 10349, but it produced no terminal status. Capture
  `37bfe1e3c3494f54e2f953936996086ecd31f9d7f2f0d6cb8ef2dd2a2323d4e2`
  also proves fresh-generation reconciliation: cleanup sold only the campaign
  option delta. Later cleanup capture
  `f5ad48b54b8fc0867aeaa10931107b9850fb750ef1488fff245de11f43dd077c`
  records exact code 10147 (`OrderId 8 ... is not found`) after targeted and
  global cancellation attempts. The final open-order snapshot is empty, and
  the final position plus execution/fee rows exactly match their pre-cleanup
  snapshots. No terminal exercise status was observed, so settlement remains
  unproven. Exact sv208 historical bars and sv211 option calculations are
  replay-promoted; the current sv225 historical account remains blocked by
  code 2188.
- The complete request-family boundary campaign across sv208–225 is not done;
  the exact matrix currently proves handshake and `CurrentTime` reachability,
  while 20 decoder/layout pairs remain explicitly unattested. Exact classic
  sv208 boundary captures now cover the supported scanner, news, PnL,
  reference-data, time, display-group, and user-info callbacks that the
  available roles could produce.
- Manual `orderBound` remains unproven. Paper TWS is now online at `7497`, but
  no manual TWS order was created while the API harness was armed; that action
  requires explicit user participation and is not GUI-automated.
- The single newly authorized regulatory snapshot attempt returned code 0
  (`Internal server error`). Capture
  `20260824T195855Z-regulatory_snapshot_aapl_v201_authorized_once`, events
  SHA-256
  `bca23abbf9e562746b79b758378fcd6752130c0b489a53fd97db3fac3ba3a2e2`,
  must never be retried. Post-attempt capture
  `20260824T202345Z-account_updates` (events SHA-256
  `d7063b2455654c8aed9ecd6c9f395addf9f95a2bf70a08eff7af99bc28707f6c`)
  reports `Billable=0.00 EUR` and is replay-promoted.

## Release command gate

The exact release tree passed:

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

The command block, all 328 raw captures, transcript provenance, both doctors,
and both safe role suites passed on 2026-08-27. Final paper snapshots contain no
ordinary open order and match the reconciled position and execution/fee
baseline. These results close the mechanical release gate; they do not turn
the open entitlement, broker, and manual-interaction gaps above into positive
evidence.

# v2 Release Readiness

RC.3 is the final breaking API and protocol candidate. A correctness, public
API, or supported-wire-layout change after the candidate is cut requires RC.4
and restarts the stable soak. Documentation-only evidence promotion does not
change the candidate when it describes already-frozen behavior.

## RC.3 candidate gate

- the public API baseline is regenerated only after the advanced-order and
  broker-echo model is frozen;
- module, formatting, API, vet, test, race, 386, lint, vulnerability, capture,
  and exact 17-target fuzz gates pass;
- both configured Gateway roles complete the supported-version live matrix at
  200, 203, and 225, subject to the current Gateway accepting the requested
  lower maximum; run `TestLiveReleaseVersionSmoke` once against each role and
  record any Gateway-side lower-version refusal rather than hiding it;
- invalid private captures stay outside the active evidence corpus; and
- every checked-in transcript either has a live provenance header or is named
  in `testdata/transcripts/provenance.json`.

RC.3 has no transcript-provenance exceptions. The remaining recovery fault
tests inject typed engine events directly; no synthetic 1100/1101/1102 wire
transcript is represented as live evidence.

## Stable-only blockers

The machine-readable state is `testdata/release/v2.0.0.json`. Stable-tag CI
sets `IBKR_STABLE_RELEASE=1`; that makes both the readiness manifest and the
transcript-provenance inventory release gates. Changing the manifest to
`ready` requires structured role, server-version, capture, event-hash, and
transcript records for both live proofs, plus seven consecutive dated feedback
records that name the same 40-character RC candidate revision.

Transcript replacement and retirement is complete: three redundant replays
were removed, historical and concurrent-one-shot replays were rebuilt from
exact captures, transport reconnect was grounded in an exact quote capture,
and the synthetic connectivity fixtures were replaced by direct engine fault
tests.

The remaining requirements are:

1. Obtain immediate explicit authorization, then record one successful
   fee-bearing regulatory snapshot. Record role, negotiated server version,
   capture ID, event hash, sanitized transcript, and billing reconciliation.
2. Run paper TWS, create a manual order, and promote the raw `orderBound`
   callback through the client-0 auto-open-orders path. Gateway-only evidence
   cannot satisfy this requirement.
3. Complete seven consecutive calendar days of feedback monitoring on the
   exact unchanged RC.3 candidate. Any correctness, public API, or protocol
   change resets day one and requires RC.4.

The latest authorized attempt was made on 2026-07-15 through `readonly-live`
at `server_version 225`. Exactly one regulatory request was sent, and the
Gateway definitively rejected request ID 2 with API code 10213. The verified
private capture is
`20260715T152010Z-regulatory_snapshot_aapl_authorized`; its events SHA-256 is
`42e466e6af9ecb65d091579cd4b4546815bc414fd76b6457aa5618621c40dff4`.
No retry was made. This does not satisfy requirement 1, so the manifest proof
remains `null`. Resolve the account-access restriction before seeking fresh
authorization for another fee-bearing attempt.

Consumer upgrades are valuable validation but are independent projects and do
not gate this library release.

## Manual orderBound capture

This opt-in harness connects as client ID 0, enables auto-open-order binding,
waits for one manual paper-TWS order, validates the positive binding IDs, then
disables binding and proves the session remains usable. Start paper TWS on the
paper API port first; IB Gateway cannot produce this callback.

Record it through the raw proxy:

```bash
go build -o /tmp/ibkr-recorder ./cmd/ibkr-recorder
/tmp/ibkr-recorder -listen 127.0.0.1:4102 -upstream 127.0.0.1:4002 \
  -out /tmp/ibkr-v2-rc3 -scenario manual_tws_order_bound -client-id 0 \
  -notes 'manual paper-TWS order bound through client 0' -timeout 5m
```

In another shell, wait until the recorder is listening, then run:

```bash
IBKR_LIVE=1 IBKR_LIVE_TRADING=1 \
IBKR_LIVE_PAPER_ADDR=127.0.0.1:4102 IBKR_LIVE_CLIENT_ID=0 \
IBKR_LIVE_MANUAL_ORDER_BOUND=1 \
go test . -run '^TestLiveManualTWSOrderBound$' -count=1 -v -timeout=4m
```

Create one safely non-marketable manual paper order only after the test prints
`AUTO-OPEN ARMED`, and cancel it in TWS after capture.
Verify and sanitize the capture before promoting its raw callback; do not add
`order_bound` evidence to the readiness manifest until the promoted transcript
passes replay and the capture hash is recorded.

## Soak record

Day zero is the unchanged RC.3 revision after the full release gate passes.
For each following calendar day, review repository issues/discussions and
known consumer feedback for correctness, API, and protocol regressions. Put a
dated record of the surfaces checked in `gate_record`; do not pre-populate or
backfill it. Re-run the full release gate on day seven before changing the
manifest to `ready`. Any candidate code change invalidates every recorded day.

| Day | Date | Candidate | Gate record |
|---:|---|---|---|
| 1 | pending | pending | pending |
| 2 | pending | pending | pending |
| 3 | pending | pending | pending |
| 4 | pending | pending | pending |
| 5 | pending | pending | pending |
| 6 | pending | pending | pending |
| 7 | pending | pending | pending |

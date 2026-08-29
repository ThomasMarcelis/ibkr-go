# Roadmap

Next steps, in priority order. What is proven lives in
[`live-coverage-matrix.md`](live-coverage-matrix.md); what shipped lives in
the [CHANGELOG](../CHANGELOG.md).

## Next

1. **Release v2.0.3.** Ships the empty-TIF fix (constructor-built orders
   were rejected with code 10052 on sv225), the 10052 rejection
   classification, the width-preserving perm-ID sanitizer, the `bracket`
   example, and the docs cleanup. Follow the release checklist in
   [`CONTRIBUTING.md`](../CONTRIBUTING.md).
2. **Settle cross-client cancel.** The promoted captures show a second client
   ID cancelling a `Submitted` order; a 2026-07-04 probe outside regular hours
   drew code 10147 for the same request against a `PreSubmitted` order.
   Re-probe during US market hours and fix the fixtures or the docs.
3. **Market-hours evidence that needs an API market-data subscription** on one
   of the two logins: real-time bars, tick-by-tick, market depth, odd-lot
   ticks, and historical bars, ticks, histograms, and keep-up-to-date
   updates. The decoders exist; the pending rows in
   `internal/codec/codec_capture_coverage_test.go` are the checklist.
4. **Evidence that needs an account product or an event:** FA configuration,
   Wall Street Horizon, bulletins, EFP and delta-neutral ticks (a single-stock
   future or an accepted BAG), and manual `orderBound` (paper TWS with a
   user-entered order). Nothing to build; capture when available.
5. **Raise the ceiling to sv226** when a Gateway negotiates it: add
   `Order.conditionsIncludeOvernight` (API 10.50.01 field 145) and extend the
   exact negotiation matrix.
6. **Track official API releases** for schema drift against the 10.50.01
   baseline.

## Open questions

- `Quote` carries IBKR's `-1` "no quote" sentinel as a decimal. Decide whether
  an absent bid or ask should be unset instead.
- An order-targeted error outside the live-attested rejection set leaves a
  never-acknowledged handle open. Decide whether a bounded no-acknowledgement
  window should close it.

## Code hygiene

From the August 2026 review; none of these change the public API.

- Replace the repeated hand-rolled "request ID plus repeated submessage"
  protowire loops in `internal/codec` with one helper.
- Drop the `sv` parameter from codec functions that never gate on it, and the
  `unparam` exclusion that hides them.
- Split `cmd/ibkr-capture/api_scenarios.go` by domain.

## Not planned

- Client Portal Web API or Flex.
- An `EWrapper` / `EClient` compatibility bridge.
- The official SDK as a runtime dependency.
- Server-log, verification/auth, redirect, or operator-configuration mutation.

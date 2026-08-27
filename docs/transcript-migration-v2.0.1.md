# v2.0.1 transcript migration inventory

This is the completed deletion and evidence inventory for the move to
`server_version` 208–225. It records the v2.0.1 release tree as of 2026-08-27.

## Current tracked corpus

The tracked corpus contains 99 transcripts, all with raw captured frames and
full `events.jsonl` SHA-256 provenance:

- 95 fixtures negotiate sv225.
- `historical_bars_sv208.txt` is exact positive sv208 historical-bar evidence.
- `user_info.txt` is exact sv208 user-info evidence.
- `option_calculations_aapl.txt` is exact positive sv211 option-price and
  implied-volatility evidence.
- `supported_version_matrix.txt` negotiates every exact version from sv208
  through sv225. Capture
  `20260824T213929Z-supported_version_matrix_paper`, events SHA-256
  `64ee4350f0bde347a9da914a82865e88e0a68d06924cb13335fd2084595a7727`.
- No tracked transcript negotiates a version below 208 or uses a legacy hash
  prefix. `TestTranscriptProvenanceInventory` freezes those invariants and the
  count of 99.

The parent corpus had 134 files. Eighty-seven retained names were replaced in place
with verified supported-range captures; their migration mapping is identity by
filename, and each initial comment block names the replacement capture and full hash.
The remaining twelve current files are explicit additions or renames:

| Current fixture | Replaces or proves |
|-----------------|--------------------|
| `cfd_quote_reroute.txt` | current IBM CFD request, protobuf reroute, conID request, delayed-data notice, and positive quote callbacks |
| `executions_concurrent_aapl.txt` | current simultaneous BUY- and SELL-filtered execution queries; replaces the derived `executions_correlated.txt` and `executions_overlapping.txt` fixtures |
| `executions_empty.txt` | current empty execution snapshot; replaces `executions_empty_sv201_live.txt` |
| `open_orders_empty.txt` | current empty open-order snapshot; replaces `open_orders_empty_sv203_live.txt` |
| `historical_bars_subscription_required.txt` | exact sv225 historical code 2188 entitlement blocker |
| `historical_bars_sv208.txt` | exact positive sv208 bars; replaces unsupported `grounded_historical_bars.txt` proof |
| `include_overnight_lifecycle_aapl.txt` | true placement echo and exact code-462 true-to-false replacement blocker |
| `option_calculations_aapl.txt` | exact positive sv211 option calculations; replaces `option_calculations_aapl_live.txt` |
| `positions_subscription.txt` | current subscription rows through `SnapshotComplete`, cancellation, and fence |
| `scanner_subscription.txt` | current ten-row HOT_BY_VOLUME result; replaces `scanner_subscription_live.txt` |
| `supported_version_matrix.txt` | exact sv208–225 handshake and `CurrentTime` reachability |
| `tick_news_aapl.txt` | current sv225 public TickNews callback; replaces `tick_news_aapl_live.txt` |

Every retained fixture is referenced by a public replay or a deterministic
fault-injection test. Fault injection changes only transport chunking, delay,
disconnect timing, or malformed truncation around captured frames.

## Removed pre208 fixtures with current replacements

These files no longer add distinct proof. Their still-supported behavior is
covered by the named sv225 fixture or family:

- `api_market_data_type_cycle.txt` → `set_type_switch_while_streaming.txt` and
  `quote_snapshot.txt`.
- `api_order_handle_reconnect_cancel_aapl.txt` and
  `api_reconnect_recovered_cancel_status_aapl.txt` →
  `api_reconnect_active_order_aapl.txt`.
- `api_order_rest_cancel_161_aapl.txt` → `api_order_rest_cancel_aapl.txt`.
- `api_realtime_bars_request_errors_aapl.txt` →
  `realtime_bars_api_error.txt`.
- `api_security_type_probe_errors.txt` → the current stock, option, future,
  forex, bond, ambiguous, and not-found contract-detail fixtures.
- `api_stop_loss_management_aapl.txt` → `api_order_stop_cancel_aapl.txt` and
  the current bracket fixtures.
- `completed_orders_cancelled_system_live.txt` and `completed_orders_empty.txt`
  → `api_completed_orders_variants_aapl.txt`.
- `executions_correlated.txt` and `executions_overlapping.txt` →
  `executions_concurrent_aapl.txt`. The replacement is a real concurrent
  public-API capture rather than a derived duplicate/rebound execution stream.
- `historical_news_end_bound_sv206_live.txt` → `news_article.txt`.
- `tick_news_aapl_live.txt` → `tick_news_aapl.txt`.
- `place_order_fill_native_execution_time.txt`, `place_order_lmt_buy_aapl.txt`,
  `place_order_mkt_buy_aapl.txt`, `place_order_mkt_sell_aapl.txt`, and
  `place_order_modify_to_market_late_execution.txt` →
  `api_order_fill_aapl.txt`.
- `place_order_invalid_type_live_error.txt` → `api_order_rejects_aapl.txt`.
- `whatif_rejections.txt` and `whatif_tif_default_live.txt` →
  `api_whatif_margin_aapl.txt`.
- `lifecycle_bootstrap_reordered.txt` was removed because it fabricated a
  server callback order that was not observed live and is outside the allowed
  fault transformations. Current bootstrap success and independently missing
  callback failures remain frozen from exact sv225 frames.

The following files proved only a deleted pre208 boundary and have no runtime
counterpart after the floor increase:

- `completed_orders_sv204_live.txt`
- `contract_details_sv205_live.txt`
- `executions_zero_strike_sv202_live.txt`
- `managed_accounts_sv207_live.txt`
- `market_data_sv206_live.txt`
- `order_lifecycle_sv203_live.txt`

## Removed fixtures whose positive proof is still open

These pre208 files were removed so the active test corpus cannot imply support
from an unsupported protocol version. They are not counted as positive sv208+
proof. Their positive replacements remain follow-up coverage work where the
behavior stays in scope:

- `api_open_orders_refresh_aapl.txt`
- `api_option_exercise_not_itm_aapl.txt`
- `api_option_exercise_server_reject_aapl.txt`
- `api_scale_in_campaign_aapl.txt`
- `api_tick_by_tick_entitlement_errors_aapl.txt`
- `histogram_data.txt`
- `historical_bars_108_end.txt`
- `historical_ticks_bid_ask.txt`
- `historical_ticks_midpoint.txt`
- `historical_ticks_trades.txt`
- `news_bulletins_live.txt`
- `open_orders_auto_refresh.txt`
- `open_orders_readonly_refusal_sv206_live.txt`
- `open_orders_snapshot_burst_live.txt`

Exact entitlement, account, market-state, or TWS-only errors may explain why a
positive callback is unavailable, but they do not become positive proof. The
coverage matrix and live tracker must keep these distinctions visible.

## Current blocker evidence

- Historical requests returned exact typed code 2188 at sv225. This grounds
  `ErrCodeHistoricalDataSubscriptionRequired` and entitlement classification.
  Exact positive sv208 bars are separately replay-promoted.
- Market depth returned exact code 10092; no current positive depth row is
  available.
- Capture `20260825T181306Z-api_include_overnight_lifecycle_aapl` (events
  SHA-256
  `f3585ff96e9a0a936a559e52e9d69e860e6182e769418d987314ce9419c90e9c`)
  proves a
  nonmarketable one-share SMART DAY placement with `IncludeOvernight=true` and
  its true broker echo. Replacing it with false returned code 462. SDK 10.48.01
  reproduced the blocker, so it is not evidence for an encoding change and it
  does not prove a false replacement echo. A fresh explicit-false placement
  was accepted and broker-canonicalized to an absent field with `TIF=DAY`.
  This does not prove the required distinct false replacement echo, so the
  capture remains blocker evidence rather than completed positive proof.
- The option exercise instruction reached `PreSubmitted` but never emitted a
  terminal status. Fenced targeted/global cleanup was followed by exact code
  10147 because order 8 was no longer found. Fresh open-order, position,
  execution, and fee captures reconcile to the pre-cleanup state, but they do
  not turn the missing terminal callback into settlement proof.
- Exact sv211 public option calculations returned option-price availability
  `247` and implied-volatility availability `133` and are replay-promoted.
- The sole newly authorized regulatory attempt is capture
  `20260824T195855Z-regulatory_snapshot_aapl_v201_authorized_once`, events
  SHA-256
  `bca23abbf9e562746b79b758378fcd6752130c0b489a53fd97db3fac3ba3a2e2`.
  It returned code 0 (`Internal server error`) and must never be retried.
  Post-attempt capture `20260824T202345Z-account_updates` (events SHA-256
  `d7063b2455654c8aed9ecd6c9f395addf9f95a2bf70a08eff7af99bc28707f6c`)
  reports `Billable=0.00 EUR` and is replay-promoted.
- Manual `orderBound` remains blocked because no manual paper-TWS order was
  created while the API harness was armed. TWS is now reachable at `7497`, but
  the required manual create/cancel action was not performed.
- The exact-version transcript proves handshake and `CurrentTime` reachability,
  not the complete request-family boundary campaign.

## Raw-corpus deletion result

The enumerated pre208 directories and quarantine were removed recoverably,
without a broad glob or unresolved version predicate. The retained raw corpus
contains 328 directories. `scripts/verify-captures.sh` passes for all of them,
and every retained handshake negotiates sv208 or later.

# Examples

Complete programs against a real TWS or IB Gateway session. Start with the
five small ones; the advanced set adds one production concern at a time without
hiding the public API behind example-only helpers.

Enable the socket API in TWS or Gateway, then set `IBKR_ADDR`. The commands
below use the paper Gateway's default port `4002`; a live Gateway usually
listens on `4001`. Delayed quotes need no market-data subscription. Historical
data and scanners may still depend on the login's permissions, and the examples
say so when IBKR refuses.

## Start here

| Example | What it shows | Typical run |
| --- | --- | --- |
| [`connect`](connect/) | Dial, session snapshot, server time, shutdown | under 10s |
| [`quotes`](quotes/) | Delayed bid/ask/last through `Subscription.All` | under 20s |
| [`historical`](historical/) | Typed one-shot AAPL OHLCV bars, typed entitlement error | under 15s |
| [`portfolio`](portfolio/) | Summary, positions, first P&L update | under 30s |
| [`order`](order/) | Paper limit order, typed events, confirmed cancellation | under 30s; paper only |

```bash
IBKR_ADDR=127.0.0.1:4002 go run ./examples/connect
IBKR_ADDR=127.0.0.1:4002 go run ./examples/quotes
IBKR_ADDR=127.0.0.1:4002 go run ./examples/historical
IBKR_ADDR=127.0.0.1:4002 go run ./examples/portfolio
IBKR_ADDR=127.0.0.1:4002 IBKR_TRADING=paper go run ./examples/order
```

## Advanced

| Example | What it adds | Typical run |
| --- | --- | --- |
| [`bracket`](bracket/) | Entry plus take-profit and stop-loss as one `PlaceBracket`, cancelled as a unit | under 45s; paper only |
| [`option-chain`](option-chain/) | Qualify an underlying, pick a chain, resolve one full expiry | under 45s |
| [`scanner`](scanner/) | One ranked scan snapshot, clean unsubscribe, quote its leader | under 35s |
| [`resilient-quotes`](resilient-quotes/) | Automatic reconnect and resume on one ordered data and lifecycle stream | until Ctrl-C |
| [`margin-preview`](margin-preview/) | Typed what-if margin and commission, without a resting order | under 30s; paper only |

```bash
IBKR_ADDR=127.0.0.1:4002 IBKR_TRADING=paper go run ./examples/bracket
IBKR_ADDR=127.0.0.1:4002 go run ./examples/option-chain
IBKR_ADDR=127.0.0.1:4002 go run ./examples/scanner
IBKR_ADDR=127.0.0.1:4002 go run ./examples/resilient-quotes
IBKR_ADDR=127.0.0.1:4002 IBKR_TRADING=paper go run ./examples/margin-preview
```

The order-shaped examples require `IBKR_TRADING=paper` and a session whose
managed accounts all carry IBKR's `DU` paper prefix; they refuse to run
otherwise. `order` and `bracket` rest far from the market and cancel
themselves; `margin-preview` uses `Orders().Preview`, which creates no order.

Every program is built by the normal repository gates, and the maintainers run
them against a live paper Gateway before a release.

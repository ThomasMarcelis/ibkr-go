# Examples

These are complete programs against a real TWS or IB Gateway session. Start
with the five small examples; the advanced set adds one production concern at
a time without hiding the public API behind example-only abstractions.

Enable the socket API in TWS or Gateway, then set `IBKR_ADDR`. The commands
below use the conventional live read-only port `4001`; paper Gateway commonly
uses `4002`. Delayed quotes do not require a real-time market-data subscription.
Historical data and scanners may still depend on the account's permissions.

## Start here

| Example | What it shows | Typical run |
| --- | --- | --- |
| [`connect`](connect/) | Dial, ready-session metadata, server time, shutdown | under 10s |
| [`quotes`](quotes/) | Delayed BBO, available-field checks, `Subscription.All` | under 20s |
| [`historical`](historical/) | Typed one-shot AAPL OHLCV bars | under 15s |
| [`portfolio`](portfolio/) | Summary, positions, nullable P&L values | under 30s |
| [`order`](order/) | Paper limit order, typed events, confirmed cancellation | under 30s; paper only |

```bash
IBKR_ADDR=127.0.0.1:4001 go run ./examples/connect
IBKR_ADDR=127.0.0.1:4001 go run ./examples/quotes
IBKR_ADDR=127.0.0.1:4001 go run ./examples/historical
IBKR_ADDR=127.0.0.1:4001 go run ./examples/portfolio
IBKR_ADDR=127.0.0.1:4002 IBKR_TRADING=paper go run ./examples/order
```

## Advanced

| Example | What it adds | Typical run |
| --- | --- | --- |
| [`option-chain`](option-chain/) | Qualify an underlying, select chain parameters, resolve a full expiry | under 45s |
| [`scanner`](scanner/) | Assemble a ranked scan snapshot, cleanly unsubscribe, quote its leader | under 35s |
| [`resilient-quotes`](resilient-quotes/) | Automatic reconnect/resume on one ordered data and lifecycle stream | until Ctrl-C |
| [`margin-preview`](margin-preview/) | Typed what-if margin and commission, without a resting order | under 30s; paper only |

```bash
IBKR_ADDR=127.0.0.1:4001 go run ./examples/option-chain
IBKR_ADDR=127.0.0.1:4001 go run ./examples/scanner
IBKR_ADDR=127.0.0.1:4001 go run ./examples/resilient-quotes
IBKR_ADDR=127.0.0.1:4002 IBKR_TRADING=paper go run ./examples/margin-preview
```

The two order-shaped examples require both `IBKR_TRADING=paper` and a session
whose managed accounts all use IBKR's `DU` paper-account prefix. They refuse to
run otherwise. `margin-preview` uses `Orders().Preview`, which forces what-if
mode and creates no `OrderHandle`; `order` places a deliberately remote limit
order and waits for IBKR's terminal cancellation status before detaching.

All programs are built by the normal repository gates. The underlying protocol
paths have deterministic replay coverage grounded in sanitized live captures;
maintainers also live-run the read-only examples against the read-only Gateway
role and run order-shaped examples only against paper Gateway.

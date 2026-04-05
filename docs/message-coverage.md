# Message Coverage

This matrix tracks the v1 logical message surface that the public contract,
session engine, and scenario corpus are expected to cover.

Current repo truth:

- the checked-in scenarios and codec currently use repo-local symbolic message
  names
- this document is the target logical coverage map, not a claim that the live
  IBKR wire format is fully mapped today

## Bootstrap

- client hello
- server hello ack
- managed accounts
- next valid id
- current time
- API error / status codes

Bootstrap is load-bearing. `DialContext` is not ready until the negotiated
server version and managed-account bootstrap fields are known.

## Contract And Reference Data

- contract details request
- contract details
- contract details end

## Accounts And Positions

- account summary request / cancel
- account summary value
- account summary end
- positions request / cancel
- position
- position end

## Market Data

- quote request / cancel
- tick price
- tick size
- market data type
- tick snapshot end
- real-time bars request / cancel
- real-time bar
- historical bars request
- historical bar
- historical bars end

## Order / Execution Observation

- open orders request / cancel
- open order
- open order end
- executions request
- execution detail
- executions end
- commission report

## Session-Level Status

- API/system codes that drive `Ready`, `Degraded`, `Reconnecting`, and
  `Gap`/`Resumed` semantics

## Completion Markers

Snapshot and one-shot flows rely on explicit end markers:

- contract details end
- account summary end
- position end
- tick snapshot end
- historical bars end
- open order end
- executions end

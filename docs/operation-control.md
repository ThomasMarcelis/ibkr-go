# Operation control and cancellation

Context cancellation is not one universal wire operation. IBKR callbacks use
three ownership shapes, and the safe way to stop them depends on which shape
the operation owns:

- A request-ID-correlated reply can be detached locally without confusing a
  later request. When IBKR defines a cancel opcode, the client sends it too.
- A request-ID-less reply cannot be reassigned safely while its old callback
  may still arrive. Ending an unresolved operation therefore retires the
  owning connection generation.
- Orders and option exercises can have external effects. Ending local
  observation never implies that the server-side action was cancelled.

The context passed to an operation controls admission and observation; it does
not change these ownership facts. A context ending before admission sends no
request. After admission, the table below applies.

## Query and finite-stream matrix

| Public operations | Correlation and completion | Broker cancel | Early context end or Close | Connection impact |
|---|---|---|---|---|
| `Contracts.Details` / `StreamDetails` | request ID; `contractDetailsEnd` | `cancelContractData` at server version 215+ | cancel when supported, otherwise correlated local detach | retirement only if cancel admission fails |
| `Contracts.SecDefOptParams` / `StreamSecDefOptParams` | request ID; `securityDefinitionOptionParameterEnd` | none | correlated local detach | none |
| `Orders.Completed` / `StreamCompleted` | request-ID-less singleton; `completedOrdersEnd` | none | an unresolved stream cannot be detached | owning generation is retired before the end marker |
| historical bars/schedule, head timestamp, histogram, historical ticks, implied volatility, option price | request ID; operation-specific result or end marker | operation-specific cancel; historical-tick cancel requires server version 215+ | route removed and supported cancel sent | retirement only where a cancel cannot be admitted safely |
| WSH metadata and event data | request ID; one response | explicit WSH cancel | route removed and cancel sent | owning generation is retired if cancellation admission fails |
| contract search, smart components, news article/page, soft-dollar tiers, display groups, user info, and TWS config | request ID; one response or explicit end | none | correlated local detach | none |
| managed accounts, family codes, news providers, scanner parameters, market rule, current time, current time millis, FA config, and order-ID refresh | request-ID-less singleton; one response | none | unresolved ownership cannot be detached | owning generation is retired |

The slice-returning `Details`, `SecDefOptParams`, and `Completed` methods retain
their full finite result before returning. Their `Stream...` counterparts use
the ordinary bounded subscription queue and close automatically after
`SnapshotComplete`; use the streaming form when result cardinality can be
large. `WithQueueSize` controls that bound. Finite streams do not support
`ResumeAuto` because a partial result is not replay-safe.

## Subscription matrix

| Public operations | Broker ownership | Close behavior | Reconnect behavior |
|---|---|---|---|
| quotes and real-time bars | request-ID keyed; explicit cancel | cancel; failed admission retires the owning generation | optional `ResumeAuto` |
| account summary, account/position multi, PnL, tick-by-tick, depth, scanner results, display-group events, live historical bars | request-ID keyed; explicit cancel | cancel; failed admission retires the owning generation | `ResumeNever` |
| execution queries | request-ID keyed; no cancel opcode | correlated local detach | `ResumeNever`; late unknown fees remain outside that query route |
| positions and account updates | request-ID-less singleton; explicit cancel | before the initial snapshot boundary, retire the generation; afterward send cancel | `ResumeNever` |
| open orders | request-ID-less singleton | auto scope sends `cancelOpenOrders`; finite scopes detach only after their snapshot boundary | `ResumeNever` |
| news bulletins | request-ID-less singleton; explicit cancel | send cancel | `ResumeNever` |
| `SubscribeExecutionEvents` | passive local observer; no wire request | clean local detach; sends nothing | follows client reconnects and emits `Gap`/`Restored` |

For subscriptions with a broker cancel, `Subscription.Wait` returns
`*SubscriptionCancelError` if that cancel cannot enter the active transport
queue. The client retires the connection so the remote stream cannot remain
silently live. A clean `Wait` proves only that the local cancellation path was
admitted or detach was safe; it is not a broker acknowledgement.

`Subscription.All(ctx)` is deliberately data-only. It consumes and discards
`StreamNotice` as well as lifecycle events. Safety-conscious consumers that
need warnings or gap evidence must read `Subscription.Events()`.

## Orders and exercises

`OrderHandle.Close` and `ExerciseHandle.Close` detach observation only. They
send no order cancel and do not reverse an exercise instruction.
`OrderHandle.Cancel` and `Orders.Cancel` are the explicit targeted order-cancel
operations; `Orders.CancelAll` is the explicit global cancel. Placement and
exercise admission can leave externally visible work even if observation is
later lost, so their recovery errors are intentionally non-retryable.

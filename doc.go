// Package ibkr is a Go client for the Interactive Brokers TWS/Gateway socket
// protocol. It exposes broad account, contract, market-data, historical, order,
// option, news, scanner, advisor, and TWS functionality through typed methods
// and generic subscriptions with explicit lifecycle semantics. The client
// negotiates server_version 208 through 225. Versions below 208 are rejected;
// the supported range covers the remaining staged protobuf boundaries and
// semantic changes recorded in the project coverage matrix.
//
// # Connecting
//
// [DialContext] establishes a connection and returns a ready [Client]. It blocks
// until the handshake completes, the server version is negotiated, and managed
// accounts are loaded. Pass functional options to configure the connection:
//
//	client, err := ibkr.DialContext(ctx,
//	    ibkr.WithHost("127.0.0.1"),
//	    ibkr.WithPort(7497),
//	)
//	if err != nil {
//	    return err
//	}
//	defer client.Close()
//
// Once DialContext returns, [Client.Session] provides the negotiated server
// version, managed accounts, and connection sequence number. [Client.SessionEvents]
// is a bounded observational channel: if unread, older queued session events
// may be dropped in favor of the latest one. Event.TransitionSeq exposes such
// gaps, and Event.Snapshot is the exact post-transition session snapshot. It is
// one channel, not a broadcast: multiple readers divide the events.
//
// # Client Layout
//
// [Client] groups operations into domain sub-clients such as [Client.Accounts],
// [Client.Contracts], [Client.MarketData], and [Client.Orders]. A Stream-prefixed
// method returns a bounded finite stream that closes after SnapshotComplete; a
// Subscribe-prefixed method stays open until it is closed or fails. Option
// qualification and chain metadata live under [Client.Contracts];
// [Client.Options] owns calculations and exercise instructions.
//
// # One-Shot Requests
//
// Most query methods follow a simple call-and-return pattern. Pass a context
// for cancellation and a typed request; get back typed results:
//
//	details, err := client.Contracts().Qualify(ctx, ibkr.Contract{
//	    Symbol:   "AAPL",
//	    SecType:  ibkr.SecTypeStock,
//	    Exchange: "SMART",
//	    Currency: "USD",
//	})
//	if err != nil {
//	    return err
//	}
//	fmt.Println(details.LongName, details.MinTick)
//
// One-shots block until the server sends all result messages and the protocol
// completion marker. They return [*APIError] when the server rejects the request.
//
// # Subscriptions
//
// Streaming data uses [Subscription]. [Subscription.Events] is one ordered
// stream of [StreamEvent] values, so data cannot race past a reconnect boundary
// on a second channel. Event kinds are Started, Data, SnapshotComplete, Notice,
// Gap, Restored (a request-backed stream was retained, or passive execution
// observation resumed locally without sending a request), and Resubscribed
// (the client physically sent the request again). [Subscription.Done] closes
// when the subscription terminates; channel close plus Err or Wait is the
// terminal signal, so there is no redundant Closed event.
//
// Read [Subscription.Events] when lifecycle continuity or request-scoped
// notices matter:
//
//	sub, err := client.MarketData().SubscribeQuotes(ctx, req,
//	    ibkr.WithResumePolicy(ibkr.ResumeAuto))
//	if err != nil {
//	    return err
//	}
//	defer sub.Close()
//
//	for event := range sub.Events() {
//	    switch event.Kind {
//	    case ibkr.StreamData:
//	        fmt.Println(event.Value.Snapshot.Bid, event.Value.Snapshot.Ask)
//	    case ibkr.StreamNotice:
//	        log.Printf("quote notice: %v", event.Notice)
//	    case ibkr.StreamGap:
//	        log.Printf("quote continuity lost: %v", event.Err)
//	    case ibkr.StreamRestored, ibkr.StreamResubscribed:
//	        log.Printf("quote stream recovered")
//	    }
//	}
//	if err := sub.Err(); err != nil {
//	    return err
//	}
//
// [Subscription.All] is the data-only convenience iterator. It consumes and
// filters every non-data event from the same queue, including reconnect
// boundaries and request-scoped [StreamNotice] warnings. Events and All are
// alternative single consumers; use exactly one goroutine to drain one of them.
//
// Admission to the transport queue is the ownership boundary for a
// subscription. Once admitted, the subscription result wins context-cancellation
// and session-close races so a live remote stream is never hidden behind a
// context error.
//
// Call Close to unsubscribe. Wait blocks until termination and returns the
// final error, if any. A [*SubscriptionCancelError] means cancellation could
// not enter the active transport queue; the client retires that connection
// generation, so wait for a ready replacement before subscribing again. Err returns the
// currently recorded terminal error without waiting for Done. If slow-consumer
// shutdown also cannot admit its cancellation, the terminal
// error preserves both [ErrSlowConsumer] and [*SubscriptionCancelError].
// Use [IsRetryable] on the final error. Done is useful for coordinating other
// goroutines, but consumers that need every event should drain Events (or All)
// until it closes, then check Err or call Wait.
//
// # Order Management
//
// [OrdersClient.Place] submits an order and returns an [*OrderHandle] that tracks
// its full lifecycle. [OrderHandle.Events] is one ordered stream. [OrderEvent]
// is a union: exactly one of OpenOrder, Status, Execution, CommissionAndFees,
// Warning, Binding, or Lifecycle is non-nil per event. Terminal order statuses
// do not close the handle because executions and fees can follow; the caller
// closes the observation window after collecting the evidence it needs.
//
// The order-event queue is lossless and bounded. Its default capacity is 64
// and [WithOrderEventBuffer] configures it for every handle on the client. If
// it fills, the handle closes with [ErrSlowConsumer] instead of silently
// dropping events. Only local observation ends: the order may remain live at
// IBKR, and [OrderHandle.OrderID] remains the cancellation and reconciliation
// coordinate.
//
// [OrderHandle.Close] detaches the handle without cancelling the order.
// [OrderHandle.Cancel] sends a cancel request. [OrderHandle.Replace] sends a
// modified order with the same ID.
//
// Admission to the transport queue is the ownership boundary for Place and
// PlaceBracket. Once admitted, the handle result wins context-cancellation and
// session-close races. Every partially admitted bracket returns
// [*OrderRecoveryError] identifying all admitted order IDs whose live state
// must be reconciled before placing again; admission of a compensating cancel
// to the local queue is not a Gateway acknowledgement.
//
// # Margin Previews
//
// [OrdersClient.Preview] runs a what-if order: a margin-and-commission preview,
// not a trade. It forces the what-if flag, sends the same place_order frame,
// and returns the [OrderState] the Gateway attaches to the single open_order
// echo — the nine InitMargin*/MaintMargin*/EquityWithLoan* decimals plus the
// commission range and currency. Nothing rests on the server and no OrderHandle
// is created; Preview blocks for the one reply and returns:
//
//	state, err := client.Orders().Preview(ctx, ibkr.PlaceOrderRequest{
//	    Contract: contract,
//	    Order: ibkr.Order{
//	        Action:    ibkr.ActionBuy,
//	        OrderType: ibkr.OrderTypeMarket,
//	        Quantity:  decimal.NewFromInt(100),
//	    },
//	})
//	if err != nil {
//	    return err
//	}
//	fmt.Println(state.InitMarginAfter, state.CommissionAndFees)
//
// The what-if flag is operation-owned: previews go through Preview, while
// Place and [OrderHandle.Replace] always submit live orders.
//
// # Session Lifecycle
//
// The session state machine is observable through [Client.SessionEvents].
// States progress through Connecting, Handshaking, Ready, and optionally
// Degraded or Reconnecting on connection loss. Set [WithReconnectPolicy] to
// control automatic transport reconnection; the default is [ReconnectAuto].
// Reconnection does not imply request replay: request-backed subscriptions
// default to [ResumeNever]. [ResumeAuto] is supported only by streaming quotes
// and real-time bars, and reissues those requests after an automatic reconnect
// or a data-lost restoration (code 1101) on the existing socket.
//
// Calls made while an automatic reconnect is in progress wait for the next
// Ready transition or for their context to end. After a non-resumable
// long-lived subscription ends with [ErrResumeRequired], create a new
// subscription with a live context; the call waits for readiness when
// necessary.
//
// Across a connection continuity gap, ResumeAuto subscriptions and the passive
// execution observer receive a Gap event on their ordered Events stream.
// Recovery is Restored when IBKR retained a request-backed stream or passive
// execution observation resumed locally without sending a request; it is
// Resubscribed when the client sent the request again. Other long-lived
// request-backed subscriptions terminate with [ErrResumeRequired], while finite
// streams terminate with [ErrInterrupted]. Order handles report corresponding
// lifecycle values inside OrderEvent. No reconnect boundary is inferred from
// silence or split across a second channel.
//
// # Errors
//
// Nine structured error types cover the main failure modes:
//
//   - [*ConnectError] — connection or handshake failure
//   - [*ProtocolError] — wire protocol violation
//   - [*APIError] — server-side rejection (error code + message)
//   - [*ValidationError] — caller-side request validation failure
//   - [*OrderRecoveryError] — uncertain live IDs after partial bracket rollback
//   - [*ExerciseUncertainError] — unresolved exercise or lapse after involuntary observation loss
//   - [*RegulatorySnapshotUncertainError] — fee-bearing snapshot with unresolved completion evidence
//   - [*SubscriptionCancelError] — uncertain remote stream after cancellation admission failure
//   - [*InboundFrameTooLargeError] — raw frame rejected before body allocation
//
// IBKR codes attested in live captures have named ErrCode constants (e.g.
// [ErrCodeOrderCanceled], [ErrCodeMarketDataNotSubscribed]) and [*APIError]
// classification helpers ([APIError.IsEntitlement], [APIError.IsPacingViolation],
// [APIError.IsOrderRejection],
// [APIError.IsConnectivityTransition], [APIError.IsFarmStatus],
// [APIError.IsWarning]).
//
// Sentinel errors cover common conditions: [ErrNotReady], [ErrClosed],
// [ErrInterrupted], [ErrOrderRecoveryRequired], [ErrRegulatorySnapshotUncertain], [ErrSlowConsumer],
// [ErrExecutionCorrelationOverflow], [ErrNoMatch], [ErrAmbiguousContract].
// [IsRetryable] classifies final errors for retry/backoff policy. Recovery
// errors are never retryable because retrying can duplicate live orders or
// subscriptions.
//
// # Financial Types
//
// All prices, quantities, and money values use [github.com/shopspring/decimal.Decimal],
// an exact decimal type that avoids the rounding errors inherent in float64
// arithmetic. Construct values with decimal.NewFromString, decimal.NewFromInt,
// or decimal.RequireFromString.
package ibkr

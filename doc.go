// Package ibkr is a Go client for the Interactive Brokers TWS/Gateway socket
// protocol. It exposes broad account, contract, market-data, historical, order,
// option, news, scanner, advisor, and TWS functionality through typed methods
// and generic subscriptions with explicit lifecycle semantics. The supported
// and live-attested classic baseline is server_version 200; the project
// coverage matrix records partial, blocked, and future protocol areas.
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
// may be dropped in favor of the latest one.
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
// Streaming data uses [Subscription], a generic type that separates business
// events from lifecycle state. Every subscription exposes three channels:
//
//   - Events() delivers business data (quotes, bars, positions, etc.)
//   - Lifecycle() delivers lifecycle transitions ([SubscriptionStarted],
//     [SubscriptionSnapshotComplete], [SubscriptionGap], [SubscriptionResumed],
//     [SubscriptionClosed])
//   - Done() closes when the subscription terminates
//
// Lifecycle() is a bounded observational channel. If the caller stops draining
// it, older queued lifecycle events may be dropped in favor of the latest one.
// [SubscriptionClosed] is still guaranteed before the channel closes.
// [Subscription.AwaitSnapshot] is durable for snapshot-style subscriptions.
//
// The typical read loop ranges over [Subscription.All], an iterator that
// yields business events until the subscription closes or ctx is canceled:
//
//	sub, err := client.MarketData().SubscribeQuotes(ctx, req)
//	if err != nil {
//	    return err
//	}
//	defer sub.Close()
//
//	for update := range sub.All(ctx) {
//	    // handle business data
//	}
//	if err := sub.Err(); err != nil {
//	    return err
//	}
//
// Lifecycle transitions (SnapshotComplete, Gap, Resumed, Closed) are not part
// of that iteration; a consumer that needs them reads Lifecycle() from
// another goroutine, or drops to a manual select over Events() and
// Lifecycle() to observe both streams in one loop.
//
// Admission to the transport queue is the ownership boundary for a
// subscription. Once admitted, the subscription result wins context-cancellation
// and session-close races so a live remote stream is never hidden behind a
// context error.
//
// Call Close to unsubscribe. Wait blocks until termination and returns the
// final error, if any. A [*SubscriptionCancelError] means cancellation could
// not enter the active transport queue and the remote stream may still be live;
// recycle the client connection before subscribing again. Err returns the
// currently recorded terminal error without waiting for Done.
// If slow-consumer shutdown also cannot admit its cancellation, the terminal
// error preserves both [ErrSlowConsumer] and [*SubscriptionCancelError].
// SubscriptionClosed and Gap lifecycle events include a Retryable flag. API
// errors are terminal request rejections; use [IsRetryable] on the final error
// when a consumer loop only observes Events() or All(). Done is useful for
// coordinating other goroutines, but consumers that need every business event
// should drain Events (or All) until it closes, then check Err or call Wait.
//
// # Order Management
//
// [OrdersClient.Place] submits an order and returns an [*OrderHandle] that tracks
// its full lifecycle. The handle follows the same Events/Lifecycle/Done pattern as
// subscriptions. OrderEvent is a union: exactly one of OpenOrder, Status,
// Execution, CommissionAndFees, or Warning is non-nil per event. The handle
// auto-closes when the order reaches a terminal state (Filled, Cancelled,
// Inactive). Its
// Lifecycle() channel is also bounded and observational: if unread, older queued
// lifecycle events may be dropped in favor of the latest one.
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
// control automatic reconnect behavior.
//
// During a reconnect cycle, active subscriptions receive a Gap event through
// Lifecycle(). When the connection is re-established, subscriptions that support
// resume receive a Resumed event. The reconnect boundary is always explicit
// and never mixed into business event streams.
//
// # Errors
//
// Six structured error types cover the main failure modes:
//
//   - [*ConnectError] — connection or handshake failure
//   - [*ProtocolError] — wire protocol violation
//   - [*APIError] — server-side rejection (error code + message)
//   - [*ValidationError] — caller-side request validation failure
//   - [*OrderRecoveryError] — uncertain live IDs after partial bracket rollback
//   - [*SubscriptionCancelError] — uncertain remote stream after cancellation admission failure
//
// IBKR codes attested in live captures have named ErrCode constants (e.g.
// [ErrCodeOrderCanceled], [ErrCodeMarketDataNotSubscribed]) and [*APIError]
// classification helpers ([APIError.IsEntitlement],
// [APIError.IsConnectivityTransition], [APIError.IsFarmStatus],
// [APIError.IsWarning]).
//
// Sentinel errors cover common conditions: [ErrNotReady], [ErrClosed],
// [ErrInterrupted], [ErrSlowConsumer], [ErrNoMatch], [ErrAmbiguousContract].
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

// Package ibkr is a Go client for the Interactive Brokers TWS/Gateway socket
// protocol. It covers the full TWS API surface through typed methods and generic
// subscriptions with explicit lifecycle semantics.
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
// The typical read loop:
//
//	sub, err := client.MarketData().SubscribeQuotes(ctx, req)
//	if err != nil {
//	    return err
//	}
//	defer sub.Close()
//
//	for {
//	    select {
//	    case update, ok := <-sub.Events():
//	        if !ok {
//	            return sub.Wait()
//	        }
//	        // handle business data
//	    case state, ok := <-sub.Lifecycle():
//	        if ok {
//	            // handle lifecycle (SnapshotComplete, Gap, Resumed, etc.)
//	        }
//	    case <-sub.Done():
//	        return sub.Wait()
//	    }
//	}
//
// Call Close to unsubscribe. Wait blocks until termination and returns the
// final error, if any. SubscriptionClosed and Gap lifecycle events include a
// Retryable flag. API errors are terminal request rejections; use [IsRetryable]
// on the final error when a consumer loop only observes Events().
//
// # Order Management
//
// [OrdersClient.Place] submits an order and returns an [*OrderHandle] that tracks
// its full lifecycle. The handle follows the same Events/Lifecycle/Done pattern as
// subscriptions. OrderEvent is a union: exactly one of OpenOrder, Status,
// Execution, or Commission is non-nil per event. The handle auto-closes when
// the order reaches a terminal state (Filled, Cancelled, Inactive). Its
// Lifecycle() channel is also bounded and observational: if unread, older queued
// lifecycle events may be dropped in favor of the latest one.
//
// [OrderHandle.Close] detaches the handle without cancelling the order.
// [OrderHandle.Cancel] sends a cancel request. [OrderHandle.Modify] sends a
// modified order with the same ID.
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
// Three structured error types cover the main failure modes:
//
//   - [*ConnectError] — connection or handshake failure
//   - [*ProtocolError] — wire protocol violation
//   - [*APIError] — server-side rejection (error code + message)
//
// Sentinel errors cover common conditions: [ErrNotReady], [ErrClosed],
// [ErrInterrupted], [ErrSlowConsumer], [ErrNoMatch], [ErrAmbiguousContract].
// [IsRetryable] classifies final subscription errors for retry/backoff policy.
//
// # Financial Types
//
// All prices, quantities, and money values use [github.com/shopspring/decimal.Decimal],
// an exact decimal type that avoids the rounding errors inherent in float64
// arithmetic. Construct values with decimal.NewFromString, decimal.NewFromInt,
// or decimal.RequireFromString.
package ibkr

// Package ibkr is an SDK-backed Go client for Interactive Brokers TWS and IB
// Gateway. The public API is typed and Go-shaped: one-shot methods,
// subscriptions with explicit lifecycle semantics, and order handles.
//
// The SDK-native runtime is still being migrated. Current native coverage
// includes session bootstrap, current time, current time millis, account
// summary, account updates, account updates multi, positions, positions multi,
// PnL, PnL single, family codes, contract details, market-data type control,
// quote snapshots and streams, real-time bars, tick-by-tick data, market depth
// streams, contract search, market rules, sec-def option params, smart
// components, market-depth exchange metadata, head timestamp, histogram data,
// fundamental data, news providers, news bulletins, news articles, historical
// news, scanner parameters, scanner result subscriptions, historical bars,
// historical bar subscriptions, historical schedule, historical ticks, option
// implied-volatility and price calculations, FA config reads and writes,
// soft-dollar tiers, option exercise, user info, WSH metadata/event data,
// display groups, display group subscriptions, order placement and modification,
// open-order snapshots and subscriptions, completed-order snapshots, execution
// snapshots, commission reports, and cancellation for account summary, account
// updates, account updates multi, positions, positions multi, PnL, PnL single,
// order cancellation, global cancellation, news bulletins, scanner
// subscriptions, display group subscriptions, quote streams, real-time bars,
// tick-by-tick data, market depth, historical bars, historical ticks, option
// calculations, head timestamp, histogram data, WSH metadata/event data, and
// fundamental data.
// Rows without current live SDK evidence remain partial in the migration matrix.
//
// # Connecting
//
// [DialContext] establishes a connection and returns a ready [Client]. It blocks
// until the official SDK connection is established, server metadata is loaded,
// and managed accounts are available. Pass functional options to configure the
// connection:
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
// Migrated query methods follow a simple call-and-return pattern. Pass a
// context for cancellation and a typed request; get back typed results:
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
// Migrated one-shots block until the server sends all result callbacks and the
// SDK completion callback. They return [*APIError] when the server rejects the
// request.
//
// # Subscriptions
//
// Migrated streaming data uses [Subscription], a generic type that separates
// business events from lifecycle state. Every subscription exposes three
// channels:
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
//	events := sub.Events()
//	lifecycle := sub.Lifecycle()
//	for events != nil {
//	    select {
//	    case update, ok := <-events:
//	        if !ok {
//	            return sub.Wait()
//	        }
//	        // handle business data
//	    case state, ok := <-lifecycle:
//	        if !ok {
//	            lifecycle = nil
//	            continue
//	        }
//	        // handle lifecycle (SnapshotComplete, Gap, Resumed, etc.)
//	    }
//	}
//
// Call Close to unsubscribe. Wait blocks until termination and returns the
// final error, if any. Err returns the currently recorded terminal error without
// waiting for Done. SubscriptionClosed and Gap lifecycle events include a
// Retryable flag. API errors are terminal request rejections; use [IsRetryable]
// on the final error when a consumer loop only observes Events().
// Done is useful for coordinating other goroutines, but consumers that need
// every business event should drain Events until it closes, then call Wait.
//
// # Order Management
//
// The order-management API is part of the public target surface, but it is not
// yet SDK-native in the current adapter.
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
// Four structured error types cover the main failure modes:
//
//   - [*ConnectError] — SDK adapter creation, connection, or bootstrap failure
//   - [*AdapterError] — SDK adapter boundary failure
//   - [*APIError] — server-side rejection (error code + message)
//   - [*ValidationError] — caller-side request validation failure
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

package ibkr

import (
	"encoding/json"
	"time"
)

// State is the connection lifecycle state of a [Client], reported by
// [Client.Session] and on the [Client.SessionEvents] stream.
type State string

const (
	StateDisconnected State = "Disconnected" // no active connection
	StateConnecting   State = "Connecting"   // TCP dial in progress
	StateHandshaking  State = "Handshaking"  // negotiating protocol version and startup
	StateReady        State = "Ready"        // connected and serving requests
	StateDegraded     State = "Degraded"     // connected but a subsystem is impaired
	StateReconnecting State = "Reconnecting" // connection lost, auto-reconnect in progress
	StateClosed       State = "Closed"       // terminally closed, no further reconnects
)

// OpKind names the request family an [APIError] originated from, so callers can
// attribute a server error code to the operation that produced it. The string
// values are stable identifiers, not IBKR wire tokens.
type OpKind string

// The Op constants enumerate every request family the client can issue; each
// value names the operation it labels.
const (
	OpContractDetails      OpKind = "contract_details"
	OpHistoricalBars       OpKind = "historical_bars"
	OpAccountSummary       OpKind = "account_summary"
	OpPositions            OpKind = "positions"
	OpQuotes               OpKind = "quotes"
	OpRealTimeBars         OpKind = "realtime_bars"
	OpOpenOrders           OpKind = "open_orders"
	OpExecutions           OpKind = "executions"
	OpFamilyCodes          OpKind = "family_codes"
	OpMktDepthExchanges    OpKind = "mkt_depth_exchanges"
	OpNewsProviders        OpKind = "news_providers"
	OpScannerParameters    OpKind = "scanner_parameters"
	OpUserInfo             OpKind = "user_info"
	OpMatchingSymbols      OpKind = "matching_symbols"
	OpHeadTimestamp        OpKind = "head_timestamp"
	OpMarketRule           OpKind = "market_rule"
	OpCompletedOrders      OpKind = "completed_orders"
	OpAccountUpdates       OpKind = "account_updates"
	OpAccountUpdatesMulti  OpKind = "account_updates_multi"
	OpPositionsMulti       OpKind = "positions_multi"
	OpPnL                  OpKind = "pnl"
	OpPnLSingle            OpKind = "pnl_single"
	OpTickByTick           OpKind = "tick_by_tick"
	OpNewsBulletins        OpKind = "news_bulletins"
	OpHistoricalBarsStream OpKind = "historical_bars_stream"
	OpSecDefOptParams      OpKind = "sec_def_opt_params"
	OpSmartComponents      OpKind = "smart_components"
	OpCalcImpliedVol       OpKind = "calc_implied_vol"
	OpCalcOptionPrice      OpKind = "calc_option_price"
	OpHistogramData        OpKind = "histogram_data"
	OpHistoricalTicks      OpKind = "historical_ticks"
	OpNewsArticle          OpKind = "news_article"
	OpHistoricalNews       OpKind = "historical_news"
	OpScannerSubscription  OpKind = "scanner_subscription"
	OpFAConfig             OpKind = "fa_config"
	OpSoftDollarTiers      OpKind = "soft_dollar_tiers"
	OpWSHMetaData          OpKind = "wsh_meta_data"
	OpWSHEventData         OpKind = "wsh_event_data"
	OpDisplayGroups        OpKind = "display_groups"
	OpDisplayGroupEvents   OpKind = "display_group_events"
	OpMarketDepth          OpKind = "market_depth"
	OpExerciseOptions      OpKind = "exercise_options"
	OpPlaceOrder           OpKind = "place_order"
	OpCancelOrder          OpKind = "cancel_order"
	OpGlobalCancel         OpKind = "global_cancel"
	OpHistoricalSchedule   OpKind = "historical_schedule"
	OpCurrentTime          OpKind = "current_time"
	OpOrderID              OpKind = "order_id"
)

// Event is a connection lifecycle transition delivered on [Client.SessionEvents].
type Event struct {
	At            time.Time // when the transition was observed
	State         State     // the state entered
	Previous      State     // the state left
	ConnectionSeq uint64    // generation counter, incremented on each reconnect
	Code          int       // IBKR notification code when the transition carries one, else 0
	Message       string    // human-readable detail, may be empty
	Err           error     // non-nil when the transition was caused by an error
}

// Snapshot is a point-in-time view of the session, returned by [Client.Session].
type Snapshot struct {
	State           State     // current connection state
	ConnectionSeq   uint64    // generation counter, incremented on each reconnect
	ServerVersion   int       // negotiated TWS API server version
	ManagedAccounts []string  // account IDs this login controls
	NextValidID     int64     // next order ID the server has reserved for this client
	CurrentTime     time.Time // server time captured at connect, in UTC
}

// SubscriptionStateKind classifies a lifecycle transition on a subscription's
// or order handle's [SubscriptionStateEvent] channel.
type SubscriptionStateKind string

const (
	SubscriptionStarted          SubscriptionStateKind = "Started"          // stream established
	SubscriptionSnapshotComplete SubscriptionStateKind = "SnapshotComplete" // initial snapshot boundary reached
	SubscriptionGap              SubscriptionStateKind = "Gap"              // connection lost, events may be missing
	SubscriptionResumed          SubscriptionStateKind = "Resumed"          // stream re-established after a gap
	SubscriptionClosed           SubscriptionStateKind = "Closed"           // terminally closed, see Err for the reason
)

// SubscriptionStateEvent reports a lifecycle transition on a subscription or
// order handle, distinct from the business events on the Events channel.
type SubscriptionStateEvent struct {
	At            time.Time             // when the transition was observed, UTC
	Kind          SubscriptionStateKind // which transition occurred
	ConnectionSeq uint64                // connection generation the transition belongs to
	Err           error                 // non-nil on a Closed caused by an error
	// Retryable reports whether the caller can expect this stream to recover on
	// its own (a Gap, or a Closed with a transient error). A Closed with a
	// server rejection or clean shutdown is not retryable.
	Retryable bool
}

// ReconnectPolicy controls whether a [Client] automatically re-dials the
// Gateway after the connection drops. Set with [WithReconnectPolicy].
type ReconnectPolicy string

const (
	ReconnectOff  ReconnectPolicy = "off"  // stay closed after a drop
	ReconnectAuto ReconnectPolicy = "auto" // re-dial and rehandshake automatically
)

func (p ReconnectPolicy) valid() bool {
	return p == ReconnectOff || p == ReconnectAuto
}

// ResumePolicy controls whether a subscription re-establishes itself after a
// reconnect. Set the default with [WithDefaultResumePolicy] or per-subscription
// with [WithResumePolicy].
type ResumePolicy string

const (
	ResumeNever ResumePolicy = "never" // close the subscription on connection loss
	ResumeAuto  ResumePolicy = "auto"  // re-issue the request after reconnect
)

func (p ResumePolicy) valid() bool {
	return p == ResumeNever || p == ResumeAuto
}

// SlowConsumerPolicy controls what happens when a subscriber cannot keep up
// with the event rate and the delivery queue fills. Set the default with
// [WithDefaultSlowConsumerPolicy] or per-subscription with [WithSlowConsumerPolicy].
//
// The default is [SlowConsumerClose]. High-rate streams (market depth,
// tick-by-tick) can outrun a slow consumer and trip this; such consumers should
// choose [SlowConsumerDropOldest] or raise the queue with [WithQueueSize].
type SlowConsumerPolicy string

const (
	SlowConsumerClose      SlowConsumerPolicy = "close"       // fail the subscription with [ErrSlowConsumer]
	SlowConsumerDropOldest SlowConsumerPolicy = "drop_oldest" // evict the oldest queued event to make room
)

func (p SlowConsumerPolicy) valid() bool {
	return p == SlowConsumerClose || p == SlowConsumerDropOldest
}

// XMLDocument is a raw XML payload returned by the Gateway (scanner parameters
// and FA configuration), passed through unparsed.
type XMLDocument []byte

// JSONDocument is a raw JSON payload returned by the Gateway (Wall Street
// Horizon metadata and event data), passed through unparsed.
type JSONDocument = json.RawMessage

package ibkr

import (
	"encoding/json"
	"time"
)

type State string

const (
	StateDisconnected State = "Disconnected"
	StateConnecting   State = "Connecting"
	StateHandshaking  State = "Handshaking"
	StateReady        State = "Ready"
	StateDegraded     State = "Degraded"
	StateReconnecting State = "Reconnecting"
	StateClosed       State = "Closed"
)

type OpKind string

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
	OpFundamentalData      OpKind = "fundamental_data"
	OpExerciseOptions      OpKind = "exercise_options"
	OpPlaceOrder           OpKind = "place_order"
	OpCancelOrder          OpKind = "cancel_order"
	OpGlobalCancel         OpKind = "global_cancel"
	OpHistoricalSchedule   OpKind = "historical_schedule"
	OpCurrentTime          OpKind = "current_time"
)

type Event struct {
	At            time.Time
	State         State
	Previous      State
	ConnectionSeq uint64
	Code          int
	Message       string
	Err           error
}

type Snapshot struct {
	State           State
	ConnectionSeq   uint64
	ServerVersion   int
	ManagedAccounts []string
	NextValidID     int64
	CurrentTime     time.Time
}

type SubscriptionStateKind string

const (
	SubscriptionStarted          SubscriptionStateKind = "Started"
	SubscriptionSnapshotComplete SubscriptionStateKind = "SnapshotComplete"
	SubscriptionGap              SubscriptionStateKind = "Gap"
	SubscriptionResumed          SubscriptionStateKind = "Resumed"
	SubscriptionClosed           SubscriptionStateKind = "Closed"
)

type SubscriptionStateEvent struct {
	At            time.Time
	Kind          SubscriptionStateKind
	ConnectionSeq uint64
	Err           error
	Retryable     bool
}

type ReconnectPolicy string

const (
	ReconnectOff  ReconnectPolicy = "off"
	ReconnectAuto ReconnectPolicy = "auto"
)

type ResumePolicy string

const (
	ResumeNever ResumePolicy = "never"
	ResumeAuto  ResumePolicy = "auto"
)

type SlowConsumerPolicy string

const (
	SlowConsumerClose      SlowConsumerPolicy = "close"
	SlowConsumerDropOldest SlowConsumerPolicy = "drop_oldest"
)

type XMLDocument []byte

type JSONDocument = json.RawMessage

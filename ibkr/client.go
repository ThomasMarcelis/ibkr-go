package ibkr

import (
	"context"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/session"
	"github.com/ThomasMarcelis/ibkr-go/internal/transport"
)

type (
	// Dialer abstracts the TCP connection to TWS/Gateway, allowing custom
	// transports or test harnesses. The default is [*net.Dialer].
	Dialer = transport.Dialer

	// Option configures a [Client] during [DialContext]. Pass functional options
	// like [WithHost], [WithPort], and [WithReconnectPolicy].
	Option = session.Option

	// SubscriptionOption configures a single subscription. Pass options like
	// [WithResumePolicy], [WithSlowConsumerPolicy], and [WithQueueSize].
	SubscriptionOption = session.SubscriptionOption

	// SessionState represents the connection lifecycle. Values progress through
	// [StateConnecting], [StateHandshaking], [StateReady], and optionally
	// [StateDegraded] or [StateReconnecting] on connection loss.
	SessionState = session.State

	// SessionSnapshot is a point-in-time view of the session returned by
	// [Client.Session]. It contains the current state, negotiated server version,
	// managed accounts, and connection sequence number.
	SessionSnapshot = session.Snapshot

	// SessionEvent is delivered through [Client.SessionEvents] when the session
	// state changes. It carries the new and previous state, a connection sequence
	// number, and any associated error.
	SessionEvent = session.Event

	// OpKind identifies an operation type. Used internally for rate limiting and
	// diagnostics; exposed for logging and error context.
	OpKind = session.OpKind

	// ReconnectPolicy controls automatic reconnect behavior after connection loss.
	// Use [ReconnectOff] to disable or [ReconnectAuto] to enable.
	ReconnectPolicy = session.ReconnectPolicy

	// ResumePolicy controls whether a subscription attempts to resume after
	// reconnect. Use [ResumeNever] to require explicit re-subscribe or
	// [ResumeAuto] for automatic resume (supported for quotes and real-time bars).
	ResumePolicy = session.ResumePolicy

	// SlowConsumerPolicy controls what happens when a subscription's event buffer
	// fills. [SlowConsumerClose] terminates the subscription; [SlowConsumerDropOldest]
	// discards the oldest buffered event.
	SlowConsumerPolicy = session.SlowConsumerPolicy

	// SubscriptionStateKind identifies a subscription lifecycle transition.
	// See [SubscriptionStarted], [SubscriptionSnapshotComplete],
	// [SubscriptionGap], [SubscriptionResumed], and [SubscriptionClosed].
	SubscriptionStateKind = session.SubscriptionStateKind

	// SubscriptionStateEvent is delivered through Subscription.State() when
	// the subscription's lifecycle changes. It carries the transition kind,
	// a connection sequence number, and any associated error.
	SubscriptionStateEvent = session.SubscriptionStateEvent

	// Subscription tracks a streaming data feed. Business events arrive on
	// Events(), lifecycle transitions on State(), and Done() closes when the
	// subscription terminates. Call Close to unsubscribe.
	Subscription[T any] = session.Subscription[T]

	// Decimal is an exact decimal type for financial values. All prices,
	// quantities, and money in the API use Decimal instead of float64 to
	// avoid rounding errors. Parse from strings with [ParseDecimal].
	Decimal = session.Decimal

	// Contract identifies a financial instrument. At minimum, provide Symbol,
	// SecType, Exchange, and Currency. Use [Client.QualifyContract] to resolve
	// an ambiguous contract to a single match.
	Contract = session.Contract

	// ComboLeg describes one leg of a combo order or observed combo structure.
	ComboLeg = session.ComboLeg

	// TagValue carries IBKR tag/value parameter pairs for algo and routing options.
	TagValue = session.TagValue

	// OrderCondition describes one server-side order condition.
	OrderCondition = session.OrderCondition

	// ContractDetailsRequest holds parameters for [Client.ContractDetails].
	ContractDetailsRequest = session.ContractDetailsRequest

	// ContractDetails holds metadata for a single contract: market name, long
	// name, minimum tick, time zone, and the resolved [Contract].
	ContractDetails = session.ContractDetails

	// QualifiedContract is the result of [Client.QualifyContract]. It wraps a
	// fully resolved [ContractDetails] for a single unambiguous match.
	QualifiedContract = session.QualifiedContract

	// HistoricalBarsRequest holds parameters for [Client.HistoricalBars] and
	// [Client.SubscribeHistoricalBars]: contract, time range, bar size, and
	// data type (TRADES, MIDPOINT, BID, ASK, etc.).
	HistoricalBarsRequest = session.HistoricalBarsRequest

	// Bar is a single OHLCV bar with exact [Decimal] values for Open, High,
	// Low, Close, Volume, and WAP.
	Bar = session.Bar

	// AccountSummaryRequest holds parameters for [Client.AccountSummary] and
	// [Client.SubscribeAccountSummary]. Specify an account and the tags to
	// retrieve (e.g., "NetLiquidation", "TotalCashValue").
	AccountSummaryRequest = session.AccountSummaryRequest

	// AccountValue is a single account summary tag value returned by
	// [Client.AccountSummary].
	AccountValue = session.AccountValue

	// AccountSummaryUpdate wraps an [AccountValue] delivered through a
	// streaming account summary subscription.
	AccountSummaryUpdate = session.AccountSummaryUpdate

	// Position holds a single portfolio position: account, contract, quantity,
	// and average cost.
	Position = session.Position

	// PositionUpdate wraps a [Position] delivered through a streaming position
	// subscription.
	PositionUpdate = session.PositionUpdate

	// QuoteFields is a bitmask indicating which fields are populated in a
	// [Quote]. Test with bitwise AND against [QuoteFieldBid], [QuoteFieldAsk], etc.
	QuoteFields = session.QuoteFields

	// MarketDataType identifies the data class: live, frozen, delayed, or
	// delayed-frozen. Set via [Client.SetMarketDataType].
	MarketDataType = session.MarketDataType

	// Quote holds a market data snapshot with bid, ask, last, sizes, and
	// OHLC fields. The Available bitmask indicates which fields are populated.
	Quote = session.Quote

	// QuoteSubscriptionRequest holds parameters for [Client.QuoteSnapshot] and
	// [Client.SubscribeQuotes]: the target contract and optional generic tick types.
	QuoteSubscriptionRequest = session.QuoteSubscriptionRequest

	// QuoteUpdate is delivered through a streaming quote subscription. It
	// contains the full [Quote] snapshot, a bitmask of Changed fields, and
	// the receive timestamp.
	QuoteUpdate = session.QuoteUpdate

	// RealTimeBarsRequest holds parameters for [Client.SubscribeRealTimeBars]:
	// contract, data type (TRADES, MIDPOINT, BID, ASK), and RTH flag.
	RealTimeBarsRequest = session.RealTimeBarsRequest

	// OpenOrdersScope controls which orders are returned by
	// [Client.OpenOrdersSnapshot] and [Client.SubscribeOpenOrders].
	// See [OpenOrdersScopeAll], [OpenOrdersScopeClient], [OpenOrdersScopeAuto].
	OpenOrdersScope = session.OpenOrdersScope

	// OpenOrder holds a single open order's state: order ID, account, contract,
	// action, type, status, fill quantities, and auxiliary fields.
	OpenOrder = session.OpenOrder

	// OpenOrderUpdate wraps an [OpenOrder] delivered through a streaming open
	// orders subscription.
	OpenOrderUpdate = session.OpenOrderUpdate

	// ExecutionsRequest holds filters for [Client.Executions] and
	// [Client.SubscribeExecutions]: optional account and symbol filters.
	ExecutionsRequest = session.ExecutionsRequest

	// Execution holds a single trade execution: order ID, exec ID, account,
	// symbol, side, shares, price, and time.
	Execution = session.Execution

	// ExecutionUpdate is delivered by [Client.Executions] and
	// [Client.SubscribeExecutions]. It is a union: Execution and Commission
	// may arrive as separate events for the same trade.
	ExecutionUpdate = session.ExecutionUpdate

	// CommissionReport holds commission details for a trade execution,
	// including the exec ID, commission amount, currency, and realized PnL.
	CommissionReport = session.CommissionReport

	// FamilyCode represents a linked account family membership returned by
	// [Client.FamilyCodes].
	FamilyCode = session.FamilyCode

	// DepthExchange holds exchange metadata for Level 2 market depth
	// availability, returned by [Client.MktDepthExchanges].
	DepthExchange = session.DepthExchange

	// NewsProvider identifies a news source returned by [Client.NewsProviders].
	NewsProvider = session.NewsProvider

	// MatchingSymbolsRequest holds parameters for [Client.MatchingSymbols]:
	// a symbol search pattern.
	MatchingSymbolsRequest = session.MatchingSymbolsRequest

	// MatchingSymbol is a symbol search result returned by [Client.MatchingSymbols],
	// containing the contract ID, symbol, security type, and available derivative types.
	MatchingSymbol = session.MatchingSymbol

	// HeadTimestampRequest holds parameters for [Client.HeadTimestamp]: contract,
	// data type, and RTH flag.
	HeadTimestampRequest = session.HeadTimestampRequest

	// PriceIncrement defines a price step at a given price level, part of a
	// [MarketRuleResult].
	PriceIncrement = session.PriceIncrement

	// MarketRuleResult holds the price increment table for a market rule ID,
	// returned by [Client.MarketRule].
	MarketRuleResult = session.MarketRuleResult

	// CompletedOrderResult holds a filled or cancelled order returned by
	// [Client.CompletedOrders].
	CompletedOrderResult = session.CompletedOrderResult

	// AccountUpdateValue is a single key/value/currency entry from a streaming
	// account updates subscription.
	AccountUpdateValue = session.AccountUpdateValue

	// PortfolioUpdate holds per-holding market value and PnL data from a
	// streaming account updates subscription.
	PortfolioUpdate = session.PortfolioUpdate

	// AccountUpdate is delivered by [Client.SubscribeAccountUpdates]. It is a
	// union: exactly one of AccountValue or Portfolio is non-nil per event.
	AccountUpdate = session.AccountUpdate

	// AccountUpdatesMultiRequest holds parameters for
	// [Client.AccountUpdatesMultiSnapshot] and [Client.SubscribeAccountUpdatesMulti]:
	// account and model code filters.
	AccountUpdatesMultiRequest = session.AccountUpdatesMultiRequest

	// AccountUpdateMultiValue is a single key/value entry from a multi-account
	// updates subscription.
	AccountUpdateMultiValue = session.AccountUpdateMultiValue

	// PositionsMultiRequest holds parameters for [Client.PositionsMultiSnapshot]
	// and [Client.SubscribePositionsMulti]: account and model code filters.
	PositionsMultiRequest = session.PositionsMultiRequest

	// PositionMulti holds a single position filtered by account or model,
	// returned by multi-position methods.
	PositionMulti = session.PositionMulti

	// PnLRequest holds parameters for [Client.SubscribePnL]: account and
	// optional model code.
	PnLRequest = session.PnLRequest

	// PnLUpdate holds real-time daily, unrealized, and realized PnL for an
	// account, delivered through a PnL subscription.
	PnLUpdate = session.PnLUpdate

	// PnLSingleRequest holds parameters for [Client.SubscribePnLSingle]:
	// account, optional model code, and contract ID.
	PnLSingleRequest = session.PnLSingleRequest

	// PnLSingleUpdate holds real-time PnL for a single position, delivered
	// through a single-position PnL subscription.
	PnLSingleUpdate = session.PnLSingleUpdate

	// TickByTickRequest holds parameters for [Client.SubscribeTickByTick]:
	// contract, tick type ("Last", "AllLast", "BidAsk", "MidPoint"),
	// number of ticks, and whether to ignore size.
	TickByTickRequest = session.TickByTickRequest

	// TickByTickData holds a single tick event. Fields are populated based
	// on the requested tick type: Price/Size for Last/AllLast, BidPrice/AskPrice
	// for BidAsk, MidPoint for MidPoint.
	TickByTickData = session.TickByTickData

	// NewsBulletin holds a system bulletin from IB, delivered through a
	// news bulletin subscription.
	NewsBulletin = session.NewsBulletin

	// SecDefOptParamsRequest holds parameters for [Client.SecDefOptParams]:
	// underlying symbol, exchange, security type, and contract ID.
	SecDefOptParamsRequest = session.SecDefOptParamsRequest

	// SecDefOptParams holds option chain parameters for a single exchange:
	// trading class, multiplier, available expirations, and strikes.
	SecDefOptParams = session.SecDefOptParams

	// SmartComponent maps a bit number to an exchange name and letter for
	// SMART routing, returned by [Client.SmartComponents].
	SmartComponent = session.SmartComponent

	// CalcImpliedVolatilityRequest holds parameters for
	// [Client.CalcImpliedVolatility]: contract, option price, and underlying price.
	CalcImpliedVolatilityRequest = session.CalcImpliedVolatilityRequest

	// CalcOptionPriceRequest holds parameters for [Client.CalcOptionPrice]:
	// contract, volatility, and underlying price.
	CalcOptionPriceRequest = session.CalcOptionPriceRequest

	// OptionComputation holds the result of [Client.CalcImpliedVolatility] or
	// [Client.CalcOptionPrice]: implied vol, delta, gamma, vega, theta, and
	// theoretical price.
	OptionComputation = session.OptionComputation

	// HistogramDataRequest holds parameters for [Client.HistogramData]:
	// contract, RTH flag, and time period.
	HistogramDataRequest = session.HistogramDataRequest

	// HistogramEntry is a single price/size bucket in a histogram, returned
	// by [Client.HistogramData].
	HistogramEntry = session.HistogramEntry

	// HistoricalTicksRequest holds parameters for [Client.HistoricalTicks]:
	// contract, time range, tick count, data type, RTH flag, and size flag.
	HistoricalTicksRequest = session.HistoricalTicksRequest

	// HistoricalTick holds a single midpoint tick from a historical ticks query.
	HistoricalTick = session.HistoricalTick

	// HistoricalTickBidAsk holds a single bid/ask tick from a historical ticks query.
	HistoricalTickBidAsk = session.HistoricalTickBidAsk

	// HistoricalTickLast holds a single last-trade tick from a historical ticks query.
	HistoricalTickLast = session.HistoricalTickLast

	// HistoricalTicksResult holds the result of [Client.HistoricalTicks].
	// Exactly one of Ticks, BidAsk, or Last is populated based on WhatToShow.
	HistoricalTicksResult = session.HistoricalTicksResult

	// NewsArticleRequest holds parameters for [Client.NewsArticle]: provider
	// code and article ID.
	NewsArticleRequest = session.NewsArticleRequest

	// NewsArticleResult holds the body of a news article returned by
	// [Client.NewsArticle].
	NewsArticleResult = session.NewsArticle

	// HistoricalNewsRequest holds parameters for [Client.HistoricalNews]:
	// contract ID, provider codes, date range, and result limit.
	HistoricalNewsRequest = session.HistoricalNewsRequest

	// HistoricalNewsItemResult holds a single historical news headline returned
	// by [Client.HistoricalNews].
	HistoricalNewsItemResult = session.HistoricalNewsItem

	// ScannerSubscriptionRequest holds parameters for
	// [Client.SubscribeScannerResults]: row count, instrument type, location,
	// and scan code.
	ScannerSubscriptionRequest = session.ScannerSubscriptionRequest

	// ScannerResult is a single scanner hit: rank, contract, and metadata.
	ScannerResult = session.ScannerResult

	// SoftDollarTier holds a soft dollar tier returned by [Client.SoftDollarTiers].
	SoftDollarTier = session.SoftDollarTier

	// WSHEventDataRequest holds parameters for [Client.WSHEventData]: contract ID,
	// filter criteria, and date range.
	WSHEventDataRequest = session.WSHEventDataRequest

	// DisplayGroupUpdate holds a contract info string delivered through a
	// display group subscription.
	DisplayGroupUpdate = session.DisplayGroupUpdate

	// DisplayGroupHandle wraps a display group subscription and exposes an
	// Update method to push contract selections to the TWS display group.
	DisplayGroupHandle = session.DisplayGroupHandle

	// MarketDepthRequest holds parameters for [Client.SubscribeMarketDepth]:
	// contract, number of rows, and smart depth flag.
	MarketDepthRequest = session.MarketDepthRequest

	// DepthRow holds a single order book row from a market depth subscription:
	// position, side, price, size, and operation (insert/update/delete).
	DepthRow = session.DepthRow

	// FundamentalDataRequest holds parameters for [Client.FundamentalData]:
	// contract and report type (e.g., "ReportsFinSummary", "ReportsOwnership").
	FundamentalDataRequest = session.FundamentalDataRequest

	// ExerciseOptionsRequest holds parameters for [Client.ExerciseOptions]:
	// contract, exercise action, quantity, account, and override flag.
	ExerciseOptionsRequest = session.ExerciseOptionsRequest

	// ConnectError indicates a connection or handshake failure. Unwrap to
	// get the underlying network error.
	ConnectError = session.ConnectError

	// ProtocolError indicates a wire protocol violation during message
	// encoding or decoding.
	ProtocolError = session.ProtocolError

	// APIError is returned when the IBKR server rejects a request. It carries
	// the error code, message, operation kind, and connection sequence number.
	APIError = session.APIError

	// OrderAction specifies the side of an order: [Buy] or [Sell].
	OrderAction = session.OrderAction

	// TimeInForce specifies how long an order remains active. See [TIFDay],
	// [TIFGTC], [TIFIOC], and other TIF constants.
	TimeInForce = session.TimeInForce

	// Order holds the parameters for a new or modified order: action, type,
	// quantity, limit price, TIF, and auxiliary fields. Set OrderID to 0 for
	// auto-allocation from NextValidID.
	Order = session.Order

	// PlaceOrderRequest holds parameters for [Client.PlaceOrder]: the target
	// contract and the order to submit.
	PlaceOrderRequest = session.PlaceOrderRequest

	// OrderEvent is delivered through [OrderHandle.Events]. It is a union:
	// exactly one of OpenOrder, Status, Execution, or Commission is non-nil.
	OrderEvent = session.OrderEvent

	// OrderStatusUpdate holds fill progress for an order: status string,
	// filled/remaining quantities, average fill price, and related IDs.
	OrderStatusUpdate = session.OrderStatusUpdate

	// OrderHandle tracks a placed order's lifecycle. Events arrive via Events();
	// lifecycle state changes (Gap, Resumed) arrive via State(). Close detaches
	// the handle without cancelling the order. Cancel sends a cancel request.
	// The handle auto-closes when the order reaches a terminal state.
	OrderHandle = session.OrderHandle
)

// Session states. See [SessionState].
const (
	StateDisconnected = session.StateDisconnected // Not connected.
	StateConnecting   = session.StateConnecting   // TCP dial in progress.
	StateHandshaking  = session.StateHandshaking  // Protocol handshake in progress.
	StateReady        = session.StateReady        // Session ready for requests.
	StateDegraded     = session.StateDegraded     // Connected but experiencing issues.
	StateReconnecting = session.StateReconnecting // Reconnect in progress after connection loss.
	StateClosed       = session.StateClosed       // Session permanently closed.

	// Reconnect policies for [WithReconnectPolicy].
	ReconnectOff  = session.ReconnectOff  // Do not reconnect after connection loss.
	ReconnectAuto = session.ReconnectAuto // Automatically reconnect with backoff. Default.

	// Operation kinds. Used in [APIError] and for rate-limiting diagnostics.
	OpContractDetails      = session.OpContractDetails
	OpHistoricalBars       = session.OpHistoricalBars
	OpAccountSummary       = session.OpAccountSummary
	OpPositions            = session.OpPositions
	OpQuotes               = session.OpQuotes
	OpRealTimeBars         = session.OpRealTimeBars
	OpOpenOrders           = session.OpOpenOrders
	OpExecutions           = session.OpExecutions
	OpFamilyCodes          = session.OpFamilyCodes
	OpMktDepthExchanges    = session.OpMktDepthExchanges
	OpNewsProviders        = session.OpNewsProviders
	OpScannerParameters    = session.OpScannerParameters
	OpUserInfo             = session.OpUserInfo
	OpMatchingSymbols      = session.OpMatchingSymbols
	OpHeadTimestamp        = session.OpHeadTimestamp
	OpMarketRule           = session.OpMarketRule
	OpCompletedOrders      = session.OpCompletedOrders
	OpAccountUpdates       = session.OpAccountUpdates
	OpAccountUpdatesMulti  = session.OpAccountUpdatesMulti
	OpPositionsMulti       = session.OpPositionsMulti
	OpPnL                  = session.OpPnL
	OpPnLSingle            = session.OpPnLSingle
	OpTickByTick           = session.OpTickByTick
	OpNewsBulletins        = session.OpNewsBulletins
	OpHistoricalBarsStream = session.OpHistoricalBarsStream
	OpSecDefOptParams      = session.OpSecDefOptParams
	OpSmartComponents      = session.OpSmartComponents
	OpCalcImpliedVol       = session.OpCalcImpliedVol
	OpCalcOptionPrice      = session.OpCalcOptionPrice
	OpHistogramData        = session.OpHistogramData
	OpHistoricalTicks      = session.OpHistoricalTicks
	OpNewsArticle          = session.OpNewsArticle
	OpHistoricalNews       = session.OpHistoricalNews
	OpScannerSubscription  = session.OpScannerSubscription
	OpFAConfig             = session.OpFAConfig
	OpSoftDollarTiers      = session.OpSoftDollarTiers
	OpWSHMetaData          = session.OpWSHMetaData
	OpWSHEventData         = session.OpWSHEventData
	OpDisplayGroups        = session.OpDisplayGroups
	OpDisplayGroupEvents   = session.OpDisplayGroupEvents
	OpMarketDepth          = session.OpMarketDepth
	OpFundamentalData      = session.OpFundamentalData
	OpExerciseOptions      = session.OpExerciseOptions
	OpPlaceOrder           = session.OpPlaceOrder
	OpCancelOrder          = session.OpCancelOrder
	OpGlobalCancel         = session.OpGlobalCancel

	// Resume policies for [WithResumePolicy] and [WithDefaultResumePolicy].
	ResumeNever = session.ResumeNever // Do not resume after reconnect. Default.
	ResumeAuto  = session.ResumeAuto  // Resume automatically (quotes, real-time bars).

	// Slow consumer policies for [WithSlowConsumerPolicy].
	SlowConsumerClose      = session.SlowConsumerClose      // Close the subscription. Default.
	SlowConsumerDropOldest = session.SlowConsumerDropOldest // Drop the oldest buffered event.

	// Subscription lifecycle kinds. See [SubscriptionStateEvent].
	SubscriptionStarted          = session.SubscriptionStarted          // Subscription registered with the server.
	SubscriptionSnapshotComplete = session.SubscriptionSnapshotComplete // Initial snapshot boundary reached.
	SubscriptionGap              = session.SubscriptionGap              // Data gap due to connection loss.
	SubscriptionResumed          = session.SubscriptionResumed          // Subscription resumed after reconnect.
	SubscriptionClosed           = session.SubscriptionClosed           // Subscription terminated.

	// Open order scopes for [Client.OpenOrdersSnapshot] and [Client.SubscribeOpenOrders].
	OpenOrdersScopeAll    = session.OpenOrdersScopeAll    // All orders across all clients.
	OpenOrdersScopeClient = session.OpenOrdersScopeClient // Orders from this client ID only.
	OpenOrdersScopeAuto   = session.OpenOrdersScopeAuto   // Auto-bind open orders to this client.

	// Quote field bitmask values for [QuoteFields].
	QuoteFieldBid            = session.QuoteFieldBid
	QuoteFieldAsk            = session.QuoteFieldAsk
	QuoteFieldLast           = session.QuoteFieldLast
	QuoteFieldBidSize        = session.QuoteFieldBidSize
	QuoteFieldAskSize        = session.QuoteFieldAskSize
	QuoteFieldLastSize       = session.QuoteFieldLastSize
	QuoteFieldOpen           = session.QuoteFieldOpen
	QuoteFieldHigh           = session.QuoteFieldHigh
	QuoteFieldLow            = session.QuoteFieldLow
	QuoteFieldClose          = session.QuoteFieldClose
	QuoteFieldMarketDataType = session.QuoteFieldMarketDataType

	// Order actions for [Order.Action].
	Buy  = session.Buy  // Buy order.
	Sell = session.Sell // Sell order.

	// Time-in-force values for [Order.TIF].
	TIFDay = session.TIFDay // Day order. Expires at end of trading day.
	TIFGTC = session.TIFGTC // Good till cancelled.
	TIFIOC = session.TIFIOC // Immediate or cancel.
	TIFGTD = session.TIFGTD // Good till date.
	TIFOPG = session.TIFOPG // Market on open.
	TIFFOK = session.TIFFOK // Fill or kill.
	TIFDTC = session.TIFDTC // Day till cancelled.
)

var (
	// ErrInvalidDecimal is returned by [ParseDecimal] for malformed input.
	ErrInvalidDecimal = session.ErrInvalidDecimal

	// ErrNotReady is returned when a method is called before the session
	// reaches [StateReady].
	ErrNotReady = session.ErrNotReady

	// ErrInterrupted is returned when a pending request is interrupted by
	// connection loss or session close.
	ErrInterrupted = session.ErrInterrupted

	// ErrResumeRequired is returned when a subscription cannot be automatically
	// resumed after reconnect and requires explicit re-subscribe.
	ErrResumeRequired = session.ErrResumeRequired

	// ErrSlowConsumer is returned when a subscription is closed because the
	// consumer is not reading events fast enough.
	ErrSlowConsumer = session.ErrSlowConsumer

	// ErrUnsupportedServerVersion is returned when the TWS/Gateway server
	// version is below the minimum required for the requested operation.
	ErrUnsupportedServerVersion = session.ErrUnsupportedServerVersion

	// ErrClosed is returned when a method is called on a closed session.
	ErrClosed = session.ErrClosed

	// ErrNoMatch is returned by [Client.QualifyContract] when no contract
	// matches the given criteria.
	ErrNoMatch = session.ErrNoMatch

	// ErrAmbiguousContract is returned by [Client.QualifyContract] when
	// multiple contracts match the given criteria.
	ErrAmbiguousContract = session.ErrAmbiguousContract
)

var (
	// ParseDecimal parses a decimal string into a [Decimal] value.
	// Returns [ErrInvalidDecimal] on malformed input.
	ParseDecimal = session.ParseDecimal

	// MustParseDecimal parses a decimal string into a [Decimal] value,
	// panicking on malformed input. Use only for known-valid literals.
	MustParseDecimal = session.MustParseDecimal

	// IsTerminalOrderStatus reports whether a status string represents a
	// final order state (Filled, Cancelled, Inactive) after which no further
	// updates arrive.
	IsTerminalOrderStatus = session.IsTerminalOrderStatus

	// WithHost sets the TWS/Gateway hostname. Default: "127.0.0.1".
	WithHost = session.WithHost

	// WithPort sets the TWS/Gateway port. Default: 7497.
	WithPort = session.WithPort

	// WithClientID sets the API client ID. Default: 1. Use 0 to receive
	// order updates for all clients.
	WithClientID = session.WithClientID

	// WithLogger sets a structured logger for session diagnostics. Default:
	// silent (no output).
	WithLogger = session.WithLogger

	// WithReconnectPolicy sets the reconnect policy. Default: [ReconnectAuto].
	WithReconnectPolicy = session.WithReconnectPolicy

	// WithSendRate sets the maximum outbound messages per second. Default: 50.
	WithSendRate = session.WithSendRate

	// WithEventBuffer sets the session event channel buffer size. Default: 64.
	WithEventBuffer = session.WithEventBuffer

	// WithSubscriptionBuffer sets the default event buffer size for new
	// subscriptions. Default: 64. Override per-subscription with [WithQueueSize].
	WithSubscriptionBuffer = session.WithSubscriptionBuffer

	// WithDefaultResumePolicy sets the default resume policy for new
	// subscriptions. Default: [ResumeNever].
	WithDefaultResumePolicy = session.WithDefaultResumePolicy

	// WithDefaultSlowConsumerPolicy sets the default slow consumer policy for
	// new subscriptions. Default: [SlowConsumerClose].
	WithDefaultSlowConsumerPolicy = session.WithDefaultSlowConsumerPolicy

	// WithResumePolicy sets the resume policy for a single subscription.
	// Overrides the session default set by [WithDefaultResumePolicy].
	WithResumePolicy = session.WithResumePolicy

	// WithSlowConsumerPolicy sets the slow consumer policy for a single
	// subscription. Overrides the session default.
	WithSlowConsumerPolicy = session.WithSlowConsumerPolicy

	// WithQueueSize sets the event buffer size for a single subscription.
	// Overrides the session default set by [WithSubscriptionBuffer].
	WithQueueSize = session.WithQueueSize
)

// WithDialer sets a custom [Dialer] for the TCP connection to TWS/Gateway.
// The default is [*net.Dialer].
func WithDialer(dialer Dialer) Option {
	return session.WithDialer(dialer)
}

// Client is a connection to an Interactive Brokers TWS or IB Gateway instance.
// It is safe for concurrent use by multiple goroutines. Create one with
// [DialContext] and close it with [Client.Close] when done.
type Client struct {
	engine *session.Engine
}

// DialContext connects to TWS/Gateway and returns a ready [Client]. It blocks
// until the handshake completes, the server version is negotiated, and managed
// accounts are loaded. The context controls only the connection phase; once
// DialContext returns, the session is independent of context cancellation.
func DialContext(ctx context.Context, opts ...Option) (*Client, error) {
	engine, err := session.DialContext(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{engine: engine}, nil
}

// Close shuts down the session and releases all resources. Active subscriptions
// and order handles are terminated.
func (c *Client) Close() error { return c.engine.Close() }

// Done returns a channel that is closed when the session terminates, whether
// from an explicit Close call, an unrecoverable error, or reconnect failure.
func (c *Client) Done() <-chan struct{} { return c.engine.Done() }

// Wait blocks until the session terminates and returns the final error, if any.
func (c *Client) Wait() error { return c.engine.Wait() }

// Session returns a point-in-time snapshot of the session state, including
// managed accounts, negotiated server version, and connection sequence number.
func (c *Client) Session() SessionSnapshot { return c.engine.Session() }

// SessionEvents returns a channel that delivers [SessionEvent] values when the
// session state changes (Ready, Degraded, Reconnecting, Closed, etc.).
func (c *Client) SessionEvents() <-chan SessionEvent { return c.engine.SessionEvents() }

// ContractDetails queries IBKR for full contract metadata matching the given
// request. Returns all matching contracts. Returns [*APIError] if the server
// rejects the query.
func (c *Client) ContractDetails(ctx context.Context, req ContractDetailsRequest) ([]ContractDetails, error) {
	return c.engine.ContractDetails(ctx, req)
}

// QualifyContract resolves an ambiguous [Contract] to a single match. Returns
// [ErrNoMatch] if no contract matches or [ErrAmbiguousContract] if multiple
// contracts match.
func (c *Client) QualifyContract(ctx context.Context, contract Contract) (QualifiedContract, error) {
	return c.engine.QualifyContract(ctx, contract)
}

// HistoricalBars queries IBKR for historical OHLCV bars matching the given
// request parameters.
func (c *Client) HistoricalBars(ctx context.Context, req HistoricalBarsRequest) ([]Bar, error) {
	return c.engine.HistoricalBars(ctx, req)
}

// AccountSummary queries account tag values for the given account and tags.
// Returns all matching values as a snapshot.
func (c *Client) AccountSummary(ctx context.Context, req AccountSummaryRequest) ([]AccountValue, error) {
	return c.engine.AccountSummary(ctx, req)
}

// SubscribeAccountSummary starts a streaming subscription for account summary
// values. The subscription delivers an initial snapshot followed by updates.
func (c *Client) SubscribeAccountSummary(ctx context.Context, req AccountSummaryRequest, opts ...SubscriptionOption) (*Subscription[AccountSummaryUpdate], error) {
	return c.engine.SubscribeAccountSummary(ctx, req, opts...)
}

// PositionsSnapshot queries all current positions across all accounts.
func (c *Client) PositionsSnapshot(ctx context.Context) ([]Position, error) {
	return c.engine.PositionsSnapshot(ctx)
}

// SubscribePositions starts a streaming subscription for position updates.
// The subscription delivers an initial snapshot followed by real-time changes.
func (c *Client) SubscribePositions(ctx context.Context, opts ...SubscriptionOption) (*Subscription[PositionUpdate], error) {
	return c.engine.SubscribePositions(ctx, opts...)
}

// SetMarketDataType switches the market data class for subsequent quote
// subscriptions. Pass 1 for live, 2 for frozen, 3 for delayed, 4 for
// delayed-frozen.
func (c *Client) SetMarketDataType(dataType int) error {
	return c.engine.SetMarketDataType(dataType)
}

// QuoteSnapshot requests a single market data snapshot for the given contract.
// The snapshot completes when all initial tick fields arrive.
func (c *Client) QuoteSnapshot(ctx context.Context, req QuoteSubscriptionRequest) (Quote, error) {
	return c.engine.QuoteSnapshot(ctx, req)
}

// SubscribeQuotes starts a streaming market data subscription. Events carry
// [QuoteUpdate] values with the full snapshot and a bitmask of changed fields.
func (c *Client) SubscribeQuotes(ctx context.Context, req QuoteSubscriptionRequest, opts ...SubscriptionOption) (*Subscription[QuoteUpdate], error) {
	return c.engine.SubscribeQuotes(ctx, req, opts...)
}

// SubscribeRealTimeBars starts a streaming 5-second bar subscription for the
// given contract. Each bar arrives as a [Bar] event.
func (c *Client) SubscribeRealTimeBars(ctx context.Context, req RealTimeBarsRequest, opts ...SubscriptionOption) (*Subscription[Bar], error) {
	return c.engine.SubscribeRealTimeBars(ctx, req, opts...)
}

// SubscribeMarketDepth starts a Level 2 order book subscription. Each event
// is a [DepthRow] representing an insert, update, or delete at a book position.
// Requires a paid L2 market data subscription.
func (c *Client) SubscribeMarketDepth(ctx context.Context, req MarketDepthRequest, opts ...SubscriptionOption) (*Subscription[DepthRow], error) {
	return c.engine.SubscribeMarketDepth(ctx, req, opts...)
}

// OpenOrdersSnapshot queries all open orders matching the given scope and
// returns them as a snapshot.
func (c *Client) OpenOrdersSnapshot(ctx context.Context, scope OpenOrdersScope) ([]OpenOrder, error) {
	return c.engine.OpenOrdersSnapshot(ctx, scope)
}

// SubscribeOpenOrders starts a streaming subscription for open order updates
// matching the given scope.
func (c *Client) SubscribeOpenOrders(ctx context.Context, scope OpenOrdersScope, opts ...SubscriptionOption) (*Subscription[OpenOrderUpdate], error) {
	return c.engine.SubscribeOpenOrders(ctx, scope, opts...)
}

// Executions queries recent trade executions matching the given filters.
func (c *Client) Executions(ctx context.Context, req ExecutionsRequest) ([]ExecutionUpdate, error) {
	return c.engine.Executions(ctx, req)
}

// SubscribeExecutions starts a streaming subscription for execution reports.
// This is a finite snapshot flow: after SnapshotComplete, it closes with nil.
func (c *Client) SubscribeExecutions(ctx context.Context, req ExecutionsRequest, opts ...SubscriptionOption) (*Subscription[ExecutionUpdate], error) {
	return c.engine.SubscribeExecutions(ctx, req, opts...)
}

// FamilyCodes queries account family membership codes.
func (c *Client) FamilyCodes(ctx context.Context) ([]FamilyCode, error) {
	return c.engine.FamilyCodes(ctx)
}

// MktDepthExchanges queries exchange metadata for Level 2 market depth
// availability.
func (c *Client) MktDepthExchanges(ctx context.Context) ([]DepthExchange, error) {
	return c.engine.MktDepthExchanges(ctx)
}

// NewsProviders queries available news provider sources.
func (c *Client) NewsProviders(ctx context.Context) ([]NewsProvider, error) {
	return c.engine.NewsProviders(ctx)
}

// ScannerParameters queries the XML definition of all available scanner
// filters and parameters.
func (c *Client) ScannerParameters(ctx context.Context) (string, error) {
	return c.engine.ScannerParameters(ctx)
}

// UserInfo queries the white branding ID for the current session.
func (c *Client) UserInfo(ctx context.Context) (string, error) {
	return c.engine.UserInfo(ctx)
}

// MatchingSymbols searches for contracts matching a symbol pattern. Returns
// up to 16 results.
func (c *Client) MatchingSymbols(ctx context.Context, req MatchingSymbolsRequest) ([]MatchingSymbol, error) {
	return c.engine.MatchingSymbols(ctx, req)
}

// HeadTimestamp queries the earliest available data point for a contract.
func (c *Client) HeadTimestamp(ctx context.Context, req HeadTimestampRequest) (time.Time, error) {
	return c.engine.HeadTimestamp(ctx, req)
}

// MarketRule queries the price increment table for a market rule ID.
func (c *Client) MarketRule(ctx context.Context, marketRuleID int) (MarketRuleResult, error) {
	return c.engine.MarketRule(ctx, marketRuleID)
}

// CompletedOrders queries all filled or cancelled orders. Set apiOnly to true
// to restrict to API-placed orders.
func (c *Client) CompletedOrders(ctx context.Context, apiOnly bool) ([]CompletedOrderResult, error) {
	return c.engine.CompletedOrders(ctx, apiOnly)
}

// AccountUpdatesSnapshot queries per-holding portfolio values and account
// key/value pairs for the given account as a snapshot.
func (c *Client) AccountUpdatesSnapshot(ctx context.Context, account string) ([]AccountUpdate, error) {
	return c.engine.AccountUpdatesSnapshot(ctx, account)
}

// SubscribeAccountUpdates starts a streaming subscription for per-holding
// portfolio values and account key/value pairs. Events are [AccountUpdate]
// unions: exactly one of AccountValue or Portfolio is non-nil per event.
func (c *Client) SubscribeAccountUpdates(ctx context.Context, account string, opts ...SubscriptionOption) (*Subscription[AccountUpdate], error) {
	return c.engine.SubscribeAccountUpdates(ctx, account, opts...)
}

// AccountUpdatesMultiSnapshot queries account key/value pairs for the given
// account and model code as a snapshot.
func (c *Client) AccountUpdatesMultiSnapshot(ctx context.Context, req AccountUpdatesMultiRequest) ([]AccountUpdateMultiValue, error) {
	return c.engine.AccountUpdatesMultiSnapshot(ctx, req)
}

// SubscribeAccountUpdatesMulti starts a streaming multi-account subscription
// for account key/value pairs.
func (c *Client) SubscribeAccountUpdatesMulti(ctx context.Context, req AccountUpdatesMultiRequest, opts ...SubscriptionOption) (*Subscription[AccountUpdateMultiValue], error) {
	return c.engine.SubscribeAccountUpdatesMulti(ctx, req, opts...)
}

// PositionsMultiSnapshot queries positions filtered by account and model code
// as a snapshot.
func (c *Client) PositionsMultiSnapshot(ctx context.Context, req PositionsMultiRequest) ([]PositionMulti, error) {
	return c.engine.PositionsMultiSnapshot(ctx, req)
}

// SubscribePositionsMulti starts a streaming subscription for positions
// filtered by account and model code.
func (c *Client) SubscribePositionsMulti(ctx context.Context, req PositionsMultiRequest, opts ...SubscriptionOption) (*Subscription[PositionMulti], error) {
	return c.engine.SubscribePositionsMulti(ctx, req, opts...)
}

// SubscribePnL starts a real-time PnL subscription for an account. Delivers
// daily, unrealized, and realized PnL as [PnLUpdate] events.
func (c *Client) SubscribePnL(ctx context.Context, req PnLRequest, opts ...SubscriptionOption) (*Subscription[PnLUpdate], error) {
	return c.engine.SubscribePnL(ctx, req, opts...)
}

// SubscribePnLSingle starts a real-time PnL subscription for a single position
// identified by contract ID.
func (c *Client) SubscribePnLSingle(ctx context.Context, req PnLSingleRequest, opts ...SubscriptionOption) (*Subscription[PnLSingleUpdate], error) {
	return c.engine.SubscribePnLSingle(ctx, req, opts...)
}

// SubscribeTickByTick starts a tick-level data subscription. The tick type
// determines which fields are populated: "Last"/"AllLast" for trade ticks,
// "BidAsk" for bid/ask, "MidPoint" for midpoint.
func (c *Client) SubscribeTickByTick(ctx context.Context, req TickByTickRequest, opts ...SubscriptionOption) (*Subscription[TickByTickData], error) {
	return c.engine.SubscribeTickByTick(ctx, req, opts...)
}

// SubscribeNewsBulletins starts a subscription for IB system bulletins and
// exchange halt notifications.
func (c *Client) SubscribeNewsBulletins(ctx context.Context, allMessages bool, opts ...SubscriptionOption) (*Subscription[NewsBulletin], error) {
	return c.engine.SubscribeNewsBulletins(ctx, allMessages, opts...)
}

// SubscribeHistoricalBars starts a live-updating historical bar subscription
// using the keepUpToDate flag. Delivers an initial snapshot of historical bars
// followed by streaming updates.
func (c *Client) SubscribeHistoricalBars(ctx context.Context, req HistoricalBarsRequest, opts ...SubscriptionOption) (*Subscription[Bar], error) {
	return c.engine.SubscribeHistoricalBars(ctx, req, opts...)
}

// SecDefOptParams queries option chain parameters for an underlying: available
// exchanges, expirations, strikes, and multipliers.
func (c *Client) SecDefOptParams(ctx context.Context, req SecDefOptParamsRequest) ([]SecDefOptParams, error) {
	return c.engine.SecDefOptParams(ctx, req)
}

// SmartComponents queries exchange abbreviation mappings for SMART routing
// on the given BBO exchange.
func (c *Client) SmartComponents(ctx context.Context, bboExchange string) ([]SmartComponent, error) {
	return c.engine.SmartComponents(ctx, bboExchange)
}

// CalcImpliedVolatility requests a server-side implied volatility calculation
// from option and underlying prices.
func (c *Client) CalcImpliedVolatility(ctx context.Context, req CalcImpliedVolatilityRequest) (OptionComputation, error) {
	return c.engine.CalcImpliedVolatility(ctx, req)
}

// CalcOptionPrice requests a server-side theoretical option price calculation
// from volatility and underlying price.
func (c *Client) CalcOptionPrice(ctx context.Context, req CalcOptionPriceRequest) (OptionComputation, error) {
	return c.engine.CalcOptionPrice(ctx, req)
}

// HistogramData queries a price distribution histogram for the given contract
// and time period.
func (c *Client) HistogramData(ctx context.Context, req HistogramDataRequest) ([]HistogramEntry, error) {
	return c.engine.HistogramData(ctx, req)
}

// HistoricalTicks queries tick-level historical data (up to 1000 ticks per
// request). The result type depends on WhatToShow: MIDPOINT populates Ticks,
// BID_ASK populates BidAsk, TRADES populates Last.
func (c *Client) HistoricalTicks(ctx context.Context, req HistoricalTicksRequest) (HistoricalTicksResult, error) {
	return c.engine.HistoricalTicks(ctx, req)
}

// NewsArticle retrieves the full body of a news article by provider code
// and article ID.
func (c *Client) NewsArticle(ctx context.Context, req NewsArticleRequest) (NewsArticleResult, error) {
	return c.engine.NewsArticle(ctx, req)
}

// HistoricalNews queries past news headlines for a contract from one or more
// provider sources.
func (c *Client) HistoricalNews(ctx context.Context, req HistoricalNewsRequest) ([]HistoricalNewsItemResult, error) {
	return c.engine.HistoricalNews(ctx, req)
}

// SubscribeScannerResults starts a server-side scanner subscription. Results
// are delivered as batches of [ScannerResult] on each scan update.
func (c *Client) SubscribeScannerResults(ctx context.Context, req ScannerSubscriptionRequest, opts ...SubscriptionOption) (*Subscription[[]ScannerResult], error) {
	return c.engine.SubscribeScannerResults(ctx, req, opts...)
}

// FundamentalData retrieves Reuters fundamental data XML for the given
// contract and report type. Requires a fundamental data subscription.
func (c *Client) FundamentalData(ctx context.Context, req FundamentalDataRequest) (string, error) {
	return c.engine.FundamentalData(ctx, req)
}

// ExerciseOptions sends an option exercise request. This is fire-and-forget;
// results arrive as order status updates.
func (c *Client) ExerciseOptions(ctx context.Context, req ExerciseOptionsRequest) error {
	return c.engine.ExerciseOptions(ctx, req)
}

// PlaceOrder submits an order and returns an [*OrderHandle] that tracks its
// lifecycle. Order IDs are auto-allocated from NextValidID when Order.OrderID
// is 0. The handle auto-closes on terminal status (Filled, Cancelled, Inactive).
func (c *Client) PlaceOrder(ctx context.Context, req PlaceOrderRequest) (*OrderHandle, error) {
	return c.engine.PlaceOrder(ctx, req)
}

// CancelOrder sends a cancel request for an order by ID. The cancellation
// result arrives as an OrderStatus event on the order's handle.
func (c *Client) CancelOrder(ctx context.Context, orderID int64) error {
	return c.engine.CancelOrder(ctx, orderID)
}

// GlobalCancel sends a cancel request for all open orders. Individual
// cancellation results arrive on each order's handle.
func (c *Client) GlobalCancel(ctx context.Context) error {
	return c.engine.GlobalCancel(ctx)
}

// RequestFA retrieves Financial Advisor account configuration XML. The
// faDataType parameter selects the configuration type (1=groups, 2=profiles,
// 3=aliases).
func (c *Client) RequestFA(ctx context.Context, faDataType int) (string, error) {
	return c.engine.RequestFA(ctx, faDataType)
}

// ReplaceFA updates Financial Advisor account configuration. The faDataType
// parameter selects the configuration type and xml is the replacement content.
func (c *Client) ReplaceFA(ctx context.Context, faDataType int, xml string) error {
	return c.engine.ReplaceFA(ctx, faDataType, xml)
}

// SoftDollarTiers queries available soft dollar tiers. FA accounts only.
func (c *Client) SoftDollarTiers(ctx context.Context) ([]SoftDollarTier, error) {
	return c.engine.SoftDollarTiers(ctx)
}

// WSHMetaData queries available Wall Street Horizon event types and filter
// metadata. Returns JSON. Requires a WSH subscription.
func (c *Client) WSHMetaData(ctx context.Context) (string, error) {
	return c.engine.WSHMetaData(ctx)
}

// WSHEventData queries Wall Street Horizon calendar events matching the given
// filter criteria. Returns JSON. Requires a WSH subscription.
func (c *Client) WSHEventData(ctx context.Context, req WSHEventDataRequest) (string, error) {
	return c.engine.WSHEventData(ctx, req)
}

// QueryDisplayGroups queries available TWS display groups. Returns a
// pipe-delimited string of group IDs.
func (c *Client) QueryDisplayGroups(ctx context.Context) (string, error) {
	return c.engine.QueryDisplayGroups(ctx)
}

// SubscribeDisplayGroup starts a subscription for display group contract
// changes and returns a [*DisplayGroupHandle] that also supports pushing
// contract selections via Update.
func (c *Client) SubscribeDisplayGroup(ctx context.Context, groupID int, opts ...SubscriptionOption) (*DisplayGroupHandle, error) {
	return c.engine.SubscribeDisplayGroup(ctx, groupID, opts...)
}

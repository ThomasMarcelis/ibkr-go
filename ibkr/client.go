package ibkr

import (
	"context"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/session"
	"github.com/ThomasMarcelis/ibkr-go/internal/transport"
)

type (
	Dialer                       = transport.Dialer
	Option                       = session.Option
	SubscriptionOption           = session.SubscriptionOption
	SessionState                 = session.State
	SessionSnapshot              = session.Snapshot
	SessionEvent                 = session.Event
	OpKind                       = session.OpKind
	ReconnectPolicy              = session.ReconnectPolicy
	ResumePolicy                 = session.ResumePolicy
	SlowConsumerPolicy           = session.SlowConsumerPolicy
	SubscriptionStateKind        = session.SubscriptionStateKind
	SubscriptionStateEvent       = session.SubscriptionStateEvent
	Subscription[T any]          = session.Subscription[T]
	Decimal                      = session.Decimal
	Contract                     = session.Contract
	ContractDetailsRequest       = session.ContractDetailsRequest
	ContractDetails              = session.ContractDetails
	QualifiedContract            = session.QualifiedContract
	HistoricalBarsRequest        = session.HistoricalBarsRequest
	Bar                          = session.Bar
	AccountSummaryRequest        = session.AccountSummaryRequest
	AccountValue                 = session.AccountValue
	AccountSummaryUpdate         = session.AccountSummaryUpdate
	Position                     = session.Position
	PositionUpdate               = session.PositionUpdate
	QuoteFields                  = session.QuoteFields
	MarketDataType               = session.MarketDataType
	Quote                        = session.Quote
	QuoteSubscriptionRequest     = session.QuoteSubscriptionRequest
	QuoteUpdate                  = session.QuoteUpdate
	RealTimeBarsRequest          = session.RealTimeBarsRequest
	OpenOrdersScope              = session.OpenOrdersScope
	OpenOrder                    = session.OpenOrder
	OpenOrderUpdate              = session.OpenOrderUpdate
	ExecutionsRequest            = session.ExecutionsRequest
	Execution                    = session.Execution
	ExecutionUpdate              = session.ExecutionUpdate
	CommissionReport             = session.CommissionReport
	FamilyCode                   = session.FamilyCode
	DepthExchange                = session.DepthExchange
	NewsProvider                 = session.NewsProvider
	MatchingSymbolsRequest       = session.MatchingSymbolsRequest
	MatchingSymbol               = session.MatchingSymbol
	HeadTimestampRequest         = session.HeadTimestampRequest
	PriceIncrement               = session.PriceIncrement
	MarketRuleResult             = session.MarketRuleResult
	CompletedOrderResult         = session.CompletedOrderResult
	AccountUpdateValue           = session.AccountUpdateValue
	PortfolioUpdate              = session.PortfolioUpdate
	AccountUpdate                = session.AccountUpdate
	AccountUpdatesMultiRequest   = session.AccountUpdatesMultiRequest
	AccountUpdateMultiValue      = session.AccountUpdateMultiValue
	PositionsMultiRequest        = session.PositionsMultiRequest
	PositionMulti                = session.PositionMulti
	PnLRequest                   = session.PnLRequest
	PnLUpdate                    = session.PnLUpdate
	PnLSingleRequest             = session.PnLSingleRequest
	PnLSingleUpdate              = session.PnLSingleUpdate
	TickByTickRequest            = session.TickByTickRequest
	TickByTickData               = session.TickByTickData
	NewsBulletin                 = session.NewsBulletin
	SecDefOptParamsRequest       = session.SecDefOptParamsRequest
	SecDefOptParams              = session.SecDefOptParams
	SmartComponent               = session.SmartComponent
	CalcImpliedVolatilityRequest = session.CalcImpliedVolatilityRequest
	CalcOptionPriceRequest       = session.CalcOptionPriceRequest
	OptionComputation            = session.OptionComputation
	HistogramDataRequest         = session.HistogramDataRequest
	HistogramEntry               = session.HistogramEntry
	HistoricalTicksRequest       = session.HistoricalTicksRequest
	HistoricalTick               = session.HistoricalTick
	HistoricalTickBidAsk         = session.HistoricalTickBidAsk
	HistoricalTickLast           = session.HistoricalTickLast
	HistoricalTicksResult        = session.HistoricalTicksResult
	NewsArticleRequest           = session.NewsArticleRequest
	NewsArticleResult            = session.NewsArticle
	HistoricalNewsRequest        = session.HistoricalNewsRequest
	HistoricalNewsItemResult     = session.HistoricalNewsItem
	ScannerSubscriptionRequest   = session.ScannerSubscriptionRequest
	ScannerResult                = session.ScannerResult
	SoftDollarTier               = session.SoftDollarTier
	WSHEventDataRequest          = session.WSHEventDataRequest
	DisplayGroupUpdate           = session.DisplayGroupUpdate
	MarketDepthRequest           = session.MarketDepthRequest
	DepthRow                     = session.DepthRow
	FundamentalDataRequest       = session.FundamentalDataRequest
	ExerciseOptionsRequest       = session.ExerciseOptionsRequest
	ConnectError                 = session.ConnectError
	ProtocolError                = session.ProtocolError
	APIError                     = session.APIError
	OrderAction                  = session.OrderAction
	TimeInForce                  = session.TimeInForce
	Order                        = session.Order
	PlaceOrderRequest            = session.PlaceOrderRequest
	OrderEvent                   = session.OrderEvent
	OrderStatusUpdate            = session.OrderStatusUpdate
	OrderHandle                  = session.OrderHandle
)

const (
	StateDisconnected = session.StateDisconnected
	StateConnecting   = session.StateConnecting
	StateHandshaking  = session.StateHandshaking
	StateReady        = session.StateReady
	StateDegraded     = session.StateDegraded
	StateReconnecting = session.StateReconnecting
	StateClosed       = session.StateClosed

	ReconnectOff  = session.ReconnectOff
	ReconnectAuto = session.ReconnectAuto

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

	ResumeNever = session.ResumeNever
	ResumeAuto  = session.ResumeAuto

	SlowConsumerClose      = session.SlowConsumerClose
	SlowConsumerDropOldest = session.SlowConsumerDropOldest

	SubscriptionStarted          = session.SubscriptionStarted
	SubscriptionSnapshotComplete = session.SubscriptionSnapshotComplete
	SubscriptionGap              = session.SubscriptionGap
	SubscriptionResumed          = session.SubscriptionResumed
	SubscriptionClosed           = session.SubscriptionClosed

	OpenOrdersScopeAll    = session.OpenOrdersScopeAll
	OpenOrdersScopeClient = session.OpenOrdersScopeClient
	OpenOrdersScopeAuto   = session.OpenOrdersScopeAuto

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

	Buy  = session.Buy
	Sell = session.Sell

	TIFDay = session.TIFDay
	TIFGTC = session.TIFGTC
	TIFIOC = session.TIFIOC
	TIFGTD = session.TIFGTD
	TIFOPG = session.TIFOPG
	TIFFOK = session.TIFFOK
	TIFDTC = session.TIFDTC

	OpPlaceOrder   = session.OpPlaceOrder
	OpCancelOrder  = session.OpCancelOrder
	OpGlobalCancel = session.OpGlobalCancel
)

var (
	ErrInvalidDecimal           = session.ErrInvalidDecimal
	ErrNotReady                 = session.ErrNotReady
	ErrInterrupted              = session.ErrInterrupted
	ErrResumeRequired           = session.ErrResumeRequired
	ErrSlowConsumer             = session.ErrSlowConsumer
	ErrUnsupportedServerVersion = session.ErrUnsupportedServerVersion
	ErrClosed                   = session.ErrClosed
	ErrNoMatch                  = session.ErrNoMatch
	ErrAmbiguousContract        = session.ErrAmbiguousContract
)

var (
	ParseDecimal     = session.ParseDecimal
	MustParseDecimal = session.MustParseDecimal

	WithHost                      = session.WithHost
	WithPort                      = session.WithPort
	WithClientID                  = session.WithClientID
	WithLogger                    = session.WithLogger
	WithReconnectPolicy           = session.WithReconnectPolicy
	WithSendRate                  = session.WithSendRate
	WithEventBuffer               = session.WithEventBuffer
	WithSubscriptionBuffer        = session.WithSubscriptionBuffer
	WithDefaultResumePolicy       = session.WithDefaultResumePolicy
	WithDefaultSlowConsumerPolicy = session.WithDefaultSlowConsumerPolicy
	WithResumePolicy              = session.WithResumePolicy
	WithSlowConsumerPolicy        = session.WithSlowConsumerPolicy
	WithQueueSize                 = session.WithQueueSize
)

func WithDialer(dialer Dialer) Option {
	return session.WithDialer(dialer)
}

type Client struct {
	engine *session.Engine
}

func DialContext(ctx context.Context, opts ...Option) (*Client, error) {
	engine, err := session.DialContext(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{engine: engine}, nil
}

func (c *Client) Close() error { return c.engine.Close() }

func (c *Client) Done() <-chan struct{} { return c.engine.Done() }

func (c *Client) Wait() error { return c.engine.Wait() }

func (c *Client) Session() SessionSnapshot { return c.engine.Session() }

func (c *Client) SessionEvents() <-chan SessionEvent { return c.engine.SessionEvents() }

func (c *Client) ContractDetails(ctx context.Context, req ContractDetailsRequest) ([]ContractDetails, error) {
	return c.engine.ContractDetails(ctx, req)
}

func (c *Client) QualifyContract(ctx context.Context, contract Contract) (QualifiedContract, error) {
	return c.engine.QualifyContract(ctx, contract)
}

func (c *Client) HistoricalBars(ctx context.Context, req HistoricalBarsRequest) ([]Bar, error) {
	return c.engine.HistoricalBars(ctx, req)
}

func (c *Client) AccountSummary(ctx context.Context, req AccountSummaryRequest) ([]AccountValue, error) {
	return c.engine.AccountSummary(ctx, req)
}

func (c *Client) SubscribeAccountSummary(ctx context.Context, req AccountSummaryRequest, opts ...SubscriptionOption) (*Subscription[AccountSummaryUpdate], error) {
	return c.engine.SubscribeAccountSummary(ctx, req, opts...)
}

func (c *Client) PositionsSnapshot(ctx context.Context) ([]Position, error) {
	return c.engine.PositionsSnapshot(ctx)
}

func (c *Client) SubscribePositions(ctx context.Context, opts ...SubscriptionOption) (*Subscription[PositionUpdate], error) {
	return c.engine.SubscribePositions(ctx, opts...)
}

func (c *Client) SetMarketDataType(dataType int) error {
	return c.engine.SetMarketDataType(dataType)
}

func (c *Client) QuoteSnapshot(ctx context.Context, req QuoteSubscriptionRequest) (Quote, error) {
	return c.engine.QuoteSnapshot(ctx, req)
}

func (c *Client) SubscribeQuotes(ctx context.Context, req QuoteSubscriptionRequest, opts ...SubscriptionOption) (*Subscription[QuoteUpdate], error) {
	return c.engine.SubscribeQuotes(ctx, req, opts...)
}

func (c *Client) SubscribeRealTimeBars(ctx context.Context, req RealTimeBarsRequest, opts ...SubscriptionOption) (*Subscription[Bar], error) {
	return c.engine.SubscribeRealTimeBars(ctx, req, opts...)
}

func (c *Client) SubscribeMarketDepth(ctx context.Context, req MarketDepthRequest, opts ...SubscriptionOption) (*Subscription[DepthRow], error) {
	return c.engine.SubscribeMarketDepth(ctx, req, opts...)
}

func (c *Client) OpenOrdersSnapshot(ctx context.Context, scope OpenOrdersScope) ([]OpenOrder, error) {
	return c.engine.OpenOrdersSnapshot(ctx, scope)
}

func (c *Client) SubscribeOpenOrders(ctx context.Context, scope OpenOrdersScope, opts ...SubscriptionOption) (*Subscription[OpenOrderUpdate], error) {
	return c.engine.SubscribeOpenOrders(ctx, scope, opts...)
}

func (c *Client) Executions(ctx context.Context, req ExecutionsRequest) ([]ExecutionUpdate, error) {
	return c.engine.Executions(ctx, req)
}

func (c *Client) SubscribeExecutions(ctx context.Context, req ExecutionsRequest, opts ...SubscriptionOption) (*Subscription[ExecutionUpdate], error) {
	return c.engine.SubscribeExecutions(ctx, req, opts...)
}

func (c *Client) FamilyCodes(ctx context.Context) ([]FamilyCode, error) {
	return c.engine.FamilyCodes(ctx)
}

func (c *Client) MktDepthExchanges(ctx context.Context) ([]DepthExchange, error) {
	return c.engine.MktDepthExchanges(ctx)
}

func (c *Client) NewsProviders(ctx context.Context) ([]NewsProvider, error) {
	return c.engine.NewsProviders(ctx)
}

func (c *Client) ScannerParameters(ctx context.Context) (string, error) {
	return c.engine.ScannerParameters(ctx)
}

func (c *Client) UserInfo(ctx context.Context) (string, error) {
	return c.engine.UserInfo(ctx)
}

func (c *Client) MatchingSymbols(ctx context.Context, req MatchingSymbolsRequest) ([]MatchingSymbol, error) {
	return c.engine.MatchingSymbols(ctx, req)
}

func (c *Client) HeadTimestamp(ctx context.Context, req HeadTimestampRequest) (time.Time, error) {
	return c.engine.HeadTimestamp(ctx, req)
}

func (c *Client) MarketRule(ctx context.Context, marketRuleID int) (MarketRuleResult, error) {
	return c.engine.MarketRule(ctx, marketRuleID)
}

func (c *Client) CompletedOrders(ctx context.Context, apiOnly bool) ([]CompletedOrderResult, error) {
	return c.engine.CompletedOrders(ctx, apiOnly)
}

func (c *Client) AccountUpdatesSnapshot(ctx context.Context, account string) ([]AccountUpdate, error) {
	return c.engine.AccountUpdatesSnapshot(ctx, account)
}

func (c *Client) SubscribeAccountUpdates(ctx context.Context, account string, opts ...SubscriptionOption) (*Subscription[AccountUpdate], error) {
	return c.engine.SubscribeAccountUpdates(ctx, account, opts...)
}

func (c *Client) AccountUpdatesMultiSnapshot(ctx context.Context, req AccountUpdatesMultiRequest) ([]AccountUpdateMultiValue, error) {
	return c.engine.AccountUpdatesMultiSnapshot(ctx, req)
}

func (c *Client) SubscribeAccountUpdatesMulti(ctx context.Context, req AccountUpdatesMultiRequest, opts ...SubscriptionOption) (*Subscription[AccountUpdateMultiValue], error) {
	return c.engine.SubscribeAccountUpdatesMulti(ctx, req, opts...)
}

func (c *Client) PositionsMultiSnapshot(ctx context.Context, req PositionsMultiRequest) ([]PositionMulti, error) {
	return c.engine.PositionsMultiSnapshot(ctx, req)
}

func (c *Client) SubscribePositionsMulti(ctx context.Context, req PositionsMultiRequest, opts ...SubscriptionOption) (*Subscription[PositionMulti], error) {
	return c.engine.SubscribePositionsMulti(ctx, req, opts...)
}

func (c *Client) SubscribePnL(ctx context.Context, req PnLRequest, opts ...SubscriptionOption) (*Subscription[PnLUpdate], error) {
	return c.engine.SubscribePnL(ctx, req, opts...)
}

func (c *Client) SubscribePnLSingle(ctx context.Context, req PnLSingleRequest, opts ...SubscriptionOption) (*Subscription[PnLSingleUpdate], error) {
	return c.engine.SubscribePnLSingle(ctx, req, opts...)
}

func (c *Client) SubscribeTickByTick(ctx context.Context, req TickByTickRequest, opts ...SubscriptionOption) (*Subscription[TickByTickData], error) {
	return c.engine.SubscribeTickByTick(ctx, req, opts...)
}

func (c *Client) SubscribeNewsBulletins(ctx context.Context, allMessages bool, opts ...SubscriptionOption) (*Subscription[NewsBulletin], error) {
	return c.engine.SubscribeNewsBulletins(ctx, allMessages, opts...)
}

func (c *Client) SubscribeHistoricalBars(ctx context.Context, req HistoricalBarsRequest, opts ...SubscriptionOption) (*Subscription[Bar], error) {
	return c.engine.SubscribeHistoricalBars(ctx, req, opts...)
}

func (c *Client) SecDefOptParams(ctx context.Context, req SecDefOptParamsRequest) ([]SecDefOptParams, error) {
	return c.engine.SecDefOptParams(ctx, req)
}

func (c *Client) SmartComponents(ctx context.Context, bboExchange string) ([]SmartComponent, error) {
	return c.engine.SmartComponents(ctx, bboExchange)
}

func (c *Client) CalcImpliedVolatility(ctx context.Context, req CalcImpliedVolatilityRequest) (OptionComputation, error) {
	return c.engine.CalcImpliedVolatility(ctx, req)
}

func (c *Client) CalcOptionPrice(ctx context.Context, req CalcOptionPriceRequest) (OptionComputation, error) {
	return c.engine.CalcOptionPrice(ctx, req)
}

func (c *Client) HistogramData(ctx context.Context, req HistogramDataRequest) ([]HistogramEntry, error) {
	return c.engine.HistogramData(ctx, req)
}

func (c *Client) HistoricalTicks(ctx context.Context, req HistoricalTicksRequest) (HistoricalTicksResult, error) {
	return c.engine.HistoricalTicks(ctx, req)
}

func (c *Client) NewsArticle(ctx context.Context, req NewsArticleRequest) (NewsArticleResult, error) {
	return c.engine.NewsArticle(ctx, req)
}

func (c *Client) HistoricalNews(ctx context.Context, req HistoricalNewsRequest) ([]HistoricalNewsItemResult, error) {
	return c.engine.HistoricalNews(ctx, req)
}

func (c *Client) SubscribeScannerResults(ctx context.Context, req ScannerSubscriptionRequest, opts ...SubscriptionOption) (*Subscription[[]ScannerResult], error) {
	return c.engine.SubscribeScannerResults(ctx, req, opts...)
}

func (c *Client) FundamentalData(ctx context.Context, req FundamentalDataRequest) (string, error) {
	return c.engine.FundamentalData(ctx, req)
}

func (c *Client) ExerciseOptions(ctx context.Context, req ExerciseOptionsRequest) error {
	return c.engine.ExerciseOptions(ctx, req)
}

func (c *Client) PlaceOrder(ctx context.Context, req PlaceOrderRequest) (*OrderHandle, error) {
	return c.engine.PlaceOrder(ctx, req)
}

func (c *Client) CancelOrder(ctx context.Context, orderID int64) error {
	return c.engine.CancelOrder(ctx, orderID)
}

func (c *Client) GlobalCancel(ctx context.Context) error {
	return c.engine.GlobalCancel(ctx)
}

func (c *Client) RequestFA(ctx context.Context, faDataType int) (string, error) {
	return c.engine.RequestFA(ctx, faDataType)
}

func (c *Client) ReplaceFA(ctx context.Context, faDataType int, xml string) error {
	return c.engine.ReplaceFA(ctx, faDataType, xml)
}

func (c *Client) SoftDollarTiers(ctx context.Context) ([]SoftDollarTier, error) {
	return c.engine.SoftDollarTiers(ctx)
}

func (c *Client) WSHMetaData(ctx context.Context) (string, error) {
	return c.engine.WSHMetaData(ctx)
}

func (c *Client) WSHEventData(ctx context.Context, req WSHEventDataRequest) (string, error) {
	return c.engine.WSHEventData(ctx, req)
}

func (c *Client) QueryDisplayGroups(ctx context.Context) (string, error) {
	return c.engine.QueryDisplayGroups(ctx)
}

func (c *Client) SubscribeDisplayGroup(ctx context.Context, groupID int, opts ...SubscriptionOption) (*Subscription[DisplayGroupUpdate], error) {
	return c.engine.SubscribeDisplayGroup(ctx, groupID, opts...)
}

func (c *Client) UpdateDisplayGroup(ctx context.Context, reqID int, contractInfo string) error {
	return c.engine.UpdateDisplayGroup(ctx, reqID, contractInfo)
}

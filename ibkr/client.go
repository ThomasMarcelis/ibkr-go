package ibkr

import (
	"context"

	"github.com/ThomasMarcelis/ibkr-go/internal/session"
)

type (
	Option                   = session.Option
	SubscriptionOption       = session.SubscriptionOption
	SessionState             = session.State
	SessionSnapshot          = session.Snapshot
	SessionEvent             = session.Event
	ReconnectPolicy          = session.ReconnectPolicy
	ResumePolicy             = session.ResumePolicy
	SlowConsumerPolicy       = session.SlowConsumerPolicy
	SubscriptionStateKind    = session.SubscriptionStateKind
	SubscriptionStateEvent   = session.SubscriptionStateEvent
	Subscription[T any]      = session.Subscription[T]
	Decimal                  = session.Decimal
	Contract                 = session.Contract
	ContractDetailsRequest   = session.ContractDetailsRequest
	ContractDetails          = session.ContractDetails
	QualifiedContract        = session.QualifiedContract
	HistoricalBarsRequest    = session.HistoricalBarsRequest
	Bar                      = session.Bar
	AccountSummaryRequest    = session.AccountSummaryRequest
	AccountValue             = session.AccountValue
	AccountSummaryUpdate     = session.AccountSummaryUpdate
	Position                 = session.Position
	PositionUpdate           = session.PositionUpdate
	QuoteFields              = session.QuoteFields
	MarketDataType           = session.MarketDataType
	Quote                    = session.Quote
	QuoteSubscriptionRequest = session.QuoteSubscriptionRequest
	QuoteUpdate              = session.QuoteUpdate
	RealTimeBarsRequest      = session.RealTimeBarsRequest
	OpenOrdersScope          = session.OpenOrdersScope
	OpenOrder                = session.OpenOrder
	OpenOrderUpdate          = session.OpenOrderUpdate
	ExecutionsRequest        = session.ExecutionsRequest
	Execution                = session.Execution
	ExecutionUpdate          = session.ExecutionUpdate
	CommissionReport         = session.CommissionReport
	ConnectError             = session.ConnectError
	ProtocolError            = session.ProtocolError
	APIError                 = session.APIError
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
)

var (
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
	WithDialer                    = session.WithDialer
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

func (c *Client) QuoteSnapshot(ctx context.Context, req QuoteSubscriptionRequest) (Quote, error) {
	return c.engine.QuoteSnapshot(ctx, req)
}

func (c *Client) SubscribeQuotes(ctx context.Context, req QuoteSubscriptionRequest, opts ...SubscriptionOption) (*Subscription[QuoteUpdate], error) {
	return c.engine.SubscribeQuotes(ctx, req, opts...)
}

func (c *Client) SubscribeRealTimeBars(ctx context.Context, req RealTimeBarsRequest, opts ...SubscriptionOption) (*Subscription[Bar], error) {
	return c.engine.SubscribeRealTimeBars(ctx, req, opts...)
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

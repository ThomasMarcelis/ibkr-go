package ibkr

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Client is a connection to an Interactive Brokers TWS or IB Gateway instance.
// It is safe for concurrent use by multiple goroutines.
type Client struct {
	engine *engine
}

// DialContext connects to a TWS or IB Gateway instance and completes the
// protocol handshake, returning a ready [Client]. Configure the target and
// behavior with [Option] values such as [WithHost], [WithPort], and
// [WithClientID]; the defaults target 127.0.0.1:7497 with client ID 1. The
// context bounds the dial and handshake only, not the lifetime of the client.
func DialContext(ctx context.Context, opts ...Option) (*Client, error) {
	engine, err := dialEngine(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{engine: engine}, nil
}

// Close initiates shutdown of the client and its connection. It is idempotent.
// Active operations terminate with [ErrClosed], while [Client.Wait] returns nil
// after this intentional shutdown.
func (c *Client) Close() { c.engine.Close() }

// Done returns a channel closed when the client has terminated.
func (c *Client) Done() <-chan struct{} { return c.engine.Done() }

// Wait blocks until the client terminates and returns its terminal error. It
// returns nil when termination was initiated by [Client.Close].
func (c *Client) Wait() error { return c.engine.Wait() }

// Session returns a point-in-time [Snapshot] of the connection state.
func (c *Client) Session() Snapshot { return c.engine.Session() }

// SessionEvents returns the bounded stream of connection lifecycle [Event]
// values. Compare TransitionSeq values to detect evicted state transitions;
// each event carries its exact post-transition Snapshot. Repeated calls return
// the same channel; multiple readers divide events rather than each receiving
// a copy.
func (c *Client) SessionEvents() <-chan Event { return c.engine.SessionEvents() }

// CurrentTime asks the Gateway for the server's current wall-clock time. It
// shares a 4.25-second admission gate with [Client.CurrentTimeMillis] because
// live Gateway suppresses closely spaced clock requests. The context bounds
// both that wait and the response. Only one CurrentTime call may be in flight.
// If its context ends after admission, the client retires the owning connection
// generation because the reply has no request ID. The returned time is UTC.
func (c *Client) CurrentTime(ctx context.Context) (time.Time, error) {
	return c.engine.CurrentTime(ctx)
}

// CurrentTimeMillis requests the IBKR server time at millisecond precision. It
// has the same shared gate, context, and connection-retirement semantics as
// [Client.CurrentTime]. Only one CurrentTimeMillis call may be in flight.
func (c *Client) CurrentTimeMillis(ctx context.Context) (time.Time, error) {
	return c.engine.CurrentTimeMillis(ctx)
}

// ManagedAccounts asks the Gateway for the account IDs controlled by this
// login. It refreshes [Client.Session]'s ManagedAccounts snapshot and returns
// an independently owned copy. Only one refresh may be in flight at a time.
func (c *Client) ManagedAccounts(ctx context.Context) ([]string, error) {
	return c.engine.ManagedAccounts(ctx)
}

// Accounts returns the sub-client for account values, positions, and PnL.
func (c *Client) Accounts() AccountsClient { return AccountsClient{engine: c.engine} }

// Contracts returns the sub-client for contract search, qualification, and details.
func (c *Client) Contracts() ContractsClient { return ContractsClient{engine: c.engine} }

// MarketData returns the sub-client for live quotes, ticks, and market depth.
func (c *Client) MarketData() MarketDataClient { return MarketDataClient{engine: c.engine} }

// History returns the sub-client for historical bars, tick data, and schedules.
func (c *Client) History() HistoryClient { return HistoryClient{engine: c.engine} }

// Orders returns the sub-client for placing, cancelling, modifying, and observing orders.
func (c *Client) Orders() OrdersClient { return OrdersClient{engine: c.engine} }

// Options returns the sub-client for option calculations and exercise.
// Contract qualification and option-chain metadata are available through
// [Client.Contracts].
func (c *Client) Options() OptionsClient { return OptionsClient{engine: c.engine} }

// News returns the sub-client for news providers, articles, and headlines.
func (c *Client) News() NewsClient { return NewsClient{engine: c.engine} }

// Scanner returns the sub-client for server-side market scanners.
func (c *Client) Scanner() ScannerClient { return ScannerClient{engine: c.engine} }

// Advisors returns the sub-client for Financial Advisor configuration (FA accounts).
func (c *Client) Advisors() AdvisorsClient { return AdvisorsClient{engine: c.engine} }

// WSH returns the sub-client for Wall Street Horizon calendar events.
func (c *Client) WSH() WSHClient { return WSHClient{engine: c.engine} }

// TWS returns the sub-client for display groups and TWS integration.
func (c *Client) TWS() TWSClient { return TWSClient{engine: c.engine} }

// AccountsClient groups requests for account values, positions, and P&L.
// Obtain one from [Client.Accounts].
type AccountsClient struct{ engine *engine }

// Summary returns a one-shot account summary for the requested tags.
func (c AccountsClient) Summary(ctx context.Context, req AccountSummaryRequest) ([]AccountValue, error) {
	return c.engine.AccountSummary(ctx, req)
}

// SubscribeSummary streams account summary updates for the requested tags.
func (c AccountsClient) SubscribeSummary(ctx context.Context, req AccountSummaryRequest, opts ...SubscriptionOption) (*Subscription[AccountValue], error) {
	return c.engine.SubscribeAccountSummary(ctx, req, opts...)
}

// Positions returns a one-shot snapshot of all positions across accounts.
func (c AccountsClient) Positions(ctx context.Context) ([]Position, error) {
	return c.engine.PositionsSnapshot(ctx)
}

// SubscribePositions streams position updates across accounts.
func (c AccountsClient) SubscribePositions(ctx context.Context, opts ...SubscriptionOption) (*Subscription[Position], error) {
	return c.engine.SubscribePositions(ctx, opts...)
}

// Updates returns a one-shot snapshot of account values and portfolio for an account.
func (c AccountsClient) Updates(ctx context.Context, account string) ([]AccountUpdate, error) {
	return c.engine.AccountUpdatesSnapshot(ctx, account)
}

// SubscribeUpdates streams account value and portfolio updates for an account.
func (c AccountsClient) SubscribeUpdates(ctx context.Context, account string, opts ...SubscriptionOption) (*Subscription[AccountUpdate], error) {
	return c.engine.SubscribeAccountUpdates(ctx, account, opts...)
}

// UpdatesMulti returns a one-shot snapshot of account values for an account and model.
func (c AccountsClient) UpdatesMulti(ctx context.Context, req AccountUpdatesMultiRequest) ([]AccountUpdateMultiValue, error) {
	return c.engine.AccountUpdatesMultiSnapshot(ctx, req)
}

// SubscribeUpdatesMulti streams account value updates for an account and model.
func (c AccountsClient) SubscribeUpdatesMulti(ctx context.Context, req AccountUpdatesMultiRequest, opts ...SubscriptionOption) (*Subscription[AccountUpdateMultiValue], error) {
	return c.engine.SubscribeAccountUpdatesMulti(ctx, req, opts...)
}

// PositionsMulti returns a one-shot snapshot of positions for an account and model.
func (c AccountsClient) PositionsMulti(ctx context.Context, req PositionsMultiRequest) ([]PositionMulti, error) {
	return c.engine.PositionsMultiSnapshot(ctx, req)
}

// SubscribePositionsMulti streams position updates for an account and model.
func (c AccountsClient) SubscribePositionsMulti(ctx context.Context, req PositionsMultiRequest, opts ...SubscriptionOption) (*Subscription[PositionMulti], error) {
	return c.engine.SubscribePositionsMulti(ctx, req, opts...)
}

// SubscribePnL streams account-level profit-and-loss updates.
func (c AccountsClient) SubscribePnL(ctx context.Context, req PnLRequest, opts ...SubscriptionOption) (*Subscription[PnLUpdate], error) {
	return c.engine.SubscribePnL(ctx, req, opts...)
}

// SubscribePnLSingle streams profit-and-loss updates for a single position.
func (c AccountsClient) SubscribePnLSingle(ctx context.Context, req PnLSingleRequest, opts ...SubscriptionOption) (*Subscription[PnLSingleUpdate], error) {
	return c.engine.SubscribePnLSingle(ctx, req, opts...)
}

// FamilyCodes returns the account/family-code mapping for this login.
func (c AccountsClient) FamilyCodes(ctx context.Context) ([]FamilyCode, error) {
	return c.engine.FamilyCodes(ctx)
}

// ContractsClient groups contract search, qualification, and reference-data
// requests. Obtain one from [Client.Contracts].
type ContractsClient struct{ engine *engine }

// Details returns the full contract details matching a (possibly partial) contract.
func (c ContractsClient) Details(ctx context.Context, contract Contract) ([]ContractDetails, error) {
	return c.engine.ContractDetails(ctx, contract)
}

// StreamDetails streams contract matches through a bounded queue and closes
// after SnapshotComplete. Use it instead of Details when the result cardinality
// is not known in advance.
func (c ContractsClient) StreamDetails(ctx context.Context, contract Contract, opts ...SubscriptionOption) (*Subscription[ContractDetails], error) {
	return c.engine.StreamContractDetails(ctx, contract, opts...)
}

// Qualify resolves a partial contract to a single fully specified contract,
// returning [ErrNoMatch] or [ErrAmbiguousContract] when it does not resolve uniquely.
func (c ContractsClient) Qualify(ctx context.Context, contract Contract) (ContractDetails, error) {
	return c.engine.QualifyContract(ctx, contract)
}

// Search looks up contracts by symbol or name pattern.
func (c ContractsClient) Search(ctx context.Context, pattern string) ([]MatchingSymbol, error) {
	return c.engine.MatchingSymbols(ctx, pattern)
}

// MarketRule returns the tick-size schedule for a market rule ID (from [ContractDetails]).
func (c ContractsClient) MarketRule(ctx context.Context, marketRuleID MarketRuleID) (MarketRuleResult, error) {
	return c.engine.MarketRule(ctx, marketRuleID)
}

// SecDefOptParams returns the option chain parameters for an underlying.
func (c ContractsClient) SecDefOptParams(ctx context.Context, req SecDefOptParamsRequest) ([]SecDefOptParams, error) {
	return c.engine.SecDefOptParams(ctx, req)
}

// StreamSecDefOptParams streams option-chain parameter sets through a bounded
// queue and closes after SnapshotComplete.
func (c ContractsClient) StreamSecDefOptParams(ctx context.Context, req SecDefOptParamsRequest, opts ...SubscriptionOption) (*Subscription[SecDefOptParams], error) {
	return c.engine.StreamSecDefOptParams(ctx, req, opts...)
}

// SmartComponents returns the exchange mapping for a SMART-routed BBO exchange.
func (c ContractsClient) SmartComponents(ctx context.Context, bboExchange string) ([]SmartComponent, error) {
	return c.engine.SmartComponents(ctx, bboExchange)
}

// DepthExchanges returns the exchanges that offer market depth.
func (c ContractsClient) DepthExchanges(ctx context.Context) ([]DepthExchange, error) {
	return c.engine.MktDepthExchanges(ctx)
}

// MarketDataClient groups live quote, tick, and market-depth requests. Obtain
// one from [Client.MarketData].
type MarketDataClient struct{ engine *engine }

// SetType selects live, frozen, delayed, or delayed-frozen market data.
// It returns after local queue admission. The last admitted selection is
// restored before subscriptions and new work on each physical reconnect.
func (c MarketDataClient) SetType(ctx context.Context, dataType MarketDataType) error {
	return c.engine.SetMarketDataType(ctx, dataType)
}

// Quote returns a one-shot market data snapshot for a contract.
func (c MarketDataClient) Quote(ctx context.Context, req QuoteRequest) (Quote, error) {
	return c.engine.QuoteSnapshot(ctx, req)
}

// RegulatorySnapshot requests IBKR's fee-bearing US regulatory snapshot for
// contract. IBKR bills eligible requests individually and completes the call
// only when tickSnapshotEnd arrives. Once admitted, loss of completion
// evidence returns [ErrRegulatorySnapshotUncertain], which is not safe for an
// automatic retry because the fee may already have been incurred.
func (c MarketDataClient) RegulatorySnapshot(ctx context.Context, contract Contract) (Quote, error) {
	return c.engine.RegulatorySnapshot(ctx, contract)
}

// SubscribeQuotes streams quote updates for a contract. The default
// [ResumeNever] policy does not replay the request after data loss; opt into
// reissuing it with [WithResumePolicy] and [ResumeAuto].
func (c MarketDataClient) SubscribeQuotes(ctx context.Context, req QuoteRequest, opts ...SubscriptionOption) (*Subscription[QuoteUpdate], error) {
	return c.engine.SubscribeQuotes(ctx, req, opts...)
}

// SubscribeRealTimeBars streams 5-second real-time bars for a contract. The
// default [ResumeNever] policy does not replay the request after data loss; opt
// into reissuing it with [WithResumePolicy] and [ResumeAuto].
func (c MarketDataClient) SubscribeRealTimeBars(ctx context.Context, req RealTimeBarsRequest, opts ...SubscriptionOption) (*Subscription[Bar], error) {
	return c.engine.SubscribeRealTimeBars(ctx, req, opts...)
}

// SubscribeTickByTick streams tick-by-tick data for a contract. Use
// [WithQueueSize] when the default queue is too small for expected bursts.
func (c MarketDataClient) SubscribeTickByTick(ctx context.Context, req TickByTickRequest, opts ...SubscriptionOption) (*Subscription[TickByTickData], error) {
	return c.engine.SubscribeTickByTick(ctx, req, opts...)
}

// SubscribeDepth streams market depth (Level 2) order-book updates. Because
// dropping a delta corrupts the local book, queue overflow terminates the
// subscription with [ErrSlowConsumer]. Use [WithQueueSize] for more burst capacity.
func (c MarketDataClient) SubscribeDepth(ctx context.Context, req MarketDepthRequest, opts ...SubscriptionOption) (*Subscription[DepthRow], error) {
	return c.engine.SubscribeMarketDepth(ctx, req, opts...)
}

// HistoryClient groups historical bar, tick, and schedule requests. Obtain one
// from [Client.History].
type HistoryClient struct{ engine *engine }

// Bars returns historical bars for a contract.
func (c HistoryClient) Bars(ctx context.Context, req HistoricalBarsRequest) ([]Bar, error) {
	return c.engine.HistoricalBars(ctx, req)
}

// SubscribeBars streams historical bars followed by live updates ("keep up to date").
func (c HistoryClient) SubscribeBars(ctx context.Context, req HistoricalBarsRequest, opts ...SubscriptionOption) (*Subscription[Bar], error) {
	return c.engine.SubscribeHistoricalBars(ctx, req, opts...)
}

// HeadTimestamp returns the earliest available data timestamp for a contract.
func (c HistoryClient) HeadTimestamp(ctx context.Context, req HeadTimestampRequest) (time.Time, error) {
	return c.engine.HeadTimestamp(ctx, req)
}

// Histogram returns a price histogram for a contract over a period.
func (c HistoryClient) Histogram(ctx context.Context, req HistogramDataRequest) ([]HistogramEntry, error) {
	return c.engine.HistogramData(ctx, req)
}

// Ticks returns historical ticks for a contract; the populated result slice
// depends on [HistoricalTicksRequest].WhatToShow.
func (c HistoryClient) Ticks(ctx context.Context, req HistoricalTicksRequest) (HistoricalTicksResult, error) {
	return c.engine.HistoricalTicks(ctx, req)
}

// Schedule returns the trading session schedule that would cover the bars a
// matching [HistoricalBarsRequest] with whatToShow=SCHEDULE would produce.
// The Gateway reuses REQ_HISTORICAL_DATA (msg_id 20) for this request and
// replies with a distinct historicalSchedule callback (msg_id 106).
func (c HistoryClient) Schedule(ctx context.Context, req HistoricalScheduleRequest) (HistoricalSchedule, error) {
	return c.engine.HistoricalSchedule(ctx, req)
}

// OrdersClient groups order placement, modification, cancellation, and
// observation. Obtain one from [Client.Orders].
type OrdersClient struct{ engine *engine }

// RefreshOrderID asks the Gateway to refresh the engine's conservative
// order-ID floor. The returned ID remains engine-owned; later allocation may
// advance past it to avoid request or order collisions, and callers do not
// pass it back to Place.
func (c OrdersClient) RefreshOrderID(ctx context.Context) (int64, error) {
	return c.engine.RefreshOrderID(ctx)
}

// Place submits an order and returns an [OrderHandle] tracking its lifecycle.
// Once the request enters the transport queue, the handle and a nil error win
// context-cancellation and session-close races. Before admission, Place returns
// an error and no handle. Use [OrdersClient.Preview] for a margin preview.
// The [paper order example] demonstrates bounded observation, cancellation,
// and reconciliation; closing a handle does not cancel its order.
//
// [paper order example]: https://github.com/ThomasMarcelis/ibkr-go/blob/main/examples/order/main.go
func (c OrdersClient) Place(ctx context.Context, req PlaceOrderRequest) (*OrderHandle, error) {
	return c.engine.PlaceOrder(ctx, req)
}

// PlaceBracket submits a parent, take-profit, and stop-loss as one safely
// sequenced bracket. It allocates the three IDs together and controls ParentID
// and Transmit so the final child releases the complete bracket atomically. A
// partial send returns [*OrderRecoveryError] with every admitted leg, even when
// all compensating cancellations entered the transport queue, because queue
// admission is not Gateway acknowledgement.
func (c OrdersClient) PlaceBracket(ctx context.Context, req PlaceBracketRequest) (BracketOrder, error) {
	return c.engine.PlaceBracket(ctx, req)
}

// Preview submits a what-if order and returns the Gateway's margin-and-commission
// preview as an [OrderState]. It sets the wire-level what-if flag; nothing
// rests on the server and no OrderHandle is created.
func (c OrdersClient) Preview(ctx context.Context, req PlaceOrderRequest) (OrderState, error) {
	return c.engine.PreviewOrder(ctx, req)
}

// Cancel requests cancellation of a single order by ID when IBKR's client-ID
// ownership rules permit it. Options are only needed for operator-entered
// compliance metadata.
func (c OrdersClient) Cancel(ctx context.Context, orderID int64, opts ...CancelOption) error {
	cfg, err := applyCancelOptions(opts)
	if err != nil {
		return err
	}
	return c.engine.CancelOrder(ctx, orderID, cfg)
}

// CancelAll issues a global cancel for all open orders. External-operator and
// manual-order-indicator options are supported; manual cancel time is not.
func (c OrdersClient) CancelAll(ctx context.Context, opts ...CancelOption) error {
	cfg, err := applyCancelOptions(opts)
	if err != nil {
		return err
	}
	return c.engine.GlobalCancel(ctx, cfg)
}

// Open returns a one-shot snapshot of open orders in the client or all scope.
// It returns [ErrNoSnapshot] for [OpenOrdersScopeAuto], which binds future
// manual orders but has no snapshot boundary; use [OrdersClient.SubscribeOpen]
// for that scope. Open never creates an [OrderHandle] for an observed order.
// Cancellation by ID remains subject to IBKR client-ID ownership, and
// replacement is available only through an existing handle returned by
// [OrdersClient.Place] or [OrdersClient.PlaceBracket].
func (c OrdersClient) Open(ctx context.Context, scope OpenOrdersScope) ([]OpenOrder, error) {
	return c.engine.OpenOrdersSnapshot(ctx, scope)
}

// SubscribeOpen streams open-order echoes and status updates in the given
// scope. An [OpenOrdersScopeAuto] subscription is persistent and has no
// snapshot phase, so [Subscription.AwaitSnapshot] returns [ErrNoSnapshot];
// closing it disables the automatic binding of future manual orders.
// SubscribeOpen never creates an [OrderHandle] for an observed order.
// Cancellation by ID remains subject to IBKR client-ID ownership, and
// replacement is available only through an existing handle returned by
// [OrdersClient.Place] or [OrdersClient.PlaceBracket].
func (c OrdersClient) SubscribeOpen(ctx context.Context, scope OpenOrdersScope, opts ...SubscriptionOption) (*OpenOrdersSubscription, error) {
	return c.engine.SubscribeOpenOrders(ctx, scope, opts...)
}

// Completed returns terminal orders processed this session; apiOnly restricts
// the result to API-placed orders.
func (c OrdersClient) Completed(ctx context.Context, apiOnly bool) ([]CompletedOrderResult, error) {
	return c.engine.CompletedOrders(ctx, apiOnly)
}

// StreamCompleted streams terminal orders through a bounded queue and closes
// after SnapshotComplete. Closing before that boundary retires the owning
// connection generation because the callback sequence has no request ID.
func (c OrdersClient) StreamCompleted(ctx context.Context, apiOnly bool, opts ...SubscriptionOption) (*Subscription[CompletedOrderResult], error) {
	return c.engine.StreamCompletedOrders(ctx, apiOnly, opts...)
}

// Executions returns executions and the commission-and-fee reports observed by
// the execution-details end marker. Its finite collector accepts at most 4096
// execution events, 4096 fee-report events, 4096 distinct execution IDs, and
// 4096 pre-execution fee-report versions. If a legitimate snapshot can exceed
// those fixed bounds, use [OrdersClient.SubscribeExecutions] with
// [WithExecutionCorrelationLimit], drain through SnapshotComplete, then close
// the subscription. Exceeding a bound returns
// [ErrExecutionCorrelationOverflow]. IBKR can send additional fee reports
// after the end marker; the subscription form also preserves those late reports.
func (c OrdersClient) Executions(ctx context.Context, req ExecutionsRequest) (ExecutionSnapshot, error) {
	return c.engine.Executions(ctx, req)
}

// SubscribeExecutions streams executions and their separate commission-and-fee
// callbacks. SnapshotComplete marks executionsEnd; the stream remains open for
// late fee reports for executions observed in that snapshot until Close is
// called. Unmatched fee reports are discarded at that boundary, and later
// reports for unknown execution IDs are ignored. Correlation memory is finite:
// the default retains at most 4096 distinct execution IDs and 4096
// pre-execution fee-report versions. Use [WithExecutionCorrelationLimit] to
// change both bounds; overflow closes the stream with
// [ErrExecutionCorrelationOverflow].
func (c OrdersClient) SubscribeExecutions(ctx context.Context, req ExecutionsRequest, opts ...SubscriptionOption) (*Subscription[ExecutionUpdate], error) {
	return c.engine.subscribeExecutions(ctx, req, opts...)
}

// SubscribeExecutionEvents observes every execution-detail and
// commission-and-fee callback received by this client without issuing a
// Gateway request. It does not correlate, deduplicate, or discard unmatched
// callbacks. Only one observer may be active per client. WithQueueSize is the
// only operation-specific subscription option; the stream follows the local
// client across automatic reconnects and marks evidence gaps explicitly.
func (c OrdersClient) SubscribeExecutionEvents(ctx context.Context, opts ...SubscriptionOption) (*Subscription[ExecutionEvent], error) {
	return c.engine.SubscribeExecutionEvents(ctx, opts...)
}

// OptionsClient groups option pricing, implied-volatility, and exercise
// requests. Obtain one from [Client.Options]. Contract qualification and
// option-chain metadata belong to [ContractsClient].
type OptionsClient struct{ engine *engine }

// ImpliedVolatility computes an option's implied volatility from a given option
// and underlying price.
func (c OptionsClient) ImpliedVolatility(ctx context.Context, req CalcImpliedVolatilityRequest) (OptionComputation, error) {
	return c.engine.CalcImpliedVolatility(ctx, req)
}

// Price computes an option's price and greeks from a given volatility and
// underlying price.
func (c OptionsClient) Price(ctx context.Context, req CalcOptionPriceRequest) (OptionComputation, error) {
	return c.engine.CalcOptionPrice(ctx, req)
}

// Exercise admits an option exercise or lapse instruction to the client
// transport and returns its lossless request-scoped observation handle. A
// returned handle does not prove IBKR accepted or settled the instruction; use
// its events and independently reconcile the resulting account or position.
func (c OptionsClient) Exercise(ctx context.Context, req ExerciseOptionsRequest) (*ExerciseHandle, error) {
	if err := validateExerciseOptionsRequest(req); err != nil {
		return nil, err
	}
	return c.engine.ExerciseOptions(ctx, req)
}

// NewsClient groups news provider, article, and headline requests. Obtain one
// from [Client.News].
type NewsClient struct{ engine *engine }

// Providers returns the subscribed news providers.
func (c NewsClient) Providers(ctx context.Context) ([]NewsProvider, error) {
	return c.engine.NewsProviders(ctx)
}

// Article fetches the body of a news article.
func (c NewsClient) Article(ctx context.Context, req NewsArticleRequest) (NewsArticle, error) {
	return c.engine.NewsArticle(ctx, req)
}

// Historical returns one page of historical headlines and the Gateway's
// pagination signal.
func (c NewsClient) Historical(ctx context.Context, req HistoricalNewsRequest) (HistoricalNewsResult, error) {
	return c.engine.HistoricalNews(ctx, req)
}

// SubscribeBulletins streams news bulletins. When allMessages is true, the
// Gateway also replays the day's earlier bulletins.
func (c NewsClient) SubscribeBulletins(ctx context.Context, allMessages bool, opts ...SubscriptionOption) (*Subscription[NewsBulletin], error) {
	return c.engine.SubscribeNewsBulletins(ctx, allMessages, opts...)
}

// ScannerClient groups market scanner requests. Obtain one from [Client.Scanner].
type ScannerClient struct{ engine *engine }

// Parameters returns the scanner parameter definitions as a raw XML document,
// enumerating valid instruments, locations, and scan codes.
func (c ScannerClient) Parameters(ctx context.Context) (XMLDocument, error) {
	data, err := c.engine.ScannerParameters(ctx)
	return XMLDocument(data), err
}

// SubscribeResults streams ranked scanner results; each event is the full
// ranked list for a scan snapshot.
func (c ScannerClient) SubscribeResults(ctx context.Context, req ScannerSubscriptionRequest, opts ...SubscriptionOption) (*Subscription[[]ScannerResult], error) {
	return c.engine.SubscribeScannerResults(ctx, req, opts...)
}

// AdvisorsClient groups Financial Advisor configuration requests. Obtain one
// from [Client.Advisors].
type AdvisorsClient struct{ engine *engine }

// Config returns an FA configuration document as raw XML.
func (c AdvisorsClient) Config(ctx context.Context, dataType FADataType) (XMLDocument, error) {
	data, err := c.engine.RequestFA(ctx, dataType)
	return XMLDocument(data), err
}

// SoftDollarTiers returns the available soft-dollar commission tiers.
func (c AdvisorsClient) SoftDollarTiers(ctx context.Context) ([]SoftDollarTier, error) {
	return c.engine.SoftDollarTiers(ctx)
}

// WSHClient groups Wall Street Horizon calendar-event requests. Obtain one from
// [Client.WSH].
type WSHClient struct{ engine *engine }

// MetaData returns the WSH event metadata as a raw JSON document.
func (c WSHClient) MetaData(ctx context.Context) (JSONDocument, error) {
	data, err := c.engine.WSHMetaData(ctx)
	return JSONDocument(data), err
}

// EventData returns WSH calendar events as a raw JSON document.
func (c WSHClient) EventData(ctx context.Context, req WSHEventDataRequest) (JSONDocument, error) {
	data, err := c.engine.WSHEventData(ctx, req)
	return JSONDocument(data), err
}

// TWSClient groups display-group and TWS-integration requests. Obtain one from
// [Client.TWS].
type TWSClient struct{ engine *engine }

// Config returns the current TWS or IB Gateway configuration exposed by the
// socket API. Pointer fields preserve protobuf presence independently of a
// setting's zero value.
func (c TWSClient) Config(ctx context.Context) (TWSConfig, error) {
	return c.engine.Config(ctx)
}

// UserInfo returns the white-branding user information string for this login.
func (c TWSClient) UserInfo(ctx context.Context) (string, error) {
	return c.engine.UserInfo(ctx)
}

// DisplayGroups returns the IDs of the available TWS display groups.
func (c TWSClient) DisplayGroups(ctx context.Context) ([]DisplayGroupID, error) {
	groups, err := c.engine.QueryDisplayGroups(ctx)
	if err != nil {
		return nil, err
	}
	return parseDisplayGroups(groups)
}

// SubscribeDisplayGroup subscribes to a TWS display group, returning a handle
// that also lets the caller push the group's selected contract.
func (c TWSClient) SubscribeDisplayGroup(ctx context.Context, groupID DisplayGroupID, opts ...SubscriptionOption) (*DisplayGroupHandle, error) {
	return c.engine.SubscribeDisplayGroup(ctx, groupID, opts...)
}

func parseDisplayGroups(raw string) ([]DisplayGroupID, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parts := strings.Split(raw, "|")
	groups := make([]DisplayGroupID, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.ParseInt(strings.TrimSpace(part), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("ibkr: parse display group %q: %w", part, err)
		}
		groups = append(groups, DisplayGroupID(value))
	}
	return groups, nil
}

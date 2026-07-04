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

// Close shuts down the client and its connection. It is idempotent.
func (c *Client) Close() error { return c.engine.Close() }

// Done returns a channel closed when the client has terminated.
func (c *Client) Done() <-chan struct{} { return c.engine.Done() }

// Wait blocks until the client terminates and returns its terminal error.
func (c *Client) Wait() error { return c.engine.Wait() }

// Session returns a point-in-time [Snapshot] of the connection state.
func (c *Client) Session() Snapshot { return c.engine.Session() }

// SessionEvents returns the stream of connection lifecycle [Event] values.
func (c *Client) SessionEvents() <-chan Event { return c.engine.SessionEvents() }

// CurrentTime asks the Gateway for the server's current wall-clock time. The
// request is a one-shot keyed only by session singleton; only one request
// may be in flight at a time. The returned time is the parsed server time
// (UTC) as reported by IBKR's reqCurrentTime / currentTime callback pair.
func (c *Client) CurrentTime(ctx context.Context) (time.Time, error) {
	return c.engine.CurrentTime(ctx)
}

// CurrentTimeMillis requests the IBKR server time at millisecond precision
// (official reqCurrentTimeInMillis / currentTimeInMillis, server_version 197
// or later). Like [Client.CurrentTime], one request may be in flight at a
// time and the returned time is UTC.
func (c *Client) CurrentTimeMillis(ctx context.Context) (time.Time, error) {
	return c.engine.CurrentTimeMillis(ctx)
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

// Options returns the sub-client for option chains and calculation.
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
func (c AccountsClient) SubscribeSummary(ctx context.Context, req AccountSummaryRequest, opts ...SubscriptionOption) (*Subscription[AccountSummaryUpdate], error) {
	return c.engine.SubscribeAccountSummary(ctx, req, opts...)
}

// Positions returns a one-shot snapshot of all positions across accounts.
func (c AccountsClient) Positions(ctx context.Context) ([]Position, error) {
	return c.engine.PositionsSnapshot(ctx)
}

// SubscribePositions streams position updates across accounts.
func (c AccountsClient) SubscribePositions(ctx context.Context, opts ...SubscriptionOption) (*Subscription[PositionUpdate], error) {
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
func (c ContractsClient) MarketRule(ctx context.Context, marketRuleID int) (MarketRuleResult, error) {
	return c.engine.MarketRule(ctx, marketRuleID)
}

// SecDefOptParams returns the option chain parameters for an underlying.
func (c ContractsClient) SecDefOptParams(ctx context.Context, req SecDefOptParamsRequest) ([]SecDefOptParams, error) {
	return c.engine.SecDefOptParams(ctx, req)
}

// SmartComponents returns the exchange mapping for a SMART-routed BBO exchange.
func (c ContractsClient) SmartComponents(ctx context.Context, bboExchange string) ([]SmartComponent, error) {
	return c.engine.SmartComponents(ctx, bboExchange)
}

// DepthExchanges returns the exchanges that offer market depth.
func (c ContractsClient) DepthExchanges(ctx context.Context) ([]DepthExchange, error) {
	return c.engine.MktDepthExchanges(ctx)
}

// FundamentalData returns a fundamental data report as a raw XML document.
func (c ContractsClient) FundamentalData(ctx context.Context, req FundamentalDataRequest) (XMLDocument, error) {
	data, err := c.engine.FundamentalData(ctx, req)
	return XMLDocument(data), err
}

// MarketDataClient groups live quote, tick, and market-depth requests. Obtain
// one from [Client.MarketData].
type MarketDataClient struct{ engine *engine }

// SetType sets the market data type (live, frozen, delayed) for this session.
func (c MarketDataClient) SetType(ctx context.Context, dataType MarketDataType) error {
	return c.engine.SetMarketDataType(ctx, dataType)
}

// Quote returns a one-shot market data snapshot for a contract.
func (c MarketDataClient) Quote(ctx context.Context, req QuoteRequest) (Quote, error) {
	return c.engine.QuoteSnapshot(ctx, req)
}

// SubscribeQuotes streams quote updates for a contract.
func (c MarketDataClient) SubscribeQuotes(ctx context.Context, req QuoteRequest, opts ...SubscriptionOption) (*Subscription[QuoteUpdate], error) {
	return c.engine.SubscribeQuotes(ctx, req, opts...)
}

// SubscribeRealTimeBars streams 5-second real-time bars for a contract.
func (c MarketDataClient) SubscribeRealTimeBars(ctx context.Context, req RealTimeBarsRequest, opts ...SubscriptionOption) (*Subscription[Bar], error) {
	return c.engine.SubscribeRealTimeBars(ctx, req, opts...)
}

// SubscribeTickByTick streams tick-by-tick data for a contract. This is a
// high-rate stream; see [SlowConsumerPolicy] and [WithQueueSize].
func (c MarketDataClient) SubscribeTickByTick(ctx context.Context, req TickByTickRequest, opts ...SubscriptionOption) (*Subscription[TickByTickData], error) {
	return c.engine.SubscribeTickByTick(ctx, req, opts...)
}

// SubscribeDepth streams market depth (Level 2) order-book updates. This is a
// high-rate stream; see [SlowConsumerPolicy] and [WithQueueSize].
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

// Place submits an order and returns an [OrderHandle] tracking its lifecycle.
// What-if orders are rejected here; use [OrdersClient.Preview] for a margin preview.
func (c OrdersClient) Place(ctx context.Context, req PlaceOrderRequest) (*OrderHandle, error) {
	if req.Order.WhatIf != nil && *req.Order.WhatIf {
		return nil, &ValidationError{
			Field:   "Order.WhatIf",
			Message: "what-if orders are margin previews, not trades; use Orders().Preview",
		}
	}
	return c.engine.PlaceOrder(ctx, req)
}

// Preview submits a what-if order and returns the Gateway's margin-and-commission
// preview as an [OrderState]. It forces the what-if flag, so the place_order
// frame is byte-identical to a what-if order placed through Place; nothing rests
// on the server and no OrderHandle is created.
func (c OrdersClient) Preview(ctx context.Context, req PlaceOrderRequest) (OrderState, error) {
	return c.engine.PreviewOrder(ctx, req)
}

// Cancel requests cancellation of a single order by ID.
func (c OrdersClient) Cancel(ctx context.Context, orderID int64) error {
	return c.engine.CancelOrder(ctx, orderID)
}

// CancelAll issues a global cancel for all open orders.
func (c OrdersClient) CancelAll(ctx context.Context) error {
	return c.engine.GlobalCancel(ctx)
}

// Open returns a one-shot snapshot of open orders in the given scope.
func (c OrdersClient) Open(ctx context.Context, scope OpenOrdersScope) ([]OpenOrder, error) {
	return c.engine.OpenOrdersSnapshot(ctx, scope)
}

// SubscribeOpen streams open-order echoes and status updates in the given scope.
func (c OrdersClient) SubscribeOpen(ctx context.Context, scope OpenOrdersScope, opts ...SubscriptionOption) (*Subscription[OpenOrderUpdate], error) {
	return c.engine.SubscribeOpenOrders(ctx, scope, opts...)
}

// RefreshOpen requests a fresh open-orders snapshot on the active
// SubscribeOpen subscription: the current open orders arrive as Order events
// followed by another SnapshotComplete lifecycle event. Returns
// ErrNoSubscription when no open-orders subscription is active.
func (c OrdersClient) RefreshOpen(ctx context.Context) error {
	return c.engine.RefreshOpenOrders(ctx)
}

// Completed returns terminal orders processed this session; apiOnly restricts
// the result to API-placed orders.
func (c OrdersClient) Completed(ctx context.Context, apiOnly bool) ([]CompletedOrderResult, error) {
	return c.engine.CompletedOrders(ctx, apiOnly)
}

// Executions returns recent trade executions matching the request filter.
func (c OrdersClient) Executions(ctx context.Context, req ExecutionsRequest) ([]ExecutionUpdate, error) {
	return c.engine.Executions(ctx, req)
}

// OptionsClient groups option pricing, implied-volatility, and exercise
// requests. Obtain one from [Client.Options].
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

// Exercise exercises or lapses an option position.
func (c OptionsClient) Exercise(ctx context.Context, req ExerciseOptionsRequest) error {
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

// Historical returns historical news headlines for a contract.
func (c NewsClient) Historical(ctx context.Context, req HistoricalNewsRequest) ([]HistoricalNewsItem, error) {
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

// ReplaceConfig replaces an FA configuration document with the given raw XML.
// The call is fire-and-forget: the Gateway's replaceFAEnd acknowledgement is
// decoded but not awaited, since only financial-advisor accounts can verify
// the round trip.
func (c AdvisorsClient) ReplaceConfig(ctx context.Context, dataType FADataType, data XMLDocument) error {
	return c.engine.ReplaceFA(ctx, dataType, string(data))
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
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("ibkr: parse display group %q: %w", part, err)
		}
		groups = append(groups, DisplayGroupID(value))
	}
	return groups, nil
}

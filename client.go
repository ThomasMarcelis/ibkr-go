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

func DialContext(ctx context.Context, opts ...Option) (*Client, error) {
	engine, err := dialEngine(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{engine: engine}, nil
}

func (c *Client) Close() error                { return c.engine.Close() }
func (c *Client) Done() <-chan struct{}       { return c.engine.Done() }
func (c *Client) Wait() error                 { return c.engine.Wait() }
func (c *Client) Session() Snapshot           { return c.engine.Session() }
func (c *Client) SessionEvents() <-chan Event { return c.engine.SessionEvents() }

// CurrentTime asks the Gateway for the server's current wall-clock time. The
// request is a one-shot keyed only by session singleton; only one request
// may be in flight at a time. The returned time is the parsed server time
// (UTC) as reported by IBKR's reqCurrentTime / currentTime callback pair.
func (c *Client) CurrentTime(ctx context.Context) (time.Time, error) {
	return c.engine.CurrentTime(ctx)
}

func (c *Client) Accounts() AccountsClient   { return AccountsClient{engine: c.engine} }
func (c *Client) Contracts() ContractsClient { return ContractsClient{engine: c.engine} }
func (c *Client) MarketData() MarketDataClient {
	return MarketDataClient{engine: c.engine}
}
func (c *Client) History() HistoryClient   { return HistoryClient{engine: c.engine} }
func (c *Client) Orders() OrdersClient     { return OrdersClient{engine: c.engine} }
func (c *Client) Options() OptionsClient   { return OptionsClient{engine: c.engine} }
func (c *Client) News() NewsClient         { return NewsClient{engine: c.engine} }
func (c *Client) Scanner() ScannerClient   { return ScannerClient{engine: c.engine} }
func (c *Client) Advisors() AdvisorsClient { return AdvisorsClient{engine: c.engine} }
func (c *Client) WSH() WSHClient           { return WSHClient{engine: c.engine} }
func (c *Client) TWS() TWSClient           { return TWSClient{engine: c.engine} }

type AccountsClient struct{ engine *engine }

func (c AccountsClient) Summary(ctx context.Context, req AccountSummaryRequest) ([]AccountValue, error) {
	return c.engine.AccountSummary(ctx, req)
}
func (c AccountsClient) SubscribeSummary(ctx context.Context, req AccountSummaryRequest, opts ...SubscriptionOption) (*Subscription[AccountSummaryUpdate], error) {
	return c.engine.SubscribeAccountSummary(ctx, req, opts...)
}
func (c AccountsClient) Positions(ctx context.Context) ([]Position, error) {
	return c.engine.PositionsSnapshot(ctx)
}
func (c AccountsClient) SubscribePositions(ctx context.Context, opts ...SubscriptionOption) (*Subscription[PositionUpdate], error) {
	return c.engine.SubscribePositions(ctx, opts...)
}
func (c AccountsClient) Updates(ctx context.Context, account string) ([]AccountUpdate, error) {
	return c.engine.AccountUpdatesSnapshot(ctx, account)
}
func (c AccountsClient) SubscribeUpdates(ctx context.Context, account string, opts ...SubscriptionOption) (*Subscription[AccountUpdate], error) {
	return c.engine.SubscribeAccountUpdates(ctx, account, opts...)
}
func (c AccountsClient) UpdatesMulti(ctx context.Context, req AccountUpdatesMultiRequest) ([]AccountUpdateMultiValue, error) {
	return c.engine.AccountUpdatesMultiSnapshot(ctx, req)
}
func (c AccountsClient) SubscribeUpdatesMulti(ctx context.Context, req AccountUpdatesMultiRequest, opts ...SubscriptionOption) (*Subscription[AccountUpdateMultiValue], error) {
	return c.engine.SubscribeAccountUpdatesMulti(ctx, req, opts...)
}
func (c AccountsClient) PositionsMulti(ctx context.Context, req PositionsMultiRequest) ([]PositionMulti, error) {
	return c.engine.PositionsMultiSnapshot(ctx, req)
}
func (c AccountsClient) SubscribePositionsMulti(ctx context.Context, req PositionsMultiRequest, opts ...SubscriptionOption) (*Subscription[PositionMulti], error) {
	return c.engine.SubscribePositionsMulti(ctx, req, opts...)
}
func (c AccountsClient) SubscribePnL(ctx context.Context, req PnLRequest, opts ...SubscriptionOption) (*Subscription[PnLUpdate], error) {
	return c.engine.SubscribePnL(ctx, req, opts...)
}
func (c AccountsClient) SubscribePnLSingle(ctx context.Context, req PnLSingleRequest, opts ...SubscriptionOption) (*Subscription[PnLSingleUpdate], error) {
	return c.engine.SubscribePnLSingle(ctx, req, opts...)
}
func (c AccountsClient) FamilyCodes(ctx context.Context) ([]FamilyCode, error) {
	return c.engine.FamilyCodes(ctx)
}

type ContractsClient struct{ engine *engine }

func (c ContractsClient) Details(ctx context.Context, contract Contract) ([]ContractDetails, error) {
	return c.engine.ContractDetails(ctx, contract)
}
func (c ContractsClient) Qualify(ctx context.Context, contract Contract) (ContractDetails, error) {
	return c.engine.QualifyContract(ctx, contract)
}
func (c ContractsClient) Search(ctx context.Context, pattern string) ([]MatchingSymbol, error) {
	return c.engine.MatchingSymbols(ctx, pattern)
}
func (c ContractsClient) MarketRule(ctx context.Context, marketRuleID int) (MarketRuleResult, error) {
	return c.engine.MarketRule(ctx, marketRuleID)
}
func (c ContractsClient) SecDefOptParams(ctx context.Context, req SecDefOptParamsRequest) ([]SecDefOptParams, error) {
	return c.engine.SecDefOptParams(ctx, req)
}
func (c ContractsClient) SmartComponents(ctx context.Context, bboExchange string) ([]SmartComponent, error) {
	return c.engine.SmartComponents(ctx, bboExchange)
}
func (c ContractsClient) DepthExchanges(ctx context.Context) ([]DepthExchange, error) {
	return c.engine.MktDepthExchanges(ctx)
}
func (c ContractsClient) FundamentalData(ctx context.Context, req FundamentalDataRequest) (XMLDocument, error) {
	data, err := c.engine.FundamentalData(ctx, req)
	return XMLDocument(data), err
}

type MarketDataClient struct{ engine *engine }

func (c MarketDataClient) SetType(ctx context.Context, dataType MarketDataType) error {
	return c.engine.SetMarketDataType(ctx, dataType)
}
func (c MarketDataClient) Quote(ctx context.Context, req QuoteRequest) (Quote, error) {
	return c.engine.QuoteSnapshot(ctx, req)
}
func (c MarketDataClient) SubscribeQuotes(ctx context.Context, req QuoteRequest, opts ...SubscriptionOption) (*Subscription[QuoteUpdate], error) {
	return c.engine.SubscribeQuotes(ctx, req, opts...)
}
func (c MarketDataClient) SubscribeRealTimeBars(ctx context.Context, req RealTimeBarsRequest, opts ...SubscriptionOption) (*Subscription[Bar], error) {
	return c.engine.SubscribeRealTimeBars(ctx, req, opts...)
}
func (c MarketDataClient) SubscribeTickByTick(ctx context.Context, req TickByTickRequest, opts ...SubscriptionOption) (*Subscription[TickByTickData], error) {
	return c.engine.SubscribeTickByTick(ctx, req, opts...)
}
func (c MarketDataClient) SubscribeDepth(ctx context.Context, req MarketDepthRequest, opts ...SubscriptionOption) (*Subscription[DepthRow], error) {
	return c.engine.SubscribeMarketDepth(ctx, req, opts...)
}

type HistoryClient struct{ engine *engine }

func (c HistoryClient) Bars(ctx context.Context, req HistoricalBarsRequest) ([]Bar, error) {
	return c.engine.HistoricalBars(ctx, req)
}
func (c HistoryClient) SubscribeBars(ctx context.Context, req HistoricalBarsRequest, opts ...SubscriptionOption) (*Subscription[Bar], error) {
	return c.engine.SubscribeHistoricalBars(ctx, req, opts...)
}
func (c HistoryClient) HeadTimestamp(ctx context.Context, req HeadTimestampRequest) (time.Time, error) {
	return c.engine.HeadTimestamp(ctx, req)
}
func (c HistoryClient) Histogram(ctx context.Context, req HistogramDataRequest) ([]HistogramEntry, error) {
	return c.engine.HistogramData(ctx, req)
}
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

type OrdersClient struct{ engine *engine }

func (c OrdersClient) Place(ctx context.Context, req PlaceOrderRequest) (*OrderHandle, error) {
	return c.engine.PlaceOrder(ctx, req)
}
func (c OrdersClient) Cancel(ctx context.Context, orderID int64) error {
	return c.engine.CancelOrder(ctx, orderID)
}
func (c OrdersClient) CancelAll(ctx context.Context) error {
	return c.engine.GlobalCancel(ctx)
}
func (c OrdersClient) Open(ctx context.Context, scope OpenOrdersScope) ([]OpenOrder, error) {
	return c.engine.OpenOrdersSnapshot(ctx, scope)
}
func (c OrdersClient) SubscribeOpen(ctx context.Context, scope OpenOrdersScope, opts ...SubscriptionOption) (*Subscription[OpenOrderUpdate], error) {
	return c.engine.SubscribeOpenOrders(ctx, scope, opts...)
}
func (c OrdersClient) Completed(ctx context.Context, apiOnly bool) ([]CompletedOrderResult, error) {
	return c.engine.CompletedOrders(ctx, apiOnly)
}
func (c OrdersClient) Executions(ctx context.Context, req ExecutionsRequest) ([]ExecutionUpdate, error) {
	return c.engine.Executions(ctx, req)
}

type OptionsClient struct{ engine *engine }

func (c OptionsClient) ImpliedVolatility(ctx context.Context, req CalcImpliedVolatilityRequest) (OptionComputation, error) {
	return c.engine.CalcImpliedVolatility(ctx, req)
}
func (c OptionsClient) Price(ctx context.Context, req CalcOptionPriceRequest) (OptionComputation, error) {
	return c.engine.CalcOptionPrice(ctx, req)
}
func (c OptionsClient) Exercise(ctx context.Context, req ExerciseOptionsRequest) error {
	return c.engine.ExerciseOptions(ctx, req)
}

type NewsClient struct{ engine *engine }

func (c NewsClient) Providers(ctx context.Context) ([]NewsProvider, error) {
	return c.engine.NewsProviders(ctx)
}
func (c NewsClient) Article(ctx context.Context, req NewsArticleRequest) (NewsArticle, error) {
	return c.engine.NewsArticle(ctx, req)
}
func (c NewsClient) Historical(ctx context.Context, req HistoricalNewsRequest) ([]HistoricalNewsItem, error) {
	return c.engine.HistoricalNews(ctx, req)
}
func (c NewsClient) SubscribeBulletins(ctx context.Context, allMessages bool, opts ...SubscriptionOption) (*Subscription[NewsBulletin], error) {
	return c.engine.SubscribeNewsBulletins(ctx, allMessages, opts...)
}

type ScannerClient struct{ engine *engine }

func (c ScannerClient) Parameters(ctx context.Context) (XMLDocument, error) {
	data, err := c.engine.ScannerParameters(ctx)
	return XMLDocument(data), err
}
func (c ScannerClient) SubscribeResults(ctx context.Context, req ScannerSubscriptionRequest, opts ...SubscriptionOption) (*Subscription[[]ScannerResult], error) {
	return c.engine.SubscribeScannerResults(ctx, req, opts...)
}

type AdvisorsClient struct{ engine *engine }

func (c AdvisorsClient) Config(ctx context.Context, dataType FADataType) (XMLDocument, error) {
	data, err := c.engine.RequestFA(ctx, dataType)
	return XMLDocument(data), err
}
func (c AdvisorsClient) ReplaceConfig(ctx context.Context, dataType FADataType, data XMLDocument) error {
	return c.engine.ReplaceFA(ctx, dataType, string(data))
}
func (c AdvisorsClient) SoftDollarTiers(ctx context.Context) ([]SoftDollarTier, error) {
	return c.engine.SoftDollarTiers(ctx)
}

type WSHClient struct{ engine *engine }

func (c WSHClient) MetaData(ctx context.Context) (JSONDocument, error) {
	data, err := c.engine.WSHMetaData(ctx)
	return JSONDocument(data), err
}
func (c WSHClient) EventData(ctx context.Context, req WSHEventDataRequest) (JSONDocument, error) {
	data, err := c.engine.WSHEventData(ctx, req)
	return JSONDocument(data), err
}

type TWSClient struct{ engine *engine }

func (c TWSClient) UserInfo(ctx context.Context) (string, error) {
	return c.engine.UserInfo(ctx)
}
func (c TWSClient) DisplayGroups(ctx context.Context) ([]DisplayGroupID, error) {
	groups, err := c.engine.QueryDisplayGroups(ctx)
	if err != nil {
		return nil, err
	}
	return parseDisplayGroups(groups)
}
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

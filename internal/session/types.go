package session

import (
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
)

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
}

type Contract struct {
	ConID           int
	Symbol          string
	SecType         string
	Expiry          string
	Strike          string
	Right           string
	Multiplier      string
	Exchange        string
	Currency        string
	LocalSymbol     string
	TradingClass    string
	PrimaryExchange string
}

type ContractDetailsRequest struct {
	Contract Contract
}

type ContractDetails struct {
	Contract   Contract
	MarketName string
	LongName   string
	MinTick    Decimal
	TimeZoneID string
}

type QualifiedContract struct {
	ContractDetails ContractDetails
}

type HistoricalBarsRequest struct {
	Contract   Contract
	EndTime    time.Time
	Duration   time.Duration
	BarSize    time.Duration
	WhatToShow string
	UseRTH     bool
}

type Bar struct {
	Time   time.Time
	Open   Decimal
	High   Decimal
	Low    Decimal
	Close  Decimal
	Volume Decimal
	WAP    Decimal
	Count  int
}

type AccountSummaryRequest struct {
	Account string
	Tags    []string
}

type AccountValue struct {
	Account  string
	Tag      string
	Value    string
	Currency string
}

type AccountSummaryUpdate struct {
	Value AccountValue
}

type Position struct {
	Account  string
	Contract Contract
	Position Decimal
	AvgCost  Decimal
}

type PositionUpdate struct {
	Position Position
}

type QuoteFields uint64

const (
	QuoteFieldBid QuoteFields = 1 << iota
	QuoteFieldAsk
	QuoteFieldLast
	QuoteFieldBidSize
	QuoteFieldAskSize
	QuoteFieldLastSize
	QuoteFieldOpen
	QuoteFieldHigh
	QuoteFieldLow
	QuoteFieldClose
	QuoteFieldMarketDataType
)

type MarketDataType int

type Quote struct {
	Available      QuoteFields
	Bid            Decimal
	Ask            Decimal
	Last           Decimal
	BidSize        Decimal
	AskSize        Decimal
	LastSize       Decimal
	Open           Decimal
	High           Decimal
	Low            Decimal
	Close          Decimal
	MarketDataType MarketDataType
}

type QuoteSubscriptionRequest struct {
	Contract     Contract
	Snapshot     bool
	GenericTicks []string
}

type QuoteUpdate struct {
	Snapshot   Quote
	Changed    QuoteFields
	ReceivedAt time.Time
}

type RealTimeBarsRequest struct {
	Contract   Contract
	WhatToShow string
	UseRTH     bool
}

type OpenOrdersScope string

const (
	OpenOrdersScopeAll    OpenOrdersScope = "all"
	OpenOrdersScopeClient OpenOrdersScope = "client"
	OpenOrdersScopeAuto   OpenOrdersScope = "auto"
)

type OpenOrder struct {
	OrderID   int64
	Account   string
	Contract  Contract
	Action    string
	OrderType string
	Status    string
	Quantity  Decimal
	Filled    Decimal
	Remaining Decimal
}

type OpenOrderUpdate struct {
	Order OpenOrder
}

type ExecutionsRequest struct {
	Account string
	Symbol  string
}

type CommissionReport struct {
	ExecID      string
	Commission  Decimal
	Currency    string
	RealizedPNL Decimal
}

type Execution struct {
	ExecID  string
	Account string
	Symbol  string
	Side    string
	Shares  Decimal
	Price   Decimal
	Time    time.Time
}

type ExecutionUpdate struct {
	Execution  *Execution
	Commission *CommissionReport
}

type FamilyCode struct {
	AccountID  string
	FamilyCode string
}

type DepthExchange struct {
	Exchange        string
	SecType         string
	ListingExch     string
	ServiceDataType string
	AggGroup        int
}

type NewsProvider struct {
	Code string
	Name string
}

type MatchingSymbolsRequest struct {
	Pattern string
}

type MatchingSymbol struct {
	ConID              int
	Symbol             string
	SecType            string
	PrimaryExchange    string
	Currency           string
	DerivativeSecTypes []string
}

type HeadTimestampRequest struct {
	Contract   Contract
	WhatToShow string
	UseRTH     bool
}

type PriceIncrement struct {
	LowEdge   Decimal
	Increment Decimal
}

type MarketRuleResult struct {
	MarketRuleID int
	Increments   []PriceIncrement
}

type CompletedOrderResult struct {
	Contract  Contract
	Action    string
	OrderType string
	Status    string
	Quantity  Decimal
	Filled    Decimal
	Remaining Decimal
}

type AccountUpdateValue struct {
	Key      string
	Value    string
	Currency string
	Account  string
}

type PortfolioUpdate struct {
	Account       string
	Contract      Contract
	Position      Decimal
	MarketPrice   Decimal
	MarketValue   Decimal
	AvgCost       Decimal
	UnrealizedPNL Decimal
	RealizedPNL   Decimal
}

// AccountUpdate is a union event from SubscribeAccountUpdates. Exactly one field is non-nil.
type AccountUpdate struct {
	AccountValue *AccountUpdateValue
	Portfolio    *PortfolioUpdate
}

type AccountUpdatesMultiRequest struct {
	Account   string
	ModelCode string
}

type AccountUpdateMultiValue struct {
	Account   string
	ModelCode string
	Key       string
	Value     string
	Currency  string
}

type PositionsMultiRequest struct {
	Account   string
	ModelCode string
}

type PositionMulti struct {
	Account   string
	ModelCode string
	Contract  Contract
	Position  Decimal
	AvgCost   Decimal
}

type PnLRequest struct {
	Account   string
	ModelCode string
}

type PnLUpdate struct {
	DailyPnL      Decimal
	UnrealizedPnL Decimal
	RealizedPnL   Decimal
}

type PnLSingleRequest struct {
	Account   string
	ModelCode string
	ConID     int
}

type PnLSingleUpdate struct {
	Position      Decimal
	DailyPnL      Decimal
	UnrealizedPnL Decimal
	RealizedPnL   Decimal
	Value         Decimal
}

type TickByTickRequest struct {
	Contract      Contract
	TickType      string // "Last", "AllLast", "BidAsk", "MidPoint"
	NumberOfTicks int
	IgnoreSize    bool
}

type TickByTickData struct {
	Time              time.Time
	TickType          int
	Price             Decimal
	Size              Decimal
	Exchange          string
	SpecialConditions string
	BidPrice          Decimal
	AskPrice          Decimal
	BidSize           Decimal
	AskSize           Decimal
	MidPoint          Decimal
}

type NewsBulletin struct {
	MsgID    int
	MsgType  int
	Headline string
	Source   string
}

type SecDefOptParamsRequest struct {
	UnderlyingSymbol  string
	FutFopExchange    string
	UnderlyingSecType string
	UnderlyingConID   int
}

type SecDefOptParams struct {
	Exchange        string
	UnderlyingConID int
	TradingClass    string
	Multiplier      string
	Expirations     []string
	Strikes         []Decimal
}

type SmartComponent struct {
	BitNumber      int
	ExchangeName   string
	ExchangeLetter string
}

type CalcImpliedVolatilityRequest struct {
	Contract    Contract
	OptionPrice Decimal
	UnderPrice  Decimal
}

type CalcOptionPriceRequest struct {
	Contract   Contract
	Volatility Decimal
	UnderPrice Decimal
}

type OptionComputation struct {
	ImpliedVol Decimal
	Delta      Decimal
	OptPrice   Decimal
	PvDividend Decimal
	Gamma      Decimal
	Vega       Decimal
	Theta      Decimal
	UndPrice   Decimal
}

type HistogramDataRequest struct {
	Contract Contract
	UseRTH   bool
	Period   string
}

type HistogramEntry struct {
	Price Decimal
	Size  Decimal
}

type HistoricalTicksRequest struct {
	Contract      Contract
	StartDateTime string
	EndDateTime   string
	NumberOfTicks int
	WhatToShow    string
	UseRTH        bool
	IgnoreSize    bool
}

type HistoricalTick struct {
	Time  time.Time
	Price Decimal
	Size  Decimal
}

type HistoricalTickBidAsk struct {
	Time     time.Time
	BidPrice Decimal
	AskPrice Decimal
	BidSize  Decimal
	AskSize  Decimal
}

type HistoricalTickLast struct {
	Time              time.Time
	Price             Decimal
	Size              Decimal
	Exchange          string
	SpecialConditions string
}

// HistoricalTicksResult holds the result of a historical ticks request.
// Exactly one of the three slices is populated based on WhatToShow.
type HistoricalTicksResult struct {
	Ticks  []HistoricalTick       // populated for MIDPOINT
	BidAsk []HistoricalTickBidAsk // populated for BID_ASK
	Last   []HistoricalTickLast   // populated for TRADES
}

type NewsArticleRequest struct {
	ProviderCode string
	ArticleID    string
}

type NewsArticle struct {
	ArticleType int
	ArticleText string
}

type HistoricalNewsRequest struct {
	ConID         int
	ProviderCodes string
	StartDate     string
	EndDate       string
	TotalResults  int
}

type HistoricalNewsItem struct {
	Time         time.Time
	ProviderCode string
	ArticleID    string
	Headline     string
}

type ScannerSubscriptionRequest struct {
	NumberOfRows int
	Instrument   string
	LocationCode string
	ScanCode     string
}

type ScannerResult struct {
	Rank       int
	Contract   Contract
	Distance   string
	Benchmark  string
	Projection string
	LegsStr    string
}

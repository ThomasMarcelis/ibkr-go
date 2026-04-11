package codec

type Message interface {
	messageName() string
}

// Contract holds the fields used for contract identification on the wire.
// The full TWS wire contract has 11 fields (conID, symbol, secType, expiry,
// strike, right, multiplier, exchange, currency, localSymbol, tradingClass).
// PrimaryExchange is used by some request/response messages outside the 11-field block.
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

type StartAPI struct {
	ClientID             int
	OptionalCapabilities string
}

func (StartAPI) messageName() string { return "start_api" }

type ServerInfo struct {
	ServerVersion  int
	ConnectionTime string
}

func (ServerInfo) messageName() string { return "server_info" }

type ManagedAccounts struct {
	Accounts []string
}

func (ManagedAccounts) messageName() string { return "managed_accounts" }

type NextValidID struct {
	OrderID int64
}

func (NextValidID) messageName() string { return "next_valid_id" }

type CurrentTime struct {
	Time string
}

func (CurrentTime) messageName() string { return "current_time" }

// CurrentTimeRequest is the outbound reqCurrentTime message (OUT 49). The
// server responds asynchronously with a CurrentTime frame using the same
// numeric msg_id.
type CurrentTimeRequest struct{}

func (CurrentTimeRequest) messageName() string { return "req_current_time" }

// ReqIDsRequest is the outbound reqIds message (OUT 8). The server responds
// with a NextValidID frame (msg_id 9) carrying the next available order ID.
// NumIDs is a legacy parameter kept at 1 in the official EClient.
type ReqIDsRequest struct {
	NumIDs int
}

func (ReqIDsRequest) messageName() string { return "req_ids" }

type APIError struct {
	ReqID                   int
	Code                    int
	Message                 string
	AdvancedOrderRejectJSON string
	ErrorTimeMs             string
}

func (APIError) messageName() string { return "api_error" }

type ContractDetailsRequest struct {
	ReqID    int
	Contract Contract
}

func (ContractDetailsRequest) messageName() string { return "req_contract_details" }

type ContractDetails struct {
	ReqID      int
	Contract   Contract
	MarketName string
	MinTick    string
	LongName   string
	TimeZoneID string
}

func (ContractDetails) messageName() string { return "contract_details" }

type ContractDetailsEnd struct {
	ReqID int
}

func (ContractDetailsEnd) messageName() string { return "contract_details_end" }

type HistoricalBarsRequest struct {
	ReqID        int
	Contract     Contract
	EndDateTime  string
	Duration     string
	BarSize      string
	WhatToShow   string
	UseRTH       bool
	KeepUpToDate bool
}

func (HistoricalBarsRequest) messageName() string { return "req_historical_bars" }

type HistoricalBar struct {
	ReqID  int
	Time   string
	Open   string
	High   string
	Low    string
	Close  string
	Volume string
	WAP    string
	Count  string
}

func (HistoricalBar) messageName() string { return "historical_bar" }

type HistoricalBarsEnd struct {
	ReqID int
}

func (HistoricalBarsEnd) messageName() string { return "historical_bars_end" }

type AccountSummaryRequest struct {
	ReqID   int
	Account string
	Tags    []string
}

func (AccountSummaryRequest) messageName() string { return "req_account_summary" }

type CancelAccountSummary struct {
	ReqID int
}

func (CancelAccountSummary) messageName() string { return "cancel_account_summary" }

type AccountSummaryValue struct {
	ReqID    int
	Account  string
	Tag      string
	Value    string
	Currency string
}

func (AccountSummaryValue) messageName() string { return "account_summary" }

type AccountSummaryEnd struct {
	ReqID int
}

func (AccountSummaryEnd) messageName() string { return "account_summary_end" }

type PositionsRequest struct{}

func (PositionsRequest) messageName() string { return "req_positions" }

type CancelPositions struct{}

func (CancelPositions) messageName() string { return "cancel_positions" }

type Position struct {
	Account  string
	Contract Contract
	Position string
	AvgCost  string
}

func (Position) messageName() string { return "position" }

type PositionEnd struct{}

func (PositionEnd) messageName() string { return "position_end" }

type QuoteRequest struct {
	ReqID        int
	Contract     Contract
	Snapshot     bool
	GenericTicks []string
}

func (QuoteRequest) messageName() string { return "req_quote" }

type CancelQuote struct {
	ReqID int
}

func (CancelQuote) messageName() string { return "cancel_quote" }

type TickPrice struct {
	ReqID    int
	TickType int
	Price    string
	Size     string // companion size from the same frame
	AttrMask int    // tick attrib bitmask
}

func (TickPrice) messageName() string { return "tick_price" }

type TickSize struct {
	ReqID    int
	TickType int
	Size     string
}

func (TickSize) messageName() string { return "tick_size" }

type MarketDataType struct {
	ReqID    int
	DataType int
}

func (MarketDataType) messageName() string { return "market_data_type" }

type TickSnapshotEnd struct {
	ReqID int
}

func (TickSnapshotEnd) messageName() string { return "tick_snapshot_end" }

type RealTimeBarsRequest struct {
	ReqID      int
	Contract   Contract
	WhatToShow string
	UseRTH     bool
}

func (RealTimeBarsRequest) messageName() string { return "req_realtime_bars" }

type CancelRealTimeBars struct {
	ReqID int
}

func (CancelRealTimeBars) messageName() string { return "cancel_realtime_bars" }

type RealTimeBar struct {
	ReqID  int
	Time   string
	Open   string
	High   string
	Low    string
	Close  string
	Volume string
	WAP    string
	Count  string
}

func (RealTimeBar) messageName() string { return "realtime_bar" }

type OpenOrdersRequest struct {
	Scope string
}

func (OpenOrdersRequest) messageName() string { return "req_open_orders" }

type CancelOpenOrders struct{}

func (CancelOpenOrders) messageName() string { return "cancel_open_orders" }

type ComboLeg struct {
	ConID              int
	Ratio              int
	Action             string
	Exchange           string
	OpenClose          string
	ShortSaleSlot      string
	DesignatedLocation string
	ExemptCode         string
}

type TagValue struct {
	Tag   string
	Value string
}

type OrderCondition struct {
	Type          int
	Conjunction   string
	ConID         int
	Exchange      string
	Operator      int
	Value         string
	TriggerMethod int
	SecType       string
	Symbol        string
}

type OpenOrder struct {
	OrderID  int64
	Contract Contract

	// Core order fields (fixed wire positions r[12]-r[19] after contract block).
	Action    string
	Quantity  string // totalQuantity on wire
	OrderType string
	LmtPrice  string
	AuxPrice  string
	TIF       string
	OcaGroup  string
	Account   string

	// Order detail fields (r[20]-r[28]).
	OpenClose             string
	Origin                string
	OrderRef              string
	ClientID              string
	PermID                string
	OutsideRTH            string
	Hidden                string
	DiscretionAmt         string
	GoodAfterTime         string
	ComboLegs             []ComboLeg
	OrderComboLegPrices   []string
	SmartComboRouting     []TagValue
	AlgoStrategy          string
	AlgoParams            []TagValue
	Conditions            []OrderCondition
	ConditionsIgnoreRTH   string
	ConditionsCancelOrder string

	// Status at wire position r[91].
	Status string

	// OrderState margin/commission section (r[92]-r[105]).
	InitMarginBefore     string
	MaintMarginBefore    string
	EquityWithLoanBefore string
	InitMarginChange     string
	MaintMarginChange    string
	EquityWithLoanChange string
	InitMarginAfter      string
	MaintMarginAfter     string
	EquityWithLoanAfter  string
	Commission           string
	MinCommission        string
	MaxCommission        string
	CommissionCurrency   string
	WarningText          string

	// Trailing order-status block (last 9 fields of the message).
	Filled    string
	Remaining string
	ParentID  string
}

func (OpenOrder) messageName() string { return "open_order" }

type OpenOrderEnd struct{}

func (OpenOrderEnd) messageName() string { return "open_order_end" }

type OrderStatus struct {
	OrderID       int64
	Status        string
	Filled        string
	Remaining     string
	AvgFillPrice  string
	PermID        string
	ParentID      string
	LastFillPrice string
	ClientID      string
	WhyHeld       string
	MktCapPrice   string
}

func (OrderStatus) messageName() string { return "order_status" }

type ExecutionsRequest struct {
	ReqID   int
	Account string
	Symbol  string
}

func (ExecutionsRequest) messageName() string { return "req_executions" }

type ExecutionDetail struct {
	ReqID   int
	OrderID int64
	ExecID  string
	Account string
	Symbol  string
	Side    string
	Shares  string
	Price   string
	Time    string
}

func (ExecutionDetail) messageName() string { return "execution_detail" }

type ExecutionsEnd struct {
	ReqID int
}

func (ExecutionsEnd) messageName() string { return "executions_end" }

type CommissionReport struct {
	ExecID      string
	Commission  string
	Currency    string
	RealizedPNL string
}

func (CommissionReport) messageName() string { return "commission_report" }

type TickGeneric struct {
	ReqID    int
	TickType int
	Value    string
}

func (TickGeneric) messageName() string { return "tick_generic" }

type TickString struct {
	ReqID    int
	TickType int
	Value    string
}

func (TickString) messageName() string { return "tick_string" }

type TickReqParams struct {
	ReqID               int
	MinTick             string
	BBOExchange         string
	SnapshotPermissions int
}

func (TickReqParams) messageName() string { return "tick_req_params" }

type ReqMarketDataType struct {
	DataType int
}

func (ReqMarketDataType) messageName() string { return "req_market_data_type" }

type CancelHistoricalData struct {
	ReqID int
}

func (CancelHistoricalData) messageName() string { return "cancel_historical_data" }

type FamilyCodesRequest struct{}

func (FamilyCodesRequest) messageName() string { return "req_family_codes" }

type FamilyCodes struct {
	Codes []FamilyCodeEntry
}

func (FamilyCodes) messageName() string { return "family_codes" }

type FamilyCodeEntry struct {
	AccountID  string
	FamilyCode string
}

type MktDepthExchangesRequest struct{}

func (MktDepthExchangesRequest) messageName() string { return "req_mkt_depth_exchanges" }

type MktDepthExchanges struct {
	Exchanges []DepthExchangeEntry
}

func (MktDepthExchanges) messageName() string { return "mkt_depth_exchanges" }

type DepthExchangeEntry struct {
	Exchange        string
	SecType         string
	ListingExch     string
	ServiceDataType string
	AggGroup        int
}

type NewsProvidersRequest struct{}

func (NewsProvidersRequest) messageName() string { return "req_news_providers" }

type NewsProviders struct {
	Providers []NewsProviderEntry
}

func (NewsProviders) messageName() string { return "news_providers" }

type NewsProviderEntry struct {
	Code string
	Name string
}

type ScannerParametersRequest struct{}

func (ScannerParametersRequest) messageName() string { return "req_scanner_parameters" }

type ScannerParameters struct {
	XML string
}

func (ScannerParameters) messageName() string { return "scanner_parameters" }

type UserInfoRequest struct {
	ReqID int
}

func (UserInfoRequest) messageName() string { return "req_user_info" }

type UserInfo struct {
	ReqID           int
	WhiteBrandingID string
}

func (UserInfo) messageName() string { return "user_info" }

type MatchingSymbolsRequest struct {
	ReqID   int
	Pattern string
}

func (MatchingSymbolsRequest) messageName() string { return "req_matching_symbols" }

type SymbolSample struct {
	ConID              int
	Symbol             string
	SecType            string
	PrimaryExchange    string
	Currency           string
	DerivativeSecTypes []string
	Description        string
	IssuerID           string
}

type MatchingSymbols struct {
	ReqID   int
	Symbols []SymbolSample
}

func (MatchingSymbols) messageName() string { return "matching_symbols" }

type HeadTimestampRequest struct {
	ReqID      int
	Contract   Contract
	WhatToShow string
	UseRTH     bool
}

func (HeadTimestampRequest) messageName() string { return "req_head_timestamp" }

type HeadTimestamp struct {
	ReqID     int
	Timestamp string
}

func (HeadTimestamp) messageName() string { return "head_timestamp" }

type CancelHeadTimestamp struct {
	ReqID int
}

func (CancelHeadTimestamp) messageName() string { return "cancel_head_timestamp" }

type MarketRuleRequest struct {
	MarketRuleID int
}

func (MarketRuleRequest) messageName() string { return "req_market_rule" }

type PriceIncrement struct {
	LowEdge   string
	Increment string
}

type MarketRule struct {
	MarketRuleID int
	Increments   []PriceIncrement
}

func (MarketRule) messageName() string { return "market_rule" }

type CompletedOrdersRequest struct {
	APIOnly bool
}

func (CompletedOrdersRequest) messageName() string { return "req_completed_orders" }

type CompletedOrder struct {
	Contract  Contract
	Action    string
	OrderType string
	Status    string
	Quantity  string
	Filled    string
	Remaining string
}

func (CompletedOrder) messageName() string { return "completed_order" }

type CompletedOrderEnd struct{}

func (CompletedOrderEnd) messageName() string { return "completed_order_end" }

// Account updates (OUT 6 / IN 6,7,8,54)

type AccountUpdatesRequest struct {
	Subscribe bool
	Account   string
}

func (AccountUpdatesRequest) messageName() string { return "req_account_updates" }

type UpdateAccountValue struct {
	Key      string
	Value    string
	Currency string
	Account  string
}

func (UpdateAccountValue) messageName() string { return "update_account_value" }

type UpdatePortfolio struct {
	Contract      Contract
	Position      string
	MarketPrice   string
	MarketValue   string
	AvgCost       string
	UnrealizedPNL string
	RealizedPNL   string
	Account       string
}

func (UpdatePortfolio) messageName() string { return "update_portfolio" }

type UpdateAccountTime struct {
	Timestamp string
}

func (UpdateAccountTime) messageName() string { return "update_account_time" }

type AccountDownloadEnd struct {
	Account string
}

func (AccountDownloadEnd) messageName() string { return "account_download_end" }

// Account updates multi (OUT 76, cancel OUT 77 / IN 73, 74)

type AccountUpdatesMultiRequest struct {
	ReqID     int
	Account   string
	ModelCode string
}

func (AccountUpdatesMultiRequest) messageName() string { return "req_account_updates_multi" }

type CancelAccountUpdatesMulti struct {
	ReqID int
}

func (CancelAccountUpdatesMulti) messageName() string { return "cancel_account_updates_multi" }

type AccountUpdateMultiValue struct {
	ReqID     int
	Account   string
	ModelCode string
	Key       string
	Value     string
	Currency  string
}

func (AccountUpdateMultiValue) messageName() string { return "account_update_multi" }

type AccountUpdateMultiEnd struct {
	ReqID int
}

func (AccountUpdateMultiEnd) messageName() string { return "account_update_multi_end" }

// Positions multi (OUT 74, cancel OUT 75 / IN 71, 72)

type PositionsMultiRequest struct {
	ReqID     int
	Account   string
	ModelCode string
}

func (PositionsMultiRequest) messageName() string { return "req_positions_multi" }

type CancelPositionsMulti struct {
	ReqID int
}

func (CancelPositionsMulti) messageName() string { return "cancel_positions_multi" }

type PositionMulti struct {
	ReqID     int
	Account   string
	ModelCode string
	Contract  Contract
	Position  string
	AvgCost   string
}

func (PositionMulti) messageName() string { return "position_multi" }

type PositionMultiEnd struct {
	ReqID int
}

func (PositionMultiEnd) messageName() string { return "position_multi_end" }

// PnL (OUT 92, cancel OUT 93 / IN 94)

type PnLRequest struct {
	ReqID     int
	Account   string
	ModelCode string
}

func (PnLRequest) messageName() string { return "req_pnl" }

type CancelPnL struct {
	ReqID int
}

func (CancelPnL) messageName() string { return "cancel_pnl" }

type PnLValue struct {
	ReqID         int
	DailyPnL      string
	UnrealizedPnL string
	RealizedPnL   string
}

func (PnLValue) messageName() string { return "pnl" }

// PnL single (OUT 94, cancel OUT 95 / IN 95)

type PnLSingleRequest struct {
	ReqID     int
	Account   string
	ModelCode string
	ConID     int
}

func (PnLSingleRequest) messageName() string { return "req_pnl_single" }

type CancelPnLSingle struct {
	ReqID int
}

func (CancelPnLSingle) messageName() string { return "cancel_pnl_single" }

type PnLSingleValue struct {
	ReqID         int
	Position      string
	DailyPnL      string
	UnrealizedPnL string
	RealizedPnL   string
	Value         string
}

func (PnLSingleValue) messageName() string { return "pnl_single" }

// Tick by tick (OUT 97, cancel OUT 98 / IN 99)

type TickByTickRequest struct {
	ReqID         int
	Contract      Contract
	TickType      string
	NumberOfTicks int
	IgnoreSize    bool
}

func (TickByTickRequest) messageName() string { return "req_tick_by_tick" }

type CancelTickByTick struct {
	ReqID int
}

func (CancelTickByTick) messageName() string { return "cancel_tick_by_tick" }

type TickByTickData struct {
	ReqID             int
	TickType          int
	Time              string
	Price             string
	Size              string
	Exchange          string
	SpecialConditions string
	BidPrice          string
	AskPrice          string
	BidSize           string
	AskSize           string
	MidPoint          string
	// TickAttrib bitmasks
	TickAttribLast   int
	TickAttribBidAsk int
}

func (TickByTickData) messageName() string { return "tick_by_tick" }

// News bulletins (OUT 12, cancel OUT 13 / IN 14)

type NewsBulletinsRequest struct {
	AllMessages bool
}

func (NewsBulletinsRequest) messageName() string { return "req_news_bulletins" }

type CancelNewsBulletins struct{}

func (CancelNewsBulletins) messageName() string { return "cancel_news_bulletins" }

type NewsBulletin struct {
	MsgID    int
	MsgType  int
	Headline string
	Source   string
}

func (NewsBulletin) messageName() string { return "news_bulletin" }

// SecDefOptParams (OUT 78 / IN 75+76)

type SecDefOptParamsRequest struct {
	ReqID             int
	UnderlyingSymbol  string
	FutFopExchange    string
	UnderlyingSecType string
	UnderlyingConID   int
}

func (SecDefOptParamsRequest) messageName() string { return "req_sec_def_opt_params" }

type SecDefOptParamsResponse struct {
	ReqID           int
	Exchange        string
	UnderlyingConID int
	TradingClass    string
	Multiplier      string
	Expirations     []string
	Strikes         []string
}

func (SecDefOptParamsResponse) messageName() string { return "sec_def_opt_params" }

type SecDefOptParamsEnd struct {
	ReqID int
}

func (SecDefOptParamsEnd) messageName() string { return "sec_def_opt_params_end" }

// SmartComponents (OUT 83 / IN 82)

type SmartComponentsRequest struct {
	ReqID       int
	BBOExchange string
}

func (SmartComponentsRequest) messageName() string { return "req_smart_components" }

type SmartComponentEntry struct {
	BitNumber      int
	ExchangeName   string
	ExchangeLetter string
}

type SmartComponentsResponse struct {
	ReqID      int
	Components []SmartComponentEntry
}

func (SmartComponentsResponse) messageName() string { return "smart_components" }

// CalcImpliedVolatility (OUT 54 / cancel OUT 56) / CalcOptionPrice (OUT 55 / cancel OUT 57)

type CalcImpliedVolatilityRequest struct {
	ReqID       int
	Contract    Contract
	OptionPrice string
	UnderPrice  string
}

func (CalcImpliedVolatilityRequest) messageName() string { return "req_calc_implied_volatility" }

type CancelCalcImpliedVolatility struct {
	ReqID int
}

func (CancelCalcImpliedVolatility) messageName() string { return "cancel_calc_implied_volatility" }

type CalcOptionPriceRequest struct {
	ReqID      int
	Contract   Contract
	Volatility string
	UnderPrice string
}

func (CalcOptionPriceRequest) messageName() string { return "req_calc_option_price" }

type CancelCalcOptionPrice struct {
	ReqID int
}

func (CancelCalcOptionPrice) messageName() string { return "cancel_calc_option_price" }

type TickOptionComputation struct {
	ReqID      int
	TickType   int
	TickAttrib int
	ImpliedVol string
	Delta      string
	OptPrice   string
	PvDividend string
	Gamma      string
	Vega       string
	Theta      string
	UndPrice   string
}

func (TickOptionComputation) messageName() string { return "tick_option_computation" }

// HistogramData (OUT 88 / cancel OUT 89 / IN 89)

type HistogramDataRequest struct {
	ReqID    int
	Contract Contract
	UseRTH   bool
	Period   string
}

func (HistogramDataRequest) messageName() string { return "req_histogram_data" }

type CancelHistogramData struct {
	ReqID int
}

func (CancelHistogramData) messageName() string { return "cancel_histogram_data" }

type HistogramDataEntry struct {
	Price string
	Size  string
}

type HistogramDataResponse struct {
	ReqID   int
	Entries []HistogramDataEntry
}

func (HistogramDataResponse) messageName() string { return "histogram_data" }

// HistoricalTicks (OUT 96 / IN 96,97,98)

type HistoricalTicksRequest struct {
	ReqID         int
	Contract      Contract
	StartDateTime string
	EndDateTime   string
	NumberOfTicks int
	WhatToShow    string
	UseRTH        bool
	IgnoreSize    bool
}

func (HistoricalTicksRequest) messageName() string { return "req_historical_ticks" }

type HistoricalTickEntry struct {
	Time  string
	Price string
	Size  string
}

type HistoricalTicksResponse struct {
	ReqID int
	Ticks []HistoricalTickEntry
	Done  bool
}

func (HistoricalTicksResponse) messageName() string { return "historical_ticks" }

type HistoricalTickBidAskEntry struct {
	TickAttrib int
	Time       string
	BidPrice   string
	AskPrice   string
	BidSize    string
	AskSize    string
}

type HistoricalTicksBidAskResponse struct {
	ReqID int
	Ticks []HistoricalTickBidAskEntry
	Done  bool
}

func (HistoricalTicksBidAskResponse) messageName() string { return "historical_ticks_bid_ask" }

type HistoricalTickLastEntry struct {
	TickAttrib        int
	Time              string
	Price             string
	Size              string
	Exchange          string
	SpecialConditions string
}

type HistoricalTicksLastResponse struct {
	ReqID int
	Ticks []HistoricalTickLastEntry
	Done  bool
}

func (HistoricalTicksLastResponse) messageName() string { return "historical_ticks_last" }

// NewsArticle (OUT 84 / IN 83)

type NewsArticleRequest struct {
	ReqID        int
	ProviderCode string
	ArticleID    string
}

func (NewsArticleRequest) messageName() string { return "req_news_article" }

type NewsArticleResponse struct {
	ReqID       int
	ArticleType int
	ArticleText string
}

func (NewsArticleResponse) messageName() string { return "news_article" }

// HistoricalNews (OUT 86 / IN 87+80)

type HistoricalNewsRequest struct {
	ReqID         int
	ConID         int
	ProviderCodes string
	StartDate     string
	EndDate       string
	TotalResults  int
}

func (HistoricalNewsRequest) messageName() string { return "req_historical_news" }

type HistoricalNewsItem struct {
	ReqID        int
	Time         string
	ProviderCode string
	ArticleID    string
	Headline     string
}

func (HistoricalNewsItem) messageName() string { return "historical_news" }

type HistoricalNewsEnd struct {
	ReqID   int
	HasMore bool
}

func (HistoricalNewsEnd) messageName() string { return "historical_news_end" }

// ScannerSubscription (OUT 22 / cancel OUT 23 / IN 20)

type ScannerSubscriptionRequest struct {
	ReqID        int
	NumberOfRows int
	Instrument   string
	LocationCode string
	ScanCode     string
}

func (ScannerSubscriptionRequest) messageName() string { return "req_scanner_subscription" }

type CancelScannerSubscription struct {
	ReqID int
}

func (CancelScannerSubscription) messageName() string { return "cancel_scanner_subscription" }

type ScannerDataEntry struct {
	Rank       int
	Contract   Contract
	Distance   string
	Benchmark  string
	Projection string
	LegsStr    string
}

type ScannerDataResponse struct {
	ReqID   int
	Entries []ScannerDataEntry
}

func (ScannerDataResponse) messageName() string { return "scanner_data" }

// FA Configuration (OUT 18, OUT 19 / IN 16)

type RequestFA struct {
	FADataType int // 1=Groups, 2=Profiles, 3=AccountAliases
}

func (RequestFA) messageName() string { return "req_fa" }

type ReplaceFA struct {
	FADataType int
	XML        string
}

func (ReplaceFA) messageName() string { return "replace_fa" }

type ReceiveFA struct {
	FADataType int
	XML        string
}

func (ReceiveFA) messageName() string { return "receive_fa" }

// SoftDollarTiers (OUT 79 / IN 77)

type SoftDollarTiersRequest struct {
	ReqID int
}

func (SoftDollarTiersRequest) messageName() string { return "req_soft_dollar_tiers" }

type SoftDollarTier struct {
	Name        string
	Value       string
	DisplayName string
}

type SoftDollarTiersResponse struct {
	ReqID int
	Tiers []SoftDollarTier
}

func (SoftDollarTiersResponse) messageName() string { return "soft_dollar_tiers" }

// WSH Calendar Events (OUT 100, cancel OUT 101 / IN 105)
// WSH Event Data (OUT 102, cancel OUT 103 / IN 106)

type WSHMetaDataRequest struct {
	ReqID int
}

func (WSHMetaDataRequest) messageName() string { return "req_wsh_meta_data" }

type CancelWSHMetaData struct {
	ReqID int
}

func (CancelWSHMetaData) messageName() string { return "cancel_wsh_meta_data" }

type WSHEventDataRequest struct {
	ReqID           int
	ConID           int
	Filter          string
	FillWatchlist   bool
	FillPortfolio   bool
	FillCompetitors bool
	StartDate       string
	EndDate         string
	TotalLimit      int
}

func (WSHEventDataRequest) messageName() string { return "req_wsh_event_data" }

type CancelWSHEventData struct {
	ReqID int
}

func (CancelWSHEventData) messageName() string { return "cancel_wsh_event_data" }

type WSHMetaDataResponse struct {
	ReqID    int
	DataJSON string
}

func (WSHMetaDataResponse) messageName() string { return "wsh_meta_data" }

type WSHEventDataResponse struct {
	ReqID    int
	DataJSON string
}

func (WSHEventDataResponse) messageName() string { return "wsh_event_data" }

// HistoricalScheduleResponse is the decoded inbound response to a
// REQ_HISTORICAL_DATA request with whatToShow=SCHEDULE. Each session entry
// describes one contiguous trading window inside the requested duration.
// Live evidence: server_version 200 emits msg_id 106 with 5 header fields
// (reqID, startDateTime, endDateTime, timeZone, sessionCount) followed by
// 3 fields per session.
type HistoricalScheduleResponse struct {
	ReqID         int
	StartDateTime string
	EndDateTime   string
	TimeZone      string
	Sessions      []HistoricalScheduleSession
}

func (HistoricalScheduleResponse) messageName() string { return "historical_schedule" }

// HistoricalScheduleSession describes one trading session entry inside a
// HistoricalScheduleResponse. IBKR emits three string fields per session:
// StartDateTime, EndDateTime, and RefDate (the calendar date the session
// belongs to, useful when a session crosses midnight).
type HistoricalScheduleSession struct {
	StartDateTime string
	EndDateTime   string
	RefDate       string
}

// Display Groups (OUT 67, 68, 69, 70 / IN 67, 68)

type QueryDisplayGroupsRequest struct {
	ReqID int
}

func (QueryDisplayGroupsRequest) messageName() string { return "query_display_groups" }

type SubscribeToGroupEventsRequest struct {
	ReqID   int
	GroupID int
}

func (SubscribeToGroupEventsRequest) messageName() string { return "subscribe_to_group_events" }

type UpdateDisplayGroupRequest struct {
	ReqID        int
	ContractInfo string
}

func (UpdateDisplayGroupRequest) messageName() string { return "update_display_group" }

type UnsubscribeFromGroupEventsRequest struct {
	ReqID int
}

func (UnsubscribeFromGroupEventsRequest) messageName() string { return "unsubscribe_from_group_events" }

type DisplayGroupList struct {
	ReqID  int
	Groups string
}

func (DisplayGroupList) messageName() string { return "display_group_list" }

type DisplayGroupUpdated struct {
	ReqID        int
	ContractInfo string
}

func (DisplayGroupUpdated) messageName() string { return "display_group_updated" }

// PlaceOrder (OUT 3 / IN 3,5) — order management

// PlaceOrderRequest encodes a new or modified order (outbound msg_id=3).
// At server_version >= 145 there is no version field. All fields are strings
// on the wire; UNSET float/int values are encoded as empty string "".
type PlaceOrderRequest struct {
	OrderID  int64
	Contract Contract // 14 wire fields: conId through secId

	// Core order fields
	Action        string // "BUY", "SELL", "SSHORT"
	TotalQuantity string // decimal string
	OrderType     string // "MKT", "LMT", "STP", "STP LMT", "TRAIL", etc.
	LmtPrice      string // empty = UNSET
	AuxPrice      string // empty = UNSET

	// Extended order fields
	TIF                     string // "DAY", "GTC", "IOC", "GTD", "OPG", "FOK", "DTC"
	OcaGroup                string
	Account                 string
	OpenClose               string
	Origin                  string // "0" = customer
	OrderRef                string
	Transmit                string // "0" or "1"
	ParentID                string // "0" = no parent
	BlockOrder              string
	SweepToFill             string
	DisplaySize             string
	TriggerMethod           string
	OutsideRTH              string
	Hidden                  string
	ComboLegs               []ComboLeg
	OrderComboLegPrices     []string
	SmartComboRoutingParams []TagValue

	// FA fields
	FAGroup      string
	FAMethod     string
	FAPercentage string
	ModelCode    string

	// Short sale
	ShortSaleSlot      string
	DesignatedLocation string
	ExemptCode         string // "-1" default

	// Order type extensions
	DiscretionaryAmt              string
	GoodAfterTime                 string
	GoodTillDate                  string
	OcaType                       string
	Rule80A                       string
	SettlingFirm                  string
	AllOrNone                     string
	MinQty                        string // empty = UNSET
	PercentOffset                 string // empty = UNSET
	AuctionStrategy               string
	StartingPrice                 string // empty = UNSET
	StockRefPrice                 string // empty = UNSET
	Delta                         string // empty = UNSET
	StockRangeLower               string // empty = UNSET
	StockRangeUpper               string // empty = UNSET
	OverridePercentageConstraints string

	// Volatility
	Volatility            string // empty = UNSET
	VolatilityType        string // empty = UNSET
	DeltaNeutralOrderType string
	DeltaNeutralAuxPrice  string // empty = UNSET
	ContinuousUpdate      string
	ReferencePriceType    string // empty = UNSET

	// Trailing
	TrailStopPrice  string // empty = UNSET
	TrailingPercent string // empty = UNSET

	// Scale
	ScaleInitLevelSize  string // empty = UNSET
	ScaleSubsLevelSize  string // empty = UNSET
	ScalePriceIncrement string // empty = UNSET
	ScaleTable          string
	ActiveStartTime     string
	ActiveStopTime      string

	// Hedge
	HedgeType  string
	HedgeParam string

	// Misc
	OptOutSmartRouting          string
	ClearingAccount             string
	ClearingIntent              string
	NotHeld                     string
	DeltaNeutralContractPresent string // "0" or "1"
	AlgoStrategy                string
	AlgoParams                  []TagValue
	AlgoID                      string
	WhatIf                      string
	OrderMiscOptions            string
	Solicited                   string
	RandomizeSize               string
	RandomizePrice              string

	// Conditions
	Conditions            []OrderCondition
	ConditionsIgnoreRTH   string
	ConditionsCancelOrder string

	// Adjusted order type
	AdjustedOrderType      string
	TriggerPrice           string // empty = UNSET
	LmtPriceOffset         string // empty = UNSET
	AdjustedStopPrice      string // empty = UNSET
	AdjustedStopLimitPrice string // empty = UNSET
	AdjustedTrailingAmount string // empty = UNSET
	AdjustableTrailingUnit string

	// Ext operator + soft dollar
	ExtOperator     string
	SoftDollarName  string
	SoftDollarValue string

	// Cash, MIFID, flags
	CashQty                     string // empty = UNSET
	Mifid2DecisionMaker         string
	Mifid2DecisionAlgo          string
	Mifid2ExecutionTrader       string
	Mifid2ExecutionAlgo         string
	DontUseAutoPriceForHedge    string
	IsOmsContainer              string
	DiscretionaryUpToLimitPrice string
	UsePriceMgmtAlgo            string // empty = UNSET
	Duration                    string // empty = UNSET
	PostToAts                   string // empty = UNSET
	AutoCancelParent            string
	AdvancedErrorOverride       string
	ManualOrderTime             string
	CustomerAccount             string
	ProfessionalCustomer        string
	IncludeOvernight            string
	ManualOrderIndicator        string // empty = UNSET
	ImbalanceOnly               string
}

func (PlaceOrderRequest) messageName() string { return "place_order" }

// CancelOrderRequest cancels an order (outbound msg_id=4).
// At server_version >= 169, no version field is sent.
type CancelOrderRequest struct {
	OrderID               int64
	ManualOrderCancelTime string
}

func (CancelOrderRequest) messageName() string { return "cancel_order" }

// GlobalCancelRequest cancels all open orders (outbound msg_id=58).
type GlobalCancelRequest struct{}

func (GlobalCancelRequest) messageName() string { return "global_cancel" }

// Market depth (OUT 10, cancel OUT 11 / IN 12, 13)

type MarketDepthRequest struct {
	ReqID        int
	Contract     Contract
	NumRows      int
	IsSmartDepth bool
}

func (MarketDepthRequest) messageName() string { return "req_mkt_depth" }

type CancelMarketDepth struct {
	ReqID int
}

func (CancelMarketDepth) messageName() string { return "cancel_mkt_depth" }

type MarketDepthUpdate struct {
	ReqID     int
	Position  int
	Operation int // 0=insert, 1=update, 2=delete
	Side      int // 0=ask, 1=bid
	Price     string
	Size      string
}

func (MarketDepthUpdate) messageName() string { return "market_depth" }

type MarketDepthL2Update struct {
	ReqID        int
	Position     int
	MarketMaker  string
	Operation    int
	Side         int
	Price        string
	Size         string
	IsSmartDepth bool
}

func (MarketDepthL2Update) messageName() string { return "market_depth_l2" }

// FundamentalData (OUT 52, cancel OUT 53 / IN 51)

type FundamentalDataRequest struct {
	ReqID      int
	Contract   Contract
	ReportType string
}

func (FundamentalDataRequest) messageName() string { return "req_fundamental_data" }

type CancelFundamentalData struct {
	ReqID int
}

func (CancelFundamentalData) messageName() string { return "cancel_fundamental_data" }

type FundamentalDataResponse struct {
	ReqID int
	Data  string
}

func (FundamentalDataResponse) messageName() string { return "fundamental_data" }

// ExerciseOptions (OUT 21)

type ExerciseOptionsRequest struct {
	ReqID            int
	Contract         Contract
	ExerciseAction   int
	ExerciseQuantity int
	Account          string
	Override         int
}

func (ExerciseOptionsRequest) messageName() string { return "exercise_options" }

// Historical data update (IN 108) — streaming bar for keepUpToDate

type HistoricalDataUpdate struct {
	ReqID    int
	BarCount int
	Time     string
	Open     string
	High     string
	Low      string
	Close    string
	Volume   string
	WAP      string
	Count    string
}

func (HistoricalDataUpdate) messageName() string { return "historical_data_update" }

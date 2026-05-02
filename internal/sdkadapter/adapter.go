package sdkadapter

import (
	"context"
	"errors"
	"strconv"
	"time"
)

// Adapter is the Go-owned boundary to the repo-owned C++ SDK adapter. It
// carries copied command and event values only; SDK objects and callback
// lifetimes stay on the native side.
type Adapter interface {
	Connect(context.Context, ConnectRequest) error
	Disconnect() error
	IsConnected() bool
	ServerVersion() int
	ConnectionTime() string
	Submit(context.Context, Command) error
	DrainEvents(context.Context, int) ([]Event, error)
	Close() error
}

type ConnectRequest struct {
	Host      string
	Port      int
	ClientID  int
	Timeout   time.Duration
	QueueSize int
}

type BuildInfo struct {
	AdapterABIVersion string
	SDKAPIVersion     string
	Compiler          string
	ProtobufMode      string
}

type CommandKind string

const (
	CommandCurrentTime                CommandKind = "current_time"
	CommandCurrentTimeMillis          CommandKind = "current_time_millis"
	CommandAccountSummary             CommandKind = "account_summary"
	CommandCancelAccountSummary       CommandKind = "cancel_account_summary"
	CommandAccountUpdates             CommandKind = "account_updates"
	CommandAccountUpdatesMulti        CommandKind = "account_updates_multi"
	CommandCancelAccountUpdatesMulti  CommandKind = "cancel_account_updates_multi"
	CommandContractDetails            CommandKind = "contract_details"
	CommandPositions                  CommandKind = "positions"
	CommandCancelPositions            CommandKind = "cancel_positions"
	CommandPositionsMulti             CommandKind = "positions_multi"
	CommandCancelPositionsMulti       CommandKind = "cancel_positions_multi"
	CommandPnL                        CommandKind = "pnl"
	CommandCancelPnL                  CommandKind = "cancel_pnl"
	CommandPnLSingle                  CommandKind = "pnl_single"
	CommandCancelPnLSingle            CommandKind = "cancel_pnl_single"
	CommandMarketDataType             CommandKind = "market_data_type"
	CommandQuote                      CommandKind = "quote"
	CommandCancelQuote                CommandKind = "cancel_quote"
	CommandRealTimeBars               CommandKind = "real_time_bars"
	CommandCancelRealTimeBars         CommandKind = "cancel_real_time_bars"
	CommandTickByTick                 CommandKind = "tick_by_tick"
	CommandCancelTickByTick           CommandKind = "cancel_tick_by_tick"
	CommandMarketDepth                CommandKind = "market_depth"
	CommandCancelMarketDepth          CommandKind = "cancel_market_depth"
	CommandCalcImpliedVolatility      CommandKind = "calc_implied_volatility"
	CommandCancelCalcImpliedVol       CommandKind = "cancel_calc_implied_volatility"
	CommandCalcOptionPrice            CommandKind = "calc_option_price"
	CommandCancelCalcOptionPrice      CommandKind = "cancel_calc_option_price"
	CommandExerciseOptions            CommandKind = "exercise_options"
	CommandPlaceOrder                 CommandKind = "place_order"
	CommandOpenOrders                 CommandKind = "open_orders"
	CommandCompletedOrders            CommandKind = "completed_orders"
	CommandCancelOrder                CommandKind = "cancel_order"
	CommandGlobalCancel               CommandKind = "global_cancel"
	CommandExecutions                 CommandKind = "executions"
	CommandFamilyCodes                CommandKind = "family_codes"
	CommandMktDepthExchanges          CommandKind = "mkt_depth_exchanges"
	CommandNewsProviders              CommandKind = "news_providers"
	CommandNewsBulletins              CommandKind = "news_bulletins"
	CommandCancelNewsBulletins        CommandKind = "cancel_news_bulletins"
	CommandNewsArticle                CommandKind = "news_article"
	CommandHistoricalNews             CommandKind = "historical_news"
	CommandScannerParameters          CommandKind = "scanner_parameters"
	CommandScannerSubscription        CommandKind = "scanner_subscription"
	CommandCancelScannerSubscription  CommandKind = "cancel_scanner_subscription"
	CommandRequestFA                  CommandKind = "request_fa"
	CommandReplaceFA                  CommandKind = "replace_fa"
	CommandHistoricalData             CommandKind = "historical_data"
	CommandCancelHistoricalData       CommandKind = "cancel_historical_data"
	CommandHistoricalTicks            CommandKind = "historical_ticks"
	CommandCancelHistoricalTicks      CommandKind = "cancel_historical_ticks"
	CommandHeadTimestamp              CommandKind = "head_timestamp"
	CommandCancelHeadTimestamp        CommandKind = "cancel_head_timestamp"
	CommandHistogramData              CommandKind = "histogram_data"
	CommandCancelHistogramData        CommandKind = "cancel_histogram_data"
	CommandWSHMetaData                CommandKind = "wsh_meta_data"
	CommandCancelWSHMetaData          CommandKind = "cancel_wsh_meta_data"
	CommandWSHEventData               CommandKind = "wsh_event_data"
	CommandCancelWSHEventData         CommandKind = "cancel_wsh_event_data"
	CommandUserInfo                   CommandKind = "user_info"
	CommandSoftDollarTiers            CommandKind = "soft_dollar_tiers"
	CommandQueryDisplayGroups         CommandKind = "query_display_groups"
	CommandSubscribeToGroupEvents     CommandKind = "subscribe_to_group_events"
	CommandUpdateDisplayGroup         CommandKind = "update_display_group"
	CommandUnsubscribeFromGroupEvents CommandKind = "unsubscribe_from_group_events"
	CommandMatchingSymbols            CommandKind = "matching_symbols"
	CommandMarketRule                 CommandKind = "market_rule"
	CommandSecDefOptParams            CommandKind = "sec_def_opt_params"
	CommandSmartComponents            CommandKind = "smart_components"
	CommandFundamentalData            CommandKind = "fundamental_data"
	CommandCancelFundamentalData      CommandKind = "cancel_fundamental_data"
)

type Command struct {
	Kind CommandKind

	CurrentTime                CurrentTimeRequest
	AccountSummary             AccountSummaryCommand
	CancelAccountSummary       CancelAccountSummaryCommand
	AccountUpdates             AccountUpdatesCommand
	AccountUpdatesMulti        AccountUpdatesMultiCommand
	CancelAccountUpdatesMulti  CancelAccountUpdatesMultiCommand
	ContractDetails            ContractDetailsCommand
	Positions                  PositionsCommand
	CancelPositions            CancelPositionsCommand
	PositionsMulti             PositionsMultiCommand
	CancelPositionsMulti       CancelPositionsMultiCommand
	PnL                        PnLCommand
	CancelPnL                  CancelPnLCommand
	PnLSingle                  PnLSingleCommand
	CancelPnLSingle            CancelPnLSingleCommand
	MarketDataType             MarketDataTypeCommand
	Quote                      QuoteCommand
	CancelQuote                CancelQuoteCommand
	RealTimeBars               RealTimeBarsCommand
	CancelRealTimeBars         CancelRealTimeBarsCommand
	TickByTick                 TickByTickCommand
	CancelTickByTick           CancelTickByTickCommand
	MarketDepth                MarketDepthCommand
	CancelMarketDepth          CancelMarketDepthCommand
	CalcImpliedVolatility      CalcImpliedVolatilityCommand
	CancelCalcImpliedVol       CancelCalcImpliedVolCommand
	CalcOptionPrice            CalcOptionPriceCommand
	CancelCalcOptionPrice      CancelCalcOptionPriceCommand
	ExerciseOptions            ExerciseOptionsCommand
	PlaceOrder                 PlaceOrderRequest
	OpenOrders                 OpenOrdersCommand
	CompletedOrders            CompletedOrdersCommand
	CancelOrder                CancelOrderCommand
	GlobalCancel               GlobalCancelCommand
	Executions                 ExecutionsCommand
	FamilyCodes                FamilyCodesCommand
	MktDepthExchanges          MktDepthExchangesCommand
	NewsProviders              NewsProvidersCommand
	NewsBulletins              NewsBulletinsCommand
	CancelNewsBulletins        CancelNewsBulletinsCommand
	NewsArticle                NewsArticleCommand
	HistoricalNews             HistoricalNewsCommand
	ScannerParameters          ScannerParametersCommand
	ScannerSubscription        ScannerSubscriptionCommand
	CancelScannerSubscription  CancelScannerSubscriptionCommand
	RequestFA                  RequestFACommand
	ReplaceFA                  ReplaceFACommand
	HistoricalData             HistoricalDataCommand
	CancelHistoricalData       CancelHistoricalDataCommand
	HistoricalTicks            HistoricalTicksCommand
	CancelHistoricalTicks      CancelHistoricalTicksCommand
	HeadTimestamp              HeadTimestampCommand
	CancelHeadTimestamp        CancelHeadTimestampCommand
	HistogramData              HistogramDataCommand
	CancelHistogramData        CancelHistogramDataCommand
	WSHMetaData                WSHMetaDataCommand
	CancelWSHMetaData          CancelWSHMetaDataCommand
	WSHEventData               WSHEventDataCommand
	CancelWSHEventData         CancelWSHEventDataCommand
	UserInfo                   UserInfoCommand
	SoftDollarTiers            SoftDollarTiersCommand
	QueryDisplayGroups         QueryDisplayGroupsCommand
	SubscribeToGroupEvents     SubscribeToGroupEventsCommand
	UpdateDisplayGroup         UpdateDisplayGroupCommand
	UnsubscribeFromGroupEvents UnsubscribeFromGroupEventsCommand
	MatchingSymbols            MatchingSymbolsCommand
	MarketRule                 MarketRuleCommand
	SecDefOptParams            SecDefOptParamsCommand
	SmartComponents            SmartComponentsCommand
	FundamentalData            FundamentalDataCommand
	CancelFundamentalData      CancelFundamentalDataCommand
}

type AccountSummaryCommand struct {
	ReqID int
	Group string
	Tags  []string
}

type CancelAccountSummaryCommand struct {
	ReqID int
}

type AccountUpdatesCommand struct {
	Subscribe bool
	Account   string
}

type AccountUpdatesMultiCommand struct {
	ReqID     int
	Account   string
	ModelCode string
}

type CancelAccountUpdatesMultiCommand struct {
	ReqID int
}

type ContractDetailsCommand struct {
	ReqID    int
	Contract Contract
}

type PositionsCommand struct{}

type CancelPositionsCommand struct{}

type PositionsMultiCommand struct {
	ReqID     int
	Account   string
	ModelCode string
}

type CancelPositionsMultiCommand struct {
	ReqID int
}

type PnLCommand struct {
	ReqID     int
	Account   string
	ModelCode string
}

type CancelPnLCommand struct {
	ReqID int
}

type PnLSingleCommand struct {
	ReqID     int
	Account   string
	ModelCode string
	ConID     int
}

type CancelPnLSingleCommand struct {
	ReqID int
}

type MarketDataTypeCommand struct {
	DataType int
}

type QuoteCommand struct {
	ReqID        int
	Contract     Contract
	Snapshot     bool
	GenericTicks []string
}

type CancelQuoteCommand struct {
	ReqID int
}

type RealTimeBarsCommand struct {
	ReqID      int
	Contract   Contract
	WhatToShow string
	UseRTH     bool
}

type CancelRealTimeBarsCommand struct {
	ReqID int
}

type TickByTickCommand struct {
	ReqID         int
	Contract      Contract
	TickType      string
	NumberOfTicks int
	IgnoreSize    bool
}

type CancelTickByTickCommand struct {
	ReqID int
}

type MarketDepthCommand struct {
	ReqID        int
	Contract     Contract
	NumRows      int
	IsSmartDepth bool
}

type CancelMarketDepthCommand struct {
	ReqID        int
	IsSmartDepth bool
}

type CalcImpliedVolatilityCommand struct {
	ReqID       int
	Contract    Contract
	OptionPrice string
	UnderPrice  string
}

type CancelCalcImpliedVolCommand struct {
	ReqID int
}

type CalcOptionPriceCommand struct {
	ReqID      int
	Contract   Contract
	Volatility string
	UnderPrice string
}

type CancelCalcOptionPriceCommand struct {
	ReqID int
}

type ExerciseOptionsCommand struct {
	ReqID            int
	Contract         Contract
	ExerciseAction   int
	ExerciseQuantity int
	Account          string
	Override         int
}

type OpenOrdersCommand struct {
	Scope string
}

type CompletedOrdersCommand struct {
	APIOnly bool
}

type CancelOrderCommand struct {
	OrderID               int64
	ManualOrderCancelTime string
	ExtOperator           string
	ManualOrderIndicator  string
}

type GlobalCancelCommand struct {
	ExtOperator          string
	ManualOrderIndicator string
}

type ExecutionsCommand struct {
	ReqID   int
	Account string
	Symbol  string
}

type FamilyCodesCommand struct{}

type MktDepthExchangesCommand struct{}

type NewsProvidersCommand struct{}

type NewsBulletinsCommand struct {
	AllMessages bool
}

type CancelNewsBulletinsCommand struct{}

type NewsArticleCommand struct {
	ReqID        int
	ProviderCode string
	ArticleID    string
}

type HistoricalNewsCommand struct {
	ReqID         int
	ConID         int
	ProviderCodes string
	StartDate     string
	EndDate       string
	TotalResults  int
}

type ScannerParametersCommand struct{}

type ScannerSubscriptionCommand struct {
	ReqID        int
	NumberOfRows int
	Instrument   string
	LocationCode string
	ScanCode     string
}

type CancelScannerSubscriptionCommand struct {
	ReqID int
}

type RequestFACommand struct {
	FADataType int
}

type ReplaceFACommand struct {
	ReqID      int
	FADataType int
	XML        string
}

type HistoricalDataCommand struct {
	ReqID        int
	Contract     Contract
	EndDateTime  string
	Duration     string
	BarSize      string
	WhatToShow   string
	UseRTH       bool
	KeepUpToDate bool
}

type CancelHistoricalDataCommand struct {
	ReqID int
}

type HistoricalTicksCommand struct {
	ReqID         int
	Contract      Contract
	StartDateTime string
	EndDateTime   string
	NumberOfTicks int
	WhatToShow    string
	UseRTH        bool
	IgnoreSize    bool
}

type CancelHistoricalTicksCommand struct {
	ReqID int
}

type HeadTimestampCommand struct {
	ReqID      int
	Contract   Contract
	WhatToShow string
	UseRTH     bool
}

type CancelHeadTimestampCommand struct {
	ReqID int
}

type HistogramDataCommand struct {
	ReqID    int
	Contract Contract
	UseRTH   bool
	Period   string
}

type CancelHistogramDataCommand struct {
	ReqID int
}

type WSHMetaDataCommand struct {
	ReqID int
}

type CancelWSHMetaDataCommand struct {
	ReqID int
}

type WSHEventDataCommand struct {
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

type CancelWSHEventDataCommand struct {
	ReqID int
}

type UserInfoCommand struct {
	ReqID int
}

type SoftDollarTiersCommand struct {
	ReqID int
}

type QueryDisplayGroupsCommand struct {
	ReqID int
}

type SubscribeToGroupEventsCommand struct {
	ReqID   int
	GroupID int
}

type UpdateDisplayGroupCommand struct {
	ReqID        int
	ContractInfo string
}

type UnsubscribeFromGroupEventsCommand struct {
	ReqID int
}

type MatchingSymbolsCommand struct {
	ReqID   int
	Pattern string
}

type MarketRuleCommand struct {
	MarketRuleID int
}

type SecDefOptParamsCommand struct {
	ReqID             int
	UnderlyingSymbol  string
	FutFopExchange    string
	UnderlyingSecType string
	UnderlyingConID   int
}

type SmartComponentsCommand struct {
	ReqID       int
	BBOExchange string
}

type FundamentalDataCommand struct {
	ReqID      int
	Contract   Contract
	ReportType string
}

type CancelFundamentalDataCommand struct {
	ReqID int
}

type EventKind string

const (
	EventConnectionMetadata    EventKind = "connection_metadata"
	EventConnectionClosed      EventKind = "connection_closed"
	EventNextValidID           EventKind = "next_valid_id"
	EventManagedAccounts       EventKind = "managed_accounts"
	EventCurrentTime           EventKind = "current_time"
	EventCurrentTimeMillis     EventKind = "current_time_millis"
	EventAccountSummary        EventKind = "account_summary"
	EventAccountSummaryEnd     EventKind = "account_summary_end"
	EventUpdateAccountValue    EventKind = "update_account_value"
	EventUpdatePortfolio       EventKind = "update_portfolio"
	EventUpdateAccountTime     EventKind = "update_account_time"
	EventAccountDownloadEnd    EventKind = "account_download_end"
	EventAccountUpdateMulti    EventKind = "account_update_multi"
	EventAccountUpdateMultiEnd EventKind = "account_update_multi_end"
	EventAPIError              EventKind = "api_error"
	EventAdapterFatal          EventKind = "adapter_fatal"
	EventContractDetails       EventKind = "contract_details"
	EventBondContractDetails   EventKind = "bond_contract_details"
	EventContractDetailsEnd    EventKind = "contract_details_end"
	EventPosition              EventKind = "position"
	EventPositionEnd           EventKind = "position_end"
	EventPositionMulti         EventKind = "position_multi"
	EventPositionMultiEnd      EventKind = "position_multi_end"
	EventPnL                   EventKind = "pnl"
	EventPnLSingle             EventKind = "pnl_single"
	EventOpenOrder             EventKind = "open_order"
	EventOpenOrderEnd          EventKind = "open_order_end"
	EventCompletedOrder        EventKind = "completed_order"
	EventCompletedOrderEnd     EventKind = "completed_order_end"
	EventOrderStatus           EventKind = "order_status"
	EventExecutionDetail       EventKind = "execution_detail"
	EventExecutionsEnd         EventKind = "executions_end"
	EventCommissionReport      EventKind = "commission_report"
	EventMarketDataType        EventKind = "market_data_type"
	EventTickPrice             EventKind = "tick_price"
	EventTickSize              EventKind = "tick_size"
	EventTickGeneric           EventKind = "tick_generic"
	EventTickString            EventKind = "tick_string"
	EventTickReqParams         EventKind = "tick_req_params"
	EventTickSnapshotEnd       EventKind = "tick_snapshot_end"
	EventRealTimeBar           EventKind = "real_time_bar"
	EventTickByTick            EventKind = "tick_by_tick"
	EventMarketDepth           EventKind = "market_depth"
	EventMarketDepthL2         EventKind = "market_depth_l2"
	EventTickOptionComputation EventKind = "tick_option_computation"
	EventFamilyCodes           EventKind = "family_codes"
	EventMktDepthExchanges     EventKind = "mkt_depth_exchanges"
	EventNewsProviders         EventKind = "news_providers"
	EventNewsBulletin          EventKind = "news_bulletin"
	EventNewsArticle           EventKind = "news_article"
	EventHistoricalNews        EventKind = "historical_news"
	EventHistoricalNewsEnd     EventKind = "historical_news_end"
	EventScannerParameters     EventKind = "scanner_parameters"
	EventScannerData           EventKind = "scanner_data"
	EventReceiveFA             EventKind = "receive_fa"
	EventReplaceFAEnd          EventKind = "replace_fa_end"
	EventHistoricalData        EventKind = "historical_data"
	EventHistoricalDataEnd     EventKind = "historical_data_end"
	EventHistoricalDataUpdate  EventKind = "historical_data_update"
	EventHistoricalSchedule    EventKind = "historical_schedule"
	EventHistoricalTicks       EventKind = "historical_ticks"
	EventHistoricalTicksBidAsk EventKind = "historical_ticks_bid_ask"
	EventHistoricalTicksLast   EventKind = "historical_ticks_last"
	EventHeadTimestamp         EventKind = "head_timestamp"
	EventHistogramData         EventKind = "histogram_data"
	EventWSHMetaData           EventKind = "wsh_meta_data"
	EventWSHEventData          EventKind = "wsh_event_data"
	EventUserInfo              EventKind = "user_info"
	EventSoftDollarTiers       EventKind = "soft_dollar_tiers"
	EventDisplayGroupList      EventKind = "display_group_list"
	EventDisplayGroupUpdated   EventKind = "display_group_updated"
	EventMatchingSymbols       EventKind = "matching_symbols"
	EventMarketRule            EventKind = "market_rule"
	EventSecDefOptParams       EventKind = "sec_def_opt_params"
	EventSecDefOptParamsEnd    EventKind = "sec_def_opt_params_end"
	EventSmartComponents       EventKind = "smart_components"
	EventFundamentalData       EventKind = "fundamental_data"
)

type Event struct {
	Kind EventKind

	ReqID int

	ServerVersion  int
	ConnectionTime string
	NextValidID    int64
	Accounts       []string
	CurrentTime    int64

	AccountSummary           AccountSummaryValue
	AccountValue             AccountValueEvent
	Portfolio                PortfolioValueEvent
	AccountTime              string
	AccountDownloadEnd       string
	AccountUpdateMulti       AccountUpdateMultiEvent
	ContractDetails          ContractDetailsValue
	Position                 PositionValue
	PositionMulti            PositionMultiEvent
	PnL                      PnLEvent
	PnLSingle                PnLSingleEvent
	OpenOrder                OpenOrder
	CompletedOrder           CompletedOrder
	OrderStatus              OrderStatusValue
	ExecutionDetail          ExecutionDetailValue
	CommissionReport         CommissionReportValue
	MarketDataType           int
	TickPrice                TickPriceValue
	TickSize                 TickSizeValue
	TickGeneric              TickGenericValue
	TickString               TickStringValue
	TickReqParams            TickReqParamsValue
	RealTimeBar              HistoricalBarValue
	TickByTick               TickByTickValue
	MarketDepth              MarketDepthValue
	MarketDepthL2            MarketDepthL2Value
	TickOptionComputation    TickOptionComputationValue
	FamilyCodes              []FamilyCodeValue
	DepthExchanges           []DepthExchangeValue
	NewsProviders            []NewsProviderValue
	NewsBulletin             NewsBulletinEvent
	NewsArticle              NewsArticleValue
	HistoricalNews           HistoricalNewsValue
	HistoricalHasMore        bool
	ScannerXML               string
	ScannerData              []ScannerDataValue
	ReceiveFA                ReceiveFAValue
	ReplaceFAEndText         string
	HistoricalBar            HistoricalBarValue
	HistoricalSchedule       HistoricalScheduleValue
	HistoricalTicks          []HistoricalTickValue
	HistoricalTicksBidAsk    []HistoricalTickBidAskValue
	HistoricalTicksLast      []HistoricalTickLastValue
	HistoricalTicksDone      bool
	HeadTimestamp            string
	HistogramData            []HistogramDataValue
	WSHDataJSON              string
	UserInfo                 UserInfoValue
	SoftDollarTiers          []SoftDollarTierValue
	DisplayGroups            string
	DisplayGroupContractInfo string
	SymbolSamples            []SymbolSampleValue
	MarketRuleID             int
	PriceIncrements          []PriceIncrementValue
	SecDefOptParams          []SecDefOptParamsValue
	SmartComponents          []SmartComponentValue
	FundamentalData          string
	APIError                 Error
	FatalMessage             string
}

type ContractDetailsValue struct {
	Contract   Contract
	MarketName string
	MinTick    string
	LongName   string
	TimeZoneID string
}

type PositionValue struct {
	Account  string
	Contract Contract
	Position string
	AvgCost  string
}

type AccountValueEvent struct {
	Key      string
	Value    string
	Currency string
	Account  string
}

type PortfolioValueEvent struct {
	Account       string
	Contract      Contract
	Position      string
	MarketPrice   string
	MarketValue   string
	AvgCost       string
	UnrealizedPNL string
	RealizedPNL   string
}

type AccountUpdateMultiEvent struct {
	Account   string
	ModelCode string
	Key       string
	Value     string
	Currency  string
}

type PositionMultiEvent struct {
	Account   string
	ModelCode string
	Contract  Contract
	Position  string
	AvgCost   string
}

type PnLEvent struct {
	DailyPnL      string
	UnrealizedPnL string
	RealizedPnL   string
}

type PnLSingleEvent struct {
	Position      string
	DailyPnL      string
	UnrealizedPnL string
	RealizedPnL   string
	Value         string
}

type OrderStatusValue struct {
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

type ExecutionDetailValue struct {
	OrderID int64
	ExecID  string
	Account string
	Symbol  string
	Side    string
	Shares  string
	Price   string
	Time    string
}

type CommissionReportValue struct {
	ExecID      string
	Commission  string
	Currency    string
	RealizedPNL string
}

type TickPriceValue struct {
	TickType int
	Price    string
	Size     string
	AttrMask int
}

type TickSizeValue struct {
	TickType int
	Size     string
}

type TickGenericValue struct {
	TickType int
	Value    string
}

type TickStringValue struct {
	TickType int
	Value    string
}

type TickReqParamsValue struct {
	MinTick             string
	BBOExchange         string
	SnapshotPermissions int
}

type TickOptionComputationValue struct {
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

type FamilyCodeValue struct {
	AccountID  string
	FamilyCode string
}

type DepthExchangeValue struct {
	Exchange        string
	SecType         string
	ListingExch     string
	ServiceDataType string
	AggGroup        int
}

type NewsProviderValue struct {
	Code string
	Name string
}

type NewsBulletinEvent struct {
	MsgID    int
	MsgType  int
	Headline string
	Source   string
}

type NewsArticleValue struct {
	ArticleType int
	ArticleText string
}

type HistoricalNewsValue struct {
	Time         string
	ProviderCode string
	ArticleID    string
	Headline     string
}

type ReceiveFAValue struct {
	FADataType int
	XML        string
}

type HistoricalBarValue struct {
	Time   string
	Open   string
	High   string
	Low    string
	Close  string
	Volume string
	WAP    string
	Count  string
}

type HistoricalScheduleValue struct {
	StartDateTime string
	EndDateTime   string
	TimeZone      string
	Sessions      []HistoricalScheduleSessionValue
}

type HistoricalScheduleSessionValue struct {
	StartDateTime string
	EndDateTime   string
	RefDate       string
}

type HistoricalTickValue struct {
	Time  string
	Price string
	Size  string
}

type HistoricalTickBidAskValue struct {
	TickAttrib int
	Time       string
	BidPrice   string
	AskPrice   string
	BidSize    string
	AskSize    string
}

type HistoricalTickLastValue struct {
	TickAttrib        int
	Time              string
	Price             string
	Size              string
	Exchange          string
	SpecialConditions string
}

type TickByTickValue struct {
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
	TickAttribLast    int
	TickAttribBidAsk  int
}

type MarketDepthValue struct {
	Position  int
	Operation int
	Side      int
	Price     string
	Size      string
}

type MarketDepthL2Value struct {
	Position     int
	MarketMaker  string
	Operation    int
	Side         int
	Price        string
	Size         string
	IsSmartDepth bool
}

type HistogramDataValue struct {
	Price string
	Size  string
}

type ScannerDataValue struct {
	Rank       int
	Contract   Contract
	Distance   string
	Benchmark  string
	Projection string
	LegsStr    string
}

type UserInfoValue struct {
	WhiteBrandingID string
}

type SoftDollarTierValue struct {
	Name        string
	Value       string
	DisplayName string
}

type SymbolSampleValue struct {
	ConID              int
	Symbol             string
	SecType            string
	PrimaryExchange    string
	Currency           string
	DerivativeSecTypes []string
	Description        string
	IssuerID           string
}

type PriceIncrementValue struct {
	LowEdge   string
	Increment string
}

type SecDefOptParamsValue struct {
	Exchange        string
	UnderlyingConID int
	TradingClass    string
	Multiplier      string
	Expirations     []string
	Strikes         []string
}

type SmartComponentValue struct {
	BitNumber      int
	ExchangeName   string
	ExchangeLetter string
}

type Error struct {
	Op                      string
	ReqID                   int
	OrderID                 int64
	Code                    int
	Message                 string
	AdvancedOrderRejectJSON string
	Phase                   string
}

func (e Error) Error() string {
	if e.Op == "" {
		return e.Message
	}
	if e.Code != 0 {
		return e.Op + ": code=" + strconv.Itoa(e.Code) + ": " + e.Message
	}
	return e.Op + ": " + e.Message
}

var (
	ErrClosed             = errors.New("sdkadapter: closed")
	ErrUnsupportedCommand = errors.New("sdkadapter: unsupported command")
)

func CloneEvents(events []Event) []Event {
	out := make([]Event, len(events))
	for i, event := range events {
		out[i] = event
		out[i].Accounts = append([]string(nil), event.Accounts...)
		out[i].FamilyCodes = append([]FamilyCodeValue(nil), event.FamilyCodes...)
		out[i].DepthExchanges = append([]DepthExchangeValue(nil), event.DepthExchanges...)
		out[i].NewsProviders = append([]NewsProviderValue(nil), event.NewsProviders...)
		out[i].HistogramData = append([]HistogramDataValue(nil), event.HistogramData...)
		out[i].ScannerData = append([]ScannerDataValue(nil), event.ScannerData...)
		out[i].SoftDollarTiers = append([]SoftDollarTierValue(nil), event.SoftDollarTiers...)
		out[i].OpenOrder = CloneOpenOrder(event.OpenOrder)
		out[i].SymbolSamples = make([]SymbolSampleValue, len(event.SymbolSamples))
		for j, sample := range event.SymbolSamples {
			out[i].SymbolSamples[j] = sample
			out[i].SymbolSamples[j].DerivativeSecTypes = append([]string(nil), sample.DerivativeSecTypes...)
		}
		out[i].PriceIncrements = append([]PriceIncrementValue(nil), event.PriceIncrements...)
		out[i].SecDefOptParams = make([]SecDefOptParamsValue, len(event.SecDefOptParams))
		for j, params := range event.SecDefOptParams {
			out[i].SecDefOptParams[j] = params
			out[i].SecDefOptParams[j].Expirations = append([]string(nil), params.Expirations...)
			out[i].SecDefOptParams[j].Strikes = append([]string(nil), params.Strikes...)
		}
		out[i].SmartComponents = append([]SmartComponentValue(nil), event.SmartComponents...)
	}
	return out
}

func CloneCommand(command Command) Command {
	command.AccountSummary.Tags = append([]string(nil), command.AccountSummary.Tags...)
	command.Quote.GenericTicks = append([]string(nil), command.Quote.GenericTicks...)
	command.PlaceOrder = ClonePlaceOrderRequest(command.PlaceOrder)
	return command
}

func CloneOpenOrder(order OpenOrder) OpenOrder {
	order.ComboLegs = append([]ComboLeg(nil), order.ComboLegs...)
	order.OrderComboLegPrices = append([]string(nil), order.OrderComboLegPrices...)
	order.SmartComboRouting = append([]TagValue(nil), order.SmartComboRouting...)
	order.AlgoParams = append([]TagValue(nil), order.AlgoParams...)
	order.Conditions = append([]OrderCondition(nil), order.Conditions...)
	return order
}

func ClonePlaceOrderRequest(request PlaceOrderRequest) PlaceOrderRequest {
	request.ComboLegs = append([]ComboLeg(nil), request.ComboLegs...)
	request.OrderComboLegPrices = append([]string(nil), request.OrderComboLegPrices...)
	request.SmartComboRoutingParams = append([]TagValue(nil), request.SmartComboRoutingParams...)
	request.AlgoParams = append([]TagValue(nil), request.AlgoParams...)
	request.Conditions = append([]OrderCondition(nil), request.Conditions...)
	return request
}

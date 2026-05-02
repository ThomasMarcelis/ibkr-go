package sdkadapter

// Contract holds the fields used for contract identification through the SDK adapter.
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

type ServerInfo struct {
	ServerVersion  int
	ConnectionTime string
}

type ManagedAccounts struct {
	Accounts []string
}

type NextValidID struct {
	OrderID int64
}

type CurrentTime struct {
	Time string
}

// CurrentTimeRequest is the outbound reqCurrentTime message (OUT 49). The
// server responds asynchronously with a CurrentTime frame using the same
// numeric msg_id.
type CurrentTimeRequest struct{}

type CurrentTimeMillis struct {
	Time string
}

type CurrentTimeMillisRequest struct{}

// ReqIDsRequest is the outbound reqIds message (OUT 8). The server responds
// with a NextValidID frame (msg_id 9) carrying the next available order ID.
// NumIDs is a legacy parameter kept at 1 in the official EClient.
type ReqIDsRequest struct {
	NumIDs int
}

type APIError struct {
	ReqID                   int
	Code                    int
	Message                 string
	AdvancedOrderRejectJSON string
	ErrorTimeMs             string
}

type ContractDetailsRequest struct {
	ReqID    int
	Contract Contract
}

type ContractDetails struct {
	ReqID      int
	Contract   Contract
	MarketName string
	MinTick    string
	LongName   string
	TimeZoneID string
}

type ContractDetailsEnd struct {
	ReqID int
}

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

type HistoricalBarsEnd struct {
	ReqID int
}

type AccountSummaryRequest struct {
	ReqID   int
	Account string
	Tags    []string
}

type CancelAccountSummary struct {
	ReqID int
}

type AccountSummaryValue struct {
	ReqID    int
	Account  string
	Tag      string
	Value    string
	Currency string
}

type AccountSummaryEnd struct {
	ReqID int
}

type PositionsRequest struct{}

type CancelPositions struct{}

type Position struct {
	Account  string
	Contract Contract
	Position string
	AvgCost  string
}

type PositionEnd struct{}

type QuoteRequest struct {
	ReqID        int
	Contract     Contract
	Snapshot     bool
	GenericTicks []string
}

type CancelQuote struct {
	ReqID int
}

type TickPrice struct {
	ReqID    int
	TickType int
	Price    string
	Size     string // companion size from the same frame
	AttrMask int    // tick attrib bitmask
}

type TickSize struct {
	ReqID    int
	TickType int
	Size     string
}

type MarketDataType struct {
	ReqID    int
	DataType int
}

type TickSnapshotEnd struct {
	ReqID int
}

type RealTimeBarsRequest struct {
	ReqID      int
	Contract   Contract
	WhatToShow string
	UseRTH     bool
}

type CancelRealTimeBars struct {
	ReqID int
}

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

type OpenOrdersRequest struct {
	Scope string
}

type CancelOpenOrders struct{}

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

type OpenOrderEnd struct{}

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

type ExecutionsRequest struct {
	ReqID   int
	Account string
	Symbol  string
}

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

type ExecutionsEnd struct {
	ReqID int
}

type CommissionReport struct {
	ExecID      string
	Commission  string
	Currency    string
	RealizedPNL string
}

type TickGeneric struct {
	ReqID    int
	TickType int
	Value    string
}

type TickString struct {
	ReqID    int
	TickType int
	Value    string
}

type TickReqParams struct {
	ReqID               int
	MinTick             string
	BBOExchange         string
	SnapshotPermissions int
}

type ReqMarketDataType struct {
	DataType int
}

type CancelHistoricalData struct {
	ReqID int
}

type FamilyCodesRequest struct{}

type FamilyCodes struct {
	Codes []FamilyCodeEntry
}

type FamilyCodeEntry struct {
	AccountID  string
	FamilyCode string
}

type MktDepthExchangesRequest struct{}

type MktDepthExchanges struct {
	Exchanges []DepthExchangeEntry
}

type DepthExchangeEntry struct {
	Exchange        string
	SecType         string
	ListingExch     string
	ServiceDataType string
	AggGroup        int
}

type NewsProvidersRequest struct{}

type NewsProviders struct {
	Providers []NewsProviderEntry
}

type NewsProviderEntry struct {
	Code string
	Name string
}

type ScannerParametersRequest struct{}

type ScannerParameters struct {
	XML string
}

type UserInfoRequest struct {
	ReqID int
}

type UserInfo struct {
	ReqID           int
	WhiteBrandingID string
}

type MatchingSymbolsRequest struct {
	ReqID   int
	Pattern string
}

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

type HeadTimestampRequest struct {
	ReqID      int
	Contract   Contract
	WhatToShow string
	UseRTH     bool
}

type HeadTimestamp struct {
	ReqID     int
	Timestamp string
}

type CancelHeadTimestamp struct {
	ReqID int
}

type MarketRuleRequest struct {
	MarketRuleID int
}

type PriceIncrement struct {
	LowEdge   string
	Increment string
}

type MarketRule struct {
	MarketRuleID int
	Increments   []PriceIncrement
}

type CompletedOrdersRequest struct {
	APIOnly bool
}

type CompletedOrder struct {
	Contract  Contract
	Action    string
	OrderType string
	Status    string
	Quantity  string
	Filled    string
	Remaining string
}

type CompletedOrderEnd struct{}

// Account updates (OUT 6 / IN 6,7,8,54)

type AccountUpdatesRequest struct {
	Subscribe bool
	Account   string
}

type UpdateAccountValue struct {
	Key      string
	Value    string
	Currency string
	Account  string
}

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

type UpdateAccountTime struct {
	Timestamp string
}

type AccountDownloadEnd struct {
	Account string
}

// Account updates multi (OUT 76, cancel OUT 77 / IN 73, 74)

type AccountUpdatesMultiRequest struct {
	ReqID     int
	Account   string
	ModelCode string
}

type CancelAccountUpdatesMulti struct {
	ReqID int
}

type AccountUpdateMultiValue struct {
	ReqID     int
	Account   string
	ModelCode string
	Key       string
	Value     string
	Currency  string
}

type AccountUpdateMultiEnd struct {
	ReqID int
}

// Positions multi (OUT 74, cancel OUT 75 / IN 71, 72)

type PositionsMultiRequest struct {
	ReqID     int
	Account   string
	ModelCode string
}

type CancelPositionsMulti struct {
	ReqID int
}

type PositionMulti struct {
	ReqID     int
	Account   string
	ModelCode string
	Contract  Contract
	Position  string
	AvgCost   string
}

type PositionMultiEnd struct {
	ReqID int
}

// PnL (OUT 92, cancel OUT 93 / IN 94)

type PnLRequest struct {
	ReqID     int
	Account   string
	ModelCode string
}

type CancelPnL struct {
	ReqID int
}

type PnLValue struct {
	ReqID         int
	DailyPnL      string
	UnrealizedPnL string
	RealizedPnL   string
}

// PnL single (OUT 94, cancel OUT 95 / IN 95)

type PnLSingleRequest struct {
	ReqID     int
	Account   string
	ModelCode string
	ConID     int
}

type CancelPnLSingle struct {
	ReqID int
}

type PnLSingleValue struct {
	ReqID         int
	Position      string
	DailyPnL      string
	UnrealizedPnL string
	RealizedPnL   string
	Value         string
}

// Tick by tick (OUT 97, cancel OUT 98 / IN 99)

type TickByTickRequest struct {
	ReqID         int
	Contract      Contract
	TickType      string
	NumberOfTicks int
	IgnoreSize    bool
}

type CancelTickByTick struct {
	ReqID int
}

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

// News bulletins (OUT 12, cancel OUT 13 / IN 14)

type NewsBulletinsRequest struct {
	AllMessages bool
}

type CancelNewsBulletins struct{}

type NewsBulletin struct {
	MsgID    int
	MsgType  int
	Headline string
	Source   string
}

// SecDefOptParams (OUT 78 / IN 75+76)

type SecDefOptParamsRequest struct {
	ReqID             int
	UnderlyingSymbol  string
	FutFopExchange    string
	UnderlyingSecType string
	UnderlyingConID   int
}

type SecDefOptParamsResponse struct {
	ReqID           int
	Exchange        string
	UnderlyingConID int
	TradingClass    string
	Multiplier      string
	Expirations     []string
	Strikes         []string
}

type SecDefOptParamsEnd struct {
	ReqID int
}

// SmartComponents (OUT 83 / IN 82)

type SmartComponentsRequest struct {
	ReqID       int
	BBOExchange string
}

type SmartComponentEntry struct {
	BitNumber      int
	ExchangeName   string
	ExchangeLetter string
}

type SmartComponentsResponse struct {
	ReqID      int
	Components []SmartComponentEntry
}

// CalcImpliedVolatility (OUT 54 / cancel OUT 56) / CalcOptionPrice (OUT 55 / cancel OUT 57)

type CalcImpliedVolatilityRequest struct {
	ReqID       int
	Contract    Contract
	OptionPrice string
	UnderPrice  string
}

type CancelCalcImpliedVolatility struct {
	ReqID int
}

type CalcOptionPriceRequest struct {
	ReqID      int
	Contract   Contract
	Volatility string
	UnderPrice string
}

type CancelCalcOptionPrice struct {
	ReqID int
}

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

// HistogramData (OUT 88 / cancel OUT 89 / IN 89)

type HistogramDataRequest struct {
	ReqID    int
	Contract Contract
	UseRTH   bool
	Period   string
}

type CancelHistogramData struct {
	ReqID int
}

type HistogramDataEntry struct {
	Price string
	Size  string
}

type HistogramDataResponse struct {
	ReqID   int
	Entries []HistogramDataEntry
}

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

type CancelHistoricalTicks struct {
	ReqID int
}

// NewsArticle (OUT 84 / IN 83)

type NewsArticleRequest struct {
	ReqID        int
	ProviderCode string
	ArticleID    string
}

type NewsArticleResponse struct {
	ReqID       int
	ArticleType int
	ArticleText string
}

// HistoricalNews (OUT 86 / IN 87+80)

type HistoricalNewsRequest struct {
	ReqID         int
	ConID         int
	ProviderCodes string
	StartDate     string
	EndDate       string
	TotalResults  int
}

type HistoricalNewsItem struct {
	ReqID        int
	Time         string
	ProviderCode string
	ArticleID    string
	Headline     string
}

type HistoricalNewsEnd struct {
	ReqID   int
	HasMore bool
}

// ScannerSubscription (OUT 22 / cancel OUT 23 / IN 20)

type ScannerSubscriptionRequest struct {
	ReqID        int
	NumberOfRows int
	Instrument   string
	LocationCode string
	ScanCode     string
}

type CancelScannerSubscription struct {
	ReqID int
}

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

// FA Configuration (OUT 18, OUT 19 / IN 16)

type RequestFA struct {
	FADataType int // 1=Groups, 2=Profiles, 3=AccountAliases
}

type ReplaceFA struct {
	ReqID      int
	FADataType int
	XML        string
}

type ReceiveFA struct {
	FADataType int
	XML        string
}

type ReplaceFAEnd struct {
	ReqID int
	Text  string
}

// SoftDollarTiers (OUT 79 / IN 77)

type SoftDollarTiersRequest struct {
	ReqID int
}

type SoftDollarTier struct {
	Name        string
	Value       string
	DisplayName string
}

type SoftDollarTiersResponse struct {
	ReqID int
	Tiers []SoftDollarTier
}

// WSH Calendar Events (OUT 100, cancel OUT 101 / IN 105)
// WSH Event Data (OUT 102, cancel OUT 103 / IN 106)

type WSHMetaDataRequest struct {
	ReqID int
}

type CancelWSHMetaData struct {
	ReqID int
}

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

type CancelWSHEventData struct {
	ReqID int
}

type WSHMetaDataResponse struct {
	ReqID    int
	DataJSON string
}

type WSHEventDataResponse struct {
	ReqID    int
	DataJSON string
}

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

type SubscribeToGroupEventsRequest struct {
	ReqID   int
	GroupID int
}

type UpdateDisplayGroupRequest struct {
	ReqID        int
	ContractInfo string
}

type UnsubscribeFromGroupEventsRequest struct {
	ReqID int
}

type DisplayGroupList struct {
	ReqID  int
	Groups string
}

type DisplayGroupUpdated struct {
	ReqID        int
	ContractInfo string
}

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
	DisplaySize             string // always a decimal digit; "0" = unset iceberg display
	TriggerMethod           string // always a decimal digit; "0" = Default
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

// CancelOrderRequest cancels an order (outbound msg_id=4).
// At server_version >= 169 (MANUAL_ORDER_TIME), no version field is sent and
// manualOrderCancelTime is included. At server_version >= 192
// (CME_TAGGING_FIELDS), extOperator and manualOrderIndicator are appended.
type CancelOrderRequest struct {
	OrderID               int64
	ManualOrderCancelTime string
	ExtOperator           string
	ManualOrderIndicator  string // empty = UNSET
}

// GlobalCancelRequest cancels all open orders (outbound msg_id=58).
// At server_version >= 192 (CME_TAGGING_FIELDS), extOperator and
// manualOrderIndicator are sent instead of the legacy version field.
type GlobalCancelRequest struct {
	ExtOperator          string
	ManualOrderIndicator string // empty = UNSET
}

// Market depth (OUT 10, cancel OUT 11 / IN 12, 13)

type MarketDepthRequest struct {
	ReqID        int
	Contract     Contract
	NumRows      int
	IsSmartDepth bool
}

type CancelMarketDepth struct {
	ReqID        int
	IsSmartDepth bool
}

type MarketDepthUpdate struct {
	ReqID     int
	Position  int
	Operation int // 0=insert, 1=update, 2=delete
	Side      int // 0=ask, 1=bid
	Price     string
	Size      string
}

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

// FundamentalData (OUT 52, cancel OUT 53 / IN 51)

type FundamentalDataRequest struct {
	ReqID      int
	Contract   Contract
	ReportType string
}

type CancelFundamentalData struct {
	ReqID int
}

type FundamentalDataResponse struct {
	ReqID int
	Data  string
}

// ExerciseOptions (OUT 21)

type ExerciseOptionsRequest struct {
	ReqID            int
	Contract         Contract
	ExerciseAction   int
	ExerciseQuantity int
	Account          string
	Override         int
}

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

package session

import (
	"context"
	"fmt"
	"sync"
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
	OpFAConfig             OpKind = "fa_config"
	OpSoftDollarTiers      OpKind = "soft_dollar_tiers"
	OpWSHMetaData          OpKind = "wsh_meta_data"
	OpWSHEventData         OpKind = "wsh_event_data"
	OpDisplayGroups        OpKind = "display_groups"
	OpDisplayGroupEvents   OpKind = "display_group_events"
	OpMarketDepth          OpKind = "market_depth"
	OpFundamentalData      OpKind = "fundamental_data"
	OpExerciseOptions      OpKind = "exercise_options"
	OpPlaceOrder           OpKind = "place_order"
	OpCancelOrder          OpKind = "cancel_order"
	OpGlobalCancel         OpKind = "global_cancel"
)

type OrderAction string

const (
	Buy  OrderAction = "BUY"
	Sell OrderAction = "SELL"
)

type TimeInForce string

const (
	TIFDay TimeInForce = "DAY"
	TIFGTC TimeInForce = "GTC"
	TIFIOC TimeInForce = "IOC"
	TIFGTD TimeInForce = "GTD"
	TIFOPG TimeInForce = "OPG"
	TIFFOK TimeInForce = "FOK"
	TIFDTC TimeInForce = "DTC"
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

// SecType identifies the instrument class of a [Contract]. Values mirror the
// IBKR TWS API Contract.secType vocabulary.
type SecType string

const (
	SecTypeStock        SecType = "STK"     // Stock or ETF
	SecTypeOption       SecType = "OPT"     // Option
	SecTypeFuture       SecType = "FUT"     // Future
	SecTypeContFuture   SecType = "CONTFUT" // Continuous future
	SecTypeFutureOption SecType = "FOP"     // Option on a future
	SecTypeIndex        SecType = "IND"     // Index
	SecTypeForex        SecType = "CASH"    // Forex pair
	SecTypeCombo        SecType = "BAG"     // Combo (multi-leg)
	SecTypeBond         SecType = "BOND"    // Bond
	SecTypeBill         SecType = "BILL"    // Treasury bill
	SecTypeCFD          SecType = "CFD"     // Contract for difference
	SecTypeWarrant      SecType = "WAR"     // Warrant
	SecTypeStructured   SecType = "IOPT"    // Structured product / Dutch warrant
	SecTypeForward      SecType = "FWD"     // Forward
	SecTypeCommodity    SecType = "CMDTY"   // Commodity
	SecTypeFund         SecType = "FUND"    // Mutual fund
	SecTypeFixed        SecType = "FIXED"   // Fixed income
	SecTypeSecLending   SecType = "SLB"     // Securities lending / borrowing
	SecTypeNews         SecType = "NEWS"    // News feed
	SecTypeBasket       SecType = "BSK"     // Basket
	SecTypeInterCmdty   SecType = "ICU"     // Inter-commodity spread (unsigned)
	SecTypeInterCmdtyS  SecType = "ICS"     // Inter-commodity spread (signed)
	SecTypeCrypto       SecType = "CRYPTO"  // Cryptocurrency
)

// Right identifies option direction: [RightCall] or [RightPut].
type Right string

const (
	RightCall Right = "C"
	RightPut  Right = "P"
)

type Contract struct {
	ConID           int
	Symbol          string
	SecType         SecType
	Expiry          string
	Strike          string
	Right           Right
	Multiplier      string
	Exchange        string
	Currency        string
	LocalSymbol     string
	TradingClass    string
	PrimaryExchange string
}

type ComboLeg struct {
	ConID              int
	Ratio              int
	Action             string
	Exchange           string
	OpenClose          string
	ShortSaleSlot      int
	DesignatedLocation string
	ExemptCode         int
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
	SecType       SecType
	Symbol        string
}

type ContractDetails struct {
	Contract
	MarketName string
	LongName   string
	MinTick    Decimal
	TimeZoneID string
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

	LmtPrice              Decimal
	AuxPrice              Decimal
	TIF                   string
	OcaGroup              string
	OpenClose             string
	Origin                int
	OrderRef              string
	ClientID              int
	PermID                int64
	OutsideRTH            bool
	Hidden                bool
	GoodAfterTime         string
	ParentID              int64
	ComboLegs             []ComboLeg
	OrderComboLegPrices   []string
	SmartComboRouting     []TagValue
	AlgoStrategy          string
	AlgoParams            []TagValue
	Conditions            []OrderCondition
	ConditionsIgnoreRTH   bool
	ConditionsCancelOrder bool
	Commission            Decimal
	MinCommission         Decimal
	MaxCommission         Decimal
	CommissionCurrency    string
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
	OrderID int64
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

type OrderStatusUpdate struct {
	OrderID       int64
	Status        string
	Filled        Decimal
	Remaining     Decimal
	AvgFillPrice  Decimal
	PermID        int64
	ParentID      int64
	LastFillPrice Decimal
	ClientID      int
	WhyHeld       string
	MktCapPrice   Decimal
}

// OrderEvent is a union event dispatched to per-order handles. Exactly one field is non-nil.
type OrderEvent struct {
	OpenOrder  *OpenOrder
	Status     *OrderStatusUpdate
	Execution  *Execution
	Commission *CommissionReport
}

type Order struct {
	OrderID                 int64 // 0 = auto-allocate
	Action                  OrderAction
	OrderType               string // "MKT", "LMT", "STP", etc.
	Quantity                Decimal
	LmtPrice                Decimal
	AuxPrice                Decimal
	TIF                     TimeInForce
	Account                 string
	Transmit                *bool // nil = true (default)
	ParentID                int64 // 0 = no parent
	OcaGroup                string
	OutsideRTH              bool
	OrderRef                string
	GoodAfterTime           string
	GoodTillDate            string
	ComboLegs               []ComboLeg
	OrderComboLegPrices     []string
	SmartComboRoutingParams []TagValue
	AlgoStrategy            string
	AlgoParams              []TagValue
	Conditions              []OrderCondition
	ConditionsIgnoreRTH     bool
	ConditionsCancelOrder   bool
}

type PlaceOrderRequest struct {
	Contract Contract
	Order    Order
}

// OrderHandle tracks a placed order's lifecycle. Events arrive via Events();
// lifecycle state changes (Gap, Resumed) arrive via State(). Close() detaches
// the handle without cancelling the order. Cancel() sends a cancel request.
type OrderHandle struct {
	orderID int64
	events  chan OrderEvent
	state   *observer[SubscriptionStateEvent]
	done    chan struct{}

	closeOnce sync.Once
	err       error
	errMu     sync.Mutex

	cancelFn func(context.Context) error        // set by engine, sends CancelOrder
	modifyFn func(context.Context, Order) error // set by engine, sends PlaceOrder with same ID
}

func newOrderHandle(orderID int64) *OrderHandle {
	return &OrderHandle{
		orderID: orderID,
		events:  make(chan OrderEvent, 64),
		state:   newObserver[SubscriptionStateEvent](8),
		done:    make(chan struct{}),
	}
}

func (h *OrderHandle) OrderID() int64                       { return h.orderID }
func (h *OrderHandle) Events() <-chan OrderEvent            { return h.events }
func (h *OrderHandle) State() <-chan SubscriptionStateEvent { return h.state.Chan() }
func (h *OrderHandle) Done() <-chan struct{}                { return h.done }

func (h *OrderHandle) Wait() error {
	<-h.done
	h.errMu.Lock()
	defer h.errMu.Unlock()
	return h.err
}

// Close detaches the handle. The order continues executing on the server.
// Events() and State() channels are closed.
func (h *OrderHandle) Close() error {
	h.closeOnce.Do(func() {
		close(h.done)
		close(h.events)
		h.state.Close()
	})
	return nil
}

// Cancel sends a cancel request for this order to the server.
func (h *OrderHandle) Cancel(ctx context.Context) error {
	if h.cancelFn == nil {
		return fmt.Errorf("ibkr: order handle not connected")
	}
	return h.cancelFn(ctx)
}

// Modify sends a modified order (same OrderID) to the server.
func (h *OrderHandle) Modify(ctx context.Context, order Order) error {
	if h.modifyFn == nil {
		return fmt.Errorf("ibkr: order handle not connected")
	}
	return h.modifyFn(ctx, order)
}

func (h *OrderHandle) isDone() bool {
	select {
	case <-h.done:
		return true
	default:
		return false
	}
}

func (h *OrderHandle) emitOrder(o OpenOrder) {
	if h.isDone() {
		return
	}
	select {
	case h.events <- OrderEvent{OpenOrder: &o}:
	case <-h.done:
	}
}

// IsTerminalOrderStatus reports whether a status string represents a final
// order state after which no further updates are expected.
func IsTerminalOrderStatus(status string) bool {
	return status == "Filled" || status == "Cancelled" || status == "Inactive"
}

func (h *OrderHandle) emitStatus(s OrderStatusUpdate) {
	if h.isDone() {
		return
	}
	select {
	case h.events <- OrderEvent{Status: &s}:
	case <-h.done:
	}
	if IsTerminalOrderStatus(s.Status) {
		h.closeWithErr(nil)
	}
}

func (h *OrderHandle) emitExecution(exec Execution) {
	if h.isDone() {
		return
	}
	select {
	case h.events <- OrderEvent{Execution: &exec}:
	case <-h.done:
	}
}

func (h *OrderHandle) emitCommission(cr CommissionReport) {
	if h.isDone() {
		return
	}
	select {
	case h.events <- OrderEvent{Commission: &cr}:
	case <-h.done:
	}
}

func (h *OrderHandle) emitState(evt SubscriptionStateEvent) {
	if h.isDone() {
		return
	}
	if evt.At.IsZero() {
		evt.At = time.Now().UTC()
	}
	h.state.EmitLatest(evt)
}

func (h *OrderHandle) emitOrderError(err error) {
	h.closeWithErr(err)
}

func (h *OrderHandle) closeWithErr(err error) {
	h.closeOnce.Do(func() {
		h.errMu.Lock()
		h.err = err
		h.errMu.Unlock()
		close(h.done)
		close(h.events)
		h.state.Close()
	})
}

type FamilyCode struct {
	AccountID  string
	FamilyCode string
}

type DepthExchange struct {
	Exchange        string
	SecType         SecType
	ListingExch     string
	ServiceDataType string
	AggGroup        int
}

type NewsProvider struct {
	Code string
	Name string
}

type MatchingSymbol struct {
	ConID              int
	Symbol             string
	SecType            SecType
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
	UnderlyingSecType SecType
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
	TickAttrib int
	Time       time.Time
	BidPrice   Decimal
	AskPrice   Decimal
	BidSize    Decimal
	AskSize    Decimal
}

type HistoricalTickLast struct {
	TickAttrib        int
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

type MarketDepthRequest struct {
	Contract     Contract
	NumRows      int
	IsSmartDepth bool
}

type DepthRow struct {
	Position     int
	MarketMaker  string // only populated for L2
	Operation    int    // 0=insert, 1=update, 2=delete
	Side         int    // 0=ask, 1=bid
	Price        Decimal
	Size         Decimal
	IsSmartDepth bool
}

type FundamentalDataRequest struct {
	Contract   Contract
	ReportType string
}

type ExerciseOptionsRequest struct {
	Contract         Contract
	ExerciseAction   int
	ExerciseQuantity int
	Account          string
	Override         bool
}

type SoftDollarTier struct {
	Name        string
	Value       string
	DisplayName string
}

type WSHEventDataRequest struct {
	ConID           int
	Filter          string
	FillWatchlist   bool
	FillPortfolio   bool
	FillCompetitors bool
	StartDate       string
	EndDate         string
	TotalLimit      int
}

type DisplayGroupUpdate struct {
	ContractInfo string
}

// DisplayGroupHandle wraps a display group subscription and exposes an
// Update method that targets the same protocol-level request ID.
type DisplayGroupHandle struct {
	*Subscription[DisplayGroupUpdate]
	updateFn func(context.Context, string) error
}

// Update sends an UpdateDisplayGroup request for this subscription's group.
func (h *DisplayGroupHandle) Update(ctx context.Context, contractInfo string) error {
	return h.updateFn(ctx, contractInfo)
}

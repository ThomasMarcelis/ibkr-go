package ibkr

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/shopspring/decimal"
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
	OpHistoricalSchedule   OpKind = "historical_schedule"
	OpCurrentTime          OpKind = "current_time"
)

type OrderAction string

const (
	Buy  OrderAction = "BUY"
	Sell OrderAction = "SELL"
)

type OrderType string

const (
	OrderTypeMarket          OrderType = "MKT"
	OrderTypeLimit           OrderType = "LMT"
	OrderTypeStop            OrderType = "STP"
	OrderTypeStopLimit       OrderType = "STP LMT"
	OrderTypeMarketOnClose   OrderType = "MOC"
	OrderTypeLimitOnClose    OrderType = "LOC"
	OrderTypeMarketOnOpen    OrderType = "MOO"
	OrderTypeLimitOnOpen     OrderType = "LOO"
	OrderTypeTrailingStop    OrderType = "TRAIL"
	OrderTypeTrailingLimit   OrderType = "TRAIL LIMIT"
	OrderTypeMarketIfTouched OrderType = "MIT"
	OrderTypeLimitIfTouched  OrderType = "LIT"
	OrderTypeMarketToLimit   OrderType = "MTL"
	OrderTypeRelative        OrderType = "REL"
	OrderTypePeggedToMarket  OrderType = "PEG MKT"
	OrderTypePeggedToPrimary OrderType = "PEG PRI"
	OrderTypePeggedToMid     OrderType = "PEG MID"
	OrderTypePeggedToBest    OrderType = "PEG BEST"
	OrderTypePeggedBenchmark OrderType = "PEG BENCH"
)

type OrderStatus string

const (
	OrderStatusPendingSubmit OrderStatus = "PendingSubmit"
	OrderStatusPendingCancel OrderStatus = "PendingCancel"
	OrderStatusPreSubmitted  OrderStatus = "PreSubmitted"
	OrderStatusSubmitted     OrderStatus = "Submitted"
	OrderStatusApiCancelled  OrderStatus = "ApiCancelled"
	OrderStatusCancelled     OrderStatus = "Cancelled"
	OrderStatusFilled        OrderStatus = "Filled"
	OrderStatusInactive      OrderStatus = "Inactive"
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
	Retryable     bool
}

type XMLDocument []byte

type JSONDocument = json.RawMessage

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
	MinTick    decimal.Decimal
	TimeZoneID string
}

type WhatToShow string

const (
	ShowTrades                  WhatToShow = "TRADES"
	ShowMidpoint                WhatToShow = "MIDPOINT"
	ShowBid                     WhatToShow = "BID"
	ShowAsk                     WhatToShow = "ASK"
	ShowBidAsk                  WhatToShow = "BID_ASK"
	ShowHistoricalVolatility    WhatToShow = "HISTORICAL_VOLATILITY"
	ShowOptionImpliedVolatility WhatToShow = "OPTION_IMPLIED_VOLATILITY"
	ShowAdjustedLast            WhatToShow = "ADJUSTED_LAST"
	ShowFeeRate                 WhatToShow = "FEE_RATE"
	ShowYieldBid                WhatToShow = "YIELD_BID"
	ShowYieldAsk                WhatToShow = "YIELD_ASK"
	ShowYieldBidAsk             WhatToShow = "YIELD_BID_ASK"
	ShowYieldLast               WhatToShow = "YIELD_LAST"
	ShowSchedule                WhatToShow = "SCHEDULE"
	ShowAggTrades               WhatToShow = "AGGTRADES"
)

type HistoricalDuration string

func Seconds(n int) HistoricalDuration { return historicalDuration(n, "S") }
func Minutes(n int) HistoricalDuration { return Seconds(n * 60) }
func Hours(n int) HistoricalDuration   { return Seconds(n * 60 * 60) }
func Days(n int) HistoricalDuration    { return historicalDuration(n, "D") }
func Weeks(n int) HistoricalDuration   { return historicalDuration(n, "W") }
func Months(n int) HistoricalDuration  { return historicalDuration(n, "M") }
func Years(n int) HistoricalDuration   { return historicalDuration(n, "Y") }

func historicalDuration(n int, unit string) HistoricalDuration {
	if n <= 0 {
		return ""
	}
	return HistoricalDuration(fmt.Sprintf("%d %s", n, unit))
}

type BarSize string

const (
	Bar1Sec   BarSize = "1 sec"
	Bar5Secs  BarSize = "5 secs"
	Bar10Secs BarSize = "10 secs"
	Bar15Secs BarSize = "15 secs"
	Bar30Secs BarSize = "30 secs"
	Bar1Min   BarSize = "1 min"
	Bar2Mins  BarSize = "2 mins"
	Bar3Mins  BarSize = "3 mins"
	Bar5Mins  BarSize = "5 mins"
	Bar10Mins BarSize = "10 mins"
	Bar15Mins BarSize = "15 mins"
	Bar20Mins BarSize = "20 mins"
	Bar30Mins BarSize = "30 mins"
	Bar1Hour  BarSize = "1 hour"
	Bar2Hours BarSize = "2 hours"
	Bar3Hours BarSize = "3 hours"
	Bar4Hours BarSize = "4 hours"
	Bar8Hours BarSize = "8 hours"
	Bar1Day   BarSize = "1 day"
	Bar1Week  BarSize = "1 week"
	Bar1Month BarSize = "1 month"
)

type HistoricalBarsRequest struct {
	Contract   Contract
	EndTime    time.Time
	Duration   HistoricalDuration
	BarSize    BarSize
	WhatToShow WhatToShow
	UseRTH     bool
}

type Bar struct {
	Time   time.Time
	Open   decimal.Decimal
	High   decimal.Decimal
	Low    decimal.Decimal
	Close  decimal.Decimal
	Volume decimal.Decimal
	WAP    decimal.Decimal
	Count  int
}

// HistoricalScheduleRequest asks the Gateway to return the session schedule
// that would cover a bar request for the given contract and duration. The
// request reuses REQ_HISTORICAL_DATA under the hood with whatToShow=SCHEDULE,
// so Duration and BarSize behave the same as for [History.Bars]. UseRTH is
// respected by the Gateway but the schedule response already encodes the
// regular-hours boundaries per session.
type HistoricalScheduleRequest struct {
	Contract Contract
	EndTime  time.Time
	Duration HistoricalDuration
	BarSize  BarSize
	UseRTH   bool
}

// HistoricalSchedule is the result of [History.Schedule]. StartDateTime,
// EndDateTime, and TimeZone describe the overall window returned by the
// Gateway; Sessions lists the contiguous trading windows inside it.
type HistoricalSchedule struct {
	StartDateTime string
	EndDateTime   string
	TimeZone      string
	Sessions      []HistoricalScheduleSession
}

// HistoricalScheduleSession describes a single trading session returned as
// part of a [HistoricalSchedule]. RefDate is the calendar date the session
// belongs to, which is useful when a session crosses midnight.
type HistoricalScheduleSession struct {
	StartDateTime string
	EndDateTime   string
	RefDate       string
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
	Position decimal.Decimal
	AvgCost  decimal.Decimal
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

const (
	MarketDataLive          MarketDataType = 1
	MarketDataFrozen        MarketDataType = 2
	MarketDataDelayed       MarketDataType = 3
	MarketDataDelayedFrozen MarketDataType = 4
)

func (t MarketDataType) String() string {
	switch t {
	case MarketDataLive:
		return "Live"
	case MarketDataFrozen:
		return "Frozen"
	case MarketDataDelayed:
		return "Delayed"
	case MarketDataDelayedFrozen:
		return "DelayedFrozen"
	default:
		return fmt.Sprintf("MarketDataType(%d)", t)
	}
}

type Quote struct {
	Available      QuoteFields
	Bid            decimal.Decimal
	Ask            decimal.Decimal
	Last           decimal.Decimal
	BidSize        decimal.Decimal
	AskSize        decimal.Decimal
	LastSize       decimal.Decimal
	Open           decimal.Decimal
	High           decimal.Decimal
	Low            decimal.Decimal
	Close          decimal.Decimal
	MarketDataType MarketDataType
}

type GenericTick string

type QuoteRequest struct {
	Contract     Contract
	GenericTicks []GenericTick
}

type QuoteUpdate struct {
	Snapshot   Quote
	Changed    QuoteFields
	ReceivedAt time.Time
}

type RealTimeBarsRequest struct {
	Contract   Contract
	WhatToShow WhatToShow
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
	Action    OrderAction
	OrderType OrderType
	Status    OrderStatus
	Quantity  decimal.Decimal
	Filled    decimal.Decimal
	Remaining decimal.Decimal

	LmtPrice              decimal.Decimal
	AuxPrice              decimal.Decimal
	TIF                   TimeInForce
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
	Commission            decimal.Decimal
	MinCommission         decimal.Decimal
	MaxCommission         decimal.Decimal
	CommissionCurrency    string
}

type OpenOrderUpdate struct {
	Order OpenOrder
}

type ExecutionsRequest struct {
	Account string
	Symbol  string
}

// CommissionReport holds commission details for a trade execution. A zero
// Commission or RealizedPNL is ambiguous: it can mean either a literal zero or
// "not yet computed by the server" (the Java reference client sends an unset
// double sentinel for fields that the server has not filled in). Consumers that
// need to distinguish should correlate with order status or poll executions.
type CommissionReport struct {
	ExecID      string
	Commission  decimal.Decimal
	Currency    string
	RealizedPNL decimal.Decimal
}

type Execution struct {
	OrderID int64
	ExecID  string
	Account string
	Symbol  string
	Side    string
	Shares  decimal.Decimal
	Price   decimal.Decimal
	Time    time.Time
}

type ExecutionUpdate struct {
	Execution  *Execution
	Commission *CommissionReport
}

type OrderStatusUpdate struct {
	OrderID       int64
	Status        OrderStatus
	Filled        decimal.Decimal
	Remaining     decimal.Decimal
	AvgFillPrice  decimal.Decimal
	PermID        int64
	ParentID      int64
	LastFillPrice decimal.Decimal
	ClientID      int
	WhyHeld       string
	MktCapPrice   decimal.Decimal
}

// OrderEvent is a union event dispatched to per-order handles. Exactly one field is non-nil.
type OrderEvent struct {
	OpenOrder  *OpenOrder
	Status     *OrderStatusUpdate
	Execution  *Execution
	Commission *CommissionReport
}

type Order struct {
	OrderID                  int64 // 0 = auto-allocate
	Action                   OrderAction
	OrderType                OrderType
	Quantity                 decimal.Decimal
	LmtPrice                 decimal.Decimal
	AuxPrice                 decimal.Decimal
	TIF                      TimeInForce
	Account                  string
	Transmit                 *bool // nil = true (default)
	ParentID                 int64 // 0 = no parent
	OcaGroup                 string
	OcaType                  int
	OutsideRTH               bool
	TriggerMethod            int
	DisplaySize              int
	OrderRef                 string
	GoodAfterTime            string
	GoodTillDate             string
	AllOrNone                *bool
	MinQty                   decimal.Decimal
	PercentOffset            decimal.Decimal
	TrailStopPrice           decimal.Decimal
	TrailingPercent          decimal.Decimal
	ScaleInitLevelSize       int
	ScaleSubsLevelSize       int
	ScalePriceIncrement      decimal.Decimal
	ScaleTable               string
	ActiveStartTime          string
	ActiveStopTime           string
	HedgeType                string
	HedgeParam               string
	ComboLegs                []ComboLeg
	OrderComboLegPrices      []string
	SmartComboRoutingParams  []TagValue
	AlgoStrategy             string
	AlgoParams               []TagValue
	WhatIf                   *bool
	Conditions               []OrderCondition
	ConditionsIgnoreRTH      bool
	ConditionsCancelOrder    bool
	AdjustedOrderType        OrderType
	TriggerPrice             decimal.Decimal
	LmtPriceOffset           decimal.Decimal
	AdjustedStopPrice        decimal.Decimal
	AdjustedStopLimitPrice   decimal.Decimal
	AdjustedTrailingAmount   decimal.Decimal
	AdjustableTrailingUnit   int
	CashQty                  decimal.Decimal
	DontUseAutoPriceForHedge *bool
	UsePriceMgmtAlgo         *bool
	AdvancedErrorOverride    string
	ManualOrderTime          string
}

type PlaceOrderRequest struct {
	Contract Contract
	Order    Order
}

// OrderHandle tracks a placed order's lifecycle. Events arrive via Events();
// lifecycle state changes (Gap, Resumed) arrive via Lifecycle(). Close() detaches
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
	detachFn func()                             // set by engine, routes Close through the actor loop
}

func newOrderHandle(orderID int64) *OrderHandle {
	return &OrderHandle{
		orderID: orderID,
		events:  make(chan OrderEvent, 64),
		state:   newObserver[SubscriptionStateEvent](8),
		done:    make(chan struct{}),
	}
}

func (h *OrderHandle) OrderID() int64            { return h.orderID }
func (h *OrderHandle) Events() <-chan OrderEvent { return h.events }
func (h *OrderHandle) Lifecycle() <-chan SubscriptionStateEvent {
	return h.state.Chan()
}
func (h *OrderHandle) Done() <-chan struct{} { return h.done }

func (h *OrderHandle) Wait() error {
	<-h.done
	h.errMu.Lock()
	defer h.errMu.Unlock()
	return h.err
}

// Close detaches the handle. The order continues executing on the server.
// Events() and Lifecycle() channels are closed on the engine goroutine,
// serialized with in-flight emits.
func (h *OrderHandle) Close() error {
	if h.detachFn != nil {
		h.detachFn()
		return nil
	}
	// Unbound handles only appear in tests; without an engine route there is no
	// concurrent protocol emitter, so direct teardown is safe.
	h.closeWithErr(nil)
	return nil
}

// Cancel sends a cancel request for this order to the server.
func (h *OrderHandle) Cancel(ctx context.Context) error {
	if h.cancelFn == nil {
		return fmt.Errorf("ibkr: order handle not connected")
	}
	return h.cancelFn(ctx)
}

// Modify sends a modified order to the server. The order is re-sent with the
// handle's bound order ID; the Contract is fixed at placement time. Callers
// may leave order.OrderID at zero for convenience, but setting it to any
// value other than the handle's bound ID is rejected as a misuse rather than
// silently corrected.
func (h *OrderHandle) Modify(ctx context.Context, order Order) error {
	if h.modifyFn == nil {
		return fmt.Errorf("ibkr: order handle not connected")
	}
	if order.OrderID != 0 && order.OrderID != h.orderID {
		return fmt.Errorf("ibkr: modify order: order.OrderID %d does not match handle %d", order.OrderID, h.orderID)
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

func (h *OrderHandle) emitEvent(evt OrderEvent) bool {
	select {
	case <-h.done:
		return false
	default:
	}

	select {
	case h.events <- evt:
		return true
	case <-h.done:
		return false
	default:
		h.closeWithErr(ErrSlowConsumer)
		return false
	}
}

func (h *OrderHandle) emitOrder(o OpenOrder) bool {
	return h.emitEvent(OrderEvent{OpenOrder: &o})
}

// IsTerminalOrderStatus reports whether a status represents a final order
// state. Live Gateway can still deliver execution or commission callbacks just
// after a terminal status, so the engine owns the final handle close.
func IsTerminalOrderStatus(status OrderStatus) bool {
	return status == OrderStatusFilled || status == OrderStatusCancelled || status == OrderStatusInactive
}

func (h *OrderHandle) emitStatus(s OrderStatusUpdate) bool {
	return h.emitEvent(OrderEvent{Status: &s})
}

func (h *OrderHandle) emitExecution(exec Execution) bool {
	return h.emitEvent(OrderEvent{Execution: &exec})
}

func (h *OrderHandle) emitCommission(cr CommissionReport) bool {
	return h.emitEvent(OrderEvent{Commission: &cr})
}

func (h *OrderHandle) emitState(evt SubscriptionStateEvent) {
	if h.isDone() {
		return
	}
	evt.Retryable = retryableSubscriptionState(evt)
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

type NewsProviderCode string

type NewsProvider struct {
	Code NewsProviderCode
	Name string
}

type MatchingSymbol struct {
	ConID              int
	Symbol             string
	SecType            SecType
	PrimaryExchange    string
	Currency           string
	DerivativeSecTypes []string
	Description        string
	IssuerID           string
}

type HeadTimestampRequest struct {
	Contract   Contract
	WhatToShow WhatToShow
	UseRTH     bool
}

type PriceIncrement struct {
	LowEdge   decimal.Decimal
	Increment decimal.Decimal
}

type MarketRuleResult struct {
	MarketRuleID int
	Increments   []PriceIncrement
}

type CompletedOrderResult struct {
	Contract  Contract
	Action    OrderAction
	OrderType OrderType
	Status    OrderStatus
	Quantity  decimal.Decimal
	Filled    decimal.Decimal
	Remaining decimal.Decimal
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
	Position      decimal.Decimal
	MarketPrice   decimal.Decimal
	MarketValue   decimal.Decimal
	AvgCost       decimal.Decimal
	UnrealizedPNL decimal.Decimal
	RealizedPNL   decimal.Decimal
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
	Position  decimal.Decimal
	AvgCost   decimal.Decimal
}

type PnLRequest struct {
	Account   string
	ModelCode string
}

type PnLUpdate struct {
	DailyPnL      decimal.Decimal
	UnrealizedPnL decimal.Decimal
	RealizedPnL   decimal.Decimal
}

type PnLSingleRequest struct {
	Account   string
	ModelCode string
	ConID     int
}

type PnLSingleUpdate struct {
	Position      decimal.Decimal
	DailyPnL      decimal.Decimal
	UnrealizedPnL decimal.Decimal
	RealizedPnL   decimal.Decimal
	Value         decimal.Decimal
}

type TickByTickType string

const (
	TickByTickLast     TickByTickType = "Last"
	TickByTickAllLast  TickByTickType = "AllLast"
	TickByTickBidAsk   TickByTickType = "BidAsk"
	TickByTickMidPoint TickByTickType = "MidPoint"
)

type TickByTickRequest struct {
	Contract      Contract
	TickType      TickByTickType
	NumberOfTicks int
	IgnoreSize    bool
}

type TickByTickData struct {
	Time              time.Time
	TickType          int
	Price             decimal.Decimal
	Size              decimal.Decimal
	Exchange          string
	SpecialConditions string
	BidPrice          decimal.Decimal
	AskPrice          decimal.Decimal
	BidSize           decimal.Decimal
	AskSize           decimal.Decimal
	MidPoint          decimal.Decimal
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
	Strikes         []decimal.Decimal
}

type SmartComponent struct {
	BitNumber      int
	ExchangeName   string
	ExchangeLetter string
}

type CalcImpliedVolatilityRequest struct {
	Contract    Contract
	OptionPrice decimal.Decimal
	UnderPrice  decimal.Decimal
}

type CalcOptionPriceRequest struct {
	Contract   Contract
	Volatility decimal.Decimal
	UnderPrice decimal.Decimal
}

type OptionComputation struct {
	ImpliedVol decimal.Decimal
	Delta      decimal.Decimal
	OptPrice   decimal.Decimal
	PvDividend decimal.Decimal
	Gamma      decimal.Decimal
	Vega       decimal.Decimal
	Theta      decimal.Decimal
	UndPrice   decimal.Decimal
}

type HistogramDataRequest struct {
	Contract Contract
	UseRTH   bool
	Period   string
}

type HistogramEntry struct {
	Price decimal.Decimal
	Size  decimal.Decimal
}

type HistoricalTicksRequest struct {
	Contract      Contract
	StartTime     time.Time
	EndTime       time.Time
	NumberOfTicks int
	WhatToShow    WhatToShow
	UseRTH        bool
	IgnoreSize    bool
}

type HistoricalTick struct {
	Time  time.Time
	Price decimal.Decimal
	Size  decimal.Decimal
}

type HistoricalTickBidAsk struct {
	TickAttrib int
	Time       time.Time
	BidPrice   decimal.Decimal
	AskPrice   decimal.Decimal
	BidSize    decimal.Decimal
	AskSize    decimal.Decimal
}

type HistoricalTickLast struct {
	TickAttrib        int
	Time              time.Time
	Price             decimal.Decimal
	Size              decimal.Decimal
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
	ProviderCode NewsProviderCode
	ArticleID    string
}

type NewsArticle struct {
	ArticleType int
	ArticleText string
}

type HistoricalNewsRequest struct {
	ConID         int
	ProviderCodes []NewsProviderCode
	StartTime     time.Time
	EndTime       time.Time
	TotalResults  int
}

type HistoricalNewsItem struct {
	Time         time.Time
	ProviderCode NewsProviderCode
	ArticleID    string
	Headline     string
}

type ScannerInstrument string
type ScannerLocationCode string
type ScannerCode string

type ScannerSubscriptionRequest struct {
	NumberOfRows int
	Instrument   ScannerInstrument
	LocationCode ScannerLocationCode
	ScanCode     ScannerCode
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

type DepthOperation int

const (
	DepthInsert DepthOperation = 0
	DepthUpdate DepthOperation = 1
	DepthDelete DepthOperation = 2
)

func (o DepthOperation) String() string {
	switch o {
	case DepthInsert:
		return "Insert"
	case DepthUpdate:
		return "Update"
	case DepthDelete:
		return "Delete"
	default:
		return fmt.Sprintf("DepthOperation(%d)", o)
	}
}

type BookSide int

const (
	BookAsk BookSide = 0
	BookBid BookSide = 1
)

func (s BookSide) String() string {
	switch s {
	case BookAsk:
		return "Ask"
	case BookBid:
		return "Bid"
	default:
		return fmt.Sprintf("BookSide(%d)", s)
	}
}

type DepthRow struct {
	Position     int
	MarketMaker  string // only populated for L2
	Operation    DepthOperation
	Side         BookSide
	Price        decimal.Decimal
	Size         decimal.Decimal
	IsSmartDepth bool
}

type FundamentalReportType string

const (
	FundamentalReportSnapshot       FundamentalReportType = "ReportSnapshot"
	FundamentalReportsFinSummary    FundamentalReportType = "ReportsFinSummary"
	FundamentalReportsOwnership     FundamentalReportType = "ReportsOwnership"
	FundamentalReportRatios         FundamentalReportType = "ReportRatios"
	FundamentalReportsFinStatements FundamentalReportType = "ReportsFinStatements"
	FundamentalRESC                 FundamentalReportType = "RESC"
)

type FundamentalDataRequest struct {
	Contract   Contract
	ReportType FundamentalReportType
}

type ExerciseAction int

const (
	Exercise ExerciseAction = 1
	Lapse    ExerciseAction = 2
)

func (a ExerciseAction) String() string {
	switch a {
	case Exercise:
		return "Exercise"
	case Lapse:
		return "Lapse"
	default:
		return fmt.Sprintf("ExerciseAction(%d)", a)
	}
}

type ExerciseOptionsRequest struct {
	Contract         Contract
	ExerciseAction   ExerciseAction
	ExerciseQuantity int
	Account          string
	Override         bool
}

type SoftDollarTier struct {
	Name        string
	Value       string
	DisplayName string
}

type FADataType int

const (
	FADataGroups   FADataType = 1
	FADataProfiles FADataType = 2
	FADataAliases  FADataType = 3
)

func (t FADataType) String() string {
	switch t {
	case FADataGroups:
		return "Groups"
	case FADataProfiles:
		return "Profiles"
	case FADataAliases:
		return "Aliases"
	default:
		return fmt.Sprintf("FADataType(%d)", t)
	}
}

type WSHEventDataRequest struct {
	ConID           int
	Filter          JSONDocument
	FillWatchlist   bool
	FillPortfolio   bool
	FillCompetitors bool
	StartDate       time.Time
	EndDate         time.Time
	TotalLimit      int
}

type DisplayGroupID int

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

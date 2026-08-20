package ibkr

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// OrderAction is the side of an [Order]: buy or sell.
type OrderAction string

const (
	ActionBuy       OrderAction = "BUY"
	ActionSell      OrderAction = "SELL"
	ActionSellShort OrderAction = "SSHORT"
	ActionSellLong  OrderAction = "SLONG"
)

// OrderType is the execution instruction for an [Order]. The type determines
// which price fields the Gateway reads: MKT ignores prices, LMT reads
// [Order.LmtPrice], STP reads [Order.AuxPrice] as the stop trigger, and
// STP LMT reads both.
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

// OrderStatus is the lifecycle state the Gateway reports for an order. See
// [IsTerminalOrderStatus] for which values are final.
type OrderStatus string

const (
	OrderStatusPendingSubmit OrderStatus = "PendingSubmit" // accepted locally, not yet sent to the venue
	OrderStatusPendingCancel OrderStatus = "PendingCancel" // cancel requested, not yet confirmed
	OrderStatusPreSubmitted  OrderStatus = "PreSubmitted"  // held by IBKR pending a trigger or market open
	OrderStatusSubmitted     OrderStatus = "Submitted"     // working at the venue
	OrderStatusAPICancelled  OrderStatus = "ApiCancelled"  // cancelled by an API request
	OrderStatusCancelled     OrderStatus = "Cancelled"     // cancelled
	OrderStatusFilled        OrderStatus = "Filled"        // fully filled
	OrderStatusInactive      OrderStatus = "Inactive"      // rejected or deactivated by IBKR
)

// TimeInForce controls how long an [Order] stays active before it is
// automatically cancelled.
type TimeInForce string

const (
	TIFDay TimeInForce = "DAY" // valid for the current trading day
	TIFGTC TimeInForce = "GTC" // good till cancelled
	TIFIOC TimeInForce = "IOC" // immediate or cancel
	TIFGTD TimeInForce = "GTD" // good till date (see Order.GoodTillDate)
	TIFOPG TimeInForce = "OPG" // at the market open only
	TIFFOK TimeInForce = "FOK" // fill or kill
	TIFDTC TimeInForce = "DTC" // day till cancelled
)

// OpenOrdersScope selects which orders an open-orders request returns.
type OpenOrdersScope string

const (
	OpenOrdersScopeAll    OpenOrdersScope = "all"    // all open orders across every client
	OpenOrdersScopeClient OpenOrdersScope = "client" // only orders placed by this client ID
	OpenOrdersScopeAuto   OpenOrdersScope = "auto"   // persistently bind future manual TWS orders; client ID 0 only, no snapshot
)

// OpenOrder is the typed open_order echo from the Gateway. Order uses the same
// complete broker-echo model as [CompletedOrderResult], while State preserves
// the margin, commission, allocation, and rejection details carried by the
// open-order frame. Live open_order frames carry no fill progress; track fills
// through [OrderStatusUpdate] and executions instead.
type OpenOrder struct {
	Contract Contract
	Order    OrderDetails
	State    OrderState
}

// OpenOrderUpdate is a union event from the open-orders subscription. Exactly
// one field is non-nil. Status carries order-status transitions for orders
// observed through the subscription, including orders that have no live
// OrderHandle in this process (e.g. recovered after a restart).
type OpenOrderUpdate struct {
	Order   *OpenOrder
	Status  *OrderStatusUpdate
	Binding *OrderBinding
}

// OpenOrdersSubscription owns one request-ID-less open-orders stream. Refresh
// requests another snapshot on that exact stream after its current snapshot
// boundary has completed.
type OpenOrdersSubscription struct {
	*Subscription[OpenOrderUpdate]
	refreshFn func(context.Context) error
}

// Refresh requests a fresh snapshot on this subscription. Only one snapshot
// may be in flight. Auto-open-orders streams have no snapshot boundary and
// return [ErrNoSnapshot].
func (s *OpenOrdersSubscription) Refresh(ctx context.Context) error {
	if s == nil || s.Subscription == nil || s.refreshFn == nil {
		return ErrClosed
	}
	return s.refreshFn(ctx)
}

// OrderBinding reports the API order ID assigned when client ID 0 binds a
// manual TWS order through an auto-open-orders subscription.
type OrderBinding struct {
	PermID   int64
	ClientID ClientID
	OrderID  int64
}

// ExecutionFilterSide filters executions by the original order action. It is
// distinct from [ExecutionSide], whose BOT and SLD values describe a fill.
type ExecutionFilterSide string

const (
	ExecutionFilterBuy  ExecutionFilterSide = "BUY"
	ExecutionFilterSell ExecutionFilterSide = "SELL"
)

// ExecutionSide describes which side of a fill executed.
type ExecutionSide string

const (
	ExecutionSideBought ExecutionSide = "BOT"
	ExecutionSideSold   ExecutionSide = "SLD"
)

// ExecutionLiquidity describes how a fill interacted with venue liquidity.
// Unknown integer values are preserved when IBKR extends the protocol.
type ExecutionLiquidity int

const (
	ExecutionLiquidityNone      ExecutionLiquidity = 0
	ExecutionLiquidityAdded     ExecutionLiquidity = 1
	ExecutionLiquidityRemoved   ExecutionLiquidity = 2
	ExecutionLiquidityRoutedOut ExecutionLiquidity = 3
)

// OptionExerciseType reports how an option execution originated. Unknown
// integer values are preserved when IBKR extends the wire enum.
type OptionExerciseType int

const (
	OptionExerciseTypeNone                 OptionExerciseType = 0
	OptionExerciseTypeExercise             OptionExerciseType = 1
	OptionExerciseTypeLapse                OptionExerciseType = 2
	OptionExerciseTypeDoNothing            OptionExerciseType = 3
	OptionExerciseTypeAssigned             OptionExerciseType = 100
	OptionExerciseTypeAutoexerciseClearing OptionExerciseType = 101
	OptionExerciseTypeExpired              OptionExerciseType = 102
	OptionExerciseTypeNetting              OptionExerciseType = 103
	OptionExerciseTypeAutoexerciseTrading  OptionExerciseType = 200
)

// ExecutionsRequest filters an [OrdersClient.Executions] query. The zero value
// selects every execution visible to the current API session; it does not mean
// the account's complete trade history. Since is transmitted in UTC as IBKR's
// time lower bound. SpecificDates use only their calendar components.
type ExecutionsRequest struct {
	ClientID      ClientID // zero disables the client-ID filter
	Account       string
	Since         time.Time
	Symbol        string
	SecType       SecType
	Exchange      string
	Side          ExecutionFilterSide
	LastDays      int         // zero disables the filter; IBKR accepts 1 through 7
	SpecificDates []time.Time // explicit execution dates; requires server_version 200
}

// CommissionAndFeesReport holds IBKR's separate cost report for an execution.
// Decimal pointers are nil when the Gateway sent its unset sentinel; a pointer
// to zero is a computed literal zero. BondYield follows the current IBKR
// protocol name; this package does not invent units the classic API omits.
type CommissionAndFeesReport struct {
	ExecID              string
	Amount              *decimal.Decimal
	Currency            string
	RealizedPnL         *decimal.Decimal
	BondYield           *decimal.Decimal
	YieldRedemptionDate string // YYYYMMDD, or empty when unavailable
}

// ExecutionUpdate is an executions-query event. Exactly one field is non-nil.
// CommissionAndFees reports are separate Gateway callbacks and can arrive
// before their execution or after the executions snapshot boundary.
type ExecutionUpdate struct {
	Execution         *Execution
	CommissionAndFees *CommissionAndFeesReport
}

// ExecutionEvent is one unfiltered execution-related Gateway callback.
// Exactly one payload field is non-nil. RequestID is present only for an
// execution-detail callback; commission-and-fee callbacks have no request ID
// on the wire. Unlike an executions query, this observer does not correlate,
// deduplicate, or discard callbacks.
type ExecutionEvent struct {
	RequestID         *RequestID
	Execution         *Execution
	CommissionAndFees *CommissionAndFeesReport
}

// ExecutionSnapshot is the executions and commission-and-fee reports observed
// through the Gateway's executions-end boundary. The boundary does not promise
// that every fee report has arrived; use [OrdersClient.SubscribeExecutions] to
// keep observing late reports.
type ExecutionSnapshot struct {
	Executions        []Execution
	CommissionAndFees []CommissionAndFeesReport
}

// Execution is a single trade execution report from the Gateway, carrying
// the fill details for one leg of an order.
type Execution struct {
	OrderID                 int64
	Contract                Contract
	ExecID                  string
	Time                    time.Time
	Account                 string
	Exchange                string
	Side                    ExecutionSide
	Shares                  decimal.Decimal
	Price                   decimal.Decimal
	PermID                  int64
	ClientID                ClientID
	Liquidation             int
	CumulativeQuantity      decimal.Decimal
	AveragePrice            decimal.Decimal
	OrderRef                string
	EconomicValueRule       string
	EconomicValueMultiplier *decimal.Decimal
	ModelCode               string
	Liquidity               ExecutionLiquidity
	PriceRevisionPending    bool
	Submitter               string
	OptionExerciseType      OptionExerciseType
}

// OrderStatusUpdate reports a change in an order's fill progress or state.
type OrderStatusUpdate struct {
	OrderID       int64
	Status        OrderStatus
	Filled        decimal.Decimal // quantity filled so far
	Remaining     decimal.Decimal // quantity still working
	AvgFillPrice  decimal.Decimal // average price across all fills
	PermID        int64           // IBKR permanent order ID, stable across sessions
	ParentID      int64           // parent order ID for bracket/child orders, else 0
	LastFillPrice decimal.Decimal // price of the most recent fill
	ClientID      ClientID        // client ID that owns the order
	WhyHeld       string          // reason the order is held (e.g. "locate"), else empty
	MktCapPrice   decimal.Decimal // capped price for price-capped orders
}

// OrderLifecycleKind classifies local write and connection transitions for an
// observed order.
type OrderLifecycleKind string

const (
	OrderStarted          OrderLifecycleKind = "Started"
	OrderGap              OrderLifecycleKind = "Gap"
	OrderRestored         OrderLifecycleKind = "Restored"
	OrderRecoveryRequired OrderLifecycleKind = "RecoveryRequired"
)

// OrderLifecycleEvent reports whether order observation is continuous. A
// RecoveryRequired event means IBKR may have changed the order during a data
// gap. Reconcile open orders, executions, and completed orders for business
// decisions. Replace remains permanently disabled on this handle because that
// reconciliation cannot restore its lost event history; cancellation remains
// safe by stable OrderID. RecoveryRequired uses the prospective replacement
// ConnectionSeq before that connection reaches Ready. Err matches
// [ErrOrderRecoveryRequired].
type OrderLifecycleEvent struct {
	Kind          OrderLifecycleKind
	ConnectionSeq uint64
	Err           error
}

// OrderEvent is a union event dispatched to per-order handles. Exactly one field is non-nil.
type OrderEvent struct {
	OpenOrder         *OpenOrder
	Status            *OrderStatusUpdate
	Execution         *Execution
	CommissionAndFees *CommissionAndFeesReport
	Binding           *OrderBinding
	Lifecycle         *OrderLifecycleEvent
	// Warning is a non-terminal, order-targeted notice (e.g. code 399, the
	// off-hours deferral). The order stays working at IB and the handle stays
	// open; contrast with a terminal failure, which closes the handle and is
	// reported via Wait rather than as an event.
	Warning *APIError
}

// OCAType controls how IBKR handles the remaining orders in a one-cancels-all
// group after one member executes.
type OCAType int

const (
	OCACancelWithBlock    OCAType = 1
	OCAReduceWithBlock    OCAType = 2
	OCAReduceWithoutBlock OCAType = 3
)

// OrderOCA configures membership in a one-cancels-all group. The zero value
// disables OCA behavior.
type OrderOCA struct {
	Group string
	Type  OCAType
}

// OrderCombo holds per-leg prices and routing instructions for a BAG order.
// The contract's leg definitions live in [Contract.ComboLegs].
type OrderCombo struct {
	LegPrices    []*decimal.Decimal
	SmartRouting []TagValue
}

// OrderScale configures scale-order sizing, price increments, and its active
// window. The zero value disables scale behavior.
type OrderScale struct {
	InitialLevelSize    int
	SubsequentLevelSize int
	PriceIncrement      decimal.Decimal
	PriceAdjustValue    *decimal.Decimal
	PriceAdjustInterval *int
	ProfitOffset        *decimal.Decimal
	AutoReset           *bool
	InitialPosition     *int
	InitialFillQty      *int
	RandomPercent       *bool
	Table               string
	ActiveStartTime     string
	ActiveStopTime      string
}

// OrderShortSale carries the locate metadata required by SSHORT and SLONG
// orders. ExemptCode is nil when IBKR's default exemption code should be used.
type OrderShortSale struct {
	Slot               int
	DesignatedLocation string
	ExemptCode         *int
}

// HedgeType selects the relationship between a child hedge order and its
// parent.
type HedgeType string

const (
	HedgeDelta HedgeType = "D"
	HedgeBeta  HedgeType = "B"
	HedgeFX    HedgeType = "F"
	HedgePair  HedgeType = "P"
)

// OrderHedge configures a hedge child. A hedge child commonly has zero
// Quantity because IBKR derives its size from the parent.
type OrderHedge struct {
	Type                  HedgeType
	Param                 string
	DisableAutomaticPrice *bool
	MaxSize               *int // maximum hedge size; requires server_version 223
}

// OrderAlgorithm configures an IB algorithm and its strategy-specific
// parameters. Strategy is intentionally open-ended because IBKR adds and
// entitlement-gates algorithms independently of socket protocol releases.
type OrderAlgorithm struct {
	Strategy string
	Params   []TagValue
}

// OrderConditions configures the triggers that submit or cancel an order.
// The zero value means the order is unconditional.
type OrderConditions struct {
	Values      []OrderCondition
	IgnoreRTH   bool
	CancelOrder bool
}

// OrderAdjustment configures an adjustable order transition. The zero value
// disables adjustment behavior.
type OrderAdjustment struct {
	OrderType      OrderType
	TriggerPrice   decimal.Decimal
	StopPrice      decimal.Decimal
	StopLimitPrice decimal.Decimal
	TrailingAmount decimal.Decimal
	TrailingUnit   int
}

// OrderPeggedBenchmark configures or reports the reference contract and price
// movement relationship for a PEG BENCH order.
type OrderPeggedBenchmark struct {
	ReferenceContractID   ContractID
	ChangeAmountDecrease  bool
	ChangeAmount          decimal.Decimal
	ReferenceChangeAmount *decimal.Decimal
	ReferenceExchangeID   string
}

// Order is the instruction a user fills in to place or replace an order via
// [OrdersClient.Place], [OrdersClient.Preview], or [OrderHandle.Replace]. Only a
// handful of fields are needed for a common order; the rest cover advanced
// order types and are left at their zero value.
//
// Zero-value conventions, derived from the wire encoder:
//
//   - optional decimal fields are pointers: nil omits the value while a
//     non-nil pointer sends it, including literal zero. Set the fields the
//     [OrderType] requires (Quantity always; LmtPrice for LMT and STP LMT;
//     AuxPrice for STP, STP LMT, and as the trailing amount for TRAIL).
//   - *bool fields are tri-state: nil sends the server default, while a
//     non-nil pointer forces true or false. Transmit is the exception — nil
//     defaults to true (transmit immediately); set it to false to stage an
//     untransmitted parent for a bracket.
//   - int and string fields default to their empty value, which the Gateway
//     reads as "not specified".
//
// A minimal market order needs only Action, OrderType, and Quantity. A limit
// order additionally sets LmtPrice.
type Order struct {
	Action                OrderAction           // BUY or SELL (required)
	OrderType             OrderType             // execution instruction (required); selects which price fields apply
	Quantity              decimal.Decimal       // order size (required); zero is treated as unset
	LmtPrice              *decimal.Decimal      // limit price for LMT / STP LMT
	AuxPrice              *decimal.Decimal      // stop trigger for STP / STP LMT, trailing amount for TRAIL
	TIF                   TimeInForce           // time in force; empty defaults to DAY at the server
	Account               string                // account to place under; required only for multi-account logins
	Transmit              *bool                 // nil = transmit (true); false stages an untransmitted order
	ParentID              int64                 // parent order ID for a bracket child; 0 = no parent
	OCA                   OrderOCA              // one-cancels-all behavior; zero value disables it
	OutsideRTH            bool                  // allow execution outside regular trading hours
	IncludeOvernight      bool                  // include IBKR overnight trading in a SMART-routed DAY order
	TriggerMethod         int                   // stop-trigger method; 0 = default
	DisplaySize           int                   // iceberg display size; 0 = show full size
	OrderRef              string                // free-form client order reference/tag
	GoodAfterTime         string                // activate at this time; "YYYYMMDD HH:MM:SS tz"
	GoodTillDate          string                // expiry for TIF GTD; "YYYYMMDD HH:MM:SS tz"
	AllOrNone             *bool                 // nil = server default; require the whole quantity to fill at once
	MinQty                *int                  // minimum fill quantity
	PercentOffset         *decimal.Decimal      // offset percent for REL/pegged orders
	TrailStopPrice        *decimal.Decimal      // trailing-stop trigger price
	TrailingPercent       *decimal.Decimal      // trailing amount as a percent
	LmtPriceOffset        *decimal.Decimal      // limit offset for TRAIL LIMIT and adjustable orders
	Scale                 OrderScale            // scale-order sizing and active window
	Auction               OrderAuction          // auction and PEG BENCH reference-price controls
	ShortSale             OrderShortSale        // locate metadata for SSHORT and SLONG
	Hedge                 OrderHedge            // hedge-child behavior
	Combo                 OrderCombo            // BAG legs, per-leg prices, and smart routing
	Algorithm             OrderAlgorithm        // IB algo strategy and parameters
	Conditions            OrderConditions       // conditional submission/cancellation triggers
	Adjustment            OrderAdjustment       // adjustable-stop transition
	PeggedBenchmark       *OrderPeggedBenchmark // PEG BENCH reference relationship
	CashQty               *decimal.Decimal      // cash quantity for cash-quantity orders
	UsePriceMgmtAlgo      *bool                 // nil = server default; enable IB's price management algo
	AdvancedErrorOverride string                // override token for advanced-order warnings
	ManualOrderTime       string                // manual order entry time for compliance
	Deactivate            bool                  // submit the order inactive; requires server_version 216
	PostOnly              bool                  // reject instead of taking liquidity; requires server_version 216
	AllowPreOpen          bool                  // allow routing in the venue pre-open; requires server_version 216
	IgnoreOpenAuction     bool                  // exclude the opening auction; requires server_version 216
	RouteMarketableToBBO  *bool                 // nil = server default; requires server_version 217
	SeekPriceImprovement  *bool                 // nil = server default; requires server_version 217
	WhatIfType            *int                  // optional IBKR what-if mode; requires server_version 217
}

// PlaceOrderRequest pairs the [Contract] to trade with the [Order] instruction.
type PlaceOrderRequest struct {
	Contract Contract
	Order    Order
}

// PlaceBracketRequest describes a parent order and the two closing children
// that protect it. [OrdersClient.PlaceBracket] assigns all IDs and controls
// ParentID and Transmit; callers leave those fields unset on all three orders.
type PlaceBracketRequest struct {
	Contract   Contract
	Parent     Order
	TakeProfit Order
	StopLoss   Order
}

// BracketOrder contains the independently observable handles created by
// [OrdersClient.PlaceBracket].
type BracketOrder struct {
	Parent     *OrderHandle
	TakeProfit *OrderHandle
	StopLoss   *OrderHandle
}

// OrderState is the margin-and-commission preview returned by
// [OrdersClient.Preview]. Nil monetary fields were omitted by IBKR; non-nil
// zeros are reported zeros.
type OrderState struct {
	Status               OrderStatus
	InitMarginBefore     *decimal.Decimal
	MaintMarginBefore    *decimal.Decimal
	EquityWithLoanBefore *decimal.Decimal
	InitMarginChange     *decimal.Decimal
	MaintMarginChange    *decimal.Decimal
	EquityWithLoanChange *decimal.Decimal
	InitMarginAfter      *decimal.Decimal
	MaintMarginAfter     *decimal.Decimal
	EquityWithLoanAfter  *decimal.Decimal

	CommissionAndFees         *decimal.Decimal
	MinCommissionAndFees      *decimal.Decimal
	MaxCommissionAndFees      *decimal.Decimal
	CommissionAndFeesCurrency string
	MarginCurrency            string

	InitMarginBeforeOutsideRTH     *decimal.Decimal
	MaintMarginBeforeOutsideRTH    *decimal.Decimal
	EquityWithLoanBeforeOutsideRTH *decimal.Decimal
	InitMarginChangeOutsideRTH     *decimal.Decimal
	MaintMarginChangeOutsideRTH    *decimal.Decimal
	EquityWithLoanChangeOutsideRTH *decimal.Decimal
	InitMarginAfterOutsideRTH      *decimal.Decimal
	MaintMarginAfterOutsideRTH     *decimal.Decimal
	EquityWithLoanAfterOutsideRTH  *decimal.Decimal
	SuggestedSize                  *decimal.Decimal
	RejectReason                   string
	Allocations                    []OrderAllocation

	// WarningText carries the Gateway's advisory attached to the preview,
	// e.g. price cap adjustments; empty when the server sent none.
	WarningText string
}

// OrderAllocation is one account allocation returned in a full order preview.
// Decimal and boolean pointers preserve omitted values separately from
// explicit zero and false.
type OrderAllocation struct {
	Account         string
	Position        *decimal.Decimal
	PositionDesired *decimal.Decimal
	PositionAfter   *decimal.Decimal
	DesiredAllocQty *decimal.Decimal
	AllowedAllocQty *decimal.Decimal
	IsMonetary      *bool
}

// CompletedOrderResult is one entry from [OrdersClient.Completed]: the
// contract, complete order snapshot, and terminal completion state returned by
// IBKR. It intentionally does not derive order-status fields such as remaining
// quantity that are absent from the completed-order message.
type CompletedOrderResult struct {
	Contract   Contract
	Order      OrderDetails
	Completion CompletedOrderCompletion
}

// OrderDetails is the complete order specification echoed by open-order and
// completed-order messages. Pointer-valued values distinguish an explicit
// zero or false from a field the broker did not send.
type OrderDetails struct {
	// OrderID, ClientID, and ParentID are nil for classic completed-order
	// replies, whose wire shape does not carry those identities. A non-nil
	// pointer preserves an explicit zero from protobuf replies.
	OrderID          *int64
	ClientID         *ClientID
	ParentID         *int64
	Action           OrderAction
	Quantity         decimal.Decimal
	OrderType        OrderType
	TIF              TimeInForce
	Account          string
	OpenClose        string
	Origin           int
	OrderRef         string
	PermID           *int64
	OutsideRTH       bool
	IncludeOvernight *bool
	Hidden           bool
	GoodAfterTime    string
	GoodTillDate     string
	ModelCode        string
	Transmit         *bool

	Prices                OrderPrices
	OCA                   OrderOCA
	Allocation            OrderAllocationDetails
	Routing               OrderRoutingDetails
	Auction               OrderAuctionDetails
	Execution             OrderExecutionDetails
	Volatility            OrderVolatilityDetails
	Combo                 OrderCombo
	ComboDescription      string
	Scale                 OrderScaleDetails
	Hedge                 OrderHedge
	Algorithm             OrderAlgorithm
	Conditions            OrderConditions
	PeggedBenchmark       *OrderPeggedBenchmark
	Adjustment            OrderAdjustmentDetails
	Compliance            OrderComplianceDetails
	UsePriceMgmtAlgo      *bool
	AdvancedErrorOverride string
	ManualOrderTime       string
	Deactivate            *bool
	PostOnly              *bool
	AllowPreOpen          *bool
	IgnoreOpenAuction     *bool
	SeekPriceImprovement  *bool
	WhatIfType            *int
}

// CompletedOrderCompletion is the terminal state returned with a completed
// order. Time and StatusText preserve IBKR's exact strings; they can contain
// named zones and free-form cancellation or rejection text.
// CommissionAndFees is nil when the completed OrderState omitted an amount;
// the currency may still be present independently.
type CompletedOrderCompletion struct {
	Status                    OrderStatus
	Filled                    decimal.Decimal
	CommissionAndFees         *decimal.Decimal
	CommissionAndFeesCurrency string
	AutoCancelDate            string
	AutoCancelParent          bool
	ParentPermID              *int64
	Time                      string
	StatusText                string
}

// OrderPrices contains price and quantity-like optional values from
// a completed order.
type OrderPrices struct {
	LmtPrice            *decimal.Decimal
	AuxPrice            *decimal.Decimal
	DiscretionaryAmount *decimal.Decimal
	PercentOffset       *decimal.Decimal
	TrailStopPrice      *decimal.Decimal
	TrailingPercent     *decimal.Decimal
	StopPrice           *decimal.Decimal
	LmtPriceOffset      *decimal.Decimal
	CashQty             *decimal.Decimal
}

// OrderAllocationDetails contains the financial-advisor allocation fields.
type OrderAllocationDetails struct {
	Group      string
	Method     string
	Percentage string
}

// OrderRoutingDetails contains short-sale, clearing, and routing metadata.
type OrderRoutingDetails struct {
	Rule80A              string
	SettlingFirm         string
	ShortSaleSlot        int
	DesignatedLocation   string
	ExemptCode           *int
	ClearingAccount      string
	ClearingIntent       string
	NotHeld              bool
	ImbalanceOnly        bool
	RouteMarketableToBBO *bool
}

// OrderAuction contains BOX auction and pegged-to-stock price inputs.
type OrderAuction struct {
	Strategy        int
	StartingPrice   *decimal.Decimal
	StockRefPrice   *decimal.Decimal
	Delta           *decimal.Decimal
	StockRangeLower *decimal.Decimal
	StockRangeUpper *decimal.Decimal
}

// OrderAuctionDetails contains BOX auction and pegged-to-stock price echoes.
type OrderAuctionDetails struct {
	Strategy        *int
	StartingPrice   *decimal.Decimal
	StockRefPrice   *decimal.Decimal
	Delta           *decimal.Decimal
	StockRangeLower *decimal.Decimal
	StockRangeUpper *decimal.Decimal
}

// OrderExecutionDetails contains execution controls and venue competition
// parameters from a completed order.
type OrderExecutionDetails struct {
	DisplaySize              *int
	SweepToFill              bool
	AllOrNone                bool
	MinQty                   *int
	TriggerMethod            int
	RandomizeSize            bool
	RandomizePrice           bool
	RefFuturesConID          *ContractID
	MinTradeQty              *int
	MinCompeteSize           *int
	CompeteAgainstBestOffset *decimal.Decimal
	MidOffsetAtWhole         *decimal.Decimal
	MidOffsetAtHalf          *decimal.Decimal
}

// OrderVolatilityDetails contains volatility-order and delta-neutral hedge
// parameters from a completed order.
type OrderVolatilityDetails struct {
	Value              *decimal.Decimal
	Type               *int
	DeltaNeutral       *OrderDeltaNeutralDetails
	ContinuousUpdate   bool
	ReferencePriceType *int
}

// OrderDeltaNeutralDetails contains the active delta-neutral order block of
// a volatility order.
type OrderDeltaNeutralDetails struct {
	OrderType          OrderType
	AuxPrice           *decimal.Decimal
	ConID              ContractID
	ShortSale          bool
	ShortSaleSlot      int
	DesignatedLocation string
}

// OrderScaleDetails contains the full scale-order block echoed by IBKR.
type OrderScaleDetails struct {
	InitialLevelSize    *int
	SubsequentLevelSize *int
	PriceIncrement      *decimal.Decimal
	PriceAdjustValue    *decimal.Decimal
	PriceAdjustInterval *int
	ProfitOffset        *decimal.Decimal
	AutoReset           *bool
	InitialPosition     *int
	InitialFillQty      *int
	RandomPercent       *bool
	Table               string
	ActiveStartTime     string
	ActiveStopTime      string
}

// OrderComplianceDetails contains operator and account-classification
// metadata attached to a completed order.
type OrderComplianceDetails struct {
	Solicited            bool
	OMSContainer         bool
	Shareholder          string
	CustomerAccount      string
	ProfessionalCustomer bool
	Submitter            string
}

// OrderAdjustmentDetails contains the optional adjustable-order echo values.
type OrderAdjustmentDetails struct {
	OrderType      OrderType
	TriggerPrice   *decimal.Decimal
	StopPrice      *decimal.Decimal
	StopLimitPrice *decimal.Decimal
	TrailingAmount *decimal.Decimal
	TrailingUnit   *int
}

// SoftDollarTier is a soft-dollar commission tier from
// [AdvisorsClient.SoftDollarTiers]. Name is the wire identifier; DisplayName is
// the human-readable label.
type SoftDollarTier struct {
	Name        string
	Value       string
	DisplayName string
}

// ExerciseAction selects whether to exercise or lapse an option in an
// [ExerciseOptionsRequest].
type ExerciseAction int

const (
	Exercise ExerciseAction = 1 // exercise the option
	Lapse    ExerciseAction = 2 // let the option lapse
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

// ExerciseOptionsRequest instructs the Gateway to exercise or lapse an option
// position via [OptionsClient.Exercise].
type ExerciseOptionsRequest struct {
	Contract         Contract
	ExerciseAction   ExerciseAction
	ExerciseQuantity int // must be positive
	Account          string
	Override         bool // override IBKR's default handling for an out-of-the-money exercise
}

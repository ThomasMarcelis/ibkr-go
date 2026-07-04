package ibkr

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
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

type OpenOrdersScope string

const (
	OpenOrdersScopeAll    OpenOrdersScope = "all"
	OpenOrdersScopeClient OpenOrdersScope = "client"
	OpenOrdersScopeAuto   OpenOrdersScope = "auto"
)

// OpenOrder is the typed open_order echo from the Gateway. Live open_order
// frames carry no fill progress; track fills through [OrderStatusUpdate] and
// executions instead.
type OpenOrder struct {
	OrderID   int64
	Account   string
	Contract  Contract
	Action    OrderAction
	OrderType OrderType
	Status    OrderStatus
	Quantity  decimal.Decimal

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

	// Order-state margin preview. Populated on the open_order reply to a
	// what-if order ([Order].WhatIf); on regular orders the Gateway sends
	// unset sentinels, which decode as zero.
	InitMarginBefore     decimal.Decimal
	MaintMarginBefore    decimal.Decimal
	EquityWithLoanBefore decimal.Decimal
	InitMarginChange     decimal.Decimal
	MaintMarginChange    decimal.Decimal
	EquityWithLoanChange decimal.Decimal
	InitMarginAfter      decimal.Decimal
	MaintMarginAfter     decimal.Decimal
	EquityWithLoanAfter  decimal.Decimal

	Commission         decimal.Decimal
	MinCommission      decimal.Decimal
	MaxCommission      decimal.Decimal
	CommissionCurrency string
}

// OpenOrderUpdate is a union event from the open-orders subscription. Exactly
// one field is non-nil. Status carries order-status transitions for orders
// observed through the subscription, including orders that have no live
// OrderHandle in this process (e.g. recovered after a restart).
type OpenOrderUpdate struct {
	Order  *OpenOrder
	Status *OrderStatusUpdate
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

// Execution is a single trade execution report from the Gateway, carrying
// the fill details for one leg of an order.
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

type CompletedOrderResult struct {
	Contract  Contract
	Action    OrderAction
	OrderType OrderType
	Status    OrderStatus
	Quantity  decimal.Decimal
	Filled    decimal.Decimal
	Remaining decimal.Decimal
}

type SoftDollarTier struct {
	Name        string
	Value       string
	DisplayName string
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

package codec

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

	// Status at wire position r[92] of the live sv200 layout.
	Status string

	// OrderState margin/commission section (follows Status on the wire).
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

	// ParentID rides the pre-status slot of the live layout (bracket
	// children carry real values there). Live open_order frames carry no
	// fill echo; fills arrive on the separate order_status message.
	ParentID string
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

func (PlaceOrderRequest) messageName() string { return "place_order" }

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

func (CancelOrderRequest) messageName() string { return "cancel_order" }

// GlobalCancelRequest cancels all open orders (outbound msg_id=58).
// At server_version >= 192 (CME_TAGGING_FIELDS), extOperator and
// manualOrderIndicator are sent instead of the legacy version field.
type GlobalCancelRequest struct {
	ExtOperator          string
	ManualOrderIndicator string // empty = UNSET
}

func (GlobalCancelRequest) messageName() string { return "global_cancel" }

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

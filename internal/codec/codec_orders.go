package codec

import "github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"

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
	OrderID int64
	OrderDetails

	// Status at wire position r[92] of the captured classic layout.
	Status string

	// OrderState margin/commission section (follows Status on the wire).
	InitMarginBefore               string
	MaintMarginBefore              string
	EquityWithLoanBefore           string
	InitMarginChange               string
	MaintMarginChange              string
	EquityWithLoanChange           string
	InitMarginAfter                string
	MaintMarginAfter               string
	EquityWithLoanAfter            string
	Commission                     string
	MinCommission                  string
	MaxCommission                  string
	CommissionCurrency             string
	MarginCurrency                 string
	InitMarginBeforeOutsideRTH     string
	MaintMarginBeforeOutsideRTH    string
	EquityWithLoanBeforeOutsideRTH string
	InitMarginChangeOutsideRTH     string
	MaintMarginChangeOutsideRTH    string
	EquityWithLoanChangeOutsideRTH string
	InitMarginAfterOutsideRTH      string
	MaintMarginAfterOutsideRTH     string
	EquityWithLoanAfterOutsideRTH  string
	SuggestedSize                  string
	RejectReason                   string
	Allocations                    []OrderAllocation
	WarningText                    string
}

type OrderAllocation struct {
	Account         string
	Position        string
	PositionDesired string
	PositionAfter   string
	DesiredAllocQty string
	AllowedAllocQty string
	IsMonetary      string
}

type OpenOrderEnd struct{}

type OrderBound struct {
	PermID   int64
	ClientID int
	OrderID  int64
}

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
	ReqID         int
	ClientID      int
	Account       string
	Time          string
	Symbol        string
	SecType       string
	Exchange      string
	Side          string
	LastNDays     *int
	SpecificDates []int
}

type ExecutionDetail struct {
	ReqID    int
	OrderID  int64
	Contract Contract

	ExecID                  string
	Time                    string
	Account                 string
	Exchange                string
	Side                    string
	Shares                  string
	Price                   string
	PermID                  string
	ClientID                string
	Liquidation             string
	CumulativeQuantity      string
	AveragePrice            string
	OrderRef                string
	EconomicValueRule       string
	EconomicValueMultiplier string
	ModelCode               string
	LastLiquidity           string
	PendingPriceRevision    string
	Submitter               string
	OptExerciseOrLapseType  string
}

type ExecutionsEnd struct {
	ReqID int
}

type CommissionReport struct {
	ExecID              string
	Commission          string
	Currency            string
	RealizedPNL         string
	Yield               string
	YieldRedemptionDate string
}

type CompletedOrdersRequest struct {
	APIOnly bool
}

type OrderDetails struct {
	Contract Contract

	ClientID           string
	OrderID            string
	ParentID           string
	Action             string
	Quantity           string
	OrderType          string
	LmtPrice           string
	AuxPrice           string
	TIF                string
	OcaGroup           string
	Account            string
	OpenClose          string
	Origin             string
	OrderRef           string
	PermID             string
	OutsideRTH         string
	Hidden             string
	DiscretionAmt      string
	GoodAfterTime      string
	Transmit           string
	FAGroup            string
	FAMethod           string
	FAPercentage       string
	ModelCode          string
	GoodTillDate       string
	Rule80A            string
	PercentOffset      string
	SettlingFirm       string
	ShortSaleSlot      string
	DesignatedLocation string
	ExemptCode         string
	StartingPrice      string
	StockRefPrice      string
	Delta              string
	StockRangeLower    string
	StockRangeUpper    string
	DisplaySize        string
	SweepToFill        string
	AllOrNone          string
	MinQty             string
	OcaType            string
	TriggerMethod      string

	Volatility                     string
	VolatilityType                 string
	DeltaNeutralOrderType          string
	DeltaNeutralAuxPrice           string
	DeltaNeutralConID              string
	DeltaNeutralShortSale          string
	DeltaNeutralShortSaleSlot      string
	DeltaNeutralDesignatedLocation string
	ContinuousUpdate               string
	ReferencePriceType             string
	TrailStopPrice                 string
	TrailingPercent                string

	ComboLegsDescription string
	OrderComboLegPrices  []string
	SmartComboRouting    []TagValue

	ScaleInitLevelSize       string
	ScaleSubsLevelSize       string
	ScalePriceIncrement      string
	ScalePriceAdjustValue    string
	ScalePriceAdjustInterval string
	ScaleProfitOffset        string
	ScaleAutoReset           string
	ScaleInitPosition        string
	ScaleInitFillQty         string
	ScaleRandomPercent       string
	ScaleTable               string
	ActiveStartTime          string
	ActiveStopTime           string

	HedgeType       string
	HedgeParam      string
	HedgeMaxSize    string
	ClearingAccount string
	ClearingIntent  string
	NotHeld         string

	AlgoStrategy   string
	AlgoParams     []TagValue
	Solicited      string
	WhatIf         string
	Status         string
	RandomizeSize  string
	RandomizePrice string

	ReferenceContractID        string
	PeggedChangeAmountDecrease string
	PeggedChangeAmount         string
	ReferenceChangeAmount      string
	ReferenceExchangeID        string
	Conditions                 []OrderCondition
	ConditionsIgnoreRTH        string
	ConditionsCancelOrder      string

	AdjustedOrderType        string
	TriggerPrice             string
	StopPrice                string
	LmtPriceOffset           string
	AdjustedStopPrice        string
	AdjustedStopLimitPrice   string
	AdjustedTrailingAmount   string
	AdjustableTrailingUnit   string
	CashQty                  string
	DontUseAutoPriceForHedge string
	UsePriceMgmtAlgo         string
	AdvancedErrorOverride    string
	ManualOrderTime          string
	Deactivate               string
	PostOnly                 string
	AllowPreOpen             string
	IgnoreOpenAuction        string
	SeekPriceImprovement     string
	WhatIfType               string
	IsOMSContainer           string
	AutoCancelDate           string
	Filled                   string
	RefFuturesConID          string
	AutoCancelParent         string
	Shareholder              string
	ImbalanceOnly            string
	RouteMarketableToBBO     string
	ParentPermID             string
	CompletedTime            string
	CompletedStatus          string
	MinTradeQty              string
	MinCompeteSize           string
	CompeteAgainstBestOffset string
	MidOffsetAtWhole         string
	MidOffsetAtHalf          string
	CustomerAccount          string
	ProfessionalCustomer     string
	IncludeOvernight         string
	Submitter                string
	CommissionAndFees        string
	CommissionCurrency       string
}

type CompletedOrder struct {
	OrderDetails
}

type CompletedOrderEnd struct{}

// SoftDollarTiers (OUT 79 / IN 77)

type SoftDollarTiersRequest struct {
	ReqID int
}

func (m SoftDollarTiersRequest) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutReqSoftDollarTiers), itoa(m.ReqID)}, nil
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

// PlaceOrder (OUT 3 / IN 3,5) — order management

// PlaceOrderRequest encodes a new or modified order (outbound msg_id=3).
// At server_version >= 145 there is no version field. All fields are strings
// on the wire; UNSET float/int values are encoded as empty string "".
type PlaceOrderRequest struct {
	OrderID  int64
	Contract Contract // 14 wire fields: conId through secId

	// Core order fields
	Action        string // "BUY", "SELL", "SSHORT", "SLONG"
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
	ScaleInitLevelSize       string // empty = UNSET
	ScaleSubsLevelSize       string // empty = UNSET
	ScalePriceIncrement      string // empty = UNSET
	ScalePriceAdjustValue    string // empty = UNSET
	ScalePriceAdjustInterval string // empty = UNSET
	ScaleProfitOffset        string // empty = UNSET
	ScaleAutoReset           string // empty = UNSET
	ScaleInitPosition        string // empty = UNSET
	ScaleInitFillQty         string // empty = UNSET
	ScaleRandomPercent       string // empty = UNSET
	ScaleTable               string
	ActiveStartTime          string
	ActiveStopTime           string

	// Hedge
	HedgeType  string
	HedgeParam string

	// Misc
	OptOutSmartRouting         string
	ClearingAccount            string
	ClearingIntent             string
	NotHeld                    string
	AlgoStrategy               string
	AlgoParams                 []TagValue
	AlgoID                     string
	WhatIf                     string
	OrderMiscOptions           string
	Solicited                  string
	RandomizeSize              string
	RandomizePrice             string
	ReferenceContractID        string
	PeggedChangeAmountDecrease string
	PeggedChangeAmount         string
	ReferenceChangeAmount      string
	ReferenceExchangeID        string

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
	Deactivate                  string
	PostOnly                    string
	AllowPreOpen                string
	IgnoreOpenAuction           string
	RouteMarketableToBBO        string // empty = server default; 0 or 1 when explicit
	SeekPriceImprovement        string // empty = server default; 0 or 1 when explicit
	WhatIfType                  string // empty = UNSET
	HedgeMaxSize                string // empty = UNSET

	AttachedStopLossOrderID     int64
	AttachedStopLossOrderType   string
	AttachedTakeProfitOrderID   int64
	AttachedTakeProfitOrderType string
}

// CancelOrderRequest cancels an order (outbound msg_id=4).
type CancelOrderRequest struct {
	OrderID               int64
	ManualOrderCancelTime string
	ExtOperator           string
	ManualOrderIndicator  string // empty = UNSET
}

// GlobalCancelRequest cancels all open orders (outbound msg_id=58).
type GlobalCancelRequest struct {
	ExtOperator          string
	ManualOrderIndicator string // empty = UNSET
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

func (m ExerciseOptionsRequest) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(protocol.OutExerciseOptions)
	w.WriteInt(2) // version
	w.WriteInt(m.ReqID)
	w.WriteInt(m.Contract.ConID)
	w.WriteString(m.Contract.Symbol)
	w.WriteString(m.Contract.SecType)
	w.WriteString(m.Contract.Expiry)
	if m.Contract.Strike == "" {
		w.WriteString("0")
	} else {
		w.WriteString(m.Contract.Strike)
	}
	w.WriteString(m.Contract.Right)
	w.WriteString(m.Contract.Multiplier)
	w.WriteString(m.Contract.Exchange)
	w.WriteString(m.Contract.Currency)
	w.WriteString(m.Contract.LocalSymbol)
	w.WriteString(m.Contract.TradingClass)
	w.WriteInt(m.ExerciseAction)
	w.WriteInt(m.ExerciseQuantity)
	w.WriteString(m.Account)
	w.WriteInt(m.Override)
	// The classic shape includes the manual-order-time, customer-account,
	// and professional-customer tail after override.
	w.WriteString("")  // manualOrderTime (client.py:1775)
	w.WriteString("")  // customerAccount (client.py:1779)
	w.WriteBool(false) // professionalCustomer (client.py:1783)
	return w.Fields(), nil
}

// [77, reqId, count, (name, value, displayName) * count]
func decodeSoftDollarTiers(r *fieldReader, sv int) ([]Message, error) {
	reqID, _ := r.ReadInt()
	count, err := r.ReadCount("soft dollar tier count")
	if err != nil {
		return nil, err
	}
	if err := r.RequireFixedEntryFields("soft dollar tiers", count, 3, 0); err != nil {
		return nil, err
	}
	tiers := make([]SoftDollarTier, count)
	for i := range tiers {
		tiers[i] = SoftDollarTier{Name: r.ReadString(), Value: r.ReadString(), DisplayName: r.ReadString()}
	}
	return []Message{SoftDollarTiersResponse{ReqID: reqID, Tiers: tiers}}, nil
}

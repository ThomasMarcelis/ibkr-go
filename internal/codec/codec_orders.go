package codec

import (
	"fmt"

	"github.com/ThomasMarcelis/ibkr-go/internal/protocol"
)

type OpenOrdersRequest struct {
	Scope string
}

func (m OpenOrdersRequest) encodeWire(sv int) ([]string, error) {
	switch m.Scope {
	case "all":
		return []string{itoa(protocol.OutReqAllOpenOrders), "1"}, nil
	case "client":
		return []string{itoa(protocol.OutReqOpenOrders), "1"}, nil
	case "auto":
		return []string{itoa(protocol.OutReqAutoOpenOrders), "1", "1"}, nil
	default:
		return []string{itoa(protocol.OutReqAllOpenOrders), "1"}, nil
	}
}

type CancelOpenOrders struct{}

func (m CancelOpenOrders) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutReqAutoOpenOrders), "1", "0"}, nil
}

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
	ComboLegsDescription  string
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

	// Partial marks a frame whose layout diverged from the attested
	// server->client shape at some advanced-order block, so decoding stopped
	// early and returned only the pre-status fields; Status and the margin
	// section are empty.
	Partial bool
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

func (m ExecutionsRequest) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(protocol.OutReqExecutions)
	w.WriteInt(3) // version
	w.WriteInt(m.ReqID)
	w.WriteInt(m.ClientID)
	w.WriteString(m.Account)
	w.WriteString(m.Time)
	w.WriteString(m.Symbol)
	w.WriteString(m.SecType)
	w.WriteString(m.Exchange)
	w.WriteString(m.Side)
	if sv >= protocol.MinServerVersionParametrizedDaysOfExecutions {
		// The official clients encode an unset lastNDays as UNSET_INTEGER
		// (2^31-1), followed by the specific YYYYMMDD dates.
		if m.LastNDays == nil {
			w.WriteInt(2147483647)
		} else {
			w.WriteInt(*m.LastNDays)
		}
		w.WriteInt(len(m.SpecificDates))
		for _, date := range m.SpecificDates {
			w.WriteInt(date)
		}
	}
	return w.Fields(), nil
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

func (m CompletedOrdersRequest) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(protocol.OutReqCompletedOrders)
	w.WriteBool(m.APIOnly)
	return w.Fields(), nil
}

type CompletedOrder struct {
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

	HedgeType       string
	HedgeParam      string
	ClearingAccount string
	ClearingIntent  string
	NotHeld         string

	AlgoStrategy   string
	AlgoParams     []TagValue
	Solicited      string
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

	StopPrice                string
	LmtPriceOffset           string
	CashQty                  string
	DontUseAutoPriceForHedge string
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
	Submitter                string
	CommissionAndFees        string
	CommissionCurrency       string
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
	OptOutSmartRouting string
	ClearingAccount    string
	ClearingIntent     string
	NotHeld            string
	AlgoStrategy       string
	AlgoParams         []TagValue
	AlgoID             string
	WhatIf             string
	OrderMiscOptions   string
	Solicited          string
	RandomizeSize      string
	RandomizePrice     string

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

func (m PlaceOrderRequest) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(protocol.OutPlaceOrder)
	// No version field at sv >= 145
	w.WriteInt64(m.OrderID)
	// Contract: 14 fields (conId, symbol, secType, lastTradeDate, strike, right,
	// multiplier, exchange, primaryExchange, currency, localSymbol, tradingClass,
	// secIdType, secId)
	w.WriteInt(m.Contract.ConID)
	w.WriteString(m.Contract.Symbol)
	w.WriteString(m.Contract.SecType)
	w.WriteString(m.Contract.Expiry)
	w.WriteString(m.Contract.Strike)
	w.WriteString(m.Contract.Right)
	w.WriteString(m.Contract.Multiplier)
	w.WriteString(m.Contract.Exchange)
	w.WriteString(m.Contract.PrimaryExchange)
	w.WriteString(m.Contract.Currency)
	w.WriteString(m.Contract.LocalSymbol)
	w.WriteString(m.Contract.TradingClass)
	w.WriteString(m.Contract.SecurityIDType)
	w.WriteString(m.Contract.SecurityID)
	// Main order fields
	w.WriteString(m.Action)
	w.WriteString(m.TotalQuantity)
	w.WriteString(m.OrderType)
	w.WriteString(m.LmtPrice) // empty = UNSET
	w.WriteString(m.AuxPrice) // empty = UNSET
	// Extended order fields
	w.WriteString(m.TIF)
	w.WriteString(m.OcaGroup)
	w.WriteString(m.Account)
	w.WriteString(m.OpenClose)
	w.WriteString(m.Origin) // "0" = customer
	w.WriteString(m.OrderRef)
	w.WriteString(m.Transmit) // "1" = true
	w.WriteString(m.ParentID) // "0" = no parent
	w.WriteString(m.BlockOrder)
	w.WriteString(m.SweepToFill)
	w.WriteString(m.DisplaySize)
	w.WriteString(m.TriggerMethod)
	w.WriteString(m.OutsideRTH)
	w.WriteString(m.Hidden)
	if m.Contract.SecType == "BAG" {
		w.WriteInt(len(m.Contract.ComboLegs))
		for _, leg := range m.Contract.ComboLegs {
			w.WriteInt(leg.ConID)
			w.WriteInt(leg.Ratio)
			w.WriteString(leg.Action)
			w.WriteString(leg.Exchange)
			w.WriteString(leg.OpenClose)
			w.WriteString(leg.ShortSaleSlot)
			w.WriteString(leg.DesignatedLocation)
			w.WriteString(leg.ExemptCode)
		}
		w.WriteInt(len(m.OrderComboLegPrices))
		for _, price := range m.OrderComboLegPrices {
			w.WriteString(price)
		}
		writeTagValuePairs(&w, m.SmartComboRoutingParams)
	}
	// Deprecated + FA + model
	w.WriteString("") // deprecated sharesAllocation
	w.WriteString(m.DiscretionaryAmt)
	w.WriteString(m.GoodAfterTime)
	w.WriteString(m.GoodTillDate)
	w.WriteString(m.FAGroup)
	w.WriteString(m.FAMethod)
	w.WriteString(m.FAPercentage)
	if sv < protocol.MinServerVersionFAProfileDesupport {
		w.WriteString("") // deprecated faProfile (client.py:2463-2464)
	}
	w.WriteString(m.ModelCode)
	// Short sale
	w.WriteString(m.ShortSaleSlot)
	w.WriteString(m.DesignatedLocation)
	w.WriteString(m.ExemptCode) // "-1" default
	// Order type extensions
	w.WriteString(m.OcaType)
	w.WriteString(m.Rule80A)
	w.WriteString(m.SettlingFirm)
	w.WriteString(m.AllOrNone)
	w.WriteString(m.MinQty)        // empty = UNSET
	w.WriteString(m.PercentOffset) // empty = UNSET
	w.WriteString("0")             // deprecated eTradeOnly
	w.WriteString("0")             // deprecated firmQuoteOnly
	w.WriteString("")              // deprecated nbboPriceCap (UNSET=empty)
	w.WriteString(m.AuctionStrategy)
	w.WriteString(m.StartingPrice)
	w.WriteString(m.StockRefPrice)
	w.WriteString(m.Delta)
	w.WriteString(m.StockRangeLower)
	w.WriteString(m.StockRangeUpper)
	w.WriteString(m.OverridePercentageConstraints)
	// Volatility
	w.WriteString(m.Volatility)
	w.WriteString(m.VolatilityType)
	w.WriteString(m.DeltaNeutralOrderType)
	w.WriteString(m.DeltaNeutralAuxPrice)
	// grounded v1.2 leaves delta-neutral extension fields deferred
	w.WriteString(m.ContinuousUpdate)
	w.WriteString(m.ReferencePriceType)
	// Trailing
	w.WriteString(m.TrailStopPrice)
	w.WriteString(m.TrailingPercent)
	// Scale
	w.WriteString(m.ScaleInitLevelSize)
	w.WriteString(m.ScaleSubsLevelSize)
	w.WriteString(m.ScalePriceIncrement)
	// grounded v1.2 leaves scale extension fields deferred
	w.WriteString(m.ScaleTable)
	w.WriteString(m.ActiveStartTime)
	w.WriteString(m.ActiveStopTime)
	// Hedge
	w.WriteString(m.HedgeType)
	if m.HedgeType != "" {
		w.WriteString(m.HedgeParam)
	}
	// Misc
	w.WriteString(m.OptOutSmartRouting)
	w.WriteString(m.ClearingAccount)
	w.WriteString(m.ClearingIntent)
	w.WriteString(m.NotHeld)
	w.WriteBool(m.Contract.DeltaNeutral != nil)
	if m.Contract.DeltaNeutral != nil {
		w.WriteInt(m.Contract.DeltaNeutral.ConID)
		w.WriteString(m.Contract.DeltaNeutral.Delta)
		w.WriteString(m.Contract.DeltaNeutral.Price)
	}
	w.WriteString(m.AlgoStrategy)
	if m.AlgoStrategy != "" {
		writeTagValuePairs(&w, m.AlgoParams)
	}
	w.WriteString(m.AlgoID)
	w.WriteString(m.WhatIf)
	w.WriteString(m.OrderMiscOptions)
	w.WriteString(m.Solicited)
	w.WriteString(m.RandomizeSize)
	w.WriteString(m.RandomizePrice)
	// [OrderType != "PEG BENCH" => skip peg bench fields]
	w.WriteInt(len(m.Conditions))
	for _, cond := range m.Conditions {
		if err := writeOrderCondition(&w, cond); err != nil {
			return nil, err
		}
	}
	if len(m.Conditions) > 0 {
		w.WriteString(m.ConditionsIgnoreRTH)
		w.WriteString(m.ConditionsCancelOrder)
	}
	w.WriteString(m.AdjustedOrderType)
	w.WriteString(m.TriggerPrice)
	w.WriteString(m.LmtPriceOffset)
	w.WriteString(m.AdjustedStopPrice)
	w.WriteString(m.AdjustedStopLimitPrice)
	w.WriteString(m.AdjustedTrailingAmount)
	w.WriteString(m.AdjustableTrailingUnit)
	w.WriteString(m.ExtOperator)
	w.WriteString(m.SoftDollarName)
	w.WriteString(m.SoftDollarValue)
	w.WriteString(m.CashQty)
	w.WriteString(m.Mifid2DecisionMaker)
	w.WriteString(m.Mifid2DecisionAlgo)
	w.WriteString(m.Mifid2ExecutionTrader)
	w.WriteString(m.Mifid2ExecutionAlgo)
	w.WriteString(m.DontUseAutoPriceForHedge)
	w.WriteString(m.IsOmsContainer)
	w.WriteString(m.DiscretionaryUpToLimitPrice)
	w.WriteString(m.UsePriceMgmtAlgo)
	w.WriteString(m.Duration)
	w.WriteString(m.PostToAts)
	w.WriteString(m.AutoCancelParent)
	w.WriteString(m.AdvancedErrorOverride)
	w.WriteString(m.ManualOrderTime)
	// [Exchange != IBKRATS, OrderType != PEG BEST/MID => skip peg offsets]
	if sv >= protocol.MinServerVersionCustomerAccount {
		w.WriteString(m.CustomerAccount) // client.py:2746
	}
	if sv >= protocol.MinServerVersionProfessionalCustomer {
		w.WriteString(m.ProfessionalCustomer) // client.py:2749
	}
	if sv >= protocol.MinServerVersionRFQFields && sv < protocol.MinServerVersionUndoRFQFields {
		// Legacy RFQ window (client.py:2752-2754): empty placeholder then
		// UNSET_INTEGER (2^31-1). Removed at UndoRFQFields (190).
		w.WriteString("")
		w.WriteString("2147483647")
	}
	if sv >= protocol.MinServerVersionIncludeOvernight {
		w.WriteString(m.IncludeOvernight) // client.py:2756
	}
	if sv >= protocol.MinServerVersionCMETaggingFields {
		w.WriteString(m.ManualOrderIndicator) // client.py:2759
	}
	if sv >= protocol.MinServerVersionImbalanceOnly {
		w.WriteString(m.ImbalanceOnly) // client.py:2762
	}
	return w.Fields(), nil
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

func (m CancelOrderRequest) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(protocol.OutCancelOrder)
	if sv < protocol.MinServerVersionCMETaggingFields {
		w.WriteString("1") // legacy version field (client.py:2899-2900)
	}
	w.WriteInt64(m.OrderID)
	w.WriteString(m.ManualOrderCancelTime)
	if sv >= protocol.MinServerVersionRFQFields && sv < protocol.MinServerVersionUndoRFQFields {
		// Legacy RFQ window (client.py:2905-2908): two empty placeholders
		// then UNSET_INTEGER. Removed at UndoRFQFields (190).
		w.WriteString("")
		w.WriteString("")
		w.WriteString("2147483647")
	}
	if sv >= protocol.MinServerVersionCMETaggingFields {
		w.WriteString(m.ExtOperator)          // client.py:2911
		w.WriteString(m.ManualOrderIndicator) // client.py:2912
	}
	return w.Fields(), nil
}

// GlobalCancelRequest cancels all open orders (outbound msg_id=58).
// At server_version >= 192 (CME_TAGGING_FIELDS), extOperator and
// manualOrderIndicator are sent instead of the legacy version field.
type GlobalCancelRequest struct {
	ExtOperator          string
	ManualOrderIndicator string // empty = UNSET
}

func (m GlobalCancelRequest) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(protocol.OutReqGlobalCancel)
	if sv < protocol.MinServerVersionCMETaggingFields {
		w.WriteString("1") // legacy version field (client.py:3131-3132)
	}
	if sv >= protocol.MinServerVersionCMETaggingFields {
		w.WriteString(m.ExtOperator)          // client.py:3135
		w.WriteString(m.ManualOrderIndicator) // client.py:3136
	}
	return w.Fields(), nil
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
	// server_version 200 expects the manual-order-time, customer-account,
	// and professional-customer tail; ending the frame at override drew
	// code 10300 from the live Gateway (capture 20260611T074859Z,
	// sha 241a49023701e9ec).
	if sv >= protocol.MinServerVersionManualOrderTimeExerciseOptions {
		w.WriteString("") // manualOrderTime (client.py:1775)
	}
	if sv >= protocol.MinServerVersionCustomerAccount {
		w.WriteString("") // customerAccount (client.py:1779)
	}
	if sv >= protocol.MinServerVersionProfessionalCustomer {
		w.WriteBool(false) // professionalCustomer (client.py:1783)
	}
	return w.Fields(), nil
}

// [3, orderId, status, filled, remaining, avgFillPrice, permId, parentId, lastFillPrice, clientId, whyHeld, mktCapPrice]
func decodeOrderStatus(r *fieldReader, sv int) ([]Message, error) {
	orderID, _ := r.ReadInt64()
	status := r.ReadString()
	filled := r.ReadString()
	remaining := r.ReadString()
	avgFillPrice := r.ReadString()
	permID := r.ReadString()
	parentID := r.ReadString()
	lastFillPrice := r.ReadString()
	clientID := r.ReadString()
	whyHeld := r.ReadString()
	mktCapPrice := r.ReadString()
	return []Message{OrderStatus{
		OrderID: orderID, Status: status, Filled: filled, Remaining: remaining,
		AvgFillPrice: avgFillPrice, PermID: permID, ParentID: parentID,
		LastFillPrice: lastFillPrice, ClientID: clientID,
		WhyHeld: whyHeld, MktCapPrice: mktCapPrice,
	}}, nil
}

func (m OrderStatus) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(protocol.InOrderStatus)
	w.WriteInt64(m.OrderID)
	w.WriteString(m.Status)
	w.WriteString(m.Filled)
	w.WriteString(m.Remaining)
	w.WriteString(m.AvgFillPrice)
	w.WriteString(m.PermID)
	w.WriteString(m.ParentID)
	w.WriteString(m.LastFillPrice)
	w.WriteString(m.ClientID)
	w.WriteString(m.WhyHeld)
	w.WriteString(m.MktCapPrice)
	return w.Fields(), nil
}

func decodeOpenOrder(r *fieldReader, sv int) ([]Message, error) {
	orderID, _ := r.ReadInt64()     // r[0]
	contract := readWireContract(r) // r[1..11]
	action := r.ReadString()        // r[12]
	quantity := r.ReadString()      // r[13]
	orderType := r.ReadString()     // r[14]
	lmtPrice := r.ReadString()      // r[15]
	auxPrice := r.ReadString()      // r[16]
	tif := r.ReadString()           // r[17]
	ocaGroup := r.ReadString()      // r[18]
	account := r.ReadString()       // r[19]
	openClose := r.ReadString()     // r[20]
	origin := r.ReadString()        // r[21]
	orderRef := r.ReadString()      // r[22]
	clientID := r.ReadString()      // r[23]
	permID := r.ReadString()        // r[24]
	outsideRTH := r.ReadString()    // r[25]
	hidden := r.ReadString()        // r[26]
	discretionAmt := r.ReadString() // r[27]
	goodAfterTime := r.ReadString() // r[28]

	partial := OpenOrder{
		OrderID: orderID, Contract: contract,
		Action: action, Quantity: quantity, OrderType: orderType,
		LmtPrice: lmtPrice, AuxPrice: auxPrice, TIF: tif,
		OcaGroup: ocaGroup, Account: account,
		OpenClose: openClose, Origin: origin, OrderRef: orderRef,
		ClientID: clientID, PermID: permID, OutsideRTH: outsideRTH,
		Hidden: hidden, DiscretionAmt: discretionAmt, GoodAfterTime: goodAfterTime,
		Partial: true,
	}

	// Shared pre-status order fields in the observed server->client layout.
	r.ReadString() // deprecated sharesAllocation
	r.ReadString() // FAGroup
	r.ReadString() // FAMethod
	r.ReadString() // FAPercentage
	if sv < protocol.MinServerVersionFAProfileDesupport {
		r.ReadString() // deprecated faProfile (orderdecoder.py:136-137)
	}
	r.ReadString() // ModelCode
	r.ReadString() // GoodTillDate
	r.ReadString() // Rule80A
	r.ReadString() // PercentOffset
	r.ReadString() // SettlingFirm
	r.ReadString() // ShortSaleSlot
	r.ReadString() // DesignatedLocation
	r.ReadString() // ExemptCode
	r.ReadString() // AuctionStrategy
	r.ReadString() // StartingPrice
	r.ReadString() // StockRefPrice
	r.ReadString() // Delta
	r.ReadString() // StockRangeLower
	r.ReadString() // StockRangeUpper
	r.ReadString() // DisplaySize
	r.ReadString() // BlockOrder
	r.ReadString() // SweepToFill
	r.ReadString() // AllOrNone
	r.ReadString() // MinQty
	r.ReadString() // OcaType
	r.ReadString() // deprecated ETradeOnly
	r.ReadString() // deprecated FirmQuoteOnly
	r.ReadString() // deprecated NBBOPriceCap
	// The pre-status slot carries the wire's parentId on every live
	// frame (bracket children hold real values here); the live tail has
	// no second copy, so this slot feeds OpenOrder.ParentID.
	preStatusParentID := r.ReadString()
	r.ReadString() // TriggerMethod

	r.ReadString() // Volatility
	r.ReadString() // VolatilityType
	deltaNeutralOrderType := r.ReadString()
	r.ReadString() // DeltaNeutralAuxPrice
	// Live IB Gateway (server_version 200) sends the sentinel "None" for
	// orders without a delta-neutral leg. "None" is non-empty, so per the
	// official field order the 8-field delta-neutral block still follows
	// (conId, settlingFirm, clearingAccount, clearingIntent, openClose,
	// shortSale, shortSaleSlot, designatedLocation — observed as
	// 0,"","","","?",0,0,"" in captures/20260610T200935Z-api_conditions_matrix_aapl
	// sha256 prefix 87059663ed139026, captures/20260611T073844Z-place_order_price_condition_aapl
	// sha256 prefix 6c588c638895f152, and the 20260405T215248Z-open_orders_all
	// OBDC frame). Every live sv200 open_order carries this shape, and
	// codec.Encode emits the same layout for replay frames. Any other
	// DeltaNeutralOrderType (a real delta-neutral order, or the empty
	// pre-sentinel form) remains unattested live and falls back to the
	// partial decode.
	if deltaNeutralOrderType != "None" {
		return []Message{partial}, nil
	}
	r.Skip(8)      // delta-neutral block (see layout note above)
	r.ReadString() // ContinuousUpdate
	r.ReadString() // ReferencePriceType
	r.ReadString() // TrailStopPrice
	r.ReadString() // TrailingPercent
	r.ReadString() // BasisPoints
	r.ReadString() // BasisPointsType
	comboLegsDescription := r.ReadString()

	comboLegsCount, err := r.ReadOptionalCount("open order combo legs")
	if err != nil {
		return nil, err
	}
	if err := r.RequireFixedEntryFields("open order combo legs", comboLegsCount, 8, 0); err != nil {
		return nil, err
	}
	comboLegs := make([]ComboLeg, comboLegsCount)
	for i := range comboLegs {
		comboLegs[i] = ComboLeg{
			ConID:              mustReadInt(r),
			Ratio:              mustReadInt(r),
			Action:             r.ReadString(),
			Exchange:           r.ReadString(),
			OpenClose:          r.ReadString(),
			ShortSaleSlot:      r.ReadString(),
			DesignatedLocation: r.ReadString(),
			ExemptCode:         r.ReadString(),
		}
	}
	contract.ComboLegs = comboLegs
	partial.Contract = contract

	orderComboLegsCount, err := r.ReadOptionalCount("open order combo leg prices")
	if err != nil {
		return nil, err
	}
	if err := r.RequireFixedEntryFields("open order combo leg prices", orderComboLegsCount, 1, 0); err != nil {
		return nil, err
	}
	orderComboLegPrices := make([]string, orderComboLegsCount)
	for i := range orderComboLegPrices {
		orderComboLegPrices[i] = r.ReadString()
	}

	smartComboRoutingParamsCount, err := r.ReadOptionalCount("open order smart combo routing params")
	if err != nil {
		return nil, err
	}
	smartComboRouting, err := readTagValuePairs(r, "open order smart combo routing params", smartComboRoutingParamsCount)
	if err != nil {
		return nil, err
	}

	r.ReadString() // ScaleInitLevelSize
	r.ReadString() // ScaleSubsLevelSize
	scalePriceIncrement := r.ReadString()
	if scalePriceIncrement != unsetDoubleSentinel && isPositiveWireNumber(scalePriceIncrement) {
		// A real scale order appends scale fields that are unattested
		// live; no-scale frames echo 2147483647/2147483647 with the
		// increment empty or carrying the unset-double sentinel, and go
		// straight to hedgeType (official decoding appends the scale
		// block only for a real positive increment).
		return []Message{partial}, nil
	}
	hedgeType := r.ReadString()
	if hedgeType != "" {
		r.ReadString() // HedgeParam
	}
	r.ReadString() // OptOutSmartRouting
	r.ReadString() // ClearingAccount
	r.ReadString() // ClearingIntent
	r.ReadString() // NotHeld
	deltaNeutralContractPresent := r.ReadString()
	if deltaNeutralContractPresent == "1" {
		contract.DeltaNeutral = &DeltaNeutralContract{
			ConID: mustReadInt(r),
			Delta: r.ReadString(),
			Price: r.ReadString(),
		}
		partial.Contract = contract
	}
	algoStrategy := r.ReadString()
	var algoParams []TagValue
	if algoStrategy != "" {
		algoParamsCount, err := r.ReadCount("open order algo params")
		if err != nil {
			return nil, err
		}
		algoParams, err = readTagValuePairs(r, "open order algo params", algoParamsCount)
		if err != nil {
			return nil, err
		}
	}
	r.ReadString() // Solicited
	r.ReadString() // WhatIf

	status := r.ReadString()
	initMarginBefore := r.ReadString()
	maintMarginBefore := r.ReadString()
	equityWithLoanBefore := r.ReadString()
	initMarginChange := r.ReadString()
	maintMarginChange := r.ReadString()
	equityWithLoanChange := r.ReadString()
	initMarginAfter := r.ReadString()
	maintMarginAfter := r.ReadString()
	equityWithLoanAfter := r.ReadString()
	commission := r.ReadString()
	minCommission := r.ReadString()
	maxCommission := r.ReadString()
	commissionCurrency := r.ReadString()
	if sv >= protocol.MinServerVersionFullOrderPreviewFields {
		// FULL_ORDER_PREVIEW block (orderdecoder.py:369-395): marginCurrency,
		// nine outside-RTH margin fields, suggestedSize, rejectReason, then
		// the order-allocations vector. warningText below is unconditional.
		r.ReadString() // MarginCurrency
		r.ReadString() // InitMarginBeforeOutsideRTH
		r.ReadString() // MaintMarginBeforeOutsideRTH
		r.ReadString() // EquityWithLoanBeforeOutsideRTH
		r.ReadString() // InitMarginChangeOutsideRTH
		r.ReadString() // MaintMarginChangeOutsideRTH
		r.ReadString() // EquityWithLoanChangeOutsideRTH
		r.ReadString() // InitMarginAfterOutsideRTH
		r.ReadString() // MaintMarginAfterOutsideRTH
		r.ReadString() // EquityWithLoanAfterOutsideRTH
		r.ReadString() // SuggestedSize
		r.ReadString() // RejectReason
		orderAllocationsCount, err := r.ReadOptionalCount("open order allocations")
		if err != nil {
			return nil, err
		}
		// Official allocations carry seven fields per entry
		// (account..isMonetary); there is no trailing reserved slot.
		if err := r.RequireFixedEntryFields("open order allocations", orderAllocationsCount, 7, 0); err != nil {
			return nil, err
		}
		for range orderAllocationsCount {
			r.ReadString() // Account
			r.ReadString() // Position
			r.ReadString() // PositionDesired
			r.ReadString() // PositionAfter
			r.ReadString() // DesiredAllocQty
			r.ReadString() // AllowedAllocQty
			r.ReadString() // IsMonetary
		}
	}
	warningText := r.ReadString()

	// Post-status advanced-order fields needed to reach conditional blocks.
	r.ReadString() // RandomizeSize
	r.ReadString() // RandomizePrice
	if orderType == "PEG BENCH" {
		for range 5 {
			r.ReadString()
		}
	}

	conditionsCount, err := r.ReadOptionalCount("open order conditions")
	if err != nil {
		return nil, err
	}
	// Every condition consumes at least four fields; a count past the
	// remaining payload is a malformed frame, not an allocation request.
	if conditionsCount > r.Remaining()/4+1 {
		return nil, fmt.Errorf("codec: open order conditions count %d exceeds remaining fields %d", conditionsCount, r.Remaining())
	}
	conditions := make([]OrderCondition, conditionsCount)
	for i := range conditions {
		conditionType, err := r.ReadInt()
		if err != nil {
			return nil, err
		}
		conditions[i], err = readOrderCondition(r, conditionType)
		if err != nil {
			return nil, err
		}
	}
	conditionsIgnoreRTH := ""
	conditionsCancelOrder := ""
	if conditionsCount > 0 {
		conditionsIgnoreRTH = btoa(mustReadBool(r))
		conditionsCancelOrder = btoa(mustReadBool(r))
	}
	// Live v200 frames end with the official 32-field tail:
	// adjustedOrderType, triggerPrice, trailStopPrice, lmtPriceOffset,
	// adjustedStopPrice, adjustedStopLimitPrice, adjustedTrailingAmount,
	// adjustableTrailingUnit, softDollar name/value/displayName,
	// cashQty, dontUseAutoPriceForHedge, isOmsContainer,
	// discretionaryUpToLimitPrice, usePriceMgmtAlgo, duration,
	// postToAts, autoCancelParent, minTradeQty, minCompeteSize,
	// competeAgainstBestOffset, midOffsetAtWhole, midOffsetAtHalf,
	// customerAccount, professionalCustomer, bondAccruedInterest,
	// includeOvernight, extOperator, manualOrderIndicator, submitter,
	// imbalanceOnly. None map to OpenOrder fields, and there is no fill
	// echo; fills arrive on the separate order_status frame. Any other
	// tail width is an unattested shape and falls back to the partial
	// decode.
	//
	// The tail is a fixed 24-field base always present at the 176 floor
	// (adjustedOrderParams 8, softDollarTier 3, cashQty, autoPriceForHedge,
	// omsContainer, discretionaryUpToLimit, usePriceMgmtAlgo, duration,
	// postToAts, autoCancelParent, pegBest/pegMid offsets 5) plus one field
	// per gated extension present at this version (orderdecoder.py:372-391).
	// At sv200 the extensions sum to 8, giving the 32-field live tail.
	expectedTail := 24
	if sv >= protocol.MinServerVersionCustomerAccount {
		expectedTail++
	}
	if sv >= protocol.MinServerVersionProfessionalCustomer {
		expectedTail++
	}
	if sv >= protocol.MinServerVersionBondAccruedInterest {
		expectedTail++
	}
	if sv >= protocol.MinServerVersionIncludeOvernight {
		expectedTail++
	}
	if sv >= protocol.MinServerVersionCMETaggingFieldsInOpenOrder {
		expectedTail += 2 // extOperator + manualOrderIndicator
	}
	if sv >= protocol.MinServerVersionSubmitter {
		expectedTail++
	}
	if sv >= protocol.MinServerVersionImbalanceOnly {
		expectedTail++
	}
	if r.Remaining() != expectedTail {
		return []Message{partial}, nil
	}
	r.Skip(expectedTail)

	return []Message{OpenOrder{
		OrderID: orderID, Contract: contract,
		Action: action, Quantity: quantity, OrderType: orderType,
		LmtPrice: lmtPrice, AuxPrice: auxPrice, TIF: tif,
		OcaGroup: ocaGroup, Account: account,
		OpenClose: openClose, Origin: origin, OrderRef: orderRef,
		ClientID: clientID, PermID: permID, OutsideRTH: outsideRTH,
		Hidden: hidden, DiscretionAmt: discretionAmt, GoodAfterTime: goodAfterTime,
		ComboLegsDescription:  comboLegsDescription,
		OrderComboLegPrices:   orderComboLegPrices,
		SmartComboRouting:     smartComboRouting,
		AlgoStrategy:          algoStrategy,
		AlgoParams:            algoParams,
		Conditions:            conditions,
		ConditionsIgnoreRTH:   conditionsIgnoreRTH,
		ConditionsCancelOrder: conditionsCancelOrder,
		Status:                status,
		InitMarginBefore:      initMarginBefore,
		MaintMarginBefore:     maintMarginBefore,
		EquityWithLoanBefore:  equityWithLoanBefore,
		InitMarginChange:      initMarginChange,
		MaintMarginChange:     maintMarginChange,
		EquityWithLoanChange:  equityWithLoanChange,
		InitMarginAfter:       initMarginAfter,
		MaintMarginAfter:      maintMarginAfter,
		EquityWithLoanAfter:   equityWithLoanAfter,
		Commission:            commission,
		MinCommission:         minCommission,
		MaxCommission:         maxCommission,
		CommissionCurrency:    commissionCurrency,
		WarningText:           warningText,
		ParentID:              preStatusParentID,
	}}, nil
}

func (m OpenOrder) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(protocol.InOpenOrder)
	w.WriteInt64(m.OrderID)
	writeObservedWireContract(&w, m.Contract)
	w.WriteString(m.Action)
	w.WriteString(m.Quantity)
	w.WriteString(m.OrderType)
	w.WriteString(m.LmtPrice)
	w.WriteString(m.AuxPrice)
	w.WriteString(m.TIF)
	w.WriteString(m.OcaGroup)
	w.WriteString(m.Account)
	w.WriteString(m.OpenClose)
	w.WriteString(m.Origin)
	w.WriteString(m.OrderRef)
	w.WriteString(m.ClientID)
	w.WriteString(m.PermID)
	w.WriteString(m.OutsideRTH)
	w.WriteString(m.Hidden)
	w.WriteString(m.DiscretionAmt)
	w.WriteString(m.GoodAfterTime)
	w.WriteString("") // deprecated sharesAllocation
	w.WriteString("") // FAGroup
	w.WriteString("") // FAMethod
	w.WriteString("") // FAPercentage
	w.WriteString("") // ModelCode
	w.WriteString("") // GoodTillDate
	w.WriteString("") // Rule80A
	w.WriteString("") // PercentOffset
	w.WriteString("") // SettlingFirm
	w.WriteString("") // ShortSaleSlot
	w.WriteString("") // DesignatedLocation
	w.WriteString("") // ExemptCode
	w.WriteString("") // AuctionStrategy
	w.WriteString("") // StartingPrice
	w.WriteString("") // StockRefPrice
	w.WriteString("") // Delta
	w.WriteString("") // StockRangeLower
	w.WriteString("") // StockRangeUpper
	w.WriteString("") // DisplaySize
	w.WriteString("") // BlockOrder
	w.WriteString("") // SweepToFill
	w.WriteString("") // AllOrNone
	w.WriteString("") // MinQty
	w.WriteString("") // OcaType
	w.WriteString("") // deprecated ETradeOnly
	w.WriteString("") // deprecated FirmQuoteOnly
	w.WriteString("") // deprecated NBBOPriceCap
	w.WriteString(m.ParentID)
	w.WriteString("") // TriggerMethod
	w.WriteString("") // Volatility
	w.WriteString("") // VolatilityType
	// Live sv200 layout: DeltaNeutralOrderType "None" for orders without
	// a delta-neutral leg, followed by the 8-field delta-neutral block in
	// the captured shape (see the InOpenOrder decode note).
	w.WriteString("None") // DeltaNeutralOrderType
	w.WriteString("")     // DeltaNeutralAuxPrice
	w.WriteString("0")    // delta-neutral conId
	w.WriteString("")     // delta-neutral settlingFirm
	w.WriteString("")     // delta-neutral clearingAccount
	w.WriteString("")     // delta-neutral clearingIntent
	w.WriteString("?")    // delta-neutral openClose
	w.WriteString("0")    // delta-neutral shortSale
	w.WriteString("0")    // delta-neutral shortSaleSlot
	w.WriteString("")     // delta-neutral designatedLocation
	w.WriteString("")     // ContinuousUpdate
	w.WriteString("")     // ReferencePriceType
	w.WriteString("")     // TrailStopPrice
	w.WriteString("")     // TrailingPercent
	w.WriteString("")     // BasisPoints
	w.WriteString("")     // BasisPointsType
	w.WriteString(m.ComboLegsDescription)
	w.WriteInt(len(m.Contract.ComboLegs))
	for _, leg := range m.Contract.ComboLegs {
		w.WriteInt(leg.ConID)
		w.WriteInt(leg.Ratio)
		w.WriteString(leg.Action)
		w.WriteString(leg.Exchange)
		w.WriteString(leg.OpenClose)
		w.WriteString(leg.ShortSaleSlot)
		w.WriteString(leg.DesignatedLocation)
		w.WriteString(leg.ExemptCode)
	}
	w.WriteInt(len(m.OrderComboLegPrices))
	for _, price := range m.OrderComboLegPrices {
		w.WriteString(price)
	}
	writeTagValuePairs(&w, m.SmartComboRouting)
	// Live no-scale echo: UNSET-int level sizes, empty increment, then
	// straight to hedgeType (no scaleTable/activeStartTime/activeStopTime
	// on the live layout).
	w.WriteString("2147483647") // ScaleInitLevelSize
	w.WriteString("2147483647") // ScaleSubsLevelSize
	w.WriteString("")           // ScalePriceIncrement
	w.WriteString("")           // HedgeType
	w.WriteString("")           // OptOutSmartRouting
	w.WriteString("")           // ClearingAccount
	w.WriteString("")           // ClearingIntent
	w.WriteString("")           // NotHeld
	w.WriteBool(m.Contract.DeltaNeutral != nil)
	if m.Contract.DeltaNeutral != nil {
		w.WriteInt(m.Contract.DeltaNeutral.ConID)
		w.WriteString(m.Contract.DeltaNeutral.Delta)
		w.WriteString(m.Contract.DeltaNeutral.Price)
	}
	w.WriteString(m.AlgoStrategy)
	if m.AlgoStrategy != "" {
		writeTagValuePairs(&w, m.AlgoParams)
	}
	w.WriteString("") // Solicited
	w.WriteString("") // WhatIf
	w.WriteString(m.Status)
	w.WriteString(m.InitMarginBefore)
	w.WriteString(m.MaintMarginBefore)
	w.WriteString(m.EquityWithLoanBefore)
	w.WriteString(m.InitMarginChange)
	w.WriteString(m.MaintMarginChange)
	w.WriteString(m.EquityWithLoanChange)
	w.WriteString(m.InitMarginAfter)
	w.WriteString(m.MaintMarginAfter)
	w.WriteString(m.EquityWithLoanAfter)
	w.WriteString(m.Commission)
	w.WriteString(m.MinCommission)
	w.WriteString(m.MaxCommission)
	w.WriteString(m.CommissionCurrency)
	w.WriteString("") // MarginCurrency
	w.WriteString("") // InitMarginBeforeOutsideRTH
	w.WriteString("") // MaintMarginBeforeOutsideRTH
	w.WriteString("") // EquityWithLoanBeforeOutsideRTH
	w.WriteString("") // InitMarginChangeOutsideRTH
	w.WriteString("") // MaintMarginChangeOutsideRTH
	w.WriteString("") // EquityWithLoanChangeOutsideRTH
	w.WriteString("") // InitMarginAfterOutsideRTH
	w.WriteString("") // MaintMarginAfterOutsideRTH
	w.WriteString("") // EquityWithLoanAfterOutsideRTH
	w.WriteString("") // SuggestedSize
	w.WriteString("") // RejectReason
	w.WriteInt(0)     // OrderAllocationsCount
	w.WriteString(m.WarningText)
	w.WriteString("") // RandomizeSize
	w.WriteString("") // RandomizePrice
	w.WriteInt(len(m.Conditions))
	for _, cond := range m.Conditions {
		if err := writeOrderCondition(&w, cond); err != nil {
			return nil, err
		}
	}
	if len(m.Conditions) > 0 {
		w.WriteString(m.ConditionsIgnoreRTH)
		w.WriteString(m.ConditionsCancelOrder)
	}
	// Official 32-field tail of the live sv200 layout (must mirror the
	// InOpenOrder decode tail). No fill echo on open_order; fills ride
	// the separate order_status frame.
	w.WriteString("") // AdjustedOrderType
	w.WriteString("") // TriggerPrice
	w.WriteString("") // TrailStopPrice
	w.WriteString("") // LmtPriceOffset
	w.WriteString("") // AdjustedStopPrice
	w.WriteString("") // AdjustedStopLimitPrice
	w.WriteString("") // AdjustedTrailingAmount
	w.WriteString("") // AdjustableTrailingUnit
	w.WriteString("") // SoftDollarName
	w.WriteString("") // SoftDollarValue
	w.WriteString("") // SoftDollarDisplayName
	w.WriteString("") // CashQty
	w.WriteString("") // DontUseAutoPriceForHedge
	w.WriteString("") // IsOmsContainer
	w.WriteString("") // DiscretionaryUpToLimitPrice
	w.WriteString("") // UsePriceMgmtAlgo
	w.WriteString("") // Duration
	w.WriteString("") // PostToAts
	w.WriteString("") // AutoCancelParent
	w.WriteString("") // MinTradeQty
	w.WriteString("") // MinCompeteSize
	w.WriteString("") // CompeteAgainstBestOffset
	w.WriteString("") // MidOffsetAtWhole
	w.WriteString("") // MidOffsetAtHalf
	w.WriteString("") // CustomerAccount
	w.WriteString("") // ProfessionalCustomer
	w.WriteString("") // BondAccruedInterest
	w.WriteString("") // IncludeOvernight
	w.WriteString("") // ExtOperator
	w.WriteString("") // ManualOrderIndicator
	w.WriteString("") // Submitter
	w.WriteString("") // ImbalanceOnly
	return w.Fields(), nil
}

// [11, reqID, orderId, conID, symbol, secType, expiry, strike,
func decodeExecutionData(r *fieldReader, sv int) ([]Message, error) {
	//   right, multiplier, exchange, currency, localSymbol, tradingClass,
	//   execID, time, account, exchange(exec), side, shares, price, permID,
	//   clientID, liquidation, cumQty, avgPrice, orderRef, evRule,
	//   evMultiplier, modelCode, lastLiquidity, pendingRevision, submitter]
	m := ExecutionDetail{}
	m.ReqID, _ = r.ReadInt()
	m.OrderID, _ = r.ReadInt64()
	m.Contract = readWireContract(r)
	m.ExecID = r.ReadString()
	m.Time = r.ReadString()
	m.Account = r.ReadString()
	m.Exchange = r.ReadString()
	m.Side = r.ReadString()
	m.Shares = r.ReadString()
	m.Price = r.ReadString()
	m.PermID = r.ReadString()
	m.ClientID = r.ReadString()
	m.Liquidation = r.ReadString()
	m.CumulativeQuantity = r.ReadString()
	m.AveragePrice = r.ReadString()
	m.OrderRef = r.ReadString()
	m.EconomicValueRule = r.ReadString()
	m.EconomicValueMultiplier = r.ReadString()
	m.ModelCode = r.ReadString()
	if sv >= protocol.MinServerVersionLastLiquidity {
		m.LastLiquidity = r.ReadString()
	}
	if sv >= protocol.MinServerVersionPendingPriceRevision {
		m.PendingPriceRevision = r.ReadString()
	}
	if sv >= protocol.MinServerVersionSubmitter {
		m.Submitter = r.ReadString()
	}
	if remaining := r.Remaining(); remaining != 0 {
		return nil, fmt.Errorf("ibkr codec: execution detail has %d trailing fields", remaining)
	}
	return []Message{m}, nil
}

func (m ExecutionDetail) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(protocol.InExecutionData)
	w.WriteInt(m.ReqID)
	w.WriteString(i64toa(m.OrderID))
	w.WriteInt(m.Contract.ConID)
	w.WriteString(m.Contract.Symbol)
	w.WriteString(m.Contract.SecType)
	w.WriteString(m.Contract.Expiry)
	w.WriteString(m.Contract.Strike)
	w.WriteString(m.Contract.Right)
	w.WriteString(m.Contract.Multiplier)
	w.WriteString(m.Contract.Exchange)
	w.WriteString(m.Contract.Currency)
	w.WriteString(m.Contract.LocalSymbol)
	w.WriteString(m.Contract.TradingClass)
	w.WriteString(m.ExecID)
	w.WriteString(m.Time)
	w.WriteString(m.Account)
	w.WriteString(m.Exchange)
	w.WriteString(m.Side)
	w.WriteString(m.Shares)
	w.WriteString(m.Price)
	w.WriteString(m.PermID)
	w.WriteString(m.ClientID)
	w.WriteString(m.Liquidation)
	w.WriteString(m.CumulativeQuantity)
	w.WriteString(m.AveragePrice)
	w.WriteString(m.OrderRef)
	w.WriteString(m.EconomicValueRule)
	w.WriteString(m.EconomicValueMultiplier)
	w.WriteString(m.ModelCode)
	if sv >= protocol.MinServerVersionLastLiquidity {
		w.WriteString(m.LastLiquidity)
	}
	if sv >= protocol.MinServerVersionPendingPriceRevision {
		w.WriteString(m.PendingPriceRevision)
	}
	if sv >= protocol.MinServerVersionSubmitter {
		w.WriteString(m.Submitter)
	}
	return w.Fields(), nil
}

func decodeOpenOrderEnd(r *fieldReader, sv int) ([]Message, error) {
	return []Message{OpenOrderEnd{}}, nil
}

func (m OpenOrderEnd) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.InOpenOrderEnd), "1"}, nil
}

// [55, version, reqID]
func decodeExecutionDataEnd(r *fieldReader, sv int) ([]Message, error) {
	r.Skip(1)
	reqID, _ := r.ReadInt()
	return []Message{ExecutionsEnd{ReqID: reqID}}, nil
}

func (m ExecutionsEnd) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.InExecutionDataEnd), "1", itoa(m.ReqID)}, nil
}

// [59, version, execID, commission, currency, realizedPNL, yield, redemptionDate]
func decodeCommissionReport(r *fieldReader, sv int) ([]Message, error) {
	r.Skip(1)
	execID := r.ReadString()
	commission := r.ReadString()
	currency := r.ReadString()
	realizedPNL := r.ReadString()
	yield := r.ReadString()
	yieldRedemptionDate := r.ReadString()
	if remaining := r.Remaining(); remaining != 0 {
		return nil, fmt.Errorf("ibkr codec: commission and fees report has %d trailing fields", remaining)
	}
	return []Message{CommissionReport{
		ExecID: execID, Commission: commission, Currency: currency,
		RealizedPNL: realizedPNL, Yield: yield, YieldRedemptionDate: yieldRedemptionDate,
	}}, nil
}

func (m CommissionReport) encodeWire(sv int) ([]string, error) {
	return []string{
		itoa(protocol.InCommissionReport), "1", m.ExecID, m.Commission, m.Currency,
		m.RealizedPNL, m.Yield, m.YieldRedemptionDate,
	}, nil
}

// [101, contract(11-field), action, totalQty, orderType, ...]
func decodeCompletedOrder(r *fieldReader, sv int) ([]Message, error) {
	m := CompletedOrder{Contract: readWireContract(r)}
	m.Action = r.ReadString()
	m.Quantity = r.ReadString()
	m.OrderType = r.ReadString()
	m.LmtPrice = r.ReadString()
	m.AuxPrice = r.ReadString()
	m.TIF = r.ReadString()
	m.OcaGroup = r.ReadString()
	m.Account = r.ReadString()
	m.OpenClose = r.ReadString()
	m.Origin = r.ReadString()
	m.OrderRef = r.ReadString()
	m.PermID = r.ReadString()
	m.OutsideRTH = r.ReadString()
	m.Hidden = r.ReadString()
	m.DiscretionAmt = r.ReadString()
	m.GoodAfterTime = r.ReadString()
	m.FAGroup = r.ReadString()
	m.FAMethod = r.ReadString()
	m.FAPercentage = r.ReadString()
	if sv < protocol.MinServerVersionFAProfileDesupport {
		r.ReadString() // deprecated FA profile
	}
	// Models are present throughout ibkr-go's supported server range (the
	// official gate is 103; the classic floor here is 176).
	m.ModelCode = r.ReadString()
	m.GoodTillDate = r.ReadString()
	m.Rule80A = r.ReadString()
	m.PercentOffset = r.ReadString()
	m.SettlingFirm = r.ReadString()
	m.ShortSaleSlot = r.ReadString()
	m.DesignatedLocation = r.ReadString()
	m.ExemptCode = r.ReadString()
	m.StartingPrice = r.ReadString()
	m.StockRefPrice = r.ReadString()
	m.Delta = r.ReadString()
	m.StockRangeLower = r.ReadString()
	m.StockRangeUpper = r.ReadString()
	m.DisplaySize = r.ReadString()
	m.SweepToFill = r.ReadString()
	m.AllOrNone = r.ReadString()
	m.MinQty = r.ReadString()
	m.OcaType = r.ReadString()
	m.TriggerMethod = r.ReadString()

	m.Volatility = r.ReadString()
	m.VolatilityType = r.ReadString()
	m.DeltaNeutralOrderType = r.ReadString()
	m.DeltaNeutralAuxPrice = r.ReadString()
	if m.DeltaNeutralOrderType != "" {
		m.DeltaNeutralConID = r.ReadString()
		m.DeltaNeutralShortSale = r.ReadString()
		m.DeltaNeutralShortSaleSlot = r.ReadString()
		m.DeltaNeutralDesignatedLocation = r.ReadString()
	}
	m.ContinuousUpdate = r.ReadString()
	m.ReferencePriceType = r.ReadString()
	m.TrailStopPrice = r.ReadString()
	m.TrailingPercent = r.ReadString()

	m.ComboLegsDescription = r.ReadString()
	comboLegsCount, err := r.ReadOptionalCount("completed order combo legs")
	if err != nil {
		return nil, err
	}
	if err := r.RequireFixedEntryFields("completed order combo legs", comboLegsCount, 8, 0); err != nil {
		return nil, err
	}
	if comboLegsCount > 0 {
		m.Contract.ComboLegs = make([]ComboLeg, comboLegsCount)
	}
	for i := range m.Contract.ComboLegs {
		m.Contract.ComboLegs[i] = ComboLeg{
			ConID:              mustReadInt(r),
			Ratio:              mustReadInt(r),
			Action:             r.ReadString(),
			Exchange:           r.ReadString(),
			OpenClose:          r.ReadString(),
			ShortSaleSlot:      r.ReadString(),
			DesignatedLocation: r.ReadString(),
			ExemptCode:         r.ReadString(),
		}
	}
	orderComboLegsCount, err := r.ReadOptionalCount("completed order combo leg prices")
	if err != nil {
		return nil, err
	}
	if err := r.RequireFixedEntryFields("completed order combo leg prices", orderComboLegsCount, 1, 0); err != nil {
		return nil, err
	}
	if orderComboLegsCount > 0 {
		m.OrderComboLegPrices = make([]string, orderComboLegsCount)
	}
	for i := range m.OrderComboLegPrices {
		m.OrderComboLegPrices[i] = r.ReadString()
	}
	smartComboRoutingCount, err := r.ReadOptionalCount("completed order smart combo routing params")
	if err != nil {
		return nil, err
	}
	if smartComboRoutingCount > 0 {
		m.SmartComboRouting, err = readTagValuePairs(r, "completed order smart combo routing params", smartComboRoutingCount)
		if err != nil {
			return nil, err
		}
	}

	m.ScaleInitLevelSize = r.ReadString()
	m.ScaleSubsLevelSize = r.ReadString()
	m.ScalePriceIncrement = r.ReadString()
	if isPositiveWireNumber(m.ScalePriceIncrement) && m.ScalePriceIncrement != unsetDoubleSentinel {
		m.ScalePriceAdjustValue = r.ReadString()
		m.ScalePriceAdjustInterval = r.ReadString()
		m.ScaleProfitOffset = r.ReadString()
		m.ScaleAutoReset = r.ReadString()
		m.ScaleInitPosition = r.ReadString()
		m.ScaleInitFillQty = r.ReadString()
		m.ScaleRandomPercent = r.ReadString()
	}
	m.HedgeType = r.ReadString()
	if m.HedgeType != "" {
		m.HedgeParam = r.ReadString()
	}
	m.ClearingAccount = r.ReadString()
	m.ClearingIntent = r.ReadString()
	m.NotHeld = r.ReadString()
	if r.ReadString() == "1" {
		m.Contract.DeltaNeutral = &DeltaNeutralContract{
			ConID: mustReadInt(r),
			Delta: r.ReadString(),
			Price: r.ReadString(),
		}
	}
	m.AlgoStrategy = r.ReadString()
	if m.AlgoStrategy != "" {
		algoParamsCount, err := r.ReadCount("completed order algo params")
		if err != nil {
			return nil, err
		}
		m.AlgoParams, err = readTagValuePairs(r, "completed order algo params", algoParamsCount)
		if err != nil {
			return nil, err
		}
	}
	m.Solicited = r.ReadString()
	m.Status = r.ReadString()
	m.RandomizeSize = r.ReadString()
	m.RandomizePrice = r.ReadString()
	if m.OrderType == "PEG BENCH" {
		m.ReferenceContractID = r.ReadString()
		m.PeggedChangeAmountDecrease = r.ReadString()
		m.PeggedChangeAmount = r.ReadString()
		m.ReferenceChangeAmount = r.ReadString()
		m.ReferenceExchangeID = r.ReadString()
	}
	conditionsCount, err := r.ReadOptionalCount("completed order conditions")
	if err != nil {
		return nil, err
	}
	if conditionsCount > r.Remaining()/4+1 {
		return nil, fmt.Errorf("codec: completed order conditions count %d exceeds remaining fields %d", conditionsCount, r.Remaining())
	}
	if conditionsCount > 0 {
		m.Conditions = make([]OrderCondition, conditionsCount)
	}
	for i := range m.Conditions {
		conditionType, err := r.ReadInt()
		if err != nil {
			return nil, err
		}
		m.Conditions[i], err = readOrderCondition(r, conditionType)
		if err != nil {
			return nil, err
		}
	}
	if conditionsCount > 0 {
		m.ConditionsIgnoreRTH = r.ReadString()
		m.ConditionsCancelOrder = r.ReadString()
	}

	m.StopPrice = r.ReadString()
	m.LmtPriceOffset = r.ReadString()
	m.CashQty = r.ReadString()
	m.DontUseAutoPriceForHedge = r.ReadString()
	m.IsOMSContainer = r.ReadString()
	m.AutoCancelDate = r.ReadString()
	m.Filled = r.ReadString()
	m.RefFuturesConID = r.ReadString()
	m.AutoCancelParent = r.ReadString()
	m.Shareholder = r.ReadString()
	m.ImbalanceOnly = r.ReadString()
	m.RouteMarketableToBBO = r.ReadString()
	m.ParentPermID = r.ReadString()
	m.CompletedTime = r.ReadString()
	m.CompletedStatus = r.ReadString()
	m.MinTradeQty = r.ReadString()
	m.MinCompeteSize = r.ReadString()
	m.CompeteAgainstBestOffset = r.ReadString()
	m.MidOffsetAtWhole = r.ReadString()
	m.MidOffsetAtHalf = r.ReadString()
	if sv >= protocol.MinServerVersionCustomerAccount {
		m.CustomerAccount = r.ReadString()
	}
	if sv >= protocol.MinServerVersionProfessionalCustomer {
		m.ProfessionalCustomer = r.ReadString()
	}
	if sv >= protocol.MinServerVersionSubmitter {
		m.Submitter = r.ReadString()
	}
	if err := r.Err(); err != nil {
		return nil, err
	}
	if r.Remaining() != 0 {
		return nil, fmt.Errorf("codec: completed order has %d unparsed fields", r.Remaining())
	}
	return []Message{m}, nil
}

func (m CompletedOrder) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(protocol.InCompletedOrder)
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
	w.WriteString(m.Action)
	w.WriteString(m.Quantity)
	w.WriteString(m.OrderType)
	w.WriteString(m.LmtPrice)
	w.WriteString(m.AuxPrice)
	w.WriteString(m.TIF)
	w.WriteString(m.OcaGroup)
	w.WriteString(m.Account)
	w.WriteString(m.OpenClose)
	w.WriteString(m.Origin)
	w.WriteString(m.OrderRef)
	w.WriteString(m.PermID)
	w.WriteString(m.OutsideRTH)
	w.WriteString(m.Hidden)
	w.WriteString(m.DiscretionAmt)
	w.WriteString(m.GoodAfterTime)
	w.WriteString(m.FAGroup)
	w.WriteString(m.FAMethod)
	w.WriteString(m.FAPercentage)
	if sv < protocol.MinServerVersionFAProfileDesupport {
		w.WriteString("") // deprecated FA profile
	}
	w.WriteString(m.ModelCode)
	w.WriteString(m.GoodTillDate)
	w.WriteString(m.Rule80A)
	w.WriteString(m.PercentOffset)
	w.WriteString(m.SettlingFirm)
	w.WriteString(m.ShortSaleSlot)
	w.WriteString(m.DesignatedLocation)
	w.WriteString(m.ExemptCode)
	w.WriteString(m.StartingPrice)
	w.WriteString(m.StockRefPrice)
	w.WriteString(m.Delta)
	w.WriteString(m.StockRangeLower)
	w.WriteString(m.StockRangeUpper)
	w.WriteString(m.DisplaySize)
	w.WriteString(m.SweepToFill)
	w.WriteString(m.AllOrNone)
	w.WriteString(m.MinQty)
	w.WriteString(m.OcaType)
	w.WriteString(m.TriggerMethod)
	w.WriteString(m.Volatility)
	w.WriteString(m.VolatilityType)
	w.WriteString(m.DeltaNeutralOrderType)
	w.WriteString(m.DeltaNeutralAuxPrice)
	if m.DeltaNeutralOrderType != "" {
		w.WriteString(m.DeltaNeutralConID)
		w.WriteString(m.DeltaNeutralShortSale)
		w.WriteString(m.DeltaNeutralShortSaleSlot)
		w.WriteString(m.DeltaNeutralDesignatedLocation)
	}
	w.WriteString(m.ContinuousUpdate)
	w.WriteString(m.ReferencePriceType)
	w.WriteString(m.TrailStopPrice)
	w.WriteString(m.TrailingPercent)
	w.WriteString(m.ComboLegsDescription)
	w.WriteInt(len(m.Contract.ComboLegs))
	for _, leg := range m.Contract.ComboLegs {
		w.WriteInt(leg.ConID)
		w.WriteInt(leg.Ratio)
		w.WriteString(leg.Action)
		w.WriteString(leg.Exchange)
		w.WriteString(leg.OpenClose)
		w.WriteString(leg.ShortSaleSlot)
		w.WriteString(leg.DesignatedLocation)
		w.WriteString(leg.ExemptCode)
	}
	w.WriteInt(len(m.OrderComboLegPrices))
	for _, price := range m.OrderComboLegPrices {
		w.WriteString(price)
	}
	writeTagValuePairs(&w, m.SmartComboRouting)
	w.WriteString(m.ScaleInitLevelSize)
	w.WriteString(m.ScaleSubsLevelSize)
	w.WriteString(m.ScalePriceIncrement)
	if isPositiveWireNumber(m.ScalePriceIncrement) && m.ScalePriceIncrement != unsetDoubleSentinel {
		w.WriteString(m.ScalePriceAdjustValue)
		w.WriteString(m.ScalePriceAdjustInterval)
		w.WriteString(m.ScaleProfitOffset)
		w.WriteString(m.ScaleAutoReset)
		w.WriteString(m.ScaleInitPosition)
		w.WriteString(m.ScaleInitFillQty)
		w.WriteString(m.ScaleRandomPercent)
	}
	w.WriteString(m.HedgeType)
	if m.HedgeType != "" {
		w.WriteString(m.HedgeParam)
	}
	w.WriteString(m.ClearingAccount)
	w.WriteString(m.ClearingIntent)
	w.WriteString(m.NotHeld)
	w.WriteBool(m.Contract.DeltaNeutral != nil)
	if m.Contract.DeltaNeutral != nil {
		w.WriteInt(m.Contract.DeltaNeutral.ConID)
		w.WriteString(m.Contract.DeltaNeutral.Delta)
		w.WriteString(m.Contract.DeltaNeutral.Price)
	}
	w.WriteString(m.AlgoStrategy)
	if m.AlgoStrategy != "" {
		writeTagValuePairs(&w, m.AlgoParams)
	}
	w.WriteString(m.Solicited)
	w.WriteString(m.Status)
	w.WriteString(m.RandomizeSize)
	w.WriteString(m.RandomizePrice)
	if m.OrderType == "PEG BENCH" {
		w.WriteString(m.ReferenceContractID)
		w.WriteString(m.PeggedChangeAmountDecrease)
		w.WriteString(m.PeggedChangeAmount)
		w.WriteString(m.ReferenceChangeAmount)
		w.WriteString(m.ReferenceExchangeID)
	}
	w.WriteInt(len(m.Conditions))
	for _, condition := range m.Conditions {
		if err := writeOrderCondition(&w, condition); err != nil {
			return nil, err
		}
	}
	if len(m.Conditions) > 0 {
		w.WriteString(m.ConditionsIgnoreRTH)
		w.WriteString(m.ConditionsCancelOrder)
	}
	w.WriteString(m.StopPrice)
	w.WriteString(m.LmtPriceOffset)
	w.WriteString(m.CashQty)
	w.WriteString(m.DontUseAutoPriceForHedge)
	w.WriteString(m.IsOMSContainer)
	w.WriteString(m.AutoCancelDate)
	w.WriteString(m.Filled)
	w.WriteString(m.RefFuturesConID)
	w.WriteString(m.AutoCancelParent)
	w.WriteString(m.Shareholder)
	w.WriteString(m.ImbalanceOnly)
	w.WriteString(m.RouteMarketableToBBO)
	w.WriteString(m.ParentPermID)
	w.WriteString(m.CompletedTime)
	w.WriteString(m.CompletedStatus)
	w.WriteString(m.MinTradeQty)
	w.WriteString(m.MinCompeteSize)
	w.WriteString(m.CompeteAgainstBestOffset)
	w.WriteString(m.MidOffsetAtWhole)
	w.WriteString(m.MidOffsetAtHalf)
	if sv >= protocol.MinServerVersionCustomerAccount {
		w.WriteString(m.CustomerAccount)
	}
	if sv >= protocol.MinServerVersionProfessionalCustomer {
		w.WriteString(m.ProfessionalCustomer)
	}
	if sv >= protocol.MinServerVersionSubmitter {
		w.WriteString(m.Submitter)
	}
	return w.Fields(), nil
}

// [102]
func decodeCompletedOrderEnd(r *fieldReader, sv int) ([]Message, error) {
	return []Message{CompletedOrderEnd{}}, nil
}

func (m CompletedOrderEnd) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.InCompletedOrderEnd)}, nil
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

func (m SoftDollarTiersResponse) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(protocol.InSoftDollarTiers)
	w.WriteInt(m.ReqID)
	w.WriteInt(len(m.Tiers))
	for _, t := range m.Tiers {
		w.WriteString(t.Name)
		w.WriteString(t.Value)
		w.WriteString(t.DisplayName)
	}
	return w.Fields(), nil
}

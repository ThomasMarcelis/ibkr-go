package codec

import (
	"fmt"
)

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

// [3, orderId, status, filled, remaining, avgFillPrice, permId, parentId, lastFillPrice, clientId, whyHeld, mktCapPrice]
func decodeOrderStatus(r *fieldReader) ([]Message, error) {
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

func decodeOpenOrder(r *fieldReader) ([]Message, error) {
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
	}

	// Shared pre-status order fields in the observed server->client layout.
	r.ReadString() // deprecated sharesAllocation
	r.ReadString() // FAGroup
	r.ReadString() // FAMethod
	r.ReadString() // FAPercentage
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
	r.ReadString() // ComboLegsDescrip

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
		return []Message{partial}, nil
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
	if r.Remaining() != 32 {
		return []Message{partial}, nil
	}
	r.Skip(32)

	return []Message{OpenOrder{
		OrderID: orderID, Contract: contract,
		Action: action, Quantity: quantity, OrderType: orderType,
		LmtPrice: lmtPrice, AuxPrice: auxPrice, TIF: tif,
		OcaGroup: ocaGroup, Account: account,
		OpenClose: openClose, Origin: origin, OrderRef: orderRef,
		ClientID: clientID, PermID: permID, OutsideRTH: outsideRTH,
		Hidden: hidden, DiscretionAmt: discretionAmt, GoodAfterTime: goodAfterTime,
		ComboLegs:             comboLegs,
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

// [11, reqID, orderId, conID, symbol, secType, expiry, strike,
func decodeExecutionData(r *fieldReader) ([]Message, error) {
	//   right, multiplier, exchange, currency, localSymbol, tradingClass,
	//   execID, time, account, exchange(exec), side, shares, price, ...]
	reqID, _ := r.ReadInt()
	orderID, _ := r.ReadInt64()
	r.Skip(1) // conID
	symbol := r.ReadString()
	r.Skip(9) // secType, expiry, strike, right, multiplier, exchange, currency, localSymbol, tradingClass
	execID := r.ReadString()
	execTime := r.ReadString()
	account := r.ReadString()
	r.Skip(1) // execution exchange
	side := r.ReadString()
	shares := r.ReadString()
	price := r.ReadString()
	return []Message{ExecutionDetail{ReqID: reqID, OrderID: orderID, ExecID: execID, Account: account, Symbol: symbol, Side: side, Shares: shares, Price: price, Time: execTime}}, nil
}

func decodeOpenOrderEnd(r *fieldReader) ([]Message, error) {
	return []Message{OpenOrderEnd{}}, nil
}

// [55, version, reqID]
func decodeExecutionDataEnd(r *fieldReader) ([]Message, error) {
	r.Skip(1)
	reqID, _ := r.ReadInt()
	return []Message{ExecutionsEnd{ReqID: reqID}}, nil
}

// [59, version, execID, commission, currency, realizedPNL, ...]
func decodeCommissionReport(r *fieldReader) ([]Message, error) {
	r.Skip(1)
	execID := r.ReadString()
	commission := r.ReadString()
	currency := r.ReadString()
	realizedPNL := r.ReadString()
	return []Message{CommissionReport{ExecID: execID, Commission: commission, Currency: currency, RealizedPNL: realizedPNL}}, nil
}

// [101, contract(11-field), action, totalQty, orderType, ...]
func decodeCompletedOrder(r *fieldReader) ([]Message, error) {
	// CompletedOrder uses the broad server->client order layout, with many
	// advanced order sections whose presence varies by order type and algo.
	// Decode only the public fields we expose, and anchor the tail on the
	// live status field instead of inventing a full advanced-order parser.
	contract := readWireContract(r)
	action := r.ReadString()
	quantity := r.ReadString()
	orderType := r.ReadString()
	// completedOrderStatusTail scans absolute field indices anchored at the
	// msg_id prefix that newFieldReader dropped; restore that view.
	fields := append([]string{itoa(InCompletedOrder)}, r.fields...)
	status, filled, err := completedOrderStatusTail(fields, orderType)
	if err != nil {
		return nil, err
	}
	return []Message{CompletedOrder{
		Contract: contract, Action: action, OrderType: orderType,
		Status: status, Quantity: quantity, Filled: filled,
	}}, nil
}

// [102]
func decodeCompletedOrderEnd(r *fieldReader) ([]Message, error) {
	return []Message{CompletedOrderEnd{}}, nil
}

// [77, reqId, count, (name, value, displayName) * count]
func decodeSoftDollarTiers(r *fieldReader) ([]Message, error) {
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

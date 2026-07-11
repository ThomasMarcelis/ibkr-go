package main

import (
	"net"
	"strconv"
)

type comboLegSpec struct {
	ConID              int
	Ratio              int
	Action             string
	Exchange           string
	OpenClose          string
	ShortSaleSlot      string
	DesignatedLocation string
	ExemptCode         string
}

type tagValueSpec struct {
	Tag   string
	Value string
}

type orderConditionSpec struct {
	Type          int
	Conjunction   string
	Operator      int
	ConID         int
	Exchange      string
	Value         string
	TriggerMethod int
	SecType       string
	Symbol        string
}

// orderSpec holds the order fields needed by the capture tool.
type orderSpec struct {
	Action                  string // "BUY", "SELL"
	TotalQuantity           string // "1", "100", etc.
	OrderType               string // "MKT", "LMT", "STP", "STP LMT"
	LmtPrice                string // empty for MKT
	AuxPrice                string // stop price for STP, empty for LMT/MKT
	TIF                     string // "DAY", "GTC"
	Account                 string
	Transmit                bool
	ParentID                int64 // 0 = no parent
	OcaGroup                string
	OutsideRTH              bool
	OrderRef                string
	ComboLegs               []comboLegSpec
	OrderComboLegPrices     []string
	SmartComboRoutingParams []tagValueSpec
	AlgoStrategy            string
	AlgoParams              []tagValueSpec
	Conditions              []orderConditionSpec
	ConditionsIgnoreRTH     bool
	ConditionsCancelOrder   bool
}

// contractSpec is a minimal contract shape for request building. Any field
// that is empty on the wire is sent as an empty string.
type contractSpec struct {
	ConID                        int
	Symbol                       string
	SecType                      string
	LastTradeDateOrContractMonth string
	Strike                       float64
	Right                        string
	Multiplier                   string
	Exchange                     string
	PrimaryExchange              string
	Currency                     string
	LocalSymbol                  string
	TradingClass                 string
	IncludeExpired               bool
	IssuerID                     string
}

// contractFields returns the standard contract field layout used by most
// feature requests. It covers conId, symbol, secType, lastTradeDate, strike,
// right, multiplier, exchange, primaryExchange, currency, localSymbol,
// tradingClass, includeExpired. The caller is responsible for appending
// sec_id_type/sec_id/etc if needed.
func contractRequestFields(c contractSpec) []string {
	return []string{
		strconv.Itoa(c.ConID),
		c.Symbol,
		c.SecType,
		c.LastTradeDateOrContractMonth,
		strconv.FormatFloat(c.Strike, 'f', -1, 64),
		c.Right,
		c.Multiplier,
		c.Exchange,
		c.PrimaryExchange,
		c.Currency,
		c.LocalSymbol,
		c.TradingClass,
		boolField(c.IncludeExpired),
	}
}

// contractRequestFieldsNoExpired is the same minus the includeExpired flag,
// for requests like REQ_MKT_DATA that don't carry it.
func contractRequestFieldsNoExpired(c contractSpec) []string {
	return []string{
		strconv.Itoa(c.ConID),
		c.Symbol,
		c.SecType,
		c.LastTradeDateOrContractMonth,
		strconv.FormatFloat(c.Strike, 'f', -1, 64),
		c.Right,
		c.Multiplier,
		c.Exchange,
		c.PrimaryExchange,
		c.Currency,
		c.LocalSymbol,
		c.TradingClass,
	}
}

func boolField(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// --- Account summary (msg_id=62) / cancel (msg_id=63) ---
//
//	[62, version=1, reqId, group, tags]
//	[63, version=1, reqId]
func sendReqAccountSummary(conn net.Conn, reqID int, group, tags string) error {
	return sendMessage(conn, []string{"62", "1", strconv.Itoa(reqID), group, tags})
}

func sendCancelAccountSummary(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"63", "1", strconv.Itoa(reqID)})
}

// --- Positions (msg_id=61) ---
//
//	[61, version=1]
func sendReqPositions(conn net.Conn) error {
	return sendMessage(conn, []string{"61", "1"})
}

// --- Market data (msg_id=1) / cancel (msg_id=2) ---
//
//	[1, version=11, reqId, <contract fields no includeExpired>,
//	 (combo legs if BAG), (delta neutral if v>=40),
//	 genericTickList, snapshot, regulatorySnapshot, mktDataOptions]
func sendReqMktData(conn net.Conn, reqID, _ int, c contractSpec, genericTicks string, snapshot bool) error {
	fields := []string{"1", "11", strconv.Itoa(reqID)}
	fields = append(fields, contractRequestFieldsNoExpired(c)...)
	// Skip BAG combo legs (not used for STK here).
	fields = append(fields, "0") // deltaNeutralContract present bool = false
	fields = append(fields,
		genericTicks,
		boolField(snapshot),
		"0", // regulatorySnapshot=false
		"",  // mktDataOptions empty tag-value list
	)
	return sendMessage(conn, fields)
}

// sendReqEFPMarketData sends the BAG shape used by IBKR's EFP sample: one
// single-stock future leg against the future multiplier's number of shares.
// EFP tick IDs 38-44 are default outputs for this contract, not values for the
// genericTickList request field.
func sendReqEFPMarketData(conn net.Conn, reqID int, c contractSpec, legs []comboLegSpec) error {
	fields := []string{"1", "11", strconv.Itoa(reqID)}
	fields = append(fields, contractRequestFieldsNoExpired(c)...)
	fields = append(fields, strconv.Itoa(len(legs)))
	for _, leg := range legs {
		fields = append(fields,
			strconv.Itoa(leg.ConID),
			strconv.Itoa(leg.Ratio),
			leg.Action,
			leg.Exchange,
		)
	}
	fields = append(fields,
		"0", // deltaNeutralContract present bool = false
		"",  // genericTickList; EFP ticks are automatic
		"0", // snapshot=false
		"0", // regulatorySnapshot=false
		"",  // mktDataOptions
	)
	return sendMessage(conn, fields)
}

func sendCancelMktData(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"2", "2", strconv.Itoa(reqID)})
}

// --- Executions (msg_id=7) ---
//
//	[7, version=3, reqId, filter.clientId, filter.acctCode, filter.time,
//	 filter.symbol, filter.secType, filter.exchange, filter.side,
//	 filter.lastNDays, filter.specificDatesCount]
//
// filter.clientId is an int on the wire; sending "0" matches the ibapi
// ExecutionFilter default and means "no filter". Empty string fails int
// parsing on the server and causes the request to be silently dropped.
func sendReqExecutions(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"7", "3", strconv.Itoa(reqID), "0", "", "", "", "", "", "", "2147483647", "0"})
}

// --- Market data type (msg_id=59) ---
//
//	[59, version=1, dataType]
//
// dataType: 1=live, 2=frozen, 3=delayed, 4=delayed-frozen.
// Used to fall back to delayed ticks when the account lacks live subscriptions.
func sendReqMarketDataType(conn net.Conn, dataType int) error {
	return sendMessage(conn, []string{"59", "1", strconv.Itoa(dataType)})
}

// --- Completed orders (msg_id=99) ---
//
//	[99, apiOnly]
func sendReqCompletedOrders(conn net.Conn, apiOnly bool) error {
	return sendMessage(conn, []string{"99", boolField(apiOnly)})
}

// --- Cancel historical data (msg_id=25) ---
//
//	[25, version=1, reqId]
func sendCancelHistoricalData(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"25", "1", strconv.Itoa(reqID)})
}

// --- Account updates (msg_id=6) ---
//
//	[6, version=2, subscribe, acctCode]
func sendReqAccountUpdates(conn net.Conn, subscribe bool, acctCode string) error {
	return sendMessage(conn, []string{"6", "2", boolField(subscribe), acctCode})
}

// --- PnL (msg_id=92) / cancel (msg_id=93) ---
//
//	[92, reqId, account, modelCode]
//	[93, reqId]
func sendReqPnL(conn net.Conn, reqID int, account, modelCode string) error {
	return sendMessage(conn, []string{"92", strconv.Itoa(reqID), account, modelCode})
}

func sendCancelPnL(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"93", strconv.Itoa(reqID)})
}

// --- PnL single (msg_id=94) / cancel (msg_id=95) ---
//
//	[94, reqId, account, modelCode, conId]
//	[95, reqId]
func sendReqPnLSingle(conn net.Conn, reqID int, account, modelCode string, conID int) error {
	return sendMessage(conn, []string{"94", strconv.Itoa(reqID), account, modelCode, strconv.Itoa(conID)})
}

func sendCancelPnLSingle(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"95", strconv.Itoa(reqID)})
}

// --- News bulletins (msg_id=12) / cancel (msg_id=13) ---
//
//	[12, version=1, allMessages]
//	[13, version=1]
func sendReqNewsBulletins(conn net.Conn, allMessages bool) error {
	return sendMessage(conn, []string{"12", "1", boolField(allMessages)})
}

func sendCancelNewsBulletins(conn net.Conn) error {
	return sendMessage(conn, []string{"13", "1"})
}

// --- Historical data with keepUpToDate (msg_id=20) ---
//
// Uses keepUpToDate=true; endDateTime must be empty and barSize at least 5s.
func sendReqHistoricalDataKeepUp(conn net.Conn, reqID, _ int, c contractSpec, barSize, whatToShow string, useRTH bool) error {
	fields := []string{"20", strconv.Itoa(reqID)}
	fields = append(fields, contractRequestFields(c)...)
	fields = append(fields,
		"",       // endDateTime must be empty for keepUpToDate
		barSize,  // e.g. "5 secs"
		"3600 S", // duration
		boolField(useRTH),
		whatToShow,
		"1", // formatDate
	)
	fields = append(fields,
		"1", // keepUpToDate=true
		"",  // chartOptions
	)
	return sendMessage(conn, fields)
}

// --- Place order (msg_id=3) ---
//
// On server v>=145 the version field is elided. Layout:
//
//	[3, orderID, conId, symbol, secType, lastTradeDate, strike, right,
//	 multiplier, exchange, primaryExchange, currency, localSymbol, tradingClass,
//	 secIdType, secId, action, totalQuantity, orderType, lmtPrice, auxPrice,
//	 tif, ocaGroup, account, openClose, origin, orderRef, transmit, parentId,
//	 blockOrder, sweepToFill, displaySize, triggerMethod, outsideRTH, hidden,
//	 (BAG combo legs - skip for non-BAG),
//	 deprecated_sharesAllocation, discretionaryAmt, goodAfterTime, goodTillDate,
//	 faGroup, faMethod, faPercentage, modelCode,
//	 shortSaleSlot, designatedLocation, exemptCode,
//	 ocaType, rule80A, settlingFirm, allOrNone, minQty, percentOffset,
//	 deprecated_eTradeOnly, deprecated_firmQuoteOnly, deprecated_nbboPriceCap,
//	 auctionStrategy, startingPrice, stockRefPrice, delta, stockRangeLower,
//	 stockRangeUpper, overridePercentageConstraints,
//	 volatility, volatilityType, deltaNeutralOrderType, deltaNeutralAuxPrice,
//	 continuousUpdate, referencePriceType,
//	 trailStopPrice, trailingPercent,
//	 scaleInitLevelSize, scaleSubsLevelSize, scalePriceIncrement,
//	 scaleTable, activeStartTime, activeStopTime,
//	 hedgeType, optOutSmartRouting, clearingAccount, clearingIntent, notHeld,
//	 deltaNeutralContractPresent,
//	 algoStrategy, algoID, whatIf, orderMiscOptions, solicited,
//	 randomizeSize, randomizePrice,
//	 conditionsCount,
//	 adjustedOrderType, triggerPrice, lmtPriceOffset, adjustedStopPrice,
//	 adjustedStopLimitPrice, adjustedTrailingAmount, adjustableTrailingUnit,
//	 extOperator, softDollarName, softDollarValue,
//	 cashQty, mifid2DecisionMaker, mifid2DecisionAlgo,
//	 mifid2ExecutionTrader, mifid2ExecutionAlgo,
//	 dontUseAutoPriceForHedge, isOmsContainer, discretionaryUpToLimitPrice,
//	 usePriceMgmtAlgo, duration, postToAts, autoCancelParent,
//	 advancedErrorOverride, manualOrderTime,
//	 customerAccount, professionalCustomer,
//	 includeOvernight, manualOrderIndicator, imbalanceOnly]
func sendPlaceOrder(conn net.Conn, orderID int64, c contractSpec, o orderSpec) error {
	fields := []string{
		"3",
		strconv.FormatInt(orderID, 10),
	}
	// Contract: conId through tradingClass, plus secIdType and secId.
	fields = append(fields, contractRequestFieldsNoExpired(c)...)
	fields = append(fields,
		"", // secIdType
		"", // secId
	)
	// Main order fields.
	fields = append(fields,
		o.Action,
		o.TotalQuantity,
		o.OrderType,
		o.LmtPrice,
		o.AuxPrice,
	)
	// Extended order fields.
	fields = append(fields,
		o.TIF,
		o.OcaGroup,
		o.Account,
		"",  // openClose
		"0", // origin = customer
		o.OrderRef,
		boolField(o.Transmit),
		strconv.FormatInt(o.ParentID, 10),
		"0", // blockOrder
		"0", // sweepToFill
		"0", // displaySize
		"0", // triggerMethod
		boolField(o.OutsideRTH),
		"0", // hidden
	)
	if c.SecType == "BAG" || len(o.ComboLegs) > 0 || len(o.OrderComboLegPrices) > 0 || len(o.SmartComboRoutingParams) > 0 {
		fields = append(fields, strconv.Itoa(len(o.ComboLegs)))
		for _, leg := range o.ComboLegs {
			fields = append(fields,
				strconv.Itoa(leg.ConID),
				strconv.Itoa(leg.Ratio),
				leg.Action,
				leg.Exchange,
				leg.OpenClose,
				leg.ShortSaleSlot,
				leg.DesignatedLocation,
				leg.ExemptCode,
			)
		}
		fields = append(fields, strconv.Itoa(len(o.OrderComboLegPrices)))
		fields = append(fields, o.OrderComboLegPrices...)
		fields = append(fields, strconv.Itoa(len(o.SmartComboRoutingParams)))
		for _, value := range o.SmartComboRoutingParams {
			fields = append(fields, value.Tag, value.Value)
		}
	}
	// Deprecated + FA + model.
	fields = append(fields,
		"",  // deprecated sharesAllocation
		"0", // discretionaryAmt
		"",  // goodAfterTime
		"",  // goodTillDate
		"",  // faGroup
		"",  // faMethod
		"",  // faPercentage
		"",  // modelCode
	)
	// Short sale.
	fields = append(fields,
		"0",  // shortSaleSlot
		"",   // designatedLocation
		"-1", // exemptCode
	)
	// Order type extensions.
	fields = append(fields,
		"0", // ocaType
		"",  // rule80A
		"",  // settlingFirm
		"0", // allOrNone
		"",  // minQty (UNSET)
		"",  // percentOffset (UNSET)
		"0", // deprecated eTradeOnly
		"0", // deprecated firmQuoteOnly
		"",  // deprecated nbboPriceCap (UNSET)
		"0", // auctionStrategy
		"",  // startingPrice
		"",  // stockRefPrice
		"",  // delta
		"",  // stockRangeLower
		"",  // stockRangeUpper
		"0", // overridePercentageConstraints
	)
	// Volatility.
	fields = append(fields,
		"",  // volatility
		"",  // volatilityType
		"",  // deltaNeutralOrderType
		"",  // deltaNeutralAuxPrice
		"0", // continuousUpdate
		"0", // referencePriceType
	)
	// Trailing.
	fields = append(fields,
		"", // trailStopPrice
		"", // trailingPercent
	)
	// Scale.
	fields = append(fields,
		"", // scaleInitLevelSize
		"", // scaleSubsLevelSize
		"", // scalePriceIncrement
	)
	// scalePriceIncrement empty => skip scale3 extended.
	fields = append(fields,
		"", // scaleTable
		"", // activeStartTime
		"", // activeStopTime
	)
	// Hedge: empty hedgeType => no hedgeParam.
	fields = append(fields,
		"", // hedgeType
	)
	// Misc.
	fields = append(fields,
		"0", // optOutSmartRouting
		"",  // clearingAccount
		"",  // clearingIntent
		"0", // notHeld
		"0", // deltaNeutralContractPresent
	)
	// Algo.
	fields = append(fields, o.AlgoStrategy)
	if o.AlgoStrategy != "" {
		fields = append(fields, strconv.Itoa(len(o.AlgoParams)))
		for _, value := range o.AlgoParams {
			fields = append(fields, value.Tag, value.Value)
		}
	}
	fields = append(fields,
		"",  // algoID
		"0", // whatIf
		"",  // orderMiscOptions
		"0", // solicited
		"0", // randomizeSize
		"0", // randomizePrice
	)
	// PEG BENCH fields skipped (orderType != "PEG BENCH").
	fields = append(fields, strconv.Itoa(len(o.Conditions)))
	for _, cond := range o.Conditions {
		fields = append(fields, strconv.Itoa(cond.Type))
		if cond.Conjunction == "o" {
			fields = append(fields, "o")
		} else {
			fields = append(fields, "a")
		}
		// Value precedes the conId/exchange pair, matching the official
		// client hierarchy; the Gateway rejects the reversed order with
		// code 320 (see the codec condition field-order fix).
		switch cond.Type {
		case 1:
			fields = append(fields, boolField(cond.Operator == 2), cond.Value, strconv.Itoa(cond.ConID), cond.Exchange, strconv.Itoa(cond.TriggerMethod))
		case 3, 4:
			fields = append(fields, boolField(cond.Operator == 2), cond.Value)
		case 5:
			fields = append(fields, cond.SecType, cond.Exchange, cond.Symbol)
		case 6, 7:
			fields = append(fields, boolField(cond.Operator == 2), cond.Value, strconv.Itoa(cond.ConID), cond.Exchange)
		}
	}
	if len(o.Conditions) > 0 {
		fields = append(fields, boolField(o.ConditionsIgnoreRTH), boolField(o.ConditionsCancelOrder))
	}
	// Adjusted order type fields.
	fields = append(fields,
		"",  // adjustedOrderType
		"",  // triggerPrice
		"",  // lmtPriceOffset
		"",  // adjustedStopPrice
		"",  // adjustedStopLimitPrice
		"",  // adjustedTrailingAmount
		"0", // adjustableTrailingUnit
	)
	fields = append(fields,
		"",  // extOperator
		"",  // softDollarName
		"",  // softDollarValue
		"",  // cashQty
		"",  // mifid2DecisionMaker
		"",  // mifid2DecisionAlgo
		"",  // mifid2ExecutionTrader
		"",  // mifid2ExecutionAlgo
		"0", // dontUseAutoPriceForHedge
		"0", // isOmsContainer
		"0", // discretionaryUpToLimitPrice
		"",  // usePriceMgmtAlgo
		"",  // duration
		"",  // postToAts
		"0", // autoCancelParent
		"",  // advancedErrorOverride
		"",  // manualOrderTime
	)
	// PEG BEST/MID offsets skipped.
	fields = append(fields,
		"",  // customerAccount
		"0", // professionalCustomer
		"0", // includeOvernight
		"",  // manualOrderIndicator
		"0", // imbalanceOnly
	)
	return sendMessage(conn, fields)
}

// --- Cancel order (msg_id=4) ---
//
//	[4, orderID, manualOrderCancelTime, extOperator, manualOrderIndicator]
//
// At server_version >= 192 (CME_TAGGING_FIELDS), extOperator and
// manualOrderIndicator are required. Empty manualOrderIndicator means "not set".
func sendCancelOrder(conn net.Conn, orderID int64) error {
	return sendMessage(conn, []string{"4", strconv.FormatInt(orderID, 10), "", "", ""})
}

// --- Global cancel (msg_id=58) ---
//
//	[58, extOperator, manualOrderIndicator]
func sendGlobalCancel(conn net.Conn) error {
	return sendMessage(conn, []string{"58", "", ""})
}

// --- Request FA (msg_id=18) ---
//
//	[18, version=1, faDataType]
func sendRequestFA(conn net.Conn, faDataType int) error {
	return sendMessage(conn, []string{"18", "1", strconv.Itoa(faDataType)})
}

// --- Query display groups (msg_id=67) ---
//
//	[67, version=1, reqId]
func sendQueryDisplayGroups(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"67", "1", strconv.Itoa(reqID)})
}

// --- Subscribe to group events (msg_id=68) ---
//
//	[68, version=1, reqId, groupId]
func sendSubscribeToGroupEvents(conn net.Conn, reqID, groupID int) error {
	return sendMessage(conn, []string{"68", "1", strconv.Itoa(reqID), strconv.Itoa(groupID)})
}

// --- Unsubscribe from group events (msg_id=70) ---
//
//	[70, version=1, reqId]
func sendUnsubscribeFromGroupEvents(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"70", "1", strconv.Itoa(reqID)})
}

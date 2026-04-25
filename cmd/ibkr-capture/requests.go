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

// --- Session-level one-shots ---
//
//	reqCurrentTime:  [49, version=1]
//	reqIds:          [8, version=1, numIds]
func sendReqCurrentTime(conn net.Conn) error {
	return sendMessage(conn, []string{"49", "1"})
}

func sendReqIds(conn net.Conn, numIDs int) error {
	if numIDs <= 0 {
		numIDs = 1
	}
	return sendMessage(conn, []string{"8", "1", strconv.Itoa(numIDs)})
}

// --- Contract details (msg_id=9) ---
//
// Layout on server v200 (trading_class and issuer_id gates active):
//
//	[9, version=8, reqId, conId, symbol, secType, lastTradeDateOrContractMonth,
//	 strike, right, multiplier, exchange, primaryExchange, currency, localSymbol,
//	 tradingClass, includeExpired, secIdType, secId, issuerId]
func sendReqContractDetails(conn net.Conn, reqID int, c contractSpec) error {
	fields := []string{"9", "8", strconv.Itoa(reqID)}
	fields = append(fields, contractRequestFields(c)...)
	fields = append(fields,
		"", // secIdType
		"", // secId
		"", // issuerId (v>=MinServerVersionBondIssuerId)
	)
	return sendMessage(conn, fields)
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

// --- Positions (msg_id=61) / cancel (msg_id=64) ---
//
//	[61, version=1]
//	[64, version=1]
func sendReqPositions(conn net.Conn) error {
	return sendMessage(conn, []string{"61", "1"})
}

func sendCancelPositions(conn net.Conn) error {
	return sendMessage(conn, []string{"64", "1"})
}

// --- Historical bars (msg_id=20) / cancel (msg_id=25) ---
//
// On server v>=124 the version field is elided. Layout:
//
//	[20, reqId, <contract fields+tradingClass+includeExpired>,
//	 endDateTime, barSize, duration, useRTH, whatToShow, formatDate,
//	 (combo legs iff BAG), keepUpToDate, chartOptions]
func sendReqHistoricalData(conn net.Conn, reqID, _ int, c contractSpec, endDateTime, duration, barSize, whatToShow string, useRTH bool) error {
	fields := []string{"20", strconv.Itoa(reqID)}
	fields = append(fields, contractRequestFields(c)...)
	fields = append(fields,
		endDateTime,
		barSize,
		duration,
		boolField(useRTH),
		whatToShow,
		"1", // formatDate=1 (Eastern Time string)
	)
	// BAG combo legs go here if sec_type == "BAG" — skipped for STK/FUT/CASH/OPT/IND.
	fields = append(fields,
		"0", // keepUpToDate=false
		"",  // chartOptions (empty tag-value list)
	)
	return sendMessage(conn, fields)
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

func sendCancelMktData(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"2", "2", strconv.Itoa(reqID)})
}

// --- Real-time bars (msg_id=50) / cancel (msg_id=51) ---
//
//	[50, version=3, reqId, <contract fields no includeExpired>, barSize, whatToShow, useRTH, realTimeBarsOptions]
func sendReqRealTimeBars(conn net.Conn, reqID, _ int, c contractSpec, whatToShow string, useRTH bool) error {
	fields := []string{"50", "3", strconv.Itoa(reqID)}
	fields = append(fields, contractRequestFieldsNoExpired(c)...)
	fields = append(fields,
		"5", // barSize hardcoded to 5 seconds (only valid value)
		whatToShow,
		boolField(useRTH),
		"", // realTimeBarsOptions empty tag-value list
	)
	return sendMessage(conn, fields)
}

func sendCancelRealTimeBars(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"51", "1", strconv.Itoa(reqID)})
}

// --- Open orders variants ---
//
//	[5, version=1]    REQ_OPEN_ORDERS (client's own)
//	[16, version=1]   REQ_ALL_OPEN_ORDERS (all clients, requires client_id=0)
//	[15, version=1, autoBind] REQ_AUTO_OPEN_ORDERS
func sendReqOpenOrders(conn net.Conn) error {
	return sendMessage(conn, []string{"5", "1"})
}

func sendReqAllOpenOrders(conn net.Conn) error {
	return sendMessage(conn, []string{"16", "1"})
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

// --- Family codes (msg_id=80) ---
//
//	[80]
func sendReqFamilyCodes(conn net.Conn) error {
	return sendMessage(conn, []string{"80"})
}

// --- News providers (msg_id=85) ---
//
//	[85]
func sendReqNewsProviders(conn net.Conn) error {
	return sendMessage(conn, []string{"85"})
}

// --- Market depth exchanges (msg_id=82) ---
//
//	[82]
func sendReqMktDepthExchanges(conn net.Conn) error {
	return sendMessage(conn, []string{"82"})
}

// --- Scanner parameters (msg_id=24) ---
//
//	[24, version=1]
func sendReqScannerParameters(conn net.Conn) error {
	return sendMessage(conn, []string{"24", "1"})
}

// --- User info (msg_id=104) ---
//
//	[104, version=1, reqId]
func sendReqUserInfo(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"104", "1", strconv.Itoa(reqID)})
}

// --- Matching symbols (msg_id=81) ---
//
//	[81, reqId, pattern]
func sendReqMatchingSymbols(conn net.Conn, reqID int, pattern string) error {
	return sendMessage(conn, []string{"81", strconv.Itoa(reqID), pattern})
}

// --- Market rule (msg_id=91) ---
//
//	[91, marketRuleId]
func sendReqMarketRule(conn net.Conn, marketRuleID int) error {
	return sendMessage(conn, []string{"91", strconv.Itoa(marketRuleID)})
}

// --- Head timestamp (msg_id=87) ---
//
//	[87, reqId, contract fields (13), includeExpired, useRTH, whatToShow, formatDate]
func sendReqHeadTimestamp(conn net.Conn, reqID int, c contractSpec, whatToShow string, useRTH bool) error {
	fields := []string{"87", strconv.Itoa(reqID)}
	fields = append(fields, contractRequestFields(c)...)
	fields = append(fields,
		"0", // includeExpired
		boolField(useRTH),
		whatToShow,
		"1", // formatDate
	)
	return sendMessage(conn, fields)
}

// --- Cancel head timestamp (msg_id=90) ---
//
//	[90, reqId]
func sendCancelHeadTimestamp(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"90", strconv.Itoa(reqID)})
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

// --- Account updates multi (msg_id=76) / cancel (msg_id=77) ---
//
//	[76, version=1, reqId, account, modelCode, subscribe=true]
//	[77, version=1, reqId]
func sendReqAccountUpdatesMulti(conn net.Conn, reqID int, account, modelCode string) error {
	return sendMessage(conn, []string{"76", "1", strconv.Itoa(reqID), account, modelCode, "1"})
}

func sendCancelAccountUpdatesMulti(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"77", "1", strconv.Itoa(reqID)})
}

// --- Positions multi (msg_id=74) / cancel (msg_id=75) ---
//
//	[74, version=1, reqId, account, modelCode]
//	[75, version=1, reqId]
func sendReqPositionsMulti(conn net.Conn, reqID int, account, modelCode string) error {
	return sendMessage(conn, []string{"74", "1", strconv.Itoa(reqID), account, modelCode})
}

func sendCancelPositionsMulti(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"75", "1", strconv.Itoa(reqID)})
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

// --- Tick by tick (msg_id=97) / cancel (msg_id=98) ---
//
//	[97, reqId, contract(12), tickType, numberOfTicks, ignoreSize]
//	[98, reqId]
func sendReqTickByTickData(conn net.Conn, reqID int, c contractSpec, tickType string, numberOfTicks int, ignoreSize bool) error {
	fields := []string{"97", strconv.Itoa(reqID)}
	fields = append(fields, contractRequestFieldsNoExpired(c)...)
	fields = append(fields,
		tickType,
		strconv.Itoa(numberOfTicks),
		boolField(ignoreSize),
	)
	return sendMessage(conn, fields)
}

func sendCancelTickByTickData(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"98", strconv.Itoa(reqID)})
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

// --- Sec def opt params (msg_id=78) ---
//
//	[78, reqId, underlyingSymbol, futFopExchange, underlyingSecType, underlyingConId]
func sendReqSecDefOptParams(conn net.Conn, reqID int, symbol, futFopExchange, secType string, conID int) error {
	return sendMessage(conn, []string{"78", strconv.Itoa(reqID), symbol, futFopExchange, secType, strconv.Itoa(conID)})
}

// --- Smart components (msg_id=83) ---
//
//	[83, reqId, bboExchange]
func sendReqSmartComponents(conn net.Conn, reqID int, bboExchange string) error {
	return sendMessage(conn, []string{"83", strconv.Itoa(reqID), bboExchange})
}

// --- Histogram data (msg_id=88) / cancel (msg_id=89) ---
//
//	[88, reqId, contract(13), useRTH, timePeriod]
//	[89, reqId]
func sendReqHistogramData(conn net.Conn, reqID int, c contractSpec, useRTH bool, timePeriod string) error {
	fields := []string{"88", strconv.Itoa(reqID)}
	fields = append(fields, contractRequestFields(c)...)
	fields = append(fields, boolField(useRTH), timePeriod)
	return sendMessage(conn, fields)
}

func sendCancelHistogramData(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"89", strconv.Itoa(reqID)})
}

// --- Historical ticks (msg_id=96) ---
//
//	[96, reqId, contract(13), startDateTime, endDateTime, numberOfTicks,
//	 whatToShow, useRTH, ignoreSize, miscOptions]
func sendReqHistoricalTicks(conn net.Conn, reqID int, c contractSpec, startDateTime, endDateTime string, numberOfTicks int, whatToShow string, useRTH bool, ignoreSize bool) error {
	fields := []string{"96", strconv.Itoa(reqID)}
	fields = append(fields, contractRequestFields(c)...)
	fields = append(fields,
		startDateTime,
		endDateTime,
		strconv.Itoa(numberOfTicks),
		whatToShow,
		boolField(useRTH),
		boolField(ignoreSize),
		"", // miscOptions
	)
	return sendMessage(conn, fields)
}

// --- Calc implied volatility (msg_id=54) / cancel (msg_id=56) ---
//
//	[54, version=3, reqId, contract(13), optionPrice, underPrice, implVolOptions]
//	[56, version=1, reqId]
func sendReqCalcImpliedVolatility(conn net.Conn, reqID int, c contractSpec, optionPrice, underPrice float64) error {
	fields := []string{"54", "3", strconv.Itoa(reqID)}
	fields = append(fields, contractRequestFields(c)...)
	fields = append(fields,
		strconv.FormatFloat(optionPrice, 'f', -1, 64),
		strconv.FormatFloat(underPrice, 'f', -1, 64),
		"", // implVolOptions
	)
	return sendMessage(conn, fields)
}

func sendCancelCalcImpliedVolatility(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"56", "1", strconv.Itoa(reqID)})
}

// --- Calc option price (msg_id=55) / cancel (msg_id=57) ---
//
//	[55, version=3, reqId, contract(13), volatility, underPrice, optPxOptions]
//	[57, version=1, reqId]
func sendReqCalcOptionPrice(conn net.Conn, reqID int, c contractSpec, volatility, underPrice float64) error {
	fields := []string{"55", "3", strconv.Itoa(reqID)}
	fields = append(fields, contractRequestFields(c)...)
	fields = append(fields,
		strconv.FormatFloat(volatility, 'f', -1, 64),
		strconv.FormatFloat(underPrice, 'f', -1, 64),
		"", // optPxOptions
	)
	return sendMessage(conn, fields)
}

func sendCancelCalcOptionPrice(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"57", "1", strconv.Itoa(reqID)})
}

// --- News article (msg_id=84) ---
//
//	[84, reqId, providerCode, articleId, newsArticleOptions]
func sendReqNewsArticle(conn net.Conn, reqID int, providerCode, articleID string) error {
	return sendMessage(conn, []string{"84", strconv.Itoa(reqID), providerCode, articleID, ""})
}

// --- Historical news (msg_id=86) ---
//
//	[86, reqId, conId, providerCodes, startDate, endDate, totalResults, historicalNewsOptions]
func sendReqHistoricalNews(conn net.Conn, reqID int, conID int, providerCodes, startDate, endDate string, totalResults int) error {
	return sendMessage(conn, []string{"86", strconv.Itoa(reqID), strconv.Itoa(conID), providerCodes, startDate, endDate, strconv.Itoa(totalResults), ""})
}

// --- Scanner subscription (msg_id=22) / cancel (msg_id=23) ---
//
//	[22, reqId, numberOfRows, instrument, locationCode, scanCode,
//	 abovePrice, belowPrice, aboveVolume, marketCapAbove, marketCapBelow,
//	 moodyRatingAbove, moodyRatingBelow, spRatingAbove, spRatingBelow,
//	 maturityDateAbove, maturityDateBelow, couponRateAbove, couponRateBelow,
//	 excludeConvertible, averageOptionVolumeAbove, scannerSettingPairs,
//	 stockTypeFilter, scannerSubscriptionFilterOptions,
//	 scannerSubscriptionOptions]
func sendReqScannerSubscription(conn net.Conn, reqID int, numberOfRows int, instrument, locationCode, scanCode string) error {
	const maxDouble = "1.7976931348623157E308"
	const maxInt = "2147483647"
	fields := []string{
		"22", strconv.Itoa(reqID),
		strconv.Itoa(numberOfRows),
		instrument,
		locationCode,
		scanCode,
		maxDouble, // abovePrice
		maxDouble, // belowPrice
		maxInt,    // aboveVolume
		maxDouble, // marketCapAbove
		maxDouble, // marketCapBelow
		"",        // moodyRatingAbove
		"",        // moodyRatingBelow
		"",        // spRatingAbove
		"",        // spRatingBelow
		"",        // maturityDateAbove
		"",        // maturityDateBelow
		maxDouble, // couponRateAbove
		maxDouble, // couponRateBelow
		"",        // excludeConvertible
		maxInt,    // averageOptionVolumeAbove
		"",        // scannerSettingPairs
		"",        // stockTypeFilter
		"",        // scannerSubscriptionFilterOptions (tag-value list)
		"",        // scannerSubscriptionOptions (tag-value list)
	}
	return sendMessage(conn, fields)
}

func sendCancelScannerSubscription(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"23", "1", strconv.Itoa(reqID)})
}

// --- Historical data with keepUpToDate (msg_id=20) ---
//
// Same as sendReqHistoricalData but with keepUpToDate=true. Requires
// endDateTime="" and barSize >= 5s.
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
		switch cond.Type {
		case 1:
			fields = append(fields, boolField(cond.Operator == 2), strconv.Itoa(cond.ConID), cond.Exchange, cond.Value, strconv.Itoa(cond.TriggerMethod))
		case 3, 4:
			fields = append(fields, boolField(cond.Operator == 2), cond.Value)
		case 5:
			fields = append(fields, cond.SecType, cond.Exchange, cond.Symbol)
		case 6, 7:
			fields = append(fields, boolField(cond.Operator == 2), strconv.Itoa(cond.ConID), cond.Exchange, cond.Value)
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

// --- Market depth (msg_id=10) / cancel (msg_id=11) ---
//
//	[10, version=5, reqId, conId, symbol, secType, expiry, strike, right,
//	 multiplier, exchange, primaryExchange, currency, localSymbol, tradingClass,
//	 numRows, isSmartDepth, mktDepthOptions]
func sendReqMktDepth(conn net.Conn, reqID int, c contractSpec, numRows int, isSmartDepth bool) error {
	fields := []string{"10", "5", strconv.Itoa(reqID)}
	fields = append(fields, contractRequestFieldsNoExpired(c)...)
	fields = append(fields,
		strconv.Itoa(numRows),
		boolField(isSmartDepth),
		"", // mktDepthOptions
	)
	return sendMessage(conn, fields)
}

func sendCancelMktDepth(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"11", "1", strconv.Itoa(reqID)})
}

// --- Fundamental data (msg_id=52) / cancel (msg_id=53) ---
//
//	[52, version=2, reqId, conId, symbol, secType, exchange, primaryExchange,
//	 currency, localSymbol, reportType]
//	[53, version=1, reqId]
func sendReqFundamentalData(conn net.Conn, reqID int, c contractSpec, reportType string) error {
	return sendMessage(conn, []string{
		"52", "2", strconv.Itoa(reqID),
		strconv.Itoa(c.ConID),
		c.Symbol,
		c.SecType,
		c.Exchange,
		c.PrimaryExchange,
		c.Currency,
		c.LocalSymbol,
		reportType,
	})
}

func sendCancelFundamentalData(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"53", "1", strconv.Itoa(reqID)})
}

// --- Exercise options (msg_id=21) ---
//
//	[21, version=2, reqId, conId, symbol, secType, expiry, strike, right,
//	 multiplier, exchange, currency, localSymbol, tradingClass,
//	 exerciseAction, exerciseQuantity, account, override]
func sendExerciseOptions(conn net.Conn, reqID int, c contractSpec, exerciseAction, exerciseQuantity int, account string, override int) error {
	return sendMessage(conn, []string{
		"21", "2", strconv.Itoa(reqID),
		strconv.Itoa(c.ConID),
		c.Symbol,
		c.SecType,
		c.LastTradeDateOrContractMonth,
		strconv.FormatFloat(c.Strike, 'f', -1, 64),
		c.Right,
		c.Multiplier,
		c.Exchange,
		c.Currency,
		c.LocalSymbol,
		c.TradingClass,
		strconv.Itoa(exerciseAction),
		strconv.Itoa(exerciseQuantity),
		account,
		strconv.Itoa(override),
	})
}

// --- Request FA (msg_id=18) / Replace FA (msg_id=19) ---
//
//	[18, version=1, faDataType]
//	[19, version=1, faDataType, xml]
func sendRequestFA(conn net.Conn, faDataType int) error {
	return sendMessage(conn, []string{"18", "1", strconv.Itoa(faDataType)})
}

func sendReplaceFA(conn net.Conn, faDataType int, xml string) error {
	return sendMessage(conn, []string{"19", "1", strconv.Itoa(faDataType), xml})
}

// --- Soft dollar tiers (msg_id=79) ---
//
//	[79, reqId]
func sendReqSoftDollarTiers(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"79", strconv.Itoa(reqID)})
}

// --- WSH meta data (msg_id=100) / cancel (msg_id=101) ---
//
//	[100, reqId]
//	[101, reqId]
func sendReqWSHMetaData(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"100", strconv.Itoa(reqID)})
}

func sendCancelWSHMetaData(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"101", strconv.Itoa(reqID)})
}

// --- WSH event data (msg_id=102) / cancel (msg_id=103) ---
//
//	[102, reqId, conId, filter, fillWatchlist, fillPortfolio, fillCompetitors,
//	 startDate, endDate, totalLimit]
//	[103, reqId]
func sendReqWSHEventData(conn net.Conn, reqID, conID int, filter string, fillWatchlist, fillPortfolio, fillCompetitors bool, startDate, endDate string, totalLimit int) error {
	return sendMessage(conn, []string{
		"102", strconv.Itoa(reqID),
		strconv.Itoa(conID),
		filter,
		boolField(fillWatchlist),
		boolField(fillPortfolio),
		boolField(fillCompetitors),
		startDate,
		endDate,
		strconv.Itoa(totalLimit),
	})
}

func sendCancelWSHEventData(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"103", strconv.Itoa(reqID)})
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

// --- Update display group (msg_id=69) ---
//
//	[69, version=1, reqId, contractInfo]
func sendUpdateDisplayGroup(conn net.Conn, reqID int, contractInfo string) error {
	return sendMessage(conn, []string{"69", "1", strconv.Itoa(reqID), contractInfo})
}

// --- Unsubscribe from group events (msg_id=70) ---
//
//	[70, version=1, reqId]
func sendUnsubscribeFromGroupEvents(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"70", "1", strconv.Itoa(reqID)})
}

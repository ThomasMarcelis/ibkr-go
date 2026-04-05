package main

import (
	"net"
	"strconv"
)

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
//	 filter.symbol, filter.secType, filter.exchange, filter.side]
//
// filter.clientId is an int on the wire; sending "0" matches the ibapi
// ExecutionFilter default and means "no filter". Empty string fails int
// parsing on the server and causes the request to be silently dropped.
func sendReqExecutions(conn net.Conn, reqID int) error {
	return sendMessage(conn, []string{"7", "3", strconv.Itoa(reqID), "0", "", "", "", "", "", ""})
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

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

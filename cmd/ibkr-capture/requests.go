package main

import (
	"net"
	"strconv"
)

type comboLegSpec struct {
	ConID    int
	Ratio    int
	Action   string
	Exchange string
}

// contractSpec is the wire contract shape required by the EFP market-data
// probe. Empty fields remain empty on the wire.
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
}

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

// sendReqMarketDataType selects live, frozen, delayed, or delayed-frozen data.
func sendReqMarketDataType(conn net.Conn, dataType int) error {
	return sendMessage(conn, []string{"59", "1", strconv.Itoa(dataType)})
}

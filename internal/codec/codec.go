package codec

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

// EncodeHandshakePrefix returns the raw API prefix bytes sent before framing begins.
func EncodeHandshakePrefix() []byte {
	return []byte("API\x00")
}

// EncodeVersionRange returns the version negotiation payload (to be length-framed by caller).
func EncodeVersionRange(minVer, maxVer int) []byte {
	return []byte(fmt.Sprintf("v%d..%d", minVer, maxVer))
}

// DecodeServerInfo parses the server info frame returned during the handshake.
func DecodeServerInfo(payload []byte) (ServerInfo, error) {
	fields, err := wire.ParseFields(payload)
	if err != nil {
		return ServerInfo{}, err
	}
	if len(fields) < 2 {
		return ServerInfo{}, fmt.Errorf("codec: server info: want >= 2 fields, got %d", len(fields))
	}
	version, err := strconv.Atoi(fields[0])
	if err != nil {
		return ServerInfo{}, fmt.Errorf("codec: server info: parse version %q: %w", fields[0], err)
	}
	return ServerInfo{ServerVersion: version, ConnectionTime: fields[1]}, nil
}

// DecodeBatch decodes a framed payload into one or more messages keyed by integer msg_id.
func DecodeBatch(payload []byte) ([]Message, error) {
	fields, err := wire.ParseFields(payload)
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("codec: empty message")
	}
	msgID, err := strconv.Atoi(fields[0])
	if err != nil {
		return nil, fmt.Errorf("codec: parse msg_id %q: %w", fields[0], err)
	}
	return decodeByMsgID(msgID, fields)
}

// Decode decodes a framed payload into exactly one message.
func Decode(payload []byte) (Message, error) {
	msgs, err := DecodeBatch(payload)
	if err != nil {
		return nil, err
	}
	if len(msgs) != 1 {
		return nil, fmt.Errorf("codec: expected 1 message, got %d", len(msgs))
	}
	return msgs[0], nil
}

// Encode encodes a message in the real TWS wire format (integer msg_id prefix).
func Encode(msg Message) ([]byte, error) {
	fields, err := encodeFields(msg)
	if err != nil {
		return nil, err
	}
	return wire.EncodeFields(fields), nil
}

// EncodeWire is an alias for Encode.
func EncodeWire(msg Message) ([]byte, error) {
	return Encode(msg)
}

// decodeByMsgID dispatches on the integer message ID and reads fields in real TWS wire layout.
// Returns []Message because historical data packs multiple bars into one frame.
func decodeByMsgID(msgID int, fields []string) ([]Message, error) {
	r := newFieldReader(fields[1:]) // skip msg_id
	switch msgID {

	case InTickPrice: // [1, version, reqID, tickType, price, size, attrMask]
		r.Skip(1) // version
		reqID, _ := r.ReadInt()
		tickType, _ := r.ReadInt()
		price := r.ReadString()
		size := r.ReadString()
		attrMask, _ := r.ReadInt()
		return []Message{TickPrice{ReqID: reqID, TickType: tickType, Price: price, Size: size, AttrMask: attrMask}}, nil

	case InTickSize: // [2, version, reqID, tickType, size]
		r.Skip(1) // version
		reqID, _ := r.ReadInt()
		tickType, _ := r.ReadInt()
		size := r.ReadString()
		return []Message{TickSize{ReqID: reqID, TickType: tickType, Size: size}}, nil

	case InOrderStatus: // [3, orderId, status, filled, remaining, ...]
		orderID, _ := r.ReadInt64()
		status := r.ReadString()
		filled := r.ReadString()
		remaining := r.ReadString()
		return []Message{OrderStatus{OrderID: orderID, Status: status, Filled: filled, Remaining: remaining}}, nil

	case InErrMsg: // [4, reqId, code, message, advancedJson, errorTimeMs]
		reqID, _ := r.ReadInt()
		code, _ := r.ReadInt()
		message := r.ReadString()
		advJSON := r.ReadString()
		errTime := r.ReadString()
		return []Message{APIError{ReqID: reqID, Code: code, Message: message, AdvancedOrderRejectJSON: advJSON, ErrorTimeMs: errTime}}, nil

	case InOpenOrder:
		// v200 wire layout verified against live IB Gateway capture (server_version 200).
		// Field positions (0-indexed from after msg_id):
		//   [0]     orderID
		//   [1..11] contract (11-field server→client block)
		//   [12]    action
		//   [13]    totalQty
		//   [14]    orderType
		//   [15..18] lmtPrice, auxPrice, tif, ocaGroup
		//   [19]    account
		//   [20..90] 71 order detail fields (permId, FA params, algo, etc.)
		//   [91]    OrderState.status
		//   [92..159] 68 margin/state fields
		//   [160]   filled   (trailing order-status section)
		//   [161]   remaining
		orderID, _ := r.ReadInt64()
		contract := readWireContract(r)
		action := r.ReadString()
		quantity := r.ReadString()
		orderType := r.ReadString()
		r.Skip(4)  // lmtPrice, auxPrice, tif, ocaGroup
		account := r.ReadString()
		r.Skip(71) // order detail fields through to OrderState
		status := r.ReadString()
		r.Skip(68) // margin/state fields through to trailing order-status
		filled := r.ReadString()
		remaining := r.ReadString()
		return []Message{OpenOrder{
			OrderID: orderID, Account: account, Contract: contract,
			Action: action, OrderType: orderType,
			Status: status, Quantity: quantity, Filled: filled, Remaining: remaining,
		}}, nil

	case InNextValidID: // [9, version, orderID]
		r.Skip(1) // version
		orderID, err := r.ReadInt64()
		if err != nil {
			return nil, err
		}
		return []Message{NextValidID{OrderID: orderID}}, nil

	case InContractData: // v200 wire layout verified against live IB Gateway capture.
		// [10, reqID, symbol, secType, lastTradeDate, lastTradeDateOrContractMonth,
		//   strike, right, exchange, currency, localSymbol, marketName, tradingClass,
		//   conID, minTick, mdSizeMultiplier, orderTypes, validExchanges,
		//   priceMagnifier, underConID, longName, primaryExchange, contractMonth,
		//   industry, category, subcategory, timeZoneID, ...]
		reqID, _ := r.ReadInt()
		symbol := r.ReadString()
		secType := r.ReadString()
		expiry := r.ReadString()
		r.Skip(1) // lastTradeDateOrContractMonth (duplicate/variant of expiry)
		strike := r.ReadString()
		right := r.ReadString()
		exchange := r.ReadString()
		currency := r.ReadString()
		localSymbol := r.ReadString()
		marketName := r.ReadString()
		tradingClass := r.ReadString()
		conID, _ := r.ReadInt()
		minTick := r.ReadString()
		r.Skip(5) // mdSizeMultiplier, orderTypes, validExchanges, priceMagnifier, underConID
		longName := r.ReadString()
		primaryExchange := r.ReadString()
		r.Skip(4) // contractMonth, industry, category, subcategory
		timeZoneID := r.ReadString()
		return []Message{ContractDetails{
			ReqID: reqID,
			Contract: Contract{
				ConID: conID, Symbol: symbol, SecType: secType,
				Expiry: expiry, Strike: strike, Right: right,
				Exchange: exchange, Currency: currency,
				LocalSymbol: localSymbol, TradingClass: tradingClass,
				PrimaryExchange: primaryExchange,
			},
			MarketName: marketName, MinTick: minTick,
			LongName: longName, TimeZoneID: timeZoneID,
		}}, nil

	case InExecutionData: // [11, reqID, orderId, conID, symbol, secType, expiry, strike,
		//   right, multiplier, exchange, localSymbol, tradingClass,
		//   execID, time, account, exchange(exec), side, shares, price, ...]
		reqID, _ := r.ReadInt()
		r.Skip(2) // orderId, conID
		symbol := r.ReadString()
		r.Skip(8) // secType, expiry, strike, right, multiplier, exchange, localSymbol, tradingClass
		execID := r.ReadString()
		execTime := r.ReadString()
		account := r.ReadString()
		r.Skip(1) // execution exchange
		side := r.ReadString()
		shares := r.ReadString()
		price := r.ReadString()
		return []Message{ExecutionDetail{ReqID: reqID, ExecID: execID, Account: account, Symbol: symbol, Side: side, Shares: shares, Price: price, Time: execTime}}, nil

	case InManagedAccounts: // [15, version, accountsList]
		r.Skip(1)
		raw := r.ReadString()
		accounts := []string{}
		if raw != "" {
			accounts = strings.Split(strings.TrimRight(raw, ","), ",")
		}
		return []Message{ManagedAccounts{Accounts: accounts}}, nil

	case InHistoricalData: // [17, reqID, barCount, time, O, H, L, C, vol, wap, count, ...]
		reqID, _ := r.ReadInt()
		barCount, _ := r.ReadInt()
		if barCount <= 0 {
			return []Message{HistoricalBarsEnd{ReqID: reqID}}, nil
		}
		msgs := make([]Message, 0, barCount+1)
		for i := 0; i < barCount; i++ {
			msgs = append(msgs, HistoricalBar{
				ReqID: reqID, Time: r.ReadString(),
				Open: r.ReadString(), High: r.ReadString(),
				Low: r.ReadString(), Close: r.ReadString(),
				Volume: r.ReadString(), WAP: r.ReadString(), Count: r.ReadString(),
			})
		}
		msgs = append(msgs, HistoricalBarsEnd{ReqID: reqID})
		return msgs, nil

	case InTickGeneric, InTickString, InTickReqParams:
		return nil, nil

	case InCurrentTime: // [49, version, time]
		r.Skip(1)
		return []Message{CurrentTime{Time: r.ReadString()}}, nil

	case InRealTimeBars: // [50, version, reqID, time, O, H, L, C, vol, wap, count]
		r.Skip(1)
		reqID, _ := r.ReadInt()
		return []Message{RealTimeBar{
			ReqID: reqID, Time: r.ReadString(),
			Open: r.ReadString(), High: r.ReadString(), Low: r.ReadString(),
			Close: r.ReadString(), Volume: r.ReadString(),
			WAP: r.ReadString(), Count: r.ReadString(),
		}}, nil

	case InContractDataEnd: // [52, version, reqID]
		r.Skip(1)
		reqID, _ := r.ReadInt()
		return []Message{ContractDetailsEnd{ReqID: reqID}}, nil

	case InOpenOrderEnd:
		return []Message{OpenOrderEnd{}}, nil

	case InExecutionDataEnd: // [55, version, reqID]
		r.Skip(1)
		reqID, _ := r.ReadInt()
		return []Message{ExecutionsEnd{ReqID: reqID}}, nil

	case InTickSnapshotEnd: // [57, version, reqID]
		r.Skip(1)
		reqID, _ := r.ReadInt()
		return []Message{TickSnapshotEnd{ReqID: reqID}}, nil

	case InMarketDataType: // [58, version, reqID, dataType]
		r.Skip(1)
		reqID, _ := r.ReadInt()
		dataType, _ := r.ReadInt()
		return []Message{MarketDataType{ReqID: reqID, DataType: dataType}}, nil

	case InCommissionReport: // [59, version, execID, commission, currency, realizedPNL, ...]
		r.Skip(1)
		execID := r.ReadString()
		commission := r.ReadString()
		currency := r.ReadString()
		realizedPNL := r.ReadString()
		return []Message{CommissionReport{ExecID: execID, Commission: commission, Currency: currency, RealizedPNL: realizedPNL}}, nil

	case InPositionData: // [61, version, account, contract(11), position, avgCost]
		r.Skip(1)
		account := r.ReadString()
		contract := readWireContract(r)
		position := r.ReadString()
		avgCost := r.ReadString()
		return []Message{Position{Account: account, Contract: contract, Position: position, AvgCost: avgCost}}, nil

	case InPositionEnd:
		return []Message{PositionEnd{}}, nil

	case InAccountSummary: // [63, version, reqID, account, tag, value, currency]
		r.Skip(1)
		reqID, _ := r.ReadInt()
		account := r.ReadString()
		tag := r.ReadString()
		value := r.ReadString()
		currency := r.ReadString()
		return []Message{AccountSummaryValue{ReqID: reqID, Account: account, Tag: tag, Value: value, Currency: currency}}, nil

	case InAccountSummaryEnd: // [64, version, reqID]
		r.Skip(1)
		reqID, _ := r.ReadInt()
		return []Message{AccountSummaryEnd{ReqID: reqID}}, nil

	default:
		return nil, fmt.Errorf("codec: unknown msg_id %d", msgID)
	}
}

func encodeFields(msg Message) ([]string, error) {
	switch m := msg.(type) {

	case StartAPI:
		return []string{itoa(OutStartAPI), "2", itoa(m.ClientID), m.OptionalCapabilities}, nil

	case ContractDetailsRequest:
		w := fieldWriter{}
		w.WriteInt(OutReqContractData)
		w.WriteInt(8) // version
		w.WriteInt(m.ReqID)
		w.WriteInt(m.Contract.ConID)
		writeWireContract(&w, m.Contract)
		w.WriteBool(false) // includeExpired
		w.WriteString("")  // secIdType
		w.WriteString("")  // secId
		w.WriteString("")  // issuerId (v>=MinServerVersionBondIssuerId)
		return w.Fields(), nil

	case HistoricalBarsRequest:
		w := fieldWriter{}
		w.WriteInt(OutReqHistoricalData)
		w.WriteInt(m.ReqID)
		w.WriteInt(m.Contract.ConID)
		writeWireContract(&w, m.Contract)
		w.WriteBool(false)         // includeExpired
		w.WriteString(m.EndDateTime)
		w.WriteString(m.BarSize)
		w.WriteString(m.Duration)
		w.WriteBool(m.UseRTH)
		w.WriteString(m.WhatToShow)
		w.WriteInt(1)       // formatDate
		w.WriteBool(false)  // keepUpToDate
		w.WriteString("")   // chartOptions
		return w.Fields(), nil

	case AccountSummaryRequest:
		return []string{itoa(OutReqAccountSummary), "1", itoa(m.ReqID), m.Account, strings.Join(m.Tags, ",")}, nil

	case CancelAccountSummary:
		return []string{itoa(OutCancelAccountSummary), "1", itoa(m.ReqID)}, nil

	case PositionsRequest:
		return []string{itoa(OutReqPositions), "1"}, nil

	case CancelPositions:
		return []string{itoa(OutCancelPositions), "1"}, nil

	case QuoteRequest:
		w := fieldWriter{}
		w.WriteInt(OutReqMktData)
		w.WriteInt(11) // version
		w.WriteInt(m.ReqID)
		w.WriteInt(m.Contract.ConID)
		writeWireContract(&w, m.Contract)
		// BAG combo legs omitted (not supported in v1).
		w.WriteBool(false) // deltaNeutralContract present
		w.WriteString(strings.Join(m.GenericTicks, ","))
		w.WriteBool(m.Snapshot)
		w.WriteBool(false) // regulatorySnapshot
		w.WriteString("")  // mktDataOptions
		return w.Fields(), nil

	case CancelQuote:
		return []string{itoa(OutCancelMktData), "1", itoa(m.ReqID)}, nil

	case RealTimeBarsRequest:
		w := fieldWriter{}
		w.WriteInt(OutReqRealTimeBars)
		w.WriteInt(3) // version
		w.WriteInt(m.ReqID)
		w.WriteInt(m.Contract.ConID)
		writeWireContract(&w, m.Contract)
		w.WriteInt(5) // barSize (always 5 sec)
		w.WriteString(m.WhatToShow)
		w.WriteBool(m.UseRTH)
		w.WriteString("") // options
		return w.Fields(), nil

	case CancelRealTimeBars:
		return []string{itoa(OutCancelRealTimeBars), "1", itoa(m.ReqID)}, nil

	case OpenOrdersRequest:
		switch m.Scope {
		case "all":
			return []string{itoa(OutReqAllOpenOrders), "1"}, nil
		case "client":
			return []string{itoa(OutReqOpenOrders), "1"}, nil
		case "auto":
			return []string{itoa(OutReqAutoOpenOrders), "1", "1"}, nil
		default:
			return []string{itoa(OutReqAllOpenOrders), "1"}, nil
		}

	case CancelOpenOrders:
		return []string{itoa(OutReqAutoOpenOrders), "1", "0"}, nil

	case ExecutionsRequest:
		w := fieldWriter{}
		w.WriteInt(OutReqExecutions)
		w.WriteInt(3) // version
		w.WriteInt(m.ReqID)
		w.WriteInt(0) // clientId filter
		w.WriteString(m.Account)
		w.WriteString("") // time
		w.WriteString(m.Symbol)
		w.WriteString("") // secType
		w.WriteString("") // exchange
		w.WriteString("") // side
		return w.Fields(), nil

	// Server -> client (testhost)

	case ManagedAccounts:
		return []string{itoa(InManagedAccounts), "1", strings.Join(m.Accounts, ",")}, nil

	case NextValidID:
		return []string{itoa(InNextValidID), "1", i64toa(m.OrderID)}, nil

	case CurrentTime:
		return []string{itoa(InCurrentTime), "1", m.Time}, nil

	case APIError:
		return []string{itoa(InErrMsg), itoa(m.ReqID), itoa(m.Code), m.Message, m.AdvancedOrderRejectJSON, m.ErrorTimeMs}, nil

	case ContractDetails:
		return []string{
			itoa(InContractData), itoa(m.ReqID),
			m.Contract.Symbol, m.Contract.SecType, m.Contract.Expiry,
			m.Contract.Expiry, // lastTradeDateOrContractMonth (duplicate)
			m.Contract.Strike, m.Contract.Right,
			m.Contract.Exchange, m.Contract.Currency,
			m.Contract.LocalSymbol, m.MarketName, m.Contract.TradingClass,
			itoa(m.Contract.ConID), m.MinTick,
			"", "", "", "", "",
			m.LongName, m.Contract.PrimaryExchange,
			"", "", "", "",
			m.TimeZoneID,
		}, nil

	case ContractDetailsEnd:
		return []string{itoa(InContractDataEnd), "1", itoa(m.ReqID)}, nil

	case HistoricalBar:
		return []string{
			itoa(InHistoricalData), itoa(m.ReqID), "1",
			m.Time, m.Open, m.High, m.Low, m.Close, m.Volume, m.WAP, m.Count,
		}, nil

	case HistoricalBarsEnd:
		return []string{itoa(InHistoricalData), itoa(m.ReqID), "0"}, nil

	case AccountSummaryValue:
		return []string{itoa(InAccountSummary), "1", itoa(m.ReqID), m.Account, m.Tag, m.Value, m.Currency}, nil

	case AccountSummaryEnd:
		return []string{itoa(InAccountSummaryEnd), "1", itoa(m.ReqID)}, nil

	case Position:
		// Encode in server→client wire format matching readWireContract:
		// [conID, symbol, secType, expiry, strike, right, multiplier,
		//  exchange, currency, localSymbol, tradingClass]
		w := fieldWriter{}
		w.WriteInt(InPositionData)
		w.WriteInt(3) // version
		w.WriteString(m.Account)
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
		w.WriteString(m.Position)
		w.WriteString(m.AvgCost)
		return w.Fields(), nil

	case PositionEnd:
		return []string{itoa(InPositionEnd), "1"}, nil

	case TickPrice:
		return []string{itoa(InTickPrice), "6", itoa(m.ReqID), itoa(m.TickType), m.Price, m.Size, itoa(m.AttrMask)}, nil

	case TickSize:
		return []string{itoa(InTickSize), "6", itoa(m.ReqID), itoa(m.TickType), m.Size}, nil

	case MarketDataType:
		return []string{itoa(InMarketDataType), "1", itoa(m.ReqID), itoa(m.DataType)}, nil

	case TickSnapshotEnd:
		return []string{itoa(InTickSnapshotEnd), "1", itoa(m.ReqID)}, nil

	case RealTimeBar:
		return []string{itoa(InRealTimeBars), "3", itoa(m.ReqID), m.Time, m.Open, m.High, m.Low, m.Close, m.Volume, m.WAP, m.Count}, nil

	case OpenOrder:
		// Encode in the v200 wire layout. The decoder reads fields at fixed
		// positions verified against live captures, so the encoder must pad
		// intermediate fields to keep positions aligned.
		w := fieldWriter{}
		w.WriteInt(InOpenOrder)
		w.WriteInt64(m.OrderID)            // r[0]
		writeWireContract(&w, m.Contract)  // r[1..11]
		w.WriteString(m.Action)            // r[12]
		w.WriteString(m.Quantity)          // r[13]
		w.WriteString(m.OrderType)         // r[14]
		for range 4 {                      // r[15..18] lmtPrice, auxPrice, tif, ocaGroup
			w.WriteString("")
		}
		w.WriteString(m.Account)           // r[19]
		for range 71 {                     // r[20..90] order detail padding
			w.WriteString("")
		}
		w.WriteString(m.Status)            // r[91]
		for range 68 {                     // r[92..159] margin/state padding
			w.WriteString("")
		}
		w.WriteString(m.Filled)            // r[160]
		w.WriteString(m.Remaining)         // r[161]
		return w.Fields(), nil

	case OrderStatus:
		w := fieldWriter{}
		w.WriteInt(InOrderStatus)
		w.WriteInt64(m.OrderID)
		w.WriteString(m.Status)
		w.WriteString(m.Filled)
		w.WriteString(m.Remaining)
		return w.Fields(), nil

	case OpenOrderEnd:
		return []string{itoa(InOpenOrderEnd), "1"}, nil

	case ExecutionDetail:
		return []string{
			itoa(InExecutionData), itoa(m.ReqID),
			"0", "0",
			m.Symbol, "", "", "", "", "", "", "", "",
			m.ExecID, m.Time, m.Account,
			"",
			m.Side, m.Shares, m.Price,
		}, nil

	case ExecutionsEnd:
		return []string{itoa(InExecutionDataEnd), "1", itoa(m.ReqID)}, nil

	case CommissionReport:
		return []string{itoa(InCommissionReport), "1", m.ExecID, m.Commission, m.Currency, m.RealizedPNL}, nil

	default:
		return nil, fmt.Errorf("codec: unsupported message type %T", msg)
	}
}

// writeWireContract writes the 11-field contract block (client->server):
// [symbol, secType, expiry, strike, right, multiplier, exchange, primaryExchange, currency, localSymbol, tradingClass]
func writeWireContract(w *fieldWriter, c Contract) {
	w.WriteString(c.Symbol)
	w.WriteString(c.SecType)
	w.WriteString(c.Expiry)
	if c.Strike == "" {
		w.WriteString("0")
	} else {
		w.WriteString(c.Strike)
	}
	w.WriteString(c.Right)
	w.WriteString(c.Multiplier)
	w.WriteString(c.Exchange)
	w.WriteString(c.PrimaryExchange)
	w.WriteString(c.Currency)
	w.WriteString(c.LocalSymbol)
	w.WriteString(c.TradingClass)
}

// readWireContract reads the 11-field contract block (server->client):
// [conID, symbol, secType, expiry, strike, right, multiplier, exchange, currency, localSymbol, tradingClass]
func readWireContract(r *fieldReader) Contract {
	conID, _ := r.ReadInt()
	symbol := r.ReadString()
	secType := r.ReadString()
	expiry := r.ReadString()
	strike := r.ReadString()
	right := r.ReadString()
	multiplier := r.ReadString()
	exchange := r.ReadString()
	currency := r.ReadString()
	localSymbol := r.ReadString()
	tradingClass := r.ReadString()
	return Contract{
		ConID: conID, Symbol: symbol, SecType: secType,
		Expiry: expiry, Strike: strike, Right: right,
		Multiplier: multiplier, Exchange: exchange, Currency: currency,
		LocalSymbol: localSymbol, TradingClass: tradingClass,
	}
}

func itoa(v int) string    { return strconv.Itoa(v) }
func i64toa(v int64) string { return strconv.FormatInt(v, 10) }

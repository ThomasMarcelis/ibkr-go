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

	case InOrderStatus: // [3, orderId, status, filled, remaining, avgFillPrice, permId, parentId, lastFillPrice, clientId, whyHeld, mktCapPrice]
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

	case InErrMsg: // [4, reqId, code, message, advancedJson, errorTimeMs]
		reqID, _ := r.ReadInt()
		code, _ := r.ReadInt()
		message := r.ReadString()
		advJSON := r.ReadString()
		errTime := r.ReadString()
		return []Message{APIError{ReqID: reqID, Code: code, Message: message, AdvancedOrderRejectJSON: advJSON, ErrorTimeMs: errTime}}, nil

	case InOpenOrder:
		// v200 wire layout verified against live IB Gateway captures (server_version 200).
		// 170 total fields (including msg_id). After msg_id: 169 fields at r[0]-r[168].
		// The message has a fixed positional layout for simple orders (no combos,
		// no algo params). Conditional variable-length sections (comboLegs, algo
		// params, conditions) all have count=0 in the common case.
		//
		// Key anchor positions:
		//   r[0]      orderID
		//   r[1..11]  contract (11-field server→client block)
		//   r[12..18] action, totalQty, orderType, lmtPrice, auxPrice, tif, ocaGroup
		//   r[19]     account
		//   r[20..28] openClose, origin, orderRef, clientId, permId,
		//             outsideRth, hidden, discretionaryAmt, goodAfterTime
		//   r[29..90] 62 order detail fields (FA, model, rule80A, vol params, etc.)
		//   r[91]     OrderState.status
		//   r[92..100] initMarginBefore..equityWithLoanAfter (9 margin fields)
		//   r[101..105] commission..warningText (5 fields, may include empty prefix)
		//   r[106..159] remaining OrderState and post-status fields
		//   r[160..168] trailing order-status block (filled, remaining, + 7 more)
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
		r.Skip(62)                      // r[29..90]: FA params, order details, vol params, combos, scale, algo, etc.
		status := r.ReadString()        // r[91]
		// OrderState section: margin fields and commission.
		initMarginBefore := r.ReadString()     // r[92]
		maintMarginBefore := r.ReadString()    // r[93]
		equityWithLoanBefore := r.ReadString() // r[94]
		initMarginChange := r.ReadString()     // r[95]
		maintMarginChange := r.ReadString()    // r[96]
		equityWithLoanChange := r.ReadString() // r[97]
		initMarginAfter := r.ReadString()      // r[98]
		maintMarginAfter := r.ReadString()     // r[99]
		equityWithLoanAfter := r.ReadString()  // r[100]
		commission := r.ReadString()           // r[101]
		minCommission := r.ReadString()        // r[102]
		maxCommission := r.ReadString()        // r[103]
		commissionCurrency := r.ReadString()   // r[104]
		warningText := r.ReadString()          // r[105]
		r.Skip(54)                             // r[106..159]: remaining state/post-status fields
		filled := r.ReadString()               // r[160]
		remaining := r.ReadString()            // r[161]
		// r[162..168]: lastFillPrice, permId, parentId, lastLiquidity, whyHeld, mktCapPrice, trailing
		return []Message{OpenOrder{
			OrderID: orderID, Contract: contract,
			Action: action, Quantity: quantity, OrderType: orderType,
			LmtPrice: lmtPrice, AuxPrice: auxPrice, TIF: tif,
			OcaGroup: ocaGroup, Account: account,
			OpenClose: openClose, Origin: origin, OrderRef: orderRef,
			ClientID: clientID, PermID: permID, OutsideRTH: outsideRTH,
			Hidden: hidden, DiscretionAmt: discretionAmt, GoodAfterTime: goodAfterTime,
			Status:           status,
			InitMarginBefore: initMarginBefore, MaintMarginBefore: maintMarginBefore,
			EquityWithLoanBefore: equityWithLoanBefore,
			InitMarginChange:     initMarginChange, MaintMarginChange: maintMarginChange,
			EquityWithLoanChange: equityWithLoanChange,
			InitMarginAfter:      initMarginAfter, MaintMarginAfter: maintMarginAfter,
			EquityWithLoanAfter: equityWithLoanAfter,
			Commission:          commission, MinCommission: minCommission,
			MaxCommission: maxCommission, CommissionCurrency: commissionCurrency,
			WarningText: warningText,
			Filled:      filled, Remaining: remaining,
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
		orderID, _ := r.ReadInt64()
		r.Skip(1) // conID
		symbol := r.ReadString()
		r.Skip(8) // secType, expiry, strike, right, multiplier, exchange, localSymbol, tradingClass
		execID := r.ReadString()
		execTime := r.ReadString()
		account := r.ReadString()
		r.Skip(1) // execution exchange
		side := r.ReadString()
		shares := r.ReadString()
		price := r.ReadString()
		return []Message{ExecutionDetail{ReqID: reqID, OrderID: orderID, ExecID: execID, Account: account, Symbol: symbol, Side: side, Shares: shares, Price: price, Time: execTime}}, nil

	case InMarketDepth: // [12, version, reqID, position, operation, side, price, size]
		r.Skip(1) // version
		reqID, _ := r.ReadInt()
		position, _ := r.ReadInt()
		operation, _ := r.ReadInt()
		side, _ := r.ReadInt()
		price := r.ReadString()
		size := r.ReadString()
		return []Message{MarketDepthUpdate{ReqID: reqID, Position: position, Operation: operation, Side: side, Price: price, Size: size}}, nil

	case InMarketDepthL2: // [13, version, reqID, position, marketMaker, operation, side, price, size, isSmartDepth]
		r.Skip(1) // version
		reqID, _ := r.ReadInt()
		position, _ := r.ReadInt()
		marketMaker := r.ReadString()
		operation, _ := r.ReadInt()
		side, _ := r.ReadInt()
		price := r.ReadString()
		size := r.ReadString()
		isSmartDepth, _ := r.ReadBool()
		return []Message{MarketDepthL2Update{ReqID: reqID, Position: position, MarketMaker: marketMaker, Operation: operation, Side: side, Price: price, Size: size, IsSmartDepth: isSmartDepth}}, nil

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
		barCount, err := r.ReadCount("bar count")
		if err != nil {
			return nil, err
		}
		if barCount <= 0 {
			return []Message{HistoricalBarsEnd{ReqID: reqID}}, nil
		}
		if err := r.RequireFixedEntryFields("historical data", barCount, 8, 0); err != nil {
			return nil, err
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

	case InScannerParameters: // [19, version=1, xml]
		r.Skip(1) // version
		xml := r.ReadString()
		return []Message{ScannerParameters{XML: xml}}, nil

	case InScannerData: // [20, version=3, reqID, numberOfElements, entries(rank, contract(11), distance, benchmark, projection, legsStr)]
		r.Skip(1) // version
		reqID, _ := r.ReadInt()
		count, err := r.ReadCount("scanner entry count")
		if err != nil {
			return nil, err
		}
		if err := r.RequireFixedEntryFields("scanner data", count, 16, 0); err != nil {
			return nil, err
		}
		entries := make([]ScannerDataEntry, count)
		for i := range entries {
			rank, _ := r.ReadInt()
			contract := readWireContract(r)
			distance := r.ReadString()
			benchmark := r.ReadString()
			projection := r.ReadString()
			legsStr := r.ReadString()
			entries[i] = ScannerDataEntry{Rank: rank, Contract: contract, Distance: distance, Benchmark: benchmark, Projection: projection, LegsStr: legsStr}
		}
		return []Message{ScannerDataResponse{ReqID: reqID, Entries: entries}}, nil

	case InTickOptionComputation: // [21, version=6, reqID, tickType, tickAttrib, impliedVol, delta, optPrice, pvDividend, gamma, vega, theta, undPrice]
		r.Skip(1) // version
		reqID, _ := r.ReadInt()
		tickType, _ := r.ReadInt()
		tickAttrib, _ := r.ReadInt()
		impliedVol := r.ReadString()
		delta := r.ReadString()
		optPrice := r.ReadString()
		pvDividend := r.ReadString()
		gamma := r.ReadString()
		vega := r.ReadString()
		theta := r.ReadString()
		undPrice := r.ReadString()
		return []Message{TickOptionComputation{
			ReqID: reqID, TickType: tickType, TickAttrib: tickAttrib,
			ImpliedVol: impliedVol, Delta: delta, OptPrice: optPrice,
			PvDividend: pvDividend, Gamma: gamma, Vega: vega,
			Theta: theta, UndPrice: undPrice,
		}}, nil

	case InTickGeneric: // [45, version, reqID, tickType, value]
		r.Skip(1) // version
		reqID, _ := r.ReadInt()
		tickType, _ := r.ReadInt()
		value := r.ReadString()
		return []Message{TickGeneric{ReqID: reqID, TickType: tickType, Value: value}}, nil

	case InTickString: // [46, version, reqID, tickType, value]
		r.Skip(1) // version
		reqID, _ := r.ReadInt()
		tickType, _ := r.ReadInt()
		value := r.ReadString()
		return []Message{TickString{ReqID: reqID, TickType: tickType, Value: value}}, nil

	case InTickReqParams: // [81, reqID, minTick, bboExchange, snapshotPermissions] — no version
		reqID, _ := r.ReadInt()
		minTick := r.ReadString()
		bboExchange := r.ReadString()
		snapshotPermissions, _ := r.ReadInt()
		return []Message{TickReqParams{ReqID: reqID, MinTick: minTick, BBOExchange: bboExchange, SnapshotPermissions: snapshotPermissions}}, nil

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

	case InFundamentalData: // [51, version, reqID, data]
		r.Skip(1) // version
		reqID, _ := r.ReadInt()
		data := r.ReadString()
		return []Message{FundamentalDataResponse{ReqID: reqID, Data: data}}, nil

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

	case InSecDefOptParams: // [75, reqID, exchange, underlyingConID, tradingClass, multiplier, marketRuleId, expirationsCount, expirations..., strikesCount, strikes...] — no version
		reqID, _ := r.ReadInt()
		exchange := r.ReadString()
		underConID, _ := r.ReadInt()
		tradingClass := r.ReadString()
		multiplier := r.ReadString()
		r.Skip(1) // marketRuleId
		expirationCount, err := r.ReadCount("expiration count")
		if err != nil {
			return nil, err
		}
		if expirationCount > r.Remaining() {
			return nil, fmt.Errorf("codec: sec def opt params: expiration count %d exceeds remaining fields %d", expirationCount, r.Remaining())
		}
		expirations := make([]string, expirationCount)
		for i := range expirations {
			expirations[i] = r.ReadString()
		}
		strikeCount, err := r.ReadCount("strike count")
		if err != nil {
			return nil, err
		}
		if strikeCount > r.Remaining() {
			return nil, fmt.Errorf("codec: sec def opt params: strike count %d exceeds remaining fields %d", strikeCount, r.Remaining())
		}
		strikes := make([]string, strikeCount)
		for i := range strikes {
			strikes[i] = r.ReadString()
		}
		return []Message{SecDefOptParamsResponse{
			ReqID: reqID, Exchange: exchange, UnderlyingConID: underConID,
			TradingClass: tradingClass, Multiplier: multiplier,
			Expirations: expirations, Strikes: strikes,
		}}, nil

	case InSecDefOptParamsEnd: // [76, reqID] — no version
		reqID, _ := r.ReadInt()
		return []Message{SecDefOptParamsEnd{ReqID: reqID}}, nil

	case InFamilyCodes: // [78, count, repeated(accountID, familyCode)] — no version
		count, err := r.ReadCount("family code count")
		if err != nil {
			return nil, err
		}
		if err := r.RequireFixedEntryFields("family codes", count, 2, 0); err != nil {
			return nil, err
		}
		entries := make([]FamilyCodeEntry, count)
		for i := range entries {
			entries[i] = FamilyCodeEntry{AccountID: r.ReadString(), FamilyCode: r.ReadString()}
		}
		return []Message{FamilyCodes{Codes: entries}}, nil

	case InMktDepthExchanges: // msg_id 80 is shared: MktDepthExchanges or HistoricalNewsEnd
		// Disambiguate: HistoricalNewsEnd has exactly 2 fields after msg_id [reqID, hasMore].
		// MktDepthExchanges has [count, repeated(5 fields)] = 1 + 5*count fields.
		if r.Remaining() == 2 {
			reqID, _ := r.ReadInt()
			hasMore, _ := r.ReadBool()
			return []Message{HistoricalNewsEnd{ReqID: reqID, HasMore: hasMore}}, nil
		}
		// MktDepthExchanges: [80, count, repeated(exchange, secType, listingExch, serviceDataType, aggGroup)] — no version
		count, err := r.ReadCount("depth exchange count")
		if err != nil {
			return nil, err
		}
		if err := r.RequireFixedEntryFields("market depth exchanges", count, 5, 0); err != nil {
			return nil, err
		}
		entries := make([]DepthExchangeEntry, count)
		for i := range entries {
			entries[i] = DepthExchangeEntry{
				Exchange: r.ReadString(), SecType: r.ReadString(),
				ListingExch: r.ReadString(), ServiceDataType: r.ReadString(),
			}
			entries[i].AggGroup, _ = r.ReadInt()
		}
		return []Message{MktDepthExchanges{Exchanges: entries}}, nil

	case InNewsArticle: // [83, reqID, articleType, articleText] — no version
		reqID, _ := r.ReadInt()
		articleType, _ := r.ReadInt()
		articleText := r.ReadString()
		return []Message{NewsArticleResponse{ReqID: reqID, ArticleType: articleType, ArticleText: articleText}}, nil

	case InNewsProviders: // [85, count, repeated(code, name)] — no version
		count, err := r.ReadCount("news provider count")
		if err != nil {
			return nil, err
		}
		if err := r.RequireFixedEntryFields("news providers", count, 2, 0); err != nil {
			return nil, err
		}
		entries := make([]NewsProviderEntry, count)
		for i := range entries {
			entries[i] = NewsProviderEntry{Code: r.ReadString(), Name: r.ReadString()}
		}
		return []Message{NewsProviders{Providers: entries}}, nil

	case InSymbolSamples: // msg_id 82 is shared: SymbolSamples or SmartComponents
		// SmartComponents entries are exactly 3 fields each (bitNumber, exchangeName, exchangeLetter).
		// SymbolSamples entries are 6+ fields each (conID, symbol, secType, primaryExch, currency, derivCount, derivTypes...).
		// Disambiguate by checking if remaining fields after [reqID, count] == count * 3.
		reqID, _ := r.ReadInt()
		count, err := r.ReadCount("sample count")
		if err != nil {
			return nil, err
		}
		if r.Remaining() == count*3 {
			if err := r.RequireFixedEntryFields("smart components", count, 3, 0); err != nil {
				return nil, err
			}
			components := make([]SmartComponentEntry, count)
			for i := range components {
				bitNumber, _ := r.ReadInt()
				exchangeName := r.ReadString()
				exchangeLetter := r.ReadString()
				components[i] = SmartComponentEntry{BitNumber: bitNumber, ExchangeName: exchangeName, ExchangeLetter: exchangeLetter}
			}
			return []Message{SmartComponentsResponse{ReqID: reqID, Components: components}}, nil
		}
		if count > r.Remaining()/6 {
			return nil, fmt.Errorf("codec: symbol samples: count %d exceeds minimum available fields %d", count, r.Remaining())
		}
		symbols := make([]SymbolSample, count)
		for i := range symbols {
			conID, _ := r.ReadInt()
			symbol := r.ReadString()
			secType := r.ReadString()
			primaryExch := r.ReadString()
			currency := r.ReadString()
			derivCount, _ := r.ReadInt()
			derivTypes := make([]string, derivCount)
			for j := range derivTypes {
				derivTypes[j] = r.ReadString()
			}
			symbols[i] = SymbolSample{
				ConID: conID, Symbol: symbol, SecType: secType,
				PrimaryExchange: primaryExch, Currency: currency,
				DerivativeSecTypes: derivTypes,
			}
		}
		return []Message{MatchingSymbols{ReqID: reqID, Symbols: symbols}}, nil

	case InHistoricalNews: // [87, reqID, time, providerCode, articleId, headline] — no version
		reqID, _ := r.ReadInt()
		timeStr := r.ReadString()
		providerCode := r.ReadString()
		articleID := r.ReadString()
		headline := r.ReadString()
		return []Message{HistoricalNewsItem{ReqID: reqID, Time: timeStr, ProviderCode: providerCode, ArticleID: articleID, Headline: headline}}, nil

	case InHeadTimestamp: // [88, reqId, headTimestamp] — no version
		reqID, _ := r.ReadInt()
		timestamp := r.ReadString()
		return []Message{HeadTimestamp{ReqID: reqID, Timestamp: timestamp}}, nil

	case InHistogramData: // [89, reqID, count, entries(price, size)] — no version
		reqID, _ := r.ReadInt()
		count, err := r.ReadCount("histogram entry count")
		if err != nil {
			return nil, err
		}
		if err := r.RequireFixedEntryFields("histogram data", count, 2, 0); err != nil {
			return nil, err
		}
		entries := make([]HistogramDataEntry, count)
		for i := range entries {
			entries[i] = HistogramDataEntry{Price: r.ReadString(), Size: r.ReadString()}
		}
		return []Message{HistogramDataResponse{ReqID: reqID, Entries: entries}}, nil

	case InMarketRule: // [92, marketRuleId, count, repeated(lowEdge, increment)] — no version
		marketRuleID, _ := r.ReadInt()
		count, err := r.ReadCount("market rule increment count")
		if err != nil {
			return nil, err
		}
		if err := r.RequireFixedEntryFields("market rule", count, 2, 0); err != nil {
			return nil, err
		}
		increments := make([]PriceIncrement, count)
		for i := range increments {
			increments[i] = PriceIncrement{LowEdge: r.ReadString(), Increment: r.ReadString()}
		}
		return []Message{MarketRule{MarketRuleID: marketRuleID, Increments: increments}}, nil

	case InCompletedOrder: // [101, contract(11-field), action, totalQty, orderType, ...]
		// Simplified decoder: reads just the key fields from the v200 wire layout.
		// The full CompletedOrder message has ~80 fields; we extract the
		// contract block, action, quantity, order type, status, filled, remaining.
		contract := readWireContract(r)
		action := r.ReadString()
		quantity := r.ReadString()
		orderType := r.ReadString()
		r.Skip(4)  // lmtPrice, auxPrice, tif, ocaGroup
		r.Skip(71) // order detail fields to OrderState
		status := r.ReadString()
		r.Skip(3) // completedTime, completedStatus, and misc
		filled := r.ReadString()
		remaining := r.ReadString()
		return []Message{CompletedOrder{
			Contract: contract, Action: action, OrderType: orderType,
			Status: status, Quantity: quantity, Filled: filled, Remaining: remaining,
		}}, nil

	case InCompletedOrderEnd: // [102]
		return []Message{CompletedOrderEnd{}}, nil

	case InUserInfo: // [103, reqId, whiteBrandingId] — no version
		reqID, _ := r.ReadInt()
		whiteBrandingID := r.ReadString()
		return []Message{UserInfo{ReqID: reqID, WhiteBrandingID: whiteBrandingID}}, nil

	case InUpdateAccountValue: // [6, version=2, key, value, currency, accountName]
		r.Skip(1) // version
		key := r.ReadString()
		value := r.ReadString()
		currency := r.ReadString()
		account := r.ReadString()
		return []Message{UpdateAccountValue{Key: key, Value: value, Currency: currency, Account: account}}, nil

	case InUpdatePortfolio: // [7, version=8, conID, symbol, secType, expiry, strike, right, multiplier, primaryExchange, currency, localSymbol, tradingClass, position, marketPrice, marketValue, avgCost, unrealizedPNL, realizedPNL, accountName]
		r.Skip(1) // version
		conID, _ := r.ReadInt()
		symbol := r.ReadString()
		secType := r.ReadString()
		expiry := r.ReadString()
		strike := r.ReadString()
		right := r.ReadString()
		multiplier := r.ReadString()
		primaryExchange := r.ReadString()
		currency := r.ReadString()
		localSymbol := r.ReadString()
		tradingClass := r.ReadString()
		position := r.ReadString()
		marketPrice := r.ReadString()
		marketValue := r.ReadString()
		avgCost := r.ReadString()
		unrealizedPNL := r.ReadString()
		realizedPNL := r.ReadString()
		account := r.ReadString()
		return []Message{UpdatePortfolio{
			Contract: Contract{
				ConID: conID, Symbol: symbol, SecType: secType,
				Expiry: expiry, Strike: strike, Right: right,
				Multiplier: multiplier, PrimaryExchange: primaryExchange,
				Currency: currency, LocalSymbol: localSymbol, TradingClass: tradingClass,
			},
			Position: position, MarketPrice: marketPrice, MarketValue: marketValue,
			AvgCost: avgCost, UnrealizedPNL: unrealizedPNL, RealizedPNL: realizedPNL,
			Account: account,
		}}, nil

	case InUpdateAccountTime: // [8, version=1, timestamp]
		r.Skip(1) // version
		timestamp := r.ReadString()
		return []Message{UpdateAccountTime{Timestamp: timestamp}}, nil

	case InAccountDownloadEnd: // [54, version=1, accountName]
		r.Skip(1) // version
		account := r.ReadString()
		return []Message{AccountDownloadEnd{Account: account}}, nil

	case InNewsBulletins: // [14, version=1, msgId, msgType, headline, source]
		r.Skip(1) // version
		msgId, _ := r.ReadInt()
		msgType, _ := r.ReadInt()
		headline := r.ReadString()
		source := r.ReadString()
		return []Message{NewsBulletin{MsgID: msgId, MsgType: msgType, Headline: headline, Source: source}}, nil

	case InPositionMulti: // [71, version=1, reqID, account, modelCode, contract(11), position, avgCost]
		r.Skip(1) // version
		reqID, _ := r.ReadInt()
		account := r.ReadString()
		modelCode := r.ReadString()
		contract := readWireContract(r)
		position := r.ReadString()
		avgCost := r.ReadString()
		return []Message{PositionMulti{ReqID: reqID, Account: account, ModelCode: modelCode, Contract: contract, Position: position, AvgCost: avgCost}}, nil

	case InPositionMultiEnd: // [72, version=1, reqID]
		r.Skip(1) // version
		reqID, _ := r.ReadInt()
		return []Message{PositionMultiEnd{ReqID: reqID}}, nil

	case InAccountUpdateMulti: // [73, version=1, reqID, account, modelCode, key, value, currency]
		r.Skip(1) // version
		reqID, _ := r.ReadInt()
		account := r.ReadString()
		modelCode := r.ReadString()
		key := r.ReadString()
		value := r.ReadString()
		currency := r.ReadString()
		return []Message{AccountUpdateMultiValue{ReqID: reqID, Account: account, ModelCode: modelCode, Key: key, Value: value, Currency: currency}}, nil

	case InAccountUpdateMultiEnd: // [74, version=1, reqID]
		r.Skip(1) // version
		reqID, _ := r.ReadInt()
		return []Message{AccountUpdateMultiEnd{ReqID: reqID}}, nil

	case InPnL: // [94, reqID, dailyPnL, unrealizedPnL, realizedPnL] — no version
		reqID, _ := r.ReadInt()
		dailyPnL := r.ReadString()
		unrealizedPnL := r.ReadString()
		realizedPnL := r.ReadString()
		return []Message{PnLValue{ReqID: reqID, DailyPnL: dailyPnL, UnrealizedPnL: unrealizedPnL, RealizedPnL: realizedPnL}}, nil

	case InPnLSingle: // [95, reqID, pos, dailyPnL, unrealizedPnL, realizedPnL, value] — no version
		reqID, _ := r.ReadInt()
		position := r.ReadString()
		dailyPnL := r.ReadString()
		unrealizedPnL := r.ReadString()
		realizedPnL := r.ReadString()
		value := r.ReadString()
		return []Message{PnLSingleValue{ReqID: reqID, Position: position, DailyPnL: dailyPnL, UnrealizedPnL: unrealizedPnL, RealizedPnL: realizedPnL, Value: value}}, nil

	case InHistoricalTicks: // [96, reqID, count, entries(time, unused, price, size), done] — MIDPOINT
		reqID, _ := r.ReadInt()
		count, err := r.ReadCount("historical midpoint tick count")
		if err != nil {
			return nil, err
		}
		if err := r.RequireFixedEntryFields("historical midpoint ticks", count, 4, 1); err != nil {
			return nil, err
		}
		ticks := make([]HistoricalTickEntry, count)
		for i := range ticks {
			timeStr := r.ReadString()
			r.Skip(1) // unused
			price := r.ReadString()
			size := r.ReadString()
			ticks[i] = HistoricalTickEntry{Time: timeStr, Price: price, Size: size}
		}
		done, _ := r.ReadBool()
		return []Message{HistoricalTicksResponse{ReqID: reqID, Ticks: ticks, Done: done}}, nil

	case InHistoricalTicksBidAsk: // [97, reqID, count, entries(time, attrib, bidPrice, askPrice, bidSize, askSize), done] — BID_ASK
		reqID, _ := r.ReadInt()
		count, err := r.ReadCount("historical bid/ask tick count")
		if err != nil {
			return nil, err
		}
		if err := r.RequireFixedEntryFields("historical bid/ask ticks", count, 6, 1); err != nil {
			return nil, err
		}
		ticks := make([]HistoricalTickBidAskEntry, count)
		for i := range ticks {
			timeStr := r.ReadString()
			r.Skip(1) // tickAttribBidAsk
			bidPrice := r.ReadString()
			askPrice := r.ReadString()
			bidSize := r.ReadString()
			askSize := r.ReadString()
			ticks[i] = HistoricalTickBidAskEntry{Time: timeStr, BidPrice: bidPrice, AskPrice: askPrice, BidSize: bidSize, AskSize: askSize}
		}
		done, _ := r.ReadBool()
		return []Message{HistoricalTicksBidAskResponse{ReqID: reqID, Ticks: ticks, Done: done}}, nil

	case InHistoricalTicksLast: // [98, reqID, count, entries(time, attrib, price, size, exchange, specialConditions), done] — TRADES
		reqID, _ := r.ReadInt()
		count, err := r.ReadCount("historical trade tick count")
		if err != nil {
			return nil, err
		}
		if err := r.RequireFixedEntryFields("historical trade ticks", count, 6, 1); err != nil {
			return nil, err
		}
		ticks := make([]HistoricalTickLastEntry, count)
		for i := range ticks {
			timeStr := r.ReadString()
			r.Skip(1) // tickAttribLast
			price := r.ReadString()
			size := r.ReadString()
			exchange := r.ReadString()
			specialConditions := r.ReadString()
			ticks[i] = HistoricalTickLastEntry{Time: timeStr, Price: price, Size: size, Exchange: exchange, SpecialConditions: specialConditions}
		}
		done, _ := r.ReadBool()
		return []Message{HistoricalTicksLastResponse{ReqID: reqID, Ticks: ticks, Done: done}}, nil

	case InTickByTick: // [99, reqID, tickType, time, ...]
		reqID, _ := r.ReadInt()
		tickType, _ := r.ReadInt()
		timeStr := r.ReadString()
		tick := TickByTickData{ReqID: reqID, TickType: tickType, Time: timeStr}
		switch tickType {
		case 1, 2: // Last, AllLast
			tick.Price = r.ReadString()
			tick.Size = r.ReadString()
			tick.TickAttribLast, _ = r.ReadInt()
			tick.Exchange = r.ReadString()
			tick.SpecialConditions = r.ReadString()
		case 3: // BidAsk
			tick.BidPrice = r.ReadString()
			tick.AskPrice = r.ReadString()
			tick.BidSize = r.ReadString()
			tick.AskSize = r.ReadString()
			tick.TickAttribBidAsk, _ = r.ReadInt()
		case 4: // MidPoint
			tick.MidPoint = r.ReadString()
		}
		return []Message{tick}, nil

	case InHistoricalDataUpdate: // [108, reqID, barCount, time, O, H, L, C, vol, wap, count]
		reqID, _ := r.ReadInt()
		barCount, _ := r.ReadInt()
		return []Message{HistoricalDataUpdate{
			ReqID: reqID, BarCount: barCount,
			Time: r.ReadString(), Open: r.ReadString(), High: r.ReadString(),
			Low: r.ReadString(), Close: r.ReadString(), Volume: r.ReadString(),
			WAP: r.ReadString(), Count: r.ReadString(),
		}}, nil

	case InReceiveFA: // [16, version, faDataType, xml]
		r.Skip(1) // version
		faDataType, _ := r.ReadInt()
		xml := r.ReadString()
		return []Message{ReceiveFA{FADataType: faDataType, XML: xml}}, nil

	case InSoftDollarTiers: // [77, reqId, count, (name, value, displayName) * count]
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

	case InWSHMetaData: // [105, reqId, dataJson]
		reqID, _ := r.ReadInt()
		dataJSON := r.ReadString()
		return []Message{WSHMetaDataResponse{ReqID: reqID, DataJSON: dataJSON}}, nil

	case InWSHEventData: // [106, reqId, dataJson]
		reqID, _ := r.ReadInt()
		dataJSON := r.ReadString()
		return []Message{WSHEventDataResponse{ReqID: reqID, DataJSON: dataJSON}}, nil

	case InDisplayGroupList: // [67, version, reqId, groups]
		r.Skip(1) // version
		reqID, _ := r.ReadInt()
		groups := r.ReadString()
		return []Message{DisplayGroupList{ReqID: reqID, Groups: groups}}, nil

	case InDisplayGroupUpdated: // [68, version, reqId, contractInfo]
		r.Skip(1) // version
		reqID, _ := r.ReadInt()
		contractInfo := r.ReadString()
		return []Message{DisplayGroupUpdated{ReqID: reqID, ContractInfo: contractInfo}}, nil

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
		w.WriteBool(false) // includeExpired
		w.WriteString(m.EndDateTime)
		w.WriteString(m.BarSize)
		w.WriteString(m.Duration)
		w.WriteBool(m.UseRTH)
		w.WriteString(m.WhatToShow)
		w.WriteInt(1) // formatDate
		w.WriteBool(m.KeepUpToDate)
		w.WriteString("") // chartOptions
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

	case ReqMarketDataType:
		return []string{itoa(OutReqMarketDataType), "1", itoa(m.DataType)}, nil

	case CancelHistoricalData:
		return []string{itoa(OutCancelHistoricalData), "1", itoa(m.ReqID)}, nil

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

	case MarketDepthRequest:
		w := fieldWriter{}
		w.WriteInt(OutReqMktDepth)
		w.WriteInt(5) // version
		w.WriteInt(m.ReqID)
		w.WriteInt(m.Contract.ConID)
		writeWireContract(&w, m.Contract)
		w.WriteInt(m.NumRows)
		w.WriteBool(m.IsSmartDepth)
		w.WriteString("") // mktDepthOptions
		return w.Fields(), nil

	case CancelMarketDepth:
		return []string{itoa(OutCancelMktDepth), "1", itoa(m.ReqID)}, nil

	case FundamentalDataRequest:
		w := fieldWriter{}
		w.WriteInt(OutReqFundamentalData)
		w.WriteInt(2) // version
		w.WriteInt(m.ReqID)
		w.WriteInt(m.Contract.ConID)
		w.WriteString(m.Contract.Symbol)
		w.WriteString(m.Contract.SecType)
		w.WriteString(m.Contract.Exchange)
		w.WriteString(m.Contract.PrimaryExchange)
		w.WriteString(m.Contract.Currency)
		w.WriteString(m.Contract.LocalSymbol)
		w.WriteString(m.ReportType)
		return w.Fields(), nil

	case CancelFundamentalData:
		return []string{itoa(OutCancelFundamentalData), "1", itoa(m.ReqID)}, nil

	case ExerciseOptionsRequest:
		w := fieldWriter{}
		w.WriteInt(OutExerciseOptions)
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
		return w.Fields(), nil

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

	case FamilyCodesRequest:
		return []string{itoa(OutReqFamilyCodes)}, nil

	case MktDepthExchangesRequest:
		return []string{itoa(OutReqMktDepthExchanges)}, nil

	case NewsProvidersRequest:
		return []string{itoa(OutReqNewsProviders)}, nil

	case ScannerParametersRequest:
		return []string{itoa(OutReqScannerParameters), "1"}, nil

	case UserInfoRequest:
		return []string{itoa(OutReqUserInfo), "1", itoa(m.ReqID)}, nil

	case MatchingSymbolsRequest:
		return []string{itoa(OutReqMatchingSymbols), itoa(m.ReqID), m.Pattern}, nil

	case HeadTimestampRequest:
		w := fieldWriter{}
		w.WriteInt(OutReqHeadTimestamp)
		w.WriteInt(m.ReqID)
		w.WriteInt(m.Contract.ConID)
		writeWireContract(&w, m.Contract)
		w.WriteBool(false) // includeExpired
		w.WriteBool(m.UseRTH)
		w.WriteString(m.WhatToShow)
		w.WriteInt(1) // formatDate
		return w.Fields(), nil

	case CancelHeadTimestamp:
		return []string{itoa(OutCancelHeadTimestamp), itoa(m.ReqID)}, nil

	case MarketRuleRequest:
		return []string{itoa(OutReqMarketRule), itoa(m.MarketRuleID)}, nil

	case CompletedOrdersRequest:
		w := fieldWriter{}
		w.WriteInt(OutReqCompletedOrders)
		w.WriteBool(m.APIOnly)
		return w.Fields(), nil

	case AccountUpdatesRequest:
		return []string{itoa(OutReqAccountUpdates), "2", btoa(m.Subscribe), m.Account}, nil

	case AccountUpdatesMultiRequest:
		return []string{itoa(OutReqAccountUpdatesMulti), "1", itoa(m.ReqID), m.Account, m.ModelCode, "1"}, nil

	case CancelAccountUpdatesMulti:
		return []string{itoa(OutCancelAccountUpdatesMulti), "1", itoa(m.ReqID)}, nil

	case PositionsMultiRequest:
		return []string{itoa(OutReqPositionsMulti), "1", itoa(m.ReqID), m.Account, m.ModelCode}, nil

	case CancelPositionsMulti:
		return []string{itoa(OutCancelPositionsMulti), "1", itoa(m.ReqID)}, nil

	case PnLRequest:
		return []string{itoa(OutReqPnL), itoa(m.ReqID), m.Account, m.ModelCode}, nil

	case CancelPnL:
		return []string{itoa(OutCancelPnL), itoa(m.ReqID)}, nil

	case PnLSingleRequest:
		return []string{itoa(OutReqPnLSingle), itoa(m.ReqID), m.Account, m.ModelCode, itoa(m.ConID)}, nil

	case CancelPnLSingle:
		return []string{itoa(OutCancelPnLSingle), itoa(m.ReqID)}, nil

	case SecDefOptParamsRequest:
		return []string{itoa(OutReqSecDefOptParams), itoa(m.ReqID), m.UnderlyingSymbol, m.FutFopExchange, m.UnderlyingSecType, itoa(m.UnderlyingConID)}, nil

	case SmartComponentsRequest:
		return []string{itoa(OutReqSmartComponents), itoa(m.ReqID), m.BBOExchange}, nil

	case CalcImpliedVolatilityRequest:
		w := fieldWriter{}
		w.WriteInt(OutReqCalcImpliedVolatility)
		w.WriteInt(3) // version
		w.WriteInt(m.ReqID)
		w.WriteInt(m.Contract.ConID)
		writeWireContract(&w, m.Contract)
		w.WriteBool(false) // includeExpired
		w.WriteString(m.OptionPrice)
		w.WriteString(m.UnderPrice)
		w.WriteString("") // implVolOptions
		return w.Fields(), nil

	case CancelCalcImpliedVolatility:
		return []string{itoa(OutCancelCalcImpliedVolatility), "1", itoa(m.ReqID)}, nil

	case CalcOptionPriceRequest:
		w := fieldWriter{}
		w.WriteInt(OutReqCalcOptionPrice)
		w.WriteInt(3) // version
		w.WriteInt(m.ReqID)
		w.WriteInt(m.Contract.ConID)
		writeWireContract(&w, m.Contract)
		w.WriteBool(false) // includeExpired
		w.WriteString(m.Volatility)
		w.WriteString(m.UnderPrice)
		w.WriteString("") // optPxOptions
		return w.Fields(), nil

	case CancelCalcOptionPrice:
		return []string{itoa(OutCancelCalcOptionPrice), "1", itoa(m.ReqID)}, nil

	case HistogramDataRequest:
		w := fieldWriter{}
		w.WriteInt(OutReqHistogramData)
		w.WriteInt(m.ReqID)
		w.WriteInt(m.Contract.ConID)
		writeWireContract(&w, m.Contract)
		w.WriteBool(false) // includeExpired
		w.WriteBool(m.UseRTH)
		w.WriteString(m.Period)
		return w.Fields(), nil

	case CancelHistogramData:
		return []string{itoa(OutCancelHistogramData), itoa(m.ReqID)}, nil

	case HistoricalTicksRequest:
		w := fieldWriter{}
		w.WriteInt(OutReqHistoricalTicks)
		w.WriteInt(m.ReqID)
		w.WriteInt(m.Contract.ConID)
		writeWireContract(&w, m.Contract)
		w.WriteBool(false) // includeExpired
		w.WriteString(m.StartDateTime)
		w.WriteString(m.EndDateTime)
		w.WriteInt(m.NumberOfTicks)
		w.WriteString(m.WhatToShow)
		w.WriteBool(m.UseRTH)
		w.WriteBool(m.IgnoreSize)
		w.WriteString("") // miscOptions
		return w.Fields(), nil

	case NewsArticleRequest:
		return []string{itoa(OutReqNewsArticle), itoa(m.ReqID), m.ProviderCode, m.ArticleID, ""}, nil

	case HistoricalNewsRequest:
		return []string{itoa(OutReqHistoricalNews), itoa(m.ReqID), itoa(m.ConID), m.ProviderCodes, m.StartDate, m.EndDate, itoa(m.TotalResults), ""}, nil

	case ScannerSubscriptionRequest:
		w := fieldWriter{}
		w.WriteInt(OutReqScannerSubscription)
		w.WriteInt(m.ReqID)
		w.WriteMaxInt(m.NumberOfRows)
		w.WriteString(m.Instrument)
		w.WriteString(m.LocationCode)
		w.WriteString(m.ScanCode)
		for range 14 { // abovePrice, belowPrice, aboveVolume, marketCapAbove/Below, moody/sp ratings, maturityDates, couponRates, excludeConvertible, averageOptionVolumeAbove
			w.WriteString("")
		}
		w.WriteString("") // scannerSettingPairs
		w.WriteString("") // stockTypeFilter
		w.WriteString("") // scannerSubscriptionFilterOptions
		w.WriteString("") // scannerSubscriptionOptions
		return w.Fields(), nil

	case CancelScannerSubscription:
		return []string{itoa(OutCancelScannerSubscription), "1", itoa(m.ReqID)}, nil

	case TickByTickRequest:
		w := fieldWriter{}
		w.WriteInt(OutReqTickByTickData)
		w.WriteInt(m.ReqID)
		w.WriteInt(m.Contract.ConID)
		writeWireContract(&w, m.Contract)
		w.WriteString(m.TickType)
		w.WriteInt(m.NumberOfTicks)
		w.WriteBool(m.IgnoreSize)
		return w.Fields(), nil

	case CancelTickByTick:
		return []string{itoa(OutCancelTickByTickData), itoa(m.ReqID)}, nil

	case NewsBulletinsRequest:
		return []string{itoa(OutReqNewsBulletins), "1", btoa(m.AllMessages)}, nil

	case CancelNewsBulletins:
		return []string{itoa(OutCancelNewsBulletins), "1"}, nil

	case RequestFA:
		return []string{itoa(OutRequestFA), "1", itoa(m.FADataType)}, nil

	case ReplaceFA:
		return []string{itoa(OutReplaceFA), "1", itoa(m.FADataType), strings.ReplaceAll(m.XML, "\n", "")}, nil

	case SoftDollarTiersRequest:
		return []string{itoa(OutReqSoftDollarTiers), itoa(m.ReqID)}, nil

	case WSHMetaDataRequest:
		return []string{itoa(OutReqWSHMetaData), itoa(m.ReqID)}, nil

	case CancelWSHMetaData:
		return []string{itoa(OutCancelWSHMetaData), itoa(m.ReqID)}, nil

	case WSHEventDataRequest:
		w := fieldWriter{}
		w.WriteInt(OutReqWSHEventData)
		w.WriteInt(m.ReqID)
		w.WriteInt(m.ConID)
		w.WriteString(m.Filter)
		w.WriteBool(m.FillWatchlist)
		w.WriteBool(m.FillPortfolio)
		w.WriteBool(m.FillCompetitors)
		w.WriteString(m.StartDate)
		w.WriteString(m.EndDate)
		w.WriteInt(m.TotalLimit)
		return w.Fields(), nil

	case CancelWSHEventData:
		return []string{itoa(OutCancelWSHEventData), itoa(m.ReqID)}, nil

	case QueryDisplayGroupsRequest:
		return []string{itoa(OutQueryDisplayGroups), "1", itoa(m.ReqID)}, nil

	case SubscribeToGroupEventsRequest:
		return []string{itoa(OutSubscribeToGroupEvents), "1", itoa(m.ReqID), itoa(m.GroupID)}, nil

	case UpdateDisplayGroupRequest:
		return []string{itoa(OutUpdateDisplayGroup), "1", itoa(m.ReqID), m.ContractInfo}, nil

	case UnsubscribeFromGroupEventsRequest:
		return []string{itoa(OutUnsubscribeFromGroupEvents), "1", itoa(m.ReqID)}, nil

	case PlaceOrderRequest:
		w := fieldWriter{}
		w.WriteInt(OutPlaceOrder)
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
		w.WriteString("") // secIdType
		w.WriteString("") // secId
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
		// [BAG combo legs would go here - skipped for non-BAG]
		// Deprecated + FA + model
		w.WriteString("") // deprecated sharesAllocation
		w.WriteString(m.DiscretionaryAmt)
		w.WriteString(m.GoodAfterTime)
		w.WriteString(m.GoodTillDate)
		w.WriteString(m.FAGroup)
		w.WriteString(m.FAMethod)
		w.WriteString(m.FAPercentage)
		// sv >= 177: no deprecated faProfile
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
		// [deltaNeutralOrderType="" => skip extended delta neutral fields]
		w.WriteString(m.ContinuousUpdate)
		w.WriteString(m.ReferencePriceType)
		// Trailing
		w.WriteString(m.TrailStopPrice)
		w.WriteString(m.TrailingPercent)
		// Scale
		w.WriteString(m.ScaleInitLevelSize)
		w.WriteString(m.ScaleSubsLevelSize)
		w.WriteString(m.ScalePriceIncrement)
		// [scalePriceIncrement empty/unset => skip scale3 extended]
		w.WriteString(m.ScaleTable)
		w.WriteString(m.ActiveStartTime)
		w.WriteString(m.ActiveStopTime)
		// Hedge
		w.WriteString(m.HedgeType)
		// [hedgeType="" => skip hedgeParam]
		// Misc
		w.WriteString(m.OptOutSmartRouting)
		w.WriteString(m.ClearingAccount)
		w.WriteString(m.ClearingIntent)
		w.WriteString(m.NotHeld)
		w.WriteString(m.DeltaNeutralContractPresent)
		// [DeltaNeutralContractPresent="0" => skip DNC fields]
		w.WriteString(m.AlgoStrategy)
		// [AlgoStrategy="" => skip algo params]
		w.WriteString(m.AlgoID)
		w.WriteString(m.WhatIf)
		w.WriteString(m.OrderMiscOptions)
		w.WriteString(m.Solicited)
		w.WriteString(m.RandomizeSize)
		w.WriteString(m.RandomizePrice)
		// [OrderType != "PEG BENCH" => skip peg bench fields]
		w.WriteString(m.ConditionsCount)
		// [ConditionsCount="0" => skip condition details]
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
		w.WriteString(m.CustomerAccount)
		w.WriteString(m.ProfessionalCustomer)
		// [sv >= 190 => no RFQ fields]
		w.WriteString(m.IncludeOvernight)
		w.WriteString(m.ManualOrderIndicator)
		w.WriteString(m.ImbalanceOnly)
		return w.Fields(), nil

	case CancelOrderRequest:
		w := fieldWriter{}
		w.WriteInt(OutCancelOrder)
		w.WriteInt64(m.OrderID)
		w.WriteString(m.ManualOrderCancelTime)
		return w.Fields(), nil

	case GlobalCancelRequest:
		return []string{itoa(OutReqGlobalCancel), "1"}, nil

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
		// intermediate fields to keep positions aligned. Total: msg_id + 169 fields.
		w := fieldWriter{}
		w.WriteInt(InOpenOrder)
		w.WriteInt64(m.OrderID)           // r[0]
		writeWireContract(&w, m.Contract) // r[1..11]
		w.WriteString(m.Action)           // r[12]
		w.WriteString(m.Quantity)         // r[13]
		w.WriteString(m.OrderType)        // r[14]
		w.WriteString(m.LmtPrice)         // r[15]
		w.WriteString(m.AuxPrice)         // r[16]
		w.WriteString(m.TIF)              // r[17]
		w.WriteString(m.OcaGroup)         // r[18]
		w.WriteString(m.Account)          // r[19]
		w.WriteString(m.OpenClose)        // r[20]
		w.WriteString(m.Origin)           // r[21]
		w.WriteString(m.OrderRef)         // r[22]
		w.WriteString(m.ClientID)         // r[23]
		w.WriteString(m.PermID)           // r[24]
		w.WriteString(m.OutsideRTH)       // r[25]
		w.WriteString(m.Hidden)           // r[26]
		w.WriteString(m.DiscretionAmt)    // r[27]
		w.WriteString(m.GoodAfterTime)    // r[28]
		for range 62 {                    // r[29..90] order detail padding
			w.WriteString("")
		}
		w.WriteString(m.Status)               // r[91]
		w.WriteString(m.InitMarginBefore)     // r[92]
		w.WriteString(m.MaintMarginBefore)    // r[93]
		w.WriteString(m.EquityWithLoanBefore) // r[94]
		w.WriteString(m.InitMarginChange)     // r[95]
		w.WriteString(m.MaintMarginChange)    // r[96]
		w.WriteString(m.EquityWithLoanChange) // r[97]
		w.WriteString(m.InitMarginAfter)      // r[98]
		w.WriteString(m.MaintMarginAfter)     // r[99]
		w.WriteString(m.EquityWithLoanAfter)  // r[100]
		w.WriteString(m.Commission)           // r[101]
		w.WriteString(m.MinCommission)        // r[102]
		w.WriteString(m.MaxCommission)        // r[103]
		w.WriteString(m.CommissionCurrency)   // r[104]
		w.WriteString(m.WarningText)          // r[105]
		for range 54 {                        // r[106..159] post-commission padding
			w.WriteString("")
		}
		w.WriteString(m.Filled)    // r[160]
		w.WriteString(m.Remaining) // r[161]
		for range 7 {              // r[162..168] trailing status padding
			w.WriteString("")
		}
		return w.Fields(), nil

	case OrderStatus:
		w := fieldWriter{}
		w.WriteInt(InOrderStatus)
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

	case OpenOrderEnd:
		return []string{itoa(InOpenOrderEnd), "1"}, nil

	case ExecutionDetail:
		return []string{
			itoa(InExecutionData), itoa(m.ReqID),
			i64toa(m.OrderID), "0",
			m.Symbol, "", "", "", "", "", "", "", "",
			m.ExecID, m.Time, m.Account,
			"",
			m.Side, m.Shares, m.Price,
		}, nil

	case ExecutionsEnd:
		return []string{itoa(InExecutionDataEnd), "1", itoa(m.ReqID)}, nil

	case CommissionReport:
		return []string{itoa(InCommissionReport), "1", m.ExecID, m.Commission, m.Currency, m.RealizedPNL}, nil

	case TickGeneric:
		return []string{itoa(InTickGeneric), "6", itoa(m.ReqID), itoa(m.TickType), m.Value}, nil

	case TickString:
		return []string{itoa(InTickString), "6", itoa(m.ReqID), itoa(m.TickType), m.Value}, nil

	case TickReqParams:
		return []string{itoa(InTickReqParams), itoa(m.ReqID), m.MinTick, m.BBOExchange, itoa(m.SnapshotPermissions)}, nil

	case FamilyCodes:
		w := fieldWriter{}
		w.WriteInt(InFamilyCodes)
		w.WriteInt(len(m.Codes))
		for _, c := range m.Codes {
			w.WriteString(c.AccountID)
			w.WriteString(c.FamilyCode)
		}
		return w.Fields(), nil

	case MktDepthExchanges:
		w := fieldWriter{}
		w.WriteInt(InMktDepthExchanges)
		w.WriteInt(len(m.Exchanges))
		for _, e := range m.Exchanges {
			w.WriteString(e.Exchange)
			w.WriteString(e.SecType)
			w.WriteString(e.ListingExch)
			w.WriteString(e.ServiceDataType)
			w.WriteInt(e.AggGroup)
		}
		return w.Fields(), nil

	case NewsProviders:
		w := fieldWriter{}
		w.WriteInt(InNewsProviders)
		w.WriteInt(len(m.Providers))
		for _, p := range m.Providers {
			w.WriteString(p.Code)
			w.WriteString(p.Name)
		}
		return w.Fields(), nil

	case ScannerParameters:
		return []string{itoa(InScannerParameters), "1", m.XML}, nil

	case UserInfo:
		return []string{itoa(InUserInfo), itoa(m.ReqID), m.WhiteBrandingID}, nil

	case MatchingSymbols:
		w := fieldWriter{}
		w.WriteInt(InSymbolSamples)
		w.WriteInt(m.ReqID)
		w.WriteInt(len(m.Symbols))
		for _, s := range m.Symbols {
			w.WriteInt(s.ConID)
			w.WriteString(s.Symbol)
			w.WriteString(s.SecType)
			w.WriteString(s.PrimaryExchange)
			w.WriteString(s.Currency)
			w.WriteInt(len(s.DerivativeSecTypes))
			for _, dt := range s.DerivativeSecTypes {
				w.WriteString(dt)
			}
		}
		return w.Fields(), nil

	case HeadTimestamp:
		return []string{itoa(InHeadTimestamp), itoa(m.ReqID), m.Timestamp}, nil

	case MarketRule:
		w := fieldWriter{}
		w.WriteInt(InMarketRule)
		w.WriteInt(m.MarketRuleID)
		w.WriteInt(len(m.Increments))
		for _, inc := range m.Increments {
			w.WriteString(inc.LowEdge)
			w.WriteString(inc.Increment)
		}
		return w.Fields(), nil

	case CompletedOrder:
		// Simplified encoder for testhost: server->client contract format
		// (conID, symbol, secType, expiry, strike, right, multiplier, exchange,
		// currency, localSymbol, tradingClass) padded to match the decoder's
		// Skip(4) + Skip(71) + status + Skip(3) pattern.
		w := fieldWriter{}
		w.WriteInt(InCompletedOrder)
		// Server->client 11-field contract block
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
		for range 4 { // lmtPrice, auxPrice, tif, ocaGroup
			w.WriteString("")
		}
		for range 71 { // order detail fields to OrderState
			w.WriteString("")
		}
		w.WriteString(m.Status)
		for range 3 { // completedTime, completedStatus, misc
			w.WriteString("")
		}
		w.WriteString(m.Filled)
		w.WriteString(m.Remaining)
		return w.Fields(), nil

	case CompletedOrderEnd:
		return []string{itoa(InCompletedOrderEnd)}, nil

	case UpdateAccountValue:
		return []string{itoa(InUpdateAccountValue), "2", m.Key, m.Value, m.Currency, m.Account}, nil

	case UpdatePortfolio:
		w := fieldWriter{}
		w.WriteInt(InUpdatePortfolio)
		w.WriteInt(8) // version
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
		w.WriteString(m.Contract.PrimaryExchange)
		w.WriteString(m.Contract.Currency)
		w.WriteString(m.Contract.LocalSymbol)
		w.WriteString(m.Contract.TradingClass)
		w.WriteString(m.Position)
		w.WriteString(m.MarketPrice)
		w.WriteString(m.MarketValue)
		w.WriteString(m.AvgCost)
		w.WriteString(m.UnrealizedPNL)
		w.WriteString(m.RealizedPNL)
		w.WriteString(m.Account)
		return w.Fields(), nil

	case UpdateAccountTime:
		return []string{itoa(InUpdateAccountTime), "1", m.Timestamp}, nil

	case AccountDownloadEnd:
		return []string{itoa(InAccountDownloadEnd), "1", m.Account}, nil

	case AccountUpdateMultiValue:
		return []string{itoa(InAccountUpdateMulti), "1", itoa(m.ReqID), m.Account, m.ModelCode, m.Key, m.Value, m.Currency}, nil

	case AccountUpdateMultiEnd:
		return []string{itoa(InAccountUpdateMultiEnd), "1", itoa(m.ReqID)}, nil

	case PositionMulti:
		w := fieldWriter{}
		w.WriteInt(InPositionMulti)
		w.WriteInt(1) // version
		w.WriteInt(m.ReqID)
		w.WriteString(m.Account)
		w.WriteString(m.ModelCode)
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

	case PositionMultiEnd:
		return []string{itoa(InPositionMultiEnd), "1", itoa(m.ReqID)}, nil

	case PnLValue:
		return []string{itoa(InPnL), itoa(m.ReqID), m.DailyPnL, m.UnrealizedPnL, m.RealizedPnL}, nil

	case PnLSingleValue:
		return []string{itoa(InPnLSingle), itoa(m.ReqID), m.Position, m.DailyPnL, m.UnrealizedPnL, m.RealizedPnL, m.Value}, nil

	case TickByTickData:
		w := fieldWriter{}
		w.WriteInt(InTickByTick)
		w.WriteInt(m.ReqID)
		w.WriteInt(m.TickType)
		w.WriteString(m.Time)
		switch m.TickType {
		case 1, 2: // Last, AllLast
			w.WriteString(m.Price)
			w.WriteString(m.Size)
			w.WriteInt(m.TickAttribLast)
			w.WriteString(m.Exchange)
			w.WriteString(m.SpecialConditions)
		case 3: // BidAsk
			w.WriteString(m.BidPrice)
			w.WriteString(m.AskPrice)
			w.WriteString(m.BidSize)
			w.WriteString(m.AskSize)
			w.WriteInt(m.TickAttribBidAsk)
		case 4: // MidPoint
			w.WriteString(m.MidPoint)
		}
		return w.Fields(), nil

	case NewsBulletin:
		return []string{itoa(InNewsBulletins), "1", itoa(m.MsgID), itoa(m.MsgType), m.Headline, m.Source}, nil

	case SecDefOptParamsResponse:
		w := fieldWriter{}
		w.WriteInt(InSecDefOptParams)
		w.WriteInt(m.ReqID)
		w.WriteString(m.Exchange)
		w.WriteInt(m.UnderlyingConID)
		w.WriteString(m.TradingClass)
		w.WriteString(m.Multiplier)
		w.WriteString("") // marketRuleId
		w.WriteInt(len(m.Expirations))
		for _, exp := range m.Expirations {
			w.WriteString(exp)
		}
		w.WriteInt(len(m.Strikes))
		for _, strike := range m.Strikes {
			w.WriteString(strike)
		}
		return w.Fields(), nil

	case SecDefOptParamsEnd:
		return []string{itoa(InSecDefOptParamsEnd), itoa(m.ReqID)}, nil

	case SmartComponentsResponse:
		w := fieldWriter{}
		w.WriteInt(InSymbolSamples) // msg_id 82 shared with SymbolSamples
		w.WriteInt(m.ReqID)
		w.WriteInt(len(m.Components))
		for _, c := range m.Components {
			w.WriteInt(c.BitNumber)
			w.WriteString(c.ExchangeName)
			w.WriteString(c.ExchangeLetter)
		}
		return w.Fields(), nil

	case TickOptionComputation:
		return []string{
			itoa(InTickOptionComputation), "6", itoa(m.ReqID), itoa(m.TickType), itoa(m.TickAttrib),
			m.ImpliedVol, m.Delta, m.OptPrice, m.PvDividend, m.Gamma, m.Vega, m.Theta, m.UndPrice,
		}, nil

	case HistogramDataResponse:
		w := fieldWriter{}
		w.WriteInt(InHistogramData)
		w.WriteInt(m.ReqID)
		w.WriteInt(len(m.Entries))
		for _, e := range m.Entries {
			w.WriteString(e.Price)
			w.WriteString(e.Size)
		}
		return w.Fields(), nil

	case HistoricalTicksResponse:
		w := fieldWriter{}
		w.WriteInt(InHistoricalTicks)
		w.WriteInt(m.ReqID)
		w.WriteInt(len(m.Ticks))
		for _, t := range m.Ticks {
			w.WriteString(t.Time)
			w.WriteString("") // unused
			w.WriteString(t.Price)
			w.WriteString(t.Size)
		}
		w.WriteBool(m.Done)
		return w.Fields(), nil

	case HistoricalTicksBidAskResponse:
		w := fieldWriter{}
		w.WriteInt(InHistoricalTicksBidAsk)
		w.WriteInt(m.ReqID)
		w.WriteInt(len(m.Ticks))
		for _, t := range m.Ticks {
			w.WriteString(t.Time)
			w.WriteInt(0) // tickAttribBidAsk
			w.WriteString(t.BidPrice)
			w.WriteString(t.AskPrice)
			w.WriteString(t.BidSize)
			w.WriteString(t.AskSize)
		}
		w.WriteBool(m.Done)
		return w.Fields(), nil

	case HistoricalTicksLastResponse:
		w := fieldWriter{}
		w.WriteInt(InHistoricalTicksLast)
		w.WriteInt(m.ReqID)
		w.WriteInt(len(m.Ticks))
		for _, t := range m.Ticks {
			w.WriteString(t.Time)
			w.WriteInt(0) // tickAttribLast
			w.WriteString(t.Price)
			w.WriteString(t.Size)
			w.WriteString(t.Exchange)
			w.WriteString(t.SpecialConditions)
		}
		w.WriteBool(m.Done)
		return w.Fields(), nil

	case NewsArticleResponse:
		return []string{itoa(InNewsArticle), itoa(m.ReqID), itoa(m.ArticleType), m.ArticleText}, nil

	case HistoricalNewsItem:
		return []string{itoa(InHistoricalNews), itoa(m.ReqID), m.Time, m.ProviderCode, m.ArticleID, m.Headline}, nil

	case HistoricalNewsEnd:
		w := fieldWriter{}
		w.WriteInt(InMktDepthExchanges) // msg_id 80 shared with MktDepthExchanges
		w.WriteInt(m.ReqID)
		w.WriteBool(m.HasMore)
		return w.Fields(), nil

	case ScannerDataResponse:
		w := fieldWriter{}
		w.WriteInt(InScannerData)
		w.WriteInt(3) // version
		w.WriteInt(m.ReqID)
		w.WriteInt(len(m.Entries))
		for _, e := range m.Entries {
			w.WriteInt(e.Rank)
			// Server->client 11-field contract: conID, symbol, secType, expiry, strike, right, multiplier, exchange, currency, localSymbol, tradingClass
			w.WriteInt(e.Contract.ConID)
			w.WriteString(e.Contract.Symbol)
			w.WriteString(e.Contract.SecType)
			w.WriteString(e.Contract.Expiry)
			if e.Contract.Strike == "" {
				w.WriteString("0")
			} else {
				w.WriteString(e.Contract.Strike)
			}
			w.WriteString(e.Contract.Right)
			w.WriteString(e.Contract.Multiplier)
			w.WriteString(e.Contract.Exchange)
			w.WriteString(e.Contract.Currency)
			w.WriteString(e.Contract.LocalSymbol)
			w.WriteString(e.Contract.TradingClass)
			w.WriteString(e.Distance)
			w.WriteString(e.Benchmark)
			w.WriteString(e.Projection)
			w.WriteString(e.LegsStr)
		}
		return w.Fields(), nil

	case ReceiveFA:
		return []string{itoa(InReceiveFA), "1", itoa(m.FADataType), m.XML}, nil

	case SoftDollarTiersResponse:
		w := fieldWriter{}
		w.WriteInt(InSoftDollarTiers)
		w.WriteInt(m.ReqID)
		w.WriteInt(len(m.Tiers))
		for _, t := range m.Tiers {
			w.WriteString(t.Name)
			w.WriteString(t.Value)
			w.WriteString(t.DisplayName)
		}
		return w.Fields(), nil

	case WSHMetaDataResponse:
		return []string{itoa(InWSHMetaData), itoa(m.ReqID), m.DataJSON}, nil

	case WSHEventDataResponse:
		return []string{itoa(InWSHEventData), itoa(m.ReqID), m.DataJSON}, nil

	case DisplayGroupList:
		return []string{itoa(InDisplayGroupList), "1", itoa(m.ReqID), m.Groups}, nil

	case DisplayGroupUpdated:
		return []string{itoa(InDisplayGroupUpdated), "1", itoa(m.ReqID), m.ContractInfo}, nil

	case MarketDepthUpdate:
		return []string{itoa(InMarketDepth), "6", itoa(m.ReqID), itoa(m.Position), itoa(m.Operation), itoa(m.Side), m.Price, m.Size}, nil

	case MarketDepthL2Update:
		w := fieldWriter{}
		w.WriteInt(InMarketDepthL2)
		w.WriteInt(6) // version
		w.WriteInt(m.ReqID)
		w.WriteInt(m.Position)
		w.WriteString(m.MarketMaker)
		w.WriteInt(m.Operation)
		w.WriteInt(m.Side)
		w.WriteString(m.Price)
		w.WriteString(m.Size)
		w.WriteBool(m.IsSmartDepth)
		return w.Fields(), nil

	case FundamentalDataResponse:
		return []string{itoa(InFundamentalData), "1", itoa(m.ReqID), m.Data}, nil

	case HistoricalDataUpdate:
		w := fieldWriter{}
		w.WriteInt(InHistoricalDataUpdate)
		w.WriteInt(m.ReqID)
		w.WriteInt(m.BarCount)
		w.WriteString(m.Time)
		w.WriteString(m.Open)
		w.WriteString(m.High)
		w.WriteString(m.Low)
		w.WriteString(m.Close)
		w.WriteString(m.Volume)
		w.WriteString(m.WAP)
		w.WriteString(m.Count)
		return w.Fields(), nil

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

func itoa(v int) string     { return strconv.Itoa(v) }
func i64toa(v int64) string { return strconv.FormatInt(v, 10) }

func btoa(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

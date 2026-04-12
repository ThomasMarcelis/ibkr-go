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
	msgs, err := decodeByMsgID(msgID, fields)
	if err != nil {
		return nil, fmt.Errorf("codec: msg_id %d: %w", msgID, err)
	}
	return msgs, nil
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

func isWireInt(value string) bool {
	if value == "" {
		return false
	}
	_, err := strconv.Atoi(value)
	return err == nil
}

func isHistoricalRangeBoundary(value string) bool {
	if value == "" || isWireInt(value) {
		return false
	}
	return strings.Contains(value, " ") && strings.Contains(value, "/")
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
func decodeByMsgID(msgID int, fields []string) (msgs []Message, err error) {
	r := newFieldReader(fields[1:]) // skip msg_id
	defer func() {
		if err == nil && r.Err() != nil {
			err = r.Err()
			msgs = nil
		}
	}()
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

		if r.Len() == 169 {
			r.Skip(62)
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
			warningText := r.ReadString()
			r.Skip(54)
			filled := r.ReadString()
			remaining := r.ReadString()
			r.Skip(2)
			parentID := r.ReadString()
			return []Message{OpenOrder{
				OrderID: orderID, Contract: contract,
				Action: action, Quantity: quantity, OrderType: orderType,
				LmtPrice: lmtPrice, AuxPrice: auxPrice, TIF: tif,
				OcaGroup: ocaGroup, Account: account,
				OpenClose: openClose, Origin: origin, OrderRef: orderRef,
				ClientID: clientID, PermID: permID, OutsideRTH: outsideRTH,
				Hidden: hidden, DiscretionAmt: discretionAmt, GoodAfterTime: goodAfterTime,
				Status:               status,
				InitMarginBefore:     initMarginBefore,
				MaintMarginBefore:    maintMarginBefore,
				EquityWithLoanBefore: equityWithLoanBefore,
				InitMarginChange:     initMarginChange,
				MaintMarginChange:    maintMarginChange,
				EquityWithLoanChange: equityWithLoanChange,
				InitMarginAfter:      initMarginAfter,
				MaintMarginAfter:     maintMarginAfter,
				EquityWithLoanAfter:  equityWithLoanAfter,
				Commission:           commission,
				MinCommission:        minCommission,
				MaxCommission:        maxCommission,
				CommissionCurrency:   commissionCurrency,
				WarningText:          warningText,
				Filled:               filled,
				Remaining:            remaining,
				ParentID:             parentID,
			}}, nil
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
		r.ReadString() // ParentID (pre-status)
		r.ReadString() // TriggerMethod

		r.ReadString() // Volatility
		r.ReadString() // VolatilityType
		deltaNeutralOrderType := r.ReadString()
		r.ReadString() // DeltaNeutralAuxPrice
		if deltaNeutralOrderType != "" {
			return []Message{partial}, nil
		}
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
		if isPositiveWireNumber(scalePriceIncrement) {
			return []Message{partial}, nil
		}
		r.ReadString() // ScaleTable
		r.ReadString() // ActiveStartTime
		r.ReadString() // ActiveStopTime
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
		if err := r.RequireFixedEntryFields("open order allocations", orderAllocationsCount, 8, 0); err != nil {
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
			r.ReadString() // reserved
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
		if r.Remaining() != 40 {
			return []Message{partial}, nil
		}
		r.ReadString() // AdjustedOrderType
		r.ReadString() // TriggerPrice
		r.ReadString() // LmtPriceOffset
		r.ReadString() // AdjustedStopPrice
		r.ReadString() // AdjustedStopLimitPrice
		r.ReadString() // AdjustedTrailingAmount
		r.ReadString() // AdjustableTrailingUnit
		r.ReadString() // SoftDollarName
		r.ReadString() // SoftDollarValue
		r.ReadString() // SoftDollarDisplayName
		r.ReadString() // CashQty
		r.ReadString() // DontUseAutoPriceForHedge
		r.ReadString() // IsOmsContainer
		r.ReadString() // DiscretionaryUpToLimitPrice
		r.ReadString() // UsePriceMgmtAlgo
		r.ReadString() // Duration
		r.ReadString() // PostToAts
		r.ReadString() // AutoCancelParent
		r.ReadString() // MinTradeQty
		r.ReadString() // MinCompeteSize
		r.ReadString() // CompeteAgainstBestOffset
		r.ReadString() // MidOffsetAtWhole
		r.ReadString() // MidOffsetAtHalf
		r.ReadString() // CustomerAccount
		r.ReadString() // ProfessionalCustomer
		r.ReadString() // BondAccruedInterest
		r.ReadString() // IncludeOvernight
		r.ReadString() // ExtOperator
		r.ReadString() // ManualOrderIndicator
		r.ReadString() // Submitter
		r.ReadString() // ImbalanceOnly
		filled := r.ReadString()
		remaining := r.ReadString()
		r.ReadString() // lastFillPrice
		r.ReadString() // permId
		parentID := r.ReadString()
		r.ReadString() // lastLiquidity
		r.ReadString() // whyHeld
		r.ReadString() // mktCapPrice
		r.ReadString() // trailing

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
			Filled:                filled,
			Remaining:             remaining,
			ParentID:              parentID,
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
		reqID, err := r.ReadInt()
		if err != nil {
			return nil, err
		}
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

	case InMktDepthExchanges:
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

	case InSymbolSamples: // [79, reqID, count, repeated(conID, symbol, secType, primaryExch, currency, derivCount, derivTypes...)]
		reqID, _ := r.ReadInt()
		count, err := r.ReadCount("sample count")
		if err != nil {
			return nil, err
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
			description := ""
			issuerID := ""
			if r.Remaining() >= 2 && !isWireInt(r.fields[r.pos]) {
				description = r.ReadString()
				issuerID = r.ReadString()
			}
			symbols[i] = SymbolSample{
				ConID: conID, Symbol: symbol, SecType: secType,
				PrimaryExchange: primaryExch, Currency: currency,
				DerivativeSecTypes: derivTypes,
				Description:        description, IssuerID: issuerID,
			}
		}
		return []Message{MatchingSymbols{ReqID: reqID, Symbols: symbols}}, nil

	case InSmartComponents: // [82, reqID, count, repeated(bitNumber, exchangeName, exchangeLetter)]
		reqID, _ := r.ReadInt()
		count, err := r.ReadCount("smart component count")
		if err != nil {
			return nil, err
		}
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

	case InHistoricalNews: // [86, reqID, time, providerCode, articleId, headline] — no version
		reqID, _ := r.ReadInt()
		timeStr := r.ReadString()
		providerCode := r.ReadString()
		articleID := r.ReadString()
		headline := r.ReadString()
		return []Message{HistoricalNewsItem{ReqID: reqID, Time: timeStr, ProviderCode: providerCode, ArticleID: articleID, Headline: headline}}, nil

	case InHistoricalNewsEnd: // [87, reqID, hasMore]
		reqID, _ := r.ReadInt()
		hasMore, _ := r.ReadBool()
		return []Message{HistoricalNewsEnd{ReqID: reqID, HasMore: hasMore}}, nil

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
			tickAttrib, _ := r.ReadInt()
			bidPrice := r.ReadString()
			askPrice := r.ReadString()
			bidSize := r.ReadString()
			askSize := r.ReadString()
			ticks[i] = HistoricalTickBidAskEntry{Time: timeStr, TickAttrib: tickAttrib, BidPrice: bidPrice, AskPrice: askPrice, BidSize: bidSize, AskSize: askSize}
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
			tickAttrib, _ := r.ReadInt()
			price := r.ReadString()
			size := r.ReadString()
			exchange := r.ReadString()
			specialConditions := r.ReadString()
			ticks[i] = HistoricalTickLastEntry{Time: timeStr, TickAttrib: tickAttrib, Price: price, Size: size, Exchange: exchange, SpecialConditions: specialConditions}
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

	case InHistoricalDataUpdate:
		// Live Gateway v200 sends two distinct msg_id 108 shapes:
		//   [108, reqID, barCount, time, O, H, L, C, vol, wap, count]
		//   [108, reqID, startDateTime, endDateTime]
		// Older captures also show [108, reqID, startDateTime]. The range
		// shapes are terminal markers for the preceding historical data batch.
		if (len(r.fields) == 2 || len(r.fields) == 3) && isWireInt(r.fields[0]) && isHistoricalRangeBoundary(r.fields[1]) {
			reqID, _ := strconv.Atoi(r.fields[0])
			return []Message{HistoricalBarsEnd{ReqID: reqID}}, nil
		}
		reqID, err := r.ReadInt()
		if err != nil {
			return nil, err
		}
		barCount, err := r.ReadCount("historical data update bar count")
		if err != nil {
			return nil, err
		}
		if err := r.RequireFixedEntryFields("historical data update", 1, 8, 0); err != nil {
			return nil, err
		}
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

	case InWSHMetaData: // [104, reqId, dataJson]
		reqID, _ := r.ReadInt()
		dataJSON := r.ReadString()
		return []Message{WSHMetaDataResponse{ReqID: reqID, DataJSON: dataJSON}}, nil

	case InWSHEventData: // [105, reqId, dataJson]
		reqID, _ := r.ReadInt()
		dataJSON := r.ReadString()
		return []Message{WSHEventDataResponse{ReqID: reqID, DataJSON: dataJSON}}, nil

	case InHistoricalSchedule: // [106, reqId, startDateTime, endDateTime, timeZone, sessionCount, (startDateTime,endDateTime,refDate)*count]
		reqID, _ := r.ReadInt()
		startDateTime := r.ReadString()
		endDateTime := r.ReadString()
		timeZone := r.ReadString()
		sessionCount, err := r.ReadCount("historical schedule session count")
		if err != nil {
			return nil, err
		}
		if err := r.RequireFixedEntryFields("historical schedule", sessionCount, 3, 0); err != nil {
			return nil, err
		}
		sessions := make([]HistoricalScheduleSession, sessionCount)
		for i := 0; i < sessionCount; i++ {
			sessions[i] = HistoricalScheduleSession{
				StartDateTime: r.ReadString(),
				EndDateTime:   r.ReadString(),
				RefDate:       r.ReadString(),
			}
		}
		return []Message{HistoricalScheduleResponse{
			ReqID:         reqID,
			StartDateTime: startDateTime,
			EndDateTime:   endDateTime,
			TimeZone:      timeZone,
			Sessions:      sessions,
		}}, nil

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

	case CurrentTimeRequest:
		return []string{itoa(OutReqCurrentTime), "1"}, nil

	case ReqIDsRequest:
		numIDs := m.NumIDs
		if numIDs <= 0 {
			numIDs = 1
		}
		return []string{itoa(OutReqIds), "1", itoa(numIDs)}, nil

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
		if m.Contract.SecType == "BAG" || len(m.ComboLegs) > 0 || len(m.OrderComboLegPrices) > 0 || len(m.SmartComboRoutingParams) > 0 {
			w.WriteInt(len(m.ComboLegs))
			for _, leg := range m.ComboLegs {
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
		w.WriteString(m.DeltaNeutralContractPresent)
		// grounded v1.2 leaves delta-neutral contract fields deferred
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
		w := fieldWriter{}
		w.WriteInt(InOpenOrder)
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
		w.WriteString("") // DeltaNeutralOrderType
		w.WriteString("") // DeltaNeutralAuxPrice
		w.WriteString("") // ContinuousUpdate
		w.WriteString("") // ReferencePriceType
		w.WriteString("") // TrailStopPrice
		w.WriteString("") // TrailingPercent
		w.WriteString("") // BasisPoints
		w.WriteString("") // BasisPointsType
		w.WriteString("") // ComboLegsDescrip
		w.WriteInt(len(m.ComboLegs))
		for _, leg := range m.ComboLegs {
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
		w.WriteString("") // ScaleInitLevelSize
		w.WriteString("") // ScaleSubsLevelSize
		w.WriteString("") // ScalePriceIncrement
		w.WriteString("") // ScaleTable
		w.WriteString("") // ActiveStartTime
		w.WriteString("") // ActiveStopTime
		w.WriteString("") // HedgeType
		w.WriteString("") // OptOutSmartRouting
		w.WriteString("") // ClearingAccount
		w.WriteString("") // ClearingIntent
		w.WriteString("") // NotHeld
		w.WriteString("0")
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
		w.WriteString("") // AdjustedOrderType
		w.WriteString("") // TriggerPrice
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
		w.WriteString(m.Filled)
		w.WriteString(m.Remaining)
		w.WriteString("") // lastFillPrice
		w.WriteString("") // permId
		w.WriteString(m.ParentID)
		w.WriteString("") // lastLiquidity
		w.WriteString("") // whyHeld
		w.WriteString("") // mktCapPrice
		w.WriteString("") // trailing
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
			w.WriteString(s.Description)
			w.WriteString(s.IssuerID)
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
		w.WriteInt(InSmartComponents)
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
			w.WriteInt(t.TickAttrib)
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
			w.WriteInt(t.TickAttrib)
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
		w.WriteInt(InHistoricalNewsEnd)
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

	case HistoricalScheduleResponse:
		w := fieldWriter{}
		w.WriteInt(InHistoricalSchedule)
		w.WriteInt(m.ReqID)
		w.WriteString(m.StartDateTime)
		w.WriteString(m.EndDateTime)
		w.WriteString(m.TimeZone)
		w.WriteInt(len(m.Sessions))
		for _, s := range m.Sessions {
			w.WriteString(s.StartDateTime)
			w.WriteString(s.EndDateTime)
			w.WriteString(s.RefDate)
		}
		return w.Fields(), nil

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

func isPositiveWireNumber(raw string) bool {
	if raw == "" {
		return false
	}
	v, err := strconv.ParseFloat(raw, 64)
	return err == nil && v > 0
}

func mustReadInt(r *fieldReader) int {
	v, _ := r.ReadInt()
	return v
}

func mustReadBool(r *fieldReader) bool {
	v, _ := r.ReadBool()
	return v
}

func readTagValuePairs(r *fieldReader, label string, count int) ([]TagValue, error) {
	if err := r.RequireFixedEntryFields(label, count, 2, 0); err != nil {
		return nil, err
	}
	values := make([]TagValue, count)
	for i := range values {
		values[i] = TagValue{Tag: r.ReadString(), Value: r.ReadString()}
	}
	return values, nil
}

func writeTagValuePairs(w *fieldWriter, values []TagValue) {
	w.WriteInt(len(values))
	for _, value := range values {
		w.WriteString(value.Tag)
		w.WriteString(value.Value)
	}
}

func readOrderCondition(r *fieldReader, conditionType int) (OrderCondition, error) {
	cond := OrderCondition{Type: conditionType}
	switch conditionType {
	case 1: // Price
		cond.Conjunction = r.ReadString()
		if more, err := r.ReadBool(); err != nil {
			return OrderCondition{}, err
		} else if more {
			cond.Operator = 2
		} else {
			cond.Operator = 1
		}
		cond.ConID, _ = r.ReadInt()
		cond.Exchange = r.ReadString()
		cond.Value = r.ReadString()
		cond.TriggerMethod, _ = r.ReadInt()
	case 3: // Time
		cond.Conjunction = r.ReadString()
		if more, err := r.ReadBool(); err != nil {
			return OrderCondition{}, err
		} else if more {
			cond.Operator = 2
		} else {
			cond.Operator = 1
		}
		cond.Value = r.ReadString()
	case 4: // Margin
		cond.Conjunction = r.ReadString()
		if more, err := r.ReadBool(); err != nil {
			return OrderCondition{}, err
		} else if more {
			cond.Operator = 2
		} else {
			cond.Operator = 1
		}
		cond.Value = r.ReadString()
	case 5: // Execution
		cond.Conjunction = r.ReadString()
		cond.SecType = r.ReadString()
		cond.Exchange = r.ReadString()
		cond.Symbol = r.ReadString()
	case 6: // Volume
		cond.Conjunction = r.ReadString()
		if more, err := r.ReadBool(); err != nil {
			return OrderCondition{}, err
		} else if more {
			cond.Operator = 2
		} else {
			cond.Operator = 1
		}
		cond.ConID, _ = r.ReadInt()
		cond.Exchange = r.ReadString()
		cond.Value = r.ReadString()
	case 7: // Percent change
		cond.Conjunction = r.ReadString()
		if more, err := r.ReadBool(); err != nil {
			return OrderCondition{}, err
		} else if more {
			cond.Operator = 2
		} else {
			cond.Operator = 1
		}
		cond.ConID, _ = r.ReadInt()
		cond.Exchange = r.ReadString()
		cond.Value = r.ReadString()
	default:
		return OrderCondition{}, fmt.Errorf("codec: unsupported order condition type %d", conditionType)
	}
	return cond, nil
}

func writeOrderCondition(w *fieldWriter, cond OrderCondition) error {
	w.WriteInt(cond.Type)
	if cond.Conjunction == "o" {
		w.WriteString("o")
	} else {
		w.WriteString("a")
	}
	isMore := cond.Operator == 2
	switch cond.Type {
	case 1:
		w.WriteBool(isMore)
		w.WriteInt(cond.ConID)
		w.WriteString(cond.Exchange)
		w.WriteString(cond.Value)
		w.WriteInt(cond.TriggerMethod)
	case 3, 4:
		w.WriteBool(isMore)
		w.WriteString(cond.Value)
	case 5:
		w.WriteString(cond.SecType)
		w.WriteString(cond.Exchange)
		w.WriteString(cond.Symbol)
	case 6, 7:
		w.WriteBool(isMore)
		w.WriteInt(cond.ConID)
		w.WriteString(cond.Exchange)
		w.WriteString(cond.Value)
	default:
		return fmt.Errorf("codec: unsupported order condition type %d", cond.Type)
	}
	return nil
}

func writeObservedWireContract(w *fieldWriter, c Contract) {
	w.WriteInt(c.ConID)
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
	w.WriteString(c.Currency)
	w.WriteString(c.LocalSymbol)
	w.WriteString(c.TradingClass)
}

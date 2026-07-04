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

func completedOrderTail(fields []string, statusIndex int, orderType string) (string, bool) {
	r := newFieldReader(fields[statusIndex+1:])
	if err := skipCompletedOrderPostStatusPrefix(r, orderType); err != nil {
		return "", false
	}
	filled := r.ReadString()
	if r.Err() != nil || !isNonNegativeWireNumber(filled) {
		return "", false
	}
	if r.Remaining() < 8 {
		return "", false
	}
	r.Skip(7)      // refFuturesConId through completedTime
	r.ReadString() // completedStatus
	if r.Err() != nil || r.Remaining() > 8 {
		return "", false
	}
	return filled, true
}

func completedOrderStatusTail(fields []string, orderType string) (string, string, error) {
	for i := 15; i < len(fields); i++ {
		if !isOrderStatusField(fields[i]) {
			continue
		}
		filled, ok := completedOrderTail(fields, i, orderType)
		if ok {
			return fields[i], filled, nil
		}
	}
	return "", "", fmt.Errorf("codec: completed order status tail not found")
}

func isOrderStatusField(value string) bool {
	switch value {
	case "PendingSubmit", "PendingCancel", "PreSubmitted", "Submitted",
		"ApiPending", "ApiCancelled", "Cancelled", "Filled", "Inactive":
		return true
	default:
		return false
	}
}

func skipCompletedOrderPostStatusPrefix(r *fieldReader, orderType string) error {
	r.Skip(2) // randomizeSize, randomizePrice
	if orderType == "PEG BENCH" {
		r.Skip(5)
	}
	conditionsCount, err := r.ReadOptionalCount("completed order conditions")
	if err != nil {
		return err
	}
	if conditionsCount > 0 {
		for range conditionsCount {
			conditionType, err := r.ReadInt()
			if err != nil {
				return err
			}
			if _, err := readOrderCondition(r, conditionType); err != nil {
				return err
			}
		}
		r.Skip(2) // conditionsIgnoreRTH, conditionsCancelOrder
	}
	r.Skip(2) // stop price, limit price offset
	r.Skip(4) // cashQty, dontUseAutoPriceForHedge, isOmsContainer, autoCancelDate
	return nil
}

// Encode encodes a message in the real TWS wire format (integer msg_id prefix).
func Encode(msg Message) ([]byte, error) {
	fields, err := encodeFields(msg)
	if err != nil {
		return nil, err
	}
	return wire.EncodeFields(fields), nil
}

type decodeFunc func(r *fieldReader) ([]Message, error)

// inboundDecoders maps msg_id to its decoder. One explicit table, no
// init() registration.
var inboundDecoders = map[int]decodeFunc{
	InTickPrice:             decodeTickPrice,
	InTickSize:              decodeTickSize,
	InOrderStatus:           decodeOrderStatus,
	InErrMsg:                decodeErrMsg,
	InOpenOrder:             decodeOpenOrder,
	InCurrentTimeInMillis:   decodeCurrentTimeInMillis,
	InNextValidID:           decodeNextValidID,
	InContractData:          decodeContractData,
	InExecutionData:         decodeExecutionData,
	InMarketDepth:           decodeMarketDepth,
	InMarketDepthL2:         decodeMarketDepthL2,
	InManagedAccounts:       decodeManagedAccounts,
	InHistoricalData:        decodeHistoricalData,
	InScannerParameters:     decodeScannerParameters,
	InScannerData:           decodeScannerData,
	InTickOptionComputation: decodeTickOptionComputation,
	InTickGeneric:           decodeTickGeneric,
	InTickString:            decodeTickString,
	InTickReqParams:         decodeTickReqParams,
	InCurrentTime:           decodeCurrentTime,
	InRealTimeBars:          decodeRealTimeBars,
	InFundamentalData:       decodeFundamentalData,
	InContractDataEnd:       decodeContractDataEnd,
	InOpenOrderEnd:          decodeOpenOrderEnd,
	InExecutionDataEnd:      decodeExecutionDataEnd,
	InTickSnapshotEnd:       decodeTickSnapshotEnd,
	InMarketDataType:        decodeMarketDataType,
	InCommissionReport:      decodeCommissionReport,
	InPositionData:          decodePositionData,
	InPositionEnd:           decodePositionEnd,
	InAccountSummary:        decodeAccountSummary,
	InAccountSummaryEnd:     decodeAccountSummaryEnd,
	InSecDefOptParams:       decodeSecDefOptParams,
	InSecDefOptParamsEnd:    decodeSecDefOptParamsEnd,
	InFamilyCodes:           decodeFamilyCodes,
	InMktDepthExchanges:     decodeMktDepthExchanges,
	InNewsArticle:           decodeNewsArticle,
	InNewsProviders:         decodeNewsProviders,
	InSymbolSamples:         decodeSymbolSamples,
	InSmartComponents:       decodeSmartComponents,
	InHistoricalNews:        decodeHistoricalNews,
	InHistoricalNewsEnd:     decodeHistoricalNewsEnd,
	InHeadTimestamp:         decodeHeadTimestamp,
	InHistogramData:         decodeHistogramData,
	InMarketRule:            decodeMarketRule,
	InCompletedOrder:        decodeCompletedOrder,
	InCompletedOrderEnd:     decodeCompletedOrderEnd,
	InUserInfo:              decodeUserInfo,
	InUpdateAccountValue:    decodeUpdateAccountValue,
	InUpdatePortfolio:       decodeUpdatePortfolio,
	InUpdateAccountTime:     decodeUpdateAccountTime,
	InAccountDownloadEnd:    decodeAccountDownloadEnd,
	InNewsBulletins:         decodeNewsBulletins,
	InPositionMulti:         decodePositionMulti,
	InPositionMultiEnd:      decodePositionMultiEnd,
	InAccountUpdateMulti:    decodeAccountUpdateMulti,
	InAccountUpdateMultiEnd: decodeAccountUpdateMultiEnd,
	InPnL:                   decodePnL,
	InPnLSingle:             decodePnLSingle,
	InHistoricalTicks:       decodeHistoricalTicks,
	InHistoricalTicksBidAsk: decodeHistoricalTicksBidAsk,
	InHistoricalTicksLast:   decodeHistoricalTicksLast,
	InTickByTick:            decodeTickByTick,
	InHistoricalDataUpdate:  decodeHistoricalDataUpdate,
	InReceiveFA:             decodeReceiveFA,
	InSoftDollarTiers:       decodeSoftDollarTiers,
	InWSHMetaData:           decodeWSHMetaData,
	InWSHEventData:          decodeWSHEventData,
	InHistoricalSchedule:    decodeHistoricalSchedule,
	InDisplayGroupList:      decodeDisplayGroupList,
	InDisplayGroupUpdated:   decodeDisplayGroupUpdated,
}

// decodeByMsgID dispatches on the integer message ID and reads fields in real TWS wire layout.
// Returns []Message because historical data packs multiple bars into one frame.
func decodeByMsgID(msgID int, fields []string) ([]Message, error) {
	dec, ok := inboundDecoders[msgID]
	if !ok {
		return nil, fmt.Errorf("codec: unknown msg_id %d", msgID)
	}
	r := newFieldReader(fields[1:]) // skip msg_id
	msgs, err := dec(r)
	if err == nil && r.Err() != nil {
		return nil, r.Err()
	}
	return msgs, err
}

func encodeFields(msg Message) ([]string, error) {
	switch m := msg.(type) {

	case StartAPI:
		return []string{itoa(OutStartAPI), "2", itoa(m.ClientID), m.OptionalCapabilities}, nil

	case CurrentTimeRequest:
		return []string{itoa(OutReqCurrentTime), "1"}, nil

	case CurrentTimeMillisRequest:
		return []string{itoa(OutReqCurrentTimeInMillis)}, nil

	case CurrentTimeMillis:
		return []string{itoa(InCurrentTimeInMillis), m.TimeMs}, nil

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
		// server_version 200 expects the manual-order-time, customer-account,
		// and professional-customer tail; ending the frame at override drew
		// code 10300 from the live Gateway (capture 20260611T074859Z,
		// sha 241a49023701e9ec).
		w.WriteString("")  // manualOrderTime
		w.WriteString("")  // customerAccount
		w.WriteBool(false) // professionalCustomer
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
		w.WriteString("")      // secType
		w.WriteString("")      // exchange
		w.WriteString("")      // side
		w.WriteInt(2147483647) // lastNDays unset
		w.WriteInt(0)          // specificDates count
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
		// No includeExpired field: the live sv200 Gateway parses optionPrice
		// directly after tradingClass (code 320 evidence, capture
		// 20260611T074859Z, sha 241a49023701e9ec).
		w := fieldWriter{}
		w.WriteInt(OutReqCalcImpliedVolatility)
		w.WriteInt(3) // version
		w.WriteInt(m.ReqID)
		w.WriteInt(m.Contract.ConID)
		writeWireContract(&w, m.Contract)
		w.WriteString(m.OptionPrice)
		w.WriteString(m.UnderPrice)
		w.WriteString("") // implVolOptions
		return w.Fields(), nil

	case CancelCalcImpliedVolatility:
		return []string{itoa(OutCancelCalcImpliedVolatility), "1", itoa(m.ReqID)}, nil

	case CalcOptionPriceRequest:
		// No includeExpired field; see CalcImpliedVolatilityRequest.
		w := fieldWriter{}
		w.WriteInt(OutReqCalcOptionPrice)
		// Official REQ_CALC_OPTION_PRICE version is 2 (3 belongs to the
		// implied-volatility request); the live Gateway tolerated 3 but the
		// official client is the conformance contract.
		w.WriteInt(2) // version
		w.WriteInt(m.ReqID)
		w.WriteInt(m.Contract.ConID)
		writeWireContract(&w, m.Contract)
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
		w.WriteString(m.ExtOperator)
		w.WriteString(m.ManualOrderIndicator)
		return w.Fields(), nil

	case GlobalCancelRequest:
		return []string{itoa(OutReqGlobalCancel), m.ExtOperator, m.ManualOrderIndicator}, nil

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
			m.Contract.Multiplier, "", "", "", "",
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
		w.WriteString("")     // ComboLegsDescrip
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
		w.WriteString("0")          // deltaNeutralContractPresent
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
			m.Symbol, "", "", "", "", "", "", "", "", "",
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
		// followed by the live completed-order v200 field order. Most fields are
		// intentionally empty because public tests only assert the public fields
		// this package currently exposes.
		w := fieldWriter{}
		w.WriteInt(InCompletedOrder)
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
		for range 13 { // lmtPrice through goodAfterTime
			w.WriteString("")
		}
		for range 3 { // FAGroup, FAMethod, FAPercentage
			w.WriteString("")
		}
		for range 5 { // modelCode through settlingFirm
			w.WriteString("")
		}
		for range 3 { // short-sale params
			w.WriteString("")
		}
		for range 3 { // BOX order params
			w.WriteString("")
		}
		for range 2 { // peg-to-stock/vol order params
			w.WriteString("")
		}
		for range 5 { // displaySize through ocaType
			w.WriteString("")
		}
		w.WriteString("") // triggerMethod
		for range 6 {     // vol order params
			w.WriteString("")
		}
		for range 2 { // trailStopPrice, trailingPercent
			w.WriteString("")
		}
		w.WriteString("") // comboLegsDescrip
		w.WriteString("0")
		w.WriteString("0")
		w.WriteString("0")
		for range 6 { // scale params plus table/start/stop
			w.WriteString("")
		}
		w.WriteString("")  // hedgeType
		w.WriteString("")  // optOutSmartRouting
		w.WriteString("")  // clearingAccount
		w.WriteString("")  // clearingIntent
		w.WriteString("")  // notHeld
		w.WriteString("0") // deltaNeutralContract present
		w.WriteString("")  // algoStrategy
		w.WriteString("")  // solicited
		w.WriteString(m.Status)
		for range 2 { // randomizeSize, randomizePrice
			w.WriteString("")
		}
		w.WriteString("0") // conditions count
		for range 2 {      // stop price, limit price offset
			w.WriteString("")
		}
		for range 4 { // cashQty through autoCancelDate
			w.WriteString("")
		}
		w.WriteString(m.Filled)
		for range 7 { // refFuturesConId through completedTime
			w.WriteString("")
		}
		w.WriteString("") // completedStatus
		for range 8 {     // post-completed-status optional fields
			w.WriteString("")
		}
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
			itoa(InTickOptionComputation), itoa(m.ReqID), itoa(m.TickType), itoa(m.TickAttrib),
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

// unsetDoubleSentinel is the wire rendering of the official UNSET_DOUBLE
// (DBL_MAX): an unset numeric slot, not a value.
const unsetDoubleSentinel = "1.7976931348623157E308"

func isPositiveWireNumber(raw string) bool {
	if raw == "" {
		return false
	}
	v, err := strconv.ParseFloat(raw, 64)
	return err == nil && v > 0
}

func isNonNegativeWireNumber(raw string) bool {
	if raw == "" {
		return false
	}
	v, err := strconv.ParseFloat(raw, 64)
	return err == nil && v >= 0
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
		cond.Value = r.ReadString()
		cond.ConID, _ = r.ReadInt()
		cond.Exchange = r.ReadString()
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
		cond.Value = r.ReadString()
		cond.ConID, _ = r.ReadInt()
		cond.Exchange = r.ReadString()
	case 7: // Percent change
		cond.Conjunction = r.ReadString()
		if more, err := r.ReadBool(); err != nil {
			return OrderCondition{}, err
		} else if more {
			cond.Operator = 2
		} else {
			cond.Operator = 1
		}
		cond.Value = r.ReadString()
		cond.ConID, _ = r.ReadInt()
		cond.Exchange = r.ReadString()
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
	// Contract-bound conditions follow the official client's writeExternal
	// hierarchy: the OperatorCondition value precedes the ContractCondition
	// conId/exchange pair. Live Gateway (server_version 200) rejects the
	// reversed order with code 320 field-parse errors.
	isMore := cond.Operator == 2
	switch cond.Type {
	case 1:
		w.WriteBool(isMore)
		w.WriteString(cond.Value)
		w.WriteInt(cond.ConID)
		w.WriteString(cond.Exchange)
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
		w.WriteString(cond.Value)
		w.WriteInt(cond.ConID)
		w.WriteString(cond.Exchange)
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

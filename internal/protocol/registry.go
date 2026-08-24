// Package protocol owns the TWS socket protocol vocabulary and negotiated
// envelope used by ibkr-go. Codecs and audit tooling consume this package; it
// has no transport or public API dependencies.
package protocol

import "slices"

const (
	SupportedMinServerVersion = 208
	SupportedMaxServerVersion = 225
)

// Direction identifies which peer sends a message.
type Direction uint8

const (
	ClientToServer Direction = iota + 1
	ServerToClient
)

func (d Direction) String() string {
	switch d {
	case ClientToServer:
		return "client_to_server"
	case ServerToClient:
		return "server_to_client"
	default:
		return "unknown"
	}
}

// Message identifies one base message independently of its negotiated body
// encoding.
type Message struct {
	Name      string
	ID        int
	Direction Direction
}

// Outbound message IDs (client to server).
const (
	OutReqMktData                  = 1
	OutCancelMktData               = 2
	OutPlaceOrder                  = 3
	OutCancelOrder                 = 4
	OutReqOpenOrders               = 5
	OutReqAccountUpdates           = 6
	OutReqExecutions               = 7
	OutReqIds                      = 8
	OutReqContractData             = 9
	OutReqMktDepth                 = 10
	OutCancelMktDepth              = 11
	OutReqNewsBulletins            = 12
	OutCancelNewsBulletins         = 13
	OutReqAutoOpenOrders           = 15
	OutReqAllOpenOrders            = 16
	OutReqManagedAccounts          = 17
	OutRequestFA                   = 18
	OutReqHistoricalData           = 20
	OutExerciseOptions             = 21
	OutReqScannerSubscription      = 22
	OutCancelScannerSubscription   = 23
	OutReqScannerParameters        = 24
	OutCancelHistoricalData        = 25
	OutReqCurrentTime              = 49
	OutReqRealTimeBars             = 50
	OutCancelRealTimeBars          = 51
	OutReqCalcImpliedVolatility    = 54
	OutReqCalcOptionPrice          = 55
	OutCancelCalcImpliedVolatility = 56
	OutCancelCalcOptionPrice       = 57
	OutReqGlobalCancel             = 58
	OutReqMarketDataType           = 59
	OutReqPositions                = 61
	OutReqAccountSummary           = 62
	OutCancelAccountSummary        = 63
	OutCancelPositions             = 64
	OutQueryDisplayGroups          = 67
	OutSubscribeToGroupEvents      = 68
	OutUpdateDisplayGroup          = 69
	OutUnsubscribeFromGroupEvents  = 70
	OutStartAPI                    = 71
	OutReqPositionsMulti           = 74
	OutCancelPositionsMulti        = 75
	OutReqAccountUpdatesMulti      = 76
	OutCancelAccountUpdatesMulti   = 77
	OutReqSecDefOptParams          = 78
	OutReqSoftDollarTiers          = 79
	OutReqFamilyCodes              = 80
	OutReqMatchingSymbols          = 81
	OutReqMktDepthExchanges        = 82
	OutReqSmartComponents          = 83
	OutReqNewsArticle              = 84
	OutReqNewsProviders            = 85
	OutReqHistoricalNews           = 86
	OutReqHeadTimestamp            = 87
	OutReqHistogramData            = 88
	OutCancelHistogramData         = 89
	OutCancelHeadTimestamp         = 90
	OutReqMarketRule               = 91
	OutReqPnL                      = 92
	OutCancelPnL                   = 93
	OutReqPnLSingle                = 94
	OutCancelPnLSingle             = 95
	OutReqHistoricalTicks          = 96
	OutReqTickByTickData           = 97
	OutCancelTickByTickData        = 98
	OutReqCompletedOrders          = 99
	OutReqWSHMetaData              = 100
	OutCancelWSHMetaData           = 101
	OutReqWSHEventData             = 102
	OutCancelWSHEventData          = 103
	OutReqUserInfo                 = 104
	OutReqCurrentTimeInMillis      = 105
	OutCancelContractData          = 106
	OutCancelHistoricalTicks       = 107
	OutReqConfig                   = 108
)

// Inbound message IDs (server to client).
const (
	InTickPrice              = 1
	InTickSize               = 2
	InOrderStatus            = 3
	InErrMsg                 = 4
	InOpenOrder              = 5
	InUpdateAccountValue     = 6
	InUpdatePortfolio        = 7
	InUpdateAccountTime      = 8
	InNextValidID            = 9
	InContractData           = 10
	InExecutionData          = 11
	InMarketDepth            = 12
	InMarketDepthL2          = 13
	InNewsBulletins          = 14
	InManagedAccounts        = 15
	InReceiveFA              = 16
	InHistoricalData         = 17
	InBondContractData       = 18
	InScannerParameters      = 19
	InScannerData            = 20
	InTickOptionComputation  = 21
	InTickGeneric            = 45
	InTickString             = 46
	InTickEFP                = 47
	InCurrentTime            = 49
	InRealTimeBars           = 50
	InContractDataEnd        = 52
	InOpenOrderEnd           = 53
	InAccountDownloadEnd     = 54
	InExecutionDataEnd       = 55
	InDeltaNeutralValidation = 56
	InTickSnapshotEnd        = 57
	InMarketDataType         = 58
	InCommissionReport       = 59
	InPositionData           = 61
	InPositionEnd            = 62
	InAccountSummary         = 63
	InAccountSummaryEnd      = 64
	InDisplayGroupList       = 67
	InDisplayGroupUpdated    = 68
	InPositionMulti          = 71
	InPositionMultiEnd       = 72
	InAccountUpdateMulti     = 73
	InAccountUpdateMultiEnd  = 74
	InSecDefOptParams        = 75
	InSecDefOptParamsEnd     = 76
	InSoftDollarTiers        = 77
	InFamilyCodes            = 78
	InSymbolSamples          = 79
	InMktDepthExchanges      = 80
	InTickReqParams          = 81
	InSmartComponents        = 82
	InNewsArticle            = 83
	InTickNews               = 84
	InNewsProviders          = 85
	InHistoricalNews         = 86
	InHistoricalNewsEnd      = 87
	InHeadTimestamp          = 88
	InHistogramData          = 89
	InHistoricalDataUpdate   = 90
	InMarketDataReroute      = 91
	InMarketDepthReroute     = 92
	InMarketRule             = 93
	InPnL                    = 94
	InPnLSingle              = 95
	InHistoricalTicks        = 96
	InHistoricalTicksBidAsk  = 97
	InHistoricalTicksLast    = 98
	InTickByTick             = 99
	InOrderBound             = 100
	InCompletedOrder         = 101
	InCompletedOrderEnd      = 102
	InWSHMetaData            = 104
	InWSHEventData           = 105
	InHistoricalSchedule     = 106
	InUserInfo               = 107
	InHistoricalDataEnd      = 108
	InCurrentTimeInMillis    = 109
	InConfig                 = 110
)

var messages = [...]Message{
	{"OutReqMktData", OutReqMktData, ClientToServer},
	{"OutCancelMktData", OutCancelMktData, ClientToServer},
	{"OutPlaceOrder", OutPlaceOrder, ClientToServer},
	{"OutCancelOrder", OutCancelOrder, ClientToServer},
	{"OutReqOpenOrders", OutReqOpenOrders, ClientToServer},
	{"OutReqAccountUpdates", OutReqAccountUpdates, ClientToServer},
	{"OutReqExecutions", OutReqExecutions, ClientToServer},
	{"OutReqIds", OutReqIds, ClientToServer},
	{"OutReqContractData", OutReqContractData, ClientToServer},
	{"OutReqMktDepth", OutReqMktDepth, ClientToServer},
	{"OutCancelMktDepth", OutCancelMktDepth, ClientToServer},
	{"OutReqNewsBulletins", OutReqNewsBulletins, ClientToServer},
	{"OutCancelNewsBulletins", OutCancelNewsBulletins, ClientToServer},
	{"OutReqAutoOpenOrders", OutReqAutoOpenOrders, ClientToServer},
	{"OutReqAllOpenOrders", OutReqAllOpenOrders, ClientToServer},
	{"OutReqManagedAccounts", OutReqManagedAccounts, ClientToServer},
	{"OutRequestFA", OutRequestFA, ClientToServer},
	{"OutReqHistoricalData", OutReqHistoricalData, ClientToServer},
	{"OutExerciseOptions", OutExerciseOptions, ClientToServer},
	{"OutReqScannerSubscription", OutReqScannerSubscription, ClientToServer},
	{"OutCancelScannerSubscription", OutCancelScannerSubscription, ClientToServer},
	{"OutReqScannerParameters", OutReqScannerParameters, ClientToServer},
	{"OutCancelHistoricalData", OutCancelHistoricalData, ClientToServer},
	{"OutReqCurrentTime", OutReqCurrentTime, ClientToServer},
	{"OutReqRealTimeBars", OutReqRealTimeBars, ClientToServer},
	{"OutCancelRealTimeBars", OutCancelRealTimeBars, ClientToServer},
	{"OutReqCalcImpliedVolatility", OutReqCalcImpliedVolatility, ClientToServer},
	{"OutReqCalcOptionPrice", OutReqCalcOptionPrice, ClientToServer},
	{"OutCancelCalcImpliedVolatility", OutCancelCalcImpliedVolatility, ClientToServer},
	{"OutCancelCalcOptionPrice", OutCancelCalcOptionPrice, ClientToServer},
	{"OutReqGlobalCancel", OutReqGlobalCancel, ClientToServer},
	{"OutReqMarketDataType", OutReqMarketDataType, ClientToServer},
	{"OutReqPositions", OutReqPositions, ClientToServer},
	{"OutReqAccountSummary", OutReqAccountSummary, ClientToServer},
	{"OutCancelAccountSummary", OutCancelAccountSummary, ClientToServer},
	{"OutCancelPositions", OutCancelPositions, ClientToServer},
	{"OutQueryDisplayGroups", OutQueryDisplayGroups, ClientToServer},
	{"OutSubscribeToGroupEvents", OutSubscribeToGroupEvents, ClientToServer},
	{"OutUpdateDisplayGroup", OutUpdateDisplayGroup, ClientToServer},
	{"OutUnsubscribeFromGroupEvents", OutUnsubscribeFromGroupEvents, ClientToServer},
	{"OutStartAPI", OutStartAPI, ClientToServer},
	{"OutReqPositionsMulti", OutReqPositionsMulti, ClientToServer},
	{"OutCancelPositionsMulti", OutCancelPositionsMulti, ClientToServer},
	{"OutReqAccountUpdatesMulti", OutReqAccountUpdatesMulti, ClientToServer},
	{"OutCancelAccountUpdatesMulti", OutCancelAccountUpdatesMulti, ClientToServer},
	{"OutReqSecDefOptParams", OutReqSecDefOptParams, ClientToServer},
	{"OutReqSoftDollarTiers", OutReqSoftDollarTiers, ClientToServer},
	{"OutReqFamilyCodes", OutReqFamilyCodes, ClientToServer},
	{"OutReqMatchingSymbols", OutReqMatchingSymbols, ClientToServer},
	{"OutReqMktDepthExchanges", OutReqMktDepthExchanges, ClientToServer},
	{"OutReqSmartComponents", OutReqSmartComponents, ClientToServer},
	{"OutReqNewsArticle", OutReqNewsArticle, ClientToServer},
	{"OutReqNewsProviders", OutReqNewsProviders, ClientToServer},
	{"OutReqHistoricalNews", OutReqHistoricalNews, ClientToServer},
	{"OutReqHeadTimestamp", OutReqHeadTimestamp, ClientToServer},
	{"OutReqHistogramData", OutReqHistogramData, ClientToServer},
	{"OutCancelHistogramData", OutCancelHistogramData, ClientToServer},
	{"OutCancelHeadTimestamp", OutCancelHeadTimestamp, ClientToServer},
	{"OutReqMarketRule", OutReqMarketRule, ClientToServer},
	{"OutReqPnL", OutReqPnL, ClientToServer},
	{"OutCancelPnL", OutCancelPnL, ClientToServer},
	{"OutReqPnLSingle", OutReqPnLSingle, ClientToServer},
	{"OutCancelPnLSingle", OutCancelPnLSingle, ClientToServer},
	{"OutReqHistoricalTicks", OutReqHistoricalTicks, ClientToServer},
	{"OutReqTickByTickData", OutReqTickByTickData, ClientToServer},
	{"OutCancelTickByTickData", OutCancelTickByTickData, ClientToServer},
	{"OutReqCompletedOrders", OutReqCompletedOrders, ClientToServer},
	{"OutReqWSHMetaData", OutReqWSHMetaData, ClientToServer},
	{"OutCancelWSHMetaData", OutCancelWSHMetaData, ClientToServer},
	{"OutReqWSHEventData", OutReqWSHEventData, ClientToServer},
	{"OutCancelWSHEventData", OutCancelWSHEventData, ClientToServer},
	{"OutReqUserInfo", OutReqUserInfo, ClientToServer},
	{"OutReqCurrentTimeInMillis", OutReqCurrentTimeInMillis, ClientToServer},
	{"OutCancelContractData", OutCancelContractData, ClientToServer},
	{"OutCancelHistoricalTicks", OutCancelHistoricalTicks, ClientToServer},
	{"OutReqConfig", OutReqConfig, ClientToServer},
	{"InTickPrice", InTickPrice, ServerToClient},
	{"InTickSize", InTickSize, ServerToClient},
	{"InOrderStatus", InOrderStatus, ServerToClient},
	{"InErrMsg", InErrMsg, ServerToClient},
	{"InOpenOrder", InOpenOrder, ServerToClient},
	{"InUpdateAccountValue", InUpdateAccountValue, ServerToClient},
	{"InUpdatePortfolio", InUpdatePortfolio, ServerToClient},
	{"InUpdateAccountTime", InUpdateAccountTime, ServerToClient},
	{"InNextValidID", InNextValidID, ServerToClient},
	{"InContractData", InContractData, ServerToClient},
	{"InExecutionData", InExecutionData, ServerToClient},
	{"InMarketDepth", InMarketDepth, ServerToClient},
	{"InMarketDepthL2", InMarketDepthL2, ServerToClient},
	{"InNewsBulletins", InNewsBulletins, ServerToClient},
	{"InManagedAccounts", InManagedAccounts, ServerToClient},
	{"InReceiveFA", InReceiveFA, ServerToClient},
	{"InHistoricalData", InHistoricalData, ServerToClient},
	{"InBondContractData", InBondContractData, ServerToClient},
	{"InScannerParameters", InScannerParameters, ServerToClient},
	{"InScannerData", InScannerData, ServerToClient},
	{"InTickOptionComputation", InTickOptionComputation, ServerToClient},
	{"InTickGeneric", InTickGeneric, ServerToClient},
	{"InTickString", InTickString, ServerToClient},
	{"InTickEFP", InTickEFP, ServerToClient},
	{"InCurrentTime", InCurrentTime, ServerToClient},
	{"InRealTimeBars", InRealTimeBars, ServerToClient},
	{"InContractDataEnd", InContractDataEnd, ServerToClient},
	{"InOpenOrderEnd", InOpenOrderEnd, ServerToClient},
	{"InAccountDownloadEnd", InAccountDownloadEnd, ServerToClient},
	{"InExecutionDataEnd", InExecutionDataEnd, ServerToClient},
	{"InDeltaNeutralValidation", InDeltaNeutralValidation, ServerToClient},
	{"InTickSnapshotEnd", InTickSnapshotEnd, ServerToClient},
	{"InMarketDataType", InMarketDataType, ServerToClient},
	{"InCommissionReport", InCommissionReport, ServerToClient},
	{"InPositionData", InPositionData, ServerToClient},
	{"InPositionEnd", InPositionEnd, ServerToClient},
	{"InAccountSummary", InAccountSummary, ServerToClient},
	{"InAccountSummaryEnd", InAccountSummaryEnd, ServerToClient},
	{"InDisplayGroupList", InDisplayGroupList, ServerToClient},
	{"InDisplayGroupUpdated", InDisplayGroupUpdated, ServerToClient},
	{"InPositionMulti", InPositionMulti, ServerToClient},
	{"InPositionMultiEnd", InPositionMultiEnd, ServerToClient},
	{"InAccountUpdateMulti", InAccountUpdateMulti, ServerToClient},
	{"InAccountUpdateMultiEnd", InAccountUpdateMultiEnd, ServerToClient},
	{"InSecDefOptParams", InSecDefOptParams, ServerToClient},
	{"InSecDefOptParamsEnd", InSecDefOptParamsEnd, ServerToClient},
	{"InSoftDollarTiers", InSoftDollarTiers, ServerToClient},
	{"InFamilyCodes", InFamilyCodes, ServerToClient},
	{"InSymbolSamples", InSymbolSamples, ServerToClient},
	{"InMktDepthExchanges", InMktDepthExchanges, ServerToClient},
	{"InTickReqParams", InTickReqParams, ServerToClient},
	{"InSmartComponents", InSmartComponents, ServerToClient},
	{"InNewsArticle", InNewsArticle, ServerToClient},
	{"InTickNews", InTickNews, ServerToClient},
	{"InNewsProviders", InNewsProviders, ServerToClient},
	{"InHistoricalNews", InHistoricalNews, ServerToClient},
	{"InHistoricalNewsEnd", InHistoricalNewsEnd, ServerToClient},
	{"InHeadTimestamp", InHeadTimestamp, ServerToClient},
	{"InHistogramData", InHistogramData, ServerToClient},
	{"InHistoricalDataUpdate", InHistoricalDataUpdate, ServerToClient},
	{"InMarketDataReroute", InMarketDataReroute, ServerToClient},
	{"InMarketDepthReroute", InMarketDepthReroute, ServerToClient},
	{"InMarketRule", InMarketRule, ServerToClient},
	{"InPnL", InPnL, ServerToClient},
	{"InPnLSingle", InPnLSingle, ServerToClient},
	{"InHistoricalTicks", InHistoricalTicks, ServerToClient},
	{"InHistoricalTicksBidAsk", InHistoricalTicksBidAsk, ServerToClient},
	{"InHistoricalTicksLast", InHistoricalTicksLast, ServerToClient},
	{"InTickByTick", InTickByTick, ServerToClient},
	{"InOrderBound", InOrderBound, ServerToClient},
	{"InCompletedOrder", InCompletedOrder, ServerToClient},
	{"InCompletedOrderEnd", InCompletedOrderEnd, ServerToClient},
	{"InWSHMetaData", InWSHMetaData, ServerToClient},
	{"InWSHEventData", InWSHEventData, ServerToClient},
	{"InHistoricalSchedule", InHistoricalSchedule, ServerToClient},
	{"InUserInfo", InUserInfo, ServerToClient},
	{"InHistoricalDataEnd", InHistoricalDataEnd, ServerToClient},
	{"InCurrentTimeInMillis", InCurrentTimeInMillis, ServerToClient},
	{"InConfig", InConfig, ServerToClient},
}

// Messages returns the implemented base-message registry in ID-independent
// declaration order. The returned slice can be mutated by the caller.
func Messages() []Message {
	return slices.Clone(messages[:])
}

// Lookup finds an implemented base message by direction and numeric ID.
func Lookup(direction Direction, id int) (Message, bool) {
	for _, message := range messages {
		if message.Direction == direction && message.ID == id {
			return message, true
		}
	}
	return Message{}, false
}

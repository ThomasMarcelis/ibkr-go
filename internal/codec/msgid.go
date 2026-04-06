package codec

// Outbound message IDs (client → server)
const (
	OutReqMktData           = 1
	OutCancelMktData        = 2
	OutReqOpenOrders        = 5
	OutReqExecutions        = 7
	OutReqContractData      = 9
	OutReqAutoOpenOrders    = 15
	OutReqAllOpenOrders     = 16
	OutReqHistoricalData    = 20
	OutCancelHistoricalData = 25
	OutReqRealTimeBars      = 50
	OutCancelRealTimeBars   = 51
	OutReqMarketDataType    = 59
	OutReqPositions         = 61
	OutReqAccountSummary    = 62
	OutCancelAccountSummary = 63
	OutCancelPositions      = 64
	OutStartAPI             = 71
)

// Inbound message IDs (server → client)
const (
	InTickPrice         = 1
	InTickSize          = 2
	InOrderStatus       = 3
	InErrMsg            = 4
	InOpenOrder         = 5
	InNextValidID       = 9
	InContractData      = 10
	InExecutionData     = 11
	InManagedAccounts   = 15
	InHistoricalData    = 17
	InTickGeneric       = 45
	InTickString        = 46
	InCurrentTime       = 49
	InRealTimeBars      = 50
	InContractDataEnd   = 52
	InOpenOrderEnd      = 53
	InExecutionDataEnd  = 55
	InTickSnapshotEnd   = 57
	InMarketDataType    = 58
	InCommissionReport  = 59
	InPositionData      = 61
	InPositionEnd       = 62
	InAccountSummary    = 63
	InAccountSummaryEnd = 64
)

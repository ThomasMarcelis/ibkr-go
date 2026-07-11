package codec

import "github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"

// inboundProtobufDecoders is deliberately explicit. A protobuf envelope with
// an unknown base ID remains observable as UnknownInbound; it is never fed to
// a classic field decoder.
var inboundProtobufDecoders = map[int]protobufDecodeFunc{
	protocol.InTickPrice:             decodeTickPriceProto,
	protocol.InTickSize:              decodeTickSizeProto,
	protocol.InUpdateAccountValue:    decodeUpdateAccountValueProto,
	protocol.InUpdatePortfolio:       decodeUpdatePortfolioProto,
	protocol.InUpdateAccountTime:     decodeUpdateAccountTimeProto,
	protocol.InMarketDepth:           decodeMarketDepthProto,
	protocol.InMarketDepthL2:         decodeMarketDepthL2Proto,
	protocol.InManagedAccounts:       decodeManagedAccountsProto,
	protocol.InTickOptionComputation: decodeTickOptionComputationProto,
	protocol.InTickGeneric:           decodeTickGenericProto,
	protocol.InTickString:            decodeTickStringProto,
	protocol.InOpenOrderEnd:          decodeOpenOrdersEndProto,
	protocol.InAccountDownloadEnd:    decodeAccountDownloadEndProto,
	protocol.InExecutionDataEnd:      decodeExecutionDetailsEndProto,
	protocol.InTickSnapshotEnd:       decodeTickSnapshotEndProto,
	protocol.InMarketDataType:        decodeMarketDataTypeProto,
	protocol.InCommissionReport:      decodeCommissionAndFeesReportProto,
	protocol.InPositionData:          decodePositionProto,
	protocol.InPositionEnd:           decodePositionEndProto,
	protocol.InAccountSummary:        decodeAccountSummaryProto,
	protocol.InAccountSummaryEnd:     decodeAccountSummaryEndProto,
	protocol.InTickReqParams:         decodeTickReqParamsProto,
	protocol.InPositionMulti:         decodePositionMultiProto,
	protocol.InPositionMultiEnd:      decodePositionMultiEndProto,
	protocol.InAccountUpdateMulti:    decodeAccountUpdateMultiProto,
	protocol.InAccountUpdateMultiEnd: decodeAccountUpdateMultiEndProto,
	protocol.InCompletedOrder:        decodeCompletedOrderProto,
	protocol.InOrderBound:            decodeOrderBoundProto,
	protocol.InCompletedOrderEnd:     decodeCompletedOrdersEndProto,
	protocol.InOrderStatus:           decodeOrderStatusProto,
	protocol.InOpenOrder:             decodeOpenOrderProto,
	protocol.InExecutionData:         decodeExecutionDetailsProto,
	protocol.InContractData:          decodeContractDataProto,
	protocol.InBondContractData:      decodeBondContractDataProto,
	protocol.InContractDataEnd:       decodeContractDataEndProto,
	protocol.InErrMsg:                decodeErrorProto,
}

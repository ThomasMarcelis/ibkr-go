package protocol

// outboundProtobufVersion is the first server version at which an outbound
// classic message ID is replaced by ID+200 with a protobuf body. It mirrors
// GetServerVersionForMessage in the official API 10.48.01 EClient.h. The four
// messages introduced after the staged migration are protobuf-only.
var outboundProtobufVersion = map[int]int{
	OutReqExecutions:             MinServerVersionProtobuf,
	OutPlaceOrder:                MinServerVersionProtobufPlaceOrder,
	OutCancelOrder:               MinServerVersionProtobufPlaceOrder,
	OutReqGlobalCancel:           MinServerVersionProtobufPlaceOrder,
	OutReqOpenOrders:             MinServerVersionProtobufCompletedOrder,
	OutReqAutoOpenOrders:         MinServerVersionProtobufCompletedOrder,
	OutReqAllOpenOrders:          MinServerVersionProtobufCompletedOrder,
	OutReqCompletedOrders:        MinServerVersionProtobufCompletedOrder,
	OutReqContractData:           MinServerVersionProtobufContractData,
	OutReqMktData:                MinServerVersionProtobufMarketData,
	OutCancelMktData:             MinServerVersionProtobufMarketData,
	OutReqMktDepth:               MinServerVersionProtobufMarketData,
	OutCancelMktDepth:            MinServerVersionProtobufMarketData,
	OutReqMarketDataType:         MinServerVersionProtobufMarketData,
	OutReqAccountUpdates:         MinServerVersionProtobufAccountsPositions,
	OutReqManagedAccounts:        MinServerVersionProtobufAccountsPositions,
	OutReqPositions:              MinServerVersionProtobufAccountsPositions,
	OutCancelPositions:           MinServerVersionProtobufAccountsPositions,
	OutReqAccountSummary:         MinServerVersionProtobufAccountsPositions,
	OutCancelAccountSummary:      MinServerVersionProtobufAccountsPositions,
	OutReqPositionsMulti:         MinServerVersionProtobufAccountsPositions,
	OutCancelPositionsMulti:      MinServerVersionProtobufAccountsPositions,
	OutReqAccountUpdatesMulti:    MinServerVersionProtobufAccountsPositions,
	OutCancelAccountUpdatesMulti: MinServerVersionProtobufAccountsPositions,
}

// OutboundProtobufVersion reports the first negotiated server version that
// requires a protobuf body for the outbound base message ID.
func OutboundProtobufVersion(msgID int) (int, bool) {
	version, ok := outboundProtobufVersion[msgID]
	return version, ok
}

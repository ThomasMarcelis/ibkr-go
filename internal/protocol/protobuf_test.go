package protocol

import "testing"

func TestOutboundProtobufMigrationGates(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		msgID int
		want  int
	}{
		{OutReqExecutions, 208},
		{OutPlaceOrder, 208},
		{OutReqOpenOrders, 208},
		{OutReqAutoOpenOrders, 208},
		{OutReqAllOpenOrders, 208},
		{OutReqCompletedOrders, 208},
		{OutReqContractData, 208},
		{OutReqAccountSummary, 208},
		{OutReqHistoricalData, 208},
		{OutCancelHistoricalData, 208},
		{OutReqTickByTickData, 208},
		{OutReqNewsProviders, 209},
		{OutReqHistoricalNews, 209},
		{OutReqScannerSubscription, 210},
		{OutReqPnL, 210},
		{OutRequestFA, 211},
		{OutReqCalcOptionPrice, 211},
		{OutReqSecDefOptParams, 212},
		{OutReqSoftDollarTiers, 212},
		{OutReqFamilyCodes, 212},
		{OutReqMatchingSymbols, 212},
		{OutReqSmartComponents, 212},
		{OutReqMarketRule, 212},
		{OutReqUserInfo, 212},
		{OutReqIds, 213},
		{OutReqCurrentTime, 213},
		{OutReqCurrentTimeInMillis, 213},
		{OutStartAPI, 213},
		{OutQueryDisplayGroups, 213},
		{OutSubscribeToGroupEvents, 213},
		{OutUpdateDisplayGroup, 213},
		{OutUnsubscribeFromGroupEvents, 213},
		{OutReqMktDepthExchanges, 213},
		{OutCancelContractData, 215},
		{OutCancelHistoricalTicks, 215},
		{OutReqConfig, 219},
	} {
		got, ok := OutboundProtobufVersion(tc.msgID)
		if !ok || got != tc.want {
			t.Fatalf("OutboundProtobufVersion(%d) = (%d, %t), want (%d, true)", tc.msgID, got, ok, tc.want)
		}
	}
	if _, ok := OutboundProtobufVersion(10_000); ok {
		t.Fatal("unknown outbound ID has a protobuf migration gate")
	}
}

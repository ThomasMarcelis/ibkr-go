package codec

import (
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
)

func TestOutboundMessageIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  OutboundMessage
		want int
	}{
		{"open all", OpenOrdersRequest{Scope: "all"}, protocol.OutReqAllOpenOrders},
		{"open client", OpenOrdersRequest{Scope: "client"}, protocol.OutReqOpenOrders},
		{"open auto", OpenOrdersRequest{Scope: "auto"}, protocol.OutReqAutoOpenOrders},
		{"cancel open", CancelOpenOrders{}, protocol.OutReqAutoOpenOrders},
		{"executions", ExecutionsRequest{}, protocol.OutReqExecutions},
		{"completed orders", CompletedOrdersRequest{}, protocol.OutReqCompletedOrders},
		{"soft dollar tiers", SoftDollarTiersRequest{}, protocol.OutReqSoftDollarTiers},
		{"place order", PlaceOrderRequest{}, protocol.OutPlaceOrder},
		{"cancel order", CancelOrderRequest{}, protocol.OutCancelOrder},
		{"global cancel", GlobalCancelRequest{}, protocol.OutReqGlobalCancel},
		{"exercise options", ExerciseOptionsRequest{}, protocol.OutExerciseOptions},
		{"historical bars", HistoricalBarsRequest{}, protocol.OutReqHistoricalData},
		{"cancel historical", CancelHistoricalData{}, protocol.OutCancelHistoricalData},
		{"head timestamp", HeadTimestampRequest{}, protocol.OutReqHeadTimestamp},
		{"cancel head timestamp", CancelHeadTimestamp{}, protocol.OutCancelHeadTimestamp},
		{"histogram", HistogramDataRequest{}, protocol.OutReqHistogramData},
		{"cancel histogram", CancelHistogramData{}, protocol.OutCancelHistogramData},
		{"historical ticks", HistoricalTicksRequest{}, protocol.OutReqHistoricalTicks},
		{"contract details", ContractDetailsRequest{}, protocol.OutReqContractData},
		{"matching symbols", MatchingSymbolsRequest{}, protocol.OutReqMatchingSymbols},
		{"market rule", MarketRuleRequest{}, protocol.OutReqMarketRule},
		{"option parameters", SecDefOptParamsRequest{}, protocol.OutReqSecDefOptParams},
		{"smart components", SmartComponentsRequest{}, protocol.OutReqSmartComponents},
		{"config", ConfigRequest{}, protocol.OutReqConfig},
		{"cancel contract details", CancelContractData{}, protocol.OutCancelContractData},
		{"cancel historical ticks", CancelHistoricalTicks{}, protocol.OutCancelHistoricalTicks},
		{"start API", StartAPI{}, protocol.OutStartAPI},
		{"managed accounts", ManagedAccountsRequest{}, protocol.OutReqManagedAccounts},
		{"current time", CurrentTimeRequest{}, protocol.OutReqCurrentTime},
		{"current time millis", CurrentTimeMillisRequest{}, protocol.OutReqCurrentTimeInMillis},
		{"request IDs", ReqIDsRequest{}, protocol.OutReqIds},
		{"user info", UserInfoRequest{}, protocol.OutReqUserInfo},
		{"account summary", AccountSummaryRequest{}, protocol.OutReqAccountSummary},
		{"cancel account summary", CancelAccountSummary{}, protocol.OutCancelAccountSummary},
		{"positions", PositionsRequest{}, protocol.OutReqPositions},
		{"cancel positions", CancelPositions{}, protocol.OutCancelPositions},
		{"family codes", FamilyCodesRequest{}, protocol.OutReqFamilyCodes},
		{"account updates", AccountUpdatesRequest{}, protocol.OutReqAccountUpdates},
		{"account updates multi", AccountUpdatesMultiRequest{}, protocol.OutReqAccountUpdatesMulti},
		{"cancel account updates multi", CancelAccountUpdatesMulti{}, protocol.OutCancelAccountUpdatesMulti},
		{"positions multi", PositionsMultiRequest{}, protocol.OutReqPositionsMulti},
		{"cancel positions multi", CancelPositionsMulti{}, protocol.OutCancelPositionsMulti},
		{"PnL", PnLRequest{}, protocol.OutReqPnL},
		{"cancel PnL", CancelPnL{}, protocol.OutCancelPnL},
		{"single PnL", PnLSingleRequest{}, protocol.OutReqPnLSingle},
		{"cancel single PnL", CancelPnLSingle{}, protocol.OutCancelPnLSingle},
		{"quote", QuoteRequest{}, protocol.OutReqMktData},
		{"cancel quote", CancelQuote{}, protocol.OutCancelMktData},
		{"real-time bars", RealTimeBarsRequest{}, protocol.OutReqRealTimeBars},
		{"cancel real-time bars", CancelRealTimeBars{}, protocol.OutCancelRealTimeBars},
		{"market data type", ReqMarketDataType{}, protocol.OutReqMarketDataType},
		{"depth exchanges", MktDepthExchangesRequest{}, protocol.OutReqMktDepthExchanges},
		{"tick by tick", TickByTickRequest{}, protocol.OutReqTickByTickData},
		{"cancel tick by tick", CancelTickByTick{}, protocol.OutCancelTickByTickData},
		{"implied volatility", CalcImpliedVolatilityRequest{}, protocol.OutReqCalcImpliedVolatility},
		{"cancel implied volatility", CancelCalcImpliedVolatility{}, protocol.OutCancelCalcImpliedVolatility},
		{"option price", CalcOptionPriceRequest{}, protocol.OutReqCalcOptionPrice},
		{"cancel option price", CancelCalcOptionPrice{}, protocol.OutCancelCalcOptionPrice},
		{"market depth", MarketDepthRequest{}, protocol.OutReqMktDepth},
		{"cancel market depth", CancelMarketDepth{}, protocol.OutCancelMktDepth},
		{"scanner parameters", ScannerParametersRequest{}, protocol.OutReqScannerParameters},
		{"scanner subscription", ScannerSubscriptionRequest{}, protocol.OutReqScannerSubscription},
		{"cancel scanner", CancelScannerSubscription{}, protocol.OutCancelScannerSubscription},
		{"FA", RequestFA{}, protocol.OutRequestFA},
		{"WSH metadata", WSHMetaDataRequest{}, protocol.OutReqWSHMetaData},
		{"cancel WSH metadata", CancelWSHMetaData{}, protocol.OutCancelWSHMetaData},
		{"WSH event data", WSHEventDataRequest{}, protocol.OutReqWSHEventData},
		{"cancel WSH event data", CancelWSHEventData{}, protocol.OutCancelWSHEventData},
		{"query display groups", QueryDisplayGroupsRequest{}, protocol.OutQueryDisplayGroups},
		{"subscribe display group", SubscribeToGroupEventsRequest{}, protocol.OutSubscribeToGroupEvents},
		{"update display group", UpdateDisplayGroupRequest{}, protocol.OutUpdateDisplayGroup},
		{"unsubscribe display group", UnsubscribeFromGroupEventsRequest{}, protocol.OutUnsubscribeFromGroupEvents},
		{"news providers", NewsProvidersRequest{}, protocol.OutReqNewsProviders},
		{"news bulletins", NewsBulletinsRequest{}, protocol.OutReqNewsBulletins},
		{"cancel news bulletins", CancelNewsBulletins{}, protocol.OutCancelNewsBulletins},
		{"news article", NewsArticleRequest{}, protocol.OutReqNewsArticle},
		{"historical news", HistoricalNewsRequest{}, protocol.OutReqHistoricalNews},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.msg.messageID(); got != tc.want {
				t.Fatalf("messageID() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestSupportedFloorProtobufMessagesHaveNoClassicEncoder(t *testing.T) {
	t.Parallel()

	for _, msg := range []OutboundMessage{
		OpenOrdersRequest{}, CancelOpenOrders{}, ExecutionsRequest{}, CompletedOrdersRequest{},
		PlaceOrderRequest{}, CancelOrderRequest{}, GlobalCancelRequest{}, ContractDetailsRequest{},
		HistoricalBarsRequest{}, CancelHistoricalData{}, HeadTimestampRequest{}, CancelHeadTimestamp{},
		HistogramDataRequest{}, CancelHistogramData{}, HistoricalTicksRequest{},
		AccountSummaryRequest{}, CancelAccountSummary{}, PositionsRequest{}, CancelPositions{},
		AccountUpdatesRequest{}, AccountUpdatesMultiRequest{}, CancelAccountUpdatesMulti{},
		PositionsMultiRequest{}, CancelPositionsMulti{}, ManagedAccountsRequest{},
		QuoteRequest{}, CancelQuote{}, RealTimeBarsRequest{}, CancelRealTimeBars{},
		ReqMarketDataType{}, TickByTickRequest{}, CancelTickByTick{},
		MarketDepthRequest{}, CancelMarketDepth{},
	} {
		if _, ok := msg.(classicEncoder); ok {
			t.Errorf("%T retains an unreachable classic encoder at server_version 208", msg)
		}
	}
}

package codec

import "github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"

// messageID is deliberately explicit. It lets Encode select the negotiated
// body format before touching a legacy encoder, so protobuf-only messages at
// the supported floor cannot execute an obsolete classic path.
func (m OpenOrdersRequest) messageID() int {
	switch m.Scope {
	case "client":
		return protocol.OutReqOpenOrders
	case "auto":
		return protocol.OutReqAutoOpenOrders
	default:
		return protocol.OutReqAllOpenOrders
	}
}

func (CancelOpenOrders) messageID() int              { return protocol.OutReqAutoOpenOrders }
func (ExecutionsRequest) messageID() int             { return protocol.OutReqExecutions }
func (CompletedOrdersRequest) messageID() int        { return protocol.OutReqCompletedOrders }
func (SoftDollarTiersRequest) messageID() int        { return protocol.OutReqSoftDollarTiers }
func (PlaceOrderRequest) messageID() int             { return protocol.OutPlaceOrder }
func (CancelOrderRequest) messageID() int            { return protocol.OutCancelOrder }
func (GlobalCancelRequest) messageID() int           { return protocol.OutReqGlobalCancel }
func (ExerciseOptionsRequest) messageID() int        { return protocol.OutExerciseOptions }
func (HistoricalBarsRequest) messageID() int         { return protocol.OutReqHistoricalData }
func (CancelHistoricalData) messageID() int          { return protocol.OutCancelHistoricalData }
func (HeadTimestampRequest) messageID() int          { return protocol.OutReqHeadTimestamp }
func (CancelHeadTimestamp) messageID() int           { return protocol.OutCancelHeadTimestamp }
func (HistogramDataRequest) messageID() int          { return protocol.OutReqHistogramData }
func (CancelHistogramData) messageID() int           { return protocol.OutCancelHistogramData }
func (HistoricalTicksRequest) messageID() int        { return protocol.OutReqHistoricalTicks }
func (ContractDetailsRequest) messageID() int        { return protocol.OutReqContractData }
func (MatchingSymbolsRequest) messageID() int        { return protocol.OutReqMatchingSymbols }
func (MarketRuleRequest) messageID() int             { return protocol.OutReqMarketRule }
func (SecDefOptParamsRequest) messageID() int        { return protocol.OutReqSecDefOptParams }
func (SmartComponentsRequest) messageID() int        { return protocol.OutReqSmartComponents }
func (ConfigRequest) messageID() int                 { return protocol.OutReqConfig }
func (CancelContractData) messageID() int            { return protocol.OutCancelContractData }
func (CancelHistoricalTicks) messageID() int         { return protocol.OutCancelHistoricalTicks }
func (StartAPI) messageID() int                      { return protocol.OutStartAPI }
func (ManagedAccountsRequest) messageID() int        { return protocol.OutReqManagedAccounts }
func (CurrentTimeRequest) messageID() int            { return protocol.OutReqCurrentTime }
func (CurrentTimeMillisRequest) messageID() int      { return protocol.OutReqCurrentTimeInMillis }
func (ReqIDsRequest) messageID() int                 { return protocol.OutReqIds }
func (UserInfoRequest) messageID() int               { return protocol.OutReqUserInfo }
func (AccountSummaryRequest) messageID() int         { return protocol.OutReqAccountSummary }
func (CancelAccountSummary) messageID() int          { return protocol.OutCancelAccountSummary }
func (PositionsRequest) messageID() int              { return protocol.OutReqPositions }
func (CancelPositions) messageID() int               { return protocol.OutCancelPositions }
func (FamilyCodesRequest) messageID() int            { return protocol.OutReqFamilyCodes }
func (AccountUpdatesRequest) messageID() int         { return protocol.OutReqAccountUpdates }
func (AccountUpdatesMultiRequest) messageID() int    { return protocol.OutReqAccountUpdatesMulti }
func (CancelAccountUpdatesMulti) messageID() int     { return protocol.OutCancelAccountUpdatesMulti }
func (PositionsMultiRequest) messageID() int         { return protocol.OutReqPositionsMulti }
func (CancelPositionsMulti) messageID() int          { return protocol.OutCancelPositionsMulti }
func (PnLRequest) messageID() int                    { return protocol.OutReqPnL }
func (CancelPnL) messageID() int                     { return protocol.OutCancelPnL }
func (PnLSingleRequest) messageID() int              { return protocol.OutReqPnLSingle }
func (CancelPnLSingle) messageID() int               { return protocol.OutCancelPnLSingle }
func (QuoteRequest) messageID() int                  { return protocol.OutReqMktData }
func (CancelQuote) messageID() int                   { return protocol.OutCancelMktData }
func (RealTimeBarsRequest) messageID() int           { return protocol.OutReqRealTimeBars }
func (CancelRealTimeBars) messageID() int            { return protocol.OutCancelRealTimeBars }
func (ReqMarketDataType) messageID() int             { return protocol.OutReqMarketDataType }
func (MktDepthExchangesRequest) messageID() int      { return protocol.OutReqMktDepthExchanges }
func (TickByTickRequest) messageID() int             { return protocol.OutReqTickByTickData }
func (CancelTickByTick) messageID() int              { return protocol.OutCancelTickByTickData }
func (CalcImpliedVolatilityRequest) messageID() int  { return protocol.OutReqCalcImpliedVolatility }
func (CancelCalcImpliedVolatility) messageID() int   { return protocol.OutCancelCalcImpliedVolatility }
func (CalcOptionPriceRequest) messageID() int        { return protocol.OutReqCalcOptionPrice }
func (CancelCalcOptionPrice) messageID() int         { return protocol.OutCancelCalcOptionPrice }
func (MarketDepthRequest) messageID() int            { return protocol.OutReqMktDepth }
func (CancelMarketDepth) messageID() int             { return protocol.OutCancelMktDepth }
func (ScannerParametersRequest) messageID() int      { return protocol.OutReqScannerParameters }
func (ScannerSubscriptionRequest) messageID() int    { return protocol.OutReqScannerSubscription }
func (CancelScannerSubscription) messageID() int     { return protocol.OutCancelScannerSubscription }
func (RequestFA) messageID() int                     { return protocol.OutRequestFA }
func (WSHMetaDataRequest) messageID() int            { return protocol.OutReqWSHMetaData }
func (CancelWSHMetaData) messageID() int             { return protocol.OutCancelWSHMetaData }
func (WSHEventDataRequest) messageID() int           { return protocol.OutReqWSHEventData }
func (CancelWSHEventData) messageID() int            { return protocol.OutCancelWSHEventData }
func (QueryDisplayGroupsRequest) messageID() int     { return protocol.OutQueryDisplayGroups }
func (SubscribeToGroupEventsRequest) messageID() int { return protocol.OutSubscribeToGroupEvents }
func (UpdateDisplayGroupRequest) messageID() int     { return protocol.OutUpdateDisplayGroup }
func (UnsubscribeFromGroupEventsRequest) messageID() int {
	return protocol.OutUnsubscribeFromGroupEvents
}
func (NewsProvidersRequest) messageID() int  { return protocol.OutReqNewsProviders }
func (NewsBulletinsRequest) messageID() int  { return protocol.OutReqNewsBulletins }
func (CancelNewsBulletins) messageID() int   { return protocol.OutCancelNewsBulletins }
func (NewsArticleRequest) messageID() int    { return protocol.OutReqNewsArticle }
func (HistoricalNewsRequest) messageID() int { return protocol.OutReqHistoricalNews }

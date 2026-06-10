package ibkr

// IBKR error and message codes attested in this repository's live-derived
// captures against IB Gateway server_version 200. Meanings follow the
// official TWS API message-code tables. Only attested codes are registered;
// the set grows as live captures attest new ones.
const (
	// ErrCodeNoSecurityDefinition: no security definition has been found
	// for the request, or the contract description is ambiguous.
	ErrCodeNoSecurityDefinition = 200
	// ErrCodeOrderRejected: the order was rejected; the reason follows in
	// the message text.
	ErrCodeOrderRejected = 201
	// ErrCodeOrderCanceled: an active order was canceled. A cancellation
	// notice, not a placement failure.
	ErrCodeOrderCanceled = 202
	// ErrCodeServerErrorReadingRequest: server error when reading an API
	// client request.
	ErrCodeServerErrorReadingRequest = 320
	// ErrCodeServerErrorValidatingRequest: server error when validating an
	// API client request.
	ErrCodeServerErrorValidatingRequest = 321
	// ErrCodeTrailingStopAttachRejected: a trailing stop order can only be
	// attached to a limit or stop-limit parent.
	ErrCodeTrailingStopAttachRejected = 328
	// ErrCodeMarketDataNotSubscribed: not subscribed to requested market
	// data; the request fails outright.
	ErrCodeMarketDataNotSubscribed = 354
	// ErrCodeOrderMessage: order held with a warning, e.g. an off-hours
	// order deferred until the next session ("will not be placed at the
	// exchange until ..."). The order stays working at IB; the engine
	// surfaces the warning as the order handle's terminal error.
	ErrCodeOrderMessage = 399
	// ErrCodeInvalidRealTimeQuery: invalid real-time bars query for the
	// requested contract or what-to-show.
	ErrCodeInvalidRealTimeQuery = 420
	// ErrCodeFundamentalsNotAvailable: fundamentals data for the specified
	// security is not available.
	ErrCodeFundamentalsNotAvailable = 430
	// ErrCodeConnectivityLost: connectivity between IB and TWS/Gateway has
	// been lost.
	ErrCodeConnectivityLost = 1100
	// ErrCodeConnectivityRestoredDataLost: connectivity restored; market
	// data requests were lost and must be re-submitted.
	ErrCodeConnectivityRestoredDataLost = 1101
	// ErrCodeConnectivityRestoredDataMaintained: connectivity restored;
	// market data requests were recovered.
	ErrCodeConnectivityRestoredDataMaintained = 1102
	// ErrCodeMarketDataFarmOK: market data farm connection is OK.
	ErrCodeMarketDataFarmOK = 2104
	// ErrCodeHistoricalDataFarmOK: historical (HMDS) data farm connection
	// is OK.
	ErrCodeHistoricalDataFarmOK = 2106
	// ErrCodeHistoricalDataFarmInactive: historical (HMDS) data farm
	// connection is inactive but should be available upon demand.
	ErrCodeHistoricalDataFarmInactive = 2107
	// ErrCodeSecDefDataFarmOK: security-definition data farm connection is
	// OK.
	ErrCodeSecDefDataFarmOK = 2158
	// ErrCodeAdditionalSubscriptionRequired: requested market data requires
	// an additional subscription for API use.
	ErrCodeAdditionalSubscriptionRequired = 10089
	// ErrCodeDeepMarketDataNotSupported: deep (Level 2) market data is not
	// supported for this combination of security type and exchange.
	ErrCodeDeepMarketDataNotSupported = 10092
	// ErrCodeDelayedMarketDataDisplayed: requested market data is not
	// subscribed; delayed market data is displayed and the stream continues
	// with delayed ticks.
	ErrCodeDelayedMarketDataDisplayed = 10167
	// ErrCodeNewsFeedNotAllowed: the API client is not permissioned for the
	// requested (WSH) news feed.
	ErrCodeNewsFeedNotAllowed = 10276
)

// IsEntitlement reports whether the error signals a missing market-data or
// news entitlement: [ErrCodeMarketDataNotSubscribed],
// [ErrCodeAdditionalSubscriptionRequired], [ErrCodeDelayedMarketDataDisplayed],
// and [ErrCodeNewsFeedNotAllowed]. [ErrCodeDeepMarketDataNotSupported] is
// excluded: it reports a venue capability gap, not a missing subscription.
func (e *APIError) IsEntitlement() bool {
	switch e.Code {
	case ErrCodeMarketDataNotSubscribed, ErrCodeAdditionalSubscriptionRequired,
		ErrCodeDelayedMarketDataDisplayed, ErrCodeNewsFeedNotAllowed:
		return true
	}
	return false
}

// IsConnectivityTransition reports whether the code is one of the official
// system message codes for Gateway-to-IB connectivity transitions:
// [ErrCodeConnectivityLost], [ErrCodeConnectivityRestoredDataLost], and
// [ErrCodeConnectivityRestoredDataMaintained]. The client intercepts these
// codes to drive session state, so they normally surface as [Event] codes
// rather than request errors.
func (e *APIError) IsConnectivityTransition() bool {
	switch e.Code {
	case ErrCodeConnectivityLost, ErrCodeConnectivityRestoredDataLost,
		ErrCodeConnectivityRestoredDataMaintained:
		return true
	}
	return false
}

// IsFarmStatus reports whether the code is a data-farm status notification:
// [ErrCodeMarketDataFarmOK], [ErrCodeHistoricalDataFarmOK],
// [ErrCodeHistoricalDataFarmInactive], and [ErrCodeSecDefDataFarmOK]. Farm
// status codes are informational; the client emits them as session [Event]
// values and they never fail a request.
func (e *APIError) IsFarmStatus() bool {
	switch e.Code {
	case ErrCodeMarketDataFarmOK, ErrCodeHistoricalDataFarmOK,
		ErrCodeHistoricalDataFarmInactive, ErrCodeSecDefDataFarmOK:
		return true
	}
	return false
}

// IsWarning reports whether the code is informational rather than a request
// failure: the farm-status set (see [APIError.IsFarmStatus]) plus
// [ErrCodeDelayedMarketDataDisplayed], which IBKR delivers on a stream that
// continues with delayed ticks.
func (e *APIError) IsWarning() bool {
	return e.IsFarmStatus() || e.Code == ErrCodeDelayedMarketDataDisplayed
}

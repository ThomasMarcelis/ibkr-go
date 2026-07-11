package ibkr

import "strings"

// IBKR error and message codes attested in this repository's live-derived
// IB Gateway captures or defined by the official TWS API message-code tables.
// The set stays deliberately narrow: register a code only when its public
// classification is useful and its meaning is grounded in primary evidence.
const (
	// ErrCodeMaxMessageRate means the client exceeded the Gateway's maximum
	// outbound message rate. The official table says the Gateway will likely
	// disconnect the client after this error.
	ErrCodeMaxMessageRate = 100
	// ErrCodeCancelNotCancellableState: a cancel was attempted while the
	// order was not in a cancellable state (already cancelled or filled);
	// the live Gateway appends the order's permId to the message. This is a
	// cancellation reply, not an order-placement failure.
	ErrCodeCancelNotCancellableState = 161
	// ErrCodeHistoricalDataService is the generic HMDS service-error envelope.
	// The message, not this code alone, distinguishes pacing violations from
	// other historical-data failures.
	ErrCodeHistoricalDataService = 162
	// ErrCodeHistoricalDataQueryMessage: an HMDS query message. The scanner
	// route recognizes its exact live "no items retrieved" form as
	// nonterminal because the Gateway follows it with a valid empty result.
	ErrCodeHistoricalDataQueryMessage = 165
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
	// ErrCodeServerErrorProcessingRequest: server error when processing an
	// API client request. Live-attested wrapping the option-exercise
	// refusals ("Exercise ignored because option is not in-the-money.",
	// "Exercise/Lapse failed due to server rejection.") delivered on the
	// exercise request id.
	ErrCodeServerErrorProcessingRequest = 322
	// ErrCodeTrailingStopAttachRejected: a trailing stop order can only be
	// attached to a limit or stop-limit parent.
	ErrCodeTrailingStopAttachRejected = 328
	// ErrCodeMarketDataNotSubscribed: not subscribed to requested market
	// data; the request fails outright.
	ErrCodeMarketDataNotSubscribed = 354
	// ErrCodeUnsupportedOrderType: unsupported order type for this exchange
	// and security type (live-attested rejecting a PEG MKT placement on
	// SMART/STK); the placement is rejected outright with no order_status.
	ErrCodeUnsupportedOrderType = 387
	// ErrCodeOrderMessage: order held with a warning, e.g. an off-hours
	// order deferred until the next session ("will not be placed at the
	// exchange until ..."). The order stays working at IB and remains
	// cancellable, so the engine delivers it non-terminally as an
	// [OrderEvent].Warning; the handle stays open and its real lifecycle
	// (later status updates, the eventual terminal close) continues.
	ErrCodeOrderMessage = 399
	// ErrCodeInvalidRealTimeQuery: invalid real-time bars query. The live
	// attested instance was permission-flavored ("No market data permissions
	// for ISLAND STK"), but the official meaning is the generic invalid
	// query, so it stays out of the entitlement class.
	ErrCodeInvalidRealTimeQuery = 420
	// ErrCodeAlgoDefinitionNotFound: order processing failed because the
	// Gateway has no algorithm definition for the requested algo strategy;
	// the placement is rejected outright with no order_status.
	ErrCodeAlgoDefinitionNotFound = 439
	// ErrCodeUnknownAlgoAttribute: order processing failed because an algo
	// param tag is not a known attribute of the requested strategy; the
	// message names the offending tag and the placement is rejected outright.
	ErrCodeUnknownAlgoAttribute = 443
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
	// ErrCodeSmartDepthExchanges: a SMART market-depth availability notice
	// listing exchanges that can supply depth and exchanges that need
	// additional permissions. It is nonterminal; depth rows may follow.
	ErrCodeSmartDepthExchanges = 2152
	// ErrCodeSecDefDataFarmOK: security-definition data farm connection is
	// OK.
	ErrCodeSecDefDataFarmOK = 2158
	// ErrCodeInvalidFXHedgeOrder: invalid FX hedge order; the hedging
	// contract can only be a currency pair where one of the currencies
	// matches the parent order. The placement is rejected outright with no
	// order_status.
	ErrCodeInvalidFXHedgeOrder = 10063
	// ErrCodeAdditionalSubscriptionRequired: requested market data requires
	// an additional subscription for API use.
	ErrCodeAdditionalSubscriptionRequired = 10089
	// ErrCodeDeepMarketDataNotSupported: deep (Level 2) market data is not
	// supported for this combination of security type and exchange.
	ErrCodeDeepMarketDataNotSupported = 10092
	// ErrCodeOrderToCancelNotFound: the order id named in a cancel request
	// is not known to the Gateway.
	ErrCodeOrderToCancelNotFound = 10147
	// ErrCodeOrderCannotBeCancelled: the order id named in a cancel request
	// is in a state that cannot be cancelled; the message names the state
	// (e.g. Filled or Cancelled).
	ErrCodeOrderCannotBeCancelled = 10148
	// ErrCodeDelayedMarketDataDisplayed: requested market data is not
	// subscribed; delayed market data is displayed and the stream continues
	// with delayed ticks.
	ErrCodeDelayedMarketDataDisplayed = 10167
	// ErrCodeTickByTickDataNotAllowed: a tick-by-tick request lacks the
	// market-data permission required for the contract. Unlike delayed quote
	// code 10167, this response terminates the tick-by-tick subscription.
	ErrCodeTickByTickDataNotAllowed = 10189
	// ErrCodeDisplaySizeNotAllowed: the 'Display Size' order attribute may
	// not be specified for this order (live-attested rejecting a DarkIce
	// algo placement carrying display size 1); the placement is rejected
	// outright with no order_status.
	ErrCodeDisplaySizeNotAllowed = 10255
	// ErrCodeNewsFeedNotAllowed: the API client is not permissioned for the
	// requested (WSH) news feed.
	ErrCodeNewsFeedNotAllowed = 10276
	// ErrCodeImbalanceOnlyNotAllowed: the 'ImbalanceOnly' order attribute may
	// not be specified for this order. Live-attested replying to the cancel
	// of a silently accepted PEG MID / PEG BEST order, which the Gateway
	// later discarded on a global cancel.
	ErrCodeImbalanceOnlyNotAllowed = 10342
	// ErrCodeOrderTIFSetFromPreset: notice that the Gateway set the
	// instruction's TIF from an order preset ("Order TIF was set to DAY
	// based on order preset."), live-attested acknowledging an option
	// exercise as a working DAY instruction.
	ErrCodeOrderTIFSetFromPreset = 10349
)

// IsEntitlement reports whether the error signals a missing market-data or
// news entitlement: [ErrCodeMarketDataNotSubscribed],
// [ErrCodeAdditionalSubscriptionRequired], [ErrCodeDelayedMarketDataDisplayed],
// [ErrCodeTickByTickDataNotAllowed], and [ErrCodeNewsFeedNotAllowed].
// [ErrCodeDeepMarketDataNotSupported] is excluded: it reports a venue
// capability gap, not a missing subscription.
func (e *APIError) IsEntitlement() bool {
	switch e.Code {
	case ErrCodeMarketDataNotSubscribed, ErrCodeAdditionalSubscriptionRequired,
		ErrCodeDelayedMarketDataDisplayed, ErrCodeTickByTickDataNotAllowed,
		ErrCodeNewsFeedNotAllowed:
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
// failure: the farm-status set (see [APIError.IsFarmStatus]),
// [ErrCodeDelayedMarketDataDisplayed] (the stream continues with delayed
// ticks), [ErrCodeSmartDepthExchanges] (available depth venues), and
// [ErrCodeOrderMessage] (the order stays working at IB; live replays show it
// still cancellable after the warning), and [ErrCodeOrderTIFSetFromPreset]
// (the Gateway supplies a missing TIF and continues processing the request).
//
// The engine consults this predicate for order-targeted sub-10000 codes to
// deliver [OrderEvent].Warning without closing the handle; the 10xxx band's
// order handling is attestation-gated separately, so a newly attested
// order-targeted 10xxx warning needs its own wiring there.
func (e *APIError) IsWarning() bool {
	return e.IsFarmStatus() || e.Code == ErrCodeDelayedMarketDataDisplayed ||
		e.Code == ErrCodeSmartDepthExchanges || e.Code == ErrCodeOrderMessage ||
		e.Code == ErrCodeOrderTIFSetFromPreset
}

// IsPacingViolation reports whether the error requires retrying with backoff.
// Code [ErrCodeMaxMessageRate] is always a pacing violation. Historical and
// real-time query errors use generic codes, so they qualify only when the
// Gateway message explicitly identifies a pacing violation.
func (e *APIError) IsPacingViolation() bool {
	if e.Code == ErrCodeMaxMessageRate {
		return true
	}
	return (e.Code == ErrCodeHistoricalDataService || e.Code == ErrCodeInvalidRealTimeQuery) &&
		strings.Contains(strings.ToLower(e.Message), "pacing violation")
}

// IsOrderRejection reports whether the error is in the live-attested set that
// proves a placement failed before the Gateway exposed working-order evidence.
// Unknown order-targeted errors are deliberately excluded because detaching a
// live order is more dangerous than retaining its handle and surfacing a
// warning.
func (e *APIError) IsOrderRejection() bool {
	return isOrderRejectionCode(e.Code)
}

func isOrderRejectionCode(code int) bool {
	switch code {
	case ErrCodeNoSecurityDefinition, ErrCodeOrderRejected,
		ErrCodeServerErrorReadingRequest, ErrCodeServerErrorValidatingRequest,
		ErrCodeTrailingStopAttachRejected, ErrCodeUnsupportedOrderType,
		ErrCodeAlgoDefinitionNotFound, ErrCodeUnknownAlgoAttribute,
		ErrCodeInvalidFXHedgeOrder, ErrCodeDisplaySizeNotAllowed:
		return true
	}
	return false
}

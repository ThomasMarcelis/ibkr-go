package ibkr

import "testing"

// TestAPIErrorClassification asserts every helper's membership over the full
// registered code set, so each code doubles as a negative case for the classes
// it does not belong to.
func TestAPIErrorClassification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code         int
		message      string
		entitlement  bool
		connectivity bool
		farmStatus   bool
		warning      bool
		pacing       bool
		orderReject  bool
	}{
		{code: ErrCodeMaxMessageRate, pacing: true},
		{code: ErrCodeCancelNotCancellableState},
		{code: ErrCodeHistoricalDataService},
		{code: ErrCodeHistoricalDataService, message: "Historical data request pacing violation", pacing: true},
		{code: ErrCodeHistoricalDataQueryMessage},
		{code: ErrCodeNoSecurityDefinition, orderReject: true},
		{code: ErrCodeOrderRejected, orderReject: true},
		{code: ErrCodeOrderCanceled},
		{code: ErrCodeServerErrorReadingRequest, orderReject: true},
		{code: ErrCodeServerErrorValidatingRequest, orderReject: true},
		{code: ErrCodeServerErrorProcessingRequest},
		{code: ErrCodeTrailingStopAttachRejected, orderReject: true},
		{code: ErrCodeMarketDataNotSubscribed, entitlement: true},
		{code: ErrCodeUnsupportedOrderType, orderReject: true},
		{code: ErrCodeOrderMessage, warning: true},
		{code: ErrCodeInvalidRealTimeQuery},
		{code: ErrCodeInvalidRealTimeQuery, message: "Invalid real-time query: pacing violation", pacing: true},
		{code: ErrCodeAlgoDefinitionNotFound, orderReject: true},
		{code: ErrCodeUnknownAlgoAttribute, orderReject: true},
		{code: ErrCodeConnectivityLost, connectivity: true},
		{code: ErrCodeConnectivityRestoredDataLost, connectivity: true},
		{code: ErrCodeConnectivityRestoredDataMaintained, connectivity: true},
		{code: ErrCodeMarketDataFarmOK, farmStatus: true, warning: true},
		{code: ErrCodeHistoricalDataFarmOK, farmStatus: true, warning: true},
		{code: ErrCodeHistoricalDataFarmInactive, farmStatus: true, warning: true},
		{code: ErrCodeHistoricalDataSubscriptionRequired, entitlement: true},
		{code: ErrCodeSecDefDataFarmOK, farmStatus: true, warning: true},
		{code: ErrCodeSmartDepthExchanges, warning: true},
		{code: ErrCodeAdditionalSubscriptionRequired, entitlement: true},
		{code: ErrCodeInvalidFXHedgeOrder, orderReject: true},
		{code: ErrCodeDeepMarketDataNotSupported},
		{code: ErrCodeOrderToCancelNotFound},
		{code: ErrCodeOrderCannotBeCancelled},
		{code: ErrCodeDelayedMarketDataDisplayed, entitlement: true, warning: true},
		{code: ErrCodeTickByTickDataNotAllowed, entitlement: true},
		{code: ErrCodeDisplaySizeNotAllowed, orderReject: true},
		{code: ErrCodeNewsFeedNotAllowed, entitlement: true},
		{code: ErrCodeImbalanceOnlyNotAllowed},
		{code: ErrCodeOrderTIFSetFromPreset, warning: true},
	}
	for _, tt := range tests {
		err := &APIError{Code: tt.code, Message: tt.message}
		if got := err.IsEntitlement(); got != tt.entitlement {
			t.Errorf("APIError{Code: %d}.IsEntitlement() = %v, want %v", tt.code, got, tt.entitlement)
		}
		if got := err.IsConnectivityTransition(); got != tt.connectivity {
			t.Errorf("APIError{Code: %d}.IsConnectivityTransition() = %v, want %v", tt.code, got, tt.connectivity)
		}
		if got := err.IsFarmStatus(); got != tt.farmStatus {
			t.Errorf("APIError{Code: %d}.IsFarmStatus() = %v, want %v", tt.code, got, tt.farmStatus)
		}
		if got := err.IsWarning(); got != tt.warning {
			t.Errorf("APIError{Code: %d}.IsWarning() = %v, want %v", tt.code, got, tt.warning)
		}
		if got := err.IsPacingViolation(); got != tt.pacing {
			t.Errorf("APIError{Code: %d, Message: %q}.IsPacingViolation() = %v, want %v", tt.code, tt.message, got, tt.pacing)
		}
		if got := err.IsOrderRejection(); got != tt.orderReject {
			t.Errorf("APIError{Code: %d}.IsOrderRejection() = %v, want %v", tt.code, got, tt.orderReject)
		}
	}
}

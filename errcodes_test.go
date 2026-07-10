package ibkr

import "testing"

// TestAPIErrorClassification asserts every helper's membership over the full
// registered code set, so each code doubles as a negative case for the classes
// it does not belong to.
func TestAPIErrorClassification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code         int
		entitlement  bool
		connectivity bool
		farmStatus   bool
		warning      bool
	}{
		{code: ErrCodeCancelNotCancellableState},
		{code: ErrCodeNoSecurityDefinition},
		{code: ErrCodeOrderRejected},
		{code: ErrCodeOrderCanceled},
		{code: ErrCodeServerErrorReadingRequest},
		{code: ErrCodeServerErrorValidatingRequest},
		{code: ErrCodeServerErrorProcessingRequest},
		{code: ErrCodeTrailingStopAttachRejected},
		{code: ErrCodeMarketDataNotSubscribed, entitlement: true},
		{code: ErrCodeUnsupportedOrderType},
		{code: ErrCodeOrderMessage, warning: true},
		{code: ErrCodeInvalidRealTimeQuery},
		{code: ErrCodeAlgoDefinitionNotFound},
		{code: ErrCodeUnknownAlgoAttribute},
		{code: ErrCodeConnectivityLost, connectivity: true},
		{code: ErrCodeConnectivityRestoredDataLost, connectivity: true},
		{code: ErrCodeConnectivityRestoredDataMaintained, connectivity: true},
		{code: ErrCodeMarketDataFarmOK, farmStatus: true, warning: true},
		{code: ErrCodeHistoricalDataFarmOK, farmStatus: true, warning: true},
		{code: ErrCodeHistoricalDataFarmInactive, farmStatus: true, warning: true},
		{code: ErrCodeSecDefDataFarmOK, farmStatus: true, warning: true},
		{code: ErrCodeAdditionalSubscriptionRequired, entitlement: true},
		{code: ErrCodeInvalidFXHedgeOrder},
		{code: ErrCodeDeepMarketDataNotSupported},
		{code: ErrCodeOrderToCancelNotFound},
		{code: ErrCodeOrderCannotBeCancelled},
		{code: ErrCodeDelayedMarketDataDisplayed, entitlement: true, warning: true},
		{code: ErrCodeTickByTickDataNotAllowed, entitlement: true},
		{code: ErrCodeDisplaySizeNotAllowed},
		{code: ErrCodeNewsFeedNotAllowed, entitlement: true},
		{code: ErrCodeImbalanceOnlyNotAllowed},
		{code: ErrCodeOrderTIFSetFromPreset},
	}
	for _, tt := range tests {
		err := &APIError{Code: tt.code}
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
	}
}

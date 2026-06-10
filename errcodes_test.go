package ibkr

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TestErrCodeRegistryCoversTranscriptEvidence walks every replay transcript
// and asserts each api_error code is either a named ErrCode constant or an
// explicit allowlist entry, so a transcript attesting a new code forces a
// registry decision.
func TestErrCodeRegistryCoversTranscriptEvidence(t *testing.T) {
	t.Parallel()

	registered := map[int]bool{
		ErrCodeNoSecurityDefinition:               true,
		ErrCodeOrderRejected:                      true,
		ErrCodeOrderCanceled:                      true,
		ErrCodeServerErrorReadingRequest:          true,
		ErrCodeServerErrorValidatingRequest:       true,
		ErrCodeTrailingStopAttachRejected:         true,
		ErrCodeMarketDataNotSubscribed:            true,
		ErrCodeInvalidRealTimeQuery:               true,
		ErrCodeFundamentalsNotAvailable:           true,
		ErrCodeConnectivityLost:                   true,
		ErrCodeConnectivityRestoredDataLost:       true,
		ErrCodeConnectivityRestoredDataMaintained: true,
		ErrCodeMarketDataFarmOK:                   true,
		ErrCodeHistoricalDataFarmOK:               true,
		ErrCodeHistoricalDataFarmInactive:         true,
		ErrCodeSecDefDataFarmOK:                   true,
		ErrCodeAdditionalSubscriptionRequired:     true,
		ErrCodeDeepMarketDataNotSupported:         true,
		ErrCodeDelayedMarketDataDisplayed:         true,
		ErrCodeNewsFeedNotAllowed:                 true,
	}
	// Codes attested in transcripts but deliberately left unregistered.
	// Empty today: a transcript attesting a new api_error code must land
	// with either a named constant or an explicit entry here.
	unregisteredAttested := map[int]bool{}

	codes := transcriptAPIErrorCodes(t)
	if len(codes) == 0 {
		t.Fatal("no api_error codes found in testdata/transcripts")
	}
	for code, files := range codes {
		if registered[code] || unregisteredAttested[code] {
			continue
		}
		t.Errorf("api_error code %d attested in %v has no ErrCode constant and no unregisteredAttested entry", code, files)
	}
}

var apiErrorCodeRe = regexp.MustCompile(`"code":(-?[0-9]+)`)

// transcriptAPIErrorCodes returns every api_error code in the replay
// transcripts, mapped to the fixtures that attest it.
func transcriptAPIErrorCodes(t *testing.T) map[int][]string {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join("testdata", "transcripts", "*.txt"))
	if err != nil {
		t.Fatalf("Glob(testdata/transcripts) error = %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no transcripts found under testdata/transcripts")
	}

	codes := map[int][]string{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", path, err)
		}
		name := filepath.Base(path)
		for line := range strings.Lines(string(data)) {
			if !strings.Contains(line, "api_error") {
				continue
			}
			m := apiErrorCodeRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			code, err := strconv.Atoi(m[1])
			if err != nil {
				t.Fatalf("Atoi(%q) in %s error = %v", m[1], name, err)
			}
			if !slices.Contains(codes[code], name) {
				codes[code] = append(codes[code], name)
			}
		}
	}
	return codes
}

// TestAPIErrorClassification asserts every helper's membership over the full
// attested code set, so each code doubles as a negative case for the classes
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
		{code: ErrCodeNoSecurityDefinition},
		{code: ErrCodeOrderCanceled},
		{code: ErrCodeServerErrorReadingRequest},
		{code: ErrCodeServerErrorValidatingRequest},
		{code: ErrCodeMarketDataNotSubscribed, entitlement: true},
		{code: ErrCodeConnectivityLost, connectivity: true},
		{code: ErrCodeConnectivityRestoredDataLost, connectivity: true},
		{code: ErrCodeConnectivityRestoredDataMaintained, connectivity: true},
		{code: ErrCodeMarketDataFarmOK, farmStatus: true, warning: true},
		{code: ErrCodeHistoricalDataFarmOK, farmStatus: true, warning: true},
		{code: ErrCodeHistoricalDataFarmInactive, farmStatus: true, warning: true},
		{code: ErrCodeSecDefDataFarmOK, farmStatus: true, warning: true},
		{code: ErrCodeAdditionalSubscriptionRequired, entitlement: true},
		{code: ErrCodeDeepMarketDataNotSupported},
		{code: ErrCodeDelayedMarketDataDisplayed, entitlement: true, warning: true},
		{code: ErrCodeNewsFeedNotAllowed, entitlement: true},
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

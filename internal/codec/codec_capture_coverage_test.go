package codec

import (
	"os"
	"strings"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/protocol"
)

// Raw-frame capture-coverage gate and inbound evidence ledger.
//
// Every decoder registered in inboundDecoders must be attested by at least one
// "capture-decode" test: a test that feeds a HARDCODED raw wire frame (a
// []byte("…\x00…") literal, or a hardcoded field slice whose first token is the
// literal msg_id) into DecodeBatch/Decode and asserts on the typed result. The
// frame must be live-derived, not synthesized.
//
// Why this gate exists. This library once leaned on symmetric round-trip tests
// (Encode → Decode). Those tests share a single msg_id constant on both sides,
// so a wrong constant or a wrong field layout replays perfectly green and only
// fails against live Gateway traffic — which, before unknown ids became
// UnknownInbound, killed the whole session with ErrInterrupted. Two shipped
// bugs proved the class:
//
//   - InMarketRule was 92; the live Gateway sends MarketRule on 93. Every live
//     reply decoded as an unknown frame. Round-trip stayed green because Encode
//     and Decode both used 92. Frozen by TestCaptureDecode_MarketRuleLive
//     (raw "93\x00…" frame from captures/v1/market_rule.log).
//   - msg 108 was decoded as the streaming historical-bar update; it is
//     actually HISTORICAL_DATA_END. The real streaming update is msg 90. Frozen
//     by TestCaptureDecode_HistoricalDataEndLive (raw "108\x00…" frame).
//
// A raw-frame test hardcodes the msg_id token on the wire, so it exercises the
// real dispatch path and catches exactly this class. Round-trip tests and fuzz
// tests do NOT count — they are the class that masked the bugs.
//
// The two catalogs below partition every inboundDecoders key. rawFrameAttested
// records ids proven by a cited live frame. pendingLiveAttestation records ids
// whose decoder has no live-attested raw frame yet, each with the capture that
// would provide one; this list is the capture-planning backlog.
//
// TestInboundDecoderRawFrameCoverage is the hard gate: every registered decoder
// must appear in EXACTLY one catalog, and neither catalog may name an id that
// has no registered decoder. Adding a decoder without a raw frame fails the
// build until you either write the capture-decode test (and move the id to
// rawFrameAttested) or add a justified pending entry. This is a deliberate
// static catalog, not reflection over test names: the membership set is the
// contract.

// TestInboundDecoderRegistryCoverage keeps the decoder table and canonical
// protocol registry in lockstep. A classic inbound message is either decoded
// deliberately or absent from the implemented registry; silent half-support
// is not allowed.
func TestInboundDecoderRegistryCoverage(t *testing.T) {
	t.Parallel()

	registered := make(map[int]protocol.Message)
	for _, message := range protocol.Messages() {
		if message.Direction == protocol.ServerToClient {
			registered[message.ID] = message
		}
	}
	for id := range inboundDecoders {
		if _, ok := registered[id]; !ok {
			t.Errorf("decoder for inbound message ID %d is absent from protocol registry", id)
		}
	}
	for id, message := range registered {
		if _, ok := inboundDecoders[id]; !ok {
			t.Errorf("protocol registry claims %s (%d), but no decoder is registered", message.Name, id)
		}
	}
}

// rawFrameAttested maps a decoder's msg_id to one existing test that decodes a
// hardcoded live wire frame for it and asserts the typed result. Where several
// tests qualify, the most representative live frame is named.
var rawFrameAttested = map[int]string{
	InTickPrice:             "TestCaptureDecode_TickPrice",
	InTickSize:              "TestCaptureDecode_TickSize",
	InErrMsg:                "TestCaptureDecode_APIError_2104",
	InOpenOrder:             "TestCaptureDecode_OpenOrder",
	InNextValidID:           "TestCaptureDecode_NextValidID",
	InContractData:          "TestCaptureDecode_ContractDetails",
	InBondContractData:      "TestCaptureDecode_BondContractDetails",
	InExecutionData:         "TestCaptureDecode_ExecutionDetailNativeTime",
	InExecutionDataEnd:      "TestCaptureDecode_ExecutionsEndLive",
	InCommissionReport:      "TestCaptureDecode_CommissionAndFeesLive",
	InManagedAccounts:       "TestCaptureDecode_ManagedAccounts",
	InHistoricalData:        "TestCaptureDecode_HistoricalData",
	InTickOptionComputation: "TestCaptureDecode_TickOptionComputationLive",
	InContractDataEnd:       "TestCaptureDecode_ContractDetailsEnd",
	InOpenOrderEnd:          "TestCaptureDecode_OpenOrderEnd",
	InTickSnapshotEnd:       "TestCaptureDecode_TickSnapshotEnd",
	InMarketDataType:        "TestCaptureDecode_MarketDataType",
	InPositionData:          "TestCaptureDecode_Position",
	InPositionEnd:           "TestCaptureDecode_PositionEnd",
	InAccountSummary:        "TestCaptureDecode_AccountSummaryValue",
	InAccountSummaryEnd:     "TestCaptureDecode_AccountSummaryEnd",
	InSecDefOptParams:       "TestCaptureDecode_SecDefOptParamsLive",
	InMarketRule:            "TestCaptureDecode_MarketRuleLive",
	InNewsBulletins:         "TestCaptureDecode_NewsBulletinLive",
	InCompletedOrder:        "TestCaptureDecode_CompletedOrderTrailLimitLive",
	InHistoricalSchedule:    "TestCaptureDecode_HistoricalSchedule",
	InUserInfo:              "TestDecodeUserInfoLiveFrame",
	InHistoricalDataEnd:     "TestCaptureDecode_HistoricalDataEndLive",
	InTickGeneric:           "TestCaptureDecode_QuoteAncillaryTicksLive",
	InTickString:            "TestCaptureDecode_QuoteAncillaryTicksLive",
	InTickReqParams:         "TestCaptureDecode_QuoteAncillaryTicksLive",
	InTickNews:              "TestCaptureDecode_TickNewsLive",
	InScannerData:           "TestCaptureDecode_ScannerDataLive",
	InCurrentTimeInMillis:   "TestCaptureDecode_CurrentTimeMillis",
	InCurrentTime:           "TestCaptureDecode_CurrentTimeLive",
	InPositionMultiEnd:      "TestCaptureDecode_PositionMultiEndLive",
	InFamilyCodes:           "TestCaptureDecode_FamilyCodesLive",
	InNewsProviders:         "TestCaptureDecode_NewsProvidersLive",
	InPnL:                   "TestCaptureDecode_PnLLive",
	InOrderStatus:           "TestCaptureDecode_OrderStatusLive",
	InUpdateAccountValue:    "TestCaptureDecode_AccountUpdatesLive",
	InUpdatePortfolio:       "TestCaptureDecode_AccountUpdatesLive",
	InUpdateAccountTime:     "TestCaptureDecode_AccountUpdatesLive",
	InAccountDownloadEnd:    "TestCaptureDecode_AccountUpdatesLive",
	InDisplayGroupList:      "TestCaptureDecode_DisplayGroupsLive",
	InDisplayGroupUpdated:   "TestCaptureDecode_DisplayGroupsLive",
	InAccountUpdateMulti:    "TestCaptureDecode_AccountUpdatesMultiLive",
	InAccountUpdateMultiEnd: "TestCaptureDecode_AccountUpdatesMultiLive",
	InSecDefOptParamsEnd:    "TestCaptureDecode_SecDefOptParamsEndLive",
	InSoftDollarTiers:       "TestCaptureDecode_SoftDollarTiersLive",
	InNewsArticle:           "TestCaptureDecode_HistoricalNewsFlowLive",
	InHistoricalNews:        "TestCaptureDecode_HistoricalNewsFlowLive",
	InHistoricalNewsEnd:     "TestCaptureDecode_HistoricalNewsFlowLive",
	InHeadTimestamp:         "TestCaptureDecode_HeadTimestampLive",
	InCompletedOrderEnd:     "TestCaptureDecode_CompletedOrderEndLive",
}

// pendingLiveAttestation maps a decoder's msg_id to a one-line reason it has no
// live-attested raw frame yet and the capture that would provide one. An entry
// here is a promise to capture, not a license to skip: move the id to
// rawFrameAttested the moment a cited live frame exists.
var pendingLiveAttestation = map[int]string{
	InMarketDepth:           "needs a reqMktDepth (L1) capture",
	InMarketDepthL2:         "needs a reqMktDepth L2 (isSmartDepth) capture",
	InReceiveFA:             "needs a requestFA capture (FA account entitlement)",
	InScannerParameters:     "needs a reqScannerParameters capture (large XML frame)",
	InRealTimeBars:          "needs a reqRealTimeBars 5s-bar capture (market hours)",
	InPositionMulti:         "needs a reqPositionsMulti row capture",
	InSymbolSamples:         "TestDecodeLiveSymbolSamplesFrameShape decodes a live-shaped frame but cites no capture — promote by adding a captures/ citation or a fresh reqMatchingSymbols capture",
	InMktDepthExchanges:     "needs a reqMktDepthExchanges capture",
	InSmartComponents:       "needs a reqSmartComponents capture",
	InHistogramData:         "needs a reqHistogramData capture",
	InHistoricalDataUpdate:  "source-referenced from the official client library; live attestation pending (needs a market-hours keepUpToDate reqHistoricalData capture) — see captures/v1/WIRE_TRUTH.md",
	InPnLSingle:             "needs a reqPnLSingle capture",
	InHistoricalTicks:       "needs a reqHistoricalTicks whatToShow=MIDPOINT capture",
	InHistoricalTicksBidAsk: "needs a reqHistoricalTicks whatToShow=BID_ASK capture",
	InHistoricalTicksLast:   "needs a reqHistoricalTicks whatToShow=TRADES capture",
	InTickByTick:            "needs a reqTickByTickData capture (market hours)",
	InReplaceFAEnd:          "source-referenced decoder coverage only — needs a live replaceFAEnd frame from an FA-entitled account",
	InWSHMetaData:           "needs a reqWSHMetaData capture (WSH entitlement)",
	InWSHEventData:          "needs a reqWSHEventData capture (WSH entitlement)",
}

// TestInboundDecoderRawFrameCoverage enforces that the two catalogs above
// partition inboundDecoders exactly: every registered decoder is either
// attested by a cited raw live frame or explicitly pending, never both and
// never neither, and neither catalog names an unregistered id.
func TestInboundDecoderRawFrameCoverage(t *testing.T) {
	for id := range inboundDecoders {
		_, attested := rawFrameAttested[id]
		_, pending := pendingLiveAttestation[id]

		switch {
		case attested && pending:
			t.Errorf("msg id %d is in BOTH rawFrameAttested and pendingLiveAttestation; "+
				"a decoder is attested or pending, never both — delete the stale entry", id)
		case !attested && !pending:
			t.Errorf("msg id %d has a registered decoder but no raw-frame coverage entry.\n"+
				"    Write a capture-decode test that feeds a hardcoded live wire frame into DecodeBatch "+
				"and asserts the typed result, then add the id to rawFrameAttested.\n"+
				"    If no live frame is capturable yet, add a justified pendingLiveAttestation entry "+
				"naming the capture that would provide it.", id)
		}
	}

	for id := range rawFrameAttested {
		if _, ok := inboundDecoders[id]; !ok {
			t.Errorf("rawFrameAttested names msg id %d, which has no decoder in inboundDecoders; "+
				"drop the entry or fix the id", id)
		}
	}
	for id := range pendingLiveAttestation {
		if _, ok := inboundDecoders[id]; !ok {
			t.Errorf("pendingLiveAttestation names msg id %d, which has no decoder in inboundDecoders; "+
				"drop the entry or fix the id", id)
		}
	}
}

// TestRawFrameAttestationTestsExist closes the gate's own loophole: the
// catalogs record attestation as test-name strings, so a rename or deletion
// of an attesting test would otherwise leave the gate green with a dangling
// reference. This scans the package's test sources for each named function.
func TestRawFrameAttestationTestsExist(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var src strings.Builder
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", entry.Name(), err)
		}
		src.Write(data)
		src.WriteByte('\n')
	}
	sources := src.String()

	for id, testName := range rawFrameAttested {
		if !strings.Contains(sources, "func "+testName+"(t *testing.T)") {
			t.Errorf("msg id %d claims attestation by %s, but no such test exists in this package; "+
				"update the entry to the test's current name or move the id to pendingLiveAttestation", id, testName)
		}
	}
}

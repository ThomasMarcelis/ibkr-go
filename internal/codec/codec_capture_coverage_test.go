package codec

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
)

// Raw-frame capture-coverage gate and inbound evidence ledger.
//
// Every decoder registered in either inbound decoder table must be attested by at least one
// test that feeds a hardcoded live-derived wire frame through the production
// decoder and asserts on the typed result. The frame may live directly in a
// codec test or in an exact raw public replay fixture; large frames should not
// be duplicated merely to satisfy this ledger.
//
// Why this gate exists. Symmetric round-trip tests share message IDs and field
// layouts between encoder and decoder, so the same defect on both sides can
// replay green while failing against a Gateway. A raw-frame test hardcodes the
// live wire token and body, exercises real dispatch, and catches that class.
// Round-trip and fuzz tests remain useful, but they do not count as live
// attestation here.
//
// The two catalogs below partition every registered (msg_id, encoding) pair.
// rawFrameAttested records pairs proven by a cited live frame.
// pendingLiveAttestation records pairs whose decoder has no live-attested raw
// frame yet; this list is the capture-planning backlog.
//
// TestInboundDecoderRawFrameCoverage is the hard gate: every registered decoder
// must appear in EXACTLY one catalog, and neither catalog may name an id that
// has no registered decoder. Adding a decoder without a raw frame fails the
// build until you either write the capture-decode test (and move the id to
// rawFrameAttested) or add a justified pending entry. This is a deliberate
// static catalog, not reflection over test names: the membership set is the
// contract.

// TestInboundDecoderRegistryCoverage keeps both decoder tables and the
// canonical protocol registry in lockstep. An inbound message is either
// decoded deliberately or absent from the implemented registry; silent
// half-support is not allowed.
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
	for id := range inboundProtobufDecoders {
		if _, ok := registered[id]; !ok {
			t.Errorf("protobuf decoder for inbound message ID %d is absent from protocol registry", id)
		}
	}
	for id, message := range registered {
		_, classic := inboundDecoders[id]
		_, protobuf := inboundProtobufDecoders[id]
		if !classic && !protobuf {
			t.Errorf("protocol registry claims %s (%d), but no decoder is registered", message.Name, id)
		}
	}
}

type decoderAttestationKey struct {
	msgID    int
	encoding protocol.BodyEncoding
}

// rawFrameAttested maps a decoder pair to a representative direct decode or
// exact raw public replay test. The names are navigation aids; the executable
// assertions and their cited live frames remain the evidence.
var rawFrameAttested = map[decoderAttestationKey]string{
	{protocol.InNextValidID, protocol.ClassicBody}:         "TestSupportedVersionMatrixReplay",
	{protocol.InCurrentTime, protocol.ClassicBody}:         "TestSupportedVersionMatrixReplay",
	{protocol.InCurrentTimeInMillis, protocol.ClassicBody}: "TestDecodeClassicSV208LiveFrames",
	{protocol.InScannerParameters, protocol.ClassicBody}:   "TestDecodeClassicSV208LiveFrames",
	{protocol.InScannerData, protocol.ClassicBody}:         "TestDecodeClassicSV208LiveFrames",
	{protocol.InSecDefOptParams, protocol.ClassicBody}:     "TestDecodeClassicSV208LiveFrames",
	{protocol.InSecDefOptParamsEnd, protocol.ClassicBody}:  "TestDecodeClassicSV208LiveFrames",
	{protocol.InFamilyCodes, protocol.ClassicBody}:         "TestDecodeClassicSV208LiveFrames",
	{protocol.InMktDepthExchanges, protocol.ClassicBody}:   "TestDecodeClassicSV208LiveFrames",
	{protocol.InNewsArticle, protocol.ClassicBody}:         "TestDecodeClassicSV208LiveFrames",
	{protocol.InTickNews, protocol.ClassicBody}:            "TestDecodeClassicSV208LiveFrames",
	{protocol.InNewsProviders, protocol.ClassicBody}:       "TestDecodeClassicSV208LiveFrames",
	{protocol.InSymbolSamples, protocol.ClassicBody}:       "TestDecodeClassicSV208LiveFrames",
	{protocol.InSmartComponents, protocol.ClassicBody}:     "TestDecodeClassicSV208LiveFrames",
	{protocol.InHistoricalNews, protocol.ClassicBody}:      "TestDecodeClassicSV208LiveFrames",
	{protocol.InHistoricalNewsEnd, protocol.ClassicBody}:   "TestDecodeClassicSV208LiveFrames",
	{protocol.InMarketRule, protocol.ClassicBody}:          "TestDecodeClassicSV208LiveFrames",
	{protocol.InSoftDollarTiers, protocol.ClassicBody}:     "TestDecodeClassicSV208LiveFrames",
	{protocol.InUserInfo, protocol.ClassicBody}:            "TestUserInfoClassicSV208LiveVector",
	{protocol.InPnL, protocol.ClassicBody}:                 "TestDecodeClassicSV208PnLLiveFrames",
	{protocol.InPnLSingle, protocol.ClassicBody}:           "TestDecodeClassicSV208PnLLiveFrames",
	{protocol.InDisplayGroupList, protocol.ClassicBody}:    "TestDecodeClassicSV208LiveFrames",
	{protocol.InDisplayGroupUpdated, protocol.ClassicBody}: "TestDecodeClassicSV208DisplayGroupUpdateLiveFrame",

	{protocol.InTickPrice, protocol.ProtobufBody}:             "TestQuoteSnapshot",
	{protocol.InTickSize, protocol.ProtobufBody}:              "TestQuoteSnapshot",
	{protocol.InOrderStatus, protocol.ProtobufBody}:           "TestAPIFutureCampaignMESReplay",
	{protocol.InErrMsg, protocol.ProtobufBody}:                "TestHistoricalBarsSubscriptionRequiredReplay",
	{protocol.InOpenOrder, protocol.ProtobufBody}:             "TestAPIFutureCampaignMESReplay",
	{protocol.InUpdateAccountValue, protocol.ProtobufBody}:    "TestAccountUpdatesSnapshot",
	{protocol.InUpdatePortfolio, protocol.ProtobufBody}:       "TestAccountUpdatesSnapshot",
	{protocol.InUpdateAccountTime, protocol.ProtobufBody}:     "TestAccountUpdatesSnapshot",
	{protocol.InNextValidID, protocol.ProtobufBody}:           "TestSupportedVersionMatrixReplay",
	{protocol.InContractData, protocol.ProtobufBody}:          "TestContractDetailsESFutureReplay",
	{protocol.InExecutionData, protocol.ProtobufBody}:         "TestAPIFutureCampaignMESReplay",
	{protocol.InManagedAccounts, protocol.ProtobufBody}:       "TestManagedAccountsRefreshReplay",
	{protocol.InHistoricalData, protocol.ProtobufBody}:        "TestHistoricalProto208LiveVectors",
	{protocol.InBondContractData, protocol.ProtobufBody}:      "TestContractDetailsAppleBondsReplay",
	{protocol.InScannerData, protocol.ProtobufBody}:           "TestScannerSubscriptionReturnsCurrentRankedResults",
	{protocol.InTickOptionComputation, protocol.ProtobufBody}: "TestQuoteRoutePreservesLiveOptionComputationPresence",
	{protocol.InTickGeneric, protocol.ProtobufBody}:           "TestQuoteRouteEmitsLiveAncillaryTicks",
	{protocol.InTickString, protocol.ProtobufBody}:            "TestQuoteSnapshot",
	{protocol.InTickReqParams, protocol.ProtobufBody}:         "TestQuoteSnapshot",
	{protocol.InCurrentTime, protocol.ProtobufBody}:           "TestDecodeSessionProto213LiveVectors",
	{protocol.InContractDataEnd, protocol.ProtobufBody}:       "TestContractDetailsESFutureReplay",
	{protocol.InOpenOrderEnd, protocol.ProtobufBody}:          "TestOpenOrdersEmptyReplay",
	{protocol.InAccountDownloadEnd, protocol.ProtobufBody}:    "TestAccountUpdatesSnapshot",
	{protocol.InExecutionDataEnd, protocol.ProtobufBody}:      "TestExecutionsEmptyReplay",
	{protocol.InTickSnapshotEnd, protocol.ProtobufBody}:       "TestQuoteSnapshot",
	{protocol.InMarketDataType, protocol.ProtobufBody}:        "TestQuoteSnapshot",
	{protocol.InCommissionReport, protocol.ProtobufBody}:      "TestAPIFutureCampaignMESReplay",
	{protocol.InPositionData, protocol.ProtobufBody}:          "TestPositionsSubscriptionSnapshotCompleteReplay",
	{protocol.InPositionEnd, protocol.ProtobufBody}:           "TestPositionsSubscriptionSnapshotCompleteReplay",
	{protocol.InAccountSummary, protocol.ProtobufBody}:        "TestAccountSummary",
	{protocol.InAccountSummaryEnd, protocol.ProtobufBody}:     "TestAccountSummary",
	{protocol.InDisplayGroupList, protocol.ProtobufBody}:      "TestDisplayGroupLifecycleIntegration",
	{protocol.InDisplayGroupUpdated, protocol.ProtobufBody}:   "TestDisplayGroupLifecycleIntegration",
	{protocol.InPositionMulti, protocol.ProtobufBody}:         "TestPositionsMultiSnapshot",
	{protocol.InPositionMultiEnd, protocol.ProtobufBody}:      "TestPositionsMultiSnapshot",
	{protocol.InAccountUpdateMulti, protocol.ProtobufBody}:    "TestAccountUpdatesMultiSnapshot",
	{protocol.InAccountUpdateMultiEnd, protocol.ProtobufBody}: "TestAccountUpdatesMultiSnapshot",
	{protocol.InSecDefOptParams, protocol.ProtobufBody}:       "TestAPIHedgeOrderReplay",
	{protocol.InSecDefOptParamsEnd, protocol.ProtobufBody}:    "TestDecodeReferenceProto212LiveVectors",
	{protocol.InSoftDollarTiers, protocol.ProtobufBody}:       "TestDecodeReferenceProto212LiveVectors",
	{protocol.InFamilyCodes, protocol.ProtobufBody}:           "TestDecodeReferenceProto212LiveVectors",
	{protocol.InSymbolSamples, protocol.ProtobufBody}:         "TestMatchingSymbols",
	{protocol.InMktDepthExchanges, protocol.ProtobufBody}:     "TestMktDepthExchanges",
	{protocol.InSmartComponents, protocol.ProtobufBody}:       "TestDecodeReferenceProto212LiveVectors",
	{protocol.InNewsArticle, protocol.ProtobufBody}:           "TestNewsArticle",
	{protocol.InNewsProviders, protocol.ProtobufBody}:         "TestDecodeNewsProto209LiveVectors",
	{protocol.InHistoricalNews, protocol.ProtobufBody}:        "TestDecodeNewsProto209LiveVectors",
	{protocol.InHistoricalNewsEnd, protocol.ProtobufBody}:     "TestDecodeNewsProto209LiveVectors",
	{protocol.InHeadTimestamp, protocol.ProtobufBody}:         "TestHeadTimestamp",
	{protocol.InMarketRule, protocol.ProtobufBody}:            "TestDecodeReferenceProto212LiveVectors",
	{protocol.InMarketDataReroute, protocol.ProtobufBody}:     "TestCaptureDecodeMarketDataRerouteProto225Live",
	{protocol.InPnL, protocol.ProtobufBody}:                   "TestDecodeScannerPnLProto210LiveVectors",
	{protocol.InPnLSingle, protocol.ProtobufBody}:             "TestDecodeScannerPnLProto210LiveVectors",
	{protocol.InCompletedOrder, protocol.ProtobufBody}:        "TestAPICompletedOrdersVariantsAAPLReplay",
	{protocol.InCompletedOrderEnd, protocol.ProtobufBody}:     "TestAPICompletedOrdersVariantsAAPLReplay",
	{protocol.InHistoricalSchedule, protocol.ProtobufBody}:    "TestHistoricalSchedule",
	{protocol.InUserInfo, protocol.ProtobufBody}:              "TestDecodeReferenceProto212LiveVectors",
	{protocol.InHistoricalDataEnd, protocol.ProtobufBody}:     "TestHistoricalProto208LiveVectors",
	{protocol.InCurrentTimeInMillis, protocol.ProtobufBody}:   "TestCurrentTimeMillisReplay",
	{protocol.InTickNews, protocol.ProtobufBody}:              "TestDecodeTickNewsProto225LiveFrame",
	{protocol.InHistoricalTicksLast, protocol.ProtobufBody}:   "TestDecodeHistoricalTicksLastProto215LiveFrame",
	{protocol.InScannerParameters, protocol.ProtobufBody}:     "TestDecodeScannerParametersProto225ExactLiveFrame",
	{protocol.InConfig, protocol.ProtobufBody}:                "TestDecodeConfigResponseProto225ExactLiveFrame",
}

// pendingLiveAttestation maps a decoder pair to the evidence still needed.
var pendingLiveAttestation = map[decoderAttestationKey]string{
	{protocol.InNewsBulletins, protocol.ClassicBody}:          "needs exact sv208 boundary evidence",
	{protocol.InReceiveFA, protocol.ClassicBody}:              "needs a requestFA capture",
	{protocol.InTickEFP, protocol.ClassicBody}:                "needs entitled single-stock-future data",
	{protocol.InDeltaNeutralValidation, protocol.ClassicBody}: "needs a successful BAG delta-neutral capture",
	{protocol.InWSHMetaData, protocol.ClassicBody}:            "needs a WSH metadata capture",
	{protocol.InWSHEventData, protocol.ClassicBody}:           "needs a WSH event-data capture",
	{protocol.InMarketDepth, protocol.ProtobufBody}:           "current sv225 account returned code 10092 before any depth row",
	{protocol.InMarketDepthL2, protocol.ProtobufBody}:         "needs an entitled exact protobuf L2 frame",
	{protocol.InNewsBulletins, protocol.ProtobufBody}:         "needs an exact protobuf bulletin frame",
	{protocol.InReceiveFA, protocol.ProtobufBody}:             "needs an exact protobuf requestFA response",
	{protocol.InRealTimeBars, protocol.ProtobufBody}:          "needs an exact market-hours protobuf frame",
	{protocol.InHistogramData, protocol.ProtobufBody}:         "needs an exact protobuf histogram frame",
	{protocol.InHistoricalDataUpdate, protocol.ProtobufBody}:  "needs an exact protobuf historical-update frame",
	{protocol.InMarketDepthReroute, protocol.ProtobufBody}:    "needs an exact protobuf market-depth reroute frame",
	{protocol.InHistoricalTicks, protocol.ProtobufBody}:       "needs an exact protobuf historical-ticks frame",
	{protocol.InHistoricalTicksBidAsk, protocol.ProtobufBody}: "needs an exact protobuf BID_ASK ticks frame",
	{protocol.InTickByTick, protocol.ProtobufBody}:            "needs an exact market-hours protobuf frame",
	{protocol.InOrderBound, protocol.ProtobufBody}:            "needs an exact protobuf order-bound frame",
	{protocol.InWSHMetaData, protocol.ProtobufBody}:           "needs an exact protobuf WSH metadata frame",
	{protocol.InWSHEventData, protocol.ProtobufBody}:          "needs an exact protobuf WSH event-data frame",
}

// TestInboundDecoderRawFrameCoverage enforces that the two catalogs above
// partition every registered decoder pair exactly: each pair is either
// attested by a cited raw live frame or explicitly pending, never both and
// never neither, and neither catalog names an unregistered pair.
func TestInboundDecoderRawFrameCoverage(t *testing.T) {
	registered := make(map[decoderAttestationKey]struct{}, len(inboundDecoders)+len(inboundProtobufDecoders))
	for id := range inboundDecoders {
		registered[decoderAttestationKey{id, protocol.ClassicBody}] = struct{}{}
	}
	for id := range inboundProtobufDecoders {
		registered[decoderAttestationKey{id, protocol.ProtobufBody}] = struct{}{}
	}
	if got, want := len(registered), 106; got != want {
		t.Fatalf("registered decoder pairs = %d, want %d", got, want)
	}
	if got, want := len(rawFrameAttested), 86; got != want {
		t.Fatalf("exact-attested decoder pairs = %d, want %d", got, want)
	}
	if got, want := len(pendingLiveAttestation), 20; got != want {
		t.Fatalf("pending decoder pairs = %d, want %d", got, want)
	}

	for key := range registered {
		_, attested := rawFrameAttested[key]
		_, pending := pendingLiveAttestation[key]

		switch {
		case attested && pending:
			t.Errorf("decoder pair (%d, %v) is in BOTH rawFrameAttested and pendingLiveAttestation; "+
				"a decoder is attested or pending, never both — delete the stale entry", key.msgID, key.encoding)
		case !attested && !pending:
			t.Errorf("decoder pair (%d, %v) has no raw-frame coverage entry.\n"+
				"    Write a capture-decode test that feeds a hardcoded live wire frame into DecodeBatch "+
				"and asserts the typed result, then add the pair to rawFrameAttested.\n"+
				"    If no live frame is capturable yet, add a justified pendingLiveAttestation entry "+
				"naming the capture that would provide it.", key.msgID, key.encoding)
		}
	}

	for key := range rawFrameAttested {
		if _, ok := registered[key]; !ok {
			t.Errorf("rawFrameAttested names unregistered decoder pair (%d, %v)", key.msgID, key.encoding)
		}
	}
	for key := range pendingLiveAttestation {
		if _, ok := registered[key]; !ok {
			t.Errorf("pendingLiveAttestation names unregistered decoder pair (%d, %v)", key.msgID, key.encoding)
		}
	}
}

// Keep the navigation ledger executable: renamed or deleted attestation tests
// must update the entry, including references to public replay tests.
func TestAttestationTestNamesExist(t *testing.T) {
	names := make(map[string]bool)
	for _, pattern := range []string{"*_test.go", "../../*_test.go"} {
		paths, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range paths {
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil {
					names[fn.Name.Name] = true
				}
			}
		}
	}
	for key, name := range rawFrameAttested {
		if !names[name] {
			t.Errorf("attestation %+v cites missing test %s", key, name)
		}
	}
}

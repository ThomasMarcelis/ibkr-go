package codec

import (
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
	{protocol.InTickPrice, protocol.ClassicBody}:             "TestCaptureDecode_TickPrice",
	{protocol.InTickSize, protocol.ClassicBody}:              "TestCaptureDecode_TickSize",
	{protocol.InOrderStatus, protocol.ClassicBody}:           "TestCaptureDecode_OrderStatusLive",
	{protocol.InErrMsg, protocol.ClassicBody}:                "TestCaptureDecode_APIError_2104",
	{protocol.InOpenOrder, protocol.ClassicBody}:             "TestCaptureDecode_OpenOrder",
	{protocol.InUpdateAccountValue, protocol.ClassicBody}:    "TestCaptureDecode_AccountUpdatesLive",
	{protocol.InUpdatePortfolio, protocol.ClassicBody}:       "TestCaptureDecode_AccountUpdatesLive",
	{protocol.InUpdateAccountTime, protocol.ClassicBody}:     "TestCaptureDecode_AccountUpdatesLive",
	{protocol.InNextValidID, protocol.ClassicBody}:           "TestCaptureDecode_NextValidID",
	{protocol.InContractData, protocol.ClassicBody}:          "TestCaptureDecode_ContractDetails",
	{protocol.InExecutionData, protocol.ClassicBody}:         "TestCaptureDecode_ExecutionDetailNativeTime",
	{protocol.InNewsBulletins, protocol.ClassicBody}:         "TestCaptureDecode_NewsBulletinLive",
	{protocol.InManagedAccounts, protocol.ClassicBody}:       "TestCaptureDecode_ManagedAccounts",
	{protocol.InHistoricalData, protocol.ClassicBody}:        "TestCaptureDecode_HistoricalData",
	{protocol.InBondContractData, protocol.ClassicBody}:      "TestCaptureDecode_BondContractDetails",
	{protocol.InScannerData, protocol.ClassicBody}:           "TestCaptureDecode_ScannerDataLive",
	{protocol.InTickOptionComputation, protocol.ClassicBody}: "TestCaptureDecode_TickOptionComputationLive",
	{protocol.InTickGeneric, protocol.ClassicBody}:           "TestCaptureDecode_QuoteAncillaryTicksLive",
	{protocol.InTickString, protocol.ClassicBody}:            "TestCaptureDecode_QuoteAncillaryTicksLive",
	{protocol.InTickReqParams, protocol.ClassicBody}:         "TestCaptureDecode_QuoteAncillaryTicksLive",
	{protocol.InCurrentTime, protocol.ClassicBody}:           "TestCaptureDecode_CurrentTimeLive",
	{protocol.InContractDataEnd, protocol.ClassicBody}:       "TestCaptureDecode_ContractDetailsEnd",
	{protocol.InOpenOrderEnd, protocol.ClassicBody}:          "TestCaptureDecode_OpenOrderEnd",
	{protocol.InAccountDownloadEnd, protocol.ClassicBody}:    "TestCaptureDecode_AccountUpdatesLive",
	{protocol.InExecutionDataEnd, protocol.ClassicBody}:      "TestCaptureDecode_ExecutionsEndLive",
	{protocol.InTickSnapshotEnd, protocol.ClassicBody}:       "TestCaptureDecode_TickSnapshotEnd",
	{protocol.InMarketDataType, protocol.ClassicBody}:        "TestCaptureDecode_MarketDataType",
	{protocol.InCommissionReport, protocol.ClassicBody}:      "TestCaptureDecode_CommissionAndFeesLive",
	{protocol.InPositionData, protocol.ClassicBody}:          "TestCaptureDecode_Position",
	{protocol.InPositionEnd, protocol.ClassicBody}:           "TestCaptureDecode_PositionEnd",
	{protocol.InAccountSummary, protocol.ClassicBody}:        "TestCaptureDecode_AccountSummaryValue",
	{protocol.InAccountSummaryEnd, protocol.ClassicBody}:     "TestCaptureDecode_AccountSummaryEnd",
	{protocol.InDisplayGroupList, protocol.ClassicBody}:      "TestCaptureDecode_DisplayGroupsLive",
	{protocol.InDisplayGroupUpdated, protocol.ClassicBody}:   "TestCaptureDecode_DisplayGroupsLive",
	{protocol.InPositionMulti, protocol.ClassicBody}:         "TestCaptureDecode_PositionMultiServerVersion206",
	{protocol.InPositionMultiEnd, protocol.ClassicBody}:      "TestCaptureDecode_PositionMultiEndLive",
	{protocol.InAccountUpdateMulti, protocol.ClassicBody}:    "TestCaptureDecode_AccountUpdatesMultiLive",
	{protocol.InAccountUpdateMultiEnd, protocol.ClassicBody}: "TestCaptureDecode_AccountUpdatesMultiLive",
	{protocol.InSecDefOptParams, protocol.ClassicBody}:       "TestCaptureDecode_SecDefOptParamsLive",
	{protocol.InSecDefOptParamsEnd, protocol.ClassicBody}:    "TestCaptureDecode_SecDefOptParamsEndLive",
	{protocol.InSoftDollarTiers, protocol.ClassicBody}:       "TestCaptureDecode_SoftDollarTiersLive",
	{protocol.InFamilyCodes, protocol.ClassicBody}:           "TestCaptureDecode_FamilyCodesLive",
	{protocol.InSymbolSamples, protocol.ClassicBody}:         "TestMatchingSymbols",
	{protocol.InMktDepthExchanges, protocol.ClassicBody}:     "TestMktDepthExchanges",
	{protocol.InSmartComponents, protocol.ClassicBody}:       "TestSmartComponents",
	{protocol.InNewsArticle, protocol.ClassicBody}:           "TestCaptureDecode_HistoricalNewsFlowLive",
	{protocol.InTickNews, protocol.ClassicBody}:              "TestCaptureDecode_TickNewsLive",
	{protocol.InNewsProviders, protocol.ClassicBody}:         "TestCaptureDecode_NewsProvidersLive",
	{protocol.InHistoricalNews, protocol.ClassicBody}:        "TestCaptureDecode_HistoricalNewsFlowLive",
	{protocol.InHistoricalNewsEnd, protocol.ClassicBody}:     "TestCaptureDecode_HistoricalNewsFlowLive",
	{protocol.InHeadTimestamp, protocol.ClassicBody}:         "TestCaptureDecode_HeadTimestampLive",
	{protocol.InHistogramData, protocol.ClassicBody}:         "TestHistogramData",
	{protocol.InMarketDataReroute, protocol.ClassicBody}:     "TestCaptureDecode_MarketDataReroutesLive",
	{protocol.InMarketDepthReroute, protocol.ClassicBody}:    "TestCaptureDecode_MarketDataReroutesLive",
	{protocol.InMarketRule, protocol.ClassicBody}:            "TestCaptureDecode_MarketRuleLive",
	{protocol.InPnL, protocol.ClassicBody}:                   "TestCaptureDecode_PnLLive",
	{protocol.InHistoricalTicksLast, protocol.ClassicBody}:   "TestHistoricalTicksTrades",
	{protocol.InCompletedOrder, protocol.ClassicBody}:        "TestCaptureDecode_CompletedOrderTrailLimitLive",
	{protocol.InCompletedOrderEnd, protocol.ClassicBody}:     "TestCaptureDecode_CompletedOrderEndLive",
	{protocol.InHistoricalSchedule, protocol.ClassicBody}:    "TestCaptureDecode_HistoricalSchedule",
	{protocol.InUserInfo, protocol.ClassicBody}:              "TestDecodeUserInfoLiveFrame",
	{protocol.InHistoricalDataEnd, protocol.ClassicBody}:     "TestCaptureDecode_HistoricalDataEndLive",
	{protocol.InCurrentTimeInMillis, protocol.ClassicBody}:   "TestCaptureDecode_CurrentTimeMillis",

	{protocol.InTickPrice, protocol.ProtobufBody}:             "TestDecodeMarketDataProto206LiveVectors",
	{protocol.InTickSize, protocol.ProtobufBody}:              "TestDecodeMarketDataProto206LiveVectors",
	{protocol.InOrderStatus, protocol.ProtobufBody}:           "TestDecodeServer203OrderCallbacks",
	{protocol.InErrMsg, protocol.ProtobufBody}:                "TestDecodeProtobufCommissionAndErrorSchemas",
	{protocol.InOpenOrder, protocol.ProtobufBody}:             "TestDecodeServer203OrderCallbacks",
	{protocol.InUpdateAccountValue, protocol.ProtobufBody}:    "TestDecodeAccountProto207LiveVectors",
	{protocol.InUpdatePortfolio, protocol.ProtobufBody}:       "TestDecodeAccountProto207LiveVectors",
	{protocol.InUpdateAccountTime, protocol.ProtobufBody}:     "TestDecodeAccountProto207LiveVectors",
	{protocol.InNextValidID, protocol.ProtobufBody}:           "TestDecodeSessionProto213LiveVectors",
	{protocol.InContractData, protocol.ProtobufBody}:          "TestDecodeServer205ContractDetailsTypeMatrix",
	{protocol.InExecutionData, protocol.ProtobufBody}:         "TestDecodeServer201ExecutionDetailLiveVector",
	{protocol.InMarketDepth, protocol.ProtobufBody}:           "TestDecodeMarketDataProto206LiveVectors",
	{protocol.InManagedAccounts, protocol.ProtobufBody}:       "TestDecodeAccountProto207LiveVectors",
	{protocol.InHistoricalData, protocol.ProtobufBody}:        "TestHistoricalProto208LiveVectors",
	{protocol.InBondContractData, protocol.ProtobufBody}:      "TestDecodeServer205ContractDetailsTypeMatrix",
	{protocol.InScannerData, protocol.ProtobufBody}:           "TestDecodeScannerPnLProto210LiveVectors",
	{protocol.InTickOptionComputation, protocol.ProtobufBody}: "TestDecodeMarketDataProto206LiveVectors",
	{protocol.InTickGeneric, protocol.ProtobufBody}:           "TestDecodeMarketDataProto206LiveVectors",
	{protocol.InTickString, protocol.ProtobufBody}:            "TestDecodeMarketDataProto206LiveVectors",
	{protocol.InTickReqParams, protocol.ProtobufBody}:         "TestDecodeMarketDataProto206LiveVectors",
	{protocol.InCurrentTime, protocol.ProtobufBody}:           "TestDecodeSessionProto213LiveVectors",
	{protocol.InContractDataEnd, protocol.ProtobufBody}:       "TestDecodeServer205LiveIneligibilityReasonsAndEnds",
	{protocol.InOpenOrderEnd, protocol.ProtobufBody}:          "TestDecodeServer203OrderCallbacks",
	{protocol.InAccountDownloadEnd, protocol.ProtobufBody}:    "TestDecodeAccountProto207LiveVectors",
	{protocol.InExecutionDataEnd, protocol.ProtobufBody}:      "TestDecodeServer201ExecutionEndLiveVector",
	{protocol.InTickSnapshotEnd, protocol.ProtobufBody}:       "TestDecodeMarketDataProto206LiveVectors",
	{protocol.InMarketDataType, protocol.ProtobufBody}:        "TestDecodeMarketDataProto206LiveVectors",
	{protocol.InPositionData, protocol.ProtobufBody}:          "TestDecodeAccountProto207LiveVectors",
	{protocol.InPositionEnd, protocol.ProtobufBody}:           "TestDecodeAccountProto207LiveVectors",
	{protocol.InAccountSummary, protocol.ProtobufBody}:        "TestDecodeAccountProto207LiveVectors",
	{protocol.InAccountSummaryEnd, protocol.ProtobufBody}:     "TestDecodeAccountProto207LiveVectors",
	{protocol.InDisplayGroupList, protocol.ProtobufBody}:      "TestDecodeSessionProto213LiveVectors",
	{protocol.InDisplayGroupUpdated, protocol.ProtobufBody}:   "TestDecodeSessionProto213LiveVectors",
	{protocol.InPositionMulti, protocol.ProtobufBody}:         "TestDecodeAccountProto207LiveVectors",
	{protocol.InPositionMultiEnd, protocol.ProtobufBody}:      "TestDecodeAccountProto207LiveVectors",
	{protocol.InAccountUpdateMulti, protocol.ProtobufBody}:    "TestDecodeAccountProto207LiveVectors",
	{protocol.InAccountUpdateMultiEnd, protocol.ProtobufBody}: "TestDecodeAccountProto207LiveVectors",
	{protocol.InSecDefOptParamsEnd, protocol.ProtobufBody}:    "TestDecodeReferenceProto212LiveVectors",
	{protocol.InSoftDollarTiers, protocol.ProtobufBody}:       "TestDecodeReferenceProto212LiveVectors",
	{protocol.InFamilyCodes, protocol.ProtobufBody}:           "TestDecodeReferenceProto212LiveVectors",
	{protocol.InSmartComponents, protocol.ProtobufBody}:       "TestDecodeReferenceProto212LiveVectors",
	{protocol.InNewsProviders, protocol.ProtobufBody}:         "TestDecodeNewsProto209LiveVectors",
	{protocol.InHistoricalNews, protocol.ProtobufBody}:        "TestDecodeNewsProto209LiveVectors",
	{protocol.InHistoricalNewsEnd, protocol.ProtobufBody}:     "TestDecodeNewsProto209LiveVectors",
	{protocol.InMarketRule, protocol.ProtobufBody}:            "TestDecodeReferenceProto212LiveVectors",
	{protocol.InMarketDataReroute, protocol.ProtobufBody}:     "TestCaptureDecode_MarketDataRerouteProto225Live",
	{protocol.InPnL, protocol.ProtobufBody}:                   "TestDecodeScannerPnLProto210LiveVectors",
	{protocol.InPnLSingle, protocol.ProtobufBody}:             "TestDecodeScannerPnLProto210LiveVectors",
	{protocol.InCompletedOrder, protocol.ProtobufBody}:        "TestDecodeServer204CompletedOrders",
	{protocol.InCompletedOrderEnd, protocol.ProtobufBody}:     "TestDecodeServer204CompletedOrders",
	{protocol.InUserInfo, protocol.ProtobufBody}:              "TestDecodeReferenceProto212LiveVectors",
	{protocol.InHistoricalDataEnd, protocol.ProtobufBody}:     "TestHistoricalProto208LiveVectors",
	{protocol.InCurrentTimeInMillis, protocol.ProtobufBody}:   "TestDecodeSessionProto213LiveVectors",
}

// pendingLiveAttestation maps a decoder pair to the evidence still needed.
var pendingLiveAttestation = map[decoderAttestationKey]string{
	{protocol.InMarketDepth, protocol.ClassicBody}:            "needs an exact classic market-depth capture",
	{protocol.InMarketDepthL2, protocol.ClassicBody}:          "needs an entitled classic L2 capture",
	{protocol.InScannerParameters, protocol.ClassicBody}:      "the 1.8 MB live XML frame is not checked in",
	{protocol.InReceiveFA, protocol.ClassicBody}:              "needs a requestFA capture",
	{protocol.InTickEFP, protocol.ClassicBody}:                "needs entitled single-stock-future data",
	{protocol.InRealTimeBars, protocol.ClassicBody}:           "needs a market-hours 5s-bar capture",
	{protocol.InDeltaNeutralValidation, protocol.ClassicBody}: "needs a successful BAG delta-neutral capture",
	{protocol.InHistoricalDataUpdate, protocol.ClassicBody}:   "needs an exact market-hours message-90 capture",
	{protocol.InPnLSingle, protocol.ClassicBody}:              "needs a reqPnLSingle capture",
	{protocol.InHistoricalTicks, protocol.ClassicBody}:        "needs a MIDPOINT historical-ticks capture",
	{protocol.InHistoricalTicksBidAsk, protocol.ClassicBody}:  "needs a BID_ASK historical-ticks capture",
	{protocol.InTickByTick, protocol.ClassicBody}:             "needs a market-hours tick-by-tick capture",
	{protocol.InOrderBound, protocol.ClassicBody}:             "needs a client-0 manual paper-TWS order capture",
	{protocol.InWSHMetaData, protocol.ClassicBody}:            "needs a WSH metadata capture",
	{protocol.InWSHEventData, protocol.ClassicBody}:           "needs a WSH event-data capture",

	{protocol.InMarketDepthL2, protocol.ProtobufBody}:         "needs an entitled exact protobuf L2 frame",
	{protocol.InNewsBulletins, protocol.ProtobufBody}:         "needs an exact protobuf bulletin frame",
	{protocol.InScannerParameters, protocol.ProtobufBody}:     "needs an exact checked-in protobuf frame",
	{protocol.InReceiveFA, protocol.ProtobufBody}:             "needs an exact protobuf requestFA response",
	{protocol.InRealTimeBars, protocol.ProtobufBody}:          "needs an exact market-hours protobuf frame",
	{protocol.InCommissionReport, protocol.ProtobufBody}:      "needs an independently enveloped exact protobuf frame",
	{protocol.InSecDefOptParams, protocol.ProtobufBody}:       "the reduced source-law vector is not exact capture evidence",
	{protocol.InSymbolSamples, protocol.ProtobufBody}:         "the re-enveloped vector is not exact capture evidence",
	{protocol.InMktDepthExchanges, protocol.ProtobufBody}:     "the reduced source-law vector is not exact capture evidence",
	{protocol.InNewsArticle, protocol.ProtobufBody}:           "needs an exact protobuf news-article frame",
	{protocol.InTickNews, protocol.ProtobufBody}:              "needs an exact protobuf tick-news frame",
	{protocol.InHeadTimestamp, protocol.ProtobufBody}:         "needs an exact protobuf head-timestamp frame",
	{protocol.InHistogramData, protocol.ProtobufBody}:         "needs an exact protobuf histogram frame",
	{protocol.InHistoricalDataUpdate, protocol.ProtobufBody}:  "needs an exact protobuf historical-update frame",
	{protocol.InMarketDepthReroute, protocol.ProtobufBody}:    "needs an exact protobuf market-depth reroute frame",
	{protocol.InHistoricalTicks, protocol.ProtobufBody}:       "needs an exact protobuf historical-ticks frame",
	{protocol.InHistoricalTicksBidAsk, protocol.ProtobufBody}: "needs an exact protobuf BID_ASK ticks frame",
	{protocol.InHistoricalTicksLast, protocol.ProtobufBody}:   "needs an exact protobuf last-ticks frame",
	{protocol.InTickByTick, protocol.ProtobufBody}:            "needs an exact market-hours protobuf frame",
	{protocol.InOrderBound, protocol.ProtobufBody}:            "needs an exact protobuf order-bound frame",
	{protocol.InWSHMetaData, protocol.ProtobufBody}:           "needs an exact protobuf WSH metadata frame",
	{protocol.InWSHEventData, protocol.ProtobufBody}:          "needs an exact protobuf WSH event-data frame",
	{protocol.InHistoricalSchedule, protocol.ProtobufBody}:    "needs an exact protobuf schedule frame",
	{protocol.InConfig, protocol.ProtobufBody}:                "the reduced config vector is not exact capture evidence",
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
	if got, want := len(registered), 155; got != want {
		t.Fatalf("registered decoder pairs = %d, want %d", got, want)
	}
	if got, want := len(rawFrameAttested), 116; got != want {
		t.Fatalf("exact-attested decoder pairs = %d, want %d", got, want)
	}
	if got, want := len(pendingLiveAttestation), 39; got != want {
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

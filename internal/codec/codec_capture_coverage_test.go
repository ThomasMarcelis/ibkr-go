package codec

import (
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/protocol"
)

// Raw-frame capture-coverage gate and inbound evidence ledger.
//
// Every decoder registered in inboundDecoders must be attested by at least one
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

// rawFrameAttested maps a decoder's msg_id to a representative direct decode
// or exact raw public replay test. The names are navigation aids; the
// executable assertions and their cited live frames remain the evidence.
var rawFrameAttested = map[int]string{
	protocol.InTickPrice:             "TestCaptureDecode_TickPrice",
	protocol.InTickSize:              "TestCaptureDecode_TickSize",
	protocol.InMarketDepth:           "TestDecodeMarketDataProto206LiveVectors",
	protocol.InErrMsg:                "TestCaptureDecode_APIError_2104",
	protocol.InOpenOrder:             "TestCaptureDecode_OpenOrder",
	protocol.InNextValidID:           "TestCaptureDecode_NextValidID",
	protocol.InContractData:          "TestCaptureDecode_ContractDetails",
	protocol.InBondContractData:      "TestCaptureDecode_BondContractDetails",
	protocol.InExecutionData:         "TestCaptureDecode_ExecutionDetailNativeTime",
	protocol.InExecutionDataEnd:      "TestCaptureDecode_ExecutionsEndLive",
	protocol.InCommissionReport:      "TestCaptureDecode_CommissionAndFeesLive",
	protocol.InManagedAccounts:       "TestCaptureDecode_ManagedAccounts",
	protocol.InHistoricalData:        "TestCaptureDecode_HistoricalData",
	protocol.InTickOptionComputation: "TestCaptureDecode_TickOptionComputationLive",
	protocol.InContractDataEnd:       "TestCaptureDecode_ContractDetailsEnd",
	protocol.InOpenOrderEnd:          "TestCaptureDecode_OpenOrderEnd",
	protocol.InTickSnapshotEnd:       "TestCaptureDecode_TickSnapshotEnd",
	protocol.InMarketDataType:        "TestCaptureDecode_MarketDataType",
	protocol.InPositionData:          "TestCaptureDecode_Position",
	protocol.InPositionEnd:           "TestCaptureDecode_PositionEnd",
	protocol.InAccountSummary:        "TestCaptureDecode_AccountSummaryValue",
	protocol.InAccountSummaryEnd:     "TestCaptureDecode_AccountSummaryEnd",
	protocol.InSecDefOptParams:       "TestCaptureDecode_SecDefOptParamsLive",
	protocol.InMarketRule:            "TestCaptureDecode_MarketRuleLive",
	protocol.InNewsBulletins:         "TestCaptureDecode_NewsBulletinLive",
	protocol.InCompletedOrder:        "TestCaptureDecode_CompletedOrderTrailLimitLive",
	protocol.InHistoricalSchedule:    "TestCaptureDecode_HistoricalSchedule",
	protocol.InUserInfo:              "TestDecodeUserInfoLiveFrame",
	protocol.InHistoricalDataEnd:     "TestCaptureDecode_HistoricalDataEndLive",
	protocol.InTickGeneric:           "TestCaptureDecode_QuoteAncillaryTicksLive",
	protocol.InTickString:            "TestCaptureDecode_QuoteAncillaryTicksLive",
	protocol.InTickReqParams:         "TestCaptureDecode_QuoteAncillaryTicksLive",
	protocol.InMarketDataReroute:     "TestCaptureDecode_MarketDataReroutesLive",
	protocol.InMarketDepthReroute:    "TestCaptureDecode_MarketDataReroutesLive",
	protocol.InTickNews:              "TestCaptureDecode_TickNewsLive",
	protocol.InScannerData:           "TestCaptureDecode_ScannerDataLive",
	protocol.InCurrentTimeInMillis:   "TestCaptureDecode_CurrentTimeMillis",
	protocol.InCurrentTime:           "TestCaptureDecode_CurrentTimeLive",
	protocol.InPositionMultiEnd:      "TestCaptureDecode_PositionMultiEndLive",
	protocol.InPositionMulti:         "TestCaptureDecode_PositionMultiServerVersion206",
	protocol.InFamilyCodes:           "TestCaptureDecode_FamilyCodesLive",
	protocol.InNewsProviders:         "TestCaptureDecode_NewsProvidersLive",
	protocol.InSymbolSamples:         "TestMatchingSymbols",
	protocol.InMktDepthExchanges:     "TestMktDepthExchanges",
	protocol.InSmartComponents:       "TestSmartComponents",
	protocol.InHistogramData:         "TestHistogramData",
	protocol.InPnL:                   "TestCaptureDecode_PnLLive",
	protocol.InOrderStatus:           "TestCaptureDecode_OrderStatusLive",
	protocol.InUpdateAccountValue:    "TestCaptureDecode_AccountUpdatesLive",
	protocol.InUpdatePortfolio:       "TestCaptureDecode_AccountUpdatesLive",
	protocol.InUpdateAccountTime:     "TestCaptureDecode_AccountUpdatesLive",
	protocol.InAccountDownloadEnd:    "TestCaptureDecode_AccountUpdatesLive",
	protocol.InDisplayGroupList:      "TestCaptureDecode_DisplayGroupsLive",
	protocol.InDisplayGroupUpdated:   "TestCaptureDecode_DisplayGroupsLive",
	protocol.InAccountUpdateMulti:    "TestCaptureDecode_AccountUpdatesMultiLive",
	protocol.InAccountUpdateMultiEnd: "TestCaptureDecode_AccountUpdatesMultiLive",
	protocol.InSecDefOptParamsEnd:    "TestCaptureDecode_SecDefOptParamsEndLive",
	protocol.InSoftDollarTiers:       "TestCaptureDecode_SoftDollarTiersLive",
	protocol.InNewsArticle:           "TestCaptureDecode_HistoricalNewsFlowLive",
	protocol.InHistoricalNews:        "TestCaptureDecode_HistoricalNewsFlowLive",
	protocol.InHistoricalNewsEnd:     "TestCaptureDecode_HistoricalNewsFlowLive",
	protocol.InHeadTimestamp:         "TestCaptureDecode_HeadTimestampLive",
	protocol.InCompletedOrderEnd:     "TestCaptureDecode_CompletedOrderEndLive",
	protocol.InHistoricalTicksLast:   "TestHistoricalTicksTrades",
}

// pendingLiveAttestation maps a decoder's msg_id to a one-line reason it has no
// live-attested raw frame yet and the capture that would provide one. An entry
// here is a promise to capture, not a license to skip: move the id to
// rawFrameAttested the moment a cited live frame exists.
var pendingLiveAttestation = map[int]string{
	protocol.InMarketDepthL2:         "exact-sv206 protobuf dispatch/schema are official-source-attested; positive raw 213 remains pending because the local capture account lacked L2 entitlement",
	protocol.InReceiveFA:             "needs a requestFA capture (FA account entitlement)",
	protocol.InScannerParameters:     "live sv206 response is attested by capture e50db8964130d14bcf8c5d02fe8c1383d15f55daf58363ab1433b999ccd79660, but its 1.8 MB XML frame is intentionally not checked in",
	protocol.InRealTimeBars:          "needs a reqRealTimeBars 5s-bar capture (market hours)",
	protocol.InHistoricalDataUpdate:  "source-referenced from the official client library; live attestation pending (needs a market-hours keepUpToDate reqHistoricalData capture) — see captures/v1/WIRE_TRUTH.md",
	protocol.InPnLSingle:             "needs a reqPnLSingle capture",
	protocol.InHistoricalTicks:       "needs a reqHistoricalTicks whatToShow=MIDPOINT capture",
	protocol.InHistoricalTicksBidAsk: "needs a reqHistoricalTicks whatToShow=BID_ASK capture",
	protocol.InTickByTick:            "needs a reqTickByTickData capture (market hours)",
	protocol.InReplaceFAEnd:          "source-referenced decoder coverage only — needs a live replaceFAEnd frame from an FA-entitled account",
	protocol.InWSHMetaData:           "needs a reqWSHMetaData capture (WSH entitlement)",
	protocol.InWSHEventData:          "needs a reqWSHEventData capture (WSH entitlement)",
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

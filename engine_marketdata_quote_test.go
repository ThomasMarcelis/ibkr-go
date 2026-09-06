package ibkr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/transport"
)

func TestQuoteRouteFollowsLiveProtobufReroute225(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	e.serverVersion = 225
	e.nextReqID = 1
	sub := installObservedQuoteRoute(t, e, QuoteRequest{Contract: Contract{
		Symbol: "IBM", SecType: SecTypeCFD, Exchange: "SMART", Currency: "USD",
	}})
	_ = readObservedFrame(t, peer)

	// Capture 20260825T201807Z-live_cfd_quote_reroute_v201_positive,
	// events.jsonl sha256
	// ca8fbdf11d260066fb7cd1c3d60e6e44808a54bf6a8fc678f3597bd71a666f1c.
	reroute, err := codec.Decode(225, []byte{
		0x00, 0x00, 0x01, 0x23, 0x08, 0x01, 0x10, 0xfa,
		0x40, 0x1a, 0x05, 0x53, 0x4d, 0x41, 0x52, 0x54,
	})
	if err != nil {
		t.Fatal(err)
	}
	e.handleIncoming(reroute)

	active := e.keyed[1]
	routedRequest, ok := active.request.(codec.QuoteRequest)
	if !ok {
		t.Fatalf("route request = %T, want codec.QuoteRequest", active.request)
	}
	if !reflect.DeepEqual(routedRequest.Contract, codec.Contract{ConID: 8314, Exchange: "SMART"}) {
		t.Fatalf("rerouted contract = %+v", routedRequest.Contract)
	}
	want, err := codec.Encode(225, routedRequest)
	if err != nil {
		t.Fatal(err)
	}
	if got := readObservedFrame(t, peer); !bytes.Equal(got, want) {
		t.Fatalf("rerouted protobuf request = %x, want %x", got, want)
	}

	sub.Close()
	(<-e.cmds)()
}

func TestQuoteOddLotGenericTickServerVersionBoundary(t *testing.T) {
	// API 10.48.01 EClient.MIN_SERVER_VER_ODD_LOT_BID_ASK_QUOTES is 225;
	// MarketDataSamplesProto requests the corresponding tick family with 787.
	tests := []struct {
		serverVersion int
		wantErr       bool
	}{
		{serverVersion: 224, wantErr: true},
		{serverVersion: 225},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.serverVersion), func(t *testing.T) {
			e, peer := newObservedMarketDataEngine(t)
			e.serverVersion = tt.serverVersion
			e.nextReqID = 7871
			result := make(chan struct {
				sub *Subscription[QuoteUpdate]
				err error
			}, 1)
			go func() {
				sub, err := e.SubscribeQuotes(context.Background(), QuoteRequest{
					Contract:     Stock("AAPL"),
					GenericTicks: []GenericTick{GenericTickOddLotBidAsk},
				})
				result <- struct {
					sub *Subscription[QuoteUpdate]
					err error
				}{sub: sub, err: err}
			}()
			(<-e.cmds)()
			got := <-result
			if tt.wantErr {
				if !errors.Is(got.err, ErrUnsupportedServerVersion) {
					t.Fatalf("SubscribeQuotes() error = %v, want ErrUnsupportedServerVersion", got.err)
				}
				if e.nextReqID != 7871 {
					t.Fatalf("next request ID = %d, want no allocation", e.nextReqID)
				}
				return
			}
			if got.err != nil {
				t.Fatal(got.err)
			}
			if payload := readObservedFrame(t, peer); !bytes.Contains(payload, []byte("787")) {
				t.Fatalf("market data request = %x, want generic tick 787", payload)
			}
			got.sub.Close()
			(<-e.cmds)()
		})
	}
}

func TestSmartDepthExchangeNoticePreservesLiveRoute(t *testing.T) {
	// captures/20260824T202754Z-market_depth_aapl_smart, server_version 225,
	// events.jsonl sha256
	// 9f37f9d5ce3f78cfef6ef3a77749deffe5ef197533baeb9734a1b69d8f6c8d89.
	// The exact request-scoped code 2152 frame must reach the keyed depth route
	// without closing it: the official notice may precede valid rows when any
	// listed depth venue is available.
	e, peer := newObservedMarketDataEngine(t)
	e.serverVersion = 225
	e.nextReqID = 1
	sub := installObservedDepthRoute(t, e, MarketDepthRequest{
		Contract: Contract{Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD"},
		NumRows:  5, IsSmartDepth: true,
	})
	_ = readObservedFrame(t, peer)

	message, err := codec.Decode(225, liveCapturedFrame(t, "AAAA4wAAAMwIARCIntWrgzQY6BAi0AFFeGNoYW5nZXMgLSBUb3A6IElCRU9TOyBPVkVSTklHSFQ7IE5lZWQgYWRkaXRpb25hbCBtYXJrZXQgZGF0YSBwZXJtaXNzaW9ucyAtIERlcHRoOiBCQVRTOyBOQVNEQVE7IEFSQ0E7IEJFWDsgTllTRTsgSUVYOyBUb3A6IEJZWDsgUEVBUkw7IEFNRVg7IFQyNFg7IE1FTVg7IEVER0VBOyBUWFNFOyBDSFg7IE5ZU0VOQVQ7IFBTWDsgTFRTRTsgSVNFOyBEUkNURURHRTsg"))
	if err != nil {
		t.Fatalf("decode exact live SMART-depth exchange notice: %v", err)
	}
	e.handleIncoming(message)

	if _, ok := e.keyed[1]; !ok {
		t.Fatal("live code-2152 SMART-depth exchange notice deleted the route")
	}
	event := <-sub.Events() // Started
	event = <-sub.Events()
	if event.Kind != StreamNotice || event.Notice == nil || event.Notice.Code != ErrCodeSmartDepthExchanges || event.Notice.Message == "" {
		t.Fatalf("market-depth stream event = %+v, want code-2152 availability notice", event)
	}
	select {
	case duplicate := <-e.SessionEvents():
		t.Fatalf("request-scoped notice was duplicated as session event: %+v", duplicate)
	default:
	}
	if err := sub.Err(); err != nil {
		t.Fatalf("market-depth subscription error after notice = %v", err)
	}

	sub.Close()
	(<-e.cmds)()
	wantCancel, err := codec.Encode(225, codec.CancelMarketDepth{ReqID: 1, IsSmartDepth: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := readObservedFrame(t, peer); !bytes.Equal(got, wantCancel) {
		t.Fatalf("market-depth cancel = %x, want %x", got, wantCancel)
	}
	if err := sub.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
}

func TestRegulatorySnapshotErrorPreservesDefinitiveRejection(t *testing.T) {
	t.Parallel()

	apiErr := &APIError{Code: ErrCodeMarketDataNotSubscribed, OpKind: OpQuotes}
	if got := regulatorySnapshotError(7, 9, apiErr); got != apiErr {
		t.Fatalf("regulatorySnapshotError(APIError) = %v, want unchanged rejection", got)
	}

	got := regulatorySnapshotError(7, 9, context.DeadlineExceeded)
	if !errors.Is(got, ErrRegulatorySnapshotUncertain) || !errors.Is(got, context.DeadlineExceeded) {
		t.Fatalf("regulatorySnapshotError(deadline) = %v, want uncertainty and deadline", got)
	}
	uncertain, ok := errors.AsType[*RegulatorySnapshotUncertainError](got)
	if !ok || uncertain.RequestID != 7 || uncertain.ConnectionSeq != 9 {
		t.Fatalf("regulatory uncertainty identity = %#v, %t", uncertain, ok)
	}
	if IsRetryable(got) {
		t.Fatal("uncertain fee-bearing request is retryable")
	}
}

func TestQuoteRoutePreservesParameterPresenceAndPrecision(t *testing.T) {
	e := newBenchEngine(t)
	e.serverVersion = 225
	e.nextReqID = 1
	sub := installQuoteRoute(t, e)
	closeInstalledQuoteRoute(t, e, sub)

	// Capture 20260824T202841Z-quote_stream_aapl, server_version 225,
	// events.jsonl SHA-256
	// 5ca580636aa0fbd11781fa6a4d85c4c8f8f78ad1139ef7c577e3f11363d219e3.
	msg, err := codec.Decode(225, liveCapturedFrame(t, "AAAAKgAAARkIARIEMC4wMRoGOWMwMDAxIAQqCDAuMDAwMDAxMggwLjAwMDAwMQ=="))
	if err != nil {
		t.Fatal(err)
	}
	e.handleIncoming(msg)
	update := nextQuoteUpdate(t, sub)
	parameters := update.Parameters
	if update.Kind != QuoteUpdateParameters || parameters == nil ||
		parameters.SnapshotPermissions == nil || *parameters.SnapshotPermissions != 4 ||
		parameters.LastPricePrecision == nil || parameters.LastPricePrecision.String() != "0.000001" ||
		parameters.LastSizePrecision == nil || parameters.LastSizePrecision.String() != "0.000001" {
		t.Fatalf("parameters = %+v", parameters)
	}

	e.handleIncoming(codec.TickReqParams{ReqID: 1})
	absent := nextQuoteUpdate(t, sub).Parameters
	if absent == nil || absent.SnapshotPermissions != nil || absent.LastPricePrecision != nil || absent.LastSizePrecision != nil {
		t.Fatalf("omitted parameters = %+v, want nil presence fields", absent)
	}
}

func TestMarketDataRoutesPreserveOfficialUnavailableSizes(t *testing.T) {
	t.Run("quote size", func(t *testing.T) {
		e := newBenchEngine(t)
		e.serverVersion = 225
		e.nextReqID = 1
		sub := installQuoteRoute(t, e)
		closeInstalledQuoteRoute(t, e, sub)

		// API 10.48.01 maps an omitted TickSize.size to UNSET_DECIMAL.
		e.handleIncoming(codec.TickSize{ReqID: 1, TickType: 0})
		update := nextQuoteUpdate(t, sub)
		if update.Kind != QuoteUpdateSizeTick || update.Changed != 0 ||
			update.SizeTick == nil || update.SizeTick.Size != nil || update.Snapshot.Available != 0 {
			t.Fatalf("unavailable size update = %+v", update)
		}
	})

	tests := []struct {
		name    string
		convert func() (DepthRow, error)
	}{
		{
			"depth",
			func() (DepthRow, error) {
				return fromCodecMarketDepth(codec.MarketDepthUpdate{ReqID: 20614, Price: "0"})
			},
		},
		{
			"depth L2",
			func() (DepthRow, error) {
				return fromCodecMarketDepthL2(codec.MarketDepthL2Update{ReqID: 20614, Price: "0"})
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			row, err := tc.convert()
			if err != nil {
				t.Fatal(err)
			}
			if row.Size != nil || !row.Price.IsZero() {
				t.Fatalf("unavailable depth size = %+v", row)
			}
		})
	}
}

func newObservedMarketDataEngine(t *testing.T) (*engine, net.Conn) {
	t.Helper()
	peer, client := net.Pipe()
	cfg := defaultConfig()
	cfg.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	tr := transport.New(client, cfg.logger, 0)
	e := &engine{
		cfg: cfg, cmds: make(chan func(), 8), incoming: make(chan actorInput, 8),
		transportErr: make(chan transportLoss, 1), done: make(chan struct{}),
		events: newObserver[Event](cfg.eventBuffer), transport: tr,
		transportGeneration: 1,
		bootstrap:           bootstrapState{readyReported: true},
		serverVersion:       225, keyed: make(map[int]*route), singletons: make(map[string]*route),
		orders:               make(map[int64]*orderRoute),
		execDeliveries:       make(map[string]*execDelivery),
		malformedInboundSeen: make(map[int]struct{}),
		unknownInboundSeen:   make(map[int]struct{}),
		snapshot:             Snapshot{State: StateReady, ConnectionSeq: 1},
	}
	t.Cleanup(func() {
		_ = tr.Close()
		_ = peer.Close()
		_ = tr.Wait()
	})
	return e, peer
}

func installObservedQuoteRoute(t *testing.T, e *engine, req QuoteRequest, opts ...SubscriptionOption) *Subscription[QuoteUpdate] {
	t.Helper()
	result := make(chan *Subscription[QuoteUpdate], 1)
	go func() {
		sub, err := e.subscribeQuotes(context.Background(), req, false, false, opts...)
		if err != nil {
			t.Errorf("subscribeQuotes: %v", err)
		}
		result <- sub
	}()
	(<-e.cmds)()
	return <-result
}

func installObservedDepthRoute(t *testing.T, e *engine, req MarketDepthRequest) *Subscription[DepthRow] {
	t.Helper()
	result := make(chan *Subscription[DepthRow], 1)
	go func() {
		sub, err := e.SubscribeMarketDepth(context.Background(), req)
		if err != nil {
			t.Errorf("SubscribeMarketDepth: %v", err)
		}
		result <- sub
	}()
	(<-e.cmds)()
	return <-result
}

func readObservedFrame(t *testing.T, peer net.Conn) []byte {
	t.Helper()
	payload, err := transport.ReadOneFrame(peer, time.Now().Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func TestSlowQuoteConsumerDoesNotAffectSiblingRoute(t *testing.T) {
	// captures/20260824T202345Z-api_duplicate_quote_subscriptions_aapl,
	// server_version 225, events.jsonl sha256
	// 1fbb60beec41483729e2f9e7c96b1bfdd89649810ffdc5e7e4a4077c1eb8b290.
	// These exact frames were delivered to two independent AAPL subscriptions.
	// One stalled consumer must close only its own route while its sibling still
	// receives the complete delayed bid/ask sequence.
	e := newBenchEngine(t)
	e.cfg.subscriptionBuffer = 1
	first := installQuoteRoute(t, e)
	e.cfg.subscriptionBuffer = 8
	second := installQuoteRoute(t, e)
	closeInstalledQuoteRoute(t, e, second)

	messages := []codec.Message{
		codec.TickReqParams{ReqID: 1, MinTick: "0.01", BBOExchange: "9c0001", SnapshotPermissions: new(4), LastPricePrecision: "0.000001", LastSizePrecision: "0.000001"},
		codec.TickReqParams{ReqID: 2, MinTick: "0.01", BBOExchange: "9c0001", SnapshotPermissions: new(4), LastPricePrecision: "0.000001", LastSizePrecision: "0.000001"},
		codec.MarketDataType{ReqID: 1, DataType: 3},
	}
	for _, message := range messages {
		e.handleIncoming(message)
	}

	if err := first.Wait(); err != ErrSlowConsumer {
		t.Fatalf("first Wait() = %v, want exact ErrSlowConsumer", err)
	}
	if _, ok := e.keyed[1]; ok {
		t.Fatal("slow consumer route 1 remains registered")
	}
	if len(e.cmds) != 0 {
		t.Fatalf("actor-owned cancellation queued %d command(s), want direct route removal", len(e.cmds))
	}

	for _, message := range []codec.Message{
		codec.MarketDataType{ReqID: 2, DataType: 3},
		codec.TickPrice{ReqID: 1, TickType: 66, Price: "310.4", Size: "840"},
		codec.TickPrice{ReqID: 2, TickType: 66, Price: "310.4", Size: "840"},
		codec.TickPrice{ReqID: 1, TickType: 67, Price: "310.55", Size: "80"},
		codec.TickPrice{ReqID: 2, TickType: 67, Price: "310.55", Size: "80"},
	} {
		e.handleIncoming(message)
	}

	var latest Quote
	for range 4 {
		latest = nextQuoteUpdate(t, second).Snapshot
	}
	want := QuoteFieldBid | QuoteFieldAsk | QuoteFieldMarketDataType
	if latest.Available&want != want || latest.Bid.String() != "310.4" || latest.Ask.String() != "310.55" {
		t.Fatalf("sibling quote = %+v, want delayed bid 310.4 ask 310.55", latest)
	}
	if err := second.Err(); err != nil {
		t.Fatalf("sibling Err() = %v", err)
	}
}

func TestQuoteRouteEmitsLiveAncillaryTicks(t *testing.T) {
	// captures/20260824T202842Z-quote_stream_genericticks, server_version 225,
	// events.jsonl sha256
	// 87ff4c1b76c6e94c4cbec0cc20e230750d29aabf1137e5919bb3113dbd8a556f.
	e := newBenchEngine(t)
	e.nextReqID = 1
	sub := installQuoteRoute(t, e)
	closeInstalledQuoteRoute(t, e, sub)

	frames := []string{
		"AAAAKgAAARkIARIEMC4wMRoGOWMwMDAxIAQqCDAuMDAwMDAxMggwLjAwMDAwMQ==",
		"AAAAEQAAAPUIARAuGQAAAAAAAAhA",
		"AAAAEwAAAMoIARBZGgkxOTE4NTY0OTI=",
	}
	for _, frame := range frames {
		message, err := codec.Decode(225, liveCapturedFrame(t, frame))
		if err != nil {
			t.Fatal(err)
		}
		e.handleIncoming(message)
	}

	parameters := nextQuoteUpdate(t, sub)
	if parameters.Kind != QuoteUpdateParameters {
		t.Fatalf("parameters Kind = %v, want %v", parameters.Kind, QuoteUpdateParameters)
	}
	if parameters.Parameters == nil {
		t.Fatal("parameters payload = nil")
	}
	if parameters.Parameters.MinTick == nil || parameters.Parameters.MinTick.String() != "0.01" ||
		parameters.Parameters.BBOExchange != "9c0001" ||
		parameters.Parameters.SnapshotPermissions == nil || *parameters.Parameters.SnapshotPermissions != 4 {
		t.Fatalf("parameters = %+v", parameters.Parameters)
	}

	generic := nextQuoteUpdate(t, sub)
	if generic.Kind != QuoteUpdateGenericTick {
		t.Fatalf("generic Kind = %v, want %v", generic.Kind, QuoteUpdateGenericTick)
	}
	if generic.GenericTick == nil {
		t.Fatal("generic payload = nil")
	}
	if generic.GenericTick.TickType != 46 || generic.GenericTick.Value.String() != "3" {
		t.Fatalf("generic tick = %+v", generic.GenericTick)
	}

	size := nextQuoteUpdate(t, sub)
	if size.Kind != QuoteUpdateSizeTick {
		t.Fatalf("size Kind = %v, want %v", size.Kind, QuoteUpdateSizeTick)
	}
	if size.SizeTick == nil {
		t.Fatal("size payload = nil")
	}
	if size.SizeTick.TickType != 89 || size.SizeTick.Size == nil || size.SizeTick.Size.String() != "191856492" {
		t.Fatalf("size tick = %+v", size.SizeTick)
	}

	for _, update := range []QuoteUpdate{parameters, generic, size} {
		if update.Changed != 0 || update.Snapshot.Available != 0 {
			t.Fatalf("ancillary update mutated quote snapshot: %+v", update)
		}
		if update.ReceivedAt.IsZero() {
			t.Fatal("ancillary update has zero ReceivedAt")
		}
	}
}

func TestQuoteRoutePreservesLiveOptionComputationPresence(t *testing.T) {
	// captures/20260824T202418Z-api_option_calculations_aapl,
	// server_version 225, events.jsonl sha256
	// 10efd6de08b5927fc677546388fb14883031596c9ceed5b5b09bbb98b04c3141.
	// The calculation callback mixes computed zero and positive values with
	// field-specific -1/-2 sentinels.
	e := newBenchEngine(t)
	e.nextReqID = 5
	sub := installQuoteRoute(t, e)
	closeInstalledQuoteRoute(t, e, sub)

	message, err := codec.Decode(225, liveCapturedFrame(t, "AAAAUgAAAN0IBRA1GAAhMzMzMzMz0z8pAAAAAAAAAMAxAAAAAAAA8L85AAAAAAAA8L9BAAAAAAAAAMBJAAAAAAAAAMBRAAAAAAAAAMBZZmZmZmZqc0A="))
	if err != nil {
		t.Fatal(err)
	}
	e.handleIncoming(message)

	update := nextQuoteUpdate(t, sub)
	if update.Kind != QuoteUpdateOptionComputation {
		t.Fatalf("Kind = %v, want %v", update.Kind, QuoteUpdateOptionComputation)
	}
	if update.OptionComputation == nil {
		t.Fatal("option computation payload = nil")
	}
	if update.OptionComputation.TickType != 53 || update.OptionComputation.TickAttrib != 0 {
		t.Fatalf("option header = %+v", update.OptionComputation)
	}
	computation := update.OptionComputation.Computation
	wantAvailable := OptionComputationImpliedVol | OptionComputationUnderlyingPrice
	if computation.Available != wantAvailable {
		t.Fatalf("Available = %08b, want %08b", computation.Available, wantAvailable)
	}
	if computation.ImpliedVol.String() != "0.3" || !computation.Delta.IsZero() || computation.UndPrice.String() != "310.65" {
		t.Fatalf("computed values = %+v", computation)
	}
	if !computation.OptPrice.IsZero() || !computation.PvDividend.IsZero() || !computation.Gamma.IsZero() ||
		!computation.Vega.IsZero() || !computation.Theta.IsZero() ||
		computation.UndPrice.IsZero() {
		t.Fatalf("unavailable computation values were not zero: %+v", computation)
	}
}

func TestQuoteRouteAppliesLiveCompanionSize(t *testing.T) {
	// captures/20260824T202345Z-api_duplicate_quote_subscriptions_aapl,
	// server_version 225, events.jsonl sha256
	// 1fbb60beec41483729e2f9e7c96b1bfdd89649810ffdc5e7e4a4077c1eb8b290.
	e := newBenchEngine(t)
	e.nextReqID = 1
	sub := installQuoteRoute(t, e)
	closeInstalledQuoteRoute(t, e, sub)

	message, err := codec.Decode(225, liveCapturedFrame(t, "AAAAGAAAAMkIARBEGc3MzMzMaHNAIgMxNDEoAA=="))
	if err != nil {
		t.Fatal(err)
	}
	e.handleIncoming(message)

	update := nextQuoteUpdate(t, sub)
	wantChanged := QuoteFieldLast | QuoteFieldLastSize
	if update.Kind != QuoteUpdatePriceTick || update.Changed != wantChanged {
		t.Fatalf("update = Kind %v Changed %v, want fields %v", update.Kind, update.Changed, wantChanged)
	}
	if update.PriceTick == nil {
		t.Fatal("price tick payload = nil")
	}
	if update.PriceTick.TickType != 68 || update.PriceTick.Price.String() != "310.55" {
		t.Fatalf("price tick = %+v", update.PriceTick)
	}
	if update.PriceTick.Size == nil || update.PriceTick.Size.String() != "141" {
		t.Fatalf("price tick companion size = %v, want 141", update.PriceTick.Size)
	}
	if update.PriceTick.AttrMask != 0 {
		t.Fatalf("price tick AttrMask = %d, want 0", update.PriceTick.AttrMask)
	}
	if update.Snapshot.Last.String() != "310.55" || update.Snapshot.LastSize.String() != "141" {
		t.Fatalf("snapshot last/size = %s/%s, want 310.55/141", update.Snapshot.Last, update.Snapshot.LastSize)
	}
}

func TestQuoteRouteAppliesServer220ShareVolume(t *testing.T) {
	// Exact API 10.48.01 / server_version 220 delayed AAPL TickSize frame,
	// captured 2026-07-13. The server reports US-stock market-data volume in
	// shares at this boundary. Capture SHA-256:
	// 0d3d9ec599a4b36c74a0885e5fbbb72af477aab0e6891d28ec480b5980155ad1.
	e := newBenchEngine(t)
	e.serverVersion = 220
	e.nextReqID = 7801
	sub := installQuoteRoute(t, e)
	closeInstalledQuoteRoute(t, e, sub)

	message, err := codec.Decode(220, []byte("\x00\x00\x00\xca\x08\xf9\x3c\x10\x4a\x1a\x06\x35\x33\x37\x30\x31\x36"))
	if err != nil {
		t.Fatal(err)
	}
	e.handleIncoming(message)

	update := nextQuoteUpdate(t, sub)
	if update.Changed != QuoteFieldVolume || update.Snapshot.Available != QuoteFieldVolume || update.Snapshot.Volume.String() != "537016" {
		t.Fatalf("share-volume update = %+v", update)
	}
}

func TestQuoteRouteSupportsServer222FractionalLastSize(t *testing.T) {
	// API 10.48.01 names server_version 222 as the fractional-last-size
	// boundary and defines protobuf sizes as decimal strings. This is a
	// source-law invariant; the live market-hours captures did not happen to
	// contain a fractional AAPL print.
	e := newBenchEngine(t)
	e.serverVersion = 222
	e.nextReqID = 7801
	sub := installQuoteRoute(t, e)
	closeInstalledQuoteRoute(t, e, sub)
	e.handleIncoming(codec.TickSize{ReqID: 7801, TickType: 5, Size: "0.125"})

	update := nextQuoteUpdate(t, sub)
	if update.Changed != QuoteFieldLastSize || update.Snapshot.LastSize.String() != "0.125" {
		t.Fatalf("fractional last-size update = %+v", update)
	}
}

func TestQuoteRouteMapsOfficialOddLotTickFamily(t *testing.T) {
	// API 10.48.01 TickType.java defines IDs 105..110, and its official
	// MarketDataSamplesProto requests them with generic tick 787. The exact
	// server_version 225 market-hours capture (SHA-256 below) proved the request
	// was accepted, but delayed data returned no positive odd-lot rows:
	// 85d0dba58ba9d80c029fac5b658d01ac48128d0513228c07f555dbac6fbff2b0.
	e := newBenchEngine(t)
	e.serverVersion = 225
	e.nextReqID = 7801
	sub := installQuoteRoute(t, e)
	closeInstalledQuoteRoute(t, e, sub)

	for _, message := range []codec.Message{
		codec.TickPrice{ReqID: 7801, TickType: TickTypeOddLotBid, Price: "316.11"},
		codec.TickPrice{ReqID: 7801, TickType: TickTypeOddLotAsk, Price: "316.12"},
		codec.TickSize{ReqID: 7801, TickType: TickTypeOddLotBidSize, Size: "7"},
		codec.TickSize{ReqID: 7801, TickType: TickTypeOddLotAskSize, Size: "9"},
		codec.TickString{ReqID: 7801, TickType: TickTypeOddLotBidExchange, Value: "NASDAQ"},
		codec.TickString{ReqID: 7801, TickType: TickTypeOddLotAskExchange, Value: "NYSE"},
	} {
		e.handleIncoming(message)
	}

	wantChanged := []QuoteFields{
		QuoteFieldOddLotBid, QuoteFieldOddLotAsk, QuoteFieldOddLotBidSize,
		QuoteFieldOddLotAskSize, QuoteFieldOddLotBidExchange, QuoteFieldOddLotAskExchange,
	}
	var last QuoteUpdate
	for _, want := range wantChanged {
		last = nextQuoteUpdate(t, sub)
		if last.Changed != want {
			t.Fatalf("odd-lot Changed = %v, want %v", last.Changed, want)
		}
	}
	wantAvailable := QuoteFieldOddLotBid | QuoteFieldOddLotAsk | QuoteFieldOddLotBidSize |
		QuoteFieldOddLotAskSize | QuoteFieldOddLotBidExchange | QuoteFieldOddLotAskExchange
	if last.Snapshot.Available != wantAvailable || last.Snapshot.OddLotBid.String() != "316.11" ||
		last.Snapshot.OddLotAsk.String() != "316.12" || last.Snapshot.OddLotBidSize.String() != "7" ||
		last.Snapshot.OddLotAskSize.String() != "9" || last.Snapshot.OddLotBidExchange != "NASDAQ" ||
		last.Snapshot.OddLotAskExchange != "NYSE" {
		t.Fatalf("odd-lot snapshot = %+v", last.Snapshot)
	}
	if GenericTickOddLotBidAsk != "787" {
		t.Fatalf("GenericTickOddLotBidAsk = %q", GenericTickOddLotBidAsk)
	}
}

func TestQuoteRouteExposesEFPAndDeltaNeutralCallbacks(t *testing.T) {
	// These callback shapes follow API 10.48.01 EWrapper/EDecoder source law.
	// The executable market-hours probes still lack the entitlements needed for
	// a positive single-stock-future or BAG callback.
	e := newBenchEngine(t)
	e.nextReqID = 7801
	sub := installQuoteRoute(t, e)
	closeInstalledQuoteRoute(t, e, sub)

	e.handleIncoming(codec.TickEFP{
		ReqID: 7801, TickType: 38, BasisPoints: "12.5", FormattedBasisPoints: "12.5%",
		ImpliedFuturesPrice: "316.25", HoldDays: 42, FutureLastTradeDate: "20260918",
		DividendImpact: "0.75", DividendsToLastTradeDate: "1.5",
	})
	e.handleIncoming(codec.DeltaNeutralValidation{
		ReqID:    7801,
		Contract: codec.DeltaNeutralContract{ConID: 265598, Delta: "0.52", Price: "316.25"},
	})

	efp := nextQuoteUpdate(t, sub)
	if efp.Kind != QuoteUpdateEFP || efp.EFP == nil || efp.EFP.BasisPoints.String() != "12.5" ||
		efp.EFP.FormattedBasisPoints != "12.5%" || efp.EFP.ImpliedFuturesPrice.String() != "316.25" ||
		efp.EFP.HoldDays != 42 || efp.EFP.FutureLastTradeDate != "20260918" ||
		efp.EFP.DividendImpact.String() != "0.75" || efp.EFP.DividendsToLastTradeDate.String() != "1.5" ||
		efp.Changed != 0 {
		t.Fatalf("EFP update = %+v", efp)
	}
	validation := nextQuoteUpdate(t, sub)
	if validation.Kind != QuoteUpdateDeltaNeutralValidation || validation.DeltaNeutral == nil ||
		validation.DeltaNeutral.ConID != 265598 || validation.DeltaNeutral.Delta.String() != "0.52" ||
		validation.DeltaNeutral.Price.String() != "316.25" || validation.Changed != 0 {
		t.Fatalf("delta-neutral validation update = %+v", validation)
	}
}

func TestQuoteRoutePreservesLiveUnnormalizedSizeTick(t *testing.T) {
	// Same live capture and hash as TestQuoteRouteEmitsLiveAncillaryTicks.
	// Generic tick request 236 produced tickSize type 89 (shortable shares),
	// which has no normalized Quote field.
	e := newBenchEngine(t)
	e.nextReqID = 1
	sub := installQuoteRoute(t, e)
	closeInstalledQuoteRoute(t, e, sub)

	message, err := codec.Decode(225, liveCapturedFrame(t, "AAAAEwAAAMoIARBZGgkxOTE4NTY0OTI="))
	if err != nil {
		t.Fatal(err)
	}
	e.handleIncoming(message)

	update := nextQuoteUpdate(t, sub)
	if update.Kind != QuoteUpdateSizeTick || update.Changed != 0 {
		t.Fatalf("update = Kind %v Changed %v, want unnormalized size tick", update.Kind, update.Changed)
	}
	if update.SizeTick == nil || update.SizeTick.TickType != 89 || update.SizeTick.Size == nil || update.SizeTick.Size.String() != "191856492" {
		t.Fatalf("size tick = %+v", update.SizeTick)
	}
	if update.Snapshot.Available != 0 {
		t.Fatalf("unnormalized tick mutated snapshot: %+v", update.Snapshot)
	}
}

func TestQuoteRoutePreservesLiveUnnormalizedPriceTick(t *testing.T) {
	// captures/20260824T202402Z-api_generic_tick_matrix_aapl,
	// server_version 225, events.jsonl sha256
	// 5288cb6711b5be6ac94a68c5f42285ffb86e0f2181bf836ac75f491758d7c15a.
	// Generic tick request 221 produced tickPrice type 37 (mark price), which
	// has no normalized Quote field. The exact frame's companion size is zero.
	e := newBenchEngine(t)
	e.nextReqID = 1
	sub := installQuoteRoute(t, e)
	closeInstalledQuoteRoute(t, e, sub)

	message, err := codec.Decode(225, liveCapturedFrame(t, "AAAAFgAAAMkIARAlGTiGAOA4aHNAIgEwKAA="))
	if err != nil {
		t.Fatal(err)
	}
	e.handleIncoming(message)

	update := nextQuoteUpdate(t, sub)
	if update.Kind != QuoteUpdatePriceTick || update.Changed != 0 {
		t.Fatalf("update = Kind %v Changed %v, want unnormalized price tick", update.Kind, update.Changed)
	}
	if update.PriceTick == nil || update.PriceTick.TickType != 37 || update.PriceTick.Price.String() != "310.5138855" {
		t.Fatalf("price tick = %+v", update.PriceTick)
	}
	if update.PriceTick.Size == nil || !update.PriceTick.Size.IsZero() || update.PriceTick.AttrMask != 0 {
		t.Fatalf("price tick size/attributes = %v/%d, want 0/0", update.PriceTick.Size, update.PriceTick.AttrMask)
	}
	if update.Snapshot.Available != 0 {
		t.Fatalf("unnormalized tick mutated snapshot: %+v", update.Snapshot)
	}
}

func nextQuoteUpdate(t *testing.T, sub *Subscription[QuoteUpdate]) QuoteUpdate {
	t.Helper()
	select {
	case event := <-sub.Events():
		if event.Kind != StreamData {
			return nextQuoteUpdate(t, sub)
		}
		return event.Value
	default:
		t.Fatal("quote update was not emitted synchronously")
		return QuoteUpdate{}
	}
}

func closeInstalledQuoteRoute(t *testing.T, e *engine, sub *Subscription[QuoteUpdate]) {
	t.Helper()
	t.Cleanup(func() {
		sub.Close()
		select {
		case closeRoute := <-e.cmds:
			closeRoute()
		default:
			t.Error("quote subscription close was not enqueued")
		}
	})
}

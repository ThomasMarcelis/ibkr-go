package ibkr

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/transport"
)

func TestQuoteRouteFollowsLiveRerouteAndFreezesResumeRequest(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	e.nextReqID = 20621
	req := QuoteRequest{
		Contract:     Contract{Symbol: "IBM", SecType: SecTypeCFD, Exchange: "SMART", Currency: "USD"},
		GenericTicks: []GenericTick{"100", "233"},
	}
	sub := installObservedQuoteRoute(t, e, req, WithResumePolicy(ResumeAuto))
	_ = readObservedFrame(t, peer)

	rawReroute := []byte("\x00\x00\x00\x5b20621\x008314\x00SMART\x00")
	reroute, err := codec.Decode(206, rawReroute)
	if err != nil {
		t.Fatal(err)
	}
	e.handleIncoming(reroute)

	routedPayload := readObservedFrame(t, peer)
	route := e.keyed[20621]
	if route == nil || route.resume != ResumeAuto {
		t.Fatalf("route = %+v, want resumable active route", route)
	}
	routedRequest, ok := route.request.(codec.QuoteRequest)
	if !ok {
		t.Fatalf("route request = %T, want codec.QuoteRequest", route.request)
	}
	if !reflect.DeepEqual(routedRequest.Contract, codec.Contract{ConID: 8314, Exchange: "SMART"}) ||
		routedRequest.Snapshot || len(routedRequest.GenericTicks) != 2 ||
		routedRequest.GenericTicks[0] != "100" || routedRequest.GenericTicks[1] != "233" {
		t.Fatalf("rerouted request = %+v, want replacement contract with original request configuration", routedRequest)
	}
	wantPayload, err := codec.Encode(206, routedRequest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(routedPayload, wantPayload) {
		t.Fatalf("rerouted payload = %x, want %x", routedPayload, wantPayload)
	}

	e.handleIncoming(reroute)
	if _, ok := e.keyed[20621]; ok {
		t.Fatal("second reroute left the request active")
	}
	cancelPayload := readObservedFrame(t, peer)
	wantCancel, err := codec.Encode(206, codec.CancelQuote{ReqID: 20621})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(cancelPayload, wantCancel) {
		t.Fatalf("second reroute cancel = %x, want %x", cancelPayload, wantCancel)
	}
	if err := sub.Err(); err == nil || err.Error() != "ibkr: market data request 20621 was rerouted more than once" {
		t.Fatalf("second reroute error = %v", err)
	}
}

func TestQuoteResumeRejectsContractFieldsAfterVersionDowngrade(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	e.nextReqID = 20623
	// IncludeExpired is present in the exact-sv206 shared Contract request but
	// has no field in the classic sv205 quote layout. This test freezes the
	// reconnect actor behavior; it does not claim a positive expired-future
	// market-data result from the Gateway.
	req := QuoteRequest{Contract: Contract{
		Symbol: "MES", SecType: SecTypeFuture, Expiry: "202606",
		Exchange: "CME", Currency: "USD", IncludeExpired: true,
	}}
	sub := installObservedQuoteRoute(t, e, req, WithResumePolicy(ResumeAuto))
	_ = readObservedFrame(t, peer)

	route := e.keyed[20623]
	if route == nil {
		t.Fatal("quote route was not installed")
	}
	route.gapped = true
	e.serverVersion = 205
	e.resumeRoutes()

	if _, ok := e.keyed[20623]; ok {
		t.Fatal("unsupported quote resume left the route active")
	}
	validation, ok := errors.AsType[*ValidationError](sub.Err())
	if !ok || validation.Field != "Contract.IncludeExpired" ||
		validation.Message != "is not represented by resume market data quote at negotiated server_version 205" {
		t.Fatalf("resume error = %#v, want precise IncludeExpired version failure", sub.Err())
	}
	select {
	case <-e.done:
		t.Fatal("unsupported quote resume terminated the session")
	default:
	}
	for event := range sub.Events() {
		if event.Kind == StreamResubscribed {
			t.Fatal("unsupported quote resume emitted Resubscribed")
		}
	}
}

func TestMarketDepthRouteRejectsRepeatedLiveRerouteWithSmartCancel(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	e.nextReqID = 20622
	req := MarketDepthRequest{
		Contract: Contract{Symbol: "IBM", SecType: SecTypeCFD, Exchange: "SMART", Currency: "USD"},
		NumRows:  5, IsSmartDepth: true,
	}
	sub := installObservedDepthRoute(t, e, req)
	_ = readObservedFrame(t, peer)

	rawReroute := []byte("\x00\x00\x00\x5c20622\x008314\x00SMART\x00")
	reroute, err := codec.Decode(206, rawReroute)
	if err != nil {
		t.Fatal(err)
	}
	e.handleIncoming(reroute)
	_ = readObservedFrame(t, peer)

	active := e.keyed[20622]
	routedRequest, ok := active.request.(codec.MarketDepthRequest)
	if !ok {
		t.Fatalf("route request = %T, want codec.MarketDepthRequest", active.request)
	}
	if !reflect.DeepEqual(routedRequest.Contract, codec.Contract{ConID: 8314, Exchange: "SMART"}) ||
		routedRequest.NumRows != 5 || !routedRequest.IsSmartDepth || active.resume != ResumeNever {
		t.Fatalf("rerouted depth request = %+v, route resume=%v", routedRequest, active.resume)
	}

	e.handleIncoming(reroute)
	if _, ok := e.keyed[20622]; ok {
		t.Fatal("second depth reroute left the request active")
	}
	cancelPayload := readObservedFrame(t, peer)
	wantCancel, err := codec.Encode(206, codec.CancelMarketDepth{ReqID: 20622, IsSmartDepth: true})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(cancelPayload, wantCancel) {
		t.Fatalf("cancel payload = %x, want smart-depth cancel %x", cancelPayload, wantCancel)
	}
	if err := sub.Err(); err == nil || err.Error() != "ibkr: market depth request 20622 was rerouted more than once" {
		t.Fatalf("second reroute error = %v", err)
	}
}

func TestSmartDepthExchangeNoticePreservesLiveRoute(t *testing.T) {
	// /tmp/ibkr-go-depth-public-final-20260711/
	// 20260711T000232Z-market_depth_aapl_smart, server_version 206,
	// events.jsonl sha256 a5df84945a10440ab4ef4a6336f837570cd53d036314c2461104a2555f94e8a3.
	// The exact request-scoped code 2152 frame must reach the keyed depth route
	// without closing it: the official notice may precede valid rows when any
	// listed depth venue is available.
	e, peer := newObservedMarketDataEngine(t)
	e.serverVersion = 206
	e.nextReqID = 1
	sub := installObservedDepthRoute(t, e, MarketDepthRequest{
		Contract: Contract{Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD"},
		NumRows:  5, IsSmartDepth: true,
	})
	_ = readObservedFrame(t, peer)

	message, err := codec.Decode(206, liveCapturedFrame(t, "AAAA3QAAAMwIARDwxe7z9DMY6BAiygFFeGNoYW5nZXMgLSBUb3A6IElCRU9TOyBPVkVSTklHSFQ7IE5lZWQgYWRkaXRpb25hbCBtYXJrZXQgZGF0YSBwZXJtaXNzaW9ucyAtIERlcHRoOiBOQVNEQVE7IEJBVFM7IEFSQ0E7IEJFWDsgTllTRTsgSUVYOyBUb3A6IEJZWDsgQU1FWDsgUEVBUkw7IFQyNFg7IE1FTVg7IEVER0VBOyBDSFg7IE5ZU0VOQVQ7IFBTWDsgTFRTRTsgSVNFOyBEUkNURURHRTsg"))
	if err != nil {
		t.Fatalf("decode exact live SMART-depth exchange notice: %v", err)
	}
	e.handleIncoming(message)

	if _, ok := e.keyed[1]; !ok {
		t.Fatal("live code-2152 SMART-depth exchange notice deleted the route")
	}
	event := <-e.SessionEvents()
	if event.Code != ErrCodeSmartDepthExchanges || event.Message == "" {
		t.Fatalf("market-depth session event = %+v, want code-2152 availability notice", event)
	}
	if err := sub.Err(); err != nil {
		t.Fatalf("market-depth subscription error after notice = %v", err)
	}

	sub.Close()
	(<-e.cmds)()
	wantCancel, err := codec.Encode(206, codec.CancelMarketDepth{ReqID: 1, IsSmartDepth: true})
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

func TestQuoteRoutePreservesSV206ParameterPresenceAndPrecision(t *testing.T) {
	e := newBenchEngine(t)
	e.serverVersion = 206
	e.nextReqID = 20611
	sub := installQuoteRoute(t, e)
	closeInstalledQuoteRoute(t, e, sub)

	raw := []byte("\x00\x00\x01\x19\x08\x83\xa1\x01\x12\x04\x30\x2e\x30\x31\x1a\x06\x39\x63\x30\x30\x30\x31\x20\x04\x2a\x08\x30\x2e\x30\x30\x30\x30\x30\x31\x32\x08\x30\x2e\x30\x30\x30\x30\x30\x31")
	msg, err := codec.Decode(206, raw)
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

	e.handleIncoming(codec.TickReqParams{ReqID: 20611})
	absent := nextQuoteUpdate(t, sub).Parameters
	if absent == nil || absent.SnapshotPermissions != nil || absent.LastPricePrecision != nil || absent.LastSizePrecision != nil {
		t.Fatalf("omitted parameters = %+v, want nil presence fields", absent)
	}
}

func TestMarketDataRoutesPreserveOfficialUnavailableSizes(t *testing.T) {
	t.Run("quote size", func(t *testing.T) {
		e := newBenchEngine(t)
		e.serverVersion = 206
		e.nextReqID = 20611
		sub := installQuoteRoute(t, e)
		closeInstalledQuoteRoute(t, e, sub)

		// API 10.48.01 maps an omitted TickSize.size to UNSET_DECIMAL.
		e.handleIncoming(codec.TickSize{ReqID: 20611, TickType: 0})
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
		cfg: cfg, cmds: make(chan func(), 8), incoming: make(chan any, 8),
		transportErr: make(chan transportLoss, 1), done: make(chan struct{}),
		events: newObserver[Event](cfg.eventBuffer), transport: tr,
		serverVersion: 206, keyed: make(map[int]*route), singletons: make(map[string]*route),
		orders:               make(map[int64]*orderRoute),
		execDeliveries:       make(map[string]*execDelivery),
		malformedInboundSeen: make(map[int]struct{}),
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

func installObservedDepthRoute(t *testing.T, e *engine, req MarketDepthRequest, opts ...SubscriptionOption) *Subscription[DepthRow] {
	t.Helper()
	result := make(chan *Subscription[DepthRow], 1)
	go func() {
		sub, err := e.SubscribeMarketDepth(context.Background(), req, opts...)
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
	// captures/20260415T162742Z-api_duplicate_quote_subscriptions_aapl,
	// server_version 200, events.jsonl sha256 prefix 84f1e78a18616e0f.
	// These exact frames were delivered to two independent AAPL subscriptions.
	// One stalled consumer must close only its own route while its sibling still
	// receives the complete delayed bid/ask sequence.
	e := newBenchEngine(t)
	e.cfg.subscriptionBuffer = 1
	first := installQuoteRoute(t, e)
	e.cfg.subscriptionBuffer = 8
	second := installQuoteRoute(t, e)
	closeInstalledQuoteRoute(t, e, second)

	frames := [][]byte{
		[]byte("81\x001\x000.01\x009c0001\x004\x00"),
		[]byte("81\x002\x000.01\x009c0001\x004\x00"),
		[]byte("58\x001\x001\x003\x00"),
	}
	for _, frame := range frames {
		e.handleIncoming(decodeOne(t, frame))
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

	for _, frame := range [][]byte{
		[]byte("58\x001\x002\x003\x00"),
		[]byte("1\x006\x001\x0066\x00263.45\x000\x000\x00"),
		[]byte("1\x006\x002\x0066\x00263.45\x000\x000\x00"),
		[]byte("1\x006\x001\x0067\x00263.48\x000\x000\x00"),
		[]byte("1\x006\x002\x0067\x00263.48\x000\x000\x00"),
	} {
		e.handleIncoming(decodeOne(t, frame))
	}

	var latest Quote
	for range 4 {
		latest = nextQuoteUpdate(t, second).Snapshot
	}
	want := QuoteFieldBid | QuoteFieldAsk | QuoteFieldMarketDataType
	if latest.Available&want != want || latest.Bid.String() != "263.45" || latest.Ask.String() != "263.48" {
		t.Fatalf("sibling quote = %+v, want delayed bid 263.45 ask 263.48", latest)
	}
	if err := second.Err(); err != nil {
		t.Fatalf("sibling Err() = %v", err)
	}
}

func TestQuoteRouteEmitsLiveAncillaryTicks(t *testing.T) {
	// captures/20260405T215752Z-quote_stream_genericticks, IB Gateway
	// server_version 200. raw.txt sha256:
	// 9c4fec0cd44041ccfec4fee372ed6cea437418183b42591936c64ee4fdf52bee.
	// These are exact payloads captured during the live AAPL request for
	// generic ticks 233 and 236; only the four-byte frame lengths are omitted.
	e := newBenchEngine(t)
	e.nextReqID = 1001
	sub := installQuoteRoute(t, e)
	closeInstalledQuoteRoute(t, e, sub)

	frames := [][]byte{
		[]byte("81\x001001\x000.01\x009c0001\x004\x00"),
		[]byte("45\x006\x001001\x0046\x003.0\x00"),
		[]byte("46\x006\x001001\x0088\x001775174157\x00"),
	}
	for _, frame := range frames {
		e.handleIncoming(decodeOne(t, frame))
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

	text := nextQuoteUpdate(t, sub)
	if text.Kind != QuoteUpdateStringTick {
		t.Fatalf("string Kind = %v, want %v", text.Kind, QuoteUpdateStringTick)
	}
	if text.StringTick == nil {
		t.Fatal("string payload = nil")
	}
	if text.StringTick.TickType != 88 || text.StringTick.Value != "1775174157" {
		t.Fatalf("string tick = %+v", text.StringTick)
	}

	for _, update := range []QuoteUpdate{parameters, generic, text} {
		if update.Changed != 0 || update.Snapshot.Available != 0 {
			t.Fatalf("ancillary update mutated quote snapshot: %+v", update)
		}
		if update.ReceivedAt.IsZero() {
			t.Fatal("ancillary update has zero ReceivedAt")
		}
	}
}

func TestQuoteRoutePreservesLiveOptionComputationAbsence(t *testing.T) {
	// captures/20260611T080111Z-api_option_campaign_aapl, paper IB Gateway
	// server_version 200. raw.txt sha256:
	// 9272d7fc1b381a0e1ccfb4d506138c9ba925ad96063c4eed0be96a0fe00a010c.
	// This exact msg-21 payload came from the live option quote subscription.
	// IBKR uses field-specific -1/-2 sentinels; pvDividend is a computed zero.
	e := newBenchEngine(t)
	e.nextReqID = 5
	sub := installQuoteRoute(t, e)
	closeInstalledQuoteRoute(t, e, sub)

	frame := []byte("21\x005\x0010\x000\x00-1\x00-2\x00-1\x000.0\x00-2\x00-2\x00-2\x00-1\x00")
	e.handleIncoming(decodeOne(t, frame))

	update := nextQuoteUpdate(t, sub)
	if update.Kind != QuoteUpdateOptionComputation {
		t.Fatalf("Kind = %v, want %v", update.Kind, QuoteUpdateOptionComputation)
	}
	if update.OptionComputation == nil {
		t.Fatal("option computation payload = nil")
	}
	if update.OptionComputation.TickType != 10 || update.OptionComputation.TickAttrib != 0 {
		t.Fatalf("option header = %+v", update.OptionComputation)
	}
	computation := update.OptionComputation.Computation
	if computation.Available != OptionComputationPvDividend {
		t.Fatalf("Available = %08b, want only PvDividend", computation.Available)
	}
	if !computation.PvDividend.IsZero() {
		t.Fatalf("PvDividend = %s, want computed zero", computation.PvDividend)
	}
	if !computation.ImpliedVol.IsZero() || !computation.Delta.IsZero() ||
		!computation.OptPrice.IsZero() || !computation.Gamma.IsZero() ||
		!computation.Vega.IsZero() || !computation.Theta.IsZero() ||
		!computation.UndPrice.IsZero() {
		t.Fatalf("unavailable computation values were not zero: %+v", computation)
	}
}

func TestQuoteRouteAppliesLiveCompanionSize(t *testing.T) {
	// captures/20260405T215752Z-quote_stream_genericticks, IB Gateway
	// server_version 200, raw.txt sha256
	// 9c4fec0cd44041ccfec4fee372ed6cea437418183b42591936c64ee4fdf52bee.
	// The classic tickPrice frame carries a companion size which the official
	// decoder delivers as a second tickSize callback.
	e := newBenchEngine(t)
	e.nextReqID = 1001
	sub := installQuoteRoute(t, e)
	closeInstalledQuoteRoute(t, e, sub)

	frame := []byte("1\x006\x001001\x0068\x00255.45\x00200\x000\x00")
	e.handleIncoming(decodeOne(t, frame))

	update := nextQuoteUpdate(t, sub)
	wantChanged := QuoteFieldLast | QuoteFieldLastSize
	if update.Kind != QuoteUpdatePriceTick || update.Changed != wantChanged {
		t.Fatalf("update = Kind %v Changed %v, want fields %v", update.Kind, update.Changed, wantChanged)
	}
	if update.PriceTick == nil {
		t.Fatal("price tick payload = nil")
	}
	if update.PriceTick.TickType != 68 || update.PriceTick.Price.String() != "255.45" {
		t.Fatalf("price tick = %+v", update.PriceTick)
	}
	if update.PriceTick.Size == nil || update.PriceTick.Size.String() != "200" {
		t.Fatalf("price tick companion size = %v, want 200", update.PriceTick.Size)
	}
	if update.PriceTick.AttrMask != 0 {
		t.Fatalf("price tick AttrMask = %d, want 0", update.PriceTick.AttrMask)
	}
	if update.Snapshot.Last.String() != "255.45" || update.Snapshot.LastSize.String() != "200" {
		t.Fatalf("snapshot last/size = %s/%s, want 255.45/200", update.Snapshot.Last, update.Snapshot.LastSize)
	}
}

func TestQuoteRoutePreservesLiveUnnormalizedSizeTick(t *testing.T) {
	// Same live capture and hash as TestQuoteRouteAppliesLiveCompanionSize.
	// Generic tick request 236 produced tickSize type 89 (shortable shares),
	// which has no normalized Quote field.
	e := newBenchEngine(t)
	e.nextReqID = 1001
	sub := installQuoteRoute(t, e)
	closeInstalledQuoteRoute(t, e, sub)

	frame := []byte("2\x006\x001001\x0089\x00104796567\x00")
	e.handleIncoming(decodeOne(t, frame))

	update := nextQuoteUpdate(t, sub)
	if update.Kind != QuoteUpdateSizeTick || update.Changed != 0 {
		t.Fatalf("update = Kind %v Changed %v, want unnormalized size tick", update.Kind, update.Changed)
	}
	if update.SizeTick == nil || update.SizeTick.TickType != 89 || update.SizeTick.Size == nil || update.SizeTick.Size.String() != "104796567" {
		t.Fatalf("size tick = %+v", update.SizeTick)
	}
	if update.Snapshot.Available != 0 {
		t.Fatalf("unnormalized tick mutated snapshot: %+v", update.Snapshot)
	}
}

func TestQuoteRoutePreservesLiveUnnormalizedPriceTick(t *testing.T) {
	// captures/20260709T223341Z-api_generic_tick_matrix_aapl, read-only IB
	// Gateway server_version 200. raw.txt sha256:
	// 5c40260d783971d22e6de209c90a61fd489479e0e7fc2ebf20be4e76d677a45e.
	// Generic tick request 221 produced tickPrice type 37 (mark price), which
	// has no normalized Quote field. The exact frame's companion size is zero.
	e := newBenchEngine(t)
	e.nextReqID = 1
	sub := installQuoteRoute(t, e)
	closeInstalledQuoteRoute(t, e, sub)

	frame := []byte("1\x006\x001\x0037\x00315.50\x000\x000\x00")
	e.handleIncoming(decodeOne(t, frame))

	update := nextQuoteUpdate(t, sub)
	if update.Kind != QuoteUpdatePriceTick || update.Changed != 0 {
		t.Fatalf("update = Kind %v Changed %v, want unnormalized price tick", update.Kind, update.Changed)
	}
	if update.PriceTick == nil || update.PriceTick.TickType != 37 || update.PriceTick.Price.String() != "315.5" {
		t.Fatalf("price tick = %+v", update.PriceTick)
	}
	if update.PriceTick.Size == nil || !update.PriceTick.Size.IsZero() || update.PriceTick.AttrMask != 0 {
		t.Fatalf("price tick size/attributes = %v/%d, want 0/0", update.PriceTick.Size, update.PriceTick.AttrMask)
	}
	if update.Snapshot.Available != 0 {
		t.Fatalf("unnormalized tick mutated snapshot: %+v", update.Snapshot)
	}
}

func TestQuoteRoutePreservesLivePriceAttributes(t *testing.T) {
	// captures/20260611T074859Z-api_option_campaign_aapl, paper IB Gateway
	// server_version 200. raw.txt sha256:
	// 1e35bce4310dbcd5c62c10cb8e9db5bf4961cebecb0b9a526c83f385a3a05fe5.
	// This exact option quote tick carries attrMask=1 (canAutoExecute).
	e := newBenchEngine(t)
	e.nextReqID = 5
	sub := installQuoteRoute(t, e)
	closeInstalledQuoteRoute(t, e, sub)

	frame := []byte("1\x006\x005\x001\x00-1.00\x000\x001\x00")
	e.handleIncoming(decodeOne(t, frame))

	update := nextQuoteUpdate(t, sub)
	if update.Kind != QuoteUpdatePriceTick || update.PriceTick == nil {
		t.Fatalf("update = Kind %v PriceTick %+v", update.Kind, update.PriceTick)
	}
	attributes := update.PriceTick.AttrMask
	if attributes != 1 || !attributes.CanAutoExecute() || attributes.PastLimit() || attributes.PreOpen() {
		t.Fatalf("attributes = %d auto=%t pastLimit=%t preOpen=%t", attributes,
			attributes.CanAutoExecute(), attributes.PastLimit(), attributes.PreOpen())
	}
}

func TestQuoteRoutePreservesLiveMissingMinimumTick(t *testing.T) {
	// captures/20260709T223247Z-api_generic_tick_matrix_aapl, read-only IB
	// Gateway server_version 200. raw.txt sha256:
	// bd284e22771394b3baf7b827d62ed22d45f15e401dd478f430e18f4e715b0377.
	// The Gateway omitted minTick while still sending BBO and permission data.
	e := newBenchEngine(t)
	e.nextReqID = 1
	sub := installQuoteRoute(t, e)
	closeInstalledQuoteRoute(t, e, sub)

	frame := []byte("81\x001\x00\x009c0001\x004\x00")
	e.handleIncoming(decodeOne(t, frame))

	update := nextQuoteUpdate(t, sub)
	if update.Kind != QuoteUpdateParameters || update.Parameters == nil {
		t.Fatalf("update = Kind %v Parameters %+v", update.Kind, update.Parameters)
	}
	if update.Parameters.MinTick != nil || update.Parameters.BBOExchange != "9c0001" || update.Parameters.SnapshotPermissions == nil || *update.Parameters.SnapshotPermissions != 4 {
		t.Fatalf("parameters = %+v", update.Parameters)
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

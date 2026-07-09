package ibkr

import "testing"

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
		parameters.Parameters.SnapshotPermissions != 4 {
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
	if update.SizeTick == nil || update.SizeTick.TickType != 89 || update.SizeTick.Size.String() != "104796567" {
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
	if update.Parameters.MinTick != nil || update.Parameters.BBOExchange != "9c0001" || update.Parameters.SnapshotPermissions != 4 {
		t.Fatalf("parameters = %+v", update.Parameters)
	}
}

func nextQuoteUpdate(t *testing.T, sub *Subscription[QuoteUpdate]) QuoteUpdate {
	t.Helper()
	select {
	case update := <-sub.Events():
		return update
	default:
		t.Fatal("quote update was not emitted synchronously")
		return QuoteUpdate{}
	}
}

func closeInstalledQuoteRoute(t *testing.T, e *engine, sub *Subscription[QuoteUpdate]) {
	t.Helper()
	t.Cleanup(func() {
		if err := sub.Close(); err != nil {
			t.Errorf("close quote subscription: %v", err)
			return
		}
		select {
		case closeRoute := <-e.cmds:
			closeRoute()
		default:
			t.Error("quote subscription close was not enqueued")
		}
	})
}

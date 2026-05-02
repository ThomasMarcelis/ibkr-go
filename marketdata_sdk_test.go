package ibkr

import (
	"context"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
)

func TestSDKMarketDataTypeUsesSDKCommandAndEvent(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	if err := e.sendSDKContext(context.Background(), sdkadapter.ReqMarketDataType{DataType: int(MarketDataDelayed)}); err != nil {
		t.Fatalf("sendSDKContext(ReqMarketDataType) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 1 {
		t.Fatalf("commands len = %d, want 1", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandMarketDataType {
		t.Fatalf("command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandMarketDataType)
	}
	if commands[0].MarketDataType.DataType != int(MarketDataDelayed) {
		t.Fatalf("market data type = %d, want %d", commands[0].MarketDataType.DataType, MarketDataDelayed)
	}

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind:           sdkadapter.EventMarketDataType,
		ReqID:          61,
		MarketDataType: int(MarketDataDelayed),
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(market data type) error = %v", err)
	}
	got, ok := msg.(sdkadapter.MarketDataType)
	if !ok {
		t.Fatalf("sdkEventToMessage(market data type) type = %T, want sdkadapter.MarketDataType", msg)
	}
	if got.ReqID != 61 || got.DataType != int(MarketDataDelayed) {
		t.Fatalf("market data type event = %+v, want reqID 61 delayed", got)
	}
}

func TestSDKQuotesUseSDKCommandsAndEvents(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	contract := sdkadapter.Contract{
		Symbol:   "AAPL",
		SecType:  "STK",
		Exchange: "SMART",
		Currency: "USD",
	}
	genericTicks := []string{"233", "236"}
	if err := e.sendSDKContext(context.Background(), sdkadapter.QuoteRequest{
		ReqID:        65,
		Contract:     contract,
		Snapshot:     false,
		GenericTicks: genericTicks,
	}); err != nil {
		t.Fatalf("sendSDKContext(QuoteRequest) error = %v", err)
	}
	genericTicks[0] = "mutated"
	if err := e.sendSDKContext(context.Background(), sdkadapter.CancelQuote{ReqID: 65}); err != nil {
		t.Fatalf("sendSDKContext(CancelQuote) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 2 {
		t.Fatalf("commands len = %d, want 2", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandQuote {
		t.Fatalf("quote command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandQuote)
	}
	if commands[0].Quote.ReqID != 65 || commands[0].Quote.Snapshot || len(commands[0].Quote.GenericTicks) != 2 {
		t.Fatalf("quote command = %+v, want reqID 65 stream with two generic ticks", commands[0].Quote)
	}
	if commands[0].Quote.GenericTicks[0] != "233" || commands[0].Quote.GenericTicks[1] != "236" {
		t.Fatalf("quote generic ticks = %v, want copied 233/236", commands[0].Quote.GenericTicks)
	}
	if commands[0].Quote.Contract.Symbol != "AAPL" || commands[0].Quote.Contract.Exchange != "SMART" {
		t.Fatalf("quote contract = %+v, want AAPL SMART", commands[0].Quote.Contract)
	}
	if commands[1].Kind != sdkadapter.CommandCancelQuote || commands[1].CancelQuote.ReqID != 65 {
		t.Fatalf("cancel quote command = %+v, want reqID 65", commands[1])
	}

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind:  sdkadapter.EventTickPrice,
		ReqID: 65,
		TickPrice: sdkadapter.TickPriceValue{
			TickType: 1,
			Price:    "188.1",
			AttrMask: 3,
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(tick price) error = %v", err)
	}
	price, ok := msg.(sdkadapter.TickPrice)
	if !ok {
		t.Fatalf("sdkEventToMessage(tick price) type = %T, want sdkadapter.TickPrice", msg)
	}
	if price.ReqID != 65 || price.TickType != 1 || price.Price != "188.1" || price.AttrMask != 3 {
		t.Fatalf("tick price = %+v, want copied price", price)
	}

	msg, err = sdkEventToMessage(sdkadapter.Event{
		Kind:     sdkadapter.EventTickSize,
		ReqID:    65,
		TickSize: sdkadapter.TickSizeValue{TickType: 0, Size: "100"},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(tick size) error = %v", err)
	}
	size, ok := msg.(sdkadapter.TickSize)
	if !ok {
		t.Fatalf("sdkEventToMessage(tick size) type = %T, want sdkadapter.TickSize", msg)
	}
	if size.ReqID != 65 || size.TickType != 0 || size.Size != "100" {
		t.Fatalf("tick size = %+v, want copied size", size)
	}

	msg, err = sdkEventToMessage(sdkadapter.Event{
		Kind:        sdkadapter.EventTickGeneric,
		ReqID:       65,
		TickGeneric: sdkadapter.TickGenericValue{TickType: 49, Value: "1"},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(tick generic) error = %v", err)
	}
	generic, ok := msg.(sdkadapter.TickGeneric)
	if !ok {
		t.Fatalf("sdkEventToMessage(tick generic) type = %T, want sdkadapter.TickGeneric", msg)
	}
	if generic.ReqID != 65 || generic.TickType != 49 || generic.Value != "1" {
		t.Fatalf("tick generic = %+v, want copied value", generic)
	}

	msg, err = sdkEventToMessage(sdkadapter.Event{
		Kind:       sdkadapter.EventTickString,
		ReqID:      65,
		TickString: sdkadapter.TickStringValue{TickType: 45, Value: "1777651200"},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(tick string) error = %v", err)
	}
	str, ok := msg.(sdkadapter.TickString)
	if !ok {
		t.Fatalf("sdkEventToMessage(tick string) type = %T, want sdkadapter.TickString", msg)
	}
	if str.ReqID != 65 || str.TickType != 45 || str.Value != "1777651200" {
		t.Fatalf("tick string = %+v, want copied value", str)
	}

	msg, err = sdkEventToMessage(sdkadapter.Event{
		Kind:  sdkadapter.EventTickReqParams,
		ReqID: 65,
		TickReqParams: sdkadapter.TickReqParamsValue{
			MinTick:             "0.01",
			BBOExchange:         "NASDAQ",
			SnapshotPermissions: 7,
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(tick req params) error = %v", err)
	}
	params, ok := msg.(sdkadapter.TickReqParams)
	if !ok {
		t.Fatalf("sdkEventToMessage(tick req params) type = %T, want sdkadapter.TickReqParams", msg)
	}
	if params.ReqID != 65 || params.MinTick != "0.01" || params.BBOExchange != "NASDAQ" || params.SnapshotPermissions != 7 {
		t.Fatalf("tick req params = %+v, want copied params", params)
	}

	msg, err = sdkEventToMessage(sdkadapter.Event{Kind: sdkadapter.EventTickSnapshotEnd, ReqID: 65})
	if err != nil {
		t.Fatalf("sdkEventToMessage(tick snapshot end) error = %v", err)
	}
	end, ok := msg.(sdkadapter.TickSnapshotEnd)
	if !ok {
		t.Fatalf("sdkEventToMessage(tick snapshot end) type = %T, want sdkadapter.TickSnapshotEnd", msg)
	}
	if end.ReqID != 65 {
		t.Fatalf("tick snapshot end reqID = %d, want 65", end.ReqID)
	}
}

func TestSDKQuoteTickReqParamsSurfaceOnSubscription(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newReadySDKEngineForSubscriptionTest(t, adapter)
	client := &Client{engine: e}

	sub, err := client.MarketData().SubscribeQuotes(context.Background(), QuoteRequest{
		Contract: Contract{
			Symbol:   "AAPL",
			SecType:  SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
	})
	if err != nil {
		t.Fatalf("SubscribeQuotes() error = %v", err)
	}
	defer func() { _ = sub.Close() }()

	commands := adapter.Commands()
	if len(commands) != 1 || commands[0].Kind != sdkadapter.CommandQuote {
		t.Fatalf("commands = %+v, want one quote command", commands)
	}
	reqID := commands[0].Quote.ReqID

	runEngineStep(t, e, func() {
		e.handleIncoming(sdkadapter.TickReqParams{
			ReqID:               reqID,
			MinTick:             "0.01",
			BBOExchange:         "9c0001",
			SnapshotPermissions: 7,
		})
	})

	select {
	case update := <-sub.Events():
		if update.Changed != QuoteFieldRequestParams {
			t.Fatalf("Changed = %v, want QuoteFieldRequestParams", update.Changed)
		}
		if update.Snapshot.Available&QuoteFieldRequestParams == 0 {
			t.Fatalf("Available = %v, want QuoteFieldRequestParams", update.Snapshot.Available)
		}
		if got := update.Snapshot.MinTick.String(); got != "0.01" {
			t.Fatalf("MinTick = %s, want 0.01", got)
		}
		if update.Snapshot.BBOExchange != "9c0001" {
			t.Fatalf("BBOExchange = %q, want 9c0001", update.Snapshot.BBOExchange)
		}
		if update.Snapshot.SnapshotPermissions != 7 {
			t.Fatalf("SnapshotPermissions = %d, want 7", update.Snapshot.SnapshotPermissions)
		}
	default:
		t.Fatal("SubscribeQuotes() emitted no tick request params update")
	}
}

func newReadySDKEngineForSubscriptionTest(t *testing.T, adapter sdkadapter.Adapter) *engine {
	t.Helper()

	cfg := defaultConfig()
	e := &engine{
		cfg:                      cfg,
		cmds:                     make(chan func(), 256),
		incoming:                 make(chan any, 256),
		adapterErr:               make(chan error, 8),
		ready:                    make(chan error, 1),
		done:                     make(chan struct{}),
		events:                   newObserver[Event](cfg.eventBuffer),
		adapter:                  adapter,
		keyed:                    make(map[int]*route),
		singletons:               make(map[string]*route),
		orders:                   make(map[int64]*orderRoute),
		executions:               newExecutionCorrelator(),
		execToOrder:              make(map[string]int64),
		nextReqID:                1,
		recentHistoricalRequests: make(map[string]time.Time),
		snapshot: Snapshot{
			State:           StateReady,
			ConnectionSeq:   1,
			ServerVersion:   203,
			ManagedAccounts: []string{"DU1"},
			NextValidID:     1,
		},
	}
	go e.run()
	t.Cleanup(func() {
		_ = e.Close()
		_ = e.Wait()
	})
	return e
}

func runEngineStep(t *testing.T, e *engine, fn func()) {
	t.Helper()

	done := make(chan struct{})
	e.enqueue(func() {
		fn()
		close(done)
	})
	<-done
}

func TestSDKRealTimeBarsUsesSDKCommandsAndEvents(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	contract := sdkadapter.Contract{
		Symbol:   "AAPL",
		SecType:  "STK",
		Exchange: "SMART",
		Currency: "USD",
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.RealTimeBarsRequest{
		ReqID:      62,
		Contract:   contract,
		WhatToShow: "TRADES",
		UseRTH:     true,
	}); err != nil {
		t.Fatalf("sendSDKContext(RealTimeBarsRequest) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.CancelRealTimeBars{ReqID: 62}); err != nil {
		t.Fatalf("sendSDKContext(CancelRealTimeBars) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 2 {
		t.Fatalf("commands len = %d, want 2", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandRealTimeBars {
		t.Fatalf("real-time bars command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandRealTimeBars)
	}
	if commands[0].RealTimeBars.ReqID != 62 || commands[0].RealTimeBars.WhatToShow != "TRADES" || !commands[0].RealTimeBars.UseRTH {
		t.Fatalf("real-time bars command = %+v, want reqID 62 TRADES useRTH", commands[0].RealTimeBars)
	}
	if commands[0].RealTimeBars.Contract.Symbol != "AAPL" || commands[0].RealTimeBars.Contract.Exchange != "SMART" {
		t.Fatalf("real-time bars contract = %+v, want AAPL SMART", commands[0].RealTimeBars.Contract)
	}
	if commands[1].Kind != sdkadapter.CommandCancelRealTimeBars || commands[1].CancelRealTimeBars.ReqID != 62 {
		t.Fatalf("cancel real-time bars command = %+v, want reqID 62", commands[1])
	}

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind:  sdkadapter.EventRealTimeBar,
		ReqID: 62,
		RealTimeBar: sdkadapter.HistoricalBarValue{
			Time:   "1777651200",
			Open:   "187.1",
			High:   "188.2",
			Low:    "186.9",
			Close:  "187.8",
			Volume: "1000",
			WAP:    "187.5",
			Count:  "12",
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(real-time bar) error = %v", err)
	}
	got, ok := msg.(sdkadapter.RealTimeBar)
	if !ok {
		t.Fatalf("sdkEventToMessage(real-time bar) type = %T, want sdkadapter.RealTimeBar", msg)
	}
	if got.ReqID != 62 || got.Time != "1777651200" || got.Close != "187.8" || got.Volume != "1000" {
		t.Fatalf("real-time bar = %+v, want copied values", got)
	}
}

func TestSDKTickByTickUsesSDKCommandsAndEvents(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	contract := sdkadapter.Contract{
		Symbol:   "AAPL",
		SecType:  "STK",
		Exchange: "SMART",
		Currency: "USD",
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.TickByTickRequest{
		ReqID:         63,
		Contract:      contract,
		TickType:      string(TickByTickLast),
		NumberOfTicks: 5,
		IgnoreSize:    true,
	}); err != nil {
		t.Fatalf("sendSDKContext(TickByTickRequest) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.CancelTickByTick{ReqID: 63}); err != nil {
		t.Fatalf("sendSDKContext(CancelTickByTick) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 2 {
		t.Fatalf("commands len = %d, want 2", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandTickByTick {
		t.Fatalf("tick-by-tick command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandTickByTick)
	}
	if commands[0].TickByTick.ReqID != 63 || commands[0].TickByTick.TickType != "Last" || commands[0].TickByTick.NumberOfTicks != 5 || !commands[0].TickByTick.IgnoreSize {
		t.Fatalf("tick-by-tick command = %+v, want reqID 63 Last 5 ignoreSize", commands[0].TickByTick)
	}
	if commands[0].TickByTick.Contract.Symbol != "AAPL" || commands[0].TickByTick.Contract.Exchange != "SMART" {
		t.Fatalf("tick-by-tick contract = %+v, want AAPL SMART", commands[0].TickByTick.Contract)
	}
	if commands[1].Kind != sdkadapter.CommandCancelTickByTick || commands[1].CancelTickByTick.ReqID != 63 {
		t.Fatalf("cancel tick-by-tick command = %+v, want reqID 63", commands[1])
	}

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind:  sdkadapter.EventTickByTick,
		ReqID: 63,
		TickByTick: sdkadapter.TickByTickValue{
			TickType:          1,
			Time:              "1777651200",
			Price:             "188.1",
			Size:              "10",
			Exchange:          "NASDAQ",
			SpecialConditions: "@",
			TickAttribLast:    2,
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(tick-by-tick last) error = %v", err)
	}
	last, ok := msg.(sdkadapter.TickByTickData)
	if !ok {
		t.Fatalf("sdkEventToMessage(tick-by-tick last) type = %T, want sdkadapter.TickByTickData", msg)
	}
	if last.ReqID != 63 || last.TickType != 1 || last.Price != "188.1" || last.TickAttribLast != 2 || last.Exchange != "NASDAQ" {
		t.Fatalf("tick-by-tick last = %+v, want copied last tick", last)
	}

	msg, err = sdkEventToMessage(sdkadapter.Event{
		Kind:  sdkadapter.EventTickByTick,
		ReqID: 63,
		TickByTick: sdkadapter.TickByTickValue{
			TickType:         3,
			Time:             "1777651201",
			BidPrice:         "188.0",
			AskPrice:         "188.2",
			BidSize:          "20",
			AskSize:          "30",
			TickAttribBidAsk: 3,
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(tick-by-tick bid ask) error = %v", err)
	}
	bidAsk, ok := msg.(sdkadapter.TickByTickData)
	if !ok {
		t.Fatalf("sdkEventToMessage(tick-by-tick bid ask) type = %T, want sdkadapter.TickByTickData", msg)
	}
	if bidAsk.ReqID != 63 || bidAsk.TickType != 3 || bidAsk.AskPrice != "188.2" || bidAsk.TickAttribBidAsk != 3 {
		t.Fatalf("tick-by-tick bid ask = %+v, want copied bid ask tick", bidAsk)
	}

	msg, err = sdkEventToMessage(sdkadapter.Event{
		Kind:  sdkadapter.EventTickByTick,
		ReqID: 63,
		TickByTick: sdkadapter.TickByTickValue{
			TickType: 4,
			Time:     "1777651202",
			MidPoint: "188.15",
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(tick-by-tick midpoint) error = %v", err)
	}
	midpoint, ok := msg.(sdkadapter.TickByTickData)
	if !ok {
		t.Fatalf("sdkEventToMessage(tick-by-tick midpoint) type = %T, want sdkadapter.TickByTickData", msg)
	}
	if midpoint.ReqID != 63 || midpoint.TickType != 4 || midpoint.MidPoint != "188.15" {
		t.Fatalf("tick-by-tick midpoint = %+v, want copied midpoint tick", midpoint)
	}
}

func TestSDKMarketDepthUsesSDKCommandsAndEvents(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	contract := sdkadapter.Contract{
		Symbol:   "AAPL",
		SecType:  "STK",
		Exchange: "SMART",
		Currency: "USD",
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.MarketDepthRequest{
		ReqID:        64,
		Contract:     contract,
		NumRows:      5,
		IsSmartDepth: true,
	}); err != nil {
		t.Fatalf("sendSDKContext(MarketDepthRequest) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.CancelMarketDepth{ReqID: 64, IsSmartDepth: true}); err != nil {
		t.Fatalf("sendSDKContext(CancelMarketDepth) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 2 {
		t.Fatalf("commands len = %d, want 2", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandMarketDepth {
		t.Fatalf("market depth command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandMarketDepth)
	}
	if commands[0].MarketDepth.ReqID != 64 || commands[0].MarketDepth.NumRows != 5 || !commands[0].MarketDepth.IsSmartDepth {
		t.Fatalf("market depth command = %+v, want reqID 64 5 smart depth", commands[0].MarketDepth)
	}
	if commands[0].MarketDepth.Contract.Symbol != "AAPL" || commands[0].MarketDepth.Contract.Exchange != "SMART" {
		t.Fatalf("market depth contract = %+v, want AAPL SMART", commands[0].MarketDepth.Contract)
	}
	if commands[1].Kind != sdkadapter.CommandCancelMarketDepth || commands[1].CancelMarketDepth.ReqID != 64 || !commands[1].CancelMarketDepth.IsSmartDepth {
		t.Fatalf("cancel market depth command = %+v, want smart-depth reqID 64", commands[1])
	}

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind:  sdkadapter.EventMarketDepth,
		ReqID: 64,
		MarketDepth: sdkadapter.MarketDepthValue{
			Position:  1,
			Operation: int(DepthUpdate),
			Side:      int(BookBid),
			Price:     "188.1",
			Size:      "100",
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(market depth) error = %v", err)
	}
	l1, ok := msg.(sdkadapter.MarketDepthUpdate)
	if !ok {
		t.Fatalf("sdkEventToMessage(market depth) type = %T, want sdkadapter.MarketDepthUpdate", msg)
	}
	if l1.ReqID != 64 || l1.Position != 1 || l1.Operation != int(DepthUpdate) || l1.Side != int(BookBid) || l1.Price != "188.1" || l1.Size != "100" {
		t.Fatalf("market depth update = %+v, want copied L1 row", l1)
	}

	msg, err = sdkEventToMessage(sdkadapter.Event{
		Kind:  sdkadapter.EventMarketDepthL2,
		ReqID: 64,
		MarketDepthL2: sdkadapter.MarketDepthL2Value{
			Position:     2,
			MarketMaker:  "ISLAND",
			Operation:    int(DepthInsert),
			Side:         int(BookAsk),
			Price:        "188.2",
			Size:         "75",
			IsSmartDepth: true,
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(market depth L2) error = %v", err)
	}
	l2, ok := msg.(sdkadapter.MarketDepthL2Update)
	if !ok {
		t.Fatalf("sdkEventToMessage(market depth L2) type = %T, want sdkadapter.MarketDepthL2Update", msg)
	}
	if l2.ReqID != 64 || l2.Position != 2 || l2.MarketMaker != "ISLAND" || l2.Operation != int(DepthInsert) || l2.Side != int(BookAsk) || l2.Price != "188.2" || !l2.IsSmartDepth {
		t.Fatalf("market depth L2 update = %+v, want copied smart-depth row", l2)
	}
}

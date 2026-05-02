package ibkr

import (
	"context"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
)

func TestSDKHistoricalBarsUsesSDKCommandsAndEvents(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	contract := sdkadapter.Contract{
		Symbol:   "AAPL",
		SecType:  "STK",
		Exchange: "SMART",
		Currency: "USD",
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.HistoricalBarsRequest{
		ReqID:        40,
		Contract:     contract,
		EndDateTime:  "20260501 16:00:00",
		Duration:     "1 D",
		BarSize:      "1 hour",
		WhatToShow:   "TRADES",
		UseRTH:       true,
		KeepUpToDate: true,
	}); err != nil {
		t.Fatalf("sendSDKContext(HistoricalBarsRequest) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.CancelHistoricalData{ReqID: 40}); err != nil {
		t.Fatalf("sendSDKContext(CancelHistoricalData) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 2 {
		t.Fatalf("commands len = %d, want 2", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandHistoricalData {
		t.Fatalf("historical data command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandHistoricalData)
	}
	if commands[0].HistoricalData.ReqID != 40 || commands[0].HistoricalData.Duration != "1 D" || commands[0].HistoricalData.BarSize != "1 hour" {
		t.Fatalf("historical data command = %+v, want reqID 40 duration/bar size", commands[0].HistoricalData)
	}
	if commands[0].HistoricalData.WhatToShow != "TRADES" || !commands[0].HistoricalData.UseRTH || !commands[0].HistoricalData.KeepUpToDate {
		t.Fatalf("historical data command = %+v, want TRADES/useRTH/keepUpToDate", commands[0].HistoricalData)
	}
	if commands[0].HistoricalData.Contract.Symbol != "AAPL" || commands[0].HistoricalData.Contract.Exchange != "SMART" {
		t.Fatalf("historical data contract = %+v, want AAPL SMART", commands[0].HistoricalData.Contract)
	}
	if commands[1].Kind != sdkadapter.CommandCancelHistoricalData {
		t.Fatalf("cancel historical data command kind = %s, want %s", commands[1].Kind, sdkadapter.CommandCancelHistoricalData)
	}
	if commands[1].CancelHistoricalData.ReqID != 40 {
		t.Fatalf("cancel historical data reqID = %d, want 40", commands[1].CancelHistoricalData.ReqID)
	}

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind:  sdkadapter.EventHistoricalData,
		ReqID: 40,
		HistoricalBar: sdkadapter.HistoricalBarValue{
			Time:   "20260501  09:30:00",
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
		t.Fatalf("sdkEventToMessage(historical data) error = %v", err)
	}
	bar, ok := msg.(sdkadapter.HistoricalBar)
	if !ok {
		t.Fatalf("sdkEventToMessage(historical data) type = %T, want sdkadapter.HistoricalBar", msg)
	}
	if bar.ReqID != 40 || bar.Time != "20260501  09:30:00" || bar.Close != "187.8" || bar.Volume != "1000" {
		t.Fatalf("historical bar = %+v, want copied values", bar)
	}

	msg, err = sdkEventToMessage(sdkadapter.Event{Kind: sdkadapter.EventHistoricalDataEnd, ReqID: 40})
	if err != nil {
		t.Fatalf("sdkEventToMessage(historical data end) error = %v", err)
	}
	end, ok := msg.(sdkadapter.HistoricalBarsEnd)
	if !ok {
		t.Fatalf("sdkEventToMessage(historical data end) type = %T, want sdkadapter.HistoricalBarsEnd", msg)
	}
	if end.ReqID != 40 {
		t.Fatalf("historical bars end reqID = %d, want 40", end.ReqID)
	}

	msg, err = sdkEventToMessage(sdkadapter.Event{
		Kind:  sdkadapter.EventHistoricalDataUpdate,
		ReqID: 40,
		HistoricalBar: sdkadapter.HistoricalBarValue{
			Time:  "20260501  10:30:00",
			Open:  "187.8",
			High:  "189.0",
			Low:   "187.2",
			Close: "188.4",
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(historical data update) error = %v", err)
	}
	update, ok := msg.(sdkadapter.HistoricalDataUpdate)
	if !ok {
		t.Fatalf("sdkEventToMessage(historical data update) type = %T, want sdkadapter.HistoricalDataUpdate", msg)
	}
	if update.ReqID != 40 || update.Time != "20260501  10:30:00" || update.Close != "188.4" {
		t.Fatalf("historical data update = %+v, want copied update", update)
	}
}

func TestSDKHistoricalScheduleUsesSDKEvent(t *testing.T) {
	t.Parallel()

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind:  sdkadapter.EventHistoricalSchedule,
		ReqID: 43,
		HistoricalSchedule: sdkadapter.HistoricalScheduleValue{
			StartDateTime: "20260501 09:30:00",
			EndDateTime:   "20260501 16:00:00",
			TimeZone:      "US/Eastern",
			Sessions: []sdkadapter.HistoricalScheduleSessionValue{{
				StartDateTime: "20260501 09:30:00",
				EndDateTime:   "20260501 16:00:00",
				RefDate:       "20260501",
			}},
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(historical schedule) error = %v", err)
	}
	got, ok := msg.(sdkadapter.HistoricalScheduleResponse)
	if !ok {
		t.Fatalf("sdkEventToMessage(historical schedule) type = %T, want sdkadapter.HistoricalScheduleResponse", msg)
	}
	if got.ReqID != 43 || got.TimeZone != "US/Eastern" || len(got.Sessions) != 1 {
		t.Fatalf("historical schedule = %+v, want reqID 43 timezone and one session", got)
	}
	if got.Sessions[0].RefDate != "20260501" || got.Sessions[0].StartDateTime != "20260501 09:30:00" {
		t.Fatalf("historical schedule session = %+v, want copied session", got.Sessions[0])
	}
}

func TestSDKHistoricalBarsSubscriptionPublicRouteReplaysOfficialFixture(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan struct {
		sub *Subscription[Bar]
		err error
	}, 1)
	go func() {
		sub, err := client.History().SubscribeBars(ctx, HistoricalBarsRequest{
			Contract: Contract{
				Symbol:   "AAPL",
				SecType:  SecTypeStock,
				Exchange: "SMART",
				Currency: "USD",
			},
			Duration:   Days(1),
			BarSize:    Bar1Hour,
			WhatToShow: ShowTrades,
			UseRTH:     true,
		})
		resultCh <- struct {
			sub *Subscription[Bar]
			err error
		}{sub: sub, err: err}
	}()

	runNextEngineCommand(t, e)
	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandHistoricalData ||
		!command.HistoricalData.KeepUpToDate ||
		command.HistoricalData.Contract.Symbol != "AAPL" ||
		command.HistoricalData.Duration != "1 D" ||
		command.HistoricalData.BarSize != "1 hour" {
		t.Fatalf("historical bars subscription command = %+v, want AAPL 1D/1h keep-up", command)
	}

	result := receiveHistoricalBarsSubscriptionResult(t, resultCh)
	if result.err != nil {
		t.Fatalf("History().SubscribeBars() error = %v", result.err)
	}

	const path = "internal/sdkadapter/testdata/fixtures/official_sdk_historical_bars_keepup_short_20260502.json"
	barEvent := fixtureEvent(t, path, sdkadapter.EventHistoricalData, 406)
	barEvent.ReqID = command.HistoricalData.ReqID
	dispatchSDKFixtureEvent(t, e, barEvent)
	select {
	case bar := <-result.sub.Events():
		if !bar.Time.Equal(time.Date(2026, 5, 1, 9, 30, 0, 0, time.UTC)) ||
			bar.Close.String() != "283.94999999999999" ||
			bar.Volume.String() != "15127267" {
			t.Fatalf("SubscribeBars() bar = %+v, want first captured AAPL RTH bar", bar)
		}
	case <-time.After(time.Second):
		t.Fatal("History().SubscribeBars() did not emit replayed bar")
	}

	end := fixtureEvent(t, path, sdkadapter.EventHistoricalDataEnd, 406)
	end.ReqID = command.HistoricalData.ReqID
	dispatchSDKFixtureEvent(t, e, end)
	if err := result.sub.AwaitSnapshot(ctx); err != nil {
		t.Fatalf("History().SubscribeBars().AwaitSnapshot() error = %v", err)
	}
	if err := result.sub.Close(); err != nil {
		t.Fatalf("Subscription.Close() error = %v", err)
	}
	runNextEngineCommand(t, e)
	commands := adapter.Commands()
	if len(commands) != 2 {
		t.Fatalf("commands len = %d, want request and cancel: %+v", len(commands), commands)
	}
	cancelCommand := commands[1]
	if cancelCommand.Kind != sdkadapter.CommandCancelHistoricalData ||
		cancelCommand.CancelHistoricalData.ReqID != command.HistoricalData.ReqID {
		t.Fatalf("historical bars cancel command = %+v, want reqID %d", cancelCommand, command.HistoricalData.ReqID)
	}
}

func TestSDKHistoricalTicksUsesSDKCommandsAndEvents(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	contract := sdkadapter.Contract{
		Symbol:   "AAPL",
		SecType:  "STK",
		Exchange: "SMART",
		Currency: "USD",
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.HistoricalTicksRequest{
		ReqID:         44,
		Contract:      contract,
		StartDateTime: "20260501 15:59:00",
		EndDateTime:   "20260501 16:00:00",
		NumberOfTicks: 100,
		WhatToShow:    "TRADES",
		UseRTH:        true,
		IgnoreSize:    true,
	}); err != nil {
		t.Fatalf("sendSDKContext(HistoricalTicksRequest) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.CancelHistoricalTicks{ReqID: 44}); err != nil {
		t.Fatalf("sendSDKContext(CancelHistoricalTicks) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 2 {
		t.Fatalf("commands len = %d, want 2", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandHistoricalTicks {
		t.Fatalf("historical ticks command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandHistoricalTicks)
	}
	if commands[0].HistoricalTicks.ReqID != 44 || commands[0].HistoricalTicks.NumberOfTicks != 100 || commands[0].HistoricalTicks.WhatToShow != "TRADES" {
		t.Fatalf("historical ticks command = %+v, want reqID 44 100 TRADES", commands[0].HistoricalTicks)
	}
	if !commands[0].HistoricalTicks.UseRTH || !commands[0].HistoricalTicks.IgnoreSize {
		t.Fatalf("historical ticks command = %+v, want useRTH/ignoreSize", commands[0].HistoricalTicks)
	}
	if commands[1].Kind != sdkadapter.CommandCancelHistoricalTicks || commands[1].CancelHistoricalTicks.ReqID != 44 {
		t.Fatalf("cancel historical ticks command = %+v, want reqID 44", commands[1])
	}

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind:                sdkadapter.EventHistoricalTicks,
		ReqID:               44,
		HistoricalTicksDone: true,
		HistoricalTicks: []sdkadapter.HistoricalTickValue{{
			Time:  "1777651200",
			Price: "188.1",
			Size:  "10",
		}},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(historical ticks) error = %v", err)
	}
	midpoint, ok := msg.(sdkadapter.HistoricalTicksResponse)
	if !ok {
		t.Fatalf("sdkEventToMessage(historical ticks) type = %T, want sdkadapter.HistoricalTicksResponse", msg)
	}
	if midpoint.ReqID != 44 || !midpoint.Done || len(midpoint.Ticks) != 1 || midpoint.Ticks[0].Price != "188.1" {
		t.Fatalf("historical ticks = %+v, want copied midpoint tick", midpoint)
	}

	msg, err = sdkEventToMessage(sdkadapter.Event{
		Kind:                sdkadapter.EventHistoricalTicksBidAsk,
		ReqID:               44,
		HistoricalTicksDone: true,
		HistoricalTicksBidAsk: []sdkadapter.HistoricalTickBidAskValue{{
			TickAttrib: 3,
			Time:       "1777651201",
			BidPrice:   "188.0",
			AskPrice:   "188.2",
			BidSize:    "20",
			AskSize:    "30",
		}},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(historical bid ask ticks) error = %v", err)
	}
	bidAsk, ok := msg.(sdkadapter.HistoricalTicksBidAskResponse)
	if !ok {
		t.Fatalf("sdkEventToMessage(historical bid ask ticks) type = %T, want sdkadapter.HistoricalTicksBidAskResponse", msg)
	}
	if bidAsk.ReqID != 44 || !bidAsk.Done || len(bidAsk.Ticks) != 1 || bidAsk.Ticks[0].TickAttrib != 3 || bidAsk.Ticks[0].AskPrice != "188.2" {
		t.Fatalf("historical bid ask ticks = %+v, want copied bid ask tick", bidAsk)
	}

	msg, err = sdkEventToMessage(sdkadapter.Event{
		Kind:                sdkadapter.EventHistoricalTicksLast,
		ReqID:               44,
		HistoricalTicksDone: true,
		HistoricalTicksLast: []sdkadapter.HistoricalTickLastValue{{
			TickAttrib:        2,
			Time:              "1777651202",
			Price:             "188.3",
			Size:              "15",
			Exchange:          "NASDAQ",
			SpecialConditions: "@",
		}},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(historical last ticks) error = %v", err)
	}
	last, ok := msg.(sdkadapter.HistoricalTicksLastResponse)
	if !ok {
		t.Fatalf("sdkEventToMessage(historical last ticks) type = %T, want sdkadapter.HistoricalTicksLastResponse", msg)
	}
	if last.ReqID != 44 || !last.Done || len(last.Ticks) != 1 || last.Ticks[0].TickAttrib != 2 || last.Ticks[0].Exchange != "NASDAQ" {
		t.Fatalf("historical last ticks = %+v, want copied last tick", last)
	}
}

func TestSDKHistoricalTicksTradesPublicRouteReplaysOfficialFixture(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan struct {
		result HistoricalTicksResult
		err    error
	}, 1)
	go func() {
		result, err := client.History().Ticks(ctx, HistoricalTicksRequest{
			Contract: Contract{
				Symbol:   "AAPL",
				SecType:  SecTypeStock,
				Exchange: "SMART",
				Currency: "USD",
			},
			EndTime:       time.Date(2026, 5, 1, 20, 0, 0, 0, time.UTC),
			NumberOfTicks: 10,
			WhatToShow:    ShowTrades,
			UseRTH:        true,
			IgnoreSize:    true,
		})
		resultCh <- struct {
			result HistoricalTicksResult
			err    error
		}{result: result, err: err}
	}()

	runNextEngineCommand(t, e)
	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandHistoricalTicks {
		t.Fatalf("command kind = %s, want %s", command.Kind, sdkadapter.CommandHistoricalTicks)
	}
	if command.HistoricalTicks.Contract.Symbol != "AAPL" ||
		command.HistoricalTicks.WhatToShow != "TRADES" ||
		command.HistoricalTicks.NumberOfTicks != 10 ||
		!command.HistoricalTicks.UseRTH ||
		!command.HistoricalTicks.IgnoreSize {
		t.Fatalf("historical ticks command = %+v, want AAPL TRADES request", command.HistoricalTicks)
	}

	event := fixtureEvent(t, "internal/sdkadapter/testdata/fixtures/official_sdk_historical_ticks_trades_short_20260502.json", sdkadapter.EventHistoricalTicksLast, 405)
	event.ReqID = command.HistoricalTicks.ReqID
	dispatchSDKFixtureEvent(t, e, event)

	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("History().Ticks(TRADES) error = %v", result.err)
		}
		if len(result.result.Last) < 10 {
			t.Fatalf("History().Ticks(TRADES) last len = %d, want at least 10", len(result.result.Last))
		}
		first := result.result.Last[0]
		if !first.Time.Equal(time.Unix(1777665599, 0).UTC()) ||
			first.Price.String() != "280.04000000000002" ||
			first.Size.String() != "100" ||
			first.Exchange != "NASDAQ" ||
			first.SpecialConditions != " F  " {
			t.Fatalf("History().Ticks(TRADES) first = %+v, want captured first trade", first)
		}
	case <-time.After(time.Second):
		t.Fatal("History().Ticks(TRADES) did not return")
	}
}

func TestSDKHistoricalTicksBidAskPublicRouteReplaysOfficialFixture(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan struct {
		result HistoricalTicksResult
		err    error
	}, 1)
	go func() {
		result, err := client.History().Ticks(ctx, HistoricalTicksRequest{
			Contract: Contract{
				Symbol:   "AAPL",
				SecType:  SecTypeStock,
				Exchange: "SMART",
				Currency: "USD",
			},
			EndTime:       time.Date(2026, 5, 1, 20, 0, 0, 0, time.UTC),
			NumberOfTicks: 10,
			WhatToShow:    ShowBidAsk,
			UseRTH:        true,
			IgnoreSize:    true,
		})
		resultCh <- struct {
			result HistoricalTicksResult
			err    error
		}{result: result, err: err}
	}()

	runNextEngineCommand(t, e)
	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandHistoricalTicks {
		t.Fatalf("command kind = %s, want %s", command.Kind, sdkadapter.CommandHistoricalTicks)
	}
	if command.HistoricalTicks.Contract.Symbol != "AAPL" ||
		command.HistoricalTicks.WhatToShow != "BID_ASK" ||
		command.HistoricalTicks.NumberOfTicks != 10 ||
		!command.HistoricalTicks.UseRTH ||
		!command.HistoricalTicks.IgnoreSize {
		t.Fatalf("historical ticks command = %+v, want AAPL BID_ASK request", command.HistoricalTicks)
	}

	event := fixtureEvent(t, "internal/sdkadapter/testdata/fixtures/official_sdk_historical_ticks_bidask_short_20260502.json", sdkadapter.EventHistoricalTicksBidAsk, 404)
	event.ReqID = command.HistoricalTicks.ReqID
	dispatchSDKFixtureEvent(t, e, event)

	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("History().Ticks(BID_ASK) error = %v", result.err)
		}
		if len(result.result.BidAsk) < 10 {
			t.Fatalf("History().Ticks(BID_ASK) bidAsk len = %d, want at least 10", len(result.result.BidAsk))
		}
		first := result.result.BidAsk[0]
		if !first.Time.Equal(time.Unix(1777665598, 0).UTC()) ||
			first.BidPrice.String() != "280.04000000000002" ||
			first.AskPrice.String() != "280.06999999999999" ||
			first.BidSize.String() != "200" ||
			first.AskSize.String() != "80" {
			t.Fatalf("History().Ticks(BID_ASK) first = %+v, want captured first bid/ask", first)
		}
	case <-time.After(time.Second):
		t.Fatal("History().Ticks(BID_ASK) did not return")
	}
}

func TestSDKHistoryHeadTimestampUsesSDKCommandAndEvent(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	contract := sdkadapter.Contract{
		Symbol:   "AAPL",
		SecType:  "STK",
		Exchange: "SMART",
		Currency: "USD",
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.HeadTimestampRequest{
		ReqID:      41,
		Contract:   contract,
		WhatToShow: "TRADES",
		UseRTH:     true,
	}); err != nil {
		t.Fatalf("sendSDKContext(HeadTimestampRequest) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.CancelHeadTimestamp{ReqID: 41}); err != nil {
		t.Fatalf("sendSDKContext(CancelHeadTimestamp) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 2 {
		t.Fatalf("commands len = %d, want 2", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandHeadTimestamp {
		t.Fatalf("head timestamp command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandHeadTimestamp)
	}
	if commands[0].HeadTimestamp.ReqID != 41 || commands[0].HeadTimestamp.WhatToShow != "TRADES" || !commands[0].HeadTimestamp.UseRTH {
		t.Fatalf("head timestamp command = %+v, want reqID 41 TRADES useRTH true", commands[0].HeadTimestamp)
	}
	if commands[0].HeadTimestamp.Contract.Symbol != "AAPL" || commands[0].HeadTimestamp.Contract.Exchange != "SMART" {
		t.Fatalf("head timestamp contract = %+v, want AAPL SMART", commands[0].HeadTimestamp.Contract)
	}
	if commands[1].Kind != sdkadapter.CommandCancelHeadTimestamp {
		t.Fatalf("cancel head timestamp command kind = %s, want %s", commands[1].Kind, sdkadapter.CommandCancelHeadTimestamp)
	}
	if commands[1].CancelHeadTimestamp.ReqID != 41 {
		t.Fatalf("cancel head timestamp reqID = %d, want 41", commands[1].CancelHeadTimestamp.ReqID)
	}

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind:          sdkadapter.EventHeadTimestamp,
		ReqID:         41,
		HeadTimestamp: "20260501-12:00:00",
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(head timestamp) error = %v", err)
	}
	got, ok := msg.(sdkadapter.HeadTimestamp)
	if !ok {
		t.Fatalf("sdkEventToMessage(head timestamp) type = %T, want sdkadapter.HeadTimestamp", msg)
	}
	if got.ReqID != 41 || got.Timestamp != "20260501-12:00:00" {
		t.Fatalf("head timestamp = %+v, want reqID 41 timestamp", got)
	}
}

func TestSDKHistoryHistogramUsesSDKCommandAndEvent(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	contract := sdkadapter.Contract{
		Symbol:   "AAPL",
		SecType:  "STK",
		Exchange: "SMART",
		Currency: "USD",
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.HistogramDataRequest{
		ReqID:    42,
		Contract: contract,
		UseRTH:   true,
		Period:   "1 week",
	}); err != nil {
		t.Fatalf("sendSDKContext(HistogramDataRequest) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.CancelHistogramData{ReqID: 42}); err != nil {
		t.Fatalf("sendSDKContext(CancelHistogramData) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 2 {
		t.Fatalf("commands len = %d, want 2", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandHistogramData {
		t.Fatalf("histogram command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandHistogramData)
	}
	if commands[0].HistogramData.ReqID != 42 || commands[0].HistogramData.Period != "1 week" || !commands[0].HistogramData.UseRTH {
		t.Fatalf("histogram command = %+v, want reqID 42 1 week useRTH true", commands[0].HistogramData)
	}
	if commands[0].HistogramData.Contract.Symbol != "AAPL" || commands[0].HistogramData.Contract.Exchange != "SMART" {
		t.Fatalf("histogram contract = %+v, want AAPL SMART", commands[0].HistogramData.Contract)
	}
	if commands[1].Kind != sdkadapter.CommandCancelHistogramData {
		t.Fatalf("cancel histogram command kind = %s, want %s", commands[1].Kind, sdkadapter.CommandCancelHistogramData)
	}
	if commands[1].CancelHistogramData.ReqID != 42 {
		t.Fatalf("cancel histogram reqID = %d, want 42", commands[1].CancelHistogramData.ReqID)
	}

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind:  sdkadapter.EventHistogramData,
		ReqID: 42,
		HistogramData: []sdkadapter.HistogramDataValue{{
			Price: "187.5",
			Size:  "100",
		}},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(histogram) error = %v", err)
	}
	got, ok := msg.(sdkadapter.HistogramDataResponse)
	if !ok {
		t.Fatalf("sdkEventToMessage(histogram) type = %T, want sdkadapter.HistogramDataResponse", msg)
	}
	if got.ReqID != 42 || len(got.Entries) != 1 {
		t.Fatalf("histogram = %+v, want reqID 42 one entry", got)
	}
	if got.Entries[0].Price != "187.5" || got.Entries[0].Size != "100" {
		t.Fatalf("histogram entry = %+v, want 187.5/100", got.Entries[0])
	}
}

func receiveHistoricalBarsSubscriptionResult(t *testing.T, resultCh <-chan struct {
	sub *Subscription[Bar]
	err error
}) struct {
	sub *Subscription[Bar]
	err error
} {
	t.Helper()

	select {
	case result := <-resultCh:
		return result
	case <-time.After(time.Second):
		t.Fatal("History().SubscribeBars() did not return")
		return struct {
			sub *Subscription[Bar]
			err error
		}{}
	}
}

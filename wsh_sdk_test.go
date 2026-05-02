package ibkr

import (
	"context"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
)

func TestSDKWSHUsesSDKCommandsAndEvents(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	if err := e.sendSDKContext(context.Background(), sdkadapter.WSHMetaDataRequest{ReqID: 51}); err != nil {
		t.Fatalf("sendSDKContext(WSHMetaDataRequest) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.CancelWSHMetaData{ReqID: 51}); err != nil {
		t.Fatalf("sendSDKContext(CancelWSHMetaData) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.WSHEventDataRequest{
		ReqID:           52,
		ConID:           265598,
		Filter:          `{"watchlist":["AAPL"]}`,
		FillWatchlist:   true,
		FillPortfolio:   true,
		FillCompetitors: true,
		StartDate:       "20260501",
		EndDate:         "20260601",
		TotalLimit:      10,
	}); err != nil {
		t.Fatalf("sendSDKContext(WSHEventDataRequest) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.CancelWSHEventData{ReqID: 52}); err != nil {
		t.Fatalf("sendSDKContext(CancelWSHEventData) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 4 {
		t.Fatalf("commands len = %d, want 4", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandWSHMetaData || commands[0].WSHMetaData.ReqID != 51 {
		t.Fatalf("metadata command = %+v, want reqID 51", commands[0])
	}
	if commands[1].Kind != sdkadapter.CommandCancelWSHMetaData || commands[1].CancelWSHMetaData.ReqID != 51 {
		t.Fatalf("cancel metadata command = %+v, want reqID 51", commands[1])
	}
	if commands[2].Kind != sdkadapter.CommandWSHEventData {
		t.Fatalf("event-data command kind = %s, want %s", commands[2].Kind, sdkadapter.CommandWSHEventData)
	}
	eventCommand := commands[2].WSHEventData
	if eventCommand.ReqID != 52 || eventCommand.ConID != 265598 || eventCommand.Filter != `{"watchlist":["AAPL"]}` {
		t.Fatalf("event-data command = %+v, want reqID 52 conID 265598 filter", eventCommand)
	}
	if !eventCommand.FillWatchlist || !eventCommand.FillPortfolio || !eventCommand.FillCompetitors {
		t.Fatalf("event-data fill flags = %+v, want all true", eventCommand)
	}
	if eventCommand.StartDate != "20260501" || eventCommand.EndDate != "20260601" || eventCommand.TotalLimit != 10 {
		t.Fatalf("event-data date/limit = %+v, want 20260501/20260601/10", eventCommand)
	}
	if commands[3].Kind != sdkadapter.CommandCancelWSHEventData || commands[3].CancelWSHEventData.ReqID != 52 {
		t.Fatalf("cancel event-data command = %+v, want reqID 52", commands[3])
	}

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind:        sdkadapter.EventWSHMetaData,
		ReqID:       51,
		WSHDataJSON: `{"metadata":[]}`,
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(WSH metadata) error = %v", err)
	}
	meta, ok := msg.(sdkadapter.WSHMetaDataResponse)
	if !ok {
		t.Fatalf("sdkEventToMessage(WSH metadata) type = %T, want sdkadapter.WSHMetaDataResponse", msg)
	}
	if meta.ReqID != 51 || meta.DataJSON != `{"metadata":[]}` {
		t.Fatalf("metadata response = %+v, want reqID 51 JSON", meta)
	}

	msg, err = sdkEventToMessage(sdkadapter.Event{
		Kind:        sdkadapter.EventWSHEventData,
		ReqID:       52,
		WSHDataJSON: `{"events":[]}`,
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(WSH event data) error = %v", err)
	}
	events, ok := msg.(sdkadapter.WSHEventDataResponse)
	if !ok {
		t.Fatalf("sdkEventToMessage(WSH event data) type = %T, want sdkadapter.WSHEventDataResponse", msg)
	}
	if events.ReqID != 52 || events.DataJSON != `{"events":[]}` {
		t.Fatalf("event-data response = %+v, want reqID 52 JSON", events)
	}
}

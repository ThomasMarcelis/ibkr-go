package ibkr

import (
	"context"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
)

func TestSDKDisplayGroupSubscriptionUsesSDKCommandsAndEvents(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	if err := e.sendSDKContext(context.Background(), sdkadapter.SubscribeToGroupEventsRequest{ReqID: 91, GroupID: 3}); err != nil {
		t.Fatalf("sendSDKContext(SubscribeToGroupEventsRequest) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.UpdateDisplayGroupRequest{ReqID: 91, ContractInfo: "8314@SMART"}); err != nil {
		t.Fatalf("sendSDKContext(UpdateDisplayGroupRequest) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.UnsubscribeFromGroupEventsRequest{ReqID: 91}); err != nil {
		t.Fatalf("sendSDKContext(UnsubscribeFromGroupEventsRequest) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 3 {
		t.Fatalf("commands len = %d, want 3", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandSubscribeToGroupEvents {
		t.Fatalf("subscribe command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandSubscribeToGroupEvents)
	}
	if commands[0].SubscribeToGroupEvents.ReqID != 91 || commands[0].SubscribeToGroupEvents.GroupID != 3 {
		t.Fatalf("subscribe command = %+v, want reqID 91 groupID 3", commands[0].SubscribeToGroupEvents)
	}
	if commands[1].Kind != sdkadapter.CommandUpdateDisplayGroup {
		t.Fatalf("update command kind = %s, want %s", commands[1].Kind, sdkadapter.CommandUpdateDisplayGroup)
	}
	if commands[1].UpdateDisplayGroup.ReqID != 91 || commands[1].UpdateDisplayGroup.ContractInfo != "8314@SMART" {
		t.Fatalf("update command = %+v, want reqID 91 contract 8314@SMART", commands[1].UpdateDisplayGroup)
	}
	if commands[2].Kind != sdkadapter.CommandUnsubscribeFromGroupEvents {
		t.Fatalf("unsubscribe command kind = %s, want %s", commands[2].Kind, sdkadapter.CommandUnsubscribeFromGroupEvents)
	}
	if commands[2].UnsubscribeFromGroupEvents.ReqID != 91 {
		t.Fatalf("unsubscribe reqID = %d, want 91", commands[2].UnsubscribeFromGroupEvents.ReqID)
	}

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind:                     sdkadapter.EventDisplayGroupUpdated,
		ReqID:                    91,
		DisplayGroupContractInfo: "8314@SMART",
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(display group updated) error = %v", err)
	}
	got, ok := msg.(sdkadapter.DisplayGroupUpdated)
	if !ok {
		t.Fatalf("sdkEventToMessage(display group updated) type = %T, want sdkadapter.DisplayGroupUpdated", msg)
	}
	if got.ReqID != 91 || got.ContractInfo != "8314@SMART" {
		t.Fatalf("display group update = %+v, want reqID 91 contract 8314@SMART", got)
	}
}

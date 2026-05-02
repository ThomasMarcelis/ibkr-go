package ibkr

import (
	"context"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
)

func TestSDKFAConfigUsesSDKCommandAndEvent(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	if err := e.sendSDKContext(context.Background(), sdkadapter.RequestFA{FADataType: int(FADataGroups)}); err != nil {
		t.Fatalf("sendSDKContext(RequestFA) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 1 {
		t.Fatalf("commands len = %d, want 1", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandRequestFA {
		t.Fatalf("command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandRequestFA)
	}
	if commands[0].RequestFA.FADataType != int(FADataGroups) {
		t.Fatalf("FA data type = %d, want %d", commands[0].RequestFA.FADataType, FADataGroups)
	}

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind: sdkadapter.EventReceiveFA,
		ReceiveFA: sdkadapter.ReceiveFAValue{
			FADataType: int(FADataGroups),
			XML:        "<ListOfGroups/>",
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage() error = %v", err)
	}
	got, ok := msg.(sdkadapter.ReceiveFA)
	if !ok {
		t.Fatalf("sdkEventToMessage() type = %T, want sdkadapter.ReceiveFA", msg)
	}
	if got.FADataType != int(FADataGroups) || got.XML != "<ListOfGroups/>" {
		t.Fatalf("receive FA = %+v, want groups XML", got)
	}
}

func TestSDKReplaceFAUsesSDKCommandAndEvent(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	if err := e.sendSDKContext(context.Background(), sdkadapter.ReplaceFA{
		ReqID:      77,
		FADataType: int(FADataGroups),
		XML:        "<ListOfGroups/>",
	}); err != nil {
		t.Fatalf("sendSDKContext(ReplaceFA) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 1 {
		t.Fatalf("commands len = %d, want 1", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandReplaceFA {
		t.Fatalf("command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandReplaceFA)
	}
	if commands[0].ReplaceFA.ReqID != 77 || commands[0].ReplaceFA.FADataType != int(FADataGroups) || commands[0].ReplaceFA.XML != "<ListOfGroups/>" {
		t.Fatalf("replace FA command = %+v, want reqID/type/XML", commands[0].ReplaceFA)
	}

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind:             sdkadapter.EventReplaceFAEnd,
		ReqID:            77,
		ReplaceFAEndText: "ok",
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage() error = %v", err)
	}
	got, ok := msg.(sdkadapter.ReplaceFAEnd)
	if !ok {
		t.Fatalf("sdkEventToMessage() type = %T, want sdkadapter.ReplaceFAEnd", msg)
	}
	if got.ReqID != 77 || got.Text != "ok" {
		t.Fatalf("replace FA end = %+v, want reqID/text", got)
	}
}

func TestSDKReplaceFACompletesOnReplaceFAEnd(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{
		cmds:       make(chan func(), 2),
		done:       make(chan struct{}),
		adapter:    adapter,
		keyed:      make(map[int]*route),
		singletons: make(map[string]*route),
		nextReqID:  1,
		snapshot:   Snapshot{State: StateReady},
	}
	t.Cleanup(func() { close(e.done) })

	result := make(chan error, 1)
	go func() {
		result <- e.ReplaceFA(context.Background(), FADataGroups, "<ListOfGroups/>")
	}()

	select {
	case fn := <-e.cmds:
		fn()
	case <-time.After(time.Second):
		t.Fatal("ReplaceFA did not enqueue setup")
	}

	commands := adapter.Commands()
	if len(commands) != 1 {
		t.Fatalf("commands len = %d, want 1", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandReplaceFA {
		t.Fatalf("command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandReplaceFA)
	}
	reqID := commands[0].ReplaceFA.ReqID
	if reqID <= 0 {
		t.Fatalf("replace FA reqID = %d, want positive", reqID)
	}
	if _, ok := e.keyed[reqID]; !ok {
		t.Fatalf("replace FA route for reqID %d was not registered", reqID)
	}

	e.handleIncoming(sdkadapter.ReplaceFAEnd{ReqID: reqID, Text: "ok"})

	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("ReplaceFA() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("ReplaceFA did not complete on replaceFAEnd")
	}
	if _, ok := e.keyed[reqID]; ok {
		t.Fatalf("replace FA route for reqID %d was not removed", reqID)
	}
}

package sdkadapter

import (
	"context"
	"testing"
)

func TestReplayAdapterCopiesEventsAndCommands(t *testing.T) {
	source := []Event{{
		Kind:     EventManagedAccounts,
		Accounts: []string{"DU1"},
	}}
	adapter := NewReplayAdapter(source)
	source[0].Accounts[0] = "mutated"

	if err := adapter.Connect(context.Background(), ConnectRequest{}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	events, err := adapter.DrainEvents(context.Background(), 10)
	if err != nil {
		t.Fatalf("DrainEvents() error = %v", err)
	}
	if got := events[0].Accounts[0]; got != "DU1" {
		t.Fatalf("event account = %q, want copied DU1", got)
	}
	events[0].Accounts[0] = "changed"

	command := Command{Kind: CommandAccountSummary, ReqID: 7, Group: "All", Tags: []string{"NetLiquidation"}}
	if err := adapter.Submit(context.Background(), command); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	command.Tags[0] = "mutated"

	commands := adapter.Commands()
	if got := commands[0].Tags[0]; got != "NetLiquidation" {
		t.Fatalf("recorded command tag = %q, want copied NetLiquidation", got)
	}
}

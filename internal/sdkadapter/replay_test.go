package sdkadapter

import (
	"context"
	"strings"
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

	command := Command{
		Kind: CommandAccountSummary,
		AccountSummary: AccountSummaryCommand{
			ReqID: 7,
			Group: "All",
			Tags:  []string{"NetLiquidation"},
		},
	}
	if err := adapter.Submit(context.Background(), command); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	command.AccountSummary.Tags[0] = "mutated"

	commands := adapter.Commands()
	if got := commands[0].AccountSummary.Tags[0]; got != "NetLiquidation" {
		t.Fatalf("recorded command tag = %q, want copied NetLiquidation", got)
	}
}

func TestDecodeFixtureRequiresTraceableMetadata(t *testing.T) {
	_, err := DecodeFixture(strings.NewReader(`{"metadata":{"sdk_version":"10.46.01"},"events":[]}`))
	if err == nil {
		t.Fatal("DecodeFixture() error = nil, want missing metadata error")
	}
}

func TestReplayAdapterFromFixtureCopiesEvents(t *testing.T) {
	fixture := Fixture{
		Metadata: FixtureMetadata{
			SDKVersion:     "10.46.01",
			ServerVersion:  200,
			CapturedAt:     "2026-04-27T12:00:00Z",
			Scenario:       "sdkadapter-unit-copy",
			RedactionNotes: "unit fixture contains no account data",
			SourceSHA256:   "unit-schema-only",
		},
		Events: []Event{{
			Kind:     EventManagedAccounts,
			Accounts: []string{"DU1"},
		}},
	}

	adapter, err := NewReplayAdapterFromFixture(fixture)
	if err != nil {
		t.Fatalf("NewReplayAdapterFromFixture() error = %v", err)
	}
	fixture.Events[0].Accounts[0] = "mutated"

	events, err := adapter.DrainEvents(context.Background(), 1)
	if err != nil {
		t.Fatalf("DrainEvents() error = %v", err)
	}
	if got := events[0].Accounts[0]; got != "DU1" {
		t.Fatalf("event account = %q, want copied DU1", got)
	}
	if got := adapter.ServerVersion(); got != 200 {
		t.Fatalf("ServerVersion() = %d, want 200", got)
	}
}

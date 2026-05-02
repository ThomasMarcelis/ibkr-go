package sdkadapter

import (
	"context"
	"strings"
	"testing"
)

func TestReplayAdapterCopiesEventsAndCommands(t *testing.T) {
	source := []Event{{
		Kind: EventManagedAccounts,
		Accounts: []string{
			"DU1",
		},
	}, {
		Kind: EventFamilyCodes,
		FamilyCodes: []FamilyCodeValue{{
			AccountID:  "DU1",
			FamilyCode: "FAMILY",
		}},
	}, {
		Kind: EventMktDepthExchanges,
		DepthExchanges: []DepthExchangeValue{{
			Exchange:        "ISLAND",
			SecType:         "STK",
			ListingExch:     "NASDAQ",
			ServiceDataType: "Deep",
			AggGroup:        1,
		}},
	}, {
		Kind: EventNewsProviders,
		NewsProviders: []NewsProviderValue{{
			Code: "BRFG",
			Name: "Briefing.com",
		}},
	}, {
		Kind: EventSoftDollarTiers,
		SoftDollarTiers: []SoftDollarTierValue{{
			Name:        "Tier",
			Value:       "VALUE",
			DisplayName: "Display",
		}},
	}, {
		Kind: EventMatchingSymbols,
		SymbolSamples: []SymbolSampleValue{{
			ConID:              265598,
			Symbol:             "AAPL",
			SecType:            "STK",
			PrimaryExchange:    "NASDAQ",
			Currency:           "USD",
			DerivativeSecTypes: []string{"OPT", "WAR"},
			Description:        "APPLE INC",
			IssuerID:           "issuer",
		}},
	}, {
		Kind:         EventMarketRule,
		MarketRuleID: 26,
		PriceIncrements: []PriceIncrementValue{{
			LowEdge:   "0",
			Increment: "0.01",
		}},
	}, {
		Kind: EventSecDefOptParams,
		SecDefOptParams: []SecDefOptParamsValue{{
			Exchange:        "SMART",
			UnderlyingConID: 265598,
			TradingClass:    "AAPL",
			Multiplier:      "100",
			Expirations:     []string{"20260619"},
			Strikes:         []string{"100"},
		}},
	}, {
		Kind: EventSmartComponents,
		SmartComponents: []SmartComponentValue{{
			BitNumber:      0,
			ExchangeName:   "ARCA",
			ExchangeLetter: "P",
		}},
	}}
	adapter := NewReplayAdapter(source)
	source[0].Accounts[0] = "mutated"
	source[1].FamilyCodes[0].FamilyCode = "mutated"
	source[2].DepthExchanges[0].Exchange = "mutated"
	source[3].NewsProviders[0].Name = "mutated"
	source[4].SoftDollarTiers[0].DisplayName = "mutated"
	source[5].SymbolSamples[0].DerivativeSecTypes[0] = "mutated"
	source[6].PriceIncrements[0].Increment = "mutated"
	source[7].SecDefOptParams[0].Expirations[0] = "mutated"
	source[7].SecDefOptParams[0].Strikes[0] = "mutated"
	source[8].SmartComponents[0].ExchangeName = "mutated"

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
	if got := events[1].FamilyCodes[0].FamilyCode; got != "FAMILY" {
		t.Fatalf("event family code = %q, want copied FAMILY", got)
	}
	events[1].FamilyCodes[0].FamilyCode = "changed"
	if got := events[2].DepthExchanges[0].Exchange; got != "ISLAND" {
		t.Fatalf("event depth exchange = %q, want copied ISLAND", got)
	}
	events[2].DepthExchanges[0].Exchange = "changed"
	if got := events[3].NewsProviders[0].Name; got != "Briefing.com" {
		t.Fatalf("event news provider = %q, want copied Briefing.com", got)
	}
	events[3].NewsProviders[0].Name = "changed"
	if got := events[4].SoftDollarTiers[0].DisplayName; got != "Display" {
		t.Fatalf("event soft dollar tier = %q, want copied Display", got)
	}
	events[4].SoftDollarTiers[0].DisplayName = "changed"
	if got := events[5].SymbolSamples[0].DerivativeSecTypes[0]; got != "OPT" {
		t.Fatalf("event derivative sec type = %q, want copied OPT", got)
	}
	events[5].SymbolSamples[0].DerivativeSecTypes[0] = "changed"
	if got := events[6].PriceIncrements[0].Increment; got != "0.01" {
		t.Fatalf("event price increment = %q, want copied 0.01", got)
	}
	events[6].PriceIncrements[0].Increment = "changed"
	if got := events[7].SecDefOptParams[0].Expirations[0]; got != "20260619" {
		t.Fatalf("event sec def opt params expiration = %q, want copied 20260619", got)
	}
	if got := events[7].SecDefOptParams[0].Strikes[0]; got != "100" {
		t.Fatalf("event sec def opt params strike = %q, want copied 100", got)
	}
	events[7].SecDefOptParams[0].Expirations[0] = "changed"
	events[7].SecDefOptParams[0].Strikes[0] = "changed"
	if got := events[8].SmartComponents[0].ExchangeName; got != "ARCA" {
		t.Fatalf("event smart component exchange = %q, want copied ARCA", got)
	}
	events[8].SmartComponents[0].ExchangeName = "changed"

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

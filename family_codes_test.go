package ibkr

import (
	"context"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
)

func TestFamilyCodesUsesSDKCommandAndEvent(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	if err := e.sendSDKContext(context.Background(), sdkadapter.FamilyCodesRequest{}); err != nil {
		t.Fatalf("sendSDKContext() error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 1 {
		t.Fatalf("commands len = %d, want 1", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandFamilyCodes {
		t.Fatalf("command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandFamilyCodes)
	}

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind: sdkadapter.EventFamilyCodes,
		FamilyCodes: []sdkadapter.FamilyCodeValue{{
			AccountID:  "DU12345",
			FamilyCode: "MAIN",
		}},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage() error = %v", err)
	}
	got, ok := msg.(sdkadapter.FamilyCodes)
	if !ok {
		t.Fatalf("sdkEventToMessage() type = %T, want sdkadapter.FamilyCodes", msg)
	}
	if len(got.Codes) != 1 {
		t.Fatalf("family codes len = %d, want 1", len(got.Codes))
	}
	if got.Codes[0].AccountID != "DU12345" || got.Codes[0].FamilyCode != "MAIN" {
		t.Fatalf("family code = %+v, want DU12345/MAIN", got.Codes[0])
	}
}

func TestSDKSingletonMetadataUsesSDKCommandsAndEvents(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		request any
		command sdkadapter.CommandKind
		event   sdkadapter.Event
		assert  func(*testing.T, any)
	}{
		{
			name:    "mkt depth exchanges",
			request: sdkadapter.MktDepthExchangesRequest{},
			command: sdkadapter.CommandMktDepthExchanges,
			event: sdkadapter.Event{
				Kind: sdkadapter.EventMktDepthExchanges,
				DepthExchanges: []sdkadapter.DepthExchangeValue{{
					Exchange:        "ISLAND",
					SecType:         "STK",
					ListingExch:     "NASDAQ",
					ServiceDataType: "Deep",
					AggGroup:        1,
				}},
			},
			assert: func(t *testing.T, msg any) {
				t.Helper()
				got, ok := msg.(sdkadapter.MktDepthExchanges)
				if !ok {
					t.Fatalf("sdkEventToMessage() type = %T, want sdkadapter.MktDepthExchanges", msg)
				}
				if len(got.Exchanges) != 1 {
					t.Fatalf("exchanges len = %d, want 1", len(got.Exchanges))
				}
				if got.Exchanges[0].Exchange != "ISLAND" || got.Exchanges[0].SecType != "STK" || got.Exchanges[0].AggGroup != 1 {
					t.Fatalf("exchange = %+v, want ISLAND/STK/1", got.Exchanges[0])
				}
			},
		},
		{
			name:    "news providers",
			request: sdkadapter.NewsProvidersRequest{},
			command: sdkadapter.CommandNewsProviders,
			event: sdkadapter.Event{
				Kind: sdkadapter.EventNewsProviders,
				NewsProviders: []sdkadapter.NewsProviderValue{{
					Code: "BRFG",
					Name: "Briefing.com",
				}},
			},
			assert: func(t *testing.T, msg any) {
				t.Helper()
				got, ok := msg.(sdkadapter.NewsProviders)
				if !ok {
					t.Fatalf("sdkEventToMessage() type = %T, want sdkadapter.NewsProviders", msg)
				}
				if len(got.Providers) != 1 {
					t.Fatalf("providers len = %d, want 1", len(got.Providers))
				}
				if got.Providers[0].Code != "BRFG" || got.Providers[0].Name != "Briefing.com" {
					t.Fatalf("provider = %+v, want BRFG/Briefing.com", got.Providers[0])
				}
			},
		},
		{
			name:    "scanner parameters",
			request: sdkadapter.ScannerParametersRequest{},
			command: sdkadapter.CommandScannerParameters,
			event: sdkadapter.Event{
				Kind:       sdkadapter.EventScannerParameters,
				ScannerXML: "<ScannerParameters/>",
			},
			assert: func(t *testing.T, msg any) {
				t.Helper()
				got, ok := msg.(sdkadapter.ScannerParameters)
				if !ok {
					t.Fatalf("sdkEventToMessage() type = %T, want sdkadapter.ScannerParameters", msg)
				}
				if got.XML != "<ScannerParameters/>" {
					t.Fatalf("scanner XML = %q, want fixture XML", got.XML)
				}
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			adapter := sdkadapter.NewReplayAdapter(nil)
			e := &engine{adapter: adapter}
			if err := e.sendSDKContext(context.Background(), tc.request); err != nil {
				t.Fatalf("sendSDKContext() error = %v", err)
			}

			commands := adapter.Commands()
			if len(commands) != 1 {
				t.Fatalf("commands len = %d, want 1", len(commands))
			}
			if commands[0].Kind != tc.command {
				t.Fatalf("command kind = %s, want %s", commands[0].Kind, tc.command)
			}

			msg, err := sdkEventToMessage(tc.event)
			if err != nil {
				t.Fatalf("sdkEventToMessage() error = %v", err)
			}
			tc.assert(t, msg)
		})
	}
}

func TestSDKKeyedMetadataUsesSDKCommandsAndEvents(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		request any
		command sdkadapter.CommandKind
		reqID   func(sdkadapter.Command) int
		event   sdkadapter.Event
		assert  func(*testing.T, any)
	}{
		{
			name:    "user info",
			request: sdkadapter.UserInfoRequest{ReqID: 11},
			command: sdkadapter.CommandUserInfo,
			reqID: func(command sdkadapter.Command) int {
				return command.UserInfo.ReqID
			},
			event: sdkadapter.Event{
				Kind:  sdkadapter.EventUserInfo,
				ReqID: 11,
				UserInfo: sdkadapter.UserInfoValue{
					WhiteBrandingID: "IBKR",
				},
			},
			assert: func(t *testing.T, msg any) {
				t.Helper()
				got, ok := msg.(sdkadapter.UserInfo)
				if !ok {
					t.Fatalf("sdkEventToMessage() type = %T, want sdkadapter.UserInfo", msg)
				}
				if got.ReqID != 11 || got.WhiteBrandingID != "IBKR" {
					t.Fatalf("user info = %+v, want reqID 11 and IBKR", got)
				}
			},
		},
		{
			name:    "soft dollar tiers",
			request: sdkadapter.SoftDollarTiersRequest{ReqID: 12},
			command: sdkadapter.CommandSoftDollarTiers,
			reqID: func(command sdkadapter.Command) int {
				return command.SoftDollarTiers.ReqID
			},
			event: sdkadapter.Event{
				Kind:  sdkadapter.EventSoftDollarTiers,
				ReqID: 12,
				SoftDollarTiers: []sdkadapter.SoftDollarTierValue{{
					Name:        "Tier",
					Value:       "VALUE",
					DisplayName: "Display",
				}},
			},
			assert: func(t *testing.T, msg any) {
				t.Helper()
				got, ok := msg.(sdkadapter.SoftDollarTiersResponse)
				if !ok {
					t.Fatalf("sdkEventToMessage() type = %T, want sdkadapter.SoftDollarTiersResponse", msg)
				}
				if got.ReqID != 12 || len(got.Tiers) != 1 {
					t.Fatalf("soft dollar tiers = %+v, want reqID 12 and one tier", got)
				}
				if got.Tiers[0].Name != "Tier" || got.Tiers[0].Value != "VALUE" || got.Tiers[0].DisplayName != "Display" {
					t.Fatalf("soft dollar tier = %+v, want Tier/VALUE/Display", got.Tiers[0])
				}
			},
		},
		{
			name:    "display groups",
			request: sdkadapter.QueryDisplayGroupsRequest{ReqID: 13},
			command: sdkadapter.CommandQueryDisplayGroups,
			reqID: func(command sdkadapter.Command) int {
				return command.QueryDisplayGroups.ReqID
			},
			event: sdkadapter.Event{
				Kind:          sdkadapter.EventDisplayGroupList,
				ReqID:         13,
				DisplayGroups: "1|2|3",
			},
			assert: func(t *testing.T, msg any) {
				t.Helper()
				got, ok := msg.(sdkadapter.DisplayGroupList)
				if !ok {
					t.Fatalf("sdkEventToMessage() type = %T, want sdkadapter.DisplayGroupList", msg)
				}
				if got.ReqID != 13 || got.Groups != "1|2|3" {
					t.Fatalf("display group list = %+v, want reqID 13 and groups", got)
				}
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			adapter := sdkadapter.NewReplayAdapter(nil)
			e := &engine{adapter: adapter}
			if err := e.sendSDKContext(context.Background(), tc.request); err != nil {
				t.Fatalf("sendSDKContext() error = %v", err)
			}

			commands := adapter.Commands()
			if len(commands) != 1 {
				t.Fatalf("commands len = %d, want 1", len(commands))
			}
			if commands[0].Kind != tc.command {
				t.Fatalf("command kind = %s, want %s", commands[0].Kind, tc.command)
			}
			if got := tc.reqID(commands[0]); got != tc.event.ReqID {
				t.Fatalf("command reqID = %d, want %d", got, tc.event.ReqID)
			}

			msg, err := sdkEventToMessage(tc.event)
			if err != nil {
				t.Fatalf("sdkEventToMessage() error = %v", err)
			}
			tc.assert(t, msg)
		})
	}
}

func TestSDKContractMetadataUsesSDKCommandsAndEvents(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	if err := e.sendSDKContext(context.Background(), sdkadapter.MatchingSymbolsRequest{ReqID: 21, Pattern: "AAPL"}); err != nil {
		t.Fatalf("sendSDKContext(MatchingSymbolsRequest) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.MarketRuleRequest{MarketRuleID: 26}); err != nil {
		t.Fatalf("sendSDKContext(MarketRuleRequest) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.SecDefOptParamsRequest{
		ReqID:             22,
		UnderlyingSymbol:  "AAPL",
		FutFopExchange:    "",
		UnderlyingSecType: "STK",
		UnderlyingConID:   265598,
	}); err != nil {
		t.Fatalf("sendSDKContext(SecDefOptParamsRequest) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.SmartComponentsRequest{ReqID: 23, BBOExchange: "9c0001"}); err != nil {
		t.Fatalf("sendSDKContext(SmartComponentsRequest) error = %v", err)
	}
	fundamentalContract := sdkadapter.Contract{
		Symbol:   "AAPL",
		SecType:  "STK",
		Exchange: "SMART",
		Currency: "USD",
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.FundamentalDataRequest{
		ReqID:      24,
		Contract:   fundamentalContract,
		ReportType: "ReportsFinSummary",
	}); err != nil {
		t.Fatalf("sendSDKContext(FundamentalDataRequest) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.CancelFundamentalData{ReqID: 24}); err != nil {
		t.Fatalf("sendSDKContext(CancelFundamentalData) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 6 {
		t.Fatalf("commands len = %d, want 6", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandMatchingSymbols {
		t.Fatalf("matching command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandMatchingSymbols)
	}
	if commands[0].MatchingSymbols.ReqID != 21 || commands[0].MatchingSymbols.Pattern != "AAPL" {
		t.Fatalf("matching command = %+v, want reqID 21 pattern AAPL", commands[0].MatchingSymbols)
	}
	if commands[1].Kind != sdkadapter.CommandMarketRule {
		t.Fatalf("market rule command kind = %s, want %s", commands[1].Kind, sdkadapter.CommandMarketRule)
	}
	if commands[1].MarketRule.MarketRuleID != 26 {
		t.Fatalf("market rule id = %d, want 26", commands[1].MarketRule.MarketRuleID)
	}
	if commands[2].Kind != sdkadapter.CommandSecDefOptParams {
		t.Fatalf("sec def opt params command kind = %s, want %s", commands[2].Kind, sdkadapter.CommandSecDefOptParams)
	}
	if commands[2].SecDefOptParams.ReqID != 22 || commands[2].SecDefOptParams.UnderlyingSymbol != "AAPL" || commands[2].SecDefOptParams.UnderlyingConID != 265598 {
		t.Fatalf("sec def opt params command = %+v, want reqID 22 AAPL conID 265598", commands[2].SecDefOptParams)
	}
	if commands[3].Kind != sdkadapter.CommandSmartComponents {
		t.Fatalf("smart components command kind = %s, want %s", commands[3].Kind, sdkadapter.CommandSmartComponents)
	}
	if commands[3].SmartComponents.ReqID != 23 || commands[3].SmartComponents.BBOExchange != "9c0001" {
		t.Fatalf("smart components command = %+v, want reqID 23 bbo 9c0001", commands[3].SmartComponents)
	}
	if commands[4].Kind != sdkadapter.CommandFundamentalData {
		t.Fatalf("fundamental data command kind = %s, want %s", commands[4].Kind, sdkadapter.CommandFundamentalData)
	}
	if commands[4].FundamentalData.ReqID != 24 || commands[4].FundamentalData.ReportType != "ReportsFinSummary" {
		t.Fatalf("fundamental data command = %+v, want reqID 24 ReportsFinSummary", commands[4].FundamentalData)
	}
	if commands[4].FundamentalData.Contract.Symbol != "AAPL" || commands[4].FundamentalData.Contract.Exchange != "SMART" {
		t.Fatalf("fundamental data contract = %+v, want AAPL SMART", commands[4].FundamentalData.Contract)
	}
	if commands[5].Kind != sdkadapter.CommandCancelFundamentalData {
		t.Fatalf("cancel fundamental data command kind = %s, want %s", commands[5].Kind, sdkadapter.CommandCancelFundamentalData)
	}
	if commands[5].CancelFundamentalData.ReqID != 24 {
		t.Fatalf("cancel fundamental data reqID = %d, want 24", commands[5].CancelFundamentalData.ReqID)
	}

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind:  sdkadapter.EventMatchingSymbols,
		ReqID: 21,
		SymbolSamples: []sdkadapter.SymbolSampleValue{{
			ConID:              265598,
			Symbol:             "AAPL",
			SecType:            "STK",
			PrimaryExchange:    "NASDAQ",
			Currency:           "USD",
			DerivativeSecTypes: []string{"OPT", "WAR"},
			Description:        "APPLE INC",
			IssuerID:           "issuer",
		}},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(matching) error = %v", err)
	}
	matching, ok := msg.(sdkadapter.MatchingSymbols)
	if !ok {
		t.Fatalf("sdkEventToMessage(matching) type = %T, want sdkadapter.MatchingSymbols", msg)
	}
	if matching.ReqID != 21 || len(matching.Symbols) != 1 {
		t.Fatalf("matching symbols = %+v, want reqID 21 and one symbol", matching)
	}
	if matching.Symbols[0].Symbol != "AAPL" || matching.Symbols[0].DerivativeSecTypes[0] != "OPT" {
		t.Fatalf("matching symbol = %+v, want AAPL with OPT derivative", matching.Symbols[0])
	}

	msg, err = sdkEventToMessage(sdkadapter.Event{
		Kind:         sdkadapter.EventMarketRule,
		MarketRuleID: 26,
		PriceIncrements: []sdkadapter.PriceIncrementValue{{
			LowEdge:   "0",
			Increment: "0.01",
		}},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(market rule) error = %v", err)
	}
	rule, ok := msg.(sdkadapter.MarketRule)
	if !ok {
		t.Fatalf("sdkEventToMessage(market rule) type = %T, want sdkadapter.MarketRule", msg)
	}
	if rule.MarketRuleID != 26 || len(rule.Increments) != 1 {
		t.Fatalf("market rule = %+v, want id 26 and one increment", rule)
	}
	if rule.Increments[0].LowEdge != "0" || rule.Increments[0].Increment != "0.01" {
		t.Fatalf("market rule increment = %+v, want 0/0.01", rule.Increments[0])
	}

	msg, err = sdkEventToMessage(sdkadapter.Event{
		Kind:  sdkadapter.EventSecDefOptParams,
		ReqID: 22,
		SecDefOptParams: []sdkadapter.SecDefOptParamsValue{{
			Exchange:        "SMART",
			UnderlyingConID: 265598,
			TradingClass:    "AAPL",
			Multiplier:      "100",
			Expirations:     []string{"20260619"},
			Strikes:         []string{"100", "105.5"},
		}},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(sec def opt params) error = %v", err)
	}
	params, ok := msg.(sdkadapter.SecDefOptParamsResponse)
	if !ok {
		t.Fatalf("sdkEventToMessage(sec def opt params) type = %T, want sdkadapter.SecDefOptParamsResponse", msg)
	}
	if params.ReqID != 22 || params.UnderlyingConID != 265598 || params.Exchange != "SMART" {
		t.Fatalf("sec def opt params = %+v, want reqID 22 conID 265598 SMART", params)
	}
	if len(params.Expirations) != 1 || params.Expirations[0] != "20260619" || len(params.Strikes) != 2 || params.Strikes[1] != "105.5" {
		t.Fatalf("sec def opt params expirations/strikes = %+v/%+v, want copied values", params.Expirations, params.Strikes)
	}

	msg, err = sdkEventToMessage(sdkadapter.Event{Kind: sdkadapter.EventSecDefOptParamsEnd, ReqID: 22})
	if err != nil {
		t.Fatalf("sdkEventToMessage(sec def opt params end) error = %v", err)
	}
	paramsEnd, ok := msg.(sdkadapter.SecDefOptParamsEnd)
	if !ok {
		t.Fatalf("sdkEventToMessage(sec def opt params end) type = %T, want sdkadapter.SecDefOptParamsEnd", msg)
	}
	if paramsEnd.ReqID != 22 {
		t.Fatalf("sec def opt params end reqID = %d, want 22", paramsEnd.ReqID)
	}

	msg, err = sdkEventToMessage(sdkadapter.Event{
		Kind:  sdkadapter.EventSmartComponents,
		ReqID: 23,
		SmartComponents: []sdkadapter.SmartComponentValue{{
			BitNumber:      0,
			ExchangeName:   "ARCA",
			ExchangeLetter: "P",
		}},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(smart components) error = %v", err)
	}
	components, ok := msg.(sdkadapter.SmartComponentsResponse)
	if !ok {
		t.Fatalf("sdkEventToMessage(smart components) type = %T, want sdkadapter.SmartComponentsResponse", msg)
	}
	if components.ReqID != 23 || len(components.Components) != 1 {
		t.Fatalf("smart components = %+v, want reqID 23 and one component", components)
	}
	if components.Components[0].BitNumber != 0 || components.Components[0].ExchangeName != "ARCA" || components.Components[0].ExchangeLetter != "P" {
		t.Fatalf("smart component = %+v, want 0/ARCA/P", components.Components[0])
	}

	msg, err = sdkEventToMessage(sdkadapter.Event{
		Kind:            sdkadapter.EventFundamentalData,
		ReqID:           24,
		FundamentalData: "<ReportSnapshot/>",
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(fundamental data) error = %v", err)
	}
	fundamental, ok := msg.(sdkadapter.FundamentalDataResponse)
	if !ok {
		t.Fatalf("sdkEventToMessage(fundamental data) type = %T, want sdkadapter.FundamentalDataResponse", msg)
	}
	if fundamental.ReqID != 24 || fundamental.Data != "<ReportSnapshot/>" {
		t.Fatalf("fundamental data = %+v, want reqID 24 XML", fundamental)
	}
}

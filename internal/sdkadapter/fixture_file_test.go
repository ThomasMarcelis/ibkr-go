package sdkadapter

import (
	"context"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestCommittedFixturesDecodeAndReplay(t *testing.T) {
	paths, err := filepath.Glob("testdata/fixtures/*.json")
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no committed SDK-event fixtures found")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			f, err := os.Open(path)
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			defer f.Close()

			fixture, err := DecodeFixture(f)
			if err != nil {
				t.Fatalf("DecodeFixture() error = %v", err)
			}
			if _, err := hex.DecodeString(fixture.Metadata.SourceSHA256); err != nil {
				t.Fatalf("SourceSHA256 = %q, want hex: %v", fixture.Metadata.SourceSHA256, err)
			}

			adapter, err := NewReplayAdapterFromFixture(fixture)
			if err != nil {
				t.Fatalf("NewReplayAdapterFromFixture() error = %v", err)
			}
			if err := adapter.Connect(context.Background(), ConnectRequest{}); err != nil {
				t.Fatalf("Connect() error = %v", err)
			}
			events, err := adapter.DrainEvents(context.Background(), len(fixture.Events)+1)
			if err != nil {
				t.Fatalf("DrainEvents() error = %v", err)
			}
			if len(events) != len(fixture.Events) {
				t.Fatalf("DrainEvents() len = %d, want %d", len(events), len(fixture.Events))
			}
		})
	}
}

func TestCommittedFixturesDoNotContainUnredactedAccountOrOrderIDs(t *testing.T) {
	paths, err := filepath.Glob("testdata/fixtures/*.json")
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no committed SDK-event fixtures found")
	}

	privatePattern := regexp.MustCompile(`DUP[0-9A-Z]+|DU[0-9]{3,}|DA[0-9]{3,}|U[0-9]{6,}|PermID": "[1-9]`)
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}
			if match := privatePattern.Find(raw); match != nil {
				t.Fatalf("fixture contains unredacted account/order identifier match %q", match)
			}
		})
	}
}

func TestReadOnlySmokeFixtureContainsPublicReferenceEvidence(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_read_only_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "read_only_smoke" {
		t.Fatalf("Scenario = %q, want read_only_smoke", fixture.Metadata.Scenario)
	}
	if fixture.Metadata.ServerVersion != 203 {
		t.Fatalf("ServerVersion = %d, want 203", fixture.Metadata.ServerVersion)
	}

	var sawRedactedManagedAccounts bool
	var sawCurrentTime bool
	var sawAAPLContractDetails bool
	var sawHeadTimestamp bool
	var histogramRows int
	var sawQuoteBBO bool
	var sawQuoteDelayedWarning bool
	var sawQuoteSnapshotEnd bool
	var matchingSymbolRows int
	var sawAAPLMatchingSymbol bool
	var sawMarketRule26 bool
	var secDefRows int
	var sawAAPLSecDef bool
	var sawSecDefEnd bool
	var sawSmartComponentsResponse bool
	var depthExchangeRows int
	var sawIslandDepth bool
	var newsProviderRows int
	var sawDowJonesProvider bool
	var sawSoftDollarTiers bool
	var sawUserInfo bool
	var sawDisplayGroups bool
	var sawWSHMetaDataEntitlement bool
	var sawWSHEventDataEntitlement bool

	for _, event := range fixture.Events {
		switch event.Kind {
		case EventManagedAccounts:
			if len(event.Accounts) == 1 && event.Accounts[0] == "DU_REDACTED" {
				sawRedactedManagedAccounts = true
			}
		case EventCurrentTime:
			if event.CurrentTime > 0 {
				sawCurrentTime = true
			}
		case EventContractDetails:
			if event.ReqID == 101 &&
				event.ContractDetails.Contract.ConID == 265598 &&
				event.ContractDetails.Contract.Symbol == "AAPL" &&
				event.ContractDetails.Contract.SecType == "STK" &&
				event.ContractDetails.Contract.PrimaryExchange == "NASDAQ" &&
				event.ContractDetails.LongName == "APPLE INC" &&
				event.ContractDetails.TimeZoneID == "US/Eastern" {
				sawAAPLContractDetails = true
			}
		case EventHeadTimestamp:
			if event.ReqID == 102 && event.HeadTimestamp == "19801212-14:30:00" {
				sawHeadTimestamp = true
			}
		case EventHistogramData:
			if event.ReqID == 108 {
				histogramRows = len(event.HistogramData)
			}
		case EventTickReqParams:
			if event.ReqID == 103 &&
				event.TickReqParams.BBOExchange == "9c0001" &&
				event.TickReqParams.MinTick == "0.01" {
				sawQuoteBBO = true
			}
		case EventTickSnapshotEnd:
			if event.ReqID == 103 {
				sawQuoteSnapshotEnd = true
			}
		case EventAPIError:
			switch event.ReqID {
			case 103:
				if event.APIError.Code == 10167 &&
					strings.Contains(event.APIError.Message, "Displaying delayed market data") {
					sawQuoteDelayedWarning = true
				}
			case 107:
				if event.APIError.Code == 10276 &&
					strings.Contains(event.APIError.Message, "News feed is not allowed") {
					sawWSHMetaDataEntitlement = true
				}
			case 112:
				if event.APIError.Code == 10276 &&
					strings.Contains(event.APIError.Message, "News feed is not allowed") {
					sawWSHEventDataEntitlement = true
				}
			}
		case EventMatchingSymbols:
			if event.ReqID != 104 {
				continue
			}
			matchingSymbolRows = len(event.SymbolSamples)
			for _, sample := range event.SymbolSamples {
				if sample.ConID == 265598 &&
					sample.Symbol == "AAPL" &&
					sample.SecType == "STK" &&
					sample.PrimaryExchange == "NASDAQ" &&
					sample.Currency == "USD" &&
					len(sample.DerivativeSecTypes) > 0 {
					sawAAPLMatchingSymbol = true
				}
			}
		case EventMarketRule:
			if event.MarketRuleID == 26 &&
				len(event.PriceIncrements) == 1 &&
				event.PriceIncrements[0].LowEdge == "0" &&
				event.PriceIncrements[0].Increment == "0.01" {
				sawMarketRule26 = true
			}
		case EventSecDefOptParams:
			if event.ReqID != 105 {
				continue
			}
			secDefRows += len(event.SecDefOptParams)
			for _, params := range event.SecDefOptParams {
				if params.UnderlyingConID == 265598 &&
					params.TradingClass == "AAPL" &&
					params.Multiplier == "100" &&
					len(params.Expirations) > 0 &&
					len(params.Strikes) > 0 {
					sawAAPLSecDef = true
				}
			}
		case EventSecDefOptParamsEnd:
			if event.ReqID == 105 {
				sawSecDefEnd = true
			}
		case EventSmartComponents:
			if event.ReqID == 106 {
				sawSmartComponentsResponse = true
			}
		case EventMktDepthExchanges:
			depthExchangeRows = len(event.DepthExchanges)
			for _, exchange := range event.DepthExchanges {
				if exchange.Exchange == "ISLAND" &&
					exchange.SecType == "STK" &&
					exchange.ServiceDataType == "Deep" {
					sawIslandDepth = true
				}
			}
		case EventNewsProviders:
			newsProviderRows = len(event.NewsProviders)
			for _, provider := range event.NewsProviders {
				if provider.Code == "DJ-RT" && provider.Name == "Dow Jones Trader News" {
					sawDowJonesProvider = true
				}
			}
		case EventSoftDollarTiers:
			if event.ReqID == 109 && len(event.SoftDollarTiers) == 0 {
				sawSoftDollarTiers = true
			}
		case EventUserInfo:
			if event.ReqID == 110 && event.UserInfo.WhiteBrandingID == "" {
				sawUserInfo = true
			}
		case EventDisplayGroupList:
			if event.ReqID == 111 && event.DisplayGroups == "1|2|3|4|5|6|7" {
				sawDisplayGroups = true
			}
		}
	}

	if !sawRedactedManagedAccounts {
		t.Fatal("read-only fixture has no redacted managed_accounts evidence")
	}
	if !sawCurrentTime {
		t.Fatal("read-only fixture has no current_time evidence")
	}
	if !sawAAPLContractDetails {
		t.Fatal("read-only fixture has no AAPL contract_details evidence")
	}
	if !sawHeadTimestamp {
		t.Fatal("read-only fixture has no captured AAPL head_timestamp")
	}
	if histogramRows < 1000 {
		t.Fatalf("histogram rows = %d, want at least 1000", histogramRows)
	}
	if !sawQuoteBBO || !sawQuoteDelayedWarning || !sawQuoteSnapshotEnd {
		t.Fatalf("quote snapshot evidence BBO=%v delayedWarning=%v snapshotEnd=%v, want all true", sawQuoteBBO, sawQuoteDelayedWarning, sawQuoteSnapshotEnd)
	}
	if matchingSymbolRows < 10 || !sawAAPLMatchingSymbol {
		t.Fatalf("matching symbols rows=%d sawAAPL=%v, want AAPL row among captured symbol samples", matchingSymbolRows, sawAAPLMatchingSymbol)
	}
	if !sawMarketRule26 {
		t.Fatal("read-only fixture has no captured market_rule 26 increment evidence")
	}
	if secDefRows != 39 || !sawAAPLSecDef || !sawSecDefEnd {
		t.Fatalf("sec-def evidence rows=%d sawAAPL=%v sawEnd=%v, want 39 rows, AAPL option params, and end", secDefRows, sawAAPLSecDef, sawSecDefEnd)
	}
	if !sawSmartComponentsResponse {
		t.Fatal("read-only fixture has no smart_components response evidence")
	}
	if depthExchangeRows < 300 || !sawIslandDepth {
		t.Fatalf("depth exchanges rows=%d sawISLAND=%v, want public market-depth exchange catalog evidence", depthExchangeRows, sawIslandDepth)
	}
	if newsProviderRows != 8 || !sawDowJonesProvider {
		t.Fatalf("news providers rows=%d sawDJRT=%v, want captured provider catalog", newsProviderRows, sawDowJonesProvider)
	}
	if !sawSoftDollarTiers {
		t.Fatal("read-only fixture has no empty soft_dollar_tiers response evidence")
	}
	if !sawUserInfo {
		t.Fatal("read-only fixture has no Gateway user_info response evidence")
	}
	if !sawDisplayGroups {
		t.Fatal("read-only fixture has no Gateway display_group_list evidence")
	}
	if !sawWSHMetaDataEntitlement || !sawWSHEventDataEntitlement {
		t.Fatalf("WSH entitlement evidence metadata=%v eventData=%v, want both code 10276 errors", sawWSHMetaDataEntitlement, sawWSHEventDataEntitlement)
	}
}

func TestCurrentTimeMillisFixtureContainsFreshSessionTimestamp(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_current_time_millis_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "current_time_millis" {
		t.Fatalf("Scenario = %q, want current_time_millis", fixture.Metadata.Scenario)
	}
	if fixture.Metadata.ServerVersion != 203 {
		t.Fatalf("ServerVersion = %d, want 203", fixture.Metadata.ServerVersion)
	}

	var sawConnectionMetadata bool
	var sawRedactedManagedAccounts bool
	var sawNextValidID bool
	var millis int64
	for _, event := range fixture.Events {
		switch event.Kind {
		case EventConnectionMetadata:
			if event.ServerVersion == 203 && event.ConnectionTime == "20260502 17:39:52 Central European Standard Time" {
				sawConnectionMetadata = true
			}
		case EventManagedAccounts:
			if len(event.Accounts) == 1 && event.Accounts[0] == "DU_REDACTED" {
				sawRedactedManagedAccounts = true
			}
		case EventNextValidID:
			if event.NextValidID == 1 {
				sawNextValidID = true
			}
		case EventCurrentTimeMillis:
			millis = event.CurrentTime
		}
	}
	if !sawConnectionMetadata {
		t.Fatal("current-time-millis fixture has no captured connection metadata")
	}
	if !sawRedactedManagedAccounts {
		t.Fatal("current-time-millis fixture has no redacted managed_accounts evidence")
	}
	if !sawNextValidID {
		t.Fatal("current-time-millis fixture has no next_valid_id evidence")
	}
	if millis != 1777736392854 {
		t.Fatalf("current time millis = %d, want captured value 1777736392854", millis)
	}
	got := time.UnixMilli(millis).UTC().Format(time.RFC3339Nano)
	if got != "2026-05-02T15:39:52.854Z" {
		t.Fatalf("current time millis UTC = %s, want captured timestamp", got)
	}
}

func TestBondContractDetailsFixtureContainsDistinctBondCallback(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_bond_contract_details_snapshot_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "bond_contract_details_snapshot" {
		t.Fatalf("Scenario = %q, want bond_contract_details_snapshot", fixture.Metadata.Scenario)
	}
	if fixture.Metadata.ServerVersion != 203 {
		t.Fatalf("ServerVersion = %d, want 203", fixture.Metadata.ServerVersion)
	}
	if fixture.Metadata.SourceSHA256 != "4c01eb8b0be1532ae2b678f62ac731838f57c89c48b83a9c5cfbb0141932171d" {
		t.Fatalf("SourceSHA256 = %q, want captured source hash", fixture.Metadata.SourceSHA256)
	}

	var sawBondDetails bool
	var sawEnd bool
	var sawBondSizeWarning bool
	for _, event := range fixture.Events {
		switch event.Kind {
		case EventBondContractDetails:
			if event.ReqID == 1601 &&
				event.ContractDetails.Contract.ConID == 681308048 &&
				event.ContractDetails.Contract.SecType == "BOND" &&
				event.ContractDetails.Contract.Exchange == "SMART" &&
				event.ContractDetails.Contract.TradingClass == "IBM" &&
				event.ContractDetails.MinTick == "0.001" {
				sawBondDetails = true
			}
		case EventContractDetails:
			t.Fatalf("fixture used generic contract_details for bond callback: %+v", event)
		case EventContractDetailsEnd:
			if event.ReqID == 1601 {
				sawEnd = true
			}
		case EventAPIError:
			if event.APIError.Code == 2113 && strings.Contains(event.APIError.Message, "order size for Bonds") {
				sawBondSizeWarning = true
			}
		}
	}
	if !sawBondDetails {
		t.Fatal("bond fixture has no distinct bond_contract_details evidence")
	}
	if !sawEnd {
		t.Fatal("bond fixture has no contract_details_end marker")
	}
	if !sawBondSizeWarning {
		t.Fatal("bond fixture has no captured official bond-size warning")
	}
}

func TestAccountSummaryFixtureContainsRedactedValues(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_account_summary_snapshot_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "account_summary_snapshot" {
		t.Fatalf("Scenario = %q, want account_summary_snapshot", fixture.Metadata.Scenario)
	}
	if fixture.Metadata.ServerVersion != 203 {
		t.Fatalf("ServerVersion = %d, want 203", fixture.Metadata.ServerVersion)
	}
	if fixture.Metadata.SourceSHA256 != "64adc4661bdaeba4b6c40b3808159194a32cd302b65978a17d0a622cef2b7364" {
		t.Fatalf("SourceSHA256 = %q, want captured source hash", fixture.Metadata.SourceSHA256)
	}

	tags := make(map[string]int)
	var sawEnd bool
	var sawReqError bool
	for _, event := range fixture.Events {
		switch event.Kind {
		case EventAccountSummary:
			if event.ReqID != 121 {
				continue
			}
			if event.AccountSummary.Account != "DU_REDACTED" {
				t.Fatalf("AccountSummary.Account = %q, want redacted account", event.AccountSummary.Account)
			}
			if event.AccountSummary.Value != "REDACTED_VALUE" {
				t.Fatalf("AccountSummary.Value = %q, want redacted value", event.AccountSummary.Value)
			}
			if event.AccountSummary.Currency != "EUR" {
				t.Fatalf("AccountSummary.Currency = %q, want captured account currency", event.AccountSummary.Currency)
			}
			tags[event.AccountSummary.Tag]++
		case EventAccountSummaryEnd:
			if event.ReqID == 121 {
				sawEnd = true
			}
		case EventAPIError:
			if event.ReqID == 121 {
				sawReqError = true
			}
		}
	}
	if tags["NetLiquidation"] == 0 {
		t.Fatal("account summary fixture has no NetLiquidation callback")
	}
	if tags["BuyingPower"] == 0 {
		t.Fatal("account summary fixture has no BuyingPower callback")
	}
	if !sawEnd {
		t.Fatal("account summary fixture has no accountSummaryEnd callback")
	}
	if sawReqError {
		t.Fatal("account summary fixture unexpectedly has a request-scoped API error")
	}
}

func TestAccountStreamsFixtureContainsRedactedPrivateValues(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_account_streams_snapshot_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "account_streams_snapshot" {
		t.Fatalf("Scenario = %q, want account_streams_snapshot", fixture.Metadata.Scenario)
	}
	if fixture.Metadata.ServerVersion != 203 {
		t.Fatalf("ServerVersion = %d, want 203", fixture.Metadata.ServerVersion)
	}
	if fixture.Metadata.SourceSHA256 != "287bbbdbe96422f18b116f67bc1fcc09b34d017e1009784539f21aad420bd923" {
		t.Fatalf("SourceSHA256 = %q, want captured source hash", fixture.Metadata.SourceSHA256)
	}

	var accountValues int
	var portfolioRows int
	var accountUpdateMultiRows int
	var positionRows int
	var positionMultiRows int
	var sawAccountDownloadEnd bool
	var sawAccountUpdateMultiEnd bool
	var sawPositionEnd bool
	var sawPositionMultiEnd bool
	var sawPnL bool
	var sawPnLSingle bool
	var sawReqError bool
	for _, event := range fixture.Events {
		switch event.Kind {
		case EventUpdateAccountValue:
			accountValues++
			if event.AccountValue.Account != "DU_REDACTED" {
				t.Fatalf("AccountValue.Account = %q, want redacted account", event.AccountValue.Account)
			}
			if event.AccountValue.Value != "REDACTED_VALUE" {
				t.Fatalf("AccountValue.Value = %q, want redacted value", event.AccountValue.Value)
			}
			if event.AccountValue.Key == "" {
				t.Fatal("account stream fixture has update_account_value with empty key")
			}
		case EventUpdatePortfolio:
			portfolioRows++
			if event.Portfolio.Account != "DU_REDACTED" {
				t.Fatalf("Portfolio.Account = %q, want redacted account", event.Portfolio.Account)
			}
			assertPrivateContractRedacted(t, event.Portfolio.Contract)
			assertRedactedValue(t, "Portfolio.Position", event.Portfolio.Position)
			assertRedactedValue(t, "Portfolio.MarketPrice", event.Portfolio.MarketPrice)
			assertRedactedValue(t, "Portfolio.MarketValue", event.Portfolio.MarketValue)
			assertRedactedValue(t, "Portfolio.AvgCost", event.Portfolio.AvgCost)
			assertRedactedValue(t, "Portfolio.UnrealizedPNL", event.Portfolio.UnrealizedPNL)
			assertRedactedValue(t, "Portfolio.RealizedPNL", event.Portfolio.RealizedPNL)
		case EventAccountDownloadEnd:
			if event.AccountDownloadEnd == "DU_REDACTED" {
				sawAccountDownloadEnd = true
			}
		case EventAccountUpdateMulti:
			if event.ReqID != 811 {
				continue
			}
			accountUpdateMultiRows++
			if event.AccountUpdateMulti.Account != "DU_REDACTED" {
				t.Fatalf("AccountUpdateMulti.Account = %q, want redacted account", event.AccountUpdateMulti.Account)
			}
			assertRedactedModelCode(t, event.AccountUpdateMulti.ModelCode)
			assertRedactedValue(t, "AccountUpdateMulti.Value", event.AccountUpdateMulti.Value)
		case EventAccountUpdateMultiEnd:
			if event.ReqID == 811 {
				sawAccountUpdateMultiEnd = true
			}
		case EventPosition:
			positionRows++
			if event.Position.Account != "DU_REDACTED" {
				t.Fatalf("Position.Account = %q, want redacted account", event.Position.Account)
			}
			assertPrivateContractRedacted(t, event.Position.Contract)
			assertRedactedValue(t, "Position.Position", event.Position.Position)
			assertRedactedValue(t, "Position.AvgCost", event.Position.AvgCost)
		case EventPositionEnd:
			sawPositionEnd = true
		case EventPositionMulti:
			if event.ReqID != 812 {
				continue
			}
			positionMultiRows++
			if event.PositionMulti.Account != "DU_REDACTED" {
				t.Fatalf("PositionMulti.Account = %q, want redacted account", event.PositionMulti.Account)
			}
			assertRedactedModelCode(t, event.PositionMulti.ModelCode)
			assertPrivateContractRedacted(t, event.PositionMulti.Contract)
			assertRedactedValue(t, "PositionMulti.Position", event.PositionMulti.Position)
			assertRedactedValue(t, "PositionMulti.AvgCost", event.PositionMulti.AvgCost)
		case EventPositionMultiEnd:
			if event.ReqID == 812 {
				sawPositionMultiEnd = true
			}
		case EventPnL:
			if event.ReqID != 813 {
				continue
			}
			sawPnL = true
			assertRedactedValue(t, "PnL.DailyPnL", event.PnL.DailyPnL)
			assertRedactedValue(t, "PnL.UnrealizedPnL", event.PnL.UnrealizedPnL)
			assertRedactedValue(t, "PnL.RealizedPnL", event.PnL.RealizedPnL)
		case EventPnLSingle:
			if event.ReqID != 814 {
				continue
			}
			sawPnLSingle = true
			assertRedactedValue(t, "PnLSingle.Position", event.PnLSingle.Position)
			assertRedactedValue(t, "PnLSingle.DailyPnL", event.PnLSingle.DailyPnL)
			assertRedactedValue(t, "PnLSingle.UnrealizedPnL", event.PnLSingle.UnrealizedPnL)
			assertRedactedValue(t, "PnLSingle.RealizedPnL", event.PnLSingle.RealizedPnL)
			assertRedactedValue(t, "PnLSingle.Value", event.PnLSingle.Value)
		case EventAPIError:
			if event.ReqID >= 811 && event.ReqID <= 814 {
				sawReqError = true
			}
		}
	}
	if accountValues < 100 || portfolioRows == 0 || accountUpdateMultiRows < 50 {
		t.Fatalf("account stream rows accountValues=%d portfolio=%d multi=%d, want captured snapshot shape", accountValues, portfolioRows, accountUpdateMultiRows)
	}
	if positionRows == 0 || positionMultiRows == 0 {
		t.Fatalf("position stream rows positions=%d positionsMulti=%d, want captured private position shapes", positionRows, positionMultiRows)
	}
	if !sawAccountDownloadEnd || !sawAccountUpdateMultiEnd || !sawPositionEnd || !sawPositionMultiEnd || !sawPnL || !sawPnLSingle {
		t.Fatalf("account stream completions accountEnd=%v multiEnd=%v positionEnd=%v positionMultiEnd=%v pnl=%v pnlSingle=%v, want all true", sawAccountDownloadEnd, sawAccountUpdateMultiEnd, sawPositionEnd, sawPositionMultiEnd, sawPnL, sawPnLSingle)
	}
	if sawReqError {
		t.Fatal("account streams fixture unexpectedly has a request-scoped API error")
	}
}

func assertPrivateContractRedacted(t *testing.T, contract Contract) {
	t.Helper()
	if contract.ConID != 0 {
		t.Fatalf("private contract ConID = %d, want redacted zero", contract.ConID)
	}
	if contract.Symbol != "REDACTED_CONTRACT" ||
		contract.SecType != "REDACTED_CONTRACT" ||
		contract.Exchange != "REDACTED_CONTRACT" ||
		contract.Currency != "REDACTED_CONTRACT" {
		t.Fatalf("private contract = %+v, want redacted placeholders", contract)
	}
}

func assertRedactedModelCode(t *testing.T, value string) {
	t.Helper()
	if value != "" && value != "REDACTED_MODEL" {
		t.Fatalf("model code = %q, want redacted placeholder", value)
	}
}

func assertRedactedValue(t *testing.T, label string, value string) {
	t.Helper()
	if value != "" && value != "REDACTED_VALUE" {
		t.Fatalf("%s = %q, want redacted value", label, value)
	}
}

func TestFamilyCodesFixtureContainsRedactedAccount(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_family_codes_snapshot_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "family_codes_snapshot" {
		t.Fatalf("Scenario = %q, want family_codes_snapshot", fixture.Metadata.Scenario)
	}
	if fixture.Metadata.ServerVersion != 203 {
		t.Fatalf("ServerVersion = %d, want 203", fixture.Metadata.ServerVersion)
	}
	if fixture.Metadata.SourceSHA256 != "af54ff7bd7a7cf603b9ca8774b9f42f99ca242ce3f0fa34db66f5177df44e3d1" {
		t.Fatalf("SourceSHA256 = %q, want captured source hash", fixture.Metadata.SourceSHA256)
	}

	var sawFamilyCodes bool
	for _, event := range fixture.Events {
		if event.Kind != EventFamilyCodes {
			continue
		}
		sawFamilyCodes = true
		if len(event.FamilyCodes) != 1 {
			t.Fatalf("FamilyCodes len = %d, want captured single-account family code", len(event.FamilyCodes))
		}
		if event.FamilyCodes[0].AccountID != "DU_REDACTED" {
			t.Fatalf("FamilyCodes[0].AccountID = %q, want redacted account", event.FamilyCodes[0].AccountID)
		}
		if event.FamilyCodes[0].FamilyCode != "" {
			t.Fatalf("FamilyCodes[0].FamilyCode = %q, want captured empty family code", event.FamilyCodes[0].FamilyCode)
		}
	}
	if !sawFamilyCodes {
		t.Fatal("family-codes fixture has no family_codes callback")
	}
}

func TestPaperOrderFixtureContainsPlaceCancelEvidence(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_paper_order_place_cancel_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "paper_order_place_cancel" {
		t.Fatalf("Scenario = %q, want paper_order_place_cancel", fixture.Metadata.Scenario)
	}

	var orderID int64
	var sawOpenOrder bool
	var sawWarning399 bool
	var sawCancelled bool
	for _, event := range fixture.Events {
		switch event.Kind {
		case EventOpenOrder:
			if event.OpenOrder.Action == "BUY" &&
				event.OpenOrder.OrderType == "LMT" &&
				event.OpenOrder.Account == "DU_REDACTED" &&
				event.OpenOrder.OrderID > 0 {
				orderID = event.OpenOrder.OrderID
				sawOpenOrder = true
			}
		case EventAPIError:
			if event.APIError.Code == 399 && orderID != 0 && int64(event.ReqID) == orderID {
				sawWarning399 = true
			}
		case EventOrderStatus:
			if event.OrderStatus.Status == "Cancelled" && orderID != 0 && event.OrderStatus.OrderID == orderID {
				sawCancelled = true
			}
		}
	}
	if !sawOpenOrder {
		t.Fatal("paper order fixture has no redacted BUY LMT open_order evidence")
	}
	if !sawWarning399 {
		t.Fatal("paper order fixture has no order-scoped warning code 399 evidence")
	}
	if !sawCancelled {
		t.Fatal("paper order fixture has no final Cancelled order_status evidence")
	}
}

func TestPaperOrderModifyFixtureContainsModifyCancelEvidence(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_paper_order_modify_cancel_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "paper_order_modify_cancel" {
		t.Fatalf("Scenario = %q, want paper_order_modify_cancel", fixture.Metadata.Scenario)
	}

	var orderID int64
	var sawInitialOpenOrder bool
	var sawModifiedOpenOrder bool
	var sawCancelled bool
	for _, event := range fixture.Events {
		switch event.Kind {
		case EventOpenOrder:
			if event.OpenOrder.Account != "DU_REDACTED" ||
				event.OpenOrder.Action != "BUY" ||
				event.OpenOrder.OrderType != "LMT" ||
				event.OpenOrder.OrderID <= 0 {
				continue
			}
			if orderID == 0 {
				orderID = event.OpenOrder.OrderID
			}
			if event.OpenOrder.OrderID != orderID {
				continue
			}
			quantity, err := decimal.NewFromString(event.OpenOrder.Quantity)
			if err != nil {
				t.Fatalf("OpenOrder.Quantity = %q: %v", event.OpenOrder.Quantity, err)
			}
			if quantity.Equal(decimal.NewFromInt(1)) {
				sawInitialOpenOrder = true
			}
			if quantity.Equal(decimal.NewFromInt(2)) {
				sawModifiedOpenOrder = true
			}
		case EventOrderStatus:
			if event.OrderStatus.Status == "Cancelled" && orderID != 0 && event.OrderStatus.OrderID == orderID {
				sawCancelled = true
			}
		}
	}
	if !sawInitialOpenOrder {
		t.Fatal("paper modify fixture has no initial quantity=1 open_order evidence")
	}
	if !sawModifiedOpenOrder {
		t.Fatal("paper modify fixture has no modified quantity=2 open_order evidence")
	}
	if !sawCancelled {
		t.Fatal("paper modify fixture has no final Cancelled order_status evidence")
	}
}

func TestPaperOpenOrdersFixtureContainsSnapshotEvidence(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_paper_open_orders_place_cancel_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "paper_open_orders_place_cancel" {
		t.Fatalf("Scenario = %q, want paper_open_orders_place_cancel", fixture.Metadata.Scenario)
	}

	var orderID int64
	var openOrderCount int
	var sawOpenOrderEndAfterSnapshot bool
	var sawCancelled bool
	for _, event := range fixture.Events {
		switch event.Kind {
		case EventOpenOrder:
			if event.OpenOrder.Account != "DU_REDACTED" ||
				event.OpenOrder.Action != "BUY" ||
				event.OpenOrder.OrderType != "LMT" ||
				!strings.HasPrefix(event.OpenOrder.OrderRef, "ibkr-go-sdk-fixture-open-") ||
				event.OpenOrder.OrderID <= 0 {
				continue
			}
			if orderID == 0 {
				orderID = event.OpenOrder.OrderID
			}
			if event.OpenOrder.OrderID == orderID {
				openOrderCount++
			}
		case EventOpenOrderEnd:
			if openOrderCount >= 2 {
				sawOpenOrderEndAfterSnapshot = true
			}
		case EventOrderStatus:
			if event.OrderStatus.Status == "Cancelled" && orderID != 0 && event.OrderStatus.OrderID == orderID {
				sawCancelled = true
			}
		}
	}
	if openOrderCount < 2 {
		t.Fatalf("paper open-orders fixture has %d scenario open_order callbacks, want placement and snapshot echoes", openOrderCount)
	}
	if !sawOpenOrderEndAfterSnapshot {
		t.Fatal("paper open-orders fixture has no open_order_end after snapshot open_order evidence")
	}
	if !sawCancelled {
		t.Fatal("paper open-orders fixture has no final Cancelled order_status evidence")
	}
}

func TestPaperOrderRejectFixtureContainsInvalidTypeEvidence(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_paper_order_reject_invalid_type_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "paper_order_reject_invalid_type" {
		t.Fatalf("Scenario = %q, want paper_order_reject_invalid_type", fixture.Metadata.Scenario)
	}

	var orderID int64
	var sawInvalidTypeError bool
	var sawUnexpectedOpenOrder bool
	for _, event := range fixture.Events {
		if event.Kind == EventNextValidID && event.NextValidID > 0 {
			orderID = event.NextValidID
		}
		if orderID == 0 {
			continue
		}
		switch event.Kind {
		case EventAPIError:
			if int64(event.ReqID) == orderID &&
				event.APIError.Code == 321 &&
				strings.Contains(event.APIError.Message, "Invalid order type") {
				sawInvalidTypeError = true
			}
		case EventOpenOrder:
			if event.OpenOrder.OrderID == orderID {
				sawUnexpectedOpenOrder = true
			}
		}
	}
	if orderID == 0 {
		t.Fatal("paper invalid-order fixture has no next_valid_id evidence")
	}
	if !sawInvalidTypeError {
		t.Fatal("paper invalid-order fixture has no order-scoped code 321 invalid order type rejection")
	}
	if sawUnexpectedOpenOrder {
		t.Fatal("paper invalid-order fixture unexpectedly has an open_order callback for the rejected order")
	}
}

func TestQuoteStreamFixtureContainsStreamingTicks(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_quote_stream_short_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "quote_stream_short" {
		t.Fatalf("Scenario = %q, want quote_stream_short", fixture.Metadata.Scenario)
	}

	var sawTickReqParams bool
	var sawMarketDataType bool
	var sawDelayedWarning bool
	var sawTickPrice bool
	var sawTickSize bool
	var sawTickString bool
	var sawSnapshotEnd bool
	for _, event := range fixture.Events {
		if event.ReqID != 301 {
			continue
		}
		switch event.Kind {
		case EventTickReqParams:
			if event.TickReqParams.BBOExchange == "9c0001" {
				sawTickReqParams = true
			}
		case EventMarketDataType:
			if event.MarketDataType == 3 {
				sawMarketDataType = true
			}
		case EventAPIError:
			if event.APIError.Code == 10167 {
				sawDelayedWarning = true
			}
		case EventTickPrice:
			sawTickPrice = true
		case EventTickSize:
			sawTickSize = true
		case EventTickString:
			sawTickString = true
		case EventTickSnapshotEnd:
			sawSnapshotEnd = true
		}
	}
	if !sawTickReqParams {
		t.Fatal("quote stream fixture has no tick_req_params BBO exchange evidence")
	}
	if !sawMarketDataType {
		t.Fatal("quote stream fixture has no delayed market_data_type callback")
	}
	if !sawDelayedWarning {
		t.Fatal("quote stream fixture has no delayed-data warning callback")
	}
	if !sawTickPrice || !sawTickSize || !sawTickString {
		t.Fatalf("quote stream fixture tick evidence price=%v size=%v string=%v, want all true", sawTickPrice, sawTickSize, sawTickString)
	}
	if sawSnapshotEnd {
		t.Fatal("quote stream fixture unexpectedly has tick_snapshot_end for a streaming subscription")
	}
}

func TestRealTimeBarsFixtureContainsEntitlementError(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_real_time_bars_short_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "real_time_bars_short" {
		t.Fatalf("Scenario = %q, want real_time_bars_short", fixture.Metadata.Scenario)
	}

	var sawEntitlementError bool
	var sawBar bool
	for _, event := range fixture.Events {
		if event.ReqID != 304 {
			continue
		}
		switch event.Kind {
		case EventAPIError:
			if event.APIError.Code == 420 &&
				strings.Contains(event.APIError.Message, "Invalid Real-time Query") &&
				strings.Contains(event.APIError.Message, "No market data permissions") {
				sawEntitlementError = true
			}
		case EventRealTimeBar:
			sawBar = true
		}
	}
	if !sawEntitlementError {
		t.Fatal("real-time bars fixture has no request-scoped code 420 entitlement error")
	}
	if sawBar {
		t.Fatal("real-time bars fixture unexpectedly has a bar callback despite entitlement error")
	}
}

func TestTickByTickFixtureContainsEntitlementError(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_tick_by_tick_midpoint_short_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "tick_by_tick_midpoint_short" {
		t.Fatalf("Scenario = %q, want tick_by_tick_midpoint_short", fixture.Metadata.Scenario)
	}

	var sawEntitlementError bool
	var sawTick bool
	for _, event := range fixture.Events {
		if event.ReqID != 302 {
			continue
		}
		switch event.Kind {
		case EventAPIError:
			if event.APIError.Code == 10189 &&
				strings.Contains(event.APIError.Message, "tick-by-tick") &&
				strings.Contains(event.APIError.Message, "No market data permissions") {
				sawEntitlementError = true
			}
		case EventTickByTick:
			sawTick = true
		}
	}
	if !sawEntitlementError {
		t.Fatal("tick-by-tick fixture has no request-scoped code 10189 entitlement error")
	}
	if sawTick {
		t.Fatal("tick-by-tick fixture unexpectedly has a data callback despite entitlement error")
	}
}

func TestMarketDepthFixtureContainsEntitlementError(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_market_depth_smart_short_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "market_depth_smart_short" {
		t.Fatalf("Scenario = %q, want market_depth_smart_short", fixture.Metadata.Scenario)
	}

	var sawEntitlementError bool
	var sawDepthData bool
	for _, event := range fixture.Events {
		if event.ReqID != 303 {
			continue
		}
		switch event.Kind {
		case EventAPIError:
			if event.APIError.Code == 2152 &&
				strings.Contains(event.APIError.Message, "Need additional market data permissions") &&
				strings.Contains(event.APIError.Message, "Depth:") {
				sawEntitlementError = true
			}
		case EventMarketDepth, EventMarketDepthL2:
			sawDepthData = true
		}
	}
	if !sawEntitlementError {
		t.Fatal("market-depth fixture has no request-scoped code 2152 entitlement error")
	}
	if sawDepthData {
		t.Fatal("market-depth fixture unexpectedly has a depth data callback despite entitlement error")
	}
}

func TestHistoricalBarsFixtureContainsBarsAndEnd(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_historical_bars_short_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "historical_bars_short" {
		t.Fatalf("Scenario = %q, want historical_bars_short", fixture.Metadata.Scenario)
	}

	var barCount int
	var sawFirstRTHBar bool
	var sawEndAfterBars bool
	for _, event := range fixture.Events {
		if event.ReqID != 401 {
			continue
		}
		switch event.Kind {
		case EventHistoricalData:
			barCount++
			if event.HistoricalBar.Time == "20260501 09:30:00 US/Eastern" &&
				event.HistoricalBar.Open != "" &&
				event.HistoricalBar.Close != "" &&
				event.HistoricalBar.Volume != "" {
				sawFirstRTHBar = true
			}
		case EventHistoricalDataEnd:
			if barCount > 0 {
				sawEndAfterBars = true
			}
		}
	}
	if barCount == 0 {
		t.Fatal("historical bars fixture has no historical_data rows")
	}
	if !sawFirstRTHBar {
		t.Fatal("historical bars fixture has no first RTH AAPL bar with OHLCV values")
	}
	if !sawEndAfterBars {
		t.Fatal("historical bars fixture has no historical_data_end after bars")
	}
}

func TestHistoricalBarsKeepUpFixtureContainsInitialSnapshot(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_historical_bars_keepup_short_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "historical_bars_keepup_short" {
		t.Fatalf("Scenario = %q, want historical_bars_keepup_short", fixture.Metadata.Scenario)
	}
	if fixture.Metadata.ServerVersion != 203 {
		t.Fatalf("ServerVersion = %d, want 203", fixture.Metadata.ServerVersion)
	}
	if fixture.Metadata.SourceSHA256 != "f6d87ab4c4a407bcd24e003647eebb87866a871411ad8bb06edc3769a15fbf32" {
		t.Fatalf("SourceSHA256 = %q, want captured source hash", fixture.Metadata.SourceSHA256)
	}

	var barCount int
	var sawFirstRTHBar bool
	var sawEndAfterBars bool
	var sawUpdate bool
	for _, event := range fixture.Events {
		if event.ReqID != 406 {
			continue
		}
		switch event.Kind {
		case EventHistoricalData:
			barCount++
			if event.HistoricalBar.Time == "20260501 09:30:00 US/Eastern" &&
				event.HistoricalBar.Open != "" &&
				event.HistoricalBar.Close != "" &&
				event.HistoricalBar.Volume != "" {
				sawFirstRTHBar = true
			}
		case EventHistoricalDataEnd:
			if barCount > 0 {
				sawEndAfterBars = true
			}
		case EventHistoricalDataUpdate:
			sawUpdate = true
		}
	}
	if barCount != 7 {
		t.Fatalf("historical keep-up fixture bars = %d, want captured seven-row initial snapshot", barCount)
	}
	if !sawFirstRTHBar {
		t.Fatal("historical keep-up fixture has no first RTH AAPL bar with OHLCV values")
	}
	if !sawEndAfterBars {
		t.Fatal("historical keep-up fixture has no historical_data_end after initial snapshot")
	}
	if sawUpdate {
		t.Fatal("historical keep-up fixture unexpectedly has historical_data_update on Saturday capture")
	}
}

func TestHistoricalScheduleFixtureContainsSessions(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_historical_schedule_short_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "historical_schedule_short" {
		t.Fatalf("Scenario = %q, want historical_schedule_short", fixture.Metadata.Scenario)
	}

	var schedule HistoricalScheduleValue
	for _, event := range fixture.Events {
		if event.Kind == EventHistoricalSchedule && event.ReqID == 402 {
			schedule = event.HistoricalSchedule
			break
		}
	}
	if schedule.TimeZone != "US/Eastern" {
		t.Fatalf("HistoricalSchedule.TimeZone = %q, want US/Eastern", schedule.TimeZone)
	}
	if schedule.StartDateTime != "20260402-09:30:00" || schedule.EndDateTime != "20260501-16:00:00" {
		t.Fatalf("HistoricalSchedule window = %q..%q, want captured AAPL window", schedule.StartDateTime, schedule.EndDateTime)
	}
	if len(schedule.Sessions) != 21 {
		t.Fatalf("HistoricalSchedule sessions len = %d, want 21", len(schedule.Sessions))
	}
	last := schedule.Sessions[len(schedule.Sessions)-1]
	if last.RefDate != "20260501" ||
		last.StartDateTime != "20260501-09:30:00" ||
		last.EndDateTime != "20260501-16:00:00" {
		t.Fatalf("last HistoricalSchedule session = %+v, want 20260501 RTH session", last)
	}
}

func TestHistoricalTicksFixtureContainsMidpointTicks(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_historical_ticks_midpoint_short_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "historical_ticks_midpoint_short" {
		t.Fatalf("Scenario = %q, want historical_ticks_midpoint_short", fixture.Metadata.Scenario)
	}

	var ticks []HistoricalTickValue
	var done bool
	for _, event := range fixture.Events {
		if event.Kind == EventHistoricalTicks && event.ReqID == 403 {
			ticks = event.HistoricalTicks
			done = event.HistoricalTicksDone
			break
		}
	}
	if !done {
		t.Fatal("historical ticks fixture has no done=true midpoint callback")
	}
	if len(ticks) < 10 {
		t.Fatalf("historical ticks len = %d, want at least 10", len(ticks))
	}
	first := ticks[0]
	if first.Time != "1777665598" || first.Price != "280.05500000000001" || first.Size != "+0E+0" {
		t.Fatalf("first historical midpoint tick = %+v, want captured first tick", first)
	}
}

func TestHistoricalTicksTradesFixtureContainsLastTicks(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_historical_ticks_trades_short_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "historical_ticks_trades_short" {
		t.Fatalf("Scenario = %q, want historical_ticks_trades_short", fixture.Metadata.Scenario)
	}
	if fixture.Metadata.ServerVersion != 203 {
		t.Fatalf("ServerVersion = %d, want 203", fixture.Metadata.ServerVersion)
	}
	if fixture.Metadata.SourceSHA256 != "f1792cfa680c5d33a0e9834f616236085bfa862af84900f8546d1a545f1f7aa0" {
		t.Fatalf("SourceSHA256 = %q, want captured source hash", fixture.Metadata.SourceSHA256)
	}

	var ticks []HistoricalTickLastValue
	var done bool
	for _, event := range fixture.Events {
		if event.Kind == EventHistoricalTicksLast && event.ReqID == 405 {
			ticks = event.HistoricalTicksLast
			done = event.HistoricalTicksDone
			break
		}
	}
	if !done {
		t.Fatal("historical trade ticks fixture has no done=true callback")
	}
	if len(ticks) < 10 {
		t.Fatalf("historical trade ticks len = %d, want at least 10", len(ticks))
	}
	first := ticks[0]
	if first.Time != "1777665599" ||
		first.Price != "280.04000000000002" ||
		first.Size != "+100E+0" ||
		first.Exchange != "NASDAQ" ||
		first.SpecialConditions != " F  " {
		t.Fatalf("first historical trade tick = %+v, want captured first trade", first)
	}
}

func TestHistoricalTicksBidAskFixtureContainsBidAskTicks(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_historical_ticks_bidask_short_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "historical_ticks_bidask_short" {
		t.Fatalf("Scenario = %q, want historical_ticks_bidask_short", fixture.Metadata.Scenario)
	}
	if fixture.Metadata.ServerVersion != 203 {
		t.Fatalf("ServerVersion = %d, want 203", fixture.Metadata.ServerVersion)
	}
	if fixture.Metadata.SourceSHA256 != "7a344933e7d5dd06221df36f9f3282b63bb2377c63149fd54899a19faf8ec3ec" {
		t.Fatalf("SourceSHA256 = %q, want captured source hash", fixture.Metadata.SourceSHA256)
	}

	var ticks []HistoricalTickBidAskValue
	var done bool
	for _, event := range fixture.Events {
		if event.Kind == EventHistoricalTicksBidAsk && event.ReqID == 404 {
			ticks = event.HistoricalTicksBidAsk
			done = event.HistoricalTicksDone
			break
		}
	}
	if !done {
		t.Fatal("historical bid/ask ticks fixture has no done=true callback")
	}
	if len(ticks) < 10 {
		t.Fatalf("historical bid/ask ticks len = %d, want at least 10", len(ticks))
	}
	first := ticks[0]
	if first.Time != "1777665598" ||
		first.BidPrice != "280.04000000000002" ||
		first.AskPrice != "280.06999999999999" ||
		first.BidSize != "+200E+0" ||
		first.AskSize != "+80E+0" {
		t.Fatalf("first historical bid/ask tick = %+v, want captured first quote", first)
	}
}

func TestFundamentalDataFixtureContainsReportSnapshot(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_fundamental_data_snapshot_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "fundamental_data_snapshot" {
		t.Fatalf("Scenario = %q, want fundamental_data_snapshot", fixture.Metadata.Scenario)
	}

	var report string
	var sawReqError bool
	for _, event := range fixture.Events {
		if event.ReqID != 501 {
			continue
		}
		switch event.Kind {
		case EventFundamentalData:
			report = event.FundamentalData
		case EventAPIError:
			sawReqError = true
		}
	}
	if sawReqError {
		t.Fatal("fundamental data fixture unexpectedly has a request-scoped API error")
	}
	for _, want := range []string{
		"<ReportSnapshot",
		`<CoID Type="CompanyName">Apple Inc</CoID>`,
		`<IssueID Type="Ticker">AAPL</IssueID>`,
		"<Ratios ",
		"<ForecastData ",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("FundamentalData report does not contain %q", want)
		}
	}
}

func TestScannerSubscriptionFixtureContainsRows(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_scanner_subscription_short_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "scanner_subscription_short" {
		t.Fatalf("Scenario = %q, want scanner_subscription_short", fixture.Metadata.Scenario)
	}

	var rows []ScannerDataValue
	var sawReqError bool
	for _, event := range fixture.Events {
		if event.ReqID != 701 {
			continue
		}
		switch event.Kind {
		case EventScannerData:
			rows = event.ScannerData
		case EventAPIError:
			sawReqError = true
		}
	}
	if sawReqError {
		t.Fatal("scanner subscription fixture unexpectedly has a request-scoped API error")
	}
	if len(rows) != 5 {
		t.Fatalf("scanner rows len = %d, want 5", len(rows))
	}
	for i, row := range rows {
		if row.Rank != i {
			t.Fatalf("scanner row %d rank = %d, want %d", i, row.Rank, i)
		}
		if row.Contract.SecType != "STK" || row.Contract.Exchange != "SMART" || row.Contract.Currency != "USD" {
			t.Fatalf("scanner row %d contract = %+v, want SMART USD stock", i, row.Contract)
		}
		if row.Contract.Symbol == "" || row.Contract.ConID == 0 {
			t.Fatalf("scanner row %d contract = %+v, want symbol and conID", i, row.Contract)
		}
	}
}

func TestScannerParametersFixtureContainsCatalogXML(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_scanner_parameters_snapshot_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "scanner_parameters_snapshot" {
		t.Fatalf("Scenario = %q, want scanner_parameters_snapshot", fixture.Metadata.Scenario)
	}

	var xml string
	for _, event := range fixture.Events {
		if event.Kind == EventScannerParameters {
			xml = event.ScannerXML
			break
		}
	}
	if len(xml) < 1_000_000 {
		t.Fatalf("ScannerXML len = %d, want full scanner catalog XML", len(xml))
	}
	for _, want := range []string{
		"<ScanParameterResponse>",
		`<InstrumentList varName="instrumentList">`,
		"<name>US Stocks</name>",
		"<type>STK</type>",
		"TOP_PERC_GAIN",
		"</FilterList>",
	} {
		if !strings.Contains(xml, want) {
			t.Fatalf("ScannerXML does not contain %q", want)
		}
	}
}

func TestDisplayGroupSubscriptionFixtureContainsInitialUpdate(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_display_group_subscription_short_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "display_group_subscription_short" {
		t.Fatalf("Scenario = %q, want display_group_subscription_short", fixture.Metadata.Scenario)
	}

	var groups string
	var update string
	var sawReqError bool
	for _, event := range fixture.Events {
		switch {
		case event.Kind == EventDisplayGroupList && event.ReqID == 801:
			groups = event.DisplayGroups
		case event.Kind == EventDisplayGroupUpdated && event.ReqID == 802:
			update = event.DisplayGroupContractInfo
		case event.Kind == EventAPIError && event.ReqID == 802:
			sawReqError = true
		}
	}
	if groups != "1|2|3|4|5|6|7" {
		t.Fatalf("DisplayGroups = %q, want captured Gateway group list", groups)
	}
	if sawReqError {
		t.Fatal("display group subscription fixture unexpectedly has a request-scoped API error")
	}
	if update != "none" {
		t.Fatalf("DisplayGroupContractInfo = %q, want captured initial update", update)
	}
}

func TestNewsInvalidRequestsFixtureContainsProviderErrors(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_news_invalid_requests_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "news_invalid_requests" {
		t.Fatalf("Scenario = %q, want news_invalid_requests", fixture.Metadata.Scenario)
	}

	var sawHistoricalError bool
	var sawArticleError bool
	var sawUnexpectedNewsData bool
	for _, event := range fixture.Events {
		switch event.Kind {
		case EventAPIError:
			if event.APIError.Code != 321 ||
				!strings.Contains(event.APIError.Message, "Not subscribed for 'NO_SUCH_PROVIDER' provider") {
				continue
			}
			switch event.ReqID {
			case 901:
				sawHistoricalError = true
			case 902:
				sawArticleError = true
			}
		case EventHistoricalNews, EventHistoricalNewsEnd, EventNewsArticle:
			if event.ReqID == 901 || event.ReqID == 902 {
				sawUnexpectedNewsData = true
			}
		}
	}
	if !sawHistoricalError {
		t.Fatal("invalid news fixture has no historical-news provider error")
	}
	if !sawArticleError {
		t.Fatal("invalid news fixture has no article provider error")
	}
	if sawUnexpectedNewsData {
		t.Fatal("invalid news fixture unexpectedly has news data callbacks")
	}
}

func TestNewsArticleSnapshotFixtureContainsRedactedSuccessCallbacks(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_news_article_snapshot_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "news_article_snapshot" {
		t.Fatalf("Scenario = %q, want news_article_snapshot", fixture.Metadata.Scenario)
	}
	if fixture.Metadata.ServerVersion != 203 {
		t.Fatalf("ServerVersion = %d, want 203", fixture.Metadata.ServerVersion)
	}
	if fixture.Metadata.SourceSHA256 != "e25f9cf9301804a7c6614efc580e1825d7487ab5b501b45242a67edde03567f6" {
		t.Fatalf("SourceSHA256 = %q, want captured source hash", fixture.Metadata.SourceSHA256)
	}
	if !strings.Contains(fixture.Metadata.RedactionNotes, "article text") {
		t.Fatalf("RedactionNotes = %q, want article text redaction note", fixture.Metadata.RedactionNotes)
	}

	var historicalRows int
	var sawEnd bool
	var sawArticle bool
	for _, event := range fixture.Events {
		switch event.Kind {
		case EventHistoricalNews:
			if event.ReqID != 903 {
				continue
			}
			historicalRows++
			if event.HistoricalNews.ProviderCode == "" || event.HistoricalNews.Time == "" {
				t.Fatalf("historical news event missing provider/time: %+v", event.HistoricalNews)
			}
			if event.HistoricalNews.ArticleID != "REDACTED_ARTICLE_ID" ||
				event.HistoricalNews.Headline != "REDACTED_HEADLINE" {
				t.Fatalf("historical news event not redacted: %+v", event.HistoricalNews)
			}
		case EventHistoricalNewsEnd:
			if event.ReqID == 903 && event.HistoricalHasMore {
				sawEnd = true
			}
		case EventNewsArticle:
			if event.ReqID != 904 {
				continue
			}
			sawArticle = true
			if event.NewsArticle.ArticleText != "REDACTED_ARTICLE_TEXT" {
				t.Fatalf("news article text = %q, want redacted placeholder", event.NewsArticle.ArticleText)
			}
		case EventAPIError:
			if event.ReqID == 903 || event.ReqID == 904 {
				t.Fatalf("news article snapshot has request-scoped API error: %+v", event.APIError)
			}
		}
	}
	if historicalRows != 5 {
		t.Fatalf("historical news rows = %d, want 5", historicalRows)
	}
	if !sawEnd {
		t.Fatal("news article snapshot has no historical_news_end with hasMore")
	}
	if !sawArticle {
		t.Fatal("news article snapshot has no news_article callback")
	}
}

func TestOptionCalculationsFixtureContainsSecurityDefinitionErrors(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_option_calculations_short_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "option_calculations_short" {
		t.Fatalf("Scenario = %q, want option_calculations_short", fixture.Metadata.Scenario)
	}

	var sawImpliedError bool
	var sawPriceError bool
	var sawComputation bool
	for _, event := range fixture.Events {
		switch event.Kind {
		case EventAPIError:
			if event.APIError.Code != 200 ||
				!strings.Contains(event.APIError.Message, "No security definition has been found") {
				continue
			}
			switch event.ReqID {
			case 1001:
				sawImpliedError = true
			case 1002:
				sawPriceError = true
			}
		case EventTickOptionComputation:
			if event.ReqID == 1001 || event.ReqID == 1002 {
				sawComputation = true
			}
		}
	}
	if !sawImpliedError {
		t.Fatal("option calculation fixture has no implied-volatility security-definition error")
	}
	if !sawPriceError {
		t.Fatal("option calculation fixture has no option-price security-definition error")
	}
	if sawComputation {
		t.Fatal("option calculation fixture unexpectedly has option computation callbacks")
	}
}

func TestQualifiedOptionCalculationsFixtureContainsComputations(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_option_calculations_qualified_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "option_calculations_qualified" {
		t.Fatalf("Scenario = %q, want option_calculations_qualified", fixture.Metadata.Scenario)
	}
	if fixture.Metadata.SourceSHA256 != "c51064689faf03226922a56aa00da6d464784b60408d7f688ab5e6b9778435e5" {
		t.Fatalf("SourceSHA256 = %q, want qualified option capture hash", fixture.Metadata.SourceSHA256)
	}

	var sawContract bool
	var sawImpliedComputation bool
	var sawPriceComputation bool
	for _, event := range fixture.Events {
		switch event.Kind {
		case EventContractDetails:
			if event.ReqID == 1000 &&
				event.ContractDetails.Contract.ConID == 675813159 &&
				event.ContractDetails.Contract.LocalSymbol == "AAPL  260618C00200000" &&
				event.ContractDetails.Contract.TradingClass == "AAPL" {
				sawContract = true
			}
		case EventTickOptionComputation:
			switch event.ReqID {
			case 1001:
				if event.TickOptionComputation.ImpliedVol == "0.17100140275259834" &&
					event.TickOptionComputation.OptPrice == "5.25" &&
					event.TickOptionComputation.UndPrice == "200" {
					sawImpliedComputation = true
				}
			case 1002:
				if event.TickOptionComputation.ImpliedVol == "0.29999999999999999" &&
					event.TickOptionComputation.OptPrice == "8.9158022449933707" &&
					event.TickOptionComputation.UndPrice == "200" {
					sawPriceComputation = true
				}
			}
		case EventAPIError:
			if event.ReqID == 1000 || event.ReqID == 1001 || event.ReqID == 1002 {
				t.Fatalf("qualified option fixture has request-scoped API error: %+v", event.APIError)
			}
		}
	}
	if !sawContract {
		t.Fatal("qualified option fixture has no contractDetails callback")
	}
	if !sawImpliedComputation {
		t.Fatal("qualified option fixture has no implied-volatility computation callback")
	}
	if !sawPriceComputation {
		t.Fatal("qualified option fixture has no option-price computation callback")
	}
}

func TestCompletedOrdersFixtureContainsRedactedSnapshot(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_completed_orders_snapshot_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "completed_orders_snapshot" {
		t.Fatalf("Scenario = %q, want completed_orders_snapshot", fixture.Metadata.Scenario)
	}
	if fixture.Metadata.ServerVersion != 203 {
		t.Fatalf("ServerVersion = %d, want 203", fixture.Metadata.ServerVersion)
	}
	if fixture.Metadata.SourceSHA256 != "3e7d4b241f5b122c6802b13b788b367e4583eaa77b7bfd442fd462fc54d66696" {
		t.Fatalf("SourceSHA256 = %q, want captured source hash", fixture.Metadata.SourceSHA256)
	}

	var completedRows int
	var sawEnd bool
	for _, event := range fixture.Events {
		switch event.Kind {
		case EventCompletedOrder:
			completedRows++
			assertPrivateContractRedacted(t, event.CompletedOrder.Contract)
			assertRedactedValue(t, "CompletedOrder.Action", event.CompletedOrder.Action)
			assertRedactedValue(t, "CompletedOrder.OrderType", event.CompletedOrder.OrderType)
			assertRedactedValue(t, "CompletedOrder.Status", event.CompletedOrder.Status)
			assertRedactedValue(t, "CompletedOrder.Quantity", event.CompletedOrder.Quantity)
			assertRedactedValue(t, "CompletedOrder.Filled", event.CompletedOrder.Filled)
			assertRedactedValue(t, "CompletedOrder.Remaining", event.CompletedOrder.Remaining)
		case EventCompletedOrderEnd:
			sawEnd = true
		}
	}
	if completedRows == 0 {
		t.Fatal("completed-orders fixture has no completed_order callbacks")
	}
	if !sawEnd {
		t.Fatal("completed-orders fixture has no completedOrdersEnd callback")
	}
}

func TestExecutionsEmptyFilterFixtureContainsEndOnly(t *testing.T) {
	f, err := os.Open("testdata/fixtures/official_sdk_executions_empty_filter_20260502.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if fixture.Metadata.Scenario != "executions_empty_filter" {
		t.Fatalf("Scenario = %q, want executions_empty_filter", fixture.Metadata.Scenario)
	}

	var sawEnd bool
	var sawExecutionDetail bool
	var sawCommissionReport bool
	for _, event := range fixture.Events {
		switch event.Kind {
		case EventExecutionsEnd:
			if event.ReqID == 1101 {
				sawEnd = true
			}
		case EventExecutionDetail:
			if event.ReqID == 1101 {
				sawExecutionDetail = true
			}
		case EventCommissionReport:
			sawCommissionReport = true
		}
	}
	if !sawEnd {
		t.Fatal("empty executions fixture has no execDetailsEnd callback")
	}
	if sawExecutionDetail {
		t.Fatal("empty executions fixture unexpectedly has execution details")
	}
	if sawCommissionReport {
		t.Fatal("empty executions fixture unexpectedly has commission reports")
	}
}

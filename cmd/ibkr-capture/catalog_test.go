package main

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
	"github.com/shopspring/decimal"
)

func TestScenarioCatalogCoversEveryScenario(t *testing.T) {
	t.Parallel()

	entries, err := catalogEntries()
	if err != nil {
		t.Fatalf("catalogEntries() error = %v", err)
	}
	if len(entries) != len(scenarios) {
		t.Fatalf("catalog entries = %d, scenarios = %d", len(entries), len(scenarios))
	}
	for _, entry := range entries {
		if entry.Description == "" {
			t.Errorf("%s missing description", entry.Name)
		}
		if entry.Domain == "" {
			t.Errorf("%s missing domain", entry.Name)
		}
		if len(entry.PublicAPI) == 0 {
			t.Errorf("%s missing public API", entry.Name)
		}
		if len(entry.MessageIDs) == 0 {
			t.Errorf("%s missing message IDs", entry.Name)
		}
		if entry.RiskClass == "" {
			t.Errorf("%s missing risk class", entry.Name)
		}
		if len(entry.ExpectedOutcomes) == 0 {
			t.Errorf("%s missing expected outcomes", entry.Name)
		}
		if len(entry.Batches) == 0 {
			t.Errorf("%s missing batches", entry.Name)
		}
		if entry.DefaultClientID < 0 {
			t.Errorf("%s default client ID = %d, want >= 0", entry.Name, entry.DefaultClientID)
		}
		switch entry.PromotionStatus {
		case "candidate", "blocked", "promoted":
		default:
			t.Errorf("%s has invalid promotion status %q", entry.Name, entry.PromotionStatus)
		}
	}
}

func TestUserInfoCatalogMessageIDs(t *testing.T) {
	t.Parallel()

	got := scenarios["user_info"].metadata.MessageIDs
	want := []int{protocol.OutReqUserInfo, protocol.InUserInfo}
	if !slices.Equal(got, want) {
		t.Fatalf("user_info message IDs = %v, want request/response %v", got, want)
	}
}

func TestTWSConfigCatalogMessageIDs(t *testing.T) {
	t.Parallel()

	got := scenarios["tws_config"].metadata.MessageIDs
	want := []int{protocol.OutReqConfig, protocol.InConfig}
	if !slices.Equal(got, want) {
		t.Fatalf("tws_config message IDs = %v, want request/response %v", got, want)
	}
}

func TestOddLotScenarioUsesNamedGenericTick(t *testing.T) {
	t.Parallel()

	if GenericTickOddLotBidAsk := string(ibkr.GenericTickOddLotBidAsk); GenericTickOddLotBidAsk != "787" {
		t.Fatalf("GenericTickOddLotBidAsk = %q, want 787", GenericTickOddLotBidAsk)
	}
	if !slices.Contains(scenarios["quote_odd_lot_aapl"].metadata.Requirements, "live_market_data_for_odd_lots") {
		t.Fatal("odd-lot scenario does not expose its live entitlement requirement")
	}
}

func TestScenarioMessageIDsExistInProtocolRegistry(t *testing.T) {
	t.Parallel()

	known := make(map[int]struct{})
	for _, message := range protocol.Messages() {
		known[message.ID] = struct{}{}
	}
	for name, scenario := range scenarios {
		for _, id := range scenario.metadata.MessageIDs {
			if _, ok := known[id]; !ok {
				t.Errorf("scenario %q refers to unknown classic message ID %d", name, id)
			}
		}
	}
}

func TestWriteCatalogJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := writeCatalogJSON(&buf); err != nil {
		t.Fatalf("writeCatalogJSON() error = %v", err)
	}
	var entries []scenarioCatalogEntry
	if err := json.Unmarshal(buf.Bytes(), &entries); err != nil {
		t.Fatalf("catalog JSON did not decode: %v", err)
	}
	if len(entries) != len(scenarios) {
		t.Fatalf("JSON entries = %d, scenarios = %d", len(entries), len(scenarios))
	}
}

func TestWriteBatchList(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := writeBatchList(&buf, batchNewV2); err != nil {
		t.Fatalf("writeBatchList() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) == 0 {
		t.Fatal("new-v2 batch is empty")
	}
	for _, line := range lines {
		parts := strings.Split(line, "|")
		if len(parts) != 2 {
			t.Fatalf("batch line %q should be name|client_id", line)
		}
		if _, ok := scenarios[parts[0]]; !ok {
			t.Fatalf("batch line references unknown scenario %q", parts[0])
		}
	}
}

func TestWriteScenarioRole(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		scenarios []string
		want      string
	}{
		{
			name:      "read only",
			scenarios: []string{"quote_stream_multi_asset|1", "api_historical_matrix_aapl"},
			want:      captureRoleReadOnlyLive,
		},
		{
			name:      "paper",
			scenarios: []string{"api_pairs_trading_aapl_msft|1"},
			want:      captureRolePaperDev,
		},
		{
			name:      "mixed prefers paper",
			scenarios: []string{"api_historical_matrix_aapl", "api_pairs_trading_aapl_msft|1"},
			want:      captureRolePaperDev,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := writeScenarioRole(&buf, tc.scenarios); err != nil {
				t.Fatalf("writeScenarioRole() error = %v", err)
			}
			if got := strings.TrimSpace(buf.String()); got != tc.want {
				t.Fatalf("writeScenarioRole() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReplayBatches(t *testing.T) {
	t.Parallel()

	entries, err := catalogEntries()
	if err != nil {
		t.Fatalf("catalogEntries() error = %v", err)
	}

	var all bytes.Buffer
	if err := writeBatchList(&all, batchReplayAll); err != nil {
		t.Fatalf("writeBatchList(replay-all) error = %v", err)
	}
	allLines := strings.Split(strings.TrimSpace(all.String()), "\n")
	wantExecutable := 0
	for _, entry := range entries {
		if entry.PromotionStatus != "blocked" {
			wantExecutable++
		}
	}
	if len(allLines) != wantExecutable {
		t.Fatalf("replay-all entries = %d, want every non-blocked scenario %d", len(allLines), wantExecutable)
	}

	var defaults bytes.Buffer
	if err := writeBatchList(&defaults, batchReplayDefault); err != nil {
		t.Fatalf("writeBatchList(replay-default) error = %v", err)
	}
	defaultList := strings.Split(strings.TrimSpace(defaults.String()), "\n")
	defaultsByName := map[string]bool{}
	for _, line := range defaultList {
		parts := strings.Split(line, "|")
		if len(parts) != 2 {
			t.Fatalf("default replay line %q should be name|client_id", line)
		}
		defaultsByName[parts[0]] = true
	}
	if !defaultsByName["api_order_type_matrix_aapl"] {
		t.Fatal("replay-default missing curated API order matrix scenario")
	}
	for _, entry := range entries {
		if entry.DefaultReplay && !defaultsByName[entry.Name] {
			t.Fatalf("replay-default missing promoted scenario %q", entry.Name)
		}
	}
}

func TestExhaustiveBatchesArePopulated(t *testing.T) {
	t.Parallel()

	for _, batch := range []string{
		batchExhaustiveReadOnly,
		batchExhaustiveTrading,
		batchExhaustiveMarketHours,
		batchExhaustivePremarket,
		batchExhaustivePermissionProbes,
	} {
		var buf bytes.Buffer
		if err := writeBatchList(&buf, batch); err != nil {
			t.Fatalf("writeBatchList(%s) error = %v", batch, err)
		}
		if strings.TrimSpace(buf.String()) == "" {
			t.Fatalf("%s batch is empty", batch)
		}
	}
}

func TestExhaustivePlanScenariosAreCatalogued(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"api_tif_attribute_matrix_aapl",
		"api_security_type_probe_matrix",
		"api_generic_tick_matrix_aapl",
		"api_tick_news_aapl_probe",
		"api_scanner_subscription",
		"api_historical_matrix_aapl",
		"api_news_article_aapl",
		"api_wsh_variants_aapl",
		"api_algo_variants_aapl",
		"api_completed_orders_variants_aapl",
		"api_transmit_false_then_transmit_aapl",
		"api_duplicate_quote_subscriptions_aapl",
		"api_reconnect_active_order_aapl",
		"api_client_id0_order_observation_aapl",
		"api_cross_client_cancel_aapl",
		"api_pairs_trading_aapl_msft",
		"api_dollar_cost_averaging_aapl",
		"api_stop_loss_management_aapl",
		"api_bracket_trailing_stop_aapl",
	} {
		scenario, ok := scenarios[name]
		if !ok {
			t.Fatalf("scenario %q missing from executable scenario map", name)
		}
		if scenario.metadata.Domain == "" {
			t.Fatalf("scenario %q missing catalog metadata", name)
		}
	}
}

func TestOrderTypeMatrixCoversPublicOrderTypes(t *testing.T) {
	t.Parallel()

	entry := scenarios["api_order_type_matrix_aapl"].metadata
	text := strings.Join(entry.ExpectedOutcomes, " ")
	for _, orderType := range []string{
		"MKT",
		"LMT",
		"STP",
		"STP LMT",
		"MOC",
		"LOC",
		"MOO",
		"LOO",
		"TRAIL",
		"TRAIL LIMIT",
		"MIT",
		"LIT",
		"MTL",
		"REL",
		"PEG",
	} {
		if !strings.Contains(text, orderType) {
			t.Fatalf("api_order_type_matrix_aapl expected outcomes missing %q", orderType)
		}
	}
}

func TestMinimalPaperSizingDefaults(t *testing.T) {
	t.Parallel()

	if got := apiStockOrderQuantity.String(); got != "1" {
		t.Fatalf("apiStockOrderQuantity = %s, want 1", got)
	}
	if got := apiStockCampaignOrderQuantity.String(); got != "1" {
		t.Fatalf("apiStockCampaignOrderQuantity = %s, want 1", got)
	}
	if got := apiOptionContractQuantity.String(); got != "1" {
		t.Fatalf("apiOptionContractQuantity = %s, want 1", got)
	}
}

func TestScenarioCaptureRoleRejectsUnknownRisk(t *testing.T) {
	t.Parallel()

	if _, err := scenarioCaptureRole(scenarioMetadata{RiskClass: "future_mutation"}); err == nil {
		t.Fatal("scenarioCaptureRole() error = nil, want fail-closed unknown risk")
	}
}

func TestOrderObservationMergeAccumulatesExecutionQuantities(t *testing.T) {
	t.Parallel()

	first := orderObservation{executionQty: decimal.NewFromInt(200)}
	first.refreshFilledQty()
	second := orderObservation{executionQty: decimal.NewFromInt(150)}
	second.refreshFilledQty()
	first.Merge(second)
	if got := first.filledQty.String(); got != "350" {
		t.Fatalf("merged execution filledQty = %s, want 350", got)
	}

	status := orderObservation{statusQty: decimal.NewFromInt(500)}
	status.refreshFilledQty()
	first.Merge(status)
	if got := first.filledQty.String(); got != "500" {
		t.Fatalf("status filledQty = %s, want 500", got)
	}
}

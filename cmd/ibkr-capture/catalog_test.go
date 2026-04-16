package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

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
		if entry.Domain == "" {
			t.Errorf("%s missing domain", entry.Name)
		}
		if entry.Driver != driverWire && entry.Driver != driverAPI {
			t.Errorf("%s driver = %q, want %q or %q", entry.Name, entry.Driver, driverWire, driverAPI)
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
		if entry.PromotionStatus == "" {
			t.Errorf("%s missing promotion status", entry.Name)
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
	if len(allLines) != len(entries) {
		t.Fatalf("replay-all entries = %d, want every scenario %d", len(allLines), len(entries))
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
		"api_market_data_completeness_aapl",
		"api_historical_matrix_aapl",
		"api_news_article_aapl",
		"api_fundamental_reports_aapl",
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
		if _, ok := scenarios[name]; !ok {
			t.Fatalf("scenario %q missing from executable scenario map", name)
		}
		if _, ok := scenarioMetadataByName[name]; !ok {
			t.Fatalf("scenario %q missing catalog metadata", name)
		}
	}
}

func TestOrderTypeMatrixCoversPublicOrderTypes(t *testing.T) {
	t.Parallel()

	entry := scenarioMetadataByName["api_order_type_matrix_aapl"]
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

func TestAggressivePaperSizingDefaults(t *testing.T) {
	t.Parallel()

	if got := apiStockOrderQuantity.String(); got != "100" {
		t.Fatalf("apiStockOrderQuantity = %s, want 100", got)
	}
	if got := apiStockCampaignOrderQuantity.String(); got != "500" {
		t.Fatalf("apiStockCampaignOrderQuantity = %s, want 500", got)
	}
	if got := apiOptionContractQuantity.String(); got != "5" {
		t.Fatalf("apiOptionContractQuantity = %s, want 5", got)
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

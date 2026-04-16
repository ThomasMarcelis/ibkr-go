package main

import (
	"context"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/testing/ibkrlive"
)

func TestLiveCaptureHighSignalTradingScenarios(t *testing.T) {
	ibkrlive.RequireTrading(t)

	for _, name := range []string{
		"api_pairs_trading_aapl_msft",
		"api_dollar_cost_averaging_aapl",
		"api_stop_loss_management_aapl",
		"api_bracket_trailing_stop_aapl",
		"api_algorithmic_campaign_aapl",
	} {
		t.Run(name, func(t *testing.T) {
			events := runLiveCaptureScenario(t, name, 12*time.Minute)
			requireDriverEvent(t, events, "session_ready")
			requireDriverEvent(t, events, "pre_cleanup_global_cancel_sent")
			requireDriverEvent(t, events, "cleanup_global_cancel_sent")
			requireDriverEvent(t, events, "scenario_end")
			requireDriverEvent(t, events, "place_order_sent")
		})
	}
}

func TestLiveCapturePermissionAndMultiAssetScenarios(t *testing.T) {
	ibkrlive.RequireTrading(t)

	for _, name := range []string{
		"api_option_campaign_aapl",
		"api_combo_option_vertical_aapl",
		"api_future_campaign_mes",
		"api_forex_lifecycle_eurusd",
	} {
		t.Run(name, func(t *testing.T) {
			events := runLiveCaptureScenario(t, name, 8*time.Minute)
			requireDriverEvent(t, events, "session_ready")
			requireDriverEvent(t, events, "pre_cleanup_global_cancel_sent")
			requireDriverEvent(t, events, "cleanup_global_cancel_sent")
			requireDriverEvent(t, events, "scenario_end")
			requireAnyDriverEvent(t, events, "place_order_sent", "place_order_error", "option_qualify_error", "contract_probe_error")
		})
	}
}

func runLiveCaptureScenario(t *testing.T, name string, timeout time.Duration) []apiDriverEvent {
	t.Helper()

	cfg := ibkrlive.Require(t)
	sc, ok := scenarios[name]
	if !ok {
		t.Fatalf("scenario %q is not registered", name)
	}
	if sc.runAPI == nil {
		t.Fatalf("scenario %q is not an API scenario", name)
	}
	md, ok := scenarioMetadataByName[name]
	if !ok {
		t.Fatalf("scenario %q has no catalog metadata", name)
	}

	rec, err := newAPIDriverRecorder("", name)
	if err != nil {
		t.Fatalf("newAPIDriverRecorder: %v", err)
	}
	previous := apiDriver
	apiDriver = rec
	t.Cleanup(func() {
		apiDriver = previous
		_ = rec.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := sc.runAPI(ctx, cfg.Addr, md.DefaultClientID); err != nil {
		t.Fatalf("%s live run: %v", name, err)
	}
	events := rec.Events()
	if len(events) == 0 {
		t.Fatalf("%s produced no driver events", name)
	}
	return events
}

func requireDriverEvent(t *testing.T, events []apiDriverEvent, kind string) {
	t.Helper()
	for _, event := range events {
		if event.Kind == kind {
			return
		}
	}
	t.Fatalf("driver event %q not found; saw %v", kind, driverEventKinds(events))
}

func requireAnyDriverEvent(t *testing.T, events []apiDriverEvent, kinds ...string) {
	t.Helper()
	for _, event := range events {
		for _, kind := range kinds {
			if event.Kind == kind {
				return
			}
		}
	}
	t.Fatalf("none of driver events %v found; saw %v", kinds, driverEventKinds(events))
}

func driverEventKinds(events []apiDriverEvent) []string {
	kinds := make([]string, 0, len(events))
	for _, event := range events {
		kinds = append(kinds, event.Kind)
	}
	return kinds
}

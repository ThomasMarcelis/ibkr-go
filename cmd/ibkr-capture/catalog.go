package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

type scenarioMetadata struct {
	Domain           string   `json:"domain"`
	PublicAPI        []string `json:"public_api"`
	MessageIDs       []int    `json:"message_ids"`
	RiskClass        string   `json:"risk_class"`
	Assets           []string `json:"assets,omitempty"`
	Requirements     []string `json:"requirements,omitempty"`
	ExpectedOutcomes []string `json:"expected_outcomes"`
	DefaultClientID  int      `json:"default_client_id"`
	Batches          []string `json:"batches"`
	PromotionStatus  string   `json:"promotion_status"`
	DefaultReplay    bool     `json:"default_replay"`
}

type scenarioCatalogEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	scenarioMetadata
}

const (
	batchAll      = "all"
	batchReadOnly = "read-only"
	batchTrading  = "trading"
	batchNewV2    = "new-v2"

	batchTradingBasic     = "trading-basic"
	batchTradingAdvanced  = "trading-advanced"
	batchTradingCampaigns = "trading-campaigns"
	batchTradingAll       = "trading-all"
	batchReplayDefault    = "replay-default"
	batchReplayAll        = "replay-all"

	batchExhaustiveReadOnly         = "exhaustive-read-only"
	batchExhaustiveTrading          = "exhaustive-trading"
	batchExhaustiveMarketHours      = "exhaustive-market-hours"
	batchExhaustivePremarket        = "exhaustive-premarket"
	batchExhaustivePermissionProbes = "exhaustive-permission-probes"

	captureRoleReadOnlyLive = "readonly-live"
	captureRolePaperDev     = "paper-dev"
)

func meta(domain string, publicAPI []string, messageIDs []int, riskClass string, requirements []string, expected []string, defaultClientID int, promotionStatus string, batches ...string) scenarioMetadata {
	return scenarioMetadata{
		Domain:           domain,
		PublicAPI:        publicAPI,
		MessageIDs:       messageIDs,
		RiskClass:        riskClass,
		Requirements:     requirements,
		ExpectedOutcomes: expected,
		DefaultClientID:  defaultClientID,
		Batches:          append([]string{batchAll}, batches...),
		PromotionStatus:  promotionStatus,
		DefaultReplay:    promotionStatus == "promoted",
	}
}

func metaWithAssets(domain string, publicAPI []string, messageIDs []int, riskClass string, requirements []string, expected []string, defaultClientID int, promotionStatus string, assets []string, batches ...string) scenarioMetadata {
	md := meta(domain, publicAPI, messageIDs, riskClass, requirements, expected, defaultClientID, promotionStatus, batches...)
	md.Assets = append([]string(nil), assets...)
	return md
}

func catalogEntries() ([]scenarioCatalogEntry, error) {
	names := make([]string, 0, len(scenarios))
	for name := range scenarios {
		names = append(names, name)
	}
	sort.Strings(names)

	entries := make([]scenarioCatalogEntry, 0, len(names))
	for _, name := range names {
		sc := scenarios[name]
		if sc.run == nil {
			return nil, fmt.Errorf("scenario %s: missing runner", name)
		}
		if err := validateRiskClass(sc.metadata.RiskClass); err != nil {
			return nil, fmt.Errorf("scenario %s: %w", name, err)
		}
		entries = append(entries, scenarioCatalogEntry{
			Name:             name,
			Description:      sc.description,
			scenarioMetadata: sc.metadata,
		})
	}
	return entries, nil
}

func writeCatalogJSON(w io.Writer) error {
	entries, err := catalogEntries()
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}

func writeBatchList(w io.Writer, batch string) error {
	if batch == "" {
		batch = batchExhaustiveReadOnly
	}
	entries, err := catalogEntries()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.PromotionStatus == "blocked" {
			continue
		}
		if !entry.inBatch(batch) && !entry.inReplayDefaultBatch(batch) {
			continue
		}
		if _, err := fmt.Fprintf(w, "%s|%d\n", entry.Name, entry.DefaultClientID); err != nil {
			return err
		}
	}
	return nil
}

func writeScenarioRole(w io.Writer, scenarioNames []string) error {
	role := captureRoleReadOnlyLive
	for _, name := range scenarioNames {
		scenarioName := name
		if pipe := strings.IndexByte(scenarioName, '|'); pipe >= 0 {
			scenarioName = scenarioName[:pipe]
		}
		sc, ok := scenarios[scenarioName]
		if !ok {
			return fmt.Errorf("unknown scenario %q", scenarioName)
		}
		scenarioRole, err := scenarioCaptureRole(sc.metadata)
		if err != nil {
			return fmt.Errorf("scenario %q: %w", scenarioName, err)
		}
		if scenarioRole == captureRolePaperDev {
			role = captureRolePaperDev
		}
	}
	_, err := fmt.Fprintln(w, role)
	return err
}

// cancelsAllowedForRiskClass reports whether a scenario of the given catalog
// RiskClass is a paper-trading class that mutates order state. Only these
// classes run targeted cancellation and paper-state reconciliation and route
// to the paper-dev capture role. A separately gated global cancel is only the
// uncertain-cleanup fallback; every other class (read_only,
// entitlement_probe, ...) must never reach either mutation path. This is the
// single source of truth for that split.
func cancelsAllowedForRiskClass(riskClass string) bool {
	switch riskClass {
	case "paper_order", "paper_marketable_order", "paper_trigger", "paper_destructive":
		return true
	default:
		return false
	}
}

func validateRiskClass(riskClass string) error {
	switch riskClass {
	case "read_only", "entitlement_probe", "paper_order", "paper_marketable_order", "paper_trigger", "paper_destructive":
		return nil
	default:
		return fmt.Errorf("unknown risk class %q", riskClass)
	}
}

func scenarioCaptureRole(md scenarioMetadata) (string, error) {
	if err := validateRiskClass(md.RiskClass); err != nil {
		return "", err
	}
	if cancelsAllowedForRiskClass(md.RiskClass) {
		return captureRolePaperDev, nil
	}
	return captureRoleReadOnlyLive, nil
}

func (e scenarioCatalogEntry) inBatch(batch string) bool {
	if batch == batchReplayAll {
		return true
	}
	for _, candidate := range e.Batches {
		if candidate == batch {
			return true
		}
	}
	switch batch {
	case batchExhaustiveReadOnly:
		return e.RiskClass == "read_only" || e.RiskClass == "entitlement_probe"
	case batchExhaustiveTrading:
		return e.RiskClass == "paper_order" || e.RiskClass == "paper_trigger" ||
			e.RiskClass == "paper_marketable_order" || e.RiskClass == "paper_destructive"
	case batchExhaustiveMarketHours:
		return hasString(e.Requirements, "market_hours")
	case batchExhaustivePremarket:
		return hasString(e.Requirements, "premarket") || hasString(e.Requirements, "pre_market")
	case batchExhaustivePermissionProbes:
		return e.RiskClass == "entitlement_probe" ||
			hasString(e.Requirements, "option_permissions") ||
			hasString(e.Requirements, "future_permissions") ||
			hasString(e.Requirements, "forex_hours") ||
			hasString(e.Requirements, "security_type_permissions_or_real_error") ||
			hasString(e.Requirements, "wsh_subscription_or_error") ||
			hasString(e.Requirements, "news_or_historical_news") ||
			hasString(e.Requirements, "l2_market_data_or_error")
	}
	return false
}

func hasString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (e scenarioCatalogEntry) inReplayDefaultBatch(batch string) bool {
	return batch == batchReplayDefault && e.DefaultReplay
}

package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/capturelog"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
)

type verificationStats struct {
	clientBytes    int
	serverBytes    int
	connects       int
	disconnects    int
	clientFrames   int
	serverFrames   int
	clientMessages map[string]int
	serverMessages map[string]int
	serverVersions map[int]bool
	endMarkers     map[string]int
	apiErrors      []codec.APIError
}

func writeVerification(out io.Writer, captureDir string, meta capturelog.Meta, events []capturelog.Event, replayEvents []capturelog.ReplayEvent) error {
	if strings.TrimSpace(meta.Scenario) == "" {
		return fmt.Errorf("capture metadata has no scenario")
	}
	stats := verificationStats{
		clientMessages: make(map[string]int),
		serverMessages: make(map[string]int),
		serverVersions: make(map[int]bool),
		endMarkers:     make(map[string]int),
	}
	for _, event := range events {
		switch event.Kind {
		case capturelog.EventConnect:
			stats.connects++
		case capturelog.EventDisconnect:
			stats.disconnects++
		case "", capturelog.EventChunk:
			data, err := capturelog.DecodeData(event)
			if err != nil {
				return fmt.Errorf("decode leg %d %s chunk: %w", event.Leg, event.Direction, err)
			}
			switch event.Direction {
			case "client":
				stats.clientBytes += len(data)
			case "server":
				stats.serverBytes += len(data)
			}
		}
	}

	frameState := newCaptureFrameState()
	for _, event := range replayEvents {
		switch event.Kind {
		case capturelog.EventConnect:
			if err := frameState.connect(event.Leg); err != nil {
				return err
			}
			continue
		case capturelog.EventDisconnect:
			if err := frameState.disconnect(event.Leg); err != nil {
				return err
			}
			continue
		case capturelog.ReplayEventFrame:
		default:
			return fmt.Errorf("verify leg %d: unsupported replay event kind %q", event.Leg, event.Kind)
		}
		payload, err := base64.StdEncoding.DecodeString(event.Data)
		if err != nil {
			return fmt.Errorf("decode replay leg %d %s frame: %w", event.Leg, event.Direction, err)
		}
		description, err := frameState.describe(event, payload)
		if err != nil {
			return err
		}

		messageID := description.messageID()
		if !description.session {
			switch event.Direction {
			case "client":
				messageID = string(payload)
			case "server":
				messageID = strconv.Itoa(description.serverVersion)
				stats.serverVersions[description.serverVersion] = true
			}
		}
		switch event.Direction {
		case "client":
			stats.clientFrames++
			stats.clientMessages[messageID]++
		case "server":
			stats.serverFrames++
			stats.serverMessages[messageID]++
		default:
			return fmt.Errorf("verify leg %d: unsupported direction %q", event.Leg, event.Direction)
		}

		if !description.session || event.Direction != "server" {
			continue
		}
		msgID := description.msgID
		if message, ok := protocol.Lookup(protocol.ServerToClient, msgID); ok && strings.HasSuffix(message.Name, "End") {
			stats.endMarkers[message.Name]++
		}
		messages, err := codec.DecodeBatch(description.serverVersion, payload)
		if err != nil {
			return fmt.Errorf("verify leg %d server msg_id %d: %w", event.Leg, msgID, err)
		}
		for _, message := range messages {
			if malformed, ok := message.(codec.MalformedInbound); ok {
				return fmt.Errorf("verify leg %d server msg_id %d decoded malformed body: %w", event.Leg, msgID, malformed.Err)
			}
		}
		stats.observeDecoded(messages)
	}

	if stats.clientBytes == 0 || stats.serverBytes == 0 || stats.clientFrames == 0 || stats.serverFrames == 0 {
		return fmt.Errorf(
			"empty capture evidence (client=%dB/%df, server=%dB/%df)",
			stats.clientBytes,
			stats.clientFrames,
			stats.serverBytes,
			stats.serverFrames,
		)
	}
	if stats.connects != stats.disconnects {
		return fmt.Errorf("connect/disconnect mismatch connect=%d disconnect=%d", stats.connects, stats.disconnects)
	}

	hash, err := fileSHA256(filepath.Join(captureDir, "events.jsonl"))
	if err != nil {
		return err
	}
	name := filepath.Base(captureDir)
	if _, err := fmt.Fprintf(
		out,
		"  %-50s client=%6dB/%3df server=%7dB/%3df server_version=%s sha256=%x\n",
		name,
		stats.clientBytes,
		stats.clientFrames,
		stats.serverBytes,
		stats.serverFrames,
		formatServerVersions(stats.serverVersions),
		hash[:],
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "    client_msg_ids: %s\n", formatHistogram(stats.clientMessages)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "    server_msg_ids: %s\n", formatHistogram(stats.serverMessages)); err != nil {
		return err
	}
	if len(stats.endMarkers) > 0 {
		if _, err := fmt.Fprintf(out, "    end_markers: %s\n", formatHistogram(stats.endMarkers)); err != nil {
			return err
		}
	}
	if len(stats.apiErrors) > 0 {
		if _, err := fmt.Fprintf(out, "    api_errors: %s\n", formatAPIErrors(stats.apiErrors)); err != nil {
			return err
		}
	}
	driverLogPath := filepath.Join(captureDir, "driver.log")
	driverEventsPath := filepath.Join(captureDir, "driver_events.jsonl")
	_, driverLogErr := os.Stat(driverLogPath)
	_, driverEventsErr := os.Stat(driverEventsPath)
	driverLogExists := driverLogErr == nil
	driverEventsExist := driverEventsErr == nil
	if driverLogErr != nil && !errors.Is(driverLogErr, os.ErrNotExist) {
		return fmt.Errorf("stat driver log: %w", driverLogErr)
	}
	if driverEventsErr != nil && !errors.Is(driverEventsErr, os.ErrNotExist) {
		return fmt.Errorf("stat driver events: %w", driverEventsErr)
	}
	if driverLogExists != driverEventsExist {
		return fmt.Errorf("incomplete driver evidence: driver.log=%t driver_events.jsonl=%t", driverLogExists, driverEventsExist)
	}
	driver, err := verifyDriverEvents(driverEventsPath, meta, stats.apiErrors)
	if err != nil {
		return err
	}
	if driver.count >= 0 {
		_, err = fmt.Fprintf(out, "    driver_events: %d run_id=%s outcomes=%d\n", driver.count, driver.runID, driver.outcomes)
		return err
	}
	return nil
}

func (s *verificationStats) observeDecoded(messages []codec.Message) {
	for _, message := range messages {
		if apiErr, ok := message.(codec.APIError); ok {
			s.apiErrors = append(s.apiErrors, apiErr)
		}
	}
}

func fileSHA256(path string) ([sha256.Size]byte, error) {
	// #nosec G304 -- the operator explicitly selects the capture directory.
	file, err := os.Open(path)
	if err != nil {
		return [sha256.Size]byte{}, fmt.Errorf("hash capture events: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return [sha256.Size]byte{}, fmt.Errorf("hash capture events: %w", err)
	}
	var sum [sha256.Size]byte
	copy(sum[:], hash.Sum(nil))
	return sum, nil
}

type driverEvidence struct {
	At       time.Time         `json:"at"`
	Scenario string            `json:"scenario"`
	RunID    string            `json:"run_id"`
	Kind     string            `json:"kind"`
	Label    string            `json:"label"`
	Server   string            `json:"server"`
	ClientID int               `json:"client_id"`
	Count    int               `json:"count"`
	Status   string            `json:"status"`
	Values   map[string]string `json:"values"`
	Error    string            `json:"error"`
}

type driverEvidenceStats struct {
	count    int
	runID    string
	outcomes int
	kinds    map[string]int
	events   []driverEvidence
}

func verifyDriverEvents(path string, meta capturelog.Meta, apiErrors []codec.APIError) (driverEvidenceStats, error) {
	// #nosec G304 -- the operator explicitly selects the capture directory.
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return driverEvidenceStats{count: -1}, nil
	}
	if err != nil {
		return driverEvidenceStats{}, fmt.Errorf("open driver events: %w", err)
	}
	defer file.Close()
	stats := driverEvidenceStats{kinds: make(map[string]int)}
	var previous time.Time
	var starts, ready, ends int
	var lastKind string
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			return driverEvidenceStats{}, fmt.Errorf("driver events line %d is empty", stats.count+1)
		}
		var event driverEvidence
		if err := json.Unmarshal(line, &event); err != nil {
			return driverEvidenceStats{}, fmt.Errorf("decode driver event line %d: %w", stats.count+1, err)
		}
		stats.count++
		stats.kinds[event.Kind]++
		stats.events = append(stats.events, event)
		if event.Scenario != meta.Scenario {
			return driverEvidenceStats{}, fmt.Errorf("driver event line %d scenario %q does not match capture %q", stats.count, event.Scenario, meta.Scenario)
		}
		if event.RunID == "" {
			return driverEvidenceStats{}, fmt.Errorf("driver event line %d has no run_id", stats.count)
		}
		if stats.runID == "" {
			stats.runID = event.RunID
		} else if event.RunID != stats.runID {
			return driverEvidenceStats{}, fmt.Errorf("driver event line %d run_id %q does not match %q", stats.count, event.RunID, stats.runID)
		}
		if event.At.IsZero() || !previous.IsZero() && event.At.Before(previous) {
			return driverEvidenceStats{}, fmt.Errorf("driver event line %d has invalid chronology", stats.count)
		}
		previous = event.At
		lastKind = event.Kind
		if event.Kind == "paper_reconciliation_failed" {
			return driverEvidenceStats{}, fmt.Errorf("driver event line %d reports failed paper reconciliation: %s", stats.count, event.Error)
		}
		switch event.Kind {
		case "scenario_start":
			starts++
			if stats.count != 1 {
				return driverEvidenceStats{}, fmt.Errorf("scenario_start is driver event %d, want first", stats.count)
			}
			if event.ClientID != meta.ClientID {
				return driverEvidenceStats{}, fmt.Errorf("driver client_id %d does not match capture %d", event.ClientID, meta.ClientID)
			}
			if meta.ListenAddr != "" && event.Server != meta.ListenAddr {
				return driverEvidenceStats{}, fmt.Errorf("driver server %q does not match capture listen_addr %q", event.Server, meta.ListenAddr)
			}
		case "session_ready":
			ready++
		case "scenario_end":
			ends++
			if event.Error != "" && !isAttestedScenarioBlocker(meta.Scenario, event.Error, apiErrors) {
				return driverEvidenceStats{}, fmt.Errorf("scenario_end reports failure: %s", event.Error)
			}
			if event.Error != "" {
				stats.outcomes++
			}
		default:
			stats.outcomes++
		}
	}
	if err := scanner.Err(); err != nil {
		return driverEvidenceStats{}, fmt.Errorf("scan driver events: %w", err)
	}
	if stats.count == 0 {
		return driverEvidenceStats{}, fmt.Errorf("driver events are empty")
	}
	if starts != 1 || ready == 0 || ends != 1 {
		return driverEvidenceStats{}, fmt.Errorf("driver lifecycle start=%d ready=%d end=%d, want 1/>=1/1", starts, ready, ends)
	}
	if lastKind != "scenario_end" {
		return driverEvidenceStats{}, fmt.Errorf("last driver event is %q, want scenario_end", lastKind)
	}
	if stats.kinds["paper_baseline"] > 0 && stats.kinds["paper_reconciled"] != 1 {
		return driverEvidenceStats{}, fmt.Errorf("paper campaign baseline=%d reconciliation=%d, want 1/1", stats.kinds["paper_baseline"], stats.kinds["paper_reconciled"])
	}
	if stats.kinds["paper_reconciled"] > 0 && stats.kinds["paper_baseline"] != 1 {
		return driverEvidenceStats{}, fmt.Errorf("paper reconciliation has no unique baseline")
	}
	if stats.outcomes == 0 && scenarioNeedsDriverOutcome(meta.Scenario) {
		return driverEvidenceStats{}, fmt.Errorf("scenario %s has no result or attested blocker driver evidence", meta.Scenario)
	}
	if err := validateDriverScenarioEvidence(meta.Scenario, stats); err != nil {
		return driverEvidenceStats{}, err
	}
	return stats, nil
}

func scenarioNeedsDriverOutcome(scenario string) bool {
	return scenario != "bootstrap" && scenario != "bootstrap_client_id_0"
}

func validateDriverScenarioEvidence(scenario string, stats driverEvidenceStats) error {
	requireKinds := func(kinds ...string) error {
		for _, kind := range kinds {
			if stats.kinds[kind] == 0 {
				return fmt.Errorf("scenario %s has no %s driver evidence", scenario, kind)
			}
		}
		return nil
	}
	switch scenario {
	case "api_order_fill_aapl":
		return requireKinds("paper_baseline", "execution", "execution_and_fee_reconciled", "paper_reconciled")
	case "api_include_overnight_lifecycle_aapl":
		if err := requireKinds("paper_baseline", "paper_reconciled"); err != nil {
			return err
		}
		var truePlacement, falsePlacement, replacementDisposition bool
		for _, event := range stats.events {
			if event.Kind == "include_overnight_echo" && event.Label == "placement" && event.Values["include_overnight"] == "true" {
				truePlacement = true
			}
			if event.Kind == "include_overnight_echo" && event.Label == "fresh placement" &&
				event.Values["requested"] == "false" && event.Values["tif"] == "DAY" &&
				(event.Values["include_overnight"] == "false" || event.Values["include_overnight"] == "absent") {
				falsePlacement = true
			}
			if event.Kind == "include_overnight_echo" && event.Label == "replacement" && event.Values["include_overnight"] == "false" ||
				event.Kind == "include_overnight_blocked" && event.Label == "replacement" && event.Values["code"] == "462" {
				replacementDisposition = true
			}
		}
		if !truePlacement || !falsePlacement || !replacementDisposition {
			return fmt.Errorf("scenario %s lifecycle evidence true=%t fresh_false=%t replacement=%t", scenario, truePlacement, falsePlacement, replacementDisposition)
		}
	case "api_option_exercise_aapl":
		if err := requireKinds("paper_baseline", "paper_reconciled"); err != nil {
			return err
		}
		var accepted, presetWarning bool
		for _, event := range stats.events {
			if event.Kind == "option_exercise_completed" {
				return nil
			}
			if event.Kind == "option_exercise_event" {
				accepted = accepted || event.Status == "PreSubmitted"
				presetWarning = presetWarning || strings.Contains(event.Error, "code=10349")
			}
			if event.Kind == "order_warning" && event.Label == "option exercise seed" &&
				event.Values["code"] == "399" &&
				strings.Contains(event.Values["message"], "will not be placed at the exchange until") {
				return nil
			}
		}
		if accepted && presetWarning {
			return nil
		}
		return fmt.Errorf("scenario %s has neither a completed exercise nor exact market-hours blocker", scenario)
	}
	return nil
}

func isAttestedScenarioBlocker(scenario, scenarioErr string, apiErrors []codec.APIError) bool {
	for _, apiErr := range apiErrors {
		switch {
		case scenario == "api_include_overnight_lifecycle_aapl" &&
			strings.Contains(scenarioErr, "include overnight replacement echo") &&
			strings.Contains(scenarioErr, "want explicit false") &&
			apiErr.Code == 462 &&
			strings.Contains(apiErr.Message, "Cannot change to the new Time in Force.DAY"):
			return true
		case scenario == "contract_details_apple_bonds" &&
			attestedDriverAPIError(scenarioErr, apiErr, 2130, "products are trading on the basis of currency price with factor"):
			return true
		case slices.Contains([]string{
			"histogram_data_aapl",
			"historical_bars_1d_1h",
			"historical_bars_30d_1day",
			"historical_ticks_aapl_timezone_start",
			"historical_ticks_aapl_trades",
		}, scenario) && attestedDriverAPIError(scenarioErr, apiErr, 2188, "Up-to-the-second historical data requires additional subscription for the API."):
			return true
		case scenario == "historical_bars_keepup" &&
			attestedDriverAPIError(scenarioErr, apiErr, 162, "No market data permissions for ISLAND STK"):
			return true
		case scenario == "market_depth_aapl_smart" &&
			strings.Contains(scenarioErr, "required market-depth evidence not observed within 15s") &&
			apiErr.Code == 2152 &&
			strings.Contains(apiErr.Message, "Need additional market data permissions"):
			return true
		case scenario == "api_option_exercise_aapl" &&
			strings.Contains(scenarioErr, "option exercise seed status=Cancelled filled=0 execution=false") &&
			apiErr.Code == 399 &&
			strings.Contains(apiErr.Message, "will not be placed at the exchange until"):
			return true
		case scenario == "api_option_exercise_aapl" &&
			strings.Contains(scenarioErr, "AAPL call exercise produced no terminal evidence within 1m0s") &&
			apiErr.Code == 10349 &&
			strings.Contains(apiErr.Message, "Order TIF was set to DAY based on order preset"):
			return true
		case scenario == "quote_odd_lot_aapl" &&
			attestedDriverAPIError(scenarioErr, apiErr, 2186, "Warning: Requested real-time market data requires additional subscription for API. You elected to receive delayed market data instead."):
			return true
		}
	}
	return false
}

func attestedDriverAPIError(scenarioErr string, apiErr codec.APIError, code int, message string) bool {
	return apiErr.Code == code &&
		strings.Contains(apiErr.Message, message) &&
		strings.Contains(scenarioErr, "code="+strconv.Itoa(code)) &&
		strings.Contains(scenarioErr, apiErr.Message)
}

func formatServerVersions(versions map[int]bool) string {
	values := make([]int, 0, len(versions))
	for version := range versions {
		values = append(values, version)
	}
	if len(values) == 0 {
		return "?"
	}
	sort.Ints(values)
	parts := make([]string, len(values))
	for i, version := range values {
		parts[i] = strconv.Itoa(version)
	}
	return strings.Join(parts, ",")
}

func formatHistogram(histogram map[string]int) string {
	keys := make([]string, 0, len(histogram))
	for key := range histogram {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		left, leftErr := strconv.Atoi(keys[i])
		right, rightErr := strconv.Atoi(keys[j])
		if leftErr == nil && rightErr == nil {
			return left < right
		}
		if leftErr == nil {
			return true
		}
		if rightErr == nil {
			return false
		}
		return keys[i] < keys[j]
	})
	if len(keys) == 0 {
		return "-"
	}
	parts := make([]string, len(keys))
	for i, key := range keys {
		parts[i] = fmt.Sprintf("%s:%d", key, histogram[key])
	}
	return strings.Join(parts, ",")
}

func formatAPIErrors(apiErrors []codec.APIError) string {
	limit := min(len(apiErrors), 8)
	parts := make([]string, 0, limit+1)
	for _, apiErr := range apiErrors[:limit] {
		message := []rune(apiErr.Message)
		if len(message) > 80 {
			message = message[:80]
		}
		parts = append(parts, fmt.Sprintf("req=%d code=%d msg=%s", apiErr.ReqID, apiErr.Code, string(message)))
	}
	if remaining := len(apiErrors) - limit; remaining > 0 {
		parts = append(parts, fmt.Sprintf("... +%d more", remaining))
	}
	return strings.Join(parts, "; ")
}

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
		stats.observeDecoded(msgID, messages)
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
	driver, err := verifyDriverEvents(filepath.Join(captureDir, "driver_events.jsonl"), meta)
	if err != nil {
		return err
	}
	if driver.count >= 0 {
		_, err = fmt.Fprintf(out, "    driver_events: %d run_id=%s outcomes=%d\n", driver.count, driver.runID, driver.outcomes)
		return err
	}
	return nil
}

func (s *verificationStats) observeDecoded(msgID int, messages []codec.Message) {
	for _, message := range messages {
		if apiErr, ok := message.(codec.APIError); ok {
			s.apiErrors = append(s.apiErrors, apiErr)
		}
		// Before server_version 196, historical-data completion is carried
		// inside IN 17 rather than a standalone IN 108 envelope.
		if _, ok := message.(codec.HistoricalBarsEnd); ok && msgID == protocol.InHistoricalData {
			s.endMarkers["InHistoricalDataEnd"]++
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
	At       time.Time `json:"at"`
	Scenario string    `json:"scenario"`
	RunID    string    `json:"run_id"`
	Kind     string    `json:"kind"`
	Server   string    `json:"server"`
	ClientID int       `json:"client_id"`
	Error    string    `json:"error"`
}

type driverEvidenceStats struct {
	count    int
	runID    string
	outcomes int
}

func verifyDriverEvents(path string, meta capturelog.Meta) (driverEvidenceStats, error) {
	// #nosec G304 -- the operator explicitly selects the capture directory.
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return driverEvidenceStats{count: -1}, nil
	}
	if err != nil {
		return driverEvidenceStats{}, fmt.Errorf("open driver events: %w", err)
	}
	defer file.Close()
	stats := driverEvidenceStats{}
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
			if event.Error != "" {
				return driverEvidenceStats{}, fmt.Errorf("scenario_end reports failure: %s", event.Error)
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
	return stats, nil
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

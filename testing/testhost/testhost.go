package testhost

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

type Host struct {
	listener net.Listener
	addr     string
	steps    []step

	done chan struct{}

	mu  sync.Mutex
	err error
}

type step struct {
	kind      string
	direction string
	name      string
	body      map[string]any
	sizes     []int
	duration  time.Duration
	raw       []byte
}

func New(script string) (*Host, error) {
	steps, err := parse(script)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	h := &Host{
		listener: listener,
		addr:     listener.Addr().String(),
		steps:    steps,
		done:     make(chan struct{}),
	}
	go h.run()
	return h, nil
}

func NewFromFile(path string) (*Host, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return New(string(data))
}

func (h *Host) Addr() string {
	return h.addr
}

func (h *Host) Close() error {
	return h.listener.Close()
}

func (h *Host) Wait() error {
	<-h.done
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.err
}

func (h *Host) run() {
	defer close(h.done)

	bindings := map[string]any{}
	var conn net.Conn

	for i := 0; i < len(h.steps); i++ {
		cur := h.steps[i]
		switch cur.kind {
		case "sleep":
			time.Sleep(cur.duration)
		case "handshake":
			if conn == nil {
				var err error
				conn, err = h.listener.Accept()
				if err != nil {
					h.finish(err)
					return
				}
			}
			// 1. Read raw API prefix (4 bytes)
			prefix, err := readExact(conn, 4)
			if err != nil {
				h.finish(fmt.Errorf("testhost: handshake: read prefix: %w", err))
				return
			}
			if !bytes.Equal(prefix, []byte("API\x00")) {
				h.finish(fmt.Errorf("testhost: handshake: prefix = %x, want API\\x00", prefix))
				return
			}
			// 2. Read framed version string
			versionPayload, err := wire.ReadFrame(conn)
			if err != nil {
				h.finish(fmt.Errorf("testhost: handshake: read version: %w", err))
				return
			}
			_ = versionPayload
			// 3. Send framed server info
			serverVersion := asInt(resolveBindings(cur.body["server_version"], bindings))
			connTime := asString(resolveBindings(cur.body["connection_time"], bindings))
			serverInfoPayload := wire.EncodeFields([]string{strconv.Itoa(serverVersion), connTime})
			if err := wire.WriteFrame(conn, serverInfoPayload); err != nil {
				h.finish(fmt.Errorf("testhost: handshake: write server info: %w", err))
				return
			}
			// 4. Read framed START_API
			startPayload, err := wire.ReadFrame(conn)
			if err != nil {
				h.finish(fmt.Errorf("testhost: handshake: read start_api: %w", err))
				return
			}
			startFields, err := wire.ParseFields(startPayload)
			if err != nil {
				h.finish(fmt.Errorf("testhost: handshake: parse start_api: %w", err))
				return
			}
			wantStartAPI := strconv.Itoa(codec.OutStartAPI)
			if len(startFields) < 1 || startFields[0] != wantStartAPI {
				h.finish(fmt.Errorf("testhost: handshake: start_api msg_id = %v, want %s", startFields[0], wantStartAPI))
				return
			}
			// Store client_id in bindings if body requests it
			if cid, ok := cur.body["client_id"]; ok {
				if s, ok := cid.(string); ok && strings.HasPrefix(s, "$") {
					if len(startFields) >= 3 {
						bindings[s] = startFields[2]
					}
				}
			}
		case "disconnect":
			if conn != nil {
				_ = conn.Close()
				conn = nil
			}
		case "client":
			if conn == nil {
				var err error
				conn, err = h.listener.Accept()
				if err != nil {
					h.finish(err)
					return
				}
			}
			payload, err := wire.ReadFrame(conn)
			if err != nil {
				h.finish(err)
				return
			}
			name, body, err := decodeClientMessage(payload)
			if err != nil {
				h.finish(err)
				return
			}
			if name != cur.name {
				h.finish(fmt.Errorf("testhost: client message = %q, want %q", name, cur.name))
				return
			}
			if err := matchValue(cur.body, body, bindings); err != nil {
				h.finish(err)
				return
			}
		case "server":
			if conn == nil {
				var err error
				conn, err = h.listener.Accept()
				if err != nil {
					h.finish(err)
					return
				}
			}

			// Pack consecutive historical_bar steps into a single frame,
			// matching the real IBKR protocol where msg 17 carries all bars
			// in one batch: [17, reqID, N, bar1_fields..., bar2_fields...].
			if cur.name == "historical_bar" {
				bars := []step{cur}
				reqID := asString(resolveBindings(cur.body["req_id"], bindings))
				for j := i + 1; j < len(h.steps); j++ {
					next := h.steps[j]
					if next.kind != "server" || next.name != "historical_bar" {
						break
					}
					nextReqID := asString(resolveBindings(next.body["req_id"], bindings))
					if nextReqID != reqID {
						break
					}
					bars = append(bars, next)
				}
				payload, err := buildPackedHistoricalBars(bars, bindings)
				if err != nil {
					h.finish(err)
					return
				}
				if err := wire.WriteFrame(conn, payload); err != nil {
					h.finish(err)
					return
				}
				// Advance past consumed bar steps (current step is bars[0])
				i += len(bars) - 1
				// Skip a trailing historical_bars_end step if present, since
				// the packed frame already causes the decoder to emit one.
				if i+1 < len(h.steps) {
					next := h.steps[i+1]
					if next.kind == "server" && next.name == "historical_bars_end" {
						i++
					}
				}
				continue
			}

			msg, err := buildMessage(cur.name, cur.body, bindings)
			if err != nil {
				h.finish(err)
				return
			}
			payload, err := codec.Encode(msg)
			if err != nil {
				h.finish(err)
				return
			}
			if err := wire.WriteFrame(conn, payload); err != nil {
				h.finish(err)
				return
			}
		case "split":
			if conn == nil {
				var err error
				conn, err = h.listener.Accept()
				if err != nil {
					h.finish(err)
					return
				}
			}
			msg, err := buildMessage(cur.name, cur.body, bindings)
			if err != nil {
				h.finish(err)
				return
			}
			var payload []byte
			if cur.direction == "server" {
				payload, err = codec.Encode(msg)
			} else {
				payload, err = codec.Encode(msg)
			}
			if err != nil {
				h.finish(err)
				return
			}
			frame := appendLengthPrefix(payload)
			switch cur.direction {
			case "server":
				cursor := 0
				for _, size := range cur.sizes {
					if cursor >= len(frame) {
						break
					}
					end := cursor + size
					if end > len(frame) {
						end = len(frame)
					}
					if _, err := conn.Write(frame[cursor:end]); err != nil {
						h.finish(err)
						return
					}
					cursor = end
				}
				if cursor < len(frame) {
					if _, err := conn.Write(frame[cursor:]); err != nil {
						h.finish(err)
						return
					}
				}
			case "client":
				got, err := readChunked(conn, len(frame), cur.sizes)
				if err != nil {
					h.finish(err)
					return
				}
				if !bytes.Equal(got, frame) {
					h.finish(fmt.Errorf("testhost: split client frame = %x, want %x", got, frame))
					return
				}
			default:
				h.finish(fmt.Errorf("testhost: unsupported split direction %q", cur.direction))
				return
			}
		case "raw":
			if conn == nil {
				var err error
				conn, err = h.listener.Accept()
				if err != nil {
					h.finish(err)
					return
				}
			}
			switch cur.direction {
			case "server":
				if _, err := conn.Write(cur.raw); err != nil {
					h.finish(err)
					return
				}
			case "client":
				got, err := readExact(conn, len(cur.raw))
				if err != nil {
					h.finish(err)
					return
				}
				if !bytes.Equal(got, cur.raw) {
					h.finish(fmt.Errorf("testhost: raw client bytes = %x, want %x", got, cur.raw))
					return
				}
			default:
				h.finish(fmt.Errorf("testhost: unsupported raw direction %q", cur.direction))
				return
			}
		}
	}

	if conn != nil {
		_ = conn.Close()
	}
}

func (h *Host) finish(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.err = err
}

func parse(script string) ([]step, error) {
	lines := strings.Split(script, "\n")
	steps := make([]step, 0, len(lines))
	for idx, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		switch {
		case strings.HasPrefix(line, "sleep "):
			d, err := time.ParseDuration(strings.TrimSpace(strings.TrimPrefix(line, "sleep ")))
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", idx+1, err)
			}
			steps = append(steps, step{kind: "sleep", duration: d})
		case strings.HasPrefix(line, "handshake "):
			body, err := parseBody(strings.TrimPrefix(line, "handshake "))
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", idx+1, err)
			}
			steps = append(steps, step{kind: "handshake", body: body})
		case line == "disconnect":
			steps = append(steps, step{kind: "disconnect"})
		case strings.HasPrefix(line, "raw "):
			parts := strings.SplitN(line, " ", 3)
			if len(parts) != 3 {
				return nil, fmt.Errorf("line %d: invalid raw step", idx+1)
			}
			raw, err := base64.StdEncoding.DecodeString(parts[2])
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", idx+1, err)
			}
			steps = append(steps, step{kind: "raw", direction: parts[1], raw: raw})
		case strings.HasPrefix(line, "split "):
			parts := strings.SplitN(line, " ", 5)
			if len(parts) != 5 {
				return nil, fmt.Errorf("line %d: invalid split step", idx+1)
			}
			body, err := parseBody(parts[4])
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", idx+1, err)
			}
			steps = append(steps, step{
				kind:      "split",
				direction: parts[1],
				sizes:     parseSizes(parts[2]),
				name:      parts[3],
				body:      body,
			})
		default:
			parts := strings.SplitN(line, " ", 3)
			if len(parts) != 3 {
				return nil, fmt.Errorf("line %d: invalid step", idx+1)
			}
			body, err := parseBody(parts[2])
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", idx+1, err)
			}
			steps = append(steps, step{
				kind:      parts[0],
				direction: parts[0],
				name:      parts[1],
				body:      body,
			})
		}
	}
	return steps, nil
}

func parseBody(raw string) (map[string]any, error) {
	var body map[string]any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return nil, err
	}
	return body, nil
}

func parseSizes(raw string) []int {
	parts := strings.Split(raw, ",")
	sizes := make([]int, 0, len(parts))
	for _, part := range parts {
		value, _ := strconv.Atoi(strings.TrimSpace(part))
		if value > 0 {
			sizes = append(sizes, value)
		}
	}
	return sizes
}

func appendLengthPrefix(payload []byte) []byte {
	header := []byte{0, 0, 0, 0}
	size := len(payload)
	header[0] = byte(size >> 24)
	header[1] = byte(size >> 16)
	header[2] = byte(size >> 8)
	header[3] = byte(size)
	return append(header, payload...)
}

func readExact(r io.Reader, size int) ([]byte, error) {
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func readChunked(r io.Reader, total int, sizes []int) ([]byte, error) {
	buf := make([]byte, 0, total)
	cursor := 0
	for _, size := range sizes {
		if cursor >= total {
			break
		}
		if size <= 0 {
			continue
		}
		remaining := total - cursor
		if size > remaining {
			size = remaining
		}
		chunk, err := readExact(r, size)
		if err != nil {
			return nil, err
		}
		buf = append(buf, chunk...)
		cursor += size
	}
	if cursor < total {
		chunk, err := readExact(r, total-cursor)
		if err != nil {
			return nil, err
		}
		buf = append(buf, chunk...)
	}
	return buf, nil
}

// decodeClientMessage decodes a real wire format client message into
// a name and body map for transcript matching.
func decodeClientMessage(payload []byte) (string, map[string]any, error) {
	fields, err := wire.ParseFields(payload)
	if err != nil {
		return "", nil, err
	}
	if len(fields) == 0 {
		return "", nil, fmt.Errorf("testhost: empty client message")
	}
	msgID, err := strconv.Atoi(fields[0])
	if err != nil {
		return "", nil, fmt.Errorf("testhost: parse client msg_id %q: %w", fields[0], err)
	}

	switch msgID {
	case 71: // OutStartAPI
		body := map[string]any{}
		if len(fields) >= 3 {
			body["client_id"] = fields[2]
		}
		if len(fields) >= 4 {
			body["optional_capabilities"] = fields[3]
		}
		return "start_api", body, nil
	case 9: // OutReqContractData: [9, 8, reqId, conId, symbol, secType, ...]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		if len(fields) >= 8 {
			body["contract"] = map[string]any{
				"symbol":           fields[4],
				"sec_type":         fields[5],
				"exchange":         safeField(fields, 10),
				"currency":         safeField(fields, 12),
				"primary_exchange": safeField(fields, 11),
				"local_symbol":     safeField(fields, 13),
			}
		}
		return "req_contract_details", body, nil
	case 20: // OutReqHistoricalData: [20, reqId, conId, symbol, secType, ..., endDateTime, barSize, duration, useRTH, whatToShow, ...]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["req_id"] = fields[1]
		}
		if len(fields) >= 23 {
			body["contract"] = map[string]any{
				"symbol":           fields[3],
				"sec_type":         fields[4],
				"exchange":         safeField(fields, 9),
				"currency":         safeField(fields, 11),
				"primary_exchange": safeField(fields, 10),
				"local_symbol":     safeField(fields, 12),
			}
			body["end_time"] = fields[15]
			body["bar_size"] = fields[16]
			body["duration"] = fields[17]
			body["use_rth"] = fields[18] == "1"
			body["what_to_show"] = fields[19]
		}
		return "req_historical_bars", body, nil
	case 62: // OutReqAccountSummary: [62, 1, reqId, group, tags_csv]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		if len(fields) >= 4 {
			body["account"] = fields[3]
		}
		if len(fields) >= 5 {
			tags := []any{}
			if fields[4] != "" {
				for _, t := range strings.Split(fields[4], ",") {
					tags = append(tags, t)
				}
			}
			body["tags"] = tags
		}
		return "req_account_summary", body, nil
	case 63: // OutCancelAccountSummary: [63, 1, reqId]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		return "cancel_account_summary", body, nil
	case 61: // OutReqPositions: [61, 1]
		return "req_positions", map[string]any{}, nil
	case 64: // OutCancelPositions: [64, 1]
		return "cancel_positions", map[string]any{}, nil
	case 1: // OutReqMktData: [1, 11, reqId, conId, contract(11), deltaNeutral, genericTicks, snapshot, regSnapshot, opts]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		if len(fields) >= 20 {
			body["contract"] = map[string]any{
				"symbol":           fields[4],
				"sec_type":         fields[5],
				"exchange":         safeField(fields, 10),
				"currency":         safeField(fields, 12),
				"primary_exchange": safeField(fields, 11),
				"local_symbol":     safeField(fields, 13),
			}
			body["snapshot"] = fields[17] == "1"
			genericTicks := []any{}
			if fields[16] != "" {
				for _, t := range strings.Split(fields[16], ",") {
					genericTicks = append(genericTicks, t)
				}
			}
			body["generic_ticks"] = genericTicks
		}
		return "req_quote", body, nil
	case 2: // OutCancelMktData: [2, 1, reqId]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		return "cancel_quote", body, nil
	case 50: // OutReqRealTimeBars: [50, 3, reqId, conId, symbol, secType, ...]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		if len(fields) >= 19 {
			body["contract"] = map[string]any{
				"symbol":           fields[4],
				"sec_type":         fields[5],
				"exchange":         safeField(fields, 10),
				"currency":         safeField(fields, 12),
				"primary_exchange": safeField(fields, 11),
				"local_symbol":     safeField(fields, 13),
			}
			body["what_to_show"] = fields[16]
			body["use_rth"] = fields[17] == "1"
		}
		return "req_realtime_bars", body, nil
	case 51: // OutCancelRealTimeBars: [51, 1, reqId]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		return "cancel_realtime_bars", body, nil
	case 5: // OutReqOpenOrders: [5, 1]
		return "req_open_orders", map[string]any{"scope": "client"}, nil
	case 16: // OutReqAllOpenOrders: [16, 1]
		return "req_open_orders", map[string]any{"scope": "all"}, nil
	case 15: // OutReqAutoOpenOrders: [15, 1, bind] — bind=1 means subscribe, bind=0 means cancel
		if len(fields) >= 3 && fields[2] == "0" {
			return "cancel_open_orders", map[string]any{}, nil
		}
		return "req_open_orders", map[string]any{"scope": "auto"}, nil
	case 7: // OutReqExecutions: [7, 3, reqId, clientId, acct, time, symbol, secType, exchange, side]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		if len(fields) >= 5 {
			body["account"] = fields[4]
		}
		if len(fields) >= 7 {
			body["symbol"] = fields[6]
		}
		return "req_executions", body, nil
	case 59: // OutReqMarketDataType: [59, 1, dataType]
		body := map[string]any{}
		if len(fields) >= 3 {
			dt, _ := strconv.Atoi(fields[2])
			body["data_type"] = float64(dt)
		}
		return "req_market_data_type", body, nil
	case 25: // OutCancelHistoricalData: [25, 1, reqId]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		return "cancel_historical_data", body, nil
	case 80: // OutReqFamilyCodes: [80]
		return "req_family_codes", map[string]any{}, nil
	case 82: // OutReqMktDepthExchanges: [82]
		return "req_mkt_depth_exchanges", map[string]any{}, nil
	case 85: // OutReqNewsProviders: [85]
		return "req_news_providers", map[string]any{}, nil
	case 24: // OutReqScannerParameters: [24, 1]
		return "req_scanner_parameters", map[string]any{}, nil
	case 81: // OutReqMatchingSymbols: [81, reqId, pattern]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["req_id"] = fields[1]
		}
		if len(fields) >= 3 {
			body["pattern"] = fields[2]
		}
		return "req_matching_symbols", body, nil
	case 87: // OutReqHeadTimestamp: [87, reqId, conId, contract(...), includeExpired, useRTH, whatToShow, formatDate]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["req_id"] = fields[1]
		}
		return "req_head_timestamp", body, nil
	case 90: // OutCancelHeadTimestamp: [90, reqId]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["req_id"] = fields[1]
		}
		return "cancel_head_timestamp", body, nil
	case 91: // OutReqMarketRule: [91, marketRuleId]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["market_rule_id"] = fields[1]
		}
		return "req_market_rule", body, nil
	case 99: // OutReqCompletedOrders: [99, apiOnly]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["api_only"] = fields[1] == "1"
		}
		return "req_completed_orders", body, nil
	case 104: // OutReqUserInfo: [104, 1, reqId]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		return "req_user_info", body, nil
	case 6: // OutReqAccountUpdates: [6, 2, subscribe, acctCode]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["subscribe"] = fields[2] == "1"
		}
		if len(fields) >= 4 {
			body["account"] = fields[3]
		}
		return "req_account_updates", body, nil
	case 76: // OutReqAccountUpdatesMulti: [76, 1, reqId, account, modelCode, subscribe=1]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		if len(fields) >= 4 {
			body["account"] = fields[3]
		}
		if len(fields) >= 5 {
			body["model_code"] = fields[4]
		}
		return "req_account_updates_multi", body, nil
	case 77: // OutCancelAccountUpdatesMulti: [77, 1, reqId]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		return "cancel_account_updates_multi", body, nil
	case 74: // OutReqPositionsMulti: [74, 1, reqId, account, modelCode]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		if len(fields) >= 4 {
			body["account"] = fields[3]
		}
		if len(fields) >= 5 {
			body["model_code"] = fields[4]
		}
		return "req_positions_multi", body, nil
	case 75: // OutCancelPositionsMulti: [75, 1, reqId]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		return "cancel_positions_multi", body, nil
	case 92: // OutReqPnL: [92, reqId, account, modelCode]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["req_id"] = fields[1]
		}
		if len(fields) >= 3 {
			body["account"] = fields[2]
		}
		if len(fields) >= 4 {
			body["model_code"] = fields[3]
		}
		return "req_pnl", body, nil
	case 93: // OutCancelPnL: [93, reqId]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["req_id"] = fields[1]
		}
		return "cancel_pnl", body, nil
	case 94: // OutReqPnLSingle: [94, reqId, account, modelCode, conId]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["req_id"] = fields[1]
		}
		if len(fields) >= 3 {
			body["account"] = fields[2]
		}
		if len(fields) >= 4 {
			body["model_code"] = fields[3]
		}
		if len(fields) >= 5 {
			body["con_id"] = fields[4]
		}
		return "req_pnl_single", body, nil
	case 95: // OutCancelPnLSingle: [95, reqId]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["req_id"] = fields[1]
		}
		return "cancel_pnl_single", body, nil
	case 97: // OutReqTickByTickData: [97, reqId, conId, contract(11), tickType, numberOfTicks, ignoreSize]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["req_id"] = fields[1]
		}
		return "req_tick_by_tick", body, nil
	case 98: // OutCancelTickByTickData: [98, reqId]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["req_id"] = fields[1]
		}
		return "cancel_tick_by_tick", body, nil
	case 22: // OutReqScannerSubscription: [22, reqId, ...]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["req_id"] = fields[1]
		}
		return "req_scanner_subscription", body, nil
	case 23: // OutCancelScannerSubscription: [23, 1, reqId]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		return "cancel_scanner_subscription", body, nil
	case 54: // OutReqCalcImpliedVolatility: [54, 3, reqId, ...]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		return "req_calc_implied_volatility", body, nil
	case 55: // OutReqCalcOptionPrice: [55, 3, reqId, ...]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		return "req_calc_option_price", body, nil
	case 56: // OutCancelCalcImpliedVolatility: [56, 1, reqId]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		return "cancel_calc_implied_volatility", body, nil
	case 57: // OutCancelCalcOptionPrice: [57, 1, reqId]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		return "cancel_calc_option_price", body, nil
	case 78: // OutReqSecDefOptParams: [78, reqId, underlyingSymbol, futFopExchange, underlyingSecType, underlyingConId]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["req_id"] = fields[1]
		}
		return "req_sec_def_opt_params", body, nil
	case 83: // OutReqSmartComponents: [83, reqId, bboExchange]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["req_id"] = fields[1]
		}
		if len(fields) >= 3 {
			body["bbo_exchange"] = fields[2]
		}
		return "req_smart_components", body, nil
	case 84: // OutReqNewsArticle: [84, reqId, providerCode, articleId, options]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["req_id"] = fields[1]
		}
		return "req_news_article", body, nil
	case 86: // OutReqHistoricalNews: [86, reqId, conId, providerCodes, startDate, endDate, totalResults, options]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["req_id"] = fields[1]
		}
		return "req_historical_news", body, nil
	case 88: // OutReqHistogramData: [88, reqId, ...]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["req_id"] = fields[1]
		}
		return "req_histogram_data", body, nil
	case 89: // OutCancelHistogramData: [89, reqId]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["req_id"] = fields[1]
		}
		return "cancel_histogram_data", body, nil
	case 96: // OutReqHistoricalTicks: [96, reqId, ...]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["req_id"] = fields[1]
		}
		return "req_historical_ticks", body, nil
	case 12: // OutReqNewsBulletins: [12, 1, allMessages]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["all_messages"] = fields[2] == "1"
		}
		return "req_news_bulletins", body, nil
	case 13: // OutCancelNewsBulletins: [13, 1]
		return "cancel_news_bulletins", map[string]any{}, nil
	default:
		return "", nil, fmt.Errorf("testhost: unsupported client msg_id %d", msgID)
	}
}

func safeField(fields []string, idx int) string {
	if idx < len(fields) {
		return fields[idx]
	}
	return ""
}

func matchValue(expected, actual any, bindings map[string]any) error {
	switch exp := expected.(type) {
	case string:
		if strings.HasPrefix(exp, "$") {
			if got, ok := bindings[exp]; ok {
				if fmt.Sprint(got) != fmt.Sprint(actual) {
					return fmt.Errorf("binding %s = %v, got %v", exp, got, actual)
				}
				return nil
			}
			bindings[exp] = actual
			return nil
		}
		if exp != fmt.Sprint(actual) {
			return fmt.Errorf("value = %v, want %v", actual, exp)
		}
		return nil
	case float64:
		if exp != actual {
			return fmt.Errorf("value = %v, want %v", actual, exp)
		}
		return nil
	case bool:
		if exp != actual {
			return fmt.Errorf("value = %v, want %v", actual, exp)
		}
		return nil
	case []any:
		act, ok := actual.([]any)
		if !ok {
			return fmt.Errorf("value type = %T, want array", actual)
		}
		if len(exp) != len(act) {
			return fmt.Errorf("array len = %d, want %d", len(act), len(exp))
		}
		for i := range exp {
			if err := matchValue(exp[i], act[i], bindings); err != nil {
				return err
			}
		}
		return nil
	case map[string]any:
		act, ok := actual.(map[string]any)
		if !ok {
			return fmt.Errorf("value type = %T, want object", actual)
		}
		for key, value := range exp {
			if err := matchValue(value, act[key], bindings); err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported expected type %T", expected)
	}
}

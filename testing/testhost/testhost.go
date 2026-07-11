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
	"github.com/ThomasMarcelis/ibkr-go/internal/protocol"
	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
	"google.golang.org/protobuf/encoding/protowire"
)

// defaultServerVersion is the fallback layout for legacy symbolic transcripts
// that omit a handshake version.
const defaultServerVersion = 200

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
	// #nosec G304 -- test callers explicitly select the replay transcript.
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
	defer func() {
		if conn != nil {
			_ = conn.Close()
		}
	}()
	serverVersion := defaultServerVersion

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
			prefix, err := readExact(conn, 4)
			if err != nil {
				h.finish(fmt.Errorf("testhost: handshake: read prefix: %w", err))
				return
			}
			if !bytes.Equal(prefix, []byte("API\x00")) {
				h.finish(fmt.Errorf("testhost: handshake: prefix = %x, want API\\x00", prefix))
				return
			}
			versionPayload, err := wire.ReadFrame(conn)
			if err != nil {
				h.finish(fmt.Errorf("testhost: handshake: read version: %w", err))
				return
			}
			_ = versionPayload
			if raw, ok := cur.body["server_version"]; ok {
				serverVersion = asInt(resolveBindings(raw, bindings))
			}
			connTime := asString(resolveBindings(cur.body["connection_time"], bindings))
			serverInfoPayload := wire.EncodeFields([]string{strconv.Itoa(serverVersion), connTime})
			if err := wire.WriteFrame(conn, serverInfoPayload); err != nil {
				h.finish(fmt.Errorf("testhost: handshake: write server info: %w", err))
				return
			}
			startPayload, err := wire.ReadFrame(conn)
			if err != nil {
				h.finish(fmt.Errorf("testhost: handshake: read start_api: %w", err))
				return
			}
			startEnvelope, err := protocol.DecodeEnvelope(serverVersion, startPayload)
			if err != nil {
				h.finish(fmt.Errorf("testhost: handshake: decode start_api envelope: %w", err))
				return
			}
			if startEnvelope.MsgID != protocol.OutStartAPI || startEnvelope.Encoding != protocol.ClassicBody {
				h.finish(fmt.Errorf("testhost: handshake: start_api envelope = {msg_id:%d encoding:%d}, want classic msg_id %d", startEnvelope.MsgID, startEnvelope.Encoding, protocol.OutStartAPI))
				return
			}
			startFields, err := parseClassicEnvelopeBody(startEnvelope.Body)
			if err != nil {
				h.finish(fmt.Errorf("testhost: handshake: parse start_api: %w", err))
				return
			}
			if cid, ok := cur.body["client_id"]; ok {
				if s, ok := cid.(string); ok && strings.HasPrefix(s, "$") {
					if len(startFields) >= 2 {
						bindings[s] = startFields[1]
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
			name, body, err := decodeClientMessageAt(serverVersion, payload)
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
				if serverVersion != defaultServerVersion {
					h.finish(fmt.Errorf("testhost: transcripts below server_version %d must use raw server frames; DSL historical_bar re-encodes through the codec under test and would mask version-gated layout bugs", defaultServerVersion))
					return
				}
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
				i += len(bars) - 1
				continue
			}

			if serverVersion != defaultServerVersion {
				h.finish(fmt.Errorf("testhost: transcripts below server_version %d must use raw server frames; DSL-form frames re-encode through the codec under test and would mask version-gated layout bugs", defaultServerVersion))
				return
			}
			msg, err := buildMessage(cur.name, cur.body, bindings)
			if err != nil {
				h.finish(err)
				return
			}
			payload, err := codec.Encode(serverVersion, msg)
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
			if cur.direction == "server" && serverVersion != defaultServerVersion {
				h.finish(fmt.Errorf("testhost: transcripts below server_version %d must use raw server frames (split step)", defaultServerVersion))
				return
			}
			payload, err := codec.Encode(serverVersion, msg)
			if err != nil {
				h.finish(err)
				return
			}
			frame := appendLengthPrefix(payload)
			switch cur.direction {
			case "server":
				if err := writeChunked(conn, frame, cur.sizes); err != nil {
					h.finish(err)
					return
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
		case "raw", "splitraw":
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
				var err error
				if cur.kind == "splitraw" {
					err = writeChunked(conn, cur.raw, cur.sizes)
				} else {
					_, err = conn.Write(cur.raw)
				}
				if err != nil {
					h.finish(err)
					return
				}
			case "client":
				var got []byte
				var err error
				if cur.kind == "splitraw" {
					got, err = readChunked(conn, len(cur.raw), cur.sizes)
				} else {
					got, err = readExact(conn, len(cur.raw))
				}
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
		case strings.HasPrefix(line, "splitraw "):
			parts := strings.SplitN(line, " ", 4)
			if len(parts) != 4 {
				return nil, fmt.Errorf("line %d: invalid splitraw step", idx+1)
			}
			raw, err := base64.StdEncoding.DecodeString(parts[3])
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", idx+1, err)
			}
			steps = append(steps, step{kind: "splitraw", direction: parts[1], sizes: parseSizes(parts[2]), raw: raw})
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
	var frame bytes.Buffer
	if err := wire.WriteFrame(&frame, payload); err != nil {
		panic(fmt.Sprintf("testhost: frame server payload: %v", err))
	}
	return frame.Bytes()
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

func writeChunked(w io.Writer, frame []byte, sizes []int) error {
	cursor := 0
	for _, size := range sizes {
		if cursor >= len(frame) {
			break
		}
		end := min(cursor+size, len(frame))
		if _, err := w.Write(frame[cursor:end]); err != nil {
			return err
		}
		cursor = end
	}
	if cursor < len(frame) {
		_, err := w.Write(frame[cursor:])
		return err
	}
	return nil
}

func decodeClientMessage(payload []byte) (string, map[string]any, error) {
	return decodeClientMessageAt(defaultServerVersion, payload)
}

func decodeClientMessageAt(serverVersion int, payload []byte) (string, map[string]any, error) {
	envelope, err := protocol.DecodeEnvelope(serverVersion, payload)
	if err != nil {
		return "", nil, err
	}
	if envelope.Encoding == protocol.ProtobufBody {
		if envelope.MsgID != protocol.OutReqExecutions {
			return "", nil, fmt.Errorf("testhost: protobuf client msg_id %d is not supported", envelope.MsgID)
		}
		body, err := decodeProtoExecutionsRequest(envelope.Body)
		return "req_executions", body, err
	}
	fields, err := parseClassicEnvelopeBody(envelope.Body)
	if err != nil {
		return "", nil, err
	}
	fields = append([]string{strconv.Itoa(envelope.MsgID)}, fields...)
	if len(fields) == 0 {
		return "", nil, fmt.Errorf("testhost: empty client message")
	}
	msgID := envelope.MsgID

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
	case 49: // OutReqCurrentTime: [49, 1]
		return "req_current_time", map[string]any{}, nil
	case 105: // OutReqCurrentTimeInMillis: [105] — bare msg id
		return "req_current_time_millis", map[string]any{}, nil
	case 8: // OutReqIds: [8, 1, numIds]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["num_ids"] = fields[2]
		}
		return "req_ids", body, nil
	case 9: // OutReqContractData: [9, 8, reqId, conId, symbol, secType, ...]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		if len(fields) >= 8 {
			body["contract"] = map[string]any{
				"con_id":           fields[3],
				"symbol":           fields[4],
				"sec_type":         fields[5],
				"exchange":         safeField(fields, 10),
				"currency":         safeField(fields, 12),
				"primary_exchange": safeField(fields, 11),
				"local_symbol":     safeField(fields, 13),
				"issuer_id":        safeField(fields, 18),
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
		if len(fields) >= 2 {
			body["version"] = fields[1]
		}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		if len(fields) >= 19 {
			body["contract"] = map[string]any{
				"con_id":           fields[3],
				"symbol":           fields[4],
				"sec_type":         fields[5],
				"exchange":         safeField(fields, 10),
				"currency":         safeField(fields, 12),
				"primary_exchange": safeField(fields, 11),
				"local_symbol":     safeField(fields, 13),
			}
			body["bar_size"] = fields[15]
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
	case 7: // OutReqExecutions: [7, 3, reqId, clientId, acct, time, symbol, secType, exchange, side, lastNDays, datesCount]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		if len(fields) >= 5 {
			body["client_id"] = fields[3]
			body["account"] = fields[4]
		}
		if len(fields) >= 7 {
			body["time"] = fields[5]
			body["symbol"] = fields[6]
		}
		if len(fields) >= 10 {
			body["sec_type"] = fields[7]
			body["exchange"] = fields[8]
			body["side"] = fields[9]
		}
		if len(fields) >= 12 {
			body["last_days"] = fields[10]
			count, _ := strconv.Atoi(fields[11])
			dates := make([]any, 0, count)
			for i := 0; i < count && 12+i < len(fields); i++ {
				dates = append(dates, fields[12+i])
			}
			body["specific_dates"] = dates
		}
		return "req_executions", body, nil
	case 59: // OutReqMarketDataType: [59, 1, dataType]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["version"] = fields[1]
		}
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
	case 76: // OutReqAccountUpdatesMulti: [76, 1, reqId, account, modelCode, ledgerAndNLV]
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
		if len(fields) >= 6 {
			body["ledger_and_nlv"] = fields[5] == "1"
		}
		return "req_account_updates_multi", body, nil
	case 77: // OutCancelAccountUpdatesMulti: [77, 1, reqId]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		return "cancel_account_updates_multi", body, nil
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
		if len(fields) >= 17 {
			body["contract"] = map[string]any{
				"con_id":           fields[2],
				"symbol":           fields[3],
				"sec_type":         fields[4],
				"exchange":         safeField(fields, 9),
				"currency":         safeField(fields, 11),
				"primary_exchange": safeField(fields, 10),
				"local_symbol":     safeField(fields, 12),
			}
			body["tick_type"] = fields[14]
			body["number_of_ticks"] = fields[15]
			body["ignore_size"] = fields[16] == "1"
		}
		return "req_tick_by_tick", body, nil
	case 98: // OutCancelTickByTickData: [98, reqId]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["req_id"] = fields[1]
		}
		return "cancel_tick_by_tick", body, nil
	case 22: // OutReqScannerSubscription: [22, reqId, subscription fields..., filterOptions, options]
		if len(fields) != 25 {
			return "", nil, fmt.Errorf("testhost: req_scanner_subscription field count = %d, want 25", len(fields))
		}
		body := map[string]any{
			"req_id":                      fields[1],
			"number_of_rows":              fields[2],
			"instrument":                  fields[3],
			"location_code":               fields[4],
			"scan_code":                   fields[5],
			"above_price":                 fields[6],
			"below_price":                 fields[7],
			"above_volume":                fields[8],
			"market_cap_above":            fields[9],
			"market_cap_below":            fields[10],
			"moody_rating_above":          fields[11],
			"moody_rating_below":          fields[12],
			"sp_rating_above":             fields[13],
			"sp_rating_below":             fields[14],
			"maturity_date_above":         fields[15],
			"maturity_date_below":         fields[16],
			"coupon_rate_above":           fields[17],
			"coupon_rate_below":           fields[18],
			"exclude_convertible":         fields[19],
			"average_option_volume_above": fields[20],
			"scanner_setting_pairs":       fields[21],
			"stock_type_filter":           fields[22],
			"filter_options":              fields[23],
			"subscription_options":        fields[24],
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
		if len(fields) >= 7 {
			body["con_id"] = fields[2]
			body["provider_codes"] = fields[3]
			body["start_time"] = fields[4]
			body["end_time"] = fields[5]
			body["total_results"] = fields[6]
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
		if len(fields) >= 21 {
			body["contract"] = map[string]any{
				"con_id":   fields[2],
				"symbol":   fields[3],
				"sec_type": fields[4],
				"exchange": fields[9],
				"currency": fields[11],
			}
			body["start_time"] = fields[15]
			body["end_time"] = fields[16]
			body["number_of_ticks"] = fields[17]
			body["what_to_show"] = fields[18]
			body["use_rth"] = fields[19] == "1"
			body["ignore_size"] = fields[20] == "1"
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
	case 3: // OutPlaceOrder: [3, orderID, conID, symbol, secType, expiry, strike,
		// right, multiplier, exchange, primaryExchange, currency, localSymbol,
		// tradingClass, secIdType, secId, action, totalQty, orderType, lmtPrice,
		// auxPrice, tif, ocaGroup, account, ...]
		return "place_order", decodePlaceOrderClientBody(fields), nil
	case 4: // OutCancelOrder: [4, orderID, manualOrderCancelTime, extOperator, manualOrderIndicator]
		body := map[string]any{
			"field_count":              strconv.Itoa(len(fields)),
			"order_id":                 safeField(fields, 1),
			"manual_order_cancel_time": safeField(fields, 2),
			"ext_operator":             safeField(fields, 3),
			"manual_order_indicator":   safeField(fields, 4),
		}
		return "cancel_order", body, nil
	case 58: // OutReqGlobalCancel: [58, extOperator, manualOrderIndicator]
		body := map[string]any{
			"field_count":            strconv.Itoa(len(fields)),
			"ext_operator":           safeField(fields, 1),
			"manual_order_indicator": safeField(fields, 2),
		}
		return "global_cancel", body, nil
	case 10: // OutReqMktDepth: [10, version=5, reqId, conId, symbol, secType, expiry, strike, right, multiplier, exchange, currency, localSymbol, tradingClass, numRows, isSmartDepth, mktDepthOptions]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		if len(fields) >= 14 {
			body["contract"] = map[string]any{
				"symbol":   fields[4],
				"sec_type": fields[5],
				"exchange": safeField(fields, 10),
				"currency": safeField(fields, 11),
			}
		}
		if len(fields) >= 15 {
			body["num_rows"] = fields[14]
		}
		if len(fields) >= 16 {
			body["is_smart_depth"] = fields[15]
		}
		return "req_market_depth", body, nil
	case 11: // OutCancelMktDepth: [11, version=1, reqId, isSmartDepth?]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		if len(fields) >= 4 {
			body["is_smart_depth"] = fields[3]
		}
		return "cancel_market_depth", body, nil
	case 18: // OutRequestFA: [18, version=1, faDataType]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["fa_data_type"] = fields[2]
		}
		return "request_fa", body, nil
	case 21: // OutExerciseOptions: [21, version=2, reqId, conId, symbol, secType, expiry, strike, right, multiplier, exchange, currency, localSymbol, tradingClass, exerciseAction, exerciseQuantity, account, override, manualOrderTime, customerAccount, professionalCustomer]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		if len(fields) >= 14 {
			body["contract"] = map[string]any{
				"con_id":        fields[3],
				"symbol":        fields[4],
				"sec_type":      fields[5],
				"expiry":        fields[6],
				"strike":        fields[7],
				"right":         fields[8],
				"multiplier":    fields[9],
				"exchange":      safeField(fields, 10),
				"currency":      safeField(fields, 11),
				"local_symbol":  safeField(fields, 12),
				"trading_class": safeField(fields, 13),
			}
		}
		if len(fields) >= 15 {
			body["exercise_action"] = fields[14]
		}
		if len(fields) >= 16 {
			body["exercise_quantity"] = fields[15]
		}
		if len(fields) >= 17 {
			body["account"] = fields[16]
		}
		if len(fields) >= 18 {
			body["override"] = fields[17]
		}
		// server_version 200 tail (manual order time, customer account,
		// professional customer), live-attested in the 2026-06-11 exercise
		// captures.
		if len(fields) >= 19 {
			body["manual_order_time"] = fields[18]
		}
		if len(fields) >= 20 {
			body["customer_account"] = fields[19]
		}
		if len(fields) >= 21 {
			body["professional_customer"] = fields[20]
		}
		return "exercise_options", body, nil
	case 67: // OutQueryDisplayGroups: [67, version=1, reqId]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		return "query_display_groups", body, nil
	case 68: // OutSubscribeToGroupEvents: [68, version=1, reqId, groupId]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		if len(fields) >= 4 {
			body["group_id"] = fields[3]
		}
		return "subscribe_group_events", body, nil
	case 69: // OutUpdateDisplayGroup: [69, version=1, reqId, contractInfo]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		if len(fields) >= 4 {
			body["contract_info"] = fields[3]
		}
		return "update_display_group", body, nil
	case 70: // OutUnsubscribeFromGroupEvents: [70, version=1, reqId]
		body := map[string]any{}
		if len(fields) >= 3 {
			body["req_id"] = fields[2]
		}
		return "unsubscribe_group_events", body, nil
	case 79: // OutReqSoftDollarTiers: [79, reqId]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["req_id"] = fields[1]
		}
		return "req_soft_dollar_tiers", body, nil
	case 100: // OutReqWSHMetaData: [100, reqId]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["req_id"] = fields[1]
		}
		return "req_wsh_meta_data", body, nil
	case 101: // OutCancelWSHMetaData: [101, reqId]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["req_id"] = fields[1]
		}
		return "cancel_wsh_meta_data", body, nil
	case 102: // OutReqWSHEventData: [102, reqId, conId, filter, fillWatchlist, fillPortfolio, fillCompetitors, startDate, endDate, totalLimit]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["req_id"] = fields[1]
		}
		if len(fields) >= 3 {
			body["con_id"] = fields[2]
		}
		if len(fields) >= 4 {
			body["filter"] = fields[3]
		}
		if len(fields) >= 5 {
			body["fill_watchlist"] = fields[4]
		}
		if len(fields) >= 6 {
			body["fill_portfolio"] = fields[5]
		}
		if len(fields) >= 7 {
			body["fill_competitors"] = fields[6]
		}
		if len(fields) >= 8 {
			body["start_date"] = fields[7]
		}
		if len(fields) >= 9 {
			body["end_date"] = fields[8]
		}
		if len(fields) >= 10 {
			body["total_limit"] = fields[9]
		}
		return "req_wsh_event_data", body, nil
	case 103: // OutCancelWSHEventData: [103, reqId]
		body := map[string]any{}
		if len(fields) >= 2 {
			body["req_id"] = fields[1]
		}
		return "cancel_wsh_event_data", body, nil
	default:
		return "", nil, fmt.Errorf("testhost: unsupported client msg_id %d", msgID)
	}
}

func parseClassicEnvelopeBody(body []byte) ([]string, error) {
	if len(body) == 0 {
		return nil, nil
	}
	if body[len(body)-1] != 0 {
		return nil, wire.ErrMalformedFrame
	}
	return strings.Split(string(body[:len(body)-1]), "\x00"), nil
}

func decodeProtoExecutionsRequest(payload []byte) (map[string]any, error) {
	body := map[string]any{}
	for len(payload) > 0 {
		number, typ, n := protowire.ConsumeTag(payload)
		if n < 0 {
			return nil, fmt.Errorf("testhost: executions protobuf tag: %w", protowire.ParseError(n))
		}
		payload = payload[n:]
		switch number {
		case 1:
			value, n := protowire.ConsumeVarint(payload)
			if typ != protowire.VarintType || n < 0 {
				return nil, fmt.Errorf("testhost: executions protobuf req_id is malformed")
			}
			payload = payload[n:]
			body["req_id"] = strconv.Itoa(decodeProtoInt32(value))
		case 2:
			filter, n := protowire.ConsumeBytes(payload)
			if typ != protowire.BytesType || n < 0 {
				return nil, fmt.Errorf("testhost: executions protobuf filter is malformed")
			}
			payload = payload[n:]
			if err := decodeProtoExecutionFilter(filter, body); err != nil {
				return nil, err
			}
		default:
			n := protowire.ConsumeFieldValue(number, typ, payload)
			if n < 0 {
				return nil, fmt.Errorf("testhost: executions protobuf field %d: %w", number, protowire.ParseError(n))
			}
			payload = payload[n:]
		}
	}
	return body, nil
}

func decodeProtoExecutionFilter(payload []byte, body map[string]any) error {
	for len(payload) > 0 {
		number, typ, n := protowire.ConsumeTag(payload)
		if n < 0 {
			return fmt.Errorf("testhost: execution filter protobuf tag: %w", protowire.ParseError(n))
		}
		payload = payload[n:]
		switch number {
		case 1, 8:
			value, n := protowire.ConsumeVarint(payload)
			if typ != protowire.VarintType || n < 0 {
				return fmt.Errorf("testhost: execution filter protobuf field %d is malformed", number)
			}
			payload = payload[n:]
			key := "client_id"
			if number == 8 {
				key = "last_days"
			}
			body[key] = strconv.Itoa(decodeProtoInt32(value))
		case 2, 3, 4, 5, 6, 7:
			value, n := protowire.ConsumeBytes(payload)
			if typ != protowire.BytesType || n < 0 {
				return fmt.Errorf("testhost: execution filter protobuf field %d is malformed", number)
			}
			payload = payload[n:]
			key := map[protowire.Number]string{2: "account", 3: "time", 4: "symbol", 5: "sec_type", 6: "exchange", 7: "side"}[number]
			body[key] = string(value)
		case 9:
			if typ == protowire.VarintType {
				value, n := protowire.ConsumeVarint(payload)
				if n < 0 {
					return fmt.Errorf("testhost: execution filter protobuf specific_dates is malformed")
				}
				payload = payload[n:]
				body["specific_dates"] = appendAny(body["specific_dates"], strconv.Itoa(decodeProtoInt32(value)))
				continue
			}
			packed, n := protowire.ConsumeBytes(payload)
			if typ != protowire.BytesType || n < 0 {
				return fmt.Errorf("testhost: execution filter protobuf specific_dates is malformed")
			}
			payload = payload[n:]
			for len(packed) > 0 {
				value, n := protowire.ConsumeVarint(packed)
				if n < 0 {
					return fmt.Errorf("testhost: execution filter protobuf specific_dates value is malformed")
				}
				packed = packed[n:]
				body["specific_dates"] = appendAny(body["specific_dates"], strconv.Itoa(decodeProtoInt32(value)))
			}
		default:
			n := protowire.ConsumeFieldValue(number, typ, payload)
			if n < 0 {
				return fmt.Errorf("testhost: execution filter protobuf field %d: %w", number, protowire.ParseError(n))
			}
			payload = payload[n:]
		}
	}
	return nil
}

func appendAny(current any, value any) []any {
	values, _ := current.([]any)
	return append(values, value)
}

func decodeProtoInt32(value uint64) int {
	return int(int32(value)) // #nosec G115 -- protobuf int32 wire semantics
}

func decodePlaceOrderClientBody(fields []string) map[string]any {
	body := map[string]any{}
	if len(fields) >= 2 {
		body["order_id"] = fields[1]
	}
	if len(fields) < 24 {
		return body
	}

	secType := safeField(fields, 4)
	body["contract"] = map[string]any{
		"con_id":           fields[2],
		"symbol":           fields[3],
		"sec_type":         secType,
		"expiry":           safeField(fields, 5),
		"strike":           safeField(fields, 6),
		"right":            safeField(fields, 7),
		"multiplier":       safeField(fields, 8),
		"exchange":         safeField(fields, 9),
		"primary_exchange": safeField(fields, 10),
		"currency":         safeField(fields, 11),
		"local_symbol":     safeField(fields, 12),
		"trading_class":    safeField(fields, 13),
	}
	body["action"] = fields[16]
	body["total_quantity"] = fields[17]
	body["order_type"] = fields[18]
	body["lmt_price"] = fields[19]
	body["aux_price"] = fields[20]
	body["tif"] = fields[21]
	body["oca_group"] = fields[22]
	body["account"] = fields[23]
	body["open_close"] = safeField(fields, 24)
	body["origin"] = safeField(fields, 25)
	body["order_ref"] = safeField(fields, 26)
	body["transmit"] = safeField(fields, 27) == "1"
	body["parent_id"] = safeField(fields, 28)
	body["display_size"] = safeField(fields, 31)
	body["trigger_method"] = safeField(fields, 32)
	body["outside_rth"] = safeField(fields, 33) == "1"
	body["hidden"] = safeField(fields, 34) == "1"

	cursor := 35
	if secType == "BAG" {
		body["combo_legs"] = decodeComboLegClientFields(fields, &cursor)
		body["order_combo_leg_prices"] = decodeStringListClientFields(fields, &cursor)
		body["smart_combo_routing_params"] = decodeTagValueClientFields(fields, &cursor)
	}

	_ = readClientField(fields, &cursor) // deprecated sharesAllocation
	body["discretionary_amt"] = readClientField(fields, &cursor)
	body["good_after_time"] = readClientField(fields, &cursor)
	body["good_till_date"] = readClientField(fields, &cursor)
	body["fa_group"] = readClientField(fields, &cursor)
	body["fa_method"] = readClientField(fields, &cursor)
	body["fa_percentage"] = readClientField(fields, &cursor)
	body["model_code"] = readClientField(fields, &cursor)
	body["short_sale_slot"] = readClientField(fields, &cursor)
	body["designated_location"] = readClientField(fields, &cursor)
	body["exempt_code"] = readClientField(fields, &cursor)
	body["oca_type"] = readClientField(fields, &cursor)
	body["rule80a"] = readClientField(fields, &cursor)
	body["settling_firm"] = readClientField(fields, &cursor)
	body["all_or_none"] = readClientField(fields, &cursor)
	body["min_qty"] = readClientField(fields, &cursor)
	body["percent_offset"] = readClientField(fields, &cursor)
	cursor += 3 // deprecated eTradeOnly, firmQuoteOnly, nbboPriceCap
	body["auction_strategy"] = readClientField(fields, &cursor)
	body["starting_price"] = readClientField(fields, &cursor)
	body["stock_ref_price"] = readClientField(fields, &cursor)
	body["delta"] = readClientField(fields, &cursor)
	body["stock_range_lower"] = readClientField(fields, &cursor)
	body["stock_range_upper"] = readClientField(fields, &cursor)
	body["override_percentage_constraints"] = readClientField(fields, &cursor)
	body["volatility"] = readClientField(fields, &cursor)
	body["volatility_type"] = readClientField(fields, &cursor)
	body["delta_neutral_order_type"] = readClientField(fields, &cursor)
	body["delta_neutral_aux_price"] = readClientField(fields, &cursor)
	body["continuous_update"] = readClientField(fields, &cursor)
	body["reference_price_type"] = readClientField(fields, &cursor)
	body["trail_stop_price"] = readClientField(fields, &cursor)
	body["trailing_percent"] = readClientField(fields, &cursor)
	body["scale_init_level_size"] = readClientField(fields, &cursor)
	body["scale_subs_level_size"] = readClientField(fields, &cursor)
	body["scale_price_increment"] = readClientField(fields, &cursor)
	body["scale_table"] = readClientField(fields, &cursor)
	body["active_start_time"] = readClientField(fields, &cursor)
	body["active_stop_time"] = readClientField(fields, &cursor)
	body["hedge_type"] = readClientField(fields, &cursor)
	if body["hedge_type"] != "" {
		body["hedge_param"] = readClientField(fields, &cursor)
	}
	body["opt_out_smart_routing"] = readClientField(fields, &cursor)
	body["clearing_account"] = readClientField(fields, &cursor)
	body["clearing_intent"] = readClientField(fields, &cursor)
	body["not_held"] = readClientField(fields, &cursor)
	body["delta_neutral_contract_present"] = readClientField(fields, &cursor)
	body["algo_strategy"] = readClientField(fields, &cursor)
	if body["algo_strategy"] != "" {
		body["algo_params"] = decodeTagValueClientFields(fields, &cursor)
	}
	body["algo_id"] = readClientField(fields, &cursor)
	body["what_if"] = readClientField(fields, &cursor) == "1"
	body["order_misc_options"] = readClientField(fields, &cursor)
	body["solicited"] = readClientField(fields, &cursor)
	body["randomize_size"] = readClientField(fields, &cursor)
	body["randomize_price"] = readClientField(fields, &cursor)
	body["conditions"] = decodeOrderConditionClientFields(fields, &cursor)
	if conditions, ok := body["conditions"].([]any); ok && len(conditions) > 0 {
		body["conditions_ignore_rth"] = readClientField(fields, &cursor) == "1"
		body["conditions_cancel_order"] = readClientField(fields, &cursor) == "1"
	}
	body["adjusted_order_type"] = readClientField(fields, &cursor)
	body["trigger_price"] = readClientField(fields, &cursor)
	body["lmt_price_offset"] = readClientField(fields, &cursor)
	body["adjusted_stop_price"] = readClientField(fields, &cursor)
	body["adjusted_stop_limit_price"] = readClientField(fields, &cursor)
	body["adjusted_trailing_amount"] = readClientField(fields, &cursor)
	body["adjustable_trailing_unit"] = readClientField(fields, &cursor)
	body["ext_operator"] = readClientField(fields, &cursor)
	body["soft_dollar_name"] = readClientField(fields, &cursor)
	body["soft_dollar_value"] = readClientField(fields, &cursor)
	body["cash_qty"] = readClientField(fields, &cursor)
	body["mifid2_decision_maker"] = readClientField(fields, &cursor)
	body["mifid2_decision_algo"] = readClientField(fields, &cursor)
	body["mifid2_execution_trader"] = readClientField(fields, &cursor)
	body["mifid2_execution_algo"] = readClientField(fields, &cursor)
	body["dont_use_auto_price_for_hedge"] = readClientField(fields, &cursor)
	body["is_oms_container"] = readClientField(fields, &cursor)
	body["discretionary_up_to_limit_price"] = readClientField(fields, &cursor)
	body["use_price_mgmt_algo"] = readClientField(fields, &cursor)
	body["duration"] = readClientField(fields, &cursor)
	body["post_to_ats"] = readClientField(fields, &cursor)
	body["auto_cancel_parent"] = readClientField(fields, &cursor)
	body["advanced_error_override"] = readClientField(fields, &cursor)
	body["manual_order_time"] = readClientField(fields, &cursor)
	body["customer_account"] = readClientField(fields, &cursor)
	body["professional_customer"] = readClientField(fields, &cursor)
	body["include_overnight"] = readClientField(fields, &cursor)
	body["manual_order_indicator"] = readClientField(fields, &cursor)
	body["imbalance_only"] = readClientField(fields, &cursor)
	return body
}

func decodeComboLegClientFields(fields []string, cursor *int) []any {
	count := readClientCount(fields, cursor)
	legs := make([]any, 0, count)
	for range count {
		legs = append(legs, map[string]any{
			"con_id":              readClientField(fields, cursor),
			"ratio":               readClientField(fields, cursor),
			"action":              readClientField(fields, cursor),
			"exchange":            readClientField(fields, cursor),
			"open_close":          readClientField(fields, cursor),
			"short_sale_slot":     readClientField(fields, cursor),
			"designated_location": readClientField(fields, cursor),
			"exempt_code":         readClientField(fields, cursor),
		})
	}
	return legs
}

func decodeStringListClientFields(fields []string, cursor *int) []any {
	count := readClientCount(fields, cursor)
	values := make([]any, 0, count)
	for range count {
		values = append(values, readClientField(fields, cursor))
	}
	return values
}

func decodeTagValueClientFields(fields []string, cursor *int) []any {
	count := readClientCount(fields, cursor)
	values := make([]any, 0, count)
	for range count {
		values = append(values, map[string]any{
			"tag":   readClientField(fields, cursor),
			"value": readClientField(fields, cursor),
		})
	}
	return values
}

func decodeOrderConditionClientFields(fields []string, cursor *int) []any {
	count := readClientCount(fields, cursor)
	conditions := make([]any, 0, count)
	for range count {
		conditionType := readClientField(fields, cursor)
		condition := map[string]any{
			"type":        conditionType,
			"conjunction": readClientField(fields, cursor),
		}
		switch conditionType {
		case "1":
			condition["operator"] = readClientBool(fields, cursor)
			condition["value"] = readClientField(fields, cursor)
			condition["con_id"] = readClientField(fields, cursor)
			condition["exchange"] = readClientField(fields, cursor)
			condition["trigger_method"] = readClientField(fields, cursor)
		case "3", "4":
			condition["operator"] = readClientBool(fields, cursor)
			condition["value"] = readClientField(fields, cursor)
		case "5":
			condition["sec_type"] = readClientField(fields, cursor)
			condition["exchange"] = readClientField(fields, cursor)
			condition["symbol"] = readClientField(fields, cursor)
		case "6", "7":
			condition["operator"] = readClientBool(fields, cursor)
			condition["value"] = readClientField(fields, cursor)
			condition["con_id"] = readClientField(fields, cursor)
			condition["exchange"] = readClientField(fields, cursor)
		}
		conditions = append(conditions, condition)
	}
	return conditions
}

func readClientField(fields []string, cursor *int) string {
	value := safeField(fields, *cursor)
	*cursor = *cursor + 1
	return value
}

func readClientCount(fields []string, cursor *int) int {
	value, _ := strconv.Atoi(readClientField(fields, cursor))
	return value
}

func readClientBool(fields []string, cursor *int) bool {
	return readClientField(fields, cursor) == "1"
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

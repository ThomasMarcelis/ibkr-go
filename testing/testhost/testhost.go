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

	for _, step := range h.steps {
		switch step.kind {
		case "sleep":
			time.Sleep(step.duration)
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
			serverVersion := asInt(resolveBindings(step.body["server_version"], bindings))
			connTime := asString(resolveBindings(step.body["connection_time"], bindings))
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
			if len(startFields) < 1 || startFields[0] != "71" {
				h.finish(fmt.Errorf("testhost: handshake: start_api msg_id = %v, want 71", startFields[0]))
				return
			}
			// Store client_id in bindings if body requests it
			if cid, ok := step.body["client_id"]; ok {
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
			msg, err := codec.DecodeSymbolic(payload)
			if err != nil {
				h.finish(err)
				return
			}
			body, err := messageBody(msg)
			if err != nil {
				h.finish(err)
				return
			}
			if messageName(msg) != step.name {
				h.finish(fmt.Errorf("testhost: client message = %q, want %q", messageName(msg), step.name))
				return
			}
			if err := matchValue(step.body, body, bindings); err != nil {
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
			msg, err := buildMessage(step.name, step.body, bindings)
			if err != nil {
				h.finish(err)
				return
			}
			payload, err := codec.EncodeWire(msg)
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
			msg, err := buildMessage(step.name, step.body, bindings)
			if err != nil {
				h.finish(err)
				return
			}
			var payload []byte
			if step.direction == "server" {
				payload, err = codec.EncodeWire(msg)
			} else {
				payload, err = codec.Encode(msg)
			}
			if err != nil {
				h.finish(err)
				return
			}
			frame := appendLengthPrefix(payload)
			switch step.direction {
			case "server":
				cursor := 0
				for _, size := range step.sizes {
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
				got, err := readChunked(conn, len(frame), step.sizes)
				if err != nil {
					h.finish(err)
					return
				}
				if !bytes.Equal(got, frame) {
					h.finish(fmt.Errorf("testhost: split client frame = %x, want %x", got, frame))
					return
				}
			default:
				h.finish(fmt.Errorf("testhost: unsupported split direction %q", step.direction))
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
			switch step.direction {
			case "server":
				if _, err := conn.Write(step.raw); err != nil {
					h.finish(err)
					return
				}
			case "client":
				got, err := readExact(conn, len(step.raw))
				if err != nil {
					h.finish(err)
					return
				}
				if !bytes.Equal(got, step.raw) {
					h.finish(fmt.Errorf("testhost: raw client bytes = %x, want %x", got, step.raw))
					return
				}
			default:
				h.finish(fmt.Errorf("testhost: unsupported raw direction %q", step.direction))
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

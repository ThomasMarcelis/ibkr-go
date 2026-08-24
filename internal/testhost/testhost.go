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

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
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

	var conn net.Conn
	defer func() {
		if conn != nil {
			_ = conn.Close()
		}
	}()
	serverVersion := 0

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
			raw, ok := cur.body["server_version"]
			if !ok {
				h.finish(fmt.Errorf("testhost: handshake omits server_version"))
				return
			}
			serverVersion = asInt(raw)
			if serverVersion < protocol.SupportedMinServerVersion || serverVersion > protocol.SupportedMaxServerVersion {
				h.finish(fmt.Errorf("testhost: unsupported server_version %d", serverVersion))
				return
			}
			connTime := asString(cur.body["connection_time"])
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
			if startEnvelope.MsgID != protocol.OutStartAPI {
				h.finish(fmt.Errorf("testhost: handshake: start_api msg_id = %d, want %d", startEnvelope.MsgID, protocol.OutStartAPI))
				return
			}
		case "disconnect":
			if conn != nil {
				_ = conn.Close()
				conn = nil
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
			return nil, fmt.Errorf("line %d: symbolic split steps are unsupported; use splitraw with captured bytes", idx+1)
		default:
			return nil, fmt.Errorf("line %d: symbolic steps are unsupported; use captured raw frames", idx+1)
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

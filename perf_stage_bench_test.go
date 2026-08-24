package ibkr

// Performance benchmarks for the inbound hot path:
//
//	frame -> transport readLoop -> codec.DecodeBatch -> chan any
//	      -> engine.handleIncoming -> route.handle -> Subscription.emit
//
// Two classes:
//
//   - Actor-stage benches drive engine.handleIncoming directly against a real
//     production route (installed via the actual subscribe setup closure), so
//     route.handle is the exact shipped code path with no scheduler noise.
//   - BenchmarkE2EQuoteStreamTCP runs the whole pipeline over real TCP
//     loopback: DialContext -> SubscribeQuotes -> N live tick frames -> count.
//     It is the only bench that observes syscall-level read cost, so it is the
//     one that moves when the transport read seam changes.
//
// Inputs are live IB Gateway server_version 225 frames captured in
// 20260824T202345Z-api_duplicate_quote_subscriptions_aapl, events SHA-256
// 1fbb60beec41483729e2f9e7c96b1bfdd89649810ffdc5e7e4a4077c1eb8b290.
// Exact length-prefixed frames are decoded from the transcript literals below
// so the benchmark runs independently of the ignored raw-capture tree.

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/transport"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
)

// benchTickFrames holds exact live sv225 protobuf tick frames:
//
//	[0] tick_price field 68 (delayed last)
//	[1] tick_price field 66 (delayed bid)
//	[2] tick_size  field 74 (delayed volume)
//
// A tiny distinct set replayed in a loop stands in for a long stream: the
// pipeline cost is per-frame, so looping three frames measures the same thing
// as storing thousands.
var benchTickFrames = [][]byte{
	mustDecodeBenchFrame("AAAAGAAAAMkIARBEGc3MzMzMaHNAIgMxNDEoAA=="),
	mustDecodeBenchFrame("AAAAGAAAAMkIARBCGWZmZmZmZnNAIgM4NDAoAA=="),
	mustDecodeBenchFrame("AAAAEAAAAMoIARBKGgY4MTIyNTQ="),
}

func mustDecodeBenchFrame(encoded string) []byte {
	framed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		panic("perf bench: decode captured frame: " + err.Error())
	}
	payload, err := wire.ReadFrame(bytes.NewReader(framed))
	if err != nil {
		panic("perf bench: parse captured frame: " + err.Error())
	}
	return payload
}

var (
	frameTickPriceLast = benchTickFrames[0]
	frameTickPriceBid  = benchTickFrames[1]
	frameTickSizeVol   = benchTickFrames[2]
)

var benchContract = Contract{Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD"}

// newBenchEngine builds an engine in StateReady with a live transport over a
// loopback TCP pair whose peer discards everything. No actor goroutine runs:
// benchmarks drive handleIncoming directly (single-goroutine, same as the
// production actor).
func newBenchEngine(tb testing.TB) *engine {
	tb.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatal(err)
	}
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		accepted <- conn
	}()
	clientConn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		tb.Fatal(err)
	}
	peer := <-accepted
	go func() { _, _ = io.Copy(io.Discard, peer) }()
	tb.Cleanup(func() {
		_ = clientConn.Close()
		_ = peer.Close()
		_ = ln.Close()
	})

	cfg := defaultConfig()
	cfg.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	e := &engine{
		cfg:                      cfg,
		cmds:                     make(chan func(), 256),
		incoming:                 make(chan any, 256),
		transportErr:             make(chan transportLoss, 8),
		ready:                    make(chan error, 1),
		done:                     make(chan struct{}),
		events:                   newObserver[Event](cfg.eventBuffer),
		keyed:                    make(map[int]*route),
		singletons:               make(map[string]*route),
		orders:                   make(map[int64]*orderRoute),
		execDeliveries:           make(map[string]*execDelivery),
		malformedInboundSeen:     make(map[int]struct{}),
		unknownInboundSeen:       make(map[int]struct{}),
		recentHistoricalRequests: make(map[string]time.Time),
		nextReqID:                1,
		serverVersion:            225,
		snapshot:                 Snapshot{State: StateReady},
	}
	e.transport = transport.New(clientConn, cfg.logger, 0)
	return e
}

// installQuoteRoute registers a real production quote route by running the
// subscribeQuotes setup closure synchronously (the closure the actor would
// run), so route.handle is the exact shipped code path.
func installQuoteRoute(tb testing.TB, e *engine) *Subscription[QuoteUpdate] {
	tb.Helper()
	type result struct {
		sub *Subscription[QuoteUpdate]
		err error
	}
	res := make(chan result, 1)
	go func() {
		sub, err := e.subscribeQuotes(context.Background(), QuoteRequest{Contract: benchContract}, false, false)
		res <- result{sub, err}
	}()
	fn := <-e.cmds
	fn()
	r := <-res
	if r.err != nil {
		tb.Fatalf("subscribeQuotes: %v", r.err)
	}
	return r.sub
}

func decodeOne(tb testing.TB, payload []byte) codec.Message {
	tb.Helper()
	msgs, err := codec.DecodeBatch(225, payload)
	if err != nil {
		tb.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		tb.Fatalf("got %d messages, want 1", len(msgs))
	}
	return msgs[0]
}

// --- actor-side stage: handleIncoming with a live quote route ---

func benchActorTick(b *testing.B) {
	e := newBenchEngine(b)
	sub := installQuoteRoute(b, e)
	go func() {
		for range sub.Events() {
		}
	}()
	msgLast := decodeOne(b, frameTickPriceLast)
	msgVol := decodeOne(b, frameTickSizeVol)

	e.handleIncoming(msgLast)
	select {
	case <-sub.Events():
	default:
	}

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		if i%4 == 3 {
			e.handleIncoming(msgVol)
		} else {
			e.handleIncoming(msgLast)
		}
	}
}

// BenchmarkActorHandleTickQuote_Drained: actor cost with a spinning consumer.
func BenchmarkActorHandleTickQuote_Drained(b *testing.B) { benchActorTick(b) }

// --- full pipeline over real TCP loopback ---

// handshakeFrames are the sanitized sv225 bootstrap frames from the same
// capture as benchTickFrames.
var (
	frameServerInfo      = []byte("225\x0020260824 22:23:45 CET\x00")
	frameManagedAccounts = mustDecodeBenchFrame("AAAADwAAANcKCURVOTAwMDAwMQ==")
	frameNextValidID     = mustDecodeBenchFrame("AAAABgAAANEIAQ==")
)

// benchTCPServer speaks the minimal real handshake, then blasts pre-encoded
// frames in 64 KiB writes (the gateway coalesces frames the same way; see the
// multi-frame chunks in the quote_stream capture).
func benchTCPServer(tb testing.TB, stream []byte, nConns int) string {
	tb.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = ln.Close() })
	go func() {
		for range nConns {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				prefix := make([]byte, 4) // "API\x00"
				if _, err := io.ReadFull(conn, prefix); err != nil {
					return
				}
				if _, err := wire.ReadFrame(conn); err != nil { // v208..225
					return
				}
				if err := wire.WriteFrame(conn, frameServerInfo); err != nil {
					return
				}
				if _, err := wire.ReadFrame(conn); err != nil { // startAPI
					return
				}
				if err := wire.WriteFrame(conn, frameManagedAccounts); err != nil {
					return
				}
				if err := wire.WriteFrame(conn, frameNextValidID); err != nil {
					return
				}
				if _, err := wire.ReadFrame(conn); err != nil { // reqMktData
					return
				}
				const chunk = 64 << 10
				for off := 0; off < len(stream); off += chunk {
					end := min(off+chunk, len(stream))
					if _, err := conn.Write(stream[off:end]); err != nil {
						return
					}
				}
				// Hold the conn open until the client is done counting.
				_, _ = io.Copy(io.Discard, conn)
			}(conn)
		}
	}()
	return ln.Addr().String()
}

// BenchmarkE2EQuoteStreamTCP: DialContext -> SubscribeQuotes -> N live tick
// frames over loopback -> count N QuoteUpdates. The complete production
// inbound path: transport readLoop, decode pump, actor, route, emit. This is
// the bench that sees the buffered-read seam in internal/transport.
func BenchmarkE2EQuoteStreamTCP(b *testing.B) {
	const nMsgs = 100_000
	var stream []byte
	frames := [][]byte{frameTickPriceLast, frameTickPriceBid, frameTickPriceLast, frameTickSizeVol}
	for i := range nMsgs {
		f := frames[i%len(frames)]
		var frame bytes.Buffer
		if err := wire.WriteFrame(&frame, f); err != nil {
			b.Fatalf("frame benchmark tick: %v", err)
		}
		stream = append(stream, frame.Bytes()...)
	}

	// ns/op includes dial+subscribe setup; the reported msgs/sec and ns/msg
	// metrics cover only the streaming window (first to last update). Best
	// iteration wins: external machine load only ever slows the pipeline.
	bestRate := 0.0
	b.ReportAllocs()
	for b.Loop() {
		addr := benchTCPServer(b, stream, 1)
		host, port, _ := net.SplitHostPort(addr)
		portN, _ := net.LookupPort("tcp", port)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		client, err := DialContext(ctx, WithHost(host), WithPort(portN), WithReconnectPolicy(ReconnectOff))
		if err != nil {
			cancel()
			b.Fatal(err)
		}
		sub, err := client.MarketData().SubscribeQuotes(ctx, QuoteRequest{Contract: benchContract},
			WithQueueSize(nMsgs+16))
		if err != nil {
			cancel()
			b.Fatal(err)
		}

		start := time.Now()
		got := 0
		for range sub.Events() {
			got++
			if got == nMsgs {
				break
			}
		}
		elapsed := time.Since(start)
		if got != nMsgs {
			b.Fatalf("stream ended early: got %d updates, want %d (err=%v)", got, nMsgs, sub.Err())
		}
		if rate := float64(nMsgs) / elapsed.Seconds(); rate > bestRate {
			bestRate = rate
		}
		sub.Close()
		client.Close()
		cancel()
	}
	b.ReportMetric(bestRate, "msgs/sec")
	b.ReportMetric(1e9/bestRate, "ns/msg")
}

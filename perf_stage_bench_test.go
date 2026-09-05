package ibkr

// Capture-backed performance benchmarks for the inbound quote path:
//
//	frame -> transport readLoop -> codec.DecodeBatch -> actor
//	      -> keyed route -> quote projection -> subscription channel
//
// Inputs are exact IB Gateway server_version 225 frames from capture
// 20260824T202345Z-api_duplicate_quote_subscriptions_aapl, events.jsonl
// SHA-256 1fbb60beec41483729e2f9e7c96b1bfdd89649810ffdc5e7e4a4077c1eb8b290.
// Repetition and fragmentation below are controlled amplification; their
// cadence and write boundaries are not claims about the source capture.

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/transport"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
	"github.com/shopspring/decimal"
)

// benchTickFrames holds exact live sv225 protobuf tick frames:
//
//	[0] tick_price field 68 (delayed last), price 310.55, companion size 141
//	[1] tick_price field 66 (delayed bid), price 310.4, companion size 840
//	[2] tick_size  field 74 (delayed volume), size 812254
var benchTickFrames = [][]byte{
	mustDecodeBenchFrame("AAAAGAAAAMkIARBEGc3MzMzMaHNAIgMxNDEoAA=="),
	mustDecodeBenchFrame("AAAAGAAAAMkIARBCGWZmZmZmZnNAIgM4NDAoAA=="),
	mustDecodeBenchFrame("AAAAEAAAAMoIARBKGgY4MTIyNTQ="),
}

var (
	frameTickPriceLast = benchTickFrames[0]
	frameTickPriceBid  = benchTickFrames[1]
	frameTickSizeVol   = benchTickFrames[2]
	benchFramedTicks   = [][]byte{
		mustEncodeBenchFrame(frameTickPriceLast),
		mustEncodeBenchFrame(frameTickPriceBid),
		mustEncodeBenchFrame(frameTickPriceLast),
		mustEncodeBenchFrame(frameTickSizeVol),
	}

	benchQuoteRequest = mustDecodeBenchFrame("AAAAIwAAAMkIARIbCP6aEBIEQUFQTBoDU1RLQgVTTUFSVFIDVVNE")
	benchQuoteCancel  = mustDecodeBenchFrame("AAAABgAAAMoIAQ==")

	benchLastPrice = decimal.RequireFromString("310.55")
	benchLastSize  = decimal.RequireFromString("141")
	benchBidPrice  = decimal.RequireFromString("310.4")
	benchBidSize   = decimal.RequireFromString("840")
	benchVolume    = decimal.RequireFromString("812254")
)

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

func mustEncodeBenchFrame(payload []byte) []byte {
	framed, err := wire.EncodeFrame(payload)
	if err != nil {
		panic("perf bench: frame captured payload: " + err.Error())
	}
	return framed
}

var benchContract = Contract{ConID: 265598, Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD"}

// newBenchEngine builds a ready engine with a live transport over a loopback
// TCP pair. No actor goroutine runs: actor-stage benchmarks drive setup and
// handleIncoming synchronously, preserving the production actor's ownership.
func newBenchEngine(tb testing.TB) *engine {
	tb.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatal(err)
	}
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			accepted <- conn
		}
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
		incoming:                 make(chan actorInput, 256),
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

// installQuoteRoute runs the real subscribeQuotes setup closure synchronously,
// exactly as the actor would, so the installed route uses the shipped handler.
func installQuoteRoute(tb testing.TB, e *engine) *Subscription[QuoteUpdate] {
	tb.Helper()
	type result struct {
		sub *Subscription[QuoteUpdate]
		err error
	}
	resultCh := make(chan result, 1)
	go func() {
		sub, err := e.subscribeQuotes(context.Background(), QuoteRequest{Contract: benchContract}, false, false)
		resultCh <- result{sub, err}
	}()
	setup := <-e.cmds
	setup()
	out := <-resultCh
	if out.err != nil {
		tb.Fatalf("subscribeQuotes: %v", out.err)
	}
	return out.sub
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

var benchQuoteSequence = []codec.Message{
	decodeBenchMessage(frameTickPriceLast),
	decodeBenchMessage(frameTickPriceBid),
	decodeBenchMessage(frameTickPriceLast),
	decodeBenchMessage(frameTickSizeVol),
}

func decodeBenchMessage(payload []byte) codec.Message {
	msgs, err := codec.DecodeBatch(225, payload)
	if err != nil || len(msgs) != 1 {
		panic(fmt.Sprintf("perf bench: decode captured tick: messages=%d err=%v", len(msgs), err))
	}
	return msgs[0]
}

func validateBenchQuoteEvent(tb testing.TB, index int, event StreamEvent[QuoteUpdate]) {
	tb.Helper()
	if event.Kind != StreamData {
		tb.Fatalf("event %d kind = %s, want %s", index, event.Kind, StreamData)
	}
	update := event.Value
	switch index % len(benchQuoteSequence) {
	case 0, 2:
		if update.Kind != QuoteUpdatePriceTick || update.PriceTick == nil || update.SizeTick != nil {
			tb.Fatalf("event %d = kind %s price=%+v size=%+v, want price tick", index, update.Kind, update.PriceTick, update.SizeTick)
		}
		if update.PriceTick.TickType != 68 || !update.PriceTick.Price.Equal(benchLastPrice) || update.PriceTick.Size == nil || !update.PriceTick.Size.Equal(benchLastSize) {
			tb.Fatalf("event %d delayed-last tick = %+v", index, update.PriceTick)
		}
		if !update.Snapshot.Last.Equal(benchLastPrice) || !update.Snapshot.LastSize.Equal(benchLastSize) {
			tb.Fatalf("event %d delayed-last snapshot = %+v", index, update.Snapshot)
		}
	case 1:
		if update.Kind != QuoteUpdatePriceTick || update.PriceTick == nil || update.SizeTick != nil {
			tb.Fatalf("event %d = kind %s price=%+v size=%+v, want price tick", index, update.Kind, update.PriceTick, update.SizeTick)
		}
		if update.PriceTick.TickType != 66 || !update.PriceTick.Price.Equal(benchBidPrice) || update.PriceTick.Size == nil || !update.PriceTick.Size.Equal(benchBidSize) {
			tb.Fatalf("event %d delayed-bid tick = %+v", index, update.PriceTick)
		}
		if !update.Snapshot.Bid.Equal(benchBidPrice) || !update.Snapshot.BidSize.Equal(benchBidSize) {
			tb.Fatalf("event %d delayed-bid snapshot = %+v", index, update.Snapshot)
		}
	case 3:
		if update.Kind != QuoteUpdateSizeTick || update.SizeTick == nil || update.PriceTick != nil {
			tb.Fatalf("event %d = kind %s price=%+v size=%+v, want size tick", index, update.Kind, update.PriceTick, update.SizeTick)
		}
		if update.SizeTick.TickType != 74 || update.SizeTick.Size == nil || !update.SizeTick.Size.Equal(benchVolume) {
			tb.Fatalf("event %d delayed-volume tick = %+v", index, update.SizeTick)
		}
		if !update.Snapshot.Volume.Equal(benchVolume) {
			tb.Fatalf("event %d delayed-volume snapshot = %+v", index, update.Snapshot)
		}
	}
}

// BenchmarkActorQuoteDispatchDelivery includes keyed lookup, quote projection,
// event construction, and synchronous delivery through the subscription
// channel. Each timed operation is a 1,024-event batch so even -benchtime=1x
// crosses the historical 64-event queue-failure boundary.
func BenchmarkActorQuoteDispatchDelivery(b *testing.B) {
	const deliveriesPerOp = 1024
	e := newBenchEngine(b)
	sub := installQuoteRoute(b, e)
	started := <-sub.Events()
	if started.Kind != StreamStarted {
		b.Fatalf("first event = %s, want %s", started.Kind, StreamStarted)
	}
	ownedRoute := e.keyed[1]
	if ownedRoute == nil {
		b.Fatal("quote route 1 was not installed")
	}

	// Validate a full captured sequence before timing. Repeating expensive
	// assertions per delivery would measure the test harness in the hot path.
	for i, message := range benchQuoteSequence {
		e.handleIncoming(message)
		validateBenchQuoteEvent(b, i, <-sub.Events())
	}
	var last StreamEvent[QuoteUpdate]
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	produced := 0
	consumed := 0
	for b.Loop() {
		for range deliveriesPerOp {
			message := benchQuoteSequence[produced%len(benchQuoteSequence)]
			e.handleIncoming(message)
			produced++
			event, ok := <-sub.Events()
			if !ok {
				b.Fatalf("subscription closed after %d deliveries: %v", consumed, sub.Err())
			}
			if event.Kind != StreamData {
				b.Fatalf("event %d kind = %s, want Data", consumed, event.Kind)
			}
			last = event
			consumed++
		}
	}
	runtime.ReadMemStats(&after)

	validateBenchQuoteEvent(b, consumed-1, last)
	if produced != consumed || produced < deliveriesPerOp {
		b.Fatalf("produced=%d consumed=%d, want equal and at least %d", produced, consumed, deliveriesPerOp)
	}
	if err := sub.Err(); err != nil {
		b.Fatalf("subscription error after delivery: %v", err)
	}
	if e.keyed[1] != ownedRoute {
		b.Fatal("quote route ownership changed during delivery")
	}
	reportUpdateMetrics(b, uint64(produced), before, after)

	// Use the production route-owned cancellation path outside timing.
	sub.Close()
	cancelRoute := <-e.cmds
	cancelRoute()
	if err := sub.Wait(); err != nil {
		b.Fatalf("quote cancellation: %v", err)
	}
	if _, ok := e.keyed[1]; ok {
		b.Fatal("quote route remained installed after cancellation")
	}
}

// BenchmarkActorKeyedRouteLookupNoop isolates request-ID map dispatch and
// callback ownership validation. Its route intentionally performs no
// projection or channel delivery.
func BenchmarkActorKeyedRouteLookupNoop(b *testing.B) {
	e := newBenchEngine(b)
	message := decodeOne(b, frameTickPriceLast)
	ownedRoute := &route{opKind: OpQuotes, handle: func(any, *engine) {}}
	e.keyed[1] = ownedRoute

	for b.Loop() {
		e.handleIncoming(message)
	}
	if e.keyed[1] != ownedRoute {
		b.Fatal("route ownership changed during lookup benchmark")
	}
}

var (
	frameServerInfo      = []byte("225\x0020260824 22:23:45 CET\x00")
	frameManagedAccounts = mustDecodeBenchFrame("AAAADwAAANcKCURVOTAwMDAwMQ==")
	frameNextValidID     = mustDecodeBenchFrame("AAAABgAAANEIAQ==")
)

type benchWriteShape string

const (
	benchWriteCoalesced benchWriteShape = "coalesced"
	benchWriteFrames    benchWriteShape = "one_frame_per_write"
	benchWriteSplit     benchWriteShape = "header_body_fragmentation"
)

type benchStreamServer struct {
	addr       string
	ready      chan struct{}
	start      chan int
	credit     chan struct{}
	sentAt     chan time.Time
	result     chan error
	creditSize int
	latency    bool
}

func newBenchStreamServer(tb testing.TB, creditSize int, shape benchWriteShape, latency bool) *benchStreamServer {
	tb.Helper()
	if creditSize <= 0 {
		tb.Fatalf("invalid stream credit size %d", creditSize)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatal(err)
	}
	server := &benchStreamServer{
		addr:       ln.Addr().String(),
		ready:      make(chan struct{}),
		start:      make(chan int),
		credit:     make(chan struct{}),
		sentAt:     make(chan time.Time, creditSize),
		result:     make(chan error, 1),
		creditSize: creditSize,
		latency:    latency,
	}
	var coalesced []byte
	if shape == benchWriteCoalesced {
		coalesced = make([]byte, 0, creditSize*32)
		for i := range creditSize {
			coalesced = append(coalesced, benchFramedTicks[i%len(benchFramedTicks)]...)
		}
	}
	go func() {
		server.result <- server.serve(ln, shape, coalesced)
	}()
	return server
}

func (s *benchStreamServer) serve(ln net.Listener, shape benchWriteShape, coalesced []byte) error {
	defer ln.Close()

	conn, err := ln.Accept()
	if err != nil {
		return fmt.Errorf("accept: %w", err)
	}
	defer conn.Close()
	prefix := make([]byte, 4)
	if _, err := io.ReadFull(conn, prefix); err != nil {
		return fmt.Errorf("read handshake prefix: %w", err)
	}
	if !bytes.Equal(prefix, []byte("API\x00")) {
		return fmt.Errorf("handshake prefix = %x, want API\\x00", prefix)
	}
	if _, err := wire.ReadFrame(conn); err != nil {
		return fmt.Errorf("read version range: %w", err)
	}
	if err := wire.WriteFrame(conn, frameServerInfo); err != nil {
		return fmt.Errorf("write server info: %w", err)
	}
	if _, err := wire.ReadFrame(conn); err != nil {
		return fmt.Errorf("read start API: %w", err)
	}
	if err := wire.WriteFrame(conn, frameManagedAccounts); err != nil {
		return fmt.Errorf("write managed accounts: %w", err)
	}
	if err := wire.WriteFrame(conn, frameNextValidID); err != nil {
		return fmt.Errorf("write next valid ID: %w", err)
	}
	request, err := wire.ReadFrame(conn)
	if err != nil {
		return fmt.Errorf("read quote request: %w", err)
	}
	if !bytes.Equal(request, benchQuoteRequest) {
		return fmt.Errorf("quote request = %x, want captured %x", request, benchQuoteRequest)
	}
	close(s.ready)

	for messageCount := range s.start {
		if messageCount <= 0 {
			return fmt.Errorf("invalid stream workload size %d", messageCount)
		}
		for first := 0; first < messageCount; first += s.creditSize {
			count := min(s.creditSize, messageCount-first)
			if err := writeBenchFrames(conn, first, count, shape, s.latency, s.sentAt, coalesced, s.creditSize); err != nil {
				return fmt.Errorf("write messages %d..%d: %w", first, first+count, err)
			}
			<-s.credit
		}
	}

	cancel, err := wire.ReadFrame(conn)
	if err != nil {
		return fmt.Errorf("read quote cancel: %w", err)
	}
	if !bytes.Equal(cancel, benchQuoteCancel) {
		return fmt.Errorf("quote cancel = %x, want captured %x", cancel, benchQuoteCancel)
	}
	return nil
}

func writeBenchFrames(conn net.Conn, first, count int, shape benchWriteShape, latency bool, sentAt chan<- time.Time, coalesced []byte, coalescedCount int) error {
	switch shape {
	case benchWriteCoalesced:
		if count == coalescedCount {
			return writeBenchBytes(conn, coalesced)
		}
		for i := range count {
			frame := benchFramedTicks[(first+i)%len(benchFramedTicks)]
			if err := writeBenchBytes(conn, frame); err != nil {
				return err
			}
		}
		return nil
	case benchWriteFrames:
		for i := range count {
			if latency {
				sentAt <- time.Now()
			}
			frame := benchFramedTicks[(first+i)%len(benchFramedTicks)]
			if err := writeBenchBytes(conn, frame); err != nil {
				return err
			}
		}
		return nil
	case benchWriteSplit:
		for i := range count {
			if latency {
				sentAt <- time.Now()
			}
			frame := benchFramedTicks[(first+i)%len(benchFramedTicks)]
			if err := writeBenchBytes(conn, frame[:4]); err != nil {
				return err
			}
			if err := writeBenchBytes(conn, frame[4:]); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown write shape %q", shape)
	}
}

func writeBenchBytes(conn net.Conn, data []byte) error {
	for len(data) != 0 {
		n, err := conn.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

func waitBenchReady(tb testing.TB, server *benchStreamServer) {
	tb.Helper()
	select {
	case <-server.ready:
		return
	case err := <-server.result:
		tb.Fatalf("benchmark server before readiness: %v", err)
	case <-time.After(30 * time.Second):
		tb.Fatal("timed out waiting for benchmark server readiness")
	}
}

func waitBenchResult(tb testing.TB, server *benchStreamServer) {
	tb.Helper()
	select {
	case err := <-server.result:
		if err != nil {
			tb.Fatalf("benchmark server: %v", err)
		}
	case <-time.After(30 * time.Second):
		tb.Fatal("timed out waiting for benchmark server teardown")
	}
}

type benchStreamClient struct {
	client *Client
	sub    *Subscription[QuoteUpdate]
	cancel context.CancelFunc
}

func dialBenchStream(tb testing.TB, server *benchStreamServer, queueSize int) benchStreamClient {
	tb.Helper()
	host, portText, err := net.SplitHostPort(server.addr)
	if err != nil {
		tb.Fatal(err)
	}
	port, err := net.LookupPort("tcp", portText)
	if err != nil {
		tb.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	client, err := DialContext(ctx, WithHost(host), WithPort(port), WithReconnectPolicy(ReconnectOff))
	if err != nil {
		cancel()
		tb.Fatal(err)
	}
	var opts []SubscriptionOption
	if queueSize != 0 {
		opts = append(opts, WithQueueSize(queueSize))
	}
	sub, err := client.MarketData().SubscribeQuotes(ctx, QuoteRequest{Contract: benchContract}, opts...)
	if err != nil {
		client.Close()
		cancel()
		tb.Fatal(err)
	}
	waitBenchReady(tb, server)
	started, ok := <-sub.Events()
	if !ok || started.Kind != StreamStarted {
		tb.Fatalf("first quote event = %+v ok=%t, want Started", started, ok)
	}
	select {
	case event := <-sub.Events():
		tb.Fatalf("data was queued before start barrier: %+v", event)
	default:
	}
	return benchStreamClient{client: client, sub: sub, cancel: cancel}
}

func closeBenchStream(tb testing.TB, stream benchStreamClient, server *benchStreamServer) {
	tb.Helper()
	close(server.start)
	stream.sub.Close()
	if err := stream.sub.Wait(); err != nil {
		tb.Fatalf("quote cancellation: %v", err)
	}
	waitBenchResult(tb, server)
	stream.client.Close()
	stream.cancel()
}

func preflightBenchStream(tb testing.TB, server *benchStreamServer, stream benchStreamClient) {
	tb.Helper()
	server.start <- len(benchQuoteSequence)
	for i := range benchQuoteSequence {
		event, ok := <-stream.sub.Events()
		if !ok {
			tb.Fatalf("preflight stream ended at update %d: %v", i, stream.sub.Err())
		}
		if server.latency {
			<-server.sentAt
		}
		validateBenchQuoteEvent(tb, i, event)
		if (i+1)%server.creditSize == 0 || i+1 == len(benchQuoteSequence) {
			server.credit <- struct{}{}
		}
	}
}

func reportUpdateMetrics(b *testing.B, updates uint64, before, after runtime.MemStats) {
	if updates == 0 {
		return
	}
	elapsed := b.Elapsed()
	b.ReportMetric(0, "ns/op")
	b.ReportMetric(float64(elapsed.Nanoseconds())/1e3/float64(updates), "us/update")
	b.ReportMetric(float64(updates)/elapsed.Seconds(), "updates/s")
	b.ReportMetric(float64(after.TotalAlloc-before.TotalAlloc)/float64(updates), "B/update")
	b.ReportMetric(float64(after.Mallocs-before.Mallocs)/float64(updates), "allocs/update")
}

func runBenchSteadyStream(b *testing.B, messagesPerOp, creditSize, queueSize int, shape benchWriteShape) {
	if messagesPerOp%len(benchQuoteSequence) != 0 {
		b.Fatalf("stream workload %d is not a whole captured sequence", messagesPerOp)
	}
	server := newBenchStreamServer(b, creditSize, shape, false)
	stream := dialBenchStream(b, server, queueSize)
	preflightBenchStream(b, server, stream)

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	total := 0
	var last StreamEvent[QuoteUpdate]
	for b.Loop() {
		server.start <- messagesPerOp
		for i := range messagesPerOp {
			event, ok := <-stream.sub.Events()
			if !ok {
				b.Fatalf("stream ended at message %d: %v", i, stream.sub.Err())
			}
			last = event
			total++
			if (i+1)%creditSize == 0 || i+1 == messagesPerOp {
				server.credit <- struct{}{}
			}
		}
	}
	runtime.ReadMemStats(&after)
	if want := b.N * messagesPerOp; total != want {
		b.Fatalf("received %d messages, want %d", total, want)
	}
	if err := stream.sub.Err(); err != nil {
		b.Fatalf("subscription error after stream: %v", err)
	}
	validateBenchQuoteEvent(b, total-1, last)
	reportUpdateMetrics(b, uint64(total), before, after)
	closeBenchStream(b, stream, server)
}

// BenchmarkPublicQuoteStream measures the public client path over loopback TCP.
// The steady-state and fragmentation operations contain exactly 100,000
// capture-backed quote updates; the diagnostic cases stay out of README claims.
func BenchmarkPublicQuoteStream(b *testing.B) {
	b.Run("steady_state/default_queue", func(b *testing.B) {
		runBenchSteadyStream(b, 100_000, 32, 0, benchWriteCoalesced)
	})
	b.Run("delivery_latency/ack_each_frame", benchmarkQuoteDeliveryLatency)
	b.Run("startup/dial_to_first_update", benchmarkQuoteDialToFirstUpdate)
	b.Run("diagnostic/burst_queue_4096", func(b *testing.B) {
		runBenchSteadyStream(b, 4096, 4096, 4096, benchWriteCoalesced)
		b.ReportMetric(4096, "queue_events")
	})
	b.Run("diagnostic/one_frame_per_write", func(b *testing.B) {
		runBenchSteadyStream(b, 100_000, 32, 0, benchWriteFrames)
	})
	b.Run("diagnostic/header_body_fragmentation", func(b *testing.B) {
		runBenchSteadyStream(b, 100_000, 32, 0, benchWriteSplit)
	})
}

func benchmarkQuoteDeliveryLatency(b *testing.B) {
	const messagesPerOp = 10_000
	server := newBenchStreamServer(b, 1, benchWriteFrames, true)
	stream := dialBenchStream(b, server, 0)
	preflightBenchStream(b, server, stream)
	latencies := make([]time.Duration, 0, messagesPerOp)
	var last StreamEvent[QuoteUpdate]

	for b.Loop() {
		server.start <- messagesPerOp
		for i := range messagesPerOp {
			event, ok := <-stream.sub.Events()
			if !ok {
				b.Fatalf("latency stream ended at message %d: %v", i, stream.sub.Err())
			}
			last = event
			latencies = append(latencies, time.Since(<-server.sentAt))
			server.credit <- struct{}{}
		}
	}
	if len(latencies) < messagesPerOp {
		b.Fatalf("collected %d latency samples, want at least %d", len(latencies), messagesPerOp)
	}
	validateBenchQuoteEvent(b, len(latencies)-1, last)
	slices.Sort(latencies)
	b.ReportMetric(0, "ns/op")
	b.ReportMetric(float64(latencies[len(latencies)*50/100].Nanoseconds())/1e3, "p50-us")
	b.ReportMetric(float64(latencies[len(latencies)*95/100].Nanoseconds())/1e3, "p95-us")
	b.ReportMetric(float64(latencies[len(latencies)*99/100].Nanoseconds())/1e3, "p99-us")
	closeBenchStream(b, stream, server)
}

func benchmarkQuoteDialToFirstUpdate(b *testing.B) {
	for b.Loop() {
		b.StopTimer()
		server := newBenchStreamServer(b, 1, benchWriteCoalesced, false)
		b.StartTimer()
		stream := dialBenchStream(b, server, 0)
		server.start <- 1
		event, ok := <-stream.sub.Events()
		if !ok {
			b.Fatalf("startup stream ended before first update: %v", stream.sub.Err())
		}
		b.StopTimer()
		validateBenchQuoteEvent(b, 0, event)
		server.credit <- struct{}{}
		closeBenchStream(b, stream, server)
		b.StartTimer()
	}
	b.ReportMetric(0, "ns/op")
	b.ReportMetric(float64(b.Elapsed())/float64(b.N)/float64(time.Millisecond), "ms/session")
}

package ibkr

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/transport"
)

func TestResumeRoutesDrainBeyondTransportQueueInRequestOrder(t *testing.T) {
	t.Parallel()

	peer, client := net.Pipe()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	e := &engine{
		cfg:            defaultConfig(),
		cmds:           make(chan func(), 256),
		incoming:       make(chan any, 1),
		transportErr:   make(chan transportLoss, 1),
		done:           make(chan struct{}),
		events:         newObserver[Event](1),
		transport:      transport.New(client, logger, 0),
		serverVersion:  200,
		keyed:          make(map[int]*route),
		singletons:     make(map[string]*route),
		orders:         make(map[int64]*orderRoute),
		execDeliveries: make(map[string]*execDelivery),
		snapshot:       Snapshot{State: StateReady, ConnectionSeq: 2},
	}
	go e.run()
	t.Cleanup(func() {
		e.Close()
		select {
		case <-e.Done():
		case <-time.After(time.Second):
			t.Fatal("engine did not close")
		}
		_ = peer.Close()
	})

	const routeCount = 300
	resumed := make(chan int, routeCount)
	for reqID := routeCount; reqID >= 1; reqID-- {
		id := reqID
		e.keyed[id] = &route{
			opKind:       OpQuotes,
			subscription: true,
			resume:       ResumeAuto,
			request: codec.QuoteRequest{
				ReqID: id,
				Contract: codec.Contract{
					Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD",
				},
			},
			emitResumed: func(*engine) { resumed <- id },
			close:       func(error) {},
			gapped:      true,
		}
	}

	started := make(chan struct{})
	e.enqueue(func() {
		e.resumeRoutes()
		close(started)
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("resumeRoutes did not return after local queue saturation")
	}

	backpressure := make(chan bool, 1)
	e.enqueue(func() { backpressure <- e.resumeWaiting && len(e.resumePending) > 0 })
	if !<-backpressure {
		t.Fatal("300 resumes did not exercise the bounded transport queue")
	}

	frames := make(chan []int, 1)
	go func() {
		ids := make([]int, 0, routeCount)
		for range routeCount {
			payload, err := transport.ReadOneFrame(peer, time.Now().Add(5*time.Second))
			if err != nil {
				frames <- nil
				return
			}
			fields := bytes.Split(payload, []byte{0})
			if len(fields) < 3 {
				frames <- nil
				return
			}
			id, err := strconv.Atoi(string(fields[2]))
			if err != nil {
				frames <- nil
				return
			}
			ids = append(ids, id)
		}
		frames <- ids
	}()

	for want := 1; want <= routeCount; want++ {
		select {
		case got := <-resumed:
			if got != want {
				t.Fatalf("Resumed order[%d] = %d", want-1, got)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for Resumed %d", want)
		}
	}
	select {
	case ids := <-frames:
		if len(ids) != routeCount {
			t.Fatalf("wire frames = %d, want %d", len(ids), routeCount)
		}
		for i, id := range ids {
			if id != i+1 {
				t.Fatalf("wire request order[%d] = %d, want %d", i, id, i+1)
			}
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout draining resumed frames")
	}

	// A final admitted request proves the transport remains usable after the
	// resume burst rather than merely retaining the routes locally.
	if err := e.transport.Send(context.Background(), []byte("fence")); err != nil {
		t.Fatalf("post-resume Send: %v", err)
	}
}

func TestResumeTransportFailureRetainsRouteForNextReconnect(t *testing.T) {
	t.Parallel()

	peer, client := net.Pipe()
	tr := transport.New(client, nil, 0)
	if err := peer.Close(); err != nil {
		t.Fatalf("close peer: %v", err)
	}
	<-tr.Done()

	closed := make(chan error, 1)
	resumed := make(chan struct{}, 1)
	r := &route{
		opKind:       OpQuotes,
		subscription: true,
		resume:       ResumeAuto,
		request: codec.QuoteRequest{
			ReqID: 1,
			Contract: codec.Contract{
				Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD",
			},
		},
		emitResumed: func(*engine) { resumed <- struct{}{} },
		close:       func(err error) { closed <- err },
		gapped:      true,
	}
	e := &engine{
		transport:     tr,
		serverVersion: 200,
		keyed:         map[int]*route{1: r},
		singletons:    make(map[string]*route),
		orders:        make(map[int64]*orderRoute),
	}

	e.resumeRoutes()

	if e.keyed[1] != r {
		t.Fatal("resume transport failure removed the subscription route")
	}
	if len(e.resumePending) != 1 || e.resumePending[0].route != r {
		t.Fatalf("resumePending = %+v, want retained route", e.resumePending)
	}
	if !r.gapped {
		t.Fatal("resume transport failure cleared the route gap")
	}
	select {
	case err := <-closed:
		t.Fatalf("resume transport failure closed route: %v", err)
	default:
	}
	select {
	case <-resumed:
		t.Fatal("resume transport failure emitted Resumed")
	default:
	}
}

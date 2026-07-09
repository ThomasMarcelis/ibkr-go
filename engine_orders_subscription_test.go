package ibkr

import (
	"context"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/transport"
	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

// TestAutoOpenOrdersCloseDisablesBinding freezes the request lifecycle already
// represented by codec.CancelOpenOrders: reqAutoOpenOrders(true) persistently
// binds future manual orders, so closing the subscription must pair it with
// reqAutoOpenOrders(false). The net.Pipe observes only client output; no fake
// Gateway response or transcript is involved.
func TestAutoOpenOrdersCloseDisablesBinding(t *testing.T) {
	t.Parallel()

	serverConn, clientConn := net.Pipe()
	tr := transport.New(clientConn, nil, 0)
	cfg := defaultConfig()
	cfg.clientID = 0
	e := &engine{
		cfg:            cfg,
		cmds:           make(chan func(), 8),
		incoming:       make(chan any, 8),
		transportErr:   make(chan transportLoss, 1),
		done:           make(chan struct{}),
		events:         newObserver[Event](8),
		transport:      tr,
		serverVersion:  200,
		keyed:          make(map[int]*route),
		singletons:     make(map[string]*route),
		orders:         make(map[int64]*orderRoute),
		executions:     newExecutionCorrelator(),
		execDeliveries: make(map[string]*execDelivery),
		snapshot:       Snapshot{State: StateReady, ConnectionSeq: 1},
	}
	go e.run()
	t.Cleanup(func() {
		_ = e.Close()
		_ = e.Wait()
		_ = serverConn.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sub, err := e.SubscribeOpenOrders(ctx, OpenOrdersScopeAuto)
	if err != nil {
		t.Fatalf("SubscribeOpenOrders(auto): %v", err)
	}

	if got := readWireFields(t, serverConn); !slices.Equal(got, []string{"15", "1", "1"}) {
		t.Fatalf("auto-open bind fields = %q, want [15 1 1]", got)
	}

	if err := sub.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}
	if got := readWireFields(t, serverConn); !slices.Equal(got, []string{"15", "1", "0"}) {
		t.Fatalf("auto-open unbind fields = %q, want [15 1 0]", got)
	}
	if err := sub.Wait(); err != nil {
		t.Fatalf("Wait(): %v", err)
	}
}

func readWireFields(t *testing.T, conn net.Conn) []string {
	t.Helper()
	payload, err := transport.ReadOneFrame(conn, time.Now().Add(time.Second))
	if err != nil {
		t.Fatalf("ReadOneFrame(): %v", err)
	}
	fields, err := wire.ParseFields(payload)
	if err != nil {
		t.Fatalf("ParseFields(): %v", err)
	}
	return fields
}

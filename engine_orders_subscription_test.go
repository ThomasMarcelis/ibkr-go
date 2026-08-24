package ibkr

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/transport"
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
		serverVersion:  225,
		keyed:          make(map[int]*route),
		singletons:     make(map[string]*route),
		orders:         make(map[int64]*orderRoute),
		execDeliveries: make(map[string]*execDelivery),
		snapshot:       Snapshot{State: StateReady, ConnectionSeq: 1},
	}
	go e.run()
	t.Cleanup(func() {
		e.Close()
		_ = e.Wait()
		_ = serverConn.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sub, err := e.SubscribeOpenOrders(ctx, OpenOrdersScopeAuto)
	if err != nil {
		t.Fatalf("SubscribeOpenOrders(auto): %v", err)
	}

	wantBind, err := codec.Encode(225, codec.OpenOrdersRequest{Scope: "auto"})
	if err != nil {
		t.Fatalf("encode auto-open bind: %v", err)
	}
	if got := readWirePayload(t, serverConn); !bytes.Equal(got, wantBind) {
		t.Fatalf("auto-open bind = %x, want %x", got, wantBind)
	}

	// This is an official-schema routing law, not a claim of raw live evidence:
	// only the client-0 auto-open scope owns orderBound callbacks.
	e.enqueue(func() {
		e.handleIncoming(codec.OrderBound{PermID: 123456789, ClientID: 0, OrderID: 42})
	})
	select {
	case event := <-sub.Events():
		if event.Kind != StreamData {
			event = <-sub.Events()
		}
		update := event.Value
		if update.Binding == nil || *update.Binding != (OrderBinding{PermID: 123456789, ClientID: 0, OrderID: 42}) {
			t.Fatalf("binding update = %+v", update.Binding)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for orderBound routing")
	}

	sub.Close()
	wantUnbind, err := codec.Encode(225, codec.CancelOpenOrders{})
	if err != nil {
		t.Fatalf("encode auto-open unbind: %v", err)
	}
	if got := readWirePayload(t, serverConn); !bytes.Equal(got, wantUnbind) {
		t.Fatalf("auto-open unbind = %x, want %x", got, wantUnbind)
	}
	if err := sub.Wait(); err != nil {
		t.Fatalf("Wait(): %v", err)
	}
}

func readWirePayload(t *testing.T, conn net.Conn) []byte {
	t.Helper()
	payload, err := transport.ReadOneFrame(conn, time.Now().Add(time.Second))
	if err != nil {
		t.Fatalf("ReadOneFrame(): %v", err)
	}
	return payload
}

package ibkr_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
)

type blockingLogHandler struct {
	entered chan struct{}
	release chan struct{}
	once    *sync.Once
}

func (h blockingLogHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h blockingLogHandler) Handle(context.Context, slog.Record) error {
	h.once.Do(func() { close(h.entered) })
	<-h.release
	return nil
}

func (h blockingLogHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h blockingLogHandler) WithGroup(string) slog.Handler      { return h }

func TestBlockingLoggerDoesNotBlockClientClose(t *testing.T) {
	// A nonfatal diagnostic keeps the session alive until the handler enters.
	// A fatal frame can close the client and stop its best-effort logger before
	// queued diagnostics are consumed, making this synchronization unreliable.
	gateway := newUnknownInboundGateway(t)
	handler := blockingLogHandler{
		entered: make(chan struct{}),
		release: make(chan struct{}),
		once:    new(sync.Once),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := ibkr.DialContext(ctx,
		ibkr.WithDialer(gateway.dialer),
		ibkr.WithReconnectPolicy(ibkr.ReconnectOff),
		ibkr.WithLogger(slog.New(handler)),
	)
	if err != nil {
		t.Fatalf("DialContext() error = %v", err)
	}
	t.Cleanup(func() {
		close(handler.release)
		client.Close()
		gateway.Wait(t)
	})

	select {
	case <-handler.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("unknown-frame diagnostic did not reach blocking logger")
	}

	// Complete the captured clock exchange while logging is blocked. The
	// Gateway then waits for Client.Close, keeping teardown caller-controlled.
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime with blocked logger: %v", err)
	}
	closed := make(chan struct{})
	go func() {
		client.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Client.Close blocked behind configured slog handler")
	}
}

//go:build linux

package ibkr_test

import (
	"context"
	"net"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
	"github.com/ThomasMarcelis/ibkr-go/testing/ibkrlive"
)

func TestLiveIssue12TCPKeepAliveDefaultAndDisable(t *testing.T) {
	cfg := ibkrlive.Require(t)

	t.Run("default enabled", func(t *testing.T) {
		dialer := &recordingLiveTCPDialer{}
		client := dialLiveWithRecordedTCPConn(t, cfg, dialer)
		defer closeLiveIssueClient(t, client)

		if got := socketKeepAlive(t, dialer.Conn()); got != 1 {
			t.Fatalf("SO_KEEPALIVE = %d, want 1", got)
		}
	})

	t.Run("disabled", func(t *testing.T) {
		dialer := &recordingLiveTCPDialer{}
		client := dialLiveWithRecordedTCPConn(t, cfg, dialer, ibkr.WithTCPKeepAlive(0))
		defer closeLiveIssueClient(t, client)

		if got := socketKeepAlive(t, dialer.Conn()); got != 0 {
			t.Fatalf("SO_KEEPALIVE = %d, want 0", got)
		}
	})
}

type recordingLiveTCPDialer struct {
	mu   sync.Mutex
	conn *net.TCPConn
}

func (d *recordingLiveTCPDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	dialer := net.Dialer{KeepAlive: -1}
	conn, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		_ = conn.Close()
		return nil, &net.OpError{Op: "dial", Net: network, Err: syscall.EPROTONOSUPPORT}
	}
	d.mu.Lock()
	d.conn = tcpConn
	d.mu.Unlock()
	return tcpConn, nil
}

func (d *recordingLiveTCPDialer) Conn() *net.TCPConn {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.conn
}

func dialLiveWithRecordedTCPConn(t *testing.T, cfg ibkrlive.Config, dialer *recordingLiveTCPDialer, extra ...ibkr.Option) *ibkr.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	opts := []ibkr.Option{
		ibkr.WithHost(cfg.Host),
		ibkr.WithPort(cfg.Port),
		ibkr.WithClientID(liveIssueClientID()),
		ibkr.WithDialer(dialer),
	}
	opts = append(opts, extra...)
	client, err := ibkr.DialContext(ctx, opts...)
	if err != nil {
		t.Fatalf("DialContext() error = %v", err)
	}
	if dialer.Conn() == nil {
		_ = client.Close()
		t.Fatal("recording dialer did not capture TCP connection")
	}
	return client
}

func socketKeepAlive(t *testing.T, conn *net.TCPConn) int {
	t.Helper()
	raw, err := conn.SyscallConn()
	if err != nil {
		t.Fatalf("SyscallConn() error = %v", err)
	}
	var value int
	var sockErr error
	if err := raw.Control(func(fd uintptr) {
		value, sockErr = syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_KEEPALIVE)
	}); err != nil {
		t.Fatalf("Control() error = %v", err)
	}
	if sockErr != nil {
		t.Fatalf("GetsockoptInt(SO_KEEPALIVE) error = %v", sockErr)
	}
	return value
}

//go:build legacy_native_socket

package main

import (
	"io"
	"net"
	"strings"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

func TestBootstrapDecodesTypedMessages(t *testing.T) {
	t.Parallel()

	conn, cleanup := startBootstrapPeer(t,
		wire.EncodeFields([]string{"4", "-1", "2104", "market data farm ok", "", ""}),
		wire.EncodeFields([]string{"15", "1", "DU12345,DU67890,"}),
		wire.EncodeFields([]string{"9", "1", "1001"}),
	)
	defer cleanup()

	info, err := bootstrap(conn, 1, 100, 200)
	if err != nil {
		t.Fatalf("bootstrap() error = %v", err)
	}
	if info.ServerVersion != 200 {
		t.Fatalf("ServerVersion = %d, want 200", info.ServerVersion)
	}
	if info.ManagedAccounts != "DU12345,DU67890" {
		t.Fatalf("ManagedAccounts = %q, want DU12345,DU67890", info.ManagedAccounts)
	}
	if info.NextValidID != 1001 {
		t.Fatalf("NextValidID = %d, want 1001", info.NextValidID)
	}
}

func TestBootstrapFailsFastOnMalformedFrame(t *testing.T) {
	t.Parallel()

	conn, cleanup := startBootstrapPeer(t, []byte("not-a-message-id"))
	defer cleanup()

	_, err := bootstrap(conn, 1, 100, 200)
	if err == nil {
		t.Fatal("bootstrap() error = nil, want malformed bootstrap frame error")
	}
	if !strings.Contains(err.Error(), "parse bootstrap frame") {
		t.Fatalf("bootstrap() error = %v, want parse bootstrap frame failure", err)
	}
}

func startBootstrapPeer(t *testing.T, frames ...[]byte) (net.Conn, func()) {
	t.Helper()

	serverConn, clientConn := net.Pipe()
	errCh := make(chan error, 1)
	stop := make(chan struct{})

	go func() {
		errCh <- serveBootstrapPeer(serverConn, stop, frames)
	}()

	cleanup := func() {
		close(stop)
		_ = clientConn.Close()
		if err := <-errCh; err != nil {
			t.Fatalf("bootstrap peer error = %v", err)
		}
	}
	return clientConn, cleanup
}

func serveBootstrapPeer(conn net.Conn, stop <-chan struct{}, frames [][]byte) error {
	defer conn.Close()

	prefix := make([]byte, len(codec.EncodeHandshakePrefix()))
	if _, err := io.ReadFull(conn, prefix); err != nil {
		return err
	}
	if string(prefix) != string(codec.EncodeHandshakePrefix()) {
		return io.ErrUnexpectedEOF
	}
	if _, err := wire.ReadFrame(conn); err != nil {
		return err
	}
	if err := wire.WriteFrame(conn, wire.EncodeFields([]string{"200", "2026-04-10T12:00:00Z"})); err != nil {
		return err
	}
	if _, err := wire.ReadFrame(conn); err != nil {
		return err
	}
	for _, frame := range frames {
		if err := wire.WriteFrame(conn, frame); err != nil {
			return err
		}
	}
	<-stop
	return nil
}

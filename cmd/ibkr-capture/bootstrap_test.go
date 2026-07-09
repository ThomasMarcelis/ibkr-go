package main

import (
	"bytes"
	"encoding/hex"
	"io"
	"net"
	"slices"
	"strings"
	"testing"
	"time"

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

func TestExecutionsCaptureServer201ExactVectors(t *testing.T) {
	t.Parallel()

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- sendReqExecutionsAt(client, 201, 1001)
	}()
	payload, err := wire.ReadFrame(server)
	if err != nil {
		t.Fatalf("ReadFrame(request) error = %v", err)
	}
	if want := decodeHex(t, "000000cf08e9071200"); !bytes.Equal(payload, want) {
		t.Fatalf("request = %x, want live vector %x", payload, want)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("sendReqExecutionsAt() error = %v", err)
	}

	go func() {
		errCh <- wire.WriteFrame(server, decodeHex(t, "000000ff08e907"))
	}()
	var gotFields []string
	err = readFramesAt(client, 201, time.Second, func(_ int, fields []string) {
		gotFields = append([]string(nil), fields...)
	}, stopOnMsgIDWithReq(55, "1001", 1))
	if err != nil {
		t.Fatalf("readFramesAt() error = %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("WriteFrame(response) error = %v", err)
	}
	if want := []string{"55", "1001"}; !slices.Equal(gotFields, want) {
		t.Fatalf("fields = %q, want %q", gotFields, want)
	}
}

func decodeHex(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
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

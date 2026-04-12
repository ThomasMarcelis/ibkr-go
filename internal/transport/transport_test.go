package transport

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

func TestConnRoundTrip(t *testing.T) {
	t.Parallel()

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	conn := New(clientConn, nil, 0)
	defer conn.Close()

	payload := wire.EncodeFields([]string{"hello", "1", "7"})
	if err := conn.Send(context.Background(), payload); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	gotPayload, err := wire.ReadFrame(serverConn)
	if err != nil {
		t.Fatalf("ReadFrame(server) error = %v", err)
	}
	gotFields, err := wire.ParseFields(gotPayload)
	if err != nil {
		t.Fatalf("ParseFields() error = %v", err)
	}
	if gotFields[0] != "hello" {
		t.Fatalf("message = %q, want hello", gotFields[0])
	}
}

func TestConnReceivesIncomingFrames(t *testing.T) {
	t.Parallel()

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	conn := New(clientConn, nil, 0)
	defer conn.Close()

	go func() {
		_ = wire.WriteFrame(serverConn, wire.EncodeFields([]string{"managed_accounts", "DU12345"}))
	}()

	select {
	case payload := <-conn.Incoming():
		fields, err := wire.ParseFields(payload)
		if err != nil {
			t.Fatalf("ParseFields() error = %v", err)
		}
		if fields[0] != "managed_accounts" {
			t.Fatalf("message = %q, want managed_accounts", fields[0])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for incoming payload")
	}
}

func TestConnCloseCompletes(t *testing.T) {
	t.Parallel()

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	conn := New(clientConn, nil, 0)
	if err := conn.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := conn.Wait(); err != nil && !errors.Is(err, net.ErrClosed) {
		t.Fatalf("Wait() error = %v", err)
	}
}

func TestConnSendBackpressuresWithoutClosing(t *testing.T) {
	t.Parallel()

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	conn := New(clientConn, nil, 1)
	defer conn.Close()

	payload := wire.EncodeFields([]string{"hello", "1", "7"})

	for i := 0; i < 512; i++ {
		err := conn.Send(context.Background(), payload)
		if err != nil {
			if !errors.Is(err, ErrSendQueueFull) {
				t.Fatalf("Send() error = %v, want ErrSendQueueFull when queue fills", err)
			}
			select {
			case <-conn.Done():
				t.Fatalf("Conn closed after local backpressure; Wait() = %v", conn.Wait())
			default:
			}
			return
		}
	}

	t.Fatal("Send() never backpressured despite unread peer")
}

func TestConnSendCopiesPayload(t *testing.T) {
	t.Parallel()

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	conn := New(clientConn, nil, 0)
	defer conn.Close()

	payload := wire.EncodeFields([]string{"hello", "1", "7"})
	if err := conn.Send(context.Background(), payload); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	copy(payload, []byte("corrupt"))

	gotPayload, err := wire.ReadFrame(serverConn)
	if err != nil {
		t.Fatalf("ReadFrame(server) error = %v", err)
	}
	gotFields, err := wire.ParseFields(gotPayload)
	if err != nil {
		t.Fatalf("ParseFields() error = %v", err)
	}
	if gotFields[0] != "hello" {
		t.Fatalf("message = %q, want hello", gotFields[0])
	}
}

func TestConnCloseDoesNotWaitForQueuedFrames(t *testing.T) {
	t.Parallel()

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	conn := New(clientConn, nil, 1)

	payload := wire.EncodeFields([]string{"hello", "1", "7"})
	for i := 0; i < 8; i++ {
		if err := conn.Send(context.Background(), payload); err != nil {
			t.Fatalf("Send() error = %v", err)
		}
	}

	closeDone := make(chan error, 1)
	go func() {
		closeDone <- conn.Close()
	}()

	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close() blocked with queued frames")
	}
}

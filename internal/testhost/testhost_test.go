package testhost

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
)

func TestSplitRawDirections(t *testing.T) {
	t.Parallel()

	for _, direction := range []string{"client", "server"} {
		t.Run(direction, func(t *testing.T) {
			t.Parallel()

			frame := []byte("captured frame")
			script := "splitraw " + direction + " 1,2,3 " + base64.StdEncoding.EncodeToString(frame)
			host, err := New(script)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			conn, err := net.Dial("tcp", host.Addr())
			if err != nil {
				t.Fatalf("Dial() error = %v", err)
			}
			defer conn.Close()

			if direction == "client" {
				if _, err := conn.Write(frame); err != nil {
					t.Fatalf("Write() error = %v", err)
				}
			} else {
				got := make([]byte, len(frame))
				if _, err := io.ReadFull(conn, got); err != nil {
					t.Fatalf("ReadFull() error = %v", err)
				}
				if !bytes.Equal(got, frame) {
					t.Fatalf("read = %q, want %q", got, frame)
				}
			}
			if err := host.Wait(); err != nil {
				t.Fatalf("host.Wait() error = %v", err)
			}
		})
	}
}

func TestRawClientDirection(t *testing.T) {
	t.Parallel()

	host, err := New("raw client aGVsbG8=")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	conn, err := net.Dial("tcp", host.Addr())
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("hello")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if err := host.Wait(); err != nil {
		t.Fatalf("host.Wait() error = %v", err)
	}
}

func TestRawClientMismatchClosesConnection(t *testing.T) {
	t.Parallel()

	host, err := New("raw client aGVsbG8=")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	conn, err := net.Dial("tcp", host.Addr())
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("jello")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("SetReadDeadline() error = %v", err)
	}
	if _, err := conn.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("Read() error = %v, want EOF", err)
	}

	err = host.Wait()
	if err == nil || !strings.Contains(err.Error(), "raw client bytes") {
		t.Fatalf("host.Wait() error = %v, want raw-client mismatch", err)
	}
}

func TestSplitClientDirection(t *testing.T) {
	t.Parallel()

	payload := wire.EncodeFields([]string{"15", "1", "DU12345"})
	frame := appendLengthPrefix(payload)
	host, err := New("splitraw client 2,2 " + base64.StdEncoding.EncodeToString(frame))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	conn, err := net.Dial("tcp", host.Addr())
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write(frame); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if err := host.Wait(); err != nil {
		t.Fatalf("host.Wait() error = %v", err)
	}
}

func TestRunClosesListenerOnSuccessAndReadFailure(t *testing.T) {
	// Exact sv225 current-time request from supported_version_matrix.txt.
	frame, err := base64.StdEncoding.DecodeString("AAAABAAAAPk=")
	if err != nil {
		t.Fatal(err)
	}
	for _, failRead := range []bool{false, true} {
		host, err := New("raw client AAAABAAAAPk=")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = host.Close() })
		conn, err := net.Dial("tcp", host.Addr())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		outgoing := frame
		if failRead {
			outgoing = frame[:1]
		} // Fault injection: cut the captured write short.
		if _, err := conn.Write(outgoing); err != nil {
			t.Fatal(err)
		}
		if failRead {
			_ = conn.Close()
		}
		if err := host.Wait(); (err != nil) != failRead {
			t.Fatalf("Wait = %v, failed read = %t", err, failRead)
		}
		if _, err := host.listener.Accept(); !errors.Is(err, net.ErrClosed) {
			t.Fatalf("completed host listener = %v", err)
		}
	}
}

func TestCloseInterruptsReplaySleep(t *testing.T) {
	host, err := New("sleep 1h") // Local scheduling instruction; no protocol data.
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = host.Close() })
	_ = host.Close()
	select {
	case <-host.done:
	case <-time.After(time.Second):
		t.Fatal("Close left replay sleeping")
	}
	if !errors.Is(host.Wait(), net.ErrClosed) {
		t.Fatalf("interrupted replay = %v", host.Wait())
	}
}

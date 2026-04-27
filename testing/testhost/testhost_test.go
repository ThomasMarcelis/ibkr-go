//go:build legacy_native_socket

package testhost

import (
	"net"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

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

func TestSplitClientDirection(t *testing.T) {
	t.Parallel()

	host, err := New(`split client 2,2 managed_accounts {"accounts":["DU12345"]}`)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	conn, err := net.Dial("tcp", host.Addr())
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()

	payload := wire.EncodeFields([]string{"15", "1", "DU12345"})
	frame := appendLengthPrefix(payload)
	if _, err := conn.Write(frame); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if err := host.Wait(); err != nil {
		t.Fatalf("host.Wait() error = %v", err)
	}
}

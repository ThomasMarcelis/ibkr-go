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

func TestDecodeClientScannerSubscriptionRequiresCompleteServer200Shape(t *testing.T) {
	t.Parallel()

	// captures/20260407T190657Z-scanner_subscription/events.jsonl,
	// 2026-04-07T19:06:58.035817304Z, after stripping the frame length.
	const maxFloat = "1.7976931348623157E308"
	const maxInt = "2147483647"
	fields := []string{
		"22", "1001", "10", "STK", "STK.US.MAJOR", "HOT_BY_VOLUME",
		maxFloat, maxFloat, maxInt, maxFloat, maxFloat,
		"", "", "", "", "", "",
		maxFloat, maxFloat, "", maxInt, "", "", "", "",
	}
	name, body, err := decodeClientMessage(wire.EncodeFields(fields))
	if err != nil {
		t.Fatalf("decodeClientMessage() error = %v", err)
	}
	if name != "req_scanner_subscription" {
		t.Fatalf("message name = %q, want req_scanner_subscription", name)
	}
	if got, want := body["average_option_volume_above"], maxInt; got != want {
		t.Fatalf("average_option_volume_above = %q, want %q", got, want)
	}
	if got := body["subscription_options"]; got != "" {
		t.Fatalf("subscription_options = %q, want empty", got)
	}

	if _, _, err := decodeClientMessage(wire.EncodeFields(fields[:len(fields)-1])); err == nil {
		t.Fatal("decodeClientMessage() accepted the legacy one-field-short request")
	}
}

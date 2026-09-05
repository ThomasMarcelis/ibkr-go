package transport

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

type writeErrorConn struct {
	net.Conn
	accepted int
	err      error
}

func (c writeErrorConn) Write(p []byte) (int, error) { return min(c.accepted, len(p)), c.err }

func TestTrackedWriteClassifiesAcceptedBytesDespiteError(t *testing.T) {
	// Exact sv225 current_time_live.txt request; events.jsonl SHA-256:
	// a9029ff8e7cfed19cab1e3e2eccc4c36d7c91b95aa6aa03f75543bacac454a9e.
	// Fault injection varies local socket acceptance, not Gateway semantics.
	payload := []byte{0, 0, 0, 249}
	for _, test := range []struct {
		name    string
		bytes   int
		outcome WriteOutcome
	}{
		{"unwritten", 0, WriteUnwritten},
		{"incomplete", 3, WriteIncomplete},
		{"complete with error", len(payload) + 4, WriteCompleteLocal},
	} {
		t.Run(test.name, func(t *testing.T) {
			peer, client := net.Pipe()
			defer peer.Close()
			fault := errors.New("injected write failure")
			tr := New(writeErrorConn{Conn: client, accepted: test.bytes, err: fault}, nil, 0)
			defer tr.Close()
			id, err := tr.SendTracked(context.Background(), payload)
			if err != nil {
				t.Fatal(err)
			}
			select {
			case <-tr.Done():
			case <-time.After(time.Second):
				t.Fatal("write error did not retire transport")
			}
			result, ok := <-tr.Completions()
			if !ok || result.ID != id || result.Outcome != test.outcome || !errors.Is(result.Err, fault) {
				t.Fatalf("completion = %+v, want id=%d outcome=%v and original error", result, id, test.outcome)
			}
			if _, ok := <-tr.Completions(); ok {
				t.Fatal("duplicate completion")
			}
			if err := tr.Wait(); !errors.Is(err, fault) {
				t.Fatalf("Wait: %v", err)
			}
		})
	}
}

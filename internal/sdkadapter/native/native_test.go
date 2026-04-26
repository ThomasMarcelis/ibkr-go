//go:build ibkr_sdk && cgo && linux

package native

import "testing"

func TestNewCloseIsIdempotent(t *testing.T) {
	adapter, err := New(1)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if adapter.IsConnected() {
		t.Fatal("new adapter is connected")
	}
	if err := adapter.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

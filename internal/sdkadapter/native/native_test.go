//go:build ibkr_sdk && cgo && linux

package native

import (
	"context"
	"errors"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
)

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

func TestBuildInfoReturnsCopiedMetadata(t *testing.T) {
	info, err := BuildInfo()
	if err != nil {
		t.Fatalf("BuildInfo() error = %v", err)
	}
	if info.AdapterABIVersion == "" {
		t.Fatal("BuildInfo() returned empty adapter ABI version")
	}
	if info.SDKAPIVersion == "" {
		t.Fatal("BuildInfo() returned empty SDK API version")
	}
	if info.Compiler == "" {
		t.Fatal("BuildInfo() returned empty compiler")
	}
	if info.ProtobufMode == "" {
		t.Fatal("BuildInfo() returned empty protobuf mode")
	}
}

func TestNullNativeHandlesReturnErrors(t *testing.T) {
	adapter := &Adapter{}

	if err := adapter.Submit(context.Background(), sdkadapter.Command{Kind: sdkadapter.CommandCurrentTime}); err == nil {
		t.Fatal("Submit() error = nil, want nil native handle error")
	}

	if _, err := adapter.DrainEvents(context.Background(), 1); err == nil {
		t.Fatal("DrainEvents() error = nil, want nil native handle error")
	}

	err := adapter.Connect(context.Background(), sdkadapter.ConnectRequest{Host: "127.0.0.1", Port: 4002, ClientID: 1})
	if err == nil {
		t.Fatal("Connect() error = nil, want nil native handle error")
	}
}

func TestDrainEmptyBatchFreePath(t *testing.T) {
	adapter, err := New(1)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer adapter.Close()

	events, err := adapter.DrainEvents(context.Background(), 8)
	if err != nil {
		t.Fatalf("DrainEvents() error = %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("DrainEvents() returned %d events, want 0", len(events))
	}
}

func TestSubmitAfterCloseReturnsClosed(t *testing.T) {
	adapter, err := New(1)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	err = adapter.Submit(context.Background(), sdkadapter.Command{Kind: sdkadapter.CommandCurrentTime})
	if !errors.Is(err, sdkadapter.ErrClosed) {
		t.Fatalf("Submit() error = %v, want ErrClosed", err)
	}
}

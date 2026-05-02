//go:build ibkr_sdk && cgo && linux

package native

import (
	"context"
	"errors"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

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

func TestConnectSilentListenerTimesOut(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			t.Skipf("local listen denied by sandbox: %v", err)
		}
		t.Fatalf("Listen() error = %v", err)
	}

	var connsMu sync.Mutex
	var conns []net.Conn
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			connsMu.Lock()
			conns = append(conns, conn)
			connsMu.Unlock()
		}
	}()
	defer func() {
		_ = ln.Close()
		<-acceptDone
		connsMu.Lock()
		defer connsMu.Unlock()
		for _, conn := range conns {
			_ = conn.Close()
		}
	}()

	adapter, err := New(8)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer adapter.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = adapter.Connect(ctx, sdkadapter.ConnectRequest{
		Host:     "127.0.0.1",
		Port:     ln.Addr().(*net.TCPAddr).Port,
		ClientID: 77,
		Timeout:  200 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("Connect() error = nil, want metadata timeout")
	}
	if !strings.Contains(err.Error(), "official SDK server metadata timed out") {
		t.Fatalf("Connect() error = %v, want metadata timeout", err)
	}
	if adapter.IsConnected() {
		t.Fatal("adapter is connected after metadata timeout")
	}
}

func TestConnectReturnsSDKErrorDetailWhenEConnectFails(t *testing.T) {
	adapter, err := New(8)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer adapter.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = adapter.Connect(ctx, sdkadapter.ConnectRequest{
		Host:     "127.0.0.1",
		Port:     1,
		ClientID: 77,
		Timeout:  200 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("Connect() error = nil, want eConnect failure")
	}
	for _, want := range []string{"official SDK eConnect returned false", "SDK error"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Connect() error = %v, want %q", err, want)
		}
	}
	if adapter.IsConnected() {
		t.Fatal("adapter is connected after eConnect failure")
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

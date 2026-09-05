package main

import (
	"bytes"
	"context"
	"net"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestInterruptDuringConnect(t *testing.T) {
	if os.Getenv("IBKR_TEST_INTERRUPT_HELPER") == "1" {
		main()
		return
	}
	if runtime.GOOS == "windows" {
		t.Skip("sending os.Interrupt to a process is unsupported on Windows")
	}

	// Hold the TCP connection open without completing the handshake. Accept
	// confirms that the example installed its signal handler and started dialing.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	if err := listener.(*net.TCPListener).SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	// #nosec G204 -- os.Executable returns this test binary; arguments are fixed.
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestInterruptDuringConnect$")
	cmd.Env = append(os.Environ(), "IBKR_TEST_INTERRUPT_HELPER=1", "IBKR_ADDR="+listener.Addr().String())
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	conn, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("interrupt during connection: %v\n%s", err, output.String())
	}
}

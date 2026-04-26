//go:build ibkr_swig && cgo && linux

package ibkrsdkprobe

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestCompileProbeVersion(t *testing.T) {
	if got := CompileProbeVersion(); got != "ibkr-swig-official-sdk-probe" {
		t.Fatalf("CompileProbeVersion() = %q, want %q", got, "ibkr-swig-official-sdk-probe")
	}

	probe := NewProbe()
	defer probe.Close()

	if got := probe.Snapshot(); got.Connected {
		t.Fatalf("new probe connected = true, want false")
	}
}

func TestSDKProbeLive(t *testing.T) {
	if os.Getenv("IBKR_LIVE") != "1" {
		t.Skip("set IBKR_LIVE=1 after the compile/link probe succeeds")
	}

	host := envString("IBKR_HOST", "127.0.0.1")
	port := envInt(t, "IBKR_PORT", 4002)
	clientID := envInt(t, "IBKR_CLIENT_ID", 9101)

	probe := NewProbe()
	defer probe.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	snapshot, err := probe.Connect(ctx, host, port, clientID)
	if err != nil {
		t.Fatalf("Connect(%s:%d, clientID=%d) error = %v; snapshot = %+v", host, port, clientID, err, snapshot)
	}
	t.Logf("official SDK bootstrap complete: serverVersion=%d connectionTime=%q nextValidID=%d managedAccounts=%d errorCount=%d",
		snapshot.ServerVersion,
		snapshot.ConnectionTime,
		snapshot.NextValidID,
		managedAccountCount(snapshot.ManagedAccountsCSV),
		len(snapshot.Errors),
	)

	currentTime, err := probe.RequestCurrentTime(ctx)
	if err != nil {
		t.Fatalf("RequestCurrentTime() error = %v; snapshot = %+v", err, probe.Snapshot())
	}
	if currentTime.IsZero() {
		t.Fatal("RequestCurrentTime() returned zero time")
	}
	t.Logf("official SDK currentTime received: %s", currentTime.Format(time.RFC3339))

	rows, err := probe.RequestAccountSummary(ctx, 9001, "All", []string{"NetLiquidation", "BuyingPower"})
	if err != nil {
		t.Fatalf("RequestAccountSummary() error = %v; snapshot = %+v", err, probe.Snapshot())
	}
	if len(rows) == 0 {
		t.Fatalf("RequestAccountSummary() returned 0 rows; snapshot = %+v", probe.Snapshot())
	}
	t.Logf("official SDK accountSummary received rows=%d", len(rows))
}

func envString(name string, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}

func envInt(t *testing.T, name string, fallback int) int {
	t.Helper()
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatalf("parse %s=%q: %v", name, raw, err)
	}
	return value
}

func managedAccountCount(csv string) int {
	if csv == "" {
		return 0
	}
	count := 1
	for _, ch := range csv {
		if ch == ',' {
			count++
		}
	}
	return count
}

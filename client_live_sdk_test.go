//go:build ibkr_sdk && cgo && linux

package ibkr_test

import (
	"context"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
	"github.com/ThomasMarcelis/ibkr-go/testing/ibkrlive"
)

func TestLiveOfficialSDKSmoke(t *testing.T) {
	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	snapshot := client.Session()
	t.Logf("official SDK session ready: serverVersion=%d managedAccounts=%d nextValidID=%d",
		snapshot.ServerVersion,
		len(snapshot.ManagedAccounts),
		snapshot.NextValidID,
	)
	if snapshot.State != ibkr.StateReady {
		t.Fatalf("state = %s, want %s", snapshot.State, ibkr.StateReady)
	}
	if snapshot.ServerVersion == 0 {
		t.Fatal("server version is zero")
	}

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()

	currentTime, err := client.CurrentTime(ctx)
	if err != nil {
		t.Fatalf("CurrentTime() error = %v", err)
	}
	if currentTime.IsZero() {
		t.Fatal("CurrentTime() returned zero time")
	}
	t.Logf("official SDK currentTime received: %s", currentTime.Format(time.RFC3339))

	currentTimeMillis, err := client.CurrentTimeMillis(ctx)
	if err != nil {
		t.Fatalf("CurrentTimeMillis() error = %v", err)
	}
	if currentTimeMillis.IsZero() {
		t.Fatal("CurrentTimeMillis() returned zero time")
	}
	t.Logf("official SDK currentTimeMillis received: %s", currentTimeMillis.Format(time.RFC3339Nano))

	values, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Account: "All",
		Tags:    []string{"NetLiquidation", "BuyingPower"},
	})
	if err != nil {
		t.Fatalf("AccountSummary() error = %v", err)
	}
	if len(values) == 0 {
		t.Fatal("AccountSummary() returned no rows")
	}
	t.Logf("official SDK accountSummary rows=%d", len(values))

	details, err := client.Contracts().Details(ctx, ibkr.Contract{
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	})
	if err != nil {
		t.Fatalf("ContractDetails() error = %v", err)
	}
	if len(details) == 0 {
		t.Fatal("ContractDetails() returned no rows")
	}
	t.Logf("official SDK contractDetails rows=%d", len(details))

	positions, err := client.Accounts().Positions(ctx)
	if err != nil {
		t.Fatalf("Positions() error = %v", err)
	}
	t.Logf("official SDK positions rows=%d", len(positions))
}

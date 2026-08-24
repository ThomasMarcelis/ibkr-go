package ibkr_test

import (
	"context"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
)

func TestScannerSubscriptionReturnsCurrentRankedResults(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "scanner_subscription.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.Scanner().SubscribeResults(ctx, ibkr.ScannerSubscriptionRequest{
		NumberOfRows: 10,
		Instrument:   "STK",
		LocationCode: "STK.US.MAJOR",
		ScanCode:     "HOT_BY_VOLUME",
	})
	if err != nil {
		t.Fatalf("SubscribeResults() error = %v", err)
	}

	results := waitForStreamData(t, sub.Events())
	if len(results) != 10 {
		t.Fatalf("results len = %d, want 10", len(results))
	}
	if got := results[0]; got.Rank != 0 || got.Contract.ConID != 912285503 ||
		got.Contract.Symbol != "HXA" || got.Contract.Exchange != "SMART" ||
		got.Contract.Currency != "USD" || got.Contract.LocalSymbol != "HXA" ||
		got.MarketName != "HXA" || got.Contract.TradingClass != "HXA" {
		t.Fatalf("first result = %+v, want exact live HXA scanner result", got)
	}
	if got := results[9]; got.Rank != 9 || got.Contract.ConID != 810784496 || got.Contract.Symbol != "PMSE" {
		t.Fatalf("last result = %+v, want exact live rank 9 PMSE result", got)
	}

	sub.Close()
	if err := sub.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime() cleanup fence error = %v", err)
	}
}

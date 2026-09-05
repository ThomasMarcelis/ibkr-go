package ibkr_test

import (
	"context"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/shopspring/decimal"
)

// These scenarios previously ran as documentation examples. Keep their
// live-derived assertions here so public examples need no replay harness.
// Each retained transcript's header records its server version and source hash.
func TestQualifyCapturedContract(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "grounded_contract_details_aapl.txt")
	defer cleanupClientHost(t, client, host)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	details, err := client.Contracts().Qualify(ctx, ibkr.Stock("AAPL"))
	if err != nil {
		t.Fatal(err)
	}
	if details.ConID != 265598 || details.LongName != "APPLE INC" {
		t.Fatalf("qualified contract = %d %q, want 265598 APPLE INC", details.ConID, details.LongName)
	}
}

func TestAwaitCapturedAccountSnapshotWithConsumer(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "grounded_account_summary.txt")
	defer cleanupClientHost(t, client, host)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	sub, err := client.Accounts().SubscribeSummary(ctx, ibkr.AccountSummaryRequest{
		Group: "All",
		Tags:  []string{"NetLiquidation", "TotalCashValue", "BuyingPower", "ExcessLiquidity"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	consumed := make(chan int, 1)
	go func() {
		rows := 0
		for event := range sub.Events() {
			if event.Kind == ibkr.StreamData {
				rows++
			}
		}
		consumed <- rows
	}()
	if err := sub.AwaitSnapshot(ctx); err != nil {
		t.Fatalf("AwaitSnapshot: %v", err)
	}
	sub.Close()
	if rows := <-consumed; rows != 4 {
		t.Fatalf("snapshot rows = %d, want 4", rows)
	}
	if err := sub.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if err := sub.AwaitSnapshot(ctx); err != nil {
		t.Fatalf("completed snapshot lost after close: %v", err)
	}
}

func TestHandleCancelCapturedOrder(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "direct_cancel_order.txt")
	defer cleanupClientHost(t, client, host)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			ConID: 265598, Symbol: "AAPL", SecType: ibkr.SecTypeStock,
			Exchange: "SMART", Currency: "USD",
		},
		Order: ibkr.Order{
			Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeLimit,
			Quantity: decimal.NewFromInt(1), LmtPrice: new(decimal.NewFromInt(10)),
			TIF: ibkr.TIFDay, Account: "DU9000001",
			OrderRef: "sanitized-order-ref-0000000000000001",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	if handle.OrderID() != 506 {
		t.Fatalf("order ID = %d, want 506", handle.OrderID())
	}
	waitOrderStatusUpdate(t, ctx, handle, ibkr.OrderStatusPreSubmitted)
	if err := handle.Cancel(ctx); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	waitOrderStatusUpdate(t, ctx, handle, ibkr.OrderStatusCancelled)
	handle.Close()
	if err := handle.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}
}

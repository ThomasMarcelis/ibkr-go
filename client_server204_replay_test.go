package ibkr_test

import (
	"context"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go"
	"github.com/shopspring/decimal"
)

func TestCompletedOrdersServer204Replay(t *testing.T) {
	restore := ibkr.SetAdvertisedServerVersionMaxForTest(204)
	defer restore()

	client, host := newClient(t, "completed_orders_sv204_live.txt")
	defer client.Close()
	defer waitHost(t, host)
	if got := client.Session().ServerVersion; got != 204 {
		t.Fatalf("Session().ServerVersion = %d, want 204", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	orders, err := client.Orders().Completed(ctx, false)
	if err != nil {
		t.Fatalf("Completed() error = %v", err)
	}
	if len(orders) != 2 {
		t.Fatalf("Completed() returned %d orders, want 2 live-derived results", len(orders))
	}

	cancelled := orders[0]
	if cancelled.Contract.ConID != 265598 || cancelled.Order.OrderID == nil || *cancelled.Order.OrderID != 1 ||
		cancelled.Order.ClientID == nil || *cancelled.Order.ClientID != 901 ||
		cancelled.Order.ParentID == nil || *cancelled.Order.ParentID != 0 ||
		cancelled.Order.PermID == nil || *cancelled.Order.PermID != 900000001 ||
		cancelled.Completion.Status != ibkr.OrderStatusCancelled || !cancelled.Completion.Filled.IsZero() ||
		cancelled.Completion.CommissionAndFees != nil {
		t.Fatalf("cancelled completed order = %+v", cancelled)
	}

	filled := orders[1]
	if filled.Order.OrderID == nil || *filled.Order.OrderID != 2 ||
		filled.Order.ClientID == nil || *filled.Order.ClientID != 201 ||
		filled.Order.ParentID == nil || *filled.Order.ParentID != 0 ||
		filled.Order.PermID == nil || *filled.Order.PermID != 900000002 ||
		filled.Completion.Status != ibkr.OrderStatusFilled || !filled.Completion.Filled.Equal(decimal.NewFromInt(1)) ||
		filled.Completion.CommissionAndFees == nil || !filled.Completion.CommissionAndFees.Equal(decimal.RequireFromString("1.006695")) ||
		filled.Completion.CommissionAndFeesCurrency != "USD" {
		t.Fatalf("filled completed order = %+v", filled)
	}
}

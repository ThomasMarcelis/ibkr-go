package ibkr_test

import (
	"context"
	"strings"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/shopspring/decimal"
)

func TestIncludeOvernightTrueToFalseLifecycleReplay(t *testing.T) {
	client, host := newClient(t, "include_overnight_lifecycle_aapl.txt", ibkr.WithClientID(0))
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	order := ibkr.Order{
		Action:           ibkr.ActionBuy,
		OrderType:        ibkr.OrderTypeLimit,
		Quantity:         decimal.NewFromInt(1),
		LmtPrice:         new(decimal.RequireFromString("15.48")),
		TIF:              ibkr.TIFDay,
		Account:          "DU9000001",
		OrderRef:         "sanitized-order-ref-0000000000000001",
		IncludeOvernight: new(true),
	}
	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{ConID: 265598, Symbol: "AAPL", SecType: ibkr.SecTypeStock, Exchange: "SMART", Currency: "USD"},
		Order:    order,
	})
	if err != nil {
		t.Fatalf("Place() error = %v", err)
	}
	if handle.OrderID() != 490 {
		t.Fatalf("OrderID() = %d, want 490", handle.OrderID())
	}

	placed := nextOpenOrderEvent(t, ctx, handle)
	if placed.Order.IncludeOvernight == nil || !*placed.Order.IncludeOvernight {
		t.Fatalf("placement IncludeOvernight = %v, want explicit true", placed.Order.IncludeOvernight)
	}
	if placed.Order.TIF != ibkr.TimeInForce("OVERNIGHT + DAY") {
		t.Fatalf("placement TIF = %q, want broker overnight echo", placed.Order.TIF)
	}
	if placed.Order.Compliance.Submitter != "paper-user-01" {
		t.Fatalf("placement submitter = %q, want sanitized live echo", placed.Order.Compliance.Submitter)
	}

	order.IncludeOvernight = new(false)
	if err := handle.Replace(ctx, order); err != nil {
		t.Fatalf("Replace(false) admission error = %v", err)
	}
	var replacement *ibkr.OpenOrder
	var warning *ibkr.APIError
	for replacement == nil || warning == nil {
		select {
		case event, ok := <-handle.Events():
			if !ok {
				t.Fatalf("order events closed during replacement: %v", handle.Wait())
			}
			if event.OpenOrder != nil {
				replacement = event.OpenOrder
			}
			if event.Warning != nil && event.Warning.Code == 462 {
				warning = event.Warning
			}
		case <-ctx.Done():
			t.Fatalf("waiting for replacement evidence: %v", ctx.Err())
		}
	}
	if replacement.Order.IncludeOvernight == nil || !*replacement.Order.IncludeOvernight {
		t.Fatalf("blocked replacement IncludeOvernight = %v, want retained true", replacement.Order.IncludeOvernight)
	}
	if warning.OpKind != ibkr.OpPlaceOrder || !strings.Contains(warning.Message, "Cannot change to the new Time in Force.DAY") {
		t.Fatalf("replacement warning = %+v, want typed code-462 TIF blocker", warning)
	}
	if ibkr.IsRetryable(warning) {
		t.Fatalf("replacement warning = %v, want non-retryable", warning)
	}

	cancelAndAwaitZeroFill(t, ctx, handle)
	handle.Close()
	requireCloseOrCapturedDisconnect(t, "IncludeOvernight true lifecycle", handle.Wait())

	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime() cleanup fence error = %v", err)
	}

	order.OrderRef = "sanitized-order-ref-0000000000000005"
	fresh, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{ConID: 265598, Symbol: "AAPL", SecType: ibkr.SecTypeStock, Exchange: "SMART", Currency: "USD"},
		Order:    order,
	})
	if err != nil {
		t.Fatalf("Place(false) error = %v", err)
	}
	if fresh.OrderID() != 491 {
		t.Fatalf("fresh OrderID() = %d, want 491", fresh.OrderID())
	}
	freshEcho := nextOpenOrderEvent(t, ctx, fresh)
	if freshEcho.Order.IncludeOvernight != nil {
		t.Fatalf("fresh false IncludeOvernight = %v, want broker-canonical absence", freshEcho.Order.IncludeOvernight)
	}
	if freshEcho.Order.TIF != ibkr.TIFDay {
		t.Fatalf("fresh false TIF = %q, want DAY", freshEcho.Order.TIF)
	}
	if freshEcho.Order.Compliance.Submitter != "paper-user-01" {
		t.Fatalf("fresh false submitter = %q, want sanitized live echo", freshEcho.Order.Compliance.Submitter)
	}
	cancelAndAwaitZeroFill(t, ctx, fresh)
	fresh.Close()
	requireCloseOrCapturedDisconnect(t, "IncludeOvernight false lifecycle", fresh.Wait())
}

func nextOpenOrderEvent(t *testing.T, ctx context.Context, handle *ibkr.OrderHandle) ibkr.OpenOrder {
	t.Helper()
	for {
		select {
		case event, ok := <-handle.Events():
			if !ok {
				t.Fatalf("order events closed before open-order echo: %v", handle.Wait())
			}
			if event.OpenOrder != nil {
				return *event.OpenOrder
			}
		case <-ctx.Done():
			t.Fatalf("waiting for open-order echo: %v", ctx.Err())
		}
	}
}

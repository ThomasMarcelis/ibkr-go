package ibkr_test

import (
	"context"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/shopspring/decimal"
)

// Capture 20260829T142318Z-api_empty_tif_default_aapl, server_version 225,
// events SHA-256 50351d930e5f74c0974f6bd2553caacc2c7ca2e160f217177a78553a6cd45101.
// The same request without the DAY default drew code 10052 from the live
// Gateway (capture 20260829T142218Z-api_empty_tif_default_aapl), so this
// replay fails byte-for-byte on the placeOrder frame if the default regresses.
func TestEmptyTIFIsSentAsDayReplay(t *testing.T) {
	client, host := newClient(t, "empty_tif_default_aapl.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	order := ibkr.LimitOrder(ibkr.ActionBuy, decimal.NewFromInt(1), decimal.RequireFromString("16.01"))
	order.Account = "DU9000001"
	order.OrderRef = "sanitized-order-ref-0000000000000001"
	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{ConID: 265598, Symbol: "AAPL", SecType: ibkr.SecTypeStock, Exchange: "SMART", Currency: "USD"},
		Order:    order,
	})
	if err != nil {
		t.Fatalf("Place() error = %v", err)
	}
	if handle.OrderID() != 684 {
		t.Fatalf("OrderID() = %d, want 684", handle.OrderID())
	}

	echo := nextOpenOrderEvent(t, ctx, handle)
	if echo.Order.TIF != ibkr.TIFDay {
		t.Fatalf("OpenOrder echo TIF = %q, want DAY", echo.Order.TIF)
	}

	cancelAndAwaitZeroFill(t, ctx, handle)
	handle.Close()
	requireCloseOrCapturedDisconnect(t, "empty TIF lifecycle", handle.Wait())

	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime() cleanup fence error = %v", err)
	}
}

package ibkr_test

import (
	"context"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
)

func TestOpenOrdersEmptyReplay(t *testing.T) {
	client, host := newClient(t, "open_orders_empty.txt")
	defer cleanupClientHost(t, client, host)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	orders, err := client.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if len(orders) != 0 {
		t.Fatalf("Open() returned %d orders, want live empty snapshot", len(orders))
	}
}

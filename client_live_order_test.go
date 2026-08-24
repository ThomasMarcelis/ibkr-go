package ibkr_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/ibkrlive"
)

func TestLiveManualTWSOrderBound(t *testing.T) {
	if os.Getenv("IBKR_LIVE_MANUAL_ORDER_BOUND") != "1" {
		t.Skip("set IBKR_LIVE_MANUAL_ORDER_BOUND=1 only while a manual paper-TWS order is ready to be created")
	}

	client, ctx, cancel := ibkrlive.DialTradingContext(t, 3*time.Minute, ibkr.WithClientID(0))
	defer cancel()
	defer client.Close()

	sub, err := client.Orders().SubscribeOpen(ctx, ibkr.OpenOrdersScopeAuto, ibkr.WithResumePolicy(ibkr.ResumeNever))
	if err != nil {
		t.Fatalf("SubscribeOpen(auto): %v", err)
	}
	defer func() {
		sub.Close()
		if err := sub.Wait(); err != nil {
			t.Errorf("auto-open subscription cleanup: %v", err)
		}
		probeCtx, probeCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer probeCancel()
		if _, err := client.CurrentTime(probeCtx); err != nil {
			t.Errorf("CurrentTime after disabling auto-open binding: %v", err)
		}
	}()

	t.Log("AUTO-OPEN ARMED: create one safely non-marketable manual order in paper TWS now; cancel it in TWS after capture")
	for {
		select {
		case event, ok := <-sub.Events():
			if !ok {
				t.Fatalf("auto-open subscription closed before orderBound: %v", sub.Err())
			}
			if event.Kind != ibkr.StreamData || event.Value.Binding == nil {
				continue
			}
			binding := *event.Value.Binding
			if binding.ClientID != 0 || binding.OrderID <= 0 || binding.PermID <= 0 {
				t.Fatalf("orderBound = %+v, want positive IDs bound to client 0", binding)
			}
			t.Logf("ORDER_BOUND_CAPTURED permID=%d clientID=%d orderID=%d serverVersion=%d", binding.PermID, binding.ClientID, binding.OrderID, client.Session().ServerVersion)
			return
		case <-ctx.Done():
			t.Fatalf("waiting for manual paper-TWS orderBound: %v", context.Cause(ctx))
		}
	}
}

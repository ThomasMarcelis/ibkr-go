package ibkr

import (
	"testing"
	"testing/synctest"
)

func TestOrderRouteIdentityPreventsStaleHandleClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newRunningEngineForOrderHandleTest(t)
		// ID 47 and the AAPL contract come from capture
		// 20260413T174517Z-api_bracket_trigger_aapl (sv200, hash prefix
		// 682a1390b2acf04c). This test rotates actor state only.
		const orderID = int64(47)

		bound := make(chan *OrderHandle, 1)
		e.enqueue(func() {
			bound <- e.bindOrderHandle(orderID, Contract{ConID: 265598, Exchange: "SMART"}, 0)
		})
		synctest.Wait()
		stale := <-bound

		replacement := newOrderHandle(orderID, e.cfg.orderEventBuffer)
		replacementRoute := &orderRoute{orderID: orderID, handle: replacement}
		rotated := make(chan struct{})
		e.enqueue(func() {
			e.closeOrderRoute(orderID, e.orders[orderID], nil)
			e.orders[orderID] = replacementRoute
			close(rotated)
		})
		synctest.Wait()
		<-rotated
		if err := stale.Wait(); err != nil {
			t.Fatalf("stale handle Wait() = %v, want nil", err)
		}

		// Route keys can later belong to a distinct order or exercise route.
		// Closing the retired handle must act only on its placement-time route.
		stale.Close()
		synctest.Wait()

		got := make(chan *orderRoute, 1)
		e.enqueue(func() { got <- e.orders[orderID] })
		synctest.Wait()
		if route := <-got; route != replacementRoute {
			t.Fatalf("route after stale Close() = %p, want replacement %p", route, replacementRoute)
		}
		select {
		case <-replacement.Done():
			t.Fatal("stale Close() terminated the replacement route")
		default:
		}
	})
}

package session

import "testing"

func TestSubscriptionCloseWaitsForCloseWithErr(t *testing.T) {
	t.Parallel()

	var sub *Subscription[int]
	sub = newSubscription[int](defaultSubscriptionConfig(defaultConfig()), func() {
		sub.closeWithErr(nil)
	})

	if err := sub.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := sub.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
}

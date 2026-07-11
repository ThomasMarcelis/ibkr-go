package ibkr_test

import (
	"context"
	"strings"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
)

func TestSubscribeNewsBulletinsLiveReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "news_bulletins_live.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.News().SubscribeBulletins(ctx, true)
	if err != nil {
		t.Fatalf("SubscribeBulletins() error = %v", err)
	}
	waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionStarted)

	first := waitForEvent(t, sub.Events())
	if first.MsgID != 1783570115 || first.MsgType != 1 || first.Source != "TWSE" {
		t.Fatalf("first bulletin identity = %+v", first)
	}
	if !strings.Contains(first.Headline, "market will be closed on Friday, July 10, 2026") {
		t.Fatalf("first bulletin headline = %q", first.Headline)
	}

	second := waitForEvent(t, sub.Events())
	if second.MsgID != 1783570118 || second.MsgType != 1 || second.Source != "HKFE,SGX" {
		t.Fatalf("second bulletin identity = %+v", second)
	}
	if !strings.Contains(second.Headline, "continue to trade according to their normal trading schedule") {
		t.Fatalf("second bulletin headline = %q", second.Headline)
	}

	sub.Close()
}

package ibkr_test

import (
	"context"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
)

func TestTickNewsAAPLReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "tick_news_aapl.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("SetType() error = %v", err)
	}
	sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
		Contract:     ibkr.Contract{ConID: 265598, Symbol: "AAPL", SecType: ibkr.SecTypeStock, Exchange: "SMART", Currency: "USD"},
		GenericTicks: []ibkr.GenericTick{"mdoff", "292:BRFG"},
	}, ibkr.WithResumePolicy(ibkr.ResumeNever))
	if err != nil {
		t.Fatalf("SubscribeQuotes() error = %v", err)
	}

	var update ibkr.QuoteUpdate
	for update.Kind != ibkr.QuoteUpdateNewsTick {
		update = waitForStreamData(t, sub.Events())
	}
	news := update.NewsTick
	if news == nil || !news.Time.Equal(time.UnixMilli(1761921315000).UTC()) ||
		news.ProviderCode != "BRFG" || news.ArticleID != "BRFG$1c921f7c" ||
		news.Headline != "Apple's Strong Q4 Sets Stage for Record Q1, But Investors Want More AI Clarity" ||
		news.ExtraData != "A:800015:L:en:K:0.90:C:0.897268533706665" {
		t.Fatalf("news tick = %+v, want exact captured BRFG headline", news)
	}

	sub.Close()
	if err := sub.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime() cleanup fence error = %v", err)
	}
}

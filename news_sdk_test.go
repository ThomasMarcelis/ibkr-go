package ibkr

import (
	"context"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
)

func TestSDKHistoricalNewsPublicRouteReplaysOfficialFixture(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan struct {
		items []HistoricalNewsItem
		err   error
	}, 1)
	go func() {
		items, err := client.News().Historical(ctx, HistoricalNewsRequest{
			ConID:         265598,
			ProviderCodes: []NewsProviderCode{"BRFG", "BRFUPDN", "DJNL"},
			StartTime:     time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC),
			EndTime:       time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC),
			TotalResults:  5,
		})
		resultCh <- struct {
			items []HistoricalNewsItem
			err   error
		}{items: items, err: err}
	}()

	runNextEngineCommand(t, e)
	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandHistoricalNews {
		t.Fatalf("command kind = %s, want %s", command.Kind, sdkadapter.CommandHistoricalNews)
	}
	if command.HistoricalNews.ConID != 265598 ||
		command.HistoricalNews.ProviderCodes != "BRFG+BRFUPDN+DJNL" ||
		command.HistoricalNews.TotalResults != 5 {
		t.Fatalf("historical news command = %+v, want AAPL providers and total 5", command.HistoricalNews)
	}

	path := "internal/sdkadapter/testdata/fixtures/official_sdk_news_article_snapshot_20260502.json"
	event := fixtureEvent(t, path, sdkadapter.EventHistoricalNews, 903)
	event.ReqID = command.HistoricalNews.ReqID
	dispatchSDKFixtureEvent(t, e, event)
	end := fixtureEvent(t, path, sdkadapter.EventHistoricalNewsEnd, 903)
	end.ReqID = command.HistoricalNews.ReqID
	dispatchSDKFixtureEvent(t, e, end)

	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("News().Historical() error = %v", result.err)
		}
		if len(result.items) != 1 {
			t.Fatalf("News().Historical() len = %d, want 1 replayed item", len(result.items))
		}
		item := result.items[0]
		if item.ProviderCode != "BRFG" ||
			item.ArticleID != "REDACTED_ARTICLE_ID" ||
			item.Headline != "REDACTED_HEADLINE" ||
			!item.Time.Equal(time.Date(2026, 5, 1, 14, 27, 6, 0, time.UTC)) {
			t.Fatalf("News().Historical() item = %+v, want redacted captured BRFG item", item)
		}
	case <-time.After(time.Second):
		t.Fatal("News().Historical() did not return")
	}
}

func TestSDKNewsArticlePublicRouteReplaysOfficialFixture(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan struct {
		article NewsArticle
		err     error
	}, 1)
	go func() {
		article, err := client.News().Article(ctx, NewsArticleRequest{
			ProviderCode: "BRFG",
			ArticleID:    "REDACTED_ARTICLE_ID",
		})
		resultCh <- struct {
			article NewsArticle
			err     error
		}{article: article, err: err}
	}()

	runNextEngineCommand(t, e)
	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandNewsArticle {
		t.Fatalf("command kind = %s, want %s", command.Kind, sdkadapter.CommandNewsArticle)
	}
	if command.NewsArticle.ProviderCode != "BRFG" || command.NewsArticle.ArticleID != "REDACTED_ARTICLE_ID" {
		t.Fatalf("news article command = %+v, want BRFG redacted article ID", command.NewsArticle)
	}

	event := fixtureEvent(t, "internal/sdkadapter/testdata/fixtures/official_sdk_news_article_snapshot_20260502.json", sdkadapter.EventNewsArticle, 904)
	event.ReqID = command.NewsArticle.ReqID
	dispatchSDKFixtureEvent(t, e, event)

	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("News().Article() error = %v", result.err)
		}
		if result.article.ArticleType != 0 || result.article.ArticleText != "REDACTED_ARTICLE_TEXT" {
			t.Fatalf("News().Article() = %+v, want redacted captured article body", result.article)
		}
	case <-time.After(time.Second):
		t.Fatal("News().Article() did not return")
	}
}

func TestSDKNewsOneShotsUseSDKCommandsAndEvents(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	if err := e.sendSDKContext(context.Background(), sdkadapter.NewsArticleRequest{
		ReqID:        31,
		ProviderCode: "BRFG",
		ArticleID:    "BRFG$123",
	}); err != nil {
		t.Fatalf("sendSDKContext(NewsArticleRequest) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.HistoricalNewsRequest{
		ReqID:         32,
		ConID:         265598,
		ProviderCodes: "BRFG",
		StartDate:     "20260501 00:00:00",
		EndDate:       "20260502 00:00:00",
		TotalResults:  10,
	}); err != nil {
		t.Fatalf("sendSDKContext(HistoricalNewsRequest) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 2 {
		t.Fatalf("commands len = %d, want 2", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandNewsArticle {
		t.Fatalf("news article command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandNewsArticle)
	}
	if commands[0].NewsArticle.ReqID != 31 || commands[0].NewsArticle.ProviderCode != "BRFG" || commands[0].NewsArticle.ArticleID != "BRFG$123" {
		t.Fatalf("news article command = %+v, want reqID 31 BRFG BRFG$123", commands[0].NewsArticle)
	}
	if commands[1].Kind != sdkadapter.CommandHistoricalNews {
		t.Fatalf("historical news command kind = %s, want %s", commands[1].Kind, sdkadapter.CommandHistoricalNews)
	}
	if commands[1].HistoricalNews.ReqID != 32 || commands[1].HistoricalNews.ConID != 265598 || commands[1].HistoricalNews.TotalResults != 10 {
		t.Fatalf("historical news command = %+v, want reqID 32 conID 265598 total 10", commands[1].HistoricalNews)
	}

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind:  sdkadapter.EventNewsArticle,
		ReqID: 31,
		NewsArticle: sdkadapter.NewsArticleValue{
			ArticleType: 0,
			ArticleText: "article body",
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(news article) error = %v", err)
	}
	article, ok := msg.(sdkadapter.NewsArticleResponse)
	if !ok {
		t.Fatalf("sdkEventToMessage(news article) type = %T, want sdkadapter.NewsArticleResponse", msg)
	}
	if article.ReqID != 31 || article.ArticleText != "article body" {
		t.Fatalf("news article = %+v, want reqID 31 article body", article)
	}

	msg, err = sdkEventToMessage(sdkadapter.Event{
		Kind:  sdkadapter.EventHistoricalNews,
		ReqID: 32,
		HistoricalNews: sdkadapter.HistoricalNewsValue{
			Time:         "2026-05-01 12:00:00.0",
			ProviderCode: "BRFG",
			ArticleID:    "BRFG$123",
			Headline:     "Headline",
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(historical news) error = %v", err)
	}
	item, ok := msg.(sdkadapter.HistoricalNewsItem)
	if !ok {
		t.Fatalf("sdkEventToMessage(historical news) type = %T, want sdkadapter.HistoricalNewsItem", msg)
	}
	if item.ReqID != 32 || item.ProviderCode != "BRFG" || item.ArticleID != "BRFG$123" || item.Headline != "Headline" {
		t.Fatalf("historical news item = %+v, want copied values", item)
	}

	msg, err = sdkEventToMessage(sdkadapter.Event{
		Kind:              sdkadapter.EventHistoricalNewsEnd,
		ReqID:             32,
		HistoricalHasMore: true,
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(historical news end) error = %v", err)
	}
	end, ok := msg.(sdkadapter.HistoricalNewsEnd)
	if !ok {
		t.Fatalf("sdkEventToMessage(historical news end) type = %T, want sdkadapter.HistoricalNewsEnd", msg)
	}
	if end.ReqID != 32 || !end.HasMore {
		t.Fatalf("historical news end = %+v, want reqID 32 hasMore", end)
	}
}

func TestSDKNewsBulletinsUseSDKCommandsAndEvents(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	if err := e.sendSDKContext(context.Background(), sdkadapter.NewsBulletinsRequest{AllMessages: true}); err != nil {
		t.Fatalf("sendSDKContext(NewsBulletinsRequest) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.CancelNewsBulletins{}); err != nil {
		t.Fatalf("sendSDKContext(CancelNewsBulletins) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 2 {
		t.Fatalf("commands len = %d, want 2", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandNewsBulletins || !commands[0].NewsBulletins.AllMessages {
		t.Fatalf("news bulletins command = %+v, want all messages", commands[0])
	}
	if commands[1].Kind != sdkadapter.CommandCancelNewsBulletins {
		t.Fatalf("cancel news bulletins command kind = %s, want %s", commands[1].Kind, sdkadapter.CommandCancelNewsBulletins)
	}

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind: sdkadapter.EventNewsBulletin,
		NewsBulletin: sdkadapter.NewsBulletinEvent{
			MsgID:    7,
			MsgType:  1,
			Headline: "bulletin",
			Source:   "IBKR",
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(news bulletin) error = %v", err)
	}
	got, ok := msg.(sdkadapter.NewsBulletin)
	if !ok {
		t.Fatalf("sdkEventToMessage(news bulletin) type = %T, want sdkadapter.NewsBulletin", msg)
	}
	if got.MsgID != 7 || got.MsgType != 1 || got.Headline != "bulletin" || got.Source != "IBKR" {
		t.Fatalf("news bulletin = %+v, want copied bulletin", got)
	}
}

func TestSDKNewsBulletinsPublicSubscriptionSendsSDKCancel(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan struct {
		sub *Subscription[NewsBulletin]
		err error
	}, 1)
	go func() {
		sub, err := client.News().SubscribeBulletins(ctx, true)
		resultCh <- struct {
			sub *Subscription[NewsBulletin]
			err error
		}{sub: sub, err: err}
	}()

	runNextEngineCommand(t, e)
	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandNewsBulletins ||
		!command.NewsBulletins.AllMessages {
		t.Fatalf("news bulletins command = %+v, want all-messages request", command)
	}
	result := receiveNewsBulletinsSubscriptionResult(t, resultCh)
	if result.err != nil {
		t.Fatalf("News().SubscribeBulletins() error = %v", result.err)
	}
	if err := result.sub.Close(); err != nil {
		t.Fatalf("Subscription.Close() error = %v", err)
	}

	runNextEngineCommand(t, e)
	commands := adapter.Commands()
	if len(commands) != 2 {
		t.Fatalf("commands len = %d, want request and cancel: %+v", len(commands), commands)
	}
	if commands[1].Kind != sdkadapter.CommandCancelNewsBulletins {
		t.Fatalf("news bulletins cancel command = %+v, want cancel", commands[1])
	}
}

func receiveNewsBulletinsSubscriptionResult(t *testing.T, resultCh <-chan struct {
	sub *Subscription[NewsBulletin]
	err error
}) struct {
	sub *Subscription[NewsBulletin]
	err error
} {
	t.Helper()

	select {
	case result := <-resultCh:
		return result
	case <-time.After(time.Second):
		t.Fatal("News().SubscribeBulletins() did not return")
		return struct {
			sub *Subscription[NewsBulletin]
			err error
		}{}
	}
}

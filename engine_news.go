package ibkr

import (
	"context"
	"fmt"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
)

func (e *engine) NewsProviders(ctx context.Context) ([]NewsProvider, error) {
	type result struct {
		providers []NewsProvider
		err       error
	}
	resp := make(chan result, 1)

	enqueueOneShotSetup(ctx, e, func() {
		if _, exists := e.singletons[singletonNewsProviders]; exists {
			resp <- result{err: fmt.Errorf("ibkr: news providers request already in progress")}
			return
		}

		e.singletons[singletonNewsProviders] = &route{
			opKind: OpNewsProviders,
			handle: func(msg any, eng *engine) {
				switch m := msg.(type) {
				case codec.NewsProviders:
					delete(eng.singletons, singletonNewsProviders)
					providers := make([]NewsProvider, len(m.Providers))
					for i, p := range m.Providers {
						providers[i] = NewsProvider{Code: NewsProviderCode(p.Code), Name: p.Name}
					}
					resp <- result{providers: providers}
				}
			},
			onDisconnect: func(eng *engine, err error) bool {
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.sendContext(ctx, codec.NewsProvidersRequest{}); err != nil {
			delete(e.singletons, singletonNewsProviders)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, nil)
	if err != nil {
		return nil, err
	}
	return out.providers, out.err
}

// SubscribeNewsBulletins is a singleton subscription for news bulletins.
func (e *engine) SubscribeNewsBulletins(ctx context.Context, allMessages bool, opts ...SubscriptionOption) (*Subscription[NewsBulletin], error) {
	type result struct {
		sub *Subscription[NewsBulletin]
		err error
	}
	resp := make(chan result, 1)

	enqueueSubscriptionSetup(ctx, e, resp, func() {
		if _, exists := e.singletons[singletonNewsBulletins]; exists {
			resp <- result{err: fmt.Errorf("ibkr: news bulletins subscription already active")}
			return
		}

		cfg, err := applySubscriptionOptionsFor(e.cfg, OpNewsBulletins, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		sub, ownedRoute := newSingletonSubscriptionRoute[NewsBulletin](
			e, cfg, singletonNewsBulletins, OpNewsBulletins, codec.CancelNewsBulletins{},
		)

		ownedRoute.request = codec.NewsBulletinsRequest{AllMessages: allMessages}
		ownedRoute.handle = func(msg any, e *engine) {
			if m, ok := msg.(codec.NewsBulletin); ok {
				sub.emit(NewsBulletin{MsgID: m.MsgID, MsgType: m.MsgType, Headline: m.Headline, Source: m.Source})
			}
		}
		e.singletons[singletonNewsBulletins] = ownedRoute
		sub.emitState(StreamStarted, e.connectionSeq(), nil)
		if err := e.sendContext(ctx, codec.NewsBulletinsRequest{AllMessages: allMessages}); err != nil {
			delete(e.singletons, singletonNewsBulletins)
			sub.closeWithErr(err)
			resp <- result{err: err}
			return
		}
		resp <- result{sub: sub}
	})

	out, err := awaitSubscriptionResponse(ctx, e, resp, func(out result) bool { return out.sub != nil })
	if err != nil {
		return nil, err
	}
	if out.err == nil && out.sub != nil {
		bindContext(ctx, out.sub)
	}
	return out.sub, out.err
}

func (e *engine) NewsArticle(ctx context.Context, req NewsArticleRequest) (NewsArticle, error) {
	type result struct {
		article NewsArticle
		err     error
	}
	resp := make(chan result, 1)
	var reqID int
	enqueueOneShotSetup(ctx, e, func() {
		reqID = e.allocReqID()
		e.keyed[reqID] = newKeyedOneShotRoute(reqID, OpNewsArticle,
			func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.NewsArticleResponse:
					e.deleteKeyedRoute(reqID)
					resp <- result{article: NewsArticle{ArticleType: m.ArticleType, ArticleText: m.ArticleText}}
				}
			}, func(err error) {
				resp <- result{err: err}
			})
		if err := e.sendContext(ctx, codec.NewsArticleRequest{ReqID: reqID, ProviderCode: string(req.ProviderCode), ArticleID: req.ArticleID}); err != nil {
			e.deleteKeyedRoute(reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.deleteKeyedRoute(reqID) })
	})
	if err != nil {
		return NewsArticle{}, err
	}
	return out.article, out.err
}

func (e *engine) HistoricalNews(ctx context.Context, req HistoricalNewsRequest) (HistoricalNewsResult, error) {
	if err := validateHistoricalNewsRequest(req); err != nil {
		return HistoricalNewsResult{}, err
	}
	providerCodes := formatProviderCodes(req.ProviderCodes)
	type result struct {
		page HistoricalNewsResult
		err  error
	}
	resp := make(chan result, 1)
	var reqID int
	enqueueOneShotSetup(ctx, e, func() {
		reqID = e.allocReqID()
		var collected []HistoricalNewsItem
		e.keyed[reqID] = newKeyedOneShotRoute(reqID, OpHistoricalNews,
			func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.HistoricalNewsItem:
					timestamp, err := parseHistoricalNewsTime(m.Time)
					if err != nil {
						e.deleteKeyedRoute(reqID)
						resp <- result{err: err}
						return
					}
					collected = append(collected, HistoricalNewsItem{
						Time: timestamp, ProviderCode: NewsProviderCode(m.ProviderCode),
						ArticleID: m.ArticleID, Headline: m.Headline,
					})
				case codec.HistoricalNewsEnd:
					e.deleteKeyedRoute(reqID)
					resp <- result{page: HistoricalNewsResult{Items: collected, HasMore: m.HasMore}}
				}
			}, func(err error) {
				resp <- result{err: err}
			})
		if err := e.sendContext(ctx, codec.HistoricalNewsRequest{
			ReqID: reqID, ConID: req.ConID, ProviderCodes: providerCodes,
			StartDate: formatHistoricalNewsTime(req.StartTime), EndDate: formatHistoricalNewsTime(req.EndTime), TotalResults: req.TotalResults,
		}); err != nil {
			e.deleteKeyedRoute(reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.deleteKeyedRoute(reqID) })
	})
	if err != nil {
		return HistoricalNewsResult{}, err
	}
	return out.page, out.err
}

func parseHistoricalNewsTime(raw string) (time.Time, error) {
	if ts, err := parseEpochMilliseconds(raw); err == nil {
		return ts, nil
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05.0",
		"2006-01-02 15:04:05",
	} {
		if ts, err := time.ParseInLocation(layout, raw, time.UTC); err == nil {
			return ts.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("ibkr: parse historical news time %q", raw)
}

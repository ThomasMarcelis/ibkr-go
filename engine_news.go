package ibkr

import (
	"context"
	"fmt"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
)

func (e *engine) NewsProviders(ctx context.Context) ([]NewsProvider, error) {
	type result struct {
		providers []NewsProvider
		err       error
	}
	resp := make(chan result, 1)

	enqueueOneShotSetup(ctx, e, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
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
				delete(eng.singletons, singletonNewsProviders)
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

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { delete(e.singletons, singletonNewsProviders) })
	})
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
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		if _, exists := e.singletons[singletonNewsBulletins]; exists {
			resp <- result{err: fmt.Errorf("ibkr: news bulletins subscription already active")}
			return
		}

		cfg, err := applySubscriptionOptions(e.cfg, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		if err := validateResumePolicy(OpNewsBulletins, cfg.resume); err != nil {
			resp <- result{err: err}
			return
		}
		var sub *Subscription[NewsBulletin]
		sub = newSubscription[NewsBulletin](cfg, func() {
			e.enqueue(func() {
				if _, ok := e.singletons[singletonNewsBulletins]; !ok {
					return
				}
				delete(e.singletons, singletonNewsBulletins)
				_ = e.send(codec.CancelNewsBulletins{})
				sub.closeWithErr(nil)
			})
		})

		e.singletons[singletonNewsBulletins] = &route{
			opKind:       OpNewsBulletins,
			subscription: true,
			resume:       cfg.resume,
			request:      codec.NewsBulletinsRequest{AllMessages: allMessages},
			handle: func(msg any, e *engine) {
				if m, ok := msg.(codec.NewsBulletin); ok {
					emitSubscription(sub, NewsBulletin{MsgID: m.MsgID, MsgType: m.MsgType, Headline: m.Headline, Source: m.Source})
				}
			},
			onDisconnect: func(e *engine, err error) bool {
				delete(e.singletons, singletonNewsBulletins)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) { sub.closeWithErr(err) },
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.sendContext(ctx, codec.NewsBulletinsRequest{AllMessages: allMessages}); err != nil {
			delete(e.singletons, singletonNewsBulletins)
			sub.closeWithErr(err)
			resp <- result{err: err}
			return
		}
		resp <- result{sub: sub}
	})

	out, err := awaitSubscriptionResponse(ctx, e, resp, func(out result) {
		if out.sub != nil {
			_ = out.sub.Close()
		}
	})
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
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		reqID = e.allocReqID()
		e.keyed[reqID] = &route{
			opKind: OpNewsArticle,
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.NewsArticleResponse:
					delete(e.keyed, reqID)
					resp <- result{article: NewsArticle{ArticleType: m.ArticleType, ArticleText: m.ArticleText}}
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpNewsArticle, m)}
			},
			onDisconnect: func(e *engine, err error) bool {
				delete(e.keyed, reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.sendContext(ctx, codec.NewsArticleRequest{ReqID: reqID, ProviderCode: string(req.ProviderCode), ArticleID: req.ArticleID}); err != nil {
			delete(e.keyed, reqID)
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

func (e *engine) HistoricalNews(ctx context.Context, req HistoricalNewsRequest) ([]HistoricalNewsItem, error) {
	type result struct {
		items []HistoricalNewsItem
		err   error
	}
	resp := make(chan result, 1)
	var reqID int
	enqueueOneShotSetup(ctx, e, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		reqID = e.allocReqID()
		var collected []HistoricalNewsItem
		e.keyed[reqID] = &route{
			opKind: OpHistoricalNews,
			handle: func(msg any, e *engine) {
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
					resp <- result{items: collected}
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				e.deleteKeyedRoute(reqID)
				resp <- result{err: e.apiErr(OpHistoricalNews, m)}
			},
			onDisconnect: func(e *engine, err error) bool {
				e.deleteKeyedRoute(reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.sendContext(ctx, codec.HistoricalNewsRequest{
			ReqID: reqID, ConID: req.ConID, ProviderCodes: formatProviderCodes(req.ProviderCodes),
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
		return nil, err
	}
	return out.items, out.err
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

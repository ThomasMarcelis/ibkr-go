package ibkr

import (
	"context"
	"fmt"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
	"github.com/shopspring/decimal"
)

func (e *engine) AccountSummary(ctx context.Context, req AccountSummaryRequest) ([]AccountValue, error) {
	sub, err := e.SubscribeAccountSummary(ctx, req, withSnapshotCollector())
	if err != nil {
		return nil, err
	}
	defer func() { _ = sub.Close() }()
	return collectSnapshot(ctx, sub, func(update AccountSummaryUpdate) (AccountValue, bool) { return update.Value, true })
}

func (e *engine) SubscribeAccountSummary(ctx context.Context, req AccountSummaryRequest, opts ...SubscriptionOption) (*Subscription[AccountSummaryUpdate], error) {
	req = cloneAccountSummaryRequest(req)
	type result struct {
		sub *Subscription[AccountSummaryUpdate]
		err error
	}
	resp := make(chan result, 1)

	enqueueSubscriptionSetup(ctx, e, resp, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		if e.activeAccountSummarySubscriptions() >= 2 {
			resp <- result{err: fmt.Errorf("ibkr: account summary supports at most two active subscriptions")}
			return
		}

		cfg, err := applySubscriptionOptions(e.cfg, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		if err := validateResumePolicy(OpAccountSummary, cfg.resume); err != nil {
			resp <- result{err: err}
			return
		}

		reqID := e.allocReqID()
		plan := newAccountSummaryPlan(reqID, req)
		var sub *Subscription[AccountSummaryUpdate]
		sub = newSubscription[AccountSummaryUpdate](cfg, func() {
			e.enqueue(func() {
				if _, ok := e.keyed[reqID]; !ok {
					return
				}
				e.deleteKeyedRoute(reqID)
				_ = e.send(codec.CancelAccountSummary{ReqID: reqID})
				sub.closeWithErr(nil)
			})
		})
		sub.expectSnapshot()

		e.keyed[reqID] = &route{
			opKind:       OpAccountSummary,
			subscription: true,
			resume:       cfg.resume,
			request:      plan.request,
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.AccountSummaryValue:
					if !plan.matches(m.Account) {
						return
					}
					emitSubscription(sub, AccountSummaryUpdate{
						Value: AccountValue{
							Account:  m.Account,
							Tag:      m.Tag,
							Value:    m.Value,
							Currency: m.Currency,
						},
					})
				case codec.AccountSummaryEnd:
					sub.emitState(SubscriptionStateEvent{
						Kind:          SubscriptionSnapshotComplete,
						ConnectionSeq: e.connectionSeq(),
					})
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpAccountSummary, m))
			},
			onDisconnect: func(e *engine, err error) bool {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) {
				sub.closeWithErr(err)
			},
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.sendContext(ctx, e.keyed[reqID].request); err != nil {
			e.deleteKeyedRoute(reqID)
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

func (e *engine) PositionsSnapshot(ctx context.Context) ([]Position, error) {
	sub, err := e.SubscribePositions(ctx, withSnapshotCollector())
	if err != nil {
		return nil, err
	}
	defer func() { _ = sub.Close() }()
	return collectSnapshot(ctx, sub, func(update PositionUpdate) (Position, bool) { return update.Position, true })
}

func (e *engine) SubscribePositions(ctx context.Context, opts ...SubscriptionOption) (*Subscription[PositionUpdate], error) {
	type result struct {
		sub *Subscription[PositionUpdate]
		err error
	}
	resp := make(chan result, 1)

	enqueueSubscriptionSetup(ctx, e, resp, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		if _, exists := e.singletons[singletonPositions]; exists {
			resp <- result{err: fmt.Errorf("ibkr: positions subscription already active")}
			return
		}

		cfg, err := applySubscriptionOptions(e.cfg, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		if err := validateResumePolicy(OpPositions, cfg.resume); err != nil {
			resp <- result{err: err}
			return
		}
		var sub *Subscription[PositionUpdate]
		sub = newSubscription[PositionUpdate](cfg, func() {
			e.enqueue(func() {
				if _, ok := e.singletons[singletonPositions]; !ok {
					return
				}
				delete(e.singletons, singletonPositions)
				_ = e.send(codec.CancelPositions{})
				sub.closeWithErr(nil)
			})
		})
		sub.expectSnapshot()

		e.singletons[singletonPositions] = &route{
			opKind:       OpPositions,
			subscription: true,
			resume:       cfg.resume,
			request:      codec.PositionsRequest{},
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.Position:
					position, err := fromCodecPosition(m)
					if err != nil {
						delete(e.singletons, singletonPositions)
						sub.closeWithErr(err)
						return
					}
					emitSubscription(sub, PositionUpdate{Position: position})
				case codec.PositionEnd:
					sub.emitState(SubscriptionStateEvent{Kind: SubscriptionSnapshotComplete, ConnectionSeq: e.connectionSeq()})
				}
			},
			onDisconnect: func(e *engine, err error) bool {
				delete(e.singletons, singletonPositions)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) {
				sub.closeWithErr(err)
			},
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.sendContext(ctx, codec.PositionsRequest{}); err != nil {
			delete(e.singletons, singletonPositions)
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

func (e *engine) FamilyCodes(ctx context.Context) ([]FamilyCode, error) {
	type result struct {
		codes []FamilyCode
		err   error
	}
	resp := make(chan result, 1)

	enqueueOneShotSetup(ctx, e, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		if _, exists := e.singletons[singletonFamilyCodes]; exists {
			resp <- result{err: fmt.Errorf("ibkr: family codes request already in progress")}
			return
		}

		e.singletons[singletonFamilyCodes] = &route{
			opKind: OpFamilyCodes,
			handle: func(msg any, eng *engine) {
				switch m := msg.(type) {
				case codec.FamilyCodes:
					delete(eng.singletons, singletonFamilyCodes)
					codes := make([]FamilyCode, len(m.Codes))
					for i, c := range m.Codes {
						codes[i] = FamilyCode{AccountID: c.AccountID, FamilyCode: c.FamilyCode}
					}
					resp <- result{codes: codes}
				}
			},
			onDisconnect: func(eng *engine, err error) bool {
				delete(eng.singletons, singletonFamilyCodes)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.sendContext(ctx, codec.FamilyCodesRequest{}); err != nil {
			delete(e.singletons, singletonFamilyCodes)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { delete(e.singletons, singletonFamilyCodes) })
	})
	if err != nil {
		return nil, err
	}
	return out.codes, out.err
}

// AccountUpdatesSnapshot subscribes, collects to AccountDownloadEnd, and closes.
func (e *engine) AccountUpdatesSnapshot(ctx context.Context, account string) ([]AccountUpdate, error) {
	sub, err := e.SubscribeAccountUpdates(ctx, account, withSnapshotCollector())
	if err != nil {
		return nil, err
	}
	defer func() { _ = sub.Close() }()
	return collectSnapshot(ctx, sub, func(u AccountUpdate) (AccountUpdate, bool) { return u, true })
}

// SubscribeAccountUpdates is a singleton subscription for account value/portfolio updates.
func (e *engine) SubscribeAccountUpdates(ctx context.Context, account string, opts ...SubscriptionOption) (*Subscription[AccountUpdate], error) {
	type result struct {
		sub *Subscription[AccountUpdate]
		err error
	}
	resp := make(chan result, 1)

	enqueueSubscriptionSetup(ctx, e, resp, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		if _, exists := e.singletons[singletonAccountUpdates]; exists {
			resp <- result{err: fmt.Errorf("ibkr: account updates subscription already active")}
			return
		}

		cfg, err := applySubscriptionOptions(e.cfg, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		if err := validateResumePolicy(OpAccountUpdates, cfg.resume); err != nil {
			resp <- result{err: err}
			return
		}
		var sub *Subscription[AccountUpdate]
		sub = newSubscription[AccountUpdate](cfg, func() {
			e.enqueue(func() {
				if _, ok := e.singletons[singletonAccountUpdates]; !ok {
					return
				}
				delete(e.singletons, singletonAccountUpdates)
				_ = e.send(codec.AccountUpdatesRequest{Subscribe: false, Account: account})
				sub.closeWithErr(nil)
			})
		})
		sub.expectSnapshot()

		e.singletons[singletonAccountUpdates] = &route{
			opKind:       OpAccountUpdates,
			subscription: true,
			resume:       cfg.resume,
			request:      codec.AccountUpdatesRequest{Subscribe: true, Account: account},
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.UpdateAccountValue:
					if !emitSubscription(sub, AccountUpdate{AccountValue: &AccountUpdateValue{
						Key: m.Key, Value: m.Value, Currency: m.Currency, Account: m.Account,
					}}) {
						return
					}
				case codec.UpdatePortfolio:
					position, err := parseOptionalDecimal(m.Position, "account updates position")
					if err != nil {
						delete(e.singletons, singletonAccountUpdates)
						sub.closeWithErr(err)
						return
					}
					marketPrice, err := parseOptionalDecimal(m.MarketPrice, "account updates market price")
					if err != nil {
						delete(e.singletons, singletonAccountUpdates)
						sub.closeWithErr(err)
						return
					}
					marketValue, err := parseOptionalDecimal(m.MarketValue, "account updates market value")
					if err != nil {
						delete(e.singletons, singletonAccountUpdates)
						sub.closeWithErr(err)
						return
					}
					avgCost, err := parseOptionalDecimal(m.AvgCost, "account updates average cost")
					if err != nil {
						delete(e.singletons, singletonAccountUpdates)
						sub.closeWithErr(err)
						return
					}
					unrealizedPNL, err := parseOptionalDecimal(m.UnrealizedPNL, "account updates unrealized pnl")
					if err != nil {
						delete(e.singletons, singletonAccountUpdates)
						sub.closeWithErr(err)
						return
					}
					realizedPNL, err := parseOptionalDecimal(m.RealizedPNL, "account updates realized pnl")
					if err != nil {
						delete(e.singletons, singletonAccountUpdates)
						sub.closeWithErr(err)
						return
					}
					emitSubscription(sub, AccountUpdate{Portfolio: &PortfolioUpdate{
						Account:       m.Account,
						Contract:      fromCodecContract(m.Contract),
						Position:      position,
						MarketPrice:   marketPrice,
						MarketValue:   marketValue,
						AvgCost:       avgCost,
						UnrealizedPNL: unrealizedPNL,
						RealizedPNL:   realizedPNL,
					}})
				case codec.UpdateAccountTime:
					// Informational timestamp — silently consumed.
				case codec.AccountDownloadEnd:
					sub.emitState(SubscriptionStateEvent{Kind: SubscriptionSnapshotComplete, ConnectionSeq: e.connectionSeq()})
				}
			},
			onDisconnect: func(e *engine, err error) bool {
				delete(e.singletons, singletonAccountUpdates)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) { sub.closeWithErr(err) },
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.sendContext(ctx, codec.AccountUpdatesRequest{Subscribe: true, Account: account}); err != nil {
			delete(e.singletons, singletonAccountUpdates)
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

// AccountUpdatesMultiSnapshot subscribes, collects to end marker, and closes.
func (e *engine) AccountUpdatesMultiSnapshot(ctx context.Context, req AccountUpdatesMultiRequest) ([]AccountUpdateMultiValue, error) {
	sub, err := e.SubscribeAccountUpdatesMulti(ctx, req, withSnapshotCollector())
	if err != nil {
		return nil, err
	}
	defer func() { _ = sub.Close() }()
	return collectSnapshot(ctx, sub, func(u AccountUpdateMultiValue) (AccountUpdateMultiValue, bool) { return u, true })
}

func (e *engine) SubscribeAccountUpdatesMulti(ctx context.Context, req AccountUpdatesMultiRequest, opts ...SubscriptionOption) (*Subscription[AccountUpdateMultiValue], error) {
	type result struct {
		sub *Subscription[AccountUpdateMultiValue]
		err error
	}
	resp := make(chan result, 1)

	enqueueSubscriptionSetup(ctx, e, resp, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}

		cfg, err := applySubscriptionOptions(e.cfg, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		if err := validateResumePolicy(OpAccountUpdatesMulti, cfg.resume); err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		var sub *Subscription[AccountUpdateMultiValue]
		sub = newSubscription[AccountUpdateMultiValue](cfg, func() {
			e.enqueue(func() {
				if _, ok := e.keyed[reqID]; !ok {
					return
				}
				e.deleteKeyedRoute(reqID)
				_ = e.send(codec.CancelAccountUpdatesMulti{ReqID: reqID})
				sub.closeWithErr(nil)
			})
		})
		sub.expectSnapshot()

		e.keyed[reqID] = &route{
			opKind:       OpAccountUpdatesMulti,
			subscription: true,
			resume:       cfg.resume,
			request:      codec.AccountUpdatesMultiRequest{ReqID: reqID, Account: req.Account, ModelCode: req.ModelCode},
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.AccountUpdateMultiValue:
					emitSubscription(sub, AccountUpdateMultiValue{
						Account: m.Account, ModelCode: m.ModelCode,
						Key: m.Key, Value: m.Value, Currency: m.Currency,
					})
				case codec.AccountUpdateMultiEnd:
					sub.emitState(SubscriptionStateEvent{Kind: SubscriptionSnapshotComplete, ConnectionSeq: e.connectionSeq()})
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpAccountUpdatesMulti, m))
			},
			onDisconnect: func(e *engine, err error) bool {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) { sub.closeWithErr(err) },
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.sendContext(ctx, e.keyed[reqID].request); err != nil {
			e.deleteKeyedRoute(reqID)
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

// PositionsMultiSnapshot subscribes, collects to end marker, and closes.
func (e *engine) PositionsMultiSnapshot(ctx context.Context, req PositionsMultiRequest) ([]PositionMulti, error) {
	sub, err := e.SubscribePositionsMulti(ctx, req, withSnapshotCollector())
	if err != nil {
		return nil, err
	}
	defer func() { _ = sub.Close() }()
	return collectSnapshot(ctx, sub, func(u PositionMulti) (PositionMulti, bool) { return u, true })
}

func (e *engine) SubscribePositionsMulti(ctx context.Context, req PositionsMultiRequest, opts ...SubscriptionOption) (*Subscription[PositionMulti], error) {
	type result struct {
		sub *Subscription[PositionMulti]
		err error
	}
	resp := make(chan result, 1)

	enqueueSubscriptionSetup(ctx, e, resp, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}

		cfg, err := applySubscriptionOptions(e.cfg, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		if err := validateResumePolicy(OpPositionsMulti, cfg.resume); err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		var sub *Subscription[PositionMulti]
		sub = newSubscription[PositionMulti](cfg, func() {
			e.enqueue(func() {
				if _, ok := e.keyed[reqID]; !ok {
					return
				}
				e.deleteKeyedRoute(reqID)
				_ = e.send(codec.CancelPositionsMulti{ReqID: reqID})
				sub.closeWithErr(nil)
			})
		})
		sub.expectSnapshot()

		e.keyed[reqID] = &route{
			opKind:       OpPositionsMulti,
			subscription: true,
			resume:       cfg.resume,
			request:      codec.PositionsMultiRequest{ReqID: reqID, Account: req.Account, ModelCode: req.ModelCode},
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.PositionMulti:
					position, err := parseRequiredDecimal(m.Position, "positions multi position")
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					avgCost, err := parseRequiredDecimal(m.AvgCost, "positions multi average cost")
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					emitSubscription(sub, PositionMulti{
						Account: m.Account, ModelCode: m.ModelCode,
						Contract: fromCodecContract(m.Contract),
						Position: position, AvgCost: avgCost,
					})
				case codec.PositionMultiEnd:
					sub.emitState(SubscriptionStateEvent{Kind: SubscriptionSnapshotComplete, ConnectionSeq: e.connectionSeq()})
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpPositionsMulti, m))
			},
			onDisconnect: func(e *engine, err error) bool {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) { sub.closeWithErr(err) },
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.sendContext(ctx, e.keyed[reqID].request); err != nil {
			e.deleteKeyedRoute(reqID)
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

func (e *engine) SubscribePnL(ctx context.Context, req PnLRequest, opts ...SubscriptionOption) (*Subscription[PnLUpdate], error) {
	type result struct {
		sub *Subscription[PnLUpdate]
		err error
	}
	resp := make(chan result, 1)

	enqueueSubscriptionSetup(ctx, e, resp, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}

		cfg, err := applySubscriptionOptions(e.cfg, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		if err := validateResumePolicy(OpPnL, cfg.resume); err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		var sub *Subscription[PnLUpdate]
		sub = newSubscription[PnLUpdate](cfg, func() {
			e.enqueue(func() {
				if _, ok := e.keyed[reqID]; !ok {
					return
				}
				e.deleteKeyedRoute(reqID)
				_ = e.send(codec.CancelPnL{ReqID: reqID})
				sub.closeWithErr(nil)
			})
		})

		e.keyed[reqID] = &route{
			opKind:       OpPnL,
			subscription: true,
			resume:       cfg.resume,
			request:      codec.PnLRequest{ReqID: reqID, Account: req.Account, ModelCode: req.ModelCode},
			handle: func(msg any, e *engine) {
				if m, ok := msg.(codec.PnLValue); ok {
					daily, err := parseOptionalDecimal(m.DailyPnL, "pnl daily")
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					unrealized, err := parseOptionalDecimal(m.UnrealizedPnL, "pnl unrealized")
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					realized, err := parseOptionalDecimal(m.RealizedPnL, "pnl realized")
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					emitSubscription(sub, PnLUpdate{DailyPnL: daily, UnrealizedPnL: unrealized, RealizedPnL: realized})
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpPnL, m))
			},
			onDisconnect: func(e *engine, err error) bool {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) { sub.closeWithErr(err) },
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.sendContext(ctx, e.keyed[reqID].request); err != nil {
			e.deleteKeyedRoute(reqID)
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

func (e *engine) SubscribePnLSingle(ctx context.Context, req PnLSingleRequest, opts ...SubscriptionOption) (*Subscription[PnLSingleUpdate], error) {
	type result struct {
		sub *Subscription[PnLSingleUpdate]
		err error
	}
	resp := make(chan result, 1)

	enqueueSubscriptionSetup(ctx, e, resp, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}

		cfg, err := applySubscriptionOptions(e.cfg, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		if err := validateResumePolicy(OpPnLSingle, cfg.resume); err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		var sub *Subscription[PnLSingleUpdate]
		sub = newSubscription[PnLSingleUpdate](cfg, func() {
			e.enqueue(func() {
				if _, ok := e.keyed[reqID]; !ok {
					return
				}
				e.deleteKeyedRoute(reqID)
				_ = e.send(codec.CancelPnLSingle{ReqID: reqID})
				sub.closeWithErr(nil)
			})
		})

		e.keyed[reqID] = &route{
			opKind:       OpPnLSingle,
			subscription: true,
			resume:       cfg.resume,
			request:      codec.PnLSingleRequest{ReqID: reqID, Account: req.Account, ModelCode: req.ModelCode, ConID: req.ConID},
			handle: func(msg any, e *engine) {
				if m, ok := msg.(codec.PnLSingleValue); ok {
					pos, err := parseOptionalDecimal(m.Position, "pnl single position")
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					daily, err := parseOptionalDecimal(m.DailyPnL, "pnl single daily")
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					unrealized, err := parseOptionalDecimal(m.UnrealizedPnL, "pnl single unrealized")
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					realized, err := parseOptionalDecimal(m.RealizedPnL, "pnl single realized")
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					value, err := parseOptionalDecimal(m.Value, "pnl single value")
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					emitSubscription(sub, PnLSingleUpdate{Position: pos, DailyPnL: daily, UnrealizedPnL: unrealized, RealizedPnL: realized, Value: value})
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpPnLSingle, m))
			},
			onDisconnect: func(e *engine, err error) bool {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) { sub.closeWithErr(err) },
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.sendContext(ctx, e.keyed[reqID].request); err != nil {
			e.deleteKeyedRoute(reqID)
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

func fromCodecPosition(m codec.Position) (Position, error) {
	position, err := decimal.NewFromString(m.Position)
	if err != nil {
		return Position{}, err
	}
	avgCost, err := decimal.NewFromString(m.AvgCost)
	if err != nil {
		return Position{}, err
	}
	return Position{
		Account:  m.Account,
		Contract: fromCodecContract(m.Contract),
		Position: position,
		AvgCost:  avgCost,
	}, nil
}

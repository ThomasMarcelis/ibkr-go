package ibkr

import (
	"context"
	"fmt"
	"strings"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/shopspring/decimal"
)

func (e *engine) AccountSummary(ctx context.Context, req AccountSummaryRequest) ([]AccountValue, error) {
	sub, err := e.SubscribeAccountSummary(ctx, req, withSnapshotCollector())
	if err != nil {
		return nil, err
	}
	return collectSnapshotAndClose(ctx, sub, func(value AccountValue) (AccountValue, bool) { return value, true })
}

func (e *engine) SubscribeAccountSummary(ctx context.Context, req AccountSummaryRequest, opts ...SubscriptionOption) (*Subscription[AccountValue], error) {
	req = cloneAccountSummaryRequest(req)
	if len(req.Tags) == 0 {
		return nil, &ValidationError{Field: "AccountSummaryRequest.Tags", Message: "must contain at least one tag"}
	}
	for i, tag := range req.Tags {
		if strings.TrimSpace(tag) == "" {
			return nil, &ValidationError{Field: fmt.Sprintf("AccountSummaryRequest.Tags[%d]", i), Message: "must not be empty"}
		}
	}
	type result struct {
		sub *Subscription[AccountValue]
		err error
	}
	resp := make(chan result, 1)

	enqueueSubscriptionSetup(ctx, e, resp, func() {
		if e.activeAccountSummarySubscriptions() >= 2 {
			resp <- result{err: operationActive("account summary subscription limit reached")}
			return
		}

		cfg, err := applySubscriptionOptionsFor(e.cfg, OpAccountSummary, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}

		reqID := e.allocReqID()
		plan := newAccountSummaryPlan(reqID, req)
		sub, ownedRoute := newKeyedSubscriptionRoute[AccountValue](
			e, cfg, reqID, OpAccountSummary, codec.CancelAccountSummary{ReqID: reqID},
		)
		sub.expectSnapshot()

		ownedRoute.request = plan.request
		ownedRoute.handle = func(msg any, e *engine) {
			switch m := msg.(type) {
			case codec.AccountSummaryValue:
				if !plan.matches(m.Account) {
					return
				}
				sub.emit(AccountValue{
					Account:  m.Account,
					Tag:      m.Tag,
					Value:    m.Value,
					Currency: m.Currency,
				})
			case codec.AccountSummaryEnd:
				sub.emitState(StreamSnapshotComplete, e.connectionSeq(), nil)
			}
		}
		e.keyed[reqID] = ownedRoute
		sub.emitState(StreamStarted, e.connectionSeq(), nil)
		if err := e.sendContext(ctx, e.keyed[reqID].request); err != nil {
			e.deleteKeyedRoute(reqID)
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

func (e *engine) PositionsSnapshot(ctx context.Context) ([]Position, error) {
	sub, err := e.SubscribePositions(ctx, withSnapshotCollector())
	if err != nil {
		return nil, err
	}
	return collectSnapshotAndClose(ctx, sub, func(position Position) (Position, bool) { return position, true })
}

func (e *engine) ManagedAccounts(ctx context.Context) ([]string, error) {
	type result struct {
		accounts []string
		err      error
	}
	resp := make(chan result, 1)
	var ownedRoute *route

	enqueueOneShotSetup(ctx, e, func() {
		if _, exists := e.singletons[singletonManagedAccounts]; exists {
			resp <- result{err: operationActive("managed accounts")}
			return
		}

		ownedRoute = &route{
			opKind: OpManagedAccounts,
			handle: func(msg any, eng *engine) {
				m, ok := msg.(codec.ManagedAccounts)
				if !ok {
					return
				}
				delete(eng.singletons, singletonManagedAccounts)
				resp <- result{accounts: append([]string(nil), m.Accounts...)}
			},
			onDisconnect: func(eng *engine, err error) bool {
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		e.singletons[singletonManagedAccounts] = ownedRoute
		if err := e.sendContext(ctx, codec.ManagedAccountsRequest{}); err != nil {
			delete(e.singletons, singletonManagedAccounts)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.cancelSingletonOneShot(singletonManagedAccounts, ownedRoute) })
	})
	if err != nil {
		return nil, err
	}
	return out.accounts, out.err
}

func (e *engine) SubscribePositions(ctx context.Context, opts ...SubscriptionOption) (*Subscription[Position], error) {
	type result struct {
		sub *Subscription[Position]
		err error
	}
	resp := make(chan result, 1)

	enqueueSubscriptionSetup(ctx, e, resp, func() {
		if _, exists := e.singletons[singletonPositions]; exists {
			resp <- result{err: operationActive("positions subscription")}
			return
		}

		cfg, err := applySubscriptionOptionsFor(e.cfg, OpPositions, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		sub, ownedRoute := newSingletonSubscriptionRoute[Position](
			e, cfg, singletonPositions, OpPositions, codec.CancelPositions{},
		)
		sub.expectSnapshot()

		ownedRoute.request = codec.PositionsRequest{}
		ownedRoute.handle = func(msg any, e *engine) {
			switch m := msg.(type) {
			case codec.Position:
				position, err := fromCodecPosition(m)
				if err != nil {
					sub.cancelFromActor(err)
					return
				}
				sub.emit(position)
			case codec.PositionEnd:
				sub.emitState(StreamSnapshotComplete, e.connectionSeq(), nil)
			}
		}
		e.singletons[singletonPositions] = ownedRoute
		sub.emitState(StreamStarted, e.connectionSeq(), nil)
		if err := e.sendContext(ctx, codec.PositionsRequest{}); err != nil {
			delete(e.singletons, singletonPositions)
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

func (e *engine) FamilyCodes(ctx context.Context) ([]FamilyCode, error) {
	type result struct {
		codes []FamilyCode
		err   error
	}
	resp := make(chan result, 1)
	var ownedRoute *route

	enqueueOneShotSetup(ctx, e, func() {
		if _, exists := e.singletons[singletonFamilyCodes]; exists {
			resp <- result{err: operationActive("family codes")}
			return
		}

		ownedRoute = &route{
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
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		e.singletons[singletonFamilyCodes] = ownedRoute
		if err := e.sendContext(ctx, codec.FamilyCodesRequest{}); err != nil {
			delete(e.singletons, singletonFamilyCodes)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.cancelSingletonOneShot(singletonFamilyCodes, ownedRoute) })
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
	return collectSnapshotAndClose(ctx, sub, func(u AccountUpdate) (AccountUpdate, bool) { return u, true })
}

// SubscribeAccountUpdates is a singleton subscription for account value/portfolio updates.
func (e *engine) SubscribeAccountUpdates(ctx context.Context, account string, opts ...SubscriptionOption) (*Subscription[AccountUpdate], error) {
	type result struct {
		sub *Subscription[AccountUpdate]
		err error
	}
	resp := make(chan result, 1)

	enqueueSubscriptionSetup(ctx, e, resp, func() {
		if _, exists := e.singletons[singletonAccountUpdates]; exists {
			resp <- result{err: operationActive("account updates subscription")}
			return
		}

		cfg, err := applySubscriptionOptionsFor(e.cfg, OpAccountUpdates, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		sub, ownedRoute := newSingletonSubscriptionRoute[AccountUpdate](
			e, cfg, singletonAccountUpdates, OpAccountUpdates,
			codec.AccountUpdatesRequest{Subscribe: false, Account: account},
		)
		sub.expectSnapshot()

		ownedRoute.request = codec.AccountUpdatesRequest{Subscribe: true, Account: account}
		ownedRoute.handle = func(msg any, e *engine) {
			switch m := msg.(type) {
			case codec.UpdateAccountValue:
				if !sub.emit(AccountUpdate{AccountValue: &AccountUpdateValue{
					Key: m.Key, Value: m.Value, Currency: m.Currency, Account: m.Account,
				}}) {
					return
				}
			case codec.UpdatePortfolio:
				fail := func(err error) { sub.cancelFromActor(err) }
				contract, err := fromCodecContract(m.Contract)
				if err != nil {
					fail(err)
					return
				}
				position, err := parseOptionalDecimal(m.Position, "account updates position")
				if err != nil {
					fail(err)
					return
				}
				marketPrice, err := parseOptionalDecimalPointer(m.MarketPrice, "account updates market price")
				if err != nil {
					fail(err)
					return
				}
				marketValue, err := parseOptionalDecimalPointer(m.MarketValue, "account updates market value")
				if err != nil {
					fail(err)
					return
				}
				avgCost, err := parseOptionalDecimalPointer(m.AvgCost, "account updates average cost")
				if err != nil {
					fail(err)
					return
				}
				unrealizedPNL, err := parseOptionalDecimalPointer(m.UnrealizedPNL, "account updates unrealized pnl")
				if err != nil {
					fail(err)
					return
				}
				realizedPNL, err := parseOptionalDecimalPointer(m.RealizedPNL, "account updates realized pnl")
				if err != nil {
					fail(err)
					return
				}
				sub.emit(AccountUpdate{Portfolio: &PortfolioUpdate{
					Account:       m.Account,
					Contract:      contract,
					Position:      position,
					MarketPrice:   marketPrice,
					MarketValue:   marketValue,
					AvgCost:       avgCost,
					UnrealizedPnL: unrealizedPNL,
					RealizedPnL:   realizedPNL,
				}})
			case codec.UpdateAccountTime:
				sub.emit(AccountUpdate{UpdateTime: new(m.Timestamp)})
			case codec.AccountDownloadEnd:
				sub.emitState(StreamSnapshotComplete, e.connectionSeq(), nil)
			}
		}
		e.singletons[singletonAccountUpdates] = ownedRoute
		sub.emitState(StreamStarted, e.connectionSeq(), nil)
		if err := e.sendContext(ctx, codec.AccountUpdatesRequest{Subscribe: true, Account: account}); err != nil {
			delete(e.singletons, singletonAccountUpdates)
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

// AccountUpdatesMultiSnapshot subscribes, collects to end marker, and closes.
func (e *engine) AccountUpdatesMultiSnapshot(ctx context.Context, req AccountUpdatesMultiRequest) ([]AccountUpdateMultiValue, error) {
	sub, err := e.SubscribeAccountUpdatesMulti(ctx, req, withSnapshotCollector())
	if err != nil {
		return nil, err
	}
	return collectSnapshotAndClose(ctx, sub, func(u AccountUpdateMultiValue) (AccountUpdateMultiValue, bool) { return u, true })
}

func (e *engine) SubscribeAccountUpdatesMulti(ctx context.Context, req AccountUpdatesMultiRequest, opts ...SubscriptionOption) (*Subscription[AccountUpdateMultiValue], error) {
	type result struct {
		sub *Subscription[AccountUpdateMultiValue]
		err error
	}
	resp := make(chan result, 1)

	enqueueSubscriptionSetup(ctx, e, resp, func() {
		cfg, err := applySubscriptionOptionsFor(e.cfg, OpAccountUpdatesMulti, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		sub, ownedRoute := newKeyedSubscriptionRoute[AccountUpdateMultiValue](
			e, cfg, reqID, OpAccountUpdatesMulti, codec.CancelAccountUpdatesMulti{ReqID: reqID},
		)
		sub.expectSnapshot()

		ownedRoute.request = codec.AccountUpdatesMultiRequest{
			ReqID: reqID, Account: req.Account, ModelCode: req.ModelCode, LedgerAndNLV: req.LedgerAndNLV,
		}
		ownedRoute.handle = func(msg any, e *engine) {
			switch m := msg.(type) {
			case codec.AccountUpdateMultiValue:
				sub.emit(AccountUpdateMultiValue{
					Account: m.Account, ModelCode: m.ModelCode,
					Key: m.Key, Value: m.Value, Currency: m.Currency,
				})
			case codec.AccountUpdateMultiEnd:
				sub.emitState(StreamSnapshotComplete, e.connectionSeq(), nil)
			}
		}
		e.keyed[reqID] = ownedRoute
		sub.emitState(StreamStarted, e.connectionSeq(), nil)
		if err := e.sendContext(ctx, e.keyed[reqID].request); err != nil {
			e.deleteKeyedRoute(reqID)
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

// PositionsMultiSnapshot subscribes, collects to end marker, and closes.
func (e *engine) PositionsMultiSnapshot(ctx context.Context, req PositionsMultiRequest) ([]PositionMulti, error) {
	sub, err := e.SubscribePositionsMulti(ctx, req, withSnapshotCollector())
	if err != nil {
		return nil, err
	}
	return collectSnapshotAndClose(ctx, sub, func(u PositionMulti) (PositionMulti, bool) { return u, true })
}

func (e *engine) SubscribePositionsMulti(ctx context.Context, req PositionsMultiRequest, opts ...SubscriptionOption) (*Subscription[PositionMulti], error) {
	type result struct {
		sub *Subscription[PositionMulti]
		err error
	}
	resp := make(chan result, 1)

	enqueueSubscriptionSetup(ctx, e, resp, func() {
		cfg, err := applySubscriptionOptionsFor(e.cfg, OpPositionsMulti, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		sub, ownedRoute := newKeyedSubscriptionRoute[PositionMulti](
			e, cfg, reqID, OpPositionsMulti, codec.CancelPositionsMulti{ReqID: reqID},
		)
		sub.expectSnapshot()

		ownedRoute.request = codec.PositionsMultiRequest{ReqID: reqID, Account: req.Account, ModelCode: req.ModelCode}
		ownedRoute.handle = func(msg any, e *engine) {
			switch m := msg.(type) {
			case codec.PositionMulti:
				fail := func(err error) { sub.cancelFromActor(err) }
				contract, err := fromCodecContract(m.Contract)
				if err != nil {
					fail(err)
					return
				}
				position, err := parseRequiredDecimal(m.Position, "positions multi position")
				if err != nil {
					fail(err)
					return
				}
				avgCost, err := parseRequiredDecimal(m.AvgCost, "positions multi average cost")
				if err != nil {
					fail(err)
					return
				}
				sub.emit(PositionMulti{
					Account: m.Account, ModelCode: m.ModelCode,
					Contract: contract,
					Position: position, AvgCost: avgCost,
				})
			case codec.PositionMultiEnd:
				sub.emitState(StreamSnapshotComplete, e.connectionSeq(), nil)
			}
		}
		e.keyed[reqID] = ownedRoute
		sub.emitState(StreamStarted, e.connectionSeq(), nil)
		if err := e.sendContext(ctx, e.keyed[reqID].request); err != nil {
			e.deleteKeyedRoute(reqID)
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

func (e *engine) SubscribePnL(ctx context.Context, req PnLRequest, opts ...SubscriptionOption) (*Subscription[PnLUpdate], error) {
	type result struct {
		sub *Subscription[PnLUpdate]
		err error
	}
	resp := make(chan result, 1)

	enqueueSubscriptionSetup(ctx, e, resp, func() {
		cfg, err := applySubscriptionOptionsFor(e.cfg, OpPnL, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		sub, ownedRoute := newKeyedSubscriptionRoute[PnLUpdate](
			e, cfg, reqID, OpPnL, codec.CancelPnL{ReqID: reqID},
		)

		ownedRoute.request = codec.PnLRequest{ReqID: reqID, Account: req.Account, ModelCode: req.ModelCode}
		ownedRoute.handle = func(msg any, e *engine) {
			if m, ok := msg.(codec.PnLValue); ok {
				fail := func(err error) { sub.cancelFromActor(err) }
				daily, err := parseOptionalDecimalPointer(m.DailyPnL, "pnl daily")
				if err != nil {
					fail(err)
					return
				}
				unrealized, err := parseOptionalDecimalPointer(m.UnrealizedPnL, "pnl unrealized")
				if err != nil {
					fail(err)
					return
				}
				realized, err := parseOptionalDecimalPointer(m.RealizedPnL, "pnl realized")
				if err != nil {
					fail(err)
					return
				}
				sub.emit(PnLUpdate{DailyPnL: daily, UnrealizedPnL: unrealized, RealizedPnL: realized})
			}
		}
		e.keyed[reqID] = ownedRoute
		sub.emitState(StreamStarted, e.connectionSeq(), nil)
		if err := e.sendContext(ctx, e.keyed[reqID].request); err != nil {
			e.deleteKeyedRoute(reqID)
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

func (e *engine) SubscribePnLSingle(ctx context.Context, req PnLSingleRequest, opts ...SubscriptionOption) (*Subscription[PnLSingleUpdate], error) {
	type result struct {
		sub *Subscription[PnLSingleUpdate]
		err error
	}
	resp := make(chan result, 1)

	enqueueSubscriptionSetup(ctx, e, resp, func() {
		cfg, err := applySubscriptionOptionsFor(e.cfg, OpPnLSingle, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		sub, ownedRoute := newKeyedSubscriptionRoute[PnLSingleUpdate](
			e, cfg, reqID, OpPnLSingle, codec.CancelPnLSingle{ReqID: reqID},
		)

		ownedRoute.request = codec.PnLSingleRequest{ReqID: reqID, Account: req.Account, ModelCode: req.ModelCode, ConID: req.ConID}
		ownedRoute.handle = func(msg any, e *engine) {
			if m, ok := msg.(codec.PnLSingleValue); ok {
				fail := func(err error) { sub.cancelFromActor(err) }
				pos, err := parseOptionalDecimal(m.Position, "pnl single position")
				if err != nil {
					fail(err)
					return
				}
				daily, err := parseOptionalDecimalPointer(m.DailyPnL, "pnl single daily")
				if err != nil {
					fail(err)
					return
				}
				unrealized, err := parseOptionalDecimalPointer(m.UnrealizedPnL, "pnl single unrealized")
				if err != nil {
					fail(err)
					return
				}
				realized, err := parseOptionalDecimalPointer(m.RealizedPnL, "pnl single realized")
				if err != nil {
					fail(err)
					return
				}
				value, err := parseOptionalDecimalPointer(m.Value, "pnl single value")
				if err != nil {
					fail(err)
					return
				}
				sub.emit(PnLSingleUpdate{Position: pos, DailyPnL: daily, UnrealizedPnL: unrealized, RealizedPnL: realized, Value: value})
			}
		}
		e.keyed[reqID] = ownedRoute
		sub.emitState(StreamStarted, e.connectionSeq(), nil)
		if err := e.sendContext(ctx, e.keyed[reqID].request); err != nil {
			e.deleteKeyedRoute(reqID)
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

func fromCodecPosition(m codec.Position) (Position, error) {
	contract, err := fromCodecContract(m.Contract)
	if err != nil {
		return Position{}, err
	}
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
		Contract: contract,
		Position: position,
		AvgCost:  avgCost,
	}, nil
}

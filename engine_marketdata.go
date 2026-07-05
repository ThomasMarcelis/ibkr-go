package ibkr

import (
	"context"
	"fmt"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
	"github.com/shopspring/decimal"
)

func (e *engine) SetMarketDataType(ctx context.Context, dataType MarketDataType) error {
	if dataType < MarketDataLive || dataType > MarketDataDelayedFrozen {
		return fmt.Errorf("invalid market data type %d: must be 1 (live), 2 (frozen), 3 (delayed), or 4 (delayed-frozen)", dataType)
	}
	return awaitFireAndForget(ctx, e, func(ctx context.Context) error {
		if !e.isReady() {
			return ErrNotReady
		}
		return e.sendContext(ctx, codec.ReqMarketDataType{DataType: int(dataType)})
	})
}

func (e *engine) QuoteSnapshot(ctx context.Context, req QuoteRequest) (Quote, error) {
	sub, err := e.subscribeQuotes(ctx, req, true)
	if err != nil {
		return Quote{}, err
	}
	defer func() { _ = sub.Close() }()

	var latest Quote
	for {
		select {
		case update, ok := <-sub.Events():
			if !ok {
				return latest, sub.Wait()
			}
			latest = update.Snapshot
		case state, ok := <-sub.Lifecycle():
			if !ok {
				return latest, sub.Wait()
			}
			if state.Kind == SubscriptionSnapshotComplete {
				for {
					select {
					case update, ok := <-sub.Events():
						if !ok {
							return latest, sub.Wait()
						}
						latest = update.Snapshot
					default:
						return latest, nil
					}
				}
			}
			if state.Kind == SubscriptionClosed && state.Err != nil {
				return Quote{}, state.Err
			}
		case <-ctx.Done():
			return Quote{}, ctx.Err()
		}
	}
}

func (e *engine) SubscribeQuotes(ctx context.Context, req QuoteRequest, opts ...SubscriptionOption) (*Subscription[QuoteUpdate], error) {
	return e.subscribeQuotes(ctx, req, false, opts...)
}

func (e *engine) subscribeQuotes(ctx context.Context, req QuoteRequest, snapshot bool, opts ...SubscriptionOption) (*Subscription[QuoteUpdate], error) {
	type result struct {
		sub *Subscription[QuoteUpdate]
		err error
	}
	resp := make(chan result, 1)

	enqueueSubscriptionSetup(ctx, e, resp, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}

		cfg := defaultSubscriptionConfig(e.cfg)
		for _, opt := range opts {
			opt(&cfg)
		}
		if err := validateResumePolicy(OpQuotes, cfg.resume); err != nil {
			resp <- result{err: err}
			return
		}
		if err := validateQuoteRequest(req, snapshot, cfg.resume); err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		var sub *Subscription[QuoteUpdate]
		sub = newSubscription[QuoteUpdate](cfg, func() {
			e.enqueue(func() {
				if _, ok := e.keyed[reqID]; !ok {
					return
				}
				e.deleteKeyedRoute(reqID)
				_ = e.send(codec.CancelQuote{ReqID: reqID})
				sub.closeWithErr(nil)
			})
		})
		if snapshot {
			sub.expectSnapshot()
		}
		quote := Quote{}

		e.keyed[reqID] = &route{
			opKind:       OpQuotes,
			subscription: true,
			resume:       cfg.resume,
			request: codec.QuoteRequest{
				ReqID:        reqID,
				Contract:     toCodecContract(req.Contract),
				Snapshot:     snapshot,
				GenericTicks: formatGenericTicks(req.GenericTicks),
			},
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.TickPrice:
					changed, err := applyTickPrice(&quote, m.TickType, m.Price)
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					emitSubscription(sub, QuoteUpdate{Snapshot: quote, Changed: changed, ReceivedAt: time.Now().UTC()})
				case codec.TickSize:
					changed, err := applyTickSize(&quote, m.TickType, m.Size)
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					emitSubscription(sub, QuoteUpdate{Snapshot: quote, Changed: changed, ReceivedAt: time.Now().UTC()})
				case codec.MarketDataType:
					quote.MarketDataType = MarketDataType(m.DataType)
					quote.Available |= QuoteFieldMarketDataType
					emitSubscription(sub, QuoteUpdate{Snapshot: quote, Changed: QuoteFieldMarketDataType, ReceivedAt: time.Now().UTC()})
				case codec.TickGeneric:
					// Generic ticks carry informational data (e.g. halted status).
					// Silently consumed — no standard quote field mapping.
				case codec.TickString:
					// String ticks carry informational data (e.g. last timestamp).
					// Silently consumed — no standard quote field mapping.
				case codec.TickReqParams:
					// Tick request params are informational (minTick, BBO exchange).
					// Silently consumed.
				case codec.TickSnapshotEnd:
					sub.emitState(SubscriptionStateEvent{Kind: SubscriptionSnapshotComplete, ConnectionSeq: e.connectionSeq()})
					if snapshot {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(nil)
					}
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				// 10167: delayed market data warning — the subscription
				// stays open and will receive delayed ticks.
				if m.Code == 10167 {
					e.emitEvent(m.Code, m.Message)
					return
				}
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpQuotes, m))
			},
			onDisconnect: func(e *engine, err error) bool {
				if snapshot {
					e.deleteKeyedRoute(reqID)
					sub.closeWithErr(ErrInterrupted)
					return false
				}
				if cfg.resume == ResumeAuto && e.cfg.reconnect == ReconnectAuto {
					sub.emitState(SubscriptionStateEvent{
						Kind:          SubscriptionGap,
						ConnectionSeq: e.connectionSeq(),
						Err:           err,
					})
					return true
				}
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			emitGap: func(e *engine) {
				sub.emitState(SubscriptionStateEvent{
					Kind:          SubscriptionGap,
					ConnectionSeq: e.connectionSeq(),
				})
			},
			emitResumed: func(e *engine) {
				sub.emitState(SubscriptionStateEvent{
					Kind:          SubscriptionResumed,
					ConnectionSeq: e.connectionSeq(),
				})
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

func (e *engine) SubscribeRealTimeBars(ctx context.Context, req RealTimeBarsRequest, opts ...SubscriptionOption) (*Subscription[Bar], error) {
	type result struct {
		sub *Subscription[Bar]
		err error
	}
	resp := make(chan result, 1)

	enqueueSubscriptionSetup(ctx, e, resp, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}

		cfg := defaultSubscriptionConfig(e.cfg)
		for _, opt := range opts {
			opt(&cfg)
		}
		if err := validateResumePolicy(OpRealTimeBars, cfg.resume); err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		var sub *Subscription[Bar]
		sub = newSubscription[Bar](cfg, func() {
			e.enqueue(func() {
				if _, ok := e.keyed[reqID]; !ok {
					return
				}
				e.deleteKeyedRoute(reqID)
				_ = e.send(codec.CancelRealTimeBars{ReqID: reqID})
				sub.closeWithErr(nil)
			})
		})

		e.keyed[reqID] = &route{
			opKind:       OpRealTimeBars,
			subscription: true,
			resume:       cfg.resume,
			request: codec.RealTimeBarsRequest{
				ReqID:      reqID,
				Contract:   toCodecContract(req.Contract),
				WhatToShow: string(req.WhatToShow),
				UseRTH:     req.UseRTH,
			},
			handle: func(msg any, e *engine) {
				barMsg, ok := msg.(codec.RealTimeBar)
				if !ok {
					return
				}
				bar, err := fromCodecRealtimeBar(barMsg)
				if err != nil {
					e.deleteKeyedRoute(reqID)
					sub.closeWithErr(err)
					return
				}
				emitSubscription(sub, bar)
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				if m.Code == 10167 {
					e.emitEvent(m.Code, m.Message)
					return
				}
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpRealTimeBars, m))
			},
			onDisconnect: func(e *engine, err error) bool {
				if cfg.resume == ResumeAuto && e.cfg.reconnect == ReconnectAuto {
					sub.emitState(SubscriptionStateEvent{
						Kind:          SubscriptionGap,
						ConnectionSeq: e.connectionSeq(),
						Err:           err,
					})
					return true
				}
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			emitGap: func(e *engine) {
				sub.emitState(SubscriptionStateEvent{
					Kind:          SubscriptionGap,
					ConnectionSeq: e.connectionSeq(),
				})
			},
			emitResumed: func(e *engine) {
				sub.emitState(SubscriptionStateEvent{
					Kind:          SubscriptionResumed,
					ConnectionSeq: e.connectionSeq(),
				})
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

func (e *engine) SubscribeMarketDepth(ctx context.Context, req MarketDepthRequest, opts ...SubscriptionOption) (*Subscription[DepthRow], error) {
	type result struct {
		sub *Subscription[DepthRow]
		err error
	}
	resp := make(chan result, 1)

	enqueueSubscriptionSetup(ctx, e, resp, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}

		cfg := defaultSubscriptionConfig(e.cfg)
		for _, opt := range opts {
			opt(&cfg)
		}
		if err := validateResumePolicy(OpMarketDepth, cfg.resume); err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		var sub *Subscription[DepthRow]
		sub = newSubscription[DepthRow](cfg, func() {
			e.enqueue(func() {
				if _, ok := e.keyed[reqID]; !ok {
					return
				}
				e.deleteKeyedRoute(reqID)
				_ = e.send(codec.CancelMarketDepth{ReqID: reqID, IsSmartDepth: req.IsSmartDepth})
				sub.closeWithErr(nil)
			})
		})

		e.keyed[reqID] = &route{
			opKind:       OpMarketDepth,
			subscription: true,
			resume:       cfg.resume,
			request: codec.MarketDepthRequest{
				ReqID:        reqID,
				Contract:     toCodecContract(req.Contract),
				NumRows:      req.NumRows,
				IsSmartDepth: req.IsSmartDepth,
			},
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.MarketDepthUpdate:
					row, err := fromCodecMarketDepth(m)
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					emitSubscription(sub, row)
				case codec.MarketDepthL2Update:
					row, err := fromCodecMarketDepthL2(m)
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					emitSubscription(sub, row)
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpMarketDepth, m))
			},
			onDisconnect: func(e *engine, err error) bool {
				if cfg.resume == ResumeAuto && e.cfg.reconnect == ReconnectAuto {
					sub.emitState(SubscriptionStateEvent{
						Kind:          SubscriptionGap,
						ConnectionSeq: e.connectionSeq(),
						Err:           err,
					})
					return true
				}
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			emitGap: func(e *engine) {
				sub.emitState(SubscriptionStateEvent{
					Kind:          SubscriptionGap,
					ConnectionSeq: e.connectionSeq(),
				})
			},
			emitResumed: func(e *engine) {
				sub.emitState(SubscriptionStateEvent{
					Kind:          SubscriptionResumed,
					ConnectionSeq: e.connectionSeq(),
				})
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

func (e *engine) MktDepthExchanges(ctx context.Context) ([]DepthExchange, error) {
	type result struct {
		exchanges []DepthExchange
		err       error
	}
	resp := make(chan result, 1)

	enqueueOneShotSetup(ctx, e, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		if _, exists := e.singletons[singletonMktDepthExchanges]; exists {
			resp <- result{err: fmt.Errorf("ibkr: mkt depth exchanges request already in progress")}
			return
		}

		e.singletons[singletonMktDepthExchanges] = &route{
			opKind: OpMktDepthExchanges,
			handle: func(msg any, eng *engine) {
				switch m := msg.(type) {
				case codec.MktDepthExchanges:
					delete(eng.singletons, singletonMktDepthExchanges)
					exchanges := make([]DepthExchange, len(m.Exchanges))
					for i, x := range m.Exchanges {
						exchanges[i] = DepthExchange{
							Exchange: x.Exchange, SecType: SecType(x.SecType),
							ListingExch: x.ListingExch, ServiceDataType: x.ServiceDataType,
							AggGroup: x.AggGroup,
						}
					}
					resp <- result{exchanges: exchanges}
				}
			},
			onDisconnect: func(eng *engine, err error) bool {
				delete(eng.singletons, singletonMktDepthExchanges)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.sendContext(ctx, codec.MktDepthExchangesRequest{}); err != nil {
			delete(e.singletons, singletonMktDepthExchanges)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { delete(e.singletons, singletonMktDepthExchanges) })
	})
	if err != nil {
		return nil, err
	}
	return out.exchanges, out.err
}

func (e *engine) SubscribeTickByTick(ctx context.Context, req TickByTickRequest, opts ...SubscriptionOption) (*Subscription[TickByTickData], error) {
	type result struct {
		sub *Subscription[TickByTickData]
		err error
	}
	resp := make(chan result, 1)

	enqueueSubscriptionSetup(ctx, e, resp, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}

		cfg := defaultSubscriptionConfig(e.cfg)
		for _, opt := range opts {
			opt(&cfg)
		}
		if err := validateResumePolicy(OpTickByTick, cfg.resume); err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		var sub *Subscription[TickByTickData]
		sub = newSubscription[TickByTickData](cfg, func() {
			e.enqueue(func() {
				if _, ok := e.keyed[reqID]; !ok {
					return
				}
				e.deleteKeyedRoute(reqID)
				_ = e.send(codec.CancelTickByTick{ReqID: reqID})
				sub.closeWithErr(nil)
			})
		})

		e.keyed[reqID] = &route{
			opKind:       OpTickByTick,
			subscription: true,
			resume:       cfg.resume,
			request: codec.TickByTickRequest{
				ReqID: reqID, Contract: toCodecContract(req.Contract),
				TickType: string(req.TickType), NumberOfTicks: req.NumberOfTicks, IgnoreSize: req.IgnoreSize,
			},
			handle: func(msg any, e *engine) {
				if m, ok := msg.(codec.TickByTickData); ok {
					ts, err := parseTickByTickTime(m.Time)
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					tick := TickByTickData{Time: ts, TickType: m.TickType}
					switch m.TickType {
					case 1, 2:
						tick.Price, err = parseOptionalDecimal(m.Price, "tick by tick price")
						if err != nil {
							e.deleteKeyedRoute(reqID)
							sub.closeWithErr(err)
							return
						}
						tick.Size, err = parseOptionalDecimal(m.Size, "tick by tick size")
						if err != nil {
							e.deleteKeyedRoute(reqID)
							sub.closeWithErr(err)
							return
						}
						tick.Exchange = m.Exchange
						tick.SpecialConditions = m.SpecialConditions
					case 3:
						tick.BidPrice, err = parseOptionalDecimal(m.BidPrice, "tick by tick bid price")
						if err != nil {
							e.deleteKeyedRoute(reqID)
							sub.closeWithErr(err)
							return
						}
						tick.AskPrice, err = parseOptionalDecimal(m.AskPrice, "tick by tick ask price")
						if err != nil {
							e.deleteKeyedRoute(reqID)
							sub.closeWithErr(err)
							return
						}
						tick.BidSize, err = parseOptionalDecimal(m.BidSize, "tick by tick bid size")
						if err != nil {
							e.deleteKeyedRoute(reqID)
							sub.closeWithErr(err)
							return
						}
						tick.AskSize, err = parseOptionalDecimal(m.AskSize, "tick by tick ask size")
						if err != nil {
							e.deleteKeyedRoute(reqID)
							sub.closeWithErr(err)
							return
						}
					case 4:
						tick.MidPoint, err = parseOptionalDecimal(m.MidPoint, "tick by tick midpoint")
						if err != nil {
							e.deleteKeyedRoute(reqID)
							sub.closeWithErr(err)
							return
						}
					}
					emitSubscription(sub, tick)
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				if m.Code == 10167 {
					e.emitEvent(m.Code, m.Message)
					return
				}
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpTickByTick, m))
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

func (e *engine) CalcImpliedVolatility(ctx context.Context, req CalcImpliedVolatilityRequest) (OptionComputation, error) {
	type result struct {
		value OptionComputation
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
		e.keyed[reqID] = &route{
			opKind: OpCalcImpliedVol,
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.TickOptionComputation:
					delete(e.keyed, reqID)
					value, err := fromCodecOptionComputation(m)
					if err != nil {
						resp <- result{err: err}
						return
					}
					resp <- result{value: value}
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpCalcImpliedVol, m)}
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
		if err := e.sendContext(ctx, codec.CalcImpliedVolatilityRequest{
			ReqID:       reqID,
			Contract:    toCodecContract(req.Contract),
			OptionPrice: req.OptionPrice.String(),
			UnderPrice:  req.UnderPrice.String(),
		}); err != nil {
			delete(e.keyed, reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() {
			if _, ok := e.keyed[reqID]; ok {
				e.deleteKeyedRoute(reqID)
				_ = e.send(codec.CancelCalcImpliedVolatility{ReqID: reqID})
			}
		})
	})
	if err != nil {
		return OptionComputation{}, err
	}
	return out.value, out.err
}

func (e *engine) CalcOptionPrice(ctx context.Context, req CalcOptionPriceRequest) (OptionComputation, error) {
	type result struct {
		value OptionComputation
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
		e.keyed[reqID] = &route{
			opKind: OpCalcOptionPrice,
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.TickOptionComputation:
					delete(e.keyed, reqID)
					value, err := fromCodecOptionComputation(m)
					if err != nil {
						resp <- result{err: err}
						return
					}
					resp <- result{value: value}
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpCalcOptionPrice, m)}
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
		if err := e.sendContext(ctx, codec.CalcOptionPriceRequest{
			ReqID:      reqID,
			Contract:   toCodecContract(req.Contract),
			Volatility: req.Volatility.String(),
			UnderPrice: req.UnderPrice.String(),
		}); err != nil {
			delete(e.keyed, reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() {
			if _, ok := e.keyed[reqID]; ok {
				e.deleteKeyedRoute(reqID)
				_ = e.send(codec.CancelCalcOptionPrice{ReqID: reqID})
			}
		})
	})
	if err != nil {
		return OptionComputation{}, err
	}
	return out.value, out.err
}

func fromCodecOptionComputation(m codec.TickOptionComputation) (OptionComputation, error) {
	iv, err := parseOptionalDecimal(m.ImpliedVol, "option computation implied vol")
	if err != nil {
		return OptionComputation{}, err
	}
	delta, err := parseOptionalDecimal(m.Delta, "option computation delta")
	if err != nil {
		return OptionComputation{}, err
	}
	optPrice, err := parseOptionalDecimal(m.OptPrice, "option computation option price")
	if err != nil {
		return OptionComputation{}, err
	}
	pvDiv, err := parseOptionalDecimal(m.PvDividend, "option computation pv dividend")
	if err != nil {
		return OptionComputation{}, err
	}
	gamma, err := parseOptionalDecimal(m.Gamma, "option computation gamma")
	if err != nil {
		return OptionComputation{}, err
	}
	vega, err := parseOptionalDecimal(m.Vega, "option computation vega")
	if err != nil {
		return OptionComputation{}, err
	}
	theta, err := parseOptionalDecimal(m.Theta, "option computation theta")
	if err != nil {
		return OptionComputation{}, err
	}
	undPrice, err := parseOptionalDecimal(m.UndPrice, "option computation underlying price")
	if err != nil {
		return OptionComputation{}, err
	}
	return OptionComputation{
		ImpliedVol: iv, Delta: delta, OptPrice: optPrice,
		PvDividend: pvDiv, Gamma: gamma, Vega: vega,
		Theta: theta, UndPrice: undPrice,
	}, nil
}

func fromCodecRealtimeBar(m codec.RealTimeBar) (Bar, error) {
	return fromCodecBar(codec.HistoricalBar(m))
}

func fromCodecMarketDepth(m codec.MarketDepthUpdate) (DepthRow, error) {
	price, err := decimal.NewFromString(m.Price)
	if err != nil {
		return DepthRow{}, fmt.Errorf("ibkr: market depth price: %w", err)
	}
	size, err := decimal.NewFromString(m.Size)
	if err != nil {
		return DepthRow{}, fmt.Errorf("ibkr: market depth size: %w", err)
	}
	return DepthRow{
		Position:  m.Position,
		Operation: DepthOperation(m.Operation),
		Side:      BookSide(m.Side),
		Price:     price,
		Size:      size,
	}, nil
}

func fromCodecMarketDepthL2(m codec.MarketDepthL2Update) (DepthRow, error) {
	price, err := decimal.NewFromString(m.Price)
	if err != nil {
		return DepthRow{}, fmt.Errorf("ibkr: market depth l2 price: %w", err)
	}
	size, err := decimal.NewFromString(m.Size)
	if err != nil {
		return DepthRow{}, fmt.Errorf("ibkr: market depth l2 size: %w", err)
	}
	return DepthRow{
		Position:     m.Position,
		MarketMaker:  m.MarketMaker,
		Operation:    DepthOperation(m.Operation),
		Side:         BookSide(m.Side),
		Price:        price,
		Size:         size,
		IsSmartDepth: m.IsSmartDepth,
	}, nil
}

// parseTickByTickTime parses a tick-by-tick timestamp. The wire sends a Unix
// epoch seconds value as a string. Falls back to RFC3339 for test transcripts.
func parseTickByTickTime(raw string) (time.Time, error) {
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts, nil
	}
	return parseEpochSeconds(raw)
}

func applyTickPrice(quote *Quote, field int, raw string) (QuoteFields, error) {
	switch field {
	case 1, 66: // bid
		value, err := parseRequiredDecimal(raw, "quote bid")
		if err != nil {
			return 0, err
		}
		quote.Bid = value
		quote.Available |= QuoteFieldBid
		return QuoteFieldBid, nil
	case 2, 67: // ask
		value, err := parseRequiredDecimal(raw, "quote ask")
		if err != nil {
			return 0, err
		}
		quote.Ask = value
		quote.Available |= QuoteFieldAsk
		return QuoteFieldAsk, nil
	case 4, 68: // last
		value, err := parseRequiredDecimal(raw, "quote last")
		if err != nil {
			return 0, err
		}
		quote.Last = value
		quote.Available |= QuoteFieldLast
		return QuoteFieldLast, nil
	case 6, 72: // high
		value, err := parseRequiredDecimal(raw, "quote high")
		if err != nil {
			return 0, err
		}
		quote.High = value
		quote.Available |= QuoteFieldHigh
		return QuoteFieldHigh, nil
	case 7, 73: // low
		value, err := parseRequiredDecimal(raw, "quote low")
		if err != nil {
			return 0, err
		}
		quote.Low = value
		quote.Available |= QuoteFieldLow
		return QuoteFieldLow, nil
	case 9, 75: // close
		value, err := parseRequiredDecimal(raw, "quote close")
		if err != nil {
			return 0, err
		}
		quote.Close = value
		quote.Available |= QuoteFieldClose
		return QuoteFieldClose, nil
	case 14, 76: // open
		value, err := parseRequiredDecimal(raw, "quote open")
		if err != nil {
			return 0, err
		}
		quote.Open = value
		quote.Available |= QuoteFieldOpen
		return QuoteFieldOpen, nil
	default:
		return 0, nil
	}
}

func applyTickSize(quote *Quote, field int, raw string) (QuoteFields, error) {
	switch field {
	case 0, 69: // bid_size
		value, err := parseRequiredDecimal(raw, "quote bid size")
		if err != nil {
			return 0, err
		}
		quote.BidSize = value
		quote.Available |= QuoteFieldBidSize
		return QuoteFieldBidSize, nil
	case 3, 70: // ask_size
		value, err := parseRequiredDecimal(raw, "quote ask size")
		if err != nil {
			return 0, err
		}
		quote.AskSize = value
		quote.Available |= QuoteFieldAskSize
		return QuoteFieldAskSize, nil
	case 5, 71: // last_size
		value, err := parseRequiredDecimal(raw, "quote last size")
		if err != nil {
			return 0, err
		}
		quote.LastSize = value
		quote.Available |= QuoteFieldLastSize
		return QuoteFieldLastSize, nil
	case 8, 74: // volume
		return 0, nil
	default:
		return 0, nil
	}
}

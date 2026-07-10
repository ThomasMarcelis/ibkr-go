package ibkr

import (
	"context"
	"errors"
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
	if err := validateContract(req.Contract); err != nil {
		return nil, err
	}
	req.Contract = cloneContract(req.Contract)
	genericTicks := formatGenericTicks(req.GenericTicks)
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
		if err := validateContractFieldSupport(req.Contract, "market data quote", e.serverVersion, quoteContractFields(e.serverVersion)); err != nil {
			resp <- result{err: err}
			return
		}

		cfg, err := applySubscriptionOptions(e.cfg, opts)
		if err != nil {
			resp <- result{err: err}
			return
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
		actorCancel := func() {
			if _, ok := e.keyed[reqID]; !ok {
				return
			}
			e.deleteKeyedRoute(reqID)
			sub.closeWithErr(e.cancelSubscription(OpQuotes, codec.CancelQuote{ReqID: reqID}))
		}
		sub = newEngineSubscription[QuoteUpdate](cfg, e, actorCancel)
		if snapshot {
			sub.expectSnapshot()
		}
		quote := Quote{}
		var quoteRoute *route
		rerouted := false
		resumeContract := cloneContract(req.Contract)

		quoteRoute = &route{
			opKind:       OpQuotes,
			subscription: true,
			resume:       cfg.resume,
			request: codec.QuoteRequest{
				ReqID:        reqID,
				Contract:     toCodecContract(req.Contract),
				Snapshot:     snapshot,
				GenericTicks: genericTicks,
			},
			validateResume: func(e *engine) error {
				return validateContractFieldSupport(resumeContract, "resume market data quote", e.serverVersion, quoteContractFields(e.serverVersion))
			},
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.MarketDataReroute:
					if rerouted {
						cancelErr := e.cancelSubscription(OpQuotes, codec.CancelQuote{ReqID: reqID})
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(errors.Join(
							fmt.Errorf("ibkr: market data request %d was rerouted more than once", reqID),
							cancelErr,
						))
						return
					}
					request := quoteRoute.request.(codec.QuoteRequest)
					request.Contract = codec.Contract{ConID: m.ConID, Exchange: m.Exchange}
					quoteRoute.request = request
					resumeContract = Contract{ConID: m.ConID, Exchange: m.Exchange}
					rerouted = true
					if err := e.send(request); err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(fmt.Errorf("ibkr: reroute market data request %d: %w", reqID, err))
					}
				case codec.TickPrice:
					price, err := parseRequiredDecimal(m.Price, "quote price tick")
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					size, err := parseOptionalDecimalPointer(m.Size, "quote price tick companion size")
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					changed := applyTickPrice(&quote, m.TickType, price)
					if sizeField, ok := companionSizeTickType(m.TickType); ok && size != nil {
						changed |= applyTickSize(&quote, sizeField, *size)
					}
					emitSubscription(sub, QuoteUpdate{
						Kind:     QuoteUpdatePriceTick,
						Snapshot: quote,
						Changed:  changed,
						PriceTick: new(QuotePriceTick{
							TickType: m.TickType,
							Price:    price,
							Size:     size,
							AttrMask: QuotePriceAttributes(m.AttrMask),
						}),
						ReceivedAt: time.Now().UTC(),
					})
				case codec.TickSize:
					size, err := parseOptionalDecimalPointer(m.Size, "quote size tick")
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					var changed QuoteFields
					if size != nil {
						changed = applyTickSize(&quote, m.TickType, *size)
					}
					emitSubscription(sub, QuoteUpdate{
						Kind:       QuoteUpdateSizeTick,
						Snapshot:   quote,
						Changed:    changed,
						SizeTick:   new(QuoteSizeTick{TickType: m.TickType, Size: size}),
						ReceivedAt: time.Now().UTC(),
					})
				case codec.MarketDataType:
					quote.MarketDataType = MarketDataType(m.DataType)
					quote.Available |= QuoteFieldMarketDataType
					emitSubscription(sub, QuoteUpdate{Kind: QuoteUpdateFields, Snapshot: quote, Changed: QuoteFieldMarketDataType, ReceivedAt: time.Now().UTC()})
				case codec.TickGeneric:
					value, err := parseRequiredDecimal(m.Value, "generic tick value")
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					emitSubscription(sub, QuoteUpdate{
						Kind:        QuoteUpdateGenericTick,
						Snapshot:    quote,
						GenericTick: new(QuoteGenericTick{TickType: m.TickType, Value: value}),
						ReceivedAt:  time.Now().UTC(),
					})
				case codec.TickString:
					emitSubscription(sub, QuoteUpdate{
						Kind:       QuoteUpdateStringTick,
						Snapshot:   quote,
						StringTick: new(QuoteStringTick{TickType: m.TickType, Value: m.Value}),
						ReceivedAt: time.Now().UTC(),
					})
				case codec.TickNews:
					timestamp, err := parseEpochMilliseconds(m.Time)
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					emitSubscription(sub, QuoteUpdate{
						Kind:     QuoteUpdateNewsTick,
						Snapshot: quote,
						NewsTick: new(QuoteNewsTick{
							Time:         timestamp,
							ProviderCode: NewsProviderCode(m.ProviderCode),
							ArticleID:    m.ArticleID,
							Headline:     m.Headline,
							ExtraData:    m.ExtraData,
						}),
						ReceivedAt: time.Now().UTC(),
					})
				case codec.TickReqParams:
					minTick, err := parseOptionalDecimalPointer(m.MinTick, "quote parameters minimum tick")
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					lastPricePrecision, err := parseOptionalDecimalPointer(m.LastPricePrecision, "quote parameters last price precision")
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					lastSizePrecision, err := parseOptionalDecimalPointer(m.LastSizePrecision, "quote parameters last size precision")
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					emitSubscription(sub, QuoteUpdate{
						Kind:     QuoteUpdateParameters,
						Snapshot: quote,
						Parameters: new(QuoteParameters{
							MinTick:             minTick,
							BBOExchange:         m.BBOExchange,
							SnapshotPermissions: m.SnapshotPermissions,
							LastPricePrecision:  lastPricePrecision,
							LastSizePrecision:   lastSizePrecision,
						}),
						ReceivedAt: time.Now().UTC(),
					})
				case codec.TickOptionComputation:
					computation, err := fromCodecOptionComputation(m)
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					emitSubscription(sub, QuoteUpdate{
						Kind:     QuoteUpdateOptionComputation,
						Snapshot: quote,
						OptionComputation: new(QuoteOptionComputation{
							TickType:    m.TickType,
							TickAttrib:  m.TickAttrib,
							Computation: computation,
						}),
						ReceivedAt: time.Now().UTC(),
					})
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
		e.keyed[reqID] = quoteRoute
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
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

func (e *engine) SubscribeRealTimeBars(ctx context.Context, req RealTimeBarsRequest, opts ...SubscriptionOption) (*Subscription[Bar], error) {
	if err := validateContract(req.Contract); err != nil {
		return nil, err
	}
	req.Contract = cloneContract(req.Contract)
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
		if err := validateContractFieldSupport(req.Contract, "real-time bars", e.serverVersion, contractFieldPrimaryExchange); err != nil {
			resp <- result{err: err}
			return
		}

		cfg, err := applySubscriptionOptions(e.cfg, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		if err := validateResumePolicy(OpRealTimeBars, cfg.resume); err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		resumeContract := cloneContract(req.Contract)
		var sub *Subscription[Bar]
		actorCancel := func() {
			if _, ok := e.keyed[reqID]; !ok {
				return
			}
			e.deleteKeyedRoute(reqID)
			sub.closeWithErr(e.cancelSubscription(OpRealTimeBars, codec.CancelRealTimeBars{ReqID: reqID}))
		}
		sub = newEngineSubscription[Bar](cfg, e, actorCancel)

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
			validateResume: func(e *engine) error {
				return validateContractFieldSupport(resumeContract, "resume real-time bars", e.serverVersion, contractFieldPrimaryExchange)
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

	out, err := awaitSubscriptionResponse(ctx, e, resp, func(out result) bool { return out.sub != nil })
	if err != nil {
		return nil, err
	}
	if out.err == nil && out.sub != nil {
		bindContext(ctx, out.sub)
	}
	return out.sub, out.err
}

func (e *engine) SubscribeMarketDepth(ctx context.Context, req MarketDepthRequest, opts ...SubscriptionOption) (*Subscription[DepthRow], error) {
	if err := validateContract(req.Contract); err != nil {
		return nil, err
	}
	req.Contract = cloneContract(req.Contract)
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
		if err := validateContractFieldSupport(req.Contract, "market depth", e.serverVersion, depthContractFields(e.serverVersion)); err != nil {
			resp <- result{err: err}
			return
		}

		cfg, err := applySubscriptionOptions(e.cfg, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		if cfg.slowConsumer == SlowConsumerDropOldest {
			resp <- result{err: &ValidationError{
				Field:   "SlowConsumerPolicy",
				Value:   string(cfg.slowConsumer),
				Message: "must be SlowConsumerClose for market depth because every update mutates book state",
			}}
			return
		}
		if err := validateResumePolicy(OpMarketDepth, cfg.resume); err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		var depthRoute *route
		rerouted := false
		var sub *Subscription[DepthRow]
		actorCancel := func() {
			if _, ok := e.keyed[reqID]; !ok {
				return
			}
			request := depthRoute.request.(codec.MarketDepthRequest)
			e.deleteKeyedRoute(reqID)
			sub.closeWithErr(e.cancelSubscription(OpMarketDepth, codec.CancelMarketDepth{ReqID: reqID, IsSmartDepth: request.IsSmartDepth}))
		}
		sub = newEngineSubscription[DepthRow](cfg, e, actorCancel)

		depthRoute = &route{
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
				case codec.MarketDepthReroute:
					if rerouted {
						request := depthRoute.request.(codec.MarketDepthRequest)
						cancelErr := e.cancelSubscription(OpMarketDepth, codec.CancelMarketDepth{ReqID: reqID, IsSmartDepth: request.IsSmartDepth})
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(errors.Join(
							fmt.Errorf("ibkr: market depth request %d was rerouted more than once", reqID),
							cancelErr,
						))
						return
					}
					request := depthRoute.request.(codec.MarketDepthRequest)
					request.Contract = codec.Contract{ConID: m.ConID, Exchange: m.Exchange}
					depthRoute.request = request
					rerouted = true
					if err := e.send(request); err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(fmt.Errorf("ibkr: reroute market depth request %d: %w", reqID, err))
					}
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
		e.keyed[reqID] = depthRoute
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
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
	if err := validateContract(req.Contract); err != nil {
		return nil, err
	}
	req.Contract = cloneContract(req.Contract)
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
		if err := validateContractFieldSupport(req.Contract, "tick-by-tick data", e.serverVersion, contractFieldPrimaryExchange); err != nil {
			resp <- result{err: err}
			return
		}

		cfg, err := applySubscriptionOptions(e.cfg, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		if err := validateResumePolicy(OpTickByTick, cfg.resume); err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		var sub *Subscription[TickByTickData]
		actorCancel := func() {
			if _, ok := e.keyed[reqID]; !ok {
				return
			}
			e.deleteKeyedRoute(reqID)
			sub.closeWithErr(e.cancelSubscription(OpTickByTick, codec.CancelTickByTick{ReqID: reqID}))
		}
		sub = newEngineSubscription[TickByTickData](cfg, e, actorCancel)

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

	out, err := awaitSubscriptionResponse(ctx, e, resp, func(out result) bool { return out.sub != nil })
	if err != nil {
		return nil, err
	}
	if out.err == nil && out.sub != nil {
		bindContext(ctx, out.sub)
	}
	return out.sub, out.err
}

func (e *engine) CalcImpliedVolatility(ctx context.Context, req CalcImpliedVolatilityRequest) (OptionComputation, error) {
	if err := validateContract(req.Contract); err != nil {
		return OptionComputation{}, err
	}
	req.Contract = cloneContract(req.Contract)
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
		if err := validateContractFieldSupport(req.Contract, "calculate implied volatility", e.serverVersion, contractFieldPrimaryExchange); err != nil {
			resp <- result{err: err}
			return
		}
		reqID = e.allocReqID()
		e.keyed[reqID] = &route{
			opKind: OpCalcImpliedVol,
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.TickOptionComputation:
					e.deleteKeyedRoute(reqID)
					value, err := fromCodecOptionComputation(m)
					if err != nil {
						resp <- result{err: err}
						return
					}
					resp <- result{value: value}
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				e.deleteKeyedRoute(reqID)
				resp <- result{err: e.apiErr(OpCalcImpliedVol, m)}
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
		if err := e.sendContext(ctx, codec.CalcImpliedVolatilityRequest{
			ReqID:       reqID,
			Contract:    toCodecContract(req.Contract),
			OptionPrice: req.OptionPrice.String(),
			UnderPrice:  req.UnderPrice.String(),
		}); err != nil {
			e.deleteKeyedRoute(reqID)
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
	if err := validateContract(req.Contract); err != nil {
		return OptionComputation{}, err
	}
	req.Contract = cloneContract(req.Contract)
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
		if err := validateContractFieldSupport(req.Contract, "calculate option price", e.serverVersion, contractFieldPrimaryExchange); err != nil {
			resp <- result{err: err}
			return
		}
		reqID = e.allocReqID()
		e.keyed[reqID] = &route{
			opKind: OpCalcOptionPrice,
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.TickOptionComputation:
					e.deleteKeyedRoute(reqID)
					value, err := fromCodecOptionComputation(m)
					if err != nil {
						resp <- result{err: err}
						return
					}
					resp <- result{value: value}
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				e.deleteKeyedRoute(reqID)
				resp <- result{err: e.apiErr(OpCalcOptionPrice, m)}
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
		if err := e.sendContext(ctx, codec.CalcOptionPriceRequest{
			ReqID:      reqID,
			Contract:   toCodecContract(req.Contract),
			Volatility: req.Volatility.String(),
			UnderPrice: req.UnderPrice.String(),
		}); err != nil {
			e.deleteKeyedRoute(reqID)
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
	var available OptionComputationFields
	parse := func(raw, unavailable, field string, bit OptionComputationFields) (decimal.Decimal, error) {
		if raw == unavailable {
			return decimal.Decimal{}, nil
		}
		value, err := parseOptionalDecimalPointer(raw, field)
		if err != nil {
			return decimal.Decimal{}, err
		}
		if value == nil {
			return decimal.Decimal{}, nil
		}
		available |= bit
		return *value, nil
	}

	iv, err := parse(m.ImpliedVol, "-1", "option computation implied vol", OptionComputationImpliedVol)
	if err != nil {
		return OptionComputation{}, err
	}
	delta, err := parse(m.Delta, "-2", "option computation delta", OptionComputationDelta)
	if err != nil {
		return OptionComputation{}, err
	}
	optPrice, err := parse(m.OptPrice, "-1", "option computation option price", OptionComputationPrice)
	if err != nil {
		return OptionComputation{}, err
	}
	pvDiv, err := parse(m.PvDividend, "-1", "option computation pv dividend", OptionComputationPvDividend)
	if err != nil {
		return OptionComputation{}, err
	}
	gamma, err := parse(m.Gamma, "-2", "option computation gamma", OptionComputationGamma)
	if err != nil {
		return OptionComputation{}, err
	}
	vega, err := parse(m.Vega, "-2", "option computation vega", OptionComputationVega)
	if err != nil {
		return OptionComputation{}, err
	}
	theta, err := parse(m.Theta, "-2", "option computation theta", OptionComputationTheta)
	if err != nil {
		return OptionComputation{}, err
	}
	undPrice, err := parse(m.UndPrice, "-1", "option computation underlying price", OptionComputationUnderlyingPrice)
	if err != nil {
		return OptionComputation{}, err
	}
	return OptionComputation{
		Available:  available,
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
	size, err := parseOptionalDecimalPointer(m.Size, "market depth size")
	if err != nil {
		return DepthRow{}, err
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
	size, err := parseOptionalDecimalPointer(m.Size, "market depth l2 size")
	if err != nil {
		return DepthRow{}, err
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

func applyTickPrice(quote *Quote, field int, value decimal.Decimal) QuoteFields {
	switch field {
	case 1, 66: // bid
		quote.Bid = value
		quote.Available |= QuoteFieldBid
		return QuoteFieldBid
	case 2, 67: // ask
		quote.Ask = value
		quote.Available |= QuoteFieldAsk
		return QuoteFieldAsk
	case 4, 68: // last
		quote.Last = value
		quote.Available |= QuoteFieldLast
		return QuoteFieldLast
	case 6, 72: // high
		quote.High = value
		quote.Available |= QuoteFieldHigh
		return QuoteFieldHigh
	case 7, 73: // low
		quote.Low = value
		quote.Available |= QuoteFieldLow
		return QuoteFieldLow
	case 9, 75: // close
		quote.Close = value
		quote.Available |= QuoteFieldClose
		return QuoteFieldClose
	case 14, 76: // open
		quote.Open = value
		quote.Available |= QuoteFieldOpen
		return QuoteFieldOpen
	default:
		return 0
	}
}

func applyTickSize(quote *Quote, field int, value decimal.Decimal) QuoteFields {
	switch field {
	case 0, 69: // bid_size
		quote.BidSize = value
		quote.Available |= QuoteFieldBidSize
		return QuoteFieldBidSize
	case 3, 70: // ask_size
		quote.AskSize = value
		quote.Available |= QuoteFieldAskSize
		return QuoteFieldAskSize
	case 5, 71: // last_size
		quote.LastSize = value
		quote.Available |= QuoteFieldLastSize
		return QuoteFieldLastSize
	case 8, 74: // volume
		quote.Volume = value
		quote.Available |= QuoteFieldVolume
		return QuoteFieldVolume
	default:
		return 0
	}
}

func companionSizeTickType(priceTickType int) (int, bool) {
	switch priceTickType {
	case 1: // bid
		return 0, true
	case 2: // ask
		return 3, true
	case 4: // last
		return 5, true
	case 66: // delayed_bid
		return 69, true
	case 67: // delayed_ask
		return 70, true
	case 68: // delayed_last
		return 71, true
	default:
		return 0, false
	}
}

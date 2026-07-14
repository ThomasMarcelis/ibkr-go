package ibkr

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/shopspring/decimal"
)

func (e *engine) SetMarketDataType(ctx context.Context, dataType MarketDataType) error {
	if dataType < MarketDataLive || dataType > MarketDataDelayedFrozen {
		return &ValidationError{
			Field:   "MarketDataType",
			Value:   fmt.Sprint(dataType),
			Message: "must be Live, Frozen, Delayed, or DelayedFrozen (1..4)",
		}
	}
	return awaitFireAndForget(ctx, e, func(ctx context.Context) error {
		return e.sendContext(ctx, codec.ReqMarketDataType{DataType: int(dataType)})
	})
}

func (e *engine) QuoteSnapshot(ctx context.Context, req QuoteRequest) (Quote, error) {
	return e.quoteSnapshot(ctx, req, false)
}

func (e *engine) RegulatorySnapshot(ctx context.Context, contract Contract) (Quote, error) {
	return e.quoteSnapshot(ctx, QuoteRequest{Contract: contract}, true)
}

func (e *engine) quoteSnapshot(ctx context.Context, req QuoteRequest, regulatory bool) (Quote, error) {
	sub, err := e.subscribeQuotes(ctx, req, true, regulatory)
	if err != nil {
		return Quote{}, err
	}
	defer sub.Close()

	var latest Quote
	finish := func(err error) (Quote, error) {
		if !regulatory {
			return latest, err
		}
		return latest, regulatorySnapshotError(err)
	}
	for {
		select {
		case event, ok := <-sub.Events():
			if !ok {
				return finish(sub.Wait())
			}
			switch event.Kind {
			case StreamData:
				latest = event.Value.Snapshot
			case StreamSnapshotComplete:
				return latest, nil
			}
		case <-ctx.Done():
			// A completion already queued by the actor owns the result even if
			// the caller deadline became ready at the same instant.
			for {
				select {
				case event, ok := <-sub.Events():
					if !ok {
						return finish(sub.Wait())
					}
					if event.Kind == StreamData {
						latest = event.Value.Snapshot
					}
					if event.Kind == StreamSnapshotComplete {
						return latest, nil
					}
				default:
					return finish(context.Cause(ctx))
				}
			}
		}
	}
}

func regulatorySnapshotError(err error) error {
	if err == nil {
		return nil
	}
	if _, definitive := errors.AsType[*APIError](err); definitive {
		return err
	}
	return errors.Join(ErrRegulatorySnapshotUncertain, err)
}

func (e *engine) SubscribeQuotes(ctx context.Context, req QuoteRequest, opts ...SubscriptionOption) (*Subscription[QuoteUpdate], error) {
	return e.subscribeQuotes(ctx, req, false, false, opts...)
}

func (e *engine) subscribeQuotes(ctx context.Context, req QuoteRequest, snapshot, regulatory bool, opts ...SubscriptionOption) (*Subscription[QuoteUpdate], error) {
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
		if err := validateContractFieldSupport(req.Contract, "market data quote", e.serverVersion, quoteContractFields(e.serverVersion)); err != nil {
			resp <- result{err: err}
			return
		}

		cfg, err := applySubscriptionOptionsFor(e.cfg, OpQuotes, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		if err := validateQuoteRequest(req, snapshot, cfg.resume); err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		sub, quoteRoute := newKeyedSubscriptionRoute[QuoteUpdate](
			e, cfg, reqID, OpQuotes, codec.CancelQuote{ReqID: reqID},
		)
		if snapshot {
			sub.expectSnapshot()
		}
		quote := Quote{}
		rerouted := false
		resumeContract := cloneContract(req.Contract)

		quoteRoute.request = codec.QuoteRequest{
			ReqID:              reqID,
			Contract:           toCodecContract(req.Contract),
			Snapshot:           snapshot && !regulatory,
			RegulatorySnapshot: regulatory,
			GenericTicks:       genericTicks,
		}
		quoteRoute.validateResume = func(e *engine) error {
			return validateContractFieldSupport(resumeContract, "resume market data quote", e.serverVersion, quoteContractFields(e.serverVersion))
		}
		quoteRoute.handle = func(msg any, e *engine) {
			fail := func(err error) { sub.cancelFromActor(err) }
			switch m := msg.(type) {
			case codec.MarketDataReroute:
				if rerouted {
					cancelErr := e.cancelRouteSubscription(quoteRoute, OpQuotes, codec.CancelQuote{ReqID: reqID})
					e.deleteKeyedRoute(reqID)
					closeErr := errors.Join(
						fmt.Errorf("ibkr: market data request %d was rerouted more than once", reqID),
						cancelErr,
					)
					sub.closeWithErr(closeErr)
					e.retireSubscriptionTransport(cancelErr)
					return
				}
				request := quoteRoute.request.(codec.QuoteRequest)
				request.Contract = codec.Contract{ConID: m.ConID, Exchange: m.Exchange}
				quoteRoute.request = request
				resumeContract = Contract{ConID: protocolIDFromInt[ContractID](m.ConID), Exchange: m.Exchange}
				rerouted = true
				if err := e.send(request); err != nil {
					fail(fmt.Errorf("ibkr: reroute market data request %d: %w", reqID, err))
				}
			case codec.TickPrice:
				price, err := parseRequiredDecimal(m.Price, "quote price tick")
				if err != nil {
					fail(err)
					return
				}
				size, err := parseOptionalDecimalPointer(m.Size, "quote price tick companion size")
				if err != nil {
					fail(err)
					return
				}
				changed := applyTickPrice(&quote, m.TickType, price)
				if sizeField, ok := companionSizeTickType(m.TickType); ok && size != nil {
					changed |= applyTickSize(&quote, sizeField, *size)
				}
				sub.emit(QuoteUpdate{
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
					fail(err)
					return
				}
				var changed QuoteFields
				if size != nil {
					changed = applyTickSize(&quote, m.TickType, *size)
				}
				sub.emit(QuoteUpdate{
					Kind:       QuoteUpdateSizeTick,
					Snapshot:   quote,
					Changed:    changed,
					SizeTick:   new(QuoteSizeTick{TickType: m.TickType, Size: size}),
					ReceivedAt: time.Now().UTC(),
				})
			case codec.MarketDataType:
				quote.MarketDataType = MarketDataType(m.DataType)
				quote.Available |= QuoteFieldMarketDataType
				sub.emit(QuoteUpdate{Kind: QuoteUpdateFields, Snapshot: quote, Changed: QuoteFieldMarketDataType, ReceivedAt: time.Now().UTC()})
			case codec.TickGeneric:
				value, err := parseRequiredDecimal(m.Value, "generic tick value")
				if err != nil {
					fail(err)
					return
				}
				sub.emit(QuoteUpdate{
					Kind:        QuoteUpdateGenericTick,
					Snapshot:    quote,
					GenericTick: new(QuoteGenericTick{TickType: m.TickType, Value: value}),
					ReceivedAt:  time.Now().UTC(),
				})
			case codec.TickString:
				changed := applyTickString(&quote, m.TickType, m.Value)
				sub.emit(QuoteUpdate{
					Kind:       QuoteUpdateStringTick,
					Snapshot:   quote,
					Changed:    changed,
					StringTick: new(QuoteStringTick{TickType: m.TickType, Value: m.Value}),
					ReceivedAt: time.Now().UTC(),
				})
			case codec.TickEFP:
				basisPoints, err := parseRequiredDecimal(m.BasisPoints, "EFP basis points")
				if err != nil {
					fail(err)
					return
				}
				impliedFuturesPrice, err := parseRequiredDecimal(m.ImpliedFuturesPrice, "EFP implied futures price")
				if err != nil {
					fail(err)
					return
				}
				dividendImpact, err := parseRequiredDecimal(m.DividendImpact, "EFP dividend impact")
				if err != nil {
					fail(err)
					return
				}
				dividendsToLastTradeDate, err := parseRequiredDecimal(m.DividendsToLastTradeDate, "EFP dividends to last trade date")
				if err != nil {
					fail(err)
					return
				}
				sub.emit(QuoteUpdate{
					Kind:     QuoteUpdateEFP,
					Snapshot: quote,
					EFP: new(QuoteEFP{
						TickType: m.TickType, BasisPoints: basisPoints,
						FormattedBasisPoints: m.FormattedBasisPoints,
						ImpliedFuturesPrice:  impliedFuturesPrice, HoldDays: m.HoldDays,
						FutureLastTradeDate: m.FutureLastTradeDate, DividendImpact: dividendImpact,
						DividendsToLastTradeDate: dividendsToLastTradeDate,
					}),
					ReceivedAt: time.Now().UTC(),
				})
			case codec.DeltaNeutralValidation:
				deltaNeutral, err := deltaNeutralFromCodec(&m.Contract)
				if err != nil {
					fail(err)
					return
				}
				sub.emit(QuoteUpdate{
					Kind:         QuoteUpdateDeltaNeutralValidation,
					Snapshot:     quote,
					DeltaNeutral: deltaNeutral,
					ReceivedAt:   time.Now().UTC(),
				})
			case codec.TickNews:
				timestamp, err := parseEpochMilliseconds(m.Time)
				if err != nil {
					fail(err)
					return
				}
				sub.emit(QuoteUpdate{
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
					fail(err)
					return
				}
				lastPricePrecision, err := parseOptionalDecimalPointer(m.LastPricePrecision, "quote parameters last price precision")
				if err != nil {
					fail(err)
					return
				}
				lastSizePrecision, err := parseOptionalDecimalPointer(m.LastSizePrecision, "quote parameters last size precision")
				if err != nil {
					fail(err)
					return
				}
				sub.emit(QuoteUpdate{
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
					fail(err)
					return
				}
				sub.emit(QuoteUpdate{
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
				sub.emitState(StreamSnapshotComplete, e.connectionSeq(), nil)
				if snapshot {
					e.deleteKeyedRoute(reqID)
					sub.closeWithErr(nil)
				}
			}
		}
		quoteRoute.handleAPIErr = func(m codec.APIError, e *engine) {
			if e.keyed[reqID] != quoteRoute {
				return
			}
			// 10167: delayed market data warning — the subscription
			// stays open and will receive delayed ticks.
			if m.Code == 10167 {
				sub.emitNotice(e.apiNotice(OpQuotes, m), e.connectionSeq())
				return
			}
			e.deleteKeyedRoute(reqID)
			sub.closeWithErr(e.apiErr(OpQuotes, m))
		}
		quoteRoute.onDisconnect = func(e *engine, err error) bool {
			if snapshot {
				sub.closeWithErr(ErrInterrupted)
				return false
			}
			if cfg.resume == ResumeAuto && e.cfg.reconnect == ReconnectAuto {
				quote = Quote{}
				sub.emitState(StreamGap, e.connectionSeq(), err)
				return true
			}
			sub.closeWithErr(ErrResumeRequired)
			return false
		}
		quoteRoute.emitGap = func(e *engine) {
			quote = Quote{}
			sub.emitState(StreamGap, e.connectionSeq(), nil)
		}
		quoteRoute.emitRestored = func(e *engine) {
			sub.emitState(StreamRestored, e.connectionSeq(), nil)
		}
		quoteRoute.emitResubscribed = func(e *engine) {
			sub.emitState(StreamResubscribed, e.connectionSeq(), nil)
		}
		e.keyed[reqID] = quoteRoute
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
		if err := validateContractFieldSupport(req.Contract, "real-time bars", e.serverVersion, contractFieldPrimaryExchange); err != nil {
			resp <- result{err: err}
			return
		}

		cfg, err := applySubscriptionOptionsFor(e.cfg, OpRealTimeBars, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		resumeContract := cloneContract(req.Contract)
		sub, ownedRoute := newKeyedSubscriptionRoute[Bar](
			e, cfg, reqID, OpRealTimeBars, codec.CancelRealTimeBars{ReqID: reqID},
		)

		ownedRoute.request = codec.RealTimeBarsRequest{
			ReqID:      reqID,
			Contract:   toCodecContract(req.Contract),
			WhatToShow: string(req.WhatToShow),
			UseRTH:     req.UseRTH,
		}
		ownedRoute.validateResume = func(e *engine) error {
			return validateContractFieldSupport(resumeContract, "resume real-time bars", e.serverVersion, contractFieldPrimaryExchange)
		}
		ownedRoute.handle = func(msg any, e *engine) {
			barMsg, ok := msg.(codec.RealTimeBar)
			if !ok {
				return
			}
			bar, err := fromCodecRealtimeBar(barMsg)
			if err != nil {
				sub.cancelFromActor(err)
				return
			}
			sub.emit(bar)
		}
		ownedRoute.handleAPIErr = func(m codec.APIError, e *engine) {
			if e.keyed[reqID] != ownedRoute {
				return
			}
			if m.Code == 10167 {
				sub.emitNotice(e.apiNotice(OpRealTimeBars, m), e.connectionSeq())
				return
			}
			e.deleteKeyedRoute(reqID)
			sub.closeWithErr(e.apiErr(OpRealTimeBars, m))
		}
		ownedRoute.onDisconnect = func(e *engine, err error) bool {
			if cfg.resume == ResumeAuto && e.cfg.reconnect == ReconnectAuto {
				sub.emitState(StreamGap, e.connectionSeq(), err)
				return true
			}
			sub.closeWithErr(ErrResumeRequired)
			return false
		}
		ownedRoute.emitGap = func(e *engine) {
			sub.emitState(StreamGap, e.connectionSeq(), nil)
		}
		ownedRoute.emitRestored = func(e *engine) {
			sub.emitState(StreamRestored, e.connectionSeq(), nil)
		}
		ownedRoute.emitResubscribed = func(e *engine) {
			sub.emitState(StreamResubscribed, e.connectionSeq(), nil)
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
		if err := validateContractFieldSupport(req.Contract, "market depth", e.serverVersion, depthContractFields(e.serverVersion)); err != nil {
			resp <- result{err: err}
			return
		}

		cfg, err := applySubscriptionOptionsFor(e.cfg, OpMarketDepth, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		rerouted := false
		sub, depthRoute := newKeyedSubscriptionRoute[DepthRow](
			e, cfg, reqID, OpMarketDepth,
			codec.CancelMarketDepth{ReqID: reqID, IsSmartDepth: req.IsSmartDepth},
		)

		depthRoute.request = codec.MarketDepthRequest{
			ReqID:        reqID,
			Contract:     toCodecContract(req.Contract),
			NumRows:      req.NumRows,
			IsSmartDepth: req.IsSmartDepth,
		}
		depthRoute.handle = func(msg any, e *engine) {
			switch m := msg.(type) {
			case codec.MarketDepthReroute:
				if rerouted {
					e.cancelAndCloseMarketDepthRoute(
						reqID,
						fmt.Errorf("ibkr: market depth request %d was rerouted more than once", reqID),
					)
					return
				}
				request := depthRoute.request.(codec.MarketDepthRequest)
				request.Contract = codec.Contract{ConID: m.ConID, Exchange: m.Exchange}
				depthRoute.request = request
				rerouted = true
				if err := e.send(request); err != nil {
					e.cancelAndCloseMarketDepthRoute(
						reqID,
						fmt.Errorf("ibkr: reroute market depth request %d: %w", reqID, err),
					)
				}
			case codec.MarketDepthUpdate:
				row, err := fromCodecMarketDepth(m)
				if err != nil {
					e.cancelAndCloseMarketDepthRoute(reqID, err)
					return
				}
				sub.emit(row)
			case codec.MarketDepthL2Update:
				row, err := fromCodecMarketDepthL2(m)
				if err != nil {
					e.cancelAndCloseMarketDepthRoute(reqID, err)
					return
				}
				sub.emit(row)
			}
		}
		depthRoute.handleAPIErr = func(m codec.APIError, e *engine) {
			if e.keyed[reqID] != depthRoute {
				return
			}
			if m.Code == ErrCodeSmartDepthExchanges {
				sub.emitNotice(e.apiNotice(OpMarketDepth, m), e.connectionSeq())
				return
			}
			e.deleteKeyedRoute(reqID)
			sub.closeWithErr(e.apiErr(OpMarketDepth, m))
		}
		e.keyed[reqID] = depthRoute
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

// cancelAndCloseMarketDepthRoute is the terminal actor-owned teardown for a
// depth stream whose local book can no longer be trusted. Cancellation must be
// admitted before local route deletion; if admission fails, the caller needs
// both the data-integrity failure and the uncertain remote-stream state.
func (e *engine) cancelAndCloseMarketDepthRoute(reqID int, cause error) {
	e.cancelAndCloseMarketDepthRoutes([]int{reqID}, cause)
}

func (e *engine) cancelAndCloseQuoteRoutes(reqIDs []int, cause error) {
	var retirementErrs []error
	for _, reqID := range reqIDs {
		quoteRoute, ok := e.keyed[reqID]
		if !ok || quoteRoute.opKind != OpQuotes {
			continue
		}
		cancelErr := e.cancelRouteSubscription(quoteRoute, OpQuotes, codec.CancelQuote{ReqID: reqID})
		e.deleteKeyedRoute(reqID)
		closeErr := cause
		if cancelErr != nil {
			closeErr = errors.Join(cause, cancelErr)
		}
		quoteRoute.close(closeErr)
		if cancelErr != nil {
			retirementErrs = append(retirementErrs, cancelErr)
		}
	}
	e.retireSubscriptionTransport(errors.Join(retirementErrs...))
}

func (e *engine) cancelAndCloseMarketDepthRoutes(reqIDs []int, cause error) {
	var retirementErrs []error
	for _, reqID := range reqIDs {
		depthRoute, ok := e.keyed[reqID]
		if !ok || depthRoute.opKind != OpMarketDepth {
			continue
		}
		request := depthRoute.request.(codec.MarketDepthRequest)
		cancelErr := e.cancelRouteSubscription(depthRoute, OpMarketDepth, codec.CancelMarketDepth{
			ReqID: reqID, IsSmartDepth: request.IsSmartDepth,
		})
		e.deleteKeyedRoute(reqID)
		closeErr := cause
		if cancelErr != nil {
			closeErr = errors.Join(cause, cancelErr)
		}
		depthRoute.close(closeErr)
		if cancelErr != nil {
			retirementErrs = append(retirementErrs, cancelErr)
		}
	}
	e.retireSubscriptionTransport(errors.Join(retirementErrs...))
}

func (e *engine) MktDepthExchanges(ctx context.Context) ([]DepthExchange, error) {
	type result struct {
		exchanges []DepthExchange
		err       error
	}
	resp := make(chan result, 1)
	var ownedRoute *route

	enqueueOneShotSetup(ctx, e, func() {
		if _, exists := e.singletons[singletonMktDepthExchanges]; exists {
			resp <- result{err: operationActive("market depth exchanges")}
			return
		}

		ownedRoute = &route{
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
							AggGroup: protocolIDFromInt[AggregateGroupID](x.AggGroup),
						}
					}
					resp <- result{exchanges: exchanges}
				}
			},
			onDisconnect: func(eng *engine, err error) bool {
				resp <- result{err: interrupted(err)}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		e.singletons[singletonMktDepthExchanges] = ownedRoute
		if err := e.sendContext(ctx, codec.MktDepthExchangesRequest{}); err != nil {
			delete(e.singletons, singletonMktDepthExchanges)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.abortUnresolvedSingletonOneShot(singletonMktDepthExchanges, ownedRoute) })
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
		if err := validateContractFieldSupport(req.Contract, "tick-by-tick data", e.serverVersion, contractFieldPrimaryExchange); err != nil {
			resp <- result{err: err}
			return
		}

		cfg, err := applySubscriptionOptionsFor(e.cfg, OpTickByTick, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		sub, ownedRoute := newKeyedSubscriptionRoute[TickByTickData](
			e, cfg, reqID, OpTickByTick, codec.CancelTickByTick{ReqID: reqID},
		)

		ownedRoute.request = codec.TickByTickRequest{
			ReqID: reqID, Contract: toCodecContract(req.Contract),
			TickType: string(req.TickType), NumberOfTicks: req.NumberOfTicks, IgnoreSize: req.IgnoreSize,
		}
		ownedRoute.handle = func(msg any, e *engine) {
			if m, ok := msg.(codec.TickByTickData); ok {
				fail := func(err error) { sub.cancelFromActor(err) }
				ts, err := parseTickByTickTime(m.Time)
				if err != nil {
					fail(err)
					return
				}
				tick := TickByTickData{Time: ts, TickType: m.TickType}
				switch m.TickType {
				case 1, 2:
					tick.Price, err = parseOptionalDecimal(m.Price, "tick by tick price")
					if err != nil {
						fail(err)
						return
					}
					tick.Size, err = parseOptionalDecimal(m.Size, "tick by tick size")
					if err != nil {
						fail(err)
						return
					}
					tick.Exchange = m.Exchange
					tick.SpecialConditions = m.SpecialConditions
				case 3:
					tick.BidPrice, err = parseOptionalDecimal(m.BidPrice, "tick by tick bid price")
					if err != nil {
						fail(err)
						return
					}
					tick.AskPrice, err = parseOptionalDecimal(m.AskPrice, "tick by tick ask price")
					if err != nil {
						fail(err)
						return
					}
					tick.BidSize, err = parseOptionalDecimal(m.BidSize, "tick by tick bid size")
					if err != nil {
						fail(err)
						return
					}
					tick.AskSize, err = parseOptionalDecimal(m.AskSize, "tick by tick ask size")
					if err != nil {
						fail(err)
						return
					}
				case 4:
					tick.MidPoint, err = parseOptionalDecimal(m.MidPoint, "tick by tick midpoint")
					if err != nil {
						fail(err)
						return
					}
				}
				sub.emit(tick)
			}
		}
		ownedRoute.handleAPIErr = func(m codec.APIError, e *engine) {
			if e.keyed[reqID] != ownedRoute {
				return
			}
			if m.Code == 10167 {
				sub.emitNotice(e.apiNotice(OpTickByTick, m), e.connectionSeq())
				return
			}
			e.deleteKeyedRoute(reqID)
			sub.closeWithErr(e.apiErr(OpTickByTick, m))
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
		if err := validateContractFieldSupport(req.Contract, "calculate implied volatility", e.serverVersion, contractFieldPrimaryExchange); err != nil {
			resp <- result{err: err}
			return
		}
		reqID = e.allocReqID()
		e.keyed[reqID] = newKeyedOneShotRoute(reqID, OpCalcImpliedVol,
			func(msg any, e *engine) {
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
			}, func(err error) {
				resp <- result{err: err}
			})
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
		if err := validateContractFieldSupport(req.Contract, "calculate option price", e.serverVersion, contractFieldPrimaryExchange); err != nil {
			resp <- result{err: err}
			return
		}
		reqID = e.allocReqID()
		e.keyed[reqID] = newKeyedOneShotRoute(reqID, OpCalcOptionPrice,
			func(msg any, e *engine) {
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
			}, func(err error) {
				resp <- result{err: err}
			})
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
	seconds, err := strconv.ParseInt(strings.TrimSpace(m.Time), 10, 64)
	if err != nil {
		return Bar{}, inboundProtocolError("real-time bar epoch", fmt.Errorf("parse %q: %w", m.Time, err))
	}
	return fromCodecBarAt(codec.HistoricalBar(m), time.Unix(seconds, 0).UTC())
}

func fromCodecMarketDepth(m codec.MarketDepthUpdate) (DepthRow, error) {
	price, err := decimal.NewFromString(m.Price)
	if err != nil {
		return DepthRow{}, inboundProtocolError("market depth price", err)
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
		return DepthRow{}, inboundProtocolError("market depth l2 price", err)
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
	case TickTypeOddLotBid:
		quote.OddLotBid = value
		quote.Available |= QuoteFieldOddLotBid
		return QuoteFieldOddLotBid
	case TickTypeOddLotAsk:
		quote.OddLotAsk = value
		quote.Available |= QuoteFieldOddLotAsk
		return QuoteFieldOddLotAsk
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
	case TickTypeOddLotBidSize:
		quote.OddLotBidSize = value
		quote.Available |= QuoteFieldOddLotBidSize
		return QuoteFieldOddLotBidSize
	case TickTypeOddLotAskSize:
		quote.OddLotAskSize = value
		quote.Available |= QuoteFieldOddLotAskSize
		return QuoteFieldOddLotAskSize
	default:
		return 0
	}
}

func applyTickString(quote *Quote, field int, value string) QuoteFields {
	switch field {
	case TickTypeOddLotBidExchange:
		quote.OddLotBidExchange = value
		quote.Available |= QuoteFieldOddLotBidExchange
		return QuoteFieldOddLotBidExchange
	case TickTypeOddLotAskExchange:
		quote.OddLotAskExchange = value
		quote.Available |= QuoteFieldOddLotAskExchange
		return QuoteFieldOddLotAskExchange
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

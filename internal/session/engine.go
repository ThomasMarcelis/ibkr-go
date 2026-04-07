package session

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/internal/transport"
	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

type Engine struct {
	cfg config

	cmds         chan func()
	incoming     chan any
	transportErr chan error
	ready        chan error
	done         chan struct{}
	events       chan Event

	waitMu  sync.Mutex
	waitErr error

	snapshotMu sync.RWMutex
	snapshot   Snapshot

	transport *transport.Conn

	keyed       map[int]*route
	singletons  map[string]*route
	orders      map[int64]*orderRoute
	executions  executionCorrelator
	execToOrder map[string]int64 // execID → orderID for commission routing to order handles

	nextReqID int

	bootstrap bootstrapState
	closed    bool
}

type bootstrapState struct {
	serverInfo    bool
	managed       bool
	nextValidID   bool
	readyReported bool
}

const (
	singletonPositions         = "positions"
	singletonOpenOrders        = "open_orders"
	singletonFamilyCodes       = "family_codes"
	singletonMktDepthExchanges = "mkt_depth_exchanges"
	singletonNewsProviders     = "news_providers"
	singletonScannerParameters = "scanner_parameters"
	singletonMarketRule        = "market_rule"
	singletonCompletedOrders   = "completed_orders"
	singletonAccountUpdates    = "account_updates"
	singletonNewsBulletins     = "news_bulletins"
	singletonFA                = "fa"
)

type route struct {
	opKind       OpKind
	subscription bool
	resume       ResumePolicy
	request      codec.Message
	handle       func(any, *Engine)
	handleAPIErr func(codec.APIError, *Engine)
	onDisconnect func(*Engine, error) bool
	emitGap      func(*Engine)
	emitResumed  func(*Engine)
	close        func(error)
	gapped       bool // true after Gap emitted, reset on Resumed; prevents double emission
}

type orderRoute struct {
	orderID int64
	handle  *OrderHandle
	closed  bool
	gapped  bool // true after Gap emitted, reset on Resumed; prevents double emission
}

func DialContext(ctx context.Context, opts ...Option) (*Engine, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.clientID < 0 {
		return nil, fmt.Errorf("ibkr: client id must be >= 0")
	}

	e := &Engine{
		cfg:          cfg,
		cmds:         make(chan func(), 256),
		incoming:     make(chan any, 256),
		transportErr: make(chan error, 8),
		ready:        make(chan error, 1),
		done:         make(chan struct{}),
		events:       make(chan Event, cfg.eventBuffer),
		keyed:        make(map[int]*route),
		singletons:   make(map[string]*route),
		orders:       make(map[int64]*orderRoute),
		executions:   newExecutionCorrelator(),
		execToOrder:  make(map[string]int64),
		nextReqID:    1,
		snapshot: Snapshot{
			State: StateDisconnected,
		},
	}

	go e.run()
	e.enqueue(func() {
		e.startConnect(ctx)
	})

	select {
	case err := <-e.ready:
		if err != nil {
			return nil, err
		}
		return e, nil
	case <-ctx.Done():
		_ = e.Close()
		return nil, ctx.Err()
	}
}

func (e *Engine) Close() error {
	e.enqueue(func() {
		e.closeEngine(ErrClosed)
	})
	return nil
}

func (e *Engine) Done() <-chan struct{} {
	return e.done
}

func (e *Engine) Wait() error {
	<-e.done
	e.waitMu.Lock()
	defer e.waitMu.Unlock()
	return e.waitErr
}

func (e *Engine) Session() Snapshot {
	e.snapshotMu.RLock()
	defer e.snapshotMu.RUnlock()

	snap := e.snapshot
	snap.ManagedAccounts = append([]string(nil), snap.ManagedAccounts...)
	return snap
}

func (e *Engine) SessionEvents() <-chan Event {
	return e.events
}

func (e *Engine) ContractDetails(ctx context.Context, req ContractDetailsRequest) ([]ContractDetails, error) {
	type result struct {
		values []ContractDetails
		err    error
	}

	resp := make(chan result, 1)
	var reqID int
	enqueueOneShotSetup(ctx, e, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}

		reqID = e.allocReqID()
		values := make([]ContractDetails, 0, 4)
		e.keyed[reqID] = &route{
			opKind:       OpContractDetails,
			subscription: false,
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.ContractDetails:
					values = append(values, fromCodecContractDetails(m))
				case codec.ContractDetailsEnd:
					delete(e.keyed, reqID)
					resp <- result{values: values}
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpContractDetails, m)}
			},
			onDisconnect: func(e *Engine, err error) bool {
				delete(e.keyed, reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.send(codec.ContractDetailsRequest{
			ReqID:    reqID,
			Contract: toCodecContract(req.Contract),
		}); err != nil {
			delete(e.keyed, reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.deleteKeyedRoute(reqID) })
	})
	if err != nil {
		return nil, err
	}
	return out.values, out.err
}

func (e *Engine) QualifyContract(ctx context.Context, contract Contract) (QualifiedContract, error) {
	details, err := e.ContractDetails(ctx, ContractDetailsRequest{Contract: contract})
	if err != nil {
		return QualifiedContract{}, err
	}
	switch len(details) {
	case 0:
		return QualifiedContract{}, ErrNoMatch
	case 1:
		return QualifiedContract{ContractDetails: details[0]}, nil
	default:
		return QualifiedContract{}, ErrAmbiguousContract
	}
}

func (e *Engine) HistoricalBars(ctx context.Context, req HistoricalBarsRequest) ([]Bar, error) {
	type result struct {
		values []Bar
		err    error
	}

	resp := make(chan result, 1)
	var reqID int
	enqueueOneShotSetup(ctx, e, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}

		reqID = e.allocReqID()
		values := make([]Bar, 0, 16)
		request, err := buildHistoricalBarsRequest(reqID, req)
		if err != nil {
			resp <- result{err: err}
			return
		}
		e.keyed[reqID] = &route{
			opKind: OpHistoricalBars,
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.HistoricalBar:
					bar, err := fromCodecBar(m)
					if err != nil {
						delete(e.keyed, reqID)
						resp <- result{err: err}
						return
					}
					values = append(values, bar)
				case codec.HistoricalBarsEnd:
					delete(e.keyed, reqID)
					resp <- result{values: values}
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpHistoricalBars, m)}
			},
			onDisconnect: func(e *Engine, err error) bool {
				delete(e.keyed, reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.send(request); err != nil {
			delete(e.keyed, reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.deleteKeyedRoute(reqID) })
	})
	if err != nil {
		return nil, err
	}
	return out.values, out.err
}

func (e *Engine) AccountSummary(ctx context.Context, req AccountSummaryRequest) ([]AccountValue, error) {
	sub, err := e.SubscribeAccountSummary(ctx, req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = sub.Close() }()
	return collectSnapshot(ctx, sub, func(update AccountSummaryUpdate) AccountValue { return update.Value })
}

func (e *Engine) SubscribeAccountSummary(ctx context.Context, req AccountSummaryRequest, opts ...SubscriptionOption) (*Subscription[AccountSummaryUpdate], error) {
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

		cfg := defaultSubscriptionConfig(e.cfg)
		for _, opt := range opts {
			opt(&cfg)
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

		e.keyed[reqID] = &route{
			opKind:       OpAccountSummary,
			subscription: true,
			resume:       cfg.resume,
			request:      plan.request,
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.AccountSummaryValue:
					if !plan.matches(m.Account) {
						return
					}
					sub.emit(AccountSummaryUpdate{
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
			handleAPIErr: func(m codec.APIError, e *Engine) {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpAccountSummary, m))
			},
			onDisconnect: func(e *Engine, err error) bool {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) {
				sub.closeWithErr(err)
			},
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.send(e.keyed[reqID].request); err != nil {
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

func (e *Engine) PositionsSnapshot(ctx context.Context) ([]Position, error) {
	sub, err := e.SubscribePositions(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = sub.Close() }()
	return collectSnapshot(ctx, sub, func(update PositionUpdate) Position { return update.Position })
}

func (e *Engine) SubscribePositions(ctx context.Context, opts ...SubscriptionOption) (*Subscription[PositionUpdate], error) {
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

		cfg := defaultSubscriptionConfig(e.cfg)
		for _, opt := range opts {
			opt(&cfg)
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

		e.singletons[singletonPositions] = &route{
			opKind:       OpPositions,
			subscription: true,
			resume:       cfg.resume,
			request:      codec.PositionsRequest{},
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.Position:
					position, err := fromCodecPosition(m)
					if err != nil {
						delete(e.singletons, singletonPositions)
						sub.closeWithErr(err)
						return
					}
					sub.emit(PositionUpdate{Position: position})
				case codec.PositionEnd:
					sub.emitState(SubscriptionStateEvent{Kind: SubscriptionSnapshotComplete, ConnectionSeq: e.connectionSeq()})
				}
			},
			onDisconnect: func(e *Engine, err error) bool {
				delete(e.singletons, singletonPositions)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) {
				sub.closeWithErr(err)
			},
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.send(codec.PositionsRequest{}); err != nil {
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

func (e *Engine) SetMarketDataType(dataType int) error {
	if dataType < 1 || dataType > 4 {
		return fmt.Errorf("invalid market data type %d: must be 1 (live), 2 (frozen), 3 (delayed), or 4 (delayed-frozen)", dataType)
	}
	return awaitFireAndForget(context.Background(), e, func() error {
		if !e.isReady() {
			return ErrNotReady
		}
		return e.send(codec.ReqMarketDataType{DataType: dataType})
	})
}

func (e *Engine) QuoteSnapshot(ctx context.Context, req QuoteSubscriptionRequest) (Quote, error) {
	req.Snapshot = true
	sub, err := e.SubscribeQuotes(ctx, req)
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
		case state, ok := <-sub.State():
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

func (e *Engine) SubscribeQuotes(ctx context.Context, req QuoteSubscriptionRequest, opts ...SubscriptionOption) (*Subscription[QuoteUpdate], error) {
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
		if err := validateQuoteRequest(req, cfg.resume); err != nil {
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
		quote := Quote{}

		e.keyed[reqID] = &route{
			opKind:       OpQuotes,
			subscription: true,
			resume:       cfg.resume,
			request: codec.QuoteRequest{
				ReqID:        reqID,
				Contract:     toCodecContract(req.Contract),
				Snapshot:     req.Snapshot,
				GenericTicks: append([]string(nil), req.GenericTicks...),
			},
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.TickPrice:
					changed, err := applyTickPrice(&quote, m.TickType, m.Price)
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					sub.emit(QuoteUpdate{Snapshot: quote, Changed: changed, ReceivedAt: time.Now().UTC()})
				case codec.TickSize:
					changed, err := applyTickSize(&quote, m.TickType, m.Size)
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					sub.emit(QuoteUpdate{Snapshot: quote, Changed: changed, ReceivedAt: time.Now().UTC()})
				case codec.MarketDataType:
					quote.MarketDataType = MarketDataType(m.DataType)
					quote.Available |= QuoteFieldMarketDataType
					sub.emit(QuoteUpdate{Snapshot: quote, Changed: QuoteFieldMarketDataType, ReceivedAt: time.Now().UTC()})
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
					if req.Snapshot {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(nil)
					}
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpQuotes, m))
			},
			onDisconnect: func(e *Engine, err error) bool {
				if req.Snapshot {
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
			emitGap: func(e *Engine) {
				sub.emitState(SubscriptionStateEvent{
					Kind:          SubscriptionGap,
					ConnectionSeq: e.connectionSeq(),
				})
			},
			emitResumed: func(e *Engine) {
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
		if err := e.send(e.keyed[reqID].request); err != nil {
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

func (e *Engine) SubscribeRealTimeBars(ctx context.Context, req RealTimeBarsRequest, opts ...SubscriptionOption) (*Subscription[Bar], error) {
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
				WhatToShow: req.WhatToShow,
				UseRTH:     req.UseRTH,
			},
			handle: func(msg any, e *Engine) {
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
				sub.emit(bar)
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpRealTimeBars, m))
			},
			onDisconnect: func(e *Engine, err error) bool {
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
			emitGap: func(e *Engine) {
				sub.emitState(SubscriptionStateEvent{
					Kind:          SubscriptionGap,
					ConnectionSeq: e.connectionSeq(),
				})
			},
			emitResumed: func(e *Engine) {
				sub.emitState(SubscriptionStateEvent{
					Kind:          SubscriptionResumed,
					ConnectionSeq: e.connectionSeq(),
				})
			},
			close: func(err error) { sub.closeWithErr(err) },
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.send(e.keyed[reqID].request); err != nil {
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

func (e *Engine) SubscribeMarketDepth(ctx context.Context, req MarketDepthRequest, opts ...SubscriptionOption) (*Subscription[DepthRow], error) {
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
				_ = e.send(codec.CancelMarketDepth{ReqID: reqID})
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
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.MarketDepthUpdate:
					row, err := fromCodecMarketDepth(m)
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					sub.emit(row)
				case codec.MarketDepthL2Update:
					row, err := fromCodecMarketDepthL2(m)
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					sub.emit(row)
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpMarketDepth, m))
			},
			onDisconnect: func(e *Engine, err error) bool {
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
			emitGap: func(e *Engine) {
				sub.emitState(SubscriptionStateEvent{
					Kind:          SubscriptionGap,
					ConnectionSeq: e.connectionSeq(),
				})
			},
			emitResumed: func(e *Engine) {
				sub.emitState(SubscriptionStateEvent{
					Kind:          SubscriptionResumed,
					ConnectionSeq: e.connectionSeq(),
				})
			},
			close: func(err error) { sub.closeWithErr(err) },
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.send(e.keyed[reqID].request); err != nil {
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

func (e *Engine) OpenOrdersSnapshot(ctx context.Context, scope OpenOrdersScope) ([]OpenOrder, error) {
	sub, err := e.SubscribeOpenOrders(ctx, scope)
	if err != nil {
		return nil, err
	}
	defer func() { _ = sub.Close() }()
	return collectSnapshot(ctx, sub, func(update OpenOrderUpdate) OpenOrder { return update.Order })
}

func (e *Engine) SubscribeOpenOrders(ctx context.Context, scope OpenOrdersScope, opts ...SubscriptionOption) (*Subscription[OpenOrderUpdate], error) {
	type result struct {
		sub *Subscription[OpenOrderUpdate]
		err error
	}
	resp := make(chan result, 1)
	enqueueSubscriptionSetup(ctx, e, resp, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		if err := validateOpenOrdersScope(scope, e.cfg.clientID); err != nil {
			resp <- result{err: err}
			return
		}
		if _, exists := e.singletons[singletonOpenOrders]; exists {
			resp <- result{err: fmt.Errorf("ibkr: open orders subscription already active")}
			return
		}

		cfg := defaultSubscriptionConfig(e.cfg)
		for _, opt := range opts {
			opt(&cfg)
		}
		if err := validateResumePolicy(OpOpenOrders, cfg.resume); err != nil {
			resp <- result{err: err}
			return
		}
		var sub *Subscription[OpenOrderUpdate]
		sub = newSubscription[OpenOrderUpdate](cfg, func() {
			e.enqueue(func() {
				if _, ok := e.singletons[singletonOpenOrders]; !ok {
					return
				}
				delete(e.singletons, singletonOpenOrders)
				sub.closeWithErr(nil)
			})
		})

		e.singletons[singletonOpenOrders] = &route{
			opKind:       OpOpenOrders,
			subscription: true,
			resume:       cfg.resume,
			request:      codec.OpenOrdersRequest{Scope: string(scope)},
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.OpenOrder:
					sub.emit(OpenOrderUpdate{Order: fromCodecOpenOrder(m)})
				case codec.OpenOrderEnd:
					sub.emitState(SubscriptionStateEvent{Kind: SubscriptionSnapshotComplete, ConnectionSeq: e.connectionSeq()})
				}
			},
			onDisconnect: func(e *Engine, err error) bool {
				delete(e.singletons, singletonOpenOrders)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) { sub.closeWithErr(err) },
		}

		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.send(codec.OpenOrdersRequest{Scope: string(scope)}); err != nil {
			delete(e.singletons, singletonOpenOrders)
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

func (e *Engine) Executions(ctx context.Context, req ExecutionsRequest) ([]ExecutionUpdate, error) {
	sub, err := e.SubscribeExecutions(ctx, req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = sub.Close() }()
	return collectSnapshot(ctx, sub, func(update ExecutionUpdate) ExecutionUpdate { return update })
}

func (e *Engine) SubscribeExecutions(ctx context.Context, req ExecutionsRequest, opts ...SubscriptionOption) (*Subscription[ExecutionUpdate], error) {
	type result struct {
		sub *Subscription[ExecutionUpdate]
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
		if err := validateResumePolicy(OpExecutions, cfg.resume); err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		var sub *Subscription[ExecutionUpdate]
		sub = newSubscription[ExecutionUpdate](cfg, func() {
			e.enqueue(func() {
				if _, ok := e.keyed[reqID]; !ok {
					return
				}
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(nil)
			})
		})
		e.executions.registerRoute(reqID, req)

		e.keyed[reqID] = &route{
			opKind:       OpExecutions,
			subscription: true,
			resume:       cfg.resume,
			request: codec.ExecutionsRequest{
				ReqID:   reqID,
				Account: req.Account,
				Symbol:  req.Symbol,
			},
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.ExecutionDetail:
					update, err := fromCodecExecution(m)
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					e.executions.observeExecution(reqID, m)
					sub.emit(update)
					if !e.emitUndeliveredExecutionCommissions(reqID, m.ExecID, sub) {
						return
					}
				case codec.ExecutionsEnd:
					sub.emitState(SubscriptionStateEvent{Kind: SubscriptionSnapshotComplete, ConnectionSeq: e.connectionSeq()})
					e.deleteKeyedRoute(reqID)
					sub.closeWithErr(nil)
				case codec.CommissionReport:
					if !e.emitUndeliveredExecutionCommissions(reqID, m.ExecID, sub) {
						return
					}
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpExecutions, m))
			},
			onDisconnect: func(e *Engine, err error) bool {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) { sub.closeWithErr(err) },
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.send(e.keyed[reqID].request); err != nil {
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

func (e *Engine) FamilyCodes(ctx context.Context) ([]FamilyCode, error) {
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
			handle: func(msg any, eng *Engine) {
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
			onDisconnect: func(eng *Engine, err error) bool {
				delete(eng.singletons, singletonFamilyCodes)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.send(codec.FamilyCodesRequest{}); err != nil {
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

func (e *Engine) MktDepthExchanges(ctx context.Context) ([]DepthExchange, error) {
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
			handle: func(msg any, eng *Engine) {
				switch m := msg.(type) {
				case codec.MktDepthExchanges:
					delete(eng.singletons, singletonMktDepthExchanges)
					exchanges := make([]DepthExchange, len(m.Exchanges))
					for i, x := range m.Exchanges {
						exchanges[i] = DepthExchange{
							Exchange: x.Exchange, SecType: x.SecType,
							ListingExch: x.ListingExch, ServiceDataType: x.ServiceDataType,
							AggGroup: x.AggGroup,
						}
					}
					resp <- result{exchanges: exchanges}
				}
			},
			onDisconnect: func(eng *Engine, err error) bool {
				delete(eng.singletons, singletonMktDepthExchanges)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.send(codec.MktDepthExchangesRequest{}); err != nil {
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

func (e *Engine) NewsProviders(ctx context.Context) ([]NewsProvider, error) {
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
			handle: func(msg any, eng *Engine) {
				switch m := msg.(type) {
				case codec.NewsProviders:
					delete(eng.singletons, singletonNewsProviders)
					providers := make([]NewsProvider, len(m.Providers))
					for i, p := range m.Providers {
						providers[i] = NewsProvider{Code: p.Code, Name: p.Name}
					}
					resp <- result{providers: providers}
				}
			},
			onDisconnect: func(eng *Engine, err error) bool {
				delete(eng.singletons, singletonNewsProviders)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.send(codec.NewsProvidersRequest{}); err != nil {
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

func (e *Engine) ScannerParameters(ctx context.Context) (string, error) {
	type result struct {
		xml string
		err error
	}
	resp := make(chan result, 1)

	enqueueOneShotSetup(ctx, e, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		if _, exists := e.singletons[singletonScannerParameters]; exists {
			resp <- result{err: fmt.Errorf("ibkr: scanner parameters request already in progress")}
			return
		}

		e.singletons[singletonScannerParameters] = &route{
			opKind: OpScannerParameters,
			handle: func(msg any, eng *Engine) {
				switch m := msg.(type) {
				case codec.ScannerParameters:
					delete(eng.singletons, singletonScannerParameters)
					resp <- result{xml: m.XML}
				}
			},
			onDisconnect: func(eng *Engine, err error) bool {
				delete(eng.singletons, singletonScannerParameters)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.send(codec.ScannerParametersRequest{}); err != nil {
			delete(e.singletons, singletonScannerParameters)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { delete(e.singletons, singletonScannerParameters) })
	})
	if err != nil {
		return "", err
	}
	return out.xml, out.err
}

func (e *Engine) UserInfo(ctx context.Context) (string, error) {
	type result struct {
		whiteBrandingID string
		err             error
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
			opKind: OpUserInfo,
			handle: func(msg any, eng *Engine) {
				switch m := msg.(type) {
				case codec.UserInfo:
					eng.deleteKeyedRoute(reqID)
					resp <- result{whiteBrandingID: m.WhiteBrandingID}
				}
			},
			handleAPIErr: func(m codec.APIError, eng *Engine) {
				eng.deleteKeyedRoute(reqID)
				resp <- result{err: eng.apiErr(OpUserInfo, m)}
			},
			onDisconnect: func(eng *Engine, err error) bool {
				eng.deleteKeyedRoute(reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.send(codec.UserInfoRequest{ReqID: reqID}); err != nil {
			e.deleteKeyedRoute(reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.deleteKeyedRoute(reqID) })
	})
	if err != nil {
		return "", err
	}
	return out.whiteBrandingID, out.err
}

func (e *Engine) MatchingSymbols(ctx context.Context, req MatchingSymbolsRequest) ([]MatchingSymbol, error) {
	type result struct {
		symbols []MatchingSymbol
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
			opKind: OpMatchingSymbols,
			handle: func(msg any, eng *Engine) {
				switch m := msg.(type) {
				case codec.MatchingSymbols:
					eng.deleteKeyedRoute(reqID)
					symbols := make([]MatchingSymbol, len(m.Symbols))
					for i, s := range m.Symbols {
						derivTypes := make([]string, len(s.DerivativeSecTypes))
						copy(derivTypes, s.DerivativeSecTypes)
						symbols[i] = MatchingSymbol{
							ConID: s.ConID, Symbol: s.Symbol, SecType: s.SecType,
							PrimaryExchange: s.PrimaryExchange, Currency: s.Currency,
							DerivativeSecTypes: derivTypes,
						}
					}
					resp <- result{symbols: symbols}
				}
			},
			handleAPIErr: func(m codec.APIError, eng *Engine) {
				eng.deleteKeyedRoute(reqID)
				resp <- result{err: eng.apiErr(OpMatchingSymbols, m)}
			},
			onDisconnect: func(eng *Engine, err error) bool {
				eng.deleteKeyedRoute(reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.send(codec.MatchingSymbolsRequest{ReqID: reqID, Pattern: req.Pattern}); err != nil {
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
	return out.symbols, out.err
}

func (e *Engine) HeadTimestamp(ctx context.Context, req HeadTimestampRequest) (time.Time, error) {
	type result struct {
		timestamp time.Time
		err       error
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
			opKind: OpHeadTimestamp,
			handle: func(msg any, eng *Engine) {
				switch m := msg.(type) {
				case codec.HeadTimestamp:
					timestamp, err := parseHeadTimestamp(m.Timestamp)
					if err != nil {
						eng.deleteKeyedRoute(reqID)
						resp <- result{err: err}
						return
					}
					eng.deleteKeyedRoute(reqID)
					resp <- result{timestamp: timestamp}
				}
			},
			handleAPIErr: func(m codec.APIError, eng *Engine) {
				eng.deleteKeyedRoute(reqID)
				resp <- result{err: eng.apiErr(OpHeadTimestamp, m)}
			},
			onDisconnect: func(eng *Engine, err error) bool {
				eng.deleteKeyedRoute(reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.send(codec.HeadTimestampRequest{
			ReqID:      reqID,
			Contract:   toCodecContract(req.Contract),
			WhatToShow: req.WhatToShow,
			UseRTH:     req.UseRTH,
		}); err != nil {
			e.deleteKeyedRoute(reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() {
			if _, ok := e.keyed[reqID]; ok {
				e.deleteKeyedRoute(reqID)
				_ = e.send(codec.CancelHeadTimestamp{ReqID: reqID})
			}
		})
	})
	if err != nil {
		return time.Time{}, err
	}
	return out.timestamp, out.err
}

func (e *Engine) MarketRule(ctx context.Context, marketRuleID int) (MarketRuleResult, error) {
	type result struct {
		rule MarketRuleResult
		err  error
	}
	resp := make(chan result, 1)

	enqueueOneShotSetup(ctx, e, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		if _, exists := e.singletons[singletonMarketRule]; exists {
			resp <- result{err: fmt.Errorf("ibkr: market rule request already in progress")}
			return
		}

		e.singletons[singletonMarketRule] = &route{
			opKind: OpMarketRule,
			handle: func(msg any, eng *Engine) {
				switch m := msg.(type) {
				case codec.MarketRule:
					delete(eng.singletons, singletonMarketRule)
					increments := make([]PriceIncrement, len(m.Increments))
					for i, inc := range m.Increments {
						lowEdge, _ := ParseDecimal(inc.LowEdge)
						increment, _ := ParseDecimal(inc.Increment)
						increments[i] = PriceIncrement{
							LowEdge:   lowEdge,
							Increment: increment,
						}
					}
					resp <- result{rule: MarketRuleResult{
						MarketRuleID: m.MarketRuleID,
						Increments:   increments,
					}}
				}
			},
			onDisconnect: func(eng *Engine, err error) bool {
				delete(eng.singletons, singletonMarketRule)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.send(codec.MarketRuleRequest{MarketRuleID: marketRuleID}); err != nil {
			delete(e.singletons, singletonMarketRule)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { delete(e.singletons, singletonMarketRule) })
	})
	if err != nil {
		return MarketRuleResult{}, err
	}
	return out.rule, out.err
}

func (e *Engine) CompletedOrders(ctx context.Context, apiOnly bool) ([]CompletedOrderResult, error) {
	type result struct {
		orders []CompletedOrderResult
		err    error
	}
	resp := make(chan result, 1)

	enqueueOneShotSetup(ctx, e, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		if _, exists := e.singletons[singletonCompletedOrders]; exists {
			resp <- result{err: fmt.Errorf("ibkr: completed orders request already in progress")}
			return
		}

		var collected []CompletedOrderResult

		e.singletons[singletonCompletedOrders] = &route{
			opKind: OpCompletedOrders,
			handle: func(msg any, eng *Engine) {
				switch m := msg.(type) {
				case codec.CompletedOrder:
					qty, _ := ParseDecimal(m.Quantity)
					filled, _ := ParseDecimal(m.Filled)
					remaining, _ := ParseDecimal(m.Remaining)
					collected = append(collected, CompletedOrderResult{
						Contract:  fromCodecContract(m.Contract),
						Action:    m.Action,
						OrderType: m.OrderType,
						Status:    m.Status,
						Quantity:  qty,
						Filled:    filled,
						Remaining: remaining,
					})
				case codec.CompletedOrderEnd:
					delete(eng.singletons, singletonCompletedOrders)
					resp <- result{orders: collected}
				}
			},
			onDisconnect: func(eng *Engine, err error) bool {
				delete(eng.singletons, singletonCompletedOrders)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.send(codec.CompletedOrdersRequest{APIOnly: apiOnly}); err != nil {
			delete(e.singletons, singletonCompletedOrders)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { delete(e.singletons, singletonCompletedOrders) })
	})
	if err != nil {
		return nil, err
	}
	return out.orders, out.err
}

// AccountUpdatesSnapshot subscribes, collects to AccountDownloadEnd, and closes.
func (e *Engine) AccountUpdatesSnapshot(ctx context.Context, account string) ([]AccountUpdate, error) {
	sub, err := e.SubscribeAccountUpdates(ctx, account)
	if err != nil {
		return nil, err
	}
	defer func() { _ = sub.Close() }()
	return collectSnapshot(ctx, sub, func(u AccountUpdate) AccountUpdate { return u })
}

// SubscribeAccountUpdates is a singleton subscription for account value/portfolio updates.
func (e *Engine) SubscribeAccountUpdates(ctx context.Context, account string, opts ...SubscriptionOption) (*Subscription[AccountUpdate], error) {
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

		cfg := defaultSubscriptionConfig(e.cfg)
		for _, opt := range opts {
			opt(&cfg)
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

		e.singletons[singletonAccountUpdates] = &route{
			opKind:       OpAccountUpdates,
			subscription: true,
			resume:       cfg.resume,
			request:      codec.AccountUpdatesRequest{Subscribe: true, Account: account},
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.UpdateAccountValue:
					sub.emit(AccountUpdate{AccountValue: &AccountUpdateValue{
						Key: m.Key, Value: m.Value, Currency: m.Currency, Account: m.Account,
					}})
				case codec.UpdatePortfolio:
					position, _ := ParseDecimal(m.Position)
					marketPrice, _ := ParseDecimal(m.MarketPrice)
					marketValue, _ := ParseDecimal(m.MarketValue)
					avgCost, _ := ParseDecimal(m.AvgCost)
					unrealizedPNL, _ := ParseDecimal(m.UnrealizedPNL)
					realizedPNL, _ := ParseDecimal(m.RealizedPNL)
					sub.emit(AccountUpdate{Portfolio: &PortfolioUpdate{
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
			onDisconnect: func(e *Engine, err error) bool {
				delete(e.singletons, singletonAccountUpdates)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) { sub.closeWithErr(err) },
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.send(codec.AccountUpdatesRequest{Subscribe: true, Account: account}); err != nil {
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
func (e *Engine) AccountUpdatesMultiSnapshot(ctx context.Context, req AccountUpdatesMultiRequest) ([]AccountUpdateMultiValue, error) {
	sub, err := e.SubscribeAccountUpdatesMulti(ctx, req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = sub.Close() }()
	return collectSnapshot(ctx, sub, func(u AccountUpdateMultiValue) AccountUpdateMultiValue { return u })
}

func (e *Engine) SubscribeAccountUpdatesMulti(ctx context.Context, req AccountUpdatesMultiRequest, opts ...SubscriptionOption) (*Subscription[AccountUpdateMultiValue], error) {
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

		cfg := defaultSubscriptionConfig(e.cfg)
		for _, opt := range opts {
			opt(&cfg)
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

		e.keyed[reqID] = &route{
			opKind:       OpAccountUpdatesMulti,
			subscription: true,
			resume:       cfg.resume,
			request:      codec.AccountUpdatesMultiRequest{ReqID: reqID, Account: req.Account, ModelCode: req.ModelCode},
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.AccountUpdateMultiValue:
					sub.emit(AccountUpdateMultiValue{
						Account: m.Account, ModelCode: m.ModelCode,
						Key: m.Key, Value: m.Value, Currency: m.Currency,
					})
				case codec.AccountUpdateMultiEnd:
					sub.emitState(SubscriptionStateEvent{Kind: SubscriptionSnapshotComplete, ConnectionSeq: e.connectionSeq()})
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpAccountUpdatesMulti, m))
			},
			onDisconnect: func(e *Engine, err error) bool {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) { sub.closeWithErr(err) },
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.send(e.keyed[reqID].request); err != nil {
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
func (e *Engine) PositionsMultiSnapshot(ctx context.Context, req PositionsMultiRequest) ([]PositionMulti, error) {
	sub, err := e.SubscribePositionsMulti(ctx, req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = sub.Close() }()
	return collectSnapshot(ctx, sub, func(u PositionMulti) PositionMulti { return u })
}

func (e *Engine) SubscribePositionsMulti(ctx context.Context, req PositionsMultiRequest, opts ...SubscriptionOption) (*Subscription[PositionMulti], error) {
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

		cfg := defaultSubscriptionConfig(e.cfg)
		for _, opt := range opts {
			opt(&cfg)
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

		e.keyed[reqID] = &route{
			opKind:       OpPositionsMulti,
			subscription: true,
			resume:       cfg.resume,
			request:      codec.PositionsMultiRequest{ReqID: reqID, Account: req.Account, ModelCode: req.ModelCode},
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.PositionMulti:
					position, _ := ParseDecimal(m.Position)
					avgCost, _ := ParseDecimal(m.AvgCost)
					sub.emit(PositionMulti{
						Account: m.Account, ModelCode: m.ModelCode,
						Contract: fromCodecContract(m.Contract),
						Position: position, AvgCost: avgCost,
					})
				case codec.PositionMultiEnd:
					sub.emitState(SubscriptionStateEvent{Kind: SubscriptionSnapshotComplete, ConnectionSeq: e.connectionSeq()})
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpPositionsMulti, m))
			},
			onDisconnect: func(e *Engine, err error) bool {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) { sub.closeWithErr(err) },
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.send(e.keyed[reqID].request); err != nil {
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

func (e *Engine) SubscribePnL(ctx context.Context, req PnLRequest, opts ...SubscriptionOption) (*Subscription[PnLUpdate], error) {
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

		cfg := defaultSubscriptionConfig(e.cfg)
		for _, opt := range opts {
			opt(&cfg)
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
			handle: func(msg any, e *Engine) {
				if m, ok := msg.(codec.PnLValue); ok {
					daily, _ := ParseDecimal(m.DailyPnL)
					unrealized, _ := ParseDecimal(m.UnrealizedPnL)
					realized, _ := ParseDecimal(m.RealizedPnL)
					sub.emit(PnLUpdate{DailyPnL: daily, UnrealizedPnL: unrealized, RealizedPnL: realized})
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpPnL, m))
			},
			onDisconnect: func(e *Engine, err error) bool {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) { sub.closeWithErr(err) },
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.send(e.keyed[reqID].request); err != nil {
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

func (e *Engine) SubscribePnLSingle(ctx context.Context, req PnLSingleRequest, opts ...SubscriptionOption) (*Subscription[PnLSingleUpdate], error) {
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

		cfg := defaultSubscriptionConfig(e.cfg)
		for _, opt := range opts {
			opt(&cfg)
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
			handle: func(msg any, e *Engine) {
				if m, ok := msg.(codec.PnLSingleValue); ok {
					pos, _ := ParseDecimal(m.Position)
					daily, _ := ParseDecimal(m.DailyPnL)
					unrealized, _ := ParseDecimal(m.UnrealizedPnL)
					realized, _ := ParseDecimal(m.RealizedPnL)
					value, _ := ParseDecimal(m.Value)
					sub.emit(PnLSingleUpdate{Position: pos, DailyPnL: daily, UnrealizedPnL: unrealized, RealizedPnL: realized, Value: value})
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpPnLSingle, m))
			},
			onDisconnect: func(e *Engine, err error) bool {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) { sub.closeWithErr(err) },
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.send(e.keyed[reqID].request); err != nil {
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

func (e *Engine) SubscribeTickByTick(ctx context.Context, req TickByTickRequest, opts ...SubscriptionOption) (*Subscription[TickByTickData], error) {
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
				TickType: req.TickType, NumberOfTicks: req.NumberOfTicks, IgnoreSize: req.IgnoreSize,
			},
			handle: func(msg any, e *Engine) {
				if m, ok := msg.(codec.TickByTickData); ok {
					ts, _ := parseTickByTickTime(m.Time)
					tick := TickByTickData{Time: ts, TickType: m.TickType}
					switch m.TickType {
					case 1, 2:
						tick.Price, _ = ParseDecimal(m.Price)
						tick.Size, _ = ParseDecimal(m.Size)
						tick.Exchange = m.Exchange
						tick.SpecialConditions = m.SpecialConditions
					case 3:
						tick.BidPrice, _ = ParseDecimal(m.BidPrice)
						tick.AskPrice, _ = ParseDecimal(m.AskPrice)
						tick.BidSize, _ = ParseDecimal(m.BidSize)
						tick.AskSize, _ = ParseDecimal(m.AskSize)
					case 4:
						tick.MidPoint, _ = ParseDecimal(m.MidPoint)
					}
					sub.emit(tick)
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpTickByTick, m))
			},
			onDisconnect: func(e *Engine, err error) bool {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) { sub.closeWithErr(err) },
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.send(e.keyed[reqID].request); err != nil {
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

// SubscribeNewsBulletins is a singleton subscription for news bulletins.
func (e *Engine) SubscribeNewsBulletins(ctx context.Context, allMessages bool, opts ...SubscriptionOption) (*Subscription[NewsBulletin], error) {
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

		cfg := defaultSubscriptionConfig(e.cfg)
		for _, opt := range opts {
			opt(&cfg)
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
			handle: func(msg any, e *Engine) {
				if m, ok := msg.(codec.NewsBulletin); ok {
					sub.emit(NewsBulletin{MsgID: m.MsgID, MsgType: m.MsgType, Headline: m.Headline, Source: m.Source})
				}
			},
			onDisconnect: func(e *Engine, err error) bool {
				delete(e.singletons, singletonNewsBulletins)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) { sub.closeWithErr(err) },
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.send(codec.NewsBulletinsRequest{AllMessages: allMessages}); err != nil {
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

// SubscribeHistoricalBars sends a historical bars request with keepUpToDate=true,
// returning initial bars as events, SnapshotComplete on the initial batch end,
// then streaming bar updates via IN 108.
func (e *Engine) SubscribeHistoricalBars(ctx context.Context, req HistoricalBarsRequest, opts ...SubscriptionOption) (*Subscription[Bar], error) {
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
		if err := validateResumePolicy(OpHistoricalBarsStream, cfg.resume); err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		codecReq, err := buildHistoricalBarsRequest(reqID, req)
		if err != nil {
			resp <- result{err: err}
			return
		}
		codecReq.KeepUpToDate = true

		var sub *Subscription[Bar]
		sub = newSubscription[Bar](cfg, func() {
			e.enqueue(func() {
				if _, ok := e.keyed[reqID]; !ok {
					return
				}
				e.deleteKeyedRoute(reqID)
				_ = e.send(codec.CancelHistoricalData{ReqID: reqID})
				sub.closeWithErr(nil)
			})
		})

		e.keyed[reqID] = &route{
			opKind:       OpHistoricalBarsStream,
			subscription: true,
			resume:       cfg.resume,
			request:      codecReq,
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.HistoricalBar:
					bar, err := fromCodecBar(m)
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					sub.emit(bar)
				case codec.HistoricalBarsEnd:
					sub.emitState(SubscriptionStateEvent{Kind: SubscriptionSnapshotComplete, ConnectionSeq: e.connectionSeq()})
				case codec.HistoricalDataUpdate:
					bar, err := fromCodecBar(codec.HistoricalBar{
						ReqID: m.ReqID, Time: m.Time, Open: m.Open, High: m.High,
						Low: m.Low, Close: m.Close, Volume: m.Volume, WAP: m.WAP, Count: m.Count,
					})
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					sub.emit(bar)
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpHistoricalBarsStream, m))
			},
			onDisconnect: func(e *Engine, err error) bool {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) { sub.closeWithErr(err) },
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.send(codecReq); err != nil {
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

func (e *Engine) SecDefOptParams(ctx context.Context, req SecDefOptParamsRequest) ([]SecDefOptParams, error) {
	type result struct {
		values []SecDefOptParams
		err    error
	}
	resp := make(chan result, 1)
	var reqID int
	enqueueOneShotSetup(ctx, e, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		reqID = e.allocReqID()
		values := make([]SecDefOptParams, 0, 4)
		e.keyed[reqID] = &route{
			opKind: OpSecDefOptParams,
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.SecDefOptParamsResponse:
					strikes := make([]Decimal, len(m.Strikes))
					for i, s := range m.Strikes {
						strikes[i], _ = ParseDecimal(s)
					}
					values = append(values, SecDefOptParams{
						Exchange:        m.Exchange,
						UnderlyingConID: m.UnderlyingConID,
						TradingClass:    m.TradingClass,
						Multiplier:      m.Multiplier,
						Expirations:     append([]string(nil), m.Expirations...),
						Strikes:         strikes,
					})
				case codec.SecDefOptParamsEnd:
					delete(e.keyed, reqID)
					resp <- result{values: values}
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpSecDefOptParams, m)}
			},
			onDisconnect: func(e *Engine, err error) bool {
				delete(e.keyed, reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.send(codec.SecDefOptParamsRequest{
			ReqID:             reqID,
			UnderlyingSymbol:  req.UnderlyingSymbol,
			FutFopExchange:    req.FutFopExchange,
			UnderlyingSecType: req.UnderlyingSecType,
			UnderlyingConID:   req.UnderlyingConID,
		}); err != nil {
			delete(e.keyed, reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.deleteKeyedRoute(reqID) })
	})
	if err != nil {
		return nil, err
	}
	return out.values, out.err
}

func (e *Engine) SmartComponents(ctx context.Context, bboExchange string) ([]SmartComponent, error) {
	type result struct {
		components []SmartComponent
		err        error
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
			opKind: OpSmartComponents,
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.SmartComponentsResponse:
					delete(e.keyed, reqID)
					components := make([]SmartComponent, len(m.Components))
					for i, c := range m.Components {
						components[i] = SmartComponent{
							BitNumber:      c.BitNumber,
							ExchangeName:   c.ExchangeName,
							ExchangeLetter: c.ExchangeLetter,
						}
					}
					resp <- result{components: components}
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpSmartComponents, m)}
			},
			onDisconnect: func(e *Engine, err error) bool {
				delete(e.keyed, reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.send(codec.SmartComponentsRequest{ReqID: reqID, BBOExchange: bboExchange}); err != nil {
			delete(e.keyed, reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.deleteKeyedRoute(reqID) })
	})
	if err != nil {
		return nil, err
	}
	return out.components, out.err
}

func (e *Engine) CalcImpliedVolatility(ctx context.Context, req CalcImpliedVolatilityRequest) (OptionComputation, error) {
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
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.TickOptionComputation:
					delete(e.keyed, reqID)
					resp <- result{value: fromCodecOptionComputation(m)}
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpCalcImpliedVol, m)}
			},
			onDisconnect: func(e *Engine, err error) bool {
				delete(e.keyed, reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.send(codec.CalcImpliedVolatilityRequest{
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

func (e *Engine) CalcOptionPrice(ctx context.Context, req CalcOptionPriceRequest) (OptionComputation, error) {
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
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.TickOptionComputation:
					delete(e.keyed, reqID)
					resp <- result{value: fromCodecOptionComputation(m)}
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpCalcOptionPrice, m)}
			},
			onDisconnect: func(e *Engine, err error) bool {
				delete(e.keyed, reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.send(codec.CalcOptionPriceRequest{
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

func (e *Engine) HistogramData(ctx context.Context, req HistogramDataRequest) ([]HistogramEntry, error) {
	type result struct {
		entries []HistogramEntry
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
			opKind: OpHistogramData,
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.HistogramDataResponse:
					delete(e.keyed, reqID)
					entries := make([]HistogramEntry, len(m.Entries))
					for i, entry := range m.Entries {
						entries[i].Price, _ = ParseDecimal(entry.Price)
						entries[i].Size, _ = ParseDecimal(entry.Size)
					}
					resp <- result{entries: entries}
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpHistogramData, m)}
			},
			onDisconnect: func(e *Engine, err error) bool {
				delete(e.keyed, reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.send(codec.HistogramDataRequest{
			ReqID:    reqID,
			Contract: toCodecContract(req.Contract),
			UseRTH:   req.UseRTH,
			Period:   req.Period,
		}); err != nil {
			delete(e.keyed, reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() {
			if _, ok := e.keyed[reqID]; ok {
				e.deleteKeyedRoute(reqID)
				_ = e.send(codec.CancelHistogramData{ReqID: reqID})
			}
		})
	})
	if err != nil {
		return nil, err
	}
	return out.entries, out.err
}

func (e *Engine) HistoricalTicks(ctx context.Context, req HistoricalTicksRequest) (HistoricalTicksResult, error) {
	type result struct {
		value HistoricalTicksResult
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
			opKind: OpHistoricalTicks,
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.HistoricalTicksResponse:
					e.deleteKeyedRoute(reqID)
					ticks := make([]HistoricalTick, len(m.Ticks))
					for i, t := range m.Ticks {
						parsedTime, err := parseEpochSeconds(t.Time)
						if err != nil {
							resp <- result{err: err}
							return
						}
						ticks[i].Time = parsedTime
						ticks[i].Price, _ = ParseDecimal(t.Price)
						ticks[i].Size, _ = ParseDecimal(t.Size)
					}
					resp <- result{value: HistoricalTicksResult{Ticks: ticks}}
				case codec.HistoricalTicksBidAskResponse:
					e.deleteKeyedRoute(reqID)
					ticks := make([]HistoricalTickBidAsk, len(m.Ticks))
					for i, t := range m.Ticks {
						parsedTime, err := parseEpochSeconds(t.Time)
						if err != nil {
							resp <- result{err: err}
							return
						}
						ticks[i].Time = parsedTime
						ticks[i].BidPrice, _ = ParseDecimal(t.BidPrice)
						ticks[i].AskPrice, _ = ParseDecimal(t.AskPrice)
						ticks[i].BidSize, _ = ParseDecimal(t.BidSize)
						ticks[i].AskSize, _ = ParseDecimal(t.AskSize)
					}
					resp <- result{value: HistoricalTicksResult{BidAsk: ticks}}
				case codec.HistoricalTicksLastResponse:
					e.deleteKeyedRoute(reqID)
					ticks := make([]HistoricalTickLast, len(m.Ticks))
					for i, t := range m.Ticks {
						parsedTime, err := parseEpochSeconds(t.Time)
						if err != nil {
							resp <- result{err: err}
							return
						}
						ticks[i].Time = parsedTime
						ticks[i].Price, _ = ParseDecimal(t.Price)
						ticks[i].Size, _ = ParseDecimal(t.Size)
						ticks[i].Exchange = t.Exchange
						ticks[i].SpecialConditions = t.SpecialConditions
					}
					resp <- result{value: HistoricalTicksResult{Last: ticks}}
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				e.deleteKeyedRoute(reqID)
				resp <- result{err: e.apiErr(OpHistoricalTicks, m)}
			},
			onDisconnect: func(e *Engine, err error) bool {
				e.deleteKeyedRoute(reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.send(codec.HistoricalTicksRequest{
			ReqID:         reqID,
			Contract:      toCodecContract(req.Contract),
			StartDateTime: req.StartDateTime,
			EndDateTime:   req.EndDateTime,
			NumberOfTicks: req.NumberOfTicks,
			WhatToShow:    req.WhatToShow,
			UseRTH:        req.UseRTH,
			IgnoreSize:    req.IgnoreSize,
		}); err != nil {
			e.deleteKeyedRoute(reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.deleteKeyedRoute(reqID) })
	})
	if err != nil {
		return HistoricalTicksResult{}, err
	}
	return out.value, out.err
}

func (e *Engine) NewsArticle(ctx context.Context, req NewsArticleRequest) (NewsArticle, error) {
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
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.NewsArticleResponse:
					delete(e.keyed, reqID)
					resp <- result{article: NewsArticle{ArticleType: m.ArticleType, ArticleText: m.ArticleText}}
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpNewsArticle, m)}
			},
			onDisconnect: func(e *Engine, err error) bool {
				delete(e.keyed, reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.send(codec.NewsArticleRequest{ReqID: reqID, ProviderCode: req.ProviderCode, ArticleID: req.ArticleID}); err != nil {
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

func (e *Engine) HistoricalNews(ctx context.Context, req HistoricalNewsRequest) ([]HistoricalNewsItem, error) {
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
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.HistoricalNewsItem:
					timestamp, err := parseEpochMilliseconds(m.Time)
					if err != nil {
						e.deleteKeyedRoute(reqID)
						resp <- result{err: err}
						return
					}
					collected = append(collected, HistoricalNewsItem{
						Time: timestamp, ProviderCode: m.ProviderCode,
						ArticleID: m.ArticleID, Headline: m.Headline,
					})
				case codec.HistoricalNewsEnd:
					e.deleteKeyedRoute(reqID)
					resp <- result{items: collected}
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				e.deleteKeyedRoute(reqID)
				resp <- result{err: e.apiErr(OpHistoricalNews, m)}
			},
			onDisconnect: func(e *Engine, err error) bool {
				e.deleteKeyedRoute(reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.send(codec.HistoricalNewsRequest{
			ReqID: reqID, ConID: req.ConID, ProviderCodes: req.ProviderCodes,
			StartDate: req.StartDate, EndDate: req.EndDate, TotalResults: req.TotalResults,
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

func (e *Engine) SubscribeScannerResults(ctx context.Context, req ScannerSubscriptionRequest, opts ...SubscriptionOption) (*Subscription[[]ScannerResult], error) {
	type result struct {
		sub *Subscription[[]ScannerResult]
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
		if err := validateResumePolicy(OpScannerSubscription, cfg.resume); err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		var sub *Subscription[[]ScannerResult]
		sub = newSubscription[[]ScannerResult](cfg, func() {
			e.enqueue(func() {
				if _, ok := e.keyed[reqID]; !ok {
					return
				}
				e.deleteKeyedRoute(reqID)
				_ = e.send(codec.CancelScannerSubscription{ReqID: reqID})
				sub.closeWithErr(nil)
			})
		})

		e.keyed[reqID] = &route{
			opKind:       OpScannerSubscription,
			subscription: true,
			resume:       cfg.resume,
			request: codec.ScannerSubscriptionRequest{
				ReqID:        reqID,
				NumberOfRows: req.NumberOfRows,
				Instrument:   req.Instrument,
				LocationCode: req.LocationCode,
				ScanCode:     req.ScanCode,
			},
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.ScannerDataResponse:
					results := make([]ScannerResult, len(m.Entries))
					for i, entry := range m.Entries {
						results[i] = ScannerResult{
							Rank:       entry.Rank,
							Contract:   fromCodecContract(entry.Contract),
							Distance:   entry.Distance,
							Benchmark:  entry.Benchmark,
							Projection: entry.Projection,
							LegsStr:    entry.LegsStr,
						}
					}
					sub.emit(results)
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpScannerSubscription, m))
			},
			onDisconnect: func(e *Engine, err error) bool {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) { sub.closeWithErr(err) },
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.send(e.keyed[reqID].request); err != nil {
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

// FA Configuration

func (e *Engine) RequestFA(ctx context.Context, faDataType int) (string, error) {
	type result struct {
		xml string
		err error
	}
	resp := make(chan result, 1)

	enqueueOneShotSetup(ctx, e, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		if _, exists := e.singletons[singletonFA]; exists {
			resp <- result{err: fmt.Errorf("ibkr: FA request already in progress")}
			return
		}

		e.singletons[singletonFA] = &route{
			opKind: OpFAConfig,
			handle: func(msg any, eng *Engine) {
				switch m := msg.(type) {
				case codec.ReceiveFA:
					delete(eng.singletons, singletonFA)
					resp <- result{xml: m.XML}
				}
			},
			onDisconnect: func(eng *Engine, err error) bool {
				delete(eng.singletons, singletonFA)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.send(codec.RequestFA{FADataType: faDataType}); err != nil {
			delete(e.singletons, singletonFA)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { delete(e.singletons, singletonFA) })
	})
	if err != nil {
		return "", err
	}
	return out.xml, out.err
}

func (e *Engine) ReplaceFA(ctx context.Context, faDataType int, xml string) error {
	return awaitFireAndForget(ctx, e, func() error {
		if !e.isReady() {
			return ErrNotReady
		}
		return e.send(codec.ReplaceFA{FADataType: faDataType, XML: xml})
	})
}

func (e *Engine) SoftDollarTiers(ctx context.Context) ([]SoftDollarTier, error) {
	type result struct {
		tiers []SoftDollarTier
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
			opKind: OpSoftDollarTiers,
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.SoftDollarTiersResponse:
					delete(e.keyed, reqID)
					tiers := make([]SoftDollarTier, len(m.Tiers))
					for i, t := range m.Tiers {
						tiers[i] = SoftDollarTier{Name: t.Name, Value: t.Value, DisplayName: t.DisplayName}
					}
					resp <- result{tiers: tiers}
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpSoftDollarTiers, m)}
			},
			onDisconnect: func(e *Engine, err error) bool {
				delete(e.keyed, reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.send(codec.SoftDollarTiersRequest{ReqID: reqID}); err != nil {
			delete(e.keyed, reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.deleteKeyedRoute(reqID) })
	})
	if err != nil {
		return nil, err
	}
	return out.tiers, out.err
}

// WSH Calendar Events

func (e *Engine) WSHMetaData(ctx context.Context) (string, error) {
	type result struct {
		dataJSON string
		err      error
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
			opKind: OpWSHMetaData,
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.WSHMetaDataResponse:
					delete(e.keyed, reqID)
					resp <- result{dataJSON: m.DataJSON}
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpWSHMetaData, m)}
			},
			onDisconnect: func(e *Engine, err error) bool {
				delete(e.keyed, reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.send(codec.WSHMetaDataRequest{ReqID: reqID}); err != nil {
			delete(e.keyed, reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.deleteKeyedRoute(reqID) })
	})
	if err != nil {
		return "", err
	}
	return out.dataJSON, out.err
}

func (e *Engine) WSHEventData(ctx context.Context, req WSHEventDataRequest) (string, error) {
	type result struct {
		dataJSON string
		err      error
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
			opKind: OpWSHEventData,
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.WSHEventDataResponse:
					delete(e.keyed, reqID)
					resp <- result{dataJSON: m.DataJSON}
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpWSHEventData, m)}
			},
			onDisconnect: func(e *Engine, err error) bool {
				delete(e.keyed, reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.send(codec.WSHEventDataRequest{
			ReqID:           reqID,
			ConID:           req.ConID,
			Filter:          req.Filter,
			FillWatchlist:   req.FillWatchlist,
			FillPortfolio:   req.FillPortfolio,
			FillCompetitors: req.FillCompetitors,
			StartDate:       req.StartDate,
			EndDate:         req.EndDate,
			TotalLimit:      req.TotalLimit,
		}); err != nil {
			delete(e.keyed, reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.deleteKeyedRoute(reqID) })
	})
	if err != nil {
		return "", err
	}
	return out.dataJSON, out.err
}

// Display Groups

func (e *Engine) QueryDisplayGroups(ctx context.Context) (string, error) {
	type result struct {
		groups string
		err    error
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
			opKind: OpDisplayGroups,
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.DisplayGroupList:
					delete(e.keyed, reqID)
					resp <- result{groups: m.Groups}
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpDisplayGroups, m)}
			},
			onDisconnect: func(e *Engine, err error) bool {
				delete(e.keyed, reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.send(codec.QueryDisplayGroupsRequest{ReqID: reqID}); err != nil {
			delete(e.keyed, reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.deleteKeyedRoute(reqID) })
	})
	if err != nil {
		return "", err
	}
	return out.groups, out.err
}

func (e *Engine) SubscribeDisplayGroup(ctx context.Context, groupID int, opts ...SubscriptionOption) (*DisplayGroupHandle, error) {
	type result struct {
		sub *Subscription[DisplayGroupUpdate]
		err error
	}
	resp := make(chan result, 1)
	var reqID int

	enqueueSubscriptionSetup(ctx, e, resp, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}

		cfg := defaultSubscriptionConfig(e.cfg)
		for _, opt := range opts {
			opt(&cfg)
		}
		if err := validateResumePolicy(OpDisplayGroupEvents, cfg.resume); err != nil {
			resp <- result{err: err}
			return
		}
		reqID = e.allocReqID()
		var sub *Subscription[DisplayGroupUpdate]
		sub = newSubscription[DisplayGroupUpdate](cfg, func() {
			e.enqueue(func() {
				if _, ok := e.keyed[reqID]; !ok {
					return
				}
				e.deleteKeyedRoute(reqID)
				_ = e.send(codec.UnsubscribeFromGroupEventsRequest{ReqID: reqID})
				sub.closeWithErr(nil)
			})
		})

		e.keyed[reqID] = &route{
			opKind:       OpDisplayGroupEvents,
			subscription: true,
			resume:       cfg.resume,
			request:      codec.SubscribeToGroupEventsRequest{ReqID: reqID, GroupID: groupID},
			handle: func(msg any, e *Engine) {
				if m, ok := msg.(codec.DisplayGroupUpdated); ok {
					sub.emit(DisplayGroupUpdate{ContractInfo: m.ContractInfo})
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpDisplayGroupEvents, m))
			},
			onDisconnect: func(e *Engine, err error) bool {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) { sub.closeWithErr(err) },
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.send(e.keyed[reqID].request); err != nil {
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
	if out.err != nil {
		return nil, out.err
	}
	handle := &DisplayGroupHandle{
		Subscription: out.sub,
		updateFn: func(ctx context.Context, contractInfo string) error {
			return e.updateDisplayGroup(ctx, reqID, contractInfo)
		},
	}
	return handle, nil
}

func (e *Engine) updateDisplayGroup(ctx context.Context, reqID int, contractInfo string) error {
	return awaitFireAndForget(ctx, e, func() error {
		if !e.isReady() {
			return ErrNotReady
		}
		return e.send(codec.UpdateDisplayGroupRequest{ReqID: reqID, ContractInfo: contractInfo})
	})
}

func fromCodecOptionComputation(m codec.TickOptionComputation) OptionComputation {
	iv, _ := ParseDecimal(m.ImpliedVol)
	delta, _ := ParseDecimal(m.Delta)
	optPrice, _ := ParseDecimal(m.OptPrice)
	pvDiv, _ := ParseDecimal(m.PvDividend)
	gamma, _ := ParseDecimal(m.Gamma)
	vega, _ := ParseDecimal(m.Vega)
	theta, _ := ParseDecimal(m.Theta)
	undPrice, _ := ParseDecimal(m.UndPrice)
	return OptionComputation{
		ImpliedVol: iv, Delta: delta, OptPrice: optPrice,
		PvDividend: pvDiv, Gamma: gamma, Vega: vega,
		Theta: theta, UndPrice: undPrice,
	}
}

// PlaceOrder submits a new order and returns an OrderHandle that tracks its
// lifecycle. The handle receives OpenOrder, OrderStatus, Execution, and
// Commission events via dual dispatch. The order can be modified or cancelled
// through the returned handle.
func (e *Engine) PlaceOrder(ctx context.Context, req PlaceOrderRequest) (*OrderHandle, error) {
	type result struct {
		handle *OrderHandle
		err    error
	}

	resp := make(chan result, 1)
	enqueueOneShotSetup(ctx, e, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}

		orderID := e.allocOrderID()
		handle := newOrderHandle(orderID)

		handle.cancelFn = func(ctx context.Context) error {
			ch := make(chan error, 1)
			e.enqueue(func() {
				if !e.isReady() {
					ch <- ErrNotReady
					return
				}
				ch <- e.send(codec.CancelOrderRequest{OrderID: orderID})
			})
			select {
			case err := <-ch:
				return err
			case <-ctx.Done():
				return ctx.Err()
			case <-e.done:
				return e.Wait()
			}
		}

		handle.modifyFn = func(ctx context.Context, order Order) error {
			return awaitFireAndForget(ctx, e, func() error {
				if !e.isReady() {
					return ErrNotReady
				}
				return e.send(toCodecPlaceOrder(orderID, PlaceOrderRequest{
					Contract: req.Contract,
					Order:    order,
				}))
			})
		}

		e.orders[orderID] = &orderRoute{orderID: orderID, handle: handle}

		if err := e.send(toCodecPlaceOrder(orderID, req)); err != nil {
			delete(e.orders, orderID)
			handle.closeWithErr(err)
			resp <- result{err: err}
			return
		}

		resp <- result{handle: handle}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, nil)
	if err != nil {
		return nil, err
	}
	return out.handle, out.err
}

// CancelOrder sends a cancel request for the given order ID. This is
// fire-and-forget; the cancellation result arrives via the OrderHandle's
// events channel as an OrderStatus with Status "Cancelled".
func (e *Engine) CancelOrder(ctx context.Context, orderID int64) error {
	return awaitFireAndForget(ctx, e, func() error {
		if !e.isReady() {
			return ErrNotReady
		}
		return e.send(codec.CancelOrderRequest{OrderID: orderID})
	})
}

// GlobalCancel requests cancellation of all open orders. This is
// fire-and-forget; individual cancellation results arrive via any active
// OrderHandle events channels.
func (e *Engine) GlobalCancel(ctx context.Context) error {
	return awaitFireAndForget(ctx, e, func() error {
		if !e.isReady() {
			return ErrNotReady
		}
		return e.send(codec.GlobalCancelRequest{})
	})
}

func (e *Engine) FundamentalData(ctx context.Context, req FundamentalDataRequest) (string, error) {
	type result struct {
		data string
		err  error
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
			opKind: OpFundamentalData,
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.FundamentalDataResponse:
					delete(e.keyed, reqID)
					resp <- result{data: m.Data}
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpFundamentalData, m)}
			},
			onDisconnect: func(e *Engine, err error) bool {
				delete(e.keyed, reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.send(codec.FundamentalDataRequest{
			ReqID:      reqID,
			Contract:   toCodecContract(req.Contract),
			ReportType: req.ReportType,
		}); err != nil {
			delete(e.keyed, reqID)
			resp <- result{err: err}
			return
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() {
			if _, ok := e.keyed[reqID]; ok {
				e.deleteKeyedRoute(reqID)
				_ = e.send(codec.CancelFundamentalData{ReqID: reqID})
			}
		})
	})
	if err != nil {
		return "", err
	}
	return out.data, out.err
}

func (e *Engine) ExerciseOptions(ctx context.Context, req ExerciseOptionsRequest) error {
	return awaitFireAndForget(ctx, e, func() error {
		if !e.isReady() {
			return ErrNotReady
		}
		override := 0
		if req.Override {
			override = 1
		}
		reqID := e.allocReqID()
		return e.send(codec.ExerciseOptionsRequest{
			ReqID:            reqID,
			Contract:         toCodecContract(req.Contract),
			ExerciseAction:   req.ExerciseAction,
			ExerciseQuantity: req.ExerciseQuantity,
			Account:          req.Account,
			Override:         override,
		})
	})
}

func toCodecPlaceOrder(orderID int64, req PlaceOrderRequest) codec.PlaceOrderRequest {
	return codec.PlaceOrderRequest{
		OrderID:  orderID,
		Contract: toCodecContract(req.Contract),

		Action:        string(req.Order.Action),
		TotalQuantity: decimalOrEmpty(req.Order.Quantity),
		OrderType:     req.Order.OrderType,
		LmtPrice:      decimalOrEmpty(req.Order.LmtPrice),
		AuxPrice:      decimalOrEmpty(req.Order.AuxPrice),

		TIF:        string(req.Order.TIF),
		OcaGroup:   req.Order.OcaGroup,
		Account:    req.Order.Account,
		Origin:     "0",
		OrderRef:   req.Order.OrderRef,
		Transmit:   optBoolToString(req.Order.Transmit, "1"),
		ParentID:   strconv.FormatInt(req.Order.ParentID, 10),
		OutsideRTH: boolToString(req.Order.OutsideRTH),

		ExemptCode:                  "-1",
		GoodAfterTime:               req.Order.GoodAfterTime,
		GoodTillDate:                req.Order.GoodTillDate,
		ConditionsCount:             "0",
		DeltaNeutralContractPresent: "0",
	}
}

func decimalOrEmpty(d Decimal) string {
	if d == (Decimal{}) {
		return ""
	}
	return d.String()
}

func boolToString(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func optBoolToString(b *bool, dflt string) string {
	if b == nil {
		return dflt
	}
	if *b {
		return "1"
	}
	return "0"
}

func (e *Engine) run() {
	for {
		for {
			select {
			case msg := <-e.incoming:
				e.handleIncoming(msg)
				continue
			default:
			}
			break
		}

		select {
		case fn := <-e.cmds:
			if fn != nil {
				fn()
			}
		case msg := <-e.incoming:
			e.handleIncoming(msg)
		case err := <-e.transportErr:
			if len(e.incoming) > 0 {
				go func(err error) {
					e.transportErr <- err
				}(err)
				continue
			}
			e.handleTransportLoss(err)
		case <-e.done:
			return
		}
	}
}

func (e *Engine) enqueue(fn func()) {
	select {
	case <-e.done:
		return
	case e.cmds <- fn:
	}
}

func (e *Engine) startConnect(ctx context.Context) {
	if e.closed {
		return
	}
	e.bootstrap = bootstrapState{}
	e.setState(StateConnecting, 0, "", nil)

	conn, err := e.cfg.dialer.DialContext(ctx, "tcp", net.JoinHostPort(e.cfg.host, strconv.Itoa(e.cfg.port)))
	if err != nil {
		e.reportReady(&ConnectError{Op: "dial", Err: err})
		e.closeEngine(&ConnectError{Op: "dial", Err: err})
		return
	}

	// Synchronous handshake before starting transport goroutines.
	deadline := time.Now().Add(10 * time.Second)

	// 1. Send API prefix (raw bytes, not framed)
	if err := transport.WriteRaw(conn, codec.EncodeHandshakePrefix()); err != nil {
		conn.Close()
		e.reportReady(&ConnectError{Op: "handshake", Err: err})
		e.closeEngine(&ConnectError{Op: "handshake", Err: err})
		return
	}

	// 2. Send version range (framed)
	if err := wire.WriteFrame(conn, codec.EncodeVersionRange(100, 200)); err != nil {
		conn.Close()
		e.reportReady(&ConnectError{Op: "handshake", Err: err})
		e.closeEngine(&ConnectError{Op: "handshake", Err: err})
		return
	}

	// 3. Read server info (framed, but no msg_id prefix)
	serverPayload, err := transport.ReadOneFrame(conn, deadline)
	if err != nil {
		conn.Close()
		e.reportReady(&ConnectError{Op: "handshake", Err: err})
		e.closeEngine(&ConnectError{Op: "handshake", Err: err})
		return
	}
	info, err := codec.DecodeServerInfo(serverPayload)
	if err != nil {
		conn.Close()
		e.reportReady(&ConnectError{Op: "handshake", Err: err})
		e.closeEngine(&ConnectError{Op: "handshake", Err: err})
		return
	}

	// 4. Version check
	if info.ServerVersion < e.cfg.minServerVersion {
		conn.Close()
		e.reportReady(ErrUnsupportedServerVersion)
		e.closeEngine(ErrUnsupportedServerVersion)
		return
	}
	e.updateSnapshot(func(s *Snapshot) {
		s.ServerVersion = info.ServerVersion
	})
	e.bootstrap.serverInfo = true

	// 5. Send START_API (framed normal message)
	startPayload, err := codec.Encode(codec.StartAPI{ClientID: e.cfg.clientID})
	if err != nil {
		conn.Close()
		e.reportReady(&ConnectError{Op: "handshake", Err: err})
		e.closeEngine(&ConnectError{Op: "handshake", Err: err})
		return
	}
	if err := wire.WriteFrame(conn, startPayload); err != nil {
		conn.Close()
		e.reportReady(&ConnectError{Op: "handshake", Err: err})
		e.closeEngine(&ConnectError{Op: "handshake", Err: err})
		return
	}

	// 6. Start async transport — ManagedAccounts + NextValidID arrive on incoming channel
	e.transport = transport.New(conn, e.cfg.logger, e.cfg.sendRate)
	e.attachTransport(e.transport)
	e.setState(StateHandshaking, 0, "", nil)
}

func (e *Engine) attachTransport(tr *transport.Conn) {
	decodedDone := make(chan struct{})
	go func() {
		defer close(decodedDone)
		for payload := range tr.Incoming() {
			msgs, err := codec.DecodeBatch(payload)
			if err != nil {
				e.transportErr <- &ProtocolError{Direction: "inbound", Err: err}
				return
			}
			for _, msg := range msgs {
				e.incoming <- msg
			}
		}
	}()

	go func() {
		<-tr.Done()
		<-decodedDone
		e.transportErr <- tr.Wait()
	}()
}

func (e *Engine) handleIncoming(msg any) {
	switch m := msg.(type) {
	case codec.ManagedAccounts:
		e.updateSnapshot(func(s *Snapshot) {
			s.ManagedAccounts = append([]string(nil), m.Accounts...)
		})
		e.bootstrap.managed = true
		e.maybeReady()
		return
	case codec.NextValidID:
		e.updateSnapshot(func(s *Snapshot) {
			s.NextValidID = m.OrderID
		})
		e.bootstrap.nextValidID = true
		e.maybeReady()
		return
	case codec.CurrentTime:
		if ts, err := time.Parse(time.RFC3339, m.Time); err == nil {
			e.updateSnapshot(func(s *Snapshot) {
				s.CurrentTime = ts
			})
		}
		return
	case codec.APIError:
		e.handleAPIError(m)
		return
	}

	if reqID, ok := messageReqID(msg); ok {
		if route, found := e.keyed[reqID]; found {
			route.handle(msg, e)
			// ExecutionDetail needs dual dispatch: keyed subscription + order handle.
			if m, ok := msg.(codec.ExecutionDetail); ok {
				e.dispatchExecutionToOrder(m)
			}
			return
		}
	}

	switch msg := msg.(type) {
	case codec.ExecutionDetail:
		// Unsolicited execution (reqID=-1 or no matching keyed route).
		e.dispatchExecutionToOrder(msg)

	case codec.Position, codec.PositionEnd:
		if route, ok := e.singletons[singletonPositions]; ok {
			route.handle(msg, e)
		}
	case codec.OpenOrder:
		// Dual dispatch: per-order route first, then singleton observer.
		if or, ok := e.orders[msg.OrderID]; ok && !or.closed {
			or.handle.emitOrder(fromCodecOpenOrder(msg))
		}
		if route, ok := e.singletons[singletonOpenOrders]; ok {
			route.handle(msg, e)
		}
	case codec.OrderStatus:
		if or, ok := e.orders[msg.OrderID]; ok && !or.closed {
			status := fromCodecOrderStatus(msg)
			or.handle.emitStatus(status)
			if IsTerminalOrderStatus(status.Status) {
				or.closed = true
			}
		}
		if route, ok := e.singletons[singletonOpenOrders]; ok {
			route.handle(msg, e)
		}
	case codec.OpenOrderEnd:
		if route, ok := e.singletons[singletonOpenOrders]; ok {
			route.handle(msg, e)
		}
	case codec.CommissionReport:
		e.routeCommissionReport(msg)
	case codec.FamilyCodes:
		if rt, ok := e.singletons[singletonFamilyCodes]; ok {
			rt.handle(msg, e)
		}
	case codec.MktDepthExchanges:
		if rt, ok := e.singletons[singletonMktDepthExchanges]; ok {
			rt.handle(msg, e)
		}
	case codec.NewsProviders:
		if rt, ok := e.singletons[singletonNewsProviders]; ok {
			rt.handle(msg, e)
		}
	case codec.ScannerParameters:
		if rt, ok := e.singletons[singletonScannerParameters]; ok {
			rt.handle(msg, e)
		}
	case codec.MarketRule:
		if rt, ok := e.singletons[singletonMarketRule]; ok {
			rt.handle(msg, e)
		}
	case codec.CompletedOrder, codec.CompletedOrderEnd:
		if rt, ok := e.singletons[singletonCompletedOrders]; ok {
			rt.handle(msg, e)
		}
	case codec.UpdateAccountValue, codec.UpdatePortfolio, codec.UpdateAccountTime, codec.AccountDownloadEnd:
		if rt, ok := e.singletons[singletonAccountUpdates]; ok {
			rt.handle(msg, e)
		}
	case codec.NewsBulletin:
		if rt, ok := e.singletons[singletonNewsBulletins]; ok {
			rt.handle(msg, e)
		}
	case codec.ReceiveFA:
		if rt, ok := e.singletons[singletonFA]; ok {
			rt.handle(msg, e)
		}
	}
}

func (e *Engine) handleAPIError(msg codec.APIError) {
	// Connectivity codes drive session state transitions.
	switch msg.Code {
	case 1100:
		e.setState(StateDegraded, msg.Code, msg.Message, nil)
		e.emitGap()
		return
	case 1101:
		e.setState(StateReady, msg.Code, msg.Message, nil)
		e.resumeRoutes()
		return
	case 1102:
		e.setState(StateReady, msg.Code, msg.Message, nil)
		e.emitResumed()
		return
	case 1300:
		if e.transport != nil {
			_ = e.transport.Close()
		}
		return
	}

	// 2xxx: bootstrap/farm-status informational codes (reqID -1).
	// Emitted as session events for observability; they never target a
	// request or subscription and must not interfere with bootstrap.
	if msg.Code >= 2000 && msg.Code < 3000 {
		e.emitEvent(msg.Code, msg.Message)
		return
	}

	// 10xxx: market-data warnings such as 10167 "displaying delayed data".
	// The reqID references the subscription that will receive degraded data,
	// but the subscription must stay open -- this is not a terminal error.
	if msg.Code >= 10000 && msg.Code < 20000 {
		e.emitEvent(msg.Code, msg.Message)
		return
	}

	// Request-specific errors (200, 420, etc.) are routed to the keyed
	// subscription that owns the reqID. If the route is already gone
	// (e.g., stale cancel response like code 300), the message is dropped.
	if msg.ReqID > 0 {
		if route, ok := e.keyed[msg.ReqID]; ok && route.handleAPIErr != nil {
			route.handleAPIErr(msg, e)
			return
		}
		// Order-specific API errors: the reqID field carries the orderID
		// for order rejections (e.g., code 201 "order rejected").
		if or, ok := e.orders[int64(msg.ReqID)]; ok && !or.closed {
			or.handle.emitOrderError(e.apiErr(OpPlaceOrder, msg))
			return
		}
	}
}

func (e *Engine) maybeReady() {
	if !e.bootstrap.serverInfo || !e.bootstrap.managed || !e.bootstrap.nextValidID {
		return
	}
	e.updateSnapshot(func(s *Snapshot) {
		s.ConnectionSeq++
	})
	e.setState(StateReady, 0, "", nil)
	e.reportReady(nil)
	e.resumeRoutes()
}

func (e *Engine) handleTransportLoss(err error) {
	if e.closed {
		return
	}
	if err == nil && e.transport == nil {
		return
	}
	e.transport = nil
	e.executions.reset()
	if e.cfg.reconnect == ReconnectOff {
		if err == nil {
			err = ErrClosed
		}
		e.disconnectRoutes(err)
		e.closeEngine(err)
		return
	}
	e.setState(StateReconnecting, 0, "transport lost", err)
	e.disconnectRoutes(err)

	time.AfterFunc(e.cfg.reconnectBackoff, func() {
		e.enqueue(func() {
			if e.closed {
				return
			}
			dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			e.startConnect(dialCtx)
		})
	})
}

func (e *Engine) disconnectRoutes(err error) {
	for reqID, route := range e.keyed {
		// Already gapped (e.g. from code 1100) — route survives, skip duplicate Gap.
		if route.gapped {
			continue
		}
		if route.onDisconnect == nil {
			route.close(ErrInterrupted)
			e.deleteKeyedRoute(reqID)
			continue
		}
		if !route.onDisconnect(e, err) {
			e.deleteKeyedRoute(reqID)
		}
	}
	for key, route := range e.singletons {
		if route.gapped {
			continue
		}
		if route.onDisconnect == nil {
			route.close(ErrInterrupted)
			delete(e.singletons, key)
			continue
		}
		if !route.onDisconnect(e, err) {
			delete(e.singletons, key)
		}
	}
	// Order handles survive disconnect: emit Gap, do not close.
	for _, or := range e.orders {
		if !or.closed && !or.gapped {
			or.gapped = true
			or.handle.emitState(SubscriptionStateEvent{Kind: SubscriptionGap, ConnectionSeq: e.connectionSeq()})
		}
	}
}

func (e *Engine) resumeRoutes() {
	for reqID, route := range e.keyed {
		if route.subscription && route.resume == ResumeAuto {
			route.gapped = false
			if err := e.send(route.request); err != nil {
				route.close(err)
				e.deleteKeyedRoute(reqID)
				continue
			}
			if route.emitResumed != nil {
				route.emitResumed(e)
			}
		}
	}
	for key, route := range e.singletons {
		if route.subscription && route.resume == ResumeAuto {
			route.gapped = false
			if err := e.send(route.request); err != nil {
				route.close(err)
				delete(e.singletons, key)
				continue
			}
			if route.emitResumed != nil {
				route.emitResumed(e)
			}
		}
	}
	// Emit Resumed to all active order handles after reconnect.
	for _, or := range e.orders {
		if !or.closed && or.gapped {
			or.gapped = false
			or.handle.emitState(SubscriptionStateEvent{Kind: SubscriptionResumed, ConnectionSeq: e.connectionSeq()})
		}
	}
}

func (e *Engine) closeEngine(err error) {
	if e.closed {
		return
	}
	e.closed = true
	if e.transport != nil {
		_ = e.transport.Close()
	}
	for reqID, route := range e.keyed {
		route.close(err)
		e.deleteKeyedRoute(reqID)
	}
	for key, route := range e.singletons {
		route.close(err)
		delete(e.singletons, key)
	}
	for id, or := range e.orders {
		if !or.closed {
			or.closed = true
			or.handle.closeWithErr(err)
		}
		delete(e.orders, id)
	}
	e.executions.reset()
	e.execToOrder = make(map[string]int64)
	e.setState(StateClosed, 0, "", err)
	e.reportReady(err)
	e.waitMu.Lock()
	e.waitErr = err
	e.waitMu.Unlock()
	close(e.done)
	close(e.events)
}

func (e *Engine) reportReady(err error) {
	if e.bootstrap.readyReported {
		return
	}
	e.bootstrap.readyReported = true
	e.ready <- err
}

func (e *Engine) setState(next State, code int, message string, err error) {
	e.snapshotMu.Lock()
	prev := e.snapshot.State
	e.snapshot.State = next
	connSeq := e.snapshot.ConnectionSeq
	e.snapshotMu.Unlock()

	select {
	case e.events <- Event{
		At:            time.Now().UTC(),
		State:         next,
		Previous:      prev,
		ConnectionSeq: connSeq,
		Code:          code,
		Message:       message,
		Err:           err,
	}:
	default:
	}
}

// emitEvent publishes an informational session event (e.g. farm-status
// or market-data warnings) without changing session state.
func (e *Engine) emitEvent(code int, message string) {
	e.snapshotMu.RLock()
	state := e.snapshot.State
	connSeq := e.snapshot.ConnectionSeq
	e.snapshotMu.RUnlock()
	select {
	case e.events <- Event{
		At:            time.Now().UTC(),
		State:         state,
		Previous:      state,
		ConnectionSeq: connSeq,
		Code:          code,
		Message:       message,
	}:
	default:
	}
}

func (e *Engine) updateSnapshot(update func(*Snapshot)) {
	e.snapshotMu.Lock()
	defer e.snapshotMu.Unlock()
	update(&e.snapshot)
}

func (e *Engine) send(msg codec.Message) error {
	if e.transport == nil {
		return ErrNotReady
	}
	payload, err := codec.Encode(msg)
	if err != nil {
		return err
	}
	return e.transport.Send(context.Background(), payload)
}

func (e *Engine) allocReqID() int {
	for {
		id := e.nextReqID
		e.nextReqID++
		if _, conflict := e.orders[int64(id)]; !conflict {
			return id
		}
	}
}

func (e *Engine) allocOrderID() int64 {
	for {
		id := e.snapshot.NextValidID
		e.updateSnapshot(func(s *Snapshot) {
			s.NextValidID++
		})
		if _, conflict := e.keyed[int(id)]; !conflict {
			return id
		}
	}
}

func (e *Engine) connectionSeq() uint64 {
	e.snapshotMu.RLock()
	defer e.snapshotMu.RUnlock()
	return e.snapshot.ConnectionSeq
}

func (e *Engine) isReady() bool {
	e.snapshotMu.RLock()
	state := e.snapshot.State
	e.snapshotMu.RUnlock()
	return state == StateReady || state == StateDegraded
}

func (e *Engine) apiErr(opKind OpKind, msg codec.APIError) error {
	return &APIError{
		Code:          msg.Code,
		Message:       msg.Message,
		OpKind:        opKind,
		ConnectionSeq: e.connectionSeq(),
	}
}

func (e *Engine) emitGap() {
	for _, route := range e.keyed {
		if route.subscription && route.resume == ResumeAuto && route.emitGap != nil && !route.gapped {
			route.gapped = true
			route.emitGap(e)
		}
	}
	for _, route := range e.singletons {
		if route.subscription && route.resume == ResumeAuto && route.emitGap != nil && !route.gapped {
			route.gapped = true
			route.emitGap(e)
		}
	}
	for _, or := range e.orders {
		if !or.closed && !or.gapped {
			or.gapped = true
			or.handle.emitState(SubscriptionStateEvent{Kind: SubscriptionGap, ConnectionSeq: e.connectionSeq()})
		}
	}
}

func (e *Engine) emitResumed() {
	for _, route := range e.keyed {
		if route.subscription && route.resume == ResumeAuto && route.emitResumed != nil {
			route.gapped = false
			route.emitResumed(e)
		}
	}
	for _, route := range e.singletons {
		if route.subscription && route.resume == ResumeAuto && route.emitResumed != nil {
			route.gapped = false
			route.emitResumed(e)
		}
	}
	for _, or := range e.orders {
		if !or.closed && or.gapped {
			or.gapped = false
			or.handle.emitState(SubscriptionStateEvent{Kind: SubscriptionResumed, ConnectionSeq: e.connectionSeq()})
		}
	}
}

func (e *Engine) activeAccountSummarySubscriptions() int {
	count := 0
	for _, route := range e.keyed {
		if route.subscription && route.opKind == OpAccountSummary {
			count++
		}
	}
	return count
}

func (e *Engine) deleteKeyedRoute(reqID int) {
	route, ok := e.keyed[reqID]
	if !ok {
		return
	}
	delete(e.keyed, reqID)
	if route.opKind == OpExecutions {
		e.executions.unregisterRoute(reqID)
	}
}

func (e *Engine) routeCommissionReport(report codec.CommissionReport) {
	for _, reqID := range e.executions.recordCommission(report) {
		route, found := e.keyed[reqID]
		if !found || route.opKind != OpExecutions {
			continue
		}
		route.handle(report, e)
	}
	// Also dispatch to the order handle that owns this execution.
	if orderID, ok := e.execToOrder[report.ExecID]; ok {
		if or, ok := e.orders[orderID]; ok && !or.closed {
			cr, err := fromCodecCommission(report)
			if err == nil {
				or.handle.emitCommission(cr)
			}
		}
	}
}

func (e *Engine) dispatchExecutionToOrder(m codec.ExecutionDetail) {
	if m.OrderID == 0 {
		return
	}
	or, ok := e.orders[m.OrderID]
	if !ok || or.closed {
		return
	}
	exec, err := fromCodecExecution(m)
	if err != nil {
		return
	}
	or.handle.emitExecution(*exec.Execution)
	e.execToOrder[m.ExecID] = m.OrderID
}

func (e *Engine) undeliveredCommissions(reqID int, execID string) []codec.CommissionReport {
	return e.executions.undeliveredCommissions(reqID, execID)
}

func (e *Engine) emitUndeliveredExecutionCommissions(reqID int, execID string, sub *Subscription[ExecutionUpdate]) bool {
	for _, commissionMsg := range e.undeliveredCommissions(reqID, execID) {
		report, err := fromCodecCommission(commissionMsg)
		if err != nil {
			e.deleteKeyedRoute(reqID)
			sub.closeWithErr(err)
			return false
		}
		sub.emit(ExecutionUpdate{Commission: &report})
	}
	return true
}

func messageReqID(msg any) (int, bool) {
	switch m := msg.(type) {
	case codec.ContractDetails:
		return m.ReqID, true
	case codec.ContractDetailsEnd:
		return m.ReqID, true
	case codec.HistoricalBar:
		return m.ReqID, true
	case codec.HistoricalBarsEnd:
		return m.ReqID, true
	case codec.AccountSummaryValue:
		return m.ReqID, true
	case codec.AccountSummaryEnd:
		return m.ReqID, true
	case codec.TickPrice:
		return m.ReqID, true
	case codec.TickSize:
		return m.ReqID, true
	case codec.TickGeneric:
		return m.ReqID, true
	case codec.TickString:
		return m.ReqID, true
	case codec.TickReqParams:
		return m.ReqID, true
	case codec.MarketDataType:
		return m.ReqID, true
	case codec.TickSnapshotEnd:
		return m.ReqID, true
	case codec.RealTimeBar:
		return m.ReqID, true
	case codec.ExecutionDetail:
		return m.ReqID, true
	case codec.ExecutionsEnd:
		return m.ReqID, true
	case codec.UserInfo:
		return m.ReqID, true
	case codec.MatchingSymbols:
		return m.ReqID, true
	case codec.HeadTimestamp:
		return m.ReqID, true
	case codec.AccountUpdateMultiValue:
		return m.ReqID, true
	case codec.AccountUpdateMultiEnd:
		return m.ReqID, true
	case codec.PositionMulti:
		return m.ReqID, true
	case codec.PositionMultiEnd:
		return m.ReqID, true
	case codec.PnLValue:
		return m.ReqID, true
	case codec.PnLSingleValue:
		return m.ReqID, true
	case codec.TickByTickData:
		return m.ReqID, true
	case codec.HistoricalDataUpdate:
		return m.ReqID, true
	case codec.SecDefOptParamsResponse:
		return m.ReqID, true
	case codec.SecDefOptParamsEnd:
		return m.ReqID, true
	case codec.SmartComponentsResponse:
		return m.ReqID, true
	case codec.TickOptionComputation:
		return m.ReqID, true
	case codec.HistogramDataResponse:
		return m.ReqID, true
	case codec.HistoricalTicksResponse:
		return m.ReqID, true
	case codec.HistoricalTicksBidAskResponse:
		return m.ReqID, true
	case codec.HistoricalTicksLastResponse:
		return m.ReqID, true
	case codec.NewsArticleResponse:
		return m.ReqID, true
	case codec.HistoricalNewsItem:
		return m.ReqID, true
	case codec.HistoricalNewsEnd:
		return m.ReqID, true
	case codec.ScannerDataResponse:
		return m.ReqID, true
	case codec.SoftDollarTiersResponse:
		return m.ReqID, true
	case codec.WSHMetaDataResponse:
		return m.ReqID, true
	case codec.WSHEventDataResponse:
		return m.ReqID, true
	case codec.DisplayGroupList:
		return m.ReqID, true
	case codec.DisplayGroupUpdated:
		return m.ReqID, true
	case codec.MarketDepthUpdate:
		return m.ReqID, true
	case codec.MarketDepthL2Update:
		return m.ReqID, true
	case codec.FundamentalDataResponse:
		return m.ReqID, true
	default:
		return 0, false
	}
}

func bindContext[T any](ctx context.Context, sub *Subscription[T]) {
	go func() {
		select {
		case <-ctx.Done():
			_ = sub.Close()
		case <-sub.Done():
		}
	}()
}

func collectSnapshot[T any, U any](ctx context.Context, sub *Subscription[T], mapFn func(T) U) ([]U, error) {
	values := make([]U, 0, 8)
	for {
		select {
		case item, ok := <-sub.Events():
			if !ok {
				if sub.snapshotComplete() {
					return values, nil
				}
				return values, sub.Wait()
			}
			values = append(values, mapFn(item))
		case state, ok := <-sub.State():
			if !ok {
				if sub.snapshotComplete() {
					return drainSnapshotEvents(values, sub, mapFn), nil
				}
				return values, sub.Wait()
			}
			switch state.Kind {
			case SubscriptionSnapshotComplete:
				return drainSnapshotEvents(values, sub, mapFn), nil
			case SubscriptionClosed:
				if sub.snapshotComplete() {
					return drainSnapshotEvents(values, sub, mapFn), nil
				}
				return values, state.Err
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func drainSnapshotEvents[T any, U any](values []U, sub *Subscription[T], mapFn func(T) U) []U {
	for {
		select {
		case item, ok := <-sub.Events():
			if !ok {
				return values
			}
			values = append(values, mapFn(item))
		default:
			return values
		}
	}
}

func toCodecContract(c Contract) codec.Contract {
	return codec.Contract{
		ConID:           c.ConID,
		Symbol:          c.Symbol,
		SecType:         c.SecType,
		Expiry:          c.Expiry,
		Strike:          c.Strike,
		Right:           c.Right,
		Multiplier:      c.Multiplier,
		Exchange:        c.Exchange,
		Currency:        c.Currency,
		LocalSymbol:     c.LocalSymbol,
		TradingClass:    c.TradingClass,
		PrimaryExchange: c.PrimaryExchange,
	}
}

func fromCodecContract(c codec.Contract) Contract {
	return Contract{
		ConID:           c.ConID,
		Symbol:          c.Symbol,
		SecType:         c.SecType,
		Expiry:          c.Expiry,
		Strike:          c.Strike,
		Right:           c.Right,
		Multiplier:      c.Multiplier,
		Exchange:        c.Exchange,
		Currency:        c.Currency,
		LocalSymbol:     c.LocalSymbol,
		TradingClass:    c.TradingClass,
		PrimaryExchange: c.PrimaryExchange,
	}
}

func fromCodecContractDetails(m codec.ContractDetails) ContractDetails {
	minTick, _ := ParseDecimal(m.MinTick)
	return ContractDetails{
		Contract:   fromCodecContract(m.Contract),
		MarketName: m.MarketName,
		LongName:   m.LongName,
		MinTick:    minTick,
		TimeZoneID: m.TimeZoneID,
	}
}

func fromCodecBar(m codec.HistoricalBar) (Bar, error) {
	ts, err := parseBarTime(m.Time)
	if err != nil {
		return Bar{}, err
	}
	open, err := ParseDecimal(m.Open)
	if err != nil {
		return Bar{}, err
	}
	high, err := ParseDecimal(m.High)
	if err != nil {
		return Bar{}, err
	}
	low, err := ParseDecimal(m.Low)
	if err != nil {
		return Bar{}, err
	}
	closeValue, err := ParseDecimal(m.Close)
	if err != nil {
		return Bar{}, err
	}
	volume, err := ParseDecimal(m.Volume)
	if err != nil {
		return Bar{}, err
	}
	wap, _ := ParseDecimal(m.WAP)
	count, _ := strconv.Atoi(m.Count)
	return Bar{Time: ts, Open: open, High: high, Low: low, Close: closeValue, Volume: volume, WAP: wap, Count: count}, nil
}

// parseBarTime handles both RFC3339 (from testhost transcripts) and IBKR native
// bar date formats ("20260402  09:30:00" or "20260402" for daily bars).
func parseBarTime(raw string) (time.Time, error) {
	// Try RFC3339 first (for backward compat with test transcripts)
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts, nil
	}
	// IBKR intraday format: "20260402  09:30:00" (note: double space, no timezone)
	if ts, err := time.Parse("20060102  15:04:05", raw); err == nil {
		return ts, nil
	}
	// IBKR daily format: "20260402"
	if ts, err := time.Parse("20060102", raw); err == nil {
		return ts, nil
	}
	// IBKR format with timezone: "20260402 09:30:00 US/Eastern"
	// Strip timezone suffix and parse the datetime prefix.
	if len(raw) > 17 {
		if ts, err := time.Parse("20060102 15:04:05", raw[:17]); err == nil {
			return ts, nil
		}
	}
	return time.Time{}, fmt.Errorf("ibkr: parse bar time %q", raw)
}

func fromCodecRealtimeBar(m codec.RealTimeBar) (Bar, error) {
	return fromCodecBar(codec.HistoricalBar(m))
}

func fromCodecMarketDepth(m codec.MarketDepthUpdate) (DepthRow, error) {
	price, err := ParseDecimal(m.Price)
	if err != nil {
		return DepthRow{}, fmt.Errorf("ibkr: market depth price: %w", err)
	}
	size, err := ParseDecimal(m.Size)
	if err != nil {
		return DepthRow{}, fmt.Errorf("ibkr: market depth size: %w", err)
	}
	return DepthRow{
		Position:  m.Position,
		Operation: m.Operation,
		Side:      m.Side,
		Price:     price,
		Size:      size,
	}, nil
}

func fromCodecMarketDepthL2(m codec.MarketDepthL2Update) (DepthRow, error) {
	price, err := ParseDecimal(m.Price)
	if err != nil {
		return DepthRow{}, fmt.Errorf("ibkr: market depth l2 price: %w", err)
	}
	size, err := ParseDecimal(m.Size)
	if err != nil {
		return DepthRow{}, fmt.Errorf("ibkr: market depth l2 size: %w", err)
	}
	return DepthRow{
		Position:     m.Position,
		MarketMaker:  m.MarketMaker,
		Operation:    m.Operation,
		Side:         m.Side,
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

func parseEpochSeconds(raw string) (time.Time, error) {
	epoch, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("ibkr: parse epoch seconds %q", raw)
	}
	return time.Unix(epoch, 0).UTC(), nil
}

func parseEpochMilliseconds(raw string) (time.Time, error) {
	epoch, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("ibkr: parse epoch milliseconds %q", raw)
	}
	return time.UnixMilli(epoch).UTC(), nil
}

func parseHeadTimestamp(raw string) (time.Time, error) {
	ts, err := time.Parse("20060102-15:04:05", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("ibkr: parse head timestamp %q", raw)
	}
	return ts.UTC(), nil
}

func fromCodecPosition(m codec.Position) (Position, error) {
	position, err := ParseDecimal(m.Position)
	if err != nil {
		return Position{}, err
	}
	avgCost, err := ParseDecimal(m.AvgCost)
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

func fromCodecOpenOrder(m codec.OpenOrder) OpenOrder {
	// Lenient decimal parsing: the v200 OpenOrder message has ~170 fields
	// and skip counts may not be exact for all order types. Parse what we
	// can; zero values are acceptable for read-only observation.
	quantity, _ := ParseDecimal(m.Quantity)
	filled, _ := ParseDecimal(m.Filled)
	remaining, _ := ParseDecimal(m.Remaining)
	lmtPrice, _ := ParseDecimal(m.LmtPrice)
	auxPrice, _ := ParseDecimal(m.AuxPrice)
	origin, _ := strconv.Atoi(m.Origin)
	clientID, _ := strconv.Atoi(m.ClientID)
	permID, _ := strconv.ParseInt(m.PermID, 10, 64)
	parentID, _ := strconv.ParseInt(m.ParentID, 10, 64)
	return OpenOrder{
		OrderID:       m.OrderID,
		Account:       m.Account,
		Contract:      fromCodecContract(m.Contract),
		Action:        m.Action,
		OrderType:     m.OrderType,
		Status:        m.Status,
		Quantity:      quantity,
		Filled:        filled,
		Remaining:     remaining,
		LmtPrice:      lmtPrice,
		AuxPrice:      auxPrice,
		TIF:           m.TIF,
		OcaGroup:      m.OcaGroup,
		OpenClose:     m.OpenClose,
		Origin:        origin,
		OrderRef:      m.OrderRef,
		ClientID:      clientID,
		PermID:        permID,
		OutsideRTH:    m.OutsideRTH == "1",
		Hidden:        m.Hidden == "1",
		GoodAfterTime: m.GoodAfterTime,
		ParentID:      parentID,
	}
}

func fromCodecOrderStatus(m codec.OrderStatus) OrderStatusUpdate {
	filled, _ := ParseDecimal(m.Filled)
	remaining, _ := ParseDecimal(m.Remaining)
	avgFillPrice, _ := ParseDecimal(m.AvgFillPrice)
	lastFillPrice, _ := ParseDecimal(m.LastFillPrice)
	mktCapPrice, _ := ParseDecimal(m.MktCapPrice)
	permID, _ := strconv.ParseInt(m.PermID, 10, 64)
	parentID, _ := strconv.ParseInt(m.ParentID, 10, 64)
	clientID, _ := strconv.Atoi(m.ClientID)
	return OrderStatusUpdate{
		OrderID:       m.OrderID,
		Status:        m.Status,
		Filled:        filled,
		Remaining:     remaining,
		AvgFillPrice:  avgFillPrice,
		PermID:        permID,
		ParentID:      parentID,
		LastFillPrice: lastFillPrice,
		ClientID:      clientID,
		WhyHeld:       m.WhyHeld,
		MktCapPrice:   mktCapPrice,
	}
}

func fromCodecExecution(m codec.ExecutionDetail) (ExecutionUpdate, error) {
	shares, err := ParseDecimal(m.Shares)
	if err != nil {
		return ExecutionUpdate{}, err
	}
	price, err := ParseDecimal(m.Price)
	if err != nil {
		return ExecutionUpdate{}, err
	}
	ts, err := time.Parse(time.RFC3339, m.Time)
	if err != nil {
		return ExecutionUpdate{}, err
	}
	return ExecutionUpdate{
		Execution: &Execution{
			OrderID: m.OrderID,
			ExecID:  m.ExecID,
			Account: m.Account,
			Symbol:  m.Symbol,
			Side:    m.Side,
			Shares:  shares,
			Price:   price,
			Time:    ts,
		},
	}, nil
}

func fromCodecCommission(m codec.CommissionReport) (CommissionReport, error) {
	commission, err := ParseDecimal(m.Commission)
	if err != nil {
		return CommissionReport{}, err
	}
	realized, err := ParseDecimal(m.RealizedPNL)
	if err != nil {
		return CommissionReport{}, err
	}
	return CommissionReport{
		ExecID:      m.ExecID,
		Commission:  commission,
		Currency:    m.Currency,
		RealizedPNL: realized,
	}, nil
}

func applyTickPrice(quote *Quote, field int, raw string) (QuoteFields, error) {
	value, err := ParseDecimal(raw)
	if err != nil {
		return 0, err
	}
	switch field {
	case 1, 66: // bid
		quote.Bid = value
		quote.Available |= QuoteFieldBid
		return QuoteFieldBid, nil
	case 2, 67: // ask
		quote.Ask = value
		quote.Available |= QuoteFieldAsk
		return QuoteFieldAsk, nil
	case 4, 68: // last
		quote.Last = value
		quote.Available |= QuoteFieldLast
		return QuoteFieldLast, nil
	case 6, 72: // high
		quote.High = value
		quote.Available |= QuoteFieldHigh
		return QuoteFieldHigh, nil
	case 7, 73: // low
		quote.Low = value
		quote.Available |= QuoteFieldLow
		return QuoteFieldLow, nil
	case 9, 75: // close
		quote.Close = value
		quote.Available |= QuoteFieldClose
		return QuoteFieldClose, nil
	case 14, 76: // open
		quote.Open = value
		quote.Available |= QuoteFieldOpen
		return QuoteFieldOpen, nil
	default:
		return 0, nil
	}
}

func applyTickSize(quote *Quote, field int, raw string) (QuoteFields, error) {
	value, err := ParseDecimal(raw)
	if err != nil {
		return 0, err
	}
	switch field {
	case 0, 69: // bid_size
		quote.BidSize = value
		quote.Available |= QuoteFieldBidSize
		return QuoteFieldBidSize, nil
	case 3, 70: // ask_size
		quote.AskSize = value
		quote.Available |= QuoteFieldAskSize
		return QuoteFieldAskSize, nil
	case 5, 71: // last_size
		quote.LastSize = value
		quote.Available |= QuoteFieldLastSize
		return QuoteFieldLastSize, nil
	case 8, 74: // volume
		return 0, nil
	default:
		return 0, nil
	}
}

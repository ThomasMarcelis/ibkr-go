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

	keyed      map[int]*keyedRoute
	singletons map[string]*singletonRoute

	nextReqID int

	bootstrap bootstrapState
	closed    bool
}

type bootstrapState struct {
	helloAck      bool
	managed       bool
	nextValidID   bool
	readyReported bool
}

type keyedRoute struct {
	opKind       OpKind
	subscription bool
	resume       ResumePolicy
	request      codec.Message
	cancel       codec.Message
	handle       func(any, *Engine)
	handleAPIErr func(codec.APIError, *Engine)
	onDisconnect func(*Engine, error) bool
	onReconnect  func(*Engine)
	close        func(error)
}

type singletonRoute struct {
	opKind       OpKind
	subscription bool
	resume       ResumePolicy
	request      codec.Message
	cancel       codec.Message
	handle       func(any, *Engine)
	handleAPIErr func(codec.APIError, *Engine)
	onDisconnect func(*Engine, error) bool
	onReconnect  func(*Engine)
	close        func(error)
}

func DialContext(ctx context.Context, opts ...Option) (*Engine, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	e := &Engine{
		cfg:          cfg,
		cmds:         make(chan func(), 256),
		incoming:     make(chan any, 256),
		transportErr: make(chan error, 8),
		ready:        make(chan error, 1),
		done:         make(chan struct{}),
		events:       make(chan Event, cfg.eventBuffer),
		keyed:        make(map[int]*keyedRoute),
		singletons:   make(map[string]*singletonRoute),
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
	e.enqueue(func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}

		reqID = e.allocReqID()
		values := make([]ContractDetails, 0, 4)
		e.keyed[reqID] = &keyedRoute{
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

	select {
	case out := <-resp:
		return out.values, out.err
	case <-ctx.Done():
		e.enqueue(func() { delete(e.keyed, reqID) })
		return nil, ctx.Err()
	}
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
	e.enqueue(func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}

		reqID = e.allocReqID()
		values := make([]Bar, 0, 16)
		e.keyed[reqID] = &keyedRoute{
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
		if err := e.send(codec.HistoricalBarsRequest{
			ReqID:       reqID,
			Contract:    toCodecContract(req.Contract),
			EndDateTime: req.EndTime.UTC().Format(time.RFC3339),
			Duration:    req.Duration.String(),
			BarSize:     req.BarSize.String(),
			WhatToShow:  req.WhatToShow,
			UseRTH:      req.UseRTH,
		}); err != nil {
			delete(e.keyed, reqID)
			resp <- result{err: err}
		}
	})

	select {
	case out := <-resp:
		return out.values, out.err
	case <-ctx.Done():
		e.enqueue(func() { delete(e.keyed, reqID) })
		return nil, ctx.Err()
	}
}

func (e *Engine) AccountSummary(ctx context.Context, req AccountSummaryRequest) ([]AccountValue, error) {
	sub, err := e.SubscribeAccountSummary(ctx, req)
	if err != nil {
		return nil, err
	}
	defer sub.Close()
	return collectSnapshot(ctx, sub, func(update AccountSummaryUpdate) AccountValue { return update.Value })
}

func (e *Engine) SubscribeAccountSummary(ctx context.Context, req AccountSummaryRequest, opts ...SubscriptionOption) (*Subscription[AccountSummaryUpdate], error) {
	type result struct {
		sub *Subscription[AccountSummaryUpdate]
		err error
	}
	resp := make(chan result, 1)

	e.enqueue(func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}

		cfg := defaultSubscriptionConfig(e.cfg)
		for _, opt := range opts {
			opt(&cfg)
		}

		reqID := e.allocReqID()
		var sub *Subscription[AccountSummaryUpdate]
		snapshotComplete := false
		sub = newSubscription[AccountSummaryUpdate](cfg, func() {
			e.enqueue(func() {
				if _, ok := e.keyed[reqID]; !ok {
					return
				}
				delete(e.keyed, reqID)
				_ = e.send(codec.CancelAccountSummary{ReqID: reqID})
				sub.closeWithErr(nil)
			})
		})

		e.keyed[reqID] = &keyedRoute{
			opKind:       OpAccountSummary,
			subscription: true,
			resume:       cfg.resume,
			request: codec.AccountSummaryRequest{
				ReqID:   reqID,
				Account: req.Account,
				Tags:    append([]string(nil), req.Tags...),
			},
			cancel: codec.CancelAccountSummary{ReqID: reqID},
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.AccountSummaryValue:
					sub.emit(AccountSummaryUpdate{
						Value: AccountValue{
							Account:  m.Account,
							Tag:      m.Tag,
							Value:    m.Value,
							Currency: m.Currency,
						},
					})
				case codec.AccountSummaryEnd:
					snapshotComplete = true
					sub.emitState(SubscriptionStateEvent{
						Kind:          SubscriptionSnapshotComplete,
						ConnectionSeq: e.Session().ConnectionSeq,
					})
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				delete(e.keyed, reqID)
				sub.closeWithErr(e.apiErr(OpAccountSummary, m))
			},
			onDisconnect: func(e *Engine, err error) bool {
				if snapshotComplete {
					delete(e.keyed, reqID)
					sub.closeWithErr(nil)
					return false
				}
				if cfg.resume == ResumeAuto {
					sub.emitState(SubscriptionStateEvent{Kind: SubscriptionGap, ConnectionSeq: e.Session().ConnectionSeq, Err: err})
					return true
				}
				delete(e.keyed, reqID)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			onReconnect: func(e *Engine) {
				sub.emitState(SubscriptionStateEvent{Kind: SubscriptionResumed, ConnectionSeq: e.Session().ConnectionSeq})
			},
			close: func(err error) {
				sub.closeWithErr(err)
			},
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.Session().ConnectionSeq})
		if err := e.send(e.keyed[reqID].request); err != nil {
			delete(e.keyed, reqID)
			sub.closeWithErr(err)
			resp <- result{err: err}
			return
		}
		resp <- result{sub: sub}
	})

	select {
	case out := <-resp:
		if out.err == nil && out.sub != nil {
			bindContext(ctx, out.sub)
		}
		return out.sub, out.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (e *Engine) PositionsSnapshot(ctx context.Context) ([]Position, error) {
	sub, err := e.SubscribePositions(ctx)
	if err != nil {
		return nil, err
	}
	defer sub.Close()
	return collectSnapshot(ctx, sub, func(update PositionUpdate) Position { return update.Position })
}

func (e *Engine) SubscribePositions(ctx context.Context, opts ...SubscriptionOption) (*Subscription[PositionUpdate], error) {
	type result struct {
		sub *Subscription[PositionUpdate]
		err error
	}
	resp := make(chan result, 1)

	e.enqueue(func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		if _, exists := e.singletons["positions"]; exists {
			resp <- result{err: fmt.Errorf("ibkr: positions subscription already active")}
			return
		}

		cfg := defaultSubscriptionConfig(e.cfg)
		for _, opt := range opts {
			opt(&cfg)
		}
		var sub *Subscription[PositionUpdate]
		snapshotComplete := false
		sub = newSubscription[PositionUpdate](cfg, func() {
			e.enqueue(func() {
				if _, ok := e.singletons["positions"]; !ok {
					return
				}
				delete(e.singletons, "positions")
				_ = e.send(codec.CancelPositions{})
				sub.closeWithErr(nil)
			})
		})

		e.singletons["positions"] = &singletonRoute{
			opKind:       OpPositions,
			subscription: true,
			resume:       cfg.resume,
			request:      codec.PositionsRequest{},
			cancel:       codec.CancelPositions{},
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.Position:
					position, err := fromCodecPosition(m)
					if err != nil {
						delete(e.singletons, "positions")
						sub.closeWithErr(err)
						return
					}
					sub.emit(PositionUpdate{Position: position})
				case codec.PositionEnd:
					snapshotComplete = true
					sub.emitState(SubscriptionStateEvent{Kind: SubscriptionSnapshotComplete, ConnectionSeq: e.Session().ConnectionSeq})
				}
			},
			onDisconnect: func(e *Engine, err error) bool {
				if snapshotComplete {
					delete(e.singletons, "positions")
					sub.closeWithErr(nil)
					return false
				}
				if cfg.resume == ResumeAuto {
					sub.emitState(SubscriptionStateEvent{Kind: SubscriptionGap, ConnectionSeq: e.Session().ConnectionSeq, Err: err})
					return true
				}
				delete(e.singletons, "positions")
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			onReconnect: func(e *Engine) {
				sub.emitState(SubscriptionStateEvent{Kind: SubscriptionResumed, ConnectionSeq: e.Session().ConnectionSeq})
			},
			close: func(err error) {
				sub.closeWithErr(err)
			},
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.Session().ConnectionSeq})
		if err := e.send(codec.PositionsRequest{}); err != nil {
			delete(e.singletons, "positions")
			sub.closeWithErr(err)
			resp <- result{err: err}
			return
		}
		resp <- result{sub: sub}
	})

	select {
	case out := <-resp:
		if out.err == nil && out.sub != nil {
			bindContext(ctx, out.sub)
		}
		return out.sub, out.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (e *Engine) QuoteSnapshot(ctx context.Context, req QuoteSubscriptionRequest) (Quote, error) {
	req.Snapshot = true
	sub, err := e.SubscribeQuotes(ctx, req)
	if err != nil {
		return Quote{}, err
	}
	defer sub.Close()

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

	e.enqueue(func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}

		cfg := defaultSubscriptionConfig(e.cfg)
		for _, opt := range opts {
			opt(&cfg)
		}
		reqID := e.allocReqID()
		var sub *Subscription[QuoteUpdate]
		snapshotComplete := false
		sub = newSubscription[QuoteUpdate](cfg, func() {
			e.enqueue(func() {
				if _, ok := e.keyed[reqID]; !ok {
					return
				}
				delete(e.keyed, reqID)
				_ = e.send(codec.CancelQuote{ReqID: reqID})
				sub.closeWithErr(nil)
			})
		})
		quote := Quote{}

		e.keyed[reqID] = &keyedRoute{
			opKind:       OpQuotes,
			subscription: true,
			resume:       cfg.resume,
			request: codec.QuoteRequest{
				ReqID:        reqID,
				Contract:     toCodecContract(req.Contract),
				Snapshot:     req.Snapshot,
				GenericTicks: append([]string(nil), req.GenericTicks...),
			},
			cancel: codec.CancelQuote{ReqID: reqID},
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.TickPrice:
					changed, err := applyTickPrice(&quote, m.Field, m.Price)
					if err != nil {
						delete(e.keyed, reqID)
						sub.closeWithErr(err)
						return
					}
					sub.emit(QuoteUpdate{Snapshot: quote, Changed: changed, ReceivedAt: time.Now().UTC()})
				case codec.TickSize:
					changed, err := applyTickSize(&quote, m.Field, m.Size)
					if err != nil {
						delete(e.keyed, reqID)
						sub.closeWithErr(err)
						return
					}
					sub.emit(QuoteUpdate{Snapshot: quote, Changed: changed, ReceivedAt: time.Now().UTC()})
				case codec.MarketDataType:
					quote.MarketDataType = MarketDataType(m.DataType)
					quote.Available |= QuoteFieldMarketDataType
					sub.emit(QuoteUpdate{Snapshot: quote, Changed: QuoteFieldMarketDataType, ReceivedAt: time.Now().UTC()})
				case codec.TickSnapshotEnd:
					snapshotComplete = true
					sub.emitState(SubscriptionStateEvent{Kind: SubscriptionSnapshotComplete, ConnectionSeq: e.Session().ConnectionSeq})
					if req.Snapshot {
						delete(e.keyed, reqID)
						sub.closeWithErr(nil)
					}
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				delete(e.keyed, reqID)
				sub.closeWithErr(e.apiErr(OpQuotes, m))
			},
			onDisconnect: func(e *Engine, err error) bool {
				if snapshotComplete {
					delete(e.keyed, reqID)
					sub.closeWithErr(nil)
					return false
				}
				if cfg.resume == ResumeAuto && !req.Snapshot {
					sub.emitState(SubscriptionStateEvent{Kind: SubscriptionGap, ConnectionSeq: e.Session().ConnectionSeq, Err: err})
					return true
				}
				delete(e.keyed, reqID)
				if req.Snapshot {
					sub.closeWithErr(ErrInterrupted)
					return false
				}
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			onReconnect: func(e *Engine) {
				if !req.Snapshot {
					sub.emitState(SubscriptionStateEvent{Kind: SubscriptionResumed, ConnectionSeq: e.Session().ConnectionSeq})
				}
			},
			close: func(err error) {
				sub.closeWithErr(err)
			},
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.Session().ConnectionSeq})
		if err := e.send(e.keyed[reqID].request); err != nil {
			delete(e.keyed, reqID)
			sub.closeWithErr(err)
			resp <- result{err: err}
			return
		}
		resp <- result{sub: sub}
	})

	select {
	case out := <-resp:
		if out.err == nil && out.sub != nil {
			bindContext(ctx, out.sub)
		}
		return out.sub, out.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (e *Engine) SubscribeRealTimeBars(ctx context.Context, req RealTimeBarsRequest, opts ...SubscriptionOption) (*Subscription[Bar], error) {
	type result struct {
		sub *Subscription[Bar]
		err error
	}
	resp := make(chan result, 1)

	e.enqueue(func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}

		cfg := defaultSubscriptionConfig(e.cfg)
		for _, opt := range opts {
			opt(&cfg)
		}
		reqID := e.allocReqID()
		var sub *Subscription[Bar]
		sub = newSubscription[Bar](cfg, func() {
			e.enqueue(func() {
				if _, ok := e.keyed[reqID]; !ok {
					return
				}
				delete(e.keyed, reqID)
				_ = e.send(codec.CancelRealTimeBars{ReqID: reqID})
				sub.closeWithErr(nil)
			})
		})

		e.keyed[reqID] = &keyedRoute{
			opKind:       OpRealTimeBars,
			subscription: true,
			resume:       cfg.resume,
			request: codec.RealTimeBarsRequest{
				ReqID:      reqID,
				Contract:   toCodecContract(req.Contract),
				WhatToShow: req.WhatToShow,
				UseRTH:     req.UseRTH,
			},
			cancel: codec.CancelRealTimeBars{ReqID: reqID},
			handle: func(msg any, e *Engine) {
				barMsg, ok := msg.(codec.RealTimeBar)
				if !ok {
					return
				}
				bar, err := fromCodecRealtimeBar(barMsg)
				if err != nil {
					delete(e.keyed, reqID)
					sub.closeWithErr(err)
					return
				}
				sub.emit(bar)
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				delete(e.keyed, reqID)
				sub.closeWithErr(e.apiErr(OpRealTimeBars, m))
			},
			onDisconnect: func(e *Engine, err error) bool {
				if cfg.resume == ResumeAuto {
					sub.emitState(SubscriptionStateEvent{Kind: SubscriptionGap, ConnectionSeq: e.Session().ConnectionSeq, Err: err})
					return true
				}
				delete(e.keyed, reqID)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			onReconnect: func(e *Engine) {
				sub.emitState(SubscriptionStateEvent{Kind: SubscriptionResumed, ConnectionSeq: e.Session().ConnectionSeq})
			},
			close: func(err error) { sub.closeWithErr(err) },
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.Session().ConnectionSeq})
		if err := e.send(e.keyed[reqID].request); err != nil {
			delete(e.keyed, reqID)
			sub.closeWithErr(err)
			resp <- result{err: err}
			return
		}
		resp <- result{sub: sub}
	})

	select {
	case out := <-resp:
		if out.err == nil && out.sub != nil {
			bindContext(ctx, out.sub)
		}
		return out.sub, out.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (e *Engine) OpenOrdersSnapshot(ctx context.Context, scope OpenOrdersScope) ([]OpenOrder, error) {
	sub, err := e.SubscribeOpenOrders(ctx, scope)
	if err != nil {
		return nil, err
	}
	defer sub.Close()
	return collectSnapshot(ctx, sub, func(update OpenOrderUpdate) OpenOrder { return update.Order })
}

func (e *Engine) SubscribeOpenOrders(ctx context.Context, scope OpenOrdersScope, opts ...SubscriptionOption) (*Subscription[OpenOrderUpdate], error) {
	type result struct {
		sub *Subscription[OpenOrderUpdate]
		err error
	}
	resp := make(chan result, 1)
	e.enqueue(func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		if _, exists := e.singletons["open_orders"]; exists {
			resp <- result{err: fmt.Errorf("ibkr: open orders subscription already active")}
			return
		}

		cfg := defaultSubscriptionConfig(e.cfg)
		for _, opt := range opts {
			opt(&cfg)
		}
		var sub *Subscription[OpenOrderUpdate]
		snapshotComplete := false
		sub = newSubscription[OpenOrderUpdate](cfg, func() {
			e.enqueue(func() {
				if _, ok := e.singletons["open_orders"]; !ok {
					return
				}
				delete(e.singletons, "open_orders")
				_ = e.send(codec.CancelOpenOrders{})
				sub.closeWithErr(nil)
			})
		})

		e.singletons["open_orders"] = &singletonRoute{
			opKind:       OpOpenOrders,
			subscription: true,
			resume:       cfg.resume,
			request:      codec.OpenOrdersRequest{Scope: string(scope)},
			cancel:       codec.CancelOpenOrders{},
			handle: func(msg any, e *Engine) {
				switch m := msg.(type) {
				case codec.OpenOrder:
					order, err := fromCodecOpenOrder(m)
					if err != nil {
						delete(e.singletons, "open_orders")
						sub.closeWithErr(err)
						return
					}
					sub.emit(OpenOrderUpdate{Order: order})
				case codec.OpenOrderEnd:
					snapshotComplete = true
					sub.emitState(SubscriptionStateEvent{Kind: SubscriptionSnapshotComplete, ConnectionSeq: e.Session().ConnectionSeq})
				}
			},
			onDisconnect: func(e *Engine, err error) bool {
				if snapshotComplete {
					delete(e.singletons, "open_orders")
					sub.closeWithErr(nil)
					return false
				}
				if cfg.resume == ResumeAuto {
					sub.emitState(SubscriptionStateEvent{Kind: SubscriptionGap, ConnectionSeq: e.Session().ConnectionSeq, Err: err})
					return true
				}
				delete(e.singletons, "open_orders")
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			onReconnect: func(e *Engine) {
				sub.emitState(SubscriptionStateEvent{Kind: SubscriptionResumed, ConnectionSeq: e.Session().ConnectionSeq})
			},
			close: func(err error) { sub.closeWithErr(err) },
		}

		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.Session().ConnectionSeq})
		if err := e.send(codec.OpenOrdersRequest{Scope: string(scope)}); err != nil {
			delete(e.singletons, "open_orders")
			sub.closeWithErr(err)
			resp <- result{err: err}
			return
		}
		resp <- result{sub: sub}
	})

	select {
	case out := <-resp:
		if out.err == nil && out.sub != nil {
			bindContext(ctx, out.sub)
		}
		return out.sub, out.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (e *Engine) Executions(ctx context.Context, req ExecutionsRequest) ([]ExecutionUpdate, error) {
	sub, err := e.SubscribeExecutions(ctx, req)
	if err != nil {
		return nil, err
	}
	defer sub.Close()
	return collectSnapshot(ctx, sub, func(update ExecutionUpdate) ExecutionUpdate { return update })
}

func (e *Engine) SubscribeExecutions(ctx context.Context, req ExecutionsRequest, opts ...SubscriptionOption) (*Subscription[ExecutionUpdate], error) {
	type result struct {
		sub *Subscription[ExecutionUpdate]
		err error
	}
	resp := make(chan result, 1)

	e.enqueue(func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}

		cfg := defaultSubscriptionConfig(e.cfg)
		for _, opt := range opts {
			opt(&cfg)
		}
		reqID := e.allocReqID()
		var sub *Subscription[ExecutionUpdate]
		snapshotComplete := false
		sub = newSubscription[ExecutionUpdate](cfg, func() {
			e.enqueue(func() {
				if _, ok := e.keyed[reqID]; !ok {
					return
				}
				delete(e.keyed, reqID)
				sub.closeWithErr(nil)
			})
		})

		e.keyed[reqID] = &keyedRoute{
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
						delete(e.keyed, reqID)
						sub.closeWithErr(err)
						return
					}
					sub.emit(update)
				case codec.ExecutionsEnd:
					snapshotComplete = true
					sub.emitState(SubscriptionStateEvent{Kind: SubscriptionSnapshotComplete, ConnectionSeq: e.Session().ConnectionSeq})
				case codec.CommissionReport:
					report, err := fromCodecCommission(m)
					if err != nil {
						delete(e.keyed, reqID)
						sub.closeWithErr(err)
						return
					}
					sub.emit(ExecutionUpdate{Commission: &report})
				}
			},
			handleAPIErr: func(m codec.APIError, e *Engine) {
				delete(e.keyed, reqID)
				sub.closeWithErr(e.apiErr(OpExecutions, m))
			},
			onDisconnect: func(e *Engine, err error) bool {
				if snapshotComplete {
					delete(e.keyed, reqID)
					sub.closeWithErr(nil)
					return false
				}
				if cfg.resume == ResumeAuto {
					sub.emitState(SubscriptionStateEvent{Kind: SubscriptionGap, ConnectionSeq: e.Session().ConnectionSeq, Err: err})
					return true
				}
				delete(e.keyed, reqID)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			onReconnect: func(e *Engine) {
				sub.emitState(SubscriptionStateEvent{Kind: SubscriptionResumed, ConnectionSeq: e.Session().ConnectionSeq})
			},
			close: func(err error) { sub.closeWithErr(err) },
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.Session().ConnectionSeq})
		if err := e.send(e.keyed[reqID].request); err != nil {
			delete(e.keyed, reqID)
			sub.closeWithErr(err)
			resp <- result{err: err}
			return
		}
		resp <- result{sub: sub}
	})

	select {
	case out := <-resp:
		if out.err == nil && out.sub != nil {
			bindContext(ctx, out.sub)
		}
		return out.sub, out.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
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

	e.transport = transport.New(conn, e.cfg.logger, e.cfg.sendRate)
	e.attachTransport(e.transport)
	e.setState(StateHandshaking, 0, "", nil)
	if err := e.send(codec.Hello{
		MinVersion: e.cfg.minServerVersion,
		MaxVersion: e.cfg.minServerVersion,
		ClientID:   1,
	}); err != nil {
		e.reportReady(err)
		e.closeEngine(err)
		return
	}
}

func (e *Engine) attachTransport(tr *transport.Conn) {
	decodedDone := make(chan struct{})
	go func() {
		defer close(decodedDone)
		for payload := range tr.Incoming() {
			msg, err := codec.Decode(payload)
			if err != nil {
				e.transportErr <- &ProtocolError{Direction: "inbound", Err: err}
				return
			}
			e.incoming <- msg
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
	case codec.HelloAck:
		if m.ServerVersion < e.cfg.minServerVersion {
			err := ErrUnsupportedServerVersion
			e.reportReady(err)
			e.closeEngine(err)
			return
		}
		e.updateSnapshot(func(s *Snapshot) {
			s.ServerVersion = m.ServerVersion
		})
		e.bootstrap.helloAck = true
		e.maybeReady()
		return
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
			return
		}
	}

	switch msg.(type) {
	case codec.Position, codec.PositionEnd:
		if route, ok := e.singletons["positions"]; ok {
			route.handle(msg, e)
		}
	case codec.OpenOrder, codec.OpenOrderEnd:
		if route, ok := e.singletons["open_orders"]; ok {
			route.handle(msg, e)
		}
	case codec.CommissionReport:
		for _, route := range e.keyed {
			if route.opKind == OpExecutions {
				route.handle(msg, e)
			}
		}
	}
}

func (e *Engine) handleAPIError(msg codec.APIError) {
	switch msg.Code {
	case 1100:
		e.setState(StateDegraded, msg.Code, msg.Message, nil)
		e.emitGap(msg.Message)
	case 1101:
		e.setState(StateReady, msg.Code, msg.Message, nil)
		e.emitGap(msg.Message)
	case 1102:
		e.setState(StateReady, msg.Code, msg.Message, nil)
		e.emitResumed(msg.Message)
	case 1300:
		if e.transport != nil {
			_ = e.transport.Close()
		}
	}

	if msg.ReqID > 0 {
		if route, ok := e.keyed[msg.ReqID]; ok && route.handleAPIErr != nil {
			route.handleAPIErr(msg, e)
			return
		}
	}
}

func (e *Engine) maybeReady() {
	if !e.bootstrap.helloAck || !e.bootstrap.managed || !e.bootstrap.nextValidID {
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
	if e.cfg.reconnect == ReconnectOff {
		if err == nil {
			err = ErrClosed
		}
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
		if route.onDisconnect == nil {
			route.close(ErrInterrupted)
			delete(e.keyed, reqID)
			continue
		}
		if !route.onDisconnect(e, err) {
			delete(e.keyed, reqID)
		}
	}
	for key, route := range e.singletons {
		if route.onDisconnect == nil {
			route.close(ErrInterrupted)
			delete(e.singletons, key)
			continue
		}
		if !route.onDisconnect(e, err) {
			delete(e.singletons, key)
		}
	}
}

func (e *Engine) resumeRoutes() {
	for _, route := range e.keyed {
		if route.subscription && route.resume == ResumeAuto {
			_ = e.send(route.request)
			if route.onReconnect != nil {
				route.onReconnect(e)
			}
		}
	}
	for _, route := range e.singletons {
		if route.subscription && route.resume == ResumeAuto {
			_ = e.send(route.request)
			if route.onReconnect != nil {
				route.onReconnect(e)
			}
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
		delete(e.keyed, reqID)
	}
	for key, route := range e.singletons {
		route.close(err)
		delete(e.singletons, key)
	}
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
	reqID := e.nextReqID
	e.nextReqID++
	return reqID
}

func (e *Engine) isReady() bool {
	return e.Session().State == StateReady || e.Session().State == StateDegraded
}

func (e *Engine) apiErr(opKind OpKind, msg codec.APIError) error {
	return &APIError{
		Code:          msg.Code,
		Message:       msg.Message,
		OpKind:        opKind,
		ConnectionSeq: e.Session().ConnectionSeq,
	}
}

func (e *Engine) emitGap(message string) {
	for _, route := range e.keyed {
		if route.subscription && route.resume == ResumeAuto && route.onReconnect != nil {
			route.onReconnect(e)
		}
	}
	_ = message
}

func (e *Engine) emitResumed(message string) {
	for _, route := range e.keyed {
		if route.subscription && route.resume == ResumeAuto && route.onReconnect != nil {
			route.onReconnect(e)
		}
	}
	_ = message
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
				return values, sub.Wait()
			}
			values = append(values, mapFn(item))
		case state, ok := <-sub.State():
			if !ok {
				return values, sub.Wait()
			}
			switch state.Kind {
			case SubscriptionSnapshotComplete:
				return drainSnapshotEvents(values, sub, mapFn), nil
			case SubscriptionClosed:
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
		Symbol:          c.Symbol,
		SecType:         c.SecType,
		Exchange:        c.Exchange,
		Currency:        c.Currency,
		PrimaryExchange: c.PrimaryExchange,
		LocalSymbol:     c.LocalSymbol,
	}
}

func fromCodecContract(c codec.Contract) Contract {
	return Contract{
		Symbol:          c.Symbol,
		SecType:         c.SecType,
		Exchange:        c.Exchange,
		Currency:        c.Currency,
		PrimaryExchange: c.PrimaryExchange,
		LocalSymbol:     c.LocalSymbol,
	}
}

func fromCodecContractDetails(m codec.ContractDetails) ContractDetails {
	minTick, _ := ParseDecimal(m.MinTick)
	return ContractDetails{
		Contract:   fromCodecContract(m.Contract),
		MarketName: m.MarketName,
		MinTick:    minTick,
		TimeZoneID: m.TimeZoneID,
	}
}

func fromCodecBar(m codec.HistoricalBar) (Bar, error) {
	ts, err := time.Parse(time.RFC3339, m.Time)
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
	return Bar{Time: ts, Open: open, High: high, Low: low, Close: closeValue, Volume: volume}, nil
}

func fromCodecRealtimeBar(m codec.RealTimeBar) (Bar, error) {
	return fromCodecBar(codec.HistoricalBar(m))
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

func fromCodecOpenOrder(m codec.OpenOrder) (OpenOrder, error) {
	quantity, err := ParseDecimal(m.Quantity)
	if err != nil {
		return OpenOrder{}, err
	}
	filled, err := ParseDecimal(m.Filled)
	if err != nil {
		return OpenOrder{}, err
	}
	remaining, err := ParseDecimal(m.Remaining)
	if err != nil {
		return OpenOrder{}, err
	}
	return OpenOrder{
		OrderID:   m.OrderID,
		Account:   m.Account,
		Contract:  fromCodecContract(m.Contract),
		Status:    m.Status,
		Quantity:  quantity,
		Filled:    filled,
		Remaining: remaining,
	}, nil
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

func applyTickPrice(quote *Quote, field string, raw string) (QuoteFields, error) {
	value, err := ParseDecimal(raw)
	if err != nil {
		return 0, err
	}
	switch field {
	case "bid":
		quote.Bid = value
		quote.Available |= QuoteFieldBid
		return QuoteFieldBid, nil
	case "ask":
		quote.Ask = value
		quote.Available |= QuoteFieldAsk
		return QuoteFieldAsk, nil
	case "last":
		quote.Last = value
		quote.Available |= QuoteFieldLast
		return QuoteFieldLast, nil
	case "open":
		quote.Open = value
		quote.Available |= QuoteFieldOpen
		return QuoteFieldOpen, nil
	case "high":
		quote.High = value
		quote.Available |= QuoteFieldHigh
		return QuoteFieldHigh, nil
	case "low":
		quote.Low = value
		quote.Available |= QuoteFieldLow
		return QuoteFieldLow, nil
	case "close":
		quote.Close = value
		quote.Available |= QuoteFieldClose
		return QuoteFieldClose, nil
	default:
		return 0, nil
	}
}

func applyTickSize(quote *Quote, field string, raw string) (QuoteFields, error) {
	value, err := ParseDecimal(raw)
	if err != nil {
		return 0, err
	}
	switch field {
	case "bid_size":
		quote.BidSize = value
		quote.Available |= QuoteFieldBidSize
		return QuoteFieldBidSize, nil
	case "ask_size":
		quote.AskSize = value
		quote.Available |= QuoteFieldAskSize
		return QuoteFieldAskSize, nil
	case "last_size":
		quote.LastSize = value
		quote.Available |= QuoteFieldLastSize
		return QuoteFieldLastSize, nil
	default:
		return 0, nil
	}
}

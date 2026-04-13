package ibkr

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/internal/transport"
	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
	"github.com/shopspring/decimal"
)

type engine struct {
	cfg config

	cmds         chan func()
	incoming     chan any
	transportErr chan error
	ready        chan error
	done         chan struct{}
	events       *observer[Event]

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

	nextReqID                int
	nextHistoricalRequest    time.Time
	recentHistoricalRequests map[string]time.Time

	bootstrap bootstrapState
	closed    bool

	reconnectAttempt int
}

type bootstrapState struct {
	serverInfo    bool
	managed       bool
	nextValidID   bool
	readyReported bool
}

const (
	minServerVersion = 100
	maxServerVersion = 200
	bootstrapTimeout = 5 * time.Second

	reconnectBackoff    = time.Second
	reconnectBackoffMax = 16 * time.Second

	historicalRequestSpacing   = 2 * time.Second
	historicalIdenticalSpacing = 15 * time.Second
)

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
	singletonCurrentTime       = "current_time"
)

type route struct {
	opKind       OpKind
	subscription bool
	resume       ResumePolicy
	request      codec.Message
	handle       func(any, *engine)
	handleAPIErr func(codec.APIError, *engine)
	onDisconnect func(*engine, error) bool
	emitGap      func(*engine)
	emitResumed  func(*engine)
	close        func(error)
	gapped       bool // true after Gap emitted, reset on Resumed; prevents double emission
}

type orderRoute struct {
	orderID          int64
	handle           *OrderHandle
	closed           bool
	gapped           bool // true after Gap emitted, reset on Resumed; prevents double emission
	terminalCloseSeq uint64
}

type parsedOpenOrder struct {
	order OpenOrder
}

func dialEngine(ctx context.Context, opts ...Option) (*engine, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.clientID < 0 {
		return nil, fmt.Errorf("ibkr: client id must be >= 0")
	}
	if cfg.eventBuffer < 1 {
		return nil, fmt.Errorf("ibkr: event buffer must be >= 1")
	}

	e := &engine{
		cfg:                      cfg,
		cmds:                     make(chan func(), 256),
		incoming:                 make(chan any, 256),
		transportErr:             make(chan error, 8),
		ready:                    make(chan error, 1),
		done:                     make(chan struct{}),
		events:                   newObserver[Event](cfg.eventBuffer),
		keyed:                    make(map[int]*route),
		singletons:               make(map[string]*route),
		orders:                   make(map[int64]*orderRoute),
		executions:               newExecutionCorrelator(),
		execToOrder:              make(map[string]int64),
		recentHistoricalRequests: make(map[string]time.Time),
		nextReqID:                1,
		snapshot: Snapshot{
			State: StateDisconnected,
		},
	}
	go e.run()
	e.enqueue(func() {
		e.startConnect(ctx, false)
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

func (e *engine) Close() error {
	e.enqueue(func() {
		e.closeEngine(ErrClosed)
	})
	return nil
}

func (e *engine) Done() <-chan struct{} {
	return e.done
}

func (e *engine) Wait() error {
	<-e.done
	e.waitMu.Lock()
	defer e.waitMu.Unlock()
	return e.waitErr
}

func (e *engine) Session() Snapshot {
	e.snapshotMu.RLock()
	defer e.snapshotMu.RUnlock()

	snap := e.snapshot
	snap.ManagedAccounts = append([]string(nil), snap.ManagedAccounts...)
	return snap
}

func (e *engine) SessionEvents() <-chan Event {
	return e.events.Chan()
}

func (e *engine) ContractDetails(ctx context.Context, contract Contract) ([]ContractDetails, error) {
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
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.ContractDetails:
					detail, err := fromCodecContractDetails(m)
					if err != nil {
						delete(e.keyed, reqID)
						resp <- result{err: err}
						return
					}
					values = append(values, detail)
				case codec.ContractDetailsEnd:
					delete(e.keyed, reqID)
					resp <- result{values: values}
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpContractDetails, m)}
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
		if err := e.sendContext(ctx, codec.ContractDetailsRequest{
			ReqID:    reqID,
			Contract: toCodecContract(contract),
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

func (e *engine) QualifyContract(ctx context.Context, contract Contract) (ContractDetails, error) {
	details, err := e.ContractDetails(ctx, contract)
	if err != nil {
		return ContractDetails{}, err
	}
	switch len(details) {
	case 0:
		return ContractDetails{}, ErrNoMatch
	case 1:
		return details[0], nil
	default:
		return ContractDetails{}, ErrAmbiguousContract
	}
}

func (e *engine) HistoricalBars(ctx context.Context, req HistoricalBarsRequest) ([]Bar, error) {
	if err := validateHistoricalBarsRequest(req); err != nil {
		return nil, err
	}

	type result struct {
		values []Bar
		err    error
	}

	resp := make(chan result, 1)
	var reqID int
	enqueueHistoricalSetup(ctx, e, historicalBarsPacingKey(req), nil, func() {
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
			handle: func(msg any, e *engine) {
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
			handleAPIErr: func(m codec.APIError, e *engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpHistoricalBars, m)}
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
		if err := e.sendContext(ctx, request); err != nil {
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

func (e *engine) HistoricalSchedule(ctx context.Context, req HistoricalScheduleRequest) (HistoricalSchedule, error) {
	if err := validateHistoricalScheduleRequest(req); err != nil {
		return HistoricalSchedule{}, err
	}

	type result struct {
		value HistoricalSchedule
		err   error
	}

	resp := make(chan result, 1)
	var reqID int
	enqueueHistoricalSetup(ctx, e, historicalSchedulePacingKey(req), nil, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}

		reqID = e.allocReqID()
		request, err := buildHistoricalScheduleRequest(reqID, req)
		if err != nil {
			resp <- result{err: err}
			return
		}
		e.keyed[reqID] = &route{
			opKind: OpHistoricalSchedule,
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.HistoricalScheduleResponse:
					delete(e.keyed, reqID)
					sessions := make([]HistoricalScheduleSession, len(m.Sessions))
					for i, s := range m.Sessions {
						sessions[i] = HistoricalScheduleSession{
							StartDateTime: s.StartDateTime,
							EndDateTime:   s.EndDateTime,
							RefDate:       s.RefDate,
						}
					}
					resp <- result{value: HistoricalSchedule{
						StartDateTime: m.StartDateTime,
						EndDateTime:   m.EndDateTime,
						TimeZone:      m.TimeZone,
						Sessions:      sessions,
					}}
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpHistoricalSchedule, m)}
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
		if err := e.sendContext(ctx, request); err != nil {
			delete(e.keyed, reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.deleteKeyedRoute(reqID) })
	})
	if err != nil {
		return HistoricalSchedule{}, err
	}
	return out.value, out.err
}

func (e *engine) AccountSummary(ctx context.Context, req AccountSummaryRequest) ([]AccountValue, error) {
	sub, err := e.SubscribeAccountSummary(ctx, req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = sub.Close() }()
	return collectSnapshot(ctx, sub, func(update AccountSummaryUpdate) AccountValue { return update.Value })
}

func (e *engine) SubscribeAccountSummary(ctx context.Context, req AccountSummaryRequest, opts ...SubscriptionOption) (*Subscription[AccountSummaryUpdate], error) {
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
	sub, err := e.SubscribePositions(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = sub.Close() }()
	return collectSnapshot(ctx, sub, func(update PositionUpdate) Position { return update.Position })
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
					sub.emit(PositionUpdate{Position: position})
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
				sub.emit(bar)
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
			handle: func(msg any, e *engine) {
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

func (e *engine) OpenOrdersSnapshot(ctx context.Context, scope OpenOrdersScope) ([]OpenOrder, error) {
	sub, err := e.SubscribeOpenOrders(ctx, scope)
	if err != nil {
		return nil, err
	}
	defer func() { _ = sub.Close() }()
	return collectSnapshot(ctx, sub, func(update OpenOrderUpdate) OpenOrder { return update.Order })
}

func (e *engine) SubscribeOpenOrders(ctx context.Context, scope OpenOrdersScope, opts ...SubscriptionOption) (*Subscription[OpenOrderUpdate], error) {
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
		sub.expectSnapshot()

		e.singletons[singletonOpenOrders] = &route{
			opKind:       OpOpenOrders,
			subscription: true,
			resume:       cfg.resume,
			request:      codec.OpenOrdersRequest{Scope: string(scope)},
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case parsedOpenOrder:
					sub.emit(OpenOrderUpdate{Order: m.order})
				case codec.OpenOrderEnd:
					sub.emitState(SubscriptionStateEvent{Kind: SubscriptionSnapshotComplete, ConnectionSeq: e.connectionSeq()})
				}
			},
			onDisconnect: func(e *engine, err error) bool {
				delete(e.singletons, singletonOpenOrders)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) { sub.closeWithErr(err) },
		}

		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.sendContext(ctx, codec.OpenOrdersRequest{Scope: string(scope)}); err != nil {
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

func (e *engine) Executions(ctx context.Context, req ExecutionsRequest) ([]ExecutionUpdate, error) {
	sub, err := e.subscribeExecutions(ctx, req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = sub.Close() }()
	return collectSnapshot(ctx, sub, func(update ExecutionUpdate) ExecutionUpdate { return update })
}

func (e *engine) subscribeExecutions(ctx context.Context, req ExecutionsRequest, opts ...SubscriptionOption) (*Subscription[ExecutionUpdate], error) {
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
		sub.expectSnapshot()
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
			handle: func(msg any, e *engine) {
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
			handleAPIErr: func(m codec.APIError, e *engine) {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpExecutions, m))
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

func (e *engine) CurrentTime(ctx context.Context) (time.Time, error) {
	type result struct {
		ts  time.Time
		err error
	}
	resp := make(chan result, 1)

	enqueueOneShotSetup(ctx, e, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		if _, exists := e.singletons[singletonCurrentTime]; exists {
			resp <- result{err: fmt.Errorf("ibkr: current time request already in progress")}
			return
		}

		// No handleAPIErr: req_current_time carries no reqID, so the engine
		// cannot route an APIError to this singleton. ctx cancellation and
		// onDisconnect are the only failure paths.
		e.singletons[singletonCurrentTime] = &route{
			opKind: OpCurrentTime,
			handle: func(msg any, eng *engine) {
				m, ok := msg.(codec.CurrentTime)
				if !ok {
					return
				}
				delete(eng.singletons, singletonCurrentTime)
				ts, parseErr := parseEpochSeconds(m.Time)
				if parseErr != nil {
					resp <- result{err: fmt.Errorf("ibkr: current time: %w", parseErr)}
					return
				}
				resp <- result{ts: ts}
			},
			onDisconnect: func(eng *engine, err error) bool {
				delete(eng.singletons, singletonCurrentTime)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.sendContext(ctx, codec.CurrentTimeRequest{}); err != nil {
			delete(e.singletons, singletonCurrentTime)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { delete(e.singletons, singletonCurrentTime) })
	})
	if err != nil {
		return time.Time{}, err
	}
	return out.ts, out.err
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

func (e *engine) ScannerParameters(ctx context.Context) (string, error) {
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
			handle: func(msg any, eng *engine) {
				switch m := msg.(type) {
				case codec.ScannerParameters:
					delete(eng.singletons, singletonScannerParameters)
					resp <- result{xml: m.XML}
				}
			},
			onDisconnect: func(eng *engine, err error) bool {
				delete(eng.singletons, singletonScannerParameters)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.sendContext(ctx, codec.ScannerParametersRequest{}); err != nil {
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

func (e *engine) UserInfo(ctx context.Context) (string, error) {
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
			handle: func(msg any, eng *engine) {
				switch m := msg.(type) {
				case codec.UserInfo:
					eng.deleteKeyedRoute(reqID)
					resp <- result{whiteBrandingID: m.WhiteBrandingID}
				}
			},
			handleAPIErr: func(m codec.APIError, eng *engine) {
				eng.deleteKeyedRoute(reqID)
				resp <- result{err: eng.apiErr(OpUserInfo, m)}
			},
			onDisconnect: func(eng *engine, err error) bool {
				eng.deleteKeyedRoute(reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.sendContext(ctx, codec.UserInfoRequest{ReqID: reqID}); err != nil {
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

func (e *engine) MatchingSymbols(ctx context.Context, pattern string) ([]MatchingSymbol, error) {
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
			handle: func(msg any, eng *engine) {
				switch m := msg.(type) {
				case codec.MatchingSymbols:
					eng.deleteKeyedRoute(reqID)
					symbols := make([]MatchingSymbol, len(m.Symbols))
					for i, s := range m.Symbols {
						derivTypes := make([]string, len(s.DerivativeSecTypes))
						copy(derivTypes, s.DerivativeSecTypes)
						symbols[i] = MatchingSymbol{
							ConID: s.ConID, Symbol: s.Symbol, SecType: SecType(s.SecType),
							PrimaryExchange: s.PrimaryExchange, Currency: s.Currency,
							DerivativeSecTypes: derivTypes,
							Description:        s.Description,
							IssuerID:           s.IssuerID,
						}
					}
					resp <- result{symbols: symbols}
				}
			},
			handleAPIErr: func(m codec.APIError, eng *engine) {
				eng.deleteKeyedRoute(reqID)
				resp <- result{err: eng.apiErr(OpMatchingSymbols, m)}
			},
			onDisconnect: func(eng *engine, err error) bool {
				eng.deleteKeyedRoute(reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.sendContext(ctx, codec.MatchingSymbolsRequest{ReqID: reqID, Pattern: pattern}); err != nil {
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

func (e *engine) HeadTimestamp(ctx context.Context, req HeadTimestampRequest) (time.Time, error) {
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
			handle: func(msg any, eng *engine) {
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
			handleAPIErr: func(m codec.APIError, eng *engine) {
				eng.deleteKeyedRoute(reqID)
				resp <- result{err: eng.apiErr(OpHeadTimestamp, m)}
			},
			onDisconnect: func(eng *engine, err error) bool {
				eng.deleteKeyedRoute(reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.sendContext(ctx, codec.HeadTimestampRequest{
			ReqID:      reqID,
			Contract:   toCodecContract(req.Contract),
			WhatToShow: string(req.WhatToShow),
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

func (e *engine) MarketRule(ctx context.Context, marketRuleID int) (MarketRuleResult, error) {
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
			handle: func(msg any, eng *engine) {
				switch m := msg.(type) {
				case codec.MarketRule:
					delete(eng.singletons, singletonMarketRule)
					increments := make([]PriceIncrement, len(m.Increments))
					for i, inc := range m.Increments {
						lowEdge, err := parseRequiredDecimal(inc.LowEdge, "market rule low edge")
						if err != nil {
							delete(eng.singletons, singletonMarketRule)
							resp <- result{err: err}
							return
						}
						increment, err := parseRequiredDecimal(inc.Increment, "market rule increment")
						if err != nil {
							delete(eng.singletons, singletonMarketRule)
							resp <- result{err: err}
							return
						}
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
			onDisconnect: func(eng *engine, err error) bool {
				delete(eng.singletons, singletonMarketRule)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.sendContext(ctx, codec.MarketRuleRequest{MarketRuleID: marketRuleID}); err != nil {
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

func (e *engine) CompletedOrders(ctx context.Context, apiOnly bool) ([]CompletedOrderResult, error) {
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
			handle: func(msg any, eng *engine) {
				switch m := msg.(type) {
				case codec.CompletedOrder:
					qty, err := parseRequiredDecimal(m.Quantity, "completed order quantity")
					if err != nil {
						delete(eng.singletons, singletonCompletedOrders)
						resp <- result{err: err}
						return
					}
					filled, err := parseOptionalDecimal(m.Filled, "completed order filled")
					if err != nil {
						delete(eng.singletons, singletonCompletedOrders)
						resp <- result{err: err}
						return
					}
					remaining, err := parseOptionalDecimal(m.Remaining, "completed order remaining")
					if err != nil {
						delete(eng.singletons, singletonCompletedOrders)
						resp <- result{err: err}
						return
					}
					collected = append(collected, CompletedOrderResult{
						Contract:  fromCodecContract(m.Contract),
						Action:    OrderAction(m.Action),
						OrderType: OrderType(m.OrderType),
						Status:    OrderStatus(m.Status),
						Quantity:  qty,
						Filled:    filled,
						Remaining: remaining,
					})
				case codec.CompletedOrderEnd:
					delete(eng.singletons, singletonCompletedOrders)
					resp <- result{orders: collected}
				}
			},
			onDisconnect: func(eng *engine, err error) bool {
				delete(eng.singletons, singletonCompletedOrders)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.sendContext(ctx, codec.CompletedOrdersRequest{APIOnly: apiOnly}); err != nil {
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
func (e *engine) AccountUpdatesSnapshot(ctx context.Context, account string) ([]AccountUpdate, error) {
	sub, err := e.SubscribeAccountUpdates(ctx, account)
	if err != nil {
		return nil, err
	}
	defer func() { _ = sub.Close() }()
	return collectSnapshot(ctx, sub, func(u AccountUpdate) AccountUpdate { return u })
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
		sub.expectSnapshot()

		e.singletons[singletonAccountUpdates] = &route{
			opKind:       OpAccountUpdates,
			subscription: true,
			resume:       cfg.resume,
			request:      codec.AccountUpdatesRequest{Subscribe: true, Account: account},
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.UpdateAccountValue:
					sub.emit(AccountUpdate{AccountValue: &AccountUpdateValue{
						Key: m.Key, Value: m.Value, Currency: m.Currency, Account: m.Account,
					}})
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
	sub, err := e.SubscribeAccountUpdatesMulti(ctx, req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = sub.Close() }()
	return collectSnapshot(ctx, sub, func(u AccountUpdateMultiValue) AccountUpdateMultiValue { return u })
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
		sub.expectSnapshot()

		e.keyed[reqID] = &route{
			opKind:       OpAccountUpdatesMulti,
			subscription: true,
			resume:       cfg.resume,
			request:      codec.AccountUpdatesMultiRequest{ReqID: reqID, Account: req.Account, ModelCode: req.ModelCode},
			handle: func(msg any, e *engine) {
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
	sub, err := e.SubscribePositionsMulti(ctx, req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = sub.Close() }()
	return collectSnapshot(ctx, sub, func(u PositionMulti) PositionMulti { return u })
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
					sub.emit(PositionMulti{
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
					sub.emit(PnLUpdate{DailyPnL: daily, UnrealizedPnL: unrealized, RealizedPnL: realized})
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
					sub.emit(PnLSingleUpdate{Position: pos, DailyPnL: daily, UnrealizedPnL: unrealized, RealizedPnL: realized, Value: value})
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
					sub.emit(tick)
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
			handle: func(msg any, e *engine) {
				if m, ok := msg.(codec.NewsBulletin); ok {
					sub.emit(NewsBulletin{MsgID: m.MsgID, MsgType: m.MsgType, Headline: m.Headline, Source: m.Source})
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

// SubscribeHistoricalBars sends a historical bars request with keepUpToDate=true,
// returning initial bars as events, SnapshotComplete on the initial batch end,
// then streaming bar updates via IN 108.
func (e *engine) SubscribeHistoricalBars(ctx context.Context, req HistoricalBarsRequest, opts ...SubscriptionOption) (*Subscription[Bar], error) {
	if err := validateHistoricalBarsStreamRequest(req); err != nil {
		return nil, err
	}

	type result struct {
		sub *Subscription[Bar]
		err error
	}
	resp := make(chan result, 1)

	enqueueHistoricalSetup(ctx, e, historicalBarsPacingKey(req), func() {
		resp <- result{}
	}, func() {
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
		sub.expectSnapshot()

		e.keyed[reqID] = &route{
			opKind:       OpHistoricalBarsStream,
			subscription: true,
			resume:       cfg.resume,
			request:      codecReq,
			handle: func(msg any, e *engine) {
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
			handleAPIErr: func(m codec.APIError, e *engine) {
				if m.Code == 10167 {
					e.emitEvent(m.Code, m.Message)
					return
				}
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpHistoricalBarsStream, m))
			},
			onDisconnect: func(e *engine, err error) bool {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) { sub.closeWithErr(err) },
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.sendContext(ctx, codecReq); err != nil {
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

func (e *engine) SecDefOptParams(ctx context.Context, req SecDefOptParamsRequest) ([]SecDefOptParams, error) {
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
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.SecDefOptParamsResponse:
					strikes := make([]decimal.Decimal, len(m.Strikes))
					for i, s := range m.Strikes {
						strike, err := parseRequiredDecimal(s, "sec def opt params strike")
						if err != nil {
							delete(e.keyed, reqID)
							resp <- result{err: err}
							return
						}
						strikes[i] = strike
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
			handleAPIErr: func(m codec.APIError, e *engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpSecDefOptParams, m)}
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
		if err := e.sendContext(ctx, codec.SecDefOptParamsRequest{
			ReqID:             reqID,
			UnderlyingSymbol:  req.UnderlyingSymbol,
			FutFopExchange:    req.FutFopExchange,
			UnderlyingSecType: string(req.UnderlyingSecType),
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

func (e *engine) SmartComponents(ctx context.Context, bboExchange string) ([]SmartComponent, error) {
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
			handle: func(msg any, e *engine) {
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
			handleAPIErr: func(m codec.APIError, e *engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpSmartComponents, m)}
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
		if err := e.sendContext(ctx, codec.SmartComponentsRequest{ReqID: reqID, BBOExchange: bboExchange}); err != nil {
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

func (e *engine) HistogramData(ctx context.Context, req HistogramDataRequest) ([]HistogramEntry, error) {
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
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.HistogramDataResponse:
					delete(e.keyed, reqID)
					entries := make([]HistogramEntry, len(m.Entries))
					for i, entry := range m.Entries {
						price, err := parseRequiredDecimal(entry.Price, "histogram price")
						if err != nil {
							e.deleteKeyedRoute(reqID)
							resp <- result{err: err}
							return
						}
						size, err := parseRequiredDecimal(entry.Size, "histogram size")
						if err != nil {
							e.deleteKeyedRoute(reqID)
							resp <- result{err: err}
							return
						}
						entries[i].Price = price
						entries[i].Size = size
					}
					resp <- result{entries: entries}
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpHistogramData, m)}
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
		if err := e.sendContext(ctx, codec.HistogramDataRequest{
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

func (e *engine) HistoricalTicks(ctx context.Context, req HistoricalTicksRequest) (HistoricalTicksResult, error) {
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
			handle: func(msg any, e *engine) {
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
						ticks[i].Price, err = parseRequiredDecimal(t.Price, "historical midpoint tick price")
						if err != nil {
							resp <- result{err: err}
							return
						}
						ticks[i].Size, err = parseRequiredDecimal(t.Size, "historical midpoint tick size")
						if err != nil {
							resp <- result{err: err}
							return
						}
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
						ticks[i].TickAttrib = t.TickAttrib
						ticks[i].Time = parsedTime
						ticks[i].BidPrice, err = parseRequiredDecimal(t.BidPrice, "historical bid price")
						if err != nil {
							resp <- result{err: err}
							return
						}
						ticks[i].AskPrice, err = parseRequiredDecimal(t.AskPrice, "historical ask price")
						if err != nil {
							resp <- result{err: err}
							return
						}
						ticks[i].BidSize, err = parseRequiredDecimal(t.BidSize, "historical bid size")
						if err != nil {
							resp <- result{err: err}
							return
						}
						ticks[i].AskSize, err = parseRequiredDecimal(t.AskSize, "historical ask size")
						if err != nil {
							resp <- result{err: err}
							return
						}
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
						ticks[i].TickAttrib = t.TickAttrib
						ticks[i].Time = parsedTime
						ticks[i].Price, err = parseRequiredDecimal(t.Price, "historical trade tick price")
						if err != nil {
							resp <- result{err: err}
							return
						}
						ticks[i].Size, err = parseRequiredDecimal(t.Size, "historical trade tick size")
						if err != nil {
							resp <- result{err: err}
							return
						}
						ticks[i].Exchange = t.Exchange
						ticks[i].SpecialConditions = t.SpecialConditions
					}
					resp <- result{value: HistoricalTicksResult{Last: ticks}}
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				e.deleteKeyedRoute(reqID)
				resp <- result{err: e.apiErr(OpHistoricalTicks, m)}
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
		if err := e.sendContext(ctx, codec.HistoricalTicksRequest{
			ReqID:         reqID,
			Contract:      toCodecContract(req.Contract),
			StartDateTime: formatHistoricalTickTime(req.StartTime),
			EndDateTime:   formatHistoricalTickTime(req.EndTime),
			NumberOfTicks: req.NumberOfTicks,
			WhatToShow:    string(req.WhatToShow),
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

func (e *engine) SubscribeScannerResults(ctx context.Context, req ScannerSubscriptionRequest, opts ...SubscriptionOption) (*Subscription[[]ScannerResult], error) {
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
				Instrument:   string(req.Instrument),
				LocationCode: string(req.LocationCode),
				ScanCode:     string(req.ScanCode),
			},
			handle: func(msg any, e *engine) {
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
			handleAPIErr: func(m codec.APIError, e *engine) {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpScannerSubscription, m))
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

// FA Configuration

func (e *engine) RequestFA(ctx context.Context, faDataType FADataType) (string, error) {
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
			handle: func(msg any, eng *engine) {
				switch m := msg.(type) {
				case codec.ReceiveFA:
					delete(eng.singletons, singletonFA)
					resp <- result{xml: m.XML}
				}
			},
			onDisconnect: func(eng *engine, err error) bool {
				delete(eng.singletons, singletonFA)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.sendContext(ctx, codec.RequestFA{FADataType: int(faDataType)}); err != nil {
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

func (e *engine) ReplaceFA(ctx context.Context, faDataType FADataType, xml string) error {
	return awaitFireAndForget(ctx, e, func(ctx context.Context) error {
		if !e.isReady() {
			return ErrNotReady
		}
		return e.sendContext(ctx, codec.ReplaceFA{FADataType: int(faDataType), XML: xml})
	})
}

func (e *engine) SoftDollarTiers(ctx context.Context) ([]SoftDollarTier, error) {
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
			handle: func(msg any, e *engine) {
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
			handleAPIErr: func(m codec.APIError, e *engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpSoftDollarTiers, m)}
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
		if err := e.sendContext(ctx, codec.SoftDollarTiersRequest{ReqID: reqID}); err != nil {
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

func (e *engine) WSHMetaData(ctx context.Context) (string, error) {
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
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.WSHMetaDataResponse:
					delete(e.keyed, reqID)
					resp <- result{dataJSON: m.DataJSON}
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpWSHMetaData, m)}
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
		if err := e.sendContext(ctx, codec.WSHMetaDataRequest{ReqID: reqID}); err != nil {
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

func (e *engine) WSHEventData(ctx context.Context, req WSHEventDataRequest) (string, error) {
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
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.WSHEventDataResponse:
					delete(e.keyed, reqID)
					resp <- result{dataJSON: m.DataJSON}
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpWSHEventData, m)}
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
		if err := e.sendContext(ctx, codec.WSHEventDataRequest{
			ReqID:           reqID,
			ConID:           req.ConID,
			Filter:          string(req.Filter),
			FillWatchlist:   req.FillWatchlist,
			FillPortfolio:   req.FillPortfolio,
			FillCompetitors: req.FillCompetitors,
			StartDate:       formatWSHDate(req.StartDate),
			EndDate:         formatWSHDate(req.EndDate),
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

func (e *engine) QueryDisplayGroups(ctx context.Context) (string, error) {
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
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.DisplayGroupList:
					delete(e.keyed, reqID)
					resp <- result{groups: m.Groups}
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpDisplayGroups, m)}
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
		if err := e.sendContext(ctx, codec.QueryDisplayGroupsRequest{ReqID: reqID}); err != nil {
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

func (e *engine) SubscribeDisplayGroup(ctx context.Context, groupID DisplayGroupID, opts ...SubscriptionOption) (*DisplayGroupHandle, error) {
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
			request:      codec.SubscribeToGroupEventsRequest{ReqID: reqID, GroupID: int(groupID)},
			handle: func(msg any, e *engine) {
				if m, ok := msg.(codec.DisplayGroupUpdated); ok {
					sub.emit(DisplayGroupUpdate{ContractInfo: m.ContractInfo})
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpDisplayGroupEvents, m))
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

func (e *engine) updateDisplayGroup(ctx context.Context, reqID int, contractInfo string) error {
	return awaitFireAndForget(ctx, e, func(ctx context.Context) error {
		if !e.isReady() {
			return ErrNotReady
		}
		return e.sendContext(ctx, codec.UpdateDisplayGroupRequest{ReqID: reqID, ContractInfo: contractInfo})
	})
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

// PlaceOrder submits a new order and returns an OrderHandle that tracks its
// lifecycle. The handle receives OpenOrder, OrderStatus, Execution, and
// Commission events via dual dispatch. The order can be modified or cancelled
// through the returned handle.
func (e *engine) PlaceOrder(ctx context.Context, req PlaceOrderRequest) (*OrderHandle, error) {
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
				ch <- e.sendContext(ctx, codec.CancelOrderRequest{OrderID: orderID})
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
			return awaitFireAndForget(ctx, e, func(ctx context.Context) error {
				if !e.isReady() {
					return ErrNotReady
				}
				return e.sendContext(ctx, toCodecPlaceOrder(orderID, PlaceOrderRequest{
					Contract: req.Contract,
					Order:    order,
				}))
			})
		}

		handle.detachFn = func() {
			e.enqueue(func() {
				if or, ok := e.orders[orderID]; ok && !or.closed {
					or.closed = true
				}
				handle.closeWithErr(nil)
			})
		}

		e.orders[orderID] = &orderRoute{orderID: orderID, handle: handle}

		if err := e.sendContext(ctx, toCodecPlaceOrder(orderID, req)); err != nil {
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
func (e *engine) CancelOrder(ctx context.Context, orderID int64) error {
	return awaitFireAndForget(ctx, e, func(ctx context.Context) error {
		if !e.isReady() {
			return ErrNotReady
		}
		return e.sendContext(ctx, codec.CancelOrderRequest{OrderID: orderID})
	})
}

// GlobalCancel requests cancellation of all open orders. This is
// fire-and-forget; individual cancellation results arrive via any active
// OrderHandle events channels.
func (e *engine) GlobalCancel(ctx context.Context) error {
	return awaitFireAndForget(ctx, e, func(ctx context.Context) error {
		if !e.isReady() {
			return ErrNotReady
		}
		return e.sendContext(ctx, codec.GlobalCancelRequest{})
	})
}

func (e *engine) FundamentalData(ctx context.Context, req FundamentalDataRequest) (string, error) {
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
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.FundamentalDataResponse:
					delete(e.keyed, reqID)
					resp <- result{data: m.Data}
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpFundamentalData, m)}
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
		if err := e.sendContext(ctx, codec.FundamentalDataRequest{
			ReqID:      reqID,
			Contract:   toCodecContract(req.Contract),
			ReportType: string(req.ReportType),
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

func (e *engine) ExerciseOptions(ctx context.Context, req ExerciseOptionsRequest) error {
	return awaitFireAndForget(ctx, e, func(ctx context.Context) error {
		if !e.isReady() {
			return ErrNotReady
		}
		override := 0
		if req.Override {
			override = 1
		}
		reqID := e.allocReqID()
		return e.sendContext(ctx, codec.ExerciseOptionsRequest{
			ReqID:            reqID,
			Contract:         toCodecContract(req.Contract),
			ExerciseAction:   int(req.ExerciseAction),
			ExerciseQuantity: req.ExerciseQuantity,
			Account:          req.Account,
			Override:         override,
		})
	})
}

func (e *engine) run() {
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
		case err := <-e.transportErr:
			if len(e.incoming) > 0 {
				go func(err error) {
					e.transportErr <- err
				}(err)
				continue
			}
			e.handleTransportLoss(err)
			continue
		default:
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

func (e *engine) enqueue(fn func()) {
	select {
	case <-e.done:
		return
	case e.cmds <- fn:
	}
}

func (e *engine) startConnect(ctx context.Context, reconnect bool) {
	if e.closed {
		return
	}
	e.bootstrap = bootstrapState{}
	if reconnect {
		e.setState(StateReconnecting, 0, "reconnect attempt", nil)
	} else {
		e.setState(StateConnecting, 0, "", nil)
	}

	conn, err := e.cfg.dialer.DialContext(ctx, "tcp", net.JoinHostPort(e.cfg.host, strconv.Itoa(e.cfg.port)))
	if err != nil {
		e.connectFailed("dial", err, reconnect)
		return
	}
	if err := configureTCPKeepAlive(conn, e.cfg.tcpKeepAlive); err != nil {
		_ = conn.Close()
		e.connectFailed("keepalive", err, reconnect)
		return
	}

	// Synchronous handshake before starting transport goroutines.
	deadline := time.Now().Add(10 * time.Second)

	// 1. Send API prefix (raw bytes, not framed)
	if err := transport.WriteRaw(conn, codec.EncodeHandshakePrefix()); err != nil {
		conn.Close()
		e.connectFailed("handshake", err, reconnect)
		return
	}

	// 2. Send version range (framed)
	if err := wire.WriteFrame(conn, codec.EncodeVersionRange(minServerVersion, maxServerVersion)); err != nil {
		conn.Close()
		e.connectFailed("handshake", err, reconnect)
		return
	}

	// 3. Read server info (framed, but no msg_id prefix)
	serverPayload, err := transport.ReadOneFrame(conn, deadline)
	if err != nil {
		conn.Close()
		e.connectFailed("handshake", err, reconnect)
		return
	}
	info, err := codec.DecodeServerInfo(serverPayload)
	if err != nil {
		conn.Close()
		e.connectFailed("handshake", err, reconnect)
		return
	}

	// 4. Version check
	if info.ServerVersion < minServerVersion {
		// A server-version mismatch is a protocol capability failure, not a
		// transient reconnect failure. Terminate even during reconnect.
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
		e.connectFailed("handshake", err, reconnect)
		return
	}
	if err := wire.WriteFrame(conn, startPayload); err != nil {
		conn.Close()
		e.connectFailed("handshake", err, reconnect)
		return
	}

	// 6. Start async transport — ManagedAccounts + NextValidID arrive on incoming channel
	e.transport = transport.New(conn, e.cfg.logger, e.cfg.sendRate)
	e.attachTransport(e.transport)
	e.scheduleBootstrapTimeout(e.transport)
	e.setState(StateHandshaking, 0, "", nil)
}

func (e *engine) connectFailed(op string, err error, reconnect bool) {
	connectErr := &ConnectError{Op: op, Err: err}
	if !reconnect {
		e.reportReady(connectErr)
		e.closeEngine(connectErr)
		return
	}
	e.setState(StateReconnecting, 0, "reconnect failed", connectErr)
	e.scheduleReconnect()
}

func configureTCPKeepAlive(conn net.Conn, period time.Duration) error {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return nil
	}
	if period <= 0 {
		return tcpConn.SetKeepAlive(false)
	}
	if err := tcpConn.SetKeepAlive(true); err != nil {
		return err
	}
	return tcpConn.SetKeepAlivePeriod(period)
}

func (e *engine) attachTransport(tr *transport.Conn) {
	decodedDone := make(chan struct{})
	go func() {
		defer close(decodedDone)
		for payload := range tr.Incoming() {
			msgs, err := codec.DecodeBatch(payload)
			if err != nil {
				_ = tr.Close()
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

func (e *engine) scheduleBootstrapTimeout(tr *transport.Conn) {
	time.AfterFunc(bootstrapTimeout, func() {
		e.enqueue(func() {
			if e.closed || e.transport != tr {
				return
			}
			e.snapshotMu.RLock()
			state := e.snapshot.State
			e.snapshotMu.RUnlock()
			if state != StateHandshaking {
				return
			}
			_ = tr.Close()
		})
	})
}

func (e *engine) handleIncoming(msg any) {
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
		// CurrentTime responses arrive without a reqID. Parse the server's
		// epoch-seconds string, update the session snapshot, and route to a
		// registered singleton one-shot if one exists. The route handler
		// re-parses via the same helper so the snapshot and the caller
		// response cannot disagree.
		if ts, err := parseEpochSeconds(m.Time); err == nil {
			e.updateSnapshot(func(s *Snapshot) {
				s.CurrentTime = ts
			})
		}
		if route, ok := e.singletons[singletonCurrentTime]; ok {
			route.handle(m, e)
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
		e.dispatchObservedOpenOrder(msg)
	case codec.OrderStatus:
		e.dispatchObservedOrderStatus(msg)
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

func (e *engine) handleAPIError(msg codec.APIError) {
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
	// Route to keyed handler when one exists (the handler decides whether
	// the code is terminal); otherwise emit as a session-level event.
	if msg.Code >= 10000 && msg.Code < 20000 {
		if msg.ReqID > 0 {
			if route, ok := e.keyed[msg.ReqID]; ok && route.handleAPIErr != nil {
				route.handleAPIErr(msg, e)
				return
			}
		}
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

func (e *engine) maybeReady() {
	if !e.bootstrap.serverInfo || !e.bootstrap.managed || !e.bootstrap.nextValidID {
		return
	}
	// Completed bootstrap is the success boundary for reconnect backoff.
	e.reconnectAttempt = 0
	e.updateSnapshot(func(s *Snapshot) {
		s.ConnectionSeq++
	})
	e.setState(StateReady, 0, "", nil)
	e.reportReady(nil)
	e.resumeRoutes()
}

func (e *engine) handleTransportLoss(err error) {
	if e.closed {
		return
	}
	if e.transport == nil {
		return
	}
	err = normalizeTransportErr(err)
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
	e.scheduleReconnect()
}

func (e *engine) scheduleReconnect() {
	delay := reconnectDelay(e.reconnectAttempt)
	e.reconnectAttempt++
	time.AfterFunc(delay, func() {
		e.enqueue(func() {
			if e.closed || e.transport != nil || e.cfg.reconnect == ReconnectOff {
				return
			}
			dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			e.startConnect(dialCtx, true)
		})
	})
}

func reconnectDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	delay := reconnectBackoff
	for i := 0; i < attempt; i++ {
		delay *= 2
		if delay >= reconnectBackoffMax {
			return reconnectBackoffMax
		}
	}
	return delay
}

func (e *engine) disconnectRoutes(err error) {
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

func (e *engine) resumeRoutes() {
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

func (e *engine) closeEngine(err error) {
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
			orderErr := err
			if or.terminalCloseSeq > 0 {
				orderErr = nil
			}
			or.handle.closeWithErr(orderErr)
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
	e.events.Close()
}

func (e *engine) reportReady(err error) {
	if e.bootstrap.readyReported {
		return
	}
	e.bootstrap.readyReported = true
	select {
	case e.ready <- err:
	default:
	}
}

func (e *engine) setState(next State, code int, message string, err error) {
	e.snapshotMu.Lock()
	prev := e.snapshot.State
	e.snapshot.State = next
	connSeq := e.snapshot.ConnectionSeq
	e.snapshotMu.Unlock()

	e.events.EmitLatest(Event{
		At:            time.Now().UTC(),
		State:         next,
		Previous:      prev,
		ConnectionSeq: connSeq,
		Code:          code,
		Message:       message,
		Err:           err,
	})
}

// emitEvent publishes an informational session event (e.g. farm-status
// or market-data warnings) without changing session state.
func (e *engine) emitEvent(code int, message string) {
	e.snapshotMu.RLock()
	state := e.snapshot.State
	connSeq := e.snapshot.ConnectionSeq
	e.snapshotMu.RUnlock()
	e.events.EmitLatest(Event{
		At:            time.Now().UTC(),
		State:         state,
		Previous:      state,
		ConnectionSeq: connSeq,
		Code:          code,
		Message:       message,
	})
}

func (e *engine) updateSnapshot(update func(*Snapshot)) {
	e.snapshotMu.Lock()
	defer e.snapshotMu.Unlock()
	update(&e.snapshot)
}

func (e *engine) send(msg codec.Message) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return e.sendContext(ctx, msg)
}

func (e *engine) sendContext(ctx context.Context, msg codec.Message) error {
	if e.transport == nil {
		return ErrNotReady
	}
	payload, err := codec.Encode(msg)
	if err != nil {
		return err
	}
	err = e.transport.Send(ctx, payload)
	if errors.Is(err, transport.ErrSendQueueFull) {
		return ErrInterrupted
	}
	return err
}

func normalizeTransportErr(err error) error {
	if errors.Is(err, transport.ErrSendQueueFull) {
		return ErrInterrupted
	}
	return err
}

func (e *engine) allocReqID() int {
	for {
		id := e.nextReqID
		e.nextReqID++
		if _, conflict := e.orders[int64(id)]; !conflict {
			return id
		}
	}
}

func (e *engine) allocOrderID() int64 {
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

func (e *engine) connectionSeq() uint64 {
	e.snapshotMu.RLock()
	defer e.snapshotMu.RUnlock()
	return e.snapshot.ConnectionSeq
}

func (e *engine) isReady() bool {
	if e.transport == nil {
		return false
	}
	e.snapshotMu.RLock()
	state := e.snapshot.State
	e.snapshotMu.RUnlock()
	return state == StateReady || state == StateDegraded
}

func (e *engine) apiErr(opKind OpKind, msg codec.APIError) error {
	return &APIError{
		Code:          msg.Code,
		Message:       msg.Message,
		OpKind:        opKind,
		ConnectionSeq: e.connectionSeq(),
	}
}

func (e *engine) emitGap() {
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

func (e *engine) emitResumed() {
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

func (e *engine) dispatchObservedOpenOrder(msg codec.OpenOrder) {
	orderRoute, orderObserved := e.orders[msg.OrderID]
	singletonRoute, singletonObserved := e.singletons[singletonOpenOrders]
	if (!orderObserved || orderRoute.closed) && !singletonObserved {
		return
	}

	order, err := fromCodecOpenOrder(msg)
	if err != nil {
		if orderObserved && !orderRoute.closed {
			orderRoute.closed = true
			orderRoute.handle.emitOrderError(err)
		}
		if singletonObserved {
			delete(e.singletons, singletonOpenOrders)
			singletonRoute.close(err)
		}
		return
	}

	if orderObserved && !orderRoute.closed {
		if !orderRoute.handle.emitOrder(order) {
			orderRoute.closed = true
		}
	}
	if singletonObserved {
		singletonRoute.handle(parsedOpenOrder{order: order}, e)
	}
}

func (e *engine) dispatchObservedOrderStatus(msg codec.OrderStatus) {
	orderRoute, ok := e.orders[msg.OrderID]
	if !ok || orderRoute.closed {
		return
	}

	status, err := fromCodecOrderStatus(msg)
	if err != nil {
		orderRoute.closed = true
		orderRoute.handle.emitOrderError(err)
		return
	}

	if !orderRoute.handle.emitStatus(status) {
		orderRoute.closed = true
		return
	}
	if IsTerminalOrderStatus(status.Status) {
		e.scheduleTerminalOrderClose(msg.OrderID, orderRoute)
	}
}

const orderTerminalDrainWindow = 750 * time.Millisecond

func (e *engine) scheduleTerminalOrderClose(orderID int64, route *orderRoute) {
	route.terminalCloseSeq++
	seq := route.terminalCloseSeq
	time.AfterFunc(orderTerminalDrainWindow, func() {
		e.enqueue(func() {
			current, ok := e.orders[orderID]
			if !ok || current.closed || current.terminalCloseSeq != seq {
				return
			}
			current.closed = true
			current.handle.closeWithErr(nil)
		})
	})
}

func (e *engine) activeAccountSummarySubscriptions() int {
	count := 0
	for _, route := range e.keyed {
		if route.subscription && route.opKind == OpAccountSummary {
			count++
		}
	}
	return count
}

func (e *engine) deleteKeyedRoute(reqID int) {
	route, ok := e.keyed[reqID]
	if !ok {
		return
	}
	delete(e.keyed, reqID)
	if route.opKind == OpExecutions {
		e.executions.unregisterRoute(reqID)
	}
}

func (e *engine) routeCommissionReport(report codec.CommissionReport) {
	for _, reqID := range e.executions.recordCommission(report) {
		route, found := e.keyed[reqID]
		if !found || route.opKind != OpExecutions {
			continue
		}
		route.handle(report, e)
	}
	// Also dispatch to the order handle that owns this execution. The order
	// is live on the server, so a decode failure must not tear down the
	// handle — drop the event and log so the problem is observable.
	if orderID, ok := e.execToOrder[report.ExecID]; ok {
		if or, ok := e.orders[orderID]; ok && !or.closed {
			cr, err := fromCodecCommission(report)
			if err != nil {
				e.cfg.logger.Warn("ibkr: drop commission report on decode error",
					"order_id", orderID, "exec_id", report.ExecID, "err", err)
				return
			}
			if !or.handle.emitCommission(cr) {
				or.closed = true
			}
		}
	}
}

func (e *engine) dispatchExecutionToOrder(m codec.ExecutionDetail) {
	if m.OrderID == 0 {
		return
	}
	or, ok := e.orders[m.OrderID]
	if !ok || or.closed {
		return
	}
	// Per-order dispatch: the order is live on the server, so a decode
	// failure must not tear down the handle — drop the event and log so the
	// problem is observable.
	exec, err := fromCodecExecution(m)
	if err != nil {
		e.cfg.logger.Warn("ibkr: drop execution detail on decode error",
			"order_id", m.OrderID, "exec_id", m.ExecID, "err", err)
		return
	}
	if !or.handle.emitExecution(*exec.Execution) {
		or.closed = true
		return
	}
	e.execToOrder[m.ExecID] = m.OrderID
}

func (e *engine) undeliveredCommissions(reqID int, execID string) []codec.CommissionReport {
	return e.executions.undeliveredCommissions(reqID, execID)
}

func (e *engine) emitUndeliveredExecutionCommissions(reqID int, execID string, sub *Subscription[ExecutionUpdate]) bool {
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
	case codec.HistoricalScheduleResponse:
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

func toCodecContract(c Contract) codec.Contract {
	return codec.Contract{
		ConID:           c.ConID,
		Symbol:          c.Symbol,
		SecType:         string(c.SecType),
		Expiry:          c.Expiry,
		Strike:          c.Strike,
		Right:           string(c.Right),
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
		SecType:         SecType(c.SecType),
		Expiry:          c.Expiry,
		Strike:          c.Strike,
		Right:           Right(c.Right),
		Multiplier:      c.Multiplier,
		Exchange:        c.Exchange,
		Currency:        c.Currency,
		LocalSymbol:     c.LocalSymbol,
		TradingClass:    c.TradingClass,
		PrimaryExchange: c.PrimaryExchange,
	}
}

func fromCodecContractDetails(m codec.ContractDetails) (ContractDetails, error) {
	minTick, err := parseOptionalDecimal(m.MinTick, "contract details min tick")
	if err != nil {
		return ContractDetails{}, err
	}
	return ContractDetails{
		Contract:   fromCodecContract(m.Contract),
		MarketName: m.MarketName,
		LongName:   m.LongName,
		MinTick:    minTick,
		TimeZoneID: m.TimeZoneID,
	}, nil
}

func fromCodecBar(m codec.HistoricalBar) (Bar, error) {
	ts, err := parseBarTime(m.Time)
	if err != nil {
		return Bar{}, err
	}
	open, err := parseRequiredDecimal(m.Open, "bar open")
	if err != nil {
		return Bar{}, err
	}
	high, err := parseRequiredDecimal(m.High, "bar high")
	if err != nil {
		return Bar{}, err
	}
	low, err := parseRequiredDecimal(m.Low, "bar low")
	if err != nil {
		return Bar{}, err
	}
	closeValue, err := parseRequiredDecimal(m.Close, "bar close")
	if err != nil {
		return Bar{}, err
	}
	volume, err := parseRequiredDecimal(m.Volume, "bar volume")
	if err != nil {
		return Bar{}, err
	}
	wap, err := parseOptionalDecimal(m.WAP, "bar wap")
	if err != nil {
		return Bar{}, err
	}
	count, err := parseOptionalInt(m.Count, "bar count")
	if err != nil {
		return Bar{}, err
	}
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

func parseHeadTimestamp(raw string) (time.Time, error) {
	ts, err := time.Parse("20060102-15:04:05", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("ibkr: parse head timestamp %q", raw)
	}
	return ts.UTC(), nil
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

func fromCodecOpenOrder(m codec.OpenOrder) (OpenOrder, error) {
	quantity, err := parseOptionalDecimal(m.Quantity, "open order quantity")
	if err != nil {
		return OpenOrder{}, err
	}
	filled, err := parseOptionalDecimal(m.Filled, "open order filled")
	if err != nil {
		return OpenOrder{}, err
	}
	remaining, err := parseOptionalDecimal(m.Remaining, "open order remaining")
	if err != nil {
		return OpenOrder{}, err
	}
	lmtPrice, err := parseOptionalDecimal(m.LmtPrice, "open order limit price")
	if err != nil {
		return OpenOrder{}, err
	}
	auxPrice, err := parseOptionalDecimal(m.AuxPrice, "open order aux price")
	if err != nil {
		return OpenOrder{}, err
	}
	commission, err := parseOptionalDecimal(m.Commission, "open order commission")
	if err != nil {
		return OpenOrder{}, err
	}
	minCommission, err := parseOptionalDecimal(m.MinCommission, "open order min commission")
	if err != nil {
		return OpenOrder{}, err
	}
	maxCommission, err := parseOptionalDecimal(m.MaxCommission, "open order max commission")
	if err != nil {
		return OpenOrder{}, err
	}
	origin, err := parseOptionalInt(m.Origin, "open order origin")
	if err != nil {
		return OpenOrder{}, err
	}
	clientID, err := parseOptionalInt(m.ClientID, "open order client id")
	if err != nil {
		return OpenOrder{}, err
	}
	permID, err := parseOptionalInt64(m.PermID, "open order perm id")
	if err != nil {
		return OpenOrder{}, err
	}
	parentID, err := parseOptionalInt64(m.ParentID, "open order parent id")
	if err != nil {
		return OpenOrder{}, err
	}
	outsideRTH, err := parseOptionalBoolString(m.OutsideRTH, "open order outside rth")
	if err != nil {
		return OpenOrder{}, err
	}
	hidden, err := parseOptionalBoolString(m.Hidden, "open order hidden")
	if err != nil {
		return OpenOrder{}, err
	}
	return OpenOrder{
		OrderID:               m.OrderID,
		Account:               m.Account,
		Contract:              fromCodecContract(m.Contract),
		Action:                OrderAction(m.Action),
		OrderType:             OrderType(m.OrderType),
		Status:                OrderStatus(m.Status),
		Quantity:              quantity,
		Filled:                filled,
		Remaining:             remaining,
		LmtPrice:              lmtPrice,
		AuxPrice:              auxPrice,
		TIF:                   TimeInForce(m.TIF),
		OcaGroup:              m.OcaGroup,
		OpenClose:             m.OpenClose,
		Origin:                origin,
		OrderRef:              m.OrderRef,
		ClientID:              clientID,
		PermID:                permID,
		OutsideRTH:            outsideRTH,
		Hidden:                hidden,
		GoodAfterTime:         m.GoodAfterTime,
		ParentID:              parentID,
		ComboLegs:             comboLegsFromCodec(m.ComboLegs),
		OrderComboLegPrices:   append([]string(nil), m.OrderComboLegPrices...),
		SmartComboRouting:     tagValuesFromCodec(m.SmartComboRouting),
		AlgoStrategy:          m.AlgoStrategy,
		AlgoParams:            tagValuesFromCodec(m.AlgoParams),
		Conditions:            orderConditionsFromCodec(m.Conditions),
		ConditionsIgnoreRTH:   m.ConditionsIgnoreRTH == "1",
		ConditionsCancelOrder: m.ConditionsCancelOrder == "1",
		Commission:            commission,
		MinCommission:         minCommission,
		MaxCommission:         maxCommission,
		CommissionCurrency:    m.CommissionCurrency,
	}, nil
}

func fromCodecOrderStatus(m codec.OrderStatus) (OrderStatusUpdate, error) {
	filled, err := parseOptionalDecimal(m.Filled, "order status filled")
	if err != nil {
		return OrderStatusUpdate{}, err
	}
	remaining, err := parseOptionalDecimal(m.Remaining, "order status remaining")
	if err != nil {
		return OrderStatusUpdate{}, err
	}
	avgFillPrice, err := parseOptionalDecimal(m.AvgFillPrice, "order status average fill price")
	if err != nil {
		return OrderStatusUpdate{}, err
	}
	lastFillPrice, err := parseOptionalDecimal(m.LastFillPrice, "order status last fill price")
	if err != nil {
		return OrderStatusUpdate{}, err
	}
	mktCapPrice, err := parseOptionalDecimal(m.MktCapPrice, "order status market cap price")
	if err != nil {
		return OrderStatusUpdate{}, err
	}
	permID, err := parseOptionalInt64(m.PermID, "order status perm id")
	if err != nil {
		return OrderStatusUpdate{}, err
	}
	parentID, err := parseOptionalInt64(m.ParentID, "order status parent id")
	if err != nil {
		return OrderStatusUpdate{}, err
	}
	clientID, err := parseOptionalInt(m.ClientID, "order status client id")
	if err != nil {
		return OrderStatusUpdate{}, err
	}
	return OrderStatusUpdate{
		OrderID:       m.OrderID,
		Status:        OrderStatus(m.Status),
		Filled:        filled,
		Remaining:     remaining,
		AvgFillPrice:  avgFillPrice,
		PermID:        permID,
		ParentID:      parentID,
		LastFillPrice: lastFillPrice,
		ClientID:      clientID,
		WhyHeld:       m.WhyHeld,
		MktCapPrice:   mktCapPrice,
	}, nil
}

func fromCodecExecution(m codec.ExecutionDetail) (ExecutionUpdate, error) {
	shares, err := parseRequiredDecimal(m.Shares, "execution shares")
	if err != nil {
		return ExecutionUpdate{}, err
	}
	price, err := parseRequiredDecimal(m.Price, "execution price")
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
	// Commission and RealizedPNL are parsed as optional so that the Java
	// reference encoding of "unset" — either an empty string or the literal
	// Double.MAX_VALUE sentinel — decodes to a zero decimal instead of an error.
	// RealizedPNL in particular arrives unset for trades whose position is not
	// yet closed.
	commission, err := parseOptionalDecimal(m.Commission, "commission amount")
	if err != nil {
		return CommissionReport{}, err
	}
	realized, err := parseOptionalDecimal(m.RealizedPNL, "commission realized pnl")
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

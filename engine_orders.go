package ibkr

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
	"github.com/shopspring/decimal"
)

// RefreshOrderID asks the Gateway for a fresh next-valid order ID and updates
// the engine's allocation seed before returning it.
func (e *engine) RefreshOrderID(ctx context.Context) (int64, error) {
	type result struct {
		orderID int64
		err     error
	}
	resp := make(chan result, 1)
	var ownedRoute *route
	enqueueOneShotSetup(ctx, e, func() {
		if _, exists := e.singletons[singletonOrderID]; exists {
			resp <- result{err: operationActive("order ID refresh")}
			return
		}
		ownedRoute = &route{
			opKind: OpOrderID,
			handle: func(msg any, eng *engine) {
				m, ok := msg.(codec.NextValidID)
				if !ok {
					return
				}
				delete(eng.singletons, singletonOrderID)
				if m.OrderID <= 0 {
					resp <- result{err: fmt.Errorf("ibkr: invalid next valid order ID %d", m.OrderID)}
					return
				}
				resp <- result{orderID: eng.snapshot.NextValidID}
			},
			handleAPIErr: func(msg codec.APIError, eng *engine) {
				delete(eng.singletons, singletonOrderID)
				resp <- result{err: eng.apiErr(OpOrderID, msg)}
			},
			onDisconnect: func(eng *engine, err error) bool {
				resp <- result{err: interrupted(err)}
				return false
			},
			close: func(err error) { resp <- result{err: err} },
		}
		e.singletons[singletonOrderID] = ownedRoute
		if err := e.sendContext(ctx, codec.ReqIDsRequest{NumIDs: 1}); err != nil {
			delete(e.singletons, singletonOrderID)
			resp <- result{err: err}
		}
	})
	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.abortUnresolvedSingletonOneShot(singletonOrderID, ownedRoute) })
	})
	if err != nil {
		return 0, err
	}
	return out.orderID, out.err
}

func (e *engine) OpenOrdersSnapshot(ctx context.Context, scope OpenOrdersScope) ([]OpenOrder, error) {
	if scope == OpenOrdersScopeAuto {
		return nil, fmt.Errorf("%w: auto-scope open orders", ErrNoSnapshot)
	}
	sub, err := e.SubscribeOpenOrders(ctx, scope, withSnapshotCollector())
	if err != nil {
		return nil, err
	}
	defer sub.Close()
	return collectSnapshot(ctx, sub.Subscription, func(update OpenOrderUpdate) (OpenOrder, bool) {
		if update.Order == nil {
			return OpenOrder{}, false
		}
		return *update.Order, true
	})
}

func (e *engine) SubscribeOpenOrders(ctx context.Context, scope OpenOrdersScope, opts ...SubscriptionOption) (*OpenOrdersSubscription, error) {
	type result struct {
		sub *OpenOrdersSubscription
		err error
	}
	resp := make(chan result, 1)
	enqueueSingletonSubscriptionSetup(ctx, e, singletonOpenOrders, resp, func() {
		if err := validateOpenOrdersScope(scope, e.cfg.clientID); err != nil {
			resp <- result{err: err}
			return
		}
		if _, exists := e.singletons[singletonOpenOrders]; exists {
			resp <- result{err: operationActive("open orders subscription")}
			return
		}

		cfg, err := applySubscriptionOptionsFor(e.cfg, OpOpenOrders, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		var cancel codec.Message
		if scope == OpenOrdersScopeAuto {
			cancel = codec.CancelOpenOrders{}
		}
		sub, ownedRoute := newSingletonSubscriptionRoute[OpenOrderUpdate](
			e, cfg, singletonOpenOrders, OpOpenOrders, cancel,
		)
		handle := &OpenOrdersSubscription{Subscription: sub}
		refreshPending := scope != OpenOrdersScopeAuto
		// Auto scope binds future manual orders and emits no open_order_end, so
		// it is a stream with no initial snapshot phase.
		if scope != OpenOrdersScopeAuto {
			sub.expectSnapshot()
		}

		ownedRoute.request = codec.OpenOrdersRequest{Scope: string(scope)}
		ownedRoute.handle = func(msg any, e *engine) {
			switch m := msg.(type) {
			case OpenOrder:
				sub.emit(OpenOrderUpdate{Order: &m})
			case OrderStatusUpdate:
				sub.emit(OpenOrderUpdate{Status: &m})
			case OrderBinding:
				sub.emit(OpenOrderUpdate{Binding: &m})
			case codec.OpenOrderEnd:
				if scope != OpenOrdersScopeAuto {
					refreshPending = false
					sub.emitState(StreamSnapshotComplete, e.connectionSeq(), nil)
				}
			}
		}
		ownedRoute.responsePending = func() bool { return refreshPending }
		ownedRoute.handleAPIErr = func(m codec.APIError, e *engine) {
			if e.singletons[singletonOpenOrders] != ownedRoute {
				return
			}
			delete(e.singletons, singletonOpenOrders)
			sub.closeWithErr(e.apiErr(OpOpenOrders, m))
		}
		e.singletons[singletonOpenOrders] = ownedRoute
		handle.refreshFn = func(ctx context.Context) error {
			return awaitFireAndForget(ctx, e, func(ctx context.Context) error {
				if e.singletons[singletonOpenOrders] != ownedRoute {
					return ErrClosed
				}
				if scope == OpenOrdersScopeAuto {
					return fmt.Errorf("%w: auto-scope open orders", ErrNoSnapshot)
				}
				if refreshPending {
					return operationActive("open orders snapshot")
				}
				if err := e.sendContext(ctx, ownedRoute.request); err != nil {
					return err
				}
				refreshPending = true
				return nil
			})
		}

		sub.emitState(StreamStarted, e.connectionSeq(), nil)
		if err := e.sendContext(ctx, codec.OpenOrdersRequest{Scope: string(scope)}); err != nil {
			delete(e.singletons, singletonOpenOrders)
			sub.closeWithErr(err)
			resp <- result{err: err}
			return
		}
		resp <- result{sub: handle}
	})

	out, err := awaitSubscriptionResponse(ctx, e, resp, func(out result) bool { return out.sub != nil })
	if err != nil {
		return nil, err
	}
	if out.err == nil && out.sub != nil {
		bindContext(ctx, out.sub.Subscription)
	}
	return out.sub, out.err
}

func (e *engine) Executions(ctx context.Context, req ExecutionsRequest) (ExecutionSnapshot, error) {
	sub, err := e.subscribeExecutions(ctx, req, withSnapshotCollector())
	if err != nil {
		return ExecutionSnapshot{}, err
	}
	updates, err := collectSnapshotAndClose(ctx, sub, func(update ExecutionUpdate) (ExecutionUpdate, bool) { return update, true })
	if err != nil {
		return ExecutionSnapshot{}, err
	}
	result := ExecutionSnapshot{}
	for _, update := range updates {
		if update.Execution != nil {
			result.Executions = append(result.Executions, *update.Execution)
		}
		if update.CommissionAndFees != nil {
			result.CommissionAndFees = append(result.CommissionAndFees, *update.CommissionAndFees)
		}
	}
	return result, nil
}

type executionCorrelation struct {
	limit            int
	pendingLimit     int
	byExecID         map[string]*executionCorrelationEntry
	pendingReports   int
	snapshotComplete bool
}

type executionCorrelationEntry struct {
	executionSeen bool
	pending       []codec.CommissionReport
	delivered     codec.CommissionReport
	hasDelivered  bool
}

func newExecutionCorrelation(limit int) *executionCorrelation {
	return &executionCorrelation{
		limit:        limit,
		pendingLimit: limit,
		byExecID:     make(map[string]*executionCorrelationEntry),
	}
}

func (c *executionCorrelation) entry(execID string) (*executionCorrelationEntry, error) {
	if entry, ok := c.byExecID[execID]; ok {
		return entry, nil
	}
	if len(c.byExecID) >= c.limit {
		return nil, executionCorrelationOverflow("distinct execution IDs", c.limit)
	}
	entry := &executionCorrelationEntry{}
	c.byExecID[execID] = entry
	return entry, nil
}

func (c *executionCorrelation) queuePending(entry *executionCorrelationEntry, report codec.CommissionReport) (bool, error) {
	if len(entry.pending) > 0 && entry.pending[len(entry.pending)-1] == report {
		return false, nil
	}
	if c.pendingReports >= c.pendingLimit {
		return false, executionCorrelationOverflow("pending fee-report versions", c.pendingLimit)
	}
	entry.pending = append(entry.pending, report)
	c.pendingReports++
	return true, nil
}

func (c *executionCorrelation) takePending(entry *executionCorrelationEntry) []codec.CommissionReport {
	pending := entry.pending
	entry.pending = nil
	c.pendingReports -= len(pending)
	return pending
}

func (c *executionCorrelation) completeSnapshot() {
	c.snapshotComplete = true
	for execID, entry := range c.byExecID {
		if entry.executionSeen {
			continue
		}
		c.pendingReports -= len(entry.pending)
		delete(c.byExecID, execID)
	}
}

func (c *executionCorrelation) clear() {
	c.byExecID = nil
	c.pendingReports = 0
	c.snapshotComplete = false
}

func executionCorrelationOverflow(resource string, limit int) error {
	return fmt.Errorf("%w: %s exceeds %d", ErrExecutionCorrelationOverflow, resource, limit)
}

func (e *engine) subscribeExecutions(ctx context.Context, req ExecutionsRequest, opts ...SubscriptionOption) (*Subscription[ExecutionUpdate], error) {
	req.SpecificDates = append([]time.Time(nil), req.SpecificDates...)
	type result struct {
		sub *Subscription[ExecutionUpdate]
		err error
	}
	resp := make(chan result, 1)

	enqueueSubscriptionSetup(ctx, e, resp, func() {
		cfg, err := applySubscriptionOptionsFor(e.cfg, OpExecutions, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		wireReq, err := executionsRequest(req, e.serverVersion)
		if err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		wireReq.ReqID = reqID
		sub, ownedRoute := newKeyedSubscriptionRoute[ExecutionUpdate](e, cfg, reqID, OpExecutions, nil)
		sub.expectSnapshot()
		correlation := newExecutionCorrelation(cfg.executionCorrelationLimit)
		ownedRoute.cleanup = correlation.clear
		collectedExecutions := 0
		collectedFees := 0
		terminate := func(err error) {
			if e.keyed[reqID] != ownedRoute {
				return
			}
			sub.cancelFromActor(err)
		}
		emitExecution := func(update Execution) bool {
			if cfg.collectSnapshot {
				if collectedExecutions >= cfg.executionCorrelationLimit {
					terminate(executionCorrelationOverflow("collected execution events", cfg.executionCorrelationLimit))
					return false
				}
				collectedExecutions++
			}
			return sub.emit(ExecutionUpdate{Execution: &update})
		}
		emitFee := func(entry *executionCorrelationEntry, report codec.CommissionReport) bool {
			if entry.hasDelivered && entry.delivered == report {
				return true
			}
			if cfg.collectSnapshot {
				if collectedFees >= cfg.executionCorrelationLimit {
					terminate(executionCorrelationOverflow("collected fee-report events", cfg.executionCorrelationLimit))
					return false
				}
				collectedFees++
			}
			fee, err := fromCodecCommission(report)
			if err != nil {
				terminate(err)
				return false
			}
			entry.delivered = report
			entry.hasDelivered = true
			return sub.emit(ExecutionUpdate{CommissionAndFees: &fee})
		}

		ownedRoute.request = wireReq
		ownedRoute.handleCommission = func(report codec.CommissionReport, _ *engine) {
			entry, ok := correlation.byExecID[report.ExecID]
			if !ok {
				if correlation.snapshotComplete {
					return
				}
				var err error
				entry, err = correlation.entry(report.ExecID)
				if err != nil {
					terminate(err)
					return
				}
			}
			if !entry.executionSeen {
				if _, err := correlation.queuePending(entry, report); err != nil {
					terminate(err)
				}
				return
			}
			emitFee(entry, report)
		}
		ownedRoute.handle = func(msg any, e *engine) {
			switch m := msg.(type) {
			case codec.ExecutionDetail:
				entry, err := correlation.entry(m.ExecID)
				if err != nil {
					terminate(err)
					return
				}
				update, err := fromCodecExecution(m)
				if err != nil {
					terminate(err)
					return
				}
				entry.executionSeen = true
				if !emitExecution(update) {
					return
				}
				for _, report := range correlation.takePending(entry) {
					if !emitFee(entry, report) {
						return
					}
				}
			case codec.ExecutionsEnd:
				correlation.completeSnapshot()
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

func (e *engine) CompletedOrders(ctx context.Context, apiOnly bool) ([]CompletedOrderResult, error) {
	sub, err := e.StreamCompletedOrders(ctx, apiOnly, withSnapshotCollector())
	if err != nil {
		return nil, err
	}
	return collectSnapshotAndClose(ctx, sub, func(order CompletedOrderResult) (CompletedOrderResult, bool) { return order, true })
}

func (e *engine) StreamCompletedOrders(ctx context.Context, apiOnly bool, opts ...SubscriptionOption) (*Subscription[CompletedOrderResult], error) {
	type result struct {
		sub *Subscription[CompletedOrderResult]
		err error
	}
	resp := make(chan result, 1)

	enqueueSingletonSubscriptionSetup(ctx, e, singletonCompletedOrders, resp, func() {
		if _, exists := e.singletons[singletonCompletedOrders]; exists {
			resp <- result{err: operationActive("completed orders")}
			return
		}
		cfg, err := applySubscriptionOptionsFor(e.cfg, OpCompletedOrders, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		sub, ownedRoute := newSingletonSubscriptionRoute[CompletedOrderResult](
			e, cfg, singletonCompletedOrders, OpCompletedOrders, nil,
		)
		sub.expectSnapshot()
		ownedRoute.onDisconnect = func(_ *engine, err error) bool {
			sub.closeWithErr(interrupted(err))
			return false
		}
		ownedRoute.handleAPIErr = func(msg codec.APIError, eng *engine) {
			if eng.singletons[singletonCompletedOrders] != ownedRoute {
				return
			}
			delete(eng.singletons, singletonCompletedOrders)
			sub.closeWithErr(eng.apiErr(OpCompletedOrders, msg))
		}
		ownedRoute.handle = func(msg any, eng *engine) {
			switch m := msg.(type) {
			case codec.CompletedOrder:
				order, err := fromCodecCompletedOrder(m)
				if err != nil {
					sub.cancelFromActor(err)
					return
				}
				sub.emit(order)
			case codec.CompletedOrderEnd:
				delete(eng.singletons, singletonCompletedOrders)
				sub.emitState(StreamSnapshotComplete, eng.connectionSeq(), nil)
				sub.closeWithErr(nil)
			}
		}
		e.singletons[singletonCompletedOrders] = ownedRoute
		sub.emitState(StreamStarted, e.connectionSeq(), nil)
		if err := e.sendContext(ctx, codec.CompletedOrdersRequest{APIOnly: apiOnly}); err != nil {
			delete(e.singletons, singletonCompletedOrders)
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

// orderRollbackTimeout bounds cancellation admission after a bracket place
// frame was admitted but a later frame was not.
const orderRollbackTimeout = 15 * time.Second

// placeOrderResult is the single value the PlaceOrder setup delivers on resp.
// Exactly one is sent on every path: before transport admission it carries an
// error; after admission it carries the handle that owns the live order.
type placeOrderResult struct {
	handle *OrderHandle
	err    error
}

type bracketOrderResult struct {
	bracket BracketOrder
	err     error
}

func awaitPlaceOrderResponse(ctx context.Context, e *engine, resp <-chan placeOrderResult) (*OrderHandle, error) {
	out, err := awaitAdmittedResponse(ctx, e, resp)
	if err != nil {
		return nil, err
	}
	if out.handle != nil {
		return out.handle, nil
	}
	return nil, out.err
}

func awaitBracketOrderResponse(ctx context.Context, e *engine, resp <-chan bracketOrderResult) (BracketOrder, error) {
	out, err := awaitAdmittedResponse(ctx, e, resp)
	if err != nil {
		return BracketOrder{}, err
	}
	if out.bracket.Parent != nil {
		return out.bracket, nil
	}
	return BracketOrder{}, out.err
}

// PlaceOrder submits a new order and returns an OrderHandle that tracks its
// lifecycle. The handle receives OpenOrder, OrderStatus, Execution, and
// Commission events via dual dispatch. The order can be modified or cancelled
// through the returned handle.
//
// Transport-queue admission is the ownership boundary. If ctx is canceled or
// the engine closes after admission, PlaceOrder still returns the handle and a
// nil error; the handle remains the caller's authority to observe or cancel the
// order. Before admission, PlaceOrder returns an error and no handle.
func (e *engine) PlaceOrder(ctx context.Context, req PlaceOrderRequest) (*OrderHandle, error) {
	if err := validateOrderRequest(req); err != nil {
		return nil, err
	}
	req = clonePlaceOrderRequest(req)
	resp := make(chan placeOrderResult, 1)
	// enqueueReadySetup with a drop callback guarantees resp receives exactly
	// one result even when ctx is canceled before the actor runs the setup.
	enqueueReadySetup(ctx, e, func() {
		resp <- placeOrderResult{err: context.Cause(ctx)}
	}, func() {
		if err := validateContractFieldSupport(req.Contract, "place order", e.serverVersion, placeOrderContractFields(e.serverVersion)); err != nil {
			resp <- placeOrderResult{err: err}
			return
		}
		if err := validateOrderServerVersion(req.Order, e.serverVersion); err != nil {
			resp <- placeOrderResult{err: err}
			return
		}

		orderID, err := e.allocOrderID()
		if err != nil {
			resp <- placeOrderResult{err: err}
			return
		}
		handle := e.bindOrderHandle(orderID, req.Contract)

		write, err := e.sendTrackedContext(ctx, toCodecPlaceOrder(orderID, req))
		if err != nil {
			delete(e.orders, orderID)
			handle.closeWithErr(err)
			resp <- placeOrderResult{err: err}
			return
		}
		e.trackOrderWrite(orderID, write)

		resp <- placeOrderResult{handle: handle}
	})

	return awaitPlaceOrderResponse(ctx, e, resp)
}

// PlaceBracket allocates three consecutive order IDs and sends the parent,
// take-profit, and stop-loss in one actor turn. The first two orders are staged
// with Transmit=false; the final child is transmitted and releases the bracket.
func (e *engine) PlaceBracket(ctx context.Context, req PlaceBracketRequest) (BracketOrder, error) {
	prepared, err := prepareBracketRequest(req)
	if err != nil {
		return BracketOrder{}, err
	}
	req = prepared
	resp := make(chan bracketOrderResult, 1)
	enqueueReadySetup(ctx, e, func() {
		resp <- bracketOrderResult{err: context.Cause(ctx)}
	}, func() {
		if err := validateContractFieldSupport(req.Contract, "place bracket", e.serverVersion, placeOrderContractFields(e.serverVersion)); err != nil {
			resp <- bracketOrderResult{err: err}
			return
		}
		for _, order := range []Order{req.Parent, req.TakeProfit, req.StopLoss} {
			if err := validateOrderServerVersion(order, e.serverVersion); err != nil {
				resp <- bracketOrderResult{err: err}
				return
			}
		}

		parentID, err := e.allocOrderID()
		if err != nil {
			resp <- bracketOrderResult{err: err}
			return
		}
		takeProfitID, err := e.allocOrderID()
		if err != nil {
			resp <- bracketOrderResult{err: err}
			return
		}
		stopLossID, err := e.allocOrderID()
		if err != nil {
			resp <- bracketOrderResult{err: err}
			return
		}
		req.TakeProfit.ParentID = parentID
		req.StopLoss.ParentID = parentID

		bracket := BracketOrder{
			Parent:     e.bindOrderHandle(parentID, req.Contract),
			TakeProfit: e.bindOrderHandle(takeProfitID, req.Contract),
			StopLoss:   e.bindOrderHandle(stopLossID, req.Contract),
		}
		allIDs := []int64{parentID, takeProfitID, stopLossID}
		sentIDs := make([]int64, 0, len(allIDs))
		orders := []struct {
			id    int64
			order Order
		}{
			{parentID, req.Parent},
			{takeProfitID, req.TakeProfit},
			{stopLossID, req.StopLoss},
		}
		for _, item := range orders {
			write, err := e.sendTrackedContext(ctx, toCodecPlaceOrder(item.id, PlaceOrderRequest{Contract: req.Contract, Order: item.order}))
			if err != nil {
				resp <- bracketOrderResult{err: e.cancelAndCloseOrderRoutes(sentIDs, allIDs, err)}
				return
			}
			e.trackOrderWrite(item.id, write)
			sentIDs = append(sentIDs, item.id)
		}
		resp <- bracketOrderResult{bracket: bracket}
	})

	return awaitBracketOrderResponse(ctx, e, resp)
}

// PlacePresetBracket submits one parent request whose attached-order metadata
// asks TWS to create stop-loss and profit-taker children from its configured
// order presets. Unlike PlaceBracket, the child instructions are owned by the
// TWS configuration and no separate child place frames are sent.
func (e *engine) PlacePresetBracket(ctx context.Context, req PlaceOrderRequest) (BracketOrder, error) {
	if err := validateOrderRequest(req); err != nil {
		return BracketOrder{}, err
	}
	req = clonePlaceOrderRequest(req)
	resp := make(chan bracketOrderResult, 1)
	enqueueReadySetup(ctx, e, func() {
		resp <- bracketOrderResult{err: context.Cause(ctx)}
	}, func() {
		if e.serverVersion < protocol.MinServerVersionAttachedOrders {
			resp <- bracketOrderResult{err: fmt.Errorf("ibkr: preset brackets require server_version %d, negotiated %d: %w", protocol.MinServerVersionAttachedOrders, e.serverVersion, ErrUnsupportedServerVersion)}
			return
		}
		if err := validateContractFieldSupport(req.Contract, "place preset bracket", e.serverVersion, placeOrderContractFields(e.serverVersion)); err != nil {
			resp <- bracketOrderResult{err: err}
			return
		}
		if err := validateOrderServerVersion(req.Order, e.serverVersion); err != nil {
			resp <- bracketOrderResult{err: err}
			return
		}

		parentID, err := e.allocOrderID()
		if err != nil {
			resp <- bracketOrderResult{err: err}
			return
		}
		stopLossID, err := e.allocOrderID()
		if err != nil {
			resp <- bracketOrderResult{err: err}
			return
		}
		takeProfitID, err := e.allocOrderID()
		if err != nil {
			resp <- bracketOrderResult{err: err}
			return
		}
		bracket := BracketOrder{
			Parent:     e.bindOrderHandle(parentID, req.Contract),
			TakeProfit: e.bindOrderHandle(takeProfitID, req.Contract),
			StopLoss:   e.bindOrderHandle(stopLossID, req.Contract),
		}
		e.orders[parentID].attachedOrderIDs = []int64{stopLossID, takeProfitID}
		allIDs := []int64{parentID, stopLossID, takeProfitID}
		write, err := e.sendTrackedContext(ctx, toCodecPresetBracketOrder(parentID, stopLossID, takeProfitID, req))
		if err != nil {
			resp <- bracketOrderResult{err: e.cancelAndCloseOrderRoutes(nil, allIDs, err)}
			return
		}
		e.trackOrderWrite(parentID, write)
		resp <- bracketOrderResult{bracket: bracket}
	})

	return awaitBracketOrderResponse(ctx, e, resp)
}

// cancelAndCloseOrderRoutes rolls back a bracket placement on the actor
// goroutine. It sends cancellation only for admitted place frames. Any partial
// bracket returns an OrderRecoveryError naming every admitted ID because queue
// admission of a cancellation is not a Gateway acknowledgement. Only a
// failure before the first place admission returns placementErr directly.
func (e *engine) cancelAndCloseOrderRoutes(sentIDs, allIDs []int64, placementErr error) error {
	var cancelErrs []error
	if len(sentIDs) > 0 {
		cancelCtx, cancel := context.WithTimeout(context.Background(), orderRollbackTimeout)
		defer cancel()
		for _, orderID := range sentIDs {
			if err := e.sendContext(cancelCtx, codec.CancelOrderRequest{OrderID: orderID}); err != nil {
				cancelErrs = append(cancelErrs, fmt.Errorf("cancel order %d: %w", orderID, err))
			}
		}
	}
	resultErr := placementErr
	if len(sentIDs) > 0 {
		resultErr = newOrderRecoveryError(sentIDs, placementErr, errors.Join(cancelErrs...))
	}
	for _, orderID := range allIDs {
		if or, ok := e.orders[orderID]; ok {
			e.closeOrderRoute(orderID, or, resultErr)
		}
	}
	return resultErr
}

// bindOrderHandle installs a new order route and its public handle. It must be
// called on the actor goroutine before the corresponding place_order is sent.
func (e *engine) bindOrderHandle(orderID int64, contract Contract) *OrderHandle {
	handle := newOrderHandle(orderID, e.cfg.orderEventBuffer)
	handle.cancelFn = func(ctx context.Context, cfg cancelConfig) error {
		return e.CancelOrder(ctx, orderID, cfg)
	}
	handle.replaceFn = func(ctx context.Context, order Order) error {
		if err := validateOrderRequest(PlaceOrderRequest{Contract: contract, Order: order}); err != nil {
			return err
		}
		order = cloneOrder(order)
		return awaitFireAndForget(ctx, e, func(ctx context.Context) error {
			if err := validateContractFieldSupport(contract, "modify order", e.serverVersion, placeOrderContractFields(e.serverVersion)); err != nil {
				return err
			}
			if err := validateOrderServerVersion(order, e.serverVersion); err != nil {
				return err
			}
			or, ok := e.orders[orderID]
			if !ok || or.closed || or.handle.isDone() {
				return ErrClosed
			}
			if or.recoveryRequired {
				return ErrOrderRecoveryRequired
			}
			return e.sendContext(ctx, toCodecPlaceOrder(orderID, PlaceOrderRequest{
				Contract: contract,
				Order:    order,
			}))
		})
	}
	handle.detachFn = func() {
		e.enqueue(func() {
			if or, ok := e.orders[orderID]; ok {
				e.closeOrderRoute(orderID, or, nil)
				return
			}
			handle.closeWithErr(nil)
		})
	}
	e.orders[orderID] = &orderRoute{orderID: orderID, handle: handle}
	return handle
}

func (e *engine) trackOrderWrite(orderID int64, write transportWriteKey) {
	if e.pendingOrderWrites == nil {
		e.pendingOrderWrites = make(map[transportWriteKey]int64)
	}
	e.pendingOrderWrites[write] = orderID
	e.orders[orderID].pendingWrite = write
}

// PreviewOrder submits a what-if order and returns the margin-and-commission
// preview the Gateway attaches to the single open_order echo. The encoder sets
// WhatIf=true on the place_order frame; the difference is purely in how the
// reply is consumed.
// No OrderHandle is ever created — the preview route is resolved and torn down
// on the one open_order echo, and nothing rests on the server.
func (e *engine) PreviewOrder(ctx context.Context, req PlaceOrderRequest) (OrderState, error) {
	if err := validateOrderRequest(req); err != nil {
		return OrderState{}, err
	}
	req = clonePlaceOrderRequest(req)
	type setup struct {
		ch  chan previewResult
		err error
	}

	setupResp := make(chan setup, 1)
	orderIDCh := make(chan int64, 1)

	enqueueOneShotSetup(ctx, e, func() {
		if err := validateContractFieldSupport(req.Contract, "preview order", e.serverVersion, placeOrderContractFields(e.serverVersion)); err != nil {
			setupResp <- setup{err: err}
			return
		}
		if err := validateOrderServerVersion(req.Order, e.serverVersion); err != nil {
			setupResp <- setup{err: err}
			return
		}

		orderID, err := e.allocOrderID()
		if err != nil {
			setupResp <- setup{err: err}
			return
		}
		orderIDCh <- orderID
		ch := make(chan previewResult, 1)
		e.previews[orderID] = &previewRoute{result: ch}

		if err := e.sendContext(ctx, toCodecPreviewOrder(orderID, req)); err != nil {
			delete(e.previews, orderID)
			setupResp <- setup{err: err}
			return
		}
		setupResp <- setup{ch: ch}
	})

	cleanup := func() {
		select {
		case orderID := <-orderIDCh:
			e.enqueue(func() {
				delete(e.previews, orderID)
			})
		default:
		}
	}

	select {
	case s := <-setupResp:
		if s.err != nil {
			return OrderState{}, s.err
		}
		select {
		case pr := <-s.ch:
			if pr.err != nil {
				return OrderState{}, pr.err
			}
			return pr.state, nil
		case <-ctx.Done():
			cleanup()
			return OrderState{}, context.Cause(ctx)
		case <-e.done:
			return OrderState{}, e.closedOperationError()
		}
	case <-ctx.Done():
		cleanup()
		return OrderState{}, context.Cause(ctx)
	case <-e.done:
		return OrderState{}, e.closedOperationError()
	}
}

// CancelOrder sends a cancel request for the given order ID. This is
// fire-and-forget; the cancellation result arrives via the OrderHandle's
// events channel as an OrderStatus with Status "Cancelled".
func (e *engine) CancelOrder(ctx context.Context, orderID int64, cfg cancelConfig) error {
	if err := validateOrderID("OrderID", orderID, false); err != nil {
		return err
	}
	return awaitFireAndForget(ctx, e, func(ctx context.Context) error {
		return e.sendContext(ctx, cancelOrderRequest(orderID, cfg))
	})
}

// GlobalCancel requests cancellation of all open orders. This is
// fire-and-forget; individual cancellation results arrive via any active
// OrderHandle events channels.
func (e *engine) GlobalCancel(ctx context.Context, cfg cancelConfig) error {
	return awaitFireAndForget(ctx, e, func(ctx context.Context) error {
		req, err := globalCancelRequest(cfg)
		if err != nil {
			return err
		}
		return e.sendContext(ctx, req)
	})
}

func (e *engine) installExerciseRoute(reqID int) *ExerciseHandle {
	orderHandle := newOrderHandle(int64(reqID), e.cfg.orderEventBuffer)
	handle := &ExerciseHandle{requestID: protocolIDFromInt[RequestID](reqID), order: orderHandle}
	var exerciseRoute *route
	var exerciseOrderRoute *orderRoute
	closeExercise := func(err error) {
		if !exerciseOrderRoute.closed {
			e.closeOrderRoute(int64(reqID), exerciseOrderRoute, err)
		}
		if e.keyed[reqID] == exerciseRoute {
			e.deleteKeyedRoute(reqID)
		}
	}
	exerciseRoute = &route{
		opKind: OpExerciseOptions,
		handle: func(any, *engine) {},
		handleAPIErr: func(m codec.APIError, e *engine) {
			apiErr, _ := errors.AsType[*APIError](e.apiErr(OpExerciseOptions, m))
			if m.Code == ErrCodeOrderTIFSetFromPreset {
				if !orderHandle.emitWarning(apiErr) {
					closeExercise(nil)
				}
				return
			}
			closeExercise(apiErr)
		},
		onDisconnect: func(e *engine, err error) bool {
			closeExercise(errors.Join(ErrInterrupted, err))
			return false
		},
		close: func(err error) {
			if err == nil {
				closeExercise(nil)
				return
			}
			closeExercise(errors.Join(ErrInterrupted, err))
		},
	}
	exerciseOrderRoute = &orderRoute{
		orderID: int64(reqID),
		handle:  orderHandle,
		cleanup: func() {
			if e.keyed[reqID] == exerciseRoute {
				e.deleteKeyedRoute(reqID)
			}
		},
	}
	e.keyed[reqID] = exerciseRoute
	e.orders[int64(reqID)] = exerciseOrderRoute
	orderHandle.detachFn = func() {
		e.enqueue(func() { closeExercise(nil) })
	}
	return handle
}

func (e *engine) ExerciseOptions(ctx context.Context, req ExerciseOptionsRequest) (*ExerciseHandle, error) {
	if err := validateExerciseOptionsRequest(req); err != nil {
		return nil, err
	}
	req.Contract = cloneContract(req.Contract)
	type result struct {
		handle *ExerciseHandle
		err    error
	}
	resp := make(chan result, 1)
	enqueueReadySetup(ctx, e, func() { resp <- result{err: context.Cause(ctx)} }, func() {
		if err := validateContractFieldSupport(req.Contract, "exercise options", e.serverVersion, 0); err != nil {
			resp <- result{err: err}
			return
		}
		override := 0
		if req.Override {
			override = 1
		}
		reqID := e.allocReqID()
		handle := e.installExerciseRoute(reqID)
		if err := e.sendContext(ctx, codec.ExerciseOptionsRequest{
			ReqID:            reqID,
			Contract:         toCodecContract(req.Contract),
			ExerciseAction:   int(req.ExerciseAction),
			ExerciseQuantity: req.ExerciseQuantity,
			Account:          req.Account,
			Override:         override,
		}); err != nil {
			if or, ok := e.orders[int64(reqID)]; ok {
				e.closeOrderRoute(int64(reqID), or, err)
			}
			e.deleteKeyedRoute(reqID)
			resp <- result{err: err}
			return
		}
		resp <- result{handle: handle}
	})
	out, err := awaitAdmittedResponse(ctx, e, resp)
	if err != nil {
		return nil, err
	}
	return out.handle, out.err
}

func fromCodecOpenOrder(m codec.OpenOrder) (OpenOrder, error) {
	order, _, err := decodeCodecOpenOrder(m)
	return order, err
}

func decodeCodecOpenOrder(m codec.OpenOrder) (OpenOrder, OrderState, error) {
	details, err := fromCodecCompletedOrder(codec.CompletedOrder{OrderDetails: m.OrderDetails})
	if err != nil {
		return OpenOrder{}, OrderState{}, err
	}
	initMarginBefore, err := parseOptionalDecimalPointer(m.InitMarginBefore, "open order init margin before")
	if err != nil {
		return OpenOrder{}, OrderState{}, err
	}
	maintMarginBefore, err := parseOptionalDecimalPointer(m.MaintMarginBefore, "open order maint margin before")
	if err != nil {
		return OpenOrder{}, OrderState{}, err
	}
	equityWithLoanBefore, err := parseOptionalDecimalPointer(m.EquityWithLoanBefore, "open order equity with loan before")
	if err != nil {
		return OpenOrder{}, OrderState{}, err
	}
	initMarginChange, err := parseOptionalDecimalPointer(m.InitMarginChange, "open order init margin change")
	if err != nil {
		return OpenOrder{}, OrderState{}, err
	}
	maintMarginChange, err := parseOptionalDecimalPointer(m.MaintMarginChange, "open order maint margin change")
	if err != nil {
		return OpenOrder{}, OrderState{}, err
	}
	equityWithLoanChange, err := parseOptionalDecimalPointer(m.EquityWithLoanChange, "open order equity with loan change")
	if err != nil {
		return OpenOrder{}, OrderState{}, err
	}
	initMarginAfter, err := parseOptionalDecimalPointer(m.InitMarginAfter, "open order init margin after")
	if err != nil {
		return OpenOrder{}, OrderState{}, err
	}
	maintMarginAfter, err := parseOptionalDecimalPointer(m.MaintMarginAfter, "open order maint margin after")
	if err != nil {
		return OpenOrder{}, OrderState{}, err
	}
	equityWithLoanAfter, err := parseOptionalDecimalPointer(m.EquityWithLoanAfter, "open order equity with loan after")
	if err != nil {
		return OpenOrder{}, OrderState{}, err
	}
	commission, err := parseOptionalDecimalPointer(m.Commission, "open order commission")
	if err != nil {
		return OpenOrder{}, OrderState{}, err
	}
	minCommission, err := parseOptionalDecimalPointer(m.MinCommission, "open order min commission")
	if err != nil {
		return OpenOrder{}, OrderState{}, err
	}
	maxCommission, err := parseOptionalDecimalPointer(m.MaxCommission, "open order max commission")
	if err != nil {
		return OpenOrder{}, OrderState{}, err
	}
	var initMarginBeforeOutsideRTH, maintMarginBeforeOutsideRTH, equityWithLoanBeforeOutsideRTH *decimal.Decimal
	var initMarginChangeOutsideRTH, maintMarginChangeOutsideRTH, equityWithLoanChangeOutsideRTH *decimal.Decimal
	var initMarginAfterOutsideRTH, maintMarginAfterOutsideRTH, equityWithLoanAfterOutsideRTH *decimal.Decimal
	var suggestedSize *decimal.Decimal
	for _, field := range []struct {
		raw   string
		label string
		dst   **decimal.Decimal
	}{
		{m.InitMarginBeforeOutsideRTH, "open order init margin before outside RTH", &initMarginBeforeOutsideRTH},
		{m.MaintMarginBeforeOutsideRTH, "open order maint margin before outside RTH", &maintMarginBeforeOutsideRTH},
		{m.EquityWithLoanBeforeOutsideRTH, "open order equity with loan before outside RTH", &equityWithLoanBeforeOutsideRTH},
		{m.InitMarginChangeOutsideRTH, "open order init margin change outside RTH", &initMarginChangeOutsideRTH},
		{m.MaintMarginChangeOutsideRTH, "open order maint margin change outside RTH", &maintMarginChangeOutsideRTH},
		{m.EquityWithLoanChangeOutsideRTH, "open order equity with loan change outside RTH", &equityWithLoanChangeOutsideRTH},
		{m.InitMarginAfterOutsideRTH, "open order init margin after outside RTH", &initMarginAfterOutsideRTH},
		{m.MaintMarginAfterOutsideRTH, "open order maint margin after outside RTH", &maintMarginAfterOutsideRTH},
		{m.EquityWithLoanAfterOutsideRTH, "open order equity with loan after outside RTH", &equityWithLoanAfterOutsideRTH},
		{m.SuggestedSize, "open order suggested size", &suggestedSize},
	} {
		value, err := parseOptionalDecimalPointer(field.raw, field.label)
		if err != nil {
			return OpenOrder{}, OrderState{}, err
		}
		*field.dst = value
	}
	var allocations []OrderAllocation
	if len(m.Allocations) != 0 {
		allocations = make([]OrderAllocation, len(m.Allocations))
	}
	for i, allocation := range m.Allocations {
		allocations[i].Account = allocation.Account
		for _, field := range []struct {
			raw   string
			label string
			dst   **decimal.Decimal
		}{
			{allocation.Position, "order allocation position", &allocations[i].Position},
			{allocation.PositionDesired, "order allocation desired position", &allocations[i].PositionDesired},
			{allocation.PositionAfter, "order allocation position after", &allocations[i].PositionAfter},
			{allocation.DesiredAllocQty, "order allocation desired quantity", &allocations[i].DesiredAllocQty},
			{allocation.AllowedAllocQty, "order allocation allowed quantity", &allocations[i].AllowedAllocQty},
		} {
			value, err := parseOptionalDecimalPointer(field.raw, field.label)
			if err != nil {
				return OpenOrder{}, OrderState{}, err
			}
			*field.dst = value
		}
		allocations[i].IsMonetary, err = parseOptionalBoolPointer(allocation.IsMonetary, "order allocation is monetary")
		if err != nil {
			return OpenOrder{}, OrderState{}, err
		}
	}
	state := OrderState{
		Status:                         OrderStatus(m.Status),
		InitMarginBefore:               initMarginBefore,
		MaintMarginBefore:              maintMarginBefore,
		EquityWithLoanBefore:           equityWithLoanBefore,
		InitMarginChange:               initMarginChange,
		MaintMarginChange:              maintMarginChange,
		EquityWithLoanChange:           equityWithLoanChange,
		InitMarginAfter:                initMarginAfter,
		MaintMarginAfter:               maintMarginAfter,
		EquityWithLoanAfter:            equityWithLoanAfter,
		CommissionAndFees:              commission,
		MinCommissionAndFees:           minCommission,
		MaxCommissionAndFees:           maxCommission,
		CommissionAndFeesCurrency:      m.CommissionCurrency,
		MarginCurrency:                 m.MarginCurrency,
		InitMarginBeforeOutsideRTH:     initMarginBeforeOutsideRTH,
		MaintMarginBeforeOutsideRTH:    maintMarginBeforeOutsideRTH,
		EquityWithLoanBeforeOutsideRTH: equityWithLoanBeforeOutsideRTH,
		InitMarginChangeOutsideRTH:     initMarginChangeOutsideRTH,
		MaintMarginChangeOutsideRTH:    maintMarginChangeOutsideRTH,
		EquityWithLoanChangeOutsideRTH: equityWithLoanChangeOutsideRTH,
		InitMarginAfterOutsideRTH:      initMarginAfterOutsideRTH,
		MaintMarginAfterOutsideRTH:     maintMarginAfterOutsideRTH,
		EquityWithLoanAfterOutsideRTH:  equityWithLoanAfterOutsideRTH,
		SuggestedSize:                  suggestedSize,
		RejectReason:                   m.RejectReason,
		Allocations:                    allocations,
		WarningText:                    m.WarningText,
	}
	order := OpenOrder{Contract: details.Contract, Order: details.Order, State: state}
	return order, state, nil
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
	clientIDValue, err := parseOptionalInt32(m.ClientID, "order status client id")
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
		ClientID:      ClientID(clientIDValue),
		WhyHeld:       m.WhyHeld,
		MktCapPrice:   mktCapPrice,
	}, nil
}

func fromCodecExecution(m codec.ExecutionDetail) (Execution, error) {
	contract, err := fromCodecContract(m.Contract)
	if err != nil {
		return Execution{}, err
	}
	shares, err := parseRequiredDecimal(m.Shares, "execution shares")
	if err != nil {
		return Execution{}, err
	}
	price, err := parseRequiredDecimal(m.Price, "execution price")
	if err != nil {
		return Execution{}, err
	}
	ts, err := parseExecutionTime(m.Time)
	if err != nil {
		return Execution{}, err
	}
	permID, err := parseOptionalInt64(m.PermID, "execution permanent id")
	if err != nil {
		return Execution{}, err
	}
	clientIDValue, err := parseOptionalInt32(m.ClientID, "execution client id")
	if err != nil {
		return Execution{}, err
	}
	liquidation, err := parseOptionalInt(m.Liquidation, "execution liquidation")
	if err != nil {
		return Execution{}, err
	}
	cumulativeQuantity, err := parseOptionalDecimal(m.CumulativeQuantity, "execution cumulative quantity")
	if err != nil {
		return Execution{}, err
	}
	averagePrice, err := parseOptionalDecimal(m.AveragePrice, "execution average price")
	if err != nil {
		return Execution{}, err
	}
	economicValueMultiplier, err := parseOptionalDecimalPointer(m.EconomicValueMultiplier, "execution economic value multiplier")
	if err != nil {
		return Execution{}, err
	}
	lastLiquidity, err := parseOptionalInt(m.LastLiquidity, "execution last liquidity")
	if err != nil {
		return Execution{}, err
	}
	pendingPriceRevision, err := parseOptionalBoolString(m.PendingPriceRevision, "execution pending price revision")
	if err != nil {
		return Execution{}, err
	}
	optExerciseOrLapseType, err := parseOptionalInt(m.OptExerciseOrLapseType, "execution option exercise or lapse type")
	if err != nil {
		return Execution{}, err
	}
	optionExerciseType := OptionExerciseType(optExerciseOrLapseType)
	if optExerciseOrLapseType == -1 {
		optionExerciseType = OptionExerciseTypeNone
	}
	return Execution{
		OrderID:                 m.OrderID,
		Contract:                contract,
		ExecID:                  m.ExecID,
		Time:                    ts,
		Account:                 m.Account,
		Exchange:                m.Exchange,
		Side:                    ExecutionSide(m.Side),
		Shares:                  shares,
		Price:                   price,
		PermID:                  permID,
		ClientID:                ClientID(clientIDValue),
		Liquidation:             liquidation,
		CumulativeQuantity:      cumulativeQuantity,
		AveragePrice:            averagePrice,
		OrderRef:                m.OrderRef,
		EconomicValueRule:       m.EconomicValueRule,
		EconomicValueMultiplier: economicValueMultiplier,
		ModelCode:               m.ModelCode,
		Liquidity:               ExecutionLiquidity(lastLiquidity),
		PriceRevisionPending:    pendingPriceRevision,
		Submitter:               m.Submitter,
		OptionExerciseType:      optionExerciseType,
	}, nil
}

// parseExecutionTime handles the Gateway's execution time forms: the UTC
// dash notation ("20260610-19:58:22", observed live 2026-06-10), its
// server_version 214 UTC suffix form ("20260610-19:58:22Z"), the
// space-and-zone form ("20260413 13:35:50 US/Eastern"), and RFC3339 (from
// test transcripts).
func parseExecutionTime(raw string) (time.Time, error) {
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts, nil
	}
	if ts, err := time.Parse("20060102-15:04:05Z07:00", raw); err == nil {
		return ts.UTC(), nil
	}
	// IBKR UTC dash notation: "YYYYMMDD-HH:MM:SS".
	if ts, err := time.Parse("20060102-15:04:05", raw); err == nil {
		return ts, nil
	}
	// IBKR native: "YYYYMMDD HH:MM:SS TZ_NAME" where TZ_NAME is an IANA zone
	// like "US/Eastern", "US/Central", "Europe/London", etc.
	if idx := strings.LastIndex(raw, " "); idx > 0 && idx > 16 {
		dtPart := raw[:idx]
		tzPart := raw[idx+1:]
		loc, err := time.LoadLocation(tzPart)
		if err == nil {
			if ts, err := time.ParseInLocation("20060102 15:04:05", dtPart, loc); err == nil {
				return ts.UTC(), nil
			}
		}
	}
	return time.Time{}, inboundProtocolError("execution time", fmt.Errorf("parse %q", raw))
}

func fromCodecCommission(m codec.CommissionReport) (CommissionAndFeesReport, error) {
	commissionAndFees, err := parseOptionalDecimalPointer(m.Commission, "commission and fees amount")
	if err != nil {
		return CommissionAndFeesReport{}, err
	}
	realized, err := parseOptionalDecimalPointer(m.RealizedPNL, "commission and fees realized pnl")
	if err != nil {
		return CommissionAndFeesReport{}, err
	}
	bondYield, err := parseOptionalDecimalPointer(m.Yield, "commission and fees bond yield")
	if err != nil {
		return CommissionAndFeesReport{}, err
	}
	yieldRedemptionDate, err := parseYieldRedemptionDate(m.YieldRedemptionDate)
	if err != nil {
		return CommissionAndFeesReport{}, err
	}
	return CommissionAndFeesReport{
		ExecID:              m.ExecID,
		Amount:              commissionAndFees,
		Currency:            m.Currency,
		RealizedPnL:         realized,
		BondYield:           bondYield,
		YieldRedemptionDate: yieldRedemptionDate,
	}, nil
}

func parseYieldRedemptionDate(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "0" || trimmed == "2147483647" {
		return "", nil
	}
	if _, err := time.Parse("20060102", trimmed); err != nil {
		return "", inboundProtocolError("commission and fees yield redemption date", fmt.Errorf("parse %q: %w", raw, err))
	}
	return trimmed, nil
}

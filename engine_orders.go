package ibkr

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
)

func (e *engine) OpenOrdersSnapshot(ctx context.Context, scope OpenOrdersScope) ([]OpenOrder, error) {
	sub, err := e.SubscribeOpenOrders(ctx, scope)
	if err != nil {
		return nil, err
	}
	defer func() { _ = sub.Close() }()
	return collectSnapshot(ctx, sub, func(update OpenOrderUpdate) (OpenOrder, bool) {
		if update.Order == nil {
			return OpenOrder{}, false
		}
		return *update.Order, true
	})
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

		cfg, err := applySubscriptionOptions(e.cfg, opts)
		if err != nil {
			resp <- result{err: err}
			return
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
					emitSubscription(sub, OpenOrderUpdate{Order: &m.order})
				case OrderStatusUpdate:
					emitSubscription(sub, OpenOrderUpdate{Status: &m})
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
	return collectSnapshot(ctx, sub, func(update ExecutionUpdate) (ExecutionUpdate, bool) { return update, true })
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

		cfg, err := applySubscriptionOptions(e.cfg, opts)
		if err != nil {
			resp <- result{err: err}
			return
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
					if !emitSubscription(sub, update) {
						return
					}
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

// orphanCancelTimeout bounds the best-effort cancel of an order that reached
// the wire just as its PlaceOrder caller's context was canceled.
const orphanCancelTimeout = 15 * time.Second

// placeOrderResult is the single value the PlaceOrder setup delivers on resp.
// Exactly one is sent on every path (drop-on-cancel, not-ready, send error, or
// success), so the orphan resolver's drain always completes.
type placeOrderResult struct {
	handle *OrderHandle
	err    error
}

type bracketOrderResult struct {
	bracket BracketOrder
	err     error
}

// PlaceOrder submits a new order and returns an OrderHandle that tracks its
// lifecycle. The handle receives OpenOrder, OrderStatus, Execution, and
// Commission events via dual dispatch. The order can be modified or cancelled
// through the returned handle.
//
// If ctx is canceled after the order already reached the wire — the window
// between the actor sending place_order and the caller receiving the handle —
// PlaceOrder returns ctx.Err() and the engine best-effort cancels the now
// ownerless order (bounded background context) and detaches its handle, so a
// canceled call cannot leave a live order resting with no way to reach it.
func (e *engine) PlaceOrder(ctx context.Context, req PlaceOrderRequest) (*OrderHandle, error) {
	if err := validateOrderRequest(req, orderIntentPlace); err != nil {
		return nil, err
	}
	req = clonePlaceOrderRequest(req)
	resp := make(chan placeOrderResult, 1)
	// enqueueReadySetup with a drop callback guarantees resp receives exactly
	// one result even when ctx is canceled before the actor runs the setup;
	// nothing reached the wire on that path, so a plain error is correct.
	enqueueReadySetup(ctx, e, func() {
		resp <- placeOrderResult{err: ctx.Err()}
	}, func() {
		if !e.isReady() {
			resp <- placeOrderResult{err: ErrNotReady}
			return
		}

		orderID := e.allocOrderID()
		handle := e.bindOrderHandle(orderID, req.Contract)

		if err := e.sendContext(ctx, toCodecPlaceOrder(orderID, req)); err != nil {
			delete(e.orders, orderID)
			handle.closeWithErr(err)
			resp <- placeOrderResult{err: err}
			return
		}

		resp <- placeOrderResult{handle: handle}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.resolveOrphanedPlaceOrder(resp, ctx.Err())
	})
	if err != nil {
		return nil, err
	}
	return out.handle, out.err
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
		resp <- bracketOrderResult{err: ctx.Err()}
	}, func() {
		if !e.isReady() {
			resp <- bracketOrderResult{err: ErrNotReady}
			return
		}

		parentID := e.allocOrderID()
		takeProfitID := e.allocOrderID()
		stopLossID := e.allocOrderID()
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
			if err := e.sendContext(ctx, toCodecPlaceOrder(item.id, PlaceOrderRequest{Contract: req.Contract, Order: item.order})); err != nil {
				e.cancelAndCloseOrderRoutes(sentIDs, allIDs, err)
				resp <- bracketOrderResult{err: err}
				return
			}
			sentIDs = append(sentIDs, item.id)
		}
		resp <- bracketOrderResult{bracket: bracket}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.resolveOrphanedBracket(resp, ctx.Err())
	})
	if err != nil {
		return BracketOrder{}, err
	}
	return out.bracket, out.err
}

func (e *engine) resolveOrphanedBracket(resp <-chan bracketOrderResult, cause error) {
	go func() {
		var out bracketOrderResult
		select {
		case out = <-resp:
		case <-e.done:
			return
		}
		if out.bracket.Parent == nil {
			return
		}
		ids := []int64{out.bracket.Parent.orderID, out.bracket.TakeProfit.orderID, out.bracket.StopLoss.orderID}
		e.enqueue(func() { e.cancelAndCloseOrderRoutes(ids, ids, cause) })
	}()
}

// cancelAndCloseOrderRoutes is the common rollback for partially sent and
// caller-orphaned order groups. It runs on the actor goroutine.
func (e *engine) cancelAndCloseOrderRoutes(sentIDs, allIDs []int64, cause error) {
	if len(sentIDs) > 0 {
		cancelCtx, cancel := context.WithTimeout(context.Background(), orphanCancelTimeout)
		defer cancel()
		for _, orderID := range sentIDs {
			if or, ok := e.orders[orderID]; ok && !or.closed {
				_ = e.sendContext(cancelCtx, codec.CancelOrderRequest{OrderID: orderID})
			}
		}
	}
	for _, orderID := range allIDs {
		if or, ok := e.orders[orderID]; ok {
			e.closeOrderRoute(orderID, or, cause)
		}
	}
}

// bindOrderHandle installs a new order route and its public handle. It must be
// called on the actor goroutine before the corresponding place_order is sent.
func (e *engine) bindOrderHandle(orderID int64, contract Contract) *OrderHandle {
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
		if err := validateOrderRequest(PlaceOrderRequest{Contract: contract, Order: order}, orderIntentModify); err != nil {
			return err
		}
		order = cloneOrder(order)
		return awaitFireAndForget(ctx, e, func(ctx context.Context) error {
			if !e.isReady() {
				return ErrNotReady
			}
			or, ok := e.orders[orderID]
			if !ok || or.closed || or.handle == nil || or.handle.isDone() {
				return ErrClosed
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

// resolveOrphanedPlaceOrder runs after a PlaceOrder caller abandoned the call
// on context cancellation. The setup guarantees resp receives exactly one
// result; draining it in a background goroutine lets the caller return at once.
// If the result carries a handle, the order reached the wire with no owner, so
// the engine best-effort cancels it under a bounded background context and
// tears the route down with the caller's cancellation cause — all inside one
// actor turn, so the cancel only goes out while the route is still live (an
// order that was rejected or filled inside the cancellation window draws no
// spurious cancel_order). The Gateway's acknowledgements for the auto-cancel
// arrive after the route is gone and surface as session events.
func (e *engine) resolveOrphanedPlaceOrder(resp <-chan placeOrderResult, cause error) {
	go func() {
		var out placeOrderResult
		select {
		case out = <-resp:
		case <-e.done:
			return
		}
		if out.handle == nil {
			return
		}
		e.enqueue(func() {
			or, ok := e.orders[out.handle.orderID]
			if !ok || or.closed {
				out.handle.closeWithErr(cause)
				return
			}
			cancelCtx, cancel := context.WithTimeout(context.Background(), orphanCancelTimeout)
			defer cancel()
			_ = e.sendContext(cancelCtx, codec.CancelOrderRequest{OrderID: out.handle.orderID})
			e.closeOrderRoute(out.handle.orderID, or, cause)
		})
	}()
}

// PreviewOrder submits a what-if order and returns the margin-and-commission
// preview the Gateway attaches to the single open_order echo. It forces
// WhatIf=true, so the place_order frame is byte-identical to a what-if
// [engine.PlaceOrder]; the difference is purely in how the reply is consumed.
// No OrderHandle is ever created — the preview route is resolved and torn down
// on the one open_order echo, and nothing rests on the server.
func (e *engine) PreviewOrder(ctx context.Context, req PlaceOrderRequest) (OrderState, error) {
	if err := validateOrderRequest(req, orderIntentPreview); err != nil {
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
		if !e.isReady() {
			setupResp <- setup{err: ErrNotReady}
			return
		}

		orderID := e.allocOrderID()
		orderIDCh <- orderID
		ch := make(chan previewResult, 1)
		e.orders[orderID] = &orderRoute{orderID: orderID, preview: ch}

		// Force the what-if flag; the frame is otherwise the caller's order.
		previewReq := req
		previewReq.Order.WhatIf = new(true)
		if err := e.sendContext(ctx, toCodecPlaceOrder(orderID, previewReq)); err != nil {
			delete(e.orders, orderID)
			setupResp <- setup{err: err}
			return
		}
		setupResp <- setup{ch: ch}
	})

	cleanup := func() {
		select {
		case orderID := <-orderIDCh:
			e.enqueue(func() {
				if or, ok := e.orders[orderID]; ok && or.preview != nil {
					delete(e.orders, orderID)
				}
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
			return orderStateFromOpenOrder(pr.order), nil
		case <-ctx.Done():
			cleanup()
			return OrderState{}, ctx.Err()
		case <-e.done:
			return OrderState{}, e.Wait()
		}
	case <-ctx.Done():
		cleanup()
		return OrderState{}, ctx.Err()
	case <-e.done:
		return OrderState{}, e.Wait()
	}
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

// RefreshOpenOrders re-sends the active open-orders subscription's request.
// The Gateway answers with a fresh snapshot burst: the subscription receives
// the current open orders as Order events followed by another
// SubscriptionSnapshotComplete lifecycle event. The open-orders reply carries
// no request ID on the wire, so a one-shot snapshot cannot coexist with the
// subscription; refresh is the supported way to resync without tearing the
// subscription down.
func (e *engine) RefreshOpenOrders(ctx context.Context) error {
	return awaitFireAndForget(ctx, e, func(ctx context.Context) error {
		if !e.isReady() {
			return ErrNotReady
		}
		route, ok := e.singletons[singletonOpenOrders]
		if !ok {
			return fmt.Errorf("%w: open orders", ErrNoSubscription)
		}
		// The auto scope binds future orders only; the live Gateway sends no
		// open_order_end for req_auto_open_orders, so there is no snapshot
		// to refresh.
		if req, ok := route.request.(codec.OpenOrdersRequest); ok && req.Scope == string(OpenOrdersScopeAuto) {
			return fmt.Errorf("%w: auto-scope open orders", ErrNoSnapshot)
		}
		return e.sendContext(ctx, route.request)
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

const exerciseRouteTTL = 2 * time.Minute

func (e *engine) installExerciseRoute(reqID int) {
	route := &route{
		opKind: OpExerciseOptions,
		handle: func(any, *engine) {},
		handleAPIErr: func(m codec.APIError, e *engine) {
			e.emitSessionEvent(m.Code, m.Message, e.apiErr(OpExerciseOptions, m))
			if isTerminalExerciseNotice(m.Code) {
				e.deleteKeyedRoute(reqID)
			}
		},
		onDisconnect: func(e *engine, err error) bool {
			e.deleteKeyedRoute(reqID)
			return false
		},
		close: func(error) {},
	}
	e.keyed[reqID] = route
	if e.cmds == nil {
		return
	}
	time.AfterFunc(exerciseRouteTTL, func() {
		e.enqueue(func() {
			if e.keyed[reqID] == route {
				e.deleteKeyedRoute(reqID)
			}
		})
	})
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
		// Register a keyed route for the exercise request id. Exercise is
		// fire-and-forget on the wire, but request-id-targeted replies —
		// refusals (322), the TIF-preset acknowledgement (10349), and the 202
		// that cancels a working instruction — must remain observable and must
		// not be mistaken for an unrelated order with the same numeric id.
		// There is no success-end callback, so terminal errors retire the route
		// immediately and a bounded TTL retires quiet/successful instructions.
		e.installExerciseRoute(reqID)
		if err := e.sendContext(ctx, codec.ExerciseOptionsRequest{
			ReqID:            reqID,
			Contract:         toCodecContract(req.Contract),
			ExerciseAction:   int(req.ExerciseAction),
			ExerciseQuantity: req.ExerciseQuantity,
			Account:          req.Account,
			Override:         override,
		}); err != nil {
			e.deleteKeyedRoute(reqID)
			return err
		}
		return nil
	})
}

func isTerminalExerciseNotice(code int) bool {
	return code == ErrCodeServerErrorProcessingRequest || code == ErrCodeOrderCanceled
}

func fromCodecOpenOrder(m codec.OpenOrder) (OpenOrder, error) {
	quantity, err := parseOptionalDecimal(m.Quantity, "open order quantity")
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
	initMarginBefore, err := parseOptionalDecimal(m.InitMarginBefore, "open order init margin before")
	if err != nil {
		return OpenOrder{}, err
	}
	maintMarginBefore, err := parseOptionalDecimal(m.MaintMarginBefore, "open order maint margin before")
	if err != nil {
		return OpenOrder{}, err
	}
	equityWithLoanBefore, err := parseOptionalDecimal(m.EquityWithLoanBefore, "open order equity with loan before")
	if err != nil {
		return OpenOrder{}, err
	}
	initMarginChange, err := parseOptionalDecimal(m.InitMarginChange, "open order init margin change")
	if err != nil {
		return OpenOrder{}, err
	}
	maintMarginChange, err := parseOptionalDecimal(m.MaintMarginChange, "open order maint margin change")
	if err != nil {
		return OpenOrder{}, err
	}
	equityWithLoanChange, err := parseOptionalDecimal(m.EquityWithLoanChange, "open order equity with loan change")
	if err != nil {
		return OpenOrder{}, err
	}
	initMarginAfter, err := parseOptionalDecimal(m.InitMarginAfter, "open order init margin after")
	if err != nil {
		return OpenOrder{}, err
	}
	maintMarginAfter, err := parseOptionalDecimal(m.MaintMarginAfter, "open order maint margin after")
	if err != nil {
		return OpenOrder{}, err
	}
	equityWithLoanAfter, err := parseOptionalDecimal(m.EquityWithLoanAfter, "open order equity with loan after")
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
		WarningText:           m.WarningText,
		Quantity:              quantity,
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
		InitMarginBefore:      initMarginBefore,
		MaintMarginBefore:     maintMarginBefore,
		EquityWithLoanBefore:  equityWithLoanBefore,
		InitMarginChange:      initMarginChange,
		MaintMarginChange:     maintMarginChange,
		EquityWithLoanChange:  equityWithLoanChange,
		InitMarginAfter:       initMarginAfter,
		MaintMarginAfter:      maintMarginAfter,
		EquityWithLoanAfter:   equityWithLoanAfter,
		Commission:            commission,
		MinCommission:         minCommission,
		MaxCommission:         maxCommission,
		CommissionCurrency:    m.CommissionCurrency,
		Partial:               m.Partial,
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
	ts, err := parseExecutionTime(m.Time)
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

// parseExecutionTime handles the Gateway's execution time forms: the UTC
// dash notation ("20260610-19:58:22", observed live 2026-06-10), the
// space-and-zone form ("20260413 13:35:50 US/Eastern"), and RFC3339 (from
// test transcripts).
func parseExecutionTime(raw string) (time.Time, error) {
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts, nil
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
	// Fallback: parse without timezone.
	if len(raw) >= 17 {
		if ts, err := time.Parse("20060102 15:04:05", raw[:17]); err == nil {
			return ts, nil
		}
	}
	return time.Time{}, fmt.Errorf("ibkr: parse execution time %q", raw)
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

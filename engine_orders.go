package ibkr

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
)

// RefreshOrderID asks the Gateway for a fresh next-valid order ID and updates
// the engine's allocation seed before returning it.
func (e *engine) RefreshOrderID(ctx context.Context) (int64, error) {
	type result struct {
		orderID int64
		err     error
	}
	resp := make(chan result, 1)
	enqueueOneShotSetup(ctx, e, func() {
		if _, exists := e.singletons[singletonOrderID]; exists {
			resp <- result{err: fmt.Errorf("ibkr: order ID refresh already in progress")}
			return
		}
		e.singletons[singletonOrderID] = &route{
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
				resp <- result{orderID: m.OrderID}
			},
			handleAPIErr: func(msg codec.APIError, eng *engine) {
				delete(eng.singletons, singletonOrderID)
				resp <- result{err: eng.apiErr(OpOrderID, msg)}
			},
			onDisconnect: func(eng *engine, err error) bool {
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) { resp <- result{err: err} },
		}
		if err := e.sendContext(ctx, codec.ReqIDsRequest{NumIDs: 1}); err != nil {
			delete(e.singletons, singletonOrderID)
			resp <- result{err: err}
		}
	})
	out, err := awaitOneShotResponse(ctx, e, resp, nil)
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
		if err := validateOpenOrdersScope(scope, e.cfg.clientID); err != nil {
			resp <- result{err: err}
			return
		}
		if _, exists := e.singletons[singletonOpenOrders]; exists {
			resp <- result{err: fmt.Errorf("ibkr: open orders subscription already active")}
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
			case codec.OpenOrderEnd:
				if scope != OpenOrdersScopeAuto {
					sub.emitState(SubscriptionStateEvent{Kind: SubscriptionSnapshotComplete, ConnectionSeq: e.connectionSeq()})
				}
			}
		}
		ownedRoute.handleAPIErr = func(m codec.APIError, e *engine) {
			if e.singletons[singletonOpenOrders] != ownedRoute {
				return
			}
			delete(e.singletons, singletonOpenOrders)
			sub.closeWithErr(e.apiErr(OpOpenOrders, m))
		}
		e.singletons[singletonOpenOrders] = ownedRoute

		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.sendContext(ctx, codec.OpenOrdersRequest{Scope: string(scope)}); err != nil {
			delete(e.singletons, singletonOpenOrders)
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

func (e *engine) Executions(ctx context.Context, req ExecutionsRequest) ([]Execution, error) {
	sub, err := e.subscribeExecutions(ctx, req, withSnapshotCollector())
	if err != nil {
		return nil, err
	}
	defer sub.Close()
	return collectSnapshot(ctx, sub, func(execution Execution) (Execution, bool) { return execution, true })
}

func (e *engine) subscribeExecutions(ctx context.Context, req ExecutionsRequest, opts ...SubscriptionOption) (*Subscription[Execution], error) {
	req.SpecificDates = append([]time.Time(nil), req.SpecificDates...)
	type result struct {
		sub *Subscription[Execution]
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
		sub, ownedRoute := newKeyedSubscriptionRoute[Execution](e, cfg, reqID, OpExecutions, nil)
		sub.expectSnapshot()

		ownedRoute.request = wireReq
		ownedRoute.handle = func(msg any, e *engine) {
			switch m := msg.(type) {
			case codec.ExecutionDetail:
				update, err := fromCodecExecution(m)
				if err != nil {
					e.deleteKeyedRoute(reqID)
					sub.closeWithErr(err)
					return
				}
				sub.emit(update)
			case codec.ExecutionsEnd:
				sub.emitState(SubscriptionStateEvent{Kind: SubscriptionSnapshotComplete, ConnectionSeq: e.connectionSeq()})
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(nil)
			}
		}
		e.keyed[reqID] = ownedRoute
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

func (e *engine) CompletedOrders(ctx context.Context, apiOnly bool) ([]CompletedOrderResult, error) {
	type result struct {
		orders []CompletedOrderResult
		err    error
	}
	resp := make(chan result, 1)

	enqueueOneShotSetup(ctx, e, func() {
		if _, exists := e.singletons[singletonCompletedOrders]; exists {
			resp <- result{err: fmt.Errorf("ibkr: completed orders request already in progress")}
			return
		}

		var collected []CompletedOrderResult

		e.singletons[singletonCompletedOrders] = &route{
			opKind: OpCompletedOrders,
			handleAPIErr: func(msg codec.APIError, eng *engine) {
				delete(eng.singletons, singletonCompletedOrders)
				resp <- result{err: eng.apiErr(OpCompletedOrders, msg)}
			},
			handle: func(msg any, eng *engine) {
				switch m := msg.(type) {
				case codec.CompletedOrder:
					order, err := fromCodecCompletedOrder(m)
					if err != nil {
						delete(eng.singletons, singletonCompletedOrders)
						resp <- result{err: err}
						return
					}
					collected = append(collected, order)
				case codec.CompletedOrderEnd:
					delete(eng.singletons, singletonCompletedOrders)
					resp <- result{orders: collected}
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
		if err := e.sendContext(ctx, codec.CompletedOrdersRequest{APIOnly: apiOnly}); err != nil {
			delete(e.singletons, singletonCompletedOrders)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, nil)
	if err != nil {
		return nil, err
	}
	return out.orders, out.err
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
		resp <- placeOrderResult{err: ctx.Err()}
	}, func() {
		if err := validateContractFieldSupport(req.Contract, "place order", e.serverVersion, placeOrderContractFields(e.serverVersion)); err != nil {
			resp <- placeOrderResult{err: err}
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
		resp <- bracketOrderResult{err: ctx.Err()}
	}, func() {
		if err := validateContractFieldSupport(req.Contract, "place bracket", e.serverVersion, placeOrderContractFields(e.serverVersion)); err != nil {
			resp <- bracketOrderResult{err: err}
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
				resp <- bracketOrderResult{err: e.cancelAndCloseOrderRoutes(sentIDs, allIDs, err)}
				return
			}
			sentIDs = append(sentIDs, item.id)
		}
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
			or, ok := e.orders[orderID]
			if !ok || or.closed || or.handle.isDone() {
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

		orderID := e.allocOrderID()
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
			return OrderState{}, ctx.Err()
		case <-e.done:
			return OrderState{}, e.closedOperationError()
		}
	case <-ctx.Done():
		cleanup()
		return OrderState{}, ctx.Err()
	case <-e.done:
		return OrderState{}, e.closedOperationError()
	}
}

// CancelOrder sends a cancel request for the given order ID. This is
// fire-and-forget; the cancellation result arrives via the OrderHandle's
// events channel as an OrderStatus with Status "Cancelled".
func (e *engine) CancelOrder(ctx context.Context, orderID int64, cfg cancelConfig) error {
	return awaitFireAndForget(ctx, e, func(ctx context.Context) error {
		return e.sendContext(ctx, cancelOrderRequest(orderID, cfg))
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
func (e *engine) GlobalCancel(ctx context.Context, cfg cancelConfig) error {
	return awaitFireAndForget(ctx, e, func(ctx context.Context) error {
		req, err := globalCancelRequest(cfg)
		if err != nil {
			return err
		}
		return e.sendContext(ctx, req)
	})
}

const exerciseRouteTTL = 2 * time.Minute

func (e *engine) installExerciseRoute(reqID int) *ExerciseHandle {
	orderHandle := newOrderHandle(int64(reqID), e.cfg.orderEventBuffer)
	handle := &ExerciseHandle{requestID: reqID, order: orderHandle}
	var exerciseRoute *route
	closeExercise := func(err error) {
		if or, ok := e.orders[int64(reqID)]; ok {
			e.closeOrderRoute(int64(reqID), or, err)
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
			closeExercise(ErrInterrupted)
			return false
		},
		close: closeExercise,
	}
	e.keyed[reqID] = exerciseRoute
	e.orders[int64(reqID)] = &orderRoute{orderID: int64(reqID), handle: orderHandle}
	orderHandle.detachFn = func() {
		e.enqueue(func() { closeExercise(nil) })
	}
	if e.cmds == nil {
		return handle
	}
	time.AfterFunc(exerciseRouteTTL, func() {
		e.enqueue(func() {
			if e.keyed[reqID] == exerciseRoute {
				closeExercise(nil)
			}
		})
	})
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
	enqueueReadySetup(ctx, e, func() { resp <- result{err: ctx.Err()} }, func() {
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
	contract, err := fromCodecContract(m.Contract)
	if err != nil {
		return OpenOrder{}, OrderState{}, err
	}
	comboLegPrices, err := comboLegPricesFromCodec(m.OrderComboLegPrices, "open order combo leg price")
	if err != nil {
		return OpenOrder{}, OrderState{}, err
	}
	quantity, err := parseOptionalDecimal(m.Quantity, "open order quantity")
	if err != nil {
		return OpenOrder{}, OrderState{}, err
	}
	lmtPrice, err := parseOptionalDecimal(m.LmtPrice, "open order limit price")
	if err != nil {
		return OpenOrder{}, OrderState{}, err
	}
	auxPrice, err := parseOptionalDecimal(m.AuxPrice, "open order aux price")
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
	origin, err := parseOptionalInt(m.Origin, "open order origin")
	if err != nil {
		return OpenOrder{}, OrderState{}, err
	}
	clientID, err := parseOptionalInt(m.ClientID, "open order client id")
	if err != nil {
		return OpenOrder{}, OrderState{}, err
	}
	permID, err := parseOptionalInt64(m.PermID, "open order perm id")
	if err != nil {
		return OpenOrder{}, OrderState{}, err
	}
	parentID, err := parseOptionalInt64(m.ParentID, "open order parent id")
	if err != nil {
		return OpenOrder{}, OrderState{}, err
	}
	outsideRTH, err := parseOptionalBoolString(m.OutsideRTH, "open order outside rth")
	if err != nil {
		return OpenOrder{}, OrderState{}, err
	}
	hidden, err := parseOptionalBoolString(m.Hidden, "open order hidden")
	if err != nil {
		return OpenOrder{}, OrderState{}, err
	}
	return OpenOrder{
			OrderID:       m.OrderID,
			Account:       m.Account,
			Contract:      contract,
			Action:        OrderAction(m.Action),
			OrderType:     OrderType(m.OrderType),
			Status:        OrderStatus(m.Status),
			WarningText:   m.WarningText,
			Quantity:      quantity,
			LmtPrice:      lmtPrice,
			AuxPrice:      auxPrice,
			TIF:           TimeInForce(m.TIF),
			OcaGroup:      m.OcaGroup,
			OpenClose:     m.OpenClose,
			Origin:        origin,
			OrderRef:      m.OrderRef,
			ClientID:      clientID,
			PermID:        permID,
			OutsideRTH:    outsideRTH,
			Hidden:        hidden,
			GoodAfterTime: m.GoodAfterTime,
			ParentID:      parentID,
			Combo: OrderCombo{
				LegPrices:    comboLegPrices,
				SmartRouting: tagValuesFromCodec(m.SmartComboRouting),
			},
			ComboDescription:      m.ComboLegsDescription,
			AlgoStrategy:          m.AlgoStrategy,
			AlgoParams:            tagValuesFromCodec(m.AlgoParams),
			Conditions:            orderConditionsFromCodec(m.Conditions),
			ConditionsIgnoreRTH:   m.ConditionsIgnoreRTH == "1",
			ConditionsCancelOrder: m.ConditionsCancelOrder == "1",
			Partial:               m.Partial,
		}, OrderState{
			InitMarginBefore:     initMarginBefore,
			MaintMarginBefore:    maintMarginBefore,
			EquityWithLoanBefore: equityWithLoanBefore,
			InitMarginChange:     initMarginChange,
			MaintMarginChange:    maintMarginChange,
			EquityWithLoanChange: equityWithLoanChange,
			InitMarginAfter:      initMarginAfter,
			MaintMarginAfter:     maintMarginAfter,
			EquityWithLoanAfter:  equityWithLoanAfter,
			Commission:           commission,
			CommissionMin:        minCommission,
			CommissionMax:        maxCommission,
			Currency:             m.CommissionCurrency,
			WarningText:          m.WarningText,
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
	clientID, err := parseOptionalInt(m.ClientID, "execution client id")
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
		ClientID:                clientID,
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
	return time.Time{}, fmt.Errorf("ibkr: parse execution time %q", raw)
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
		RealizedPNL:         realized,
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
		return "", fmt.Errorf("ibkr: parse commission and fees yield redemption date %q: %w", raw, err)
	}
	return trimmed, nil
}

package ibkr

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/transport"
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
	singletonManagedAccounts   = "managed_accounts"
	singletonCurrentTime       = "current_time"
	singletonCurrentTimeMillis = "current_time_millis"
	singletonOrderID           = "order_id"
)

func newKeyedOneShotRoute(reqID int, opKind OpKind, handle func(any, *engine), fail func(error)) *route {
	return &route{
		opKind: opKind,
		handle: handle,
		handleAPIErr: func(msg codec.APIError, e *engine) {
			e.deleteKeyedRoute(reqID)
			fail(e.apiErr(opKind, msg))
		},
		onDisconnect: func(*engine, error) bool {
			fail(ErrInterrupted)
			return false
		},
		close: fail,
	}
}

func (e *engine) handleIncoming(msg any) {
	switch m := msg.(type) {
	case codec.ManagedAccounts:
		e.updateSnapshot(func(s *Snapshot) {
			s.ManagedAccounts = append([]string(nil), m.Accounts...)
		})
		e.bootstrap.managed = true
		e.maybeReady()
		if route, ok := e.singletons[singletonManagedAccounts]; ok {
			route.handle(m, e)
		}
		return
	case codec.NextValidID:
		e.observeNextValidID(m.OrderID)
		e.bootstrap.nextValidID = true
		e.maybeReady()
		if route, ok := e.singletons[singletonOrderID]; ok {
			route.handle(m, e)
		}
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
	case codec.CurrentTimeMillis:
		// Same contract as CurrentTime: no reqID, snapshot update plus
		// singleton routing.
		if ts, err := parseEpochMilliseconds(m.TimeMs); err == nil {
			e.updateSnapshot(func(s *Snapshot) {
				s.CurrentTime = ts
			})
		}
		if route, ok := e.singletons[singletonCurrentTimeMillis]; ok {
			route.handle(m, e)
		}
		return
	case codec.APIError:
		e.handleAPIError(m)
		return
	case codec.UnknownInbound:
		// An unmapped msg id used to surface as a ProtocolError and close the
		// transport — a session kill over a message nobody asked for. Keep the
		// session and make the drift observable instead. Report once per
		// distinct msg_id: a hot misdecoded feed must not become per-frame
		// allocations and event spam that evicts genuine session events from
		// the drop-oldest observer, and once is enough to see the drift.
		if _, seen := e.unknownInboundSeen[m.MsgID]; !seen {
			e.unknownInboundSeen[m.MsgID] = struct{}{}
			e.cfg.logger.Warn("ibkr: dropping inbound frames with unknown msg_id",
				"msg_id", m.MsgID, "field_count", len(m.Fields), "binary_body_bytes", len(m.Payload))
			e.emitEvent(0, fmt.Sprintf("dropping inbound frames with unknown msg_id %d (%d fields, %d binary body bytes)", m.MsgID, len(m.Fields), len(m.Payload)))
		}
		return
	}

	if keyed, ok := msg.(codec.ReqIDer); ok {
		if route, found := e.keyed[keyed.RequestID()]; found {
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
	case codec.OrderBound:
		binding := OrderBinding{PermID: msg.PermID, ClientID: msg.ClientID, OrderID: msg.OrderID}
		if or, ok := e.orders[msg.OrderID]; ok && !or.closed {
			or.working = true
			if !e.ensureOrderStarted(or) || !or.handle.emitBinding(binding) {
				e.closeOrderRoute(msg.OrderID, or, nil)
			}
		}
		if route, ok := e.singletons[singletonOpenOrders]; ok {
			if request, auto := route.request.(codec.OpenOrdersRequest); auto && request.Scope == string(OpenOrdersScopeAuto) {
				route.handle(binding, e)
			}
		}
	case codec.CommissionReport:
		for _, route := range e.keyed {
			if route.handleCommission != nil {
				route.handleCommission(msg, e)
			}
		}
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
		e.invalidateReconnectStability()
		apiErr, _ := errors.AsType[*APIError](e.apiErr("", msg))
		e.setState(StateDegraded, msg.Code, msg.Message, nil, apiErr)
		e.emitGap()
		return
	case 1101:
		// Data lost: every subscription and in-flight request died with the
		// Gateway's IB connection. Auto-resumed subscriptions are re-sent by
		// resumeRoutes; everything else is interrupted, mirroring the
		// transport-loss teardown, so callers are not left waiting on
		// answers that are never coming.
		apiErr, _ := errors.AsType[*APIError](e.apiErr("", msg))
		e.setState(StateReady, msg.Code, msg.Message, nil, apiErr)
		e.dropLostRoutes()
		e.resumeRoutes()
		return
	case 1102:
		apiErr, _ := errors.AsType[*APIError](e.apiErr("", msg))
		e.setState(StateReady, msg.Code, msg.Message, nil, apiErr)
		e.emitResumed()
		e.scheduleReconnectStability(e.transport)
		return
	case 1300:
		if e.transport != nil {
			_ = e.transport.Close()
		}
		return
	}

	// These no-request-ID code-321 failures carry stable operation markers in
	// the Gateway's validation text. Route by that live-attested identity, not
	// by whichever singleton happens to be active concurrently.
	if singleton := unkeyedAPIErrorSingleton(msg); singleton != "" {
		if route, ok := e.singletons[singleton]; ok && route.handleAPIErr != nil {
			if singleton == singletonOpenOrders {
				request, ok := route.request.(codec.OpenOrdersRequest)
				if !ok || request.Scope != string(OpenOrdersScopeClient) {
					e.emitAPIEvent(msg)
					return
				}
			}
			route.handleAPIErr(msg, e)
			return
		}
	}

	// Code 2152 is an exact live request-scoped market-depth availability
	// notice. Route it before the otherwise session-scoped 2xxx band so the
	// subscription can preserve its route while publishing the notice.
	if msg.Code == ErrCodeSmartDepthExchanges && msg.ReqID > 0 {
		if route, ok := e.keyed[msg.ReqID]; ok && route.opKind == OpMarketDepth && route.handleAPIErr != nil {
			route.handleAPIErr(msg, e)
			return
		}
		e.emitAPIEvent(msg)
		return
	}

	// Other 2xxx: bootstrap/farm-status informational codes (reqID -1).
	// Emitted as session events for observability; they never target a
	// request or subscription and must not interfere with bootstrap.
	if msg.Code >= 2000 && msg.Code < 3000 {
		e.emitAPIEvent(msg)
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
			if preview, ok := e.previews[int64(msg.ReqID)]; ok {
				// A what-if rejected in the 10xxx band never gets an echo
				// (live-attested: code 10255 on a what-if DarkIce placement,
				// captures/20260705T011725Z), so the order-targeted error is
				// the preview's only completion signal.
				preview.resolve(previewResult{err: e.apiErr(OpPlaceOrder, msg)})
				delete(e.previews, int64(msg.ReqID))
				return
			}
			if or, ok := e.orders[int64(msg.ReqID)]; ok && !or.closed {
				if !e.ensureOrderStarted(or) {
					e.closeOrderRoute(int64(msg.ReqID), or, nil)
					return
				}
				if isOrderCancellationReply(msg.Code) {
					e.emitAPIEvent(msg)
					return
				}
				if !or.working && isInitialOrderRejection(msg.Code) {
					e.closeOrderRoute(int64(msg.ReqID), or, e.apiErr(OpPlaceOrder, msg))
					return
				}
				apiErr, _ := errors.AsType[*APIError](e.apiErr(OpPlaceOrder, msg))
				if !or.handle.emitWarning(apiErr) {
					e.closeOrderRoute(int64(msg.ReqID), or, nil)
				}
				return
			}
		}
		e.emitAPIEvent(msg)
		return
	}

	// Request-specific errors (200, 420, etc.) are routed to the keyed
	// subscription that owns the reqID.
	if msg.ReqID > 0 {
		if route, ok := e.keyed[msg.ReqID]; ok && route.handleAPIErr != nil {
			route.handleAPIErr(msg, e)
			return
		}
		if preview, ok := e.previews[int64(msg.ReqID)]; ok {
			preview.resolve(previewResult{err: e.apiErr(OpPlaceOrder, msg)})
			delete(e.previews, int64(msg.ReqID))
			return
		}
		// Order-specific API errors: the reqID field carries the orderID
		// for order rejections (e.g., code 201 "order rejected").
		if or, ok := e.orders[int64(msg.ReqID)]; ok && !or.closed {
			if !e.ensureOrderStarted(or) {
				e.closeOrderRoute(int64(msg.ReqID), or, nil)
				return
			}
			if isOrderCancellationReply(msg.Code) {
				e.emitAPIEvent(msg)
				return
			}
			orderErr, isAPIErr := errors.AsType[*APIError](e.apiErr(OpPlaceOrder, msg))
			if or.working || !isInitialOrderRejection(msg.Code) {
				if !or.handle.emitWarning(orderErr) {
					e.closeOrderRoute(int64(msg.ReqID), or, nil)
				}
				return
			}
			if isAPIErr {
				e.closeOrderRoute(int64(msg.ReqID), or, orderErr)
			}
			return
		}
		// A reqID-targeted error that matches no keyed route and no order
		// route is surfaced as a session event rather than dropped: it is the
		// only trace of failures on request ids whose observation has ended
		// (for example, a late option-exercise refusal).
		e.emitAPIEvent(msg)
		return
	}

	// An unattributable request-range error with no request ID is still
	// operational evidence. Keep it visible without guessing which concurrent
	// singleton caller owns it.
	e.emitAPIEvent(msg)
}

func unkeyedAPIErrorSingleton(msg codec.APIError) string {
	if msg.ReqID > 0 || msg.Code != 321 {
		return ""
	}
	switch {
	case strings.Contains(msg.Message, "-'b7'") && strings.Contains(msg.Message, "The API interface is currently in Read-Only mode"):
		return singletonOrderID
	case strings.Contains(msg.Message, "-'as'") && strings.Contains(msg.Message, "The API interface is currently in Read-Only mode"):
		return singletonOpenOrders
	case strings.Contains(msg.Message, "-'S'") && strings.Contains(msg.Message, "The API interface is currently in Read-Only mode"):
		return singletonCompletedOrders
	case strings.Contains(msg.Message, "-'b4'") && strings.Contains(msg.Message, "FA data operations ignored for non FA customers"):
		return singletonFA
	default:
		return ""
	}
}

func (e *engine) apiErr(opKind OpKind, msg codec.APIError) error {
	apiErr := &APIError{
		RequestID:               msg.ReqID,
		Code:                    msg.Code,
		Message:                 msg.Message,
		AdvancedOrderRejectJSON: msg.AdvancedOrderRejectJSON,
		OpKind:                  opKind,
		ConnectionSeq:           e.connectionSeq(),
	}
	if msg.ErrorTimeMs != "" {
		if timestamp, err := parseEpochMilliseconds(msg.ErrorTimeMs); err == nil {
			apiErr.ServerTime = timestamp
		}
	}
	return apiErr
}

func isOrderCancellationReply(code int) bool {
	return code == ErrCodeCancelNotCancellableState || code == ErrCodeOrderCanceled ||
		code == ErrCodeOrderToCancelNotFound || code == ErrCodeOrderCannotBeCancelled ||
		code == ErrCodeImbalanceOnlyNotAllowed
}

// isInitialOrderRejection is the attested set of errors that prove a placement
// failed before the Gateway exposed any working-order evidence. Unknown codes
// and every error after working evidence remain warnings: detaching a live
// order is the dangerous failure direction.
func isInitialOrderRejection(code int) bool {
	switch code {
	case ErrCodeNoSecurityDefinition, ErrCodeOrderRejected,
		ErrCodeServerErrorReadingRequest, ErrCodeServerErrorValidatingRequest,
		ErrCodeTrailingStopAttachRejected, ErrCodeUnsupportedOrderType,
		ErrCodeAlgoDefinitionNotFound, ErrCodeUnknownAlgoAttribute,
		ErrCodeInvalidFXHedgeOrder, ErrCodeDisplaySizeNotAllowed:
		return true
	}
	return false
}

func (e *engine) dispatchObservedOpenOrder(msg codec.OpenOrder) {
	e.observeOrderID(msg.OrderID)
	// A what-if preview route resolves on its single open_order echo: decode,
	// tear the route down, and hand the result (OpenOrder or decode error) to
	// the blocked PreviewOrder caller. No OrderHandle is ever involved.
	if preview, ok := e.previews[msg.OrderID]; ok {
		delete(e.previews, msg.OrderID)
		_, state, err := decodeCodecOpenOrder(msg)
		preview.resolve(previewResult{state: state, err: err})
		return
	}

	orderRoute, orderObserved := e.orders[msg.OrderID]
	singletonRoute, singletonObserved := e.singletons[singletonOpenOrders]
	if (!orderObserved || orderRoute.closed) && !singletonObserved {
		return
	}

	order, err := fromCodecOpenOrder(msg)
	if err != nil {
		if orderObserved && !orderRoute.closed {
			e.closeOrderRoute(msg.OrderID, orderRoute, err)
		}
		if singletonObserved {
			delete(e.singletons, singletonOpenOrders)
			singletonRoute.close(err)
		}
		return
	}

	if orderObserved && !orderRoute.closed {
		if !e.ensureOrderStarted(orderRoute) {
			e.closeOrderRoute(msg.OrderID, orderRoute, nil)
			return
		}
		if order.Status != OrderStatusInactive && order.Status != OrderStatusApiCancelled {
			orderRoute.working = true
		}
		if !orderRoute.handle.emitOrder(cloneOpenOrder(order)) {
			e.closeOrderRoute(msg.OrderID, orderRoute, nil)
		}
	}
	if singletonObserved {
		singletonRoute.handle(cloneOpenOrder(order), e)
	}
}

func (e *engine) dispatchObservedOrderStatus(msg codec.OrderStatus) {
	e.observeOrderID(msg.OrderID)
	orderRoute, orderObserved := e.orders[msg.OrderID]
	singletonRoute, singletonObserved := e.singletons[singletonOpenOrders]
	if (!orderObserved || orderRoute.closed) && !singletonObserved {
		return
	}

	status, err := fromCodecOrderStatus(msg)
	if err != nil {
		if orderObserved && !orderRoute.closed {
			e.closeOrderRoute(msg.OrderID, orderRoute, err)
		}
		if singletonObserved {
			delete(e.singletons, singletonOpenOrders)
			singletonRoute.close(err)
		}
		return
	}

	if orderObserved && !orderRoute.closed {
		if !e.ensureOrderStarted(orderRoute) {
			e.closeOrderRoute(msg.OrderID, orderRoute, nil)
			return
		}
		if status.Status != OrderStatusInactive && status.Status != OrderStatusApiCancelled {
			orderRoute.working = true
		}
		if !orderRoute.handle.emitStatus(status) {
			e.closeOrderRoute(msg.OrderID, orderRoute, nil)
		}
	}
	if singletonObserved {
		singletonRoute.handle(status, e)
	}
}

// closeOrderRoute finishes an order route outside the terminal drain window:
// rejection, decode-failure, and slow-consumer paths where no further
// legitimate traffic can reach the handle. It closes the handle (idempotent),
// drops the route, and forgets the order's execution correlations — without
// this, rejected orders accumulated routes for the connection lifetime the
// same way filled ones once did. Frames that straggle in after the deletion
// drop at the missing-route check, identical to the closed-route behavior.
func (e *engine) closeOrderRoute(orderID int64, or *orderRoute, err error) {
	or.closed = true
	if or.pendingWrite.id != 0 {
		delete(e.pendingOrderWrites, or.pendingWrite)
		or.pendingWrite = transportWriteKey{}
	}
	or.handle.closeWithErr(err)
	delete(e.orders, orderID)
	e.forgetOrderExecutions(orderID)
}

func (e *engine) handleTransportWrite(write transportWrite) {
	key := transportWriteKey{transport: write.transport, id: write.result.ID}
	orderID, ok := e.pendingOrderWrites[key]
	if !ok {
		return
	}
	delete(e.pendingOrderWrites, key)
	or, ok := e.orders[orderID]
	if !ok || or.closed || or.pendingWrite != key {
		return
	}
	or.pendingWrite = transportWriteKey{}

	switch write.result.Outcome {
	case transport.WriteCompleteLocal:
		or.handle.emitLifecycle(OrderStarted, e.connectionSeq(), nil)
	case transport.WriteUnwritten:
		e.closeOrderRoute(orderID, or, ErrInterrupted)
	case transport.WriteIncomplete:
		// The Gateway may have observed a partial frame. The transport loss
		// that follows keeps the route alive but marks its state uncertain.
	}
}

func (e *engine) ensureOrderStarted(or *orderRoute) bool {
	if or.pendingWrite.id == 0 {
		return true
	}
	delete(e.pendingOrderWrites, or.pendingWrite)
	or.pendingWrite = transportWriteKey{}
	return or.handle.emitLifecycle(OrderStarted, e.connectionSeq(), nil)
}

const unclaimedCommissionTTL = 750 * time.Millisecond

// execDelivery is the order-handle leg's delivery record for one ExecID.
// See the engine.execDeliveries field comment for the full contract.
type execDelivery struct {
	orderID   int64
	delivered *codec.CommissionReport
	pending   []codec.CommissionReport
}

// forgetOrderExecutions drops every execution correlation owned by orderID.
// It runs once per terminal order (after the drain window), so the linear
// scan is bounded by the session's fills. Pending-only entries (orderID 0)
// are not touched; their eviction timer owns them.
func (e *engine) forgetOrderExecutions(orderID int64) {
	for execID, st := range e.execDeliveries {
		if st.orderID == orderID {
			delete(e.execDeliveries, execID)
		}
	}
}

// scheduleUnclaimedExecEviction drops a pending-only delivery record that no
// execution detail claimed within the drain window. Commissions for fills
// owned by other clients (or orders this client never tracked) arrive with no
// claiming execution, so without the timer they would accumulate for the
// connection lifetime.
func (e *engine) scheduleUnclaimedExecEviction(execID string) {
	time.AfterFunc(unclaimedCommissionTTL, func() {
		e.enqueue(func() {
			if st, ok := e.execDeliveries[execID]; ok && st.orderID == 0 {
				delete(e.execDeliveries, execID)
			}
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
	if _, ok := e.keyed[reqID]; !ok {
		return
	}
	delete(e.keyed, reqID)
}

func (e *engine) routeCommissionReport(report codec.CommissionReport) {
	st, ok := e.execDeliveries[report.ExecID]
	if !ok {
		// No execution detail has claimed this ExecID yet: the Gateway can
		// send the commission ahead of the execution (the keyed leg buffers
		// the same race in the correlator). Buffer it for the claim; the
		// eviction timer reclaims entries no execution ever claims.
		st = &execDelivery{pending: []codec.CommissionReport{report}}
		e.execDeliveries[report.ExecID] = st
		e.scheduleUnclaimedExecEviction(report.ExecID)
		return
	}
	if st.orderID == 0 {
		st.pending = append(st.pending, report)
		return
	}
	e.deliverCommissionToOrder(st, report)
}

// deliverCommissionToOrder emits one commission report to the handle that owns
// the execution. An identical re-send (an Executions() snapshot replaying a
// commission the handle already saw live) is deduped; a re-send with changed
// content (e.g. a realizedPNL update) goes through and becomes the new
// delivered record. The order is live on the server, so a decode failure must
// not tear down the handle — drop the event and log so the problem is
// observable.
func (e *engine) deliverCommissionToOrder(st *execDelivery, report codec.CommissionReport) {
	if st.delivered != nil && *st.delivered == report {
		return
	}
	or, ok := e.orders[st.orderID]
	if !ok || or.closed {
		return
	}
	or.working = true
	cr, err := fromCodecCommission(report)
	if err != nil {
		e.cfg.logger.Warn("ibkr: drop commission report on decode error",
			"order_id", st.orderID, "exec_id", report.ExecID, "err", err)
		return
	}
	if !or.handle.emitCommissionAndFees(cr) {
		e.closeOrderRoute(st.orderID, or, nil)
		return
	}
	st.delivered = &report
}

func (e *engine) dispatchExecutionToOrder(m codec.ExecutionDetail) {
	if m.OrderID == 0 {
		return
	}
	or, ok := e.orders[m.OrderID]
	if !ok || or.closed {
		return
	}
	or.working = true
	if !e.ensureOrderStarted(or) {
		e.closeOrderRoute(m.OrderID, or, nil)
		return
	}
	// A fill already delivered to this handle must not be re-emitted when a
	// later Executions() snapshot query replays the same ExecID. A claimed
	// delivery record (orderID set) marks exactly the fills the handle has
	// seen; the keyed subscription leg (dispatched separately) is untouched.
	st := e.execDeliveries[m.ExecID]
	if st != nil && st.orderID != 0 {
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
	if !or.handle.emitExecution(exec) {
		e.closeOrderRoute(m.OrderID, or, nil)
		return
	}
	if st == nil {
		st = &execDelivery{}
		e.execDeliveries[m.ExecID] = st
	}
	st.orderID = m.OrderID
	// Flush any commissions that raced ahead of this execution: without the
	// claim they had no owning handle and would otherwise be lost.
	pending := st.pending
	st.pending = nil
	for _, buffered := range pending {
		e.deliverCommissionToOrder(st, buffered)
	}
}

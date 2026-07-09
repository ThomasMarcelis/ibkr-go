package ibkr

import (
	"errors"
	"fmt"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
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
	singletonCurrentTimeMillis = "current_time_millis"
	singletonOrderID           = "order_id"
)

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
				"msg_id", m.MsgID, "field_count", len(m.Fields))
			e.emitEvent(0, fmt.Sprintf("dropping inbound frames with unknown msg_id %d (%d fields)", m.MsgID, len(m.Fields)))
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
		// Data lost: every subscription and in-flight request died with the
		// Gateway's IB connection. Auto-resumed subscriptions are re-sent by
		// resumeRoutes; everything else is interrupted, mirroring the
		// transport-loss teardown, so callers are not left waiting on
		// answers that are never coming.
		e.setState(StateReady, msg.Code, msg.Message, nil)
		e.dropLostRoutes()
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

	// reqIds has no request ID. The read-only Gateway's live-attested failure
	// is req_id=-1/code 321, so it can only be attributed while the singleton
	// refresh is active.
	if msg.ReqID <= 0 && msg.Code == 321 {
		if route, ok := e.singletons[singletonOrderID]; ok && route.handleAPIErr != nil {
			route.handleAPIErr(msg, e)
			return
		}
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
			if or, ok := e.orders[int64(msg.ReqID)]; ok && !or.closed {
				// A what-if rejected in the 10xxx band never gets an echo
				// (live-attested: code 10255 on a what-if DarkIce placement,
				// captures/20260705T011725Z), so the order-targeted error is
				// the preview's only completion signal.
				if or.resolvePreview(previewResult{err: e.apiErr(OpPlaceOrder, msg)}) {
					delete(e.orders, int64(msg.ReqID))
					return
				}
				// Live handles terminate only on 10xxx codes attested as
				// outright placement rejections: no order_status ever follows
				// them, so this error is the handle's only completion signal.
				// Order-targeted notices (10147/10148 cancel replies) stay
				// session events; the handle already holds its real state.
				if isOrderPlacementRejection(msg.Code) {
					e.closeOrderRoute(int64(msg.ReqID), or, e.apiErr(OpPlaceOrder, msg))
					return
				}
			}
		}
		e.emitEvent(msg.Code, msg.Message)
		return
	}

	// Request-specific errors (200, 420, etc.) are routed to the keyed
	// subscription that owns the reqID.
	if msg.ReqID > 0 {
		if route, ok := e.keyed[msg.ReqID]; ok && route.handleAPIErr != nil {
			route.handleAPIErr(msg, e)
			return
		}
		// Order-specific API errors: the reqID field carries the orderID
		// for order rejections (e.g., code 201 "order rejected").
		if or, ok := e.orders[int64(msg.ReqID)]; ok && !or.closed {
			if isOrderCancellationNotice(msg) {
				e.emitEvent(msg.Code, msg.Message)
				return
			}
			// The gateway rejecting a what-if order is Preview's ordinary
			// failure mode; resolve the blocked caller instead of touching
			// the handle no preview route has.
			if or.resolvePreview(previewResult{err: e.apiErr(OpPlaceOrder, msg)}) {
				delete(e.orders, int64(msg.ReqID))
				return
			}
			// A warning targeting a live order (e.g. code 399, the off-hours
			// deferral) leaves the order working at IB and still cancellable,
			// so it is delivered non-terminally and the handle stays open.
			// Only genuine failures close the handle.
			orderErr, isAPIErr := errors.AsType[*APIError](e.apiErr(OpPlaceOrder, msg))
			if isAPIErr && orderErr.IsWarning() {
				if !or.handle.emitWarning(orderErr) {
					e.closeOrderRoute(int64(msg.ReqID), or, nil)
				}
				return
			}
			// A terminal rejection ends the order at the Gateway; no further
			// traffic for this id is legitimate, so the route goes with it.
			e.closeOrderRoute(int64(msg.ReqID), or, orderErr)
			return
		}
		// A reqID-targeted error that matches no keyed route and no order
		// route is surfaced as a session event rather than dropped: it is the
		// only trace of failures on fire-and-forget request ids (e.g. an
		// option-exercise refusal on an id whose route is already gone).
		e.emitEvent(msg.Code, msg.Message)
		return
	}
}

func (e *engine) apiErr(opKind OpKind, msg codec.APIError) error {
	return &APIError{
		Code:          msg.Code,
		Message:       msg.Message,
		OpKind:        opKind,
		ConnectionSeq: e.connectionSeq(),
	}
}

func isOrderCancellationNotice(msg codec.APIError) bool {
	return msg.Code == 202
}

// isOrderPlacementRejection reports whether a 10xxx code is live-attested as
// an outright placement rejection: the Gateway discards the order and never
// sends an order_status for it, so the order-targeted api_error is the only
// signal the handle will ever receive. The set grows as live captures attest
// new codes; unattested 10xxx codes conservatively stay session events
// because closing a handle on a notice for a live order would detach it.
// ErrCodeImbalanceOnlyNotAllowed (10342) is deliberately absent: its attested
// context is a reply to cancelling a silently accepted resting order.
func isOrderPlacementRejection(code int) bool {
	switch code {
	case ErrCodeInvalidFXHedgeOrder, ErrCodeDisplaySizeNotAllowed:
		return true
	}
	return false
}

func (e *engine) dispatchObservedOpenOrder(msg codec.OpenOrder) {
	orderRoute, orderObserved := e.orders[msg.OrderID]

	// A what-if preview route resolves on its single open_order echo: decode,
	// tear the route down, and hand the result (OpenOrder or decode error) to
	// the blocked PreviewOrder caller. No OrderHandle is ever involved.
	if orderObserved && orderRoute.preview != nil {
		if orderRoute.closed {
			return
		}
		orderRoute.closed = true
		delete(e.orders, msg.OrderID)
		order, err := fromCodecOpenOrder(msg)
		orderRoute.preview <- previewResult{order: order, err: err}
		return
	}

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
		if !orderRoute.handle.emitOrder(order) {
			e.closeOrderRoute(msg.OrderID, orderRoute, nil)
		}
	}
	if singletonObserved {
		singletonRoute.handle(parsedOpenOrder{order: order}, e)
	}
}

func (e *engine) dispatchObservedOrderStatus(msg codec.OrderStatus) {
	orderRoute, orderObserved := e.orders[msg.OrderID]
	// A what-if preview route has no handle and resolves only on its open_order
	// echo; a live what-if never emits order status, so ignore the order side.
	if orderObserved && orderRoute.preview != nil {
		orderObserved = false
	}
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
		if !orderRoute.handle.emitStatus(status) {
			e.closeOrderRoute(msg.OrderID, orderRoute, nil)
		} else if IsTerminalOrderStatus(status.Status) {
			e.scheduleTerminalOrderClose(msg.OrderID, orderRoute)
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
	if or.handle != nil {
		or.handle.closeWithErr(err)
	}
	delete(e.orders, orderID)
	e.forgetOrderExecutions(orderID)
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
			// The drain window has elapsed and the handle is closing: drop the
			// route and every execution correlation it owns. Retaining them
			// only bounded the maps by connection lifetime. Deleting the
			// execToOrder entries here is safe precisely because the route is
			// gone — dispatchExecutionToOrder and routeCommissionReport both
			// early-return on a missing route, and any commission that could
			// still legitimately arrive for this order is exactly what the
			// drain window existed to absorb; anything later is post-terminal
			// noise the closed handle would drop anyway.
			delete(e.orders, orderID)
			e.forgetOrderExecutions(orderID)
		})
	})
}

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
	time.AfterFunc(orderTerminalDrainWindow, func() {
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
	// Also dispatch to the order handle that owns this execution.
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
	cr, err := fromCodecCommission(report)
	if err != nil {
		e.cfg.logger.Warn("ibkr: drop commission report on decode error",
			"order_id", st.orderID, "exec_id", report.ExecID, "err", err)
		return
	}
	if or.handle == nil {
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
	if or.handle == nil {
		return
	}
	if !or.handle.emitExecution(*exec.Execution) {
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
		if !emitSubscription(sub, ExecutionUpdate{CommissionAndFees: &report}) {
			return false
		}
	}
	return true
}

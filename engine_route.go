package ibkr

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
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
		onDisconnect: func(_ *engine, err error) bool {
			fail(interrupted(err))
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
		if m.OrderID <= 0 || m.OrderID > maxWireOrderID {
			err := &ProtocolError{
				Direction: "inbound",
				Message:   "next valid order ID",
				Err:       fmt.Errorf("value %d is outside the signed 32-bit order-ID range", m.OrderID),
			}
			if route, ok := e.singletons[singletonOrderID]; ok {
				delete(e.singletons, singletonOrderID)
				route.close(err)
			}
			e.retireTransport(err)
			return
		}
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
	case codec.ExecutionDetail:
		e.emitExecutionDetailEvent(m)
	case codec.CommissionReport:
		e.emitCommissionEvent(m)
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
			if msgID, expected, valid := marketDataCallbackOwner(msg, route.opKind); !valid {
				e.poisonedGeneration = e.transportGeneration
				e.handleMalformedInbound(codec.MalformedInbound{
					MsgID: msgID,
					Err: fmt.Errorf(
						"request id %d belongs to %s, but the callback belongs to %s",
						keyed.RequestID(), route.opKind, expected,
					),
				})
				return
			}
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
		binding := OrderBinding{PermID: msg.PermID, ClientID: protocolIDFromInt[ClientID](msg.ClientID), OrderID: msg.OrderID}
		if or, ok := e.orders[msg.OrderID]; ok && !or.closed && e.claimOrderCallback(or, msg.ClientID, msg.PermID) {
			or.working = true
			if !e.ensureOrderStarted(or) || !or.handle.emitBinding(binding) {
				e.closeOrderRoute(msg.OrderID, or, ErrSlowConsumer)
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

// marketDataCallbackOwner validates the two request-scoped callback families
// whose stateful handlers cannot safely ignore each other's rows. A positive
// request ID naming another active operation is protocol corruption, not a
// late callback: accepting it would leave that operation open with missing
// data and could let a later end marker report a partial result as complete.
func marketDataCallbackOwner(msg any, owner OpKind) (msgID int, expected OpKind, valid bool) {
	switch msg.(type) {
	case codec.MarketDepthUpdate:
		return protocol.InMarketDepth, OpMarketDepth, owner == OpMarketDepth
	case codec.MarketDepthL2Update:
		return protocol.InMarketDepthL2, OpMarketDepth, owner == OpMarketDepth
	case codec.MarketDepthReroute:
		return protocol.InMarketDepthReroute, OpMarketDepth, owner == OpMarketDepth
	case codec.TickOptionComputation:
		return protocol.InTickOptionComputation, OpQuotes,
			owner == OpQuotes || owner == OpCalcImpliedVol || owner == OpCalcOptionPrice
	case codec.TickPrice:
		return protocol.InTickPrice, OpQuotes, owner == OpQuotes
	case codec.TickSize:
		return protocol.InTickSize, OpQuotes, owner == OpQuotes
	case codec.TickGeneric:
		return protocol.InTickGeneric, OpQuotes, owner == OpQuotes
	case codec.TickString:
		return protocol.InTickString, OpQuotes, owner == OpQuotes
	case codec.TickEFP:
		return protocol.InTickEFP, OpQuotes, owner == OpQuotes
	case codec.DeltaNeutralValidation:
		return protocol.InDeltaNeutralValidation, OpQuotes, owner == OpQuotes
	case codec.TickSnapshotEnd:
		return protocol.InTickSnapshotEnd, OpQuotes, owner == OpQuotes
	case codec.MarketDataType:
		return protocol.InMarketDataType, OpQuotes, owner == OpQuotes
	case codec.TickReqParams:
		return protocol.InTickReqParams, OpQuotes, owner == OpQuotes
	case codec.TickNews:
		return protocol.InTickNews, OpQuotes, owner == OpQuotes
	case codec.MarketDataReroute:
		return protocol.InMarketDataReroute, OpQuotes, owner == OpQuotes
	default:
		return 0, "", true
	}
}

func (e *engine) handleMalformedInbound(m codec.MalformedInbound) {
	protocolErr := &ProtocolError{
		Direction: "inbound",
		Message:   fmt.Sprintf("msg_id %d", m.MsgID),
		Err:       m.Err,
	}
	if _, seen := e.malformedInboundSeen[m.MsgID]; !seen {
		e.malformedInboundSeen[m.MsgID] = struct{}{}
		e.cfg.logger.Warn("ibkr: dropping malformed inbound frame and retiring transport generation",
			"msg_id", m.MsgID, "field_count", len(m.Fields), "binary_body_bytes", len(m.Payload), "error", m.Err)
		e.emitSessionEvent(0, fmt.Sprintf("dropping malformed inbound frame with msg_id %d", m.MsgID), protocolErr)
	}
	// A registered decoder no longer has a trustworthy semantic boundary.
	// Interrupt every route on this generation and retain the protocol cause so
	// consumers cannot classify a corrupt partial snapshot as safely retryable.
	e.retireTransport(interrupted(protocolErr))
}

func (e *engine) handleAPIError(msg codec.APIError) {
	// Connectivity codes drive session state transitions.
	switch msg.Code {
	case ErrCodeConnectivityLost:
		e.invalidateReconnectStability()
		apiErr := e.apiErr("", msg)
		e.setState(StateDegraded, msg.Code, msg.Message, nil, apiErr)
		e.emitGap()
		return
	case ErrCodeConnectivityRestoredDataLost:
		// Data lost: every subscription and in-flight request died with the
		// Gateway's IB connection. Auto-resumed subscriptions are re-sent by
		// resumeRoutes; everything else is interrupted, mirroring the
		// transport-loss teardown, so callers are not left waiting on
		// answers that are never coming.
		apiErr := e.apiErr("", msg)
		e.setState(StateReady, msg.Code, msg.Message, nil, apiErr)
		e.emitGap()
		e.restoreExecutionEvents()
		e.requireOrderRecovery(e.connectionSeq())
		e.dropLostRoutes()
		e.resumeRoutes()
		return
	case ErrCodeConnectivityRestoredDataMaintained:
		apiErr := e.apiErr("", msg)
		e.setState(StateReady, msg.Code, msg.Message, nil, apiErr)
		e.emitResumed()
		e.scheduleReconnectStability(e.transport)
		return
	case 1300:
		// Official code 1300 is a socket-port reset; transport teardown owns it.
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

	// Unkeyed 2xxx bootstrap/farm-status informational codes are session
	// events. A positive request ID remains authoritative and follows the
	// ordinary keyed/preview/order routing below.
	if msg.Code >= 2000 && msg.Code < 3000 && msg.ReqID <= 0 {
		e.emitAPIEvent(msg)
		return
	}

	// Cancellation replies use the order-ID namespace. They can arrive after
	// an order route and even its connection are gone, when the same number is
	// already serving an unrelated request ID. Keep them out of keyed routes;
	// order status remains the handle's lifecycle signal and the API notice is
	// session-level evidence.
	if msg.ReqID > 0 && isOrderCancellationReply(msg.Code) {
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
		if e.handlePreviewAPIError(msg) {
			return
		}
		// Order-specific API errors: the reqID field carries the orderID
		// for order rejections (e.g., code 201 "order rejected").
		if or, ok := e.orders[int64(msg.ReqID)]; ok && !or.closed {
			if !e.ensureOrderStarted(or) {
				e.closeOrderRoute(int64(msg.ReqID), or, ErrSlowConsumer)
				return
			}
			orderErr := e.apiErr(OpPlaceOrder, msg)
			// Once the Gateway has exposed working evidence, retaining the live
			// handle and surfacing an unknown error is safer than detaching it.
			if or.working || !isOrderRejectionCode(msg.Code) {
				if !or.handle.emitWarning(orderErr) {
					e.closeOrderRoute(int64(msg.ReqID), or, ErrSlowConsumer)
				}
				return
			}
			e.closeOrderRoute(int64(msg.ReqID), or, orderErr)
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
	if msg.ReqID > 0 || msg.Code != ErrCodeServerErrorValidatingRequest {
		return ""
	}
	switch {
	case strings.Contains(msg.Message, "-'aa'") && strings.Contains(msg.Message, "The API interface is currently in Read-Only mode"):
		return singletonOrderID
	case strings.Contains(msg.Message, "-'as'") && strings.Contains(msg.Message, "The API interface is currently in Read-Only mode"):
		return singletonOpenOrders
	case strings.Contains(msg.Message, "-'S'") && strings.Contains(msg.Message, "The API interface is currently in Read-Only mode"):
		return singletonCompletedOrders
	// The operation marker changed from b4 to X when RequestFA moved to
	// protobuf at server version 211. The cause text is operation-specific and
	// therefore remains the stable identity across both wire encodings.
	case strings.Contains(msg.Message, "FA data operations ignored for non FA customers"):
		return singletonFA
	default:
		return ""
	}
}

func (e *engine) apiErr(opKind OpKind, msg codec.APIError) *APIError {
	apiErr := &APIError{
		RequestID:               protocolIDFromInt[RequestID](msg.ReqID),
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
		code == ErrCodeOrderToCancelNotFound || code == ErrCodeOrderCannotBeCancelled
}

func (e *engine) handlePreviewAPIError(msg codec.APIError) bool {
	preview, ok := e.previews[int64(msg.ReqID)]
	if !ok {
		return false
	}
	apiErr := e.apiErr(OpPlaceOrder, msg)
	if apiErr.IsWarning() {
		e.emitAPIEvent(msg)
		return true
	}
	// Rejected what-if requests do not get an open-order echo (sv225 capture
	// 20260824T210426Z-api_whatif_margin_aapl, events SHA-256
	// 686932b92e69fcf4030a9637d82117682be8c76d1039819d048ee58272ae81ee),
	// so the targeted error is the preview's only completion signal.
	preview.resolve(previewResult{err: apiErr})
	delete(e.previews, int64(msg.ReqID))
	return true
}

func (e *engine) dispatchObservedOpenOrder(msg codec.OpenOrder) {
	e.observeOrderID(msg.OrderID)
	// A what-if preview route resolves on its single open_order echo: decode,
	// tear the route down, and hand the result (OpenOrder or decode error) to
	// the blocked PreviewOrder caller. No OrderHandle is ever involved.
	if preview, ok := e.previews[msg.OrderID]; ok && msg.WhatIf == "1" {
		delete(e.previews, msg.OrderID)
		_, state, err := decodeCodecOpenOrder(msg)
		preview.resolve(previewResult{state: state, err: err})
		return
	}
	if msg.WhatIf == "1" {
		return
	}

	orderRoute, orderObserved := e.orders[msg.OrderID]
	singletonRoute, singletonObserved := e.singletons[singletonOpenOrders]
	orderAttributed := orderObserved && !orderRoute.closed && e.claimOrderCallbackStrings(orderRoute, msg.ClientID, msg.PermID)
	if !orderAttributed && !singletonObserved {
		return
	}

	order, err := fromCodecOpenOrder(msg)
	if err != nil {
		if orderAttributed {
			e.closeOrderRoute(msg.OrderID, orderRoute, err)
		}
		if singletonObserved {
			singletonRoute.cancel(err)
		}
		return
	}

	if orderAttributed {
		orderRoute.handle.acknowledge()
		if !e.ensureOrderStarted(orderRoute) {
			e.closeOrderRoute(msg.OrderID, orderRoute, ErrSlowConsumer)
		} else {
			if order.State.Status != OrderStatusInactive && order.State.Status != OrderStatusAPICancelled {
				orderRoute.working = true
			}
			if !orderRoute.handle.emitOrder(cloneOpenOrder(order)) {
				e.closeOrderRoute(msg.OrderID, orderRoute, ErrSlowConsumer)
			}
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
	orderAttributed := orderObserved && !orderRoute.closed && e.claimOrderCallbackStrings(orderRoute, msg.ClientID, msg.PermID)
	if !orderAttributed && !singletonObserved {
		return
	}

	status, err := fromCodecOrderStatus(msg)
	if err != nil {
		if orderAttributed {
			e.closeOrderRoute(msg.OrderID, orderRoute, err)
		}
		if singletonObserved {
			singletonRoute.cancel(err)
		}
		return
	}

	if orderAttributed {
		orderRoute.handle.acknowledge()
		if !e.ensureOrderStarted(orderRoute) {
			e.closeOrderRoute(msg.OrderID, orderRoute, ErrSlowConsumer)
		} else {
			if status.Status != OrderStatusInactive && status.Status != OrderStatusAPICancelled {
				orderRoute.working = true
			}
			if !orderRoute.handle.emitStatus(status) {
				e.closeOrderRoute(msg.OrderID, orderRoute, ErrSlowConsumer)
			}
		}
	}
	if singletonObserved {
		singletonRoute.handle(status, e)
	}
}

// closeOrderRoute ends local observation and releases its routing state.
// It never cancels the order at IBKR.
func (e *engine) closeOrderRoute(orderID int64, or *orderRoute, err error) {
	or.closed = true
	if or.pendingWrite.id != 0 {
		delete(e.pendingOrderWrites, or.pendingWrite)
		or.pendingWrite = transportWriteKey{}
	}
	or.handle.closeWithErr(err)
	if e.orders[orderID] == or {
		delete(e.orders, orderID)
		e.forgetOrderExecutions(orderID)
	}
	if or.cleanup != nil {
		cleanup := or.cleanup
		or.cleanup = nil
		cleanup()
	}
}

func (e *engine) handleTransportWrite(key transportWriteKey, outcome transport.WriteOutcome) {
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

	switch outcome {
	case transport.WriteCompleteLocal:
		if !or.handle.emitLifecycle(OrderStarted, e.connectionSeq(), nil) {
			e.closeOrderRoute(orderID, or, ErrSlowConsumer)
		}
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

// execDelivery is the order-handle leg's delivery record for one ExecID.
// See the engine.execDeliveries field comment for the full contract.
type execDelivery struct {
	orderID   int64
	delivered *codec.CommissionReport
	pending   []codec.CommissionReport
}

// forgetOrderExecutions releases claimed entries with their handle. Unmatched
// fees can still belong to another active handle, so retain them until the
// last handle closes. The scan is bounded by the configured correlation limit.
func (e *engine) forgetOrderExecutions(orderID int64) {
	if len(e.orders) == 0 {
		e.clearOrderCorrelations()
		return
	}
	for execID, st := range e.execDeliveries {
		if st.orderID == orderID {
			e.excludeOrderExecution(execID)
		}
	}
}

func (e *engine) clearOrderCorrelations() {
	e.execDeliveries = make(map[string]*execDelivery)
	e.pendingOrderFees = 0
	e.excludedOrderExecutions = nil
	e.excludedOrderExecFIFO = nil
	e.excludedOrderExecNext = 0
}

func (e *engine) excludeOrderExecution(execID string) {
	if len(e.orders) == 0 || execID == "" {
		return
	}
	if st := e.execDeliveries[execID]; st != nil {
		if owner := e.orders[st.orderID]; owner != nil && !owner.closed {
			return // An existing positive claim outranks contradictory callbacks.
		}
		e.pendingOrderFees -= len(st.pending)
		delete(e.execDeliveries, execID)
	}
	if _, exists := e.excludedOrderExecutions[execID]; exists {
		return
	}
	limit := e.cfg.orderExecutionCorrelationLimit
	if limit <= 0 {
		return
	}
	if e.excludedOrderExecutions == nil {
		e.excludedOrderExecutions = make(map[string]int)
	}
	slot := len(e.excludedOrderExecFIFO)
	if slot < limit {
		e.excludedOrderExecFIFO = append(e.excludedOrderExecFIFO, execID)
	} else {
		slot = e.excludedOrderExecNext
		delete(e.excludedOrderExecutions, e.excludedOrderExecFIFO[slot])
		e.excludedOrderExecFIFO[slot] = execID
		e.excludedOrderExecNext = (slot + 1) % limit
	}
	e.excludedOrderExecutions[execID] = slot
}

func (e *engine) claimExcludedOrderExecution(execID string) {
	if slot, exists := e.excludedOrderExecutions[execID]; exists {
		delete(e.excludedOrderExecutions, execID)
		e.excludedOrderExecFIFO[slot] = ""
	}
}

func (e *engine) orderCorrelationOverflow(resource string) error {
	return errors.Join(executionCorrelationOverflow(resource, e.cfg.orderExecutionCorrelationLimit), ErrOrderRecoveryRequired)
}

func (e *engine) closeOrdersForCorrelationOverflow(resource string) {
	err := e.orderCorrelationOverflow(resource)
	for id, or := range e.orders {
		e.closeOrderRoute(id, or, err)
	}
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
	if route.cleanup != nil {
		cleanup := route.cleanup
		route.cleanup = nil
		cleanup()
	}
}

func (e *engine) routeCommissionReport(report codec.CommissionReport) {
	if len(e.orders) == 0 {
		return
	}
	st := e.execDeliveries[report.ExecID]
	if st == nil {
		if _, excluded := e.excludedOrderExecutions[report.ExecID]; excluded {
			return
		}
		if len(e.execDeliveries) >= e.cfg.orderExecutionCorrelationLimit {
			e.closeOrdersForCorrelationOverflow("distinct execution IDs")
			return
		}
		st = &execDelivery{}
		e.execDeliveries[report.ExecID] = st
	}
	if st.orderID != 0 {
		e.deliverCommissionToOrder(st, report)
		return
	}
	if len(st.pending) > 0 && st.pending[len(st.pending)-1] == report {
		return
	}
	if e.pendingOrderFees >= e.cfg.orderExecutionCorrelationLimit {
		e.closeOrdersForCorrelationOverflow("pending fee-report versions")
		return
	}
	st.pending = append(st.pending, report)
	e.pendingOrderFees++
}

// deliverCommissionToOrder emits one commission report to the handle that owns
// the execution. An identical re-send (an Executions() snapshot replaying a
// commission the handle already saw live) is deduped; a re-send with changed
// content (e.g. a realizedPNL update) goes through and becomes the new
// delivered record. A projection failure makes this local event stream
// incomplete, so terminate observation without changing the live order.
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
		e.closeOrderRoute(st.orderID, or, &ProtocolError{
			Direction: "inbound",
			Message:   fmt.Sprintf("commission report order_id %d exec_id %s", st.orderID, report.ExecID),
			Err:       err,
		})
		return
	}
	if !or.handle.emitCommissionAndFees(cr) {
		e.closeOrderRoute(st.orderID, or, ErrSlowConsumer)
		return
	}
	st.delivered = &report
}

func (e *engine) dispatchExecutionToOrder(m codec.ExecutionDetail) {
	or, ok := e.orders[m.OrderID]
	if !ok || or.closed {
		e.excludeOrderExecution(m.ExecID)
		return
	}
	if !e.claimOrderCallbackStrings(or, m.ClientID, m.PermID) {
		// Omission alone does not prove foreign ownership. Explicit client ID
		// zero is valid; the integer-unset sentinel is not an identity.
		permanent, _ := parseOptionalInt64(m.PermID, "execution permanent id")
		if (m.ClientID != "" && m.ClientID != "2147483647") || (or.permID > 0 && permanent > 0) {
			e.excludeOrderExecution(m.ExecID)
		}
		return
	}
	e.claimExcludedOrderExecution(m.ExecID)
	// Per-order dispatch: a decode failure terminates only local observation;
	// the order remains live and no cancellation frame is sent.
	exec, err := fromCodecExecution(m)
	if err != nil {
		e.closeOrderRoute(m.OrderID, or, &ProtocolError{
			Direction: "inbound",
			Message:   fmt.Sprintf("execution detail order_id %d exec_id %s", m.OrderID, m.ExecID),
			Err:       err,
		})
		return
	}
	or.handle.acknowledge()
	or.working = true
	if !e.ensureOrderStarted(or) {
		e.closeOrderRoute(m.OrderID, or, ErrSlowConsumer)
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
	if st == nil && len(e.execDeliveries) >= e.cfg.orderExecutionCorrelationLimit {
		e.closeOrderRoute(m.OrderID, or, e.orderCorrelationOverflow("distinct execution IDs"))
		return
	}
	if !or.handle.emitExecution(exec) {
		e.closeOrderRoute(m.OrderID, or, ErrSlowConsumer)
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
	e.pendingOrderFees -= len(pending)
	for _, buffered := range pending {
		e.deliverCommissionToOrder(st, buffered)
	}
}

func (e *engine) claimOrderCallbackStrings(or *orderRoute, clientID, permID string) bool {
	client, clientErr := parseOptionalInt(clientID, "order callback client id")
	permanent, permErr := parseOptionalInt64(permID, "order callback permanent id")
	if clientErr != nil || permErr != nil {
		// Preserve the pre-attribution failure path: the public projection will
		// surface malformed identity fields as a protocol error on this route.
		return true
	}
	if (clientID == "" || clientID == "2147483647") && (or.permID <= 0 || permanent <= 0) {
		return false
	}
	return e.claimOrderCallback(or, client, permanent)
}

func (e *engine) claimOrderCallback(or *orderRoute, clientID int, permID int64) bool {
	if permID > 0 {
		if or.permID > 0 {
			return or.permID == permID
		}
		if protocolIDFromInt[ClientID](clientID) != e.cfg.clientID {
			return false
		}
		or.permID = permID
		return true
	}
	return protocolIDFromInt[ClientID](clientID) == e.cfg.clientID
}

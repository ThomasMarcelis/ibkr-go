package ibkr

import (
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
			if isOrderCancellationNotice(msg) {
				e.emitEvent(msg.Code, msg.Message)
				return
			}
			or.handle.emitOrderError(e.apiErr(OpPlaceOrder, msg))
			return
		}
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
	orderRoute, orderObserved := e.orders[msg.OrderID]
	singletonRoute, singletonObserved := e.singletons[singletonOpenOrders]
	if (!orderObserved || orderRoute.closed) && !singletonObserved {
		return
	}

	status, err := fromCodecOrderStatus(msg)
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
		if !orderRoute.handle.emitStatus(status) {
			orderRoute.closed = true
		} else if IsTerminalOrderStatus(status.Status) {
			e.scheduleTerminalOrderClose(msg.OrderID, orderRoute)
		}
	}
	if singletonObserved {
		singletonRoute.handle(status, e)
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

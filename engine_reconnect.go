package ibkr

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/transport"
)

func (e *engine) handleTransportLoss(loss transportLoss) {
	if e.closed {
		return
	}
	if e.transport == nil {
		return
	}
	if loss.transport != nil && e.transport != loss.transport {
		return
	}
	routeLoss := loss.err
	if e.retiringTransport == e.transport {
		loss.err = errors.Join(loss.err, e.transportRetireErr)
		routeLoss = errors.Join(routeLoss, e.transportRouteErr)
		e.retiringTransport = nil
		e.transportRetireErr = nil
		e.transportRouteErr = nil
	}
	err := normalizeTransportErr(loss.err)
	routeErr := normalizeTransportErr(routeLoss)
	e.invalidateReconnectStability()
	if !e.bootstrap.readyReported {
		e.rememberConnectionError(&ConnectError{Op: "bootstrap", Err: err})
	}
	// A capacity waiter belongs to this validated transport generation. Its
	// Done arm cannot safely touch actor state, so transport loss owns reset.
	e.resumeWaiting = false
	e.transport = nil
	if e.cfg.reconnect == ReconnectOff {
		if routeErr == nil {
			routeErr = ErrClosed
		}
		if err == nil {
			err = routeErr
		}
		e.disconnectRoutes(routeErr, false)
		for id, order := range e.orders {
			if !order.closed {
				e.closeOrderRoute(id, order, errors.Join(ErrOrderRecoveryRequired, routeErr))
			}
		}
		e.closeEngine(routeErr, err, err)
		return
	}
	e.setState(StateReconnecting, 0, "transport lost", err)
	e.gapExecutionEvents(routeErr)
	e.disconnectRoutes(routeErr, true)
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
			lifetime := e.lifetimeCtx
			if lifetime == nil {
				lifetime = context.Background()
			}
			e.startConnect(lifetime, true)
		})
	})
}

// abortUnresolvedSingletonOneShot retires the connection generation that owns an
// unresolved request-ID-less reply. Merely deleting target would let a late
// reply satisfy a newer call of the same operation, so reconnecting is the
// only safe way to release the singleton slot.
func (e *engine) abortUnresolvedSingletonOneShot(key string, target *route) {
	if target == nil || e.singletons[key] != target || e.transport == nil {
		return
	}
	e.retireTransport(ErrInterrupted)
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

func (e *engine) disconnectRoutes(err error, preserveOrders bool) {
	for reqID, route := range e.keyed {
		// Already gapped (e.g. from code 1100) — route survives, skip duplicate Gap.
		if route.gapped {
			continue
		}
		if route.onDisconnect == nil {
			route.close(interrupted(err))
			e.deleteKeyedRoute(reqID)
			continue
		}
		if !route.onDisconnect(e, err) {
			e.deleteKeyedRoute(reqID)
		} else {
			route.gapped = true
		}
	}
	for key, route := range e.singletons {
		if route.gapped {
			continue
		}
		if route.onDisconnect == nil {
			route.close(interrupted(err))
			delete(e.singletons, key)
			continue
		}
		if !route.onDisconnect(e, err) {
			delete(e.singletons, key)
		} else {
			route.gapped = true
		}
	}
	for id, preview := range e.previews {
		preview.resolve(previewResult{err: interrupted(err)})
		delete(e.previews, id)
	}
	if !preserveOrders {
		return
	}
	// Order handles survive an automatically recovered disconnect: emit Gap,
	// do not close.
	for id, or := range e.orders {
		if !or.closed && !or.gapped {
			or.gapped = true
			if !or.handle.emitLifecycle(OrderGap, e.connectionSeq(), err) {
				e.closeOrderRoute(id, or, ErrSlowConsumer)
			}
		}
	}
}

// dropLostRoutes interrupts every route the Gateway cannot answer after a
// data-lost restoration (code 1101). The official semantics are that market
// and account data subscriptions must be resubmitted: auto-resumed
// subscriptions are re-sent by resumeRoutes, so they are skipped here;
// non-resumable subscriptions and in-flight one-shots go through the same
// onDisconnect teardown a transport loss would apply (ErrResumeRequired and
// ErrInterrupted respectively). Pending what-if previews resolve with
// ErrInterrupted — their echo died with the data connection. Live order
// handles are untouched: orders rest at IB and survive the Gateway's blip.
func (e *engine) dropLostRoutes() {
	for reqID, route := range e.keyed {
		if route.subscription && route.resume == ResumeAuto {
			continue
		}
		if route.onDisconnect == nil {
			route.close(ErrInterrupted)
			e.deleteKeyedRoute(reqID)
			continue
		}
		if !route.onDisconnect(e, nil) {
			e.deleteKeyedRoute(reqID)
		}
	}
	for key, route := range e.singletons {
		if route.onDisconnect == nil {
			route.close(ErrInterrupted)
			delete(e.singletons, key)
			continue
		}
		if !route.onDisconnect(e, nil) {
			delete(e.singletons, key)
		}
	}
	for id, preview := range e.previews {
		preview.resolve(previewResult{err: ErrInterrupted})
		delete(e.previews, id)
	}
}

func (e *engine) resumeRoutes() {
	e.resumePending = e.resumePending[:0]
	reqIDs := make([]int, 0, len(e.keyed))
	for reqID, route := range e.keyed {
		if route.subscription && route.resume == ResumeAuto {
			reqIDs = append(reqIDs, reqID)
		}
	}
	sort.Ints(reqIDs)
	for _, reqID := range reqIDs {
		e.resumePending = append(e.resumePending, resumeRoute{reqID: reqID, route: e.keyed[reqID]})
	}
	e.continueResumeRoutes()
}

func (e *engine) marketDataTypePending() bool {
	return e.marketDataType != 0 && e.marketDataTypeGeneration != e.transportGeneration
}

func (e *engine) continueResumeRoutes() {
	if e.marketDataTypePending() {
		tr := e.transport
		if tr == nil {
			return
		}
		payload, err := codec.Encode(e.serverVersion, codec.ReqMarketDataType{DataType: int(e.marketDataType)})
		if err != nil {
			e.retireTransport(err)
			return
		}
		if err := tr.Send(context.Background(), payload); err != nil {
			if errors.Is(err, transport.ErrSendQueueFull) {
				e.waitForResumeCapacity(tr)
			}
			return
		}
		e.marketDataTypeGeneration = e.transportGeneration
	}

	for len(e.resumePending) > 0 {
		pending := e.resumePending[0]
		if e.keyed[pending.reqID] != pending.route {
			e.resumePending = e.resumePending[1:]
			continue
		}

		route := pending.route
		if route.validateResume != nil {
			if err := route.validateResume(e); err != nil {
				e.dropResumeRoute(pending, err)
				e.resumePending = e.resumePending[1:]
				continue
			}
		}
		payload, err := codec.Encode(e.serverVersion, route.request)
		if err != nil {
			e.dropResumeRoute(pending, err)
			e.resumePending = e.resumePending[1:]
			continue
		}
		tr := e.transport
		if tr == nil {
			return
		}
		err = tr.Send(context.Background(), payload)
		if errors.Is(err, transport.ErrSendQueueFull) {
			e.waitForResumeCapacity(tr)
			return
		}
		if err != nil {
			// The request was valid when its route was first installed. A send
			// failure here is therefore a transport failure, not a per-route
			// failure. Leave it pending so the next ready connection retries it.
			return
		}
		select {
		case <-tr.Stopping():
			return
		default:
		}
		e.resumePending = e.resumePending[1:]
		route.generation = e.transportGeneration
		if route.gapped && route.emitResubscribed != nil {
			route.emitResubscribed(e)
		}
		route.gapped = false
	}
	e.scheduleReconnectStability(e.transport)
	e.flushReadySetups()
}

func (e *engine) waitForResumeCapacity(tr *transport.Conn) {
	if e.resumeWaiting {
		return
	}
	e.resumeWaiting = true
	go func() {
		select {
		case <-tr.Writable():
			e.enqueue(func() {
				if e.transport != tr {
					return
				}
				e.resumeWaiting = false
				e.continueResumeRoutes()
			})
		case <-tr.Done():
		}
	}()
}

func (e *engine) dropResumeRoute(pending resumeRoute, err error) {
	pending.route.close(err)
	e.deleteKeyedRoute(pending.reqID)
}

// closeEngine terminates active work with workErr, reports sessionErr on the
// final state transition, and records waitErr as the client's terminal result.
// Intentional Client.Close uses ErrClosed for work and session state but nil
// for Client.Wait; a forced retirement may need distinct work and session
// causes so one route's uncertainty does not contaminate its siblings.
func (e *engine) closeEngine(workErr, sessionErr, waitErr error) {
	if e.closed {
		return
	}
	e.closed = true
	e.invalidateReconnectStability()
	if e.connectCancel != nil {
		e.connectCancel()
		e.connectCancel = nil
	}
	if e.cancelLifetime != nil {
		e.cancelLifetime()
	}
	e.clearReadySetups()
	if e.transport != nil {
		_ = e.transport.Close()
	}
	for reqID, route := range e.keyed {
		route.close(workErr)
		e.deleteKeyedRoute(reqID)
	}
	for key, route := range e.singletons {
		route.close(workErr)
		delete(e.singletons, key)
	}
	if e.executionEvents != nil {
		e.executionEvents.sub.closeWithErr(workErr)
		e.executionEvents = nil
	}
	previewErr := workErr
	if previewErr == nil {
		previewErr = ErrInterrupted
	}
	for id, preview := range e.previews {
		preview.resolve(previewResult{err: previewErr})
		delete(e.previews, id)
	}
	for id, or := range e.orders {
		if !or.closed {
			or.closed = true
			or.handle.closeWithErr(workErr)
		}
		delete(e.orders, id)
	}
	e.execDeliveries = make(map[string]*execDelivery)
	e.pendingOrderFees = 0
	e.setState(StateClosed, 0, "", sessionErr)
	e.reportReady(sessionErr)
	e.waitMu.Lock()
	e.waitErr = waitErr
	e.waitMu.Unlock()
	if e.stopLogger != nil {
		e.stopLogger()
		e.stopLogger = nil
	}
	close(e.done)
	e.events.Close()
}

func (e *engine) invalidateReconnectStability() {
	e.stabilityEpoch++
}

func (e *engine) scheduleReconnectStability(tr *transport.Conn) {
	if tr == nil {
		return
	}
	e.stabilityEpoch++
	epoch := e.stabilityEpoch
	time.AfterFunc(reconnectBackoffMax, func() {
		e.enqueue(func() {
			if e.closed || e.transport != tr || e.stabilityEpoch != epoch {
				return
			}
			state := e.snapshot.State
			if state == StateReady {
				e.reconnectAttempt = 0
			}
		})
	})
}

func (e *engine) emitGap() {
	e.gapExecutionEvents(ErrInterrupted)
	for _, route := range e.keyed {
		if route.subscription && route.resume == ResumeAuto && route.emitGap != nil && !route.gapped {
			route.gapped = true
			route.emitGap(e)
		}
	}
	for id, or := range e.orders {
		if !or.closed && !or.gapped {
			or.gapped = true
			if !or.handle.emitLifecycle(OrderGap, e.connectionSeq(), ErrInterrupted) {
				e.closeOrderRoute(id, or, ErrSlowConsumer)
			}
		}
	}
}

func (e *engine) emitResumed() {
	e.restoreExecutionEvents()
	for _, route := range e.keyed {
		if route.subscription && route.resume == ResumeAuto && route.emitRestored != nil && route.gapped {
			route.gapped = false
			route.emitRestored(e)
		}
	}
	for id, or := range e.orders {
		if !or.closed && or.gapped {
			or.gapped = false
			if !or.handle.emitLifecycle(OrderRestored, e.connectionSeq(), nil) {
				e.closeOrderRoute(id, or, ErrSlowConsumer)
			}
		}
	}
}

// requireOrderRecovery publishes the uncertainty boundary before any business
// callback from a replacement connection can be dispatched. The caller
// supplies the sequence that the replacement connection will publish at
// readiness so the lifecycle marker and eventual Ready event agree.
func (e *engine) requireOrderRecovery(connectionSeq uint64) {
	for id, or := range e.orders {
		if or.closed || !or.gapped {
			continue
		}
		or.gapped = false
		or.recoveryRequired = true
		if !or.handle.emitLifecycle(OrderRecoveryRequired, connectionSeq, ErrOrderRecoveryRequired) {
			e.closeOrderRoute(id, or, ErrSlowConsumer)
		}
	}
}

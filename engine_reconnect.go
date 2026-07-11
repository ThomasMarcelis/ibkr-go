package ibkr

import (
	"context"
	"time"
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
	err := normalizeTransportErr(loss.err)
	e.transport = nil
	e.executions.reset()
	if e.cfg.reconnect == ReconnectOff {
		if err == nil {
			err = ErrClosed
		}
		e.disconnectRoutes(err)
		e.closeEngine(err, err)
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
	for id, preview := range e.previews {
		preview.resolve(previewResult{err: ErrInterrupted})
		delete(e.previews, id)
	}
	// Order handles survive disconnect: emit Gap, do not close.
	for _, or := range e.orders {
		if !or.closed && !or.gapped {
			or.gapped = true
			or.handle.emitState(SubscriptionStateEvent{Kind: SubscriptionGap, ConnectionSeq: e.connectionSeq()})
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
		if route.subscription && route.resume == ResumeAuto {
			continue
		}
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
	for reqID, route := range e.keyed {
		if route.subscription && route.resume == ResumeAuto {
			if route.validateResume != nil {
				if err := route.validateResume(e); err != nil {
					route.close(err)
					e.deleteKeyedRoute(reqID)
					continue
				}
			}
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
			if route.validateResume != nil {
				if err := route.validateResume(e); err != nil {
					route.close(err)
					delete(e.singletons, key)
					continue
				}
			}
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

// closeEngine terminates active work with err and records waitErr as the
// client's terminal result. An intentional Client.Close uses ErrClosed for
// active work but nil for Client.Wait; failures use the same error for both.
func (e *engine) closeEngine(err, waitErr error) {
	if e.closed {
		return
	}
	e.closed = true
	e.clearReadySetups()
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
	previewErr := err
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
			// Terminal order status is authoritative for the handle's close (see
			// docs/session-contract.md, "OrderHandle"). Session-level transport and
			// reconnect errors stay observable via SessionEvents() and Client.Wait()
			// and must not bleed into per-order Wait() once the order reached a
			// terminal business state (see "Completion and Reconnect").
			orderErr := err
			if or.terminalCloseSeq > 0 {
				orderErr = nil
			}
			or.handle.closeWithErr(orderErr)
		}
		delete(e.orders, id)
	}
	e.executions.reset()
	e.execDeliveries = make(map[string]*execDelivery)
	e.setState(StateClosed, 0, "", err)
	e.reportReady(err)
	e.waitMu.Lock()
	e.waitErr = waitErr
	e.waitMu.Unlock()
	close(e.done)
	e.events.Close()
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

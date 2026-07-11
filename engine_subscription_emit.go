package ibkr

import "github.com/ThomasMarcelis/ibkr-go/internal/codec"

// newEngineSubscription gives public cancellation and actor-owned overflow the
// same route-specific implementation. Public Close queues it onto the actor;
// overflow already runs on the actor and executes it directly, so a full
// command queue cannot make the actor wait on itself.
func newEngineSubscription[T any](cfg subscriptionConfig, e *engine, actorCancelFn func()) *Subscription[T] {
	sub := newSubscription[T](cfg, func() { e.enqueue(actorCancelFn) })
	sub.actorCancelFn = actorCancelFn
	return sub
}

// newKeyedSubscriptionRoute builds the ownership and terminal lifecycle shared
// by non-resumable request-ID subscriptions. The caller supplies the request,
// message handler, and any operation-specific API-error or reconnect behavior
// before installing the returned route in e.keyed.
func newKeyedSubscriptionRoute[T any](e *engine, cfg subscriptionConfig, reqID int, opKind OpKind, cancel codec.Message) (*Subscription[T], *route) {
	var ownedRoute *route
	var sub *Subscription[T]
	actorCancel := func() {
		if e.keyed[reqID] != ownedRoute {
			return
		}
		e.deleteKeyedRoute(reqID)
		var err error
		if cancel != nil {
			err = e.cancelSubscription(opKind, cancel)
		}
		sub.closeWithErr(err)
	}
	sub = newEngineSubscription[T](cfg, e, actorCancel)
	ownedRoute = &route{
		opKind:       opKind,
		subscription: true,
		resume:       cfg.resume,
		handleAPIErr: func(msg codec.APIError, e *engine) {
			if e.keyed[reqID] != ownedRoute {
				return
			}
			e.deleteKeyedRoute(reqID)
			sub.closeWithErr(e.apiErr(opKind, msg))
		},
		onDisconnect: func(_ *engine, _ error) bool {
			sub.closeWithErr(ErrResumeRequired)
			return false
		},
		close: func(err error) { sub.closeWithErr(err) },
	}
	return sub, ownedRoute
}

// newSingletonSubscriptionRoute is the singleton counterpart to
// newKeyedSubscriptionRoute. Unkeyed API errors remain operation-specific and
// are intentionally installed by callers when the Gateway provides a stable
// attribution signal.
func newSingletonSubscriptionRoute[T any](e *engine, cfg subscriptionConfig, key string, opKind OpKind, cancel codec.Message) (*Subscription[T], *route) {
	var ownedRoute *route
	var sub *Subscription[T]
	actorCancel := func() {
		if e.singletons[key] != ownedRoute {
			return
		}
		delete(e.singletons, key)
		var err error
		if cancel != nil {
			err = e.cancelSubscription(opKind, cancel)
		}
		sub.closeWithErr(err)
	}
	sub = newEngineSubscription[T](cfg, e, actorCancel)
	ownedRoute = &route{
		opKind:       opKind,
		subscription: true,
		resume:       cfg.resume,
		onDisconnect: func(_ *engine, _ error) bool {
			sub.closeWithErr(ErrResumeRequired)
			return false
		},
		close: func(err error) { sub.closeWithErr(err) },
	}
	return sub, ownedRoute
}

func emitSubscription[T any](sub *Subscription[T], value T) bool {
	return sub.emit(value)
}

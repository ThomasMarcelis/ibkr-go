package ibkr

import (
	"errors"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
)

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
			err = e.cancelRouteSubscription(ownedRoute, opKind, cancel)
		}
		sub.closeWithErr(err)
		e.retireSubscriptionTransport(err)
	}
	sub = newEngineSubscription[T](cfg, e, actorCancel)
	ownedRoute = &route{
		opKind:       opKind,
		subscription: true,
		resume:       cfg.resume,
		generation:   e.transportGeneration,
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
		close:  func(err error) { sub.closeWithErr(err) },
		cancel: sub.cancelFromActor,
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
		ambiguous := sub.snapshotWant && !sub.snapshotComplete()
		if ownedRoute.responsePending != nil {
			ambiguous = ownedRoute.responsePending()
		}
		delete(e.singletons, key)
		e.markSingletonGenerationDirty(key, ownedRoute)
		var err error
		if cancel != nil && !ambiguous {
			err = e.cancelRouteSubscription(ownedRoute, opKind, cancel)
		}
		sub.closeWithErr(err)
		if ambiguous {
			e.retireTransport(errors.Join(ErrInterrupted, err))
		} else {
			e.retireSubscriptionTransport(err)
		}
	}
	sub = newEngineSubscription[T](cfg, e, actorCancel)
	ownedRoute = &route{
		opKind:       opKind,
		subscription: true,
		resume:       cfg.resume,
		generation:   e.transportGeneration,
		onDisconnect: func(_ *engine, _ error) bool {
			sub.closeWithErr(ErrResumeRequired)
			return false
		},
		close:  func(err error) { sub.closeWithErr(err) },
		cancel: sub.cancelFromActor,
	}
	return sub, ownedRoute
}

func (e *engine) markSingletonGenerationDirty(key string, ownedRoute *route) {
	if ownedRoute == nil || e.transport == nil || ownedRoute.generation != e.transportGeneration {
		return
	}
	if e.dirtySingletons == nil {
		e.dirtySingletons = make(map[string]uint64)
	}
	e.dirtySingletons[key] = ownedRoute.generation
}

func (e *engine) singletonGenerationDirty(key string) bool {
	generation, ok := e.dirtySingletons[key]
	return ok && generation == e.transportGeneration
}

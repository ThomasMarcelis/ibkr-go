package ibkr

import (
	"errors"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
)

// cancelRouteSubscription records cancellation only when this route has been
// admitted on the current transport generation. A route still waiting in the
// reconnect resume queue is absent remotely and needs only local teardown.
func (e *engine) cancelRouteSubscription(route *route, opKind OpKind, msg codec.Message) error {
	if route == nil || route.generation != e.transportGeneration || e.routeAwaitingResume(route) {
		return nil
	}
	return e.cancelCurrentRequest(opKind, msg)
}

func (e *engine) routeAwaitingResume(target *route) bool {
	for _, pending := range e.resumePending {
		if pending.route == target {
			return true
		}
	}
	return false
}

// cancelCurrentRequest records cancellation admission for an in-flight
// request that is known to belong to the current transport.
func (e *engine) cancelCurrentRequest(opKind OpKind, msg codec.Message) error {
	if !e.hasReadyTransport() {
		return nil
	}
	tr := e.transport
	select {
	case <-tr.Stopping():
		return nil
	default:
	}

	if err := e.send(msg); err != nil {
		// Transport loss racing cancellation also destroys the remote stream;
		// only a failure on the still-active connection is uncertain.
		select {
		case <-tr.Stopping():
			return nil
		default:
			return &SubscriptionCancelError{OpKind: opKind, Err: err}
		}
	}
	return nil
}

// retireSubscriptionTransport applies the connection-level consequence of a
// cancellation admission failure after the initiating route has preserved its
// own terminal cause. Keeping this separate from cancelSubscription lets a
// batch teardown record the same failed admission on every affected route
// before disconnect processing reaches their siblings.
func (e *engine) retireSubscriptionTransport(err error) {
	if _, ok := errors.AsType[*SubscriptionCancelError](err); !ok {
		return
	}
	e.retireTransport(err)
}

// retireTransport stops the current connection generation but leaves loss
// delivery to the transport pumps. Their normal completion ordering ensures
// admitted write outcomes and decoded frames reach the actor first.
func (e *engine) retireTransport(err error) {
	if e.transport == nil {
		return
	}
	tr := e.transport
	if e.retiringTransport == tr {
		e.transportRetireErr = errors.Join(e.transportRetireErr, err)
		return
	}
	e.retiringTransport = tr
	e.transportRetireErr = err
	_ = tr.Close()
}

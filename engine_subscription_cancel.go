package ibkr

import (
	"errors"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
)

// cancelSubscription records cancellation admission only while the route is
// active on the current transport. A route retained across a lost connection
// is already gone remotely, and a route closed during the replacement
// handshake has not yet been resumed, so both are clean local detachments.
func (e *engine) cancelSubscription(opKind OpKind, msg codec.Message) error {
	if !e.isReady() {
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
	if _, ok := errors.AsType[*SubscriptionCancelError](err); !ok || e.transport == nil {
		return
	}
	tr := e.transport
	_ = tr.Close()
	e.handleTransportLoss(transportLoss{transport: tr, err: err})
}

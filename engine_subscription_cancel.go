package ibkr

import "github.com/ThomasMarcelis/ibkr-go/internal/codec"

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
	case <-tr.Done():
		return nil
	default:
	}

	if err := e.send(msg); err != nil {
		// Transport loss racing cancellation also destroys the remote stream;
		// only a failure on the still-active connection is uncertain.
		select {
		case <-tr.Done():
			return nil
		default:
			return &SubscriptionCancelError{OpKind: opKind, Err: err}
		}
	}
	return nil
}

package ibkr

func (e *engine) run() {
	for {
		e.drainIncoming()

		// Give a pending transport error priority over cmds, but only
		// after all decoded messages have been handled.
		select {
		case err := <-e.transportErr:
			e.drainIncoming()
			e.handleTransportLoss(err)
			continue
		default:
		}

		select {
		case fn := <-e.cmds:
			if fn != nil {
				fn()
			}
		case msg := <-e.incoming:
			e.handleIncoming(msg)
		case err := <-e.transportErr:
			e.drainIncoming()
			e.handleTransportLoss(err)
		case <-e.done:
			return
		}
	}
}

// drainIncoming synchronously consumes every buffered decoded message.
// Draining before handling a transport error is complete, not just
// best-effort: attachTransport guarantees that all of a connection's
// decoded messages are sent to e.incoming before its transportErr is
// sent — the ProtocolError send happens on the decode goroutine after
// its final incoming send, and the tr.Wait() send is gated on
// decodedDone.
func (e *engine) drainIncoming() {
	for {
		select {
		case msg := <-e.incoming:
			e.handleIncoming(msg)
		default:
			return
		}
	}
}

package ibkr

func (e *engine) run() {
	for {
		// Closed is absorbing. Once closeEngine closes done, do not let a fair
		// select choose already-buffered input and mutate the terminal state.
		select {
		case <-e.done:
			return
		default:
		}
		// One fair select per iteration. An earlier version drained e.incoming
		// to empty at the top of every loop; under a sustained hot feed that
		// drain never returned and control commands (subscribe/cancel/place)
		// on e.cmds starved. The full drain is only required to satisfy the
		// transport-loss ordering invariant, so it now lives on the
		// transportErr arms alone.
		select {
		case fn := <-e.cmds:
			if fn != nil {
				fn()
			}
		case msg := <-e.incoming:
			e.handleActorInput(msg)
		case loss := <-e.transportErr:
			e.drainIncoming()
			e.handleTransportLoss(loss)
		case result := <-e.connectResults:
			e.handleConnectResult(result)
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
// decodedDone. Because that transportErr send only happens after the last
// incoming send, by the time this arm observes the error every one of the
// connection's messages is already buffered on e.incoming, so this drain
// handles all of them before handleTransportLoss tears the routes down.
func (e *engine) drainIncoming() {
	for {
		select {
		case msg := <-e.incoming:
			e.handleActorInput(msg)
		default:
			return
		}
	}
}

func (e *engine) handleActorInput(msg any) {
	if write, ok := msg.(transportWrite); ok {
		e.handleTransportWrite(write)
		return
	}
	e.handleIncoming(msg)
}

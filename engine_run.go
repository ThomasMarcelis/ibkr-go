package ibkr

func (e *engine) run() {
	for {
		for {
			select {
			case msg := <-e.incoming:
				e.handleIncoming(msg)
				continue
			default:
			}
			break
		}

		select {
		case err := <-e.transportErr:
			if len(e.incoming) > 0 {
				go func(err error) {
					e.transportErr <- err
				}(err)
				continue
			}
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
			if len(e.incoming) > 0 {
				go func(err error) {
					e.transportErr <- err
				}(err)
				continue
			}
			e.handleTransportLoss(err)
		case <-e.done:
			return
		}
	}
}

package ibkr

func emitSubscription[T any](sub *Subscription[T], value T) bool {
	if sub.emit(value) {
		return true
	}
	_ = sub.Close()
	return false
}

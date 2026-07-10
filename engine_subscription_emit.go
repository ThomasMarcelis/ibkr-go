package ibkr

// newEngineSubscription gives public cancellation and actor-owned overflow the
// same route-specific implementation. Public Close queues it onto the actor;
// overflow already runs on the actor and executes it directly, so a full
// command queue cannot make the actor wait on itself.
func newEngineSubscription[T any](cfg subscriptionConfig, e *engine, actorCancelFn func()) *Subscription[T] {
	sub := newSubscription[T](cfg, func() { e.enqueue(actorCancelFn) })
	sub.actorCancelFn = actorCancelFn
	return sub
}

func emitSubscription[T any](sub *Subscription[T], value T) bool {
	return sub.emit(value)
}

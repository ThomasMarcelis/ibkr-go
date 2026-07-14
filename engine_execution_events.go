package ibkr

import (
	"context"
	"fmt"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
)

type executionEventRoute struct {
	sub    *Subscription[ExecutionEvent]
	gapped bool
}

func (e *engine) SubscribeExecutionEvents(ctx context.Context, opts ...SubscriptionOption) (*Subscription[ExecutionEvent], error) {
	type result struct {
		sub *Subscription[ExecutionEvent]
		err error
	}
	resp := make(chan result, 1)
	enqueueSubscriptionSetup(ctx, e, resp, func() {
		if e.executionEvents != nil {
			resp <- result{err: operationActive("execution event observer")}
			return
		}
		cfg, err := applySubscriptionOptions(e.cfg, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		if cfg.resume != ResumeNever {
			resp <- result{err: &ValidationError{
				Field: "ResumePolicy", Value: string(cfg.resume),
				Message: "execution event observer follows the client connection automatically",
			}}
			return
		}
		if cfg.executionCorrelationLimitSet {
			resp <- result{err: &ValidationError{
				Field: "ExecutionCorrelationLimit", Message: "does not apply to the uncorrelated execution event observer",
			}}
			return
		}

		var owned *executionEventRoute
		var sub *Subscription[ExecutionEvent]
		actorCancel := func() {
			if e.executionEvents != owned {
				return
			}
			e.executionEvents = nil
			sub.closeWithErr(nil)
		}
		sub = newEngineSubscription[ExecutionEvent](cfg, e, actorCancel)
		owned = &executionEventRoute{sub: sub}
		e.executionEvents = owned
		sub.emitState(StreamStarted, e.connectionSeq(), nil)
		resp <- result{sub: sub}
	})

	out, err := awaitSubscriptionResponse(ctx, e, resp, func(out result) bool { return out.sub != nil })
	if err != nil {
		return nil, err
	}
	if out.err == nil && out.sub != nil {
		bindContext(ctx, out.sub)
	}
	return out.sub, out.err
}

func (e *engine) emitExecutionDetailEvent(message codec.ExecutionDetail) {
	owned := e.executionEvents
	if owned == nil {
		return
	}
	execution, err := fromCodecExecution(message)
	if err != nil {
		owned.sub.cancelFromActor(&ProtocolError{
			Direction: "inbound",
			Message:   fmt.Sprintf("execution detail req_id %d exec_id %s", message.ReqID, message.ExecID),
			Err:       err,
		})
		return
	}
	owned.sub.emit(ExecutionEvent{
		RequestID: new(protocolIDFromInt[RequestID](message.ReqID)),
		Execution: &execution,
	})
}

func (e *engine) emitCommissionEvent(message codec.CommissionReport) {
	owned := e.executionEvents
	if owned == nil {
		return
	}
	report, err := fromCodecCommission(message)
	if err != nil {
		owned.sub.cancelFromActor(&ProtocolError{
			Direction: "inbound",
			Message:   fmt.Sprintf("commission report exec_id %s", message.ExecID),
			Err:       err,
		})
		return
	}
	owned.sub.emit(ExecutionEvent{CommissionAndFees: &report})
}

func (e *engine) gapExecutionEvents(err error) {
	owned := e.executionEvents
	if owned == nil || owned.gapped {
		return
	}
	owned.gapped = true
	owned.sub.emitState(StreamGap, e.connectionSeq(), err)
}

func (e *engine) restoreExecutionEvents() {
	owned := e.executionEvents
	if owned == nil || !owned.gapped {
		return
	}
	owned.gapped = false
	owned.sub.emitState(StreamRestored, e.connectionSeq(), nil)
}

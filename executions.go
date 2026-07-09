package ibkr

import "github.com/ThomasMarcelis/ibkr-go/internal/codec"

type executionCorrelator struct {
	routes map[int]struct{}
	execs  map[string]*executionState
}

type executionState struct {
	routes      map[int]*executionRouteState
	commissions []codec.CommissionReport
}

type executionRouteState struct {
	seenExecution        bool
	deliveredCommissions int
}

func newExecutionCorrelator() executionCorrelator {
	return executionCorrelator{
		routes: make(map[int]struct{}),
		execs:  make(map[string]*executionState),
	}
}

func (c *executionCorrelator) registerRoute(reqID int) {
	c.routes[reqID] = struct{}{}
}

func (c *executionCorrelator) unregisterRoute(reqID int) {
	delete(c.routes, reqID)
	for execID, state := range c.execs {
		delete(state.routes, reqID)
		c.maybeClearCommissionHistory(execID)
		if len(state.routes) == 0 && len(state.commissions) == 0 {
			delete(c.execs, execID)
		}
	}
	if len(c.routes) == 0 {
		c.reset()
	}
}

func (c *executionCorrelator) observeExecution(reqID int, detail codec.ExecutionDetail) {
	state := c.ensureExecState(detail.ExecID)
	// A commission report carries only ExecID, not reqID. Track every active
	// route as a possible observer until the Gateway either delivers this
	// execution on that route or closes the route. This preserves overlapping
	// query delivery without duplicating IBKR's filter semantics locally.
	for routeID := range c.routes {
		if state.routes[routeID] == nil {
			state.routes[routeID] = &executionRouteState{}
		}
	}
	if state.routes[reqID] == nil {
		state.routes[reqID] = &executionRouteState{}
	}
	state.routes[reqID].seenExecution = true
}

func (c *executionCorrelator) recordCommission(report codec.CommissionReport) []int {
	// With no registered Executions() route, no route can ever observe this
	// execution, so buffering the commission only grows c.execs by one entry
	// per live fill for the whole connection with nothing to consume it — the
	// leak the caller hits when it places orders but never queries executions.
	// Drop it: a later query re-observes the execution and the Gateway re-sends
	// the commission frame then. When routes DO exist the commission is
	// buffered even before its execution is observed, preserving the backlog
	// contract for a report that races ahead of its execution detail.
	if len(c.routes) == 0 {
		return nil
	}
	state := c.ensureExecState(report.ExecID)
	for routeID := range c.routes {
		if state.routes[routeID] == nil {
			state.routes[routeID] = &executionRouteState{}
		}
	}
	state.commissions = append(state.commissions, report)

	ready := make([]int, 0, len(state.routes))
	for reqID, routeState := range state.routes {
		if routeState.seenExecution {
			ready = append(ready, reqID)
		}
	}
	return ready
}

func (c *executionCorrelator) undeliveredCommissions(reqID int, execID string) []codec.CommissionReport {
	state, ok := c.execs[execID]
	if !ok {
		return nil
	}
	routeState, ok := state.routes[reqID]
	if !ok || !routeState.seenExecution {
		return nil
	}
	if routeState.deliveredCommissions >= len(state.commissions) {
		return nil
	}

	out := append([]codec.CommissionReport(nil), state.commissions[routeState.deliveredCommissions:]...)
	routeState.deliveredCommissions = len(state.commissions)
	c.maybeClearCommissionHistory(execID)
	return out
}

func (c *executionCorrelator) reset() {
	c.routes = make(map[int]struct{})
	c.execs = make(map[string]*executionState)
}

func (c *executionCorrelator) ensureExecState(execID string) *executionState {
	if c.execs[execID] == nil {
		c.execs[execID] = &executionState{
			routes: make(map[int]*executionRouteState),
		}
	}
	return c.execs[execID]
}

func (c *executionCorrelator) maybeClearCommissionHistory(execID string) {
	state, ok := c.execs[execID]
	if !ok || len(state.commissions) == 0 {
		return
	}
	if len(state.routes) == 0 {
		return
	}
	for _, routeState := range state.routes {
		if !routeState.seenExecution {
			return
		}
		if routeState.deliveredCommissions < len(state.commissions) {
			return
		}
	}
	state.commissions = nil
	for _, routeState := range state.routes {
		routeState.deliveredCommissions = 0
	}
}

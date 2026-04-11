package ibkr

import "github.com/ThomasMarcelis/ibkr-go/internal/codec"

type executionCorrelator struct {
	routes map[int]ExecutionsRequest
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
		routes: make(map[int]ExecutionsRequest),
		execs:  make(map[string]*executionState),
	}
}

func (c *executionCorrelator) registerRoute(reqID int, req ExecutionsRequest) {
	c.routes[reqID] = req
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
	for routeID, req := range c.routes {
		if !matchesExecutionRequest(req, detail) {
			continue
		}
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
	state := c.ensureExecState(report.ExecID)
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
	c.routes = make(map[int]ExecutionsRequest)
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

func matchesExecutionRequest(req ExecutionsRequest, detail codec.ExecutionDetail) bool {
	if req.Account != "" && req.Account != detail.Account {
		return false
	}
	if req.Symbol != "" && req.Symbol != detail.Symbol {
		return false
	}
	return true
}

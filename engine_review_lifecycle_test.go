package ibkr

import (
	"context"
	"errors"
	"io"
	"testing"
	"testing/synctest"
	"time"
)

func TestBootstrapTimeoutAndFailureTaxonomy(t *testing.T) {
	for _, tc := range []struct {
		ready  bool
		policy ReconnectPolicy
	}{{false, ReconnectOff}, {false, ReconnectAuto}, {true, ReconnectOff}} {
		ready := tc.ready
		t.Run(map[bool]string{false: "bootstrap", true: "ready"}[ready]+string(tc.policy), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				e, _ := newObservedMarketDataEngine(t)
				e.ready = make(chan error, 1)
				e.cfg.reconnect = tc.policy
				if !ready {
					e.snapshot.State = StateHandshaking
					e.bootstrap.readyReported = false
				}
				tr := e.transport
				e.scheduleBootstrapTimeout(tr)
				<-time.After(bootstrapTimeout)
				synctest.Wait()
				select {
				case fn := <-e.cmds:
					fn()
				default:
					t.Fatal("bootstrap timer did not reach actor")
				}
				select {
				case <-tr.Stopping():
					if ready {
						t.Fatal("bootstrap timer stopped ready transport")
					}
				default:
					if !ready {
						t.Fatal("bootstrap timer did not stop stalled transport")
					}
				}
				if ready {
					return
				}
				e.handleTransportLoss(transportLoss{transport: tr})
				err := e.lastConnectionError()
				if tc.policy == ReconnectOff {
					err = e.Wait()
				} else {
					if e.Session().State != StateReconnecting {
						t.Fatal("auto bootstrap failure did not reconnect")
					}
					e.closeEngine(ErrClosed, nil, nil)
				}
				connect, ok := errors.AsType[*ConnectError](err)
				if !ok || connect.Op != "bootstrap" || connect.Err == nil || !IsRetryable(err) {
					t.Fatalf("bootstrap timeout = %v", err)
				}
				// A closed observer may still contain buffered events, but no read
				// can block once Done has reported engine completion.
				for {
					select {
					case _, open := <-e.SessionEvents():
						if !open {
							return
						}
					default:
						t.Fatal("Done preceded SessionEvents closure")
					}
				}
			})
		})
	}
}

func TestBootstrapSocketLossPreservesCause(t *testing.T) {
	e, _ := newObservedMarketDataEngine(t)
	e.ready = make(chan error, 1)
	e.cfg.reconnect = ReconnectOff
	e.snapshot.State = StateHandshaking
	e.bootstrap.readyReported = false
	e.handleTransportLoss(transportLoss{transport: e.transport, err: io.EOF})
	connect, ok := errors.AsType[*ConnectError](e.Wait())
	if !ok || connect.Op != "bootstrap" || !errors.Is(connect, io.EOF) {
		t.Fatalf("bootstrap socket loss = %v", e.Wait())
	}
	if IsRetryable(errors.Join(context.DeadlineExceeded, connect)) {
		t.Fatal("caller cancellation lost classification precedence")
	}
}

func TestInvalidRequestsNeverReachAdmission(t *testing.T) {
	// Consumer sv225 traces showed silent MarketRule(0), and code 321
	// "Please enter exchange" for ConID-only option calculations. Positive
	// qualified calculations remain frozen in the public option replay.
	e := &engine{cmds: make(chan func(), 1), done: make(chan struct{})}
	cases := []struct {
		name, field string
		call        func() error
	}{
		{"rule zero", "MarketRuleID", func() error { _, err := e.MarketRule(t.Context(), 0); return err }},
		{"rule negative", "MarketRuleID", func() error { _, err := e.MarketRule(t.Context(), -1); return err }},
		{"option price", "Contract.Exchange", func() error {
			_, err := e.CalcOptionPrice(t.Context(), CalcOptionPriceRequest{Contract: Contract{ConID: 909906426}})
			return err
		}},
		{"implied volatility", "Contract.Exchange", func() error {
			_, err := e.CalcImpliedVolatility(t.Context(), CalcImpliedVolatilityRequest{Contract: Contract{ConID: 909906426}})
			return err
		}},
		{"foreign cancel", "ClientID", func() error {
			return (OrdersClient{engine: e}).Cancel(t.Context(), OrderTarget{ClientID: 1, OrderID: 493})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			validation, ok := errors.AsType[*ValidationError](err)
			if !ok || validation.Field != tc.field {
				t.Fatalf("validation = %v, want %s", err, tc.field)
			}
			if len(e.cmds) != 0 {
				t.Fatal("invalid request reached admission")
			}
		})
	}
}

func TestHistoricalPacingBoundariesAndCancellationRace(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, _ := newObservedMarketDataEngine(t)
		calls, canceled := 0, 0
		ctx, cancel := context.WithCancel(t.Context())
		enqueueHistoricalSetup(ctx, e, "same", nil, func() { calls++ })
		(<-e.cmds)()
		enqueueHistoricalSetup(ctx, e, "same", func() { canceled++ }, func() { calls++ })
		(<-e.cmds)()
		<-time.After(historicalRequestSpacing)
		synctest.Wait()
		if len(e.cmds) != 0 || calls != 1 {
			t.Fatal("identical request escaped fifteen-second gate")
		}
		<-time.After(historicalIdenticalSpacing - historicalRequestSpacing)
		synctest.Wait() // Timer wake is queued, but admission has not run.
		cancel()
		synctest.Wait()
		for len(e.cmds) > 0 {
			(<-e.cmds)()
		}
		if calls != 1 || canceled != 1 || len(e.historicalWaits) != 0 {
			t.Fatalf("calls/cancellations/waits = %d/%d/%d", calls, canceled, len(e.historicalWaits))
		}
		enqueueHistoricalSetup(t.Context(), e, "same", nil, func() { calls++ })
		(<-e.cmds)()
		if calls != 2 {
			t.Fatal("request did not become admissible at fifteen seconds")
		}
	})
}

func TestHistoricalPacingWaitRechecksReadinessAndShutdown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, _ := newObservedMarketDataEngine(t)
		calls := 0
		enqueueHistoricalSetup(t.Context(), e, "same", nil, func() { calls++ })
		(<-e.cmds)()
		enqueueHistoricalSetup(t.Context(), e, "same", nil, func() { calls++ })
		(<-e.cmds)()
		e.snapshot.State = StateHandshaking
		<-time.After(historicalIdenticalSpacing)
		synctest.Wait()
		(<-e.cmds)()
		if calls != 1 || len(e.readySetups) != 1 || len(e.historicalWaits) != 0 {
			t.Fatal("paced work entered bootstrap")
		}
		e.snapshot.State = StateReady
		e.flushReadySetups()
		if calls != 2 {
			t.Fatal("ready work did not resume")
		}
		enqueueHistoricalSetup(t.Context(), e, "same", nil, func() { calls++ })
		(<-e.cmds)()
		e.closeEngine(ErrClosed, nil, nil)
		<-time.After(historicalIdenticalSpacing)
		synctest.Wait()
		if calls != 2 || len(e.historicalWaits) != 0 || len(e.cmds) != 0 {
			t.Fatal("shutdown retained pacing work")
		}
	})
}

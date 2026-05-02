package ibkr

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
)

func TestSDKContractDetailsPublicRouteReplaysReadOnlyFixture(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan struct {
		details []ContractDetails
		err     error
	}, 1)
	go func() {
		details, err := client.Contracts().Details(ctx, Contract{
			Symbol:   "AAPL",
			SecType:  SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		})
		resultCh <- struct {
			details []ContractDetails
			err     error
		}{details: details, err: err}
	}()

	runNextEngineCommand(t, e)
	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandContractDetails {
		t.Fatalf("command kind = %s, want %s", command.Kind, sdkadapter.CommandContractDetails)
	}
	if command.ContractDetails.Contract.Symbol != "AAPL" || command.ContractDetails.Contract.Exchange != "SMART" {
		t.Fatalf("contract details command contract = %+v, want AAPL SMART", command.ContractDetails.Contract)
	}

	event := readOnlyFixtureEvent(t, sdkadapter.EventContractDetails, 101)
	event.ReqID = command.ContractDetails.ReqID
	dispatchSDKFixtureEvent(t, e, event)
	dispatchSDKFixtureEvent(t, e, sdkadapter.Event{
		Kind:  sdkadapter.EventContractDetailsEnd,
		ReqID: command.ContractDetails.ReqID,
	})

	result := receiveContractDetailsResult(t, resultCh)
	if result.err != nil {
		t.Fatalf("Contracts().Details() error = %v", result.err)
	}
	if len(result.details) != 1 {
		t.Fatalf("Contracts().Details() len = %d, want 1", len(result.details))
	}
	detail := result.details[0]
	if detail.Contract.ConID != 265598 ||
		detail.Contract.Symbol != "AAPL" ||
		detail.Contract.SecType != SecTypeStock ||
		detail.Contract.PrimaryExchange != "NASDAQ" ||
		detail.LongName != "APPLE INC" ||
		detail.TimeZoneID != "US/Eastern" ||
		detail.MinTick.String() != "0.01" {
		t.Fatalf("Contracts().Details() = %+v, want captured AAPL fixture details", detail)
	}
}

func TestSDKBondContractDetailsPublicRouteReplaysOfficialFixture(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan struct {
		details []ContractDetails
		err     error
	}, 1)
	go func() {
		details, err := client.Contracts().Details(ctx, Contract{
			Symbol:   "449276AA2",
			SecType:  SecTypeBond,
			Exchange: "SMART",
			Currency: "USD",
		})
		resultCh <- struct {
			details []ContractDetails
			err     error
		}{details: details, err: err}
	}()

	runNextEngineCommand(t, e)
	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandContractDetails {
		t.Fatalf("command kind = %s, want %s", command.Kind, sdkadapter.CommandContractDetails)
	}
	if command.ContractDetails.Contract.Symbol != "449276AA2" ||
		command.ContractDetails.Contract.SecType != "BOND" ||
		command.ContractDetails.Contract.Exchange != "SMART" {
		t.Fatalf("bond contract command contract = %+v, want official sample CUSIP bond", command.ContractDetails.Contract)
	}

	event := fixtureEvent(t, "internal/sdkadapter/testdata/fixtures/official_sdk_bond_contract_details_snapshot_20260502.json", sdkadapter.EventBondContractDetails, 1601)
	event.ReqID = command.ContractDetails.ReqID
	dispatchSDKFixtureEvent(t, e, event)
	dispatchSDKFixtureEvent(t, e, sdkadapter.Event{
		Kind:  sdkadapter.EventContractDetailsEnd,
		ReqID: command.ContractDetails.ReqID,
	})

	result := receiveContractDetailsResult(t, resultCh)
	if result.err != nil {
		t.Fatalf("Contracts().Details() error = %v", result.err)
	}
	if len(result.details) != 1 {
		t.Fatalf("Contracts().Details() len = %d, want 1", len(result.details))
	}
	detail := result.details[0]
	if detail.Contract.ConID != 681308048 ||
		detail.Contract.SecType != SecTypeBond ||
		detail.Contract.Exchange != "SMART" ||
		detail.Contract.TradingClass != "IBM" ||
		detail.MinTick.String() != "0.001" {
		t.Fatalf("Contracts().Details() = %+v, want captured IBM bond fixture details", detail)
	}
}

func TestSDKQualifyPublicRouteReplaysReadOnlyFixture(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan struct {
		detail ContractDetails
		err    error
	}, 1)
	go func() {
		detail, err := client.Contracts().Qualify(ctx, Contract{
			Symbol:   "AAPL",
			SecType:  SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		})
		resultCh <- struct {
			detail ContractDetails
			err    error
		}{detail: detail, err: err}
	}()

	runNextEngineCommand(t, e)
	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandContractDetails {
		t.Fatalf("command kind = %s, want %s", command.Kind, sdkadapter.CommandContractDetails)
	}

	event := readOnlyFixtureEvent(t, sdkadapter.EventContractDetails, 101)
	event.ReqID = command.ContractDetails.ReqID
	dispatchSDKFixtureEvent(t, e, event)
	dispatchSDKFixtureEvent(t, e, sdkadapter.Event{
		Kind:  sdkadapter.EventContractDetailsEnd,
		ReqID: command.ContractDetails.ReqID,
	})

	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("Contracts().Qualify() error = %v", result.err)
		}
		if result.detail.Contract.ConID != 265598 || result.detail.LongName != "APPLE INC" {
			t.Fatalf("Contracts().Qualify() = %+v, want captured AAPL fixture details", result.detail)
		}
	case <-time.After(time.Second):
		t.Fatal("Contracts().Qualify() did not return")
	}
}

func TestSDKContractDetailsContextCancelDoesNotSendUnsupportedCancel(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithCancel(context.Background())
	resultCh := make(chan error, 1)
	go func() {
		_, err := client.Contracts().Details(ctx, Contract{
			Symbol:   "AAPL",
			SecType:  SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		})
		resultCh <- err
	}()

	runNextEngineCommand(t, e)
	cancel()

	select {
	case err := <-resultCh:
		if err == nil {
			t.Fatal("Contracts().Details() error = nil, want context cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("Contracts().Details() did not return after context cancel")
	}

	runNextEngineCommand(t, e)
	commands := adapter.Commands()
	if len(commands) != 1 {
		t.Fatalf("commands len = %d, want only original contract-details request: %+v", len(commands), commands)
	}
	if commands[0].Kind != sdkadapter.CommandContractDetails {
		t.Fatalf("command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandContractDetails)
	}
}

func newManualReadySDKEngine(adapter sdkadapter.Adapter) *engine {
	cfg := defaultConfig()
	return &engine{
		cfg:                      cfg,
		cmds:                     make(chan func(), 256),
		incoming:                 make(chan any, 256),
		adapterErr:               make(chan error, 8),
		ready:                    make(chan error, 1),
		done:                     make(chan struct{}),
		events:                   newObserver[Event](cfg.eventBuffer),
		adapter:                  adapter,
		keyed:                    make(map[int]*route),
		singletons:               make(map[string]*route),
		orders:                   make(map[int64]*orderRoute),
		executions:               newExecutionCorrelator(),
		execToOrder:              make(map[string]int64),
		nextReqID:                1,
		recentHistoricalRequests: make(map[string]time.Time),
		snapshot: Snapshot{
			State:           StateReady,
			ConnectionSeq:   1,
			ServerVersion:   203,
			ManagedAccounts: []string{"DU_REDACTED"},
			NextValidID:     1,
		},
	}
}

func runNextEngineCommand(t *testing.T, e *engine) {
	t.Helper()

	select {
	case fn := <-e.cmds:
		fn()
	case <-time.After(time.Second):
		t.Fatal("engine command was not enqueued")
	}
}

func onlySDKCommand(t *testing.T, adapter *sdkadapter.ReplayAdapter) sdkadapter.Command {
	t.Helper()

	commands := adapter.Commands()
	if len(commands) != 1 {
		t.Fatalf("commands len = %d, want 1: %+v", len(commands), commands)
	}
	return commands[0]
}

func readOnlyFixtureEvent(t *testing.T, kind sdkadapter.EventKind, reqID int) sdkadapter.Event {
	t.Helper()
	return fixtureEvent(t, "internal/sdkadapter/testdata/fixtures/official_sdk_read_only_20260502.json", kind, reqID)
}

func fixtureEvent(t *testing.T, path string, kind sdkadapter.EventKind, reqID int) sdkadapter.Event {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := sdkadapter.DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	for _, event := range fixture.Events {
		if event.Kind == kind && event.ReqID == reqID {
			return event
		}
	}
	t.Fatalf("fixture event kind=%s reqID=%d not found", kind, reqID)
	return sdkadapter.Event{}
}

func dispatchSDKFixtureEvent(t *testing.T, e *engine, event sdkadapter.Event) {
	t.Helper()

	msg, err := sdkEventToMessage(event)
	if err != nil {
		t.Fatalf("sdkEventToMessage(%s) error = %v", event.Kind, err)
	}
	e.handleIncoming(msg)
}

func receiveContractDetailsResult(t *testing.T, resultCh <-chan struct {
	details []ContractDetails
	err     error
}) struct {
	details []ContractDetails
	err     error
} {
	t.Helper()

	select {
	case result := <-resultCh:
		return result
	case <-time.After(time.Second):
		t.Fatal("Contracts().Details() did not return")
		return struct {
			details []ContractDetails
			err     error
		}{}
	}
}

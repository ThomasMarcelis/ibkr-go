package ibkr

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
)

func TestSDKBootstrapFixtureUpdatesClientSession(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	e.snapshot = Snapshot{State: StateHandshaking}
	e.bootstrap = bootstrapState{}
	client := &Client{engine: e}

	dispatchSDKFixtureEvent(t, e, readOnlyFixtureEvent(t, sdkadapter.EventConnectionMetadata, 0))
	dispatchSDKFixtureEvent(t, e, readOnlyFixtureEvent(t, sdkadapter.EventManagedAccounts, 0))
	dispatchSDKFixtureEvent(t, e, readOnlyFixtureEvent(t, sdkadapter.EventNextValidID, 0))

	snap := client.Session()
	if snap.State != StateReady ||
		snap.ConnectionSeq != 1 ||
		snap.ServerVersion != 203 ||
		snap.NextValidID != 1 ||
		len(snap.ManagedAccounts) != 1 ||
		snap.ManagedAccounts[0] != "DU_REDACTED" {
		t.Fatalf("Session() = %+v, want ready session from SDK bootstrap fixture", snap)
	}
	snap.ManagedAccounts[0] = "mutated"
	if got := client.Session().ManagedAccounts[0]; got != "DU_REDACTED" {
		t.Fatalf("Session() managed account after caller mutation = %q, want copied DU_REDACTED", got)
	}

	select {
	case err := <-e.ready:
		if err != nil {
			t.Fatalf("ready error = %v, want nil", err)
		}
	default:
		t.Fatal("SDK bootstrap fixture did not report ready")
	}

	select {
	case event := <-client.SessionEvents():
		if event.State != StateReady ||
			event.Previous != StateHandshaking ||
			event.ConnectionSeq != 1 {
			t.Fatalf("SessionEvents() event = %+v, want ready transition", event)
		}
	default:
		t.Fatal("SessionEvents() did not emit ready transition")
	}
}

func TestSDKClientCloseDoneWait(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	if err := client.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	runNextEngineCommand(t, e)

	select {
	case <-client.Done():
	case <-time.After(time.Second):
		t.Fatal("Done() did not close")
	}
	if err := client.Wait(); !errors.Is(err, ErrClosed) {
		t.Fatalf("Wait() error = %v, want ErrClosed", err)
	}
}

func TestCurrentTimeUsesSDKCommandAndEvent(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	if err := e.sendSDKContext(context.Background(), sdkadapter.CurrentTimeRequest{}); err != nil {
		t.Fatalf("sendSDKContext() error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 1 {
		t.Fatalf("commands len = %d, want 1", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandCurrentTime {
		t.Fatalf("command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandCurrentTime)
	}

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind:        sdkadapter.EventCurrentTime,
		CurrentTime: 1712345678,
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage() error = %v", err)
	}
	got, ok := msg.(sdkadapter.CurrentTime)
	if !ok {
		t.Fatalf("sdkEventToMessage() type = %T, want sdkadapter.CurrentTime", msg)
	}
	if got.Time != "1712345678" {
		t.Fatalf("CurrentTime time = %q, want 1712345678", got.Time)
	}
}

func TestSDKCurrentTimePublicRouteReplaysReadOnlyFixture(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan struct {
		ts  time.Time
		err error
	}, 1)
	go func() {
		ts, err := client.CurrentTime(ctx)
		resultCh <- struct {
			ts  time.Time
			err error
		}{ts: ts, err: err}
	}()

	runNextEngineCommand(t, e)
	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandCurrentTime {
		t.Fatalf("command kind = %s, want %s", command.Kind, sdkadapter.CommandCurrentTime)
	}

	dispatchSDKFixtureEvent(t, e, readOnlyFixtureEvent(t, sdkadapter.EventCurrentTime, 0))

	result := receiveTimeResult(t, "CurrentTime", resultCh)
	if result.err != nil {
		t.Fatalf("CurrentTime() error = %v", result.err)
	}
	want := time.Unix(1777736583, 0).UTC()
	if !result.ts.Equal(want) {
		t.Fatalf("CurrentTime() = %s, want %s", result.ts, want)
	}
	if got := client.Session().CurrentTime; !got.Equal(want) {
		t.Fatalf("Session().CurrentTime = %s, want %s", got, want)
	}
}

func TestCurrentTimeMillisUsesSDKCommandAndEvent(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	if err := e.sendSDKContext(context.Background(), sdkadapter.CurrentTimeMillisRequest{}); err != nil {
		t.Fatalf("sendSDKContext() error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 1 {
		t.Fatalf("commands len = %d, want 1", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandCurrentTimeMillis {
		t.Fatalf("command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandCurrentTimeMillis)
	}

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind:        sdkadapter.EventCurrentTimeMillis,
		CurrentTime: 1712345678123,
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage() error = %v", err)
	}
	got, ok := msg.(sdkadapter.CurrentTimeMillis)
	if !ok {
		t.Fatalf("sdkEventToMessage() type = %T, want sdkadapter.CurrentTimeMillis", msg)
	}
	if got.Time != "1712345678123" {
		t.Fatalf("CurrentTimeMillis time = %q, want 1712345678123", got.Time)
	}
}

func TestSDKCurrentTimeMillisPublicRouteReplaysOfficialFixture(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan struct {
		ts  time.Time
		err error
	}, 1)
	go func() {
		ts, err := client.CurrentTimeMillis(ctx)
		resultCh <- struct {
			ts  time.Time
			err error
		}{ts: ts, err: err}
	}()

	runNextEngineCommand(t, e)
	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandCurrentTimeMillis {
		t.Fatalf("command kind = %s, want %s", command.Kind, sdkadapter.CommandCurrentTimeMillis)
	}

	dispatchSDKFixtureEvent(t, e, fixtureEvent(t, "internal/sdkadapter/testdata/fixtures/official_sdk_current_time_millis_20260502.json", sdkadapter.EventCurrentTimeMillis, 0))

	result := receiveTimeResult(t, "CurrentTimeMillis", resultCh)
	if result.err != nil {
		t.Fatalf("CurrentTimeMillis() error = %v", result.err)
	}
	want := time.UnixMilli(1777736392854).UTC()
	if !result.ts.Equal(want) {
		t.Fatalf("CurrentTimeMillis() = %s, want %s", result.ts, want)
	}
	if got := client.Session().CurrentTime; !got.Equal(want) {
		t.Fatalf("Session().CurrentTime = %s, want %s", got, want)
	}
}

func TestCurrentTimeMillisEventUpdatesSnapshotAndSingleton(t *testing.T) {
	t.Parallel()

	handled := make(chan string, 1)
	e := &engine{
		singletons: make(map[string]*route),
		snapshot: Snapshot{
			State: StateReady,
		},
	}
	e.singletons[singletonCurrentTimeMillis] = &route{
		handle: func(msg any, _ *engine) {
			handled <- msg.(sdkadapter.CurrentTimeMillis).Time
		},
	}

	e.handleIncoming(sdkadapter.CurrentTimeMillis{Time: "1712345678123"})

	want := time.UnixMilli(1712345678123).UTC()
	if got := e.Session().CurrentTime; !got.Equal(want) {
		t.Fatalf("snapshot current time = %s, want %s", got, want)
	}

	select {
	case got := <-handled:
		if got != "1712345678123" {
			t.Fatalf("handled time = %q, want 1712345678123", got)
		}
	default:
		t.Fatal("current time millis singleton did not receive event")
	}
}

func TestNoReqIDAPIErrorsRouteToCurrentTimeSingleton(t *testing.T) {
	t.Parallel()

	e := &engine{
		singletons: make(map[string]*route),
	}
	handled := make(chan sdkadapter.APIError, 1)
	e.singletons[singletonCurrentTimeMillis] = &route{
		handleAPIErr: func(msg sdkadapter.APIError, _ *engine) {
			handled <- msg
		},
	}

	e.handleAPIError(sdkadapter.APIError{
		ReqID:   -1,
		Code:    503,
		Message: "current time millis unsupported",
	})

	select {
	case got := <-handled:
		if got.Code != 503 {
			t.Fatalf("API error code = %d, want 503", got.Code)
		}
	default:
		t.Fatal("no-reqID API error did not route to current time millis singleton")
	}
}

func receiveTimeResult(t *testing.T, name string, resultCh <-chan struct {
	ts  time.Time
	err error
}) struct {
	ts  time.Time
	err error
} {
	t.Helper()

	select {
	case result := <-resultCh:
		return result
	case <-time.After(time.Second):
		t.Fatalf("%s() did not return", name)
		return struct {
			ts  time.Time
			err error
		}{}
	}
}

package ibkr

import (
	"context"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
)

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

package sdkadapter

import (
	"context"
	"sync"
)

type ReplayAdapter struct {
	mu             sync.Mutex
	connected      bool
	closed         bool
	serverVersion  int
	connectionTime string
	events         []Event
	commands       []Command
	submitErr      error
	drainErr       error
}

func NewReplayAdapter(events []Event) *ReplayAdapter {
	return &ReplayAdapter{events: CloneEvents(events)}
}

func (a *ReplayAdapter) Connect(context.Context, ConnectRequest) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return ErrClosed
	}
	a.connected = true
	return nil
}

func (a *ReplayAdapter) Disconnect() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.connected = false
	return nil
}

func (a *ReplayAdapter) IsConnected() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.connected
}

func (a *ReplayAdapter) ServerVersion() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.serverVersion
}

func (a *ReplayAdapter) ConnectionTime() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.connectionTime
}

func (a *ReplayAdapter) Submit(_ context.Context, command Command) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.submitErr != nil {
		return a.submitErr
	}
	a.commands = append(a.commands, CloneCommand(command))
	return nil
}

func (a *ReplayAdapter) DrainEvents(_ context.Context, maxEvents int) ([]Event, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.drainErr != nil {
		return nil, a.drainErr
	}
	if maxEvents <= 0 || maxEvents > len(a.events) {
		maxEvents = len(a.events)
	}
	out := CloneEvents(a.events[:maxEvents])
	a.events = a.events[maxEvents:]
	return out, nil
}

func (a *ReplayAdapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closed = true
	a.connected = false
	return nil
}

func (a *ReplayAdapter) Commands() []Command {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]Command, len(a.commands))
	for i, command := range a.commands {
		out[i] = CloneCommand(command)
	}
	return out
}

func (a *ReplayAdapter) SetSubmitError(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.submitErr = err
}

func (a *ReplayAdapter) SetDrainError(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.drainErr = err
}

var ErrClosed = errorString("sdkadapter: closed")

type errorString string

func (e errorString) Error() string { return string(e) }

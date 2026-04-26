package sdkadapter

import (
	"context"
	"errors"
	"strconv"
	"time"
)

// Adapter is the Go-owned boundary to the repo-owned C++ SDK adapter. It
// carries copied command and event values only; SDK objects and callback
// lifetimes stay on the native side.
type Adapter interface {
	Connect(context.Context, ConnectRequest) error
	Disconnect() error
	IsConnected() bool
	ServerVersion() int
	ConnectionTime() string
	Submit(context.Context, Command) error
	DrainEvents(context.Context, int) ([]Event, error)
	Close() error
}

type ConnectRequest struct {
	Host      string
	Port      int
	ClientID  int
	Timeout   time.Duration
	QueueSize int
}

type CommandKind string

const (
	CommandCurrentTime          CommandKind = "current_time"
	CommandAccountSummary       CommandKind = "account_summary"
	CommandCancelAccountSummary CommandKind = "cancel_account_summary"
)

type Command struct {
	Kind  CommandKind
	ReqID int
	Group string
	Tags  []string
}

type EventKind string

const (
	EventConnectionMetadata EventKind = "connection_metadata"
	EventConnectionClosed   EventKind = "connection_closed"
	EventNextValidID        EventKind = "next_valid_id"
	EventManagedAccounts    EventKind = "managed_accounts"
	EventCurrentTime        EventKind = "current_time"
	EventAccountSummary     EventKind = "account_summary"
	EventAccountSummaryEnd  EventKind = "account_summary_end"
	EventAPIError           EventKind = "api_error"
	EventAdapterFatal       EventKind = "adapter_fatal"
)

type Event struct {
	Kind EventKind

	ReqID int

	ServerVersion  int
	ConnectionTime string
	NextValidID    int64
	Accounts       []string
	CurrentTime    int64

	AccountSummary AccountSummaryValue
	APIError       Error
	FatalMessage   string
}

type AccountSummaryValue struct {
	Account  string
	Tag      string
	Value    string
	Currency string
}

type Error struct {
	Op                      string
	ReqID                   int
	OrderID                 int64
	Code                    int
	Message                 string
	AdvancedOrderRejectJSON string
	Phase                   string
}

func (e Error) Error() string {
	if e.Op == "" {
		return e.Message
	}
	if e.Code != 0 {
		return e.Op + ": code=" + strconv.Itoa(e.Code) + ": " + e.Message
	}
	return e.Op + ": " + e.Message
}

var ErrUnsupportedCommand = errors.New("sdkadapter: unsupported command")

func CloneEvents(events []Event) []Event {
	out := make([]Event, len(events))
	for i, event := range events {
		out[i] = event
		out[i].Accounts = append([]string(nil), event.Accounts...)
	}
	return out
}

func CloneCommand(command Command) Command {
	command.Tags = append([]string(nil), command.Tags...)
	return command
}

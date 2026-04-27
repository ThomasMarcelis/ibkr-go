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

type BuildInfo struct {
	AdapterABIVersion string
	SDKAPIVersion     string
	Compiler          string
	ProtobufMode      string
}

type CommandKind string

const (
	CommandCurrentTime          CommandKind = "current_time"
	CommandCurrentTimeMillis    CommandKind = "current_time_millis"
	CommandAccountSummary       CommandKind = "account_summary"
	CommandCancelAccountSummary CommandKind = "cancel_account_summary"
	CommandContractDetails      CommandKind = "contract_details"
	CommandPositions            CommandKind = "positions"
	CommandCancelPositions      CommandKind = "cancel_positions"
)

type Command struct {
	Kind CommandKind

	CurrentTime          CurrentTimeRequest
	AccountSummary       AccountSummaryCommand
	CancelAccountSummary CancelAccountSummaryCommand
	ContractDetails      ContractDetailsCommand
	Positions            PositionsCommand
	CancelPositions      CancelPositionsCommand
}

type AccountSummaryCommand struct {
	ReqID int
	Group string
	Tags  []string
}

type CancelAccountSummaryCommand struct {
	ReqID int
}

type ContractDetailsCommand struct {
	ReqID    int
	Contract Contract
}

type PositionsCommand struct{}

type CancelPositionsCommand struct{}

type EventKind string

const (
	EventConnectionMetadata EventKind = "connection_metadata"
	EventConnectionClosed   EventKind = "connection_closed"
	EventNextValidID        EventKind = "next_valid_id"
	EventManagedAccounts    EventKind = "managed_accounts"
	EventCurrentTime        EventKind = "current_time"
	EventCurrentTimeMillis  EventKind = "current_time_millis"
	EventAccountSummary     EventKind = "account_summary"
	EventAccountSummaryEnd  EventKind = "account_summary_end"
	EventAPIError           EventKind = "api_error"
	EventAdapterFatal       EventKind = "adapter_fatal"
	EventContractDetails    EventKind = "contract_details"
	EventContractDetailsEnd EventKind = "contract_details_end"
	EventPosition           EventKind = "position"
	EventPositionEnd        EventKind = "position_end"
)

type Event struct {
	Kind EventKind

	ReqID int

	ServerVersion  int
	ConnectionTime string
	NextValidID    int64
	Accounts       []string
	CurrentTime    int64

	AccountSummary  AccountSummaryValue
	ContractDetails ContractDetailsValue
	Position        PositionValue
	APIError        Error
	FatalMessage    string
}

type ContractDetailsValue struct {
	Contract   Contract
	MarketName string
	MinTick    string
	LongName   string
	TimeZoneID string
}

type PositionValue struct {
	Account  string
	Contract Contract
	Position string
	AvgCost  string
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
	command.AccountSummary.Tags = append([]string(nil), command.AccountSummary.Tags...)
	return command
}

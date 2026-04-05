package session

import "time"

type State string

const (
	StateDisconnected State = "Disconnected"
	StateConnecting   State = "Connecting"
	StateHandshaking  State = "Handshaking"
	StateReady        State = "Ready"
	StateDegraded     State = "Degraded"
	StateReconnecting State = "Reconnecting"
	StateClosed       State = "Closed"
)

type OpKind string

const (
	OpContractDetails OpKind = "contract_details"
	OpHistoricalBars  OpKind = "historical_bars"
	OpAccountSummary  OpKind = "account_summary"
	OpPositions       OpKind = "positions"
	OpQuotes          OpKind = "quotes"
	OpRealTimeBars    OpKind = "realtime_bars"
	OpOpenOrders      OpKind = "open_orders"
	OpExecutions      OpKind = "executions"
)

type ReconnectPolicy string

const (
	ReconnectOff  ReconnectPolicy = "off"
	ReconnectAuto ReconnectPolicy = "auto"
)

type ResumePolicy string

const (
	ResumeNever ResumePolicy = "never"
	ResumeAuto  ResumePolicy = "auto"
)

type SlowConsumerPolicy string

const (
	SlowConsumerClose      SlowConsumerPolicy = "close"
	SlowConsumerDropOldest SlowConsumerPolicy = "drop_oldest"
)

type Event struct {
	At            time.Time
	State         State
	Previous      State
	ConnectionSeq uint64
	Code          int
	Message       string
	Err           error
}

type Snapshot struct {
	State           State
	ConnectionSeq   uint64
	ServerVersion   int
	ManagedAccounts []string
	NextValidID     int64
	CurrentTime     time.Time
}

type SubscriptionStateKind string

const (
	SubscriptionStarted          SubscriptionStateKind = "Started"
	SubscriptionSnapshotComplete SubscriptionStateKind = "SnapshotComplete"
	SubscriptionGap              SubscriptionStateKind = "Gap"
	SubscriptionResumed          SubscriptionStateKind = "Resumed"
	SubscriptionClosed           SubscriptionStateKind = "Closed"
)

type SubscriptionStateEvent struct {
	At            time.Time
	Kind          SubscriptionStateKind
	ConnectionSeq uint64
	Err           error
}

type Contract struct {
	Symbol          string
	SecType         string
	Exchange        string
	Currency        string
	PrimaryExchange string
	LocalSymbol     string
}

type ContractDetailsRequest struct {
	Contract Contract
}

type ContractDetails struct {
	Contract   Contract
	MarketName string
	MinTick    Decimal
	TimeZoneID string
}

type QualifiedContract struct {
	ContractDetails ContractDetails
}

type HistoricalBarsRequest struct {
	Contract   Contract
	EndTime    time.Time
	Duration   time.Duration
	BarSize    time.Duration
	WhatToShow string
	UseRTH     bool
}

type Bar struct {
	Time   time.Time
	Open   Decimal
	High   Decimal
	Low    Decimal
	Close  Decimal
	Volume Decimal
}

type AccountSummaryRequest struct {
	Account string
	Tags    []string
}

type AccountValue struct {
	Account  string
	Tag      string
	Value    string
	Currency string
}

type AccountSummaryUpdate struct {
	Value AccountValue
}

type Position struct {
	Account  string
	Contract Contract
	Position Decimal
	AvgCost  Decimal
}

type PositionUpdate struct {
	Position Position
}

type QuoteFields uint64

const (
	QuoteFieldBid QuoteFields = 1 << iota
	QuoteFieldAsk
	QuoteFieldLast
	QuoteFieldBidSize
	QuoteFieldAskSize
	QuoteFieldLastSize
	QuoteFieldOpen
	QuoteFieldHigh
	QuoteFieldLow
	QuoteFieldClose
	QuoteFieldMarketDataType
)

type MarketDataType int

type Quote struct {
	Available      QuoteFields
	Bid            Decimal
	Ask            Decimal
	Last           Decimal
	BidSize        Decimal
	AskSize        Decimal
	LastSize       Decimal
	Open           Decimal
	High           Decimal
	Low            Decimal
	Close          Decimal
	MarketDataType MarketDataType
}

type QuoteSubscriptionRequest struct {
	Contract     Contract
	Snapshot     bool
	GenericTicks []string
}

type QuoteUpdate struct {
	Snapshot   Quote
	Changed    QuoteFields
	ReceivedAt time.Time
}

type RealTimeBarsRequest struct {
	Contract   Contract
	WhatToShow string
	UseRTH     bool
}

type OpenOrdersScope string

const (
	OpenOrdersScopeAll    OpenOrdersScope = "all"
	OpenOrdersScopeClient OpenOrdersScope = "client"
	OpenOrdersScopeAuto   OpenOrdersScope = "auto"
)

type OpenOrder struct {
	OrderID   int64
	Account   string
	Contract  Contract
	Status    string
	Quantity  Decimal
	Filled    Decimal
	Remaining Decimal
}

type OpenOrderUpdate struct {
	Order OpenOrder
}

type ExecutionsRequest struct {
	Account string
	Symbol  string
}

type CommissionReport struct {
	ExecID      string
	Commission  Decimal
	Currency    string
	RealizedPNL Decimal
}

type Execution struct {
	ExecID  string
	Account string
	Symbol  string
	Side    string
	Shares  Decimal
	Price   Decimal
	Time    time.Time
}

type ExecutionUpdate struct {
	Execution  *Execution
	Commission *CommissionReport
}

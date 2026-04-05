package codec

type Message interface {
	messageName() string
}

type Contract struct {
	Symbol          string
	SecType         string
	Exchange        string
	Currency        string
	PrimaryExchange string
	LocalSymbol     string
}

type Hello struct {
	MinVersion int
	MaxVersion int
	ClientID   int
}

func (Hello) messageName() string { return "hello" }

type HelloAck struct {
	ServerVersion  int
	ConnectionTime string
}

func (HelloAck) messageName() string { return "hello_ack" }

type ManagedAccounts struct {
	Accounts []string
}

func (ManagedAccounts) messageName() string { return "managed_accounts" }

type NextValidID struct {
	OrderID int64
}

func (NextValidID) messageName() string { return "next_valid_id" }

type CurrentTime struct {
	Time string
}

func (CurrentTime) messageName() string { return "current_time" }

type APIError struct {
	ReqID   int
	Code    int
	Message string
}

func (APIError) messageName() string { return "api_error" }

type ContractDetailsRequest struct {
	ReqID    int
	Contract Contract
}

func (ContractDetailsRequest) messageName() string { return "req_contract_details" }

type ContractDetails struct {
	ReqID      int
	Contract   Contract
	MarketName string
	MinTick    string
	TimeZoneID string
}

func (ContractDetails) messageName() string { return "contract_details" }

type ContractDetailsEnd struct {
	ReqID int
}

func (ContractDetailsEnd) messageName() string { return "contract_details_end" }

type HistoricalBarsRequest struct {
	ReqID       int
	Contract    Contract
	EndDateTime string
	Duration    string
	BarSize     string
	WhatToShow  string
	UseRTH      bool
}

func (HistoricalBarsRequest) messageName() string { return "req_historical_bars" }

type HistoricalBar struct {
	ReqID  int
	Time   string
	Open   string
	High   string
	Low    string
	Close  string
	Volume string
}

func (HistoricalBar) messageName() string { return "historical_bar" }

type HistoricalBarsEnd struct {
	ReqID int
	Start string
	End   string
}

func (HistoricalBarsEnd) messageName() string { return "historical_bars_end" }

type AccountSummaryRequest struct {
	ReqID   int
	Account string
	Tags    []string
}

func (AccountSummaryRequest) messageName() string { return "req_account_summary" }

type CancelAccountSummary struct {
	ReqID int
}

func (CancelAccountSummary) messageName() string { return "cancel_account_summary" }

type AccountSummaryValue struct {
	ReqID    int
	Account  string
	Tag      string
	Value    string
	Currency string
}

func (AccountSummaryValue) messageName() string { return "account_summary" }

type AccountSummaryEnd struct {
	ReqID int
}

func (AccountSummaryEnd) messageName() string { return "account_summary_end" }

type PositionsRequest struct{}

func (PositionsRequest) messageName() string { return "req_positions" }

type CancelPositions struct{}

func (CancelPositions) messageName() string { return "cancel_positions" }

type Position struct {
	Account  string
	Contract Contract
	Position string
	AvgCost  string
}

func (Position) messageName() string { return "position" }

type PositionEnd struct{}

func (PositionEnd) messageName() string { return "position_end" }

type QuoteRequest struct {
	ReqID        int
	Contract     Contract
	Snapshot     bool
	GenericTicks []string
}

func (QuoteRequest) messageName() string { return "req_quote" }

type CancelQuote struct {
	ReqID int
}

func (CancelQuote) messageName() string { return "cancel_quote" }

type TickPrice struct {
	ReqID int
	Field string
	Price string
}

func (TickPrice) messageName() string { return "tick_price" }

type TickSize struct {
	ReqID int
	Field string
	Size  string
}

func (TickSize) messageName() string { return "tick_size" }

type MarketDataType struct {
	ReqID    int
	DataType int
}

func (MarketDataType) messageName() string { return "market_data_type" }

type TickSnapshotEnd struct {
	ReqID int
}

func (TickSnapshotEnd) messageName() string { return "tick_snapshot_end" }

type RealTimeBarsRequest struct {
	ReqID      int
	Contract   Contract
	WhatToShow string
	UseRTH     bool
}

func (RealTimeBarsRequest) messageName() string { return "req_realtime_bars" }

type CancelRealTimeBars struct {
	ReqID int
}

func (CancelRealTimeBars) messageName() string { return "cancel_realtime_bars" }

type RealTimeBar struct {
	ReqID  int
	Time   string
	Open   string
	High   string
	Low    string
	Close  string
	Volume string
}

func (RealTimeBar) messageName() string { return "realtime_bar" }

type OpenOrdersRequest struct {
	Scope string
}

func (OpenOrdersRequest) messageName() string { return "req_open_orders" }

type CancelOpenOrders struct{}

func (CancelOpenOrders) messageName() string { return "cancel_open_orders" }

type OpenOrder struct {
	OrderID   int64
	Account   string
	Contract  Contract
	Status    string
	Quantity  string
	Filled    string
	Remaining string
}

func (OpenOrder) messageName() string { return "open_order" }

type OpenOrderEnd struct{}

func (OpenOrderEnd) messageName() string { return "open_order_end" }

type ExecutionsRequest struct {
	ReqID   int
	Account string
	Symbol  string
}

func (ExecutionsRequest) messageName() string { return "req_executions" }

type ExecutionDetail struct {
	ReqID   int
	ExecID  string
	Account string
	Symbol  string
	Side    string
	Shares  string
	Price   string
	Time    string
}

func (ExecutionDetail) messageName() string { return "execution_detail" }

type ExecutionsEnd struct {
	ReqID int
}

func (ExecutionsEnd) messageName() string { return "executions_end" }

type CommissionReport struct {
	ExecID      string
	Commission  string
	Currency    string
	RealizedPNL string
}

func (CommissionReport) messageName() string { return "commission_report" }

package codec

type Message interface {
	messageName() string
}

// Contract holds the fields used for contract identification on the wire.
// The full TWS wire contract has 11 fields (conID, symbol, secType, expiry,
// strike, right, multiplier, exchange, currency, localSymbol, tradingClass).
// PrimaryExchange is used by some request/response messages outside the 11-field block.
type Contract struct {
	ConID           int
	Symbol          string
	SecType         string
	Expiry          string
	Strike          string
	Right           string
	Multiplier      string
	Exchange        string
	Currency        string
	LocalSymbol     string
	TradingClass    string
	PrimaryExchange string
}

type StartAPI struct {
	ClientID             int
	OptionalCapabilities string
}

func (StartAPI) messageName() string { return "start_api" }

type ServerInfo struct {
	ServerVersion  int
	ConnectionTime string
}

func (ServerInfo) messageName() string { return "server_info" }

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
	ReqID                   int
	Code                    int
	Message                 string
	AdvancedOrderRejectJSON string
	ErrorTimeMs             string
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
	LongName   string
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
	WAP    string
	Count  string
}

func (HistoricalBar) messageName() string { return "historical_bar" }

type HistoricalBarsEnd struct {
	ReqID int
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
	ReqID    int
	TickType int
	Price    string
	Size     string // companion size from the same frame
	AttrMask int    // tick attrib bitmask
}

func (TickPrice) messageName() string { return "tick_price" }

type TickSize struct {
	ReqID    int
	TickType int
	Size     string
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
	WAP    string
	Count  string
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
	Action    string
	OrderType string
	Status    string
	Quantity  string
	Filled    string
	Remaining string
}

func (OpenOrder) messageName() string { return "open_order" }

type OpenOrderEnd struct{}

func (OpenOrderEnd) messageName() string { return "open_order_end" }

type OrderStatus struct {
	OrderID   int64
	Status    string
	Filled    string
	Remaining string
}

func (OrderStatus) messageName() string { return "order_status" }

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

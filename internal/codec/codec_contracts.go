package codec

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

type MatchingSymbolsRequest struct {
	ReqID   int
	Pattern string
}

func (MatchingSymbolsRequest) messageName() string { return "req_matching_symbols" }

type SymbolSample struct {
	ConID              int
	Symbol             string
	SecType            string
	PrimaryExchange    string
	Currency           string
	DerivativeSecTypes []string
	Description        string
	IssuerID           string
}

type MatchingSymbols struct {
	ReqID   int
	Symbols []SymbolSample
}

func (MatchingSymbols) messageName() string { return "matching_symbols" }

type MarketRuleRequest struct {
	MarketRuleID int
}

func (MarketRuleRequest) messageName() string { return "req_market_rule" }

type PriceIncrement struct {
	LowEdge   string
	Increment string
}

type MarketRule struct {
	MarketRuleID int
	Increments   []PriceIncrement
}

func (MarketRule) messageName() string { return "market_rule" }

// SecDefOptParams (OUT 78 / IN 75+76)

type SecDefOptParamsRequest struct {
	ReqID             int
	UnderlyingSymbol  string
	FutFopExchange    string
	UnderlyingSecType string
	UnderlyingConID   int
}

func (SecDefOptParamsRequest) messageName() string { return "req_sec_def_opt_params" }

type SecDefOptParamsResponse struct {
	ReqID           int
	Exchange        string
	UnderlyingConID int
	TradingClass    string
	Multiplier      string
	Expirations     []string
	Strikes         []string
}

func (SecDefOptParamsResponse) messageName() string { return "sec_def_opt_params" }

type SecDefOptParamsEnd struct {
	ReqID int
}

func (SecDefOptParamsEnd) messageName() string { return "sec_def_opt_params_end" }

// SmartComponents (OUT 83 / IN 82)

type SmartComponentsRequest struct {
	ReqID       int
	BBOExchange string
}

func (SmartComponentsRequest) messageName() string { return "req_smart_components" }

type SmartComponentEntry struct {
	BitNumber      int
	ExchangeName   string
	ExchangeLetter string
}

type SmartComponentsResponse struct {
	ReqID      int
	Components []SmartComponentEntry
}

func (SmartComponentsResponse) messageName() string { return "smart_components" }

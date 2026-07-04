package codec

import (
	"fmt"
)

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

// v200 wire layout verified against live IB Gateway capture.
func decodeContractData(r *fieldReader) ([]Message, error) {
	// [10, reqID, symbol, secType, lastTradeDate, lastTradeDateOrContractMonth,
	//   strike, right, exchange, currency, localSymbol, marketName, tradingClass,
	//   conID, minTick, multiplier, orderTypes, validExchanges,
	//   priceMagnifier, underConID, longName, primaryExchange, contractMonth,
	//   industry, category, subcategory, timeZoneID, ...]
	// The slot after minTick is the contract multiplier: mdSizeMultiplier
	// left the wire at server version 164 (size rules), and the v200 captures carry
	// "100" there for AAPL options (20260405T214941Z, sha256 prefix
	// 3dcaf0b74a7c27a4) and "50" for ES futures (20260405T215018Z,
	// sha256 prefix e863bfbafe48370f).
	reqID, _ := r.ReadInt()
	symbol := r.ReadString()
	secType := r.ReadString()
	expiry := r.ReadString()
	r.Skip(1) // lastTradeDateOrContractMonth (duplicate/variant of expiry)
	strike := r.ReadString()
	right := r.ReadString()
	exchange := r.ReadString()
	currency := r.ReadString()
	localSymbol := r.ReadString()
	marketName := r.ReadString()
	tradingClass := r.ReadString()
	conID, _ := r.ReadInt()
	minTick := r.ReadString()
	multiplier := r.ReadString()
	r.Skip(4) // orderTypes, validExchanges, priceMagnifier, underConID
	longName := r.ReadString()
	primaryExchange := r.ReadString()
	r.Skip(4) // contractMonth, industry, category, subcategory
	timeZoneID := r.ReadString()
	return []Message{ContractDetails{
		ReqID: reqID,
		Contract: Contract{
			ConID: conID, Symbol: symbol, SecType: secType,
			Expiry: expiry, Strike: strike, Right: right,
			Multiplier: multiplier,
			Exchange:   exchange, Currency: currency,
			LocalSymbol: localSymbol, TradingClass: tradingClass,
			PrimaryExchange: primaryExchange,
		},
		MarketName: marketName, MinTick: minTick,
		LongName: longName, TimeZoneID: timeZoneID,
	}}, nil
}

// [52, version, reqID]
func decodeContractDataEnd(r *fieldReader) ([]Message, error) {
	r.Skip(1)
	reqID, _ := r.ReadInt()
	return []Message{ContractDetailsEnd{ReqID: reqID}}, nil
}

// [75, reqID, exchange, underlyingConID, tradingClass, multiplier, expirationsCount, expirations..., strikesCount, strikes...] — no version
func decodeSecDefOptParams(r *fieldReader) ([]Message, error) {
	// Live server_version 200 frames carry the expiration count directly
	// after the multiplier (capture 20260611T074417Z, sha fa7f3f46793d3277);
	// a phantom marketRuleId skip here used to consume the count and kill
	// the session on the first row.
	reqID, _ := r.ReadInt()
	exchange := r.ReadString()
	underConID, _ := r.ReadInt()
	tradingClass := r.ReadString()
	multiplier := r.ReadString()
	expirationCount, err := r.ReadCount("expiration count")
	if err != nil {
		return nil, err
	}
	if expirationCount > r.Remaining() {
		return nil, fmt.Errorf("codec: sec def opt params: expiration count %d exceeds remaining fields %d", expirationCount, r.Remaining())
	}
	expirations := make([]string, expirationCount)
	for i := range expirations {
		expirations[i] = r.ReadString()
	}
	strikeCount, err := r.ReadCount("strike count")
	if err != nil {
		return nil, err
	}
	if strikeCount > r.Remaining() {
		return nil, fmt.Errorf("codec: sec def opt params: strike count %d exceeds remaining fields %d", strikeCount, r.Remaining())
	}
	strikes := make([]string, strikeCount)
	for i := range strikes {
		strikes[i] = r.ReadString()
	}
	return []Message{SecDefOptParamsResponse{
		ReqID: reqID, Exchange: exchange, UnderlyingConID: underConID,
		TradingClass: tradingClass, Multiplier: multiplier,
		Expirations: expirations, Strikes: strikes,
	}}, nil
}

// [76, reqID] — no version
func decodeSecDefOptParamsEnd(r *fieldReader) ([]Message, error) {
	reqID, _ := r.ReadInt()
	return []Message{SecDefOptParamsEnd{ReqID: reqID}}, nil
}

// [79, reqID, count, repeated(conID, symbol, secType, primaryExch, currency, derivCount, derivTypes...)]
func decodeSymbolSamples(r *fieldReader) ([]Message, error) {
	reqID, _ := r.ReadInt()
	count, err := r.ReadCount("sample count")
	if err != nil {
		return nil, err
	}
	if count > r.Remaining()/6 {
		return nil, fmt.Errorf("codec: symbol samples: count %d exceeds minimum available fields %d", count, r.Remaining())
	}
	symbols := make([]SymbolSample, count)
	for i := range symbols {
		conID, _ := r.ReadInt()
		symbol := r.ReadString()
		secType := r.ReadString()
		primaryExch := r.ReadString()
		currency := r.ReadString()
		derivCount, _ := r.ReadInt()
		derivTypes := make([]string, derivCount)
		for j := range derivTypes {
			derivTypes[j] = r.ReadString()
		}
		description := ""
		issuerID := ""
		if r.Remaining() >= 2 && !isWireInt(r.fields[r.pos]) {
			description = r.ReadString()
			issuerID = r.ReadString()
		}
		symbols[i] = SymbolSample{
			ConID: conID, Symbol: symbol, SecType: secType,
			PrimaryExchange: primaryExch, Currency: currency,
			DerivativeSecTypes: derivTypes,
			Description:        description, IssuerID: issuerID,
		}
	}
	return []Message{MatchingSymbols{ReqID: reqID, Symbols: symbols}}, nil
}

// [82, reqID, count, repeated(bitNumber, exchangeName, exchangeLetter)]
func decodeSmartComponents(r *fieldReader) ([]Message, error) {
	reqID, _ := r.ReadInt()
	count, err := r.ReadCount("smart component count")
	if err != nil {
		return nil, err
	}
	if err := r.RequireFixedEntryFields("smart components", count, 3, 0); err != nil {
		return nil, err
	}
	components := make([]SmartComponentEntry, count)
	for i := range components {
		bitNumber, _ := r.ReadInt()
		exchangeName := r.ReadString()
		exchangeLetter := r.ReadString()
		components[i] = SmartComponentEntry{BitNumber: bitNumber, ExchangeName: exchangeName, ExchangeLetter: exchangeLetter}
	}
	return []Message{SmartComponentsResponse{ReqID: reqID, Components: components}}, nil
}

// [92, marketRuleId, count, repeated(lowEdge, increment)] — no version
func decodeMarketRule(r *fieldReader) ([]Message, error) {
	marketRuleID, _ := r.ReadInt()
	count, err := r.ReadCount("market rule increment count")
	if err != nil {
		return nil, err
	}
	if err := r.RequireFixedEntryFields("market rule", count, 2, 0); err != nil {
		return nil, err
	}
	increments := make([]PriceIncrement, count)
	for i := range increments {
		increments[i] = PriceIncrement{LowEdge: r.ReadString(), Increment: r.ReadString()}
	}
	return []Message{MarketRule{MarketRuleID: marketRuleID, Increments: increments}}, nil
}

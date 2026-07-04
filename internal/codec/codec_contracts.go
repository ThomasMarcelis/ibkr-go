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

func (m ContractDetailsRequest) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(OutReqContractData)
	w.WriteInt(8) // version
	w.WriteInt(m.ReqID)
	w.WriteInt(m.Contract.ConID)
	writeWireContract(&w, m.Contract)
	w.WriteBool(false) // includeExpired
	w.WriteString("")  // secIdType
	w.WriteString("")  // secId
	w.WriteString("")  // issuerId (BOND_ISSUER_ID 176, always present in 176..200)
	return w.Fields(), nil
}

type ContractDetails struct {
	ReqID      int
	Contract   Contract
	MarketName string
	MinTick    string
	LongName   string
	TimeZoneID string
}

type ContractDetailsEnd struct {
	ReqID int
}

type MatchingSymbolsRequest struct {
	ReqID   int
	Pattern string
}

func (m MatchingSymbolsRequest) encodeWire(sv int) ([]string, error) {
	return []string{itoa(OutReqMatchingSymbols), itoa(m.ReqID), m.Pattern}, nil
}

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

type MarketRuleRequest struct {
	MarketRuleID int
}

func (m MarketRuleRequest) encodeWire(sv int) ([]string, error) {
	return []string{itoa(OutReqMarketRule), itoa(m.MarketRuleID)}, nil
}

type PriceIncrement struct {
	LowEdge   string
	Increment string
}

type MarketRule struct {
	MarketRuleID int
	Increments   []PriceIncrement
}

// SecDefOptParams (OUT 78 / IN 75+76)

type SecDefOptParamsRequest struct {
	ReqID             int
	UnderlyingSymbol  string
	FutFopExchange    string
	UnderlyingSecType string
	UnderlyingConID   int
}

func (m SecDefOptParamsRequest) encodeWire(sv int) ([]string, error) {
	return []string{itoa(OutReqSecDefOptParams), itoa(m.ReqID), m.UnderlyingSymbol, m.FutFopExchange, m.UnderlyingSecType, itoa(m.UnderlyingConID)}, nil
}

type SecDefOptParamsResponse struct {
	ReqID           int
	Exchange        string
	UnderlyingConID int
	TradingClass    string
	Multiplier      string
	Expirations     []string
	Strikes         []string
}

type SecDefOptParamsEnd struct {
	ReqID int
}

// SmartComponents (OUT 83 / IN 82)

type SmartComponentsRequest struct {
	ReqID       int
	BBOExchange string
}

func (m SmartComponentsRequest) encodeWire(sv int) ([]string, error) {
	return []string{itoa(OutReqSmartComponents), itoa(m.ReqID), m.BBOExchange}, nil
}

type SmartComponentEntry struct {
	BitNumber      int
	ExchangeName   string
	ExchangeLetter string
}

type SmartComponentsResponse struct {
	ReqID      int
	Components []SmartComponentEntry
}

// v200 wire layout verified against live IB Gateway capture.
func decodeContractData(r *fieldReader, sv int) ([]Message, error) {
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
	expiry := r.ReadString() // lastTradeDateOrContractMonth (readLastTradeDate)
	if sv >= MinServerVersionLastTradeDate {
		r.Skip(1) // explicit lastTradeDate (decoder.py:509-510)
	}
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

func (m ContractDetails) encodeWire(sv int) ([]string, error) {
	return []string{
		itoa(InContractData), itoa(m.ReqID),
		m.Contract.Symbol, m.Contract.SecType, m.Contract.Expiry,
		m.Contract.Expiry, // lastTradeDateOrContractMonth (duplicate)
		m.Contract.Strike, m.Contract.Right,
		m.Contract.Exchange, m.Contract.Currency,
		m.Contract.LocalSymbol, m.MarketName, m.Contract.TradingClass,
		itoa(m.Contract.ConID), m.MinTick,
		m.Contract.Multiplier, "", "", "", "",
		m.LongName, m.Contract.PrimaryExchange,
		"", "", "", "",
		m.TimeZoneID,
	}, nil
}

// [52, version, reqID]
func decodeContractDataEnd(r *fieldReader, sv int) ([]Message, error) {
	r.Skip(1)
	reqID, _ := r.ReadInt()
	return []Message{ContractDetailsEnd{ReqID: reqID}}, nil
}

func (m ContractDetailsEnd) encodeWire(sv int) ([]string, error) {
	return []string{itoa(InContractDataEnd), "1", itoa(m.ReqID)}, nil
}

// [75, reqID, exchange, underlyingConID, tradingClass, multiplier, expirationsCount, expirations..., strikesCount, strikes...] — no version
func decodeSecDefOptParams(r *fieldReader, sv int) ([]Message, error) {
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

func (m SecDefOptParamsResponse) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(InSecDefOptParams)
	w.WriteInt(m.ReqID)
	w.WriteString(m.Exchange)
	w.WriteInt(m.UnderlyingConID)
	w.WriteString(m.TradingClass)
	w.WriteString(m.Multiplier)
	w.WriteInt(len(m.Expirations))
	for _, exp := range m.Expirations {
		w.WriteString(exp)
	}
	w.WriteInt(len(m.Strikes))
	for _, strike := range m.Strikes {
		w.WriteString(strike)
	}
	return w.Fields(), nil
}

// [76, reqID] — no version
func decodeSecDefOptParamsEnd(r *fieldReader, sv int) ([]Message, error) {
	reqID, _ := r.ReadInt()
	return []Message{SecDefOptParamsEnd{ReqID: reqID}}, nil
}

func (m SecDefOptParamsEnd) encodeWire(sv int) ([]string, error) {
	return []string{itoa(InSecDefOptParamsEnd), itoa(m.ReqID)}, nil
}

// [79, reqID, count, repeated(conID, symbol, secType, primaryExch, currency, derivCount, derivTypes...)]
func decodeSymbolSamples(r *fieldReader, sv int) ([]Message, error) {
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
		if r.Remaining() >= 2 && !isWireInt(string(r.peek())) {
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

func (m MatchingSymbols) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(InSymbolSamples)
	w.WriteInt(m.ReqID)
	w.WriteInt(len(m.Symbols))
	for _, s := range m.Symbols {
		w.WriteInt(s.ConID)
		w.WriteString(s.Symbol)
		w.WriteString(s.SecType)
		w.WriteString(s.PrimaryExchange)
		w.WriteString(s.Currency)
		w.WriteInt(len(s.DerivativeSecTypes))
		for _, dt := range s.DerivativeSecTypes {
			w.WriteString(dt)
		}
		w.WriteString(s.Description)
		w.WriteString(s.IssuerID)
	}
	return w.Fields(), nil
}

// [82, reqID, count, repeated(bitNumber, exchangeName, exchangeLetter)]
func decodeSmartComponents(r *fieldReader, sv int) ([]Message, error) {
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

func (m SmartComponentsResponse) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(InSmartComponents)
	w.WriteInt(m.ReqID)
	w.WriteInt(len(m.Components))
	for _, c := range m.Components {
		w.WriteInt(c.BitNumber)
		w.WriteString(c.ExchangeName)
		w.WriteString(c.ExchangeLetter)
	}
	return w.Fields(), nil
}

// [92, marketRuleId, count, repeated(lowEdge, increment)] — no version
func decodeMarketRule(r *fieldReader, sv int) ([]Message, error) {
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

func (m MarketRule) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(InMarketRule)
	w.WriteInt(m.MarketRuleID)
	w.WriteInt(len(m.Increments))
	for _, inc := range m.Increments {
		w.WriteString(inc.LowEdge)
		w.WriteString(inc.Increment)
	}
	return w.Fields(), nil
}

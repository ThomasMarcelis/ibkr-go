package codec

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
)

// Contract is the canonical internal contract model. Classic encoders select
// the request-specific subset they own; protobuf encoders use the complete
// shared Contract schema.
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
	IncludeExpired  bool
	SecurityIDType  string
	SecurityID      string
	IssuerID        string
	ComboLegs       []ComboLeg
	DeltaNeutral    *DeltaNeutralContract
}

type DeltaNeutralContract struct {
	ConID int
	Delta string
	Price string
}

type ContractDetailsRequest struct {
	ReqID    int
	Contract Contract
}

func (m ContractDetailsRequest) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(protocol.OutReqContractData)
	w.WriteInt(8) // version
	w.WriteInt(m.ReqID)
	w.WriteInt(m.Contract.ConID)
	writeWireContract(&w, m.Contract)
	w.WriteBool(m.Contract.IncludeExpired)
	w.WriteString(m.Contract.SecurityIDType)
	w.WriteString(m.Contract.SecurityID)
	w.WriteString(m.Contract.IssuerID)
	return w.Fields(), nil
}

type ContractDetails struct {
	ReqID                     int
	Contract                  Contract
	MarketName                string
	MinTick                   string
	PriceMagnifier            int
	OrderTypes                string
	ValidExchanges            string
	UnderConID                int
	LongName                  string
	ContractMonth             string
	Industry                  string
	Category                  string
	Subcategory               string
	TimeZoneID                string
	TradingHours              string
	LiquidHours               string
	EconomicValueRule         string
	EconomicValueMultiplier   string
	SecurityIDs               []TagValue
	AggGroup                  int
	UnderSymbol               string
	UnderSecType              string
	MarketRuleIDs             string
	RealExpirationDate        string
	LastTradeDate             string
	LastTradeTime             string
	StockType                 string
	MinSize                   string
	SizeIncrement             string
	SuggestedSizeIncrement    string
	EventContract1            string
	EventContractDescription1 string
	EventContractDescription2 string
	MinAlgoSize               string
	LastPricePrecision        string
	LastSizePrecision         string
	Fund                      *FundDetails
	IneligibilityReasons      []IneligibilityReason
}

// BondContractDetails is the distinct classic message-18 response used for
// bonds. ContractDetails carries the common fields shared with message 10;
// the remaining fields retain the official bond callback shape.
type BondContractDetails struct {
	ContractDetails
	CUSIP             string
	Coupon            string
	Maturity          string
	IssueDate         string
	Ratings           string
	BondType          string
	CouponType        string
	Convertible       bool
	Callable          bool
	Putable           bool
	DescriptionAppend string
	NextOptionDate    string
	NextOptionType    string
	NextOptionPartial bool
	Notes             string
}

type FundDetails struct {
	Name                      string
	Family                    string
	Type                      string
	FrontLoad                 string
	BackLoad                  string
	BackLoadTimeInterval      string
	ManagementFee             string
	Closed                    bool
	ClosedForNewInvestors     bool
	ClosedForNewMoney         bool
	NotifyAmount              string
	MinimumInitialPurchase    string
	MinimumSubsequentPurchase string
	BlueSkyStates             string
	BlueSkyTerritories        string
	DistributionPolicy        string
	AssetType                 string
}

type IneligibilityReason struct {
	ID          string
	Description string
}

type ContractDetailsEnd struct {
	ReqID int
}

type MatchingSymbolsRequest struct {
	ReqID   int
	Pattern string
}

func (m MatchingSymbolsRequest) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutReqMatchingSymbols), itoa(m.ReqID), m.Pattern}, nil
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
	return []string{itoa(protocol.OutReqMarketRule), itoa(m.MarketRuleID)}, nil
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
	return []string{itoa(protocol.OutReqSecDefOptParams), itoa(m.ReqID), m.UnderlyingSymbol, m.FutFopExchange, m.UnderlyingSecType, itoa(m.UnderlyingConID)}, nil
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
	return []string{itoa(protocol.OutReqSmartComponents), itoa(m.ReqID), m.BBOExchange}, nil
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

// Server-version 200 classic layout. Explicit codec literals freeze live stock,
// option, future, and mutual-fund responses; index and crypto responses are
// additionally present in the checked-in capture corpus.
func decodeContractData(r *fieldReader, sv int) ([]Message, error) {
	reqID, _ := r.ReadInt()
	symbol := r.ReadString()
	secType := r.ReadString()
	expiry, lastTradeTime := splitLastTradeDate(r.ReadString())
	lastTradeDate := r.ReadString()
	strike := r.ReadString()
	right := r.ReadString()
	exchange := r.ReadString()
	currency := r.ReadString()
	localSymbol := r.ReadString()
	marketName := r.ReadString()
	tradingClass := r.ReadString()
	conID, _ := r.ReadInt()
	minTick := r.ReadString()
	// mdSizeMultiplier left the wire at SIZE_RULES (164). In every supported
	// server version this slot is the contract multiplier.
	multiplier := r.ReadString()
	orderTypes := r.ReadString()
	validExchanges := r.ReadString()
	priceMagnifier, _ := r.ReadInt()
	underConID, _ := r.ReadInt()
	longName := decodeUnicodeEscapes(r.ReadString())
	primaryExchange := r.ReadString()
	contractMonth := r.ReadString()
	industry := r.ReadString()
	category := r.ReadString()
	subcategory := r.ReadString()
	timeZoneID := r.ReadString()
	tradingHours := r.ReadString()
	liquidHours := r.ReadString()
	economicValueRule := r.ReadString()
	economicValueMultiplier := r.ReadString()

	securityIDCount, err := r.ReadCount("contract security id count")
	if err != nil {
		return nil, err
	}
	trailerFields := 10 // fixed tail plus ineligibility-reason count
	if secType == "FUND" {
		trailerFields += 17
	}
	if err := r.RequireFixedEntryFields("contract security ids", securityIDCount, 2, trailerFields); err != nil {
		return nil, err
	}
	var securityIDs []TagValue
	if securityIDCount > 0 {
		securityIDs = make([]TagValue, securityIDCount)
	}
	for i := range securityIDs {
		securityIDs[i] = TagValue{Tag: r.ReadString(), Value: r.ReadString()}
	}

	aggGroup, _ := r.ReadInt()
	underSymbol := r.ReadString()
	underSecType := r.ReadString()
	marketRuleIDs := r.ReadString()
	realExpirationDate := r.ReadString()
	stockType := r.ReadString()
	minSize := r.ReadDecimal()
	sizeIncrement := r.ReadDecimal()
	suggestedSizeIncrement := r.ReadDecimal()

	var fund *FundDetails
	if secType == "FUND" {
		fund = &FundDetails{
			Name:                 r.ReadString(),
			Family:               r.ReadString(),
			Type:                 r.ReadString(),
			FrontLoad:            r.ReadString(),
			BackLoad:             r.ReadString(),
			BackLoadTimeInterval: r.ReadString(),
			ManagementFee:        r.ReadString(),
		}
		fund.Closed, _ = r.ReadBool()
		fund.ClosedForNewInvestors, _ = r.ReadBool()
		fund.ClosedForNewMoney, _ = r.ReadBool()
		fund.NotifyAmount = r.ReadString()
		fund.MinimumInitialPurchase = r.ReadString()
		fund.MinimumSubsequentPurchase = r.ReadString()
		fund.BlueSkyStates = r.ReadString()
		fund.BlueSkyTerritories = r.ReadString()
		fund.DistributionPolicy = r.ReadString()
		fund.AssetType = r.ReadString()
	}

	count, err := r.ReadCount("contract ineligibility reason count")
	if err != nil {
		return nil, err
	}
	if err := r.RequireFixedEntryFields("contract ineligibility reasons", count, 2, 0); err != nil {
		return nil, err
	}
	var ineligibilityReasons []IneligibilityReason
	if count > 0 {
		ineligibilityReasons = make([]IneligibilityReason, count)
	}
	for i := range ineligibilityReasons {
		ineligibilityReasons[i] = IneligibilityReason{ID: r.ReadString(), Description: r.ReadString()}
	}
	if remaining := r.Remaining(); remaining != 0 {
		return nil, fmt.Errorf("ibkr codec: contract details has %d trailing fields", remaining)
	}

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
		PriceMagnifier: priceMagnifier, OrderTypes: orderTypes,
		ValidExchanges: validExchanges, UnderConID: underConID,
		LongName: longName, ContractMonth: contractMonth,
		Industry: industry, Category: category, Subcategory: subcategory,
		TimeZoneID: timeZoneID, TradingHours: tradingHours, LiquidHours: liquidHours,
		EconomicValueRule: economicValueRule, EconomicValueMultiplier: economicValueMultiplier,
		SecurityIDs: securityIDs, AggGroup: aggGroup,
		UnderSymbol: underSymbol, UnderSecType: underSecType,
		MarketRuleIDs: marketRuleIDs, RealExpirationDate: realExpirationDate,
		LastTradeDate: lastTradeDate, LastTradeTime: lastTradeTime, StockType: stockType,
		MinSize: minSize, SizeIncrement: sizeIncrement, SuggestedSizeIncrement: suggestedSizeIncrement,
		Fund: fund, IneligibilityReasons: ineligibilityReasons,
	}}, nil
}

func (m ContractDetails) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(protocol.InContractData)
	w.WriteInt(m.ReqID)
	w.WriteString(m.Contract.Symbol)
	w.WriteString(m.Contract.SecType)
	lastTradeDate := m.Contract.Expiry
	if m.LastTradeTime != "" && !strings.ContainsAny(lastTradeDate, " -") {
		lastTradeDate += " " + m.LastTradeTime
		if m.TimeZoneID != "" {
			lastTradeDate += " " + m.TimeZoneID
		}
	}
	w.WriteString(lastTradeDate)
	explicitLastTradeDate := m.LastTradeDate
	if explicitLastTradeDate == "" {
		explicitLastTradeDate, _ = splitLastTradeDate(m.Contract.Expiry)
	}
	w.WriteString(explicitLastTradeDate)
	w.WriteString(m.Contract.Strike)
	w.WriteString(m.Contract.Right)
	w.WriteString(m.Contract.Exchange)
	w.WriteString(m.Contract.Currency)
	w.WriteString(m.Contract.LocalSymbol)
	w.WriteString(m.MarketName)
	w.WriteString(m.Contract.TradingClass)
	w.WriteInt(m.Contract.ConID)
	w.WriteString(m.MinTick)
	w.WriteString(m.Contract.Multiplier)
	w.WriteString(m.OrderTypes)
	w.WriteString(m.ValidExchanges)
	w.WriteInt(m.PriceMagnifier)
	w.WriteInt(m.UnderConID)
	w.WriteString(m.LongName)
	w.WriteString(m.Contract.PrimaryExchange)
	w.WriteString(m.ContractMonth)
	w.WriteString(m.Industry)
	w.WriteString(m.Category)
	w.WriteString(m.Subcategory)
	w.WriteString(m.TimeZoneID)
	w.WriteString(m.TradingHours)
	w.WriteString(m.LiquidHours)
	w.WriteString(m.EconomicValueRule)
	w.WriteString(m.EconomicValueMultiplier)
	w.WriteInt(len(m.SecurityIDs))
	for _, id := range m.SecurityIDs {
		w.WriteString(id.Tag)
		w.WriteString(id.Value)
	}
	w.WriteInt(m.AggGroup)
	w.WriteString(m.UnderSymbol)
	w.WriteString(m.UnderSecType)
	w.WriteString(m.MarketRuleIDs)
	w.WriteString(m.RealExpirationDate)
	w.WriteString(m.StockType)
	w.WriteDecimal(m.MinSize)
	w.WriteDecimal(m.SizeIncrement)
	w.WriteDecimal(m.SuggestedSizeIncrement)
	if m.Contract.SecType == "FUND" {
		fund := m.Fund
		if fund == nil {
			fund = &FundDetails{}
		}
		w.WriteString(fund.Name)
		w.WriteString(fund.Family)
		w.WriteString(fund.Type)
		w.WriteString(fund.FrontLoad)
		w.WriteString(fund.BackLoad)
		w.WriteString(fund.BackLoadTimeInterval)
		w.WriteString(fund.ManagementFee)
		w.WriteBool(fund.Closed)
		w.WriteBool(fund.ClosedForNewInvestors)
		w.WriteBool(fund.ClosedForNewMoney)
		w.WriteString(fund.NotifyAmount)
		w.WriteString(fund.MinimumInitialPurchase)
		w.WriteString(fund.MinimumSubsequentPurchase)
		w.WriteString(fund.BlueSkyStates)
		w.WriteString(fund.BlueSkyTerritories)
		w.WriteString(fund.DistributionPolicy)
		w.WriteString(fund.AssetType)
	}
	w.WriteInt(len(m.IneligibilityReasons))
	for _, reason := range m.IneligibilityReasons {
		w.WriteString(reason.ID)
		w.WriteString(reason.Description)
	}
	return w.Fields(), nil
}

// Server-version 200 classic bond layout. Live frames are frozen
// in codec_capture_test.go; API 10.48.01 processBondContractDataMsg is the
// source reference for field order and version gates.
func decodeBondContractData(r *fieldReader, sv int) ([]Message, error) {
	reqID, _ := r.ReadInt()
	symbol := r.ReadString()
	secType := r.ReadString()
	cusip := r.ReadString()
	coupon := r.ReadDecimal()
	maturity, lastTradeTime, _ := splitBondLastTradeDate(r.ReadString())
	issueDate := r.ReadString()
	ratings := r.ReadString()
	bondType := r.ReadString()
	couponType := r.ReadString()
	convertible, _ := r.ReadBool()
	callable, _ := r.ReadBool()
	putable, _ := r.ReadBool()
	descriptionAppend := r.ReadString()
	exchange := r.ReadString()
	currency := r.ReadString()
	marketName := r.ReadString()
	tradingClass := r.ReadString()
	conID, _ := r.ReadInt()
	minTick := r.ReadDecimal()
	orderTypes := r.ReadString()
	validExchanges := r.ReadString()
	nextOptionDate := r.ReadString()
	nextOptionType := r.ReadString()
	nextOptionPartial, _ := r.ReadBool()
	notes := r.ReadString()
	longName := decodeUnicodeEscapes(r.ReadString())
	timeZoneID := r.ReadString()
	tradingHours := r.ReadString()
	liquidHours := r.ReadString()
	economicValueRule := r.ReadString()
	economicValueMultiplier := r.ReadDecimal()

	securityIDCount, err := r.ReadCount("bond contract security id count")
	if err != nil {
		return nil, err
	}
	if err := r.RequireFixedEntryFields("bond contract security ids", securityIDCount, 2, 5); err != nil {
		return nil, err
	}
	var securityIDs []TagValue
	if securityIDCount > 0 {
		securityIDs = make([]TagValue, securityIDCount)
	}
	for i := range securityIDs {
		securityIDs[i] = TagValue{Tag: r.ReadString(), Value: r.ReadString()}
	}

	aggGroup, _ := r.ReadInt()
	marketRuleIDs := r.ReadString()
	minSize := r.ReadDecimal()
	sizeIncrement := r.ReadDecimal()
	suggestedSizeIncrement := r.ReadDecimal()
	if remaining := r.Remaining(); remaining != 0 {
		return nil, fmt.Errorf("ibkr codec: bond contract details has %d trailing fields", remaining)
	}

	return []Message{BondContractDetails{
		ContractDetails: ContractDetails{
			ReqID: reqID,
			Contract: Contract{
				ConID: conID, Symbol: symbol, SecType: secType,
				Exchange: exchange, Currency: currency, TradingClass: tradingClass,
			},
			MarketName: marketName, MinTick: minTick,
			OrderTypes: orderTypes, ValidExchanges: validExchanges,
			LongName: longName, TimeZoneID: timeZoneID,
			TradingHours: tradingHours, LiquidHours: liquidHours,
			EconomicValueRule: economicValueRule, EconomicValueMultiplier: economicValueMultiplier,
			SecurityIDs: securityIDs, AggGroup: aggGroup,
			MarketRuleIDs: marketRuleIDs, LastTradeTime: lastTradeTime,
			MinSize: minSize, SizeIncrement: sizeIncrement, SuggestedSizeIncrement: suggestedSizeIncrement,
		},
		CUSIP: cusip, Coupon: coupon, Maturity: maturity, IssueDate: issueDate,
		Ratings: ratings, BondType: bondType, CouponType: couponType,
		Convertible: convertible, Callable: callable, Putable: putable,
		DescriptionAppend: descriptionAppend,
		NextOptionDate:    nextOptionDate, NextOptionType: nextOptionType,
		NextOptionPartial: nextOptionPartial, Notes: notes,
	}}, nil
}

func (m BondContractDetails) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(protocol.InBondContractData)
	w.WriteInt(m.ReqID)
	w.WriteString(m.Contract.Symbol)
	w.WriteString(m.Contract.SecType)
	w.WriteString(m.CUSIP)
	w.WriteDecimal(m.Coupon)
	maturity := m.Maturity
	if m.LastTradeTime != "" && !strings.ContainsAny(maturity, " -") {
		maturity += " " + m.LastTradeTime
		if m.TimeZoneID != "" {
			maturity += " " + m.TimeZoneID
		}
	}
	w.WriteString(maturity)
	w.WriteString(m.IssueDate)
	w.WriteString(m.Ratings)
	w.WriteString(m.BondType)
	w.WriteString(m.CouponType)
	w.WriteBool(m.Convertible)
	w.WriteBool(m.Callable)
	w.WriteBool(m.Putable)
	w.WriteString(m.DescriptionAppend)
	w.WriteString(m.Contract.Exchange)
	w.WriteString(m.Contract.Currency)
	w.WriteString(m.MarketName)
	w.WriteString(m.Contract.TradingClass)
	w.WriteInt(m.Contract.ConID)
	w.WriteDecimal(m.MinTick)
	w.WriteString(m.OrderTypes)
	w.WriteString(m.ValidExchanges)
	w.WriteString(m.NextOptionDate)
	w.WriteString(m.NextOptionType)
	w.WriteBool(m.NextOptionPartial)
	w.WriteString(m.Notes)
	w.WriteString(m.LongName)
	w.WriteString(m.TimeZoneID)
	w.WriteString(m.TradingHours)
	w.WriteString(m.LiquidHours)
	w.WriteString(m.EconomicValueRule)
	w.WriteDecimal(m.EconomicValueMultiplier)
	w.WriteInt(len(m.SecurityIDs))
	for _, id := range m.SecurityIDs {
		w.WriteString(id.Tag)
		w.WriteString(id.Value)
	}
	w.WriteInt(m.AggGroup)
	w.WriteString(m.MarketRuleIDs)
	w.WriteDecimal(m.MinSize)
	w.WriteDecimal(m.SizeIncrement)
	w.WriteDecimal(m.SuggestedSizeIncrement)
	return w.Fields(), nil
}

func splitLastTradeDate(value string) (date, tradeTime string) {
	var fields []string
	if strings.Contains(value, "-") {
		fields = strings.Split(value, "-")
	} else {
		fields = strings.Fields(value)
	}
	if len(fields) > 0 {
		date = fields[0]
	}
	if len(fields) > 1 {
		tradeTime = fields[1]
	}
	return date, tradeTime
}

func splitBondLastTradeDate(value string) (maturity, tradeTime, timeZone string) {
	var fields []string
	if strings.Contains(value, "-") {
		fields = strings.Split(value, "-")
	} else {
		fields = strings.Fields(value)
	}
	if len(fields) > 0 {
		maturity = fields[0]
	}
	if len(fields) > 1 {
		tradeTime = fields[1]
	}
	if len(fields) > 2 {
		timeZone = fields[2]
	}
	return maturity, tradeTime, timeZone
}

// decodeUnicodeEscapes reverses the ASCII7 encoding used by IBKR for classic
// string fields. The official clients decode each \uXXXX UTF-16 code unit; a
// valid adjacent surrogate pair therefore becomes one Unicode code point.
func decodeUnicodeEscapes(value string) string {
	if !strings.Contains(value, `\u`) {
		return value
	}

	var out strings.Builder
	for len(value) > 0 {
		escape := strings.Index(value, `\u`)
		if escape < 0 || len(value)-escape < 6 {
			out.WriteString(value)
			break
		}
		out.WriteString(value[:escape])
		first, err := strconv.ParseUint(value[escape+2:escape+6], 16, 16)
		if err != nil {
			out.WriteString(value[escape : escape+2])
			value = value[escape+2:]
			continue
		}

		r := rune(first)
		consumed := escape + 6
		if utf16.IsSurrogate(r) && len(value) >= escape+12 && value[escape+6:escape+8] == `\u` {
			second, err := strconv.ParseUint(value[escape+8:escape+12], 16, 16)
			if err == nil {
				if decoded := utf16.DecodeRune(r, rune(second)); decoded != '\uFFFD' {
					r = decoded
					consumed = escape + 12
				}
			}
		}
		out.WriteRune(r)
		value = value[consumed:]
	}
	return out.String()
}

// [52, version, reqID]
func decodeContractDataEnd(r *fieldReader, sv int) ([]Message, error) {
	r.Skip(1)
	reqID, _ := r.ReadInt()
	return []Message{ContractDetailsEnd{ReqID: reqID}}, nil
}

func (m ContractDetailsEnd) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.InContractDataEnd), "1", itoa(m.ReqID)}, nil
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
	w.WriteInt(protocol.InSecDefOptParams)
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
	return []string{itoa(protocol.InSecDefOptParamsEnd), itoa(m.ReqID)}, nil
}

// [79, reqID, count, repeated(conID, symbol, secType, primaryExch, currency, derivCount, derivTypes..., description, issuerID)]
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
		derivCount, err := r.ReadCount("derivative sec type count")
		if err != nil {
			return nil, err
		}
		if derivCount > r.Remaining() {
			return nil, fmt.Errorf("codec: symbol samples: derivative count %d exceeds remaining fields %d", derivCount, r.Remaining())
		}
		derivTypes := make([]string, derivCount)
		for j := range derivTypes {
			derivTypes[j] = r.ReadString()
		}
		description := r.ReadString()
		issuerID := r.ReadString()
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
	w.WriteInt(protocol.InSymbolSamples)
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
	w.WriteInt(protocol.InSmartComponents)
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
	w.WriteInt(protocol.InMarketRule)
	w.WriteInt(m.MarketRuleID)
	w.WriteInt(len(m.Increments))
	for _, inc := range m.Increments {
		w.WriteString(inc.LowEdge)
		w.WriteString(inc.Increment)
	}
	return w.Fields(), nil
}

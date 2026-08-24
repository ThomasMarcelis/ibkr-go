package codec

import (
	"fmt"
	"strings"

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

// Contract-detail protobuf callbacks combine the last trade date and optional
// time metadata into one string. Split only the components owned by the public
// projection while preserving empty values.
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

// [75, reqID, exchange, underlyingConID, tradingClass, multiplier, expirationsCount, expirations..., strikesCount, strikes...] — no version
func decodeSecDefOptParams(r *fieldReader, sv int) ([]Message, error) {
	// Captured classic frames carry the expiration count directly after the
	// multiplier (sv208 capture 20260825T195326Z-sv208_classic_boundary_families,
	// events SHA-256 25aa15fdaeff68a48689bc70e68ddcc519783427a0fd835172c9aaad55246b08);
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
func decodeSecDefOptParamsEnd(r *fieldReader, sv int) ([]Message, error) {
	reqID, _ := r.ReadInt()
	return []Message{SecDefOptParamsEnd{ReqID: reqID}}, nil
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

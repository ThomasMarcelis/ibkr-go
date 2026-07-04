package ibkr

import (
	"github.com/shopspring/decimal"
)

// SecType identifies the instrument class of a [Contract]. Values mirror the
// IBKR TWS API Contract.secType vocabulary.
type SecType string

const (
	SecTypeStock        SecType = "STK"     // Stock or ETF
	SecTypeOption       SecType = "OPT"     // Option
	SecTypeFuture       SecType = "FUT"     // Future
	SecTypeContFuture   SecType = "CONTFUT" // Continuous future
	SecTypeFutureOption SecType = "FOP"     // Option on a future
	SecTypeIndex        SecType = "IND"     // Index
	SecTypeForex        SecType = "CASH"    // Forex pair
	SecTypeCombo        SecType = "BAG"     // Combo (multi-leg)
	SecTypeBond         SecType = "BOND"    // Bond
	SecTypeBill         SecType = "BILL"    // Treasury bill
	SecTypeCFD          SecType = "CFD"     // Contract for difference
	SecTypeWarrant      SecType = "WAR"     // Warrant
	SecTypeStructured   SecType = "IOPT"    // Structured product / Dutch warrant
	SecTypeForward      SecType = "FWD"     // Forward
	SecTypeCommodity    SecType = "CMDTY"   // Commodity
	SecTypeFund         SecType = "FUND"    // Mutual fund
	SecTypeFixed        SecType = "FIXED"   // Fixed income
	SecTypeSecLending   SecType = "SLB"     // Securities lending / borrowing
	SecTypeNews         SecType = "NEWS"    // News feed
	SecTypeBasket       SecType = "BSK"     // Basket
	SecTypeInterCmdty   SecType = "ICU"     // Inter-commodity spread (unsigned)
	SecTypeInterCmdtyS  SecType = "ICS"     // Inter-commodity spread (signed)
	SecTypeCrypto       SecType = "CRYPTO"  // Cryptocurrency
)

// Right identifies option direction: [RightCall] or [RightPut].
type Right string

const (
	RightCall Right = "C"
	RightPut  Right = "P"
)

// Contract identifies a tradable instrument. Users fill it in by hand to
// request data or place orders; the Gateway resolves it against its security
// database. Which fields are required depends on [Contract.SecType]:
//
//   - STK: Symbol, Exchange (conventionally "SMART"), Currency (e.g. "USD").
//   - OPT / FOP: Symbol, Expiry, Strike, Right, plus Exchange and Currency;
//     Multiplier and TradingClass disambiguate where a symbol has several.
//   - FUT / CONTFUT: Symbol (or LocalSymbol), Expiry, Exchange, Currency.
//   - CASH (forex): Symbol is the base currency, Currency the quote currency,
//     Exchange "IDEALPRO".
//
// Setting ConID alone unambiguously identifies a contract the client has
// already qualified; the descriptive fields can then be left zero. A zero
// Strike, empty Expiry, empty Right, or empty Multiplier all mean "unset" and
// are omitted from the request. Use [ContractsClient.Qualify] to resolve a
// partial contract to a fully specified one.
type Contract struct {
	ConID           int             // IBKR contract ID; nonzero pins an exact contract
	Symbol          string          // underlying symbol (ticker); base currency for forex
	SecType         SecType         // instrument class; drives which other fields matter
	Expiry          string          // YYYYMMDD (or YYYYMM) for derivatives; empty for cash instruments
	Strike          decimal.Decimal // option strike; zero means unset
	Right           Right           // option right; empty means unset
	Multiplier      string          // contract multiplier as a string, e.g. "100"; empty means default
	Exchange        string          // routing/listing exchange, commonly "SMART"
	Currency        string          // trading currency, e.g. "USD"; quote currency for forex
	LocalSymbol     string          // exchange-local symbol; an alternative to Symbol for futures
	TradingClass    string          // trading class, disambiguates options sharing a symbol
	PrimaryExchange string          // primary listing exchange, resolves SMART ambiguity for dual-listed stocks
}

// ComboLeg is one leg of a multi-leg (BAG) combo contract.
type ComboLeg struct {
	ConID              int    // contract ID of the leg instrument
	Ratio              int    // relative size of this leg within the combo
	Action             string // "BUY" or "SELL" for this leg
	Exchange           string // routing exchange for the leg
	OpenClose          string // open/close indicator (combo orders)
	ShortSaleSlot      int    // short-sale slot: 0 unset, 1 broker, 2 third party
	DesignatedLocation string // required when ShortSaleSlot is 2
	ExemptCode         int    // short-sale exempt code; -1 when unset
}

// TagValue is a generic name/value pair used for smart-routing and algo
// parameters on orders.
type TagValue struct {
	Tag   string
	Value string
}

// OrderCondition is a single conditional trigger attached to an order (for
// example price or time conditions). Type selects the condition kind and the
// remaining fields carry its parameters.
type OrderCondition struct {
	Type          int     // condition kind
	Conjunction   string  // "a" (and) or "o" (or) joining this condition to the next
	ConID         int     // contract the condition observes
	Exchange      string  // exchange the condition observes
	Operator      int     // comparison operator
	Value         string  // threshold value
	TriggerMethod int     // trigger method for price conditions
	SecType       SecType // security type of the observed contract
	Symbol        string  // symbol of the observed contract
}

// ContractDetails is the fully resolved description of a contract returned by
// [ContractsClient.Details] and [ContractsClient.Qualify]. It embeds the
// canonical [Contract] plus descriptive metadata.
type ContractDetails struct {
	Contract
	MarketName string
	LongName   string
	MinTick    decimal.Decimal // smallest price increment
	TimeZoneID string          // trading-hours time zone
}

// MatchingSymbol is one hit from a [ContractsClient.Search] symbol lookup.
type MatchingSymbol struct {
	ConID              int
	Symbol             string
	SecType            SecType
	PrimaryExchange    string
	Currency           string
	DerivativeSecTypes []string // derivative types available on this underlying
	Description        string
	IssuerID           string
}

// SecDefOptParamsRequest asks for the option chain parameters of an
// underlying via [ContractsClient.SecDefOptParams].
type SecDefOptParamsRequest struct {
	UnderlyingSymbol  string
	FutFopExchange    string  // exchange for FOP chains; empty for equity options
	UnderlyingSecType SecType // security type of the underlying
	UnderlyingConID   int     // contract ID of the underlying
}

// SecDefOptParams is one exchange's option chain definition for an underlying:
// the available expirations and strikes under a trading class.
type SecDefOptParams struct {
	Exchange        string
	UnderlyingConID int
	TradingClass    string
	Multiplier      string
	Expirations     []string
	Strikes         []decimal.Decimal
}

// PriceIncrement is one tier of a market rule: at prices at or above LowEdge,
// the minimum price increment is Increment.
type PriceIncrement struct {
	LowEdge   decimal.Decimal
	Increment decimal.Decimal
}

// MarketRuleResult is the tick-size schedule for a market rule ID, returned by
// [ContractsClient.MarketRule].
type MarketRuleResult struct {
	MarketRuleID int
	Increments   []PriceIncrement
}

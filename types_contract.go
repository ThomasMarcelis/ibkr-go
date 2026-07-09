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
	ConID              int         // contract ID of the leg instrument
	Ratio              int         // relative size of this leg within the combo
	Action             OrderAction // BUY, SELL, or SSHORT for this leg
	Exchange           string      // routing exchange for the leg
	OpenClose          string      // open/close indicator (combo orders)
	ShortSaleSlot      int         // short-sale slot: 0 unset, 1 broker, 2 third party
	DesignatedLocation string      // required when ShortSaleSlot is 2
	ExemptCode         int         // short-sale exempt code; -1 when unset
}

// DeltaNeutralContract describes the delta-neutral underlier attached to a
// contract. It is present only when IBKR echoes an explicit delta-neutral
// contract block.
type DeltaNeutralContract struct {
	ConID int
	Delta decimal.Decimal
	Price decimal.Decimal
}

// TagValue is a generic name/value pair used for contract security identifiers,
// scanner options, and smart-routing and algo parameters on orders.
type TagValue struct {
	Tag   string
	Value string
}

// OrderConditionType selects the payload carried by an [OrderCondition].
type OrderConditionType int

const (
	ConditionPrice         OrderConditionType = 1
	ConditionTime          OrderConditionType = 3
	ConditionMargin        OrderConditionType = 4
	ConditionExecution     OrderConditionType = 5
	ConditionVolume        OrderConditionType = 6
	ConditionPercentChange OrderConditionType = 7
)

// ConditionConjunction joins an order condition to the condition after it.
type ConditionConjunction string

const (
	ConditionAnd ConditionConjunction = "a"
	ConditionOr  ConditionConjunction = "o"
)

// ConditionOperator selects the direction of a threshold comparison.
type ConditionOperator int

const (
	ConditionLess ConditionOperator = 1
	ConditionMore ConditionOperator = 2
)

// OrderCondition is a single conditional trigger attached to an order (for
// example price or time conditions). Type selects the condition kind and the
// remaining fields carry its parameters.
type OrderCondition struct {
	Type          OrderConditionType
	Conjunction   ConditionConjunction
	ConID         int    // contract the condition observes
	Exchange      string // exchange the condition observes
	Operator      ConditionOperator
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
	MarketName              string
	LongName                string
	MinTick                 decimal.Decimal // smallest price increment
	PriceMagnifier          int
	OrderTypes              []string // IBKR order capabilities; includes order types and modifiers
	ValidExchanges          []ContractExchange
	UnderConID              int
	ContractMonth           string
	Industry                string
	Category                string
	Subcategory             string
	TimeZoneID              string // trading-hours time zone
	TradingHours            string // raw IBKR trading-hours calendar
	LiquidHours             string // raw IBKR liquid-hours calendar
	EconomicValueRule       string
	EconomicValueMultiplier *decimal.Decimal
	SecurityIDs             []TagValue
	AggGroup                *int
	UnderSymbol             string
	UnderSecType            SecType
	RealExpirationDate      string
	LastTradeDate           string // explicit YYYYMMDD date supplied by server versions 182+
	LastTradeTime           string // local HH:MM:SS component; use TimeZoneID for its zone
	StockType               string
	MinSize                 *decimal.Decimal // nil when IBKR omits the size rule
	SizeIncrement           *decimal.Decimal // nil when IBKR omits the size rule
	SuggestedSizeIncrement  *decimal.Decimal // nil when IBKR omits the size rule
	Fund                    *FundDetails
	IneligibilityReasons    []IneligibilityReason
}

// ContractExchange is a venue on which a contract is valid and the market
// rule that defines its price increments. Pass MarketRuleID to
// [ContractsClient.MarketRule] to resolve the complete tick-size schedule.
type ContractExchange struct {
	Exchange     string
	MarketRuleID int
}

// FundDetails carries the mutual-fund-only tail of [ContractDetails]. IBKR
// represents loads, fees, notification thresholds, and minimum purchases as
// strings; they remain strings because their formatting and units vary by
// fund family.
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

// IneligibilityReason explains why an account cannot trade a contract.
type IneligibilityReason struct {
	ID          string
	Description string
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

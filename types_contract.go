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

type Contract struct {
	ConID           int
	Symbol          string
	SecType         SecType
	Expiry          string
	Strike          decimal.Decimal
	Right           Right
	Multiplier      string
	Exchange        string
	Currency        string
	LocalSymbol     string
	TradingClass    string
	PrimaryExchange string
}

type ComboLeg struct {
	ConID              int
	Ratio              int
	Action             string
	Exchange           string
	OpenClose          string
	ShortSaleSlot      int
	DesignatedLocation string
	ExemptCode         int
}

type TagValue struct {
	Tag   string
	Value string
}

type OrderCondition struct {
	Type          int
	Conjunction   string
	ConID         int
	Exchange      string
	Operator      int
	Value         string
	TriggerMethod int
	SecType       SecType
	Symbol        string
}

type ContractDetails struct {
	Contract
	MarketName string
	LongName   string
	MinTick    decimal.Decimal
	TimeZoneID string
}

type MatchingSymbol struct {
	ConID              int
	Symbol             string
	SecType            SecType
	PrimaryExchange    string
	Currency           string
	DerivativeSecTypes []string
	Description        string
	IssuerID           string
}

type SecDefOptParamsRequest struct {
	UnderlyingSymbol  string
	FutFopExchange    string
	UnderlyingSecType SecType
	UnderlyingConID   int
}

type SecDefOptParams struct {
	Exchange        string
	UnderlyingConID int
	TradingClass    string
	Multiplier      string
	Expirations     []string
	Strikes         []decimal.Decimal
}

type PriceIncrement struct {
	LowEdge   decimal.Decimal
	Increment decimal.Decimal
}

type MarketRuleResult struct {
	MarketRuleID int
	Increments   []PriceIncrement
}

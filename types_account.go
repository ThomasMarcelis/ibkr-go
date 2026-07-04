package ibkr

import (
	"github.com/shopspring/decimal"
)

type AccountSummaryRequest struct {
	Account string
	Tags    []string
}

type AccountValue struct {
	Account  string
	Tag      string
	Value    string
	Currency string
}

type AccountSummaryUpdate struct {
	Value AccountValue
}

type Position struct {
	Account  string
	Contract Contract
	Position decimal.Decimal
	AvgCost  decimal.Decimal
}

type PositionUpdate struct {
	Position Position
}

type FamilyCode struct {
	AccountID  string
	FamilyCode string
}

type AccountUpdateValue struct {
	Key      string
	Value    string
	Currency string
	Account  string
}

type PortfolioUpdate struct {
	Account       string
	Contract      Contract
	Position      decimal.Decimal
	MarketPrice   decimal.Decimal
	MarketValue   decimal.Decimal
	AvgCost       decimal.Decimal
	UnrealizedPNL decimal.Decimal
	RealizedPNL   decimal.Decimal
}

// AccountUpdate is a union event from SubscribeAccountUpdates. Exactly one field is non-nil.
type AccountUpdate struct {
	AccountValue *AccountUpdateValue
	Portfolio    *PortfolioUpdate
}

type AccountUpdatesMultiRequest struct {
	Account   string
	ModelCode string
}

type AccountUpdateMultiValue struct {
	Account   string
	ModelCode string
	Key       string
	Value     string
	Currency  string
}

type PositionsMultiRequest struct {
	Account   string
	ModelCode string
}

type PositionMulti struct {
	Account   string
	ModelCode string
	Contract  Contract
	Position  decimal.Decimal
	AvgCost   decimal.Decimal
}

type PnLRequest struct {
	Account   string
	ModelCode string
}

type PnLUpdate struct {
	DailyPnL      decimal.Decimal
	UnrealizedPnL decimal.Decimal
	RealizedPnL   decimal.Decimal
}

type PnLSingleRequest struct {
	Account   string
	ModelCode string
	ConID     int
}

type PnLSingleUpdate struct {
	Position      decimal.Decimal
	DailyPnL      decimal.Decimal
	UnrealizedPnL decimal.Decimal
	RealizedPnL   decimal.Decimal
	Value         decimal.Decimal
}

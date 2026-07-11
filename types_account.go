package ibkr

import (
	"github.com/shopspring/decimal"
)

// AccountSummaryRequest selects the IBKR account group, optional local account
// filter, and summary tags for [AccountsClient.Summary]. An empty Group requests
// "All", and an empty AccountFilter accepts every returned account. Tags must
// contain at least one nonempty IBKR summary tag.
type AccountSummaryRequest struct {
	Group         string   // IBKR account group; empty requests "All"
	AccountFilter string   // exact returned account to accept; empty accepts all
	Tags          []string // summary tag names, e.g. "NetLiquidation"; empty requests all
}

// AccountValue is one account summary tag/value pair. Value is always a string
// as sent by the Gateway; Currency is empty for non-monetary tags.
type AccountValue struct {
	Account  string
	Tag      string
	Value    string
	Currency string
}

// Position is a held position: the signed quantity and average cost of a
// contract in an account.
type Position struct {
	Account  string
	Contract Contract
	Position decimal.Decimal // signed quantity; negative is short
	AvgCost  decimal.Decimal
}

// FamilyCode maps an account ID to its family code, returned by
// [AccountsClient.FamilyCodes].
type FamilyCode struct {
	AccountID  string
	FamilyCode string
}

// AccountUpdateValue is one account attribute from an account-updates
// subscription. Currency is empty for non-monetary keys.
type AccountUpdateValue struct {
	Key      string
	Value    string
	Currency string
	Account  string
}

// PortfolioUpdate is one portfolio position from an account-updates
// subscription. Monetary pointers preserve the difference between an omitted
// IBKR value and a reported zero.
type PortfolioUpdate struct {
	Account       string
	Contract      Contract
	Position      decimal.Decimal // signed quantity; negative is short
	MarketPrice   *decimal.Decimal
	MarketValue   *decimal.Decimal
	AvgCost       *decimal.Decimal
	UnrealizedPnL *decimal.Decimal
	RealizedPnL   *decimal.Decimal
}

// AccountUpdate is a union event from SubscribeAccountUpdates. Exactly one field is non-nil.
type AccountUpdate struct {
	AccountValue *AccountUpdateValue
	Portfolio    *PortfolioUpdate
	UpdateTime   *string // Gateway account-update time, formatted as received
}

// AccountUpdatesMultiRequest selects the account and advisor model for a
// multi-account updates subscription. Empty fields request all.
type AccountUpdatesMultiRequest struct {
	Account      string
	ModelCode    string // advisor model code; empty for the account itself
	LedgerAndNLV bool   // include ledger and net-liquidation-value rows
}

// AccountUpdateMultiValue is one account attribute from a multi-account updates
// subscription.
type AccountUpdateMultiValue struct {
	Account   string
	ModelCode string
	Key       string
	Value     string
	Currency  string
}

// PositionsMultiRequest selects the account and advisor model for a
// multi-account positions subscription.
type PositionsMultiRequest struct {
	Account   string
	ModelCode string // advisor model code; empty for the account itself
}

// PositionMulti is a held position scoped to an account and advisor model.
type PositionMulti struct {
	Account   string
	ModelCode string
	Contract  Contract
	Position  decimal.Decimal // signed quantity; negative is short
	AvgCost   decimal.Decimal
}

// PnLRequest selects the account and advisor model for an account-level P&L
// subscription.
type PnLRequest struct {
	Account   string
	ModelCode string
}

// PnLUpdate is an account-level profit-and-loss snapshot. A nil field was
// omitted by IBKR; a non-nil zero is a reported zero.
type PnLUpdate struct {
	DailyPnL      *decimal.Decimal
	UnrealizedPnL *decimal.Decimal
	RealizedPnL   *decimal.Decimal
}

// PnLSingleRequest selects the account, advisor model, and contract for a
// single-position P&L subscription.
type PnLSingleRequest struct {
	Account   string
	ModelCode string
	ConID     int // contract ID of the position to track
}

// PnLSingleUpdate is a single-position profit-and-loss snapshot. Optional
// monetary pointers use the same omitted-versus-zero convention as PnLUpdate.
type PnLSingleUpdate struct {
	Position      decimal.Decimal // current position size
	DailyPnL      *decimal.Decimal
	UnrealizedPnL *decimal.Decimal
	RealizedPnL   *decimal.Decimal
	Value         *decimal.Decimal // current market value of the position
}

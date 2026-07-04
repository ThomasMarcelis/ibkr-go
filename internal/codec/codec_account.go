package codec

type AccountSummaryRequest struct {
	ReqID   int
	Account string
	Tags    []string
}

func (AccountSummaryRequest) messageName() string { return "req_account_summary" }

type CancelAccountSummary struct {
	ReqID int
}

func (CancelAccountSummary) messageName() string { return "cancel_account_summary" }

type AccountSummaryValue struct {
	ReqID    int
	Account  string
	Tag      string
	Value    string
	Currency string
}

func (AccountSummaryValue) messageName() string { return "account_summary" }

type AccountSummaryEnd struct {
	ReqID int
}

func (AccountSummaryEnd) messageName() string { return "account_summary_end" }

type PositionsRequest struct{}

func (PositionsRequest) messageName() string { return "req_positions" }

type CancelPositions struct{}

func (CancelPositions) messageName() string { return "cancel_positions" }

type Position struct {
	Account  string
	Contract Contract
	Position string
	AvgCost  string
}

func (Position) messageName() string { return "position" }

type PositionEnd struct{}

func (PositionEnd) messageName() string { return "position_end" }

type FamilyCodesRequest struct{}

func (FamilyCodesRequest) messageName() string { return "req_family_codes" }

type FamilyCodes struct {
	Codes []FamilyCodeEntry
}

func (FamilyCodes) messageName() string { return "family_codes" }

type FamilyCodeEntry struct {
	AccountID  string
	FamilyCode string
}

// Account updates (OUT 6 / IN 6,7,8,54)

type AccountUpdatesRequest struct {
	Subscribe bool
	Account   string
}

func (AccountUpdatesRequest) messageName() string { return "req_account_updates" }

type UpdateAccountValue struct {
	Key      string
	Value    string
	Currency string
	Account  string
}

func (UpdateAccountValue) messageName() string { return "update_account_value" }

type UpdatePortfolio struct {
	Contract      Contract
	Position      string
	MarketPrice   string
	MarketValue   string
	AvgCost       string
	UnrealizedPNL string
	RealizedPNL   string
	Account       string
}

func (UpdatePortfolio) messageName() string { return "update_portfolio" }

type UpdateAccountTime struct {
	Timestamp string
}

func (UpdateAccountTime) messageName() string { return "update_account_time" }

type AccountDownloadEnd struct {
	Account string
}

func (AccountDownloadEnd) messageName() string { return "account_download_end" }

// Account updates multi (OUT 76, cancel OUT 77 / IN 73, 74)

type AccountUpdatesMultiRequest struct {
	ReqID     int
	Account   string
	ModelCode string
}

func (AccountUpdatesMultiRequest) messageName() string { return "req_account_updates_multi" }

type CancelAccountUpdatesMulti struct {
	ReqID int
}

func (CancelAccountUpdatesMulti) messageName() string { return "cancel_account_updates_multi" }

type AccountUpdateMultiValue struct {
	ReqID     int
	Account   string
	ModelCode string
	Key       string
	Value     string
	Currency  string
}

func (AccountUpdateMultiValue) messageName() string { return "account_update_multi" }

type AccountUpdateMultiEnd struct {
	ReqID int
}

func (AccountUpdateMultiEnd) messageName() string { return "account_update_multi_end" }

// Positions multi (OUT 74, cancel OUT 75 / IN 71, 72)

type PositionsMultiRequest struct {
	ReqID     int
	Account   string
	ModelCode string
}

func (PositionsMultiRequest) messageName() string { return "req_positions_multi" }

type CancelPositionsMulti struct {
	ReqID int
}

func (CancelPositionsMulti) messageName() string { return "cancel_positions_multi" }

type PositionMulti struct {
	ReqID     int
	Account   string
	ModelCode string
	Contract  Contract
	Position  string
	AvgCost   string
}

func (PositionMulti) messageName() string { return "position_multi" }

type PositionMultiEnd struct {
	ReqID int
}

func (PositionMultiEnd) messageName() string { return "position_multi_end" }

// PnL (OUT 92, cancel OUT 93 / IN 94)

type PnLRequest struct {
	ReqID     int
	Account   string
	ModelCode string
}

func (PnLRequest) messageName() string { return "req_pnl" }

type CancelPnL struct {
	ReqID int
}

func (CancelPnL) messageName() string { return "cancel_pnl" }

type PnLValue struct {
	ReqID         int
	DailyPnL      string
	UnrealizedPnL string
	RealizedPnL   string
}

func (PnLValue) messageName() string { return "pnl" }

// PnL single (OUT 94, cancel OUT 95 / IN 95)

type PnLSingleRequest struct {
	ReqID     int
	Account   string
	ModelCode string
	ConID     int
}

func (PnLSingleRequest) messageName() string { return "req_pnl_single" }

type CancelPnLSingle struct {
	ReqID int
}

func (CancelPnLSingle) messageName() string { return "cancel_pnl_single" }

type PnLSingleValue struct {
	ReqID         int
	Position      string
	DailyPnL      string
	UnrealizedPnL string
	RealizedPnL   string
	Value         string
}

func (PnLSingleValue) messageName() string { return "pnl_single" }

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

// [61, version, account, contract(11), position, avgCost]
func decodePositionData(r *fieldReader) ([]Message, error) {
	r.Skip(1)
	account := r.ReadString()
	contract := readWireContract(r)
	position := r.ReadString()
	avgCost := r.ReadString()
	return []Message{Position{Account: account, Contract: contract, Position: position, AvgCost: avgCost}}, nil
}

func decodePositionEnd(r *fieldReader) ([]Message, error) {
	return []Message{PositionEnd{}}, nil
}

// [63, version, reqID, account, tag, value, currency]
func decodeAccountSummary(r *fieldReader) ([]Message, error) {
	r.Skip(1)
	reqID, _ := r.ReadInt()
	account := r.ReadString()
	tag := r.ReadString()
	value := r.ReadString()
	currency := r.ReadString()
	return []Message{AccountSummaryValue{ReqID: reqID, Account: account, Tag: tag, Value: value, Currency: currency}}, nil
}

// [64, version, reqID]
func decodeAccountSummaryEnd(r *fieldReader) ([]Message, error) {
	r.Skip(1)
	reqID, _ := r.ReadInt()
	return []Message{AccountSummaryEnd{ReqID: reqID}}, nil
}

// [78, count, repeated(accountID, familyCode)] — no version
func decodeFamilyCodes(r *fieldReader) ([]Message, error) {
	count, err := r.ReadCount("family code count")
	if err != nil {
		return nil, err
	}
	if err := r.RequireFixedEntryFields("family codes", count, 2, 0); err != nil {
		return nil, err
	}
	entries := make([]FamilyCodeEntry, count)
	for i := range entries {
		entries[i] = FamilyCodeEntry{AccountID: r.ReadString(), FamilyCode: r.ReadString()}
	}
	return []Message{FamilyCodes{Codes: entries}}, nil
}

// [6, version=2, key, value, currency, accountName]
func decodeUpdateAccountValue(r *fieldReader) ([]Message, error) {
	r.Skip(1) // version
	key := r.ReadString()
	value := r.ReadString()
	currency := r.ReadString()
	account := r.ReadString()
	return []Message{UpdateAccountValue{Key: key, Value: value, Currency: currency, Account: account}}, nil
}

// [7, version=8, conID, symbol, secType, expiry, strike, right, multiplier, primaryExchange, currency, localSymbol, tradingClass, position, marketPrice, marketValue, avgCost, unrealizedPNL, realizedPNL, accountName]
func decodeUpdatePortfolio(r *fieldReader) ([]Message, error) {
	r.Skip(1) // version
	conID, _ := r.ReadInt()
	symbol := r.ReadString()
	secType := r.ReadString()
	expiry := r.ReadString()
	strike := r.ReadString()
	right := r.ReadString()
	multiplier := r.ReadString()
	primaryExchange := r.ReadString()
	currency := r.ReadString()
	localSymbol := r.ReadString()
	tradingClass := r.ReadString()
	position := r.ReadString()
	marketPrice := r.ReadString()
	marketValue := r.ReadString()
	avgCost := r.ReadString()
	unrealizedPNL := r.ReadString()
	realizedPNL := r.ReadString()
	account := r.ReadString()
	return []Message{UpdatePortfolio{
		Contract: Contract{
			ConID: conID, Symbol: symbol, SecType: secType,
			Expiry: expiry, Strike: strike, Right: right,
			Multiplier: multiplier, PrimaryExchange: primaryExchange,
			Currency: currency, LocalSymbol: localSymbol, TradingClass: tradingClass,
		},
		Position: position, MarketPrice: marketPrice, MarketValue: marketValue,
		AvgCost: avgCost, UnrealizedPNL: unrealizedPNL, RealizedPNL: realizedPNL,
		Account: account,
	}}, nil
}

// [8, version=1, timestamp]
func decodeUpdateAccountTime(r *fieldReader) ([]Message, error) {
	r.Skip(1) // version
	timestamp := r.ReadString()
	return []Message{UpdateAccountTime{Timestamp: timestamp}}, nil
}

// [54, version=1, accountName]
func decodeAccountDownloadEnd(r *fieldReader) ([]Message, error) {
	r.Skip(1) // version
	account := r.ReadString()
	return []Message{AccountDownloadEnd{Account: account}}, nil
}

// [71, version=1, reqID, account, modelCode, contract(11), position, avgCost]
func decodePositionMulti(r *fieldReader) ([]Message, error) {
	r.Skip(1) // version
	reqID, _ := r.ReadInt()
	account := r.ReadString()
	modelCode := r.ReadString()
	contract := readWireContract(r)
	position := r.ReadString()
	avgCost := r.ReadString()
	return []Message{PositionMulti{ReqID: reqID, Account: account, ModelCode: modelCode, Contract: contract, Position: position, AvgCost: avgCost}}, nil
}

// [72, version=1, reqID]
func decodePositionMultiEnd(r *fieldReader) ([]Message, error) {
	r.Skip(1) // version
	reqID, _ := r.ReadInt()
	return []Message{PositionMultiEnd{ReqID: reqID}}, nil
}

// [73, version=1, reqID, account, modelCode, key, value, currency]
func decodeAccountUpdateMulti(r *fieldReader) ([]Message, error) {
	r.Skip(1) // version
	reqID, _ := r.ReadInt()
	account := r.ReadString()
	modelCode := r.ReadString()
	key := r.ReadString()
	value := r.ReadString()
	currency := r.ReadString()
	return []Message{AccountUpdateMultiValue{ReqID: reqID, Account: account, ModelCode: modelCode, Key: key, Value: value, Currency: currency}}, nil
}

// [74, version=1, reqID]
func decodeAccountUpdateMultiEnd(r *fieldReader) ([]Message, error) {
	r.Skip(1) // version
	reqID, _ := r.ReadInt()
	return []Message{AccountUpdateMultiEnd{ReqID: reqID}}, nil
}

// [94, reqID, dailyPnL, unrealizedPnL, realizedPnL] — no version
func decodePnL(r *fieldReader) ([]Message, error) {
	reqID, _ := r.ReadInt()
	dailyPnL := r.ReadString()
	unrealizedPnL := r.ReadString()
	realizedPnL := r.ReadString()
	return []Message{PnLValue{ReqID: reqID, DailyPnL: dailyPnL, UnrealizedPnL: unrealizedPnL, RealizedPnL: realizedPnL}}, nil
}

// [95, reqID, pos, dailyPnL, unrealizedPnL, realizedPnL, value] — no version
func decodePnLSingle(r *fieldReader) ([]Message, error) {
	reqID, _ := r.ReadInt()
	position := r.ReadString()
	dailyPnL := r.ReadString()
	unrealizedPnL := r.ReadString()
	realizedPnL := r.ReadString()
	value := r.ReadString()
	return []Message{PnLSingleValue{ReqID: reqID, Position: position, DailyPnL: dailyPnL, UnrealizedPnL: unrealizedPnL, RealizedPnL: realizedPnL, Value: value}}, nil
}

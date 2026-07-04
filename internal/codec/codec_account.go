package codec

import "strings"

type AccountSummaryRequest struct {
	ReqID   int
	Account string
	Tags    []string
}

func (m AccountSummaryRequest) encodeWire() ([]string, error) {
	return []string{itoa(OutReqAccountSummary), "1", itoa(m.ReqID), m.Account, strings.Join(m.Tags, ",")}, nil
}

type CancelAccountSummary struct {
	ReqID int
}

func (m CancelAccountSummary) encodeWire() ([]string, error) {
	return []string{itoa(OutCancelAccountSummary), "1", itoa(m.ReqID)}, nil
}

type AccountSummaryValue struct {
	ReqID    int
	Account  string
	Tag      string
	Value    string
	Currency string
}

type AccountSummaryEnd struct {
	ReqID int
}

type PositionsRequest struct{}

func (m PositionsRequest) encodeWire() ([]string, error) {
	return []string{itoa(OutReqPositions), "1"}, nil
}

type CancelPositions struct{}

func (m CancelPositions) encodeWire() ([]string, error) {
	return []string{itoa(OutCancelPositions), "1"}, nil
}

type Position struct {
	Account  string
	Contract Contract
	Position string
	AvgCost  string
}

type PositionEnd struct{}

type FamilyCodesRequest struct{}

func (m FamilyCodesRequest) encodeWire() ([]string, error) {
	return []string{itoa(OutReqFamilyCodes)}, nil
}

type FamilyCodes struct {
	Codes []FamilyCodeEntry
}

type FamilyCodeEntry struct {
	AccountID  string
	FamilyCode string
}

// Account updates (OUT 6 / IN 6,7,8,54)

type AccountUpdatesRequest struct {
	Subscribe bool
	Account   string
}

func (m AccountUpdatesRequest) encodeWire() ([]string, error) {
	return []string{itoa(OutReqAccountUpdates), "2", btoa(m.Subscribe), m.Account}, nil
}

type UpdateAccountValue struct {
	Key      string
	Value    string
	Currency string
	Account  string
}

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

type UpdateAccountTime struct {
	Timestamp string
}

type AccountDownloadEnd struct {
	Account string
}

// Account updates multi (OUT 76, cancel OUT 77 / IN 73, 74)

type AccountUpdatesMultiRequest struct {
	ReqID     int
	Account   string
	ModelCode string
}

func (m AccountUpdatesMultiRequest) encodeWire() ([]string, error) {
	return []string{itoa(OutReqAccountUpdatesMulti), "1", itoa(m.ReqID), m.Account, m.ModelCode, "1"}, nil
}

type CancelAccountUpdatesMulti struct {
	ReqID int
}

func (m CancelAccountUpdatesMulti) encodeWire() ([]string, error) {
	return []string{itoa(OutCancelAccountUpdatesMulti), "1", itoa(m.ReqID)}, nil
}

type AccountUpdateMultiValue struct {
	ReqID     int
	Account   string
	ModelCode string
	Key       string
	Value     string
	Currency  string
}

type AccountUpdateMultiEnd struct {
	ReqID int
}

// Positions multi (OUT 74, cancel OUT 75 / IN 71, 72)

type PositionsMultiRequest struct {
	ReqID     int
	Account   string
	ModelCode string
}

func (m PositionsMultiRequest) encodeWire() ([]string, error) {
	return []string{itoa(OutReqPositionsMulti), "1", itoa(m.ReqID), m.Account, m.ModelCode}, nil
}

type CancelPositionsMulti struct {
	ReqID int
}

func (m CancelPositionsMulti) encodeWire() ([]string, error) {
	return []string{itoa(OutCancelPositionsMulti), "1", itoa(m.ReqID)}, nil
}

type PositionMulti struct {
	ReqID     int
	Account   string
	ModelCode string
	Contract  Contract
	Position  string
	AvgCost   string
}

type PositionMultiEnd struct {
	ReqID int
}

// PnL (OUT 92, cancel OUT 93 / IN 94)

type PnLRequest struct {
	ReqID     int
	Account   string
	ModelCode string
}

func (m PnLRequest) encodeWire() ([]string, error) {
	return []string{itoa(OutReqPnL), itoa(m.ReqID), m.Account, m.ModelCode}, nil
}

type CancelPnL struct {
	ReqID int
}

func (m CancelPnL) encodeWire() ([]string, error) {
	return []string{itoa(OutCancelPnL), itoa(m.ReqID)}, nil
}

type PnLValue struct {
	ReqID         int
	DailyPnL      string
	UnrealizedPnL string
	RealizedPnL   string
}

// PnL single (OUT 94, cancel OUT 95 / IN 95)

type PnLSingleRequest struct {
	ReqID     int
	Account   string
	ModelCode string
	ConID     int
}

func (m PnLSingleRequest) encodeWire() ([]string, error) {
	return []string{itoa(OutReqPnLSingle), itoa(m.ReqID), m.Account, m.ModelCode, itoa(m.ConID)}, nil
}

type CancelPnLSingle struct {
	ReqID int
}

func (m CancelPnLSingle) encodeWire() ([]string, error) {
	return []string{itoa(OutCancelPnLSingle), itoa(m.ReqID)}, nil
}

type PnLSingleValue struct {
	ReqID         int
	Position      string
	DailyPnL      string
	UnrealizedPnL string
	RealizedPnL   string
	Value         string
}

// [61, version, account, contract(11), position, avgCost]
func decodePositionData(r *fieldReader) ([]Message, error) {
	r.Skip(1)
	account := r.ReadString()
	contract := readWireContract(r)
	position := r.ReadString()
	avgCost := r.ReadString()
	return []Message{Position{Account: account, Contract: contract, Position: position, AvgCost: avgCost}}, nil
}

func (m Position) encodeWire() ([]string, error) {
	// Encode in server→client wire format matching readWireContract:
	// [conID, symbol, secType, expiry, strike, right, multiplier,
	//  exchange, currency, localSymbol, tradingClass]
	w := fieldWriter{}
	w.WriteInt(InPositionData)
	w.WriteInt(3) // version
	w.WriteString(m.Account)
	w.WriteInt(m.Contract.ConID)
	w.WriteString(m.Contract.Symbol)
	w.WriteString(m.Contract.SecType)
	w.WriteString(m.Contract.Expiry)
	if m.Contract.Strike == "" {
		w.WriteString("0")
	} else {
		w.WriteString(m.Contract.Strike)
	}
	w.WriteString(m.Contract.Right)
	w.WriteString(m.Contract.Multiplier)
	w.WriteString(m.Contract.Exchange)
	w.WriteString(m.Contract.Currency)
	w.WriteString(m.Contract.LocalSymbol)
	w.WriteString(m.Contract.TradingClass)
	w.WriteString(m.Position)
	w.WriteString(m.AvgCost)
	return w.Fields(), nil
}

func decodePositionEnd(r *fieldReader) ([]Message, error) {
	return []Message{PositionEnd{}}, nil
}

func (m PositionEnd) encodeWire() ([]string, error) {
	return []string{itoa(InPositionEnd), "1"}, nil
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

func (m AccountSummaryValue) encodeWire() ([]string, error) {
	return []string{itoa(InAccountSummary), "1", itoa(m.ReqID), m.Account, m.Tag, m.Value, m.Currency}, nil
}

// [64, version, reqID]
func decodeAccountSummaryEnd(r *fieldReader) ([]Message, error) {
	r.Skip(1)
	reqID, _ := r.ReadInt()
	return []Message{AccountSummaryEnd{ReqID: reqID}}, nil
}

func (m AccountSummaryEnd) encodeWire() ([]string, error) {
	return []string{itoa(InAccountSummaryEnd), "1", itoa(m.ReqID)}, nil
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

func (m FamilyCodes) encodeWire() ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(InFamilyCodes)
	w.WriteInt(len(m.Codes))
	for _, c := range m.Codes {
		w.WriteString(c.AccountID)
		w.WriteString(c.FamilyCode)
	}
	return w.Fields(), nil
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

func (m UpdateAccountValue) encodeWire() ([]string, error) {
	return []string{itoa(InUpdateAccountValue), "2", m.Key, m.Value, m.Currency, m.Account}, nil
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

func (m UpdatePortfolio) encodeWire() ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(InUpdatePortfolio)
	w.WriteInt(8) // version
	w.WriteInt(m.Contract.ConID)
	w.WriteString(m.Contract.Symbol)
	w.WriteString(m.Contract.SecType)
	w.WriteString(m.Contract.Expiry)
	if m.Contract.Strike == "" {
		w.WriteString("0")
	} else {
		w.WriteString(m.Contract.Strike)
	}
	w.WriteString(m.Contract.Right)
	w.WriteString(m.Contract.Multiplier)
	w.WriteString(m.Contract.PrimaryExchange)
	w.WriteString(m.Contract.Currency)
	w.WriteString(m.Contract.LocalSymbol)
	w.WriteString(m.Contract.TradingClass)
	w.WriteString(m.Position)
	w.WriteString(m.MarketPrice)
	w.WriteString(m.MarketValue)
	w.WriteString(m.AvgCost)
	w.WriteString(m.UnrealizedPNL)
	w.WriteString(m.RealizedPNL)
	w.WriteString(m.Account)
	return w.Fields(), nil
}

// [8, version=1, timestamp]
func decodeUpdateAccountTime(r *fieldReader) ([]Message, error) {
	r.Skip(1) // version
	timestamp := r.ReadString()
	return []Message{UpdateAccountTime{Timestamp: timestamp}}, nil
}

func (m UpdateAccountTime) encodeWire() ([]string, error) {
	return []string{itoa(InUpdateAccountTime), "1", m.Timestamp}, nil
}

// [54, version=1, accountName]
func decodeAccountDownloadEnd(r *fieldReader) ([]Message, error) {
	r.Skip(1) // version
	account := r.ReadString()
	return []Message{AccountDownloadEnd{Account: account}}, nil
}

func (m AccountDownloadEnd) encodeWire() ([]string, error) {
	return []string{itoa(InAccountDownloadEnd), "1", m.Account}, nil
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

func (m PositionMulti) encodeWire() ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(InPositionMulti)
	w.WriteInt(1) // version
	w.WriteInt(m.ReqID)
	w.WriteString(m.Account)
	w.WriteString(m.ModelCode)
	w.WriteInt(m.Contract.ConID)
	w.WriteString(m.Contract.Symbol)
	w.WriteString(m.Contract.SecType)
	w.WriteString(m.Contract.Expiry)
	if m.Contract.Strike == "" {
		w.WriteString("0")
	} else {
		w.WriteString(m.Contract.Strike)
	}
	w.WriteString(m.Contract.Right)
	w.WriteString(m.Contract.Multiplier)
	w.WriteString(m.Contract.Exchange)
	w.WriteString(m.Contract.Currency)
	w.WriteString(m.Contract.LocalSymbol)
	w.WriteString(m.Contract.TradingClass)
	w.WriteString(m.Position)
	w.WriteString(m.AvgCost)
	return w.Fields(), nil
}

// [72, version=1, reqID]
func decodePositionMultiEnd(r *fieldReader) ([]Message, error) {
	r.Skip(1) // version
	reqID, _ := r.ReadInt()
	return []Message{PositionMultiEnd{ReqID: reqID}}, nil
}

func (m PositionMultiEnd) encodeWire() ([]string, error) {
	return []string{itoa(InPositionMultiEnd), "1", itoa(m.ReqID)}, nil
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

func (m AccountUpdateMultiValue) encodeWire() ([]string, error) {
	return []string{itoa(InAccountUpdateMulti), "1", itoa(m.ReqID), m.Account, m.ModelCode, m.Key, m.Value, m.Currency}, nil
}

// [74, version=1, reqID]
func decodeAccountUpdateMultiEnd(r *fieldReader) ([]Message, error) {
	r.Skip(1) // version
	reqID, _ := r.ReadInt()
	return []Message{AccountUpdateMultiEnd{ReqID: reqID}}, nil
}

func (m AccountUpdateMultiEnd) encodeWire() ([]string, error) {
	return []string{itoa(InAccountUpdateMultiEnd), "1", itoa(m.ReqID)}, nil
}

// [94, reqID, dailyPnL, unrealizedPnL, realizedPnL] — no version
func decodePnL(r *fieldReader) ([]Message, error) {
	reqID, _ := r.ReadInt()
	dailyPnL := r.ReadString()
	unrealizedPnL := r.ReadString()
	realizedPnL := r.ReadString()
	return []Message{PnLValue{ReqID: reqID, DailyPnL: dailyPnL, UnrealizedPnL: unrealizedPnL, RealizedPnL: realizedPnL}}, nil
}

func (m PnLValue) encodeWire() ([]string, error) {
	return []string{itoa(InPnL), itoa(m.ReqID), m.DailyPnL, m.UnrealizedPnL, m.RealizedPnL}, nil
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

func (m PnLSingleValue) encodeWire() ([]string, error) {
	return []string{itoa(InPnLSingle), itoa(m.ReqID), m.Position, m.DailyPnL, m.UnrealizedPnL, m.RealizedPnL, m.Value}, nil
}

package codec

import "github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"

type AccountSummaryRequest struct {
	ReqID   int
	Account string
	Tags    []string
}

type CancelAccountSummary struct {
	ReqID int
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

type CancelPositions struct{}

type Position struct {
	Account  string
	Contract Contract
	Position string
	AvgCost  string
}

type PositionEnd struct{}

type FamilyCodesRequest struct{}

func (m FamilyCodesRequest) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutReqFamilyCodes)}, nil
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
	ReqID        int
	Account      string
	ModelCode    string
	LedgerAndNLV bool
}

type CancelAccountUpdatesMulti struct {
	ReqID int
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

type CancelPositionsMulti struct {
	ReqID int
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

func (m PnLRequest) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutReqPnL), itoa(m.ReqID), m.Account, m.ModelCode}, nil
}

type CancelPnL struct {
	ReqID int
}

func (m CancelPnL) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutCancelPnL), itoa(m.ReqID)}, nil
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

func (m PnLSingleRequest) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutReqPnLSingle), itoa(m.ReqID), m.Account, m.ModelCode, itoa(m.ConID)}, nil
}

type CancelPnLSingle struct {
	ReqID int
}

func (m CancelPnLSingle) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutCancelPnLSingle), itoa(m.ReqID)}, nil
}

type PnLSingleValue struct {
	ReqID         int
	Position      string
	DailyPnL      string
	UnrealizedPnL string
	RealizedPnL   string
	Value         string
}

// [78, count, repeated(accountID, familyCode)] — no version
func decodeFamilyCodes(r *fieldReader, sv int) ([]Message, error) {
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

// [94, reqID, dailyPnL, unrealizedPnL, realizedPnL] — no version
func decodePnL(r *fieldReader, sv int) ([]Message, error) {
	reqID, _ := r.ReadInt()
	dailyPnL := r.ReadString()
	unrealizedPnL := r.ReadString()
	realizedPnL := r.ReadString()
	return []Message{PnLValue{ReqID: reqID, DailyPnL: dailyPnL, UnrealizedPnL: unrealizedPnL, RealizedPnL: realizedPnL}}, nil
}

// [95, reqID, pos, dailyPnL, unrealizedPnL, realizedPnL, value] — no version
func decodePnLSingle(r *fieldReader, sv int) ([]Message, error) {
	reqID, _ := r.ReadInt()
	position := r.ReadString()
	dailyPnL := r.ReadString()
	unrealizedPnL := r.ReadString()
	realizedPnL := r.ReadString()
	value := r.ReadString()
	return []Message{PnLSingleValue{ReqID: reqID, Position: position, DailyPnL: dailyPnL, UnrealizedPnL: unrealizedPnL, RealizedPnL: realizedPnL, Value: value}}, nil
}

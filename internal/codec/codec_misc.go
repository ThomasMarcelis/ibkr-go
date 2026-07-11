package codec

import (
	"strings"

	"github.com/ThomasMarcelis/ibkr-go/internal/protocol"
)

// Message is a wire message that knows its own field encoding.
type Message interface {
	encodeWire(sv int) ([]string, error)
}

type ScannerParametersRequest struct{}

func (m ScannerParametersRequest) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutReqScannerParameters), "1"}, nil
}

type ScannerParameters struct {
	XML string
}

// ScannerSubscription (OUT 22 / cancel OUT 23 / IN 20)

type ScannerSubscriptionRequest struct {
	ReqID                    int
	NumberOfRows             int
	Instrument               string
	LocationCode             string
	ScanCode                 string
	AbovePrice               string
	BelowPrice               string
	AboveVolume              string
	MarketCapAbove           string
	MarketCapBelow           string
	MoodyRatingAbove         string
	MoodyRatingBelow         string
	SPRatingAbove            string
	SPRatingBelow            string
	MaturityDateAbove        string
	MaturityDateBelow        string
	CouponRateAbove          string
	CouponRateBelow          string
	ExcludeConvertible       string
	AverageOptionVolumeAbove string
	ScannerSettingPairs      string
	StockTypeFilter          string
	FilterOptions            []TagValue
	SubscriptionOptions      []TagValue
}

func (m ScannerSubscriptionRequest) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(protocol.OutReqScannerSubscription)
	w.WriteInt(m.ReqID)
	w.WriteMaxInt(m.NumberOfRows)
	w.WriteString(m.Instrument)
	w.WriteString(m.LocationCode)
	w.WriteString(m.ScanCode)
	w.WriteString(m.AbovePrice)
	w.WriteString(m.BelowPrice)
	w.WriteString(m.AboveVolume)
	w.WriteString(m.MarketCapAbove)
	w.WriteString(m.MarketCapBelow)
	w.WriteString(m.MoodyRatingAbove)
	w.WriteString(m.MoodyRatingBelow)
	w.WriteString(m.SPRatingAbove)
	w.WriteString(m.SPRatingBelow)
	w.WriteString(m.MaturityDateAbove)
	w.WriteString(m.MaturityDateBelow)
	w.WriteString(m.CouponRateAbove)
	w.WriteString(m.CouponRateBelow)
	w.WriteString(m.ExcludeConvertible)
	w.WriteString(m.AverageOptionVolumeAbove)
	w.WriteString(m.ScannerSettingPairs)
	w.WriteString(m.StockTypeFilter)
	w.WriteString(encodeScannerTagValues(m.FilterOptions))
	w.WriteString(encodeScannerTagValues(m.SubscriptionOptions))
	return w.Fields(), nil
}

func encodeScannerTagValues(values []TagValue) string {
	var encoded strings.Builder
	for _, value := range values {
		encoded.WriteString(value.Tag)
		encoded.WriteByte('=')
		encoded.WriteString(value.Value)
		encoded.WriteByte(';')
	}
	return encoded.String()
}

type CancelScannerSubscription struct {
	ReqID int
}

func (m CancelScannerSubscription) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutCancelScannerSubscription), "1", itoa(m.ReqID)}, nil
}

type ScannerDataEntry struct {
	Rank       int
	Contract   Contract
	Distance   string
	Benchmark  string
	Projection string
	LegsStr    string
}

type ScannerDataResponse struct {
	ReqID   int
	Entries []ScannerDataEntry
}

// FA Configuration (OUT 18 / IN 16)

type RequestFA struct {
	FADataType int // 1=Groups, 2=Profiles, 3=AccountAliases
}

func (m RequestFA) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutRequestFA), "1", itoa(m.FADataType)}, nil
}

type ReceiveFA struct {
	FADataType int
	XML        string
}

// WSH Calendar Events (OUT 100, cancel OUT 101 / IN 105)
// WSH Event Data (OUT 102, cancel OUT 103 / IN 106)

type WSHMetaDataRequest struct {
	ReqID int
}

func (m WSHMetaDataRequest) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutReqWSHMetaData), itoa(m.ReqID)}, nil
}

type CancelWSHMetaData struct {
	ReqID int
}

func (m CancelWSHMetaData) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutCancelWSHMetaData), itoa(m.ReqID)}, nil
}

type WSHEventDataRequest struct {
	ReqID           int
	ConID           int
	Filter          string
	FillWatchlist   bool
	FillPortfolio   bool
	FillCompetitors bool
	StartDate       string
	EndDate         string
	TotalLimit      int
}

func (m WSHEventDataRequest) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(protocol.OutReqWSHEventData)
	w.WriteInt(m.ReqID)
	w.WriteInt(m.ConID)
	w.WriteString(m.Filter)
	w.WriteBool(m.FillWatchlist)
	w.WriteBool(m.FillPortfolio)
	w.WriteBool(m.FillCompetitors)
	w.WriteString(m.StartDate)
	w.WriteString(m.EndDate)
	w.WriteInt(m.TotalLimit)
	return w.Fields(), nil
}

type CancelWSHEventData struct {
	ReqID int
}

func (m CancelWSHEventData) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutCancelWSHEventData), itoa(m.ReqID)}, nil
}

type WSHMetaDataResponse struct {
	ReqID    int
	DataJSON string
}

type WSHEventDataResponse struct {
	ReqID    int
	DataJSON string
}

// Display Groups (OUT 67, 68, 69, 70 / IN 67, 68)

type QueryDisplayGroupsRequest struct {
	ReqID int
}

func (m QueryDisplayGroupsRequest) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutQueryDisplayGroups), "1", itoa(m.ReqID)}, nil
}

type SubscribeToGroupEventsRequest struct {
	ReqID   int
	GroupID int
}

func (m SubscribeToGroupEventsRequest) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutSubscribeToGroupEvents), "1", itoa(m.ReqID), itoa(m.GroupID)}, nil
}

type UpdateDisplayGroupRequest struct {
	ReqID        int
	ContractInfo string
}

func (m UpdateDisplayGroupRequest) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutUpdateDisplayGroup), "1", itoa(m.ReqID), m.ContractInfo}, nil
}

type UnsubscribeFromGroupEventsRequest struct {
	ReqID int
}

func (m UnsubscribeFromGroupEventsRequest) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutUnsubscribeFromGroupEvents), "1", itoa(m.ReqID)}, nil
}

type DisplayGroupList struct {
	ReqID  int
	Groups string
}

type DisplayGroupUpdated struct {
	ReqID        int
	ContractInfo string
}

// [19, version=1, xml]
func decodeScannerParameters(r *fieldReader, sv int) ([]Message, error) {
	r.Skip(1) // version
	xml := r.ReadString()
	return []Message{ScannerParameters{XML: xml}}, nil
}

func (m ScannerParameters) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.InScannerParameters), "1", m.XML}, nil
}

// [20, version=3, reqID, numberOfElements, entries(rank, contract(11), distance, benchmark, projection, legsStr)]
func decodeScannerData(r *fieldReader, sv int) ([]Message, error) {
	r.Skip(1) // version
	reqID, _ := r.ReadInt()
	count, err := r.ReadCount("scanner entry count")
	if err != nil {
		return nil, err
	}
	if err := r.RequireFixedEntryFields("scanner data", count, 16, 0); err != nil {
		return nil, err
	}
	entries := make([]ScannerDataEntry, count)
	for i := range entries {
		rank, _ := r.ReadInt()
		contract := readWireContract(r)
		distance := r.ReadString()
		benchmark := r.ReadString()
		projection := r.ReadString()
		legsStr := r.ReadString()
		entries[i] = ScannerDataEntry{Rank: rank, Contract: contract, Distance: distance, Benchmark: benchmark, Projection: projection, LegsStr: legsStr}
	}
	return []Message{ScannerDataResponse{ReqID: reqID, Entries: entries}}, nil
}

func (m ScannerDataResponse) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(protocol.InScannerData)
	w.WriteInt(3) // version
	w.WriteInt(m.ReqID)
	w.WriteInt(len(m.Entries))
	for _, e := range m.Entries {
		w.WriteInt(e.Rank)
		// Server->client 11-field contract: conID, symbol, secType, expiry, strike, right, multiplier, exchange, currency, localSymbol, tradingClass
		w.WriteInt(e.Contract.ConID)
		w.WriteString(e.Contract.Symbol)
		w.WriteString(e.Contract.SecType)
		w.WriteString(e.Contract.Expiry)
		if e.Contract.Strike == "" {
			w.WriteString("0")
		} else {
			w.WriteString(e.Contract.Strike)
		}
		w.WriteString(e.Contract.Right)
		w.WriteString(e.Contract.Multiplier)
		w.WriteString(e.Contract.Exchange)
		w.WriteString(e.Contract.Currency)
		w.WriteString(e.Contract.LocalSymbol)
		w.WriteString(e.Contract.TradingClass)
		w.WriteString(e.Distance)
		w.WriteString(e.Benchmark)
		w.WriteString(e.Projection)
		w.WriteString(e.LegsStr)
	}
	return w.Fields(), nil
}

// [16, version, faDataType, xml]
func decodeReceiveFA(r *fieldReader, sv int) ([]Message, error) {
	r.Skip(1) // version
	faDataType, _ := r.ReadInt()
	xml := r.ReadString()
	return []Message{ReceiveFA{FADataType: faDataType, XML: xml}}, nil
}

func (m ReceiveFA) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.InReceiveFA), "1", itoa(m.FADataType), m.XML}, nil
}

// [104, reqId, dataJson]
func decodeWSHMetaData(r *fieldReader, sv int) ([]Message, error) {
	reqID, _ := r.ReadInt()
	dataJSON := r.ReadString()
	return []Message{WSHMetaDataResponse{ReqID: reqID, DataJSON: dataJSON}}, nil
}

func (m WSHMetaDataResponse) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.InWSHMetaData), itoa(m.ReqID), m.DataJSON}, nil
}

// [105, reqId, dataJson]
func decodeWSHEventData(r *fieldReader, sv int) ([]Message, error) {
	reqID, _ := r.ReadInt()
	dataJSON := r.ReadString()
	return []Message{WSHEventDataResponse{ReqID: reqID, DataJSON: dataJSON}}, nil
}

func (m WSHEventDataResponse) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.InWSHEventData), itoa(m.ReqID), m.DataJSON}, nil
}

// [67, version, reqId, groups]
func decodeDisplayGroupList(r *fieldReader, sv int) ([]Message, error) {
	r.Skip(1) // version
	reqID, _ := r.ReadInt()
	groups := r.ReadString()
	return []Message{DisplayGroupList{ReqID: reqID, Groups: groups}}, nil
}

func (m DisplayGroupList) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.InDisplayGroupList), "1", itoa(m.ReqID), m.Groups}, nil
}

// [68, version, reqId, contractInfo]
func decodeDisplayGroupUpdated(r *fieldReader, sv int) ([]Message, error) {
	r.Skip(1) // version
	reqID, _ := r.ReadInt()
	contractInfo := r.ReadString()
	return []Message{DisplayGroupUpdated{ReqID: reqID, ContractInfo: contractInfo}}, nil
}

func (m DisplayGroupUpdated) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.InDisplayGroupUpdated), "1", itoa(m.ReqID), m.ContractInfo}, nil
}

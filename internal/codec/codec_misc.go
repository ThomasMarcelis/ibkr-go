package codec

type Message interface {
	messageName() string
}

type ScannerParametersRequest struct{}

func (ScannerParametersRequest) messageName() string { return "req_scanner_parameters" }

type ScannerParameters struct {
	XML string
}

func (ScannerParameters) messageName() string { return "scanner_parameters" }

// ScannerSubscription (OUT 22 / cancel OUT 23 / IN 20)

type ScannerSubscriptionRequest struct {
	ReqID        int
	NumberOfRows int
	Instrument   string
	LocationCode string
	ScanCode     string
}

func (ScannerSubscriptionRequest) messageName() string { return "req_scanner_subscription" }

type CancelScannerSubscription struct {
	ReqID int
}

func (CancelScannerSubscription) messageName() string { return "cancel_scanner_subscription" }

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

func (ScannerDataResponse) messageName() string { return "scanner_data" }

// FA Configuration (OUT 18, OUT 19 / IN 16)

type RequestFA struct {
	FADataType int // 1=Groups, 2=Profiles, 3=AccountAliases
}

func (RequestFA) messageName() string { return "req_fa" }

type ReplaceFA struct {
	FADataType int
	XML        string
}

func (ReplaceFA) messageName() string { return "replace_fa" }

type ReceiveFA struct {
	FADataType int
	XML        string
}

func (ReceiveFA) messageName() string { return "receive_fa" }

// WSH Calendar Events (OUT 100, cancel OUT 101 / IN 105)
// WSH Event Data (OUT 102, cancel OUT 103 / IN 106)

type WSHMetaDataRequest struct {
	ReqID int
}

func (WSHMetaDataRequest) messageName() string { return "req_wsh_meta_data" }

type CancelWSHMetaData struct {
	ReqID int
}

func (CancelWSHMetaData) messageName() string { return "cancel_wsh_meta_data" }

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

func (WSHEventDataRequest) messageName() string { return "req_wsh_event_data" }

type CancelWSHEventData struct {
	ReqID int
}

func (CancelWSHEventData) messageName() string { return "cancel_wsh_event_data" }

type WSHMetaDataResponse struct {
	ReqID    int
	DataJSON string
}

func (WSHMetaDataResponse) messageName() string { return "wsh_meta_data" }

type WSHEventDataResponse struct {
	ReqID    int
	DataJSON string
}

func (WSHEventDataResponse) messageName() string { return "wsh_event_data" }

// Display Groups (OUT 67, 68, 69, 70 / IN 67, 68)

type QueryDisplayGroupsRequest struct {
	ReqID int
}

func (QueryDisplayGroupsRequest) messageName() string { return "query_display_groups" }

type SubscribeToGroupEventsRequest struct {
	ReqID   int
	GroupID int
}

func (SubscribeToGroupEventsRequest) messageName() string { return "subscribe_to_group_events" }

type UpdateDisplayGroupRequest struct {
	ReqID        int
	ContractInfo string
}

func (UpdateDisplayGroupRequest) messageName() string { return "update_display_group" }

type UnsubscribeFromGroupEventsRequest struct {
	ReqID int
}

func (UnsubscribeFromGroupEventsRequest) messageName() string { return "unsubscribe_from_group_events" }

type DisplayGroupList struct {
	ReqID  int
	Groups string
}

func (DisplayGroupList) messageName() string { return "display_group_list" }

type DisplayGroupUpdated struct {
	ReqID        int
	ContractInfo string
}

func (DisplayGroupUpdated) messageName() string { return "display_group_updated" }

// FundamentalData (OUT 52, cancel OUT 53 / IN 51)

type FundamentalDataRequest struct {
	ReqID      int
	Contract   Contract
	ReportType string
}

func (FundamentalDataRequest) messageName() string { return "req_fundamental_data" }

type CancelFundamentalData struct {
	ReqID int
}

func (CancelFundamentalData) messageName() string { return "cancel_fundamental_data" }

type FundamentalDataResponse struct {
	ReqID int
	Data  string
}

func (FundamentalDataResponse) messageName() string { return "fundamental_data" }

// [19, version=1, xml]
func decodeScannerParameters(r *fieldReader) ([]Message, error) {
	r.Skip(1) // version
	xml := r.ReadString()
	return []Message{ScannerParameters{XML: xml}}, nil
}

// [20, version=3, reqID, numberOfElements, entries(rank, contract(11), distance, benchmark, projection, legsStr)]
func decodeScannerData(r *fieldReader) ([]Message, error) {
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

// [51, version, reqID, data]
func decodeFundamentalData(r *fieldReader) ([]Message, error) {
	r.Skip(1) // version
	reqID, _ := r.ReadInt()
	data := r.ReadString()
	return []Message{FundamentalDataResponse{ReqID: reqID, Data: data}}, nil
}

// [16, version, faDataType, xml]
func decodeReceiveFA(r *fieldReader) ([]Message, error) {
	r.Skip(1) // version
	faDataType, _ := r.ReadInt()
	xml := r.ReadString()
	return []Message{ReceiveFA{FADataType: faDataType, XML: xml}}, nil
}

// [104, reqId, dataJson]
func decodeWSHMetaData(r *fieldReader) ([]Message, error) {
	reqID, _ := r.ReadInt()
	dataJSON := r.ReadString()
	return []Message{WSHMetaDataResponse{ReqID: reqID, DataJSON: dataJSON}}, nil
}

// [105, reqId, dataJson]
func decodeWSHEventData(r *fieldReader) ([]Message, error) {
	reqID, _ := r.ReadInt()
	dataJSON := r.ReadString()
	return []Message{WSHEventDataResponse{ReqID: reqID, DataJSON: dataJSON}}, nil
}

// [67, version, reqId, groups]
func decodeDisplayGroupList(r *fieldReader) ([]Message, error) {
	r.Skip(1) // version
	reqID, _ := r.ReadInt()
	groups := r.ReadString()
	return []Message{DisplayGroupList{ReqID: reqID, Groups: groups}}, nil
}

// [68, version, reqId, contractInfo]
func decodeDisplayGroupUpdated(r *fieldReader) ([]Message, error) {
	r.Skip(1) // version
	reqID, _ := r.ReadInt()
	contractInfo := r.ReadString()
	return []Message{DisplayGroupUpdated{ReqID: reqID, ContractInfo: contractInfo}}, nil
}

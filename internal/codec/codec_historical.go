package codec

type HistoricalBarsRequest struct {
	ReqID        int
	Contract     Contract
	EndDateTime  string
	Duration     string
	BarSize      string
	WhatToShow   string
	UseRTH       bool
	KeepUpToDate bool
}

func (HistoricalBarsRequest) messageName() string { return "req_historical_bars" }

type HistoricalBar struct {
	ReqID  int
	Time   string
	Open   string
	High   string
	Low    string
	Close  string
	Volume string
	WAP    string
	Count  string
}

func (HistoricalBar) messageName() string { return "historical_bar" }

type HistoricalBarsEnd struct {
	ReqID int
}

func (HistoricalBarsEnd) messageName() string { return "historical_bars_end" }

type CancelHistoricalData struct {
	ReqID int
}

func (CancelHistoricalData) messageName() string { return "cancel_historical_data" }

type HeadTimestampRequest struct {
	ReqID      int
	Contract   Contract
	WhatToShow string
	UseRTH     bool
}

func (HeadTimestampRequest) messageName() string { return "req_head_timestamp" }

type HeadTimestamp struct {
	ReqID     int
	Timestamp string
}

func (HeadTimestamp) messageName() string { return "head_timestamp" }

type CancelHeadTimestamp struct {
	ReqID int
}

func (CancelHeadTimestamp) messageName() string { return "cancel_head_timestamp" }

// HistogramData (OUT 88 / cancel OUT 89 / IN 89)

type HistogramDataRequest struct {
	ReqID    int
	Contract Contract
	UseRTH   bool
	Period   string
}

func (HistogramDataRequest) messageName() string { return "req_histogram_data" }

type CancelHistogramData struct {
	ReqID int
}

func (CancelHistogramData) messageName() string { return "cancel_histogram_data" }

type HistogramDataEntry struct {
	Price string
	Size  string
}

type HistogramDataResponse struct {
	ReqID   int
	Entries []HistogramDataEntry
}

func (HistogramDataResponse) messageName() string { return "histogram_data" }

// HistoricalTicks (OUT 96 / IN 96,97,98)

type HistoricalTicksRequest struct {
	ReqID         int
	Contract      Contract
	StartDateTime string
	EndDateTime   string
	NumberOfTicks int
	WhatToShow    string
	UseRTH        bool
	IgnoreSize    bool
}

func (HistoricalTicksRequest) messageName() string { return "req_historical_ticks" }

type HistoricalTickEntry struct {
	Time  string
	Price string
	Size  string
}

type HistoricalTicksResponse struct {
	ReqID int
	Ticks []HistoricalTickEntry
	Done  bool
}

func (HistoricalTicksResponse) messageName() string { return "historical_ticks" }

type HistoricalTickBidAskEntry struct {
	TickAttrib int
	Time       string
	BidPrice   string
	AskPrice   string
	BidSize    string
	AskSize    string
}

type HistoricalTicksBidAskResponse struct {
	ReqID int
	Ticks []HistoricalTickBidAskEntry
	Done  bool
}

func (HistoricalTicksBidAskResponse) messageName() string { return "historical_ticks_bid_ask" }

type HistoricalTickLastEntry struct {
	TickAttrib        int
	Time              string
	Price             string
	Size              string
	Exchange          string
	SpecialConditions string
}

type HistoricalTicksLastResponse struct {
	ReqID int
	Ticks []HistoricalTickLastEntry
	Done  bool
}

func (HistoricalTicksLastResponse) messageName() string { return "historical_ticks_last" }

// HistoricalScheduleResponse is the decoded inbound response to a
// REQ_HISTORICAL_DATA request with whatToShow=SCHEDULE. Each session entry
// describes one contiguous trading window inside the requested duration.
// Live evidence: server_version 200 emits msg_id 106 with 5 header fields
// (reqID, startDateTime, endDateTime, timeZone, sessionCount) followed by
// 3 fields per session.
type HistoricalScheduleResponse struct {
	ReqID         int
	StartDateTime string
	EndDateTime   string
	TimeZone      string
	Sessions      []HistoricalScheduleSession
}

func (HistoricalScheduleResponse) messageName() string { return "historical_schedule" }

// HistoricalScheduleSession describes one trading session entry inside a
// HistoricalScheduleResponse. IBKR emits three string fields per session:
// StartDateTime, EndDateTime, and RefDate (the calendar date the session
// belongs to, useful when a session crosses midnight).
type HistoricalScheduleSession struct {
	StartDateTime string
	EndDateTime   string
	RefDate       string
}

// Historical data update (IN 108) — streaming bar for keepUpToDate

type HistoricalDataUpdate struct {
	ReqID    int
	BarCount int
	Time     string
	Open     string
	High     string
	Low      string
	Close    string
	Volume   string
	WAP      string
	Count    string
}

func (HistoricalDataUpdate) messageName() string { return "historical_data_update" }

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

type HistoricalBarsEnd struct {
	ReqID int
	// StartDate and EndDate carry the dataset bounds echoed by the terminal
	// IN 108 marker. Some live captures omit EndDate.
	StartDate string
	EndDate   string
}

type CancelHistoricalData struct {
	ReqID int
}

type HeadTimestampRequest struct {
	ReqID      int
	Contract   Contract
	WhatToShow string
	UseRTH     bool
}

type HeadTimestamp struct {
	ReqID     int
	Timestamp string
}

type CancelHeadTimestamp struct {
	ReqID int
}

// HistogramData (OUT 88 / cancel OUT 89 / IN 89)

type HistogramDataRequest struct {
	ReqID    int
	Contract Contract
	UseRTH   bool
	Period   string
}

type CancelHistogramData struct {
	ReqID int
}

type HistogramDataEntry struct {
	Price string
	Size  string
}

type HistogramDataResponse struct {
	ReqID   int
	Entries []HistogramDataEntry
}

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

// HistoricalScheduleResponse is the decoded inbound response to a
// REQ_HISTORICAL_DATA request with whatToShow=SCHEDULE. Each session entry
// describes one contiguous trading window inside the requested duration.
// The captured classic layout emits msg_id 106 with 5 header fields
// (reqID, startDateTime, endDateTime, timeZone, sessionCount) followed by
// 3 fields per session.
type HistoricalScheduleResponse struct {
	ReqID         int
	StartDateTime string
	EndDateTime   string
	TimeZone      string
	Sessions      []HistoricalScheduleSession
}

// HistoricalScheduleSession describes one trading session entry inside a
// HistoricalScheduleResponse. IBKR emits three string fields per session:
// StartDateTime, EndDateTime, and RefDate (the calendar date the session
// belongs to, useful when a session crosses midnight).
type HistoricalScheduleSession struct {
	StartDateTime string
	EndDateTime   string
	RefDate       string
}

// Historical data update (IN 90) — streaming bar for keepUpToDate. Unlike
// the packed IN 17 bars there is no per-bar trade count on the wire.

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
}

// [88, reqId, headTimestamp] — no version

// [89, reqID, count, entries(price, size)] — no version

// [96, reqID, count, entries(time, unused, price, size), done] — MIDPOINT

// [97, reqID, count, entries(time, attrib, bidPrice, askPrice, bidSize, askSize), done] — BID_ASK

// [98, reqID, count, entries(time, attrib, price, size, exchange, specialConditions), done] — TRADES

// [90, reqID, barCount, time, open, close, high, low, WAP, volume]
// Official HISTORICAL_DATA_UPDATE layout (note the open/close/high/low field
// order, unlike the packed IN 17 bars). Source-referenced from the official
// client library; an exact current protobuf callback remains pending.

// [108, reqID, startDateTime, endDateTime]
// Official HISTORICAL_DATA_END. Sent after the packed IN 17 batch. Some
// captures have a 2-field
// [reqID, startDateTime] shape, so the end date stays optional.

// [106, reqId, startDateTime, endDateTime, timeZone, sessionCount, (startDateTime,endDateTime,refDate)*count]

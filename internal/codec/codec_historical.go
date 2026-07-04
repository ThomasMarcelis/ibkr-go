package codec

import (
	"strconv"
)

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

// [17, reqID, barCount, time, O, H, L, C, vol, wap, count, ...]
func decodeHistoricalData(r *fieldReader) ([]Message, error) {
	reqID, err := r.ReadInt()
	if err != nil {
		return nil, err
	}
	barCount, err := r.ReadCount("bar count")
	if err != nil {
		return nil, err
	}
	if barCount <= 0 {
		return []Message{HistoricalBarsEnd{ReqID: reqID}}, nil
	}
	if err := r.RequireFixedEntryFields("historical data", barCount, 8, 0); err != nil {
		return nil, err
	}
	msgs := make([]Message, 0, barCount+1)
	for i := 0; i < barCount; i++ {
		msgs = append(msgs, HistoricalBar{
			ReqID: reqID, Time: r.ReadString(),
			Open: r.ReadString(), High: r.ReadString(),
			Low: r.ReadString(), Close: r.ReadString(),
			Volume: r.ReadString(), WAP: r.ReadString(), Count: r.ReadString(),
		})
	}
	msgs = append(msgs, HistoricalBarsEnd{ReqID: reqID})
	return msgs, nil
}

// [88, reqId, headTimestamp] — no version
func decodeHeadTimestamp(r *fieldReader) ([]Message, error) {
	reqID, _ := r.ReadInt()
	timestamp := r.ReadString()
	return []Message{HeadTimestamp{ReqID: reqID, Timestamp: timestamp}}, nil
}

// [89, reqID, count, entries(price, size)] — no version
func decodeHistogramData(r *fieldReader) ([]Message, error) {
	reqID, _ := r.ReadInt()
	count, err := r.ReadCount("histogram entry count")
	if err != nil {
		return nil, err
	}
	if err := r.RequireFixedEntryFields("histogram data", count, 2, 0); err != nil {
		return nil, err
	}
	entries := make([]HistogramDataEntry, count)
	for i := range entries {
		entries[i] = HistogramDataEntry{Price: r.ReadString(), Size: r.ReadString()}
	}
	return []Message{HistogramDataResponse{ReqID: reqID, Entries: entries}}, nil
}

// [96, reqID, count, entries(time, unused, price, size), done] — MIDPOINT
func decodeHistoricalTicks(r *fieldReader) ([]Message, error) {
	reqID, _ := r.ReadInt()
	count, err := r.ReadCount("historical midpoint tick count")
	if err != nil {
		return nil, err
	}
	if err := r.RequireFixedEntryFields("historical midpoint ticks", count, 4, 1); err != nil {
		return nil, err
	}
	ticks := make([]HistoricalTickEntry, count)
	for i := range ticks {
		timeStr := r.ReadString()
		r.Skip(1) // unused
		price := r.ReadString()
		size := r.ReadString()
		ticks[i] = HistoricalTickEntry{Time: timeStr, Price: price, Size: size}
	}
	done, _ := r.ReadBool()
	return []Message{HistoricalTicksResponse{ReqID: reqID, Ticks: ticks, Done: done}}, nil
}

// [97, reqID, count, entries(time, attrib, bidPrice, askPrice, bidSize, askSize), done] — BID_ASK
func decodeHistoricalTicksBidAsk(r *fieldReader) ([]Message, error) {
	reqID, _ := r.ReadInt()
	count, err := r.ReadCount("historical bid/ask tick count")
	if err != nil {
		return nil, err
	}
	if err := r.RequireFixedEntryFields("historical bid/ask ticks", count, 6, 1); err != nil {
		return nil, err
	}
	ticks := make([]HistoricalTickBidAskEntry, count)
	for i := range ticks {
		timeStr := r.ReadString()
		tickAttrib, _ := r.ReadInt()
		bidPrice := r.ReadString()
		askPrice := r.ReadString()
		bidSize := r.ReadString()
		askSize := r.ReadString()
		ticks[i] = HistoricalTickBidAskEntry{Time: timeStr, TickAttrib: tickAttrib, BidPrice: bidPrice, AskPrice: askPrice, BidSize: bidSize, AskSize: askSize}
	}
	done, _ := r.ReadBool()
	return []Message{HistoricalTicksBidAskResponse{ReqID: reqID, Ticks: ticks, Done: done}}, nil
}

// [98, reqID, count, entries(time, attrib, price, size, exchange, specialConditions), done] — TRADES
func decodeHistoricalTicksLast(r *fieldReader) ([]Message, error) {
	reqID, _ := r.ReadInt()
	count, err := r.ReadCount("historical trade tick count")
	if err != nil {
		return nil, err
	}
	if err := r.RequireFixedEntryFields("historical trade ticks", count, 6, 1); err != nil {
		return nil, err
	}
	ticks := make([]HistoricalTickLastEntry, count)
	for i := range ticks {
		timeStr := r.ReadString()
		tickAttrib, _ := r.ReadInt()
		price := r.ReadString()
		size := r.ReadString()
		exchange := r.ReadString()
		specialConditions := r.ReadString()
		ticks[i] = HistoricalTickLastEntry{Time: timeStr, TickAttrib: tickAttrib, Price: price, Size: size, Exchange: exchange, SpecialConditions: specialConditions}
	}
	done, _ := r.ReadBool()
	return []Message{HistoricalTicksLastResponse{ReqID: reqID, Ticks: ticks, Done: done}}, nil
}

func decodeHistoricalDataUpdate(r *fieldReader) ([]Message, error) {
	// Live Gateway v200 sends two distinct msg_id 108 shapes:
	//   [108, reqID, barCount, time, O, H, L, C, vol, wap, count]
	//   [108, reqID, startDateTime, endDateTime]
	// Older captures also show [108, reqID, startDateTime]. The range
	// shapes are terminal markers for the preceding historical data batch.
	if (len(r.fields) == 2 || len(r.fields) == 3) && isWireInt(r.fields[0]) && isHistoricalRangeBoundary(r.fields[1]) {
		reqID, _ := strconv.Atoi(r.fields[0])
		return []Message{HistoricalBarsEnd{ReqID: reqID}}, nil
	}
	reqID, err := r.ReadInt()
	if err != nil {
		return nil, err
	}
	barCount, err := r.ReadCount("historical data update bar count")
	if err != nil {
		return nil, err
	}
	if err := r.RequireFixedEntryFields("historical data update", 1, 8, 0); err != nil {
		return nil, err
	}
	return []Message{HistoricalDataUpdate{
		ReqID: reqID, BarCount: barCount,
		Time: r.ReadString(), Open: r.ReadString(), High: r.ReadString(),
		Low: r.ReadString(), Close: r.ReadString(), Volume: r.ReadString(),
		WAP: r.ReadString(), Count: r.ReadString(),
	}}, nil
}

// [106, reqId, startDateTime, endDateTime, timeZone, sessionCount, (startDateTime,endDateTime,refDate)*count]
func decodeHistoricalSchedule(r *fieldReader) ([]Message, error) {
	reqID, _ := r.ReadInt()
	startDateTime := r.ReadString()
	endDateTime := r.ReadString()
	timeZone := r.ReadString()
	sessionCount, err := r.ReadCount("historical schedule session count")
	if err != nil {
		return nil, err
	}
	if err := r.RequireFixedEntryFields("historical schedule", sessionCount, 3, 0); err != nil {
		return nil, err
	}
	sessions := make([]HistoricalScheduleSession, sessionCount)
	for i := 0; i < sessionCount; i++ {
		sessions[i] = HistoricalScheduleSession{
			StartDateTime: r.ReadString(),
			EndDateTime:   r.ReadString(),
			RefDate:       r.ReadString(),
		}
	}
	return []Message{HistoricalScheduleResponse{
		ReqID:         reqID,
		StartDateTime: startDateTime,
		EndDateTime:   endDateTime,
		TimeZone:      timeZone,
		Sessions:      sessions,
	}}, nil
}

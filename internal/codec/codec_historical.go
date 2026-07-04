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

func (m HistoricalBarsRequest) encodeWire() ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(OutReqHistoricalData)
	w.WriteInt(m.ReqID)
	w.WriteInt(m.Contract.ConID)
	writeWireContract(&w, m.Contract)
	w.WriteBool(false) // includeExpired
	w.WriteString(m.EndDateTime)
	w.WriteString(m.BarSize)
	w.WriteString(m.Duration)
	w.WriteBool(m.UseRTH)
	w.WriteString(m.WhatToShow)
	w.WriteInt(1) // formatDate
	w.WriteBool(m.KeepUpToDate)
	w.WriteString("") // chartOptions
	return w.Fields(), nil
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
}

type CancelHistoricalData struct {
	ReqID int
}

func (m CancelHistoricalData) encodeWire() ([]string, error) {
	return []string{itoa(OutCancelHistoricalData), "1", itoa(m.ReqID)}, nil
}

type HeadTimestampRequest struct {
	ReqID      int
	Contract   Contract
	WhatToShow string
	UseRTH     bool
}

func (m HeadTimestampRequest) encodeWire() ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(OutReqHeadTimestamp)
	w.WriteInt(m.ReqID)
	w.WriteInt(m.Contract.ConID)
	writeWireContract(&w, m.Contract)
	w.WriteBool(false) // includeExpired
	w.WriteBool(m.UseRTH)
	w.WriteString(m.WhatToShow)
	w.WriteInt(1) // formatDate
	return w.Fields(), nil
}

type HeadTimestamp struct {
	ReqID     int
	Timestamp string
}

type CancelHeadTimestamp struct {
	ReqID int
}

func (m CancelHeadTimestamp) encodeWire() ([]string, error) {
	return []string{itoa(OutCancelHeadTimestamp), itoa(m.ReqID)}, nil
}

// HistogramData (OUT 88 / cancel OUT 89 / IN 89)

type HistogramDataRequest struct {
	ReqID    int
	Contract Contract
	UseRTH   bool
	Period   string
}

func (m HistogramDataRequest) encodeWire() ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(OutReqHistogramData)
	w.WriteInt(m.ReqID)
	w.WriteInt(m.Contract.ConID)
	writeWireContract(&w, m.Contract)
	w.WriteBool(false) // includeExpired
	w.WriteBool(m.UseRTH)
	w.WriteString(m.Period)
	return w.Fields(), nil
}

type CancelHistogramData struct {
	ReqID int
}

func (m CancelHistogramData) encodeWire() ([]string, error) {
	return []string{itoa(OutCancelHistogramData), itoa(m.ReqID)}, nil
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

func (m HistoricalTicksRequest) encodeWire() ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(OutReqHistoricalTicks)
	w.WriteInt(m.ReqID)
	w.WriteInt(m.Contract.ConID)
	writeWireContract(&w, m.Contract)
	w.WriteBool(false) // includeExpired
	w.WriteString(m.StartDateTime)
	w.WriteString(m.EndDateTime)
	w.WriteInt(m.NumberOfTicks)
	w.WriteString(m.WhatToShow)
	w.WriteBool(m.UseRTH)
	w.WriteBool(m.IgnoreSize)
	w.WriteString("") // miscOptions
	return w.Fields(), nil
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

func (m HistoricalBar) encodeWire() ([]string, error) {
	return []string{
		itoa(InHistoricalData), itoa(m.ReqID), "1",
		m.Time, m.Open, m.High, m.Low, m.Close, m.Volume, m.WAP, m.Count,
	}, nil
}

func (m HistoricalBarsEnd) encodeWire() ([]string, error) {
	return []string{itoa(InHistoricalData), itoa(m.ReqID), "0"}, nil
}

// [88, reqId, headTimestamp] — no version
func decodeHeadTimestamp(r *fieldReader) ([]Message, error) {
	reqID, _ := r.ReadInt()
	timestamp := r.ReadString()
	return []Message{HeadTimestamp{ReqID: reqID, Timestamp: timestamp}}, nil
}

func (m HeadTimestamp) encodeWire() ([]string, error) {
	return []string{itoa(InHeadTimestamp), itoa(m.ReqID), m.Timestamp}, nil
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

func (m HistogramDataResponse) encodeWire() ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(InHistogramData)
	w.WriteInt(m.ReqID)
	w.WriteInt(len(m.Entries))
	for _, e := range m.Entries {
		w.WriteString(e.Price)
		w.WriteString(e.Size)
	}
	return w.Fields(), nil
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

func (m HistoricalTicksResponse) encodeWire() ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(InHistoricalTicks)
	w.WriteInt(m.ReqID)
	w.WriteInt(len(m.Ticks))
	for _, t := range m.Ticks {
		w.WriteString(t.Time)
		w.WriteString("") // unused
		w.WriteString(t.Price)
		w.WriteString(t.Size)
	}
	w.WriteBool(m.Done)
	return w.Fields(), nil
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

func (m HistoricalTicksBidAskResponse) encodeWire() ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(InHistoricalTicksBidAsk)
	w.WriteInt(m.ReqID)
	w.WriteInt(len(m.Ticks))
	for _, t := range m.Ticks {
		w.WriteString(t.Time)
		w.WriteInt(t.TickAttrib)
		w.WriteString(t.BidPrice)
		w.WriteString(t.AskPrice)
		w.WriteString(t.BidSize)
		w.WriteString(t.AskSize)
	}
	w.WriteBool(m.Done)
	return w.Fields(), nil
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

func (m HistoricalTicksLastResponse) encodeWire() ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(InHistoricalTicksLast)
	w.WriteInt(m.ReqID)
	w.WriteInt(len(m.Ticks))
	for _, t := range m.Ticks {
		w.WriteString(t.Time)
		w.WriteInt(t.TickAttrib)
		w.WriteString(t.Price)
		w.WriteString(t.Size)
		w.WriteString(t.Exchange)
		w.WriteString(t.SpecialConditions)
	}
	w.WriteBool(m.Done)
	return w.Fields(), nil
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

func (m HistoricalDataUpdate) encodeWire() ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(InHistoricalDataUpdate)
	w.WriteInt(m.ReqID)
	w.WriteInt(m.BarCount)
	w.WriteString(m.Time)
	w.WriteString(m.Open)
	w.WriteString(m.High)
	w.WriteString(m.Low)
	w.WriteString(m.Close)
	w.WriteString(m.Volume)
	w.WriteString(m.WAP)
	w.WriteString(m.Count)
	return w.Fields(), nil
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

func (m HistoricalScheduleResponse) encodeWire() ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(InHistoricalSchedule)
	w.WriteInt(m.ReqID)
	w.WriteString(m.StartDateTime)
	w.WriteString(m.EndDateTime)
	w.WriteString(m.TimeZone)
	w.WriteInt(len(m.Sessions))
	for _, s := range m.Sessions {
		w.WriteString(s.StartDateTime)
		w.WriteString(s.EndDateTime)
		w.WriteString(s.RefDate)
	}
	return w.Fields(), nil
}

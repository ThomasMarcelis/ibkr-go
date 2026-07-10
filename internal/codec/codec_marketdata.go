package codec

import "strings"

type QuoteRequest struct {
	ReqID        int
	Contract     Contract
	Snapshot     bool
	GenericTicks []string
}

func (m QuoteRequest) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(OutReqMktData)
	w.WriteInt(11) // version
	w.WriteInt(m.ReqID)
	w.WriteInt(m.Contract.ConID)
	writeWireContract(&w, m.Contract)
	if m.Contract.SecType == "BAG" {
		w.WriteInt(len(m.Contract.ComboLegs))
		for _, leg := range m.Contract.ComboLegs {
			w.WriteInt(leg.ConID)
			w.WriteInt(leg.Ratio)
			w.WriteString(leg.Action)
			w.WriteString(leg.Exchange)
		}
	}
	w.WriteBool(m.Contract.DeltaNeutral != nil)
	if m.Contract.DeltaNeutral != nil {
		w.WriteInt(m.Contract.DeltaNeutral.ConID)
		w.WriteString(m.Contract.DeltaNeutral.Delta)
		w.WriteString(m.Contract.DeltaNeutral.Price)
	}
	w.WriteString(strings.Join(m.GenericTicks, ","))
	w.WriteBool(m.Snapshot)
	w.WriteBool(false) // regulatorySnapshot
	w.WriteString("")  // mktDataOptions
	return w.Fields(), nil
}

type CancelQuote struct {
	ReqID int
}

func (m CancelQuote) encodeWire(sv int) ([]string, error) {
	return []string{itoa(OutCancelMktData), "1", itoa(m.ReqID)}, nil
}

type TickPrice struct {
	ReqID    int
	TickType int
	Price    string
	Size     string // companion size from the same frame
	AttrMask int    // tick attrib bitmask
}

type TickSize struct {
	ReqID    int
	TickType int
	Size     string
}

type MarketDataType struct {
	ReqID    int
	DataType int
}

type TickSnapshotEnd struct {
	ReqID int
}

type RealTimeBarsRequest struct {
	ReqID      int
	Contract   Contract
	WhatToShow string
	UseRTH     bool
}

func (m RealTimeBarsRequest) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(OutReqRealTimeBars)
	w.WriteInt(3) // version
	w.WriteInt(m.ReqID)
	w.WriteInt(m.Contract.ConID)
	writeWireContract(&w, m.Contract)
	w.WriteInt(5) // barSize (always 5 sec)
	w.WriteString(m.WhatToShow)
	w.WriteBool(m.UseRTH)
	w.WriteString("") // options
	return w.Fields(), nil
}

type CancelRealTimeBars struct {
	ReqID int
}

func (m CancelRealTimeBars) encodeWire(sv int) ([]string, error) {
	return []string{itoa(OutCancelRealTimeBars), "1", itoa(m.ReqID)}, nil
}

type RealTimeBar struct {
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

type TickGeneric struct {
	ReqID    int
	TickType int
	Value    string
}

type TickString struct {
	ReqID    int
	TickType int
	Value    string
}

type TickReqParams struct {
	ReqID               int
	MinTick             string
	BBOExchange         string
	SnapshotPermissions *int
	LastPricePrecision  string
	LastSizePrecision   string
}

type MarketDataReroute struct {
	ReqID    int
	ConID    int
	Exchange string
}

type MarketDepthReroute struct {
	ReqID    int
	ConID    int
	Exchange string
}

type ReqMarketDataType struct {
	DataType int
}

func (m ReqMarketDataType) encodeWire(sv int) ([]string, error) {
	return []string{itoa(OutReqMarketDataType), "1", itoa(m.DataType)}, nil
}

type MktDepthExchangesRequest struct{}

func (m MktDepthExchangesRequest) encodeWire(sv int) ([]string, error) {
	return []string{itoa(OutReqMktDepthExchanges)}, nil
}

type MktDepthExchanges struct {
	Exchanges []DepthExchangeEntry
}

type DepthExchangeEntry struct {
	Exchange        string
	SecType         string
	ListingExch     string
	ServiceDataType string
	AggGroup        int
}

// Tick by tick (OUT 97, cancel OUT 98 / IN 99)

type TickByTickRequest struct {
	ReqID         int
	Contract      Contract
	TickType      string
	NumberOfTicks int
	IgnoreSize    bool
}

func (m TickByTickRequest) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(OutReqTickByTickData)
	w.WriteInt(m.ReqID)
	w.WriteInt(m.Contract.ConID)
	writeWireContract(&w, m.Contract)
	w.WriteString(m.TickType)
	w.WriteInt(m.NumberOfTicks)
	w.WriteBool(m.IgnoreSize)
	return w.Fields(), nil
}

type CancelTickByTick struct {
	ReqID int
}

func (m CancelTickByTick) encodeWire(sv int) ([]string, error) {
	return []string{itoa(OutCancelTickByTickData), itoa(m.ReqID)}, nil
}

type TickByTickData struct {
	ReqID             int
	TickType          int
	Time              string
	Price             string
	Size              string
	Exchange          string
	SpecialConditions string
	BidPrice          string
	AskPrice          string
	BidSize           string
	AskSize           string
	MidPoint          string
	// TickAttrib bitmasks
	TickAttribLast   int
	TickAttribBidAsk int
}

// CalcImpliedVolatility (OUT 54 / cancel OUT 56) / CalcOptionPrice (OUT 55 / cancel OUT 57)

type CalcImpliedVolatilityRequest struct {
	ReqID       int
	Contract    Contract
	OptionPrice string
	UnderPrice  string
}

func (m CalcImpliedVolatilityRequest) encodeWire(sv int) ([]string, error) {
	// No includeExpired field: the live sv200 Gateway parses optionPrice
	// directly after tradingClass (code 320 evidence, capture
	// 20260611T074859Z, sha 241a49023701e9ec).
	w := fieldWriter{}
	w.WriteInt(OutReqCalcImpliedVolatility)
	w.WriteInt(3) // version
	w.WriteInt(m.ReqID)
	w.WriteInt(m.Contract.ConID)
	writeWireContract(&w, m.Contract)
	w.WriteString(m.OptionPrice)
	w.WriteString(m.UnderPrice)
	w.WriteString("") // implVolOptions
	return w.Fields(), nil
}

type CancelCalcImpliedVolatility struct {
	ReqID int
}

func (m CancelCalcImpliedVolatility) encodeWire(sv int) ([]string, error) {
	return []string{itoa(OutCancelCalcImpliedVolatility), "1", itoa(m.ReqID)}, nil
}

type CalcOptionPriceRequest struct {
	ReqID      int
	Contract   Contract
	Volatility string
	UnderPrice string
}

func (m CalcOptionPriceRequest) encodeWire(sv int) ([]string, error) {
	// No includeExpired field; see CalcImpliedVolatilityRequest.
	w := fieldWriter{}
	w.WriteInt(OutReqCalcOptionPrice)
	// Official REQ_CALC_OPTION_PRICE version is 2 (3 belongs to the
	// implied-volatility request); the live Gateway tolerated 3 but the
	// official client is the conformance contract.
	w.WriteInt(2) // version
	w.WriteInt(m.ReqID)
	w.WriteInt(m.Contract.ConID)
	writeWireContract(&w, m.Contract)
	w.WriteString(m.Volatility)
	w.WriteString(m.UnderPrice)
	w.WriteString("") // optPxOptions
	return w.Fields(), nil
}

type CancelCalcOptionPrice struct {
	ReqID int
}

func (m CancelCalcOptionPrice) encodeWire(sv int) ([]string, error) {
	return []string{itoa(OutCancelCalcOptionPrice), "1", itoa(m.ReqID)}, nil
}

type TickOptionComputation struct {
	ReqID      int
	TickType   int
	TickAttrib int
	ImpliedVol string
	Delta      string
	OptPrice   string
	PvDividend string
	Gamma      string
	Vega       string
	Theta      string
	UndPrice   string
}

// Market depth (OUT 10, cancel OUT 11 / IN 12, 13)

type MarketDepthRequest struct {
	ReqID        int
	Contract     Contract
	NumRows      int
	IsSmartDepth bool
}

func (m MarketDepthRequest) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(OutReqMktDepth)
	w.WriteInt(5) // version
	w.WriteInt(m.ReqID)
	w.WriteInt(m.Contract.ConID)
	writeWireContract(&w, m.Contract)
	w.WriteInt(m.NumRows)
	w.WriteBool(m.IsSmartDepth)
	w.WriteString("") // mktDepthOptions
	return w.Fields(), nil
}

type CancelMarketDepth struct {
	ReqID        int
	IsSmartDepth bool
}

func (m CancelMarketDepth) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(OutCancelMktDepth)
	w.WriteInt(1)
	w.WriteInt(m.ReqID)
	if sv >= MinServerVersionSmartDepth {
		w.WriteBool(m.IsSmartDepth)
	}
	return w.Fields(), nil
}

type MarketDepthUpdate struct {
	ReqID     int
	Position  int
	Operation int // 0=insert, 1=update, 2=delete
	Side      int // 0=ask, 1=bid
	Price     string
	Size      string
}

type MarketDepthL2Update struct {
	ReqID        int
	Position     int
	MarketMaker  string
	Operation    int
	Side         int
	Price        string
	Size         string
	IsSmartDepth bool
}

// [1, version, reqID, tickType, price, size, attrMask]
func decodeTickPrice(r *fieldReader, sv int) ([]Message, error) {
	r.Skip(1) // version
	reqID, _ := r.ReadInt()
	tickType, _ := r.ReadInt()
	price := r.ReadString()
	size := r.ReadString()
	attrMask, _ := r.ReadInt()
	return []Message{TickPrice{ReqID: reqID, TickType: tickType, Price: price, Size: size, AttrMask: attrMask}}, nil
}

func (m TickPrice) encodeWire(sv int) ([]string, error) {
	return []string{itoa(InTickPrice), "6", itoa(m.ReqID), itoa(m.TickType), m.Price, m.Size, itoa(m.AttrMask)}, nil
}

// [2, version, reqID, tickType, size]
func decodeTickSize(r *fieldReader, sv int) ([]Message, error) {
	r.Skip(1) // version
	reqID, _ := r.ReadInt()
	tickType, _ := r.ReadInt()
	size := r.ReadString()
	return []Message{TickSize{ReqID: reqID, TickType: tickType, Size: size}}, nil
}

func (m TickSize) encodeWire(sv int) ([]string, error) {
	return []string{itoa(InTickSize), "6", itoa(m.ReqID), itoa(m.TickType), m.Size}, nil
}

// [12, version, reqID, position, operation, side, price, size]
func decodeMarketDepth(r *fieldReader, sv int) ([]Message, error) {
	r.Skip(1) // version
	reqID, _ := r.ReadInt()
	position, _ := r.ReadInt()
	operation, _ := r.ReadInt()
	side, _ := r.ReadInt()
	price := r.ReadString()
	size := r.ReadString()
	return []Message{MarketDepthUpdate{ReqID: reqID, Position: position, Operation: operation, Side: side, Price: price, Size: size}}, nil
}

func (m MarketDepthUpdate) encodeWire(sv int) ([]string, error) {
	return []string{itoa(InMarketDepth), "6", itoa(m.ReqID), itoa(m.Position), itoa(m.Operation), itoa(m.Side), m.Price, m.Size}, nil
}

// [13, version, reqID, position, marketMaker, operation, side, price, size, isSmartDepth]
func decodeMarketDepthL2(r *fieldReader, sv int) ([]Message, error) {
	r.Skip(1) // version
	reqID, _ := r.ReadInt()
	position, _ := r.ReadInt()
	marketMaker := r.ReadString()
	operation, _ := r.ReadInt()
	side, _ := r.ReadInt()
	price := r.ReadString()
	size := r.ReadString()
	isSmartDepth, _ := r.ReadBool()
	return []Message{MarketDepthL2Update{ReqID: reqID, Position: position, MarketMaker: marketMaker, Operation: operation, Side: side, Price: price, Size: size, IsSmartDepth: isSmartDepth}}, nil
}

func (m MarketDepthL2Update) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(InMarketDepthL2)
	w.WriteInt(6) // version
	w.WriteInt(m.ReqID)
	w.WriteInt(m.Position)
	w.WriteString(m.MarketMaker)
	w.WriteInt(m.Operation)
	w.WriteInt(m.Side)
	w.WriteString(m.Price)
	w.WriteString(m.Size)
	w.WriteBool(m.IsSmartDepth)
	return w.Fields(), nil
}

// [21, reqID, tickType, tickAttrib, impliedVol, delta, optPrice, pvDividend, gamma, vega, theta, undPrice]
func decodeTickOptionComputation(r *fieldReader, sv int) ([]Message, error) {
	// No version field on the live sv200 wire; the legacy version skip
	// consumed the request id and killed the session on the first real
	// greeks reply (capture 20260611T075300Z-api_option_campaign_aapl).
	reqID, _ := r.ReadInt()
	tickType, _ := r.ReadInt()
	tickAttrib, _ := r.ReadInt()
	impliedVol := r.ReadString()
	delta := r.ReadString()
	optPrice := r.ReadString()
	pvDividend := r.ReadString()
	gamma := r.ReadString()
	vega := r.ReadString()
	theta := r.ReadString()
	undPrice := r.ReadString()
	return []Message{TickOptionComputation{
		ReqID: reqID, TickType: tickType, TickAttrib: tickAttrib,
		ImpliedVol: impliedVol, Delta: delta, OptPrice: optPrice,
		PvDividend: pvDividend, Gamma: gamma, Vega: vega,
		Theta: theta, UndPrice: undPrice,
	}}, nil
}

func (m TickOptionComputation) encodeWire(sv int) ([]string, error) {
	return []string{
		itoa(InTickOptionComputation), itoa(m.ReqID), itoa(m.TickType), itoa(m.TickAttrib),
		m.ImpliedVol, m.Delta, m.OptPrice, m.PvDividend, m.Gamma, m.Vega, m.Theta, m.UndPrice,
	}, nil
}

// [45, version, reqID, tickType, value]
func decodeTickGeneric(r *fieldReader, sv int) ([]Message, error) {
	r.Skip(1) // version
	reqID, _ := r.ReadInt()
	tickType, _ := r.ReadInt()
	value := r.ReadString()
	return []Message{TickGeneric{ReqID: reqID, TickType: tickType, Value: value}}, nil
}

func (m TickGeneric) encodeWire(sv int) ([]string, error) {
	return []string{itoa(InTickGeneric), "6", itoa(m.ReqID), itoa(m.TickType), m.Value}, nil
}

// [46, version, reqID, tickType, value]
func decodeTickString(r *fieldReader, sv int) ([]Message, error) {
	r.Skip(1) // version
	reqID, _ := r.ReadInt()
	tickType, _ := r.ReadInt()
	value := r.ReadString()
	return []Message{TickString{ReqID: reqID, TickType: tickType, Value: value}}, nil
}

func (m TickString) encodeWire(sv int) ([]string, error) {
	return []string{itoa(InTickString), "6", itoa(m.ReqID), itoa(m.TickType), m.Value}, nil
}

// [81, reqID, minTick, bboExchange, snapshotPermissions] — no version
func decodeTickReqParams(r *fieldReader, sv int) ([]Message, error) {
	reqID, _ := r.ReadInt()
	minTick := r.ReadString()
	bboExchange := r.ReadString()
	snapshotPermissions, _ := r.ReadInt()
	return []Message{TickReqParams{ReqID: reqID, MinTick: minTick, BBOExchange: bboExchange, SnapshotPermissions: new(snapshotPermissions)}}, nil
}

func (m TickReqParams) encodeWire(sv int) ([]string, error) {
	permissions := 0
	if m.SnapshotPermissions != nil {
		permissions = *m.SnapshotPermissions
	}
	return []string{itoa(InTickReqParams), itoa(m.ReqID), m.MinTick, m.BBOExchange, itoa(permissions)}, nil
}

// [91, reqID, conID, exchange] — no version, including after server_version 200.
func decodeMarketDataReroute(r *fieldReader, sv int) ([]Message, error) {
	reqID, _ := r.ReadInt()
	conID, _ := r.ReadInt()
	return []Message{MarketDataReroute{ReqID: reqID, ConID: conID, Exchange: r.ReadString()}}, nil
}

func (m MarketDataReroute) encodeWire(sv int) ([]string, error) {
	return []string{itoa(InMarketDataReroute), itoa(m.ReqID), itoa(m.ConID), m.Exchange}, nil
}

// [92, reqID, conID, exchange] — no version, including after server_version 200.
func decodeMarketDepthReroute(r *fieldReader, sv int) ([]Message, error) {
	reqID, _ := r.ReadInt()
	conID, _ := r.ReadInt()
	return []Message{MarketDepthReroute{ReqID: reqID, ConID: conID, Exchange: r.ReadString()}}, nil
}

func (m MarketDepthReroute) encodeWire(sv int) ([]string, error) {
	return []string{itoa(InMarketDepthReroute), itoa(m.ReqID), itoa(m.ConID), m.Exchange}, nil
}

// [50, version, reqID, time, O, H, L, C, vol, wap, count]
func decodeRealTimeBars(r *fieldReader, sv int) ([]Message, error) {
	r.Skip(1)
	reqID, _ := r.ReadInt()
	return []Message{RealTimeBar{
		ReqID: reqID, Time: r.ReadString(),
		Open: r.ReadString(), High: r.ReadString(), Low: r.ReadString(),
		Close: r.ReadString(), Volume: r.ReadString(),
		WAP: r.ReadString(), Count: r.ReadString(),
	}}, nil
}

func (m RealTimeBar) encodeWire(sv int) ([]string, error) {
	return []string{itoa(InRealTimeBars), "3", itoa(m.ReqID), m.Time, m.Open, m.High, m.Low, m.Close, m.Volume, m.WAP, m.Count}, nil
}

// [57, version, reqID]
func decodeTickSnapshotEnd(r *fieldReader, sv int) ([]Message, error) {
	r.Skip(1)
	reqID, _ := r.ReadInt()
	return []Message{TickSnapshotEnd{ReqID: reqID}}, nil
}

func (m TickSnapshotEnd) encodeWire(sv int) ([]string, error) {
	return []string{itoa(InTickSnapshotEnd), "1", itoa(m.ReqID)}, nil
}

// [58, version, reqID, dataType]
func decodeMarketDataType(r *fieldReader, sv int) ([]Message, error) {
	r.Skip(1)
	reqID, _ := r.ReadInt()
	dataType, _ := r.ReadInt()
	return []Message{MarketDataType{ReqID: reqID, DataType: dataType}}, nil
}

func (m MarketDataType) encodeWire(sv int) ([]string, error) {
	return []string{itoa(InMarketDataType), "1", itoa(m.ReqID), itoa(m.DataType)}, nil
}

func decodeMktDepthExchanges(r *fieldReader, sv int) ([]Message, error) {
	// MktDepthExchanges: [80, count, repeated(exchange, secType, listingExch, serviceDataType, aggGroup)] — no version
	count, err := r.ReadCount("depth exchange count")
	if err != nil {
		return nil, err
	}
	if err := r.RequireFixedEntryFields("market depth exchanges", count, 5, 0); err != nil {
		return nil, err
	}
	entries := make([]DepthExchangeEntry, count)
	for i := range entries {
		entries[i] = DepthExchangeEntry{
			Exchange: r.ReadString(), SecType: r.ReadString(),
			ListingExch: r.ReadString(), ServiceDataType: r.ReadString(),
		}
		entries[i].AggGroup, _ = r.ReadInt()
	}
	return []Message{MktDepthExchanges{Exchanges: entries}}, nil
}

func (m MktDepthExchanges) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(InMktDepthExchanges)
	w.WriteInt(len(m.Exchanges))
	for _, e := range m.Exchanges {
		w.WriteString(e.Exchange)
		w.WriteString(e.SecType)
		w.WriteString(e.ListingExch)
		w.WriteString(e.ServiceDataType)
		w.WriteInt(e.AggGroup)
	}
	return w.Fields(), nil
}

// [99, reqID, tickType, time, ...]
func decodeTickByTick(r *fieldReader, sv int) ([]Message, error) {
	reqID, _ := r.ReadInt()
	tickType, _ := r.ReadInt()
	timeStr := r.ReadString()
	tick := TickByTickData{ReqID: reqID, TickType: tickType, Time: timeStr}
	switch tickType {
	case 1, 2: // Last, AllLast
		tick.Price = r.ReadString()
		tick.Size = r.ReadString()
		tick.TickAttribLast, _ = r.ReadInt()
		tick.Exchange = r.ReadString()
		tick.SpecialConditions = r.ReadString()
	case 3: // BidAsk
		tick.BidPrice = r.ReadString()
		tick.AskPrice = r.ReadString()
		tick.BidSize = r.ReadString()
		tick.AskSize = r.ReadString()
		tick.TickAttribBidAsk, _ = r.ReadInt()
	case 4: // MidPoint
		tick.MidPoint = r.ReadString()
	}
	return []Message{tick}, nil
}

func (m TickByTickData) encodeWire(sv int) ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(InTickByTick)
	w.WriteInt(m.ReqID)
	w.WriteInt(m.TickType)
	w.WriteString(m.Time)
	switch m.TickType {
	case 1, 2: // Last, AllLast
		w.WriteString(m.Price)
		w.WriteString(m.Size)
		w.WriteInt(m.TickAttribLast)
		w.WriteString(m.Exchange)
		w.WriteString(m.SpecialConditions)
	case 3: // BidAsk
		w.WriteString(m.BidPrice)
		w.WriteString(m.AskPrice)
		w.WriteString(m.BidSize)
		w.WriteString(m.AskSize)
		w.WriteInt(m.TickAttribBidAsk)
	case 4: // MidPoint
		w.WriteString(m.MidPoint)
	}
	return w.Fields(), nil
}

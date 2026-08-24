package codec

import "github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"

type QuoteRequest struct {
	ReqID              int
	Contract           Contract
	Snapshot           bool
	RegulatorySnapshot bool
	GenericTicks       []string
}

type CancelQuote struct {
	ReqID int
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

type CancelRealTimeBars struct {
	ReqID int
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

type TickEFP struct {
	ReqID                    int
	TickType                 int
	BasisPoints              string
	FormattedBasisPoints     string
	ImpliedFuturesPrice      string
	HoldDays                 int
	FutureLastTradeDate      string
	DividendImpact           string
	DividendsToLastTradeDate string
}

type DeltaNeutralValidation struct {
	ReqID    int
	Contract DeltaNeutralContract
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

type MktDepthExchangesRequest struct{}

func (m MktDepthExchangesRequest) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutReqMktDepthExchanges)}, nil
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

type CancelTickByTick struct {
	ReqID int
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
	// No includeExpired field: the captured classic layout parses optionPrice
	// directly after tradingClass (sv210 capture
	// 20260825T203959Z-sv210_classic_option_calculations, events sha256
	// 510dedb3be94ed96c3201807cc7d91e0fcd9756e9f98444efa0dbb66faea2289).
	w := fieldWriter{}
	w.WriteInt(protocol.OutReqCalcImpliedVolatility)
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
	return []string{itoa(protocol.OutCancelCalcImpliedVolatility), "1", itoa(m.ReqID)}, nil
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
	w.WriteInt(protocol.OutReqCalcOptionPrice)
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
	return []string{itoa(protocol.OutCancelCalcOptionPrice), "1", itoa(m.ReqID)}, nil
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

type CancelMarketDepth struct {
	ReqID        int
	IsSmartDepth bool
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

// [21, reqID, tickType, tickAttrib, impliedVol, delta, optPrice, pvDividend, gamma, vega, theta, undPrice]

// [45, version, reqID, tickType, value]

// [46, version, reqID, tickType, value]

// [47, version, reqID, tickType, basisPoints, formattedBasisPoints,
// impliedFuturesPrice, holdDays, futureLastTradeDate, dividendImpact,
// dividendsToLastTradeDate]
func decodeTickEFP(r *fieldReader, sv int) ([]Message, error) {
	r.Skip(1) // version
	reqID, _ := r.ReadInt()
	tickType, _ := r.ReadInt()
	basisPoints := r.ReadString()
	formattedBasisPoints := r.ReadString()
	impliedFuturesPrice := r.ReadString()
	holdDays, _ := r.ReadInt()
	message := TickEFP{
		ReqID:                    reqID,
		TickType:                 tickType,
		BasisPoints:              basisPoints,
		FormattedBasisPoints:     formattedBasisPoints,
		ImpliedFuturesPrice:      impliedFuturesPrice,
		HoldDays:                 holdDays,
		FutureLastTradeDate:      r.ReadString(),
		DividendImpact:           r.ReadString(),
		DividendsToLastTradeDate: r.ReadString(),
	}
	return []Message{message}, nil
}

func decodeDeltaNeutralValidation(r *fieldReader, sv int) ([]Message, error) {
	r.Skip(1) // version
	reqID, _ := r.ReadInt()
	conID, _ := r.ReadInt()
	message := DeltaNeutralValidation{
		ReqID: reqID,
		Contract: DeltaNeutralContract{
			ConID: conID,
			Delta: r.ReadString(),
			Price: r.ReadString(),
		},
	}
	return []Message{message}, nil
}

// [81, reqID, minTick, bboExchange, snapshotPermissions] — no version

// [91, reqID, conID, exchange] — no version.

// [92, reqID, conID, exchange] — no version.

// [50, version, reqID, time, O, H, L, C, vol, wap, count]

// [57, version, reqID]

// [58, version, reqID, dataType]

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

// [99, reqID, tickType, time, ...]

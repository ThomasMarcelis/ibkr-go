package codec

type QuoteRequest struct {
	ReqID        int
	Contract     Contract
	Snapshot     bool
	GenericTicks []string
}

func (QuoteRequest) messageName() string { return "req_quote" }

type CancelQuote struct {
	ReqID int
}

func (CancelQuote) messageName() string { return "cancel_quote" }

type TickPrice struct {
	ReqID    int
	TickType int
	Price    string
	Size     string // companion size from the same frame
	AttrMask int    // tick attrib bitmask
}

func (TickPrice) messageName() string { return "tick_price" }

type TickSize struct {
	ReqID    int
	TickType int
	Size     string
}

func (TickSize) messageName() string { return "tick_size" }

type MarketDataType struct {
	ReqID    int
	DataType int
}

func (MarketDataType) messageName() string { return "market_data_type" }

type TickSnapshotEnd struct {
	ReqID int
}

func (TickSnapshotEnd) messageName() string { return "tick_snapshot_end" }

type RealTimeBarsRequest struct {
	ReqID      int
	Contract   Contract
	WhatToShow string
	UseRTH     bool
}

func (RealTimeBarsRequest) messageName() string { return "req_realtime_bars" }

type CancelRealTimeBars struct {
	ReqID int
}

func (CancelRealTimeBars) messageName() string { return "cancel_realtime_bars" }

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

func (RealTimeBar) messageName() string { return "realtime_bar" }

type TickGeneric struct {
	ReqID    int
	TickType int
	Value    string
}

func (TickGeneric) messageName() string { return "tick_generic" }

type TickString struct {
	ReqID    int
	TickType int
	Value    string
}

func (TickString) messageName() string { return "tick_string" }

type TickReqParams struct {
	ReqID               int
	MinTick             string
	BBOExchange         string
	SnapshotPermissions int
}

func (TickReqParams) messageName() string { return "tick_req_params" }

type ReqMarketDataType struct {
	DataType int
}

func (ReqMarketDataType) messageName() string { return "req_market_data_type" }

type MktDepthExchangesRequest struct{}

func (MktDepthExchangesRequest) messageName() string { return "req_mkt_depth_exchanges" }

type MktDepthExchanges struct {
	Exchanges []DepthExchangeEntry
}

func (MktDepthExchanges) messageName() string { return "mkt_depth_exchanges" }

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

func (TickByTickRequest) messageName() string { return "req_tick_by_tick" }

type CancelTickByTick struct {
	ReqID int
}

func (CancelTickByTick) messageName() string { return "cancel_tick_by_tick" }

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

func (TickByTickData) messageName() string { return "tick_by_tick" }

// CalcImpliedVolatility (OUT 54 / cancel OUT 56) / CalcOptionPrice (OUT 55 / cancel OUT 57)

type CalcImpliedVolatilityRequest struct {
	ReqID       int
	Contract    Contract
	OptionPrice string
	UnderPrice  string
}

func (CalcImpliedVolatilityRequest) messageName() string { return "req_calc_implied_volatility" }

type CancelCalcImpliedVolatility struct {
	ReqID int
}

func (CancelCalcImpliedVolatility) messageName() string { return "cancel_calc_implied_volatility" }

type CalcOptionPriceRequest struct {
	ReqID      int
	Contract   Contract
	Volatility string
	UnderPrice string
}

func (CalcOptionPriceRequest) messageName() string { return "req_calc_option_price" }

type CancelCalcOptionPrice struct {
	ReqID int
}

func (CancelCalcOptionPrice) messageName() string { return "cancel_calc_option_price" }

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

func (TickOptionComputation) messageName() string { return "tick_option_computation" }

// Market depth (OUT 10, cancel OUT 11 / IN 12, 13)

type MarketDepthRequest struct {
	ReqID        int
	Contract     Contract
	NumRows      int
	IsSmartDepth bool
}

func (MarketDepthRequest) messageName() string { return "req_mkt_depth" }

type CancelMarketDepth struct {
	ReqID int
}

func (CancelMarketDepth) messageName() string { return "cancel_mkt_depth" }

type MarketDepthUpdate struct {
	ReqID     int
	Position  int
	Operation int // 0=insert, 1=update, 2=delete
	Side      int // 0=ask, 1=bid
	Price     string
	Size      string
}

func (MarketDepthUpdate) messageName() string { return "market_depth" }

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

func (MarketDepthL2Update) messageName() string { return "market_depth_l2" }

// [1, version, reqID, tickType, price, size, attrMask]
func decodeTickPrice(r *fieldReader) ([]Message, error) {
	r.Skip(1) // version
	reqID, _ := r.ReadInt()
	tickType, _ := r.ReadInt()
	price := r.ReadString()
	size := r.ReadString()
	attrMask, _ := r.ReadInt()
	return []Message{TickPrice{ReqID: reqID, TickType: tickType, Price: price, Size: size, AttrMask: attrMask}}, nil
}

// [2, version, reqID, tickType, size]
func decodeTickSize(r *fieldReader) ([]Message, error) {
	r.Skip(1) // version
	reqID, _ := r.ReadInt()
	tickType, _ := r.ReadInt()
	size := r.ReadString()
	return []Message{TickSize{ReqID: reqID, TickType: tickType, Size: size}}, nil
}

// [12, version, reqID, position, operation, side, price, size]
func decodeMarketDepth(r *fieldReader) ([]Message, error) {
	r.Skip(1) // version
	reqID, _ := r.ReadInt()
	position, _ := r.ReadInt()
	operation, _ := r.ReadInt()
	side, _ := r.ReadInt()
	price := r.ReadString()
	size := r.ReadString()
	return []Message{MarketDepthUpdate{ReqID: reqID, Position: position, Operation: operation, Side: side, Price: price, Size: size}}, nil
}

// [13, version, reqID, position, marketMaker, operation, side, price, size, isSmartDepth]
func decodeMarketDepthL2(r *fieldReader) ([]Message, error) {
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

// [21, reqID, tickType, tickAttrib, impliedVol, delta, optPrice, pvDividend, gamma, vega, theta, undPrice]
func decodeTickOptionComputation(r *fieldReader) ([]Message, error) {
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

// [45, version, reqID, tickType, value]
func decodeTickGeneric(r *fieldReader) ([]Message, error) {
	r.Skip(1) // version
	reqID, _ := r.ReadInt()
	tickType, _ := r.ReadInt()
	value := r.ReadString()
	return []Message{TickGeneric{ReqID: reqID, TickType: tickType, Value: value}}, nil
}

// [46, version, reqID, tickType, value]
func decodeTickString(r *fieldReader) ([]Message, error) {
	r.Skip(1) // version
	reqID, _ := r.ReadInt()
	tickType, _ := r.ReadInt()
	value := r.ReadString()
	return []Message{TickString{ReqID: reqID, TickType: tickType, Value: value}}, nil
}

// [81, reqID, minTick, bboExchange, snapshotPermissions] — no version
func decodeTickReqParams(r *fieldReader) ([]Message, error) {
	reqID, _ := r.ReadInt()
	minTick := r.ReadString()
	bboExchange := r.ReadString()
	snapshotPermissions, _ := r.ReadInt()
	return []Message{TickReqParams{ReqID: reqID, MinTick: minTick, BBOExchange: bboExchange, SnapshotPermissions: snapshotPermissions}}, nil
}

// [50, version, reqID, time, O, H, L, C, vol, wap, count]
func decodeRealTimeBars(r *fieldReader) ([]Message, error) {
	r.Skip(1)
	reqID, _ := r.ReadInt()
	return []Message{RealTimeBar{
		ReqID: reqID, Time: r.ReadString(),
		Open: r.ReadString(), High: r.ReadString(), Low: r.ReadString(),
		Close: r.ReadString(), Volume: r.ReadString(),
		WAP: r.ReadString(), Count: r.ReadString(),
	}}, nil
}

// [57, version, reqID]
func decodeTickSnapshotEnd(r *fieldReader) ([]Message, error) {
	r.Skip(1)
	reqID, _ := r.ReadInt()
	return []Message{TickSnapshotEnd{ReqID: reqID}}, nil
}

// [58, version, reqID, dataType]
func decodeMarketDataType(r *fieldReader) ([]Message, error) {
	r.Skip(1)
	reqID, _ := r.ReadInt()
	dataType, _ := r.ReadInt()
	return []Message{MarketDataType{ReqID: reqID, DataType: dataType}}, nil
}

func decodeMktDepthExchanges(r *fieldReader) ([]Message, error) {
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
func decodeTickByTick(r *fieldReader) ([]Message, error) {
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

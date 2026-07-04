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

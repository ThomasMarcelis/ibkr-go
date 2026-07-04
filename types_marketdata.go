package ibkr

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

type WhatToShow string

const (
	ShowTrades                  WhatToShow = "TRADES"
	ShowMidpoint                WhatToShow = "MIDPOINT"
	ShowBid                     WhatToShow = "BID"
	ShowAsk                     WhatToShow = "ASK"
	ShowBidAsk                  WhatToShow = "BID_ASK"
	ShowHistoricalVolatility    WhatToShow = "HISTORICAL_VOLATILITY"
	ShowOptionImpliedVolatility WhatToShow = "OPTION_IMPLIED_VOLATILITY"
	ShowAdjustedLast            WhatToShow = "ADJUSTED_LAST"
	ShowFeeRate                 WhatToShow = "FEE_RATE"
	ShowYieldBid                WhatToShow = "YIELD_BID"
	ShowYieldAsk                WhatToShow = "YIELD_ASK"
	ShowYieldBidAsk             WhatToShow = "YIELD_BID_ASK"
	ShowYieldLast               WhatToShow = "YIELD_LAST"
	ShowSchedule                WhatToShow = "SCHEDULE"
	ShowAggTrades               WhatToShow = "AGGTRADES"
)

type HistoricalDuration string

func Seconds(n int) HistoricalDuration { return historicalDuration(n, "S") }
func Minutes(n int) HistoricalDuration { return Seconds(n * 60) }
func Hours(n int) HistoricalDuration   { return Seconds(n * 60 * 60) }
func Days(n int) HistoricalDuration    { return historicalDuration(n, "D") }
func Weeks(n int) HistoricalDuration   { return historicalDuration(n, "W") }
func Months(n int) HistoricalDuration  { return historicalDuration(n, "M") }
func Years(n int) HistoricalDuration   { return historicalDuration(n, "Y") }

func historicalDuration(n int, unit string) HistoricalDuration {
	if n <= 0 {
		return ""
	}
	return HistoricalDuration(fmt.Sprintf("%d %s", n, unit))
}

type BarSize string

const (
	Bar1Sec   BarSize = "1 sec"
	Bar5Secs  BarSize = "5 secs"
	Bar10Secs BarSize = "10 secs"
	Bar15Secs BarSize = "15 secs"
	Bar30Secs BarSize = "30 secs"
	Bar1Min   BarSize = "1 min"
	Bar2Mins  BarSize = "2 mins"
	Bar3Mins  BarSize = "3 mins"
	Bar5Mins  BarSize = "5 mins"
	Bar10Mins BarSize = "10 mins"
	Bar15Mins BarSize = "15 mins"
	Bar20Mins BarSize = "20 mins"
	Bar30Mins BarSize = "30 mins"
	Bar1Hour  BarSize = "1 hour"
	Bar2Hours BarSize = "2 hours"
	Bar3Hours BarSize = "3 hours"
	Bar4Hours BarSize = "4 hours"
	Bar8Hours BarSize = "8 hours"
	Bar1Day   BarSize = "1 day"
	Bar1Week  BarSize = "1 week"
	Bar1Month BarSize = "1 month"
)

type HistoricalBarsRequest struct {
	Contract   Contract
	EndTime    time.Time
	Duration   HistoricalDuration
	BarSize    BarSize
	WhatToShow WhatToShow
	UseRTH     bool
}

// Bar is an OHLCV price bar for a single time interval, returned by
// historical and real-time bar requests.
type Bar struct {
	Time   time.Time
	Open   decimal.Decimal
	High   decimal.Decimal
	Low    decimal.Decimal
	Close  decimal.Decimal
	Volume decimal.Decimal
	WAP    decimal.Decimal
	Count  int
}

// HistoricalScheduleRequest asks the Gateway to return the session schedule
// that would cover a bar request for the given contract and duration. The
// request reuses REQ_HISTORICAL_DATA under the hood with whatToShow=SCHEDULE,
// so Duration and BarSize behave the same as for [History.Bars]. UseRTH is
// respected by the Gateway but the schedule response already encodes the
// regular-hours boundaries per session.
type HistoricalScheduleRequest struct {
	Contract Contract
	EndTime  time.Time
	Duration HistoricalDuration
	BarSize  BarSize
	UseRTH   bool
}

// HistoricalSchedule is the result of [History.Schedule]. StartDateTime,
// EndDateTime, and TimeZone describe the overall window returned by the
// Gateway; Sessions lists the contiguous trading windows inside it.
type HistoricalSchedule struct {
	StartDateTime string
	EndDateTime   string
	TimeZone      string
	Sessions      []HistoricalScheduleSession
}

// HistoricalScheduleSession describes a single trading session returned as
// part of a [HistoricalSchedule]. RefDate is the calendar date the session
// belongs to, which is useful when a session crosses midnight.
type HistoricalScheduleSession struct {
	StartDateTime string
	EndDateTime   string
	RefDate       string
}

type QuoteFields uint64

const (
	QuoteFieldBid QuoteFields = 1 << iota
	QuoteFieldAsk
	QuoteFieldLast
	QuoteFieldBidSize
	QuoteFieldAskSize
	QuoteFieldLastSize
	QuoteFieldOpen
	QuoteFieldHigh
	QuoteFieldLow
	QuoteFieldClose
	QuoteFieldMarketDataType
)

type MarketDataType int

const (
	MarketDataLive          MarketDataType = 1
	MarketDataFrozen        MarketDataType = 2
	MarketDataDelayed       MarketDataType = 3
	MarketDataDelayedFrozen MarketDataType = 4
)

func (t MarketDataType) String() string {
	switch t {
	case MarketDataLive:
		return "Live"
	case MarketDataFrozen:
		return "Frozen"
	case MarketDataDelayed:
		return "Delayed"
	case MarketDataDelayedFrozen:
		return "DelayedFrozen"
	default:
		return fmt.Sprintf("MarketDataType(%d)", t)
	}
}

// Quote is a snapshot of the current market quote fields for a contract.
// Available tracks which fields have been populated by the server; unpopulated
// fields remain at their zero value.
type Quote struct {
	Available      QuoteFields
	Bid            decimal.Decimal
	Ask            decimal.Decimal
	Last           decimal.Decimal
	BidSize        decimal.Decimal
	AskSize        decimal.Decimal
	LastSize       decimal.Decimal
	Open           decimal.Decimal
	High           decimal.Decimal
	Low            decimal.Decimal
	Close          decimal.Decimal
	MarketDataType MarketDataType
}

type GenericTick string

type QuoteRequest struct {
	Contract     Contract
	GenericTicks []GenericTick
}

type QuoteUpdate struct {
	Snapshot   Quote
	Changed    QuoteFields
	ReceivedAt time.Time
}

type RealTimeBarsRequest struct {
	Contract   Contract
	WhatToShow WhatToShow
	UseRTH     bool
}

type HeadTimestampRequest struct {
	Contract   Contract
	WhatToShow WhatToShow
	UseRTH     bool
}

type TickByTickType string

const (
	TickByTickLast     TickByTickType = "Last"
	TickByTickAllLast  TickByTickType = "AllLast"
	TickByTickBidAsk   TickByTickType = "BidAsk"
	TickByTickMidPoint TickByTickType = "MidPoint"
)

type TickByTickRequest struct {
	Contract      Contract
	TickType      TickByTickType
	NumberOfTicks int
	IgnoreSize    bool
}

type TickByTickData struct {
	Time              time.Time
	TickType          int
	Price             decimal.Decimal
	Size              decimal.Decimal
	Exchange          string
	SpecialConditions string
	BidPrice          decimal.Decimal
	AskPrice          decimal.Decimal
	BidSize           decimal.Decimal
	AskSize           decimal.Decimal
	MidPoint          decimal.Decimal
}

type SmartComponent struct {
	BitNumber      int
	ExchangeName   string
	ExchangeLetter string
}

type CalcImpliedVolatilityRequest struct {
	Contract    Contract
	OptionPrice decimal.Decimal
	UnderPrice  decimal.Decimal
}

type CalcOptionPriceRequest struct {
	Contract   Contract
	Volatility decimal.Decimal
	UnderPrice decimal.Decimal
}

type OptionComputation struct {
	ImpliedVol decimal.Decimal
	Delta      decimal.Decimal
	OptPrice   decimal.Decimal
	PvDividend decimal.Decimal
	Gamma      decimal.Decimal
	Vega       decimal.Decimal
	Theta      decimal.Decimal
	UndPrice   decimal.Decimal
}

type HistogramDataRequest struct {
	Contract Contract
	UseRTH   bool
	Period   string
}

type HistogramEntry struct {
	Price decimal.Decimal
	Size  decimal.Decimal
}

type HistoricalTicksRequest struct {
	Contract      Contract
	StartTime     time.Time
	EndTime       time.Time
	NumberOfTicks int
	WhatToShow    WhatToShow
	UseRTH        bool
	IgnoreSize    bool
}

type HistoricalTick struct {
	Time  time.Time
	Price decimal.Decimal
	Size  decimal.Decimal
}

type HistoricalTickBidAsk struct {
	TickAttrib int
	Time       time.Time
	BidPrice   decimal.Decimal
	AskPrice   decimal.Decimal
	BidSize    decimal.Decimal
	AskSize    decimal.Decimal
}

type HistoricalTickLast struct {
	TickAttrib        int
	Time              time.Time
	Price             decimal.Decimal
	Size              decimal.Decimal
	Exchange          string
	SpecialConditions string
}

// HistoricalTicksResult holds the result of a historical ticks request.
// Exactly one of the three slices is populated based on WhatToShow.
type HistoricalTicksResult struct {
	Ticks  []HistoricalTick       // populated for MIDPOINT
	BidAsk []HistoricalTickBidAsk // populated for BID_ASK
	Last   []HistoricalTickLast   // populated for TRADES
}

type MarketDepthRequest struct {
	Contract     Contract
	NumRows      int
	IsSmartDepth bool
}

type DepthOperation int

const (
	DepthInsert DepthOperation = 0
	DepthUpdate DepthOperation = 1
	DepthDelete DepthOperation = 2
)

func (o DepthOperation) String() string {
	switch o {
	case DepthInsert:
		return "Insert"
	case DepthUpdate:
		return "Update"
	case DepthDelete:
		return "Delete"
	default:
		return fmt.Sprintf("DepthOperation(%d)", o)
	}
}

type BookSide int

const (
	BookAsk BookSide = 0
	BookBid BookSide = 1
)

func (s BookSide) String() string {
	switch s {
	case BookAsk:
		return "Ask"
	case BookBid:
		return "Bid"
	default:
		return fmt.Sprintf("BookSide(%d)", s)
	}
}

type DepthRow struct {
	Position     int
	MarketMaker  string // only populated for L2
	Operation    DepthOperation
	Side         BookSide
	Price        decimal.Decimal
	Size         decimal.Decimal
	IsSmartDepth bool
}

type DepthExchange struct {
	Exchange        string
	SecType         SecType
	ListingExch     string
	ServiceDataType string
	AggGroup        int
}

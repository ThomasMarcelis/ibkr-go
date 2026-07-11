package ibkr

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// WhatToShow selects which data series a historical or real-time bar request
// returns (trades, midpoint, bid/ask, and various derived series).
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

// HistoricalDuration is the look-back window of a historical data request,
// counted back from the request's end time. Build one with the helper
// constructors ([Seconds], [Days], and so on); a non-positive count yields an
// empty, invalid duration.
type HistoricalDuration string

// Seconds returns a [HistoricalDuration] spanning n seconds.
func Seconds(n int) HistoricalDuration { return historicalDuration(n, "S") }

// Minutes returns a [HistoricalDuration] spanning n minutes (expressed in seconds).
func Minutes(n int) HistoricalDuration { return Seconds(n * 60) }

// Hours returns a [HistoricalDuration] spanning n hours (expressed in seconds).
func Hours(n int) HistoricalDuration { return Seconds(n * 60 * 60) }

// Days returns a [HistoricalDuration] spanning n days.
func Days(n int) HistoricalDuration { return historicalDuration(n, "D") }

// Weeks returns a [HistoricalDuration] spanning n weeks.
func Weeks(n int) HistoricalDuration { return historicalDuration(n, "W") }

// Months returns a [HistoricalDuration] spanning n months.
func Months(n int) HistoricalDuration { return historicalDuration(n, "M") }

// Years returns a [HistoricalDuration] spanning n years.
func Years(n int) HistoricalDuration { return historicalDuration(n, "Y") }

func historicalDuration(n int, unit string) HistoricalDuration {
	if n <= 0 {
		return ""
	}
	return HistoricalDuration(fmt.Sprintf("%d %s", n, unit))
}

// BarSize is the aggregation interval of a historical or real-time bar.
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

// HistoricalBarsRequest describes a historical bar query for
// [HistoryClient.Bars] and [HistoryClient.SubscribeBars].
type HistoricalBarsRequest struct {
	Contract   Contract
	EndTime    time.Time          // window end; zero means "now"
	Duration   HistoricalDuration // look-back window from EndTime
	BarSize    BarSize            // bar aggregation interval
	WhatToShow WhatToShow         // which data series to return
	UseRTH     bool               // restrict to regular trading hours
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

// QuoteFields is a bitmask of the quote fields the server has populated. It is
// carried in [Quote.Available] and [QuoteUpdate.Changed]; test membership with
// a bitwise AND.
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
	QuoteFieldVolume
)

// MarketDataType selects live, frozen, delayed, or delayed-frozen market data.
// Set it with [MarketDataClient.SetType]; it also appears in [Quote.MarketDataType]
// to report what the server actually delivered.
type MarketDataType int

const (
	MarketDataLive          MarketDataType = 1 // real-time streaming (requires subscription)
	MarketDataFrozen        MarketDataType = 2 // last recorded values when the market is closed
	MarketDataDelayed       MarketDataType = 3 // delayed data, no subscription required
	MarketDataDelayedFrozen MarketDataType = 4 // delayed last recorded values
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
	Volume         decimal.Decimal
	Open           decimal.Decimal
	High           decimal.Decimal
	Low            decimal.Decimal
	Close          decimal.Decimal
	MarketDataType MarketDataType
}

// GenericTick is an IBKR generic tick type ID requested alongside a quote to
// pull in extra fields (for example "233" for RTVolume). Encoded on the wire as
// a comma-separated list.
type GenericTick string

// QuoteRequest describes a market-data quote request for
// [MarketDataClient.Quote] and [MarketDataClient.SubscribeQuotes].
type QuoteRequest struct {
	Contract     Contract
	GenericTicks []GenericTick // extra generic tick types; unsupported for one-shot snapshots
}

// QuoteUpdateKind identifies which payload a [QuoteUpdate] carries.
type QuoteUpdateKind uint8

const (
	// QuoteUpdateFields reports a snapshot-field update with no separate tick
	// payload. Market-data-type callbacks use this kind.
	QuoteUpdateFields QuoteUpdateKind = iota
	QuoteUpdateGenericTick
	QuoteUpdateStringTick
	QuoteUpdateParameters
	QuoteUpdateOptionComputation
	QuoteUpdatePriceTick
	QuoteUpdateSizeTick
	QuoteUpdateNewsTick
)

func (k QuoteUpdateKind) String() string {
	switch k {
	case QuoteUpdateFields:
		return "Fields"
	case QuoteUpdateGenericTick:
		return "GenericTick"
	case QuoteUpdateStringTick:
		return "StringTick"
	case QuoteUpdateParameters:
		return "Parameters"
	case QuoteUpdateOptionComputation:
		return "OptionComputation"
	case QuoteUpdatePriceTick:
		return "PriceTick"
	case QuoteUpdateSizeTick:
		return "SizeTick"
	case QuoteUpdateNewsTick:
		return "NewsTick"
	default:
		return fmt.Sprintf("QuoteUpdateKind(%d)", k)
	}
}

// QuotePriceAttributes is the exact attribute bitmask attached to a classic
// price tick. The methods expose the three bits defined by IBKR without hiding
// any unknown future bits in the underlying mask.
type QuotePriceAttributes int

// CanAutoExecute reports whether the tick can be used for automatic order execution.
func (a QuotePriceAttributes) CanAutoExecute() bool { return a&1 != 0 }

// PastLimit reports whether the tick is outside the price limits.
func (a QuotePriceAttributes) PastLimit() bool { return a&2 != 0 }

// PreOpen reports whether the tick is from the pre-open period.
func (a QuotePriceAttributes) PreOpen() bool { return a&4 != 0 }

// QuotePriceTick preserves one classic IBKR tickPrice callback. Size is the
// optional companion size carried in the same wire message; nil means the wire
// field was unset. TickType is IBKR's numeric tick-type ID.
type QuotePriceTick struct {
	TickType int
	Price    decimal.Decimal
	Size     *decimal.Decimal
	AttrMask QuotePriceAttributes
}

// QuoteSizeTick preserves one IBKR tickSize callback. Size is nil when a
// protobuf callback omits the value and the official decoder reports
// UNSET_DECIMAL. TickType is IBKR's numeric tick-type ID.
type QuoteSizeTick struct {
	TickType int
	Size     *decimal.Decimal
}

// QuoteGenericTick is one numeric IBKR tick that has no normalized [Quote]
// field. TickType is IBKR's numeric tick-type ID.
type QuoteGenericTick struct {
	TickType int
	Value    decimal.Decimal
}

// QuoteStringTick is one textual IBKR tick that has no normalized [Quote]
// field. TickType is IBKR's numeric tick-type ID.
type QuoteStringTick struct {
	TickType int
	Value    string
}

// QuoteNewsTick is one contract-specific news headline. Time is the provider
// timestamp carried by IBKR; ExtraData preserves the provider metadata string
// verbatim.
type QuoteNewsTick struct {
	Time         time.Time
	ProviderCode NewsProviderCode
	ArticleID    string
	Headline     string
	ExtraData    string
}

// QuoteParameters describes the market-data rules attached to a quote
// request. BBOExchange is IBKR's exchange bit field, not an exchange name.
type QuoteParameters struct {
	MinTick             *decimal.Decimal // nil when IBKR omits the value
	BBOExchange         string
	SnapshotPermissions *int             // nil when IBKR omits the bit mask
	LastPricePrecision  *decimal.Decimal // nil when IBKR omits the value
	LastSizePrecision   *decimal.Decimal // nil when IBKR omits the value
}

// QuoteOptionComputation is one option-price or greeks tick. TickType selects
// bid, ask, last, model, or a delayed equivalent; TickAttrib is the IBKR option
// computation attribute bitmask.
type QuoteOptionComputation struct {
	TickType    int
	TickAttrib  int
	Computation OptionComputation
}

// QuoteUpdate is one event from a quote subscription. Kind selects exactly one
// payload. Snapshot is always the full accumulated [Quote]; ancillary ticks do
// not mutate it. Changed reports the normalized fields touched by price, size,
// or market-data-type callbacks and is zero when a tick has no [Quote] mapping.
// Every kind except [QuoteUpdateFields] sets its corresponding payload pointer;
// unrelated payload pointers are nil.
type QuoteUpdate struct {
	Kind              QuoteUpdateKind
	Snapshot          Quote       // cumulative quote state after this update
	Changed           QuoteFields // fields changed by this update
	PriceTick         *QuotePriceTick
	SizeTick          *QuoteSizeTick
	GenericTick       *QuoteGenericTick
	StringTick        *QuoteStringTick
	NewsTick          *QuoteNewsTick
	Parameters        *QuoteParameters
	OptionComputation *QuoteOptionComputation
	ReceivedAt        time.Time // client receive time
}

// RealTimeBarsRequest describes a 5-second real-time bar subscription for
// [MarketDataClient.SubscribeRealTimeBars].
type RealTimeBarsRequest struct {
	Contract   Contract
	WhatToShow WhatToShow
	UseRTH     bool
}

// HeadTimestampRequest asks for the earliest available data timestamp of a
// contract via [HistoryClient.HeadTimestamp].
type HeadTimestampRequest struct {
	Contract   Contract
	WhatToShow WhatToShow
	UseRTH     bool
}

// TickByTickType selects which tick-by-tick stream to subscribe to.
type TickByTickType string

const (
	TickByTickLast     TickByTickType = "Last"     // last trade, exchange-reported
	TickByTickAllLast  TickByTickType = "AllLast"  // last trade including non-reportable prints
	TickByTickBidAsk   TickByTickType = "BidAsk"   // best bid and ask
	TickByTickMidPoint TickByTickType = "MidPoint" // midpoint of the spread
)

// TickByTickRequest describes a tick-by-tick subscription for
// [MarketDataClient.SubscribeTickByTick].
type TickByTickRequest struct {
	Contract      Contract
	TickType      TickByTickType
	NumberOfTicks int  // historical ticks to prepend; 0 = live only
	IgnoreSize    bool // ignore size changes for BidAsk streams
}

// TickByTickData is one tick from a tick-by-tick stream. Which fields are
// populated depends on the subscribed [TickByTickType]: Last/AllLast fill Price
// and Size; BidAsk fills the Bid/Ask fields; MidPoint fills MidPoint.
type TickByTickData struct {
	Time              time.Time
	TickType          int // numeric tick type reported by the Gateway
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

// SmartComponent maps a SMART-routing bit to the exchange it represents, as
// returned by [ContractsClient.SmartComponents].
type SmartComponent struct {
	BitNumber      int
	ExchangeName   string
	ExchangeLetter string
}

// CalcImpliedVolatilityRequest asks the Gateway to imply an option's volatility
// from a given option price and underlying price, via
// [OptionsClient.ImpliedVolatility].
type CalcImpliedVolatilityRequest struct {
	Contract    Contract
	OptionPrice decimal.Decimal
	UnderPrice  decimal.Decimal
}

// CalcOptionPriceRequest asks the Gateway to price an option from a given
// volatility and underlying price, via [OptionsClient.Price].
type CalcOptionPriceRequest struct {
	Contract   Contract
	Volatility decimal.Decimal
	UnderPrice decimal.Decimal
}

// OptionComputationFields is a bitmask of values IBKR actually computed. A
// missing value remains zero in [OptionComputation] and is absent from
// OptionComputation.Available.
type OptionComputationFields uint16

const (
	OptionComputationImpliedVol OptionComputationFields = 1 << iota
	OptionComputationDelta
	OptionComputationPrice
	OptionComputationPvDividend
	OptionComputationGamma
	OptionComputationVega
	OptionComputationTheta
	OptionComputationUnderlyingPrice
)

// OptionComputation is an option pricing/greeks result returned by option
// calculation requests and quote subscriptions. Available distinguishes a
// computed zero from IBKR's field-specific "not computed" wire sentinels.
type OptionComputation struct {
	Available  OptionComputationFields
	ImpliedVol decimal.Decimal
	Delta      decimal.Decimal
	OptPrice   decimal.Decimal
	PvDividend decimal.Decimal
	Gamma      decimal.Decimal
	Vega       decimal.Decimal
	Theta      decimal.Decimal
	UndPrice   decimal.Decimal
}

// HistogramDataRequest asks for a price histogram over a period via
// [HistoryClient.Histogram].
type HistogramDataRequest struct {
	Contract Contract
	UseRTH   bool
	Period   string // aggregation period, e.g. "3 days"
}

// HistogramEntry is one price bucket of a histogram: the traded Size at Price.
type HistogramEntry struct {
	Price decimal.Decimal
	Size  decimal.Decimal
}

// HistoricalTicksRequest describes a historical tick query for
// [HistoryClient.Ticks]. Provide StartTime or EndTime (not both) together with
// NumberOfTicks.
type HistoricalTicksRequest struct {
	Contract      Contract
	StartTime     time.Time
	EndTime       time.Time
	NumberOfTicks int
	WhatToShow    WhatToShow // TRADES, BID_ASK, or MIDPOINT; selects the result slice
	UseRTH        bool
	IgnoreSize    bool
}

// HistoricalTick is a single midpoint historical tick.
type HistoricalTick struct {
	Time  time.Time
	Price decimal.Decimal
	Size  decimal.Decimal
}

// HistoricalBidAskAttributes is the exact attribute bitmask attached to a
// historical bid/ask tick. Unknown bits remain preserved in the value.
type HistoricalBidAskAttributes int

func (a HistoricalBidAskAttributes) BidPastLow() bool  { return a&1 != 0 }
func (a HistoricalBidAskAttributes) AskPastHigh() bool { return a&2 != 0 }

// HistoricalLastAttributes is the exact attribute bitmask attached to a
// historical trade tick. Unknown bits remain preserved in the value.
type HistoricalLastAttributes int

func (a HistoricalLastAttributes) PastLimit() bool  { return a&1 != 0 }
func (a HistoricalLastAttributes) Unreported() bool { return a&2 != 0 }

// HistoricalTickBidAsk is a single bid/ask historical tick.
type HistoricalTickBidAsk struct {
	TickAttrib HistoricalBidAskAttributes
	Time       time.Time
	BidPrice   decimal.Decimal
	AskPrice   decimal.Decimal
	BidSize    decimal.Decimal
	AskSize    decimal.Decimal
}

// HistoricalTickLast is a single trade (last) historical tick.
type HistoricalTickLast struct {
	TickAttrib        HistoricalLastAttributes
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

// MarketDepthRequest describes a market depth (Level 2 order book)
// subscription for [MarketDataClient.SubscribeDepth].
//
// Depth is a high-rate, stateful stream. A consumer that cannot keep up fails
// the subscription with [ErrSlowConsumer] because losing one delta corrupts
// the local book. Raise the queue with [WithQueueSize] when more burst capacity
// is needed.
type MarketDepthRequest struct {
	Contract     Contract
	NumRows      int  // number of book levels per side to stream
	IsSmartDepth bool // aggregated SMART depth across exchanges rather than a single venue
}

// DepthOperation is the mutation a [DepthRow] applies to the local order book.
type DepthOperation int

const (
	DepthInsert DepthOperation = 0 // insert a new level at Position
	DepthUpdate DepthOperation = 1 // update the level at Position
	DepthDelete DepthOperation = 2 // remove the level at Position
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

// BookSide identifies which side of the order book a [DepthRow] belongs to.
type BookSide int

const (
	BookAsk BookSide = 0 // ask (offer) side
	BookBid BookSide = 1 // bid side
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

// DepthRow is one order-book mutation from a market depth subscription: apply
// Operation to Side at Position with the given Price and Size. Size is nil
// when an exact protobuf callback omits it and IBKR reports UNSET_DECIMAL.
type DepthRow struct {
	Position     int
	MarketMaker  string // only populated for L2
	Operation    DepthOperation
	Side         BookSide
	Price        decimal.Decimal
	Size         *decimal.Decimal
	IsSmartDepth bool
}

// DepthExchange is one exchange that offers market depth, returned by
// [ContractsClient.DepthExchanges].
type DepthExchange struct {
	Exchange        string
	SecType         SecType
	ListingExch     string
	ServiceDataType string // depth service type, e.g. "Deep" or "Deep2"
	AggGroup        int
}

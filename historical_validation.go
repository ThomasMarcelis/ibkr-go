package ibkr

import (
	"strconv"
	"strings"
)

var validHistoricalBarSizes = map[BarSize]struct{}{
	Bar1Sec:   {},
	Bar5Secs:  {},
	Bar10Secs: {},
	Bar15Secs: {},
	Bar30Secs: {},
	Bar1Min:   {},
	Bar2Mins:  {},
	Bar3Mins:  {},
	Bar5Mins:  {},
	Bar10Mins: {},
	Bar15Mins: {},
	Bar20Mins: {},
	Bar30Mins: {},
	Bar1Hour:  {},
	Bar2Hours: {},
	Bar3Hours: {},
	Bar4Hours: {},
	Bar8Hours: {},
	Bar1Day:   {},
	Bar1Week:  {},
	Bar1Month: {},
}

var validWhatToShowValues = map[WhatToShow]struct{}{
	ShowTrades:                  {},
	ShowMidpoint:                {},
	ShowBid:                     {},
	ShowAsk:                     {},
	ShowBidAsk:                  {},
	ShowHistoricalVolatility:    {},
	ShowOptionImpliedVolatility: {},
	ShowAdjustedLast:            {},
	ShowFeeRate:                 {},
	ShowYieldBid:                {},
	ShowYieldAsk:                {},
	ShowYieldBidAsk:             {},
	ShowYieldLast:               {},
	ShowSchedule:                {},
	ShowAggTrades:               {},
}

func (w WhatToShow) Valid() bool {
	_, ok := validWhatToShowValues[w]
	return ok
}

func (d HistoricalDuration) Valid() bool {
	parts := strings.Fields(string(d))
	if len(parts) != 2 {
		return false
	}
	n, err := strconv.Atoi(parts[0])
	if err != nil || n <= 0 {
		return false
	}
	switch parts[1] {
	case "S", "D", "W", "M", "Y":
		return true
	default:
		return false
	}
}

func (b BarSize) Valid() bool {
	_, ok := validHistoricalBarSizes[b]
	return ok
}

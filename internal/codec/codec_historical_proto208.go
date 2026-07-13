package codec

import (
	"strconv"

	"google.golang.org/protobuf/encoding/protowire"
)

func historicalRequestProto(reqID int, contract Contract, label string) ([]byte, error) {
	body, err := appendProtoInt(nil, 1, reqID, label+" request id")
	if err != nil {
		return nil, err
	}
	encodedContract, err := encodeSharedContractProto(contract, nil, true)
	if err != nil {
		return nil, err
	}
	return appendProtoMessage(body, 2, encodedContract), nil
}

func (m HistoricalBarsRequest) encodeProto(sv int) ([]byte, error) {
	body, err := historicalRequestProto(m.ReqID, m.Contract, "historical data")
	if err != nil {
		return nil, err
	}
	if m.EndDateTime != "" {
		body = appendProtoString(body, 3, m.EndDateTime)
	}
	if m.BarSize != "" {
		body = appendProtoString(body, 4, m.BarSize)
	}
	if m.Duration != "" {
		body = appendProtoString(body, 5, m.Duration)
	}
	if m.UseRTH {
		body = appendProtoVarint(body, 6, 1)
	}
	if m.WhatToShow != "" {
		body = appendProtoString(body, 7, m.WhatToShow)
	}
	body = appendProtoVarint(body, 8, 1)
	if m.KeepUpToDate {
		body = appendProtoVarint(body, 9, 1)
	}
	return canonicalProtoFields(body), nil
}

func (m CancelHistoricalData) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.ReqID, "cancel historical data request id")
}

func (m RealTimeBarsRequest) encodeProto(sv int) ([]byte, error) {
	body, err := historicalRequestProto(m.ReqID, m.Contract, "real-time bars")
	if err != nil {
		return nil, err
	}
	body = appendProtoVarint(body, 3, 5)
	if m.WhatToShow != "" {
		body = appendProtoString(body, 4, m.WhatToShow)
	}
	if m.UseRTH {
		body = appendProtoVarint(body, 5, 1)
	}
	return canonicalProtoFields(body), nil
}

func (m CancelRealTimeBars) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.ReqID, "cancel real-time bars request id")
}

func (m HeadTimestampRequest) encodeProto(sv int) ([]byte, error) {
	body, err := historicalRequestProto(m.ReqID, m.Contract, "head timestamp")
	if err != nil {
		return nil, err
	}
	if m.UseRTH {
		body = appendProtoVarint(body, 3, 1)
	}
	if m.WhatToShow != "" {
		body = appendProtoString(body, 4, m.WhatToShow)
	}
	body = appendProtoVarint(body, 5, 1)
	return canonicalProtoFields(body), nil
}

func (m CancelHeadTimestamp) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.ReqID, "cancel head timestamp request id")
}

func (m HistogramDataRequest) encodeProto(sv int) ([]byte, error) {
	body, err := historicalRequestProto(m.ReqID, m.Contract, "histogram data")
	if err != nil {
		return nil, err
	}
	if m.UseRTH {
		body = appendProtoVarint(body, 3, 1)
	}
	if m.Period != "" {
		body = appendProtoString(body, 4, m.Period)
	}
	return canonicalProtoFields(body), nil
}

func (m CancelHistogramData) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.ReqID, "cancel histogram data request id")
}

func (m HistoricalTicksRequest) encodeProto(sv int) ([]byte, error) {
	body, err := historicalRequestProto(m.ReqID, m.Contract, "historical ticks")
	if err != nil {
		return nil, err
	}
	for _, field := range []struct {
		number protowire.Number
		value  string
	}{{3, m.StartDateTime}, {4, m.EndDateTime}, {6, m.WhatToShow}} {
		if field.value != "" {
			body = appendProtoString(body, field.number, field.value)
		}
	}
	body, err = appendProtoInt(body, 5, m.NumberOfTicks, "historical tick count")
	if err != nil {
		return nil, err
	}
	if m.UseRTH {
		body = appendProtoVarint(body, 7, 1)
	}
	if m.IgnoreSize {
		body = appendProtoVarint(body, 8, 1)
	}
	return canonicalProtoFields(body), nil
}

func (m TickByTickRequest) encodeProto(sv int) ([]byte, error) {
	body, err := historicalRequestProto(m.ReqID, m.Contract, "tick-by-tick")
	if err != nil {
		return nil, err
	}
	if m.TickType != "" {
		body = appendProtoString(body, 3, m.TickType)
	}
	body, err = appendProtoInt(body, 4, m.NumberOfTicks, "tick-by-tick count")
	if err != nil {
		return nil, err
	}
	if m.IgnoreSize {
		body = appendProtoVarint(body, 5, 1)
	}
	return canonicalProtoFields(body), nil
}

func (m CancelTickByTick) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.ReqID, "cancel tick-by-tick request id")
}

func decodeHistoricalDataProto(body []byte, sv int) ([]Message, error) {
	reqID := -1
	var bars []HistoricalBar
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			messages := make([]Message, len(bars))
			for i := range bars {
				bars[i].ReqID = reqID
				messages[i] = bars[i]
			}
			return messages, nil
		}
		switch number {
		case 1:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("historical data", number, err)
			}
			reqID = decodeProtoInt32(value)
		case 2:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("historical data", number, err)
			}
			bar, err := decodeHistoricalDataBarProto(value)
			if err != nil {
				return nil, protoFieldError("historical data bar", number, err)
			}
			bars = append(bars, bar)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("historical data", number, err)
			}
		}
	}
}

func decodeHistoricalDataUpdateProto(body []byte, sv int) ([]Message, error) {
	reqID := -1
	var bar *HistoricalBar
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			if bar == nil {
				return nil, nil
			}
			return []Message{HistoricalDataUpdate{ReqID: reqID, BarCount: mustAtoi(bar.Count), Time: bar.Time, Open: bar.Open, High: bar.High, Low: bar.Low, Close: bar.Close, Volume: bar.Volume, WAP: bar.WAP}}, nil
		}
		switch number {
		case 1:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("historical data update", number, err)
			}
			reqID = decodeProtoInt32(value)
		case 2:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("historical data update", number, err)
			}
			decoded, err := decodeHistoricalDataBarProto(value)
			if err != nil {
				return nil, protoFieldError("historical data update bar", number, err)
			}
			bar = &decoded
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("historical data update", number, err)
			}
		}
	}
}

func decodeHistoricalDataBarProto(body []byte) (HistoricalBar, error) {
	bar := HistoricalBar{Open: "0", High: "0", Low: "0", Close: "0", Count: "0"}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return HistoricalBar{}, err
		}
		if !ok {
			return bar, nil
		}
		switch number {
		case 1, 6, 7:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return HistoricalBar{}, protoFieldError("historical data bar", number, err)
			}
			switch number {
			case 1:
				bar.Time = string(value)
			case 6:
				bar.Volume = string(value)
			case 7:
				bar.WAP = string(value)
			}
		case 2, 3, 4, 5:
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return HistoricalBar{}, protoFieldError("historical data bar", number, err)
			}
			formatted := formatProtoDouble(value)
			switch number {
			case 2:
				bar.Open = formatted
			case 3:
				bar.High = formatted
			case 4:
				bar.Low = formatted
			case 5:
				bar.Close = formatted
			}
		case 8:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return HistoricalBar{}, protoFieldError("historical data bar", number, err)
			}
			bar.Count = itoa(decodeProtoInt32(value))
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return HistoricalBar{}, protoFieldError("historical data bar", number, err)
			}
		}
	}
}

func decodeHistoricalDataEndProto(body []byte, sv int) ([]Message, error) {
	m := HistoricalBarsEnd{ReqID: -1}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		switch number {
		case 1:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("historical data end", number, err)
			}
			m.ReqID = decodeProtoInt32(value)
		case 2, 3:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("historical data end", number, err)
			}
			if number == 2 {
				m.StartDate = string(value)
			} else {
				m.EndDate = string(value)
			}
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("historical data end", number, err)
			}
		}
	}
}

func decodeRealTimeBarsProto(body []byte, sv int) ([]Message, error) {
	m := RealTimeBar{ReqID: -1, Open: "0", High: "0", Low: "0", Close: "0", Count: "0"}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		switch number {
		case 1, 2, 9:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("real-time bar", number, err)
			}
			switch number {
			case 1:
				m.ReqID = decodeProtoInt32(value)
			case 2:
				m.Time = i64toa(decodeProtoInt64(value))
			case 9:
				m.Count = itoa(decodeProtoInt32(value))
			}
		case 3, 4, 5, 6:
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return nil, protoFieldError("real-time bar", number, err)
			}
			formatted := formatProtoDouble(value)
			switch number {
			case 3:
				m.Open = formatted
			case 4:
				m.High = formatted
			case 5:
				m.Low = formatted
			case 6:
				m.Close = formatted
			}
		case 7, 8:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("real-time bar", number, err)
			}
			if number == 7 {
				m.Volume = string(value)
			} else {
				m.WAP = string(value)
			}
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("real-time bar", number, err)
			}
		}
	}
}

func decodeHeadTimestampProto(body []byte, sv int) ([]Message, error) {
	m := HeadTimestamp{ReqID: -1}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		if number == 1 {
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("head timestamp", number, err)
			}
			m.ReqID = decodeProtoInt32(value)
		} else if number == 2 {
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("head timestamp", number, err)
			}
			m.Timestamp = string(value)
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return nil, protoFieldError("head timestamp", number, err)
		}
	}
}

func decodeHistogramDataProto(body []byte, sv int) ([]Message, error) {
	m := HistogramDataResponse{ReqID: -1}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		if number == 1 {
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("histogram data", number, err)
			}
			m.ReqID = decodeProtoInt32(value)
		} else if number == 2 {
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("histogram data", number, err)
			}
			entry, err := decodeHistogramEntryProto(value)
			if err != nil {
				return nil, protoFieldError("histogram entry", number, err)
			}
			m.Entries = append(m.Entries, entry)
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return nil, protoFieldError("histogram data", number, err)
		}
	}
}

func decodeHistogramEntryProto(body []byte) (HistogramDataEntry, error) {
	m := HistogramDataEntry{Price: "0"}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return HistogramDataEntry{}, err
		}
		if !ok {
			return m, nil
		}
		if number == 1 {
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return HistogramDataEntry{}, protoFieldError("histogram entry", number, err)
			}
			m.Price = formatProtoDouble(value)
		} else if number == 2 {
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return HistogramDataEntry{}, protoFieldError("histogram entry", number, err)
			}
			m.Size = string(value)
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return HistogramDataEntry{}, protoFieldError("histogram entry", number, err)
		}
	}
}

func decodeHistoricalTicksProto(body []byte, sv int) ([]Message, error) {
	m := HistoricalTicksResponse{ReqID: -1}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		switch number {
		case 1, 3:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("historical ticks", number, err)
			}
			if number == 1 {
				m.ReqID = decodeProtoInt32(value)
			} else {
				m.Done = value != 0
			}
		case 2:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("historical ticks", number, err)
			}
			tick, err := decodeHistoricalTickProto(value)
			if err != nil {
				return nil, protoFieldError("historical tick", number, err)
			}
			m.Ticks = append(m.Ticks, tick)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("historical ticks", number, err)
			}
		}
	}
}

func decodeHistoricalTicksBidAskProto(body []byte, sv int) ([]Message, error) {
	m := HistoricalTicksBidAskResponse{ReqID: -1}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		switch number {
		case 1, 3:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("historical bid/ask ticks", number, err)
			}
			if number == 1 {
				m.ReqID = decodeProtoInt32(value)
			} else {
				m.Done = value != 0
			}
		case 2:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("historical bid/ask ticks", number, err)
			}
			tick, err := decodeHistoricalBidAskTickProto(value)
			if err != nil {
				return nil, protoFieldError("historical bid/ask tick", number, err)
			}
			m.Ticks = append(m.Ticks, tick)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("historical bid/ask ticks", number, err)
			}
		}
	}
}

func decodeHistoricalTicksLastProto(body []byte, sv int) ([]Message, error) {
	m := HistoricalTicksLastResponse{ReqID: -1}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		switch number {
		case 1, 3:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("historical trade ticks", number, err)
			}
			if number == 1 {
				m.ReqID = decodeProtoInt32(value)
			} else {
				m.Done = value != 0
			}
		case 2:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("historical trade ticks", number, err)
			}
			tick, err := decodeHistoricalLastTickProto(value)
			if err != nil {
				return nil, protoFieldError("historical trade tick", number, err)
			}
			m.Ticks = append(m.Ticks, tick)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("historical trade ticks", number, err)
			}
		}
	}
}

func decodeHistoricalTickProto(body []byte) (HistoricalTickEntry, error) {
	m := HistoricalTickEntry{Price: "0"}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return HistoricalTickEntry{}, err
		}
		if !ok {
			return m, nil
		}
		switch number {
		case 1:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return HistoricalTickEntry{}, protoFieldError("historical tick", number, err)
			}
			m.Time = i64toa(decodeProtoInt64(value))
		case 2:
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return HistoricalTickEntry{}, protoFieldError("historical tick", number, err)
			}
			m.Price = formatProtoDouble(value)
		case 3:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return HistoricalTickEntry{}, protoFieldError("historical tick", number, err)
			}
			m.Size = string(value)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return HistoricalTickEntry{}, protoFieldError("historical tick", number, err)
			}
		}
	}
}

func decodeHistoricalBidAskTickProto(body []byte) (HistoricalTickBidAskEntry, error) {
	m := HistoricalTickBidAskEntry{BidPrice: "0", AskPrice: "0"}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return HistoricalTickBidAskEntry{}, err
		}
		if !ok {
			return m, nil
		}
		switch number {
		case 1:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return HistoricalTickBidAskEntry{}, protoFieldError("historical bid/ask tick", number, err)
			}
			m.Time = i64toa(decodeProtoInt64(value))
		case 2:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return HistoricalTickBidAskEntry{}, protoFieldError("historical bid/ask tick", number, err)
			}
			m.TickAttrib, err = decodeBidAskAttributesProto(value)
			if err != nil {
				return HistoricalTickBidAskEntry{}, err
			}
		case 3, 4:
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return HistoricalTickBidAskEntry{}, protoFieldError("historical bid/ask tick", number, err)
			}
			if number == 3 {
				m.BidPrice = formatProtoDouble(value)
			} else {
				m.AskPrice = formatProtoDouble(value)
			}
		case 5, 6:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return HistoricalTickBidAskEntry{}, protoFieldError("historical bid/ask tick", number, err)
			}
			if number == 5 {
				m.BidSize = string(value)
			} else {
				m.AskSize = string(value)
			}
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return HistoricalTickBidAskEntry{}, protoFieldError("historical bid/ask tick", number, err)
			}
		}
	}
}

func decodeHistoricalLastTickProto(body []byte) (HistoricalTickLastEntry, error) {
	m := HistoricalTickLastEntry{Price: "0"}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return HistoricalTickLastEntry{}, err
		}
		if !ok {
			return m, nil
		}
		switch number {
		case 1:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return HistoricalTickLastEntry{}, protoFieldError("historical trade tick", number, err)
			}
			m.Time = i64toa(decodeProtoInt64(value))
		case 2:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return HistoricalTickLastEntry{}, protoFieldError("historical trade tick", number, err)
			}
			m.TickAttrib, err = decodeLastAttributesProto(value)
			if err != nil {
				return HistoricalTickLastEntry{}, err
			}
		case 3:
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return HistoricalTickLastEntry{}, protoFieldError("historical trade tick", number, err)
			}
			m.Price = formatProtoDouble(value)
		case 4, 5, 6:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return HistoricalTickLastEntry{}, protoFieldError("historical trade tick", number, err)
			}
			switch number {
			case 4:
				m.Size = string(value)
			case 5:
				m.Exchange = string(value)
			case 6:
				m.SpecialConditions = string(value)
			}
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return HistoricalTickLastEntry{}, protoFieldError("historical trade tick", number, err)
			}
		}
	}
}

func decodeBidAskAttributesProto(body []byte) (int, error) {
	mask := 0
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return 0, err
		}
		if !ok {
			return mask, nil
		}
		if number == 1 || number == 2 {
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return 0, protoFieldError("bid/ask attributes", number, err)
			}
			if value != 0 {
				mask |= 1 << (number - 1)
			}
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return 0, protoFieldError("bid/ask attributes", number, err)
		}
	}
}

func decodeLastAttributesProto(body []byte) (int, error) {
	return decodeBidAskAttributesProto(body)
}

func decodeTickByTickProto(body []byte, sv int) ([]Message, error) {
	reqID, tickType := -1, 0
	var last *HistoricalTickLastEntry
	var bidAsk *HistoricalTickBidAskEntry
	var midpoint *HistoricalTickEntry
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			m := TickByTickData{ReqID: reqID, TickType: tickType}
			switch tickType {
			case 1, 2:
				if last == nil {
					return nil, nil
				}
				m.Time, m.Price, m.Size = last.Time, last.Price, last.Size
				m.TickAttribLast, m.Exchange, m.SpecialConditions = last.TickAttrib, last.Exchange, last.SpecialConditions
			case 3:
				if bidAsk == nil {
					return nil, nil
				}
				m.Time, m.BidPrice, m.AskPrice = bidAsk.Time, bidAsk.BidPrice, bidAsk.AskPrice
				m.BidSize, m.AskSize, m.TickAttribBidAsk = bidAsk.BidSize, bidAsk.AskSize, bidAsk.TickAttrib
			case 4:
				if midpoint == nil {
					return nil, nil
				}
				m.Time, m.MidPoint = midpoint.Time, midpoint.Price
			default:
				return nil, nil
			}
			return []Message{m}, nil
		}
		switch number {
		case 1, 2:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("tick-by-tick data", number, err)
			}
			if number == 1 {
				reqID = decodeProtoInt32(value)
			} else {
				tickType = decodeProtoInt32(value)
			}
		case 3:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("tick-by-tick data", number, err)
			}
			decoded, err := decodeHistoricalLastTickProto(value)
			if err != nil {
				return nil, err
			}
			last = &decoded
		case 4:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("tick-by-tick data", number, err)
			}
			decoded, err := decodeHistoricalBidAskTickProto(value)
			if err != nil {
				return nil, err
			}
			bidAsk = &decoded
		case 5:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("tick-by-tick data", number, err)
			}
			decoded, err := decodeHistoricalTickProto(value)
			if err != nil {
				return nil, err
			}
			midpoint = &decoded
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("tick-by-tick data", number, err)
			}
		}
	}
}

func decodeHistoricalScheduleProto(body []byte, sv int) ([]Message, error) {
	m := HistoricalScheduleResponse{ReqID: -1}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		switch number {
		case 1:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("historical schedule", number, err)
			}
			m.ReqID = decodeProtoInt32(value)
		case 2, 3, 4:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("historical schedule", number, err)
			}
			switch number {
			case 2:
				m.StartDateTime = string(value)
			case 3:
				m.EndDateTime = string(value)
			case 4:
				m.TimeZone = string(value)
			}
		case 5:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("historical schedule", number, err)
			}
			session, err := decodeHistoricalSessionProto(value)
			if err != nil {
				return nil, err
			}
			m.Sessions = append(m.Sessions, session)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("historical schedule", number, err)
			}
		}
	}
}

func decodeHistoricalSessionProto(body []byte) (HistoricalScheduleSession, error) {
	m := HistoricalScheduleSession{}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return HistoricalScheduleSession{}, err
		}
		if !ok {
			return m, nil
		}
		if number >= 1 && number <= 3 {
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return HistoricalScheduleSession{}, protoFieldError("historical session", number, err)
			}
			switch number {
			case 1:
				m.StartDateTime = string(value)
			case 2:
				m.EndDateTime = string(value)
			case 3:
				m.RefDate = string(value)
			}
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return HistoricalScheduleSession{}, protoFieldError("historical session", number, err)
		}
	}
}

func mustAtoi(value string) int {
	parsed, _ := strconv.Atoi(value)
	return parsed
}

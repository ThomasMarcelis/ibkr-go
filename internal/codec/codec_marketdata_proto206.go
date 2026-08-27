package codec

import (
	"fmt"
	"strings"
)

func (m QuoteRequest) encodeProto(sv int) ([]byte, error) {
	body, err := appendProtoInt(nil, 1, m.ReqID, "market data request id")
	if err != nil {
		return nil, err
	}
	contract, err := encodeSharedContractProto(m.Contract, nil, true)
	if err != nil {
		return nil, err
	}
	body = appendProtoMessage(body, 2, contract)
	if ticks := strings.Join(m.GenericTicks, ","); ticks != "" {
		body = appendProtoString(body, 3, ticks)
	}
	if m.Snapshot {
		body = appendProtoVarint(body, 4, 1)
	}
	if m.RegulatorySnapshot {
		body = appendProtoVarint(body, 5, 1)
	}
	return canonicalProtoFields(body), nil
}

func (m CancelQuote) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.ReqID, "cancel market data request id")
}

func (m MarketDepthRequest) encodeProto(sv int) ([]byte, error) {
	body, err := appendProtoInt(nil, 1, m.ReqID, "market depth request id")
	if err != nil {
		return nil, err
	}
	contract, err := encodeSharedContractProto(m.Contract, nil, true)
	if err != nil {
		return nil, err
	}
	body = appendProtoMessage(body, 2, contract)
	body, err = appendProtoInt(body, 3, m.NumRows, "market depth row count")
	if err != nil {
		return nil, err
	}
	if m.IsSmartDepth {
		body = appendProtoVarint(body, 4, 1)
	}
	return canonicalProtoFields(body), nil
}

func (m CancelMarketDepth) encodeProto(sv int) ([]byte, error) {
	body, err := appendProtoInt(nil, 1, m.ReqID, "cancel market depth request id")
	if err != nil {
		return nil, err
	}
	if m.IsSmartDepth {
		body = appendProtoVarint(body, 2, 1)
	}
	return body, nil
}

func (m ReqMarketDataType) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.DataType, "market data type")
}

func decodeTickPriceProto(body []byte, sv int) ([]Message, error) {
	// EDecoder.cpp defaults omitted proto3 optional scalars before invoking the
	// classic callback: reqId=-1, tickType=0, price=0, size=UNSET_DECIMAL, and
	// attrMask=0. Keep the empty size string as the codec's unavailable decimal.
	m := TickPrice{ReqID: -1, Price: "0"}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		switch number {
		case 1, 2, 5:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("tick price", number, err)
			}
			switch number {
			case 1:
				m.ReqID = decodeProtoInt32(value)
			case 2:
				m.TickType = decodeProtoInt32(value)
			case 5:
				m.AttrMask = decodeProtoInt32(value)
			}
		case 3:
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return nil, protoFieldError("tick price", number, err)
			}
			m.Price = formatProtoDouble(value)
		case 4:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("tick price", number, err)
			}
			m.Size = string(value)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("tick price", number, err)
			}
		}
	}
}

func decodeTickSizeProto(body []byte, sv int) ([]Message, error) {
	// An omitted size is UNSET_DECIMAL in the official decoder. The empty codec
	// string carries that absence into the public presence-aware size pointer.
	m := TickSize{ReqID: -1}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		switch number {
		case 1, 2:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("tick size", number, err)
			}
			if number == 1 {
				m.ReqID = decodeProtoInt32(value)
			} else {
				m.TickType = decodeProtoInt32(value)
			}
		case 3:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("tick size", number, err)
			}
			m.Size = string(value)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("tick size", number, err)
			}
		}
	}
}

func decodeTickOptionComputationProto(body []byte, sv int) ([]Message, error) {
	m := TickOptionComputation{ReqID: -1}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		switch number {
		case 1, 2, 3:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("tick option computation", number, err)
			}
			switch number {
			case 1:
				m.ReqID = decodeProtoInt32(value)
			case 2:
				m.TickType = decodeProtoInt32(value)
			case 3:
				m.TickAttrib = decodeProtoInt32(value)
			}
		case 4, 5, 6, 7, 8, 9, 10, 11:
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return nil, protoFieldError("tick option computation", number, err)
			}
			formatted := formatProtoDouble(value)
			switch number {
			case 4:
				m.ImpliedVol = formatted
			case 5:
				m.Delta = formatted
			case 6:
				m.OptPrice = formatted
			case 7:
				m.PvDividend = formatted
			case 8:
				m.Gamma = formatted
			case 9:
				m.Vega = formatted
			case 10:
				m.Theta = formatted
			case 11:
				m.UndPrice = formatted
			}
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("tick option computation", number, err)
			}
		}
	}
}

func decodeTickGenericProto(body []byte, sv int) ([]Message, error) {
	m := TickGeneric{ReqID: -1, Value: "0"}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		switch number {
		case 1, 2:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("tick generic", number, err)
			}
			if number == 1 {
				m.ReqID = decodeProtoInt32(value)
			} else {
				m.TickType = decodeProtoInt32(value)
			}
		case 3:
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return nil, protoFieldError("tick generic", number, err)
			}
			m.Value = formatProtoDouble(value)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("tick generic", number, err)
			}
		}
	}
}

func decodeTickStringProto(body []byte, sv int) ([]Message, error) {
	m := TickString{ReqID: -1}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		switch number {
		case 1, 2:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("tick string", number, err)
			}
			if number == 1 {
				m.ReqID = decodeProtoInt32(value)
			} else {
				m.TickType = decodeProtoInt32(value)
			}
		case 3:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("tick string", number, err)
			}
			m.Value = string(value)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("tick string", number, err)
			}
		}
	}
}

func decodeTickSnapshotEndProto(body []byte, sv int) ([]Message, error) {
	m := TickSnapshotEnd{ReqID: -1}
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
				return nil, protoFieldError("tick snapshot end", number, err)
			}
			m.ReqID = decodeProtoInt32(value)
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return nil, protoFieldError("tick snapshot end", number, err)
		}
	}
}

func decodeMarketDataTypeProto(body []byte, sv int) ([]Message, error) {
	m := MarketDataType{ReqID: -1}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		if number == 1 || number == 2 {
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("market data type", number, err)
			}
			if number == 1 {
				m.ReqID = decodeProtoInt32(value)
			} else {
				m.DataType = decodeProtoInt32(value)
			}
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return nil, protoFieldError("market data type", number, err)
		}
	}
}

func decodeMarketDataRerouteProto(body []byte, sv int) ([]Message, error) {
	reqID, conID, exchange, err := decodeMarketDataRerouteBodyProto(body, "market data reroute")
	if err != nil {
		return nil, err
	}
	return []Message{MarketDataReroute{ReqID: reqID, ConID: conID, Exchange: exchange}}, nil
}

func decodeMarketDepthRerouteProto(body []byte, sv int) ([]Message, error) {
	reqID, conID, exchange, err := decodeMarketDataRerouteBodyProto(body, "market depth reroute")
	if err != nil {
		return nil, err
	}
	return []Message{MarketDepthReroute{ReqID: reqID, ConID: conID, Exchange: exchange}}, nil
}

func decodeMarketDataRerouteBodyProto(body []byte, label string) (int, int, string, error) {
	var reqID int
	var hasReqID bool
	var conID int
	var exchange string
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return 0, 0, "", err
		}
		if !ok {
			if !hasReqID {
				return 0, 0, "", fmt.Errorf("%s missing required request id", label)
			}
			if reqID <= 0 {
				return 0, 0, "", fmt.Errorf("%s invalid request id %d", label, reqID)
			}
			return reqID, conID, exchange, nil
		}
		switch number {
		case 1, 2:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return 0, 0, "", protoFieldError(label, number, err)
			}
			if number == 1 {
				reqID = decodeProtoInt32(value)
				hasReqID = true
			} else {
				conID = decodeProtoInt32(value)
			}
		case 3:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return 0, 0, "", protoFieldError(label, number, err)
			}
			exchange = string(value)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return 0, 0, "", protoFieldError(label, number, err)
			}
		}
	}
}

func decodeMarketDepthProto(body []byte, sv int) ([]Message, error) {
	reqID, depth, present, err := decodeMarketDepthMessageProto(body, "market depth")
	if err != nil || !present {
		return nil, err
	}
	return []Message{MarketDepthUpdate{
		ReqID: reqID, Position: depth.Position, Operation: depth.Operation,
		Side: depth.Side, Price: depth.Price, Size: depth.Size,
	}}, nil
}

func decodeMarketDepthL2Proto(body []byte, sv int) ([]Message, error) {
	reqID, depth, present, err := decodeMarketDepthMessageProto(body, "market depth L2")
	if err != nil || !present {
		return nil, err
	}
	return []Message{MarketDepthL2Update{
		ReqID: reqID, Position: depth.Position, MarketMaker: depth.MarketMaker,
		Operation: depth.Operation, Side: depth.Side, Price: depth.Price,
		Size: depth.Size, IsSmartDepth: depth.IsSmartDepth,
	}}, nil
}

type protoMarketDepth struct {
	Position     int
	Operation    int
	Side         int
	Price        string
	Size         string
	MarketMaker  string
	IsSmartDepth bool
}

func decodeMarketDepthMessageProto(body []byte, label string) (int, protoMarketDepth, bool, error) {
	var reqID int
	var hasReqID bool
	depth := protoMarketDepth{Price: "0"}
	var depthPresent bool
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return 0, protoMarketDepth{}, false, err
		}
		if !ok {
			if !depthPresent {
				return 0, protoMarketDepth{}, false, fmt.Errorf("%s missing required depth data", label)
			}
			if !hasReqID {
				return 0, protoMarketDepth{}, false, fmt.Errorf("%s missing required request id", label)
			}
			if reqID <= 0 {
				return 0, protoMarketDepth{}, false, fmt.Errorf("%s invalid request id %d", label, reqID)
			}
			return reqID, depth, true, nil
		}
		switch number {
		case 1:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return 0, protoMarketDepth{}, false, protoFieldError(label, number, err)
			}
			reqID = decodeProtoInt32(value)
			hasReqID = true
		case 2:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return 0, protoMarketDepth{}, false, protoFieldError(label, number, err)
			}
			if err := decodeMarketDepthDataProto(value, &depth); err != nil {
				return 0, protoMarketDepth{}, false, protoFieldError(label+" data", number, err)
			}
			depthPresent = true
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return 0, protoMarketDepth{}, false, protoFieldError(label, number, err)
			}
		}
	}
}

func decodeMarketDepthDataProto(body []byte, depth *protoMarketDepth) error {
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		switch number {
		case 1, 2, 3:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return protoFieldError("market depth data", number, err)
			}
			switch number {
			case 1:
				depth.Position = decodeProtoInt32(value)
			case 2:
				depth.Operation = decodeProtoInt32(value)
			case 3:
				depth.Side = decodeProtoInt32(value)
			}
		case 4:
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return protoFieldError("market depth data", number, err)
			}
			depth.Price = formatProtoDouble(value)
		case 5, 6:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return protoFieldError("market depth data", number, err)
			}
			if number == 5 {
				depth.Size = string(value)
			} else {
				depth.MarketMaker = string(value)
			}
		case 7:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return protoFieldError("market depth data", number, err)
			}
			depth.IsSmartDepth = value != 0
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return protoFieldError("market depth data", number, err)
			}
		}
	}
}

func decodeTickReqParamsProto(body []byte, sv int) ([]Message, error) {
	m := TickReqParams{ReqID: -1}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		switch number {
		case 1, 4:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("tick request parameters", number, err)
			}
			if number == 1 {
				m.ReqID = decodeProtoInt32(value)
			} else {
				m.SnapshotPermissions = new(decodeProtoInt32(value))
			}
		case 2, 3, 5, 6:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("tick request parameters", number, err)
			}
			switch number {
			case 2:
				m.MinTick = string(value)
			case 3:
				m.BBOExchange = string(value)
			case 5:
				m.LastPricePrecision = string(value)
			case 6:
				m.LastSizePrecision = string(value)
			}
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("tick request parameters", number, err)
			}
		}
	}
}

package codec

import (
	"strconv"

	"google.golang.org/protobuf/encoding/protowire"
)

func (m StartAPI) encodeProto(sv int) ([]byte, error) {
	body, err := appendProtoInt(nil, 1, m.ClientID, "start API client id")
	if err != nil {
		return nil, err
	}
	if m.OptionalCapabilities != "" {
		body = appendProtoString(body, 2, m.OptionalCapabilities)
	}
	return body, nil
}

func (m ReqIDsRequest) encodeProto(sv int) ([]byte, error) {
	numIDs := m.NumIDs
	if numIDs <= 0 {
		numIDs = 1
	}
	return appendProtoInt(nil, 1, numIDs, "requested order-id count")
}

func (CurrentTimeRequest) encodeProto(sv int) ([]byte, error)       { return []byte{}, nil }
func (CurrentTimeMillisRequest) encodeProto(sv int) ([]byte, error) { return []byte{}, nil }

func (m QueryDisplayGroupsRequest) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.ReqID, "display-group query request id")
}

func (m SubscribeToGroupEventsRequest) encodeProto(sv int) ([]byte, error) {
	body, err := appendProtoInt(nil, 1, m.ReqID, "display-group subscription request id")
	if err != nil {
		return nil, err
	}
	return appendProtoInt(body, 2, m.GroupID, "display-group id")
}

func (m UpdateDisplayGroupRequest) encodeProto(sv int) ([]byte, error) {
	body, err := appendProtoInt(nil, 1, m.ReqID, "display-group update request id")
	if err != nil {
		return nil, err
	}
	if m.ContractInfo != "" {
		body = appendProtoString(body, 2, m.ContractInfo)
	}
	return body, nil
}

func (m UnsubscribeFromGroupEventsRequest) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.ReqID, "display-group unsubscribe request id")
}

func (MktDepthExchangesRequest) encodeProto(sv int) ([]byte, error) { return []byte{}, nil }

func decodeNextValidIDProto(body []byte, sv int) ([]Message, error) {
	orderID, err := decodeSingleProtoInt32(body, 1, "next valid order id")
	if err != nil {
		return nil, err
	}
	return []Message{NextValidID{OrderID: int64(orderID)}}, nil
}

func decodeCurrentTimeProto(body []byte, sv int) ([]Message, error) {
	value, err := decodeSingleProtoInt64(body, 1, "current time")
	if err != nil {
		return nil, err
	}
	return []Message{CurrentTime{Time: strconv.FormatInt(value, 10)}}, nil
}

func decodeCurrentTimeMillisProto(body []byte, sv int) ([]Message, error) {
	value, err := decodeSingleProtoInt64(body, 1, "current time in milliseconds")
	if err != nil {
		return nil, err
	}
	return []Message{CurrentTimeMillis{TimeMs: strconv.FormatInt(value, 10)}}, nil
}

func decodeDisplayGroupListProto(body []byte, sv int) ([]Message, error) {
	m := DisplayGroupList{ReqID: -1}
	if err := decodeDisplayGroupMessage(body, "display-group list", &m.ReqID, &m.Groups); err != nil {
		return nil, err
	}
	return []Message{m}, nil
}

func decodeDisplayGroupUpdatedProto(body []byte, sv int) ([]Message, error) {
	m := DisplayGroupUpdated{ReqID: -1}
	if err := decodeDisplayGroupMessage(body, "display-group update", &m.ReqID, &m.ContractInfo); err != nil {
		return nil, err
	}
	return []Message{m}, nil
}

func decodeDisplayGroupMessage(body []byte, label string, reqID *int, value *string) error {
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		switch number {
		case 1:
			encoded, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return protoFieldError(label, number, err)
			}
			*reqID = decodeProtoInt32(encoded)
		case 2:
			encoded, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return protoFieldError(label, number, err)
			}
			*value = string(encoded)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return protoFieldError(label, number, err)
			}
		}
	}
}

func decodeMktDepthExchangesProto(body []byte, sv int) ([]Message, error) {
	m := MktDepthExchanges{}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		if number == 1 {
			encoded, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("market-depth exchanges", number, err)
			}
			entry, err := decodeDepthExchangeProto(encoded)
			if err != nil {
				return nil, err
			}
			m.Exchanges = append(m.Exchanges, entry)
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return nil, protoFieldError("market-depth exchanges", number, err)
		}
	}
}

func decodeDepthExchangeProto(body []byte) (DepthExchangeEntry, error) {
	m := DepthExchangeEntry{}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return DepthExchangeEntry{}, err
		}
		if !ok {
			return m, nil
		}
		switch number {
		case 1, 2, 3, 4:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return DepthExchangeEntry{}, protoFieldError("market-depth exchange", number, err)
			}
			switch number {
			case 1:
				m.Exchange = string(value)
			case 2:
				m.SecType = string(value)
			case 3:
				m.ListingExch = string(value)
			case 4:
				m.ServiceDataType = string(value)
			}
		case 5:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return DepthExchangeEntry{}, protoFieldError("market-depth exchange", number, err)
			}
			m.AggGroup = decodeProtoInt32(value)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return DepthExchangeEntry{}, protoFieldError("market-depth exchange", number, err)
			}
		}
	}
}

func decodeSingleProtoInt64(body []byte, want protowire.Number, label string) (int64, error) {
	var value int64
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return 0, err
		}
		if !ok {
			return value, nil
		}
		if number == want {
			encoded, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return 0, protoFieldError(label, number, err)
			}
			value = int64(encoded) // #nosec G115 -- protobuf int64 preserves its two's-complement bits
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return 0, protoFieldError(label, number, err)
		}
	}
}

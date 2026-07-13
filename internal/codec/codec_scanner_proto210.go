package codec

import (
	"fmt"
	"strconv"

	"google.golang.org/protobuf/encoding/protowire"
)

func (ScannerParametersRequest) encodeProto(sv int) ([]byte, error) { return []byte{}, nil }

func (m ScannerSubscriptionRequest) encodeProto(sv int) ([]byte, error) {
	body, err := appendProtoInt(nil, 1, m.ReqID, "scanner request id")
	if err != nil {
		return nil, err
	}
	subscription, err := encodeScannerSubscriptionProto(m)
	if err != nil {
		return nil, err
	}
	return appendProtoMessage(body, 2, subscription), nil
}

func encodeScannerSubscriptionProto(m ScannerSubscriptionRequest) ([]byte, error) {
	var body []byte
	var err error
	if m.NumberOfRows >= 0 {
		body, err = appendProtoInt(body, 1, m.NumberOfRows, "scanner row count")
		if err != nil {
			return nil, err
		}
	}
	for _, field := range []struct {
		number protowire.Number
		value  string
	}{{2, m.Instrument}, {3, m.LocationCode}, {4, m.ScanCode}, {10, m.MoodyRatingAbove}, {11, m.MoodyRatingBelow}, {12, m.SPRatingAbove}, {13, m.SPRatingBelow}, {14, m.MaturityDateAbove}, {15, m.MaturityDateBelow}, {20, m.ScannerSettingPairs}, {21, m.StockTypeFilter}} {
		if field.value != "" {
			body = appendProtoString(body, field.number, field.value)
		}
	}
	for _, field := range []struct {
		number protowire.Number
		value  string
		label  string
	}{{5, m.AbovePrice, "above price"}, {6, m.BelowPrice, "below price"}, {8, m.MarketCapAbove, "market cap above"}, {9, m.MarketCapBelow, "market cap below"}, {16, m.CouponRateAbove, "coupon rate above"}, {17, m.CouponRateBelow, "coupon rate below"}} {
		body, err = appendOptionalProtoDouble(body, field.number, field.value, "scanner "+field.label)
		if err != nil {
			return nil, err
		}
	}
	body, err = appendOptionalProtoInt64String(body, 7, m.AboveVolume, "scanner above volume")
	if err != nil {
		return nil, err
	}
	body, err = appendOptionalProtoInt64String(body, 19, m.AverageOptionVolumeAbove, "scanner average option volume")
	if err != nil {
		return nil, err
	}
	switch m.ExcludeConvertible {
	case "":
	case "0":
		body = appendProtoVarint(body, 18, 0)
	case "1":
		body = appendProtoVarint(body, 18, 1)
	default:
		return nil, fmt.Errorf("codec: scanner exclude convertible %q is not a protobuf bool", m.ExcludeConvertible)
	}
	body = appendProtoMap(body, 22, m.FilterOptions)
	body = appendProtoMap(body, 23, m.SubscriptionOptions)
	return canonicalProtoFields(body), nil
}

func appendOptionalProtoInt64String(body []byte, number protowire.Number, value, label string) ([]byte, error) {
	if value == "" {
		return body, nil
	}
	parsed, err := strconv.ParseUint(value, 10, 63)
	if err != nil {
		return nil, fmt.Errorf("codec: %s %q is not a non-negative protobuf int64: %w", label, value, err)
	}
	return appendProtoVarint(body, number, parsed), nil
}

func (m CancelScannerSubscription) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.ReqID, "cancel scanner request id")
}

func (m PnLRequest) encodeProto(sv int) ([]byte, error) {
	body, err := appendProtoInt(nil, 1, m.ReqID, "PnL request id")
	if err != nil {
		return nil, err
	}
	if m.Account != "" {
		body = appendProtoString(body, 2, m.Account)
	}
	if m.ModelCode != "" {
		body = appendProtoString(body, 3, m.ModelCode)
	}
	return body, nil
}

func (m CancelPnL) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.ReqID, "cancel PnL request id")
}

func (m PnLSingleRequest) encodeProto(sv int) ([]byte, error) {
	body, err := appendProtoInt(nil, 1, m.ReqID, "single PnL request id")
	if err != nil {
		return nil, err
	}
	if m.Account != "" {
		body = appendProtoString(body, 2, m.Account)
	}
	if m.ModelCode != "" {
		body = appendProtoString(body, 3, m.ModelCode)
	}
	body, err = appendProtoInt(body, 4, m.ConID, "single PnL conid")
	if err != nil {
		return nil, err
	}
	return canonicalProtoFields(body), nil
}

func (m CancelPnLSingle) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.ReqID, "cancel single PnL request id")
}

func decodeScannerParametersProto(body []byte, sv int) ([]Message, error) {
	m := ScannerParameters{}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		if number == 1 {
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("scanner parameters", number, err)
			}
			m.XML = string(value)
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return nil, protoFieldError("scanner parameters", number, err)
		}
	}
}

func decodeScannerDataProto(body []byte, sv int) ([]Message, error) {
	m := ScannerDataResponse{ReqID: -1}
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
				return nil, protoFieldError("scanner data", number, err)
			}
			m.ReqID = decodeProtoInt32(value)
		} else if number == 2 {
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("scanner data", number, err)
			}
			entry, err := decodeScannerDataEntryProto(value)
			if err != nil {
				return nil, err
			}
			m.Entries = append(m.Entries, entry)
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return nil, protoFieldError("scanner data", number, err)
		}
	}
}

func decodeScannerDataEntryProto(body []byte) (ScannerDataEntry, error) {
	m := ScannerDataEntry{}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return ScannerDataEntry{}, err
		}
		if !ok {
			return m, nil
		}
		switch number {
		case 1:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return ScannerDataEntry{}, protoFieldError("scanner data entry", number, err)
			}
			m.Rank = decodeProtoInt32(value)
		case 2:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return ScannerDataEntry{}, protoFieldError("scanner data entry", number, err)
			}
			contract, err := decodeSharedContractProto(value)
			if err != nil {
				return ScannerDataEntry{}, err
			}
			m.Contract = contract.Contract
		case 3, 4, 5, 6, 7:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return ScannerDataEntry{}, protoFieldError("scanner data entry", number, err)
			}
			switch number {
			case 3:
				m.MarketName = string(value)
			case 4:
				m.Distance = string(value)
			case 5:
				m.Benchmark = string(value)
			case 6:
				m.Projection = string(value)
			case 7:
				m.LegsStr = string(value)
			}
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return ScannerDataEntry{}, protoFieldError("scanner data entry", number, err)
			}
		}
	}
}

func decodePnLProto(body []byte, sv int) ([]Message, error) {
	m := PnLValue{ReqID: -1}
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
				return nil, protoFieldError("PnL", number, err)
			}
			m.ReqID = decodeProtoInt32(value)
		} else if number >= 2 && number <= 4 {
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return nil, protoFieldError("PnL", number, err)
			}
			formatted := formatProtoDouble(value)
			switch number {
			case 2:
				m.DailyPnL = formatted
			case 3:
				m.UnrealizedPnL = formatted
			case 4:
				m.RealizedPnL = formatted
			}
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return nil, protoFieldError("PnL", number, err)
		}
	}
}

func decodePnLSingleProto(body []byte, sv int) ([]Message, error) {
	m := PnLSingleValue{ReqID: -1}
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
				return nil, protoFieldError("single PnL", number, err)
			}
			m.ReqID = decodeProtoInt32(value)
		case 2:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("single PnL", number, err)
			}
			m.Position = string(value)
		case 3, 4, 5, 6:
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return nil, protoFieldError("single PnL", number, err)
			}
			formatted := formatProtoDouble(value)
			switch number {
			case 3:
				m.DailyPnL = formatted
			case 4:
				m.UnrealizedPnL = formatted
			case 5:
				m.RealizedPnL = formatted
			case 6:
				m.Value = formatted
			}
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("single PnL", number, err)
			}
		}
	}
}

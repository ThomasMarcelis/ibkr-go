package codec

import (
	"math"

	"google.golang.org/protobuf/encoding/protowire"
)

func (m SecDefOptParamsRequest) encodeProto(sv int) ([]byte, error) {
	body, err := appendProtoInt(nil, 1, m.ReqID, "security-definition option request id")
	if err != nil {
		return nil, err
	}
	for _, field := range []struct {
		number protowire.Number
		value  string
	}{{2, m.UnderlyingSymbol}, {3, m.FutFopExchange}, {4, m.UnderlyingSecType}} {
		if field.value != "" {
			body = appendProtoString(body, field.number, field.value)
		}
	}
	return appendProtoInt(body, 5, m.UnderlyingConID, "security-definition underlying conid")
}

func (m SoftDollarTiersRequest) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.ReqID, "soft-dollar request id")
}

func (FamilyCodesRequest) encodeProto(sv int) ([]byte, error) { return []byte{}, nil }

func (m MatchingSymbolsRequest) encodeProto(sv int) ([]byte, error) {
	body, err := appendProtoInt(nil, 1, m.ReqID, "matching-symbols request id")
	if err != nil {
		return nil, err
	}
	if m.Pattern != "" {
		body = appendProtoString(body, 2, m.Pattern)
	}
	return body, nil
}

func (m SmartComponentsRequest) encodeProto(sv int) ([]byte, error) {
	body, err := appendProtoInt(nil, 1, m.ReqID, "smart-components request id")
	if err != nil {
		return nil, err
	}
	if m.BBOExchange != "" {
		body = appendProtoString(body, 2, m.BBOExchange)
	}
	return body, nil
}

func (m MarketRuleRequest) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.MarketRuleID, "market-rule id")
}

func (m UserInfoRequest) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.ReqID, "user-info request id")
}

func decodeSecDefOptParamsProto(body []byte, sv int) ([]Message, error) {
	m := SecDefOptParamsResponse{ReqID: -1}
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
				return nil, protoFieldError("security-definition option parameter", number, err)
			}
			if number == 1 {
				m.ReqID = decodeProtoInt32(value)
			} else {
				m.UnderlyingConID = decodeProtoInt32(value)
			}
		case 2, 4, 5, 6:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("security-definition option parameter", number, err)
			}
			switch number {
			case 2:
				m.Exchange = string(value)
			case 4:
				m.TradingClass = string(value)
			case 5:
				m.Multiplier = string(value)
			case 6:
				m.Expirations = append(m.Expirations, string(value))
			}
		case 7:
			values, err := consumeProtoRepeatedDoubles(&body, typ)
			if err != nil {
				return nil, protoFieldError("security-definition option parameter", number, err)
			}
			for _, value := range values {
				m.Strikes = append(m.Strikes, formatProtoDouble(value))
			}
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("security-definition option parameter", number, err)
			}
		}
	}
}

func consumeProtoRepeatedDoubles(body *[]byte, typ protowire.Type) ([]float64, error) {
	if typ == protowire.Fixed64Type {
		value, err := consumeProtoDouble(body, typ)
		return []float64{value}, err
	}
	packed, err := consumeProtoBytes(body, typ)
	if err != nil {
		return nil, err
	}
	values := make([]float64, 0, len(packed)/8)
	for len(packed) != 0 {
		bits, n := protowire.ConsumeFixed64(packed)
		if n < 0 {
			return nil, protowire.ParseError(n)
		}
		packed = packed[n:]
		values = append(values, math.Float64frombits(bits))
	}
	return values, nil
}

func decodeSecDefOptParamsEndProto(body []byte, sv int) ([]Message, error) {
	reqID, err := decodeSingleProtoInt32(body, 1, "security-definition option parameter end")
	if err != nil {
		return nil, err
	}
	return []Message{SecDefOptParamsEnd{ReqID: reqID}}, nil
}

func decodeSoftDollarTiersProto(body []byte, sv int) ([]Message, error) {
	m := SoftDollarTiersResponse{ReqID: -1}
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
				return nil, protoFieldError("soft-dollar tiers", number, err)
			}
			m.ReqID = decodeProtoInt32(value)
		case 2:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("soft-dollar tiers", number, err)
			}
			tier, err := decodeSoftDollarTierProto(value)
			if err != nil {
				return nil, err
			}
			m.Tiers = append(m.Tiers, tier)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("soft-dollar tiers", number, err)
			}
		}
	}
}

func decodeSoftDollarTierProto(body []byte) (SoftDollarTier, error) {
	m := SoftDollarTier{}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return SoftDollarTier{}, err
		}
		if !ok {
			return m, nil
		}
		if number >= 1 && number <= 3 {
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return SoftDollarTier{}, protoFieldError("soft-dollar tier", number, err)
			}
			switch number {
			case 1:
				m.Name = string(value)
			case 2:
				m.Value = string(value)
			case 3:
				m.DisplayName = string(value)
			}
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return SoftDollarTier{}, protoFieldError("soft-dollar tier", number, err)
		}
	}
}

func decodeFamilyCodesProto(body []byte, sv int) ([]Message, error) {
	m := FamilyCodes{}
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
				return nil, protoFieldError("family codes", number, err)
			}
			entry, err := decodeFamilyCodeProto(value)
			if err != nil {
				return nil, err
			}
			m.Codes = append(m.Codes, entry)
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return nil, protoFieldError("family codes", number, err)
		}
	}
}

func decodeFamilyCodeProto(body []byte) (FamilyCodeEntry, error) {
	m := FamilyCodeEntry{}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return FamilyCodeEntry{}, err
		}
		if !ok {
			return m, nil
		}
		if number == 1 || number == 2 {
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return FamilyCodeEntry{}, protoFieldError("family code", number, err)
			}
			if number == 1 {
				m.AccountID = string(value)
			} else {
				m.FamilyCode = string(value)
			}
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return FamilyCodeEntry{}, protoFieldError("family code", number, err)
		}
	}
}

func decodeSymbolSamplesProto(body []byte, sv int) ([]Message, error) {
	m := MatchingSymbols{ReqID: -1}
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
				return nil, protoFieldError("symbol samples", number, err)
			}
			m.ReqID = decodeProtoInt32(value)
		case 2:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("symbol samples", number, err)
			}
			sample, err := decodeContractDescriptionProto(value)
			if err != nil {
				return nil, err
			}
			m.Symbols = append(m.Symbols, sample)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("symbol samples", number, err)
			}
		}
	}
}

func decodeContractDescriptionProto(body []byte) (SymbolSample, error) {
	m := SymbolSample{}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return SymbolSample{}, err
		}
		if !ok {
			return m, nil
		}
		if number == 1 {
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return SymbolSample{}, protoFieldError("contract description", number, err)
			}
			contract, err := decodeSharedContractProto(value)
			if err != nil {
				return SymbolSample{}, err
			}
			m.ConID = contract.Contract.ConID
			m.Symbol = contract.Contract.Symbol
			m.SecType = contract.Contract.SecType
			m.PrimaryExchange = contract.Contract.PrimaryExchange
			m.Currency = contract.Contract.Currency
			m.Description = contract.Description
			m.IssuerID = contract.Contract.IssuerID
		} else if number == 2 {
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return SymbolSample{}, protoFieldError("contract description", number, err)
			}
			m.DerivativeSecTypes = append(m.DerivativeSecTypes, string(value))
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return SymbolSample{}, protoFieldError("contract description", number, err)
		}
	}
}

func decodeSmartComponentsProto(body []byte, sv int) ([]Message, error) {
	m := SmartComponentsResponse{ReqID: -1}
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
				return nil, protoFieldError("smart components", number, err)
			}
			m.ReqID = decodeProtoInt32(value)
		case 2:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("smart components", number, err)
			}
			component, err := decodeSmartComponentProto(value)
			if err != nil {
				return nil, err
			}
			m.Components = append(m.Components, component)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("smart components", number, err)
			}
		}
	}
}

func decodeSmartComponentProto(body []byte) (SmartComponentEntry, error) {
	m := SmartComponentEntry{}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return SmartComponentEntry{}, err
		}
		if !ok {
			return m, nil
		}
		switch number {
		case 1:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return SmartComponentEntry{}, protoFieldError("smart component", number, err)
			}
			m.BitNumber = decodeProtoInt32(value)
		case 2, 3:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return SmartComponentEntry{}, protoFieldError("smart component", number, err)
			}
			if number == 2 {
				m.ExchangeName = string(value)
			} else {
				m.ExchangeLetter = string(value)
			}
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return SmartComponentEntry{}, protoFieldError("smart component", number, err)
			}
		}
	}
}

func decodeMarketRuleProto(body []byte, sv int) ([]Message, error) {
	m := MarketRule{MarketRuleID: -1}
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
				return nil, protoFieldError("market rule", number, err)
			}
			m.MarketRuleID = decodeProtoInt32(value)
		case 2:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("market rule", number, err)
			}
			increment, err := decodePriceIncrementProto(value)
			if err != nil {
				return nil, err
			}
			m.Increments = append(m.Increments, increment)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("market rule", number, err)
			}
		}
	}
}

func decodePriceIncrementProto(body []byte) (PriceIncrement, error) {
	m := PriceIncrement{}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return PriceIncrement{}, err
		}
		if !ok {
			return m, nil
		}
		if number == 1 || number == 2 {
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return PriceIncrement{}, protoFieldError("price increment", number, err)
			}
			if number == 1 {
				m.LowEdge = formatProtoDouble(value)
			} else {
				m.Increment = formatProtoDouble(value)
			}
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return PriceIncrement{}, protoFieldError("price increment", number, err)
		}
	}
}

func decodeUserInfoProto(body []byte, sv int) ([]Message, error) {
	m := UserInfo{ReqID: -1}
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
				return nil, protoFieldError("user info", number, err)
			}
			m.ReqID = decodeProtoInt32(value)
		case 2:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("user info", number, err)
			}
			m.WhiteBrandingID = string(value)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("user info", number, err)
			}
		}
	}
}

func decodeSingleProtoInt32(body []byte, want protowire.Number, label string) (int, error) {
	value := -1
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
			value = decodeProtoInt32(encoded)
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return 0, protoFieldError(label, number, err)
		}
	}
}

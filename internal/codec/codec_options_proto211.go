package codec

import "fmt"

func (m RequestFA) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.FADataType, "FA data type")
}

func (m ExerciseOptionsRequest) encodeProto(sv int) ([]byte, error) {
	body, err := appendProtoInt(nil, 1, m.ReqID, "exercise order id")
	if err != nil {
		return nil, err
	}
	contract, err := encodeSharedContractProto(m.Contract, nil, true)
	if err != nil {
		return nil, err
	}
	body = appendProtoMessage(body, 2, contract)
	body, err = appendProtoInt(body, 3, m.ExerciseAction, "exercise action")
	if err != nil {
		return nil, err
	}
	body, err = appendProtoInt(body, 4, m.ExerciseQuantity, "exercise quantity")
	if err != nil {
		return nil, err
	}
	if m.Account != "" {
		body = appendProtoString(body, 5, m.Account)
	}
	switch m.Override {
	case 0:
	case 1:
		body = appendProtoVarint(body, 6, 1)
	default:
		return nil, fmt.Errorf("codec: exercise override %d is not a protobuf bool", m.Override)
	}
	return canonicalProtoFields(body), nil
}

func (m CalcImpliedVolatilityRequest) encodeProto(sv int) ([]byte, error) {
	body, err := calculationRequestProto(m.ReqID, m.Contract, "implied volatility")
	if err != nil {
		return nil, err
	}
	body, err = appendRequiredProtoDouble(body, 3, m.OptionPrice, "option price")
	if err != nil {
		return nil, err
	}
	body, err = appendRequiredProtoDouble(body, 4, m.UnderPrice, "underlying price")
	if err != nil {
		return nil, err
	}
	return canonicalProtoFields(body), nil
}

func (m CancelCalcImpliedVolatility) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.ReqID, "cancel implied volatility request id")
}

func (m CalcOptionPriceRequest) encodeProto(sv int) ([]byte, error) {
	body, err := calculationRequestProto(m.ReqID, m.Contract, "option price")
	if err != nil {
		return nil, err
	}
	body, err = appendRequiredProtoDouble(body, 3, m.Volatility, "volatility")
	if err != nil {
		return nil, err
	}
	body, err = appendRequiredProtoDouble(body, 4, m.UnderPrice, "underlying price")
	if err != nil {
		return nil, err
	}
	return canonicalProtoFields(body), nil
}

func calculationRequestProto(reqID int, contract Contract, label string) ([]byte, error) {
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

func (m CancelCalcOptionPrice) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.ReqID, "cancel option price request id")
}

func decodeReceiveFAProto(body []byte, sv int) ([]Message, error) {
	m := ReceiveFA{}
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
				return nil, protoFieldError("FA data", number, err)
			}
			m.FADataType = decodeProtoInt32(value)
		} else if number == 2 {
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("FA data", number, err)
			}
			m.XML = string(value)
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return nil, protoFieldError("FA data", number, err)
		}
	}
}

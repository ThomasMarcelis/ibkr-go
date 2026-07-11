package codec

import (
	"fmt"
	"strings"

	"github.com/ThomasMarcelis/ibkr-go/internal/protocol"
)

func requireAccountProto(sv int, operation string) error {
	if sv < protocol.MinServerVersionProtobufAccountsPositions {
		return fmt.Errorf("codec: %s protobuf requires server_version %d", operation, protocol.MinServerVersionProtobufAccountsPositions)
	}
	return nil
}

func (AccountUpdatesRequest) protobufVersion() int {
	return protocol.MinServerVersionProtobufAccountsPositions
}

func (m AccountUpdatesRequest) encodeProto(sv int) ([]byte, error) {
	if err := requireAccountProto(sv, "account updates"); err != nil {
		return nil, err
	}
	var body []byte
	if m.Subscribe {
		body = appendProtoVarint(body, 1, 1)
	}
	if m.Account != "" {
		body = appendProtoString(body, 2, m.Account)
	}
	return body, nil
}

func (ManagedAccountsRequest) protobufVersion() int {
	return protocol.MinServerVersionProtobufAccountsPositions
}

func (ManagedAccountsRequest) encodeProto(sv int) ([]byte, error) {
	if err := requireAccountProto(sv, "managed accounts"); err != nil {
		return nil, err
	}
	return []byte{}, nil
}

func (PositionsRequest) protobufVersion() int {
	return protocol.MinServerVersionProtobufAccountsPositions
}

func (PositionsRequest) encodeProto(sv int) ([]byte, error) {
	if err := requireAccountProto(sv, "positions"); err != nil {
		return nil, err
	}
	return []byte{}, nil
}

func (CancelPositions) protobufVersion() int {
	return protocol.MinServerVersionProtobufAccountsPositions
}

func (CancelPositions) encodeProto(sv int) ([]byte, error) {
	if err := requireAccountProto(sv, "cancel positions"); err != nil {
		return nil, err
	}
	return []byte{}, nil
}

func (AccountSummaryRequest) protobufVersion() int {
	return protocol.MinServerVersionProtobufAccountsPositions
}

func (m AccountSummaryRequest) encodeProto(sv int) ([]byte, error) {
	if err := requireAccountProto(sv, "account summary"); err != nil {
		return nil, err
	}
	body, err := appendProtoInt(nil, 1, m.ReqID, "account summary request id")
	if err != nil {
		return nil, err
	}
	if m.Account != "" {
		body = appendProtoString(body, 2, m.Account)
	}
	if tags := strings.Join(m.Tags, ","); tags != "" {
		body = appendProtoString(body, 3, tags)
	}
	return body, nil
}

func (CancelAccountSummary) protobufVersion() int {
	return protocol.MinServerVersionProtobufAccountsPositions
}

func (m CancelAccountSummary) encodeProto(sv int) ([]byte, error) {
	if err := requireAccountProto(sv, "cancel account summary"); err != nil {
		return nil, err
	}
	return appendProtoInt(nil, 1, m.ReqID, "cancel account summary request id")
}

func (PositionsMultiRequest) protobufVersion() int {
	return protocol.MinServerVersionProtobufAccountsPositions
}

func (m PositionsMultiRequest) encodeProto(sv int) ([]byte, error) {
	if err := requireAccountProto(sv, "positions multi"); err != nil {
		return nil, err
	}
	body, err := appendProtoInt(nil, 1, m.ReqID, "positions multi request id")
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

func (CancelPositionsMulti) protobufVersion() int {
	return protocol.MinServerVersionProtobufAccountsPositions
}

func (m CancelPositionsMulti) encodeProto(sv int) ([]byte, error) {
	if err := requireAccountProto(sv, "cancel positions multi"); err != nil {
		return nil, err
	}
	return appendProtoInt(nil, 1, m.ReqID, "cancel positions multi request id")
}

func (AccountUpdatesMultiRequest) protobufVersion() int {
	return protocol.MinServerVersionProtobufAccountsPositions
}

func (m AccountUpdatesMultiRequest) encodeProto(sv int) ([]byte, error) {
	if err := requireAccountProto(sv, "account updates multi"); err != nil {
		return nil, err
	}
	body, err := appendProtoInt(nil, 1, m.ReqID, "account updates multi request id")
	if err != nil {
		return nil, err
	}
	if m.Account != "" {
		body = appendProtoString(body, 2, m.Account)
	}
	if m.ModelCode != "" {
		body = appendProtoString(body, 3, m.ModelCode)
	}
	if m.LedgerAndNLV {
		body = appendProtoVarint(body, 4, 1)
	}
	return body, nil
}

func (CancelAccountUpdatesMulti) protobufVersion() int {
	return protocol.MinServerVersionProtobufAccountsPositions
}

func (m CancelAccountUpdatesMulti) encodeProto(sv int) ([]byte, error) {
	if err := requireAccountProto(sv, "cancel account updates multi"); err != nil {
		return nil, err
	}
	return appendProtoInt(nil, 1, m.ReqID, "cancel account updates multi request id")
}

func decodeManagedAccountsProto(body []byte, sv int) ([]Message, error) {
	m := ManagedAccounts{Accounts: []string{}}
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
				return nil, protoFieldError("managed accounts", number, err)
			}
			if accounts := strings.TrimRight(string(value), ","); accounts != "" {
				m.Accounts = strings.Split(accounts, ",")
			}
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return nil, protoFieldError("managed accounts", number, err)
		}
	}
}

func decodePositionProto(body []byte, sv int) ([]Message, error) {
	m := Position{AvgCost: "0"}
	hasContract := false
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			if !hasContract {
				return nil, nil
			}
			return []Message{m}, nil
		}
		switch number {
		case 1, 3:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("position", number, err)
			}
			if number == 1 {
				m.Account = string(value)
			} else {
				m.Position = string(value)
			}
		case 2:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("position", number, err)
			}
			m.Contract, err = decodeContractProto(value)
			if err != nil {
				return nil, protoFieldError("position contract", number, err)
			}
			hasContract = true
		case 4:
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return nil, protoFieldError("position", number, err)
			}
			m.AvgCost = formatProtoDouble(value)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("position", number, err)
			}
		}
	}
}

func decodePositionEndProto(body []byte, sv int) ([]Message, error) {
	if err := skipAccountProtoFields(body, "position end"); err != nil {
		return nil, err
	}
	return []Message{PositionEnd{}}, nil
}

func decodeAccountSummaryProto(body []byte, sv int) ([]Message, error) {
	m := AccountSummaryValue{ReqID: -1}
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
				return nil, protoFieldError("account summary", number, err)
			}
			m.ReqID = decodeProtoInt32(value)
		case 2, 3, 4, 5:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("account summary", number, err)
			}
			switch number {
			case 2:
				m.Account = string(value)
			case 3:
				m.Tag = string(value)
			case 4:
				m.Value = string(value)
			case 5:
				m.Currency = string(value)
			}
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("account summary", number, err)
			}
		}
	}
}

func decodeAccountSummaryEndProto(body []byte, sv int) ([]Message, error) {
	m := AccountSummaryEnd{ReqID: -1}
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
				return nil, protoFieldError("account summary end", number, err)
			}
			m.ReqID = decodeProtoInt32(value)
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return nil, protoFieldError("account summary end", number, err)
		}
	}
}

func decodeUpdateAccountValueProto(body []byte, sv int) ([]Message, error) {
	m := UpdateAccountValue{}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		if number >= 1 && number <= 4 {
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("account value", number, err)
			}
			switch number {
			case 1:
				m.Key = string(value)
			case 2:
				m.Value = string(value)
			case 3:
				m.Currency = string(value)
			case 4:
				m.Account = string(value)
			}
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return nil, protoFieldError("account value", number, err)
		}
	}
}

func decodeUpdatePortfolioProto(body []byte, sv int) ([]Message, error) {
	m := UpdatePortfolio{
		MarketPrice: "0", MarketValue: "0", AvgCost: "0",
		UnrealizedPNL: "0", RealizedPNL: "0",
	}
	hasContract := false
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			if !hasContract {
				return nil, nil
			}
			return []Message{m}, nil
		}
		switch number {
		case 1:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("portfolio value", number, err)
			}
			m.Contract, err = decodeContractProto(value)
			if err != nil {
				return nil, protoFieldError("portfolio value contract", number, err)
			}
			hasContract = true
		case 2, 8:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("portfolio value", number, err)
			}
			if number == 2 {
				m.Position = string(value)
			} else {
				m.Account = string(value)
			}
		case 3, 4, 5, 6, 7:
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return nil, protoFieldError("portfolio value", number, err)
			}
			formatted := formatProtoDouble(value)
			switch number {
			case 3:
				m.MarketPrice = formatted
			case 4:
				m.MarketValue = formatted
			case 5:
				m.AvgCost = formatted
			case 6:
				m.UnrealizedPNL = formatted
			case 7:
				m.RealizedPNL = formatted
			}
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("portfolio value", number, err)
			}
		}
	}
}

func decodeUpdateAccountTimeProto(body []byte, sv int) ([]Message, error) {
	m := UpdateAccountTime{}
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
				return nil, protoFieldError("account update time", number, err)
			}
			m.Timestamp = string(value)
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return nil, protoFieldError("account update time", number, err)
		}
	}
}

func decodeAccountDownloadEndProto(body []byte, sv int) ([]Message, error) {
	m := AccountDownloadEnd{}
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
				return nil, protoFieldError("account download end", number, err)
			}
			m.Account = string(value)
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return nil, protoFieldError("account download end", number, err)
		}
	}
}

func decodePositionMultiProto(body []byte, sv int) ([]Message, error) {
	m := PositionMulti{ReqID: -1, AvgCost: "0"}
	hasContract := false
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			if !hasContract {
				return nil, nil
			}
			return []Message{m}, nil
		}
		switch number {
		case 1:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("position multi", number, err)
			}
			m.ReqID = decodeProtoInt32(value)
		case 2, 4, 6:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("position multi", number, err)
			}
			switch number {
			case 2:
				m.Account = string(value)
			case 4:
				m.Position = string(value)
			case 6:
				m.ModelCode = string(value)
			}
		case 3:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("position multi", number, err)
			}
			m.Contract, err = decodeContractProto(value)
			if err != nil {
				return nil, protoFieldError("position multi contract", number, err)
			}
			hasContract = true
		case 5:
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return nil, protoFieldError("position multi", number, err)
			}
			m.AvgCost = formatProtoDouble(value)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("position multi", number, err)
			}
		}
	}
}

func decodePositionMultiEndProto(body []byte, sv int) ([]Message, error) {
	m := PositionMultiEnd{ReqID: -1}
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
				return nil, protoFieldError("position multi end", number, err)
			}
			m.ReqID = decodeProtoInt32(value)
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return nil, protoFieldError("position multi end", number, err)
		}
	}
}

func decodeAccountUpdateMultiProto(body []byte, sv int) ([]Message, error) {
	m := AccountUpdateMultiValue{ReqID: -1}
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
				return nil, protoFieldError("account update multi", number, err)
			}
			m.ReqID = decodeProtoInt32(value)
		case 2, 3, 4, 5, 6:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("account update multi", number, err)
			}
			switch number {
			case 2:
				m.Account = string(value)
			case 3:
				m.ModelCode = string(value)
			case 4:
				m.Key = string(value)
			case 5:
				m.Value = string(value)
			case 6:
				m.Currency = string(value)
			}
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("account update multi", number, err)
			}
		}
	}
}

func decodeAccountUpdateMultiEndProto(body []byte, sv int) ([]Message, error) {
	m := AccountUpdateMultiEnd{ReqID: -1}
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
				return nil, protoFieldError("account update multi end", number, err)
			}
			m.ReqID = decodeProtoInt32(value)
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return nil, protoFieldError("account update multi end", number, err)
		}
	}
}

func skipAccountProtoFields(body []byte, label string) error {
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		if err := skipProtoField(&body, number, typ); err != nil {
			return protoFieldError(label, number, err)
		}
	}
}

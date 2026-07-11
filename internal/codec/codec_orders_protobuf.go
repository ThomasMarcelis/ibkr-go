package codec

import (
	"fmt"
	"math"

	"github.com/ThomasMarcelis/ibkr-go/internal/protocol"
	"google.golang.org/protobuf/encoding/protowire"
)

func decodeOpenOrdersEndProto(body []byte, sv int) ([]Message, error) {
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{OpenOrderEnd{}}, nil
		}
		if err := skipProtoField(&body, number, typ); err != nil {
			return nil, protoFieldError("open orders end", number, err)
		}
	}
}

func (m ExecutionsRequest) protobufVersion() int {
	return protocol.MinServerVersionProtobuf
}

func (m ExecutionsRequest) encodeProto(sv int) ([]byte, error) {
	if sv < m.protobufVersion() {
		return nil, fmt.Errorf("codec: executions protobuf requires server_version %d", m.protobufVersion())
	}

	reqID, err := encodeProtoInt32(m.ReqID, "executions request id")
	if err != nil {
		return nil, err
	}
	body := appendProtoVarint(nil, 1, reqID)
	filter := make([]byte, 0, 64)
	if m.ClientID != 0 {
		clientID, err := encodeProtoInt32(m.ClientID, "executions client id")
		if err != nil {
			return nil, err
		}
		filter = appendProtoVarint(filter, 1, clientID)
	}
	if m.Account != "" {
		filter = appendProtoString(filter, 2, m.Account)
	}
	if m.Time != "" {
		filter = appendProtoString(filter, 3, m.Time)
	}
	if m.Symbol != "" {
		filter = appendProtoString(filter, 4, m.Symbol)
	}
	if m.SecType != "" {
		filter = appendProtoString(filter, 5, m.SecType)
	}
	if m.Exchange != "" {
		filter = appendProtoString(filter, 6, m.Exchange)
	}
	if m.Side != "" {
		filter = appendProtoString(filter, 7, m.Side)
	}
	if m.LastNDays != nil {
		lastNDays, err := encodeProtoInt32(*m.LastNDays, "executions last n days")
		if err != nil {
			return nil, err
		}
		filter = appendProtoVarint(filter, 8, lastNDays)
	}
	if len(m.SpecificDates) > 0 {
		packed := make([]byte, 0, len(m.SpecificDates)*4)
		for _, date := range m.SpecificDates {
			encoded, err := encodeProtoInt32(date, "executions specific date")
			if err != nil {
				return nil, err
			}
			packed = protowire.AppendVarint(packed, encoded)
		}
		filter = appendProtoMessage(filter, 9, packed)
	}
	// The live sv201 request contains an explicitly present, empty filter.
	// Presence is semantically different from omitting this optional message.
	return appendProtoMessage(body, 2, filter), nil
}

func decodeExecutionDetailsProto(body []byte, sv int) ([]Message, error) {
	m := ExecutionDetail{}
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
				return nil, protoFieldError("execution details", number, err)
			}
			m.ReqID = decodeProtoInt32(value)
		case 2:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("execution details", number, err)
			}
			m.Contract, err = decodeContractProto(value)
			if err != nil {
				return nil, protoFieldError("execution details contract", number, err)
			}
		case 3:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("execution details", number, err)
			}
			if err := decodeExecutionProto(value, &m); err != nil {
				return nil, protoFieldError("execution details execution", number, err)
			}
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("execution details", number, err)
			}
		}
	}
}

func decodeContractProto(body []byte) (Contract, error) {
	decoded, err := decodeSharedContractProto(body)
	return decoded.Contract, err
}

func decodeExecutionProto(body []byte, m *ExecutionDetail) error {
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		switch number {
		case 1, 9, 10, 11, 18, 19, 21:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return protoFieldError("execution", number, err)
			}
			switch number {
			case 1:
				m.OrderID = int64(decodeProtoInt32(value))
			case 9:
				m.PermID = i64toa(decodeProtoInt64(value))
			case 10:
				m.ClientID = itoa(decodeProtoInt32(value))
			case 11:
				m.Liquidation = protoBoolString(value)
			case 18:
				m.LastLiquidity = itoa(decodeProtoInt32(value))
			case 19:
				m.PendingPriceRevision = protoBoolString(value)
			case 21:
				m.OptExerciseOrLapseType = itoa(decodeProtoInt32(value))
			}
		case 2, 3, 4, 5, 6, 7, 12, 14, 15, 17, 20:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return protoFieldError("execution", number, err)
			}
			switch number {
			case 2:
				m.ExecID = string(value)
			case 3:
				m.Time = string(value)
			case 4:
				m.Account = string(value)
			case 5:
				m.Exchange = string(value)
			case 6:
				m.Side = string(value)
			case 7:
				m.Shares = string(value)
			case 12:
				m.CumulativeQuantity = string(value)
			case 14:
				m.OrderRef = string(value)
			case 15:
				m.EconomicValueRule = string(value)
			case 17:
				m.ModelCode = string(value)
			case 20:
				m.Submitter = string(value)
			}
		case 8, 13, 16:
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return protoFieldError("execution", number, err)
			}
			formatted := formatProtoDouble(value)
			switch number {
			case 8:
				m.Price = formatted
			case 13:
				m.AveragePrice = formatted
			case 16:
				m.EconomicValueMultiplier = formatted
			}
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return protoFieldError("execution", number, err)
			}
		}
	}
}

func decodeExecutionDetailsEndProto(body []byte, sv int) ([]Message, error) {
	m := ExecutionsEnd{}
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
				return nil, protoFieldError("execution details end", number, err)
			}
			m.ReqID = decodeProtoInt32(value)
			continue
		}
		if err := skipProtoField(&body, number, typ); err != nil {
			return nil, protoFieldError("execution details end", number, err)
		}
	}
}

func decodeCommissionAndFeesReportProto(body []byte, sv int) ([]Message, error) {
	m := CommissionReport{}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		switch number {
		case 1, 3, 6:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("commission and fees report", number, err)
			}
			switch number {
			case 1:
				m.ExecID = string(value)
			case 3:
				m.Currency = string(value)
			case 6:
				m.YieldRedemptionDate = string(value)
			}
		case 2, 4, 5:
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return nil, protoFieldError("commission and fees report", number, err)
			}
			formatted := formatProtoDouble(value)
			switch number {
			case 2:
				m.Commission = formatted
			case 4:
				m.RealizedPNL = formatted
			case 5:
				m.Yield = formatted
			}
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("commission and fees report", number, err)
			}
		}
	}
}

func decodeErrorProto(body []byte, sv int) ([]Message, error) {
	m := APIError{}
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
				return nil, protoFieldError("error", number, err)
			}
			switch number {
			case 1:
				m.ReqID = decodeProtoInt32(value)
			case 2:
				m.ErrorTimeMs = i64toa(decodeProtoInt64(value))
			case 3:
				m.Code = decodeProtoInt32(value)
			}
		case 4, 5:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("error", number, err)
			}
			if number == 4 {
				m.Message = string(value)
			} else {
				m.AdvancedOrderRejectJSON = string(value)
			}
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("error", number, err)
			}
		}
	}
}

func protoBoolString(value uint64) string {
	if value == 0 {
		return "0"
	}
	return "1"
}

func encodeProtoInt32(value int, label string) (uint64, error) {
	if value < math.MinInt32 || value > math.MaxInt32 {
		return 0, fmt.Errorf("codec: %s %d exceeds protobuf int32", label, value)
	}
	// Protobuf int32 uses a sign-extended varint for negative values. The
	// bounds check above proves the narrowing conversion is exact.
	return uint64(int64(int32(value))), nil // #nosec G115 -- checked protobuf int32 conversion
}

func decodeProtoInt32(value uint64) int {
	// Protobuf parsers recover int32 by retaining the low 32 bits; negative
	// values arrive sign-extended as ten-byte varints.
	return int(int32(value)) // #nosec G115 -- protobuf int32 wire semantics
}

func decodeProtoInt64(value uint64) int64 {
	// Varints carry the two's-complement bit pattern for protobuf int64.
	return int64(value) // #nosec G115 -- protobuf int64 wire semantics
}

func protoFieldError(message string, number protowire.Number, err error) error {
	return fmt.Errorf("%s field %d: %w", message, number, err)
}

package codec

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

func Encode(msg Message) ([]byte, error) {
	fields, err := encodeFields(msg)
	if err != nil {
		return nil, err
	}
	return wire.EncodeFields(fields), nil
}

func Decode(payload []byte) (Message, error) {
	fields, err := wire.ParseFields(payload)
	if err != nil {
		return nil, err
	}
	return decodeFields(fields)
}

func encodeFields(msg Message) ([]string, error) {
	switch m := msg.(type) {
	case Hello:
		return []string{m.messageName(), itoa(m.MinVersion), itoa(m.MaxVersion), itoa(m.ClientID)}, nil
	case HelloAck:
		return []string{m.messageName(), itoa(m.ServerVersion), m.ConnectionTime}, nil
	case ManagedAccounts:
		return []string{m.messageName(), strings.Join(m.Accounts, ",")}, nil
	case NextValidID:
		return []string{m.messageName(), i64toa(m.OrderID)}, nil
	case CurrentTime:
		return []string{m.messageName(), m.Time}, nil
	case APIError:
		return []string{m.messageName(), itoa(m.ReqID), itoa(m.Code), m.Message}, nil
	case ContractDetailsRequest:
		return append([]string{m.messageName(), itoa(m.ReqID)}, contractFields(m.Contract)...), nil
	case ContractDetails:
		fields := append([]string{m.messageName(), itoa(m.ReqID)}, contractFields(m.Contract)...)
		fields = append(fields, m.MarketName, m.MinTick, m.TimeZoneID)
		return fields, nil
	case ContractDetailsEnd:
		return []string{m.messageName(), itoa(m.ReqID)}, nil
	case HistoricalBarsRequest:
		fields := append([]string{m.messageName(), itoa(m.ReqID)}, contractFields(m.Contract)...)
		fields = append(fields, m.EndDateTime, m.Duration, m.BarSize, m.WhatToShow, boolString(m.UseRTH))
		return fields, nil
	case HistoricalBar:
		return []string{m.messageName(), itoa(m.ReqID), m.Time, m.Open, m.High, m.Low, m.Close, m.Volume}, nil
	case HistoricalBarsEnd:
		return []string{m.messageName(), itoa(m.ReqID), m.Start, m.End}, nil
	case AccountSummaryRequest:
		return []string{m.messageName(), itoa(m.ReqID), m.Account, strings.Join(m.Tags, ",")}, nil
	case CancelAccountSummary:
		return []string{m.messageName(), itoa(m.ReqID)}, nil
	case AccountSummaryValue:
		return []string{m.messageName(), itoa(m.ReqID), m.Account, m.Tag, m.Value, m.Currency}, nil
	case AccountSummaryEnd:
		return []string{m.messageName(), itoa(m.ReqID)}, nil
	case PositionsRequest:
		return []string{m.messageName()}, nil
	case CancelPositions:
		return []string{m.messageName()}, nil
	case Position:
		fields := append([]string{m.messageName(), m.Account}, contractFields(m.Contract)...)
		fields = append(fields, m.Position, m.AvgCost)
		return fields, nil
	case PositionEnd:
		return []string{m.messageName()}, nil
	case QuoteRequest:
		fields := append([]string{m.messageName(), itoa(m.ReqID)}, contractFields(m.Contract)...)
		fields = append(fields, boolString(m.Snapshot), strings.Join(m.GenericTicks, ","))
		return fields, nil
	case CancelQuote:
		return []string{m.messageName(), itoa(m.ReqID)}, nil
	case TickPrice:
		return []string{m.messageName(), itoa(m.ReqID), m.Field, m.Price}, nil
	case TickSize:
		return []string{m.messageName(), itoa(m.ReqID), m.Field, m.Size}, nil
	case MarketDataType:
		return []string{m.messageName(), itoa(m.ReqID), itoa(m.DataType)}, nil
	case TickSnapshotEnd:
		return []string{m.messageName(), itoa(m.ReqID)}, nil
	case RealTimeBarsRequest:
		fields := append([]string{m.messageName(), itoa(m.ReqID)}, contractFields(m.Contract)...)
		fields = append(fields, m.WhatToShow, boolString(m.UseRTH))
		return fields, nil
	case CancelRealTimeBars:
		return []string{m.messageName(), itoa(m.ReqID)}, nil
	case RealTimeBar:
		return []string{m.messageName(), itoa(m.ReqID), m.Time, m.Open, m.High, m.Low, m.Close, m.Volume}, nil
	case OpenOrdersRequest:
		return []string{m.messageName(), m.Scope}, nil
	case CancelOpenOrders:
		return []string{m.messageName()}, nil
	case OpenOrder:
		fields := append([]string{m.messageName(), i64toa(m.OrderID), m.Account}, contractFields(m.Contract)...)
		fields = append(fields, m.Status, m.Quantity, m.Filled, m.Remaining)
		return fields, nil
	case OpenOrderEnd:
		return []string{m.messageName()}, nil
	case ExecutionsRequest:
		return []string{m.messageName(), itoa(m.ReqID), m.Account, m.Symbol}, nil
	case ExecutionDetail:
		return []string{m.messageName(), itoa(m.ReqID), m.ExecID, m.Account, m.Symbol, m.Side, m.Shares, m.Price, m.Time}, nil
	case ExecutionsEnd:
		return []string{m.messageName(), itoa(m.ReqID)}, nil
	case CommissionReport:
		return []string{m.messageName(), m.ExecID, m.Commission, m.Currency, m.RealizedPNL}, nil
	default:
		return nil, fmt.Errorf("codec: unsupported message type %T", msg)
	}
}

func decodeFields(fields []string) (Message, error) {
	switch fields[0] {
	case "hello":
		if err := wantLen(fields, 4); err != nil {
			return nil, err
		}
		minVersion, err := atoi(fields[1])
		if err != nil {
			return nil, err
		}
		maxVersion, err := atoi(fields[2])
		if err != nil {
			return nil, err
		}
		clientID, err := atoi(fields[3])
		if err != nil {
			return nil, err
		}
		return Hello{MinVersion: minVersion, MaxVersion: maxVersion, ClientID: clientID}, nil
	case "hello_ack":
		if err := wantLen(fields, 3); err != nil {
			return nil, err
		}
		serverVersion, err := atoi(fields[1])
		if err != nil {
			return nil, err
		}
		return HelloAck{ServerVersion: serverVersion, ConnectionTime: fields[2]}, nil
	case "managed_accounts":
		if err := wantLen(fields, 2); err != nil {
			return nil, err
		}
		accounts := []string{}
		if fields[1] != "" {
			accounts = strings.Split(fields[1], ",")
		}
		return ManagedAccounts{Accounts: accounts}, nil
	case "next_valid_id":
		if err := wantLen(fields, 2); err != nil {
			return nil, err
		}
		orderID, err := atoi64(fields[1])
		if err != nil {
			return nil, err
		}
		return NextValidID{OrderID: orderID}, nil
	case "current_time":
		if err := wantLen(fields, 2); err != nil {
			return nil, err
		}
		return CurrentTime{Time: fields[1]}, nil
	case "api_error":
		if err := wantLen(fields, 4); err != nil {
			return nil, err
		}
		reqID, err := atoi(fields[1])
		if err != nil {
			return nil, err
		}
		code, err := atoi(fields[2])
		if err != nil {
			return nil, err
		}
		return APIError{ReqID: reqID, Code: code, Message: fields[3]}, nil
	case "req_contract_details":
		reqID, contract, err := parseReqContract(fields)
		if err != nil {
			return nil, err
		}
		return ContractDetailsRequest{ReqID: reqID, Contract: contract}, nil
	case "contract_details":
		if err := wantLen(fields, 11); err != nil {
			return nil, err
		}
		reqID, err := atoi(fields[1])
		if err != nil {
			return nil, err
		}
		contract, _, err := parseContract(fields, 2)
		if err != nil {
			return nil, err
		}
		return ContractDetails{
			ReqID:      reqID,
			Contract:   contract,
			MarketName: fields[8],
			MinTick:    fields[9],
			TimeZoneID: fields[10],
		}, nil
	case "contract_details_end":
		reqID, err := parseSingleReqID(fields)
		if err != nil {
			return nil, err
		}
		return ContractDetailsEnd{ReqID: reqID}, nil
	case "req_historical_bars":
		if err := wantLen(fields, 13); err != nil {
			return nil, err
		}
		reqID, err := atoi(fields[1])
		if err != nil {
			return nil, err
		}
		contract, _, err := parseContract(fields, 2)
		if err != nil {
			return nil, err
		}
		useRTH, err := parseBool(fields[12])
		if err != nil {
			return nil, err
		}
		return HistoricalBarsRequest{
			ReqID:       reqID,
			Contract:    contract,
			EndDateTime: fields[8],
			Duration:    fields[9],
			BarSize:     fields[10],
			WhatToShow:  fields[11],
			UseRTH:      useRTH,
		}, nil
	case "historical_bar":
		if err := wantLen(fields, 8); err != nil {
			return nil, err
		}
		reqID, err := atoi(fields[1])
		if err != nil {
			return nil, err
		}
		return HistoricalBar{ReqID: reqID, Time: fields[2], Open: fields[3], High: fields[4], Low: fields[5], Close: fields[6], Volume: fields[7]}, nil
	case "historical_bars_end":
		if err := wantLen(fields, 4); err != nil {
			return nil, err
		}
		reqID, err := atoi(fields[1])
		if err != nil {
			return nil, err
		}
		return HistoricalBarsEnd{ReqID: reqID, Start: fields[2], End: fields[3]}, nil
	case "req_account_summary":
		if err := wantLen(fields, 4); err != nil {
			return nil, err
		}
		reqID, err := atoi(fields[1])
		if err != nil {
			return nil, err
		}
		tags := []string{}
		if fields[3] != "" {
			tags = strings.Split(fields[3], ",")
		}
		return AccountSummaryRequest{ReqID: reqID, Account: fields[2], Tags: tags}, nil
	case "cancel_account_summary":
		reqID, err := parseSingleReqID(fields)
		if err != nil {
			return nil, err
		}
		return CancelAccountSummary{ReqID: reqID}, nil
	case "account_summary":
		if err := wantLen(fields, 6); err != nil {
			return nil, err
		}
		reqID, err := atoi(fields[1])
		if err != nil {
			return nil, err
		}
		return AccountSummaryValue{ReqID: reqID, Account: fields[2], Tag: fields[3], Value: fields[4], Currency: fields[5]}, nil
	case "account_summary_end":
		reqID, err := parseSingleReqID(fields)
		if err != nil {
			return nil, err
		}
		return AccountSummaryEnd{ReqID: reqID}, nil
	case "req_positions":
		if err := wantLen(fields, 1); err != nil {
			return nil, err
		}
		return PositionsRequest{}, nil
	case "cancel_positions":
		if err := wantLen(fields, 1); err != nil {
			return nil, err
		}
		return CancelPositions{}, nil
	case "position":
		if err := wantLen(fields, 10); err != nil {
			return nil, err
		}
		contract, _, err := parseContract(fields, 2)
		if err != nil {
			return nil, err
		}
		return Position{Account: fields[1], Contract: contract, Position: fields[8], AvgCost: fields[9]}, nil
	case "position_end":
		if err := wantLen(fields, 1); err != nil {
			return nil, err
		}
		return PositionEnd{}, nil
	case "req_quote":
		if err := wantLen(fields, 10); err != nil {
			return nil, err
		}
		reqID, err := atoi(fields[1])
		if err != nil {
			return nil, err
		}
		contract, _, err := parseContract(fields, 2)
		if err != nil {
			return nil, err
		}
		snapshot, err := parseBool(fields[8])
		if err != nil {
			return nil, err
		}
		genericTicks := []string{}
		if fields[9] != "" {
			genericTicks = strings.Split(fields[9], ",")
		}
		return QuoteRequest{ReqID: reqID, Contract: contract, Snapshot: snapshot, GenericTicks: genericTicks}, nil
	case "cancel_quote":
		reqID, err := parseSingleReqID(fields)
		if err != nil {
			return nil, err
		}
		return CancelQuote{ReqID: reqID}, nil
	case "tick_price":
		if err := wantLen(fields, 4); err != nil {
			return nil, err
		}
		reqID, err := atoi(fields[1])
		if err != nil {
			return nil, err
		}
		return TickPrice{ReqID: reqID, Field: fields[2], Price: fields[3]}, nil
	case "tick_size":
		if err := wantLen(fields, 4); err != nil {
			return nil, err
		}
		reqID, err := atoi(fields[1])
		if err != nil {
			return nil, err
		}
		return TickSize{ReqID: reqID, Field: fields[2], Size: fields[3]}, nil
	case "market_data_type":
		if err := wantLen(fields, 3); err != nil {
			return nil, err
		}
		reqID, err := atoi(fields[1])
		if err != nil {
			return nil, err
		}
		dataType, err := atoi(fields[2])
		if err != nil {
			return nil, err
		}
		return MarketDataType{ReqID: reqID, DataType: dataType}, nil
	case "tick_snapshot_end":
		reqID, err := parseSingleReqID(fields)
		if err != nil {
			return nil, err
		}
		return TickSnapshotEnd{ReqID: reqID}, nil
	case "req_realtime_bars":
		if err := wantLen(fields, 10); err != nil {
			return nil, err
		}
		reqID, err := atoi(fields[1])
		if err != nil {
			return nil, err
		}
		contract, _, err := parseContract(fields, 2)
		if err != nil {
			return nil, err
		}
		useRTH, err := parseBool(fields[9])
		if err != nil {
			return nil, err
		}
		return RealTimeBarsRequest{ReqID: reqID, Contract: contract, WhatToShow: fields[8], UseRTH: useRTH}, nil
	case "cancel_realtime_bars":
		reqID, err := parseSingleReqID(fields)
		if err != nil {
			return nil, err
		}
		return CancelRealTimeBars{ReqID: reqID}, nil
	case "realtime_bar":
		if err := wantLen(fields, 8); err != nil {
			return nil, err
		}
		reqID, err := atoi(fields[1])
		if err != nil {
			return nil, err
		}
		return RealTimeBar{ReqID: reqID, Time: fields[2], Open: fields[3], High: fields[4], Low: fields[5], Close: fields[6], Volume: fields[7]}, nil
	case "req_open_orders":
		if err := wantLen(fields, 2); err != nil {
			return nil, err
		}
		return OpenOrdersRequest{Scope: fields[1]}, nil
	case "cancel_open_orders":
		if err := wantLen(fields, 1); err != nil {
			return nil, err
		}
		return CancelOpenOrders{}, nil
	case "open_order":
		if err := wantLen(fields, 13); err != nil {
			return nil, err
		}
		orderID, err := atoi64(fields[1])
		if err != nil {
			return nil, err
		}
		contract, _, err := parseContract(fields, 3)
		if err != nil {
			return nil, err
		}
		return OpenOrder{
			OrderID:   orderID,
			Account:   fields[2],
			Contract:  contract,
			Status:    fields[9],
			Quantity:  fields[10],
			Filled:    fields[11],
			Remaining: fields[12],
		}, nil
	case "open_order_end":
		if err := wantLen(fields, 1); err != nil {
			return nil, err
		}
		return OpenOrderEnd{}, nil
	case "req_executions":
		if err := wantLen(fields, 4); err != nil {
			return nil, err
		}
		reqID, err := atoi(fields[1])
		if err != nil {
			return nil, err
		}
		return ExecutionsRequest{ReqID: reqID, Account: fields[2], Symbol: fields[3]}, nil
	case "execution_detail":
		if err := wantLen(fields, 9); err != nil {
			return nil, err
		}
		reqID, err := atoi(fields[1])
		if err != nil {
			return nil, err
		}
		return ExecutionDetail{
			ReqID:   reqID,
			ExecID:  fields[2],
			Account: fields[3],
			Symbol:  fields[4],
			Side:    fields[5],
			Shares:  fields[6],
			Price:   fields[7],
			Time:    fields[8],
		}, nil
	case "executions_end":
		reqID, err := parseSingleReqID(fields)
		if err != nil {
			return nil, err
		}
		return ExecutionsEnd{ReqID: reqID}, nil
	case "commission_report":
		if err := wantLen(fields, 5); err != nil {
			return nil, err
		}
		return CommissionReport{ExecID: fields[1], Commission: fields[2], Currency: fields[3], RealizedPNL: fields[4]}, nil
	default:
		return nil, fmt.Errorf("codec: unknown message %q", fields[0])
	}
}

func contractFields(c Contract) []string {
	return []string{
		c.Symbol,
		c.SecType,
		c.Exchange,
		c.Currency,
		c.PrimaryExchange,
		c.LocalSymbol,
	}
}

func parseReqContract(fields []string) (int, Contract, error) {
	if err := wantLen(fields, 8); err != nil {
		return 0, Contract{}, err
	}
	reqID, err := atoi(fields[1])
	if err != nil {
		return 0, Contract{}, err
	}
	contract, _, err := parseContract(fields, 2)
	return reqID, contract, err
}

func parseContract(fields []string, idx int) (Contract, int, error) {
	if idx+6 > len(fields) {
		return Contract{}, idx, fmt.Errorf("codec: contract fields truncated")
	}
	return Contract{
		Symbol:          fields[idx],
		SecType:         fields[idx+1],
		Exchange:        fields[idx+2],
		Currency:        fields[idx+3],
		PrimaryExchange: fields[idx+4],
		LocalSymbol:     fields[idx+5],
	}, idx + 6, nil
}

func parseSingleReqID(fields []string) (int, error) {
	if err := wantLen(fields, 2); err != nil {
		return 0, err
	}
	return atoi(fields[1])
}

func wantLen(fields []string, want int) error {
	if len(fields) != want {
		return fmt.Errorf("codec: field count = %d, want %d for %q", len(fields), want, fields[0])
	}
	return nil
}

func atoi(v string) (int, error) {
	i, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("codec: parse int %q: %w", v, err)
	}
	return i, nil
}

func atoi64(v string) (int64, error) {
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("codec: parse int64 %q: %w", v, err)
	}
	return i, nil
}

func itoa(v int) string {
	return strconv.Itoa(v)
}

func i64toa(v int64) string {
	return strconv.FormatInt(v, 10)
}

func boolString(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func parseBool(v string) (bool, error) {
	switch v {
	case "1", "true":
		return true, nil
	case "0", "false":
		return false, nil
	default:
		return false, fmt.Errorf("codec: parse bool %q", v)
	}
}

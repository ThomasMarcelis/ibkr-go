package codec

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

// EncodeHandshakePrefix returns the raw API prefix bytes sent before framing begins.
func EncodeHandshakePrefix() []byte {
	return []byte("API\x00")
}

// EncodeVersionRange returns the version negotiation payload (to be length-framed by caller).
func EncodeVersionRange(minVer, maxVer int) []byte {
	return []byte(fmt.Sprintf("v%d..%d", minVer, maxVer))
}

// DecodeServerInfo parses the server info frame returned during the handshake.
func DecodeServerInfo(payload []byte) (ServerInfo, error) {
	fields, err := wire.ParseFields(payload)
	if err != nil {
		return ServerInfo{}, err
	}
	if len(fields) < 2 {
		return ServerInfo{}, fmt.Errorf("codec: server info: want >= 2 fields, got %d", len(fields))
	}
	version, err := strconv.Atoi(fields[0])
	if err != nil {
		return ServerInfo{}, fmt.Errorf("codec: server info: parse version %q: %w", fields[0], err)
	}
	return ServerInfo{ServerVersion: version, ConnectionTime: fields[1]}, nil
}

// DecodeBatch decodes a framed payload into one or more messages keyed by integer msg_id.
func DecodeBatch(payload []byte) ([]Message, error) {
	fields, err := wire.ParseFields(payload)
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("codec: empty message")
	}
	msgID, err := strconv.Atoi(fields[0])
	if err != nil {
		return nil, fmt.Errorf("codec: parse msg_id %q: %w", fields[0], err)
	}
	msgs, err := decodeByMsgID(msgID, fields)
	if err != nil {
		return nil, fmt.Errorf("codec: msg_id %d: %w", msgID, err)
	}
	return msgs, nil
}

// Decode decodes a framed payload into exactly one message.
func Decode(payload []byte) (Message, error) {
	msgs, err := DecodeBatch(payload)
	if err != nil {
		return nil, err
	}
	if len(msgs) != 1 {
		return nil, fmt.Errorf("codec: expected 1 message, got %d", len(msgs))
	}
	return msgs[0], nil
}

func isWireInt(value string) bool {
	if value == "" {
		return false
	}
	_, err := strconv.Atoi(value)
	return err == nil
}

func isHistoricalRangeBoundary(value string) bool {
	if value == "" || isWireInt(value) {
		return false
	}
	return strings.Contains(value, " ") && strings.Contains(value, "/")
}

func completedOrderTail(fields []string, statusIndex int, orderType string) (string, bool) {
	r := newFieldReader(fields[statusIndex+1:])
	if err := skipCompletedOrderPostStatusPrefix(r, orderType); err != nil {
		return "", false
	}
	filled := r.ReadString()
	if r.Err() != nil || !isNonNegativeWireNumber(filled) {
		return "", false
	}
	if r.Remaining() < 8 {
		return "", false
	}
	r.Skip(7)      // refFuturesConId through completedTime
	r.ReadString() // completedStatus
	if r.Err() != nil || r.Remaining() > 8 {
		return "", false
	}
	return filled, true
}

func completedOrderStatusTail(fields []string, orderType string) (string, string, error) {
	for i := 15; i < len(fields); i++ {
		if !isOrderStatusField(fields[i]) {
			continue
		}
		filled, ok := completedOrderTail(fields, i, orderType)
		if ok {
			return fields[i], filled, nil
		}
	}
	return "", "", fmt.Errorf("codec: completed order status tail not found")
}

func isOrderStatusField(value string) bool {
	switch value {
	case "PendingSubmit", "PendingCancel", "PreSubmitted", "Submitted",
		"ApiPending", "ApiCancelled", "Cancelled", "Filled", "Inactive":
		return true
	default:
		return false
	}
}

func skipCompletedOrderPostStatusPrefix(r *fieldReader, orderType string) error {
	r.Skip(2) // randomizeSize, randomizePrice
	if orderType == "PEG BENCH" {
		r.Skip(5)
	}
	conditionsCount, err := r.ReadOptionalCount("completed order conditions")
	if err != nil {
		return err
	}
	if conditionsCount > 0 {
		for range conditionsCount {
			conditionType, err := r.ReadInt()
			if err != nil {
				return err
			}
			if _, err := readOrderCondition(r, conditionType); err != nil {
				return err
			}
		}
		r.Skip(2) // conditionsIgnoreRTH, conditionsCancelOrder
	}
	r.Skip(2) // stop price, limit price offset
	r.Skip(4) // cashQty, dontUseAutoPriceForHedge, isOmsContainer, autoCancelDate
	return nil
}

// Encode encodes a message in the real TWS wire format (integer msg_id prefix).
func Encode(msg Message) ([]byte, error) {
	fields, err := msg.encodeWire()
	if err != nil {
		return nil, err
	}
	return wire.EncodeFields(fields), nil
}

type decodeFunc func(r *fieldReader) ([]Message, error)

// inboundDecoders maps msg_id to its decoder. One explicit table, no
// init() registration.
var inboundDecoders = map[int]decodeFunc{
	InTickPrice:             decodeTickPrice,
	InTickSize:              decodeTickSize,
	InOrderStatus:           decodeOrderStatus,
	InErrMsg:                decodeErrMsg,
	InOpenOrder:             decodeOpenOrder,
	InCurrentTimeInMillis:   decodeCurrentTimeInMillis,
	InNextValidID:           decodeNextValidID,
	InContractData:          decodeContractData,
	InExecutionData:         decodeExecutionData,
	InMarketDepth:           decodeMarketDepth,
	InMarketDepthL2:         decodeMarketDepthL2,
	InManagedAccounts:       decodeManagedAccounts,
	InHistoricalData:        decodeHistoricalData,
	InScannerParameters:     decodeScannerParameters,
	InScannerData:           decodeScannerData,
	InTickOptionComputation: decodeTickOptionComputation,
	InTickGeneric:           decodeTickGeneric,
	InTickString:            decodeTickString,
	InTickReqParams:         decodeTickReqParams,
	InCurrentTime:           decodeCurrentTime,
	InRealTimeBars:          decodeRealTimeBars,
	InFundamentalData:       decodeFundamentalData,
	InContractDataEnd:       decodeContractDataEnd,
	InOpenOrderEnd:          decodeOpenOrderEnd,
	InExecutionDataEnd:      decodeExecutionDataEnd,
	InTickSnapshotEnd:       decodeTickSnapshotEnd,
	InMarketDataType:        decodeMarketDataType,
	InCommissionReport:      decodeCommissionReport,
	InPositionData:          decodePositionData,
	InPositionEnd:           decodePositionEnd,
	InAccountSummary:        decodeAccountSummary,
	InAccountSummaryEnd:     decodeAccountSummaryEnd,
	InSecDefOptParams:       decodeSecDefOptParams,
	InSecDefOptParamsEnd:    decodeSecDefOptParamsEnd,
	InFamilyCodes:           decodeFamilyCodes,
	InMktDepthExchanges:     decodeMktDepthExchanges,
	InNewsArticle:           decodeNewsArticle,
	InNewsProviders:         decodeNewsProviders,
	InSymbolSamples:         decodeSymbolSamples,
	InSmartComponents:       decodeSmartComponents,
	InHistoricalNews:        decodeHistoricalNews,
	InHistoricalNewsEnd:     decodeHistoricalNewsEnd,
	InHeadTimestamp:         decodeHeadTimestamp,
	InHistogramData:         decodeHistogramData,
	InMarketRule:            decodeMarketRule,
	InCompletedOrder:        decodeCompletedOrder,
	InCompletedOrderEnd:     decodeCompletedOrderEnd,
	InUserInfo:              decodeUserInfo,
	InUpdateAccountValue:    decodeUpdateAccountValue,
	InUpdatePortfolio:       decodeUpdatePortfolio,
	InUpdateAccountTime:     decodeUpdateAccountTime,
	InAccountDownloadEnd:    decodeAccountDownloadEnd,
	InNewsBulletins:         decodeNewsBulletins,
	InPositionMulti:         decodePositionMulti,
	InPositionMultiEnd:      decodePositionMultiEnd,
	InAccountUpdateMulti:    decodeAccountUpdateMulti,
	InAccountUpdateMultiEnd: decodeAccountUpdateMultiEnd,
	InPnL:                   decodePnL,
	InPnLSingle:             decodePnLSingle,
	InHistoricalTicks:       decodeHistoricalTicks,
	InHistoricalTicksBidAsk: decodeHistoricalTicksBidAsk,
	InHistoricalTicksLast:   decodeHistoricalTicksLast,
	InTickByTick:            decodeTickByTick,
	InHistoricalDataUpdate:  decodeHistoricalDataUpdate,
	InReceiveFA:             decodeReceiveFA,
	InSoftDollarTiers:       decodeSoftDollarTiers,
	InWSHMetaData:           decodeWSHMetaData,
	InWSHEventData:          decodeWSHEventData,
	InHistoricalSchedule:    decodeHistoricalSchedule,
	InDisplayGroupList:      decodeDisplayGroupList,
	InDisplayGroupUpdated:   decodeDisplayGroupUpdated,
}

// decodeByMsgID dispatches on the integer message ID and reads fields in real TWS wire layout.
// Returns []Message because historical data packs multiple bars into one frame.
func decodeByMsgID(msgID int, fields []string) ([]Message, error) {
	dec, ok := inboundDecoders[msgID]
	if !ok {
		return nil, fmt.Errorf("codec: unknown msg_id %d", msgID)
	}
	r := newFieldReader(fields[1:]) // skip msg_id
	msgs, err := dec(r)
	if err == nil && r.Err() != nil {
		return nil, r.Err()
	}
	return msgs, err
}

// writeWireContract writes the 11-field contract block (client->server):
// [symbol, secType, expiry, strike, right, multiplier, exchange, primaryExchange, currency, localSymbol, tradingClass]
func writeWireContract(w *fieldWriter, c Contract) {
	w.WriteString(c.Symbol)
	w.WriteString(c.SecType)
	w.WriteString(c.Expiry)
	if c.Strike == "" {
		w.WriteString("0")
	} else {
		w.WriteString(c.Strike)
	}
	w.WriteString(c.Right)
	w.WriteString(c.Multiplier)
	w.WriteString(c.Exchange)
	w.WriteString(c.PrimaryExchange)
	w.WriteString(c.Currency)
	w.WriteString(c.LocalSymbol)
	w.WriteString(c.TradingClass)
}

// readWireContract reads the 11-field contract block (server->client):
// [conID, symbol, secType, expiry, strike, right, multiplier, exchange, currency, localSymbol, tradingClass]
func readWireContract(r *fieldReader) Contract {
	conID, _ := r.ReadInt()
	symbol := r.ReadString()
	secType := r.ReadString()
	expiry := r.ReadString()
	strike := r.ReadString()
	right := r.ReadString()
	multiplier := r.ReadString()
	exchange := r.ReadString()
	currency := r.ReadString()
	localSymbol := r.ReadString()
	tradingClass := r.ReadString()
	return Contract{
		ConID: conID, Symbol: symbol, SecType: secType,
		Expiry: expiry, Strike: strike, Right: right,
		Multiplier: multiplier, Exchange: exchange, Currency: currency,
		LocalSymbol: localSymbol, TradingClass: tradingClass,
	}
}

func itoa(v int) string     { return strconv.Itoa(v) }
func i64toa(v int64) string { return strconv.FormatInt(v, 10) }

func btoa(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

// unsetDoubleSentinel is the wire rendering of the official UNSET_DOUBLE
// (DBL_MAX): an unset numeric slot, not a value.
const unsetDoubleSentinel = "1.7976931348623157E308"

func isPositiveWireNumber(raw string) bool {
	if raw == "" {
		return false
	}
	v, err := strconv.ParseFloat(raw, 64)
	return err == nil && v > 0
}

func isNonNegativeWireNumber(raw string) bool {
	if raw == "" {
		return false
	}
	v, err := strconv.ParseFloat(raw, 64)
	return err == nil && v >= 0
}

func mustReadInt(r *fieldReader) int {
	v, _ := r.ReadInt()
	return v
}

func mustReadBool(r *fieldReader) bool {
	v, _ := r.ReadBool()
	return v
}

func readTagValuePairs(r *fieldReader, label string, count int) ([]TagValue, error) {
	if err := r.RequireFixedEntryFields(label, count, 2, 0); err != nil {
		return nil, err
	}
	values := make([]TagValue, count)
	for i := range values {
		values[i] = TagValue{Tag: r.ReadString(), Value: r.ReadString()}
	}
	return values, nil
}

func writeTagValuePairs(w *fieldWriter, values []TagValue) {
	w.WriteInt(len(values))
	for _, value := range values {
		w.WriteString(value.Tag)
		w.WriteString(value.Value)
	}
}

func readOrderCondition(r *fieldReader, conditionType int) (OrderCondition, error) {
	cond := OrderCondition{Type: conditionType}
	switch conditionType {
	case 1: // Price
		cond.Conjunction = r.ReadString()
		if more, err := r.ReadBool(); err != nil {
			return OrderCondition{}, err
		} else if more {
			cond.Operator = 2
		} else {
			cond.Operator = 1
		}
		cond.Value = r.ReadString()
		cond.ConID, _ = r.ReadInt()
		cond.Exchange = r.ReadString()
		cond.TriggerMethod, _ = r.ReadInt()
	case 3: // Time
		cond.Conjunction = r.ReadString()
		if more, err := r.ReadBool(); err != nil {
			return OrderCondition{}, err
		} else if more {
			cond.Operator = 2
		} else {
			cond.Operator = 1
		}
		cond.Value = r.ReadString()
	case 4: // Margin
		cond.Conjunction = r.ReadString()
		if more, err := r.ReadBool(); err != nil {
			return OrderCondition{}, err
		} else if more {
			cond.Operator = 2
		} else {
			cond.Operator = 1
		}
		cond.Value = r.ReadString()
	case 5: // Execution
		cond.Conjunction = r.ReadString()
		cond.SecType = r.ReadString()
		cond.Exchange = r.ReadString()
		cond.Symbol = r.ReadString()
	case 6: // Volume
		cond.Conjunction = r.ReadString()
		if more, err := r.ReadBool(); err != nil {
			return OrderCondition{}, err
		} else if more {
			cond.Operator = 2
		} else {
			cond.Operator = 1
		}
		cond.Value = r.ReadString()
		cond.ConID, _ = r.ReadInt()
		cond.Exchange = r.ReadString()
	case 7: // Percent change
		cond.Conjunction = r.ReadString()
		if more, err := r.ReadBool(); err != nil {
			return OrderCondition{}, err
		} else if more {
			cond.Operator = 2
		} else {
			cond.Operator = 1
		}
		cond.Value = r.ReadString()
		cond.ConID, _ = r.ReadInt()
		cond.Exchange = r.ReadString()
	default:
		return OrderCondition{}, fmt.Errorf("codec: unsupported order condition type %d", conditionType)
	}
	return cond, nil
}

func writeOrderCondition(w *fieldWriter, cond OrderCondition) error {
	w.WriteInt(cond.Type)
	if cond.Conjunction == "o" {
		w.WriteString("o")
	} else {
		w.WriteString("a")
	}
	// Contract-bound conditions follow the official client's writeExternal
	// hierarchy: the OperatorCondition value precedes the ContractCondition
	// conId/exchange pair. Live Gateway (server_version 200) rejects the
	// reversed order with code 320 field-parse errors.
	isMore := cond.Operator == 2
	switch cond.Type {
	case 1:
		w.WriteBool(isMore)
		w.WriteString(cond.Value)
		w.WriteInt(cond.ConID)
		w.WriteString(cond.Exchange)
		w.WriteInt(cond.TriggerMethod)
	case 3, 4:
		w.WriteBool(isMore)
		w.WriteString(cond.Value)
	case 5:
		w.WriteString(cond.SecType)
		w.WriteString(cond.Exchange)
		w.WriteString(cond.Symbol)
	case 6, 7:
		w.WriteBool(isMore)
		w.WriteString(cond.Value)
		w.WriteInt(cond.ConID)
		w.WriteString(cond.Exchange)
	default:
		return fmt.Errorf("codec: unsupported order condition type %d", cond.Type)
	}
	return nil
}

func writeObservedWireContract(w *fieldWriter, c Contract) {
	w.WriteInt(c.ConID)
	w.WriteString(c.Symbol)
	w.WriteString(c.SecType)
	w.WriteString(c.Expiry)
	if c.Strike == "" {
		w.WriteString("0")
	} else {
		w.WriteString(c.Strike)
	}
	w.WriteString(c.Right)
	w.WriteString(c.Multiplier)
	w.WriteString(c.Exchange)
	w.WriteString(c.Currency)
	w.WriteString(c.LocalSymbol)
	w.WriteString(c.TradingClass)
}

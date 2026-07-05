package codec

import (
	"bytes"
	"fmt"
	"strconv"

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
	if len(payload) == 0 {
		return ServerInfo{}, wire.ErrEmptyMessage
	}
	if payload[len(payload)-1] != 0 {
		return ServerInfo{}, wire.ErrMalformedFrame
	}
	r := newFieldReaderBytes(payload)
	if got := r.Remaining(); got < 2 {
		return ServerInfo{}, fmt.Errorf("codec: server info: want >= 2 fields, got %d", got)
	}
	verStr := r.ReadString()
	version, err := strconv.Atoi(verStr)
	if err != nil {
		return ServerInfo{}, fmt.Errorf("codec: server info: parse version %q: %w", verStr, err)
	}
	return ServerInfo{ServerVersion: version, ConnectionTime: r.ReadString()}, nil
}

// DecodeBatch decodes a framed payload into one or more messages keyed by integer msg_id.
func DecodeBatch(sv int, payload []byte) ([]Message, error) {
	if len(payload) == 0 {
		return nil, wire.ErrEmptyMessage
	}
	if payload[len(payload)-1] != 0 {
		return nil, wire.ErrMalformedFrame
	}
	// Peek the msg_id straight from the raw bytes, before splitting fields.
	// This is the future protobuf branch point: msg_ids above 200 carry
	// protobuf payloads whose bodies contain embedded NUL bytes, so a
	// parse-fields-first (NUL-split) approach would corrupt them. Today every
	// frame is classic NUL-delimited, so we only take that path.
	nul := bytes.IndexByte(payload, 0)
	if nul <= 0 {
		return nil, wire.ErrMalformedFrame
	}
	msgID, err := strconv.Atoi(asString(payload[:nul]))
	if err != nil {
		return nil, fmt.Errorf("codec: parse msg_id %q: %w", payload[:nul], err)
	}
	// Reader positioned just past the msg_id field.
	r := newFieldReaderBytes(payload)
	r.off = nul + 1
	r.pos = 1
	msgs, err := decodeByMsgID(sv, msgID, r)
	if err != nil {
		return nil, fmt.Errorf("codec: msg_id %d: %w", msgID, err)
	}
	return msgs, nil
}

// Decode decodes a framed payload into exactly one message.
func Decode(sv int, payload []byte) (Message, error) {
	msgs, err := DecodeBatch(sv, payload)
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

// completedOrderTail probes the fields following a candidate status field. r is
// a value copy positioned just past the status field, so the probe never
// disturbs the caller's cursor. It returns the filled quantity when the tail
// matches the live completed-order layout.
func completedOrderTail(r fieldReader, orderType string) (string, bool) {
	if err := skipCompletedOrderPostStatusPrefix(&r, orderType); err != nil {
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

// completedOrderStatusTail scans forward from the reader's current position
// (anchored right after orderType) for an order-status field whose trailing
// fields match the completed-order layout, returning the status and filled
// quantity. It walks a byte-view copy so the caller's reader is untouched.
func completedOrderStatusTail(r *fieldReader, orderType string) (string, string, error) {
	probe := *r
	probe.err = nil
	for probe.off < len(probe.buf) {
		field := probe.peek()
		if isOrderStatusField(field) {
			if filled, ok := completedOrderTail(probe.after(), orderType); ok {
				return string(field), filled, nil
			}
		}
		probe.Skip(1)
	}
	return "", "", fmt.Errorf("codec: completed order status tail not found")
}

// after returns a value copy of the reader advanced past the current field,
// used to probe a candidate tail without consuming the outer scan cursor.
func (r fieldReader) after() fieldReader {
	r.Skip(1)
	return r
}

func isOrderStatusField(value []byte) bool {
	switch string(value) {
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
func Encode(sv int, msg Message) ([]byte, error) {
	fields, err := msg.encodeWire(sv)
	if err != nil {
		return nil, err
	}
	return wire.EncodeFields(fields), nil
}

type decodeFunc func(r *fieldReader, sv int) ([]Message, error)

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
	InReplaceFAEnd:          decodeReplaceFAEnd,
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
	InHistoricalDataEnd:     decodeHistoricalDataEnd,
	InReceiveFA:             decodeReceiveFA,
	InSoftDollarTiers:       decodeSoftDollarTiers,
	InWSHMetaData:           decodeWSHMetaData,
	InWSHEventData:          decodeWSHEventData,
	InHistoricalSchedule:    decodeHistoricalSchedule,
	InDisplayGroupList:      decodeDisplayGroupList,
	InDisplayGroupUpdated:   decodeDisplayGroupUpdated,
}

// UnknownInbound carries a frame whose msg_id has no registered decoder. An
// unmapped id is not a protocol violation: the Gateway grows new message ids
// over time, and killing the session over one (the pre-fix failure mode) tears
// down every subscription and order handle. The engine surfaces these as
// session events so drift stays observable; the raw fields are preserved for
// diagnosis and re-encode verbatim.
type UnknownInbound struct {
	MsgID  int
	Fields []string
}

func (m UnknownInbound) encodeWire(sv int) ([]string, error) {
	return append([]string{itoa(m.MsgID)}, m.Fields...), nil
}

// decodeByMsgID dispatches on the integer message ID and reads fields in real TWS wire layout.
// Returns []Message because historical data packs multiple bars into one frame.
// r is positioned just past the msg_id field.
func decodeByMsgID(sv int, msgID int, r *fieldReader) ([]Message, error) {
	dec, ok := inboundDecoders[msgID]
	if !ok {
		fields := make([]string, 0, r.Remaining())
		for r.Remaining() > 0 {
			fields = append(fields, r.ReadString())
		}
		return []Message{UnknownInbound{MsgID: msgID, Fields: fields}}, nil
	}
	msgs, err := dec(r, sv)
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

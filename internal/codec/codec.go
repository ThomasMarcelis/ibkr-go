package codec

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
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
	envelope, err := protocol.DecodeEnvelope(sv, payload)
	if err != nil {
		return nil, err
	}
	if envelope.Encoding == protocol.ProtobufBody {
		dec, ok := inboundProtobufDecoders[envelope.MsgID]
		if !ok {
			return []Message{UnknownInbound{
				MsgID:    envelope.MsgID,
				Encoding: protocol.ProtobufBody,
				Payload:  append([]byte(nil), envelope.Body...),
			}}, nil
		}
		msgs, err := dec(envelope.Body, sv)
		if err != nil {
			return []Message{MalformedInbound{
				MsgID:    envelope.MsgID,
				Encoding: protocol.ProtobufBody,
				Payload:  append([]byte(nil), envelope.Body...),
				Err:      fmt.Errorf("codec: protobuf msg_id %d: %w", envelope.MsgID, err),
			}}, nil
		}
		return msgs, nil
	}
	if _, ok := inboundDecoders[envelope.MsgID]; !ok {
		return []Message{UnknownInbound{
			MsgID:    envelope.MsgID,
			Encoding: protocol.ClassicBody,
			Fields:   copyClassicFields(envelope.Body),
			Payload:  append([]byte(nil), envelope.Body...),
		}}, nil
	}

	if len(envelope.Body) > 0 && envelope.Body[len(envelope.Body)-1] != 0 {
		fields := copyClassicFields(envelope.Body)
		return []Message{MalformedInbound{
			MsgID:    envelope.MsgID,
			Encoding: protocol.ClassicBody,
			Fields:   fields,
			Err:      wire.ErrMalformedFrame,
		}}, nil
	}
	r := newFieldReaderBytes(envelope.Body)
	// Decoder field positions include the message ID. The negotiated envelope
	// has already consumed it, so retain that logical position while keeping
	// Remaining based on the full original field count.
	r.pos = 1
	r.total++
	msgs, err := decodeByMsgID(sv, envelope.MsgID, r)
	if err != nil {
		fields := copyClassicFields(envelope.Body)
		return []Message{MalformedInbound{
			MsgID:    envelope.MsgID,
			Encoding: protocol.ClassicBody,
			Fields:   fields,
			Err:      fmt.Errorf("codec: msg_id %d: %w", envelope.MsgID, err),
		}}, nil
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
	if malformed, ok := msgs[0].(MalformedInbound); ok {
		return nil, malformed.Err
	}
	return msgs[0], nil
}

// Encode encodes a message in the real TWS wire format (integer msg_id prefix).
func Encode(sv int, msg OutboundMessage) ([]byte, error) {
	msgID := msg.messageID()
	if msgID < 0 {
		return nil, fmt.Errorf("codec: invalid outbound msg_id %d", msgID)
	}
	if version, ok := protocol.OutboundProtobufVersion(msgID); ok && sv >= version {
		if proto, ok := msg.(protobufEncoder); ok {
			body, err := proto.encodeProto(sv)
			if err != nil {
				return nil, err
			}
			return protocol.EncodeProtobufEnvelope(sv, msgID, body)
		}
		return nil, fmt.Errorf("codec: msg_id %d protobuf encoding is not implemented for server_version %d", msgID, sv)
	}
	classic, ok := msg.(classicEncoder)
	if !ok {
		if version, versioned := protocol.OutboundProtobufVersion(msgID); versioned {
			return nil, fmt.Errorf("codec: msg_id %d requires server_version %d", msgID, version)
		}
		return nil, fmt.Errorf("codec: msg_id %d classic encoding is not implemented for server_version %d", msgID, sv)
	}
	fields, err := classic.encodeWire(sv)
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("codec: message encoded no fields")
	}
	wireMsgID, err := strconv.Atoi(fields[0])
	if err != nil || wireMsgID != msgID {
		return nil, fmt.Errorf("codec: classic encoder returned msg_id %q, want %d", fields[0], msgID)
	}
	return protocol.EncodeClassicEnvelope(sv, msgID, fields[1:])
}

type decodeFunc func(r *fieldReader, sv int) ([]Message, error)
type protobufDecodeFunc func(body []byte, sv int) ([]Message, error)

type protobufEncoder interface {
	encodeProto(sv int) ([]byte, error)
}

// inboundDecoders maps msg_id to its decoder. One explicit table, no
// init() registration.
var inboundDecoders = map[int]decodeFunc{
	protocol.InCurrentTimeInMillis:    decodeCurrentTimeInMillis,
	protocol.InNextValidID:            decodeNextValidID,
	protocol.InScannerParameters:      decodeScannerParameters,
	protocol.InScannerData:            decodeScannerData,
	protocol.InTickEFP:                decodeTickEFP,
	protocol.InCurrentTime:            decodeCurrentTime,
	protocol.InDeltaNeutralValidation: decodeDeltaNeutralValidation,
	protocol.InSecDefOptParams:        decodeSecDefOptParams,
	protocol.InSecDefOptParamsEnd:     decodeSecDefOptParamsEnd,
	protocol.InFamilyCodes:            decodeFamilyCodes,
	protocol.InMktDepthExchanges:      decodeMktDepthExchanges,
	protocol.InNewsArticle:            decodeNewsArticle,
	protocol.InTickNews:               decodeTickNews,
	protocol.InNewsProviders:          decodeNewsProviders,
	protocol.InSymbolSamples:          decodeSymbolSamples,
	protocol.InSmartComponents:        decodeSmartComponents,
	protocol.InHistoricalNews:         decodeHistoricalNews,
	protocol.InHistoricalNewsEnd:      decodeHistoricalNewsEnd,
	protocol.InMarketRule:             decodeMarketRule,
	protocol.InUserInfo:               decodeUserInfo,
	protocol.InNewsBulletins:          decodeNewsBulletins,
	protocol.InPnL:                    decodePnL,
	protocol.InPnLSingle:              decodePnLSingle,
	protocol.InReceiveFA:              decodeReceiveFA,
	protocol.InSoftDollarTiers:        decodeSoftDollarTiers,
	protocol.InWSHMetaData:            decodeWSHMetaData,
	protocol.InWSHEventData:           decodeWSHEventData,
	protocol.InDisplayGroupList:       decodeDisplayGroupList,
	protocol.InDisplayGroupUpdated:    decodeDisplayGroupUpdated,
}

// UnknownInbound carries a frame whose msg_id has no registered decoder. An
// unmapped id is not a protocol violation: the Gateway grows new message ids
// over time, and killing the session over one (the pre-fix failure mode) tears
// down every subscription and order handle. The engine surfaces these as
// session events so drift stays observable; the raw fields are preserved for
// diagnosis.
type UnknownInbound struct {
	MsgID    int
	Encoding protocol.BodyEncoding
	Fields   []string
	Payload  []byte
}

// MalformedInbound carries a self-contained frame with a trustworthy message
// ID whose body violates classic framing or did not match its registered
// decoder. The engine reports the diagnostic and retires the entire transport
// generation because later frames cannot safely complete a partial snapshot.
type MalformedInbound struct {
	MsgID    int
	Encoding protocol.BodyEncoding
	Fields   []string
	Payload  []byte
	Err      error
}

func copyClassicFields(body []byte) []string {
	if len(body) == 0 {
		return nil
	}
	if body[len(body)-1] == 0 {
		body = body[:len(body)-1]
	}
	return strings.Split(string(body), "\x00")
}

// decodeByMsgID dispatches on the integer message ID and reads fields in real TWS wire layout.
// Returns []Message because historical data packs multiple bars into one frame.
// r is positioned just past the msg_id field.
func decodeByMsgID(sv int, msgID int, r *fieldReader) ([]Message, error) {
	dec := inboundDecoders[msgID]
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
	w.WriteString(c.Strike)
	w.WriteString(c.Right)
	w.WriteString(c.Multiplier)
	w.WriteString(c.Exchange)
	w.WriteString(c.PrimaryExchange)
	w.WriteString(c.Currency)
	w.WriteString(c.LocalSymbol)
	w.WriteString(c.TradingClass)
}

func itoa(v int) string     { return strconv.Itoa(v) }
func i64toa(v int64) string { return strconv.FormatInt(v, 10) }

func btoa(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

// ProtobufMessageID is added to a classic message ID when its body is encoded
// with protobuf. At server_version 201 and later every message ID, including
// those whose body remains classic, is a raw four-byte big-endian integer.
const ProtobufMessageID = 200

// BodyEncoding identifies the encoding of the bytes after a message ID.
type BodyEncoding uint8

const (
	ClassicBody BodyEncoding = iota + 1
	ProtobufBody
)

// Envelope is the negotiated message envelope inside the outer length frame.
// Body aliases payload and is intended for immediate decoding.
type Envelope struct {
	MsgID    int
	WireID   int
	Encoding BodyEncoding
	Body     []byte
}

// DecodeEnvelope separates a message ID from its body according to the
// negotiated server version. The pre-session server-info frame has no message
// ID and must not be passed here.
func DecodeEnvelope(serverVersion int, payload []byte) (Envelope, error) {
	if len(payload) == 0 {
		return Envelope{}, wire.ErrEmptyMessage
	}
	if serverVersion < MinServerVersionProtobuf {
		nul := bytes.IndexByte(payload, 0)
		if nul <= 0 {
			return Envelope{}, wire.ErrMalformedFrame
		}
		msgID, ok := parseClassicMessageID(payload[:nul])
		if !ok {
			return Envelope{}, fmt.Errorf("protocol: parse classic msg_id %q: %w", payload[:nul], wire.ErrMalformedFrame)
		}
		return Envelope{
			MsgID:    msgID,
			WireID:   msgID,
			Encoding: ClassicBody,
			Body:     payload[nul+1:],
		}, nil
	}

	if len(payload) < 4 {
		return Envelope{}, fmt.Errorf("protocol: raw msg_id requires 4 bytes, got %d: %w", len(payload), wire.ErrMalformedFrame)
	}
	raw := binary.BigEndian.Uint32(payload[:4])
	if raw == 0 || raw > math.MaxInt32 {
		return Envelope{}, fmt.Errorf("protocol: invalid raw msg_id %d: %w", raw, wire.ErrMalformedFrame)
	}
	wireID := int(raw)
	envelope := Envelope{
		MsgID:    wireID,
		WireID:   wireID,
		Encoding: ClassicBody,
		Body:     payload[4:],
	}
	if wireID > ProtobufMessageID {
		envelope.MsgID -= ProtobufMessageID
		envelope.Encoding = ProtobufBody
	}
	return envelope, nil
}

func parseClassicMessageID(field []byte) (int, bool) {
	if len(field) == 0 {
		return 0, false
	}
	value := 0
	for _, digit := range field {
		if digit < '0' || digit > '9' {
			return 0, false
		}
		d := int(digit - '0')
		if value > (math.MaxInt32-d)/10 {
			return 0, false
		}
		value = value*10 + d
	}
	return value, true
}

// EncodeClassicEnvelope encodes a classic field body with the message-ID
// representation selected by the negotiated server version. fields excludes
// the message ID and may be empty for an ID-only message.
func EncodeClassicEnvelope(serverVersion, msgID int, fields []string) ([]byte, error) {
	if msgID < 0 || msgID > math.MaxInt32 || (serverVersion >= MinServerVersionProtobuf && msgID == 0) {
		return nil, fmt.Errorf("protocol: invalid classic msg_id %d", msgID)
	}
	for i, field := range fields {
		if strings.IndexByte(field, 0) >= 0 {
			return nil, fmt.Errorf("protocol: classic field %d contains NUL", i)
		}
	}
	if serverVersion < MinServerVersionProtobuf {
		all := make([]string, 1, len(fields)+1)
		all[0] = strconv.Itoa(msgID)
		all = append(all, fields...)
		return wire.EncodeFields(all), nil
	}

	body := wire.EncodeFields(fields)
	payload := make([]byte, 4, 4+len(body))
	binary.BigEndian.PutUint32(payload, uint32(msgID))
	return append(payload, body...), nil
}

// EncodeProtobufEnvelope encodes a protobuf body. msgID is the classic base
// ID; the protobuf discriminator is applied exactly once here.
func EncodeProtobufEnvelope(serverVersion, msgID int, body []byte) ([]byte, error) {
	if serverVersion < MinServerVersionProtobuf {
		return nil, fmt.Errorf("protocol: protobuf msg_id %d requires server_version %d", msgID, MinServerVersionProtobuf)
	}
	if msgID <= 0 || msgID > math.MaxInt32-ProtobufMessageID {
		return nil, fmt.Errorf("protocol: invalid protobuf msg_id %d", msgID)
	}
	payload := make([]byte, 4, 4+len(body))
	binary.BigEndian.PutUint32(payload, uint32(msgID+ProtobufMessageID))
	return append(payload, body...), nil
}

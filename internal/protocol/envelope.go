package protocol

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
)

// ProtobufMessageID is added to a classic message ID when its body is encoded
// with protobuf. Every supported server version uses a raw four-byte
// big-endian message ID, including messages whose body remains classic.
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

// DecodeEnvelope separates a supported raw message ID from its body. The
// pre-session server-info frame has no message ID and must not be passed here.
func DecodeEnvelope(_ int, payload []byte) (Envelope, error) {
	if len(payload) == 0 {
		return Envelope{}, wire.ErrEmptyMessage
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

// EncodeClassicEnvelope encodes a classic field body after a raw message ID.
// fields excludes the message ID and may be empty for an ID-only message.
func EncodeClassicEnvelope(_ int, msgID int, fields []string) ([]byte, error) {
	if msgID <= 0 || msgID > math.MaxInt32 {
		return nil, fmt.Errorf("protocol: invalid classic msg_id %d", msgID)
	}
	for i, field := range fields {
		if strings.IndexByte(field, 0) >= 0 {
			return nil, fmt.Errorf("protocol: classic field %d contains NUL", i)
		}
	}
	body := wire.EncodeFields(fields)
	payload := make([]byte, 4, 4+len(body))
	binary.BigEndian.PutUint32(payload, uint32(msgID))
	return append(payload, body...), nil
}

// EncodeProtobufEnvelope encodes a protobuf body. msgID is the classic base
// ID; the protobuf discriminator is applied exactly once here.
func EncodeProtobufEnvelope(_ int, msgID int, body []byte) ([]byte, error) {
	if msgID <= 0 || msgID > math.MaxInt32-ProtobufMessageID {
		return nil, fmt.Errorf("protocol: invalid protobuf msg_id %d", msgID)
	}
	payload := make([]byte, 4, 4+len(body))
	binary.BigEndian.PutUint32(payload, uint32(msgID+ProtobufMessageID))
	return append(payload, body...), nil
}

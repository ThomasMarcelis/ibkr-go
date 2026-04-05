package wire

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

var (
	ErrMalformedFrame = errors.New("wire: malformed frame")
	ErrEmptyMessage   = errors.New("wire: empty message")
)

// ReadFrame reads one length-prefixed payload.
func ReadFrame(r io.Reader) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}

	size := binary.BigEndian.Uint32(header[:])
	if size == 0 {
		return nil, ErrEmptyMessage
	}

	payload := make([]byte, size)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

// WriteFrame writes one length-prefixed payload.
func WriteFrame(w io.Writer, payload []byte) error {
	if len(payload) == 0 {
		return ErrEmptyMessage
	}
	if len(payload) > int(^uint32(0)) {
		return fmt.Errorf("wire: payload too large: %d", len(payload))
	}

	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(payload)))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

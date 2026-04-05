package wire

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const MaxFrameSize = 64 << 20 // 64 MiB — well above any real IBKR message

var (
	ErrMalformedFrame = errors.New("wire: malformed frame")
	ErrEmptyMessage   = errors.New("wire: empty message")
	ErrFrameTooLarge  = errors.New("wire: frame exceeds maximum size")
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
	if size > MaxFrameSize {
		return nil, ErrFrameTooLarge
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
	if len(payload) > MaxFrameSize {
		return ErrFrameTooLarge
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

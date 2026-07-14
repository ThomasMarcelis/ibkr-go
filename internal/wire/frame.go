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

// FrameTooLargeError reports the rejected payload length and configured
// ceiling. The body has not been allocated or read when this is returned.
type FrameTooLargeError struct {
	Size  int
	Limit int
}

func (e *FrameTooLargeError) Error() string {
	return fmt.Sprintf("%v: size %d exceeds limit %d", ErrFrameTooLarge, e.Size, e.Limit)
}

func (e *FrameTooLargeError) Unwrap() error { return ErrFrameTooLarge }

// ReadFrame reads one length-prefixed payload.
func ReadFrame(r io.Reader) ([]byte, error) {
	return ReadFrameWithLimit(r, MaxFrameSize)
}

// ReadFrameWithLimit reads one frame while rejecting an oversized header
// before allocating or consuming the payload.
func ReadFrameWithLimit(r io.Reader, limit int) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}

	size := binary.BigEndian.Uint32(header[:])
	if size == 0 {
		return nil, ErrEmptyMessage
	}
	if limit < 1 || uint64(size) > uint64(limit) {
		return nil, &FrameTooLargeError{Size: int(size), Limit: limit}
	}

	payload := make([]byte, size)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

// WriteFrame writes one length-prefixed payload.
func WriteFrame(w io.Writer, payload []byte) error {
	frame, err := EncodeFrame(payload)
	if err != nil {
		return err
	}
	for len(frame) > 0 {
		n, err := w.Write(frame)
		if n > 0 {
			frame = frame[n:]
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrNoProgress
		}
	}
	return nil
}

// EncodeFrame returns one contiguous length-prefixed frame. Keeping the
// prefix and payload together lets callers distinguish a complete local write
// from a partial frame when a connection fails.
func EncodeFrame(payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return nil, ErrEmptyMessage
	}
	if len(payload) > MaxFrameSize {
		return nil, ErrFrameTooLarge
	}

	frame := make([]byte, 4+len(payload))
	// MaxFrameSize is 64 MiB, so the length is proven to fit uint32 above.
	// The analyzer does not carry that bound through the conversion.
	//nolint:gosec // G115: payload length is bounded by MaxFrameSize.
	binary.BigEndian.PutUint32(frame[:4], uint32(len(payload)))
	copy(frame[4:], payload)
	return frame, nil
}

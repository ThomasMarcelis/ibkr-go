package transport

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// WriteRaw writes raw bytes directly to the connection without framing.
func WriteRaw(conn net.Conn, data []byte) error {
	_, err := conn.Write(data)
	return err
}

// ReadOneFrame reads a single length-prefixed frame with a deadline.
func ReadOneFrame(conn net.Conn, deadline time.Time) ([]byte, error) {
	if err := conn.SetReadDeadline(deadline); err != nil {
		return nil, err
	}
	defer conn.SetReadDeadline(time.Time{})
	return wire.ReadFrame(conn)
}

type Conn struct {
	conn      net.Conn
	logger    *slog.Logger
	sendRate  int
	incoming  chan []byte
	done      chan struct{}
	closeOnce sync.Once
	waitOnce  sync.Once
	waitErr   error
	waitErrMu sync.Mutex

	queueMu        sync.Mutex
	queueCond      *sync.Cond
	outgoing       [][]byte
	outgoingClosed bool
}

func New(conn net.Conn, logger *slog.Logger, sendRate int) *Conn {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	c := &Conn{
		conn:     conn,
		logger:   logger,
		sendRate: sendRate,
		incoming: make(chan []byte, 64),
		done:     make(chan struct{}),
	}
	c.queueCond = sync.NewCond(&c.queueMu)
	go c.readLoop()
	go c.writeLoop()
	return c
}

func (c *Conn) Incoming() <-chan []byte {
	return c.incoming
}

func (c *Conn) Done() <-chan struct{} {
	return c.done
}

func (c *Conn) Send(ctx context.Context, payload []byte) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return c.Wait()
	default:
	}

	copyPayload := append([]byte(nil), payload...)

	c.queueMu.Lock()
	if c.outgoingClosed {
		c.queueMu.Unlock()
		return c.Wait()
	}
	c.outgoing = append(c.outgoing, copyPayload)
	c.queueCond.Signal()
	c.queueMu.Unlock()
	return nil
}

func (c *Conn) Close() error {
	c.closeOnce.Do(func() {
		c.queueMu.Lock()
		c.outgoingClosed = true
		c.queueCond.Broadcast()
		c.queueMu.Unlock()
		_ = c.conn.Close()
	})
	return nil
}

func (c *Conn) Wait() error {
	<-c.done
	c.waitErrMu.Lock()
	defer c.waitErrMu.Unlock()
	return c.waitErr
}

func (c *Conn) readLoop() {
	defer close(c.incoming)
	for {
		payload, err := wire.ReadFrame(c.conn)
		if err != nil {
			c.finish(err)
			return
		}
		select {
		case <-c.done:
			return
		case c.incoming <- payload:
		}
	}
}

func (c *Conn) writeLoop() {
	var ticker <-chan time.Time
	if c.sendRate > 0 {
		interval := time.Second / time.Duration(c.sendRate)
		t := time.NewTicker(interval)
		defer t.Stop()
		ticker = t.C
	}

	for {
		c.queueMu.Lock()
		for len(c.outgoing) == 0 && !c.outgoingClosed {
			c.queueCond.Wait()
		}
		if len(c.outgoing) == 0 && c.outgoingClosed {
			c.queueMu.Unlock()
			c.finish(nil)
			return
		}
		payload := c.outgoing[0]
		c.outgoing[0] = nil
		c.outgoing = c.outgoing[1:]
		c.queueMu.Unlock()

		select {
		case <-c.done:
			return
		default:
		}

		if ticker != nil {
			select {
			case <-c.done:
				return
			case <-ticker:
			}
		}
		if err := wire.WriteFrame(c.conn, payload); err != nil {
			c.finish(err)
			return
		}
	}
}

func (c *Conn) finish(err error) {
	c.waitOnce.Do(func() {
		c.queueMu.Lock()
		c.outgoingClosed = true
		c.queueCond.Broadcast()
		c.queueMu.Unlock()

		c.waitErrMu.Lock()
		defer c.waitErrMu.Unlock()
		if err != nil && !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.ErrClosedPipe) {
			c.waitErr = err
			c.logger.Debug("transport closed", "error", err)
		}
		close(c.done)
		_ = c.conn.Close()
	})
}

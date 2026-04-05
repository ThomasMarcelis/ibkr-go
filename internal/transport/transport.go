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

type Conn struct {
	conn      net.Conn
	logger    *slog.Logger
	sendRate  int
	incoming  chan []byte
	outgoing  chan []byte
	done      chan struct{}
	closeOnce sync.Once
	waitOnce  sync.Once
	waitErr   error
	waitErrMu sync.Mutex
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
		outgoing: make(chan []byte, 64),
		done:     make(chan struct{}),
	}
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
	case c.outgoing <- payload:
		return nil
	}
}

func (c *Conn) Close() error {
	c.closeOnce.Do(func() {
		close(c.outgoing)
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

	for payload := range c.outgoing {
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
	c.finish(nil)
}

func (c *Conn) finish(err error) {
	c.waitOnce.Do(func() {
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

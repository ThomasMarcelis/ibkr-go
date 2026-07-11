package transport

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

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
	writable  chan struct{}
	done      chan struct{}
	closeOnce sync.Once
	closeErr  error
	waitOnce  sync.Once
	waitErr   error
	waitErrMu sync.Mutex

	queueMu        sync.Mutex
	queueCond      *sync.Cond
	outgoing       [][]byte
	outgoingClosed bool
}

const outgoingQueueCap = 256

// maxSendRate is the largest per-second pacing rate the writeLoop ticker can
// represent. The interval is time.Second/sendRate; at 1e9 the interval is 1ns,
// and any higher rate would round the interval to 0, which time.NewTicker
// panics on. Rates at or above this are effectively unbounded pacing anyway, so
// clamping here loses nothing while making the panic unreachable.
const maxSendRate = int(time.Second / time.Nanosecond) // 1_000_000_000

var ErrSendQueueFull = errors.New("transport: outbound queue full")

func New(conn net.Conn, logger *slog.Logger, sendRate int) *Conn {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if sendRate > maxSendRate {
		sendRate = maxSendRate
	}
	c := &Conn{
		conn:     conn,
		logger:   logger,
		sendRate: sendRate,
		incoming: make(chan []byte, 64),
		writable: make(chan struct{}, 1),
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

// Writable is signaled whenever the writer removes an item from the bounded
// outbound queue. A caller that received ErrSendQueueFull can wait for this
// edge and retry admission without polling.
func (c *Conn) Writable() <-chan struct{} { return c.writable }

func (c *Conn) Send(ctx context.Context, payload []byte) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(payload) == 0 {
		return wire.ErrEmptyMessage
	}
	if len(payload) > wire.MaxFrameSize {
		return wire.ErrFrameTooLarge
	}
	copyPayload := append([]byte(nil), payload...)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return c.Wait()
	default:
	}

	c.queueMu.Lock()
	if c.outgoingClosed {
		c.queueMu.Unlock()
		return c.Wait()
	}
	if len(c.outgoing) >= outgoingQueueCap {
		c.queueMu.Unlock()
		return ErrSendQueueFull
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
		c.closeErr = c.conn.Close()
	})
	return c.closeErr
}

func (c *Conn) Wait() error {
	<-c.done
	c.waitErrMu.Lock()
	defer c.waitErrMu.Unlock()
	return c.waitErr
}

func (c *Conn) readLoop() {
	defer close(c.incoming)
	// readLoop owns every post-handshake read on c.conn: the handshake reads
	// its frames with transport.ReadOneFrame on the raw conn *before*
	// transport.New starts this goroutine (see engine_connect.go), and nothing
	// else touches the socket afterward. That exclusive ownership makes it safe
	// to buffer here — wire.ReadFrame issues two reads per frame (length prefix
	// then payload), so an unbuffered conn is two syscalls per frame. A 64 KiB
	// buffer collapses those into one syscall per fill, since the gateway
	// coalesces many small tick frames into each TCP segment.
	br := bufio.NewReaderSize(c.conn, 64<<10)
	for {
		payload, err := wire.ReadFrame(br)
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
		select {
		case c.writable <- struct{}{}:
		default:
		}
		c.queueMu.Unlock()

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

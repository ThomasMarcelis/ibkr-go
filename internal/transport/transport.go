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

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
)

// WriteRaw writes raw bytes directly to the connection without framing.
func WriteRaw(conn net.Conn, data []byte) error {
	_, err := writeAll(conn, data)
	return err
}

// ReadOneFrame reads a single length-prefixed frame with a deadline.
func ReadOneFrame(conn net.Conn, deadline time.Time) ([]byte, error) {
	return ReadOneFrameWithLimit(conn, deadline, wire.MaxFrameSize)
}

// ReadOneFrameWithLimit reads one length-prefixed frame with a deadline and
// rejects an oversized header before reading its body.
func ReadOneFrameWithLimit(conn net.Conn, deadline time.Time, limit int) ([]byte, error) {
	if err := conn.SetReadDeadline(deadline); err != nil {
		return nil, err
	}
	defer conn.SetReadDeadline(time.Time{})
	return wire.ReadFrameWithLimit(conn, limit)
}

type Conn struct {
	conn                 net.Conn
	logger               *slog.Logger
	sendRate             int
	maxInboundFrameBytes int
	incoming             chan []byte
	completed            chan WriteResult
	writable             chan struct{}
	stopping             chan struct{}
	done                 chan struct{}
	stopOnce             sync.Once
	closeErr             error
	waitErr              error
	waitErrMu            sync.Mutex

	queueMu        sync.Mutex
	queueCond      *sync.Cond
	outgoing       []outboundFrame
	outgoingClosed bool
	nextWriteID    WriteID
}

type outboundFrame struct {
	id    WriteID
	frame []byte
}

// WriteID identifies a tracked frame admitted to the outbound queue. Zero is
// reserved for ordinary untracked sends.
type WriteID uint64

// WriteOutcome classifies how much of a tracked frame reached the local
// socket before the transport stopped.
type WriteOutcome uint8

const (
	WriteUnwritten WriteOutcome = iota
	WriteIncomplete
	WriteCompleteLocal
)

// WriteResult is emitted exactly once for each successful SendTracked
// admission. CompleteLocal means the whole framed message was accepted by the
// local socket; it is not a Gateway acknowledgement.
type WriteResult struct {
	ID      WriteID
	Outcome WriteOutcome
	Err     error
}

const outgoingQueueCap = 256

// maxSendRate is the largest per-second pacing rate the writeLoop ticker can
// represent. The interval is time.Second/sendRate; at 1e9 the interval is 1ns,
// and any higher rate would round the interval to 0, which time.NewTicker
// panics on. Rates at or above this are effectively unbounded pacing anyway, so
// clamping here loses nothing while making the panic unreachable.
const maxSendRate = int(time.Second / time.Nanosecond) // 1_000_000_000

var (
	ErrClosed        = errors.New("transport: closed")
	ErrSendQueueFull = errors.New("transport: outbound queue full")
)

func New(conn net.Conn, logger *slog.Logger, sendRate int) *Conn {
	return NewWithInboundFrameLimit(conn, logger, sendRate, wire.MaxFrameSize)
}

// NewWithInboundFrameLimit starts a connection whose inbound reader rejects
// frames larger than maxInboundFrameBytes before allocation.
func NewWithInboundFrameLimit(conn net.Conn, logger *slog.Logger, sendRate, maxInboundFrameBytes int) *Conn {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if sendRate > maxSendRate {
		sendRate = maxSendRate
	}
	c := &Conn{
		conn:                 conn,
		logger:               logger,
		sendRate:             sendRate,
		maxInboundFrameBytes: maxInboundFrameBytes,
		incoming:             make(chan []byte, 64),
		completed:            make(chan WriteResult, outgoingQueueCap+1),
		writable:             make(chan struct{}, 1),
		stopping:             make(chan struct{}),
		done:                 make(chan struct{}),
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

// Stopping closes as soon as the connection can no longer admit writes. Done
// closes later, after the writer reports every tracked admission outcome.
func (c *Conn) Stopping() <-chan struct{} { return c.stopping }

// Completions reports the local write outcome of tracked sends. Every admitted
// tracked frame yields exactly one result, and the channel closes after all
// results are published and before Done. Receiving one result is not a writer
// finalization barrier; consumers must keep draining while shutdown proceeds.
func (c *Conn) Completions() <-chan WriteResult { return c.completed }

// Writable is signaled whenever the writer removes an item from the bounded
// outbound queue. A caller that received ErrSendQueueFull can wait for this
// edge and retry admission without polling.
func (c *Conn) Writable() <-chan struct{} { return c.writable }

func (c *Conn) Send(ctx context.Context, payload []byte) error {
	_, err := c.admit(ctx, payload, false)
	return err
}

// SendTracked admits a frame and returns the ID used by Completions. Admission
// does not imply that any byte has reached the socket.
func (c *Conn) SendTracked(ctx context.Context, payload []byte) (WriteID, error) {
	return c.admit(ctx, payload, true)
}

func (c *Conn) admit(ctx context.Context, payload []byte, tracked bool) (WriteID, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(payload) == 0 {
		return 0, wire.ErrEmptyMessage
	}
	if len(payload) > wire.MaxFrameSize {
		return 0, wire.ErrFrameTooLarge
	}
	frame, err := wire.EncodeFrame(payload)
	if err != nil {
		return 0, err
	}

	select {
	case <-ctx.Done():
		return 0, context.Cause(ctx)
	case <-c.done:
		return 0, c.terminalError()
	default:
	}

	c.queueMu.Lock()
	if c.outgoingClosed {
		c.queueMu.Unlock()
		return 0, c.terminalError()
	}
	if len(c.outgoing) >= outgoingQueueCap {
		c.queueMu.Unlock()
		return 0, ErrSendQueueFull
	}
	var id WriteID
	if tracked {
		c.nextWriteID++
		id = c.nextWriteID
	}
	c.outgoing = append(c.outgoing, outboundFrame{id: id, frame: frame})
	c.queueCond.Signal()
	c.queueMu.Unlock()

	return id, nil
}

func (c *Conn) terminalError() error {
	if err := c.WaitError(); err != nil {
		return err
	}
	return ErrClosed
}

func (c *Conn) Close() error {
	c.stop(nil)
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
		payload, err := wire.ReadFrameWithLimit(br, c.maxInboundFrameBytes)
		if err != nil {
			c.stop(err)
			return
		}
		select {
		case <-c.stopping:
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
		if c.outgoingClosed {
			unwritten := c.takeQueuedLocked()
			c.queueMu.Unlock()
			c.reportUnwritten(unwritten)
			c.finishWriter()
			return
		}
		c.queueMu.Unlock()

		if ticker != nil {
			select {
			case <-c.stopping:
				continue
			case <-ticker:
			}
		}

		c.queueMu.Lock()
		if c.outgoingClosed {
			unwritten := c.takeQueuedLocked()
			c.queueMu.Unlock()
			c.reportUnwritten(unwritten)
			c.finishWriter()
			return
		}
		item := c.outgoing[0]
		c.outgoing[0] = outboundFrame{}
		c.outgoing = c.outgoing[1:]
		select {
		case c.writable <- struct{}{}:
		default:
		}
		c.queueMu.Unlock()

		n, err := writeAll(c.conn, item.frame)
		outcome := WriteUnwritten
		if n == len(item.frame) {
			outcome = WriteCompleteLocal
		} else if n > 0 {
			outcome = WriteIncomplete
		}
		if err != nil {
			c.stop(err)
		}
		if item.id != 0 {
			c.completed <- WriteResult{ID: item.id, Outcome: outcome, Err: err}
		}
		if err != nil {
			c.queueMu.Lock()
			unwritten := c.takeQueuedLocked()
			c.queueMu.Unlock()
			c.reportUnwritten(unwritten)
			c.finishWriter()
			return
		}
	}
}

func (c *Conn) stop(err error) {
	c.stopOnce.Do(func() {
		logErr := err != nil && !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.ErrClosedPipe)
		if logErr {
			c.waitErrMu.Lock()
			c.waitErr = err
			c.waitErrMu.Unlock()
		}

		c.queueMu.Lock()
		c.outgoingClosed = true
		close(c.stopping)
		c.queueCond.Broadcast()
		c.queueMu.Unlock()
		if logErr {
			c.logger.Debug("transport closed", "error", err)
		}
		c.closeErr = c.conn.Close()
	})
}

func (c *Conn) takeQueuedLocked() []outboundFrame {
	queued := c.outgoing
	c.outgoing = nil
	return queued
}

func (c *Conn) reportUnwritten(items []outboundFrame) {
	for _, item := range items {
		if item.id != 0 {
			c.completed <- WriteResult{ID: item.id, Outcome: WriteUnwritten, Err: c.WaitError()}
		}
	}
}

func (c *Conn) finishWriter() {
	// Done is the terminal publication barrier: observing it guarantees every
	// tracked result was published and Completions was closed.
	close(c.completed)
	close(c.done)
}

func (c *Conn) WaitError() error {
	c.waitErrMu.Lock()
	defer c.waitErrMu.Unlock()
	return c.waitErr
}

func writeAll(w io.Writer, p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		n, err := w.Write(p)
		if n > 0 {
			written += n
			p = p[n:]
		}
		if err != nil {
			return written, err
		}
		if n == 0 {
			return written, io.ErrNoProgress
		}
	}
	return written, nil
}

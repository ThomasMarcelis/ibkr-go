package ibkr

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/internal/transport"
)

type engine struct {
	cfg config

	cmds         chan func()
	incoming     chan any
	transportErr chan error
	ready        chan error
	done         chan struct{}
	events       *observer[Event]

	waitMu  sync.Mutex
	waitErr error

	snapshotMu sync.RWMutex
	snapshot   Snapshot

	transport     *transport.Conn
	serverVersion int

	keyed       map[int]*route
	singletons  map[string]*route
	orders      map[int64]*orderRoute
	executions  executionCorrelator
	execToOrder map[string]int64 // execID → orderID for commission routing to order handles

	nextReqID                int
	nextHistoricalRequest    time.Time
	recentHistoricalRequests map[string]time.Time

	bootstrap bootstrapState
	closed    bool

	reconnectAttempt int
}

type bootstrapState struct {
	serverInfo    bool
	managed       bool
	nextValidID   bool
	readyReported bool
}

const (
	// The codec gates post-176 wire fields on the negotiated version. The
	// sv200 layout is live-validated; 176..199 are covered by version-gated
	// encode/decode paths.
	minServerVersion = 176
	maxServerVersion = 200
	bootstrapTimeout = 5 * time.Second

	reconnectBackoff    = time.Second
	reconnectBackoffMax = 16 * time.Second

	historicalRequestSpacing   = 2 * time.Second
	historicalIdenticalSpacing = 15 * time.Second
)

// advertisedServerVersionMax is the upper bound sent in the v100+ handshake.
// The gateway negotiates down to it, so capping it below maxServerVersion
// forces a live session onto an older wire layout. Only the version-matrix
// live tests override it (see export_test.go); production always advertises
// maxServerVersion.
var advertisedServerVersionMax = maxServerVersion

type route struct {
	opKind       OpKind
	subscription bool
	resume       ResumePolicy
	request      codec.Message
	handle       func(any, *engine)
	handleAPIErr func(codec.APIError, *engine)
	onDisconnect func(*engine, error) bool
	emitGap      func(*engine)
	emitResumed  func(*engine)
	close        func(error)
	gapped       bool // true after Gap emitted, reset on Resumed; prevents double emission
}

type orderRoute struct {
	orderID          int64
	handle           *OrderHandle
	preview          chan previewResult // non-nil for a what-if preview route; no handle is created
	closed           bool
	gapped           bool // true after Gap emitted, reset on Resumed; prevents double emission
	terminalCloseSeq uint64
}

// previewResult carries the single what-if open_order echo back to a blocked
// PreviewOrder caller: either the decoded OpenOrder or the decode error.
type previewResult struct {
	order OpenOrder
	err   error
}

type parsedOpenOrder struct {
	order OpenOrder
}

func dialEngine(ctx context.Context, opts ...Option) (*engine, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.clientID < 0 {
		return nil, fmt.Errorf("ibkr: client id must be >= 0")
	}
	if cfg.eventBuffer < 1 {
		return nil, fmt.Errorf("ibkr: event buffer must be >= 1")
	}

	e := &engine{
		cfg:                      cfg,
		cmds:                     make(chan func(), 256),
		incoming:                 make(chan any, 256),
		transportErr:             make(chan error, 8),
		ready:                    make(chan error, 1),
		done:                     make(chan struct{}),
		events:                   newObserver[Event](cfg.eventBuffer),
		keyed:                    make(map[int]*route),
		singletons:               make(map[string]*route),
		orders:                   make(map[int64]*orderRoute),
		executions:               newExecutionCorrelator(),
		execToOrder:              make(map[string]int64),
		recentHistoricalRequests: make(map[string]time.Time),
		nextReqID:                1,
		snapshot: Snapshot{
			State: StateDisconnected,
		},
	}
	go e.run()
	e.enqueue(func() {
		e.startConnect(ctx, false)
	})

	select {
	case err := <-e.ready:
		if err != nil {
			return nil, err
		}
		return e, nil
	case <-ctx.Done():
		_ = e.Close()
		return nil, ctx.Err()
	}
}

func (e *engine) Close() error {
	e.enqueue(func() {
		e.closeEngine(ErrClosed)
	})
	return nil
}

func (e *engine) Done() <-chan struct{} {
	return e.done
}

func (e *engine) Wait() error {
	<-e.done
	e.waitMu.Lock()
	defer e.waitMu.Unlock()
	return e.waitErr
}

func (e *engine) Session() Snapshot {
	e.snapshotMu.RLock()
	defer e.snapshotMu.RUnlock()

	snap := e.snapshot
	snap.ManagedAccounts = append([]string(nil), snap.ManagedAccounts...)
	return snap
}

func (e *engine) SessionEvents() <-chan Event {
	return e.events.Chan()
}

func (e *engine) enqueue(fn func()) {
	select {
	case <-e.done:
		return
	case e.cmds <- fn:
	}
}

func (e *engine) reportReady(err error) {
	if e.bootstrap.readyReported {
		return
	}
	e.bootstrap.readyReported = true
	select {
	case e.ready <- err:
	default:
	}
}

func (e *engine) setState(next State, code int, message string, err error) {
	e.snapshotMu.Lock()
	prev := e.snapshot.State
	e.snapshot.State = next
	connSeq := e.snapshot.ConnectionSeq
	e.snapshotMu.Unlock()

	e.events.EmitLatest(Event{
		At:            time.Now().UTC(),
		State:         next,
		Previous:      prev,
		ConnectionSeq: connSeq,
		Code:          code,
		Message:       message,
		Err:           err,
	})
}

// emitEvent publishes an informational session event (e.g. farm-status
// or market-data warnings) without changing session state.
func (e *engine) emitEvent(code int, message string) {
	e.snapshotMu.RLock()
	state := e.snapshot.State
	connSeq := e.snapshot.ConnectionSeq
	e.snapshotMu.RUnlock()
	e.events.EmitLatest(Event{
		At:            time.Now().UTC(),
		State:         state,
		Previous:      state,
		ConnectionSeq: connSeq,
		Code:          code,
		Message:       message,
	})
}

func (e *engine) updateSnapshot(update func(*Snapshot)) {
	e.snapshotMu.Lock()
	defer e.snapshotMu.Unlock()
	update(&e.snapshot)
}

func (e *engine) send(msg codec.Message) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return e.sendContext(ctx, msg)
}

func (e *engine) sendContext(ctx context.Context, msg codec.Message) error {
	if e.transport == nil {
		return ErrNotReady
	}
	payload, err := codec.Encode(e.serverVersion, msg)
	if err != nil {
		return err
	}
	err = e.transport.Send(ctx, payload)
	if errors.Is(err, transport.ErrSendQueueFull) {
		return ErrInterrupted
	}
	return err
}

func normalizeTransportErr(err error) error {
	if errors.Is(err, transport.ErrSendQueueFull) {
		return ErrInterrupted
	}
	return err
}

func (e *engine) allocReqID() int {
	for {
		id := e.nextReqID
		e.nextReqID++
		if _, conflict := e.orders[int64(id)]; !conflict {
			return id
		}
	}
}

func (e *engine) allocOrderID() int64 {
	for {
		id := e.snapshot.NextValidID
		e.updateSnapshot(func(s *Snapshot) {
			s.NextValidID++
		})
		if _, conflict := e.keyed[int(id)]; !conflict {
			return id
		}
	}
}

func (e *engine) connectionSeq() uint64 {
	e.snapshotMu.RLock()
	defer e.snapshotMu.RUnlock()
	return e.snapshot.ConnectionSeq
}

func (e *engine) isReady() bool {
	if e.transport == nil {
		return false
	}
	e.snapshotMu.RLock()
	state := e.snapshot.State
	e.snapshotMu.RUnlock()
	return state == StateReady || state == StateDegraded
}

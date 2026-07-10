package ibkr

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/internal/protocol"
	"github.com/ThomasMarcelis/ibkr-go/internal/transport"
)

type engine struct {
	cfg config

	cmds         chan func()
	incoming     chan any
	transportErr chan transportLoss
	ready        chan error
	done         chan struct{}
	events       *observer[Event]

	waitMu  sync.Mutex
	waitErr error

	snapshotMu sync.RWMutex
	snapshot   Snapshot

	transport     *transport.Conn
	serverVersion int

	keyed      map[int]*route
	singletons map[string]*route
	orders     map[int64]*orderRoute
	executions executionCorrelator
	// execDeliveries is the order-handle leg's per-ExecID delivery record.
	// orderID routes commissions to the owning handle and its presence dedupes
	// an Executions() snapshot replaying a fill the handle already saw live.
	// delivered dedupes an identical commission re-send while letting a
	// re-send with changed content (e.g. a realizedPNL update) through.
	// pending buffers commissions that arrived before their execution detail;
	// they flush when the execution claims the ExecID, and an entry no
	// execution ever claims (another client's fill) evicts itself after the
	// drain window. Entries are dropped with their order's route
	// (forgetOrderExecutions).
	execDeliveries map[string]*execDelivery
	// unknownInboundSeen records msg ids already reported as unknown, so a
	// hot misdecoded feed logs and emits once instead of per frame.
	unknownInboundSeen map[int]struct{}
	readySetups        []*readySetup

	nextReqID                int
	nextHistoricalRequest    time.Time
	recentHistoricalRequests map[string]time.Time

	bootstrap bootstrapState
	closed    bool

	reconnectAttempt int
}

type transportLoss struct {
	transport *transport.Conn
	err       error
}

type bootstrapState struct {
	serverInfo    bool
	managed       bool
	nextValidID   bool
	readyReported bool
}

const (
	// The codec gates post-176 wire fields and the sv201 envelope on the
	// negotiated version. The classic sv200 layout, exact-sv201 executions
	// migration, exact-sv202 zero-strike boundary, exact-sv203 order
	// protobuf lifecycle, exact-sv204 order queries, and exact-sv205 contract
	// data are live-validated;
	// 176..199 are compatibility paths.
	minServerVersion = protocol.SupportedMinServerVersion
	maxServerVersion = protocol.SupportedMaxServerVersion
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
	opKind         OpKind
	subscription   bool
	resume         ResumePolicy
	request        codec.Message
	handle         func(any, *engine)
	handleAPIErr   func(codec.APIError, *engine)
	onDisconnect   func(*engine, error) bool
	emitGap        func(*engine)
	emitResumed    func(*engine)
	validateResume func(*engine) error
	close          func(error)
	gapped         bool // true after Gap emitted, reset on Resumed; prevents double emission
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

// resolvePreview resolves a pending what-if preview route and reports whether
// the route was a preview. Preview routes have no OrderHandle, so every
// dispatch path that would touch or.handle must divert through this first:
// the buffered channel is resolved at most once, guarded by closed. Callers
// on the actor goroutine only.
func (or *orderRoute) resolvePreview(res previewResult) bool {
	if or.preview == nil {
		return false
	}
	if !or.closed {
		or.closed = true
		or.preview <- res
	}
	return true
}

type parsedOpenOrder struct {
	order OpenOrder
}

func dialEngine(ctx context.Context, opts ...Option) (*engine, error) {
	cfg, err := applyOptions(opts)
	if err != nil {
		return nil, err
	}

	e := &engine{
		cfg:                      cfg,
		cmds:                     make(chan func(), 256),
		incoming:                 make(chan any, 256),
		transportErr:             make(chan transportLoss, 8),
		ready:                    make(chan error, 1),
		done:                     make(chan struct{}),
		events:                   newObserver[Event](cfg.eventBuffer),
		keyed:                    make(map[int]*route),
		singletons:               make(map[string]*route),
		orders:                   make(map[int64]*orderRoute),
		executions:               newExecutionCorrelator(),
		execDeliveries:           make(map[string]*execDelivery),
		unknownInboundSeen:       make(map[int]struct{}),
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
		e.closeEngine(ErrClosed, nil)
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

func (e *engine) closedOperationError() error {
	if err := e.Wait(); err != nil {
		return err
	}
	return ErrClosed
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
	e.emitSessionEvent(code, message, nil)
}

func (e *engine) emitSessionEvent(code int, message string, err error) {
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
		Err:           err,
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

package ibkr

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/transport"
)

type engine struct {
	cfg config

	cmds           chan func()
	incoming       chan any
	transportErr   chan transportLoss
	connectResults chan connectResult
	ready          chan error
	done           chan struct{}
	events         *observer[Event]

	waitMu         sync.Mutex
	waitErr        error
	connectErrMu   sync.Mutex
	lastConnectErr error

	snapshotMu sync.RWMutex
	snapshot   Snapshot

	transport           *transport.Conn
	retiringTransport   *transport.Conn
	transportRetireErr  error
	transportGeneration uint64
	serverVersion       int

	keyed      map[int]*route
	singletons map[string]*route
	orders     map[int64]*orderRoute
	previews   map[int64]*previewRoute
	// pendingOrderWrites owns the admission-to-write gap for order frames.
	// Completion is handled on the actor before transport loss, so an
	// unwritten order cannot be mistaken for one IBKR received.
	pendingOrderWrites map[transportWriteKey]int64
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
	unknownInboundSeen   map[int]struct{}
	malformedInboundSeen map[int]struct{}
	dirtySingletons      map[string]uint64
	readySetups          []*readySetup

	nextReqID                int
	orderIDHighWater         int64
	nextClockRequest         time.Time
	nextHistoricalRequest    time.Time
	recentHistoricalRequests map[string]time.Time

	bootstrap        bootstrapState
	closed           bool
	lifetimeCtx      context.Context
	cancelLifetime   context.CancelFunc
	connectAttemptID uint64
	connectCancel    context.CancelFunc
	stabilityEpoch   uint64

	reconnectAttempt int
	resumePending    []resumeRoute
	resumeWaiting    bool
}

type resumeRoute struct {
	reqID int
	key   string
	route *route
}

type transportLoss struct {
	transport *transport.Conn
	err       error
}

type transportWriteKey struct {
	transport *transport.Conn
	id        transport.WriteID
}

type transportWrite struct {
	transport *transport.Conn
	result    transport.WriteResult
}

type bootstrapState struct {
	serverInfo    bool
	managed       bool
	nextValidID   bool
	readyReported bool
}

const (
	// Supported versions are the live-validated 200 classic layout and the
	// staged protobuf migrations and protocol additions through 225.
	minServerVersion = protocol.SupportedMinServerVersion
	maxServerVersion = protocol.SupportedMaxServerVersion
	bootstrapTimeout = 5 * time.Second

	reconnectBackoff    = time.Second
	reconnectBackoffMax = 16 * time.Second

	historicalRequestSpacing   = 2 * time.Second
	historicalIdenticalSpacing = 15 * time.Second
	// Live Gateway observations show reqCurrentTime requests inside four
	// seconds may be silently suppressed. Keep one conservative shared clock
	// gate until the seconds and milliseconds opcodes can be re-measured live.
	clockRequestSpacing = 4250 * time.Millisecond
)

// advertisedServerVersionMax is the upper bound sent in the v100+ handshake.
// The gateway negotiates down to it, so capping it below maxServerVersion
// forces a live session onto a lower supported layout. Only the version-matrix
// live tests override it (see export_test.go); production always advertises
// maxServerVersion.
var advertisedServerVersionMax = maxServerVersion

type route struct {
	opKind           OpKind
	subscription     bool
	resume           ResumePolicy
	request          codec.Message
	handle           func(any, *engine)
	handleCommission func(codec.CommissionReport, *engine)
	handleAPIErr     func(codec.APIError, *engine)
	onDisconnect     func(*engine, error) bool // true retains the route; caller deletes on false
	emitGap          func(*engine)
	emitRestored     func(*engine)
	emitResubscribed func(*engine)
	validateResume   func(*engine) error
	responsePending  func() bool
	cancel           func(error)
	close            func(error)
	cleanup          func()
	gapped           bool // true after Gap emitted; prevents double emission
	generation       uint64
}

type orderRoute struct {
	orderID          int64
	handle           *OrderHandle
	cleanup          func()
	attachedOrderIDs []int64
	closed           bool
	gapped           bool // true after Gap emitted; prevents duplicate gap events
	recoveryRequired bool
	working          bool
	pendingWrite     transportWriteKey
}

type previewRoute struct {
	result   chan previewResult
	resolved bool
}

type previewResult struct {
	state OrderState
	err   error
}

// resolve completes a pending what-if preview at most once. Callers run on
// the actor goroutine; the buffered result channel lets the caller disappear
// concurrently without blocking engine shutdown.
func (pr *previewRoute) resolve(res previewResult) {
	if !pr.resolved {
		pr.resolved = true
		pr.result <- res
	}
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
		connectResults:           make(chan connectResult),
		ready:                    make(chan error, 1),
		done:                     make(chan struct{}),
		events:                   newObserver[Event](cfg.eventBuffer),
		keyed:                    make(map[int]*route),
		singletons:               make(map[string]*route),
		orders:                   make(map[int64]*orderRoute),
		previews:                 make(map[int64]*previewRoute),
		pendingOrderWrites:       make(map[transportWriteKey]int64),
		execDeliveries:           make(map[string]*execDelivery),
		unknownInboundSeen:       make(map[int]struct{}),
		malformedInboundSeen:     make(map[int]struct{}),
		dirtySingletons:          make(map[string]uint64),
		recentHistoricalRequests: make(map[string]time.Time),
		nextReqID:                1,
		snapshot: Snapshot{
			State: StateDisconnected,
		},
	}
	e.lifetimeCtx, e.cancelLifetime = context.WithCancel(context.Background())
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
		select {
		case err := <-e.ready:
			if err != nil {
				return nil, err
			}
			return e, nil
		default:
		}
		e.Close()
		return nil, errors.Join(context.Cause(ctx), e.lastConnectionError())
	}
}

func (e *engine) Close() {
	select {
	case <-e.done:
		return
	default:
	}
	e.enqueue(func() {
		e.closeEngine(ErrClosed, nil)
	})
	<-e.done
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

func (e *engine) rememberConnectionError(err error) {
	if err == nil {
		return
	}
	e.connectErrMu.Lock()
	e.lastConnectErr = err
	e.connectErrMu.Unlock()
}

func (e *engine) lastConnectionError() error {
	e.connectErrMu.Lock()
	defer e.connectErrMu.Unlock()
	return e.lastConnectErr
}

func (e *engine) setState(next State, code int, message string, err error, apiErrors ...*APIError) {
	if e.closed && next != StateClosed {
		return
	}
	e.snapshotMu.Lock()
	prev := e.snapshot.State
	e.snapshot.State = next
	connSeq := e.snapshot.ConnectionSeq
	e.snapshotMu.Unlock()

	event := Event{
		At:            time.Now().UTC(),
		State:         next,
		Previous:      prev,
		ConnectionSeq: connSeq,
		Code:          code,
		Message:       message,
		Err:           err,
	}
	if len(apiErrors) != 0 {
		event.APIError = apiErrors[0]
	}
	e.events.EmitLatest(event)
}

// emitEvent publishes an informational session event (e.g. farm-status
// or market-data warnings) without changing session state.
func (e *engine) emitEvent(code int, message string) {
	e.emitSessionEvent(code, message, nil)
}

func (e *engine) emitAPIEvent(msg codec.APIError) {
	apiErr, _ := errors.AsType[*APIError](e.apiErr("", msg))
	e.snapshotMu.RLock()
	state := e.snapshot.State
	connSeq := e.snapshot.ConnectionSeq
	e.snapshotMu.RUnlock()
	e.events.EmitLatest(Event{
		At:            time.Now().UTC(),
		State:         state,
		Previous:      state,
		ConnectionSeq: connSeq,
		Code:          msg.Code,
		Message:       msg.Message,
		APIError:      apiErr,
	})
}

func (e *engine) apiNotice(op OpKind, msg codec.APIError) *APIError {
	notice, _ := errors.AsType[*APIError](e.apiErr(op, msg))
	return notice
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
	tr := e.transport
	payload, err := codec.Encode(e.serverVersion, msg)
	if err != nil {
		return err
	}
	err = tr.Send(ctx, payload)
	return normalizeSendErr(ctx, tr, err)
}

func (e *engine) sendTrackedContext(ctx context.Context, msg codec.Message) (transportWriteKey, error) {
	if e.transport == nil {
		return transportWriteKey{}, ErrNotReady
	}
	tr := e.transport
	payload, err := codec.Encode(e.serverVersion, msg)
	if err != nil {
		return transportWriteKey{}, err
	}
	id, err := tr.SendTracked(ctx, payload)
	if err != nil {
		return transportWriteKey{}, normalizeSendErr(ctx, tr, err)
	}
	return transportWriteKey{transport: tr, id: id}, nil
}

func normalizeSendErr(ctx context.Context, tr *transport.Conn, err error) error {
	if err == nil {
		return nil
	}
	if cause := context.Cause(ctx); cause != nil {
		return cause
	}
	if errors.Is(err, transport.ErrSendQueueFull) {
		return ErrInterrupted
	}
	select {
	case <-tr.Stopping():
		return errors.Join(ErrInterrupted, err)
	default:
		return err
	}
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
		if _, conflict := e.orders[int64(id)]; conflict {
			continue
		}
		if _, conflict := e.previews[int64(id)]; !conflict {
			return id
		}
	}
}

func (e *engine) allocOrderID() int64 {
	for {
		id := max(e.snapshot.NextValidID, e.orderIDHighWater+1)
		e.updateSnapshot(func(s *Snapshot) {
			s.NextValidID = id + 1
		})
		if _, conflict := e.keyed[int(id)]; conflict {
			continue
		}
		if _, conflict := e.orders[id]; conflict {
			continue
		}
		if _, conflict := e.previews[id]; conflict {
			continue
		}
		e.orderIDHighWater = id
		return id
	}
}

func (e *engine) observeOrderID(id int64) {
	if id <= e.orderIDHighWater {
		return
	}
	e.orderIDHighWater = id
	e.updateSnapshot(func(s *Snapshot) {
		if s.NextValidID <= id {
			s.NextValidID = id + 1
		}
	})
}

func (e *engine) observeNextValidID(id int64) {
	next := max(id, e.orderIDHighWater+1, e.snapshot.NextValidID)
	e.updateSnapshot(func(s *Snapshot) {
		s.NextValidID = next
	})
}

func (e *engine) connectionSeq() uint64 {
	e.snapshotMu.RLock()
	defer e.snapshotMu.RUnlock()
	return e.snapshot.ConnectionSeq
}

func (e *engine) isReady() bool {
	if !e.hasReadyTransport() {
		return false
	}
	return len(e.resumePending) == 0 && !e.resumeWaiting
}

// hasReadyTransport reports whether the current physical connection can
// admit protocol teardown. Ordinary new work additionally waits for the
// reconnect resume barrier in isReady.
func (e *engine) hasReadyTransport() bool {
	if e.transport == nil {
		return false
	}
	if e.retiringTransport == e.transport {
		return false
	}
	select {
	case <-e.transport.Stopping():
		return false
	default:
	}
	e.snapshotMu.RLock()
	state := e.snapshot.State
	e.snapshotMu.RUnlock()
	return state == StateReady || state == StateDegraded
}

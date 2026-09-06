package ibkr

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/transport"
)

type engine struct {
	cfg config

	cmds           chan func()
	incoming       chan actorInput
	transportErr   chan transportLoss
	connectResults chan connectResult
	ready          chan error
	done           chan struct{}
	events         *observer[Event]

	waitMu         sync.Mutex
	waitErr        error
	stopLogger     func()
	connectErrMu   sync.Mutex
	lastConnectErr error

	// snapshot is actor-owned; all writes and unguarded reads run on the actor
	// goroutine. snapshotMu exists solely so Session can read a consistent copy
	// off-actor; setState and updateSnapshot take its write lock for that reader.
	snapshotMu sync.RWMutex
	snapshot   Snapshot

	transport           *transport.Conn
	retiringTransport   *transport.Conn
	transportRetireErr  error
	transportRouteErr   error
	transportGeneration uint64
	poisonedGeneration  uint64
	serverVersion       int

	keyed      map[int]*route
	singletons map[string]*route
	orders     map[int64]*orderRoute
	previews   map[int64]*previewRoute
	// pendingOrderWrites owns the admission-to-write gap for order frames.
	// Completion is handled on the actor before transport loss, so an
	// unwritten order cannot be mistaken for one IBKR received.
	pendingOrderWrites map[transportWriteKey]int64
	// execDeliveries retains order-owned fills and unmatched fees until their
	// handle closes. Both IDs and pending versions have client-wide caps;
	// no timer can determine when a fee has become irrelevant.
	execDeliveries   map[string]*execDelivery
	pendingOrderFees int
	// Exclusions are an eviction-only FIFO, separately capped at the same
	// limit. Foreign fills must not spend the lossless ownership budget.
	excludedOrderExecutions map[string]int
	excludedOrderExecFIFO   []string
	excludedOrderExecNext   int
	// executionEvents is a passive, client-wide observer. It owns no Gateway
	// request and sees each execution-detail and commission callback before
	// query correlation or per-order deduplication.
	executionEvents *executionEventRoute
	// unknownInboundSeen records msg ids already reported as unknown, so a
	// hot misdecoded feed logs and emits once instead of per frame.
	unknownInboundSeen   map[int]struct{}
	malformedInboundSeen map[int]struct{}
	dirtySingletons      map[string]uint64
	readySetups          []*readySetup
	historicalWaits      map[*historicalWait]struct{}

	// Request IDs keep their independent low sequence but never enter the
	// historical order interval conservatively bounded by orderIDLowWater and
	// snapshot.NextValidID. The low-water mark includes order IDs allocated or
	// observed by this engine, including orders owned by another client ID.
	nextReqID          int
	requestIDHighWater int64
	orderIDLowWater    int64

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

	// The selection belongs to the client; its application belongs to a socket.
	marketDataType           MarketDataType
	marketDataTypeGeneration uint64
}

type resumeRoute struct {
	reqID int
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

type actorInputKind uint8

const (
	actorInputDecoded actorInputKind = iota
	actorInputTransportWrite
)

// actorInput keeps decoded callbacks and tracked write completions on one FIFO
// channel. generation binds decoded callbacks to their physical transport;
// write completions remain actor-control input when that generation is
// poisoned.
type actorInput struct {
	kind         actorInputKind
	writeOutcome transport.WriteOutcome
	generation   uint64
	message      any
	writeKey     transportWriteKey
}

type bootstrapState struct {
	serverInfo    bool
	managed       bool
	nextValidID   bool
	readyReported bool
}

const (
	// Supported versions are the live-validated protocol train from the first
	// fully current v2 layout through the supported API 10.50.01 layouts.
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

var errRequestIDExhausted = errors.New("request ID space exhausted")

type route struct {
	opKind           OpKind // keyed routes only; singleton dispatch never reads it
	subscription     bool
	resume           ResumePolicy
	request          codec.OutboundMessage
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
	permID           int64
	handle           *OrderHandle
	cleanup          func()
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
	logger, stopLogger := newAsyncLogger(cfg.logger)
	cfg.logger = logger

	e := &engine{
		cfg:                      cfg,
		stopLogger:               stopLogger,
		cmds:                     make(chan func(), 256),
		incoming:                 make(chan actorInput, 256),
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
		e.closeEngine(ErrClosed, ErrClosed, nil)
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
	return cloneSnapshot(e.snapshot)
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	if snapshot.ManagedAccounts != nil {
		snapshot.ManagedAccounts = append([]string(nil), snapshot.ManagedAccounts...)
	}
	return snapshot
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
	if prev != next {
		e.snapshot.TransitionSeq++
	}
	e.snapshot.State = next
	snapshot := cloneSnapshot(e.snapshot)
	e.snapshotMu.Unlock()

	event := Event{
		At:            time.Now().UTC(),
		State:         snapshot.State,
		Previous:      prev,
		ConnectionSeq: snapshot.ConnectionSeq,
		TransitionSeq: snapshot.TransitionSeq,
		Snapshot:      snapshot,
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
	apiErr := e.apiErr("", msg)
	snapshot := cloneSnapshot(e.snapshot)
	e.events.EmitLatest(Event{
		At:            time.Now().UTC(),
		State:         snapshot.State,
		Previous:      snapshot.State,
		ConnectionSeq: snapshot.ConnectionSeq,
		TransitionSeq: snapshot.TransitionSeq,
		Snapshot:      snapshot,
		Code:          msg.Code,
		Message:       msg.Message,
		APIError:      apiErr,
	})
}

func (e *engine) apiNotice(op OpKind, msg codec.APIError) *APIError {
	return e.apiErr(op, msg)
}

func (e *engine) emitSessionEvent(code int, message string, err error) {
	snapshot := cloneSnapshot(e.snapshot)
	e.events.EmitLatest(Event{
		At:            time.Now().UTC(),
		State:         snapshot.State,
		Previous:      snapshot.State,
		ConnectionSeq: snapshot.ConnectionSeq,
		TransitionSeq: snapshot.TransitionSeq,
		Snapshot:      snapshot,
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

func (e *engine) send(msg codec.OutboundMessage) error {
	// Transport queue-capacity admission never waits for space: a full queue
	// returns transport.ErrSendQueueFull.
	return e.sendContext(context.Background(), msg)
}

func (e *engine) sendContext(ctx context.Context, msg codec.OutboundMessage) error {
	if e.transport == nil {
		return ErrNotReady
	}
	tr := e.transport
	payload, err := codec.Encode(e.serverVersion, msg)
	if err != nil {
		return &ProtocolError{Direction: "outbound", Message: fmt.Sprintf("%T", msg), Err: err}
	}
	err = tr.Send(ctx, payload)
	return normalizeSendErr(ctx, tr, err)
}

func (e *engine) sendTrackedContext(ctx context.Context, msg codec.OutboundMessage) (transportWriteKey, error) {
	if e.transport == nil {
		return transportWriteKey{}, ErrNotReady
	}
	tr := e.transport
	payload, err := codec.Encode(e.serverVersion, msg)
	if err != nil {
		return transportWriteKey{}, &ProtocolError{Direction: "outbound", Message: fmt.Sprintf("%T", msg), Err: err}
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
		return interrupted(err)
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

func (e *engine) allocReqID() (int, error) {
	normalize := func(id int64) (int64, bool) {
		for range 2 {
			if id < 1 || id > maxWireOrderID {
				id = 1
			}
			if e.orderIDLowWater > 0 && id >= e.orderIDLowWater && id < e.snapshot.NextValidID {
				id = e.snapshot.NextValidID
				continue
			}
			return id, true
		}
		return 0, false
	}

	id, ok := normalize(int64(e.nextReqID))
	if !ok {
		return 0, fmt.Errorf("ibkr: allocate request ID: %w", errRequestIDExhausted)
	}
	start := id
	for {
		_, keyed := e.keyed[int(id)]
		_, order := e.orders[id]
		_, preview := e.previews[id]
		if !keyed && !order && !preview {
			next := id + 1
			if next > maxWireOrderID {
				next = 1
			}
			e.nextReqID = int(next)
			e.observeRequestID(int(id))
			return int(id), nil
		}

		id, ok = normalize(id + 1)
		if !ok {
			return 0, fmt.Errorf("ibkr: allocate request ID: %w", errRequestIDExhausted)
		}
		if id == start {
			return 0, fmt.Errorf("ibkr: allocate request ID: %w", errRequestIDExhausted)
		}
	}
}

func (e *engine) allocOrderID() (int64, error) {
	for {
		id := max(e.snapshot.NextValidID, e.requestIDHighWater+1)
		if err := validateOrderID("OrderID", id, false); err != nil {
			return 0, err
		}
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
		e.observeOrderID(id)
		return id, nil
	}
}

func (e *engine) observeOrderID(id int64) {
	if id <= 0 {
		return
	}
	if e.orderIDLowWater == 0 || id < e.orderIDLowWater {
		e.orderIDLowWater = id
	}
	if id < e.snapshot.NextValidID {
		return
	}
	e.updateSnapshot(func(s *Snapshot) {
		s.NextValidID = id + 1
	})
}

func (e *engine) observeRequestID(id int) {
	if int64(id) > e.requestIDHighWater {
		e.requestIDHighWater = int64(id)
	}
}

func (e *engine) observeNextValidID(id int64) {
	next := max(id, e.snapshot.NextValidID)
	e.updateSnapshot(func(s *Snapshot) {
		s.NextValidID = next
	})
}

func (e *engine) connectionSeq() uint64 {
	return e.snapshot.ConnectionSeq
}

func (e *engine) isReady() bool {
	if !e.hasReadyTransport() {
		return false
	}
	return !e.marketDataTypePending() && len(e.resumePending) == 0 && !e.resumeWaiting
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
	state := e.snapshot.State
	return state == StateReady || state == StateDegraded
}

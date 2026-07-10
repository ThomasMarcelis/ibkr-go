package ibkr

import (
	"bytes"
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/internal/transport"
)

func TestSubscriptionWaitReportsCancellationAdmissionFailure(t *testing.T) {
	t.Parallel()

	e, peer := newObservedMarketDataEngine(t)
	e.nextReqID = 20621
	// The request is the exact live-derived IBM CFD reroute case frozen in
	// TestQuoteRouteFollowsLiveRerouteAndFreezesResumeRequest.
	sub := installObservedQuoteRoute(t, e, QuoteRequest{
		Contract: Contract{Symbol: "IBM", SecType: SecTypeCFD, Exchange: "SMART", Currency: "USD"},
	})
	_ = readObservedFrame(t, peer)
	fillTransportQueue(t, e.transport, peer)

	if err := sub.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	(<-e.cmds)()

	waitErr := sub.Wait()
	cancelErr, ok := errors.AsType[*SubscriptionCancelError](waitErr)
	if !ok {
		t.Fatalf("Wait() error = %T %v, want *SubscriptionCancelError", waitErr, waitErr)
	}
	if cancelErr.OpKind != OpQuotes || !errors.Is(cancelErr, ErrInterrupted) {
		t.Fatalf("cancellation error = %+v, want quotes wrapping ErrInterrupted", cancelErr)
	}
	if text := cancelErr.Error(); !strings.Contains(text, "recycle the client connection before subscribing again") {
		t.Fatalf("cancellation error = %q, want exact recovery guidance", text)
	}
	if sub.Err() != waitErr {
		t.Fatalf("Err() = %v, want Wait() error %v", sub.Err(), waitErr)
	}
	if IsRetryable(waitErr) {
		t.Fatal("cancellation uncertainty is retryable; replacement could duplicate the live stream")
	}
	if _, ok := e.keyed[20621]; ok {
		t.Fatal("failed cancellation left the local quote route active")
	}

	var closed SubscriptionStateEvent
	for event := range sub.Lifecycle() {
		if event.Kind == SubscriptionClosed {
			closed = event
		}
	}
	if closed.Err != waitErr || closed.Retryable {
		t.Fatalf("closed lifecycle = %+v, want exact non-retryable cancellation error", closed)
	}
}

func TestSlowQuoteConsumerPreservesCancellationAdmissionFailure(t *testing.T) {
	t.Parallel()

	e, peer := newObservedMarketDataEngine(t)
	e.nextReqID = 1
	sub := installObservedQuoteRoute(t, e, QuoteRequest{
		Contract: Contract{Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD"},
	}, WithQueueSize(1))
	_ = readObservedFrame(t, peer)
	fillTransportQueue(t, e.transport, peer)

	// captures/20260415T162742Z-api_duplicate_quote_subscriptions_aapl,
	// server_version 200, events.jsonl sha256 prefix 84f1e78a18616e0f.
	// These are the capture's first two updates for request 1; the full outbound
	// queue is deterministic fault injection for the cancellation-admission edge.
	e.handleIncoming(decodeOne(t, []byte("81\x001\x000.01\x009c0001\x004\x00")))
	e.handleIncoming(decodeOne(t, []byte("58\x001\x001\x003\x00")))

	waitErr := sub.Wait()
	if !errors.Is(waitErr, ErrSlowConsumer) || !errors.Is(waitErr, ErrInterrupted) {
		t.Fatalf("Wait() error = %v, want slow-consumer and cancellation-admission causes", waitErr)
	}
	cancelErr, ok := errors.AsType[*SubscriptionCancelError](waitErr)
	if !ok || cancelErr.OpKind != OpQuotes {
		t.Fatalf("Wait() error = %T %v, want joined quotes *SubscriptionCancelError", waitErr, waitErr)
	}
	if sub.Err() != waitErr {
		t.Fatalf("Err() = %v, want exact Wait() error %v", sub.Err(), waitErr)
	}
	if IsRetryable(waitErr) {
		t.Fatal("joined slow-consumer cancellation uncertainty is retryable")
	}
	if _, ok := e.keyed[1]; ok {
		t.Fatal("failed slow-consumer cancellation left the quote route active")
	}

	var closed SubscriptionStateEvent
	for event := range sub.Lifecycle() {
		if event.Kind == SubscriptionClosed {
			closed = event
		}
	}
	if closed.Err != waitErr || closed.Retryable {
		t.Fatalf("closed lifecycle = %+v, want exact non-retryable joined error", closed)
	}
}

func TestActorSlowConsumerCancelsWhilePublicCloseWaitsOnFullCommandQueue(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := &engine{
			cmds: make(chan func(), 1),
			done: make(chan struct{}),
		}
		active := true
		cancelCalls := 0
		var sub *Subscription[int]
		actorCancel := func() {
			if !active {
				return
			}
			active = false
			cancelCalls++
			sub.closeWithErr(nil)
		}
		sub = newEngineSubscription[int](subscriptionConfig{
			buffer:       1,
			slowConsumer: SlowConsumerClose,
		}, e, actorCancel)
		e.cmds <- func() {}

		closeResult := make(chan error, 1)
		go func() { closeResult <- sub.Close() }()
		synctest.Wait()
		select {
		case err := <-closeResult:
			t.Fatalf("Close() returned with full command queue: %v", err)
		default:
		}

		if !sub.emit(1) || sub.emit(2) {
			t.Fatal("emits did not trigger actor-owned slow-consumer cancellation")
		}
		if err := sub.Wait(); err != ErrSlowConsumer {
			t.Fatalf("Wait() = %v, want exact ErrSlowConsumer", err)
		}
		if active || cancelCalls != 1 {
			t.Fatalf("actor cancellation active=%t calls=%d, want false/1", active, cancelCalls)
		}

		<-e.cmds // Admit the public cancellation that already owns cancelOnce.
		synctest.Wait()
		if err := <-closeResult; err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		(<-e.cmds)()
		if cancelCalls != 1 {
			t.Fatalf("queued public cancellation calls = %d, want actor-owned callback once", cancelCalls)
		}
	})
}

func TestSubscriptionCancelSkipsUnresumedReconnectRoute(t *testing.T) {
	t.Parallel()

	e, peer := newObservedMarketDataEngine(t)
	e.snapshot.State = StateHandshaking

	if err := e.cancelSubscription(OpQuotes, codec.CancelQuote{ReqID: 20621}); err != nil {
		t.Fatalf("cancel during replacement handshake = %v, want clean local detach", err)
	}
	fence := codec.ReqMarketDataType{DataType: int(MarketDataLive)}
	if err := e.sendContext(context.Background(), fence); err != nil {
		t.Fatalf("enqueue transport fence: %v", err)
	}
	wantFence, err := codec.Encode(206, fence)
	if err != nil {
		t.Fatalf("encode transport fence: %v", err)
	}
	if got := readObservedFrame(t, peer); !bytes.Equal(got, wantFence) {
		t.Fatalf("first frame after unresumed cancellation = %x, want fence %x", got, wantFence)
	}
}

func TestClosedSingletonCannotCancelReplacement(t *testing.T) {
	t.Parallel()

	e, oldPeer := newObservedMarketDataEngine(t)
	oldSub := installObservedPositionsRoute(t, e)
	_ = readObservedFrame(t, oldPeer)
	oldRoute := e.singletons[singletonPositions]
	if oldRoute == nil {
		t.Fatal("old positions route was not installed")
	}

	// Reproduce a terminal route close that does not consume the public
	// handle's independent Close call. A later Close must not act on a new
	// route merely because it reuses the singleton key.
	delete(e.singletons, singletonPositions)
	oldRoute.close(ErrInterrupted)
	if err := oldSub.Wait(); !errors.Is(err, ErrInterrupted) {
		t.Fatalf("old Wait() error = %v, want ErrInterrupted", err)
	}

	oldTransport := e.transport
	if err := oldTransport.Close(); err != nil {
		t.Fatalf("close old transport: %v", err)
	}
	if err := oldTransport.Wait(); err != nil {
		t.Fatalf("wait for old transport: %v", err)
	}

	replacementPeer, replacementClient := net.Pipe()
	e.transport = transport.New(replacementClient, e.cfg.logger, 0)
	t.Cleanup(func() { _ = replacementPeer.Close() })
	replacementSub := installObservedPositionsRoute(t, e)
	_ = readObservedFrame(t, replacementPeer)
	replacementRoute := e.singletons[singletonPositions]
	if replacementRoute == nil || replacementRoute == oldRoute {
		t.Fatalf("replacement route = %p, old route = %p", replacementRoute, oldRoute)
	}

	if err := oldSub.Close(); err != nil {
		t.Fatalf("old Close() error = %v", err)
	}
	(<-e.cmds)()
	if got := e.singletons[singletonPositions]; got != replacementRoute {
		t.Fatalf("singleton route after stale Close() = %p, want replacement %p", got, replacementRoute)
	}
	select {
	case <-replacementSub.Done():
		t.Fatalf("stale Close() terminated replacement: %v", replacementSub.Err())
	default:
	}

	fence := codec.ReqMarketDataType{DataType: int(MarketDataLive)}
	if err := e.sendContext(context.Background(), fence); err != nil {
		t.Fatalf("enqueue transport fence: %v", err)
	}
	wantFence, err := codec.Encode(206, fence)
	if err != nil {
		t.Fatalf("encode transport fence: %v", err)
	}
	if got := readObservedFrame(t, replacementPeer); !bytes.Equal(got, wantFence) {
		t.Fatalf("first frame after stale singleton Close() = %x, want fence %x", got, wantFence)
	}
}

func TestDisplayGroupUpdateAfterCloseReturnsErrClosed(t *testing.T) {
	t.Parallel()

	e, peer := newObservedMarketDataEngine(t)
	handle := installObservedDisplayGroup(t, e)
	_ = readObservedFrame(t, peer)

	if err := handle.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	(<-e.cmds)()
	_ = readObservedFrame(t, peer)
	if err := handle.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	e.snapshot.State = StateHandshaking

	updateErr := make(chan error, 1)
	go func() { updateErr <- handle.Update(context.Background(), "265598@SMART") }()
	(<-e.cmds)()
	if err := <-updateErr; !errors.Is(err, ErrClosed) {
		t.Fatalf("Update() error = %v, want ErrClosed", err)
	}
	fence := codec.ReqMarketDataType{DataType: int(MarketDataLive)}
	if err := e.sendContext(context.Background(), fence); err != nil {
		t.Fatalf("enqueue transport fence: %v", err)
	}
	wantFence, err := codec.Encode(206, fence)
	if err != nil {
		t.Fatalf("encode transport fence: %v", err)
	}
	if got := readObservedFrame(t, peer); !bytes.Equal(got, wantFence) {
		t.Fatalf("first frame after closed Update() = %x, want fence %x", got, wantFence)
	}
}

func TestDisplayGroupUpdateWaitsForReconnectReady(t *testing.T) {
	t.Parallel()

	e, peer := newObservedMarketDataEngine(t)
	handle := installObservedDisplayGroup(t, e)
	_ = readObservedFrame(t, peer)
	e.snapshot.State = StateHandshaking

	updateErr := make(chan error, 1)
	go func() { updateErr <- handle.Update(context.Background(), "265598@SMART") }()
	(<-e.cmds)() // Capture the exact owned route without waiting for readiness.
	(<-e.cmds)() // Park the revalidated send in the existing readiness queue.
	select {
	case err := <-updateErr:
		t.Fatalf("Update() returned before reconnect readiness: %v", err)
	default:
	}

	e.snapshot.State = StateReady
	e.flushReadySetups()
	if err := <-updateErr; err != nil {
		t.Fatalf("Update() after reconnect readiness = %v", err)
	}
	want, err := codec.Encode(206, codec.UpdateDisplayGroupRequest{ReqID: 0, ContractInfo: "265598@SMART"})
	if err != nil {
		t.Fatalf("encode display group update: %v", err)
	}
	if got := readObservedFrame(t, peer); !bytes.Equal(got, want) {
		t.Fatalf("display group update after reconnect = %x, want %x", got, want)
	}
}

func TestDisplayGroupUpdateRechecksRouteAfterReconnectWait(t *testing.T) {
	t.Parallel()

	e, peer := newObservedMarketDataEngine(t)
	handle := installObservedDisplayGroup(t, e)
	_ = readObservedFrame(t, peer)
	e.snapshot.State = StateHandshaking

	updateErr := make(chan error, 1)
	go func() { updateErr <- handle.Update(context.Background(), "265598@SMART") }()
	(<-e.cmds)()
	(<-e.cmds)()
	if err := handle.Close(); err != nil {
		t.Fatalf("Close() while update waits = %v", err)
	}
	(<-e.cmds)()
	if err := handle.Wait(); err != nil {
		t.Fatalf("Wait() after close during reconnect = %v", err)
	}

	e.snapshot.State = StateReady
	e.flushReadySetups()
	if err := <-updateErr; !errors.Is(err, ErrClosed) {
		t.Fatalf("Update() after owned route closed = %v, want ErrClosed", err)
	}
	fence := codec.ReqMarketDataType{DataType: int(MarketDataLive)}
	if err := e.sendContext(context.Background(), fence); err != nil {
		t.Fatalf("enqueue transport fence: %v", err)
	}
	wantFence, err := codec.Encode(206, fence)
	if err != nil {
		t.Fatalf("encode transport fence: %v", err)
	}
	if got := readObservedFrame(t, peer); !bytes.Equal(got, wantFence) {
		t.Fatalf("first frame after route closed during update wait = %x, want fence %x", got, wantFence)
	}
}

func TestAccountSnapshotRetainsRowsWhenCleanupFails(t *testing.T) {
	t.Parallel()

	cancelErr := &SubscriptionCancelError{OpKind: OpAccountSummary, Err: ErrInterrupted}
	var sub *Subscription[AccountSummaryUpdate]
	sub = newSubscription[AccountSummaryUpdate](subscriptionConfig{
		buffer:          1,
		slowConsumer:    SlowConsumerClose,
		collectSnapshot: true,
	}, func() {
		sub.closeWithErr(cancelErr)
	})
	sub.expectSnapshot()
	// captures/20260405T215025Z-account_summary_snapshot, server_version 200;
	// retained exactly in testdata/transcripts/grounded_account_summary.txt.
	sub.emit(AccountSummaryUpdate{Value: AccountValue{
		Account: "DU9000001", Tag: "NetLiquidation", Value: "68000.00", Currency: "EUR",
	}})
	sub.emitState(SubscriptionStateEvent{Kind: SubscriptionSnapshotComplete})

	values, err := collectSnapshotAndClose(context.Background(), sub, func(update AccountSummaryUpdate) (AccountValue, bool) {
		return update.Value, true
	})
	if len(values) != 1 || values[0].Account != "DU9000001" || values[0].Value != "68000.00" || values[0].Currency != "EUR" {
		t.Fatalf("snapshot values = %+v, want retained live-derived row", values)
	}
	if err != cancelErr {
		t.Fatalf("snapshot cleanup error = %v, want %v", err, cancelErr)
	}
}

func installObservedPositionsRoute(t *testing.T, e *engine) *Subscription[PositionUpdate] {
	t.Helper()
	result := make(chan struct {
		sub *Subscription[PositionUpdate]
		err error
	}, 1)
	go func() {
		sub, err := e.SubscribePositions(context.Background())
		result <- struct {
			sub *Subscription[PositionUpdate]
			err error
		}{sub: sub, err: err}
	}()
	(<-e.cmds)()
	out := <-result
	if out.err != nil {
		t.Fatalf("SubscribePositions() error = %v", out.err)
	}
	return out.sub
}

func installObservedDisplayGroup(t *testing.T, e *engine) *DisplayGroupHandle {
	t.Helper()
	result := make(chan struct {
		handle *DisplayGroupHandle
		err    error
	}, 1)
	go func() {
		handle, err := e.SubscribeDisplayGroup(context.Background(), 1)
		result <- struct {
			handle *DisplayGroupHandle
			err    error
		}{handle: handle, err: err}
	}()
	(<-e.cmds)()
	out := <-result
	if out.err != nil {
		t.Fatalf("SubscribeDisplayGroup() error = %v", out.err)
	}
	return out.handle
}

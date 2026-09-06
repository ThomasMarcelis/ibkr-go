package ibkr

import (
	"bytes"
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/transport"
)

func TestCanceledSingletonRetiresTransportBeforeReplacement(t *testing.T) {
	t.Parallel()

	e, peer := newObservedMarketDataEngine(t)
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	go func() {
		_, err := e.CurrentTime(ctx)
		first <- err
	}()
	(<-e.cmds)()
	_ = readObservedFrame(t, peer)
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled CurrentTime() error = %v, want context.Canceled", err)
	}
	(<-e.cmds)()
	finishObservedRetirement(t, e)
	if _, ok := e.singletons[singletonCurrentTime]; ok {
		t.Fatal("canceled CurrentTime() left its unkeyed route active")
	}
	if e.transport != nil {
		t.Fatal("canceled CurrentTime() retained the ambiguous transport generation")
	}
	if e.snapshot.State != StateReconnecting {
		t.Fatalf("state = %s, want Reconnecting", e.snapshot.State)
	}
}

func TestOpenOrdersCloseDuringRefreshRetiresResponseOwner(t *testing.T) {
	t.Parallel()

	e, peer := newObservedMarketDataEngine(t)
	result := make(chan *OpenOrdersSubscription, 1)
	go func() {
		sub, err := e.SubscribeOpenOrders(context.Background(), OpenOrdersScopeAll)
		if err != nil {
			t.Errorf("SubscribeOpenOrders: %v", err)
		}
		result <- sub
	}()
	(<-e.cmds)()
	sub := <-result
	_ = readObservedFrame(t, peer)
	e.handleIncoming(codec.OpenOrderEnd{})
	<-sub.Events() // Started
	<-sub.Events() // SnapshotComplete

	refreshed := make(chan error, 1)
	go func() { refreshed <- sub.Refresh(context.Background()) }()
	(<-e.cmds)()
	if err := <-refreshed; err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	_ = readObservedFrame(t, peer)

	sub.Close()
	(<-e.cmds)()
	if e.retiringTransport != e.transport {
		t.Fatal("closing an in-flight request-ID-less refresh did not retire its response owner")
	}
	if err := sub.Wait(); err != nil {
		t.Fatalf("Wait after explicit Close = %v, want clean local detach", err)
	}
	finishObservedRetirement(t, e)
}

func TestClosedUnkeyedStreamCannotBeReusedBeforeNewTransportGeneration(t *testing.T) {
	t.Parallel()

	e, oldPeer := newObservedMarketDataEngine(t)
	first := installObservedPositionsRoute(t, e)
	_ = readObservedFrame(t, oldPeer)
	e.handleIncoming(codec.PositionEnd{})
	<-first.Events() // Started
	<-first.Events() // SnapshotComplete

	first.Close()
	(<-e.cmds)()
	_ = readObservedFrame(t, oldPeer) // cancel positions
	if !e.singletonGenerationDirty(singletonPositions) {
		t.Fatal("closed request-ID-less stream did not dirty its transport generation")
	}

	type result struct {
		sub *Subscription[Position]
		err error
	}
	replacement := make(chan result, 1)
	go func() {
		sub, err := e.SubscribePositions(context.Background())
		replacement <- result{sub: sub, err: err}
	}()
	(<-e.cmds)()
	if e.retiringTransport != e.transport {
		t.Fatal("same-generation replacement did not retire the ambiguous transport")
	}
	if _, ok := e.singletons[singletonPositions]; ok {
		t.Fatal("same-generation replacement installed a route before rotation")
	}
	select {
	case out := <-replacement:
		t.Fatalf("replacement completed before transport rotation: %+v", out)
	default:
	}

	finishObservedRetirement(t, e)
	newPeer, newClient := net.Pipe()
	t.Cleanup(func() { _ = newPeer.Close() })
	e.connectAttemptID = 7
	e.bootstrap = bootstrapState{} // startConnect normally resets this before the result.
	e.handleConnectResult(connectResult{
		attempt: 7, reconnect: true, conn: newClient, serverVersion: 225,
	})
	// A buffered callback from the retired generation has no replacement owner.
	e.handleIncoming(codec.Position{Account: "DU9000001", Position: "999"})
	e.bootstrap.managed = true
	e.bootstrap.nextValidID = true
	e.maybeReady()

	out := <-replacement
	if out.err != nil || out.sub == nil {
		t.Fatalf("replacement after generation rotation = %+v", out)
	}
	_ = readObservedFrame(t, newPeer)
	select {
	case update := <-out.sub.Events():
		if update.Value.Position.String() == "999" {
			t.Fatal("retired-generation position reached the replacement subscription")
		}
	default:
	}
}

func TestSendRejectedByStoppingTransportIsInterrupted(t *testing.T) {
	t.Parallel()

	e, peer := newObservedMarketDataEngine(t)
	if err := e.transport.Close(); err != nil {
		t.Fatalf("Close transport: %v", err)
	}
	err := e.sendContext(context.Background(), codec.ReqMarketDataType{DataType: int(MarketDataDelayed)})
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("sendContext on stopping transport = %v, want ErrInterrupted", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = e.sendContext(ctx, codec.ReqMarketDataType{DataType: int(MarketDataDelayed)})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled sendContext on stopping transport = %v, want context.Canceled", err)
	}
	_ = peer.Close()
}

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

	sub.Close()
	(<-e.cmds)()
	finishObservedRetirement(t, e)

	waitErr := sub.Wait()
	cancelErr, ok := errors.AsType[*SubscriptionCancelError](waitErr)
	if !ok {
		t.Fatalf("Wait() error = %T %v, want *SubscriptionCancelError", waitErr, waitErr)
	}
	if cancelErr.OpKind != OpQuotes || !errors.Is(cancelErr, ErrInterrupted) {
		t.Fatalf("cancellation error = %+v, want quotes wrapping ErrInterrupted", cancelErr)
	}
	if text := cancelErr.Error(); !strings.Contains(text, "owning connection generation retired") {
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
	if e.transport != nil || e.snapshot.State != StateReconnecting {
		t.Fatalf("failed cancellation transport/state = %v/%s, want nil/Reconnecting", e.transport, e.snapshot.State)
	}

	for range sub.Events() {
	}
}

func TestSubscriptionCancellationAdmissionFailureTerminatesReconnectOffClient(t *testing.T) {
	t.Parallel()

	e, peer := newObservedMarketDataEngine(t)
	e.cfg.reconnect = ReconnectOff
	e.nextReqID = 20621
	sub := installObservedQuoteRoute(t, e, QuoteRequest{
		Contract: Contract{Symbol: "IBM", SecType: SecTypeCFD, Exchange: "SMART", Currency: "USD"},
	})
	_ = readObservedFrame(t, peer)
	sibling := installObservedQuoteRoute(t, e, QuoteRequest{
		Contract: Contract{Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD"},
	}, WithResumePolicy(ResumeNever))
	_ = readObservedFrame(t, peer)
	fillTransportQueue(t, e.transport, peer)

	sub.Close()
	(<-e.cmds)()
	finishObservedRetirement(t, e)

	if _, ok := errors.AsType[*SubscriptionCancelError](sub.Wait()); !ok {
		t.Fatalf("Wait() error = %T %v, want *SubscriptionCancelError", sub.Err(), sub.Err())
	}
	siblingErr := sibling.Wait()
	if !errors.Is(siblingErr, ErrResumeRequired) || !IsRetryable(siblingErr) {
		t.Fatalf("sibling Wait() error = %v, want retryable ErrResumeRequired", siblingErr)
	}
	if _, ok := errors.AsType[*SubscriptionCancelError](siblingErr); ok {
		t.Fatalf("sibling Wait() error = %v, must not contain another route's cancellation failure", siblingErr)
	}
	waitErr := e.Wait()
	if _, ok := errors.AsType[*SubscriptionCancelError](waitErr); !ok {
		t.Fatalf("client Wait() error = %T %v, want *SubscriptionCancelError", waitErr, waitErr)
	}
	if !e.closed || e.transport != nil || e.snapshot.State != StateClosed {
		t.Fatalf("failed cancellation client closed/transport/state = %t/%v/%s, want true/nil/Closed", e.closed, e.transport, e.snapshot.State)
	}
	select {
	case <-e.done:
	default:
		t.Fatal("ReconnectOff client remained running after its connection was retired")
	}
	var closedEvent Event
	for event := range e.SessionEvents() {
		if event.State == StateClosed {
			closedEvent = event
		}
	}
	if _, ok := errors.AsType[*SubscriptionCancelError](closedEvent.Err); !ok {
		t.Fatalf("StateClosed event error = %T %v, want *SubscriptionCancelError", closedEvent.Err, closedEvent.Err)
	}
}

func TestSubscriptionCancellationAdmissionFailurePreservesResumeAutoSibling(t *testing.T) {
	t.Parallel()

	e, peer := newObservedMarketDataEngine(t)
	e.nextReqID = 20621
	canceled := installObservedQuoteRoute(t, e, QuoteRequest{
		Contract: Contract{Symbol: "IBM", SecType: SecTypeCFD, Exchange: "SMART", Currency: "USD"},
	})
	_ = readObservedFrame(t, peer)
	survivor := installObservedQuoteRoute(t, e, QuoteRequest{
		Contract: Contract{Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD"},
	}, WithResumePolicy(ResumeAuto))
	_ = readObservedFrame(t, peer)
	fillTransportQueue(t, e.transport, peer)

	canceled.Close()
	(<-e.cmds)()
	finishObservedRetirement(t, e)

	if _, ok := errors.AsType[*SubscriptionCancelError](canceled.Wait()); !ok {
		t.Fatalf("canceled Wait() error = %T %v, want *SubscriptionCancelError", canceled.Err(), canceled.Err())
	}
	if _, ok := e.keyed[20621]; ok {
		t.Fatal("canceled route survived connection retirement")
	}
	route := e.keyed[20622]
	if route == nil || !route.gapped {
		t.Fatalf("ResumeAuto sibling route = %+v, want retained gapped route", route)
	}
	select {
	case <-survivor.Done():
		t.Fatalf("ResumeAuto sibling terminated: %v", survivor.Err())
	default:
	}
	if e.transport != nil || e.snapshot.State != StateReconnecting {
		t.Fatalf("failed cancellation transport/state = %v/%s, want nil/Reconnecting", e.transport, e.snapshot.State)
	}

	route.close(ErrClosed)
	e.deleteKeyedRoute(20622)
}

func TestSubscriptionCancellationAdmissionFailureDoesNotContaminateSibling(t *testing.T) {
	t.Parallel()

	e, peer := newObservedMarketDataEngine(t)
	e.nextReqID = 20621
	canceled := installObservedQuoteRoute(t, e, QuoteRequest{
		Contract: Contract{Symbol: "IBM", SecType: SecTypeCFD, Exchange: "SMART", Currency: "USD"},
	})
	_ = readObservedFrame(t, peer)
	sibling := installObservedQuoteRoute(t, e, QuoteRequest{
		Contract: Contract{Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD"},
	}, WithResumePolicy(ResumeNever))
	_ = readObservedFrame(t, peer)
	fillTransportQueue(t, e.transport, peer)

	canceled.Close()
	(<-e.cmds)()
	finishObservedRetirement(t, e)

	if _, ok := errors.AsType[*SubscriptionCancelError](canceled.Wait()); !ok {
		t.Fatalf("canceled Wait() error = %T %v, want *SubscriptionCancelError", canceled.Err(), canceled.Err())
	}
	siblingErr := sibling.Wait()
	if !errors.Is(siblingErr, ErrResumeRequired) || !IsRetryable(siblingErr) {
		t.Fatalf("sibling Wait() error = %v, want retryable ErrResumeRequired", siblingErr)
	}
	if _, ok := errors.AsType[*SubscriptionCancelError](siblingErr); ok {
		t.Fatalf("sibling Wait() error = %v, must not contain another route's cancellation failure", siblingErr)
	}
	if _, ok := e.keyed[20622]; ok {
		t.Fatal("ResumeNever sibling survived connection retirement")
	}

	event := <-e.SessionEvents()
	if _, ok := errors.AsType[*SubscriptionCancelError](event.Err); !ok {
		t.Fatalf("session retirement error = %T %v, want *SubscriptionCancelError", event.Err, event.Err)
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

	// captures/20260824T202345Z-api_duplicate_quote_subscriptions_aapl,
	// server_version 225, events.jsonl sha256
	// 1fbb60beec41483729e2f9e7c96b1bfdd89649810ffdc5e7e4a4077c1eb8b290.
	// These are the capture's first two updates for request 1; the full outbound
	// queue is deterministic fault injection for the cancellation-admission edge.
	e.handleIncoming(decodeOne(t, decodeHexBytes(t, "0000011908011204302e30311a0639633030303120042a08302e3030303030313208302e303030303031")))
	e.handleIncoming(decodeOne(t, decodeHexBytes(t, "0000010208011003")))

	waitErr := sub.Wait()
	finishObservedRetirement(t, e)
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
	if e.transport != nil || e.snapshot.State != StateReconnecting {
		t.Fatalf("slow-consumer transport/state = %v/%s, want nil/Reconnecting", e.transport, e.snapshot.State)
	}

	for range sub.Events() {
	}
}

func finishObservedRetirement(t *testing.T, e *engine) {
	t.Helper()
	tr := e.retiringTransport
	if tr == nil {
		return
	}
	err := tr.Wait()
	e.handleTransportLoss(transportLoss{transport: tr, err: err})
}

func TestSubscriptionCancelRouteErrPreservesEveryBatchCause(t *testing.T) {
	t.Parallel()

	first := errors.New("first cancellation interruption")
	second := errors.New("second cancellation interruption")
	direct := &SubscriptionCancelError{OpKind: OpQuotes, Err: first}
	if got, ok := subscriptionCancelRouteErr(direct); !ok || got != first {
		t.Fatalf("direct route cause = %T %v, %t; want exact first cause", got, got, ok)
	}

	batch := errors.Join(
		direct,
		&SubscriptionCancelError{OpKind: OpMarketDepth, Err: second},
	)
	got, ok := subscriptionCancelRouteErr(batch)
	if !ok || !errors.Is(got, first) || !errors.Is(got, second) {
		t.Fatalf("batch route cause = %v, %t; want both underlying causes", got, ok)
	}
	if _, ok := errors.AsType[*SubscriptionCancelError](got); ok {
		t.Fatalf("batch route cause = %v, must not retain cancellation wrappers", got)
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
			buffer: 1,
		}, e, actorCancel)
		e.cmds <- func() {}

		closed := make(chan struct{})
		go func() {
			sub.Close()
			close(closed)
		}()
		synctest.Wait()
		select {
		case <-closed:
			t.Fatal("Close() returned with full command queue")
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
		<-closed
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
	staleRoute := &route{generation: e.transportGeneration - 1}

	if err := e.cancelRouteSubscription(staleRoute, OpQuotes, codec.CancelQuote{ReqID: 20621}); err != nil {
		t.Fatalf("cancel during replacement handshake = %v, want clean local detach", err)
	}
	fence := codec.ReqMarketDataType{DataType: int(MarketDataLive)}
	if err := e.sendContext(context.Background(), fence); err != nil {
		t.Fatalf("enqueue transport fence: %v", err)
	}
	wantFence, err := codec.Encode(e.serverVersion, fence)
	if err != nil {
		t.Fatalf("encode transport fence: %v", err)
	}
	if got := readObservedFrame(t, peer); !bytes.Equal(got, wantFence) {
		t.Fatalf("first frame after unresumed cancellation = %x, want fence %x", got, wantFence)
	}
}

func TestSubscriptionCancelUsesRouteResumeOwnership(t *testing.T) {
	t.Parallel()

	e, peer := newObservedMarketDataEngine(t)
	e.nextReqID = 801
	resumed := installObservedQuoteRoute(t, e, QuoteRequest{Contract: Stock("AAPL")}, WithResumePolicy(ResumeAuto))
	_ = readObservedFrame(t, peer)
	pending := installObservedQuoteRoute(t, e, QuoteRequest{Contract: Stock("MSFT")}, WithResumePolicy(ResumeAuto))
	_ = readObservedFrame(t, peer)

	resumedRoute := e.keyed[801]
	pendingRoute := e.keyed[802]
	e.transportGeneration++
	resumedRoute.generation = e.transportGeneration
	// Same-generation pending models a data-lost 1101 replay on the existing
	// socket. Physical reconnect pending routes are also excluded by their old
	// generation.
	pendingRoute.generation = e.transportGeneration
	e.resumePending = []resumeRoute{{reqID: 802, route: pendingRoute}}
	e.resumeWaiting = true

	resumed.Close()
	(<-e.cmds)()
	wantCancel, err := codec.Encode(e.serverVersion, codec.CancelQuote{ReqID: 801})
	if err != nil {
		t.Fatalf("encode resumed cancel: %v", err)
	}
	if got := readObservedFrame(t, peer); !bytes.Equal(got, wantCancel) {
		t.Fatalf("resumed route cancel = %x, want %x", got, wantCancel)
	}
	if err := resumed.Wait(); err != nil {
		t.Fatalf("resumed route close: %v", err)
	}

	pending.Close()
	(<-e.cmds)()
	if err := pending.Wait(); err != nil {
		t.Fatalf("pending route close: %v", err)
	}
	if e.retiringTransport != nil {
		t.Fatal("closing a pending resume route retired the replacement transport")
	}

	fence := codec.ReqMarketDataType{DataType: int(MarketDataLive)}
	if err := e.sendContext(context.Background(), fence); err != nil {
		t.Fatalf("send post-close fence: %v", err)
	}
	wantFence, err := codec.Encode(e.serverVersion, fence)
	if err != nil {
		t.Fatalf("encode post-close fence: %v", err)
	}
	if got := readObservedFrame(t, peer); !bytes.Equal(got, wantFence) {
		t.Fatalf("first frame after pending close = %x, want fence %x", got, wantFence)
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

	oldSub.Close()
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
	wantFence, err := codec.Encode(e.serverVersion, fence)
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

	handle.Close()
	(<-e.cmds)()
	_ = readObservedFrame(t, peer)
	if err := handle.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	e.snapshot.State = StateHandshaking

	updateErr := make(chan error, 1)
	go func() { updateErr <- handle.Update(context.Background(), "265598@SMART") }()
	if err := <-updateErr; !errors.Is(err, ErrClosed) {
		t.Fatalf("Update() error = %v, want ErrClosed", err)
	}
	fence := codec.ReqMarketDataType{DataType: int(MarketDataLive)}
	if err := e.sendContext(context.Background(), fence); err != nil {
		t.Fatalf("enqueue transport fence: %v", err)
	}
	wantFence, err := codec.Encode(e.serverVersion, fence)
	if err != nil {
		t.Fatalf("encode transport fence: %v", err)
	}
	if got := readObservedFrame(t, peer); !bytes.Equal(got, wantFence) {
		t.Fatalf("first frame after closed Update() = %x, want fence %x", got, wantFence)
	}
}

func TestDisplayGroupUpdateOnNilOrZeroHandleReturnsErrClosed(t *testing.T) {
	t.Parallel()

	for name, handle := range map[string]*DisplayGroupHandle{
		"nil":  nil,
		"zero": {},
	} {
		t.Run(name, func(t *testing.T) {
			if err := handle.Update(context.Background(), "265598@SMART"); !errors.Is(err, ErrClosed) {
				t.Fatalf("Update() error = %v, want ErrClosed", err)
			}
		})
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
	want, err := codec.Encode(e.serverVersion, codec.UpdateDisplayGroupRequest{ReqID: 1, ContractInfo: "265598@SMART"})
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
	handle.Close()
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
	wantFence, err := codec.Encode(e.serverVersion, fence)
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
	var sub *Subscription[AccountValue]
	sub = newSubscription[AccountValue](subscriptionConfig{
		buffer:          1,
		collectSnapshot: true,
	}, func() {
		sub.closeWithErr(cancelErr)
	})
	sub.expectSnapshot()
	// captures/20260824T202344Z-account_summary_snapshot, server_version 225,
	// events.jsonl SHA-256
	// 6f8ede19db82acc23de6bd988381d40b339af0270133fdcabceba33082f6c181.
	sub.emit(AccountValue{
		Account: "DU9000001", Tag: "NetLiquidation", Value: "36992.49", Currency: "EUR",
	})
	sub.emitState(StreamSnapshotComplete, 0, nil)

	values, err := collectSnapshotAndClose(context.Background(), sub, func(value AccountValue) (AccountValue, bool) {
		return value, true
	})
	if len(values) != 1 || values[0].Account != "DU9000001" || values[0].Value != "36992.49" || values[0].Currency != "EUR" {
		t.Fatalf("snapshot values = %+v, want retained live-derived row", values)
	}
	if err != cancelErr {
		t.Fatalf("snapshot cleanup error = %v, want %v", err, cancelErr)
	}
}

func installObservedPositionsRoute(t *testing.T, e *engine) *Subscription[Position] {
	t.Helper()
	result := make(chan struct {
		sub *Subscription[Position]
		err error
	}, 1)
	go func() {
		sub, err := e.SubscribePositions(context.Background())
		result <- struct {
			sub *Subscription[Position]
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

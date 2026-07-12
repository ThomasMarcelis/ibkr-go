package ibkr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
)

// Normal depth-row delivery in this file is anchored to
// 20260710T134721Z-sdk_sv206_market_data_readonly_retry: server_version 206,
// events.jsonl sha256
// 989563f9c4cad108e34058beac205c576a9ebdc0fffe03e421e829bca851e7de.
// Its raw message 212 decoded to request 20614, position 0, insert, bid,
// price 1.14248, and size 7500000. Malformed frames, omitted or reassigned
// request IDs, and invalid decimals below are structural fault injection. No
// malformed or invalid-decimal live Gateway frame is claimed.

func TestMalformedMarketDepthInboundClosesEveryDepthRoute(t *testing.T) {
	for _, msgID := range []int{protocol.InMarketDepth, protocol.InMarketDepthL2} {
		t.Run(fmt.Sprintf("msg_id_%d", msgID), func(t *testing.T) {
			e, peer := newObservedMarketDataEngine(t)
			var logs strings.Builder
			e.cfg.logger = slog.New(slog.NewTextHandler(&logs, nil))

			e.nextReqID = 301
			depths := make([]observedDepthSubscription, 0, 4)
			for _, smart := range []bool{false, true, false, true} {
				reqID := e.nextReqID
				sub := installObservedDepthRoute(t, e, observedDepthRequest(smart))
				_ = readObservedFrame(t, peer)
				depths = append(depths, observedDepthSubscription{reqID: reqID, smart: smart, sub: sub})
			}

			quoteReqID := e.nextReqID
			quote := installObservedQuoteRoute(t, e, QuoteRequest{Contract: Contract{
				Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD",
			}})
			_ = readObservedFrame(t, peer)

			firstCause := errors.New("first malformed depth frame")
			e.handleIncoming(codec.MalformedInbound{MsgID: msgID, Err: firstCause})

			assertObservedDepthCancels(t, e, peer, depths)
			for _, depth := range depths {
				assertMalformedDepthError(t, depth.sub.Wait(), msgID, firstCause)
			}
			if len(e.keyed) != 1 || e.keyed[quoteReqID] == nil || e.keyed[quoteReqID].opKind != OpQuotes {
				t.Fatalf("routes after malformed depth frame = %+v, want only quote request %d", e.keyed, quoteReqID)
			}
			select {
			case <-quote.Done():
				t.Fatalf("unrelated quote closed: %v", quote.Err())
			default:
			}
			select {
			case <-e.done:
				t.Fatal("malformed depth frame closed the session")
			default:
			}

			event := <-e.SessionEvents()
			assertMalformedDepthError(t, event.Err, msgID, firstCause)

			replacementReqID := e.nextReqID
			replacement := installObservedDepthRoute(t, e, observedDepthRequest(true))
			_ = readObservedFrame(t, peer)
			secondCause := errors.New("repeated malformed depth frame")
			e.handleIncoming(codec.MalformedInbound{MsgID: msgID, Err: secondCause})

			assertObservedDepthCancels(t, e, peer, []observedDepthSubscription{{
				reqID: replacementReqID, smart: true, sub: replacement,
			}})
			assertMalformedDepthError(t, replacement.Wait(), msgID, secondCause)
			if got := e.keyed[quoteReqID]; got == nil || got.opKind != OpQuotes {
				t.Fatalf("repeated malformed frame changed unrelated quote route: %+v", got)
			}
			select {
			case event := <-e.SessionEvents():
				t.Fatalf("repeated malformed msg_id emitted deduped session event: %+v", event)
			default:
			}
			if got := strings.Count(logs.String(), "dropping malformed inbound frame"); got != 1 {
				t.Fatalf("malformed-frame log count = %d, want 1; logs: %s", got, logs.String())
			}

			quote.Close()
			(<-e.cmds)()
			_ = readObservedFrame(t, peer)
			if err := quote.Wait(); err != nil {
				t.Fatalf("quote cleanup: %v", err)
			}
		})
	}
}

func TestMalformedMarketDepthInboundJoinsCancellationAdmissionFailures(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	e.nextReqID = 401

	depths := make([]observedDepthSubscription, 0, 2)
	for _, smart := range []bool{false, true} {
		reqID := e.nextReqID
		sub := installObservedDepthRoute(t, e, observedDepthRequest(smart))
		_ = readObservedFrame(t, peer)
		depths = append(depths, observedDepthSubscription{reqID: reqID, smart: smart, sub: sub})
	}
	fillTransportQueue(t, e.transport, peer)

	cause := errors.New("malformed market depth L2 frame")
	e.handleIncoming(codec.MalformedInbound{MsgID: protocol.InMarketDepthL2, Err: cause})

	for _, depth := range depths {
		waitErr := depth.sub.Wait()
		assertMalformedDepthError(t, waitErr, protocol.InMarketDepthL2, cause)
		cancelErr, ok := errors.AsType[*SubscriptionCancelError](waitErr)
		if !ok || cancelErr.OpKind != OpMarketDepth || !errors.Is(cancelErr, ErrInterrupted) {
			t.Fatalf("request %d error = %T %v, want joined market-depth cancellation failure", depth.reqID, waitErr, waitErr)
		}
		if IsRetryable(waitErr) {
			t.Fatalf("request %d malformed/cancellation uncertainty is retryable", depth.reqID)
		}
		if _, ok := e.keyed[depth.reqID]; ok {
			t.Fatalf("request %d remains active after terminal malformed frame", depth.reqID)
		}
	}
}

func TestUnattributableDepthRowClosesEveryDepthRoute(t *testing.T) {
	tests := []struct {
		name      string
		msgID     int
		collision bool
		message   func(*testing.T, int) any
	}{
		{
			name:  "L1 omitted protobuf request ID",
			msgID: protocol.InMarketDepth,
			message: func(t *testing.T, _ int) any {
				return omittedRequestIDDepthMessage(t, protocol.InMarketDepth)
			},
		},
		{
			name:  "L2 omitted protobuf request ID",
			msgID: protocol.InMarketDepthL2,
			message: func(t *testing.T, _ int) any {
				return omittedRequestIDDepthMessage(t, protocol.InMarketDepthL2)
			},
		},
		{
			name:      "L1 request ID owned by quote",
			msgID:     protocol.InMarketDepth,
			collision: true,
			message: func(_ *testing.T, reqID int) any {
				return codec.MarketDepthUpdate{ReqID: reqID, Price: "0"}
			},
		},
		{
			name:      "L2 request ID owned by quote",
			msgID:     protocol.InMarketDepthL2,
			collision: true,
			message: func(_ *testing.T, reqID int) any {
				return codec.MarketDepthL2Update{ReqID: reqID, Price: "0"}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e, peer := newObservedMarketDataEngine(t)
			e.nextReqID = 701

			depths := make([]observedDepthSubscription, 0, 2)
			for _, smart := range []bool{false, true} {
				reqID := e.nextReqID
				sub := installObservedDepthRoute(t, e, observedDepthRequest(smart))
				_ = readObservedFrame(t, peer)
				depths = append(depths, observedDepthSubscription{reqID: reqID, smart: smart, sub: sub})
			}

			quoteReqID := e.nextReqID
			quote := installObservedQuoteRoute(t, e, QuoteRequest{Contract: Contract{
				Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD",
			}})
			_ = readObservedFrame(t, peer)
			quoteRoute := e.keyed[quoteReqID]
			quoteHandleCalls := 0
			originalQuoteHandle := quoteRoute.handle
			quoteRoute.handle = func(message any, e *engine) {
				quoteHandleCalls++
				originalQuoteHandle(message, e)
			}

			reqID := -1
			if tc.collision {
				reqID = quoteReqID
			}
			e.handleIncoming(tc.message(t, reqID))

			assertObservedDepthCancels(t, e, peer, depths)
			for _, depth := range depths {
				assertUnattributableDepthError(t, depth.sub.Wait(), tc.msgID, reqID)
			}
			if got := e.keyed[quoteReqID]; got != quoteRoute {
				t.Fatalf("unrelated quote route = %p, want original %p", got, quoteRoute)
			}
			if quoteHandleCalls != 0 {
				t.Fatalf("unattributable depth row reached unrelated quote handler %d time(s)", quoteHandleCalls)
			}
			select {
			case <-quote.Done():
				t.Fatalf("unrelated quote closed: %v", quote.Err())
			default:
			}
			select {
			case <-e.done:
				t.Fatal("unattributable depth row closed the session")
			default:
			}

			quote.Close()
			(<-e.cmds)()
			_ = readObservedFrame(t, peer)
			if err := quote.Wait(); err != nil {
				t.Fatalf("quote cleanup: %v", err)
			}
		})
	}
}

func TestDepthRowConversionFailureCancelsMatchingRoute(t *testing.T) {
	tests := []struct {
		name        string
		smart       bool
		wantMessage string
		message     func(int) any
	}{
		{
			name:        "regular",
			wantMessage: "market depth price",
			message: func(reqID int) any {
				return codec.MarketDepthUpdate{ReqID: reqID, Price: "not-a-decimal"}
			},
		},
		{
			name:        "smart L2",
			smart:       true,
			wantMessage: "market depth l2 price",
			message: func(reqID int) any {
				return codec.MarketDepthL2Update{ReqID: reqID, Price: "not-a-decimal", IsSmartDepth: true}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e, peer := newObservedMarketDataEngine(t)
			e.nextReqID = 501
			sub := installObservedDepthRoute(t, e, observedDepthRequest(tc.smart))
			_ = readObservedFrame(t, peer)

			e.handleIncoming(tc.message(501))

			assertObservedDepthCancels(t, e, peer, []observedDepthSubscription{{
				reqID: 501, smart: tc.smart, sub: sub,
			}})
			if err := sub.Wait(); err == nil || !strings.Contains(err.Error(), tc.wantMessage) {
				t.Fatalf("Wait() error = %v, want market-depth decimal conversion failure", err)
			}
			if _, ok := e.keyed[501]; ok {
				t.Fatal("decimal conversion failure left depth route active")
			}
			select {
			case <-e.done:
				t.Fatal("depth conversion failure closed the session")
			default:
			}
		})
	}
}

func TestDepthRowConversionFailureJoinsCancellationAdmissionFailure(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	e.nextReqID = 601
	sub := installObservedDepthRoute(t, e, observedDepthRequest(true))
	_ = readObservedFrame(t, peer)
	fillTransportQueue(t, e.transport, peer)

	e.handleIncoming(codec.MarketDepthL2Update{
		ReqID: 601, Price: "not-a-decimal", IsSmartDepth: true,
	})

	waitErr := sub.Wait()
	if waitErr == nil || !strings.Contains(waitErr.Error(), "market depth l2 price") {
		t.Fatalf("Wait() error = %v, want market-depth L2 decimal conversion failure", waitErr)
	}
	cancelErr, ok := errors.AsType[*SubscriptionCancelError](waitErr)
	if !ok || cancelErr.OpKind != OpMarketDepth || !errors.Is(cancelErr, ErrInterrupted) {
		t.Fatalf("Wait() error = %T %v, want joined market-depth cancellation failure", waitErr, waitErr)
	}
	if IsRetryable(waitErr) {
		t.Fatal("depth conversion/cancellation uncertainty is retryable")
	}
	if _, ok := e.keyed[601]; ok {
		t.Fatal("failed cancellation admission left local depth route active")
	}
}

func TestLateDeletedDepthRouteDoesNotAffectReplacement(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	e.nextReqID = 20613
	old := installObservedDepthRoute(t, e, observedDepthRequest(false))
	_ = readObservedFrame(t, peer)
	old.Close()
	(<-e.cmds)()
	assertObservedDepthCancels(t, e, peer, []observedDepthSubscription{{
		reqID: 20613, sub: old,
	}})
	if err := old.Wait(); err != nil {
		t.Fatalf("old depth cleanup: %v", err)
	}

	replacement := installObservedDepthRoute(t, e, observedDepthRequest(false))
	_ = readObservedFrame(t, peer)

	// Only the request ID is changed from the normal captured row to model a
	// late callback for the deleted route; this is not claimed as a live late row.
	late := capturedSV206DepthUpdate()
	late.ReqID = 20613
	e.handleIncoming(late)
	if got := e.keyed[20614]; got == nil || got.opKind != OpMarketDepth {
		t.Fatalf("replacement route after late old row = %+v", got)
	}
	select {
	case <-replacement.Done():
		t.Fatalf("late old row closed replacement: %v", replacement.Err())
	default:
	}

	e.handleIncoming(capturedSV206DepthUpdate())
	assertCapturedSV206DepthRow(t, nextObservedDepthRow(t, replacement))

	replacement.Close()
	(<-e.cmds)()
	assertObservedDepthCancels(t, e, peer, []observedDepthSubscription{{
		reqID: 20614, sub: replacement,
	}})
	if err := replacement.Wait(); err != nil {
		t.Fatalf("replacement depth cleanup: %v", err)
	}
}

func TestDepthConversionFailureLeavesSiblingLive(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	e.nextReqID = 20613
	target := installObservedDepthRoute(t, e, observedDepthRequest(true))
	_ = readObservedFrame(t, peer)
	sibling := installObservedDepthRoute(t, e, observedDepthRequest(false))
	_ = readObservedFrame(t, peer)

	// Invalid decimal content is structural conversion-failure injection; the
	// valid sibling row below is the exact captured raw-212 decode.
	e.handleIncoming(codec.MarketDepthL2Update{
		ReqID: 20613, Price: "not-a-decimal", IsSmartDepth: true,
	})
	assertObservedDepthCancels(t, e, peer, []observedDepthSubscription{{
		reqID: 20613, smart: true, sub: target,
	}})
	if err := target.Wait(); err == nil || !strings.Contains(err.Error(), "market depth l2 price") {
		t.Fatalf("target Wait() error = %v, want decimal conversion failure", err)
	}
	if got := e.keyed[20614]; got == nil || got.opKind != OpMarketDepth {
		t.Fatalf("sibling route after target conversion failure = %+v", got)
	}
	select {
	case <-sibling.Done():
		t.Fatalf("target conversion failure closed sibling: %v", sibling.Err())
	default:
	}

	e.handleIncoming(capturedSV206DepthUpdate())
	assertCapturedSV206DepthRow(t, nextObservedDepthRow(t, sibling))

	sibling.Close()
	(<-e.cmds)()
	assertObservedDepthCancels(t, e, peer, []observedDepthSubscription{{
		reqID: 20614, sub: sibling,
	}})
	if err := sibling.Wait(); err != nil {
		t.Fatalf("sibling depth cleanup: %v", err)
	}
}

func TestQueuedDepthCloseAfterMalformedTeardownIsNoOp(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	e.nextReqID = 20614
	sub := installObservedDepthRoute(t, e, observedDepthRequest(true))
	_ = readObservedFrame(t, peer)

	sub.Close() // Queue public cancellation without running its actor callback.
	cause := errors.New("structural malformed market depth frame")
	e.handleIncoming(codec.MalformedInbound{MsgID: protocol.InMarketDepth, Err: cause})

	assertObservedDepthCancels(t, e, peer, []observedDepthSubscription{{
		reqID: 20614, smart: true, sub: sub,
	}})
	assertMalformedDepthError(t, sub.Wait(), protocol.InMarketDepth, cause)
	if _, ok := e.keyed[20614]; ok {
		t.Fatal("malformed teardown left depth route active")
	}

	(<-e.cmds)() // The queued public callback no longer owns a route.
	fence := codec.ReqMarketDataType{DataType: int(MarketDataLive)}
	if err := e.sendContext(context.Background(), fence); err != nil {
		t.Fatalf("send post-close fence: %v", err)
	}
	wantFence, err := codec.Encode(e.serverVersion, fence)
	if err != nil {
		t.Fatalf("encode post-close fence: %v", err)
	}
	if got := readObservedFrame(t, peer); !bytes.Equal(got, wantFence) {
		t.Fatalf("first frame after queued callback = %x, want fence %x", got, wantFence)
	}
	assertMalformedDepthError(t, sub.Err(), protocol.InMarketDepth, cause)
}

func TestMarketDepthRerouteResendFailureJoinsCancellationFailure(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	e.nextReqID = 20622
	sub := installObservedDepthRoute(t, e, MarketDepthRequest{
		Contract: Contract{Symbol: "IBM", SecType: SecTypeCFD, Exchange: "SMART", Currency: "USD"},
		NumRows:  5, IsSmartDepth: true,
	})
	_ = readObservedFrame(t, peer)
	fillTransportQueue(t, e.transport, peer)

	// The reroute is the existing exact sv206 live-derived frame. The full
	// outbound queue is deterministic admission-failure injection.
	reroute, err := codec.Decode(206, []byte("\x00\x00\x00\x5c20622\x008314\x00SMART\x00"))
	if err != nil {
		t.Fatalf("decode market-depth reroute: %v", err)
	}
	e.handleIncoming(reroute)

	waitErr := sub.Wait()
	if waitErr == nil || !strings.Contains(waitErr.Error(), "reroute market depth request 20622") {
		t.Fatalf("Wait() error = %v, want reroute resend failure", waitErr)
	}
	cancelErr, ok := errors.AsType[*SubscriptionCancelError](waitErr)
	if !ok || cancelErr.OpKind != OpMarketDepth || !errors.Is(cancelErr, ErrInterrupted) {
		t.Fatalf("Wait() error = %T %v, want joined market-depth cancellation failure", waitErr, waitErr)
	}
	if _, ok := e.keyed[20622]; ok {
		t.Fatal("reroute resend failure left depth route active")
	}
}

type observedDepthSubscription struct {
	reqID int
	smart bool
	sub   *Subscription[DepthRow]
}

func observedDepthRequest(smart bool) MarketDepthRequest {
	return MarketDepthRequest{
		Contract: Contract{
			Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD",
		},
		NumRows: 5, IsSmartDepth: smart,
	}
}

func assertObservedDepthCancels(t *testing.T, e *engine, peer net.Conn, depths []observedDepthSubscription) {
	t.Helper()

	want := make(map[string]int, len(depths))
	for _, depth := range depths {
		payload, err := codec.Encode(e.serverVersion, codec.CancelMarketDepth{
			ReqID: depth.reqID, IsSmartDepth: depth.smart,
		})
		if err != nil {
			t.Fatalf("encode depth cancel for request %d: %v", depth.reqID, err)
		}
		want[string(payload)]++
	}
	for range depths {
		payload := readObservedFrame(t, peer)
		if want[string(payload)] == 0 {
			t.Fatalf("unexpected depth cancel payload %x", payload)
		}
		want[string(payload)]--
	}
	for payload, count := range want {
		if count != 0 {
			t.Fatalf("depth cancel payload %x count = %d, want 0", payload, count)
		}
	}
}

func assertMalformedDepthError(t *testing.T, err error, msgID int, cause error) {
	t.Helper()
	protocolErr, ok := errors.AsType[*ProtocolError](err)
	if !ok || protocolErr.Direction != "inbound" || protocolErr.Message != fmt.Sprintf("msg_id %d", msgID) {
		t.Fatalf("error = %T %v, want inbound ProtocolError for msg_id %d", err, err, msgID)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("error = %v, want malformed cause %v", err, cause)
	}
}

func assertUnattributableDepthError(t *testing.T, err error, msgID, reqID int) {
	t.Helper()
	protocolErr, ok := errors.AsType[*ProtocolError](err)
	if !ok || protocolErr.Direction != "inbound" ||
		!strings.Contains(protocolErr.Message, fmt.Sprintf("msg_id %d", msgID)) ||
		!strings.Contains(protocolErr.Message, fmt.Sprintf("req_id %d", reqID)) {
		t.Fatalf("error = %T %v, want inbound ProtocolError for msg_id %d req_id %d", err, err, msgID, reqID)
	}
	if !strings.Contains(err.Error(), "unattributable market depth row") {
		t.Fatalf("error = %v, want unattributable-depth cause", err)
	}
}

func omittedRequestIDDepthMessage(t *testing.T, msgID int) any {
	t.Helper()
	// This is the same valid field-2 empty nested message frozen by
	// TestMarketDataProto206OfficialOmissionDefaults in internal/codec.
	payload, err := protocol.EncodeProtobufEnvelope(206, msgID, []byte{0x12, 0})
	if err != nil {
		t.Fatalf("encode omitted-request-id depth envelope: %v", err)
	}
	messages, err := codec.DecodeBatch(206, payload)
	if err != nil {
		t.Fatalf("decode omitted-request-id depth envelope: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("omitted-request-id depth messages = %d, want 1", len(messages))
	}
	return messages[0]
}

func capturedSV206DepthUpdate() codec.MarketDepthUpdate {
	return codec.MarketDepthUpdate{
		ReqID: 20614, Position: 0, Operation: 0, Side: 1,
		Price: "1.14248", Size: "7500000",
	}
}

func nextObservedDepthRow(t *testing.T, sub *Subscription[DepthRow]) DepthRow {
	t.Helper()
	for {
		select {
		case event, ok := <-sub.Events():
			if !ok {
				t.Fatalf("depth events closed: %v", sub.Err())
			}
			if event.Kind == StreamData {
				return event.Value
			}
		default:
			t.Fatal("depth row was not emitted synchronously")
			return DepthRow{}
		}
	}
}

func assertCapturedSV206DepthRow(t *testing.T, row DepthRow) {
	t.Helper()
	if row.Position != 0 || row.Operation != DepthInsert || row.Side != BookBid ||
		row.Price.String() != "1.14248" || row.Size == nil || row.Size.String() != "7500000" {
		t.Fatalf("depth row = %+v, want captured position 0 insert bid 1.14248 x 7500000", row)
	}
}

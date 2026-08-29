package ibkr

import (
	"bytes"
	"encoding/base64"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/transport"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
)

// The source row is the first sanitized position callback from
// captures/20260824T195734Z-positions_snapshot at server_version 225,
// events.jsonl SHA-256
// a1b1c8fdda7af5634f462b11c62f63200f7a1125a3c0c5419f0945dee99c3919.
// Removing its final byte is deterministic transport fault injection; the
// capture itself contained a complete callback.
func capturedMalformedPosition(t *testing.T) codec.MalformedInbound {
	t.Helper()
	payload := capturedPositionPayload(t)
	messages, err := codec.DecodeBatch(225, payload[:len(payload)-1])
	if err != nil || len(messages) != 1 {
		t.Fatalf("decode truncated captured position = %v, messages=%d", err, len(messages))
	}
	malformed, ok := messages[0].(codec.MalformedInbound)
	if !ok || malformed.MsgID != protocol.InPositionData {
		t.Fatalf("decoded message = %#v, want malformed position", messages[0])
	}
	return malformed
}

func capturedPositionPayload(t *testing.T) []byte {
	t.Helper()
	payload, err := base64.StdEncoding.DecodeString("AAABBQoJRFU5MDAwMDAxEjEI6anfFRIETUVMSRoDU1RLKQAAAAAAAAAAQgZOQVNEQVFSA1VTRFoETUVMSWIDTk1TGgExIY/C9ShctJhA")
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func TestMalformedRegisteredCallbackPoisonsWholeGeneration(t *testing.T) {
	e, _ := newObservedMarketDataEngine(t)
	e.cfg.reconnect = ReconnectOff
	e.serverVersion = 225
	e.transportGeneration = 7
	oldTransport := e.transport

	positionHandled := 0
	positionResult := make(chan error, 1)
	siblingHandled := 0
	siblingResult := make(chan error, 1)
	e.singletons[singletonPositions] = &route{
		generation: e.transportGeneration,
		handle: func(any, *engine) {
			positionHandled++
		},
		close: func(err error) { positionResult <- err },
	}
	e.keyed[41] = &route{
		generation: e.transportGeneration,
		handle: func(any, *engine) {
			siblingHandled++
		},
		close: func(err error) { siblingResult <- err },
	}
	handle := e.bindOrderHandle(47, Contract{ConID: 265598, Exchange: "SMART"}, 0)
	writeKey := transportWriteKey{transport: oldTransport, id: 9}
	e.trackOrderWrite(47, writeKey)

	e.handleActorInput(actorInput{
		kind:       actorInputDecoded,
		generation: 7, message: capturedMalformedPosition(t),
	})
	// Write completions are actor-control input, not decoded broker input. A
	// completion already queued behind the malformed callback must still drain
	// so an admitted order is not misclassified as unwritten.
	e.handleActorInput(actorInput{
		kind:         actorInputTransportWrite,
		writeKey:     writeKey,
		writeOutcome: transport.WriteCompleteLocal,
	})
	// A valid row and snapshot end were already buffered behind the malformed
	// callback. Neither may make a corrupt partial snapshot look complete.
	e.handleActorInput(actorInput{
		kind:       actorInputDecoded,
		generation: 7,
		message:    codec.Position{Account: "DU9000001", Contract: codec.Contract{Symbol: "MELI"}, Position: "1"},
	})
	e.handleActorInput(actorInput{
		kind:       actorInputDecoded,
		generation: 7, message: codec.PositionEnd{},
	})
	e.handleActorInput(actorInput{
		kind:       actorInputDecoded,
		generation: 7, message: codec.TickPrice{ReqID: 41, TickType: 1, Price: "311.19"},
	})

	if positionHandled != 0 || siblingHandled != 0 {
		t.Fatalf("poisoned generation dispatched position=%d sibling=%d callbacks", positionHandled, siblingHandled)
	}
	select {
	case event := <-handle.Events():
		if event.Lifecycle == nil || event.Lifecycle.Kind != OrderStarted {
			t.Fatalf("post-poison write event = %+v, want OrderStarted", event)
		}
	default:
		t.Fatal("post-poison tracked write completion was discarded")
	}
	e.handleTransportLoss(transportLoss{transport: oldTransport})

	for name, err := range map[string]error{
		"positions snapshot": <-positionResult,
		"sibling route":      <-siblingResult,
	} {
		if !errors.Is(err, ErrInterrupted) {
			t.Errorf("%s error = %v, want ErrInterrupted", name, err)
		}
		protocolErr, ok := errors.AsType[*ProtocolError](err)
		if !ok || protocolErr.Direction != "inbound" || protocolErr.Message != "msg_id 61" {
			t.Errorf("%s error = %T %v, want inbound position ProtocolError", name, err, err)
		}
		if IsRetryable(err) {
			t.Errorf("IsRetryable(%s error) = true, want false", name)
		}
	}
}

func TestMalformedGenerationPumpDropsBufferedTail(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	e.cfg.reconnect = ReconnectOff
	e.attachTransport(e.transport)

	positions := installObservedPositionsRoute(t, e)
	_ = readObservedFrame(t, peer)
	if event := <-positions.Events(); event.Kind != StreamStarted {
		t.Fatalf("positions first event = %s, want %s", event.Kind, StreamStarted)
	}

	quotes := installObservedQuoteRoute(t, e, QuoteRequest{Contract: Stock("AAPL")}, WithResumePolicy(ResumeNever))
	_ = readObservedFrame(t, peer)
	if event := <-quotes.Events(); event.Kind != StreamStarted {
		t.Fatalf("quotes first event = %s, want %s", event.Kind, StreamStarted)
	}

	malformed := capturedPositionPayload(t)
	malformed = malformed[:len(malformed)-1]
	frames := [][]byte{
		malformed,
		capturedPositionPayload(t),
		liveCapturedFrame(t, "AAAABAAAAQY="),
		liveCapturedFrame(t, "AAAAFgAAAMkIARBIGfYoXI/ClXNAIgEwKAA="),
	}
	for i, payload := range frames {
		if err := wire.WriteFrame(peer, payload); err != nil {
			t.Fatalf("write captured inbound frame %d: %v", i, err)
		}
	}
	if err := peer.Close(); err != nil {
		t.Fatal(err)
	}

	var loss transportLoss
	select {
	case loss = <-e.transportErr:
	case <-time.After(time.Second):
		t.Fatal("transport pumps did not publish loss after buffered inbound frames")
	}
	e.drainIncoming()
	e.handleTransportLoss(loss)

	for name, sub := range map[string]interface {
		Wait() error
	}{"positions": positions, "quotes": quotes} {
		err := sub.Wait()
		if !errors.Is(err, ErrInterrupted) {
			t.Errorf("%s error = %v, want ErrInterrupted", name, err)
		}
		protocolErr, ok := errors.AsType[*ProtocolError](err)
		if !ok || protocolErr.Direction != "inbound" || protocolErr.Message != "msg_id 61" {
			t.Errorf("%s error = %T %v, want inbound position ProtocolError", name, err, err)
		}
		if IsRetryable(err) {
			t.Errorf("IsRetryable(%s error) = true, want false", name)
		}
	}
	for event := range positions.Events() {
		if event.Kind == StreamData || event.Kind == StreamSnapshotComplete {
			t.Errorf("poisoned positions tail emitted %s", event.Kind)
		}
	}
	for event := range quotes.Events() {
		if event.Kind == StreamData {
			t.Errorf("poisoned sibling tick emitted data: %+v", event)
		}
	}

}

func TestMarketDataRouteCollisionPoisonsGeneration(t *testing.T) {
	// Capture 20260825T201807Z-live_cfd_quote_reroute_v201_positive,
	// events.jsonl SHA-256
	// ca8fbdf11d260066fb7cd1c3d60e6e44808a54bf6a8fc678f3597bd71a666f1c.
	// The unmodified sv225 callback claims request ID 1. Binding that ID to a
	// depth route is deterministic ownership fault injection over the captured
	// broker frame; no protocol payload is invented here.
	// The second case changes only the registered envelope ID from market-data
	// reroute (291) to market-depth reroute (292). That is deterministic
	// malformed-input injection over the same captured body and proves the
	// ownership boundary in both directions without fabricating a broker row.
	for _, test := range []struct {
		name    string
		frame   string
		owner   OpKind
		message string
	}{
		{name: "quote callback on depth route", frame: "AAAADwAAASMIARD6QBoFU01BUlQ=", owner: OpMarketDepth, message: "msg_id 91"},
		{name: "depth callback on quote route", frame: "AAAADwAAASQIARD6QBoFU01BUlQ=", owner: OpQuotes, message: "msg_id 92"},
	} {
		t.Run(test.name, func(t *testing.T) {
			message, err := codec.Decode(225, liveCapturedFrame(t, test.frame))
			if err != nil {
				t.Fatal(err)
			}
			keyed, ok := message.(codec.ReqIDer)
			if !ok || keyed.RequestID() != 1 {
				t.Fatalf("callback = %#v, want request-scoped reroute for request 1", message)
			}

			e, _ := newObservedMarketDataEngine(t)
			e.cfg.reconnect = ReconnectOff
			e.serverVersion = 225
			e.transportGeneration = 7
			oldTransport := e.transport

			ownerHandled := false
			ownerResult := make(chan error, 1)
			siblingHandled := false
			siblingResult := make(chan error, 1)
			e.keyed[1] = &route{
				opKind:     test.owner,
				generation: e.transportGeneration,
				handle:     func(any, *engine) { ownerHandled = true },
				close:      func(err error) { ownerResult <- err },
			}
			e.keyed[2] = &route{
				opKind:     OpQuotes,
				generation: e.transportGeneration,
				handle:     func(any, *engine) { siblingHandled = true },
				close:      func(err error) { siblingResult <- err },
			}

			e.handleActorInput(actorInput{kind: actorInputDecoded, generation: 7, message: message})
			e.handleActorInput(actorInput{
				kind:       actorInputDecoded,
				generation: 7,
				message:    codec.TickPrice{ReqID: 2, TickType: 1, Price: "311.19"},
			})
			if ownerHandled || siblingHandled {
				t.Fatalf("poisoned generation dispatched owner=%t sibling=%t callbacks", ownerHandled, siblingHandled)
			}
			e.handleTransportLoss(transportLoss{transport: oldTransport})

			for name, routeErr := range map[string]error{
				"owner route":   <-ownerResult,
				"sibling route": <-siblingResult,
			} {
				if !errors.Is(routeErr, ErrInterrupted) {
					t.Errorf("%s error = %v, want ErrInterrupted", name, routeErr)
				}
				protocolErr, ok := errors.AsType[*ProtocolError](routeErr)
				if !ok || protocolErr.Direction != "inbound" || protocolErr.Message != test.message {
					t.Errorf("%s error = %T %v, want inbound reroute ProtocolError", name, routeErr, routeErr)
				}
				if IsRetryable(routeErr) {
					t.Errorf("IsRetryable(%s error) = true, want false", name)
				}
			}
		})
	}
}

func TestMalformedGenerationResumesStreamOnlyOnFreshTransport(t *testing.T) {
	e, initialPeer := newObservedMarketDataEngine(t)
	e.cfg.reconnect = ReconnectAuto
	e.serverVersion = 225
	e.transportGeneration = 11
	e.nextReqID = 71
	t.Cleanup(func() {
		select {
		case <-e.done:
		default:
			close(e.done)
		}
	})

	sub := installObservedQuoteRoute(t, e, QuoteRequest{Contract: Stock("AAPL")}, WithResumePolicy(ResumeAuto))
	_ = readObservedFrame(t, initialPeer)
	if event := <-sub.Events(); event.Kind != StreamStarted {
		t.Fatalf("initial event = %s, want %s", event.Kind, StreamStarted)
	}
	oldTransport := e.transport
	oldGeneration := e.transportGeneration

	e.handleActorInput(actorInput{
		kind:       actorInputDecoded,
		generation: oldGeneration, message: capturedMalformedPosition(t),
	})
	e.handleTransportLoss(transportLoss{transport: oldTransport})
	if event := <-sub.Events(); event.Kind != StreamGap {
		t.Fatalf("post-malformed event = %s, want %s", event.Kind, StreamGap)
	}

	replacementPeer, replacementConn := net.Pipe()
	replacement := transport.New(replacementConn, e.cfg.logger, 0)
	t.Cleanup(func() {
		_ = replacement.Close()
		_ = replacementPeer.Close()
		_ = replacement.Wait()
	})
	e.transport = replacement
	e.transportGeneration++
	e.snapshot.ConnectionSeq++
	e.snapshot.State = StateReady
	e.resumeRoutes()

	wantResume, err := codec.Encode(225, e.keyed[71].request)
	if err != nil {
		t.Fatal(err)
	}
	if got := readObservedFrame(t, replacementPeer); !bytes.Equal(got, wantResume) {
		t.Fatalf("resumed request = %x, want %x", got, wantResume)
	}
	if event := <-sub.Events(); event.Kind != StreamResubscribed {
		t.Fatalf("fresh-generation event = %s, want %s", event.Kind, StreamResubscribed)
	}

	// Even after the replacement route exists, a callback bound to the retired
	// generation cannot reach it.
	e.handleActorInput(actorInput{
		kind:       actorInputDecoded,
		generation: oldGeneration,
		message:    codec.TickPrice{ReqID: 71, TickType: 1, Price: "310.00"},
	})
	select {
	case event := <-sub.Events():
		t.Fatalf("stale generation emitted event %+v", event)
	default:
	}

	e.handleActorInput(actorInput{
		kind:       actorInputDecoded,
		generation: e.transportGeneration,
		message:    codec.TickPrice{ReqID: 71, TickType: 1, Price: "311.19"},
	})
	if event := <-sub.Events(); event.Kind != StreamData || event.Value.Snapshot.Bid.String() != "311.19" {
		t.Fatalf("replacement callback event = %+v, want bid 311.19 data", event)
	}
}

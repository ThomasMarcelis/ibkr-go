package ibkr_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
)

// Replay freezes for the 2026-06-11 read-only live-Gateway session and
// reference captures (server_version 200). Each test replays a transcript
// derived frame-for-frame from a sanitized live capture; the capture dir and
// sha256 prefix are recorded in the transcript headers.

// TestCurrentTimeExplicitReplay freezes the explicit reqCurrentTime exchange
// (SESS-002): client [49, 1] is answered by [49, 1, 1781163646] after the
// bootstrap farm-status triple. Capture 20260611T074046Z-current_time.
func TestCurrentTimeExplicitReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "current_time_live.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ts, err := client.CurrentTime(ctx)
	if err != nil {
		t.Fatalf("CurrentTime() error = %v", err)
	}
	// 1781163646 epoch seconds = 2026-06-11 07:40:46 UTC (transcript header
	// shows 09:40:46 CEST).
	want := time.Unix(1781163646, 0).UTC()
	if !ts.Equal(want) {
		t.Errorf("CurrentTime() = %v, want %v", ts, want)
	}
	if got := client.Session().CurrentTime; !got.Equal(want) {
		t.Errorf("Session().CurrentTime = %v, want %v", got, want)
	}
}

// TestReqIDsReadOnlyRejectedReplay freezes the read-only-mode reqIds
// rejection (SESS-003): the live Gateway answered the capture driver's
// explicit reqIds with an unsolicited-shaped req_id=-1 code-321 error and
// never sent a next_valid_id refresh. No ibkr-go public API emits reqIds
// (order ids derive from the bootstrap next_valid_id), so the transcript
// replays the rejection as an unsolicited push and this test pins the
// client-side surface: the frame is dropped without a session event, a state
// change, or a NextValidID perturbation. Capture 20260611T074047Z-req_ids.
func TestReqIDsReadOnlyRejectedReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "req_ids_read_only.txt")
	defer client.Close()
	defer waitHost(t, host)

	// The transcript ends with the gateway disconnect. With ReconnectOff the
	// engine drains every received frame (farm statuses, then the code-321
	// rejection) before terminating, so Done() proves the 321 was processed.
	select {
	case <-client.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for engine shutdown after transcript disconnect")
	}

	codes := map[int]bool{}
	events := client.SessionEvents()
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				break
			}
			codes[ev.Code] = true
			continue
		case <-time.After(2 * time.Second):
		}
		break
	}

	// Farm-status events prove the frames preceding the rejection were
	// delivered and surfaced normally.
	for _, code := range []int{2104, 2106, 2158} {
		if !codes[code] {
			t.Errorf("farm-status code %d not observed in session events", code)
		}
	}
	// The req_id=-1 code-321 rejection has no public surface: it is neither
	// routed to a request nor emitted as a session event.
	if codes[321] {
		t.Error("code 321 surfaced as a session event, want the req_id=-1 rejection dropped")
	}
	if got := client.Session().NextValidID; got != 1 {
		t.Errorf("NextValidID = %d, want 1 (gateway sent no next_valid_id refresh)", got)
	}
	if got := client.Session().State; got != ibkr.StateClosed {
		t.Errorf("session state = %s, want %s (only the disconnect ended the session)", got, ibkr.StateClosed)
	}
}

// TestMatchingSymbolsPartialReplay freezes the partial-pattern symbol search
// (REF-002): Search("AA") returns the live Gateway's full 97-row
// symbolSamples reply, including derivative sec types and issuer-id BOND
// rows. Capture 20260611T074053Z-matching_symbols_partial.
func TestMatchingSymbolsPartialReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "matching_symbols_partial.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	symbols, err := client.Contracts().Search(ctx, "AA")
	if err != nil {
		t.Fatalf("Contracts().Search() error = %v", err)
	}
	if len(symbols) != 97 {
		t.Fatalf("symbols len = %d, want 97", len(symbols))
	}

	first := symbols[0]
	if first.ConID != 251962528 || first.Symbol != "AA" || first.SecType != ibkr.SecTypeStock {
		t.Errorf("symbols[0] = %+v, want AA STK conID 251962528", first)
	}
	if first.PrimaryExchange != "NYSE" || first.Currency != "USD" {
		t.Errorf("symbols[0] venue = %s/%s, want NYSE/USD", first.PrimaryExchange, first.Currency)
	}
	if want := []string{"CFD", "OPT", "IOPT", "WAR", "BAG"}; !reflect.DeepEqual(first.DerivativeSecTypes, want) {
		t.Errorf("symbols[0] derivative sec types = %v, want %v", first.DerivativeSecTypes, want)
	}
	if first.Description != "ALCOA CORP" {
		t.Errorf("symbols[0] description = %q, want ALCOA CORP", first.Description)
	}

	// Issuer-id BOND rows carry conID -1, no symbol, and no venue fields.
	bond := symbols[2]
	if bond.ConID != -1 || bond.SecType != ibkr.SecTypeBond || bond.Symbol != "" {
		t.Errorf("symbols[2] = %+v, want conID -1 BOND with empty symbol", bond)
	}
	if bond.Description != "Alcoa Nederland Holding BV" || bond.IssuerID != "e3231099" {
		t.Errorf("symbols[2] = %q / %q, want Alcoa Nederland Holding BV / e3231099", bond.Description, bond.IssuerID)
	}

	aapl := symbols[19]
	if aapl.ConID != 265598 || aapl.Symbol != "AAPL" || aapl.PrimaryExchange != "NASDAQ" || aapl.Description != "APPLE INC" {
		t.Errorf("symbols[19] = %+v, want AAPL 265598 NASDAQ APPLE INC", aapl)
	}

	last := symbols[96]
	if last.SecType != ibkr.SecTypeBond || last.Description != "AAA & Sons Enterprises Pvt Ltd" || last.IssuerID != "e3888094" {
		t.Errorf("symbols[96] = %+v, want trailing BOND row AAA & Sons Enterprises Pvt Ltd / e3888094", last)
	}
}

// TestSetTypeSwitchWhileStreamingReplay freezes the mid-stream market-data
// type switch (MD1-001): SetType(Delayed) plus an AAPL quote stream draws the
// marketDataType(3) push, the code-10167 warning as a session event, and
// delayed ticks; switching to SetType(Live) mid-stream drew no
// marketDataType(1) re-ack in the captured window and the delayed ticks kept
// flowing. Capture 20260611T074112Z-set_type_switch_while_streaming.
func TestSetTypeSwitchWhileStreamingReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "set_type_switch_while_streaming.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("SetType(MarketDataDelayed) error = %v", err)
	}

	sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
	})
	if err != nil {
		t.Fatalf("SubscribeQuotes() error = %v", err)
	}
	waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionStarted)

	// 1. The marketDataType(3) push arrives before any tick.
	update := waitForEvent(t, sub.Events())
	if update.Changed != ibkr.QuoteFieldMarketDataType {
		t.Fatalf("update 1 Changed = %v, want QuoteFieldMarketDataType", update.Changed)
	}
	if update.Snapshot.MarketDataType != ibkr.MarketDataDelayed {
		t.Fatalf("update 1 MarketDataType = %v, want %v", update.Snapshot.MarketDataType, ibkr.MarketDataDelayed)
	}

	// 2. Delayed volume (tick 74) has no quote-field mapping: the update
	// carries no changed fields.
	update = waitForEvent(t, sub.Events())
	if update.Changed != 0 {
		t.Fatalf("update 2 Changed = %v, want 0 (delayed volume tick)", update.Changed)
	}

	// 3. Delayed close (tick 75) lands in the normalized Close field.
	update = waitForEvent(t, sub.Events())
	if update.Changed != ibkr.QuoteFieldClose {
		t.Fatalf("update 3 Changed = %v, want QuoteFieldClose", update.Changed)
	}
	if update.Snapshot.Close.String() != "291.58" {
		t.Fatalf("close = %s, want 291.58", update.Snapshot.Close.String())
	}

	// 4. Delayed last (tick 68) lands in the normalized Last field.
	update = waitForEvent(t, sub.Events())
	if update.Changed != ibkr.QuoteFieldLast {
		t.Fatalf("update 4 Changed = %v, want QuoteFieldLast", update.Changed)
	}
	if update.Snapshot.Last.String() != "0" {
		t.Fatalf("last = %s, want 0", update.Snapshot.Last.String())
	}

	// Switch back to live mid-stream. The Gateway accepted the request...
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataLive); err != nil {
		t.Fatalf("SetType(MarketDataLive) error = %v", err)
	}

	// 5-7. ...but sent no marketDataType(1) re-ack in the captured window:
	// the stream continues with delayed open/bid/ask ticks (wire value -1.00
	// = no delayed quote available outside market hours).
	update = waitForEvent(t, sub.Events())
	if update.Changed != ibkr.QuoteFieldOpen {
		t.Fatalf("update 5 Changed = %v, want QuoteFieldOpen", update.Changed)
	}
	update = waitForEvent(t, sub.Events())
	if update.Changed != ibkr.QuoteFieldBid {
		t.Fatalf("update 6 Changed = %v, want QuoteFieldBid", update.Changed)
	}
	if update.Snapshot.Bid.String() != "-1" {
		t.Fatalf("bid = %s, want -1", update.Snapshot.Bid.String())
	}
	update = waitForEvent(t, sub.Events())
	if update.Changed != ibkr.QuoteFieldAsk {
		t.Fatalf("update 7 Changed = %v, want QuoteFieldAsk", update.Changed)
	}
	if update.Snapshot.Ask.String() != "-1" {
		t.Fatalf("ask = %s, want -1", update.Snapshot.Ask.String())
	}
	if update.Snapshot.MarketDataType != ibkr.MarketDataDelayed {
		t.Fatalf("MarketDataType after switch = %v, want still %v (no type-1 re-ack in capture window)",
			update.Snapshot.MarketDataType, ibkr.MarketDataDelayed)
	}

	// The code-10167 warning surfaced as a session event (the subscription
	// stayed open); it was processed before the ticks consumed above, so it
	// is already buffered.
	events := client.SessionEvents()
	deadline := time.After(2 * time.Second)
	for found := false; !found; {
		select {
		case ev := <-events:
			found = ev.Code == 10167
		case <-deadline:
			t.Fatal("code 10167 delayed-data warning not observed in session events")
		}
	}

	if err := sub.Close(); err != nil {
		t.Fatalf("sub.Close() error = %v", err)
	}
}

// TestCurrentTimeMillisReplay freezes explicit reqCurrentTimeInMillis
// (OUT 105) answered by the live epoch-millisecond reply (IN 109), both
// versionless, captured 2026-06-11 against the paper Gateway
// (captures/20260611T091447Z-current_time_millis, events.jsonl sha256
// prefix 23d6cedcf61b86fa).
func TestCurrentTimeMillisReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "current_time_millis.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ts, err := client.CurrentTimeMillis(ctx)
	if err != nil {
		t.Fatalf("CurrentTimeMillis: %v", err)
	}
	if want := time.UnixMilli(1781169286652).UTC(); !ts.Equal(want) {
		t.Fatalf("CurrentTimeMillis = %v, want %v", ts, want)
	}
}

// TestAPIFAReplaceNonFAReplay freezes the non-FA blocker for FA group
// replacement (captures/20260611T143728Z-api_fa_replace_non_fa, events.jsonl
// sha256 prefix 81e43254856879c6): ReplaceConfig is fire-and-forget and
// returns nil once sent, and the Gateway's code-321 "FA data operations
// ignored for non FA customers" reply matches no route, so the engine drops
// it and the session stays healthy.
func TestAPIFAReplaceNonFAReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_fa_replace_non_fa.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	groups := ibkr.XMLDocument(`<?xml version="1.0" encoding="UTF-8"?><ListOfGroups><Group><name>capture_probe</name><defaultMethod>EqualQuantity</defaultMethod><ListOfAccts varName="list"><Account><acct>DU9000001</acct></Account></ListOfAccts></Group></ListOfGroups>`)
	if err := client.Advisors().ReplaceConfig(ctx, ibkr.FADataGroups, groups); err != nil {
		t.Fatalf("ReplaceConfig: %v", err)
	}

	// The 321 blocker is dropped without perturbing the session: no event
	// surfaces and the session stays usable until the scripted disconnect.
	select {
	case evt := <-client.SessionEvents():
		if evt.Code == 321 {
			t.Fatalf("321 unexpectedly surfaced as a session event: %+v", evt)
		}
	case <-time.After(300 * time.Millisecond):
	}
}

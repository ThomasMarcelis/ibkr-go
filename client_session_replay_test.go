package ibkr_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
)

// Replay freezes for read-only live-Gateway sessions and reference captures.
// Each test replays a transcript
// derived frame-for-frame from a sanitized live capture; the capture dir and
// sha256 prefix are recorded in the transcript headers.

// TestCurrentTimeExplicitReplay freezes the explicit reqCurrentTime exchange
// (SESS-002): client [49, 1] is answered by a seconds-resolution epoch.
// Capture 20260824T202747Z-current_time at server version 225.
func TestCurrentTimeExplicitReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "current_time_live.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ts, err := client.CurrentTime(ctx)
	if err != nil {
		t.Fatalf("CurrentTime() error = %v", err)
	}
	want := time.Unix(1787603266, 0).UTC()
	if !ts.Equal(want) {
		t.Errorf("CurrentTime() = %v, want %v", ts, want)
	}
	if got := client.Session().CurrentTime; !got.Equal(want) {
		t.Errorf("Session().CurrentTime = %v, want %v", got, want)
	}
}

func TestRefreshOrderIDReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "req_ids.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	orderID, err := client.Orders().RefreshOrderID(ctx)
	if err != nil {
		t.Fatalf("RefreshOrderID() error = %v", err)
	}
	if orderID != 581 {
		t.Fatalf("RefreshOrderID() = %d, want 581", orderID)
	}
	if got := client.Session().NextValidID; got != 581 {
		t.Fatalf("Session().NextValidID = %d, want 581", got)
	}
}

func TestManagedAccountsRefreshReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "managed_accounts_refresh.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	accounts, err := client.ManagedAccounts(ctx)
	if err != nil {
		t.Fatalf("ManagedAccounts() error = %v", err)
	}
	want := []string{"DU9000001"}
	if !reflect.DeepEqual(accounts, want) {
		t.Fatalf("ManagedAccounts() = %v, want %v", accounts, want)
	}
	if got := client.Session().ManagedAccounts; !reflect.DeepEqual(got, want) {
		t.Fatalf("Session().ManagedAccounts = %v, want %v", got, want)
	}

	accounts[0] = "mutated"
	if got := client.Session().ManagedAccounts; !reflect.DeepEqual(got, want) {
		t.Fatalf("mutating result changed Session().ManagedAccounts to %v", got)
	}
}

// TestReqIDsReadOnlyRejectedReplay freezes the read-only-mode reqIds
// rejection (SESS-003): RefreshOrderID sends the captured reqIds frame, the
// Gateway answers with req_id=-1/code 321, and the one-shot returns that real
// APIError without changing the allocation seed. Capture
// 20260824T202844Z-req_ids, events SHA-256
// 3ac9d3b8565581414dd7499809fea1781c17bf3fd674d28b6a21081c9538a01c.
func TestReqIDsReadOnlyRejectedReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "req_ids_read_only.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.Orders().RefreshOrderID(ctx)
	apiErr, ok := errors.AsType[*ibkr.APIError](err)
	if !ok {
		t.Fatalf("RefreshOrderID() error = %v, want *APIError", err)
	}
	if apiErr.Code != 321 || apiErr.OpKind != ibkr.OpOrderID {
		t.Fatalf("RefreshOrderID() error = %#v, want code 321 op %s", apiErr, ibkr.OpOrderID)
	}

	// The transcript ends with the gateway disconnect. With ReconnectOff,
	// Done proves the code-321 rejection was processed before shutdown.
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

	// The req_id=-1 code-321 rejection belongs to RefreshOrderID and is not
	// duplicated as a session event.
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
// rows. Capture 20260824T202810Z-matching_symbols_partial at server version
// 225, events.jsonl SHA-256
// 365b920c42a3c9fdb34be92b5dadf8e49cfd8bb79eb808f08316106c6befb556.
func TestMatchingSymbolsPartialReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "matching_symbols_partial.txt")
	defer cleanupClientHost(t, client, host)

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

	// Protobuf issuer-id BOND rows omit conID, so the public zero value is 0;
	// they also carry no symbol or venue fields.
	bond := symbols[2]
	if bond.ConID != 0 || bond.SecType != ibkr.SecTypeBond || bond.Symbol != "" {
		t.Errorf("symbols[2] = %+v, want conID 0 BOND with empty symbol", bond)
	}
	if bond.Description != "Alcoa Nederland Holding BV" || bond.IssuerID != "e3231099" {
		t.Errorf("symbols[2] = %q / %q, want Alcoa Nederland Holding BV / e3231099", bond.Description, bond.IssuerID)
	}

	aapl := symbols[19]
	if aapl.ConID != 265598 || aapl.Symbol != "AAPL" || aapl.PrimaryExchange != "NASDAQ" || aapl.Description != "APPLE INC" {
		t.Errorf("symbols[19] = %+v, want AAPL 265598 NASDAQ APPLE INC", aapl)
	}

}

// TestSetTypeSwitchWhileStreamingReplay freezes the mid-stream market-data
// type switch (MD1-001): SetType(Delayed) plus an AAPL quote stream draws its
// parameters, marketDataType(3), the code-10167 warning, and a delayed open;
// switching to SetType(Live) draws no type-1 re-ack before the next delayed
// high tick. Capture 20260824T202845Z-set_type_switch_while_streaming.
func TestSetTypeSwitchWhileStreamingReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "set_type_switch_while_streaming.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("SetType(MarketDataDelayed) error = %v", err)
	}

	sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
	})
	if err != nil {
		t.Fatalf("SubscribeQuotes() error = %v", err)
	}
	waitForStateKind(t, sub.Events(), ibkr.StreamStarted)
	var notices []*ibkr.APIError
	nextData := func() ibkr.QuoteUpdate {
		for {
			event := waitForEvent(t, sub.Events())
			if event.Kind == ibkr.StreamNotice {
				notices = append(notices, event.Notice)
				continue
			}
			if event.Kind == ibkr.StreamData {
				return event.Value
			}
		}
	}

	// 1. The live tickReqParams callback is ancillary and does not mutate the
	// accumulated quote.
	update := nextData()
	if update.Kind != ibkr.QuoteUpdateParameters || update.Parameters == nil {
		t.Fatalf("update 1 = %+v, want QuoteUpdateParameters", update)
	}
	if update.Parameters.MinTick == nil || update.Parameters.MinTick.String() != "0.01" ||
		update.Parameters.BBOExchange != "9c0001" ||
		update.Parameters.SnapshotPermissions == nil || *update.Parameters.SnapshotPermissions != 4 {
		t.Fatalf("update 1 Parameters = %+v", update.Parameters)
	}

	// 2. The Gateway then identifies the stream as delayed.
	update = nextData()
	if update.Changed != ibkr.QuoteFieldMarketDataType || update.Snapshot.MarketDataType != ibkr.MarketDataDelayed {
		t.Fatalf("update 2 = %+v, want delayed market-data type", update)
	}

	// 3. Delayed open (tick 76) lands in the normalized Open field.
	update = nextData()
	if update.Changed != ibkr.QuoteFieldOpen || update.Snapshot.Open.String() != "311.22" {
		t.Fatalf("update 3 = %+v, want delayed open 311.22", update)
	}

	// Switch back to live mid-stream. The Gateway accepts the request without
	// sending a marketDataType(1) acknowledgement in the captured window.
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataLive); err != nil {
		t.Fatalf("SetType(MarketDataLive) error = %v", err)
	}

	// The next callback remains a delayed high tick, proving no type-1 re-ack.
	update = nextData()
	if update.Changed != ibkr.QuoteFieldHigh || update.Snapshot.High.String() != "313.36" {
		t.Fatalf("update 4 = %+v, want delayed high 313.36", update)
	}
	if update.Snapshot.MarketDataType != ibkr.MarketDataDelayed {
		t.Fatalf("MarketDataType after switch = %v, want still %v (no type-1 re-ack in capture window)",
			update.Snapshot.MarketDataType, ibkr.MarketDataDelayed)
	}

	found := false
	for _, notice := range notices {
		found = found || notice != nil && notice.Code == 10167
	}
	if !found {
		t.Fatal("code 10167 delayed-data warning not observed on quote stream")
	}

	sub.Close()
	if err := sub.Wait(); err != nil {
		t.Fatalf("quote subscription Wait() error = %v", err)
	}
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime() fence error = %v", err)
	}
}

// TestCurrentTimeMillisReplay freezes explicit reqCurrentTimeInMillis
// (OUT 105) answered by the live epoch-millisecond reply (IN 109), both
// versionless, captured 2026-08-24 against readonly-live at server version
// 225 (capture 20260824T202747Z-current_time_millis, events.jsonl SHA-256
// a428454d37be0a2d4176d70fb02d8d2163836b6cce86fae0af7481fe1fdb9433).
func TestCurrentTimeMillisReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "current_time_millis.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ts, err := client.CurrentTimeMillis(ctx)
	if err != nil {
		t.Fatalf("CurrentTimeMillis: %v", err)
	}
	if want := time.UnixMilli(1787603266761).UTC(); !ts.Equal(want) {
		t.Fatalf("CurrentTimeMillis = %v, want %v", ts, want)
	}
}

package ibkr_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
)

// Replay freezes for read-only live-Gateway sessions and reference captures.
// Each test replays a transcript
// derived frame-for-frame from a sanitized live capture; the capture dir and
// sha256 prefix are recorded in the transcript headers.

// TestCurrentTimeExplicitReplay freezes the explicit reqCurrentTime exchange
// (SESS-002): client [49, 1] is answered by a seconds-resolution epoch.
// Capture 20260710T215126Z-current_time at server version 206.
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
	want := time.Unix(1783720285, 0).UTC()
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
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	orderID, err := client.Orders().RefreshOrderID(ctx)
	if err != nil {
		t.Fatalf("RefreshOrderID() error = %v", err)
	}
	if orderID != 1 {
		t.Fatalf("RefreshOrderID() = %d, want 1", orderID)
	}
	if got := client.Session().NextValidID; got != 1 {
		t.Fatalf("Session().NextValidID = %d, want 1", got)
	}
}

func TestManagedAccountsRefreshReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "managed_accounts_refresh.txt")
	defer client.Close()
	defer waitHost(t, host)

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

func TestManagedAccountsRefreshSV207Replay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "managed_accounts_sv207_live.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	accounts, err := client.ManagedAccounts(ctx)
	if err != nil {
		t.Fatalf("ManagedAccounts() error = %v", err)
	}
	if want := []string{"DU9000001"}; !reflect.DeepEqual(accounts, want) {
		t.Fatalf("ManagedAccounts() = %v, want %v", accounts, want)
	}
	if got := client.Session().ServerVersion; got != 207 {
		t.Fatalf("Session().ServerVersion = %d, want 207", got)
	}
}

// TestReqIDsReadOnlyRejectedReplay freezes the read-only-mode reqIds
// rejection (SESS-003): RefreshOrderID sends the captured reqIds frame, the
// Gateway answers with req_id=-1/code 321, and the one-shot returns that real
// APIError without changing the allocation seed. Capture
// 20260710T215126Z-req_ids.
func TestReqIDsReadOnlyRejectedReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "req_ids_read_only.txt")
	defer client.Close()
	defer waitHost(t, host)

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

}

// TestSetTypeSwitchWhileStreamingReplay freezes the mid-stream market-data
// type switch (MD1-001): SetType(Delayed) plus an AAPL quote stream draws its
// parameters, marketDataType(3), the code-10167 warning, and delayed ticks;
// switching to SetType(Live) draws no type-1 re-ack before the next delayed
// tick. Capture 20260711T003316Z-set_type_switch_while_streaming.
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
	waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionStarted)

	// 1. The live tickReqParams callback is ancillary and does not mutate the
	// accumulated quote.
	update := waitForEvent(t, sub.Events())
	if update.Kind != ibkr.QuoteUpdateParameters || update.Parameters == nil {
		t.Fatalf("update 1 = %+v, want QuoteUpdateParameters", update)
	}
	if update.Parameters.MinTick == nil || update.Parameters.MinTick.String() != "0.01" ||
		update.Parameters.BBOExchange != "9c0001" ||
		update.Parameters.SnapshotPermissions == nil || *update.Parameters.SnapshotPermissions != 4 {
		t.Fatalf("update 1 Parameters = %+v", update.Parameters)
	}

	// 2. The Gateway then identifies the stream as delayed.
	update = waitForEvent(t, sub.Events())
	if update.Changed != ibkr.QuoteFieldMarketDataType || update.Snapshot.MarketDataType != ibkr.MarketDataDelayed {
		t.Fatalf("update 2 = %+v, want delayed market-data type", update)
	}

	// 3. Delayed last (tick 68) lands in the normalized Last field.
	update = waitForEvent(t, sub.Events())
	if update.Changed != ibkr.QuoteFieldLast|ibkr.QuoteFieldLastSize ||
		update.Snapshot.Last.String() != "314.96" || update.Snapshot.LastSize.String() != "53" {
		t.Fatalf("update 3 = %+v, want delayed last 314.96 x 53", update)
	}

	// Switch back to live mid-stream. The Gateway accepts the request without
	// sending a marketDataType(1) acknowledgement in the captured window.
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataLive); err != nil {
		t.Fatalf("SetType(MarketDataLive) error = %v", err)
	}

	// The delayed last timestamp was already in flight before the switch.
	update = waitForEvent(t, sub.Events())
	if update.Kind != ibkr.QuoteUpdateStringTick || update.StringTick == nil ||
		update.StringTick.TickType != 88 || update.StringTick.Value != "1783727950" {
		t.Fatalf("update 4 = %+v, want delayed last timestamp", update)
	}

	// The next callback remains a delayed open tick, proving no type-1 re-ack.
	update = waitForEvent(t, sub.Events())
	if update.Changed != ibkr.QuoteFieldOpen || update.Snapshot.Open.String() != "314.7" {
		t.Fatalf("update 5 = %+v, want delayed open 314.7", update)
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

func TestTickNewsReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "tick_news_aapl_sv201_live.txt")
	defer client.Close()
	defer waitHost(t, host)

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
		GenericTicks: []ibkr.GenericTick{"mdoff", "292:BRFG"},
	})
	if err != nil {
		t.Fatalf("SubscribeQuotes() error = %v", err)
	}
	waitForStateKind(t, sub.Lifecycle(), ibkr.SubscriptionStarted)

	if update := waitForEvent(t, sub.Events()); update.Kind != ibkr.QuoteUpdateFields || update.Snapshot.MarketDataType != ibkr.MarketDataDelayed {
		t.Fatalf("market-data-type update = %+v", update)
	}
	if update := waitForEvent(t, sub.Events()); update.Kind != ibkr.QuoteUpdateParameters || update.Parameters == nil {
		t.Fatalf("quote-parameters update = %+v", update)
	}
	update := waitForEvent(t, sub.Events())
	if update.Kind != ibkr.QuoteUpdateNewsTick || update.NewsTick == nil {
		t.Fatalf("news update = %+v", update)
	}
	wantTime := time.UnixMilli(1758294759000).UTC()
	if !update.NewsTick.Time.Equal(wantTime) ||
		update.NewsTick.ProviderCode != "BRFG" ||
		update.NewsTick.ArticleID != "BRFG$1c2d5728" ||
		update.NewsTick.Headline != "Apple's iPhone 17 debuts to long lines and high demand as company eyes upgrade cycle boost" ||
		update.NewsTick.ExtraData != "A:800015:L:en:K:1.00:C:0.9999533295631409" {
		t.Fatalf("NewsTick = %+v", update.NewsTick)
	}
	if update.Changed != 0 || update.Snapshot.Available != ibkr.QuoteFieldMarketDataType || update.ReceivedAt.IsZero() {
		t.Fatalf("news update mutated snapshot or lacked receive time: %+v", update)
	}
	if err := sub.Close(); err != nil {
		t.Fatalf("Subscription.Close() error = %v", err)
	}
}

// TestCurrentTimeMillisReplay freezes explicit reqCurrentTimeInMillis
// (OUT 105) answered by the live epoch-millisecond reply (IN 109), both
// versionless, captured 2026-07-10 against the readonly Gateway
// (/tmp/ibkr-api-migration-captures/20260710T215126Z-current_time_millis,
// events.jsonl sha256 prefix 3070c6c9296d0eb2).
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
	if want := time.UnixMilli(1783720285807).UTC(); !ts.Equal(want) {
		t.Fatalf("CurrentTimeMillis = %v, want %v", ts, want)
	}
}

// TestAPIFAReplaceNonFAReplay freezes the non-FA blocker for FA group
// replacement (/tmp/ibkr-go-fa-replace-current-20260711/
// 20260711T033010Z-api_fa_replace_non_fa, events.jsonl sha256
// 132e15c631f93b7f97f02db8749febe580ed5f0a4a5c7b5a60b0dd30d9bf4954):
// ReplaceConfig is fire-and-forget and
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

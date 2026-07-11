package ibkr_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
)

func TestMarketDataServer206Replay(t *testing.T) {
	restore := ibkr.SetAdvertisedServerVersionMaxForTest(206)
	defer restore()

	client, host := newClient(t, "market_data_sv206_live.txt")
	defer client.Close()
	defer waitHost(t, host)
	if got := client.Session().ServerVersion; got != 206 {
		t.Fatalf("Session().ServerVersion = %d, want 206", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatal(err)
	}
	sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
		Contract: ibkr.Contract{
			ConID: 265598, Symbol: "AAPL", SecType: ibkr.SecTypeStock,
			Exchange: "SMART", PrimaryExchange: "NASDAQ", Currency: "USD",
		},
		GenericTicks: []ibkr.GenericTick{
			"100", "101", "105", "106", "165", "221", "225", "233",
			"236", "293", "294", "295", "318", "375", "411", "456",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	updates := make(map[ibkr.QuoteUpdateKind]ibkr.QuoteUpdate)
	for len(updates) < 6 {
		select {
		case update, ok := <-sub.Events():
			if !ok {
				t.Fatalf("quote events closed before server_version 206 updates: %v", sub.Err())
			}
			updates[update.Kind] = update
		case <-sub.Done():
			t.Fatalf("quote subscription closed before server_version 206 updates: %v", sub.Err())
		case <-ctx.Done():
			t.Fatalf("waiting for server_version 206 updates: %v", ctx.Err())
		}
	}
	parameters := updates[ibkr.QuoteUpdateParameters].Parameters
	if parameters == nil || parameters.SnapshotPermissions == nil || *parameters.SnapshotPermissions != 4 ||
		parameters.LastPricePrecision == nil || parameters.LastPricePrecision.String() != "0.000001" ||
		parameters.LastSizePrecision == nil || parameters.LastSizePrecision.String() != "0.000001" {
		t.Fatalf("parameters = %+v", parameters)
	}
	price := updates[ibkr.QuoteUpdatePriceTick].PriceTick
	if price == nil || price.TickType != 37 || price.Price.String() != "314.39199829" || price.Size == nil || !price.Size.IsZero() {
		t.Fatalf("price tick = %+v", price)
	}
	size := updates[ibkr.QuoteUpdateSizeTick].SizeTick
	if size == nil || size.TickType != 89 || size.Size == nil || size.Size.String() != "87871162" {
		t.Fatalf("size tick = %+v", size)
	}
	if generic := updates[ibkr.QuoteUpdateGenericTick].GenericTick; generic == nil || generic.Value.String() != "0.2757165688996371" {
		t.Fatalf("generic tick = %+v", generic)
	}
	if text := updates[ibkr.QuoteUpdateStringTick].StringTick; text == nil || text.Value != "1.05,1.09,20260810,0.27" {
		t.Fatalf("string tick = %+v", text)
	}

	sub.Close()
	if err := sub.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenOrdersReadOnlyRefusalServer206Replay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "open_orders_readonly_refusal_sv206_live.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.Orders().Open(ctx, ibkr.OpenOrdersScopeClient)
	apiErr, ok := errors.AsType[*ibkr.APIError](err)
	if !ok || apiErr.OpKind != ibkr.OpOpenOrders || apiErr.Code != 321 || !strings.Contains(apiErr.Message, "Read-Only mode") {
		t.Fatalf("Open() error = %v, want live code 321 read-only refusal", err)
	}
}

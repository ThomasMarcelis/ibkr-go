package ibkr_test

import (
	"context"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
)

func TestCFDQuoteRerouteReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "cfd_quote_reroute.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("SetType(Delayed) error = %v", err)
	}
	sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{Contract: ibkr.Contract{
		Symbol: "IBM", SecType: ibkr.SecTypeCFD, Exchange: "SMART", Currency: "USD",
	}}, ibkr.WithResumePolicy(ibkr.ResumeNever))
	if err != nil {
		t.Fatalf("SubscribeQuotes(IBM CFD) error = %v", err)
	}
	defer sub.Close()

	var latest ibkr.Quote
	var sawParameters, sawDelayed, sawWarning bool
	for latest.Available&ibkr.QuoteFieldClose == 0 {
		event := waitForEvent(t, sub.Events())
		switch event.Kind {
		case ibkr.StreamNotice:
			notice := event.Notice
			if notice != nil && notice.OpKind == ibkr.OpQuotes && notice.Code == 10167 {
				sawWarning = true
			}
		case ibkr.StreamData:
			update := event.Value
			latest = update.Snapshot
			sawParameters = sawParameters || update.Kind == ibkr.QuoteUpdateParameters &&
				update.Parameters != nil && update.Parameters.MinTick != nil &&
				update.Parameters.MinTick.String() == "0.01" &&
				update.Parameters.BBOExchange == "a60001"
			sawDelayed = sawDelayed || update.Changed == ibkr.QuoteFieldMarketDataType &&
				latest.MarketDataType == ibkr.MarketDataDelayed
		}
	}

	wantFields := ibkr.QuoteFieldHigh | ibkr.QuoteFieldLow | ibkr.QuoteFieldClose |
		ibkr.QuoteFieldVolume | ibkr.QuoteFieldMarketDataType
	if latest.Available&wantFields != wantFields || latest.High.String() != "234.43" ||
		latest.Low.String() != "229.51" || latest.Close.String() != "231.04" ||
		latest.Volume.String() != "29966" {
		t.Fatalf("rerouted IBM CFD quote = %+v, want captured delayed high/low/close/volume", latest)
	}
	if !sawParameters || !sawDelayed || !sawWarning {
		t.Fatalf("reroute callbacks: parameters=%t delayed=%t warning10167=%t", sawParameters, sawDelayed, sawWarning)
	}
}

package ibkr_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
)

func TestReconnectOneShotInterrupted(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "reconnect_oneshot_interrupted.txt",
		ibkr.WithReconnectPolicy(ibkr.ReconnectOff))
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := client.History().Bars(ctx, ibkr.HistoricalBarsRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Duration:   ibkr.Days(1),
		BarSize:    ibkr.Bar1Hour,
		WhatToShow: ibkr.ShowTrades,
		UseRTH:     true,
	})
	if !errors.Is(err, ibkr.ErrInterrupted) {
		t.Fatalf("HistoricalBars() error = %v, want %v", err, ibkr.ErrInterrupted)
	}
}

func TestReconnectPolicyOff(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "quote_stream_disconnect.txt",
		ibkr.WithReconnectPolicy(ibkr.ReconnectOff))
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("SetType() error = %v", err)
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

	started := waitForStateKind(t, sub.Events(), ibkr.StreamStarted)
	if started.ConnectionSeq != 1 {
		t.Fatalf("started.ConnectionSeq = %d, want 1", started.ConnectionSeq)
	}

	// Transport drops. With ReconnectOff, the subscription and client close.
	if err := sub.Wait(); err == nil {
		t.Fatal("sub.Wait() error = nil, want error on disconnect")
	}

	select {
	case <-client.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("client.Done() did not complete after disconnect")
	}

	if err := client.Wait(); err == nil {
		t.Fatal("client.Wait() error = nil, want error on disconnect")
	}
}

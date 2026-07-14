package ibkr

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
)

func TestHistoricalTicksRejectsMismatchedResponseFamily(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	e.nextReqID = 1
	result := make(chan error, 1)
	// Request shape: historical_ticks_trades.txt, server_version 206,
	// events sha256 ecc715f0a1aea00c. The MIDPOINT callback below is deliberate
	// fault injection across the three live-grounded response families.
	go func() {
		_, err := e.HistoricalTicks(context.Background(), HistoricalTicksRequest{
			Contract: Contract{
				ConID: 265598, Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD",
			},
			EndTime:       time.Date(2026, 7, 10, 22, 44, 44, 0, time.UTC),
			NumberOfTicks: 100,
			WhatToShow:    ShowTrades,
			UseRTH:        true,
		})
		result <- err
	}()

	(<-e.cmds)()
	_ = readObservedFrame(t, peer)
	e.handleIncoming(codec.HistoricalTicksResponse{ReqID: 1, Done: true})

	err := <-result
	protocolErr, ok := errors.AsType[*ProtocolError](err)
	if !ok || protocolErr.Direction != "inbound" || !strings.Contains(protocolErr.Error(), "MIDPOINT response for TRADES request") {
		t.Fatalf("HistoricalTicks() error = %T %v, want inbound family mismatch", err, err)
	}
	if _, ok := e.keyed[1]; ok {
		t.Fatal("mismatched historical-tick response retained its route")
	}
}

func TestHistoricalTicksResultIdentifiesEmptyFamily(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	e.nextReqID = 1
	type historicalResult struct {
		value HistoricalTicksResult
		err   error
	}
	result := make(chan historicalResult, 1)
	go func() {
		got, err := e.HistoricalTicks(context.Background(), HistoricalTicksRequest{
			Contract:      Contract{ConID: 265598, Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD"},
			EndTime:       time.Date(2026, 7, 13, 17, 0, 0, 0, time.UTC),
			NumberOfTicks: 1,
			WhatToShow:    ShowBidAsk,
		})
		result <- historicalResult{value: got, err: err}
	}()

	(<-e.cmds)()
	_ = readObservedFrame(t, peer)
	e.handleIncoming(codec.HistoricalTicksBidAskResponse{ReqID: 1, Done: true})

	got := <-result
	if got.err != nil {
		t.Fatalf("HistoricalTicks() error = %v", got.err)
	}
	if got.value.WhatToShow != ShowBidAsk || got.value.Len() != 0 {
		t.Fatalf("HistoricalTicks() = %+v, want identified empty BID_ASK result", got.value)
	}
}

func TestHistoricalDataUpdatePreservesCapturedBarCount(t *testing.T) {
	t.Parallel()

	bar, err := fromCodecHistoricalDataUpdate(codec.HistoricalDataUpdate{
		ReqID: 7, BarCount: 14193, Time: "20260713 17:00:00 Europe/Amsterdam",
		Open: "319.13", High: "319.23", Low: "316.85", Close: "317.07",
		Volume: "1520619", WAP: "318.027",
	})
	if err != nil {
		t.Fatalf("fromCodecHistoricalDataUpdate() error = %v", err)
	}
	if bar.Count != 14193 {
		t.Fatalf("Bar.Count = %d, want captured count 14193", bar.Count)
	}
}

func TestRealtimeBarUsesCapturedEpochSeconds(t *testing.T) {
	t.Parallel()

	bar, err := fromCodecRealtimeBar(codec.RealTimeBar{
		ReqID: 7, Time: "1752423600", Open: "317", High: "318", Low: "316",
		Close: "317.5", Volume: "1000", WAP: "317.2", Count: "42",
	})
	if err != nil {
		t.Fatalf("fromCodecRealtimeBar() error = %v", err)
	}
	want := time.Unix(1752423600, 0).UTC()
	if !bar.Time.Equal(want) || bar.Time.Location() != time.UTC {
		t.Fatalf("Bar.Time = %s (%s), want %s UTC", bar.Time, bar.Time.Location(), want)
	}
	if bar.Count != 42 {
		t.Fatalf("Bar.Count = %d, want 42", bar.Count)
	}
}

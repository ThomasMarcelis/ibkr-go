package ibkr

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
)

func TestContractDetailsContextCancellationReachesBrokerAt215(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	e.serverVersion = 215
	e.nextReqID = 7601
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)

	go func() {
		_, err := e.ContractDetails(ctx, Contract{
			ConID: 265598, Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD",
		})
		result <- err
	}()

	(<-e.cmds)()
	_ = readObservedFrame(t, peer)
	cancel()
	(<-e.cmds)()

	want, err := codec.Encode(215, codec.CancelContractData{ReqID: 7601})
	if err != nil {
		t.Fatal(err)
	}
	if got := readObservedFrame(t, peer); !bytes.Equal(got, want) {
		t.Fatalf("broker cancellation = %x, want %x", got, want)
	}
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("ContractDetails() error = %v, want context.Canceled", err)
	}
	if _, ok := e.keyed[7601]; ok {
		t.Fatal("canceled contract-details request retained its route")
	}
}

func TestHistoricalTicksContextCancellationReachesBrokerAt215(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	e.serverVersion = 215
	e.nextReqID = 7602
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)

	go func() {
		_, err := e.HistoricalTicks(ctx, HistoricalTicksRequest{
			Contract: Contract{
				ConID: 265598, Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD",
			},
			EndTime:       time.Date(2026, 7, 13, 16, 0, 0, 0, time.UTC),
			NumberOfTicks: 100,
			WhatToShow:    ShowTrades,
		})
		result <- err
	}()

	(<-e.cmds)()
	_ = readObservedFrame(t, peer)
	cancel()
	(<-e.cmds)()

	want, err := codec.Encode(215, codec.CancelHistoricalTicks{ReqID: 7602})
	if err != nil {
		t.Fatal(err)
	}
	if got := readObservedFrame(t, peer); !bytes.Equal(got, want) {
		t.Fatalf("broker cancellation = %x, want %x", got, want)
	}
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("HistoricalTicks() error = %v, want context.Canceled", err)
	}
	if _, ok := e.keyed[7602]; ok {
		t.Fatal("canceled historical-ticks request retained its route")
	}
}

func TestContractDetailsCanceledBeforeAdmissionDoesNotSendRequestZero(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	e.serverVersion = 215
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)

	go func() {
		_, err := e.ContractDetails(ctx, Contract{
			ConID: 265598, Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD",
		})
		result <- err
	}()

	setup := <-e.cmds
	cancel()
	setup()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("ContractDetails() error = %v, want context.Canceled", err)
	}

	assertNextFrameIsFence(t, e, peer)
}

func TestHistoricalTicksCanceledBeforeAdmissionDoesNotSendRequestZero(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	e.serverVersion = 215
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)

	go func() {
		_, err := e.HistoricalTicks(ctx, HistoricalTicksRequest{
			Contract: Contract{
				ConID: 265598, Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD",
			},
			EndTime:       time.Date(2026, 7, 13, 16, 0, 0, 0, time.UTC),
			NumberOfTicks: 100,
			WhatToShow:    ShowTrades,
		})
		result <- err
	}()

	setup := <-e.cmds
	cancel()
	setup()
	(<-e.cmds)()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("HistoricalTicks() error = %v, want context.Canceled", err)
	}

	assertNextFrameIsFence(t, e, peer)
}

func assertNextFrameIsFence(t *testing.T, e *engine, peer io.Reader) {
	t.Helper()
	fence := codec.ReqMarketDataType{DataType: int(MarketDataLive)}
	if err := e.sendContext(context.Background(), fence); err != nil {
		t.Fatalf("send fence: %v", err)
	}
	want, err := codec.Encode(e.serverVersion, fence)
	if err != nil {
		t.Fatalf("encode fence: %v", err)
	}
	got, err := wire.ReadFrame(peer)
	if err != nil {
		t.Fatalf("read fence: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("first frame = %x, want fence %x", got, want)
	}
}

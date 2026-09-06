package ibkr

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
)

func TestHistoricalLastTicksPublicProjectionCaptured(t *testing.T) {
	// Exact sv215 callback, API 10.48.01 capture
	// 20260713T164153Z-sdk_sv215_request_cancels, events.jsonl SHA-256
	// b3515b46284970f338db6ede7b2864f4d63449027f9f48ff203a67f4fd34d019.
	// The allocator is set to the captured query ID; response bytes are unchanged.
	compressed, err := os.ReadFile("internal/codec/testdata/historical_ticks_last_sv215.gz")
	if err != nil {
		t.Fatal(err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	frame, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	messages, err := codec.DecodeBatch(215, frame)
	if err != nil {
		t.Fatal(err)
	}
	e, peer := newObservedMarketDataEngine(t)
	e.serverVersion, e.nextReqID = 215, 7602
	result := make(chan HistoricalTicksResult, 1)
	failure := make(chan error, 1)
	go func() {
		ticks, err := (HistoryClient{engine: e}).Ticks(t.Context(), HistoricalTicksRequest{Contract: Stock("AAPL"), StartTime: time.Unix(1783960023, 0), NumberOfTicks: 100, WhatToShow: ShowTrades})
		result <- ticks
		failure <- err
	}()
	(<-e.cmds)()
	_ = readObservedFrame(t, peer)
	for _, message := range messages {
		e.handleIncoming(message)
	}
	ticks := <-result
	if err := <-failure; err != nil {
		t.Fatal(err)
	}
	if ticks.Len() != 128 || len(ticks.Last) != 128 || len(ticks.BidAsk) != 0 || len(ticks.Ticks) != 0 {
		t.Fatalf("tick families/count = %+v", ticks)
	}
	first := ticks.Last[0]
	if first.Time.Unix() != 1783960023 || first.Size.String() != "13" || first.Exchange != "BATS" || first.SpecialConditions != " F I" {
		t.Fatalf("first trade = %+v", first)
	}
	if len(e.keyed) != 0 {
		t.Fatal("completed tick collector retained route")
	}
}

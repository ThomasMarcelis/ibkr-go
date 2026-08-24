package ibkr

import (
	"bytes"
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
)

func TestWhatIfUnclaimedEchoDoesNotBecomeOpenOrder(t *testing.T) {
	whatIf := capturedWhatIfOpenOrder(t)
	e, _ := newAttributionEngine()
	e.previews[whatIf.OrderID] = &previewRoute{result: make(chan previewResult, 1)}
	delete(e.previews, whatIf.OrderID) // caller cancellation wins before the echo
	observed := make(chan OpenOrder, 1)
	e.singletons[singletonOpenOrders] = &route{handle: func(msg any, _ *engine) {
		observed <- msg.(OpenOrder)
	}}

	e.handleIncoming(whatIf)
	select {
	case order := <-observed:
		t.Fatalf("cancelled preview leaked as open order %d", *order.Order.OrderID)
	default:
	}
}

func TestWhatIfUnclaimedPreviewDoesNotClaimRealOpenOrder(t *testing.T) {
	realOrder := attributionCallback("open_order", 1, attributionPermID, 0).(codec.OpenOrder)
	e, _ := newAttributionEngine()
	preview := &previewRoute{result: make(chan previewResult, 1)}
	e.previews[realOrder.OrderID] = preview
	observed := make(chan OpenOrder, 1)
	e.singletons[singletonOpenOrders] = &route{handle: func(msg any, _ *engine) {
		observed <- msg.(OpenOrder)
	}}

	e.handleIncoming(realOrder)
	select {
	case result := <-preview.result:
		t.Fatalf("real open order resolved preview: %+v", result)
	default:
	}
	select {
	case <-observed:
	default:
		t.Fatal("real open order was not routed normally")
	}
}

func capturedWhatIfOpenOrder(t *testing.T) codec.OpenOrder {
	t.Helper()
	const path = "testdata/transcripts/api_whatif_margin_aapl.txt"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		encoded, ok := strings.CutPrefix(line, "raw server ")
		if !ok {
			continue
		}
		frame, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatal(err)
		}
		payload, err := wire.ReadFrame(bytes.NewReader(frame))
		if err != nil {
			t.Fatal(err)
		}
		messages, err := codec.DecodeBatch(225, payload)
		if err != nil {
			t.Fatal(err)
		}
		for _, message := range messages {
			if order, ok := message.(codec.OpenOrder); ok {
				return order
			}
		}
	}
	t.Fatalf("%s contains no open order", path)
	return codec.OpenOrder{}
}

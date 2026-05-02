package ibkr

import (
	"context"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
)

func TestSDKScannerSubscriptionUsesSDKCommandsAndEvents(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	if err := e.sendSDKContext(context.Background(), sdkadapter.ScannerSubscriptionRequest{
		ReqID:        81,
		NumberOfRows: 10,
		Instrument:   "STK",
		LocationCode: "STK.US.MAJOR",
		ScanCode:     "TOP_PERC_GAIN",
	}); err != nil {
		t.Fatalf("sendSDKContext(ScannerSubscriptionRequest) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.CancelScannerSubscription{ReqID: 81}); err != nil {
		t.Fatalf("sendSDKContext(CancelScannerSubscription) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 2 {
		t.Fatalf("commands len = %d, want 2", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandScannerSubscription {
		t.Fatalf("scanner subscription command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandScannerSubscription)
	}
	if commands[0].ScannerSubscription.ReqID != 81 || commands[0].ScannerSubscription.NumberOfRows != 10 {
		t.Fatalf("scanner subscription command = %+v, want reqID 81 rows 10", commands[0].ScannerSubscription)
	}
	if commands[0].ScannerSubscription.Instrument != "STK" || commands[0].ScannerSubscription.LocationCode != "STK.US.MAJOR" || commands[0].ScannerSubscription.ScanCode != "TOP_PERC_GAIN" {
		t.Fatalf("scanner subscription command = %+v, want STK/STK.US.MAJOR/TOP_PERC_GAIN", commands[0].ScannerSubscription)
	}
	if commands[1].Kind != sdkadapter.CommandCancelScannerSubscription {
		t.Fatalf("cancel scanner command kind = %s, want %s", commands[1].Kind, sdkadapter.CommandCancelScannerSubscription)
	}
	if commands[1].CancelScannerSubscription.ReqID != 81 {
		t.Fatalf("cancel scanner reqID = %d, want 81", commands[1].CancelScannerSubscription.ReqID)
	}

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind:  sdkadapter.EventScannerData,
		ReqID: 81,
		ScannerData: []sdkadapter.ScannerDataValue{{
			Rank: 0,
			Contract: sdkadapter.Contract{
				Symbol:   "AAPL",
				SecType:  "STK",
				Exchange: "SMART",
				Currency: "USD",
			},
			Distance:   "1.0",
			Benchmark:  "SPX",
			Projection: "projected",
			LegsStr:    "legs",
		}},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(scanner data) error = %v", err)
	}
	got, ok := msg.(sdkadapter.ScannerDataResponse)
	if !ok {
		t.Fatalf("sdkEventToMessage(scanner data) type = %T, want sdkadapter.ScannerDataResponse", msg)
	}
	if got.ReqID != 81 || len(got.Entries) != 1 {
		t.Fatalf("scanner data response = %+v, want reqID 81 one entry", got)
	}
	entry := got.Entries[0]
	if entry.Rank != 0 || entry.Contract.Symbol != "AAPL" || entry.Distance != "1.0" || entry.Benchmark != "SPX" || entry.Projection != "projected" || entry.LegsStr != "legs" {
		t.Fatalf("scanner data entry = %+v, want copied scanner row", entry)
	}
}
